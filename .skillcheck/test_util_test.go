package main

import "sync"

// captureHook 并发安全的测试捕获器：异步派发的 handler 在 goroutine 里写入，
// 测试主协程轮询读取——不加锁在 -race 下会报数据竞争。
type captureHook struct {
	mu  sync.Mutex
	got []string
}

func (c *captureHook) add(s string) {
	c.mu.Lock()
	c.got = append(c.got, s)
	c.mu.Unlock()
}

func (c *captureHook) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.got)
}

func (c *captureHook) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.got...)
}
