package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func initPluginControllerForTest(t *testing.T) {
	t.Helper()
	pluginController.SetPluginDirs(t.TempDir(), t.TempDir())
}

// TestHandlePluginCreate 验证 HTTP 层创建插件返回 id。
func TestHandlePluginCreate(t *testing.T) {
	initPluginControllerForTest(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/plugin/create", strings.NewReader(`{"type":"lua","name":"HTTP创建","description":"d"}`))
	handlePluginCreate(rr, req)
	var resp struct {
		Ok bool   `json:"ok"`
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v, body=%s", err, rr.Body.String())
	}
	if !resp.Ok || resp.ID == "" {
		t.Errorf("创建失败: %s", rr.Body.String())
	}
	// 名称校验
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/plugin/create", strings.NewReader(`{"type":"lua","name":"  ","description":""}`))
	handlePluginCreate(rr, req)
	if strings.Contains(rr.Body.String(), `"ok":true`) {
		t.Errorf("空名称应被拒: %s", rr.Body.String())
	}
}

// TestHandlePluginFileReadWriteCode 代码文件读写往返（HTTP 层）。
func TestHandlePluginFileReadWriteCode(t *testing.T) {
	initPluginControllerForTest(t)
	id, err := pluginController.CreatePlugin("lua", "HTTP文件", "")
	if err != nil {
		t.Fatalf("CreatePlugin 失败: %v", err)
	}
	// write
	wr := httptest.NewRecorder()
	wreq := httptest.NewRequest("POST", "/api/plugin/file/write", strings.NewReader(`{"id":"`+id+`","scope":"code","file":"","content":"print(1)"}`))
	handlePluginFileWrite(wr, wreq)
	if !strings.Contains(wr.Body.String(), `"ok":true`) {
		t.Fatalf("写入失败: %s", wr.Body.String())
	}
	// read
	rr := httptest.NewRecorder()
	rreq := httptest.NewRequest("GET", "/api/plugin/file/read?id="+id+"&scope=code&file=", nil)
	handlePluginFileRead(rr, rreq)
	var resp struct {
		Ok      bool   `json:"ok"`
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("读响应解析失败: %v", err)
	}
	if !resp.Ok || resp.Name != id+".lua" || resp.Content != "print(1)" {
		t.Errorf("代码读写不符: %s", rr.Body.String())
	}
}

func TestPluginEditorReadsWritesNestedModulesAndData(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	id, e := pc.CreatePlugin("lua", "nested editor", "")
	if e != nil {
		t.Fatal(e)
	}
	for _, scope := range []string{"code", "data"} {
		file := "modules/death.lua"
		if scope == "data" {
			file = "ai_prompts/core.txt"
		}
		if e := pc.WritePluginFile(scope, id, file, "nested source"); e != nil {
			t.Fatal(e)
		}
		_, value, e := pc.ReadPluginFile(scope, id, file)
		if e != nil || value != "nested source" {
			t.Fatal(value, e)
		}
		for _, bad := range []string{"../escape", "/absolute", "C:/escape", `modules/../../escape`} {
			if e := pc.WritePluginFile(scope, id, bad, "bad"); e == nil {
				t.Fatal("escape accepted", bad)
			}
		}
	}
}

// TestHandlePluginFileReadRejectsTraversal 数据文件路径穿越在 HTTP 层被拒。
func TestHandlePluginFileReadRejectsTraversal(t *testing.T) {
	initPluginControllerForTest(t)
	rr := httptest.NewRecorder()
	rreq := httptest.NewRequest("GET", "/api/plugin/file/read?id=x&scope=data&file=..%2F..%2Fetc%2Fpasswd", nil)
	handlePluginFileRead(rr, rreq)
	if strings.Contains(rr.Body.String(), `"ok":true`) {
		t.Errorf("路径穿越应被拒: %s", rr.Body.String())
	}
}

// TestHandlePluginApiDoc 开发文档接口返回内容。
func TestHandlePluginApiDoc(t *testing.T) {
	rr := httptest.NewRecorder()
	handlePluginApiDoc(rr, httptest.NewRequest("GET", "/api/plugin/api-doc", nil))
	var resp struct {
		Ok      bool   `json:"ok"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if !resp.Ok || resp.Content == "" || !strings.Contains(resp.Content, "API") {
		t.Errorf("api-doc 内容异常: %s", rr.Body.String())
	}
	_ = http.MethodGet
}

// TestHandlePluginDeleteNoConnection 验证删除接口在未连接机器人时也能删除（无需连接）。
func TestHandlePluginDeleteNoConnection(t *testing.T) {
	initPluginControllerForTest(t)
	id, err := pluginController.CreatePlugin("lua", "HTTP删除", "")
	if err != nil {
		t.Fatalf("CreatePlugin 失败: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/plugin/delete", strings.NewReader(`{"id":"`+id+`"}`))
	handlePluginDelete(rr, req)
	if !strings.Contains(rr.Body.String(), `"ok":true`) {
		t.Fatalf("未连接时应能删除，实际: %s", rr.Body.String())
	}
	if pluginController.status(id) != nil {
		t.Error("删除后插件仍存在")
	}
}

// TestHandlePluginWordbankDoc 词库文档接口返回词库格式内容（区别于 Lua API 文档）。
func TestHandlePluginWordbankDoc(t *testing.T) {
	rr := httptest.NewRecorder()
	handlePluginWordbankDoc(rr, httptest.NewRequest("GET", "/api/plugin/wordbank-doc", nil))
	var resp struct {
		Ok      bool   `json:"ok"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if !resp.Ok || resp.Content == "" {
		t.Fatalf("wordbank-doc 为空: %s", rr.Body.String())
	}
	if !strings.Contains(resp.Content, "菜单项") {
		t.Errorf("词库文档应包含 菜单项 格式说明")
	}
	if strings.Contains(resp.Content, "game.") {
		t.Errorf("词库文档不应混入 Lua API 内容")
	}
}
