package main

import (
	"bot-apk/config"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

func init() {
	for method, doc := range map[string]string{
		"accountsPool.captureStatus":    "无参数；读取宿主全局抓包config/plan/states/candidates。真实玩家维度未知不会猜测；active/submitted不是全局无漏包证明。",
		"accountsPool.captureRoster":    "options={include_bots:false}；只读本服当前名单和原生querytarget真实维度，用于诊断。include_bots=true仅诊断受管机器人，不改变生产观察排除规则，不传送/发镜头；维度缺失标unknown。",
		"accountsPool.captureConfigure": "patch对象；统一保存enabled/accounts/auto_fleet/auto_provision/account_limit/fleet_steps/idle_seconds/player_limit/enforce_player_limit和镜头参数。本人或明确shared_capture账号可选；自动1/2/3、>10为5、>=15为6台，数量非0为人工基础投入；fleet_steps=[{min_players:1,observers:1},...]可自定义递增人数阶梯，最多32台子进程，不限双机器人；无真人停镜头，专用采集进程60秒回收。分维度回防/借调/错峰，失败冷却替补。修改会影响本设备统一观察，不创建插件。",
	} {
		yyAccountPoolMethods[method] = true
		yyExtraMethods[method] = doc
	}
	yyExtraMethods["accountsPool.captureConfigure"] += " dimension_landings按0/1/2配置{x,y,z}（其他维度标识暂不支持落点坐标）；仅配置过的维度启用上线前救援，实际坐标及子连接确认后停止TP；无确认60秒停止子连接，states中landing:<id>可核对结果。"
	yyExtraMethods["accountsPool.captureConfigure"] += " game_mode设默认模式，dimension_game_modes按维度标识覆盖；keep保持当前。invisible/quick_tp/auto_respawn分别选择隐身、自动靠近和死亡自动重生；省略或auto_respawn=null沿用账号设置。"
	yyExtraMethods["accountsPool.captureConfigure"] += " dimension_accounts与dimension_game_modes支持字符串维度标识：0主世界/1下界/2末地老配置照旧，也接受 dm、minecraft:overworld 等自定义标识，最多8条，格式限1～40位小写字母数字与 _ : . -。当前运行时只能调度主世界/下界/末地，映射不到的维度保留配置、跳过并记一条warn，不会崩溃也不会猜维度。"
}

// Shared capture never grants access to another owner's files, token, console,
// plugin management or generic accountsPool.invoke.
func yyCaptureAvailable(actor yyPoolActor) []yyManagedAccount {
	yyAccounts.Lock()
	defer yyAccounts.Unlock()
	out := []yyManagedAccount{}
	for _, a := range yyAccounts.Items {
		if yyPoolOwner(a) == actor.ID || a.SharedCapture {
			out = append(out, a)
		}
	}
	return out
}
func yyCaptureAllowed(actor yyPoolActor, id string) bool {
	if id == "main" {
		return actor.ID == yyPoolBoundActor().ID
	}
	for _, a := range yyCaptureAvailable(actor) {
		if a.ID == id {
			return true
		}
	}
	return false
}
func yyCaptureCall(actor yyPoolActor, id, method string, args []any) (map[string]any, error) {
	if !yyCaptureAllowed(actor, id) {
		return nil, fmt.Errorf("账号未授予采集权限")
	}
	a, ok := yyManagedFind(id)
	if id == "main" || !ok || yyPoolOwner(a) == actor.ID {
		return yyObserverAccountCall(actor, id, method, args)
	}
	// Deliberately narrow shared-account capability.
	switch method {
	case "bot.self", "observerBridge.dimensionStatus", "observerBridge.assignDimension", "observerBridge.watchStatus", "observerBridge.watchStart", "observerBridge.watchStop":
	default:
		return nil, fmt.Errorf("共享账号仅允许观察调度")
	}
	r, e := yyChildJSON(a, "POST", "/api/youyuan/game/call", map[string]any{"method": method, "args": args})
	if e != nil {
		return nil, e
	}
	if r["ok"] != true {
		return nil, fmt.Errorf("采集账号: %v", r["error"])
	}
	v, _ := r["result"].(map[string]any)
	return v, nil
}

type yyCaptureLaunch struct {
	Busy     bool
	At       time.Time
	Attempts int
	Error    string
	IdleAt   time.Time
	Started  bool
	PermitAt time.Time
}

var yyCaptureLaunches = struct {
	sync.Mutex
	Items map[string]*yyCaptureLaunch
}{Items: map[string]*yyCaptureLaunch{}}

func yyCaptureGrantOP(actor yyPoolActor, id, name string, c yyGlobalCaptureConfig) {
	_, ok := yyManagedFind(id)
	// The caller only reaches here for accounts the user selected for capture.
	// Requiring CaptureOnly silently skipped manually added accounts, which then
	// never became ready and left a dimension (usually the End) uncovered.
	if !ok || !yyCaptureAllowed(actor, id) || !yyWatchName(name) || c.AutoProvision != nil && !*c.AutoProvision {
		return
	}
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() || !bm.IsOP() {
		return
	}
	yyCaptureLaunches.Lock()
	l := yyCaptureLaunches.Items[id]
	if l == nil {
		l = &yyCaptureLaunch{}
		yyCaptureLaunches.Items[id] = l
	}
	if time.Since(l.PermitAt) < 15*time.Second {
		yyCaptureLaunches.Unlock()
		return
	}
	l.PermitAt = time.Now()
	yyCaptureLaunches.Unlock()
	go func() { q, _ := json.Marshal(name); bm.SendWSCommandWithTimeout("op "+string(q), time.Second) }()
}

func yyCaptureProvision(actor yyPoolActor, id string, c yyGlobalCaptureConfig) map[string]any {
	if id == "main" || c.AutoProvision != nil && !*c.AutoProvision {
		return map[string]any{"phase": "offline", "auto_provision": false}
	}
	yyCaptureLaunches.Lock()
	for other, pending := range yyCaptureLaunches.Items {
		if !pending.Busy && pending.Started && !pending.At.IsZero() && time.Since(pending.At) >= 60*time.Second {
			// The old connection can still be authenticating for several minutes.
			// Hold the shared admission gate until its actual worker is stopped.
			pending.Busy = true
			yyCaptureLaunches.Unlock()
			a, exists := yyManagedFind(other)
			var stopErr error
			if exists && a.CaptureOnly {
				_, stopErr = yyPoolCall(yyPoolActor{ID: yyPoolOwner(a), Scope: "control"}, "accountsPool.stop", []any{other})
				if stopErr == nil && a.Slot > 0 {
					if _, alive := yyChildJSON(a, "GET", "/health", nil); alive == nil {
						stopErr = fmt.Errorf("旧采集内核仍响应，尚未确认退出")
					}
				}
			} else if exists {
				stopErr = fmt.Errorf("账号已转为业务用途，未自动停止")
			}
			yyCaptureLaunches.Lock()
			pending.Busy = false
			pending.At = time.Now()
			pending.Error = "采集连接60秒内未就绪，旧尝试已停止，冷却后再试"
			if stopErr == nil {
				pending.Started = false
			} else {
				pending.Error = "旧采集连接未停止，暂停替补: " + stopErr.Error()
			}
			message := pending.Error
			yyCaptureLaunches.Unlock()
			return map[string]any{"phase": "expired_login_cleanup", "pending_account": other, "error": message}
		}
		if pending.Busy || pending.Started && time.Since(pending.At) < 60*time.Second {
			yyCaptureLaunches.Unlock()
			return map[string]any{"phase": "waiting_previous_join", "pending_account": other}
		}
	}
	l := yyCaptureLaunches.Items[id]
	if l == nil {
		l = &yyCaptureLaunch{}
		yyCaptureLaunches.Items[id] = l
	}
	delay := time.Duration(15*(1<<min(l.Attempts, 4))) * time.Second
	if l.Busy || time.Since(l.At) < delay {
		r := map[string]any{"phase": "starting_or_cooldown", "error": l.Error, "attempts": l.Attempts, "retry_after_ms": max(0, int(time.Until(l.At.Add(delay)).Milliseconds()))}
		yyCaptureLaunches.Unlock()
		return r
	}
	l.Busy = true
	l.At = time.Now()
	l.Attempts++
	attempt := l.Attempts
	yyCaptureLaunches.Unlock()
	go func() {
		a, ok := yyManagedFind(id)
		var e error
		yyGlobalCapture.Lock()
		wanted := yyGlobalCapture.Config.Enabled
		stillSelected := false
		for _, candidate := range yyGlobalCapture.Config.Accounts {
			if candidate == id {
				stillSelected = true
			}
		}
		yyGlobalCapture.Unlock()
		if !wanted || !stillSelected {
			e = fmt.Errorf("投放配置已取消")
		} else if !ok || !yyCaptureAllowed(actor, id) {
			e = fmt.Errorf("账号已移除或未授权")
		} else {
			// Never replace another owner's bound/shared active assignment.
			yyPool.Lock()
			lease := yyPool.Leases[id]
			yyPool.Unlock()
			if lease.Phase == "bound" && lease.Owner != actor.ID {
				e = fmt.Errorf("共享机器人已被其他任务占用")
			} else {
				c := config.LoadConfig()
				a.Server = c.ServerCode
				a.Password = c.ServerPass
				if bm := getBotManagerLocked(); bm != nil {
					bm.mu.Lock()
					if bm.serverCode != "" {
						a.Server = bm.serverCode
					}
					bm.mu.Unlock()
				}
				a.CaptureOnly = true
				owner := yyPoolActor{ID: yyPoolOwner(a), Scope: "control"}
				_, e = yyPoolInvokeActor(owner, "POST", "/api/youyuan/accounts", map[string]any{"action": "save", "id": id, "account": a, "defer_connect": true})
				if e == nil {
					_, e = yyPoolCall(owner, "accountsPool.start", []any{id, map[string]any{"server": a.Server, "connect": true, "purpose": "global_capture"}})
				}
			}
		}
		yyCaptureLaunches.Lock()
		defer yyCaptureLaunches.Unlock()
		l.Busy = false
		if e != nil {
			l.Error = e.Error()
		} else {
			l.Started = true
			l.Error = "等待游戏连接和OP确认"
		}
	}()
	return map[string]any{"phase": "starting", "attempts": attempt}
}
func yyCaptureRetire(actor yyPoolActor, selected map[string]bool, c yyGlobalCaptureConfig) {
	idle := c.IdleSeconds
	if idle == 0 {
		idle = 60
	}
	for _, a := range yyCaptureAvailable(actor) {
		if !a.CaptureOnly {
			continue
		}
		yyCaptureLaunches.Lock()
		l := yyCaptureLaunches.Items[a.ID]
		if l == nil {
			l = &yyCaptureLaunch{Started: true}
			yyCaptureLaunches.Items[a.ID] = l
		}
		if selected[a.ID] {
			l.IdleAt = time.Time{}
			l.Attempts = 0
			l.Error = ""
			yyCaptureLaunches.Unlock()
			continue
		}
		// A released account must receive a fresh idle grace period when it
		// is next enabled. Keeping the old deadline killed new workers before
		// their authentication could finish.
		if !a.Enabled {
			l.IdleAt = time.Time{}
			yyCaptureLaunches.Unlock()
			continue
		}
		if l.IdleAt.IsZero() {
			l.IdleAt = time.Now()
		}
		due := time.Since(l.IdleAt) > time.Duration(idle)*time.Second && !l.Busy && a.Enabled
		if due {
			l.Busy = true
		}
		yyCaptureLaunches.Unlock()
		if due {
			go func(a yyManagedAccount, l *yyCaptureLaunch) {
				_, e := yyPoolCall(yyPoolActor{ID: yyPoolOwner(a), Scope: "control"}, "accountsPool.stop", []any{a.ID})
				yyCaptureLaunches.Lock()
				l.Busy = false
				l.IdleAt = time.Now()
				if e != nil {
					l.Error = e.Error()
				}
				yyCaptureLaunches.Unlock()
			}(a, l)
		}
	}
}

// yyCaptureIdentityAudit records why an account could not confirm its robot
// identity, so operators can tell "slot points at a dead kernel" apart from
// "kernel is alive but refuses to answer". Only the second case may hold back
// provisioning: a stale slot must never poison the whole roster.
type yyCaptureIdentityAudit struct {
	sync.Mutex
	// Accounts whose slot is set but whose child kernel is not answerable.
	// They carry no verified identity, so they are neither trusted as robots
	// nor used as a reason to suspend provisioning.
	StaleSlots []string
	// Connected accounts that did not report a usable name. These do hold
	// back provisioning, because an unidentified online entity could be a
	// real player that we would then re-admit and observe recursively.
	UnverifiedOnline []string
	CheckedAt        time.Time
}

func (a *yyCaptureIdentityAudit) snapshot() ([]string, []string) {
	a.Lock()
	defer a.Unlock()
	return append([]string{}, a.StaleSlots...), append([]string{}, a.UnverifiedOnline...)
}

var yyCaptureAuditLog yyCaptureIdentityAudit

var yyCaptureVerifiedNames = struct {
	sync.RWMutex
	Names []string
}{}

// Camera ticks use local exclusions; only reconciliation probes child kernels.
func yyCaptureRobotNames() []string {
	yyCaptureVerifiedNames.RLock()
	names := append([]string(nil), yyCaptureVerifiedNames.Names...)
	yyCaptureVerifiedNames.RUnlock()
	yyAccounts.Lock()
	for _, a := range yyAccounts.Items {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	yyAccounts.Unlock()
	if bm := getBotManagerLocked(); bm != nil {
		bm.mu.Lock()
		names = append(names, bm.display)
		bm.mu.Unlock()
	}
	return names
}

// yyCaptureRobotNamesWithStatus collects the machine-readable robot names that
// must be excluded from player detection, and reports whether every *online*
// account was able to confirm the identity it plays under.
//
// The contract matters: an account whose slot is set but whose kernel is dead
// (stopped, released, or crashed) is simply absent. Treating it as "identity
// unknown" pins identitiesKnown=false forever, which froze desired observers
// at zero and left every other robot offline. Only an account that is online
// yet cannot report its own name is allowed to clear the flag.
func yyCaptureRobotNamesWithStatus() ([]string, bool) {
	names := map[string]bool{}
	identitiesKnown := true
	yyAccounts.Lock()
	items := append([]yyManagedAccount{}, yyAccounts.Items...)
	yyAccounts.Unlock()
	staleSlots := []string{}
	unverifiedOnline := []string{}
	for _, a := range items {
		if profile := yyAccountProfileView(a.ID, a.Name); profile["nickname"] != nil {
			if n, ok := profile["nickname"].(string); ok && n != "" {
				names[n] = true
			}
		}
		if a.Name != "" {
			names[a.Name] = true
		}
	}
	if bm := getBotManagerLocked(); bm != nil {
		bm.mu.Lock()
		names[bm.display] = true
		bm.mu.Unlock()
	}
	// Active workers may use a game nickname different from the saved profile.
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, a := range items {
		if a.Slot < 1 {
			continue
		}
		wg.Add(1)
		go func(a yyManagedAccount) {
			defer wg.Done()
			// Probe first: a dead kernel is not a robot whose identity we
			// failed to learn, it is an account that is simply not in game.
			status, statusErr := yyChildJSON(a, "GET", "/api/bot/status", nil)
			if statusErr != nil || status["connected"] != true {
				mu.Lock()
				staleSlots = append(staleSlots, a.ID)
				mu.Unlock()
				return
			}
			r, e := yyChildJSON(a, "POST", "/api/youyuan/game/call", map[string]any{"method": "bot.self", "args": []any{}})
			if e == nil {
				v, _ := r["result"].(map[string]any)
				n, _ := v["name"].(string)
				if n != "" {
					mu.Lock()
					names[n] = true
					mu.Unlock()
				} else {
					mu.Lock()
					identitiesKnown = false
					unverifiedOnline = append(unverifiedOnline, a.ID)
					mu.Unlock()
				}
			} else {
				mu.Lock()
				identitiesKnown = false
				unverifiedOnline = append(unverifiedOnline, a.ID)
				mu.Unlock()
			}
		}(a)
	}
	wg.Wait()
	yyCaptureAuditLog.Lock()
	yyCaptureAuditLog.StaleSlots = staleSlots
	yyCaptureAuditLog.UnverifiedOnline = unverifiedOnline
	yyCaptureAuditLog.CheckedAt = time.Now()
	yyCaptureAuditLog.Unlock()
	out := []string{}
	for n := range names {
		if n != "" {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	yyCaptureVerifiedNames.Lock()
	yyCaptureVerifiedNames.Names = append([]string(nil), out...)
	yyCaptureVerifiedNames.Unlock()
	return out, identitiesKnown
}

func yyCaptureIdleStates(actor yyPoolActor, selected map[string]bool) map[string]any {
	out := map[string]any{}
	for _, a := range yyCaptureAvailable(actor) {
		if selected[a.ID] || !a.CaptureOnly {
			continue
		}
		yyCaptureLaunches.Lock()
		l := yyCaptureLaunches.Items[a.ID]
		v := map[string]any{"enabled": a.Enabled, "phase": "idle"}
		if !a.Enabled {
			v["phase"] = "released"
		}
		if l != nil {
			if !l.IdleAt.IsZero() {
				v["idle_since"] = l.IdleAt.Format(time.RFC3339Nano)
			}
			v["error"] = l.Error
			if l.Busy {
				v["phase"] = "starting_or_releasing"
			}
		}
		yyCaptureLaunches.Unlock()
		out[a.ID] = v
	}
	return out
}

func yyCaptureCandidates(actor yyPoolActor) []map[string]any {
	out := []map[string]any{}
	if actor.ID == yyPoolBoundActor().ID {
		out = append(out, map[string]any{"id": "main", "name": "当前主机器人", "source": "own", "capture_only": false})
	}
	for _, a := range yyCaptureAvailable(actor) {
		kind := "own"
		if yyPoolOwner(a) != actor.ID {
			kind = "shared"
		}
		out = append(out, map[string]any{"id": a.ID, "name": a.Name, "source": kind, "capture_only": a.CaptureOnly, "enabled": a.Enabled, "has_token": a.Token != ""})
	}
	return out
}
func yyCapturePlayers(excluded []string) ([]yyCapturePlayer, string) {
	ex := map[string]bool{}
	for _, n := range excluded {
		ex[n] = true
	}
	// Copy under lock; GetAllPlayers returns pointers and is unsafe to retain
	// while the packet reader updates identity/coordinates.
	playerInfoMgr.mu.RLock()
	byID := map[int64]string{}
	byUUID := map[string]string{}
	all := map[string]yyCapturePlayer{}
	for _, p := range playerInfoMgr.players {
		if p.Online && !ex[p.Name] {
			all[p.Name] = yyCapturePlayer{Name: p.Name}
			byID[p.EntityID] = p.Name
			byUUID[p.UUID] = p.Name
		}
	}
	playerInfoMgr.mu.RUnlock()
	if len(all) == 0 {
		return []yyCapturePlayer{}, ""
	}
	errText := ""
	bm := getBotManagerLocked()
	if bm != nil && bm.IsConnected() {
		// NetEase execute can return success with an empty data set. Query
		// directly so the native querytarget response retains details/parameters.
		p, timed, e := bm.SendWSCommandWithTimeout("querytarget @a", 2*time.Second)
		if e != nil {
			errText = e.Error()
		} else if timed || p == nil {
			errText = "维度查询超时，未知玩家保留为待探测"
		} else {
			rows := yyCaptureQueryRows(p)
			for _, r := range rows {
				n := byUUID[r.PlayerUUID.String()]
				if n == "" {
					n = byID[r.PlayerUniqueID]
				}
				if _, ok := all[n]; ok && yyDimensionIDValid(int32(r.Dimension)) {
					all[n] = yyCapturePlayer{Name: n, Dimension: int32(r.Dimension), Known: true}
				}
			}
			if len(rows) == 0 {
				errText = "服务器未返回可验证的玩家维度；执行同维度探测，不声称已定位"
			}
		}
	}
	out := []yyCapturePlayer{}
	for _, p := range all {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, errText
}
