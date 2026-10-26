package main

// 游园生图异步队列。
//
// 触发词 `image` 动作以前是每条群消息起一个 goroutine 直接画：一个人连发
// 十条就十条一起打接口，慢请求还会把后续动作全拖住。这个队列把生图收拢成
// 固定小并发的 FIFO 工人池，做到四件事：
//  1. 并发可配（1-2），超出排队，不无限起 goroutine；
//  2. 单任务超时：context 到期即放弃这一次生成，不等慢接口把队列焊死；
//  3. 失败自动重试一次，仍失败才把原因回给用户；
//  4. 同群同 QQ 未完成的前任务合并（coalesce）：以新 prompt 替换旧的，
//     只画最新一条，避免刷屏式重复出图。
//
// 生图函数是队列上的可注入字段，默认实现复用「生图」面板已配置的
// provider（yyQQImageGenerate），不另起一套密钥配置；测试里整体换掉，
// 不碰线上链路，也不需要连网。

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"
)

// yyImgGenFunc 生图函数。返回图片引用（默认是 base64:// 数据，由调用方的
// sendImage 负责发送）；error 即失败原因。ctx 到期时必须尽快返回，队列
// 按它判断单任务超时。
type yyImgGenFunc func(ctx context.Context, qq int64, prompt string, useAvatar bool) (string, error)

// yyImgTask 一条生图任务。prompt 会被合并逻辑原地替换，所以 worker 读
// prompt、判断「是否已被更新的请求顶掉」都必须在锁内做。
type yyImgTask struct {
	group     string
	qq        int64
	prompt    string
	useAvatar bool
	reply     func(string)
	sendImage func(string) error
}

// yyImgQueue 生图任务队列：并发 1-2 可配、FIFO、同人合并、超时加重试一次。
type yyImgQueue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	queue   []*yyImgTask          // 等待中的任务，FIFO
	active  map[string]*yyImgTask // "群\x00QQ" -> 排队中或执行中的任务
	closed  bool
	workers int
	timeout time.Duration
	retry   int
	gen     yyImgGenFunc
	wg      sync.WaitGroup
}

// newYYImgQueue 建队列并拉起 worker。workers 夹到 1-2；timeout 非正回退
// 150 秒（与生图 HTTP 客户端一致）；retry 是失败重试次数；gen 为空时用
// 线上默认实现。
func newYYImgQueue(workers int, timeout time.Duration, retry int, gen yyImgGenFunc) *yyImgQueue {
	if workers < 1 {
		workers = 1
	}
	if workers > 2 {
		workers = 2
	}
	if timeout <= 0 {
		timeout = 150 * time.Second
	}
	if retry < 0 {
		retry = 0
	}
	if gen == nil {
		gen = yyImgGenerateDefault
	}
	q := &yyImgQueue{
		workers: workers,
		timeout: timeout,
		retry:   retry,
		gen:     gen,
		active:  map[string]*yyImgTask{},
	}
	q.cond = sync.NewCond(&q.mu)
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.loop()
	}
	return q
}

// Submit 入队一条生图任务。
//   - 同群同 QQ 有未完成的前任务时合并：原地替换成新 prompt，只回一句
//     「已更新为最新请求」，不重复出图；回调也切到最新这条消息上，图和
//     回复都挂在用户最后发的那条；
//   - 否则排队，并立即回复「🎨 开始画（队列 N）…」，N 是排队数（含自己）；
//   - reply / sendImage 在 worker goroutine 上回调，调用方需自行保证其并发安全。
func (q *yyImgQueue) Submit(group string, qq int64, prompt string, useAvatar bool, reply func(string), sendImage func(string) error) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("缺少画面描述")
	}
	if reply == nil || sendImage == nil {
		return fmt.Errorf("缺少回复或发送回调")
	}
	key := yyImgKey(group, qq)
	q.mu.Lock()
	if old, ok := q.active[key]; ok {
		old.prompt = prompt
		old.useAvatar = useAvatar
		old.reply = reply
		old.sendImage = sendImage
		q.mu.Unlock()
		reply("已更新为最新请求")
		return nil
	}
	task := &yyImgTask{
		group:     group,
		qq:        qq,
		prompt:    prompt,
		useAvatar: useAvatar,
		reply:     reply,
		sendImage: sendImage,
	}
	q.queue = append(q.queue, task)
	q.active[key] = task
	pos := len(q.queue)
	q.cond.Signal()
	q.mu.Unlock()
	reply(fmt.Sprintf("🎨 开始画（队列 %d）…", pos))
	return nil
}

// loop worker 主循环：取队首任务执行，直到 Close。
func (q *yyImgQueue) loop() {
	defer q.wg.Done()
	for {
		q.mu.Lock()
		for len(q.queue) == 0 && !q.closed {
			q.cond.Wait()
		}
		if q.closed {
			q.mu.Unlock()
			return
		}
		task := q.queue[0]
		q.queue = q.queue[1:]
		q.mu.Unlock()
		q.runTask(task)
	}
}

// runTask 跑一个任务：单任务超时（整个任务一个 context）、失败重试一次、
// 执行期间被合并时丢弃旧结果改画最新 prompt。
func (q *yyImgQueue) runTask(task *yyImgTask) {
	defer q.finish(task)
	ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
	defer cancel()
	fails := 0
	for {
		q.mu.Lock()
		prompt, useAvatar := task.prompt, task.useAvatar
		q.mu.Unlock()
		ref, err := q.gen(ctx, task.qq, prompt, useAvatar)
		if err == nil {
			q.mu.Lock()
			stale := task.prompt != prompt || task.useAvatar != useAvatar
			q.mu.Unlock()
			if !stale {
				if e := task.sendImage(ref); e != nil {
					task.reply("图画好了但发送失败：" + e.Error())
				}
				return
			}
			// 画图期间用户又发了新描述：这张作废，改画最新的，重试次数重新计。
			fails = 0
			continue
		}
		if fails < q.retry {
			fails++
			continue
		}
		task.reply("图没画出来：" + err.Error())
		return
	}
}

// finish 任务收尾：从在途表摘掉，同一用户的下一条请求才能重新排队。
func (q *yyImgQueue) finish(task *yyImgTask) {
	key := yyImgKey(task.group, task.qq)
	q.mu.Lock()
	if cur, ok := q.active[key]; ok && cur == task {
		delete(q.active, key)
	}
	q.mu.Unlock()
}

// Close 停止接活并等 worker 退出。排队中但还没开始的任务会被丢弃（其回调
// 不再触发），正在画的任务会画完当前这一轮。
func (q *yyImgQueue) Close() {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		q.cond.Broadcast()
	}
	q.mu.Unlock()
	q.wg.Wait()
}

func yyImgKey(group string, qq int64) string {
	return group + "\x00" + strconv.FormatInt(qq, 10)
}

var (
	yyImgQueueOnce    sync.Once
	yyImgQueueDefault *yyImgQueue
)

// yyImgQueueGet 取全局默认队列（首次用时按默认参数建：2 并发、150 秒超时、重试 1 次）。
func yyImgQueueGet() *yyImgQueue {
	yyImgQueueOnce.Do(func() {
		yyImgQueueDefault = newYYImgQueue(2, 150*time.Second, 1, yyImgGenerateDefault)
	})
	return yyImgQueueDefault
}

// yyImgSubmit 触发词 `image` 动作的入队口：异步出图，不阻塞消息循环。
// group 是互通群 ID；qq 是提问者（同一人群内合并请求用）；useAvatar 是否
// 拿提问者头像当参考图；reply 回文字，sendImage 发图（参数是图片引用）。
func yyImgSubmit(group string, qq int64, prompt string, useAvatar bool, reply func(string), sendImage func(string) error) error {
	return yyImgQueueGet().Submit(group, qq, prompt, useAvatar, reply, sendImage)
}

// yyImgQQOf 把触发词里的 userID 归一成 int64。OneBot 侧是 QQ 号，直接转；
// 官方机器人的 openid 不是数字，取 FNV-1a 稳定哈希兜底，保证同一用户在
// 不同消息里键相同，合并逻辑才对得上。
func yyImgQQOf(userID string) int64 {
	s := strings.TrimSpace(userID)
	if n, e := strconv.ParseInt(s, 10, 64); e == nil {
		return n
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}

// yyImgGenerateDefault 线上生图实现：复用「生图」面板已配置的 provider /
// model / key（用户在面板里换模型不用等发版），prompt 前后缀也按配置拼；
// useAvatar 时取提问者 QQ 头像做参考图。返回 base64:// 图片引用。
func yyImgGenerateDefault(ctx context.Context, qq int64, prompt string, useAvatar bool) (string, error) {
	c := yyQQImageSnapshot()
	if !c.Enabled {
		return "", fmt.Errorf("生图功能未启用，先去面板「生图」里打开")
	}
	full := strings.TrimSpace(c.PromptPrefix + " " + strings.TrimSpace(prompt) + " " + c.PromptSuffix)
	var avatar []byte
	if useAvatar {
		if url := yyQQAvatarURL(strconv.FormatInt(qq, 10)); url != "" {
			if raw, _, e := yyQQImageFetch(url); e == nil && len(raw) > 0 {
				avatar = raw
			}
		}
	}
	img, _, e := yyImgGenerateCtx(ctx, c, full, avatar)
	if e != nil {
		return "", e
	}
	return "base64://" + base64.StdEncoding.EncodeToString(img), nil
}

// yyImgGenerateCtx 给不接 context 的 yyQQImageGenerate 套一层超时看门：
// 超时就放弃等待（那一路的结果丢弃），不让慢接口把队列焊死。
func yyImgGenerateCtx(ctx context.Context, c yyQQImageConfig, prompt string, avatar []byte) ([]byte, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	type outcome struct {
		img  []byte
		mime string
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		img, mime, err := yyQQImageGenerate(c, prompt, avatar)
		done <- outcome{img: img, mime: mime, err: err}
	}()
	select {
	case got := <-done:
		return got.img, got.mime, got.err
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
}
