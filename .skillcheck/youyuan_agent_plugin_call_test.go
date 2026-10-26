package main

import (
	"context"
	"testing"
)

func TestAgentCanCallResidentPluginWithoutLosingOptOut(t *testing.T) {
	ws := &yyWorkspace{Language: "lua"}
	if !yyAgentAllowed(ws, "call_plugin_function") {
		t.Fatal("plugin function missing from default tool directory")
	}
	disabled := &yyWorkspace{Language: "lua", AllowedTools: []string{}}
	if yyAgentAllowed(disabled, "call_plugin_function") {
		t.Fatal("explicit player opt-out ignored")
	}
	bm := &BotManager{}
	if e := LoadLuaPlugin(bm, "agent_function_test", `counter=0;function next_count() counter=counter+1;return counter end`); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(bm, "agent_function_test")
	for _, want := range []string{"1", "2"} {
		result, failed := yyWorkspaceTool(context.Background(), ws, "call_plugin_function", map[string]any{"plugin_id": "agent_function_test", "func_name": "next_count"})
		if failed || result != want {
			t.Fatalf("resident call got %s error=%v want %s", result, failed, want)
		}
	}
}
