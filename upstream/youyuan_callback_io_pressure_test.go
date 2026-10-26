package main

import (
	"context"
	"os"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestLiveInventoryMonitorUsesYieldingQueryWithoutDroppingChecks(t *testing.T) {
	source, err := os.ReadFile("testdata/callback_io/ac_inventory_monitor.lua")
	if err != nil {
		t.Fatal(err)
	}
	L := newLuaState(&BotManager{})
	defer L.Close()
	checks := 0
	L.SetField(L.GetGlobal("game"), "queryInventory", L.NewFunction(func(L *lua.LState) int {
		if L.CheckString(1) != "fixture-player" {
			L.RaiseError("unexpected player")
		}
		checks++
		L.Push(goToLua(L, map[string]any{"items": []any{map[string]any{"name": "diamond", "count": 2}}}))
		return 1
	}))
	prelude := `
game.dataDir=function()return 'fixture' end
player.list=function()return {{name='fixture-player'}} end
player.isOP=function()return false end
player.getInventory=function()error('legacy VM-blocking entry was used')end
is_admin=function()return false end
util={timestamp=function()return 100 end,json_load=function()error('no prior baseline')end,json_write=function()return true end,scheduleSeconds=function()return 1 end}
local m=(function()
`
	if err := L.DoString(prelude + string(source) + "\n" + `end)();m.apply();yy_ac_inventory_check();assert(plugin_api.acInventoryStatus().scans==1,'scan count '..tostring(plugin_api.acInventoryStatus().scans))`); err != nil {
		t.Fatal(err)
	}
	if checks != 1 {
		t.Fatal("inventory check was skipped", checks)
	}
}

func TestPlayerInventoryUsesCooperativeGameBinding(t *testing.T) {
	L := newLuaState(&BotManager{})
	defer L.Close()
	if L.GetField(L.GetGlobal("player"), "getInventory") != L.GetField(L.GetGlobal("game"), "queryInventory") {
		t.Fatal("inventory alias bypasses the shared query's cooperative I/O")
	}
	block := L.GetField(L.GetGlobal("world"), "block")
	if block.Type() != lua.LTFunction || block != L.GetField(L.GetGlobal("game"), "getBlockTile") {
		t.Fatal("world block alias lost the existing cooperative query")
	}
}

func TestTargetedSendBackpressureDoesNotBlockPriorityCallbacks(t *testing.T) {
	bm := &BotManager{}
	const id = "targeted-send-pressure"
	if err := LoadLuaPlugin(bm, id, `n=0;function send()game.sendTargeted('self','probe')end;function event(e)assert(e.seq==n+1);n=e.seq end;function read()return n end`); err != nil {
		t.Fatal(err)
	}
	r, _ := yyGetResident(bm, id)
	started, release := make(chan struct{}), make(chan struct{})
	old := getSendTargetedHook()
	setSendTargetedHook(func(string, string) error { close(started); <-release; return nil })
	done := make(chan error, 1)
	defer func() { close(release); <-done; setSendTargetedHook(old); yyStopResident(bm, id) }()
	go func() { _, err := yyCallResident(bm, id, "send", nil, false, false); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("send never entered the slow transport")
	}
	for i := 1; i <= 100; i++ {
		yyQueueSenseLua(r, bm, id, "event", []any{map[string]any{"type": "health", "seq": i}})
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		v, err := yyCallResident(bm, id, "read", nil, false, true)
		if err == nil && v == float64(100) {
			if r.senseDropped.Load() != 0 || r.callbackFailures.Load() != 0 {
				t.Fatal("lost or reordered priority events")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("network backpressure held the Lua VM and blocked priority callbacks")
}

func TestInventoryAliasUnderSlowQueryPreservesEventOrder(t *testing.T) {
	bm := &BotManager{}
	const id = "inventory-alias-pressure"
	if err := LoadLuaPlugin(bm, id, `n=0;function inventory()return player.getInventory('self')end;function event(e)assert(e.seq==n+1);n=e.seq end;function read()return n end`); err != nil {
		t.Fatal(err)
	}
	r, _ := yyGetResident(bm, id)
	started, release := make(chan struct{}), make(chan struct{})
	r.mu.Lock()
	r.L.SetField(r.L.GetGlobal("game"), "queryInventory", r.L.NewFunction(func(L *lua.LState) int {
		_, err := yyLuaWaitValue(L, func(ctx context.Context) (bool, error) {
			close(started)
			select {
			case <-release:
				return true, nil
			case <-ctx.Done():
				return false, ctx.Err()
			}
		})
		if err != nil {
			L.RaiseError("%v", err)
		}
		L.Push(L.NewTable())
		return 1
	}))
	registerPlayerAPI(r.L)
	r.mu.Unlock()
	done := make(chan error, 1)
	defer func() { close(release); <-done; yyStopResident(bm, id) }()
	go func() { _, err := yyCallResident(bm, id, "inventory", nil, false, false); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("query did not start")
	}
	for i := 1; i <= 200; i++ {
		yyQueueSenseLua(r, bm, id, "event", []any{map[string]any{"seq": i}})
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		v, err := yyCallResident(bm, id, "read", nil, false, true)
		if err == nil && v == float64(200) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("slow inventory blocked the ordinary event FIFO")
}
