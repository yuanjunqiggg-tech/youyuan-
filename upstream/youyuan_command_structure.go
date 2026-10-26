package main

import (
	"archive/zip"
	"bot-apk/building"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Plain commands share the chain parser, but the physical first block needs a pulse.
func yyCommandStructure(content string, x, z, delay int) (*building.StructureData, error) {
	if x < 1 || z < 1 || x > 128 || z > 128 || delay < 0 || delay > 12000 {
		return nil, fmt.Errorf("长宽需为1..128，延迟需为0..12000游戏刻")
	}
	plan, err := yyParseChains(content)
	if err != nil {
		return nil, err
	}
	explicit := false
	for _, line := range strings.Split(content, "\n") {
		if yyChainHeader.MatchString(strings.TrimSpace(line)) {
			explicit = true
			break
		}
	}
	total := plan.Commands + len(plan.Chains) - 1
	y := (total + x*z - 1) / (x * z)
	if int64(x)*int64(y)*int64(z) > 131072 {
		return nil, fmt.Errorf("命令结构超过131072方块，请减小尺寸或拆分文件")
	}
	s := &building.StructureData{SizeX: x, SizeY: y, SizeZ: z, Blocks: map[uint64]uint32{}}
	position := func(i int) [3]int {
		layer, row, col := i/(x*z), (i/z)%x, i%z
		px := row
		if layer%2 == 1 {
			px = x - 1 - row
		}
		pz := col
		if (layer*x+row)%2 == 1 {
			pz = z - 1 - col
		}
		return [3]int{px, layer, pz}
	}
	ordinal := 0
	for _, chain := range plan.Chains {
		if chain.Repeat != 1 || chain.Period != 0 {
			return nil, fmt.Errorf("分组循环/!周期属于机器人调度，不能无损写成原生命令方块；请拆为独立文件")
		}
		for i, node := range chain.Nodes {
			if node.Control != nil || node.DelayMS != 0 || node.Channel != "" {
				return nil, fmt.Errorf("第%d行含宿主API/通道/毫秒调度，不能写入原生命令方块", node.Line)
			}
			p, next := position(ordinal), position(ordinal+1)
			facing := 1
			if i+1 < len(chain.Nodes) {
				switch {
				case next[0] > p[0]:
					facing = 5
				case next[0] < p[0]:
					facing = 4
				case next[2] > p[2]:
					facing = 3
				case next[2] < p[2]:
					facing = 2
				}
			}
			typ := 1
			if explicit {
				typ = []int{1, 0, 2}[node.Type]
			} else if i == 0 {
				typ = 0
			}
			ticks := delay
			if i == 0 && chain.Hz > 0 {
				period := 20 / chain.Hz
				if period < 1 || period > 12000 || period != float64(int(period)) {
					return nil, fmt.Errorf("第%d行Hz无法精确换成原生20刻时钟，未丢弃该设置", node.Line)
				}
				ticks = int(period)
			}
			name := []string{"minecraft:command_block", "minecraft:chain_command_block", "minecraft:repeating_command_block"}[typ]
			conditional, auto := byte(0), byte(1)
			if node.Conditional {
				conditional = 1
			}
			if node.Redstone || (!explicit && i == 0) {
				auto = 0
			}
			s.Set(p[0], p[1], p[2], &building.BlockInfo{Name: name, States: fmt.Sprintf(`["conditional_bit"=%t,"facing_direction"=%d]`, node.Conditional, facing), NBT: map[string]any{
				"id": "CommandBlock", "Command": node.Command, "CustomName": "", "Version": int32(25), "TickDelay": int32(ticks), "TrackOutput": byte(0), "auto": auto, "conditionalMode": conditional, "ExecuteOnFirstTick": byte(1),
			}})
			ordinal++
		}
		ordinal++ // An air gap prevents separate TXT chains from powering each other.
	}
	return s, nil
}

func yyNaturalLess(a, b string) bool {
	for len(a) > 0 && len(b) > 0 {
		if a[0] >= '0' && a[0] <= '9' && b[0] >= '0' && b[0] <= '9' {
			i, j := 0, 0
			for i < len(a) && a[i] >= '0' && a[i] <= '9' {
				i++
			}
			for j < len(b) && b[j] >= '0' && b[j] <= '9' {
				j++
			}
			x, y := strings.TrimLeft(a[:i], "0"), strings.TrimLeft(b[:j], "0")
			if len(x) != len(y) {
				return len(x) < len(y)
			}
			if x != y {
				return x < y
			}
			a, b = a[i:], b[j:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func yyTextFrames(data []byte, filename string) ([]string, error) {
	if len(data) > 32<<20 {
		return nil, fmt.Errorf("输入超过32MiB")
	}
	if !strings.EqualFold(filepath.Ext(filename), ".zip") {
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("文本需要UTF-8编码")
		}
		return []string{strings.TrimPrefix(string(data), "\ufeff")}, nil
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := []*zip.File{}
	for _, f := range z.File {
		if !f.FileInfo().IsDir() && strings.EqualFold(filepath.Ext(f.Name), ".txt") {
			files = append(files, f)
		}
	}
	if len(files) == 0 || len(files) > 10000 {
		return nil, fmt.Errorf("ZIP需要1..10000个TXT帧")
	}
	sort.SliceStable(files, func(i, j int) bool {
		return yyNaturalLess(strings.ToLower(files[i].Name), strings.ToLower(files[j].Name))
	})
	frames := make([]string, 0, len(files))
	remaining := int64(32 << 20)
	for _, f := range files {
		if f.UncompressedSize64 > uint64(remaining) {
			return nil, fmt.Errorf("ZIP展开超过32MiB")
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		raw, e := io.ReadAll(io.LimitReader(r, remaining+1))
		closeErr := r.Close()
		if e != nil {
			return nil, e
		}
		if closeErr != nil {
			return nil, closeErr
		}
		remaining -= int64(len(raw))
		if remaining < 0 {
			return nil, fmt.Errorf("ZIP展开超过32MiB")
		}
		if !utf8.Valid(raw) {
			return nil, fmt.Errorf("%s不是UTF-8文本", f.Name)
		}
		frames = append(frames, strings.TrimPrefix(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\ufeff"))
	}
	return frames, nil
}

func yyCommandStructureRoute(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Content, Data, Filename, Mode, Selector, Display, Output string
		Width, Depth, Delay                                      int
	}
	fail := func(e error) { writeJSON(w, map[string]any{"ok": false, "error": e.Error()}) }
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !yyConvertMu.TryLock() {
		fail(fmt.Errorf("已有转换进行中"))
		return
	}
	defer yyConvertMu.Unlock()
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 48<<20)).Decode(&p); err != nil {
		fail(err)
		return
	}
	if p.Width == 0 {
		p.Width = 16
	}
	if p.Depth == 0 {
		p.Depth = 16
	}
	content := p.Content
	frames := 0
	if len(content) > 2<<20 {
		fail(fmt.Errorf("指令文本超过2MiB"))
		return
	}
	if p.Data != "" {
		raw, e := base64.StdEncoding.DecodeString(p.Data)
		if e != nil {
			fail(e)
			return
		}
		texts, e := yyTextFrames(raw, p.Filename)
		if e != nil {
			fail(e)
			return
		}
		if p.Mode == "animation" {
			if p.Selector == "" {
				p.Selector = "@a"
			}
			if !yyConvertSelector.MatchString(p.Selector) {
				fail(fmt.Errorf("请输入有效的玩家选择器"))
				return
			}
			if p.Display == "" {
				p.Display = "actionbar"
			}
			if p.Display != "actionbar" && p.Display != "title" && p.Display != "subtitle" {
				fail(fmt.Errorf("显示位置支持actionbar/title/subtitle"))
				return
			}
			var b strings.Builder
			for _, frame := range texts {
				payload, _ := json.Marshal(map[string]any{"rawtext": []any{map[string]any{"text": frame}}})
				fmt.Fprintf(&b, "titleraw %s %s %s\n", p.Selector, p.Display, payload)
			}
			content = b.String()
			frames = len(texts)
		} else {
			content = strings.Join(texts, "\n")
		}
	} else if p.Mode == "animation" {
		fail(fmt.Errorf("动画模式请选择TXT帧或ZIP帧包"))
		return
	}
	if len(content) > 2<<20 {
		fail(fmt.Errorf("展开后的指令超过2MiB，请拆分帧包或指令文件"))
		return
	}
	if p.Mode != "" && p.Mode != "commands" && p.Mode != "animation" {
		fail(fmt.Errorf("模式支持commands/animation"))
		return
	}
	if p.Output == "mcfunction" {
		plan, e := yyParseChains(content)
		if e != nil {
			fail(e)
			return
		}
		for _, line := range strings.Split(content, "\n") {
			if yyConvertControl(strings.TrimSpace(line)) {
				fail(fmt.Errorf("mcfunction不能保留链属性，请输出结构"))
				return
			}
		}
		writeJSON(w, map[string]any{"ok": true, "content": content, "commands": plan.Commands, "frames": frames, "filename": "游园动画帧.mcfunction", "note": "纯指令文件；逐帧定时播放请选择命令方块结构。"})
		return
	}
	if p.Output != "" && p.Output != "mcstructure" {
		fail(fmt.Errorf("输出支持mcstructure/mcfunction"))
		return
	}
	s, e := yyCommandStructure(content, p.Width, p.Depth, p.Delay)
	if e != nil {
		fail(e)
		return
	}
	raw, e := yyStructureEncode(s)
	if e != nil {
		fail(e)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "data": base64.StdEncoding.EncodeToString(raw), "filename": "游园命令结构.mcstructure", "size": []int{s.SizeX, s.SizeY, s.SizeZ}, "blocks": len(s.AllBlocks()), "palette": len(s.Palette), "block_entities": len(s.NBTBlocks), "commands": len(s.AllBlocks()), "frames": frames, "note": "普通文件首块用红石启动；带属性TXT保留红石设置。后续链沿朝向连续执行。20游戏刻=1秒。"})
}

// Parse placement TXT without executing it or silently discarding unsupported commands.
func yyPlacementText(data []byte) (*building.StructureData, error) {
	if len(data) > 2<<20 || !utf8.Valid(data) {
		return nil, fmt.Errorf("TXT需要2MiB以内的UTF-8文本")
	}
	type placement struct {
		from, to     [3]int
		name, states string
	}
	rows := []placement{}
	min, max := [3]int{30000001, 30000001, 30000001}, [3]int{-30000001, -30000001, -30000001}
	var work int64
	coordinateMode := ""
	for i, line := range strings.Split(strings.ReplaceAll(strings.TrimPrefix(string(data), "\ufeff"), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "/")
		tokens, e := yyPlacementTokens(line)
		if e != nil {
			return nil, fmt.Errorf("第%d行：%w", i+1, e)
		}
		coords := 3
		if len(tokens) > 0 && tokens[0] == "fill" {
			coords = 6
		}
		if len(tokens) < coords+2 || (tokens[0] != "setblock" && tokens[0] != "fill") {
			return nil, fmt.Errorf("第%d行：建筑TXT只接受setblock/fill；其他指令请用指令转结构", i+1)
		}
		p := placement{}
		v := make([]int, coords)
		for j := 0; j < coords; j++ {
			mode := "absolute"
			if strings.HasPrefix(tokens[j+1], "~") {
				mode = "relative"
			}
			if coordinateMode == "" {
				coordinateMode = mode
			} else if coordinateMode != mode {
				return nil, fmt.Errorf("第%d行混用了绝对与相对坐标，需先统一放置原点", i+1)
			}
			raw := strings.TrimPrefix(tokens[j+1], "~")
			if raw == "" {
				raw = "0"
			}
			n, e := strconv.Atoi(raw)
			if e != nil || n < -30000000 || n > 30000000 {
				return nil, fmt.Errorf("第%d行坐标无效", i+1)
			}
			v[j] = n
		}
		copy(p.from[:], v[:3])
		p.to = p.from
		if coords == 6 {
			copy(p.to[:], v[3:])
		}
		p.name = tokens[coords+1]
		if !yyConvertBlockName.MatchString(p.name) {
			return nil, fmt.Errorf("第%d行方块ID无效", i+1)
		}
		if !strings.Contains(p.name, ":") {
			p.name = "minecraft:" + p.name
		}
		p.states = "[]"
		for _, tail := range tokens[coords+2:] {
			if strings.HasPrefix(tail, "[") {
				if _, e := yyStateNBT(tail); e != nil {
					return nil, e
				}
				p.states = tail
			} else if tail != "replace" && tail != "0" {
				return nil, fmt.Errorf("第%d行不支持条件替换/旧版数字数据值/破坏模式，未忽略该参数", i+1)
			}
		}
		var count int64 = 1
		for axis := 0; axis < 3; axis++ {
			if p.from[axis] > p.to[axis] {
				p.from[axis], p.to[axis] = p.to[axis], p.from[axis]
			}
			if p.from[axis] < min[axis] {
				min[axis] = p.from[axis]
			}
			if p.to[axis] > max[axis] {
				max[axis] = p.to[axis]
			}
			span := int64(p.to[axis] - p.from[axis] + 1)
			if span > 131072 || count > 131072/span {
				return nil, fmt.Errorf("第%d行fill超过131072方块", i+1)
			}
			count *= span
		}
		work += count
		if work > 1000000 {
			return nil, fmt.Errorf("TXT累计写入超过100万方块，请拆分")
		}
		rows = append(rows, p)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("TXT没有放置命令")
	}
	size := [3]int{max[0] - min[0] + 1, max[1] - min[1] + 1, max[2] - min[2] + 1}
	if int64(size[0])*int64(size[1])*int64(size[2]) > 131072 {
		return nil, fmt.Errorf("TXT包围盒超过131072方块")
	}
	s := &building.StructureData{SizeX: size[0], SizeY: size[1], SizeZ: size[2], Blocks: map[uint64]uint32{}}
	for _, p := range rows {
		for x := p.from[0]; x <= p.to[0]; x++ {
			for y := p.from[1]; y <= p.to[1]; y++ {
				for z := p.from[2]; z <= p.to[2]; z++ {
					s.Set(x-min[0], y-min[1], z-min[2], &building.BlockInfo{Name: p.name, States: p.states})
				}
			}
		}
	}
	return s, nil
}

func yyPlacementTokens(line string) ([]string, error) {
	var out []string
	start, depth := 0, 0
	quoted, escaped := false, false
	for i, c := range line {
		if escaped {
			escaped = false
			continue
		}
		if quoted && c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			quoted = !quoted
		}
		if !quoted {
			if c == '[' {
				depth++
			}
			if c == ']' {
				depth--
				if depth < 0 {
					return nil, fmt.Errorf("方块状态括号不匹配")
				}
			}
			if (c == ' ' || c == '\t') && depth == 0 {
				if i > start {
					out = append(out, line[start:i])
				}
				start = i + 1
			}
		}
	}
	if quoted || depth != 0 {
		return nil, fmt.Errorf("引号或方块状态括号未闭合")
	}
	if start < len(line) {
		out = append(out, line[start:])
	}
	return out, nil
}
