package main

import (
	"context"
	"encoding/json"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPatrolRestoreUsesCommandFeetInsteadOfInputEye(t *testing.T) {
	feet := yyPatrolFeetPosition(mgl32.Vec3{-123.456, 65.62001, 99.25})
	if math.Abs(float64(feet[1]-64)) > .0001 || feet[0] != -123.456 || feet[2] != 99.25 {
		t.Fatal(feet)
	}
}

func TestMappedEventRealPacketFilterIsolationAndUnload(t *testing.T) {
	d, e := yyEventDefinitionParse(map[string]any{"name": "arm_seen", "packet": "Animate", "filter": map[string]any{"fields": map[string]any{"action_type": float64(1)}}, "fields": map[string]any{"runtime_id": "entity_runtime_id", "missing": "unknown"}})
	if e != nil {
		t.Fatal(e)
	}
	b := senseBot(t)
	rows := []map[string]any{}
	u := yyEventAdapter(b, d, func(e map[string]any) { rows = append(rows, e) })
	defer u()
	packetHubDispatch(&BotManager{}, &packet.Animate{ActionType: 1, EntityRuntimeID: 8})
	packetHubDispatch(b, &packet.Animate{ActionType: 4, EntityRuntimeID: 8})
	packetHubDispatch(b, &packet.Animate{ActionType: 1, EntityRuntimeID: 8})
	if len(rows) != 1 || rows[0]["type"] != "arm_seen" || rows[0]["inferred"] != false || rows[0]["source"] != "custom_packet_mapping" || rows[0]["missing"] != nil {
		t.Fatal(rows)
	}
	u()
	packetHubDispatch(b, &packet.Animate{ActionType: 1, EntityRuntimeID: 8})
	if len(rows) != 1 {
		t.Fatal("unsubscribe leaked")
	}
}
func TestMappedEventsRejectPrivateAndProvenanceOverride(t *testing.T) {
	for _, v := range []map[string]any{{"name": "test", "packet": "Login"}, {"name": "test", "packet": "*"}, {"name": "test", "packet": "LevelChunk"}, {"name": "test", "packet": "MissingPacket"}, {"name": "test", "packet": "Animate", "fields": map[string]any{"source": "action_type"}}} {
		if _, e := yyEventDefinitionParse(v); e == nil {
			t.Fatal(v)
		}
	}
}
func TestObservedActionDoesNotTurnHurtIntoSwing(t *testing.T) {
	for _, k := range []string{"swing", "hurt", "equipment", "rotation", "entity_kill", "damage_display"} {
		if !yySenseEventMatches("observed_action", map[string]any{"type": k}) {
			t.Fatal(k)
		}
	}
	if yySenseEventMatches("swing", map[string]any{"type": "hurt"}) || yySenseEventMatches("observed_action", map[string]any{"type": "move"}) {
		t.Fatal("fabricated swing or unrelated movement")
	}
}
func TestPatrolIntervalsRouteAndOwner(t *testing.T) {
	for _, sec := range []float64{.001, .0001, 2} {
		d, e := yyPatrolDuration(sec)
		if e != nil || d != time.Duration(sec*float64(time.Second)) {
			t.Fatal(sec, d, e)
		}
	}
	for _, sec := range []float64{0, -1, math.NaN(), math.Inf(1), 1e-12} {
		if _, e := yyPatrolDuration(sec); e == nil {
			t.Fatal(sec)
		}
	}
	if _, e := yyPatrolConfigParse(map[string]any{"players": []any{"a", "a"}}); e == nil {
		t.Fatal("duplicate player")
	}
	if yyPatrolCommand("yy_o_test", 3) != "tp @s @a[scores={yy_o_test=3},c=1]" {
		t.Fatal("route command")
	}
	b := senseBot(t)
	cancelled := false
	p := &yyPatrol{Bot: b, Owner: "lua:test", Cancel: func() { cancelled = true }, Done: make(chan struct{})}
	yyPatrols.Lock()
	yyPatrols.Items[b] = p
	yyPatrols.Unlock()
	defer func() { yyPatrols.Lock(); delete(yyPatrols.Items, b); yyPatrols.Unlock() }()
	yyPatrolStopOwner(b, "lua:other")
	if cancelled {
		t.Fatal("other owner stopped")
	}
	yyPatrolStopOwner(b, "lua:test")
	if !cancelled {
		t.Fatal("owner unload leaked patrol")
	}
}
func TestAccountTokenIsExplicitAndTenantScoped(t *testing.T) {
	yyPoolTestSetup(t)
	request := func(actor yyPoolActor, id string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/youyuan/accounts", strings.NewReader(`{"action":"token","id":"`+id+`"}`))
		r = r.WithContext(context.WithValue(r.Context(), yyPoolActorContext{}, actor))
		w := httptest.NewRecorder()
		yyMux.ServeHTTP(w, r)
		return w
	}
	w := request(yyPoolActor{ID: "alice", Scope: "control"}, "alice_bot")
	var v map[string]any
	if json.Unmarshal(w.Body.Bytes(), &v) != nil || v["token"] != "token-a" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Body.String())
	}
	for _, c := range []struct {
		a  yyPoolActor
		id string
	}{{yyPoolActor{ID: "alice", Scope: "control"}, "bob_bot"}, {yyPoolActor{ID: "alice", Scope: "read"}, "alice_bot"}, {yyPoolActor{ID: "alice", Scope: "control"}, "main"}} {
		if request(c.a, c.id).Code != 403 {
			t.Fatal("credential isolation failed", c.id)
		}
	}
}
