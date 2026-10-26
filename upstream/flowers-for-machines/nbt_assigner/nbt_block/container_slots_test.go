package nbt_block

import (
	block "github.com/OmineDev/flowers-for-machines/nbt_parser/block"
	item "github.com/OmineDev/flowers-for-machines/nbt_parser/item"
	"testing"
)

func TestOrdinarySlotsMustMatchEvenWhenContainerIsNotEmpty(t *testing.T) {
	want := block.Container{DefaultBlock: block.DefaultBlock{Name: "minecraft:chest"}}
	want.NBT.Items = []block.ItemWithSlot{
		{Slot: 0, Item: &item.DefaultItem{Basic: item.ItemBasicData{Name: "minecraft:stone", Count: 3}}},
		{Slot: 4, Item: &item.DefaultItem{Basic: item.ItemBasicData{Name: "minecraft:diamond", Count: 2}}},
	}
	row := func(slot, count byte, name string) any {
		return map[string]any{"Slot": slot, "Count": count, "Name": name, "Damage": int16(0)}
	}
	for _, tc := range []struct {
		name    string
		items   []any
		missing int
	}{
		{"empty", nil, 2},
		{"partially filled", []any{row(0, 3, "minecraft:stone")}, 1},
		{"wrong count", []any{row(0, 1, "minecraft:stone"), row(4, 2, "minecraft:diamond")}, 1},
		{"wrong slot", []any{row(0, 3, "minecraft:stone"), row(5, 2, "minecraft:diamond")}, 1},
		{"wrong item", []any{row(0, 3, "minecraft:dirt"), row(4, 2, "minecraft:diamond")}, 1},
		{"complete unordered", []any{row(4, 2, "minecraft:diamond"), row(0, 3, "minecraft:stone")}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := missingOrdinaryContainerItems(want, map[string]any{"Items": tc.items})
			if err != nil || len(got) != tc.missing {
				t.Fatalf("got %v %v", got, err)
			}
		})
	}
	if len(want.NBT.Items) != 2 {
		t.Fatal("readback modified request")
	}
}
