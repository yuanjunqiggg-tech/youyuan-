package main

import (
	"strings"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// 事件/玩家/定时器/setScore 表绑定齐全。
func TestNewCapabilityTablesBound(t *testing.T) {
	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	check := func(table string, fns ...string) {
		tbl := L.GetGlobal(table)
		for _, fn := range fns {
			if L.GetField(tbl, fn).Type() == 0 {
				t.Fatalf("%s.%s 未绑定", table, fn)
			}
		}
	}
	check("events", "onChat", "onJoin", "onLeave", "onBotReady", "onPacket", "subscribe")
	check("player", "get", "getXUID", "getUniqueID", "list", "count", "isOP", "getInventory")
	check("util", "schedule", "setTimeout", "cancel")
	if L.GetField(L.GetGlobal("game"), "setScore").Type() == 0 {
		t.Fatal("game.setScore 未绑定")
	}
}

// onChat 事件：注册后 handleChat 触发 Lua handler。
func TestEventOnChat(t *testing.T) {
	bm := &BotManager{}
	src := `
		events.onChat("onMsg")
		function onMsg(player, msg)
			game.sendTargeted("a", "got:" .. player .. ":" .. msg)
		end
	`
	if err := LoadLuaPlugin(bm, "evt", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	defer RemovePluginEvents("evt") // 清理全局事件注册，避免污染其它测试
	c := &captureHook{}
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { c.add(txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	bm.handleChat("alice", "hi")
	deadline := time.Now().Add(2 * time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.all(); len(got) != 1 || !strings.Contains(got[0], "got:alice:hi") {
		t.Fatalf("onChat 未触发: %v", got)
	}
}

// events.subscribe：订阅某类数据包，收到后触发 Lua handler；RemovePluginEvents 后停止。
func TestEventSubscribePacket(t *testing.T) {
	bm := &BotManager{}
	src := `
		events.subscribe("SetTime", "onPkt")
		function onPkt(data)
			game.sendTargeted("a", "pkt:" .. data.time)
		end
	`
	if err := LoadLuaPlugin(bm, "pktSub", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	c := &captureHook{}
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { c.add(txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	packetHubDispatch(bm, &packet.SetTime{Time: 777})
	deadline := time.Now().Add(2 * time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.all(); len(got) != 1 || !strings.Contains(got[0], "pkt:777") {
		t.Fatalf("events.subscribe 未触发: %v", got)
	}

	// 清理后不再触发
	RemovePluginEvents("pktSub")
	packetHubDispatch(bm, &packet.SetTime{Time: 888})
	time.Sleep(100 * time.Millisecond)
	if n := c.count(); n != 1 {
		t.Fatalf("RemovePluginEvents 后仍触发, count=%d", n)
	}
}

// setTimeout 一次性触发一次（重跑上下文不重复注册）。
func TestSetTimeoutOnce(t *testing.T) {
	bm := &BotManager{}
	src := `
		util.setTimeout(0.05, "onTick")
		function onTick()
			game.sendTargeted("a", "tick")
		end
	`
	if err := LoadLuaPlugin(bm, "sched1", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	c := &captureHook{}
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { c.add(txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	deadline := time.Now().Add(2 * time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.all(); len(got) != 1 {
		t.Fatalf("setTimeout 应只触发一次, got=%d", len(got))
	}
}

// 非法定时参数在加载时报错（旧版 util.schedule(0) 会让 NewTicker panic 杀死整个进程）。
func TestScheduleInvalidInterval(t *testing.T) {
	bm := &BotManager{}
	for _, src := range []string{
		`util.schedule(0, "onTick")`,
		`util.schedule(-1, "onTick")`,
		`util.schedule(999999999, "onTick")`,
		`util.setTimeout(-1, "onTick")`,
	} {
		if err := LoadLuaPlugin(bm, "badSched", src); err == nil {
			t.Fatalf("非法定时参数应报错: %s", src)
		}
	}
}

// Lua 死循环被执行超时中断（把超时调小到 100ms 验证机制生效）。
func TestLuaInfiniteLoopInterrupted(t *testing.T) {
	old := luaExecTimeout
	luaExecTimeout = 100 * time.Millisecond
	defer func() { luaExecTimeout = old }()

	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()
	start := time.Now()
	err := L.DoString(`while true do end`)
	if err == nil {
		t.Fatal("死循环应被超时中断")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("超时中断过慢: %v", elapsed)
	}
}

// schedule 重复触发，RemovePluginSchedules 后停止。
func TestScheduleCancel(t *testing.T) {
	bm := &BotManager{}
	src := `
		util.schedule(0.02, "onTick")
		function onTick()
			game.sendTargeted("a", "tick")
		end
	`
	if err := LoadLuaPlugin(bm, "sched2", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	c := &captureHook{}
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { c.add(txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	time.Sleep(120 * time.Millisecond)
	if c.count() < 2 {
		t.Fatalf("schedule 应重复触发多次, got=%d", c.count())
	}
	RemovePluginSchedules("sched2")
	time.Sleep(80 * time.Millisecond)
	n := c.count()
	time.Sleep(80 * time.Millisecond)
	if c.count() != n {
		t.Fatalf("取消后仍触发: 取消前=%d 取消后=%d", n, c.count())
	}
}
