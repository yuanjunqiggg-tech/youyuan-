package building

import (
	"bytes"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

// makeRuntimeCmdBDX 构造一个 BDXConverter 格式的 BDX：
// 用 runtime ID 放置命令方块 (bdxPlaceRuntimeBlock, 0x20)，
// runtime ID 3584 = bdxRuntimeIDs[3584] = ['command_block', 2]（data 2 = 朝北）。
func makeRuntimeCmdBDX(rid uint16) []byte {
	var inner []byte
	inner = append(inner, "BDX\x00"...)
	inner = append(inner, "test\x00"...)
	// 0x1f UseRuntimeIDPool: count=1, rtid=rid
	inner = append(inner, 0x1f)
	inner = append(inner, 0x00, 0x01) // count=1 (uint16BE)
	inner = append(inner, byte(rid>>8), byte(rid)) // runtimeID
	// 0x20 PlaceRuntimeBlock: rtid (索引到 pool 的第 0 项)
	inner = append(inner, 0x20)
	inner = append(inner, 0x00, 0x00)
	inner = append(inner, 0x58) // terminate

	var comp bytes.Buffer
	w := brotli.NewWriter(&comp)
	w.Write(inner)
	w.Close()
	return append([]byte("BD@"), comp.Bytes()...)
}

// TestBDXRuntimeCmdBlockFacing 验证 runtime ID 放置的命令方块能恢复朝向，
// 而不是默认掉成朝下（facing_direction=0）。
// 回归: 此前 bdx.go 只取 runtime ID 的方块名、丢弃 data value(朝向)，导致 states=[]。
func TestBDXRuntimeCmdBlockFacing(t *testing.T) {
	// rid 3584 = command_block, data 2 → facing_direction=2(北), conditional_bit=false
	sd, err := ParseBDX(makeRuntimeCmdBDX(3584))
	if err != nil {
		t.Fatalf("ParseBDX: %v", err)
	}
	for _, b := range sd.Palette {
		if b.Name == "minecraft:command_block" {
			if !strings.Contains(b.States, `"facing_direction"=2`) {
				t.Fatalf("命令方块朝向丢失: states=%q, 期望含 facing_direction=2", b.States)
			}
			t.Logf("OK: %s %s", b.Name, b.States)
			return
		}
	}
	t.Fatalf("未找到 command_block palette 条目")
}
