package main

// 本文件覆盖 2026-10-20 安全审查后的加固改动，防止回退：
//   - H3 任意文件写入（preview/save-image 与 file/upload）
//   - H4 插件 id 注入（validPluginID 与前端内联事件）
// 每条用例都对应一个真实攻击路径，不是形式化断言。

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// H4：插件 id 会被拼进前端内联事件处理器的 JS 字符串，引号/反引号/尖括号/圆括号
// 都能闭合字符串并注入脚本，必须在校验层拒绝。
func TestValidPluginIDRejectsScriptInjectionChars(t *testing.T) {
	bad := []string{
		"a'b", `a"b`, "a`b", "a<b", "a>b", "a(b", "a)b",
		"a');fetch('//x)//",
	}
	for _, id := range bad {
		if validPluginID(id) {
			t.Fatalf("id %q 应被拒绝：可构造 DOM XSS 并通过 WebView 原生桥读本机文件", id)
		}
	}
	good := []string{"系统测试", "my-plugin_1.2", "abc", "插件-01"}
	for _, id := range good {
		if !validPluginID(id) {
			t.Fatalf("id %q 是合法插件名，不应被拒绝", id)
		}
	}
}

func yyPreviewSaveRequest(path string) *bytes.Buffer {
	body, _ := json.Marshal(map[string]string{
		"path": path,
		"data": "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("fake-png")),
	})
	return bytes.NewBuffer(body)
}

// H3：preview/save-image 原先 os.WriteFile(req.Path) 任意路径可写，
// 写 插件/<id>/main.py 后触发插件重载即任意代码执行。
func TestPreviewSaveImageRejectsTraversalAndNonImage(t *testing.T) {
	rejected := []string{
		"../../evil.png",
		"a/../../b.png",
		"/tmp/evil.py",
		"/tmp/evil",
		"/tmp/evil.png.py",
	}
	for _, p := range rejected {
		w := httptest.NewRecorder()
		handlePreviewSaveImage(w, httptest.NewRequest("POST", "/api/preview/save-image", yyPreviewSaveRequest(p)))
		if !bytes.Contains(w.Body.Bytes(), []byte(`"ok":false`)) {
			t.Fatalf("path %q 应被拒绝，实际响应：%s", p, w.Body.String())
		}
	}
}

func TestPreviewSaveImageStillAcceptsRealImagePath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "building_angle.png")
	w := httptest.NewRecorder()
	handlePreviewSaveImage(w, httptest.NewRequest("POST", "/api/preview/save-image", yyPreviewSaveRequest(target)))
	if !bytes.Contains(w.Body.Bytes(), []byte(`"ok":true`)) {
		t.Fatalf("合法图片路径应被接受，实际响应：%s", w.Body.String())
	}
	if st, err := os.Stat(target); err != nil || st.Size() == 0 {
		t.Fatalf("文件未写出：err=%v", err)
	}
}

// H3：file/upload 原先 dstPath = outputDir + "/" + filename，filename 可带 ../
// 穿越出目标目录。修复后只取 Base 名，上级目录不得出现文件。
func TestFileUploadStripsTraversalFilename(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "../../evil.txt")
	if err != nil {
		t.Fatalf("构造 multipart 失败：%v", err)
	}
	if _, err := fw.Write([]byte("payload")); err != nil {
		t.Fatalf("写入 multipart 失败：%v", err)
	}
	if err := mw.WriteField("output_dir", dir); err != nil {
		t.Fatalf("写 output_dir 失败：%v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("关闭 multipart 失败：%v", err)
	}

	r := httptest.NewRequest("POST", "/api/file/upload", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	handleFileUpload(w, r)

	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil.txt")); err == nil {
		t.Fatalf("文件名穿越未被阻止：%s 被写到目标目录之外", filepath.Join(filepath.Dir(dir), "evil.txt"))
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(filepath.Dir(dir)), "evil.txt")); err == nil {
		t.Fatalf("文件名穿越未被阻止（上两级）：evil.txt 被写到目标目录之外")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err != nil {
		t.Fatalf("去掉目录部分后应写入 output_dir/evil.txt，实际不存在：%v（响应：%s）", err, w.Body.String())
	}
}

// H3：output_dir 自身带 .. 时必须拒绝，避免把上传目录整体挪出预期根。
func TestFileUploadRejectsTraversalOutputDir(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "ok.txt")
	fw.Write([]byte("payload"))
	mw.WriteField("output_dir", "../../tmp/evil-dir")
	mw.Close()

	r := httptest.NewRequest("POST", "/api/file/upload", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	handleFileUpload(w, r)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"ok":false`)) {
		t.Fatalf("output_dir 含 .. 应被拒绝，实际响应：%s", w.Body.String())
	}
}
