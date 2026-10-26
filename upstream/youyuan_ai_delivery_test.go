package main

import (
	"os"
	"strings"
	"testing"
)

func TestFishingHookFiltersBeforeVMAndPreservesFallback(t *testing.T) {
	raw, e := os.ReadFile("../incident-fixes/2026-10-26/modules/yy_gun_recover_init.lua")
	if e != nil {
		t.Fatal(e)
	}
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	a := strings.Index(s, "  if events then\n")
	if a < 0 {
		t.Fatal("registration missing")
	}
	b := strings.Index(s[a:], "  if util and type(util.scheduleSeconds)")
	if a < 0 || b < 0 {
		t.Fatal("registration missing")
	}
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	setup := `filters={};legacy=0;events={}
events.subscribeFiltered=function(kind,filter,handler) assert(kind=='AddActor');filters[#filters+1]=filter.fields.entity_type;return 1 end
events.onEntitySpawn=function()legacy=legacy+1 end
events.subscribe=function()legacy=legacy+1 end
`
	if e := L.DoString(setup + "function register_test()\n" + s[a:a+b] + "\nend\n" + `
register_test();assert(#filters==4 and legacy==0)
local calls=0
for i=1,10000 do for _,kind in ipairs(filters)do if kind=='minecraft:pig' then calls=calls+1 end end end
assert(calls==0)
for _,kind in ipairs(filters)do if kind=='minecraft:fishing_hook'then calls=calls+1 end end
assert(calls==1)
events.subscribeFiltered=nil;register_test();assert(legacy==3)
`); e != nil {
		t.Fatal(e)
	}
}

func TestAIPatchedActualSourcesCompile(t *testing.T) {
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	for _, p := range []string{"ds_ai_agent_r21b_test.lua", "modules/core.lua", "modules/yy_gun_recover_init.lua", "modules/tool_fix.lua"} {
		raw, e := os.ReadFile("../incident-fixes/2026-10-26/" + p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = L.LoadString(string(raw)); e != nil {
			t.Fatal(p, e)
		}
	}
}

func TestActualToolFixOwnsOrdinaryReplyBeforeToken(t *testing.T) {
	raw, e := os.ReadFile("../incident-fixes/2026-10-26/modules/tool_fix.lua")
	if e != nil {
		t.Fatal(e)
	}
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	a := strings.Index(s, "  if is_fn(on_chat) then")
	if a < 0 {
		t.Fatal("chat wrapper missing")
	}
	b := strings.Index(s[a:], `  log("tool_fix.apply`)
	if b < 0 {
		t.Fatal("chat wrapper end missing")
	}
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	setup := `sent={};function is_fn(v)return type(v)=='function'end
function tell()error('ordinary AI must use the verified sender')end
function broadcast()error('must preserve broadcast intent in shared sender')end
function on_chat()return 'legacy' end
function run_agent()return '完整普通AI正文' end
AI_USAGE={task=10}
function ai_usage_window_add()return 20 end
function require(name)assert(name=='modules.yy_ai_delivery');return {send=function(name,text,public)sent[#sent+1]={name=name,text=text,public=public};return true end} end
`
	checks := `
assert(on_chat('player','ai 测试'))
assert(#sent==2 and sent[1].text:find('完整普通AI正文',1,true) and sent[2].text:find('Token',1,true))
assert(sent[1].public==false and sent[2].public==false)
sent={};assert(on_chat('player','AI 测试'));assert(sent[1].public and sent[2].public)
sent={};run_agent=function()error('model unavailable')end;assert(on_chat('player','ai 测试'))
assert(sent[1].text:find('处理失败',1,true) and sent[2].text:find('Token',1,true))
assert(on_chat('player','ordinary chat')=='legacy')
`
	if e := L.DoString(setup + s[a:a+b] + checks); e != nil {
		t.Fatal(e)
	}
}

func TestOrdinaryAI200CharactersWithoutTickOrSilentTruncation(t *testing.T) {
	raw, e := os.ReadFile("../incident-fixes/2026-10-26/modules/yy_ai_delivery.lua")
	if e != nil {
		t.Fatal(e)
	}
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	setup := `timers={};util={setTimeout=function(sec,name)assert(sec==0.3);timers[#timers+1]=name end}
 function flush()local n=0;while #timers>0 do local name=table.remove(timers,1);_G[name]();n=n+1;assert(n<100)end end
 sent={};game.sendChat=function(name,text) sent[#sent+1]={name=name,text=text} end
 is_admin=function(name)return name=='admin'end
 on_chat=function(name,text) yy_ai_delivery_final(name,'正常正文',false);muse_deliv_push(name,'Token 10',false);return true end
 agent_echo=function(name,text)game.sendChat(name,text)end
 `
	test := `
 assert(M.apply())
 local text=string.rep('中文🙂',66)..'结束'
 assert(#M.parts(text)==1)
 assert(yy_ai_delivery_final('admin',text,false));flush()
 assert(#sent==1 and sent[1].text==text)
 sent={};text=text..'后'
 assert(yy_ai_delivery_final('admin',text,false));flush()
 assert(#sent==2 and sent[1].text..sent[2].text==text)
 local multiline='第一行\n第二行';sent={};muse_deliv_push('admin',multiline,false);flush()
 assert(#sent==1 and sent[1].text==multiline)
 sent={};assert(plugin_api.yyAiChatQuery('admin','你好'));flush()
 assert(#sent==2 and sent[1].text=='正常正文' and sent[2].text=='Token 10')
 assert(plugin_api.yyAiChatQuery('other','你好').error)
 sent={};local long=string.rep('中',2001);agent_send_long('admin',long);flush()
 assert(#sent==11)
 local parts={};for _,x in ipairs(sent) do parts[#parts+1]=x.text end;assert(table.concat(parts)==long)
 sent={};game.sendChat=function(name,text)if #sent==1 then error('closed')end;sent[#sent+1]=text end
 local accepted,s=M.send('admin',string.rep('中',401),false);flush()
 assert(accepted and s.submitted==1 and s.failed==1 and s.confirmed==false)
 `
	if e := L.DoString(setup + "local M=(function()\n" + string(raw) + "\nend)()\n" + test); e != nil {
		t.Fatal(e)
	}
}

func TestOrdinaryAIPublicFanoutAndExplicitSendFailure(t *testing.T) {
	raw, e := os.ReadFile("../incident-fixes/2026-10-26/modules/yy_ai_delivery.lua")
	if e != nil {
		t.Fatal(e)
	}
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	setup := `timers={};util={setTimeout=function(sec,name)assert(sec==0.3);timers[#timers+1]=name end}
 function flush()local n=0;while #timers>0 do local name=table.remove(timers,1);_G[name]();n=n+1;assert(n<100)end end
 sent={};game.sendChat=function(n,text)assert(n~='@a');sent[#sent+1]={n,text};return true end
 function online_players()return {'alice','bob','alice'}end
 `
	checks := `assert(M.apply());local ok,st=M.send('alice',string.rep('中',201),true);assert(#sent==1 and st.pending==3);flush();assert(ok and #sent==4 and st.submitted==4)
 sent={};game.sendChat=function(n,text)sent[#sent+1]={n,text};return false end
 ok,st=M.send('alice',string.rep('中',401),false);assert(not ok and #sent==1 and st.failed==1 and st.submitted==0)
 `
	if e := L.DoString(setup + "local M=(function()\n" + string(raw) + "\nend)()\n" + checks); e != nil {
		t.Fatal(e)
	}
}
