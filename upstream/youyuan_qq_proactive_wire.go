package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// 主动搭话接线层。
//
// youyuan_qq_proactive.go 只回答「要不要说、为什么说」（打分 + 冷却 + 日限），
// 本文件回答「什么时候轮询、让谁说、说什么、发到哪」。
// 此前 Init/Tick/OnMessage/Routes 在全仓零调用，整套能力是死代码，
// 线上连 GET /api/qq/proactive 都是 404。

// yyProactiveTickEvery 轮询间隔。打分窗口是 30 分钟，取窗口的 1/60 即 30 秒：
// 静默分是连续量，30 秒的判定精度足够；冷却与日限都是硬约束，高频判定也不会被绕过。
const yyProactiveTickEvery = yyProactiveWindow / 60

var yyProactiveLoop = struct {
	sync.Mutex
	Stop chan struct{}
}{}

// yyProactiveStart 起 tick 循环，重复调用只起一次。
// 进程退出（main 走 os.Exit）即随之结束；需要显式停时调 yyProactiveStopLoop。
func yyProactiveStart() {
	yyProactiveLoop.Lock()
	defer yyProactiveLoop.Unlock()
	if yyProactiveLoop.Stop != nil {
		return
	}
	stop := make(chan struct{})
	yyProactiveLoop.Stop = stop
	go func() {
		ticker := time.NewTicker(yyProactiveTickEvery)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				yyProactiveSweep()
			}
		}
	}()
}

// yyProactiveStopLoop 停掉 tick 循环（服务关闭或测试收尾用）。
func yyProactiveStopLoop() {
	yyProactiveLoop.Lock()
	defer yyProactiveLoop.Unlock()
	if yyProactiveLoop.Stop != nil {
		close(yyProactiveLoop.Stop)
		yyProactiveLoop.Stop = nil
	}
}

// yyProactiveSweep 跑一轮打分，把「该开口」的群交给 AI 生成内容。
func yyProactiveSweep() {
	cf := yyQQSnapshot()
	// 群服互通没开、助手没开、模型没配时连打分都不做：
	// yyProactiveTick 会认领发言额度（写 last_proactive_ts 与日计数），
	// 空跑只会白烧冷却和日限，用户真打开后要等半小时才有机会说话。
	if !cf.Enabled || !cf.Assistant.Enabled || cf.Assistant.Model == "" {
		return
	}
	if aiStore.Get(cf.Assistant.Model) == nil {
		return
	}
	// OneBot 没连上时不要打分：tick 会认领发言额度，发不出去等于白烧一次冷却，
	// 用户看到的就是「开了主动搭话却一直不说话」。
	if cf.Adapter == "onebot" {
		yyQQMu.Lock()
		state := yyQQState
		yyQQMu.Unlock()
		if state != "open" {
			return
		}
	}
	for _, prompt := range yyProactiveTick(time.Now()) {
		prompt := prompt
		go yyProactiveSpeak(cf, prompt)
	}
}

// yyProactiveSpeak 生成并发送一条主动发言。prompt 由 yyProactiveTick 产出，
// 产出时已经认领过本次发言（冷却与日限已记账），所以生成失败也不会重复轰炸。
func yyProactiveSpeak(cf yyQQConfig, prompt yyProactivePrompt) {
	target := cf
	target.GroupID = prompt.Group
	// 主动开口不继承管理员能力：没人盯着的时候不该动服务器。
	target.Assistant.Admins = nil
	target.Assistant.SuperAdmins = nil
	target.Assistant.Prompt += "\n现在是你主动开口，不是回应用户：只说一句，不要反问式追问，不要提「打分」「提示」「系统」这类内部信息。"
	ask := "（主动搭话机会）" + prompt.Reason + "\n" + prompt.Hint +
		"\n请直接输出一句能发到群里的话，不超过 40 字，不要加引号或前缀，不要 @ 任何人。"
	answer, e := yyQQAskAIWithReplyGuard(target, "proactive", ask, "", nil)
	if e != nil {
		yyQQLog("主动搭话生成失败：" + e.Error())
		return
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return
	}
	if len([]rune(answer)) > 200 {
		answer = string([]rune(answer)[:200])
	}
	next := yyQQSnapshot()
	if !next.Enabled || !yyQQGroupEnabled(next, prompt.Group) {
		yyQQLog("主动搭话已取消：目标群已停用 " + prompt.Group)
		return
	}
	if e := yyQQSendToGroup(target, answer, ""); e != nil {
		yyQQLog(fmt.Sprintf("主动搭话发送失败（群 %s）：%s", prompt.Group, e.Error()))
		return
	}
	yyQQLog("主动搭话已发送（群 " + prompt.Group + "）：" + answer)
}
