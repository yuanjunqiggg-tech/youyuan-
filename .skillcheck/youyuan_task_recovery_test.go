package main

import (
	"bot-apk/building"
	"bot-apk/state"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportStopAndDisconnectRemainResumable(t *testing.T) {
	tc := state.NewTaskController(nil)
	if status, _ := yyImportInterruptedOutcome(tc, false, nil); status != "" {
		t.Fatal(status)
	}
	if status, msg := yyImportInterruptedOutcome(tc, false, errors.New("server refused")); status != "failed" || msg != "server refused" {
		t.Fatal(status, msg)
	}
	tc.Stop()
	if status, _ := yyImportInterruptedOutcome(tc, false, errors.New("cancelled")); status != "paused" {
		t.Fatal(status)
	}
	if status, _ := yyImportInterruptedOutcome(state.NewTaskController(nil), true, errors.New("offline")); status != "paused" {
		t.Fatal(status)
	}
}

func TestStoppingTaskHoldsSlotUntilRunnerExits(t *testing.T) {
	old := activeTask
	defer func() { activeTask = old }()
	tc := state.NewTaskController(nil)
	activeTask = tc
	handleTaskStop(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/task/stop", strings.NewReader("{}")))
	if activeTask != tc || tc.Running() {
		t.Fatal("stopped task released its slot before cleanup")
	}
	r := httptest.NewRecorder()
	handleTaskStart(r, httptest.NewRequest("POST", "/api/task/start", strings.NewReader(`{"type":"import","params":{}}`)))
	var result map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &result)
	if result["ok"] != false || !strings.Contains(result["error"].(string), "停止清理") {
		t.Fatal(r.Body.String())
	}
}
func TestMultiCheckpointRejectsChangedSource(t *testing.T) {
	dir := t.TempDir()
	if e := state.InitCheckpointDB(dir); e != nil {
		t.Fatal(e)
	}
	defer state.CloseDB()
	file := filepath.Join(dir, "owned.mcstructure")
	_ = os.WriteFile(file, []byte("version-one"), 0600)
	cp, meta, e := yyLoadMultiResume("", file, 3)
	if e != nil {
		t.Fatal(e)
	}
	cp.Data, _ = json.Marshal(meta)
	if e = state.SaveCheckpointNow(cp); e != nil {
		t.Fatal(e)
	}
	_, saved, e := yyLoadMultiResume(cp.TaskID, file, 1)
	if e != nil || saved.SplitBots != 3 {
		t.Fatal("worker count must not change split layout", e)
	}
	_ = os.WriteFile(file, []byte("version-two"), 0600)
	if _, _, e = yyLoadMultiResume(cp.TaskID, file, 3); e == nil {
		t.Fatal("changed source accepted")
	}
}
func TestSelectedImportWorkersNeverPullUnselectedMain(t *testing.T) {
	main := &BotManager{display: "main", serverCode: "same"}
	child := &BotManager{display: "child", serverCode: "same"}
	other := &BotManager{display: "other", serverCode: "different"}
	got, e := yySelectImportWorkers(main, []*BotManager{main, child, other}, map[string]any{"bot_names": []any{"child"}})
	if e != nil || len(got) != 1 || got[0] != child {
		t.Fatal("unselected robot participated", e)
	}
	if _, e = yySelectImportWorkers(main, []*BotManager{main, child, other}, map[string]any{"bot_names": []any{"other"}}); e == nil {
		t.Fatal("cross-server worker allowed")
	}
}
func TestImportLayoutHashIgnoresUnorderedCoordinates(t *testing.T) {
	a := &building.SplitResult{Units: []*building.Unit{{ID: 0, Kind: building.UnitNormal, Chunks: [][2]int{{1, 0}, {0, 0}}}}}
	b := &building.SplitResult{Units: []*building.Unit{{ID: 0, Kind: building.UnitNormal, Chunks: [][2]int{{0, 0}, {1, 0}}}}}
	if yyImportLayoutHash(a) != yyImportLayoutHash(b) {
		t.Fatal("equivalent regions changed identity")
	}
}

func TestFleetIncludesMainWithoutStartingChildren(t *testing.T) {
	old := getBotManager()
	main := &BotManager{display: "main", connected: true, inGame: true}
	botMgrMu.Lock()
	botMgr = main
	botMgrMu.Unlock()
	defer func() { botMgrMu.Lock(); botMgr = old; botMgrMu.Unlock() }()
	got := GetFleetBots()
	if len(got) == 0 || got[0] != main {
		t.Fatal("ready main missing before FleetConnect")
	}
	main.mu.Lock()
	main.inGame = false
	main.mu.Unlock()
	for _, bm := range GetFleetBots() {
		if bm == main {
			t.Fatal("unready main was scheduled")
		}
	}
}
