package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestFastNativeSummaryKeepsConditionalBarrierWithoutResultTables(t *testing.T) {
	plan, err := yyParseChains("say a\nsay b\n0,1,0\nsay conditional\n0,0,0\nsay tail")
	if err != nil {
		t.Fatal(err)
	}
	var sent, confirmed []string
	skips, summaries := 0, 0
	yyChainExecuteBuffered(context.Background(), plan, yyChainConfig{Mode: "fast"}, 0,
		func(n yyChainNode) (bool, error) {
			confirmed = append(confirmed, n.Command)
			if !n.RequireResult {
				t.Fatal("conditional predecessor lost receipt barrier")
			}
			return false, nil
		},
		func([]yyChainNode) ([]map[string]any, error) {
			t.Fatal("native Fast used per-command result tables")
			return nil, nil
		},
		func(_ bool, skip bool, err error) {
			if err != nil {
				t.Fatal(err)
			}
			if skip {
				skips++
			}
		},
		yyChainFastSender{Send: func(_ context.Context, nodes []yyChainNode, _ yyChainConfig) (yyChainFastResult, error) {
			for _, n := range nodes {
				sent = append(sent, n.Command)
			}
			return yyChainFastResult{Submitted: len(nodes)}, nil
		}, RecordSubmitted: func(submitted, skipped int) {
			if submitted != 1 || skipped != 0 {
				t.Fatal(submitted, skipped)
			}
			summaries++
		}})
	if !reflect.DeepEqual(sent, []string{"say a", "say tail"}) || !reflect.DeepEqual(confirmed, []string{"say b"}) || skips != 1 || summaries != 2 {
		t.Fatal(sent, confirmed, skips)
	}
}

func TestFastNativeSummaryReachesChildGraphs(t *testing.T) {
	parent, _ := yyParseChains("@branch {\"command\":\"testfor @a\",\"then\":\"child.txt\"}\nsay parent")
	child, _ := yyParseChains("say child")
	var sent []string
	err := yyChainExecuteGraph(context.Background(), parent, yyChainConfig{Mode: "fast"}, nil,
		func(string, string) (yyChainPlan, yyChainConfig, error) {
			return child, yyChainConfig{Mode: "fast"}, nil
		},
		func(context.Context, yyChainNode, yyChainConfig) (bool, error) { return true, nil },
		func(context.Context, []yyChainNode, yyChainConfig) ([]map[string]any, error) {
			t.Fatal("child lost aggregate sender")
			return nil, nil
		},
		func() bool { return false }, func(string) {}, func(bool, bool, error) {},
		yyChainFastSender{Send: func(_ context.Context, nodes []yyChainNode, _ yyChainConfig) (yyChainFastResult, error) {
			for _, n := range nodes {
				sent = append(sent, n.Command)
			}
			return yyChainFastResult{Submitted: len(nodes)}, nil
		}})
	if err != nil || !reflect.DeepEqual(sent, []string{"say child", "say parent"}) {
		t.Fatal(err, sent)
	}
}

func TestFastNativeSummaryDoesNotAllocatePerCommandResultMaps(t *testing.T) {
	nodes := yyFastTestNodes(1000)
	allocs := testing.AllocsPerRun(10, func() {
		_, err := yyChainFastSubmit(context.Background(), nodes, yyChainConfig{}, "", "Bot", func([]packet.Packet) (bool, error) { return true, nil })
		if err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 2050 {
		t.Fatalf("per-command bookkeeping returned: %.0f allocations", allocs)
	}
}
