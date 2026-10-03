package main

import (
	"strings"
	"testing"
)

// 新扩展的 game_call 底层函数应被识别并路由到 switch（而非"未知 game 函数"）。
// 用假连接（connected+OP，但 conn=nil）进入 switch，方法会因无连接返回具体错误，
// 从而证明函数名映射存在。
func TestGameCallNewFunctionsRecognized(t *testing.T) {
	bm := &BotManager{}
	bm.logCh = make(chan string, 16)
	bm.mu.Lock()
	bm.connected = true
	bm.inGame = true
	bm.isOP = true
	bm.mu.Unlock()
	botMgrMu.Lock()
	botMgr = bm
	botMgrMu.Unlock()
	defer func() {
		botMgrMu.Lock()
		botMgr = nil
		botMgrMu.Unlock()
	}()

	newFns := []string{
		"getMultiScore", "setScore",
		"getPlayerItem", "queryInventory",
		"getTarget", "getTickingAreaList", "isCmdSuccess",
	}
	for _, fn := range newFns {
		args := map[string]any{"function": fn, "args": []any{"@a", "board", 1}}
		var s string
		func() {
			defer func() { _ = recover() }() // nil conn 若 panic 也不炸测试
			s, _ = toolGameCall(args)
		}()
		if strings.HasPrefix(s, "未知 game 函数") {
			t.Fatalf("函数 %s 未被识别: %q", fn, s)
		}
		if strings.Contains(s, "机器人") {
			t.Fatalf("函数 %s 被前置 OP/连接拦截（不应，已设 connected+OP）: %q", fn, s)
		}
	}

	// 未知函数仍应报"未知 game 函数"
	s, _ := toolGameCall(map[string]any{"function": "not_a_real_fn", "args": []any{}})
	if !strings.HasPrefix(s, "未知 game 函数") {
		t.Fatalf("未知函数应报未知，got %q", s)
	}
}
