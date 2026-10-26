package main

// QQ 存活探测与「必须扫码」升级的测试（2026-10-26 第 30 条）
//
// 用户原话：「明明已经掉线5分钟了，但是他工具箱里面又没叫我刷新二维码，
// 又没叫我重新登录」
//
// 这里守三件事：
//  1. 从没探测过时不许误判（fail-open），否则刚启动就红一片
//  2. 探测成功过后失效 → offline_seconds 要一直涨，且 logged_in 必须变 false
//  3. 掉线超过 5 分钟 → state 必须升到 need_scan，且 qr_missing 要如实标记
//     （没二维码文件时前端才知道该给「重新生成」按钮）

import (
	"testing"
	"time"
)

func yyTestLiveReset() {
	yyQQLiveReset()
}

// TestQQLivenessNeverProbed 从没探测过 = 不下判断。
func TestQQLivenessNeverProbed(t *testing.T) {
	yyTestLiveReset()
	defer yyTestLiveReset()
	alive, lastOK, offline, _, everOK, fails := yyQQLiveness()
	if everOK {
		t.Fatal("从没探测过不该 everOK")
	}
	if alive {
		t.Fatal("从没探测过不该判活着")
	}
	if lastOK != -1 || offline != -1 {
		t.Fatalf("没探测过应返回 -1，实际 lastOK=%d offline=%d", lastOK, offline)
	}
	if fails != 0 {
		t.Fatalf("初始失败次数应为 0，实际 %d", fails)
	}
}

// TestQQLivenessFreshSuccess 探测成功 → 活着。
func TestQQLivenessFreshSuccess(t *testing.T) {
	yyTestLiveReset()
	defer yyTestLiveReset()
	yyQQLive.Lock()
	yyQQLive.v.OK = true
	yyQQLive.v.EverOK = true
	yyQQLive.v.LastOK = time.Now()
	yyQQLive.v.Nick = "测试机器人"
	yyQQLive.Unlock()

	alive, lastOK, offline, errText, everOK, _ := yyQQLiveness()
	if !alive || !everOK {
		t.Fatalf("刚成功过应判活着：alive=%v everOK=%v", alive, everOK)
	}
	if lastOK < 0 || lastOK > 2 || offline < 0 || offline > 2 {
		t.Fatalf("时间差应接近 0，实际 lastOK=%d offline=%d", lastOK, offline)
	}
	if errText != "" {
		t.Fatalf("成功时不该有错误：%s", errText)
	}
}

// TestQQLivenessStale 成功过、之后探测失败且超过 stale 窗口 → 判掉线。
func TestQQLivenessStale(t *testing.T) {
	yyTestLiveReset()
	defer yyTestLiveReset()
	yyQQLive.Lock()
	yyQQLive.v.OK = false
	yyQQLive.v.EverOK = true
	yyQQLive.v.LastOK = time.Now().Add(-(yyQQLiveStaleAfter + 30*time.Second))
	yyQQLive.v.Fails = 2
	yyQQLive.v.Err = "OneBot 回执超时，结果未知"
	yyQQLive.Unlock()

	alive, lastOK, offline, errText, everOK, fails := yyQQLiveness()
	if !everOK {
		t.Fatal("曾经成功过，everOK 应为 true")
	}
	if alive {
		t.Fatal("超过 stale 窗口应判掉线")
	}
	if lastOK <= int(yyQQLiveStaleAfter.Seconds()) || offline <= int(yyQQLiveStaleAfter.Seconds()) {
		t.Fatalf("时间差应大于 stale 窗口，实际 lastOK=%d offline=%d", lastOK, offline)
	}
	if errText == "" || fails != 2 {
		t.Fatalf("失败原因与次数应保留：err=%q fails=%d", errText, fails)
	}
}

// TestQQLoginStateNeedScan 掉线超过 5 分钟 → state 必须是 need_scan，
// 且二维码文件不存在时 qr_missing=true（前端据此给「重新生成」按钮）。
func TestQQLoginStateNeedScan(t *testing.T) {
	yyTestLiveReset()
	t.Setenv("YY_NAPCAT_DIR", t.TempDir()) // 空目录 → 没有 qrcode.png
	defer yyTestLiveReset()

	yyQQLive.Lock()
	yyQQLive.v.OK = false
	yyQQLive.v.EverOK = true
	yyQQLive.v.LastOK = time.Now().Add(-(yyQQOfflineEscalate + 20*time.Second))
	yyQQLive.v.Fails = 3
	yyQQLive.v.Err = "OneBot 尚未连接"
	yyQQLive.Unlock()

	view := yyQQLoginSnapshot()
	if view.LoggedIn {
		t.Fatal("探测不到 QQ 会话时不许判已登录")
	}
	if !view.NeedScan {
		t.Fatal("掉线超过 5 分钟必须升到 need_scan")
	}
	if view.State != "need_scan" {
		t.Fatalf("state 应为 need_scan，实际 %q（hint=%q）", view.State, view.Hint)
	}
	if view.OfflineSeconds <= int(yyQQOfflineEscalate.Seconds()) {
		t.Fatalf("offline_seconds 应大于 300，实际 %d", view.OfflineSeconds)
	}
	if !view.QRMissing {
		t.Fatal("没有二维码文件时 qr_missing 必须为 true")
	}
	if view.Live {
		t.Fatal("Live 应为 false")
	}
	if view.Hint == "" {
		t.Fatal("need_scan 必须给中文提示，不能是空字符串")
	}
}

// TestQQLoginStateOfflineNotYetEscalated 掉线但没到 5 分钟 → 不许越过自愈直接逼人扫码。
func TestQQLoginStateOfflineNotYetEscalated(t *testing.T) {
	yyTestLiveReset()
	t.Setenv("YY_NAPCAT_DIR", t.TempDir())
	defer yyTestLiveReset()

	yyQQLive.Lock()
	yyQQLive.v.OK = false
	yyQQLive.v.EverOK = true
	yyQQLive.v.LastOK = time.Now().Add(-90 * time.Second) // 1.5 分钟
	yyQQLive.v.Fails = 1
	yyQQLive.v.Err = "OneBot 尚未连接"
	yyQQLive.Unlock()

	view := yyQQLoginSnapshot()
	if view.NeedScan {
		t.Fatalf("掉线 90 秒不该直接逼人扫码（自愈还没跑完）：%+v", view.State)
	}
}

// TestQQLoginStateNeverProbedFailOpen 从没探测过时必须 fail-open：
// 刚启动、探测线程还没跑第一轮时，不能把用户判成掉线。
func TestQQLoginStateNeverProbedFailOpen(t *testing.T) {
	yyTestLiveReset()
	t.Setenv("YY_NAPCAT_DIR", t.TempDir())
	defer yyTestLiveReset()

	view := yyQQLoginSnapshot()
	if view.NeedScan {
		t.Fatal("从没探测过不该升 need_scan")
	}
	if view.State == "need_scan" {
		t.Fatal("从没探测过 state 不该是 need_scan")
	}
	if view.LiveEverChecked {
		t.Fatal("从没探测过 LiveEverChecked 应为 false")
	}
}

// TestQQOfflineEscalateConstant 用户点名的是 5 分钟，别被人随手改。
func TestQQOfflineEscalateConstant(t *testing.T) {
	if yyQQOfflineEscalate != 5*time.Minute {
		t.Fatalf("升级阈值必须是 5 分钟（用户原话），实际 %v", yyQQOfflineEscalate)
	}
	if yyQQLiveProbeTimeout > 5*time.Second {
		t.Fatalf("探测超时必须短（否则拖住面板），实际 %v", yyQQLiveProbeTimeout)
	}
}
