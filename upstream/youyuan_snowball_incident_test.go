package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestSnowballIncidentShapeAndSingleRoundOrder(t *testing.T) {
	segments := []struct {
		header string
		count  int
		hz     float64
	}{
		{"1,0,0", 3, 0},
		{"2,0,0 20", 164, 20},
		{"2,0,0 1", 2, 1},
		{"2,0,0 20", 2, 20},
		{"2,0,0 0.016666667", 1, 0.016666667},
	}
	var source strings.Builder
	want := make([][]string, len(segments))
	for i, segment := range segments {
		fmt.Fprintln(&source, segment.header)
		for j := 0; j < segment.count; j++ {
			command := fmt.Sprintf("say incident_fixture_%d_%03d", i, j)
			want[i] = append(want[i], command)
			fmt.Fprintln(&source, "/"+command)
			if j == 0 {
				fmt.Fprintln(&source, "0,0,0")
			}
		}
		fmt.Fprintln(&source, ".")
	}
	fmt.Fprintln(&source, "!")
	plan, err := yyParseChains(source.String())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Commands != 172 || len(plan.Chains) != 5 || plan.Periods != 1 || !plan.Segmented || plan.Hz != 0 {
		t.Fatalf("one file round must contain 172 nodes in five mixed-frequency segments: %+v", plan)
	}
	total := 0
	for i, segment := range segments {
		chain := plan.Chains[i]
		if len(chain.Nodes) != segment.count || chain.Hz != segment.hz || chain.Repeat != 1 {
			t.Fatalf("segment %d lost its node count, frequency or repeat: %+v", i, chain)
		}
		var got []string
		batches, records := 0, 0
		// Capture one worker round; no BotManager or game connection is created.
		yyChainExecuteBuffered(context.Background(), yyChainPlan{Chains: []yyChain{chain}}, yyChainConfig{Mode: "fast"}, 0,
			func(yyChainNode) (bool, error) {
				t.Fatal("unconditional nodes unexpectedly used the result-waiting path")
				return false, nil
			},
			func(nodes []yyChainNode) ([]map[string]any, error) {
				batches++
				rows := make([]map[string]any, len(nodes))
				for j, node := range nodes {
					got = append(got, node.Command)
					rows[j] = map[string]any{"submitted": true, "confirmed": false}
				}
				return rows, nil
			}, func(_ bool, skip bool, err error) {
				if skip || err != nil {
					t.Fatalf("unexpected skipped/error node: skip=%v err=%v", skip, err)
				}
				records++
			})
		if !reflect.DeepEqual(got, want[i]) || batches != 1 || records != segment.count {
			t.Fatalf("segment %d changed order or repeated a single round: got=%v batches=%d records=%d", i, got, batches, records)
		}
		total += len(got)
	}
	if total != 172 {
		t.Fatalf("single-round node count became %d; frequency must not expand the parsed round", total)
	}
}

func TestSnowballIncidentFastConditionalBarrier(t *testing.T) {
	for _, predecessorOK := range []bool{false, true} {
		t.Run(fmt.Sprintf("predecessor_%v", predecessorOK), func(t *testing.T) {
			plan, err := yyParseChains("say first\nsay predecessor\n0,1,0\nsay conditional\n0,0,0\nsay last")
			if err != nil {
				t.Fatal(err)
			}
			var events []string
			skipped := 0
			yyChainExecuteBuffered(context.Background(), plan, yyChainConfig{Mode: "fast"}, 0,
				func(node yyChainNode) (bool, error) {
					if !node.RequireResult {
						t.Fatalf("dependency node did not require a real result: %+v", node)
					}
					events = append(events, "result:"+node.Command)
					return predecessorOK, nil
				}, func(nodes []yyChainNode) ([]map[string]any, error) {
					rows := make([]map[string]any, len(nodes))
					for i, node := range nodes {
						if node.Conditional || node.Command == "say predecessor" {
							t.Fatalf("dependency node entered the no-result batch: %+v", node)
						}
						events = append(events, "fast:"+node.Command)
						rows[i] = map[string]any{"submitted": true, "confirmed": false}
					}
					return rows, nil
				}, func(_ bool, skip bool, err error) {
					if err != nil {
						t.Fatal(err)
					}
					if skip {
						skipped++
					}
				})
			want := []string{"fast:say first", "result:say predecessor"}
			wantSkipped := 1
			if predecessorOK {
				want = append(want, "result:say conditional")
				wantSkipped = 0
			}
			want = append(want, "fast:say last")
			if !reflect.DeepEqual(events, want) || skipped != wantSkipped {
				t.Fatalf("conditional dependency changed: events=%v skipped=%d", events, skipped)
			}
		})
	}
}
