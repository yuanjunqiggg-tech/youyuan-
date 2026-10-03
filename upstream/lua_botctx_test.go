package main

import (
	"strings"
	"testing"
)

// bot.current() 默认返回主机器人（registerGameAPI 时置 active=main）。
func TestBotCurrentDefault(t *testing.T) {
	bm := &BotManager{display: "主机器人"}
	L := newLuaState(bm)
	defer L.Close()
	var got []string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = append(got, txt); return nil })
	defer func() { setSendTargetedHook(old) }()
	if err := L.DoString(`local c = bot.current(); game.sendTargeted("a", c.name)`); err != nil {
		t.Fatalf("bot.current: %v", err)
	}
	if len(got) != 1 || got[0] != "主机器人" {
		t.Fatalf("bot.current 默认应为主机器人: %v", got)
	}
}

// 舰队为空时 bot.select 应报错"未找到机器人"。
func TestBotSelectMissingRaises(t *testing.T) {
	bm := &BotManager{display: "主机器人"}
	L := newLuaState(bm)
	defer L.Close()
	err := L.DoString(`bot.select("不存在的机器人")`)
	if err == nil || !strings.Contains(err.Error(), "未找到机器人") {
		t.Fatalf("bot.select 缺失应报错, got: %v", err)
	}
}

// 路由一致性：默认活跃 bot 即主 bot；game.sendCommand 走 abot(L)（用 sendCommandHook 捕获）。
func TestBotRoutingCommandToActive(t *testing.T) {
	bm := &BotManager{display: "主机器人"}
	var got []string
	oldHook := sendCommandHook
	setSendCommandHook(func(cmd string) error { got = append(got, cmd); return nil })
	defer func() { setSendCommandHook(oldHook) }()
	L := newLuaState(bm)
	defer L.Close()
	if err := L.DoString(`game.sendCommand("say hi")`); err != nil {
		t.Fatalf("sendCommand: %v", err)
	}
	if len(got) != 1 || got[0] != "say hi" {
		t.Fatalf("sendCommand 未路由: %v", got)
	}
}
