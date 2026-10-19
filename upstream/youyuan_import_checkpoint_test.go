package main

import (
	"bot-apk/state"
	"encoding/json"
	"testing"
)

func TestImportResumeHandoffSurvivesRestartBeforeFirstProgress(t *testing.T) {
	directory := t.TempDir()
	if err := state.InitCheckpointDB(directory); err != nil {
		t.Fatal(err)
	}
	defer state.CloseDB()
	old := &state.Checkpoint{TaskID: "old", Type: "import", Status: "paused", Progress: 32.0 / 34, Data: json.RawMessage(`{"last_cx":0,"last_cz":31,"blocks_placed":32}`), Params: json.RawMessage(`{"path":"owned.mcstructure","x":42}`)}
	if err := state.SaveCheckpointNow(old); err != nil {
		t.Fatal(err)
	}
	resumed := &state.Checkpoint{TaskID: "new", Type: "import", Status: "running", Params: append(json.RawMessage(nil), old.Params...)}
	if err := yyAdoptImportCheckpoint(resumed, old); err != nil {
		t.Fatal(err)
	}
	if err := state.DeleteCheckpoint(old.TaskID); err != nil {
		t.Fatal(err)
	}
	state.CloseDB()
	if err := state.InitCheckpointDB(directory); err != nil {
		t.Fatal(err)
	}
	got, err := state.LoadCheckpoint("new")
	if err != nil || string(got.Data) != string(old.Data) || string(got.Params) != string(old.Params) || got.Progress != old.Progress {
		t.Fatal("resume position lost before first progress", got, err)
	}
}
