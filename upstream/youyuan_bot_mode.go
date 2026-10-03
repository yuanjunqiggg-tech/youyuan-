package main

import (
	"sync/atomic"

	"bot-apk/config"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
)

// Commands and permission checks must not silently change the player's mode.
const yyOPProbeCommand = "testfor @s"

type yyBotMaintainer struct {
	bm      *BotManager
	running atomic.Bool
}

func (m *yyBotMaintainer) TouchMaintainer(api *game_interface.GameInterface) error {
	if !config.ReloadConfig().AutoCreative || !m.running.CompareAndSwap(false, true) {
		return nil
	}
	defer m.running.Store(false)
	return game_interface.DefaultMaintainer.TouchMaintainer(api)
}

func (m *yyBotMaintainer) PacketToListen() map[uint32]bool {
	return game_interface.DefaultMaintainer.PacketToListen()
}

func (m *yyBotMaintainer) HandlePacket(pk packet.Packet, api *game_interface.GameInterface) {
	if !config.ReloadConfig().AutoCreative {
		return
	}
	maintain := false
	switch p := pk.(type) {
	case *packet.SetPlayerGameType:
		maintain = p.GameType != packet.GameTypeCreative
	case *packet.UpdatePlayerGameType:
		maintain = p.PlayerUniqueID == api.GetBotInfo().EntityUniqueID && p.GameType != packet.GameTypeCreative
	case *packet.Respawn:
		maintain = p.State == packet.RespawnStateReadyToSpawn
	}
	if maintain {
		go func() { _ = m.TouchMaintainer(api) }()
	}
}
