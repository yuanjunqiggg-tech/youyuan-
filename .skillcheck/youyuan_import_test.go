package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func yyTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, text := range files {
		file, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = file.Write([]byte(text))
	}
	_ = w.Close()
	return buf.Bytes()
}
func TestYouYuanPluginImport(t *testing.T) {
	root := t.TempDir()
	manifest := `{"id":"custom_plugin","language":"python","entry":"main.py"}`
	valid := yyTestZip(t, map[string]string{"manifest.json": manifest, "main.py": "def on_event(event,host): return host.status()", "helper/util.py": "value=42"})
	id, err := yyUnpackPlugin(valid, root)
	if err != nil || id != "custom_plugin" {
		t.Fatal(id, err)
	}
	if _, err = os.Stat(filepath.Join(root, id, "helper/util.py")); err != nil {
		t.Fatal(err)
	}
	if _, err = yyUnpackPlugin(valid, root); err == nil {
		t.Fatal("重复导入覆盖了插件数据")
	}
	malicious := yyTestZip(t, map[string]string{"manifest.json": manifest, "main.py": "x=1", "../outside.py": "bad"})
	if _, err = yyUnpackPlugin(malicious, t.TempDir()); err == nil {
		t.Fatal("ZIP 越界路径被接受")
	}
}
