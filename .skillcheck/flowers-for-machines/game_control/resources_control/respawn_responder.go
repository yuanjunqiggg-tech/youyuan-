package resources_control

import (
	"context"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

type respawnResponder struct {
	ctx   context.Context
	stop  context.CancelFunc
	write func(packet.Packet) error
	gate  chan struct{} // One active worker, not a task queue.
}

func newRespawnResponder(ctx context.Context, write func(packet.Packet) error) *respawnResponder {
	ctx, stop := context.WithCancel(ctx)
	return &respawnResponder{ctx: ctx, stop: stop, write: write, gate: make(chan struct{}, 1)}
}

func (r *respawnResponder) acquire() bool {
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
	return true
}

func (r *respawnResponder) search(runtimeID uint64) {
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
	if !r.acquire() {
		return
	}
	// shortcut: overlapping respawns still backpressure the reader at the gate;
	// replacing/merging their five-packet sequences needs protocol evidence first.
	go func() {
		defer func() { <-r.gate }()
		inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
		inputData.Set(packet.InputFlagStartFlying)
		timer := time.NewTimer(time.Second / 20 * 3)
		defer timer.Stop()
		for range 5 {
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
