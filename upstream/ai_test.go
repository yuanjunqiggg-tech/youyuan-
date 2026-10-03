package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// 用临时目录隔离，避免污染真实 ai_models.json。
func setupTestStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	aiStore.setPath(dir)
	return dir
}

func TestAIModelStoreCRUD(t *testing.T) {
	t.Setenv("YY_AI_BASE_URL", "https://model.example.invalid")
	t.Setenv("YY_AI_MODEL", "own-model")
	_ = setupTestStore(t)
	defer aiStore.setPath(filepath.Join(os.TempDir(), "ai_models.json"))

	// 非法校验
	if err := aiStore.Add(&AIModel{Name: "", Provider: "openai", BaseURL: "x", ModelID: "m"}); err == nil {
		t.Fatal("空模型名应报错")
	}
	if err := aiStore.Add(&AIModel{Name: "a", Provider: "bogus", BaseURL: "x", ModelID: "m"}); err == nil {
		t.Fatal("非法供应商应报错")
	}

	m := &AIModel{Name: "我的DeepSeek", Provider: "openai", BaseURL: "https://api.deepseek.com", APIKey: "sk-abc", ModelID: "deepseek-chat", Thinking: "medium", Streaming: true}
	if err := aiStore.Add(m); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}
	if got := aiStore.Get("我的DeepSeek"); got == nil || got.ModelID != "deepseek-chat" {
		t.Fatalf("Get 异常: %+v", got)
	}
	if len(aiStore.List()) != 2 { // 内置模型恒在 + 我的DeepSeek
		t.Fatalf("List 长度应为 2，got %d", len(aiStore.List()))
	}

	// 同名覆盖（更新 key）
	m2 := &AIModel{Name: "我的DeepSeek", Provider: "openai", BaseURL: "https://api.deepseek.com", APIKey: "sk-new", ModelID: "deepseek-reasoner"}
	if err := aiStore.Add(m2); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if aiStore.Get("我的DeepSeek").APIKey != "sk-new" {
		t.Fatal("更新 key 失败")
	}
	if len(aiStore.List()) != 2 {
		t.Fatal("更新不应新增条目")
	}

	aiStore.Delete("我的DeepSeek")
	if aiStore.Get("我的DeepSeek") != nil || len(aiStore.List()) != 1 { // 只剩内置
		t.Fatal("删除失败")
	}
}

func TestMaskKey(t *testing.T) {
	if maskKey("") != "" {
		t.Fatal("空 key 应返回空")
	}
	if maskKey("short") != "****" {
		t.Fatal("短 key 应全打码")
	}
	if maskKey("sk-abcdefgh1234") != "sk-a****1234" {
		t.Fatalf("脱敏结果异常: %s", maskKey("sk-abcdefgh1234"))
	}
}

func TestToolArgCoercion(t *testing.T) {
	args := map[string]any{"fn": "sendCommand", "args": []any{float64(3), "x", json.Number("5")}}
	if aiArgStr(args, "fn") != "sendCommand" {
		t.Fatal("aiArgStr 取值失败")
	}
	var argv []any
	if a, ok := args["args"].([]any); ok {
		argv = a
	}
	if argInt(argv, 0) != 3 {
		t.Fatalf("argInt(float64) 失败: %d", argInt(argv, 0))
	}
	if argInt(argv, 2) != 5 {
		t.Fatalf("argInt(json.Number) 失败: %d", argInt(argv, 2))
	}
	if argString(argv, 1) != "x" {
		t.Fatal("argString 失败")
	}
}

// 工具定义必须为每个工具提供名称与参数 schema。
func TestAIToolDefsWellFormed(t *testing.T) {
	for _, td := range aiToolDefs() {
		if td.Name == "" {
			t.Fatal("存在无名工具")
		}
		if _, ok := td.Parameters["type"]; !ok {
			t.Fatalf("工具 %s 缺 parameters.type", td.Name)
		}
		if _, ok := aiToolHandlers[td.Name]; !ok {
			t.Fatalf("工具 %s 定义了但没有执行器", td.Name)
		}
	}
}

// 沙箱：resolveAIFilePath 只放行插件目录内文件，拒绝系统路径/越界。
func TestAIFileSandbox(t *testing.T) {
	dir := t.TempDir()
	old := pluginController
	pluginController = &PluginController{privateDir: dir}
	defer func() { pluginController = old }()

	// 相对路径在插件目录内 → 放行
	p, err := resolveAIFilePath("插件文件/abc/abc.lua")
	if err != nil || p != filepath.Join(dir, "插件文件", "abc", "abc.lua") {
		t.Fatalf("插件目录内路径应放行，got %q err=%v", p, err)
	}
	// 系统绝对路径 → 拒绝（日志白名单除外）
	if _, err := resolveAIFilePath("/etc/passwd"); err == nil {
		t.Fatal("系统绝对路径应拒绝")
	}
	// 日志白名单 → 放行
	if _, err := resolveAIFilePath(aiDebugLogPath); err != nil {
		t.Fatalf("调试日志白名单应放行: %v", err)
	}
	// 越界 .. → 拒绝
	if _, err := resolveAIFilePath("../config.json"); err == nil {
		t.Fatal("越界路径应拒绝")
	}
}

// 按行编辑：write_file 整写 + edit_lines 行范围替换。
func TestAIFileEditLines(t *testing.T) {
	dir := t.TempDir()
	old := pluginController
	pluginController = &PluginController{privateDir: dir}
	defer func() { pluginController = old }()
	aiResetModified()

	rel := "插件文件/abc/abc.lua"
	if _, isErr := execTool("write_file", map[string]any{"path": rel, "content": "a\nb\nc\nd\n"}); isErr {
		t.Fatal("write_file 失败")
	}
	full := filepath.Join(dir, "插件文件", "abc", "abc.lua")
	got, _ := os.ReadFile(full)
	if string(got) != "a\nb\nc\nd\n" {
		t.Fatalf("写入内容异常: %q", string(got))
	}
	// 把第 2-3 行换成 X/Y
	res, isErr := execTool("edit_lines", map[string]any{"path": rel, "lines": "2-3", "new_content": "X\nY"})
	if isErr {
		t.Fatalf("edit_lines 失败: %s", res)
	}
	got, _ = os.ReadFile(full)
	if string(got) != "a\nX\nY\nd\n" {
		t.Fatalf("行替换异常: %q", string(got))
	}
	// 会话改动应被记录（供存档）
	if len(aiModifiedList()) == 0 {
		t.Fatal("改动未被记录")
	}
}

// 会话存储 save/load/delete。
func TestAISessionStore(t *testing.T) {
	dir := t.TempDir()
	sessionStore.setPath(dir)
	if err := sessionStore.Save("test会话", "plg", []aiTurn{{Role: "user", Text: "hi"}}); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	sess, err := sessionStore.Load("test会话")
	if err != nil || len(sess.Messages) != 1 || sess.Messages[0].Text != "hi" {
		t.Fatalf("Load 异常: %+v err=%v", sess, err)
	}
	if len(sessionStore.List("")) != 1 {
		t.Fatal("List 应为 1")
	}
	// 自动保存的会话不出现在列表
	if err := sessionStore.Save(aiAutoSession, "plg", []aiTurn{{Role: "user", Text: "auto"}}); err != nil {
		t.Fatalf("自动会话保存失败: %v", err)
	}
	if len(sessionStore.List("")) != 1 {
		t.Fatalf("自动会话不应出现在列表，List=%d", len(sessionStore.List("")))
	}
	if sess, err := sessionStore.Load(aiAutoSession); err != nil || len(sess.Messages) != 1 {
		t.Fatal("自动会话应可通过 Load 读回")
	}
	// 重命名
	if err := sessionStore.Rename("test会话", "改名后"); err != nil {
		t.Fatalf("Rename 失败: %v", err)
	}
	if _, err := sessionStore.Load("test会话"); err == nil {
		t.Fatal("旧名不应再存在")
	}
	if sess, err := sessionStore.Load("改名后"); err != nil || len(sess.Messages) != 1 {
		t.Fatal("新名读回失败")
	}
	// 最近会话（应忽略 __auto__）
	if recent, ok := sessionStore.Recent(""); !ok || recent.Name != "改名后" {
		t.Fatalf("Recent 异常: %+v ok=%v", recent, ok)
	}
	// 会话隔离：换一个 plugin_id 不应看到上面的会话
	if _, ok := sessionStore.Recent("别的插件"); ok {
		t.Fatal("Recent 不应返回其他插件的会话")
	}
	if len(sessionStore.List("别的插件")) != 0 {
		t.Fatal("List 应按 plugin_id 隔离")
	}
	// 非法名拒绝
	if err := sessionStore.Save("../evil", "", []aiTurn{}); err == nil {
		t.Fatal("非法名应拒绝")
	}
	sessionStore.Delete("改名后") // 已被重命名
	sessionStore.Delete(aiAutoSession)
	if len(sessionStore.List("")) != 0 {
		t.Fatal("Delete 失败")
	}
}

// 时间戳/中文标点等常见标题字符应能被 safeName 接受（修复 AI 改名/自动建会话）。
func TestSafeNameAllowsTitles(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"8-23 17:59", true},
		{"你好，世界！帮我写插件", true},
		{"领地系统 v2（完善）", true},
		{"", false},
		{"..", false},
		{"../evil", false},
		{"a/b", false},
	}
	for _, c := range cases {
		if _, got := safeName(c.name); got != c.want {
			t.Fatalf("safeName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// 改动存档/读档：写文件 → 存档 → 改文件 → 恢复。
func TestAISnapshot(t *testing.T) {
	dir := t.TempDir()
	old := pluginController
	pluginController = &PluginController{privateDir: dir}
	defer func() { pluginController = old }()
	aiResetModified()
	snapshotStore.setPath(dir)

	rel := "插件文件/abc/abc.lua"
	if _, isErr := execTool("write_file", map[string]any{"path": rel, "content": "v1"}); isErr {
		t.Fatal("write_file 失败")
	}
	if err := snapshotStore.Save("s1"); err != nil {
		t.Fatalf("存档失败: %v", err)
	}
	// 改动文件为 v2
	full := filepath.Join(dir, "插件文件", "abc", "abc.lua")
	os.WriteFile(full, []byte("v2"), 0644)
	if n, err := snapshotStore.Restore("s1"); err != nil || n != 1 {
		t.Fatalf("恢复失败: n=%d err=%v", n, err)
	}
	got, _ := os.ReadFile(full)
	if string(got) != "v1" {
		t.Fatalf("恢复内容异常: %q", string(got))
	}
}

// 通过 httptest 直接测 handler 接线（不经过篡改中间件）。
func TestAIModelsHandler(t *testing.T) {
	t.Setenv("YY_AI_BASE_URL", "https://model.example.invalid")
	t.Setenv("YY_AI_MODEL", "own-model")
	_ = setupTestStore(t)
	defer aiStore.Delete("h")

	// POST 新增
	rr := httptest.NewRecorder()
	body := `{"name":"h","provider":"openai","base_url":"https://x","api_key":"sk-1","model_id":"m"}`
	handleAIModels(rr, httptest.NewRequest("POST", "/api/ai/models", bytes.NewBufferString(body)))
	if rr.Code != 200 {
		t.Fatalf("POST status=%d", rr.Code)
	}
	if aiStore.Get("h") == nil {
		t.Fatal("POST 后模型未入库")
	}

	// GET 列表（key 脱敏）
	rr = httptest.NewRecorder()
	handleAIModels(rr, httptest.NewRequest("GET", "/api/ai/models", nil))
	var listResp struct {
		OK     bool `json:"ok"`
		Models []struct {
			Name    string `json:"name"`
			APIKey  string `json:"api_key"`
			Builtin bool   `json:"builtin"`
		} `json:"models"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil || !listResp.OK || len(listResp.Models) < 2 {
		t.Fatalf("GET 响应异常: %s", rr.Body.String())
	}
	// 列表应含内置模型「prism自动」（builtin），且用户模型 key 脱敏为 ****
	foundBuiltin := false
	var userKey string
	for _, m := range listResp.Models {
		if m.Name == BuiltinModelName {
			foundBuiltin = m.Builtin
		}
		if m.Name == "h" {
			userKey = m.APIKey
		}
	}
	if !foundBuiltin {
		t.Fatalf("列表缺内置模型 %q: %s", BuiltinModelName, rr.Body.String())
	}
	if userKey != "****" {
		t.Fatalf("key 应脱敏为 ****，got %q", userKey)
	}

	// DELETE
	rr = httptest.NewRecorder()
	handleAIModels(rr, httptest.NewRequest("DELETE", "/api/ai/models", bytes.NewBufferString(`{"name":"h"}`)))
	if aiStore.Get("h") != nil {
		t.Fatal("DELETE 后模型应移除")
	}
}

func TestAIChatHandlerValidation(t *testing.T) {
	// 缺模型名 → 报错
	rr := httptest.NewRecorder()
	handleAIChat(rr, httptest.NewRequest("POST", "/api/ai/chat", bytes.NewBufferString(`{"model_name":"","messages":[]}`)))
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.OK || resp.Error == "" {
		t.Fatalf("缺模型名应报错: %s", rr.Body.String())
	}

	// 模型不存在
	rr = httptest.NewRecorder()
	handleAIChat(rr, httptest.NewRequest("POST", "/api/ai/chat", bytes.NewBufferString(`{"model_name":"nope","messages":[{"role":"user","content":"hi"}]}`)))
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.OK || resp.Error == "" {
		t.Fatalf("未知模型应报错: %s", rr.Body.String())
	}
}
