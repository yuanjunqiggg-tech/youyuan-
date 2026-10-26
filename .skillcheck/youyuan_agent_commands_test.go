package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestYouYuanTerminalChainPreservesDecimalsAndJSON(t *testing.T) {
	cases := map[string][]string{
		"/tp @s 1.5 64 -2.75./say ok.":                          {"/tp @s 1.5 64 -2.75", "/say ok"},
		`/tellraw @a {"rawtext":[{"text":"a./b."}]}./say done.`: {`/tellraw @a {"rawtext":[{"text":"a./b."}]}`, "/say done"},
		"/say ordinary command":                                 {"/say ordinary command"}, "hello.": {"hello."},
	}
	for input, want := range cases {
		if got := yySplitConsole(input); !reflect.DeepEqual(got, want) {
			t.Fatalf("%q got %v want %v", input, got, want)
		}
	}
}
func TestYouYuanCommandChainActualConditions(t *testing.T) {
	plan, e := yyParseChains("1,0,0\nsay fail\n0,1,0\nsay skipped\nsay inherited\n.\n1,0,0\nsay independent\n!\n2,0,0\nsay repeating\n0,1,0\nsay after repeat")
	if e != nil {
		t.Fatal(e)
	}
	if plan.Commands != 6 || len(plan.Chains) != 3 || plan.Periods != 2 {
		t.Fatalf("unexpected plan %+v", plan)
	}
	run := func(first bool) []string {
		var sent []string
		yyChainExecute(context.Background(), plan, first, false, false, 0, func(s string) (bool, error) { sent = append(sent, s); return !strings.Contains(s, "fail"), nil }, func(bool, bool, error) {})
		return sent
	}
	if got := run(true); !reflect.DeepEqual(got, []string{"say fail", "say independent", "say repeating", "say after repeat"}) {
		t.Fatal(got)
	}
	if got := run(false); !reflect.DeepEqual(got, []string{"say repeating", "say after repeat"}) {
		t.Fatal(got)
	}
	bad := []string{"3,0,0\nsay x", "1,2,0\nsay x"}
	for _, s := range bad {
		if _, e := yyParseChains(s); e == nil {
			t.Fatal("accepted invalid header")
		}
	}
}
func TestResidentFunctionSyntaxCompilesIntoPersistentChains(t *testing.T) {
	content := "#组:巡检 通道=玩家 速率=120 次数=2 前缀=execute\n1,0,0\nsay start\n0,1,0\nsay follow\n.\n#测试:问候 名字:≤Alex≥\nsay hello ≤名字≥\n#执行:问候 名字=Steve\n!\n#组:循环 通道=0\n2,0,0\nsay tick"
	plan, err := yyParseChains(content)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Commands != 4 || len(plan.Chains) != 3 || plan.Periods != 2 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Chains[0].Repeat != 2 || plan.Chains[0].Nodes[0].Channel != "player" || plan.Chains[0].Nodes[0].DelayMS != 120 || plan.Chains[0].Nodes[0].Command != "execute say start" {
		t.Fatalf("group settings lost: %+v", plan.Chains[0])
	}
	if plan.Chains[1].Nodes[0].Command != "execute say hello Steve" {
		t.Fatalf("test expansion lost: %+v", plan.Chains[1])
	}
	var sent []string
	yyChainExecuteDetailed(context.Background(), plan, true, false, false, 0, func(node yyChainNode) (bool, error) { sent = append(sent, node.Command); return true, nil }, func(bool, bool, error) {})
	if len(sent) != 7 {
		t.Fatalf("expected repeated groups and loop: %v", sent)
	}
}
func TestFunctionParametersPreserveSpaces(t *testing.T) {
	name, cfg := parseGroupConfig("批量 前缀=execute as @a run 次数=3 通道=玩家")
	if name != "批量" || cfg.Prefix != "execute as @a run" || cfg.LoopCount != 3 || cfg.Channel != "player" {
		t.Fatalf("group: %s %+v", name, cfg)
	}
	_, params := parseTestParams("放置 坐标:≤~ ~2 ~≥ 方块:≤stone≥")
	if params["坐标"] != "~ ~2 ~" || params["方块"] != "stone" {
		t.Fatalf("test params: %+v", params)
	}
	_, args := parseExecParams("放置 坐标=~ ~3 ~ 方块=diamond_block")
	if args["坐标"] != "~ ~3 ~" || args["方块"] != "diamond_block" {
		t.Fatalf("exec args: %+v", args)
	}
}
func TestYouYuanProjectIsolationConflictsAndRollback(t *testing.T) {
	previous := yyAgentRoot
	yyAgentRoot = t.TempDir()
	defer func() { yyAgentRoot = previous }()
	ws := &yyWorkspace{ID: "one", Name: "one"}
	other := &yyWorkspace{ID: "two"}
	rev, e := yyWriteProject(ws, "src/main.py", "x=1")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = yyReadProject(other, "src/main.py"); e == nil {
		t.Fatal("cross workspace leak")
	}
	if _, e = yyWriteProject(ws, "../escape", "bad"); e == nil {
		t.Fatal("accepted traversal")
	}
	base := yyFileHash("x=1")
	if _, e = yyWriteProject(ws, "src/main.py", "x=2"); e != nil {
		t.Fatal(e)
	}
	if _, e = yyWriteProjectIf(ws, "src/main.py", "overwrite", base); e == nil {
		t.Fatal("stale editor overwrote Agent update")
	}
	text, _ := yyReadProject(ws, "src/main.py")
	if text != "x=2" {
		t.Fatal(text)
	}
	_, bad := yyWorkspaceTool(context.Background(), ws, "project_rollback", map[string]any{"revision": rev.ID})
	if bad {
		t.Fatal("rollback failed")
	}
	if _, e = os.Stat(filepath.Join(yyWorkspaceRoot(ws), "src/main.py")); !os.IsNotExist(e) {
		t.Fatal("rollback of creation did not remove file")
	}
}
func TestYouYuanCustomMCPSessionsAndDisabledServer(t *testing.T) {
	var initialized, calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "notifications/initialized" {
			initialized.Add(1)
			w.WriteHeader(202)
			return
		}
		result := any(map[string]any{"tools": []any{}})
		if req.Method == "initialize" {
			w.Header().Set("Mcp-Session-Id", "session-test")
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "test", "version": "1"}}
		} else {
			if r.Header.Get("Mcp-Session-Id") != "session-test" {
				t.Error("missing session")
			}
			calls.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()
	old := yyAgentRoot
	yyAgentRoot = t.TempDir()
	defer func() { yyAgentRoot = old }()
	ws := &yyWorkspace{ID: "mcp_test", Servers: []yyMCPServer{{ID: "one", URL: server.URL, Enabled: true}}}
	if _, e := yyWorkspaceMCP(context.Background(), ws, "one", "tools/list", map[string]any{}); e != nil {
		t.Fatal(e)
	}
	if initialized.Load() != 1 || calls.Load() != 1 {
		t.Fatalf("initialize %d calls %d", initialized.Load(), calls.Load())
	}
	ws.Servers[0].Enabled = false
	if _, e := yyWorkspaceMCP(context.Background(), ws, "one", "tools/list", nil); e == nil {
		t.Fatal("disabled server called")
	}
}

// Protocol simulation verifies the real model/tool loop; this is not a live LLM test.
func TestYouYuanAgentModelProtocolEditsAndAcceptsSteering(t *testing.T) {
	old := yyAgentRoot
	yyAgentRoot = t.TempDir()
	defer func() { yyAgentRoot = old }()
	ws := &yyWorkspace{ID: "agent_test", Name: "Agent Test", Language: "python", State: "运行中", Turns: []aiTurn{{Role: "user", Text: "write main.py"}}}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			ws.Lock()
			ws.Pending = append(ws.Pending, "also retain state")
			ws.Unlock()
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"editing","tool_calls":[{"id":"call1","type":"function","function":{"name":"project_write","arguments":"{\"path\":\"main.py\",\"content\":\"counter=7\"}"}}]}}]}`)
			return
		}
		var req struct {
			Messages []map[string]any `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		data, _ := json.Marshal(req.Messages)
		if !strings.Contains(string(data), "also retain state") {
			t.Error("steering lost")
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"completed"}}]}`)
	}))
	defer server.Close()
	ws.run(context.Background(), &AIModel{Name: "protocol-test", Provider: ProviderOpenAI, BaseURL: server.URL, ModelID: "fake-model"})
	text, e := yyReadProject(ws, "main.py")
	if e != nil || text != "counter=7" {
		t.Fatalf("%s %v", text, e)
	}
	if ws.State != "已完成" || calls.Load() != 2 {
		t.Fatalf("%s %s calls %d", ws.State, ws.Error, calls.Load())
	}
	saved, e := os.ReadFile(filepath.Join(yyAgentRoot, ws.ID, "task.json"))
	if e != nil || !strings.Contains(string(saved), "also retain state") {
		t.Fatal("checkpoint missing")
	}
}
