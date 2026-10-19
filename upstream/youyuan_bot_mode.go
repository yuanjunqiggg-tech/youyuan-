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
	watching       atomic.Bool
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
		if !m.bm.IsConnected() || !m.bm.IsOP() {
			return nil
		}
		// AutomationPlayer's @s is not reliably the logged-in player on all
		// NetEase servers. Target the actual account, just like construction.
		m.bm.mu.Lock()
		creative := m.bm.gameMode == packet.GameTypeCreative
		m.bm.mu.Unlock()
		var err error
		if !creative {
			if err = m.bm.SendCommandNoWait(ChannelAI, "gamemode creative "+m.bm.quotedName()); err != nil {
				return err
			}
		}
		// Maintenance must work with sendcommandfeedback=false. Avoid the
		// querytarget/confirmation path used by interactive movement helpers.
		m.bm.mu.Lock()
		conn, pos := m.bm.conn, m.bm.pos
		flying := m.bm.isFlying
		m.bm.flyRequested = true
		stop := m.bm.stopCh
		m.bm.mu.Unlock()
		if conn == nil || flying {
			return nil
		}
		select {
		case <-stop:
			return nil
		default:
		}
		if err = m.bm.sendFlyingInput(conn, pos, true); err != nil {
			return err
		}
	}
	return nil
}

// Packet-triggered recovery alone loses changes received during startup or
// flight setup. One watchdog per connection reconciles server-confirmed state;
// it never marks an outbound command as a confirmed mode/flight change.
func (m *yyBotMaintainer) startWatchdog(api *game_interface.GameInterface) {
	if !m.watching.CompareAndSwap(false, true) {
		return
	}
	m.bm.mu.Lock()
	stop, conn := m.bm.stopCh, m.bm.conn
	m.bm.mu.Unlock()
	go func() {
		defer m.watching.Store(false)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				m.bm.mu.Lock()
				current := m.bm.conn == conn && m.bm.connected
				needs := m.bm.gameMode != packet.GameTypeCreative || !m.bm.isFlying
				m.bm.mu.Unlock()
				if !current {
					return
				}
				if config.LoadConfig().AutoCreative && needs && !m.running.Load() {
					if err := m.TouchMaintainer(api); err != nil {
						debugLog("[模式维护] %v", err)
					}
				}
			}
		}
	}()
}

func yyFlyingAbility(layers []protocol.AbilityLayer) (flying, known bool) {
	for _, layer := range layers {
		if layer.Abilities&protocol.AbilityFlying != 0 {
			flying, known = layer.Values&protocol.AbilityFlying != 0, true
		}
	}
	return
}

func (m *yyBotMaintainer) PacketToListen() map[uint32]bool {
	packets := game_interface.DefaultMaintainer.PacketToListen()
	packets[packet.IDUpdateAbilities] = true
	return packets
}

func (m *yyBotMaintainer) HandlePacket(pk packet.Packet, api *game_interface.GameInterface) {
	if !config.LoadConfig().AutoCreative {
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
		if p.AbilityData.EntityUniqueID == api.GetBotInfo().EntityUniqueID {
			flying, known := yyFlyingAbility(p.AbilityData.Layers)
			maintain = known && !flying
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
