package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestChainMultipleModulesStayPreparedAcrossAlternatingSpans(t *testing.T) {
	nodes := yyFastTestNodes(170)
	spans := [][]yyChainNode{nodes[:80], nodes[81:]}
	cfg := yyChainConfig{Name: "menu.txt", preparedSource: &nodes[0]}
	var modules yyChainFastModules
	send := func([]packet.Packet) (bool, error) { return true, nil }
	first := make([]packet.Packet, len(spans))
	for i, span := range spans {
		p := modules.module(nil, span, cfg)
		if _, err := p.submit(context.Background(), span, cfg, "", "Bot", send); err != nil {
			t.Fatal(err)
		}
		first[i] = p.packets[0]
	}
	allocs := testing.AllocsPerRun(20, func() {
		for i, span := range spans {
			p := modules.module(nil, span, cfg)
			if _, err := p.submit(context.Background(), span, cfg, "", "Bot", send); err != nil {
				t.Fatal(err)
			}
			if p.packets[0] != first[i] {
				t.Fatal("alternating modules were rebuilt")
			}
		}
	})
	if allocs != 0 || len(modules.spans) != 2 {
		t.Fatal(allocs, len(modules.spans))
	}
	var previousSingleSlot yyChainFastPrepared
	previousAllocs := testing.AllocsPerRun(20, func() {
		for _, span := range spans {
			if _, err := previousSingleSlot.submit(context.Background(), span, cfg, "", "Bot", send); err != nil {
				t.Fatal(err)
			}
		}
	})
	if previousAllocs <= allocs {
		t.Fatal("comparison failed to expose alternating-span reconstruction", previousAllocs, allocs)
	}
	t.Logf("same two alternating spans: previous single slot %.0f allocations/round; file-owned modules %.0f allocations/round; excludes protocol encoding and server execution", previousAllocs, allocs)

}

func TestChainModuleReloadAndReconnectEvictOldBuffers(t *testing.T) {
	nodes, child := yyFastTestNodes(170), yyFastTestNodes(60)
	plan := yyChainPlan{Chains: []yyChain{{Nodes: nodes}}}
	var modules yyChainFastModules
	modules.bindRoot(plan)
	cfg := yyChainConfig{Name: "child.txt", preparedSource: &child[0]}
	one := modules.module(nil, child[:20], cfg)
	modules.module(nil, child[21:], cfg)
	newChild := yyFastTestNodes(60)
	cfg.preparedSource = &newChild[0]
	two := modules.module(nil, newChild[:20], cfg)
	if one == two || len(modules.spans) != 1 {
		t.Fatal("retained replaced child", len(modules.spans))
	}
	three := modules.module(&minecraft.Conn{}, newChild[:20], cfg)
	if two == three || len(modules.spans) != 1 {
		t.Fatal("reused previous connection")
	}
	modules.bindRoot(yyChainPlan{Chains: []yyChain{{Nodes: yyFastTestNodes(170)}}})
	if modules.spans != nil || modules.sources != nil {
		t.Fatal("root replacement retained child buffers")
	}
	data, err := json.Marshal(cfg)
	if err != nil || strings.Contains(string(data), "prepared") {
		t.Fatal("internal preparation identity escaped into configuration", err)
	}
}

func TestChainActualGraphReusesBothSidesOfConditionalBarrier(t *testing.T) {
	plan, err := yyParseChains(strings.Repeat("say first\n", 85) + "0,1,0\nsay conditional\n0,0,0\n" + strings.Repeat("say tail\n", 85))
	if err != nil {
		t.Fatal(err)
	}
	var modules yyChainFastModules
	var confirmed int
	for round := 0; round < 10; round++ {
		modules.bindRoot(plan)
		err = yyChainExecuteGraph(context.Background(), plan, yyChainConfig{Name: "menu.txt", Mode: "fast"}, nil, nil,
			func(_ context.Context, n yyChainNode, _ yyChainConfig) (bool, error) {
				if !n.RequireResult {
					t.Fatal("receipt barrier lost")
				}
				confirmed++
				return true, nil
			}, nil,
			func() bool { return false }, func(string) {}, func(bool, bool, error) {},
			yyChainFastSender{Send: func(ctx context.Context, ns []yyChainNode, c yyChainConfig) (yyChainFastResult, error) {
				p := modules.module(nil, ns, c)
				return p.submit(ctx, ns, c, "", "Bot", func([]packet.Packet) (bool, error) { return true, nil })
			}, RecordSubmitted: func(int, int) {}})
		if err != nil || len(modules.spans) != 2 {
			t.Fatal(err, len(modules.spans))
		}
	}
	if confirmed != 20 {
		t.Fatal("conditional command or its predecessor was skipped", confirmed)
	}
}
