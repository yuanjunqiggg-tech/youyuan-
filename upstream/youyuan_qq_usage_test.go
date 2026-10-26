package main

// usage 测试：bbolt 用 t.TempDir()，不发 QQ 消息、不连真实模型。
// fixture（yyUsageFixture/yyRagUsageGlobals）复用 youyuan_qq_rag_test.go。

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestYYUsageRealAndEstimatedSplitAndPersist Real=true/false 分流：totals
// 只含 provider 真值，估算只进 estimated_calls；重启后聚合与 recent 仍在。
func TestYYUsageRealAndEstimatedSplitAndPersist(t *testing.T) {
	cf, dir := yyUsageFixture(t)
	yyUsageInit(dir)
	yyUsageAddAt(cf, "u1", "chat", "chat", aiTurn{Role: "user", Text: "你好呀", InputTokens: 100, OutputTokens: 20}, time.Now())
	// 网关不返回 usage（InputTokens==0）→ 本地估算，Real=false
	yyUsageAddAt(cf, "u2", "chat", "chat", aiTurn{Role: "user", Text: "另一条没有 usage 的消息"}, time.Now())
	yyUsageFlush()
	q := yyUsageQuery(cf, "", "", "")
	totals := q["totals"].(map[string]any)
	if totals["input"].(int) != 100 || totals["output"].(int) != 20 || totals["real_calls"].(int) != 1 || totals["estimated_calls"].(int) != 1 {
		t.Fatal("真值/估算分流不对", totals)
	}
	recent := q["recent"].([]yyUsageRecord)
	if len(recent) != 2 || recent[0].Real != false || recent[0].InputTokens == 0 {
		t.Fatal("估算记录不应混入真值", recent)
	}
	if recent[1].Real != true || recent[1].InputTokens != 100 {
		t.Fatal("真值记录丢失", recent[1])
	}
	// 重启：聚合、估算计数、recent 都从 qq-usage.db 读回
	yyUsageClose()
	yyUsageInit(dir)
	q = yyUsageQuery(cf, "", "", "")
	totals = q["totals"].(map[string]any)
	if totals["input"].(int) != 100 || totals["real_calls"].(int) != 1 || totals["estimated_calls"].(int) != 1 {
		t.Fatal("重启后聚合丢失", totals)
	}
	if recent = q["recent"].([]yyUsageRecord); len(recent) != 2 {
		t.Fatal("重启后 recent 丢失", len(recent))
	}
}

// TestYYUsageDayAggregationRingAndFlushThreshold 按天聚合、from/to 过滤、
// recent 环形 500 上限、32 条阈值自动 flush、重启后环不丢。
func TestYYUsageDayAggregationRingAndFlushThreshold(t *testing.T) {
	cf, dir := yyUsageFixture(t)
	yyUsageInit(dir)
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	yyUsageAddAt(cf, "u1", "chat", "chat", aiTurn{Role: "user", Text: "甲", InputTokens: 10, OutputTokens: 2}, now)
	yyUsageAddAt(cf, "u1", "chat", "chat", aiTurn{Role: "user", Text: "乙", InputTokens: 20, OutputTokens: 4}, now)
	yyUsageAddAt(cf, "u1", "chat", "chat", aiTurn{Role: "user", Text: "丙", InputTokens: 40, OutputTokens: 8}, yesterday)
	q := yyUsageQuery(cf, "", "", "")
	series := q["series"].([]map[string]any)
	if len(series) != 2 {
		t.Fatal("应按天聚合出 2 天", series)
	}
	if series[1]["day"] != now.Format("2006-01-02") || series[1]["input"].(int) != 30 || series[1]["calls"].(int) != 2 {
		t.Fatal("当天聚合不对", series[1])
	}
	today := now.Format("2006-01-02")
	if q = yyUsageQuery(cf, "", today, today); len(q["series"].([]map[string]any)) != 1 {
		t.Fatal("from/to 过滤失效")
	}
	// 520 条：内存 recent 截到 500，且 32 条阈值触发过自动 flush
	for i := 0; i < 520; i++ {
		yyUsageAddAt(cf, "u1", "chat", "chat", aiTurn{Role: "user", Text: "刷屏", InputTokens: 1, OutputTokens: 1}, now)
	}
	yyUsage.Lock()
	recentN, pendingN := len(yyUsage.Recent), len(yyUsage.Pending)
	yyUsage.Unlock()
	if recentN != 500 {
		t.Fatal("Recent 未限制到 500", recentN)
	}
	if pendingN >= 32 {
		t.Fatal("32 条阈值未自动 flush", pendingN)
	}
	// 重启后磁盘上的 recent 环仍是 500
	yyUsageClose()
	yyUsageInit(dir)
	yyUsage.Lock()
	recentN = len(yyUsage.Recent)
	yyUsage.Unlock()
	if recentN != 500 {
		t.Fatal("重启后 recent 环流失效", recentN)
	}
}

// TestYYUsageContextThreeLayersAndAnchorInvalidation 三源分层与锚点失效：
// local_estimate → provider_reported → calibrated_estimate →（system prompt
// 漂移）local_estimate → provider_reported →（超 30 分钟）local_estimate →
// （换模型）local_estimate。
func TestYYUsageContextThreeLayersAndAnchorInvalidation(t *testing.T) {
	cf, dir := yyUsageFixture(t)
	yyUsageInit(dir)
	source := func() string {
		return yyUsageContext(cf)["context"].(map[string]any)["source"].(string)
	}
	used := func() int {
		return yyUsageContext(cf)["context"].(map[string]any)["used_tokens"].(int)
	}
	if source() != "local_estimate" {
		t.Fatal("无锚点应为 local_estimate")
	}
	tools := yyQQAITools(false)
	observe := func(systemText string) {
		yyUsageObserve(cf, "chat", []aiTurn{{Role: "system", Text: systemText}, {Role: "user", Text: "今天天气怎么样"}}, tools, nil, 1000)
	}
	// 观测一次成功请求 → provider_reported（真值 1000）
	observe(yyQQUsageSystemPrompt(cf, false))
	if source() != "provider_reported" || used() != 1000 {
		t.Fatal("观测后应为 provider_reported 且 used=1000", source(), used())
	}
	// 群消息增多 → calibrated_estimate：used = round(curEst*1000/baseEst)
	yyQQMessages.Lock()
	yyQQMessages.Groups[yyQQMessageKey(cf)] = []yyQQGroupMessage{{ID: "m1", User: "u1", Group: "group-a", Adapter: "onebot", Bot: "fixture-bot", Text: "大家在吗", Timestamp: time.Now()}}
	yyQQMessages.Unlock()
	baseEst := yyEstimateContextTokens([]aiTurn{{Role: "system", Text: yyQQUsageSystemPrompt(cf, false)}, {Role: "user", Text: "今天天气怎么样"}}, tools)
	rows := yyQQRecentMessages(cf, 80)
	curTurns := []aiTurn{{Role: "system", Text: yyQQUsageSystemPrompt(cf, false)}, {Role: "user", Text: yyQQContextJSON(cf, rows, false, "received_group_events", yyQQContextBudget(cf)/2)}}
	curEst := aiStore.Get("chat").contextEstimate(curTurns, tools)
	want := int(math.Round(float64(curEst) * 1000 / float64(baseEst)))
	if source() != "calibrated_estimate" || used() != want {
		t.Fatal("校准估算不对", source(), used(), want)
	}
	// system prompt 变化 >20% → 锚点失效 → local_estimate
	observe(strings.Repeat("系", 3000))
	if source() != "local_estimate" {
		t.Fatal("system prompt 漂移未使锚点失效", source())
	}
	// 用真实 prompt 重新观测 → 恢复 provider_reported
	observe(yyQQUsageSystemPrompt(cf, false))
	if source() != "provider_reported" || used() != 1000 {
		t.Fatal("恢复观测失败", source(), used())
	}
	// 距上次成功 >30 分钟 → local_estimate
	yyUsage.Lock()
	yyUsage.Anchors[yyQQMessageKey(cf)+"\x00chat"].At = time.Now().Add(-31 * time.Minute)
	yyUsage.Unlock()
	if source() != "local_estimate" {
		t.Fatal("超 30 分钟未失效", source())
	}
	// 换模型（锚点键变为 scope+chat2，查无此锚）→ local_estimate
	aiStore.models = append(aiStore.models, &AIModel{Name: "chat2", Provider: ProviderOpenAI, ModelID: "fixture-chat2", BaseURL: "https://fixture.invalid", ContextWindow: 32768, MaxOutputTokens: 4096})
	cf.Assistant.Model = "chat2"
	out := yyUsageContext(cf)["context"].(map[string]any)
	if out["source"].(string) != "local_estimate" || out["window"].(int) != 32768 {
		t.Fatal("换模型未退回 local_estimate", out)
	}
}

// TestYYUsageRoutesPricingAndContextEndpoint 计价（微元）、/api/qq/usage、
// /api/qq/usage/context 与 405。
func TestYYUsageRoutesPricingAndContextEndpoint(t *testing.T) {
	cf, dir := yyUsageFixture(t)
	aiStore.models = []*AIModel{{Name: "chat", Provider: ProviderOpenAI, ModelID: "fixture-chat", BaseURL: "https://fixture.invalid", ContextWindow: 65536, MaxOutputTokens: 8192, InputPricePerMillion: 2, OutputPricePerMillion: 8}}
	mux := http.NewServeMux()
	yyQQUsageRoutes(mux, dir) // 内部自行 yyUsageInit(dir)，勿重复开库
	// 100*2 + 50*8 = 600 微元
	yyUsageAddAt(cf, "u1", "chat", "chat", aiTurn{Role: "user", Text: "计费测试", InputTokens: 100, OutputTokens: 50}, time.Now())
	q := yyUsageQuery(cf, "", "", "")
	totals := q["totals"].(map[string]any)
	if totals["cost_micro"].(int64) != 600 || q["pricing_configured"].(bool) != true {
		t.Fatal("计价不对", totals["cost_micro"], q["pricing_configured"])
	}
	call := func(method, path string) (int, map[string]any) {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out) // 405 等纯文本响应不解析，返回 nil
		return w.Code, out
	}
	code, usage := call(http.MethodGet, "/api/qq/usage")
	usageTotals := usage["totals"].(map[string]any)
	if code != http.StatusOK || usage["ok"] != true || usageTotals["input"].(float64) != 100 || usageTotals["estimated_calls"].(float64) != 0 || usageTotals["cost_micro"].(float64) != 600 {
		t.Fatal("/api/qq/usage 不对", code, usage)
	}
	code, ctxOut := call(http.MethodGet, "/api/qq/usage/context?group=group-a")
	contextOut := ctxOut["context"].(map[string]any)
	if code != http.StatusOK || ctxOut["ok"] != true || contextOut["source"].(string) != "local_estimate" || contextOut["window"].(float64) != 65536 {
		t.Fatal("/api/qq/usage/context 不对", code, ctxOut)
	}
	if code, _ = call(http.MethodPost, "/api/qq/usage"); code != http.StatusMethodNotAllowed {
		t.Fatal("usage 应只接受 GET", code)
	}
}

// TestYYUsageFlushFiveSecondThreshold 5 秒阈值：LastFlush 超过 5 秒后，
// 单条记账即触发 flush（不凑满 32 条），且数据确实落盘可重载。
func TestYYUsageFlushFiveSecondThreshold(t *testing.T) {
	cf, dir := yyUsageFixture(t)
	yyUsageInit(dir)
	yyUsage.Lock()
	yyUsage.LastFlush = time.Now().Add(-6 * time.Second)
	yyUsage.Unlock()
	yyUsageAddAt(cf, "u1", "chat", "chat", aiTurn{Role: "user", Text: "单条", InputTokens: 100, OutputTokens: 10}, time.Now())
	yyUsage.Lock()
	pending := len(yyUsage.Pending)
	yyUsage.Unlock()
	if pending != 0 {
		t.Fatal("5 秒阈值未触发 flush", pending)
	}
	yyUsageClose()
	yyUsageInit(dir)
	q := yyUsageQuery(cf, "", "", "")
	if q["totals"].(map[string]any)["input"].(int) != 100 {
		t.Fatal("5 秒阈值 flush 后数据未落盘", q["totals"])
	}
}

// TestYYUsageScopeIsolationAndByUser scope 隔离：不同群的记账互不串；
// by_user 只统计 provider 真值（估算调用不计入）；estimated_calls 按 scope 过滤。
func TestYYUsageScopeIsolationAndByUser(t *testing.T) {
	cf, dir := yyUsageFixture(t)
	yyUsageInit(dir)
	other := cf
	other.GroupID = "group-b"
	yyUsageAddAt(cf, "小明", "chat", "chat", aiTurn{Role: "user", Text: "真值甲", InputTokens: 100, OutputTokens: 20}, time.Now())
	yyUsageAddAt(cf, "小红", "chat", "chat", aiTurn{Role: "user", Text: "无 usage 的估算"}, time.Now())
	yyUsageAddAt(other, "小明", "chat", "chat", aiTurn{Role: "user", Text: "真值乙", InputTokens: 50, OutputTokens: 10}, time.Now())
	q := yyUsageQuery(cf, yyQQMessageKey(cf), "", "")
	totals := q["totals"].(map[string]any)
	if totals["input"].(int) != 100 || totals["real_calls"].(int) != 1 || totals["estimated_calls"].(int) != 1 {
		t.Fatal("group-a 聚合不对", totals)
	}
	users := q["by_user"].([]map[string]any)
	if len(users) != 1 || users[0]["user"] != "小明" || users[0]["calls"].(int) != 1 || users[0]["input"].(int) != 100 {
		t.Fatal("by_user 只应含本 scope 的真值记录", users)
	}
	if q = yyUsageQuery(cf, yyQQMessageKey(other), "", ""); q["totals"].(map[string]any)["input"].(int) != 50 {
		t.Fatal("group-b scope 隔离失效", q["totals"])
	}
	if q = yyUsageQuery(cf, "", "", ""); q["totals"].(map[string]any)["input"].(int) != 150 || q["totals"].(map[string]any)["real_calls"].(int) != 2 {
		t.Fatal("空 scope 应汇总全部群", q["totals"])
	}
}

// TestYYUsageAnchorToolsHashAndIdentityInvalidation 锚点失效的另外两条：
// 换工具集（tools_hash 不符）与身份变化（contextIdentity 不符）都必须退回
// local_estimate；两项都恢复后重新回到 provider_reported。
func TestYYUsageAnchorToolsHashAndIdentityInvalidation(t *testing.T) {
	cf, dir := yyUsageFixture(t)
	yyUsageInit(dir)
	systemText := yyQQUsageSystemPrompt(cf, false)
	key := yyQQMessageKey(cf) + "\x00chat"
	put := func(identity, toolsHash string) {
		yyUsage.Lock()
		yyUsage.Anchors[key] = &yyUsageAnchor{
			Scope: yyQQMessageKey(cf), Model: "chat", Identity: identity,
			InputTokens: 1000, BaseEstimate: 500, SystemChars: len([]rune(systemText)),
			ToolsHash: toolsHash, At: time.Now(),
		}
		yyUsage.DirtyAnchor[key] = true
		yyUsage.Unlock()
	}
	source := func() string {
		return yyUsageContext(cf)["context"].(map[string]any)["source"].(string)
	}
	put("identity-ok", "tools-ok")
	if source() != "local_estimate" {
		t.Fatal("身份与工具集均不符却未退回本地估算", source())
	}
	put(aiStore.Get("chat").contextIdentity(), "tools-ok")
	if source() != "local_estimate" {
		t.Fatal("工具集不符却未使锚点失效", source())
	}
	put("identity-ok", yyUsageToolsHash(yyQQAITools(false)))
	if source() != "local_estimate" {
		t.Fatal("身份不符却未使锚点失效", source())
	}
	put(aiStore.Get("chat").contextIdentity(), yyUsageToolsHash(yyQQAITools(false)))
	if source() != "provider_reported" {
		t.Fatal("身份与工具集都匹配后应恢复 provider_reported", source())
	}
}

// TestYYUsageDiskFailureDegradation §4.5：bbolt 打不开时内存聚合继续、
// DiskError 透出为 persistence_error、不阻塞记账；重启到可用目录后恢复
// （未落盘数据丢失是文档化的可接受降级）。
func TestYYUsageDiskFailureDegradation(t *testing.T) {
	cf, dir := yyUsageFixture(t)
	blocker := filepath.Join(dir, "not-a-dir")
	if e := os.WriteFile(blocker, []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	yyUsageInit(blocker) // root 是普通文件 → qq-usage.db 必然打不开
	yyUsage.Lock()
	diskErr := yyUsage.DiskError
	yyUsage.Unlock()
	if diskErr == "" {
		t.Fatal("磁盘故障未透出 DiskError")
	}
	yyUsageAddAt(cf, "u1", "chat", "chat", aiTurn{Role: "user", Text: "降级", InputTokens: 100, OutputTokens: 10}, time.Now())
	q := yyUsageQuery(cf, "", "", "")
	totals := q["totals"].(map[string]any)
	if totals["input"].(int) != 100 || totals["real_calls"].(int) != 1 {
		t.Fatal("磁盘故障下内存聚合应继续", totals)
	}
	if q["persistence_error"].(string) == "" {
		t.Fatal("persistence_error 未透出")
	}
	// 重启到可用目录：未落盘数据丢失（降级），且不再报磁盘错误
	yyUsageClose()
	yyUsageInit(dir)
	if q = yyUsageQuery(cf, "", "", ""); q["totals"].(map[string]any)["real_calls"].(int) != 0 {
		t.Fatal("未落盘数据不应凭空恢复", q["totals"])
	}
	yyUsage.Lock()
	recovered := yyUsage.DiskError
	yyUsage.Unlock()
	if recovered != "" {
		t.Fatal("恢复后不应有 DiskError", recovered)
	}
}

// TestYYUsageRoutesMethodGuardsAndScopeParam 路由方法守卫与 scope 参数：
// 两个端点都只接受 GET；/api/qq/usage?scope= 只返回该群的聚合。
func TestYYUsageRoutesMethodGuardsAndScopeParam(t *testing.T) {
	cf, dir := yyUsageFixture(t)
	mux := http.NewServeMux()
	yyQQUsageRoutes(mux, dir)
	yyUsageAddAt(cf, "小明", "chat", "chat", aiTurn{Role: "user", Text: "路由", InputTokens: 100, OutputTokens: 10}, time.Now())
	call := func(method, path string) (int, map[string]any) {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out) // 405 纯文本响应不解析
		return w.Code, out
	}
	if code, _ := call(http.MethodPost, "/api/qq/usage/context"); code != http.StatusMethodNotAllowed {
		t.Fatal("context 应只接受 GET", code)
	}
	scope := yyQQMessageKey(cf)
	code, out := call(http.MethodGet, "/api/qq/usage?scope="+scope)
	if code != http.StatusOK || out["totals"].(map[string]any)["input"].(float64) != 100 {
		t.Fatal("scope 参数未生效", code, out)
	}
	if code, out = call(http.MethodGet, "/api/qq/usage?scope=deadbeef"); code != http.StatusOK || out["totals"].(map[string]any)["input"].(float64) != 0 {
		t.Fatal("陌生 scope 应无数据", code, out)
	}
}
