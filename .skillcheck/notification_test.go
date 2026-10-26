package main

import (
	"testing"
)

// TestDispatchNewPushMessages 验证分发器：一次性/toast 与 command 分发后即 ack，
// 常驻 banner 保留；command 动作入队待机器人连接后执行。
func TestDispatchNewPushMessages(t *testing.T) {
	notifMu.Lock()
	emittedPush = map[string]bool{}
	pendingBotMsgs = nil
	notifMu.Unlock()

	dataDir := t.TempDir()
	msgs := []PushMessage{
		{ID: "p1", Type: "admin_push", DisplayMode: "toast", Title: "t", Body: "b"},
		{ID: "p2", Type: "announcement", DisplayMode: "banner", Title: "t", Body: "b"},
		{ID: "p3", Type: "command", Action: "command", ActionData: "say hi"},
	}
	savePushMessages(dataDir, msgs)

	dispatchNewPushMessages(dataDir)

	// 一次性/静默已 ack，只剩常驻 banner
	remaining := loadPushMessages(dataDir)
	if len(remaining) != 1 || remaining[0].ID != "p2" {
		var ids []string
		for _, m := range remaining {
			ids = append(ids, m.ID)
		}
		t.Fatalf("期望只剩常驻 banner p2，得到 %v", ids)
	}

	// command 动作入队（未连接时积压，连接后执行）
	notifMu.Lock()
	q := append([]queuedBotMsg{}, pendingBotMsgs...)
	emitted := emittedPush["p2"]
	notifMu.Unlock()
	if len(q) != 1 || q[0].kind != "cmd" || q[0].payload != "say hi" {
		t.Fatalf("期望指令 'say hi' 入队，得到 %v", q)
	}
	if !emitted {
		t.Fatal("期望常驻 p2 已被标记 emitted（避免重复弹）")
	}
}

// TestResolveDisplayMode 验证 display_mode 空时的兼容回退。
func TestResolveDisplayMode(t *testing.T) {
	cases := []struct {
		pm   PushMessage
		want string
	}{
		{PushMessage{DisplayMode: "modal"}, "modal"},
		{PushMessage{Type: "command"}, "none"},
		{PushMessage{Type: "announcement"}, "banner"},
		{PushMessage{Type: "version_update"}, "popup"},
		{PushMessage{Type: "admin_push"}, "toast"},
	}
	for _, c := range cases {
		if got := resolveDisplayMode(c.pm); got != c.want {
			t.Errorf("resolveDisplayMode(%+v) = %q, want %q", c.pm, got, c.want)
		}
	}
}

// TestIsTransient 验证哪些模式分发后即 ack。
func TestIsTransient(t *testing.T) {
	transient := []string{"toast", "popup", "marquee", "chat", "tellraw", "none"}
	for _, m := range transient {
		if !isTransient(m) {
			t.Errorf("isTransient(%q) = false, want true", m)
		}
	}
	if isTransient("banner") || isTransient("modal") {
		t.Error("banner/modal 不应自动 ack")
	}
}
