package main

import (
	"context"
	"errors"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// CommandChannel 命令通道。
type CommandChannel string

const (
	ChannelPlayer  CommandChannel = "player"  // 以玩家身份执行（权限低）
	ChannelConsole CommandChannel = "console" // 控制台来源（命令走 WS 自动化通道）
	ChannelAI      CommandChannel = "ai"      // AI/WS 自动化通道（权限最高）
)

// Valid 判断通道是否合法。
func (c CommandChannel) Valid() bool {
	switch c {
	case ChannelPlayer, ChannelConsole, ChannelAI:
		return true
	}
	return false
}

// normalizeCommandChannel 将通道别名统一为规范值，非法输入返回 error。
// 与 mcfunction.go 的 normalizeChannel 同语义，但返回带类型 + 校验错误。
func normalizeCommandChannel(raw string) (CommandChannel, error) {
	switch raw {
	case "0", "ai", "魔法指令", "magic", "AI", "Ai":
		return ChannelAI, nil
	case "1", "player", "玩家":
		return ChannelPlayer, nil
	case "2", "console", "控制台":
		return ChannelConsole, nil
	}
	return "", errors.New("未知命令通道: " + raw)
}

// SendCommandCh 统一命令分发：发指令 → 超时等待 → 返回结果。
// isTimeout 表示等待响应超时（命令可能已执行）。连接未就绪时回退到无响应发送。
// 命名带 Ch 后缀以避开 bot_conn.go 中既有的单参 SendCommand(cmd string)。
func (bm *BotManager) SendCommandCh(channel CommandChannel, cmd string, timeout time.Duration) (*packet.CommandOutput, bool, error) {
	if !channel.Valid() {
		return nil, false, errors.New("未知命令通道: " + string(channel))
	}
	if err := bm.waitControlCommandSlot(); err != nil {
		return nil, false, err
	}
	switch channel {
	case ChannelPlayer:
		return bm.SendPlayerCommandWithTimeout(cmd, timeout)
	case ChannelConsole:
		// 控制台命令骑行 WS 自动化通道（与 SendConsole 对 /命令 的处理一致）。
		return bm.SendWSCommandWithTimeout(cmd, timeout)
	case ChannelAI:
		return bm.SendWSCommandWithTimeout(cmd, timeout)
	}
	return nil, false, errors.New("未知命令通道: " + string(channel))
}

// SendCommandNoWait 发指令但不等待响应。
func (bm *BotManager) SendCommandNoWait(channel CommandChannel, cmd string) error {
	return bm.SendCommandNoWaitRate(channel, cmd, 0)
}

// Zero means no artificial dispatch delay; a positive rate is opt-in.
func (bm *BotManager) SendCommandNoWaitRate(channel CommandChannel, cmd string, rate int) error {
	if !channel.Valid() {
		return errors.New("未知命令通道: " + string(channel))
	}
	if err := bm.waitControlCommandRateContext(context.Background(), 1, 1, rate); err != nil {
		return err
	}
	return bm.sendCommandBuffered(channel, cmd)
}

// All plugin/AI/chain channels share one per-account dispatch pace, including submit batches.
// Construction's separate block transport is unaffected.
func (bm *BotManager) waitControlCommandSlot() error { return bm.waitControlCommandSlots(1) }

// Reserve a bounded burst against the same per-account budget.
func (bm *BotManager) waitControlCommandSlots(count int) error {
	return bm.waitControlCommandBudget(count, 16)
}
func (bm *BotManager) waitControlCommandBudget(count, limit int) error {
	return bm.waitControlCommandBudgetContext(context.Background(), count, limit)
}
func (bm *BotManager) waitControlCommandBudgetContext(ctx context.Context, count, limit int) error {
	return bm.waitControlCommandRateContext(ctx, count, limit, 0)
}
func (bm *BotManager) waitControlCommandRateContext(ctx context.Context, count, limit, rate int) error {
	if rate != 0 && (rate < 20 || rate > 20000) {
		return errors.New("命令速率应为20..20000，0无固定CPS")
	}
	if count < 1 || count > limit {
		return errors.New("命令窗口超出允许范围")
	}
	if bm == nil {
		return errors.New("账号未连接")
	}
	if rate == 0 {
		if e := ctx.Err(); e != nil {
			return e
		}
		bm.mu.Lock()
		connected, stop := bm.connected, bm.stopCh
		bm.mu.Unlock()
		if !connected {
			return errors.New("账号未连接")
		}
		select {
		case <-stop:
			return errors.New("连接已停止")
		default:
		}
		bm.controlCommands.Add(uint64(count))
		return nil
	}
	for !bm.controlCommandMu.TryLock() {
		if e := yyChainSleep(ctx, 5*time.Millisecond); e != nil {
			return e
		}
	}
	defer bm.controlCommandMu.Unlock()
	if e := ctx.Err(); e != nil {
		return e
	}
	bm.mu.Lock()
	connected, stop := bm.connected, bm.stopCh
	bm.mu.Unlock()
	if !connected {
		return errors.New("账号未连接")
	}
	if wait := time.Until(bm.controlCommandNext); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		case <-stop:
			return errors.New("连接已停止")
		}
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	pace := yyControlDispatchPace(count, rate)
	bm.controlCommandNext = time.Now().Add(pace)
	bm.controlCommands.Add(uint64(count))
	return nil
}

func yyControlDispatchPace(count, rate int) time.Duration {
	if rate > 0 {
		return yyControlBatchPace(count, rate)
	}
	return 0
}

func yyControlBatchPace(count, rate int) time.Duration {
	if rate == 0 {
		return 0
	}
	return time.Duration(count) * time.Second / time.Duration(rate)
}
