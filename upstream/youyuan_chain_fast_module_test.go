package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func yyFastTestNodes(count int) []yyChainNode {
	nodes := make([]yyChainNode, count)
	for i := range nodes {
		nodes[i].Command = fmt.Sprintf("/say %d", i)
	}
	return nodes
}

func TestChainFastGateSerializesAndCancelsWaiting(t *testing.T) {
	var gate yyChainFastGate
	release, err := gate.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	waiting := make(chan error, 1)
	go func() {
		unlock, err := gate.acquire(ctx)
		if unlock != nil {
			unlock()
		}
		waiting <- err
	}()
	cancel()
	if err := <-waiting; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting sender acquired occupied gate: %v", err)
	}
	release()
	var active, peak atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			unlock, err := gate.acquire(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			defer unlock()
			n := active.Add(1)
			if n > peak.Load() {
				peak.Store(n)
			}
			active.Add(-1)
		}()
	}
	group.Wait()
	if peak.Load() != 1 {
		t.Fatalf("concurrent modules=%d", peak.Load())
	}
}

func TestChainFastWriteFailurePoisonsConnectionWithoutReplay(t *testing.T) {
	var gate yyChainFastGate
	oldConn, newConn := new(minecraft.Conn), new(minecraft.Conn)
	sent := 0
	failed := errors.New("write result unknown")
	run := func(conn *minecraft.Conn, writeErr error) error {
		unlock, err := gate.acquire(context.Background())
		if err != nil {
			return err
		}
		defer unlock()
		if err := gate.bind(conn); err != nil {
			return err
		}
		_, unknown, err := yyChainFastChunks(context.Background(), yyFastTestNodes(1), yyChainConfig{}, "", "",
			func([]packet.Packet) (bool, error) { sent++; return true, writeErr })
		gate.writeOutcome(unknown, err)
		return err
	}
	if err := run(oldConn, failed); !errors.Is(err, failed) || sent != 1 {
		t.Fatalf("missing write failure: sent=%d err=%v", sent, err)
	}
	if err := run(oldConn, nil); !errors.Is(err, failed) || sent != 1 {
		t.Fatalf("unknown module replayed after unlock: sent=%d err=%v", sent, err)
	}
	if err := run(newConn, nil); err != nil || sent != 2 {
		t.Fatalf("fresh connection remained poisoned: sent=%d err=%v", sent, err)
	}
}

func TestChainFastNormalCancellationDoesNotDisableHealthyConnection(t *testing.T) {
	for _, ended := range []error{context.Canceled, context.DeadlineExceeded} {
		var gate yyChainFastGate
		conn := new(minecraft.Conn)
		if err := gate.bind(conn); err != nil {
			t.Fatal(err)
		}
		_, unknown, err := yyChainFastChunks(context.Background(), yyFastTestNodes(3), yyChainConfig{}, "", "", func([]packet.Packet) (bool, error) { return true, ended })
		if !unknown || !errors.Is(err, ended) {
			t.Fatal(unknown, err)
		}
		gate.writeOutcome(unknown, err)
		if err := gate.bind(conn); err != nil {
			t.Fatal("normal stop disabled later modules on same connection", err)
		}
		gate.writeOutcome(true, errors.New("real network write failure"))
		if gate.bind(conn) == nil {
			t.Fatal("real network ambiguity no longer blocks replay")
		}
	}
}

func TestChainFastLongModuleKeepsOrderWithoutServerProbe(t *testing.T) {
	nodes := yyFastTestNodes(2500)
	var counts []int
	seen := 0
	rows, unknown, err := yyChainFastChunks(context.Background(), nodes, yyChainConfig{}, "nether", `Bot"Name`,
		func(batch []packet.Packet) (bool, error) {
			counts = append(counts, len(batch))
			for _, p := range batch {
				pk, ok := p.(*packet.SettingsCommand)
				want := yyFastSettingsPacket(fmt.Sprintf("execute in nether run say %d", seen), `Bot"Name`, true)
				if !ok || !reflect.DeepEqual(pk, want) {
					t.Fatalf("command %d lost order/executor/dimension or added a server probe: %v", seen, p)
				}
				seen++
			}
			return true, nil
		})
	if err != nil || unknown || seen != len(nodes) || !reflect.DeepEqual(counts, []int{1024, 1024, 452}) {
		t.Fatalf("seen=%d batches=%v unknown=%v err=%v", seen, counts, unknown, err)
	}
	for i, row := range rows {
		if row["submitted"] != true || row["confirmed"] != false || row["fence_received"] != nil || row["success"] == true {
			t.Fatalf("business success invented at %d: %v", i, row)
		}
	}
}

func TestChainFastStatusRowsRemainIndependentAcrossInvocations(t *testing.T) {
	success, _, err := yyChainFastChunks(context.Background(), yyFastTestNodes(2500), yyChainConfig{}, "", "", func([]packet.Packet) (bool, error) { return true, nil })
	if err != nil {
		t.Fatal(err)
	}
	failed, unknown, err := yyChainFastChunks(context.Background(), yyFastTestNodes(2500), yyChainConfig{}, "", "", func([]packet.Packet) (bool, error) { return true, errors.New("unknown write") })
	if err == nil || !unknown {
		t.Fatal("failed call lost uncertainty", err)
	}
	for i := range success {
		if success[i]["submitted"] != true || success[i]["unknown"] == true || failed[i]["submitted"] != false || (failed[i]["unknown"] == true) != (i < 1024) {
			t.Fatal("status was mutated across commands or invocations", i, success[i], failed[i])
		}
	}
}

func TestChainFastByteBoundsAndOriginalOrigins(t *testing.T) {
	nodes := []yyChainNode{{Command: strings.Repeat("a", 300*1024)}, {Command: strings.Repeat("b", 300*1024)}}
	batches := 0
	_, unknown, err := yyChainFastChunks(context.Background(), nodes, yyChainConfig{}, "", "",
		func(batch []packet.Packet) (bool, error) {
			batches++
			if len(batch) != 1 || batch[0].(*packet.SettingsCommand).CommandLine != nodes[batches-1].Command {
				t.Fatalf("byte-bound segmentation changed text: %d", len(batch))
			}
			return true, nil
		})
	if err != nil || unknown || batches != 2 {
		t.Fatalf("batches=%d unknown=%v err=%v", batches, unknown, err)
	}
	for _, channel := range []string{"player", "console"} {
		pk, _, err := yyChainFastPacket(yyChainNode{Command: "/say hi", Channel: channel}, yyChainConfig{}, "", "Bot")
		if err != nil {
			t.Fatal(err)
		}
		command := pk.(*packet.CommandRequest)
		origin := uint32(protocol.CommandOriginAutomationPlayer)
		if channel == "player" {
			origin = protocol.CommandOriginPlayer
		}
		if command.CommandLine != "say hi" || command.CommandOrigin.Origin != origin || command.Version != 44 {
			t.Fatalf("%s origin changed: %v", channel, command)
		}
	}
}

func TestChainFastPositiveRateKeepsPerCommandSubmission(t *testing.T) {
	sent := 0
	rows, unknown, err := yyChainFastChunks(context.Background(), yyFastTestNodes(3), yyChainConfig{RateCPS: 200}, "", "",
		func(batch []packet.Packet) (bool, error) {
			if len(batch) != 1 || batch[0].(*packet.SettingsCommand).CommandLine != fmt.Sprintf("say %d", sent) {
				t.Fatalf("positive rate changed per-command order: send=%d packets=%v", sent, batch)
			}
			sent++
			return true, nil
		})
	if err != nil || unknown || sent != 3 {
		t.Fatalf("sent=%d unknown=%v err=%v", sent, unknown, err)
	}
	for _, row := range rows {
		if row["submitted"] != true || row["confirmed"] != false {
			t.Fatalf("unexpected submission status: %v", row)
		}
	}
}

func TestChainFastFailureRetainsOnlyActuallySubmittedRows(t *testing.T) {
	for _, attempted := range []bool{false, true} {
		sent := 0
		rows, unknown, err := yyChainFastChunks(context.Background(), yyFastTestNodes(2500), yyChainConfig{}, "", "",
			func([]packet.Packet) (bool, error) {
				sent++
				if sent == 1 {
					return true, nil
				}
				return attempted, errors.New("send stopped")
			})
		if err == nil || unknown != attempted || sent != 2 {
			t.Fatalf("sent=%d unknown=%v err=%v", sent, unknown, err)
		}
		for i, row := range rows {
			if row["submitted"] != (i < 1024) || row["confirmed"] != false || (i >= 2048 && row["unknown"] == true) {
				t.Fatalf("attempted=%v row %d: %v", attempted, i, row)
			}
			if attempted && i >= 1024 && i < 2048 && (row["attempted"] != true || row["unknown"] != true) {
				t.Fatalf("write uncertainty lost at %d: %v", i, row)
			}
		}
	}
}

func TestChainFastCancellationAfterSuccessfulWriteNeverReplaysOrPoisons(t *testing.T) {
	for _, count := range []int{1, 2500} {
		ctx, cancel := context.WithCancel(context.Background())
		sent := 0
		rows, unknown, err := yyChainFastChunks(ctx, yyFastTestNodes(count), yyChainConfig{}, "", "",
			func([]packet.Packet) (bool, error) { sent++; cancel(); return true, nil })
		if !errors.Is(err, context.Canceled) || unknown || sent != 1 || rows[0]["submitted"] != true || rows[0]["confirmed"] != false {
			t.Fatalf("count=%d sent=%d unknown=%v err=%v", count, sent, unknown, err)
		}
		if count > 1024 && rows[1024]["submitted"] != false {
			t.Fatal("caller cancellation sent the next chunk")
		}
	}
}

func TestChainFastInvalidInputNeverWrites(t *testing.T) {
	for _, node := range []yyChainNode{
		{Command: strings.Repeat("x", 512*1024+1)},
		{Command: "say hi", Conditional: true},
		{Command: "say hi", RequireResult: true},
		{Command: "say hi", Channel: "invalid"},
	} {
		rows, unknown, err := yyChainFastChunks(context.Background(), []yyChainNode{node}, yyChainConfig{}, "", "",
			func([]packet.Packet) (bool, error) { t.Fatal("invalid input reached transport"); return false, nil })
		if err == nil || unknown || rows[0]["submitted"] != false {
			t.Fatalf("invalid input accepted: rows=%v unknown=%v err=%v", rows, unknown, err)
		}
	}
}

func TestChainFastPreservesCallerManagedDelayMetadata(t *testing.T) {
	node := yyChainNode{Command: "/say delayed", DelayMS: 60000}
	sent := 0
	rows, unknown, err := yyChainFastChunks(context.Background(), []yyChainNode{node}, yyChainConfig{}, "", "",
		func(batch []packet.Packet) (bool, error) {
			sent++
			if batch[0].(*packet.SettingsCommand).CommandLine != "say delayed" {
				t.Fatal("delay metadata changed command text")
			}
			return true, nil
		})
	if err != nil || unknown || sent != 1 || rows[0]["submitted"] != true || node.DelayMS != 60000 {
		t.Fatalf("caller-managed delay rejected: sent=%d unknown=%v err=%v", sent, unknown, err)
	}
}

func TestFastSettingsPacketKeepsLegacyExecutorWrap(t *testing.T) {
	cases := []struct {
		command string
		wrapped bool
	}{
		{"execute as @a[tag=return_surface] at @s run tp @s ~ ~-1 ~", true},
		{"/execute at @e[type=snowball] as @p[r=3] run scoreboard players add @s 雪球菜单 1", true},
		{"  EXECUTE in nether run say hi", true},
		{"tp @s ~ ~-1 ~", true},
		{"/say hello", true},
		{"give @r snowball 1", true},
	}
	for _, c := range cases {
		pk := yyFastSettingsPacket("payload", "Bot", yyNeedsBotContext(c.command))
		wrapped := strings.HasPrefix(pk.CommandLine, `execute as @a[name="Bot"] at @s run `)
		if wrapped != c.wrapped {
			t.Fatalf("%q: wrapped=%v want %v (%s)", c.command, wrapped, c.wrapped, pk.CommandLine)
		}
	}
}

func yyChainFastChunks(ctx context.Context, nodes []yyChainNode, cfg yyChainConfig, dim, name string, send func([]packet.Packet) (bool, error)) ([]map[string]any, bool, error) {
	result, err := yyChainFastSubmit(ctx, nodes, cfg, dim, name, send)
	rows := make([]map[string]any, len(nodes))
	for i := range rows {
		rows[i] = map[string]any{"submitted": i < result.Submitted, "confirmed": false}
		if i >= result.Submitted && i < result.UnknownEnd {
			rows[i]["attempted"], rows[i]["unknown"] = true, true
		}
	}
	return rows, result.UnknownEnd > result.Submitted, err
}
