package main

// 帮助菜单（注册表驱动）。
//
// 原来的 yyQQHelp 是一大段拼死的字符串：改一个指令名要改两处，面板里关掉的
// 功能帮助里照样显示，普通成员也看得到「🔑 仅超级管理员」那一组。
// 用户原话：「这种东西不要默认就有 也要给他们一个配置选项」
// 「触发词规则当中好像并没有 选择是管理员还是普通成员的分层使用」。
//
// 现在帮助 = 注册表的投影：
//  1. 条目 = {分组, 图标, 命令, 说明, 最低权限, 开关key}；
//  2. 只渲染「已启用」的条目（面板关掉的能力/技能不出现在菜单里）；
//  3. 按当前用户实际权限过滤（普通成员看不到 .改头像、看不到仅超管那一组）；
//  4. 命令串一律取生效别名（yyQQCommandText），玩家改别名后帮助同步变。
//
// 数据源：yyQQCapabilities（能力）、yyQQSkillDefs（AI 技能）、
// yyGameSkillAll()（游戏技能注册表）、yyQQHelpCmdGroups（固定指令入口）。

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// yyQQHelpEntry 帮助菜单的一个条目。前端可以照着这份结构直接渲染面板预览。
type yyQQHelpEntry struct {
	Group       string `json:"group"`       // 分组 key：chat/query/ai/cap/game/skill/admin/root
	GroupTitle  string `json:"group_title"` // 分组标题（含角色后缀），如「🤖 机器人能力 · 仅管理员」
	Icon        string `json:"icon"`        // 图标（emoji）
	Command     string `json:"command"`     // 生效命令串（别名优先）
	Desc        string `json:"desc"`        // 一句话说明
	MinRole     int    `json:"min_role"`    // 0 成员 / 1 管理员 / 2 超管
	MinRoleText string `json:"min_role_text"`
	// ToggleKey 是面板里控制这一条的开关：
	//   能力     = 能力 kind（POST /api/qq/caps 的 caps[key]）
	//   AI 技能  = 技能 id（POST /api/qq/assistant 的 skills[id].enabled）
	//   游戏技能 = "gs:<name>"（POST /api/qq/skills 的 enabled）
	// 空 = 代码里写死的固定指令，没有开关（见报告「写死项」）。
	ToggleKey string `json:"toggle_key,omitempty"`
	Source    string `json:"source"`  // command | capability | skill | game_skill
	Enabled   bool   `json:"enabled"` // 当前开关状态（不受权限影响）

	seq int
}

// yyQQHelpCmdDef 固定指令条目（这些不是能力/技能，没有独立执行闸门）。
type yyQQHelpCmdDef struct {
	Cmd  string // yyQQCommandDefaults 的 key；为空时用 Text 原样显示
	Text string // Cmd 为空时的固定指令串
	Desc string
	Icon string
}

// yyQQHelpCmdGroups 固定指令按「分组 + 最低权限」划分。
var yyQQHelpCmdGroups = []struct {
	Group string
	Role  int
	Title string
	Defs  []yyQQHelpCmdDef
}{
	{"chat", 0, "💬 聊天与账号", []yyQQHelpCmdDef{
		{Cmd: "relay", Icon: "💬", Desc: "消息 → 转发到游戏"},
		{Cmd: "bind", Icon: "🔗", Desc: "游戏ID → 游戏内验证后绑定"},
		{Text: ".我的绑定", Icon: "🔍", Desc: "查看自己的绑定"},
		{Cmd: "unbind", Icon: "✂️", Desc: "解除自己的绑定"},
		{Cmd: "speak", Icon: "🗣", Desc: "消息 → 使用绑定身份发言"},
		{Cmd: "identity", Icon: "🪪", Desc: "查看昵称与权限"},
	}},
	{"query", 0, "📋 查询与记录", []yyQQHelpCmdDef{
		{Text: "list", Icon: "👥", Desc: "当前在线玩家"},
		{Cmd: "skills", Icon: "🎮", Desc: "可用游戏技能列表"},
		{Cmd: "status", Icon: "📶", Desc: "游戏连接与个人绑定状态"},
		{Cmd: "query", Icon: "🖼", Desc: "游戏ID → 玩家头像与活动资料卡"},
		{Cmd: "log", Icon: "📜", Desc: "[条数] [页码] → 最近聊天，默认60条"},
		{Cmd: "group_log", Icon: "🧾", Desc: "[条数] → 机器人收到的近期群消息"},
	}},
	{"ai", 0, "✨ AI 助手", []yyQQHelpCmdDef{
		{Cmd: "ai", Icon: "✨", Desc: "问题 → 聊天、在线玩家与公开分数"},
	}},
	{"admin", 1, "🛠 管理操作", []yyQQHelpCmdDef{
		{Cmd: "command", Icon: "🛠", Desc: "指令 → 默认魔法通道（支持多行TXT/function，不会无限重复执行）"},
		{Cmd: "replay", Icon: "↻", Desc: "重放上次成功的指令（危险指令仍需授权）"},
		{Text: ".死亡消息 开启/关闭", Icon: "💀", Desc: "死亡播报到群"},
		{Text: ".进服消息 开启/关闭", Icon: "📥", Desc: "进服播报到群"},
		{Text: ".退服消息 开启/关闭", Icon: "📤", Desc: "退服播报到群"},
	}},
	{"root", 2, "🔑 超级管理员", []yyQQHelpCmdDef{
		{Text: "同意 编号 / 拒绝 编号", Icon: "✅", Desc: "批准群内危险指令（op/ban/fill 等）"},
		{Cmd: "authorize", Icon: "➕", Desc: "昵称 → 添加管理员"},
		{Cmd: "revoke", Icon: "➖", Desc: "昵称 → 撤销管理员"},
		{Cmd: "claim", Icon: "🔑", Desc: "认领码 → APK所有者指定超级管理员"},
	}},
}

// yyQQHelpGroupOrder 分组顺序（数字小的在前）。
var yyQQHelpGroupOrder = map[string]int{
	"chat": 0, "query": 1, "ai": 2, "cap": 3, "game": 4, "skill": 5, "admin": 6, "root": 7,
}

// yyQQHelpGroupBase 分组基础标题（不含角色后缀）。
var yyQQHelpGroupBase = map[string]string{
	"chat": "💬 聊天与账号", "query": "📋 查询与记录", "ai": "✨ AI 助手",
	"cap": "🤖 机器人能力", "game": "🎮 游戏技能", "skill": "🧩 AI 技能（提示词内）",
	"admin": "🛠 管理操作", "root": "🔑 超级管理员",
}

// yyQQHelpGroupTitle 分组标题 + 角色后缀。同组内不同权限会拆成不同标题，
// 这样「管理操作 · 仅管理员」不会混进「聊天与账号 · 所有成员」里。
func yyQQHelpGroupTitle(group string, role int) string {
	base := yyQQHelpGroupBase[group]
	if base == "" {
		base = group
	}
	suffix := []string{"所有成员", "仅管理员", "仅超级管理员"}[max(0, min(role, 2))]
	return base + " · " + suffix
}

// yyQQHelpCapCmdID 能力 kind → 指令 ID。多数同名，少数在指令表里叫别的名字。
var yyQQHelpCapCmdID = map[string]string{
	"face": "faces", "emoji_like": "emoji", "game_skill": "skills",
}

// yyQQHelpCapCommand 能力的命令串：首词换成玩家生效别名，参数原样保留。
// 能力表的 Command 是「默认文案」，玩家改过 .改头像 → .换头像 后帮助必须跟着变
// （原实现里帮助只认 cmd("avatar")，能力那一段永远显示旧名）。
func yyQQHelpCapCommand(cf yyQQConfig, kind, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	cmdID := kind
	if v, ok := yyQQHelpCapCmdID[kind]; ok {
		cmdID = v
	}
	if _, ok := yyQQCommandDefaults[cmdID]; !ok {
		return raw
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return raw
	}
	parts[0] = yyQQCommandText(cf, cmdID)
	return strings.Join(parts, " ")
}

// yyQQHelpRegistry 注册表全量条目（不做权限/开关过滤）。
func yyQQHelpRegistry(cf yyQQConfig) []yyQQHelpEntry {
	out := make([]yyQQHelpEntry, 0, 96)
	for _, g := range yyQQHelpCmdGroups {
		for _, d := range g.Defs {
			cmd := strings.TrimSpace(d.Text)
			if d.Cmd != "" {
				cmd = yyQQCommandText(cf, d.Cmd)
			}
			out = append(out, yyQQHelpEntry{
				Group: g.Group, Icon: d.Icon, Command: cmd, Desc: d.Desc,
				MinRole: g.Role, Source: "command", Enabled: true,
			})
		}
	}
	for _, c := range yyQQCapabilities {
		eff := yyQQCapEffective(c, cf.Assistant)
		out = append(out, yyQQHelpEntry{
			Group: "cap", Icon: eff.Icon, Command: yyQQHelpCapCommand(cf, eff.Kind, eff.Command),
			Desc: eff.Desc, MinRole: eff.MinLevel, ToggleKey: eff.Kind,
			Source: "capability", Enabled: yyQQCapEnabled(cf.Assistant, eff.Kind),
		})
	}
	for _, def := range yyQQSkillDefs {
		word := yyQQSkillWord(cf, def.ID)
		out = append(out, yyQQHelpEntry{
			Group: "skill", Icon: "🧩", Command: "技能[" + word + "]", Desc: def.Desc,
			MinRole: def.MinLevel, ToggleKey: def.ID, Source: "skill",
			Enabled: yyQQSkillEnabled(cf, def.ID) && yyQQSkillCapOK(cf, def.ID) && yyQQSkillToolOK(cf, def.ID),
		})
	}
	for _, gs := range yyGameSkillAll() {
		desc := gs.Label
		if strings.TrimSpace(gs.Desc) != "" {
			desc += "（" + gs.Desc + "）"
		}
		out = append(out, yyQQHelpEntry{
			Group: "game", Icon: "🎮", Command: gs.Name, Desc: desc,
			MinRole: gs.MinRole, ToggleKey: "gs:" + gs.Name, Source: "game_skill",
			Enabled: gs.Enabled,
		})
	}
	for i := range out {
		out[i].seq = i
		out[i].MinRoleText = yyQQCapLevelText(out[i].MinRole)
		out[i].GroupTitle = yyQQHelpGroupTitle(out[i].Group, out[i].MinRole)
	}
	return yyQQHelpSorted(out)
}

// yyQQHelpSorted 按（分组顺序, 权限, 注册表顺序）稳定排序。
func yyQQHelpSorted(entries []yyQQHelpEntry) []yyQQHelpEntry {
	out := append([]yyQQHelpEntry(nil), entries...)
	sort.SliceStable(out, func(i, j int) bool {
		gi, gj := yyQQHelpGroupOrder[out[i].Group], yyQQHelpGroupOrder[out[j].Group]
		if gi != gj {
			return gi < gj
		}
		if out[i].MinRole != out[j].MinRole {
			return out[i].MinRole < out[j].MinRole
		}
		return out[i].seq < out[j].seq
	})
	return out
}

// yyQQHelpEntries 实际渲染用的条目：只留「开关打开」且「权限够」的。
// AI 技能（技能[xxx]）是提示词里的召唤词、群友敲不出来，正文里不列，
// 但仍通过 /api/qq/help?raw=1 暴露给面板。
func yyQQHelpEntries(cf yyQQConfig, level int) []yyQQHelpEntry {
	level = max(0, min(level, 2))
	out := make([]yyQQHelpEntry, 0, 64)
	for _, e := range yyQQHelpRegistry(cf) {
		if e.Source == "skill" {
			continue
		}
		if !e.Enabled || e.MinRole > level {
			continue
		}
		out = append(out, e)
	}
	return out
}

// yyQQHelpDisabledCount 被玩家关掉的条目数（能力 + 技能 + 游戏技能），
// 帮助末尾提示一句「还有 N 项关着」，免得用户以为功能丢了。
func yyQQHelpDisabledCount(cf yyQQConfig) int {
	n := 0
	for _, c := range yyQQCapabilities {
		if !yyQQCapEnabled(cf.Assistant, c.Kind) {
			n++
		}
	}
	for _, def := range yyQQSkillDefs {
		if !yyQQSkillEnabled(cf, def.ID) || !yyQQSkillCapOK(cf, def.ID) || !yyQQSkillToolOK(cf, def.ID) {
			n++
		}
	}
	for _, gs := range yyGameSkillAll() {
		if !gs.Enabled {
			n++
		}
	}
	return n
}

// yyQQHelpLine 一条条目的正文。
func (e yyQQHelpEntry) yyQQHelpLine() string {
	cmd := strings.TrimSpace(e.Command)
	switch {
	case cmd == "":
		return e.Desc
	case e.Source == "capability" || e.Source == "game_skill":
		return cmd + " → " + e.Desc
	default:
		return cmd + " " + e.Desc
	}
}

// yyQQHelpText 渲染群内帮助菜单（按 level 过滤已启用条目）。
func yyQQHelpText(cf yyQQConfig, level int) string {
	level = max(0, min(level, 2))
	var b strings.Builder
	b.WriteString("✦ 游园 · 群服助手\n━━━━━━━━━━━━━━\n你的权限：" + yyQQCapLevelText(level))
	title := ""
	for _, e := range yyQQHelpEntries(cf, level) {
		if e.GroupTitle != title {
			title = e.GroupTitle
			b.WriteString("\n\n" + title)
		}
		b.WriteString("\n" + e.yyQQHelpLine())
	}
	if n := yyQQHelpDisabledCount(cf); n > 0 {
		b.WriteString("\n\n（另有 " + strconv.Itoa(n) + " 项功能在面板里关着，打开后会出现在这里）")
	}
	help := yyQQCommandText(cf, "help")
	relay := yyQQCommandText(cf, "relay")
	b.WriteString("\n━━━━━━━━━━━━━━\n群内请 @机器人 后发送指令\n游戏内使用 " + relay + " 消息 转发到群\n" + help + " / ." + help + " → 再次打开此菜单")
	return b.String()
}

// yyQQHelpRegister 只读接口：GET /api/qq/help?level=0|1|2&raw=1
// level 缺省 2（面板预览默认看全量）；raw=1 时返回未过滤条目（含未启用/更高权限）。
// 由 yyQQCapRoutes 调用注册（与能力注册表同一块，youyuan_qq.go 不用改）。
func yyQQHelpRegister(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/help", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		cf := yyQQSnapshot()
		level := 2
		if v := strings.TrimSpace(r.URL.Query().Get("level")); v != "" {
			if n, e := strconv.Atoi(v); e == nil {
				level = n
			}
		}
		level = max(0, min(level, 2))
		entries := yyQQHelpEntries(cf, level)
		if r.URL.Query().Get("raw") == "1" {
			entries = yyQQHelpRegistry(cf)
		}
		groups := make([]map[string]any, 0, len(yyQQHelpGroupOrder))
		for _, key := range []string{"chat", "query", "ai", "cap", "game", "skill", "admin", "root"} {
			groups = append(groups, map[string]any{
				"key": key, "title": yyQQHelpGroupBase[key], "order": yyQQHelpGroupOrder[key],
			})
		}
		writeJSON(w, map[string]any{
			"ok": true, "level": level, "level_text": yyQQCapLevelText(level),
			"entries": entries, "text": yyQQHelpText(cf, level),
			"groups": groups, "roles": []string{"成员", "管理员", "超级管理员"},
			"hidden_disabled": yyQQHelpDisabledCount(cf),
		})
	})
}
