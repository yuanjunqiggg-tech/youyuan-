package main

import (
	"errors"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestImportChunkProbeRejectsUnloadedAndUnconfirmedReplies(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		success       uint32
		want          bool
	}{
		{"air match", "commands.testforblock.success", 1, true},
		{"solid block mismatch", "commands.testforblock.failed", 0, true},
		{"unloaded column", "commands.testforblock.outOfWorld", 0, false},
		{"out of world takes precedence", "commands.testforblock.outOfWorld", 1, false},
		{"permission denied", "commands.generic.permission", 0, false},
		{"unknown command", "commands.generic.unknown", 0, false},
		{"empty acknowledgement", "", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := &packet.CommandOutput{SuccessCount: tc.success, OutputMessages: []protocol.CommandOutputMessage{{Message: tc.message}}}
			if got := yyImportChunkProbeLoaded(response, false, nil); got != tc.want {
				t.Fatalf("loaded=%v want=%v", got, tc.want)
			}
		})
	}
	if yyImportChunkProbeLoaded(nil, false, nil) || yyImportChunkProbeLoaded(&packet.CommandOutput{SuccessCount: 1}, true, nil) || yyImportChunkProbeLoaded(&packet.CommandOutput{SuccessCount: 1}, false, errors.New("closed")) {
		t.Fatal("missing or interrupted response accepted")
	}
}
