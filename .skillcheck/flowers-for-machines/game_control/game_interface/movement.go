package game_interface

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// 描述切换飞行状态时的最大重试次数
// 每次重试含 3 个双击跳包、间隔 sendPlayerAuthInput 内的 250ms，
// 4 次重试最坏约 3s 才回退到 bot-apk 的兜底，避免起飞/降落卡太久。
const MaxRetryChangeFlyingStates = 4

var (
	StartFlyingInputData1 = []int{
		packet.InputFlagJumpDown,
		packet.InputFlagJumping,
		packet.InputFlagWantUp,
		packet.InputFlagStartFlying,
		packet.InputFlagJumpPressedRaw,
		packet.InputFlagJumpCurrentRaw,
	}
	StartFlyingInputData2 = []int{
		packet.InputFlagJumpDown,
		packet.InputFlagJumping,
		packet.InputFlagWantUp,
		packet.InputFlagStartFlying,
		packet.InputFlagJumpCurrentRaw,
	}
	StartFlyingInputData3 = []int{
		packet.InputFlagStartFlying,
		packet.InputFlagJumpReleasedRaw,
	}
)

var (
	StopFlyingInputData1 = []int{
		packet.InputFlagJumpDown,
		packet.InputFlagJumping,
		packet.InputFlagWantUp,
		packet.InputFlagStopFlying,
		packet.InputFlagJumpPressedRaw,
		packet.InputFlagJumpCurrentRaw,
	}
	StopFlyingInputData2 = []int{
		packet.InputFlagJumpDown,
		packet.InputFlagJumping,
		packet.InputFlagWantUp,
		packet.InputFlagStopFlying,
		packet.InputFlagJumpCurrentRaw,
	}
	StopFlyingInputData3 = []int{
		packet.InputFlagStopFlying,
		packet.InputFlagJumpReleasedRaw,
	}
)

// Movement 基于 ResourcesWrapper 和 Querytarget 实现了机器人的移动
type Movement struct {
	api         *ResourcesWrapper
	querytarget *Querytarget
	localPos    func() ([3]float32, bool) // 兜底：本地数据包坐标（由外部注入）
}

// NewMovement 根据 api 和 querytarget 返回并创建一个新的 Movement
func NewMovement(api *ResourcesWrapper, querytarget *Querytarget) *Movement {
	return &Movement{
		api:         api,
		querytarget: querytarget,
	}
}

// SetLocalPositionProvider 注入本地坐标提供者，作为 querytarget 失败/匹配不到自身时的兜底。
// fn 返回 (位置, 是否有效)；位置无效时继续视为查询失败。
func (m *Movement) SetLocalPositionProvider(fn func() ([3]float32, bool)) {
	m.localPos = fn
}

// getBotPos 以 Querytarget 的方式获取机器人的当前位置。
// 注意不能用 @s：命令经 WS 自动化通道（AutomationPlayer 来源）执行时，@s 不指向
// 机器人实体，querytarget @s 会返回空导致位置查询失败。改用 @a 查询所有玩家，
// 再按机器人的 EntityUniqueID 筛出自己的位置（多机器人下其他子机器人也会返回，按 ID 区分）。
func (m *Movement) getBotPos() (pos [3]float32, err error) {
	info, qErr := m.querytarget.DoQuerytarget("@a")
	if qErr == nil {
		for _, it := range info {
			if it.PlayerUniqueID == m.api.EntityUniqueID {
				return [3]float32{it.Position.X, it.Position.Y, it.Position.Z}, nil
			}
		}
	}
	// 兜底：querytarget 失败或匹配不到自身 → 用注入的本地数据包坐标，
	// 避免在首次进服/刚传送/多机并发等命令不稳的时机让 NBT 操作台初始化整块失败。
	if m.localPos != nil {
		if p, ok := m.localPos(); ok {
			return p, nil
		}
	}
	if qErr != nil {
		return [3]float32{}, fmt.Errorf("获取机器人位置失败: %v", qErr)
	}
	return [3]float32{}, fmt.Errorf("获取机器人位置: 查询机器人位置失败")
}

// sendPlayerAuthInput 向服务器发送 packet.PlayerAuthInput 包。
// pos 指示机器人预期抵达的位置，flags 指示要提交的移动状态控制位。
// 如果该数据包被成功发送，则将确保该函数将会继续阻塞至少一个游戏刻
func (m *Movement) sendPlayerAuthInput(pos [3]float32, flags []int) error {
	inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	for _, flag := range flags {
		inputData.Set(flag)
	}

	err := m.api.WritePacket(&packet.PlayerAuthInput{
		Position:  pos,
		InputData: inputData,
	})
	if err != nil {
		return fmt.Errorf("发送玩家输入失败: %v", err)
	}
	// 双击跳之间需间隔足够久（~250ms），让第一次跳离地后再按第二次，才能触发飞行切换
	time.Sleep(250 * time.Millisecond)

	return nil
}

// StartFlying 将机器人切换到悬停的飞行状态。
// 该函数的调用者有责任确保机器人已处于创造模式
func (m *Movement) StartFlying() error {
	// Prepare
	isFlying := new(atomic.Bool)
	isFlying.Store(false)

	// Listen packet
	uniqueID, err := m.api.PacketListener().ListenPacket(
		[]uint32{packet.IDUpdateAbilities},
		func(p packet.Packet, connCloseErr error) {
			if connCloseErr == nil {
				pk := p.(*packet.UpdateAbilities)
				if pk.AbilityData.EntityUniqueID == m.api.BotInfo.EntityUniqueID {
					flying := pk.AbilityData.Layers[0].Values&protocol.AbilityFlying != 0
					isFlying.Store(flying)
				}
			}
		},
	)
	if err != nil {
		return fmt.Errorf("开始飞行失败: %v", err)
	}
	defer m.api.PacketListener().DestroyListener(uniqueID)

	// Get bot pos
	pos, err := m.getBotPos()
	if err != nil {
		return fmt.Errorf("开始飞行失败: %v", err)
	}

	// Start flying
	for range MaxRetryChangeFlyingStates {
		if err = m.sendPlayerAuthInput(pos, StartFlyingInputData1); err != nil {
			return fmt.Errorf("开始飞行失败: %v", err)
		}
		if err = m.sendPlayerAuthInput(pos, StartFlyingInputData2); err != nil {
			return fmt.Errorf("开始飞行失败: %v", err)
		}
		if err = m.sendPlayerAuthInput(pos, StartFlyingInputData3); err != nil {
			return fmt.Errorf("开始飞行失败: %v", err)
		}
		if isFlying.Load() {
			break
		}
	}
	if !isFlying.Load() {
		return fmt.Errorf("开始飞行: 切换飞行状态失败")
	}

	// Return
	for range 5 {
		if err = m.sendPlayerAuthInput(pos, []int{packet.InputFlagStartFlying}); err != nil {
			return fmt.Errorf("开始飞行失败: %v", err)
		}
	}
	return nil
}

// StopFlying 终止机器人的悬停飞行状态
func (m *Movement) StopFlying() error {
	// Prepare
	isFlying := new(atomic.Bool)
	isFlying.Store(true)

	// Listen packet
	uniqueID, err := m.api.PacketListener().ListenPacket(
		[]uint32{packet.IDUpdateAbilities},
		func(p packet.Packet, connCloseErr error) {
			if connCloseErr == nil {
				pk := p.(*packet.UpdateAbilities)
				if pk.AbilityData.EntityUniqueID == m.api.BotInfo.EntityUniqueID {
					flying := pk.AbilityData.Layers[0].Values&protocol.AbilityFlying != 0
					isFlying.Store(flying)
				}
			}
		},
	)
	if err != nil {
		return fmt.Errorf("停止飞行失败: %v", err)
	}
	defer m.api.PacketListener().DestroyListener(uniqueID)

	// Get bot pos
	pos, err := m.getBotPos()
	if err != nil {
		return fmt.Errorf("停止飞行失败: %v", err)
	}

	// Stop flying
	for range MaxRetryChangeFlyingStates {
		if err = m.sendPlayerAuthInput(pos, StopFlyingInputData1); err != nil {
			return fmt.Errorf("停止飞行失败: %v", err)
		}
		if err = m.sendPlayerAuthInput(pos, StopFlyingInputData2); err != nil {
			return fmt.Errorf("停止飞行失败: %v", err)
		}
		if err = m.sendPlayerAuthInput(pos, StopFlyingInputData3); err != nil {
			return fmt.Errorf("停止飞行失败: %v", err)
		}
		if !isFlying.Load() {
			break
		}
	}
	if isFlying.Load() {
		return fmt.Errorf("停止飞行: 切换飞行状态失败")
	}

	// Return
	for range 5 {
		if err = m.sendPlayerAuthInput(pos, []int{packet.InputFlagStartFlying}); err != nil {
			return fmt.Errorf("停止飞行失败: %v", err)
		}
	}
	return nil
}
