package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFullPluginBackupPreservesNestedFilesAndData(t *testing.T) {
	sourceDir := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(sourceDir, "")
	id := "完整备份"
	contents := map[string]string{
		"插件文件/完整备份/完整备份.lua":               "return 42",
		"插件文件/完整备份/完整备份.manifest.json":     `{"id":"完整备份"}`,
		"插件文件/完整备份/www/css/theme.css":      "body { color: blue; }",
		"插件文件/完整备份/assets/images/icon.txt": "nested asset",
		"插件配置文件/完整备份/完整备份.json":            `{"enabled":true}`,
		"插件配置文件/完整备份/nested/extra.json":    `{"custom":1}`,
		"插件数据文件/完整备份/players/alice.json":   `{"score":12}`,
		"插件数据文件/完整备份/logs/2026-09.txt":     "event log",
	}
	for name, value := range contents {
		full := filepath.Join(sourceDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	emptyDir := filepath.Join(sourceDir, "插件数据文件", id, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}
	backup, err := pc.ExportPackage(id)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(backup)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range zr.File {
		names[file.Name] = true
	}
	zr.Close()
	for name := range contents {
		if !names[name] {
			t.Fatalf("备份缺少 %s", name)
		}
	}
	if !names["插件数据文件/完整备份/empty/"] {
		t.Fatal("备份未保留空目录")
	}
	market, err := pc.ExportMarketPackage(id)
	if err != nil {
		t.Fatal(err)
	}
	marketZip, err := zip.OpenReader(market)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range marketZip.File {
		if strings.HasPrefix(file.Name, "插件数据文件/") || strings.HasPrefix(file.Name, "插件配置文件/") || file.Name == "config.json" {
			t.Fatalf("市场发布包泄露了私人配置或数据: %s", file.Name)
		}
	}
	marketZip.Close()
	restoredDir := t.TempDir()
	restored := NewPluginController()
	restored.SetPluginDirs(restoredDir, "")
	restoredID, err := restored.ImportPackage(backup)
	if err != nil || restoredID != id {
		t.Fatalf("恢复失败: id=%q err=%v", restoredID, err)
	}
	for name, value := range contents {
		data, err := os.ReadFile(filepath.Join(restoredDir, filepath.FromSlash(name)))
		if err != nil || string(data) != value {
			t.Fatalf("恢复后的 %s 内容不符: %q, %v", name, data, err)
		}
	}
	if info, err := os.Stat(filepath.Join(restoredDir, "插件数据文件", id, "empty")); err != nil || !info.IsDir() {
		t.Fatalf("恢复未保留空目录: %v", err)
	}
}

func TestFullPluginBackupRejectsCrossPluginPath(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	backup := writeZip(t, map[string]string{
		"plugin-package.json":  `{"format":"youyuan.plugin.backup","version":1,"id":"safe"}`,
		"插件文件/other/other.lua": "return 1",
	})
	if _, err := pc.ImportPackage(backup); err == nil {
		t.Fatal("跨插件路径应被拒绝")
	}
}
