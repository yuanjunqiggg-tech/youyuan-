package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentDefaultSkillsRemainLazyAndLanguageScoped(t *testing.T) {
	for _, language := range []string{"lua", "python", "native_cpp", "mcfunction"} {
		ws := &yyWorkspace{Language: language}
		got := yyEffectiveAgentSkills(ws)
		if len(got) != len(yyAvailableAgentSkills(language)) || len(got) < 10 {
			t.Fatal(language, got)
		}
		if strings.Contains(yyAgentPreferencePrompt(ws), "https://") || len(yyAgentPreferencePrompt(ws)) > 6000 {
			t.Fatal("inlined skill bodies")
		}
		ws.Skills = []string{}
		if len(yyEffectiveAgentSkills(ws)) != 0 {
			t.Fatal("explicit unload ignored")
		}
	}
}
func TestAgentConversationPersistenceAndSessionIsolation(t *testing.T) {
	oldRoot := yyAgentRoot
	yyAgentRoot = t.TempDir()
	defer func() { yyAgentRoot = oldRoot }()
	source := &yyWorkspace{ID: "session_source_test", Name: "插件任务", Language: "lua", State: "已暂停", Pending: []string{"继续编辑"}, Servers: []yyMCPServer{{ID: "private", Token: "do-not-expose-mcp-secret"}}}
	os.MkdirAll(yyWorkspaceRoot(source), 0700)
	os.WriteFile(filepath.Join(yyWorkspaceRoot(source), "main.lua"), []byte("local a = 1"), 0600)
	for i := 0; i < 45; i++ {
		source.Turns = append(source.Turns, aiTurn{Role: "user", Text: strings.Repeat("历史记录", 20)})
	}
	yyWorkspaces.Lock()
	yyWorkspaces.items[source.ID] = source
	yyWorkspaces.Unlock()
	ids := []string{source.ID}
	defer func() {
		yyWorkspaces.Lock()
		for _, id := range ids {
			delete(yyWorkspaces.items, id)
		}
		yyWorkspaces.Unlock()
	}()
	mux := http.NewServeMux()
	yyAgentSessionRoutes(mux)
	call := func(method, path, body string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
		if strings.Contains(w.Body.String(), "do-not-expose") {
			t.Fatal("credential leaked")
		}
		var result map[string]any
		if json.Unmarshal(w.Body.Bytes(), &result) != nil || result["ok"] != true {
			t.Fatal(w.Body.String())
		}
		return result
	}
	call("POST", "/api/youyuan/agent/save", `{"id":"session_source_test"}`)
	raw, e := os.ReadFile(filepath.Join(yyAgentRoot, source.ID, "task.json"))
	if e != nil {
		t.Fatal(e)
	}
	restored := &yyWorkspace{}
	if json.Unmarshal(raw, restored) != nil || len(restored.Turns) != 45 || len(restored.Pending) != 1 {
		t.Fatal("read archive lost task state")
	}
	yyWorkspaces.Lock()
	yyWorkspaces.items[source.ID] = restored
	yyWorkspaces.Unlock()
	p := call("GET", "/api/youyuan/agent/conversation?id="+source.ID, "")
	if p["total"] != float64(45) || p["before"] != float64(25) {
		t.Fatal(p)
	}
	p = call("GET", "/api/youyuan/agent/conversation?id="+source.ID+"&before=25", "")
	if p["before"] != float64(5) {
		t.Fatal(p)
	}
	p = call("GET", "/api/youyuan/agent/conversation?id="+source.ID+"&before=5", "")
	if p["has_more"] != false || len(p["turns"].([]any)) != 5 {
		t.Fatal(p)
	}
	n := call("POST", "/api/youyuan/agent/session", `{"id":"session_source_test"}`)
	id := n["id"].(string)
	ids = append(ids, id)
	copy, e := yyGetWorkspace(id)
	if e != nil || len(copy.Turns) != 0 || len(copy.Pending) != 0 {
		t.Fatal("context contaminated", e)
	}
	os.WriteFile(filepath.Join(yyWorkspaceRoot(copy), "main.lua"), []byte("local a = 2"), 0600)
	unchanged, _ := os.ReadFile(filepath.Join(yyWorkspaceRoot(source), "main.lua"))
	if string(unchanged) != "local a = 1" {
		t.Fatal("session changed source")
	}
}
func TestCloudCapabilityFailsClosedAndDoesNotFollowRedirect(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer owner-key" {
			w.WriteHeader(403)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "allowed": true})
	}))
	defer s.Close()
	if !yyCheckCloudCapability(context.Background(), "owner-key", s.URL) || yyCheckCloudCapability(context.Background(), "user-key", s.URL) {
		t.Fatal("capability authority ignored")
	}
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected = true
		writeJSON(w, map[string]any{"ok": true, "allowed": true})
	}))
	defer target.Close()
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer redir.Close()
	if yyCheckCloudCapability(context.Background(), "owner-key", redir.URL) || redirected {
		t.Fatal("credential followed redirect")
	}
}
