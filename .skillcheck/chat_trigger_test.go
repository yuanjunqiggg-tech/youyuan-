package main

import (
	"strconv"
	"testing"
	"time"
)

// 验证 hasWaitMsg：有等待者返回 true，无等待者返回 false。
func TestHasWaitMsg(t *testing.T) {
	if hasWaitMsg("ghost") {
		t.Fatal("期望无等待者时返回 false")
	}
	ch := make(chan string, 1)
	playerWaitMsgMu.Lock()
	playerWaitMsgCbs["carol"] = ch
	playerWaitMsgMu.Unlock()
	defer func() {
		playerWaitMsgMu.Lock()
		delete(playerWaitMsgCbs, "carol")
		playerWaitMsgMu.Unlock()
	}()
	if !hasWaitMsg("carol") {
		t.Fatal("期望有等待者时返回 true")
	}
}

// 验证参数按类型解析。
func TestParseArgValue(t *testing.T) {
	if v, err := parseArgValue("42", ArgTypeInt); err != nil || v != 42 {
		t.Errorf("int: got %v/%v, want 42", v, err)
	}
	if v, err := parseArgValue("3.5", ArgTypeFloat); err != nil || v != 3.5 {
		t.Errorf("float: got %v/%v, want 3.5", v, err)
	}
	if v, err := parseArgValue("true", ArgTypeBool); err != nil || v != true {
		t.Errorf("bool: got %v/%v, want true", v, err)
	}
	if v, err := parseArgValue("hello", ArgTypeStr); err != nil || v != "hello" {
		t.Errorf("str: got %v/%v, want hello", v, err)
	}
	if _, err := parseArgValue("abc", ArgTypeInt); err == nil {
		t.Error("int 解析非数字应报错")
	}
}

// 验证触发词匹配：msg 等于触发词，或 msg 以"触发词 空格"开头。
func TestMatchTriggerWord(t *testing.T) {
	words := []string{"积分", "查询"}
	word, rest, ok := matchTriggerWord("积分", words)
	if !ok || word != "积分" || rest != "" {
		t.Errorf("精确匹配失败: %q %q %v", word, rest, ok)
	}
	word, rest, ok = matchTriggerWord("查询 张三", words)
	if !ok || word != "查询" || rest != "张三" {
		t.Errorf("带参数匹配失败: %q %q %v", word, rest, ok)
	}
	if _, _, ok := matchTriggerWord("随便说", words); ok {
		t.Error("不匹配的词不应命中")
	}
}

// 验证 handleChat 命中触发词并调用处理函数（含内联参数解析）。
func TestHandleChatTrigger(t *testing.T) {
	c := &captureHook{}
	bm := &BotManager{}
	bm.AddChatTrigger(&ChatTrigger{
		TriggerWords: []string{"测试"},
		Args:         []ArgHint{{Name: "x", Type: ArgTypeInt}},
		Usage:        "测试用",
		Func: func(player string, args []any) {
			c.add(player + ":" + strconv.Itoa(args[0].(int)))
		},
	})
	bm.handleChat("alice", "测试 1")
	// handleChat 异步派发 Func，轮询等待处理函数完成
	deadline := time.Now().Add(2 * time.Second)
	for c.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.all(); len(got) != 1 || got[0] != "alice:1" {
		t.Fatalf("期望处理函数被调用并得到 alice:1，got %q", got)
	}
}

// 验证 parseArgsFromText 按提示解析内联参数，缺失时标出下标。
func TestParseArgsFromText(t *testing.T) {
	hints := []ArgHint{{Name: "a", Type: ArgTypeInt}, {Name: "b", Type: ArgTypeStr}}
	vals, missing := parseArgsFromText("42", hints)
	if len(vals) != 2 || vals[0] != 42 {
		t.Fatalf("解析错误: %v", vals)
	}
	if len(missing) != 1 || missing[0] != 1 {
		t.Fatalf("缺失标记错误: %v", missing)
	}
}

// 验证参数类型中文标签。
func TestArgTypeLabel(t *testing.T) {
	if argTypeLabel(ArgTypeInt) != "整数" {
		t.Fatal("int 标签错误")
	}
}

// 验证按插件移除触发词，返回移除数量。
func TestRemovePluginTriggers(t *testing.T) {
	chatTriggers.Lock()
	chatTriggers.list = nil
	chatTriggers.Unlock()

	bm := &BotManager{}
	bm.AddChatTrigger(&ChatTrigger{Plugin: "p1", TriggerWords: []string{"a"}})
	bm.AddChatTrigger(&ChatTrigger{Plugin: "p1", TriggerWords: []string{"b"}})
	bm.AddChatTrigger(&ChatTrigger{Plugin: "p2", TriggerWords: []string{"c"}})

	if n := bm.RemovePluginTriggers("p1"); n != 2 {
		t.Fatalf("期望移除 2 个，got %d", n)
	}
	chatTriggers.Lock()
	remaining := len(chatTriggers.list)
	chatTriggers.Unlock()
	if remaining != 1 {
		t.Fatalf("期望剩 1 个，got %d", remaining)
	}
}

// 验证列出当前活跃插件 ID。
func TestActivePluginIDs(t *testing.T) {
	chatTriggers.Lock()
	chatTriggers.list = nil
	chatTriggers.Unlock()

	bm := &BotManager{}
	bm.AddChatTrigger(&ChatTrigger{Plugin: "p1", TriggerWords: []string{"a"}})
	bm.AddChatTrigger(&ChatTrigger{Plugin: "p2", TriggerWords: []string{"b"}})

	ids := bm.ActivePluginIDs()
	if len(ids) != 2 {
		t.Fatalf("期望 2 个插件，got %v", ids)
	}
}
