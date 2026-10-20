package main

import (
	"bot-apk/config"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type yyManagedAccount struct {
	ID            string `json:"id"`
	OwnerID       string `json:"owner_id,omitempty"`
	Name          string `json:"name"`
	Token         string `json:"token,omitempty"`
	AuthURL       string `json:"auth_url"`
	Server        string `json:"server"`
	Password      string `json:"password,omitempty"`
	Enabled       bool   `json:"enabled"`
	KeepAlive     bool   `json:"keep_alive"`
	Slot          int    `json:"slot"`
	SharedCapture bool   `json:"shared_capture"`
	CaptureOnly   bool   `json:"capture_only"`
}

var yyAccounts = struct {
	sync.Mutex
	Items  []yyManagedAccount
	Root   string
	Worker bool
}{Items: []yyManagedAccount{}}
var yyAccountMutationMu sync.Mutex

const yyAccountsPrefix = "/api/youyuan/accounts/invoke/"
const yyAccountProcessSlots = 32

// Stored profiles do not consume a process. Acquire a slot only when starting.
func yyAccountAllocateLocked(a *yyManagedAccount) error {
	if a.Slot > 0 {
		return nil
	}
	used := map[int]bool{}
	for _, other := range yyAccounts.Items {
		if other.Slot > 0 {
			used[other.Slot] = true
		}
	}
	for slot := 1; slot <= yyAccountProcessSlots; slot++ {
		if !used[slot] {
			a.Slot = slot
			return nil
		}
	}
	return fmt.Errorf("32 个独立内核正在运行，请关闭一个账号释放运行位置；已保存的账号不会被删除")
}

func yyManagedAccountID(id string) bool {
	if !yyWorkspaceID(id) {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
func yyAccountDir(id string) string { return filepath.Join(yyAccounts.Root, "managed-accounts", id) }
func yyAccountAddress(account yyManagedAccount) string {
	return fmt.Sprintf("http://127.0.0.1:%d", 19500+account.Slot)
}
func yyManagedSaveLocked() error {
	data, e := json.MarshalIndent(yyAccounts.Items, "", "  ")
	if e != nil {
		return e
	}
	path := filepath.Join(yyAccounts.Root, "managed-accounts.json")
	temp := path + ".tmp"
	if e = os.WriteFile(temp, data, 0600); e != nil {
		return e
	}
	return os.Rename(temp, path)
}
func yyManagedFind(id string) (yyManagedAccount, bool) {
	yyAccounts.Lock()
	defer yyAccounts.Unlock()
	for _, a := range yyAccounts.Items {
		if a.ID == id {
			return a, true
		}
	}
	return yyManagedAccount{}, false
}
func yyChildRequest(ctx context.Context, a yyManagedAccount, method, path string, body []byte, contentType string, extra ...http.Header) (*http.Response, error) {
	if a.Slot < 1 || a.Slot > yyAccountProcessSlots {
		return nil, fmt.Errorf("账号内核未启动，请先进入工作区")
	}
	req, e := http.NewRequestWithContext(ctx, method, yyAccountAddress(a)+path, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if len(extra) > 0 {
		for _, name := range []string{"X-Signature-Ed25519", "X-Signature-Timestamp", "X-Bot-Appid"} {
			if value := extra[0].Get(name); value != "" {
				req.Header.Set(name, value)
			}
		}
	}
	return (&http.Client{Timeout: 0}).Do(req)
}
func yyChildJSON(a yyManagedAccount, method, path string, body any) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	data, _ := json.Marshal(body)
	response, e := yyChildRequest(ctx, a, method, path, data, "application/json")
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	var result map[string]any
	e = json.NewDecoder(response.Body).Decode(&result)
	return result, e
}
func yyWriteChildConfig(a yyManagedAccount, auto bool) error {
	dir := yyAccountDir(a.ID)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	if e := os.WriteFile(filepath.Join(dir, "parent-port"), []byte(yyCorePort), 0600); e != nil {
		return e
	}
	if e := yyPoolWriteChildIdentity(a, dir); e != nil {
		return e
	}
	cfg := config.AppConfig{Token: a.Token, AuthURL: a.AuthURL, ServerCode: a.Server, ServerPass: a.Password, ImportSpeed: 10000, ExportSpeed: 10000, AutoConnect: auto, AutoReconnect: a.KeepAlive, ReconnectSeconds: 10, AutoCreative: true, ImportCommands: true, ExcludeWater: true, ExcludeWaterlogged: true, ExcludeLava: true, MediaDisplay: "actionbar", MediaTarget: "@a"}
	path := filepath.Join(dir, "config.json")
	if old, e := os.ReadFile(path); e == nil {
		_ = json.Unmarshal(old, &cfg)
		cfg.Token = a.Token
		cfg.AuthURL = a.AuthURL
		cfg.ServerCode = a.Server
		cfg.ServerPass = a.Password
		cfg.AutoConnect = auto
		cfg.AutoReconnect = a.KeepAlive
	}
	if a.CaptureOnly {
		cfg.AutoCreative = false
	}
	if a.CaptureOnly {
		_ = os.WriteFile(filepath.Join(dir, "capture-only"), []byte("1"), 0600)
	} else {
		_ = os.Remove(filepath.Join(dir, "capture-only"))
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if e := os.WriteFile(path+".tmp", data, 0600); e != nil {
		return e
	}
	return os.Rename(path+".tmp", path)
}
func yyStartManaged(a yyManagedAccount, auto bool) error {
	if e := yyWriteChildConfig(a, auto); e != nil {
		return e
	}
	if result, e := yyChildJSON(a, "GET", "/health", nil); e == nil && result["ready"] == true {
		if result["account_id"] != a.ID {
			return fmt.Errorf("运行位置仍由其他账号占用，请稍后重试")
		}
		return nil
	}
	return yyAccountProcessStart(a.Slot, a.ID, yyAccountDir(a.ID))
}
func yyAccountIdentitySummary(a yyManagedAccount) map[string]any {
	item := map[string]any{"id": a.ID, "name": a.Name, "auth_url": a.AuthURL, "server": a.Server, "enabled": a.Enabled, "keep_alive": a.KeepAlive, "has_token": a.Token != "", "has_password": a.Password != "", "engine_ready": false, "connected": false, "plugins": []any{}, "online": nil}
	item["owner_id"] = yyPoolOwner(a)
	item["capture_only"], item["shared_capture"] = a.CaptureOnly, a.SharedCapture
	item["fleet_ready"] = false
	item["credential_state"] = "missing"
	item["phase"] = "credential_missing"
	item["launcher_registration_known"] = false
	if a.Token != "" {
		item["credential_state"] = "stored_unverified"
		item["phase"] = "credential_stored"
	}
	yyPool.Lock()
	if lease, ok := yyPool.Leases[a.ID]; ok {
		item["pool_binding"] = lease
	}
	yyPool.Unlock()
	for key, value := range yyAccountProfileView(a.ID, a.Name) {
		item[key] = value
	}
	item["connected"], item["engine_ready"] = nil, nil
	item["telemetry_loaded"], item["connection_known"] = false, false
	item["phase"] = "status_unknown"
	return item
}
func yyAccountSummary(a yyManagedAccount) map[string]any {
	item := yyAccountIdentitySummary(a)
	item["telemetry_loaded"], item["connection_known"] = true, true
	item["connected"], item["engine_ready"] = false, false
	status, e := yyChildJSON(a, "GET", "/api/bot/status", nil)
	if e != nil {
		item["status"] = "内核未启动"
		return item
	}
	item["engine_ready"] = true
	item["phase"] = "engine_ready"
	item["connected"] = status["connected"]
	if status["connected"] == true {
		item["phase"] = "connected"
		item["fleet_ready"] = true
		item["credential_state"] = "verified_for_current_server"
		item["server_join_verified_for"] = a.Server
	}
	item["connection"] = status
	if data, e := yyChildJSON(a, "GET", "/api/youyuan/observations", nil); e == nil {
		item["online"] = data["online"]
		item["online_source"] = data["source"]
	}
	if data, e := yyChildJSON(a, "GET", "/api/youyuan/runtime/list", nil); e == nil {
		item["instances"] = data["instances"]
		item["installed"] = data["installed"]
	}
	if data, e := yyChildJSON(a, "GET", "/api/plugin/list", nil); e == nil {
		item["plugins"] = data["plugins"]
	}
	return item
}
func yyAccountsRoutes(mux *http.ServeMux, dataDir string) {
	yyAccounts.Root = dataDir
	yyAccounts.Worker = os.Getenv("YY_ACCOUNT_WORKER") == "1" || strings.Contains(filepath.ToSlash(dataDir), "/managed-accounts/")
	if !yyAccounts.Worker {
		if data, e := os.ReadFile(filepath.Join(dataDir, "managed-accounts.json")); e == nil {
			_ = json.Unmarshal(data, &yyAccounts.Items)
		}
		// A disabled account that still carries a slot points at a kernel that
		// is gone. Any code that probes the slot then reads the failure as
		// "this robot will not identify itself", which used to freeze the
		// capture roster forever. Drop the stale slot on load so the state on
		// disk stops describing a process that does not exist.
		healed := false
		for i := range yyAccounts.Items {
			if !yyAccounts.Items[i].Enabled && yyAccounts.Items[i].Slot > 0 {
				yyAccounts.Items[i].Slot = 0
				healed = true
			}
		}
		if healed {
			yyAccounts.Lock()
			_ = yyManagedSaveLocked()
			yyAccounts.Unlock()
		}
	}
	mux.HandleFunc(yyAccountsPrefix, func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, yyAccountsPrefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 || !yyWorkspaceID(parts[0]) || !strings.HasPrefix(parts[1], "api/") {
			http.Error(w, "账号路径无效", 400)
			return
		}
		if strings.HasPrefix(parts[1], "api/youyuan/accounts") {
			http.Error(w, "不允许嵌套账号管理", 400)
			return
		}
		if e := yyPoolInvocationAllowed(r, parts[0]); e != nil {
			http.Error(w, e.Error(), 403)
			return
		}
		if parts[0] == "main" {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/" + parts[1]
			mux.ServeHTTP(w, clone)
			return
		}
		if yyAccounts.Worker {
			http.Error(w, "子内核不嵌套账号", 400)
			return
		}
		a, found := yyManagedFind(parts[0])
		if !found {
			http.Error(w, "账号不存在", 404)
			return
		}
		body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<20))
		if e != nil {
			http.Error(w, "请求超过 64 MiB", 413)
			return
		}
		path := "/" + parts[1]
		if strings.HasPrefix(path, "/api/youyuan/accounts") {
			http.Error(w, "不允许嵌套账号管理", 400)
			return
		}
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		response, e := yyChildRequest(r.Context(), a, r.Method, path, body, r.Header.Get("Content-Type"), r.Header)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "账号内核未在线，请启用账号或进入工作区"})
			return
		}
		defer response.Body.Close()
		for _, header := range []string{"Content-Type", "Content-Disposition", "Cache-Control"} {
			if value := response.Header.Get(header); value != "" {
				w.Header().Set(header, value)
			}
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	})
	mux.HandleFunc("/api/youyuan/accounts", func(w http.ResponseWriter, r *http.Request) {
		if yyAccounts.Worker {
			writeJSON(w, map[string]any{"ok": false, "error": "账号管理在主控制端使用"})
			return
		}
		actor, actorErr := yyPoolActorRequest(r)
		// Local owner sessions remain compatible; cloud sessions must retain tenant identity.
		if yyPoolHasActor(r) && actorErr != nil {
			http.Error(w, actorErr.Error(), 403)
			return
		}
		if r.Method == "GET" {
			yyAccounts.Lock()
			items := append([]yyManagedAccount(nil), yyAccounts.Items...)
			yyAccounts.Unlock()
			if yyPoolHasActor(r) && !actor.Admin {
				filtered := []yyManagedAccount{}
				for _, a := range items {
					if yyPoolCanUse(actor, a) {
						filtered = append(filtered, a)
					}
				}
				items = filtered
			}
			summaries := make([]map[string]any, len(items))
			var wg sync.WaitGroup
			for i, a := range items {
				wg.Add(1)
				go func(i int, a yyManagedAccount) {
					defer wg.Done()
					if r.URL.Query().Get("view") == "summary" {
						summaries[i] = yyAccountIdentitySummary(a)
					} else {
						summaries[i] = yyAccountSummary(a)
					}
				}(i, a)
			}
			wg.Wait()
			mainConfig := config.ReloadConfig()
			mainName := mainConfig.MainAccountName
			if mainName == "" {
				mainName = "主账号"
			}
			mainProfile := yyAccountProfileView("main", mainName)
			mainProfile["configured"] = mainConfig.Token != ""
			if yyPoolHasActor(r) && !actor.Admin && actor.ID != yyPoolBoundActor().ID {
				mainProfile = nil
			}
			if mainProfile != nil {
				for key, value := range yyMainGameIdentityView() {
					mainProfile[key] = value
				}
			}
			writeJSON(w, map[string]any{"ok": true, "accounts": summaries, "process_slots": yyAccountProcessSlots, "profiles_consume_slots": false, "main": mainProfile})
			return
		}
		yyAccountMutationMu.Lock()
		defer yyAccountMutationMu.Unlock()
		var p struct {
			Action       string           `json:"action"`
			Account      yyManagedAccount `json:"account"`
			ID           string           `json:"id"`
			Name         string           `json:"name"`
			Role         string           `json:"profile_role"`
			Avatar       *string          `json:"avatar"`
			DeferConnect bool             `json:"defer_connect"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if p.ID == "" {
			p.ID = p.Account.ID
		}
		if yyPoolHasActor(r) {
			old, exists := yyManagedFind(p.ID)
			if (p.ID == "main" && !actor.Admin && actor.ID != yyPoolBoundActor().ID) || (exists && !yyPoolCanUse(actor, old)) {
				http.Error(w, "该机器人不在你的账号池中", 403)
				return
			}
			if r.Method != "POST" || actor.Scope == "read" {
				http.Error(w, "需要控制授权", 403)
				return
			}
		}
		if p.Action == "token" {
			if r.Method != "POST" {
				http.Error(w, "POST required", 405)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			if p.ID == "main" {
				c := config.ReloadConfig()
				writeJSON(w, map[string]any{"ok": true, "token": c.Token, "auth_url": c.AuthURL})
				return
			}
			a, exists := yyManagedFind(p.ID)
			if !exists {
				writeJSON(w, map[string]any{"ok": false, "error": "账号不存在"})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "token": a.Token, "auth_url": a.AuthURL})
			return
		}
		if p.Action == "profile" {
			e := yySaveAccountProfile(p.ID, p.Name, p.Role, p.Avatar)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "profile": yyAccountProfileView(p.ID, p.Name)})
			return
		}
		if !yyManagedAccountID(p.ID) || p.ID == "main" && p.Action != "save" && p.Action != "" {
			writeJSON(w, map[string]any{"ok": false, "error": "请使用独立账号 ID（字母、数字、下划线）"})
			return
		}
		yyAccounts.Lock()
		index := -1
		for i, a := range yyAccounts.Items {
			if a.ID == p.ID {
				index = i
				break
			}
		}
		a := p.Account
		a.ID = p.ID
		a.OwnerID = actor.ID // Never accept an owner supplied by a client.
		a.Slot = 0
		a.Enabled = false
		if p.Action == "save" || p.Action == "" {
			if index >= 0 {
				old := yyAccounts.Items[index]
				a.OwnerID = old.OwnerID
				a.Slot = old.Slot
				a.Enabled = old.Enabled
				if !actor.Admin {
					a.SharedCapture = old.SharedCapture
				}
				if a.Token == "" {
					a.Token = old.Token
				}
				if a.Password == "" {
					a.Password = old.Password
				}
			}
			if index < 0 && !actor.Admin {
				a.SharedCapture = false
			}
			if a.AuthURL == "" {
				a.AuthURL = "https://auth.youyuanqi.dpdns.org"
			}
			parsed, e := url.Parse(a.AuthURL)
			if e != nil || parsed.Scheme != "https" || parsed.Host == "" {
				yyAccounts.Unlock()
				writeJSON(w, map[string]any{"ok": false, "error": "验证服务需为 HTTPS 地址"})
				return
			}
			if a.Name == "" {
				a.Name = a.ID
			}
			canPrimary := !yyPoolHasActor(r) || actor.Admin || actor.ID != "" && actor.ID == yyPoolBoundActor().ID
			if primary, primaryErr := yyTryPrimaryAccountSave(a, index >= 0, canPrimary); primary || primaryErr != nil {
				yyAccounts.Unlock()
				if primaryErr != nil {
					writeJSON(w, map[string]any{"ok": false, "error": primaryErr.Error()})
					return
				}
				writeJSON(w, map[string]any{"ok": true, "id": "main", "primary": true, "connection_requested": false})
				return
			}
			if a.ID == "main" {
				yyAccounts.Unlock()
				writeJSON(w, map[string]any{"ok": false, "error": "已有主连接不会被账号添加操作替换，请在主连接设置中明确修改"})
				return
			}
			if a.Token != "" {
				main := config.ReloadConfig()
				duplicate := a.Token == main.Token
				for _, other := range yyAccounts.Items {
					if other.ID != a.ID && other.Token == a.Token {
						duplicate = true
					}
				}
				if duplicate {
					yyAccounts.Unlock()
					writeJSON(w, map[string]any{"ok": false, "error": "同一 Token 只能维持一个游戏会话，不能重复添加主账号或其他账号"})
					return
				}
			}
			if a.Name == "" {
				a.Name = a.ID
			}
			if index < 0 {
				yyAccounts.Items = append(yyAccounts.Items, a)
			} else {
				yyAccounts.Items[index] = a
			}
			e = yyManagedSaveLocked()
			yyAccounts.Unlock()
			if e == nil {
				e = yyWriteChildConfig(a, a.Enabled)
				if e == nil && a.Enabled && !p.DeferConnect {
					_, e = yyChildJSON(a, "POST", "/api/bot/connect", map[string]any{"token": a.Token, "auth": a.AuthURL, "server": a.Server, "password": a.Password})
				}
			}
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "id": a.ID})
			return
		}
		if index < 0 {
			yyAccounts.Unlock()
			writeJSON(w, map[string]any{"ok": false, "error": "账号不存在"})
			return
		}
		a = yyAccounts.Items[index]
		if p.Action == "disable" || p.Action == "delete" {
			a.Enabled = false
			yyAccounts.Items[index] = a
			yyAccounts.Items[index].Slot = 0
			if p.Action == "delete" {
				yyAccounts.Items = append(yyAccounts.Items[:index], yyAccounts.Items[index+1:]...)
			}
			e := yyManagedSaveLocked()
			yyAccounts.Unlock()
			_ = yyWriteChildConfig(a, false)
			_, _ = yyChildJSON(a, "POST", "/api/bot/disconnect", map[string]any{})
			var stopErr error
			if a.Slot > 0 {
				stopErr = yyAccountProcessStop(a.Slot)
				for n := 0; n < 20; n++ {
					if _, probe := yyChildJSON(a, "GET", "/health", nil); probe != nil {
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
			if e == nil {
				e = stopErr
			}
			if p.Action == "delete" {
				if poolErr := yyPoolForgetAccount(a.ID); e == nil {
					e = poolErr
				}
			}
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if p.Action != "engine_start" && p.Action != "enable" && p.Action != "connect" {
			yyAccounts.Unlock()
			writeJSON(w, map[string]any{"ok": false, "error": "未知账号操作"})
			return
		}
		auto := p.Action != "engine_start"
		if auto && (a.Token == "" || a.Server == "") {
			yyAccounts.Unlock()
			writeJSON(w, map[string]any{"ok": false, "error": "启用连接需要该账号自己的 Token 和服务器号"})
			return
		}
		if auto {
			a.Enabled = true
		}
		if err := yyAccountAllocateLocked(&a); err != nil {
			yyAccounts.Unlock()
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		yyAccounts.Items[index] = a
		e := yyManagedSaveLocked()
		yyAccounts.Unlock()
		if e == nil {
			e = yyStartManaged(a, a.Enabled)
		}
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if auto {
			_, _ = yyChildJSON(a, "POST", "/api/bot/connect", map[string]any{"token": a.Token, "server": a.Server, "password": a.Password, "auth": a.AuthURL})
		}
		writeJSON(w, map[string]any{"ok": true, "id": a.ID, "connection_requested": auto})
	})
	if !yyAccounts.Worker {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				yyAccountMutationMu.Lock()
				yyAccounts.Lock()
				items := append([]yyManagedAccount(nil), yyAccounts.Items...)
				yyAccounts.Unlock()
				for _, a := range items {
					if a.Enabled && a.KeepAlive {
						if _, e := yyChildJSON(a, "GET", "/health", nil); e != nil {
							_ = yyStartManaged(a, true)
						}
					}
				}
				yyAccountMutationMu.Unlock()
			}
		}()
	}
}
