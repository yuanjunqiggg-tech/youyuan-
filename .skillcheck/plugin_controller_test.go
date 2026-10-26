package main

import (
	"testing"
	"time"
)

// 验证控制器 AddDef/List/Run/Stop/Reload/Delete 的运行状态流转（用注入的测试插件）。
func TestPluginControllerLifecycle(t *testing.T) {
	// 清理触发词注册表避免污染
	chatTriggers.Lock()
	chatTriggers.list = nil
	chatTriggers.Unlock()

	pc := NewPluginController()
	bm := &BotManager{}

	fake := &PluginDef{ID: "test", Name: "测试", Type: "lua", Description: "测试插件",
		Load: func(bm *BotManager) error {
			bm.AddChatTrigger(&ChatTrigger{Plugin: "test", TriggerWords: []string{"测试"}})
			return nil
		},
	}
	if err := pc.AddDef(fake); err != nil {
		t.Fatalf("AddDef 失败: %v", err)
	}

	st := pc.List()
	if len(st) != 1 {
		t.Fatalf("期望 1 个插件，got %d", len(st))
	}
	if st[0].Running {
		t.Fatal("期望初始未运行")
	}
	if st[0].State != "unloaded" {
		t.Fatalf("期望初始 State=unloaded，got %q", st[0].State)
	}
	// Run
	if err := pc.Run(bm, "test"); err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if s := pc.status("test"); !s.Running {
		t.Fatal("期望已运行")
	} else if s.State != "loaded" {
		t.Fatalf("期望 Run 后 State=loaded，got %q", s.State)
	}
	// 重复 Run
	if err := pc.Run(bm, "test"); err == nil {
		t.Fatal("期望重复 Run 报错")
	}
	// Stop
	if err := pc.Stop(bm, "test"); err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}
	if s := pc.status("test"); s.Running {
		t.Fatal("期望已停止")
	} else if s.State != "unloaded" {
		t.Fatalf("期望 Stop 后 State=unloaded，got %q", s.State)
	}
	// Reload
	if err := pc.Reload(bm, "test"); err != nil {
		t.Fatalf("Reload 失败: %v", err)
	}
	if s := pc.status("test"); !s.Running {
		t.Fatal("期望 Reload 后已运行")
	} else if s.State != "loaded" {
		t.Fatalf("期望 Reload 后 State=loaded，got %q", s.State)
	}
	// Delete
	if err := pc.Delete(bm, "test"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if s := pc.status("test"); s != nil {
		t.Fatal("期望 Delete 后已移除")
	}
}

// 验证 Load 返回错误时状态流转到 error 且 not running、Error 非空。
func TestPluginControllerLoadErrorState(t *testing.T) {
	pc := NewPluginController()
	bm := &BotManager{}

	bad := &PluginDef{ID: "bad", Name: "坏", Type: "lua", Description: "会失败的插件",
		Load: func(bm *BotManager) error {
			return &loadErr{}
		},
	}
	if err := pc.AddDef(bad); err != nil {
		t.Fatalf("AddDef 失败: %v", err)
	}
	if err := pc.Run(bm, "bad"); err == nil {
		t.Fatal("期望 Load 错误导致 Run 失败")
	}
	s := pc.status("bad")
	if s == nil {
		t.Fatal("期望插件仍在册")
	}
	if s.Running {
		t.Fatal("期望 error 后未运行")
	}
	if s.State != "error" {
		t.Fatalf("期望 State=error，got %q", s.State)
	}
	if s.Error == "" {
		t.Fatal("期望 Error 非空")
	}
}

// loadErr 满足 error 接口的最小错误类型。
type loadErr struct{}

func (e *loadErr) Error() string { return "模拟加载失败" }

// 验证触发词 Func panic 只杀死该次调用，不崩溃整个进程/测试。
func TestHandleChatFaultIsolation(t *testing.T) {
	chatTriggers.Lock()
	chatTriggers.list = nil
	chatTriggers.Unlock()

	bm := &BotManager{logCh: make(chan string, 100)}

	// 注册一个 Func 会 panic 的触发词
	bm.AddChatTrigger(&ChatTrigger{
		Plugin:       "boom",
		TriggerWords: []string{"boom"},
		Func: func(player string, args []any) {
			panic("boom")
		},
	})

	// 若没有 recover，这里 panic 会从 goroutine 蔓延到整个进程，测试会崩溃。
	bm.handleChat("steve", "boom")

	// 短暂等待异步 goroutine 跑完并完成 recover。
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	// 能走到这里即说明 handleChat 未崩溃、recover 生效。
	t.Log("触发词 panic 已被 recover 隔离，测试存活")
}
