package main

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerAcceptsSubMillisecondWithoutArtificialHzCeiling(t *testing.T) {
	for _, c := range []struct {
		method string
		value  float64
		want   time.Duration
	}{
		{"util.scheduleHz", 10000, 100 * time.Microsecond},
		{"util.scheduleSeconds", 0.0005, 500 * time.Microsecond},
		{"util.scheduleTick", 1, 50 * time.Millisecond},
	} {
		d, e := yyScheduleDuration(c.method, c.value)
		if e != nil || d != c.want {
			t.Fatalf("%s(%g): %v %v", c.method, c.value, d, e)
		}
	}
	for _, n := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1), 1e20} {
		if _, e := yyScheduleDuration("util.scheduleHz", n); e == nil {
			t.Fatalf("invalid Hz accepted %g", n)
		}
	}
}

func TestChainFrequencyKeepsSubMillisecondWholeRoundPeriod(t *testing.T) {
	p, e := yyParseChains("2,0,0 10000\nsay A\n0,0,0\nsay B\n!")
	if e != nil {
		t.Fatal(e)
	}
	if d := yyChainPeriod(p, yyChainConfig{IntervalMS: 1}); d != 100*time.Microsecond {
		t.Fatalf("clipped TXT Hz %v", d)
	}
	if d := yyChainPeriod(yyChainPlan{}, yyChainConfig{Hz: 2000, IntervalMS: 1}); d != 500*time.Microsecond {
		t.Fatalf("clipped config Hz %v", d)
	}
	if d := yyChainPeriod(yyChainPlan{}, yyChainConfig{IntervalMS: 25}); d != 25*time.Millisecond {
		t.Fatalf("legacy cadence changed %v", d)
	}
	if len(p.Chains) != 1 || len(p.Chains[0].Nodes) != 2 {
		t.Fatal("frequency split nodes")
	}
}

func TestSchedulerSkipsMissedAndCancelIsIdempotent(t *testing.T) {
	var concurrent, peak atomic.Int32
	id := newSchedJobWith("qa-scheduler", "slow", time.Millisecond, false, nil, func(<-chan struct{}) error {
		n := concurrent.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		time.Sleep(12 * time.Millisecond)
		concurrent.Add(-1)
		return nil
	}, nil)
	defer cancelSchedJob(id)
	schedMu.Lock()
	j := schedJobs[id]
	schedMu.Unlock()
	deadline := time.Now().Add(time.Second)
	for j.calls.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if peak.Load() != 1 || j.calls.Load() < 3 || j.missed.Load() == 0 {
		t.Fatalf("reentry/missed: %d %d %d", peak.Load(), j.calls.Load(), j.missed.Load())
	}
	if v, e := yySchedulerControl("other", nil, "util.pauseJob", []any{float64(id)}); e != nil || v != false {
		t.Fatal("cross-owner pause accepted")
	}
	if _, e := yySchedulerControl("qa-scheduler", nil, "util.pauseJob", []any{float64(id)}); e != nil {
		t.Fatal(e)
	}
	time.Sleep(20 * time.Millisecond)
	before := j.calls.Load()
	time.Sleep(20 * time.Millisecond)
	if j.calls.Load() != before {
		t.Fatal("paused task kept running")
	}
	yySchedulerControl("qa-scheduler", nil, "util.resumeJob", []any{float64(id)})
	deadline = time.Now().Add(time.Second)
	for j.calls.Load() == before && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if j.calls.Load() == before {
		t.Fatal("resume failed")
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); cancelSchedJob(id) }()
	}
	wg.Wait()
}

func TestSDKTimerHasOnlyOneOutstandingCallback(t *testing.T) {
	instance := &yyRuntimeInstance{Key: "qa:timer", Queue: make(chan map[string]any, 256), Done: make(chan struct{})}
	s := &yySDKContext{Instance: instance, Timers: map[int64]chan struct{}{}}
	id := s.schedule(time.Millisecond, false, "slow")
	defer cancelSchedJob(id)
	var event map[string]any
	select {
	case event = <-instance.Queue:
	case <-time.After(time.Second):
		t.Fatal("timer did not enqueue")
	}
	time.Sleep(30 * time.Millisecond)
	if len(instance.Queue) != 0 {
		t.Fatalf("unfinished timer accumulated %d callbacks", len(instance.Queue))
	}
	close(event["_timer_done"].(chan struct{}))
	select {
	case <-instance.Queue:
	case <-time.After(time.Second):
		t.Fatal("timer did not resume after completion")
	}
}

func TestPacketEventQueueIsBounded(t *testing.T) {
	bm := &BotManager{}
	id := "qa-packet-bounds"
	if e := yyLoadResident(bm, id, `function tick() return 1 end`, false); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(bm, id)
	yyLuaMu.Lock()
	r := yyLuaStates[yyLuaKey{bm, id}]
	yyLuaMu.Unlock()
	r.mu.Lock()
	for i := 0; i < cap(r.senseQueue)+1000; i++ {
		yyQueueResidentEvent(bm, id, "tick", nil)
	}
	if cap(r.senseQueue) != yyObservationSettings().EventQueue || r.senseDropped.Load() < 999 {
		t.Errorf("unbounded/drop missing %d %d", len(r.senseQueue), r.senseDropped.Load())
	}
	r.mu.Unlock()
}
