package main

import (
	"bot-apk/building"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/nbt"
	"regexp"
	"strings"
)

var yyConvertSelector = regexp.MustCompile(`^@[pares](\[[^\r\n]*\])?$`)
var yyConvertTriple = regexp.MustCompile(`^[012],[01],[01]$`)
var yyConvertBlockName = regexp.MustCompile(`^[a-z0-9_.:-]+$`)

func yyConvertControl(line string) bool {
	return line == "." || line == "!" || yyConvertTriple.MatchString(line) || strings.HasPrefix(line, "@") || strings.HasPrefix(line, "wait ") || strings.HasPrefix(line, "#组") || strings.HasPrefix(line, "#执行") || strings.HasPrefix(line, "#测试")
}

// The explicit envelope retains empty borders; legacy array exports are also readable.
func yyParseBlockJSON(raw []byte) (*building.StructureData, error) {
	type block struct {
		X, Y, Z      int
		Name, States string
		NBT          map[string]any
		NBTBase64    string `json:"nbt_base64"`
	}
	var doc struct {
		Size   [3]int
		Blocks []block
	}
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		if e := json.Unmarshal(raw, &doc.Blocks); e != nil {
			return nil, e
		}
	} else if e := json.Unmarshal(raw, &doc); e != nil {
		return nil, e
	}
	if len(doc.Blocks) > 131072 {
		return nil, fmt.Errorf("JSON 方块数超过 131072")
	}
	for _, b := range doc.Blocks {
		if b.X < 0 || b.Y < 0 || b.Z < 0 || b.X >= 131072 || b.Y >= 131072 || b.Z >= 131072 || !yyConvertBlockName.MatchString(b.Name) {
			return nil, fmt.Errorf("JSON 方块坐标或名称无效")
		}
		if _, e := yyStateNBT(b.States); e != nil {
			return nil, e
		}
		p := [3]int{b.X + 1, b.Y + 1, b.Z + 1}
		for i := range p {
			if doc.Size[i] == 0 {
				continue
			}
			if p[i] > doc.Size[i] {
				return nil, fmt.Errorf("方块超出 JSON size")
			}
		}
	}
	if doc.Size == [3]int{} {
		for _, b := range doc.Blocks {
			p := [3]int{b.X + 1, b.Y + 1, b.Z + 1}
			for i := range p {
				if p[i] > doc.Size[i] {
					doc.Size[i] = p[i]
				}
			}
		}
	}
	if doc.Size[0] < 1 || doc.Size[1] < 1 || doc.Size[2] < 1 || doc.Size[0] > 131072 || doc.Size[1] > 131072 || doc.Size[2] > 131072 || int64(doc.Size[0])*int64(doc.Size[1])*int64(doc.Size[2]) > 131072 {
		return nil, fmt.Errorf("JSON size 体积必须为 1..131072")
	}
	s := &building.StructureData{SizeX: doc.Size[0], SizeY: doc.Size[1], SizeZ: doc.Size[2], Blocks: map[uint64]uint32{}}
	for _, b := range doc.Blocks {
		if b.NBTBase64 != "" {
			encoded, e := base64.StdEncoding.DecodeString(b.NBTBase64)
			if e != nil {
				return nil, e
			}
			if e = nbt.NewDecoderWithEncoding(bytes.NewReader(encoded), nbt.LittleEndian).Decode(&b.NBT); e != nil {
				return nil, fmt.Errorf("方块实体NBT编码错误：%w", e)
			}
		}
		s.Set(b.X, b.Y, b.Z, &building.BlockInfo{Name: b.Name, States: b.States, NBT: b.NBT})
	}
	return s, nil
}

func yyStructureCommands(s *building.StructureData, origin [3]int, relative, merge, includeAir bool) (string, int) {
	var out strings.Builder
	out.WriteString("1,0,0\n")
	count := 0
	coord := func(x, y, z int) string {
		p := [3]int{x + origin[0], y + origin[1], z + origin[2]}
		if relative {
			return fmt.Sprintf("~%d ~%d ~%d", p[0], p[1], p[2])
		}
		return fmt.Sprintf("%d %d %d", p[0], p[1], p[2])
	}
	for y := 0; y < s.SizeY; y++ {
		for z := 0; z < s.SizeZ; z++ {
			for x := 0; x < s.SizeX; {
				idx := s.GetIndex(x, y, z)
				info := building.BlockInfo{Name: "minecraft:air"}
				if idx > 0 && int(idx) < len(s.Palette) {
					info = s.Palette[idx]
				} else if !includeAir {
					x++
					continue
				}
				end := x
				if merge && s.GetNBT(x, y, z) == nil {
					for end+1 < s.SizeX && end-x+1 < 32768 && s.GetIndex(end+1, y, z) == idx && s.GetNBT(end+1, y, z) == nil {
						end++
					}
				}
				if end > x {
					fmt.Fprintf(&out, "fill %s %s %s %s\n", coord(x, y, z), coord(end, y, z), info.Name, info.States)
				} else {
					fmt.Fprintf(&out, "setblock %s %s %s\n", coord(x, y, z), info.Name, info.States)
				}
				count++
				x = end + 1
			}
		}
	}
	return out.String(), count
}
