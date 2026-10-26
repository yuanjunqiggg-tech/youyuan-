package main

import (
	"sync"
	"time"
)

// 在线人数校准。
//
// 问题（用户反馈）：服务器里只有 1 个人，面板显示 4 人。
// 根因：playerInfoMgr.OnlineSet() 只看每个 PlayerInfo 的 Online 标记，
// 而这个标记靠 PlayerList 的 ADD/REMOVE 包维护。网易租赁服在玩家
// 静默断线、切维度、重连时并不总发 REMOVE，于是那些人永远停在 Online=true。
//
// 项目里已经有权威来源：player_info.go 的 RefreshPlayerRoster(bm)
// —— 发 `/list` 拿真实名单，再 applyConfirmedRoster 把名单外的人全部置为离线。
// 这里只做一件事：机器人连着的时候每 30 秒调一次，让 OnlineSet 保持真实。
// 不放在请求路径上，避免每次打开面板都要等一次命令往返。

const yyOnlineSyncInterval = 30 * time.Second

var yyOnlineSync = struct {
	sync.Mutex
	started bool
	lastAt  time.Time
	lastOK  bool
}{}

func yyOnlineSyncStart() {
	yyOnlineSync.Lock()
	if yyOnlineSync.started {
		yyOnlineSync.Unlock()
		return
	}
	yyOnlineSync.started = true
	yyOnlineSync.Unlock()

	go func() {
		// 启动后先等 8 秒，让机器人先完成握手与首次名单同步。
		time.Sleep(8 * time.Second)
		for {
			yyOnlineSyncOnce()
			time.Sleep(yyOnlineSyncInterval)
		}
	}()
}

// yyOnlineSyncOnce 校准一次。失败不改任何状态（宁可显示旧数据，也不要清空）。
func yyOnlineSyncOnce() {
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		return
	}
	ok := false
	func() {
		// 命令往返可能阻塞，加个兜底：任何 panic 都不能把后台 goroutine 带走。
		defer func() { _ = recover() }()
		ok = playerInfoMgr.RefreshPlayerRoster(bm)
	}()
	yyOnlineSync.Lock()
	yyOnlineSync.lastAt = time.Now()
	yyOnlineSync.lastOK = ok
	yyOnlineSync.Unlock()
}

// yyOnlineSyncState 给面板显示「这份在线人数有多新」。
func yyOnlineSyncState() map[string]any {
	yyOnlineSync.Lock()
	defer yyOnlineSync.Unlock()
	out := map[string]any{"ok": yyOnlineSync.lastOK, "interval_seconds": int(yyOnlineSyncInterval.Seconds())}
	if !yyOnlineSync.lastAt.IsZero() {
		out["checked_at"] = yyOnlineSync.lastAt.Format(time.RFC3339)
		out["age_seconds"] = int(time.Since(yyOnlineSync.lastAt).Seconds())
	}
	return out
}
