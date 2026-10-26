package main

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

func TestSegmentHzIsPerIndependentChainAndPulseIsOnce(t *testing.T) {
	plan, err := yyParseChains("1,0,0\nsay init\n.\n2,0,0 20\nsay a\n0,0,0\nsay b\n.\n2,0,0 5\nsay c\n!")
	if err != nil || !plan.Segmented || len(plan.Chains) != 3 || plan.Chains[0].Hz != 0 || plan.Chains[1].Hz != 20 || plan.Chains[2].Hz != 5 || plan.Hz != 0 {
		t.Fatal(plan, err)
	}
	if _, err = yyParseChains("2,0,0 20\nsay a\n2,0,0 5\nsay b"); err == nil {
		t.Fatal("Frequency changed inside one sequential segment")
	}
	root := yyChainRoot
	yyChainRoot = t.TempDir()
	defer func() { yyChainRoot = root }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := &yyChainInstance{Config: yyChainConfig{Name: "seg.txt", IntervalMS: 1000}, Plan: plan, StartedAt: time.Now(), done: make(chan struct{}), cancel: cancel}
	var mu sync.Mutex
	counts := map[string]int{}
	runner := func(w *yyChainInstance, c context.Context) {
		defer func() { w.Lock(); w.cancel = nil; w.State = "已完成"; w.Unlock(); close(w.done) }()
		w.Lock()
		ch := w.Plan.Chains[0]
		cfg := w.Config
		w.Unlock()
		name := ch.Nodes[0].Command
		period := time.Second
		if ch.Hz > 0 {
			period = time.Duration(float64(time.Second) / ch.Hz)
		}
		for round := 0; cfg.RunCount == 0 || round < cfg.RunCount; round++ {
			if c.Err() != nil {
				return
			}
			mu.Lock()
			counts[name]++
			mu.Unlock()
			w.Lock()
			w.Cycles++
			w.Submitted += uint64(len(ch.Nodes))
			w.State = "运行中"
			w.Unlock()
			if e := yyChainSleep(c, period); e != nil {
				return
			}
		}
	}
	go in.runSegmentsWith(ctx, runner)
	time.Sleep(625 * time.Millisecond)
	cancel()
	select {
	case <-in.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Segment tasks did not stop")
	}
	mu.Lock()
	defer mu.Unlock()
	if counts["say init"] != 1 || counts["say a"] < 10 || counts["say a"] > 15 || counts["say c"] < 3 || counts["say c"] > 5 {
		t.Fatal(counts)
	}
	files, _ := os.ReadDir(yyChainRoot)
	if len(files) != 1 || files[0].Name() != "seg.txt.json" {
		t.Fatal("Per-segment metadata leaked", files)
	}
}

func TestSegmentResourceReloadRearmsChangedPulseAndKeepsSibling(t *testing.T) {
	root := yyChainRoot
	yyChainRoot = t.TempDir()
	defer func() { yyChainRoot = root }()
	plan, e := yyParseChains("1,0,0\nsay first\n.\n2,0,0 20\nsay loop\n!")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := &yyChainInstance{Config: yyChainConfig{Name: "reload.txt", Bootstrap: true}, Plan: plan, StartedAt: time.Now(), done: make(chan struct{}), cancel: cancel}
	var mu sync.Mutex
	starts := map[string]int{}
	order := []string{}
	runner := func(w *yyChainInstance, c context.Context) {
		defer func() { w.Lock(); w.cancel = nil; w.State = "已完成"; w.Unlock(); close(w.done) }()
		w.Lock()
		ch := w.Plan.Chains[0]
		w.Unlock()
		name := ch.Nodes[0].Command
		mu.Lock()
		starts[name]++
		order = append(order, name)
		mu.Unlock()
		if ch.Nodes[0].Type == 1 {
			w.Lock()
			w.Cycles++
			w.Unlock()
			return
		}
		<-c.Done()
	}
	go in.runSegmentsWith(ctx, runner)
	time.Sleep(80 * time.Millisecond)
	changed, e := yyParseChains("1,0,0\nsay edited\n.\n2,0,0 20\nsay loop\n!")
	if e != nil {
		t.Fatal(e)
	}
	in.Lock()
	in.Plan = changed
	in.Revision++
	in.Unlock()
	time.Sleep(80 * time.Millisecond)
	cancel()
	<-in.done
	mu.Lock()
	defer mu.Unlock()
	if starts["say first"] != 1 || starts["say edited"] != 1 || starts["say loop"] != 1 || order[0] != "say first" {
		t.Fatal(starts, order)
	}
	if in.Cycles != 2 {
		t.Fatal("Retired pulse statistics lost", in.Cycles)
	}
}
