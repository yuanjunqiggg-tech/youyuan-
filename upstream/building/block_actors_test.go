package building

import (
	"testing"
)

func TestNormalizeBeehiveNBT_EmptyBees(t *testing.T) {
	// 装饰性蜂巢：bees 为空列表，应清除 bee 相关字段
	nbt := map[string]any{
		"bees":              []any{},
		"honey_level":       int32(0),
		"max_occupants":     int32(3),
		"should_refill_flowers": byte(1),
		"id":                "Beehive",
		"x":                 int32(10),
		"y":                 int32(64),
		"z":                 int32(100),
	}
	out := NormalizeBlockEntityNBT("minecraft:beehive", nbt)

	if _, ok := out["bees"]; ok {
		t.Errorf("expected 'bees' to be removed for empty list, but found: %v", out["bees"])
	}
	if _, ok := out["Bees"]; ok {
		t.Errorf("expected 'Bees' to be removed")
	}
	if _, ok := out["honey_level"]; ok {
		t.Errorf("expected 'honey_level' to be removed")
	}
	if _, ok := out["max_occupants"]; ok {
		t.Errorf("expected 'max_occupants' to be removed")
	}
	if _, ok := out["should_refill_flowers"]; ok {
		t.Errorf("expected 'should_refill_flowers' to be removed")
	}
}

func TestNormalizeBeehiveNBT_HasBees(t *testing.T) {
	// 有实际蜜蜂数据：现在应全部清除（装饰性蜂巢）
	nbt := map[string]any{
		"bees": []any{
			map[string]any{
				"MinOccupationTicks": int32(600),
			},
		},
		"honey_level":       int32(5),
		"max_occupants":     int32(3),
		"should_refill_flowers": byte(1),
		"id":                "Beehive",
	}
	out := NormalizeBlockEntityNBT("minecraft:beehive", nbt)

	if _, ok := out["bees"]; ok {
		t.Errorf("expected 'bees' to be removed")
	}
	if _, ok := out["honey_level"]; ok {
		t.Errorf("expected 'honey_level' to be removed")
	}
}

func TestNormalizeBeehiveNBT_CapitalBees(t *testing.T) {
	// Java 格式用大写 Bees，现在应全部清除
	nbt := map[string]any{
		"Bees": []any{
			map[string]any{
				"Pos": []int32{0, 0, 0},
			},
		},
		"HoneyLevel": int32(5),
		"id":         "minecraft:beehive",
	}
	out := NormalizeBlockEntityNBT("minecraft:bee_nest", nbt)

	if _, ok := out["Bees"]; ok {
		t.Errorf("expected 'Bees' to be removed")
	}
}

func TestNormalizeBeehiveNBT_CapitalBeesEmpty(t *testing.T) {
	// Java 格式大写 Bees 但为空列表，应清除
	nbt := map[string]any{
		"Bees":       []any{},
		"HoneyLevel": int32(0),
		"id":         "minecraft:beehive",
	}
	out := NormalizeBlockEntityNBT("minecraft:beehive", nbt)

	if _, ok := out["Bees"]; ok {
		t.Errorf("expected 'Bees' to be removed for empty list, but found: %v", out["Bees"])
	}
	if _, ok := out["HoneyLevel"]; ok {
		t.Errorf("expected 'HoneyLevel' to be removed")
	}
}

func TestNormalizeBeehiveNBT_PreservesOtherFields(t *testing.T) {
	// 非蜜蜂的其他字段应该被保留
	nbt := map[string]any{
		"bees":          []any{},
		"custom_field":  "hello",
		"some_data":     int32(42),
	}
	out := NormalizeBlockEntityNBT("minecraft:beehive", nbt)

	if v, ok := out["custom_field"]; !ok || v != "hello" {
		t.Errorf("expected 'custom_field' to be preserved, got %v", out["custom_field"])
	}
	if v, ok := out["some_data"]; !ok || v != int32(42) {
		t.Errorf("expected 'some_data' to be preserved, got %v", out["some_data"])
	}
}

func TestNormalizeBeehiveNBT_NoBeesField(t *testing.T) {
	// 完全没有 bees 字段的蜂巢 → 清除蜂蜜相关字段
	nbt := map[string]any{
		"honey_level":   int32(0),
		"HoneyLevel":    int32(3),
		"id":            "Beehive",
	}
	out := NormalizeBlockEntityNBT("minecraft:beehive", nbt)

	if _, ok := out["honey_level"]; ok {
		t.Errorf("expected 'honey_level' to be removed when no bees field present")
	}
}

func TestNormalizeBeehiveNBT_Occupants(t *testing.T) {
	// Occupants 字段也要清除（部分版本用此字段）
	nbt := map[string]any{
		"Occupants":    []any{},
		"honey_level":  int32(0),
		"id":           "Beehive",
	}
	out := NormalizeBlockEntityNBT("minecraft:bee_nest", nbt)

	if _, ok := out["Occupants"]; ok {
		t.Errorf("expected 'Occupants' to be removed for empty list")
	}
}
