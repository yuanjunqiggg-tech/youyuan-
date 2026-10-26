package main

// 引用原文解析的测试（2026-10-26 第 29 条）
//
// 用户反馈：机器人在群里说「看不到引用的原文」。
// 根因是 ReplyText 写了但没人读。这里的测试守三件事：
//   1. CQ 码串能被转成人能读的正文（不然模型会看到一堆 [CQ:image,file=...]）
//   2. 本地窗口里能按 message_id 找回被引用消息
//   3. 拿不到时明确标 reply_to_text_available=false，而不是编一段

import (
	"strings"
	"testing"
)

func TestQQQuoteStripCQ(t *testing.T) {
	cases := []struct{ in, want string }{
		{"你好", "你好"},
		{"[CQ:image,file=abc.jpg,url=http://x/y.jpg]看这个", "[图片]看这个"},
		{"[CQ:face,id=14]", "[表情]"},
		{"[CQ:at,qq=12345] 你来", "@12345 你来"},
		{"[CQ:at,qq=all]", "@全体成员"},
		{"[CQ:reply,id=99]收到", "收到"},
		{"[CQ:record,file=a.amr]", "[语音]"},
		{"[CQ:video,file=a.mp4]", "[视频]"},
		{"[CQ:file,name=a.zip]", "[文件]"},
	}
	for _, c := range cases {
		if got := yyQQQuoteStripCQ(c.in); got != c.want {
			t.Errorf("strip(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
	// 超长引用要截断，不能把整条 2000 字灌进上下文
	long := strings.Repeat("字", 900)
	got := yyQQQuoteStripCQ(long)
	if len([]rune(got)) > yyQQQuoteTextMax+1 {
		t.Fatalf("超长引用没截断：%d 字", len([]rune(got)))
	}
}

func TestQQQuoteFromRows(t *testing.T) {
	rows := []yyQQGroupMessage{
		{ID: "100", User: "111", Text: "第一条"},
		{ID: "101", User: "222", Text: "第二条"},
	}
	if row, ok := yyQQQuoteFromRows(rows, "101"); !ok || row.Text != "第二条" {
		t.Fatalf("应按 id 找回第二条，实际 %v %v", row, ok)
	}
	if _, ok := yyQQQuoteFromRows(rows, "999"); ok {
		t.Fatal("不存在的 id 不该命中")
	}
	if _, ok := yyQQQuoteFromRows(rows, ""); ok {
		t.Fatal("空 id 不该命中")
	}
	// 重复 id 时取最新的一条（从后往前找）
	dup := []yyQQGroupMessage{{ID: "5", Text: "旧"}, {ID: "5", Text: "新"}}
	if row, _ := yyQQQuoteFromRows(dup, "5"); row.Text != "新" {
		t.Fatalf("重复 id 应取最新，实际 %q", row.Text)
	}
}

func TestQQQuoteMessageIDArg(t *testing.T) {
	// 纯数字要转 int（多数 OneBot 实现只吃整数，穿字符串会静默失败）
	if v, ok := yyQQMessageIDArg("123456789").(int64); !ok || v != 123456789 {
		t.Fatalf("纯数字应转 int64，实际 %#v", yyQQMessageIDArg("123456789"))
	}
	// 非数字原样传（有些实现用字符串 id）
	if v, ok := yyQQMessageIDArg("abc-123").(string); !ok || v != "abc-123" {
		t.Fatalf("非数字应原样传，实际 %#v", yyQQMessageIDArg("abc-123"))
	}
	// 超过 18 位不转，避免溢出
	if _, ok := yyQQMessageIDArg("1234567890123456789012").(string); !ok {
		t.Fatal("超长 id 应保持字符串")
	}
}

func TestQQQuoteFromGetMsg(t *testing.T) {
	// 形态 A：message 是 CQ 字符串
	data := map[string]any{
		"message": "[CQ:image,file=a.jpg]这个种子好不好",
		"sender":  map[string]any{"user_id": float64(222), "card": "小明", "nickname": "xm"},
	}
	text, user, name := yyQQQuoteFromGetMsg(data)
	if text != "[图片]这个种子好不好" {
		t.Fatalf("CQ 串解析错：%q", text)
	}
	if user != "222" || name != "小明" {
		t.Fatalf("发送者解析错：%q %q", user, name)
	}

	// 形态 B：message 是 segment 数组
	data2 := map[string]any{
		"message": []any{
			map[string]any{"type": "text", "data": map[string]any{"text": "看这个"}},
			map[string]any{"type": "image", "data": map[string]any{"url": "http://x"}},
			map[string]any{"type": "reply", "data": map[string]any{"id": "1"}},
		},
		"user_id": float64(333),
	}
	text2, user2, _ := yyQQQuoteFromGetMsg(data2)
	if text2 != "看这个[图片]" {
		t.Fatalf("段数组解析错：%q", text2)
	}
	if user2 != "333" {
		t.Fatalf("user_id 兜底失败：%q", user2)
	}

	// 形态 C：垃圾输入不 panic
	if text3, _, _ := yyQQQuoteFromGetMsg(nil); text3 != "" {
		t.Fatal("nil 输入应返回空")
	}
	if text4, _, _ := yyQQQuoteFromGetMsg("not a map"); text4 != "" {
		t.Fatal("非对象输入应返回空")
	}
}

// TestQQQuoteFieldsUnavailable 拿不到引用时必须留痕（reply_to_text_available=false），
// 这样模型才能诚实地说「看不到」，而不是被提示词逼着编一段。
func TestQQQuoteFieldsUnavailable(t *testing.T) {
	cf := yyQQConfig{}
	payload := map[string]any{"message_id": "1"}
	yyQQQuoteFields(cf, payload, "", true)
	if _, has := payload["reply_to_text"]; has {
		t.Fatal("没有 replyID 时不该塞 reply_to_text")
	}

	// 空 id 之外的、取不到的 id：应该标 false，不 panic
	payload2 := map[string]any{"message_id": "2"}
	yyQQQuoteFields(cf, payload2, "999999999", true)
	if payload2["reply_to_text_available"] != false {
		t.Fatalf("取不到时应标 false，实际 %v", payload2["reply_to_text_available"])
	}

	// nil map 不许 panic
	yyQQQuoteFields(cf, nil, "123", true)

	// 缓存要能记住「取不到」，避免同一个 id 反复打网络
	if entry, ok := yyQQQuoteCached("999999999"); !ok || entry.ok {
		t.Fatalf("负缓存没生效：%v %v", entry, ok)
	}
}

func TestQQQuoteCacheBounded(t *testing.T) {
	// 缓存必须有界，否则长连接下按 message_id 无限增长
	for i := 0; i < yyQQQuoteCacheMax+40; i++ {
		yyQQQuoteStore("k"+itoaSimple(i), yyQQQuoteEntry{text: "t", ok: true})
	}
	yyQQQuoteCache.Lock()
	n := len(yyQQQuoteCache.items)
	yyQQQuoteCache.Unlock()
	if n > yyQQQuoteCacheMax {
		t.Fatalf("缓存超过上限：%d > %d", n, yyQQQuoteCacheMax)
	}
}

func itoaSimple(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
