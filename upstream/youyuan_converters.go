package main

import (
	"bot-apk/building"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/nbt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var yyConvertMu sync.Mutex

func yyStateNBT(text string) (map[string]any, error) {
	states := map[string]any{}
	text = strings.TrimSpace(text)
	if text == "" || text == "[]" {
		return states, nil
	}
	text = "{" + strings.TrimSuffix(strings.TrimPrefix(text, "["), "]") + "}"
	text = strings.ReplaceAll(text, "=", ":")
	if e := json.Unmarshal([]byte(text), &states); e != nil {
		return nil, fmt.Errorf("无法转换方块状态 %s：%w", text, e)
	}
	for k, v := range states {
		switch n := v.(type) {
		case float64:
			states[k] = int32(n)
		case bool:
			if n {
				states[k] = byte(1)
			} else {
				states[k] = byte(0)
			}
		}
	}
	return states, nil
}
func yyStructureEncode(s *building.StructureData) ([]byte, error) {
	volume := s.SizeX * s.SizeY * s.SizeZ
	if volume < 1 || volume > 131072 {
		return nil, fmt.Errorf("转换体积必须为 1..131072")
	}
	index := make([]int32, volume)
	water := make([]int32, volume)
	for i := range index {
		index[i] = -1
		water[i] = -1
	}
	palette := []any{}
	for _, p := range s.Palette {
		states, e := yyStateNBT(p.States)
		if e != nil {
			return nil, e
		}
		name := p.Name
		if name == "" {
			name = "minecraft:air"
		}
		palette = append(palette, map[string]any{"name": name, "states": states, "version": int32(18168865)})
	}
	entities := map[string]any{}
	for x := 0; x < s.SizeX; x++ {
		for y := 0; y < s.SizeY; y++ {
			for z := 0; z < s.SizeZ; z++ {
				i := x*s.SizeY*s.SizeZ + y*s.SizeZ + z
				idx := s.GetIndex(x, y, z)
				index[i] = int32(idx)
				if data := s.GetNBT(x, y, z); data != nil {
					entities[fmt.Sprint(i)] = map[string]any{"block_entity_data": data}
				}
			}
		}
	}
	var out bytes.Buffer
	e := nbt.NewEncoderWithEncoding(&out, nbt.LittleEndian).Encode(map[string]any{"format_version": int32(1), "size": []int32{int32(s.SizeX), int32(s.SizeY), int32(s.SizeZ)}, "structure_world_origin": []int32{0, 0, 0}, "structure": map[string]any{"block_indices": []any{index, water}, "entities": []any{}, "palette": map[string]any{"default": map[string]any{"block_palette": palette, "block_position_data": entities}}}})
	return out.Bytes(), e
}
func yyParseConversion(data []byte, ext string) (*building.StructureData, error) {
	s, e := yyDecodeConversion(data, ext)
	if e != nil {
		return nil, e
	}
	if s.SizeX < 1 || s.SizeY < 1 || s.SizeZ < 1 || s.SizeX > 131072 || s.SizeY > 131072 || s.SizeZ > 131072 || int64(s.SizeX)*int64(s.SizeY)*int64(s.SizeZ) > 131072 {
		return nil, fmt.Errorf("转换体积必须为 1..131072")
	}
	clean := &building.StructureData{SizeX: s.SizeX, SizeY: s.SizeY, SizeZ: s.SizeZ, Blocks: map[uint64]uint32{}}
	for _, b := range s.AllBlocks() {
		if int(b.Index) >= len(s.Palette) {
			return nil, fmt.Errorf("方块调色板索引无效")
		}
		info := s.Palette[b.Index]
		if info.Name == "minecraft:air" || info.Name == "air" {
			continue
		}
		info.NBT = s.GetNBT(int(b.X), int(b.Y), int(b.Z))
		clean.Set(int(b.X), int(b.Y), int(b.Z), &info)
	}
	return clean, nil
}
func yyDecodeConversion(data []byte, ext string) (*building.StructureData, error) {
	switch ext {
	case ".json":
		return yyParseBlockJSON(data)
	case ".mcstructure":
		if len(data) > 2 && data[0] == 31 && data[1] == 139 {
			gz, e := gzip.NewReader(bytes.NewReader(data))
			if e != nil {
				return nil, e
			}
			data, e = io.ReadAll(io.LimitReader(gz, (32<<20)+1))
			gz.Close()
			if e != nil || len(data) > 32<<20 {
				return nil, fmt.Errorf("结构解压超限/失败")
			}
		}
		var root map[string]any
		if e := nbt.NewDecoderWithEncoding(bytes.NewReader(data), nbt.LittleEndian).Decode(&root); e != nil {
			return nil, e
		}
		size, ok := root["size"].([]int32)
		if !ok || len(size) != 3 || size[0] < 1 || size[1] < 1 || size[2] < 1 || int64(size[0])*int64(size[1])*int64(size[2]) > 131072 {
			return nil, fmt.Errorf("转换区域需在 131072 方块内；大建筑请用建筑管理")
		}
		return building.ParseMCStructure(data)
	case ".bdx":
		return building.ParseBDX(data)
	case ".schem", ".schematic":
		return building.ParseSchematic(data)
	}
	return nil, fmt.Errorf("支持 mcstructure/bdx/schem/schematic/json 输入")
}
func yyConverterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/convert/commands", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Content, Mode, Objective, Entity, Selector, Output string
			Start                                              int
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		content := strings.ReplaceAll(p.Content, "\r\n", "\n")
		if p.Mode == "execute" {
			if !yyConvertSelector.MatchString(p.Selector) {
				writeJSON(w, map[string]any{"ok": false, "error": "执行者需要 Bedrock 选择器，例如 @a 或 @e[tag=builder]"})
				return
			}
			var out strings.Builder
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if yyConvertControl(line) {
					writeJSON(w, map[string]any{"ok": false, "error": "包装模式只接受纯指令"})
					return
				}
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if yyConvertControl(line) {
					writeJSON(w, map[string]any{"ok": false, "error": "执行者包装仅接受纯指令；命令链属性与标记请使用直接方式"})
					return
				}
				fmt.Fprintf(&out, "execute as %s at @s run %s\n", p.Selector, strings.TrimPrefix(line, "/"))
			}
			content = out.String()
		} else if p.Mode == "scoreboard" {
			if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,16}$`).MatchString(p.Objective) || strings.TrimSpace(p.Entity) == "" || strings.ContainsAny(p.Entity, "\r\n") || p.Start < 0 || p.Start > 2147450000 {
				writeJSON(w, map[string]any{"ok": false, "error": "需要 1..16 字符计分板名与实体名"})
				return
			}
			var out strings.Builder
			fmt.Fprintf(&out, "# 请先建立计分板 %s，并由你的调度器更新实体分数\n", p.Objective)
			n := p.Start
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if yyConvertControl(line) {
					writeJSON(w, map[string]any{"ok": false, "error": "包装模式只接受纯指令"})
					return
				}
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if yyConvertControl(line) {
					writeJSON(w, map[string]any{"ok": false, "error": "计分板包装仅接受纯指令；命令链属性与标记请使用直接方式"})
					return
				}
				fmt.Fprintf(&out, "execute as @e[name=%s,scores={%s=%d}] at @s run %s\n", mustJSON(p.Entity), p.Objective, n, strings.TrimPrefix(line, "/"))
				n++
			}
			content = out.String()
		} else if p.Mode != "" && p.Mode != "direct" {
			writeJSON(w, map[string]any{"ok": false, "error": "mode 仅支持 direct/execute/scoreboard"})
			return
		}
		if p.Output == "mcfunction" {
			var out strings.Builder
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if yyConvertControl(line) {
					writeJSON(w, map[string]any{"ok": false, "error": "mcfunction 无法保存条件链/延迟/周期属性，请选择 TXT 命令链"})
					return
				}
				out.WriteString(strings.TrimPrefix(line, "/") + "\n")
			}
			content = out.String()
		} else if p.Output != "" && p.Output != "txt" {
			writeJSON(w, map[string]any{"ok": false, "error": "output 支持 txt/mcfunction"})
			return
		}
		plan, e := yyParseChains(content)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "content": content, "commands": plan.Commands})
	})
	mux.HandleFunc("/api/youyuan/convert/structure", func(w http.ResponseWriter, r *http.Request) {
		if !yyConvertMu.TryLock() {
			writeJSON(w, map[string]any{"ok": false, "error": "已有结构转换进行中"})
			return
		}
		defer yyConvertMu.Unlock()
		defer func() {
			if v := recover(); v != nil {
				writeJSON(w, map[string]any{"ok": false, "error": fmt.Sprint(v)})
			}
		}()
		var p struct {
			Data, Filename, Target      string
			Origin                      [3]int `json:"origin"`
			Rotation                    int    `json:"rotation"`
			Merge, Relative, IncludeAir bool
		}
		fail := func(e error) { writeJSON(w, map[string]any{"ok": false, "error": e.Error()}) }
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 48<<20)).Decode(&p); e != nil {
			fail(e)
			return
		}
		raw, e := base64.StdEncoding.DecodeString(p.Data)
		if e != nil || len(raw) > 32<<20 {
			fail(fmt.Errorf("文件编码错误或超过 32MiB"))
			return
		}
		s, e := yyParseConversion(raw, strings.ToLower(filepath.Ext(p.Filename)))
		if e != nil {
			fail(e)
			return
		}
		if s.SizeX < 1 || s.SizeY < 1 || s.SizeZ < 1 || int64(s.SizeX)*int64(s.SizeY)*int64(s.SizeZ) > 131072 {
			fail(fmt.Errorf("转换区域超过 131072 方块；请先裁切"))
			return
		}
		if p.Rotation != 0 && p.Rotation != 90 && p.Rotation != 180 && p.Rotation != 270 {
			fail(fmt.Errorf("旋转角度支持 0/90/180/270"))
			return
		}
		s = building.RotateBlocksInPlace(s, p.Rotation)
		name := strings.TrimSuffix(filepath.Base(p.Filename), filepath.Ext(p.Filename))
		result := map[string]any{"ok": true, "blocks": len(s.AllBlocks()), "size": []int{s.SizeX, s.SizeY, s.SizeZ}, "note": "仅方块与方块实体；游走实体、第二水层等不由此转换器还原。TXT 指令无法还原箱子物品/方块实体 NBT。"}
		result["palette"] = len(s.Palette)
		result["block_entities"] = len(s.NBTBlocks)
		switch p.Target {
		case "command_chains":
			bundle, e := yyStructureChains(s, name, p.Origin)
			if e != nil {
				fail(e)
				return
			}
			result["files"] = bundle["files"]
			result["chains"] = bundle["chains"]
			result["warnings"] = bundle["warnings"]
			encoded, _ := json.MarshalIndent(bundle, "", "  ")
			result["content"] = string(encoded)
			result["filename"] = name + "_命令链拓扑.json"
			result["note"] = "按朝向拆分命令方块；下载完整拓扑或保存为未启动的独立TXT。请审核动态接线、原生延迟与执行原点。"
		case "inspect":
			result["filename"] = name
		case "txt":
			content, count := yyStructureCommands(s, p.Origin, p.Relative, p.Merge, p.IncludeAir)
			if _, e := yyParseChains(content); e != nil {
				fail(e)
				return
			}
			result["content"] = content
			result["commands"] = count
			result["filename"] = name + ".txt"
		case "json":
			blocks := []map[string]any{}
			for _, b := range s.AllBlocks() {
				info := s.Palette[b.Index]
				block := map[string]any{"x": b.X, "y": b.Y, "z": b.Z, "name": info.Name, "states": info.States, "nbt": s.GetNBT(int(b.X), int(b.Y), int(b.Z))}
				if data := s.GetNBT(int(b.X), int(b.Y), int(b.Z)); data != nil {
					var encoded bytes.Buffer
					if e := nbt.NewEncoderWithEncoding(&encoded, nbt.LittleEndian).Encode(data); e != nil {
						fail(e)
						return
					}
					block["nbt_base64"] = base64.StdEncoding.EncodeToString(encoded.Bytes())
				}
				blocks = append(blocks, block)
			}
			result["blocks_data"] = blocks
			encoded, _ := json.MarshalIndent(map[string]any{"format": "youyuan-blocks-v1", "size": []int{s.SizeX, s.SizeY, s.SizeZ}, "blocks": blocks}, "", "  ")
			result["content"] = string(encoded)
			result["filename"] = name + ".json"
		case "mcstructure":
			data, e := yyStructureEncode(s)
			if e != nil {
				fail(e)
				return
			}
			result["data"] = base64.StdEncoding.EncodeToString(data)
			result["filename"] = name + ".mcstructure"
		default:
			fail(fmt.Errorf("target 支持 inspect/txt/json/mcstructure/command_chains"))
			return
		}
		writeJSON(w, result)
	})
}
