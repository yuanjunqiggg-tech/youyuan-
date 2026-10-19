package main

import (
	"strings"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// A reply to testforblock is not automatically evidence of a loaded column:
// outOfWorld, missing permission and unknown commands are replies too.
// A successful match or the expected block-mismatch result proves the query
// reached a block position. This does not confirm any preceding placement.
func yyImportChunkProbeLoaded(response *packet.CommandOutput, timedOut bool, err error) bool {
	if timedOut || err != nil || response == nil {
		return false
	}
	for _, message := range response.OutputMessages {
		if strings.TrimPrefix(message.Message, "%") == "commands.testforblock.outOfWorld" {
			return false
		}
	}
	if response.SuccessCount > 0 {
		return true
	}
	for _, message := range response.OutputMessages {
		if strings.TrimPrefix(message.Message, "%") == "commands.testforblock.failed" {
			return true
		}
	}
	return false
}
