package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bot-apk/building"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/utils"
)

// nbt_world_api.go —— 供 Lua 插件（game.*）与 AI（game_call）统一调用的
// 游戏内"方块 NBT / 方块状态"读写层。
//
// 读：基于 BotManager.RequestStructure（结构保存 + StructureTemplateDataRequest），
// 一次结构保存即可同时拿到 方块名称 + 方块状态 + 方块实体 NBT。
// 写：按 building.ClassifyBlock 分类，分发到现有内部放置函数（快档直接发包、
// 重NBT走工作台），与多机器人导入行为完全一致。

// BlockData 一个方块的完整数据：名称、方块状态、方块实体 NBT。
type BlockData struct {
	Name   string         `json:"name"`
	States map[string]any `json:"states,omitempty"`
	NBT    map[string]any `json:"nbt,omitempty"`
}

// ───────────────────────── 读 ─────────────────────────

// GetBlockData 读单个方块完整数据（名称+状态+NBT）。
// 先传送加载区块（仿 GetBlockTileSafe），再 RequestStructure(1×1×1)。
func (bm *BotManager) GetBlockData(x, y, z int) (*BlockData, error) {
	region, err := bm.GetRegionBlockData([3]int32{int32(x), int32(y), int32(z)}, [3]int32{1, 1, 1})
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%d,%d,%d", x, y, z)
	if bd, ok := region[key]; ok {
		return bd, nil
	}
	// 无 NBT 且方块为空气时 block_palette 里可能没有该位置 → 返回空数据而非错误
	return &BlockData{}, nil
}

// GetBlockNBT 只读一个方块的方块实体 NBT（命令方块指令 / 箱子物品 / 告示牌文本 / 旗帜图案）。
func (bm *BotManager) GetBlockNBT(x, y, z int) (map[string]any, error) {
	bd, err := bm.GetBlockData(x, y, z)
	if err != nil {
		return nil, err
	}
	return bd.NBT, nil
}

// GetBlockState 读一个方块的名称 + 方块状态（facing/lit/open 等，不含 NBT）。
func (bm *BotManager) GetBlockState(x, y, z int) (string, map[string]any, error) {
	bd, err := bm.GetBlockData(x, y, z)
	if err != nil {
		return "", nil, err
	}
	return bd.Name, bd.States, nil
}

// GetRegionBlockData 读一块区域所有方块的名称+状态+NBT（一次结构保存）。
// 返回以 "x,y,z" 绝对坐标为键的映射。
func (bm *BotManager) GetRegionBlockData(pos, size [3]int32) (map[string]*BlockData, error) {
	if size[0] <= 0 || size[1] <= 0 || size[2] <= 0 {
		return nil, fmt.Errorf("区域尺寸必须为正: %v", size)
	}
	if bm == nil || bm.conn == nil {
		return nil, fmt.Errorf("未连接")
	}
	// 传送到区域中心上方加载区块，否则 structure save 返回空。
	bm.TP(int(pos[0]+size[0]/2), 150, int(pos[2]+size[2]/2))
	_ = bm.AwaitChangesGeneral()
	time.Sleep(400 * time.Millisecond)

	data, err := bm.RequestStructure(pos, size, 20*time.Second)
	if err != nil {
		return nil, err
	}
	return parseStructureData(data, int(pos[0]), int(pos[1]), int(pos[2])), nil
}

// parseStructureData 把 RequestStructure 返回的结构数据解析成绝对坐标 → BlockData。
// 结构与 extractCommandBlocks 一致：block_indices 是 z-fastest 线性索引，
// block_palette 提供名称+状态，block_position_data 提供 NBT（同为线性索引键）。
func parseStructureData(data map[string]any, originX, originY, originZ int) map[string]*BlockData {
	out := make(map[string]*BlockData)

	sizeRaw := anyToIntSlice(data["size"])
	if len(sizeRaw) < 3 {
		return out
	}
	sx, sy, sz := sizeRaw[0], sizeRaw[1], sizeRaw[2]
	if sx <= 0 || sy <= 0 || sz <= 0 {
		return out
	}

	structure, _ := data["structure"].(map[string]any)
	if structure == nil {
		return out
	}

	var blockPaletteRaw []any
	blockPosData := make(map[string]any)
	if paletteData, ok := structure["palette"].(map[string]any); ok {
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
	}

	blockIndices, _ := structure["block_indices"].([]any)
	if len(blockIndices) < 1 {
		return out
	}
	layer0 := anyToIntSlice(blockIndices[0])
	if layer0 == nil {
		return out
	}

	// block_position_data 用 z-fastest 线性索引作键 → 线性索引 i → NBT
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

	for i, idx := range layer0 {
		if idx < 0 || int(idx) >= len(blockPaletteRaw) {
			continue
		}
		entry, _ := blockPaletteRaw[idx].(map[string]any)
		if entry == nil {
			continue
		}
		name, _ := entry["name"].(string)
		states, _ := entry["states"].(map[string]any)

		// z-fastest 相对坐标
		rz := i % sz
		ry := (i / sz) % sy
		rx := i / (sy * sz)

		bd := &BlockData{Name: name}
		if len(states) > 0 {
			bd.States = states
		}
		if nbt, ok := blockEntityMap[i]; ok && len(nbt) > 0 {
			bd.NBT = nbt
		}
		out[fmt.Sprintf("%d,%d,%d", originX+rx, originY+ry, originZ+rz)] = bd
	}
	return out
}

// ───────────────────────── 写 ─────────────────────────

// SetBlock 放置一个普通方块（无 NBT，可带完整方块状态字符串）。
// 走 SendBlockCmd（PyRpc 网易 AI 命令通道，权限更高）。
func (bm *BotManager) SetBlock(x, y, z int, blockName, blockStates string) error {
	nm := normalizeBlockName(blockName)
	if nm == "" {
		return fmt.Errorf("blockName 不能为空")
	}
	cmd := fmt.Sprintf("setblock %d %d %d %s", x, y, z, nm)
	if blockStates != "" && blockStates != "[]" {
		cmd = fmt.Sprintf("setblock %d %d %d %s %s", x, y, z, nm, blockStates)
	}
	if err := bm.SendBlockCmd(cmd); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

// PlaceNBTBlock 放置一个带 NBT 的特殊方块（命令方块/箱子/告示牌/旗帜等）。
// 按 building.ClassifyBlock 分类分发，与多机器人导入完全一致：
//   命令/结构方块 → 直接发包（快）
//   告示牌        → PlaceSign（快）
//   重NBT（箱子/旗帜等）→ 工作台+结构复制（慢，需要结构权限）
func (bm *BotManager) PlaceNBTBlock(x, y, z int, blockName, blockStates string, nbt map[string]any) error {
	name := normalizeBlockName(blockName)
	if name == "" {
		return fmt.Errorf("blockName 不能为空")
	}
	switch building.ClassifyBlock(name, nbt) {
	case building.CatSign:
		return bm.PlaceSign(int32(x), int32(y), int32(z), name, blockStates, nbt)
	case building.CatLightNBT:
		// 命令方块 / 结构方块：直接发包，无需工作台
		return placeOneLightNBT(bm, int32(x), int32(y), int32(z), name, blockStates, nbt, false)
	case building.CatWorkspaceNBT:
		// 重NBT方块：工作台 + 结构复制
		wx, wy, wz := building.WorkspaceCenter(bm.BotIndex, x, z, 1, 1)
		center := protocol.BlockPos{int32(wx), int32(wy), int32(wz)}
		var seq int
		return placeOneNBTBlock(bm, int32(x), int32(y), int32(z), name, blockStates, nbt, center, &seq)
	default:
		// CatNormal：纯普通方块，setblock 即可
		return bm.SetBlock(x, y, z, name, blockStates)
	}
}

// SetBlockState 修改一个方块的方块状态：先读当前名称与 NBT，把新状态合并后
// 重新放置（保留原有 NBT）。对命令方块/告示牌/箱子同样适用。
func (bm *BotManager) SetBlockState(x, y, z int, states map[string]any) error {
	data, err := bm.GetBlockData(x, y, z)
	if err != nil {
		return err
	}
	if data.Name == "" {
		return fmt.Errorf("(%d,%d,%d) 未读取到方块", x, y, z)
	}
	merged := make(map[string]any, len(data.States)+len(states))
	for k, v := range data.States {
		merged[k] = v
	}
	for k, v := range states {
		merged[k] = v
	}
	return bm.PlaceNBTBlock(x, y, z, data.Name, buildStatesString(merged), data.NBT)
}

// buildStatesString 把方块状态 map 格式化为 setblock 用的 states 字符串。
// 值先归一化成 utils.MarshalBlockStates 认识的类型（string/byte/int32）。
func buildStatesString(states map[string]any) string {
	if len(states) == 0 {
		return "[]"
	}
	norm := make(map[string]any, len(states))
	for k, v := range states {
		norm[k] = normalizeStateValue(v)
	}
	return utils.MarshalBlockStates(norm)
}

func normalizeStateValue(v any) any {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return byte(1)
		}
		return byte(0)
	case byte, int32:
		return val
	case int:
		return int32(val)
	case int64:
		return int32(val)
	case float64:
		return int32(val)
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return int32(i)
		}
		return val.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}
