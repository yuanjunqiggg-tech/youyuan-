package main

import (
	"bot-apk/building"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFastDelayDoesNotCreateResponseDependency(t *testing.T) {
	p, _ := yyParseChains("say a\nsay b\n0,1,0\nsay conditional")
	var flags []bool
	yyChainExecuteBuffered(context.Background(), p, yyChainConfig{Mode: "fast"}, time.Millisecond, func(n yyChainNode) (bool, error) { flags = append(flags, n.RequireResult); return true, nil }, nil, func(bool, bool, error) {})
	if !reflect.DeepEqual(flags, []bool{false, true, true}) {
		t.Fatal("delays must not request replies; conditional predecessor must", flags)
	}
}

func TestBranchChoosesOneModuleAndReturns(t *testing.T) {
	for _, result := range []bool{false, true} {
		p, e := yyParseChains("@branch {\"command\":\"testfor @a\",\"then\":\"yes.txt\",\"else\":\"no.txt\"}\nsay return")
		if e != nil {
			t.Fatal(e)
		}
		want := "no.txt"
		if result {
			want = "yes.txt"
		}
		var got []string
		child, _ := yyParseChains("say child")
		e = yyChainExecuteGraph(context.Background(), p, yyChainConfig{Mode: "fast"}, nil, func(name, account string) (yyChainPlan, yyChainConfig, error) {
			if name != want {
				t.Fatalf("branch chose %s, want %s", name, want)
			}
			return child, yyChainConfig{Mode: "fast"}, nil
		}, func(_ context.Context, n yyChainNode, _ yyChainConfig) (bool, error) {
			if n.Command != "testfor @a" || !n.RequireResult {
				t.Fatal("branch condition did not request actual reply", n)
			}
			return result, nil
		}, func(_ context.Context, ns []yyChainNode, _ yyChainConfig) ([]map[string]any, error) {
			rows := []map[string]any{}
			for _, n := range ns {
				got = append(got, n.Command)
				rows = append(rows, map[string]any{"submitted": true})
			}
			return rows, nil
		}, func() bool { return false }, func(string) {}, func(bool, bool, error) {})
		if e != nil || !reflect.DeepEqual(got, []string{"say child", "say return"}) {
			t.Fatal(e, got)
		}
	}
}

func TestStructureChainsRespectDirectionAndPreserveWorldCoordinates(t *testing.T) {
	s := &building.StructureData{SizeX: 4, SizeY: 1, SizeZ: 2, Palette: []building.BlockInfo{{Name: "minecraft:air"}}, Blocks: map[uint64]uint32{}}
	put := func(x, y, z int, name, states, cmd string) {
		s.Set(x, y, z, &building.BlockInfo{Name: name, States: states, NBT: map[string]any{"Command": cmd, "auto": byte(1)}})
	}
	put(0, 0, 0, "minecraft:repeating_command_block", `["facing_direction"=5]`, "setblock ~ ~1 ~ stone")
	put(1, 0, 0, "minecraft:chain_command_block", `["facing_direction"=5,"conditional_bit"=true]`, "say second")
	put(2, 0, 0, "minecraft:command_block", `["facing_direction"=5]`, "tp @s 100 64 100")
	put(1, 0, 1, "minecraft:chain_command_block", `["facing_direction"=3]`, "say adjacent")
	out, e := yyStructureChains(s, "example", [3]int{10, 80, 20})
	if e != nil {
		t.Fatal(e)
	}
	files := out["files"].([]map[string]any)
	if len(files) != 3 {
		t.Fatalf("adjacent or impulse block incorrectly joined: %d", len(files))
	}
	first := files[0]["content"].(string)
	if !strings.Contains(first, "execute positioned 10 80 20 run setblock ~ ~1 ~ stone") || !strings.Contains(first, "0,1,0\nsay second") {
		t.Fatal(first)
	}
	var commands string
	for _, f := range files {
		commands += f["content"].(string)
	}
	if !strings.Contains(commands, "tp @s 100 64 100") {
		t.Fatal("functional world coordinates lost")
	}
	if files[0]["enabled"] != false {
		t.Fatal("converted files auto-enabled")
	}
}
