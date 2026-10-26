package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixtures replace only private in-memory stores and use loopback HTTP.
// No test sends a QQ message or contacts a real model/embedding provider.
func yyQQMemoryFixture(t *testing.T) yyQQConfig {
	t.Helper()
	oldQQ, oldStore := yyQQ, aiStore
	oldGroups, oldEpoch := yyQQMessages.Groups, yyQQMessages.Epoch
	oldMemory, oldMemoryEpoch, oldStatus, oldPath, oldError := yyQQMemory.Groups, yyQQMemory.Epoch, yyQQMemory.Status, yyQQMemory.Path, yyQQMemory.DiskError
	oldHistory, oldBusy, oldLast := yyQQAIHistory.Items, yyQQAIHistory.Busy, yyQQAIHistory.Last
	t.Cleanup(func() {
		yyQQ = oldQQ
		aiStore = oldStore
		yyQQMessages.Groups = oldGroups
		yyQQMessages.Epoch = oldEpoch
		yyQQMemory.Groups = oldMemory
		yyQQMemory.Epoch = oldMemoryEpoch
		yyQQMemory.Status = oldStatus
		yyQQMemory.Path = oldPath
		yyQQMemory.DiskError = oldError
		yyQQAIHistory.Items = oldHistory
		yyQQAIHistory.Busy = oldBusy
		yyQQAIHistory.Last = oldLast
	})
	yyQQMessages.Groups = map[string][]yyQQGroupMessage{}
	yyQQMessages.Epoch = map[string]uint64{}
	yyQQMemory.Groups = map[string][]yyQQMemoryRecord{}
	yyQQMemory.Epoch = map[string]uint64{}
	yyQQMemory.Status = map[string]string{}
	yyQQMemory.Path = ""
	yyQQMemory.DiskError = ""
	yyQQAIHistory.Items = map[string][]aiTurn{}
	yyQQAIHistory.Busy = map[string]bool{}
	yyQQAIHistory.Last = map[string]time.Time{}
	aiStore = &aiModelStore{}
	cf := yyQQConfig{Adapter: "qqofficial", Enabled: true, GroupID: "group-a", Groups: []yyQQGroup{{ID: "group-a", Enabled: true}, {ID: "group-b", Enabled: true}}}
	cf.Official.AppID = "fixture-bot"
	cf.Assistant.Memory.Enabled = true
	yyQQ = cf
	return cf
}

func TestQQStructuredIdentitySurvivesSameNameAndReply(t *testing.T) {
	cf := yyQQMemoryFixture(t)
	yyQQRememberGroupMessageMeta(cf, "member-a", "同名", "甲说苹果", "message-a", "", false)
	yyQQRememberGroupMessageMeta(cf, "member-b", "同名", "乙在回复甲", "message-b", "message-a", false)
	rows := yyQQRecentMessages(cf, 80)
	if len(rows) != 2 || rows[0].User == rows[1].User || rows[1].ReplyID != "message-a" {
		t.Fatal("received identity/reply missing", rows)
	}
	prompt := yyQQGroupContext(cf, false)
	for _, v := range []string{yyQQIdentityRef(cf, "member-a"), yyQQIdentityRef(cf, "member-b"), "message-a", "message-b", "reply_to_message_id"} {
		if !strings.Contains(prompt, v) {
			t.Fatalf("missing %s", v)
		}
	}
	if strings.Contains(prompt, `"sender_id":"member-a"`) {
		t.Fatal("raw official openid exposed to model instead of stable alias")
	}
	current := yyQQCurrentMessage(cf, "member-b", "这是谁说的？", "message-b", false)
	if !strings.Contains(current, yyQQIdentityRef(cf, "member-b")) || !strings.Contains(current, `"reply_to_message_id":"message-a"`) {
		t.Fatal(current)
	}
	other := cf
	other.GroupID = "group-b"
	if yyQQIdentityRef(cf, "member-a") == yyQQIdentityRef(other, "member-a") {
		t.Fatal("group alias crossed namespace")
	}
	other = cf
	other.Adapter = "onebot"
	if yyQQAISessionKey(cf, "member-a", false) == yyQQAISessionKey(other, "member-a", false) {
		t.Fatal("adapter histories shared")
	}
	other = cf
	other.Official.AppID = "another-bot"
	if yyQQAISessionKey(cf, "member-a", false) == yyQQAISessionKey(other, "member-a", false) {
		t.Fatal("bot histories shared")
	}
}

func TestQQStableIDsPreserveLargeOneBotNumbers(t *testing.T) {
	d := json.NewDecoder(strings.NewReader(`{"self_id":9007199254740993,"user_id":9007199254740995,"message":[{"type":"reply","data":{"id":9007199254740997}}]}`))
	d.UseNumber()
	var event map[string]any
	if e := d.Decode(&event); e != nil {
		t.Fatal(e)
	}
	if yyQQStableID(event["self_id"]) != "9007199254740993" || yyQQStableID(event["user_id"]) != "9007199254740995" || yyQQEventReplyID(event) != "9007199254740997" {
		t.Fatal("numeric ID lost precision")
	}
	if yyQQStableID(float64(9007199254740992)) != "" || yyQQStableID(nil) != "" {
		t.Fatal("lossy/missing ID accepted")
	}
	if yyQQEventReplyID(map[string]any{"message_reference": map[string]any{"message_id": "official-message"}}) != "official-message" {
		t.Fatal("official reply lost")
	}
}

func TestQQContextDedupTTLAndCharacterBudget(t *testing.T) {
	cf := yyQQMemoryFixture(t)
	cf.Assistant.Memory.Enabled = false
	cf.Assistant.Memory.ContextChars = 1200
	cf.Assistant.Memory.TTLHours = 1
	for i := 0; i < 150; i++ {
		yyQQRememberGroupMessage(cf, "a", "同名", strings.Repeat("测试", 2000), fmt.Sprint(i))
	}
	rows := yyQQRecentMessages(cf, 80)
	chars := 0
	for _, row := range rows {
		chars += len([]rune(row.Text))
	}
	if len(rows) > 80 || chars > 32000 {
		t.Fatal("unbounded raw context")
	}
	before := len(rows)
	yyQQRememberGroupMessage(cf, "a", "同名", "duplicate", "149")
	if len(yyQQRecentMessages(cf, 80)) != before {
		t.Fatal("duplicate delivery added context")
	}
	prompt := yyQQGroupContext(cf, false)
	if len([]rune(prompt)) > 1200 {
		t.Fatalf("prompt exceeded budget: %d", len([]rune(prompt)))
	}
	parts := strings.SplitN(prompt, "\n", 2)
	if len(parts) != 2 || !json.Valid([]byte(parts[1])) {
		t.Fatal("compression cut a JSON identity structure")
	}
	yyQQMessages.Groups[yyQQMessageKey(cf)] = []yyQQGroupMessage{{ID: "old", Timestamp: time.Now().Add(-2 * time.Hour), Text: "expired"}, {ID: "new", Timestamp: time.Now(), Text: "fresh"}}
	if got := yyQQRecentMessages(cf, 80); len(got) != 1 || got[0].ID != "new" {
		t.Fatal("TTL did not purge expired rows", got)
	}
	history := []aiTurn{}
	for i := 0; i < 30; i++ {
		history = append(history, aiTurn{Role: "user", Text: strings.Repeat("a", 1000)}, aiTurn{Role: "assistant", Text: strings.Repeat("b", 1000)})
	}
	history = yyQQBoundAIHistory(history, 3500)
	n := 0
	for _, h := range history {
		n += len(h.Text)
	}
	if len(history)%2 != 0 || n > 3500 {
		t.Fatal("history budget/pairs broken")
	}
}

func TestQQMemoryLexicalFallbackIsolationExpiryAndClear(t *testing.T) {
	cf := yyQQMemoryFixture(t)
	yyQQRememberGroupMessage(cf, "a", "同名", "苹果方案A", "m1")
	other := cf
	other.GroupID = "group-b"
	yyQQRememberGroupMessage(other, "b", "同名", "苹果方案B", "m1")
	got, mode := yyQQMemorySearch(context.Background(), cf, "苹果", 5)
	if mode != "lexical_fallback_embedding_not_configured" || len(got) != 1 || got[0].User != "a" {
		t.Fatal("fallback or isolation failed", mode, got)
	}
	if quoted, _ := yyQQMemorySearchWithReply(context.Background(), cf, "这是谁说的", "m1", 1); len(quoted) != 1 || quoted[0].User != "a" {
		t.Fatal("exact reply reference failed without lexical overlap")
	}
	session := yyQQAISessionKey(cf, "a", false)
	epoch := yyQQContextEpoch(cf)
	yyQQStoreAIHistory(cf, epoch, session, []aiTurn{{Role: "user", Text: "a"}, {Role: "assistant", Text: "b"}})
	yyQQMemoryClear(cf, "m1")
	if len(yyQQRecentMessages(cf, 80)) != 0 || len(yyQQAIHistory.Items[session]) != 0 {
		t.Fatal("delete left recent/AI history")
	}
	if yyQQStoreAIHistory(cf, epoch, session, []aiTurn{{Role: "user", Text: "stale"}}) {
		t.Fatal("inflight answer resurrected deleted history")
	}
	if rows, mode := yyQQMemorySearch(context.Background(), cf, "苹果", 5); len(rows) != 0 || mode != "empty" {
		t.Fatal(rows, mode)
	}
	if rows, _ := yyQQMemorySearch(context.Background(), other, "苹果", 5); len(rows) != 1 {
		t.Fatal("clear affected other group")
	}
	cf.Assistant.Memory.Enabled = false
	if rows, mode := yyQQMemorySearch(context.Background(), cf, "苹果", 5); rows != nil || mode != "disabled" {
		t.Fatal("disabled memory searched")
	}
}

func TestQQEmbeddingRealHTTPFixtureAndClearDuringIndex(t *testing.T) {
	cf := yyQQMemoryFixture(t)
	yyQQRememberGroupMessage(cf, "a", "同名", "苹果设计", "m1")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/v1/embeddings" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("wrong embedding endpoint or key")
		}
		var body struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "fixture-embedding" || len(body.Input) != 1 {
			t.Error("wrong embedding input")
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"index": 0, "embedding": []float32{1, 0}}}})
	}))
	defer server.Close()
	aiStore.models = []*AIModel{{Name: "embed", Provider: ProviderOpenAI, ModelID: "fixture-embedding", BaseURL: server.URL, APIKey: "fixture-key"}}
	cf.Assistant.Memory.EmbeddingModel = "embed"
	model, fp := yyQQEmbeddingProfile(cf)
	row := yyQQMemory.Groups[yyQQMessageKey(cf)][0]
	yyQQMemoryIndex(context.Background(), cf, row.Message, 0, model, fp)
	got, mode := yyQQMemorySearch(context.Background(), cf, "毫无词法重叠", 5)
	if len(got) != 1 || !strings.HasPrefix(mode, "vector_") || requests != 2 {
		t.Fatal("actual embedding fixture not used", mode, requests)
	}
	yyQQMemoryClear(cf, "")
	yyQQMemoryIndex(context.Background(), cf, row.Message, 0, model, fp)
	if len(yyQQMemory.Groups[yyQQMessageKey(cf)]) != 0 {
		t.Fatal("stale indexing recreated cleared row")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("fixture-key should never appear in errors"))
	}))
	defer bad.Close()
	_, e := yyQQEmbedding(context.Background(), &AIModel{BaseURL: bad.URL, ModelID: "fixture", APIKey: "fixture-key"}, []string{"x"})
	if e == nil || strings.Contains(e.Error(), "fixture-key") {
		t.Fatal("error leaked provider/key", e)
	}
}

func TestQQMemoryPersistenceAndAPIUseOnlyEnabledGroup(t *testing.T) {
	cf := yyQQMemoryFixture(t)
	yyQQMemory.Path = filepath.Join(t.TempDir(), "qq-group-memory.json")
	yyQQRememberGroupMessageMeta(cf, "a", "同名", "苹果", "m1", "parent", false)
	raw, e := os.ReadFile(yyQQMemory.Path)
	if e != nil || !json.Valid(raw) {
		t.Fatal("persistence missing", e)
	}
	yyQQMemory.Groups = map[string][]yyQQMemoryRecord{}
	yyQQMemoryInit(filepath.Dir(yyQQMemory.Path))
	if rows, _ := yyQQMemorySearch(context.Background(), cf, "苹果", 5); len(rows) != 1 || rows[0].ReplyID != "parent" || rows[0].User != "a" {
		t.Fatal("identity lost on persistence", rows)
	}
	mux := http.NewServeMux()
	yyQQMessageRoutes(mux)
	for _, method := range []string{"GET", "DELETE"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, "/api/qq/memory?group=foreign", nil))
		if !strings.Contains(w.Body.String(), `"ok":false`) {
			t.Fatal("foreign group accepted", w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/qq/memory?group=group-a", nil))
	if !strings.Contains(w.Body.String(), "lexical_fallback_embedding_not_configured") {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/qq/memory?group=group-a", nil))
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatal(w.Body.String())
	}
	data, _ := os.ReadFile(yyQQMemory.Path)
	if strings.Contains(string(data), "苹果") {
		t.Fatal("clear left persistent data")
	}
}

func TestQQAIRequestIdentityAndClearInvalidatesCompletion(t *testing.T) {
	cf := yyQQMemoryFixture(t)
	cf.Assistant.Model = "chat"
	cf.Assistant.Memory.Enabled = false
	yyQQ = cf
	yyQQRememberGroupMessageMeta(cf, "member-a", "同名", "甲说苹果", "m1", "", false)
	yyQQRememberGroupMessageMeta(cf, "member-b", "同名", "这是谁说的", "m2", "m1", false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []map[string]any `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		current := body.Messages[len(body.Messages)-1]["content"].(string)
		if !strings.Contains(current, yyQQIdentityRef(cf, "member-b")) || !strings.Contains(current, `"reply_to_message_id":"m1"`) {
			t.Error("current sender/reply lost", current)
		}
		if !strings.Contains(body.Messages[0]["content"].(string), "不能冒充") {
			t.Error("bot identity missing")
		}
		yyQQMemoryClear(cf, "")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "stale answer"}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	aiStore.models = []*AIModel{{Name: "chat", Provider: ProviderOpenAI, BaseURL: server.URL, ModelID: "fixture-chat"}}
	answer, e := yyQQAskAIWithReply(cf, "member-b", "这是谁说的", "m2")
	if e == nil || answer != "" || !strings.Contains(e.Error(), "清空") {
		t.Fatal("clear failed to invalidate model completion", answer, e)
	}
	if len(yyQQAIHistory.Items) != 0 {
		t.Fatal("cleared completion returned to history")
	}
}

func TestQQMemoryCapsTTLOnDiskAndMissingCloudCredential(t *testing.T) {
	cf := yyQQMemoryFixture(t)
	cf.Groups = nil
	for g := 0; g < 33; g++ {
		cf.GroupID = fmt.Sprintf("g-%d", g)
		for i := 0; i < 170; i++ {
			yyQQMemoryRemember(cf, yyQQGroupMessage{ID: fmt.Sprint(i), User: "a", Text: "苹果", Timestamp: time.Now()})
		}
	}
	if len(yyQQMemory.Groups) > 32 || len(yyQQMemory.Groups[yyQQMessageKey(cf)]) != 160 {
		t.Fatal("RAG group/entry cap exceeded")
	}
	aiStore.models = []*AIModel{{Name: "cloud-no-key", Provider: ProviderOpenAI, BaseURL: "https://embedding.example.invalid", ModelID: "embedding"}}
	cf.Assistant.Memory.EmbeddingModel = "cloud-no-key"
	if model, _ := yyQQEmbeddingProfile(cf); model != nil {
		t.Fatal("cloud embeddings without credentials enabled")
	}
	got, mode := yyQQMemorySearch(context.Background(), cf, "苹果", 5)
	if len(got) != 5 || mode != "lexical_fallback_embedding_not_configured" {
		t.Fatal("missing cloud key did not fall back", mode)
	}
	cf.Assistant.Memory.EmbeddingModel = ""
	cf.Assistant.Memory.TTLHours = 1
	yyQQMemory.Path = filepath.Join(t.TempDir(), "qq-group-memory.json")
	yyQQMemory.Groups = map[string][]yyQQMemoryRecord{yyQQMessageKey(cf): {{Message: yyQQGroupMessage{ID: "expired", User: "a", Text: "expired-private-text", Timestamp: time.Now().Add(-2 * time.Hour)}}}}
	yyQQMemorySaveLocked()
	if rows, mode := yyQQMemorySearch(context.Background(), cf, "expired", 5); len(rows) != 0 || mode != "empty" {
		t.Fatal("expired data recalled", mode)
	}
	raw, e := os.ReadFile(yyQQMemory.Path)
	if e != nil || strings.Contains(string(raw), "expired-private-text") {
		t.Fatal("TTL failed to purge disk", e)
	}
}

func TestQQEmbeddingRejectsInvalidOrFabricatedVectors(t *testing.T) {
	for _, body := range []string{`{"data":[{"index":0,"embedding":[0,0]}]}`, `{"data":[{"index":1,"embedding":[1,0]}]}`, `{"data":[]}`, `{"data":[{"index":0,"embedding":[]}]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		_, e := yyQQEmbedding(context.Background(), &AIModel{BaseURL: server.URL, ModelID: "fixture"}, []string{"query"})
		server.Close()
		if e == nil {
			t.Fatal("invalid/zero vector accepted", body)
		}
	}
}

func TestQQMemoryClearDuringHTTPIndexCannotRestoreDeletedText(t *testing.T) {
	cf := yyQQMemoryFixture(t)
	yyQQRememberGroupMessage(cf, "a", "昵称", "将删除的正文", "message")
	msg := yyQQMemory.Groups[yyQQMessageKey(cf)][0].Message
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.Write([]byte(`{"data":[{"index":0,"embedding":[1,0]}]}`))
	}))
	defer server.Close()
	go func() {
		defer close(done)
		yyQQMemoryIndex(context.Background(), cf, msg, 0, &AIModel{BaseURL: server.URL, ModelID: "fixture"}, "fixture-profile")
	}()
	<-started
	yyQQMemoryClear(cf, "")
	close(release)
	<-done
	if len(yyQQMemory.Groups[yyQQMessageKey(cf)]) != 0 || yyQQMemory.Status[yyQQMessageKey(cf)] != "cleared" {
		t.Fatal("late embedding response restored cleared memory")
	}
}

func TestQQMemoryMaintenanceExpiresIdleGroupsAndHistory(t *testing.T) {
	cf := yyQQMemoryFixture(t)
	cf.Assistant.Memory.TTLHours = 1
	now := time.Now()
	scope := yyQQMessageKey(cf)
	yyQQMessages.Groups[scope] = []yyQQGroupMessage{{ID: "old", Text: "expired", Timestamp: now.Add(-2 * time.Hour)}}
	yyQQMemory.Groups[scope] = []yyQQMemoryRecord{{Message: yyQQMessages.Groups[scope][0]}}
	session := yyQQAISessionKey(cf, "a", false)
	yyQQAIHistory.Items[session] = []aiTurn{{Role: "user", Text: "expired"}}
	yyQQAIHistory.Last[session] = now.Add(-2 * time.Hour)
	yyQQExpireMemory(cf, now)
	if len(yyQQMemory.Groups) != 0 || len(yyQQMessages.Groups) != 0 || len(yyQQAIHistory.Items) != 0 {
		t.Fatal("idle expired history not cleaned")
	}
}
