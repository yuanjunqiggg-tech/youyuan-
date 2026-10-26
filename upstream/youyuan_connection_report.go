package main

// 连接诊断报告：把"登录成功后被服务器踢下线"变成工具箱里可观测、可解释的事件。
//
// 为什么需要这个模块：
//   - /api/bot/status 只返回瞬时布尔 connected，没有任何掉线历史/计数/原因；
//   - yyAutoConnection.history 只记录拨号失败，而登录成功后再掉线会走
//     yyEndConnectionAttempt(nil) 把 failures/lastError 清零——痕迹被抹掉；
//   - 唯一留下真实原因的地方是 <dataDir>/connection-errors.log，此前无任何 API 暴露。
//
// 结果就是用户体感：bot 动不动掉线，工具箱里查不到，刷新也没用。

import (
	"bufio"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bot-apk/config"
)

// yyDropEvent 是一次「连上之后又掉了」的记录。
type yyDropEvent struct {
	At            time.Time `json:"at"`
	UptimeSeconds float64   `json:"uptime_seconds"` // 本次连接持续了多久
	AwaySeconds   float64   `json:"away_seconds"`   // 掉了多久才恢复（0=未恢复）
	Server        string    `json:"server"`
	Reason        string    `json:"reason"` // 分类原因
	Label         string    `json:"label"`  // 原因中文名
	Detail        string    `json:"detail"` // 原始错误串（截断）
	Hint          string    `json:"hint"`
}

var yyDrops = struct {
	sync.Mutex
	events       []yyDropEvent
	total        uint64
	today        string
	todayCnt     uint64
	lastAt       time.Time
	connectedAt  time.Time
	droppedAt    time.Time
	best         float64
	wasConnected bool
	pending      int // 尚未结算 away 时长的事件下标，-1 表示无
}{pending: -1}

// yyDropReasonKind 把 RakNet/传输层错误串归类成用户能看懂的原因。
func yyDropReasonKind(message string) string {
	s := strings.ToLower(message)
	switch {
	case strings.Contains(s, "封禁") || strings.Contains(s, "banned"):
		return "banned"
	case strings.Contains(s, "remote disconnect notification"):
		return "server_kicked"
	case strings.Contains(s, "connection refused"):
		return "server_unreachable"
	case strings.Contains(s, "idle timeout"):
		return "idle_timeout"
	case strings.Contains(s, "network is unreachable"):
		return "local_network"
	case strings.Contains(s, "use of closed network connection"):
		return "conn_closed"
	case strings.TrimSpace(s) == "":
		return "unknown"
	default:
		return "connection_error"
	}
}

var yyDropReasonLabels = map[string]string{
	"banned":             "被服务器封禁",
	"server_kicked":      "服务端主动断开",
	"server_unreachable": "服务端端口不可达",
	"idle_timeout":       "长时间无响应超时",
	"local_network":      "本机网络不可达",
	"conn_closed":        "连接被关闭",
	"unknown":            "原因未知",
	"connection_error":   "连接错误",
}

var yyDropReasonHints = map[string]string{
	"banned":             "服务器限制了机器人登录（受限管理员上限）。可在服务器侧放宽管理员数量，或换用普通玩家账号连接。",
	"server_kicked":      "服务器在 RakNet 层主动通知断开，通常是租赁服限时/限流策略。已开启自动重连，掉线后会自动恢复。",
	"server_unreachable": "服务器的 UDP 端口拒绝报文，多为租赁服瞬时不可用或重启。属于外部因素，工具箱会自动重连。",
	"idle_timeout":       "超过 30 秒没有收到任何报文，多为网络抖动或服务器卡顿。",
	"local_network":      "本机网络不可达，请检查box 的出口网络。",
	"conn_closed":        "底层连接已关闭后的次级报错，真实原因看上一条记录。",
	"unknown":            "未捕获到具体错误串，可查看连接日志原文。",
	"connection_error":   "连接层错误，可查看连接日志原文。",
}

func yyDropLabel(kind string) string {
	if v, ok := yyDropReasonLabels[kind]; ok {
		return v
	}
	return kind
}
func yyDropHint(kind string) string {
	if v, ok := yyDropReasonHints[kind]; ok {
		return v
	}
	return ""
}

func yyClip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// yyNoteConnectionSample 由自动连接巡检调用，识别 connected  true->false 的跳变。
func yyNoteConnectionSample(connected bool, server string) {
	yyDrops.Lock()
	defer yyDrops.Unlock()
	now := time.Now()
	if today := now.Format("2006-01-02"); today != yyDrops.today {
		yyDrops.today, yyDrops.todayCnt = today, 0
	}
	if connected {
		if !yyDrops.wasConnected {
			if yyDrops.pending >= 0 && yyDrops.pending < len(yyDrops.events) {
				yyDrops.events[yyDrops.pending].AwaySeconds = now.Sub(yyDrops.events[yyDrops.pending].At).Seconds()
			}
			yyDrops.pending = -1
			yyDrops.connectedAt = now
		}
		yyDrops.wasConnected = true
		return
	}
	if !yyDrops.wasConnected {
		return
	}
	yyDrops.wasConnected = false
	yyDrops.total++
	yyDrops.todayCnt++
	yyDrops.lastAt = now
	yyDrops.droppedAt = now
	uptime := 0.0
	if !yyDrops.connectedAt.IsZero() {
		uptime = now.Sub(yyDrops.connectedAt).Seconds()
		if uptime > yyDrops.best {
			yyDrops.best = uptime
		}
	}
	yyDrops.events = append(yyDrops.events, yyDropEvent{At: now, UptimeSeconds: uptime, Server: server, Reason: "unknown"})
	if len(yyDrops.events) > 64 {
		yyDrops.events = yyDrops.events[len(yyDrops.events)-64:]
	}
	yyDrops.pending = len(yyDrops.events) - 1
	best := yyDrops.best
	// 掉线是低频事件，这里直接把最新记录落盘，重启后「最长连续在线」不清零。
	yyUptimePersistNow(best)
}

// yyConnectionLogPath 返回连接错误日志路径（与 dialer.ErrorLog 同一文件）。
func yyConnectionLogPath() string {
	root := strings.TrimSpace(os.Getenv("YY_AUTH_CONFIG_DIR"))
	if root == "" {
		root = yyDataRoot
	}
	if root == "" {
		return ""
	}
	return filepath.Join(root, "connection-errors.log")
}

// ---- 最长连续在线：跨进程持久化 ----------------------------------------
//
// 以前 yyDrops.best 只在 yyNoteConnectionSample 的「掉线结算」分支里更新
// （本文件 149-153 行）。后果是：进程起来后一直在线、一次都没掉过，
// best 永远是 0，面板上「最长连续在线」就恒等于 0 秒——用户说的假数据。
// 现在两件事：
//   1) 读取诊断时把「本次连接已在线时长」并进 best（至少从本次连接算起）；
//   2) best 落盘到 <dataDir>/connection-uptime.json，刷新页面、重进面板、
//      甚至进程重启都不清零。

type yyUptimeRecord struct {
	BestSeconds float64 `json:"best_seconds"`
	UpdatedAt   string  `json:"updated_at"`
}

var yyUptimeState = struct {
	sync.Mutex
	loaded  bool
	persist float64   // 已落盘的历史最长秒数
	lastWr  time.Time // 上次落盘时间，用于限流
}{}

func yyUptimePath() string {
	p := yyConnectionLogPath()
	if p == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(p), "connection-uptime.json")
}

func yyUptimeLoadLocked() {
	if yyUptimeState.loaded {
		return
	}
	yyUptimeState.loaded = true
	p := yyUptimePath()
	if p == "" {
		return
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var rec yyUptimeRecord
	if json.Unmarshal(raw, &rec) != nil {
		return
	}
	v := rec.BestSeconds
	if v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
		yyUptimeState.persist = v
	}
}

func yyUptimeSaveLocked(best float64) {
	p := yyUptimePath()
	if p == "" {
		return
	}
	raw, err := json.Marshal(yyUptimeRecord{BestSeconds: best, UpdatedAt: time.Now().Format(time.RFC3339)})
	if err != nil {
		return
	}
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		return
	}
	yyUptimeState.persist = best
	yyUptimeState.lastWr = time.Now()
}

// yyUptimePersistNow 掉线结算时立即落盘（掉线是低频事件，不必限流）。
func yyUptimePersistNow(best float64) {
	yyUptimeState.Lock()
	defer yyUptimeState.Unlock()
	yyUptimeLoadLocked()
	if best > yyUptimeState.persist {
		yyUptimeSaveLocked(best)
	}
}

// yyUptimeMerge 把「内存/落盘/本次在线时长」三者取最大值并限流落盘。
func yyUptimeMerge(best, uptime float64) float64 {
	if uptime > best {
		best = uptime
	}
	yyUptimeState.Lock()
	defer yyUptimeState.Unlock()
	yyUptimeLoadLocked()
	if yyUptimeState.persist > best {
		best = yyUptimeState.persist
	}
	// 只有真的创新高才写盘，且最多每分钟一次，避免诊断轮询把磁盘打满。
	if best > yyUptimeState.persist+1 && time.Since(yyUptimeState.lastWr) >= time.Minute {
		yyUptimeSaveLocked(best)
	}
	return best
}

// yyParseConnectionLog 读取连接错误日志，抽出掉线相关事件（带真实原因）。
// 只读文件尾部 maxTailBytes：日志会持续增长，而诊断只需要今天 + 最近若干条。
func yyParseConnectionLog(limit int) []map[string]any {
	const maxTailBytes = 512 * 1024
	path := yyConnectionLogPath()
	if path == "" {
		return nil
	}
	f, e := os.Open(path)
	if e != nil {
		return nil
	}
	defer f.Close()
	if st, e := f.Stat(); e == nil && st.Size() > maxTailBytes {
		if _, e = f.Seek(st.Size()-maxTailBytes, io.SeekStart); e != nil {
			return nil
		}
	}
	type row struct {
		at   time.Time
		kind string
		msg  string
	}
	var rows []row
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "time=") {
			continue
		}
		low := strings.ToLower(line)
		var kind string
		switch {
		case strings.Contains(low, "connection closed"):
			kind = "closed"
		case strings.Contains(low, "remote disconnect notification"):
			kind = "kicked"
		case strings.Contains(low, "idle timeout"):
			kind = "idle"
		case strings.Contains(low, "connection refused"):
			kind = "refused"
		case strings.Contains(low, "network is unreachable"):
			kind = "unreachable"
		default:
			continue
		}
		at, ok := yyLogTime(line)
		if !ok {
			continue
		}
		rows = append(rows, row{at: at, kind: kind, msg: line})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].at.Before(rows[j].at) })
	if limit > 0 && len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		cause := yyLogField(r.msg, "cause=")
		if cause == "" {
			cause = yyLogField(r.msg, "msg=")
		}
		out = append(out, map[string]any{
			"at":     r.at.Format(time.RFC3339),
			"kind":   r.kind,
			"reason": yyDropReasonKind(cause),
			"label":  yyDropLabel(yyDropReasonKind(cause)),
			"detail": yyClip(cause, 220),
		})
	}
	return out
}

func yyLogTime(line string) (time.Time, bool) {
	i := strings.Index(line, "time=")
	if i < 0 {
		return time.Time{}, false
	}
	rest := line[i+5:]
	if j := strings.IndexAny(rest, " \t"); j >= 0 {
		rest = rest[:j]
	}
	t, e := time.Parse(time.RFC3339, rest)
	if e != nil {
		return time.Time{}, false
	}
	return t, true
}

// yyLogField 从 slog 文本行里取 "key=value" 或 "key=\"value\"" 的值。
func yyLogField(line, key string) string {
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	rest := line[i+len(key):]
	if strings.HasPrefix(rest, `"`) {
		rest = rest[1:]
		if j := strings.Index(rest, `"`); j >= 0 {
			rest = rest[:j]
		}
		return strings.TrimSpace(rest)
	}
	if j := strings.IndexAny(rest, " \t"); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

// yyAttachLogCauses 用日志里时间最接近的真实原因补全内存里的掉线事件。
func yyAttachLogCauses(events []yyDropEvent, logRows []map[string]any) []yyDropEvent {
	if len(logRows) == 0 {
		return events
	}
	for i := range events {
		if events[i].Reason != "unknown" && events[i].Reason != "" {
			continue
		}
		best, bestDiff := "", time.Duration(1<<62)
		for _, r := range logRows {
			at, e := time.Parse(time.RFC3339, yyDropString(r["at"]))
			if e != nil {
				continue
			}
			d := at.Sub(events[i].At)
			if d < 0 {
				d = -d
			}
			if d < bestDiff && d < 90*time.Second {
				bestDiff, best = d, yyDropString(r["reason"])
			}
		}
		if best != "" {
			events[i].Reason = best
		}
	}
	return events
}

func yyDropString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// yyLogTodayStats 统计某一天的掉线次数、原因分布与最后一次掉线时间。
// 独立成函数以便单测：内存计数器重启即清零，今日统计必须以日志为准。
func yyLogTodayStats(rows []map[string]any, today string) (count int, byReason map[string]int, lastAt time.Time) {
	byReason = map[string]int{}
	for _, r := range rows {
		at, e := time.Parse(time.RFC3339, yyDropString(r["at"]))
		if e != nil || at.Format("2006-01-02") != today {
			continue
		}
		count++
		byReason[yyDropString(r["reason"])]++
		if at.After(lastAt) {
			lastAt = at
		}
	}
	return count, byReason, lastAt
}

// yyLastLogReasonToday 取今天最后一条日志事件的原因（用于本进程还没掉过线时兜底）。
func yyLastLogReasonToday(rows []map[string]any, today string) string {
	for i := len(rows) - 1; i >= 0; i-- {
		at, e := time.Parse(time.RFC3339, yyDropString(rows[i]["at"]))
		if e != nil || at.Format("2006-01-02") != today {
			continue
		}
		return yyDropString(rows[i]["reason"])
	}
	return ""
}

// yyConnectionDiagnostics 汇总连接健康度，供 /api/youyuan/connection-diagnostics 返回。
func yyConnectionDiagnostics() map[string]any {
	c := config.ReloadConfig()
	bm := getBotManagerLocked()
	connected := bm != nil && bm.IsConnected()
	server := ""
	if bm != nil {
		server = bm.ServerCode()
	}
	var rtt any
	if connected && bm != nil {
		bm.mu.Lock()
		conn := bm.conn
		bm.mu.Unlock()
		if conn != nil {
			stats := conn.NetworkStats()
			available, _ := stats["available"].(bool)
			acknowledged, _ := stats["acknowledged_datagrams"].(uint64)
			rttMS, rttOK := stats["rtt_ms"].(float64)
			if available && acknowledged > 0 && rttOK && !math.IsNaN(rttMS) && !math.IsInf(rttMS, 0) && rttMS >= 0 {
				rtt = rttMS
			}
		}
	}

	yyDrops.Lock()
	events := append([]yyDropEvent{}, yyDrops.events...)
	total, todayCnt, lastAt := yyDrops.total, yyDrops.todayCnt, yyDrops.lastAt
	connectedAt, best := yyDrops.connectedAt, yyDrops.best
	yyDrops.Unlock()

	// 日志是全量真相：内存计数器随进程重启清零，刚启动时必须回退到日志统计，
	// 否则会误报"今日暂无掉线"（2026-10-23 部署后就出现过这个假象）。
	allLogRows := yyParseConnectionLog(0)
	today := time.Now().Format("2006-01-02")
	logToday, byReason, logLastAt := yyLogTodayStats(allLogRows, today)
	logRows := allLogRows
	if len(logRows) > 40 {
		logRows = logRows[len(logRows)-40:]
	}
	// 有效今日掉线数：内存计数优先（本进程亲眼所见），为空时用日志补齐。
	effectiveToday := todayCnt
	if effectiveToday == 0 {
		effectiveToday = uint64(logToday)
	}
	if lastAt.IsZero() && !logLastAt.IsZero() {
		lastAt = logLastAt
	}
	events = yyAttachLogCauses(events, logRows)
	for i := range events {
		events[i].Detail = yyClip(events[i].Detail, 220)
		events[i].Label = yyDropLabel(events[i].Reason)
		events[i].Hint = yyDropHint(events[i].Reason)
	}
	// 新的在前
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	uptime := 0.0
	if connected && !connectedAt.IsZero() {
		uptime = time.Since(connectedAt).Seconds()
	}
	// 最长连续在线 = max(历史结算记录, 已落盘记录, 本次连接已在线时长)。
	// 少了这一步，一直没掉过线的进程里 best 恒为 0，面板显示「0 秒」。
	best = yyUptimeMerge(best, uptime)

	lastReason, lastHint := "", ""
	if n := len(events); n > 0 {
		lastReason, lastHint = events[0].Reason, events[0].Hint
	} else if !logLastAt.IsZero() {
		// 本进程还没掉过线，但日志显示今天掉过：把最近一次日志原因顶上去。
		lastReason = yyLastLogReasonToday(allLogRows, today)
		lastHint = yyDropHint(lastReason)
	}

	policy := map[string]any{}
	if c != nil {
		policy = map[string]any{
			"auto_connect":            c.AutoConnect,
			"auto_reconnect":          c.AutoReconnect,
			"reconnect_seconds":       c.ReconnectSeconds,
			"effective_retry_seconds": yyConnectionRetryDelay(time.Duration(c.ReconnectSeconds)*time.Second, 0, "").Seconds(),
		}
	}

	assessment := "连接稳定"
	switch {
	case effectiveToday == 0:
		assessment = "今日暂无掉线"
	case effectiveToday <= 2:
		assessment = "今日掉线 " + strconv.Itoa(int(effectiveToday)) + " 次，属可接受范围"
	case effectiveToday <= 10:
		assessment = "今日掉线 " + strconv.Itoa(int(effectiveToday)) + " 次，建议查看下方原因"
	default:
		assessment = "今日掉线 " + strconv.Itoa(int(effectiveToday)) + " 次，掉线频繁，需处理"
	}

	lastDropAt := ""
	if !lastAt.IsZero() {
		lastDropAt = lastAt.Format(time.RFC3339)
	}

	return map[string]any{
		"ok":                 true,
		"connected":          connected,
		"server":             server,
		"rtt_ms":             rtt,
		"uptime_seconds":     uptime,
		"longest_uptime":     best,
		"drops_total":        total,
		"drops_today":        effectiveToday,
		"drops_today_memory": todayCnt,
		"drops_today_log":    logToday,
		"last_drop_at":       lastDropAt,
		"last_drop_reason":   lastReason,
		"last_drop_label":    yyDropLabel(lastReason),
		"last_drop_hint":     lastHint,
		"events":             events,
		"log_events":         logRows,
		"log_by_reason":      byReason,
		"log_path":           yyConnectionLogPath(),
		"policy":             policy,
		"assessment":         assessment,
	}
}

func yyConnectionDiagnosticsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	writeJSON(w, yyConnectionDiagnostics())
}
