package main

import (
	"context"
	"fmt"
	"time"
)

func yyChainValidate(c yyChainConfig) error {
	if c.Mode != "" && c.Mode != "fast" && c.Mode != "remote" && c.Mode != "pipeline" && c.Mode != "burst" && c.Mode != "stream" {
		return fmt.Errorf("命令链运行在机器人内；mode 应为 fast/remote/pipeline/burst/stream")
	}
	if c.Window < 0 || c.Window > 1024 || (c.Mode != "fast" && c.Mode != "burst" && c.Mode != "stream" && c.Window > 16) {
		return fmt.Errorf("流水线 window=1..16，整批 window=1..1024；0 使用默认值")
	}
	if c.RateCPS != 0 && (c.RateCPS < 20 || c.RateCPS > 20000) {
		return fmt.Errorf("rate_cps=0（无固定CPS）或20..20000")
	}
	return nil
}
func yyChainNodeChannel(n yyChainNode, c yyChainConfig) string {
	raw := n.Channel
	if raw == "" {
		raw = c.Channel
	}
	if raw == "" {
		raw = "ai"
	}
	ch, err := normalizeCommandChannel(raw)
	if err != nil {
		return raw
	}
	return string(ch)
}

// A conditional command is a response barrier, including the preceding window.
// Positive per-command delays and different channels also delimit windows.
func yyChainExecuteBuffered(ctx context.Context, plan yyChainPlan, cfg yyChainConfig, delay time.Duration, single func(yyChainNode) (bool, error), batch func([]yyChainNode) ([]map[string]any, error), record func(bool, bool, error)) {
	if cfg.Mode != "fast" && cfg.Mode != "pipeline" && cfg.Mode != "burst" && cfg.Mode != "stream" {
		yyChainExecuteDetailed(ctx, plan, true, true, true, delay, single, record)
		return
	}
	window := cfg.Window
	if window < 1 {
		window = 8
		if cfg.Mode == "burst" {
			window = 1024
		}
	}
	// Stream spans up to 4096 nodes; its response window is managed internally.
	if cfg.Mode == "stream" {
		window = 4096
	}
	if cfg.Mode == "fast" {
		window = int(^uint(0) >> 1) // A whole uninterrupted chain segment; no artificial 1024-node chunking.
	}
	for _, chain := range plan.Chains {
		repeat := chain.Repeat
		if repeat < 1 {
			repeat = 1
		}
		for r := 0; r < repeat; r++ {
			previous := false
			for i := 0; i < len(chain.Nodes); {
				if ctx.Err() != nil {
					return
				}
				n := chain.Nodes[i]
				if n.Conditional && !previous {
					previous = false
					record(false, true, nil)
					i++
					continue
				}
				needsResult := cfg.Mode == "fast" && i+1 < len(chain.Nodes) && chain.Nodes[i+1].Conditional
				if n.Control != nil || n.Conditional || needsResult || delay > 0 || n.DelayMS > 0 || window == 1 {
					n.RequireResult = n.Conditional || needsResult
					ok, err := single(n)
					previous = ok && err == nil
					record(previous, false, err)
					i++
					wait := delay
					if d := time.Duration(n.DelayMS) * time.Millisecond; d > wait {
						wait = d
					}
					if wait > 0 {
						timer := time.NewTimer(wait)
						select {
						case <-ctx.Done():
							timer.Stop()
							return
						case <-timer.C:
						}
					}
					continue
				}
				end := i + 1
				textBytes := len(n.Command)
				channel := yyChainNodeChannel(n, cfg)
				for end < len(chain.Nodes) && end-i < window {
					next := chain.Nodes[end]
					if next.Control != nil || next.Conditional || next.DelayMS > 0 || yyChainNodeChannel(next, cfg) != channel {
						break
					}
					// The transport text bound limits one dispatch, not the TXT.
					// Split long JSON/menu commands before reaching that bound.
					if (cfg.Mode == "stream" || cfg.Mode == "burst") && textBytes+len(next.Command) > 512*1024 {
						break
					}
					textBytes += len(next.Command)
					end++
				}
				// Confirm the immediate predecessor of a conditional node. Earlier
				// unconditional commands still use the no-wait transport.
				if cfg.Mode == "fast" && end < len(chain.Nodes) && chain.Nodes[end].Conditional && end > i+1 {
					end--
				}
				rows, err := batch(chain.Nodes[i:end])
				if ctx.Err() != nil {
					return
				}
				for j := i; j < end; j++ {
					if ctx.Err() != nil {
						return
					}
					outcome := err
					success := false
					if err == nil {
						if j-i >= len(rows) {
							outcome = fmt.Errorf("批量执行缺少回包，结果未知")
						} else {
							row := rows[j-i]
							success = row["confirmed"] == true && row["success"] == true
							if message, ok := row["error"].(string); ok {
								outcome = fmt.Errorf("%s", message)
							} else if cfg.Mode == "fast" && row["submitted"] == true {
								success = true // dispatch status only, not game success
							} else if row["confirmed"] != true {
								outcome = fmt.Errorf("命令结果未知；没有自动重发")
							}
						}
					}
					previous = success && outcome == nil
					notSubmitted := err == nil && j-i < len(rows) && rows[j-i]["submitted"] == false
					record(previous, notSubmitted, outcome)
				}
				i = end
			}
		}
	}
}
