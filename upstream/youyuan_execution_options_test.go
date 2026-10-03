package main

import (
	"encoding/json"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestAgentSchemasHaveArrayRequiredEvenForNoArguments(t *testing.T) {
	for _, def := range yyAgentToolDefs() {
		encoded, err := json.Marshal(def.Parameters)
		if err != nil || strings.Contains(string(encoded), `"required":null`) {
			t.Fatalf("invalid schema %s: %s %v", def.Name, encoded, err)
		}
	}
}

func TestPluginCommandOptionsAreIsolatedAndValidated(t *testing.T) {
	first, second := sdkFixture(t, "cmd_first"), sdkFixture(t, "cmd_second")
	_, err := first.call("game.setCommandOptions", []any{map[string]any{"channel": "玩家", "mode": "pipeline", "window": 4}})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := first.call("game.getCommandOptions", nil)
	if err != nil || actual.(map[string]any)["channel"] != "player" {
		t.Fatalf("first: %v %v", actual, err)
	}
	other, err := second.call("game.getCommandOptions", nil)
	if err != nil || other.(map[string]any)["channel"] != "ai" {
		t.Fatalf("second defaults leaked: %v %v", other, err)
	}
	if _, err = first.call("game.setCommandOptions", []any{map[string]any{"channel": "bogus"}}); err == nil {
		t.Fatal("invalid channel accepted")
	}
	if _, err = yyMergeCommandOptions(yyDefaultCommandOptions(), map[string]any{"window": 0}); err == nil {
		t.Fatal("invalid window accepted")
	}
	if _, err = yyMergeCommandOptions(yyDefaultCommandOptions(), map[string]any{"mode": "unordered"}); err == nil {
		t.Fatal("invalid mode accepted")
	}
	base := yyDefaultCommandOptions()
	base.Channel = "player"
	one, err := yyMergeCommandOptions(base, "console")
	if err != nil || one.Channel != "console" || base.Channel != "player" {
		t.Fatal("per-call override changed base", one, base, err)
	}
}

func TestLuaHasSameCommandDefaultsWithoutGameConnection(t *testing.T) {
	state := newLuaState(nil)
	defer state.Close()
	defer cancelLuaCtx(state)
	err := state.DoString(`game.setCommandOptions({channel="控制台",mode="pipeline",window=3}); options=game.getCommandOptions()`)
	if err != nil {
		t.Fatal(err)
	}
	options := state.GetGlobal("options").(*lua.LTable)
	if options.RawGetString("channel").String() != "console" || options.RawGetString("window") != lua.LNumber(3) {
		t.Fatal(options)
	}
	if err = state.DoString(`game.setCommandOptions({timeout_ms=1})`); err == nil {
		t.Fatal("invalid timeout accepted")
	}
	if err = state.DoString(`game.execute("testfor @a")`); err == nil {
		t.Fatal("offline execute accepted")
	}
}

func TestModelContextBudgetIncludesOutputAndSelectedCapacity(t *testing.T) {
	small := &AIModel{ContextWindow: 32768, MaxOutputTokens: 8192, CompressionPercent: 90}
	if small.compressionThreshold() != 24064 {
		t.Fatal(small.compressionThreshold())
	}
	large := &AIModel{ContextWindow: 1048576, MaxOutputTokens: 8192, CompressionPercent: 80}
	if large.compressionThreshold() != 838860 {
		t.Fatal(large.compressionThreshold())
	}
	if (&AIModel{}).contextWindow() != 32768 {
		t.Fatal("unsafe fallback capacity")
	}
	messages := []aiTurn{{Role: "user", Text: "保留目标"}}
	if yyEstimateContextTokens(messages, []ToolDef{{Name: "project_read", Description: "文件读取"}}) <= yyEstimateContextTokens(messages, nil) {
		t.Fatal("tool schemas excluded from context")
	}
}
