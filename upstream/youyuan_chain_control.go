package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Controls are parsed once and are response barriers, never Minecraft commands.
type yyChainControl struct {
	Kind       string `json:"kind"`
	Name       string `json:"name,omitempty"`
	FromLine   int    `json:"from_line,omitempty"`
	ToLine     int    `json:"to_line,omitempty"`
	Count      *int   `json:"count,omitempty"`
	DurationMS int    `json:"duration_ms,omitempty"`
	KeepOpen   bool   `json:"keep_open,omitempty"`
	Command    string `json:"command,omitempty"`
	PollMS     int    `json:"poll_ms,omitempty"`
	Edge       *bool  `json:"edge,omitempty"`
	Then       string `json:"then,omitempty"`
	Else       string `json:"else,omitempty"`
}

func yyParseChainControl(line string) (*yyChainControl, error) {
	kind, text, _ := strings.Cut(line, " ")
	c := &yyChainControl{}
	if kind != "@call" && kind != "@wait" && kind != "@pause" && kind != "@branch" && kind != "@frame" {
		return nil, fmt.Errorf("控制语法为 @call/@wait/@pause/@branch/@frame")
	}
	if kind == "@pause" && strings.TrimSpace(text) == "" {
		c.Kind = "pause"
		return c, nil
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(c); err != nil {
		return nil, fmt.Errorf("%s 后需要 JSON 对象：%w", kind, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("控制行只能有一个 JSON 对象")
	}
	c.Kind = strings.TrimPrefix(kind, "@")
	if c.Kind == "frame" {
		if c.DurationMS < 1 || c.DurationMS > 60000 {
			return nil, fmt.Errorf("帧间隔duration_ms需1..60000")
		}
		return c, nil
	}
	if c.Kind == "call" {
		if _, err := yyChainName(c.Name); err != nil {
			return nil, err
		}
		if c.FromLine < 0 || c.ToLine < 0 || c.ToLine > 0 && c.ToLine < c.FromLine {
			return nil, fmt.Errorf("from_line/to_line 为源文件行号，范围无效")
		}
		if c.Count != nil && (*c.Count < 0 || *c.Count > 1000000) {
			return nil, fmt.Errorf("count 为 0..1000000")
		}
		if c.DurationMS < 0 || c.DurationMS > 86400000 {
			return nil, fmt.Errorf("duration_ms 为 0..86400000")
		}
		if c.Count != nil && *c.Count == 0 && !c.KeepOpen && c.DurationMS == 0 {
			return nil, fmt.Errorf("count=0 必须指定 duration_ms 或 keep_open=true")
		}
	} else if c.Kind == "wait" || c.Kind == "branch" {
		c.Command = strings.TrimSpace(strings.TrimPrefix(c.Command, "/"))
		if c.Command == "" || len(c.Command) > 32768 || strings.ContainsAny(c.Command, "\r\n") {
			return nil, fmt.Errorf("%s 需要一条条件检测命令", kind)
		}
		if c.Kind == "branch" {
			if c.Then == "" && c.Else == "" {
				return nil, fmt.Errorf("@branch 至少指定 then 或 else 链名")
			}
			for _, name := range []string{c.Then, c.Else} {
				if name != "" {
					if _, err := yyChainName(name); err != nil {
						return nil, err
					}
				}
			}
			return c, nil
		}
		if c.PollMS == 0 {
			c.PollMS = 500
		}
		if c.PollMS < 100 || c.PollMS > 60000 {
			return nil, fmt.Errorf("poll_ms 为 100..60000")
		}
	}
	return c, nil
}

func yyChainSleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d <= time.Millisecond {
		yyPrecisionWait(d)
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func yyChainLookup(name, account string) (yyChainPlan, yyChainConfig, error) {
	yyChains.Lock()
	in := yyChains.items[name]
	yyChains.Unlock()
	if in == nil {
		return yyChainPlan{}, yyChainConfig{}, fmt.Errorf("目标链 %q 不存在", name)
	}
	in.Lock()
	plan, cfg := in.Plan, in.Config
	in.Unlock()
	owner := cfg.Account
	if owner == "" {
		owner = "main"
	}
	if account == "" {
		account = "main"
	}
	if owner != account {
		return yyChainPlan{}, cfg, fmt.Errorf("不能调用其他账号的命令链")
	}
	return plan, cfg, nil
}
func yyChainSlice(plan yyChainPlan, from, to int) (yyChainPlan, error) {
	out := yyChainPlan{Periods: plan.Periods, Hz: plan.Hz, Segmented: plan.Segmented}
	for _, chain := range plan.Chains {
		part := yyChain{Period: chain.Period, Repeat: chain.Repeat}
		for _, n := range chain.Nodes {
			if (from == 0 || n.Line >= from) && (to == 0 || n.Line <= to) {
				part.Nodes = append(part.Nodes, n)
				out.Commands++
			}
		}
		if len(part.Nodes) > 0 {
			out.Chains = append(out.Chains, part)
		}
	}
	if out.Commands == 0 {
		return out, fmt.Errorf("选定源文件行号范围没有可执行内容")
	}
	return out, nil
}

// One parent stack owns children: no detached goroutines or expanding task queue.
// Calls snapshot the parsed target and never start its independent resident task.
func yyChainExecuteGraph(ctx context.Context, plan yyChainPlan, cfg yyChainConfig, path []string,
	lookup func(string, string) (yyChainPlan, yyChainConfig, error),
	single func(context.Context, yyChainNode, yyChainConfig) (bool, error),
	batch func(context.Context, []yyChainNode, yyChainConfig) ([]map[string]any, error),
	trigger func() bool, state func(string), record func(bool, bool, error)) error {
	var fatal error
	ctx, cancelGraph := context.WithCancel(ctx)
	defer cancelGraph()
	frameStarted := time.Now()
	controlSingle := func(n yyChainNode) (bool, error) {
		if fatal != nil {
			return false, fatal
		}
		if n.Control == nil {
			return single(ctx, n, cfg)
		}
		c := n.Control
		if c.Kind == "branch" {
			test := n
			test.Control, test.Command, test.RequireResult = nil, c.Command, true
			ok, err := single(ctx, test, cfg)
			record(ok, false, err)
			if err != nil {
				return false, err
			}
			name := c.Else
			if ok {
				name = c.Then
			}
			if name == "" {
				return ok, nil
			}
			c = &yyChainControl{Kind: "call", Name: name}
		}
		switch c.Kind {
		case "frame":
			// Drawing commands were submitted through the same Fast buffer.
			// Wait once per frame, including time spent sending it. No server
			// testfor response, per-block sleep, or accumulating overdue frames.
			if e := yyChainSleep(ctx, time.Until(frameStarted.Add(time.Duration(c.DurationMS)*time.Millisecond))); e != nil {
				return false, e
			}
			frameStarted = time.Now()
			return true, nil
		case "pause":
			state("等待手动触发")
			for !trigger() {
				if e := yyChainSleep(ctx, 75*time.Millisecond); e != nil {
					return false, e
				}
			}
			state("运行中")
			return true, nil
		case "wait":
			state("等待条件")
			armed := c.Edge != nil && !*c.Edge
			for {
				test := n
				test.Control = nil
				test.Command = c.Command
				test.RequireResult = true
				ok, err := single(ctx, test, cfg)
				record(ok, false, err)
				if err != nil {
					return false, err
				}
				if !ok {
					armed = true
				}
				if ok && armed {
					state("运行中")
					return true, nil
				}
				if e := yyChainSleep(ctx, time.Duration(c.PollMS)*time.Millisecond); e != nil {
					return false, e
				}
			}
		case "call":
			if len(path) >= 8 {
				fatal = fmt.Errorf("跨链调用深度不能超过8")
				return false, fatal
			}
			for _, name := range path {
				if name == c.Name {
					fatal = fmt.Errorf("检测到递归调用 %s；自闭链请使用 count/keep_open，不递归创建任务", c.Name)
					return false, fatal
				}
			}
			child, childCfg, err := lookup(c.Name, cfg.Account)
			if err != nil {
				fatal = err
				return false, err
			}
			child, err = yyChainSlice(child, c.FromLine, c.ToLine)
			if err != nil {
				fatal = err
				return false, err
			}
			childCtx, cancel := context.WithCancel(ctx)
			if c.DurationMS > 0 {
				cancel()
				childCtx, cancel = context.WithTimeout(ctx, time.Duration(c.DurationMS)*time.Millisecond)
			}
			defer cancel()
			count := 1
			if c.KeepOpen {
				count = 0
			}
			if c.Count != nil {
				count = *c.Count
			}
			nextPath := append(append([]string(nil), path...), c.Name)
			state("调用 " + c.Name)
			for i := 0; count == 0 || i < count; i++ {
				roundStarted := time.Now()
				err = yyChainExecuteGraph(childCtx, child, childCfg, nextPath, lookup, single, batch, trigger, state, record)
				if err != nil || childCtx.Err() != nil {
					if ctx.Err() != nil {
						return false, ctx.Err()
					}
					if c.DurationMS > 0 && childCtx.Err() == context.DeadlineExceeded {
						state("运行中")
						return true, nil
					}
					fatal = err
					return false, err
				}
				if count != 0 && i+1 >= count {
					break
				}
				if err = yyChainSleep(childCtx, yyChainRoundWait(yyChainPeriod(child, childCfg), time.Since(roundStarted))); err != nil {
					if ctx.Err() != nil {
						return false, ctx.Err()
					}
					if c.DurationMS > 0 && err == context.DeadlineExceeded {
						state("运行中")
						return true, nil
					}
					return false, err
				}
			}
			state("运行中")
			return true, nil
		}
		return false, fmt.Errorf("未知控制节点")
	}
	wrappedSingle := func(n yyChainNode) (bool, error) {
		ok, err := controlSingle(n)
		if n.Control != nil && err != nil {
			fatal = err
			cancelGraph()
		}
		return ok, err
	}
	yyChainExecuteBuffered(ctx, plan, cfg, time.Duration(cfg.DelayMS)*time.Millisecond, wrappedSingle,
		func(nodes []yyChainNode) ([]map[string]any, error) {
			if fatal != nil {
				return nil, fatal
			}
			return batch(ctx, nodes, cfg)
		},
		func(ok, skip bool, e error) { record(ok, skip, e) })
	if fatal != nil {
		return fatal
	}
	return ctx.Err()
}
