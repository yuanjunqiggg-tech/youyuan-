package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeZip 构造一个临时 zip 包（name → content），返回路径。
func writeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "pkg.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

// 恶意 manifest id 路径穿越必须被拒绝。
func TestImportPackageRejectsTraversalID(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	zipPath := writeZip(t, map[string]string{
		"main.lua":      `plugin_api = {}`,
		"manifest.json": `{"id": "../../evil"}`,
	})
	if _, err := pc.ImportPackage(zipPath); err == nil || !strings.Contains(err.Error(), "id 非法") {
		t.Fatalf("路径穿越 id 应被拒绝, got: %v", err)
	}
	zipPath2 := writeZip(t, map[string]string{
		"main.lua":      `plugin_api = {}`,
		"manifest.json": `{"id": ".."}`,
	})
	if _, err := pc.ImportPackage(zipPath2); err == nil {
		t.Fatal("id \"..\" 应被拒绝")
	}
}

// 中文 id 是合法插件 id（真实用例：系统测试）。
func TestImportPackageAllowsChineseID(t *testing.T) {
	pc := NewPluginController()
	dir := t.TempDir()
	pc.SetPluginDirs(dir, "")
	zipPath := writeZip(t, map[string]string{
		"词库.json":       `{"触发词": {}}`,
		"manifest.json": `{"id": "系统测试"}`,
	})
	id, err := pc.ImportPackage(zipPath)
	if err != nil {
		t.Fatalf("中文 id 应可导入: %v", err)
	}
	if id != "系统测试" {
		t.Fatalf("id 错误: %q", id)
	}
	if _, err := os.Stat(filepath.Join(dir, "插件文件", "系统测试", "系统测试词库.json")); err != nil {
		t.Fatalf("中文 id 文件未落盘: %v", err)
	}
}

// 超大条目被拒（注入小上限验证，避免测试造 10MB 文件）。
func TestImportPackageRejectsOversizeEntry(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	old := maxPluginZipFileSize
	maxPluginZipFileSize = 1024
	defer func() { maxPluginZipFileSize = old }()
	zipPath := writeZip(t, map[string]string{
		"main.lua": `plugin_api = {}` + strings.Repeat("-- filler\n", 500), // 解压后约 5KB > 1KB 上限
	})
	if _, err := pc.ImportPackage(zipPath); err == nil || !strings.Contains(err.Error(), "过大") {
		t.Fatalf("超大条目应被拒, got: %v", err)
	}
}

// 条目数超限被拒。
func TestImportPackageRejectsTooManyEntries(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	old := maxPluginZipEntries
	maxPluginZipEntries = 3
	defer func() { maxPluginZipEntries = old }()
	files := map[string]string{"main.lua": `plugin_api = {}`}
	for i := 0; i < 5; i++ {
		files[fmt.Sprintf("extra%d.txt", i)] = "x"
	}
	zipPath := writeZip(t, files)
	if _, err := pc.ImportPackage(zipPath); err == nil || !strings.Contains(err.Error(), "条目过多") {
		t.Fatalf("条目超限应被拒, got: %v", err)
	}
}

// 加载失败回滚：Load 半途注册触发项后报错，Run 必须清理残留并置 error。
func TestRunRollbackOnLoadFailure(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	bm := &BotManager{}
	chatTriggers.Lock()
	chatTriggers.list = nil
	chatTriggers.Unlock()

	def := &PluginDef{ID: "rollback", Name: "R"}
	def.Load = func(bm *BotManager) error {
		bm.AddChatTrigger(&ChatTrigger{Plugin: "rollback", TriggerWords: []string{"半途"}})
		return fmt.Errorf("模拟加载失败")
	}
	if err := pc.AddDef(def); err != nil {
		t.Fatal(err)
	}
	if err := pc.Run(bm, "rollback"); err == nil {
		t.Fatal("Run 应返回加载错误")
	}
	chatTriggers.Lock()
	n := len(chatTriggers.list)
	chatTriggers.Unlock()
	if n != 0 {
		t.Fatalf("加载失败后应回滚触发项, 残留 %d 个", n)
	}
	if st := pc.status("rollback"); st == nil || st.State != "error" {
		t.Fatalf("应置 error 状态: %+v", st)
	}
}

// 插件 id 拒绝 URL/cookie 破坏性字符。
func TestValidPluginIDRejectsURLChars(t *testing.T) {
	for _, bad := range []string{"a b", "a?b", "a#b", "a%b", "a=b", "a&b"} {
		if validPluginID(bad) {
			t.Fatalf("id %q 应被拒绝", bad)
		}
	}
	for _, good := range []string{"core_utils", "系统测试", "a-b.c"} {
		if !validPluginID(good) {
			t.Fatalf("id %q 应被接受", good)
		}
	}
}
