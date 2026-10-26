package main

import (
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

// 回归：同一条群消息（同一 message_id）在机器人重连/重启之后被适配器再推一次，
// 触发词动作不能再执行第二遍。旧实现只有进程内的 yyQQRecent，重启即清空，
// 这就是「重连后动作又执行一次」的根因；现在认领表落盘。

func yyQQClaimTestResetTrigger() {
	yyQQTriggerState.Lock()
	yyQQTriggerState.last = map[string]time.Time{}
	yyQQTriggerState.used = map[string]int{}
	yyQQTriggerState.day = ""
	yyQQTriggerState.Unlock()
}

func yyQQClaimTestUsed(id, user string) int {
	yyQQTriggerState.Lock()
	defer yyQQTriggerState.Unlock()
	return yyQQTriggerState.used[id+"\x00"+user]
}

func yyQQClaimTestConfig() yyQQConfig {
	cf := yyQQConfig{Enabled: true, Adapter: "onebot", GroupID: "g-1", Groups: []yyQQGroup{{ID: "g-1", Enabled: true}}}
	cf.OneBot.URL = "ws://127.0.0.1:9/"
	cf.Assistant.Enabled = true
	cf.Assistant.Triggers = []yyQQTrigger{{
		ID: "t-like", Enabled: true, Name: "赞我", Type: "keyword", Patterns: []string{"赞我"},
		Actions: []yyQQTriggerAction{{Type: "reply", Params: map[string]string{"text": "收到"}}},
	}}
	return cf
}

// TestQQInboundClaimSurvivesRestart 认领表本身跨进程有效：新实例从同一目录装载后
// 依然认得旧消息；没见过的消息照常放行。
func TestQQInboundClaimSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qq-inbound-claims.json")
	cf := yyQQClaimTestConfig()
	first := newYYQQClaimStore(path)
	if !first.claim(yyQQClaimKey(cf, "m-1")) {
		t.Fatal("首次出现的消息应放行")
	}
	restarted := newYYQQClaimStore(path) // 模拟重启后的空内存 + 读盘
	if restarted.claim(yyQQClaimKey(cf, "m-1")) {
		t.Fatal("重启后同一条消息不应再放行")
	}
	if !restarted.claim(yyQQClaimKey(cf, "m-2")) {
		t.Fatal("新消息应放行")
	}
	// 换群 = 不同的消息空间，同 id 不能被跨群误挡。
	other := cf
	other.GroupID = "g-2"
	if !restarted.claim(yyQQClaimKey(other, "m-1")) {
		t.Fatal("另一个群里的同号消息应放行")
	}
}

// TestQQTriggerActionNotReplayedAfterReconnect 走真实入口：一条触发词消息执行一次，
// 模拟重启（内存全丢、只剩落盘的已处理 id）后同一条消息重推，动作不得再执行。
func TestQQTriggerActionNotReplayedAfterReconnect(t *testing.T) {
	dir := t.TempDir()
	oldQQ, oldClaims := yyQQ, yyQQClaims
	oldGroups := yyQQMessages.Groups
	t.Cleanup(func() {
		yyQQ, yyQQClaims = oldQQ, oldClaims
		yyQQMessages.Groups = oldGroups
		yyQQClaimTestResetTrigger()
	})
	yyQQMessages.Groups = map[string][]yyQQGroupMessage{}
	yyQQClaimTestResetTrigger()
	yyQQClaimsInit(dir)

	cf := yyQQClaimTestConfig()
	yyQQ = cf

	yyQQToGameMentionMeta(cf, "小红", "u-1", "member", "赞我", "m-1", false, "", false)
	if got := yyQQClaimTestUsed("t-like", "u-1"); got != 1 {
		t.Fatalf("第一次应执行一次，实际 %d", got)
	}

	// 重启/重连：内存里的冷却与去重全丢，只有落盘的认领表还在。
	yyQQClaimTestResetTrigger()
	yyQQClaims = newYYQQClaimStore(filepath.Join(dir, "qq-inbound-claims.json"))

	yyQQToGameMentionMeta(cf, "小红", "u-1", "member", "赞我", "m-1", false, "", false)
	if got := yyQQClaimTestUsed("t-like", "u-1"); got != 0 {
		t.Fatalf("重连后同一条消息不应再次执行动作，实际执行 %d 次", got)
	}

	// 新消息不能被幂等表误伤。
	yyQQToGameMentionMeta(cf, "小红", "u-1", "member", "赞我", "m-2", false, "", false)
	if got := yyQQClaimTestUsed("t-like", "u-1"); got != 1 {
		t.Fatalf("新消息应照常执行一次，实际 %d", got)
	}
}

// TestQQInboundClaimCapacityAndExpire 有界 + 过期：容量上限 500，超了淘汰最旧；
// 超过 24 小时的旧 id 允许再次认领（避免表被永久占满）。
func TestQQInboundClaimCapacityAndExpire(t *testing.T) {
	store := newYYQQClaimStore("")
	for i := 0; i < yyQQClaimMax; i++ {
		if !store.claim("k-" + strconv.Itoa(i)) {
			t.Fatal("首次出现的消息都应放行")
		}
	}
	store.mu.Lock()
	size := len(store.items)
	orderLen := len(store.order)
	store.mu.Unlock()
	if size != yyQQClaimMax || orderLen != yyQQClaimMax {
		t.Fatalf("容量应为 %d，实际 items=%d order=%d", yyQQClaimMax, size, orderLen)
	}

	// 再塞一条：总量不涨，最旧一条被淘汰后可再次认领。
	oldest := ""
	store.mu.Lock()
	for _, key := range store.order {
		oldest = key
		break
	}
	store.mu.Unlock()
	if !store.claim("fresh") {
		t.Fatal("表满后新消息不应被丢弃")
	}
	if !store.claim(oldest) {
		t.Fatal("超过容量的最旧一条应被淘汰（可再次认领）")
	}

	// 过期：把一条记成 25 小时前，应允许重新认领。
	store.mu.Lock()
	store.items["stale"] = time.Now().Add(-25 * time.Hour).Unix()
	store.order = append(store.order, "stale")
	store.mu.Unlock()
	if !store.claim("stale") {
		t.Fatal("超过 24 小时的消息 id 应允许再次认领")
	}
}

// TestQQInboundClaimConcurrentSingleWinner 多条事件并发到达（读循环每条一个 goroutine），
// 同一条消息只能有一个 winner。
func TestQQInboundClaimConcurrentSingleWinner(t *testing.T) {
	store := newYYQQClaimStore("")
	const workers = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if store.claim("same-id") {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()
	if wins != 1 {
		t.Fatalf("同一条消息应只有一个 winner，实际 %d", wins)
	}
}
