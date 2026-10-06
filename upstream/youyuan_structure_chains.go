package main

import (
	"bot-apk/building"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type yyStructureCommand struct {
	Position    [3]int `json:"position"`
	Command     string `json:"command"`
	Type        int    `json:"type"`
	Conditional bool   `json:"conditional"`
	Redstone    bool   `json:"redstone"`
	Facing      int    `json:"facing"`
	DelayTicks  int    `json:"delay_ticks"`
	Next        int    `json:"next"`
}

func yyNBTInteger(v any) (int, bool) {
	switch n := v.(type) {
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	case byte:
		return int(n), true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case int:
		return n, true
	case float64:
		return int(n), true
	case json.Number:
		x, e := strconv.Atoi(string(n))
		return x, e == nil
	}
	return 0, false
}

// Extract topology only. World-edit commands and ambiguous dynamic wiring remain
// unchanged. The result is a disabled review bundle, never deployed into the world.
func yyStructureChains(s *building.StructureData, name string, origin [3]int) (map[string]any, error) {
	nodes := []yyStructureCommand{}
	positions := map[[3]int]int{}
	warnings := []string{}
	for _, b := range s.AllBlocks() {
		info := s.Palette[b.Index]
		kind := strings.TrimPrefix(info.Name, "minecraft:")
		typ := 1
		switch kind {
		case "chain_command_block":
			typ = 0
		case "repeating_command_block":
			typ = 2
		case "command_block":
		default:
			continue
		}
		data := s.GetNBT(int(b.X), int(b.Y), int(b.Z))
		cmd, _ := data["Command"].(string)
		if strings.TrimSpace(cmd) == "" {
			warnings = append(warnings, fmt.Sprintf("坐标(%d,%d,%d)命令方块没有Command，保留空节点但不生成游戏指令。", b.X, b.Y, b.Z))
		}
		states, e := yyStateNBT(info.States)
		if e != nil {
			return nil, e
		}
		facing, known := yyNBTInteger(states["facing_direction"])
		if !known || facing < 0 || facing > 5 {
			facing = -1
			warnings = append(warnings, "存在未知朝向的命令方块，未猜测连接关系。")
		}
		cond, _ := yyNBTInteger(states["conditional_bit"])
		if v, ok := yyNBTInteger(data["conditionalMode"]); ok {
			cond = v
		}
		auto, _ := yyNBTInteger(data["auto"])
		ticks, _ := yyNBTInteger(data["TickDelay"])
		pos := [3]int{int(b.X), int(b.Y), int(b.Z)}
		nodes = append(nodes, yyStructureCommand{Position: pos, Command: cmd, Type: typ, Conditional: cond != 0, Redstone: auto == 0, Facing: facing, DelayTicks: ticks, Next: -1})
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("结构中没有命令方块")
	}
	sort.Slice(nodes, func(i, j int) bool {
		a, b := nodes[i].Position, nodes[j].Position
		for k := 0; k < 3; k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return false
	})
	for i, n := range nodes {
		positions[n.Position] = i
	}
	incoming := make([]int, len(nodes))
	dirs := [6][3]int{{0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}, {-1, 0, 0}, {1, 0, 0}}
	for i, n := range nodes {
		if n.Facing < 0 {
			continue
		}
		pos := n.Position
		for k := 0; k < 3; k++ {
			pos[k] += dirs[n.Facing][k]
		}
		if j, ok := positions[pos]; ok && nodes[j].Type == 0 {
			nodes[i].Next = j
			incoming[j]++
		}
	}
	visited := make([]bool, len(nodes))
	files := []map[string]any{}
	chains := [][]int{}
	walk := func(start int) {
		path := []int{}
		i := start
		for !visited[i] {
			visited[i] = true
			path = append(path, i)
			next := nodes[i].Next
			if next < 0 || incoming[next] != 1 {
				break
			}
			i = next
		}
		chains = append(chains, path)
	}
	for i := range nodes {
		if incoming[i] != 1 && !visited[i] {
			walk(i)
		}
	}
	for i := range nodes {
		if !visited[i] {
			warnings = append(warnings, "发现闭环或汇合，已有限拆分；请用@call/条件分支表达实际控制流，不自动无限执行。")
			walk(i)
		}
	}
	for index, path := range chains {
		var text strings.Builder
		fmt.Fprintln(&text, "# 结构静态命令链：需审核执行位置、延迟与动态接线。")
		hasCommand := false
		for _, i := range path {
			n := nodes[i]
			if n.Command == "" {
				continue
			}
			hasCommand = true
			cond, red := 0, 0
			if n.Conditional {
				cond = 1
			}
			if n.Redstone {
				red = 1
			}
			fmt.Fprintf(&text, "%d,%d,%d\n", n.Type, cond, red)
			if n.DelayTicks > 0 {
				warnings = append(warnings, fmt.Sprintf("组件%d含%d tick原生延迟；请审核后设置组延迟/调用周期，未静默丢弃语义。", index+1, n.DelayTicks))
			}
			cmd := strings.TrimSpace(strings.TrimPrefix(n.Command, "/"))
			if strings.Contains(cmd, "command_block") || strings.HasPrefix(cmd, "clone ") {
				warnings = append(warnings, fmt.Sprintf("组件%d有动态接线或clone命令，保留原文；转换为@call/@branch前必须核对目标。", index+1))
			}
			// Block-local relative coordinates must not silently become bot-relative.
			if strings.ContainsAny(cmd, "~^") {
				p := n.Position
				fmt.Fprintf(&text, "execute positioned %d %d %d run %s\n", origin[0]+p[0], origin[1]+p[1], origin[2]+p[2], cmd)
			} else {
				fmt.Fprintln(&text, cmd)
			}
		}
		if hasCommand {
			filename := fmt.Sprintf("%s_链%03d.txt", name, index+1)
			if _, e := yyChainName(filename); e != nil {
				return nil, e
			}
			if _, e := yyParseChains(text.String()); e != nil {
				return nil, e
			}
			files = append(files, map[string]any{"name": filename, "content": text.String(), "node_indices": path, "enabled": false})
		}
	}
	warnings = append(warnings, "仅恢复静态朝向连接。红石/空节点/延迟/相对朝向和条件前驱需人工审核；不会自动删除世界坐标、猜测动态门或启动文件。")
	return map[string]any{"format": "youyuan-command-topology-v1", "files": files, "nodes": nodes, "chains": len(files), "command_blocks": len(nodes), "warnings": warnings, "execution_origin": origin}, nil
}
