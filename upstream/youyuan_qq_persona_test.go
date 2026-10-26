package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 人格/群风格测试fixture：只替换包级私有状态并用临时目录落盘，
// 不发送任何QQ消息，不联系真实模型。
func yyQQPersonaFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	yyQQPersonaState.Lock()
	oldPath, oldPersona, oldStyle, oldErr := yyQQPersonaState.Path, yyQQPersonaState.Persona, yyQQPersonaState.Style, yyQQPersonaState.DiskError
	yyQQPersonaState.Unlock()
	yyQQStyleState.Lock()
	oldGroups, oldStylePath, oldStyleErr := yyQQStyleState.Groups, yyQQStyleState.Path, yyQQStyleState.DiskError
	yyQQStyleState.Unlock()
	t.Cleanup(func() {
		yyQQPersonaState.Lock()
		yyQQPersonaState.Path, yyQQPersonaState.Persona, yyQQPersonaState.Style, yyQQPersonaState.DiskError = oldPath, oldPersona, oldStyle, oldErr
		yyQQPersonaState.Unlock()
		yyQQStyleState.Lock()
		yyQQStyleState.Groups, yyQQStyleState.Path, yyQQStyleState.DiskError = oldGroups, oldStylePath, oldStyleErr
		yyQQStyleState.Unlock()
	})
	yyQQPersonaState.Lock()
	yyQQPersonaState.Path = ""
	yyQQPersonaState.DiskError = ""
	yyQQPersonaState.Persona = yyQQPersonaDefault()
	yyQQPersonaState.Style = yyStyleConfigDefault()
	yyQQPersonaState.Unlock()
	yyQQStyleState.Lock()
	yyQQStyleState.Path = ""
	yyQQStyleState.DiskError = ""
	yyQQStyleState.Groups = map[string][]yyStyleSample{}
	yyQQStyleState.Unlock()
	return dir
}

func TestYYPersonaPromptBlockHasAllDefaultRules(t *testing.T) {
	yyQQPersonaFixture(t)
	block := yyPersonaPromptBlock()
	if block == "" {
		t.Fatal("默认人格应产出提示词块")
	}
	for _, rule := range yyQQPersonaAntiAIDefault() {
		if !strings.Contains(block, rule) {
			t.Fatalf("提示词块缺少反AI规则：%s", rule)
		}
	}
	if !strings.Contains(block, "老白") {
		t.Fatal("提示词块缺少人格名称")
	}
	if n := len([]rune(block)); n > yyQQPersonaBlockMax {
		t.Fatalf("提示词块超长：%d", n)
	}
	if strings.Contains(block, "```") || strings.Contains(block, "**") {
		t.Fatal("提示词块不应带 markdown 围栏或加粗")
	}
}

func TestYYPersonaPromptBlockTruncate(t *testing.T) {
	yyQQPersonaFixture(t)
	yyQQPersonaState.Lock()
	yyQQPersonaState.Persona.Persona = strings.Repeat("人设描述", 400)
	yyQQPersonaState.Unlock()
	block := yyPersonaPromptBlock()
	runes := []rune(block)
	if len(runes) != yyQQPersonaBlockMax {
		t.Fatalf("截断后总长应为 %d，实际 %d", yyQQPersonaBlockMax, len(runes))
	}
	if !strings.HasSuffix(block, "字）") {
		t.Fatalf("末尾应标注省略字数：%q", block[len(block)-24:])
	}
	idx := strings.Index(block, "（略 ")
	if idx < 0 {
		t.Fatalf("缺少省略标注：%q", block[len(block)-24:])
	}
	end := strings.Index(block[idx:], " 字）")
	dropped, e := strconv.Atoi(block[idx+len("（略 ") : idx+end])
	if e != nil || dropped < 1000 {
		t.Fatalf("省略字数应大于1000，实际 %d（%v）", dropped, e)
	}
}

func TestYYPersonaPromptBlockEmptyWhenDisabledOrBlank(t *testing.T) {
	yyQQPersonaFixture(t)
	yyQQPersonaState.Lock()
	yyQQPersonaState.Persona.Enabled = false
	yyQQPersonaState.Unlock()
	if got := yyPersonaPromptBlock(); got != "" {
		t.Fatalf("关闭后应返回空，实际 %q", got)
	}
	yyQQPersonaState.Lock()
	yyQQPersonaState.Persona = yyQQPersona{Enabled: true}
	yyQQPersonaState.Unlock()
	if got := yyPersonaPromptBlock(); got != "" {
		t.Fatalf("空人格应返回空，实际 %q", got)
	}
}

func TestYYStyleLearnFromSyntheticMessages(t *testing.T) {
	dir := yyQQPersonaFixture(t)
	yyQQPersonaInit(dir)
	group := "group-style-1"
	for i := 0; i < 15; i++ {
		yyStyleObserve(group, "绷不住了[表情]")
	}
	for i := 0; i < 5; i++ {
		yyStyleObserve(group, "绷不住了[CQ:face,id=7]")
	}
	for i := 0; i < 5; i++ {
		yyStyleObserve(group, "大家好好学习")
	}
	stat := yyStyleLearn(group)
	if stat.Samples != 25 {
		t.Fatalf("样本数应为25，实际 %d", stat.Samples)
	}
	if stat.FaceRatio < 0.75 || stat.FaceRatio > 0.85 {
		t.Fatalf("表情频率应约0.8，实际 %v", stat.FaceRatio)
	}
	if stat.AvgRunes >= 12 {
		t.Fatalf("平均长度应偏短，实际 %v", stat.AvgRunes)
	}
	if len(stat.TopWords) == 0 || stat.TopWords[0].Text != "绷不住了" || stat.TopWords[0].N != 20 {
		t.Fatalf("最高频词应为「绷不住了」20次，实际 %+v", stat.TopWords)
	}
	total := 0
	for _, n := range stat.Hours {
		total += n
	}
	if total != 25 {
		t.Fatalf("时段直方图总数应为25，实际 %d", total)
	}
	summary := yyStyleSummary(group)
	for _, want := range []string{"本群习惯", "消息偏短", "表情多", "绷不住了"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("摘要缺少 %s：%s", want, summary)
		}
	}
}

func TestYYStyleHasFaceMarkers(t *testing.T) {
	for _, text := range []string{"[CQ:face,id=123]", "[CQ:image,file=xxx]", "[Face]", "[表情]", "[图片]"} {
		if !yyStyleHasFace(text) {
			t.Fatalf("应识别表情/图片标记：%s", text)
		}
	}
	for _, text := range []string{"纯文本消息", "face", "图片呢", "12345"} {
		if yyStyleHasFace(text) {
			t.Fatalf("不应误判为表情：%s", text)
		}
	}
}

func TestYYStyleSummaryActiveHoursFixedClock(t *testing.T) {
	dir := yyQQPersonaFixture(t)
	yyQQPersonaInit(dir)
	group := "group-hours"
	base := time.Date(2026, 10, 22, 21, 0, 0, 0, time.Local)
	yyQQStyleState.Lock()
	rows := make([]yyStyleSample, 0, 30)
	for i := 0; i < 30; i++ {
		rows = append(rows, yyStyleSample{Text: "测试消息" + strconv.Itoa(i%3), At: base.Add(time.Duration(i) * time.Minute)})
	}
	yyQQStyleState.Groups[group] = rows
	yyQQStyleState.Unlock()
	stat := yyStyleLearn(group)
	if stat.PeakHours != [2]int{21, 21} {
		t.Fatalf("峰值时段应为21-21，实际 %v", stat.PeakHours)
	}
	summary := yyStyleSummary(group)
	if !strings.Contains(summary, "21点最活跃") {
		t.Fatalf("摘要应含21点最活跃：%s", summary)
	}
	if n := len([]rune(summary)); n > yyQQPersonaSummary {
		t.Fatalf("摘要超长：%d", n)
	}
}

func TestYYPersonaPersistenceRoundTrip(t *testing.T) {
	dir := yyQQPersonaFixture(t)
	yyQQPersonaInit(dir)
	mux := http.NewServeMux()
	yyQQPersonaRoutes(mux)
	body := `{"persona":{"name":"阿青","persona":"热心但嘴笨的萌新","style":"短句带口癖","taboos":["自称管理","骂人"],"anti_ai":["不说开场白","不用markdown"],"enabled":true},"style":{"enabled":false,"min_msgs":7,"top_n":4}}`
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/qq/persona", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("POST 应成功，实际 %d %s", rr.Code, rr.Body.String())
	}
	yyQQPersonaInit(dir)
	yyQQPersonaState.Lock()
	got, gotStyle := yyQQPersonaState.Persona, yyQQPersonaState.Style
	yyQQPersonaState.Unlock()
	if got.Name != "阿青" || got.Persona != "热心但嘴笨的萌新" || got.Style != "短句带口癖" || len(got.AntiAI) != 2 || len(got.Taboos) != 2 || !got.Enabled {
		t.Fatalf("人格 round-trip 失败：%+v", got)
	}
	if gotStyle.Enabled || gotStyle.MinMsgs != 7 || gotStyle.TopN != 4 {
		t.Fatalf("风格配置 round-trip 失败：%+v", gotStyle)
	}
}

func TestYYPersonaCorruptJSONFallsBackToDefault(t *testing.T) {
	dir := yyQQPersonaFixture(t)
	if e := os.WriteFile(filepath.Join(dir, "qq-persona.json"), []byte("{ 这不是合法JSON"), 0600); e != nil {
		t.Fatal(e)
	}
	yyQQPersonaInit(dir)
	yyQQPersonaState.Lock()
	got, gotStyle, diskErr := yyQQPersonaState.Persona, yyQQPersonaState.Style, yyQQPersonaState.DiskError
	yyQQPersonaState.Unlock()
	if diskErr != "invalid_persona_config" {
		t.Fatalf("应记录损坏错误，实际 %q", diskErr)
	}
	want := yyQQPersonaDefault()
	if got.Name != want.Name || len(got.AntiAI) != len(want.AntiAI) || !got.Enabled {
		t.Fatalf("损坏后应回退默认人格：%+v", got)
	}
	if gotStyle != yyStyleConfigDefault() {
		t.Fatalf("损坏后应回退默认风格配置：%+v", gotStyle)
	}
}

func TestYYPersonaRoutesValidationAndMethod(t *testing.T) {
	dir := yyQQPersonaFixture(t)
	yyQQPersonaInit(dir)
	mux := http.NewServeMux()
	yyQQPersonaRoutes(mux)
	cases := []struct{ name, body, want string }{
		{"空名称", `{"persona":{"name":"   ","anti_ai":["x"]}}`, "人格名称不能为空"},
		{"无反AI规则", `{"persona":{"name":"小白","anti_ai":[]}}`, "反AI腔规则至少保留1条"},
		{"非法JSON", `{oops`, "请求不是合法JSON"},
		{"空payload", `{}`, "没有要保存的内容"},
	}
	for _, c := range cases {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/qq/persona", strings.NewReader(c.body)))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s：应400，实际 %d", c.name, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), c.want) {
			t.Fatalf("%s：错误信息应含 %s，实际 %s", c.name, c.want, rr.Body.String())
		}
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/qq/persona", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT 应405，实际 %d", rr.Code)
	}
	// 校验失败不应改动现有配置
	yyQQPersonaState.Lock()
	name := yyQQPersonaState.Persona.Name
	yyQQPersonaState.Unlock()
	if name != "老白" {
		t.Fatalf("校验失败不应改动配置，实际 %q", name)
	}
}

func TestYYPersonaRoutesGet(t *testing.T) {
	dir := yyQQPersonaFixture(t)
	yyQQPersonaInit(dir)
	group := "group-get"
	for i := 0; i < 20; i++ {
		yyStyleObserve(group, "哈哈哈[CQ:face,id=1]")
	}
	mux := http.NewServeMux()
	yyQQPersonaRoutes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/qq/persona", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET 应200，实际 %d", rr.Code)
	}
	var resp struct {
		OK      bool              `json:"ok"`
		Persona yyQQPersona       `json:"persona"`
		Style   yyStyleConfig     `json:"style"`
		Groups  map[string]string `json:"groups"`
	}
	if e := json.Unmarshal(rr.Body.Bytes(), &resp); e != nil {
		t.Fatal(e)
	}
	if !resp.OK || resp.Persona.Name != "老白" || resp.Persona.Name == "" {
		t.Fatalf("GET 响应异常：%s", rr.Body.String())
	}
	if resp.Groups[group] == "" {
		t.Fatalf("GET 应含群 %s 的风格摘要：%s", group, rr.Body.String())
	}
}

// 用户原话：「我群服互通里面的提示词，他好像不会保存吧，我保存了之后，后面我返回
// 看了一下，唉，提示词没了」。提示词走 POST /api/qq/config 的 assistant.prompt，
// 面板只在用户真改过时才提交该字段——没改过就整段不发，靠后端 yyMergePatch 深合并
// 保留原值。这里锁定该契约：只打 assistant.prompt 补丁不能动 assistant 其他字段，
// 否则另一个面板拿旧快照整段回传就会把刚存好的提示词清空。
func TestYYConfigPromptPatchKeepsAssistantSiblings(t *testing.T) {
	current := map[string]any{"assistant": map[string]any{
		"prompt": "旧提示词", "model": "deepseek-chat", "mcp_url": "https://example.com/mcp",
	}}
	yyMergePatch(current, map[string]any{"assistant": map[string]any{"prompt": "新提示词"}})
	raw, _ := json.Marshal(current)
	var next yyQQConfig
	if e := json.Unmarshal(raw, &next); e != nil {
		t.Fatal(e)
	}
	if next.Assistant.Prompt != "新提示词" {
		t.Fatalf("prompt 应被更新，实际 %q", next.Assistant.Prompt)
	}
	if next.Assistant.Model != "deepseek-chat" {
		t.Fatalf("只改 prompt 不该清掉 model，实际 %q", next.Assistant.Model)
	}
	if next.Assistant.MCPURL != "https://example.com/mcp" {
		t.Fatalf("只改 prompt 不该清掉 mcp_url，实际 %q", next.Assistant.MCPURL)
	}
	// 空串补丁是显式清空（用户真的删空了提示词），必须生效
	yyMergePatch(current, map[string]any{"assistant": map[string]any{"prompt": ""}})
	raw, _ = json.Marshal(current)
	if e := json.Unmarshal(raw, &next); e != nil {
		t.Fatal(e)
	}
	if next.Assistant.Prompt != "" || next.Assistant.Model != "deepseek-chat" {
		t.Fatalf("显式清空 prompt 应生效且不碰其他字段：%+v", next.Assistant)
	}
}

// 人格卡保存后读回：POST /api/qq/persona 再 GET，值必须一致
//（保存→刷新→再看值还在的接口侧证据）。
func TestYYPersonaHTTPRoundTrip(t *testing.T) {
	dir := yyQQPersonaFixture(t)
	yyQQPersonaInit(dir)
	mux := http.NewServeMux()
	yyQQPersonaRoutes(mux)
	body := `{"persona":{"name":"阿青","persona":"热心但嘴笨的萌新","style":"短句带口癖","taboos":["自称管理","骂人"],"anti_ai":["不说开场白","不用markdown"],"enabled":true},"style":{"enabled":true,"min_msgs":30,"top_n":5}}`
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/qq/persona", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("POST 应成功，实际 %d %s", rr.Code, rr.Body.String())
	}
	// 模拟刷新：重新 Init 读盘 + GET 读回
	yyQQPersonaInit(dir)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/qq/persona", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET 应200，实际 %d", rr.Code)
	}
	var resp struct {
		OK      bool              `json:"ok"`
		Persona yyQQPersona       `json:"persona"`
		Style   yyStyleConfig     `json:"style"`
		Groups  map[string]string `json:"groups"`
	}
	if e := json.Unmarshal(rr.Body.Bytes(), &resp); e != nil {
		t.Fatal(e)
	}
	if !resp.OK {
		t.Fatalf("GET 应 ok：%s", rr.Body.String())
	}
	if resp.Persona.Name != "阿青" || resp.Persona.Persona != "热心但嘴笨的萌新" ||
		resp.Persona.Style != "短句带口癖" || len(resp.Persona.AntiAI) != 2 ||
		len(resp.Persona.Taboos) != 2 || !resp.Persona.Enabled {
		t.Fatalf("读回的人格与保存不一致：%+v", resp.Persona)
	}
	if resp.Style.MinMsgs != 30 || resp.Style.TopN != 5 || !resp.Style.Enabled {
		t.Fatalf("读回的风格配置与保存不一致：%+v", resp.Style)
	}
}

func TestYYStyleToggleAndRingBuffer(t *testing.T) {
	dir := yyQQPersonaFixture(t)
	yyQQPersonaInit(dir)
	yyQQPersonaState.Lock()
	yyQQPersonaState.Style.Enabled = false
	yyQQPersonaState.Unlock()
	yyStyleObserve("g-off", "你好呀")
	yyQQStyleState.Lock()
	off := len(yyQQStyleState.Groups["g-off"])
	yyQQStyleState.Unlock()
	if off != 0 {
		t.Fatalf("关闭后不应收录，实际 %d", off)
	}
	if yyStyleSummary("g-off") != "" {
		t.Fatal("关闭后摘要应为空")
	}
	yyQQPersonaState.Lock()
	yyQQPersonaState.Style.Enabled = true
	yyQQPersonaState.Unlock()
	for i := 0; i < 250; i++ {
		yyStyleObserve("g-cap", "消息"+strconv.Itoa(i))
	}
	yyQQStyleState.Lock()
	rows := yyQQStyleState.Groups["g-cap"]
	yyQQStyleState.Unlock()
	if len(rows) != yyQQStyleBufferMax {
		t.Fatalf("环形缓冲应保持 %d，实际 %d", yyQQStyleBufferMax, len(rows))
	}
	if !strings.HasSuffix(rows[len(rows)-1].Text, "249") {
		t.Fatalf("应保留最新一条，实际 %q", rows[len(rows)-1].Text)
	}
	yyQQPersonaState.Lock()
	yyQQPersonaState.Style.MinMsgs = 300
	yyQQPersonaState.Unlock()
	if yyStyleSummary("g-cap") != "" {
		t.Fatal("样本不足时摘要应为空")
	}
}
