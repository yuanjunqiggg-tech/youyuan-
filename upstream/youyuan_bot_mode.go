package main

import (
	"sync/atomic"
	"time"

	"bot-apk/config"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
)

// Commands and permission checks must not silently change the player's mode.
const yyOPProbeCommand = "testfor @s"

type yyBotMaintainer struct {
	bm             *BotManager
	running        atomic.Bool
	pending        atomic.Bool
	lastAttempt    atomic.Int64
	retryScheduled atomic.Bool
}

func (m *yyBotMaintainer) TouchMaintainer(api *game_interface.GameInterface) error {
	if !config.ReloadConfig().AutoCreative {
		return nil
	}
	m.pending.Store(true)
	if !m.running.CompareAndSwap(false, true) {
		return nil
	}
	defer m.running.Store(false)
	// Coalesce a mode change received during flight setup instead of losing it.
	// A failed permission/flight request never creates an unbounded retry loop.
	for attempt := 0; attempt < 2 && m.pending.Swap(false); attempt++ {
		m.lastAttempt.Store(time.Now().UnixNano())
		if !config.ReloadConfig().AutoCreative {
			return nil
		}
		err := m.bm.SendCommandNoWait(ChannelAI, "gamemode creative @s")
		if err != nil {
			return err
		}
		// Maintenance must work with sendcommandfeedback=false. Avoid the
		// querytarget/confirmation path used by interactive movement helpers.
		m.bm.mu.Lock()
		conn, pos := m.bm.conn, m.bm.pos
		m.bm.isFlying, m.bm.flyRequested = true, true
		m.bm.mu.Unlock()
		if conn == nil {
			return nil
		}
		if err = m.bm.sendFlyingInput(conn, pos, true); err != nil {
			return err
		}
	}
	return nil
}

func (m *yyBotMaintainer) PacketToListen() map[uint32]bool {
	packets := game_interface.DefaultMaintainer.PacketToListen()
	packets[packet.IDUpdateAbilities] = true
	return packets
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
	case *packet.UpdateAbilities:
		if p.AbilityData.EntityUniqueID == api.GetBotInfo().EntityUniqueID && len(p.AbilityData.Layers) > 0 {
			maintain = p.AbilityData.Layers[0].Values&protocol.AbilityFlying == 0
		}
	}
	if maintain {
		if m.running.Load() {
			m.pending.Store(true)
			return
		}
		if time.Since(time.Unix(0, m.lastAttempt.Load())) < time.Second {
			m.pending.Store(true)
			if m.retryScheduled.CompareAndSwap(false, true) {
				time.AfterFunc(time.Second-time.Since(time.Unix(0, m.lastAttempt.Load())), func() {
					m.retryScheduled.Store(false)
					if err := m.TouchMaintainer(api); err != nil {
						debugLog("[模式维护] %v", err)
					}
				})
			}
			return
		}
		go func() {
			if err := m.TouchMaintainer(api); err != nil {
				debugLog("[模式维护] %v", err)
			}
		}()
	}
}
