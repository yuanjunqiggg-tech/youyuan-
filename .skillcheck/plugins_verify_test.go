package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// 每插件独立目录：导入→插件文件/<id>/，Run→配置文件在 插件配置文件/<id>/、数据目录在 插件数据文件/<id>/。
func TestPluginPerFolderStructure(t *testing.T) {
 if _,err:=os.Stat("core_utils.zip");err!=nil {t.Skip("未提供旧版 core_utils.zip 外部测试包")}
	dir := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(dir, "")
	bm := &BotManager{}
	abs, _ := filepath.Abs("core_utils.zip")
	if _, err := pc.ImportPackage(abs); err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "插件文件", "core_utils", "core_utils.lua")); err != nil {
		t.Fatalf("导入未写入 插件文件/core_utils/: %v", err)
	}
	if err := pc.ScanPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := pc.Run(bm, "core_utils"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "插件配置文件", "core_utils", "core_utils.json")); err != nil {
		t.Fatalf("配置文件未创建到 插件配置文件/core_utils/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "插件数据文件", "core_utils")); err != nil {
		t.Fatalf("数据目录未创建 插件数据文件/core_utils/: %v", err)
	}
}

// readZipMain 从 zip 包读 main.lua。
func readZipMain(t *testing.T, zipPath string) string {
	t.Helper()
	abs, err := filepath.Abs(zipPath)
	if err == nil {
		zipPath = abs
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("打开 %s: %v", zipPath, err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if filepath.Base(f.Name) == "main.lua" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("读 %s: %v", f.Name, err)
			}
			defer rc.Close()
			b, _ := io.ReadAll(rc)
			return string(b)
		}
	}
	return ""
}

// 5 个插件 zip 都能加载（无 Lua 语法错误），且 core_utils 跨插件金钱调用正确。
func TestPluginSuiteVerification(t *testing.T) {
 for _,name:=range []string{"core_utils","territory","shop","anti_crit","system_control"} {if _,err:=os.Stat(name+".zip");err!=nil {t.Skip("未提供旧版外部插件测试包")}}
	bm := &BotManager{}
	srcs := map[string]string{}
	for _, id := range []string{"core_utils", "territory", "shop", "anti_crit", "system_control"} {
		src := readZipMain(t, id+".zip")
		if src == "" {
			t.Fatalf("%s 无 main.lua", id)
		}
		srcs[id] = src
		if err := LoadLuaPlugin(bm, id, src); err != nil {
			t.Fatalf("%s 加载失败: %v", id, err)
		}
	}

	var got []string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = append(got, txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	// 跨插件调用 core_utils 金钱（territory 等依赖它）
	L := newLuaState(bm)
	defer L.Close()
	if err := L.DoString(`local c = api.require("core_utils"); c.money_add("alice", 100); game.sendTargeted("a", tostring(c.money_get("alice")))`); err != nil {
		t.Fatalf("跨插件金钱调用失败: %v", err)
	}
	if len(got) != 1 || got[0] != "100" {
		t.Fatalf("金钱跨插件结果错误: %v", got)
	}

	// 依赖声明解析：territory 声明 pre_plugins 依赖 core_utils，Run 会先加载 core_utils
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	for _, z := range []string{"core_utils.zip", "territory.zip"} {
		abs, _ := filepath.Abs(z)
		if _, err := pc.ImportPackage(abs); err != nil {
			t.Fatalf("导入 %s: %v", z, err)
		}
	}
	if err := pc.ScanPlugins(); err != nil {
		t.Fatalf("ScanPlugins: %v", err)
	}
	if err := pc.Run(bm, "territory"); err != nil {
		t.Fatalf("Run territory: %v", err)
	}
	if !pc.running["core_utils"] || !pc.running["territory"] {
		t.Fatalf("前置解析失败: core_utils=%v territory=%v", pc.running["core_utils"], pc.running["territory"])
	}
	// 清理 territory 启动的定时器/事件，避免污染后续测试
	RemovePluginSchedules("territory")
	RemovePluginEvents("territory")
}
