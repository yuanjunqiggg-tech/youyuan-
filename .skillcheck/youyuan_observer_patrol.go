package main

// Optional, cancellable observation motion. Scheduling belongs to an entire
// route; TP submission is not a claim that the server completed a teleport.
import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

var yyPatrolDocs = map[string]string{
	"observerBridge.patrolStart":  "options={players:[真实在线名],interval_seconds:0.001,single_follow_seconds:2,duration_seconds:0,restore_position:true}。空players自动选择当前真人/其他玩家，排除自己；每次开启分配独立计分板编号一次，刷新需停止重开。按1..N顺序TP只发不等，单玩家使用single_follow_seconds。默认关闭；0.001是提交目标间隔，不是服务器生效保证；插件停止自动取消。",
	"observerBridge.patrolStatus": "无参数。真实submitted/rounds、missed_intervals、position_updates、by_player事件计数、最后实际坐标、错误与清理状态；不是全局覆盖证明。",
	"observerBridge.patrolStop":   "无参数。取消当前账号轮巡，移除本次专用分数目标；默认恢复起点。插件不能关闭其他插件拥有的轮巡。",
}

func init() {
	for k, v := range yyPatrolDocs {
		yyExtraMethods[k] = v
	}
}

type yyPatrolConfig struct {
	Players  []string `json:"players"`
	Interval float64  `json:"interval_seconds"`
	Single   float64  `json:"single_follow_seconds"`
	Duration float64  `json:"duration_seconds"`
	Restore  *bool    `json:"restore_position"`
}
type yyPatrol struct {
	sync.Mutex
	Bot              *BotManager
	Owner            string
	Config           yyPatrolConfig
	Objective        string
	Started          time.Time
	Ended            time.Time
	Cancel           context.CancelFunc
	Done             chan struct{}
	Active           bool
	Submitted        uint64
	Rounds           uint64
	Missed           uint64
	PositionUpdates  uint64
	Events           map[string]uint64
	ByPlayer         map[string]map[string]uint64
	LastPosition     any
	LastPositionTime any
	Error            string
	Cleanup          []map[string]any
	Origin           mgl32.Vec3
	OriginDimension  int32
}

var yyPatrols = struct {
	sync.Mutex
	Items map[*BotManager]*yyPatrol
}{Items: map[*BotManager]*yyPatrol{}}

func yyPatrolDuration(sec float64) (time.Duration, error) {
	if math.IsNaN(sec) || math.IsInf(sec, 0) || sec <= 0 || sec > 86400*365 || sec*float64(time.Second) < 1 {
		return 0, fmt.Errorf("间隔需要正有限秒数，可表示到1纳秒；不是游戏生效速度保证")
	}
	return time.Duration(sec * float64(time.Second)), nil
}
func yyPatrolConfigParse(v any) (yyPatrolConfig, error) {
	c := yyPatrolConfig{Interval: .001, Single: 2}
	raw, e := json.Marshal(v)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(raw, &c); e != nil {
		return c, e
	}
	if _, e = yyPatrolDuration(c.Interval); e != nil {
		return c, e
	}
	if _, e = yyPatrolDuration(c.Single); e != nil {
		return c, e
	}
	if c.Duration != 0 {
		if _, e = yyPatrolDuration(c.Duration); e != nil {
			return c, e
		}
	}
	seen := map[string]bool{}
	for _, name := range c.Players {
		if strings.TrimSpace(name) == "" || len(name) > 128 || strings.ContainsAny(name, "\r\n\x00") || seen[name] {
			return c, fmt.Errorf("玩家名必须唯一有效，不能包含换行")
		}
		seen[name] = true
	}
	return c, nil
}
func yyPatrolCommand(objective string, index int) string {
	return fmt.Sprintf("tp @s @a[scores={%s=%d},c=1]", objective, index)
}
func yyPatrolGet(bm *BotManager) *yyPatrol {
	yyPatrols.Lock()
	defer yyPatrols.Unlock()
	return yyPatrols.Items[bm]
}
func (p *yyPatrol) snapshot() map[string]any {
	p.Lock()
	defer p.Unlock()
	rows := []map[string]any{}
	for i, n := range p.Config.Players {
		rows = append(rows, map[string]any{"player": n, "score": i + 1})
	}
	events := map[string]uint64{}
	for k, v := range p.Events {
		events[k] = v
	}
	by := map[string]map[string]uint64{}
	for k, v := range p.ByPlayer {
		m := map[string]uint64{}
		for x, n := range v {
			m[x] = n
		}
		by[k] = m
	}
	elapsed := time.Since(p.Started).Seconds()
	if !p.Ended.IsZero() {
		elapsed = p.Ended.Sub(p.Started).Seconds()
	}
	return map[string]any{"active": p.Active, "objective": p.Objective, "players": rows, "config": p.Config, "started_at": p.Started.UnixMilli(), "elapsed_seconds": elapsed, "submitted": p.Submitted, "rounds": p.Rounds, "submitted_per_second": float64(p.Submitted) / math.Max(elapsed, .000001), "missed_intervals": p.Missed, "position_updates": p.PositionUpdates, "last_position": p.LastPosition, "last_position_timestamp": p.LastPositionTime, "events": events, "by_player": by, "error": p.Error, "cleanup": append([]map[string]any(nil), p.Cleanup...), "global_coverage_guaranteed": false, "actual_tp_rate_known": false, "position_update_note": "位置回包可能合并/重复；不能等同每次TP完成"}
}
func yyPatrolObserve(bm *BotManager, e map[string]any) {
	p := yyPatrolGet(bm)
	if p == nil {
		return
	}
	p.Lock()
	defer p.Unlock()
	if !p.Active {
		return
	}
	kind, _ := e["type"].(string)
	p.Events[kind]++
	name, _ := e["player"].(string)
	if name != "" {
		if p.ByPlayer[name] == nil {
			p.ByPlayer[name] = map[string]uint64{}
		}
		p.ByPlayer[name][kind]++
	}
	if kind == "move" {
		bm.mu.Lock()
		nameSelf := bm.display
		bm.mu.Unlock()
		if name == nameSelf {
			p.PositionUpdates++
			p.LastPosition = e["position"]
			p.LastPositionTime = e["timestamp"]
		}
	}
}
func yyPatrolStopOwner(bm *BotManager, owner string) {
	yyWatchStopOwner(bm, owner)
	p := yyPatrolGet(bm)
	if p != nil && p.Owner == owner {
		p.Cancel()
	}
}
func yyPatrolCall(bm *BotManager, owner, method string, args []any) (any, error) {
	if method == "observerBridge.assignDimension" || method == "observerBridge.dimensionStatus" {
		return yyObserverDimensionCall(bm, method, args)
	}
	if strings.HasPrefix(method, "observerBridge.watch") {
		return yyWatchCall(bm, owner, method, args)
	}
	if method == "observerBridge.patrolStatus" {
		if p := yyPatrolGet(bm); p != nil {
			return p.snapshot(), nil
		}
		return map[string]any{"active": false, "global_coverage_guaranteed": false}, nil
	}
	if method == "observerBridge.patrolStop" {
		p := yyPatrolGet(bm)
		if p == nil {
			return map[string]any{"active": false}, nil
		}
		if owner != "manual" && p.Owner != owner {
			return nil, fmt.Errorf("轮巡由其他插件管理")
		}
		p.Cancel()
		select {
		case <-p.Done:
		case <-time.After(3 * time.Second):
		}
		return p.snapshot(), nil
	}
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("账号未连接")
	}
	if w := yyWatchGet(bm); w != nil {
		w.Lock()
		active := w.Active
		w.Unlock()
		if active {
			return nil, fmt.Errorf("先停止本账号常驻观察，避免运动冲突")
		}
	}
	bm.mu.Lock()
	op, self, origin, dim := bm.isOP, bm.display, bm.pos, bm.dimension
	bm.mu.Unlock()
	if !op {
		return nil, fmt.Errorf("轮巡需要该机器人OP传送权限")
	}
	var value any = map[string]any{}
	if len(args) > 0 {
		value = args[0]
	}
	cfg, e := yyPatrolConfigParse(value)
	if e != nil {
		return nil, e
	}
	online := playerInfoMgr.OnlineSet()
	if len(cfg.Players) == 0 {
		for name := range online {
			if name != self {
				cfg.Players = append(cfg.Players, name)
			}
		}
		sort.Strings(cfg.Players)
	}
	for _, name := range cfg.Players {
		if name == self || !online[name] {
			return nil, fmt.Errorf("目标%s不是当前在线的其他玩家", name)
		}
	}
	if len(cfg.Players) == 0 {
		return nil, fmt.Errorf("没有可观察的在线玩家")
	}
	if cfg.Restore == nil || *cfg.Restore {
		// MovePlayer's cached input position is the eye position. A command TP
		// needs feet coordinates; never restore the raw cache and drift upward.
		positions, err := bm.DoQuerytarget("@s")
		if err != nil || len(positions) != 1 {
			return nil, fmt.Errorf("无法确认轮巡起点，请重试或明确关闭恢复位置")
		}
		pos := positions[0].Position
		origin = yyPatrolFeetPosition(mgl32.Vec3{pos.X, pos.Y, pos.Z})
		dim = int32(positions[0].Dimension)
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &yyPatrol{Bot: bm, Owner: owner, Config: cfg, Objective: "yy_o_" + yyNewKey()[:8], Started: time.Now(), Cancel: cancel, Done: make(chan struct{}), Active: true, Events: map[string]uint64{}, ByPlayer: map[string]map[string]uint64{}, Origin: origin, OriginDimension: dim}
	yyPatrols.Lock()
	old := yyPatrols.Items[bm]
	if old != nil {
		select {
		case <-old.Done:
		default:
			yyPatrols.Unlock()
			cancel()
			return nil, fmt.Errorf("已有轮巡运行或正在清理，请先停止")
		}
	}
	yyPatrols.Items[bm] = p
	yyPatrols.Unlock()
	// Confirm setup once. Runtime loop uses the original no-wait fast transport.
	setup := []string{"scoreboard objectives add " + p.Objective + " dummy"}
	for i, name := range cfg.Players {
		q, _ := json.Marshal(name)
		setup = append(setup, fmt.Sprintf("scoreboard players set %s %s %d", q, p.Objective, i+1))
	}
	for _, cmd := range setup {
		if ctx.Err() != nil {
			e = ctx.Err()
			break
		}
		var v any
		v, e = yyPluginCommandCall(bm, "game.execute", []any{cmd, map[string]any{"channel": "ai", "timeout_ms": float64(3000)}}, yyDefaultCommandOptions())
		if e != nil {
			break
		}
		if v.(map[string]any)["success"] != true {
			e = fmt.Errorf("轮巡初始化未确认成功: %v", v)
			break
		}
	}
	if e != nil {
		p.Lock()
		p.Error = e.Error()
		p.Unlock()
		p.finish()
		return nil, e
	}
	p.Lock()
	p.Started = time.Now()
	p.Unlock()
	go p.run(ctx)
	return p.snapshot(), nil
}

func yyPatrolFeetPosition(eye mgl32.Vec3) mgl32.Vec3 {
	eye[1] -= 1.6200104 // Same NetEase querytarget convention as GetPlayerPos.
	return eye
}
func (p *yyPatrol) run(ctx context.Context) {
	defer p.finish()
	if p.Config.Duration > 0 {
		d, _ := yyPatrolDuration(p.Config.Duration)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	interval, _ := yyPatrolDuration(p.Config.Interval)
	if len(p.Config.Players) == 1 {
		interval, _ = yyPatrolDuration(p.Config.Single)
	}
	next := time.Now()
	deadline := time.Time{}
	if p.Config.Duration > 0 {
		d, _ := yyPatrolDuration(p.Config.Duration)
		deadline = time.Now().Add(d)
	}
	index := 0
	for {
		if ctx.Err() != nil || !p.Bot.IsConnected() || (!deadline.IsZero() && !time.Now().Before(deadline)) {
			return
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if ctx.Err() != nil || (!deadline.IsZero() && !time.Now().Before(deadline)) {
			return
		}
		cmd := yyPatrolCommand(p.Objective, index+1)
		if busy, _ := yySpatialJobState(p.Bot); busy {
			next = time.Now().Add(interval)
			continue
		}
		sent, e := yyObserverSend(p.Bot, cmd, false)
		if !sent {
			next = time.Now().Add(interval)
			continue
		}
		p.Lock()
		if e != nil {
			p.Error = e.Error()
			p.Unlock()
			return
		}
		p.Submitted++
		index = (index + 1) % len(p.Config.Players)
		if index == 0 {
			p.Rounds++
		}
		next = next.Add(interval)
		now := time.Now()
		if next.Before(now) {
			missed := uint64(now.Sub(next)/interval) + 1
			p.Missed += missed
			next = now.Add(interval)
		}
		p.Unlock()
	}
}
func (p *yyPatrol) finish() {
	p.Cancel()
	p.Lock()
	p.Active = false
	p.Ended = time.Now()
	p.Unlock()
	commands := []string{"scoreboard objectives remove " + p.Objective}
	busy, _ := yySpatialJobState(p.Bot)
	if !busy && (p.Config.Restore == nil || *p.Config.Restore) {
		dim := "overworld"
		if p.OriginDimension == 1 {
			dim = "nether"
		} else if p.OriginDimension == 2 {
			dim = "the_end"
		}
		commands = append(commands, fmt.Sprintf("execute in %s run tp @s %.3f %.3f %.3f", dim, p.Origin[0], p.Origin[1], p.Origin[2]))
	}
	for _, cmd := range commands {
		e := p.Bot.SendCommandNoWaitRate(ChannelAI, cmd, 0)
		row := map[string]any{"command": cmd, "submitted": e == nil, "confirmed": false}
		if e != nil {
			row["error"] = e.Error()
		}
		p.Lock()
		p.Cleanup = append(p.Cleanup, row)
		p.Unlock()
	}
	close(p.Done)
}
