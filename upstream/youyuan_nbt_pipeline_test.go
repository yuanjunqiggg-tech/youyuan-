package main

import (
	"bot-apk/building"
	"bytes"
	"encoding/json"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/nbt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	nbtHash "github.com/OmineDev/flowers-for-machines/nbt_parser/hash"
	nbtInterface "github.com/OmineDev/flowers-for-machines/nbt_parser/interface"
	item "github.com/OmineDev/flowers-for-machines/nbt_parser/item"
	"os"
	"testing"
)

func yyTestContainer(t *testing.T) map[string]any {
	t.Helper()
	var rows []any
	if e := json.Unmarshal([]byte(`[{"Name":"diamond_sword","Count":1,"Damage":0,"Slot":2,"tag":{"display":{"Name":"游园 · 测试剑"},"ench":[{"id":9,"lvl":2}],"Damage":7}}]`), &rows); e != nil {
		t.Fatal(e)
	}
	v, e := yyContainerNBT("barrel", rows, map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestYYNBTContainerTypedRoundTrip(t *testing.T) {
	v := yyTestContainer(t)
	raw := v["Items"].([]any)[0].(map[string]any)
	if _, ok := raw["Count"].(byte); !ok {
		t.Fatalf("Count %T", raw["Count"])
	}
	var b bytes.Buffer
	if e := nbt.NewEncoderWithEncoding(&b, nbt.LittleEndian).Encode(v); e != nil {
		t.Fatal(e)
	}
	var actual map[string]any
	if e := nbt.NewDecoderWithEncoding(&b, nbt.LittleEndian).Decode(&actual); e != nil {
		t.Fatal(e)
	}
	var d item.DefaultItem
	if e := d.ParseNormal(actual["Items"].([]any)[0].(map[string]any)); e != nil {
		t.Fatal(e)
	}
	if d.Enhance.DisplayName != "游园 · 测试剑" || len(d.Enhance.EnchList) != 1 || d.Enhance.EnchList[0].Level != 2 {
		t.Fatalf("lost item enhancement: %+v", d)
	}
	if !yyNBTContains(actual, v) {
		t.Fatal("readback comparison failed")
	}
}
func TestYYNBTRejectInvalidItems(t *testing.T) {
	for _, raw := range []string{`[{"Name":"stone","Count":1.5}]`, `[{"Name":"stone","Count":256}]`, `[{"Name":"stone","Slot":27}]`, `[{"Name":"stone","Slot":1},{"Name":"stone","Slot":1}]`, `[{"Name":"sword","tag":{"ench":[{"id":9,"lvl":40000}]}}]`} {
		var rows []any
		json.Unmarshal([]byte(raw), &rows)
		if _, e := yyContainerNBT("chest", rows, nil); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, e := yyNormalizeNBT([]any{nil}); e == nil {
		t.Fatal("accepted null list entry")
	}
	if _, e := yyNBTCall(nil, "building.containerData", []any{"barrel", map[string]any{}}); e != nil {
		t.Fatal("empty Lua Items should work", e)
	}
}

func TestYYNBTShulkerFacingSurvivesJSONAndDefaultsUp(t *testing.T) {
	for _, name := range []string{"purple_shulker_box", "undyed_shulker_box"} {
		data, err := yyContainerNBT(name, nil, nil)
		if err != nil || data["facing"] != byte(1) {
			t.Fatal(data, err)
		}
		data, err = yyContainerNBT(name, nil, map[string]any{"facing": float64(5)})
		if err != nil || data["facing"] != byte(5) {
			t.Fatal(data, err)
		}
		if _, err := yyContainerNBT(name, nil, map[string]any{"facing": 6}); err == nil {
			t.Fatal("accepted invalid facing")
		}
	}
	data, err := yyNormalizeNBT(map[string]any{"facing": float64(1)})
	if err != nil || data.(map[string]any)["facing"] != byte(1) {
		t.Fatal(data, err)
	}
}

func TestYYNBTImportedBuildingServerExportFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/nbt-import-roundtrip-20261025.mcstructure")
	if err != nil {
		t.Fatal(err)
	}
	d, err := building.ParseMCStructure(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.SizeX != 5 || d.SizeY != 1 || d.SizeZ != 1 || d.ComputeStats().SolidBlocks != 3 || len(d.NBTBlocks) != 3 {
		t.Fatal("server round trip lost block entities")
	}
	plain := d.Get(0, 0, 0)
	if !yyNBTContains(plain.NBT, map[string]any{"Items": []any{map[string]any{"Slot": 0, "Name": "minecraft:stone", "Count": 3}, map[string]any{"Slot": 4, "Name": "minecraft:diamond", "Count": 2}}}) {
		t.Fatal("ordinary slots lost", plain.NBT)
	}
	box := d.Get(2, 0, 0)
	if box.Name != "minecraft:purple_shulker_box" || !yyNBTContains(box.NBT, map[string]any{"facing": 1, "Items": []any{map[string]any{"Slot": 4, "Name": "minecraft:diamond_sword", "Count": 1, "tag": map[string]any{"display": map[string]any{"Name": "游园 NBT 文件导入测试"}, "ench": []any{map[string]any{"id": 9, "lvl": 1}}}}}}) {
		t.Fatal("shulker item enhancement lost", box.NBT)
	}
	head := d.Get(4, 0, 0)
	parsed, err := nbtInterface.ParseBlock(nil, head.Name, map[string]any{}, head.NBT)
	if err != nil || parsed.BlockName() != "minecraft:wither_skeleton_skull" {
		t.Fatal("skull subtype lost", head, err)
	}
}
func TestYYNBTReadbackRequiresEnchantAndName(t *testing.T) {
	v := yyTestContainer(t)
	broken := yyTestContainer(t)
	item := broken["Items"].([]any)[0].(map[string]any)
	delete(item["tag"].(map[string]any), "ench")
	if yyNBTContains(broken, v) {
		t.Fatal("submitted container without ench must not be verified")
	}
}
func yyTestRegionChunk(tile yyRegionTile, name string, entity map[string]any) map[string]any {
	n := int(tile.Size[0] * tile.Size[1] * tile.Size[2])
	indices := make([]int32, n)
	second := make([]int32, n)
	for i := range second {
		second[i] = -1
	}
	data := map[string]any{}
	if entity != nil {
		data["0"] = map[string]any{"block_entity_data": entity}
	}
	return map[string]any{"size": []int32{tile.Size[0], tile.Size[1], tile.Size[2]}, "_yy_offset": tile.Offset, "_yy_size": tile.Size, "structure": map[string]any{"block_indices": []any{indices, second}, "palette": map[string]any{"default": map[string]any{"block_palette": []any{map[string]any{"name": name, "states": map[string]any{"facing_direction": int32(2)}}}, "block_position_data": data}}}}
}
func TestYYRegionTilesCover3DEdgesOnce(t *testing.T) {
	p := [3]int32{-22, -5, 39}
	s := [3]int32{33, 129, 17}
	seen := map[[3]int32]bool{}
	for _, v := range yyRegionTiles(p, s) {
		for x := int32(0); x < v.Size[0]; x++ {
			for y := int32(0); y < v.Size[1]; y++ {
				for z := int32(0); z < v.Size[2]; z++ {
					k := [3]int32{v.Offset[0] + x, v.Offset[1] + y, v.Offset[2] + z}
					if seen[k] {
						t.Fatal("overlap", k)
					}
					seen[k] = true
				}
			}
		}
	}
	if len(seen) != 33*129*17 {
		t.Fatal("missing cells", len(seen))
	}
}
func TestYYMergeUnordered3DAndMissingTile(t *testing.T) {
	s := [3]int32{17, 129, 1}
	tiles := yyRegionTiles([3]int32{}, s)
	chunks := []map[string]any{}
	for i := len(tiles) - 1; i >= 0; i-- {
		chunks = append(chunks, yyTestRegionChunk(tiles[i], "minecraft:barrel", yyTestContainer(t)))
	}
	merged := mergeChunkStructures(chunks, s[0], s[1], s[2])
	if merged == nil {
		t.Fatal("unordered tiles failed")
	}
	pd := yyStructurePalette(merged)["block_position_data"].(map[string]any)
	if len(pd) != 4 || pd["2192"] == nil {
		t.Fatal("Y offset or NBT index lost", pd)
	}
	if mergeChunkStructures(chunks[1:], s[0], s[1], s[2]) != nil {
		t.Fatal("missing tile reported complete")
	}
	if mergeChunkStructures(append(chunks[:3], chunks[0]), s[0], s[1], s[2]) != nil {
		t.Fatal("overlap accepted")
	}
}
func TestYYSchematicKeepsNamedEnchantedContainer(t *testing.T) {
	tile := yyRegionTile{Size: [3]int32{1, 1, 1}}
	chunk := yyTestRegionChunk(tile, "minecraft:barrel", yyTestContainer(t))
	merged := mergeChunkStructures([]map[string]any{chunk}, 1, 1, 1)
	data := convertToSchematic(map[string]any{"structure": merged}, 1, 1, 1)
	if len(data) == 0 {
		t.Fatal("encoding failed")
	}
	st, e := building.ParseSchematic(data)
	if e != nil {
		t.Fatal(e)
	}
	if len(st.NBTBlocks) != 1 {
		t.Fatal("lost block entity")
	}
	for _, v := range st.NBTBlocks {
		if !yyNBTContains(v, yyTestContainer(t)) {
			t.Fatal("lost name/enchant")
		}
	}
}
func TestYYNBTMethodsInstalledForLuaAndSDK(t *testing.T) {
	L := newLuaState(nil)
	defer L.Close()
	defer cancelLuaCtx(L)
	if e := L.DoString(`local x=building.containerData("barrel", {{Name="diamond_sword",tag={display={Name="常驻剑"},ench={{id=9,lvl=1}}}}});assert(x.Items[1].Name=="minecraft:diamond_sword");assert(building.planRegion({0,0,0},{17,1,1}).total==2)`); e != nil {
		t.Fatal(e)
	}
	for name := range yyNBTMethods {
		if yyExtraMethods[name] == "" {
			t.Fatal("missing catalog", name)
		}
	}
}

// Captured by the real Linux bot: 33x1x1 region exported from three tiles.
func TestYYServerExportFixtureRetainsContainerItems(t *testing.T) {
	raw, e := os.ReadFile("testdata/nbt-named-enchanted-barrel.mcstructure")
	if e != nil {
		t.Fatal(e)
	}
	s, e := building.ParseMCStructure(raw)
	if e != nil {
		t.Fatal(e)
	}
	if s.SizeX != 33 || s.SizeY != 1 || s.SizeZ != 1 {
		t.Fatal("wrong merged dimensions")
	}
	plain := s.Get(2, 0, 0)
	if plain == nil || !yyNBTContains(plain.NBT, map[string]any{"Items": []any{map[string]any{"Slot": 0, "Name": "minecraft:stone", "Count": 3}}}) {
		t.Fatal("plain container lost")
	}
	sword := s.Get(3, 0, 0)
	if sword == nil || !yyNBTContains(sword.NBT, map[string]any{"Items": []any{map[string]any{"Slot": 0, "Name": "minecraft:diamond_sword", "Count": 1, "tag": map[string]any{"display": map[string]any{"Name": "游园 · NBT实测剑"}, "ench": []any{map[string]any{"id": 9, "lvl": 1}}}}}}) {
		t.Fatal("named enchanted sword lost during export")
	}
	p, e := nbtInterface.ParseBlock(nil, plain.Name, map[string]any{}, plain.NBT)
	if e != nil {
		t.Fatal(e)
	}
	v, e := nbtInterface.ParseBlock(nil, sword.Name, map[string]any{}, sword.NBT)
	if e != nil {
		t.Fatal(e)
	}
	if nbtHash.NBTBlockFullHash(p) == nbtHash.NBTBlockFullHash(v) {
		t.Fatal("different container cache keys collided")
	}
}

func TestYYCaptureControlDoesNotDumpPacketHistory(t *testing.T) {
	b := senseBot(t)
	if _, e := yyCaptureStart(b, yyCaptureConfig{Types: []string{"ActorEvent"}}); e != nil {
		t.Fatal(e)
	}
	yyCapturePacket(b, &packet.ActorEvent{EntityRuntimeID: 42, EventType: packet.ActorEventHurt})
	if got := yyCaptureStatus(b); len(got["records"].([]map[string]any)) != 0 || got["retained_count"] != 1 {
		t.Fatal("control returned packet payload", got)
	}
	if got := yyCaptureRead(b, 0, 1); len(got["records"].([]map[string]any)) != 1 {
		t.Fatal("explicit read lost packets")
	}
}
