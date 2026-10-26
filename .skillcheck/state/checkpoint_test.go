package state

import (
	"encoding/json"
	"testing"
)

func TestCheckpointRestartRetainsLatestPositionAndParameters(t *testing.T) {
	dir := t.TempDir()
	if err := InitCheckpointDB(dir); err != nil {
		t.Fatal(err)
	}
	cp := &Checkpoint{TaskID: "import-a", Type: "import", Status: "running", Progress: .4, Data: json.RawMessage(`{"LastCX":4}`), Params: json.RawMessage(`{"path":"owned.mcstructure","x":125}`)}
	if e := SaveCheckpoint(cp); e != nil {
		t.Fatal(e)
	}
	cp.Progress = .9 // Saving must snapshot the checkpoint, not retain a mutable pointer.
	other := &Checkpoint{TaskID: "import-b", Type: "import", Status: "running", Progress: .2}
	if e := SaveCheckpoint(other); e != nil {
		t.Fatal(e)
	}
	if e := CloseDB(); e != nil {
		t.Fatal(e)
	}
	if e := InitCheckpointDB(dir); e != nil {
		t.Fatal(e)
	}
	defer CloseDB()
	got, e := LoadCheckpoint("import-a")
	if e != nil {
		t.Fatal(e)
	}
	if got.Status != "paused" || got.Progress != .4 || string(got.Data) != `{"LastCX":4}` || string(got.Params) != string(cp.Params) {
		t.Fatalf("lost/replaced checkpoint: %#v", got)
	}
	if _, e = LoadCheckpoint("import-b"); e != nil {
		t.Fatal("second task overwrote first pending checkpoint", e)
	}
	if e = SaveCheckpoint(&Checkpoint{TaskID: "deleted", Status: "running"}); e != nil {
		t.Fatal(e)
	}
	_ = DeleteCheckpoint("deleted")
	FlushCheckpoints()
	if _, e = LoadCheckpoint("deleted"); e == nil {
		t.Fatal("pending write resurrected deleted checkpoint")
	}
}
