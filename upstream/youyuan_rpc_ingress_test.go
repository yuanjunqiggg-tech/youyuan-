package main

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func rpcIngressPacket(namespace, system, event any) *packet.PyRpc {
	return &packet.PyRpc{
		OperationType: packet.PyRpcOperationTypeRecv,
		Value:         []any{[]byte("ModEventS2C"), []any{namespace, system, event, map[string]any{"private": "not-retained-secret"}}, nil},
	}
}

func TestRPCIngressCaptureOptInAndPayloadExclusion(t *testing.T) {
	b := senseBot(t)
	a := &yyPacketIngress{}
	p := rpcIngressPacket([]byte("Minecraft"), "test", []byte("OnPlayerDie"))
	wanted := map[uint32]bool{packet.IDPyRpc: true}
	if !yySelectIncomingPacket(b, p, wanted, a) || a.rpc.observed != 0 {
		t.Fatal("ordinary RPC selection should not activate discovery")
	}
	if _, err := yyCaptureStart(b, yyCaptureConfig{Types: []string{"PyRpc"}}); err != nil {
		t.Fatal(err)
	}
	if !yySelectIncomingPacket(b, p, wanted, a) {
		t.Fatal("discovery changed selection")
	}
	a.rpc.observe(rpcIngressPacket("Minecraft", []byte("test"), "OnPlayerDie"), 512)
	v := yyRPCIngressSnapshot(a)
	rows := v["events"].([]map[string]any)
	if v["observed"] != uint64(2) || len(rows) != 1 || rows[0]["count"] != uint64(2) || rows[0]["namespace"] != "Minecraft" {
		t.Fatal("binary/string identities were not merged", v)
	}
	encoded, err := json.Marshal(a.snapshot())
	if err != nil || strings.Contains(string(encoded), "not-retained-secret") || strings.Contains(string(encoded), "private") {
		t.Fatal("catalog exposed arguments", string(encoded), err)
	}
	yyCaptureStop(b)
	if !yySelectIncomingPacket(b, p, wanted, a) || a.rpc.observed != 2 {
		t.Fatal("stopping capture should stop discovery, not ordinary RPC selection")
	}
	// A separate connection has no catalogue from the old session.
	if len(yyRPCIngressSnapshot(&yyPacketIngress{})["events"].([]map[string]any)) != 0 {
		t.Fatal("session contamination")
	}
}

func TestRPCIngressUnknownShapesDoNotInventEvents(t *testing.T) {
	a := &yyPacketIngress{}
	values := []any{
		nil, "engine callback", []any{}, []any{"ModEventS2C", []any{}, nil},
		[]any{"ModEventC2S", []any{"n", "s", "e", nil}, nil},
		[]any{"ModEventS2C", []any{123, "s", "e", nil}, nil},
		[]any{"ModEventS2C", []any{"n", "s", strings.Repeat("e", 257), nil}, nil},
	}
	for _, v := range values {
		a.rpc.observe(&packet.PyRpc{Value: v}, 5)
	}
	v := yyRPCIngressSnapshot(a)
	if v["unrecognised"] != uint64(len(values)) || len(v["events"].([]map[string]any)) != 0 {
		t.Fatal("unknown valid or invalid RPC shape became a fabricated event", v)
	}
}

func TestRPCIngressBoundedRetentionDoesNotChangePackets(t *testing.T) {
	a := &yyPacketIngress{}
	for _, event := range []string{"A", "B", "C"} {
		p := rpcIngressPacket("n", "s", event)
		a.rpc.observe(p, 2)
		if p.Value.([]any)[1].([]any)[2] != event {
			t.Fatal("observer mutated packet")
		}
	}
	v := yyRPCIngressSnapshot(a)
	rows := v["events"].([]map[string]any)
	if len(rows) != 2 || rows[0]["event"] != "B" || rows[1]["event"] != "C" || v["evicted"] != uint64(1) {
		t.Fatal(v)
	}
	a.rpc.observe(&packet.PyRpc{}, 1) // Live capacity decrease on unknown shape.
	v = yyRPCIngressSnapshot(a)
	if len(v["events"].([]map[string]any)) != 1 || v["evicted"] != uint64(2) || v["capacity"] != 1 {
		t.Fatal(v)
	}
	a.rpc.observe(rpcIngressPacket("n", "s", "A"), 1)
	v = yyRPCIngressSnapshot(a)
	if v["events"].([]map[string]any)[0]["count"] != uint64(1) {
		t.Fatal("count after eviction falsely retained", v)
	}
}

func TestRPCIngressConcurrentCatalogAndSnapshot(t *testing.T) {
	a := &yyPacketIngress{}
	p := rpcIngressPacket("n", "s", "e")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 250; j++ {
				a.rpc.observe(p, 3)
				_ = yyRPCIngressSnapshot(a)
			}
		}()
	}
	wg.Wait()
	v := yyRPCIngressSnapshot(a)
	if v["observed"] != uint64(2000) || v["events"].([]map[string]any)[0]["count"] != uint64(2000) {
		t.Fatal(v)
	}
}
