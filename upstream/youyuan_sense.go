package main

// Protocol observations are session scoped. Missing data is unknown, never full health.
import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

type yyActorObservation struct {
	UUID, XUID       string
	PreviousHealth   *float32
	Rotation         [3]float32
	RotationKnown    [3]bool
	RotationTime     [3]time.Time
	RotationSource   [3]string
	Equipment        map[string]yyObservedItem
	Attributes       map[string]yyObservedAttribute
	DeathID          string
	DeathTime        time.Time
	KillDelivered    bool
	RespawnDelivered bool
	LastHurtID       string
	LastHurtTime     time.Time
	PendingHurtID    string
	PendingHurtAt    time.Time
	PendingHurtTimer *time.Timer

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
	Killer         string     `json:"killer"`
	KillerKnown    bool       `json:"killer_known"`
}
type yySenseState struct {
	ModuleEvents    []map[string]any
	ModuleSeq       uint64
	ModuleKills     map[string]bool
	ModuleKillOrder []string
	Locations       map[int64]yyLocatorObservation
	Inventories     map[uint32]map[uint32]yyObservedItem
	sync.Mutex
	Actors              map[uint64]*yyActorObservation
	Blocks              map[string]map[string]any
	Events              []map[string]any
	ActionEvents        []map[string]any
	ActionEvictedSeq    uint64
	Seq                 uint64
	Session             string // Reconnect changes the cursor namespace.
	SessionBase         uint64
	Capture             yyCaptureState
	ItemNames           map[int32]string
	Identities          map[string]yyObservedIdentity
	PacketCount         uint64
	PacketCounts        map[string]uint64
	LastPacketAt        time.Time
	ServerTick          uint64
	ServerTickAt        time.Time
	ConnectionHistory   []map[string]any
	Scores              map[string]map[string]any
	ViewRequestMu       sync.Mutex
	RequestedRadius     int32
	RequestedRadiusAt   time.Time
	GrantedRadius       int32
	GrantedRadiusAt     time.Time
	GrantedRadiusSource string
	PublisherPosition   [3]int32
	PublisherRadius     uint32
	PublisherAt         time.Time
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
		s = &yySenseState{Scores: map[string]map[string]any{}, ItemNames: map[int32]string{}, Identities: map[string]yyObservedIdentity{}, PacketCounts: map[string]uint64{}, Actors: map[uint64]*yyActorObservation{}, Blocks: map[string]map[string]any{}, Events: []map[string]any{}}
		yySenses.Items[bm] = s
		s.Session = yyWorldSessionID()
	}
	return s
}
func yySenseReset(bm *BotManager) {
	yyObserverRPCReset(bm)
	if bm == nil {
		return
	}
	bm.mu.Lock()
	conn, id, eid, display := bm.conn, bm.runtimeID, bm.eid, bm.display
	bm.packetIngress = nil
	bm.mu.Unlock()
	s := yySenseFor(bm)
	s.Lock()
	for _, a := range s.Actors {
		if a.PendingHurtTimer != nil {
			a.PendingHurtTimer.Stop()
		}
	}
	s.Actors = map[uint64]*yyActorObservation{}
	s.Locations = map[int64]yyLocatorObservation{}
	s.Session = yyWorldSessionID()
	s.SessionBase = s.Seq
	s.Inventories = map[uint32]map[uint32]yyObservedItem{}
	s.Blocks = map[string]map[string]any{}
	s.Events = []map[string]any{}
	s.ActionEvents = nil
	s.ActionEvictedSeq = 0
	s.ModuleEvents = nil
	s.ModuleSeq = 0
	s.ModuleKills = nil
	s.ModuleKillOrder = nil
	s.ItemNames = map[int32]string{}
	s.Scores = map[string]map[string]any{}
	s.Identities = map[string]yyObservedIdentity{}
	s.ServerTick = 0
	s.ServerTickAt = time.Time{}
	s.RequestedRadiusAt, s.GrantedRadiusAt, s.PublisherAt = time.Time{}, time.Time{}, time.Time{}
	s.GrantedRadiusSource = ""
	s.PacketCount, s.PacketCounts, s.LastPacketAt = 0, map[string]uint64{}, time.Time{}
	s.ConnectionHistory = append(s.ConnectionHistory, map[string]any{"phase": "connected", "timestamp": time.Now().UnixMilli()})
	if len(s.ConnectionHistory) > 64 {
		s.ConnectionHistory = s.ConnectionHistory[len(s.ConnectionHistory)-64:]
	}
	if conn != nil {
		now := time.Now()
		gd := conn.GameData()
		if gd.ChunkRadius > 0 {
			s.GrantedRadius, s.GrantedRadiusAt, s.GrantedRadiusSource = gd.ChunkRadius, now, "login ChunkRadiusUpdated"
		}
		pos := gd.PlayerPosition
		for _, it := range gd.Items {
			s.ItemNames[int32(it.RuntimeID)] = it.Name
		}
		if name := conn.IdentityData().DisplayName; yyKnownPlayerName(name) {
			display = name
		} else if !yyKnownPlayerName(display) {
			display = ""
		}
		a := &yyActorObservation{RuntimeID: id, UniqueID: eid, UUID: conn.IdentityData().Identity, XUID: conn.IdentityData().XUID, Name: display, EntityType: "minecraft:player", Position: pos, PositionKnown: yyFiniteVec(pos), PositionTime: now, Dimension: gd.Dimension, Present: true, Source: "StartGame", PositionSource: "StartGame", LastSeen: now}
		yyObserveRotation(a, [3]float32{gd.Pitch, gd.Yaw, 0}, [3]bool{true, true, false}, now, "StartGame")
		s.Actors[id] = a
	}
	s.Unlock()
}
func (s *yySenseState) actor(id uint64, now time.Time) *yyActorObservation {
	a := s.Actors[id]
	if a == nil {
		if len(s.Actors) >= yyObservationSettings().Actors {
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
	self, display, dimension := bm.runtimeID, bm.display, bm.dimension
	func() {
		defer bm.mu.Unlock()
		if p, ok := pk.(*packet.ChangeDimension); ok {
			bm.dimension = p.Dimension
			dimension = p.Dimension
		}
	}()
	s := yySenseFor(bm)
	s.Lock()
	senseLocked := true
	defer func() {
		if senseLocked {
			s.Unlock()
		}
	}()
	now := time.Now()
	s.PacketCount++
	s.PacketCounts[yyPacketName(pk)]++
	s.LastPacketAt = now
	out := []map[string]any{}
	source := yyPacketName(pk)
	var deathDetails map[string]any
	emit := func(kind string, a *yyActorObservation, extra map[string]any) {
		e := map[string]any{"type": kind, "runtime_id": strconv.FormatUint(a.RuntimeID, 10), "player": a.Name, "entity_type": a.EntityType, "dimension": a.Dimension, "time": now.Format(time.RFC3339Nano), "source": source, "timestamp": now.UnixMilli()}
		for k, v := range extra {
			e[k] = v
		}
		if _, exists := e["event_id"]; !exists {
			e["event_id"] = fmt.Sprintf("%d:%d:%d", now.UnixNano(), a.RuntimeID, s.Seq+uint64(len(out))+1)
		}
		e["uuid"] = a.UUID
		e["xuid"] = a.XUID
		if _, exists := e["position"]; !exists && a.PositionKnown {
			e["position"], e["position_known"], e["position_timestamp"], e["position_stale"] = a.Position, true, a.PositionTime.UnixMilli(), yyObservationStale(a.PositionTime, a.Present, true)
		}
		if armor, known := a.Attributes["minecraft:armor"]; known {
			e["armor"], e["armor_known"] = armor.Value, true
		}
		out = append(out, e)
	}
	dead := func(a *yyActorObservation) {
		if a.Dead != nil && *a.Dead {
			return
		}
		v := true
		a.Dead = &v
		a.Killer = ""
		a.KillerKnown = false
		a.DeathID = fmt.Sprintf("death:%d:%d", now.UnixNano(), a.RuntimeID)
		a.DeathTime = now
		a.KillDelivered = false
		a.RespawnDelivered = false
		extra := map[string]any{"event_id": a.DeathID, "cause_known": false, "killer_known": false, "position_known": a.PositionKnown, "last_position": a.Position, "position_time": a.PositionTime.Format(time.RFC3339Nano)}
		for k, v := range deathDetails {
			if k == "event_id" {
				continue
			}
			extra[k] = v
		}
		if name, ok := extra["killer"].(string); ok {
			a.Killer = name
			a.KillerKnown = extra["killer_known"] == true
		}
		emit("death", a, extra)
	}
	health := func(a *yyActorObservation, h, max float32, hasMax bool) {
		if math.IsNaN(float64(h)) || math.IsInf(float64(h), 0) {
			return
		}
		var old any
		changed := a.Health == nil || *a.Health != h || (hasMax && yyFiniteFloat(max) && max > 0 && (a.MaxHealth == nil || *a.MaxHealth != max))
		if a.Health != nil {
			old = *a.Health
		}
		if changed {
			a.PreviousHealth = a.Health
		}
		a.Health = &h
		if hasMax && yyFiniteFloat(max) && max > 0 {
			a.MaxHealth = &max
		}
		a.HealthTime = now
		a.HealthSource = source
		a.Source = source
		if changed {
			emit("health", a, map[string]any{"previous": old, "health": h, "max_health": a.MaxHealth})
			if n, ok := old.(float32); ok && h < n {
				a.LastHurtID = fmt.Sprintf("hurt:%d:%d", now.UnixNano(), a.RuntimeID)
				if a.PendingHurtTimer != nil && now.Sub(a.PendingHurtAt) <= 50*time.Millisecond {
					a.PendingHurtTimer.Stop()
					a.LastHurtID = a.PendingHurtID
					a.PendingHurtTimer = nil
					a.PendingHurtID = ""
				}
				a.LastHurtTime = now
				emit("hurt", a, map[string]any{"event_id": a.LastHurtID, "target_runtime_id": strconv.FormatUint(a.RuntimeID, 10), "target": map[string]any{"runtime_id": strconv.FormatUint(a.RuntimeID, 10), "name": a.Name, "entity_type": a.EntityType}, "damage": n - h, "damage_known": true, "health": h, "max_health": a.MaxHealth, "attacker_runtime_id": nil, "attacker_known": false, "attacker": nil, "damage_source": nil, "cause_known": false})
			}
		}
		if h <= 0 {
			dead(a)
		} else {
			wasDead := a.Dead != nil && *a.Dead
			v := false
			a.Dead = &v
			if wasDead && !a.RespawnDelivered {
				a.RespawnDelivered = true
				emit("respawn", a, map[string]any{"death_event_id": a.DeathID, "position_known": a.PositionKnown})
			}
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
	case *packet.ChunkRadiusUpdated:
		s.GrantedRadius, s.GrantedRadiusAt, s.GrantedRadiusSource = p.ChunkRadius, now, source
	case *packet.NetworkChunkPublisherUpdate:
		s.PublisherPosition, s.PublisherRadius, s.PublisherAt = [3]int32(p.Position), p.Radius, now
	case *packet.SetScore:
		for _, entry := range p.Entries {
			key := fmt.Sprintf("%s:%d", entry.ObjectiveName, entry.EntryID)
			if p.ActionType == packet.ScoreboardActionRemove {
				delete(s.Scores, key)
				continue
			}
			name := entry.DisplayName
			if entry.IdentityType != protocol.ScoreboardIdentityFakePlayer {
				for _, a := range s.Actors {
					if a.UniqueID == entry.EntityUniqueID {
						name = a.Name
						break
					}
				}
			}
			s.Scores[key] = map[string]any{"known": true, "objective": entry.ObjectiveName, "entry_id": strconv.FormatInt(entry.EntryID, 10), "unique_id": strconv.FormatInt(entry.EntityUniqueID, 10), "player": name, "value": entry.Score, "source": source, "updated_at": now}
		}
	case *packet.RemoveObjective:
		for key, row := range s.Scores {
			if row["objective"] == p.ObjectiveName {
				delete(s.Scores, key)
			}
		}
	case *packet.ItemRegistry:
		for _, it := range p.Items {
			s.ItemNames[int32(it.RuntimeID)] = it.Name
		}
	case *packet.PlayerList:
		for _, it := range p.Entries {
			key := it.UUID.String()
			old, existed := s.Identities[key]
			if p.ActionType == packet.PlayerListActionAdd {
				s.Identities[key] = yyObservedIdentity{key, it.XUID, it.Username, it.EntityUniqueID, true}
				var actor *yyActorObservation
				for _, a := range s.Actors {
					if a.UUID == key || a.UniqueID == it.EntityUniqueID {
						a.UUID = key
						a.XUID = it.XUID
						a.Name = it.Username
						actor = a
						break
					}
				}
				if actor == nil {
					actor = &yyActorObservation{Name: it.Username, UUID: key, XUID: it.XUID, UniqueID: it.EntityUniqueID, EntityType: "minecraft:player"}
				}
				if !existed || !old.Online {
					emit("session_join", actor, map[string]any{"runtime_id_known": actor.RuntimeID != 0})
				}
			} else {
				if existed && old.Online {
					old.Online = false
					s.Identities[key] = old
					actor := &yyActorObservation{Name: old.Name, UUID: key, XUID: old.XUID, UniqueID: old.UniqueID, EntityType: "minecraft:player"}
					emit("session_leave", actor, map[string]any{"runtime_id_known": false})
				}
			}
		}
	case *packet.MobEquipment:
		a := s.actor(p.EntityRuntimeID, now)
		slot := "main_hand"
		if p.WindowID == protocol.WindowIDOffHand {
			slot = "off_hand"
		} else if p.WindowID != protocol.WindowIDInventory {
			break
		}
		yyObserveItem(s, a, slot, p.NewItem, now, source)
		emit("equipment", a, map[string]any{"slots": []string{slot}})
	case *packet.MobArmourEquipment:
		a := s.actor(p.EntityRuntimeID, now)
		for slot, it := range map[string]protocol.ItemInstance{"head": p.Helmet, "chest": p.Chestplate, "legs": p.Leggings, "feet": p.Boots, "body": p.Body} {
			yyObserveItem(s, a, slot, it, now, source)
		}
		emit("equipment", a, map[string]any{"slots": []string{"head", "chest", "legs", "feet", "body"}})
	case *packet.InventoryContent:
		slots := map[uint32]yyObservedItem{}
		for i, it := range p.Content {
			slots[uint32(i)] = yyObservedItem{it, now, source}
		}
		if s.Inventories == nil {
			s.Inventories = map[uint32]map[uint32]yyObservedItem{}
		}
		s.Inventories[p.WindowID] = slots
		a := s.actor(self, now)
		a.Name = display
		a.EntityType = "minecraft:player"
		for i, it := range p.Content {
			if p.WindowID == protocol.WindowIDArmour && i < 4 {
				yyObserveItem(s, a, []string{"head", "chest", "legs", "feet"}[i], it, now, source)
			} else if p.WindowID == protocol.WindowIDOffHand && i == 0 {
				yyObserveItem(s, a, "off_hand", it, now, source)
			}
		}
	case *packet.InventorySlot:
		if s.Inventories == nil {
			s.Inventories = map[uint32]map[uint32]yyObservedItem{}
		}
		if s.Inventories[p.WindowID] == nil {
			s.Inventories[p.WindowID] = map[uint32]yyObservedItem{}
		}
		s.Inventories[p.WindowID][p.Slot] = yyObservedItem{p.NewItem, now, source}
		a := s.actor(self, now)
		a.Name = display
		a.EntityType = "minecraft:player"
		if p.WindowID == protocol.WindowIDArmour && p.Slot < 4 {
			yyObserveItem(s, a, []string{"head", "chest", "legs", "feet"}[p.Slot], p.NewItem, now, source)
		} else if p.WindowID == protocol.WindowIDOffHand && p.Slot == 0 {
			yyObserveItem(s, a, "off_hand", p.NewItem, now, source)
		}
	case *packet.Disconnect:
		s.ConnectionHistory = append(s.ConnectionHistory, map[string]any{"phase": "server_disconnect", "reason_code": p.Reason, "reason": p.Message, "timestamp": now.UnixMilli()})
		if len(s.ConnectionHistory) > 64 {
			s.ConnectionHistory = s.ConnectionHistory[len(s.ConnectionHistory)-64:]
		}
	case *packet.Text:
		// Translation packets, not arbitrary chat text, provide authoritative
		// death causes. Only a known observed player may be named as the killer.
		if p.TextType != packet.TextTypeTranslation || !strings.HasPrefix(p.Message, "death.attack.") || len(p.Parameters) < 1 {
			break
		}
		var victim *yyActorObservation
		for _, a := range s.Actors {
			if a.EntityType == "minecraft:player" && a.Name == p.Parameters[0] {
				victim = a
				break
			}
		}
		if victim == nil {
			break
		}
		deathDetails = map[string]any{"event_id": victim.DeathID, "cause_known": true, "cause_key": p.Message, "killer_known": false, "position_known": victim.PositionKnown, "last_position": victim.Position}
		if len(p.Parameters) > 1 {
			for _, a := range s.Actors {
				if a.EntityType == "minecraft:player" && a.Name == p.Parameters[1] {
					deathDetails["killer"] = a.Name
					deathDetails["killer_runtime_id"] = strconv.FormatUint(a.RuntimeID, 10)
					deathDetails["killer_known"] = true
					break
				}
			}
		}
		if victim.Dead != nil && *victim.Dead {
			if name, ok := deathDetails["killer"].(string); ok {
				victim.Killer = name
				victim.KillerKnown = true
			}
			// The earlier RPC may arrive before the translated death message.
			// Enrich its queryable record, and emit a separate details event.
			for i := len(s.Events) - 1; i >= 0; i-- {
				e := s.Events[i]
				if e["type"] == "death" && e["player"] == victim.Name {
					if ts, ok := e["timestamp"].(int64); ok && now.UnixMilli()-ts <= 5000 {
						for k, v := range deathDetails {
							// Replacing history avoids mutating maps already queued to SDK callbacks.
							if k == "event_id" {
								continue
							}
							replacement := yyCloneObservationMap(e)
							replacement[k] = v
							if data, ok := replacement["data"].(map[string]any); ok {
								data[k] = v
							}
							e = replacement
							s.Events[i] = replacement
						}
					}
					break
				}
			}
			if time.Since(victim.DeathTime) <= 5*time.Second {
				emit("death_details", victim, deathDetails)
			}
		} else {
			dead(victim)
		}
		if victim.KillerKnown && !victim.KillDelivered && time.Since(victim.DeathTime) <= 5*time.Second {
			victim.KillDelivered = true
			emit("player_kill", victim, map[string]any{"event_id": victim.DeathID, "victim": victim.Name, "killer": victim.Killer, "killer_known": true, "killer_runtime_id": deathDetails["killer_runtime_id"], "cause_key": p.Message})
		}
	case *packet.PyRpc:
		out = append(out, yyModuleCombatEvents(s, p, now)...)
		uid, isDead, ok := yyDeathRPC(p.Value)
		if ok {
			for _, a := range s.Actors {
				if a.UniqueID != uid || a.EntityType != "minecraft:player" {
					continue
				}
				if isDead {
					dead(a)
				} else {
					wasDead := a.Dead != nil && *a.Dead
					v := false
					a.Dead = &v
					a.Health = nil
					a.PreviousHealth = nil
					delete(a.Attributes, "minecraft:health")
					a.MaxHealth = nil
					a.HealthTime = time.Time{}
					if wasDead && !a.RespawnDelivered {
						a.RespawnDelivered = true
						emit("respawn", a, map[string]any{"position_known": a.PositionKnown, "position": a.Position})
					}
				}
			}
		}

	case *packet.AddPlayer:
		if old := s.Actors[p.EntityRuntimeID]; old != nil && (!old.Present || old.UniqueID != p.AbilityData.EntityUniqueID) {
			delete(s.Actors, p.EntityRuntimeID)
		}
		a := s.actor(p.EntityRuntimeID, now)
		a.UUID = p.UUID.String()
		if identity, ok := s.Identities[a.UUID]; ok {
			a.XUID = identity.XUID
		}
		yyObserveRotation(a, [3]float32{p.Pitch, p.Yaw, p.HeadYaw}, [3]bool{true, true, true}, now, source)
		yyObserveItem(s, a, "main_hand", p.HeldItem, now, source)
		a.Name = p.Username
		a.UniqueID = p.AbilityData.EntityUniqueID
		a.EntityType = "minecraft:player"
		a.Dead = nil
		move(a, p.Position)
		emit("entity_spawn", a, map[string]any{"position": p.Position, "fields": packetToMap(p)})
	case *packet.AddActor:
		if old := s.Actors[p.EntityRuntimeID]; old != nil && (!old.Present || old.UniqueID != p.EntityUniqueID) {
			delete(s.Actors, p.EntityRuntimeID)
		}
		a := s.actor(p.EntityRuntimeID, now)
		yyObserveRotation(a, [3]float32{p.Pitch, p.Yaw, p.HeadYaw}, [3]bool{true, true, true}, now, source)
		a.UniqueID = p.EntityUniqueID
		a.EntityType = p.EntityType
		a.Dead = nil
		move(a, p.Position)
		for _, v := range p.Attributes {
			yyObserveAttribute(a, v, now, source)
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
			emit("swing", s.actor(p.EntityRuntimeID, now), map[string]any{"action_type": p.ActionType, "observed": true, "inferred": false, "source_scope": "server_synced_to_this_bot", "attack_hit_known": false})
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
		if p.Tick > 0 {
			s.ServerTick = p.Tick
			s.ServerTickAt = now
		}
		a := s.actor(p.EntityRuntimeID, now)
		if p.EntityRuntimeID == self {
			a.Name = display
			a.EntityType = "minecraft:player"
		}
		for _, v := range p.Attributes {
			yyObserveAttribute(a, v.AttributeValue, now, source)
			if v.Name == "minecraft:health" {
				health(a, v.Value, v.Max, true)
			}
		}
	case *packet.SetHealth:
		a := s.actor(self, now)
		a.Name = display
		a.EntityType = "minecraft:player"
		health(a, float32(p.Health), 0, false)
	case *packet.ActorEvent:
		a := s.actor(p.EntityRuntimeID, now)
		if p.EntityRuntimeID == self {
			a.Name = display
			a.EntityType = "minecraft:player"
		}
		if p.EventType == packet.ActorEventDeath || p.EventType == packet.ActorEventInstantDeath {
			dead(a)
		} else if p.EventType == packet.ActorEventHurt {
			extra := map[string]any{"damage_known": false, "attacker_known": false, "cause_known": false}
			if time.Since(a.LastHurtTime) <= 50*time.Millisecond {
				extra["event_id"] = a.LastHurtID
				extra["correlated_health_change"] = true
			}
			emit("hurt_signal", a, extra)
			if now.Sub(a.LastHurtTime) > 50*time.Millisecond && a.PendingHurtTimer == nil {
				a.PendingHurtID = fmt.Sprintf("hurt:%d:%d", now.UnixNano(), a.RuntimeID)
				a.PendingHurtAt = now
				id := a.PendingHurtID
				a.PendingHurtTimer = time.AfterFunc(50*time.Millisecond, func() { yyFlushActorHurt(bm, s, a, id) })
			}
		}
	case *packet.MovePlayer:
		a := s.actor(p.EntityRuntimeID, now)
		if p.EntityRuntimeID == self {
			a.Name = display
			a.EntityType = "minecraft:player"
		}
		move(a, p.Position)
		if yyObserveRotation(a, [3]float32{p.Pitch, p.Yaw, p.HeadYaw}, [3]bool{true, true, true}, now, source) {
			emit("rotation", a, map[string]any{"pitch": p.Pitch, "yaw": p.Yaw, "head_yaw": p.HeadYaw})
		}
	case *packet.PlayerLocation:
		if s.Locations == nil {
			s.Locations = map[int64]yyLocatorObservation{}
		}
		if len(s.Locations) >= yyObservationSettings().Actors {
			if _, exists := s.Locations[p.EntityUniqueID]; !exists {
				var oldest int64
				var at time.Time
				for id, loc := range s.Locations {
					if at.IsZero() || loc.At.Before(at) {
						oldest, at = id, loc.At
					}
				}
				delete(s.Locations, oldest)
			}
		}
		loc := yyLocatorObservation{UniqueID: p.EntityUniqueID, Position: p.Position, Known: p.Type == packet.PlayerLocationTypeCoordinates && yyFiniteVec(p.Position), At: now}
		s.Locations[p.EntityUniqueID] = loc
		out = append(out, map[string]any{"type": "player_locator", "unique_id": strconv.FormatInt(p.EntityUniqueID, 10), "position": loc.Position, "position_known": loc.Known, "dimension_known": false, "source": source, "time": now.Format(time.RFC3339Nano)})
		for _, a := range s.Actors {
			if a.UniqueID == p.EntityUniqueID {
				if p.Type == packet.PlayerLocationTypeCoordinates {
					move(a, p.Position)
				} else {
					a.PositionKnown = false
				}
				break
			}
		}
	case *packet.MoveActorAbsolute:
		a := s.actor(p.EntityRuntimeID, now)
		move(a, p.Position)
		if yyObserveRotation(a, [3]float32(p.Rotation), [3]bool{true, true, true}, now, source) {
			emit("rotation", a, map[string]any{"pitch": p.Rotation[0], "yaw": p.Rotation[1], "head_yaw": p.Rotation[2]})
		}
	case *packet.MoveActorDelta:
		a := s.actor(p.EntityRuntimeID, now)
		if yyObserveRotation(a, [3]float32(p.Rotation), [3]bool{p.Flags&packet.MoveActorDeltaFlagHasRotX != 0, p.Flags&packet.MoveActorDeltaFlagHasRotY != 0, p.Flags&packet.MoveActorDeltaFlagHasRotZ != 0}, now, source) {
			emit("rotation", a, map[string]any{"pitch_known": a.RotationKnown[0], "yaw_known": a.RotationKnown[1]})
		}
		if p.Flags&(packet.MoveActorDeltaFlagHasX|packet.MoveActorDeltaFlagHasY|packet.MoveActorDeltaFlagHasZ) == 0 {
			break
		}
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
		a.PreviousHealth = nil
		delete(a.Attributes, "minecraft:health")
		a.MaxHealth = nil
		a.HealthTime = time.Time{}
		move(a, p.Position)
		if !a.RespawnDelivered {
			a.RespawnDelivered = true
			emit("respawn", a, map[string]any{"position": p.Position, "death_event_id": a.DeathID})
		}
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
			a.Present = false
		}
		a := s.actor(self, now)
		previousDimension := a.Dimension
		dimension = p.Dimension
		a.Name = display
		move(a, p.Position)
		emit("dimension_change", a, map[string]any{"previous_dimension": previousDimension, "dimension": dimension})
	case *packet.BlockActorData:
		// Retain the block entity type only, not container contents/private NBT.
		key := yyBlockKey(dimension, p.Position, 0)
		v := map[string]any{"known": false}
		for k, x := range s.Blocks[key] {
			v[k] = x
		}
		if len(s.Blocks) >= 4096 && s.Blocks[key] == nil {
			s.Blocks = map[string]map[string]any{}
		}
		id, _ := p.NBTData["id"].(string)
		if id == "" {
			id, _ = p.NBTData["Id"].(string)
		}
		typeID := strings.TrimPrefix(strings.ToLower(id), "minecraft:")
		v["container_known"], v["is_container"] = id != "", typeID == "chest" || typeID == "barrel" || typeID == "trappedchest" || typeID == "trapped_chest"
		v["block_entity_id"], v["container_timestamp"], v["container_source"] = id, now.UnixMilli(), source
		v["position"], v["dimension"] = p.Position, dimension
		s.Blocks[key] = v
		out = append(out, map[string]any{"type": "container_change", "block": v, "cause_known": false})
	case *packet.UpdateBlock:
		key := yyBlockKey(dimension, p.Position, p.Layer)
		old := s.Blocks[key]
		if len(s.Blocks) >= 4096 && old == nil {
			s.Blocks = map[string]map[string]any{}
		}
		v := map[string]any{"known": true, "dimension": dimension, "position": p.Position, "layer": p.Layer, "runtime_id": p.NewBlockRuntimeID, "time": now.Format(time.RFC3339Nano), "source": source, "timestamp": now.UnixMilli()}
		v["container_known"] = false // A new block may replace the cached container.
		s.Blocks[key] = v
		out = append(out, map[string]any{"type": "block_change", "time": now.Format(time.RFC3339Nano), "source": source, "block": v, "previous": old, "cause_known": false})
	}
	for _, e := range out {
		e["timestamp"] = now.UnixMilli()
		yyModuleRemember(s, e)
		data := map[string]any{}
		for k, v := range e {
			if k != "player" && k != "type" && k != "timestamp" {
				data[k] = v
			}
		}
		e["data"] = data
		s.Seq++
		e["sequence"] = s.Seq
		s.Events = append(s.Events, e)
		kind, _ := e["type"].(string)
		if kind != "move" && (kind != "rotation" || e["entity_type"] == "minecraft:player") {
			s.ActionEvents = append(s.ActionEvents, e)
		}
	}
	if n := len(s.ActionEvents) - yyObservationSettings().EventHistory; n > 0 {
		s.ActionEvictedSeq = s.ActionEvents[n-1]["sequence"].(uint64)
		s.ActionEvents = s.ActionEvents[n:]
	}
	if len(s.Events) > yyObservationSettings().EventHistory {
		s.Events = s.Events[len(s.Events)-yyObservationSettings().EventHistory:]
	}
	s.Unlock()
	senseLocked = false
	for _, e := range out {
		yyPatrolObserve(bm, e)
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
	dimension, gameMode := bm.dimension, bm.gameMode
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
	// Snapshot keeps legacy flat fields while exposing typed observations separately.
	extras := map[string]any{"uuid": a.UUID, "xuid": a.XUID, "health_state": yyHealthSnapshot(a, connected), "rotation": yyRotationSnapshot(a, connected), "equipment": yyEquipmentSnapshot(s, a, connected), "attributes": yyAttributeSnapshot(a), "armor": nil, "armor_known": false}
	if x, ok := a.Attributes["minecraft:armor"]; ok {
		extras["armor"] = x.Value
		extras["armor_known"] = true
	}
	age := time.Since(a.LastSeen).Milliseconds()
	positionAge := time.Since(a.PositionTime).Milliseconds()
	healthAge := time.Since(a.HealthTime).Milliseconds()
	v := map[string]any{"known": true, "connected": connected, "runtime_id": strconv.FormatUint(a.RuntimeID, 10), "unique_id": strconv.FormatInt(a.UniqueID, 10), "name": a.Name, "entity_type": a.EntityType, "position": a.Position, "position_known": a.PositionKnown, "position_time": a.PositionTime, "position_source": a.PositionSource, "position_age_ms": positionAge, "position_stale": !connected || !a.PositionKnown || positionAge > 15000, "dimension": a.Dimension, "health": a.Health, "max_health": a.MaxHealth, "health_known": a.Health != nil, "health_time": a.HealthTime, "health_source": a.HealthSource, "health_age_ms": healthAge, "health_stale": !connected || a.Health == nil || healthAge > 15000, "dead": a.Dead, "present": a.Present, "source": a.Source, "last_seen": a.LastSeen, "age_ms": age, "stale": !connected || !a.Present || age > int64(yyObservationSettings().StaleMS)}
	v["position_stale"] = yyObservationStale(a.PositionTime, a.Present, connected)
	v["health_stale"] = yyObservationStale(a.HealthTime, a.Present, connected)
	for k, x := range extras {
		v[k] = yyObservationCopy(x)
	}
	v["position_age_ms"] = yyObservationAge(a.PositionTime)
	v["health_age_ms"] = yyObservationAge(a.HealthTime)
	v["name_known"] = yyKnownPlayerName(a.Name)
	v["uuid_known"] = a.UUID != "" && a.UUID != "00000000-0000-0000-0000-000000000000"
	v["xuid_known"] = a.XUID != ""
	if a.RuntimeID == self {
		v["dimension"] = dimension
		v["dimension_known"] = connected
		v["dimension_source"] = "connection_spawn_or_change_dimension"
		v["game_mode"] = gameMode
	}
	return v
}

// Count directly under the observation lock. Periodic counters must not copy
// inventory, attributes and removed entity snapshots into every language VM.
func yySenseStats(bm *BotManager) map[string]any {
	connected := bm != nil && bm.IsConnected()
	cached, present, fresh := 0, 0, 0
	var packets uint64
	if bm != nil {
		s := yySenseFor(bm)
		s.Lock()
		cutoff := time.Now().Add(-time.Duration(yyObservationSettings().StaleMS) * time.Millisecond)
		cached, packets = len(s.Actors), s.PacketCount
		for _, a := range s.Actors {
			if a.Present {
				present++
				if connected && !a.LastSeen.Before(cutoff) {
					fresh++
				}
			}
		}
		s.Unlock()
	}
	return map[string]any{"connected": connected, "cached": cached, "present": present, "fresh": fresh, "active_actors": fresh, "packet_count": packets, "time": time.Now().Format(time.RFC3339Nano)}
}
func yySenseSnapshot(bm *BotManager) map[string]any {
	return yySenseSnapshotFiltered(bm, false, "")
}
func yySenseSnapshotFiltered(bm *BotManager, presentOnly bool, entityType string) map[string]any {
	actors := []map[string]any{}
	if bm != nil {
		s := yySenseFor(bm)
		s.Lock()
		ids := []uint64{}
		for id, a := range s.Actors {
			if (!presentOnly || a.Present) && (entityType == "" || a.EntityType == entityType) {
				ids = append(ids, id)
			}
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
