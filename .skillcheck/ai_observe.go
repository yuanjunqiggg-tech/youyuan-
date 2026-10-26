package main

import (
	"sync"
	"time"
)

// ── AI 观察缓冲（wait_chat / listen_packet 共用骨架）──
//
// 模式：订阅数据源（聊天流 / 数据包）→ 把事件 feed 进缓冲 → 到窗口超时或达到
// max_count 时一次性返回给 AI。超时返回已收集内容（可为空），不阻塞收包循环。

type aiObserver struct {
	mu    sync.Mutex
	items []map[string]any
	done  bool
	max   int
	ch    chan struct{} // max 达成时通知（非阻塞，缓冲1）
}

// newAIObserver 创建观察缓冲；max<=0 表示不设上限（仅靠超时窗口结束）。
func newAIObserver(max int) *aiObserver {
	return &aiObserver{max: max, ch: make(chan struct{}, 1)}
}

// feed 投递一个事件；已结束（done）或已达 max 上限时丢弃。
func (o *aiObserver) feed(item map[string]any) {
	o.mu.Lock()
	if o.done {
		o.mu.Unlock()
		return
	}
	if o.max > 0 && len(o.items) >= o.max {
		o.mu.Unlock()
		return
	}
	o.items = append(o.items, item)
	reached := o.max > 0 && len(o.items) >= o.max
	o.mu.Unlock()
	if reached {
		select {
		case o.ch <- struct{}{}:
		default:
		}
	}
}

// wait 阻塞直到：达到 max_count，或超时窗口 seconds 结束。返回收集到的事件列表。
func (o *aiObserver) wait(seconds time.Duration) []map[string]any {
	deadline := time.After(seconds)
	for {
		o.mu.Lock()
		reached := o.done || (o.max > 0 && len(o.items) >= o.max)
		o.mu.Unlock()
		if reached {
			return o.drain()
		}
		select {
		case <-o.ch:
		case <-deadline:
			return o.drain()
		}
	}
}

// drain 结束观察并取出全部已收集事件（后续 feed 被忽略）。
func (o *aiObserver) drain() []map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.done = true
	out := o.items
	o.items = nil
	return out
}

// ── 聊天流订阅（AI wait_chat）──
// 与触发词/事件系统同源（都从收包循环的 Text 分支分发），但独立缓冲，不干扰它们。

type chatSub struct {
	id uint64
	cb func(bm *BotManager, player, msg string)
}

var chatHub = struct {
	sync.Mutex
	subs   []*chatSub
	nextID uint64
}{}

func chatHubSubscribe(cb func(bm *BotManager, player, msg string)) (unsub func()) {
	chatHub.Lock()
	defer chatHub.Unlock()
	chatHub.nextID++
	sub := &chatSub{id: chatHub.nextID, cb: cb}
	chatHub.subs = append(chatHub.subs, sub)
	return func() {
		chatHub.Lock()
		defer chatHub.Unlock()
		for i, s := range chatHub.subs {
			if s.id == sub.id {
				chatHub.subs = append(chatHub.subs[:i], chatHub.subs[i+1:]...)
				return
			}
		}
	}
}

// chatHubDispatch 收包循环收到聊天时调用，把消息广播给订阅者。
func chatHubDispatch(bm *BotManager, player, msg string) {
	chatHub.Lock()
	subs := make([]*chatSub, len(chatHub.subs))
	copy(subs, chatHub.subs)
	chatHub.Unlock()
	for _, s := range subs {
		s.cb(bm, player, msg)
	}
}

