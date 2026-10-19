package nbt_block

import "testing"

func TestStructureNBTDoesNotInventMissingData(t *testing.T) {
	if _, err := simpleStructureNBT(map[string]any{}); err == nil {
		t.Fatal("missing structure must fail rather than look like empty NBT")
	}
	data := map[string]any{"structure": map[string]any{"palette": map[string]any{"default": map[string]any{}}}}
	got, err := simpleStructureNBT(data)
	if err != nil || len(got) != 0 {
		t.Fatalf("valid block without block entity: %v %v", got, err)
	}
	d := data["structure"].(map[string]any)["palette"].(map[string]any)["default"].(map[string]any)
	d["block_position_data"] = map[string]any{"0": map[string]any{"block_entity_data": map[string]any{"SkullType": byte(1), "Items": []any{map[string]any{"Count": byte(2), "Slot": byte(0)}}}}}
	got, err = simpleStructureNBT(data)
	if err != nil || got["SkullType"] != byte(1) {
		t.Fatalf("lost typed NBT: %v %v", got, err)
	}
}
