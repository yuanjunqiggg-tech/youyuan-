package main

import (
	"strings"
	"testing"
)

func TestFastSettingsLegacyContextForExecuteCommands(t *testing.T) {
	for _, command := range []string{
		"tp @s ~ ~-1 ~",
		"execute if entity @s run tp @s ~ ~1 ~",
		"execute unless block ~ ~-1 ~ air run tp @s ~ ~1 ~",
		"execute in nether run tp @s ~ ~ ~",
		"execute positioned ~1 ~ ~ run tp @s ~ ~ ~",
		"execute at @e[type=snowball] as @p[r=3] run scoreboard players add @s menu 1",
		"/execute as @a[tag=return_surface] at @s run tp @s ~ ~-1 ~",
	} {
		t.Run(command, func(t *testing.T) {
			if !yyNeedsBotContext(command) {
				t.Fatal("execute syntax must not remove the legacy bot executor")
			}
			for _, dim := range []string{"overworld", "nether"} {
				wrapped := strings.TrimPrefix(wrapCommandInDimension(command, dim), "/")
				packet := yyFastSettingsPacket(wrapped, `Bot"Name`, yyNeedsBotContext(command))
				want := `execute as @a[name="Bot\"Name"] at @s run ` + wrapped
				if !packet.SuppressOutput || packet.CommandLine != want {
					t.Fatalf("dimension %q changed the legacy executor, position or inner command: got=%q want=%q", dim, packet.CommandLine, want)
				}
			}
		})
	}
}

func TestFastSettingsLegacyContextWithoutBotName(t *testing.T) {
	command := "execute if entity @s run tp @s ~ ~ ~"
	packet := yyFastSettingsPacket(command, "", yyNeedsBotContext(command))
	if packet.CommandLine != command || !packet.SuppressOutput {
		t.Fatalf("empty bot name must retain the legacy command unchanged: %+v", packet)
	}
}
