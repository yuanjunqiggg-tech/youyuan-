package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// QQ 机器人「技能系统」：让 AI 能自主使用机器人的社交/管理能力。
//
// 用户原话精髓：
//   「提示词里面告诉他，可以使用技能[meme] 他输出使用 能看到所有收藏的表情包
//     然后自己可以选择表情包发出去 他想点几个都行」
//   「玩家想改特定的功能词也可以设置 [meme] 他可以把表情包换成中文的表情包」
//   「他恼羞成怒了……他是管理员 他可以弄的东西很多……就像一个真人一样」
//
// 设计要点：
//  1. 技能 = 注册表（本文件）+ 玩家覆盖（assistant.skills[id].word/enabled）+
//     能力闸门（youyuan_qq_caps.go 的 caps）。三层叠加，缺省都不写死。
//  2. AI 在回复里输出「技能[功能词] 参数」，由 yyQQRunSkillDirectives 解析并
//     代为实现；执行结果以系统 note 的形式回给 AI（玩家看不到 note）。
//  3. 危险技能（禁言/踢人）以「机器人自己是群管理」的身份执行（level=2），
//     与「用户指挥 AI 调工具」的路径区分：工具路径按提问者权限收窄
//     （见 youyuan_qq_social_tools.go 的 ban/kick 工具定义），自主路径按
//     机器人自身的管理身份。两条路都过 yyQQRunAction 的开关/限流/留痕。

// yyQQSkillsHandler GET /api/qq/assistant/skills 只读视图。
// （原计划挂 /api/qq/skills，但该路径已被游戏技能注册表占用，重复注册会 panic。）
// 返回技能清单 + 回复延迟配置；pending = 延迟双 0（待定，走兜底 2-6 秒）。
func yyQQSkillsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	cf := yyQQSnapshot()
	effMin, effMax := yyQQReplyDelayRange(cf)
	writeJSON(w, map[string]any{
		"ok":     true,
		"skills": yyQQSkillView(cf),
		"delay": map[string]any{
			"min":           cf.Assistant.ReplyDelayMin,
			"max":           cf.Assistant.ReplyDelayMax,
			"pending":       cf.Assistant.ReplyDelayMin == 0 && cf.Assistant.ReplyDelayMax == 0,
			"fallback_min":  2,
			"fallback_max":  6,
			"effective_min": effMin,
			"effective_max": effMax,
		},
	})
}

// yyQQSkillDef 一个技能的注册表定义。
type yyQQSkillDef struct {
	ID       string   // 稳定标识（玩家覆盖的 key）
	Label    string   // 面板显示名
	Word     string   // 默认功能词（assistant.skills[id].word 可覆盖）
	Desc     string   // 给 AI 看的一句话（含使用分寸）
	Tool     string   // 对应 AI 工具名（空 = 没有直接对应的工具）
	Args     string   // 提示词里的参数示例（如「用户 分钟」）
	Cap      []string // 能力闸门 kind 列表（youyuan_qq_caps.go）；全部开启才可用
	MinLevel int      // 面板展示：对应能力的最低角色（0成员/1管理员/2超管）
	Danger   bool     // 危险技能（禁言/踢人），面板标注
}

// yyQQSkillDefs 技能注册表。初始 9 个。
var yyQQSkillDefs = []yyQQSkillDef{
	{ID: "meme", Label: "收藏表情", Word: "meme", Tool: "list_stickers+send_sticker", Args: "编号", Cap: []string{"face", "sticker"}, MinLevel: 0,
		Desc: "想发表情包时用：先 list_stickers 看有哪些收藏表情，再发编号；不带编号直接输出时系统会先列出收藏表情；可以连发多张，像真人一样看场合"},
	{ID: "poke", Label: "拍一拍", Word: "poke", Tool: "poke", Args: "用户", Cap: []string{"poke"}, MinLevel: 0,
		Desc: "和群友互动时可以拍一下对方（打招呼、叫人、开玩笑），别连着拍同一个人"},
	{ID: "voice", Label: "发语音", Word: "语音", Tool: "send_voice", Args: "你要说的话", Cap: []string{"voice"}, MinLevel: 1,
		Desc: "想用声音说话时用：技能[语音] 你要说的话（≤300字）。适合情绪强烈、想卖萌、想强调的时候，别滥用"},
	{ID: "like", Label: "点赞", Word: "like", Tool: "", Args: "用户", Cap: []string{"like"}, MinLevel: 0,
		Desc: "给聊得来的群友点赞"},
	{ID: "ban", Label: "禁言", Word: "禁言", Tool: "ban", Args: "用户 分钟", Cap: []string{"ban"}, MinLevel: 2, Danger: true,
		Desc: "你是群管理员，群里有人违规、刷屏、惹恼你时，可以禁言他。要有理由、有分寸，禁言时长合理（默认10分钟），别滥用；@某人 或给显示名+分钟数"},
	{ID: "kick", Label: "踢人", Word: "踢人", Tool: "kick", Args: "用户", Cap: []string{"kick"}, MinLevel: 1, Danger: true,
		Desc: "极端情况（严重违规）才踢人，踢了不加黑名单"},
	{ID: "avatar", Label: "改头像", Word: "改头像", Tool: "", Args: "图片URL", Cap: []string{"avatar"}, MinLevel: 2,
		Desc: "你可以给自己换头像（需要图片URL）"},
	{ID: "emoji", Label: "贴表情", Word: "表情", Tool: "", Args: "", Cap: []string{"emoji_like"}, MinLevel: 0,
		Desc: "在群友消息上贴 QQ 表情回应（不填参数用默认点赞表情）"},
	{ID: "recall", Label: "撤回", Word: "撤回", Tool: "", Args: "消息ID", Cap: []string{"recall"}, MinLevel: 2,
		Desc: "自己或机器人发错的消息可以撤回；不填参数时撤回最近一条群消息"},
}

// yyQQSkillView 面板视图：合并玩家覆盖与能力闸门后的每个技能状态。
// cap_off=true 表示对应能力在「机器人能力」里被关掉（此时 enabled=false）；
// tool_off=true 表示对应工具在「AI 工具」里被关掉（同样 enabled=false）。
func yyQQSkillView(cf yyQQConfig) []map[string]any {
	out := make([]map[string]any, 0, len(yyQQSkillDefs))
	for _, def := range yyQQSkillDefs {
		capOK := yyQQSkillCapOK(cf, def.ID)
		toolOK := yyQQSkillToolOK(cf, def.ID)
		out = append(out, map[string]any{
			"id":        def.ID,
			"label":     def.Label,
			"word":      yyQQSkillWord(cf, def.ID),
			"desc":      def.Desc,
			"min_level": def.MinLevel,
			"danger":    def.Danger,
			"enabled":   yyQQSkillEnabled(cf, def.ID) && capOK && toolOK,
			"tool":      def.Tool,
			"cap_off":   !capOK,
			"tool_off":  !toolOK,
		})
	}
	return out
}

// yyQQSkillWord 取生效功能词：玩家 Word 优先，否则注册表默认。
func yyQQSkillWord(cf yyQQConfig, id string) string {
	if v, ok := cf.Assistant.Skills[id]; ok {
		if w := strings.TrimSpace(v.Word); w != "" {
			return w
		}
	}
	for _, def := range yyQQSkillDefs {
		if def.ID == id {
			return def.Word
		}
	}
	return ""
}

// yyQQSkillEnabled 技能开关本身（玩家 Enabled 优先，缺省全开）。
// 能力闸门不在这里判断——那是另一层，见 yyQQSkillCapOK。
func yyQQSkillEnabled(cf yyQQConfig, id string) bool {
	if v, ok := cf.Assistant.Skills[id]; ok && v.Enabled != nil {
		return *v.Enabled
	}
	return true
}

// yyQQSkillCapOK 技能对应的能力闸门是否全开。找不到技能返回 false。
func yyQQSkillCapOK(cf yyQQConfig, id string) bool {
	for _, def := range yyQQSkillDefs {
		if def.ID != id {
			continue
		}
		for _, kind := range def.Cap {
			if kind == "" {
				continue
			}
			if !yyQQCapEnabled(cf.Assistant, kind) {
				return false
			}
		}
		return true
	}
	return false
}

// yyQQSkillToolOK 技能对应的 AI 工具是否开启。
// 技能只是提示词层的壳，真正执行走工具（youyuan_qq_toolbox.go 的注册表）——
// 用户在「AI 工具」里关掉 ban/kick 后，技能这条路也必须停，
// 否则「保存后真的生效」是假的：面板显示关了，AI 照样能禁言。
func yyQQSkillToolOK(cf yyQQConfig, id string) bool {
	for _, def := range yyQQSkillDefs {
		if def.ID != id {
			continue
		}
		for _, name := range strings.Split(def.Tool, "+") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if !yyQQToolEnabled(cf.Assistant, name) {
				return false
			}
		}
		return true
	}
	return false
}

// yyQQSkillPromptBlock 给 AI 提示词的技能清单段；全部关闭时返回空串。
// 功能词取玩家覆盖后的生效值（玩家把 meme 换成「表情包」，AI 看到的就是技能[表情包]）。
func yyQQSkillPromptBlock(cf yyQQConfig) string {
	lines := []string{}
	for _, def := range yyQQSkillDefs {
		if !yyQQSkillEnabled(cf, def.ID) || !yyQQSkillCapOK(cf, def.ID) || !yyQQSkillToolOK(cf, def.ID) {
			continue
		}
		line := "- 技能[" + yyQQSkillWord(cf, def.ID) + "]"
		if def.Args != "" {
			line += " " + def.Args
		}
		lines = append(lines, line+"："+def.Desc)
	}
	if len(lines) == 0 {
		return ""
	}
	return "你拥有的技能（想用时输出：技能[功能词] 参数，系统会替你执行并把执行结果告诉你）：\n" +
		strings.Join(lines, "\n") +
		"\n（技能段不会直接发给玩家；一次可以用多个，执行结果会以系统消息告诉你，你再按结果说话。）"
}

// yyQQSkillByWord 按功能词反查技能定义（大小写不敏感、去空格）。
func yyQQSkillByWord(cf yyQQConfig, word string) *yyQQSkillDef {
	w := yyQQNameNorm(word)
	if w == "" {
		return nil
	}
	for i := range yyQQSkillDefs {
		if yyQQNameNorm(yyQQSkillWord(cf, yyQQSkillDefs[i].ID)) == w {
			return &yyQQSkillDefs[i]
		}
	}
	return nil
}

// yyQQResolveGroupUser 把 AI 给的「用户引用」解析成真实 user_id
// （OneBot set_group_ban 等接口需要数字 QQ 号）。
// 解析顺序：① 5-11 位纯数字直接用；② 成员字典（GET /api/qq/assistant 同源）
// 按显示名精确/包含匹配；③ 最近群消息按显示名匹配；④ 找不到返回空，
// 调用方给 AI 可操作的错误。
func yyQQResolveGroupUser(cf yyQQConfig, ref string) string {
	ref = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ref), "@"))
	if ref == "" {
		return ""
	}
	if yyQQResolveNumericID(ref) {
		return ref
	}
	// ② 成员字典（yyQQHelper.Seen，与 /api/qq/assistant 的 members 同源）
	yyQQHelper.Lock()
	seen := make(map[string]string, len(yyQQHelper.Seen))
	for id, name := range yyQQHelper.Seen {
		seen[id] = name
	}
	yyQQHelper.Unlock()
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, exact := range []bool{true, false} {
		for _, id := range ids {
			if yyQQNameHit(yyQQDisplayName(cf, id, seen[id]), ref, exact) || yyQQNameHit(seen[id], ref, exact) {
				return id
			}
		}
	}
	// ③ 最近群消息（机器人只记录自己看到过的消息）按显示名匹配
	rows := yyQQRecentMessages(cf, 80)
	for _, exact := range []bool{true, false} {
		for _, row := range rows {
			if row.User != "" && yyQQNameHit(row.Name, ref, exact) {
				return row.User
			}
		}
	}
	return ""
}

// yyQQResolveNumericID 5-11 位纯数字（QQ 号量级）。
func yyQQResolveNumericID(ref string) bool {
	if len(ref) < 5 || len(ref) > 11 {
		return false
	}
	for _, r := range ref {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// yyQQNameNorm 名字规范化：去首尾及内部空白、转小写，让「张 三」=「张三」。
func yyQQNameNorm(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), ""))
}

// yyQQNameHit 显示名匹配：exact=精确相等，否则包含。
func yyQQNameHit(name, ref string, exact bool) bool {
	n, r := yyQQNameNorm(name), yyQQNameNorm(ref)
	if n == "" || r == "" {
		return false
	}
	if exact {
		return n == r
	}
	return strings.Contains(n, r)
}

// yyQQSkillDirectiveRe 技能指令头：技能[功能词]（半角/全角括号都认）。
// 只匹配「头」，参数单独切——Go 的 RE2 没有前向断言，若把参数写成
// ([^\n，。；;]*)  greedy，一行里第二条 技能[...] 会被第一条的参数吃光。
var yyQQSkillDirectiveRe = regexp.MustCompile(`(?:使用)?\s*技能\s*[\[［]([^\]］\n]{1,20})\s*[\]］]`)

// yyQQSkillIndexRe 从参数里取第一个编号（1-3 位数字，收藏表情上限 40）。
var yyQQSkillIndexRe = regexp.MustCompile(`[0-9]{1,3}`)

// yyQQRunSkillDirectives 解析并执行 answer 里的技能指令。
// 返回剥掉技能段后的干净文本和给 AI 的系统执行结果（notes）。
// 未启用/不存在的技能原样保留文本并记 note；执行成功与否都剥掉整段。
func yyQQRunSkillDirectives(cf yyQQConfig, answer string, ctx context.Context) (string, []string) {
	heads := yyQQSkillDirectiveRe.FindAllStringSubmatchIndex(answer, -1)
	if len(heads) == 0 {
		return answer, nil
	}
	notes := []string{}
	var b strings.Builder
	last, stripped := 0, false
	for i, h := range heads {
		word := strings.TrimSpace(answer[h[2]:h[3]])
		// 参数区：] 之后到下一个技能头或行尾，止于中英文句读（句读保留在文本里）。
		start, end := h[1], len(answer)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		if nl := strings.IndexByte(answer[start:end], '\n'); nl >= 0 {
			end = start + nl
		}
		raw := answer[start:end]
		cut := len(raw)
		for _, mark := range []string{"，", "。", "；", ";"} {
			if idx := strings.Index(raw, mark); idx >= 0 && idx < cut {
				cut = idx
			}
		}
		param := strings.TrimSpace(raw[:cut])
		stripEnd := start + cut
		def := yyQQSkillByWord(cf, word)
		if def == nil {
			notes = append(notes, "技能["+word+"] 不存在或已停用")
			continue
		}
		if !yyQQSkillEnabled(cf, def.ID) {
			notes = append(notes, "技能["+word+"] 已停用")
			continue
		}
		if !yyQQSkillCapOK(cf, def.ID) {
			notes = append(notes, "技能["+word+"] 对应的能力已在设置里关闭，先在工具箱「群服互通 · 机器人能力」打开")
			continue
		}
		// 工具被用户在「AI 工具」里关掉：整段剥掉（别把 技能[xxx] 原文
		// 漏到群里），用 note 告诉 AI 是管理开关关的，下轮别再用。
		if !yyQQSkillToolOK(cf, def.ID) {
			notes = append(notes, "技能["+word+"] 对应的 AI 工具已关闭，先在工具箱「AI 工具」里打开")
			b.WriteString(answer[last:h[0]])
			last = stripEnd
			stripped = true
			continue
		}
		notes = append(notes, yyQQExecuteSkill(cf, *def, param, ctx))
		b.WriteString(answer[last:h[0]])
		last = stripEnd
		stripped = true
	}
	if !stripped {
		return answer, notes
	}
	b.WriteString(answer[last:])
	return strings.TrimSpace(b.String()), notes
}

// yyQQExecuteSkill 执行单个技能，返回给 AI 看的执行结果（一句话）。
// 统一以「机器人自己是群管理」的身份（level=2）过 yyQQRunAction 闸门：
// 开关、冷却、每日上限、留痕与群指令共用；提问者权限只约束工具调用路径。
func yyQQExecuteSkill(cf yyQQConfig, def yyQQSkillDef, param string, ctx context.Context) string {
	group := yyQQGroupString(cf.GroupID)
	switch def.ID {
	case "meme":
		return yyQQSkillMeme(cf, def, param, ctx)
	case "ban":
		ref, minutes, reason := yyQQSkillUserMinutes(param)
		if ref == "" {
			return "禁言没成功：格式是 技能[" + def.Word + "] 用户 分钟（分钟可省，默认10）；用户可以是 @某人、显示名或 QQ 号"
		}
		if minutes <= 0 {
			minutes = 10
		}
		if minutes > 43200 { // 30 天封顶
			minutes = 43200
		}
		user := yyQQResolveGroupUser(cf, ref)
		if user == "" {
			return "禁言没成功：没找到这个人（" + yyQQCardClip(ref, 30) + "），把 QQ 号或完整昵称给我"
		}
		if group == "" {
			return "禁言没成功：还没选群，不知道在哪个群禁言"
		}
		detail := "AI 禁言 " + yyQQCardClip(ref, 30) + " " + strconv.Itoa(minutes) + " 分钟"
		if reason != "" {
			detail += "（" + yyQQCardClip(reason, 40) + "）"
		}
		if e := yyQQRunAction("ban", "ai", "AI助手", user, detail, 2, func() (string, error) {
			return "", yyQQActionBan(group, user, minutes)
		}); e != nil {
			return "禁言没成功：" + e.Error()
		}
		return "已禁言 " + yyQQCardClip(ref, 30) + " " + strconv.Itoa(minutes) + " 分钟"
	case "kick":
		ref, _, reason := yyQQSkillUserMinutes(param)
		if ref == "" {
			return "踢人没成功：格式是 技能[" + def.Word + "] 用户 [原因]；用户可以是 @某人、显示名或 QQ 号"
		}
		user := yyQQResolveGroupUser(cf, ref)
		if user == "" {
			return "踢人没成功：没找到这个人（" + yyQQCardClip(ref, 30) + "），把 QQ 号或完整昵称给我"
		}
		if group == "" {
			return "踢人没成功：还没选群，不知道在哪个群踢人"
		}
		detail := "AI 踢人 " + yyQQCardClip(ref, 30)
		if reason != "" {
			detail += "（" + yyQQCardClip(reason, 40) + "）"
		}
		if e := yyQQRunAction("kick", "ai", "AI助手", user, detail, 2, func() (string, error) {
			return "", yyQQActionKick(group, user, false)
		}); e != nil {
			return "踢人没成功：" + e.Error()
		}
		return "已把 " + yyQQCardClip(ref, 30) + " 移出群聊（没加黑名单）"
	case "poke":
		ref := strings.TrimSpace(strings.TrimPrefix(param, "@"))
		if ref == "" {
			return "拍一拍没成功：格式是 技能[" + def.Word + "] 用户（@某人、显示名或 QQ 号）"
		}
		user := yyQQResolveGroupUser(cf, ref)
		if user == "" {
			return "拍一拍没成功：没找到这个人（" + yyQQCardClip(ref, 30) + "），把 QQ 号或完整昵称给我"
		}
		if group == "" {
			return "拍一拍没成功：还没选群，无法拍一拍"
		}
		if e := yyQQRunAction("poke", "ai", "AI助手", user, "AI 拍 QQ"+user, 2, func() (string, error) {
			return "", yyPokeGroup(group, user)
		}); e != nil {
			return "拍一拍没成功：" + e.Error()
		}
		return "已拍了拍 " + yyQQCardClip(ref, 30)
	case "voice":
		raw := strings.TrimSpace(param)
		if raw == "" {
			return "发语音没成功：格式是 技能[" + def.Word + "] 你要说的话（≤300字），或音频直链"
		}
		if group == "" {
			return "发语音没成功：还没选群，不知道在哪个群发"
		}
		if !yyVoiceURL(raw) && aiStore.Get(cf.Assistant.VoiceModel) == nil {
			return "发语音没成功：还没配置语音模型，先在 QQ 控制台·人设里选语音模型"
		}
		clipped := false
		if e := yyQQRunAction("voice", "ai", "AI助手", "", "AI 语音："+yyQQCardClip(raw, 40), 2, func() (string, error) {
			file, cut, err := yyQQVoicePrepare(cf, raw)
			clipped = cut
			if err != nil {
				return "", err
			}
			return "", yyVoiceSend(group, file)
		}); e != nil {
			return "发语音没成功：" + e.Error()
		}
		if clipped {
			return "已发出语音（文本超过 " + strconv.Itoa(yyVoiceTextMax) + " 字，只念了前 " + strconv.Itoa(yyVoiceTextMax) + " 字）"
		}
		return "已发出语音"
	case "like":
		ref := strings.TrimSpace(strings.TrimPrefix(param, "@"))
		if ref == "" {
			return "点赞没成功：格式是 技能[" + def.Word + "] 用户（@某人、显示名或 QQ 号）"
		}
		user := yyQQResolveGroupUser(cf, ref)
		if user == "" {
			return "点赞没成功：没找到这个人（" + yyQQCardClip(ref, 30) + "），把 QQ 号或完整昵称给我"
		}
		if e := yyQQRunAction("like", "ai", "AI助手", user, "AI 点赞 QQ"+user, 2, func() (string, error) {
			return "", yyQQActionLike(user, 1)
		}); e != nil {
			return "点赞没成功：" + e.Error()
		}
		return "已给 " + yyQQCardClip(ref, 30) + " 点赞"
	case "emoji":
		emojiID := ""
		if f := strings.Fields(param); len(f) > 0 {
			emojiID = f[0]
		}
		// 贴表情需要消息 ID：取最近一条群友（非机器人）消息作为回应对象。
		target := ""
		for _, row := range yyQQRecentMessages(cf, 80) {
			if !row.IsBot && row.ID != "" {
				target = row.ID
			}
		}
		if target == "" {
			return "贴表情没成功：找不到可以贴表情的群友消息（机器人只记录自己看到过的消息）"
		}
		if e := yyQQRunAction("emoji_like", "ai", "AI助手", target, "AI 贴表情 emoji="+yyQQCardClip(emojiID, 20), 2, func() (string, error) {
			return "", yyQQActionEmojiLike(target, emojiID, true)
		}); e != nil {
			return "贴表情没成功：" + e.Error()
		}
		return "已在群友的消息上贴了 QQ 表情回应"
	case "avatar":
		url := strings.TrimSpace(param)
		if url == "" {
			return "改头像没成功：格式是 技能[" + def.Word + "] 图片URL（http/https）"
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return "改头像没成功：只支持 http/https 图片链接"
		}
		if len(url) > 2000 {
			return "改头像没成功：链接太长（最多 2000 字符）"
		}
		if e := yyQQRunAction("avatar", "ai", "AI助手", "", yyQQCardClip(url, 60), 2, func() (string, error) {
			return "", yyQQActionAvatar(url)
		}); e != nil {
			return "改头像没成功：" + e.Error()
		}
		return "头像已更新"
	case "recall":
		messageID := strings.TrimSpace(param)
		label := "指定消息"
		if messageID == "" {
			rows := yyQQRecentMessages(cf, 80)
			if len(rows) == 0 {
				return "撤回没成功：找不到可撤回的消息（机器人只记录自己看到过的群消息）"
			}
			last := rows[len(rows)-1]
			messageID = last.ID
			label = yyQQCardClip(strings.TrimSpace(last.Name+"："+last.Text), 40)
		}
		if messageID == "" {
			return "撤回没成功：那条消息没有可用的消息ID，撤回需要消息ID"
		}
		if e := yyQQRunAction("recall", "ai", "AI助手", messageID, "AI 撤回 "+label, 2, func() (string, error) {
			return "", yyQQActionRecall(group, messageID)
		}); e != nil {
			return "撤回没成功：" + e.Error()
		}
		return "已撤回（" + label + "）"
	}
	return "技能[" + def.Word + "] 暂不支持自动执行"
}

// yyQQSkillMeme 收藏表情技能：有编号→发送；无编号→自动拉一次列表回给 AI
// （等价于 list_stickers 工具被自动调用，NapCat 离线时记录失败原因）。
func yyQQSkillMeme(cf yyQQConfig, def yyQQSkillDef, param string, ctx context.Context) string {
	idx := 0
	if m := yyQQSkillIndexRe.FindString(param); m != "" {
		if n, e := strconv.Atoi(m); e == nil && n > 0 {
			idx = n
		}
	} else if strings.TrimSpace(param) != "" {
		return "发表情包没成功：参数应该是收藏表情编号（数字）；不带编号直接输出，系统会先列出收藏表情"
	}
	if idx <= 0 {
		out, _ := yyQQSocialToolCall(ctx, cf, "list_stickers", map[string]any{}, 2, "ai", "AI助手")
		var res map[string]any
		if e := json.Unmarshal([]byte(out), &res); e != nil || res["ok"] != true {
			return "收藏表情列表拉取失败" + yyQQSkillToolErr(res) + "（NapCat 离线时读不到收藏表情），下轮再试或先用文字回应"
		}
		stickers, _ := res["stickers"].([]any)
		if len(stickers) == 0 {
			return "机器人没有收藏表情（QQ 收藏表情是空的），换个方式表达"
		}
		names := make([]string, 0, len(stickers))
		for i, raw := range stickers {
			if i >= 40 {
				break
			}
			label := strconv.Itoa(i + 1)
			if row, ok := raw.(map[string]any); ok {
				if v := strings.TrimSpace(fmt.Sprint(row["name"])); v != "" && v != "<nil>" {
					label += "." + yyQQCardClip(v, 20)
				}
			}
			names = append(names, label)
		}
		return "已拉到收藏表情列表：" + strings.Join(names, " ") + "。想发哪张就输出 技能[" + yyQQSkillWord(cf, def.ID) + "] 编号（一次可以连发多张）"
	}
	out, _ := yyQQSocialToolCall(ctx, cf, "send_sticker", map[string]any{"index": idx}, 2, "ai", "AI助手")
	var res map[string]any
	if e := json.Unmarshal([]byte(out), &res); e != nil || res["ok"] != true {
		return "发表情包没成功" + yyQQSkillToolErr(res)
	}
	sent := "收藏表情"
	if v, ok := res["sent"].(string); ok && v != "" {
		sent = v
	}
	return "已发送" + sent
}

// yyQQSkillToolErr 从社交工具 JSON 输出里取错误说明（拼成「：原因」）。
func yyQQSkillToolErr(res map[string]any) string {
	if res == nil {
		return ""
	}
	if v, ok := res["error"].(string); ok && strings.TrimSpace(v) != "" {
		return "：" + v
	}
	return ""
}

// yyQQSkillUserMinutes 解析「用户 [分钟] [原因…]」参数。
func yyQQSkillUserMinutes(param string) (string, int, string) {
	f := strings.Fields(param)
	if len(f) == 0 {
		return "", 0, ""
	}
	ref := strings.TrimPrefix(f[0], "@")
	minutes, rest := 0, f[1:]
	if len(rest) > 0 {
		if n, e := strconv.Atoi(rest[0]); e == nil {
			minutes = n
			rest = rest[1:]
		}
	}
	return ref, minutes, strings.Join(rest, " ")
}
