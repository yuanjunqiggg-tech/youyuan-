package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// 群 AI 工具清单（面板、实际下发、自检共用同一份注册表）。
//
// 背景：工具散在三个地方——youyuan_qq_ai.go 的 yyQQAITools（查询/游戏/MCP）、
// youyuan_qq_social_tools.go 的 yyQQSocialTools（社交）、youyuan_seed_api.go 的
// yySeedAITools（种子）。以前面板只能把工具名原样打印成一行英文，用户既不知道
// 有多少个、也不知道哪些真的给了 AI，更没法关掉不想给的那个。
//
// 这里做三件事：
//  1. 用一份注册表把「系统里到底有哪些工具」写清楚（id + 中文名 + 给用户看的一句话）；
//  2. 面板按组展示并逐项开关，保存到 assistant.tools[id]；
//  3. yyQQAITools 按这份开关过滤，关掉就真的不下发给模型。
//
// 权限（成员/管理员/超管）与能力闸门（caps）仍是各自独立的一层，这里只做
// 「用户想不想给」这一层，不替代也不绕过前两层。

type yyQQToolDef struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Group   string `json:"group"`
	Desc    string `json:"desc"`
	Default bool   `json:"default"`
	Danger  bool   `json:"danger"`
	MinRole int    `json:"min_role"`
}

// yyQQToolGroups 分组顺序：面板按这个顺序渲染，空组不显示。
var yyQQToolGroups = []struct {
	ID    string
	Label string
	Note  string
}{
	{"game", "游戏操作", "会改动服务器里的东西"},
	{"query", "服务器查询", "只读，不改任何状态"},
	{"manage", "群管理", "对群成员生效"},
	{"content", "内容生成", "往群里或空间发东西"},
	{"other", "其它", ""},
}

// yyQQToolDefs 工具注册表。顺序即面板顺序。
//
// Default 说明：
//   - 查询类、社交类、游戏技能默认开（用户明确要求「AI 要能自己做事」）；
//   - 危险操作默认关：ban / kick（踢人、禁言）、game_call（绕过技能表执行
//     任意指令）、mcp_call（调外部工具，可能改配置）。用户原话：「危险操作
//     （踢人、封禁、执行指令、改配置）默认关；查询类默认开」——默认关不是
//     删掉，在面板「AI 工具」里点一下即可打开。
var yyQQToolDefs = []yyQQToolDef{
	// ---- 游戏操作 ----
	{ID: "game_skill", Label: "执行游戏技能", Group: "game", Default: true, MinRole: 0,
		Desc: "按预定义技能表操作服务器：广播、传送、改天气、改时间、踢玩家等，只能用列表里有的技能"},
	{ID: "game_call", Label: "直接调用游戏接口", Group: "game", Default: false, Danger: true, MinRole: 1,
		Desc: "不经技能表，直接调游戏接口执行任意指令。能改的东西最多，默认关闭"},
	{ID: "seed_query", Label: "种子综合查询", Group: "game", Default: true, MinRole: 1,
		Desc: "给一个种子和坐标，一次查出附近要塞、史莱姆区块、各类结构"},
	{ID: "seed_biome", Label: "生物群系", Group: "game", Default: true, MinRole: 1,
		Desc: "查某个坐标的生物群系，也可以批量查一片区域"},
	{ID: "seed_slime", Label: "史莱姆区块", Group: "game", Default: true, MinRole: 1,
		Desc: "查一片区域里哪些区块会刷史莱姆（精确）"},
	{ID: "seed_stronghold", Label: "要塞位置", Group: "game", Default: true, MinRole: 1,
		Desc: "查末地要塞坐标，128 个环位精确复刻，可直接照着传送"},
	{ID: "seed_structure", Label: "结构位置", Group: "game", Default: true, MinRole: 1,
		Desc: "查村庄、府邸、神殿、遗迹等 28 种结构坐标；多数精确，少数需进游戏确认"},
	{ID: "seed_heat", Label: "结构热点图", Group: "game", Default: true, MinRole: 1,
		Desc: "把一片区域切成格子，看哪里的结构最密集"},
	{ID: "seed_coordinates", Label: "坐标分享码", Group: "game", Default: true, MinRole: 1,
		Desc: "把种子和坐标打包成分享码，或从分享码还原出坐标"},
	{ID: "seed_manifest", Label: "种子查询说明", Group: "game", Default: true, MinRole: 1,
		Desc: "看种子查询器支持哪些结构、哪些是精确哪些是估算"},

	// ---- 服务器查询 ----
	{ID: "online_players", Label: "在线玩家", Group: "query", Default: true, MinRole: 0,
		Desc: "查当前在线的玩家名单，只给游戏 ID，不给坐标"},
	{ID: "public_scores", Label: "公开分数", Group: "query", Default: true, MinRole: 0,
		Desc: "查你设成公开的计分项和玩家分数"},
	{ID: "scoreboard_count", Label: "记分板数量", Group: "query", Default: true, MinRole: 0,
		Desc: "只查记分板有几个，不暴露记分项名字和内容"},

	// ---- 群管理 ----
	// 危险操作默认关（用户：「踢人、封禁、执行指令、改配置默认关」）。
	// 限流与留痕照旧，用户在「AI 工具」里打开后立即可用。
	{ID: "ban", Label: "禁言", Group: "manage", Default: false, Danger: true, MinRole: 2,
		Desc: "禁言群成员，要填时长和理由，每次都会留痕"},
	{ID: "kick", Label: "移出群", Group: "manage", Default: false, Danger: true, MinRole: 1,
		Desc: "把群成员移出群（不加黑名单），只在严重违规时用"},

	// ---- 内容生成 ----
	{ID: "send_sticker", Label: "发表情包", Group: "content", Default: true, MinRole: 0,
		Desc: "从机器人的收藏表情里挑一张发到群里"},
	{ID: "list_stickers", Label: "收藏表情列表", Group: "content", Default: true, MinRole: 0,
		Desc: "列出机器人 QQ 收藏了哪些表情包"},
	{ID: "send_voice", Label: "发语音", Group: "content", Default: true, MinRole: 1,
		Desc: "把一段文字转成语音发到群里（需要先配语音模型）"},
	{ID: "post_qzone", Label: "发空间动态", Group: "content", Default: true, MinRole: 1,
		Desc: "以机器人的身份发一条 QQ 空间动态"},
	{ID: "image_generate", Label: "画图", Group: "content", Default: true, MinRole: 0,
		Desc: "按描述生成一张图片发到群里（需要先配绘图模型）"},
	{ID: "view_image", Label: "看图", Group: "content", Default: true, MinRole: 0,
		Desc: "看清群里一张图片的内容，需要模型支持视觉"},

	// ---- 其它 ----
	{ID: "group_summary", Label: "群聊记录", Group: "other", Default: true, MinRole: 0,
		Desc: "读最近的群消息，用来回答「刚才聊了啥」"},
	{ID: "poke", Label: "拍一拍", Group: "other", Default: true, MinRole: 0,
		Desc: "拍一拍群里的某个人"},
	{ID: "web_fetch", Label: "读网页", Group: "other", Default: true, MinRole: 1,
		Desc: "读一个公开网页的正文，只读、不能访问内网"},
	{ID: "load_skill", Label: "读开发说明", Group: "other", Default: true, MinRole: 1,
		Desc: "按需读本机的开发说明文档，分页读"},
	{ID: "mcp_list", Label: "MCP 目录", Group: "other", Default: true, MinRole: 1,
		Desc: "列出你配置的外部 MCP 工具"},
	{ID: "mcp_describe", Label: "MCP 参数", Group: "other", Default: true, MinRole: 1,
		Desc: "看某个外部 MCP 工具要传什么参数"},
	{ID: "mcp_call", Label: "调用 MCP 工具", Group: "other", Default: false, Danger: true, MinRole: 1,
		Desc: "真正执行外部 MCP 工具，可能改服务器配置。默认关闭"},
}

func yyQQToolDefOf(id string) (yyQQToolDef, bool) {
	for _, d := range yyQQToolDefs {
		if d.ID == id {
			return d, true
		}
	}
	return yyQQToolDef{}, false
}

// yyQQToolEnabled 读某项工具的开关。没设置过 = 注册表默认值；
// 注册表里没有的 id 一律当开（避免新增工具被旧配置误杀）。
func yyQQToolEnabled(a yyQQAssistantConfig, id string) bool {
	if a.Tools != nil {
		if v, ok := a.Tools[id]; ok {
			return v
		}
	}
	if d, ok := yyQQToolDefOf(id); ok {
		return d.Default
	}
	return true
}

// yyQQToolView 面板视图：注册表 + 生效开关 + 当前等级是否会下发。
// available=false 表示这台机器/这个角色根本拿不到（不是用户关的），面板要区分。
func yyQQToolView(a yyQQAssistantConfig, level int) []map[string]any {
	out := make([]map[string]any, 0, len(yyQQToolDefs))
	for _, d := range yyQQToolDefs {
		out = append(out, map[string]any{
			"id": d.ID, "label": d.Label, "group": d.Group, "desc": d.Desc,
			"default": d.Default, "danger": d.Danger, "min_role": d.MinRole,
			"enabled":   yyQQToolEnabled(a, d.ID),
			"available": level >= d.MinRole,
		})
	}
	return out
}

// yyQQToolGroupsView 分组视图，面板直接按它渲染，不用在前端再维护一份映射。
func yyQQToolGroupsView(a yyQQAssistantConfig, level int) []map[string]any {
	all := yyQQToolView(a, level)
	out := make([]map[string]any, 0, len(yyQQToolGroups))
	for _, g := range yyQQToolGroups {
		rows := make([]map[string]any, 0, 8)
		on := 0
		for _, t := range all {
			if t["group"] == g.ID {
				rows = append(rows, t)
				if t["enabled"] == true {
					on++
				}
			}
		}
		if len(rows) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"id": g.ID, "label": g.Label, "note": g.Note,
			"tools": rows, "count": len(rows), "enabled_count": on,
		})
	}
	return out
}

// yyQQToolRoutes GET 只读视图 / POST 保存开关。
// POST 只接受注册表里有的 id；未提交的项保持原值（深合并语义），
// 提交 null 或字符串以外的值一律拒绝，避免把配置写坏。
func yyQQToolRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/ai/tools", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			cf := yyQQSnapshot()
			// level 取 2：面板要能看到「全部工具」，至于某个角色实际能不能用，
			// 由每项的 available 字段表达。
			writeJSON(w, map[string]any{
				"ok":     true,
				"groups": yyQQToolGroupsView(cf.Assistant, 2),
				"total":  len(yyQQToolDefs),
			})
			return
		}
		if r.Method != "POST" {
			http.Error(w, "GET/POST required", 405)
			return
		}
		var p struct {
			Tools map[string]*bool `json:"tools"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&p); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		for id := range p.Tools {
			if _, ok := yyQQToolDefOf(id); !ok {
				writeJSON(w, map[string]any{"ok": false, "error": "未知工具：" + id})
				return
			}
		}
		e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
			if a.Tools == nil {
				a.Tools = map[string]bool{}
			}
			for id, v := range p.Tools {
				if v == nil {
					delete(a.Tools, id) // null = 恢复默认
					continue
				}
				a.Tools[id] = *v
			}
		})
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		cf := yyQQSnapshot()
		writeJSON(w, map[string]any{
			"ok":     true,
			"groups": yyQQToolGroupsView(cf.Assistant, 2),
			"total":  len(yyQQToolDefs),
		})
	})
}

// yyQQToolNames 当前配置下真正会下发给模型的工具名（按注册表顺序）。
// 自检面板和「AI 工具」分组都用它，保证「看到的」= 「实际下发的」。
func yyQQToolNames(admin bool, level int) []string {
	out := make([]string, 0, len(yyQQToolDefs))
	for _, t := range yyQQAITools(admin, level) {
		out = append(out, t.Name)
	}
	return out
}

// yyQQToolSummary 一句话说明当前给了 AI 多少个工具，面板摘要用。
func yyQQToolSummary(a yyQQAssistantConfig) string {
	on := 0
	for _, d := range yyQQToolDefs {
		if yyQQToolEnabled(a, d.ID) {
			on++
		}
	}
	return strconv.Itoa(on) + " / " + strconv.Itoa(len(yyQQToolDefs)) + " 项开启"
}
