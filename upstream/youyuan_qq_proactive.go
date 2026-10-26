package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// 主动行为打分（设计依据 docs/design/bot-persona.md §6）。
//
// 机器人不是只被动应答：群静默一段时间后，按「静默 + 话题 + 活跃度 + 时段」
// 四个维度打分，超过阈值才开口，并受冷却与每日上限两条硬约束。
// 本文件只回答「要不要说、为什么、提示什么」，具体说什么由调用方生成。

// yyProactiveWeights 四个打分维度权重，全部非负。
type yyProactiveWeights struct {
	Silence  float64 `json:"silence"`  // 静默分：沉默越久分越高
	Topic    float64 `json:"topic"`    // 话题分：近期群聊有热点话题
	Activity float64 `json:"activity"` // 活跃度惩罚：群太吵反而闭嘴
	Night    float64 `json:"night"`    // 深夜惩罚：23 点到次日 7 点降权
}

// yyProactiveConfig 主动行为配置，由 GET/POST /api/qq/proactive 读写。
type yyProactiveConfig struct {
	Enabled       bool               `json:"enabled"`
	MinSilenceSec int                `json:"min_silence_sec"` // 静默不满该值不计静默分
	CooldownSec   int                `json:"cooldown_sec"`    // 两次主动发言最小间隔（硬约束）
	DailyCap      int                `json:"daily_cap"`       // 每群每日主动发言上限（硬约束）
	Threshold     float64            `json:"threshold"`       // 开口阈值，总分 >= 阈值才说
	Weights       yyProactiveWeights `json:"weights"`
}

func yyProactiveDefaults() yyProactiveConfig {
	return yyProactiveConfig{
		// 默认关闭：主动开口会直接往群里发消息，不能让用户升级完就被机器人搭话。
		// 面板 GET/POST /api/qq/proactive 随时可以打开。
		Enabled:       false,
		MinSilenceSec: 900,
		CooldownSec:   1800,
		DailyCap:      8,
		Threshold:     1.2,
		Weights:       yyProactiveWeights{Silence: 1.0, Topic: 0.6, Activity: 0.8, Night: 1.5},
	}
}

const (
	// 群活跃统计窗口：保留 30 分钟内的消息做话题与活跃度判断。
	yyProactiveWindow = 30 * time.Minute
	// 窗口内超过该条数视为「群很吵」，活跃度惩罚据此计算。
	yyProactiveBusy = 30
	// 单群最多缓存的活跃消息条数，超出丢最旧。
	yyProactiveMaxMsgs = 300
	// 深夜区间：23:00 到次日 07:00 主动发言降权。
	yyProactiveNightStart = 23
	yyProactiveNightEnd   = 7
	// 静默分封顶倍数：沉默达到 MinSilence 的多少倍后不再加分。
	yyProactiveSilenceCap = 3.0
	// 话题分封顶：提到热点话题的消息达到该条数拿满分。
	yyProactiveTopicCap = 2.0
)

// yyProactiveGroupState 是按群持久化的部分（qq-proactive.json）。
type yyProactiveGroupState struct {
	LastProactiveTS int64  `json:"last_proactive_ts"` // Unix 秒，0 表示从未主动发言
	DailyCount      int    `json:"daily_count"`       // 当日已主动发言次数
	DailyDate       string `json:"daily_date"`        // "2006-01-02"，跨天自动重置计数
}

// yyProactiveFile 是 qq-proactive.json 的落盘结构。
type yyProactiveFile struct {
	Config yyProactiveConfig                 `json:"config"`
	Groups map[string]*yyProactiveGroupState `json:"groups"`
}

// yyProactiveMsg 是一条群消息的活跃记录（只留时间与话题词，不落盘）。
type yyProactiveMsg struct {
	at    time.Time
	words []string
}

// yyProactiveGroup 运行时按群聚合：持久化状态 + 内存活跃窗口。
type yyProactiveGroup struct {
	State  *yyProactiveGroupState
	Msgs   []yyProactiveMsg
	LastAt time.Time
}

var yyProactive = struct {
	sync.Mutex
	Path      string
	Config    yyProactiveConfig
	Groups    map[string]*yyProactiveGroup
	DiskError string
}{Groups: map[string]*yyProactiveGroup{}}

// yyProactiveInit 加载 qq-proactive.json（照 youyuan_qq_memory.go 的范式：
// tmp + rename 原子写；文件缺失或损坏都不致命，退回默认配置）。
func yyProactiveInit(root string) {
	yyProactive.Lock()
	defer yyProactive.Unlock()
	yyProactive.Path = filepath.Join(root, "qq-proactive.json")
	yyProactive.Config = yyProactiveDefaults()
	yyProactive.Groups = map[string]*yyProactiveGroup{}
	yyProactive.DiskError = ""
	file, e := os.Open(yyProactive.Path)
	if e != nil {
		return
	}
	defer file.Close()
	var disk yyProactiveFile
	if e := json.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&disk); e != nil {
		yyProactive.DiskError = "invalid_proactive_state"
		return
	}
	yyProactive.Config = yyProactiveNormalize(disk.Config)
	for name, st := range disk.Groups {
		if st == nil {
			continue
		}
		if st.DailyCount < 0 {
			st.DailyCount = 0
		}
		if st.LastProactiveTS < 0 {
			st.LastProactiveTS = 0
		}
		yyProactive.Groups[name] = &yyProactiveGroup{State: st}
	}
}

// yyProactiveNormalize 把磁盘配置里的零值字段补成默认值，Enabled 以存储为准。
func yyProactiveNormalize(c yyProactiveConfig) yyProactiveConfig {
	d := yyProactiveDefaults()
	d.Enabled = c.Enabled
	if c.MinSilenceSec > 0 {
		d.MinSilenceSec = c.MinSilenceSec
	}
	if c.CooldownSec > 0 {
		d.CooldownSec = c.CooldownSec
	}
	if c.DailyCap > 0 {
		d.DailyCap = c.DailyCap
	}
	if c.Threshold > 0 {
		d.Threshold = c.Threshold
	}
	if c.Weights.Silence > 0 {
		d.Weights.Silence = c.Weights.Silence
	}
	if c.Weights.Topic > 0 {
		d.Weights.Topic = c.Weights.Topic
	}
	if c.Weights.Activity > 0 {
		d.Weights.Activity = c.Weights.Activity
	}
	if c.Weights.Night > 0 {
		d.Weights.Night = c.Weights.Night
	}
	return d
}

// yyProactiveValidate 校验面板提交的配置范围。
func yyProactiveValidate(c yyProactiveConfig) error {
	if c.MinSilenceSec < 0 || c.MinSilenceSec > 86400 {
		return fmt.Errorf("静默下限需在 0..86400 秒之间")
	}
	if c.CooldownSec < 0 || c.CooldownSec > 86400 {
		return fmt.Errorf("冷却时间需在 0..86400 秒之间")
	}
	if c.DailyCap < 0 || c.DailyCap > 200 {
		return fmt.Errorf("每日上限需在 0..200 条之间")
	}
	if c.Threshold < 0 || c.Threshold > 100 {
		return fmt.Errorf("开口阈值需在 0..100 之间")
	}
	for _, w := range []float64{c.Weights.Silence, c.Weights.Topic, c.Weights.Activity, c.Weights.Night} {
		if w < 0 || w > 100 {
			return fmt.Errorf("打分权重需在 0..100 之间")
		}
	}
	return nil
}

// yyProactiveSaveLocked 原子落盘。调用方需持有 yyProactive.Mutex。
func yyProactiveSaveLocked() {
	if yyProactive.Path == "" {
		return
	}
	groups := make(map[string]*yyProactiveGroupState, len(yyProactive.Groups))
	for name, g := range yyProactive.Groups {
		groups[name] = g.State
	}
	raw, e := json.Marshal(yyProactiveFile{Config: yyProactive.Config, Groups: groups})
	if e == nil && len(raw) > 1<<20 {
		e = fmt.Errorf("proactive_state_too_large")
	}
	if e == nil {
		e = os.WriteFile(yyProactive.Path+".tmp", raw, 0600)
	}
	if e == nil {
		e = os.Rename(yyProactive.Path+".tmp", yyProactive.Path)
	}
	if e != nil {
		yyProactive.DiskError = "proactive_persistence_failed"
		return
	}
	yyProactive.DiskError = ""
}

// yyProactiveGroupLocked 取（或建）一个群的运行时状态。调用方需持有锁。
func yyProactiveGroupLocked(group string) *yyProactiveGroup {
	g, ok := yyProactive.Groups[group]
	if !ok {
		g = &yyProactiveGroup{State: &yyProactiveGroupState{}}
		yyProactive.Groups[group] = g
	}
	return g
}

// sweepLocked 丢弃窗口外的消息并限制单群缓存条数。调用方需持有锁。
func (g *yyProactiveGroup) sweepLocked(now time.Time) {
	cutoff := now.Add(-yyProactiveWindow)
	kept := make([]yyProactiveMsg, 0, len(g.Msgs))
	for _, m := range g.Msgs {
		if !m.at.Before(cutoff) {
			kept = append(kept, m)
		}
	}
	g.Msgs = kept
	if len(g.Msgs) > yyProactiveMaxMsgs {
		g.Msgs = append([]yyProactiveMsg(nil), g.Msgs[len(g.Msgs)-yyProactiveMaxMsgs:]...)
	}
}

// yyProactiveOnMessage 主动行为的单一切入点：每条群消息更新该群活跃状态
// （窗口内消息数、最后消息时间、话题词）。机器人自身回声、私聊、空号不计。
func yyProactiveOnMessage(m *yyQQInbound) {
	if m == nil || m.Group == "" || m.IsPrivate || m.User == "" {
		return
	}
	if m.User == yyQQSnapshot().OneBot.BotID {
		return
	}
	now := time.Now()
	yyProactive.Lock()
	defer yyProactive.Unlock()
	g := yyProactiveGroupLocked(m.Group)
	g.LastAt = now
	g.Msgs = append(g.Msgs, yyProactiveMsg{at: now, words: yyProactiveWords(m.Text)})
	g.sweepLocked(now)
}

// yyProactiveWords 提取话题词：CJK 连续段取二字滑窗，连续字母数字整体成词。
// 先剥掉 CQ 码与开头 @，避免把 [CQ:image] 当成话题。
func yyProactiveWords(text string) []string {
	text = yyQQLeadingMention.ReplaceAllString(text, "")
	for {
		start := strings.Index(text, "[CQ:")
		if start < 0 {
			break
		}
		end := strings.IndexByte(text[start:], ']')
		if end < 0 {
			text = text[:start]
			break
		}
		text = text[:start] + " " + text[start+end+1:]
	}
	var out []string
	var latin strings.Builder
	var cjk []rune
	flushLatin := func() {
		if latin.Len() >= 3 {
			out = append(out, strings.ToLower(latin.String()))
		}
		latin.Reset()
	}
	flushCJK := func() {
		for i := 0; i+1 < len(cjk); i++ {
			out = append(out, string(cjk[i:i+2]))
		}
		cjk = cjk[:0]
	}
	for _, r := range text {
		switch {
		case r < 0x80 && ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')):
			flushCJK()
			latin.WriteRune(r)
		case r >= 0x4E00 && r <= 0x9FFF:
			flushLatin()
			cjk = append(cjk, r)
		default:
			flushCJK()
			flushLatin()
		}
	}
	flushCJK()
	flushLatin()
	return out
}

// yyProactiveScore 是一次打分的分量明细，供测试与面板预览使用。
type yyProactiveScore struct {
	Total    float64  `json:"total"`
	Silence  float64  `json:"silence"`
	Topic    float64  `json:"topic"`
	Activity float64  `json:"activity"` // <=0，群吵惩罚
	Night    float64  `json:"night"`    // <=0，深夜惩罚
	Hot      []string `json:"hot_topics"`
	Hint     string   `json:"hint"`
	Parts    []string `json:"parts"`
}

func yyProactiveClamp(v, limit float64) float64 {
	if v < 0 {
		return 0
	}
	if v > limit {
		return limit
	}
	return v
}

// yyProactiveTopicsLocked 统计窗口内热点话题：被 >=2 条消息提到的词算热点，
// 返回前 3 个热点词与提到热点的消息条数。调用方需持有锁。
func yyProactiveTopicsLocked(g *yyProactiveGroup) ([]string, int) {
	counts := map[string]int{}
	for _, m := range g.Msgs {
		seen := map[string]bool{}
		for _, w := range m.words {
			if seen[w] {
				continue
			}
			seen[w] = true
			counts[w]++
		}
	}
	hot := make([]string, 0, 4)
	for w, n := range counts {
		if n >= 2 {
			hot = append(hot, w)
		}
	}
	sort.Slice(hot, func(i, j int) bool {
		if counts[hot[i]] == counts[hot[j]] {
			return hot[i] < hot[j]
		}
		return counts[hot[i]] > counts[hot[j]]
	})
	if len(hot) > 3 {
		hot = hot[:3]
	}
	hotSet := make(map[string]bool, len(hot))
	for _, w := range hot {
		hotSet[w] = true
	}
	topicMsgs := 0
	for _, m := range g.Msgs {
		for _, w := range m.words {
			if hotSet[w] {
				topicMsgs++
				break
			}
		}
	}
	return hot, topicMsgs
}

// yyProactiveScoreLocked 按四个维度打分。调用方需持有 yyProactive.Mutex。
func yyProactiveScoreLocked(cfg yyProactiveConfig, g *yyProactiveGroup, now time.Time) yyProactiveScore {
	sc := yyProactiveScore{Hot: []string{}}
	silence := now.Sub(g.LastAt)
	if silence < 0 {
		silence = 0
	}
	if cfg.MinSilenceSec > 0 && silence >= time.Duration(cfg.MinSilenceSec)*time.Second {
		sc.Silence = cfg.Weights.Silence * yyProactiveClamp(float64(silence)/float64(time.Duration(cfg.MinSilenceSec)*time.Second), yyProactiveSilenceCap)
		sc.Parts = append(sc.Parts, fmt.Sprintf("静默 %.0f 秒 +%.2f", silence.Seconds(), sc.Silence))
	} else {
		sc.Parts = append(sc.Parts, fmt.Sprintf("静默 %.0f 秒未满 %d 秒下限，静默分不计", silence.Seconds(), cfg.MinSilenceSec))
	}
	hot, topicMsgs := yyProactiveTopicsLocked(g)
	sc.Hot = hot
	if topicMsgs > 0 && cfg.Weights.Topic > 0 {
		sc.Topic = cfg.Weights.Topic * yyProactiveClamp(float64(topicMsgs)/yyProactiveTopicCap, 1)
		sc.Parts = append(sc.Parts, fmt.Sprintf("%d 条消息提到热点话题 +%.2f", topicMsgs, sc.Topic))
	}
	if n := len(g.Msgs); n > yyProactiveBusy && cfg.Weights.Activity > 0 {
		sc.Activity = -cfg.Weights.Activity * yyProactiveClamp(float64(n-yyProactiveBusy)/float64(yyProactiveBusy), 1)
		sc.Parts = append(sc.Parts, fmt.Sprintf("窗口内 %d 条消息较吵 %.2f", n, sc.Activity))
	}
	if h := now.Hour(); h >= yyProactiveNightStart || h < yyProactiveNightEnd {
		sc.Night = -cfg.Weights.Night
		sc.Parts = append(sc.Parts, fmt.Sprintf("深夜 %d 点 %.2f", h, sc.Night))
	}
	sc.Total = sc.Silence + sc.Topic + sc.Activity + sc.Night
	if len(sc.Hot) > 0 {
		sc.Hint = "群里最近在聊：" + strings.Join(sc.Hot, "、") + "。可以就着话题自然接一句，别复读原句。"
	} else {
		sc.Hint = "群里近期没有明显热点话题，可以随口聊一句或保持沉默。"
	}
	return sc
}

// yyProactiveJudgeLocked 打分并做三条硬约束判定，可选认领本次发言。
// 调用方需持有 yyProactive.Mutex。claim=true 且 speak=true 时写入
// last_proactive_ts、daily_count++ 并落盘，防止并发重复开口。
func yyProactiveJudgeLocked(group string, now time.Time, claim bool) (bool, string, string, yyProactiveScore) {
	cfg := yyProactive.Config
	if !cfg.Enabled {
		return false, "主动行为未启用", "", yyProactiveScore{}
	}
	g, ok := yyProactive.Groups[group]
	if !ok || g.LastAt.IsZero() {
		return false, "该群近期没有消息", "", yyProactiveScore{}
	}
	g.sweepLocked(now)
	if cfg.CooldownSec > 0 && g.State.LastProactiveTS > 0 {
		elapsed := now.Unix() - g.State.LastProactiveTS
		if elapsed < int64(cfg.CooldownSec) {
			return false, fmt.Sprintf("冷却中，距上次主动发言 %d 秒，需间隔 %d 秒", elapsed, cfg.CooldownSec), "", yyProactiveScore{}
		}
	}
	day := now.Format("2006-01-02")
	if g.State.DailyDate != day {
		g.State.DailyDate = day
		g.State.DailyCount = 0
	}
	if cfg.DailyCap > 0 && g.State.DailyCount >= cfg.DailyCap {
		return false, fmt.Sprintf("今日主动发言已达上限 %d 条", cfg.DailyCap), "", yyProactiveScore{}
	}
	sc := yyProactiveScoreLocked(cfg, g, now)
	if sc.Total < cfg.Threshold {
		return false, fmt.Sprintf("总分 %.2f 低于阈值 %.2f：%s", sc.Total, cfg.Threshold, strings.Join(sc.Parts, "，")), sc.Hint, sc
	}
	if claim {
		g.State.LastProactiveTS = now.Unix()
		g.State.DailyCount++
		yyProactiveSaveLocked()
	}
	return true, fmt.Sprintf("总分 %.2f 达到阈值 %.2f：%s", sc.Total, cfg.Threshold, strings.Join(sc.Parts, "，")), sc.Hint, sc
}

// yyProactiveDecide 对单个群做主动发言决策（使用当前时间）。
// speak=true 时同时认领本次发言并落盘；reason/hint 供调用方决定怎么开口。
func yyProactiveDecide(group string) (speak bool, reason string, hint string) {
	yyProactive.Lock()
	defer yyProactive.Unlock()
	speak, reason, hint, _ = yyProactiveJudgeLocked(group, time.Now(), true)
	return speak, reason, hint
}

// yyProactivePrompt 是一次主动发言机会：group 是要发言的群，
// reason 是为什么开口，hint 是给内容生成环节的提示。
type yyProactivePrompt struct {
	Group  string `json:"group"`
	Reason string `json:"reason"`
	Hint   string `json:"hint"`
}

// yyProactiveTick 供定时器调用：对所有仍有活跃记录且已启用的群打分，
// 返回想主动发言的群与提示，由调用方生成内容并发送。
func yyProactiveTick(now time.Time) []yyProactivePrompt {
	cf := yyQQSnapshot()
	yyProactive.Lock()
	defer yyProactive.Unlock()
	var out []yyProactivePrompt
	for group, g := range yyProactive.Groups {
		g.sweepLocked(now)
		if g.LastAt.IsZero() || !yyQQGroupEnabled(cf, group) {
			continue
		}
		if speak, reason, hint, _ := yyProactiveJudgeLocked(group, now, true); speak {
			out = append(out, yyProactivePrompt{Group: group, Reason: reason, Hint: hint})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Group < out[j].Group })
	return out
}

// yyProactiveRoutes 注册 GET/POST /api/qq/proactive 并加载持久化状态。
// GET 返回配置与各群统计，?group=xxx 时附带一次不认领的预览打分；
// POST 接受 {"config":{...}} 整体更新配置。
func yyProactiveRoutes(mux *http.ServeMux, dataDir string) {
	yyProactiveInit(dataDir)
	mux.HandleFunc("/api/qq/proactive", func(w http.ResponseWriter, r *http.Request) {
		group := strings.TrimSpace(r.URL.Query().Get("group"))
		switch r.Method {
		case http.MethodGet:
			now := time.Now()
			yyProactive.Lock()
			rows := make([]map[string]any, 0, len(yyProactive.Groups))
			for name, g := range yyProactive.Groups {
				g.sweepLocked(now)
				hot, _ := yyProactiveTopicsLocked(g)
				rows = append(rows, map[string]any{
					"group":             name,
					"window_messages":   len(g.Msgs),
					"silence_sec":       int(now.Sub(g.LastAt).Seconds()),
					"hot_topics":        hot,
					"last_proactive_ts": g.State.LastProactiveTS,
					"daily_count":       g.State.DailyCount,
					"daily_date":        g.State.DailyDate,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				a, _ := rows[i]["group"].(string)
				b, _ := rows[j]["group"].(string)
				return a < b
			})
			resp := map[string]any{
				"ok":                true,
				"config":            yyProactive.Config,
				"groups":            rows,
				"persistence_error": yyProactive.DiskError,
			}
			if group != "" {
				speak, reason, hint, sc := yyProactiveJudgeLocked(group, now, false)
				resp["group"] = group
				resp["preview"] = map[string]any{
					"speak":  speak,
					"reason": reason,
					"hint":   hint,
					"score":  sc,
				}
			}
			yyProactive.Unlock()
			writeJSON(w, resp)
		case http.MethodPost:
			var payload struct {
				Config yyProactiveConfig `json:"config"`
			}
			if e := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&payload); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			if e := yyProactiveValidate(payload.Config); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			yyProactive.Lock()
			yyProactive.Config = payload.Config
			yyProactiveSaveLocked()
			diskErr := yyProactive.DiskError
			yyProactive.Unlock()
			writeJSON(w, map[string]any{"ok": diskErr == "", "config": payload.Config, "persistence_error": diskErr})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
		}
	})
}
