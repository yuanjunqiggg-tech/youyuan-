package main

// 帮助菜单 + 开关 + 触发词分层的回归用例。
//
// 守四件事：
//  1. 每个能力/技能/游戏技能都有独立开关，且缺省全开（用户要「默认全开」）；
//  2. 面板关掉一项 → 落盘 → 回读 → 帮助菜单里真的消失（不是只写内存）；
//  3. 触发词 min_role 落盘 + 匹配时真的拦人（成员发管理员规则不触发）；
//  4. /api/qq/help 通过生产注册路径（yyQQCapRoutes）可达，且按 level 过滤。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// yyHelpTestIsolate 把全局配置换成临时目录里的一份，测试结束还原。
// 游戏技能注册表也一并初始化到临时目录——生产里 yyRegisterQQ 会先调
// yyGameSkillInit，测试不初始化就会误判「注册表里没有游戏技能」。
func yyHelpTestIsolate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	yyQQMu.Lock()
	old, oldPath := yyQQ, yyQQPath
	yyQQ = yyQQConfig{}
	yyQQPath = filepath.Join(dir, "qq-bridge.json")
	yyQQMu.Unlock()
	yyGameSkillStore.Lock()
	oldSkillPath := yyGameSkillStore.Path
	oldSkillItems := append([]yyGameSkill(nil), yyGameSkillStore.Items...)
	yyGameSkillStore.Unlock()
	yyGameSkillInit(dir)
	t.Cleanup(func() {
		yyQQMu.Lock()
		yyQQ, yyQQPath = old, oldPath
		yyQQMu.Unlock()
		yyGameSkillStore.Lock()
		yyGameSkillStore.Path = oldSkillPath
		yyGameSkillStore.Items = oldSkillItems
		yyGameSkillStore.Unlock()
	})
}

func yyHelpTestDisk(t *testing.T) yyQQConfig {
	t.Helper()
	raw, e := os.ReadFile(yyQQPath)
	if e != nil {
		t.Fatalf("读回配置失败：%v", e)
	}
	var disk yyQQConfig
	if e := json.Unmarshal(raw, &disk); e != nil {
		t.Fatalf("回读的配置不是合法 JSON：%v", e)
	}
	return disk
}

// 1) 缺省全开：注册表里每一条都能被独立开关控制，且默认 true。
func TestHelpRegistryDefaultsAllOn(t *testing.T) {
	yyHelpTestIsolate(t)
	cf := yyQQSnapshot()
	for _, c := range yyQQCapabilities {
		if !yyQQCapEnabled(cf.Assistant, c.Kind) {
			t.Fatalf("能力 %s 默认应开启（用户要求默认全开）", c.Kind)
		}
	}
	for _, def := range yyQQSkillDefs {
		if !yyQQSkillEnabled(cf, def.ID) {
			t.Fatalf("技能 %s 默认应开启", def.ID)
		}
	}
	for _, gs := range yyGameSkillAll() {
		if !gs.Enabled {
			t.Fatalf("游戏技能 %s 默认应开启", gs.Name)
		}
	}
	// 每条能力/技能/游戏技能都必须有可用的开关 key（前端靠它做逐项开关）。
	entries := yyQQHelpRegistry(cf)
	seen := map[string]bool{}
	for _, e := range entries {
		if e.ToggleKey == "" {
			continue // 固定指令，没有开关（报告里列为写死项）
		}
		seen[e.Source] = true
	}
	for _, src := range []string{"capability", "skill", "game_skill"} {
		if !seen[src] {
			t.Fatalf("帮助注册表里没有 %s 条目，开关接不上", src)
		}
	}
}

// 2) 保存 → 回读 → 生效：关掉「点赞」后，帮助菜单不再出现这一项。
func TestHelpSwitchPersistsAndHidesEntry(t *testing.T) {
	yyHelpTestIsolate(t)
	if e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
		a.Caps = map[string]bool{"like": false}
	}); e != nil {
		t.Fatalf("保存失败：%v", e)
	}
	if disk := yyHelpTestDisk(t); disk.Assistant.Caps["like"] != false {
		t.Fatal("落盘文件里没有 caps.like=false，保存等于没生效")
	}
	if yyQQCapEnabled(yyQQSnapshot().Assistant, "like") {
		t.Fatal("回读后 like 仍被判定为开启")
	}
	text := yyQQHelpText(yyQQSnapshot(), 2)
	if strings.Contains(text, ".点赞") {
		t.Fatalf("关掉的能力仍出现在帮助菜单里：\n%s", text)
	}
	if !strings.Contains(text, "在面板里关着") {
		t.Fatal("帮助菜单应提示还有几项被关掉")
	}
	// 别的条目不受影响。
	if !strings.Contains(text, ".动态") {
		t.Fatal("只关了点赞，发动态不该一起消失")
	}
}

// 3) 权限分层：普通成员看不到超管能力，超管能看到；管理员分组不出现在成员视图。
func TestHelpFiltersByRole(t *testing.T) {
	yyHelpTestIsolate(t)
	cf := yyQQSnapshot()
	for _, e := range yyQQHelpEntries(cf, 0) {
		if e.MinRole > 0 {
			t.Fatalf("成员视图混进了 %s（min_role=%d）", e.Command, e.MinRole)
		}
	}
	member := yyQQHelpText(cf, 0)
	if strings.Contains(member, ".改头像") || strings.Contains(member, "仅超级管理员") {
		t.Fatalf("成员视图不该出现超管内容：\n%s", member)
	}
	root := yyQQHelpText(cf, 2)
	if !strings.Contains(root, ".改头像") || !strings.Contains(root, "仅超级管理员") {
		t.Fatalf("超管视图缺少超管条目：\n%s", root)
	}
	for _, level := range []int{0, 1, 2} {
		if strings.Contains(yyQQHelpText(cf, level), "你的权限：") == false {
			t.Fatalf("level=%d 缺少权限抬头", level)
		}
	}
}

// 4) 触发词 min_role：落盘保留 + 匹配时按身份拦。
func TestTriggerMinRolePersistsAndGates(t *testing.T) {
	yyHelpTestIsolate(t)
	if e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
		a.Triggers = []yyQQTrigger{{
			ID: "t-admin", Enabled: true, Name: "管理员规则", Type: "keyword",
			Patterns: []string{"清屏"}, MinRole: 1,
			// 用未知动作类型做探针：不产生任何真实发送，只验证「是否命中并执行」。
			Actions: []yyQQTriggerAction{{Type: "noop"}},
		}}
	}); e != nil {
		t.Fatalf("保存失败：%v", e)
	}
	disk := yyHelpTestDisk(t)
	if len(disk.Assistant.Triggers) != 1 || disk.Assistant.Triggers[0].MinRole != 1 {
		t.Fatalf("min_role 没落盘：%+v", disk.Assistant.Triggers)
	}
	cf := yyQQSnapshot()
	if yyQQTriggerRun(cf, "路人", "10001", "member", "清屏", "m1", false) {
		t.Fatal("普通成员触发了 min_role=1 的规则")
	}
	if !yyQQTriggerRun(cf, "管理", "10002", "admin", "清屏", "m2", false) {
		t.Fatal("管理员没有被 min_role=1 的规则命中")
	}
	// 缺省 min_role=0 的老规则行为不变：谁都能触发。
	if e := yyQQHelperMutate(func(a *yyQQAssistantConfig) {
		a.Triggers[0].MinRole = 0
	}); e != nil {
		t.Fatalf("保存失败：%v", e)
	}
	if !yyQQTriggerRun(yyQQSnapshot(), "路人", "10001", "member", "清屏", "m3", false) {
		t.Fatal("min_role=0 时成员也应能触发")
	}
}

// 5) 触发词接口：min_role 越界夹紧到 0..2，保存后 GET 能读回。
func TestTriggerRouteClampsMinRole(t *testing.T) {
	yyHelpTestIsolate(t)
	mux := http.NewServeMux()
	yyQQTriggerRoutes(mux)
	body := `{"triggers":[{"id":"a","enabled":true,"name":"规则A","type":"keyword","patterns":["x"],"min_role":9,"actions":[{"type":"reply","params":{"text":"y"}}]}]}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/qq/triggers", strings.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("POST /api/qq/triggers 状态 %d：%s", rec.Code, rec.Body.String())
	}
	if disk := yyHelpTestDisk(t); disk.Assistant.Triggers[0].MinRole != 2 {
		t.Fatalf("min_role=9 应夹到 2，实际 %d", disk.Assistant.Triggers[0].MinRole)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/qq/triggers", nil))
	if !strings.Contains(rec.Body.String(), `"min_role":2`) || !strings.Contains(rec.Body.String(), `"roles"`) {
		t.Fatalf("GET 没回传 min_role/roles：%s", rec.Body.String())
	}
}

// 6) /api/qq/help 走生产注册路径（yyQQCapRoutes）可达，并与 catalog/MCP 登记一致。
func TestHelpRouteRegistered(t *testing.T) {
	yyHelpTestIsolate(t)
	// yyQQCapRoutes 现在会顺带注册知识库路由，进而打开 qq-rag.db。
	// 不关掉这个句柄，TempDir 清理会失败（Windows 上文件仍被占用），
	// 整个包就会因为一个清理错误判 FAIL。
	t.Cleanup(yyRagClose)
	mux := http.NewServeMux()
	yyQQCapRoutes(mux) // 生产接线：yyRegisterQQ → yyQQCapRoutes → yyQQHelpRegister
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/qq/help?level=0", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /api/qq/help 状态 %d：%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OK      bool             `json:"ok"`
		Level   int              `json:"level"`
		Text    string           `json:"text"`
		Entries []yyQQHelpEntry  `json:"entries"`
		Groups  []map[string]any `json:"groups"`
	}
	if e := json.Unmarshal(rec.Body.Bytes(), &resp); e != nil {
		t.Fatalf("返回不是 JSON：%v", e)
	}
	if !resp.OK || resp.Level != 0 || len(resp.Entries) == 0 || len(resp.Groups) == 0 {
		t.Fatalf("返回内容不完整：%+v", resp)
	}
	for _, e := range resp.Entries {
		if e.MinRole > 0 {
			t.Fatalf("level=0 返回了 min_role=%d 的条目 %s", e.MinRole, e.Command)
		}
	}
	// raw=1 要能看到未过滤条目（面板做全量预览用）。
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/qq/help?level=0&raw=1", nil))
	var raw struct {
		Entries []yyQQHelpEntry `json:"entries"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	if len(raw.Entries) <= len(resp.Entries) {
		t.Fatal("raw=1 应返回比过滤后更多的条目")
	}
	// 路由必须同时登记在 catalog 与 MCP 只读白名单里。
	found := false
	for _, r := range yyAPIRoutes {
		if r.Path == "/api/qq/help" && r.Name == "api_qq_help" {
			found = true
		}
	}
	if !found {
		t.Fatal("yyAPIRoutes 里没有 api_qq_help")
	}
	if !yyMCPReadPaths["/api/qq/help"] {
		t.Fatal("yyMCPReadPaths 里没有 /api/qq/help")
	}
	if !yyMCPReadTool("api_qq_help") {
		t.Fatal("MCP 只读连接查不到 api_qq_help")
	}
}

// 7) 上限：技能与触发词都能配到 50 条以上（用户要求放开到四五十个）。
func TestConfigurableLimitsAboveFifty(t *testing.T) {
	if yySkillMaxCount < 50 {
		t.Fatalf("技能上限 %d 小于 50", yySkillMaxCount)
	}
	if yyQQTriggerMax < 50 {
		t.Fatalf("触发词上限 %d 小于 50", yyQQTriggerMax)
	}
	yyHelpTestIsolate(t)
	// 注册表是全局的：测完还原，别污染同进程里的别的用例。
	yyGameSkillStore.Lock()
	oldItems := append([]yyGameSkill(nil), yyGameSkillStore.Items...)
	yyGameSkillStore.Unlock()
	t.Cleanup(func() {
		yyGameSkillStore.Lock()
		yyGameSkillStore.Items = oldItems
		yyGameSkillStore.Unlock()
	})
	entries := make([]yySkillEntry, 0, 60)
	for i := 0; i < 60; i++ {
		entries = append(entries, yySkillEntry{Skill: yyGameSkill{
			Name: "custom_" + string(rune('a'+i%26)) + string(rune('a'+i/26)),
			Label: "自定义技能", Run: "console", Command: "/say hi", Channel: "ai", Format: "text", TimeoutMS: 3000,
		}})
	}
	for _, b := range yyGameSkillBuiltins() {
		entries = append(entries, yySkillEntry{Skill: b})
	}
	if e := yyGameSkillSaveAll(entries); e != nil {
		t.Fatalf("保存 60+ 个技能被拒：%v", e)
	}
	if n := len(yyGameSkillAll()); n < 60 {
		t.Fatalf("只存下 %d 个技能", n)
	}
}
