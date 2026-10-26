package nbt_block

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_console"
	"github.com/OmineDev/flowers-for-machines/utils"
)

// TODO: Merge this to game interface
func simpleStructureGetter(console *nbt_console.Console) (nbtMap map[string]any, err error) {
	var (
		api         *game_interface.GameInterface = console.API()
		resp        *packet.StructureTemplateDataResponse
		terminalErr error
	)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("获取简单结构: %v", r)
		}
	}()
	if data, available, readErr := api.ReadHostStructure(console.Center(), protocol.BlockPos{1, 1, 1}); available {
		if readErr != nil {
			return nil, fmt.Errorf("获取简单结构: %w", readErr)
		}
		return simpleStructureNBT(data)
	}
	// ExportFromSave requires an actual saved structure. A fixed unsaved name
	// may be silently ignored by the server and previously waited forever.
	saved, saveErr := api.StructureBackup().BackupStructure(console.Center())
	if saveErr != nil {
		return nil, saveErr
	}
	name := utils.MakeUUIDSafeString(saved)
	defer api.StructureBackup().DeleteStructure(saved)

	doOnce := new(sync.Once)
	channel := make(chan struct{})
	uniqueID, err := api.PacketListener().ListenPacket(
		[]uint32{packet.IDStructureTemplateDataResponse},
		func(p packet.Packet, connCloseErr error) {
			if connCloseErr == nil && strings.TrimPrefix(p.(*packet.StructureTemplateDataResponse).StructureName, "mystructure:") != name {
				return
			}
			doOnce.Do(func() {
				if connCloseErr != nil {
					terminalErr = connCloseErr
				} else {
					resp = p.(*packet.StructureTemplateDataResponse)
				}
				close(channel)
			})
		},
	)
	if err != nil {
		return nil, fmt.Errorf("获取简单结构: %v", err)
	}
	defer api.PacketListener().DestroyListener(uniqueID)

	err = api.Resources().WritePacket(
		&packet.StructureTemplateDataRequest{
			StructureName: name,
			Position:      console.Center(),
			Settings: protocol.StructureSettings{
				PaletteName:               "default",
				IgnoreEntities:            true,
				IgnoreBlocks:              false,
				Size:                      protocol.BlockPos{1, 1, 1},
				Offset:                    protocol.BlockPos{0, 0, 0},
				LastEditingPlayerUniqueID: api.GetBotInfo().EntityUniqueID,
				Rotation:                  0,
				Mirror:                    0,
				Integrity:                 100,
				Seed:                      0,
				AllowNonTickingChunks:     false,
			},
			RequestType: packet.StructureTemplateRequestExportFromSave,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("获取简单结构: %v", err)
	}

	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	select {
	case <-channel:
	case <-timer.C:
		return nil, fmt.Errorf("获取简单结构: 等待结构响应超时（8秒）")
	}
	if terminalErr != nil {
		return nil, fmt.Errorf("获取简单结构: %v", terminalErr)
	}

	if resp == nil || !resp.Success {
		return nil, fmt.Errorf("获取简单结构: 服务器未返回有效结构")
	}
	return simpleStructureNBT(resp.StructureTemplate)
}

func simpleStructureNBT(data map[string]any) (map[string]any, error) {
	s, ok := data["structure"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("结构缺少structure字段")
	}
	p, ok := s["palette"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("结构缺少palette字段")
	}
	d, ok := p["default"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("结构缺少default调色板")
	}
	positions, _ := d["block_position_data"].(map[string]any)
	first, _ := positions["0"].(map[string]any)
	nbt, _ := first["block_entity_data"].(map[string]any)
	if nbt == nil {
		nbt = map[string]any{}
	}
	return nbt, nil
}
