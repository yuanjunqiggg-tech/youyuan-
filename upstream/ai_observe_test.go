package main

import (
	"strings"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestAIObserverMaxReturnsEarly(t *testing.T) {
	obs := newAIObserver(2)
	go func() {
		time.Sleep(50 * time.Millisecond)
		obs.feed(map[string]any{"player": "a", "message": "1"})
		time.Sleep(20 * time.Millisecond)
		obs.feed(map[string]any{"player": "b", "message": "2"})
	}()
	// 达 max=2 应立即返回，远早于 5s 超时
	start := time.Now()
	items := obs.wait(5 * time.Second)
	if time.Since(start) > 2*time.Second {
		t.Fatalf("达 max_count 应提前返回，耗时 %v", time.Since(start))
	}
	if len(items) != 2 {
		t.Fatalf("应收集 2 条，got %d", len(items))
	}
}

func TestAIObserverTimeoutReturnsPartial(t *testing.T) {
	obs := newAIObserver(0)
	go func() {
		time.Sleep(50 * time.Millisecond)
		obs.feed(map[string]any{"player": "a"})
	}()
	items := obs.wait(300 * time.Millisecond)
	if len(items) != 1 {
		t.Fatalf("超时应返回已收集 1 条，got %d", len(items))
	}
	// 结束后再 feed 应被丢弃
	obs.feed(map[string]any{"player": "z"})
	if len(obs.drain()) != 0 {
		t.Fatal("结束后 feed 应被丢弃")
	}
}

func TestToolWaitChatCollects(t *testing.T) {
	bm := &BotManager{}
	type res struct {
		s     string
		isErr bool
	}
	ch := make(chan res, 1)
	go func() {
		s, e := toolWaitChat(map[string]any{"seconds": 5, "max_count": 2})
		ch <- res{s, e}
	}()
	time.Sleep(100 * time.Millisecond)
	chatHubDispatch(bm, "alice", "hello")
	chatHubDispatch(bm, "bob", "菜单已打开")
	r := <-ch
	if r.isErr {
		t.Fatal("wait_chat 不应报错")
	}
	if !strings.Contains(r.s, "alice") || !strings.Contains(r.s, "菜单已打开") {
		t.Fatalf("应收集到两条消息，got %s", r.s)
	}
}

func TestToolListenPacketCollects(t *testing.T) {
	bm := &BotManager{connected: true}
	botMgrMu.Lock()
	oldBot := botMgr
	botMgr = bm
	botMgrMu.Unlock()
	defer func() { botMgrMu.Lock(); botMgr = oldBot; botMgrMu.Unlock() }()
	type res struct {
		s     string
		isErr bool
	}
	ch := make(chan res, 1)
	go func() {
		s, e := toolListenPacket(map[string]any{"packet_type": "SetTime", "seconds": 5, "max_count": 1})
		ch <- res{s, e}
	}()
	time.Sleep(100 * time.Millisecond)
	packetHubDispatch(bm, &packet.SetTime{Time: 999})
	r := <-ch
	if r.isErr {
		t.Fatal("listen_packet 不应报错")
	}
	if !strings.Contains(r.s, "999") {
		t.Fatalf("应收到 SetTime time=999，got %s", r.s)
	}
}
