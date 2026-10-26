package main

import (
	"encoding/json"
	lua "github.com/yuin/gopher-lua"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPluginMCPUsesActualServiceAndNoCredentials(t *testing.T) {
	oldMux, oldRemote, oldPort := yyMux, yyRemote, yyCorePort
	defer func() { yyMux = oldMux; yyRemote = oldRemote; yyCorePort = oldPort }()
	yyMux = http.NewServeMux()
	yyRemote.Key = "test-private-key"
	yyCorePort = "19621"
	yyMux.HandleFunc("/api/bot/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "connected": false, "real_handler": true})
	})
	value, e := yyPluginMCP(nil, "mcp.connection", nil)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(mustJSON(value), yyRemote.Key) || value.(map[string]any)["local_mcp_url"] != "http://127.0.0.1:19621/mcp/" {
		t.Fatal(value)
	}
	value, e = yyPluginMCP(nil, "mcp.call", []any{"api_bot_status", map[string]any{}})
	if e != nil || !strings.Contains(mustJSON(value), "real_handler") {
		t.Fatal(value, e)
	}
	if _, e = yyPluginMCP(nil, "mcp.call", []any{"api_bot_status", map[string]any{"account": "other"}}); e == nil {
		t.Fatal("Account override allowed")
	}
}

// Exercise the actual adapter with a deliberately slow old wrapper. This
// verifies the notice precedes the delay, rather than adding another notice.
func TestR21AThinkingNoticePrecedesSlowWrapperAndDoesNotDuplicate(t *testing.T) {
	dir := os.Getenv("YY_R21A_ADAPTED_DIR")
	if dir == "" {
		t.Skip("supplied plugin not selected")
	}
	L := lua.NewState()
	defer L.Close()
	var sent []time.Time
	var commands []string
	L.SetGlobal("fast", L.NewFunction(func(L *lua.LState) int {
		sent = append(sent, time.Now())
		commands = append(commands, L.CheckString(1))
		return 0
	}))
	L.SetGlobal("slow", L.NewFunction(func(L *lua.LState) int { time.Sleep(120 * time.Millisecond); return 0 }))
	L.SetGlobal("encode", L.NewFunction(func(L *lua.LState) int { b, _ := json.Marshal(luaToGo(L.Get(1))); L.Push(lua.LString(b)); return 1 }))
	if e := L.DoString(`
        util={schedule=function() return 1 end}; plugin={codeDir=function() return '/tmp' end}; mcp={}
        game={sendCommandFast=fast}; http={json_encode=encode}
        AI_TAG_BLUE='[Ds]'; AI_THINK_TEXT='正在思考中'
        function_registry_save=function() return true end
        ai_predict_wait=function() return 1,'chat' end
        ai_think_db=function() return {} end; ai_think_save=function() end; periodic_beat_now=function() return 1 end
        ai_think_begin=function() fast('duplicate notice') end
        on_chat=function(name,msg) slow(); ai_think_begin(name,msg,false,true) end
    `); e != nil {
		t.Fatal(e)
	}
	if e := L.DoFile(filepath.Join(dir, "modules/native_bridge.lua")); e != nil {
		t.Fatal(e)
	}
	L.SetGlobal("bridge", L.Get(-1))
	L.Pop(1)
	if e := L.DoString("bridge.afterCore()"); e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	if e := L.DoString(`on_chat('Tester','AI hello')`); e != nil {
		t.Fatal(e)
	}
	if len(sent) != 1 || !strings.Contains(commands[0], "正在思考中") {
		t.Fatal(commands)
	}
	if sent[0].Sub(start) >= 100*time.Millisecond || time.Since(start) < 120*time.Millisecond {
		t.Fatalf("Notice delay=%s, total=%s", sent[0].Sub(start), time.Since(start))
	}
	t.Logf("notice_ms=%.3f total_ms=%.3f count=%d", float64(sent[0].Sub(start))/float64(time.Millisecond), float64(time.Since(start))/float64(time.Millisecond), len(sent))
}
func TestR21AModulesCompileOnCurrentLua(t *testing.T) {
	dir := os.Getenv("YY_R21A_ADAPTED_DIR")
	if dir == "" {
		t.Skip("supplied plugin not selected")
	}
	L := lua.NewState()
	defer L.Close()
	for _, name := range []string{"main.lua", "modules/core.lua", "modules/native_bridge.lua"} {
		if _, e := L.LoadFile(filepath.Join(dir, name)); e != nil {
			t.Fatal(name, e)
		}
	}
}
func TestResidentStopCallsCleanupExactlyOnce(t *testing.T) {
	bm := &BotManager{}
	id := "cleanup-test"
	if e := LoadLuaPlugin(bm, id, "function on_stop() cleanup() end"); e != nil {
		t.Fatal(e)
	}
	r, e := yyGetResident(bm, id)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	r.mu.Lock()
	r.L.SetGlobal("cleanup", r.L.NewFunction(func(L *lua.LState) int { count++; return 0 }))
	r.mu.Unlock()
	yyStopResident(bm, id)
	yyStopResident(bm, id)
	if count != 1 {
		t.Fatal(count)
	}
}

func TestR21AActualResidentImportPromptsAndNativeChains(t *testing.T) {
	dir := os.Getenv("YY_R21A_ADAPTED_DIR")
	if dir == "" {
		t.Skip("supplied plugin not selected")
	}
	oldPC, oldMux, oldRoot, oldBot := pluginController, yyMux, yyChainRoot, botMgr
	yyChains.Lock()
	oldItems := yyChains.items
	yyChains.items = map[string]*yyChainInstance{}
	yyChains.Unlock()
	defer func() {
		pluginController = oldPC
		yyMux = oldMux
		yyChainRoot = oldRoot
		botMgr = oldBot
		yyChains.items = oldItems
	}()
	root := t.TempDir()
	pluginController = NewPluginController()
	pluginController.SetPluginDirs(root, "")
	files := map[string]string{}
	e := filepath.WalkDir(dir, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(dir, path)
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pluginController.ImportPackage(writeZip(t, files)); e != nil {
		t.Fatal(e)
	}
	pluginController.ScanPlugins()
	yyMux = http.NewServeMux()
	yyCommandRoutes(yyMux, root)
	defer RemovePluginSchedules("__chainswatch__")
	bm := &BotManager{}
	botMgr = bm
	defer func() {
		yyStopResident(bm, "ds_ai_agent")
		RemovePluginSchedules("ds_ai_agent")
		RemovePluginEvents("ds_ai_agent")
		RemovePluginWeb("ds_ai_agent")
	}()
	if e = pluginController.Run(bm, "ds_ai_agent"); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"core", "command", "build", "vote", "redeem", "mcp"} {
		got, e := yyCallResident(bm, "ds_ai_agent", "r17_read_prompt", []any{kind}, false, false)
		if e != nil || got != files["ai_prompts/"+kind+".txt"] {
			t.Fatal(kind, e)
		}
	}
	if _, e = yyCallResident(bm, "ds_ai_agent", "r21a_prepare_function_registry", nil, false, false); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"药水系统.txt", "雪球菜单_综合功能.txt"} {
		in := yyChains.items[name]
		if in == nil || !in.Config.Bootstrap || in.Config.Enabled || in.Config.Scheduling != "segments" {
			t.Fatal(name, in)
		}
		b, e := os.ReadFile(filepath.Join(yyChainRoot, name))
		if e != nil || string(b) != files["command_txt/"+name] {
			t.Fatal("Original text lost", name, e)
		}
	}
	// Execute the real layered chat entry and numeric routing. Intercept only
	// the final display output; keep the real storage and menu state machinery.
	resident, e := yyGetResident(bm, "ds_ai_agent")
	if e != nil {
		t.Fatal(e)
	}
	resident.mu.Lock()
	resident.L.SetContext(resident.lifetime)
	e = resident.L.DoString(`__menu_messages={}; send=function(name,text) table.insert(__menu_messages,tostring(text)) end`)
	resident.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	for _, number := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "10", "11", "12", "13", "14", "15", "16"} {
		if _, e = yyCallResident(bm, "ds_ai_agent", "on_chat", []any{"菜单验收", ".help"}, false, false); e != nil {
			t.Fatal("help", e)
		}
		if _, e = yyCallResident(bm, "ds_ai_agent", "on_chat", []any{"菜单验收", number}, false, false); e != nil {
			t.Errorf("menu %s failed: %v", number, e)
		}
		t.Log("real help routing", number)
	}
}
