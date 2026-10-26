package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// 危险指令名单是玩家的服务器、玩家的开关：
// 默认 31 条照拦；默认锁定的 6 条玩家显式关也尊重；自定义能加；脏输入被规范化。
func yyGuardTestSet(t *testing.T, disabled, extra []string) {
	t.Helper()
	saved := yyQQSnapshot()
	t.Cleanup(func() {
		yyQQMu.Lock()
		yyQQ.Assistant.DangerDisabled = saved.Assistant.DangerDisabled
		yyQQ.Assistant.DangerExtra = saved.Assistant.DangerExtra
		yyQQMu.Unlock()
	})
	yyQQMu.Lock()
	yyQQ.Assistant.DangerDisabled = disabled
	yyQQ.Assistant.DangerExtra = extra
	yyQQMu.Unlock()
}

// 默认配置（老配置没有这些字段也一样）里 31 条默认名单照拦，含 6 条默认锁定项。
func TestDangerDefaultBlocksLocked(t *testing.T) {
	yyGuardTestSet(t, nil, nil)
	if e := yySrvInfoCommandGuard("/op Steve"); e == nil {
		t.Fatal("默认应拦住 op")
	}
	if e := yySrvInfoCommandGuard("/gamemode creative"); e == nil {
		t.Fatal("默认应拦住 gamemode")
	}
}

// 普通规则关掉即放行，且不影响其他规则。
func TestDangerDisabledRuleAllows(t *testing.T) {
	yyGuardTestSet(t, []string{"gamemode "}, nil)
	if e := yySrvInfoCommandGuard("/gamemode creative"); e != nil {
		t.Fatalf("关掉 gamemode 后应放行：%v", e)
	}
	if e := yySrvInfoCommandGuard("/give Steve diamond"); e == nil {
		t.Fatal("give 仍应被拦")
	}
}

// 默认锁定的 6 条不再锁死：玩家显式写进 DangerDisabled 就放行。
func TestDangerLockedCanBeDisabled(t *testing.T) {
	yyGuardTestSet(t, []string{"op ", "stop", "ban "}, nil)
	for _, cmd := range []string{"/op Steve", "/stop", "/ban Steve"} {
		if e := yySrvInfoCommandGuard(cmd); e != nil {
			t.Fatalf("玩家显式关掉锁定项后应放行 %s：%v", cmd, e)
		}
	}
	// 只关了 op/stop/ban，ban-ip 没关仍应被拦
	if e := yySrvInfoCommandGuard("/ban-ip 1.2.3.4"); e == nil {
		t.Fatal("ban-ip 未被关掉，仍应被拦")
	}
}

// 自定义前缀照常生效。
func TestDangerExtraPrefixBlocks(t *testing.T) {
	yyGuardTestSet(t, nil, []string{"title "})
	if e := yySrvInfoCommandGuard("/title Steve title hi"); e == nil {
		t.Fatal("自定义前缀应生效")
	}
	if e := yySrvInfoCommandGuard("/list"); e != nil {
		t.Fatalf("正常查询不该被拦：%v", e)
	}
}

// 接口层 round-trip：POST 把关锁定项写进配置 -> GET 读回 enabled=false -> 判定放行。
func TestDangerEndpointRoundTrip(t *testing.T) {
	saved := yyQQSnapshot()
	t.Cleanup(func() {
		yyQQMu.Lock()
		yyQQ.Assistant.DangerDisabled = saved.Assistant.DangerDisabled
		yyQQ.Assistant.DangerExtra = saved.Assistant.DangerExtra
		yyQQMu.Unlock()
	})
	// 保存接口会写 qq-bridge.json，测试里指到临时目录，别碰真实配置。
	oldPath := yyQQPath
	yyQQPath = filepath.Join(t.TempDir(), "qq-bridge.json")
	t.Cleanup(func() { yyQQPath = oldPath })

	mux := http.NewServeMux()
	yySrvInfoRoutes(mux)

	body := `{"disabled":["op ","gamemode "],"extra":["TITLE ","title ",""]}`
	req := httptest.NewRequest("POST", "/api/qq/guard", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out map[string]any
	if e := json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
		t.Fatalf("返回不是 JSON：%v", e)
	}
	if out["ok"] != true {
		t.Fatalf("应保存成功：%+v", out)
	}
	// 大小写与空白被规范化、重复被去掉
	extra, _ := out["extra"].([]any)
	if len(extra) != 1 || extra[0] != "title" {
		t.Fatalf("extra 应规范成 [title]：%+v", out["extra"])
	}
	// 写配置 -> 判定：op 已被玩家关掉，不再拦截
	if e := yySrvInfoCommandGuard("/op Steve"); e != nil {
		t.Fatalf("保存后 op 应放行：%v", e)
	}
	if e := yySrvInfoCommandGuard("/gamemode creative"); e != nil {
		t.Fatalf("保存后 gamemode 应放行：%v", e)
	}

	// 读配置：GET 回来要能看到 op 仍是默认锁定标记，但 enabled=false
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest("GET", "/api/qq/guard", nil))
	var got map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &got)
	rows, _ := got["rules"].([]any)
	found := false
	for _, row := range rows {
		m, _ := row.(map[string]any)
		if m["prefix"] == "op " {
			found = true
			if m["locked"] != true || m["enabled"] != false {
				t.Fatalf("op 应 locked=true 且 enabled=false：%+v", m)
			}
		}
	}
	if !found {
		t.Fatal("GET 没返回 op 规则")
	}
}
