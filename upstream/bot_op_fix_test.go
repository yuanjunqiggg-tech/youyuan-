package main

import "testing"

// newOPTestBot 构造一个带缓冲 logCh 的 BotManager（setOP 会写 logCh，nil 会阻塞）。
func newOPTestBot() *BotManager {
	bm := &BotManager{}
	bm.logCh = make(chan string, 16)
	return bm
}

// 包检测已权威确认 OP（UpdateAbilities/AdventureSettings/AddPlayer 权限>=Host），
// 命令验证失败不应把 OP 降级掉——这是本次修复的核心回归点。
func TestOPPacketConfirmedSurvivesCommandFailure(t *testing.T) {
	bm := newOPTestBot()
	bm.setOP(true, 3, "UpdateAbilities") // 包来源确认 OP
	if !bm.IsOP() {
		t.Fatal("包检测确认 OP 后 IsOP() 应为 true")
	}
	// 模拟命令验证失败（gamemode creative 超时/成功数0）走守卫回退
	bm.resetOPIfUnconfirmed()
	if !bm.IsOP() {
		t.Fatal("命令验证失败不应覆盖包已确认的 OP")
	}
}

// 无包确认、命令验证失败 → 应保持非 OP（保留"真没 OP → 请求授权"流程）。
func TestOPUnconfirmedClearsOnCommandFailure(t *testing.T) {
	bm := newOPTestBot()
	bm.mu.Lock()
	bm.isOP = true // 仅命令路径设的 OP，无包确认
	bm.mu.Unlock()
	bm.resetOPIfUnconfirmed()
	if bm.IsOP() {
		t.Fatal("未包确认的 OP，命令失败后应回退为非 OP")
	}
}

// opPacketConfirmed 置位后，即使 isOP 被命令路径刷掉，IsOP() 仍应返回 true。
func TestIsOPReflectsPacketConfirmed(t *testing.T) {
	bm := newOPTestBot()
	bm.setOP(true, 3, "AddPlayer")
	bm.mu.Lock()
	bm.isOP = false // 模拟命令路径把 isOP 字段刷掉
	bm.mu.Unlock()
	if !bm.IsOP() {
		t.Fatal("opPacketConfirmed 应让 IsOP() 保持 true")
	}
}
