package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	"testing"
	"time"
)

func TestContainerCacheInvalidatesWithoutInventoryExposure(t *testing.T) {
	b := senseBot(t)
	b.connected = true
	pos := protocol.BlockPos{1, 64, 2}
	yySenseHandle(b, &packet.BlockActorData{Position: pos, NBTData: map[string]any{"id": "Chest", "Items": []any{"private"}}})
	v := obsCall(t, b, "world.containerState", float64(1), float64(64), float64(2))
	if v["known"] != true || v["is_container"] != true || v["block_entity_id"] != "Chest" {
		t.Fatal(v)
	}
	if _, ok := v["Items"]; ok {
		t.Fatal("inventory leaked")
	}
	yySenseHandle(b, &packet.UpdateBlock{Position: pos, NewBlockRuntimeID: 1})
	if obsCall(t, b, "world.containerState", float64(1), float64(64), float64(2))["known"] != false {
		t.Fatal("old container survived replacement")
	}
	yySenseHandle(b, &packet.BlockActorData{Position: pos, NBTData: map[string]any{"id": "Barrel"}})
	if obsCall(t, b, "world.containerState", float64(1), float64(64), float64(2))["is_container"] != true {
		t.Fatal("rapid replacement lost")
	}
	b.dimension = 1
	if obsCall(t, b, "world.containerState", float64(1), float64(64), float64(2))["known"] != false {
		t.Fatal("dimension leak")
	}
	if obsCall(t, b, "world.blockState", float64(88), float64(64), float64(2))["known"] != false {
		t.Fatal("unobserved block known")
	}
}

func TestActionHistorySurvivesMovementFlood(t *testing.T) {
	b := senseBot(t)
	b.connected = true
	yySenseHandle(b, &packet.AddPlayer{EntityRuntimeID: 7, Username: "P"})
	yySenseHandle(b, &packet.Animate{EntityRuntimeID: 7, ActionType: packet.AnimateActionSwingArm})
	for i := 0; i < yyObservationSettings().EventHistory+10; i++ {
		yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 7, Position: mgl32.Vec3{float32(i), 64, 0}})
	}
	v, e := yyWorldLocal(b, map[string]any{"mode": "events", "stream": "actions", "limit": float64(128)})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, r := range v["events"].([]any) {
		if r.(map[string]any)["type"] == "swing" {
			found = true
		}
	}
	if !found || v["lost"] != uint64(0) {
		t.Fatal("movement displaced swing", v)
	}
}

func TestPublicExportCallKeepsStateAndDoesNotRestartStopped(t *testing.T) {
	b := &BotManager{}
	id := "qa_public_call45"
	if e := LoadLuaPlugin(b, id, `counter=0;plugin_api={next=function()counter=counter+1;return counter end}`); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(b, id)
	for i := 1; i <= 2; i++ {
		v, e := yySenseCall(b, "plugin.call", []any{id, "next", []any{}})
		if e != nil || v != float64(i) {
			t.Fatal("export state", v, e)
		}
	}
	if _, e := yySenseCall(b, "plugin.call", []any{id, "counter", []any{}}); e == nil {
		t.Fatal("private global callable")
	}
	yyStopResident(b, id)
	if _, e := yySenseCall(b, "plugin.call", []any{id, "next", []any{}}); e == nil {
		t.Fatal("stopped plugin restarted")
	}
}

func TestSceneStatsDistinguishesRetainedAndPresent(t *testing.T) {
	b := senseBot(t)
	b.connected = true
	yySenseHandle(b, &packet.AddActor{EntityRuntimeID: 11, EntityUniqueID: 11, EntityType: "minecraft:cow"})
	yySenseHandle(b, &packet.AddActor{EntityRuntimeID: 12, EntityUniqueID: 12, EntityType: "minecraft:pig"})
	yySenseHandle(b, &packet.RemoveActor{EntityUniqueID: 12})
	v := yySenseStats(b)
	if v["cached"] != 2 || v["present"] != 1 || v["fresh"] != 1 {
		t.Fatal(v)
	}
	s := yySenseFor(b)
	s.Lock()
	s.Actors[11].LastSeen = time.Now().Add(-time.Hour)
	s.Unlock()
	if yySenseStats(b)["fresh"] != 0 {
		t.Fatal("stale actor counted as fresh")
	}
	v = obsCall(t, b, "game.sceneStats")
	if v["active_actors"] != 0 {
		t.Fatal(v)
	}
	if len(yySenseSnapshot(b)["actors"].([]map[string]any)) != 2 {
		t.Fatal("legacy snapshot compatibility lost")
	}
	filtered, e := yySenseCall(b, "game.scene", []any{map[string]any{"present_only": true, "entity_type": "minecraft:cow"}})
	if e != nil || len(filtered.(map[string]any)["actors"].([]map[string]any)) != 1 {
		t.Fatal("filtered scene", filtered, e)
	}
	b.connected = false
	if yySenseStats(b)["fresh"] != 0 {
		t.Fatal("disconnected freshness")
	}
}
func TestBlockAsyncRejectsInvalidAndDisconnected(t *testing.T) {
	b := senseBot(t)
	owner := yyAsyncCommandKey{Bot: b, Plugin: "qa_block44"}
	defer yyClearCommandTickets(owner)
	if _, e := yyAsyncCommandCall(owner, "game.getBlockTileAsync", []any{float64(1), float64(2), float64(3)}, yyDefaultCommandOptions()); e == nil {
		t.Fatal("disconnected accepted")
	}
	b.connected = true
	for _, a := range [][]any{{}, {float64(1.5), float64(2), float64(3)}, {"1", float64(2), float64(3)}} {
		if _, e := yyAsyncCommandCall(owner, "game.getBlockTileAsync", a, yyDefaultCommandOptions()); e == nil {
			t.Fatal("invalid coordinate", a)
		}
	}
}
func benchmarkScene44(b *testing.B, full bool) {
	bm := &BotManager{}
	s := yySenseFor(bm)
	defer func() { yySenses.Lock(); delete(yySenses.Items, bm); yySenses.Unlock() }()
	for i := uint64(1); i <= 1835; i++ {
		s.Actors[i] = &yyActorObservation{RuntimeID: i, Present: i < 35, LastSeen: time.Now(), Equipment: map[string]yyObservedItem{}, Attributes: map[string]yyObservedAttribute{}}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if full {
			yySenseSnapshot(bm)
		} else {
			yySenseStats(bm)
		}
	}
}
func BenchmarkScene44Full(b *testing.B)  { benchmarkScene44(b, true) }
func BenchmarkScene44Stats(b *testing.B) { benchmarkScene44(b, false) }
