package protocol

import (
	"bytes"
	"testing"
)

// writeDefNeteaseFull 按历史 Netease 全段格式写出单个 BiomeDefinition。
func writeDefNeteaseFull(w *Writer, d *BiomeDefinition) {
	w.Int16(&d.NameIndex)
	OptionalFunc(w, &d.BiomeID, w.Uint16)
	w.Float32(&d.Temperature)
	w.Float32(&d.Downfall)
	w.Float32(&d.RedSporeDensity)
	w.Float32(&d.BlueSporeDensity)
	w.Float32(&d.AshDensity)
	w.Float32(&d.WhiteAshDensity)
	w.Float32(&d.Depth)
	w.Float32(&d.Scale)
	w.Int32(&d.MapWaterColour)
	w.Bool(&d.Rain)
	w.Int32(&d.Dimension)
	w.String(&d.Vanilla)
	OptionalFunc(w, &d.Tags, func(s *[]uint16) {
		FuncSlice(w, s, w.Uint16)
	})
	OptionalMarshaler(w, &d.ChunkGeneration)
}

// writeDefVanilla 按原版（无 Netease 段）格式写出单个 BiomeDefinition。
func writeDefVanilla(w *Writer, d *BiomeDefinition) {
	w.Int16(&d.NameIndex)
	OptionalFunc(w, &d.BiomeID, w.Uint16)
	w.Float32(&d.Temperature)
	w.Float32(&d.Downfall)
	w.Float32(&d.RedSporeDensity)
	w.Float32(&d.BlueSporeDensity)
	w.Float32(&d.AshDensity)
	w.Float32(&d.WhiteAshDensity)
	w.Float32(&d.Depth)
	w.Float32(&d.Scale)
	w.Int32(&d.MapWaterColour)
	w.Bool(&d.Rain)
	OptionalFunc(w, &d.Tags, func(s *[]uint16) {
		FuncSlice(w, s, w.Uint16)
	})
	OptionalMarshaler(w, &d.ChunkGeneration)
}

func buildList(t *testing.T, defs []BiomeDefinition, strs []string, netease bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := NewWriter(&buf, 0)
	var count uint32 = uint32(len(defs))
	w.Varuint32(&count)
	for i := range defs {
		if netease {
			writeDefNeteaseFull(w, &defs[i])
		} else {
			writeDefVanilla(w, &defs[i])
		}
	}
	var strCount uint32 = uint32(len(strs))
	w.Varuint32(&strCount)
	for i := range strs {
		s := strs[i]
		w.String(&s)
	}
	return buf.Bytes()
}

func TestBiomeAdaptiveNeteaseFull(t *testing.T) {
	defs := []BiomeDefinition{{
		NameIndex:    3,
		BiomeID:      Option(uint16(7)),
		Temperature:  0.8, Downfall: 0.4, Depth: 0.1, Scale: 0.2,
		MapWaterColour: 0x3F76E4,
		Rain: true,
		Dimension: 2,
		Vanilla:   "minecraft:forest",
		Tags:      Option([]uint16{1, 5}),
	}}
	strs := []string{"forest", "tag_a", "tag_b"}
	payload := buildList(t, defs, strs, true)

	r := NewReader(bytes.NewBuffer(payload), 0, false)
	gotDefs, gotStrs, ok := ReadBiomeDefinitionListAdaptive(r)
	if !ok {
		t.Fatalf("adaptive parse failed on Netease-full payload")
	}
	if len(gotDefs) != 1 {
		t.Fatalf("defs = %d, want 1", len(gotDefs))
	}
	if gotDefs[0].Dimension != 2 || gotDefs[0].Vanilla != "minecraft:forest" {
		t.Fatalf("Netease segment lost: %+v", gotDefs[0])
	}
	if id, set := gotDefs[0].BiomeID.Value(); !set || id != 7 {
		t.Fatalf("biome id wrong: set=%v id=%d", set, id)
	}
	if len(gotStrs) != 3 || gotStrs[0] != "forest" {
		t.Fatalf("strings wrong: %v", gotStrs)
	}
	if r.BufferedLen() != 0 {
		t.Fatalf("payload not drained: %d left", r.BufferedLen())
	}
}

func TestBiomeAdaptiveVanillaNoNetease(t *testing.T) {
	defs := []BiomeDefinition{{
		NameIndex:    0,
		Temperature:  0.5, Downfall: 0.0, Depth: 0.0, Scale: 0.0,
		MapWaterColour: 0,
		Rain: false,
	}}
	strs := []string{"plains"}
	payload := buildList(t, defs, strs, false)

	r := NewReader(bytes.NewBuffer(payload), 0, false)
	gotDefs, gotStrs, ok := ReadBiomeDefinitionListAdaptive(r)
	if !ok {
		t.Fatalf("adaptive parse failed on vanilla payload")
	}
	if len(gotDefs) != 1 || gotDefs[0].NameIndex != 0 {
		t.Fatalf("defs wrong: %+v", gotDefs)
	}
	if len(gotStrs) != 1 || gotStrs[0] != "plains" {
		t.Fatalf("strings wrong: %v", gotStrs)
	}
	if r.BufferedLen() != 0 {
		t.Fatalf("payload not drained: %d left", r.BufferedLen())
	}
}

func TestBiomeAdaptiveGarbageFails(t *testing.T) {
	// 随机垃圾必须所有候选全败（ok=false），而不是误报精确耗尽。
	garbage := []byte{0xFF, 0xFF, 0xFF, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A}
	r := NewReader(bytes.NewBuffer(garbage), 0, false)
	if _, _, ok := ReadBiomeDefinitionListAdaptive(r); ok {
		t.Fatalf("garbage payload must not parse")
	}
}

func TestGameRulesAdaptiveVanilla(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, 0)
	var count uint32 = 2
	w.Varuint32(&count)
	name := "showcoordinates"
	w.String(&name)
	var canMod = true
	w.Bool(&canMod)
	var t1 uint32 = 1
	w.Varuint32(&t1)
	var v1 = true
	w.Bool(&v1)
	name2 := "maxplayers"
	w.String(&name2)
	var canMod2 = false
	w.Bool(&canMod2)
	var t2 uint32 = 2
	w.Varuint32(&t2)
	var v2 int32 = 40
	w.Varint32(&v2)

	r := NewReader(bytes.NewBuffer(buf.Bytes()), 0, false)
	rules, ok := ReadGameRulesAdaptive(r)
	if !ok {
		t.Fatalf("adaptive game rules parse failed")
	}
	if len(rules) != 2 || rules[0].Name != "showcoordinates" || rules[1].Name != "maxplayers" {
		t.Fatalf("rules wrong: %+v", rules)
	}
	if r.BufferedLen() != 0 {
		t.Fatalf("payload not drained")
	}
}

func TestGameRulesAdaptiveNetease100(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, 0)
	var count uint32 = 1
	w.Varuint32(&count)
	name := "netease_custom_rule"
	w.String(&name)
	var canMod = true
	w.Bool(&canMod)
	var t1 uint32 = 100
	w.Varuint32(&t1)
	var v1 uint32 = 123456
	w.Varuint32(&v1)

	r := NewReader(bytes.NewBuffer(buf.Bytes()), 0, false)
	rules, ok := ReadGameRulesAdaptive(r)
	if !ok {
		t.Fatalf("adaptive game rules (type 100) parse failed")
	}
	if len(rules) != 1 || rules[0].Name != "netease_custom_rule" {
		t.Fatalf("rules wrong: %+v", rules)
	}
	if r.BufferedLen() != 0 {
		t.Fatalf("payload not drained")
	}
}
