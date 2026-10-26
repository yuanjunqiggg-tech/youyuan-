package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/google/uuid"
)

type yyCommandOptions struct {
	Mode      string `json:"mode"`
	Transport string `json:"transport,omitempty"`
	Channel   string `json:"channel"`
	Window    int    `json:"window"`
	TimeoutMS int    `json:"timeout_ms"`
	RateCPS   int    `json:"rate_cps,omitempty"`
	ctx       context.Context
}

type yyCommandReply struct {
	p *packet.CommandOutput
	e error
}

// A later reply may already be buffered while we await an earlier command.
// Consume that reply before considering its deadline or a stopped connection.
func yyAwaitCommandReply(result <-chan yyCommandReply, deadline time.Time, stop <-chan struct{}) (yyCommandReply, error) {
	return yyAwaitCommandReplyContext(context.Background(), result, deadline, stop)
}
func yyAwaitCommandReplyContext(ctx context.Context, result <-chan yyCommandReply, deadline time.Time, stop <-chan struct{}) (yyCommandReply, error) {
	select {
	case r := <-result:
		return r, nil
	default:
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case r := <-result:
		return r, nil
	case <-ctx.Done():
		return yyCommandReply{}, ctx.Err()
	case <-timer.C:
		return yyCommandReply{}, fmt.Errorf("命令响应超时，执行结果未知；没有自动重发")
	case <-stop:
		return yyCommandReply{}, fmt.Errorf("连接停止，执行结果未知")
	}
}

// Dispatch a bounded window in input order before awaiting any of its responses.
// A window is only valid for independent commands; ordered mode uses window=1.
func yyExecuteWindowed(commands []string, window int, submit func(string) (func() (map[string]any, error), error)) []map[string]any {
	rows := make([]map[string]any, len(commands))
	for start := 0; start < len(commands); start += window {
		end := start + window
		if end > len(commands) {
			end = len(commands)
		}
		waits := make([]func() (map[string]any, error), end-start)
		for i := start; i < end; i++ {
			wait, e := submit(commands[i])
			if e != nil {
				rows[i] = map[string]any{"index": i, "command": commands[i], "submitted": false, "confirmed": false, "error": e.Error()}
			} else {
				waits[i-start] = wait
			}
		}
		for i := start; i < end; i++ {
			if wait := waits[i-start]; wait != nil {
				v, e := wait()
				if v == nil {
					v = map[string]any{}
				}
				v["index"] = i
				v["command"] = commands[i]
				if e != nil {
					v["error"] = e.Error()
				}
				rows[i] = v
			}
		}
	}
	return rows
}
func yyCommandBatch(bm *BotManager, args []any) (any, error) {
	return yyCommandBatchDefaults(bm, args, yyDefaultCommandOptions())
}
func yyCommandBatchDefaults(bm *BotManager, args []any, defaults yyCommandOptions) (any, error) {
	return yyCommandBatchDefaultsContext(context.Background(), bm, args, defaults)
}
func yyCommandBatchDefaultsContext(ctx context.Context, bm *BotManager, args []any, defaults yyCommandOptions) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("需要 commands 数组")
	}
	values, ok := args[0].([]any)
	if !ok || len(values) == 0 || len(values) > 4096 {
		return nil, fmt.Errorf("每批需要1..1024条，stream单段最多4096条")
	}
	commands := []string{}
	for _, v := range values {
		text, ok := v.(string)
		text = strings.TrimSpace(strings.TrimPrefix(text, "/"))
		if !ok || text == "" || len(text) > 32768 || strings.ContainsAny(text, "\r\n") {
			return nil, fmt.Errorf("每项需要一条非空命令，不能包含换行")
		}
		commands = append(commands, text)
	}
	options := defaults
	if len(args) > 1 {
		var e error
		options, e = yyMergeCommandOptions(defaults, args[1])
		if e != nil {
			return nil, e
		}
	}
	if options.Mode != "stream" && len(commands) > 1024 {
		return nil, fmt.Errorf("非stream每批最多1024条命令")
	}
	options.ctx = ctx
	if options.Window < 1 || options.Window > 1024 || (options.Mode != "burst" && options.Mode != "stream" && options.Window > 16) || options.TimeoutMS < 100 || options.TimeoutMS > 10000 {
		return nil, fmt.Errorf("pipeline/submit window=1..16，burst/stream=1..1024，timeout_ms=100..10000")
	}
	switch options.Mode {
	case "ordered":
		options.Window = 1
	case "pipeline", "submit", "burst", "stream":
	default:
		return nil, fmt.Errorf("mode 应为 ordered/pipeline/submit/burst/stream")
	}
	if bm == nil {
		return nil, fmt.Errorf("账号未连接")
	}
	if bm.controlBatchPending.Add(1) > 8 {
		bm.controlBatchPending.Add(-1)
		return nil, fmt.Errorf("账号命令调度繁忙：最多8个等待调用，没有继续堆积")
	}
	defer bm.controlBatchPending.Add(-1)
	for !bm.controlBatchMu.TryLock() {
		if e := yyChainSleep(ctx, 5*time.Millisecond); e != nil {
			return nil, e
		}
	}
	defer bm.controlBatchMu.Unlock()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	bm.mu.Lock()
	conn, res, connected, stop := bm.conn, bm.resources, bm.connected, bm.stopCh
	bm.mu.Unlock()
	if !connected || conn == nil {
		return nil, fmt.Errorf("账号未连接")
	}
	if options.Mode != "submit" && res == nil {
		return nil, fmt.Errorf("响应维护器未就绪，命令未发送")
	}
	if options.Mode == "stream" {
		return yyCommandStream(bm, commands, options)
	}
	if options.Mode == "burst" {
		return yyCommandBurst(bm, commands, options)
	}
	submittedIndex := 0
	rows := yyExecuteWindowed(commands, options.Window, func(command string) (func() (map[string]any, error), error) {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		index := submittedIndex
		submittedIndex++
		if index%options.Window == 0 {
			count := options.Window
			if remaining := len(commands) - index; remaining < count {
				count = remaining
			}
			if e := bm.waitControlCommandRateContext(ctx, count, 16, options.RateCPS); e != nil {
				return nil, e
			}
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		command = bm.WrapCommandInDimension(command)
		id := uuid.New()
		base := map[string]any{"submitted": true, "confirmed": false}
		origin := uint32(protocol.CommandOriginAutomationPlayer)
		if options.Channel == "player" {
			origin = protocol.CommandOriginPlayer
		}
		request := &packet.CommandRequest{CommandLine: command, CommandOrigin: protocol.CommandOrigin{Origin: origin, UUID: id, RequestID: uuid.NewString()}, Version: 44}
		base["channel"] = options.Channel
		base["transport"] = options.Transport
		if options.Mode == "submit" {
			var e error
			switch options.Transport {
			case "settings":
				e = conn.WritePacket(&packet.SettingsCommand{CommandLine: command, SuppressOutput: true})
			case "pyrpc":
				e = bm.SendAICommand(command)
			default:
				e = conn.WritePacket(request)
			}
			if e != nil {
				return nil, e
			}
			return func() (map[string]any, error) { return base, nil }, nil
		}
		result := make(chan yyCommandReply, 1)
		res.Commands().SetCommandRequestCallback(id, func(p *packet.CommandOutput, e error) {
			select {
			case result <- yyCommandReply{p, e}:
			default:
			}
		})
		if e := conn.WritePacket(request); e != nil {
			res.Commands().DeleteCommandRequestCallback(id)
			return nil, e
		}
		deadline := time.Now().Add(time.Duration(options.TimeoutMS) * time.Millisecond)
		return func() (map[string]any, error) {
			defer res.Commands().DeleteCommandRequestCallback(id)
			r, err := yyAwaitCommandReplyContext(ctx, result, deadline, stop)
			if err != nil {
				base["unknown"] = true
				return base, err
			}
			if r.e != nil {
				return base, r.e
			}
			if r.p == nil {
				return base, fmt.Errorf("收到空命令响应")
			}
			base["confirmed"] = true
			base["success"] = r.p.SuccessCount > 0
			base["success_count"] = r.p.SuccessCount
			output := []string{}
			for _, m := range r.p.OutputMessages {
				output = append(output, formatCommandMessage(m.Message, m.Parameters))
			}
			base["output"] = output
			return base, nil
		}, nil
	})
	success := true
	confirmed := true
	for _, row := range rows {
		if row["success"] != true {
			success = false
		}
		if row["confirmed"] != true {
			confirmed = false
		}
	}
	return map[string]any{"mode": options.Mode, "transport": options.Transport, "channel": options.Channel, "window": options.Window, "commands": len(commands), "results": rows, "success": success, "confirmed": confirmed}, nil
}
