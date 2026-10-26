package main

// 群规 · 软通道：自然语言禁令 + AI 提示词注入。
//
// 用户原话：「也可以直接跟AI说话，让他以后都记住，然后禁止掉这种行为」。
// 超管在群里直接说「以后不准 玩家打架」，一句话双通道同时生效：
//   硬通道：生成一条 substring 规则（见 youyuan_qq_moderation.go），
//           玩家再发相关词会被直接拦下（block/recall/recall_ban）；
//   软通道：写进人格卡 Taboos（qq-persona.json，永久、重启不丢），
//           AI 回复时遵守并劝阻。
// 群记忆（RAG）有 TTL 会过期，存不了「永久禁令」，所以禁令不走记忆通道。
// 「取消禁令 X」可解除。

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 禁令句式前缀，长在前短在后，匹配时按顺序命中第一个。
// 注意：消息先进 yyQQModerationNormText（全角冒号会转半角），所以这里写半角。
var yyQQBanPrefixes = []string{
	"以后都不准", "以后不准", "以后别让", "以后禁止", "以后别",
	"记住不准", "记住:不准", "记住:禁止", "记住禁止",
	"不准", "禁止", "别让", "不允许",
}

// 解除禁令句式。
var yyQQUnbanPrefixes = []string{"取消禁令", "解除禁令", "取消禁止", "解除禁止", "取消不准"}

// 禁令对象主体词，抽取核心内容时剥掉（「不准玩家打架」→「打架」）。
var yyQQBanSubjects = []string{"玩家们", "群友们", "玩家", "群友", "他们", "大家", "所有人", "任何人", "这些人", "这些人"}

// yyQQBanExtract 从禁令句尾抽取违禁核心：优先取引号内内容，否则剥掉主体词。
func yyQQBanExtract(tail string) string {
	t := strings.TrimSpace(tail)
	for _, pair := range [][2]string{{"「", "」"}, {"“", "”"}, {"\"", "\""}, {"'", "'"}} {
		if i := strings.Index(t, pair[0]); i >= 0 {
			if j := strings.Index(t[i+len(pair[0]):], pair[1]); j >= 0 {
				inner := strings.TrimSpace(t[i+len(pair[0]) : i+len(pair[0])+j])
				if inner != "" {
					return yyPersonaClip(inner, yyQQModMaxPattern)
				}
			}
		}
	}
	for _, s := range yyQQBanSubjects {
		for strings.HasPrefix(t, s) {
			t = strings.TrimSpace(strings.TrimPrefix(t, s))
		}
	}
	for _, s := range []string{"在群里", "在群", "再", "又", "给我", "跟我"} {
		for strings.HasPrefix(t, s) {
			t = strings.TrimSpace(strings.TrimPrefix(t, s))
		}
	}
	t = strings.TrimLeft(t, "，。！？!?,.;；:： ")
	if t == "" || len([]rune(t)) < 2 {
		return ""
	}
	return yyPersonaClip(t, yyQQModMaxPattern)
}

// yyQQNaturalBan 识别并处理超管的自然语言禁令/解禁令。
// 返回 true 表示这条消息已被本函数消费（调用方应直接 return，不再走后续链路）。
func yyQQNaturalBan(cf yyQQConfig, userID, name, text, id string) bool {
	if yyQQRole(cf, userID) != 2 {
		return false // 只有超管能改群规
	}
	t := strings.TrimSpace(yyQQModerationNormText(text))
	if t == "" {
		return false
	}
	for _, p := range yyQQUnbanPrefixes {
		if strings.HasPrefix(t, p) {
			core := yyQQBanExtract(strings.TrimSpace(strings.TrimPrefix(t, p)))
			if core == "" {
				yyQQFeedbackTo(cf, "格式：取消禁令 关键词", id)
				return true
			}
			okRule := yyQQModerationRemove(core, "substring")
			okTaboo := yyQQPersonaRemoveTaboo(core)
			if okRule || okTaboo {
				yyQQFeedbackTo(cf, "已解除关于「"+core+"」的禁令。", id)
			} else {
				yyQQFeedbackTo(cf, "没找到关于「"+core+"」的禁令，检查下字对不对。", id)
			}
			return true
		}
	}
	for _, p := range yyQQBanPrefixes {
		if !strings.HasPrefix(t, p) {
			continue
		}
		tail := strings.TrimSpace(strings.TrimPrefix(t, p))
		// 短前缀（如「以后别」）会把「让」留在 tail 里，先剥掉，
		// 让「以后别让我操心」和「以后别让 我操心」走同一条判断。
		tail = strings.TrimPrefix(tail, "让")
		// 防误伤：「别让我失望」「不准我说话」是超管在说自己的事，不是群规。
		// 群规的对象永远是群友，tail 以「我」开头的一律不当禁令，放行给 AI。
		if strings.HasPrefix(tail, "我") {
			continue
		}
		core := yyQQBanExtract(tail)
		if core == "" {
			continue // 句式像但抽不出内容，不当禁令，放行给 AI
		}
		taboo := yyPersonaClip(t, 40)
		rule, addedRule := yyQQModerationAddNatural(core, "违反群规")
		addedTaboo := yyQQPersonaAddTaboo(taboo)
		if !addedRule && !addedTaboo {
			yyQQFeedbackTo(cf, "这条禁令已经在生效了：「"+core+"」。", id)
			return true
		}
		msg := "记住了，以后不准「" + core + "」。\n" +
			"· 群规已生效：群友再发会被直接拦下"
		if cf.Adapter == "onebot" && yyQQCapEnabled(cf.Assistant, "recall") {
			msg += "并撤回"
		}
		if cf.Adapter == "onebot" && yyQQCapEnabled(cf.Assistant, "ban") {
			msg += "、禁言 " + strconv.Itoa(yyQQModBanMinutes) + " 分钟"
		}
		msg += "。\n· 我也会记住，不再提这件事。\n发「取消禁令 " + core + "」可解除。"
		if !rule.Enabled {
			msg = "记住了，以后不准「" + core + "」。\nAI 侧已生效；群规侧的对应规则当前是关闭状态，需要在面板里打开。"
		}
		yyQQFeedbackTo(cf, msg, id)
		yyQQLog("自然语言禁令 by " + name + "(" + userID + ")：" + core)
		return true
	}
	return false
}

// yyQQPersonaAddTaboo 追加一条 AI 长期禁忌（人格卡，落 qq-persona.json）。
// 去重；超 12 条时 normalize 会裁掉最旧的之外的——实际保留前 12 条。
func yyQQPersonaAddTaboo(taboo string) bool {
	taboo = strings.TrimSpace(taboo)
	if taboo == "" {
		return false
	}
	yyQQPersonaState.Lock()
	defer yyQQPersonaState.Unlock()
	for _, v := range yyQQPersonaState.Persona.Taboos {
		if v == taboo {
			return false
		}
	}
	yyQQPersonaState.Persona.Taboos = append(yyQQPersonaState.Persona.Taboos, taboo)
	yyQQPersonaState.Persona = yyQQPersonaNormalize(yyQQPersonaState.Persona)
	yyQQPersonaSaveLocked()
	return true
}

// yyQQPersonaRemoveTaboo 解除一条禁忌。按整句或包含匹配都试一次，
// 免得超管记不清原句时解除不了。
func yyQQPersonaRemoveTaboo(taboo string) bool {
	taboo = strings.TrimSpace(taboo)
	if taboo == "" {
		return false
	}
	yyQQPersonaState.Lock()
	defer yyQQPersonaState.Unlock()
	out := make([]string, 0, len(yyQQPersonaState.Persona.Taboos))
	removed := false
	for _, v := range yyQQPersonaState.Persona.Taboos {
		if !removed && (v == taboo || strings.Contains(v, taboo) || strings.Contains(taboo, v)) {
			removed = true
			continue
		}
		out = append(out, v)
	}
	if !removed {
		return false
	}
	yyQQPersonaState.Persona.Taboos = out
	yyQQPersonaSaveLocked()
	return true
}

// yyQQPersonaTaboos 当前 AI 长期禁令（只读快照，供面板展示）。
func yyQQPersonaTaboos() []string {
	yyQQPersonaState.Lock()
	defer yyQQPersonaState.Unlock()
	out := append([]string(nil), yyQQPersonaState.Persona.Taboos...)
	if out == nil {
		return []string{}
	}
	return out
}

// yyQQPersonaSetTaboos 整表替换 AI 长期禁令：面板上的增、改、删都走它。
// 空词条丢弃、重复词条只留第一次出现的那条，条数与单条长度按人格卡上限裁剪
// （yyPersonaList 里已有裁剪，这里补一次去重）。返回实际生效的列表。
func yyQQPersonaSetTaboos(list []string) []string {
	clean := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, v := range yyPersonaList(list, 12, 40) {
		if seen[v] {
			continue
		}
		seen[v] = true
		clean = append(clean, v)
	}
	yyQQPersonaState.Lock()
	yyQQPersonaState.Persona.Taboos = clean
	yyQQPersonaState.Persona = yyQQPersonaNormalize(yyQQPersonaState.Persona)
	yyQQPersonaSaveLocked()
	out := append([]string(nil), yyQQPersonaState.Persona.Taboos...)
	yyQQPersonaState.Unlock()
	if out == nil {
		return []string{}
	}
	return out
}

// yyQQModerationPromptBlock 把 warn 级群规写进 AI 提示词（软劝阻通道）。
// block/recall 级规则由硬通道拦截，AI 根本看不到那些消息，不需要进提示词。
func yyQQModerationPromptBlock() string {
	yyQQModeration.Lock()
	warns := make([]yyQQModerationRule, 0, 4)
	for _, r := range yyQQModeration.Rules {
		if r.Enabled && r.Action == "warn" {
			warns = append(warns, r)
		}
	}
	yyQQModeration.Unlock()
	if len(warns) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("【群规·软禁令】群主设了这些禁令，系统不会拦截，你要负责打住并劝阻，也不要复述违禁词：\n")
	for i, r := range warns {
		reason := r.Reason
		if reason == "" {
			reason = "违反群规"
		}
		b.WriteString(strconv.Itoa(i+1) + ".不准" + r.Pattern + "（" + reason + "）\n")
	}
	b.WriteString("看到这类内容，简短提醒一句群规就行，别上纲上线，别跟着发。")
	return b.String()
}

// yyQQModerationAIRoutes 预留：AI 自主调用群规查询的只读接口（不做写操作，
// 写操作只认超管——AI 再聪明也不能自己改群规）。
func yyQQModerationAIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/moderation/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		rules, _ := yyQQModerationSnapshot()
		names := make([]string, 0, len(rules))
		for _, rule := range rules {
			if rule.Enabled {
				names = append(names, rule.Pattern)
			}
		}
		writeJSON(w, map[string]any{"ok": true, "rules": names, "at": time.Now().Format(time.RFC3339)})
	})
}
