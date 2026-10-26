package main

// 群规模块测试：违禁词拦截（硬通道）+ 自然语言禁令（双通道）。
// 范式照 youyuan_qq_guard_test.go：全局状态快照 + Cleanup 还原，
// 配置路径用 t.TempDir() 重定向，绝不碰真实数据目录。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ---- 全局状态保存/还原（结构体带 mutex，不能整体拷贝，逐字段搬）----

func yyModTestSave() (string, []yyQQModerationRule, string, map[string]*regexp.Regexp, map[string]time.Time) {
	yyQQModeration.Lock()
	defer yyQQModeration.Unlock()
	return yyQQModeration.Path,
		append([]yyQQModerationRule(nil), yyQQModeration.Rules...),
		yyQQModeration.DiskError, yyQQModeration.reCache, yyQQModeration.notifyAt
}

func yyModTestRestore(path string, rules []yyQQModerationRule, diskErr string, reCache map[string]*regexp.Regexp, notifyAt map[string]time.Time) {
	yyQQModeration.Lock()
	defer yyQQModeration.Unlock()
	yyQQModeration.Path = path
	yyQQModeration.Rules = rules
	yyQQModeration.DiskError = diskErr
	yyQQModeration.reCache = reCache
	yyQQModeration.notifyAt = notifyAt
}

func yyPersonaTestSave() (string, yyQQPersona, string) {
	yyQQPersonaState.Lock()
	defer yyQQPersonaState.Unlock()
	return yyQQPersonaState.Path, yyQQPersonaState.Persona, yyQQPersonaState.DiskError
}

func yyPersonaTestRestore(path string, persona yyQQPersona, diskErr string) {
	yyQQPersonaState.Lock()
	defer yyQQPersonaState.Unlock()
	yyQQPersonaState.Path = path
	yyQQPersonaState.Persona = persona
	yyQQPersonaState.DiskError = diskErr
}

// yyModTestSetup 把 moderation 与 persona 的落盘路径指到临时目录。
func yyModTestSetup(t *testing.T) string {
	t.Helper()
	modPath, modRules, modErr, modRe, modNotify := yyModTestSave()
	perPath, perPersona, perErr := yyPersonaTestSave()
	t.Cleanup(func() {
		yyModTestRestore(modPath, modRules, modErr, modRe, modNotify)
		yyPersonaTestRestore(perPath, perPersona, perErr)
	})
	root := t.TempDir()
	yyQQModerationInit(root)
	yyQQPersonaInit(root)
	return root
}

func yyModTestCF(superAdmins ...string) yyQQConfig {
	return yyQQConfig{Adapter: "onebot", Assistant: yyQQAssistantConfig{SuperAdmins: superAdmins}}
}

// ---- 匹配与规范化 ----

func TestModerationEmptyRulesDoNotBlock(t *testing.T) {
	yyModTestSetup(t)
	cf := yyModTestCF("111")
	if _, hit := yyQQModerationBlockHit(cf, "随便说点什么", "222"); hit {
		t.Fatal("空规则不该拦任何消息")
	}
}

func TestModerationNormalizationVariants(t *testing.T) {
	yyModTestSetup(t)
	if _, e := yyQQModerationReplace([]yyQQModerationRule{{Pattern: "SB", Kind: "substring", Action: "block", Enabled: true}}); e != nil {
		t.Fatal(e)
	}
	cf := yyModTestCF("111")
	// 小写、全角、零宽插入、大小写混合都要命中同一条规则。
	for _, variant := range []string{"sb", "ＳＢ", "s\u200bb", "Sb", " 你在说 sb 吧 "} {
		if _, hit := yyQQModerationBlockHit(cf, "测试"+variant+"测试", "222"); !hit {
			t.Fatalf("变体 %q 应命中规则", variant)
		}
	}
}

func TestModerationAdminsExempt(t *testing.T) {
	yyModTestSetup(t)
	if _, e := yyQQModerationReplace([]yyQQModerationRule{{Pattern: "打架", Kind: "substring", Action: "block", Enabled: true}}); e != nil {
		t.Fatal(e)
	}
	cf := yyModTestCF("111", "222")
	if _, hit := yyQQModerationBlockHit(cf, "来打架啊", "111"); hit {
		t.Fatal("超管不该被群规拦")
	}
	if _, hit := yyQQModerationBlockHit(cf, "来打架啊", "222"); hit {
		t.Fatal("管理员不该被群规拦")
	}
	if _, hit := yyQQModerationBlockHit(cf, "来打架啊", "333"); !hit {
		t.Fatal("普通群友应被拦")
	}
}

func TestModerationWarnDoesNotBlock(t *testing.T) {
	yyModTestSetup(t)
	if _, e := yyQQModerationReplace([]yyQQModerationRule{{Pattern: "菜鸡", Kind: "substring", Action: "warn", Enabled: true}}); e != nil {
		t.Fatal(e)
	}
	cf := yyModTestCF("111")
	if _, hit := yyQQModerationBlockHit(cf, "你真菜鸡", "222"); hit {
		t.Fatal("warn 级规则不该拦消息，只进 AI 提示词")
	}
	block := yyQQModerationPromptBlock()
	if !strings.Contains(block, "菜鸡") {
		t.Fatalf("warn 规则应出现在提示词块里：%q", block)
	}
}

func TestModerationRegexRule(t *testing.T) {
	yyModTestSetup(t)
	if _, e := yyQQModerationReplace([]yyQQModerationRule{{Pattern: `加\s*V\s*信`, Kind: "regex", Action: "block", Enabled: true}}); e != nil {
		t.Fatal(e)
	}
	cf := yyModTestCF("111")
	if _, hit := yyQQModerationBlockHit(cf, "有事加 V 信聊", "222"); !hit {
		t.Fatal("regex 规则应命中")
	}
	// 编译不过的正则在 Normalize 阶段就被丢弃，不会进来。
	rules, _ := yyQQModerationSnapshot()
	for _, r := range rules {
		if r.Kind == "regex" {
			if _, e := regexp.Compile(r.Pattern); e != nil {
				t.Fatalf("非法正则不该被保存：%q", r.Pattern)
			}
		}
	}
}

func TestModerationNormalizeCleans(t *testing.T) {
	yyModTestSetup(t)
	long := strings.Repeat("废", 80)
	rules, e := yyQQModerationReplace([]yyQQModerationRule{
		{Pattern: "垃圾", Kind: "substring", Action: "block"},
		{Pattern: "垃圾", Kind: "substring", Action: "block"}, // 重复
		{Pattern: "", Kind: "substring", Action: "block"},   // 空
		{Pattern: long, Kind: "substring", Action: "block"}, // 超长
		{Pattern: "[", Kind: "regex", Action: "block"},      // 非法正则
		{Pattern: "广告", Kind: "weird", Action: "hack"},      // 非法 kind/action → 兜底
	})
	if e != nil {
		t.Fatal(e)
	}
	if len(rules) != 2 {
		t.Fatalf("应只剩 2 条合法规则，实际 %d：%+v", len(rules), rules)
	}
	if rules[1].Kind != "substring" || rules[1].Action != "block" {
		t.Fatalf("非法 kind/action 应兜底为 substring/block：%+v", rules[1])
	}
}

// ---- 自然语言禁令（双通道）----

func TestModerationNaturalBanDualChannel(t *testing.T) {
	root := yyModTestSetup(t)
	cf := yyModTestCF("111")
	if !yyQQNaturalBan(cf, "111", "群主", "以后不准 玩家打架", "msg1") {
		t.Fatal("超管禁令句式应被消费")
	}
	// 硬通道：substring 规则落盘
	rules, _ := yyQQModerationSnapshot()
	if len(rules) != 1 || rules[0].Pattern != "打架" || rules[0].Source != "natural" {
		t.Fatalf("自然语言禁令应生成 substring 规则：%+v", rules)
	}
	// 软通道：Taboos 落盘
	taboos := yyQQPersonaTaboos()
	found := false
	for _, v := range taboos {
		if strings.Contains(v, "打架") {
			found = true
		}
	}
	if !found {
		t.Fatalf("自然语言禁令应写进人格禁忌：%v", taboos)
	}
	// 重启（重新 Init）后双通道都还在
	yyQQModerationInit(root)
	yyQQPersonaInit(root)
	rules2, _ := yyQQModerationSnapshot()
	if len(rules2) != 1 || rules2[0].Pattern != "打架" {
		t.Fatalf("重启后群规应保留：%+v", rules2)
	}
	taboos2 := yyQQPersonaTaboos()
	found = false
	for _, v := range taboos2 {
		if strings.Contains(v, "打架") {
			found = true
		}
	}
	if !found {
		t.Fatalf("重启后人格禁忌应保留：%v", taboos2)
	}
	// 玩家再发「打架」被硬通道拦
	if _, hit := yyQQModerationBlockHit(cf, "大家来打架", "222"); !hit {
		t.Fatal("禁令生效后普通群友应被拦")
	}
}

func TestModerationNaturalBanNotForNonSuper(t *testing.T) {
	yyModTestSetup(t)
	cf := yyModTestCF("111")
	if yyQQNaturalBan(cf, "222", "群友", "以后不准 玩家打架", "msg1") {
		t.Fatal("非超管的禁令句式不该被当指令消费")
	}
	rules, _ := yyQQModerationSnapshot()
	if len(rules) != 0 {
		t.Fatalf("非超管不该能改群规：%+v", rules)
	}
}

func TestModerationNaturalUnban(t *testing.T) {
	yyModTestSetup(t)
	cf := yyModTestCF("111")
	yyQQNaturalBan(cf, "111", "群主", "以后不准 玩家骂人", "msg1")
	if _, hit := yyQQModerationBlockHit(cf, "你别骂人", "222"); !hit {
		t.Fatal("禁令先生效")
	}
	if !yyQQNaturalBan(cf, "111", "群主", "取消禁令 骂人", "msg2") {
		t.Fatal("解除句式应被消费")
	}
	if _, hit := yyQQModerationBlockHit(cf, "你别骂人", "222"); hit {
		t.Fatal("解除后不该再拦")
	}
	for _, v := range yyQQPersonaTaboos() {
		if strings.Contains(v, "骂人") {
			t.Fatalf("解除后人格禁忌不该再含「骂人」：%v", yyQQPersonaTaboos())
		}
	}
}

func TestModerationNaturalBanPersonalNotRule(t *testing.T) {
	yyModTestSetup(t)
	cf := yyModTestCF("111")
	// 超管说自己的事，不是群规——不该被消费，更不该生成拦截规则。
	for _, sentence := range []string{"别让我失望", "不准我说话", "以后别让我操心"} {
		if yyQQNaturalBan(cf, "111", "群主", sentence, "msg1") {
			t.Fatalf("句式 %q 是超管说自己的事，不该被当禁令消费", sentence)
		}
	}
	rules, _ := yyQQModerationSnapshot()
	if len(rules) != 0 {
		t.Fatalf("不该生成任何群规：%+v", rules)
	}
}

func TestModerationBanExtract(t *testing.T) {
	cases := []struct{ in, want string }{
		{"玩家打架", "打架"},
		{"玩家们在群里骂人", "骂人"},
		{"发广告", "发广告"},
		{"说「abc」", "abc"},
		{"刷屏", "刷屏"},
		{"再提这件事", "提这件事"},
		{"玩家", ""}, // 剥完主体词没内容
		{"的", ""},  // 太短
	}
	for _, c := range cases {
		if got := yyQQBanExtract(c.in); got != c.want {
			t.Fatalf("yyQQBanExtract(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

func TestModerationBanPrefixVariants(t *testing.T) {
	yyModTestSetup(t)
	cf := yyModTestCF("111")
	for _, sentence := range []string{"以后不准玩家骂人", "记住：不准玩家骂人", "禁止玩家骂人", "别让玩家骂人"} {
		yyModTestSetup(t) // 每个句式清空重来
		if !yyQQNaturalBan(cf, "111", "群主", sentence, "msg1") {
			t.Fatalf("句式 %q 应被识别为禁令", sentence)
		}
		if _, hit := yyQQModerationBlockHit(cf, "你又骂人", "222"); !hit {
			t.Fatalf("句式 %q 应生成拦截规则", sentence)
		}
	}
}

// ---- 降级与路由 ----

func TestModerationDegradeWithoutCaps(t *testing.T) {
	cf := yyModTestCF("111")
	// 玩家在面板关掉能力 → 不撤回不禁言（消息仍会被 BlockHit 拦下）。
	// 注意：ban 自「AI 技能」需求起默认开启，这里显式关掉来验证降级路径。
	cf.Assistant.Caps = map[string]bool{"recall": false, "ban": false}
	if yyQQModerationCanRecall(cf, "recall_ban") {
		t.Fatal("recall 能力被关掉，不该执行撤回")
	}
	if yyQQModerationCanBan(cf, "recall_ban") {
		t.Fatal("ban 能力被关掉，不该执行禁言")
	}
	// 官方适配器 → 同样降级
	official := yyModTestCF("111")
	official.Adapter = "qqofficial"
	official.Assistant.Caps = map[string]bool{"recall": true, "ban": true}
	if yyQQModerationCanRecall(official, "recall") {
		t.Fatal("官方适配器没有 delete_msg，不该尝试撤回")
	}
	// 开关打开 + onebot → 才真的执行
	on := yyModTestCF("111")
	on.Assistant.Caps = map[string]bool{"recall": true, "ban": true}
	if !yyQQModerationCanRecall(on, "recall") || !yyQQModerationCanBan(on, "recall_ban") {
		t.Fatal("开关打开且 onebot 时应执行撤回/禁言")
	}
	// block 级永远不撤回
	if yyQQModerationCanRecall(on, "block") {
		t.Fatal("block 级不该触发撤回")
	}
}

// ---- 面板增删改（用户原话：「有些东西还不能删掉 比如违规词 不能修改」）----

// 编辑后立刻生效：同 pattern 改 action/reason，不重启、不 reload，
// 匹配行为与 GET 读回都必须马上变。
func TestModerationEditRuleTakesEffect(t *testing.T) {
	yyModTestSetup(t)
	cf := yyModTestCF("111")
	if _, e := yyQQModerationReplace([]yyQQModerationRule{
		{Pattern: "打架", Kind: "substring", Action: "block", Reason: "旧理由", Enabled: true},
	}); e != nil {
		t.Fatal(e)
	}
	if _, hit := yyQQModerationBlockHit(cf, "大家来打架", "222"); !hit {
		t.Fatal("block 级规则应拦截")
	}
	// 面板「改」：整表回传，同 pattern 换成 warn + 新理由（前端保留 enabled/source）
	if _, e := yyQQModerationReplace([]yyQQModerationRule{
		{Pattern: "打架", Kind: "substring", Action: "warn", Reason: "新理由", Enabled: true},
	}); e != nil {
		t.Fatal(e)
	}
	if _, hit := yyQQModerationBlockHit(cf, "大家来打架", "222"); hit {
		t.Fatal("改成 warn 后不该再拦消息")
	}
	if block := yyQQModerationPromptBlock(); !strings.Contains(block, "新理由") {
		t.Fatalf("warn 规则应带新理由进提示词块：%q", block)
	}
	rules, _ := yyQQModerationSnapshot()
	if len(rules) != 1 || rules[0].Action != "warn" || rules[0].Reason != "新理由" {
		t.Fatalf("GET 读回应马上反映编辑：%+v", rules)
	}
}

// 删除后不存在：DELETE 端点与面板的整表回传两条路都要通，
// 删完立即不再拦截，GET 也不再返回。
func TestModerationDeleteRuleGone(t *testing.T) {
	yyModTestSetup(t)
	cf := yyModTestCF("111")
	if _, e := yyQQModerationReplace([]yyQQModerationRule{
		{Pattern: "打架", Kind: "substring", Action: "block", Enabled: true},
		{Pattern: "骂人", Kind: "substring", Action: "block", Enabled: true},
	}); e != nil {
		t.Fatal(e)
	}
	if !yyQQModerationRemove("打架", "substring") {
		t.Fatal("删除应成功")
	}
	if yyQQModerationRemove("打架", "substring") {
		t.Fatal("重复删除应返回 false")
	}
	rules, _ := yyQQModerationSnapshot()
	if len(rules) != 1 || rules[0].Pattern != "骂人" {
		t.Fatalf("删除后应只剩「骂人」：%+v", rules)
	}
	if _, hit := yyQQModerationBlockHit(cf, "大家来打架", "222"); hit {
		t.Fatal("删掉的规则不该再拦")
	}
	if _, hit := yyQQModerationBlockHit(cf, "你别骂人", "222"); !hit {
		t.Fatal("没删的规则还应生效")
	}
	// 面板删除走整表回传：再删「骂人」
	if _, e := yyQQModerationReplace([]yyQQModerationRule{}); e != nil {
		t.Fatal(e)
	}
	rules, _ = yyQQModerationSnapshot()
	if len(rules) != 0 {
		t.Fatalf("整表回传空列表应清空：%+v", rules)
	}
}

// AI 长期禁令（taboos）面板增改删：走 POST {taboos:[...]} 整表替换，
// 空词条与重复词条后端也要洗干净。
func TestModerationTaboosCRUD(t *testing.T) {
	yyModTestSetup(t)
	got := yyQQPersonaSetTaboos([]string{"不准打架", "", "不准打架", "不准骂人"})
	if len(got) != 2 || got[0] != "不准打架" || got[1] != "不准骂人" {
		t.Fatalf("空词条应丢弃、重复只留一条：%v", got)
	}
	// 改：第二条换成新内容
	got = yyQQPersonaSetTaboos([]string{"不准打架", "不准刷屏"})
	if len(got) != 2 || got[1] != "不准刷屏" {
		t.Fatalf("编辑后应生效：%v", got)
	}
	// 删：去掉第一条
	got = yyQQPersonaSetTaboos([]string{"不准刷屏"})
	if len(got) != 1 || got[0] != "不准刷屏" {
		t.Fatalf("删除后应只剩一条：%v", got)
	}
	// 重启后仍在（落盘 qq-persona.json）
	taboos := yyQQPersonaTaboos()
	if len(taboos) != 1 || taboos[0] != "不准刷屏" {
		t.Fatalf("重启后禁令应保留：%v", taboos)
	}
}

// 只保存长期禁令不能把群规规则表清空（rules/taboos 指针语义回归）。
func TestModerationPostTaboosOnlyKeepsRules(t *testing.T) {
	yyModTestSetup(t)
	if _, e := yyQQModerationReplace([]yyQQModerationRule{
		{Pattern: "打架", Kind: "substring", Action: "block", Enabled: true},
	}); e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	yyQQModerationRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, e := http.Post(srv.URL+"/api/qq/moderation", "application/json",
		strings.NewReader(`{"taboos":["不准刷屏"]}`))
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	rules, _ := yyQQModerationSnapshot()
	if len(rules) != 1 || rules[0].Pattern != "打架" {
		t.Fatalf("只存 taboos 不该动规则表：%+v", rules)
	}
	taboos := yyQQPersonaTaboos()
	if len(taboos) != 1 || taboos[0] != "不准刷屏" {
		t.Fatalf("taboos 应已保存：%v", taboos)
	}
}

func TestModerationRoutes(t *testing.T) {
	yyModTestSetup(t)
	mux := http.NewServeMux()
	yyQQModerationRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// GET 初始为空
	resp, e := http.Get(srv.URL + "/api/qq/moderation")
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Ok    bool                 `json:"ok"`
		Rules []yyQQModerationRule `json:"rules"`
	}
	if e := json.NewDecoder(resp.Body).Decode(&got); e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if !got.Ok || len(got.Rules) != 0 {
		t.Fatalf("初始应为空规则：%+v", got)
	}

	// POST 保存
	body := `{"rules":[{"pattern":"广告","kind":"substring","action":"recall","reason":"不发广告","enabled":true}]}`
	resp, e = http.Post(srv.URL+"/api/qq/moderation", "application/json", strings.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	rules, _ := yyQQModerationSnapshot()
	if len(rules) != 1 || rules[0].Pattern != "广告" || rules[0].Action != "recall" {
		t.Fatalf("POST 后应保存 1 条规则：%+v", rules)
	}

	// DELETE 删除
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/qq/moderation?pattern=广告&kind=substring", nil)
	resp, e = http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	rules, _ = yyQQModerationSnapshot()
	if len(rules) != 0 {
		t.Fatalf("DELETE 后应为空：%+v", rules)
	}
}
