// Package playerrelay implements a player-owned, same-protocol encrypted gateway.
// It forwards original application frames, including unknown NetEase extensions.
package playerrelay

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

type Event map[string]any

// Observer only decodes selected frames for observation. An observation failure
// never changes or discards the original frame delivered to the other peer.
type Observer struct {
	mu                       sync.Mutex
	runtimeID                uint64
	shieldID                 int32
	healthKnown              bool
	healthCurrent, healthMax float32
}

func (o *Observer) Observe(frame []byte, fromPlayer bool) (events []Event, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var packetID uint32
	defer func() {
		if recover() != nil {
			events = nil
			err = fmt.Errorf("packet=%d observation decode failed", packetID)
		}
	}()
	b := bytes.NewBuffer(frame)
	h := packet.Header{}
	if err := h.Read(b); err != nil {
		return nil, fmt.Errorf("invalid observation header")
	}
	packetID = h.PacketID
	var p packet.Packet
	if fromPlayer {
		switch h.PacketID {
		case packet.IDAnimate:
			p = &packet.Animate{}
		case packet.IDPlayerAuthInput:
			p = &packet.PlayerAuthInput{}
		case packet.IDInventoryTransaction:
			p = &packet.InventoryTransaction{}
		default:
			return nil, nil
		}
	} else {
		switch h.PacketID {
		case packet.IDStartGame:
			// The identity prefix is stable; the rest of StartGame is versioned.
			// Read only these two fields, leaving the original world frame intact.
			r := protocol.NewReader(b, 0, true)
			var unique int64
			r.Varint64(&unique)
			r.Varuint64(&o.runtimeID)
			o.healthKnown = false
			return nil, nil
		case packet.IDItemRegistry:
			p = &packet.ItemRegistry{}
		case packet.IDUpdateAttributes:
			p = &packet.UpdateAttributes{}
		default:
			return nil, nil
		}
	}
	p.Marshal(protocol.NewReader(b, o.shieldID, true))
	// NetEase appends registry extensions. The common item table is sufficient
	// for the shield ID; forward every extension byte untouched to the client.
	if b.Len() != 0 && h.PacketID != packet.IDItemRegistry {
		return nil, fmt.Errorf("packet=%d observation protocol mismatch", packetID)
	}
	row := func(kind, name string, data map[string]any) Event {
		direction := "server_to_player"
		if fromPlayer {
			direction = "player_to_server"
		}
		return Event{"kind": kind, "packet_type": name, "observation": "authenticated_proxy_frame", "observed_at": time.Now().UTC().Format(time.RFC3339Nano), "direction": direction, "data": data}
	}
	switch v := p.(type) {
	case *packet.StartGame:
		o.runtimeID = v.EntityRuntimeID
	case *packet.ItemRegistry:
		for _, item := range v.Items {
			if item.Name == "minecraft:shield" {
				o.shieldID = int32(item.RuntimeID)
			}
		}
	case *packet.Animate:
		if v.ActionType == packet.AnimateActionSwingArm {
			if o.runtimeID == 0 || v.EntityRuntimeID != o.runtimeID {
				return nil, fmt.Errorf("input entity does not match authenticated player")
			}
			events = append(events, row("swing_input", "Animate", map[string]any{"runtime_id": strconv.FormatUint(v.EntityRuntimeID, 10), "missed_swing_known": false}))
		}
	case *packet.PlayerAuthInput:
		events = append(events, row("position_input", "PlayerAuthInput", map[string]any{"position": v.Position, "pitch": v.Pitch, "yaw": v.Yaw, "tick": strconv.FormatUint(v.Tick, 10)}))
		if v.InputData.Len() > packet.InputFlagMissedSwing && v.InputData.Load(packet.InputFlagMissedSwing) {
			events = append(events, row("swing_input", "PlayerAuthInput", map[string]any{"missed_swing": true, "tick": strconv.FormatUint(v.Tick, 10)}))
		}
	case *packet.InventoryTransaction:
		if d, ok := v.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && d.ActionType == protocol.UseItemOnEntityActionAttack {
			events = append(events, row("attack_input", "InventoryTransaction", map[string]any{"target_runtime_id": strconv.FormatUint(d.TargetEntityRuntimeID, 10), "position": d.Position}))
		}
	case *packet.UpdateAttributes:
		// This player-scoped source reports only this player's health. Other
		// entity observations require their own explicitly scoped server source.
		if o.runtimeID == 0 || v.EntityRuntimeID != o.runtimeID {
			return nil, nil
		}
		for _, a := range v.Attributes {
			if a.Name == "minecraft:health" && !math.IsNaN(float64(a.Value)) && !math.IsInf(float64(a.Value), 0) && a.Max > 0 {
				if o.healthKnown && o.healthCurrent == a.Value && o.healthMax == a.Max {
					continue
				}
				o.healthKnown = true
				o.healthCurrent = a.Value
				o.healthMax = a.Max
				events = append(events, row("health_observation", "UpdateAttributes", map[string]any{"runtime_id": strconv.FormatUint(v.EntityRuntimeID, 10), "current": a.Value, "max": a.Max, "known": true, "source": "server_packet", "tick": strconv.FormatUint(v.Tick, 10)}))
			}
		}
	}
	return events, nil
}
