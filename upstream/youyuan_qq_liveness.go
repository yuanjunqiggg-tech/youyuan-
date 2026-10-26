package main

// ═══════════════════════════════════════════════════════════════════════
// youyuan_qq_liveness.go —— QQ 侧「真的还活着吗」（2026-10-26 第 30 条）
//
// 【用户原话】「明明已经掉线5分钟了，但是他工具箱里面又没叫我刷新二维码，
// 又没叫我重新登录」
//
// 【根因：判据是假在线】之前的登录判据是
//      LoggedIn = WS 端口在监听 && 我们的 WebSocket 状态是 open
// QQ 侧掉登录时这两条**可能同时还是真的**：NapCat 的 TCP 不关、端口还在听，
// 只是它背后已经没有 QQ 会话了。于是面板一直显示「已登录，能力可直接使用」，
// 用户等 5 分钟也等不到任何扫码提示。
//
// 【修法】不看 socket，看**能不能真的问 QQ 一句话**：
// 定期发一次 get_login_info（一次真实往返），拿到 user_id 才算活着。
// 拿不到就判「假在线」，把最后一次成功的时间差暴露给面板，
// 超过 5 分钟（用户点名的时长）就升级成 need_scan，必须提示扫码。
//
// 这一层只做「探测 + 记录」，不负责重启协议端 —— 那是 watchdog 的事。
// ═══════════════════════════════════════════════════════════════════════

import (
	"sync"
	"time"
)

const (
	// yyQQLiveProbeTimeout 单次探测超时。掉线时 OneBot 可能既不回执也不关连接，
	// 用默认 15 秒会把探测拖成僵尸。
	yyQQLiveProbeTimeout = 3 * time.Second
	// yyQQLiveStaleAfter 多久没有成功回执就认为「不再活着」。
	// 比探测间隔留两轮余量，避免一次网络抖动就误判。
	yyQQLiveStaleAfter = 2 * time.Minute
	// yyQQOfflineEscalate 掉线多久必须主动叫用户扫码（用户明确说的 5 分钟）。
	yyQQOfflineEscalate = 5 * time.Minute
	// yyQQLiveFailToSuspect 连续失败几次才把「假在线」算实。
	yyQQLiveFailToSuspect = 2
)

type yyQQLiveSnapshot struct {
	OK      bool      // 最近一次探测是否成功
	EverOK  bool      // 有没有成功过一次（没探测过时用它决定「不下判断」）
	At      time.Time // 最近一次探测时间
	LastOK  time.Time // 最近一次成功时间
	// FirstFail 本轮连续失败是从什么时候开始的。
	// 为什么需要它：如果本次启动以来一次都没成功过（例如工具箱重启时机器人已经死了），
	// LastOK 是零值，「掉线多久」就永远算不出来，need_scan 永远不触发 ——
	// 正好是用户反馈的那个场景。这时用首次失败时间当基线。
	FirstFail time.Time
	Err       string
	Fails     int // 连续失败次数
	Nick      string
	UserID    string
}

var yyQQLive = struct {
	sync.Mutex
	v yyQQLiveSnapshot
}{}

// yyQQLiveProbeOnce 真正问 QQ 一句「你是谁」。成功即证明 QQ 会话还在。
//
// 用 get_login_info 而不是 get_status：前者必须经过 QQ 侧，
// 后者在部分实现里是本地返回、掉线了也照样有回执。
func yyQQLiveProbeOnce() (yyQQLiveSnapshot, error) {
	data, err := yyQQOneBotCallTimeout("get_login_info", map[string]any{}, yyQQLiveProbeTimeout)

	yyQQLive.Lock()
	defer yyQQLive.Unlock()
	now := time.Now()
	yyQQLive.v.At = now
	if err == nil {
		yyQQLive.v.OK = true
		yyQQLive.v.EverOK = true
		yyQQLive.v.LastOK = now
		yyQQLive.v.FirstFail = time.Time{}
		yyQQLive.v.Fails = 0
		yyQQLive.v.Err = ""
		if obj, ok := data.(map[string]any); ok {
			yyQQLive.v.Nick, _ = obj["nickname"].(string)
			yyQQLive.v.UserID = yyQQStableID(obj["user_id"])
		}
		return yyQQLive.v, nil
	}
	yyQQLive.v.OK = false
	yyQQLive.v.Fails++
	yyQQLive.v.Err = err.Error()
	if yyQQLive.v.FirstFail.IsZero() {
		yyQQLive.v.FirstFail = now
	}
	return yyQQLive.v, err
}

// yyQQLiveness 读探测结果（不发起调用，面板接口用它，不能阻塞）。
//
// 返回：
//
//	alive           —— 现在算活着吗
//	lastOKSeconds   —— 距上次成功多少秒（从未成功过返回 -1）
//	offlineSeconds  —— 距上次成功多少秒（同上；语义上是「掉线多久了」）
//	errText         —— 最近一次失败原因
//	everOK          —— 是否成功过（false 表示「还没探测过」，调用方应 fail-open）
//	fails           —— 连续失败次数
func yyQQLiveness() (alive bool, lastOKSeconds, offlineSeconds int, errText string, everOK bool, fails int) {
	yyQQLive.Lock()
	defer yyQQLive.Unlock()
	v := yyQQLive.v
	everOK = v.EverOK
	fails = v.Fails
	errText = v.Err
	// 基线：优先「最后一次成功」，没有就用「本轮第一次失败」。
	// 后者是为了覆盖「工具箱重启时机器人已经死了，一次都没成功过」的场景。
	base := v.LastOK
	if base.IsZero() {
		base = v.FirstFail
	}
	if base.IsZero() {
		return false, -1, -1, errText, everOK, fails
	}
	gap := int(time.Since(base).Seconds())
	if gap < 0 {
		gap = 0
	}
	lastOKSeconds, offlineSeconds = gap, gap
	alive = v.OK && time.Since(v.LastOK) <= yyQQLiveStaleAfter
	return alive, lastOKSeconds, offlineSeconds, errText, everOK, fails
}

// yyQQLiveReset 连接断开/重连时清掉旧结果，避免用上一次会话的时间差误导面板。
func yyQQLiveReset() {
	yyQQLive.Lock()
	defer yyQQLive.Unlock()
	yyQQLive.v.OK = false
	yyQQLive.v.EverOK = false
	yyQQLive.v.At = time.Time{}
	yyQQLive.v.LastOK = time.Time{}
	yyQQLive.v.Err = ""
	yyQQLive.v.Fails = 0
	yyQQLive.v.Nick = ""
	yyQQLive.v.UserID = ""
	yyQQLive.v.FirstFail = time.Time{}
}

// yyQQLiveProbeAsync 后台补一次探测。
//
// 面板每 20 秒轮询 /api/qq/login，但检测线程默认 60 秒一轮，
// 中间这一分钟面板看到的是旧数据。这里在「从来没有过数据」时补一次，
// 让新装的机器几秒内就能给出判断；有数据时不补，避免和检测线程抢连接、
// 也避免测试里被后台 goroutine 改状态。
func yyQQLiveProbeAsync() {
	yyQQLive.Lock()
	ever := yyQQLive.v.EverOK
	probing := yyQQLive.v.At.Add(30 * time.Second).After(time.Now())
	yyQQLive.Unlock()
	if ever || probing {
		return
	}
	go func() { yyQQLiveProbeOnce() }()
}

// yyQQLiveStatusMap 给 /api/qq/login 与诊断接口用的扁平 map。
func yyQQLiveStatusMap() map[string]any {
	alive, lastOK, offline, errText, everOK, fails := yyQQLiveness()
	yyQQLive.Lock()
	nick, uid := yyQQLive.v.Nick, yyQQLive.v.UserID
	yyQQLive.Unlock()
	out := map[string]any{
		"alive":           alive,
		"ever_ok":         everOK,
		"last_ok_seconds": lastOK,
		"offline_seconds": offline,
		"fails":           fails,
		"stale_after":     int(yyQQLiveStaleAfter.Seconds()),
		"escalate_after":  int(yyQQOfflineEscalate.Seconds()),
	}
	if errText != "" {
		out["error"] = errText
	}
	if nick != "" {
		out["nickname"] = nick
	}
	if uid != "" {
		out["user_id"] = uid
	}
	return out
}
