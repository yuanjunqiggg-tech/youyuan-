package main

import "github.com/OmineDev/flowers-for-machines/core/minecraft"

// yyApplySpawnStateLocked replaces all spawn state, including on reconnect.
// The caller holds bm.mu. StartGame is authoritative even if the server does
// not send a ChangeDimension or movement packet after joining.
func yyApplySpawnStateLocked(bm *BotManager, gd minecraft.GameData) {
	bm.eid = gd.EntityUniqueID
	bm.runtimeID = gd.EntityRuntimeID
	bm.gameMode = gd.PlayerGameMode
	bm.pos = gd.PlayerPosition
	bm.dimension = gd.Dimension
	bm.pitch, bm.yaw = gd.Pitch, gd.Yaw
}
