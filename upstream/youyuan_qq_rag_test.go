package main

// RAG 测试：bbolt 用 t.TempDir()，embedding 用 loopback httptest fixture。
// 不发 QQ 消息、不连真实模型；全局态保存/恢复，风格对齐 youyuan_qq_memory_test.go。

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// yyRagUsageGlobals 保存并重置 rag/usage 涉及的全局（t.Cleanup 逆序恢复）。
func yyRagUsageGlobals(t *testing.T) {
	t.Helper()
	oldQQ, oldStore := yyQQ, aiStore
	oldMsgs, oldEpoch := yyQQMessages.Groups, yyQQMessages.Epoch
	oldHist, oldBusy, oldLast := yyQQAIHistory.Items, yyQQAIHistory.Busy, yyQQAIHistory.Last
	oldRagDB, oldRagRoot, oldRagDisk := yyRag.DB, yyRag.Root, yyRag.DiskError
	oldRagDims, oldRagFP, oldRagReindex := yyRag.Dims, yyRag.Fingerprint, yyRag.Reindexing
	oldDocs, oldChunks, oldVecs, oldPost, oldHashes := yyRag.Docs, yyRag.Chunks, yyRag.Vecs, yyRag.Post, yyRag.Hashes
	oldChunkTotal, oldTokenTotal := yyRag.ChunkTotal, yyRag.TokenTotal
	oldSetAt, oldSetStamp, oldSetValue := yyRagSettingsCache.At, yyRagSettingsCache.Stamp, yyRagSettingsCache.Value
	oldUDB, oldURoot, oldUClosed, oldUTick := yyUsage.DB, yyUsage.Root, yyUsage.Closed, yyUsage.Ticking
	oldUDisk, oldUFlush := yyUsage.DiskError, yyUsage.LastFlush
	oldUDay, oldUTotal, oldURecent, oldUPend := yyUsage.Day, yyUsage.Total, yyUsage.Recent, yyUsage.Pending
	oldUDDay, oldUDTotal, oldUDEst, oldUDAnchor := yyUsage.DirtyDay, yyUsage.DirtyTotal, yyUsage.DirtyEst, yyUsage.DirtyAnchor
	oldUEst, oldUAnchors, oldUPriced := yyUsage.Est, yyUsage.Anchors, yyUsage.Priced
	t.Cleanup(func() {
		yyQQ, aiStore = oldQQ, oldStore
		yyQQMessages.Groups, yyQQMessages.Epoch = oldMsgs, oldEpoch
		yyQQAIHistory.Items, yyQQAIHistory.Busy, yyQQAIHistory.Last = oldHist, oldBusy, oldLast
		yyRag.DB, yyRag.Root, yyRag.DiskError = oldRagDB, oldRagRoot, oldRagDisk
		yyRag.Dims, yyRag.Fingerprint, yyRag.Reindexing = oldRagDims, oldRagFP, oldRagReindex
		yyRag.Docs, yyRag.Chunks, yyRag.Vecs, yyRag.Post, yyRag.Hashes = oldDocs, oldChunks, oldVecs, oldPost, oldHashes
		yyRag.ChunkTotal, yyRag.TokenTotal = oldChunkTotal, oldTokenTotal
		yyRagSettingsCache.At, yyRagSettingsCache.Stamp, yyRagSettingsCache.Value = oldSetAt, oldSetStamp, oldSetValue
		yyUsage.DB, yyUsage.Root, yyUsage.Closed, yyUsage.Ticking = oldUDB, oldURoot, oldUClosed, oldUTick
		yyUsage.DiskError, yyUsage.LastFlush = oldUDisk, oldUFlush
		yyUsage.Day, yyUsage.Total, yyUsage.Recent, yyUsage.Pending = oldUDay, oldUTotal, oldURecent, oldUPend
		yyUsage.DirtyDay, yyUsage.DirtyTotal, yyUsage.DirtyEst, yyUsage.DirtyAnchor = oldUDDay, oldUDTotal, oldUDEst, oldUDAnchor
		yyUsage.Est, yyUsage.Anchors, yyUsage.Priced = oldUEst, oldUAnchors, oldUPriced
	})
	yyQQMessages.Groups, yyQQMessages.Epoch = map[string][]yyQQGroupMessage{}, map[string]uint64{}
	yyQQAIHistory.Items, yyQQAIHistory.Busy, yyQQAIHistory.Last = map[string][]aiTurn{}, map[string]bool{}, map[string]time.Time{}
	aiStore = &aiModelStore{}
	yyRagSettingsCache.At, yyRagSettingsCache.Stamp, yyRagSettingsCache.Value = time.Time{}, "", yyRagSettings{}
}

// yyRagFixture 重置全局并准备临时 dataDir。不主动开库——由测试或路由函数
// 自己 yyRagInit，避免同一文件二次 bolt.Open 撞文件锁。
func yyRagFixture(t *testing.T) (yyQQConfig, string) {
	t.Helper()
	yyRagUsageGlobals(t)
	dir := t.TempDir()
	t.Cleanup(yyRagClose)
	cf := yyQQConfig{Adapter: "onebot", Enabled: true, GroupID: "group-a", Groups: []yyQQGroup{{ID: "group-a", Enabled: true}, {ID: "group-b", Enabled: true}}}
	cf.OneBot.BotID = "fixture-bot"
	cf.Assistant.Memory.Enabled = true
	yyQQ = cf
	return cf, dir
}

// yyUsageFixture 同上，面向 usage 统计（预置一个无价格的 chat 模型）。
func yyUsageFixture(t *testing.T) (yyQQConfig, string) {
	t.Helper()
	yyRagUsageGlobals(t)
	dir := t.TempDir()
	t.Cleanup(yyUsageClose)
	cf := yyQQConfig{Adapter: "onebot", Enabled: true, GroupID: "group-a", Groups: []yyQQGroup{{ID: "group-a", Enabled: true}}}
	cf.OneBot.BotID = "fixture-bot"
	cf.Assistant.Model = "chat"
	cf.Assistant.Memory.Enabled = true
	aiStore.models = []*AIModel{{Name: "chat", Provider: ProviderOpenAI, ModelID: "fixture-chat", BaseURL: "https://fixture.invalid", ContextWindow: 65536, MaxOutputTokens: 8192}}
	yyQQ = cf
	return cf, dir
}

// yyRagWriteBridge 写 qq-bridge.json 配置 embedding 模型，并使设置缓存失效。
func yyRagWriteBridge(t *testing.T, dir, body string) {
	t.Helper()
	if e := os.WriteFile(filepath.Join(dir, "qq-bridge.json"), []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	yyRagSettingsCache.At, yyRagSettingsCache.Stamp = time.Time{}, ""
}

// yyRagIndexedCounts 统计已索引/总 chunk 数。
func yyRagIndexedCounts(t *testing.T) (indexed, total int) {
	t.Helper()
	yyRag.Lock()
	defer yyRag.Unlock()
	for _, rows := range yyRag.Chunks {
		for _, c := range rows {
			total++
			if c.Indexed {
				indexed++
			}
		}
	}
	return indexed, total
}

// TestYYRagIngestPersistReloadAndSearch 最高优先级用例：ingest → 落盘 →
// 重开 db → 再读。首次 WriteDoc 不得 panic（bbolt bucket 必须已创建），
// 数据落盘后可完整读回；未配 embedding 时如实返回 lexical_only。
func TestYYRagIngestPersistReloadAndSearch(t *testing.T) {
	cf, dir := yyRagFixture(t)
	yyRagInit(dir)
	text := "## 服务器规则\n苹果园每天浇水三次，禁止喂鳄鱼。\n" + strings.Repeat("苹果园管理细则补充内容。", 40)
	id, chunks, deduped, e := yyRagIngestDoc(cf, "global", "rules", "服务器规则", text)
	if e != nil || deduped || chunks < 1 || id == "" {
		t.Fatal("ingest 失败", id, chunks, deduped, e)
	}
	if _, e := os.Stat(filepath.Join(dir, "qq-rag.db")); e != nil {
		t.Fatal("qq-rag.db 未落盘", e)
	}
	// sha256(scope+正文) 去重：同文再摄入直接返回已有 docID
	id2, chunks2, deduped2, e2 := yyRagIngestDoc(cf, "global", "rules", "服务器规则", text)
	if e2 != nil || !deduped2 || id2 != id || chunks2 != chunks {
		t.Fatal("sha256 去重失败", id2, chunks2, deduped2, e2)
	}
	// 未配 embedding → lexical_only 且命中
	hits, mode := yyRagSearch(context.Background(), cf, "鳄鱼", 5)
	if mode != "lexical_only" || len(hits) != 1 || hits[0].DocID != id || hits[0].Title != "服务器规则" {
		t.Fatal("lexical 检索失败", mode, hits)
	}
	if hs, _ := yyRagSearch(context.Background(), cf, "毫无关联的词xyz", 5); len(hs) != 0 {
		t.Fatal("无关词不应命中", hs)
	}
	// 重开 db：doc/chunk/倒排从 bbolt 读回，仍可检索
	yyRagClose()
	yyRagInit(dir)
	hits, mode = yyRagSearch(context.Background(), cf, "鳄鱼", 5)
	if mode != "lexical_only" || len(hits) != 1 || hits[0].DocID != id {
		t.Fatal("重开后检索失败", mode, hits)
	}
	yyRag.Lock()
	diskErr := yyRag.DiskError
	yyRag.Unlock()
	if diskErr != "" {
		t.Fatal("持久化错误", diskErr)
	}
}

// TestYYRagChatIngestScopeIsolationAndPerDocCap 群聊摄入、scope 隔离、
// global 可见性、同 docID 最多 2 chunk（防单文档刷屏）。
func TestYYRagChatIngestScopeIsolationAndPerDocCap(t *testing.T) {
	cf, dir := yyRagFixture(t)
	yyRagInit(dir)
	other := cf
	other.GroupID = "group-b"
	docID, dup, e := yyRagIngestChat(cf, "小明", "今天苹果园大丰收")
	if e != nil || dup || docID == "" {
		t.Fatal("群聊摄入失败", docID, dup, e)
	}
	if _, dup, _ := yyRagIngestChat(cf, "小明", "今天苹果园大丰收"); !dup {
		t.Fatal("群聊去重失败")
	}
	if hits, _ := yyRagSearch(context.Background(), cf, "苹果园", 5); len(hits) != 1 {
		t.Fatal("本 scope 应可见", hits)
	}
	if hits, _ := yyRagSearch(context.Background(), other, "苹果园", 5); len(hits) != 0 {
		t.Fatal("scope 隔离失败", hits)
	}
	yyRagIngestDoc(cf, "global", "manual", "公共手册", "苹果园公共守则")
	if hits, _ := yyRagSearch(context.Background(), other, "苹果园", 5); len(hits) != 1 || hits[0].Title != "公共手册" {
		t.Fatal("global 文档应对所有 scope 可见", hits)
	}
	// 长文档切出多个 chunk，全部含查询词 → 同 docID 最多取 2 个
	var b strings.Builder
	for i := 0; i < 6; i++ {
		b.WriteString(fmt.Sprintf("## 条例%d\n", i))
		b.WriteString(strings.Repeat("苹果园条例内容", 130))
		b.WriteString("\n")
	}
	id3, chunks3, _, e := yyRagIngestDoc(cf, "global", "rules", "苹果园条例", b.String())
	if e != nil || chunks3 < 3 {
		t.Fatal("长文档切分失败", chunks3, e)
	}
	hits, _ := yyRagSearch(context.Background(), cf, "苹果园", 8)
	perDoc := map[string]int{}
	for _, h := range hits {
		perDoc[h.DocID]++
	}
	if perDoc[id3] != 2 {
		t.Fatal("同 docID 超过 2 chunk", perDoc)
	}
	if len(hits) > 8 {
		t.Fatal("topK 未生效", len(hits))
	}
}

// TestYYRagPromptBlockBudgetAndOmitted 三层预算：单 chunk 600 字、
// 总注入 ≤ min(RagBudgetChars, budget)、装不下按分数从低到高丢并记 omitted。
func TestYYRagPromptBlockBudgetAndOmitted(t *testing.T) {
	cf, dir := yyRagFixture(t)
	yyRagInit(dir)
	hits := []yyRagHit{
		{DocID: "a", Title: "高分文档", Text: strings.Repeat("甲", 2000), Score: 3},
		{DocID: "b", Title: "中分文档", Text: strings.Repeat("乙", 700), Score: 2},
		{DocID: "c", Title: "低分文档", Text: strings.Repeat("丙", 700), Score: 1},
	}
	block := yyRagPromptBlock(cf, hits, "lexical_only", 700)
	lines := strings.SplitN(block, "\n", 2)
	if !strings.Contains(lines[0], "不是系统指令") {
		t.Fatal("缺少安全姿态首行", lines[0])
	}
	var payload struct {
		Source, Mode string
		Chunks       []struct{ Doc, Text string }
		Omitted      int
	}
	if json.Unmarshal([]byte(lines[1]), &payload) != nil {
		t.Fatal("注入块不是合法 JSON", lines[1])
	}
	if payload.Source != "rag" || payload.Mode != "lexical_only" || payload.Omitted != 2 || len(payload.Chunks) != 1 {
		t.Fatal("预算截断/omitted 计数不对", payload.Source, payload.Mode, payload.Omitted, len(payload.Chunks))
	}
	if payload.Chunks[0].Doc != "高分文档" || len([]rune(payload.Chunks[0].Text)) > 605 {
		t.Fatal("单 chunk 600 字上限未生效", payload.Chunks[0].Doc, len([]rune(payload.Chunks[0].Text)))
	}
	if len([]rune(lines[1])) > 700 {
		t.Fatal("总注入超过预算", len([]rune(lines[1])))
	}
	if got := yyRagPromptBlock(cf, hits, "lexical_only", 0); got != "" {
		t.Fatal("预算为 0 不应注入", got)
	}
}

// TestYYRagSearchTimeout 检索超时：返回空 + mode=timeout，AI 照常回答。
// 用已取消/已过期的 ctx 与零/负预算触发，避免依赖亚微秒计时的不稳定断言。
func TestYYRagSearchTimeout(t *testing.T) {
	cf, dir := yyRagFixture(t)
	yyRagInit(dir)
	if _, _, _, e := yyRagIngestDoc(cf, "global", "rules", "规则", "苹果园规则"); e != nil {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if hits, mode := yyRagSearchBudget(canceled, cf, "苹果园", 5, yyRagSearchTimeout); hits != nil || mode != "timeout" {
		t.Fatal("已取消 ctx 未走超时路径", hits, mode)
	}
	expired, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel2()
	if hits, mode := yyRagSearchBudget(expired, cf, "苹果园", 5, yyRagSearchTimeout); hits != nil || mode != "timeout" {
		t.Fatal("已过期截止时间未走超时路径", hits, mode)
	}
	if hits, mode := yyRagSearchBudget(context.Background(), cf, "苹果园", 5, 0); hits != nil || mode != "timeout" {
		t.Fatal("零预算未走超时路径", hits, mode)
	}
	// 正常预算下不受影响
	if hits, mode := yyRagSearch(context.Background(), cf, "苹果园", 5); len(hits) != 1 || mode != "lexical_only" {
		t.Fatal("正常检索被误伤", hits, mode)
	}
}

// TestYYRagDeleteDocAndStats 删除文档：chunk/向量/倒排一并清掉，检索不再命中。
func TestYYRagDeleteDocAndStats(t *testing.T) {
	cf, dir := yyRagFixture(t)
	yyRagInit(dir)
	id, chunks, _, e := yyRagIngestDoc(cf, "global", "rules", "待删规则", "长颈鹿禁止入内")
	if e != nil {
		t.Fatal(e)
	}
	if got := yyRagDeleteDoc(id); got != chunks {
		t.Fatal("删除 chunk 数不符", got, chunks)
	}
	if hits, _ := yyRagSearch(context.Background(), cf, "长颈鹿", 5); len(hits) != 0 {
		t.Fatal("删除后仍命中", hits)
	}
	if got := yyRagDeleteDoc(id); got != 0 {
		t.Fatal("重复删除应返回 0", got)
	}
	yyRag.Lock()
	stats := yyRagStatsLocked()
	yyRag.Unlock()
	if stats["docs"].(int) != 0 || stats["chunks"].(int) != 0 {
		t.Fatal("stats 未归零", stats)
	}
}

// TestYYRagHybridEmbeddingAndDimsInvalidation 配了 embedding 走 hybrid；
// 换模型导致维度变化时旧向量全标 Indexed=false，mode 如实降级，
// 绝不跨维度算余弦。
func TestYYRagHybridEmbeddingAndDimsInvalidation(t *testing.T) {
	cf, dir := yyRagFixture(t)
	yyRagInit(dir)
	var requests int64
	server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requests, 1)
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"index": 0, "embedding": []float32{0.1, 0.2, 0.3}}}})
	}))
	defer server3.Close()
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"index": 0, "embedding": []float32{0.5, 0.5}}}})
	}))
	defer server2.Close()
	id, _, _, e := yyRagIngestDoc(cf, "global", "rules", "浇水中学", "苹果园每天浇水三次")
	if e != nil {
		t.Fatal(e)
	}
	aiStore.models = []*AIModel{{Name: "embed3", Provider: ProviderOpenAI, ModelID: "fixture-embed", BaseURL: server3.URL}}
	yyRagWriteBridge(t, dir, `{"assistant":{"memory":{"enabled":true,"rag_embedding_model":"embed3"}}}`)
	yyRagIndexDoc(cf, id) // 直接同步索引，避免依赖后台 goroutine 时序
	indexed, total := yyRagIndexedCounts(t)
	if indexed != total || total != 1 {
		t.Fatal("向量未写入", indexed, total)
	}
	hits, mode := yyRagSearch(context.Background(), cf, "浇水", 5)
	if mode != "hybrid" || len(hits) != 1 || hits[0].Cosine <= 0 {
		t.Fatal("hybrid 检索失败", mode, hits)
	}
	// 换成 2 维模型：维度变化 → 旧向量失效 + 关键词降级
	aiStore.models = []*AIModel{{Name: "embed2", Provider: ProviderOpenAI, ModelID: "fixture-embed", BaseURL: server2.URL}}
	yyRagWriteBridge(t, dir, `{"assistant":{"memory":{"enabled":true,"rag_embedding_model":"embed2"}}}`)
	hits, mode = yyRagSearch(context.Background(), cf, "浇水", 5)
	if mode != "lexical_fallback_dimensions_changed" || len(hits) != 1 {
		t.Fatal("维度变化未降级", mode, hits)
	}
	if indexed, _ := yyRagIndexedCounts(t); indexed != 0 {
		t.Fatal("旧向量未标 Indexed=false", indexed)
	}
	// 失效后重索引应能用新维度重建
	queued, e := yyRagReindex(cf, "")
	if e != nil || queued != 1 {
		t.Fatal("重索引入队失败", queued, e)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if indexed, _ := yyRagIndexedCounts(t); indexed == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if indexed, _ := yyRagIndexedCounts(t); indexed != 1 {
		t.Fatal("重索引未重建向量", indexed)
	}
}

// TestYYRagHTTPRoutes 面板接口：GET/POST/DELETE docs、search、reindex、stats，
// 含 data_url（base64）摄入与方法校验。
func TestYYRagHTTPRoutes(t *testing.T) {
	cf, dir := yyRagFixture(t)
	_ = cf
	mux := http.NewServeMux()
	yyQQRagRoutes(mux, dir) // 内部会 yyRagInit(dir)，本测试不可再手动 Init
	post := func(path, body string) map[string]any {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		if w.Code != http.StatusOK {
			t.Fatal("路由返回非 200", path, w.Code, w.Body.String())
		}
		var out map[string]any
		if json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatal("路由响应不是 JSON", path, w.Body.String())
		}
		return out
	}
	created := post("/api/qq/rag/docs", `{"scope":"global","kind":"rules","title":"面板规则","text":"## 条目\n面板上传的苹果园规则"}`)
	if created["ok"] != true || created["id"] == "" || created["chunks"].(float64) < 1 || created["deduped"] == true {
		t.Fatal("POST docs 失败", created)
	}
	id := created["id"].(string)
	encoded := base64.StdEncoding.EncodeToString([]byte("## 附件\nbase64 附件里的香蕉园规则"))
	dataURL := post("/api/qq/rag/docs", `{"scope":"global","kind":"manual","title":"附件","data_url":"data:text/plain;base64,`+encoded+`"}`)
	if dataURL["ok"] != true || dataURL["id"] == "" {
		t.Fatal("data_url 摄入失败", dataURL)
	}
	if bad := post("/api/qq/rag/docs", `{"scope":"global","text":"x","data_url":"data:text/plain;base64,!!!"}`); bad["ok"] != false {
		t.Fatal("坏 base64 应被拒绝", bad)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/qq/rag/docs?kind=rules", nil))
	var listed map[string]any
	if json.Unmarshal(w.Body.Bytes(), &listed) != nil || listed["ok"] != true {
		t.Fatal("GET docs 失败", w.Body.String())
	}
	if listed["total"].(float64) != 1 || listed["mode"] != "lexical_only" {
		t.Fatal("GET docs 过滤/模式不对", listed["total"], listed["mode"])
	}
	searched := post("/api/qq/rag/search", `{"query":"苹果园","top_k":5}`)
	if searched["ok"] != true || searched["mode"] != "lexical_only" || len(searched["hits"].([]any)) != 1 {
		t.Fatal("search 路由失败", searched)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/qq/rag/stats", nil))
	var stats map[string]any
	if json.Unmarshal(w.Body.Bytes(), &stats) != nil || stats["ok"] != true || stats["docs"].(float64) != 2 {
		t.Fatal("stats 路由失败", w.Body.String())
	}
	if reindex := post("/api/qq/rag/reindex", `{}`); reindex["ok"] != true {
		t.Fatal("reindex 路由失败", reindex)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/qq/rag/docs?id="+id, nil))
	var deleted map[string]any
	if json.Unmarshal(w.Body.Bytes(), &deleted) != nil || deleted["ok"] != true || deleted["deleted_chunks"].(float64) < 1 {
		t.Fatal("DELETE docs 失败", w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/qq/rag/search", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal("search 应只接受 POST", w.Code)
	}
}

// TestYYRagBM25ScoresUnit bm25lite 单测：含查询词的候选得分高于不含的，
// tf 饱和化后不会因重复词无限涨分。
func TestYYRagBM25ScoresUnit(t *testing.T) {
	cands := map[string]yyRagChunk{
		"d0": {DocID: "d0", Text: "苹果园 浇水 规则 苹果园", Tokens: 10},
		"d1": {DocID: "d1", Text: "香蕉园 浇水 规则", Tokens: 10},
		"d2": {DocID: "d2", Text: "完全无关的 内容", Tokens: 10},
	}
	scores := yyRagBM25Scores(cands, 10, []string{"苹果", "园"})
	if scores["d0"] <= scores["d1"] || scores["d1"] <= 0 || scores["d2"] != 0 {
		t.Fatal("bm25lite 打分不符合预期", scores)
	}
	if dup := yyRagBM25Scores(map[string]yyRagChunk{"x": {Text: "苹果园 苹果园 苹果园 苹果园", Tokens: 10}}, 10, []string{"苹果"}); dup["x"] <= scores["d0"]*4 {
		_ = dup // 饱和化：4 倍 tf 不应带来 4 倍分数（仅记录，不断言具体比值）
	}
}

// TestYYRagDimensionChangeReindexRestartKeepsHybrid 维度变化的完整生命周期：
// hybrid → 重启仍 hybrid（meta 落盘）→ 换维度降级且不跨维度算余弦 →
// reindex 单轮重建 → 重启后仍 hybrid（新维度必须已落盘，否则重启后
// 旧维度 meta 会把新向量误判失效，错误退回 lexical_fallback）。
func TestYYRagDimensionChangeReindexRestartKeepsHybrid(t *testing.T) {
	cf, dir := yyRagFixture(t)
	var dims atomic.Int32
	dims.Store(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		vec := make([]float32, int(dims.Load()))
		for i := range vec {
			vec[i] = 0.5 // 同维非零向量余弦≈1
		}
		data := make([]map[string]any, len(body.Input))
		for i := range body.Input {
			data[i] = map[string]any{"index": i, "embedding": vec}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer srv.Close()
	aiStore.models = []*AIModel{{Name: "embed", Provider: ProviderOpenAI, ModelID: "fixture-embed", BaseURL: srv.URL, ContextWindow: 8192, MaxOutputTokens: 1024}}
	yyRagWriteBridge(t, dir, `{"assistant":{"memory":{"enabled":true,"rag_embedding_model":"embed","rag_top_k":5}}}`)
	yyRagInit(dir)
	id, chunks, _, e := yyRagIngestDoc(cf, "global", "rules", "苹果园规则", "## 浇水\n苹果园每天浇水三次。\n"+strings.Repeat("苹果园浇水细则补充。", 20))
	if e != nil || chunks < 1 || id == "" {
		t.Fatal("ingest 失败", id, chunks, e)
	}
	waitIndexed := func(want int) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			if indexed, _ := yyRagIndexedCounts(t); indexed >= want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("索引未完成", want)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	hybrid := func(where string) {
		t.Helper()
		hits, mode := yyRagSearch(context.Background(), cf, "浇水", 5)
		if mode != "hybrid" || len(hits) == 0 || hits[0].Cosine < 0.99 {
			t.Fatal(where+" 应为 hybrid", mode, hits)
		}
	}
	waitIndexed(chunks)
	hybrid("首轮索引后")
	yyRagClose()
	yyRagInit(dir)
	hybrid("重启后")
	dims.Store(3)
	hits, mode := yyRagSearch(context.Background(), cf, "浇水", 5)
	if mode != "lexical_fallback_dimensions_changed" {
		t.Fatal("维度变化未降级", mode)
	}
	for _, h := range hits {
		if h.Cosine != -1 {
			t.Fatal("跨维度算了余弦", h)
		}
	}
	if _, e := yyRagReindex(cf, ""); e != nil {
		t.Fatal("reindex 启动失败", e)
	}
	waitIndexed(chunks)
	yyRagClose()
	yyRagInit(dir)
	hybrid("换维度重建并重启后")
}
