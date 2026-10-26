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
	RelayChat            bool              `json:"relay_chat"`
	ImageModel           string            `json:"image_model"`
	Enabled              bool              `json:"enabled"`
	SuperAdmins          []string          `json:"super_admins"`
	Admins               []string          `json:"admins"`
	Death                bool              `json:"death"`
	Join                 bool              `json:"join"`
	Leave                bool              `json:"leave"`
	PublicObjectives     []string          `json:"public_objectives"`
	Model                string            `json:"model"`
	Prompt               string            `json:"prompt"`
	BotPlayer            string            `json:"bot_player"`
	Caps                 map[string]bool   `json:"caps,omitempty"`
	Triggers             []yyQQTrigger     `json:"triggers,omitempty"`
	MCPURL               string            `json:"mcp_url"`
	MCPToken             string            `json:"mcp_token,omitempty"`
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
			writeJSON(w, map[string]any{"ok": true, "code": code, "expires_seconds": 600, "instruction": "在目标群 @机器人发送 .认领 " + code})
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
		writeJSON(w, map[string]any{"ok": true, "bindings": yyQQHelper.Bindings, "members": members, "stats": yyQQHelper.Stats, "help": yyQQHelp(2)})
	})
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
func yyQQHelp(level int) string {
	level = max(0, min(level, 2))
	text := "✦ 游园 · 群服助手\n━━━━━━━━━━━━━━\n你的权限：" + []string{"成员", "管理员", "超级管理员"}[level]
	text += "\n\n💬 聊天与账号 · 所有成员\n.通 消息 → 转发到游戏\n/绑定 游戏ID → 游戏内验证后绑定\n.我的绑定 → 查看自己的绑定\n/解绑 → 解除自己的绑定\n/说 消息 → 使用绑定身份发言\n.身份 → 查看昵称与权限"
	text += "\n\n📋 查询与记录 · 所有成员\nlist → 当前在线玩家\n.技能 → 可用游戏技能列表\n.状态 → 游戏连接与个人绑定状态\n.查询 游戏ID → 玩家头像与活动资料卡\n.日志 [条数] [页码] → 最近聊天，默认60条\n例如：.日志 20\n.群日志 [条数] → 机器人收到的近期群消息"
	text += "\n\n✨ AI 助手\n.AI 问题 → 聊天、在线玩家与公开分数\n成员不能查询坐标或修改服务器\n管理员可使用已配置的游戏工具与MCP"
	text += "\n\n🤖 机器人能力\n.点赞自己 / .点赞 QQ号 [次数1-10] → 点赞\n.动态 内容 → 发表QQ空间动态（≤500字）\n.删动态 tid → 删除动态\n.改头像 图片URL / .改签名 内容 / .改昵称 名字 → 形象（超级管理员）\n.能力 → 查看能力与限额"
	text += "\n\n🛠 仅管理员\n/命令 指令 → 默认魔法通道\n支持多行TXT/function；不会无限重复执行\n.死亡消息 开启/关闭\n.进服消息 开启/关闭\n.退服消息 开启/关闭"
	text += "\n\n🔑 仅超级管理员\n/授权 昵称 → 添加管理员\n/撤权 昵称 → 撤销管理员\n.认领 认领码 → APK所有者指定超级管理员"
	text += "\n━━━━━━━━━━━━━━\n群内请 @机器人 后发送指令\n游戏内使用 .通 消息 转发到群\n帮助 / .帮助 → 再次打开此菜单"
	return text
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
	reply := func(s string) []string { return []string{s} }
	arg := func(prefix string) string { return strings.TrimSpace(strings.TrimPrefix(text, prefix)) }
	switch {
	case text == "帮助" || text == ".帮助" || text == "/帮助":
		return reply(yyQQHelp(level))
	case text == ".身份":
		return reply("昵称：" + yyQQDisplayName(cf, user, name) + fmt.Sprintf("\n权限：%s", []string{"成员", "管理员", "超级管理员"}[level]))
	case text == ".状态" || text == ".我的绑定":
		yyQQHelper.Lock()
		player := yyQQHelper.Bindings[user]
		yyQQHelper.Unlock()
		if player == "" {
			player = "未绑定（使用 /绑定 游戏ID）"
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
			ai = "已配置（.AI 问题）"
		}
		identity := ""
		if cf.Assistant.BotPlayer != "" {
			identity = "\n机器人身份：主机器人 " + cf.Assistant.BotPlayer + " 的QQ助手"
		}
		return reply("✦ 游园 · 当前状态\n游戏：" + state + identity + "\n我的绑定：" + player + "\n我的权限：" + []string{"成员", "管理员", "超级管理员"}[level] + "\n群AI：" + ai)
	case text == "/解绑":
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
	case yyQQHasPrefix(text, ".认领"):
		yyQQHelper.Lock()
		valid := yyQQHelper.Claim != "" && arg(".认领") == yyQQHelper.Claim && time.Now().Before(yyQQHelper.ClaimUntil)
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
	case text == "list" || strings.HasPrefix(text, "list ") || yyQQHasPrefix(text, ".玩家"):
		page := 1
		value := strings.TrimSpace(strings.TrimPrefix(text, "list"))
		if yyQQHasPrefix(text, ".玩家") {
			value = arg(".玩家")
		}
		if value != "" {
			v, e := strconv.Atoi(value)
			if e != nil || v < 1 {
				return reply("格式：list [页码] 或 .玩家 [页码]")
			}
			page = v
		}
		// 与 AI、面板走同一条注册表实现（doc §5.5）：确定性执行，不再发头像卡片图。
		out, e := yyGameSkillRun("online_list", map[string]string{"page": strconv.Itoa(page)})
		if e != nil {
			return reply(e.Error())
		}
		return reply(out)
	case yyQQHasPrefix(text, ".群日志"):
		n := 20
		if value := arg(".群日志"); value != "" {
			v, e := strconv.Atoi(value)
			if e != nil || v < 1 || v > 80 {
				return reply("格式：.群日志 [1..80]，默认20；只含机器人实际收到的群消息")
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
	case yyQQHasPrefix(text, ".日志"):
		n, page := 60, 1
		f := strings.Fields(arg(".日志"))
		if len(f) > 2 {
			return reply("格式：.日志 [条数1..300] [页码]")
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
			out[len(out)-1] += fmt.Sprintf("\n下一页：.日志 %d %d", n, page+1)
		}
		return out
	case yyQQHasPrefix(text, ".查询"):
		p := arg(".查询")
		if p == "" {
			return reply("格式：.查询 游戏ID")
		}
		image, e := yyQQPlayerCard(p)
		if e != nil {
			return reply(e.Error())
		}
		if e = yyQQSendImage(cf, image, id); e != nil {
			return reply(yyQQPlayerSummary(p) + "\n图片发送失败：" + e.Error())
		}
		return nil
	case yyQQHasPrefix(text, "/绑定"):
		player := arg("/绑定")
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
		if e = yyQQTellraw(toPlayerSelector(player), "QQ绑定请求：请在游戏聊天输入 .绑定 "+code+"（4分钟内）。"); e != nil {
			return reply("游戏验证提示发送失败：" + e.Error())
		}
		return reply("请该游戏玩家查看游戏内验证码，并在游戏输入 .绑定 验证码。")
	case yyQQHasPrefix(text, ".通"):
		msg := arg(".通")
		if msg == "" {
			return reply("格式：.通 具体消息")
		}
		if e := yyQQRelayGame("[群 " + yyQQGroupLabel(cf, yyQQGroupString(cf.GroupID)) + "] " + yyQQDisplayName(cf, user, name) + "：" + msg); e != nil {
			return reply(e.Error())
		}
		if cf.Adapter == "qqofficial" && cf.Official.Acknowledge != nil && !*cf.Official.Acknowledge {
			return nil
		}
		return reply("已转发到游戏。")
	case yyQQHasPrefix(text, "/说"):
		yyQQHelper.Lock()
		player := yyQQHelper.Bindings[user]
		yyQQHelper.Unlock()
		if player == "" {
			return reply("请先 /绑定 游戏ID，并在游戏完成验证。")
		}
		msg := arg("/说")
		if msg == "" {
			return reply("格式：/说 消息")
		}
		if e := yyQQRelayGame("[群 " + yyQQGroupLabel(cf, yyQQGroupString(cf.GroupID)) + "] " + player + "：" + msg); e != nil {
			return reply(e.Error())
		}
		if cf.Adapter == "qqofficial" && cf.Official.Acknowledge != nil && !*cf.Official.Acknowledge {
			return nil
		}
		return reply("已以绑定身份转发。")
	case yyQQHasPrefix(text, "/授权") || yyQQHasPrefix(text, "/撤权"):
		if level < 2 {
			return reply("只有超级管理员可以授权或撤权。")
		}
		grant := yyQQHasPrefix(text, "/授权")
		prefix := "/撤权"
		if grant {
			prefix = "/授权"
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
	case yyQQHasPrefix(text, "/命令"):
		if level == 0 {
			return reply("只有已授权管理员可以执行游戏命令。常用操作请用 .技能 或直接说需求。")
		}
		out := yyQQExecuteText(cf, arg("/命令"))
		// 逃生舱出错时引导到更安全的预定义技能路径（doc §5.5）。
		for _, mark := range []string{"语法错误", "未执行", "上限", "失败", "拒绝", "无此", "未知", "超时", "不合法"} {
			if strings.Contains(out, mark) {
				out += "\n常用操作请用 .技能 或直接说需求。"
				break
			}
		}
		return reply(out)
	case yyQQHasPrefix(text, ".AI"):
		message := arg(".AI")
		if message == "" {
			return reply("格式：.AI 消息")
		}
		answer, e := yyQQAskAIWithReply(cf, user, message, id)
		if e != nil {
			return reply("AI处理失败：" + e.Error())
		}
		return reply(answer)
	case text == ".能力":
		return yyQQCmdActions(cf, level)
	case text == ".技能":
		return yyQQCmdSkills(cf, level)
	case yyQQHasPrefix(text, ".点赞"):
		return yyQQCmdLike(cf, user, name, arg(".点赞"), level)
	case yyQQHasPrefix(text, ".动态"):
		return yyQQCmdQzone(cf, user, name, arg(".动态"), level)
	case yyQQHasPrefix(text, ".删动态"):
		return yyQQCmdDeleteQzone(cf, user, name, arg(".删动态"), level)
	case yyQQHasPrefix(text, ".改头像"):
		return yyQQCmdAvatar(cf, user, name, arg(".改头像"), level)
	case yyQQHasPrefix(text, ".改签名"):
		return yyQQCmdLongNick(cf, user, name, arg(".改签名"), level)
	case yyQQHasPrefix(text, ".改昵称"):
		return yyQQCmdNick(cf, user, name, arg(".改昵称"), level)
	case yyQQHasPrefix(text, ".表情包"):
		return yyQQCmdFaces(cf, level)
	case yyQQHasPrefix(text, ".表情"):
		return yyQQCmdEmoji(cf, user, name, arg(".表情"), id, level)
	case yyQQHasPrefix(text, ".踢"):
		return yyQQCmdKick(cf, user, name, arg(".踢"), level)
	case strings.HasPrefix(text, "/"):
		return reply("未知助手指令，请发送 帮助；常用操作请用 .技能 或直接说需求，管理员逃生舱用 /命令。")
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
func yyQQConfirmBinding(player, text string) bool {
	if !yyQQHasPrefix(text, ".绑定") {
		return false
	}
	code := strings.TrimSpace(strings.TrimPrefix(text, ".绑定"))
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
