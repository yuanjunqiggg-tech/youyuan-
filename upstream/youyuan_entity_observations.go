package main

import (
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/go-gl/mathgl/mgl32"
	lua "github.com/yuin/gopher-lua"
)

type yyObservedItem struct {
	Item   protocol.ItemInstance
	At     time.Time
	Source string
}
type yyObservedAttribute struct {
	Value, Min, Max float32
	At              time.Time
	Source          string
}
type yyObservedIdentity struct {
	UUID, XUID, Name string
	UniqueID         int64
	Online           bool
}

var yyObservationEpoch = time.Now()
var yyObservationHandlerSeq atomic.Uint64
var yyEntityDocs = map[string]string{
	"entities.get":              "runtime_id十进制字符串或玩家名；服务器观测快照，含身份/位置/health结构/attributes/装备/视角/present/stale。未收到为known=false，不扫描全世界。",
	"entity.health":             "target=self或runtime_id字符串；current/max/previous/damage/known/max_known/stale/updated_at/source。最大生命未知为null，绝不假定20。",
	"entities.health":           "同entity.health，读取服务器UpdateAttributes/SetHealth缓存，不发送查询命令。",
	"entities.attributes":       "target；返回服务器同步的属性字典，value/min/max/updated_at/source；armor仅服务器同步minecraft:armor时已知，不按穿戴猜护甲点数。",
	"player.equipment":          "name或runtime_id；head/chest/legs/feet/main_hand/off_hand/body，每槽含known/item_id/item_id_known/network_id/count/enchantments/enchantments_known/nbt/received_at/age_ms/stale。物品名来自本会话ItemRegistry；附魔来自NBT。未知槽或未同步附魔不编造。expires_at=null：装备包没有物品失效时间。",
	"player.rotation":           "name或runtime_id；pitch/yaw/head_yaw各自known、updated_at、source/stale。MovePlayer及Absolute/Delta真实角度，旋转无位移也更新。",
	"diagnostics.entity":        "target；同entities.get，额外完整字段来源及过期信息。",
	"diagnostics.trace":         "event_id或sequence字符串；查询保留的事件及关联死亡/受伤详情；被覆盖返回known=false，不虚构历史。",
	"diagnostics.combat":        "target；返回保留事件中的伤害/血量/死亡/击杀，含同一event_id的补充证据。",
	"diagnostics.stats":         "当前账号解码包计数、最后包时间、实体数、事件缓冲及配置、断线/踢出诊断和整个宿主内存/GC；非单插件内存。",
	"diagnostics.configure":     "config?；宿主持久配置：event_queue/priority_queue/runtime_queue/async_tickets/event_history/actors各1..65536，stale_ms=1..86400000。事件队列变更在插件重载时生效，其余立即影响后续操作。不是命令CPS限速，也不是包种类上限。",
	"util.monotonicMs":          "无参数，进程启动以来单调时间毫秒，适合间隔计算；不是Unix时间。",
	"util.serverTick":           "返回known/tick/source/updated_at；仅观测UpdateAttributes的真实非零tick，不以本地50ms计时冒充服务器tick。",
	"events.onEntityHurt":       "handler(event)；真实掉血优先；ActorEventHurt等50ms关联UpdateAttributes，未同步血量时damage_known=false；hurt_signal仅诊断，不计作另一伤害。含target、damage/health/max_health、attacker_known=false（无可靠攻击者包时），event_id。",
	"entity.onDamage":           "同events.onEntityHurt；血量下降确认伤害，只有ActorEvent时数量未知，不能由最近玩家或挥臂猜攻击者。",
	"events.onHealthChanged":    "同onHealthChange，含previous及真实max，未知max=null。",
	"events.onPlayerKill":       "handler(event)；可信服务器death.attack翻译包提供击杀者且关联已知玩家，唯一death event_id，仅触发一次；同次death_details补充不重复计分。",
	"events.onPlayerRespawn":    "handler(event)；与session_join独立，死亡到存活/RespawnReady的生命周期去重。",
	"events.onDimensionChange":  "handler(event)；服务器ChangeDimension，旧维度实体数据标为过期。",
	"events.onSessionJoin":      "handler(event)；真实PlayerList新增条目，不能把AddPlayer视距出现当进服。服务器不广播PlayerList则无法直接观测。",
	"events.onSessionLeave":     "handler(event)；真实PlayerList移除条目，不把RemoveActor离开视距当退服。",
	"events.onEquipmentChanged": "handler(event)；服务器装备包即时缓存更新，含已更新槽位，不同步查询玩家背包。",
	"events.onRotation":         "handler(event)；服务器角度字段变化。旋转包即使位置不变也刷新时间。",
}

func init() {
	yyEntityDocs["player.inventory"] = "target=self,window_id=0；读取本机器人实际收到的InventoryContent/InventorySlot，不做同步背包查询。其他玩家背包服务器通常不广播，返回known=false；不把装备冒充整个背包。"
	yyEntityDocs["entities.nearby"] = "position=[x,y,z],radius=32,dimension?；已观测、仍存在且位置未过期的实体；不查询不可见区域。位置省略或self时使用机器人实际位置。"
	for m, d := range yyEntityDocs {
		yyExtraMethods[m] = d
	}
	for m, k := range map[string]string{"events.onEntityHurt": "hurt", "entity.onDamage": "hurt", "events.onHealthChanged": "health", "events.onPlayerKill": "player_kill", "events.onPlayerRespawn": "player_respawn", "events.onDimensionChange": "dimension_change", "events.onSessionJoin": "session_join", "events.onSessionLeave": "session_leave", "events.onEquipmentChanged": "equipment", "events.onRotation": "rotation"} {
		yySenseEvents[m] = k
	}
	yyExtraMethods["events.onHurt"] = "handler(event)；血量下降优先；ActorEvent等50ms关联属性包，否则提供damage_known=false提示；hurt_signal仅供诊断；未知攻击者attacker_known=false。"
}
func yyObservationAge(at time.Time) any {
	if at.IsZero() {
		return nil
	}
	return time.Since(at).Milliseconds()
}
func yyObservationStale(at time.Time, present, connected bool) bool {
	return !present || !connected || at.IsZero() || time.Since(at) > time.Duration(yyObservationSettings().StaleMS)*time.Millisecond
}
func yyFiniteFloat(n float32) bool { return !math.IsNaN(float64(n)) && !math.IsInf(float64(n), 0) }
func yyObserveRotation(a *yyActorObservation, rot [3]float32, flags [3]bool, now time.Time, source string) bool {
	changed := false
	for i, v := range rot {
		if flags[i] && yyFiniteFloat(v) {
			changed = changed || !a.RotationKnown[i] || a.Rotation[i] != v
			a.Rotation[i] = v
			a.RotationKnown[i] = true
			a.RotationTime[i] = now
			a.RotationSource[i] = source
		}
	}
	return changed
}
func yyRotationSnapshot(a *yyActorObservation, connected bool) map[string]any {
	v := map[string]any{"known": a.RotationKnown[0] && a.RotationKnown[1], "runtime_id": strconv.FormatUint(a.RuntimeID, 10)}
	for i, k := range []string{"pitch", "yaw", "head_yaw"} {
		var n any
		if a.RotationKnown[i] {
			n = a.Rotation[i]
		}
		v[k] = n
		v[k+"_known"] = a.RotationKnown[i]
		v[k+"_updated_at"] = a.RotationTime[i]
		v[k+"_source"] = a.RotationSource[i]
		v[k+"_stale"] = yyObservationStale(a.RotationTime[i], a.Present, connected)
	}
	v["updated_at"] = a.RotationTime[0]
	v["stale"] = v["pitch_stale"] == true || v["yaw_stale"] == true
	return v
}
func yyHealthSnapshot(a *yyActorObservation, connected bool) map[string]any {
	var damage any
	if a.PreviousHealth != nil && a.Health != nil {
		damage = float32(math.Max(0, float64(*a.PreviousHealth-*a.Health)))
	}
	out := map[string]any{"known": a.Health != nil, "current": a.Health, "max": a.MaxHealth, "max_known": a.MaxHealth != nil, "previous": a.PreviousHealth, "damage": damage, "updated_at": a.HealthTime, "age_ms": yyObservationAge(a.HealthTime), "stale": yyObservationStale(a.HealthTime, a.Present, connected), "source": a.HealthSource, "runtime_id": strconv.FormatUint(a.RuntimeID, 10)}
	// G01 asks the health snapshot to carry armor. It is only known when the
	// server syncs minecraft:armor; armor points are never inferred from what the
	// entity appears to wear, so unknown stays unknown.
	out["armor"] = nil
	out["armor_known"] = false
	if x, ok := a.Attributes["minecraft:armor"]; ok {
		out["armor"] = x.Value
		out["armor_known"] = true
		out["armor_at"] = x.At
		out["armor_age_ms"] = yyObservationAge(x.At)
		out["armor_stale"] = yyObservationStale(x.At, a.Present, connected)
		out["armor_source"] = x.Source
	}
	return out
}
func yyItemSnapshot(s *yySenseState, it yyObservedItem, present, connected bool) map[string]any {
	item := it.Item.Stack
	name, ok := s.ItemNames[item.NetworkID]
	if item.NetworkID == 0 {
		name = "minecraft:air"
		ok = true
	}
	var ench any
	var hasEnchant bool
	for _, key := range []string{"ench", "Enchantments"} {
		if x, exists := item.NBTData[key]; exists {
			ench = x
			hasEnchant = true
			break
		}
	}
	var itemID any
	if ok {
		itemID = name
	}
	return map[string]any{"known": true, "item_id": itemID, "item_id_known": ok, "network_id": item.NetworkID, "stack_network_id": it.Item.StackNetworkID, "metadata": item.MetadataValue, "count": item.Count, "enchantments": ench, "enchantments_known": hasEnchant, "nbt": item.NBTData, "received_at": it.At, "age_ms": yyObservationAge(it.At), "source": it.Source, "stale": yyObservationStale(it.At, present, connected), "expires_at": nil, "expires_at_known": false}
}
func yyEquipmentSnapshot(s *yySenseState, a *yyActorObservation, connected bool) map[string]any {
	v := map[string]any{"runtime_id": strconv.FormatUint(a.RuntimeID, 10), "name": a.Name}
	for _, slot := range []string{"head", "chest", "legs", "feet", "main_hand", "off_hand", "body"} {
		if it, ok := a.Equipment[slot]; ok {
			v[slot] = yyItemSnapshot(s, it, a.Present, connected)
		} else {
			v[slot] = map[string]any{"known": false, "item_id": nil, "stale": true}
		}
	}
	return v
}
func yyAttributeSnapshot(a *yyActorObservation) map[string]any {
	v := map[string]any{}
	for k, x := range a.Attributes {
		v[k] = map[string]any{"value": x.Value, "min": x.Min, "max": x.Max, "updated_at": x.At, "source": x.Source}
	}
	return v
}
func yySenseFind(s *yySenseState, target string, self uint64) *yyActorObservation {
	if target == "" || target == "self" || target == "@s" {
		return s.Actors[self]
	}
	if id, e := strconv.ParseUint(target, 10, 64); e == nil {
		return s.Actors[id]
	}
	var newest *yyActorObservation
	for _, a := range s.Actors {
		if a.Name == target && (newest == nil || a.LastSeen.After(newest.LastSeen)) {
			newest = a
		}
	}
	return newest
}
func yyObservationTarget(args []any) (string, error) {
	if len(args) == 0 {
		return "self", nil
	}
	if s, ok := args[0].(string); ok {
		return s, nil
	}
	if n, ok := args[0].(float64); ok && n >= 0 && n == math.Trunc(n) && n < 1<<53 {
		return strconv.FormatUint(uint64(n), 10), nil
	}
	return "", fmt.Errorf("target需玩家名或十进制字符串runtime_id；大ID不能用浮点数")
}
func yyEntityCall(bm *BotManager, method string, args []any) (any, error) {
	if method == "entities.nearby" {
		return yyEntitiesNearby(bm, args)
	}
	if method == "diagnostics.configure" {
		return yyConfigureObservationSettings(args)
	}
	if method == "util.monotonicMs" {
		return float64(time.Since(yyObservationEpoch).Nanoseconds()) / 1e6, nil
	}
	s := yySenseFor(bm)
	connected := bm != nil && bm.IsConnected()
	var self uint64
	if bm != nil {
		bm.mu.Lock()
		self = bm.runtimeID
		bm.mu.Unlock()
	}
	target, e := yyObservationTarget(args)
	if e != nil && method != "diagnostics.stats" && method != "util.serverTick" {
		return nil, e
	}
	s.Lock()
	defer s.Unlock()
	if method == "util.serverTick" {
		return map[string]any{"known": !s.ServerTickAt.IsZero(), "tick": s.ServerTick, "source": "UpdateAttributes", "updated_at": s.ServerTickAt, "stale": yyObservationStale(s.ServerTickAt, true, connected)}, nil
	}
	if method == "diagnostics.stats" {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		return yyObservationCopy(map[string]any{"display_logs": yyDisplayLogStats(), "connected": connected, "decoded_packets": s.PacketCount, "packet_counts": s.PacketCounts, "last_packet_at": s.LastPacketAt, "actors": len(s.Actors), "events": len(s.Events), "settings": yyObservationSettings(), "connection_history": s.ConnectionHistory, "host_heap_bytes": mem.HeapAlloc, "host_gc_count": mem.NumGC, "host_gc_pause_total_ns": mem.PauseTotalNs, "scope": "this account observations; memory for entire host"}), nil
	}
	if method == "diagnostics.trace" {
		rows := []map[string]any{}
		for _, x := range s.Events {
			if x["event_id"] == target || fmt.Sprint(x["sequence"]) == target {
				rows = append(rows, x)
			}
		}
		return yyObservationCopy(map[string]any{"known": len(rows) > 0, "events": rows}), nil
	}
	a := yySenseFind(s, target, self)
	if a == nil {
		return map[string]any{"known": false, "stale": true, "target": target, "reason": "服务器未同步目标"}, nil
	}
	switch method {
	case "player.inventory":
		if a.RuntimeID != self {
			return map[string]any{"known": false, "reason": "服务器未广播其他玩家完整背包"}, nil
		}
		window, e := yyArgNumber(args, 1, 0)
		if e != nil || window < 0 || window > math.MaxUint32 || window != math.Trunc(window) {
			return nil, fmt.Errorf("window_id需uint32")
		}
		slots, known := s.Inventories[uint32(window)]
		out := map[string]any{}
		for slot, it := range slots {
			out[strconv.FormatUint(uint64(slot), 10)] = yyItemSnapshot(s, it, a.Present, connected)
		}
		return yyObservationCopy(map[string]any{"known": known, "window_id": window, "slots": out}), nil
	case "entity.health", "entities.health":
		return yyObservationCopy(yyHealthSnapshot(a, connected)), nil
	case "player.rotation":
		return yyRotationSnapshot(a, connected), nil
	case "player.equipment":
		return yyObservationCopy(yyEquipmentSnapshot(s, a, connected)), nil
	case "entities.attributes":
		return yyAttributeSnapshot(a), nil
	case "diagnostics.combat":
		rows := []map[string]any{}
		for _, x := range s.Events {
			if x["runtime_id"] == strconv.FormatUint(a.RuntimeID, 10) {
				switch x["type"] {
				case "health", "hurt", "hurt_signal", "death", "death_details", "player_kill":
					rows = append(rows, x)
				}
			}
		}
		return yyObservationCopy(map[string]any{"known": true, "events": rows}), nil
	}
	return nil, fmt.Errorf("未知实体方法%s", method)
}
func yyObservationCopy(v any) any {
	b, e := json.Marshal(v)
	if e != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}
func yyObservationCopyRows(v []map[string]any) []map[string]any {
	b, e := json.Marshal(v)
	if e != nil {
		return []map[string]any{}
	}
	var out []map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}
func yySenseLuaHandler(L *lua.LState, index int) string {
	if fn, ok := L.Get(index).(*lua.LFunction); ok {
		name := fmt.Sprintf("__yy_observation_callback_%d", yyObservationHandlerSeq.Add(1))
		L.SetGlobal(name, fn)
		return name
	}
	return L.CheckString(index)
}
func yyConnectionDiagnostic(bm *BotManager, phase, reason string) {
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	yyConnectionAppendLocked(s, phase, reason, nil)
}

// yyConnectionAppendLocked records one connection event with the attempt it
// belongs to and the first error seen in that attempt. The closed event used to
// carry only connCloseErr ("EOF", "closed network connection"), so the log could
// not answer whether the attempt died on auth, on a kick, or on the network.
// Callers must already hold s.
func yyConnectionAppendLocked(s *yySenseState, phase, reason string, extra map[string]any) {
	if phase == "connect_attempt" {
		s.ConnectionAttempt++
		s.ConnectionFirstErr = ""
		s.ConnectionFirstWhen = ""
	}
	entry := map[string]any{"phase": phase, "reason": reason, "timestamp": time.Now().UnixMilli(), "attempt": s.ConnectionAttempt}
	for k, v := range extra {
		if _, exists := entry[k]; !exists {
			entry[k] = v
		}
	}
	if reason != "" && s.ConnectionFirstErr == "" {
		s.ConnectionFirstErr, s.ConnectionFirstWhen = reason, phase
	}
	if s.ConnectionFirstErr != "" {
		entry["first_error"] = s.ConnectionFirstErr
		entry["first_error_phase"] = s.ConnectionFirstWhen
	}
	s.ConnectionHistory = append(s.ConnectionHistory, entry)
	if len(s.ConnectionHistory) > 64 {
		s.ConnectionHistory = s.ConnectionHistory[len(s.ConnectionHistory)-64:]
	}
}
func yyObserveItem(s *yySenseState, a *yyActorObservation, slot string, item protocol.ItemInstance, at time.Time, source string) {
	if a.Equipment == nil {
		a.Equipment = map[string]yyObservedItem{}
	}
	a.Equipment[slot] = yyObservedItem{item, at, source}
}
func yyObserveAttribute(a *yyActorObservation, v protocol.AttributeValue, at time.Time, source string) {
	if !yyFiniteFloat(v.Value) || !yyFiniteFloat(v.Min) || !yyFiniteFloat(v.Max) {
		return
	}
	if a.Attributes == nil {
		a.Attributes = map[string]yyObservedAttribute{}
	}
	a.Attributes[v.Name] = yyObservedAttribute{v.Value, v.Min, v.Max, at, source}
}
func yyKnownPlayerName(name string) bool {
	return name != "" && !strings.Contains(name, ":") && !strings.HasPrefix(name, "[")
}

// Only history enrichment copies maps. Normal high-frequency dispatch needs no
// JSON round-trip and never modifies already published event maps.
func yyCloneObservationMap(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		if nested, ok := v.(map[string]any); ok {
			v = yyCloneObservationMap(nested)
		}
		out[k] = v
	}
	return out
}
func yyEntitiesNearby(bm *BotManager, args []any) (any, error) {
	self := yySenseActor(bm, "self")
	origin, ok := self["position"].(mgl32.Vec3)
	dimension := self["dimension"]
	if dimension == nil && bm != nil {
		bm.mu.Lock()
		dimension = bm.dimension
		bm.mu.Unlock()
	}
	if len(args) > 0 && args[0] != "self" {
		coords, ok := args[0].([]any)
		if !ok {
			return nil, fmt.Errorf("position需[x,y,z]或self")
		}
		var e error
		origin, e = yySenseVec(coords)
		if e != nil {
			return nil, e
		}
	} else if !ok || self["position_known"] != true {
		return nil, fmt.Errorf("机器人位置未知")
	}
	radius, e := yyArgNumber(args, 1, 32)
	if e != nil || radius < 0 || radius > 512 {
		return nil, fmt.Errorf("radius需0..512")
	}
	if len(args) > 2 {
		n, e := yyArgNumber(args, 2, 0)
		if e != nil || n != math.Trunc(n) || n < math.MinInt32 || n > math.MaxInt32 {
			return nil, fmt.Errorf("dimension需整数")
		}
		dimension = int32(n)
	}
	rows := []map[string]any{}
	for _, a := range yySenseSnapshot(bm)["actors"].([]map[string]any) {
		if a["present"] != true || a["position_stale"] == true || a["dimension"] != dimension {
			continue
		}
		distance := float64(a["position"].(mgl32.Vec3).Sub(origin).Len())
		if distance <= radius {
			a["distance"] = distance
			a["health"] = a["health_state"]
			rows = append(rows, a)
		}
	}
	return rows, nil
}

// ActorEvent has no reliable damage amount/attacker. Wait only for a matching
// attribute observation, then preserve the signal if no health packet arrives.
// This delay never affects the network reader or the fast command path.
func yyFlushActorHurt(bm *BotManager, s *yySenseState, a *yyActorObservation, id string) {
	s.Lock()
	if s.Actors[a.RuntimeID] != a || a.PendingHurtID != id {
		s.Unlock()
		return
	}
	a.PendingHurtTimer = nil
	a.PendingHurtID = ""
	a.LastHurtID = id
	a.LastHurtTime = time.Now()
	now := time.Now()
	// 推送前按用户规则复核：15格以内，且仍在攻击行为后3秒窗口内。
	self := uint64(0)
	if bm != nil {
		self = bm.runtimeID
	}
	me := s.Actors[self]
	near := a.RuntimeID == self
	if !near && me != nil && me.PositionKnown && a.PositionKnown && a.Dimension == me.Dimension {
		near = float64(a.Position.Sub(me.Position).Len()) <= yyHealthDisplayRadius
	}
	if !near || !combatRecent(s, now) {
		s.Unlock()
		return
	}
	e := map[string]any{"type": "hurt", "event_id": id, "runtime_id": strconv.FormatUint(a.RuntimeID, 10), "target_runtime_id": strconv.FormatUint(a.RuntimeID, 10), "player": a.Name, "entity_type": a.EntityType, "target": map[string]any{"runtime_id": strconv.FormatUint(a.RuntimeID, 10), "name": a.Name, "entity_type": a.EntityType}, "dimension": a.Dimension, "source": "ActorEvent", "damage": nil, "damage_known": false, "health": a.Health, "max_health": a.MaxHealth, "attacker": nil, "attacker_runtime_id": nil, "attacker_known": false, "cause_known": false, "time": now.Format(time.RFC3339Nano), "timestamp": now.UnixMilli()}
	s.Seq++
	e["sequence"] = s.Seq
	e["data"] = yyCloneObservationMap(e)
	s.Events = append(s.Events, e)
	if len(s.Events) > yyObservationSettings().EventHistory {
		s.Events = s.Events[len(s.Events)-yyObservationSettings().EventHistory:]
	}
	s.Unlock()
	yyPatrolObserve(bm, e)
	yyRuntimeEmit(bm, e)
	yySenseLuaEmit(bm, e)
}
