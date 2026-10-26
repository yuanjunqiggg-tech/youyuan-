package service

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/nbt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/block_helper"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_console"
	nbt_hash "github.com/OmineDev/flowers-for-machines/nbt_parser/hash"
	nbt_parser_interface "github.com/OmineDev/flowers-for-machines/nbt_parser/interface"
	"github.com/OmineDev/flowers-for-machines/std_server/define"
	"github.com/OmineDev/flowers-for-machines/utils"
	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

func CheckAlive(c *gin.Context) {
	err := mcClient.Conn().Flush()
	if err != nil {
		c.JSON(http.StatusOK, define.CheckAliveResponse{
			Alive:     false,
			ErrorInfo: fmt.Sprintf("机器人已掉线；err = %v", err),
		})
		return
	}
	c.JSON(http.StatusOK, define.CheckAliveResponse{Alive: true})
}

func ProcessExist(c *gin.Context) {
	mu.Lock()
	_ = mcClient.Conn().Close()
	go func() {
		time.Sleep(time.Second)
		os.Exit(0)
	}()
}

func ChangeConsolePosition(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	var request define.ChangeConsolePosRequest

	err := c.BindJSON(&request)
	if err != nil {
		c.JSON(http.StatusOK, define.ChangeConsolePosResponse{
			Success:   false,
			ErrorInfo: fmt.Sprintf("解析请求失败：err = %v", err),
		})
		return
	}

	err = console.ChangeConsolePosition(
		request.DimensionID,
		protocol.BlockPos{
			request.CenterX,
			request.CenterY,
			request.CenterZ,
		},
	)
	if err != nil {
		c.JSON(http.StatusOK, define.ChangeConsolePosResponse{
			Success:   false,
			ErrorInfo: fmt.Sprintf("变更控制台位置失败：err = %v", err),
		})
		sendLogRecord(
			define.SourceDefault,
			userName,
			gameInterface.GetBotInfo().BotName,
			define.SystemNameChangeConsolePosition,
			request,
			fmt.Sprintf("%v", err),
		)
		return
	}

	c.JSON(http.StatusOK, define.ChangeConsolePosResponse{Success: true})
}

func PlaceNBTBlock(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	var request define.PlaceNBTBlockRequest
	var blockNBT map[string]any

	err := c.BindJSON(&request)
	if err != nil {
		c.JSON(http.StatusOK, define.PlaceNBTBlockResponse{
			Success:   false,
			ErrorType: define.ResponseErrorTypeParseError,
			ErrorInfo: fmt.Sprintf("解析请求失败：err = %v", err),
		})
		return
	}

	blockNBTBytes, err := base64.StdEncoding.DecodeString(request.BlockNBTBase64String)
	if err != nil {
		c.JSON(http.StatusOK, define.PlaceNBTBlockResponse{
			Success:   false,
			ErrorType: define.ResponseErrorTypeParseError,
			ErrorInfo: fmt.Sprintf("解析方块 NBT base64 字符串失败：err = %v", err),
		})
		return
	}
	err = nbt.UnmarshalEncoding(blockNBTBytes, &blockNBT, nbt.LittleEndian)
	if err != nil {
		c.JSON(http.StatusOK, define.PlaceNBTBlockResponse{
			Success:   false,
			ErrorType: define.ResponseErrorTypeParseError,
			ErrorInfo: fmt.Sprintf("方块 NBT 字节数据损坏：err = %v", err),
		})
		return
	}

	canFast, uniqueID, offset, err := wrapper.PlaceNBTBlock(
		request.BlockName,
		utils.ParseBlockStatesString(request.BlockStatesString),
		blockNBT,
	)
	if err != nil {
		c.JSON(http.StatusOK, define.PlaceNBTBlockResponse{
			Success:   false,
			ErrorType: define.ResponseErrorTypeRuntimeError,
			ErrorInfo: fmt.Sprintf("运行时错误：放置 NBT 方块失败：err = %v", err),
		})
		sendLogRecord(
			define.SourceDefault,
			userName,
			gameInterface.GetBotInfo().BotName,
			define.SystemNamePlaceNBTBlock,
			request,
			fmt.Sprintf("%v", err),
		)
		return
	}

	c.JSON(http.StatusOK, define.PlaceNBTBlockResponse{
		Success:           true,
		CanFast:           canFast,
		StructureUniqueID: uniqueID.String(),
		StructureName:     utils.MakeUUIDSafeString(uniqueID),
		OffsetX:           offset.X(),
		OffsetY:           offset.Y(),
		OffsetZ:           offset.Z(),
	})
}

func PlaceLargeChest(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	var request define.PlaceLargeChestRequest
	var success bool
	var errorInfo string

	err := c.BindJSON(&request)
	if err != nil {
		c.JSON(http.StatusOK, define.PlaceLargeChestResponse{
			Success:   false,
			ErrorInfo: fmt.Sprintf("解析请求失败：err = %v", err),
		})
		return
	}

	defer func() {
		if !success {
			c.JSON(http.StatusOK, define.PlaceLargeChestResponse{
				Success:   false,
				ErrorInfo: errorInfo,
			})
			sendLogRecord(
				define.SourceDefault,
				userName,
				gameInterface.GetBotInfo().BotName,
				define.SystemNamePlaceLargeChest,
				request,
				errorInfo,
			)
		}
	}()

	// Step 1: Prepare
	center := console.Center()
	pairedOffset := protocol.BlockPos{request.PairedChestOffsetX, 0, request.PairedChestOffsetZ}
	chestBlockStates := utils.ParseBlockStatesString(request.BlockStatesString)

	// Step 2: Clean blocks if needed
	enumOffsets := []protocol.BlockPos{
		{1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {0, 0, -1},
	}
	for _, offset := range enumOffsets {
		nearBlock := console.NearBlockByIndex(nbt_console.ConsoleIndexCenterBlock, offset)

		_, ok := (*nearBlock).(block_helper.Air)
		if ok {
			continue
		}

		err := gameInterface.SetBlock().SetBlock(
			console.NearBlockPosByIndex(nbt_console.ConsoleIndexCenterBlock, offset),
			"minecraft:air",
			"[]",
		)
		if err != nil {
			errorInfo = fmt.Sprintf("清理偏移 %v 处的方块失败：err = %v", offset, err)
			return
		}

		*nearBlock = block_helper.Air{}
	}

	// Step 2: Calculate chest position
	pairleadPos := protocol.BlockPos{
		center[0], center[1] + 1, center[2],
	}
	pairedPos := center

	// Step 3.1: Place pairlead chest
	if !request.PairleadChestStructureExist {
		err = gameInterface.SetBlock().SetBlock(pairleadPos, request.BlockName, request.BlockStatesString)
		if err != nil {
			errorInfo = fmt.Sprintf("放置带头箱子失败：err = %v", err)
			return
		}
	} else {
		uniqueID, err := uuid.Parse(request.PairleadChestUniqueID)
		if err != nil {
			errorInfo = fmt.Sprintf("解析带头箱子结构唯一 ID 失败：err = %v", err)
			return
		}
		err = gameInterface.StructureBackup().RevertStructure(uniqueID, pairleadPos)
		if err != nil {
			errorInfo = fmt.Sprintf("回滚带头箱子结构失败：err = %v", err)
			return
		}
	}
	nearBlock := console.NearBlockByIndex(nbt_console.ConsoleIndexCenterBlock, protocol.BlockPos{0, 1, 0})
	*nearBlock = block_helper.ComplexBlock{
		KnownStates: true,
		Name:        request.BlockName,
		States:      chestBlockStates,
	}

	// Step 3.2: Place paired chest
	if !request.PairedChestStructureExist {
		err = gameInterface.SetBlock().SetBlock(pairedPos, request.BlockName, request.BlockStatesString)
		if err != nil {
			errorInfo = fmt.Sprintf("放置配对箱子失败：err = %v", err)
			return
		}
	} else {
		uniqueID, err := uuid.Parse(request.PairedChestUniqueID)
		if err != nil {
			errorInfo = fmt.Sprintf("解析配对箱子结构唯一 ID 失败：err = %v", err)
			return
		}
		err = gameInterface.StructureBackup().RevertStructure(uniqueID, pairedPos)
		if err != nil {
			errorInfo = fmt.Sprintf("回滚配对箱子结构失败：err = %v", err)
			return
		}
	}
	console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        request.BlockName,
		States:      chestBlockStates,
	})

	// Step 4.1: Clone paired chest to ~~1~
	err = gameInterface.Commands().SendSettingsCommand(
		fmt.Sprintf(
			"clone %d %d %d %d %d %d %d %d %d",
			pairedPos[0], pairedPos[1], pairedPos[2],
			pairedPos[0], pairedPos[1], pairedPos[2],
			center[0]+pairedOffset[0], center[1]+1, center[2]+pairedOffset[2],
		),
		true,
	)
	if err != nil {
		errorInfo = fmt.Sprintf("克隆命令执行失败：err = %v", err)
		return
	}

	// Step 4.2: Wait clone down
	err = gameInterface.Commands().AwaitChangesGeneral()
	if err != nil {
		errorInfo = fmt.Sprintf("等待变化完成失败（阶段 1）：err = %v", err)
		return
	}

	// Step 5: Get final structure (that included a large chest)
	finalStructure, err := gameInterface.StructureBackup().BackupOffset(pairleadPos, pairedOffset)
	if err != nil {
		errorInfo = fmt.Sprintf("获取最终结构失败：err = %v", err)
		return
	}

	// Step 6.1: Clean loaded large chest
	err = gameInterface.Commands().SendSettingsCommand(
		fmt.Sprintf(
			"fill %d %d %d %d %d %d air",
			pairleadPos[0], pairleadPos[1], pairleadPos[2],
			pairleadPos[0]+pairedOffset[0], pairleadPos[1], pairleadPos[2]+pairedOffset[2],
		),
		true,
	)
	if err != nil {
		errorInfo = fmt.Sprintf("清理已放置的箱子失败：err = %v", err)
		return
	}

	// Step 6.2: Wait clean down
	err = gameInterface.Commands().AwaitChangesGeneral()
	if err != nil {
		errorInfo = fmt.Sprintf("等待变化完成失败（阶段 2）：err = %v", err)
		return
	}
	nearBlock = console.NearBlockByIndex(nbt_console.ConsoleIndexCenterBlock, protocol.BlockPos{0, 1, 0})
	*nearBlock = block_helper.Air{}

	success = true
	c.JSON(http.StatusOK, define.PlaceLargeChestResponse{
		Success:           true,
		StructureUniqueID: finalStructure.String(),
		StructureName:     utils.MakeUUIDSafeString(finalStructure),
	})
}

func GetNBTBlockHash(c *gin.Context) {
	var request define.GetNBTBlockHashRequest
	var blockNBT map[string]any
	var hash uint64

	err := c.BindJSON(&request)
	if err != nil {
		c.JSON(http.StatusOK, define.GetNBTBlockHashResponse{
			Success:   false,
			ErrorInfo: fmt.Sprintf("解析请求失败：err = %v", err),
		})
		return
	}

	blockNBTBytes, err := base64.StdEncoding.DecodeString(request.BlockNBTBase64String)
	if err != nil {
		c.JSON(http.StatusOK, define.GetNBTBlockHashResponse{
			Success:   false,
			ErrorInfo: fmt.Sprintf("解析方块 NBT base64 字符串失败：err = %v", err),
		})
		return
	}
	err = nbt.UnmarshalEncoding(blockNBTBytes, &blockNBT, nbt.LittleEndian)
	if err != nil {
		c.JSON(http.StatusOK, define.GetNBTBlockHashResponse{
			Success:   false,
			ErrorInfo: fmt.Sprintf("方块 NBT 字节数据损坏：err = %v", err),
		})
		return
	}

	block, err := nbt_parser_interface.ParseBlock(
		gameInterface.Resources().ConstantPacket().ItemCanGetByCommand,
		request.BlockName,
		utils.ParseBlockStatesString(request.BlockStatesString),
		blockNBT,
	)
	if err != nil {
		c.JSON(http.StatusOK, define.GetNBTBlockHashResponse{
			Success:   false,
			ErrorInfo: fmt.Sprintf("解析目标方块失败：err = %v", err),
		})
		sendLogRecord(
			define.SourceDefault,
			userName,
			gameInterface.GetBotInfo().BotName,
			define.SystemNameGetNBTBlockHash,
			request,
			fmt.Sprintf("%v", err),
		)
		return
	}

	switch request.RequestType {
	case define.RequestTypeFullHash:
		hash = nbt_hash.NBTBlockFullHash(block)
	case define.RequestTypeNBTHash:
		hash = nbt_hash.NBTBlockNBTHash(block)
	case define.RequestTypeContainerSetHash:
		hash = nbt_hash.ContainerSetHash(block)
	default:
		c.JSON(http.StatusOK, define.GetNBTBlockHashResponse{
			Success:   false,
			ErrorInfo: fmt.Sprintf("发现未知的请求类型 %d", request.RequestType),
		})
		return
	}

	c.JSON(http.StatusOK, define.GetNBTBlockHashResponse{
		Success: true,
		Hash:    hash,
	})
}
