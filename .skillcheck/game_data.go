package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
)

// ========== 类型定义 ==========

// PlayerPos 玩家位置信息
type PlayerPos struct {
	Dimension int                              `json:"dimension"`
	Position  game_interface.TargetQueryingPos `json:"position"`
	YRot      float32                          `json:"yRot"`
	Pitch     float32                          `json:"pitch"`
}

// PlayerInventory 玩家背包查询结果
type PlayerInventory struct {
	Items []PlayerInventoryItem `json:"items"`
}

// PlayerInventoryItem 背包中的单个物品
type PlayerInventoryItem struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Slot   int    `json:"slot"`
	Damage int    `json:"damage"`
}

// TickingAreaInfo 常加载区域信息
type TickingAreaInfo struct {
	Dimension string  `json:"dimension"`
	StartX    float64 `json:"start_x"`
	StartZ    float64 `json:"start_z"`
	EndX      float64 `json:"end_x"`
	EndZ      float64 `json:"end_z"`
}

// ========== 坐标 API ==========

// GetPlayerPos 获取目标玩家的详细位置信息。
// target 可以是玩家名或目标选择器（@a, @p, @r, @s）。
// 等效于 td_prism 的 getPos()。
func (bm *BotManager) GetPlayerPos(target string) (*PlayerPos, error) {
	if !strings.HasPrefix(target, "@") {
		target = `"` + target + `"`
	}
	infos, err := bm.DoQuerytarget(target)
	if err != nil {
		return nil, fmt.Errorf("GetPlayerPos: %v", err)
	}
	if len(infos) == 0 {
		return nil, fmt.Errorf("GetPlayerPos: 目标 %s 不存在", target)
	}
	info := infos[0]
	// 调整坐标：querytarget 返回的坐标是玩家脚部位置
	// 参考 td_prism：正数保留，负数减1，Y坐标减1.62（眼睛高度）
	x := info.Position.X
	if x >= 0 {
		x = float32(math.Floor(float64(x*100)) / 100)
	} else {
		x = float32(math.Floor(float64((x-1)*100)) / 100)
	}
	y := info.Position.Y - 1.6200103759765
	y = float32(math.Floor(float64(y*100)) / 100)
	z := info.Position.Z
	if z >= 0 {
		z = float32(math.Floor(float64(z*100)) / 100)
	} else {
		z = float32(math.Floor(float64((z-1)*100)) / 100)
	}
	// 从 PlayerInfoManager 获取 pitch（querytarget 不返回 pitch）
	pitch := float32(0)
	if pi := playerInfoMgr.GetPlayerByEID(info.PlayerUniqueID); pi != nil {
		pitch = pi.Pitch
	}
	return &PlayerPos{
		Dimension: int(info.Dimension),
		Position:  game_interface.TargetQueryingPos{X: x, Y: y, Z: z},
		YRot:      info.YRot,
		Pitch:     pitch,
	}, nil
}

// GetPlayerPosXYZ 获取目标玩家的简略坐标，以三元组返回。
func (bm *BotManager) GetPlayerPosXYZ(target string) (float64, float64, float64, error) {
	pos, err := bm.GetPlayerPos(target)
	if err != nil {
		return 0, 0, 0, err
	}
	return float64(pos.Position.X), float64(pos.Position.Y), float64(pos.Position.Z), nil
}

// ========== 物品 API ==========

// GetPlayerItem 获取玩家背包内指定物品的数量。
// 通过 /clear 命令的返回结果计算（/clear 会返回移除的物品数量）。
// 等效于 td_prism 的 getItem()。
func (bm *BotManager) GetPlayerItem(target, itemName string, itemSpecialID int) (int, error) {
	if !strings.HasPrefix(target, "@") {
		target = `"` + target + `"`
	}
	cmd := fmt.Sprintf("clear %s %s %d 0", target, itemName, itemSpecialID)
	resp, err := bm.SendWSCommandWithResp(cmd)
	if err != nil {
		return 0, fmt.Errorf("GetPlayerItem: %v", err)
	}
	if resp.SuccessCount == 0 {
		return 0, nil
	}
	if len(resp.OutputMessages) == 0 {
		return 0, nil
	}
	msg := resp.OutputMessages[0]
	if msg.Message == "commands.clear.failure.no.items" {
		return 0, nil
	}
	if msg.Message == "commands.generic.syntax" {
		return 0, fmt.Errorf("GetPlayerItem: 物品ID错误: %s", itemName)
	}
	// Parameters[1] 是移除的物品数量
	if len(msg.Parameters) < 2 {
		// 用 SuccessCount 近似
		return int(resp.SuccessCount), nil
	}
	count, err := strconv.Atoi(msg.Parameters[1])
	if err != nil {
		return int(resp.SuccessCount), nil
	}
	// /clear 返回的即实际移除数量，不再除以 2。
	return count, nil
}

// ========== 计分板 API ==========

// GetScore 获取指定计分板中目标的分数。
// 等效于 td_prism 的 getScore()。
func (bm *BotManager) GetScore(scoreboard, target string) (int, error) {
	if target == "*" || scoreboard == "*" {
		return 0, fmt.Errorf("GetScore: 不支持通配符")
	}
	if !strings.HasPrefix(target, "@") {
		target = `"` + target + `"`
	}
	cmd := fmt.Sprintf("scoreboard players test %s %s 0 0", target, scoreboard)
	resp, err := bm.SendWSCommandWithResp(cmd)
	if err != nil {
		return 0, fmt.Errorf("GetScore: %v", err)
	}
	if len(resp.OutputMessages) == 0 {
		return 0, fmt.Errorf("GetScore: 无返回")
	}
	msg := resp.OutputMessages[0]
	switch msg.Message {
	case "commands.scoreboard.objectiveNotFound":
		return 0, fmt.Errorf("GetScore: 计分板 %s 未找到", scoreboard)
	case "commands.scoreboard.players.list.player.empty":
		return 0, fmt.Errorf("GetScore: 计分板项或玩家 %s:%s 未找到", scoreboard, target)
	case "commands.scoreboard.players.score.notFound":
		return 0, fmt.Errorf("GetScore: %s 在计分板 %s 没有分数", target, scoreboard)
	}
	if len(msg.Parameters) < 1 {
		return 0, fmt.Errorf("GetScore: 无法解析分数: %s", msg.Message)
	}
	return strconv.Atoi(msg.Parameters[0])
}

// GetMultiScore 获取指定目标在多个计分板的分数。
// 等效于 td_prism 的 getMultiScore()。
// 返回 map[玩家名]map[计分板名]分数
func (bm *BotManager) GetMultiScore(target string) (map[string]map[string]int, error) {
	cmd := fmt.Sprintf("scoreboard players list %s", target)
	resp, err := bm.SendWSCommandWithResp(cmd)
	if err != nil {
		return nil, fmt.Errorf("GetMultiScore: %v", err)
	}
	result := make(map[string]map[string]int)
	var currentPlayer string
	for _, msg := range resp.OutputMessages {
		switch msg.Message {
		case "commands.scoreboard.players.list.player.empty":
			continue
		case "commands.scoreboard.players.list.player.count":
			if len(msg.Parameters) >= 2 {
				currentPlayer = strings.TrimPrefix(msg.Parameters[1], "_")
			}
		case "commands.scoreboard.players.list.player.entry":
			if len(msg.Parameters) >= 3 {
				scoreName := msg.Parameters[2]
				score, _ := strconv.Atoi(msg.Parameters[0])
				if result[currentPlayer] == nil {
					result[currentPlayer] = make(map[string]int)
				}
				result[currentPlayer][scoreName] = score
			}
		}
	}
	return result, nil
}

// ========== 方块 API ==========

// GetBlockTile 获取指定坐标的方块名称。
// 通过 /testforblock 命令判断是否为指定方块。
// 等效于 td_prism 的 getBlockTile()。
func (bm *BotManager) GetBlockTile(x, y, z int) (string, error) {
	cmd := fmt.Sprintf("testforblock %d %d %d air", x, y, z)
	resp, err := bm.SendWSCommandWithResp(cmd)
	if resp != nil {
		debugLog("[方块] GetBlockTile(%d,%d,%d) SuccessCount=%d 输出数=%d", x, y, z, resp.SuccessCount, len(resp.OutputMessages))
		for i, om := range resp.OutputMessages {
			debugLog("[方块]   out[%d] msg=%q params=%v", i, om.Message, om.Parameters)
		}
	}
	if err != nil {
		debugLog("[方块] GetBlockTile err=%v", err)
		return "", fmt.Errorf("GetBlockTile: %v", err)
	}
	if len(resp.OutputMessages) == 0 {
		return "air", nil
	}
	msg := resp.OutputMessages[0]
	if msg.Message == "commands.testforblock.outOfWorld" {
		return "出界", nil
	}
	// SuccessCount > 0 → 该坐标是空气
	if resp.SuccessCount > 0 {
		return "air", nil
	}
	// testforblock 失败：参数为 [x,y,z, 实际方块, 被测方块]。
	// 标准版实际方块在 params[4]，网易服在 params[3]。用"找非被测(air)的那个"来兼容两种布局。
	if len(msg.Parameters) >= 4 {
		for _, p := range msg.Parameters[3:] {
			name := cleanTileName(p)
			if name != "" && name != "air" {
				debugLog("[方块] 实际方块=%q (param=%q)", name, p)
				return name, nil
			}
		}
	}
	debugLog("[方块] 未解析出方块，返回 unknown")
	return "unknown", nil
}

// cleanTileName 去掉方块的本地化前缀/后缀，如 "%tile.bedrock.name" → "bedrock"。
func cleanTileName(raw string) string {
	name := strings.TrimPrefix(raw, "%tile.")
	name = strings.TrimSuffix(name, ".name")
	return name
}

// GetBlockTileSafe 查询任意坐标的方块：机器人先传送到目标上方加载区块，查询后传回原位。
// MCBE 的 testforblock 只对已加载区块有效，直接查远处会返回 outOfWorld。
// 只应在独立 goroutine 中调用（会传送机器人，且需等待区块加载）。
func (bm *BotManager) GetBlockTileSafe(x, y, z int) (string, error) {
	// 1. 记录机器人自身坐标（用于查询后传回）
	sx, sy, sz, err := bm.GetPlayerPosXYZ("@s")
	if err != nil {
		debugLog("[方块] GetBlockTileSafe(%d,%d,%d) 取自身坐标失败: %v", x, y, z, err)
		return "", err
	}
	debugLog("[方块] GetBlockTileSafe(%d,%d,%d) 自身坐标=(%.0f,%.0f,%.0f)", x, y, z, sx, sy, sz)
	// 2. 传送到目标上方高空（加载该 x,z 区块，避免卡进方块）
	tpY := 150
	if err := bm.TP(x, tpY, z); err != nil {
		debugLog("[方块] GetBlockTileSafe 传送失败: %v", err)
		return "", err
	}
	// 3. 等区块加载（游戏刻 + 一小段时间）
	_ = bm.AwaitChangesGeneral()
	time.Sleep(400 * time.Millisecond)
	// 4. 查询方块
	name, err := bm.GetBlockTile(x, y, z)
	// 5. 传送回原位
	if err2 := bm.TP(int(sx), int(sy), int(sz)); err2 != nil {
		debugLog("[方块] GetBlockTileSafe 传回失败: %v", err2)
	}
	debugLog("[方块] GetBlockTileSafe(%d,%d,%d) → %q err=%v", x, y, z, name, err)
	return name, err
}

// ========== 目标选择器 API ==========

// GetTarget 解析目标选择器，返回匹配的玩家名列表。
// 等效于 td_prism 的 getTarget()。
func (bm *BotManager) GetTarget(selector string) ([]string, error) {
	if !strings.HasPrefix(selector, "@") {
		return nil, fmt.Errorf("GetTarget: 必须使用目标选择器（以 @ 开头）")
	}
	cmd := fmt.Sprintf("testfor %s", selector)
	resp, err := bm.SendWSCommandWithResp(cmd)
	if err != nil {
		return nil, fmt.Errorf("GetTarget: %v", err)
	}
	if resp.SuccessCount == 0 {
		return []string{}, nil
	}
	if len(resp.OutputMessages) == 0 || len(resp.OutputMessages[0].Parameters) == 0 {
		return []string{}, nil
	}
	msg := resp.OutputMessages[0]
	if msg.Message == "commands.generic.syntax" {
		return nil, fmt.Errorf("GetTarget: 目标选择器语法错误: %s", selector)
	}
	players := strings.Split(msg.Parameters[0], ", ")
	// 过滤空字符串
	var result []string
	for _, p := range players {
		if p != "" {
			result = append(result, p)
		}
	}
	return result, nil
}

// ========== 命令执行检查 API ==========

// IsCmdSuccess 检查命令执行是否成功。
// 等效于 td_prism 的 isCmdSuccess()。
func (bm *BotManager) IsCmdSuccess(cmd string) (bool, error) {
	resp, err := bm.SendWSCommandWithResp(cmd)
	if err != nil {
		return false, err
	}
	return resp.SuccessCount > 0, nil
}

// ========== 常加载区域 API ==========

// GetTickingAreaList 获取所有常加载区域列表。
// 等效于 td_prism 的 getTickingAreaList()。
func (bm *BotManager) GetTickingAreaList() (map[string]*TickingAreaInfo, error) {
	cmd := "tickingarea list all-dimensions"
	resp, err := bm.SendWSCommandWithResp(cmd)
	if err != nil {
		return nil, fmt.Errorf("GetTickingAreaList: %v", err)
	}
	result := make(map[string]*TickingAreaInfo)
	if len(resp.OutputMessages) == 0 {
		return result, nil
	}
	if !resp.OutputMessages[0].Success {
		return result, nil
	}
	// 解析多行输出
	for _, msg := range resp.OutputMessages {
		text := msg.Message
		if strings.Contains(text, "%dimension.dimensionName") {
			parts := strings.Split(text, ": \n")
			if len(parts) < 2 {
				continue
			}
			dimName := strings.TrimPrefix(parts[0], "%dimension.dimensionName")
			entries := strings.Split(parts[1], "\n")
			for _, entry := range entries {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}
				// 格式: "- 名称: (startX, startZ) to (endX, endZ)"
				if !strings.HasPrefix(entry, "- ") {
					continue
				}
				entry = strings.TrimPrefix(entry, "- ")
				nameEnd := strings.Index(entry, ": ")
				if nameEnd < 0 {
					continue
				}
				name := entry[:nameEnd]
				coordPart := entry[nameEnd+2:]
				var sx, sz, ex, ez float64
				n, _ := fmt.Sscanf(coordPart, "(%f, %f) to (%f, %f)", &sx, &sz, &ex, &ez)
				if n == 4 {
					result[name] = &TickingAreaInfo{
						Dimension: dimName,
						StartX:    sx, StartZ: sz,
						EndX: ex, EndZ: ez,
					}
				}
			}
		}
	}
	return result, nil
}

// ========== 背包查询 API ==========

// QueryPlayerInventory 查询玩家背包内容。
// 通过网易 codebuilder_actorinfo 命令实现。
// 等效于 td_prism 的 queryPlayerInventory()。
func (bm *BotManager) QueryPlayerInventory(selector string) (*PlayerInventory, error) {
	cmd := fmt.Sprintf("codebuilder_actorinfo inventory %s", selector)
	resp, err := bm.SendWSCommandWithResp(cmd)
	if err != nil {
		return nil, fmt.Errorf("QueryPlayerInventory: %v", err)
	}
	if resp.SuccessCount < 1 {
		return nil, fmt.Errorf("QueryPlayerInventory: 查询失败")
	}
	if resp.DataSet == "" {
		// 尝试从 OutputMessages 解析
		if len(resp.OutputMessages) > 0 && len(resp.OutputMessages[0].Parameters) > 0 {
			data := resp.OutputMessages[0].Parameters[0]
			var inv PlayerInventory
			if err := json.Unmarshal([]byte(data), &inv); err != nil {
				return nil, fmt.Errorf("QueryPlayerInventory: 解析失败: %v", err)
			}
			return &inv, nil
		}
		return nil, fmt.Errorf("QueryPlayerInventory: 无返回数据")
	}
	var inv PlayerInventory
	if err := json.Unmarshal([]byte(resp.DataSet), &inv); err != nil {
		return nil, fmt.Errorf("QueryPlayerInventory: 解析 DataSet 失败: %v", err)
	}
	return &inv, nil
}

// ========== 消息 API ==========

// SendRichTellraw 发送 tellraw 消息给所有玩家（支持自动转义）。
func (bm *BotManager) SendRichTellraw(msg string) error {
	return bm.SendChat("@a", msg)
}

// SendTitle 发送标题给指定玩家。
// 等效于 td_prism 的 player_title()。
func (bm *BotManager) SendTitle(target, title, subtitle string) error {
	target = toPlayerSelector(target)
	if subtitle != "" {
		if err := bm.SendTitleraw(target, "title", title); err != nil {
			return err
		}
		return bm.SendTitleraw(target, "subtitle", subtitle)
	}
	return bm.SendTitleraw(target, "title", title)
}

// SendActionbar 发送动作栏消息给指定玩家。
// 等效于 td_prism 的 player_actionbar()。
func (bm *BotManager) SendActionbar(target, text string) error {
	return bm.SendTitleraw(toPlayerSelector(target), "actionbar", text)
}

// SendChat 发送聊天消息给指定玩家（使用 /tellraw 或 /say）。
// 等效于 td_prism 的 say_to()。
func (bm *BotManager) SendChat(target, message string) error {
	return bm.sendTargeted(target, message)
}

// ========== 等待消息 API ==========

// playerWaitMsgCbs 存储等待消息的回调
var playerWaitMsgCbs = make(map[string]chan string)

// playerWaitMsgMu 保护 playerWaitMsgCbs 的并发读写（收包循环与 WaitMsg 并发访问）。
var playerWaitMsgMu sync.Mutex

// deliverWaitMsg 将玩家聊天消息投递给等待该玩家消息的 WaitMsg 调用。
// 无等待者返回 false；成功投递返回 true；缓冲满导致消息被丢弃时返回 false。
// 查找与投递在同一个锁内完成，避免等待者超时注销与新的注册之间产生竞态。
func deliverWaitMsg(playerName, message string) bool {
	playerWaitMsgMu.Lock()
	defer playerWaitMsgMu.Unlock()
	ch, ok := playerWaitMsgCbs[playerName]
	if !ok {
		return false
	}
	select {
	case ch <- message:
		return true
	default:
		return false // 缓冲满，消息被丢弃
	}
}

// hasWaitMsg 判断指定玩家当前是否有活跃的 WaitMsg 会话在等待。
func hasWaitMsg(playerName string) bool {
	playerWaitMsgMu.Lock()
	defer playerWaitMsgMu.Unlock()
	_, ok := playerWaitMsgCbs[playerName]
	return ok
}

// WaitMsg 等待指定玩家发送聊天消息。
// 等效于 td_prism 的 waitMsg()。
func (bm *BotManager) WaitMsg(playerName string, timeout time.Duration) (string, error) {
	return bm.WaitMsgContext(context.Background(), playerName, timeout)
}

func (bm *BotManager) WaitMsgContext(ctx context.Context, playerName string, timeout time.Duration) (string, error) {
	ch := make(chan string, 1)
	playerWaitMsgMu.Lock()
	playerWaitMsgCbs[playerName] = ch
	playerWaitMsgMu.Unlock()
	defer func() {
		playerWaitMsgMu.Lock()
		delete(playerWaitMsgCbs, playerName)
		playerWaitMsgMu.Unlock()
	}()

	select {
	case msg := <-ch:
		return msg, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("WaitMsg: 等待 %s 的消息超时", playerName)
	case <-bm.stopCh:
		return "", fmt.Errorf("连接已断开")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// ========== 效果 API ==========

// SetPlayerEffect 设置玩家状态效果。
// 等效于 td_prism 的 set_player_effect()。
func (bm *BotManager) SetPlayerEffect(playerName, effect string, duration, level int, showParticles bool) error {
	if level > 255 {
		return fmt.Errorf("效果等级最高255，当前: %d", level)
	}
	if duration > 1000000 {
		return fmt.Errorf("持续时间最高1000000，当前: %d", duration)
	}
	if !strings.HasPrefix(playerName, "@") {
		playerName = `"` + playerName + `"`
	}
	cmd := fmt.Sprintf("effect %s %s %d %d %t", playerName, effect, duration, level, showParticles)
	resp, err := bm.SendWSCommandWithResp(cmd)
	if err != nil {
		return fmt.Errorf("SetPlayerEffect: %v", err)
	}
	if len(resp.OutputMessages) == 0 {
		return fmt.Errorf("SetPlayerEffect: 无返回")
	}
	msg := resp.OutputMessages[0]
	switch msg.Message {
	case "commands.generic.noTargetMatch":
		return fmt.Errorf("SetPlayerEffect: 没有匹配的目标: %s", playerName)
	case "commands.effect.success":
		return nil
	default:
		return fmt.Errorf("SetPlayerEffect: %s", msg.Message)
	}
}

// ========== 工具方法 ==========

// normalizePlayerName 将玩家名转为目标选择器格式。
// 如果已经是 @ 开头则不变，否则用引号包裹。
func normalizePlayerName(name string) string {
	if strings.HasPrefix(name, "@") {
		return name
	}
	return `"` + name + `"`
}
