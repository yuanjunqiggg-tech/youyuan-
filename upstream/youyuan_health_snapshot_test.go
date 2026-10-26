package main

import (
	lua "github.com/yuin/gopher-lua"
	"os"
	"testing"
)

func TestHealthDisplaySeedsOnlyObservedPresentSessionPlayers(t *testing.T) {
	source, e := os.ReadFile("../incident-fixes/2026-10-25/health_display.lua")
	if e != nil {
		t.Fatal(e)
	}
	L := lua.NewState()
	defer L.Close()
	if e = L.DoString(`
 calls={};util={timestamp=function()return 1000 end,json_write=function()end,scheduleTick=function()return 1 end}
 player={list=function()return {}end};events={subscribe=function()end,onHealthChange=function()end}
 game={dataDir=function()return '/owned' end,sendCommandFast=function(c)calls[#calls+1]=c;return true end,
 scene=function()return {connected=true,actors={
 {name='known',entity_type='minecraft:player',health_known=true,health=6.25,present=true},
 {name='unknown',entity_type='minecraft:player',health_known=false,present=true},
 {name='gone',entity_type='minecraft:player',health_known=true,health=20,present=false},
 {name='mob',entity_type='minecraft:pig',health_known=true,health=10,present=true}
 }}end}
 `); e != nil {
		t.Fatal(e)
	}
	if e = L.DoString("module=(function()\n" + string(source) + "\nend)();module.apply()\n" + `
 assert(plugin_api.hpPlayerValue('known')==6)
 assert(plugin_api.hpPlayerValue('unknown')==nil and plugin_api.hpPlayerValue('gone')==nil and plugin_api.hpPlayerValue('mob')==nil)
 local s=plugin_api.hpRuntimeStatus();assert(s.snapshot_writes==1 and s.packets==0 and s.player_writes==1)
 local count=#calls;module.apply();assert(#calls==count,'repeated apply duplicated writes')
 `); e != nil {
		t.Fatal(e)
	}
}
