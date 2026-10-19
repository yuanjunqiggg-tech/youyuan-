package main

import (
	"bot-apk/state"
	"encoding/json"
	"strings"
	"testing"
)

func TestImportResumeDataRejectsCorruptionAndMissingPosition(t *testing.T) {
	for _, raw := range []string{"", "{", "null", "{}", `{"last_cx":0,"last_cz":0}`, `{"last_cx":"0","last_cz":0,"blocks_placed":0}`, `{"last_cx":null,"last_cz":0,"blocks_placed":0}`, `{"last_cx":-1,"last_cz":0,"blocks_placed":0}`} {
		if _, err := yyImportResumeData(&state.Checkpoint{Type: "import", Data: json.RawMessage(raw)}); err == nil {
			t.Fatalf("invalid resume accepted: %q", raw)
		}
	}
	if _, err := yyImportResumeData(nil); err == nil {
		t.Fatal("nil checkpoint accepted")
	}
	valid := json.RawMessage(`{"last_cx":0,"last_cz":31,"blocks_placed":32}`)
	if _, err := yyImportResumeData(&state.Checkpoint{Type: "export", Data: valid}); err == nil {
		t.Fatal("non-import checkpoint accepted")
	}
	for _, raw := range []json.RawMessage{valid, json.RawMessage(`{"last_cx":0,"last_cz":0,"blocks_placed":0}`)} {
		if _, err := yyImportResumeData(&state.Checkpoint{Type: "import", Data: raw}); err != nil {
			t.Fatalf("valid checkpoint rejected: %s %v", raw, err)
		}
	}
}

func TestImportResumeFailureCannotStartFreshTask(t *testing.T) {
	if err := state.InitCheckpointDB(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer state.CloseDB()
	previousHub := hub
	hub = NewSSEHub()
	defer func() { hub = previousHub }()
	events := make(chan string, 20)
	hub.Add(events)
	for _, tc := range []struct{ id, raw string }{
		{"missing", ""},
		{"wrong-field-type", `{"last_cx":"broken","last_cz":31,"blocks_placed":32}`},
		{"null", "null"},
		{"missing-position", "{}"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			if tc.raw != "" {
				if err := state.SaveCheckpointNow(&state.Checkpoint{TaskID: tc.id, Type: "import", Status: "paused", Data: json.RawMessage(tc.raw)}); err != nil {
					t.Fatal(err)
				}
			}
			before, err := state.ListCheckpoints()
			if err != nil {
				t.Fatal(err)
			}
			// Deliberately nonexistent source and nil controller. The resume
			// guard must return before parsing, bot access or any game action.
			runImportTask(nil, map[string]any{"path": "never-read.mcstructure", "pre_clear_mode": 3}, tc.id)
			select {
			case event := <-events:
				if !strings.Contains(event, "event: task_error") || !strings.Contains(event, "无法恢复导入") {
					t.Fatalf("resume failure reached fresh import: %s", event)
				}
			default:
				t.Fatal("resume failure was not reported")
			}
			if len(events) != 0 {
				t.Fatal("resume emitted task start/progress events")
			}
			after, err := state.ListCheckpoints()
			if err != nil || len(after) != len(before) {
				t.Fatalf("resume created/replaced a task: %v %v", after, err)
			}
			if tc.raw != "" {
				kept, err := state.LoadCheckpoint(tc.id)
				if err != nil || string(kept.Data) != tc.raw || kept.Status != "paused" {
					t.Fatalf("original checkpoint changed: %v %v", kept, err)
				}
			}
		})
	}
}
