package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"testing"
)

func TestPermissionRevocationCannotReuseOldOperatorCache(t *testing.T) {
	opMu.Lock()
	oldPlayers, oldOps, oldCount := players, opPlayers, lastOPCnt
	players, opPlayers, lastOPCnt = map[string]int64{}, map[string]*opInfo{}, -1
	opMu.Unlock()
	t.Cleanup(func() { opMu.Lock(); players, opPlayers, lastOPCnt = oldPlayers, oldOps, oldCount; opMu.Unlock() })
	const name = "permission-control"
	const uniqueID = int64(-4294958259)
	const runtimeID = uint64(26483)
	bm := newOPTestBot()
	bm.display = name
	add := func(permission byte) {
		HandleOPPacket(&packet.AddPlayer{Username: name, EntityRuntimeID: runtimeID, AbilityData: protocol.AbilityData{EntityUniqueID: uniqueID, CommandPermissions: permission}}, bm.logCh)
	}
	add(packet.CommandPermissionLevelAdmin)
	if players[name] != uniqueID {
		t.Fatal("AddPlayer must retain UniqueID, not RuntimeID")
	}
	bm.checkSelfOP()
	if !bm.IsOP() {
		t.Fatal("operator positive control failed")
	}
	HandleOPPacket(&packet.UpdateAbilities{AbilityData: protocol.AbilityData{EntityUniqueID: uniqueID, CommandPermissions: packet.CommandPermissionLevelNormal, PlayerPermissions: packet.PermissionLevelMember}}, bm.logCh)
	bm.setOP(false, 0, "UpdateAbilities")
	bm.checkSelfOP() // Positive output from an unrelated command used this fallback.
	if bm.IsOP() {
		t.Fatal("revoked permission was restored from stale cache")
	}
	add(packet.CommandPermissionLevelAdmin)
	HandleOPPacket(&packet.AdventureSettings{PlayerUniqueID: uniqueID, CommandPermissionLevel: packet.CommandPermissionLevelNormal}, bm.logCh)
	if _, ok := opPlayers[name]; ok {
		t.Fatal("AdventureSettings revocation left stale operator")
	}
	add(packet.CommandPermissionLevelAdmin)
	add(packet.CommandPermissionLevelNormal)
	if _, ok := opPlayers[name]; ok {
		t.Fatal("AddPlayer member update left stale operator")
	}
}
