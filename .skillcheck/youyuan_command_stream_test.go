package main

import (
	"context"
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"strings"
	"testing"
	"time"
)

func TestSlidingRefillsBeforeFirstReply(t *testing.T) {
	callbacks := make(map[int]func(yyCommandReply))
	commands := make([]string, 40)
	for i := range commands {
		commands[i] = fmt.Sprint(i)
	}
	firstPending := true
	reply := yyCommandReply{p: &packet.CommandOutput{SuccessCount: 1}}
	rows, peak := yyExecuteSliding(context.Background(), commands, 32, time.Second, nil, nil, func(i int, command string, done func(yyCommandReply)) error {
		callbacks[i] = done
		if i != 0 {
			done(reply)
		}
		if i == 16 {
			// The seventeenth request must be sent while #0 is still unanswered.
			if !firstPending {
				t.Fatal("head-of-line barrier remains")
			}
			firstPending = false
			callbacks[0](reply)
		}
		return nil
	})
	if firstPending || peak > 32 {
		t.Fatalf("pending=%v peak=%d", firstPending, peak)
	}
	for i, row := range rows {
		if row["confirmed"] != true || row["success"] != true {
			t.Fatalf("row %d: %v", i, row)
		}
	}
}

func TestSlidingTimeoutDoesNotResendOrGrowQueue(t *testing.T) {
	commands := make([]string, 50)
	sent := 0
	rows, peak := yyExecuteSliding(context.Background(), commands, 32, 5*time.Millisecond, nil, nil, func(int, string, func(yyCommandReply)) error { sent++; return nil })
	if sent != 16 || peak != 16 {
		t.Fatalf("sent=%d peak=%d", sent, peak)
	}
	if rows[0]["unknown"] != true || rows[16]["submitted"] != false {
		t.Fatal(rows)
	}
}

func TestStreamOptions(t *testing.T) {
	o, e := yyMergeCommandOptions(yyDefaultCommandOptions(), map[string]any{"mode": "stream"})
	if e != nil || o.Window != 128 || o.RateCPS != 0 {
		t.Fatalf("%+v %v", o, e)
	}
	if e = yyChainValidate(yyChainConfig{Mode: "stream", Window: 128}); e != nil {
		t.Fatal(e)
	}
}

func TestSlidingFailedCommandDoesNotAbortUnconditionalTail(t *testing.T) {
	commands := make([]string, 50)
	sent := 0
	rows, _ := yyExecuteSliding(context.Background(), commands, 32, time.Second, nil, nil,
		func(i int, _ string, done func(yyCommandReply)) error {
			sent++
			count := uint32(1)
			if i == 0 {
				count = 0
			} // A selector matching no entity is a normal reply.
			done(yyCommandReply{p: &packet.CommandOutput{SuccessCount: count}})
			return nil
		})
	if sent != len(commands) || rows[0]["confirmed"] != true || rows[0]["success"] != false || rows[49]["success"] != true {
		t.Fatalf("normal command failure aborted unconditional tail: sent=%d first=%v last=%v", sent, rows[0], rows[49])
	}
}

func TestShortStreamFlushesOnceBeforeAwaiting(t *testing.T) {
	var callbacks []func(yyCommandReply)
	flushes := 0
	rows, peak := yyExecuteSliding(context.Background(), make([]string, 25), 256, time.Second, nil,
		func() error {
			flushes++
			if len(callbacks) != 25 {
				t.Fatalf("short menu split into %d commands", len(callbacks))
			}
			for _, done := range callbacks {
				done(yyCommandReply{p: &packet.CommandOutput{SuccessCount: 1}})
			}
			return nil
		}, func(_ int, _ string, done func(yyCommandReply)) error {
			callbacks = append(callbacks, done)
			return nil
		})
	if flushes != 1 || peak != 25 || rows[24]["confirmed"] != true {
		t.Fatalf("flushes=%d peak=%d last=%v", flushes, peak, rows[24])
	}
}

func TestStreamLongTXTIsSegmentedByCountAndBytes(t *testing.T) {
	for _, test := range []struct{ count, width, wantBatches int }{{60000, 1, 15}, {25, 24000, 2}} {
		nodes := make([]yyChainNode, test.count)
		for i := range nodes {
			nodes[i].Command = strings.Repeat("x", test.width)
		}
		batches, recorded := 0, 0
		yyChainExecuteBuffered(context.Background(), yyChainPlan{Chains: []yyChain{{Nodes: nodes}}}, yyChainConfig{Mode: "stream", Window: 256}, 0,
			func(yyChainNode) (bool, error) { t.Fatal("unexpected single dispatch"); return false, nil },
			func(chunk []yyChainNode) ([]map[string]any, error) {
				batches++
				size := 0
				rows := make([]map[string]any, len(chunk))
				for i, n := range chunk {
					size += len(n.Command)
					rows[i] = map[string]any{"submitted": true, "confirmed": true, "success": true}
				}
				if len(chunk) > 4096 || size > 512*1024 {
					t.Fatalf("oversized segment: count=%d bytes=%d", len(chunk), size)
				}
				return rows, nil
			}, func(success, skip bool, e error) {
				if !success || skip || e != nil {
					t.Fatalf("unexpected outcome %v %v %v", success, skip, e)
				}
				recorded++
			})
		if batches != test.wantBatches || recorded != test.count {
			t.Fatalf("count=%d batches=%d recorded=%d", test.count, batches, recorded)
		}
	}
}

func TestStreamOnlyUsesRealDependencyBarriers(t *testing.T) {
	nodes := make([]yyChainNode, 3000)
	for i := range nodes {
		nodes[i] = yyChainNode{Command: fmt.Sprint(i)}
	}
	nodes[1500].Conditional = true
	plan := yyChainPlan{Chains: []yyChain{{Repeat: 1, Nodes: nodes}}}
	var sizes []int
	singles := 0
	yyChainExecuteBuffered(context.Background(), plan, yyChainConfig{Mode: "stream", Window: 128}, 0,
		func(yyChainNode) (bool, error) { singles++; return true, nil },
		func(nodes []yyChainNode) ([]map[string]any, error) {
			sizes = append(sizes, len(nodes))
			rows := make([]map[string]any, len(nodes))
			for i := range rows {
				rows[i] = map[string]any{"submitted": true, "confirmed": true, "success": true}
			}
			return rows, nil
		},
		func(bool, bool, error) {})
	if len(sizes) != 2 || sizes[0] != 1500 || sizes[1] != 1499 || singles != 1 {
		t.Fatalf("unnecessary barrier: %v singles=%d", sizes, singles)
	}
}
