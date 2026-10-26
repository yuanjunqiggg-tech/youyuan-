package nbt_block

import (
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/block_helper"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/OmineDev/flowers-for-machines/nbt_parser/block"
)

type Skull struct {
	console *nbt_console.Console
	data    nbt_parser_block.Skull
}

func (Skull) Offset() protocol.BlockPos { return protocol.BlockPos{} }
func (s *Skull) Make() error {
	api := s.console.API()
	slot := s.console.HotbarSlotID()
	if err := api.Replaceitem().ReplaceitemInInventory("@s", game_interface.ReplacePathHotbarOnly, game_interface.ReplaceitemInfo{Name: "minecraft:skull", Count: 1, MetaData: int16(s.data.NBT.SkullType), Slot: slot}, "", true); err != nil {
		return fmt.Errorf("创建头颅物品: %w", err)
	}
	s.console.UseInventorySlot(nbt_console.RequesterUser, slot, true)
	if err := s.console.ChangeAndUpdateHotbarSlotID(slot); err != nil {
		return err
	}
	facing := uint8(1)
	if n, ok := s.data.BlockStates()["facing_direction"].(int32); ok && n >= 1 && n <= 5 {
		facing = uint8(n)
	}
	_, offset, pos, err := api.BotClick().PlaceBlockHighLevel(s.console.Center(), slot, facing)
	if pos != nil {
		s.console.UpdatePosition(*pos)
	}
	if err != nil {
		return fmt.Errorf("放置头颅: %w", err)
	}
	*s.console.NearBlockByIndex(nbt_console.ConsoleIndexCenterBlock, offset) = block_helper.NearBlock{Name: game_interface.BasePlaceBlock}
	if err = api.SetBlock().SetBlock(s.console.Center(), s.data.BlockName(), s.data.BlockStatesString()); err != nil {
		return err
	}
	s.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{KnownStates: true, Name: s.data.BlockName(), States: s.data.BlockStates()})
	return nil
}
