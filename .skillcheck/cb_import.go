package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bot-apk/state"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// ═══════════════════════════════════════════════════════════════
// 命令方块工程 — 导入引擎
// ═══════════════════════════════════════════════════════════════

// CBParsedBlock 解析后的单个命令方块
type CBParsedBlock struct {
	Command     string `json:"command"`
	Facing      string `json:"facing"` // 朝向
	Type        string `json:"type"`   // impulse / repeating / chain
	Conditional bool   `json:"conditional"`
	NeedsRS     bool   `json:"needs_rs"` // 需要红石
	TickDelay   int    `json:"tick_delay"`
	ExeFirst    bool   `json:"exe_first"` // 首次执行（仅循环）
	CustomName  string `json:"custom_name,omitempty"`
	ChainBreak  bool   `json:"chain_break"` // 断链标记（兼容旧格式）
	ChainStart  bool   `json:"chain_start"` // 是否为一条命令链的链首（用于另起布局）
}

// CBParsedProject 解析后的工程文件
type CBParsedProject struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Author      string          `json:"author"`
	Description string          `json:"description"`
	Scoreboards []string        `json:"scoreboards"`
	Tags        []string        `json:"tags"`
	Blocks      []CBParsedBlock `json:"blocks"`
	Warnings    []string        `json:"warnings"`
}

// CBImportRequest 导入请求
type CBImportRequest struct {
	Content string `json:"content"`
	StartX  int    `json:"start_x"`
	StartY  int    `json:"start_y"`
	StartZ  int    `json:"start_z"`
}

// ========== 解析引擎 ==========

// parseCBProject 解析 .mcfunction 工程文件内容（宽容模式）。
//
// 支持三种输入，无需每行都写注释：
//  1. 纯文本：每行一条命令。链首（第 1 条或空行后第 1 条）为脉冲，后续为连锁，无延迟；
//     默认"脉冲需红石、连锁/循环保持开启"。
//  2. 空行：作为一条新命令链的起始（链首重新为脉冲）。
//  3. `# 属性` 注释行：显式设置当前及后续方块的朝向/类型/有条件/红石控制/延迟/首次执行；
//     `# 断链` 等价于空行（开新链）。
//     以 `//` 开头的行在文件头部解析为元数据，其余位置跳过。
//
// 属性继承规则：
//   - 朝向/有条件/延迟/首次执行：继承上一个方块，直到被显式修改。
//   - 类型：显式设置后继承；未显式设置时按"链首脉冲、其余连锁"自动推导。
//   - 红石控制：显式设置后继承；未显式设置时默认"除脉冲外需红石"。
func parseCBProject(content string) *CBParsedProject {
	proj := &CBParsedProject{
		Blocks:   make([]CBParsedBlock, 0),
		Warnings: make([]string, 0),
	}

	lines := strings.Split(content, "\n")

	// 解析 // 元数据（文件头部）
	metaEnd := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") {
			metaEnd = i
			break
		}
		metaEnd = i + 1
		parseCBMetaLine(trimmed, proj)
	}

	// 当前属性（继承用）
	curFacing := "z+"
	curType := "" // 空 = 自动（链首脉冲，其余连锁）
	curConditional := false
	curNeedsRS := false // 显式红石值（仅 needsRSSet 时生效）
	needsRSSet := false // 是否显式设置过红石控制
	curDelay := 0
	curExeFirst := false
	inChain := false // 当前是否已进入一条链（链首之后）

	// 解析 # 属性和命令行
	for i := metaEnd; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			// 空行 = 新命令链起始，属性继承重置
			inChain = false
			curType = ""
			needsRSSet = false
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			continue // 注释跳过
		}
		if strings.HasPrefix(trimmed, "#") {
			afterHash := strings.TrimSpace(trimmed[1:])
			if afterHash == "断链" {
				inChain = false
				curType = ""
				needsRSSet = false
				continue
			}
			// 解析属性行（朝向/类型/有条件/红石/延迟/首次执行）
			curType = parseCBAttributes(afterHash, &curFacing, curType, &curConditional, &curNeedsRS, &needsRSSet, &curDelay, &curExeFirst)
			continue
		}

		// 命令行（可能有行末注释）
		cmd, customName := parseCBLineComment(trimmed)
		if cmd == "" {
			continue
		}
		// 类型：显式设置优先，否则链首脉冲、其余连锁
		typ := curType
		if typ == "" {
			if inChain {
				typ = "chain"
			} else {
				typ = "impulse"
			}
		}
		// 红石控制：显式设置优先，否则默认"脉冲需红石、连锁/循环保持开启"
		needsRS := curNeedsRS
		if !needsRSSet {
			needsRS = typ == "impulse"
		}

		proj.Blocks = append(proj.Blocks, CBParsedBlock{
			Command:     cmd,
			Facing:      curFacing,
			Type:        typ,
			Conditional: curConditional,
			NeedsRS:     needsRS,
			TickDelay:   curDelay,
			ExeFirst:    curExeFirst,
			CustomName:  customName,
			ChainStart:  !inChain, // 链首（空行/断链后的第一个，或文件第一个）
		})
		inChain = true
	}

	// 验证
	if len(proj.Blocks) > 0 && proj.Name == "" {
		proj.Name = "未命名工程"
	}
	if len(proj.Blocks) == 0 {
		proj.Warnings = append(proj.Warnings, "工程中未找到命令方块")
	}

	return proj
}

// parseCBMetaLine 解析单行 // 元数据
func parseCBMetaLine(line string, proj *CBParsedProject) {
	// 去掉 // 前缀
	content := strings.TrimSpace(strings.TrimPrefix(line, "//"))

	// 支持 : 和 = 分隔符
	sep := ":"
	if strings.Contains(content, "=") && !strings.Contains(content, ":") {
		sep = "="
	}

	idx := strings.Index(content, sep)
	if idx < 0 {
		return // 普通注释，忽略
	}

	key := strings.TrimSpace(content[:idx])
	val := strings.TrimSpace(content[idx+len(sep):])

	switch key {
	case "工程", "project":
		proj.Name = val
	case "版本", "version":
		proj.Version = val
	case "作者", "author":
		proj.Author = val
	case "说明", "description", "desc":
		proj.Description = val
	case "积分板", "scoreboards":
		parts := strings.Split(val, ",")
		for _, p := range parts {
			s := strings.TrimSpace(p)
			if s != "" {
				proj.Scoreboards = append(proj.Scoreboards, s)
			}
		}
	case "标签", "tags":
		parts := strings.Split(val, ",")
		for _, p := range parts {
			s := strings.TrimSpace(p)
			if s != "" {
				proj.Tags = append(proj.Tags, s)
			}
		}
	}
}

// parseCBAttributes 解析 # 属性行，返回解析后的当前类型（显式设置则继承，否则不变）。
func parseCBAttributes(line string, curFacing *string, curType string, curConditional, curNeedsRS *bool, needsRSSet *bool, curDelay *int, curExeFirst *bool) string {
	t := curType
	parts := strings.Fields(line)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		switch key {
		case "朝向", "facing":
			if _, ok := cbFacingOffset[val]; ok {
				*curFacing = val
			}
		case "类型", "type":
			switch val {
			case "脉冲", "impulse":
				t = "impulse"
			case "循环", "repeating":
				t = "repeating"
			case "连锁", "chain":
				t = "chain"
			}
		case "有条件", "conditional":
			*curConditional = cbTruthy(val)
		case "红石控制", "redstone", "needs_rs":
			*curNeedsRS = cbTruthy(val)
			*needsRSSet = true
		case "延迟", "delay", "tick_delay":
			if n, err := strconv.Atoi(val); err == nil && n >= 0 {
				*curDelay = n
			}
		case "首次执行", "execute_first_tick", "exe_first":
			*curExeFirst = cbTruthy(val)
		}
	}
	return t
}

// cbTruthy 解析是/否、true/false、1/0 为布尔值。
func cbTruthy(val string) bool {
	return val == "是" || val == "true" || val == "1"
}

// parseCBLineComment 解析行末 # 注释
func parseCBLineComment(line string) (cmd, customName string) {
	// 从最后一个 # 开始识别为注释
	// 注意：命令本身可能包含 #，如 /say #1
	hashIdx := -1
	for i := len(line) - 1; i >= 0; i-- {
		if line[i] == '#' {
			// 检查是否是 # 开头（属性行）
			if i == 0 || line[i-1] == ' ' {
				hashIdx = i
				break
			}
		}
	}

	if hashIdx >= 0 {
		cmd = strings.TrimSpace(line[:hashIdx])
		customName = strings.TrimSpace(line[hashIdx+1:])
	} else {
		cmd = strings.TrimSpace(line)
	}
	return
}

// ========== 导入执行 ==========

// getRequiredInt 读取必须提供的整数参数；缺失或非法则返回 ok=false。
// 命令方块工程的导入/导出一律不允许默认坐标，缺失必须显式报错。
func getRequiredInt(p map[string]any, key string) (int, bool) {
	v, ok := p[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case json.Number:
		f, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(f), true
	default:
		return 0, false
	}
}

// 导入命令方块（通过任务系统）
func runCBImportTask(tc *state.TaskController, p map[string]any) {
	content := getString(p, "content", "")
	if content == "" {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "缺少工程内容"}))
		return
	}

	// 严格坐标校验：缺少任何起点坐标直接报错，不使用默认值
	startX, okX := getRequiredInt(p, "start_x")
	startY, okY := getRequiredInt(p, "start_y")
	startZ, okZ := getRequiredInt(p, "start_z")
	if !okX || !okY || !okZ {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "缺少导入起点坐标，请填写 X/Y/Z"}))
		return
	}

	bm := getBotManager()
	if bm == nil || !bm.IsConnected() {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "未连接到服务器"}))
		return
	}

	proj := parseCBProject(content)

	if len(proj.Warnings) > 0 {
		hub.Emit("progress", mustJSON(map[string]any{
			"msg": fmt.Sprintf("警告: %s", strings.Join(proj.Warnings, "; ")),
		}))
	}

	// 过滤掉断链标记，只保留实际方块
	var placeBlocks []CBParsedBlock
	for _, b := range proj.Blocks {
		if !b.ChainBreak {
			placeBlocks = append(placeBlocks, b)
		}
	}

	total := len(placeBlocks)
	if total == 0 {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "没有需要放置的命令方块"}))
		return
	}

	hub.Emit("task_start", mustJSON(map[string]any{
		"type": fmt.Sprintf("命令方块导入: %s (%d 个方块)", proj.Name, total),
	}))

	hub.Emit("progress", mustJSON(map[string]any{
		"msg": fmt.Sprintf("开始导入 %s: %d 个命令方块，起点 (%d,%d,%d)", proj.Name, total, startX, startY, startZ),
	}))
	debugLog("CB导入: 开始 %s (%d 个方块) 起点 (%d,%d,%d)", proj.Name, total, startX, startY, startZ)

	// 导入前检查
	if bm.IsOP() {
		bm.SendTellraw(fmt.Sprintf("§a§l▍命令方块导入:§r§f%s §7%d 个方块 §7起点:(%d,%d,%d)",
			proj.Name, total, startX, startY, startZ))
	}

	// 先传送到起点，确保机器人一定移动、区块加载
	bm.TP(startX, startY+1, startZ)
	debugLog("CB导入: 已传送机器人到起点 (%d,%d,%d)", startX, startY+1, startZ)

	// 逐个放置。
	// 布局：每条命令链的链首（脉冲）从"链首位置"开始，链内沿朝向延伸；
	// 一条链结束后，下一条链的链首在 x 方向 +1 旁边排开，而不是接着上一条链末尾。
	chainStartX, chainStartY, chainStartZ := startX, startY, startZ // 当前链首的起始位置
	cx, cy, cz := chainStartX, chainStartY, chainStartZ             // 当前链内的放置游标

	for i, block := range placeBlocks {
		select {
		case <-tc.StopCh():
			hub.Emit("task_error", mustJSON(map[string]any{"error": "导入已取消"}))
			return
		default:
		}

		// 新命令链：链首从链首位置开始，并在旁边（x 方向 +1）为下一条链预留位置
		if block.ChainStart {
			cx, cy, cz = chainStartX, chainStartY, chainStartZ
			chainStartX++
		}

		// 1. 先传送到当前位置（保证机器人移动 + 区块加载）
		bm.TP(cx, cy+1, cz)
		debugLog("CB导入: [%d/%d] 传送 (%d,%d,%d)", i+1, total, cx, cy+1, cz)

		// 2. 放置命令方块。
		// 注意：网易租赁服不接受完整 states 格式（["facing_direction"=N,"conditional_bit"=B]）。
		// 必须与建筑导入一致：用 normalizeBlockStates 提取命令方块的裸 facing 数字（如 "3"），
		// conditional 由下面的 CommandBlockUpdate 包设置。
		blockName := getBlockName(block.Type, block.Conditional)
		fd := facingToDirection(block.Facing)
		condBit := 0
		if block.Conditional {
			condBit = 1
		}
		rawStates := fmt.Sprintf(`["facing_direction"=%d,"conditional_bit"=%d]`, fd, condBit)
		nm := normalizeBlockName(blockName)
		st := normalizeBlockStates(blockName, rawStates)

		var err error
		if st == "" || st == "[]" {
			err = bm.SendBlockCmd(fmt.Sprintf("setblock %d %d %d %s", cx, cy, cz, nm))
		} else {
			err = bm.SendBlockCmd(fmt.Sprintf("setblock %d %d %d %s %s", cx, cy, cz, nm, st))
		}
		if err != nil {
			debugLog("CB导入: [%d/%d] setblock 失败 (%d,%d,%d): %v", i+1, total, cx, cy, cz, err)
			hub.Emit("progress", mustJSON(map[string]any{
				"msg": fmt.Sprintf("放置失败 (%d,%d,%d): %v", cx, cy, cz, err),
			}))
			cx, cy, cz = nextCBPos(cx, cy, cz, block.Facing)
			continue
		}
		time.Sleep(50 * time.Millisecond)

		// 3. 发送 CommandBlockUpdate 包设置命令和数据
		if bm.conn != nil {
			mode := uint32(0)
			switch block.Type {
			case "repeating":
				mode = 1
			case "chain":
				mode = 2
			}
			bm.conn.WritePacket(&packet.CommandBlockUpdate{
				Block:              true,
				Position:           protocol.BlockPos{int32(cx), int32(cy), int32(cz)},
				Mode:               mode,
				NeedsRedstone:      block.NeedsRS,
				Conditional:        block.Conditional,
				Command:            block.Command,
				Name:               block.CustomName,
				ShouldTrackOutput:  true,
				ExecuteOnFirstTick: block.ExeFirst,
				TickDelay:          int32(block.TickDelay),
			})
			time.Sleep(30 * time.Millisecond)
		}

		// 4. 更新进度（游戏内 actionbar + 前端）
		progress := float64(i+1) / float64(total)
		sendActionbar(fmt.Sprintf("§a命令方块导入: §f%d/%d", i+1, total))
		hub.Emit("task_progress", mustJSON(map[string]any{"progress": progress}))
		hub.Emit("progress", mustJSON(map[string]any{
			"msg": fmt.Sprintf("[%d/%d] %s", i+1, total, truncateStr(block.Command, 50)),
		}))

		// 5. 链内按朝向移动到下一个位置
		cx, cy, cz = nextCBPos(cx, cy, cz, block.Facing)
	}

	sendActionbar("§a命令方块导入完成")
	hub.Emit("task_done", mustJSON(map[string]any{
		"msg": fmt.Sprintf("导入完成: %s (%d/%d 个方块)", proj.Name, total, total),
	}))
	debugLog("CB导入: 完成 %s (%d/%d)", proj.Name, total, total)
}

// nextCBPos 按朝向偏移得到下一个命令方块的放置位置。
func nextCBPos(x, y, z int, facing string) (int, int, int) {
	if off, ok := cbFacingOffset[facing]; ok {
		return x + off[0], y + off[1], z + off[2]
	}
	return x, y, z + 1 // 默认 z+
}

// getBlockName 获取命令方块类型名
func getBlockName(blockType string, conditional bool) string {
	switch blockType {
	case "repeating":
		if conditional {
			return "repeating_command_block"
		}
		return "repeating_command_block"
	case "chain":
		if conditional {
			return "chain_command_block"
		}
		return "chain_command_block"
	default:
		if conditional {
			return "command_block"
		}
		return "command_block"
	}
}

// facingToDirection 将朝向名转为 facing_direction 值（0-5）
func facingToDirection(facing string) int {
	for fd, name := range cbFacingNames {
		if name == facing {
			return fd
		}
	}
	return 3 // 默认 z+
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ========== API 端点 ==========

// handleCBImportPreview 导入预览（解析文件并返回统计）
func handleCBImportPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
		return
	}

	proj := parseCBProject(req.Content)

	// 统计
	total := 0
	pulse := 0
	repeating := 0
	chain := 0
	cond := 0
	uncond := 0
	hasDelay := 0
	breaks := 0

	for _, b := range proj.Blocks {
		if b.ChainBreak {
			breaks++
			continue
		}
		total++
		switch b.Type {
		case "impulse":
			pulse++
		case "repeating":
			repeating++
		case "chain":
			chain++
		}
		if b.Conditional {
			cond++
		} else {
			uncond++
		}
		if b.TickDelay > 0 {
			hasDelay++
		}
	}

	// 链结构预览
	chainCount := 0
	chainSizes := make([]int, 0)
	currentChainSize := 0
	for _, b := range proj.Blocks {
		if b.ChainBreak {
			if currentChainSize > 0 {
				chainSizes = append(chainSizes, currentChainSize)
				chainCount++
				currentChainSize = 0
			}
			continue
		}
		currentChainSize++
		if b.Type == "impulse" || b.Type == "repeating" {
			if currentChainSize > 1 {
				chainSizes = append(chainSizes, currentChainSize)
				chainCount++
				currentChainSize = 0
			}
			currentChainSize = 1
		}
	}
	if currentChainSize > 0 {
		chainSizes = append(chainSizes, currentChainSize)
		chainCount++
		_ = chainSizes
	}

	writeJSON(w, map[string]any{
		"ok":      true,
		"project": proj,
		"stats": map[string]any{
			"total":         total,
			"pulse":         pulse,
			"repeating":     repeating,
			"chain":         chain,
			"conditional":   cond,
			"unconditional": uncond,
			"has_delay":     hasDelay,
			"breaks":        breaks,
			"chain_count":   chainCount,
		},
	})
}

// handleCBImportStart 开始导入
func handleCBImportStart(w http.ResponseWriter, r *http.Request) {
	var req CBImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
		return
	}

	if req.Content == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少工程内容"})
		return
	}

	// 检查机器人连接
	botMgrMu.Lock()
	connected := botMgr != nil && botMgr.IsConnected()
	botMgrMu.Unlock()
	if !connected {
		writeJSON(w, map[string]any{"ok": false, "error": "未连接到服务器，请先连接"})
		return
	}

	// 通过任务系统启动导入
	taskMu.Lock()
	defer taskMu.Unlock()
	if activeTask != nil && activeTask.Running() {
		writeJSON(w, map[string]any{"ok": false, "error": "已有任务运行中"})
		return
	}

	tc := state.NewTaskController(hub)
	activeTask = tc

	params := map[string]any{
		"content": req.Content,
		"start_x": req.StartX,
		"start_y": req.StartY,
		"start_z": req.StartZ,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				hub.Emit("task_error", mustJSON(map[string]any{"error": fmt.Sprintf("导入异常崩溃: %v", r)}))
			}
			taskMu.Lock()
			if activeTask == tc {
				activeTask = nil
			}
			taskMu.Unlock()
		}()
		runCBImportTask(tc, params)
	}()

	writeJSON(w, map[string]any{"ok": true})
}
