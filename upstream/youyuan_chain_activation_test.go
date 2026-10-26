package main

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestFastChainDoesNotReplayPulseOrUnpoweredCommands(t *testing.T) {
	plan, err := yyParseChains("2,0,0 20\nsay loop\n1,0,0\nsay pulse\n0,0,0\nsay pulse_tail\n2,0,1\nsay powered\n0,0,0\nsay powered_tail")
	if err != nil {
		t.Fatal(err)
	}
	run := func(a yyChainActivation) []string {
		var sent []string
		ctx := context.WithValue(context.Background(), yyChainActivationKey{}, a)
		yyChainExecuteBuffered(ctx, plan, yyChainConfig{Mode: "fast"}, 0,
			func(n yyChainNode) (bool, error) { sent = append(sent, n.Command); return true, nil },
			func(ns []yyChainNode) ([]map[string]any, error) {
				rows := make([]map[string]any, len(ns))
				for i, n := range ns {
					sent = append(sent, n.Command)
					rows[i] = map[string]any{"submitted": true}
				}
				return rows, nil
			}, func(bool, bool, error) {})
		return sent
	}
	for _, tc := range []struct {
		a    yyChainActivation
		want []string
	}{
		{yyChainActivation{First: true}, []string{"say loop", "say pulse", "say pulse_tail"}},
		{yyChainActivation{}, []string{"say loop"}},
		{yyChainActivation{Redstone: true}, []string{"say loop", "say powered", "say powered_tail"}},
		{yyChainActivation{Trigger: true}, []string{"say loop", "say pulse", "say pulse_tail"}},
	} {
		if got := run(tc.a); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%+v got=%v want=%v", tc.a, got, tc.want)
		}
	}
	// 100 rounds must not turn one-shot commands into a 20Hz loop.
	counts := map[string]int{}
	for i := 0; i < 100; i++ {
		for _, s := range run(yyChainActivation{First: i == 0}) {
			counts[s]++
		}
	}
	if counts["say loop"] != 100 || counts["say pulse"] != 1 || counts["say pulse_tail"] != 1 || counts["say powered"] != 0 {
		t.Fatal(counts)
	}
}

func TestRedstonePulseWaitDoesNotBlockIndependentBootstrap(t *testing.T) {
	plan, e := yyParseChains("1,0,1\nsay powered\n.\n2,0,0 20\nsay loop")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := &yyChainInstance{Plan: plan, Config: yyChainConfig{Bootstrap: true}, Ephemeral: true, done: make(chan struct{}), cancel: cancel}
	loop := make(chan struct{}, 1)
	go in.runSegmentsWith(ctx, func(w *yyChainInstance, c context.Context) {
		defer func() { w.Lock(); w.cancel = nil; w.Unlock(); close(w.done) }()
		w.Lock()
		command, count := w.Plan.Chains[0].Nodes[0].Command, w.Config.RunCount
		w.Unlock()
		if command == "say powered" && count != 0 {
			t.Error("unpowered pulse must remain available for its next rising edge")
		}
		if command == "say loop" {
			loop <- struct{}{}
		}
		<-c.Done()
	})
	select {
	case <-loop:
	case <-time.After(time.Second):
		t.Error("redstone wait blocked independent loop")
	}
	cancel()
	select {
	case <-in.done:
	case <-time.After(time.Second):
		t.Fatal("workers did not join")
	}
}
