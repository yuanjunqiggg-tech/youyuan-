package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

// A resident single-target primitive. Route/pool allocation stays under the
// caller's control; the packet reader never performs queries or HTTP requests.
func init() {
	for k, v := range map[string]string{
		"observerBridge.watchStart":  "options={player或players:[],auto_players:false,exclude_players:[],camera_selector:false,mode:camera|follow|hybrid,interval_seconds:2,camera_height_blocks:4,camera_pitch_degrees:90,camera_yaw_degrees:0,pause_during_jobs:true,duration_seconds:0,restore_position:true,follow_radius_blocks:16}；players/auto_players或camera_selector启用原生execute at选择器相机切换，不逐次querytarget、不TP身体、不等待确认；间隔属于一次目标切换，一轮约目标数乘间隔，submitted不能视为收到所有包。follow/hybrid可能TP并查询位置；导入导出期间默认暂停，插件可先查jobState再传送。需OP，不保证跨维度或全局广播；停止插件自动取消。",
		"observerBridge.watchStatus": "返回本账号常驻观察目标、真实区域发布时间、submitted、missed_intervals、错误。region_updated只是收到对应区域包，不等于零漏包。",
		"observerBridge.watchStop":   "取消本账号常驻观察，清除本次相机并恢复已确认的原位置；不会关闭其他插件拥有的观察。",
	} {
		yyPatrolDocs[k] = v
		yyExtraMethods[k] = v
	}
}

type yyWatchConfig struct {
	Player               string   `json:"player"`
	Players              []string `json:"players,omitempty"`
	AutoPlayers          bool     `json:"auto_players"`
	ExcludePlayers       []string `json:"exclude_players,omitempty"`
	ControlLabel         string   `json:"control_label,omitempty"`
	CameraSelector       bool     `json:"camera_selector"`
	PauseDuringJobs      *bool    `json:"pause_during_jobs,omitempty"`
	Mode                 string   `json:"mode"`
	Interval             float64  `json:"interval_seconds"`
	Duration             float64  `json:"duration_seconds"`
	Restore              *bool    `json:"restore_position"`
	FollowRadius         float64  `json:"follow_radius_blocks"`
	CameraHeight         float64  `json:"camera_height_blocks"`
	CameraPitch          *float64 `json:"camera_pitch_degrees,omitempty"`
	CameraYaw            float64  `json:"camera_yaw_degrees"`
	CameraFacing         bool     `json:"camera_facing_player"`
	ObserveDimension     *int32   `json:"observe_dimension,omitempty"`
	CameraRelative       bool     `json:"camera_relative"`
	CameraFollowRotation bool     `json:"camera_follow_rotation"`
	CameraBack           float64  `json:"camera_back_blocks"`
	PhaseSeconds         float64  `json:"phase_seconds"`
	TargetOffset         int      `json:"target_offset"`
	PlanSignature        string   `json:"plan_signature,omitempty"`
}
type yyWatch struct {
	sync.Mutex
	Bot             *BotManager
	Owner           string
	Config          yyWatchConfig
	Cancel          context.CancelFunc
	Done            chan struct{}
	Active          bool
	Started         time.Time
	Target          mgl32.Vec3
	RegionOrigin    mgl32.Vec3
	TargetDimension int32
	SubmittedAt     time.Time
	TargetSince     time.Time
	Submitted       uint64
	FollowTeleports uint64
	FollowSkipped   uint64
	Missed          uint64
	Error           string
	CurrentPlayer   string
	Origin          mgl32.Vec3
	OriginDimension int32
	Cleanup         []map[string]any
}

var yyWatches = struct {
	sync.Mutex
	Items map[*BotManager]*yyWatch
}{Items: map[*BotManager]*yyWatch{}}

func yyWatchParse(v any) (yyWatchConfig, error) {
	c := yyWatchConfig{Mode: "camera", Interval: 2, FollowRadius: 16, CameraHeight: 2}
	raw, e := json.Marshal(v)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(raw, &c); e != nil {
		return c, e
	}
	if c.ObserveDimension != nil && (*c.ObserveDimension < 0 || *c.ObserveDimension > 2) {
		return c, fmt.Errorf("observe_dimension需要0/1/2")
	}
	if math.IsNaN(c.PhaseSeconds) || math.IsInf(c.PhaseSeconds, 0) || c.PhaseSeconds < 0 || c.PhaseSeconds > 3600 || c.TargetOffset < 0 {
		return c, fmt.Errorf("phase_seconds需0..3600，target_offset不能为负")
	}
	if math.IsNaN(c.CameraBack) || math.IsInf(c.CameraBack, 0) {
		return c, fmt.Errorf("camera_back_blocks需要有限数值")
	}
	if c.Player == "" && len(c.Players) > 0 {
		c.Player = c.Players[0]
	}
	if !yyWatchName(c.Player) && !c.AutoPlayers {
		return c, fmt.Errorf("需要准确的游戏玩家名")
	}
	seen := map[string]bool{}
	for _, n := range c.Players {
		if !yyWatchName(n) || seen[n] {
			return c, fmt.Errorf("玩家列表需要唯一有效的游戏名")
		}
		seen[n] = true
	}
	if c.Mode != "camera" && c.Mode != "follow" && c.Mode != "hybrid" {
		return c, fmt.Errorf("mode需要camera/follow/hybrid")
	}
	if math.IsNaN(c.FollowRadius) || math.IsInf(c.FollowRadius, 0) || c.FollowRadius < 0 || c.FollowRadius > 4096 {
		return c, fmt.Errorf("follow_radius_blocks需要0..4096；0每轮请求，默认16仅离开半径或跨维度时TP")
	}
	if math.IsNaN(c.CameraHeight) || math.IsInf(c.CameraHeight, 0) || math.Abs(c.CameraHeight) > math.MaxFloat32 || math.IsNaN(c.CameraYaw) || math.IsInf(c.CameraYaw, 0) {
		return c, fmt.Errorf("摄像机高度和朝向需要有效有限数值")
	}
	if c.CameraPitch != nil && (math.IsNaN(*c.CameraPitch) || math.IsInf(*c.CameraPitch, 0) || *c.CameraPitch < -90 || *c.CameraPitch > 90) {
		return c, fmt.Errorf("camera_pitch_degrees需要-90..90；90向下、-90向上")
	}
	if c.CameraPitch != nil && c.CameraFacing {
		return c, fmt.Errorf("选择固定俯仰角或朝向玩家，不同时指定两者")
	}
	if _, e = yyPatrolDuration(c.Interval); e != nil {
		return c, e
	}
	if c.Duration != 0 {
		_, e = yyPatrolDuration(c.Duration)
	}
	return c, e
}
func yyWatchCameraCommand(self string, c yyWatchConfig, feet mgl32.Vec3) (string, mgl32.Vec3) {
	position := feet
	position[1] += float32(c.CameraHeight)
	name, _ := json.Marshal(self)
	command := fmt.Sprintf("camera %s set minecraft:free pos %.3f %.3f %.3f", name, position[0], position[1], position[2])
	if c.CameraPitch != nil {
		command += fmt.Sprintf(" rot %.3f %.3f", *c.CameraPitch, math.Mod(c.CameraYaw, 360))
	} else if c.CameraFacing {
		target, _ := json.Marshal(c.Player)
		command += " facing " + string(target)
	}
	return command, position
}
func yyWatchName(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if r < 32 || r == '"' || r == '\\' {
			return false
		}
	}
	return true
}
func yyWatchGet(bm *BotManager) *yyWatch {
	yyWatches.Lock()
	defer yyWatches.Unlock()
	return yyWatches.Items[bm]
}
func yyWatchStopOwner(bm *BotManager, owner string) {
	if w := yyWatchGet(bm); w != nil && w.Owner == owner {
		w.Cancel()
	}
}
func yyWatchRegionReady(position mgl32.Vec3, at time.Time, expected mgl32.Vec3, submitted time.Time) bool {
	return !submitted.IsZero() && !at.Before(submitted) && math.Abs(float64(position[0]-expected[0])) < 3 && math.Abs(float64(position[2]-expected[2])) < 3
}

func yyWatchNeedsTeleport(known bool, body, target mgl32.Vec3, bodyDimension, targetDimension int32, radius float64) bool {
	return radius == 0 || !known || bodyDimension != targetDimension || float64(body.Sub(target).Len()) > radius
}
func (w *yyWatch) snapshot() map[string]any {
	w.Lock()
	defer w.Unlock()
	s := yySenseFor(w.Bot)
	s.Lock()
	p := s.PublisherPosition
	at := s.PublisherAt
	counts := map[string]uint64{}
	for _, name := range []string{"Animate", "UpdateAttributes", "ActorEvent", "UpdateBlock", "NetworkChunkPublisherUpdate"} {
		counts[name] = s.PacketCounts[name]
	}
	lastPacket, session := s.LastPacketAt, s.Session
	s.Unlock()
	result := map[string]any{"active": w.Active, "config": w.Config, "started_at": w.Started.UnixMilli(), "submitted": w.Submitted, "follow_teleports": w.FollowTeleports, "follow_skipped_near": w.FollowSkipped, "missed_intervals": w.Missed, "target_position": w.Target, "target_dimension": w.TargetDimension, "target_since": yyVisibilityTime(w.TargetSince), "last_submitted_at": yyVisibilityTime(w.SubmittedAt), "region_updated": yyWatchRegionReady(mgl32.Vec3{float32(p[0]), float32(p[1]), float32(p[2])}, at, w.Target, w.TargetSince), "region_timestamp": yyVisibilityTime(at), "region_position": p, "region_evidence": "NetworkChunkPublisherUpdate only; not complete entity synchronization", "global_coverage_guaranteed": false, "error": w.Error, "cleanup": append([]map[string]any(nil), w.Cleanup...)}
	busy, reasons := yySpatialJobState(w.Bot)
	result["spatial_job_active"] = busy
	result["paused_for_job"] = busy && (w.Config.Mode != "camera" || w.Config.PauseDuringJobs == nil || *w.Config.PauseDuringJobs)
	result["pause_reasons"] = reasons
	result["received_packet_counts"] = counts
	result["received_counts_scope"] = "this connection session, not camera-confirmed targets; Animate includes actions other than swing"
	result["last_received_packet_at"] = yyVisibilityTime(lastPacket)
	result["connection_session"] = session
	result["current_player"] = w.CurrentPlayer
	if w.Config.Mode == "camera" && (w.Config.CameraSelector || len(w.Config.Players) > 1 || w.Config.AutoPlayers) {
		result["region_updated"] = false
		result["target_position_known"] = false
		result["target_position"] = nil
		result["target_dimension_known"] = false
		result["camera_target_resolution"] = "server_selector"
	}
	return result
}
func yyWatchCall(bm *BotManager, owner, method string, args []any) (any, error) {
	if method == "observerBridge.watchStatus" {
		if w := yyWatchGet(bm); w != nil {
			return w.snapshot(), nil
		}
		return map[string]any{"active": false}, nil
	}
	if method == "observerBridge.watchStop" {
		w := yyWatchGet(bm)
		if w == nil {
			return map[string]any{"active": false}, nil
		}
		if owner != "manual" && w.Owner != owner {
			return nil, fmt.Errorf("观察由其他插件管理")
		}
		w.Cancel()
		select {
		case <-w.Done:
		case <-time.After(4 * time.Second):
		}
		return w.snapshot(), nil
	}
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("观察账号未连接")
	}
	bm.mu.Lock()
	self, dim := bm.display, bm.dimension
	bm.mu.Unlock()
	if !bm.IsOP() {
		return nil, fmt.Errorf("观察需要本机器人OP")
	}
	var options any = map[string]any{}
	if len(args) > 0 {
		options = args[0]
	}
	c, e := yyWatchParse(options)
	if e != nil {
		return nil, e
	}
	if c.Player == self && len(c.Players) <= 1 && !c.AutoPlayers {
		return nil, fmt.Errorf("不能把自己作为观察目标")
	}
	if p := yyPatrolGet(bm); p != nil {
		p.Lock()
		active := p.Active
		p.Unlock()
		if active {
			return nil, fmt.Errorf("先停止本账号原轮巡，避免运动冲突")
		}
	}
	origin := mgl32.Vec3{}
	if c.Mode != "camera" && (c.Restore == nil || *c.Restore) {
		rows, e := bm.DoQuerytarget("@s")
		if e != nil || len(rows) != 1 {
			return nil, fmt.Errorf("不能确认观察账号原位置")
		}
		p := rows[0]
		origin = yyPatrolFeetPosition(mgl32.Vec3{p.Position.X, p.Position.Y, p.Position.Z})
		dim = int32(p.Dimension)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &yyWatch{Bot: bm, Owner: owner, Config: c, Cancel: cancel, Done: make(chan struct{}), Active: true, Started: time.Now(), Origin: origin, OriginDimension: dim}
	yyWatches.Lock()
	if old := yyWatches.Items[bm]; old != nil {
		select {
		case <-old.Done:
		default:
			yyWatches.Unlock()
			cancel()
			return nil, fmt.Errorf("已有观察运行，先停止再修改")
		}
	}
	yyWatches.Items[bm] = w
	yyWatches.Unlock()
	go w.run(ctx)
	return w.snapshot(), nil
}
func (w *yyWatch) run(ctx context.Context) {
	defer w.finish()
	if w.Config.Mode == "camera" && (w.Config.CameraSelector || len(w.Config.Players) > 1 || w.Config.AutoPlayers) {
		w.runSelectorCamera(ctx)
		return
	}
	if w.Config.Duration > 0 {
		d, _ := yyPatrolDuration(w.Config.Duration)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	interval, _ := yyPatrolDuration(w.Config.Interval)
	next := time.Now()
	index := 0
watchLoop:
	for ctx.Err() == nil && w.Bot.IsConnected() {
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if ctx.Err() != nil {
			return
		}
		if busy, _ := yySpatialJobState(w.Bot); busy {
			next = time.Now().Add(interval)
			continue
		}
		// This is a separate worker: query RTT does not hold a Lua VM or packet queue.
		targets := w.targets()
		if len(targets) == 0 {
			next = time.Now().Add(interval)
			continue
		}
		targetName := targets[index%len(targets)]
		index++
		q, _ := json.Marshal(targetName)
		rows, e := w.Bot.DoQuerytarget(string(q))
		if ctx.Err() != nil {
			return
		}
		if e != nil || len(rows) != 1 {
			w.Lock()
			w.Error = "目标位置查询失败或玩家已离线"
			w.Unlock()
			return
		}
		p := rows[0]
		if w.Config.ObserveDimension != nil && int32(p.Dimension) != *w.Config.ObserveDimension {
			next = time.Now().Add(interval)
			continue
		}
		feet := yyPatrolFeetPosition(mgl32.Vec3{p.Position.X, p.Position.Y, p.Position.Z})
		w.Bot.mu.Lock()
		dim, self := w.Bot.dimension, w.Bot.display
		w.Bot.mu.Unlock()
		if w.Config.Mode == "camera" && int32(p.Dimension) != dim {
			w.Lock()
			w.Error = "相机不能跨维度；选择follow/hybrid才能移动本机器人"
			w.Unlock()
			return
		}
		name, _ := json.Marshal(self)
		commands := []string{}
		teleport := false
		if w.Config.Mode != "camera" {
			body := yySenseActor(w.Bot, "self")
			bodyPosition, known := body["position"].(mgl32.Vec3)
			known = known && body["position_known"] == true && body["position_stale"] != true
			w.Lock()
			first := w.SubmittedAt.IsZero()
			w.Unlock()
			teleport = first || yyWatchNeedsTeleport(known, yyPatrolFeetPosition(bodyPosition), feet, dim, int32(p.Dimension), w.Config.FollowRadius)
		}
		if teleport {
			dimension := yyWatchDimension(int32(p.Dimension))
			commands = append(commands, fmt.Sprintf("execute in %s run tp %s %.3f %.3f %.3f", dimension, name, feet[0], feet[1], feet[2]))
		}
		expected := feet
		if w.Config.Mode != "follow" {
			cameraConfig := w.Config
			cameraConfig.Player = targetName
			camera, position := yyWatchCameraCommand(self, cameraConfig, feet)
			expected = position
			commands = append(commands, camera)
		}
		at := time.Now()
		for _, cmd := range commands {
			if ctx.Err() != nil {
				return
			}
			sent, err := yyObserverSend(w.Bot, cmd, false)
			if !sent {
				next = time.Now().Add(interval)
				continue watchLoop
			}
			e = err
			if e != nil {
				w.Lock()
				w.Error = e.Error()
				w.Unlock()
				return
			}
		}
		w.Lock()
		// Repeated submissions to an unchanged area must not erase evidence that
		// the server has already published it. A move to another area starts a
		// fresh generation; evidence preceding that generation is rejected.
		if w.TargetSince.IsZero() || w.TargetDimension != int32(p.Dimension) || w.RegionOrigin.Sub(expected).Len() >= 3 {
			w.TargetSince = at
			w.RegionOrigin = expected
		}
		w.CurrentPlayer = targetName
		w.Target = expected
		w.TargetDimension = int32(p.Dimension)
		if len(commands) > 0 {
			w.SubmittedAt = at
		}
		w.Submitted += uint64(len(commands))
		if teleport {
			w.FollowTeleports++
		} else if w.Config.Mode != "camera" {
			w.FollowSkipped++
		}
		next = next.Add(interval)
		if now := time.Now(); next.Before(now) {
			w.Missed += uint64(now.Sub(next)/interval) + 1
			next = now.Add(interval)
		}
		w.Unlock()
	}
}
func yyWatchDimension(d int32) string {
	switch d {
	case 1:
		return "nether"
	case 2:
		return "the_end"
	default:
		return "overworld"
	}
}
func (w *yyWatch) finish() {
	w.Cancel()
	commands := []string{}
	if w.Config.Mode != "follow" {
		commands = append(commands, "camera @s clear")
	}
	busy, _ := yySpatialJobState(w.Bot)
	if !busy && w.Config.Mode != "camera" && (w.Config.Restore == nil || *w.Config.Restore) {
		commands = append(commands, fmt.Sprintf("execute in %s run tp @s %.3f %.3f %.3f", yyWatchDimension(w.OriginDimension), w.Origin[0], w.Origin[1], w.Origin[2]))
	}
	for _, cmd := range commands {
		e := w.Bot.SendCommandNoWaitRate(ChannelAI, cmd, 0)
		row := map[string]any{"command": cmd, "submitted": e == nil, "confirmed": false}
		if e != nil {
			row["error"] = e.Error()
		}
		w.Lock()
		w.Cleanup = append(w.Cleanup, row)
		w.Unlock()
	}
	w.Lock()
	w.Active = false
	w.Unlock()
	close(w.Done)
}
