package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDeleteRemovesFilesAndDisabled 验证删除：移除内存、清理磁盘文件与禁用记录，且无需连接。
func TestDeleteRemovesFilesAndDisabled(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)

	id, err := pc.CreatePlugin("lua", "待删插件", "")
	if err != nil {
		t.Fatalf("CreatePlugin 失败: %v", err)
	}
	pc.SetDisabled(id, true) // 写一条禁用记录
	// 造一个数据文件
	_ = os.WriteFile(filepath.Join(pc.DataDir(id), "data.txt"), []byte("x"), 0644)

	if err := pc.Delete(nil, id); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if pc.status(id) != nil {
		t.Error("Delete 后应已从列表移除")
	}
	if pc.IsDisabled(id) {
		t.Error("Delete 后应清除禁用记录")
	}
	// 磁盘上的三个目录都应被删除
	for _, sub := range []string{"插件文件", "插件配置文件", "插件数据文件"} {
		if _, err := os.Stat(filepath.Join(tmp, sub, id)); !os.IsNotExist(err) {
			t.Errorf("磁盘目录未删除: %s/%s", sub, id)
		}
	}
}

// stubDef 构造一个带空 Load 的插件定义（不依赖真实机器人连接）。
func stubDef(id string) *PluginDef {
	return &PluginDef{
		ID:   id,
		Name: id,
		Load: func(bm *BotManager) error { return nil },
	}
}

// TestSetDisabledPersist 验证禁用状态持久化到磁盘、重启后仍在。
func TestSetDisabledPersist(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)

	pc.SetDisabled("alpha", true)
	if !pc.IsDisabled("alpha") {
		t.Fatal("SetDisabled(true) 后应禁用")
	}

	// 用同一目录新建控制器，禁用状态应从磁盘恢复
	pc2 := NewPluginController()
	pc2.SetPluginDirs(tmp, tmp)
	if !pc2.IsDisabled("alpha") {
		t.Fatal("重启后禁用状态未恢复")
	}

	pc2.SetDisabled("alpha", false)
	if pc2.IsDisabled("alpha") {
		t.Fatal("SetDisabled(false) 后应启用")
	}
	pc3 := NewPluginController()
	pc3.SetPluginDirs(tmp, tmp)
	if pc3.IsDisabled("alpha") {
		t.Fatal("取消禁用后重启仍显示禁用")
	}
}

// TestAutoLoadSkipsDisabled 验证自动加载跳过禁用与已运行，加载未禁用未运行的。
func TestAutoLoadSkipsDisabled(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)

	if err := pc.AddDef(stubDef("a")); err != nil {
		t.Fatal(err)
	}
	if err := pc.AddDef(stubDef("b")); err != nil {
		t.Fatal(err)
	}
	if err := pc.AddDef(stubDef("c")); err != nil {
		t.Fatal(err)
	}
	pc.SetDisabled("b", true)

	// 先手动运行 a（模拟已在运行）
	if err := pc.Run(nil, "a"); err != nil {
		t.Fatalf("预运行 a 失败: %v", err)
	}

	pc.AutoLoadPlugins(nil)

	if st := pc.status("a"); st == nil || !st.Running {
		t.Error("a 应仍在运行（已运行，自动加载跳过但不停止）")
	}
	if st := pc.status("b"); st == nil || st.Running {
		t.Error("b 被禁用，不应自动加载")
	}
	if st := pc.status("c"); st == nil || !st.Running {
		t.Error("c 未禁用未运行，应被自动加载")
	}
}

// TestStopAllPlugins 验证断开停用只停止运行、不改禁用状态。
func TestStopAllPlugins(t *testing.T) {
	tmp := t.TempDir()
	pc := NewPluginController()
	pc.SetPluginDirs(tmp, tmp)
	if err := pc.AddDef(stubDef("x")); err != nil {
		t.Fatal(err)
	}
	if err := pc.AddDef(stubDef("y")); err != nil {
		t.Fatal(err)
	}
	pc.SetDisabled("y", true)

	// 手动运行 x（y 已禁用不运行）
	if err := pc.Run(nil, "x"); err != nil {
		t.Fatalf("运行 x 失败: %v", err)
	}

	pc.StopAllPlugins(nil)

	if st := pc.status("x"); st == nil || st.Running {
		t.Error("x 断开后应被停用")
	}
	if pc.IsDisabled("x") {
		t.Error("停用不应改变禁用状态（停用≠禁用）")
	}
	if !pc.IsDisabled("y") {
		t.Error("y 的禁用状态不应被断开停用影响")
	}
}
