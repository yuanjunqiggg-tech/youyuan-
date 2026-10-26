package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

var yyChainAllocPacket packet.Packet

func BenchmarkFastSettingsEscaper(b *testing.B) {
	for _, legacy := range []bool{true, false} {
		b.Run(fmt.Sprint("legacy=", legacy), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				const cmd, name = "execute as @e[type=armor_stand,tag=yy_surface_active] at @s if block ~ ~-1 ~ air run tp @s ~ ~-1 ~", "布汲岛"
				if legacy {
					escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(name)
					yyChainAllocPacket = &packet.SettingsCommand{CommandLine: fmt.Sprintf(`execute as @a[name="%s"] at @s run %s`, escaped, cmd), SuppressOutput: true}
				} else {
					yyChainAllocPacket = yyFastSettingsPacket(cmd, name, true)
				}
			}
		})
	}
}

func TestFastSettingsDoesNotRebuildNameEscaper(t *testing.T) {
	// Escaping and bot context are covered separately; this guards their hot-path cost.
	allocs := testing.AllocsPerRun(100, func() {
		yyChainAllocPacket = yyFastSettingsPacket("scoreboard players add @s test 1", "布汲岛", true)
	})
	if allocs > 3 {
		t.Fatalf("rebuilt per-command context machinery: %.0f allocations", allocs)
	}
}

func BenchmarkChainFastChunksAllocation(b *testing.B) {
	for _, count := range []int{60, 156, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			nodes := yyFastTestNodes(count)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := yyChainFastSubmit(context.Background(), nodes, yyChainConfig{}, "", "布汲岛", func(p []packet.Packet) (bool, error) {
					yyChainAllocPacket = p[len(p)-1]
					return true, nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
