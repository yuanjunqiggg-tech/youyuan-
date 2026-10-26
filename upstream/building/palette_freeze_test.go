package building

import "testing"

func TestFreezePalettePreparesExecutionWaterloggingVariant(t *testing.T) {
	d := importFixture(32, 16)
	name, states := "minecraft:oak_stairs", "[\"waterlogged\"=true,\"upside_down_bit\"=false]"
	d.Set(17, 0, 1, &BlockInfo{Name: name, States: states, NBT: map[string]any{"test": int32(7)}})
	original := d.GetIndex(17, 0, 1)
	d.FreezePalette()
	n := len(d.Palette)
	idx := d.getOrCreatePaletteIndex(name, stripWaterlogged(states))
	if len(d.Palette) != n || d.GetIndex(17, 0, 1) != original || d.GetNBT(17, 0, 1)["test"] != int32(7) {
		t.Fatal("execution still mutates palette or source changed")
	}
	if d.Palette[idx].IsDependent != d.Palette[original].IsDependent || d.Palette[idx].IsGravity != d.Palette[original].IsGravity {
		t.Fatal("variant lost placement category")
	}
	d.FreezePalette()
	if len(d.Palette) != n {
		t.Fatal("repeated freeze is not stable")
	}
}
