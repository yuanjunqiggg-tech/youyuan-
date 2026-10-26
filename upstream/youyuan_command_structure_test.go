package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestBuildingConvertersPreserveActualNBTFixtureAndEmptyBorders(t *testing.T) {
	raw, e := os.ReadFile("testdata/nbt-import-roundtrip-20261025.mcstructure")
	if e != nil {
		t.Fatal(e)
	}
	s, e := yyParseConversion(raw, ".mcstructure")
	if e != nil {
		t.Fatal(e)
	}
	for _, target := range []string{"bdx", "schem"} {
		out := conversionTestCall(t, "/api/youyuan/convert/structure", map[string]any{"data": base64.StdEncoding.EncodeToString(raw), "filename": "fixture.mcstructure", "target": target})
		if out["ok"] != true {
			t.Fatal(target, out)
		}
		data, _ := base64.StdEncoding.DecodeString(out["data"].(string))
		got, e := yyParseConversion(data, "."+target)
		if e != nil {
			t.Fatal(target, e)
		}
		if got.SizeX != s.SizeX || got.SizeY != s.SizeY || got.SizeZ != s.SizeZ || len(got.AllBlocks()) != len(s.AllBlocks()) {
			t.Fatal("shape lost", target, got)
		}
		for _, b := range s.AllBlocks() {
			n := got.GetNBT(int(b.X), int(b.Y), int(b.Z))
			want := s.GetNBT(int(b.X), int(b.Y), int(b.Z))
			if !reflect.DeepEqual(n, want) {
				t.Fatal("NBT changed", target, b, n, want)
			}
		}
	}
}

func TestCommandStructurePhysicalChain(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {2, 3}, {3, 2}, {16, 16}} {
		content := strings.Repeat("say 中文\n", 35)
		s, e := yyCommandStructure(content, size[0], size[1], 2)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := yyStructureEncode(s)
		if e != nil {
			t.Fatal(e)
		}
		parsed, e := yyParseConversion(raw, ".mcstructure")
		if e != nil {
			t.Fatal(e)
		}
		if len(parsed.AllBlocks()) != 35 {
			t.Fatal("lost blocks", len(parsed.AllBlocks()))
		}
		p := [3]int{}
		seen := map[[3]int]bool{}
		for i := 0; i < 35; i++ {
			if seen[p] {
				t.Fatal("chain cycle", p)
			}
			seen[p] = true
			idx := parsed.GetIndex(p[0], p[1], p[2])
			info := parsed.Palette[idx]
			data := parsed.GetNBT(p[0], p[1], p[2])
			if data["Command"] != "say 中文" || data["TickDelay"] != int32(2) {
				t.Fatal(p, data)
			}
			if i == 0 {
				if info.Name != "minecraft:command_block" || data["auto"] != byte(0) {
					t.Fatal(info, data)
				}
			} else if info.Name != "minecraft:chain_command_block" || data["auto"] != byte(1) {
				t.Fatal(info, data)
			}
			if i == 34 {
				break
			}
			states, e := yyStateNBT(info.States)
			if e != nil {
				t.Fatal(e)
			}
			switch states["facing_direction"] {
			case int32(1):
				p[1]++
			case int32(2):
				p[2]--
			case int32(3):
				p[2]++
			case int32(4):
				p[0]--
			case int32(5):
				p[0]++
			default:
				t.Fatal(states)
			}
		}
	}
}

func TestCommandStructureHeaderAndLossProtection(t *testing.T) {
	s, e := yyCommandStructure("2,0,0 20\ntestfor @a\n0,1,0\nsay ok\n.\n1,0,1\nsay pulse", 2, 2, 0)
	if e != nil {
		t.Fatal(e)
	}
	if s.GetNBT(0, 0, 0)["TickDelay"] != int32(1) {
		t.Fatal("Hz lost")
	}
	if s.GetNBT(0, 0, 1)["conditionalMode"] != byte(1) {
		t.Fatal("condition lost")
	}
	for _, text := range []string{"say a\n!\nsay b", "@plugin.call nope", "2,0,0 1000\nsay a"} {
		if _, e := yyCommandStructure(text, 2, 2, 0); e == nil {
			t.Fatal("silent loss", text)
		}
	}
}

func TestConversionPlacementTXT(t *testing.T) {
	raw := `setblock ~10 ~80 ~20 stone
fill ~11 ~80 ~20 ~12 ~80 ~20 oak_stairs ["upside_down_bit"=true,"weirdo_direction"=0]
setblock ~12 ~80 ~20 air`
	out := conversionTestCall(t, "/api/youyuan/convert/structure", map[string]any{"data": base64.StdEncoding.EncodeToString([]byte(raw)), "filename": "建筑.txt", "target": "mcstructure"})
	if out["ok"] != true {
		t.Fatal(out)
	}
	data, _ := base64.StdEncoding.DecodeString(out["data"].(string))
	s, e := yyParseConversion(data, ".mcstructure")
	if e != nil || s.SizeX != 3 || len(s.AllBlocks()) != 2 {
		t.Fatal(s, e)
	}
	for _, text := range []string{"say lost", "fill 0 0 0 30000000 30000000 30000000 stone", "setblock 0 0 0 stone destroy", "setblock ^ ^ ^ stone"} {
		if _, e := yyPlacementText([]byte(text)); e == nil {
			t.Fatal("unsafe/lost TXT accepted", text)
		}
	}
}

func TestConversionAnimationNaturalFrames(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, name := range []string{"frame10.txt", "frame2.txt", "frame1.txt"} {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		w.Write([]byte(name + "\n中文\"\\"))
	}
	z.Close()
	frames, e := yyTextFrames(b.Bytes(), "frames.zip")
	if e != nil || !strings.HasPrefix(frames[0], "frame1.txt") || !strings.HasPrefix(frames[1], "frame2.txt") {
		t.Fatal(frames, e)
	}
	out := conversionTestCall(t, "/api/youyuan/convert/command-structure", map[string]any{"data": base64.StdEncoding.EncodeToString(b.Bytes()), "filename": "frames.zip", "mode": "animation", "width": 2, "depth": 2, "delay": 2, "selector": "@a", "display": "actionbar"})
	if out["ok"] != true || out["frames"] != float64(3) {
		t.Fatal(out)
	}
	data, _ := base64.StdEncoding.DecodeString(out["data"].(string))
	s, e := yyParseConversion(data, ".mcstructure")
	if e != nil {
		t.Fatal(e)
	}
	cmd := s.GetNBT(0, 0, 0)["Command"].(string)
	if !strings.Contains(cmd, `titleraw @a actionbar`) || !strings.Contains(cmd, `frame1.txt\n`) {
		t.Fatal(cmd)
	}
}
