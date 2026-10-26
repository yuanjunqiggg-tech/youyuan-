package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bot-apk/state"
)

// TestDrawApplyRejectedWhenTaskBusy 验证作业串行的「反方向」：
// 导入/导出正在跑时，画板落地与同步必须被拒绝，否则两边同时往服务器发指令
// 会互相覆盖。
func TestDrawApplyRejectedWhenTaskBusy(t *testing.T) {
	res := doDrawReq(t, handleDrawCreate, "POST",
		`{"x1":0,"z1":0,"w":4,"d":4,"y":64,"mode":"transparent","sync":"batch"}`)
	if res["ok"] != true {
		t.Fatalf("建画板失败: %v", res)
	}

	// 造一个「正在跑」的作业
	taskMu.Lock()
	prev := activeTask
	activeTask = state.NewTaskController(hub)
	taskMu.Unlock()
	defer func() {
		taskMu.Lock()
		activeTask = prev
		taskMu.Unlock()
	}()

	for _, tc := range []struct {
		name string
		h    http.HandlerFunc
	}{
		{"apply", handleDrawApply},
		{"sync", handleDrawSync},
	} {
		out := doDrawReq(t, tc.h, "POST", "")
		if out["ok"] == true {
			t.Fatalf("%s：有作业在跑时应被拒绝，实际 ok=true", tc.name)
		}
		if msg := fmt.Sprint(out["error"]); !strings.Contains(msg, "正在进行") {
			t.Fatalf("%s：错误信息应提示作业正在进行，实际 %q", tc.name, msg)
		}
	}
}

// TestTaskStartRejectedWhenDrawPlacing 验证作业串行的「正方向」：
// 画板正在往服务器落方块时，不允许再起新的导入/导出作业。
func TestTaskStartRejectedWhenDrawPlacing(t *testing.T) {
	res := doDrawReq(t, handleDrawCreate, "POST",
		`{"x1":0,"z1":0,"w":4,"d":4,"y":64,"mode":"transparent","sync":"batch"}`)
	if res["ok"] != true {
		t.Fatalf("建画板失败: %v", res)
	}
	sess := currentDrawSession()
	if sess == nil {
		t.Fatal("画板会话应为非空")
	}
	sess.mu.Lock()
	sess.placing = true
	sess.mu.Unlock()
	defer func() {
		sess.mu.Lock()
		sess.placing = false
		sess.mu.Unlock()
	}()

	req := httptest.NewRequest("POST", "/api/task", strings.NewReader(`{"type":"export","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleTaskStart(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "画板正在放置方块") {
		t.Fatalf("画板放置中起作业应被拒绝，实际响应: %s", body)
	}
}
