package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 清空并还原成员字典：按昵称解析的用例不能受其它测试留下的全局状态影响。
func yySkillTestClearSeen(t *testing.T) {
	t.Helper()
	yyQQHelper.Lock()
	saved := yyQQHelper.Seen
	yyQQHelper.Seen = map[string]string{}
	yyQQHelper.Unlock()
	t.Cleanup(func() {
		yyQQHelper.Lock()
		yyQQHelper.Seen = saved
		yyQQHelper.Unlock()
	})
}

// yySkillTestEnableTools 在全局配置里打开指定 AI 工具，测试后还原。
// ban/kick 自「危险操作默认关」起注册表默认就是关的，
// 想验证「开着时」的路径（执行/自检列名）必须先显式打开。
func yySkillTestEnableTools(t *testing.T, ids ...string) {
	t.Helper()
	yyQQMu.Lock()
	saved := yyQQ
	tools := map[string]bool{}
	for k, v := range yyQQ.Assistant.Tools {
		tools[k] = v
	}
	for _, id := range ids {
		tools[id] = true
	}
	yyQQ.Assistant.Tools = tools
	yyQQMu.Unlock()
	t.Cleanup(func() {
		yyQQMu.Lock()
		yyQQ = saved
		yyQQMu.Unlock()
	})
}

func TestSkillPromptBlockContainsWord(t *testing.T) {
	cf := yyQQConfig{}
	block := yyQQSkillPromptBlock(cf)
	if !strings.Contains(block, "技能[meme]") {
		t.Fatal("默认功能词 meme 应出现在技能块里：", block)
	}
	// 玩家把 meme 的功能词改成「表情包」：块里出现新词，不再出现旧词。
	cf.Assistant.Skills = map[string]yyQQSkillConf{"meme": {Word: "表情包"}}
	block = yyQQSkillPromptBlock(cf)
	if !strings.Contains(block, "技能[表情包]") {
		t.Fatal("玩家覆盖的功能词应出现在技能块里：", block)
	}
	if strings.Contains(block, "技能[meme]") {
		t.Fatal("覆盖后旧功能词不该再出现：", block)
	}
}

func TestSkillDisabledHidden(t *testing.T) {
	off := false
	cf := yyQQConfig{Assistant: yyQQAssistantConfig{Skills: map[string]yyQQSkillConf{"ban": {Enabled: &off}}}}
	block := yyQQSkillPromptBlock(cf)
	if strings.Contains(block, "禁言") {
		t.Fatal("禁言技能被关闭后不该出现在技能块里：", block)
	}
	if yyQQSkillEnabled(cf, "ban") {
		t.Fatal("ban 应为停用状态")
	}
	// 视图里同样看不到（enabled=false），其它技能不受影响。
	rows := yyQQSkillView(cf)
	if len(rows) != len(yyQQSkillDefs) {
		t.Fatalf("视图应包含全部 %d 个技能，得到 %d", len(yyQQSkillDefs), len(rows))
	}
	for _, row := range rows {
		if row["id"] == "ban" && row["enabled"] != false {
			t.Fatal("停用的 ban 在视图里应为 enabled=false")
		}
		if row["id"] == "meme" && row["enabled"] != true {
			t.Fatal("没被碰过的 meme 应保持启用")
		}
	}
}

func TestSkillViewCapOff(t *testing.T) {
	// 能力闸门关掉（玩家在面板关了「收藏表情」）：技能 enabled=false 且标注 cap_off。
	cf := yyQQConfig{Assistant: yyQQAssistantConfig{Caps: map[string]bool{"face": false}}}
	for _, row := range yyQQSkillView(cf) {
		if row["id"] != "meme" {
			continue
		}
		if row["enabled"] != false || row["cap_off"] != true {
			t.Fatalf("face 能力关闭时 meme 应 enabled=false/cap_off=true，得到 %v/%v", row["enabled"], row["cap_off"])
		}
	}
	if yyQQSkillCapOK(cf, "meme") {
		t.Fatal("face 关闭时 meme 的闸门不该通过")
	}
	if !yyQQSkillCapOK(cf, "poke") {
		t.Fatal("没被关的 poke 闸门应通过")
	}
}

func TestSkillDirectiveParsesAndStrips(t *testing.T) {
	yySkillTestClearSeen(t)
	// ban 工具默认关（危险操作默认关），这里显式打开再验证执行路径。
	yySkillTestEnableTools(t, "ban")
	cf := yyQQConfig{}
	answer := "好的我去处理 技能[禁言] 张三 10 再见"
	clean, notes := yyQQRunSkillDirectives(cf, answer, context.Background())
	if strings.Contains(clean, "技能") || strings.Contains(clean, "张三") {
		t.Fatal("技能段（含参数）应被剥掉：", clean)
	}
	if strings.TrimSpace(clean) != "好的我去处理" {
		t.Fatal("剩余文本不对：", clean)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "禁言") {
		t.Fatalf("应有一条禁言执行结果 note，得到 %v", notes)
	}
	// 不存在的技能：原样保留文本 + note，不执行。
	clean, notes = yyQQRunSkillDirectives(cf, "请看 技能[飞天] 一下", context.Background())
	if !strings.Contains(clean, "技能[飞天]") {
		t.Fatal("未知技能应原样保留：", clean)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "不存在或已停用") {
		t.Fatalf("未知技能应记 note，得到 %v", notes)
	}
}

func TestSkillDirectiveMultipleOnOneLine(t *testing.T) {
	yySkillTestClearSeen(t)
	cf := yyQQConfig{}
	// 一行两条 + 一条独立成行：都要解析执行（「他想点几个都行」）。
	// 注意用生效功能词（禁言的默认词是「禁言」而不是 ID）。
	answer := "好的 技能[meme] 3 技能[poke] 10001\n完了 技能[禁言] 张三 10"
	clean, notes := yyQQRunSkillDirectives(cf, answer, context.Background())
	if strings.Contains(clean, "技能[") || strings.Contains(clean, "10001") {
		t.Fatal("多条技能段都应被剥掉：", clean)
	}
	if strings.TrimSpace(clean) != "好的\n完了" {
		t.Fatal("剩余文本不对：", clean)
	}
	if len(notes) != 3 {
		t.Fatalf("应有三条执行结果，得到 %v", notes)
	}
	if !strings.Contains(notes[0], "表情") || !strings.Contains(notes[1], "拍") || !strings.Contains(notes[2], "禁言") {
		t.Fatalf("三条 note 内容不对：%v", notes)
	}
}

func TestSkillMemeAutoList(t *testing.T) {
	yySkillTestClearSeen(t)
	cf := yyQQConfig{}
	// 不带编号：解析路径应自动去拉收藏表情列表（NapCat 不在线时记录失败原因）。
	answer := "好嘞 技能[meme]"
	clean, notes := yyQQRunSkillDirectives(cf, answer, context.Background())
	if strings.Contains(clean, "技能[") {
		t.Fatal("技能段应被剥掉：", clean)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "收藏表情") {
		t.Fatalf("无编号时应自动拉收藏表情列表（或记录失败原因），得到 %v", notes)
	}
}

func TestResolveGroupUserNumeric(t *testing.T) {
	yySkillTestClearSeen(t)
	cf := yyQQConfig{}
	for _, ref := range []string{"123456789", "12345", "12345678901", " 12345 ", "@10001"} {
		want := strings.TrimPrefix(strings.TrimSpace(ref), "@")
		if got := yyQQResolveGroupUser(cf, ref); got != want {
			t.Fatalf("yyQQResolveGroupUser(%q) = %q，期望原样 %q", ref, got, want)
		}
	}
	for _, ref := range []string{"", "   ", "1234", "123456789012", "abc123", "不存在的昵称"} {
		if got := yyQQResolveGroupUser(cf, ref); got != "" {
			t.Fatalf("yyQQResolveGroupUser(%q) 应解析不到，得到 %q", ref, got)
		}
	}
	// 成员字典：显示名（含玩家昵称覆盖）精确/包含匹配。
	yyQQHelper.Lock()
	yyQQHelper.Seen = map[string]string{"777": "张三", "888": "raw-name"}
	yyQQHelper.Unlock()
	cf.Assistant.Nicknames = map[string]string{"888": "李四"}
	if got := yyQQResolveGroupUser(cf, "张三"); got != "777" {
		t.Fatalf("精确匹配失败：%q", got)
	}
	if got := yyQQResolveGroupUser(cf, " 张 三 "); got != "777" {
		t.Fatalf("去空格匹配失败：%q", got)
	}
	if got := yyQQResolveGroupUser(cf, "三"); got != "777" {
		t.Fatalf("包含匹配失败：%q", got)
	}
	if got := yyQQResolveGroupUser(cf, "李四"); got != "888" {
		t.Fatalf("昵称覆盖后的显示名匹配失败：%q", got)
	}
	if got := yyQQResolveGroupUser(cf, "王五"); got != "" {
		t.Fatalf("不存在的昵称应返回空，得到 %q", got)
	}
}

func TestReplyDelayBounds(t *testing.T) {
	cases := []struct {
		min, max         int
		wantMin, wantMax int
	}{
		{0, 0, 2, 6}, // 双 0 = 待定 → 兜底 2-6
		{8, 3, 3, 8}, // min>max 交换
		{0, 5, 0, 5}, // 单方为 0 → [0,5]
		{10, 10, 10, 10},
		{-2, 4, 0, 4}, // 负数按 0
		{5, -1, 0, 5}, // 单方为 0（负数归零）另一方非 0
		{3, 9, 3, 9},  // 正常区间不动
	}
	for _, c := range cases {
		cf := yyQQConfig{}
		cf.Assistant.ReplyDelayMin, cf.Assistant.ReplyDelayMax = c.min, c.max
		gotMin, gotMax := yyQQReplyDelayRange(cf)
		if gotMin != c.wantMin || gotMax != c.wantMax {
			t.Fatalf("yyQQReplyDelayRange(min=%d,max=%d) = (%d,%d)，期望 (%d,%d)", c.min, c.max, gotMin, gotMax, c.wantMin, c.wantMax)
		}
	}
}

func TestQQSkillsHandlerPayload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/qq/assistant/skills", yyQQSkillsHandler)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/qq/assistant/skills", nil))
	if w.Code != 200 {
		t.Fatalf("状态码 %d", w.Code)
	}
	var res struct {
		OK     bool             `json:"ok"`
		Skills []map[string]any `json:"skills"`
		Delay  struct {
			Min          int  `json:"min"`
			Max          int  `json:"max"`
			Pending      bool `json:"pending"`
			FallbackMin  int  `json:"fallback_min"`
			FallbackMax  int  `json:"fallback_max"`
			EffectiveMin int  `json:"effective_min"`
			EffectiveMax int  `json:"effective_max"`
		} `json:"delay"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &res); e != nil {
		t.Fatal(e, w.Body.String())
	}
	if !res.OK || len(res.Skills) != len(yyQQSkillDefs) {
		t.Fatalf("技能数量不对：%d", len(res.Skills))
	}
	ids := map[string]bool{}
	for _, row := range res.Skills {
		for _, key := range []string{"id", "label", "word", "desc", "min_level", "danger", "enabled", "tool"} {
			if _, ok := row[key]; !ok {
				t.Fatalf("技能视图缺少字段 %s：%v", key, row)
			}
		}
		ids[row["id"].(string)] = true
	}
	for _, want := range []string{"meme", "poke", "like", "ban", "kick", "avatar", "emoji", "recall"} {
		if !ids[want] {
			t.Fatalf("缺少技能 %s", want)
		}
	}
	if !res.Delay.Pending || res.Delay.FallbackMin != 2 || res.Delay.FallbackMax != 6 {
		t.Fatalf("延迟视图不对：%+v", res.Delay)
	}
	if res.Delay.EffectiveMin != 2 || res.Delay.EffectiveMax != 6 {
		t.Fatalf("待定时生效区间应为兜底 2-6：%+v", res.Delay)
	}
	// 方法限制
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/qq/assistant/skills", nil))
	if w.Code != 405 {
		t.Fatalf("POST 应被拒绝，得到 %d", w.Code)
	}
}

// 走完整注册路径：确认 /api/qq/assistant/skills 与既有路由不冲突
// （同一 mux 重复注册会 panic），且 GET 真的能拿到技能视图。
func TestQQAssistantRoutesRegisterSkillsView(t *testing.T) {
	oldHelperPath, oldApprPath, oldApprItems := yyQQHelper.Path, yyAppr.Path, yyAppr.Items
	// 带 sync.Mutex 的全局变量不能整结构体拷贝（go vet 会拦），逐字段保存。
	oldMemPath, oldMemGroups, oldMemEpoch := yyQQMemory.Path, yyQQMemory.Groups, yyQQMemory.Epoch
	oldMemStatus, oldMemErr := yyQQMemory.Status, yyQQMemory.DiskError
	t.Cleanup(func() {
		yyQQHelper.Path = oldHelperPath
		yyAppr.Path, yyAppr.Items = oldApprPath, oldApprItems
		yyQQMemory.Path, yyQQMemory.Groups, yyQQMemory.Epoch = oldMemPath, oldMemGroups, oldMemEpoch
		yyQQMemory.Status, yyQQMemory.DiskError = oldMemStatus, oldMemErr
	})
	mux := http.NewServeMux()
	yyQQAssistantRoutes(mux, t.TempDir())
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/qq/assistant/skills", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"skills"`) {
		t.Fatalf("技能视图路由异常：%d %s", w.Code, w.Body.String())
	}
}

func TestQQAISelfcheckListsBanKick(t *testing.T) {
	// ban/kick 默认关（危险操作默认关）：先显式打开，再验证等级收窄与列名。
	yySkillTestEnableTools(t, "ban", "kick")
	has := func(tools []ToolDef, name string) bool {
		for _, tl := range tools {
			if tl.Name == name {
				return true
			}
		}
		return false
	}
	// 等级收窄：成员两个都没有；管理员有 kick 没有 ban；超管两个都有。
	if has(yyQQAITools(true, 0), "ban") || has(yyQQAITools(true, 0), "kick") {
		t.Fatal("成员不该拿到 ban/kick 工具")
	}
	if !has(yyQQAITools(true, 1), "kick") || has(yyQQAITools(true, 1), "ban") {
		t.Fatal("管理员应有 kick、不该有 ban")
	}
	if !has(yyQQAITools(true, 2), "ban") || !has(yyQQAITools(true, 2), "kick") {
		t.Fatal("超级管理员应有 ban 和 kick")
	}
	// selfcheck 输出（最高权限全量清单）里能看到这两个工具。
	mux := http.NewServeMux()
	yyQQAISelfRoutes(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/qq/ai/selfcheck", nil))
	if w.Code != 200 {
		t.Fatalf("selfcheck 状态码 %d", w.Code)
	}
	var res struct {
		OK     bool     `json:"ok"`
		Tools  []string `json:"tools"`
		Social []string `json:"social_tools"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &res); e != nil {
		t.Fatal(e, w.Body.String())
	}
	for _, name := range []string{"ban", "kick"} {
		found := false
		for _, v := range append(append([]string{}, res.Tools...), res.Social...) {
			if v == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("selfcheck 应列出 %s：tools=%v social=%v", name, res.Tools, res.Social)
		}
	}
}

// 危险操作默认关（用户：「踢人、封禁、执行指令、改配置默认关；查询类默认开」）。
func TestQQToolRegistryDangerDefaultsOff(t *testing.T) {
	for _, id := range []string{"ban", "kick", "game_call", "mcp_call"} {
		d, ok := yyQQToolDefOf(id)
		if !ok {
			t.Fatal("注册表缺少", id)
		}
		if d.Default {
			t.Fatal("危险操作应默认关：", id)
		}
	}
	for _, id := range []string{"ban", "kick", "game_call", "mcp_call"} {
		d, _ := yyQQToolDefOf(id)
		if !d.Danger {
			t.Fatal("应标 Danger：", id)
		}
	}
	for _, id := range []string{"online_players", "public_scores", "scoreboard_count", "group_summary", "poke", "game_skill"} {
		d, ok := yyQQToolDefOf(id)
		if !ok || !d.Default {
			t.Fatal("查询/游戏/社交类应默认开：", id)
		}
	}
}

// 技能 ↔ 工具联动：用户在「AI 工具」里关掉 ban，技能这条路也必须停，
// 提示词不列、指令剥掉并 note 指向「AI 工具」。
func TestSkillToolOffGate(t *testing.T) {
	cf := yyQQConfig{}
	for _, row := range yyQQSkillView(cf) {
		switch row["id"] {
		case "ban", "kick":
			if row["tool_off"] != true || row["enabled"] != false {
				t.Fatalf("%s 应 tool_off=true/enabled=false，得到 %v/%v", row["id"], row["tool_off"], row["enabled"])
			}
		case "meme":
			if row["tool_off"] != false || row["enabled"] != true {
				t.Fatalf("meme 应保持启用，得到 %v/%v", row["tool_off"], row["enabled"])
			}
		}
	}
	if block := yyQQSkillPromptBlock(cf); strings.Contains(block, "禁言") {
		t.Fatal("ban 工具关闭时禁言技能不该进提示词块：", block)
	}
	clean, notes := yyQQRunSkillDirectives(cf, "好的 技能[禁言] 张三 10 再见", context.Background())
	if strings.Contains(clean, "技能") || strings.Contains(clean, "张三") {
		t.Fatal("工具关闭时技能段也该剥掉：", clean)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "AI 工具") {
		t.Fatalf("应有一条指向「AI 工具」的 note，得到 %v", notes)
	}
}
