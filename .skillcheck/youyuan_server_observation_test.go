package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	lua "github.com/yuin/gopher-lua"
	"math"
	"testing"
	"time"
)

func serverObservationFixture(b *BotManager, kind string, data map[string]any) {
	yyObserverRPCReceive(b, &packet.PyRpc{Value: []any{"ModEventS2C", []any{"YouyuanBridge", "ObserverV1", "Observation", map[string]any{"protocol": 1, "event_type": kind, "data": data}}, nil}})
}
func TestServerObservationHealthSessionAndProvenance(t *testing.T) {
	b := senseBot(t)
	defer yyObserverRPCReset(b)
	s := yySenseFor(b)
	s.Session = "session-a"
	v, e := yyServerObservationCall(b, "observerBridge.serverSnapshot", []any{"9007199254740999"})
	if e != nil || v.(map[string]any)["known"] != false {
		t.Fatal(v, e)
	}
	serverObservationFixture(b, "health_changed", map[string]any{"entity_id": "9007199254740999", "timestamp_ms": time.Now().Add(-time.Minute).UnixMilli(), "health": map[string]any{"current": 14.0, "max": 40.0, "previous": 20.0}})
	v, e = yyServerObservationCall(b, "observerBridge.serverSnapshot", []any{"9007199254740999"})
	r := v.(map[string]any)
	if e != nil || r["known"] != true || r["current"] != 14.0 || r["max"] != 40.0 || r["stale"] != true || r["server_engine_verified"] != false {
		t.Fatal(v, e)
	}
	if len(s.Actors) != 0 {
		t.Fatal("module data overwrote native attribute cache")
	}
	v, e = yyServerObservationCall(b, "observerBridge.serverHistory", []any{map[string]any{"session": "older", "after": float64(99)}})
	r = v.(map[string]any)
	if e != nil || r["reset"] != true || len(r["events"].([]any)) != 1 {
		t.Fatal(v, e)
	}
	yyObserverRPCReset(b)
	s.Session = "session-b"
	v, _ = yyServerObservationCall(b, "observerBridge.serverSnapshot", []any{"9007199254740999"})
	if v.(map[string]any)["known"] != false {
		t.Fatal("old health survived reconnect", v)
	}
}
func TestServerObservationHealthRejectsDisplayAndInvalid(t *testing.T) {
	for _, data := range []map[string]any{
		{"damage_text": "-33"},
		{"health": map[string]any{"requested_current": 14.0}},
		{"health": map[string]any{"current": math.NaN()}},
		{"health": map[string]any{"current": 30.0, "max": 20.0}},
	} {
		if _, ok := yyServerHealth(data); ok {
			t.Fatal("invalid or display health accepted", data)
		}
	}
	h, ok := yyServerHealth(map[string]any{"health": map[string]any{"current": 0.0}})
	if !ok || h["max"] != nil || h["max_known"] != false {
		t.Fatal(h, ok)
	}
	if yyServerID(float64(9007199254740999)) != "" {
		t.Fatal("float entity ID accepted")
	}
}
func TestServerObservationBindingsAndSDKDelivery(t *testing.T) {
	b := senseBot(t)
	defer yyObserverRPCReset(b)
	L := newLuaState(b)
	defer L.Close()
	defer cancelLuaCtx(L)
	for method := range yyServerObservationMethods {
		parts := []string{"observerBridge", method[len("observerBridge."):]}
		if method[:7] == "events." {
			parts = []string{"events", method[7:]}
		}
		if L.GetField(L.GetGlobal(parts[0]), parts[1]).Type() != lua.LTFunction {
			t.Fatal("missing binding", method)
		}
	}
	inst := &yyRuntimeInstance{Key: "main:server_observation_test", Account: "main", Manifest: yyRuntimeManifest{ID: "server_observation_test"}, Queue: make(chan map[string]any, 16), Done: make(chan struct{})}
	yySDKPrepare(inst, t.TempDir())
	defer yySDKClose(inst.Key)
	ctx := yySDKGet(inst.Key)
	ctx.addListener(yySDKListener{Kind: "global_swing", Handler: "swing"})
	ctx.addListener(yySDKListener{Kind: "global_health", Handler: "health"})
	// Nil bot is the isolated main SDK fixture, not a live server.
	defer yyObserverRPCReset(nil)
	serverObservationFixture(nil, "swing", map[string]any{"player_id": "9007199254740999"})
	serverObservationFixture(nil, "health_changed", map[string]any{"entity_id": "9007199254740999", "health": map[string]any{"current": 14.0, "max": 40.0}})
	for _, handler := range []string{"swing", "health"} {
		select {
		case event := <-inst.Queue:
			if event["handler"] != handler {
				t.Fatal(event)
			}
		case <-time.After(time.Second):
			t.Fatal("missing SDK callback", handler)
		}
	}
}

func TestServerObservationSnapshotSurvivesHistoryRotation(t *testing.T) {
	b := senseBot(t)
	defer yyObserverRPCReset(b)
	old := yyObservationSettings()
	defer yyObservationConfig.Store(old)
	limits := old
	limits.EventHistory = 2
	yyObservationConfig.Store(limits)
	serverObservationFixture(b, "health_changed", map[string]any{"entity_id": "exact-id", "health": map[string]any{"current": 14.0, "max": 40.0}})
	for i := 0; i < 4; i++ {
		serverObservationFixture(b, "swing", map[string]any{"player_id": "player"})
	}
	v, e := yyServerObservationCall(b, "observerBridge.serverHistory", []any{map[string]any{"after": 0.0}})
	r := v.(map[string]any)
	if e != nil || r["lost"] != uint64(3) || len(r["events"].([]any)) != 2 {
		t.Fatal(v, e)
	}
	v, e = yyServerObservationCall(b, "observerBridge.serverSnapshot", []any{"exact-id"})
	if e != nil || v.(map[string]any)["current"] != 14.0 {
		t.Fatal("health lost when unrelated history rotated", v, e)
	}
	v.(map[string]any)["current"] = 999.0
	v, _ = yyServerObservationCall(b, "observerBridge.serverSnapshot", []any{"exact-id"})
	if v.(map[string]any)["current"] != 14.0 {
		t.Fatal("caller modified shared snapshot", v)
	}
}

func TestServerObservationFutureTimestampStaysStale(t *testing.T) {
	b := senseBot(t)
	defer yyObserverRPCReset(b)
	serverObservationFixture(b, "health_changed", map[string]any{"entity_id": "id", "timestamp_ms": time.Now().Add(time.Hour).UnixMilli(), "health": map[string]any{"current": 10.0}})
	v, e := yyServerObservationCall(b, "observerBridge.serverSnapshot", []any{"id"})
	r := v.(map[string]any)
	if e != nil || r["known"] != true || r["stale"] != true || r["timestamp_valid"] != false {
		t.Fatal(v, e)
	}
}
