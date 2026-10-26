package main

import (
	"os"
	"testing"
)

func TestSnowballStartupPreservesNativeOwnerAndStoppedState(t *testing.T) {
	for _, file := range []string{"yy_snow_native_always_r95.lua", "yy_snow_native_restore_r96.lua", "muse_snowball_menu.lua"} {
		for _, enabled := range []string{"false", "true"} {
			t.Run(file+"/"+enabled, func(t *testing.T) {
				text, err := os.ReadFile("../incident-fixes/2026-10-25/" + file)
				if err != nil {
					t.Fatal(err)
				}
				L := newYYLuaVM()
				defer L.Close()
				prelude := `
function forbidden()error('duplicate startup or game operation')end
game={dataDir=function()return 'fixture' end,sendCommand=forbidden,executeBatch=forbidden}
util={timestamp=function()return 10 end,json_write=function()return true end,scheduleSeconds=forbidden}
events={onEntitySpawn=forbidden}
chains={start=forbidden,configure=forbidden,stop=forbidden,reload=forbidden,
list=function()return {files={
{config={name='雪球菜单_综合功能.txt',enabled=` + enabled + `},commands=18,state='test'},
{config={name='_违禁收纳袋独立清除20Hz.txt',enabled=` + enabled + `},commands=18,state='test'}}}end}
`
				if err := L.DoString(prelude + "\nm=(function()\n" + string(text) + "\nend)();assert(m.apply());assert(m.apply())"); err != nil {
					t.Fatal(err)
				}
				if file == "muse_snowball_menu.lua" {
					if err := L.DoString(`assert(plugin_api.museSnowMenuStatus().owner=='native-chain');muse_snow_menu_tick();muse_snow_online_tick();museSnowSpawnEvt({entity_type='minecraft:snowball'})`); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestSnowballLuaUnknownProbeDoesNotRunMenuOrReplayFailedBatch(t *testing.T) {
	text, err := os.ReadFile("../incident-fixes/2026-10-25/muse_snowball_menu.lua")
	if err != nil {
		t.Fatal(err)
	}
	L := newYYLuaVM()
	defer L.Close()
	prelude := `
writes=0;batches=0;mode='unknown'
game={sendCommand=function()writes=writes+1 end,executeBatch=function(list)
 batches=batches+1
 if mode=='error' then error('result unknown after a partial write')end
 if #list==1 then return {confirmed=false,results={{confirmed=false}}}end
 assert(#list==2,'unknown probe authorized menu/surface writes')
 return {confirmed=true}
end}
util={scheduleSeconds=function()return 1 end};events={onEntitySpawn=function()return true end}
chains={list=function()return {files={}}end}
`
	if err := L.DoString(prelude + "\nm=(function()\n" + string(text) + "\nend)();assert(m.apply());assert(writes==5);muse_snow_menu_tick();assert(batches==4 and writes==5);mode='error';museSnowSpawnEvt({entity_type='minecraft:snowball'});assert(writes==5,'failed batch replayed through single sends');assert(plugin_api.museSnowMenuStatus().probe_failures==3)"); err != nil {
		t.Fatal(err)
	}
}
