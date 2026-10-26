package main

import (
	"context"
	"reflect"
	"time"
)

// A worker owns one independent sequential segment, never one command. Workers
// reuse the same graph/Fast/barrier implementation, and have no disk metadata.
// A slow segment cannot reenter itself or block unrelated segments.
func (in *yyChainInstance) runSegments(ctx context.Context) {
	in.runSegmentsWith(ctx, func(w *yyChainInstance, c context.Context) { w.run(c) })
}
func (in *yyChainInstance) runSegmentsWith(ctx context.Context, runner func(*yyChainInstance, context.Context)) {
	in.Lock()
	done := in.done
	in.Unlock()
	workers := []*yyChainInstance{}
	previous := []yyChain{}
	var retiredCycles, retiredSubmitted, retiredExecuted, retiredSkipped uint64
	stopWorker := func(w *yyChainInstance) {
		w.Lock()
		cancel, end := w.cancel, w.done
		w.Unlock()
		if cancel != nil {
			cancel()
		}
		if end != nil {
			<-end
		}
	}
	defer func() {
		for _, w := range workers {
			stopWorker(w)
		}
		in.Lock()
		in.cancel = nil
		in.Config.Enabled = false
		if in.State != "已完成" && in.State != "错误暂停" {
			in.State = "已停止"
		}
		if !in.Ephemeral {
			_ = yyChainSaveConfig(in.Config)
		}
		in.Unlock()
		close(done)
	}()
	makeWorker := func(chain yyChain, cfg yyChainConfig) *yyChainInstance {
		in.Lock()
		redstone := in.Redstone
		in.Unlock()
		cfg.Scheduling = "file"
		if len(chain.Nodes) > 0 && chain.Nodes[0].Type == 1 && !chain.Nodes[0].Redstone {
			cfg.RunCount = 1
		}
		childCtx, cancel := context.WithCancel(ctx)
		commands := len(chain.Nodes)
		w := &yyChainInstance{Config: cfg, Plan: yyChainPlan{Chains: []yyChain{chain}, Commands: commands, Periods: 1, Hz: chain.Hz}, Ephemeral: true, Redstone: redstone, StartedAt: time.Now(), State: "等待连接", cancel: cancel, done: make(chan struct{})}
		go runner(w, childCtx)
		return w
	}
	revision := ^uint64(0)
	for {
		if ctx.Err() != nil {
			return
		}
		in.Lock()
		plan, cfg, rev, trigger, redstone := in.Plan, in.Config, in.Revision, in.Trigger, in.Redstone
		in.Trigger = false
		in.Unlock()
		if !in.Ephemeral {
			cfg.CadenceBaseHz = 0
		}
		cfg = yyChainCadenceConfig(plan, cfg)
		if rev != revision {
			for len(workers) > len(plan.Chains) {
				w := workers[len(workers)-1]
				stopWorker(w)
				w.Lock()
				retiredCycles += w.Cycles
				retiredSubmitted += w.Submitted
				retiredExecuted += w.Executed
				retiredSkipped += w.Skipped
				w.Unlock()
				workers = workers[:len(workers)-1]
			}
			for index, chain := range plan.Chains {
				if index >= len(workers) {
					w := makeWorker(chain, cfg)
					workers = append(workers, w)
					// Optional initialization barrier, only for the leading pulse
					// prefix. Independent non-leading segments remain concurrent.
					leading := cfg.Bootstrap && len(chain.Nodes) > 0 && chain.Nodes[0].Type == 1 && !chain.Nodes[0].Redstone
					for _, prior := range plan.Chains[:index] {
						leading = leading && len(prior.Nodes) > 0 && prior.Nodes[0].Type == 1 && !prior.Nodes[0].Redstone
					}
					if leading {
						select {
						case <-w.done:
						case <-ctx.Done():
							return
						}
					}
					continue
				}
				w := workers[index]
				// A changed segment is rearmed, including a finished pulse. The
				// unchanged siblings keep their clocks and state. Cumulative
				// statistics are retained separately from each activation.
				if index >= len(previous) || !reflect.DeepEqual(previous[index], chain) {
					stopWorker(w)
					w.Lock()
					retiredCycles += w.Cycles
					retiredSubmitted += w.Submitted
					retiredExecuted += w.Executed
					retiredSkipped += w.Skipped
					w.Unlock()
					workers[index] = makeWorker(chain, cfg)
					continue
				}
				w.Lock()
				w.Config.IntervalMS = cfg.IntervalMS
				w.Config.Hz = cfg.Hz
				w.Unlock()
			}
			previous = append([]yyChain(nil), plan.Chains...)
			revision = rev
		}
		rows := make([]map[string]any, 0, len(workers))
		cycles, submitted, executed, skipped := retiredCycles, retiredSubmitted, retiredExecuted, retiredSkipped
		active, failed, waiting := false, false, false
		lastError := ""
		for index, w := range workers {
			w.Lock()
			w.Redstone = redstone
			if trigger {
				w.Trigger = true
			}
			running := w.cancel != nil
			active = active || running
			waiting = waiting || w.State == "等待连接"
			failed = failed || w.State == "错误暂停"
			if w.LastError != "" {
				lastError = w.LastError
			}
			cycles += w.Cycles
			submitted += w.Submitted
			executed += w.Executed
			skipped += w.Skipped
			elapsed := time.Since(w.StartedAt).Seconds()
			hz := float64(0)
			if elapsed > 0 {
				hz = float64(w.Cycles) / elapsed
			}
			targetHz := float64(time.Second) / float64(yyChainPeriod(w.Plan, w.Config))
			kind := 0
			if len(w.Plan.Chains) > 0 && len(w.Plan.Chains[0].Nodes) > 0 {
				kind = w.Plan.Chains[0].Nodes[0].Type
			}
			if kind == 1 {
				targetHz, hz = 0, 0
			}
			rows = append(rows, map[string]any{"segment": index + 1, "type": kind, "source_hz": w.Plan.Hz, "target_hz": targetHz, "actual_hz": hz, "cycles": w.Cycles, "submitted": w.Submitted, "executed": w.Executed, "skipped": w.Skipped, "state": w.State, "error": w.LastError, "last_round_ms": w.LastRoundMS})
			w.Unlock()
		}
		in.Lock()
		in.Segments = rows
		in.Cycles = cycles
		in.Submitted = submitted
		in.Executed = executed
		in.Skipped = skipped
		in.ElapsedMS = time.Since(in.StartedAt).Milliseconds()
		in.LastError = lastError
		switch {
		case !active && failed:
			in.State = "错误暂停"
		case !active:
			in.State = "已完成"
		case failed:
			in.State = "部分段错误暂停"
		case waiting:
			in.State = "等待连接"
		default:
			in.State = "运行中"
		}
		in.Unlock()
		if !active {
			return
		}
		if err := yyChainSleep(ctx, 20*time.Millisecond); err != nil {
			return
		}
	}
}
