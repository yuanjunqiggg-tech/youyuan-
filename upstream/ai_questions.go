package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// 用户询问：AI 通过 user_question 工具向前端发一个带选项的问题，
// 前端弹窗让用户选择，答案回传给模型继续推理。

// aiQuestionTimeout 等待用户回答的最长时间。
const aiQuestionTimeout = 5 * time.Minute

// aiQuestionRegistry 待回答的问题注册表。
type aiQuestionRegistry struct {
	mu      sync.Mutex
	seq     int
	pending map[string]chan string // req_id → 答案通道
}

var aiQuestions = &aiQuestionRegistry{pending: map[string]chan string{}}

// register 登记一个问题，返回 req_id 与答案通道。
func (q *aiQuestionRegistry) register() (string, chan string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.seq++
	id := fmt.Sprintf("q%d", q.seq)
	ch := make(chan string, 1)
	q.pending[id] = ch
	return id, ch
}

// pop 取回并删除一个问题的答案通道。
func (q *aiQuestionRegistry) pop(id string) (chan string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	ch, ok := q.pending[id]
	if ok {
		delete(q.pending, id)
	}
	return ch, ok
}

// askUser 向前端发问题并等待答案。input 为工具参数（question/options）。
func askUser(ctx context.Context, sse *aiSSE, input map[string]any) (string, bool) {
	question := aiArgStr(input, "question")
	if question == "" {
		question = "请选择："
	}
	var options []string
	if opts, ok := input["options"].([]any); ok {
		for _, o := range opts {
			if s, ok := o.(string); ok && s != "" {
				options = append(options, s)
			}
		}
	}
	if len(options) == 0 {
		options = []string{"继续", "换一种做法", "取消"}
	}
	reqID, ch := aiQuestions.register()
	sse.event("ai_question", map[string]any{
		"req_id":   reqID,
		"question": question,
		"options":  options,
	})
	select {
	case ans := <-ch:
		return "用户回答: " + ans, false
	case <-time.After(aiQuestionTimeout):
		aiQuestions.pop(reqID)
		return "用户超时未回答，请根据已掌握的信息继续，不要再问", false
	case <-ctx.Done():
		return "用户已中断", true
	}
}
