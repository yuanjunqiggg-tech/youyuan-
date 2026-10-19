package main

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// Numeric metadata only: no packet payloads, JSON, file I/O, entity lock,
// per-packet goroutines or retention sampling on the receive hot path.
// These are successfully decoded packets delivered to our registered listener,
// not raw wire packets, decode failures or packets consumed during login.
type yyPacketIngress struct {
	started time.Time
	scope   string
	types   sync.Map            // uint32 -> *yyPacketIngressType; bounded by received protocol types.
	rpc     yyRPCIngressCatalog // Event names only, while explicit PyRpc capture is active.
}

type yyPacketIngressType struct {
	name          string
	decoded       atomic.Uint64
	selected      atomic.Uint64
	filtered      atomic.Uint64
	enqueued      atomic.Uint64
	cancelled     atomic.Uint64
	loginObserved atomic.Uint64
}

func (a *yyPacketIngress) entry(pk packet.Packet) *yyPacketIngressType {
	if v, ok := a.types.Load(pk.ID()); ok {
		return v.(*yyPacketIngressType)
	}
	v, _ := a.types.LoadOrStore(pk.ID(), &yyPacketIngressType{name: yyPacketName(pk)})
	return v.(*yyPacketIngressType)
}

func (a *yyPacketIngress) observe(pk packet.Packet, selected bool) {
	if a == nil || pk == nil {
		return
	}
	e := a.entry(pk)
	e.decoded.Add(1)
	if selected {
		e.selected.Add(1)
	} else {
		e.filtered.Add(1)
	}
}

func (a *yyPacketIngress) observeLogin(pk packet.Packet) {
	if a == nil || pk == nil {
		return
	}
	e := a.entry(pk)
	e.decoded.Add(1)
	e.loginObserved.Add(1)
	if p, ok := pk.(*packet.PyRpc); ok {
		a.rpc.observe(p, yyObservationSettings().EventHistory)
	}
}

func (a *yyPacketIngress) delivery(pk packet.Packet, enqueued bool) {
	if a == nil || pk == nil {
		return
	}
	e := a.entry(pk)
	if enqueued {
		e.enqueued.Add(1)
	} else {
		e.cancelled.Add(1)
	}
}

func yySelectIncomingPacket(bm *BotManager, pk packet.Packet, wanted map[uint32]bool, a *yyPacketIngress) bool {
	if pk == nil {
		return false
	}
	selected := wanted[pk.ID()]
	if !selected {
		name := yyPacketName(pk)
		selected = packetHubWants(name) || yyCaptureWants(bm, name)
	}
	a.observe(pk, selected)
	if rpc, ok := pk.(*packet.PyRpc); ok && yyCaptureWants(bm, "PyRpc") && a != nil {
		a.rpc.observe(rpc, yyObservationSettings().EventHistory)
	}
	return selected
}

func (a *yyPacketIngress) snapshot() map[string]any {
	rows := map[string]map[string]any{}
	if a != nil {
		a.types.Range(func(key, value any) bool {
			e := value.(*yyPacketIngressType)
			name := e.name
			if name == "Unknown" {
				name += ":" + strconv.FormatUint(uint64(key.(uint32)), 10)
			}
			rows[name] = map[string]any{
				"packet_id": key.(uint32), "decoded": e.decoded.Load(),
				"selected": e.selected.Load(), "filtered": e.filtered.Load(),
				"enqueued": e.enqueued.Load(), "cancelled": e.cancelled.Load(),
				"login_observed": e.loginObserved.Load(),
			}
			return true
		})
	}
	started := any(nil)
	if a != nil && !a.started.IsZero() {
		started = a.started.UnixMilli()
	}
	scope := "successfully decoded incoming packets after listener registration, including replayed login cache; excludes decode failures and challenge-handled engine frames"
	rpcSnapshot := yyRPCIngressSnapshot(a)
	if a != nil && a.scope != "" {
		scope = a.scope
		rpcSnapshot["collect_when"] = "login challenge callback; metadata only"
	}
	return map[string]any{
		"active": a != nil, "started_at": started, "types": rows,
		"rpc_catalog": rpcSnapshot,
		"scope":       scope,
		"consistency": "live atomic counters, not a transactional snapshot; selected delivery may still be pending",
	}
}
