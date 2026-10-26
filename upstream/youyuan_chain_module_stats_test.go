package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFastModuleRecordsOneSummaryWithoutPerCommandCallbacks(t *testing.T) {
	plan, err := yyParseChains(strings.Repeat("say module\n", 10000))
	if err != nil {
		t.Fatal(err)
	}
	for _, submitted := range []int{0, 17, 10000} {
		calls, sent, skipped := 0, 0, 0
		yyChainExecuteBuffered(context.Background(), plan, yyChainConfig{Mode: "fast"}, 0,
			func(yyChainNode) (bool, error) { t.Fatal("unexpected receipt"); return false, nil },
			func([]yyChainNode) ([]map[string]any, error) { t.Fatal("unexpected result table"); return nil, nil },
			func(bool, bool, error) { t.Fatal("per-command accounting returned") },
			yyChainFastSender{Send: func(context.Context, []yyChainNode, yyChainConfig) (yyChainFastResult, error) {
				return yyChainFastResult{Submitted: submitted}, nil
			}, RecordSubmitted: func(n, k int) { calls++; sent += n; skipped += k }})
		if calls != 1 || sent != submitted || skipped != 10000-submitted {
			t.Fatal(calls, sent, skipped)
		}
	}
}

func TestFastModuleKeepsErrorsAndCancellationOutOfSuccessSummary(t *testing.T) {
	plan, _ := yyParseChains("say one\nsay two")
	for _, canceled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		failed := 0
		yyChainExecuteBuffered(ctx, plan, yyChainConfig{Mode: "fast"}, 0,
			func(yyChainNode) (bool, error) { return false, nil }, nil,
			func(_ bool, _ bool, e error) {
				if e == nil {
					t.Fatal("lost unknown failure")
				}
				failed++
			},
			yyChainFastSender{Send: func(context.Context, []yyChainNode, yyChainConfig) (yyChainFastResult, error) {
				if canceled {
					cancel()
				}
				return yyChainFastResult{Submitted: 1, UnknownEnd: 2}, errors.New("unknown write")
			}, RecordSubmitted: func(int, int) { t.Fatal("failure counted as successful module") }})
		cancel()
		if !canceled && failed != 2 || canceled && failed != 0 {
			t.Fatal(canceled, failed)
		}
	}
}
