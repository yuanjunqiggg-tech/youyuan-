package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestChainPreparedReusesOnlyUnchangedImmutableModule(t *testing.T) {
	var p yyChainFastPrepared
	nodes := yyFastTestNodes(170)
	cfg := yyChainConfig{}
	if err := p.prepare(nodes, cfg, "nether", `Bot"Name`); err != nil {
		t.Fatal(err)
	}
	first := p.packets[0]
	if err := p.prepare(nodes, cfg, "nether", `Bot"Name`); err != nil || p.packets[0] != first {
		t.Fatal("unchanged module rebuilt", err)
	}
	// A file reload supplies a new snapshot, including when its length is unchanged.
	reloaded := append([]yyChainNode(nil), nodes...)
	reloaded[0].Command = "say changed"
	if err := p.prepare(reloaded, cfg, "nether", `Bot"Name`); err != nil || p.packets[0] == first {
		t.Fatal(err)
	}
	for _, tc := range []struct{ channel, dim, name string }{
		{"", "the_end", `Bot"Name`}, {"", "the_end", "Other"}, {"console", "the_end", "Other"},
	} {
		before := p.packets[0]
		cfg.Channel = tc.channel
		if err := p.prepare(reloaded, cfg, tc.dim, tc.name); err != nil {
			t.Fatal(err)
		}
		want, _, err := yyChainFastPacket(reloaded[0], cfg, tc.dim, tc.name)
		if err != nil || p.packets[0] == before || !reflect.DeepEqual(p.packets[0], want) && tc.channel == "" {
			t.Fatalf("stale context: %v %v", tc, err)
		}
	}
}

func TestChainPreparedWireBytesStayIdenticalAcrossRounds(t *testing.T) {
	var p yyChainFastPrepared
	nodes := yyFastTestNodes(170)
	encode := func(pk packet.Packet) []byte {
		var b bytes.Buffer
		pk.Marshal(protocol.NewWriter(&b, 0))
		return b.Bytes()
	}
	for round := 0; round < 3; round++ {
		seen := 0
		result, err := p.submit(context.Background(), nodes, yyChainConfig{}, "nether", `Bot"Name`, func(ps []packet.Packet) (bool, error) {
			for _, got := range ps {
				want, _, err := yyChainFastPacket(nodes[seen], yyChainConfig{}, "nether", `Bot"Name`)
				if err != nil || !bytes.Equal(encode(got), encode(want)) {
					t.Fatal("wire fields changed", seen, err)
				}
				seen++
			}
			return true, nil
		})
		if err != nil || result.Submitted != len(nodes) || seen != len(nodes) {
			t.Fatal(result, seen, err)
		}
	}
}

func TestChainPreparedDoesNotReuseRequestIDs(t *testing.T) {
	for _, channel := range []string{"console", "player"} {
		var p yyChainFastPrepared
		nodes := []yyChainNode{{Command: "say hi", Channel: channel}}
		if err := p.prepare(nodes, yyChainConfig{}, "", "Bot"); err != nil {
			t.Fatal(err)
		}
		first := p.packets[0].(*packet.CommandRequest).CommandOrigin.UUID
		if err := p.prepare(nodes, yyChainConfig{}, "", "Bot"); err != nil {
			t.Fatal(err)
		}
		if first == p.packets[0].(*packet.CommandRequest).CommandOrigin.UUID {
			t.Fatal("reused request UUID", channel)
		}
	}
}

func TestChainPreparedPreservesRateCancellationAndUnknownPrefix(t *testing.T) {
	var p yyChainFastPrepared
	nodes := yyFastTestNodes(2500)
	send := func([]packet.Packet) (bool, error) { return true, nil }
	if _, err := p.submit(context.Background(), nodes, yyChainConfig{}, "", "Bot", send); err != nil {
		t.Fatal(err)
	}
	count := 0
	_, err := p.submit(context.Background(), nodes, yyChainConfig{RateCPS: 200}, "", "Bot", func(ps []packet.Packet) (bool, error) {
		if len(ps) != 1 {
			t.Fatal("rate change ignored", len(ps))
		}
		count++
		return true, nil
	})
	if err != nil || count != len(nodes) {
		t.Fatal(count, err)
	}
	count = 0
	failed := errors.New("write result unknown")
	result, err := p.submit(context.Background(), nodes, yyChainConfig{}, "", "Bot", func([]packet.Packet) (bool, error) {
		count++
		if count == 2 {
			return true, failed
		}
		return true, nil
	})
	if !errors.Is(err, failed) || count != 2 || result.Submitted != 1024 || result.UnknownEnd != 2048 {
		t.Fatal(result, count, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = p.submit(ctx, nodes, yyChainConfig{}, "", "Bot", func([]packet.Packet) (bool, error) { t.Fatal("sent after stop"); return false, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	bad := append([]yyChainNode(nil), nodes...)
	bad[len(bad)-1].Conditional = true
	if _, err = p.submit(context.Background(), bad, yyChainConfig{}, "", "Bot", func([]packet.Packet) (bool, error) { t.Fatal("sent a prefix of invalid module"); return false, nil }); err == nil {
		t.Fatal("invalid late node accepted")
	}
}

func TestChainPreparedWarmDispatchDoesNotAllocatePerCommand(t *testing.T) {
	var p yyChainFastPrepared
	nodes := yyFastTestNodes(10000)
	send := func([]packet.Packet) (bool, error) { return true, nil }
	if _, err := p.submit(context.Background(), nodes, yyChainConfig{}, "", "Bot", send); err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(20, func() {
		if _, err := p.submit(context.Background(), nodes, yyChainConfig{}, "", "Bot", send); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("warm module rebuilt: %.0f allocations", allocs)
	}
}

func BenchmarkChainPreparedModule(b *testing.B) {
	for _, count := range []int{170, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			var p yyChainFastPrepared
			nodes := yyFastTestNodes(count)
			send := func(ps []packet.Packet) (bool, error) { yyChainAllocPacket = ps[len(ps)-1]; return true, nil }
			if _, err := p.submit(context.Background(), nodes, yyChainConfig{}, "", "布汲岛", send); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := p.submit(context.Background(), nodes, yyChainConfig{}, "", "布汲岛", send); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
