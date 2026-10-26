package main

import (
	"context"
	"reflect"
	"testing"
)

func TestFastChainOnlyConfirmsTrueDependencies(t *testing.T) {
	p, err := yyParseChains("say a\nsay b\n0,1,0\nsay c\n0,0,0\nsay d")
	if err != nil {
		t.Fatal(err)
	}
	var fast, confirmed []string
	skipped := 0
	yyChainExecuteBuffered(context.Background(), p, yyChainConfig{Mode: "fast"}, 0,
		func(n yyChainNode) (bool, error) { confirmed = append(confirmed, n.Command); return false, nil },
		func(nodes []yyChainNode) ([]map[string]any, error) {
			rows := []map[string]any{}
			for _, n := range nodes {
				fast = append(fast, n.Command)
				rows = append(rows, map[string]any{"submitted": true, "confirmed": false})
			}
			return rows, nil
		}, func(_, skip bool, e error) {
			if e != nil {
				t.Fatal(e)
			}
			if skip {
				skipped++
			}
		})
	if !reflect.DeepEqual(fast, []string{"say a", "say d"}) || !reflect.DeepEqual(confirmed, []string{"say b"}) || skipped != 1 {
		t.Fatalf("dependency barrier lost: fast=%v confirmed=%v skipped=%d", fast, confirmed, skipped)
	}
}

func TestBufferedChainBarriersAndFailure(t *testing.T) {
	plan, e := yyParseChains("1,0,0\nsay first\nsay second\n0,1,0\nsay conditional\n0,0,0\nsay independent\n.\n0,1,0\nsay isolated\n")
	if e != nil {
		t.Fatal(e)
	}
	var batches [][]string
	var singles []string
	skipped := 0
	yyChainExecuteBuffered(context.Background(), plan, yyChainConfig{Mode: "pipeline", Window: 8}, 0,
		func(n yyChainNode) (bool, error) { singles = append(singles, n.Command); return true, nil },
		func(nodes []yyChainNode) ([]map[string]any, error) {
			var names []string
			var rows []map[string]any
			for _, n := range nodes {
				names = append(names, n.Command)
				rows = append(rows, map[string]any{"confirmed": true, "success": n.Command != "say second"})
			}
			batches = append(batches, names)
			return rows, nil
		},
		func(_, skip bool, _ error) {
			if skip {
				skipped++
			}
		})
	if !reflect.DeepEqual(batches, [][]string{{"say first", "say second"}, {"say independent"}}) || len(singles) != 0 || skipped != 2 {
		t.Fatalf("barrier lost: %v %v skipped=%d", batches, singles, skipped)
	}
}
func TestBufferedChannelAndDelayBarriers(t *testing.T) {
	plan := yyChainPlan{Chains: []yyChain{{Repeat: 1, Nodes: []yyChainNode{{Command: "a", Channel: "ai"}, {Command: "b", Channel: "player"}, {Command: "c", Channel: "player", DelayMS: 1}, {Command: "d", Channel: "player", Conditional: true}}}}}
	var batches [][]string
	var singles []string
	yyChainExecuteBuffered(context.Background(), plan, yyChainConfig{Mode: "pipeline", Window: 16}, 0,
		func(n yyChainNode) (bool, error) { singles = append(singles, n.Command); return true, nil },
		func(nodes []yyChainNode) ([]map[string]any, error) {
			var names []string
			var rows []map[string]any
			for _, n := range nodes {
				names = append(names, n.Command)
				rows = append(rows, map[string]any{"confirmed": true, "success": true})
			}
			batches = append(batches, names)
			return rows, nil
		}, func(bool, bool, error) {})
	if !reflect.DeepEqual(batches, [][]string{{"a"}, {"b"}}) || !reflect.DeepEqual(singles, []string{"c", "d"}) {
		t.Fatalf("unexpected %v %v", batches, singles)
	}
}
func TestBufferedCancelledBeforeSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p, _ := yyParseChains("say a\nsay b")
	yyChainExecuteBuffered(ctx, p, yyChainConfig{Mode: "pipeline"}, 0, func(yyChainNode) (bool, error) { t.Fatal("sent after cancel"); return false, nil }, func([]yyChainNode) ([]map[string]any, error) { t.Fatal("batch after cancel"); return nil, nil }, func(bool, bool, error) { t.Fatal("recorded after cancel") })
	if yyChainValidate(yyChainConfig{Mode: "native_blocks"}) == nil {
		t.Fatal("physical deployment accepted")
	}
}
