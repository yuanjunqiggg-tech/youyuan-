package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"testing"
)

func TestObserverRPCRequiresMatchingOwnerRequestAndEnvelope(t *testing.T) {
	b, other := senseBot(t), senseBot(t)
	ch := make(chan map[string]any, 1)
	yyObserverRPCPending.Lock()
	yyObserverRPCPending.Items["test-request"] = yyObserverRPCWaiter{Bot: b, Reply: ch}
	yyObserverRPCPending.Unlock()
	defer func() {
		yyObserverRPCPending.Lock()
		delete(yyObserverRPCPending.Items, "test-request")
		yyObserverRPCPending.Unlock()
	}()
	p := &packet.PyRpc{Value: []any{"ModEventS2C", []any{"YouyuanBridge", "ObserverV1", "Capabilities", map[string]any{"request_id": "test-request", "protocol": 1}}, nil}}
	yyObserverRPCReceive(other, p)
	if len(ch) != 0 {
		t.Fatal("other account consumed reply")
	}
	p.Value.([]any)[1].([]any)[0] = "Minecraft"
	yyObserverRPCReceive(b, p)
	if len(ch) != 0 {
		t.Fatal("other namespace consumed reply")
	}
	p.Value.([]any)[1].([]any)[0] = []byte("YouyuanBridge")
	yyObserverRPCReceive(b, p)
	if len(ch) != 1 {
		t.Fatal("valid reply missing")
	}
	yyObserverRPCReceive(b, p)
	if len(ch) != 1 {
		t.Fatal("duplicate reply blocked receive loop")
	}
}

func TestObserverRPCObservationProvenanceAndReset(t *testing.T) {
	b := senseBot(t)
	defer yyObserverRPCReset(b)
	data := map[string]any{"protocol": 1, "event_type": "swing", "health": 17}
	p := &packet.PyRpc{Value: []any{"ModEventS2C", []any{"YouyuanBridge", "ObserverV1", "Observation", data}, nil}}
	yyObserverRPCReceive(b, p)
	data["health"] = 99
	v, err := yyObserverRPCRead(b, []any{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	r := v.(map[string]any)
	rows := r["events"].([]map[string]any)
	if len(rows) != 1 || rows[0]["data"].(map[string]any)["health"] != float64(17) || rows[0]["server_engine_verified"] != false {
		t.Fatalf("invalid observation: %v", r)
	}
	rows[0]["data"].(map[string]any)["health"] = 0
	data["protocol"] = 2
	yyObserverRPCReceive(b, p)
	v, _ = yyObserverRPCRead(b, []any{0, 8})
	r = v.(map[string]any)
	if r["head_cursor"] != uint64(1) || r["events"].([]map[string]any)[0]["data"].(map[string]any)["health"] != float64(17) {
		t.Fatal("copy or protocol isolation failed")
	}
	yyObserverRPCReset(b)
	v, _ = yyObserverRPCRead(b, nil)
	if v.(map[string]any)["head_cursor"] != uint64(0) {
		t.Fatal("reconnect state not reset")
	}
}
