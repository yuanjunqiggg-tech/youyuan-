package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPWriteTimeoutNeverSuggestsBlindReplay(t *testing.T) {
	for _, c := range []struct {
		verb   string
		status int
		p      map[string]any
		err    error
		out    string
		retry  bool
	}{
		{"POST", 200, map[string]any{"ok": false, "error": "context deadline exceeded"}, nil, "timeout_unknown", false},
		{"POST", 200, map[string]any{"ok": false, "error": "超时", "submitted": false}, nil, "timeout_not_sent", true},
		{"GET", 504, nil, errors.New("timeout"), "timeout_not_sent", true},
		{"POST", 503, nil, nil, "unavailable_write_unknown", false},
		{"GET", 503, nil, nil, "unavailable", true},
		{"POST", 200, map[string]any{"ok": true, "result": map[string]any{"confirmed": true, "success": false}}, nil, "server_rejected", false},
		{"POST", 200, map[string]any{"ok": true, "result": map[string]any{"submitted": true, "confirmed": false}}, nil, "submitted_unconfirmed", false},
	} {
		out, retry, _ := yyMCPOutcome(c.verb, c.status, c.p, c.err)
		if out != c.out || retry != c.retry {
			t.Fatalf("%+v -> %s/%v", c, out, retry)
		}
	}
}
func TestMCPTraceDoesNotLeakPayloadAndIsOwnerScoped(t *testing.T) {
	oldMux, oldRemote, oldRows := yyMux, yyRemote, yyMCPTraces.Rows
	defer func() { yyMux = oldMux; yyRemote = oldRemote; yyMCPTraces.Rows = oldRows }()
	yyMCPTraces.Rows = nil
	yyRemote.Key = "fixture-key"
	yyMux = http.NewServeMux()
	received := ""
	yyMux.HandleFunc("/api/test-write", func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Get("X-YouYuan-Trace-ID")
		writeJSON(w, map[string]any{"ok": false, "error": "timeout", "submitted": true})
	})
	yyMCPTraceRoutes(yyMux)
	r := httptest.NewRequest("POST", "/mcp/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"api_call","arguments":{"method":"POST","path":"/api/test-write","body":{"secret":"private-contents"}}}}`))
	r.Header.Set("Authorization", "Bearer fixture-key")
	r = r.WithContext(context.WithValue(r.Context(), yyPoolActorContext{}, yyPoolActor{ID: "a"}))
	w := httptest.NewRecorder()
	yyMCP(w, r)
	var response map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	result := response["result"].(map[string]any)
	meta := result["structuredContent"].(map[string]any)["mcp_meta"].(map[string]any)
	if meta["trace_id"] != received || meta["outcome"] != "timeout_unknown" || result["isError"] != true || strings.Contains(mustJSON(yyMCPTraces.Rows), "private-contents") {
		t.Fatal(w.Body.String(), received)
	}
	for _, owner := range []string{"a", "b"} {
		req := httptest.NewRequest("GET", "/api/youyuan/mcp/traces", nil)
		req = req.WithContext(context.WithValue(req.Context(), yyPoolActorContext{}, yyPoolActor{ID: owner}))
		out := httptest.NewRecorder()
		yyMux.ServeHTTP(out, req)
		if strings.Contains(out.Body.String(), received) != (owner == "a") {
			t.Fatal("cross owner trace", owner, out.Body.String())
		}
	}
}
func TestObservationSummaryKeepsCountsAndDropsHugePayloads(t *testing.T) {
	bm := senseBot(t)
	s := yySenseFor(bm)
	s.Lock()
	for i := 1; i <= 90; i++ {
		s.Events = append(s.Events, map[string]any{"sequence": uint64(i), "type": "swing", "runtime_id": "99", "data": strings.Repeat("x", 100000)})
	}
	s.Seq = 90
	s.Unlock()
	v := yyObservationSummary(bm, yySummaryOptions{After: 10, Limit: 3, Types: []string{"swing"}, RuntimeID: "99"}, false)
	if v["counts"].(map[string]int)["swing"] != 80 || len(v["recent"].([]map[string]any)) != 3 || len(mustJSON(v)) > 4000 {
		t.Fatal(v)
	}
	if len(yyPacketCatalog()) < 100 {
		t.Fatal("incomplete protocol catalog")
	}
}
func TestConfirmedRosterKeepsDeadPlayerOnlineWithoutCoordinates(t *testing.T) {
	pm := NewPlayerInfoManager()
	pm.players["dead"] = &PlayerInfo{Name: "dead", Online: true}
	pm.players["quit"] = &PlayerInfo{Name: "quit", Online: true}
	pm.applyConfirmedRoster([]string{"dead", "new"})
	if !pm.IsPlayerOnline("dead") || !pm.IsPlayerOnline("new") || pm.IsPlayerOnline("quit") {
		t.Fatal(pm.OnlineSet())
	}
	// No coordinate result is available during death; repeated roster stays online.
	pm.applyConfirmedRoster([]string{"dead", "new"})
	if !pm.IsPlayerOnline("dead") {
		t.Fatal("death was treated as session leave")
	}
}
