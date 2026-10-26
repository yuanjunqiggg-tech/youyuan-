package main

import (
	"context"
	"strconv"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// luaExecTimeout 单个 LState 的最长执行时限。gopher-lua 每条指令都检查 ctx（vm.go mainLoopWithContext），
// 纯 Lua 死循环会被可靠中断；阻塞在 Go 侧调用（如 game.waitMsg 最长 30s）的 handler 在返回后中断。
// 2 分钟足以容纳正常会话交互，同时防止死循环永久占用 goroutine。测试可临时改小。
var luaExecTimeout = 2 * time.Minute

// luaBotCtxKey 用作 LState context 的键，保存"当前活跃机器人"，让 game.*/world.* 动态路由到
// 舰队中任意一个机器人（默认主机器人）。存 context 而非全局 map，随 LState 一起回收，无泄漏。
type luaBotCtxKey struct{}

// luaBotCtx 当前 Lua 状态的机器人上下文。
type luaBotCtx struct {
	main   *BotManager        // 默认主机器人
	active *BotManager        // 当前活跃机器人（bot.select 可切换）
	cancel context.CancelFunc // 执行超时定时器的取消函数（cancelLuaCtx 调用）
	parent context.Context    // plugin lifetime; background jobs must survive a handler returning
}

// setLuaBotCtx 在 LState 上记录主机器人并置 active=main，同时挂上执行超时。
func setLuaBotCtx(L *lua.LState, main *BotManager) {
	setLuaBotCtxParent(L, main, context.Background())
}
func setLuaBotCtxParent(L *lua.LState, main *BotManager, parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, luaExecTimeout)
	L.SetContext(context.WithValue(ctx, luaBotCtxKey{}, &luaBotCtx{main: main, active: main, cancel: cancel, parent: parent}))
}

// cancelLuaCtx 取消 LState 的执行超时定时器。状态关闭前调用，避免定时器残留到超时点。
func cancelLuaCtx(L *lua.LState) {
	if c := L.Context(); c != nil {
		if bc, ok := c.Value(luaBotCtxKey{}).(*luaBotCtx); ok && bc.cancel != nil {
			bc.cancel()
			bc.cancel = nil
		}
	}
}

// abot 返回当前 Lua 状态的活跃机器人（供 game.*/world.* 闭包动态解析）。
func abot(L *lua.LState) *BotManager {
	c := L.Context()
	if c == nil {
		return nil
	}
	if bc, ok := c.Value(luaBotCtxKey{}).(*luaBotCtx); ok {
		return bc.active
	}
	return nil
}

// selectBot 把 L 的活跃机器人切换为舰队中匹配 sel 的机器人（按 display/idStr 名字或数字下标）。
// 返回新活跃 bot；找不到返回 nil。
func selectBot(L *lua.LState, sel string) *BotManager {
	var found *BotManager
	if idx, err := strconv.Atoi(sel); err == nil && idx >= 1 {
		bots := GetFleetBots()
		if idx <= len(bots) {
			found = bots[idx-1]
		}
	} else {
		for _, b := range GetFleetBots() {
			if botName(b) == sel || b.idStr == sel {
				found = b
				break
			}
		}
	}
	if found == nil {
		return nil
	}
	if c := L.Context(); c != nil {
		if bc, ok := c.Value(luaBotCtxKey{}).(*luaBotCtx); ok {
			bc.active = found
		}
	}
	return found
}
