package main

import (
	"context"
	"errors"
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

func (g *yyChainFastGate) writeOutcome(unknown bool, err error) {
	// Local cancellation leaves an ordered prefix queued; it is not a failed transport write.
	if unknown && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		g.poisoned = err
	}
}

type yyChainFastResult struct {
	Submitted  int
	UnknownEnd int
}
type yyChainFastSender struct {
	Send            func(context.Context, []yyChainNode, yyChainConfig) (yyChainFastResult, error)
	RecordSubmitted func(submitted, skipped int)
}

func (bm *BotManager) sendChainFastModule(ctx context.Context, nodes []yyChainNode, cfg yyChainConfig, prepared ...*yyChainFastModules) (yyChainFastResult, error) {
	if bm == nil {
		return yyChainFastResult{}, fmt.Errorf("命令连接未就绪")
	}
	release, err := bm.chainFast.acquire(ctx)
	if err != nil {
		return yyChainFastResult{}, err
	}
	defer release()
	// Read after acquiring the slot: a waiting worker must not rebind a stale connection and clear its poison.
	bm.mu.Lock()
	conn, connected, dim, name := bm.conn, bm.connected, bm.commandDimension, bm.display
	bm.mu.Unlock()
	if !connected || conn == nil {
		return yyChainFastResult{}, fmt.Errorf("命令连接未就绪")
	}
	if err := bm.chainFast.bind(conn); err != nil {
		return yyChainFastResult{}, err
	}
	send := func(packets []packet.Packet) (bool, error) {
		if err := bm.waitControlCommandRateContext(ctx, len(packets), 1024, cfg.RateCPS); err != nil {
			return false, err
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		// shortcut: the transport write has no context deadline; add transport support if writes can stall.
		return conn.WritePacketsBounded(ctx, packets)
	}
	var result yyChainFastResult
	if len(prepared) > 0 && prepared[0] != nil {
		program := prepared[0].module(conn, nodes, cfg)
		result, err = program.submit(ctx, nodes, cfg, dim, name, send)
	} else {
		result, err = yyChainFastSubmit(ctx, nodes, cfg, dim, name, send)
	}
	bm.chainFast.writeOutcome(result.UnknownEnd > result.Submitted, err)
	return result, err
}

// Restore the shared transport's tick coalescing while retaining its 256-packet/256KiB ceiling.
func yyChainFastCoalesced(ctx context.Context, packets []packet.Packet, write func(packet.Packet) error) (bool, error) {
	attempted := false
	for _, p := range packets {
		if err := ctx.Err(); err != nil {
			return attempted, err
		}
		attempted = true
		if err := write(p); err != nil {
			return true, err
		}
	}
	return attempted, nil
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
func yyChainFastSubmit(ctx context.Context, nodes []yyChainNode, cfg yyChainConfig, dim, name string,
	send func([]packet.Packet) (bool, error),
) (result yyChainFastResult, err error) {
	var program yyChainFastPrepared
	return program.submit(ctx, nodes, cfg, dim, name, send)
}

// A prepared span contains immutable SettingsCommand data, never receipts or encrypted bytes.
type yyChainFastPrepared struct {
	nodes              []yyChainNode
	channel, dim, name string
	packets            []packet.Packet
	ends               []int
	reusable           bool
}

func (p *yyChainFastPrepared) prepare(nodes []yyChainNode, cfg yyChainConfig, dim, name string) error {
	// Parsed plans are immutable snapshots; file reload replaces their backing slices.
	if p.reusable && len(nodes) == len(p.nodes) && len(nodes) > 0 && &nodes[0] == &p.nodes[0] &&
		cfg.Channel == p.channel && dim == p.dim && name == p.name {
		return nil
	}
	*p = yyChainFastPrepared{}
	packets := make([]packet.Packet, len(nodes))
	ends := make([]int, 0, (len(nodes)+1023)/1024)
	from, textBytes := 0, 0
	reusable := len(nodes) > 0
	for i, node := range nodes {
		pk, size, err := yyChainFastPacket(node, cfg, dim, name)
		if err != nil {
			return err
		}
		if size > 512*1024 {
			return fmt.Errorf("快速模块单条命令超过512KiB，未提交")
		}
		if i > from && (i-from == 1024 || textBytes+size > 512*1024) {
			ends = append(ends, i)
			from, textBytes = i, 0
		}
		textBytes += size
		packets[i] = pk
		if _, ok := pk.(*packet.SettingsCommand); !ok {
			reusable = false // CommandRequest needs a fresh UUID on every invocation.
		}
	}
	if len(nodes) > 0 {
		ends = append(ends, len(nodes))
	}
	p.nodes, p.channel, p.dim, p.name = nodes, cfg.Channel, dim, name
	p.packets, p.ends, p.reusable = packets, ends, reusable
	return nil
}

func (p *yyChainFastPrepared) submit(ctx context.Context, nodes []yyChainNode, cfg yyChainConfig, dim, name string,
	send func([]packet.Packet) (bool, error),
) (result yyChainFastResult, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := p.prepare(nodes, cfg, dim, name); err != nil {
		return result, err
	}
	for from, chunk := 0, 0; from < len(nodes); chunk++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		to := from + 1
		if cfg.RateCPS == 0 {
			to = p.ends[chunk]
		}
		attempted, err := send(p.packets[from:to])
		if err != nil {
			if attempted {
				result.UnknownEnd = to
			}
			return result, fmt.Errorf("快速模块发送失败，没有自动重发：%w", err)
		}
		result.Submitted = to
		if err := ctx.Err(); err != nil {
			return result, err
		}
		from = to
	}
	return result, nil
}

// Owned by one sequential worker. Entries are spans of current parsed files,
// not per-command status; file reload, root reload and reconnect evict old data.
type yyChainFastSpan struct {
	file  string
	first *yyChainNode
	count int
}
type yyChainFastModules struct {
	root    *yyChainNode
	conn    *minecraft.Conn
	sources map[string]*yyChainNode
	spans   map[yyChainFastSpan]*yyChainFastPrepared
}

func yyChainPlanSource(plan yyChainPlan) *yyChainNode {
	for _, chain := range plan.Chains {
		if len(chain.Nodes) > 0 {
			return &chain.Nodes[0]
		}
	}
	return nil
}
func (p *yyChainFastModules) bindRoot(plan yyChainPlan) {
	source := yyChainPlanSource(plan)
	if p.root != source {
		*p = yyChainFastModules{root: source}
	}
}
func (p *yyChainFastModules) module(conn *minecraft.Conn, nodes []yyChainNode, cfg yyChainConfig) *yyChainFastPrepared {
	if p.conn != conn || p.spans == nil {
		*p = yyChainFastModules{root: p.root, conn: conn, sources: make(map[string]*yyChainNode), spans: make(map[yyChainFastSpan]*yyChainFastPrepared)}
	}
	if source, exists := p.sources[cfg.Name]; !exists || source != cfg.preparedSource {
		for key := range p.spans {
			if key.file == cfg.Name {
				delete(p.spans, key)
			}
		}
		p.sources[cfg.Name] = cfg.preparedSource
	}
	key := yyChainFastSpan{file: cfg.Name, count: len(nodes)}
	if len(nodes) > 0 {
		key.first = &nodes[0]
	}
	program := p.spans[key]
	if program == nil {
		program = &yyChainFastPrepared{}
		p.spans[key] = program
	}
	return program
}
