package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeExportKeepsNestedFilesImportable(t *testing.T) {
	root := t.TempDir()
	id := "complete_python"
	dir := filepath.Join(root, id)
	for name, content := range map[string]string{
		"manifest.json":   `{"id":"complete_python","language":"python","entry":"main.py"}`,
		"main.py":         "def on_event(event, host): return {}",
		"lib/helper.py":   "VALUE = 7",
		"data/state.json": `{"counter":7}`,
	} {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0755); err != nil {
		t.Fatal(err)
	}
	output, err := yyRuntimeExportPackage(root, id)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range zr.File {
		names[file.Name] = true
	}
	zr.Close()
	for _, name := range []string{"manifest.json", "main.py", "lib/helper.py", "data/state.json", "empty/"} {
		if !names[name] {
			t.Fatalf("导出缺少 %s", name)
		}
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	restoreRoot := t.TempDir()
	restored, err := yyUnpackPlugin(raw, restoreRoot)
	if err != nil || restored != id {
		t.Fatalf("重新导入失败: %q %v", restored, err)
	}
	if data, err := os.ReadFile(filepath.Join(restoreRoot, id, "lib", "helper.py")); err != nil || string(data) != "VALUE = 7" {
		t.Fatalf("子目录未还原: %q %v", data, err)
	}
}
