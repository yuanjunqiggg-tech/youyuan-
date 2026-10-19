package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObserverBridgeKeyProvenanceReplayAndRevoke(t *testing.T) {
	b := &yyObserverBridge{Sources: map[string]yyObserverSource{}, Path: filepath.Join(t.TempDir(), "sources.json")}
	created, err := b.create(map[string]any{"name": "test player", "source": "player_input", "server": "94F195C41B5", "player_id": "self-uuid"})
	if err != nil {
		t.Fatal(err)
	}
	token, id := created["token"].(string), created["id"].(string)
	batch := yyObserverBatch{ID: id, Sequence: 1, Events: []map[string]any{{"kind": "swing_input", "source": "game_server", "server_confirmed": true, "player_id": "spoof"}}}
	if _, err := b.ingest(strings.Repeat("0", 64), batch); err == nil {
		t.Fatal("wrong key accepted")
	}
	if b.Sources[id].Sequence != 0 {
		t.Fatal("unauthorized call consumed sequence")
	}
	result, err := b.ingest(token, batch)
	if err != nil || result["accepted"] != 1 {
		t.Fatal(result, err)
	}
	row := b.Events[0]
	if row["source"] != "player_input" || row["player_id"] != "self-uuid" || row["server_confirmed"] != false {
		t.Fatal("external data replaced provenance", row)
	}
	result, err = b.ingest(token, batch)
	if err != nil || result["duplicate"] != true || len(b.Events) != 1 {
		t.Fatal("replay delivered twice", result, err)
	}
	list, err := b.call("observerBridge.sources", nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(list)
	if strings.Contains(string(encoded), token) || strings.Contains(string(encoded), "key_hash") {
		t.Fatal("source listing leaked key", string(encoded))
	}
	persisted, err := os.ReadFile(b.Path)
	if err != nil || strings.Contains(string(persisted), token) {
		t.Fatal("key persisted in plaintext", err)
	}
	_, err = b.call("observerBridge.revoke", []any{id})
	if err != nil {
		t.Fatal(err)
	}
	batch.Sequence = 2
	if _, err = b.ingest(token, batch); err == nil {
		t.Fatal("revoked source accepted")
	}
}

func TestObserverBridgePersistenceCursorAndCallerIsolation(t *testing.T) {
	b := &yyObserverBridge{Sources: map[string]yyObserverSource{}, Path: filepath.Join(t.TempDir(), "sources.json")}
	c, err := b.create(map[string]any{"name": "server", "source": "game_server", "server": "test"})
	if err != nil {
		t.Fatal(err)
	}
	id, token := c["id"].(string), c["token"].(string)
	for seq := uint64(1); seq <= 3; seq++ {
		if _, err = b.ingest(token, yyObserverBatch{ID: id, Sequence: seq, Events: []map[string]any{{"kind": "server_event", "data": map[string]any{"value": seq}}}}); err != nil {
			t.Fatal(err)
		}
	}
	v, err := b.call("observerBridge.events", []any{float64(0), float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	page := v.(map[string]any)
	if page["cursor"] != uint64(1) || len(page["events"].([]map[string]any)) != 1 {
		t.Fatal("pagination skipped unread events", page)
	}
	page["events"].([]map[string]any)[0]["source"] = "corrupted"
	if b.Events[0]["source"] != "game_server" {
		t.Fatal("API caller corrupted history")
	}
	raw, _ := os.ReadFile(b.Path)
	var restored map[string]yyObserverSource
	if json.Unmarshal(raw, &restored) != nil || restored[id].Sequence != 3 {
		t.Fatal("sequence not durable")
	}
	newB := &yyObserverBridge{Sources: restored}
	result, err := newB.ingest(token, yyObserverBatch{ID: id, Sequence: 3, Events: []map[string]any{{"kind": "server_event"}}})
	if err != nil || result["duplicate"] != true {
		t.Fatal("restart forgot replay protection", result, err)
	}
	if _, err = b.ingest(token, yyObserverBatch{ID: id, Sequence: 4, Events: []map[string]any{{"kind": "made-up"}}}); err == nil || b.Sources[id].Sequence != 3 {
		t.Fatal("invalid input consumed sequence")
	}
}
