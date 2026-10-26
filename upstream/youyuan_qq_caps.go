package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 机器人能力注册表：面板、限流策略、开关、群指令共用同一份定义，
// 避免「面板列一个、实际执行另一个」的不一致。
type yyQQCapability struct {
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Icon        string `json:"icon"`
	Desc        string `json:"desc"`
	MinLevel    int    `json:"min_level"`
	MinLevelTxt string `json:"min_level_text"`
	CoolSeconds int    `json:"cool_seconds"`
	GapSeconds  int    `json:"gap_seconds"`
	Daily       int    `json:"daily"`
	Default     bool   `json:"default"`
	OneBotOnly  bool   `json:"onebot_only"`
	Command     string `json:"command,omitempty"`
	// Pending = 这行的时间/次数是「兜底值」而不是「定论」。
	// 用户明确要求「限制不要搞默认的，直接弄成待定，让用户自己进去选」。
	// 但直接把限制关掉是降级（等于没有任何风控），所以做法是：
	// 兜底值继续生效保证安全，面板标成「待定」并明确写出兜底数值，
	// 玩家一旦自己设过（custom=true），pending 自动消失。
	Pending bool `json:"pending,omitempty"`
}

// yyQQCapabilities 能力注册表。
//
// 关于 Pending：所有能力的时间/次数都标了 Pending=true，意思是
// 「这些数字是兜底，不是替你做的决定」。用户原话：
// 「他的那个限制，你不要搞默认的，比如说每日发言冷却多久，
//
//	你直接弄成待定，让用户自己进去选」。
//
// 兜底值必须留着——直接关掉限流等于没有风控，那是降级不是待定。
var yyQQCapabilities = []yyQQCapability{
	{Kind: "like", Label: "点赞", Icon: "👍", Desc: "给群友的名片点赞，每人每天次数有限", MinLevel: 0, CoolSeconds: 120, GapSeconds: 10, Daily: 3, Default: true, OneBotOnly: true, Command: ".点赞 QQ号 [次数]", Pending: true},
	{Kind: "emoji_like", Label: "表情回应", Icon: "😄", Desc: "对消息贴上表情回应，可自定义表情 ID", MinLevel: 0, CoolSeconds: 0, GapSeconds: 3, Daily: 60, Default: true, OneBotOnly: true, Command: ".表情 [表情ID]", Pending: true},
	{Kind: "face", Label: "收藏表情", Icon: "🖼", Desc: "读取机器人 QQ 的收藏表情包，看清图片再决定发哪个", MinLevel: 0, CoolSeconds: 0, GapSeconds: 0, Daily: 0, Default: true, OneBotOnly: true, Command: ".表情包", Pending: true},
	{Kind: "qzone", Label: "发动态", Icon: "📮", Desc: "以机器人身份发 QQ 空间动态", MinLevel: 1, CoolSeconds: 300, GapSeconds: 30, Daily: 6, Default: true, OneBotOnly: true, Command: ".动态 内容", Pending: true},
	{Kind: "delete_qzone", Label: "删动态", Icon: "🗑", Desc: "删除机器人自己发的动态", MinLevel: 2, CoolSeconds: 30, GapSeconds: 5, Daily: 20, Default: true, OneBotOnly: true, Command: ".删动态 tid", Pending: true},
	{Kind: "avatar", Label: "改头像", Icon: "🧑", Desc: "更换机器人 QQ 头像", MinLevel: 2, CoolSeconds: 600, GapSeconds: 30, Daily: 10, Default: true, OneBotOnly: true, Command: ".改头像 图片URL", Pending: true},
	{Kind: "long_nick", Label: "改签名", Icon: "✒️", Desc: "修改机器人个性签名", MinLevel: 2, CoolSeconds: 600, GapSeconds: 30, Daily: 10, Default: true, OneBotOnly: true, Command: ".改签名 内容", Pending: true},
	{Kind: "nick", Label: "改昵称", Icon: "🏷", Desc: "修改机器人 QQ 昵称", MinLevel: 2, CoolSeconds: 600, GapSeconds: 30, Daily: 10, Default: true, OneBotOnly: true, Command: ".改昵称 名字", Pending: true},
	{Kind: "kick", Label: "踢人", Icon: "👢", Desc: "把指定群成员移出群；AI 技能与群指令共用，需相应权限", MinLevel: 1, CoolSeconds: 10, GapSeconds: 5, Daily: 20, Default: true, OneBotOnly: true, Command: ".踢 QQ号 [原因]", Pending: true},
	// recall 原为 Default:false。用户这一轮明确「默认全开，每个给独立开关」，
	// 所以改成默认开；不想让机器人自动撤回就去面板关掉这一项。
	{Kind: "recall", Label: "撤回消息", Icon: "📥", Desc: "撤回群里的消息；群规违规时由系统自动撤，超管也可手动用", MinLevel: 2, CoolSeconds: 5, GapSeconds: 3, Daily: 100, Default: true, OneBotOnly: true, Pending: true},
	// ban 默认开启：用户明确要求「AI 是群管理员，恼羞成怒时可以禁言玩家」。
	// 限流（30秒冷却/每日30次）与留痕照旧，玩家可在面板关闭。
	{Kind: "ban", Label: "禁言", Icon: "🔇", Desc: "禁言群成员（群规 recall_ban 处置与 AI 技能共用，默认10分钟）", MinLevel: 2, CoolSeconds: 30, GapSeconds: 10, Daily: 30, Default: true, OneBotOnly: true, Pending: true},
	{Kind: "relay_history", Label: "转发聊天记录", Icon: "📋", Desc: "把群聊合并转发到当前群", MinLevel: 1, CoolSeconds: 60, GapSeconds: 10, Daily: 10, Default: true, OneBotOnly: true, Command: ".转发 条数", Pending: true},
	{Kind: "poke", Label: "拍一拍", Icon: "👋", Desc: "拍一拍群友（或拍机器人自己），常用于打招呼", MinLevel: 0, CoolSeconds: 3, GapSeconds: 2, Daily: 50, Default: true, OneBotOnly: true, Command: ".拍 [QQ号]", Pending: true},
	{Kind: "sticker", Label: "发表情包", Icon: "🎨", Desc: "按序号发某一张收藏表情，或用图片链接发图", MinLevel: 0, CoolSeconds: 3, GapSeconds: 2, Daily: 40, Default: true, OneBotOnly: true, Command: ".发图 序号|图片URL", Pending: true},
	// voice：OneBot record 段。文本走配置的 TTS 模型合成；URL 直发不需要模型。
	// MinLevel 1：语音能把任意文字念出来，也比文字更扰民，先给管理员用。
	{Kind: "voice", Label: "发语音", Icon: "🎙", Desc: "把一段文字转成语音发到群里（需要配置语音模型）", MinLevel: 1, CoolSeconds: 10, GapSeconds: 5, Daily: 50, Default: true, OneBotOnly: true, Command: ".语音 文本或音频URL", Pending: true},
	{Kind: "game_skill", Label: "游戏技能", Icon: "🎮", Desc: "按预定义技能表确定性地执行游戏内指令（查在线名单、广播、传送等），不经过AI自由发挥", MinLevel: 0, CoolSeconds: 0, GapSeconds: 3, Daily: 0, Default: true, Command: ".技能", Pending: true},
}

// yyQQCapLevelText 角色名。越界不 panic，退化成"成员"。
func yyQQCapLevelText(level int) string {
	switch level {
	case 1:
		return "管理员"
	case 2:
		return "超级管理员"
	default:
		return "成员"
	}
}

func yyQQCapabilityOf(kind string) (yyQQCapability, bool) {
	for _, c := range yyQQCapabilities {
		if c.Kind == kind {
			return c, true
		}
	}
	return yyQQCapability{}, false
}

// yyQQCapLimit 是玩家对某个能力的限流覆盖。字段用指针：
// nil 表示"这项没改，继续用默认"，非 nil 才是玩家的值——
// 这样 0（=不限制）和"没设置"能被区分开，升级默认值也不会踩掉玩家配置。
type yyQQCapLimit struct {
	MinLevel    *int `json:"min_level,omitempty"`
	CoolSeconds *int `json:"cool_seconds,omitempty"`
	GapSeconds  *int `json:"gap_seconds,omitempty"`
	Daily       *int `json:"daily,omitempty"`
}

func (l yyQQCapLimit) empty() bool {
	return l.MinLevel == nil && l.CoolSeconds == nil && l.GapSeconds == nil && l.Daily == nil
}

// yyQQCapLimitNormalize 夹紧到合理范围，防止玩家填负数把限流关掉之后
// 又填个 99999999 让 gate 的 map 撑爆。
func yyQQCapLimitNormalize(l yyQQCapLimit) yyQQCapLimit {
	out := yyQQCapLimit{}
	clamp := func(p *int, lo, hi int) *int {
		if p == nil {
			return nil
		}
		v := *p
		if v < lo {
			v = lo
		}
		if v > hi {
			v = hi
		}
		return &v
	}
	out.MinLevel = clamp(l.MinLevel, 0, 2)
	out.CoolSeconds = clamp(l.CoolSeconds, 0, 86400)
	out.GapSeconds = clamp(l.GapSeconds, 0, 3600)
	out.Daily = clamp(l.Daily, 0, 10000)
	return out
}

// yyQQCapEffective 把玩家覆盖叠加到注册表默认值上，返回实际生效的能力定义。
func yyQQCapEffective(c yyQQCapability, a yyQQAssistantConfig) yyQQCapability {
	l, ok := a.CapLimits[c.Kind]
	if !ok || l.empty() {
		return c
	}
	if l.MinLevel != nil {
		// 夹紧：面板会把 min_level 直接当数组下标取角色名，
		// 越界（比如 /api/qq/config 塞了 9）会让整个能力接口 panic。
		lv := *l.MinLevel
		if lv < 0 {
			lv = 0
		}
		if lv > 2 {
			lv = 2
		}
		c.MinLevel = lv
	}
	if l.CoolSeconds != nil {
		c.CoolSeconds = *l.CoolSeconds
	}
	if l.GapSeconds != nil {
		c.GapSeconds = *l.GapSeconds
	}
	if l.Daily != nil {
		c.Daily = *l.Daily
	}
	return c
}

// yyQQCapEffectiveOf 按 kind 取生效定义；找不到 kind 返回 false。
func yyQQCapEffectiveOf(a yyQQAssistantConfig, kind string) (yyQQCapability, bool) {
	base, ok := yyQQCapabilityOf(kind)
	if !ok {
		return yyQQCapability{}, false
	}
	return yyQQCapEffective(base, a), true
}

// yyQQCapEnabled 读取开关；未显式设置时用注册表里的默认值。
func yyQQCapEnabled(a yyQQAssistantConfig, kind string) bool {
	if a.Caps != nil {
		if v, ok := a.Caps[kind]; ok {
			return v
		}
	}
	c, ok := yyQQCapabilityOf(kind)
	if !ok {
		return false
	}
	return c.Default
}

// yyQQCapsView 给面板用的完整视图（含开关状态）。
// 同时回传默认值与"是否被玩家改过"，前端才能显示"恢复默认"。
func yyQQCapsView(a yyQQAssistantConfig) []map[string]any {
	out := make([]map[string]any, 0, len(yyQQCapabilities))
	for _, base := range yyQQCapabilities {
		c := yyQQCapEffective(base, a)
		custom := c.MinLevel != base.MinLevel || c.CoolSeconds != base.CoolSeconds ||
			c.GapSeconds != base.GapSeconds || c.Daily != base.Daily
		out = append(out, map[string]any{
			"kind": c.Kind, "label": c.Label, "icon": c.Icon, "desc": c.Desc,
			"min_level": c.MinLevel, "min_level_text": yyQQCapLevelText(c.MinLevel),
			"cool_seconds": c.CoolSeconds, "gap_seconds": c.GapSeconds, "daily": c.Daily,
			"default": base.Default, "onebot_only": base.OneBotOnly, "command": base.Command,
			"enabled": yyQQCapEnabled(a, c.Kind),
			"custom":  custom,
			// pending = 「这项还是兜底值，等你定」。
			// 玩家一旦自己改过（custom），pending 立刻消失，不再唠叨。
			"pending": base.Pending && !custom,
			"defaults": map[string]any{
				"min_level": base.MinLevel, "cool_seconds": base.CoolSeconds,
				"gap_seconds": base.GapSeconds, "daily": base.Daily,
			},
		})
	}
	return out
}

// ---- 新增 OneBot 动作 ----

func yyQQActionEmojiLike(messageID, emojiID string, set bool) error {
	if strings.TrimSpace(messageID) == "" {
		return fmt.Errorf("缺少消息 ID，无法贴表情")
	}
	if strings.TrimSpace(emojiID) == "" {
		emojiID = "128077" // 👍 默认
	}
	_, e := yyQQOneBotCall("set_msg_emoji_like", map[string]any{"message_id": messageID, "emoji_id": emojiID, "set": set})
	return e
}

func yyQQActionKick(groupID, userID string, reject bool) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("请指定要移出的 QQ 号")
	}
	if strings.TrimSpace(groupID) == "" {
		return fmt.Errorf("缺少群号")
	}
	_, e := yyQQOneBotCall("set_group_kick", map[string]any{"group_id": groupID, "user_id": userID, "reject_add_request": reject})
	return e
}

// yyQQActionRecall 撤回群里的某条消息（群规违规时由系统或超管使用）。
func yyQQActionRecall(groupID, messageID string) error {
	if strings.TrimSpace(messageID) == "" {
		return fmt.Errorf("缺少消息ID")
	}
	if strings.TrimSpace(groupID) == "" {
		return fmt.Errorf("缺少群号")
	}
	_, e := yyQQOneBotCall("delete_msg", map[string]any{"message_id": messageID})
	return e
}

// yyQQActionBan 禁言群成员 minutes 分钟（群规违规时由系统或超管使用）。
func yyQQActionBan(groupID, userID string, minutes int) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("请指定要禁言的 QQ 号")
	}
	if strings.TrimSpace(groupID) == "" {
		return fmt.Errorf("缺少群号")
	}
	if minutes <= 0 {
		minutes = yyQQModBanMinutes
	}
	_, e := yyQQOneBotCall("set_group_ban", map[string]any{"group_id": groupID, "user_id": userID, "duration": minutes * 60})
	return e
}

// yyQQFetchFaces 读取机器人 QQ 的收藏表情（图片地址）。
func yyQQFetchFaces() ([]map[string]any, error) {
	data, e := yyQQOneBotCall("fetch_custom_face", map[string]any{})
	if e != nil {
		return nil, e
	}
	rows := []map[string]any{}
	add := func(id, url, name string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		row := map[string]any{"url": url}
		if id != "" {
			row["id"] = id
		}
		if name != "" {
			row["name"] = name
		}
		rows = append(rows, row)
	}
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			switch t := item.(type) {
			case string:
				if strings.HasPrefix(t, "http") {
					add(strconv.Itoa(i+1), t, "")
				} else {
					add(t, t, "")
				}
			case map[string]any:
				add(yyQQStableID(t["id"]), fmt.Sprint(t["url"]), fmt.Sprint(t["name"]))
			}
		}
	case map[string]any:
		// NapCat 实际返回 {count, result:[...]}，result 可能是地址或对象。
		if list, ok := v["result"].([]any); ok {
			for i, item := range list {
				switch t := item.(type) {
				case string:
					if strings.HasPrefix(t, "http") {
						add(strconv.Itoa(i+1), t, "")
					} else {
						add(t, t, "")
					}
				case map[string]any:
					add(yyQQStableID(t["id"]), fmt.Sprint(t["url"]), fmt.Sprint(t["name"]))
				}
			}
		}
	}
	return rows, nil
}

// ---- 面板接口 ----

// yyQQOneBotReadOnly 允许面板只读调用 OneBot 扩展动作：不改变任何 QQ 状态，
// 供「自由连 API」查看资料、群成员、收藏表情等。
var yyQQOneBotReadOnly = map[string]bool{
	"get_login_info": true, "get_group_list": true, "get_friend_list": true,
	"get_stranger_info": true, "get_group_member_list": true, "get_group_member_info": true,
	"get_msg": true, "fetch_custom_face": true, "get_online_clients": true,
	"get_status": true, "get_version_info": true, "get_cookies": true,
	// get_image 只是按 file 取图片地址，不改任何 QQ 状态（与 get_msg 同级）。
	// 识图换链兜底（表情包/市场表情只有 file 没有 url）走的就是它，
	// 面板要能诊断「为什么这张图 AI 看不到」，所以开放。
	"get_image": true,
}

func yyQQCapRoutes(mux *http.ServeMux) {
	// 帮助菜单同属「能力/技能注册表」这一块：它读的是同一份注册表，
	// 挂在这里可以保证 yyRegisterQQ 里只需一次调用（youyuan_qq.go 不动）。
	yyQQHelpRegister(mux)
	// 知识库面板（youyuan_qq_knowledge.go）：同样挂在这个扩展点上，
	// 避免再动 youyuan_qq.go。它内部自己从 yyQQPath 反推数据目录。
	yyQQKnowledgeRoutes(mux)
	mux.HandleFunc("/api/qq/caps", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			cf := yyQQSnapshot()
			writeJSON(w, map[string]any{"ok": true, "caps": yyQQCapsView(cf.Assistant), "adapter": cf.Adapter})
			return
		}
		if r.Method != "POST" {
			http.Error(w, "GET/POST required", 405)
			return
		}
		var p struct {
			Caps map[string]bool `json:"caps"`
			// Limits 里值为 null 或空对象 = 恢复该项默认（删掉覆盖）。
			Limits map[string]yyQQCapLimit `json:"limits"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		for kind := range p.Caps {
			if _, ok := yyQQCapabilityOf(kind); !ok {
				writeJSON(w, map[string]any{"ok": false, "error": "未知能力：" + kind})
				return
			}
		}
		limits := make(map[string]yyQQCapLimit, len(p.Limits))
		for kind, lim := range p.Limits {
			if _, ok := yyQQCapabilityOf(kind); !ok {
				writeJSON(w, map[string]any{"ok": false, "error": "未知能力：" + kind})
				return
			}
			limits[kind] = yyQQCapLimitNormalize(lim)
		}
		e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
			if a.Caps == nil {
				a.Caps = map[string]bool{}
			}
			for kind, on := range p.Caps {
				a.Caps[kind] = on
			}
			if a.CapLimits == nil && len(limits) > 0 {
				a.CapLimits = map[string]yyQQCapLimit{}
			}
			for kind, lim := range limits {
				if lim.empty() {
					delete(a.CapLimits, kind)
					continue
				}
				a.CapLimits[kind] = lim
			}
		})
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		cf := yyQQSnapshot()
		writeJSON(w, map[string]any{"ok": true, "caps": yyQQCapsView(cf.Assistant)})
	})

	mux.HandleFunc("/api/qq/bot/profile", func(w http.ResponseWriter, r *http.Request) {
		cf := yyQQSnapshot()
		out := map[string]any{"ok": true, "adapter": cf.Adapter, "state": yyQQStateNow()}
		info, e := yyQQOneBotCall("get_login_info", map[string]any{})
		if e != nil {
			out["error"] = "读取机器人资料失败：" + e.Error()
			writeJSON(w, out)
			return
		}
		if m, ok := info.(map[string]any); ok {
			uin := yyQQStableID(m["user_id"])
			out["user_id"] = uin
			out["nickname"] = fmt.Sprint(m["nickname"])
			if uin != "" {
				out["avatar"] = "https://q1.qlogo.cn/g?b=qq&nk=" + uin + "&s=640"
			}
		}
		if groups, e := yyQQOneBotCall("get_group_list", map[string]any{}); e == nil {
			if list, ok := groups.([]any); ok {
				out["group_count"] = len(list)
			}
		}
		if friends, e := yyQQOneBotCall("get_friend_list", map[string]any{}); e == nil {
			if list, ok := friends.([]any); ok {
				out["friend_count"] = len(list)
			}
		}
		out["bot_player"] = cf.Assistant.BotPlayer
		out["group_id"] = yyQQGroupString(cf.GroupID)
		writeJSON(w, out)
	})

	mux.HandleFunc("/api/qq/faces", func(w http.ResponseWriter, r *http.Request) {
		cf := yyQQSnapshot()
		if cf.Adapter != "onebot" {
			writeJSON(w, map[string]any{"ok": false, "error": "收藏表情只在 OneBot(NapCat) 适配器下可用"})
			return
		}
		if !yyQQCapEnabled(cf.Assistant, "face") {
			writeJSON(w, map[string]any{"ok": false, "error": "「收藏表情」能力已在设置里关闭"})
			return
		}
		rows, e := yyQQFetchFaces()
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "读取收藏表情失败：" + e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "faces": rows, "count": len(rows)})
	})

	mux.HandleFunc("/api/qq/onebot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		var p struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		if !yyQQOneBotReadOnly[p.Action] {
			writeJSON(w, map[string]any{"ok": false, "error": "这个动作不允许面板直接调用（只开放只读查询）"})
			return
		}
		cf := yyQQSnapshot()
		if cf.Adapter != "onebot" {
			writeJSON(w, map[string]any{"ok": false, "error": "需要 OneBot(NapCat) 适配器"})
			return
		}
		data, e := yyQQOneBotCall(p.Action, p.Params)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "action": p.Action, "data": data})
	})
}

func yyQQStateNow() string {
	yyQQMu.Lock()
	defer yyQQMu.Unlock()
	return yyQQState
}

// ---- 群指令 ----

func yyQQCmdEmoji(cf yyQQConfig, user, name, arg, messageID string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	emoji := strings.TrimSpace(arg)
	e := yyQQRunAction("emoji_like", user, name, messageID, "emoji="+emoji, level, func() (string, error) {
		return "", yyQQActionEmojiLike(messageID, emoji, true)
	})
	if e != nil {
		return reply("表情回应失败：" + e.Error())
	}
	return nil
}

func yyQQCmdKick(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	f := strings.Fields(arg)
	if len(f) == 0 || !yyQQNumericQQ(f[0]) {
		return reply("格式：.踢 QQ号 [原因]")
	}
	reason := strings.TrimSpace(strings.TrimPrefix(arg, f[0]))
	group := yyQQGroupString(cf.GroupID)
	e := yyQQRunAction("kick", user, name, f[0], "群 "+group+" 原因："+reason, level, func() (string, error) {
		return "", yyQQActionKick(group, f[0], false)
	})
	if e != nil {
		return reply("移出失败：" + e.Error())
	}
	return reply("已把 " + f[0] + " 移出本群。")
}

func yyQQCmdFaces(cf yyQQConfig, level int) []string {
	rows, e := yyQQFetchFaces()
	if e != nil {
		return []string{"读取收藏表情失败：" + e.Error()}
	}
	if len(rows) == 0 {
		return []string{"机器人还没有收藏表情。"}
	}
	lines := []string{"✦ 机器人收藏表情（" + strconv.Itoa(len(rows)) + " 个）", "━━━━━━━━━━━━━━"}
	for i, row := range rows {
		if i >= 12 {
			lines = append(lines, "……还有 "+strconv.Itoa(len(rows)-12)+" 个，去工具箱面板看全部")
			break
		}
		lines = append(lines, fmt.Sprint(row["url"]))
	}
	return []string{strings.Join(lines, "\n")}
}

var _ = time.Now

// ---- 转发聊天记录 ----

// yyQQRelayHistory 把最近 n 条群消息合并转发到当前群。
// 之前只有能力表里的一行定义和一句「.转发 条数」的说明，没有任何实现，
// 所以用户在群里发 .转发 5 完全没反应。
func yyQQRelayHistory(cf yyQQConfig, n int) error {
	if cf.Adapter != "onebot" {
		return fmt.Errorf("合并转发需要 OneBot(NapCat) 适配器，QQ 官方适配器没有这个接口")
	}
	group := yyQQGroupString(cf.GroupID)
	if group == "" {
		return fmt.Errorf("还没选群")
	}
	rows := yyQQRecentMessages(cf, n)
	if len(rows) == 0 {
		return fmt.Errorf("最近没有可转发的消息（机器人只转发自己看到过的群消息）")
	}
	nodes := make([]yyForwardNode, 0, len(rows))
	for _, m := range rows {
		uin := int64(0)
		if _, e := fmt.Sscan(strings.TrimSpace(m.User), &uin); e != nil {
			uin = 0
		}
		nodes = append(nodes, yyForwardNode{UIN: uin, Name: m.Name, Content: m.Text})
	}
	return yyForwardBuild(group, nodes)
}

// yyQQCmdRelay .转发 N —— 受能力开关与限流约束（relay_history）。
func yyQQCmdRelay(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	n := 5
	if f := strings.Fields(arg); len(f) > 0 {
		if v, e := strconv.Atoi(f[0]); e == nil {
			n = v
		}
	}
	if n < 1 {
		n = 1
	}
	if n > 50 {
		n = 50
	}
	e := yyQQRunAction("relay_history", user, name, "", "条数="+strconv.Itoa(n), level, func() (string, error) {
		return "", yyQQRelayHistory(cf, n)
	})
	if e != nil {
		return reply("转发失败：" + e.Error())
	}
	return reply("已转发最近 " + strconv.Itoa(n) + " 条消息。")
}
