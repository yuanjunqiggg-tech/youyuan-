package main

// 生图队列测试。生图函数整体替换成测试替身，不连网、不动全局生图配置，
// 也不往任何真实会话里发消息；队列实例由每个用例自建自关。

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// yyImgCall 一次生图调用的记录。
type yyImgCall struct {
	qq        int64
	prompt    string
	useAvatar bool
}

// yyImgRecorder 记录回调与生图调用。回调跑在 worker goroutine 上，全部加锁。
type yyImgRecorder struct {
	mu      sync.Mutex
	replies []string
	images  []string
	calls   []yyImgCall
}

func (r *yyImgRecorder) reply(text string) {
	r.mu.Lock()
	r.replies = append(r.replies, text)
	r.mu.Unlock()
}

func (r *yyImgRecorder) sendImage(ref string) error {
	r.mu.Lock()
	r.images = append(r.images, ref)
	r.mu.Unlock()
	return nil
}

// gen 包一层调用记录；next 为空时直接返回一张以 prompt 命名的图。
func (r *yyImgRecorder) gen(next yyImgGenFunc) yyImgGenFunc {
	return func(ctx context.Context, qq int64, prompt string, useAvatar bool) (string, error) {
		r.mu.Lock()
		r.calls = append(r.calls, yyImgCall{qq: qq, prompt: prompt, useAvatar: useAvatar})
		r.mu.Unlock()
		if next == nil {
			return "img://" + prompt, nil
		}
		return next(ctx, qq, prompt, useAvatar)
	}
}

func (r *yyImgRecorder) snapshot() ([]string, []string, []yyImgCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.replies...), append([]string(nil), r.images...), append([]yyImgCall(nil), r.calls...)
}

func (r *yyImgRecorder) lastReply() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.replies) == 0 {
		return ""
	}
	return r.replies[len(r.replies)-1]
}

// yyImgWaitFor 轮询等待条件成立，最多 5 秒，避免测试挂死。
func yyImgWaitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("等待超时：%s", what)
}

// TestYYImgQueueFIFOOrder：先派一个占位任务把唯一 worker 占住，再连排两条，
// 放开后必须按提交顺序出图；入队回复要带队列位置。
func TestYYImgQueueFIFOOrder(t *testing.T) {
	rec := &yyImgRecorder{}
	gate := make(chan struct{})
	q := newYYImgQueue(1, 5*time.Second, 1, rec.gen(func(ctx context.Context, qq int64, prompt string, useAvatar bool) (string, error) {
		<-gate
		return "img://" + prompt, nil
	}))
	t.Cleanup(q.Close)

	if e := q.Submit("g1", 0, "占位", false, rec.reply, rec.sendImage); e != nil {
		t.Fatal(e)
	}
	yyImgWaitFor(t, "占位任务开始执行", func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.calls) == 1
	})
	// 占位已被 worker 取走，队列是空的：画猫排第 1，画狗排第 2。
	if e := q.Submit("g1", 1, "画猫", false, rec.reply, rec.sendImage); e != nil {
		t.Fatal(e)
	}
	if e := q.Submit("g1", 2, "画狗", false, rec.reply, rec.sendImage); e != nil {
		t.Fatal(e)
	}
	close(gate)
	yyImgWaitFor(t, "三张图都发完", func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.images) == 3
	})

	replies, images, calls := rec.snapshot()
	wantOrder := []string{"占位", "画猫", "画狗"}
	if len(calls) != 3 {
		t.Fatalf("生图调用次数不对：%+v", calls)
	}
	for i, want := range wantOrder {
		if calls[i].prompt != want {
			t.Fatalf("第 %d 个执行应是 %q，实际 %q（全部 %+v）", i, want, calls[i].prompt, calls)
		}
	}
	for i, want := range wantOrder {
		if images[i] != "img://"+want {
			t.Fatalf("第 %d 张图应是 %q，实际 %q", i, "img://"+want, images[i])
		}
	}
	wantReplies := []string{"🎨 开始画（队列 1）…", "🎨 开始画（队列 1）…", "🎨 开始画（队列 2）…"}
	if len(replies) != len(wantReplies) {
		t.Fatalf("入队回复条数不对：%v", replies)
	}
	for i, want := range wantReplies {
		if replies[i] != want {
			t.Fatalf("第 %d 条回复应是 %q，实际 %q", i, want, replies[i])
		}
	}
}

// TestYYImgQueueCoalesceSameUser：同一用户在前任务还没画完时连发两条，
// 必须合并成最新一条：只生成一次、prompt 是最新的、回复「已更新为最新请求」。
func TestYYImgQueueCoalesceSameUser(t *testing.T) {
	rec := &yyImgRecorder{}
	gate := make(chan struct{})
	q := newYYImgQueue(1, 5*time.Second, 1, rec.gen(func(ctx context.Context, qq int64, prompt string, useAvatar bool) (string, error) {
		<-gate
		return "img://" + prompt, nil
	}))
	t.Cleanup(q.Close)

	// 占位任务把唯一 worker 占住，保证同一用户的两条都停在队列里可合并。
	if e := q.Submit("g1", 0, "占位", false, rec.reply, rec.sendImage); e != nil {
		t.Fatal(e)
	}
	yyImgWaitFor(t, "占位任务开始执行", func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.calls) == 1
	})
	if e := q.Submit("g1", 7, "画猫", true, rec.reply, rec.sendImage); e != nil {
		t.Fatal(e)
	}
	if e := q.Submit("g1", 7, "画狗", false, rec.reply, rec.sendImage); e != nil {
		t.Fatal(e)
	}
	close(gate)
	yyImgWaitFor(t, "占位与合并后的图都发完", func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.images) == 2
	})

	replies, images, calls := rec.snapshot()
	userCalls := 0
	for _, c := range calls {
		if c.qq != 7 {
			continue
		}
		userCalls++
		if c.prompt != "画狗" {
			t.Fatalf("合并后应只画最新 prompt，实际 %q", c.prompt)
		}
		if c.useAvatar {
			t.Fatal("合并后 useAvatar 应换成最新请求的 false")
		}
	}
	if userCalls != 1 {
		t.Fatalf("同一用户应只生成一次，实际 %d 次（全部 %+v）", userCalls, calls)
	}
	if len(images) != 2 || images[1] != "img://画狗" {
		t.Fatalf("应只发占位图加最新图：%v", images)
	}
	joined := strings.Join(replies, "|")
	if !strings.Contains(joined, "已更新为最新请求") {
		t.Fatalf("合并时应回复「已更新为最新请求」：%v", replies)
	}
}

// TestYYImgQueueTimeoutCancels：生图卡住时单任务超时必须生效，重试一次后
// 仍超时就把原因回给用户，且不出图。
func TestYYImgQueueTimeoutCancels(t *testing.T) {
	rec := &yyImgRecorder{}
	q := newYYImgQueue(1, 80*time.Millisecond, 1, rec.gen(func(ctx context.Context, qq int64, prompt string, useAvatar bool) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
			return "img://slow", nil
		}
	}))
	t.Cleanup(q.Close)

	if e := q.Submit("g1", 7, "画慢图", false, rec.reply, rec.sendImage); e != nil {
		t.Fatal(e)
	}
	yyImgWaitFor(t, "收到超时失败回复", func() bool {
		return strings.Contains(rec.lastReply(), "图没画出来")
	})

	_, images, calls := rec.snapshot()
	if len(images) != 0 {
		t.Fatalf("超时不应出图：%v", images)
	}
	if len(calls) != 2 {
		t.Fatalf("超时后应重试一次共 2 次调用，实际 %d", len(calls))
	}
	if !strings.Contains(rec.lastReply(), context.DeadlineExceeded.Error()) {
		t.Fatalf("失败回复应说明超时原因：%q", rec.lastReply())
	}
}

// TestYYImgQueueRetryOnceThenSuccess：第一次失败后自动重试，第二次成功要出图，
// 且不能把失败原因透给用户。
func TestYYImgQueueRetryOnceThenSuccess(t *testing.T) {
	rec := &yyImgRecorder{}
	var fails atomic.Int32
	q := newYYImgQueue(1, 5*time.Second, 1, rec.gen(func(ctx context.Context, qq int64, prompt string, useAvatar bool) (string, error) {
		if fails.Add(1) == 1 {
			return "", fmt.Errorf("服务临时打嗝")
		}
		return "img://retry-ok", nil
	}))
	t.Cleanup(q.Close)

	if e := q.Submit("g1", 7, "画猫", false, rec.reply, rec.sendImage); e != nil {
		t.Fatal(e)
	}
	yyImgWaitFor(t, "重试后出图", func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.images) == 1
	})

	replies, images, calls := rec.snapshot()
	if len(calls) != 2 {
		t.Fatalf("应重试一次共 2 次调用，实际 %d", len(calls))
	}
	if len(images) != 1 || images[0] != "img://retry-ok" {
		t.Fatalf("重试成功后应出图：%v", images)
	}
	for _, r := range replies {
		if strings.Contains(r, "没画出来") {
			t.Fatalf("重试成功不应回错误：%v", replies)
		}
	}
}

// TestYYImgQueueFailureRepliesReason：一直失败时，重试一次后把错误原因回给用户。
func TestYYImgQueueFailureRepliesReason(t *testing.T) {
	rec := &yyImgRecorder{}
	q := newYYImgQueue(1, 5*time.Second, 1, rec.gen(func(ctx context.Context, qq int64, prompt string, useAvatar bool) (string, error) {
		return "", fmt.Errorf("密钥没配")
	}))
	t.Cleanup(q.Close)

	if e := q.Submit("g1", 7, "画猫", false, rec.reply, rec.sendImage); e != nil {
		t.Fatal(e)
	}
	yyImgWaitFor(t, "收到失败回复", func() bool {
		return strings.Contains(rec.lastReply(), "图没画出来")
	})

	replies, images, calls := rec.snapshot()
	if len(calls) != 2 {
		t.Fatalf("失败应重试一次共 2 次调用，实际 %d", len(calls))
	}
	if len(images) != 0 {
		t.Fatalf("失败不应出图：%v", images)
	}
	if last := replies[len(replies)-1]; !strings.Contains(last, "密钥没配") {
		t.Fatalf("失败回复应带上错误原因：%q", last)
	}
}

// TestYYImgSubmitRejectsBadInput：空白描述、缺回调都不该入队。
func TestYYImgSubmitRejectsBadInput(t *testing.T) {
	rec := &yyImgRecorder{}
	noop := func(string) {}
	if e := yyImgSubmit("g1", 7, "   ", false, noop, rec.sendImage); e == nil {
		t.Fatal("空白 prompt 应被拒绝")
	}
	if e := yyImgSubmit("g1", 7, "画猫", false, nil, rec.sendImage); e == nil {
		t.Fatal("缺少 reply 回调应被拒绝")
	}
	if e := yyImgSubmit("g1", 7, "画猫", false, noop, nil); e == nil {
		t.Fatal("缺少 sendImage 回调应被拒绝")
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.replies) != 0 || len(rec.images) != 0 || len(rec.calls) != 0 {
		t.Fatal("被拒绝的请求不应触发任何回调")
	}
}

// TestYYImgGenerateDefaultDisabledConfig：全局生图配置零值（未启用）时，默认
// 生图实现必须直接报错，不去打任何网络请求。
func TestYYImgGenerateDefaultDisabledConfig(t *testing.T) {
	yyQQImageMu.Lock()
	old := yyQQImageCfg
	yyQQImageCfg = yyQQImageConfig{}
	yyQQImageMu.Unlock()
	t.Cleanup(func() {
		yyQQImageMu.Lock()
		yyQQImageCfg = old
		yyQQImageMu.Unlock()
	})

	_, e := yyImgGenerateDefault(context.Background(), 10001, "画猫", false)
	if e == nil || !strings.Contains(e.Error(), "未启用") {
		t.Fatalf("生图未启用时应直接报错且不请求网络，实际 %v", e)
	}
}

// TestYYImgQQOfStable：QQ 号原样转换；非数字 ID（官方 openid）取稳定哈希，
// 同一 ID 永远同键、不同 ID 不撞键。
func TestYYImgQQOfStable(t *testing.T) {
	if got := yyImgQQOf(" 10001 "); got != 10001 {
		t.Fatalf("QQ 号应原样转换，实际 %d", got)
	}
	a, b := yyImgQQOf("openid-abc"), yyImgQQOf("openid-abc")
	if a != b {
		t.Fatalf("同一 openid 应得到同一键：%d != %d", a, b)
	}
	if yyImgQQOf("openid-abc") == yyImgQQOf("openid-abd") {
		t.Fatal("不同 ID 不应撞键")
	}
}
