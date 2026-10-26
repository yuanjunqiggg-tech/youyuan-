package main

import (
	"net/http"
	"testing"
	"time"
)

func TestChainStopWaitingForWorkerDoesNotHoldPluginVM(t *testing.T) {
	bm := &BotManager{}
	botMgrMu.Lock()
	oldBot := botMgr
	botMgr = bm
	botMgrMu.Unlock()
	oldMux := yyMux
	yyMux = http.NewServeMux()
	defer func() { yyMux = oldMux; botMgrMu.Lock(); botMgr = oldBot; botMgrMu.Unlock() }()
	started, release := make(chan struct{}), make(chan struct{})
	yyMux.HandleFunc("/api/youyuan/chains", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "GET" {
			w.Write([]byte(`{"ok":true,"files":[{"config":{"name":"slow.txt","account":"main"}}]}`))
			return
		}
		close(started)
		<-release
		w.Write([]byte(`{"ok":true}`))
	})
	const id = "chain-vm-pressure"
	if err := LoadLuaPlugin(bm, id, `health=0;function stop()return chains.stop('slow.txt')end;function hit(e)health=e.health end;function read()return health end`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	defer yyStopResident(bm, id)
	defer func() { close(release); <-done }()
	go func() { _, err := yyCallResident(bm, id, "stop", nil, false, false); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("chain stop did not reach worker wait")
	}
	r, _ := yyGetResident(bm, id)
	yyQueueSenseLua(r, bm, id, "hit", []any{map[string]any{"type": "health", "health": 12}})
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		v, err := yyCallResident(bm, id, "read", nil, false, true)
		if err == nil && v == float64(12) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("waiting for chain exit monopolized the plugin VM")
}
