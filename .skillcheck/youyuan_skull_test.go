package main

import (
	"bot-apk/building"
	assigner "github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_block"
	nb "github.com/OmineDev/flowers-for-machines/nbt_parser/block"
	"testing"
)

func TestSkullParserAndImportDispatch(t *testing.T) {
	for typ := 0; typ <= 6; typ++ {
		data := map[string]any{"SkullType": float64(typ)}
		b, e := nb.ParseBlock(nil, "minecraft:skull", map[string]any{"facing_direction": int32(1)}, data)
		if e != nil {
			t.Fatal(e)
		}
		s, ok := b.(*nb.Skull)
		if !ok || int(s.NBT.SkullType) != typ || !assigner.NBTBlockIsSupported(b) || building.ClassifyBlock("minecraft:skull", data) != building.CatWorkspaceNBT {
			t.Fatalf("missing skull path type=%d block=%T", typ, b)
		}
	}
	// Real server readback: named head + SkullType=255 is not invalid NBT.
	b, err := nb.ParseBlock(nil, "minecraft:wither_skeleton_skull", map[string]any{"facing_direction": int32(1)}, map[string]any{"SkullType": byte(255)})
	if err != nil || b.(*nb.Skull).NBT.SkullType != 1 { t.Fatalf("modern sentinel: %v %v", b, err) }
	for _, v := range []any{-1, 7, 1.5, "2", nil} {
		s := new(nb.Skull)
		if s.Parse(map[string]any{"SkullType": v}) == nil {
			t.Fatalf("invalid type accepted: %v", v)
		}
	}
}
