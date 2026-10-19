package main

import (
	"context"
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResidentTypedQueryWaitKeepsHealthCallbackResponsive(t *testing.T) {
	bm := &BotManager{}
	id := "typed-query-health"
	if e := LoadLuaPlugin(bm, id, `health=0;function read()return health end;function hit(e)health=e.health end;function query()return testQuery()end`); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(bm, id)
	r, e := yyGetResident(bm, id)
	if e != nil {
		t.Fatal(e)
	}
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	r.mu.Lock()
	r.L.SetGlobal("testQuery", r.L.NewFunction(func(L *lua.LState) int {
		v, err := yyLuaWaitValue(L, func(ctx context.Context) (bool, error) {
			close(started)
			select {
			case <-release:
				return true, nil
			case <-ctx.Done():
				return false, ctx.Err()
			}
		})
		if err != nil {
			L.RaiseError("%v", err)
		}
		L.Push(lua.LBool(v))
		return 1
	}))
	r.mu.Unlock()
	done := make(chan error, 1)
	go func() { _, err := yyCallResident(bm, id, "query", nil, false, false); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("query did not suspend")
	}
	yyQueueSenseLua(r, bm, id, "hit", []any{map[string]any{"type": "health", "health": 14}})
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		value, err := yyCallResident(bm, id, "read", nil, false, true)
		if err == nil && value == float64(14) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("a waiting Go query blocked the priority health callback")
}

func TestResidentHalfSecondTimerDuringWaitMsg(t *testing.T) {
	bm := &BotManager{stopCh: make(chan struct{})}
	id := "halfsecond-wait"
	if e := LoadLuaPlugin(bm, id, `n=0;function tick()n=n+1 end;function read()return n end;function wait()return game.waitMsg('probe',10)end;util.schedule(0.5,'tick')`); e != nil {
		t.Fatal(e)
	}
	defer RemovePluginSchedules(id)
	defer yyStopResident(bm, id)
	done := make(chan error, 1)
	go func() {
		value, e := yyCallResident(bm, id, "wait", nil, false, false)
		if e == nil && value != "hello" {
			e = fmt.Errorf("bad wait result %v", value)
		}
		done <- e
	}()
	deadline := time.Now().Add(time.Second)
	for !hasWaitMsg("probe") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !hasWaitMsg("probe") {
		t.Fatal("message wait never started")
	}
	time.Sleep(1650 * time.Millisecond)
	v, e := yyCallResident(bm, id, "read", nil, false, false)
	if e != nil || v.(float64) < 3 {
		t.Fatalf("half-second timer blocked during message wait: %v %v", v, e)
	}
	if !deliverWaitMsg("probe", "hello") {
		t.Fatal("message not delivered")
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not resume")
	}
}

func TestResidentTimerDuringHTTPAndStopCancelsWait(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { started <- struct{}{}; <-r.Context().Done() }))
	defer server.Close()
	bm := &BotManager{}
	id := "http-wait-cancel"
	src := fmt.Sprintf(`n=0;function tick()n=n+1 end;function read()return n end;function fetch()return http.get(%q)end;util.schedule(0.01,'tick')`, server.URL)
	if e := LoadLuaPlugin(bm, id, src); e != nil {
		t.Fatal(e)
	}
	defer RemovePluginSchedules(id)
	defer yyStopResident(bm, id)
	done := make(chan struct{})
	go func() { defer close(done); yyCallResident(bm, id, "fetch", nil, false, false) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("HTTP not started")
	}
	time.Sleep(65 * time.Millisecond)
	v, e := yyCallResident(bm, id, "read", nil, false, false)
	if e != nil || v.(float64) < 3 {
		t.Fatalf("HTTP blocked timer %v %v", v, e)
	}
	yyStopResident(bm, id)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel HTTP")
	}
}
