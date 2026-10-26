package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestFastChainChildDeadlineOnlyResumesBeforeUnknownWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wrapped := fmt.Errorf("module: %w", ctx.Err())
	if !yyChainFastInvocationEnded(ctx, wrapped, false) {
		t.Fatal("child invocation timeout before send must return to its parent")
	}
	if !yyChainFastInvocationEnded(ctx, wrapped, false) {
		t.Fatal("a completed local write must not poison a child ending before its next chunk")
	}
	if yyChainFastInvocationEnded(ctx, wrapped, true) {
		t.Fatal("child timeout after an unknown write must stop, not resume")
	}
	if yyChainFastInvocationEnded(context.Background(), fmt.Errorf("network error"), false) {
		t.Fatal("transport errors must stop")
	}
}

func TestFastChainFirstModuleFailureStopsShortSegmentsWithoutAnotherRound(t *testing.T) {
	botMgrMu.Lock()
	previous := botMgr
	botMgr = &BotManager{connected: true}
	botMgrMu.Unlock()
	defer func() { botMgrMu.Lock(); botMgr = previous; botMgrMu.Unlock() }()
	for _, text := range []string{"2,0,0 20\nsay one", "2,0,0 20\nsay one\nsay two\n.\n2,0,0 20\nsay later"} {
		plan, err := yyParseChains(text)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		in := &yyChainInstance{Config: yyChainConfig{Name: "offline.txt", Mode: "fast", Scheduling: "file", IntervalMS: 50, Enabled: true},
			Plan: plan, Ephemeral: true, State: "运行中", StartedAt: time.Now(), done: make(chan struct{}), cancel: cancel}
		in.run(ctx)
		cancel()
		if in.State != "错误暂停" || in.Cycles != 0 || in.Submitted != 0 || in.Config.Enabled || in.LastError == "" {
			t.Fatalf("first module failure continued or replayed: state=%s cycles=%d submitted=%d enabled=%v err=%s", in.State, in.Cycles, in.Submitted, in.Config.Enabled, in.LastError)
		}
	}
}
