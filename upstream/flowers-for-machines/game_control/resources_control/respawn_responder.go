package resources_control

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

type respawnResponder struct {
	ctx              context.Context
	stop             context.CancelFunc
	write            func(packet.Packet) error
	gate             chan struct{} // One active worker, not a task queue.
	dead             atomic.Bool
	enabled          atomic.Bool
	deathGeneration  atomic.Uint64
	policyGeneration atomic.Uint64
}

func newRespawnResponder(ctx context.Context, write func(packet.Packet) error) *respawnResponder {
	ctx, stop := context.WithCancel(ctx)
	r := &respawnResponder{ctx: ctx, stop: stop, write: write, gate: make(chan struct{}, 1)}
	r.enabled.Store(true)
	return r
}

func (r *respawnResponder) setEnabled(enabled bool) {
	if r.enabled.Swap(enabled) != enabled {
		r.policyGeneration.Add(1)
	}
	if !enabled {
		r.dead.Store(false)
		r.deathGeneration.Add(1)
	}
}

func (r *respawnResponder) acquire() bool {
	if !r.enabled.Load() {
		return false
	}
	select {
	case r.gate <- struct{}{}:
	case <-r.ctx.Done():
		return false
	}
	select {
	case <-r.ctx.Done():
		<-r.gate
		return false
	default:
	}
	if !r.enabled.Load() {
		<-r.gate
		return false
	}
	return true
}

func (r *respawnResponder) search(runtimeID uint64) {
	r.deathGeneration.Add(1)
	if !r.acquire() {
		return
	}
	defer func() { <-r.gate }()
	_ = r.write(&packet.Respawn{State: packet.RespawnStateClientReadyToSpawn, EntityRuntimeID: runtimeID})
	select {
	case <-r.ctx.Done():
		return
	default:
	}
	_ = r.write(&packet.PlayerAction{EntityRuntimeID: runtimeID, ActionType: protocol.PlayerActionRespawn, BlockFace: -1})
}

func (r *respawnResponder) start(position mgl32.Vec3) {
	r.deathGeneration.Add(1)
	r.dead.Store(false)
	if !r.acquire() {
		return
	}
	// shortcut: overlapping respawns still backpressure the reader at the gate;
	// replacing/merging their five-packet sequences needs protocol evidence first.
	generation := r.policyGeneration.Load()
	go func() {
		defer func() { <-r.gate }()
		inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
		inputData.Set(packet.InputFlagStartFlying)
		timer := time.NewTimer(time.Second / 20 * 3)
		defer timer.Stop()
		for range 5 {
			if !r.enabled.Load() || r.policyGeneration.Load() != generation {
				return
			}
			select {
			case <-r.ctx.Done():
				return
			default:
			}
			_ = r.write(&packet.PlayerAuthInput{Position: position, InputData: inputData})
			timer.Reset(time.Second / 20 * 3)
			select {
			case <-r.ctx.Done():
				return
			case <-timer.C:
			}
		}
	}()
}

// A server may wait for the client's respawn action after zero health rather
// than proactively sending Respawn(Searching). Do not wait on that packet.
func (r *respawnResponder) health(runtimeID, selfID uint64, health float32) {
	if !r.enabled.Load() {
		return
	}
	if selfID == 0 || runtimeID != selfID {
		return
	}
	if health > 0 {
		r.dead.Store(false)
		r.deathGeneration.Add(1)
		return
	}
	if !r.dead.CompareAndSwap(false, true) {
		return
	}
	generation := r.deathGeneration.Add(1)
	go func() {
		timer := time.NewTimer(200 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-r.ctx.Done():
			return
		case <-timer.C:
		}
		if r.enabled.Load() && r.dead.Load() && r.deathGeneration.Load() == generation {
			_ = r.write(&packet.PlayerAction{EntityRuntimeID: selfID, ActionType: protocol.PlayerActionRespawn, BlockFace: -1})
		}
	}()
}
