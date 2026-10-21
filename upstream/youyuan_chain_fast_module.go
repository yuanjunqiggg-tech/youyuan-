package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/google/uuid"
)

// Only the slot owner reads or changes conn/poisoned. Unknown writes stay blocked until a new connection.
type yyChainFastGate struct {
	once     sync.Once
	slot     chan struct{}
	conn     *minecraft.Conn
	poisoned error
}

func (g *yyChainFastGate) acquire(ctx context.Context) (func(), error) {
	g.once.Do(func() { g.slot = make(chan struct{}, 1) })
	select {
	case g.slot <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-g.slot
			return nil, err
		}
		return func() { <-g.slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *yyChainFastGate) bind(conn *minecraft.Conn) error {
	if g.conn != conn {
		g.conn, g.poisoned = conn, nil
	}
	if g.poisoned != nil {
		return fmt.Errorf("当前连接的快速命令模块结果未知，已停止后续提交；重新连接后方可继续：%w", g.poisoned)
	}
	return nil
}

func (bm *BotManager) sendChainFastModule(ctx context.Context, nodes []yyChainNode, cfg yyChainConfig) ([]map[string]any, error) {
	if bm == nil {
		return nil, fmt.Errorf("命令连接未就绪")
	}
	release, err := bm.chainFast.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	// Read after acquiring the slot: a waiting worker must not rebind a stale connection and clear its poison.
	bm.mu.Lock()
	conn, connected, dim, name := bm.conn, bm.connected, bm.commandDimension, bm.display
	bm.mu.Unlock()
	if !connected || conn == nil {
		return nil, fmt.Errorf("命令连接未就绪")
	}
	if err := bm.chainFast.bind(conn); err != nil {
		return nil, err
	}
	rows, unknown, err := yyChainFastChunks(ctx, nodes, cfg, dim, name,
		func(packets []packet.Packet) (bool, error) {
			if err := bm.waitControlCommandRateContext(ctx, len(packets), 1024, cfg.RateCPS); err != nil {
				return false, err
			}
			if err := ctx.Err(); err != nil {
				return false, err
			}
			// shortcut: the transport write has no context deadline; add transport support if writes can stall.
			return true, conn.WritePacketBatch(packets)
		})
	if unknown {
		bm.chainFast.poisoned = err
	}
	return rows, err
}

func yyChainFastPacket(node yyChainNode, cfg yyChainConfig, dim, name string) (packet.Packet, int, error) {
	if node.Control != nil || node.Conditional || node.RequireResult {
		return nil, 0, fmt.Errorf("快速模块只能包含无等待的普通命令，条件与控制节点仍需独立执行")
	}
	channel, err := normalizeCommandChannel(yyChainNodeChannel(node, cfg))
	if err != nil {
		return nil, 0, err
	}
	cmd := strings.TrimPrefix(wrapCommandInDimension(node.Command, dim), "/")
	if channel == ChannelAI {
		// Judge on the raw node command: dimension wrapping only adds
		// "execute in <dim> run" and keeps the executor unchanged.
		pk := yyFastSettingsPacket(cmd, name, yyNeedsBotContext(node.Command))
		return pk, len(pk.CommandLine), nil
	}
	origin := uint32(protocol.CommandOriginAutomationPlayer)
	if channel == ChannelPlayer {
		origin = protocol.CommandOriginPlayer
	}
	id := uuid.New()
	return &packet.CommandRequest{CommandLine: cmd, CommandOrigin: protocol.CommandOrigin{Origin: origin, UUID: id, RequestID: id.String()}, Version: 44}, len(cmd), nil
}

// Serial/bounded local submission does not prove server execution or provide server load feedback.
func yyChainFastChunks(ctx context.Context, nodes []yyChainNode, cfg yyChainConfig, dim, name string,
	send func([]packet.Packet) (bool, error),
) ([]map[string]any, bool, error) {
	rows := make([]map[string]any, len(nodes))
	packets := make([]packet.Packet, len(nodes))
	sizes := make([]int, len(nodes))
	for i, node := range nodes {
		rows[i] = map[string]any{"index": i, "submitted": false, "confirmed": false, "channel": yyChainNodeChannel(node, cfg)}
		pk, size, err := yyChainFastPacket(node, cfg, dim, name)
		if err != nil {
			return rows, false, err
		}
		if size > 512*1024 {
			return rows, false, fmt.Errorf("快速模块单条命令超过512KiB，未提交")
		}
		packets[i], sizes[i] = pk, size
	}
	limit := 1024
	if cfg.RateCPS > 0 {
		limit = 1
	}
	for from := 0; from < len(nodes); {
		if err := ctx.Err(); err != nil {
			return rows, false, err
		}
		to, size := from, 0
		for to < len(nodes) && to-from < limit && size+sizes[to] <= 512*1024 {
			size += sizes[to]
			to++
		}
		attempted, err := send(packets[from:to])
		if err != nil {
			if attempted {
				for _, row := range rows[from:to] {
					row["attempted"], row["unknown"] = true, true
				}
			}
			return rows, attempted, fmt.Errorf("快速模块发送失败，没有自动重发：%w", err)
		}
		for _, row := range rows[from:to] {
			row["submitted"] = true
		}
		if err := ctx.Err(); err != nil {
			return rows, false, err
		}
		from = to
	}
	return rows, false, nil
}
