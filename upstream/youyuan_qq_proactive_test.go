package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// yyProactiveTestInit 用临时目录重置主动行为状态。
func yyProactiveTestInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	yyProactiveInit(dir)
	return dir
}

// yyProactiveTestConfig 设置确定性配置，默认关掉与用例无关的分量。
func yyProactiveTestConfig(t *testing.T, cfg yyProactiveConfig) {
	t.Helper()
	yyProactive.Lock()
	defer yyProactive.Unlock()
	yyProactive.Config = cfg
}

// yyProactiveTestInject 直接注入一条活跃消息，时间与话题词完全可控。
func yyProactiveTestInject(group string, at time.Time, words []string) {
	yyProactive.Lock()
	defer yyProactive.Unlock()
	g := yyProactiveGroupLocked(group)
	if g.LastAt.IsZero() || at.After(g.LastAt) {
		g.LastAt = at
	}
	g.Msgs = append(g.Msgs, yyProactiveMsg{at: at, words: words})
}

// 静默时长与打分离调单调：未满下限不计分，越久越高，3 倍封顶。
func TestProactiveSilenceScoreMonotonic(t *testing.T) {
	yyProactiveTestInit(t)
	yyProactiveTestConfig(t, yyProactiveConfig{
		Enabled: true, MinSilenceSec: 100, CooldownSec: 0, DailyCap: 0, Threshold: 1000,
		Weights: yyProactiveWeights{Silence: 1.0},
	})
	base := time.Now()
	yyProactiveTestInject("1001", base, nil)
	want := []float64{0, 0, 1.0, 1.5, 2.0, 3.0, 3.0}
	deltas := []time.Duration{0, 99 * time.Second, 100 * time.Second, 150 * time.Second, 200 * time.Second, 300 * time.Second, 900 * time.Second}
	for i, d := range deltas {
		yyProactive.Lock()
		sc := yyProactiveScoreLocked(yyProactive.Config, yyProactiveGroupLocked("1001"), base.Add(d))
		yyProactive.Unlock()
		if sc.Silence != want[i] {
			t.Fatalf("沉默 %v 后期望静默分 %.2f，实际 %.2f", d, want[i], sc.Silence)
		}
		if sc.Total != want[i] {
			t.Fatalf("沉默 %v 后期望总分 %.2f，实际 %.2f", d, want[i], sc.Total)
		}
	}
}

// 冷却是硬约束：冷却内直接 false，冷却结束才恢复放行。
func TestProactiveCooldownHardBlock(t *testing.T) {
	yyProactiveTestInit(t)
	yyProactiveTestConfig(t, yyProactiveConfig{
		Enabled: true, MinSilenceSec: 10, CooldownSec: 3600, DailyCap: 10, Threshold: 0.1,
		Weights: yyProactiveWeights{Silence: 1.0},
	})
	// The cooldown case must stay within one day, including late-night test runs.
	base := time.Date(2026, 10, 23, 12, 0, 0, 0, time.Local)
	yyProactiveTestInject("1002", base, nil)
	yyProactive.Lock()
	speak, reason, _, _ := yyProactiveJudgeLocked("1002", base.Add(11*time.Second), true)
	yyProactive.Unlock()
	if !speak {
		t.Fatalf("首次应放行，实际被拦：%s", reason)
	}
	yyProactive.Lock()
	speak, reason, _, _ = yyProactiveJudgeLocked("1002", base.Add(20*time.Second), true)
	yyProactive.Unlock()
	if speak || !strings.Contains(reason, "冷却") {
		t.Fatalf("冷却内应硬拦且原因含冷却，实际 speak=%v reason=%s", speak, reason)
	}
	yyProactive.Lock()
	speak, reason, _, _ = yyProactiveJudgeLocked("1002", base.Add(3700*time.Second), true)
	st := yyProactiveGroupLocked("1002").State
	yyProactive.Unlock()
	if !speak {
		t.Fatalf("冷却结束后应放行，实际被拦：%s", reason)
	}
	if st.DailyCount != 2 {
		t.Fatalf("两次放行后日计数应为 2，实际 %d", st.DailyCount)
	}
}

// 日上限是硬约束：当天达上限拦截，跨天自动重置。
func TestProactiveDailyCapResetsAcrossDays(t *testing.T) {
	yyProactiveTestInit(t)
	yyProactiveTestConfig(t, yyProactiveConfig{
		Enabled: true, MinSilenceSec: 10, CooldownSec: 0, DailyCap: 2, Threshold: 0.1,
		Weights: yyProactiveWeights{Silence: 1.0},
	})
	base := time.Now()
	yyProactiveTestInject("1003", base, nil)
	decide := func(at time.Time) (bool, string) {
		yyProactive.Lock()
		defer yyProactive.Unlock()
		speak, reason, _, _ := yyProactiveJudgeLocked("1003", at, true)
		return speak, reason
	}
	if speak, reason := decide(base.Add(11 * time.Second)); !speak {
		t.Fatalf("第 1 条应放行：%s", reason)
	}
	if speak, reason := decide(base.Add(12 * time.Second)); !speak {
		t.Fatalf("第 2 条应放行：%s", reason)
	}
	if speak, reason := decide(base.Add(13 * time.Second)); speak || !strings.Contains(reason, "上限") {
		t.Fatalf("达上限应硬拦，实际 speak=%v reason=%s", speak, reason)
	}
	// 跨天：窗口内消息已扫光，静默分仍按最后消息时间计算，计数应重置。
	if speak, reason := decide(base.Add(25 * time.Hour)); !speak {
		t.Fatalf("跨天后应重置并放行：%s", reason)
	}
	yyProactive.Lock()
	st := yyProactiveGroupLocked("1003").State
	yyProactive.Unlock()
	if st.DailyCount != 1 || st.DailyDate != base.Add(25*time.Hour).Format("2006-01-02") {
		t.Fatalf("跨天后计数应为 1 且日期翻篇，实际 count=%d date=%s", st.DailyCount, st.DailyDate)
	}
}

// 阈值边界：总分恰好等于阈值放行（>= 语义），差一点拦截。
func TestProactiveThresholdBoundary(t *testing.T) {
	yyProactiveTestInit(t)
	yyProactiveTestConfig(t, yyProactiveConfig{
		Enabled: true, MinSilenceSec: 100, CooldownSec: 0, DailyCap: 0, Threshold: 1.5,
		Weights: yyProactiveWeights{Silence: 1.0},
	})
	base := time.Now()
	yyProactiveTestInject("1004", base, nil)
	yyProactive.Lock()
	speak, reason, _, _ := yyProactiveJudgeLocked("1004", base.Add(150*time.Second), true)
	yyProactive.Unlock()
	if !speak {
		t.Fatalf("总分恰好等于阈值时应放行，实际被拦：%s", reason)
	}
	yyProactive.Lock()
	speak, _, _, _ = yyProactiveJudgeLocked("1004", base.Add(149*time.Second), true)
	yyProactive.Unlock()
	if speak {
		t.Fatal("总分低于阈值时应拦截")
	}
}

// 持久化 round-trip：认领发言后重新加载，配置与每群状态原样回来。
func TestProactivePersistenceRoundTrip(t *testing.T) {
	dir := yyProactiveTestInit(t)
	cfg := yyProactiveConfig{
		Enabled: true, MinSilenceSec: 42, CooldownSec: 43, DailyCap: 7, Threshold: 0.5,
		Weights: yyProactiveWeights{Silence: 1.1, Topic: 2.2, Activity: 3.3, Night: 4.4},
	}
	yyProactiveTestConfig(t, cfg)
	// 固定成下午 2 点：原来用 time.Now()，深夜跑会被「深夜权重」压到阈值以下而误报，
	// 这是测试依赖真实时钟的 bug，不是功能问题。
	base := time.Date(2026, 1, 1, 14, 0, 0, 0, time.Local)
	yyProactiveTestInject("1005", base, nil)
	yyProactive.Lock()
	speak, reason, _, _ := yyProactiveJudgeLocked("1005", base.Add(60*time.Second), true)
	before := *yyProactiveGroupLocked("1005").State
	yyProactive.Unlock()
	if !speak {
		t.Fatalf("应放行一次以便落盘：%s", reason)
	}
	// 关停后保存，验证 Enabled=false 也能 round-trip。
	yyProactive.Lock()
	yyProactive.Config.Enabled = false
	yyProactiveSaveLocked()
	yyProactive.Unlock()
	if _, e := os.Stat(filepath.Join(dir, "qq-proactive.json")); e != nil {
		t.Fatalf("qq-proactive.json 应已落盘：%v", e)
	}
	yyProactiveInit(dir)
	yyProactive.Lock()
	gotCfg := yyProactive.Config
	after := *yyProactiveGroupLocked("1005").State
	yyProactive.Unlock()
	want := cfg
	want.Enabled = false
	if gotCfg != want {
		t.Fatalf("配置 round-trip 不一致：%+v", gotCfg)
	}
	if after != before {
		t.Fatalf("每群状态 round-trip 不一致：%+v vs %+v", after, before)
	}
}

// 深夜降权：23 点到次日 7 点扣 Night 权重，同状态白天不扣。
func TestProactiveNightPenalty(t *testing.T) {
	yyProactiveTestInit(t)
	yyProactiveTestConfig(t, yyProactiveConfig{
		Enabled: true, MinSilenceSec: 10, CooldownSec: 0, DailyCap: 0, Threshold: 1000,
		Weights: yyProactiveWeights{Silence: 1.0, Night: 1.5},
	})
	day := time.Date(2026, 10, 22, 3, 0, 0, 0, time.Local)
	yyProactiveTestInject("1006", day, nil)
	yyProactive.Lock()
	night := yyProactiveScoreLocked(yyProactive.Config, yyProactiveGroupLocked("1006"), day.Add(30*time.Minute))
	noon := yyProactiveScoreLocked(yyProactive.Config, yyProactiveGroupLocked("1006"), time.Date(2026, 10, 22, 12, 0, 0, 0, time.Local))
	yyProactive.Unlock()
	if night.Night != -1.5 {
		t.Fatalf("深夜惩罚应为 -1.5，实际 %.2f", night.Night)
	}
	if noon.Night != 0 {
		t.Fatalf("白天不应有深夜惩罚，实际 %.2f", noon.Night)
	}
	if night.Total != noon.Total-1.5 {
		t.Fatalf("深夜应比白天低 1.5，实际深夜 %.2f 白天 %.2f", night.Total, noon.Total)
	}
}

// 话题分与活跃度惩罚：热点话题提分，群吵扣分，两者互不干扰。
func TestProactiveTopicAndActivityParts(t *testing.T) {
	yyProactiveTestInit(t)
	yyProactiveTestConfig(t, yyProactiveConfig{
		Enabled: true, MinSilenceSec: 3600, CooldownSec: 0, DailyCap: 0, Threshold: 1000,
		Weights: yyProactiveWeights{Silence: 1.0, Topic: 0.6, Activity: 0.8},
	})
	base := time.Now()
	yyProactiveTestInject("1007", base, []string{"开服", "炸服"})
	yyProactiveTestInject("1007", base, []string{"开服", "晚上"})
	yyProactiveTestInject("1007", base, []string{"别的"})
	for i := 0; i < yyProactiveBusy+3; i++ {
		yyProactiveTestInject("1007", base, nil)
	}
	yyProactive.Lock()
	sc := yyProactiveScoreLocked(yyProactive.Config, yyProactiveGroupLocked("1007"), base.Add(time.Second))
	yyProactive.Unlock()
	if sc.Silence != 0 {
		t.Fatalf("静默不满下限不应有静默分，实际 %.2f", sc.Silence)
	}
	if sc.Topic != 0.6 {
		t.Fatalf("两条消息提到热点话题应拿满话题分 0.6，实际 %.2f", sc.Topic)
	}
	if sc.Activity >= 0 {
		t.Fatalf("群吵应有活跃度惩罚，实际 %.2f", sc.Activity)
	}
	found := false
	for _, w := range sc.Hot {
		if w == "开服" {
			found = true
		}
	}
	if !found {
		t.Fatalf("热点话题应包含开服，实际 %v", sc.Hot)
	}
}

// OnMessage 只记录真实群消息：机器人回声、私聊、空群号都忽略。
func TestProactiveOnMessageTracksGroupState(t *testing.T) {
	yyProactiveTestInit(t)
	yyQQMu.Lock()
	oldBot := yyQQ.OneBot.BotID
	yyQQ.OneBot.BotID = "999"
	yyQQMu.Unlock()
	defer func() {
		yyQQMu.Lock()
		yyQQ.OneBot.BotID = oldBot
		yyQQMu.Unlock()
	}()
	yyProactiveOnMessage(&yyQQInbound{Group: "2001", User: "100", Text: "今天服务器又炸了"})
	yyProactiveOnMessage(&yyQQInbound{Group: "2001", User: "999", Text: "机器人自己的回声"})
	yyProactiveOnMessage(&yyQQInbound{Group: "", User: "100", Text: "没有群号"})
	yyProactiveOnMessage(&yyQQInbound{Group: "2001", User: "100", Text: "私聊不算", IsPrivate: true})
	yyProactive.Lock()
	g, ok := yyProactive.Groups["2001"]
	msgs := 0
	if ok {
		msgs = len(g.Msgs)
	}
	yyProactive.Unlock()
	if !ok || msgs != 1 {
		t.Fatalf("只应记录 1 条真实群消息，实际 ok=%v msgs=%d", ok, msgs)
	}
	yyProactive.Lock()
	g, _ = yyProactive.Groups["2001"]
	hasWord := false
	for _, w := range g.Msgs[0].words {
		if w == "服务" {
			hasWord = true
		}
	}
	yyProactive.Unlock()
	if !hasWord {
		t.Fatal("话题词提取应包含服务")
	}
}

// 未启用时一律拦截，即使静默再久、阈值再低。
func TestProactiveDisabledNeverSpeaks(t *testing.T) {
	yyProactiveTestInit(t)
	yyProactiveTestConfig(t, yyProactiveConfig{
		Enabled: false, MinSilenceSec: 1, CooldownSec: 0, DailyCap: 0, Threshold: 0.01,
		Weights: yyProactiveWeights{Silence: 10},
	})
	yyProactiveTestInject("2002", time.Now(), nil)
	yyProactive.Lock()
	speak, reason, _, _ := yyProactiveJudgeLocked("2002", time.Now().Add(time.Hour), true)
	yyProactive.Unlock()
	if speak || !strings.Contains(reason, "未启用") {
		t.Fatalf("未启用时应拦截，实际 speak=%v reason=%s", speak, reason)
	}
}

// 公开放行接口：返回 speak 的同时给出原因与提示。
func TestProactiveDecidePublicAPI(t *testing.T) {
	yyProactiveTestInit(t)
	yyProactiveTestConfig(t, yyProactiveConfig{
		Enabled: true, MinSilenceSec: 1, CooldownSec: 0, DailyCap: 0, Threshold: 0.01,
		Weights: yyProactiveWeights{Silence: 1.0},
	})
	yyProactiveTestInject("4001", time.Now().Add(-time.Hour), nil)
	speak, reason, hint := yyProactiveDecide("4001")
	if !speak || reason == "" || hint == "" {
		t.Fatalf("公开放行应返回原因与提示，实际 speak=%v reason=%q hint=%q", speak, reason, hint)
	}
}

// Tick 汇总所有已启用且想发言的群，按群号排序。
func TestProactiveTickReturnsSpeakableGroups(t *testing.T) {
	yyProactiveTestInit(t)
	yyProactiveTestConfig(t, yyProactiveConfig{
		Enabled: true, MinSilenceSec: 10, CooldownSec: 0, DailyCap: 5, Threshold: 0.1,
		Weights: yyProactiveWeights{Silence: 1.0},
	})
	base := time.Now()
	yyProactiveTestInject("3002", base, nil)
	yyProactiveTestInject("3001", base, nil)
	yyQQMu.Lock()
	oldGroups := yyQQ.Groups
	yyQQ.Groups = []yyQQGroup{{ID: "3001", Enabled: true}, {ID: "3002", Enabled: true}}
	yyQQMu.Unlock()
	defer func() {
		yyQQMu.Lock()
		yyQQ.Groups = oldGroups
		yyQQMu.Unlock()
	}()
	prompts := yyProactiveTick(base.Add(20 * time.Second))
	if len(prompts) != 2 {
		t.Fatalf("两个群都应想发言，实际 %d 条：%+v", len(prompts), prompts)
	}
	if prompts[0].Group != "3001" || prompts[1].Group != "3002" {
		t.Fatalf("应按群号排序，实际 %+v", prompts)
	}
	if prompts[0].Hint == "" || prompts[0].Reason == "" {
		t.Fatal("提示与原因都不应为空")
	}
}

// HTTP 接口 round-trip：GET 默认配置、POST 更新、落盘重载、非法拒绝。
func TestProactiveAPIConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	mux := http.NewServeMux()
	yyProactiveRoutes(mux, dir)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/qq/proactive", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET 应 200，实际 %d", rec.Code)
	}
	var get struct {
		OK     bool              `json:"ok"`
		Config yyProactiveConfig `json:"config"`
	}
	if e := json.NewDecoder(rec.Body).Decode(&get); e != nil {
		t.Fatalf("GET 响应解析失败：%v", e)
	}
	if !get.OK || get.Config.MinSilenceSec != 900 || get.Config.CooldownSec != 1800 {
		t.Fatalf("GET 默认配置异常：%+v", get.Config)
	}
	body := `{"config":{"enabled":false,"min_silence_sec":120,"cooldown_sec":600,"daily_cap":3,"threshold":2.5,"weights":{"silence":1.5,"topic":0.5,"activity":0.7,"night":1.0}}}`
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/qq/proactive", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST 应 200，实际 %d", rec.Code)
	}
	var post struct {
		OK     bool              `json:"ok"`
		Config yyProactiveConfig `json:"config"`
	}
	if e := json.NewDecoder(rec.Body).Decode(&post); e != nil {
		t.Fatalf("POST 响应解析失败：%v", e)
	}
	if !post.OK || post.Config.Enabled || post.Config.MinSilenceSec != 120 || post.Config.Threshold != 2.5 || post.Config.Weights.Night != 1.0 {
		t.Fatalf("POST 回显配置异常：%+v", post.Config)
	}
	yyProactiveInit(dir)
	yyProactive.Lock()
	got := yyProactive.Config
	yyProactive.Unlock()
	if got.Enabled || got.MinSilenceSec != 120 || got.CooldownSec != 600 || got.DailyCap != 3 || got.Threshold != 2.5 || got.Weights.Silence != 1.5 {
		t.Fatalf("落盘后配置不一致：%+v", got)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/qq/proactive", strings.NewReader(`{"config":{"daily_cap":-1}}`)))
	var bad struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&bad)
	if bad.OK || bad.Error == "" {
		t.Fatalf("非法配置应被拒绝并给出原因：%+v", bad)
	}
}
