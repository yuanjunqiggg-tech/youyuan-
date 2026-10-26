package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"testing"
)

func TestTickingEmptyReplyIsNotACleanupFailure(t *testing.T) {
	for _, key := range []string{"commands.tickingarea-list.failure.allDimensions", "minecraft:commands.tickingarea-list.failure.allDimensions"} {
		got, err := yyParseTickingAreas(&packet.CommandOutput{SuccessCount: 0, OutputMessages: []protocol.CommandOutputMessage{{Message: key}}})
		if err != nil || len(got) != 0 {
			t.Fatalf("empty list rejected: %v %v", got, err)
		}
	}
	for _, key := range []string{"commands.generic.permission", "commands.tickingarea-add.failure", "commands.tickingarea-list.failure.allDimensions.extra"} {
		if _, err := yyParseTickingAreas(&packet.CommandOutput{OutputMessages: []protocol.CommandOutputMessage{{Message: key}}}); err == nil {
			t.Fatal("unknown failure accepted as empty", key)
		}
	}
}
