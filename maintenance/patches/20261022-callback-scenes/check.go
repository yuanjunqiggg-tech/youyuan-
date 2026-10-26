package main

import (
 "fmt"
 "os"
 "path/filepath"
 lua "github.com/yuin/gopher-lua"
)

func main() {
 root:=os.Args[1]
 for _,name:=range []string{"entity_hurt_info.lua","health_display.lua","yy_view_jitter_probe.lua"} {
  L:=lua.NewState()
  setup:=`
  scene_calls=0;player_scene_calls=0;single_calls=0;writes={};bad_scene=0
  util={timestamp=function()return 100 end,monotonicMs=function()return 100000 end,json_write=function()return true end,json_load=function()return{}end,scheduleSeconds=function()return 1 end,scheduleTick=function()return 1 end}
  events=setmetatable({},{__index=function()return function()return true end end})
  local actor={known=true,present=true,stale=false,runtime_id='10',entity_type='minecraft:zombie',position_known=true,position_stale=false,position_age_ms=0,position={1,64,0},dimension=0,health=17,max_health=20,health_known=true,health_stale=false}
  player={list=function()return{{name='P',position={0,64,0},online=true,dimension=0}}end,
   position=function(k)single_calls=single_calls+1;if k=='10' then return actor end;return{known=true,runtime_id='7',name='P',entity_type='minecraft:player',present=true,stale=false,position_known=true,position_stale=false,position={0,64,0},dimension=0}end}
  game={dataDir=function()return'/tmp'end,sendCommandFast=function(cmd)table.insert(writes,cmd);return{submitted=true}end,
   scene=function(o)scene_calls=scene_calls+1;if type(o)~='table' or o.present_only~=true then bad_scene=bad_scene+1;error('full scene requested')end
    if o.entity_type=='minecraft:player' then player_scene_calls=player_scene_calls+1;return{actors={player.position('P')}}end
    return{actors={actor,player.position('P')}}end}
  bot={self=function()return{name='P',runtime_id='7'}end}
  `
  if e:=L.DoString(setup);e!=nil{panic(e)}
  if e:=L.DoFile(filepath.Join(root,name));e!=nil{panic(e)}
  L.SetGlobal("mod",L.Get(-1));L.Pop(1)
  script:=`assert(mod.apply());assert(bad_scene==0)`
  if name=="entity_hurt_info.lua" {
   script+=`
    local before=player_scene_calls
    yy4_player({entity_runtime_id=7,username='P',position={0,64,0}})
    for i=1,200 do yy4_add({entity_runtime_id=1000+i,entity_type='minecraft:snowball',position={1,64,0}})end
    assert(player_scene_calls==before,'projectiles should use existing per-player snapshots without a scene copy')
    assert(single_calls>=200,'projectile/player identity lookup was skipped')
    yy4_add({entity_runtime_id=10,entity_type='minecraft:zombie',position={1,64,0},attributes={{name='minecraft:health',value=20,max=20}}})
    yy4_swing({runtime_id=7,player='P'})
    yy4_native_health({runtime_id=10,health=17,max_health=20,previous=20,position={1,64,0},entity_type='minecraft:zombie'})
    yy4_cleanup();assert(bad_scene==0)
   `
  } else if name=="health_display.lua" {
   script+=`
    local before=#writes
    yy_hp2_spawn({entity_runtime_id=10,entity_type='minecraft:zombie',position={1,64,0},attributes={{name='minecraft:health',value=17,max=20}}})
    yy_hp2_tick();assert(#writes>before,'health update no longer submitted')
    assert(bad_scene==0 and scene_calls==0,'health callback should only look up the changed runtime ID')
   `
  } else {
   script+=`yy_view_probe_add_player({entity_runtime_id=7,username='P'});yy_view_probe_move({entity_runtime_id=7,pitch=1,yaw=2});yy_view_probe_tick();assert(bad_scene==0 and player_scene_calls>0,'view diagnostic should only look up present players')`
  }
  if e:=L.DoString(script);e!=nil{panic(name+": "+e.Error())}
  fmt.Println(name,"checks passed; full-scene calls",L.GetGlobal("bad_scene"))
  L.Close()
 }
}
