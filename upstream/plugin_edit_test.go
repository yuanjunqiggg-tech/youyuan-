package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreatePluginLua 验证 Lua 插件创建：文件+manifest+注册。
func TestCreatePluginLua(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)

	id, err := pc.CreatePlugin("lua", "我的插件", "描述")
	if err != nil {
		t.Fatalf("CreatePlugin 失败: %v", err)
	}
	if id != "我的插件" {
		t.Errorf("id 应为名称本身，实际=%q", id)
	}
	// 代码文件
	if _, err := os.Stat(filepath.Join(tmp, "插件文件", id, id+".lua")); err != nil {
		t.Errorf("缺少 lua 代码文件: %v", err)
	}
	// manifest
	man := readPluginManifest(filepath.Join(tmp, "插件文件", id), id)
	if man == nil {
		t.Fatal("manifest 未生成")
	}
	if man.Name != "我的插件" || man.Description != "描述" || man.Type != "lua" || man.Version != "0.1.0" {
		t.Errorf("manifest 字段不符: %+v", man)
	}
	// 注册到控制器
	if pc.status(id) == nil {
		t.Errorf("插件未注册")
	}
}

// TestCreatePluginWordbank 验证词库插件：生成 词库.json，type=wordbank。
func TestCreatePluginWordbank(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)

	id, err := pc.CreatePlugin("wordbank", "欢迎词库", "")
	if err != nil {
		t.Fatalf("CreatePlugin 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "插件文件", id, id+"词库.json")); err != nil {
		t.Errorf("缺少词库代码文件: %v", err)
	}
	st := pc.status(id)
	if st == nil || st.Type != "wordbank" {
		t.Errorf("词库插件类型不符: %+v", st)
	}
}

// TestCreatePluginBadType 非法类型必须被拒。
func TestCreatePluginBadType(t *testing.T) {
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	if _, err := pc.CreatePlugin("evil", "x", ""); err == nil {
		t.Fatal("非法类型应报错")
	}
}

// TestCreatePluginIDSanitize 名称含非法字符 → id 去掉；重名 → 追加 -2。
func TestCreatePluginIDSanitize(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)

	id1, err := pc.CreatePlugin("lua", "测 试/插件?x", "")
	if err != nil {
		t.Fatalf("CreatePlugin1 失败: %v", err)
	}
	if id1 != "测试插件x" {
		t.Errorf("应去掉非法字符，实际=%q", id1)
	}
	id2, err := pc.CreatePlugin("lua", "测试插件x", "")
	if err != nil {
		t.Fatalf("CreatePlugin2 失败: %v", err)
	}
	if id2 != "测试插件x-2" {
		t.Errorf("重名应追加 -2，实际=%q", id2)
	}
}

// TestResolvePluginFilePathSafety 路径穿越必须被拒，合法路径解析正确。
func TestResolvePluginFilePathSafety(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)
	// 先造一个 lua 插件
	if _, err := pc.CreatePlugin("lua", "safe", ""); err != nil {
		t.Fatalf("CreatePlugin 失败: %v", err)
	}

	// 数据目录：非法文件名必须被拒
	for _, bad := range []string{"..", "../evil", "a/b", `a\b`, "..x", "a/../b"} {
		if _, err := pc.resolvePluginFile("data", "safe", bad); err == nil {
			t.Errorf("应拒绝非法文件名 %q", bad)
		}
	}
	// 合法数据文件名可解析
	p, err := pc.resolvePluginFile("data", "safe", "log.txt")
	if err != nil {
		t.Fatalf("合法文件名被拒: %v", err)
	}
	if filepath.Base(p) != "log.txt" || !strings.Contains(p, "插件数据文件") {
		t.Errorf("数据路径解析错误: %s", p)
	}

	// code scope：忽略 file 参数，返回代码文件
	p, err = pc.resolvePluginFile("code", "safe", "")
	if err != nil {
		t.Fatalf("code scope 解析失败: %v", err)
	}
	if filepath.Base(p) != "safe.lua" {
		t.Errorf("code 应解析到 safe.lua，实际=%s", p)
	}
}

// TestReadBinaryRejected 二进制文件读取必须报错。
func TestReadBinaryRejected(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)
	if _, err := pc.CreatePlugin("lua", "bin", ""); err != nil {
		t.Fatalf("CreatePlugin 失败: %v", err)
	}
	// 造一个含 NUL 的数据文件
	dataDir := pc.DataDir("bin")
	if err := os.WriteFile(filepath.Join(dataDir, "pic.dat"), []byte{0x89, 0x50, 0x00, 0x47}, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := pc.ReadPluginFile("data", "bin", "pic.dat"); err == nil {
		t.Fatal("二进制文件读取应报错")
	}
}

// TestPluginFileReadWriteRoundtrip 代码文件读写往返。
func TestPluginFileReadWriteRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)
	if _, err := pc.CreatePlugin("lua", "rw", ""); err != nil {
		t.Fatalf("CreatePlugin 失败: %v", err)
	}
	if err := pc.WritePluginFile("code", "rw", "", "print('hi')"); err != nil {
		t.Fatalf("WritePluginFile 失败: %v", err)
	}
	name, content, err := pc.ReadPluginFile("code", "rw", "")
	if err != nil {
		t.Fatalf("ReadPluginFile 失败: %v", err)
	}
	if name != "rw.lua" || content != "print('hi')" {
		t.Errorf("读写不符: name=%s content=%q", name, content)
	}
}
