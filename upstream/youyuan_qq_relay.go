package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// 群聊消息 → 游戏 的四种转发模式（用户原话：
// 「什么特定触发词，或者它能不能自动触发、自动转发开启了之后，它某一时间自动转发」）。
//
//	off      —— 关闭，默认值（用户没说要默认开）
//	keyword  —— 特定触发词：消息里含任一触发词才转这一条
//	auto     —— 全自动：群里每条（非指令）都转
//	interval —— 定时打包：每 N 分钟把这期间的新消息按时间顺序合并成一条转一次
//
// 配置落在 qq-bridge.json 的 assistant 段（relay_mode / relay_keywords /
// relay_interval_minutes / relay_max_messages），与旧的 relay_chat 布尔共存：
// 旧开关为 true 且没写 relay_mode 时按 auto 处理（老用户升级后行为不变）。
//
// 本文件只负责「转哪些、什么时候转」；真正发到游戏走 yyQQRelayGame（tellraw，
// 不会被当成指令，也不会被 [群] 前缀的聊天回环再转回来）。
const (
	yyQQRelayModeOff      = "off"
	yyQQRelayModeKeyword  = "keyword"
	yyQQRelayModeAuto     = "auto"
	yyQQRelayModeInterval = "interval"

	// 间隔模式默认 10 分钟，允许 1..1440；单次打包默认 20 条，允许 1..200。
	yyQQRelayDefaultInterval = 10
	yyQQRelayDefaultMax      = 20
	yyQQRelayMinInterval     = 1
	yyQQRelayMaxInterval     = 1440
	yyQQRelayMinMax          = 1
	yyQQRelayMaxMax          = 200

	// 排队上限：群友刷屏时不能让内存无限涨，超了丢最旧的并在下次打包里说明。
	yyQQRelayQueueCap = 500
	// 重试上限：游戏没连上时消息留在队列里，连续 3 轮发不出去就丢弃。
	yyQQRelayMaxTries = 3
	// 打包轮询粒度。最小间隔 1 分钟，15 秒一判足够准。
	yyQQRelayTick = 15 * time.Second
	// 打包单行最多保留多少字（长消息会截断，避免一次刷满屏）。
	yyQQRelayLineRunes = 200
)

type yyQQRelayItem struct {
	ID    string    `json:"id"`
	Group string    `json:"group"`
	Name  string    `json:"name"`
	User  string    `json:"user"`
	Text  string    `json:"text"`
	At    time.Time `json:"at"`
	Tries int       `json:"tries"`
}

var yyQQRelay = struct {
	sync.Mutex
	Items   []yyQQRelayItem
	Seen    map[string]time.Time
	Last    map[string]time.Time
	Dropped int
	LastAt  time.Time
	LastN   int
	Sent    int
	Warned  bool
}{Seen: map[string]time.Time{}, Last: map[string]time.Time{}}

var yyQQRelayLoopOnce sync.Once

// yyQQRelayMode 归一化 relay_mode。空值/拼错一律按 off；
// 旧配置只开了 relay_chat（布尔）而没写 relay_mode 时按 auto 处理。
func yyQQRelayMode(cf yyQQConfig) string {
	switch strings.ToLower(strings.TrimSpace(cf.Assistant.RelayMode)) {
	case yyQQRelayModeKeyword:
		return yyQQRelayModeKeyword
	case yyQQRelayModeAuto:
		return yyQQRelayModeAuto
	case yyQQRelayModeInterval:
		return yyQQRelayModeInterval
	case yyQQRelayModeOff, "关", "关闭":
		return yyQQRelayModeOff
	}
	if cf.Assistant.RelayChat {
		return yyQQRelayModeAuto
	}
	return yyQQRelayModeOff
}

// yyQQRelayKeywords 把玩家填的触发词拆开：换行、中英文逗号、顿号、分号都算分隔符。
func yyQQRelayKeywords(cf yyQQConfig) []string {
	raw := strings.ReplaceAll(cf.Assistant.RelayKeywords, "\r\n", "\n")
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case '\n', ',', '，', '、', ';', '；', '|':
			return true
		}
		return false
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if v := strings.TrimSpace(f); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func yyQQRelayClamp(v, lo, hi, fallback int) int {
	if v <= 0 {
		return fallback
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func yyQQRelayInterval(cf yyQQConfig) int {
	return yyQQRelayClamp(cf.Assistant.RelayIntervalMinutes, yyQQRelayMinInterval, yyQQRelayMaxInterval, yyQQRelayDefaultInterval)
}

func yyQQRelayMax(cf yyQQConfig) int {
	return yyQQRelayClamp(cf.Assistant.RelayMaxMessages, yyQQRelayMinMax, yyQQRelayMaxMax, yyQQRelayDefaultMax)
}

func yyQQRelayModeLabel(mode string) string {
	switch mode {
	case yyQQRelayModeKeyword:
		return "触发词"
	case yyQQRelayModeAuto:
		return "全自动"
	case yyQQRelayModeInterval:
		return "定时打包"
	}
	return "关闭"
}

// yyQQRelayLegacyWillSend 判断旧的 relay_chat 分支（youyuan_qq.go 里的助手分支）
// 会不会自己把这条消息转进游戏。会的话我们不再重复发一遍。
func yyQQRelayLegacyWillSend(cf yyQQConfig, text string) bool {
	if !cf.Assistant.Enabled || !cf.Assistant.RelayChat {
		return false
	}
	return !strings.HasPrefix(text, "/") && !strings.HasPrefix(text, ".") && text != "帮助" && text != "list"
}

// yyQQRelayOnMessage 是每条群消息的采集点（在 youyuan_qq_context.go 里接线）。
// 幂等由调用方的 yyQQClaimMessage 保证，这里再按 message_id 去一次重。
func yyQQRelayOnMessage(cf yyQQConfig, m yyQQGroupMessage) {
	if m.IsBot || strings.TrimSpace(m.Text) == "" {
		return
	}
	if strings.HasPrefix(m.Text, "/") || strings.HasPrefix(m.Text, ".") {
		// 指令不进群聊转发：执行结果另有回执，混进游戏只会变成噪声。
		return
	}
	mode := yyQQRelayMode(cf)
	if mode == yyQQRelayModeOff {
		return
	}
	if mode != yyQQRelayModeAuto && cf.Assistant.RelayChat {
		yyQQRelay.Lock()
		warn := !yyQQRelay.Warned
		yyQQRelay.Warned = true
		yyQQRelay.Unlock()
		if warn {
			yyQQLog("旧的「群聊转发进游戏」开关仍开着，会覆盖新的转发模式；在「群→游戏 转发」里保存一次即可接管")
		}
	}
	if m.ID != "" {
		yyQQRelay.Lock()
		if _, dup := yyQQRelay.Seen[m.ID]; dup {
			yyQQRelay.Unlock()
			return
		}
		yyQQRelay.Seen[m.ID] = time.Now()
		if len(yyQQRelay.Seen) > 2048 {
			cut := time.Now().Add(-30 * time.Minute)
			for k, t := range yyQQRelay.Seen {
				if t.Before(cut) {
					delete(yyQQRelay.Seen, k)
				}
			}
		}
		yyQQRelay.Unlock()
	}
	item := yyQQRelayItem{ID: m.ID, Group: m.Group, Name: m.Name, User: m.User, Text: strings.TrimSpace(m.Text), At: m.Timestamp}
	if item.At.IsZero() {
		item.At = time.Now()
	}
	if item.Name == "" {
		item.Name = "群友"
	}
	switch mode {
	case yyQQRelayModeAuto:
		// 旧分支已经会发同样的一条，别重复。
		if yyQQRelayLegacyWillSend(cf, m.Text) {
			return
		}
		if err := yyQQRelayGame("[群] " + item.Name + "：" + item.Text); err != nil {
			yyQQLog("群聊自动转发失败：" + err.Error())
			return
		}
		yyQQRelay.Lock()
		yyQQRelay.Sent++
		yyQQRelay.LastAt = time.Now()
		yyQQRelay.LastN = 1
		yyQQRelay.Unlock()
	case yyQQRelayModeKeyword:
		if !yyQQRelayHit(item.Text, yyQQRelayKeywords(cf)) {
			return
		}
		if err := yyQQRelayGame("[群] " + item.Name + "：" + item.Text); err != nil {
			yyQQLog("群聊触发词转发失败：" + err.Error())
			return
		}
		yyQQRelay.Lock()
		yyQQRelay.Sent++
		yyQQRelay.LastAt = time.Now()
		yyQQRelay.LastN = 1
		yyQQRelay.Unlock()
	case yyQQRelayModeInterval:
		yyQQRelayEnqueue(item)
	}
}

func yyQQRelayHit(text string, keywords []string) bool {
	low := strings.ToLower(text)
	for _, k := range keywords {
		if k != "" && strings.Contains(low, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func yyQQRelayEnqueue(item yyQQRelayItem) {
	yyQQRelay.Lock()
	defer yyQQRelay.Unlock()
	for _, old := range yyQQRelay.Items {
		if item.ID != "" && old.ID == item.ID {
			return
		}
	}
	yyQQRelay.Items = append(yyQQRelay.Items, item)
	if len(yyQQRelay.Items) > yyQQRelayQueueCap {
		over := len(yyQQRelay.Items) - yyQQRelayQueueCap
		yyQQRelay.Items = append([]yyQQRelayItem(nil), yyQQRelay.Items[over:]...)
		yyQQRelay.Dropped += over
	}
	// 第一次进队列时把计时起点放在现在，避免开开关的瞬间就把历史消息打包发出去。
	if _, ok := yyQQRelay.Last[item.Group]; !ok {
		yyQQRelay.Last[item.Group] = time.Now()
	}
}

// yyQQRelayTake 取该群最旧的 limit 条（按时间升序），剩下的留在队列里等下一轮。
func yyQQRelayTake(group string, limit int) ([]yyQQRelayItem, int, int) {
	yyQQRelay.Lock()
	defer yyQQRelay.Unlock()
	if limit <= 0 {
		limit = yyQQRelayDefaultMax
	}
	picked := []yyQQRelayItem{}
	rest := make([]yyQQRelayItem, 0, len(yyQQRelay.Items))
	for _, it := range yyQQRelay.Items {
		if it.Group == group {
			picked = append(picked, it)
		} else {
			rest = append(rest, it)
		}
	}
	sort.SliceStable(picked, func(i, j int) bool { return picked[i].At.Before(picked[j].At) })
	take := len(picked)
	if take > limit {
		take = limit
	}
	out := append([]yyQQRelayItem(nil), picked[:take]...)
	rest = append(rest, picked[take:]...)
	sort.SliceStable(rest, func(i, j int) bool { return rest[i].At.Before(rest[j].At) })
	yyQQRelay.Items = rest
	left := len(picked) - take
	dropped := yyQQRelay.Dropped
	yyQQRelay.Dropped = 0
	return out, left, dropped
}

// yyQQRelayPutBack 发送失败时把这一批塞回队列最前面（保留尝试次数），
// 连续失败 yyQQRelayMaxTries 次才真丢，避免游戏断线期间整段群聊被吞掉。
func yyQQRelayPutBack(items []yyQQRelayItem) {
	if len(items) == 0 {
		return
	}
	yyQQRelay.Lock()
	defer yyQQRelay.Unlock()
	keep := make([]yyQQRelayItem, 0, len(items))
	for _, it := range items {
		it.Tries++
		if it.Tries < yyQQRelayMaxTries {
			keep = append(keep, it)
		}
	}
	yyQQRelay.Items = append(keep, yyQQRelay.Items...)
	if len(yyQQRelay.Items) > yyQQRelayQueueCap {
		over := len(yyQQRelay.Items) - yyQQRelayQueueCap
		yyQQRelay.Items = append([]yyQQRelayItem(nil), yyQQRelay.Items[over:]...)
		yyQQRelay.Dropped += over
	}
}

func yyQQRelayClip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// yyQQRelayBatchText 组打包正文：标题带群名与条数，逐行「昵称：内容」，按时间序。
func yyQQRelayBatchText(cf yyQQConfig, group string, items []yyQQRelayItem, left, dropped int) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("〔" + yyQQGroupLabel(cf, group) + " · " + fmt.Sprint(len(items)) + " 条群聊〕")
	for _, it := range items {
		b.WriteString("\n" + it.Name + "：" + yyQQRelayClip(strings.ReplaceAll(it.Text, "\n", " "), yyQQRelayLineRunes))
	}
	if left > 0 {
		b.WriteString("\n…另有 " + fmt.Sprint(left) + " 条，下一轮继续")
	}
	if dropped > 0 {
		b.WriteString("\n（队列已满，丢弃了 " + fmt.Sprint(dropped) + " 条最早的消息）")
	}
	return b.String()
}

// yyQQRelayFlush 把某个群的待转发消息打包发送一次。
func yyQQRelayFlush(cf yyQQConfig, group string) {
	items, left, dropped := yyQQRelayTake(group, yyQQRelayMax(cf))
	if len(items) == 0 {
		yyQQRelay.Lock()
		yyQQRelay.Last[group] = time.Now()
		yyQQRelay.Unlock()
		return
	}
	text := yyQQRelayBatchText(cf, group, items, left, dropped)
	if err := yyQQRelayGame(text); err != nil {
		yyQQRelayLogFlushError(err)
		yyQQRelayPutBack(items)
		return
	}
	yyQQRelay.Lock()
	yyQQRelay.Last[group] = time.Now()
	yyQQRelay.LastAt = time.Now()
	yyQQRelay.LastN = len(items)
	yyQQRelay.Sent += len(items)
	yyQQRelay.Unlock()
	yyQQLog("群聊打包转发：" + fmt.Sprint(len(items)) + " 条")
}

var yyQQRelayFlushErrOnce sync.Once

func yyQQRelayLogFlushError(err error) {
	yyQQRelayFlushErrOnce.Do(func() {
		yyQQLog("群聊定时打包暂未发出：" + err.Error() + "（消息已留在队列，等下次再试）")
	})
}

// yyQQRelayLoopStart 起打包循环，重复调用只起一次（不碰主控的 youyuan_qq.go）。
func yyQQRelayLoopStart() {
	yyQQRelayLoopOnce.Do(func() {
		go func() {
			t := time.NewTicker(yyQQRelayTick)
			defer t.Stop()
			for range t.C {
				yyQQRelayTickOnce()
			}
		}()
	})
}

func yyQQRelayTickOnce() {
	cf := yyQQSnapshot()
	mode := yyQQRelayMode(cf)
	now := time.Now()
	yyQQRelay.Lock()
	if mode != yyQQRelayModeInterval && len(yyQQRelay.Items) > 0 {
		// 切走定时模式后队列里的旧消息不再有意义，直接清掉，避免以后突然冒出来。
		yyQQRelay.Items = nil
		yyQQRelay.Last = map[string]time.Time{}
		yyQQRelay.Unlock()
		return
	}
	groups := map[string]bool{}
	for _, it := range yyQQRelay.Items {
		groups[it.Group] = true
	}
	due := []string{}
	for g := range groups {
		last, ok := yyQQRelay.Last[g]
		if !ok || last.IsZero() {
			yyQQRelay.Last[g] = now
			continue
		}
		if now.Sub(last) >= time.Duration(yyQQRelayInterval(cf))*time.Minute {
			due = append(due, g)
		}
	}
	yyQQRelay.Unlock()
	sort.Strings(due)
	for _, g := range due {
		yyQQRelayFlush(cf, g)
	}
}

// yyQQRelayStatus 给面板和游戏内 .通 状态 用的统一读数。
func yyQQRelayStatus(cf yyQQConfig) map[string]any {
	mode := yyQQRelayMode(cf)
	yyQQRelay.Lock()
	queued := 0
	perGroup := map[string]int{}
	for _, it := range yyQQRelay.Items {
		queued++
		perGroup[it.Group]++
	}
	lastAt := yyQQRelay.LastAt
	lastN := yyQQRelay.LastN
	sent := yyQQRelay.Sent
	yyQQRelay.Unlock()
	return map[string]any{
		"mode":              mode,
		"mode_label":        yyQQRelayModeLabel(mode),
		"keywords":          yyQQRelayKeywords(cf),
		"interval_minutes":  yyQQRelayInterval(cf),
		"max_messages":      yyQQRelayMax(cf),
		"queued":            queued,
		"queued_by_group":   perGroup,
		"last_at":           lastAt,
		"last_count":        lastN,
		"sent_total":        sent,
		"legacy_relay_chat": cf.Assistant.RelayChat,
	}
}

// yyQQRelayStatusText 游戏内 .通 状态 的回执（一行一条，够看就行）。
func yyQQRelayStatusText(cf yyQQConfig, player string) string {
	st := yyQQRelayStatus(cf)
	lines := []string{
		"群→游戏转发：" + fmt.Sprint(st["mode_label"]),
		fmt.Sprintf("间隔 %d 分钟｜单次上限 %d 条", st["interval_minutes"], st["max_messages"]),
		"当前排队 " + fmt.Sprint(st["queued"]) + " 条",
	}
	curMode := fmt.Sprint(st["mode"])
	if curMode == yyQQRelayModeKeyword {
		kw := yyQQRelayKeywords(cf)
		if len(kw) == 0 {
			lines = append(lines, "触发词：未填写（填了才会转）")
		} else {
			lines = append(lines, "触发词："+strings.Join(kw, " / "))
		}
	}
	if last, ok := st["last_at"].(time.Time); ok && !last.IsZero() {
		lines = append(lines, fmt.Sprintf("上次转发 %d 分钟前，%d 条", int(time.Since(last).Minutes()), st["last_count"]))
	}
	cmd := yyQQCommandText(cf, "relay")
	lines = append(lines, "切换："+cmd+" 自动 开 / "+cmd+" 自动 关")
	if curMode == yyQQRelayModeOff && cf.Assistant.RelayChat {
		lines = append(lines, "（旧的转发开关还开着：在面板「群→游戏 转发」保存一次即可接管）")
	}
	return strings.Join(lines, "\n")
}

// yyQQRelaySave 保存转发配置。只写 qq-bridge.json，不改连接参数，因此不会触发断线重连。
func yyQQRelaySave(patch map[string]any) (map[string]any, error) {
	yyQQMu.Lock()
	cf := yyQQ
	mode := strings.ToLower(strings.TrimSpace(cf.Assistant.RelayMode))
	if v, ok := patch["mode"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case yyQQRelayModeOff, "":
			mode = yyQQRelayModeOff
		case yyQQRelayModeKeyword:
			mode = yyQQRelayModeKeyword
		case yyQQRelayModeAuto:
			mode = yyQQRelayModeAuto
		case yyQQRelayModeInterval:
			mode = yyQQRelayModeInterval
		default:
			yyQQMu.Unlock()
			return nil, fmt.Errorf("转发模式支持 off/keyword/auto/interval")
		}
		cf.Assistant.RelayMode = mode
	}
	if v, ok := patch["keywords"].(string); ok {
		cf.Assistant.RelayKeywords = strings.TrimSpace(v)
	}
	if v, ok := patch["interval_minutes"]; ok {
		if n, e := yyQQRelayNumber(v); e == nil {
			cf.Assistant.RelayIntervalMinutes = yyQQRelayClamp(int(n), yyQQRelayMinInterval, yyQQRelayMaxInterval, yyQQRelayDefaultInterval)
		} else {
			yyQQMu.Unlock()
			return nil, fmt.Errorf("间隔分钟数无效")
		}
	}
	if v, ok := patch["max_messages"]; ok {
		if n, e := yyQQRelayNumber(v); e == nil {
			cf.Assistant.RelayMaxMessages = yyQQRelayClamp(int(n), yyQQRelayMinMax, yyQQRelayMaxMax, yyQQRelayDefaultMax)
		} else {
			yyQQMu.Unlock()
			return nil, fmt.Errorf("单次上限无效")
		}
	}
	if clear, ok := patch["clear_queue"].(bool); ok && clear {
		yyQQRelay.Lock()
		yyQQRelay.Items = nil
		yyQQRelay.Dropped = 0
		yyQQRelay.Unlock()
	}
	// 新模式一保存，旧布尔开关就让位：否则旧分支会把每条消息再各转一遍（重复刷屏）。
	cf.Assistant.RelayChat = false
	cf.Assistant.RelayMode = yyQQRelayMode(cf)
	if cf.Assistant.RelayIntervalMinutes <= 0 {
		cf.Assistant.RelayIntervalMinutes = yyQQRelayDefaultInterval
	}
	if cf.Assistant.RelayMaxMessages <= 0 {
		cf.Assistant.RelayMaxMessages = yyQQRelayDefaultMax
	}
	raw, _ := json.MarshalIndent(cf, "", "  ")
	err := os.WriteFile(yyQQPath, raw, 0600)
	if err == nil {
		yyQQ = cf
	}
	yyQQMu.Unlock()
	if err != nil {
		return nil, err
	}
	// 间隔模式的计时从「保存这一刻」重新开始，避免刚开就把积压的旧消息打包发出去。
	yyQQRelay.Lock()
	yyQQRelay.Last = map[string]time.Time{}
	for _, it := range yyQQRelay.Items {
		yyQQRelay.Last[it.Group] = time.Now()
	}
	yyQQRelay.Unlock()
	yyQQLog("群→游戏转发模式已设为：" + yyQQRelayModeLabel(yyQQRelayMode(cf)))
	return yyQQRelayStatus(yyQQSnapshot()), nil
}

func yyQQRelayNumber(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case json.Number:
		return n.Float64()
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, fmt.Errorf("空值")
		}
		var f float64
		if _, e := fmt.Sscanf(s, "%f", &f); e != nil {
			return 0, e
		}
		return f, nil
	}
	return 0, fmt.Errorf("类型不支持")
}

// yyQQRelayGameCommand 处理游戏内的 .通 状态 / .通 自动 开|关。
// 返回 true 表示这条消息已被转发开关吃掉，不再当成普通消息发到群里。
func yyQQRelayGameCommand(cf yyQQConfig, player, text string) bool {
	cmd := yyQQCommandText(cf, "relay")
	if cmd == "" || !yyQQHasPrefix(text, cmd) {
		return false
	}
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), cmd))
	if arg == "" {
		return false
	}
	if arg == "状态" || arg == "狀態" {
		yyQQGameNotice(player, yyQQRelayStatusText(cf, player))
		return true
	}
	if !strings.HasPrefix(arg, "自动") {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(arg, "自动"))
	if rest == "" {
		yyQQGameNotice(player, "格式："+cmd+" 自动 开 或 "+cmd+" 自动 关；查看现状发 "+cmd+" 状态")
		return true
	}
	target := ""
	switch strings.ToLower(rest) {
	case "开", "打开", "开启", "on":
		target = yyQQRelayModeAuto
	case "关", "关闭", "停", "off":
		target = yyQQRelayModeOff
	default:
		return false
	}
	st, err := yyQQRelaySave(map[string]any{"mode": target})
	if err != nil {
		yyQQGameNotice(player, "切换失败："+err.Error())
		return true
	}
	if target == yyQQRelayModeOff {
		yyQQGameNotice(player, player+" 关闭了群→游戏转发")
	} else {
		yyQQGameNotice(player, player+" 开启了群→游戏全自动转发（群里每条消息都会进游戏）")
	}
	yyQQLog("游戏内切换转发模式：" + player + " → " + fmt.Sprint(st["mode_label"]))
	return true
}

// yyQQRelayRoutes 面板接口：GET 读取现状，POST 保存（前端 onchange 直接打这里）。
func yyQQRelayRoutes(mux *http.ServeMux) {
	yyQQRelayLoopStart()
	mux.HandleFunc("/api/qq/relay", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var patch map[string]any
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&patch); err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			st, err := yyQQRelaySave(patch)
			if err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "config": st})
			return
		}
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		cf := yyQQSnapshot()
		st := yyQQRelayStatus(cf)
		st["hint"] = "游戏内：" + yyQQCommandText(cf, "relay") + " 消息 手动转；" + yyQQCommandText(cf, "relay") + " 状态 看现状；" + yyQQCommandText(cf, "relay") + " 自动 开|关 切换全自动"
		writeJSON(w, map[string]any{"ok": true, "config": st})
	})
}
