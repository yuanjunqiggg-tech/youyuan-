package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/google/uuid"
)

type yyStreamCompletion struct {
	index int
	reply yyCommandReply
}

// Replies replenish capacity immediately, even when an earlier reply is late.
// Only this goroutine sends packets; callbacks never send or block the reader.
func yyExecuteSliding(ctx context.Context, commands []string, maximum int, timeout time.Duration, stop <-chan struct{}, flush func() error, submit func(int, string, func(yyCommandReply)) error) ([]map[string]any, int) {
	rows := make([]map[string]any, len(commands))
	completed := make(chan yyStreamCompletion, len(commands))
	deadlines := make(map[int]time.Time)
	sent, pending, peak, good := 0, 0, 0, 0
	window := 16
	if maximum < window {
		window = maximum
	}
	// A short menu can fit in one bounded dispatch. Repeating the 16-command
	// warmup on every 25-command tick adds an unnecessary network round trip.
	// Large functions still start at 16 and grow under acknowledgement pressure.
	if len(commands) <= 32 && len(commands) <= maximum {
		window = len(commands)
	}
	failed := false
	finish := func(index int, reply yyCommandReply, err error) {
		delete(deadlines, index)
		pending--
		row := rows[index]
		if err == nil {
			err = reply.e
		}
		if err != nil || reply.p == nil {
			failed = true
			row["unknown"] = true
			if err != nil {
				row["error"] = err.Error()
			} else {
				row["error"] = "空命令回包"
			}
			return
		}
		row["confirmed"] = true
		row["success"] = reply.p.SuccessCount > 0
		row["success_count"] = reply.p.SuccessCount
		output := []string{}
		for _, m := range reply.p.OutputMessages {
			output = append(output, formatCommandMessage(m.Message, m.Parameters))
		}
		row["output"] = output
		// A normal negative command result (for example an empty selector) is
		// still a healthy transport acknowledgement. Conditional dependencies
		// are handled by the chain runner; an unconditional tail must continue.
		good++
		if good >= window && window < maximum {
			good = 0
			window *= 2
			if window > maximum {
				window = maximum
			}
		}
	}
	for sent < len(commands) || pending > 0 {
		firstNew := sent
		for !failed && ctx.Err() == nil && sent < len(commands) && pending < window {
			i := sent
			rows[i] = map[string]any{"index": i, "command": commands[i], "submitted": false, "confirmed": false}
			// The deadline includes write time, not time spent in the unsent queue.
			deadline := time.Now().Add(timeout)
			err := submit(i, commands[i], func(r yyCommandReply) { completed <- yyStreamCompletion{i, r} })
			sent++
			if err != nil {
				rows[i]["error"] = err.Error()
				failed = true
				break
			}
			rows[i]["submitted"] = true
			deadlines[i] = deadline
			pending++
			if pending > peak {
				peak = pending
			}
		}
		if sent > firstNew && flush != nil {
			if err := flush(); err != nil {
				failed = true
				for i := firstNew; i < sent; i++ {
					if _, exists := deadlines[i]; exists {
						finish(i, yyCommandReply{}, fmt.Errorf("窗口发送失败，结果未知，未重发：%w", err))
					}
				}
			} else {
				deadline := time.Now().Add(timeout)
				for i := firstNew; i < sent; i++ {
					if _, exists := deadlines[i]; exists {
						deadlines[i] = deadline
					}
				}
			}
		}
		if pending == 0 {
			break
		}
		var nearest time.Time
		for _, d := range deadlines {
			if nearest.IsZero() || d.Before(nearest) {
				nearest = d
			}
		}
		timer := time.NewTimer(time.Until(nearest))
		select {
		case item := <-completed:
			timer.Stop()
			if _, exists := deadlines[item.index]; exists {
				finish(item.index, item.reply, nil)
			}
			// Coalesce already available replies before flushing the refill, without
			// waiting for the entire window or adding a timer-based delay.
			draining := true
			for draining {
				select {
				case item := <-completed:
					if _, exists := deadlines[item.index]; exists {
						finish(item.index, item.reply, nil)
					}
				default:
					draining = false
				}
			}
		case <-timer.C:
			// Prefer replies already buffered at the deadline over a false timeout.
			draining := true
			for draining {
				select {
				case item := <-completed:
					if _, exists := deadlines[item.index]; exists {
						finish(item.index, item.reply, nil)
					}
				default:
					draining = false
				}
			}
			now := time.Now()
			for i, d := range deadlines {
				if !d.After(now) {
					finish(i, yyCommandReply{}, fmt.Errorf("命令回包超时，结果未知，未重发"))
				}
			}
		case <-ctx.Done():
			timer.Stop()
			failed = true
			for i := range deadlines {
				finish(i, yyCommandReply{}, ctx.Err())
			}
		case <-stop:
			timer.Stop()
			failed = true
			for i := range deadlines {
				finish(i, yyCommandReply{}, fmt.Errorf("连接停止，结果未知，未重发"))
			}
		}
	}
	for i := sent; i < len(commands); i++ {
		rows[i] = map[string]any{"index": i, "command": commands[i], "submitted": false, "confirmed": false, "error": "前序失败或取消，未继续发送"}
	}
	return rows, peak
}

func yyCommandStream(bm *BotManager, commands []string, options yyCommandOptions) (any, error) {
	total := 0
	for _, command := range commands {
		total += len(command)
	}
	if total > 512*1024 {
		return nil, fmt.Errorf("滑动窗口单段命令文本最多512KiB")
	}
	bm.mu.Lock()
	conn, res, connected, stop := bm.conn, bm.resources, bm.connected, bm.stopCh
	bm.mu.Unlock()
	if !connected || conn == nil || res == nil {
		return nil, fmt.Errorf("命令连接未就绪")
	}
	ctx := options.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ids := []uuid.UUID{}
	defer func() {
		for _, id := range ids {
			res.Commands().DeleteCommandRequestCallback(id)
		}
	}()
	origin := uint32(protocol.CommandOriginAutomationPlayer)
	if options.Channel == "player" {
		origin = protocol.CommandOriginPlayer
	}
	started := time.Now()
	packets := []packet.Packet{}
	batches := 0
	flush := func() error {
		if len(packets) == 0 {
			return nil
		}
		if options.RateCPS > 0 {
			if err := bm.waitControlCommandRateContext(ctx, len(packets), 1024, options.RateCPS); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		err := conn.WritePacketBatch(packets)
		if err == nil {
			bm.controlCommands.Add(uint64(len(packets)))
			batches++
		}
		for i := range packets {
			packets[i] = nil
		}
		packets = packets[:0]
		return err
	}
	rows, peak := yyExecuteSliding(ctx, commands, options.Window, time.Duration(options.TimeoutMS)*time.Millisecond, stop, flush, func(i int, command string, done func(yyCommandReply)) error {
		id := uuid.New()
		ids = append(ids, id)
		var once sync.Once
		res.Commands().SetCommandRequestCallback(id, func(p *packet.CommandOutput, err error) { once.Do(func() { done(yyCommandReply{p, err}) }) })
		packets = append(packets, &packet.CommandRequest{CommandLine: bm.WrapCommandInDimension(command), CommandOrigin: protocol.CommandOrigin{Origin: origin, UUID: id, RequestID: id.String()}, Version: 44})
		return nil
	})
	success, confirmed := true, true
	for _, row := range rows {
		row["channel"] = options.Channel
		row["transport"] = "command_request"
		if row["success"] != true {
			success = false
		}
		if row["confirmed"] != true {
			confirmed = false
		}
	}
	return map[string]any{"mode": "stream", "commands": len(commands), "window": options.Window, "peak_inflight": peak, "network_batches": batches, "rate_cps": options.RateCPS, "same_tick_guaranteed": false, "elapsed_ms": time.Since(started).Milliseconds(), "results": rows, "success": success, "confirmed": confirmed}, nil
}
