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
}

var yyQQCapabilities = []yyQQCapability{
	{Kind: "like", Label: "点赞", Icon: "👍", Desc: "给群友的名片点赞，每人每天次数有限", MinLevel: 0, CoolSeconds: 120, GapSeconds: 10, Daily: 3, Default: true, OneBotOnly: true, Command: ".点赞 QQ号 [次数]"},
	{Kind: "emoji_like", Label: "表情回应", Icon: "😄", Desc: "对消息贴上表情回应，可自定义表情 ID", MinLevel: 0, CoolSeconds: 0, GapSeconds: 3, Daily: 60, Default: true, OneBotOnly: true, Command: ".表情 [表情ID]"},
	{Kind: "face", Label: "收藏表情", Icon: "🖼", Desc: "读取机器人 QQ 的收藏表情包，看清图片再决定发哪个", MinLevel: 0, CoolSeconds: 0, GapSeconds: 0, Daily: 0, Default: true, OneBotOnly: true, Command: ".表情包"},
	{Kind: "qzone", Label: "发动态", Icon: "📮", Desc: "以机器人身份发 QQ 空间动态", MinLevel: 1, CoolSeconds: 300, GapSeconds: 30, Daily: 6, Default: true, OneBotOnly: true, Command: ".动态 内容"},
	{Kind: "delete_qzone", Label: "删动态", Icon: "🗑", Desc: "删除机器人自己发的动态", MinLevel: 2, CoolSeconds: 30, GapSeconds: 5, Daily: 20, Default: true, OneBotOnly: true, Command: ".删动态 tid"},
	{Kind: "avatar", Label: "改头像", Icon: "🧑", Desc: "更换机器人 QQ 头像", MinLevel: 2, CoolSeconds: 600, GapSeconds: 30, Daily: 10, Default: true, OneBotOnly: true, Command: ".改头像 图片URL"},
	{Kind: "long_nick", Label: "改签名", Icon: "✒️", Desc: "修改机器人个性签名", MinLevel: 2, CoolSeconds: 600, GapSeconds: 30, Daily: 10, Default: true, OneBotOnly: true, Command: ".改签名 内容"},
	{Kind: "nick", Label: "改昵称", Icon: "🏷", Desc: "修改机器人 QQ 昵称", MinLevel: 2, CoolSeconds: 600, GapSeconds: 30, Daily: 10, Default: true, OneBotOnly: true, Command: ".改昵称 名字"},
	{Kind: "kick", Label: "踢人", Icon: "👢", Desc: "把指定群成员移出群；默认关闭，开启后仍需管理员授权", MinLevel: 1, CoolSeconds: 10, GapSeconds: 5, Daily: 20, Default: false, OneBotOnly: true, Command: ".踢 QQ号 [原因]"},
	{Kind: "relay_history", Label: "转发聊天记录", Icon: "📋", Desc: "把群聊合并转发到当前群", MinLevel: 1, CoolSeconds: 60, GapSeconds: 10, Daily: 10, Default: true, OneBotOnly: true, Command: ".转发 条数"},
	{Kind: "game_skill", Label: "游戏技能", Icon: "🎮", Desc: "按预定义技能表确定性地执行游戏内指令（查在线名单、广播、传送等），不经过AI自由发挥", MinLevel: 0, CoolSeconds: 0, GapSeconds: 3, Daily: 0, Default: true, Command: ".技能"},
}

func yyQQCapabilityOf(kind string) (yyQQCapability, bool) {
	for _, c := range yyQQCapabilities {
		if c.Kind == kind {
			return c, true
		}
	}
	return yyQQCapability{}, false
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
func yyQQCapsView(a yyQQAssistantConfig) []map[string]any {
	out := make([]map[string]any, 0, len(yyQQCapabilities))
	for _, c := range yyQQCapabilities {
		out = append(out, map[string]any{
			"kind": c.Kind, "label": c.Label, "icon": c.Icon, "desc": c.Desc,
			"min_level": c.MinLevel, "min_level_text": []string{"成员", "管理员", "超级管理员"}[c.MinLevel],
			"cool_seconds": c.CoolSeconds, "gap_seconds": c.GapSeconds, "daily": c.Daily,
			"default": c.Default, "onebot_only": c.OneBotOnly, "command": c.Command,
			"enabled": yyQQCapEnabled(a, c.Kind),
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
}

func yyQQCapRoutes(mux *http.ServeMux) {
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
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		for kind := range p.Caps {
			if _, ok := yyQQCapabilityOf(kind); !ok {
				writeJSON(w, map[string]any{"ok": false, "error": "未知能力：" + kind})
				return
			}
		}
		e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
			if a.Caps == nil {
				a.Caps = map[string]bool{}
			}
			for kind, on := range p.Caps {
				a.Caps[kind] = on
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
