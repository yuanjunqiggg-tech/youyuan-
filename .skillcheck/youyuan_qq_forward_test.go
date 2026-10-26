package main

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

// yyForwardFakeSender 假发送层：记录每次调用，可按需让合并转发失败。
type yyForwardFakeSender struct {
	mu          sync.Mutex
	calls       []yyForwardCall
	failForward bool
}

type yyForwardCall struct {
	Action string
	Params map[string]any
}

func (f *yyForwardFakeSender) send(action string, params map[string]any) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, yyForwardCall{Action: action, Params: params})
	if f.failForward && action == "send_group_forward_msg" {
		return nil, errors.New("假发送层：合并转发不被支持")
	}
	return map[string]any{"message_id": 1}, nil
}

func (f *yyForwardFakeSender) snapshot() []yyForwardCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]yyForwardCall(nil), f.calls...)
}

// yyForwardTestInstall 把假发送层装上去，返回还原后的假实现。
func yyForwardTestInstall(t *testing.T, failForward bool) *yyForwardFakeSender {
	t.Helper()
	fake := &yyForwardFakeSender{failForward: failForward}
	old := yyForwardSend
	yyForwardSend = fake.send
	t.Cleanup(func() { yyForwardSend = old })
	return fake
}

// yyForwardTestInstallBot 固定机器人节点身份，避免依赖全局配置。
func yyForwardTestInstallBot(t *testing.T, uin int64, name string) {
	t.Helper()
	yyForwardBot.Lock()
	oldUIN, oldName := yyForwardBot.UIN, yyForwardBot.Name
	yyForwardBot.UIN, yyForwardBot.Name = uin, name
	yyForwardBot.Unlock()
	t.Cleanup(func() {
		yyForwardBot.Lock()
		yyForwardBot.UIN, yyForwardBot.Name = oldUIN, oldName
		yyForwardBot.Unlock()
	})
}

// 节点结构：action、group_id、messages 里每个节点的 name/uin/content 三字段。
func TestForwardBuildNodeStructure(t *testing.T) {
	fake := yyForwardTestInstall(t, false)
	if e := yyForwardBuild("123456", []yyForwardNode{
		{UIN: 111, Name: "小明", Content: "服主在吗"},
		{UIN: 222, Name: "游园", Content: "在，说"},
	}); e != nil {
		t.Fatalf("合并转发应成功：%v", e)
	}
	calls := fake.snapshot()
	if len(calls) != 1 || calls[0].Action != "send_group_forward_msg" {
		t.Fatalf("应调用一次 send_group_forward_msg，实际 %+v", calls)
	}
	raw, e := json.Marshal(calls[0].Params)
	if e != nil {
		t.Fatalf("参数序列化失败：%v", e)
	}
	var wire struct {
		GroupID  int64 `json:"group_id"`
		Messages []struct {
			Type string `json:"type"`
			Data struct {
				Name    string `json:"name"`
				UIN     int64  `json:"uin"`
				Content []struct {
					Type string `json:"type"`
					Data struct {
						Text string `json:"text"`
					} `json:"data"`
				} `json:"content"`
			} `json:"data"`
		} `json:"messages"`
	}
	if e := json.Unmarshal(raw, &wire); e != nil {
		t.Fatalf("参数反序列化失败：%v", e)
	}
	if wire.GroupID != 123456 {
		t.Fatalf("group_id 应为 123456，实际 %d", wire.GroupID)
	}
	if len(wire.Messages) != 2 {
		t.Fatalf("应有两个节点，实际 %d", len(wire.Messages))
	}
	want := []struct {
		name    string
		uin     int64
		content string
	}{
		{"小明", 111, "服主在吗"},
		{"游园", 222, "在，说"},
	}
	for i, w := range want {
		node := wire.Messages[i]
		if node.Type != "node" {
			t.Fatalf("节点 %d 类型应为 node，实际 %s", i, node.Type)
		}
		if node.Data.Name != w.name || node.Data.UIN != w.uin {
			t.Fatalf("节点 %d name/uin 应为 %s/%d，实际 %s/%d", i, w.name, w.uin, node.Data.Name, node.Data.UIN)
		}
		if len(node.Data.Content) != 1 || node.Data.Content[0].Type != "text" || node.Data.Content[0].Data.Text != w.content {
			t.Fatalf("节点 %d content 应为单文本段 %q，实际 %+v", i, w.content, node.Data.Content)
		}
	}
	var generic map[string]any
	_ = json.Unmarshal(raw, &generic)
	msgs, _ := generic["messages"].([]any)
	for i, m := range msgs {
		node, _ := m.(map[string]any)
		data, _ := node["data"].(map[string]any)
		if len(data) != 3 {
			t.Fatalf("节点 %d data 应恰有 name/uin/content 三个字段，实际 %v", i, data)
		}
	}
}

// 非法输入直接报错，不触碰发送层。
func TestForwardBuildRejectsEmptyInput(t *testing.T) {
	fake := yyForwardTestInstall(t, false)
	if e := yyForwardBuild("123456", nil); e == nil {
		t.Fatal("空节点列表应报错")
	}
	if e := yyForwardBuild("123456", []yyForwardNode{{UIN: 1, Name: "小明", Content: "   "}}); e == nil {
		t.Fatal("内容全为空的节点应报错")
	}
	if e := yyForwardBuild("abc", []yyForwardNode{{UIN: 1, Name: "小明", Content: "在吗"}}); e == nil {
		t.Fatal("群号非法应报错")
	}
	if len(fake.snapshot()) != 0 {
		t.Fatal("报错时不应调用发送层")
	}
}

// 成功路径：提问者节点用提问者身份，机器人节点身份回退自 BotID。
func TestForwardAskAnswerUsesAskerAndBotIdentity(t *testing.T) {
	fake := yyForwardTestInstall(t, false)
	yyForwardTestInstallBot(t, 0, "") // 清空显式身份，强制走 BotID 回退
	yyQQMu.Lock()
	oldBot := yyQQ.OneBot.BotID
	yyQQ.OneBot.BotID = "2854196316"
	yyQQMu.Unlock()
	defer func() {
		yyQQMu.Lock()
		yyQQ.OneBot.BotID = oldBot
		yyQQMu.Unlock()
	}()
	if e := yyForwardAskAnswer("777", yyForwardNode{UIN: 111, Name: "小明"}, "服主在吗", "在，说"); e != nil {
		t.Fatalf("转发应成功：%v", e)
	}
	calls := fake.snapshot()
	if len(calls) != 1 || calls[0].Action != "send_group_forward_msg" {
		t.Fatalf("应调用一次合并转发，实际 %+v", calls)
	}
	raw, _ := json.Marshal(calls[0].Params)
	var wire struct {
		Messages []struct {
			Data struct {
				Name    string `json:"name"`
				UIN     int64  `json:"uin"`
				Content []struct {
					Data struct {
						Text string `json:"text"`
					} `json:"data"`
				} `json:"content"`
			} `json:"data"`
		} `json:"messages"`
	}
	if e := json.Unmarshal(raw, &wire); e != nil {
		t.Fatalf("参数解析失败：%v", e)
	}
	if len(wire.Messages) != 2 {
		t.Fatalf("应有提问者与机器人两个节点，实际 %d", len(wire.Messages))
	}
	first, second := wire.Messages[0].Data, wire.Messages[1].Data
	if first.Name != "小明" || first.UIN != 111 || first.Content[0].Data.Text != "服主在吗" {
		t.Fatalf("第一条应是提问者节点，实际 %+v", first)
	}
	if second.Name != "游园" || second.UIN != 2854196316 || second.Content[0].Data.Text != "在，说" {
		t.Fatalf("第二条应是机器人节点（身份回退自 BotID），实际 %+v", second)
	}
}

// 降级路径：合并转发失败时先复述「有人问：question」再发 answer。
func TestForwardAskAnswerDegradesWhenForwardFails(t *testing.T) {
	fake := yyForwardTestInstall(t, true)
	yyForwardTestInstallBot(t, 222, "游园")
	err := yyForwardAskAnswer("777", yyForwardNode{UIN: 111, Name: "小明"}, "服主在吗", "在，说")
	if err == nil || !strings.Contains(err.Error(), "降级") {
		t.Fatalf("应返回说明已降级的 error，实际 %v", err)
	}
	calls := fake.snapshot()
	if len(calls) != 3 {
		t.Fatalf("应有一次转发失败加两次降级消息，实际 %d 次：%+v", len(calls), calls)
	}
	if calls[0].Action != "send_group_forward_msg" {
		t.Fatalf("第一次应尝试合并转发，实际 %s", calls[0].Action)
	}
	for i, want := range []string{"有人问：服主在吗", "在，说"} {
		call := calls[i+1]
		if call.Action != "send_group_msg" {
			t.Fatalf("降级第 %d 条应是普通群消息，实际 %s", i+1, call.Action)
		}
		if msg, _ := call.Params["message"].(string); msg != want {
			t.Fatalf("降级第 %d 条内容应为 %q，实际 %q", i+1, want, msg)
		}
	}
}

// 降级也失败时如实汇报，不假装成功。
func TestForwardAskAnswerReportsWhenDegradeAlsoFails(t *testing.T) {
	yyForwardTestInstall(t, true)
	yyForwardSend = func(action string, params map[string]any) (any, error) {
		return nil, errors.New("假发送层：全线失败")
	}
	err := yyForwardAskAnswer("777", yyForwardNode{UIN: 111, Name: "小明"}, "服主在吗", "在，说")
	if err == nil || !strings.Contains(err.Error(), "降级发送也失败") {
		t.Fatalf("转发与降级都失败时应如实汇报，实际 %v", err)
	}
}

// 空问题或空回答直接拒绝，不触碰发送层。
func TestForwardAskAnswerRejectsEmptyInput(t *testing.T) {
	fake := yyForwardTestInstall(t, false)
	if e := yyForwardAskAnswer("777", yyForwardNode{UIN: 111, Name: "小明"}, "  ", "在，说"); e == nil {
		t.Fatal("空问题应报错")
	}
	if e := yyForwardAskAnswer("777", yyForwardNode{UIN: 111, Name: "小明"}, "服主在吗", ""); e == nil {
		t.Fatal("空回答应报错")
	}
	if len(fake.snapshot()) != 0 {
		t.Fatal("报错时不应调用发送层")
	}
}
