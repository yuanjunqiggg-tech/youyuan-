package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 接话/AI 失败原来只走 yyQQLog：那是 SSE 事件 + 内存环形缓冲，重启即丢，
// 前端默认也不显示。结果就是"群里问它为什么不回，谁也答不出来"。
// 这里再落一份盘（按 1MB 轮转，保留 .1/.2 两份历史），保证事后能查。
// 接话是每条消息一个 goroutine、主动搭话是每个群一个 goroutine，失败路径天然并发；
// 「判大小 → 轮转 → 追加」三步不加锁会出现半行日志和轮转竞态，所以这里必须串行。
var yyQQAIFailMu sync.Mutex

func yyQQAIFailLog(text string) {
	if yyQQPath == "" {
		return
	}
	yyQQAIFailMu.Lock()
	defer yyQQAIFailMu.Unlock()
	path := filepath.Join(filepath.Dir(yyQQPath), "qq-ai-errors.log")
	if info, err := os.Stat(path); err == nil && info.Size() > 1<<20 {
		// 保留两份历史：只留 .1 时，真失败容易被后续写入挤掉，
		// 复验的人只看主文件会误判成"零失败"。
		_ = os.Remove(path + ".2")
		_ = os.Rename(path+".1", path+".2")
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(time.Now().Format("2006-01-02 15:04:05") + " " + text + "\n")
}

type yyQQCompanionSession struct {
	Until    time.Time
	Sequence uint64
}

var yyQQCompanions = struct {
	sync.Mutex
	Sessions map[string]yyQQCompanionSession
	Epoch    map[string]uint64
	Next     uint64
}{Sessions: map[string]yyQQCompanionSession{}, Epoch: map[string]uint64{}}

// No polling or unsolicited repeat messages: only received events advance a session.
// 同时返回本次确定的窗口终点 Until：Begin 里那把全局清扫锁会删掉过期条目，
// 而「已经进入生成流程」的会话条目随时可能被别人的过期会话连带删掉，
// 所以窗口终点必须由 Begin 显式带出来，不能靠事后去 map 里查。
func yyQQCompanionBegin(cf yyQQConfig, user, text string, mentioned bool, now time.Time) (key string, epoch, sequence uint64, initial, active bool, until time.Time) {
	group := yyQQMessageKey(cf)
	key = group + "\x00" + user
	yyQQCompanions.Lock()
	defer yyQQCompanions.Unlock()
	for k, s := range yyQQCompanions.Sessions {
		if now.After(s.Until) {
			delete(yyQQCompanions.Sessions, k)
		}
	}
	if cf.Assistant.CompanionOwner != "" && user == cf.Assistant.CompanionOwner {
		yyQQCompanions.Epoch[group]++
		for k := range yyQQCompanions.Sessions {
			if strings.HasPrefix(k, group+"\x00") {
				delete(yyQQCompanions.Sessions, k)
			}
		}
		return
	}
	if !cf.Assistant.Companion || cf.Assistant.Model == "" {
		return
	}
	if strings.HasPrefix(text, ".") || strings.HasPrefix(text, "/") || text == "帮助" || text == "list" {
		return
	}
	old, exists := yyQQCompanions.Sessions[key]
	trigger := mentioned || cf.Assistant.CompanionAllMessages
	for _, word := range cf.Assistant.CompanionKeywords {
		if word != "" && strings.Contains(text, word) {
			trigger = true
		}
	}
	if !exists && !trigger {
		return
	}
	if !exists && len(yyQQCompanions.Sessions) >= 128 {
		return
	}
	initial = !exists
	yyQQCompanions.Next++
	old.Sequence = yyQQCompanions.Next
	old.Until = now.Add(time.Duration(yyQQCompanionWindowSeconds(cf)) * time.Second)
	yyQQCompanions.Sessions[key] = old
	return key, yyQQCompanions.Epoch[group], old.Sequence, initial, true, old.Until
}

// yyQQCompanionWindowSeconds 返回夹取后的接话窗口秒数。
// 玩家没设置（0）就用 30 秒兜底；夹到 5~600 秒，防止填 999999 让会话永不过期、
// 也防止填 1 秒等于没开。Begin 与首条等待窗口共用这一份规则，避免两处各算一遍。
func yyQQCompanionWindowSeconds(cf yyQQConfig) int {
	win := cf.Assistant.CompanionWindow
	if win <= 0 {
		win = 30
	}
	if win < 5 {
		win = 5
	}
	if win > 600 {
		win = 600
	}
	return win
}

// yyQQCompanionInitialWait 是会话首条消息开口前的等待：接话窗口的 40%，最多 10 秒。
// 以前是「无条件 sleep 5 秒」，醒来窗口（默认 30 秒，可被玩家调到 5 秒）早已过期，
// 于是每个会话的第一句 100% 不回。现在等待量随窗口缩放，永远不会超出窗口。
func yyQQCompanionInitialWait(cf yyQQConfig) time.Duration {
	wait := time.Duration(yyQQCompanionWindowSeconds(cf)) * time.Second * 2 / 5
	if wait > 10*time.Second {
		wait = 10 * time.Second
	}
	return wait
}

// yyQQCompanionAlive 报告「生成开始时」这次会话是否成立：配置未变、epoch 未清场、
// 且开口时刻仍在窗口内。与 yyQQCompanionCurrent 的区别是不要求「此刻」还在窗口内——
// AI 生成耗时导致的过期不该把已经算好的答案丢掉（用户侧表现就是「AI 不回」）。
// until 由 Begin 显式传入：判定不再依赖 Sessions map 里还有没有这条会话——
// Begin 开头的全局清扫会删掉过期条目，正在生成的会话可能被别人的过期条目连带删掉，
// 但开口时刻本该有效，所以「map 里没有」不等于「会话没成立」。
func yyQQCompanionAlive(cf yyQQConfig, key string, epoch uint64, until, at time.Time) bool {
	next := yyQQSnapshot()
	if !next.Enabled || !next.Assistant.Companion || next.Assistant.Model != cf.Assistant.Model ||
		next.Assistant.CompanionOwner != cf.Assistant.CompanionOwner || next.Official.AppID != cf.Official.AppID ||
		!yyQQGroupEnabled(next, yyQQGroupString(cf.GroupID)) {
		return false
	}
	yyQQCompanions.Lock()
	defer yyQQCompanions.Unlock()
	return at.Before(until) && yyQQCompanions.Epoch[yyQQMessageKey(cf)] == epoch
}
func yyQQCompanionCurrent(cf yyQQConfig, key string, epoch, sequence uint64, now time.Time) bool {
	yyQQCompanions.Lock()
	defer yyQQCompanions.Unlock()
	s, ok := yyQQCompanions.Sessions[key]
	return ok && now.Before(s.Until) && s.Sequence == sequence && yyQQCompanions.Epoch[yyQQMessageKey(cf)] == epoch
}
func yyQQCompanionHandle(cf yyQQConfig, user, text, id string, mentioned bool) bool {
	// AI 接话的三档模式在这里分流（详见 youyuan_qq_companion_aimode.go）：
	//   mention  —— 只有被叫才回，下面这套会话窗口原样保留；
	//   interval —— 由定时器自己翻聊天记录，不靠最近有人说话才启动；
	//   auto     —— 每条消息都判断一次要不要接。
	// 后两种模式必须让开会话窗口：否则同一条消息会被「会话接话」和「模式接话」
	// 回答两次。这里返回 false（而不是 true）是刻意的——返回值 true 会让上层
	// 直接 return，连 RelayChat 的「转发进游戏」也会被一起吞掉。
	yyQQAIModeLoopStart()
	if mode := yyQQAIMode(cf); mode != yyQQAIModeMention {
		if mode == yyQQAIModeAuto {
			yyQQAIModeAutoJudge(cf, user, text, id)
		}
		return false
	}
	key, epoch, seq, initial, ok, until := yyQQCompanionBegin(cf, user, text, mentioned, time.Now())
	if !ok {
		return false
	}
	go func() {
		current := func() bool {
			next := yyQQSnapshot()
			return next.Enabled && next.Assistant.Companion && next.Assistant.Model == cf.Assistant.Model && next.Assistant.CompanionOwner == cf.Assistant.CompanionOwner && next.Official.AppID == cf.Official.AppID && yyQQGroupEnabled(next, yyQQGroupString(cf.GroupID)) && yyQQCompanionCurrent(cf, key, epoch, seq, time.Now())
		}
		if initial {
			// 首条不立刻开口：等一个短窗口。窗口内同一人又说话，说明话还没说完，
			// 由那条消息自己的 goroutine 合并开口，本条安静退出，避免连回两句。
			deadline := time.Now().Add(yyQQCompanionInitialWait(cf))
			for time.Now().Before(deadline) {
				time.Sleep(200 * time.Millisecond)
				if !yyQQCompanionCurrent(cf, key, epoch, seq, time.Now()) {
					// 合并是设计行为，不是故障：用户已经拿到了第二条的回复。
					// 这里只走事件流，不写 qq-ai-errors.log —— 那个文件必须是"异常清单"，
					// 混进正常行为会让复验时还得额外过滤（GP-4/GP-5 一致结论）。
					yyQQLog(fmt.Sprintf("接话：第 %d 条在等待期间被接续（同一人又说话/配置变更），本条不单独回复：%s", seq, yyQQClip(text, 40)))
					return
				}
			}
		}
		if !current() {
			msg := fmt.Sprintf("接话：会话已失效（过期、被新消息接续或配置变更），已丢弃第 %d 条：%s", seq, yyQQClip(text, 40))
			yyQQLog(msg)
			yyQQAIFailLog("[" + yyQQGroupString(cf.GroupID) + "] " + msg)
			return
		}
		// Companion conversation never inherits administrator modification powers.
		chat := cf
		chat.Assistant.Admins = nil
		chat.Assistant.SuperAdmins = nil
		chat.Assistant.Prompt += "\n你是群里的智能助手，不能冒充服主本人。结合最近群聊自然回应当前发言，不要给每条回复加身份标签；通常用一两句话，只有明确的服务器查询才调用工具。"
		genStart := time.Now()
		answer, e := yyQQAskAIWithReplyGuard(chat, user, text, id, current)
		if e != nil {
			msg := "AI陪聊失败：" + e.Error()
			yyQQLog(msg)
			yyQQAIFailLog("[" + yyQQGroupString(cf.GroupID) + "] " + msg)
			return
		}
		// 只要开口那一刻窗口还在，生成耗时导致的过期不再丢答案。
		if !yyQQCompanionAlive(cf, key, epoch, until, genStart) {
			msg := fmt.Sprintf("接话：AI 已生成但会话在生成期间失效，已丢弃第 %d 条：%s", seq, yyQQClip(text, 40))
			yyQQLog(msg)
			yyQQAIFailLog("[" + yyQQGroupString(cf.GroupID) + "] " + msg)
			return
		}
		if strings.TrimSpace(answer) == "" {
			return
		}
		yyQQFeedbackTo(cf, answer, id)
	}()
	return true
}
