package main

import (
	"strings"
	"testing"
	"time"
)

// 验证 Lua 插件注册触发词后，聊天触发时反向调用 Lua handler。
func TestLuaRegisterTriggerCallback(t *testing.T) {
	c := &captureHook{}
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { c.add(p + "|" + txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	bm := &BotManager{}
	src := `
		game.registerTrigger({"测试词"}, "onTest")
		function onTest(player, args)
			game.sendTargeted(player, "收到:" .. args[1])
		end
	`
	if err := LoadLuaPluginSource(bm, src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	bm.handleChat("alice", "测试词 值")
	// handleChat 异步派发 Func，轮询等待 handler 完成
	deadline := time.Now().Add(2 * time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.all(); len(got) != 1 || !strings.Contains(got[0], "收到:值") {
		t.Fatalf("handler 未正确调用: %v", got)
	}
}

// 验证 Lua 触发词收到触发词后的全部参数（RawArgs），如方块测试 3 个坐标。
func TestLuaRegisterTriggerMultiArgs(t *testing.T) {
	c := &captureHook{}
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { c.add(txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	bm := &BotManager{}
	src := `
		game.registerTrigger({"方块测试"}, "onBlock")
		function onBlock(player, args)
			game.sendTargeted(player, args[1] .. "," .. args[2] .. "," .. args[3])
		end
	`
	if err := LoadLuaPluginSource(bm, src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	bm.handleChat("alice", "方块测试 289 -64 304")
	deadline := time.Now().Add(2 * time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.all(); len(got) != 1 || !strings.Contains(got[0], "289,-64,304") {
		t.Fatalf("多参数未正确传递: %v", got)
	}
}
