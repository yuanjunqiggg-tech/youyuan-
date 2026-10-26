package main

import (
	"strings"
	"testing"
)

// 验证 game.getScore 绑定可调用（走 sendCommandHook 捕获的指令不可测，这里用 hook 验证指令发出）。
// 说明：getScore 走 bm.GetScore → SendWSCommandWithResp，单测难。改为验证绑定存在且 Lua 可调用。
func TestLuaGameAPIBound(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	// 验证 game 表里有这些便捷函数
	for _, fn := range []string{"getScore", "getMultiScore", "getPlayerPos", "getPlayerPosXYZ",
		"getPlayerItem", "getBlockTile", "getTarget", "waitMsg", "showMenu", "promptChoice"} {
		if L.GetField(L.GetGlobal("game"), fn).Type() == 0 {
			t.Fatalf("game.%s 未绑定", fn)
		}
	}
}

// 验证 game.showMenu 可被 Lua 调用且不抛错。
// 说明：ShowMenu 内部走 SendWOCmd→SendAICommand→conn.WritePacket，无发送 hook 可拦截；
// 无连接时 SendAICommand 返回"未连接"错误但被 ShowMenu 忽略，故 Lua 侧无错误可验证。
func TestLuaShowMenuCallable(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	// Lua: showMenu(player, title, {"选项A","选项B"})
	err := L.DoString(`game.showMenu("alice", "测试菜单", {"选项A", "选项B"})`)
	if err != nil {
		t.Fatalf("showMenu 调用失败: %v", err)
	}
}

// 验证 game.sendLines 按行逐条发送（每行一条消息，超出 maxLines 截断并提示）。
func TestLuaSendLines(t *testing.T) {
	bm := &BotManager{}
	var got []string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = append(got, txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	L := newLuaState(bm)
	defer L.Close()
	if err := L.DoString(`game.sendLines("a", "line1\nline2\nline3", 2)`); err != nil {
		t.Fatalf("sendLines: %v", err)
	}
	// 3 行但 maxLines=2 → 提示 + 前 2 行
	if len(got) != 3 || !strings.Contains(got[0], "仅显示前 2 行") || got[1] != "line1" || got[2] != "line2" {
		t.Fatalf("sendLines 结果错误: %v", got)
	}
	// 空行跳过
	got = nil
	if err := L.DoString(`game.sendLines("a", "x\n\n\ny", 50)`); err != nil {
		t.Fatalf("sendLines 空行: %v", err)
	}
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Fatalf("sendLines 空行处理错误: %v", got)
	}
}

// 验证网络与 JSON 绑定可用（jsonEncode/jsonDecode 经 Lua 往返）。
func TestLuaJSONAndHTTPBindings(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	if L.GetField(L.GetGlobal("game"), "httpGet").Type() == 0 {
		t.Fatal("game.httpGet 未绑定")
	}
	if L.GetField(L.GetGlobal("game"), "httpPost").Type() == 0 {
		t.Fatal("game.httpPost 未绑定")
	}
	var got string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = txt; return nil })
	defer func() { setSendTargetedHook(old) }()
	err := L.DoString(`local t = game.jsonDecode('{"k":"v","n":42}'); game.sendTargeted("a", t.k .. ":" .. t.n)`)
	if err != nil {
		t.Fatalf("Lua 执行失败: %v", err)
	}
	if got != "v:42" {
		t.Fatalf("jsonDecode 解析错误: got %q want v:42", got)
	}
	err = L.DoString(`local s = game.jsonEncode({a=1,b="x"}); game.sendTargeted("a", s)`)
	if err != nil {
		t.Fatalf("jsonEncode 执行失败: %v", err)
	}
}
