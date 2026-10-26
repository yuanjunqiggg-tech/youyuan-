package main

import (
	lua "github.com/yuin/gopher-lua"
	"os"
	"path/filepath"
	"testing"
)

func TestLiveCombatPatchOnlyMatchedWithin18AndNoFullScene(t *testing.T) {
	root := os.Getenv("YY_LUA_PATCH_DIR")
	if root == "" {
		t.Skip("set YY_LUA_PATCH_DIR to actual patched modules")
	}
	L := lua.NewState()
	defer L.Close()
	setup := `
 local t=100
 captures={};scene_calls=0;query_calls=0
 util={timestamp=function()return t end,monotonicMs=function()return t*1000 end,
 json_write=function()return true end,json_load=function()return{}end,scheduleSeconds=function()return 1 end,scheduleTick=function()return 1 end}
 events=setmetatable({},{__index=function()return function()return true end end})
 hud={set=function(n,ch,v)table.insert(captures,{player=n,channel=ch,data=v});return{queued=true}end}
 entities={get=function(k)return{known=true,present=true,position_known=true,position_stale=false,
 position={1,64,0},position_age_ms=0,dimension=0,entity_type='minecraft:zombie',armor_known=false}end}
 player={list=function()return{{name='P',position={0,64,0},dimension=0,online=true}}end,
 position=function()return{position_known=true,position_stale=false,stale=false,position={0,64,0},dimension=0}end}
 game={dataDir=function()return '/tmp' end,scene=function()scene_calls=scene_calls+1;return{actors={}}end,
 sceneStats=function()return{active_actors=1}end,sendCommandFast=function(cmd)assert(not cmd:find('titleraw',1,true));return{submitted=true}end}
 function init_actors()
 yy4_player({entity_runtime_id=7,username='P',position={0,64,0}})
 yy4_add({entity_runtime_id=10,entity_type='minecraft:zombie',position={1,64,0},attributes={{name='minecraft:health',value=20,max=20}}})
 end
 `
	if e := L.DoString(setup); e != nil {
		t.Fatal(e)
	}
	if e := L.DoFile(filepath.Join(root, "entity_hurt_info.lua")); e != nil {
		t.Fatal(e)
	}
	L.SetGlobal("mod", L.Get(-1))
	L.Pop(1)
	test := `
 assert(mod.apply());local baseline=scene_calls
 init_actors()
 yy4_swing({runtime_id=7,player='P'})
 yy4_native_health({runtime_id=10,health=17,max_health=20,previous=20,position={1,64,0},entity_type='minecraft:zombie'})
 assert(#captures==1,'matched hit did not produce exactly one managed HUD: '..#captures)
 assert(captures[1].player=='P' and captures[1].data.duration_ms==1200)
 assert(captures[1].data.text:find('未知',1,true),'unknown armor invented')
 yy4_native_health({runtime_id=10,health=17,max_health=20,previous=20})
 assert(#captures==1,'duplicate native health replay displayed twice')
 yy4_cleanup();assert(scene_calls==baseline,'periodic or hit callback copied full scene')
 player.position=function()return{position_known=true,position_stale=false,stale=false,position={30,64,0},dimension=0}end
 yy4_native_health({runtime_id=10,health=14,max_health=20,previous=17})
 assert(#captures==1,'out-of-range observer got a HUD')
 `
	if e := L.DoString(test); e != nil {
		t.Fatal(e)
	}
}

func TestContainerGuardRapidRepeatDoesNotWaitOrDuplicate(t *testing.T) {
	root := os.Getenv("YY_LUA_PATCH_DIR")
	if root == "" {
		t.Skip("set YY_LUA_PATCH_DIR")
	}
	L := lua.NewState()
	defer L.Close()
	if e := L.DoString(`sent=0;util={timestamp=function()return 100 end,json_load=function()return{}end,json_write=function()return true end,scheduleTick=function()end,scheduleSeconds=function()end}
 game={dataDir=function()return'/tmp'end,sendCommandFast=function(cmd)if cmd:find('setblock',1,true)then sent=sent+1 end;return{submitted=true}end,getBlockTileAsync=function()error('should prefer BlockActorData')end}
 events={subscribe=function()return true end};player={list=function()return{}end}`); e != nil {
		t.Fatal(e)
	}
	if e := L.DoFile(filepath.Join(root, "yy_runtime_guard.lua")); e != nil {
		t.Fatal(e)
	}
	L.SetGlobal("mod", L.Get(-1))
	L.Pop(1)
	if e := L.DoString(`assert(mod.apply());for i=1,20 do yy_guard_update_block({position={0,64,0}});yy_guard_block_actor({position={0,64,0},nbtdata={id='Chest'}});yy_guard_edge_tick() end;assert(sent==20,'rapid repeats missed '..sent)`); e != nil {
		t.Fatal(e)
	}
}

func TestObservationTeamConfiguresRealNativeWatchCalls(t *testing.T) {
	root := os.Getenv("YY_LUA_PATCH_DIR")
	if root == "" {
		t.Skip("set YY_LUA_PATCH_DIR")
	}
	L := lua.NewState()
	defer L.Close()
	setup := `calls={};util={monotonicMs=function()return 100000 end,json_load=function()return{}end,json_write=function()return true end,scheduleSeconds=function()end}
 game={dataDir=function()return'/tmp'end};bot={self=function()return{name='Controller'}end}
 player={list=function()return{{name='P',online=true},{name='Q',online=true},{name='ObserverA',online=true}}end}
 accountsPool={status=function(id)return{connected=true}end,invoke=function(id,o)
 local m=o.body.method
 if m=='bot.self'then return{ok=true,result={name=id=='a'and'ObserverA'or'ObserverB'}}end
 if m=='observerBridge.watchStart'then table.insert(calls,{account=id,options=o.body.args[1]});return{ok=true,result={active=true}}end
 return{ok=true,result={active=false}}end}
 observerBridge={worldEvents=function()return{cursors={},events={}}end}`
	if e := L.DoString(setup); e != nil {
		t.Fatal(e)
	}
	if e := L.DoFile(filepath.Join(root, "observation_team.lua")); e != nil {
		t.Fatal(e)
	}
	L.SetGlobal("mod", L.Get(-1))
	L.Pop(1)
	test := `assert(mod.apply());local before=plugin_api.observationTeamStatus()
 local ok=pcall(plugin_api.observationTeamConfigure,{interval_seconds=-1});assert(not ok)
 assert(plugin_api.observationTeamStatus().config.interval_seconds==before.config.interval_seconds,'invalid config mutated state')
 plugin_api.observationTeamConfigure({enabled=true,accounts={'a','b'},targets={},camera_height_blocks=4,camera_pitch_degrees=90})
 yy_team_control_tick();assert(#calls==2,'native watchers not called')
 assert(calls[1].options.player=='P' and calls[2].options.player=='Q','bot included or target missing')
 assert(calls[1].options.camera_pitch_degrees==90 and calls[1].options.camera_height_blocks==4)
 plugin_api.observationTeamStop();assert(not plugin_api.observationTeamStatus().config.enabled)`
	if e := L.DoString(test); e != nil {
		t.Fatal(e)
	}
}
