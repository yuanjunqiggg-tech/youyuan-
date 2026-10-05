package main

// Protocol observations are session scoped. Missing data is unknown, never full health.
import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

type yyActorObservation struct {
	PositionSource string     `json:"position_source"`
	HealthSource   string     `json:"health_source"`
	RuntimeID      uint64     `json:"runtime_id"`
	UniqueID       int64      `json:"unique_id"`
	Name           string     `json:"name"`
	EntityType     string     `json:"entity_type"`
	Item           any        `json:"item,omitempty"`
	Position       mgl32.Vec3 `json:"position"`
	PositionKnown  bool       `json:"position_known"`
	PositionTime   time.Time  `json:"position_time"`
	Dimension      int32      `json:"dimension"`
	Health         *float32   `json:"health"`
	MaxHealth      *float32   `json:"max_health"`
	HealthTime     time.Time  `json:"health_time"`
	Dead           *bool      `json:"dead"`
	Present        bool       `json:"present"`
	Source         string     `json:"source"`
	LastSeen       time.Time  `json:"last_seen"`
}
type yySenseState struct {
	sync.Mutex
	Actors  map[uint64]*yyActorObservation
	Blocks  map[string]map[string]any
	Events  []map[string]any
	Seq     uint64
	Capture yyCaptureState
}

var yySenses = struct {
	sync.Mutex
	Items map[*BotManager]*yySenseState
}{Items: map[*BotManager]*yySenseState{}}

func yySenseFor(bm *BotManager) *yySenseState {
	yySenses.Lock()
	defer yySenses.Unlock()
	s := yySenses.Items[bm]
	if s == nil {
		s = &yySenseState{Actors: map[uint64]*yyActorObservation{}, Blocks: map[string]map[string]any{}, Events: []map[string]any{}}
		yySenses.Items[bm] = s
	}
	return s
}
func yySenseReset(bm *BotManager) {
	if bm == nil {
		return
	}
	bm.mu.Lock()
	conn, id, eid, display, dimension := bm.conn, bm.runtimeID, bm.eid, bm.display, bm.dimension
	bm.mu.Unlock()
	s := yySenseFor(bm)
	s.Lock()
	s.Actors = map[uint64]*yyActorObservation{}
	s.Blocks = map[string]map[string]any{}
	s.Events = []map[string]any{}
	if conn != nil {
		now := time.Now()
		pos := conn.GameData().PlayerPosition
		s.Actors[id] = &yyActorObservation{RuntimeID: id, UniqueID: eid, Name: display, EntityType: "minecraft:player", Position: pos, PositionKnown: yyFiniteVec(pos), PositionTime: now, Dimension: dimension, Present: true, Source: "StartGame", PositionSource: "StartGame", LastSeen: now}
	}
	s.Unlock()
}
func (s *yySenseState) actor(id uint64, now time.Time) *yyActorObservation {
	a := s.Actors[id]
	if a == nil {
		if len(s.Actors) >= 2048 {
			var oldest uint64
			var at time.Time
			for k, v := range s.Actors {
				if at.IsZero() || v.LastSeen.Before(at) {
					oldest, at = k, v.LastSeen
				}
			}
			delete(s.Actors, oldest)
		}
		a = &yyActorObservation{RuntimeID: id, Present: true}
		s.Actors[id] = a
	}
	a.LastSeen = now
	return a
}
func yySenseHandle(bm *BotManager, pk packet.Packet) {
	if bm == nil {
		return
	}
	bm.mu.Lock()
	if p, ok := pk.(*packet.ChangeDimension); ok {
		bm.dimension = p.Dimension
	}
	self, display, dimension := bm.runtimeID, bm.display, bm.dimension
	bm.mu.Unlock()
	s := yySenseFor(bm)
	s.Lock()
	now := time.Now()
	out := []map[string]any{}
	source := yyPacketName(pk)
	emit := func(kind string, a *yyActorObservation, extra map[string]any) {
		e := map[string]any{"type": kind, "runtime_id": strconv.FormatUint(a.RuntimeID, 10), "player": a.Name, "entity_type": a.EntityType, "dimension": a.Dimension, "time": now.Format(time.RFC3339Nano), "source": source}
		for k, v := range extra {
			e[k] = v
		}
		out = append(out, e)
	}
	dead := func(a *yyActorObservation) {
		if a.Dead != nil && *a.Dead {
			return
		}
		v := true
		a.Dead = &v
		emit("death", a, map[string]any{"cause_known": false})
	}
	health := func(a *yyActorObservation, h, max float32, hasMax bool) {
		if math.IsNaN(float64(h)) || math.IsInf(float64(h), 0) {
			return
		}
		var old any
		changed := a.Health == nil || *a.Health != h
		if a.Health != nil {
			old = *a.Health
		}
		a.Health = &h
		if hasMax {
			a.MaxHealth = &max
		}
		a.HealthTime = now
		a.HealthSource = source
		a.Source = source
		if changed {
			emit("health", a, map[string]any{"previous": old, "health": h, "max_health": a.MaxHealth})
			if n, ok := old.(float32); ok && h < n {
				emit("hurt", a, map[string]any{"damage": n - h, "cause_known": false})
			}
		}
		if h <= 0 {
			dead(a)
		} else {
			v := false
			a.Dead = &v
		}
	}
	move := func(a *yyActorObservation, pos mgl32.Vec3) {
		if !yyFiniteVec(pos) {
			return
		}
		old := a.Position
		known := a.PositionKnown
		a.Position = pos
		a.PositionKnown = true
		a.PositionTime = now
		a.PositionSource = source
		a.Dimension = dimension
		a.Source = source
		a.Present = true
		if !known || old != pos {
			emit("move", a, map[string]any{"position": pos, "previous_known": known, "previous": old})
		}
	}
	switch p := pk.(type) {
	case *packet.AddPlayer:
		a := s.actor(p.EntityRuntimeID, now)
		a.Name = p.Username
		a.UniqueID = p.AbilityData.EntityUniqueID
		a.EntityType = "minecraft:player"
		a.Dead = nil
		move(a, p.Position)
		emit("entity_spawn", a, map[string]any{"position": p.Position, "fields": packetToMap(p)})
	case *packet.AddActor:
		a := s.actor(p.EntityRuntimeID, now)
		a.UniqueID = p.EntityUniqueID
		a.EntityType = p.EntityType
		a.Dead = nil
		move(a, p.Position)
		for _, v := range p.Attributes {
			if v.Name == "minecraft:health" {
				health(a, v.Value, v.Max, true)
			}
		}
		emit("entity_spawn", a, map[string]any{"position": p.Position, "fields": packetToMap(p)})
	case *packet.AddItemActor:
		a := s.actor(p.EntityRuntimeID, now)
		a.UniqueID = p.EntityUniqueID
		a.EntityType = "minecraft:item"
		a.Item = packetToMap(p.Item)
		move(a, p.Position)
		emit("entity_spawn", a, map[string]any{"position": p.Position, "item": a.Item, "fields": packetToMap(p)})
	case *packet.Animate:
		if p.ActionType == packet.AnimateActionSwingArm {
			emit("swing", s.actor(p.EntityRuntimeID, now), map[string]any{"action_type": p.ActionType})
		}
	case *packet.TakeItemActor:
		extra := map[string]any{"item_runtime_id": strconv.FormatUint(p.ItemEntityRuntimeID, 10), "taker_runtime_id": strconv.FormatUint(p.TakerEntityRuntimeID, 10), "item_known": false}
		if item := s.Actors[p.ItemEntityRuntimeID]; item != nil {
			extra["item"] = item.Item
			extra["item_known"] = item.Item != nil
			extra["position_known"] = item.PositionKnown
			if item.PositionKnown {
				extra["position"] = item.Position
			}
			item.Present = false
		}
		emit("item_pickup", s.actor(p.TakerEntityRuntimeID, now), extra)
	case *packet.UpdateAttributes:
		a := s.actor(p.EntityRuntimeID, now)
		if p.EntityRuntimeID == self {
			a.Name = display
			a.EntityType = "minecraft:player"
		}
		for _, v := range p.Attributes {
			if v.Name == "minecraft:health" {
				health(a, v.Value, v.Max, true)
			}
		}
	case *packet.SetHealth:
		a := s.actor(self, now)
		a.Name = display
		health(a, float32(p.Health), 0, false)
	case *packet.ActorEvent:
		a := s.actor(p.EntityRuntimeID, now)
		if p.EntityRuntimeID == self {
			a.Name = display
		}
		if p.EventType == packet.ActorEventDeath || p.EventType == packet.ActorEventInstantDeath {
			dead(a)
		} else if p.EventType == packet.ActorEventHurt {
			emit("hurt", a, map[string]any{"damage_known": false, "cause_known": false})
		}
	case *packet.MovePlayer:
		a := s.actor(p.EntityRuntimeID, now)
		if p.EntityRuntimeID == self {
			a.Name = display
			a.EntityType = "minecraft:player"
		}
		move(a, p.Position)
	case *packet.MoveActorAbsolute:
		move(s.actor(p.EntityRuntimeID, now), p.Position)
	case *packet.MoveActorDelta:
		a := s.actor(p.EntityRuntimeID, now)
		pos := a.Position
		if !a.PositionKnown && p.Flags&(packet.MoveActorDeltaFlagHasX|packet.MoveActorDeltaFlagHasY|packet.MoveActorDeltaFlagHasZ) != (packet.MoveActorDeltaFlagHasX|packet.MoveActorDeltaFlagHasY|packet.MoveActorDeltaFlagHasZ) {
			break
		}
		for i, flag := range []uint16{packet.MoveActorDeltaFlagHasX, packet.MoveActorDeltaFlagHasY, packet.MoveActorDeltaFlagHasZ} {
			if p.Flags&flag != 0 {
				pos[i] = p.Position[i]
			}
		}
		move(a, pos)
	case *packet.Respawn:
		if p.State != packet.RespawnStateReadyToSpawn {
			break
		}
		a := s.actor(p.EntityRuntimeID, now)
		if p.EntityRuntimeID == self {
			a.Name = display
		}
		v := false
		a.Dead = &v
		a.Health = nil
		a.MaxHealth = nil
		a.HealthTime = time.Time{}
		move(a, p.Position)
		emit("respawn", a, map[string]any{"position": p.Position})
	case *packet.RemoveActor:
		for _, a := range s.Actors {
			if a.UniqueID == p.EntityUniqueID {
				a.Present = false
				emit("entity_remove", a, map[string]any{"unique_id": strconv.FormatInt(p.EntityUniqueID, 10), "death_confirmed": false})
			}
		}
	case *packet.ChangeDimension:
		for _, a := range s.Actors {
			a.PositionKnown = false
		}
		a := s.actor(self, now)
		dimension = p.Dimension
		a.Name = display
		move(a, p.Position)
	case *packet.UpdateBlock:
		key := yyBlockKey(dimension, p.Position, p.Layer)
		old := s.Blocks[key]
		if len(s.Blocks) >= 4096 && old == nil {
			s.Blocks = map[string]map[string]any{}
		}
		v := map[string]any{"known": true, "dimension": dimension, "position": p.Position, "layer": p.Layer, "runtime_id": p.NewBlockRuntimeID, "time": now.Format(time.RFC3339Nano), "source": source}
		s.Blocks[key] = v
		out = append(out, map[string]any{"type": "block_change", "time": now.Format(time.RFC3339Nano), "source": source, "block": v, "previous": old, "cause_known": false})
	}
	for _, e := range out {
		s.Seq++
		e["sequence"] = s.Seq
		s.Events = append(s.Events, e)
	}
	if len(s.Events) > 512 {
		s.Events = s.Events[len(s.Events)-512:]
	}
	s.Unlock()
	for _, e := range out {
		yyRuntimeEmit(bm, e)
		if e["type"] == "death" {
			yyQQAssistantEvent(bm, e)
		}
		yySenseLuaEmit(bm, e)
	}
}
func yyFiniteVec(v mgl32.Vec3) bool {
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
	}
	return true
}
func yyBlockKey(d int32, p protocol.BlockPos, layer uint32) string {
	return fmt.Sprintf("%d:%d:%d:%d:%d", d, p[0], p[1], p[2], layer)
}
func yySenseActor(bm *BotManager, target string) map[string]any {
	if bm == nil {
		return map[string]any{"known": false, "connected": false, "reason": "游戏账号未连接"}
	}
	bm.mu.Lock()
	self, display := bm.runtimeID, bm.display
	connected := bm.connected
	bm.mu.Unlock()
	s := yySenseFor(bm)
	s.Lock()
	defer s.Unlock()
	var a *yyActorObservation
	if target == "" || target == "self" || target == "@s" || target == display {
		a = s.Actors[self]
	} else {
		if id, e := strconv.ParseUint(target, 10, 64); e == nil {
			a = s.Actors[id]
		}
		if a == nil {
			for _, v := range s.Actors {
				if v.Name == target {
					a = v
					break
				}
			}
		}
	}
	if a == nil {
		return map[string]any{"known": false, "connected": connected, "reason": "当前会话未观测到目标", "target": target}
	}
	age := time.Since(a.LastSeen).Milliseconds()
	positionAge := time.Since(a.PositionTime).Milliseconds()
	healthAge := time.Since(a.HealthTime).Milliseconds()
	return map[string]any{"known": true, "connected": connected, "runtime_id": strconv.FormatUint(a.RuntimeID, 10), "unique_id": strconv.FormatInt(a.UniqueID, 10), "name": a.Name, "entity_type": a.EntityType, "position": a.Position, "position_known": a.PositionKnown, "position_time": a.PositionTime, "position_source": a.PositionSource, "position_age_ms": positionAge, "position_stale": !connected || !a.PositionKnown || positionAge > 15000, "dimension": a.Dimension, "health": a.Health, "max_health": a.MaxHealth, "health_known": a.Health != nil, "health_time": a.HealthTime, "health_source": a.HealthSource, "health_age_ms": healthAge, "health_stale": !connected || a.Health == nil || healthAge > 15000, "dead": a.Dead, "present": a.Present, "source": a.Source, "last_seen": a.LastSeen, "age_ms": age, "stale": !connected || age > 15000}
}
func yySenseSnapshot(bm *BotManager) map[string]any {
	actors := []map[string]any{}
	if bm != nil {
		s := yySenseFor(bm)
		s.Lock()
		ids := []uint64{}
		for id := range s.Actors {
			ids = append(ids, id)
		}
		s.Unlock()
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			actors = append(actors, yySenseActor(bm, strconv.FormatUint(id, 10)))
		}
	}
	bot := yySenseActor(bm, "self")
	if bm != nil {
		bm.mu.Lock()
		if bm.connected {
			bot["game_mode"] = bm.gameMode
		}
		bm.mu.Unlock()
	}
	return map[string]any{"connected": bm != nil && bm.IsConnected(), "bot": bot, "actors": actors, "coverage": "only packets delivered to this bot; no global player health guarantee", "time": time.Now().Format(time.RFC3339Nano)}
}
