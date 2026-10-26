package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// This is account orchestration, not a new game authentication provider. No
// loop here teleports players, scans entities, or changes command throughput.
var yyAccountPoolMethods = map[string]bool{}

func init() {
	for _, name := range []string{"list", "add", "start", "stop", "status", "bind", "acquire", "release", "bindings", "invoke"} {
		yyAccountPoolMethods["accountsPool."+name] = true
	}
}

type yyPoolLease struct {
	Account   string `json:"account"`
	Owner     string `json:"owner_id"`
	Player    string `json:"player"`
	Server    string `json:"server"`
	Phase     string `json:"phase"`
	Updated   int64  `json:"updated_at"`
	IdleUntil int64  `json:"idle_until,omitempty"`
	Error     string `json:"error,omitempty"`
}
type yyPoolChildIdentity struct {
	Source string `json:"source"`
	Secret string `json:"secret"`
	Owner  string `json:"owner"`
}

var yyPool = struct {
	sync.Mutex
	Path   string
	Leases map[string]yyPoolLease
	Keys   map[string]string
}{Leases: map[string]yyPoolLease{}, Keys: map[string]string{}}
var yyPoolMutationMu sync.Mutex

func yyPoolForgetAccount(id string) error {
	yyPool.Lock()
	defer yyPool.Unlock()
	delete(yyPool.Keys, id)
	delete(yyPool.Leases, id)
	return yyPoolSaveLocked()
}

type yyPoolActorContext struct{}

func yyPoolHasActor(r *http.Request) bool {
	return r.Header.Get("X-YouYuan-Actor") != "" || r.Context().Value(yyPoolActorContext{}) != nil
}
func yyPoolSaveLocked() error {
	raw, e := json.MarshalIndent(map[string]any{"version": 1, "leases": yyPool.Leases, "keys": yyPool.Keys}, "", "  ")
	if e != nil {
		return e
	}
	if yyPool.Path == "" {
		yyPool.Path = filepath.Join(yyAccounts.Root, "account-pool.json")
	}
	if e = os.WriteFile(yyPool.Path+".tmp", raw, 0600); e != nil {
		return e
	}
	return os.Rename(yyPool.Path+".tmp", yyPool.Path)
}
func yyPoolLoad(root string) {
	yyPool.Lock()
	defer yyPool.Unlock()
	yyPool.Path = filepath.Join(root, "account-pool.json")
	var data struct {
		Leases map[string]yyPoolLease `json:"leases"`
		Keys   map[string]string      `json:"keys"`
	}
	raw, _ := os.ReadFile(yyPool.Path)
	if json.Unmarshal(raw, &data) == nil {
		if data.Leases != nil {
			yyPool.Leases = data.Leases
		}
		if data.Keys != nil {
			yyPool.Keys = data.Keys
		}
	}
}
func yyPoolWriteChildIdentity(a yyManagedAccount, dir string) error {
	yyPool.Lock()
	key := yyPool.Keys[a.ID]
	if key == "" {
		key = yyNewKey()
		yyPool.Keys[a.ID] = key
		if e := yyPoolSaveLocked(); e != nil {
			yyPool.Unlock()
			return e
		}
	}
	yyPool.Unlock()
	raw, _ := json.Marshal(yyPoolChildIdentity{Source: a.ID, Secret: key, Owner: yyPoolOwner(a)})
	return os.WriteFile(filepath.Join(dir, "pool-parent.json"), raw, 0600)
}
func yyPoolChildActor(r *http.Request) (yyPoolActor, error) {
	id, secret := r.Header.Get("X-YouYuan-Pool-Source"), r.Header.Get("X-YouYuan-Pool-Secret")
	a, ok := yyManagedFind(id)
	yyPool.Lock()
	key := yyPool.Keys[id]
	yyPool.Unlock()
	if !ok || key == "" || !yyPoolEqualKey(key, secret) {
		return yyPoolActor{}, fmt.Errorf("子机器人账号池身份无效")
	}
	return yyPoolActor{ID: yyPoolOwner(a), Scope: "control"}, nil
}
func yyPoolInvokeActor(actor yyPoolActor, method, path string, body any) (any, error) {
	raw, e := json.Marshal(body)
	if e != nil {
		return nil, e
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r = r.WithContext(context.WithValue(r.Context(), yyPoolActorContext{}, actor))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	yyMux.ServeHTTP(w, r)
	var result map[string]any
	if json.Unmarshal(w.Body.Bytes(), &result) != nil {
		return nil, fmt.Errorf("账号 API 返回 %d", w.Code)
	}
	if w.Code >= 400 || result["ok"] == false {
		return nil, fmt.Errorf("%v", result["error"])
	}
	return result, nil
}
func yyPoolVisible(actor yyPoolActor, all bool) ([]yyManagedAccount, error) {
	if actor.ID == "" {
		return nil, fmt.Errorf("请先绑定游园平台账号")
	}
	if all && !actor.Admin {
		return nil, fmt.Errorf("全部账号池仅限项目管理员")
	}
	yyAccounts.Lock()
	items := append([]yyManagedAccount(nil), yyAccounts.Items...)
	yyAccounts.Unlock()
	out := []yyManagedAccount{}
	for _, a := range items {
		if yyPoolOwner(a) == actor.ID || all && actor.Admin {
			out = append(out, a)
		}
	}
	return out, nil
}
func yyPoolRequire(actor yyPoolActor, id string) (yyManagedAccount, error) {
	a, ok := yyManagedFind(id)
	if !ok || !yyPoolCanUse(actor, a) {
		return a, fmt.Errorf("该机器人不在你的账号池中")
	}
	return a, nil
}

// yyPoolMaskToken 生成列表里可以公开显示的脱敏令牌。规则与普通凭证一致：
// 默认只保留前 4 位和后 4 位，不足 12 位整体打码，避免从长度反推出内容。
func yyPoolMaskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) < 12 {
		return "••••••••"
	}
	return token[:4] + "••••" + token[len(token)-4:]
}

// yyPoolMinecraftName 判断一个名字能否当作 Minecraft 玩家名用于头像兜底。
func yyPoolMinecraftName(name string) bool {
	if len(name) < 3 || len(name) > 16 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

// yyPoolLocalGameName 从子账号自己的 config.json 读 bot_name；读不到再用账号名兜底。
// 只服务于头像显示，不参与任何身份或权限判断。
func yyPoolLocalGameName(a yyManagedAccount) string {
	if raw, e := os.ReadFile(filepath.Join(yyAccountDir(a.ID), "config.json")); e == nil {
		var cfg struct {
			BotName string `json:"bot_name"`
		}
		if json.Unmarshal(raw, &cfg) == nil {
			if name := strings.TrimSpace(cfg.BotName); yyPoolMinecraftName(name) {
				return name
			}
		}
	}
	if name := strings.TrimSpace(a.Name); yyPoolMinecraftName(name) {
		return name
	}
	return ""
}

// yyPoolChildGameIdentity 只在内核运行中时问子进程要当前游戏名和 UUID，供头像使用。
// 内核没起来或 700ms 内没答复就退回本地兜底，绝不拖住账号池列表。
func yyPoolChildGameIdentity(a yyManagedAccount) (name, uuid string) {
	if a.Slot < 1 || a.Slot > yyAccountProcessSlots {
		return "", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	response, e := yyChildRequest(ctx, a, "GET", "/api/youyuan/sense/snapshot", nil, "")
	if e != nil {
		return "", ""
	}
	defer response.Body.Close()
	var data struct {
		Scene struct {
			Bot struct {
				Name      string `json:"name"`
				UUID      string `json:"uuid"`
				UUIDKnown bool   `json:"uuid_known"`
			} `json:"bot"`
		} `json:"scene"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&data) != nil {
		return "", ""
	}
	name, uuid = strings.TrimSpace(data.Scene.Bot.Name), strings.TrimSpace(data.Scene.Bot.UUID)
	if !data.Scene.Bot.UUIDKnown || uuid == "00000000-0000-0000-0000-000000000000" {
		uuid = ""
	}
	if !yyPoolMinecraftName(name) {
		name = ""
	}
	return name, uuid
}

// yyPoolRememberedGameIdentity 读回上次记住的游戏名与 UUID。
// 内核停掉以后头像仍然可用，不必为了头像去拉起进程。
func yyPoolRememberedGameIdentity(id string) (name, uuid string) {
	if yyAccounts.Root == "" {
		return "", ""
	}
	raw, e := os.ReadFile(filepath.Join(yyAccountDir(id), "avatar-identity.json"))
	if e != nil {
		return "", ""
	}
	var data struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
	}
	if json.Unmarshal(raw, &data) != nil {
		return "", ""
	}
	name, uuid = strings.TrimSpace(data.Name), strings.TrimSpace(data.UUID)
	if !yyPoolMinecraftName(name) {
		name = ""
	}
	if uuid == "00000000-0000-0000-0000-000000000000" {
		uuid = ""
	}
	return name, uuid
}

// yyPoolRememberGameIdentity 只缓存游戏名和 UUID 这两个公开字段，值没变就不落盘。
func yyPoolRememberGameIdentity(id, name, uuid string) {
	if (name == "" && uuid == "") || yyAccounts.Root == "" {
		return
	}
	if oldName, oldUUID := yyPoolRememberedGameIdentity(id); oldName == name && oldUUID == uuid {
		return
	}
	dir := yyAccountDir(id)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return
	}
	raw, e := json.Marshal(map[string]any{"name": name, "uuid": uuid, "updated_at": time.Now().UnixMilli()})
	if e != nil {
		return
	}
	path := filepath.Join(dir, "avatar-identity.json")
	if e = os.WriteFile(path+".tmp", raw, 0600); e != nil {
		return
	}
	_ = os.Rename(path+".tmp", path)
}

// yyPoolLocalGameIdentity 只认用户为这个账号明确填过的机器人名。
//
// 为什么不复用 yyPoolLocalGameName：那个函数最后会退回账号备注名 a.Name。
// 备注名是给人看的标签，可能是"小号1"，也可能是随手打的"test1"；
// 而 minotar / crafatar 对任何合法英文名都返回一张图（不校验是不是真实正版玩家），
// 于是拿备注名去查，头像就变成一个毫不相干的陌生人。2026-10-23 用户看到的
// "一大堆奇葩头像"就是这个原因。所以这里只把确认过的那部分交出去。
func yyPoolLocalGameIdentity(a yyManagedAccount) (name string, confirmed bool) {
	if raw, e := os.ReadFile(filepath.Join(yyAccountDir(a.ID), "config.json")); e == nil {
		var cfg struct {
			BotName string `json:"bot_name"`
		}
		if json.Unmarshal(raw, &cfg) == nil {
			if n := strings.TrimSpace(cfg.BotName); yyPoolMinecraftName(n) {
				return n, true
			}
		}
	}
	return "", false
}

// yyPoolAvatarIdentity 汇总头像要用的名字与 UUID：
// 内核在线时问子进程并记住结果；离线时用上次记住的，再退回账号备注名。
func yyPoolAvatarIdentity(a yyManagedAccount) (name, uuid string) {
	if name, uuid = yyPoolChildGameIdentity(a); name != "" || uuid != "" {
		yyPoolRememberGameIdentity(a.ID, name, uuid)
		return name, uuid
	}
	if name, uuid = yyPoolRememberedGameIdentity(a.ID); name != "" || uuid != "" {
		return name, uuid
	}
	return yyPoolLocalGameName(a), ""
}

func yyPoolOptions(args []any, index int) (map[string]any, error) {
	if len(args) <= index || args[index] == nil {
		return map[string]any{}, nil
	}
	v, ok := args[index].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("参数需要对象")
	}
	return v, nil
}
func yyPoolString(v map[string]any, k string) string {
	s, _ := v[k].(string)
	return strings.TrimSpace(s)
}
func yyPoolBindLocked(actor yyPoolActor, a yyManagedAccount, o map[string]any) (yyPoolLease, error) {
	player, server := yyPoolString(o, "player"), yyPoolString(o, "server")
	if server == "" {
		server = a.Server
	}
	if player == "" || len(player) > 128 || strings.ContainsAny(player, "\r\n\x00") || server == "" {
		return yyPoolLease{}, fmt.Errorf("绑定需要有效的 player 和 server")
	}
	old, exists := yyPool.Leases[a.ID]
	if exists && old.Phase == "bound" && (old.Player != player || old.Server != server) {
		return old, fmt.Errorf("机器人已有绑定；先 release 后重新绑定")
	}
	for id, lease := range yyPool.Leases {
		if id != a.ID && lease.Owner == yyPoolOwner(a) && lease.Phase == "bound" && lease.Player == player && lease.Server == server {
			return lease, fmt.Errorf("玩家已绑定机器人 %s", id)
		}
	}
	lease := yyPoolLease{Account: a.ID, Owner: yyPoolOwner(a), Player: player, Server: server, Phase: "bound", Updated: time.Now().UnixMilli()}
	yyPool.Leases[a.ID] = lease
	if e := yyPoolSaveLocked(); e != nil {
		if exists {
			yyPool.Leases[a.ID] = old
		} else {
			delete(yyPool.Leases, a.ID)
		}
		return old, e
	}
	return lease, nil
}
func yyPoolCall(actor yyPoolActor, method string, args []any) (any, error) {
	if !yyAccountPoolMethods[method] {
		return nil, fmt.Errorf("未知账号池方法")
	}
	op := strings.TrimPrefix(method, "accountsPool.")
	read := op == "list" || op == "bindings" || op == "status" || op == "observationPlan" || op == "captureStatus" || op == "captureRoster"
	if !read && actor.Scope == "read" {
		return nil, fmt.Errorf("此授权仅可读取账号池")
	}
	if actor.ID == "" {
		return nil, fmt.Errorf("请先绑定游园平台账号")
	}
	if op == "captureStatus" {
		return yyPoolInvokeActor(actor, "GET", "/api/youyuan/global-capture", nil)
	}
	if op == "captureRoster" {
		if actor.ID != yyPoolBoundActor().ID {
			return nil, fmt.Errorf("只读取自己设备的观察名单")
		}
		o, e := yyPoolOptions(args, 0)
		if e != nil {
			return nil, e
		}
		include, _ := o["include_bots"].(bool)
		excluded := yyCaptureRobotNames()
		if include {
			excluded = nil
		}
		players, reason := yyCapturePlayers(excluded)
		return map[string]any{"players": players, "query_error": reason, "diagnostic_only": true, "includes_managed_bots": include, "observations_started": false}, nil
	}
	if op == "captureConfigure" {
		o, e := yyPoolOptions(args, 0)
		if e != nil {
			return nil, e
		}
		return yyPoolInvokeActor(actor, "POST", "/api/youyuan/global-capture", o)
	}
	if op == "observationPlan" || op == "handoff" {
		return yyPoolObservationCall(actor, method, args)
	}
	if op == "list" || op == "bindings" || op == "acquire" {
		o, e := yyPoolOptions(args, 0)
		if e != nil {
			return nil, e
		}
		all, _ := o["all"].(bool)
		items, e := yyPoolVisible(actor, all)
		if e != nil {
			return nil, e
		}
		if op == "list" {
			// 头像要用的游戏名/UUID 只能问子内核。先把租约拷出来解锁，
			// 避免在持锁状态下做网络请求；明文令牌永不出现在列表里。
			yyPool.Lock()
			leases := make(map[string]yyPoolLease, len(yyPool.Leases))
			for id, lease := range yyPool.Leases {
				leases[id] = lease
			}
			yyPool.Unlock()
			rows := make([]map[string]any, len(items))
			var wg sync.WaitGroup
			for i, a := range items {
				wg.Add(1)
				go func(i int, a yyManagedAccount) {
					defer wg.Done()
					name, uuid := yyPoolAvatarIdentity(a)
					rows[i] = map[string]any{"id": a.ID, "name": a.Name, "owner_id": yyPoolOwner(a), "server": a.Server, "enabled": a.Enabled, "keep_alive": a.KeepAlive, "has_token": a.Token != "", "token_masked": yyPoolMaskToken(a.Token), "player_name": name, "uuid": uuid, "lease": leases[a.ID]}
				}(i, a)
			}
			wg.Wait()
			return map[string]any{"accounts": rows, "process_slots": yyAccountProcessSlots}, nil
		}
		yyPoolMutationMu.Lock()
		defer yyPoolMutationMu.Unlock()
		yyPool.Lock()
		defer yyPool.Unlock()
		if op == "bindings" {
			rows := []yyPoolLease{}
			for _, a := range items {
				if l, ok := yyPool.Leases[a.ID]; ok {
					rows = append(rows, l)
				}
			}
			return rows, nil
		}
		player, server := yyPoolString(o, "player"), yyPoolString(o, "server")
		wanted := yyPoolString(o, "id")
		for _, a := range items {
			l, ok := yyPool.Leases[a.ID]
			if ok && l.Phase == "bound" && l.Player == player && l.Server == server && (wanted == "" || wanted == a.ID) {
				return l, nil
			}
		}
		// Reuse idle observers first; existing bindings never change silently.
		for pass := 0; pass < 2; pass++ {
			for _, a := range items {
				if wanted != "" && a.ID != wanted {
					continue
				}
				l, exists := yyPool.Leases[a.ID]
				if a.Token == "" || exists && l.Phase == "bound" {
					continue
				}
				if pass == 0 && (!exists || l.Phase != "idle") {
					continue
				}
				return yyPoolBindLocked(actor, a, o)
			}
		}
		return nil, fmt.Errorf("账号池没有可用机器人，请添加自己的独立游戏账号；不会重复登录同一个Token")
	}
	if op == "add" {
		o, e := yyPoolOptions(args, 0)
		if e != nil {
			return nil, e
		}
		id := yyPoolString(o, "id")
		if !yyPoolAccountID(id) {
			return nil, fmt.Errorf("需要独立账号 id")
		}
		if a, ok := yyManagedFind(id); ok && !yyPoolCanUse(actor, a) {
			return nil, fmt.Errorf("账号 id 已被占用")
		}
		return yyPoolInvokeActor(actor, "POST", "/api/youyuan/accounts", map[string]any{"action": "save", "id": id, "account": o})
	}
	id, e := yySDKString(args, 0)
	if e != nil {
		return nil, e
	}
	a, e := yyPoolRequire(actor, id)
	if e != nil {
		return nil, e
	}
	if op == "status" {
		return yyAccountSummary(a), nil
	}
	o, e := yyPoolOptions(args, 1)
	if e != nil {
		return nil, e
	}
	if op == "invoke" {
		path := yyPoolString(o, "path")
		verb := strings.ToUpper(yyPoolString(o, "method"))
		if verb == "" {
			verb = "GET"
		}
		if !strings.HasPrefix(path, "/api/") || strings.ContainsAny(path, "?#\\") || strings.Contains(path, "..") || strings.HasPrefix(path, "/api/youyuan/accounts") {
			return nil, fmt.Errorf("仅可调用此机器人自身的 /api/ 路径；账号编排使用 accountsPool")
		}
		if actor.Scope == "read" && verb != "GET" {
			return nil, fmt.Errorf("需要控制授权")
		}
		if verb != "GET" && verb != "POST" && verb != "DELETE" {
			return nil, fmt.Errorf("method 需要 GET/POST/DELETE")
		}
		query, _ := o["query"].(map[string]any)
		status, raw, e := yyInvokeWithHeaders(verb, yyAccountsPrefix+id+path, query, o["body"], nil)
		if e != nil {
			return nil, e
		}
		var value any
		if json.Unmarshal(raw, &value) != nil {
			return nil, fmt.Errorf("目标返回 %d", status)
		}
		if status >= 400 {
			return nil, fmt.Errorf("目标返回 %d：%v", status, value)
		}
		return value, nil
	}
	yyPoolMutationMu.Lock()
	defer yyPoolMutationMu.Unlock()
	switch op {
	case "bind":
		yyPool.Lock()
		defer yyPool.Unlock()
		return yyPoolBindLocked(actor, a, o)
	case "release":
		// 托管账号不设时限（用户明确要求）：未显式给出 idle_seconds 时
		// IdleUntil 保持 0，表示永不自动回收；只有调用方自己要求宽限期时
		// 才写入截止时间，回收逻辑留给那类租约。
		seconds, explicit := 60.0, false
		if n, ok := o["idle_seconds"].(float64); ok {
			seconds, explicit = n, true
		}
		if seconds < 0 || seconds > 86400 {
			return nil, fmt.Errorf("idle_seconds 需要 0..86400")
		}
		yyPool.Lock()
		old, exists := yyPool.Leases[id]
		lease := yyPoolLease{Account: id, Owner: yyPoolOwner(a), Server: a.Server, Phase: "idle", Updated: time.Now().UnixMilli()}
		if explicit {
			lease.IdleUntil = time.Now().Add(time.Duration(seconds * float64(time.Second))).UnixMilli()
		}
		yyPool.Leases[id] = lease
		e = yyPoolSaveLocked()
		if e != nil {
			if exists {
				yyPool.Leases[id] = old
			} else {
				delete(yyPool.Leases, id)
			}
		}
		yyPool.Unlock()
		return lease, e
	case "start":
		// A plugin explicitly taking over an account is not an idle capture
		// reservation. Do not let global idle reclamation fight its keep-alive.
		a.CaptureOnly = yyPoolString(o, "purpose") == "global_capture"
		if server := yyPoolString(o, "server"); server != "" {
			a.Server = server
		}
		if p, ok := o["password"].(string); ok {
			a.Password = p
		}
		a.KeepAlive = true
		yyPool.Lock()
		lease, bound := yyPool.Leases[id]
		yyPool.Unlock()
		if bound && lease.Phase == "bound" && lease.Server != a.Server {
			return nil, fmt.Errorf("目标服务器与现有绑定不同，请先解除或调整绑定")
		}
		if _, e = yyPoolInvokeActor(actor, "POST", "/api/youyuan/accounts", map[string]any{"action": "save", "id": id, "account": a, "defer_connect": true}); e != nil {
			return nil, e
		}
		action := "connect"
		if connect, ok := o["connect"].(bool); ok && !connect {
			action = "engine_start"
		}
		return yyPoolInvokeActor(actor, "POST", "/api/youyuan/accounts", map[string]any{"action": action, "id": id})
	case "stop":
		result, e := yyPoolInvokeActor(actor, "POST", "/api/youyuan/accounts", map[string]any{"action": "disable", "id": id})
		if e != nil {
			return nil, e
		}
		yyPool.Lock()
		defer yyPool.Unlock()
		delete(yyPool.Leases, id)
		return result, yyPoolSaveLocked()
	}
	return nil, fmt.Errorf("未知操作")
}
func yyPoolExpire(now int64) {
	yyPoolMutationMu.Lock()
	defer yyPoolMutationMu.Unlock()
	yyPool.Lock()
	expired := []yyPoolLease{}
	for _, l := range yyPool.Leases {
		// 托管账号不设时限，用户明确要求：IdleUntil 为 0 的租约永不自动回收。
		// 只有调用方显式要过宽限期的租约才进入回收，回收代码本身保留。
		if l.Phase == "idle" && l.IdleUntil > 0 && l.IdleUntil <= now {
			expired = append(expired, l)
		}
	}
	yyPool.Unlock()
	for _, l := range expired {
		actor := yyPoolActor{ID: l.Owner, Scope: "control"}
		_, e := yyPoolInvokeActor(actor, "POST", "/api/youyuan/accounts", map[string]any{"action": "disable", "id": l.Account})
		yyPool.Lock()
		if e != nil {
			l.Phase = "error"
			l.Error = e.Error()
			yyPool.Leases[l.Account] = l
		} else {
			delete(yyPool.Leases, l.Account)
		}
		_ = yyPoolSaveLocked()
		yyPool.Unlock()
	}
}
func yyAccountPoolRoutes(mux *http.ServeMux, root string) {
	yyGlobalCaptureRoutes(mux, root)
	if !yyAccounts.Worker {
		yyPoolLoad(root)
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for t := range ticker.C {
				yyPoolExpire(t.UnixMilli())
			}
		}()
	}
	mux.HandleFunc("/api/youyuan/account-pool", func(w http.ResponseWriter, r *http.Request) {
		if yyAccounts.Worker {
			http.Error(w, "账号池在主控制端调用，插件请使用 accountsPool", 400)
			return
		}
		actor, e := yyPoolActorRequest(r)
		if e != nil {
			http.Error(w, e.Error(), 403)
			return
		}
		var p struct {
			Action string `json:"action"`
			Args   []any  `json:"args"`
		}
		if r.Method == "GET" {
			p.Action = r.URL.Query().Get("action")
			if p.Action == "" {
				p.Action = "list"
			}
			if p.Action != "list" && p.Action != "bindings" {
				http.Error(w, "GET 仅可查询 list/bindings", 405)
				return
			}
			p.Args = []any{map[string]any{"all": r.URL.Query().Get("all") == "true"}}
		} else if r.Method == "POST" {
			if e = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); e != nil {
				http.Error(w, "请求格式无效", 400)
				return
			}
		} else {
			http.Error(w, "GET/POST required", 405)
			return
		}
		result, e := yyPoolCall(actor, "accountsPool."+p.Action, p.Args)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "result": result})
	})
	// 托管账号的明文令牌只在用户点「显示」时按需读取：
	// 只允许 GET，id 必须通过账号池校验，走与其它账号池操作相同的身份判定，
	// 不写任何日志，响应禁止缓存。
	mux.HandleFunc("/api/youyuan/accounts/pool/secret", func(w http.ResponseWriter, r *http.Request) {
		if yyAccounts.Worker {
			http.Error(w, "账号池在主控制端调用", 400)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" {
			http.Error(w, "GET required", 405)
			return
		}
		actor, e := yyPoolActorRequest(r)
		if e != nil {
			http.Error(w, e.Error(), 403)
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if !yyPoolAccountID(id) {
			http.Error(w, "账号 ID 无效", 400)
			return
		}
		a, e := yyPoolRequire(actor, id)
		if e != nil {
			http.Error(w, e.Error(), 403)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": a.ID, "token": a.Token})
	})
}
func yyPoolSDKCall(method string, args []any) (any, error) {
	if !yyAccounts.Worker {
		v, e := yyPoolCall(yyPoolBoundActor(), method, args)
		if e != nil {
			return nil, e
		}
		// Account orchestration is a low-frequency control plane. Normalise native
		// structs once here so Lua/Python/C++ receive objects, never Go string dumps.
		raw, e := json.Marshal(v)
		if e != nil {
			return nil, e
		}
		var result any
		e = json.Unmarshal(raw, &result)
		return result, e
	}
	raw, e := os.ReadFile(filepath.Join(yyAccounts.Root, "pool-parent.json"))
	var identity yyPoolChildIdentity
	if e != nil || json.Unmarshal(raw, &identity) != nil {
		return nil, fmt.Errorf("账号池身份未初始化，请重新启动这个机器人内核")
	}
	port, e := os.ReadFile(filepath.Join(yyAccounts.Root, "parent-port"))
	if e != nil {
		return nil, e
	}
	body, _ := json.Marshal(map[string]any{"action": strings.TrimPrefix(method, "accountsPool."), "args": args})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:"+strings.TrimSpace(string(port))+"/api/youyuan/account-pool", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-YouYuan-Pool-Source", identity.Source)
	r.Header.Set("X-YouYuan-Pool-Secret", identity.Secret)
	response, e := http.DefaultClient.Do(r)
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	var data struct {
		OK     bool   `json:"ok"`
		Result any    `json:"result"`
		Error  string `json:"error"`
	}
	if e = json.NewDecoder(response.Body).Decode(&data); e != nil {
		return nil, e
	}
	if !data.OK {
		return nil, fmt.Errorf("%s", data.Error)
	}
	return data.Result, nil
}
func registerAccountPoolAPI(L *lua.LState) {
	table := L.NewTable()
	L.SetGlobal("accountsPool", table)
	for name := range yyAccountPoolMethods {
		method := name
		L.SetField(table, strings.TrimPrefix(name, "accountsPool."), L.NewFunction(func(L *lua.LState) int {
			args := []any{}
			for i := 1; i <= L.GetTop(); i++ {
				args = append(args, luaToGo(L.Get(i)))
			}
			result, e := yyLuaWaitValue(L, func(_ context.Context) (any, error) { return yyPoolSDKCall(method, args) })
			if e != nil {
				L.RaiseError("%s: %v", method, e)
				return 0
			}
			L.Push(goToLua(L, result))
			return 1
		}))
	}
}
