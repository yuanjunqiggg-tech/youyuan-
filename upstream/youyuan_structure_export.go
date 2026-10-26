package main

import (
	"bot-apk/building"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/nbt"
	"github.com/andybalholm/brotli"
	"sort"
	"strings"
)

// BDXConverter operations 5/41 use a constant pool and BE IDs; block NBT is LE.
func yyStructureBDX(s *building.StructureData) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("BDX\x00YouYuan\x00")
	pool := map[string]uint16{}
	constant := func(value string) (uint16, error) {
		if id, ok := pool[value]; ok {
			return id, nil
		}
		if len(pool) >= 65536 || strings.ContainsRune(value, 0) {
			return 0, fmt.Errorf("BDX字符串池超限或含空字符")
		}
		id := uint16(len(pool))
		pool[value] = id
		out.WriteByte(1)
		out.WriteString(value)
		out.WriteByte(0)
		return id, nil
	}
	pos := [3]int{}
	put := func(p [3]int, name, states string, data map[string]any) error {
		bid, e := constant(name)
		if e != nil {
			return e
		}
		sid, e := constant(states)
		if e != nil {
			return e
		}
		for axis := 0; axis < 3; axis++ {
			delta := p[axis] - pos[axis]
			if delta != 0 {
				out.WriteByte(byte(21 + axis*2))
				binary.Write(&out, binary.BigEndian, int32(delta))
				pos[axis] = p[axis]
			}
		}
		if data == nil {
			out.WriteByte(5)
			binary.Write(&out, binary.BigEndian, bid)
			binary.Write(&out, binary.BigEndian, sid)
		} else {
			var encoded bytes.Buffer
			if e := nbt.NewEncoderWithEncoding(&encoded, nbt.LittleEndian).Encode(data); e != nil {
				return e
			}
			out.WriteByte(41)
			binary.Write(&out, binary.BigEndian, bid)
			binary.Write(&out, binary.BigEndian, sid)
			binary.Write(&out, binary.BigEndian, sid)
			out.Write(encoded.Bytes())
		}
		return nil
	}
	// BDX has no explicit dimensions; air corners retain empty borders before actual blocks.
	for _, p := range [][3]int{{0, 0, 0}, {s.SizeX - 1, s.SizeY - 1, s.SizeZ - 1}} {
		if e := put(p, "minecraft:air", "[]", nil); e != nil {
			return nil, e
		}
	}
	blocks := s.AllBlocks()
	sort.Slice(blocks, func(i, j int) bool {
		a, b := blocks[i], blocks[j]
		if a.X != b.X {
			return a.X < b.X
		}
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.Z < b.Z
	})
	for _, b := range blocks {
		info := s.Palette[b.Index]
		if e := put([3]int{int(b.X), int(b.Y), int(b.Z)}, info.Name, info.States, s.GetNBT(int(b.X), int(b.Y), int(b.Z))); e != nil {
			return nil, e
		}
	}
	out.WriteByte(88)
	var compressed bytes.Buffer
	compressed.WriteString("BD@")
	w := brotli.NewWriterLevel(&compressed, 4)
	if _, e := w.Write(out.Bytes()); e != nil {
		return nil, e
	}
	if e := w.Close(); e != nil {
		return nil, e
	}
	return compressed.Bytes(), nil
}

func yyStructureSchem(s *building.StructureData) ([]byte, error) {
	if s.SizeX > 32767 || s.SizeY > 32767 || s.SizeZ > 32767 {
		return nil, fmt.Errorf("Schem每个轴最多32767方块")
	}
	raw, e := yyStructureEncode(s)
	if e != nil {
		return nil, e
	}
	var root map[string]any
	if e = nbt.NewDecoderWithEncoding(bytes.NewReader(raw), nbt.LittleEndian).Decode(&root); e != nil {
		return nil, e
	}
	out := convertToSchematic(root, s.SizeX, s.SizeY, s.SizeZ)
	if len(out) == 0 {
		return nil, fmt.Errorf("Schem编码失败")
	}
	return out, nil
}
