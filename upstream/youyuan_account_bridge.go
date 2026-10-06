package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Account data stays separate. Only an explicit directed rule permits coordination.
type yyAccountBridgeRule struct {
	Source     string   `json:"source"`
	Plugin     string   `json:"plugin"`
	Target     string   `json:"target"`
	Operations []string `json:"operations"`
}
type yyAccountBridgeCall struct {
	Source    string `json:"source"`
	Plugin    string `json:"plugin"`
	Target    string `json:"target"`
	Operation string `json:"operation"`
	Args      []any  `json:"args"`
}

var yyAccountBridge = struct {
	sync.RWMutex
	Rules []yyAccountBridgeRule
	Path  string
}{}
var yyBridgeCallbackDepth atomic.Int32
var yyAccountBridgeMethods = map[string]string{
	"accounts.list":             "无参数；列出本控制端的账号 ID、名称，不返回令牌。",
	"accounts.status":           "target；按定向授权读取目标账号连接状态。",
	"accounts.sendMessage":      "target,text；授权后由目标机器人发游戏聊天，不伪装玩家。",
	"accounts.sendCommandsFast": "target,commands,options?；在目标账号顺序提交完整命令数组，不等待逐条回包。",
	"accounts.callPlugin":       "target,pluginID,function,args数组；调用目标常驻插件的导出函数，不重跑源码。禁止跨账号递归调用。",
}

func init() {
	for name, description := range yyAccountBridgeMethods {
		yyExtraMethods[name] = description
	}
}
func yyBridgeAccountID(id string) bool { return id == "main" || yyManagedAccountID(id) }
func yyBridgeValidateRules(rules []yyAccountBridgeRule) error {
	if len(rules) > 4096 {
		return fmt.Errorf("授权规则过多")
	}
	for _, r := range rules {
		if !yyBridgeAccountID(r.Source) || !yyBridgeAccountID(r.Target) || r.Plugin == "" || r.Source == r.Target {
			return fmt.Errorf("规则需要不同源/目标账号及源插件 ID（* 表示该账号全部插件）")
		}
		if r.Plugin != "*" && !yyManagedAccountID(r.Plugin) {
			return fmt.Errorf("插件 ID 无效")
		}
		if len(r.Operations) == 0 {
			return fmt.Errorf("请至少选择一种操作")
		}
		for _, op := range r.Operations {
			if _, ok := yyAccountBridgeMethods["accounts."+op]; !ok || op == "list" {
				return fmt.Errorf("不支持的联通操作 %s", op)
			}
		}
	}
	return nil
}
func yyBridgeAllowed(call yyAccountBridgeCall) bool {
	yyAccountBridge.RLock()
	defer yyAccountBridge.RUnlock()
	for _, r := range yyAccountBridge.Rules {
		if r.Source == call.Source && r.Target == call.Target && (r.Plugin == "*" || r.Plugin == call.Plugin) {
			for _, op := range r.Operations {
				if op == call.Operation {
					return true
				}
			}
		}
	}
	return false
}
func yyBridgeProfiles() []any {
	rows := []any{map[string]any{"id": "main", "name": "主账号"}}
	yyAccounts.Lock()
	defer yyAccounts.Unlock()
	for _, a := range yyAccounts.Items {
		rows = append(rows, map[string]any{"id": a.ID, "name": a.Name, "enabled": a.Enabled})
	}
	return rows
}
func yyBridgeExecute(ctx context.Context, call yyAccountBridgeCall) (any, error) {
	if !yyBridgeAccountID(call.Source) || !yyBridgeAccountID(call.Target) || call.Source == call.Target || !yyBridgeAllowed(call) {
		return nil, fmt.Errorf("跨账号未授权：%s/%s → %s/%s，请在设置中添加定向授权", call.Source, call.Plugin, call.Target, call.Operation)
	}
	if call.Source != "main" {
		if _, ok := yyManagedFind(call.Source); !ok {
			return nil, fmt.Errorf("源账号不存在")
		}
	}
	if call.Target == "main" {
		return yyBridgeLocal(call)
	}
	target, ok := yyManagedFind(call.Target)
	if !ok {
		return nil, fmt.Errorf("目标账号不存在")
	}
	raw, e := json.Marshal(call)
	if e != nil {
		return nil, e
	}
	response, e := yyChildRequest(ctx, target, "POST", "/api/youyuan/accounts/bridge/local", raw, "application/json")
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	var result struct {
		OK     bool   `json:"ok"`
		Result any    `json:"result"`
		Error  string `json:"error"`
	}
	if e = json.NewDecoder(response.Body).Decode(&result); e != nil {
		return nil, e
	}
	if !result.OK {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return result.Result, nil
}
func yyBridgeLocal(call yyAccountBridgeCall) (any, error) {
	if call.Operation == "status" {
		_, raw, e := yyInvoke("GET", "/api/bot/status", nil, nil)
		if e != nil {
			return nil, e
		}
		var data any
		e = json.Unmarshal(raw, &data)
		return data, e
	}
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("目标账号未连接游戏")
	}
	switch call.Operation {
	case "sendMessage":
		if len(call.Args) != 1 {
			return nil, fmt.Errorf("sendMessage 需要 text")
		}
		text, ok := call.Args[0].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("消息不能为空")
		}
		if strings.HasPrefix(strings.TrimSpace(text), "/") {
			return nil, fmt.Errorf("发言授权不能用于执行斜杠指令，请使用命令数组授权")
		}
		result, e := bm.SendConsole(text)
		return result, e
	case "sendCommandsFast":
		return yyPluginCommandCall(bm, "game.sendCommandsFast", call.Args, yyDefaultCommandOptions())
	case "callPlugin":
		yyBridgeCallbackDepth.Add(1)
		defer yyBridgeCallbackDepth.Add(-1)
		if len(call.Args) != 3 {
			return nil, fmt.Errorf("callPlugin 需要 pluginID,function,args数组")
		}
		id, e := yySDKString(call.Args, 0)
		if e != nil {
			return nil, e
		}
		fn, e := yySDKString(call.Args, 1)
		if e != nil {
			return nil, e
		}
		args, ok := call.Args[2].([]any)
		if !ok {
			return nil, fmt.Errorf("args 必须是数组")
		}
		raw, e := json.Marshal(args)
		if e != nil {
			return nil, e
		}
		text, e := callPluginFunction(bm, id, fn, string(raw))
		if e != nil {
			return nil, e
		}
		if text == "" {
			return nil, nil
		}
		var result any
		e = json.Unmarshal([]byte(text), &result)
		return result, e
	}
	return nil, fmt.Errorf("未知联通操作")
}
func yyBridgeSDKCall(pluginID, method string, args []any) (any, error) {
	if _, ok := yyAccountBridgeMethods[method]; !ok {
		return nil, fmt.Errorf("未知账号 API")
	}
	source := "main"
	if yyAccounts.Worker {
		source = filepath.Base(yyAccounts.Root)
	}
	call := yyAccountBridgeCall{Source: source, Plugin: pluginID, Operation: strings.TrimPrefix(method, "accounts.")}
	if call.Operation != "list" {
		target, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		call.Target = target
		call.Args = args[1:]
	}
	// Plugin-to-plugin callbacks cannot open another synchronous account call:
	// avoids A -> B -> A holding two resident runtimes indefinitely.
	if yyBridgeCallbackDepth.Load() > 0 {
		return nil, fmt.Errorf("跨账号导出回调中不允许嵌套账号调用，请排程到回调返回后")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if !yyAccounts.Worker {
		if call.Operation == "list" {
			return yyBridgeProfiles(), nil
		}
		return yyBridgeExecute(ctx, call)
	}
	port, e := os.ReadFile(filepath.Join(yyAccounts.Root, "parent-port"))
	if e != nil {
		return nil, fmt.Errorf("主控制端地址未初始化，请重启此账号")
	}
	raw, _ := json.Marshal(call)
	path := "/api/youyuan/accounts/bridge/execute"
	if call.Operation == "list" {
		path = "/api/youyuan/accounts/bridge/profiles"
	}
	req, e := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:"+strings.TrimSpace(string(port))+path, bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	response, e := http.DefaultClient.Do(req)
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	var result struct {
		OK     bool   `json:"ok"`
		Result any    `json:"result"`
		Error  string `json:"error"`
	}
	if e = json.NewDecoder(response.Body).Decode(&result); e != nil {
		return nil, e
	}
	if !result.OK {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return result.Result, nil
}
func registerAccountBridgeAPI(L *lua.LState) {
	table := L.NewTable()
	L.SetGlobal("accounts", table)
	for name := range yyAccountBridgeMethods {
		method := name
		L.SetField(table, strings.TrimPrefix(name, "accounts."), L.NewFunction(func(L *lua.LState) int {
			args := []any{}
			for i := 1; i <= L.GetTop(); i++ {
				args = append(args, luaToGo(L.Get(i)))
			}
			result, e := yyBridgeSDKCall(lua.LVAsString(L.GetGlobal("__PLUGIN_ID__")), method, args)
			if e != nil {
				L.RaiseError("%s: %v", method, e)
				return 0
			}
			L.Push(goToLua(L, result))
			return 1
		}))
	}
}
func yyAccountBridgeRoutes(mux *http.ServeMux, root string) {
	yyAccountBridge.Path = filepath.Join(root, "account-bridge.json")
	raw, _ := os.ReadFile(yyAccountBridge.Path)
	var rules []yyAccountBridgeRule
	if json.Unmarshal(raw, &rules) == nil && yyBridgeValidateRules(rules) == nil {
		yyAccountBridge.Rules = rules
	}
	respond := func(w http.ResponseWriter, result any, e error) {
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "result": result})
	}
	mux.HandleFunc("/api/youyuan/accounts/bridge/rules", func(w http.ResponseWriter, r *http.Request) {
		if yyAccounts.Worker {
			http.Error(w, "在主控制端设置授权", 403)
			return
		}
		if r.Method == "POST" {
			var rows []yyAccountBridgeRule
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&rows); e != nil {
				respond(w, nil, e)
				return
			}
			if e := yyBridgeValidateRules(rows); e != nil {
				respond(w, nil, e)
				return
			}
			raw, _ := json.MarshalIndent(rows, "", "  ")
			yyAccountBridge.Lock()
			e := os.WriteFile(yyAccountBridge.Path+".tmp", raw, 0600)
			if e == nil {
				e = os.Rename(yyAccountBridge.Path+".tmp", yyAccountBridge.Path)
			}
			if e == nil {
				yyAccountBridge.Rules = rows
			}
			yyAccountBridge.Unlock()
			if e != nil {
				respond(w, nil, e)
				return
			}
		} else if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		yyAccountBridge.RLock()
		snapshot := append([]yyAccountBridgeRule{}, yyAccountBridge.Rules...)
		yyAccountBridge.RUnlock()
		writeJSON(w, map[string]any{"ok": true, "rules": snapshot, "accounts": yyBridgeProfiles()})
	})
	mux.HandleFunc("/api/youyuan/accounts/bridge/profiles", func(w http.ResponseWriter, r *http.Request) {
		if yyAccounts.Worker {
			http.Error(w, "主控制端专用", 403)
			return
		}
		respond(w, yyBridgeProfiles(), nil)
	})
	mux.HandleFunc("/api/youyuan/accounts/bridge/execute", func(w http.ResponseWriter, r *http.Request) {
		if yyAccounts.Worker || r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var call yyAccountBridgeCall
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20)).Decode(&call); e != nil {
			respond(w, nil, e)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		result, e := yyBridgeExecute(ctx, call)
		respond(w, result, e)
	})
	mux.HandleFunc("/api/youyuan/accounts/bridge/local", func(w http.ResponseWriter, r *http.Request) {
		// Internal worker delivery. Never published in the MCP catalog.
		if !yyAccounts.Worker || r.Method != "POST" {
			w.WriteHeader(403)
			return
		}
		var call yyAccountBridgeCall
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20)).Decode(&call); e != nil {
			respond(w, nil, e)
			return
		}
		if call.Target != filepath.Base(root) {
			http.Error(w, "目标账号不匹配", 403)
			return
		}
		result, e := yyBridgeLocal(call)
		respond(w, result, e)
	})
}
