package main

import (
	"encoding/json"
	lua "github.com/yuin/gopher-lua"
	"os"
	"testing"
)

func TestNativeMenuUsesRunnerParserAndOneToggleOperation(t *testing.T) {
	source, err := os.ReadFile("../incident-fixes/2026-10-25/native_bridge.lua")
	if err != nil {
		t.Fatal(err)
	}
	L := lua.NewState()
	defer L.Close()
	if err = L.DoString(`
 reg={items={['1']={name='menu.txt',enabled=false,loaded=true,delay_sec=.05}}}
 running=false;starts=0;stops=0;reloads=0;saves=0;failStop=false
 notices={};text='2,0,0 20\n@wait {"command":"testfor @a","edge":false}\nscoreboard players add probe test 1'
 chains={list=function()return {files={{config={name='menu.txt',enabled=running,hz=20},state=running and '运行中' or '已停止',error=''}}}end,
 start=function()starts=starts+1;running=true;return {ok=true}end,
 stop=function()stops=stops+1;if failStop then error('stop timeout') end;running=false;return {ok=true}end,
 reload=function()reloads=reloads+1;return {ok=true}end,
 save=function()saves=saves+1;error('toggle must not rewrite or restart through save')end}
 function function_registry_load()return reg end
 function function_registry_get(id)local d=function_registry_load();return d.items[tostring(id)],d end
 function function_registry_save(d)reg=d end
 function function_read_text()return text end
 function function_parse_document()error('legacy parser was used')end
 function function_serialize_nodes(nodes)local s='';for _,n in ipairs(nodes)do s=s..'/'..n.command..'\n'end;return s end
 function function_write_text()return true end
 function is_admin()return true end
 function send(_,s)notices[#notices+1]=s end
 util={schedule=function()return 1 end};game={};plugin={};mcp={};http={}
 `); err != nil {
		t.Fatal(err)
	}
	chains := L.GetGlobal("chains").(*lua.LTable)
	L.SetField(chains, "validate", L.NewFunction(func(L *lua.LState) int {
		plan, e := yyParseChains(L.CheckString(1))
		out := map[string]any{"ok": e == nil}
		if e != nil {
			out["error"] = e.Error()
		} else {
			out["plan"] = plan
		}
		raw, _ := json.Marshal(out)
		var value any
		_ = json.Unmarshal(raw, &value)
		L.Push(goToLua(L, value))
		return 1
	}))
	if err = L.DoString("bridge=(function()\n" + string(source) + "\nend)();bridge.afterCore()"); err != nil {
		t.Fatal(err)
	}
	if err = L.DoString(`
 local doc=function_parse_document(text,{require_end=true})
 assert(#doc.errors==0 and #doc.nodes==2 and doc.nodes[1].control.kind=='wait')
 assert(not function_serialize_nodes(doc.nodes):find('/@',1,true),'editor changed a control into a raw command')
 for i=1,100 do function_toggle('owner',1);assert(running);function_toggle('owner',1);assert(not running) end
 assert(starts==100 and stops==100 and reloads==100 and saves==0,'duplicate start/save on toggle')
 text='3,0,0\nsay invalid';function_toggle('owner',1)
 assert(starts==100 and not running,'invalid TXT started')
 text='2,0,0 20\nsay valid';function_toggle('owner',1);failStop=true;function_toggle('owner',1)
 assert(running and function_registry_get(1).enabled,'failed stop was reported stopped')
 `); err != nil {
		t.Fatal(err)
	}
}
