package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	"math"
	"testing"
	"time"
)

func worldView(t *testing.T, bm *BotManager, mode, target string, options map[string]any) map[string]any {
	t.Helper()
	o := map[string]any{"mode": mode, "target": target}
	for k, v := range options {
		o[k] = v
	}
	v, e := yyWorldLocal(bm, o)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestWorldFarHealthMergePreservesLocalUnknown(t *testing.T) {
	main, far := senseBot(t), senseBot(t)
	main.connected = true
	far.connected = true
	main.serverCode = "A"
	far.serverCode = "A"
	senseHealth(far, 901, 10)
	senseHealth(far, 901, 8)
	r := yyWorldMerge("health", map[string]any{"server": "A"}, []yyWorldView{{"main", worldView(t, main, "health", "901", nil), ""}, {"far", worldView(t, far, "health", "901", nil), ""}})
	if r["known"] != true || r["health"].(map[string]any)["current"] != float64(8) || r["observer"].(map[string]any)["account"] != "far" {
		t.Fatal(r)
	}
	if yySenseActor(main, "901")["known"] != false {
		t.Fatal("federation polluted local evidence")
	}
	if r["global_coverage_guaranteed"] != false {
		t.Fatal("overpromised full coverage")
	}
}
func TestWorldIsolationRemovalAndAmbiguity(t *testing.T) {
	a, b := senseBot(t), senseBot(t)
	a.connected = true
	b.connected = true
	a.serverCode = "A"
	b.serverCode = "B"
	senseHealth(a, 901, 10)
	senseHealth(b, 901, 3)
	merge := func() map[string]any {
		return yyWorldMerge("health", map[string]any{"server": "A"}, []yyWorldView{{"a", worldView(t, a, "health", "901", nil), ""}, {"b", worldView(t, b, "health", "901", nil), ""}})
	}
	if len(merge()["observations"].([]any)) != 1 {
		t.Fatal("cross-server leak")
	}
	b.serverCode = "A"
	s := yySenseFor(b)
	s.Lock()
	s.Actors[901].UniqueID = 902
	s.Unlock()
	if merge()["ambiguous"] != true || merge()["known"] != false {
		t.Fatal("reused runtime id guessed")
	}
	s.Lock()
	s.Actors[901].UniqueID = 0
	s.Actors[901].Dimension = 1
	s.Unlock()
	if merge()["ambiguous"] != true {
		t.Fatal("dimension collision ignored")
	}
	d := worldView(t, b, "health", "901", map[string]any{"dimension": 0})
	if len(d["entities"].([]any)) != 0 {
		t.Fatal("wrong dimension returned")
	}
	b.connected = false
	if len(worldView(t, b, "health", "901", nil)["entities"].([]any)) != 0 {
		t.Fatal("disconnected data active")
	}
	yySenseHandle(a, &packet.RemoveActor{EntityUniqueID: 0})
	if len(worldView(t, a, "health", "901", nil)["entities"].([]any)) != 0 {
		t.Fatal("removed actor active")
	}
}
func TestWorldEventCursorReconnectAndLoss(t *testing.T) {
	b := senseBot(t)
	b.connected = true
	b.serverCode = "A"
	// 显示规则：15格内才推送health事件，先给出本机与目标位置。
	yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 42, Position: mgl32.Vec3{0, 64, 0}})
	yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 901, Position: mgl32.Vec3{1, 64, 0}})
	senseHealth(b, 901, 10)
	senseHealth(b, 901, 8)
	a := worldView(t, b, "events", "", nil)
	cursor := a["cursor"].(uint64)
	session := a["session"]
	if len(a["events"].([]any)) == 0 {
		t.Fatal("no real health events")
	}
	next := worldView(t, b, "events", "", map[string]any{"after": cursor, "session": session})
	if len(next["events"].([]any)) != 0 {
		t.Fatal("cursor duplicated events")
	}
	s := yySenseFor(b)
	s.Lock()
	s.Session = yyWorldSessionID()
	s.Seq = 8
	s.Events = []map[string]any{{"sequence": uint64(8), "type": "health", "timestamp": time.Now().UnixMilli(), "dimension": int32(0)}}
	s.Unlock()
	next = worldView(t, b, "events", "", map[string]any{"after": cursor, "session": session})
	if next["reset"] != true || next["lost"] != uint64(7) {
		t.Fatal(next)
	}
	filtered := worldView(t, b, "events", "", map[string]any{"dimension": 1})
	if len(filtered["events"].([]any)) != 0 || filtered["cursor"] != uint64(8) {
		t.Fatal("filtered cursor stuck")
	}
}
func TestWorldMostRecentSampleAndNoInventedHealth(t *testing.T) {
	a, b := senseBot(t), senseBot(t)
	a.connected = true
	b.connected = true
	a.serverCode = "A"
	b.serverCode = "A"
	senseHealth(a, 901, 10)
	senseHealth(b, 901, 7)
	s := yySenseFor(a)
	s.Lock()
	s.Actors[901].HealthTime = time.Now().Add(-time.Hour)
	s.Unlock()
	r := yyWorldMerge("health", map[string]any{"server": "A"}, []yyWorldView{{"a", worldView(t, a, "health", "901", nil), ""}, {"b", worldView(t, b, "health", "901", nil), ""}})
	if r["health"].(map[string]any)["current"] != float64(7) {
		t.Fatal(r)
	}
	r = yyWorldMerge("health", map[string]any{"server": "A"}, []yyWorldView{{"a", worldView(t, a, "health", "999", nil), ""}, {Account: "unreachable", Error: "timeout"}})
	if r["known"] != false || r["complete"] != false {
		t.Fatal("invented result or hidden partial failure", r)
	}
}
func TestWorldLuaMethodsAreRegistered(t *testing.T) {
	L := newLuaState(senseBot(t))
	defer L.Close()
	defer cancelLuaCtx(L)
	if err := L.DoString(`assert(type(observerBridge.health)=="function");assert(type(observerBridge.worldEvents)=="function");assert(type(observerBridge.coverage)=="function")`); err != nil {
		t.Fatal(err)
	}
}

func TestSelfPositionDoesNotFollowOtherPlayerLocator(t *testing.T) {
	b := senseBot(t)
	b.eid = 77
	b.pos = mgl32.Vec3{1, 2, 3}
	if yyApplySelfPosition(b, &packet.PlayerLocation{EntityUniqueID: 99, Position: mgl32.Vec3{1600, 60, 100}}) || b.pos != (mgl32.Vec3{1, 2, 3}) {
		t.Fatal("other locator moved bot")
	}
	if yyApplySelfPosition(b, &packet.PlayerLocation{EntityUniqueID: 77, Type: packet.PlayerLocationTypeHide}) || b.pos != (mgl32.Vec3{1, 2, 3}) {
		t.Fatal("hide reset bot position")
	}
	if !yyApplySelfPosition(b, &packet.PlayerLocation{EntityUniqueID: 77, Position: mgl32.Vec3{4, 5, 6}}) || b.pos != (mgl32.Vec3{4, 5, 6}) {
		t.Fatal("self locator ignored")
	}
	if yyApplySelfPosition(b, &packet.MovePlayer{EntityRuntimeID: 99, Position: mgl32.Vec3{7, 8, 9}}) {
		t.Fatal("other movement accepted")
	}
	if yyApplySelfPosition(b, &packet.Respawn{EntityRuntimeID: 99, State: packet.RespawnStateReadyToSpawn}) {
		t.Fatal("other respawn accepted")
	}
	if yyApplySelfPosition(b, &packet.PlayerLocation{EntityUniqueID: 77, Position: mgl32.Vec3{float32(math.NaN()), 0, 0}}) {
		t.Fatal("invalid position accepted")
	}
}
func TestLocatorWithoutActorAndUnknownDimension(t *testing.T) {
	b := senseBot(t)
	b.connected = true
	b.serverCode = "A"
	yySenseHandle(b, &packet.PlayerLocation{EntityUniqueID: 99, Position: mgl32.Vec3{1600, 60, 100}})
	if yySenseActor(b, "99")["known"] != false {
		t.Fatal("invented RuntimeID from UniqueID")
	}
	r := worldView(t, b, "locator", "99", nil)
	e := r["entities"].([]any)[0].(map[string]any)
	if e["known"] != true || e["dimension_known"] != false || e["runtime_id"] != nil {
		t.Fatal(e)
	}
	s := yySenseFor(b)
	s.Lock()
	s.Identities["uuid"] = yyObservedIdentity{Name: "远处玩家", UniqueID: 99, Online: true}
	s.Unlock()
	if len(worldView(t, b, "locator", "远处玩家", nil)["entities"].([]any)) != 1 {
		t.Fatal("name did not resolve")
	}
	yySenseHandle(b, &packet.PlayerLocation{EntityUniqueID: 99, Type: packet.PlayerLocationTypeHide})
	e = worldView(t, b, "locator", "99", nil)["entities"].([]any)[0].(map[string]any)
	if e["known"] != false || e["position"] != nil {
		t.Fatal("hidden position leaked as current")
	}
	yySenseReset(b)
	if len(worldView(t, b, "locator", "99", nil)["entities"].([]any)) != 0 {
		t.Fatal("locator leaked across session")
	}
}
