package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 这些用例来自 2026-10-23 的安全审查（H1/H2/H3 + 中危 panic）。
// 目的：把「曾经能绕过」的路径钉死，回归时立刻报错。

// H1/H2：emit 与规则写入必须带令牌；令牌为空时一律拒绝。
func TestBridgeRejectsWithoutToken(t *testing.T) {
	saved := yyQQSnapshot()
	t.Cleanup(func() {
		yyQQMu.Lock()
		yyQQ.Assistant.MCPToken = saved.Assistant.MCPToken
		yyQQMu.Unlock()
	})
	yyQQMu.Lock()
	yyQQ.Assistant.MCPToken = "test-token-123"
	yyQQMu.Unlock()

	mux := http.NewServeMux()
	yyBridgeRoutes(mux, t.TempDir())

	post := func(path, body string, token string) (int, map[string]any) {
		req := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("X-Bridge-Token", token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	// 无令牌：emit 被拒
	if _, out := post("/api/bridge/emit", `{"event":"player.arm_swing","player":"Steve"}`, ""); out["ok"] == true {
		t.Fatal("无令牌的 emit 必须被拒绝")
	}
	// 无令牌但有 127.0.0.1 也不能放行（H2：中继后 RemoteAddr 恒为 127.0.0.1）
	req := httptest.NewRequest("POST", "/api/bridge/emit", bytes.NewBufferString(`{"event":"x"}`))
	req.RemoteAddr = "127.0.0.1:5555"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["ok"] == true {
		t.Fatal("令牌为空时不能靠 RemoteAddr 放行")
	}

	// 无令牌：写规则被拒
	rule := `{"rules":[{"name":"evil","enabled":true,"event":"x","threshold":1,"action":{"type":"game_command","command":"op Steve"}}]}`
	if _, out := post("/api/bridge/rules", rule, ""); out["ok"] == true {
		t.Fatal("无令牌写规则必须被拒绝")
	}
	// 带令牌：可以写（但指令仍受守卫约束，见下一个用例）
	if _, out := post("/api/bridge/rules", rule, "test-token-123"); out["ok"] != true {
		t.Fatalf("带令牌应能写规则：%v", out)
	}
	// 无令牌：删除规则被拒
	if _, out := post("/api/bridge/rules/delete?id=whatever", "", ""); out["ok"] == true {
		t.Fatal("无令牌删规则必须被拒绝")
	}
	// 错误令牌也不行
	if _, out := post("/api/bridge/emit", `{"event":"x"}`, "wrong"); out["ok"] == true {
		t.Fatal("错误令牌必须被拒绝")
	}
}

// H3：命名空间前缀与 execute 包壳都不能绕过危险命令黑名单。
func TestSrvInfoGuardBlocksNamespaceBypass(t *testing.T) {
	blocked := []string{
		"/minecraft:op Steve",
		"/MINECRAFT:OP Steve",
		"/execute as @a run minecraft:deop Steve",
		"/execute as @a at @s run minecraft:give Steve diamond 64",
		"/gamemode creative Steve",
		"/tp Steve 0 100 0",
		"/kill @e[type=item]",
		"/setblock 0 0 0 stone",
		"/reload",
		"/save-off",
	}
	for _, cmd := range blocked {
		if e := yySrvInfoCommandGuard(cmd); e == nil {
			t.Fatalf("应当拦截：%s", cmd)
		}
	}
	// 正常查询类指令要放行，否则面板就没法用了
	for _, cmd := range []string{"/list", "/tps", "/say hello", "/weather clear"} {
		if e := yySrvInfoCommandGuard(cmd); e != nil {
			t.Fatalf("不应拦截 %s：%v", cmd, e)
		}
	}
	// 换行拼接必须拦
	if e := yySrvInfoCommandGuard("/list\n/op Steve"); e == nil {
		t.Fatal("换行拼接必须拦截")
	}
}

// 中危：min_level 越界不能让能力接口 panic。
func TestCapsViewSurvivesOutOfRangeMinLevel(t *testing.T) {
	bad := 9
	a := yyQQAssistantConfig{CapLimits: map[string]yyQQCapLimit{"like": {MinLevel: &bad}}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("越界 min_level 导致 panic：%v", r)
		}
	}()
	rows := yyQQCapsView(a)
	found := false
	for _, row := range rows {
		if row["kind"] == "like" {
			found = true
			if row["min_level"] != 2 {
				t.Fatalf("越界值应被夹到 2，实际 %v", row["min_level"])
			}
			if row["min_level_text"] != "超级管理员" {
				t.Fatalf("角色名不对：%v", row["min_level_text"])
			}
		}
	}
	if !found {
		t.Fatal("没找到 like 能力")
	}
}

// 中危：参数缺失不能变成字面量 "<nil>"。
func TestBridgeCallMissingParamIsEmpty(t *testing.T) {
	saved := yyQQSnapshot()
	t.Cleanup(func() {
		yyQQMu.Lock()
		yyQQ.Assistant.MCPToken = saved.Assistant.MCPToken
		yyQQMu.Unlock()
	})
	yyQQMu.Lock()
	yyQQ.Assistant.MCPToken = "test-token-123"
	yyQQMu.Unlock()

	mux := http.NewServeMux()
	yyBridgeRoutes(mux, t.TempDir())
	req := httptest.NewRequest("POST", "/api/bridge/call", bytes.NewBufferString(`{"capability":"qq.send","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bridge-Token", "test-token-123")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["ok"] == true {
		t.Fatal("空 text 不应该发出去")
	}
	msg := strings.TrimSpace(toString(out["error"]))
	if strings.Contains(msg, "<nil>") {
		t.Fatalf("错误信息里出现了 <nil>：%s", msg)
	}
	if !strings.Contains(msg, "text") {
		t.Fatalf("应提示缺 text：%s", msg)
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
