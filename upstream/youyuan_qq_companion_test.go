package main

import (
	"testing"
	"time"
)

// 回归：Begin 开头的全局清扫会遍历删除所有群的过期会话条目。
// 场景：A 在窗口内开始生成（genStart 早于自己的 Until），生成期间群里
// 又有人说话触发一次 Begin，此时 A 的窗口刚好过期 → A 的条目被清扫删掉。
// 旧实现 yyQQCompanionAlive 依赖 map 里还有这条会话，于是把已经算好的
// 答案当成「会话失效」丢掉（用户侧表现：AI 不回）。
// 修法：窗口终点由 Begin 显式返回，判定不再查 map。
func TestQQCompanionAliveSurvivesGlobalSweep(t *testing.T) {
	oldQQ, oldSessions, oldEpoch := yyQQ, yyQQCompanions.Sessions, yyQQCompanions.Epoch
	defer func() {
		yyQQ, yyQQCompanions.Sessions, yyQQCompanions.Epoch = oldQQ, oldSessions, oldEpoch
	}()
	yyQQCompanions.Sessions = map[string]yyQQCompanionSession{}
	yyQQCompanions.Epoch = map[string]uint64{}
	yyQQ = yyQQConfig{GroupID: "sweep", Enabled: true}
	yyQQ.Assistant.Companion = true
	yyQQ.Assistant.Model = "test"
	yyQQ.Assistant.CompanionAllMessages = true

	now := time.Now()
	// A 开口：窗口 30 秒（默认），Until = now+30s。
	key, epoch, _, _, ok, until := yyQQCompanionBegin(yyQQ, "a", "在吗", false, now)
	if !ok {
		t.Fatal("session not started")
	}
	if !until.After(now) {
		t.Fatal("Begin did not return window end")
	}
	// 生成开始于窗口内。
	genStart := now.Add(5 * time.Second)
	// 生成期间 B 说话触发一次 Begin，此刻已过 A 的 Until → 全局清扫删掉 A。
	if _, _, _, _, ok2, _ := yyQQCompanionBegin(yyQQ, "b", "有人吗", false, now.Add(31*time.Second)); !ok2 {
		t.Fatal("second session not started")
	}
	// A 的条目必然已被清扫（否则这个测试没有覆盖到缺陷路径）。
	yyQQCompanions.Lock()
	_, present := yyQQCompanions.Sessions[key]
	yyQQCompanions.Unlock()
	if present {
		t.Fatal("sweep did not remove expired session; test no longer covers the defect")
	}
	// 关键断言：开口时刻在窗口内，答案必须保留。
	if !yyQQCompanionAlive(yyQQ, key, epoch, until, genStart) {
		t.Fatal("in-flight answer dropped by unrelated sweep")
	}
	// 反向：开口时刻真的在窗口外，仍然必须丢。
	if yyQQCompanionAlive(yyQQ, key, epoch, until, until.Add(time.Second)) {
		t.Fatal("answer after window end accepted")
	}
}

// 回归：owner 中途进群清场（epoch+1）后，之前那条会话的答案必须丢。
func TestQQCompanionAliveRejectsEpochReset(t *testing.T) {
	oldQQ, oldSessions, oldEpoch := yyQQ, yyQQCompanions.Sessions, yyQQCompanions.Epoch
	defer func() {
		yyQQ, yyQQCompanions.Sessions, yyQQCompanions.Epoch = oldQQ, oldSessions, oldEpoch
	}()
	yyQQCompanions.Sessions = map[string]yyQQCompanionSession{}
	yyQQCompanions.Epoch = map[string]uint64{}
	yyQQ = yyQQConfig{GroupID: "epoch", Enabled: true}
	yyQQ.Assistant.Companion = true
	yyQQ.Assistant.Model = "test"
	yyQQ.Assistant.CompanionAllMessages = true
	yyQQ.Assistant.CompanionOwner = "owner"

	now := time.Now()
	key, epoch, _, _, ok, until := yyQQCompanionBegin(yyQQ, "a", "在吗", false, now)
	if !ok {
		t.Fatal("session not started")
	}
	// owner 进来清场。
	yyQQCompanionBegin(yyQQ, "owner", "都安静", false, now.Add(time.Second))
	if yyQQCompanionAlive(yyQQ, key, epoch, until, now.Add(2*time.Second)) {
		t.Fatal("answer survived owner epoch reset")
	}
}
