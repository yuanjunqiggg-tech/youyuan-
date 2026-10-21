package main

import (
	"strings"
	"testing"
)

func TestFastArrayDoesNotIgnoreExplicitTransport(t *testing.T) {
	_, err := yyPluginCommandCall(nil, "game.sendCommandsFast", []any{[]any{"say test"}, map[string]any{"mode": "submit", "transport": "pyrpc"}}, yyDefaultCommandOptions())
	if err == nil || !strings.Contains(err.Error(), "transport") {
		t.Fatalf("explicit transport silently ignored: %v", err)
	}
}

func TestFastSettingsPreservesBotContext(t *testing.T) {
	// Both commands are bare (no leading execute), so they rely on @s and must
	// keep the bot executor wrapper.
	if !yyNeedsBotContext("scoreboard players add @s test 1") || !yyNeedsBotContext("say test") {
		t.Fatal("bare commands must be detected as needing bot context")
	}
	p := yyFastSettingsPacket("scoreboard players add @s test 1", "机器人", true)
	if !p.SuppressOutput || p.CommandLine != `execute as @a[name="机器人"] at @s run scoreboard players add @s test 1` {
		t.Fatalf("lost bot context: %+v", p)
	}
	p = yyFastSettingsPacket("say test", `a"b\c`, true)
	if p.CommandLine != `execute as @a[name="a\"b\\c"] at @s run say test` {
		t.Fatalf("unsafe selector: %s", p.CommandLine)
	}
}
