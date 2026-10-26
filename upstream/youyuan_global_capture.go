package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type yyGlobalCaptureConfig struct {
	Enabled     bool     `json:"enabled"`
	Accounts    []string `json:"accounts"`
	Players     []string `json:"players"`
	AutoPlayers bool     `json:"auto_players"`
	Mode        string   `json:"mode"`
	GameMode    string   `json:"game_mode"`
	Invisible   *bool    `json:"invisible"`
	QuickTP     *bool    `json:"quick_tp"`
	AutoRespawn *bool    `json:"auto_respawn"`
	Interval    float64  `json:"interval_seconds"`
	Height      float64  `json:"camera_height_blocks"`
	Pitch       float64  `json:"camera_pitch_degrees"`
	Yaw         float64  `json:"camera_yaw_degrees"`
	// Pointer so an old global-capture.json (written before the field existed)
	// keeps the safe default instead of silently disabling body proximity.
	BodyRadius         *float64                    `json:"body_radius_blocks"`
	PauseJobs          bool                        `json:"pause_during_jobs"`
	DimensionRouting   *bool                       `json:"dimension_routing"`
	DimensionAccounts  map[string]string           `json:"dimension_accounts"`
	DimensionLandings  map[string]yyCaptureLanding `json:"dimension_landings"`
	DimensionGameModes map[string]string           `json:"dimension_game_modes"`
	// 用户在界面上排出来的维度条目（含还没指定机器人的条目），只用于回显与条目数上限，
	// 调度仍以 DimensionAccounts/DimensionGameModes 为准。
	DimensionList     []string             `json:"dimension_list"`
	AutoFleet         *bool                `json:"auto_fleet"`
	AutoProvision     *bool                `json:"auto_provision"`
	AccountLimit      int                  `json:"account_limit"`
	FleetSteps        []yyCaptureFleetStep `json:"fleet_steps"`
	ResidentObservers int                  `json:"resident_observers"`
	IdleSeconds       int                  `json:"idle_seconds"`
	PlayerLimit       int                  `json:"player_limit"`
	EnforceLimit      bool                 `json:"enforce_player_limit"`
}

var yyGlobalCapture = struct {
	sync.Mutex
	Config     yyGlobalCaptureConfig
	Actor      yyPoolActor
	Path       string
	Generation uint64
	States     map[string]any
	Plan       any
}{Config: yyGlobalCaptureConfig{Mode: "camera", Interval: .0005, Height: 6, Pitch: 90, PauseJobs: true, AutoPlayers: true}, States: map[string]any{}}

var yyGlobalCaptureLoopOnce sync.Once

// yyGlobalBodyRadius 取生效的接近半径。未设置时用默认 48 格：
// 服务端只在机器人机体附近广播实体与属性包，用户要求默认 48 格；
// 显式设 0 关闭接近（远处可能抓不到属性包）。
// 2026-10-26 第 27 条：前端界面已写「默认 48 格」，后端却还是 15，
// 两边对不上，这里改成 48 与界面一致。
func yyGlobalBodyRadius(c yyGlobalCaptureConfig) float64 {
	if c.BodyRadius == nil {
		return 48
	}
	return *c.BodyRadius
}

func yyGlobalWatchOptions(c yyGlobalCaptureConfig) map[string]any {
	mode, radius := c.Mode, yyGlobalBodyRadius(c)
	if c.QuickTP != nil && !*c.QuickTP {
		mode, radius = "camera", 0
	}
	return map[string]any{"players": c.Players, "auto_players": c.AutoPlayers, "camera_selector": true, "mode": mode, "interval_seconds": c.Interval, "camera_height_blocks": c.Height, "camera_pitch_degrees": c.Pitch, "camera_yaw_degrees": c.Yaw, "body_radius_blocks": radius, "pause_during_jobs": c.PauseJobs, "restore_position": true, "control_label": "global_capture"}
}

// 多维抓包的维度标识不再限定 0/1/2：用户明确要求「常驻维度数量又不一定是3，
// 还有其他维度 dm」。内部仍用 0/1/2 表示主世界/下界/末地以兼容老配置，其余合法
// 标识原样保存在配置里；只有能映射到实际游戏维度时才调度，映射不到就跳过并记 warn，
// 绝不 panic，也绝不猜维度（猜错会把机器人 TP 到别的世界）。
const yyCaptureDimensionLimit = 8

var (
	yyCaptureDimensionKeyRe = regexp.MustCompile(`^[a-z0-9_:.-]{1,40}$`)
	yyCaptureBuiltinDims    = []string{"0", "1", "2"}
)

func yyCaptureDimensionKeyValid(key string) bool {
	return yyCaptureDimensionKeyRe.MatchString(key)
}

// yyCaptureDimensionKnown 解析维度标识。返回 false 表示当前游戏维度模型里没有它，
// 调用方必须跳过；dm 这类服务器自定义维度在这里就是 false。
func yyCaptureDimensionKnown(key string) (int32, bool) {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return 0, false
	}
	// 纯数字直接当维度 ID。网易自定义维度在协议里就是 int32，
	// 所以 "3"/"4" 这种写法必须能用。
	if id, err := strconv.Atoi(k); err == nil {
		if yyDimensionIDValid(int32(id)) {
			return int32(id), true
		}
		return 0, false
	}
	switch k {
	case "overworld", "minecraft:overworld", "theworld":
		return 0, true
	case "nether", "the_nether", "minecraft:nether", "minecraft:the_nether":
		return 1, true
	case "end", "the_end", "minecraft:end", "minecraft:the_end":
		return 2, true
	}
	// dm3 / dm4 …：复用 bot_conn.go 的 dimNameToID（它按 dm%d 解析）。
	// 光写 "dm" 不带序号是无法确定的，仍然拒绝——盲猜会把机体 TP 到别的世界。
	if id := dimNameToID(k); id > 0 {
		return int32(id), true
	}
	return 0, false
}

var yyCaptureDimWarned = struct {
	sync.Mutex
	Keys map[string]bool
}{Keys: map[string]bool{}}

// yyCaptureDimensionWarn 每个维度只记一次：reconcile 每轮都会走到分配逻辑，
// 不节流会把日志刷满。warn 里说明「配置保留、运行时跳过」，避免被当成配置丢失。
func yyCaptureDimensionWarn(key string) {
	yyCaptureDimWarned.Lock()
	first := !yyCaptureDimWarned.Keys[key]
	yyCaptureDimWarned.Keys[key] = true
	yyCaptureDimWarned.Unlock()
	if !first {
		return
	}
	log.Printf("[global-capture] warn: 维度 %q 暂无法映射到主世界/下界/末地，本轮跳过该维度（配置保留）", key)
}

// yyGlobalDimensionKeys 返回界面上应展示的维度条目顺序：内置三维度在前，
// 其次是用户自排的 dimension_list，最后兜底收集配置里出现但没进列表的键。
func yyGlobalDimensionKeys(c yyGlobalCaptureConfig) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(key string) {
		if key != "" && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	for _, key := range yyCaptureBuiltinDims {
		add(key)
	}
	for _, key := range c.DimensionList {
		add(key)
	}
	rest := make([]string, 0, len(c.DimensionAccounts)+len(c.DimensionGameModes))
	for key := range c.DimensionAccounts {
		rest = append(rest, key)
	}
	for key := range c.DimensionGameModes {
		rest = append(rest, key)
	}
	sort.Strings(rest)
	for _, key := range rest {
		add(key)
	}
	return out
}

func yyGlobalDimensionAssignments(c yyGlobalCaptureConfig) map[string]int32 {
	out := map[string]int32{}
	if c.DimensionRouting != nil && !*c.DimensionRouting {
		return out
	}
	if len(c.DimensionAccounts) > 0 {
		// 固定顺序：同一账号被配到多个维度时（校验会先拦），结果不能随 map 顺序抖动。
		keys := make([]string, 0, len(c.DimensionAccounts))
		for key := range c.DimensionAccounts {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		used := map[string]bool{}
		for _, key := range keys {
			id := c.DimensionAccounts[key]
			if id == "" {
				continue
			}
			dim, known := yyCaptureDimensionKnown(key)
			if !known {
				yyCaptureDimensionWarn(key)
				continue
			}
			if used[id] {
				continue
			}
			used[id] = true
			out[id] = dim
		}
		return out
	}
	for i, id := range c.Accounts {
		out[id] = int32(i % 3)
	}
	return out
}

// yyGlobalDimensionEntries 给界面回显「每个维度分到几台、有几台正在抓」。
// 不可调度的维度（如 dm）schedulable=false，界面据此显示「已保留但不调度」。
func yyGlobalDimensionEntries(c yyGlobalCaptureConfig, states map[string]any) []map[string]any {
	out := []map[string]any{}
	assigned := map[int32]int{}
	for _, dim := range yyGlobalDimensionAssignments(c) {
		assigned[dim]++
	}
	working := map[int32]int{}
	for _, raw := range states {
		v, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if v["phase"] != "observing" && v["phase"] != "resident_idle" {
			continue
		}
		if a, ok := v["assignment"].(yyCaptureAssignment); ok {
			working[a.Dimension]++
		}
	}
	for _, key := range yyGlobalDimensionKeys(c) {
		item := map[string]any{"key": key, "schedulable": false, "accounts": 0, "workers": 0}
		if dim, known := yyCaptureDimensionKnown(key); known {
			item["schedulable"] = true
			item["dimension"] = dim
			item["accounts"] = assigned[dim]
			item["workers"] = working[dim]
		}
		out = append(out, item)
	}
	return out
}
func yyGlobalCaptureValidate(c yyGlobalCaptureConfig, actor yyPoolActor) error {
	if !yyCaptureGameModeValid(c.GameMode) {
		return fmt.Errorf("游戏模式需为keep/survival/creative/adventure/spectator")
	}
	// 维度标识改成字符串键：允许 dm、minecraft:overworld、dye_dim 等，
	// 只校验「格式合法 + 条目数不超过 8」，不再限定 0/1/2。
	dimensions := map[string]bool{}
	for _, key := range c.DimensionList {
		if !yyCaptureDimensionKeyValid(key) {
			return fmt.Errorf("维度标识需为1～40位小写字母、数字或 _ : . -，例如 dm、dye_dim")
		}
		dimensions[key] = true
	}
	for key, mode := range c.DimensionGameModes {
		if !yyCaptureDimensionKeyValid(key) {
			return fmt.Errorf("维度标识需为1～40位小写字母、数字或 _ : . -，例如 dm、dye_dim")
		}
		if !yyCaptureGameModeValid(mode) {
			return fmt.Errorf("维度%s的游戏模式需为keep/survival/creative/adventure/spectator", key)
		}
		dimensions[key] = true
	}
	for key := range c.DimensionAccounts {
		if !yyCaptureDimensionKeyValid(key) {
			return fmt.Errorf("维度标识需为1～40位小写字母、数字或 _ : . -，例如 dm、dye_dim")
		}
		dimensions[key] = true
	}
	if len(dimensions) > yyCaptureDimensionLimit {
		return fmt.Errorf("维度条目最多%d个，当前%d个；三内置维度加dm等其他维度也够用", yyCaptureDimensionLimit, len(dimensions))
	}
	// 别名（overworld/minecraft:overworld）与 0 指向同一维度时只保留一个，
	// 否则同一维度会有两套机器人和游戏模式，状态与日志都对不上。
	resolved := map[int32]string{}
	for key := range dimensions {
		if dim, known := yyCaptureDimensionKnown(key); known {
			if other, dup := resolved[dim]; dup {
				return fmt.Errorf("维度%s与%s指向同一游戏维度，请只保留一个", other, key)
			}
			resolved[dim] = key
		}
	}
	// 落点救援仍按 0/1/2 执行（运行时的落点核对按维度编号比对），
	// 其他维度先要坐标会被拒绝，而不是存下来后静默不生效。
	for key := range c.DimensionLandings {
		if key != "0" && key != "1" && key != "2" {
			return fmt.Errorf("维度%s暂不支持指定上线坐标，落点救援目前只支持0/1/2", key)
		}
	}
	if err := yyCaptureLandingsValidate(c.DimensionLandings); err != nil {
		return err
	}
	// 常驻维度数量不再写死上限 3：用户明确要求「常驻维度数量又不一定是3」。
	// 上限放到 16（受账号数约束，实际能起多少台仍由 accounts/account_limit 卡）。
	if c.ResidentObservers < 0 || c.ResidentObservers > 16 {
		return fmt.Errorf("常驻维度数需0..16，0使用默认3维度；实际数量受可用账号限制")
	}
	if err := yyCaptureFleetStepsValidate(c.FleetSteps); err != nil {
		return err
	}
	if c.AccountLimit < 0 || c.AccountLimit > yyAccountProcessSlots || c.IdleSeconds < 0 || c.IdleSeconds > 3600 || c.PlayerLimit < 0 || c.PlayerLimit > 1000 {
		return fmt.Errorf("基础投放数量需0..32（0自动），空闲回收0..3600秒，真人上限0..1000（0为22）")
	}
	if actor.Scope == "read" {
		return fmt.Errorf("需要账号池控制权限")
	}
	if len(c.Accounts) == 0 && c.Enabled {
		return fmt.Errorf("请至少选择一个本人或明确共享的机器人账号")
	}
	seen := map[string]bool{}
	own := yyCaptureAvailable(actor)
	allowed := map[string]bool{"main": actor.ID == yyPoolBoundActor().ID}
	for _, a := range own {
		allowed[a.ID] = true
	}
	for _, id := range c.Accounts {
		if !allowed[id] || seen[id] {
			return fmt.Errorf("仅可选择本人或明确共享的有效账号，且不能重复")
		}
		seen[id] = true
	}
	seenDimensionAccounts := map[string]bool{}
	for _, dimension := range yyGlobalDimensionKeys(c) {
		id := c.DimensionAccounts[dimension]
		if id == "" {
			continue
		}
		if !seen[id] || seenDimensionAccounts[id] {
			return fmt.Errorf("维度机器人必须已选中，每个账号仅负责一个维度")
		}
		seenDimensionAccounts[id] = true
	}
	if _, e := yyWatchParse(yyGlobalWatchOptions(c)); e != nil {
		return e
	}
	return nil
}
func yyGlobalCaptureRoutes(mux *http.ServeMux, root string) {
	if yyAccounts.Worker {
		return
	}
	yyGlobalCapture.Lock()
	yyGlobalCapture.Path = filepath.Join(root, "global-capture.json")
	var saved struct {
		Config yyGlobalCaptureConfig
		Actor  yyPoolActor
	}
	if raw, e := os.ReadFile(yyGlobalCapture.Path); e == nil && json.Unmarshal(raw, &saved) == nil {
		yyGlobalCapture.Config = saved.Config
		if yyGlobalCapture.Config.DimensionRouting == nil {
			yes := true
			yyGlobalCapture.Config.DimensionRouting = &yes
		}
		yyGlobalCapture.Actor = saved.Actor
	}
	yyGlobalCapture.Unlock()
	mux.HandleFunc("/api/youyuan/global-capture", func(w http.ResponseWriter, r *http.Request) {
		actor, e := yyPoolActorRequest(r)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if actor.ID != yyPoolBoundActor().ID {
			writeJSON(w, map[string]any{"ok": false, "error": "只能配置自己设备的账号池"})
			return
		}
		if r.Method == "POST" {
			var patch map[string]json.RawMessage
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&patch) != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "配置格式无效"})
				return
			}
			yyGlobalCapture.Lock()
			previous := yyGlobalCapture.Config
			yyGlobalCapture.Unlock()
			base, _ := json.Marshal(previous)
			fields := map[string]json.RawMessage{}
			json.Unmarshal(base, &fields)
			for key, value := range patch {
				if _, known := fields[key]; !known {
					writeJSON(w, map[string]any{"ok": false, "error": "未知配置项：" + key})
					return
				}
				fields[key] = value
			}
			merged, _ := json.Marshal(fields)
			var c yyGlobalCaptureConfig
			if json.Unmarshal(merged, &c) != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "配置参数类型无效"})
				return
			}
			c = yyGlobalCaptureForActor(c, actor)
			if e := yyGlobalCaptureValidate(c, actor); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			raw, _ := json.MarshalIndent(map[string]any{"Config": c, "Actor": actor}, "", "  ")
			yyGlobalCapture.Lock()
			e := os.WriteFile(yyGlobalCapture.Path+".tmp", raw, 0600)
			if e == nil {
				e = os.Rename(yyGlobalCapture.Path+".tmp", yyGlobalCapture.Path)
			}
			if e == nil {
				yyGlobalCapture.Config = c
				yyGlobalCapture.Actor = actor
				yyGlobalCapture.Generation++
			}
			yyGlobalCapture.Unlock()
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "配置保存失败"})
				return
			}
		} else if r.Method != "GET" {
			http.Error(w, "GET/POST required", 405)
			return
		}
		yyGlobalCapture.Lock()
		defer yyGlobalCapture.Unlock()
		writeJSON(w, map[string]any{"ok": true, "config": yyGlobalCapture.Config, "advanced_allowed": actor.Admin, "states": yyGlobalCapture.States, "plan": yyGlobalCapture.Plan, "candidates": yyCaptureCandidates(actor), "dimension_assignments": yyGlobalDimensionAssignments(yyGlobalCapture.Config), "dimension_entries": yyGlobalDimensionEntries(yyGlobalCapture.Config, yyGlobalCapture.States), "dimension_limit": yyCaptureDimensionLimit, "single_switch_replaces_residents": false, "global_coverage_guaranteed": false, "note": "自动调度按当前真人名单与可验证维度分配；机器人全部排除。借调可能改变观察机身体维度；纯相机不跟随TP玩家。submitted仅为提交；未知维度明确待探测，不承诺零漏包。"})
	})
	yyGlobalCaptureLoopOnce.Do(func() { go yyGlobalCaptureRun() })
}

// Ordinary users manage their own authorized accounts and capacity, not transport internals.
func yyGlobalCaptureForActor(c yyGlobalCaptureConfig, actor yyPoolActor) yyGlobalCaptureConfig {
	if actor.Admin {
		return c
	}
	yes := true
	c.Mode = "camera"
	c.Interval, c.Height, c.Pitch, c.Yaw = .0005, 6, 90, 0
	c.PauseJobs, c.AutoPlayers = true, true
	c.Players = nil
	c.AutoFleet, c.AutoProvision, c.DimensionRouting = &yes, &yes, &yes
	c.ResidentObservers = 3
	return c
}
