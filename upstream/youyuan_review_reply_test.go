package main

import (
	lua "github.com/yuin/gopher-lua"
	"os"
	"strings"
	"testing"
)

func TestReviewReplySplitHandlesBlankLinesAndNestedUTF8Iteration(t *testing.T) {
	raw, err := os.ReadFile("../incident-fixes/2026-10-25/ds_ai_agent_r21b_test.lua")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "local RS_SEND_LIMIT = 200")
	end := strings.Index(source[start:], "-- ===== 会话记忆")
	if start < 0 || end < 0 {
		t.Fatal("reply implementation missing")
	}
	L := newYYLuaVM()
	L.Options.IncludeGoStackTrace = true
	defer L.Close()
	if err := L.DoString(source[start:start+end] + "\nfunction split_test(s) return rs_split_body(s,200) end"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"abc\ndef", "abc\n\ndef", "\n", strings.Repeat("中文🙂", 150)} {
		if err := L.CallByParam(lua.P{Fn: L.GetGlobal("split_test"), NRet: 1, Protect: true}, lua.LString(s)); err != nil {
			t.Fatalf("input=%q: %v", s, err)
		}
		chunks := L.Get(-1).(*lua.LTable)
		if chunks.Len() == 0 && strings.TrimSpace(s) != "" {
			t.Fatal("reply lost")
		}
		L.Pop(1)
	}
}

func TestReviewLuaConcatReceiver(t *testing.T) {
	for _, source := range []string{
		`local text="a";assert(tostring(text).."\n"=="a\n")`,
		`local text="a";for line in (tostring(text).."\n"):gmatch("(.-)\n") do assert(line=="a") end`,
		`local text="a";local lines=tostring(text).."\n";for line in lines:gmatch("(.-)\n") do assert(line=="a") end`,
	} {
		L := newYYLuaVM()
		fn, e := L.LoadString(source)
		if e != nil {
			t.Fatal(e)
		}
		err := L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true})
		if err != nil {
			t.Log(fn.Proto.String())
		}
		L.Close()
		if err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
}

func TestReviewReplyRecordsActualGameConfirmation(t *testing.T) {
	raw, err := os.ReadFile("../incident-fixes/2026-10-25/ds_ai_agent_r21b_test.lua")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "local RS_SEND_LIMIT = 200")
	end := strings.Index(source[start:], "-- ===== 会话记忆")
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	setup := `sent={};writes=0;confirmed=true
game.execute=function(command,options)
 assert(options.channel=='ai' and options.timeout_ms==2500)
 assert(#command<=180,'oversized wire command')
 sent[#sent+1]=command
 return {confirmed=confirmed,success=confirmed}
end
game.sendChat=function()error('unconfirmed fallback must not duplicate a reply')end
game.dataDir=function()return '/test' end
util.write_file=function(path,body)writes=writes+1;assert(http.json_decode(body).segments>0)end
util.setTimeout=function(sec,handler)assert(sec==0.4 and handler=='museReviewDeliveryStep');_G[handler]();return 1 end
`
	if err := L.DoString(setup + source[start:start+end] + `
rs_send_reply('玩家"名',string.rep('中文🙂',150),'Token 100')
assert(rs_delivery.confirmed>1 and rs_delivery.failed==0)
assert(#sent==rs_delivery.segments and writes>0)
assert(sent[1]:find('@a[name="玩家\\\"名"]',1,true))
local text=string.rep('中文🙂"\\\n<&',100)
local rebuilt={}
for _,command in ipairs(rs_wire_commands('玩家"名',text)) do
 assert(#command<=180)
 rebuilt[#rebuilt+1]=http.json_decode(command:match('({.*})$')).rawtext[1].text
end
assert(table.concat(rebuilt)==text,'wire chunks lost or damaged UTF8/escaped text')
sent={};confirmed=false
rs_send_reply('玩家','短回复','')
assert(#sent==1 and rs_delivery.confirmed==0 and rs_delivery.failed==1)
`); err != nil {
		t.Fatal(err)
	}
}
