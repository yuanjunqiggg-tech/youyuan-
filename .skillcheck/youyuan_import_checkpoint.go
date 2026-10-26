package main

import (
	"bot-apk/state"
	"encoding/json"
)

// Persist the resumed position before the caller retires the old task. A
// process exit before its first progress callback must not erase that position
// and turn the next resume into a fresh pre-clear at the structure origin.
func yyAdoptImportCheckpoint(current, previous *state.Checkpoint) error {
	current.Data = append(json.RawMessage(nil), previous.Data...)
	current.Progress = previous.Progress
	current.Message = "恢复断点已接管，等待当前区域提交"
	return state.SaveCheckpointNow(current)
}
