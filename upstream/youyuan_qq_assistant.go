package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type yyQQAssistantConfig struct {
	Memory               yyQQMemoryConfig  `json:"memory"`
	Nicknames            map[string]string `json:"nicknames,omitempty"`
	Companion            bool              `json:"companion"`
	CompanionOwner       string            `json:"companion_owner"`
	CompanionKeywords    []string          `json:"companion_keywords"`
	CompanionAllMessages bool              `json:"companion_all_messages"`
	// CompanionWindow 接话窗口秒数。0 = 待定（走内置默认 30 秒）。
	// 用户明确要求「限制不要给默认值，改成待定让用户自己选」，所以这里保留 0 语义。
	CompanionWindow int `json:"companion_window,omitempty"`
	// AIMode 决定「AI 什么时候接话」，三档并存由玩家选（youyuan_qq_companion_aimode.go）：
	// mention = 现状（.AI / @机器人 才回）；interval = 定时自检；auto = 每条消息都判断。
	// 空/非法值一律按 mention 处理，老配置升级后行为不变。
	AIMode string `json:"ai_mode,omitempty"`
	// AIIntervalMinutes 是定时自检的间隔（分钟）。0 = 未设置，按 10 分钟兜底；夹到 1..1440。
	// 程序只提供「多久看一次」，回不回、回哪条、说什么全由提示词决定。
	AIIntervalMinutes int `json:"ai_interval_minutes,omitempty"`
	// 深夜降权与静默窗口：全部由玩家自己填，不替他拍值。
	// AINightStartHour/AINightEndHour 是深夜时段（0..23，默认 23/7，相等 = 不降权）；
	// AINightFactor 是降权倍数（1..10，默认 2：半夜自检间隔与自动判断间隔按倍数放宽）。
	AINightStartHour int `json:"ai_night_start_hour,omitempty"`
	AINightEndHour   int `json:"ai_night_end_hour,omitempty"`
	AINightFactor    int `json:"ai_night_factor,omitempty"`
	// AIQuietStart/AIQuietEnd 是静默窗口（"HH:MM"）。任一为空 = 未设置，AI 不受时段限制；
	// 落在窗口内时定时自检与全自动都不开口（被 @ 与 .AI 仍然照常回）。
	AIQuietStart string `json:"ai_quiet_start,omitempty"`
	AIQuietEnd   string `json:"ai_quiet_end,omitempty"`
	// RelayChat 是旧的「群聊转发进游戏」布尔开关，仅为读旧配置保留：
	// 新模式写进 RelayMode 之后它会被置回 false（见 youyuan_qq_relay.go）。
	RelayChat bool `json:"relay_chat"`
	// RelayMode 决定群消息怎么转进游戏，四档并存由玩家选：
	// off（默认，关闭）/ keyword（命中触发词才转）/ auto（每条都转）/ interval（定时打包）。
	// 空值按 off；只有 relay_chat=true 而没写 relay_mode 的旧配置按 auto 处理。
	RelayMode string `json:"relay_mode,omitempty"`
	// RelayKeywords 是触发词原文，换行或逗号分隔，keyword 模式下命中任一即转。
	RelayKeywords string `json:"relay_keywords,omitempty"`
	// RelayIntervalMinutes 是定时打包间隔（分钟），夹到 1..1440，0 = 默认 10。
	RelayIntervalMinutes int `json:"relay_interval_minutes,omitempty"`
	// RelayMaxMessages 是单次打包最多带多少条，夹到 1..200，0 = 默认 20，防止刷屏。
	RelayMaxMessages int    `json:"relay_max_messages,omitempty"`
	ImageModel       string `json:"image_model"`
	// VoiceModel 复用 /api/ai/models 里的某个配置做 TTS（OpenAI 兼容 /audio/speech）。
	// 空 = 没配语音模型：文本合成路径回可操作错误，音频 URL 直发仍然可用。
	VoiceModel string `json:"voice_model,omitempty"`
	// VoiceVoice 音色，留空 = alloy。
	VoiceVoice string `json:"voice_voice,omitempty"`
	// VoiceMode：off/空 = 不发语音（.语音 指令仍可用）；auto = AI 自己决定；
	// always = 每条文本回复都额外合成一条语音（文本照发）。非法值保存时归一化成空。
	VoiceMode string `json:"voice_mode,omitempty"`
	// Vision 是「AI 能不能看到群里的图片/表情包」的显式开关。
	// nil（缺省）= 按模型名自动检测（旧行为）；true = 强制开启；false = 强制关闭。
	// 自动检测只认模型名里的 vision/vl/4o/claude/gemini 等字样，像 DeepSeek
	// 这类实际上支持视觉但名字看不出来的模型会被漏判，所以必须有强制档。
	Vision           *bool           `json:"vision,omitempty"`
	Enabled          bool            `json:"enabled"`
	SuperAdmins      []string        `json:"super_admins"`
	Admins           []string        `json:"admins"`
	Death            bool            `json:"death"`
	Join             bool            `json:"join"`
	Leave            bool            `json:"leave"`
	PublicObjectives []string        `json:"public_objectives"`
	Model            string          `json:"model"`
	Prompt           string          `json:"prompt"`
	BotPlayer        string          `json:"bot_player"`
	Caps             map[string]bool `json:"caps,omitempty"`
	// CapLimits 是玩家对能力限流参数的覆盖（冷却/间隔/每日次数/最低角色）。
	// 只覆盖玩家显式改过的字段，其余继续用注册表默认，升级时不会冲掉玩家配置。
	CapLimits map[string]yyQQCapLimit `json:"cap_limits,omitempty"`
	// 危险指令名单交给玩家配：默认 31 条，另可自加前缀。
	// 其中 6 条默认锁定（op/deop/stop/ban/ban-ip/whitelist off），
	// 但锁不锁由玩家自己决定——服务器是他的，开关归他。
	DangerDisabled []string      `json:"danger_disabled,omitempty"`
	DangerExtra    []string      `json:"danger_extra,omitempty"`
	Triggers       []yyQQTrigger `json:"triggers,omitempty"`
	MCPURL         string        `json:"mcp_url"`
	MCPToken       string        `json:"mcp_token,omitempty"`
	// ReplyDelayMin/ReplyDelayMax 是「AI 开口前随机等待」的秒数区间。
	// 用户要求机器人不要每次秒回、要像真人，但不允许我们替他拍死数字：
	// 0 = 待定，走内置兜底 2-6 秒随机；面板必须写出兜底值，设过即消待定。
	ReplyDelayMin int `json:"reply_delay_min,omitempty"`
	ReplyDelayMax int `json:"reply_delay_max,omitempty"`
	// Skills 是玩家对技能注册表（youyuan_qq_skills.go）的逐项覆盖：
	// Word = 自定义功能词（如把 meme 换成「表情包」），Enabled = 开关。
	// 缺省 = 注册表默认；只存显式改过的项，升级不冲玩家配置。
	Skills map[string]yyQQSkillConf `json:"skills,omitempty"`
	// Tools 是玩家对 AI 工具清单（youyuan_qq_toolbox.go）的逐项开关。
	// 缺省 = 注册表默认值；只存显式改过的项，升级不冲玩家配置。
	// 关掉的工具不会下发给模型，模型也就调不动（执行侧还会再拦一次）。
	Tools map[string]bool `json:"tools,omitempty"`
	// CommandAliases 是玩家对助手指令格式的覆盖：key = 指令 ID
	// （relay/avatar/like/sticker/...），value = 自定义指令串（如 ".换头像"）。
	// 缺省 = 内置默认指令；匹配与帮助文本都以别名为准。
	CommandAliases map[string]string `json:"command_aliases,omitempty"`
}

// yyQQSkillConf 是单个技能的玩家覆盖项（见 youyuan_qq_skills.go 注册表）。
type yyQQSkillConf struct {
	// Word 是玩家自定义的功能词——AI 在系统提示词里看到的召唤词。
	// 空 = 用注册表默认词。
	Word string `json:"word,omitempty"`
	// Enabled 是技能开关；nil = 注册表默认（多数默认开）。
	Enabled *bool `json:"enabled,omitempty"`
}
type yyQQPlayerStats struct {
	Joined        time.Time `json:"joined"`
	LastJoin      time.Time `json:"last_join"`
	LastLeave     time.Time `json:"last_leave"`
	OnlineSeconds int64     `json:"online_seconds"`
	Messages      int       `json:"messages"`
	LastMessage   string    `json:"last_message"`
}
type yyQQBindingProof struct {
	User, Player, Message string
	Until                 time.Time
}

var yyQQHelper = struct {
	sync.Mutex
	Path       string
	Bindings   map[string]string
	Stats      map[string]*yyQQPlayerStats
	Proofs     map[string]yyQQBindingProof
	Seen       map[string]string
	Claim      string
	ClaimUntil time.Time
	Dirty      bool
}{Bindings: map[string]string{}, Stats: map[string]*yyQQPlayerStats{}, Proofs: map[string]yyQQBindingProof{}, Seen: map[string]string{}}
var yyQQHelperSlots = make(chan struct{}, 4)

func yyQQHasPrefix(text, prefix string) bool {
	return text == prefix || strings.HasPrefix(text, prefix+" ") || strings.HasPrefix(text, prefix+"\n")
}
func yyQQRole(cf yyQQConfig, user string) int {
	if user == "" {
		return 0
	}
	for _, v := range cf.Assistant.SuperAdmins {
		if v == user {
			return 2
		}
	}
	for _, v := range cf.Assistant.Admins {
		if v == user {
			return 1
		}
	}
	return 0
}
func yyQQHelperMutate(change func(*yyQQAssistantConfig)) error {
	yyQQMu.Lock()
	defer yyQQMu.Unlock()
	next := yyQQ
	next.Assistant.SuperAdmins = append([]string(nil), next.Assistant.SuperAdmins...)
	next.Assistant.Admins = append([]string(nil), next.Assistant.Admins...)
	if next.Assistant.Triggers != nil {
		triggers := make([]yyQQTrigger, len(next.Assistant.Triggers))
		for i, t := range next.Assistant.Triggers {
			triggers[i] = t
			triggers[i].Patterns = append([]string(nil), t.Patterns...)
			triggers[i].Groups = append([]string(nil), t.Groups...)
			actions := make([]yyQQTriggerAction, len(t.Actions))
			for j, a := range t.Actions {
				actions[j] = a
				if a.Params != nil {
					params := make(map[string]string, len(a.Params))
					for k, v := range a.Params {
						params[k] = v
					}
					actions[j].Params = params
				}
			}
			triggers[i].Actions = actions
		}
		next.Assistant.Triggers = triggers
	}
	if next.Assistant.Caps != nil {
		caps := make(map[string]bool, len(next.Assistant.Caps))
		for k, v := range next.Assistant.Caps {
			caps[k] = v
		}
		next.Assistant.Caps = caps
	}
	if next.Assistant.CapLimits != nil {
		limits := make(map[string]yyQQCapLimit, len(next.Assistant.CapLimits))
		for k, v := range next.Assistant.CapLimits {
			cp := yyQQCapLimit{}
			if v.MinLevel != nil {
				n := *v.MinLevel
				cp.MinLevel = &n
			}
			if v.CoolSeconds != nil {
				n := *v.CoolSeconds
				cp.CoolSeconds = &n
			}
			if v.GapSeconds != nil {
				n := *v.GapSeconds
				cp.GapSeconds = &n
			}
			if v.Daily != nil {
				n := *v.Daily
				cp.Daily = &n
			}
			limits[k] = cp
		}
		next.Assistant.CapLimits = limits
	}
	change(&next.Assistant)
	raw, e := json.MarshalIndent(next, "", "  ")
	if e == nil {
		e = os.WriteFile(yyQQPath, raw, 0600)
	}
	if e == nil {
		yyQQ = next
	}
	return e
}
func yyQQHelperSaveLocked() error {
	raw, e := json.Marshal(map[string]any{"bindings": yyQQHelper.Bindings, "stats": yyQQHelper.Stats, "members": yyQQHelper.Seen, "game_groups": yyQQPlayerGroups})
	if e != nil {
		return e
	}
	tmp := yyQQHelper.Path + ".tmp"
	if e = os.WriteFile(tmp, raw, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, yyQQHelper.Path)
}
func yyQQAssistantRoutes(mux *http.ServeMux, root string) {
	yyQQMemoryInit(root)
	yyQQMessageRoutes(mux)
	// 群内危险指令授权：队列落在数据目录，重启不丢未决项。
	yyApprInit(root)
	yyApprRoutes(mux)
	yyQQHelper.Path = filepath.Join(root, "qq-assistant.json")
	if raw, e := os.ReadFile(yyQQHelper.Path); e == nil {
		var saved struct {
			Bindings   map[string]string
			Stats      map[string]*yyQQPlayerStats
			Members    map[string]string
			GameGroups map[string]string
		}
		if json.Unmarshal(raw, &saved) == nil {
			if saved.Bindings != nil {
				yyQQHelper.Bindings = saved.Bindings
			}
			if saved.GameGroups != nil {
				yyQQPlayerGroups = saved.GameGroups
			}
			if saved.Members != nil {
				yyQQHelper.Seen = saved.Members
			}
			if saved.Stats != nil {
				yyQQHelper.Stats = saved.Stats
			}
			for _, v := range yyQQHelper.Stats {
				if v != nil {
					v.Joined = time.Time{}
				}
			}
		}
	}
	mux.HandleFunc("/api/qq/assistant", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var p struct {
				Action string
				User   string
				Role   string
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&p) != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
				return
			}
			if p.Action == "authorize" {
				if err := yyQQAuthorizeSeen(p.User, p.Role); err != nil {
					writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
					return
				}
				writeJSON(w, map[string]any{"ok": true, "user": p.User, "role": p.Role})
				return
			}
			if p.Action != "claim" {
				writeJSON(w, map[string]any{"ok": false, "error": "action 支持 claim/authorize"})
				return
			}
			yyQQHelper.Lock()
			yyQQHelper.Claim = yyNewKey()[:24]
			yyQQHelper.ClaimUntil = time.Now().Add(10 * time.Minute)
			code := yyQQHelper.Claim
			yyQQHelper.Unlock()
			writeJSON(w, map[string]any{"ok": true, "code": code, "expires_seconds": 600, "instruction": "在目标群 @机器人发送 " + yyQQCommandText(yyQQSnapshot(), "claim") + " " + code})
			return
		}
		if r.Method != "GET" {
			http.Error(w, "GET/POST required", 405)
			return
		}
		cf := yyQQSnapshot()
		yyQQHelper.Lock()
		defer yyQQHelper.Unlock()
		members := map[string]string{}
		for id, name := range yyQQHelper.Seen {
			members[id] = yyQQDisplayName(cf, id, name)
		}
		writeJSON(w, map[string]any{"ok": true, "bindings": yyQQHelper.Bindings, "members": members, "stats": yyQQHelper.Stats, "help": yyQQHelp(cf, 2)})
	})
	// AI 技能只读视图：面板用来看「AI 现在有哪些技能、功能词是什么、
	// 回复延迟配没配」。不走 /api/qq/skills——那个路径已被游戏技能注册表
	//（youyuan_qq_skill.go）占用，同一 mux 重复注册会 panic。
	// 保存不走新接口：统一走 POST /api/qq/config 的 assistant 深合并。
	mux.HandleFunc("/api/qq/assistant/skills", yyQQSkillsHandler)
	mux.HandleFunc("/api/qq/player-card", func(w http.ResponseWriter, r *http.Request) {
		image, e := yyQQPlayerCard(r.URL.Query().Get("player"))
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(image)
	})
	mux.HandleFunc("/api/qq/player-list", func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if v := r.URL.Query().Get("page"); v != "" {
			var e error
			page, e = strconv.Atoi(v)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "页码无效"})
				return
			}
		}
		names, e := yyQQOnlinePlayers()
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		data, e := yyQQPlayersCard(names, page)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	})
	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for range tick.C {
			bm := getBotManagerLocked()
			if bm == nil || !bm.IsConnected() {
				yyQQHelperPause()
			}
			yyQQHelper.Lock()
			if yyQQHelper.Dirty {
				if e := yyQQHelperSaveLocked(); e == nil {
					yyQQHelper.Dirty = false
				}
			}
			yyQQHelper.Unlock()
		}
	}()
}
// yyQQHelp 群内帮助菜单。正文改由注册表生成（youyuan_qq_help.go）：
// 只列「开关打开」且「当前权限够」的条目，命令串取玩家生效别名。
// 原来那一大段拼死的字符串已删除——菜单不再是第二份定义。
func yyQQHelp(cf yyQQConfig, level int) string {
	return yyQQHelpText(cf, level)
}
func yyQQAssistantHandle(cf yyQQConfig, name, user, role, text, id string) bool {
	if len(text) > 24000 {
		yyQQFeedbackTo(cf, "消息过长，请缩短到24000字节内", id)
		return true
	}
	yyQQHelper.Lock()
	if len(yyQQHelper.Seen) < 256 {
		if user != "" {
			yyQQHelper.Seen[user] = name
			yyQQHelper.Dirty = true
		}
	}
	yyQQHelper.Unlock()
	select {
	case yyQQHelperSlots <- struct{}{}:
		defer func() { <-yyQQHelperSlots }()
	default:
		yyQQFeedbackTo(cf, "助手正在处理其他任务，请稍后再试", id)
		return true
	}
	replies := yyQQAssistantCommand(cf, name, user, text, id)
	for _, reply := range replies {
		for _, part := range yyQQParts(reply, 4, 3200) {
			yyQQFeedbackTo(cf, part, id)
		}
	}
	return true
}
func yyQQParts(text string, maxParts, partSize int) []string {
	r := []rune(text)
	out := []string{}
	for len(r) > 0 && len(out) < maxParts {
		n := partSize
		if n > len(r) {
			n = len(r)
		}
		if n < len(r) {
			for j := n - 1; j > n/2; j-- {
				if r[j] == '\n' {
					n = j + 1
					break
				}
			}
		}
		out = append(out, string(r[:n]))
		r = r[n:]
	}
	if len(r) > 0 && len(out) > 0 {
		out[len(out)-1] += "\n…结果过长，缩小查询条数或在 APK 导出完整记录。"
	}
	return out
}
func yyQQAssistantCommand(cf yyQQConfig, name, user, text, id string) []string {
	level := yyQQRole(cf, user)
	group := yyQQGroupString(cf.GroupID)
	reply := func(s string) []string { return []string{s} }
	arg := func(prefix string) string { return strings.TrimSpace(strings.TrimPrefix(text, prefix)) }
	// 指令格式玩家可配：所有匹配都从生效指令串取（玩家别名优先，缺省内置默认）。
	cmd := func(id string) string { return yyQQCommandText(cf, id) }
	// 授权回复优先接管：「同意 1234」「拒绝 1234」「1234」「1」。
	// 只在有未决项且发言人是超级管理员时生效，普通聊天不会被吞。
	if handled, answer := yyApprTake(group, user, text); handled {
		if answer == "" {
			return nil
		}
		return reply(answer)
	}
	switch {
	case text == cmd("help") || text == "."+cmd("help") || text == "/"+cmd("help"):
		return reply(yyQQHelp(cf, level))
	case text == cmd("identity"):
		return reply("昵称：" + yyQQDisplayName(cf, user, name) + fmt.Sprintf("\n权限：%s", []string{"成员", "管理员", "超级管理员"}[level]))
	case text == cmd("status") || text == ".我的绑定":
		yyQQHelper.Lock()
		player := yyQQHelper.Bindings[user]
		yyQQHelper.Unlock()
		if player == "" {
			player = "未绑定（使用 " + cmd("bind") + " 游戏ID）"
		}
		if text == ".我的绑定" {
			return reply("我的游戏身份：" + player)
		}
		state := "未连接"
		if bm := getBotManagerLocked(); bm != nil && bm.IsConnected() {
			state = "已连接"
		}
		ai := "未配置"
		if cf.Assistant.Model != "" {
			ai = "已配置（" + cmd("ai") + " 问题）"
		}
		identity := ""
		if cf.Assistant.BotPlayer != "" {
			identity = "\n机器人身份：主机器人 " + cf.Assistant.BotPlayer + " 的QQ助手"
		}
		return reply("✦ 游园 · 当前状态\n游戏：" + state + identity + "\n我的绑定：" + player + "\n我的权限：" + []string{"成员", "管理员", "超级管理员"}[level] + "\n群AI：" + ai)
	case text == cmd("unbind"):
		yyQQHelper.Lock()
		player := yyQQHelper.Bindings[user]
		if player == "" {
			yyQQHelper.Unlock()
			return reply("你尚未绑定游戏身份。")
		}
		delete(yyQQHelper.Bindings, user)
		e := yyQQHelperSaveLocked()
		if e != nil {
			yyQQHelper.Bindings[user] = player
		} else {
			for code, proof := range yyQQHelper.Proofs {
				if proof.User == user {
					delete(yyQQHelper.Proofs, code)
				}
			}
		}
		yyQQHelper.Unlock()
		if e != nil {
			return reply("解除绑定保存失败：" + e.Error())
		}
		return reply("已解除自己的游戏绑定：" + player + "。管理员权限不受影响。")
	case yyQQHasPrefix(text, cmd("claim")):
		yyQQHelper.Lock()
		valid := yyQQHelper.Claim != "" && arg(cmd("claim")) == yyQQHelper.Claim && time.Now().Before(yyQQHelper.ClaimUntil)
		if valid {
			yyQQHelper.Claim = ""
		}
		yyQQHelper.Unlock()
		if !valid {
			return reply("认领码无效或过期，请在 APK 群服互通设置生成。")
		}
		e := yyQQHelperMutate(func(a *yyQQAssistantConfig) { a.SuperAdmins = append(a.SuperAdmins, user) })
		if e != nil {
			return reply("保存失败：" + e.Error())
		}
		return reply("已设为当前账号的超级管理员。")
	case text == "list" || strings.HasPrefix(text, "list ") || yyQQHasPrefix(text, cmd("player")):
		page := 1
		value := strings.TrimSpace(strings.TrimPrefix(text, "list"))
		if yyQQHasPrefix(text, cmd("player")) {
			value = arg(cmd("player"))
		}
		if value != "" {
			v, e := strconv.Atoi(value)
			if e != nil || v < 1 {
				return reply(fmt.Sprintf("格式：list [页码] 或 %s [页码]", cmd("player")))
			}
			page = v
		}
		// 与 AI、面板走同一条注册表实现（doc §5.5）：确定性执行，不再发头像卡片图。
		out, e := yyGameSkillRun("online_list", map[string]string{"page": strconv.Itoa(page)})
		if e != nil {
			return reply(e.Error())
		}
		return reply(out)
	case yyQQHasPrefix(text, cmd("group_log")):
		n := 20
		if value := arg(cmd("group_log")); value != "" {
			v, e := strconv.Atoi(value)
			if e != nil || v < 1 || v > 80 {
				return reply(fmt.Sprintf("格式：%s [1..80]，默认20；只含机器人实际收到的群消息", cmd("group_log")))
			}
			n = v
		}
		rows := yyQQRecentMessages(cf, n)
		if len(rows) == 0 {
			return reply("还没有收到这个群的消息；不能读取QQ完整历史。")
		}
		var b strings.Builder
		b.WriteString("✦ 游园 · 近期群消息\n")
		for _, row := range rows {
			content := row.Text
			if level == 0 {
				content = yyQQHideCoordinates(content)
			}
			fmt.Fprintf(&b, "[%s] %s：%s\n", row.Timestamp.Format("01-02 15:04:05"), row.Name, content)
		}
		return yyQQParts(b.String(), 4, 2800)
	case yyQQHasPrefix(text, cmd("log")):
		n, page := 60, 1
		f := strings.Fields(arg(cmd("log")))
		if len(f) > 2 {
			return reply(fmt.Sprintf("格式：%s [条数1..300] [页码]", cmd("log")))
		}
		if len(f) > 0 {
			v, e := strconv.Atoi(f[0])
			if e != nil || v < 1 || v > 300 {
				return reply("条数为1..300，默认60")
			}
			n = v
		}
		if len(f) > 1 {
			v, e := strconv.Atoi(f[1])
			if e != nil || v < 1 || v > 1000 {
				return reply("页码为1..1000")
			}
			page = v
		}
		parts := yyQQParts(yyQQChatLogs(n), 1000, 2800)
		pages := (len(parts) + 3) / 4
		if page > pages {
			return reply(fmt.Sprintf("仅%d页", pages))
		}
		start := (page - 1) * 4
		end := min(start+4, len(parts))
		out := parts[start:end]
		if page < pages && len(out) > 0 {
			out[len(out)-1] += fmt.Sprintf("\n下一页：%s %d %d", cmd("log"), n, page+1)
		}
		return out
	case yyQQHasPrefix(text, cmd("query")):
		p := arg(cmd("query"))
		if p == "" {
			return reply(fmt.Sprintf("格式：%s 游戏ID", cmd("query")))
		}
		image, e := yyQQPlayerCard(p)
		if e != nil {
			return reply(e.Error())
		}
		if e = yyQQSendImage(cf, image, id); e != nil {
			return reply(yyQQPlayerSummary(p) + "\n图片发送失败：" + e.Error())
		}
		return nil
	case yyQQHasPrefix(text, cmd("bind")):
		player := arg(cmd("bind"))
		if player == "" || strings.HasPrefix(player, "@") || len(player) > 128 || strings.ContainsAny(player, "\"\\\r\n") {
			return reply("请填写准确的游戏ID")
		}
		online, e := yyQQOnlinePlayers()
		if e != nil {
			return reply(e.Error())
		}
		found := false
		for _, p := range online {
			if p == player {
				found = true
			}
		}
		if !found {
			return reply("该玩家未在线，暂不能验证绑定。")
		}
		code := yyNewKey()[:12]
		yyQQHelper.Lock()
		for k, v := range yyQQHelper.Proofs {
			if time.Now().After(v.Until) || v.User == user {
				delete(yyQQHelper.Proofs, k)
			}
		}
		if len(yyQQHelper.Proofs) >= 128 {
			yyQQHelper.Unlock()
			return reply("绑定请求过多，请稍后再试")
		}
		yyQQHelper.Proofs[code] = yyQQBindingProof{user, player, id, time.Now().Add(4 * time.Minute)}
		yyQQHelper.Unlock()
		if e = yyQQTellraw(toPlayerSelector(player), "QQ绑定请求：请在游戏聊天输入 "+cmd("bindgame")+" "+code+"（4分钟内）。"); e != nil {
			return reply("游戏验证提示发送失败：" + e.Error())
		}
		return reply("请该游戏玩家查看游戏内验证码，并在游戏输入 " + cmd("bindgame") + " 验证码。")
	case yyQQHasPrefix(text, cmd("relay")):
		msg := arg(cmd("relay"))
		if msg == "" {
			return reply(fmt.Sprintf("格式：%s 具体消息", cmd("relay")))
		}
		if e := yyQQRelayGame("[群 " + yyQQGroupLabel(cf, yyQQGroupString(cf.GroupID)) + "] " + yyQQDisplayName(cf, user, name) + "：" + msg); e != nil {
			return reply(e.Error())
		}
		if cf.Adapter == "qqofficial" && cf.Official.Acknowledge != nil && !*cf.Official.Acknowledge {
			return nil
		}
		return reply("已转发到游戏。")
	case yyQQHasPrefix(text, cmd("speak")):
		yyQQHelper.Lock()
		player := yyQQHelper.Bindings[user]
		yyQQHelper.Unlock()
		if player == "" {
			return reply("请先 " + cmd("bind") + " 游戏ID，并在游戏完成验证。")
		}
		msg := arg(cmd("speak"))
		if msg == "" {
			return reply(fmt.Sprintf("格式：%s 消息", cmd("speak")))
		}
		if e := yyQQRelayGame("[群 " + yyQQGroupLabel(cf, yyQQGroupString(cf.GroupID)) + "] " + player + "：" + msg); e != nil {
			return reply(e.Error())
		}
		if cf.Adapter == "qqofficial" && cf.Official.Acknowledge != nil && !*cf.Official.Acknowledge {
			return nil
		}
		return reply("已以绑定身份转发。")
	case yyQQHasPrefix(text, cmd("authorize")) || yyQQHasPrefix(text, cmd("revoke")):
		if level < 2 {
			return reply("只有超级管理员可以授权或撤权。")
		}
		grant := yyQQHasPrefix(text, cmd("authorize"))
		prefix := cmd("revoke")
		if grant {
			prefix = cmd("authorize")
		}
		target := arg(prefix)
		if target == "" || len(target) > 128 || strings.ContainsAny(target, " \r\n") {
			return reply("请填写已设置的成员昵称，或在APK真实成员列表授权")
		}
		resolved, resolveErr := yyQQResolveMember(cf, target)
		if resolveErr != nil {
			return reply(resolveErr.Error())
		}
		target = resolved
		e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
			list := []string{}
			for _, v := range a.Admins {
				if v != target {
					list = append(list, v)
				}
			}
			if grant {
				list = append(list, target)
			}
			a.Admins = list
		})
		if e != nil {
			return reply(e.Error())
		}
		return reply("管理员授权已更新。")
	case yyQQHasPrefix(text, ".死亡消息") || yyQQHasPrefix(text, ".进服消息") || yyQQHasPrefix(text, ".退服消息"):
		if level == 0 {
			return reply("只有管理员可以修改通知开关。")
		}
		f := strings.Fields(text)
		if len(f) != 2 || (f[1] != "开启" && f[1] != "关闭") {
			return reply("使用 开启 或 关闭")
		}
		on := f[1] == "开启"
		e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
			switch f[0] {
			case ".死亡消息":
				a.Death = on
			case ".进服消息":
				a.Join = on
			case ".退服消息":
				a.Leave = on
			}
		})
		if e != nil {
			return reply(e.Error())
		}
		return reply(f[0] + "已" + f[1] + "。官方接口需要有效回复窗口；无窗口时发送失败会写入群服日志。")
	case yyQQHasPrefix(text, cmd("command")):
		if level == 0 {
			return reply("只有已授权管理员可以执行游戏命令。常用操作请用 " + cmd("skills") + " 或直接说需求。")
		}
		payload := arg(cmd("command"))
		if payload == "" {
			return reply(fmt.Sprintf("格式：%s 指令", cmd("command")))
		}
		// 黑名单里的指令不再「直接拒绝」：挂起成授权项，等超级管理员在群里
		// 回「同意 编号」。通过守卫的照旧直接执行，并记一条「上次执行」。
		out := yyApprRunOrAsk(cf, group, user, name, yyApprKindCommand, payload)
		// 逃生舱出错时引导到更安全的预定义技能路径（doc §5.5）。
		// 授权提示本身含有「拒绝」两个字，别把语法引导贴到它后面。
		for i := range out {
			if strings.Contains(out[i], "同意执行请回复") {
				continue
			}
			for _, mark := range []string{"语法错误", "未执行", "上限", "失败", "拒绝", "无此", "未知", "超时", "不合法"} {
				if strings.Contains(out[i], mark) {
					out[i] += "\n常用操作请用 " + cmd("skills") + " 或直接说需求。"
					break
				}
			}
		}
		return out
	case text == cmd("replay") || text == ".重复" || text == ".重放":
		if level == 0 {
			return reply("只有管理员可以重复执行指令。")
		}
		last, ok := yyApprLastOf(group, user)
		if !ok {
			return reply("这个群里你还没有成功执行过的指令。")
		}
		// 重放同样过黑名单：上次是危险指令，这次还得重新同意。
		return yyApprRunOrAsk(cf, group, user, name, last.Kind, last.Payload)
	case yyQQHasPrefix(text, cmd("ai")):
		message := arg(cmd("ai"))
		if message == "" {
			return reply(fmt.Sprintf("格式：%s 消息", cmd("ai")))
		}
		answer, e := yyQQAskAIWithReply(cf, user, message, id)
		if e != nil {
			return reply("AI处理失败：" + e.Error())
		}
		// 只执行了技能动作（如发表情包）时 AI 不产文本：不回空消息。
		if strings.TrimSpace(answer) == "" {
			return nil
		}
		return reply(answer)
	case text == cmd("actions"):
		return yyQQCmdActions(cf, level)
	case yyQQHasPrefix(text, cmd("poke")):
		return yyQQCmdPoke(cf, user, name, arg(cmd("poke")), level)
	case yyQQHasPrefix(text, cmd("sticker")):
		return yyQQCmdSticker(cf, user, name, arg(cmd("sticker")), level)
	case yyQQHasPrefix(text, cmd("voice")):
		// .语音 文本 → 走 TTS；.语音 http(s)/base64:// → 直接发音频。
		return yyQQCmdVoice(cf, user, name, arg(cmd("voice")), level)
	case yyQQHasPrefix(text, cmd("summary")) || text == ".小结":
		// 用户要「总结谁在群里发了什么」：本地确定性汇总，不走模型。
		return yyQQCmdSummary(cf, arg(cmd("summary")), level)
	case text == cmd("permission") || text == ".管理员":
		// 用户真的会在群里发 .权限，之前这条指令不存在，等于石沉大海。
		// 直接回：你是谁、超管/管理员分别是谁。
		return yyQQCmdRoles(cf, user, name, level)
	case text == cmd("skills"):
		return yyQQCmdSkills(cf, level)
	case yyQQHasPrefix(text, cmd("like")):
		return yyQQCmdLike(cf, user, name, arg(cmd("like")), level)
	case yyQQHasPrefix(text, cmd("qzone")):
		return yyQQCmdQzone(cf, user, name, arg(cmd("qzone")), level)
	case yyQQHasPrefix(text, cmd("delete_qzone")):
		return yyQQCmdDeleteQzone(cf, user, name, arg(cmd("delete_qzone")), level)
	case yyQQHasPrefix(text, cmd("avatar")):
		return yyQQCmdAvatar(cf, user, name, arg(cmd("avatar")), level)
	case yyQQHasPrefix(text, cmd("long_nick")):
		return yyQQCmdLongNick(cf, user, name, arg(cmd("long_nick")), level)
	case yyQQHasPrefix(text, cmd("nick")):
		return yyQQCmdNick(cf, user, name, arg(cmd("nick")), level)
	case yyQQHasPrefix(text, cmd("faces")):
		// 带参数 = 发那一张；不带 = 列出收藏（保持原行为）。
		if a := arg(cmd("faces")); a != "" {
			return yyQQCmdSticker(cf, user, name, a, level)
		}
		return yyQQCmdFaces(cf, level)
	case yyQQHasPrefix(text, cmd("emoji")):
		return yyQQCmdEmoji(cf, user, name, arg(cmd("emoji")), id, level)
	case yyQQHasPrefix(text, cmd("kick")):
		return yyQQCmdKick(cf, user, name, arg(cmd("kick")), level)
	case yyQQHasPrefix(text, ".转发"):
		// 用户反馈「.转发 5 不会转发」：能力表里有 relay_history，
		// 但分发这里从来没有这个分支，等于这条指令是死的。
		return yyQQCmdRelay(cf, user, name, arg(".转发"), level)
	case strings.HasPrefix(text, "/"):
		return reply("未知助手指令，请发送 " + cmd("help") + "；常用操作请用 " + cmd("skills") + " 或直接说需求，管理员逃生舱用 " + cmd("command") + "。")
	}
	return nil
}

// yyQQCmdSkills 列出当前权限等级可用的游戏技能（只读）。
// 技能来自 youyuan_qq_skill.go 注册表，与 AI、面板同一份定义。
func yyQQCmdSkills(cf yyQQConfig, level int) []string {
	rows := []string{"✦ 可用游戏技能（去AI化 · 确定性执行）"}
	count := 0
	for _, s := range yyGameSkillAll() {
		if !s.Enabled || level < s.MinRole {
			continue
		}
		count++
		line := s.Name + " → " + s.Label
		if s.Desc != "" {
			line += "（" + s.Desc + "）"
		}
		rows = append(rows, line)
	}
	if count == 0 {
		return []string{"当前没有可用的游戏技能，请联系服主在面板开启。"}
	}
	rows = append(rows, "━━━━━━━━━━━━━━", "直接对机器人说需求即可调用；AI 助手同样只能使用这些技能。")
	return []string{strings.Join(rows, "\n")}
}

// 机器人自身绑定：把机器人 QQ 号绑定到配置的主机器人玩家名（assistant.bot_player）。
// 这样“绑定的玩家就是主机器人”：助手知道自己的游戏身份，/说 等绑定能力也覆盖机器人自己。
func yyQQSyncBotBinding() {
	cf := yyQQSnapshot()
	if cf.OneBot.BotID == "" || cf.Assistant.BotPlayer == "" {
		return
	}
	yyQQHelper.Lock()
	if yyQQHelper.Bindings[cf.OneBot.BotID] != cf.Assistant.BotPlayer {
		yyQQHelper.Bindings[cf.OneBot.BotID] = cf.Assistant.BotPlayer
		yyQQHelper.Dirty = true
	}
	yyQQHelper.Unlock()
}
func yyQQConfirmBinding(cf yyQQConfig, player, text string) bool {
	// 游戏内确认指令同样走别名表（bindgame，默认 .绑定）。
	prefix := yyQQCommandText(cf, "bindgame")
	if !yyQQHasPrefix(text, prefix) {
		return false
	}
	code := strings.TrimSpace(strings.TrimPrefix(text, prefix))
	yyQQHelper.Lock()
	p, ok := yyQQHelper.Proofs[code]
	if !ok || p.Player != player || time.Now().After(p.Until) {
		yyQQHelper.Unlock()
		return true
	}
	for user, name := range yyQQHelper.Bindings {
		if name == player && user != p.User {
			yyQQHelper.Unlock()
			return true
		}
	}
	old := yyQQHelper.Bindings[p.User]
	yyQQHelper.Bindings[p.User] = player
	e := yyQQHelperSaveLocked()
	if e != nil {
		if old == "" {
			delete(yyQQHelper.Bindings, p.User)
		} else {
			yyQQHelper.Bindings[p.User] = old
		}
	} else {
		delete(yyQQHelper.Proofs, code)
	}
	yyQQHelper.Unlock()
	if e != nil {
		yyQQFeedback("绑定保存失败："+e.Error(), p.Message)
	} else {
		yyQQFeedback("绑定成功："+player, p.Message)
	}
	return true
}
func yyQQOnlinePlayers() ([]string, error) {
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("游戏尚未连接")
	}
	out, timed, e := bm.SendCommandCh(ChannelAI, "list", 3*time.Second)
	if e != nil {
		return nil, e
	}
	if timed || out == nil {
		return nil, fmt.Errorf("在线名单查询超时，未得到服务器确认")
	}
	names := parseListOutput(out)
	sort.Strings(names)
	return names, nil
}
func yyQQRelayGame(text string) error {
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		return fmt.Errorf("游戏尚未连接，消息未转发")
	}
	// tellraw transports the full text and cannot reinterpret it as a slash command.
	return yyQQTellraw("@a", text)
}
func yyQQTellraw(selector, text string) error {
	command := "tellraw " + selector + " " + mustJSON(map[string]any{"rawtext": []any{map[string]any{"text": text}}})
	value, e := yyPluginCommandCall(getBotManagerLocked(), "game.execute", []any{command}, yyDefaultCommandOptions())
	if e != nil {
		return e
	}
	r := value.(map[string]any)
	if r["confirmed"] != true {
		return fmt.Errorf("消息已提交，服务器确认超时；没有重发")
	}
	if r["success"] != true {
		return fmt.Errorf("服务器拒绝发言：%v", r["output"])
	}
	return nil
}
func yyQQChatLogs(n int) string {
	yyObservation.Lock()
	rows := append([]yyChatRecord(nil), yyObservation.Chat...)
	yyObservation.Unlock()
	if n > len(rows) {
		n = len(rows)
	}
	lines := []string{fmt.Sprintf("最近%d条聊天 · 最新在前（当前设备已记录）", n)}
	for i := len(rows) - 1; i >= len(rows)-n; i-- {
		v := rows[i]
		at, e := time.Parse(time.RFC3339Nano, v.Time)
		stamp := v.Time
		if e == nil {
			stamp = at.In(time.Local).Format("01-02 15:04:05")
		}
		lines = append(lines, stamp+" "+v.Player+"："+v.Message)
	}
	return strings.Join(lines, "\n")
}
func yyQQExecuteText(cf yyQQConfig, text string) string {
	if !strings.Contains(text, "\n") {
		text = strings.Join(yySplitConsole("/"+strings.TrimLeft(text, "/")), "\n")
	}
	plan, e := yyParseChains(text)
	if e != nil {
		return "语法错误：" + e.Error()
	}
	if plan.Commands < 1 || plan.Commands > 128 {
		return "每次执行1..128条命令，大任务请在APK命令链管理。"
	}
	total := 0
	for _, c := range plan.Chains {
		n := c.Repeat
		if n < 1 {
			n = 1
		}
		total += len(c.Nodes) * n
	}
	if total > 128 {
		return "展开重复后最多128条命令。"
	}
	check := cf
	check.Command.Permission = "所有人"
	for _, c := range plan.Chains {
		for _, node := range c.Nodes {
			if e = yyQQCommandAllowed(check, "", "", "/"+node.Command); e != nil {
				return e.Error()
			}
		}
	}
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		return "游戏尚未连接，命令未执行"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	sent, success, skipped, unknown := 0, 0, 0, 0
	failures := []string{}
	yyChainExecuteDetailed(ctx, plan, true, true, true, 20*time.Millisecond, func(node yyChainNode) (bool, error) {
		sent++
		channel := node.Channel
		if channel == "" {
			channel = "ai"
		}
		value, e := yyPluginCommandCall(bm, "game.execute", []any{node.Command, map[string]any{"channel": channel}}, yyDefaultCommandOptions())
		if e != nil {
			return false, e
		}
		r := value.(map[string]any)
		if r["confirmed"] != true {
			unknown++
			return false, fmt.Errorf("第%d行结果未知，未重发", node.Line)
		}
		return r["success"] == true, nil
	}, func(ok, skip bool, e error) {
		if ok {
			success++
		}
		if skip {
			skipped++
		}
		if e != nil && len(failures) < 8 {
			failures = append(failures, e.Error())
		}
	})
	detail := fmt.Sprintf("魔法通道执行回执：提交%d，成功%d，跳过%d，未知%d", sent, success, skipped, unknown)
	if ctx.Err() != nil {
		detail += "\n任务超时，剩余命令未执行"
	}
	if len(failures) > 0 {
		detail += "\n" + strings.Join(failures, "\n")
	}
	return detail
}
func yyQQHelperRecord(kind, player, message string) {
	if player == "" {
		return
	}
	yyQQHelper.Lock()
	v := yyQQHelper.Stats[player]
	if v == nil {
		if len(yyQQHelper.Stats) >= 10000 {
			yyQQHelper.Unlock()
			return
		}
		v = &yyQQPlayerStats{}
		yyQQHelper.Stats[player] = v
	}
	now := time.Now()
	switch kind {
	case "observe":
		if v.Joined.IsZero() {
			v.Joined = now
		}
	case "join":
		if v.Joined.IsZero() {
			v.Joined = now
			v.LastJoin = now
		}
	case "leave":
		if !v.Joined.IsZero() {
			v.OnlineSeconds += int64(now.Sub(v.Joined).Seconds())
			v.Joined = time.Time{}
		}
		v.LastLeave = now
	case "chat":
		v.Messages++
		v.LastMessage = yyQQCardClip(message, 300)
	}
	yyQQHelper.Dirty = true
	yyQQHelper.Unlock()
}
func yyQQHelperPause() {
	yyQQHelper.Lock()
	defer yyQQHelper.Unlock()
	now := time.Now()
	for _, v := range yyQQHelper.Stats {
		if v != nil && !v.Joined.IsZero() {
			v.OnlineSeconds += int64(now.Sub(v.Joined).Seconds())
			v.Joined = time.Time{}
			yyQQHelper.Dirty = true
		}
	}
}
func yyQQAssistantEvent(bm *BotManager, e map[string]any) {
	if bm != getBotManagerLocked() {
		return
	}
	cf := yyQQSnapshot()
	if !cf.Assistant.Enabled {
		return
	}
	kind, _ := e["type"].(string)
	player, _ := e["player"].(string)
	if player == "" {
		return
	}
	if kind == "join" || kind == "leave" {
		yyQQHelperRecord(kind, player, "")
	}
	if !cf.Enabled {
		return
	}
	text := ""
	switch {
	case kind == "death":
		// Record before the播报 switch: the grace window must apply even when
		// the death broadcast itself is switched off, otherwise a silent death
		// still leaks out as "离开服务器 / 进入服务器".
		playerInfoMgr.MarkDeath(player)
		if cf.Assistant.Death {
			text = player + " 死亡（服务器事件）"
		}
	case kind == "join" && cf.Assistant.Join:
		text = player + " 进入服务器"
	case kind == "leave" && cf.Assistant.Leave:
		text = player + " 离开服务器"
	}
	if text != "" {
		select {
		case yyQQOutgoing <- yyQQOutgoingJob{Text: text}:
		default:
			yyQQLog("通知队列已满，未发送")
		}
	}
}

// yyQQCmdRoles .权限 —— 回一份"谁是超管/管理员、我是什么角色"的名单。
// 名字优先用绑定表反查，拿不到就只给 QQ 号。
func yyQQCmdRoles(cf yyQQConfig, user, name string, level int) []string {
	roleName := func(lv int) string { return yyQQCapLevelText(lv) }
	label := func(qq string) string {
		yyQQHelper.Lock()
		var player string
		for p, bound := range yyQQHelper.Bindings {
			if bound == qq {
				player = p
				break
			}
		}
		yyQQHelper.Unlock()
		if player != "" {
			return player + "（" + qq + "）"
		}
		return qq
	}
	lines := []string{"✦ 互通权限", "━━━━━━━━━━━━━━"}
	lines = append(lines, "你："+(name+" ")+"（"+user+"）· "+roleName(level))
	if len(cf.Assistant.SuperAdmins) == 0 && len(cf.Assistant.Admins) == 0 {
		lines = append(lines, "还没有配置管理员：去工具箱「群服互通 → 接入设置」里填 QQ 号。")
		return []string{strings.Join(lines, "\n")}
	}
	if len(cf.Assistant.SuperAdmins) > 0 {
		lines = append(lines, "超级管理员（可批准危险指令）：")
		for _, qq := range cf.Assistant.SuperAdmins {
			lines = append(lines, "· "+label(qq))
		}
	}
	if len(cf.Assistant.Admins) > 0 {
		lines = append(lines, "管理员：")
		for _, qq := range cf.Assistant.Admins {
			lines = append(lines, "· "+label(qq))
		}
	}
	lines = append(lines, "", "危险指令（op/ban/fill 等）需要超管在群里回「同意 编号」。")
	return []string{strings.Join(lines, "\n")}
}
