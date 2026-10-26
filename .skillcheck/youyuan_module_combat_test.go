package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	lua "github.com/yuin/gopher-lua"
)

func moduleRecorded(t *testing.T) []*packet.PyRpc {
	t.Helper()
	b, err := os.ReadFile("testdata/module_combat_far_01026.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Operation uint32 `json:"operation_type"`
		Value     any    `json:"value"`
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	out := []*packet.PyRpc{}
	for _, r := range rows {
		out = append(out, &packet.PyRpc{OperationType: r.Operation, Value: r.Value})
	}
	return out
}
func TestModuleCombatRecordedFarPlayerAndNoHealthFabrication(t *testing.T) {
	bm := senseBot(t)
	bm.connected = true
	s := yySenseFor(bm)
	s.Identities["uuid"] = yyObservedIdentity{UniqueID: -8589934568, Name: "疣螈", UUID: "uuid", XUID: "076005b2", Online: true}
	rows := moduleRecorded(t)
	kills, displays := 0, 0
	for _, p := range rows {
		yySenseHandle(bm, p)
	}
	for _, e := range s.ModuleEvents {
		if e["evidence"] != "server_module_reported" {
			t.Fatal(e)
		}
		if e["type"] == "entity_kill" {
			kills++
			a := e["attacker"].(map[string]any)
			if a["name"] != "疣螈" || a["unique_id"] != "-8589934568" || a["runtime_id"] != nil || e["attacker_known"] != true {
				t.Fatal(e)
			}
			target := e["target"].(map[string]any)
			if target["health"] != nil || target["runtime_id"] != nil || target["name_known"] != false {
				t.Fatal(e)
			}
		}
		if e["type"] == "damage_display" {
			displays++
			if e["damage_known"] != false || e["attacker_known"] != false || e["dimension_known"] != false {
				t.Fatal(e)
			}
		}
	}
	if kills != 3 || displays != 31 || len(s.Actors) != 0 {
		t.Fatalf("kills=%d displays=%d actors=%d", kills, displays, len(s.Actors))
	}
	// Duplicate broadcast cannot double count a kill; display updates remain distinct.
	for _, p := range rows {
		yySenseHandle(bm, p)
	}
	total := 0
	for _, e := range s.ModuleEvents {
		if e["type"] == "entity_kill" {
			total++
		}
	}
	if total != 3 {
		t.Fatal(total)
	}
}
func TestModuleCombatRejectsForgedSourceAndFloatIDs(t *testing.T) {
	s := yySenseFor(senseBot(t))
	rows := moduleRecorded(t)
	var original *packet.PyRpc
	for _, p := range rows {
		key, _ := yyRPCIngressIdentity(p)
		if key.Namespace == "Minecraft" {
			original = p
			break
		}
	}
	if original == nil {
		t.Fatal("kill fixture missing")
	}
	for _, mutate := range []func(*packet.PyRpc){
		func(p *packet.PyRpc) { p.OperationType = packet.PyRpcOperationTypeSend },
		func(p *packet.PyRpc) { p.Value.([]any)[1].([]any)[0] = "other" },
		func(p *packet.PyRpc) { p.Value.([]any)[1].([]any)[1] = "other" },
		func(p *packet.PyRpc) { p.Value.([]any)[1].([]any)[2] = "PlayerKillMobnot-a-uuid" },
		func(p *packet.PyRpc) {
			p.Value.([]any)[1].([]any)[3].(map[string]any)["playerId"] = float64(-8589934568)
		},
	} {
		p := &packet.PyRpc{Value: yyObservationCopy(original.Value), OperationType: original.OperationType}
		mutate(p)
		if len(yyModuleCombatEvents(s, p, time.Now())) != 0 {
			t.Fatal("untrusted kill accepted")
		}
	}
}
func TestModuleHistoryPagingResetAndEvidence(t *testing.T) {
	bm := senseBot(t)
	bm.connected = true
	s := yySenseFor(bm)
	for _, p := range moduleRecorded(t) {
		yySenseHandle(bm, p)
	}
	read := func(o map[string]any) map[string]any {
		v, e := yySenseCall(bm, "events.moduleHistory", []any{o})
		if e != nil {
			t.Fatal(e)
		}
		return v.(map[string]any)
	}
	first := read(map[string]any{"limit": 2})
	second := read(map[string]any{"after": float64(first["cursor"].(uint64)), "session": first["session"], "limit": 2})
	if len(first["events"].([]any)) != 2 || second["cursor"].(uint64) <= first["cursor"].(uint64) {
		t.Fatal(first, second)
	}
	s.Events = nil // Native movement history can be cleared without losing combat journal.
	if len(read(map[string]any{})["events"].([]any)) != 34 {
		t.Fatal("journal lost")
	}
	local, _ := yyWorldLocal(bm, map[string]any{"mode": "events"})
	local["events"] = first["events"]
	merged := yyWorldMerge("events", map[string]any{}, []yyWorldView{{Account: "main", Data: local}})
	e := merged["events"].([]any)[0].(map[string]any)
	if e["evidence"] != "server_module_reported" || e["transport_evidence"] != "server_to_bot_decoded" {
		t.Fatal(e)
	}
	yySenseReset(bm)
	reset := read(map[string]any{"session": first["session"], "after": float64(99)})
	if reset["reset"] != true || reset["cursor"] != uint64(0) || len(reset["events"].([]any)) != 0 || len(s.ModuleKills) != 0 {
		t.Fatal(reset)
	}
}
func TestModuleLuaInterfacesRegister(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("__PLUGIN_ID__", lua.LString("test_module"))
	registerAllLuaAPI(L, senseBot(t))
	if err := L.DoString(`assert(type(events.onEntityKill)=="function"); assert(type(events.onDamageDisplay)=="function"); assert(type(events.moduleHistory)=="function")`); err != nil {
		t.Fatal(err)
	}
}

func TestModulePositionLuaArray(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	pos, known := yyModulePosition(map[string]any{"__type__": "tuple", "value": []any{float64(1), float64(2), float64(3)}})
	if !known {
		t.Fatal("real tuple position rejected")
	}
	L.SetGlobal("event", goToLua(L, map[string]any{"position": pos, "position_known": known}))
	if err := L.DoString(`assert(type(event.position)=="table"); assert(event.position[1]==1 and event.position[2]==2 and event.position[3]==3)`); err != nil {
		t.Fatal(err)
	}
}

func TestModuleSDKCallbackCarriesActualAttacker(t *testing.T) {
	bm := senseBot(t)
	botMgrMu.Lock()
	old := botMgr
	botMgr = bm
	botMgrMu.Unlock()
	t.Cleanup(func() { botMgrMu.Lock(); botMgr = old; botMgrMu.Unlock() })
	s := sdkFixture(t, "module_sdk")
	if _, err := s.call("events.onEntityKill", []any{"on_kill"}); err != nil {
		t.Fatal(err)
	}
	for _, p := range moduleRecorded(t) {
		key, _ := yyRPCIngressIdentity(p)
		if key.Namespace == "Minecraft" {
			yySenseHandle(bm, p)
			break
		}
	}
	if len(s.Instance.Queue) != 1 {
		t.Fatal("module callback not delivered", len(s.Instance.Queue))
	}
}
