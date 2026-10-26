package main

import (
	"runtime"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// 验证 system 表绑定存在且 system.platform 在桌面返回 "desktop"。
func TestLuaSystemAPIBound(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	for _, fn := range []string{"platform", "call", "shell", "shizukuStatus", "shizukuRequest"} {
		if L.GetField(L.GetGlobal("system"), fn).Type() == lua.LTNil {
			t.Fatalf("system.%s 未绑定", fn)
		}
	}
	var got string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = txt; return nil })
	defer func() { setSendTargetedHook(old) }()
	err := L.DoString(`game.sendTargeted("a", system.platform())`)
	if err != nil {
		t.Fatalf("platform 调用失败: %v", err)
	}
	if got != runtime.GOOS {
		t.Fatalf("platform 返回 %q 期望 %s", got, runtime.GOOS)
	}
}

// 验证触发词回调的临时 VM 里也注册了 system 表（regression：曾只注册 game，导致
// handler 里 system.platform() 报 nil）。桌面返回 "desktop"。
func TestLuaSystemAvailableInTriggerHandler(t *testing.T) {
	c := &captureHook{}
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { c.add(p + "|" + txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	bm := &BotManager{}
	src := `
		game.registerTrigger({"系统测试"}, "onTest")
		function onTest(player, args)
			game.sendTargeted(player, "平台=" .. system.platform())
		end
	`
	if err := LoadLuaPluginSource(bm, src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	bm.handleChat("alice", "系统测试")
	deadline := time.Now().Add(2 * time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.all(); len(got) != 1 || !strings.Contains(got[0], "平台="+runtime.GOOS) {
		t.Fatalf("handler 未正确使用 system 表: %v", got)
	}
}

// 验证桌面环境 system.call / system.shell 触发 RaiseError（stub 返回不支持）。
func TestLuaSystemCallDesktopUnsupported(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	for _, code := range []string{
		`system.call("shell", {command="echo hi"})`,
		`system.shell("echo hi")`,
	} {
		err := L.DoString(code)
		if err == nil {
			t.Fatalf("%s 在桌面应报错，实际无错误", code)
		}
		if !strings.Contains(err.Error(), "桌面环境不支持") {
			t.Fatalf("%s 报错内容不符: %v", code, err)
		}
	}
}
