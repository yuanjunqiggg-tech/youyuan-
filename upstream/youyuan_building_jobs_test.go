package main

import (
	"bot-apk/state"
	"encoding/json"
	"testing"
)

func TestBuildingTaskAPIReadsRealCheckpoint(t *testing.T) {
	dir := t.TempDir()
	if e := state.InitCheckpointDB(dir); e != nil {
		t.Fatal(e)
	}
	defer state.CloseDB()
	cp := &state.Checkpoint{TaskID: "owned-resume", Type: "import_multi", Status: "paused", Params: json.RawMessage(`{"path":"owned.mcstructure"}`)}
	if e := state.SaveCheckpointNow(cp); e != nil {
		t.Fatal(e)
	}
	got, e := yyPublicGameCall(nil, "building.taskList", nil)
	if e != nil {
		t.Fatal(e)
	}
	rows := got.(map[string]any)["tasks"].([]any)
	found := false
	for _, r := range rows {
		if r.(map[string]any)["task_id"] == cp.TaskID {
			found = true
		}
	}
	if !found {
		t.Fatal("task API returned no durable record")
	}
	if _, e = yyPublicGameCall(nil, "building.startImport", []any{"not-an-object"}); e == nil {
		t.Fatal("invalid import accepted")
	}
}
