package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// 合并转发（OneBot 11 send_group_forward_msg）。
//
// 场景：触发词联动最终生成一份「以提问者身份」的聊天记录并转发——
// 提问者是第一条节点（用提问者自己的昵称与 QQ 号），机器人是第二条节点。
// 发送层做成函数字段，默认包一层 yyQQOneBotCall，测试里换假实现，
// 严禁为此修改 youyuan_qq.go。

// yyForwardNode 是合并转发里的一个节点。
type yyForwardNode struct {
	UIN     int64  `json:"uin"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// yyForwardSender 是可注入的发送层签名，与 yyQQOneBotCall 一致。
type yyForwardSender func(action string, params map[string]any) (any, error)

// yyForwardSend 默认发送层：走群服桥接的 OneBot WebSocket。
var yyForwardSend yyForwardSender = yyQQOneBotCall

// yyForwardBot 机器人自身在转发节点里的身份；UIN 为 0 时回退读
// cf.OneBot.BotID（与 youyuan_qq.go 读循环里的取法一致）。
var yyForwardBot = struct {
	sync.RWMutex
	UIN  int64
	Name string
}{Name: "游园"}

// yyForwardBotNode 取机器人自身节点身份。
func yyForwardBotNode() yyForwardNode {
	yyForwardBot.RLock()
	uin, name := yyForwardBot.UIN, yyForwardBot.Name
	yyForwardBot.RUnlock()
	if uin == 0 {
		if id, e := strconv.ParseInt(strings.TrimSpace(yyQQSnapshot().OneBot.BotID), 10, 64); e == nil {
			uin = id
		}
	}
	if strings.TrimSpace(name) == "" {
		name = "游园"
	}
	return yyForwardNode{UIN: uin, Name: name}
}

// yyForwardSegments 把纯文本 Content 转成 OneBot 段数组。段数据的值按
// 字面处理，不需要 CQ 转义；发送时用段数组而不是 CQ 字符串。
func yyForwardSegments(content string) []map[string]any {
	return []map[string]any{
		{"type": "text", "data": map[string]any{"text": content}},
	}
}

// yyForwardBuild 把节点列表组成 OneBot 11 合并转发并通过发送层发出。
// 每个节点形如 {"type":"node","data":{"name":昵称,"uin":QQ,"content":[段]}}。
func yyForwardBuild(group string, nodes []yyForwardNode) error {
	gid, e := strconv.ParseInt(strings.TrimSpace(group), 10, 64)
	if e != nil {
		return fmt.Errorf("群号无效")
	}
	clean := make([]yyForwardNode, 0, len(nodes))
	for _, n := range nodes {
		n.Name = strings.TrimSpace(n.Name)
		n.Content = strings.TrimSpace(n.Content)
		if n.Content == "" {
			continue
		}
		clean = append(clean, n)
	}
	if len(clean) == 0 {
		return fmt.Errorf("合并转发没有可发送的节点")
	}
	messages := make([]map[string]any, 0, len(clean))
	for _, n := range clean {
		messages = append(messages, map[string]any{
			"type": "node",
			"data": map[string]any{
				"name":    n.Name,
				"uin":     n.UIN,
				"content": yyForwardSegments(n.Content),
			},
		})
	}
	if _, e := yyForwardSend("send_group_forward_msg", map[string]any{"group_id": gid, "messages": messages}); e != nil {
		return fmt.Errorf("合并转发发送失败：%v", e)
	}
	return nil
}

// yyForwardAskAnswer 以提问者身份转发一问一答：第一条节点是提问者本人
// （Name/UIN 用提问者的），第二条是机器人自身。合并转发不可用时降级为
// 普通消息（先复述「有人问：question」再发 answer），并返回说明已降级的 error。
func yyForwardAskAnswer(group string, asker yyForwardNode, question, answer string) error {
	question = strings.TrimSpace(question)
	answer = strings.TrimSpace(answer)
	if question == "" || answer == "" {
		return fmt.Errorf("问题或回答为空，无需转发")
	}
	asker.Name = strings.TrimSpace(asker.Name)
	if asker.Name == "" {
		asker.Name = "群友"
	}
	bot := yyForwardBotNode()
	if e := yyForwardBuild(group, []yyForwardNode{
		{UIN: asker.UIN, Name: asker.Name, Content: question},
		{UIN: bot.UIN, Name: bot.Name, Content: answer},
	}); e != nil {
		if de := yyForwardDegrade(group, question, answer); de != nil {
			return fmt.Errorf("合并转发失败且降级发送也失败：%v；%v", e, de)
		}
		return fmt.Errorf("合并转发失败，已降级为普通消息：%v", e)
	}
	return nil
}

// yyForwardDegrade 降级路径：普通群消息先复述问题，再发回答。
func yyForwardDegrade(group, question, answer string) error {
	gid, e := strconv.ParseInt(strings.TrimSpace(group), 10, 64)
	if e != nil {
		return fmt.Errorf("群号无效")
	}
	if _, e := yyForwardSend("send_group_msg", map[string]any{"group_id": gid, "message": "有人问：" + question}); e != nil {
		return e
	}
	if _, e := yyForwardSend("send_group_msg", map[string]any{"group_id": gid, "message": answer}); e != nil {
		return e
	}
	return nil
}
