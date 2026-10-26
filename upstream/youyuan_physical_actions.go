package main

import (
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
	"github.com/OmineDev/flowers-for-machines/game_control/resources_control"
	"github.com/OmineDev/flowers-for-machines/utils"
	"github.com/TriM-Organization/bedrock-world-operator/block"
	"github.com/go-gl/mathgl/mgl32"
	"math"
	"strconv"
	"time"
)

func yyPhysicalAction(bm *BotManager, method string, args []any) (any, error) {
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("游戏未连接")
	}
	if !bm.physicalMu.TryLock() {
		return nil, fmt.Errorf("当前账号还有物理动作进行中")
	}
	defer bm.physicalMu.Unlock()
	if wait := time.Until(bm.physicalNext); wait > 0 {
		time.Sleep(wait)
	}
	bm.physicalNext = time.Now().Add(100 * time.Millisecond)
	bm.mu.Lock()
	conn, res, id, dim := bm.conn, bm.resources, bm.runtimeID, bm.dimension
	bm.mu.Unlock()
	if conn == nil || res == nil {
		return nil, fmt.Errorf("库存/连接未就绪")
	}
	self := yySenseActor(bm, "self")
	if self["position_known"] != true {
		return nil, fmt.Errorf("机器人位置未知")
	}
	pos := self["position"].(mgl32.Vec3)
	// Recent Bedrock servers consume block actions from authoritative input.
	input := func(actions []protocol.PlayerBlockAction) error {
		bm.heartbeatWriteMu.Lock()
		defer bm.heartbeatWriteMu.Unlock()
		pk, err := bm.yyPhysicalInput(conn, actions)
		if err != nil {
			return err
		}
		return conn.WritePacket(pk)
	}
	faceIndex, slotIndex := 3, 4
	var blockPos protocol.BlockPos
	var runtime uint32
	var target uint64
	if method == "bot.attack" {
		text, e := yyArgString(args, 0, "")
		if e != nil {
			return nil, e
		}
		target, e = strconv.ParseUint(text, 10, 64)
		if e != nil || target == id {
			return nil, fmt.Errorf("目标 runtime_id 无效")
		}
		actor := yySenseActor(bm, text)
		if actor["position_known"] != true || actor["present"] != true || actor["dimension"] != dim || actor["position"].(mgl32.Vec3).Sub(pos).Len() > 6 {
			return nil, fmt.Errorf("目标需要已观测、同维度、仍存在且在六格内")
		}
		slotIndex = 1
	} else {
		p, e := yySenseVec(args)
		if e != nil {
			return nil, e
		}
		for _, v := range p {
			if math.Trunc(float64(v)) != float64(v) {
				return nil, fmt.Errorf("方块坐标必须为整数")
			}
		}
		if p.Sub(pos).Len() > 6 {
			return nil, fmt.Errorf("目标方块在六格范围外")
		}
		blockPos = protocol.BlockPos{int32(p[0]), int32(p[1]), int32(p[2])}
		s := yySenseFor(bm)
		s.Lock()
		b := s.Blocks[yyBlockKey(dim, blockPos, 0)]
		if b != nil {
			runtime, _ = b["runtime_id"].(uint32)
		}
		s.Unlock()
		if b == nil {
			return nil, fmt.Errorf("该方块暂无服务器观测，请先通过 UpdateBlock/抓包获取，不猜 runtime_id")
		}
		if method == "world.mineBlock" {
			slotIndex = 4
			faceIndex = 5
		}
	}
	slot, e := yyArgNumber(args, slotIndex, 0)
	if e != nil || slot < 0 || slot > 8 || math.Trunc(slot) != slot {
		return nil, fmt.Errorf("快捷栏 slot 必须为 0..8")
	}
	item, known := res.Inventories().GetItemStack(0, resources_control.SlotID(slot))
	if !known || item == nil {
		return nil, fmt.Errorf("库存未同步")
	}
	held := utils.DeepCopyItemInstance(*item)
	if e = conn.WritePacket(&packet.PlayerHotBar{SelectedHotBarSlot: uint32(slot), WindowID: 0, SelectHotBarSlot: true}); e != nil {
		return nil, e
	}
	if method == "bot.attack" {
		// Animate is a server animation notification on the tested NetEase host.
		// The actual attack transaction carries the input; do not inject Animate.
		e = conn.WritePacket(&packet.InventoryTransaction{TransactionData: &protocol.UseItemOnEntityTransactionData{TargetEntityRuntimeID: target, ActionType: protocol.UseItemOnEntityActionAttack, HotBarSlot: int32(slot), HeldItem: held, Position: pos}})
		if e != nil {
			return nil, e
		}
		for range 2 {
			time.Sleep(100 * time.Millisecond)
			if e = input(nil); e != nil {
				return nil, e
			}
		}
		return map[string]any{"submitted": true, "confirmed": false, "target": strconv.FormatUint(target, 10), "completion": "观察目标 ActorEventHurt/health 下降；交易提交不代表造成伤害"}, nil
	}
	face, e := yyArgNumber(args, faceIndex, 1)
	if e != nil || face < 0 || face > 5 || math.Trunc(face) != face {
		return nil, fmt.Errorf("face 必须为 0..5")
	}
	if method == "world.useBlock" {
		bm.mu.Lock()
		api := bm.gameInterface
		bm.mu.Unlock()
		name, states, known := block.RuntimeIDToState(runtime)
		if api == nil || !known {
			return nil, fmt.Errorf("工具箱点击接口/方块映射尚未就绪")
		}
		e = api.BotClick().PlaceBlock(game_interface.UseItemOnBlocks{HotbarSlotID: resources_control.SlotID(slot), BotPos: pos, BlockPos: blockPos, BlockName: name, BlockStates: states}, int32(face))
		if e != nil {
			return nil, e
		}
		return map[string]any{"submitted": true, "confirmed": false, "mode": "toolbox_bot_click", "completion": "已调用工具箱原生点击与同步函数；实际放置或开容器仍需服务器观测确认"}, nil
	}
	action := uint32(protocol.UseItemActionClickBlock)
	if method == "world.mineBlock" {
		ms, e := yyArgNumber(args, 3, 1000)
		if e != nil || ms < 100 || ms > 30000 {
			return nil, fmt.Errorf("duration_ms 必须为 100..30000")
		}
		send := func(action int32) error {
			if e := conn.WritePacket(&packet.PlayerAction{EntityRuntimeID: id, ActionType: action, BlockPosition: blockPos, BlockFace: int32(face)}); e != nil {
				return e
			}
			return input([]protocol.PlayerBlockAction{{Action: action, BlockPos: blockPos, Face: int32(face)}})
		}
		if e = send(protocol.PlayerActionStartBreak); e != nil {
			return nil, e
		}
		defer send(protocol.PlayerActionAbortBreak)
		deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)
		for time.Now().Before(deadline) {
			if !bm.IsConnected() {
				return nil, fmt.Errorf("挖掘过程中断开连接")
			}
			if e = send(protocol.PlayerActionCrackBreak); e != nil {
				return nil, e
			}
			time.Sleep(100 * time.Millisecond)
		}
		if e = send(protocol.PlayerActionStopBreak); e != nil {
			return nil, e
		}
		action = protocol.UseItemActionBreakBlock
	}
	e = conn.WritePacket(&packet.InventoryTransaction{TransactionData: &protocol.UseItemTransactionData{ActionType: action, TriggerType: protocol.TriggerTypePlayerInput, BlockPosition: blockPos, BlockFace: int32(face), HotBarSlot: int32(slot), HeldItem: held, Position: pos, ClickedPosition: mgl32.Vec3{.5, .5, .5}, BlockRuntimeID: runtime, ClientPrediction: protocol.ClientPredictionSuccess}})
	if e != nil {
		return nil, e
	}
	return map[string]any{"submitted": true, "confirmed": false, "mode": "inventory_transaction", "completion": "观察 world.blockState/events.onBlockChange；放置、消耗与挖掘由服务器校验"}, nil
}

// The caller holds heartbeatWriteMu so actions cannot reset or reorder input ticks.
func (bm *BotManager) yyPhysicalInput(conn *minecraft.Conn, actions []protocol.PlayerBlockAction) (*packet.PlayerAuthInput, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if conn == nil || bm.conn != conn || !bm.connected {
		return nil, fmt.Errorf("物理动作连接已关闭")
	}
	flags := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	if bm.flyRequested {
		flags.Set(packet.InputFlagStartFlying)
	}
	if len(actions) > 0 {
		flags.Set(packet.InputFlagPerformBlockActions)
	}
	bm.tick++
	return &packet.PlayerAuthInput{Position: bm.pos, Pitch: bm.pitch, Yaw: bm.yaw, HeadYaw: bm.yaw, Tick: bm.tick, InputData: flags, BlockActions: actions, InputMode: packet.InputModeMouse, PlayMode: packet.PlayModeScreen, InteractionModel: packet.InteractionModelCrosshair}, nil
}
