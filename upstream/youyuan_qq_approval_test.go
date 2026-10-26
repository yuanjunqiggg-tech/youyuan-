package main

// 群内危险指令授权的测试。
// 这些用例直接读全局量（yyQQ / yyAppr），每个用例用 t.Cleanup 还原，
// 并且把 yyApprExec 换成假实现，绝不真的往游戏里发指令。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func yyApprTestIsolate(t *testing.T) {
	t.Helper()

	yyAppr.Lock()
	savedPath := yyAppr.Path
	savedItems := yyAppr.Items
	savedLast := yyAppr.Last
	yyAppr.Path = ""
	yyAppr.Items = nil
	yyAppr.Last = map[string][]yyApprLast{}
	yyAppr.Unlock()

	savedExec := yyApprExec
	yyApprExec = nil

	yyQQMu.Lock()
	savedConfig := yyQQ
	yyQQ = yyQQConfig{}
	yyQQ.GroupID = "100200"
	yyQQ.Assistant.MCPToken = yySrvInfoTestToken
	yyQQ.Assistant.SuperAdmins = []string{"999"}
	yyQQ.Assistant.Admins = []string{"777"}
	yyQQMu.Unlock()

	t.Cleanup(func() {
		yyAppr.Lock()
		yyAppr.Path = savedPath
		yyAppr.Items = savedItems
		yyAppr.Last = savedLast
		yyAppr.Unlock()
		yyApprExec = savedExec

		yyQQMu.Lock()
		yyQQ = savedConfig
		yyQQMu.Unlock()
	})
}

// yyApprTestHook 装一个只记录、不执行的钩子。
func yyApprTestHook(t *testing.T, runs *[]string) {
	t.Helper()
	yyApprExec = func(item yyApprItem) (string, error) {
		*runs = append(*runs, item.Payload)
		return "已执行（测试）", nil
	}
}

// 用例 1：非超级管理员（含管理员）批准必须被拒，且待决项仍然挂着。
func TestApprNonSuperAdminCannotApprove(t *testing.T) {
	yyApprTestIsolate(t)
	var runs []string
	yyApprTestHook(t, &runs)

	cf := yyQQSnapshot()
	item, e := yyApprRequest(cf, "100200", "777", "管理员A", yyApprKindCommand, "op Steve")
	if e != nil {
		t.Fatalf("建授权项失败：%v", e)
	}
	if e = yyApprDecide(item.ID, "777", "管理员A", true); e == nil {
		t.Fatalf("管理员不应能批准危险指令")
	} else if !strings.Contains(e.Error(), "超级管理员") {
		t.Fatalf("拒绝原因应说明只有超级管理员能批，实际 %q", e.Error())
	}
	if len(runs) != 0 {
		t.Fatalf("被拒的批准不能执行任何指令，实际执行了 %v", runs)
	}

	// 群里回「同意 编号」：非超管应收到一句人话，而不是静默执行。
	handled, reply := yyApprTake("100200", "777", "同意 "+item.ID)
	if !handled {
		t.Fatalf("管理员回「同意 编号」应被接管并给出说明")
	}
	if !strings.Contains(reply, "超级管理员") {
		t.Fatalf("说明里应提到超级管理员，实际 %q", reply)
	}
	if p := yyApprPending("100200"); len(p) != 1 || p[0].Status != "pending" {
		t.Fatalf("被拒后待决项应保持 pending，实际 %+v", p)
	}
	if len(runs) != 0 {
		t.Fatalf("非超管批准后不应执行，实际 %v", runs)
	}
}

// 用例 2：超级管理员批准后真的执行，并留下「上次执行」供重放。
func TestApprSuperAdminApproveExecutes(t *testing.T) {
	yyApprTestIsolate(t)
	var runs []string
	yyApprTestHook(t, &runs)

	cf := yyQQSnapshot()
	item, e := yyApprRequest(cf, "100200", "777", "管理员A", yyApprKindCommand, "op Steve")
	if e != nil {
		t.Fatalf("建授权项失败：%v", e)
	}
	handled, reply := yyApprTake("100200", "999", "同意 "+item.ID)
	if !handled {
		t.Fatalf("超管回「同意 编号」应被接管")
	}
	if !strings.Contains(reply, "已批准") {
		t.Fatalf("回执应说明已批准，实际 %q", reply)
	}
	if len(runs) != 1 || runs[0] != "op Steve" {
		t.Fatalf("批准后应执行一次原指令，实际 %v", runs)
	}
	if last, ok := yyApprLastOf("100200", "777"); !ok || last.Payload != "op Steve" {
		t.Fatalf("批准执行后应记录上次执行，实际 %+v ok=%v", last, ok)
	}
	if e = yyApprDecide(item.ID, "999", "服主", true); e == nil {
		t.Fatalf("同一条不能被批准两次")
	}
	if len(runs) != 1 {
		t.Fatalf("重复批准不应重复执行，实际 %v", runs)
	}
}

// 用例 3：过期项不能批准，状态要变成 expired。
func TestApprExpiredCannotApprove(t *testing.T) {
	yyApprTestIsolate(t)
	var runs []string
	yyApprTestHook(t, &runs)

	cf := yyQQSnapshot()
	item, e := yyApprRequest(cf, "100200", "777", "管理员A", yyApprKindCommand, "ban Steve")
	if e != nil {
		t.Fatalf("建授权项失败：%v", e)
	}
	yyAppr.Lock()
	for i := range yyAppr.Items {
		if yyAppr.Items[i].ID == item.ID {
			yyAppr.Items[i].ExpiresAt = time.Now().Add(-time.Second)
		}
	}
	yyAppr.Unlock()

	if e = yyApprDecide(item.ID, "999", "服主", true); e == nil {
		t.Fatalf("过期项不应能批准")
	} else if !strings.Contains(e.Error(), "过期") {
		t.Fatalf("拒绝原因应说明过期，实际 %q", e.Error())
	}
	if len(runs) != 0 {
		t.Fatalf("过期项不能执行，实际 %v", runs)
	}
	yyAppr.Lock()
	status := ""
	for _, it := range yyAppr.Items {
		if it.ID == item.ID {
			status = it.Status
		}
	}
	yyAppr.Unlock()
	if status != "expired" {
		t.Fatalf("过期项状态应为 expired，实际 %q", status)
	}
}

// 用例 4：`.再来一次` 重放上一次成功的指令；上次是危险指令时仍然要授权。
func TestApprRepeatLastCommand(t *testing.T) {
	yyApprTestIsolate(t)
	var runs []string
	yyApprTestHook(t, &runs)

	cf := yyQQSnapshot()
	first := strings.Join(yyQQAssistantCommand(cf, "管理员A", "777", "/命令 say hello", "m1"), "\n")
	if !strings.Contains(first, "已执行") {
		t.Fatalf("普通指令应直接执行，实际 %q", first)
	}
	if len(runs) != 1 || runs[0] != "say hello" {
		t.Fatalf("第一次应执行 say hello，实际 %v", runs)
	}

	again := strings.Join(yyQQAssistantCommand(cf, "管理员A", "777", ".再来一次", "m2"), "\n")
	if len(runs) != 2 || runs[1] != "say hello" {
		t.Fatalf("`.再来一次` 应重放 say hello，实际 %v（回执 %q）", runs, again)
	}

	// 把危险指令记为上次执行：重放时必须重新走授权，不能直接执行。
	yyApprRemember("100200", "777", yyApprKindCommand, "op Steve")
	out := strings.Join(yyQQAssistantCommand(cf, "管理员A", "777", ".再来一次", "m3"), "\n")
	if len(runs) != 2 {
		t.Fatalf("危险指令重放不应直接执行，实际 %v", runs)
	}
	if !strings.Contains(out, "同意") {
		t.Fatalf("危险指令重放应给出授权提示，实际 %q", out)
	}
	if p := yyApprPending("100200"); len(p) != 1 || !strings.Contains(p[0].Payload, "op") {
		t.Fatalf("应生成一条 op 的待授权项，实际 %+v", p)
	}
}

// 用例 5：危险指令在群里走授权，不会直接执行；面板 say 也一样。
func TestApprDangerousCommandAsksInsteadOfRunning(t *testing.T) {
	yyApprTestIsolate(t)
	var runs []string
	yyApprTestHook(t, &runs)

	cf := yyQQSnapshot()
	out := strings.Join(yyQQAssistantCommand(cf, "管理员A", "777", "/命令 fill 0 0 0 200 200 200 stone", "m1"), "\n")
	if len(runs) != 0 {
		t.Fatalf("危险指令不能直接执行，实际 %v", runs)
	}
	if !strings.Contains(out, "同意") || !strings.Contains(out, "拒绝") {
		t.Fatalf("应提示「同意/拒绝 编号」，实际 %q", out)
	}
	pending := yyApprPending("100200")
	if len(pending) != 1 {
		t.Fatalf("应生成 1 条待授权项，实际 %+v", pending)
	}
	if !strings.Contains(pending[0].Payload, "fill") || pending[0].Status != "pending" {
		t.Fatalf("待授权项内容不对：%+v", pending[0])
	}

	// 面板的 /api/qq/server/say 同样只挂起、不执行。
	yyAppr.Lock()
	yyAppr.Items = nil
	yyAppr.Unlock()
	mux := http.NewServeMux()
	yySrvInfoRoutes(mux)
	req := httptest.NewRequest("POST", "/api/qq/server/say", strings.NewReader(`{"text":"/op someone","mode":"command"}`))
	req.Header.Set("X-Bridge-Token", yySrvInfoTestToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var body map[string]any
	if e := json.Unmarshal(rec.Body.Bytes(), &body); e != nil {
		t.Fatalf("say 返回的不是 JSON：%v（%s）", e, rec.Body.String())
	}
	if body["ok"] != false {
		t.Fatalf("危险指令不能立刻算成功，实际 %+v", body)
	}
	if body["pending"] != true || body["approval_id"] == nil {
		t.Fatalf("应返回待授权编号，实际 %+v", body)
	}
	if len(runs) != 0 {
		t.Fatalf("面板危险指令也不能直接执行，实际 %v", runs)
	}
}

// 用例 6：/api/qq/roles 能回答「谁是超管、谁是管理员」。
func TestApprRolesEndpoint(t *testing.T) {
	yyApprTestIsolate(t)

	yyQQHelper.Lock()
	saved := yyQQHelper.Bindings
	yyQQHelper.Bindings = map[string]string{"999": "Steve", "777": "Alex"}
	yyQQHelper.Unlock()
	t.Cleanup(func() {
		yyQQHelper.Lock()
		yyQQHelper.Bindings = saved
		yyQQHelper.Unlock()
	})

	mux := http.NewServeMux()
	yyApprRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/qq/roles", nil))

	var out struct {
		OK     bool        `json:"ok"`
		Supers []yyApprWho `json:"super_admins"`
		Admins []yyApprWho `json:"admins"`
	}
	if e := json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
		t.Fatalf("roles 返回的不是 JSON：%v（%s）", e, rec.Body.String())
	}
	if !out.OK || len(out.Supers) != 1 || len(out.Admins) != 1 {
		t.Fatalf("应各有 1 个超管/管理员，实际 %+v", out)
	}
	if out.Supers[0].QQ != "999" || out.Supers[0].Name != "Steve" {
		t.Fatalf("超管应反查出 Steve，实际 %+v", out.Supers[0])
	}
	if out.Admins[0].QQ != "777" || out.Admins[0].Name != "Alex" {
		t.Fatalf("管理员应反查出 Alex，实际 %+v", out.Admins[0])
	}
}

// 用例 7：队列落盘后重新载入，未决项还在（重启不丢）。
func TestApprQueueSurvivesReload(t *testing.T) {
	yyApprTestIsolate(t)
	dir := t.TempDir()

	yyApprInit(dir)
	cf := yyQQSnapshot()
	item, e := yyApprRequest(cf, "100200", "777", "管理员A", yyApprKindCommand, "deop Steve")
	if e != nil {
		t.Fatalf("建授权项失败：%v", e)
	}
	yyApprRemember("100200", "777", yyApprKindCommand, "say hello")

	yyApprInit(dir) // 模拟重启
	pending := yyApprPending("100200")
	if len(pending) != 1 || pending[0].ID != item.ID {
		t.Fatalf("重载后未决项应还在，实际 %+v", pending)
	}
	if last, ok := yyApprLastOf("100200", "777"); !ok || last.Payload != "say hello" {
		t.Fatalf("重载后上次执行应还在，实际 %+v ok=%v", last, ok)
	}
}
