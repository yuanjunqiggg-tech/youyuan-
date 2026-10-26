package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 触发词引擎。
//
// 设计目标（来自实际使用场景）：能力不该由开发者写死成固定指令。
// 用户要能自己在面板里加一条规则，比如「群里有人说『赞我』→ 给这个人点赞」，
// 或者「有人发『画个』→ 先回一句、再画图、再把结果作为引用转发」。
// 所以这里把「什么时候触发」和「触发后做什么」拆成可编辑的数据，
// 由规则引擎在群消息入口统一匹配执行，不依赖 AI，也不依赖发版。

type yyQQTriggerAction struct {
	Type   string            `json:"type"`             // reply | like | emoji | command | relay | image
	Params map[string]string `json:"params,omitempty"` // 各动作自己的参数
}

type yyQQTrigger struct {
	ID       string              `json:"id"`
	Enabled  bool                `json:"enabled"`
	Name     string              `json:"name"`
	Type     string              `json:"type"`               // keyword | regex | at_bot | any
	Patterns []string            `json:"patterns"`           // 命中词 / 正则
	Groups   []string            `json:"groups,omitempty"`   // 空 = 所有已启用群
	Cooldown int                 `json:"cooldown,omitempty"` // 同一人冷却秒数
	Daily    int                 `json:"daily,omitempty"`    // 同一人每日上限
	Actions  []yyQQTriggerAction `json:"actions"`
}

var yyQQTriggerState = struct {
	sync.Mutex
	last map[string]time.Time
	used map[string]int
	day  string
}{last: map[string]time.Time{}, used: map[string]int{}}

// yyQQTriggerAllow 做冷却与每日上限判定。key 是「规则 + 人」，避免一个人刷屏。
func yyQQTriggerAllow(t yyQQTrigger, userID string) bool {
	now := time.Now()
	key := t.ID + "\x00" + userID
	yyQQTriggerState.Lock()
	defer yyQQTriggerState.Unlock()
	day := now.Format("2006-01-02")
	if yyQQTriggerState.day != day {
		yyQQTriggerState.day = day
		yyQQTriggerState.used = map[string]int{}
	}
	if t.Cooldown > 0 {
		if last, ok := yyQQTriggerState.last[key]; ok && now.Sub(last) < time.Duration(t.Cooldown)*time.Second {
			return false
		}
	}
	if t.Daily > 0 && yyQQTriggerState.used[key] >= t.Daily {
		return false
	}
	yyQQTriggerState.last[key] = now
	yyQQTriggerState.used[key]++
	return true
}

// yyQQTriggerMatch 返回命中的那一条 pattern，用于给 {arg} 做剥离。
func yyQQTriggerMatch(t yyQQTrigger, group, text string, mentioned bool) (string, bool) {
	if len(t.Groups) > 0 {
		found := false
		for _, g := range t.Groups {
			if strings.TrimSpace(g) == group {
				found = true
				break
			}
		}
		if !found {
			return "", false
		}
	}
	lower := strings.ToLower(text)
	for _, p := range t.Patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		switch t.Type {
		case "regex":
			if re, e := regexp.Compile(p); e == nil && re.MatchString(text) {
				return p, true
			}
		case "at_bot":
			if mentioned && strings.Contains(lower, strings.ToLower(p)) {
				return p, true
			}
		case "any":
			return p, true
		default: // keyword
			if strings.Contains(lower, strings.ToLower(p)) {
				return p, true
			}
		}
	}
	return "", false
}

// yyQQTriggerRender 展开模板变量。{arg} 是去掉命中词之后的剩余内容。
func yyQQTriggerRender(tpl string, vars map[string]string) string {
	if tpl == "" {
		return ""
	}
	pairs := make([]string, 0, len(vars)*4)
	for k, v := range vars {
		pairs = append(pairs, "{"+k+"}", v, "${"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(tpl)
}

func yyQQTriggerID() string {
	buf := make([]byte, 4)
	if _, e := rand.Read(buf); e != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf)
}

// yyQQTriggerRun 在群消息入口调用。命中并执行了返回 true，调用方就不再走后续分支。
func yyQQTriggerRun(cf yyQQConfig, name, userID, role, text, msgID string, mentioned bool) bool {
	triggers := cf.Assistant.Triggers
	if len(triggers) == 0 || userID == "" {
		return false
	}
	group := yyQQGroupString(cf.GroupID)
	for _, t := range triggers {
		if !t.Enabled || len(t.Patterns) == 0 || len(t.Actions) == 0 {
			continue
		}
		pattern, hit := yyQQTriggerMatch(t, group, text, mentioned)
		if !hit {
			continue
		}
		if !yyQQTriggerAllow(t, userID) {
			continue
		}
		arg := text
		if t.Type != "regex" && pattern != "" {
			arg = strings.ReplaceAll(arg, pattern, "")
		}
		arg = strings.TrimSpace(strings.TrimLeft(arg, "，。！？,.!? \t"))
		vars := map[string]string{
			"user": name, "qq": userID, "group": group, "role": role,
			"arg": arg, "pattern": pattern,
		}
		yyQQTriggerExec(cf, t, vars, userID, msgID)
		return true
	}
	return false
}

func yyQQTriggerExec(cf yyQQConfig, t yyQQTrigger, vars map[string]string, userID, msgID string) {
	for _, a := range t.Actions {
		params := a.Params
		if params == nil {
			params = map[string]string{}
		}
		switch a.Type {
		case "reply":
			// 用 | 分隔可以发多条，真人聊天就是一句一句说的。
			for _, part := range strings.Split(params["text"], "|") {
				part = strings.TrimSpace(yyQQTriggerRender(part, vars))
				if part == "" {
					continue
				}
				if e := yyQQSend(part, msgID); e != nil {
					yyQQLog("触发词回复失败：" + e.Error())
				}
			}
		case "like":
			times := 1
			if v, e := strconv.Atoi(strings.TrimSpace(params["times"])); e == nil && v > 0 && v <= 10 {
				times = v
			}
			target := vars["qq"]
			if v := strings.TrimSpace(params["target"]); v != "" {
				target = yyQQTriggerRender(v, vars)
			}
			if target == "" {
				continue
			}
			e := yyQQRunAction("like", userID, vars["user"], target, t.Name, 0, func() (string, error) {
				return "", yyQQActionLike(target, times)
			})
			if e != nil {
				yyQQLog("触发词点赞失败：" + e.Error())
			}
		case "emoji":
			emoji := strings.TrimSpace(params["emoji_id"])
			if emoji == "" {
				emoji = "128077" // 👍
			}
			if msgID == "" {
				continue
			}
			e := yyQQRunAction("emoji_like", userID, vars["user"], msgID, t.Name, 0, func() (string, error) {
				return "", yyQQActionEmojiLike(msgID, emoji, true)
			})
			if e != nil {
				yyQQLog("触发词表情失败：" + e.Error())
			}
		case "command":
			cmd := strings.TrimSpace(yyQQTriggerRender(params["input"], vars))
			if cmd == "" {
				continue
			}
			botMgrMu.Lock()
			bm := botMgr
			botMgrMu.Unlock()
			if bm == nil || !bm.IsConnected() {
				continue
			}
			if _, e := bm.SendConsole(cmd); e != nil {
				yyQQLog("触发词执行游戏指令失败：" + e.Error())
			}
		case "relay":
			text := strings.TrimSpace(yyQQTriggerRender(params["text"], vars))
			if text == "" {
				text = "[群] " + vars["user"] + "：" + vars["arg"]
			}
			botMgrMu.Lock()
			bm := botMgr
			botMgrMu.Unlock()
			if bm == nil || !bm.IsConnected() {
				continue
			}
			if _, e := bm.SendConsole(text); e != nil {
				yyQQLog("触发词转发失败：" + e.Error())
			}
		case "image":
			// 生图链路在 yyQQImageRun 里，异步执行；这里只负责排队。
			prompt := strings.TrimSpace(yyQQTriggerRender(params["prompt"], vars))
			if prompt == "" {
				prompt = vars["arg"]
			}
			go yyQQImageRun(cf, userID, vars["user"], msgID, prompt, params)
		}
	}
}

// yyQQTriggerPresets 面板里一键添加的模板，覆盖最常用的几个场景。
func yyQQTriggerPresets() []yyQQTrigger {
	return []yyQQTrigger{
		{
			ID: "preset-like-me", Enabled: true, Name: "赞我", Type: "keyword",
			Patterns: []string{"赞我", "点个赞", "求赞"}, Cooldown: 120, Daily: 3,
			Actions: []yyQQTriggerAction{{Type: "like", Params: map[string]string{"times": "3"}}},
		},
		{
			ID: "preset-hello", Enabled: true, Name: "打招呼", Type: "keyword",
			Patterns: []string{"在吗", "机器人"}, Cooldown: 30, Daily: 20,
			Actions: []yyQQTriggerAction{{Type: "reply", Params: map[string]string{"text": "在的，{user} 你说|有什么要我帮忙的直接说"}}},
		},
		{
			ID: "preset-draw", Enabled: true, Name: "画图", Type: "keyword",
			Patterns: []string{"画个", "画一个", "帮我画"}, Cooldown: 60, Daily: 5,
			Actions: []yyQQTriggerAction{
				{Type: "reply", Params: map[string]string{"text": "行，我想想怎么画…"}},
				{Type: "image", Params: map[string]string{"prompt": "{arg}", "use_avatar": "true", "size": "1024x1024"}},
			},
		},
		{
			ID: "preset-list", Enabled: true, Name: "查在线", Type: "keyword",
			Patterns: []string{"在线几个", "谁在线", "list"}, Cooldown: 20, Daily: 30,
			Actions: []yyQQTriggerAction{{Type: "command", Params: map[string]string{"input": "list"}}},
		},
		{
			ID: "preset-rule", Enabled: true, Name: "发群规", Type: "keyword",
			Patterns: []string{"群规", "规则是啥"}, Cooldown: 60, Daily: 10,
			Actions: []yyQQTriggerAction{{Type: "reply", Params: map[string]string{"text": "群规就三条：别刷屏、别发广告、有问题直接问，我尽量答。"}}},
		},
	}
}

// yyQQTriggerRoutes 提供规则的读写接口。整体替换而不是逐条 patch，
// 因为规则条数很少，前端整表提交更不容易出现状态错乱。
func yyQQTriggerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/triggers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cf := yyQQSnapshot()
			triggers := cf.Assistant.Triggers
			if triggers == nil {
				triggers = []yyQQTrigger{}
			}
			writeJSON(w, map[string]any{
				"ok": true, "triggers": triggers,
				"presets": yyQQTriggerPresets(),
				"types":   []string{"keyword", "regex", "at_bot", "any"},
				"actions": []map[string]any{
					{"type": "reply", "label": "回复文字", "params": []string{"text"}},
					{"type": "like", "label": "点赞", "params": []string{"times", "target"}},
					{"type": "emoji", "label": "贴表情", "params": []string{"emoji_id"}},
					{"type": "command", "label": "执行游戏指令", "params": []string{"input"}},
					{"type": "relay", "label": "转发到游戏", "params": []string{"text"}},
					{"type": "image", "label": "生成图片", "params": []string{"prompt", "use_avatar", "size", "model"}},
				},
				"vars": []string{"{user}", "{qq}", "{group}", "{role}", "{arg}", "{pattern}"},
			})
		case http.MethodPost:
			var payload struct {
				Triggers []yyQQTrigger `json:"triggers"`
			}
			if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			cleaned := make([]yyQQTrigger, 0, len(payload.Triggers))
			for _, t := range payload.Triggers {
				if strings.TrimSpace(t.ID) == "" {
					t.ID = yyQQTriggerID()
				}
				if strings.TrimSpace(t.Name) == "" {
					t.Name = "未命名规则"
				}
				if t.Type == "" {
					t.Type = "keyword"
				}
				t.Patterns = yyQQTrimList(t.Patterns, 20, 120)
				t.Groups = yyQQTrimList(t.Groups, 50, 24)
				if len(t.Actions) > 12 {
					t.Actions = t.Actions[:12]
				}
				cleaned = append(cleaned, t)
			}
			if len(cleaned) > 60 {
				writeJSON(w, map[string]any{"ok": false, "error": "规则最多 60 条"})
				return
			}
			e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
				a.Triggers = cleaned
			})
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "triggers": cleaned})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
		}
	})
}

func yyQQTrimList(in []string, maxItems, maxLen int) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if len(v) > maxLen {
			v = v[:maxLen]
		}
		out = append(out, v)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

// yyQQTriggerDescribe 给 AI 用的规则摘要，让模型知道哪些话会触发什么，
// 避免它去重复回答已经被规则处理掉的消息。
func yyQQTriggerDescribe(triggers []yyQQTrigger) string {
	if len(triggers) == 0 {
		return ""
	}
	var b strings.Builder
	for _, t := range triggers {
		if !t.Enabled || len(t.Patterns) == 0 {
			continue
		}
		kinds := make([]string, 0, len(t.Actions))
		for _, a := range t.Actions {
			kinds = append(kinds, a.Type)
		}
		b.WriteString(fmt.Sprintf("- 听到「%s」时你会执行 %s（规则已自动处理，你不用再回答这类消息）\n",
			strings.Join(t.Patterns, "/"), strings.Join(kinds, "+")))
	}
	return b.String()
}
