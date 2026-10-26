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
	Type   string            `json:"type"`             // reply | like | emoji | command | relay | image | capability
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
	// MinRole 是这条规则的最低身份：0 所有人 / 1 管理员 / 2 超级管理员。
	// 缺省 0 = 老规则行为不变（谁发都触发）。用户原话：
	// 「触发词规则当中好像并没有 选择是管理员还是普通成员的分层使用」。
	MinRole int                 `json:"min_role"`
	Actions []yyQQTriggerAction `json:"actions"`
}

// yyQQTriggerMax 规则条数上限。原为 60，用户要求「放开到四五十个」后放到 120。
const yyQQTriggerMax = 120

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
	// any = 任意消息：不看内容、也不看 patterns。必须放在循环外面——
	// 原来它在循环里 return，patterns 为空数组时 any 反而不触发，
	// 而且面板选 any 后会把触发词输入框藏掉（数据留着但看不见），
	// 用户完全看不出为什么「每句话都触发」。语义统一成：
	// any 就是每条消息都命中，与触发词无关。
	// 返回空 pattern：调用方用 pattern 做 {arg} 剥离，返回具体词会误删正文。
	if t.Type == "any" {
		return "", true
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

// yyQQTriggerLevel 触发者身份：助手授权表（超管/管理员名单）优先，
// 其次 OneBot 群角色（owner/admin/member）。0 成员 1 管理员 2 超管。
// 与 yyQQTriggerCapability 里的判定保持同一套，避免「规则判定」和「动作判定」两套标准。
func yyQQTriggerLevel(cf yyQQConfig, userID, role string) int {
	if lv := yyQQRole(cf, userID); lv > 0 {
		return lv
	}
	switch role {
	case "owner":
		return 2
	case "admin":
		return 1
	}
	return 0
}

// yyQQTriggerRun 在群消息入口调用。命中并执行了返回 true，调用方就不再走后续分支。
func yyQQTriggerRun(cf yyQQConfig, name, userID, role, text, msgID string, mentioned bool) bool {
	triggers := cf.Assistant.Triggers
	if len(triggers) == 0 || userID == "" {
		return false
	}
	level := yyQQTriggerLevel(cf, userID, role)
	group := yyQQGroupString(cf.GroupID)
	for _, t := range triggers {
		if !t.Enabled || len(t.Patterns) == 0 || len(t.Actions) == 0 {
			continue
		}
		// min_role 分层：权限不够时这条规则整条跳过（不匹配、不冷却、不执行），
		// 后面的规则还有机会命中——普通成员不会因为触发管理员规则被吞消息。
		if level < t.MinRole {
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
			// 安全审查发现：这里的指令是 {arg} 渲染出来的，等于把群友输入直接送进控制台，
			// 之前完全没过危险命令黑名单 —— 一条「op 我」的触发词就能提权。
			// 触发器是自动执行的，没人能当场点「同意」，所以这里直接拦掉并留痕。
			if e := yySrvInfoCommandGuard(cmd); e != nil {
				yyQQLog("触发词指令被拦（危险指令）：" + e.Error())
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
			// relay 走的是控制台：如果渲染结果以 / 开头，它其实是一条指令，
			// 同样要过黑名单，否则「转发」也能变成执行通道。
			if strings.HasPrefix(strings.TrimSpace(text), "/") {
				if e := yySrvInfoCommandGuard(strings.TrimSpace(text)); e != nil {
					yyQQLog("触发词转发被拦（伪装成指令）：" + e.Error())
					continue
				}
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
		case "capability":
			// 调用机器人能力（改头像/禁言/踢人等）：用户第 16 条第 5 点
			//「触发词怎么去调用前面的改头像这种——可以用触发词」。
			yyQQTriggerCapability(cf, t.Name, params, vars, userID, msgID)
		}
	}
}

// yyQQTriggerCapability 执行「调用机器人能力」动作（触发词 type=capability）。
// params.kind 取自 youyuan_qq_caps.go 能力注册表；params.arg 是参数文本，
// 先过 yyQQTriggerRender 做 {arg}/{qq}/{user} 等变量展开，再按能力约定解析：
//
//	like           「QQ号 [次数1-10]」，空 = 给触发者点 1 次
//	emoji_like     「[表情ID]」，对触发的那条消息贴表情（不填用 👍）
//	qzone          「动态内容」；delete_qzone「tid」
//	avatar         「图片URL/base64」；long_nick「签名内容」；nick「昵称」
//	kick           「QQ号」；ban「QQ号 [分钟数]」（只有 QQ 号时默认 10 分钟）
//	recall         「[消息ID]」，空 = 撤回触发的那条消息
//	relay_history  「[条数]」，缺省 5；poke「[QQ号]」，空 = 拍触发者
//	sticker        「图片URL 或 收藏表情序号」
//
// face（只读罗列）与 game_skill（游戏指令）不走这条通道：前者没有副作用，
// 后者用 command 动作更直接。失败按 reply 动作的惯例记日志，不 panic。
func yyQQTriggerCapability(cf yyQQConfig, rule string, params, vars map[string]string, userID, msgID string) {
	kind := strings.TrimSpace(params["kind"])
	if kind == "" {
		yyQQLog("触发词能力未指定 kind，已跳过")
		return
	}
	if _, ok := yyQQCapabilityOf(kind); !ok {
		yyQQLog("触发词能力被拒（未知能力 " + kind + "）")
		return
	}
	arg := strings.TrimSpace(yyQQTriggerRender(params["arg"], vars))
	name := vars["user"]
	// 触发者权限：优先助手授权表（超管/管理员名单），其次 OneBot 群角色。
	level := yyQQRole(cf, userID)
	if level == 0 {
		switch vars["role"] {
		case "owner":
			level = 2
		case "admin":
			level = 1
		}
	}
	group := yyQQGroupString(cf.GroupID)
	reject := func(format string, args ...any) {
		yyQQLog("触发词能力参数无效（" + kind + "）：" + fmt.Sprintf(format, args...))
	}
	var target string
	var run func() (string, error)
	switch kind {
	case "like":
		times := 1
		target = userID
		if f := strings.Fields(arg); len(f) > 0 {
			if !yyQQNumericQQ(f[0]) {
				reject("%s", arg)
				return
			}
			target = f[0]
			if len(f) > 1 {
				v, e := strconv.Atoi(f[1])
				if e != nil || v < 1 || v > 10 {
					reject("次数 %s", f[1])
					return
				}
				times = v
			}
		}
		run = func() (string, error) { return "", yyQQActionLike(target, times) }
	case "emoji_like":
		if msgID == "" {
			return
		}
		target = msgID
		run = func() (string, error) { return "", yyQQActionEmojiLike(msgID, arg, true) }
	case "qzone":
		if arg == "" {
			reject("动态内容为空")
			return
		}
		content := arg
		if strings.ContainsAny(content, "\r\n") {
			content = strings.Join(strings.Fields(content), " ")
		}
		run = func() (string, error) { _, err := yyQQActionQzone(content, 1); return "", err }
	case "delete_qzone":
		if arg == "" {
			reject("tid 为空")
			return
		}
		target = arg
		run = func() (string, error) { return "", yyQQActionDeleteQzone(arg) }
	case "avatar":
		if arg == "" {
			reject("图片地址为空")
			return
		}
		run = func() (string, error) { return "", yyQQActionAvatar(arg) }
	case "long_nick":
		if arg == "" || len([]rune(arg)) > 100 || strings.ContainsAny(arg, "\r\n") {
			reject("签名需 1-100 字且不能换行")
			return
		}
		run = func() (string, error) { return "", yyQQActionLongNick(arg) }
	case "nick":
		if arg == "" || len([]rune(arg)) > 64 || strings.ContainsAny(arg, "\r\n") {
			reject("昵称需 1-64 字且不能换行")
			return
		}
		run = func() (string, error) { return "", yyQQActionNick(arg) }
	case "kick":
		if !yyQQNumericQQ(arg) {
			reject("%s", arg)
			return
		}
		target = arg
		run = func() (string, error) { return "", yyQQActionKick(group, arg, false) }
	case "ban":
		// arg 约定「QQ号 [分钟数]」：只给 QQ 号时默认 10 分钟。
		f := strings.Fields(arg)
		if len(f) == 0 || !yyQQNumericQQ(f[0]) {
			reject("格式「QQ号 [分钟数]」，收到 %s", arg)
			return
		}
		minutes := 10
		if len(f) > 1 {
			v, e := strconv.Atoi(f[1])
			if e != nil || v < 1 || v > 43200 {
				reject("分钟数 %s", f[1])
				return
			}
			minutes = v
		}
		target = f[0]
		run = func() (string, error) { return "", yyQQActionBan(group, target, minutes) }
	case "recall":
		id := arg
		if id == "" {
			id = msgID
		}
		if id == "" {
			reject("消息ID 为空")
			return
		}
		target = id
		run = func() (string, error) { return "", yyQQActionRecall(group, id) }
	case "relay_history":
		n := 5
		if arg != "" {
			v, e := strconv.Atoi(arg)
			if e != nil || v < 1 || v > 50 {
				reject("条数 %s", arg)
				return
			}
			n = v
		}
		run = func() (string, error) { return "", yyQQRelayHistory(cf, n) }
	case "poke":
		target = userID
		if arg != "" {
			t, ok := yyPokeTargetArg(arg, userID)
			if !ok {
				reject("%s", arg)
				return
			}
			target = t
		}
		run = func() (string, error) { return "", yyPokeGroup(group, target) }
	case "sticker":
		if arg == "" {
			reject("图片地址或序号为空")
			return
		}
		url := arg
		if n, e := strconv.Atoi(arg); e == nil {
			if n < 1 {
				reject("收藏表情序号从 1 开始")
				return
			}
			rows, ferr := yyStickerFaces()
			if ferr != nil {
				yyQQLog("触发词能力读取收藏表情失败：" + ferr.Error())
				return
			}
			if n > len(rows) {
				reject("没有第 %d 个收藏表情", n)
				return
			}
			url = strings.TrimSpace(fmt.Sprint(rows[n-1]["url"]))
			if url == "" {
				reject("第 %d 个收藏表情没有可用地址", n)
				return
			}
			target = "收藏表情#" + strconv.Itoa(n)
		} else {
			if !yyStickerURL(arg) || len(arg) > 2000 {
				reject("只支持 http/https 图片链接或收藏表情序号")
				return
			}
			target = yyQQCardClip(arg, 60)
		}
		send := url
		run = func() (string, error) { return "", yyStickerSend(group, send) }
	default:
		// face 只读罗列、game_skill 走 command 动作，这里明确拒绝而不是静默失败。
		yyQQLog("触发词能力暂不支持 " + kind + "（游戏指令请用 command 动作）")
		return
	}
	if e := yyQQRunAction(kind, userID, name, target, rule, level, run); e != nil {
		yyQQLog("触发词能力失败（" + kind + "）：" + e.Error())
	}
}

// yyQQTriggerCapabilityOptions 给面板的能力下拉选项：kind+label 全部从
// youyuan_qq_caps.go 注册表取，不写死；face（只读罗列）与 game_skill
// （游戏指令，走 command 动作）不进这条通道。
func yyQQTriggerCapabilityOptions() []map[string]string {
	out := make([]map[string]string, 0, len(yyQQCapabilities))
	for _, c := range yyQQCapabilities {
		if c.Kind == "face" || c.Kind == "game_skill" {
			continue
		}
		out = append(out, map[string]string{"value": c.Kind, "label": c.Label})
	}
	return out
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
		{
			ID: "preset-ban", Enabled: true, Name: "触发词禁言", Type: "keyword",
			Patterns: []string{"骂人词"}, Cooldown: 300, Daily: 10,
			Actions: []yyQQTriggerAction{{Type: "capability", Params: map[string]string{"kind": "ban", "arg": "{qq} 10"}}},
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
				// min_role：0 所有人 / 1 管理员 / 2 超管；缺省 0。
				"roles":   []string{"成员", "管理员", "超级管理员"},
				"max":     yyQQTriggerMax,
				"actions": []map[string]any{
					{"type": "reply", "label": "回复文字", "params": []string{"text"}},
					{"type": "like", "label": "点赞", "params": []string{"times", "target"}},
					{"type": "emoji", "label": "贴表情", "params": []string{"emoji_id"}},
					{"type": "command", "label": "执行游戏指令", "params": []string{"input"}},
					{"type": "relay", "label": "转发到游戏", "params": []string{"text"}},
					{"type": "image", "label": "生成图片", "params": []string{"prompt", "use_avatar", "size", "model"}},
					{"type": "capability", "label": "调用机器人能力", "params": []map[string]any{
						{"key": "kind", "label": "能力", "options": yyQQTriggerCapabilityOptions()},
						{"key": "arg", "label": "参数"},
					}},
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
				// min_role 只接受 0/1/2，越界夹紧（面板传 9 会让规则永远不触发）。
				if t.MinRole < 0 {
					t.MinRole = 0
				}
				if t.MinRole > 2 {
					t.MinRole = 2
				}
				if len(t.Actions) > 12 {
					t.Actions = t.Actions[:12]
				}
				cleaned = append(cleaned, t)
			}
			// 用户要求「以后玩家想添加到四五十个，就是把权限放开」：
			// 原上限 60 条，放到 120 条（规则是纯数据，不影响执行性能）。
			if len(cleaned) > yyQQTriggerMax {
				writeJSON(w, map[string]any{"ok": false, "error": "规则最多 " + strconv.Itoa(yyQQTriggerMax) + " 条"})
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
