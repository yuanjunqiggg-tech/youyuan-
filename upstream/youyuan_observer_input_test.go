package main

import (
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestObserverInputNormalizesSwingWithoutInventingHit(t *testing.T) {
	r := yyObserverInputPacket(&packet.Animate{ActionType: packet.AnimateActionSwingArm, EntityRuntimeID: 9007199254740993})
	if len(r) != 1 || r[0]["kind"] != "swing_input" || r[0]["data"].(map[string]any)["runtime_id"] != "9007199254740993" || r[0]["data"].(map[string]any)["missed_swing_known"] != false {
		t.Fatal(r)
	}
	if len(yyObserverInputPacket(&packet.Animate{ActionType: packet.AnimateActionCriticalHit})) != 0 {
		t.Fatal("critical animation became empty swing")
	}
	bits := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	bits.Set(packet.InputFlagMissedSwing)
	r = yyObserverInputPacket(&packet.PlayerAuthInput{InputData: bits, Tick: 44})
	if len(r) != 2 || r[1]["data"].(map[string]any)["missed_swing"] != true {
		t.Fatal(r)
	}
	if len(yyObserverInputPacket(&packet.PlayerAuthInput{})) != 1 {
		t.Fatal("empty bitset panicked or invented swing")
	}
}
func TestObserverInputAttackTargetAndDirection(t *testing.T) {
	p := &packet.InventoryTransaction{TransactionData: &protocol.UseItemOnEntityTransactionData{ActionType: protocol.UseItemOnEntityActionAttack, TargetEntityRuntimeID: 9007199254740993}}
	r := yyObserverInputPacket(p)
	if len(r) != 1 || r[0]["kind"] != "attack_input" || r[0]["data"].(map[string]any)["target_runtime_id"] != "9007199254740993" {
		t.Fatal(r)
	}
	p.TransactionData.(*protocol.UseItemOnEntityTransactionData).ActionType = protocol.UseItemOnEntityActionInteract
	if len(yyObserverInputPacket(p)) != 0 || len(yyObserverInputPacket(&packet.UpdateAttributes{})) != 0 {
		t.Fatal("interaction or outgoing health fabricated into attack")
	}
}
