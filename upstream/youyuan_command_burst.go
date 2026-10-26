package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/google/uuid"
	"time"
)

// Explicit bounded burst: one protocol batch, then a shared-account cooldown.
// It makes no atomic server-Tick promise. Conditional chain nodes remain barriers.
func yyCommandBurst(bm *BotManager, commands []string, options yyCommandOptions) (any, error) {
	window := options.Window
	if window < 1 {
		window = 1024
	}
	if len(commands) <= window {
		return yyCommandBurstWindow(bm, commands, options)
	}
	start := time.Now()
	rows := []map[string]any{}
	batches := 0
	success, confirmed := true, true
	for from := 0; from < len(commands); from += window {
		to := from + window
		if to > len(commands) {
			to = len(commands)
		}
		value, err := yyCommandBurstWindow(bm, commands[from:to], options)
		if err != nil {
			return nil, err
		}
		result := value.(map[string]any)
		batches++
		if result["success"] != true {
			success = false
		}
		if result["confirmed"] != true {
			confirmed = false
		}
		for _, row := range result["results"].([]map[string]any) {
			row["index"] = len(rows)
			rows = append(rows, row)
		}
	}
	return map[string]any{"mode": "burst", "commands": len(commands), "window": window, "network_batches": batches, "same_tick_guaranteed": false, "elapsed_ms": time.Since(start).Milliseconds(), "results": rows, "success": success, "confirmed": confirmed}, nil
}
func yyCommandBurstWindow(bm *BotManager, commands []string, options yyCommandOptions) (any, error) {
	if options.Transport != "" && options.Transport != "command_request" {
		return nil, fmt.Errorf("burst 仅支持带确认的 command_request")
	}
	total := 0
	for _, cmd := range commands {
		total += len(cmd)
	}
	if total > 512*1024 {
		return nil, fmt.Errorf("单次burst命令文本最多512KiB")
	}
	ctx := options.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if e := bm.waitControlCommandRateContext(ctx, len(commands), 1024, options.RateCPS); e != nil {
		return nil, e
	}
	bm.mu.Lock()
	conn, res, connected, stop := bm.conn, bm.resources, bm.connected, bm.stopCh
	bm.mu.Unlock()
	if !connected || conn == nil || res == nil {
		return nil, fmt.Errorf("命令连接未就绪")
	}
	ids := make([]uuid.UUID, len(commands))
	results := make([]chan yyCommandReply, len(commands))
	packets := make([]packet.Packet, len(commands))
	defer func() {
		for _, id := range ids {
			if id != uuid.Nil {
				res.Commands().DeleteCommandRequestCallback(id)
			}
		}
	}()
	origin := uint32(protocol.CommandOriginAutomationPlayer)
	if options.Channel == "player" {
		origin = protocol.CommandOriginPlayer
	}
	batchID := uuid.New()
	for i, command := range commands {
		id := batchID
		binary.BigEndian.PutUint32(id[12:], uint32(i))
		ids[i] = id
		ch := make(chan yyCommandReply, 1)
		results[i] = ch
		res.Commands().SetCommandRequestCallback(id, func(p *packet.CommandOutput, e error) {
			select {
			case ch <- yyCommandReply{p, e}:
			default:
			}
		})
		packets[i] = &packet.CommandRequest{CommandLine: bm.WrapCommandInDimension(command), CommandOrigin: protocol.CommandOrigin{Origin: origin, UUID: id, RequestID: id.String()}, Version: 44}
	}
	start := time.Now()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	err := bm.writeControlBatch(ctx, func() error { return conn.WritePacketBatch(packets) })
	if err != nil {
		return nil, fmt.Errorf("整批发送失败，执行结果未知，未重发：%w", err)
	}
	deadline := time.Now().Add(time.Duration(options.TimeoutMS) * time.Millisecond)
	rows := make([]map[string]any, len(commands))
	allOK, allConfirmed := true, true
	for i, ch := range results {
		v := map[string]any{"index": i, "command": commands[i], "submitted": true, "confirmed": false, "channel": options.Channel, "transport": "command_request"}
		rows[i] = v
		r, e := yyAwaitCommandReplyContext(ctx, ch, deadline, stop)
		if e == nil {
			e = r.e
		}
		if e != nil || r.p == nil {
			allConfirmed = false
			allOK = false
			v["unknown"] = true
			if e != nil {
				v["error"] = e.Error()
			} else {
				v["error"] = "空命令回包"
			}
			continue
		}
		v["confirmed"] = true
		v["success"] = r.p.SuccessCount > 0
		v["success_count"] = r.p.SuccessCount
		if r.p.SuccessCount == 0 {
			allOK = false
		}
		out := []string{}
		for _, m := range r.p.OutputMessages {
			out = append(out, formatCommandMessage(m.Message, m.Parameters))
		}
		v["output"] = out
	}
	return map[string]any{"mode": "burst", "commands": len(commands), "network_batches": 1, "same_tick_guaranteed": false, "elapsed_ms": time.Since(start).Milliseconds(), "results": rows, "success": allOK, "confirmed": allConfirmed}, nil
}
