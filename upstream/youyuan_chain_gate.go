package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type yyChainPathKey struct{}

var yyChainGates sync.Map

var yyReadOnlyNegatedGate = regexp.MustCompile(`^execute(?: unless entity @a(?:\[[^\]\r\n]*\])?)+ run testfor @a$`)

func yyChainGateReadOnly(command string) bool {
	command = strings.TrimSpace(strings.TrimPrefix(command, "/"))
	return strings.HasPrefix(command, "testfor ") || yyReadOnlyNegatedGate.MatchString(command)
}

type yyChainGateRuntime struct {
	Owner   string
	Account string
	Path    []string
}

func yyChainGateRows(owner, account string) []map[string]any {
	if account == "" {
		account = "main"
	}
	rows := []map[string]any{}
	yyChainGates.Range(func(k, v any) bool {
		r := v.(yyChainGateRuntime)
		if r.Owner != owner || r.Account != account {
			return true
		}
		in := k.(*yyChainInstance)
		in.Lock()
		hz := float64(0)
		if in.ElapsedMS > 0 {
			hz = float64(in.Cycles) * 1000 / float64(in.ElapsedMS)
		}
		rows = append(rows, map[string]any{"name": in.Config.Name, "path": r.Path, "state": in.State, "cycles": in.Cycles, "submitted": in.Submitted, "actual_hz": hz, "segments": in.Segments, "error": in.LastError})
		in.Unlock()
		return true
	})
	return rows
}

func yyChainCallPath(ctx context.Context, name string) []string {
	if path, ok := ctx.Value(yyChainPathKey{}).([]string); ok {
		return path
	}
	return []string{name}
}

// A gate owns at most one child. A false condition and parent cancellation both join it.
func yyChainRunGate(ctx context.Context, c *yyChainControl, cfg yyChainConfig, path []string,
	lookup func(string, string) (yyChainPlan, yyChainConfig, error),
	single func(context.Context, yyChainNode, yyChainConfig) (bool, error),
	state func(string), record func(bool, bool, error), runner func(*yyChainInstance, context.Context)) error {
	var child *yyChainInstance
	var cancel context.CancelFunc
	active := ""
	waitingForRead := false
	stop := func() {
		if child != nil {
			cancel()
			<-child.done
			yyChainGates.Delete(child)
			child = nil
		}
		active = ""
	}
	defer stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		started := time.Now()
		state("检测条件")
		ok, err := single(ctx, yyChainNode{Command: c.Command, RequireResult: true}, cfg)
		if err != nil {
			stop()
			if ctx.Err() != nil || !yyChainGateReadOnly(c.Command) {
				record(false, false, err)
				return err
			}
			// A read outage is unknown, never a false branch or a replay of a write.
			if !waitingForRead {
				record(false, false, err)
				waitingForRead = true
			}
			state("条件读取暂不可用，等待恢复：" + err.Error())
			if e := yyChainSleep(ctx, time.Second); e != nil {
				return e
			}
			continue
		}
		waitingForRead = false
		record(ok, false, nil)
		name := c.Else
		if ok {
			name = c.Then
		}
		if name != active {
			stop()
			if name != "" {
				if len(path) >= 8 {
					return fmt.Errorf("跨链调用深度不能超过8")
				}
				for _, ancestor := range path {
					if ancestor == name {
						return fmt.Errorf("检测到递归调用 %s", name)
					}
				}
				plan, childCfg, e := lookup(name, cfg.Account)
				if e != nil {
					return e
				}
				if cfg.Hz > 0 {
					if cfg.FrequencyMode == "override" {
						childCfg.Hz, childCfg.FrequencyMode, childCfg.CadenceBaseHz = cfg.Hz, "override", cfg.CadenceBaseHz
					} else if childCfg.Hz == 0 || cfg.Hz < childCfg.Hz {
						childCfg.Hz = cfg.Hz
					}
				}
				if len(plan.Chains) == 1 && len(plan.Chains[0].Nodes) > 0 && plan.Chains[0].Nodes[0].Type == 1 {
					childCfg.RunCount = 1
				}
				childCtx, childCancel := context.WithCancel(ctx)
				childCtx = context.WithValue(childCtx, yyChainPathKey{}, append(append([]string(nil), path...), name))
				cancel = childCancel
				child = &yyChainInstance{Config: childCfg, Plan: plan, Ephemeral: true, StartedAt: time.Now(), cancel: cancel, done: make(chan struct{})}
				owner := cfg.Name
				if len(path) > 0 {
					owner = path[0]
				}
				account := cfg.Account
				if account == "" {
					account = "main"
				}
				yyChainGates.Store(child, yyChainGateRuntime{owner, account, yyChainCallPath(childCtx, name)})
				go runner(child, childCtx)
				active = name
			}
		}
		if child != nil {
			state("条件模块运行 " + active)
			select {
			case <-child.done:
				child.Lock()
				e := child.LastError
				child.Unlock()
				if e != "" {
					return fmt.Errorf("条件模块 %s：%s", active, e)
				}
			default:
			}
		}
		if err := yyChainSleep(ctx, time.Until(started.Add(time.Duration(c.PollMS)*time.Millisecond))); err != nil {
			return err
		}
	}
}

func yyChainInvokePlugin(ctx context.Context, bm *BotManager, c *yyChainControl) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	args := c.Args
	if args == nil {
		args = []any{}
	}
	value, err := yySenseCall(bm, "plugin.call", []any{c.PluginID, c.Function, args})
	if err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if success, ok := value.(bool); ok {
		return success, nil
	}
	return true, nil
}
