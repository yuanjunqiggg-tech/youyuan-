package main

import (
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
)

// Regression for bug #5: Bedrock .mcworld/.mcstructure block entities store keys in lowercase
// (structureName, dataField, xStructureSize, xStructureOffset, ...). The importer previously
// read capitalized keys and never populated Settings, so structure blocks imported with the
// correct mode but empty name AND empty size/offset. buildStructureBlockUpdate must carry the
// full data.
func TestBuildStructureBlockUpdate_BedrockLowercase(t *testing.T) {
	nbt := map[string]any{
		"id":               "StructureBlock",
		"structureName":    "mystructure:house",
		"dataField":        "meta1",
		"includePlayers":   byte(1),
		"showBoundingBox":  byte(1),
		"ignoreEntities":   byte(1),
		"data":             int32(1),
		"xStructureSize":   int32(6),
		"yStructureSize":   int32(9),
		"zStructureSize":   int32(14),
		"xStructureOffset": int32(1),
		"yStructureOffset": int32(-1),
		"zStructureOffset": int32(0),
		"rotation":         byte(2),
		"mirror":           byte(1),
		"integrity":        float32(100),
		"seed":             int64(3003257088),
	}
	u := buildStructureBlockUpdate(nbt)
	if u.StructureName != "mystructure:house" {
		t.Errorf("StructureName: got %q", u.StructureName)
	}
	if u.DataField != "meta1" {
		t.Errorf("DataField: got %q", u.DataField)
	}
	if !u.IncludePlayers || !u.ShowBoundingBox {
		t.Errorf("include/show: got %v/%v", u.IncludePlayers, u.ShowBoundingBox)
	}
	if u.StructureBlockType != 2 {
		t.Errorf("StructureBlockType: got %d, want 2 (data=1)", u.StructureBlockType)
	}
	if u.Settings.Size != (protocol.BlockPos{6, 9, 14}) {
		t.Errorf("Size: got %v, want [6 9 14]", u.Settings.Size)
	}
	if u.Settings.Offset != (protocol.BlockPos{1, -1, 0}) {
		t.Errorf("Offset: got %v, want [1 -1 0]", u.Settings.Offset)
	}
	if u.Settings.Rotation != 2 {
		t.Errorf("Rotation: got %d, want 2", u.Settings.Rotation)
	}
	if u.Settings.Mirror != 1 {
		t.Errorf("Mirror: got %d, want 1", u.Settings.Mirror)
	}
	if !u.Settings.IgnoreEntities {
		t.Error("IgnoreEntities: got false, want true")
	}
	if u.Settings.Integrity != 1 {
		t.Errorf("Integrity: got %v, want 1 (100 percent -> 1.0)", u.Settings.Integrity)
	}
	if u.Settings.Seed != 3003257088 {
		t.Errorf("Seed: got %d, want 3003257088", u.Settings.Seed)
	}
}

// Java-source structures fall back to name/metadata keys.
func TestBuildStructureBlockUpdate_JavaFallback(t *testing.T) {
	nbt := map[string]any{"name": "minecraft:base", "metadata": "meta2"}
	u := buildStructureBlockUpdate(nbt)
	if u.StructureName != "minecraft:base" {
		t.Errorf("name fallback: got %q", u.StructureName)
	}
	if u.DataField != "meta2" {
		t.Errorf("metadata fallback: got %q", u.DataField)
	}
}
