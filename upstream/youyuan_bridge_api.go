package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 群服互通 · 开放接口层
// ---------------------------------------------------------------------------
// 用户的要求（原话）：
//   「群服互通的所有接口都可以用插件来修改，要给用户最大的可塑性、开放、自由性。
//     MCP 可以去改，插件也可以联通 API。比如说服务器里如果有人一直挥臂，
//     机器人检测到这个，通过 API 协议让群服互通机器人在 QQ 群里面发相关的信息；
//     QQ 群里面的相关信息也可以回发，执行在服务器上面。
//     就是群服互通是一个新的接口……记得多注册一些关于互通的。」
//
// 设计原则（第一性）：**互通本身不写死玩法**。
//   - 谁产生事件：游戏侧 / 插件，通过 POST /api/bridge/emit 上报（例如 player.arm_swing）
//   - 怎么反应：由玩家/插件写规则（事件 + 阈值 + 窗口 + 动作），存在本地 JSON 里
//   - 谁能调：/api/bridge/call 统一入口，能力清单由 /api/bridge/manifest 自描述
// 这样「挥臂刷屏 → 群里喊一句」不需要改代码，插件自己就能做。
// ---------------------------------------------------------------------------

const yyBridgeEventKeep = 300

type yyBridgeAction struct {
	// qq_send      发群消息（Text 支持 {player} {count} {event}）
	// game_command 在服务器里执行指令（Command，需要令牌）
	// game_chat    以机器人身份在服内说一句话（Text）
	Type    string `json:"type"`
	Group   string `json:"group,omitempty"`
	Text    string `json:"text,omitempty"`
	Command string `json:"command,omitempty"`
}

type yyBridgeRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Enabled     bool           `json:"enabled"`
	Event       string         `json:"event"`
	PerPlayer   bool           `json:"per_player"`
	Window      int            `json:"window_seconds"`
	Threshold   int            `json:"threshold"`
	CoolSeconds int            `json:"cool_seconds"`
	Action      yyBridgeAction `json:"action"`
}

type yyBridgeEvent struct {
	Seq    int64          `json:"seq"`
	At     string         `json:"at"`
	Event  string         `json:"event"`
	Player string         `json:"player,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

// yyBridgeRules 是规则的只读快照：事件链路只读它，不再持锁 memcpy。
// 写入方（保存/删除/加载规则）负责 Store 一份新的。
var yyBridgeRules atomic.Value

var yyBridge = struct {
	sync.Mutex
	Path     string
	Rules    []yyBridgeRule
	Events   []yyBridgeEvent
	Seq      int64
	marks    map[string][]time.Time
	lastFire map[string]time.Time
	loaded   bool
}{marks: map[string][]time.Time{}, lastFire: map[string]time.Time{}}

func yyBridgeNow() string { return time.Now().Format(time.RFC3339) }

// yyBridgeLoad 首次访问时从磁盘读规则。文件不存在不算错误。
func yyBridgeLoad() {
	yyBridge.Lock()
	defer yyBridge.Unlock()
	if yyBridge.loaded {
		return
	}
	yyBridge.loaded = true
	if yyBridge.Path == "" {
		return
	}
	raw, e := os.ReadFile(yyBridge.Path)
	if e != nil {
		return
	}
	var wrap struct {
		Rules []yyBridgeRule `json:"rules"`
	}
	if json.Unmarshal(raw, &wrap) == nil {
		yyBridge.Rules = wrap.Rules
		yyBridgeRules.Store(append([]yyBridgeRule(nil), wrap.Rules...))
	}
}

func yyBridgeSaveLocked() error {
	if yyBridge.Path == "" {
		return fmt.Errorf("规则文件路径未初始化")
	}
	raw, e := json.MarshalIndent(map[string]any{"rules": yyBridge.Rules}, "", "  ")
	if e != nil {
		return e
	}
	tmp := yyBridge.Path + ".tmp"
	if e := os.WriteFile(tmp, raw, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, yyBridge.Path)
}

// yyBridgePublish 供游戏侧/其它模块直接调用（不用走 HTTP）。
// 例如观测到玩家连续挥臂时：yyBridgePublish("player.arm_swing", name, nil)
func yyBridgePublish(event, player string, data map[string]any) yyBridgeEvent {
	yyBridge.Lock()
	yyBridge.Seq++
	ev := yyBridgeEvent{Seq: yyBridge.Seq, At: yyBridgeNow(), Event: event, Player: player, Data: data}
	yyBridge.Events = append(yyBridge.Events, ev)
	if len(yyBridge.Events) > yyBridgeEventKeep {
		yyBridge.Events = yyBridge.Events[len(yyBridge.Events)-yyBridgeEventKeep:]
	}
	rules, _ := yyBridgeRules.Load().([]yyBridgeRule)
	yyBridge.Unlock()

	for _, r := range rules {
		if !r.Enabled || r.Event != event {
			continue
		}
		yyBridgeFire(r, ev)
	}
	return ev
}

// yyBridgeFire 做窗口计数与冷却判断，满足条件才执行动作。
func yyBridgeFire(r yyBridgeRule, ev yyBridgeEvent) {
	win := r.Window
	if win <= 0 {
		win = 10
	}
	threshold := r.Threshold
	if threshold <= 0 {
		threshold = 1
	}
	key := r.ID
	if r.PerPlayer {
		key += "|" + ev.Player
	}
	now := time.Now()
	cut := now.Add(-time.Duration(win) * time.Second)

	yyBridge.Lock()
	rows := yyBridge.marks[key]
	kept := rows[:0]
	for _, t := range rows {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	yyBridge.marks[key] = kept
	count := len(kept)
	if count < threshold {
		yyBridge.Unlock()
		return
	}
	if r.CoolSeconds > 0 {
		if last, ok := yyBridge.lastFire[key]; ok && now.Sub(last) < time.Duration(r.CoolSeconds)*time.Second {
			yyBridge.Unlock()
			return
		}
	}
	yyBridge.lastFire[key] = now
	// 直接删 key：置 nil 会让 map 里留下"空值 key"，per-player 规则一多就是十万级条目。
	delete(yyBridge.marks, key)
	// 顺手清理：规则改来改去、玩家来来去去，这两个 map 不清理会一直涨。
	if len(yyBridge.lastFire) > 2000 || len(yyBridge.marks) > 2000 {
		cutoff := now.Add(-2 * time.Hour)
		for k, at := range yyBridge.lastFire {
			if at.Before(cutoff) {
				delete(yyBridge.lastFire, k)
				delete(yyBridge.marks, k)
			}
		}
	}
	yyBridge.Unlock()

	yyBridgeRun(r, ev, count)
}

func yyBridgeRender(tpl string, ev yyBridgeEvent, count int) string {
	out := tpl
	out = strings.ReplaceAll(out, "{player}", ev.Player)
	out = strings.ReplaceAll(out, "{count}", fmt.Sprint(count))
	out = strings.ReplaceAll(out, "{event}", ev.Event)
	return out
}

func yyBridgeRun(r yyBridgeRule, ev yyBridgeEvent, count int) {
	cf := yyQQSnapshot()
	switch r.Action.Type {
	case "qq_send":
		text := strings.TrimSpace(yyBridgeRender(r.Action.Text, ev, count))
		if text == "" {
			return
		}
		group := strings.TrimSpace(r.Action.Group)
		if group != "" {
			if !yyQQGroupEnabled(cf, group) {
				yyQQLog("互通规则 " + r.Name + " 未执行：目标群未启用")
				return
			}
			cf.GroupID = group
			if e := yyQQSendToGroup(cf, text, ""); e != nil {
				yyQQLog("互通规则 " + r.Name + " 发群失败：" + e.Error())
			}
			return
		}
		if e := yyQQSend(text, ""); e != nil {
			yyQQLog("互通规则 " + r.Name + " 发群失败：" + e.Error())
		}
	case "game_chat":
		text := strings.TrimSpace(yyBridgeRender(r.Action.Text, ev, count))
		if text == "" {
			return
		}
		yyBridgeGameChat(text)
	case "game_command":
		cmd := strings.TrimSpace(yyBridgeRender(r.Action.Command, ev, count))
		if cmd == "" {
			return
		}
		// 规则里的指令同样要过危险命令黑名单：规则是插件写的，不能给它开白名单。
		if e := yySrvInfoCommandGuard(cmd); e != nil {
			yyQQLog("互通规则 " + r.Name + " 的指令被拦：" + e.Error())
			return
		}
		if e := yyBridgeGameCommand(cmd); e != nil {
			yyQQLog("互通规则 " + r.Name + " 执行指令失败：" + e.Error())
		}
	}
}

// yyBridgeGameCommand 通过已有的游戏连接下发一条指令。
// 复用 BotManager.SendCommandNoWait，不新造协议。
func yyBridgeGameCommand(cmd string) error {
	if !strings.HasPrefix(cmd, "/") {
		cmd = "/" + cmd
	}
	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()
	if bm == nil || !bm.IsConnected() {
		return fmt.Errorf("游戏机器人未连接")
	}
	return bm.SendCommandNoWait(ChannelAI, strings.TrimPrefix(cmd, "/"))
}

// yyBridgeGameChat 以机器人身份在服务器里说一句话（走聊天通道）。
func yyBridgeGameChat(text string) {
	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()
	if bm == nil || !bm.IsConnected() {
		return
	}
	bm.sendChatMessage(text)
}

// yyBridgeTokenOK 互通开放接口的令牌校验。
// 令牌沿用互通设置里的 MCP 令牌。
//
// 安全审查结论（H2）：**不能**用 RemoteAddr 做兜底 —— 经过内网中继时
// youyuan_remote.go 会把请求重写成 http://127.0.0.1，RemoteAddr 永远是 127.0.0.1，
// 等于「没有令牌就全部放行」。所以这里只认令牌，没配就一律拒绝，
// 并在错误信息里告诉用户去哪里配。
func yyBridgeTokenOK(r *http.Request) bool {
	want := strings.TrimSpace(yyQQSnapshot().Assistant.MCPToken)
	if want == "" {
		return false
	}
	got := strings.TrimSpace(r.Header.Get("X-Bridge-Token"))
	if got == "" {
		got = strings.TrimSpace(r.Header.Get("Authorization"))
		got = strings.TrimSpace(strings.TrimPrefix(got, "Bearer "))
	}
	if got == "" {
		got = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// yyBridgeTokenHint 给前端/插件的可执行提示。
func yyBridgeTokenHint() string {
	if strings.TrimSpace(yyQQSnapshot().Assistant.MCPToken) == "" {
		return "互通令牌未配置：打开「群服互通 → 接入设置」，填一个 MCP 令牌后再调用（也是插件要带的 X-Bridge-Token）"
	}
	return "需要令牌：把互通设置里的 MCP 令牌放进 X-Bridge-Token 头或 ?token="
}

func yyBridgeManifest() []map[string]any {
	return []map[string]any{
		{"id": "qq.send", "label": "发群消息", "danger": false,
			"params":  map[string]string{"text": "内容", "group": "群号，可空"},
			"example": map[string]any{"text": "服务器重启完成"}},
		{"id": "qq.action", "label": "执行机器人能力（点赞/转发/动态等）", "danger": false,
			"params":  map[string]string{"kind": "能力 kind", "target": "目标", "detail": "备注"},
			"example": map[string]any{"kind": "relay_history", "detail": "条数=5"}},
		{"id": "game.chat", "label": "在服内说话", "danger": true,
			"params":  map[string]string{"text": "内容"},
			"example": map[string]any{"text": "群友让我提醒大家收一下作物"}},
		{"id": "game.command", "label": "在服内执行指令", "danger": true,
			"params":  map[string]string{"command": "不带斜杠的指令"},
			"example": map[string]any{"command": "say 大家好"}},
		{"id": "rag.search", "label": "检索知识库", "danger": false,
			"params":  map[string]string{"query": "问题", "top_k": "条数"},
			"example": map[string]any{"query": "怎么申请白名单"}},
		{"id": "server.players", "label": "玩家列表", "danger": false,
			"params":  map[string]string{"limit": "条数"},
			"example": map[string]any{"limit": 50}},
	}
}

func yyBridgeRoutes(mux *http.ServeMux, dataDir string) {
	yyBridge.Path = filepath.Join(dataDir, "bridge-rules.json")
	yyBridgeLoad()

	// 能力清单：给插件/MCP 自描述用，不用读源码猜参数。
	mux.HandleFunc("/api/bridge/manifest", func(w http.ResponseWriter, r *http.Request) {
		yyBridgeLoad()
		yyBridge.Lock()
		rules := append([]yyBridgeRule(nil), yyBridge.Rules...)
		yyBridge.Unlock()
		writeJSON(w, map[string]any{
			"ok": true, "version": 1,
			"capabilities": yyBridgeManifest(),
			"events": []map[string]string{
				{"id": "player.arm_swing", "label": "玩家连续挥臂"},
				{"id": "player.join", "label": "玩家进服"},
				{"id": "player.leave", "label": "玩家退服"},
				{"id": "player.chat", "label": "玩家在服内发言"},
				{"id": "player.death", "label": "玩家死亡"},
				{"id": "server.tps_low", "label": "服务器 TPS 过低"},
			},
			"rules": rules,
			"note":  "插件上报事件用 POST /api/bridge/emit；想让事件产生动作就配一条规则（POST /api/bridge/rules）。",
		})
	})

	// 统一调用入口。危险能力必须带令牌。
	mux.HandleFunc("/api/bridge/call", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		var p struct {
			Capability string         `json:"capability"`
			Params     map[string]any `json:"params"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		str := func(k string) string {
			if p.Params == nil {
				return ""
			}
			v, ok := p.Params[k]
			if !ok || v == nil {
				return ""
			}
			// 不能用 fmt.Sprint：nil 会变成字面量 "<nil>"，
			// 于是空值校验被绕过、真的把 "<nil>" 当参数发出去。
			return strings.TrimSpace(fmt.Sprint(v))
		}
		// 只读能力不要求令牌；会改变 QQ/服内状态的一律要求令牌。
		// 原因：共享节点上受限订阅者可以通过 /api/youyuan/accounts/invoke/<自己账号>/api/bridge/call
		// 绕过 Worker 的权限判断，所以鉴权必须落在这里，不能只靠外层。
		if p.Capability != "server.players" && p.Capability != "rag.search" && !yyBridgeTokenOK(r) {
			writeJSON(w, map[string]any{"ok": false, "error": yyBridgeTokenHint()})
			return
		}
		switch p.Capability {
		case "qq.send":
			text := str("text")
			if text == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "text 不能为空"})
				return
			}
			cf := yyQQSnapshot()
			group := str("group")
			var e error
			if group != "" {
				if !yyQQGroupEnabled(cf, group) {
					writeJSON(w, map[string]any{"ok": false, "error": "目标群未启用"})
					return
				}
				cf.GroupID = group
				e = yyQQSendToGroup(cf, text, "")
			} else {
				e = yyQQSend(text, "")
			}
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true})
		case "qq.action":
			kind := str("kind")
			if _, ok := yyQQCapabilityOf(kind); !ok {
				writeJSON(w, map[string]any{"ok": false, "error": "未知能力：" + kind})
				return
			}
			cf := yyQQSnapshot()
			level := 2 // 插件调用按超级管理员对待，但仍受能力开关与限流约束
			e := yyQQRunAction(kind, "bridge", "插件", str("target"), str("detail"), level, func() (string, error) {
				return "", yyQQBridgeRunCap(kind, str("target"), str("detail"), cf)
			})
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true})
		case "game.chat":
			if !yyBridgeTokenOK(r) {
				writeJSON(w, map[string]any{"ok": false, "error": "需要令牌：把互通设置里的 MCP 令牌放进 X-Bridge-Token 头"})
				return
			}
			text := str("text")
			if text == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "text 不能为空"})
				return
			}
			yyBridgeGameChat(text)
			writeJSON(w, map[string]any{"ok": true})
		case "game.command":
			if !yyBridgeTokenOK(r) {
				writeJSON(w, map[string]any{"ok": false, "error": "需要令牌：把互通设置里的 MCP 令牌放进 X-Bridge-Token 头"})
				return
			}
			cmd := str("command")
			if cmd == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "command 不能为空"})
				return
			}
			if e := yyBridgeGameCommand(cmd); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true})
		case "rag.search":
			hits, mode := yyRagSearch(r.Context(), yyQQSnapshot(), str("query"), 5)
			if hits == nil {
				hits = []yyRagHit{}
			}
			writeJSON(w, map[string]any{"ok": true, "mode": mode, "hits": hits})
		case "server.players":
			limit := 50
			if p.Params != nil {
				if n, ok := p.Params["limit"].(float64); ok && n > 0 {
					limit = int(n)
				}
			}
			if limit > 500 {
				limit = 500
			}
			rows := yyBridgePlayerRows(limit)
			writeJSON(w, map[string]any{"ok": true, "players": rows, "count": len(rows)})
		default:
			writeJSON(w, map[string]any{"ok": false, "error": "未知能力：" + p.Capability + "（先 GET /api/bridge/manifest 看清单）"})
		}
	})

	// 事件上报：插件/游戏侧往这里推，命中规则就会自动产生动作。
	mux.HandleFunc("/api/bridge/emit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		// 事件能触发规则、规则能执行指令，所以 emit 也属于危险入口，必须带令牌。
		if !yyBridgeTokenOK(r) {
			writeJSON(w, map[string]any{"ok": false, "error": yyBridgeTokenHint()})
			return
		}
		var p struct {
			Event  string         `json:"event"`
			Player string         `json:"player"`
			Data   map[string]any `json:"data"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		p.Event = strings.TrimSpace(p.Event)
		if p.Event == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "event 不能为空"})
			return
		}
		ev := yyBridgePublish(p.Event, strings.TrimSpace(p.Player), p.Data)
		writeJSON(w, map[string]any{"ok": true, "seq": ev.Seq})
	})

	// 事件拉取：插件按 seq 增量取，做自己的反应。
	mux.HandleFunc("/api/bridge/events", func(w http.ResponseWriter, r *http.Request) {
		since := int64(0)
		fmt.Sscan(r.URL.Query().Get("since"), &since)
		limit := 50
		fmt.Sscan(r.URL.Query().Get("limit"), &limit)
		if limit <= 0 || limit > yyBridgeEventKeep {
			limit = 50
		}
		yyBridge.Lock()
		all := append([]yyBridgeEvent(nil), yyBridge.Events...)
		yyBridge.Unlock()
		out := make([]yyBridgeEvent, 0, limit)
		for i := len(all) - 1; i >= 0 && len(out) < limit; i-- {
			if all[i].Seq > since {
				out = append(out, all[i])
			}
		}
		// 反转成时间正序，方便插件顺序处理
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		yyBridge.Lock()
		seq := yyBridge.Seq
		yyBridge.Unlock()
		writeJSON(w, map[string]any{"ok": true, "events": out, "seq": seq})
	})

	// 规则读写
	mux.HandleFunc("/api/bridge/rules", func(w http.ResponseWriter, r *http.Request) {
		yyBridgeLoad()
		switch r.Method {
		case http.MethodGet:
			yyBridge.Lock()
			rules := append([]yyBridgeRule(nil), yyBridge.Rules...)
			yyBridge.Unlock()
			writeJSON(w, map[string]any{"ok": true, "rules": rules})
		case http.MethodPost:
			// 规则里可以写 game_command，等于远程执行权限，必须带令牌。
			if !yyBridgeTokenOK(r) {
				writeJSON(w, map[string]any{"ok": false, "error": yyBridgeTokenHint()})
				return
			}
			var p struct {
				Rules []yyBridgeRule `json:"rules"`
			}
			if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&p); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
				return
			}
			if len(p.Rules) > 200 {
				writeJSON(w, map[string]any{"ok": false, "error": "规则太多（上限 200 条）"})
				return
			}
			seen := map[string]bool{}
			for i := range p.Rules {
				rule := &p.Rules[i]
				rule.Name = strings.TrimSpace(rule.Name)
				rule.Event = strings.TrimSpace(rule.Event)
				if rule.Event == "" {
					writeJSON(w, map[string]any{"ok": false, "error": "第 " + fmt.Sprint(i+1) + " 条规则缺少事件"})
					return
				}
				if rule.ID == "" {
					rule.ID = fmt.Sprintf("r%d", time.Now().UnixNano())
				}
				if seen[rule.ID] {
					rule.ID += "-" + fmt.Sprint(i)
				}
				seen[rule.ID] = true
				if rule.Window < 0 {
					rule.Window = 0
				}
				if rule.Window > 3600 {
					rule.Window = 3600
				}
				if rule.Threshold < 1 {
					rule.Threshold = 1
				}
				if rule.Threshold > 10000 {
					rule.Threshold = 10000
				}
				switch rule.Action.Type {
				case "qq_send", "game_chat", "game_command":
				default:
					writeJSON(w, map[string]any{"ok": false, "error": "第 " + fmt.Sprint(i+1) + " 条规则的动作类型不支持：" + rule.Action.Type})
					return
				}
			}
			yyBridge.Lock()
			yyBridge.Rules = p.Rules
			yyBridgeRules.Store(append([]yyBridgeRule(nil), p.Rules...))
			e := yyBridgeSaveLocked()
			yyBridge.Unlock()
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "保存失败：" + e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "rules": p.Rules})
		default:
			http.Error(w, "GET/POST required", 405)
		}
	})

	mux.HandleFunc("/api/bridge/rules/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		if !yyBridgeTokenOK(r) {
			writeJSON(w, map[string]any{"ok": false, "error": yyBridgeTokenHint()})
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "缺少 id"})
			return
		}
		yyBridgeLoad()
		yyBridge.Lock()
		kept := make([]yyBridgeRule, 0, len(yyBridge.Rules))
		for _, rule := range yyBridge.Rules {
			if rule.ID != id {
				kept = append(kept, rule)
			}
		}
		yyBridge.Rules = kept
		yyBridgeRules.Store(append([]yyBridgeRule(nil), kept...))
		e := yyBridgeSaveLocked()
		yyBridge.Unlock()
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "rules": kept})
	})
}

// yyBridgePlayerRows 复用互通里已有的玩家统计，避免再造一份数据源。
func yyBridgePlayerRows(limit int) []map[string]any {
	yyQQHelper.Lock()
	names := make([]string, 0, len(yyQQHelper.Stats))
	for name := range yyQQHelper.Stats {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]map[string]any, 0, limit)
	for _, name := range names {
		if len(out) >= limit {
			break
		}
		st := yyQQHelper.Stats[name]
		if st == nil {
			continue
		}
		out = append(out, map[string]any{
			"player": name, "joined": st.Joined, "last_join": st.LastJoin,
			"last_leave": st.LastLeave, "online_seconds": st.OnlineSeconds,
			"messages": st.Messages, "last_message": st.LastMessage,
		})
	}
	yyQQHelper.Unlock()
	return out
}

// yyBridgeRunCap 把 qq.action 落到具体实现。只覆盖有明确入口的几种，
// 其余返回「暂不支持」而不是假装成功。
func yyQQBridgeRunCap(kind, target, detail string, cf yyQQConfig) error {
	switch kind {
	case "relay_history":
		n := 5
		if v := strings.TrimPrefix(detail, "条数="); v != detail {
			fmt.Sscan(v, &n)
		}
		if n < 1 {
			n = 1
		}
		if n > 50 {
			n = 50
		}
		return yyQQRelayHistory(cf, n)
	case "like":
		if target == "" {
			return fmt.Errorf("缺少目标 QQ 号")
		}
		return yyQQActionLike(target, 1)
	default:
		return fmt.Errorf("插件调用暂不支持能力 %s（面板里手动可用）", kind)
	}
}
