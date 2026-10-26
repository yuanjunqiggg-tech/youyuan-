package main

import (
	"bytes"
	"compress/gzip"
	"testing"

	"bot-apk/building"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/nbt"
)

// TestSchematicExportRoundTrip 验证导出（BlockData 为 VarInt 编码 []byte）后的 .schematic
// 能被 ParseSchematic 重新解析，不再报"缺少 BlockData/Data"。
func TestSchematicExportRoundTrip(t *testing.T) {
	w, h, l := 3, 2, 1
	// palette: air=0, stone=1
	palette := map[string]int32{"minecraft:air": 0, "minecraft:stone": 1}
	// spongeIdx = y*w*l + z*w + x；按 y,z,x 序：[0 1 0 | 1 1 1]
	indices := []int32{0, 1, 0, 1, 1, 1}
	root := map[string]any{
		"Version": int32(2),
		"Width":   int16(w), "Height": int16(h), "Length": int16(l),
		"BlockData": encodeVarInts(indices),
		"Palette":   palette,
		"Offset":    [3]int32{0, 0, 0},
	}

	var buf bytes.Buffer
	enc := nbt.NewEncoderWithEncoding(&buf, nbt.BigEndian)
	if err := enc.Encode(root); err != nil {
		t.Fatalf("encode nbt: %v", err)
	}
	var gzbuf bytes.Buffer
	gw := gzip.NewWriter(&gzbuf)
	if _, err := gw.Write(buf.Bytes()); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	gw.Close()

	sd, err := building.ParseSchematic(gzbuf.Bytes())
	if err != nil {
		t.Fatalf("ParseSchematic 往返失败: %v", err)
	}
	if sd.SizeX != w || sd.SizeY != h || sd.SizeZ != l {
		t.Fatalf("尺寸不匹配: got %d×%d×%d want %d×%d×%d", sd.SizeX, sd.SizeY, sd.SizeZ, w, h, l)
	}
	// 应解析出 4 个 stone（索引 1,3,4,5）
	stoneCount := 0
	for _, name := range sd.Palette {
		if name.Name == "minecraft:stone" {
			stoneCount++
		}
	}
	if stoneCount != 1 {
		t.Fatalf("palette 应含 1 种 stone，got %d", stoneCount)
	}
}
