package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func roomRequest(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	yyRoomDiscoveryHandler(w, httptest.NewRequest("POST", "/api/youyuan/rooms", strings.NewReader(string(raw))))
	var out map[string]any
	if json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatal(w.Body.String())
	}
	return out
}
func TestRoomDiscoverySelectionBoundToCredentialAndInput(t *testing.T) {
	old := yyAccounts.Root
	yyAccounts.Root = t.TempDir()
	defer func() { yyAccounts.Root = old }()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/game/discover" || r.Header.Get("Authorization") != "Bearer fixture-only" {
			t.Fatal("unexpected provider request")
		}
		writeJSON(w, yyRoomResult{OK: true, Complete: true, Candidates: []yyRoomCandidate{{ID: "lobby:4691202525342713927", Type: "lobby", RoomID: "4691202525342713927", Target: "LobbyGame:room_id=4691202525342713927"}}, Sources: []map[string]any{{"type": "rental", "ok": true}, {"type": "lobby", "ok": true}, {"type": "local", "ok": true}}})
	}))
	defer srv.Close()
	body := map[string]any{"token": "fixture-only", "auth": srv.URL, "input": "#11451"}
	r := roomRequest(t, body)
	if r["complete"] != true {
		t.Fatal(r)
	}
	body["op"] = "select"
	body["candidate_id"] = "lobby:4691202525342713927"
	if roomRequest(t, body)["ok"] != true {
		t.Fatal("valid selection failed")
	}
	delete(body, "op")
	body["op"] = "current"
	if current := roomRequest(t, body); current["choice"] == nil {
		t.Fatal("original input lost the selected room display", current)
	}
	body["input"] = "LobbyGame:room_id=4691202525342713927"
	if current := roomRequest(t, body); current["choice"] == nil {
		t.Fatal("normalized input could not restore original selection", current)
	}
	body["input"] = "#11451"
	delete(body, "op")
	r = roomRequest(t, body)
	if r["remembered"] != true || calls != 1 {
		t.Fatal(r, calls)
	}
	body["input"] = "#11452"
	r = roomRequest(t, body)
	if r["remembered"] == true || calls != 2 {
		t.Fatal("changed input reused choice", r, calls)
	}
	if yyRoomBinding("a", srv.URL, "x") == yyRoomBinding("b", srv.URL, "x") {
		t.Fatal("credential binding missing")
	}
}
func TestRoomCandidateRejectsCrossTypeAndPrecisionLoss(t *testing.T) {
	for _, v := range []yyRoomCandidate{{ID: "local:1", Type: "local", RoomID: "1", Target: "LobbyGame:room_id=1"}, {ID: "rental:1", Type: "rental", RoomID: "1", Target: "@1"}} {
		if yyRoomValidCandidate(v) {
			t.Fatal(v)
		}
	}
	r := roomRequest(t, map[string]any{"token": "x", "auth": "https://example.invalid", "input": "x", "op": "select", "candidate_id": "arbitrary"})
	if r["ok"] != false {
		t.Fatal(r)
	}
}
