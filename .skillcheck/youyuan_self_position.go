package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

// Locator broadcasts for another UniqueID never move this bot's input origin.
func yyApplySelfPosition(bm *BotManager, pk packet.Packet) bool {
	if bm == nil {
		return false
	}
	bm.mu.Lock()
	defer bm.mu.Unlock()
	var position mgl32.Vec3
	switch p := pk.(type) {
	case *packet.PlayerLocation:
		if p.EntityUniqueID != bm.eid || p.Type != packet.PlayerLocationTypeCoordinates {
			return false
		}
		position = p.Position
	case *packet.MovePlayer:
		if p.EntityRuntimeID != bm.runtimeID {
			return false
		}
		position = p.Position
	case *packet.Respawn:
		if p.EntityRuntimeID != bm.runtimeID || p.State != packet.RespawnStateReadyToSpawn {
			return false
		}
		position = p.Position
	default:
		return false
	}
	if !yyFiniteVec(position) {
		return false
	}
	bm.pos = position
	return true
}
