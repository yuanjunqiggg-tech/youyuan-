package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Preserve sample edits and data in a recoverable backup, outside installed plugins.
func yyArchiveBundledSamples(dataDir string) {
	labels := map[string]string{"yy_sdk_python_demo": "Python 游戏控制示例", "yy_sdk_cpp_demo": "C++ 游戏控制示例", "yy_sense_python_demo": "Python 血量与位置监控", "yy_sense_cpp_demo": "C++ 血量与位置监控", "python_counter": "Python 常驻计数器"}
	archived := map[string]bool{}
	for id, label := range labels {
		source := filepath.Join(yyRuntimeRoot, id)
		info, e := os.Lstat(source)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		raw, e := os.ReadFile(filepath.Join(source, "manifest.json"))
		var manifest yyRuntimeManifest
		if e != nil || json.Unmarshal(raw, &manifest) != nil || manifest.ID != id || manifest.Name != label {
			continue
		}
		target := filepath.Join(dataDir, "示例备份", id+"-"+strconv.FormatInt(time.Now().UnixNano(), 10))
		if os.MkdirAll(filepath.Dir(target), 0700) != nil {
			continue
		}
		if os.Rename(source, target) == nil {
			archived[id] = true
		}
	}
	path := filepath.Join(dataDir, "resident-runtime-enabled.json")
	raw, e := os.ReadFile(path)
	if e != nil {
		return
	}
	var items map[string]yyRuntimeChoice
	if json.Unmarshal(raw, &items) != nil {
		return
	}
	for key, item := range items {
		if archived[item.ID] {
			delete(items, key)
		}
	}
	if data, e := json.Marshal(items); e == nil {
		if os.WriteFile(path+".tmp", data, 0600) == nil {
			_ = os.Rename(path+".tmp", path)
		}
	}
}
