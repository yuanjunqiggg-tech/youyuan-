package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 只有引号/JSON 外、下一段为 / 或输入末尾的点才是终端分隔符。
func yySplitConsole(input string) []string {
	text := strings.TrimSpace(input)
	if !strings.HasPrefix(text, "/") {
		return []string{text}
	}
	var out []string
	start := 0
	quote := byte(0)
	escaped := false
	depth := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '{' || c == '[' {
			depth++
		}
		if c == '}' || c == ']' {
			depth--
		}
		if c != '.' || depth > 0 {
			continue
		}
		tail := strings.TrimSpace(text[i+1:])
		if tail == "" || strings.HasPrefix(tail, "/") {
			piece := strings.TrimSpace(text[start:i])
			if piece != "" {
				out = append(out, piece)
			}
			start = i + 1
		}
	}
	if rest := strings.TrimSpace(text[start:]); rest != "" {
		out = append(out, rest)
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}
func (bm *BotManager) SendConsole(input string) (map[string]any, error) {
	commands := yySplitConsole(input)
	if len(commands) == 1 {
		return bm.sendConsoleOne(commands[0])
	}
	results := []map[string]any{}
	output := []string{}
	success := true
	for _, cmd := range commands {
		r, e := bm.sendConsoleOne(cmd)
		if e != nil {
			return map[string]any{"ok": false, "results": results, "error": e.Error()}, e
		}
		results = append(results, r)
		if r["success"] != true {
			success = false
		}
		if lines, ok := r["output"].([]string); ok {
			output = append(output, lines...)
		}
	}
	return map[string]any{"ok": true, "success": success, "results": results, "output": output, "commands": len(commands)}, nil
}

type yyChainNode struct {
	Type          int             `json:"type"`
	Conditional   bool            `json:"conditional"`
	Redstone      bool            `json:"redstone"`
	Command       string          `json:"command"`
	Line          int             `json:"line"`
	Channel       string          `json:"channel,omitempty"`
	DelayMS       int             `json:"delay_ms,omitempty"`
	Group         string          `json:"group,omitempty"`
	Control       *yyChainControl `json:"control,omitempty"`
	RequireResult bool            `json:"-"` // Internal dependency barrier, never inferred from submission alone.
}
type yyChain struct {
	Nodes  []yyChainNode `json:"nodes"`
	Period int           `json:"period"`
	Repeat int           `json:"repeat,omitempty"`
	Hz     float64       `json:"hz,omitempty"`
}
type yyChainPlan struct {
	Chains    []yyChain `json:"chains"`
	Periods   int       `json:"periods"`
	Commands  int       `json:"commands"`
	Hz        float64   `json:"hz,omitempty"`
	Segmented bool      `json:"segmented,omitempty"`
}

var yyChainHeader = regexp.MustCompile(`^([012])\s*,\s*([01])\s*,\s*([01])(?:\s+([0-9]+(?:\.[0-9]+)?))?$`)

func yyParseChains(text string) (yyChainPlan, error) {
	plan := yyChainPlan{}
	state := yyChainNode{Type: 1}
	chain := yyChain{Repeat: 1}
	period := 0
	group := defaultGroupConfig()
	groupName := ""
	parsed := parseMcfunction(text)
	tests := make(map[string]TestDef, len(parsed.Tests))
	for _, test := range parsed.Tests {
		tests[test.Name] = test
	}
	inTest := false
	currentHz := float64(0)
	commonHz := float64(0)
	mixedHz := false
	flush := func() {
		if len(chain.Nodes) > 0 {
			chain.Period = period
			plan.Chains = append(plan.Chains, chain)
			chain = yyChain{Repeat: group.LoopCount}
		}
	}
	appendCommand := func(command string, line int) {
		if len(chain.Nodes) == 0 {
			chain.Hz = currentHz
		}
		node := state
		node.Command = strings.TrimPrefix(command, "/")
		if group.Prefix != "" {
			node.Command = group.Prefix + " " + node.Command
		}
		node.Line = line
		node.Channel = group.Channel
		node.DelayMS = group.Speed
		node.Group = groupName
		chain.Nodes = append(chain.Nodes, node)
		plan.Commands++
	}
	for index, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if m := yyChainGroupHeader.FindStringSubmatch(line); m != nil {
				flush()
				groupName, group = parseGroupConfig(strings.TrimSpace(m[1]))
				if group.LoopCount > 1000 {
					return plan, fmt.Errorf("第 %d 行：组循环次数不能超过 1000", index+1)
				}
				chain.Repeat = group.LoopCount
				inTest = false
				continue
			}
			if yyChainTestHeader.MatchString(line) {
				inTest = true
				continue
			}
			if m := yyChainExecHeader.FindStringSubmatch(line); m != nil {
				inTest = false
				flush()
				state = yyChainNode{Type: 1}
				name, params := parseExecParams(strings.TrimSpace(m[1]))
				def, ok := tests[name]
				if !ok {
					return plan, fmt.Errorf("第 %d 行：找不到测试 %q", index+1, name)
				}
				for _, cmd := range def.Commands {
					if cmd == "." || cmd == "!" || yyChainHeader.MatchString(cmd) {
						continue
					}
					for key, value := range def.Params {
						if _, exists := params[key]; !exists {
							params[key] = value
						}
					}
					for key, value := range params {
						cmd = strings.ReplaceAll(cmd, "≤"+key+"≥", value)
					}
					if strings.Contains(cmd, "≤") {
						return plan, fmt.Errorf("第 %d 行：测试 %q 有未填写的参数", index+1, name)
					}
					appendCommand(cmd, index+1)
				}
				continue
			}
			continue
		}
		if inTest {
			continue
		}
		if strings.HasPrefix(line, "@") {
			control, err := yyParseChainControl(line)
			if err != nil {
				return plan, fmt.Errorf("第 %d 行：%w", index+1, err)
			}
			node := state
			node.Command, node.Line, node.Control = line, index+1, control
			if len(chain.Nodes) == 0 {
				chain.Hz = currentHz
			}
			chain.Nodes = append(chain.Nodes, node)
			plan.Commands++
			continue
		}
		if line == "." {
			flush()
			continue
		}
		if line == "!" {
			flush()
			period++
			continue
		}
		if m := yyChainHeader.FindStringSubmatch(line); m != nil {
			if m[4] != "" {
				hz, err := strconv.ParseFloat(m[4], 64)
				_, durationErr := yyScheduleDuration("util.scheduleHz", hz)
				if err != nil || durationErr != nil || m[1] != "2" {
					return plan, fmt.Errorf("第%d行：循环头Hz需有限正数且可表示为宿主周期", index+1)
				}
				if len(chain.Nodes) > 0 && chain.Hz != hz {
					return plan, fmt.Errorf("第%d行：更改段频率前用 . 分隔独立链", index+1)
				}
				currentHz = hz
				plan.Segmented = true
				if commonHz == 0 {
					commonHz = hz
				} else if commonHz != hz {
					mixedHz = true
				}
				if !mixedHz {
					plan.Hz = commonHz
				} else {
					plan.Hz = 0
				}
			}
			if m[1] == "1" {
				currentHz = 0
			}
			state.Type, _ = strconv.Atoi(m[1])
			state.Conditional = m[2] == "1"
			state.Redstone = m[3] == "1"
			continue
		}
		if regexp.MustCompile(`^\d\s*,\s*\d\s*,`).MatchString(line) {
			return plan, fmt.Errorf("第 %d 行：属性应为 类型(0/1/2),条件(0/1),红石(0/1)", index+1)
		}
		appendCommand(line, index+1)
	}
	flush()
	if plan.Commands == 0 {
		return plan, fmt.Errorf("文件没有命令")
	}
	plan.Periods = plan.Chains[len(plan.Chains)-1].Period + 1
	return plan, nil
}

var yyChainGroupHeader = regexp.MustCompile(`^#\s*(?:组|群|分区|类别)[：: 　]\s*(.+)$`)
var yyChainTestHeader = regexp.MustCompile(`^#\s*(?:定义|测试|规定)[：: 　]\s*(.+)$`)
var yyChainExecHeader = regexp.MustCompile(`^#\s*(?:执行|运行)[：: 　]\s*(.+)$`)

type yyChainConfig struct {
	Name       string  `json:"name"`
	Account    string  `json:"account"`
	IntervalMS int     `json:"interval_ms"`
	Hz         float64 `json:"hz,omitempty"` // Explicit cadence ceiling; slower TXT segments retain their own periods.
	DelayMS    int     `json:"delay_ms"`
	Channel    string  `json:"channel,omitempty"`
	RunCount   int     `json:"run_count"` // 0 means continuous; positive values count complete file rounds.
	Enabled    bool    `json:"enabled"`
	Mode       string  `json:"mode,omitempty"`
	Window     int     `json:"window,omitempty"`
	RateCPS    int     `json:"rate_cps,omitempty"`
	Scheduling string  `json:"scheduling,omitempty"` // auto, file, segments
	Bootstrap  bool    `json:"bootstrap,omitempty"`  // Leading pulse segments finish before loop workers are started.
}
type yyChainInstance struct {
	sync.Mutex
	Config      yyChainConfig
	Plan        yyChainPlan
	State       string
	Cycles      uint64
	Executed    uint64
	Skipped     uint64
	LastError   string
	Redstone    bool
	Trigger     bool
	Submitted   uint64
	StartedAt   time.Time
	ElapsedMS   int64
	LastRoundMS int64
	cancel      context.CancelFunc
	done        chan struct{}
	Ephemeral   bool
	Revision    uint64
	Segments    []map[string]any
}

var yyChainRoot string

// Serialise HTTP mutations without holding the map lock while waiting for a runner.
var yyChainOpMu sync.Mutex
var yyChains = struct {
	sync.Mutex
	items map[string]*yyChainInstance
}{items: map[string]*yyChainInstance{}}

func yyChainName(name string) (string, error) {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\:") || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("文件名无效")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".txt" && ext != ".mcfunction" {
		return "", fmt.Errorf("支持 TXT 和 mcfunction 文件")
	}
	return name, nil
}
func yyChainSaveConfig(c yyChainConfig) error {
	data, _ := json.Marshal(c)
	return os.WriteFile(filepath.Join(yyChainRoot, c.Name+".json"), data, 0600)
}
func yyChainExecute(ctx context.Context, plan yyChainPlan, first, trigger, redstone bool, delay time.Duration, send func(string) (bool, error), record func(bool, bool, error)) {
	yyChainExecuteDetailed(ctx, plan, first, trigger, redstone, delay, func(node yyChainNode) (bool, error) { return send(node.Command) }, record)
}
func yyChainExecuteDetailed(ctx context.Context, plan yyChainPlan, first, trigger, redstone bool, delay time.Duration, send func(yyChainNode) (bool, error), record func(bool, bool, error)) {
	for _, chain := range plan.Chains {
		if len(chain.Nodes) == 0 {
			continue
		}
		head := chain.Nodes[0]
		if head.Type != 2 && !first && !trigger {
			continue
		}
		repeat := chain.Repeat
		if repeat < 1 {
			repeat = 1
		}
		for iteration := 0; iteration < repeat; iteration++ {
			previous := false
			for _, node := range chain.Nodes {
				if ctx.Err() != nil {
					return
				}
				if node.Redstone && !redstone || node.Conditional && !previous {
					previous = false
					record(false, true, nil)
					continue
				}
				ok, e := send(node)
				previous = ok && e == nil
				record(previous, false, e)
				wait := delay
				if node.DelayMS > 0 {
					wait = time.Duration(node.DelayMS) * time.Millisecond
				}
				if wait < delay {
					wait = delay
				}
				if wait > 0 {
					select {
					case <-ctx.Done():
						return
					case <-time.After(wait):
					}
				}
			}
		}
	}
}
func yyChainStart(instance *yyChainInstance) error {
	instance.Lock()
	defer instance.Unlock()
	if instance.cancel != nil {
		return nil
	}
	if e := yyChainValidate(instance.Config); e != nil {
		return e
	}
	ctx, cancel := context.WithCancel(context.Background())
	instance.cancel = cancel
	instance.done = make(chan struct{})
	instance.State = "等待连接"
	instance.Cycles = 0
	instance.Submitted = 0
	instance.Executed = 0
	instance.Skipped = 0
	instance.StartedAt = time.Now()
	instance.ElapsedMS = 0
	instance.LastRoundMS = 0
	instance.LastError = ""
	instance.Config.Enabled = true
	if e := yyChainSaveConfig(instance.Config); e != nil {
		instance.cancel = nil
		cancel()
		return e
	}
	go instance.run(ctx)
	return nil
}
func (in *yyChainInstance) run(ctx context.Context) {
	in.Lock()
	segmented := in.Config.Scheduling == "segments" || in.Config.Scheduling != "file" && in.Plan.Segmented
	in.Unlock()
	if segmented && !in.Ephemeral {
		in.runSegments(ctx)
		return
	}
	in.Lock()
	done := in.done
	in.Unlock()
	defer func() {
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
	failures := 0
	for {
		roundStarted := time.Now()
		in.Lock()
		cfg := in.Config
		plan := in.Plan
		in.Unlock()
		bm := getBotManagerLocked()
		if cfg.Account != "" && cfg.Account != "main" {
			bm = yyAccountBot(cfg.Account)
		}
		if bm == nil || !bm.IsConnected() {
			in.Lock()
			in.State = "等待连接"
			in.Unlock()
		} else {
			in.Lock()
			in.State = "运行中"
			in.Unlock()
			roundCtx, stopRound := context.WithCancel(ctx)
			fastModule := func(callCtx context.Context, nodes []yyChainNode, callCfg yyChainConfig) ([]map[string]any, error) {
				rows, err := bm.sendChainFastModule(callCtx, nodes, callCfg)
				for i, row := range rows {
					if row["submitted"] == true {
						bm.yyChatSubmitted(nodes[i].Command)
					}
				}
				in.Lock()
				for _, row := range rows {
					if row["submitted"] == true {
						in.Submitted++
					}
				}
				if err != nil && ctx.Err() == nil && !yyChainFastInvocationEnded(callCtx, err, rows) {
					in.LastError = err.Error()
					failures = 3
					// A failed module write may still execute remotely. Stop on the
					// first failure instead of replaying short segments next round.
					stopRound()
				}
				in.Unlock()
				return rows, err
			}
			graphErr := yyChainExecuteGraph(roundCtx, plan, cfg, []string{cfg.Name}, yyChainLookup, func(callCtx context.Context, node yyChainNode, callCfg yyChainConfig) (bool, error) {
				if failures >= 3 {
					return false, fmt.Errorf("连续 3 次响应失败，当前轮已停止")
				}
				channel := node.Channel
				if channel == "" {
					channel = callCfg.Channel
				}
				if channel == "" {
					channel = "ai"
				}
				ch, e := normalizeCommandChannel(channel)
				if e != nil {
					return false, e
				}
				if callCfg.Mode == "fast" && !node.RequireResult {
					_, e := fastModule(callCtx, []yyChainNode{node}, callCfg)
					return e == nil, e
				}
				value, e := yyCommandBatchDefaultsContext(callCtx, bm, []any{[]any{node.Command}}, yyCommandOptions{Mode: "ordered", Channel: string(ch), Window: 1, TimeoutMS: 8000})
				if e != nil {
					return false, e
				}
				row := value.(map[string]any)["results"].([]map[string]any)[0]
				if row["submitted"] == true {
					in.Lock()
					in.Submitted++
					in.Unlock()
				}
				if row["confirmed"] != true {
					if callCtx.Err() != nil {
						return false, callCtx.Err()
					}
					return false, fmt.Errorf("未收到命令成功响应：%v", row["error"])
				}
				return row["success"] == true, nil
			}, func(callCtx context.Context, nodes []yyChainNode, callCfg yyChainConfig) ([]map[string]any, error) {
				if callCfg.Mode == "fast" {
					return fastModule(callCtx, nodes, callCfg)
				}
				values := make([]any, len(nodes))
				for i, n := range nodes {
					values[i] = n.Command
				}
				channel := yyChainNodeChannel(nodes[0], callCfg)
				mode := "pipeline"
				if callCfg.Mode == "burst" {
					mode = "burst"
				}
				window := len(nodes)
				if callCfg.Mode == "stream" {
					mode = "stream"
					window = callCfg.Window
					if window < 1 {
						window = 128
					}
				}
				value, err := yyCommandBatchDefaultsContext(callCtx, bm, []any{values}, yyCommandOptions{Mode: mode, Channel: channel, Window: window, TimeoutMS: 8000, RateCPS: callCfg.RateCPS})
				if err != nil {
					return nil, err
				}
				result := value.(map[string]any)["results"].([]map[string]any)
				in.Lock()
				for _, r := range result {
					if r["submitted"] == true {
						in.Submitted++
					}
				}
				in.Unlock()
				if callCtx.Err() != nil {
					return nil, callCtx.Err()
				}
				return result, nil

			}, func() bool { in.Lock(); defer in.Unlock(); v := in.Trigger; in.Trigger = false; return v },
				func(state string) { in.Lock(); in.State = state; in.Unlock() }, func(success, skip bool, e error) {
					in.Lock()
					defer in.Unlock()
					if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
						return
					}
					if skip {
						in.Skipped++
					} else {
						in.Executed++
					}
					if e != nil {
						if in.LastError == "" {
							in.LastError = e.Error()
						}
						failures++
						if failures >= 3 {
							stopRound()
						}
					} else if !skip {
						failures = 0
					}
				})
			if graphErr != nil && ctx.Err() == nil && roundCtx.Err() == nil {
				in.Lock()
				in.LastError = graphErr.Error()
				in.Unlock()
				failures = 3
				stopRound()
			}
			if !bm.IsConnected() && ctx.Err() == nil {
				in.Lock()
				in.LastError = "网络连接已关闭；已提交命令的游戏结果未知，没有自动重发"
				in.Unlock()
				failures = 3
				stopRound()
			}
			interrupted := roundCtx.Err() != nil
			stopRound()
			in.Lock()
			if !interrupted {
				in.Cycles++
				in.LastRoundMS = time.Since(roundStarted).Milliseconds()
				in.ElapsedMS = time.Since(in.StartedAt).Milliseconds()
			}
			completed := !interrupted && cfg.RunCount > 0 && in.Cycles >= uint64(cfg.RunCount)
			if completed {
				in.State = "已完成"
			}
			if failures >= 3 && ctx.Err() == nil {
				in.State = "错误暂停"
			}
			in.Unlock()
			if completed || failures >= 3 {
				return
			}
		}
		// Hz describes whole-chain start cadence, not a per-node delay or an
		// extra sleep after completion. Overruns continue without queued rounds.
		if err := yyChainSleep(ctx, yyChainRoundWait(yyChainPeriod(plan, cfg), time.Since(roundStarted))); err != nil {
			return
		}
	}
}

func yyChainFastInvocationEnded(ctx context.Context, err error, rows []map[string]any) bool {
	if ctx.Err() == nil || !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	for _, row := range rows {
		if row["unknown"] == true {
			return false
		}
	}
	return true
}

func yyChainRoundWait(period, elapsed time.Duration) time.Duration {
	if elapsed >= period {
		return 0
	}
	return period - elapsed
}

func yyChainPeriod(plan yyChainPlan, cfg yyChainConfig) time.Duration {
	hz := plan.Hz
	if cfg.Hz > 0 && (hz == 0 || cfg.Hz < hz) {
		hz = cfg.Hz
	}
	if hz > 0 {
		if d, e := yyScheduleDuration("util.scheduleHz", hz); e == nil {
			return d
		}
	}
	if cfg.IntervalMS > 0 {
		return time.Duration(cfg.IntervalMS) * time.Millisecond
	}
	return time.Second
}
func yyChainStop(in *yyChainInstance) error {
	in.Lock()
	cancel, done := in.cancel, in.done
	in.Config.Enabled = false
	err := yyChainSaveConfig(in.Config)
	in.Unlock()
	if cancel != nil {
		cancel()
		select {
		case <-done:
		case <-time.After(12 * time.Second):
			return fmt.Errorf("停止等待超时，请稍后查看状态")
		}
	}
	return err
}
func yyCommandRoutes(mux *http.ServeMux, dataDir string) {
	yyChainRoot = filepath.Join(dataDir, "插件数据文件", "ds_ai_agent", "command_txt")
	_ = os.MkdirAll(yyChainRoot, 0755)
	entries, _ := os.ReadDir(yyChainRoot)
	for _, entry := range entries {
		if _, e := yyChainName(entry.Name()); e != nil {
			continue
		}
		text, e := os.ReadFile(filepath.Join(yyChainRoot, entry.Name()))
		if e != nil {
			continue
		}
		plan, e := yyParseChains(string(text))
		if e != nil {
			continue
		}
		cfg := yyChainConfig{Name: entry.Name(), Account: "main", IntervalMS: 1000}
		meta, _ := os.ReadFile(filepath.Join(yyChainRoot, entry.Name()+".json"))
		_ = json.Unmarshal(meta, &cfg)
		in := &yyChainInstance{Config: cfg, Plan: plan, State: "已停止"}
		yyChains.items[cfg.Name] = in
		if cfg.Enabled {
			_ = yyChainStart(in)
		}
	}
	yyStartChainWatcher()
	mux.HandleFunc("/api/youyuan/chains", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if name := r.URL.Query().Get("name"); name != "" {
				if _, e := yyChainName(name); e != nil {
					writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
					return
				}
				data, e := os.ReadFile(filepath.Join(yyChainRoot, name))
				if e != nil {
					writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
					return
				}
				writeJSON(w, map[string]any{"ok": true, "content": string(data)})
				return
			}
			list := []map[string]any{}
			yyChains.Lock()
			for _, in := range yyChains.items {
				in.Lock()
				hz := float64(0)
				if in.ElapsedMS > 0 {
					hz = float64(in.Cycles) * 1000 / float64(in.ElapsedMS)
				}
				list = append(list, map[string]any{"config": in.Config, "segments": in.Segments, "segmented": in.Config.Scheduling == "segments" || in.Config.Scheduling != "file" && in.Plan.Segmented, "state": in.State, "cycles": in.Cycles, "executed": in.Executed, "submitted": in.Submitted, "skipped": in.Skipped, "error": in.LastError, "commands": in.Plan.Commands, "redstone": in.Redstone, "elapsed_ms": in.ElapsedMS, "last_round_ms": in.LastRoundMS, "actual_hz": hz, "results_confirmed": in.Config.Mode != "fast", "counter_semantics": "executed=attempted nodes; submitted=host accepted; cycles=file rounds or sum of independent segment rounds, not proof of server execution"})
				in.Unlock()
			}
			yyChains.Unlock()
			sort.Slice(list, func(i, j int) bool {
				return list[i]["config"].(yyChainConfig).Name < list[j]["config"].(yyChainConfig).Name
			})
			writeJSON(w, map[string]any{"ok": true, "files": list, "directory": "插件数据文件/ds_ai_agent/command_txt/"})
			return
		}
		var p struct {
			Action   string        `json:"action"`
			Config   yyChainConfig `json:"config"`
			Name     string        `json:"name"`
			Content  string        `json:"content"`
			Redstone bool          `json:"redstone"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if p.Name == "" {
			p.Name = p.Config.Name
		}
		name, e := yyChainName(p.Name)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		yyChainOpMu.Lock()
		defer yyChainOpMu.Unlock()
		yyChains.Lock()
		in := yyChains.items[name]
		yyChains.Unlock()
		fail := func(e error) { writeJSON(w, map[string]any{"ok": false, "error": e.Error()}) }
		switch p.Action {
		case "preview":
			plan, parseErr := yyParseChains(p.Content)
			if parseErr != nil {
				fail(parseErr)
				return
			}
			if err := yyChainValidate(p.Config); err != nil {
				fail(err)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "plan": plan})
			return
		case "save":
			plan, e := yyParseChains(p.Content)
			if e != nil {
				fail(e)
				return
			}
			p.Config.Name = name
			if p.Config.Account == "" {
				p.Config.Account = "main"
			}
			if p.Config.IntervalMS < 1 {
				p.Config.IntervalMS = 1
			}
			if p.Config.RunCount < 0 || p.Config.RunCount > 1000000 {
				fail(fmt.Errorf("运行次数应为 0（持续）或 1..1000000"))
				return
			}
			if p.Config.DelayMS < 0 {
				p.Config.DelayMS = 0
			}
			if p.Config.Channel != "" {
				if _, e = normalizeCommandChannel(p.Config.Channel); e != nil {
					fail(e)
					return
				}
			}
			if e = yyChainValidate(p.Config); e != nil {
				fail(e)
				return
			}
			if p.Config.Hz > 0 {
				p.Config.IntervalMS = max(1, int(1000/p.Config.Hz+0.5))
			}
			if in != nil {
				if e = yyChainStop(in); e != nil {
					fail(e)
					return
				}
			}
			if e = os.WriteFile(filepath.Join(yyChainRoot, name), []byte(p.Content), 0600); e != nil {
				fail(e)
				return
			}
			if e = yyChainSaveConfig(p.Config); e != nil {
				fail(e)
				return
			}
			in = &yyChainInstance{Config: p.Config, Plan: plan, State: "已停止"}
			yyChains.Lock()
			yyChains.items[name] = in
			yyChains.Unlock()
			if p.Config.Enabled {
				if e = yyChainStart(in); e != nil {
					fail(e)
					return
				}
			}
			writeJSON(w, map[string]any{"ok": true, "plan": plan})
			return
		case "start", "stop", "trigger", "redstone", "delete", "reload":
			if in == nil {
				fail(fmt.Errorf("文件不存在"))
				return
			}
			switch p.Action {
			case "start":
				e = yyChainStart(in)
			case "stop":
				e = yyChainStop(in)
			case "trigger":
				in.Lock()
				in.Trigger = true
				in.Unlock()
			case "redstone":
				in.Lock()
				if p.Redstone && !in.Redstone {
					in.Trigger = true
				}
				in.Redstone = p.Redstone
				in.Unlock()
			case "reload":
				e = yyReloadChainResource(in)
			case "delete":
				if e = yyChainStop(in); e == nil {
					e = os.Remove(filepath.Join(yyChainRoot, name))
					_ = os.Remove(filepath.Join(yyChainRoot, name+".json"))
					yyChains.Lock()
					delete(yyChains.items, name)
					yyChains.Unlock()
				}
			}
			if e != nil {
				fail(e)
				return
			}
		default:
			fail(fmt.Errorf("未知操作"))
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
}
