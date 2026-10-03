package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestConfigReadWrite 验证配置读写与数据目录自动创建。
func TestConfigReadWrite(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)

	if err := pc.WriteConfig("a", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("WriteConfig 失败: %v", err)
	}
	cfg, err := pc.ReadConfig("a")
	if err != nil {
		t.Fatalf("ReadConfig 失败: %v", err)
	}
	if cfg["k"] != "v" {
		t.Errorf("读回配置不符: %v", cfg)
	}

	// 缺失文件 → 空 map 不报错
	empty, err := pc.ReadConfig("nope")
	if err != nil {
		t.Fatalf("ReadConfig 缺失文件应不报错: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("缺失文件应返回空 map: %v", empty)
	}

	// 数据目录自动创建
	dir := pc.DataDir("a")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("DataDir 未创建: dir=%s err=%v", dir, err)
	}
	// 数据目录实际落在 插件数据文件/<id>
	want := filepath.Join(tmp, "插件数据文件", "a")
	if dir != want {
		t.Errorf("DataDir 路径不符: got=%s want=%s", dir, want)
	}
}

// TestImportExportRoundTrip 验证打包导入 → 扫描注册 → 导出回 zip。
func TestImportExportRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)

	// 构造插件包 zip
	zipPath := filepath.Join(tmp, "pkg.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	write := func(name, content string) {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(content))
	}
	write("manifest.json", `{"id":"pkg1","name":"测试包","type":"lua"}`)
	write("main.lua", `game.sendTargeted("alice", "hi")`)
	write("config.json", `{"enabled":true}`)
	write("docs.md", "# 测试文档")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zf.Close()

	id, err := pc.ImportPackage(zipPath)
	if err != nil {
		t.Fatalf("ImportPackage 失败: %v", err)
	}
	if id != "pkg1" {
		t.Fatalf("导入 id 不符: got=%s want=pkg1", id)
	}

	// 扫描注册
	if err := pc.ScanPlugins(); err != nil {
		t.Fatalf("ScanPlugins 失败: %v", err)
	}
	found := false
	for _, p := range pc.List() {
		if p.ID == "pkg1" {
			found = true
		}
	}
	if !found {
		t.Fatal("扫描后列表未包含导入插件 pkg1")
	}

	// 配置文件落地
	cfg, err := pc.ReadConfig("pkg1")
	if err != nil || cfg["enabled"] != true {
		t.Fatalf("导入配置未落地: cfg=%v err=%v", cfg, err)
	}

	// 导出
	outPath, err := pc.ExportPackage("pkg1")
	if err != nil {
		t.Fatalf("ExportPackage 失败: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("导出 zip 不存在: %v", err)
	}
	zr, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("导出 zip 无法打开: %v", err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, want := range []string{"plugin-package.json", "插件文件/pkg1/pkg1.lua", "插件文件/pkg1/pkg1.manifest.json", "插件配置文件/pkg1/pkg1.json", "插件文件/pkg1/pkg1.docs.md", "插件数据文件/pkg1/"} {
		if !names[want] {
			t.Errorf("导出 zip 缺少 %s，实际: %v", want, names)
		}
	}
}

// TestZipSlipRejected 验证含 .. 的条目被拒绝。
func TestZipSlipRejected(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)

	zipPath := filepath.Join(tmp, "evil.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	fw, _ := zw.Create("../escape.lua")
	fw.Write([]byte("x"))
	zw.Close()
	zf.Close()

	if _, err := pc.ImportPackage(zipPath); err == nil {
		t.Fatal("含 zip-slip 路径的包应被拒绝")
	}
}

// TestLuaGetConfig 验证 Lua 里 setConfigDefault + getConfig 落到该插件的配置并读回。
func TestLuaGetConfig(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)
	// Lua game API 解析配置路径走的是包级全局 pluginController，测试里需同步设置其私有目录。
	pluginController.SetPluginDirs(tmp, tmp)

	var got []string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = append(got, txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	bm := &BotManager{}
	src := `
		game.setConfigDefault({x = 1})
		game.sendTargeted("alice", tostring(game.getConfig("x", 0)))
	`
	if err := LoadLuaPlugin(bm, "cfgtest", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if len(got) != 1 || got[0] != "1" {
		t.Fatalf("getConfig 返回值不符: %v", got)
	}
	// 配置确实写入磁盘
	var disk map[string]any
	data, _ := os.ReadFile(pc.ConfigPath("cfgtest"))
	if json.Unmarshal(data, &disk) != nil {
		t.Fatalf("配置 JSON 解析失败: %s", data)
	}
	if v, _ := disk["x"].(float64); v != 1 {
		t.Errorf("setConfigDefault 未写入磁盘: %v", disk)
	}
}

// 验证 handler 在临时 VM 触发时能读到插件配置（而非默认值）。
func TestLuaGetConfigInHandler(t *testing.T) {
	chatTriggers.Lock()
	chatTriggers.list = nil
	chatTriggers.Unlock()

	tmp := t.TempDir()
	// 先写一份被修改过的配置
	pluginController.SetPluginDirs(tmp, tmp)
	if err := pluginController.WriteConfig("cfgtest", map[string]any{"积分板": "经济"}); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}

	c := &captureHook{}
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { c.add(txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	bm := &BotManager{}
	src := `
		game.registerTrigger({"cfg"}, "onCfg")
		function onCfg(player, args)
			game.sendTargeted(player, "cfg=" .. game.getConfig("积分板", "money"))
		end
	`
	if err := LoadLuaPlugin(bm, "cfgtest", src); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	bm.handleChat("alice", "cfg")
	deadline := time.Now().Add(2 * time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.all(); len(got) != 1 || !strings.Contains(got[0], "经济") {
		t.Fatalf("handler 应读到修改后的配置(经济)，got %v", got)
	}
}
