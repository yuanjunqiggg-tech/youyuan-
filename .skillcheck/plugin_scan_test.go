package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTestPluginFiles 在 tmpDir/插件文件/ 下写入一组测试插件文件。
func writeTestPluginFiles(t *testing.T, tmpDir string) {
	t.Helper()
	dir := filepath.Join(tmpDir, "插件文件")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll 失败: %v", err)
	}
	files := map[string]string{
		"hello.lua":         `game.registerTrigger({"hi"},"f") function f(player,args) end`,
		"hello.manifest.json": `{"id":"hello","name":"你好","version":"1.0","type":"lua","description":"测试插件","author":"prism"}`,
		"hello.docs.md":     `## 使用` + "\n\n输入 `hi` 触发。",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s 失败: %v", name, err)
		}
	}
}

// TestScanPlugins 验证 ScanPlugins 从磁盘扫描插件文件并构建带元数据的 PluginDef。
func TestScanPlugins(t *testing.T) {
	chatTriggers.Lock()
	chatTriggers.list = nil
	chatTriggers.Unlock()

	tmp := t.TempDir()
	writeTestPluginFiles(t, tmp)

	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)
	if err := pc.ScanPlugins(); err != nil {
		t.Fatalf("ScanPlugins 失败: %v", err)
	}

	st := pc.List()
	if len(st) != 1 {
		t.Fatalf("期望扫描到 1 个插件，got %d", len(st))
	}
	s := st[0]
	if s.ID != "hello" {
		t.Fatalf("期望 id=hello，got %q", s.ID)
	}
	if s.Name != "你好" {
		t.Fatalf("期望 name=你好（来自 manifest），got %q", s.Name)
	}
	if s.Type != "lua" {
		t.Fatalf("期望 type=lua，got %q", s.Type)
	}
	if s.Description != "测试插件" {
		t.Fatalf("期望 description=测试插件，got %q", s.Description)
	}
	if s.Docs == "" {
		t.Fatal("期望 docs 非空（来自 docs.md）")
	}
	// 重复扫描应跳过已注册插件
	if err := pc.ScanPlugins(); err != nil {
		t.Fatalf("第二次 ScanPlugins 失败: %v", err)
	}
	if got := len(pc.List()); got != 1 {
		t.Fatalf("期望重复扫描后仍为 1 个插件，got %d", got)
	}
}

// TestBuildDefFromFileWordBank 验证从词库 .json 构建 def，Load 后触发项以 id 归属。
func TestBuildDefFromFileWordBank(t *testing.T) {
	chatTriggers.Lock()
	chatTriggers.list = nil
	chatTriggers.Unlock()

	tmp := t.TempDir()
	dir := filepath.Join(tmp, "插件文件")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll 失败: %v", err)
	}
	wb := `{"菜单项":[{"说明":"示例","触发词":["看看"],"需要的参数(没有则填[])":[],"功能简介":"测试","触发后执行的指令":["say [玩家名]"]}]}`
	if err := os.WriteFile(filepath.Join(dir, "示例词库.json"), []byte(wb), 0644); err != nil {
		t.Fatalf("WriteFile 失败: %v", err)
	}

	def, err := BuildDefFromFile(filepath.Join(dir, "示例词库.json"))
	if err != nil {
		t.Fatalf("BuildDefFromFile 失败: %v", err)
	}
	if def.ID != "示例" {
		t.Fatalf("期望 id=示例（去掉词库后缀），got %q", def.ID)
	}
	if def.Type != "wordbank" {
		t.Fatalf("期望 type=wordbank，got %q", def.Type)
	}
	bm := &BotManager{}
	if err := def.Load(bm); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	chatTriggers.Lock()
	found := false
	for _, tr := range chatTriggers.list {
		if tr.Plugin == "示例" && len(tr.TriggerWords) == 1 && tr.TriggerWords[0] == "看看" {
			found = true
		}
	}
	chatTriggers.Unlock()
	if !found {
		t.Fatal("期望词库触发项已以 Plugin=示例 注册")
	}
}

// TestBuildDefFromFile 验证从单个 .lua 文件构建 def，且 Load 可在无连接下注册触发项。
func TestBuildDefFromFile(t *testing.T) {
	chatTriggers.Lock()
	chatTriggers.list = nil
	chatTriggers.Unlock()

	tmp := t.TempDir()
	writeTestPluginFiles(t, tmp)

	def, err := BuildDefFromFile(filepath.Join(tmp, "插件文件", "hello.lua"))
	if err != nil {
		t.Fatalf("BuildDefFromFile 失败: %v", err)
	}
	if def.ID != "hello" {
		t.Fatalf("期望 id=hello，got %q", def.ID)
	}
	if def.Type != "lua" {
		t.Fatalf("期望 type=lua，got %q", def.Type)
	}
	if def.Load == nil {
		t.Fatal("期望 Load 闭包非 nil")
	}
	bm := &BotManager{}
	if err := def.Load(bm); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got := len(bm.ActivePluginIDs()); got != 1 {
		t.Fatalf("期望注册 1 个插件触发项，got %d", got)
	}
}
