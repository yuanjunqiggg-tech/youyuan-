package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════
// 命令方块工程 — 导出引擎
// ═══════════════════════════════════════════════════════════════

// CBSimpleBlock 表示扫描到的单个命令方块（简化版，从 StructureData 中提取）
type CBSimpleBlock struct {
	X            int    `json:"x"`
	Y            int    `json:"y"`
	Z            int    `json:"z"`
	Command      string `json:"command"`
	Type         string `json:"type"`   // impulse / repeating / chain
	Facing       string `json:"facing"` // 朝向: z+ / z- / x+ / x- / y+ / y-
	Conditional  bool   `json:"conditional"`
	Auto         bool   `json:"auto"` // 是否保持开启（红石控制）
	TickDelay    int    `json:"tick_delay"`
	CustomName   string `json:"custom_name,omitempty"`
	ExeFirstTick bool   `json:"exe_first_tick"`
	IsBreak      bool   `json:"is_break,omitempty"` // 断链标记（错误朝向的连锁方块打断的链）
}

// CBlockChain 表示一条命令方块链
type CBlockChain struct {
	Blocks    []CBSimpleBlock `json:"blocks"`
	StartX    int             `json:"start_x"`
	StartY    int             `json:"start_y"`
	StartZ    int             `json:"start_z"`
	Direction string          `json:"direction"` // 链的总体走向
}

// CBExportResult 导出结果
type CBExportResult struct {
	Chains  []CBlockChain  `json:"chains"`
	Content string         `json:"content"` // 生成的 .mcfunction 内容
	Stats   *CBSimpleStats `json:"stats"`
}

// CBSimpleStats 简易统计（导出时的预览用）
type CBSimpleStats struct {
	TotalBlocks   int `json:"total_blocks"`
	ChainCount    int `json:"chain_count"`
	Pulse         int `json:"pulse"`
	Repeating     int `json:"repeating"`
	Chain         int `json:"chain"`
	Conditional   int `json:"conditional"`
	Unconditional int `json:"unconditional"`
	HasDelay      int `json:"has_delay"`
	Breaks        int `json:"breaks"`
}

// 朝向 → 位置偏移映射
var cbFacingOffset = map[string][3]int{
	"z+": {0, 0, 1},
	"z-": {0, 0, -1},
	"x+": {1, 0, 0},
	"x-": {-1, 0, 0},
	"y+": {0, 1, 0},
	"y-": {0, -1, 0},
}

// 方块状态 facing_direction 值 → 朝向名
var cbFacingNames = map[int]string{
	0: "y-", 1: "y+", 2: "z-", 3: "z+", 4: "x-", 5: "x+",
}

// 命令方块类型推導
func cbTypeFromName(name string) string {
	n := strings.ToLower(name)
	if strings.Contains(n, "repeating") {
		return "repeating"
	}
	if strings.Contains(n, "chain") {
		return "chain"
	}
	return "impulse"
}

// 解析朝向 facing_direction 值（0-5）
func cbFacingFromStates(states string) string {
	// 从 states 中提取 facing_direction 值
	// 格式: ["facing_direction"=3,...]
	idx := strings.Index(states, `facing_direction"=`)
	if idx < 0 {
		return "z+" // 默认
	}
	idx += len(`facing_direction"=`)
	end := idx
	for end < len(states) && states[end] >= '0' && states[end] <= '9' {
		end++
	}
	if end > idx {
		fd := 0
		fmt.Sscanf(states[idx:end], "%d", &fd)
		if name, ok := cbFacingNames[fd]; ok {
			return name
		}
	}
	return "z+"
}

// ========== 导出 API ==========

func handleCBExportScan(w http.ResponseWriter, r *http.Request) {
	// 防御：扫描/解析/生成过程中任何 panic 都转成可读错误，避免返回 500。
	defer func() {
		if rec := recover(); rec != nil {
			debugLog("CB导出: 处理异常崩溃: %v", rec)
			writeJSON(w, map[string]any{"ok": false, "error": fmt.Sprintf("导出处理异常: %v", rec)})
		}
	}()

	var req struct {
		X1 int `json:"x1"`
		Y1 int `json:"y1"`
		Z1 int `json:"z1"`
		X2 int `json:"x2"`
		Y2 int `json:"y2"`
		Z2 int `json:"z2"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
		return
	}

	// 严格坐标校验：缺任何扫描角点坐标直接报错，不使用默认值
	if req.X1 == 0 && req.Y1 == 0 && req.Z1 == 0 && req.X2 == 0 && req.Y2 == 0 && req.Z2 == 0 {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少扫描坐标，请填写区域 X/Y/Z"})
		return
	}

	// 软限制检查
	dx := absInt(req.X2-req.X1) + 1
	dy := absInt(req.Y2-req.Y1) + 1
	dz := absInt(req.Z2-req.Z1) + 1
	volume := dx * dy * dz
	if volume > 64*64*64 {
		writeJSON(w, map[string]any{"ok": false, "error": "扫描区域超过 64×64×64 软限制", "over_limit": true})
		return
	}

	// 检查机器人连接
	botMgrMu.Lock()
	bm := botMgr
	connected := bm != nil && bm.IsConnected()
	botMgrMu.Unlock()
	if !connected {
		writeJSON(w, map[string]any{"ok": false, "error": "未连接到服务器"})
		return
	}

	// 排序坐标
	sx, sy, sz := minInt(req.X1, req.X2), minInt(req.Y1, req.Y2), minInt(req.Z1, req.Z2)
	ex, ey, ez := maxInt(req.X1, req.X2), maxInt(req.Y1, req.Y2), maxInt(req.Z1, req.Z2)
	sizeX := ex - sx + 1
	sizeY := ey - sy + 1
	sizeZ := ez - sz + 1
	debugLog("CB导出: 扫描区域 (%d,%d,%d)~(%d,%d,%d) 尺寸 %dx%dx%d", sx, sy, sz, ex, ey, ez, sizeX, sizeY, sizeZ)

	// 逐区块请求（16×16，y 全高），像建筑导出一样。
	// 一次请求整个大区域时，网易服务器往往不返回/截断 block_position_data（NBT），
	// 导致连锁命令方块的命令内容读不到、朝向被重置。按区块请求可保证每个区块 NBT 完整。
	const cbChunkSize = 16
	chunkCountX := (sizeX + cbChunkSize - 1) / cbChunkSize
	chunkCountZ := (sizeZ + cbChunkSize - 1) / cbChunkSize
	var blocks []CBSimpleBlock
	for cx := 0; cx < chunkCountX; cx++ {
		for cz := 0; cz < chunkCountZ; cz++ {
			csx := sx + cx*cbChunkSize
			csz := sz + cz*cbChunkSize
			cdx := minInt(cbChunkSize, sizeX-cx*cbChunkSize)
			cdz := minInt(cbChunkSize, sizeZ-cz*cbChunkSize)

			// 传送到区块中心，确保区块加载
			bm.TP(csx+cdx/2, sy, csz+cdz/2)
			time.Sleep(300 * time.Millisecond)
			debugLog("CB导出: 区块 (%d,%d) %dx%d @ (%d,%d,%d)", cx, cz, cdx, cdz, csx, sy, csz)

			data, err := bm.RequestStructure(
				[3]int32{int32(csx), int32(sy), int32(csz)},
				[3]int32{int32(cdx), int32(sizeY), int32(cdz)},
				20*time.Second,
			)
			if err != nil {
				debugLog("CB导出: 区块 (%d,%d) 请求失败: %v", cx, cz, err)
				continue
			}
			chunkBlocks := extractCommandBlocks(data, csx, sy, csz)
			debugLog("CB导出: 区块 (%d,%d) 提取到 %d 个命令方块", cx, cz, len(chunkBlocks))
			blocks = append(blocks, chunkBlocks...)
		}
	}
	debugLog("CB导出: 共提取到 %d 个命令方块", len(blocks))
	if len(blocks) == 0 {
		writeJSON(w, map[string]any{"ok": false, "error": "未找到命令方块"})
		return
	}

	// 追踪链
	chains := traceCommandBlockChains(blocks)
	if len(chains) == 0 {
		// 如果链追踪失败（如只有孤立方块），把每个方块当一条链
		for _, b := range blocks {
			chains = append(chains, CBlockChain{
				Blocks: []CBSimpleBlock{b},
				StartX: b.X, StartY: b.Y, StartZ: b.Z,
				Direction: b.Facing,
			})
		}
	}

	// 生成 .mcfunction 内容
	content := generateCBProject(chains)

	// 统计信息
	stats := computeCBStats(chains, content)

	writeJSON(w, map[string]any{
		"ok":      true,
		"result":  CBExportResult{Chains: chains, Content: content, Stats: stats},
		"stats":   stats,
		"content": content,
	})
}

// extractCommandBlocks 从结构数据中提取所有命令方块
// 数据结构来自 protocol.StructureTemplateDataResponse.StructureTemplate
// 格式: {size:[...], structure:{palette:{default:{block_palette:[...], block_position_data:{...}}}, block_indices:[[...],[...]]}}
func extractCommandBlocks(data map[string]any, originX, originY, originZ int) []CBSimpleBlock {
	var blocks []CBSimpleBlock

	// 获取结构大小。
	// 实时结构数据经 flowers-for-machines nbt 包解码，TAG_IntArray 生成的是 [N]int32 数组
	// （而非 []any/[]int32），不能直接 .([]any)，用 anyToIntSlice 兼容所有形态。
	sizeRaw := anyToIntSlice(data["size"])
	if len(sizeRaw) < 3 {
		return blocks
	}
	sx := sizeRaw[0]
	sy := sizeRaw[1]
	sz := sizeRaw[2]
	if sx <= 0 || sy <= 0 || sz <= 0 {
		return blocks
	}

	structure, _ := data["structure"].(map[string]any)
	if structure == nil {
		return blocks
	}

	// 解析 palette
	paletteData, _ := structure["palette"].(map[string]any)
	if paletteData == nil {
		return blocks
	}

	var blockPaletteRaw []any
	blockPosData := make(map[string]any)
	for _, pv := range paletteData {
		pm, ok := pv.(map[string]any)
		if !ok {
			continue
		}
		if bp, ok := pm["block_palette"].([]any); ok && len(bp) > len(blockPaletteRaw) {
			blockPaletteRaw = bp
		}
		if bpd, ok := pm["block_position_data"].(map[string]any); ok {
			for k, v := range bpd {
				blockPosData[k] = v
			}
		}
	}
	if len(blockPaletteRaw) == 0 {
		return blocks
	}

	// 解析 block_indices
	blockIndices, _ := structure["block_indices"].([]any)
	if len(blockIndices) < 1 {
		return blocks
	}
	layer0 := anyToIntSlice(blockIndices[0])
	if layer0 == nil {
		return blocks
	}

	// 提取 block entity data（用 block_entity_data 下的索引）
	blockEntityMap := make(map[int]map[string]any)
	for key, value := range blockPosData {
		valMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		entityData, ok := valMap["block_entity_data"].(map[string]any)
		if !ok {
			continue
		}
		var keyInt int
		fmt.Sscanf(key, "%d", &keyInt)
		blockEntityMap[keyInt] = entityData
	}

	// 遍历所有方块，找到命令方块
	// block_indices 与 block_position_data 均使用 z-fastest 线性索引：
	//   idx = x*sy*sz + y*sz + z  →  z = i % sz, y = (i / sz) % sy, x = i / (sy*sz)
	for i, idx := range layer0 {
		if idx < 0 || int(idx) >= len(blockPaletteRaw) {
			continue
		}

		blockEntry, _ := blockPaletteRaw[idx].(map[string]any)
		if blockEntry == nil {
			continue
		}
		name, _ := blockEntry["name"].(string)
		if !strings.Contains(strings.ToLower(name), "command_block") {
			continue
		}

		// 计算坐标（z-fastest）
		z := i % sz
		y := (i / sz) % sy
		x := i / (sy * sz)

		absX := originX + x
		absY := originY + y
		absZ := originZ + z

		// 获取方块状态
		states, _ := blockEntry["states"].(map[string]any)
		fd := 3 // 默认 z+
		if v, ok := states["facing_direction"]; ok {
			fd = int(toFloat64(v))
		}
		facing := "z+"
		if f, ok := cbFacingNames[fd]; ok {
			facing = f
		}

		cond := false
		if v, ok := states["conditional_bit"]; ok {
			cond = toFloat64(v) != 0
		}

		// 获取 NBT 数据（block_entity_data 使用 z 最快顺序索引）
		cmd := ""
		customName := ""
		auto := true
		tickDelay := 0
		exeFirst := false
		needsRedstone := false

		// block_entity_data 与 block_indices 使用同一个 z-fastest 线性索引，直接用 i 查表
		if nbt, ok := blockEntityMap[i]; ok {
			if c, ok := nbt["Command"].(string); ok {
				cmd = c
			}
			if cn, ok := nbt["CustomName"].(string); ok {
				customName = cn
			}
			if v, ok := nbt["auto"]; ok {
				auto = toFloat64(v) != 0
			}
			if v, ok := nbt["TickDelay"]; ok {
				tickDelay = int(toFloat64(v))
			}
			if v, ok := nbt["ExecuteOnFirstTick"]; ok {
				exeFirst = toFloat64(v) != 0
			}
			if v, ok := nbt["NeedsRedstone"]; ok {
				needsRedstone = toFloat64(v) != 0
			}
			if v, ok := nbt["LPRedstoneMode"]; ok {
				needsRedstone = toFloat64(v) != 0
			}
		}

		needsRedstone = needsRedstone || !auto

		blockType := cbTypeFromName(name)

		blocks = append(blocks, CBSimpleBlock{
			X: absX, Y: absY, Z: absZ,
			Command:      cmd,
			Type:         blockType,
			Facing:       facing,
			Conditional:  cond,
			Auto:         auto,
			TickDelay:    tickDelay,
			CustomName:   customName,
			ExeFirstTick: exeFirst,
		})
	}

	return blocks
}

// anyToIntSlice 将任意类型转为 []int。
// 覆盖 NBT 解码可能产生的所有形态：
//   - flowers-for-machines nbt 包：IntArray → [N]int32 数组；List<Int> → []int32；List<any> → []any
//   - building/mcworld.go 自带解析器：IntArray → []int32
// 统一转成 []int 便于下游使用。
func anyToIntSlice(v any) []int {
	switch val := v.(type) {
	case []any:
		out := make([]int, len(val))
		for i, e := range val {
			out[i] = int(toFloat64(e))
		}
		return out
	case []int32:
		out := make([]int, len(val))
		for i, n := range val {
			out[i] = int(n)
		}
		return out
	case []int64:
		out := make([]int, len(val))
		for i, n := range val {
			out[i] = int(n)
		}
		return out
	case []int:
		return val
	}
	// 反射兜底：覆盖 [N]int32 定长数组（NBT IntArray 解码产物）
	rv := reflect.ValueOf(v)
	if rv.IsValid() && rv.Kind() == reflect.Array && rv.Type().Elem().Kind() == reflect.Int32 {
		n := rv.Len()
		out := make([]int, n)
		for i := 0; i < n; i++ {
			out[i] = int(rv.Index(i).Int())
		}
		return out
	}
	return nil
}

// traceCommandBlockChains 从命令方块列表中追踪链
// cbPosKey 生成命令方块 3D 位置索引键。
func cbPosKey(x, y, z int) string { return fmt.Sprintf("%d,%d,%d", x, y, z) }

// traceCommandBlockChains 从命令方块列表中追踪链。
// 链从脉冲/循环方块开始，沿每个方块的朝向（含上下左右前后 6 方向）一步步延伸，
// 找到连锁方块就继续（朝向变化只是链拐弯，不视为断链）。
func traceCommandBlockChains(blocks []CBSimpleBlock) []CBlockChain {
	blockMap := make(map[string]*CBSimpleBlock)
	for i := range blocks {
		blockMap[cbPosKey(blocks[i].X, blocks[i].Y, blocks[i].Z)] = &blocks[i]
	}

	visited := make(map[string]bool)
	var chains []CBlockChain

	// 从脉冲/循环方块开始追踪
	for i := range blocks {
		b := &blocks[i]
		startKey := cbPosKey(b.X, b.Y, b.Z)
		if visited[startKey] {
			continue
		}
		if b.Type != "impulse" && b.Type != "repeating" {
			continue
		}
		visited[startKey] = true

		chain := []CBSimpleBlock{*b}
		cur := b
		for {
			off, ok := cbFacingOffset[cur.Facing]
			if !ok {
				break
			}
			nx := cur.X + off[0]
			ny := cur.Y + off[1]
			nz := cur.Z + off[2]
			nk := cbPosKey(nx, ny, nz)
			next, exists := blockMap[nk]
			if !exists || visited[nk] || next.Type != "chain" {
				break
			}
			visited[nk] = true
			chain = append(chain, *next)
			cur = next
		}

		if len(chain) > 0 {
			chains = append(chains, CBlockChain{
				Blocks:    chain,
				StartX:    b.X,
				StartY:    b.Y,
				StartZ:    b.Z,
				Direction: b.Facing,
			})
		}
	}

	return chains
}

// generateCBProject 从链生成 .mcfunction 内容
func generateCBProject(chains []CBlockChain) string {
	var sb strings.Builder

	// 收集积分板和标签
	sb.WriteString("//工程:命令方块工程\n")
	sb.WriteString("//版本:1.0\n")
	sb.WriteString("//作者:" + yyBrandAuthor + "\n")
	sb.WriteString("//说明:由 " + yyBrandShort + " 自动导出\n")

	sb.WriteString("\n")

	for ci, chain := range chains {
		if ci > 0 {
			sb.WriteString("\n")
		}
		prevFacing := ""
		prevType := ""
		prevConditional := false
		prevDelay := 0
		prevExeFirst := false
		hasPrev := false

		for i, block := range chain.Blocks {
			// 断链标记（错误朝向的连锁方块打断的链）：写 # 断链，重置属性继承
			if block.IsBreak {
				sb.WriteString("# 断链\n")
				hasPrev = false
				continue
			}

			// 检测断链：如果当前位置和上一个不连续，插断链标记
			if hasPrev && i > 0 {
				prev := chain.Blocks[i-1]
				off, ok := cbFacingOffset[prev.Facing]
				if ok {
					expectedX := prev.X + off[0]
					expectedY := prev.Y + off[1]
					expectedZ := prev.Z + off[2]
					if block.X != expectedX || block.Y != expectedY || block.Z != expectedZ {
						sb.WriteString("# 断链\n")
						// 重置 prev 属性，让下一个方块写完整属性
						hasPrev = false
					}
				}
			}

			// 写属性行
			var props []string
			needFull := !hasPrev

			// 朝向
			if !hasPrev || block.Facing != prevFacing {
				props = append(props, "朝向="+block.Facing)
			}
			// 类型
			if !hasPrev || block.Type != prevType {
				props = append(props, "类型="+block.Type)
			}
			// 有条件
			if (!hasPrev && block.Conditional) || (hasPrev && block.Conditional != prevConditional) {
				val := "否"
				if block.Conditional {
					val = "是"
				}
				props = append(props, "有条件="+val)
			}
			// 红石控制：导入默认"脉冲需红石、连锁/循环保持开启"。
			// 当本块红石值与类型默认不符时必须显式写出，保证导出→导入 round-trip 一致。
			needsRS := !block.Auto
			defaultRS := block.Type == "impulse"
			if needsRS != defaultRS {
				val := "否"
				if needsRS {
					val = "是"
				}
				props = append(props, "红石控制="+val)
			}
			// 延迟
			if (!hasPrev && block.TickDelay > 0) || (hasPrev && block.TickDelay != prevDelay) {
				props = append(props, fmt.Sprintf("延迟=%d", block.TickDelay))
			}
			// 首次执行（仅循环方块）
			if block.Type == "repeating" {
				if (!hasPrev && block.ExeFirstTick) || (hasPrev && block.ExeFirstTick != prevExeFirst) {
					val := "否"
					if block.ExeFirstTick {
						val = "是"
					}
					props = append(props, "首次执行="+val)
				}
			}

			if needFull || len(props) > 0 {
				sb.WriteString("# " + strings.Join(props, " ") + "\n")
			}

			// 写命令行
			cmd := block.Command
			if block.CustomName != "" {
				cmd = cmd + " #" + block.CustomName
			}
			sb.WriteString(cmd + "\n")

			// 更新 prev
			prevFacing = block.Facing
			prevType = block.Type
			prevConditional = block.Conditional
			prevDelay = block.TickDelay
			prevExeFirst = block.ExeFirstTick
			hasPrev = true
		}
	}

	return sb.String()
}

// computeCBStats 计算统计信息
func computeCBStats(chains []CBlockChain, content string) *CBSimpleStats {
	stats := &CBSimpleStats{}
	for _, chain := range chains {
		for _, b := range chain.Blocks {
			stats.TotalBlocks++
			switch b.Type {
			case "impulse":
				stats.Pulse++
			case "repeating":
				stats.Repeating++
			case "chain":
				stats.Chain++
			}
			if b.Conditional {
				stats.Conditional++
			} else {
				stats.Unconditional++
			}
			if b.TickDelay > 0 {
				stats.HasDelay++
			}
		}
	}
	stats.ChainCount = len(chains)

	// 统计断链（从 content 中分析）
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# 断链" {
			stats.Breaks++
		}
	}

	return stats
}

// 处理 .mcfunction 文件导出（保存到本地）
func handleCBExportSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content  string `json:"content"`
		FilePath string `json:"file_path"`
		FileName string `json:"file_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
		return
	}

	if req.FilePath == "" {
		// 默认路径
		req.FilePath = "/storage/emulated/0/Download/"
	}
	if req.FileName == "" {
		req.FileName = "cb_project.mcfunction"
	}
	if !strings.HasSuffix(req.FileName, ".mcfunction") {
		req.FileName += ".mcfunction"
	}

	fullPath := req.FilePath
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += req.FileName

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "保存失败: " + err.Error()})
		return
	}

	writeJSON(w, map[string]any{"ok": true, "path": fullPath})
}

// ========== 辅助函数 ==========

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
