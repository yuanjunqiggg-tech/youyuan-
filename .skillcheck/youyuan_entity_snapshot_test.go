package main

import (
	"testing"
	"time"
)

func yyFloat32Ptr(v float32) *float32 { return &v }

// G01 requires the health snapshot to carry armor. It may only be known when the
// server syncs minecraft:armor; guessing from worn items is forbidden.
func TestHealthSnapshotIncludesArmorWhenSynced(t *testing.T) {
	now := time.Now()
	a := &yyActorObservation{
		RuntimeID: 42,
		Present:   true,
		Health:    yyFloat32Ptr(18),
		MaxHealth: yyFloat32Ptr(20),
		HealthTime: now,
		Attributes: map[string]yyObservedAttribute{
			"minecraft:armor": {Value: 7, Min: 0, Max: 30, At: now, Source: "UpdateAttributes"},
		},
	}
	snap := yyHealthSnapshot(a, true)
	if snap["armor_known"] != true {
		t.Fatalf("armor should be known: %v", snap)
	}
	if snap["armor"] != float32(7) {
		t.Fatalf("armor value lost: %v", snap["armor"])
	}
	if snap["armor_stale"] == true {
		t.Fatal("fresh armor must not be stale")
	}
	if snap["current"] == nil || *snap["current"].(*float32) != 18 {
		t.Fatalf("health value changed: %v", snap["current"])
	}
}

// No minecraft:armor in this session means unknown, never zero.
func TestHealthSnapshotArmorStaysUnknownWithoutSync(t *testing.T) {
	a := &yyActorObservation{RuntimeID: 7, Present: true, Health: yyFloat32Ptr(20), HealthTime: time.Now()}
	snap := yyHealthSnapshot(a, true)
	if snap["armor_known"] != false {
		t.Fatalf("armor must stay unknown, got %v", snap["armor_known"])
	}
	if snap["armor"] != nil {
		t.Fatalf("unknown armor must be nil, got %v", snap["armor"])
	}
}

// G05: each axis keeps its own knowledge flag; a known pitch must not imply a
// known yaw, and the overall flag must require both.
func TestRotationSnapshotReportsPerAxisKnowledge(t *testing.T) {
	now := time.Now()
	a := &yyActorObservation{
		RuntimeID:     9,
		Present:       true,
		Rotation:      [3]float32{-35, 0, 0},
		RotationKnown: [3]bool{true, false, false},
		RotationTime:  [3]time.Time{now, {}, {}},
		RotationSource: [3]string{
			"MoveActorDelta", "", "",
		},
	}
	snap := yyRotationSnapshot(a, true)
	if snap["known"] != false {
		t.Fatal("overall rotation must be unknown while yaw is unknown")
	}
	if snap["pitch"] != float32(-35) || snap["pitch_known"] != true {
		t.Fatalf("pitch lost: %v / %v", snap["pitch"], snap["pitch_known"])
	}
	if snap["yaw_known"] != false || snap["yaw"] != nil {
		t.Fatalf("unknown yaw must stay nil: %v / %v", snap["yaw"], snap["yaw_known"])
	}
	if snap["stale"] != true {
		t.Fatal("unknown yaw must make the snapshot stale")
	}
}

// G04: six slots plus body; a slot never observed must be reported unknown
// rather than silently missing.
func TestEquipmentSnapshotMarksUnobservedSlotsUnknown(t *testing.T) {
	a := &yyActorObservation{RuntimeID: 3, Name: "bot", Present: true}
	snap := yyEquipmentSnapshot(&yySenseState{ItemNames: map[int32]string{}}, a, true)
	for _, slot := range []string{"head", "chest", "legs", "feet", "main_hand", "off_hand", "body"} {
		entry, ok := snap[slot].(map[string]any)
		if !ok {
			t.Fatalf("slot %s missing from snapshot", slot)
		}
		if entry["known"] != false {
			t.Fatalf("slot %s must be unknown, got %v", slot, entry)
		}
	}
	if snap["runtime_id"] != "3" {
		t.Fatalf("runtime id must be a decimal string, got %v", snap["runtime_id"])
	}
}
