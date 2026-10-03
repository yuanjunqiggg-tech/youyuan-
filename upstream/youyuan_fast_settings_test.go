package main

import "testing"

func TestFastSettingsPreservesBotContext(t *testing.T) {
	p := yyFastSettingsPacket("scoreboard players add @s test 1", "机器人")
	if !p.SuppressOutput || p.CommandLine != `execute as @a[name="机器人"] at @s run scoreboard players add @s test 1` {
		t.Fatalf("lost bot context: %+v", p)
	}
	p = yyFastSettingsPacket("say test", `a"b\c`)
	if p.CommandLine != `execute as @a[name="a\"b\\c"] at @s run say test` {
		t.Fatalf("unsafe selector: %s", p.CommandLine)
	}
}
