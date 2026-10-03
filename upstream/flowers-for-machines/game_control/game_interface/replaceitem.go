package game_interface

import (
	"fmt"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/game_control/resources_control"
)

// ReplaceitemPath 指示 replaceitem 时目标物品栏的槽位类型。
// 一些例子是 "slot.inventory" 和 "slot.hotbar"
type ReplaceitemPath string

const (
	ReplacePathInventoryOnly ReplaceitemPath = "slot.inventory"
	ReplacePathHotbarOnly    ReplaceitemPath = "slot.hotbar"
	ReplacePathInventory     ReplaceitemPath = "slot.inventory | slot.hotbar"
)

// ReplaceitemInfo 指示要通过 replaceitem 生成的物品的基本信息
type ReplaceitemInfo struct {
	Name     string                   // 该物品的名称
	Count    uint8                    // 该物品的数量
	MetaData int16                    // 该物品的元数据
	Slot     resources_control.SlotID // 该物品应当生成在哪个槽位
}

// Replaceitem 是基于 Commands 实现的简单 replaceitem 包装
type Replaceitem struct {
	api *Commands
}

// NewReplaceitem 根据 api 创建并返回一个新的 Replaceitem
func NewReplaceitem(api *Commands) *Replaceitem {
	return &Replaceitem{api: api}
}

// replaceitemInInventoryNormal ..
func (r *Replaceitem) replaceitemInInventoryNormal(request string, blocked bool) error {
	api := r.api

	if blocked {
		_, isTimeout, err := api.SendWSCommandWithTimeout(request, DefaultTimeoutCommandRequest)
		if isTimeout {
			// 超时兜底：走网易自动(AICommand)通道 fire-and-forget——SettingsCommand
			// 在网易上对 replaceitem 可能静默失败（与 ReplaceitemInContainerAsync 一致）。
			err = api.SendAICommand(request)
			if err != nil {
				return fmt.Errorf("普通方式填充物品栏失败: %v", err)
			}
			err = api.AwaitChangesGeneral()
			if err != nil {
				return fmt.Errorf("普通方式填充物品栏失败: %v", err)
			}
		}
		if err != nil {
			return fmt.Errorf("普通方式填充物品栏失败: %v", err)
		}
		return nil
	}

	// 走网易自动(AICommand)通道：SettingsCommand 在网易上对 replaceitem 可能静默失败，
	// 导致书本等物品未被放入背包、后续 BookEdit 无书可编辑、完整性检查"书不完整"。
	err := api.SendAICommand(request)
	if err != nil {
		return fmt.Errorf("普通方式填充物品栏失败: %v", err)
	}

	return nil
}

// replaceitemInInventorySpecial ..
func (r *Replaceitem) replaceitemInInventorySpecial(request string, blocked bool) error {
	api := r.api

	// 与普通路径一致，走网易自动(AICommand)通道——SettingsCommand 在网易上对 replaceitem 可能静默失败。
	err := api.SendAICommand(request)
	if err != nil {
		return fmt.Errorf("特殊方式填充物品栏失败: %v", err)
	}

	if blocked {
		err = api.AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("特殊方式填充物品栏失败: %v", err)
		}
	}

	return nil
}

// ReplaceitemInInventory 向背包填充物品。
//
// target 指代被填充物品的目标，是一个目标选择器；
// path 指代该物品所处的槽位类型，一个例子是"slot.hotbar"；
// itemInfo 指代要生成的物品的基本信息；
// method 指代该物品的物品组件信息；
//
// blocked 指示是否使用以阻塞的方式运行此函数，
// 如果为真，它将等待租赁服响应后再返回值
func (r *Replaceitem) ReplaceitemInInventory(
	target string,
	path ReplaceitemPath,
	itemInfo ReplaceitemInfo,
	method string,
	blocked bool,
) error {
	var err error

	if path == ReplacePathInventory {
		if itemInfo.Slot <= 8 {
			path = ReplacePathHotbarOnly
		} else {
			path = ReplacePathInventoryOnly
			itemInfo.Slot -= 9
		}
	}

	request := fmt.Sprintf(
		"replaceitem entity %s %s %d %s %d %d %s",
		target, path,
		itemInfo.Slot, itemInfo.Name, itemInfo.Count, itemInfo.MetaData,
		method,
	)

	if len(request) <= 256 {
		err = r.replaceitemInInventoryNormal(request, blocked)
	} else {
		err = r.replaceitemInInventorySpecial(request, blocked)
	}
	if err != nil {
		return fmt.Errorf("填充物品栏失败: %v", err)
	}

	return nil
}

// ReplaceitemInContainerAsync
// 向 blockPos 处的容器填充物品。
//
// itemInfo 指代要填充的物品的基本信息，
// method 指代该物品的物品组件信息。
//
// 此实现不会等待租赁服响应，数据包被发送后将立即返回值。
// 使用自动化通道（SettingsCommand）发送，与主流程一致。
func (r *Replaceitem) ReplaceitemInContainerAsync(
	blockPos protocol.BlockPos,
	itemInfo ReplaceitemInfo,
	method string,
) error {
	request := fmt.Sprintf(
		"replaceitem block %d %d %d slot.container %d %s %d %d %s",
		blockPos[0], blockPos[1], blockPos[2],
		itemInfo.Slot, itemInfo.Name, itemInfo.Count, itemInfo.MetaData,
		method,
	)
	// 走网易 AICommand（PyRpc）通道——普通导入的 setblock 用这条通道在网易上可靠，
	// 而标准 SettingsCommand 包在网易上可能静默失败（之前容器为空）。
	err := r.api.SendAICommand(request)
	if err != nil {
		return fmt.Errorf("向容器填充物品（异步）：%v", err)
	}
	return nil
}

// ReplaceitemInContainerPlayer 用标准玩家通道（Player origin，fire-and-forget）向容器填充物品。
// 用于自动化通道静默失败、容器为空时的兜底重放。玩家通道多数时候拿不到结果，故不等待。
func (r *Replaceitem) ReplaceitemInContainerPlayer(
	blockPos protocol.BlockPos,
	itemInfo ReplaceitemInfo,
	method string,
) error {
	request := fmt.Sprintf(
		"replaceitem block %d %d %d slot.container %d %s %d %d %s",
		blockPos[0], blockPos[1], blockPos[2],
		itemInfo.Slot, itemInfo.Name, itemInfo.Count, itemInfo.MetaData,
		method,
	)
	return r.api.SendPlayerCommand(request)
}
