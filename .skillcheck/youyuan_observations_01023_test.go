package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
	lua "github.com/yuin/gopher-lua"
)

func obsCall(t *testing.T, b *BotManager, m string, args ...any) map[string]any {
	t.Helper()
	v, e := yySenseCall(b, m, args)
	if e != nil {
		t.Fatal(m, e)
	}
	return v.(map[string]any)
}
func TestObservationHealthTruthAndReuse(t *testing.T) {
	b := senseBot(t)
	b.connected = true
	// 显示规则：15格内且攻击行为后3秒内的受伤才推送事件。
	yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 42, Position: mgl32.Vec3{0, 64, 0}})
	yySenseHandle(b, &packet.AddActor{EntityRuntimeID: 5332, EntityUniqueID: 5332, EntityType: "minecraft:zombie", Position: mgl32.Vec3{1, 64, 1}, Attributes: []protocol.AttributeValue{{Name: "minecraft:health", Value: 40, Max: 40}}})
	yySenseHandle(b, &packet.Animate{EntityRuntimeID: 5332, ActionType: packet.AnimateActionSwingArm})
	yySenseHandle(b, &packet.UpdateAttributes{EntityRuntimeID: 5332, Tick: 71, Attributes: []protocol.Attribute{{AttributeValue: protocol.AttributeValue{Name: "minecraft:health", Value: 14.14, Max: 40}}}})
	h := obsCall(t, b, "entities.health", "5332")
	if h["max"] != float64(40) || h["previous"] != float64(40) || math.Abs(h["damage"].(float64)-25.86) > .001 || h["known"] != true || h["stale"] != false {
		t.Fatal(h)
	}
	if obsCall(t, b, "util.serverTick")["tick"] != uint64(71) {
		t.Fatal("server tick lost")
	}
	yySenseHandle(b, &packet.ActorEvent{EntityRuntimeID: 5332, EventType: packet.ActorEventHurt})
	s := yySenseFor(b)
	hurt := 0
	for _, e := range s.Events {
		if e["type"] == "hurt" {
			hurt++
			if e["attacker_known"] != false {
				t.Fatal("invented attacker", e)
			}
		}
	}
	if hurt != 1 {
		t.Fatal("duplicate health damage", hurt)
	}
	yySenseHandle(b, &packet.RemoveActor{EntityUniqueID: 5332})
	if obsCall(t, b, "entities.health", "5332")["stale"] != true {
		t.Fatal("removed cache trusted")
	}
	yySenseHandle(b, &packet.AddActor{EntityRuntimeID: 5332, EntityUniqueID: 9000, EntityType: "minecraft:pig"})
	if obsCall(t, b, "entities.health", "5332")["known"] != false {
		t.Fatal("runtime reuse inherited health")
	}
	yySenseHandle(b, &packet.SetHealth{Health: 12})
	h = obsCall(t, b, "entity.health", "self")
	if h["max"] != nil || h["max_known"] != false {
		t.Fatal("assumed maximum", h)
	}
	yySenseHandle(b, &packet.UpdateAttributes{EntityRuntimeID: 100, Attributes: []protocol.Attribute{{AttributeValue: protocol.AttributeValue{Name: "minecraft:health", Value: float32(math.NaN()), Max: 20}}}})
	if obsCall(t, b, "entity.health", "100")["known"] != false {
		t.Fatal("NaN accepted")
	}
}
func TestObservationRotationWithoutPosition(t *testing.T) {
	b := senseBot(t)
	b.connected = true
	yySenseHandle(b, &packet.MoveActorDelta{EntityRuntimeID: 90, Flags: packet.MoveActorDeltaFlagHasRotX, Rotation: mgl32.Vec3{67, 0, 0}})
	r := obsCall(t, b, "player.rotation", "90")
	if r["pitch"] != float32(67) || r["yaw_known"] != false || r["known"] != false {
		t.Fatal(r)
	}
	yySenseHandle(b, &packet.AddPlayer{EntityRuntimeID: 91, Username: "P", Position: mgl32.Vec3{1, 2, 3}, Pitch: 0, Yaw: 0})
	yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 91, Position: mgl32.Vec3{1, 2, 3}, Pitch: 80, Yaw: 25, HeadYaw: 27})
	r = obsCall(t, b, "player.rotation", "P")
	if r["pitch"] != float32(80) || r["yaw"] != float32(25) || r["stale"] != false {
		t.Fatal(r)
	}
}
func TestObservationEquipmentTranslationAndUnknown(t *testing.T) {
	b := senseBot(t)
	b.connected = true
	item := protocol.ItemInstance{Stack: protocol.ItemStack{ItemType: protocol.ItemType{NetworkID: 600}, Count: 1, NBTData: map[string]any{"ench": []any{map[string]any{"id": int16(9), "lvl": int16(3)}}}}}
	yySenseHandle(b, &packet.MobArmourEquipment{EntityRuntimeID: 42, Helmet: item})
	e := obsCall(t, b, "player.equipment", "self")
	head := e["head"].(map[string]any)
	if head["item_id_known"] != false || head["enchantments_known"] != true || head["expires_at"] != nil {
		t.Fatal(head)
	}
	if e["main_hand"].(map[string]any)["known"] != false {
		t.Fatal("unobserved slot invented")
	}
	yySenseHandle(b, &packet.ItemRegistry{Items: []protocol.ItemEntry{{RuntimeID: 600, Name: "minecraft:diamond_helmet"}}})
	e = obsCall(t, b, "player.equipment", "self")
	if e["head"].(map[string]any)["item_id"] != "minecraft:diamond_helmet" {
		t.Fatal(e)
	}
	yySenseHandle(b, &packet.MobEquipment{EntityRuntimeID: 42, WindowID: protocol.WindowIDOffHand, NewItem: protocol.ItemInstance{}})
	e = obsCall(t, b, "player.equipment", "self")
	if e["off_hand"].(map[string]any)["item_id"] != "minecraft:air" {
		t.Fatal(e)
	}
}
func TestObservationKillLifecycleAndImmutableHistory(t *testing.T) {
	b := senseBot(t)
	id := uuid.New()
	yySenseHandle(b, &packet.PlayerList{Entries: []protocol.PlayerListEntry{{UUID: id, Username: "V", EntityUniqueID: 7, XUID: "real-xuid"}}})
	yySenseHandle(b, &packet.AddPlayer{EntityRuntimeID: 7, UUID: id, Username: "V", AbilityData: protocol.AbilityData{EntityUniqueID: 7}})
	yySenseHandle(b, &packet.AddPlayer{EntityRuntimeID: 8, Username: "K", AbilityData: protocol.AbilityData{EntityUniqueID: 8}})
	yySenseHandle(b, &packet.ActorEvent{EntityRuntimeID: 7, EventType: packet.ActorEventDeath})
	s := yySenseFor(b)
	var original map[string]any
	for _, e := range s.Events {
		if e["type"] == "death" {
			original = e
		}
	}
	yySenseHandle(b, &packet.RemoveActor{EntityUniqueID: 7})
	msg := &packet.Text{TextType: packet.TextTypeTranslation, Message: "death.attack.player", Parameters: []string{"V", "K"}}
	yySenseHandle(b, msg)
	yySenseHandle(b, msg)
	if original["killer_known"] != false {
		t.Fatal("queued original mutated")
	}
	kills := 0
	deaths := 0
	for _, e := range s.Events {
		if e["type"] == "player_kill" {
			kills++
			if e["event_id"] != original["event_id"] {
				t.Fatal("kill ID mismatch")
			}
		}
		if e["type"] == "death" {
			deaths++
		}
	}
	if kills != 1 || deaths != 1 {
		t.Fatal(kills, deaths)
	}
	yySenseHandle(b, &packet.Respawn{EntityRuntimeID: 7, State: packet.RespawnStateReadyToSpawn})
	yySenseHandle(b, &packet.Respawn{EntityRuntimeID: 7, State: packet.RespawnStateReadyToSpawn})
	respawn := 0
	for _, e := range s.Events {
		if e["type"] == "respawn" {
			respawn++
		}
	}
	if respawn != 1 {
		t.Fatal(respawn)
	}
	if yySenseActor(b, "V")["xuid"] != "real-xuid" {
		t.Fatal("identity lost")
	}
}
func TestObservationConfigPersistenceQueuesAndCaptureRing(t *testing.T) {
	old := yyObservationSettings()
	oldPath := yyObservationConfigPath
	t.Cleanup(func() { yyObservationConfig.Store(old); yyObservationConfigPath = oldPath })
	root := t.TempDir()
	yyLoadObservationSettings(root)
	_, e := yyConfigureObservationSettings([]any{map[string]any{"event_queue": float64(2048), "priority_queue": float64(1024), "async_tickets": float64(4096), "event_history": float64(2048)}})
	if e != nil {
		t.Fatal(e)
	}
	b := senseBot(t)
	if e = yyLoadResident(b, "obs-config", "function idle() end", false); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(b, "obs-config")
	r, _ := yyGetResident(b, "obs-config")
	if cap(r.senseQueue) != 2048 || cap(r.priorityQueue) != 1024 {
		t.Fatal("configured queue not applied")
	}
	if yySenseDeliveryQueue(r, []any{map[string]any{"type": "hurt"}}) != r.priorityQueue {
		t.Fatal("hurt ordinary queue")
	}
	if _, e = os.Stat(filepath.Join(root, "observation-limits.json")); e != nil {
		t.Fatal(e)
	}
	yyObservationConfig.Store(old)
	yyLoadObservationSettings(root)
	if yyObservationSettings().AsyncTickets != 4096 {
		t.Fatal("configuration not persisted")
	}
	_, e = yyCaptureStart(b, yyCaptureConfig{Types: []string{"MovePlayer"}, Capacity: 3000})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3100; i++ {
		yyCapturePacket(b, &packet.MovePlayer{EntityRuntimeID: 42})
	}
	v := yyCaptureRead(b, 0, 65536)
	rows := v["records"].([]map[string]any)
	if len(rows) != 3000 || v["overwritten"] != uint64(100) || rows[0]["sequence"] != float64(101) || rows[2999]["sequence"] != float64(3100) {
		t.Fatal("ring chronology lost", len(rows), v["overwritten"])
	}
	_, e = yyCaptureStart(b, yyCaptureConfig{Types: []string{"MovePlayer"}, Capacity: 1000, MaxBytes: 512, TotalBytes: 1536})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 50; i++ {
		yyCapturePacket(b, &packet.MovePlayer{})
	}
	v = yyCaptureRead(b, 0, 1000)
	if v["retained_serialized_bytes_estimate"].(int) > 1536 || v["overwritten"].(uint64) == 0 {
		t.Fatal("budget ignored", v)
	}
}
func TestObservationLuaFunctionCallbackAndAsyncBinding(t *testing.T) {
	b := senseBot(t)
	id := "observation-callback"
	src := `hits=0; events.onEntityHurt(function(e) hits=hits+1 end); function count() return hits end; function request() return game.sendCommandAsync("say test") end`
	if e := yyLoadResident(b, id, src, false); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(b, id)
	senseHealth(b, 42, 20)
	// 显示规则：受伤需在攻击行为后3秒内，先制造一次挥臂。
	yySenseHandle(b, &packet.Animate{EntityRuntimeID: 42, ActionType: packet.AnimateActionSwingArm})
	senseHealth(b, 42, 18)
	deadline := time.Now().Add(time.Second)
	for {
		n, e := yyCallResident(b, id, "count", nil, false, false)
		if e != nil {
			t.Fatal(e)
		}
		if n == float64(1) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("function callback not invoked", n)
		}
		time.Sleep(time.Millisecond)
	}
	v, e := yyCallResident(b, id, "request", nil, false, false)
	if e != nil || v.(map[string]any)["ticket"] == nil {
		t.Fatal("async binding overwritten", v, e)
	}
	_, e = yySenseCall(b, "not.aMethod", []any{"not coordinates"})
	if e == nil || strings.Contains(e.Error(), "坐标/方向") {
		t.Fatal("misleading error", e)
	}
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	for _, ns := range []string{"entities", "entity", "diagnostics", "hud", "scoreboard"} {
		if _, ok := L.GetGlobal(ns).(*lua.LTable); !ok {
			t.Fatal("missing namespace", ns)
		}
	}
}
func TestObservationHUDPriorityAndScoreCache(t *testing.T) {
	now := time.Now()
	items := map[string]yyHudEntry{"stats": {Player: "P", Kind: "actionbar", Priority: 1, Text: "stats", Expires: now.Add(time.Minute)}, "combat": {Player: "P", Kind: "actionbar", Priority: 100, Text: "health", Expires: now.Add(time.Second)}}
	if yyHudWinners(items, now)["P\x00actionbar"].Text != "health" || yyHudWinners(items, now.Add(2*time.Second))["P\x00actionbar"].Text != "stats" {
		t.Fatal("HUD arbitration")
	}
	b := senseBot(t)
	yySenseHandle(b, &packet.SetScore{Entries: []protocol.ScoreboardEntry{{EntryID: 1, ObjectiveName: "coins", IdentityType: protocol.ScoreboardIdentityFakePlayer, DisplayName: "P", Score: 17}}})
	if obsCall(t, b, "scoreboard.get", "coins", "P")["value"] != float64(17) {
		t.Fatal("score cache")
	}
	yySenseHandle(b, &packet.RemoveObjective{ObjectiveName: "coins"})
	if obsCall(t, b, "scoreboard.get", "coins", "P")["known"] != false {
		t.Fatal("removed score survived")
	}
	v := obsCall(t, b, "diagnostics.stats")
	if _, e := json.Marshal(v); e != nil {
		t.Fatal(e)
	}
}

func TestObservationActorHurtBeforeHealthAndWithoutHealth(t *testing.T) {
	b := senseBot(t)
	// 显示规则：15格内且攻击行为后3秒内的受伤才推送事件。
	yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 42, Position: mgl32.Vec3{0, 64, 0}})
	yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 90, Position: mgl32.Vec3{2, 64, 0}})
	senseHealth(b, 90, 40)
	yySenseHandle(b, &packet.Animate{EntityRuntimeID: 90, ActionType: packet.AnimateActionSwingArm})
	yySenseHandle(b, &packet.ActorEvent{EntityRuntimeID: 90, EventType: packet.ActorEventHurt})
	pending := yySenseFor(b).Actors[90].PendingHurtID
	senseHealth(b, 90, 35)
	time.Sleep(70 * time.Millisecond)
	s := yySenseFor(b)
	s.Lock()
	count := 0
	for _, e := range s.Events {
		if e["type"] == "hurt" {
			count++
			if e["event_id"] != pending || e["damage_known"] != true {
				t.Fatal(e)
			}
		}
	}
	s.Unlock()
	if count != 1 {
		t.Fatal("duplicate correlated damage", count)
	}
	yySenseHandle(b, &packet.MovePlayer{EntityRuntimeID: 91, Position: mgl32.Vec3{3, 64, 0}})
	yySenseHandle(b, &packet.ActorEvent{EntityRuntimeID: 91, EventType: packet.ActorEventHurt})
	time.Sleep(70 * time.Millisecond)
	s.Lock()
	defer s.Unlock()
	found := false
	for _, e := range s.Events {
		if e["type"] == "hurt" && e["runtime_id"] == "91" {
			found = true
			if e["damage_known"] != false || e["attacker_known"] != false {
				t.Fatal("invented unknown damage", e)
			}
		}
	}
	if !found {
		t.Fatal("ActorEvent-only hurt lost")
	}
}
