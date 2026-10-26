package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	lua "github.com/yuin/gopher-lua"
	"os"
	"testing"
)

func TestWorldCombatRetainsHitsDuringCameraSpawnChurn(t *testing.T) {
	b := senseBot(t)
	b.connected = true
	b.serverCode = "A"
	yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 42, Position: mgl32.Vec3{0, 64, 0}})
	yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 901, Position: mgl32.Vec3{1, 64, 0}})
	senseHealth(b, 901, 20)
	senseHealth(b, 901, 17)
	for i := 0; i < 800; i++ {
		yySenseHandle(b, &packet.AddActor{EntityRuntimeID: uint64(2000 + i), EntityUniqueID: int64(2000 + i), EntityType: "minecraft:enderman", Position: mgl32.Vec3{1000, 64, 0}})
		yySenseHandle(b, &packet.RemoveActor{EntityUniqueID: int64(2000 + i)})
	}
	got := worldView(t, b, "events", "", map[string]any{"stream": "combat"})
	rows := got["events"].([]any)
	found := false
	for _, v := range rows {
		e := v.(map[string]any)
		if e["type"] == "health" && e["health"] == float64(17) {
			found = true
		}
	}
	if !found {
		t.Fatal("camera churn evicted real health change", rows)
	}
	next := worldView(t, b, "events", "", map[string]any{"stream": "combat", "session": got["session"], "after": got["cursor"]})
	if len(next["events"].([]any)) != 0 {
		t.Fatal("replayed consumed combat")
	}
	yySenseReset(b)
	if len(worldView(t, b, "events", "", map[string]any{"stream": "combat"})["events"].([]any)) != 0 {
		t.Fatal("combat leaked across reconnect")
	}
}

func TestWorldCombatLuaConsumesEndHitsOnceWithoutLocalCacheMixing(t *testing.T) {
	raw, err := os.ReadFile("../incident-fixes/2026-10-25/entity_hurt_info.lua")
	if err != nil {
		t.Fatal(err)
	}
	L := newYYLuaVM()
	defer L.Close()
	setup := `clock=100;sent={};polls=0
 util={timestamp=function()return clock end,monotonicMs=function()return clock*1000 end,json_write=function()end,json_load=function()return{}end,scheduleSeconds=function()end,scheduleTick=function()end}
 events=setmetatable({},{__index=function()return function()return true end end})
 player={list=function()return{{name='P',position={0,64,0},dimension=0,online=true}}end,position=function(id)assert(not tostring(id):find('world:',1,true),'remote ID sent to local cache');return{}end}
 entities={get=function()return{}end}
 game={dataDir=function()return'/tmp'end,scene=function()return{actors={}}end,sendCommandFast=function()return true end,sendActionbar=function(name,text)table.insert(sent,{name=name,text=text});return true end}
 observerBridge={worldEvents=function(o)
 assert(o.stream=='combat');polls=polls+1
 if polls>1 then assert(o.cursors.far.sequence==2);return{cursors=o.cursors,complete=true,events={}} end
 local ob={account='far',session='one',connected=true,server='A'}
 return{complete=true,cursors={far={session='one',sequence=2}},events={
 {type='swing',runtime_id='7',player='P',entity_type='minecraft:player',dimension=2,position={0,64,0},timestamp=100000,observer=ob},
 {type='health',runtime_id='10',entity_type='minecraft:enderman',dimension=2,position={1,64,0},health=17,max_health=40,previous=20,timestamp=100000,observer=ob}}} end}
 `
	if err := L.DoString(setup); err != nil {
		t.Fatal(err)
	}
	fn, err := L.LoadString(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
		t.Fatal(err)
	}
	L.SetGlobal("mod", L.Get(-1))
	L.Pop(1)
	if err := L.DoString(`assert(mod.apply());assert(yy_world_combat_poll());assert(#sent==1 and sent[1].name=='P','End hit not delivered');assert(yy_world_combat_poll());assert(#sent==1,'cursor replay');local s=plugin_api.worldCombatStatus();assert(s.health==1 and s.swing==1 and s.by_account.far==2)`); err != nil {
		t.Fatal(err)
	}
}
