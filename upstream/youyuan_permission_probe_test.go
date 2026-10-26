package main

import (
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/google/uuid"
)

func TestPermissionProbeIgnoresUnrelatedCommandReceipt(t *testing.T) {
	bm := newOPTestBot()
	bm.sayTestMsg, bm.sayTestRequestID = "unique-probe", uuid.New()
	response := &packet.CommandOutput{SuccessCount: 1, CommandOrigin: protocol.CommandOrigin{UUID: uuid.New()}}
	bm.acceptPermissionReceipt(response)
	if bm.sayTestMsg == "" {
		t.Fatal("unrelated success confirmed permission")
	}
	response.CommandOrigin.UUID = bm.sayTestRequestID
	response.SuccessCount = 0
	bm.acceptPermissionReceipt(response)
	if bm.sayTestMsg == "" {
		t.Fatal("failed probe confirmed permission")
	}
	response.SuccessCount = 1
	bm.acceptPermissionReceipt(response)
	if bm.sayTestMsg != "" {
		t.Fatal("matching successful probe lost")
	}
}

func TestExplicitMemberPacketOutranksCommandFallback(t *testing.T) {
	bm := newOPTestBot()
	ready := 0
	bm.onReady = func() { ready++ }
	bm.setOP(false, packet.CommandPermissionLevelNormal, "UpdateAbilities")
	for _, source := range []string{"SayTest", "CmdVerified", "opPlayers"} {
		if bm.confirmCommandPermission(3, source) || bm.IsOP() || ready != 0 {
			t.Fatal("fallback overrode authoritative revocation", source)
		}
	}
	bm.setOP(true, packet.CommandPermissionLevelAdmin, "UpdateAbilities")
	if !bm.confirmCommandPermission(3, "CmdVerified") || ready != 1 {
		t.Fatal("real permission grant could not complete initialization")
	}
	bm.setOP(false, packet.CommandPermissionLevelNormal, "AdventureSettings")
	if bm.confirmCommandPermission(3, "SayTest") || bm.IsOP() || ready != 1 {
		t.Fatal("late probe restored revoked permission")
	}
}

func TestUnknownPermissionKeepsLegacyCorrelatedFallback(t *testing.T) {
	bm := newOPTestBot()
	if !bm.confirmCommandPermission(3, "CmdVerified") || !bm.IsOP() || bm.opPacketConfirmed {
		t.Fatal("legacy fallback was removed or mislabeled as packet confirmation")
	}
}
