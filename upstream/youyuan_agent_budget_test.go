package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func budgetFixture(t *testing.T) *yyWorkspace {
	t.Helper()
	old := yyAgentRoot
	yyAgentRoot = t.TempDir()
	t.Cleanup(func() { yyAgentRoot = old })
	ws := &yyWorkspace{ID: "budget_test", Name: "预算回归", Language: "lua", Skills: []string{}, AllowedTools: []string{"project_read", "project_list"}, Turns: []aiTurn{{Role: "user", Text: "complete the fixture task"}}}
	if e := os.MkdirAll(yyWorkspaceRoot(ws), 0700); e != nil {
		t.Fatal(e)
	}
	return ws
}
func budgetModel(url string) *AIModel {
	return &AIModel{Provider: ProviderOpenAI, BaseURL: url, ModelID: "local-fixture", Thinking: ThinkingOff, ContextWindow: 1000000, MaxOutputTokens: 512}
}
func budgetToolResponse(w http.ResponseWriter, n int, name, args string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": fmt.Sprintf("call%d", n), "type": "function", "function": map[string]any{"name": name, "arguments": args}}}}, "finish_reason": "tool_calls"}}})
}
func TestAgentContinuesPastThreeFailuresThirtyTwoToolsAndTwentyFourRounds(t *testing.T) {
	ws := budgetFixture(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 3 {
			budgetToolResponse(w, calls, "project_read", `{"path":"missing.txt"}`)
			return
		}
		if calls <= 36 {
			budgetToolResponse(w, calls, "project_list", `{}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fixture complete"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	ws.run(context.Background(), budgetModel(server.URL))
	if ws.State != "已完成" || calls != 37 {
		t.Fatal(ws.State, ws.Error, calls)
	}
	failures, results := 0, 0
	for _, turn := range ws.Turns {
		for _, r := range turn.ToolResults {
			results++
			if r.IsError {
				failures++
			}
		}
	}
	if results != 36 || failures != 3 {
		t.Fatal(results, failures)
	}
	if !strings.Contains(mustJSON(ws.Events), "连续3次工具失败") {
		t.Fatal("failure evidence lost")
	}
	data, e := os.ReadFile(filepath.Join(yyAgentRoot, ws.ID, "task.json"))
	if e != nil || !strings.Contains(string(data), "fixture complete") {
		t.Fatal("final checkpoint missing", e)
	}
}
func TestAgentExplicitToolBudgetAndRoundBudgetStillCheckpoint(t *testing.T) {
	for _, mode := range []string{"tools", "rounds"} {
		t.Run(mode, func(t *testing.T) {
			ws := budgetFixture(t)
			ws.RunBudget = &yyAgentRunBudget{MaxMinutes: 60}
			if mode == "tools" {
				ws.RunBudget.MaxTools = 2
			} else {
				ws.RunBudget.MaxRounds = 2
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				budgetToolResponse(w, calls, "project_list", `{}`)
			}))
			defer server.Close()
			ws.run(context.Background(), budgetModel(server.URL))
			if ws.State != "等待继续" {
				t.Fatal(ws.State, ws.Error)
			}
			max := 2
			if mode == "tools" {
				max = 3
			}
			if calls != max {
				t.Fatal("budget not enforced", calls)
			}
		})
	}
}
func TestAgentStopCancelsPendingModelAndPersists(t *testing.T) {
	ws := budgetFixture(t)
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		close(entered)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { ws.run(ctx, budgetModel(server.URL)); close(done) }()
	<-entered
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stop left model request running")
	}
	if ws.State != "已暂停" {
		t.Fatal(ws.State, ws.Error)
	}
	if _, e := os.Stat(filepath.Join(yyAgentRoot, ws.ID, "task.json")); e != nil {
		t.Fatal(e)
	}
}
func TestAgentBudgetDefaultsAndValidation(t *testing.T) {
	b := yyEffectiveRunBudget(nil)
	if b.MaxRounds != 0 || b.MaxTools != 0 || b.MaxMinutes != 60 {
		t.Fatal(b)
	}
	for _, v := range []yyAgentRunBudget{{MaxRounds: -1, MaxMinutes: 60}, {MaxTools: -1, MaxMinutes: 60}, {MaxMinutes: 0}, {MaxMinutes: 1441}} {
		if v.validate() == nil {
			t.Fatal("invalid budget accepted", v)
		}
	}
}

func TestAgentRunBudgetPersistsViaAPIAndSkillIsOptional(t *testing.T) {
	ws := budgetFixture(t)
	yyWorkspaces.Lock()
	old := yyWorkspaces.items[ws.ID]
	yyWorkspaces.items[ws.ID] = ws
	yyWorkspaces.Unlock()
	t.Cleanup(func() {
		yyWorkspaces.Lock()
		if old == nil {
			delete(yyWorkspaces.items, ws.ID)
		} else {
			yyWorkspaces.items[ws.ID] = old
		}
		yyWorkspaces.Unlock()
	})
	ws.Skills = nil
	ws.AllowedTools = nil
	if text, bad := yyWorkspaceTool(context.Background(), ws, "load_skill", map[string]any{"skill": "efficient-context", "max_chars": 3000}); bad || !strings.Contains(text, "Token") {
		t.Fatal(text, bad)
	}
	ws.Skills = []string{}
	if _, bad := yyWorkspaceTool(context.Background(), ws, "load_skill", map[string]any{"skill": "efficient-context"}); !bad {
		t.Fatal("explicit opt out ignored")
	}
	ws.Lock()
	if e := ws.saveLocked(); e != nil {
		t.Fatal(e)
	}
	ws.Unlock()
	mux := http.NewServeMux()
	yyAgentBudgetRoutes(mux)
	body := []byte(`{"id":"budget_test","budget":{"max_rounds":12,"max_tools":80,"max_minutes":90}}`)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/youyuan/agent/run-budget", bytes.NewReader(body)))
	var result struct {
		OK     bool             `json:"ok"`
		Budget yyAgentRunBudget `json:"budget"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil || !result.OK || result.Budget.MaxTools != 80 {
		t.Fatal(w.Body.String(), e)
	}
	data, e := os.ReadFile(filepath.Join(yyAgentRoot, ws.ID, "task.json"))
	if e != nil {
		t.Fatal(e)
	}
	reloaded := &yyWorkspace{}
	if e = json.Unmarshal(data, reloaded); e != nil {
		t.Fatal(e)
	}
	yyWorkspaces.Lock()
	yyWorkspaces.items[ws.ID] = reloaded
	yyWorkspaces.Unlock()
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/youyuan/agent/run-budget?id=budget_test", nil))
	if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil || result.Budget.MaxMinutes != 90 {
		t.Fatal(w.Body.String(), e)
	}
}
