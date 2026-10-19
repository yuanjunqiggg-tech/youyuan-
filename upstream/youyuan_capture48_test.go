package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLegacyRegistrationsAcceptNamesAndFunctionsWithoutResetOrCrossTalk(t *testing.T) {
	a, b := &BotManager{}, &BotManager{}
	id := "legacy-register48"
	source := `local count=0
 function named(n,m) count=count+1 end
 events.onChat("named")
 events.onChat(function(n,m) count=count+10 end)
 game.registerTrigger({".closure"}, function(n,args) count=count+100 end)
 events.subscribe("Animate",function(data) count=count+1000 end)
 util.setTimeout(0.01,function() count=count+10000 end)
 plugin_api={read=function() return count end}
 `
	defer func() {
		yyStopResident(a, id)
		yyStopResident(b, id)
		RemovePluginEvents(id)
		a.RemovePluginTriggers(id)
		b.RemovePluginTriggers(id)
	}()
	if e := LoadLuaPlugin(a, id, source); e != nil {
		t.Fatal(e)
	}
	if e := LoadLuaPlugin(b, id, source); e != nil {
		t.Fatal(e)
	}
	wait := func(bot *BotManager, want float64) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for {
			v, e := yyCallResident(bot, id, "read", nil, true, false)
			if e != nil {
				t.Fatal(e)
			}
			if v == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("want %v got %v", want, v)
			}
			time.Sleep(time.Millisecond)
		}
	}
	wait(a, 10000)
	wait(b, 10000)
	fireChatEvent(a, "player", "hello")
	wait(a, 10011)
	wait(b, 10000)
	RemovePluginEvents(id, a)
	fireChatEvent(b, "player", "hello")
	wait(b, 10011)
	wait(a, 10011)
	a.handleChat("player", ".closure")
	wait(a, 10111)
	wait(b, 10011)
	b.handleChat("player", ".closure")
	wait(b, 10122)
	wait(a, 10111)
	a.RemovePluginTriggers(id)
	b.handleChat("player", ".closure")
	wait(b, 10233)
}

func TestTransferSpatialLeaseIsNestedAndOnlyBlocksSelfMovement(t *testing.T) {
	a, b := &BotManager{display: "observer"}, &BotManager{}
	first := yyBeginSpatialJob(a, "export")
	second := yyBeginSpatialJob(a, "import")
	if yyPluginMovementAllowed(a) == nil || yyPluginMovementAllowed(b) != nil {
		t.Fatal("wrong account lock")
	}
	first()
	first()
	if yyPluginMovementAllowed(a) == nil {
		t.Fatal("nested lock released early")
	}
	for _, cmd := range []string{"tp @s 1 2 3", "execute as @s run tp ~ ~ ~", "tp observer 1 2 3"} {
		if !yyPluginTeleportCommand(a, cmd) {
			t.Fatal(cmd)
		}
	}
	if yyPluginTeleportCommand(a, "tp other 1 2 3") || yyPluginTeleportCommand(a, "say teleport is paused") {
		t.Fatal("unrelated command blocked")
	}
	second()
	if yyPluginMovementAllowed(a) != nil {
		t.Fatal("not resumed")
	}
}

func TestMultiPlayerSelectorCameraHasNoQueryOrBodyTeleport(t *testing.T) {
	pitch := 90.0
	c, e := yyWatchParse(map[string]any{"players": []string{"A", "B"}, "mode": "camera", "interval_seconds": .001, "camera_height_blocks": 4, "camera_pitch_degrees": pitch})
	if e != nil || len(c.Players) != 2 {
		t.Fatal(c, e)
	}
	for _, target := range c.Players {
		command := yySelectorCameraCommand("observer", target, c)
		if !strings.Contains(command, `execute at "`+target+`" run camera "observer"`) || !strings.Contains(command, "~4.000") || !strings.Contains(command, "rot 90.000") || strings.Contains(command, "querytarget") || strings.Contains(command, " tp ") {
			t.Fatal(command)
		}
	}
	if _, e := yyWatchParse(map[string]any{"players": []string{"A", "A"}}); e == nil {
		t.Fatal("duplicate selection accepted")
	}
	if _, e := yyWatchParse(map[string]any{"auto_players": true}); e != nil {
		t.Fatal(e)
	}
	bot := &BotManager{display: "observer"}
	w := &yyWatch{Bot: bot, Config: yyWatchConfig{Players: []string{"observer", "A", "B"}, ExcludePlayers: []string{"B"}}}
	if got := w.targets(); len(got) != 1 || got[0] != "A" {
		t.Fatalf("self and excluded robots must not invalidate other selected players: %v", got)
	}
}

func TestCaptureAccountSelectionStaysInOwnPoolEvenForAdministrator(t *testing.T) {
	oldAccounts := yyAccounts.Items
	oldUser := yyIdentity.Data.User
	defer func() { yyAccounts.Items = oldAccounts; yyIdentity.Data.User = oldUser }()
	yyIdentity.Data.User = map[string]any{"id": "own-user", "role": "admin"}
	yyAccounts.Items = []yyManagedAccount{{ID: "mine", OwnerID: "own-user"}, {ID: "foreign", OwnerID: "other-user"}}
	actor := yyPoolActor{ID: "own-user", Admin: true, Scope: "control"}
	c := yyGlobalCaptureConfig{Enabled: true, Accounts: []string{"main", "mine"}, Players: []string{"A", "B"}, Mode: "camera", Interval: .001, Height: 4, Pitch: 90, PauseJobs: true}
	if e := yyGlobalCaptureValidate(c, actor); e != nil {
		t.Fatal(e)
	}
	c.Accounts = []string{"foreign"}
	if e := yyGlobalCaptureValidate(c, actor); e == nil {
		t.Fatal("foreign account accepted")
	}
	c.Accounts = []string{"mine"}
	actor.Scope = "read"
	if e := yyGlobalCaptureValidate(c, actor); e == nil {
		t.Fatal("read-only could start capture")
	}
}

func TestOldStyleLuaZipImportsResourcesAndRunsRegisteredCallbacks(t *testing.T) {
	root := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(root, "")
	previousController := pluginController
	pluginController = pc
	defer func() { pluginController = previousController }()
	source := `local helper=dofile("modules/helper.lua");assert(helper.value==42);counter=0;function oldChat(name,message)counter=counter+1 end;events.onChat("oldChat");plugin_api={read=function()return counter end}`
	path := writeZip(t, map[string]string{"manifest.json": `{"id":"compat48","name":"旧语法验收","type":"lua"}`, "main.lua": source, "modules/helper.lua": "return {value=42}", "command_txt/a.txt": "say hello"})
	id, e := pc.ImportPackage(path)
	if e != nil {
		t.Fatal(e)
	}
	if id != "compat48" {
		t.Fatal(id)
	}
	if data, e := os.ReadFile(filepath.Join(root, "插件文件", id, "command_txt", "a.txt")); e != nil || string(data) != "say hello" {
		t.Fatal("TXT resource lost", e)
	}
	code, e := os.ReadFile(filepath.Join(root, "插件文件", id, id+".lua"))
	if e != nil {
		t.Fatal(e)
	}
	bm := &BotManager{}
	defer func() { yyStopResident(bm, id); RemovePluginEvents(id, bm) }()
	if e = LoadLuaPlugin(bm, id, string(code)); e != nil {
		t.Fatal(e)
	}
	fireChatEvent(bm, "tester", ".help")
	deadline := time.Now().Add(time.Second)
	for {
		v, e := yyCallResident(bm, id, "read", nil, true, false)
		if e != nil {
			t.Fatal(e)
		}
		if v == float64(1) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("imported original callback did not execute", v)
		}
		time.Sleep(time.Millisecond)
	}
}
