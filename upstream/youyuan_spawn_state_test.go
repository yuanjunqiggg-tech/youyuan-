package main

import (
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/go-gl/mathgl/mgl32"
)

func TestReconnectRestoresSpawnDimensionAndRotationWithoutMovement(t *testing.T) {
	bm := &BotManager{dimension: 0, pitch: 80, yaw: 90, runtimeID: 1}
	for _, gd := range []minecraft.GameData{
		{Dimension: 2, PlayerPosition: mgl32.Vec3{-4, 58, -36}, Pitch: -25, Yaw: 130, EntityUniqueID: -12, EntityRuntimeID: 22696, PlayerGameMode: 1},
		{Dimension: 1, PlayerPosition: mgl32.Vec3{120, 70, 40}, Pitch: 0, Yaw: 0, EntityUniqueID: 9, EntityRuntimeID: 30000, PlayerGameMode: 0},
	} {
		bm.mu.Lock()
		yyApplySpawnStateLocked(bm, gd)
		bm.mu.Unlock()
		if bm.dimension != gd.Dimension || bm.pitch != gd.Pitch || bm.yaw != gd.Yaw || bm.pos != gd.PlayerPosition || bm.runtimeID != gd.EntityRuntimeID || bm.eid != gd.EntityUniqueID || bm.gameMode != gd.PlayerGameMode {
			t.Fatalf("new connection retained previous world state: dimension=%d pitch=%v yaw=%v position=%v runtime=%d", bm.dimension, bm.pitch, bm.yaw, bm.pos, bm.runtimeID)
		}
	}
}
