package main

import (
	lua "github.com/yuin/gopher-lua"
	"os"
	"strings"
	"testing"
)

// Opt-in attachment regression. The stubs deliberately do not connect to a
// game or provider; live integration is recorded by the separate Linux test.
func TestReceivedR21BDeathAndThinking(t *testing.T) {
	dir := os.Getenv("YY_R21B_TEST_DIR")
	if dir == "" {
		t.Skip("received R21B not selected")
	}
	patch, err := os.ReadFile(dir + "/patch.lua")
	if err != nil {
		t.Fatal(err)
	}
	core, err := os.ReadFile(dir + "/core.lua")
	if err != nil {
		t.Fatal(err)
	}
	base := `
 function trim(x) return (tostring(x or ''):gsub('^%s+',''):gsub('%s+$','')) end
 function lower(x) return string.lower(x) end
 function known_player_names() return {'测试玩家'} end
 local db={}; notices=0; records={}; sequence={}; now=100
 COMMAND_WORD='指令'; AI_TAG_BLUE='[Ds]';AI_THINK_TEXT='正在思考中'
 util={timestamp=function() return now end,strip_color=function(x) return tostring(x):gsub('§.','') end,write_file=function() end}
 game={dataDir=function() return '/tmp/r21b-mock' end,sendActionbar=function() end}
 events={subscribe=function() end}
 function ai_predict_wait() sequence[#sequence+1]='prediction';return 8,'chat' end
 function ai_think_db() sequence[#sequence+1]='read';return db end
 function ai_think_save(d) sequence[#sequence+1]='write';db=d end
 function periodic_beat_now() return now end
 function send() notices=notices+1;sequence[#sequence+1]='notice' end
 broadcast=send
 function ai_think_end(name) db[name]=nil end
 function on_chat(name,msg) ai_think_begin(name,msg,false,false);return true end
 function tick_coordinate_and_soul() end
 function on_init() end
 function r20x_death_diag() end
 function log_event() end
 function ds_death_record(name,reason) records[#records+1]={name=name,reason=reason} end
 function r19_name_by_rid(rid) if rid==7 then return '测试玩家' end end
 final_death_name_by_uid=r19_name_by_rid
 function final_death_find_payload(f) return f end
 `
	start := strings.Index(string(core), "function ai_think_begin(")
	end := strings.Index(string(core)[start:], "\nfunction ai_think_end(")
	originalBegin := string(core)[start : start+end]
	cases := []struct{ name, script string }{
		{"text_data", `final_death_text_handler({message='death.attack.generic',parameters={'测试玩家'}});assert(records[1].name=='测试玩家')`},
		{"text_desc_data_fields", `final_death_text_handler('Text',{fields={message='death.attack.generic',parameters={'测试玩家'}}});assert(records[1].name=='测试玩家')`},
		{"text_plain_chinese", `final_death_text_handler({message='测试玩家死了'});assert(records[1].name=='测试玩家')`},
		{"health_zero_only", `r21b_update_attributes_handler({entity_runtime_id=7,attributes={{attribute_value={name='minecraft:health',value=20}}}});assert(#records==0);r21b_update_attributes_handler({entity_runtime_id=7,attributes={{attribute_value={name='minecraft:health',value=0}}}});assert(records[1].name=='测试玩家')`},
		{"actor_death_only", `r21b_actor_event_handler({entity_runtime_id=7,event_type=2});assert(#records==0);r21b_actor_event_handler('ActorEvent',{fields={entity_runtime_id=7,event_type=3}});assert(records[1].name=='测试玩家')`},
		{"location", `final_death_location_handler({type=1,entity_unique_id=7});assert(records[1].name=='测试玩家')`},
		{"pyrpc", `final_death_pyrpc_handler({die=true,pid=7});assert(records[1].name=='测试玩家')`},
		{"ai_deduplicates_nested_begin", `on_chat('测试玩家','ai 你好');assert(notices==1,notices)`},
		{"ai_original_notice_after_disk_and_prediction", `on_chat('测试玩家','ai 你好');assert(sequence[1]=='read');assert(sequence[2]=='prediction');assert(sequence[3]=='read');assert(sequence[4]=='write');assert(sequence[5]=='notice')`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			L := lua.NewState()
			defer L.Close()
			if err := L.DoString(base + originalBegin + "\n" + string(patch) + "\n" + c.script); err != nil {
				t.Fatal(err)
			}
		})
	}
}
