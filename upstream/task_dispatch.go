package main

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bot-apk/building"
	"bot-apk/media"
	"bot-apk/state"
	"bytes"
	"compress/gzip"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/nbt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/utils"

	"github.com/google/uuid"
)

// brandName 品牌名称，用于进度条标题显示
var brandName = "游园"

func dispatchTaskWithResume(tc *state.TaskController, taskType string, params json.RawMessage, resumeID string) {
	var p map[string]any
	json.Unmarshal(params, &p)
	var workers []*BotManager
	if taskType == "import_multi" {
		var err error
		workers, err = yySelectImportWorkers(getBotManager(), getFleetBots(), p)
		if err != nil {
			hub.Emit("task_error", mustJSON(map[string]any{"error": err.Error()}))
			return
		}
	}
	if taskType == "import" || taskType == "export" || taskType == "import_multi" || taskType == "repair" || taskType == "mapart" || taskType == "skin" {
		if taskType == "import_multi" {
			for _, bm := range workers {
				defer yyBeginSpatialJob(bm, taskType)()
			}
		} else {
			defer yyBeginSpatialJob(getBotManager(), taskType)()
		}
	}
	switch taskType {
	case "import":
		runImportTask(tc, p, resumeID)
	case "import_multi":
		runMultiImportTask(tc, p, workers, resumeID)
	case "repair":
		runRepairTask(tc, p)
	case "export":
		runExportTask(tc, p)
	case "mapart":
		runMapArtTask(tc, p, resumeID)
	case "skin":
		runSkinTask(tc, p)
	default:
		hub.Emit("task_error", mustJSON(map[string]any{"error": "未知的任务类型：" + taskType}))
	}
}

func dispatchTask(tc *state.TaskController, taskType string, params json.RawMessage) {
	dispatchTaskWithResume(tc, taskType, params, "")
}

func rawParams(p map[string]any) json.RawMessage {
	b, _ := json.Marshal(p)
	return b
}

func getBotManager() *BotManager {
	botMgrMu.Lock()
	defer botMgrMu.Unlock()
	return botMgr
}

func normalizeBlockName(name string) string {
	return strings.TrimPrefix(name, "minecraft:")
}

func normalizeBlockStates(name, states string) string {
	// 蜂巢：honey_level 由 BlockActorData NBT 控制，不在 setblock 状态中设置
	if strings.Contains(name, "beehive") || strings.Contains(name, "bee_nest") {
		if states != "" && states != "[]" {
			states = stripStateKey(states, "honey_level")
		}
		return states
	}
	if !strings.HasSuffix(name, "command_block") || states == "" || states == "[]" {
		return states
	}
	// 提取 facing_direction 的数值，兼容带引号 ["facing_direction"=4] 与
	// 不带引号 [facing_direction=4] 两种格式（BDX 生成不带引号的格式）。
	idx := strings.Index(states, "facing_direction")
	if idx < 0 {
		return ""
	}
	eq := strings.IndexByte(states[idx+len("facing_direction"):], '=')
	if eq < 0 {
		return ""
	}
	valStart := idx + len("facing_direction") + eq + 1
	j := valStart
	for j < len(states) && states[j] >= '0' && states[j] <= '9' {
		j++
	}
	if j > valStart {
		// 网易 setblock 不接受 tileData=0，用默认 facing (down)
		if states[valStart:j] == "0" {
			return ""
		}
		return states[valStart:j]
	}
	return ""
}

// stripStateKey 从方块状态字符串中移除指定键（例如 honey_level）。
// 格式示例：["honey_level"=5,"direction"=0] → ["direction"=0]
func stripStateKey(states, key string) string {
	pat := key + "\"="
	// 匹配 ,"key"=val 或 ["key"=val
	// 从值开始删到下一个逗号或结尾
	for i := 0; i < len(states); i++ {
		if strings.HasPrefix(states[i:], pat) {
			start := i
			end := start + len(pat)
			// 跳过数字值
			for end < len(states) && states[end] >= '0' && states[end] <= '9' {
				end++
			}
			// 跳过前面的逗号或左括号
			if start > 0 && states[start-1] == ',' {
				start--
			}
			// 跳过后面的逗号
			if end < len(states) && states[end] == ',' {
				end++
			}
			states = states[:start] + states[end:]
			i = start - 1
		}
	}
	// 清理空的方括号
	if states == "[\"\"]" || states == "[]" {
		return "[]"
	}
	return states
}

func shortName(path string) string {
	// 文件名去后缀
	name := filepath.Base(path)
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[:i]
		}
	}
	return name
}

func sendActionbar(msg string) {
	bm := getBotManager()
	if bm != nil && bm.IsConnected() {
		escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(msg)
		bm.SendWOCmd(fmt.Sprintf(`titleraw @a actionbar {"rawtext":[{"text":"%s"}]}`, escaped))
	}
}

var yyDebugLogMu sync.Mutex

func debugLog(format string, args ...any) {
	if yyDataRoot == "" {
		return
	}
	yyDebugLogMu.Lock()
	defer yyDebugLogMu.Unlock()
	path := filepath.Join(yyDataRoot, "youyuan_debug.log")
	if info, e := os.Stat(path); e == nil && info.Size() > 8*1024*1024 {
		_ = os.Remove(path + ".1")
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format("15:04:05.000")+" "+format+"\n", args...)
}

// A requested resume must have a complete, typed position. Empty/corrupt
// checkpoint data must never silently become an import from the origin.
func yyImportResumeData(cp *state.Checkpoint) (building.ImportCheckpointData, error) {
	var result building.ImportCheckpointData
	if cp == nil || cp.Type != "import" {
		return result, fmt.Errorf("导入断点类型无效")
	}
	var saved struct {
		LastCX       *int `json:"last_cx"`
		LastCZ       *int `json:"last_cz"`
		BlocksPlaced *int `json:"blocks_placed"`
	}
	if err := json.Unmarshal(cp.Data, &saved); err != nil {
		return result, fmt.Errorf("导入断点数据损坏: %w", err)
	}
	if saved.LastCX == nil || saved.LastCZ == nil || saved.BlocksPlaced == nil ||
		*saved.LastCX < 0 || *saved.LastCZ < 0 || *saved.BlocksPlaced < 0 {
		return result, fmt.Errorf("导入断点缺少有效恢复位置")
	}
	result.LastCX, result.LastCZ, result.BlocksPlaced = *saved.LastCX, *saved.LastCZ, *saved.BlocksPlaced
	return result, nil
}

func runImportTask(tc *state.TaskController, p map[string]any, resumeFromCheckpoint string) {
	path := getString(p, "path", "")
	if path == "" {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "缺少文件路径"}))
		return
	}
	// Validate before file parsing, bot commands, task creation or pre-clear.
	// Keep the validated snapshot instead of reloading and ignoring later errors.
	var resumeCheckpoint *state.Checkpoint
	var resumeData building.ImportCheckpointData
	if resumeFromCheckpoint != "" {
		var err error
		resumeCheckpoint, err = state.LoadCheckpoint(resumeFromCheckpoint)
		if err == nil {
			resumeData, err = yyImportResumeData(resumeCheckpoint)
		}
		if err != nil {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "无法恢复导入，未开始新任务，原断点已保留: " + err.Error()}))
			return
		}
	}

	bm := getBotManager()
	hub.Emit("task_start", mustJSON(map[string]any{"type": "导入: " + shortName(path)}))
	hub.Emit("task_progress", mustJSON(map[string]any{"progress": 0}))
	hub.Emit("progress", mustJSON(map[string]any{"msg": "加载建筑文件: " + shortName(path)}))

	// 输出缓存命中状态
	if building.IsCached(path) {
		hub.Emit("progress", mustJSON(map[string]any{"msg": "使用已解析的建筑数据"}))
	} else {
		hub.Emit("progress", mustJSON(map[string]any{"msg": "正在解析建筑文件"}))
	}

	data, err := building.TakeStructureFileCached(path, yyImportRegion(p)...)
	if err == nil && data != nil {
		debugLog("CACHE: import cache %s (%d blocks, %dx%dx%d)", shortName(path), len(data.Blocks), data.SizeX, data.SizeY, data.SizeZ)
	}
	if err != nil {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "加载文件失败: " + err.Error()}))
		return
	}

	// Apply rotation before import
	rotation := getInt(p, "rotation", 0)
	if rotation != 0 {
		hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("旋转建筑: %d°", rotation)}))
		building.RotateBlocksInPlace(data, rotation)
	}

	stats := data.ComputeStats()
	hub.Emit("progress", mustJSON(map[string]any{
		"msg": fmt.Sprintf("建筑: %dx%dx%d, %d 实体方块, %d 种方块", data.SizeX, data.SizeY, data.SizeZ, stats.SolidBlocks, len(stats.BlockCountsList)),
	}))
	if bm != nil && bm.IsOP() {
		bm.yyTaskNotice(fmt.Sprintf("§b游园 · 建筑写入 §f%s §7| %dx%dx%d · %d方块 · 目标%d/秒 · (%d,%d,%d)",
			shortName(path), data.SizeX, data.SizeY, data.SizeZ, stats.SolidBlocks, getInt(p, "speed", 9500), getInt(p, "x", 0), getInt(p, "y", 64), getInt(p, "z", 0)))
	}

	bm = getBotManager()
	if bm == nil || !bm.IsConnected() {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "未连接到服务器，请先连接"}))
		return
	}
	// Create checkpoint
	taskID := uuid.New().String()
	serverCode := ""
	if bm != nil {
		serverCode = bm.ServerCode()
	}
	cp := &state.Checkpoint{
		TaskID:     taskID,
		Type:       "import",
		Status:     "running",
		Progress:   0,
		CreatedAt:  time.Now().Unix(),
		Params:     rawParams(p),
		Name:       shortName(path),
		ServerCode: serverCode,
	}
	state.SaveCheckpointNow(cp)

	// Force creative mode before import (silent)
	if bm != nil {
		bm.SendCommand("/gamemode creative " + bm.quotedName())
		time.Sleep(300 * time.Millisecond)
	}

	// Load resume data if any
	startChunkX := 0
	startChunkZ := 0
	// 恢复导入时跳过预清空：断点之前已建好的区域不应被重新清空，否则恢复等同于从头再来。
	// 仅当断点确有进度（放置过方块）才跳过；无进度（如中断在预清空阶段）时仍正常预清空。
	resuming := false
	if resumeFromCheckpoint != "" {
		{
			c, rd := resumeCheckpoint, resumeData
			resuming = rd.BlocksPlaced > 0 || rd.LastCX > 0 || rd.LastCZ > 0
			// Follow the same serpentine region order as Run. Replay the saved
			// region and two predecessors conservatively after interruption.
			var resumeErr error
			startChunkX, startChunkZ, resumeErr = building.ImportResumeStart(rd.LastCX, rd.LastCZ, data.SizeX, data.SizeZ, getInt(p, "region_mode", 1), 2)
			if resumeErr != nil {
				cp.Status, cp.Message = "failed", resumeErr.Error()
				state.SaveCheckpointNow(cp)
				hub.Emit("task_error", mustJSON(map[string]any{"error": resumeErr.Error()}))
				return
			}
			hub.Emit("progress", mustJSON(map[string]any{
				"msg": fmt.Sprintf("从断点恢复: 区块 (%d,%d)（沿实际遍历倒回 2 个区域重放）", startChunkX, startChunkZ),
			}))
			// 恢复的旧断点由本次新任务接管，删除以免任务列表残留一条卡在"运行中"的陈旧记录
			if err := yyAdoptImportCheckpoint(cp, c); err != nil {
				hub.Emit("task_error", mustJSON(map[string]any{"error": "恢复断点接管失败，旧断点已保留: " + err.Error()}))
				return
			}
			state.DeleteCheckpoint(resumeFromCheckpoint)
		}
	}

	importCmds := getBool(p, "import_commands", true)
	cmdDisabled := getBool(p, "cmd_disabled", false)
	excludeFluids := getBool(p, "exclude_fluids", false)
	excludeWater := excludeFluids && getBool(p, "exclude_water", true)
	excludeWaterlogged := excludeFluids && getBool(p, "exclude_waterlogged", true)
	excludeLava := excludeFluids && getBool(p, "exclude_lava", true)
	dimension := getString(p, "dimension", "overworld")
	if bm != nil {
		bm.SetDimension(dimension)
	}
	regionMode := getInt(p, "region_mode", 1)
	if regionMode < 1 {
		regionMode = 1
	}
	if regionMode > 3 {
		regionMode = 3
	}
	// keep 模式：不破坏原有地形地貌，setblock/fill 追加 keep、NBT 方块只放空气位。
	// 与预清空互斥——保留模式本质就是不清空目标区，故强制 PreClearMode=0。
	keepMode := getBool(p, "keep_mode", false)
	preClearMode := getInt(p, "pre_clear_mode", 0)
	if keepMode {
		preClearMode = 0
	}
	// 排除NBT：带 NBT 的特殊方块（容器/旗帜/告示牌/命令/结构方块）按普通方块放置，跳过 OnNBT。
	// 仅单机器人生效（多机器人 multi_bot.go 不读取该键）。
	excludeNBT := getBool(p, "exclude_nbt", false)

	hub.Emit("progress", mustJSON(map[string]any{"msg": "正在计算实体方块..."}))
	stats = data.ComputeStats()
	totalSolid := 0
	for _, bi := range data.Blocks {
		if bi != 0 {
			totalSolid++
		}
	}

	var (
		abMu              sync.Mutex
		abTotalBarPct     int
		abSubBarPct       int
		abBlocks          int
		abRegionIdx       int
		abTotalRegions    int
		abClearing        bool
		abClearCleared    int
		abClearTotal      int
		targetImportSpeed int
		// Chunk counters come straight from the importer loop, so the panel can
		// show a measured 56/80 instead of deriving it from the block ratio.
		abChunkDone  int
		abChunkTotal int
		abChunkCX    int
		abChunkCZ    int
	)
	// 获取自定义品牌名用于进度条
	importBrandName := "游园"
	if n := GetEffectiveName(); n != "" {
		if len([]rune(n)) > 8 {
			n = string([]rune(n)[:8])
		}
		importBrandName = n
	}
	importStartTime := time.Now()
	abStopTicker := make(chan struct{})
	var consoleCenter protocol.BlockPos
	task := &building.ImportTask{
		Data:         data,
		BaseX:        getInt(p, "x", 0),
		BaseY:        getInt(p, "y", 64),
		BaseZ:        getInt(p, "z", 0),
		Speed:        getInt(p, "speed", -1),
		StartCX:      startChunkX,
		StartCZ:      startChunkZ,
		SkipPreClear: resuming, // 恢复时不清空已建区域（见上方 resuming 逻辑）
		PreClearMode: preClearMode,
		KeepMode:     keepMode,
		IsAir:        botIsAir(bm),
		ExcludeNBT:   excludeNBT,
		Dimension:    dimension,
		RegionMode:   regionMode,
		ChunkLoaded: func(cx, cz int) bool {
			if bm == nil || bm.conn == nil {
				return true
			}
			// 用 testforblock 检测区块中心是否加载：区块已加载时命令能成功返回
			// （无论目标方块是否存在），未加载时命令超时或报错。
			posX := cx*16 + 8
			posZ := cz*16 + 8
			response, isTimeout, err := bm.SendWSCommandWithTimeout(
				fmt.Sprintf("testforblock %d 1 %d minecraft:air", posX, posZ),
				1*time.Second,
			)
			return yyImportChunkProbeLoaded(response, isTimeout, err)
		},
		DenyEnable:         getBool(p, "deny_enable", false),
		DenyYOffset:        getInt(p, "deny_y_offset", -1),
		BorderEnable:       getBool(p, "border_enable", false),
		BorderBlock:        getString(p, "border_block", "border_block"),
		BorderYRel:         getInt(p, "border_y", 0),
		ImportCommands:     importCmds,
		CmdDisabled:        cmdDisabled,
		ExcludeWater:       excludeWater,
		ExcludeWaterlogged: excludeWaterlogged,
		ExcludeLava:        excludeLava,
		StopOnError:        true,
		SetBlock: func(x, y, z int32, name, states string) error {
			nm, st := normalizeBlockName(name), normalizeBlockStates(name, states)
			suffix := ""
			if keepMode && name != "minecraft:air" && name != "minecraft:deny" {
				suffix = " keep"
			}
			if st == "" || st == "[]" {
				return bm.SendBlockCmd(fmt.Sprintf("setblock %d %d %d %s%s", x, y, z, nm, suffix))
			}
			return bm.SendBlockCmd(fmt.Sprintf("setblock %d %d %d %s %s%s", x, y, z, nm, st, suffix))
		},
		FillRegion: func(x1, y1, z1, x2, y2, z2 int32, name, states string) error {
			nm, st := normalizeBlockName(name), normalizeBlockStates(name, states)
			suffix := ""
			if keepMode && name != "minecraft:air" && name != "minecraft:deny" {
				suffix = " keep"
			}
			if st == "" || st == "[]" {
				return bm.SendBlockCmd(fmt.Sprintf("fill %d %d %d %d %d %d %s%s", x1, y1, z1, x2, y2, z2, nm, suffix))
			}
			return bm.SendBlockCmd(fmt.Sprintf("fill %d %d %d %d %d %d %s %s%s", x1, y1, z1, x2, y2, z2, nm, st, suffix))
		},
		Teleport: func(x, y, z int) error {
			return bm.TP(x, y, z)
		},
		OnClearProgress: func(cleared, total int) {
			abMu.Lock()
			abClearing = true
			abClearCleared = cleared
			abClearTotal = total
			abMu.Unlock()
			hub.Emit("clear_progress", mustJSON(map[string]any{
				"progress": float64(cleared) / float64(total),
				"cleared":  cleared,
				"total":    total,
			}))
		},
		OnClearDone: func() {
			abMu.Lock()
			abClearing = false
			abMu.Unlock()
			hub.Emit("clear_done", mustJSON(map[string]any{"done": true}))
		},
		OnNBT: func(x, y, z int32, nbtData map[string]any, blockName, blockStates string) {
			defer func() {
				if r := recover(); r != nil {
					hub.Emit("progress", mustJSON(map[string]any{
						"msg": fmt.Sprintf("NBT 方块处理异常 (%d,%d,%d %s): %v", x, y, z, blockName, r),
					}))
				}
			}()
			if bm == nil || bm.conn == nil {
				return
			}

			// 等待 setblock 生效后再处理 NBT

			// 告示牌：保持现有路径（自带传送 + 点击打开 + 写文字）
			if strings.Contains(blockName, "sign") {
				bm.TP(int(x), int(y+1), int(z))
				if err := bm.PlaceSign(x, y, z, blockName, blockStates, nbtData); err != nil {
					hub.Emit("progress", mustJSON(map[string]any{
						"msg": fmt.Sprintf("告示牌放置失败 (%d,%d,%d): %v", x, y, z, err),
					}))
				}
				return
			}

			// 指令方块: TP 靠近后发送 CommandBlockUpdate 包
			bm.TP(int(x), int(y+1), int(z))
			if cmd, ok := nbtData["Command"].(string); ok {
				mode := uint32(0)
				if strings.Contains(blockName, "repeating_command_block") {
					mode = 1
				} else if strings.Contains(blockName, "chain_command_block") {
					mode = 2
				}
				needsRS := false
				if m := nbtByte(nbtData, "LPRedstoneMode"); m != 0 {
					needsRS = true
				}
				if m := nbtByte(nbtData, "NeedsRedstone"); m != 0 {
					needsRS = true
				}
				if m := nbtByte(nbtData, "auto"); m == 0 {
					needsRS = true
				}
				if cmdDisabled && strings.Contains(blockName, "repeating_command_block") {
					needsRS = true
				}
				cond := nbtByte(nbtData, "conditionalMode") != 0
				if c := nbtByte(nbtData, "Conditional"); c != 0 {
					cond = true
				}
				if c := nbtByte(nbtData, "conditional_bit"); c != 0 {
					cond = true
				}
				trackOut := nbtByte(nbtData, "TrackOutput") != 0
				exeFirst := nbtByte(nbtData, "ExecuteOnFirstTick") != 0
				tickDelay := int32(nbtInt(nbtData, "TickDelay"))
				name, _ := nbtData["CustomName"].(string)
				bm.conn.WritePacket(&packet.CommandBlockUpdate{
					Block: true, Position: protocol.BlockPos{x, y, z},
					Mode: mode, NeedsRedstone: needsRS, Conditional: cond,
					Command: cmd, Name: name,
					ShouldTrackOutput: trackOut, ExecuteOnFirstTick: exeFirst,
					TickDelay: tickDelay,
				})
				return
			}

			// 结构方块: TP 靠近后发送 StructureBlockUpdate 包
			bm.TP(int(x), int(y+1), int(z))
			if strings.Contains(blockName, "structure_block") {
				upd := buildStructureBlockUpdate(nbtData)
				upd.Position = protocol.BlockPos{x, y, z}
				bm.conn.WritePacket(&upd)
				return
			}
			// TP 到目标位置确保区块加载（NBTAssigner 和 BlockActorData 都需要）
			bm.TP(int(x), int(y+1), int(z))
			time.Sleep(5 * time.Millisecond)

			handled := false

			// 需要工作台的判定复用 building.IsWorkspaceNBTBlock（§8.1 单一事实来源，
			// 与切岛分类器一致；空容器/空展示框等无可制作内容 → false，归普通单元）。
			needsNBT := building.IsWorkspaceNBTBlock(blockName, nbtData)

			if !handled {
				if needsNBT {
					var seq int
					if err := placeOneNBTBlock(bm, x, y, z, blockName, blockStates, nbtData, consoleCenter, &seq); err != nil {
						hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("游园 · NBT制作失败 (%d,%d,%d): %v", x, y, z, err)}))
					} else {
						handled = true
					}
				}

			} // 回退：简单方块实体补发 BlockActorData
			if !handled {
				if !strings.Contains(blockName, "command_block") &&
					!strings.Contains(blockName, "structure_block") &&
					!strings.Contains(blockName, "sign") {
					if nbtData["Items"] == nil && nbtData["Command"] == nil {
						norm := building.NormalizeBlockEntityNBT(blockName, nbtData)
						norm["x"] = x
						norm["y"] = y
						norm["z"] = z
						bm.conn.WritePacket(&packet.BlockActorData{
							Position: protocol.BlockPos{x, y, z},
							NBTData:  norm,
						})
						time.Sleep(10 * time.Millisecond)
					}
				}
			}
		},
		OnProgress: func(cx, cz, chunk, total, blocks int) {
			progress := float64(chunk) / float64(total)
			if progress > 1.0 {
				progress = 1.0
			}
			abMu.Lock()
			abTotalBarPct = int(progress * 100)
			abBlocks = blocks
			abChunkDone = chunk
			abChunkTotal = total
			abChunkCX = cx
			abChunkCZ = cz
			abMu.Unlock()
			hub.Emit("task_progress", mustJSON(map[string]any{
				"progress":     progress,
				"sub_progress": 1.0,
				"chunk":        chunk,
				"total":        total,
				"blocks":       blocks,
				"total_blocks": totalSolid,
				"phase":        "建筑写入", "unit": "方块已提交", "location": "工具箱设备 → 当前游戏服务器",
			}))
			hub.Emit("progress", mustJSON(map[string]any{
				"msg": fmt.Sprintf("区块 %d/%d, %d 方块", chunk, total, blocks),
			}))
			// The same figures the in-game actionbar shows, so the toolbox panel
			// and the game panel can never disagree about progress.
			link := yyStructureHudWithHealth(yyStructureHud{})
			hub.Emit("structure_progress", mustJSON(map[string]any{
				"phase":        "导入中",
				"blocks":       blocks,
				"total_blocks": totalSolid,
				"chunks":       chunk,
				"chunks_total": total,
				"chunk_x":      cx * 16,
				"chunk_z":      cz * 16,
				"elapsed_ms":   time.Since(importStartTime).Milliseconds(),
				"structure":    shortName(path),
				"size_x":       data.SizeX,
				"size_y":       data.SizeY,
				"size_z":       data.SizeZ,
				"tps":          link.TPS,
				"tps_known":    link.TPSKnown,
				"rtt_ms":       link.RTT.Milliseconds(),
				"rtt_known":    link.RTTKnown,
			}))

			d := building.ImportCheckpointData{
				LastCX:       cx,
				LastCZ:       cz,
				BlocksPlaced: blocks,
			}

			cp.Data, _ = json.Marshal(d)
			cp.Message = fmt.Sprintf("区块 %d/%d", chunk, total)
			cp.Progress = progress
			state.SaveCheckpoint(cp)
		},
		OnSubProgress: func(cx, cz, placed, chunkTotal int) {
			sp := float64(placed) / float64(chunkTotal)
			if sp > 1.0 {
				sp = 1.0
			}
			if chunkTotal == 0 {
				sp = 1.0
			}
			abMu.Lock()
			abSubBarPct = int(sp * 100)
			abMu.Unlock()
			hub.Emit("task_progress", mustJSON(map[string]any{
				"sub_progress": sp,
			}))
		},
		OnRegionProgress: func(ri, totalRegions, rx, rz int) {
			abMu.Lock()
			abRegionIdx = ri
			abTotalRegions = totalRegions
			abMu.Unlock()
		},
		StopCh: tc.StopCh(),
	}
	targetImportSpeed = task.Speed
	// 设置 NBT 操作台位置并清空旧缓存（每个导入任务重建，确保位置正确）
	if bm != nil {
		// 工作台坐标按机器人编号分配，跨机器人互不重叠（§8.3）
		wx, wy, wz := building.WorkspaceCenter(bm.BotIndex, getInt(p, "x", 0), getInt(p, "z", 0), data.SizeX, data.SizeZ)
		consoleCenter = protocol.BlockPos{int32(wx), int32(wy), int32(wz)}
		// 异步清理工作区块（fire-and-forget，不阻塞导入启动）
		bmCopy := bm
		centerCopy := consoleCenter
		go func() {
			if bmCopy != nil && bmCopy.conn != nil && centerCopy != (protocol.BlockPos{}) {
				bmCopy.SendPlayerCommand(fmt.Sprintf("execute as @s at @s run fill %d %d %d %d %d %d air",
					centerCopy[0]-5, centerCopy[1]-2, centerCopy[2]-5,
					centerCopy[0]+5, centerCopy[1]+2, centerCopy[2]+5))
			}
		}()
		bm.cleanupNBTWorkspace()
	}

	// 定时刷新 actionbar（每 500ms），让旋转动画连续不卡顿
	abTicker := time.NewTicker(500 * time.Millisecond)
	defer abTicker.Stop()
	go func() {
		var speedLastBlocks int
		var speedLastTime = time.Now()
		var lastRealSpeed int
		for {
			select {
			case <-abTicker.C:
				abMu.Lock()
				clearing := abClearing
				if clearing {
					cCleared := abClearCleared
					cTotal := abClearTotal
					abMu.Unlock()
					sendActionbar(buildClearActionbar(cCleared, cTotal))
				} else {
					blocks := abBlocks
					totalPct := abTotalBarPct
					subPct := abSubBarPct
					rIdx := abRegionIdx
					rTotal := abTotalRegions
					cDone, cTotal, cCX, cCZ := abChunkDone, abChunkTotal, abChunkCX, abChunkCZ
					abMu.Unlock()
					elapsed := time.Since(importStartTime).Seconds()
					// 每秒更新一次速度，非更新时刻复用上次速度
					now := time.Now()
					realSpeed := lastRealSpeed
					if now.Sub(speedLastTime) >= time.Second {
						dt := now.Sub(speedLastTime).Seconds()
						if dt >= 0.5 {
							realSpeed = int(float64(blocks-speedLastBlocks) / dt)
							speedLastBlocks = blocks
							speedLastTime = now
							lastRealSpeed = realSpeed
						}
					}
					// 第一秒内用平均速度估算
					if lastRealSpeed == 0 && elapsed > 0.1 {
						realSpeed = int(float64(blocks) / elapsed)
					}
					buildV5Actionbar(sendActionbar, shortName(path), data.SizeX, data.SizeY, data.SizeZ,
						regionMode, totalSolid, targetImportSpeed, realSpeed, blocks, totalPct, subPct,
						rIdx, rTotal, cDone, cTotal, cCX, cCZ, importStartTime, importBrandName)
				}
			case <-abStopTicker:
				return
			}
		}
	}()
	defer close(abStopTicker)

	// 严格失败条件：监听机器人连接，断开（含主机器人丢失）即中断任务并标记失败，
	// 不依赖 setblock/fill 碰巧报错才兜底
	watchDone := make(chan struct{})
	var lostConn atomic.Bool
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-watchDone:
				return
			case <-t.C:
				bmNow := getBotManager()
				if bmNow == nil || !bmNow.IsConnected() {
					lostConn.Store(true)
					tc.Stop() // 中断 task.Run（观察 StopCh）
					return
				}
			}
		}
	}()

	// 导入期间抑制游戏内消息（聊天/加入/退出等）转发到终端，避免大量提示冲击
	setSuppressGameLogs(true)
	err = task.Run()
	close(watchDone)
	if status, message := yyImportInterruptedOutcome(tc, lostConn.Load(), err); status != "" {
		setSuppressGameLogs(false)
		cp.Status, cp.Message = status, message
		state.SaveCheckpointNow(cp)
		if status == "paused" {
			hub.Emit("task_paused", mustJSON(map[string]any{"task_id": cp.TaskID, "msg": message}))
		} else {
			hub.Emit("task_error", mustJSON(map[string]any{"error": message}))
		}
		return
	}
	setSuppressGameLogs(false)

	// 完成提示：聊天栏 tellraw + 行动栏强调
	if bm != nil && bm.IsOP() {
		bm.yyTaskNotice(fmt.Sprintf("§b游园 · 建筑提交完成 §f%s §7| %d方块", shortName(path), totalSolid))
	}

	buildV5Actionbar(sendActionbar, shortName(path), data.SizeX, data.SizeY, data.SizeZ,
		regionMode, totalSolid, targetImportSpeed, 0, totalSolid, 100, 100, 0, 0, 0, 0, 0, 0, importStartTime, importBrandName)

	cp.Status = "done"
	// 清空工作区方块
	if bm != nil && bm.conn != nil && consoleCenter != (protocol.BlockPos{}) {
		bm.SendPlayerCommand(fmt.Sprintf("fill %d %d %d %d %d %d air",
			consoleCenter[0]-5, consoleCenter[1]-2, consoleCenter[2]-5,
			consoleCenter[0]+5, consoleCenter[1]+2, consoleCenter[2]+5))
	}
	// 任务结束：删除本机 NBT 缓存的所有结构（相同方块复用同一 uniqueID，统一在此清理）
	bm.mu.Lock()
	if bm.nbtAssigner != nil {
		bm.nbtAssigner.CleanCache()
	}
	bm.mu.Unlock()
	bm.cleanupNBTWorkspace()

	cp.Progress = 1.0
	state.SaveCheckpointNow(cp)
	// 记录心跳增量
	recordHeartbeatIncrement(IncrementData{
		BlocksImported:    totalSolid,
		BuildingsImported: 1,
		ImportSessions:    1,
	})
	hub.Emit("task_done", mustJSON(map[string]any{"msg": "导入完成", "task_id": taskID}))
}

func runRepairTask(tc *state.TaskController, p map[string]any) {
	path := getString(p, "path", "")
	if path == "" {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "缺少文件路径"}))
		return
	}

	// 修补模式必须提供起始坐标，不做默认值
	x := getInt(p, "x", 0)
	y := getInt(p, "y", 0)
	z := getInt(p, "z", 0)
	if x == 0 && y == 0 && z == 0 {
		// 检查是否真的都是 0，还是用户没传
		if _, hasX := p["x"]; !hasX {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "修补模式必须提供起始坐标 (x, y, z)"}))
			return
		}
		if _, hasY := p["y"]; !hasY {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "修补模式必须提供起始坐标 y"}))
			return
		}
		if _, hasZ := p["z"]; !hasZ {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "修补模式必须提供起始坐标 z"}))
			return
		}
	}

	hub.Emit("task_start", mustJSON(map[string]any{"type": "修补: " + shortName(path)}))
	hub.Emit("task_progress", mustJSON(map[string]any{"progress": 0}))
	hub.Emit("progress", mustJSON(map[string]any{"msg": "加载缓存的建筑文件: " + shortName(path)}))

	// 加载缓存（必须已存在，即之前已导入过此文件）
	data, err := building.TakeStructureFileCached(path, yyImportRegion(p)...)
	if err != nil || data == nil {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "无法加载建筑文件缓存，请先正常导入该文件"}))
		return
	}

	rotation := getInt(p, "rotation", 0)
	if rotation != 0 {
		hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("旋转建筑: %d°", rotation)}))
		building.RotateBlocksInPlace(data, rotation)
	}

	stats := data.ComputeStats()
	hub.Emit("progress", mustJSON(map[string]any{
		"msg": fmt.Sprintf("建筑: %dx%dx%d, %d 实体方块, 修补坐标:(%d,%d,%d)", data.SizeX, data.SizeY, data.SizeZ, stats.SolidBlocks, x, y, z),
	}))

	bm := getBotManager()
	if bm == nil || !bm.IsConnected() {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "未连接到服务器，请先连接"}))
		return
	}

	// 修补模式默认速度 20 方块/秒，不跟随全局速度
	speed := getInt(p, "speed", 20)

	// 解析修补区域参数
	useCircle := getBool(p, "repair_circle", false)
	repairCX := int32(getInt(p, "repair_cx", 0))
	repairCZ := int32(getInt(p, "repair_cz", 0))
	repairRadius := int32(getInt(p, "repair_radius", 0))
	repairX1 := int32(getInt(p, "repair_x1", 0))
	repairY1 := int32(getInt(p, "repair_y1", 0))
	repairZ1 := int32(getInt(p, "repair_z1", 0))
	repairX2 := int32(getInt(p, "repair_x2", 0))
	repairY2 := int32(getInt(p, "repair_y2", 0))
	repairZ2 := int32(getInt(p, "repair_z2", 0))

	importCmds := getBool(p, "import_commands", true)
	cmdDisabled := getBool(p, "cmd_disabled", false)
	dimension := getString(p, "dimension", "overworld")
	if bm != nil {
		bm.SetDimension(dimension)
	}
	excludeWater := getBool(p, "exclude_water", false)
	excludeWaterlogged := getBool(p, "exclude_waterlogged", false)
	excludeLava := getBool(p, "exclude_lava", false)

	// 强制 creative 模式
	if bm != nil {
		bm.SendCommand("/gamemode creative " + bm.quotedName())
		time.Sleep(300 * time.Millisecond)
	}

	var consoleCenter protocol.BlockPos
	nbtStructSeq := 0

	task := &building.ImportTask{
		Data:               data,
		BaseX:              x,
		BaseY:              y,
		BaseZ:              z,
		Speed:              speed,
		ImportCommands:     importCmds,
		CmdDisabled:        cmdDisabled,
		ExcludeWater:       excludeWater,
		ExcludeWaterlogged: excludeWaterlogged,
		ExcludeLava:        excludeLava,
		StopOnError:        true,
		RepairMode:         true,
		RepairNBTOnly:      getBool(p, "repair_nbt_only", false),
		RepairUseCircle:    useCircle,
		RepairCenterX:      repairCX,
		RepairCenterZ:      repairCZ,
		RepairRadius:       repairRadius,
		RepairRect:         [6]int32{repairX1, repairY1, repairZ1, repairX2, repairY2, repairZ2},

		SetBlock: func(x, y, z int32, name, states string) error {
			nm, st := normalizeBlockName(name), normalizeBlockStates(name, states)
			if st == "" || st == "[]" {
				return bm.SendBlockCmd(fmt.Sprintf("setblock %d %d %d %s", x, y, z, nm))
			}
			return bm.SendBlockCmd(fmt.Sprintf("setblock %d %d %d %s %s", x, y, z, nm, st))
		},
		Teleport: func(x, y, z int) error {
			return bm.TP(x, y, z)
		},
		OnNBT: func(x, y, z int32, nbtData map[string]any, blockName, blockStates string) {
			if bm == nil || bm.conn == nil {
				return
			}
			// 告示牌
			if strings.Contains(blockName, "sign") {
				bm.TP(int(x), int(y+1), int(z))
				if err := bm.PlaceSign(x, y, z, blockName, blockStates, nbtData); err != nil {
					hub.Emit("progress", mustJSON(map[string]any{
						"msg": fmt.Sprintf("告示牌放置失败 (%d,%d,%d): %v", x, y, z, err),
					}))
				}
				return
			}
			// 命令方块
			if cmd, ok := nbtData["Command"].(string); ok {
				bm.TP(int(x), int(y+1), int(z))
				mode := uint32(0)
				if strings.Contains(blockName, "repeating_command_block") {
					mode = 1
				} else if strings.Contains(blockName, "chain_command_block") {
					mode = 2
				}
				needsRS := false
				if m := nbtByte(nbtData, "LPRedstoneMode"); m != 0 {
					needsRS = true
				}
				if m := nbtByte(nbtData, "NeedsRedstone"); m != 0 {
					needsRS = true
				}
				if m := nbtByte(nbtData, "auto"); m == 0 {
					needsRS = true
				}
				if cmdDisabled && strings.Contains(blockName, "repeating_command_block") {
					needsRS = true
				}
				cond := nbtByte(nbtData, "conditionalMode") != 0
				if c := nbtByte(nbtData, "Conditional"); c != 0 {
					cond = true
				}
				if c := nbtByte(nbtData, "conditional_bit"); c != 0 {
					cond = true
				}
				trackOut := nbtByte(nbtData, "TrackOutput") != 0
				exeFirst := nbtByte(nbtData, "ExecuteOnFirstTick") != 0
				tickDelay := int32(nbtInt(nbtData, "TickDelay"))
				name, _ := nbtData["CustomName"].(string)
				bm.conn.WritePacket(&packet.CommandBlockUpdate{
					Block: true, Position: protocol.BlockPos{x, y, z},
					Mode: mode, NeedsRedstone: needsRS, Conditional: cond,
					Command: cmd, Name: name,
					ShouldTrackOutput: trackOut, ExecuteOnFirstTick: exeFirst,
					TickDelay: tickDelay,
				})
				return
			}
			// 结构方块
			if strings.Contains(blockName, "structure_block") {
				bm.TP(int(x), int(y+1), int(z))
				upd := buildStructureBlockUpdate(nbtData)
				upd.Position = protocol.BlockPos{x, y, z}
				bm.conn.WritePacket(&upd)
				return
			}
			// 需要 NBT 工作区的方块（容器、展示框等），复用 building 分类器（§8.1）
			needsNBT := building.IsWorkspaceNBTBlock(blockName, nbtData)
			if needsNBT {
				assigner, aErr := bm.getNBTAssigner(consoleCenter)
				if aErr == nil {
					bm.TP(int(consoleCenter[0]), int(consoleCenter[1]+1), int(consoleCenter[2]))
					time.Sleep(50 * time.Millisecond)
					statesMap := utils.ParseBlockStatesString(blockStates)
					_, _, _, _ = assigner.PlaceNBTBlock(blockName, statesMap, nbtData)
					cleanName := strings.ReplaceAll(strings.TrimPrefix(blockName, "minecraft:"), ":", "_")
					structID := building.NBTStructName(bm.BotIndex, cleanName, nbtStructSeq)
					nbtStructSeq++
					saveCmd := fmt.Sprintf("structure save \"%s\" %d %d %d %d %d %d", structID, consoleCenter[0], consoleCenter[1], consoleCenter[2], consoleCenter[0], consoleCenter[1], consoleCenter[2])
					bm.SendPlayerCommand(saveCmd)
					time.Sleep(100 * time.Millisecond)
					bm.TP(int(x), int(y+1), int(z))
					time.Sleep(100 * time.Millisecond)
					loadCmd := fmt.Sprintf("structure load \"%s\" %d %d %d", structID, x, y, z)
					bm.SendAICommand(loadCmd)
					time.Sleep(100 * time.Millisecond)
					bm.SendPlayerCommand(fmt.Sprintf("structure delete \"%s\"", structID))
				}
				return
			}
			// 其他方块实体：补发 BlockActorData
			if !strings.Contains(blockName, "command_block") &&
				!strings.Contains(blockName, "structure_block") &&
				!strings.Contains(blockName, "sign") {
				norm := building.NormalizeBlockEntityNBT(blockName, nbtData)
				norm["x"] = x
				norm["y"] = y
				norm["z"] = z
				bm.conn.WritePacket(&packet.BlockActorData{
					Position: protocol.BlockPos{x, y, z},
					NBTData:  norm,
				})
			}
		},
		OnProgress: func(cx, cz, chunk, total, blocks int) {
			progress := float64(chunk) / float64(total)
			if progress > 1.0 {
				progress = 1.0
			}
			hub.Emit("task_progress", mustJSON(map[string]any{"progress": progress}))
			hub.Emit("progress", mustJSON(map[string]any{
				"msg": fmt.Sprintf("区块 %d/%d, %d 方块", chunk, total, blocks),
			}))
		},
		StopCh: tc.StopCh(),
	}

	// 设置 NBT 操作台位置（同正常导入：距离起点最远的位置，按机器人编号分配不重叠 §8.3）
	if bm != nil {
		wx, wy, wz := building.WorkspaceCenter(bm.BotIndex, x, z, data.SizeX, data.SizeZ)
		consoleCenter = protocol.BlockPos{int32(wx), int32(wy), int32(wz)}
		go func() {
			if bm.conn != nil {
				bm.SendPlayerCommand(fmt.Sprintf("execute as @s at @s run fill %d %d %d %d %d %d air",
					consoleCenter[0]-5, consoleCenter[1]-2, consoleCenter[2]-5,
					consoleCenter[0]+5, consoleCenter[1]+2, consoleCenter[2]+5))
			}
		}()
		bm.cleanupNBTWorkspace()
	}

	hub.Emit("progress", mustJSON(map[string]any{"msg": "开始修补..."}))
	setSuppressGameLogs(true)
	err = task.Run()
	if err != nil {
		setSuppressGameLogs(false)
		hub.Emit("task_error", mustJSON(map[string]any{"error": fmt.Sprintf("修补失败: %v", err)}))
		return
	}
	setSuppressGameLogs(false)
	hub.Emit("task_done", mustJSON(map[string]any{"msg": "修补完成"}))
}

func runExportTask(tc *state.TaskController, p map[string]any) {
	sx := getInt(p, "x1", 0)
	sy := getInt(p, "y1", 0)
	sz := getInt(p, "z1", 0)
	ex := getInt(p, "x2", 16)
	ey := getInt(p, "y2", 16)
	ez := getInt(p, "z2", 16)

	bm := getBotManager()
	if bm == nil || !bm.IsConnected() {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "未连接到服务器"}))
		return
	}

	outputPath := getString(p, "output", "/sdcard/Download/export.mcstructure")
	format := getString(p, "format", "mcstructure")

	// Auto-rename if file exists: append (1), (2), ...
	if outputPath != "" {
		base := outputPath
		ext := ""
		if i := strings.LastIndex(base, "."); i >= 0 {
			ext = base[i:]
			base = base[:i]
		}
		for i := 1; i <= 1000; i++ {
			if _, err := os.Stat(outputPath); os.IsNotExist(err) {
				break
			}
			outputPath = fmt.Sprintf("%s(%d)%s", base, i, ext)
		}
	}

	if sx > ex {
		sx, ex = ex, sx
	}
	if sy > ey {
		sy, ey = ey, sy
	}
	if sz > ez {
		sz, ez = ez, sz
	}

	dx := ex - sx + 1
	dy := ey - sy + 1
	dz := ez - sz + 1

	hub.Emit("progress", mustJSON(map[string]any{
		"msg": fmt.Sprintf("游园 · 从当前游戏服务器读取建筑，文件保存到工具箱所在设备: (%d,%d,%d)→(%d,%d,%d) %dx%dx%d", sx, sy, sz, ex, ey, ez, dx, dy, dz),
	}))

	if bm.IsOP() {
		bm.yyTaskNotice(fmt.Sprintf("§6游园 · 建筑读取 §f%s §7| %dx%dx%d · 文件保存在工具箱设备", format, dx, dy, dz))
	}

	// Create checkpoint
	taskID := uuid.New().String()
	serverCode := ""
	if bm != nil {
		serverCode = bm.ServerCode()
	}
	cp := &state.Checkpoint{
		TaskID:     taskID,
		Type:       "export",
		Status:     "running",
		Progress:   0,
		CreatedAt:  time.Now().Unix(),
		Params:     rawParams(p),
		Name:       fmt.Sprintf("导出 %dx%dx%d", dx, dy, dz),
		ServerCode: serverCode,
	}
	state.SaveCheckpointNow(cp)

	// Tile reads are independent across configured same-server Fleet bots.
	// One bot keeps its position-sensitive operations in a single lane.
	origin, size, regionErr := yyRegionArgs([]any{[]any{sx, sy, sz}, []any{dx, dy, dz}})
	if regionErr != nil {
		cp.Status = "failed"
		cp.Message = regionErr.Error()
		state.SaveCheckpoint(cp)
		hub.Emit("task_error", mustJSON(map[string]any{"error": cp.Message}))
		return
	}
	totalChunks := len(yyRegionTiles(origin, size))
	allChunks, workersUsed, readErr := yyReadRegionTiles(bm, origin, size, max(1, getInt(p, "workers", 1)), tc.StopCh(), func(done, total int) {
		cp.Progress = float64(done) / float64(total)
		cp.Message = fmt.Sprintf("游园 · 读取 %d/%d 分区", done, total)
		state.SaveCheckpoint(cp)
		hub.Emit("task_progress", mustJSON(map[string]any{"progress": cp.Progress, "completed": done, "total": total, "phase": "游园 · 建筑读取", "unit": "分区", "location": "游戏服务器 → 工具箱设备"}))
	})
	if readErr != nil {
		cp.Status = "failed"
		cp.Message = readErr.Error()
		state.SaveCheckpoint(cp)
		hub.Emit("task_error", mustJSON(map[string]any{"error": "导出中止，未生成缺块文件: " + readErr.Error()}))
		return
	}
	successChunks := len(allChunks)
	mergedAll := mergeChunkStructures(allChunks, int32(dx), int32(dy), int32(dz))
	if mergedAll == nil {
		cp.Status = "failed"
		cp.Message = "分区数据不完整，未生成文件"
		state.SaveCheckpoint(cp)
		hub.Emit("task_error", mustJSON(map[string]any{"error": cp.Message}))
		return
	}
	hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("游园 · %d分区读取完成，实际协作机器人%d，保留方块实体NBT", successChunks, workersUsed)}))

	var outData []byte
	if format == "mcworld" {
		merged := mergedAll
		if err := building.WriteMergedStructureToMCWorld(merged, int32(dx), int32(dy), int32(dz), outputPath, int32(sx), int32(sy), int32(sz)); err != nil {
			cp.Status = "failed"
			cp.Message = err.Error()
			state.SaveCheckpoint(cp)
			hub.Emit("task_error", mustJSON(map[string]any{"error": "mcworld 导出失败: " + err.Error()}))
			return
		}
		hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("已写入: %s", outputPath)}))
	} else if format == "mcstructure" {
		mergedStructure := mergedAll
		var buf bytes.Buffer
		enc := nbt.NewEncoderWithEncoding(&buf, nbt.LittleEndian)
		root := map[string]any{
			"format_version":         int32(1),
			"size":                   []int32{int32(dx), int32(dy), int32(dz)},
			"structure":              mergedStructure,
			"structure_world_origin": []int32{int32(sx), int32(sy), int32(sz)},
		}
		if err := enc.Encode(root); err != nil {
			cp.Status = "failed"
			cp.Message = err.Error()
			state.SaveCheckpoint(cp)
			hub.Emit("task_error", mustJSON(map[string]any{"error": "编码失败: " + err.Error()}))
			return
		}
		outData = buf.Bytes()
	} else {
		if len(allChunks) == 0 {
			cp.Status = "failed"
			cp.Message = "没有区块数据可编码"
			state.SaveCheckpoint(cp)
			hub.Emit("task_error", mustJSON(map[string]any{"error": "没有区块数据可编码"}))
			return
		}
		merged := mergedAll
		if dx > 32767 || dy > 32767 || dz > 32767 {
			cp.Status = "failed"
			cp.Message = "Schematic尺寸超出short字段，请用mcstructure"
			state.SaveCheckpoint(cp)
			hub.Emit("task_error", mustJSON(map[string]any{"error": cp.Message}))
			return
		}
		outData = convertToSchematic(map[string]any{"structure": merged}, dx, dy, dz)
		if len(outData) == 0 {
			cp.Status = "failed"
			cp.Message = "Schematic编码失败，未输出文件"
			state.SaveCheckpoint(cp)
			hub.Emit("task_error", mustJSON(map[string]any{"error": cp.Message}))
			return
		}
	}

	if outputPath != "" && len(outData) > 0 {
		os.MkdirAll(filepath.Dir(outputPath), 0755)
		if err := os.WriteFile(outputPath, outData, 0644); err != nil {
			cp.Status = "failed"
			cp.Message = err.Error()
			state.SaveCheckpoint(cp)
			hub.Emit("task_error", mustJSON(map[string]any{"error": "写入失败: " + err.Error()}))
			return
		}
		hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("已写入: %s (%d 字节)", outputPath, len(outData))}))
	}

	cp.Status = "done"
	cp.Progress = 1.0
	cp.Message = fmt.Sprintf("已保存 %d/%d 区块到工具箱设备: %s", successChunks, totalChunks, outputPath)
	completion := "建筑导出完成"
	if successChunks < totalChunks {
		completion = "建筑部分导出（含缺失区块）"
	}
	if bm.IsOP() {
		sendActionbar(yyTaskActionbar(completion, filepath.Base(outputPath), "6", successChunks, totalChunks, 0, 0))
	}
	hub.Emit("task_progress", mustJSON(map[string]any{"progress": 1.0, "completed": successChunks, "total": totalChunks, "phase": completion, "unit": "区块已读取", "location": "文件已保存到工具箱设备"}))
	state.SaveCheckpoint(cp)
	recordHeartbeatIncrement(IncrementData{
		BuildingsExported: 1,
		ExportSessions:    1,
	})
	hub.Emit("task_done", mustJSON(map[string]any{
		"msg":     fmt.Sprintf("游园 · %s: %d/%d 区块, 工具箱设备文件 %s", completion, successChunks, totalChunks, outputPath),
		"output":  outputPath,
		"partial": successChunks < totalChunks,
		"format":  format,
	}))
}

func convertToSchematic(nbtData map[string]any, w, h, l int) []byte {
	// Build Java Edition schematic (Sponge v2 format, BigEndian NBT + gzip)
	// 注意：mcstructure 索引 = x*h*l + y*l + z，Sponge v2 索引 = y*w*l + z*w + x
	// 需要做坐标重映射
	structure, _ := nbtData["structure"].(map[string]any)
	blockIndices, _ := structure["block_indices"].([]any)
	paletteData, _ := structure["palette"].(map[string]any)

	// Build simplified palette and block data
	palette := map[string]int32{"minecraft:air": 0}
	nextID := int32(1)
	total := w * h * l
	blockData := make([]int32, total)

	if blockIndices != nil && len(blockIndices) > 0 {
		layer0 := toIntSlice(blockIndices[0])
		for x := 0; x < w; x++ {
			for y := 0; y < h; y++ {
				for z := 0; z < l; z++ {
					// mcstructure 索引
					mcIdx := x*h*l + y*l + z
					if mcIdx >= len(layer0) {
						continue
					}
					idx := layer0[mcIdx]
					if idx < 0 {
						continue // air
					}
					// Get block name from palette
					blockName := "minecraft:stone"
					if pal, ok := paletteData["default"].(map[string]any); ok {
						if bp, ok := pal["block_palette"].([]any); ok && idx < len(bp) {
							if bm, ok := bp[idx].(map[string]any); ok {
								if n, ok := bm["name"].(string); ok {
									blockName = n
								}
							}
						}
					}
					pid, ok := palette[blockName]
					if !ok {
						pid = nextID
						palette[blockName] = nextID
						nextID++
					}
					// Sponge v2 索引 = y*w*l + z*w + x
					spongeIdx := y*w*l + z*w + x
					blockData[spongeIdx] = pid
				}
			}
		}
	}

	// Preserve the full Bedrock palette and block entities for lossless YouYuan round trips.
	bedrockEntities := []any{}
	pd := yyStructurePalette(structure)
	positions, _ := pd["block_position_data"].(map[string]any)
	for key, v := range positions {
		var idx int
		if _, e := fmt.Sscanf(key, "%d", &idx); e != nil || idx < 0 || idx >= total {
			continue
		}
		row, _ := v.(map[string]any)
		entity, _ := row["block_entity_data"].(map[string]any)
		if entity == nil {
			continue
		}
		pos := []int32{int32(idx / (h * l)), int32(idx / l % h), int32(idx % l)}
		bedrockEntities = append(bedrockEntities, map[string]any{"Pos": pos, "NBT": entity})
	}

	root := map[string]any{
		"Version":     int32(2),
		"DataVersion": int32(0), "PaletteMax": nextID,
		"YouYuanBedrockStructure":     structure,
		"YouYuanBedrockBlockEntities": bedrockEntities,
		"Metadata":                    map[string]any{"YouYuanFormat": "Bedrock round-trip extension; Java NBT conversion not guaranteed"},
		"Width":                       int16(w), "Height": int16(h), "Length": int16(l),
		// Sponge v2 的 BlockData 是 VarInt 编码的字节数组（不是 []int32）
		"BlockData": encodeVarInts(blockData), "Palette": palette,
		"Offset": [3]int32{0, 0, 0},
	}

	var buf bytes.Buffer
	enc := nbt.NewEncoderWithEncoding(&buf, nbt.BigEndian)
	if err := enc.Encode(root); err != nil {
		return nil
	}
	// Gzip
	var gzbuf bytes.Buffer
	gw := gzip.NewWriter(&gzbuf)
	gw.Write(buf.Bytes())
	gw.Close()
	return gzbuf.Bytes()
}

// encodeVarInts 把调色板索引数组编码为 Sponge v2 的 VarInt 字节序列，
// 与 building/schematic.go 的 decodeVarInts 对应（导出→重导入需一致）。
func encodeVarInts(indices []int32) []byte {
	var buf bytes.Buffer
	for _, v := range indices {
		u := uint32(v)
		for {
			b := byte(u & 0x7F)
			u >>= 7
			if u != 0 {
				b |= 0x80
			}
			buf.WriteByte(b)
			if u == 0 {
				break
			}
		}
	}
	return buf.Bytes()
}

func toIntSlice(v any) []int {
	switch arr := v.(type) {
	case []any:
		r := make([]int, len(arr))
		for i, x := range arr {
			switch n := x.(type) {
			case int32:
				r[i] = int(n)
			case int:
				r[i] = n
			case float64:
				r[i] = int(n)
			}
		}
		return r
	case []int32:
		r := make([]int, len(arr))
		for i, n := range arr {
			r[i] = int(n)
		}
		return r
	}
	return nil
}

func makeUUIDSafe(id string) string {
	// Make UUID safe for Minecraft structure names (avoid censorship)
	result := make([]byte, 0, len(id)*2)
	for _, c := range id {
		if c == '-' {
			result = append(result, '_')
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

// mapArtBlock 存储像素画单个像素对应的方块信息和高度偏移。
// heightOff 值：-1（暗色）/ 0（中色）/ +1（亮色），用于地图上产生立体起伏。
type mapArtBlock struct {
	info      *building.BlockInfo
	heightOff int
}

var gravityReplacements = map[string]string{
	"minecraft:sand":     "minecraft:sandstone",
	"minecraft:red_sand": "minecraft:red_sandstone",
	"minecraft:gravel":   "minecraft:stone",
}

// MapArtCheckpointData 记录像素画的断点位置。
// 恢复粒度 = 32×32 区域：每个区域互相独立、方块放置幂等，区域整块做完了才计数。
// 崩溃/断开时"做了一半"的区域整体重做（浪费但正确，setblock/fill 同值两次结果一样）。
type MapArtCheckpointData struct {
	RegionsDone  int // 已完整完成的区域数（蛇形遍历顺序）
	BlocksPlaced int // 已放置方块数（用于进度）
}

func runMapArtTask(tc *state.TaskController, p map[string]any, resumeFromCheckpoint string) {
	path := getString(p, "path", "")
	if path == "" {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "缺少图片路径"}))
		return
	}

	bm := getBotManager()
	if bm == nil || !bm.IsConnected() {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "未连接到服务器"}))
		return
	}

	hub.Emit("progress", mustJSON(map[string]any{"msg": "加载图片: " + filepath.Base(path)}))
	if bm.IsOP() {
		bm.yyTaskNotice(fmt.Sprintf("§a§l▍地图画:§r§f%s §7坐标:(%d,%d,%d)", "像素画", getInt(p, "x", 0), getInt(p, "y", 64), getInt(p, "z", 0)))
	}

	f, err := os.Open(path)
	if err != nil {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "无法打开图片: " + err.Error()}))
		return
	}
	defer f.Close()

	srcImg, _, err := image.Decode(f)
	if err != nil {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "图片解码失败: " + err.Error()}))
		return
	}

	orientation := getString(p, "orientation", "vertical")
	speed := getInt(p, "speed", -1)
	useGravityBlocks := getBool(p, "use_gravity_blocks", false)
	gravityPlatform := getBool(p, "gravity_platform", false)
	useRelief := getBool(p, "use_relief", false)
	targetW := getInt(p, "target_w", 128)
	targetH := getInt(p, "target_h", 128)
	// Clamp to 128 multiples, max 1024
	if targetW < 128 {
		targetW = 128
	}
	if targetH < 128 {
		targetH = 128
	}
	if targetW > 1024 {
		targetW = 1024
	}
	if targetH > 1024 {
		targetH = 1024
	}

	baseX := getInt(p, "x", 0)
	baseY := getInt(p, "y", 64)
	baseZ := getInt(p, "z", 0)

	// 解析色彩空间和抖动算法参数
	colorSpaceStr := getString(p, "color_space", "lab")
	ditherStr := getString(p, "dither", "floyd_steinberg")
	cs := parseColorSpace(colorSpaceStr)
	dither := parseDitherAlgo(ditherStr)

	srcW, srcH := srcImg.Bounds().Dx(), srcImg.Bounds().Dy()
	hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("图片缩放: %dx%d → %dx%d, 色彩空间: %s, 抖动: %s", srcW, srcH, targetW, targetH, colorSpaceStr, ditherStr)}))

	// 使用新引擎处理（含抖动、色彩空间匹配，不旋转）
	grid := media.ProcessMapArt(srcImg, targetW, targetH, cs, dither, media.Rotation0)

	// Convert grid to block map
	var sx, sy, sz int
	blocks := make(map[[3]int]*mapArtBlock)

	sleeptime := building.UnlimitedSpeedDelay(speed)
	count := 0

	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			block := grid[y][x]
			if block.Name == "minecraft:air" {
				continue // 跳过透明像素
			}
			blockName := block.Name
			heightOff := block.HeightOff
			if !useGravityBlocks {
				if repl, ok := gravityReplacements[blockName]; ok {
					blockName = repl
				}
			}
			// 三级明暗偏移：亮色 +1，中色 0，暗色 -1
			// 让地图画在地图上产生立体起伏效果
			if orientation == "vertical" {
				blocks[[3]int{baseX, baseY + y, baseZ + x}] = &mapArtBlock{info: &building.BlockInfo{Name: blockName, States: "[]"}, heightOff: heightOff}
				if y+baseY+1 > sy {
					sy = y + baseY + 1
				}
				if x+baseZ+1 > sz {
					sz = x + baseZ + 1
				}
			} else {
				blocks[[3]int{baseX + x, baseY, baseZ + y}] = &mapArtBlock{info: &building.BlockInfo{Name: blockName, States: "[]"}, heightOff: heightOff}
				if x+baseX+1 > sx {
					sx = x + baseX + 1
				}
				if y+baseZ+1 > sz {
					sz = y + baseZ + 1
				}
			}
		}
	}

	hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("像素画: %dx%d → %d 方块, 开始放置...", targetW, targetH, len(blocks))}))

	// 任务系统：创建 checkpoint（参照导入任务），支持断点续传
	taskID := uuid.New().String()
	serverCode := ""
	if bm != nil {
		serverCode = bm.ServerCode()
	}
	cp := &state.Checkpoint{
		TaskID:     taskID,
		Type:       "mapart",
		Status:     "running",
		Progress:   0,
		CreatedAt:  time.Now().Unix(),
		Params:     rawParams(p),
		Name:       filepath.Base(path),
		ServerCode: serverCode,
	}
	state.SaveCheckpointNow(cp)
	hub.Emit("task_start", mustJSON(map[string]any{"task_id": taskID, "type": "mapart"}))

	// 断点数据：已完成区域数 + 已放方块数
	var cpData MapArtCheckpointData
	resuming := false
	if resumeFromCheckpoint != "" {
		if c, err := state.LoadCheckpoint(resumeFromCheckpoint); err == nil {
			json.Unmarshal(c.Data, &cpData)
			resuming = cpData.RegionsDone > 0
			hub.Emit("progress", mustJSON(map[string]any{
				"msg": fmt.Sprintf("从断点恢复: 已完成 %d 个区域 (%d 方块)", cpData.RegionsDone, cpData.BlocksPlaced),
			}))
			// 恢复的旧断点由本次新任务接管，删除以免任务列表残留陈旧记录
			state.DeleteCheckpoint(resumeFromCheckpoint)
		}
	}
	if resuming {
		count = cpData.BlocksPlaced
	}
	startSeq := cpData.RegionsDone

	// 32×32区域优化：在每个32×32区域内做2D矩形填充优化
	regionSize := 32
	regionsW := (targetW + regionSize - 1) / regionSize
	regionsH := (targetH + regionSize - 1) / regionSize

	for rx := startSeq / regionsH; rx < regionsW; rx++ {
		var ryStart, ryEnd, ryStep int
		if rx%2 == 0 {
			ryStart, ryEnd, ryStep = 0, regionsH, 1
		} else {
			ryStart, ryEnd, ryStep = regionsH-1, -1, -1
		}
		// 断点续传：从中间某行恢复时，跳过该行已完成的前置区域
		if resuming && rx == startSeq/regionsH {
			offset := startSeq % regionsH
			if rx%2 == 0 {
				ryStart = offset
			} else {
				ryStart = regionsH - 1 - offset
			}
		}

		for ry := ryStart; ry != ryEnd; ry += ryStep {
			select {
			case <-tc.StopCh():
				hub.Emit("task_error", mustJSON(map[string]any{"error": "已取消"}))
				cp.Status = "cancelled"
				state.SaveCheckpointNow(cp)
				return
			default:
			}

			// 严格失败条件：连接断开 / 主机器人丢失（单机下即唯一机器人断线）→ 标记失败
			if bm != nil && !bm.IsConnected() {
				cp.Status = "failed"
				cp.Message = "连接断开/主机器人丢失，任务失败"
				state.SaveCheckpointNow(cp)
				hub.Emit("task_error", mustJSON(map[string]any{"error": "连接断开/主机器人丢失，任务失败"}))
				return
			}

			if gravityPlatform && orientation == "horizontal" && useGravityBlocks {
				rx0 := baseX + rx*regionSize
				rz0 := baseZ + ry*regionSize
				rx1 := rx0 + min(regionSize, targetW-rx*regionSize) - 1
				rz1 := rz0 + min(regionSize, targetH-ry*regionSize) - 1
				bm.SendBlockCmd(fmt.Sprintf("fill %d %d %d %d %d %d glass",
					rx0, baseY-1, rz0, rx1, baseY-1, rz1))
				time.Sleep(100 * time.Millisecond)
			}

			rStartW := rx * regionSize
			rStartH := ry * regionSize
			rSizeW := min(regionSize, targetW-rStartW)
			rSizeH := min(regionSize, targetH-rStartH)
			var midX, midY, midZ int
			if orientation == "vertical" {
				midX = baseX
				midY = baseY + rStartH + rSizeH/2
				midZ = baseZ + rStartW + rSizeW/2
			} else {
				midX = baseX + rStartW + rSizeW/2
				midY = baseY
				midZ = baseZ + rStartH + rSizeH/2
			}
			bm.TP(midX, midY, midZ)
			time.Sleep(30 * time.Millisecond)

			regVisited := make([][]bool, rSizeH)
			for i := 0; i < rSizeH; i++ {
				regVisited[i] = make([]bool, rSizeW)
			}

			for ly := 0; ly < rSizeH; ly++ {
				for lx := 0; lx < rSizeW; lx++ {
					if regVisited[ly][lx] {
						continue
					}
					imgX := rStartW + lx
					imgY := rStartH + ly
					var key [3]int
					if orientation == "vertical" {
						key = [3]int{baseX, baseY + imgY, baseZ + imgX}
					} else {
						key = [3]int{baseX + imgX, baseY, baseZ + imgY}
					}
					mb, ok := blocks[key]
					if !ok {
						regVisited[ly][lx] = true
						continue
					}
					block := mb.info
					curHeightOff := mb.heightOff
					if orientation == "vertical" || (orientation == "horizontal" && !useRelief) {
						curHeightOff = 0
					}

					rw := 0
					for dx := 0; lx+dx < rSizeW; dx++ {
						var nk [3]int
						if orientation == "vertical" {
							nk = [3]int{baseX, baseY + imgY, baseZ + rStartW + lx + dx}
						} else {
							nk = [3]int{baseX + rStartW + lx + dx, baseY, baseZ + imgY}
						}
						nb, nok := blocks[nk]
						if !nok || nb.info.Name != block.Name || nb.info.States != block.States || nb.heightOff != curHeightOff || regVisited[ly][lx+dx] {
							break
						}
						rw++
					}
					if rw == 0 {
						rw = 1
					}

					bestW, bestH := rw, 1
					for dy := 1; ly+dy < rSizeH; dy++ {
						cw := 0
						for dx := 0; dx < bestW; dx++ {
							var nk [3]int
							if orientation == "vertical" {
								nk = [3]int{baseX, baseY + rStartH + ly + dy, baseZ + rStartW + lx + dx}
							} else {
								nk = [3]int{baseX + rStartW + lx + dx, baseY, baseZ + rStartH + ly + dy}
							}
							nb, nok := blocks[nk]
							if !nok || nb.info.Name != block.Name || nb.info.States != block.States || nb.heightOff != curHeightOff || regVisited[ly+dy][lx+dx] {
								break
							}
							cw++
						}
						if cw == 0 {
							break
						}
						if cw < bestW {
							bestW = cw
						}
						bestH = dy + 1
					}

					for dy := 0; dy < bestH; dy++ {
						for dx := 0; dx < bestW; dx++ {
							regVisited[ly+dy][lx+dx] = true
						}
					}

					total := bestW * bestH
					if total >= 3 {
						var x1, y1, z1, x2, y2, z2 int
						if orientation == "vertical" {
							x1, y1, z1 = baseX+curHeightOff, baseY+rStartH+ly, baseZ+rStartW+lx
							x2, y2, z2 = baseX+curHeightOff, baseY+rStartH+ly+bestH-1, baseZ+rStartW+lx+bestW-1
						} else {
							x1, y1, z1 = baseX+rStartW+lx, baseY+curHeightOff, baseZ+rStartH+ly
							x2, y2, z2 = baseX+rStartW+lx+bestW-1, baseY+curHeightOff, baseZ+rStartH+ly+bestH-1
						}
						bm.SendBlockCmd(fmt.Sprintf("fill %d %d %d %d %d %d %s %s",
							x1, y1, z1, x2, y2, z2, normalizeBlockName(block.Name), block.States))
					} else {
						for dy := 0; dy < bestH; dy++ {
							for dx := 0; dx < bestW; dx++ {
								var baseKey [3]int
								var setX, setY, setZ int
								if orientation == "vertical" {
									baseKey = [3]int{baseX, baseY + rStartH + ly + dy, baseZ + rStartW + lx + dx}
									setX, setY, setZ = baseX+curHeightOff, baseY+rStartH+ly+dy, baseZ+rStartW+lx+dx
								} else {
									baseKey = [3]int{baseX + rStartW + lx + dx, baseY, baseZ + rStartH + ly + dy}
									setX, setY, setZ = baseX+rStartW+lx+dx, baseY+curHeightOff, baseZ+rStartH+ly+dy
								}
								b := blocks[baseKey]
								if b == nil {
									continue
								}
								bm.SendBlockCmd(fmt.Sprintf("setblock %d %d %d %s %s", setX, setY, setZ, normalizeBlockName(b.info.Name), b.info.States))
							}
						}
					}
					count += total
					time.Sleep(sleeptime * time.Duration(total))
				}
			}

			pct := float64(count) / float64(len(blocks))
			sendActionbar(yyTaskActionbar("地图画写入", filepath.Base(path), "3", count, len(blocks), 0, 0))
			hub.Emit("task_progress", mustJSON(map[string]any{"progress": pct, "completed": count, "total": len(blocks), "unit": "方块已提交", "phase": "地图画写入"}))
			hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("游园 · 地图画写入: %d/%d 方块已提交", count, len(blocks))}))

			// 区域整块完成 → 更新断点并落盘
			cpData.RegionsDone++
			cpData.BlocksPlaced = count
			cp.Data, _ = json.Marshal(cpData)
			cp.Progress = pct
			cp.Message = fmt.Sprintf("地图画 %d/%d 方块已提交", count, len(blocks))
			state.SaveCheckpoint(cp)
		}
	}

	cp.Status = "done"
	cp.Progress = 1.0
	cp.Data = nil
	state.SaveCheckpointNow(cp)

	// 记录心跳增量
	recordHeartbeatIncrement(IncrementData{
		MapArtCompleted: 1,
	})
	hub.Emit("task_done", mustJSON(map[string]any{"msg": fmt.Sprintf("像素画完成: %d 方块", count)}))
}

func runSkinTask(tc *state.TaskController, p map[string]any) {
	playerName := getString(p, "player", "")
	path := getString(p, "path", "")

	bm := getBotManager()
	if bm == nil || !bm.IsConnected() {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "未连接到服务器"}))
		return
	}

	var img image.Image
	var displayName string

	if playerName != "" {
		// 在线玩家皮肤：从缓存加载
		skinData, w, h, ok := playerInfoMgr.GetPlayerSkin(playerName)
		if !ok {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "玩家不在线或没有皮肤缓存: " + playerName}))
			return
		}
		img = &image.NRGBA{
			Pix:    skinData,
			Stride: 4 * int(w),
			Rect:   image.Rect(0, 0, int(w), int(h)),
		}
		displayName = playerName + " (在线)"
		hub.Emit("progress", mustJSON(map[string]any{"msg": "加载在线玩家皮肤: " + playerName}))
	} else if path == "" {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "缺少皮肤图片路径或玩家名"}))
		return
	} else {
		// 本地文件皮肤
		displayName = filepath.Base(path)
		hub.Emit("progress", mustJSON(map[string]any{"msg": "加载皮肤图片: " + displayName}))
		f, err := os.Open(path)
		if err != nil {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "无法打开图片: " + err.Error()}))
			return
		}
		defer f.Close()

		img, _, err = image.Decode(f)
		if err != nil {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "图片解码失败: " + err.Error()}))
			return
		}
	}

	opts := media.SkinBuildOptions{
		Scale:          getInt(p, "scale", 2),
		ArmType:        media.SkinArmType(getString(p, "arm_type", "classic")),
		BlockSet:       media.SkinBlockSet(getString(p, "block_set", "mixed")),
		AlphaCutoff:    uint8(getInt(p, "alpha_cutoff", 16)),
		OuterThickness: getInt(p, "outer_thickness", 1),
		Solid:          getBool(p, "solid", false),
		FillBlock:      getString(p, "fill_block", "minecraft:stone"),
		Rotation:       getInt(p, "rotation", 0),
	}

	blocks, info, err := media.BuildSkinStatue(img, opts)
	if err != nil {
		hub.Emit("task_error", mustJSON(map[string]any{"error": "雕像构建失败: " + err.Error()}))
		return
	}

	baseX := int32(getInt(p, "x", 0))
	baseY := int32(getInt(p, "y", -64))
	baseZ := int32(getInt(p, "z", 0))
	speed := getInt(p, "speed", -1)
	sleeptime := building.UnlimitedSpeedDelay(speed)

	hub.Emit("task_start", mustJSON(map[string]any{"type": "皮肤雕像: " + displayName}))
	hub.Emit("task_progress", mustJSON(map[string]any{"progress": 0}))
	if bm.IsOP() {
		bm.yyTaskNotice(fmt.Sprintf("§a§l▍皮肤雕像:§r§f%s §7尺寸:%dx%dx%d §7方块:%d §7坐标:(%d,%d,%d)",
			displayName, info.Width, info.Height, info.Length, info.BlockCount, baseX, baseY, baseZ))
	}

	hub.Emit("progress", mustJSON(map[string]any{
		"msg": fmt.Sprintf("雕像: %dx%dx%d, %d 方块", info.Width, info.Height, info.Length, info.BlockCount),
	}))

	taskID := uuid.New().String()
	serverCode := bm.ServerCode()
	cp := &state.Checkpoint{
		TaskID:     taskID,
		Type:       "skin",
		Status:     "running",
		Progress:   0,
		CreatedAt:  time.Now().Unix(),
		Params:     rawParams(p),
		Name:       displayName,
		ServerCode: serverCode,
	}
	state.SaveCheckpointNow(cp)

	// 按 Y 从下到上排序，确保雕像从地面开始建造
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i].Y != blocks[j].Y {
			return blocks[i].Y < blocks[j].Y
		}
		if blocks[i].X != blocks[j].X {
			return blocks[i].X < blocks[j].X
		}
		return blocks[i].Z < blocks[j].Z
	})

	// 按 Y 从下到上放置方块
	total := len(blocks)
	lastProgressTime := time.Now()
	importStartTime := time.Now()

	// 皮肤雕像 actionbar 定时刷新
	var (
		skinMu     sync.Mutex
		skinBlocks int
	)
	skinStopTicker := make(chan struct{})
	skinTicker := time.NewTicker(500 * time.Millisecond)
	defer skinTicker.Stop()
	go func() {
		var speedLastBlocks int
		var speedLastTime = time.Now()
		var lastRealSpeed int
		for {
			select {
			case <-skinTicker.C:
				skinMu.Lock()
				blocks := skinBlocks
				skinMu.Unlock()

				elapsed := time.Since(importStartTime).Seconds()
				// 每秒更新一次速度，非更新时刻复用上次速度
				now := time.Now()
				realSpeed := lastRealSpeed
				if now.Sub(speedLastTime) >= time.Second {
					dt := now.Sub(speedLastTime).Seconds()
					if dt >= 0.5 {
						realSpeed = int(float64(blocks-speedLastBlocks) / dt)
						speedLastBlocks = blocks
						speedLastTime = now
						lastRealSpeed = realSpeed
					}
				}
				// 第一秒内用平均速度估算
				if lastRealSpeed == 0 && elapsed > 0.1 {
					realSpeed = int(float64(blocks) / elapsed)
				}

				sendActionbar(yyTaskActionbar("雕像写入", shortName(path), "9", blocks, total, realSpeed, time.Since(importStartTime)))
			case <-skinStopTicker:
				return
			}
		}
	}()
	defer close(skinStopTicker)

	for i, b := range blocks {
		select {
		case <-tc.StopCh():
			cp.Status = "cancelled"
			cp.Message = "已取消"
			state.SaveCheckpoint(cp)
			hub.Emit("task_error", mustJSON(map[string]any{"error": "已取消"}))
			return
		default:
		}

		x := baseX + b.X
		y := baseY + b.Y
		z := baseZ + b.Z
		bm.SendBlockCmd(fmt.Sprintf("setblock %d %d %d %s %s", x, y, z, normalizeBlockName(b.BlockName), b.BlockStates))
		time.Sleep(sleeptime)

		if i%100 == 0 || time.Since(lastProgressTime) >= 500*time.Millisecond {
			pct := float64(i+1) / float64(total)
			lastProgressTime = time.Now()

			// 更新 actionbar 共享状态
			skinMu.Lock()
			skinBlocks = i + 1
			skinMu.Unlock()

			hub.Emit("task_progress", mustJSON(map[string]any{
				"progress":     pct,
				"sub_progress": 1.0,
				"blocks":       i + 1, "total_blocks": total, "phase": "雕像写入", "unit": "方块已提交",
			}))
			hub.Emit("progress", mustJSON(map[string]any{
				"msg": fmt.Sprintf("雕像: %d/%d", i+1, total),
			}))

			cp.Progress = pct
			cp.Message = fmt.Sprintf("%d/%d", i+1, total)
			state.SaveCheckpoint(cp)
		}
	}
	close(skinStopTicker)

	sendActionbar(yyTaskActionbar("雕像提交完成", shortName(path), "9", len(blocks), len(blocks), 0, time.Since(importStartTime)))

	bm.cleanupNBTWorkspace()

	cp.Status = "done"
	cp.Progress = 1.0
	state.SaveCheckpoint(cp)

	if bm.IsOP() {
		bm.yyTaskNotice(fmt.Sprintf("§a§l▍雕像完成:§r§f%s §7共%d方块", displayName, len(blocks)))
	}
	// 记录心跳增量
	recordHeartbeatIncrement(IncrementData{
		SkinCompleted: 1,
	})
	hub.Emit("task_done", mustJSON(map[string]any{"msg": fmt.Sprintf("雕像导入完成: %d 方块", len(blocks)), "task_id": taskID}))
}

func runMediaTask(tc *state.TaskController, p map[string]any) {
	mediaType := getString(p, "media_type", "video")
	target := getString(p, "display", "actionbar")
	fps := getInt(p, "fps", 10)
	path := getString(p, "path", "")
	bm := getBotManager()

	switch mediaType {
	case "video":
		hub.Emit("progress", mustJSON(map[string]any{"msg": "视频播放: " + path}))
		if bm != nil && bm.IsOP() {
			bm.yyTaskNotice(fmt.Sprintf("§a§l▍音视频:§r§f%s §7类型:视频 §7显示:%s", filepath.Base(path), target))
		}
		hub.Emit("progress", mustJSON(map[string]any{
			"msg": "视频需要先用ffmpeg提取帧为图片文件，放入数据目录的frames/子目录",
		}))

		// Try to load frames from data directory
		framesDir := path
		if framesDir == "" {
			framesDir = "./frames/"
		}
		entries, err := os.ReadDir(framesDir)
		if err != nil {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "无法读取帧目录: " + err.Error()}))
			return
		}

		var frames [][]string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			f, err := os.Open(framesDir + "/" + entry.Name())
			if err != nil {
				continue
			}
			img, _, err := image.Decode(f)
			f.Close()
			if err != nil {
				continue
			}
			lines, _ := media.ImageToASCII(img, 80)
			frames = append(frames, lines)
		}

		if len(frames) == 0 {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "未找到有效帧文件"}))
			return
		}

		hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("找到 %d 帧，开始播放...", len(frames))}))

		player := media.NewPlayer(func(cmd string) error {
			return bm.SendWOCmd(cmd)
		})
		player.Config.Target = "@a"
		player.Config.Mode = target
		player.Config.FPS = fps
		player.PlayASCIIFrames(frames, tc.StopCh())

		hub.Emit("task_done", mustJSON(map[string]any{"msg": "播放完成"}))

	case "midi":
		hub.Emit("progress", mustJSON(map[string]any{"msg": "MIDI: " + path}))
		if bm != nil && bm.IsOP() {
			bm.yyTaskNotice(fmt.Sprintf("§a§l▍音视频:§r§f%s §7类型:MIDI", filepath.Base(path)))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			hub.Emit("task_error", mustJSON(map[string]any{"error": "无法读取: " + err.Error()}))
			return
		}
		seq, err := media.ParseMidiSeq(data)
		if err != nil {
			hub.Emit("progress", mustJSON(map[string]any{"msg": "非midseq格式，逐行播放..."}))
			lines := splitLines(string(data))
			st := time.Second / time.Duration(fps)
			for i, line := range lines {
				select {
				case <-tc.StopCh():
					return
				default:
				}
				if len(line) > 0 && line[0] != '#' {
					bm.SendWOCmd(line)
					time.Sleep(st)
				}
				if i%10 == 0 {
					hub.Emit("task_progress", mustJSON(map[string]any{"progress": float64(i) / float64(len(lines))}))
				}
			}
			hub.Emit("task_done", mustJSON(map[string]any{"msg": "播放完成"}))
			return
		}
		hub.Emit("progress", mustJSON(map[string]any{"msg": fmt.Sprintf("乐器:%d 时长:%.0fs 音符:%d", len(seq.Instruments), seq.Duration, len(seq.Notes))}))
		for i, n := range seq.Notes {
			select {
			case <-tc.StopCh():
				return
			default:
			}
			time.Sleep(time.Duration(n.Delay * float64(time.Second)))
			bm.SendWOCmd(fmt.Sprintf("execute as @a at @s run playsound %s @s ~~~ %.2f %.3f", n.Instrument, n.Volume, n.Pitch))
			if i%20 == 0 {
				hub.Emit("task_progress", mustJSON(map[string]any{"progress": float64(i) / float64(len(seq.Notes))}))
			}
		}
		hub.Emit("task_done", mustJSON(map[string]any{"msg": fmt.Sprintf("MIDI完成(%.0fs)", seq.Duration)}))
	}
}

func splitLines(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

var bedrockEnchNames = map[int]string{
	0: "protection", 1: "fire_protection", 2: "feather_falling",
	3: "blast_protection", 4: "projectile_protection", 5: "thorns",
	6: "respiration", 7: "depth_strider", 8: "aqua_affinity",
	9: "sharpness", 10: "smite", 11: "bane_of_arthropods",
	12: "knockback", 13: "fire_aspect", 14: "looting",
	15: "efficiency", 16: "silk_touch", 17: "unbreaking",
	18: "fortune", 19: "power", 20: "punch",
	21: "flame", 22: "infinity", 23: "luck_of_the_sea",
	24: "lure", 25: "frost_walker", 26: "mending",
	27: "curse_of_binding", 28: "curse_of_vanishing", 29: "impaling",
	30: "riptide", 31: "loyalty", 32: "channeling",
	33: "multishot", 34: "piercing", 35: "quick_charge",
	36: "soul_speed", 37: "swift_sneak",
}

// buildSimpleItemComponents 从物品 tag NBT 构建简单 components JSON。
// 仅包含服务端支持的：KeepOnDeath、ItemLock、CanDestroy、CanPlaceOn。
// 不包含 display_name 和 enchantments——它们会导致整个命令失败。
func buildSimpleItemComponents(tag map[string]any) string {
	parts := make([]string, 0)

	// KeepOnDeath — 死亡后不掉落
	if _, ok := tag["KeepOnDeath"]; ok {
		parts = append(parts, `"minecraft:keep_on_death":{}`)
	}

	// ItemLock — 锁定物品（lock_in_container / lock_in_slot）
	if lockVal, ok := tag["item_lock"]; ok {
		switch v := lockVal.(type) {
		case map[string]any:
			if mode, ok := v["mode"].(string); ok {
				parts = append(parts, fmt.Sprintf(`"minecraft:item_lock":{"mode":%q}`, mode))
			}
		case string:
			parts = append(parts, fmt.Sprintf(`"minecraft:item_lock":{"mode":%q}`, v))
		}
	}

	// CanDestroy — 冒险模式可破坏方块
	if cd, ok := tag["can_destroy"].(map[string]any); ok {
		if blocks, ok := cd["blocks"].([]any); ok && len(blocks) > 0 {
			blockStrs := make([]string, len(blocks))
			for i, b := range blocks {
				blockStrs[i] = fmt.Sprintf("%q", b)
			}
			parts = append(parts, fmt.Sprintf(`"minecraft:can_destroy":{"blocks":[%s]}`, strings.Join(blockStrs, ",")))
		}
	}

	// CanPlaceOn — 冒险模式可放置方块
	if cpo, ok := tag["can_place_on"].(map[string]any); ok {
		if blocks, ok := cpo["blocks"].([]any); ok && len(blocks) > 0 {
			blockStrs := make([]string, len(blocks))
			for i, b := range blocks {
				blockStrs[i] = fmt.Sprintf("%q", b)
			}
			parts = append(parts, fmt.Sprintf(`"minecraft:can_place_on":{"blocks":[%s]}`, strings.Join(blockStrs, ",")))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// itemDamage 从物品 NBT 中提取 Damage/meta 值。
func itemDamage(item map[string]any) int32 {
	switch d := item["Damage"].(type) {
	case int32:
		return d
	case int16:
		return int32(d)
	case int:
		return int32(d)
	case float64:
		return int32(d)
	case byte:
		return int32(d)
	}
	return 0
}

func getBool(p map[string]any, key string, def bool) bool {
	if v, ok := p[key].(bool); ok {
		return v
	}
	return def
}

// parseDimension 解析维度参数，兼容字符串（"overworld"/"nether"/"end"）和数字（0/1/2）。
// 导出时前端传的是 select 控件的字符串值，需要映射为 MCBE 维度 ID。
func parseDimension(p map[string]any, key string, def int) int {
	// 尝试字符串
	if s, ok := p[key].(string); ok && s != "" {
		return dimNameToID(s)
	}
	// 回退到数字
	return getInt(p, key, def)
}

func getString(p map[string]any, key, def string) string {
	if v, ok := p[key].(string); ok {
		return v
	}
	return def
}

func getInt(p map[string]any, key string, def int) int {
	switch v := p[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var i int
		fmt.Sscanf(v, "%d", &i)
		return i
	}
	return def
}

// extractBlockEntities reads block_position_data from a RequestStructure response
// and attaches it as a flat nbt_blocks list to the chunk map.
func extractBlockEntities(chunk map[string]any) {
	structure, _ := chunk["structure"].(map[string]any)
	if structure == nil {
		return
	}
	paletteData, _ := structure["palette"].(map[string]any)
	if paletteData == nil {
		return
	}

	var blockPosData map[string]any
	for _, pv := range paletteData {
		pm, ok := pv.(map[string]any)
		if !ok {
			continue
		}
		if bpd, ok := pm["block_position_data"].(map[string]any); ok && len(bpd) > len(blockPosData) {
			blockPosData = bpd
		}
	}
	if blockPosData == nil {
		return
	}

	// Build palette name lookup
	var paletteNames []string
	for _, pv := range paletteData {
		pm, ok := pv.(map[string]any)
		if !ok {
			continue
		}
		if bp, ok := pm["block_palette"].([]any); ok {
			for _, entry := range bp {
				em, _ := entry.(map[string]any)
				name, _ := em["name"].(string)
				paletteNames = append(paletteNames, name)
			}
			break
		}
	}

	var nbtBlocks []map[string]any
	for keyStr, value := range blockPosData {
		valMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		entityData, ok := valMap["block_entity_data"].(map[string]any)
		if !ok {
			continue
		}

		var keyInt int
		fmt.Sscanf(keyStr, "%d", &keyInt)
		blockName := "minecraft:air"
		if keyInt >= 0 && keyInt < len(paletteNames) {
			blockName = paletteNames[keyInt]
		}
		nbtBlocks = append(nbtBlocks, map[string]any{
			"palette_index": keyInt,
			"name":          blockName,
			"nbt":           entityData,
		})
	}
	if len(nbtBlocks) > 0 {
		chunk["nbt_blocks"] = nbtBlocks
	}
}

// mergeChunkStructures combines block_indices
// mergeChunkStructures combines block_indices from multiple 16xN chunks into one.
// Each chunk's block palette must be compatible — we remap indices and merge palettes.
// blockNameForIndex resolves a palette index to a block name.
func blockNameForIndex(idx int, paletteData map[string]any) string {
	for _, pv := range paletteData {
		pm, ok := pv.(map[string]any)
		if !ok {
			continue
		}
		bp, ok := pm["block_palette"].([]any)
		if !ok {
			continue
		}
		if idx >= 0 && idx < len(bp) {
			if bm, ok := bp[idx].(map[string]any); ok {
				if n, ok := bm["name"].(string); ok {
					return n
				}
			}
		}
		break
	}
	return "minecraft:air"
}

func mergeChunkStructures(chunks []map[string]any, w, h, l int32) map[string]any {
	if len(chunks) == 0 || w <= 0 || h <= 0 || l <= 0 {
		return nil
	}
	volume := int64(w) * int64(h) * int64(l)
	if volume > 16*1024*1024 {
		return nil
	}
	type part struct {
		s         map[string]any
		off, size [3]int32
		palette   []any
		remap     map[int]int32
	}
	parts := []part{}
	covered := int64(0)
	layers := 0
	for i, c := range chunks {
		st, ok := c["structure"].(map[string]any)
		if !ok {
			return nil
		}
		off := [3]int32{int32(i) / ((l + 15) / 16) * 16, 0, int32(i) % ((l + 15) / 16) * 16}
		sz := [3]int32{min(16, w-off[0]), h, min(16, l-off[2])}
		if v, ok := c["_yy_offset"].([3]int32); ok {
			off = v
		}
		if v, ok := c["_yy_size"].([3]int32); ok {
			sz = v
		} else if v := anyToIntSlice(c["size"]); len(v) == 3 {
			sz = [3]int32{int32(v[0]), int32(v[1]), int32(v[2])}
		}
		for d, total := range [3]int32{w, h, l} {
			if off[d] < 0 || sz[d] <= 0 || int64(off[d])+int64(sz[d]) > int64(total) {
				return nil
			}
		}
		bi, ok := st["block_indices"].([]any)
		if !ok || len(bi) == 0 {
			return nil
		}
		layers = max(layers, len(bi))
		pal := yyStructurePalette(st)
		bp, ok := pal["block_palette"].([]any)
		if !ok {
			return nil
		}
		parts = append(parts, part{st, off, sz, bp, map[int]int32{}})
		covered += int64(sz[0]) * int64(sz[1]) * int64(sz[2])
	}
	// Never report a truncated structure as a successful whole-region export.
	if covered != volume {
		return nil
	}
	palette := []any{}
	paletteKeys := map[string]int32{}
	for i := range parts {
		for old, v := range parts[i].palette {
			keyBytes, e := json.Marshal(v)
			if e != nil {
				return nil
			}
			key := string(keyBytes)
			id, ok := paletteKeys[key]
			if !ok {
				id = int32(len(palette))
				paletteKeys[key] = id
				palette = append(palette, v)
			}
			parts[i].remap[old] = id
		}
	}
	arrays := make([]any, layers)
	for d := 0; d < layers; d++ {
		a := make([]int32, int(volume))
		for i := range a {
			a[i] = -1
		}
		arrays[d] = a
	}
	visited := make([]bool, int(volume))
	bpd := map[string]any{}
	for _, p := range parts {
		bi := p.s["block_indices"].([]any)
		indices := make([][]int, len(bi))
		for layer, v := range bi {
			indices[layer] = toIntSlice(v)
			if len(indices[layer]) != int(p.size[0]*p.size[1]*p.size[2]) {
				return nil
			}
		}
		for x := int32(0); x < p.size[0]; x++ {
			for y := int32(0); y < p.size[1]; y++ {
				for z := int32(0); z < p.size[2]; z++ {
					src := int((x*p.size[1]+y)*p.size[2] + z)
					dst := int(((p.off[0]+x)*h+p.off[1]+y)*l + p.off[2] + z)
					if visited[dst] {
						return nil
					}
					visited[dst] = true
					for layer, a := range indices {
						old := a[src]
						if old < 0 {
							continue
						}
						id, ok := p.remap[old]
						if !ok {
							return nil
						}
						arrays[layer].([]int32)[dst] = id
					}
				}
			}
		}
		pal, _ := p.s["palette"].(map[string]any)
		for _, raw := range pal {
			v, _ := raw.(map[string]any)
			rows, _ := v["block_position_data"].(map[string]any)
			for key, data := range rows {
				var src int
				if _, err := fmt.Sscanf(key, "%d", &src); err != nil || src < 0 || src >= int(p.size[0]*p.size[1]*p.size[2]) {
					return nil
				}
				x := int32(src) / (p.size[1] * p.size[2])
				y := int32(src) / p.size[2] % p.size[1]
				z := int32(src) % p.size[2]
				dst := ((p.off[0]+x)*h+p.off[1]+y)*l + p.off[2] + z
				bpd[fmt.Sprint(dst)] = data
			}
		}
	}
	return map[string]any{"block_indices": arrays, "entities": []any{}, "palette": map[string]any{"default": map[string]any{"block_palette": palette, "block_position_data": bpd}}}
}

func yyStructurePalette(st map[string]any) map[string]any {
	pal, _ := st["palette"].(map[string]any)
	if p, ok := pal["default"].(map[string]any); ok {
		return p
	}
	// Deterministic fallback for a named palette, rather than Go map order.
	keys := []string{}
	for k := range pal {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if p, ok := pal[k].(map[string]any); ok {
			return p
		}
	}
	return nil
}

// nbtInt reads an integer value from an NBT map, supporting all numeric NBT types.
func nbtInt(m map[string]any, key string) int {
	v := m[key]
	switch val := v.(type) {
	case int32:
		return int(val)
	case int16:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	case byte:
		return int(val)
	case float64:
		return int(val)
	}
	return 0
}

// nbtByte reads a boolean-relevant byte from an NBT map (0=false, everything else=true).
func nbtByte(m map[string]any, key string) byte {
	v := m[key]
	switch val := v.(type) {
	case byte:
		return val
	case int32:
		return byte(val)
	case int16:
		return byte(val)
	case float64:
		return byte(val)
	case int:
		return byte(val)
	}
	return 0
}

// nbtStr returns the first non-empty string among the candidate keys.
// Bedrock block-entity NBT uses lowercase keys (e.g. structureName); pass those first.
func nbtStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// nbtF32 reads a float from an NBT map, tolerating float32/float64/int encodings.
func nbtF32(m map[string]any, key string) float32 {
	switch val := m[key].(type) {
	case float32:
		return val
	case float64:
		return float32(val)
	case int32:
		return float32(val)
	case int64:
		return float32(val)
	case int:
		return float32(val)
	}
	return 0
}

// buildStructureBlockUpdate builds a StructureBlockUpdate packet (Position set by caller) from
// raw block-entity NBT. Bedrock (.mcworld/.mcstructure) stores keys lowercase (structureName,
// dataField, xStructureSize, xStructureOffset, ...); capitalized / Java names are read as a
// fallback. It carries the full structure data — name, size, offset, rotation, mirror,
// integrity, seed — not just the name. Values are copied straight from the NBT, so each mode
// (save/load/corner/data) naturally gets the fields it stored; absent fields default to zero.
// The data→type mapping is preserved from the original code.
func buildStructureBlockUpdate(nbt map[string]any) packet.StructureBlockUpdate {
	typ := int32(1)
	if _, ok := nbt["StructureBlockType"]; ok {
		typ = int32(nbtInt(nbt, "StructureBlockType"))
	}
	if _, ok := nbt["data"]; ok {
		switch nbtInt(nbt, "data") {
		case 0:
			typ = 1
		case 1:
			typ = 2
		case 2:
			typ = 3
		case 3:
			typ = 0
		case 4:
			typ = 4
		}
	}

	// Bedrock NBT stores integrity as a percentage (0..100); the packet expects 0..1.
	integrity := nbtF32(nbt, "integrity")
	if integrity > 1 {
		integrity /= 100
	}

	return packet.StructureBlockUpdate{
		StructureName:      nbtStr(nbt, "structureName", "StructureName", "name"),
		DataField:          nbtStr(nbt, "dataField", "DataField", "metadata"),
		IncludePlayers:     nbtByte(nbt, "includePlayers") != 0 || nbtByte(nbt, "IncludePlayers") != 0,
		ShowBoundingBox:    nbtByte(nbt, "showBoundingBox") != 0 || nbtByte(nbt, "ShowBoundingBox") != 0,
		StructureBlockType: typ,
		RedstoneSaveMode:   int32(nbtInt(nbt, "redstoneSaveMode")),
		Settings: protocol.StructureSettings{
			PaletteName:    "default",
			IgnoreEntities: nbtByte(nbt, "ignoreEntities") != 0,
			Size: protocol.BlockPos{
				int32(nbtInt(nbt, "xStructureSize")),
				int32(nbtInt(nbt, "yStructureSize")),
				int32(nbtInt(nbt, "zStructureSize")),
			},
			Offset: protocol.BlockPos{
				int32(nbtInt(nbt, "xStructureOffset")),
				int32(nbtInt(nbt, "yStructureOffset")),
				int32(nbtInt(nbt, "zStructureOffset")),
			},
			Rotation:          nbtByte(nbt, "rotation"),
			Mirror:            nbtByte(nbt, "mirror"),
			AnimationMode:     nbtByte(nbt, "animationMode"),
			AnimationDuration: nbtF32(nbt, "animationSeconds"),
			Integrity:         integrity,
			Seed:              uint32(nbtInt(nbt, "seed")),
		},
	}
}

// buildParBar 渲染平行四边形进度条。
// 使用 ࡆ (U+0866) 实心字符 + §o(斜体) 实现平行四边形效果。
// 在填充/未填充交界处，当两侧都有足够字符时去掉斜体做视觉过渡；
// 但只剩最后一个字符时（进度快满或仅剩一个未填充），保留斜体保证末端统一。
func buildParBar(filled, width int, color, grayColor string) string {
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	var sb strings.Builder
	sb.Grow(width * 8)
	for i := 0; i < width; i++ {
		if i < filled {
			if i == 0 && filled > 1 {
				sb.WriteString("\u00a7o\u00a7l\u00a7" + color)
			} else if i == filled-1 && filled > 1 && filled+1 < width {
				sb.WriteString("\u00a7r\u00a7l\u00a7" + color)
			} else if i == 0 {
				sb.WriteString("\u00a7o\u00a7l\u00a7" + color)
			}
		} else {
			if i == filled {
				if filled == 0 {
					sb.WriteString("\u00a7o\u00a7l\u00a7" + grayColor)
				} else if filled+1 >= width {
					sb.WriteString("\u00a7o\u00a7l\u00a7" + grayColor)
				} else {
					sb.WriteString("\u00a7r\u00a7" + grayColor)
				}
			} else if i == filled+1 && filled > 0 {
				sb.WriteString("\u00a7o\u00a7l\u00a7" + grayColor)
			}
		}
		sb.WriteString("\u0866")
	}
	return sb.String()
}

// Building tasks use Youyuan labels and actual counters, without percentage text.
func buildClearActionbar(cleared, total int) string {
	return yyTaskActionbar("区域准备", "清空选定区域", "6", cleared, total, 0, 0)
}

// yyStructureHudTrack keeps the recent throughput samples that feed the ▂▄▆█
// meter. It is reset whenever a new job starts so a previous run cannot make a
// fresh one look fast.
type yyStructureHudTrack struct {
	sync.Mutex
	start   time.Time
	samples []float64
}

var yyStructureHudMeter yyStructureHudTrack

func (t *yyStructureHudTrack) push(start time.Time, speed float64) []float64 {
	t.Lock()
	defer t.Unlock()
	if !start.Equal(t.start) {
		t.start = start
		t.samples = nil
	}
	t.samples = append(t.samples, speed)
	const keep = 8
	if len(t.samples) > keep {
		t.samples = append([]float64{}, t.samples[len(t.samples)-keep:]...)
	}
	return append([]float64{}, t.samples...)
}

func buildV5Actionbar(sendFn func(string), name string, sx, sy, sz int,
	regionMode, totalSolid, targetSpeed, realSpeed, blocks, totalPct, subPct int,
	regionIdx, totalRegions int, chunkDone, chunkTotal, chunkCX, chunkCZ int,
	startTime time.Time, brandName string) {
	phase, colour := "导入中", "a"
	if regionMode != 0 {
		phase, colour = "导出中", "b"
	}
	if totalSolid > 0 && blocks >= totalSolid {
		phase, colour = "已完成", "a"
	}
	hud := yyStructureHudForImport(phase, colour, name, sx, sy, sz,
		blocks, totalSolid, realSpeed, time.Since(startTime),
		yyStructureHudMeter.push(startTime, float64(realSpeed)))
	if chunkTotal > 0 {
		// The importer loop reports the chunk it is working on, so this is a
		// measured count rather than a ratio.
		hud.ChunksDone, hud.ChunksTotal, hud.ChunksExact = chunkDone, chunkTotal, true
	}
	if chunkTotal > 0 && blocks < totalSolid {
		hud.Cursor = fmt.Sprintf("X%d Z%d", chunkCX*16, chunkCZ*16)
	}
	hud = yyStructureHudWithHealth(hud)
	text := yyStructureHudRender(hud)
	if totalRegions > 0 {
		text += fmt.Sprintf("\n§r§d分区 §f%d §7/ %d  ·  本区 §d%s§8%s", regionIdx, totalRegions, strings.Repeat("┃", max(0, min(10, subPct/10))), strings.Repeat("┃", 10-max(0, min(10, subPct/10))))
	}
	sendFn(text)
}

func formatSpeedV5(speed float64) string {
	if speed <= 0 {
		return "0"
	}
	switch {
	case speed >= 100:
		return fmt.Sprintf("%.0f", speed)
	case speed >= 10:
		return fmt.Sprintf("%.1f", speed)
	default:
		return fmt.Sprintf("%.2f", speed)
	}
}

func formatCountV5(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func formatDurationV5(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return d.String()
}
