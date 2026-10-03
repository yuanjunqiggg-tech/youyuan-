package main

import (
	lua "github.com/yuin/gopher-lua"
	"testing"
)

// 验证 newLuaState 创建带 game API 的 Lua 状态。
func TestNewLuaState(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	if L.GetGlobal("game").Type() != lua.LTTable {
		t.Fatal("期望 game 表已注册")
	}
}

// 验证 game.sendCommand 走 sendCommandHook（可替换，便于测试）。
func TestLuaSendCommandHook(t *testing.T) {
	var got []string
	old := sendCommandHook
	setSendCommandHook(func(cmd string) error { got = append(got, cmd); return nil })
	defer func() { setSendCommandHook(old) }()

	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	if err := L.DoString(`game.sendCommand("say hi")`); err != nil {
		t.Fatalf("Lua 执行失败: %v", err)
	}
	if len(got) != 1 || got[0] != "say hi" {
		t.Fatalf("hook 未捕获: %v", got)
	}
}
