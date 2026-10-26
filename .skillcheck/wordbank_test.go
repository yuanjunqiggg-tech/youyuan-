package main

import (
	"testing"
	"time"
)

// 验证词库 JSON 解析为触发器，触发词、参数、指令、仅OP 均正确。
func TestLoadWordBank(t *testing.T) {
	data := []byte(`{
	  "菜单项": [
	    {
	      "说明": "返回重生点",
	      "触发词": ["kill", "自尽"],
	      "需要的参数(没有则填[])": [],
	      "功能简介": "返回重生点",
	      "触发后执行的指令": ["/kill [玩家名]"],
	      "仅OP可用": false
	    }
	  ]
	}`)
	triggers, err := loadWordBank(&BotManager{}, data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("期望 1 个触发器，got %d", len(triggers))
	}
	tg := triggers[0]
	if len(tg.TriggerWords) != 2 || tg.TriggerWords[0] != "kill" {
		t.Fatalf("触发词错误: %v", tg.TriggerWords)
	}
	if tg.OpOnly {
		t.Fatal("期望非仅OP")
	}
	if tg.Usage != "返回重生点" {
		t.Fatalf("用途错误: %q", tg.Usage)
	}
}

// 验证指令中的 [玩家名] 占位符被替换为实际玩家。
func TestExecCommandReplacesPlayer(t *testing.T) {
	got := execCommand("/kill [玩家名]", "张三", nil)
	if got != "/kill 张三" {
		t.Fatalf("玩家占位符替换失败: %q", got)
	}
}

// 验证指令中的 [参数:N] 占位符被替换为第 N 个参数。
func TestExecCommandReplacesArgs(t *testing.T) {
	args := []any{10, "钻石"}
	got := execCommand("/give [玩家名] [参数:1] [参数:2]", "张三", args)
	if got != "/give 张三 10 钻石" {
		t.Fatalf("参数占位符替换失败: %q", got)
	}
}

// 验证完整链路：聊天输入触发词 → 词库触发 → 发出指令（[玩家名] 被替换）。
func TestWordBankChatFlow(t *testing.T) {
	c := &captureHook{}
	old := sendCommandHook
	setSendCommandHook(func(cmd string) error { c.add(cmd); return nil })
	defer func() { setSendCommandHook(old) }()

	bm := &BotManager{}
	data := []byte(`{"菜单项":[{"说明":"t","触发词":["积分"],"需要的参数(没有则填[])":[],"功能简介":"查询","触发后执行的指令":["/scoreboard players list [玩家名]"],"仅OP可用":false}]}`)
	triggers, err := loadWordBank(bm, data)
	if err != nil {
		t.Fatalf("词库解析失败: %v", err)
	}
	for _, tg := range triggers {
		bm.AddChatTrigger(tg)
	}
	bm.handleChat("alice", "积分")
	// handleChat 异步派发 Func，轮询等待指令发出
	deadline := time.Now().Add(2 * time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.all(); len(got) != 1 || got[0] != "/scoreboard players list alice" {
		t.Fatalf("指令捕获错误: %v", got)
	}
}
