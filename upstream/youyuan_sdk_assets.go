package main

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
)

//go:embed sdk/cpp/youyuan.hpp sdk/cpp/third_party/json.hpp sdk/python_seed.py sdk/cpp_seed.cpp sdk/python_observer.py sdk/cpp_observer.cpp
var yySDKAssets embed.FS

func yySDKInstallSamples(dataDir string) {
	yySDKInstallObservers(dataDir)
	root := filepath.Join(yyRuntimeRoot, "yy_sdk_python_demo")
	if _, e := os.Stat(filepath.Join(root, "manifest.json")); os.IsNotExist(e) {
		_ = os.MkdirAll(root, 0700)
		_ = os.WriteFile(filepath.Join(root, "main.py"), []byte(yySDKSeed("python")), 0600)
		data, _ := json.Marshal(yyRuntimeManifest{ID: "yy_sdk_python_demo", Name: "Python 游戏控制示例", Language: "python", Entry: "main.py", Events: []string{"chat", "join", "leave", "ready"}})
		_ = os.WriteFile(filepath.Join(root, "manifest.json"), data, 0600)
	}
	dir, e := os.ReadFile(filepath.Join(dataDir, "native-directory"))
	if e != nil {
		return
	}
	library, e := os.ReadFile(filepath.Join(string(dir), "libyy_sdk_demo.so"))
	if e != nil {
		return
	}
	root = filepath.Join(yyRuntimeRoot, "yy_sdk_cpp_demo")
	if _, e = os.Stat(filepath.Join(root, "manifest.json")); os.IsNotExist(e) {
		_ = os.MkdirAll(root, 0700)
		_ = os.WriteFile(filepath.Join(root, "plugin.so"), library, 0600)
		data, _ := json.Marshal(yyRuntimeManifest{ID: "yy_sdk_cpp_demo", Name: "C++ 游戏控制示例", Language: "native_cpp", Entry: "plugin.so", Events: []string{"chat", "join", "leave", "ready"}})
		_ = os.WriteFile(filepath.Join(root, "manifest.json"), data, 0600)
	}
}

func yySDKInstallObservers(dataDir string) {
	for _, language := range []string{"python", "native_cpp"} {
		id, entry, source, label := "yy_sense_python_demo", "main.py", "sdk/python_observer.py", "Python 血量与位置监控"
		if language == "native_cpp" {
			id, entry, source, label = "yy_sense_cpp_demo", "plugin.so", "", "C++ 血量与位置监控"
		}
		root := filepath.Join(yyRuntimeRoot, id)
		if _, e := os.Stat(filepath.Join(root, "manifest.json")); e == nil {
			continue
		}
		var data []byte
		var e error
		if source != "" {
			data, e = yySDKAssets.ReadFile(source)
		} else {
			var dir []byte
			dir, e = os.ReadFile(filepath.Join(dataDir, "native-directory"))
			if e == nil {
				data, e = os.ReadFile(filepath.Join(string(dir), "libyy_observer.so"))
			}
		}
		if e != nil {
			continue
		}
		_ = os.MkdirAll(root, 0700)
		if e = os.WriteFile(filepath.Join(root, entry), data, 0600); e != nil {
			continue
		}
		manifest, _ := json.Marshal(yyRuntimeManifest{ID: id, Name: label, Version: "1.1.0", Language: language, Entry: entry, Events: []string{"ready"}})
		_ = os.WriteFile(filepath.Join(root, "manifest.json"), manifest, 0600)
	}
}

func yySDKSeed(language string) string {
	name := "sdk/python_seed.py"
	if language == "native_cpp" {
		name = "sdk/cpp_seed.cpp"
	}
	data, _ := yySDKAssets.ReadFile(name)
	return string(data)
}
func yySDKInstallHeaders(ws *yyWorkspace) error {
	for _, name := range []string{"youyuan.hpp", "third_party/json.hpp"} {
		data, e := yySDKAssets.ReadFile("sdk/cpp/" + name)
		if e != nil {
			return e
		}
		path := filepath.Join(yyWorkspaceRoot(ws), name)
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return e
		}
		if e = os.WriteFile(path, data, 0600); e != nil {
			return e
		}
	}
	return nil
}
