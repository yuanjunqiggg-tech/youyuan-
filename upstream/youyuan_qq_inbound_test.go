package main

// 入站结构化（youyuan_qq_inbound.go）测试。
//
// 全部用例只碰内存态：不落盘、不发 QQ 消息、不连网、不读真实数据文件。
// 最高优先级是「三形态解析出的 Text 等价」——Text 与 raw_message 逐字节
// 一致是行为兼容硬约束（现有群消息链路只吃 text），任何回归都必须在这里
// 第一个红灯。其余覆盖：CQ 编解码与反转义、段值归一、回复字段、@ 解析、
// 私聊/群聊分流、图片与纯文本视图、入站上下文（存储/查回/TTL/有界/私聊去重）。

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// yyQQInboundFixture 保存并重置入站相关的包级状态，t.Cleanup 逆序恢复。
// 返回一份启用了 group-a 的配置（供回复解析等需要群缓存的用例使用）。
func yyQQInboundFixture(t *testing.T) yyQQConfig {
	t.Helper()
	oldQQ, oldStore := yyQQ, aiStore
	oldMsgs, oldEpoch := yyQQMessages.Groups, yyQQMessages.Epoch
	oldCtxItems, oldCtxOrder := yyQQInboundCtx.items, yyQQInboundCtx.order
	oldSeen := yyQQInboundPrivateSeen.items
	oldTTL := yyQQInboundCtxTTL
	t.Cleanup(func() {
		yyQQ, aiStore = oldQQ, oldStore
		yyQQMessages.Groups, yyQQMessages.Epoch = oldMsgs, oldEpoch
		yyQQInboundCtx.items, yyQQInboundCtx.order = oldCtxItems, oldCtxOrder
		yyQQInboundPrivateSeen.items = oldSeen
		yyQQInboundCtxTTL = oldTTL
	})
	yyQQMessages.Groups, yyQQMessages.Epoch = map[string][]yyQQGroupMessage{}, map[string]uint64{}
	yyQQInboundCtx.items, yyQQInboundCtx.order = map[string]yyQQInboundEntry{}, nil
	yyQQInboundPrivateSeen.items = map[string]time.Time{}
	aiStore = &aiModelStore{}
	cf := yyQQConfig{Adapter: "onebot", Enabled: true, GroupID: "group-a", Groups: []yyQQGroup{{ID: "group-a", Enabled: true}, {ID: "group-b", Enabled: true}}}
	cf.OneBot.BotID = "fixture-bot"
	yyQQ = cf
	return cf
}

// yyQQInboundForm 同一逻辑消息的一种事件形态。
type yyQQInboundForm struct {
	name  string
	event map[string]any
}

// yyQQInboundBaseEvent 构造群消息事件骨架。message 为 nil 表示不带 message
// 字段（用于验证退回 raw_message 的路径）；raw 为空表示不带 raw_message。
func yyQQInboundBaseEvent(message any, raw string) map[string]any {
	event := map[string]any{
		"post_type":    "message",
		"message_type": "group",
		"group_id":     float64(12345),
		"user_id":      float64(777),
		"self_id":      float64(10086),
		"message_id":   "m-1",
		"sender":       map[string]any{"card": "卡片名", "nickname": "昵称", "role": "admin"},
	}
	if message != nil {
		event["message"] = message
	}
	if raw != "" {
		event["raw_message"] = raw
	}
	return event
}

// yyQQInboundMixedCQ 混合消息的 CQ 码原文：@ + 含特殊字符的文本 + 图片。
const yyQQInboundMixedCQ = "[CQ:at,qq=10086]价格&amp;说明&#44;见&#91;公告&#93;[CQ:image,file=abc.jpg,url=https://example.com/a.jpg]"

// yyQQInboundMixedArray 与 yyQQInboundMixedCQ 等价的 array 段数组形态。
func yyQQInboundMixedArray() []any {
	return []any{
		map[string]any{"type": "at", "data": map[string]any{"qq": "10086"}},
		map[string]any{"type": "text", "data": map[string]any{"text": "价格&说明,见[公告]"}},
		map[string]any{"type": "image", "data": map[string]any{"file": "abc.jpg", "url": "https://example.com/a.jpg"}},
	}
}

// yyQQInboundSameShape 断言两条入站在所有消费者可见字段上完全一致。
func yyQQInboundSameShape(t *testing.T, tag string, a, b yyQQInbound) {
	t.Helper()
	if a.Text != b.Text {
		t.Fatalf("%s: Text 不一致 %q vs %q", tag, a.Text, b.Text)
	}
	if yyQQInboundText(a) != yyQQInboundText(b) {
		t.Fatalf("%s: 纯文本视图不一致 %q vs %q", tag, yyQQInboundText(a), yyQQInboundText(b))
	}
	if !reflect.DeepEqual(a.Segments, b.Segments) {
		t.Fatalf("%s: 段解析不一致 %+v vs %+v", tag, a.Segments, b.Segments)
	}
	if !reflect.DeepEqual(a.Mentions, b.Mentions) {
		t.Fatalf("%s: Mentions 不一致 %v vs %v", tag, a.Mentions, b.Mentions)
	}
	if !reflect.DeepEqual(yyQQInboundImages(a), yyQQInboundImages(b)) {
		t.Fatalf("%s: 图片列表不一致 %v vs %v", tag, yyQQInboundImages(a), yyQQInboundImages(b))
	}
	if a.ReplyID != b.ReplyID || a.Mentioned != b.Mentioned {
		t.Fatalf("%s: 回复/@ 字段不一致 %+v vs %+v", tag, a, b)
	}
	if a.Group != b.Group || a.User != b.User || a.Name != b.Name || a.Role != b.Role || a.ID != b.ID || a.IsPrivate != b.IsPrivate || a.PostType != b.PostType {
		t.Fatalf("%s: 基础字段不一致 %+v vs %+v", tag, a, b)
	}
}

// TestYYQQParseInboundThreeFormsTextEquivalence 最高优先级用例：同一消息的
// 三种形态（array 段数组 / string CQ 码 / 仅 raw_message 回退）解析出的 Text
// 必须等价；带 raw_message 时还必须逐字节等于原文（行为兼容硬约束）。
func TestYYQQParseInboundThreeFormsTextEquivalence(t *testing.T) {
	yyQQInboundFixture(t)
	messages := []struct {
		name  string
		cq    string
		array []any
		plain string
	}{
		{"混合消息", yyQQInboundMixedCQ, yyQQInboundMixedArray(), ""},
		{"纯文本", "大家好呀", []any{map[string]any{"type": "text", "data": map[string]any{"text": "大家好呀"}}}, "大家好呀"},
	}
	for _, msg := range messages {
		for _, withRaw := range []bool{false, true} {
			raw := ""
			if withRaw {
				raw = msg.cq
			}
			forms := []yyQQInboundForm{
				{"array", yyQQInboundBaseEvent(msg.array, raw)},
				{"cq-string", yyQQInboundBaseEvent(msg.cq, raw)},
				{"raw-only", yyQQInboundBaseEvent(nil, msg.cq)},
			}
			if msg.plain != "" {
				forms = append(forms, yyQQInboundForm{"plain-string", yyQQInboundBaseEvent(msg.plain, raw)})
			}
			var first yyQQInbound
			for i, form := range forms {
				in, ok := yyQQParseInbound(form.event)
				if !ok {
					t.Fatalf("%s/%s: 应解析成功", msg.name, form.name)
				}
				if in.Text != msg.cq {
					t.Fatalf("%s/%s: Text 应为 %q，实际 %q", msg.name, form.name, msg.cq, in.Text)
				}
				if i == 0 {
					first = in
					continue
				}
				yyQQInboundSameShape(t, msg.name+"/"+form.name, first, in)
			}
		}
	}
	// 混合消息的段级派生字段也要符合预期（@ 名单、Mentioned、图片、纯文本）。
	in, _ := yyQQParseInbound(yyQQInboundBaseEvent(yyQQInboundMixedArray(), ""))
	if !reflect.DeepEqual(in.Mentions, []string{"10086"}) || !in.Mentioned {
		t.Fatal("@ 解析不符", in.Mentions, in.Mentioned)
	}
	if !reflect.DeepEqual(yyQQInboundImages(in), []string{"https://example.com/a.jpg"}) {
		t.Fatal("图片解析不符", yyQQInboundImages(in))
	}
	if got := yyQQInboundText(in); got != "@10086价格&说明,见[公告][图片]" {
		t.Fatalf("纯文本视图不符: %q", got)
	}
}

// TestYYQQParseInboundRejectsNonMessageEvents 非 message 事件一律拒绝：
// post_type 不是 message（含 message_sent 回声）、缺 post_type、类型不对。
func TestYYQQParseInboundRejectsNonMessageEvents(t *testing.T) {
	yyQQInboundFixture(t)
	if _, ok := yyQQParseInbound(nil); ok {
		t.Fatal("nil 事件应拒绝")
	}
	for _, event := range []map[string]any{
		{},
		{"post_type": "message_sent", "message": "回声"},
		{"post_type": "notice", "notice_type": "group_increase"},
		{"post_type": "meta_event", "meta_event_type": "heartbeat"},
		{"post_type": "request", "request_type": "friend"},
		{"post_type": 123, "message": "类型不对"},
	} {
		if in, ok := yyQQParseInbound(event); ok {
			t.Fatalf("非 message 事件应拒绝: %+v", in)
		}
	}
}

// TestYYQQParseInboundTextMatchesRawMessageVerbatim Text 硬约束：raw_message
// 存在时原样采用（与改造前逐字节一致）；段级信息仍按 message array 解析；
// message 既非 array 也非 string 时退回 raw_message；全缺时 Text 为空。
func TestYYQQParseInboundTextMatchesRawMessageVerbatim(t *testing.T) {
	yyQQInboundFixture(t)
	raw := "[CQ:at,qq=1]价格&amp;说明"
	in, ok := yyQQParseInbound(map[string]any{
		"post_type": "message", "message_type": "group", "group_id": "g1",
		"raw_message": raw,
		"message":     []any{map[string]any{"type": "text", "data": map[string]any{"text": "完全不同的纯文本"}}},
	})
	if !ok || in.Text != raw {
		t.Fatal("Text 必须与 raw_message 逐字节一致", in.Text)
	}
	if len(in.Segments) != 1 || in.Segments[0].Data["text"] != "完全不同的纯文本" {
		t.Fatal("段应按 message array 解析", in.Segments)
	}
	in, ok = yyQQParseInbound(map[string]any{
		"post_type": "message", "message_type": "group", "group_id": "g1",
		"message":     123,
		"raw_message": "兜底原文",
	})
	if !ok || in.Text != "兜底原文" || len(in.Segments) != 1 || in.Segments[0].Data["text"] != "兜底原文" {
		t.Fatal("raw_message 兜底失败", in)
	}
	in, ok = yyQQParseInbound(map[string]any{
		"post_type": "message", "message_type": "group", "group_id": "g1",
	})
	if !ok || in.Text != "" || len(in.Segments) != 0 {
		t.Fatal("message 与 raw_message 全缺时应为空", in)
	}
}

// TestYYQQParseInboundReplyFields 被引用消息 id：message_reference（官方
// 事件形状）优先，其次 array/CQ 两种来源的 reply 段；都没有则留空。
func TestYYQQParseInboundReplyFields(t *testing.T) {
	yyQQInboundFixture(t)
	in, ok := yyQQParseInbound(map[string]any{
		"post_type": "message", "message_type": "group", "group_id": "g1",
		"message_reference": map[string]any{"message_id": "ref-1"},
		"message":           []any{map[string]any{"type": "reply", "data": map[string]any{"id": "seg-1"}}},
	})
	if !ok || in.ReplyID != "ref-1" {
		t.Fatal("message_reference 应优先", in.ReplyID)
	}
	in, _ = yyQQParseInbound(map[string]any{
		"post_type": "message", "message_type": "group", "group_id": "g1",
		"message": []any{map[string]any{"type": "reply", "data": map[string]any{"id": float64(9876543210)}}},
	})
	if in.ReplyID != "9876543210" {
		t.Fatal("array reply 段解析失败", in.ReplyID)
	}
	in, _ = yyQQParseInbound(map[string]any{
		"post_type": "message", "message_type": "group", "group_id": "g1",
		"message": "[CQ:reply,id=555]文本",
	})
	if in.ReplyID != "555" {
		t.Fatal("CQ reply 段解析失败", in.ReplyID)
	}
	in, _ = yyQQParseInbound(map[string]any{
		"post_type": "message", "message_type": "group", "group_id": "g1",
		"message": "普通消息",
	})
	if in.ReplyID != "" {
		t.Fatal("无回复应留空", in.ReplyID)
	}
}

// TestYYQQInboundResolveReplyFromGroupCache 本地群消息缓存补被引用消息的
// 文本与发送者：命中即填、不覆盖已有、私聊（无群）不查、未命中留空。
func TestYYQQInboundResolveReplyFromGroupCache(t *testing.T) {
	cf := yyQQInboundFixture(t)
	yyQQRememberGroupMessageMeta(cf, "member-a", "甲", "被引用的原文", "msg-a", "", false)
	in := yyQQInbound{Group: "group-a", ReplyID: "msg-a"}
	yyQQInboundResolveReply(cf, &in)
	if in.ReplyText != "被引用的原文" || in.ReplyUser != "member-a" {
		t.Fatal("缓存命中应补齐引用文本与发送者", in)
	}
	in2 := yyQQInbound{Group: "group-a", ReplyID: "msg-a", ReplyText: "已有"}
	yyQQInboundResolveReply(cf, &in2)
	if in2.ReplyText != "已有" {
		t.Fatal("不应覆盖已有 ReplyText", in2)
	}
	in3 := yyQQInbound{ReplyID: "msg-a"}
	yyQQInboundResolveReply(cf, &in3)
	if in3.ReplyText != "" {
		t.Fatal("私聊（无群）不应查群缓存", in3)
	}
	yyQQInboundResolveReply(cf, &yyQQInbound{Group: "group-a"})
	in4 := yyQQInbound{Group: "group-a", ReplyID: "不存在"}
	yyQQInboundResolveReply(cf, &in4)
	if in4.ReplyText != "" || in4.ReplyUser != "" {
		t.Fatal("缓存未命中应留空", in4)
	}
}

// TestYYQQParseInboundMentionsAndMentioned @ 解析：Mentions 收集所有 at
// 目标（含 all、跳过空值），Mentioned 判断是否 at 了机器人自己或全体；
// array 与 CQ 串两种形态结果一致。
func TestYYQQParseInboundMentionsAndMentioned(t *testing.T) {
	yyQQInboundFixture(t)
	array := []any{
		map[string]any{"type": "at", "data": map[string]any{"qq": "10086"}},
		map[string]any{"type": "at", "data": map[string]any{"qq": "all"}},
		map[string]any{"type": "at", "data": map[string]any{"qq": ""}},
		map[string]any{"type": "at", "data": map[string]any{}},
		map[string]any{"type": "text", "data": map[string]any{"text": "在吗"}},
	}
	in, ok := yyQQParseInbound(yyQQInboundBaseEvent(array, ""))
	if !ok || !reflect.DeepEqual(in.Mentions, []string{"10086", "all"}) || !in.Mentioned {
		t.Fatal("array 形态 @ 解析失败", in.Mentions, in.Mentioned)
	}
	cq := "[CQ:at,qq=10086][CQ:at,qq=all][CQ:at,qq=][CQ:at]在吗"
	inCQ, ok := yyQQParseInbound(yyQQInboundBaseEvent(cq, ""))
	if !ok {
		t.Fatal("CQ 形态应解析成功")
	}
	yyQQInboundSameShape(t, "at-mentions", in, inCQ)
	ev := yyQQInboundBaseEvent([]any{map[string]any{"type": "at", "data": map[string]any{"qq": "999"}}}, "")
	ev["self_id"] = "10086"
	other, _ := yyQQParseInbound(ev)
	if other.Mentioned || !reflect.DeepEqual(other.Mentions, []string{"999"}) {
		t.Fatal("@ 别人不应判定 at 了机器人", other.Mentions, other.Mentioned)
	}
	ev = yyQQInboundBaseEvent([]any{map[string]any{"type": "at", "data": map[string]any{"qq": "all"}}}, "")
	ev["self_id"] = "777"
	all, _ := yyQQParseInbound(ev)
	if !all.Mentioned {
		t.Fatal("@全体成员应判定 at 了机器人")
	}
	ev = yyQQInboundBaseEvent([]any{map[string]any{"type": "at", "data": map[string]any{"qq": "10086"}}}, "")
	delete(ev, "self_id")
	noSelf, _ := yyQQParseInbound(ev)
	if noSelf.Mentioned {
		t.Fatal("缺 self_id 时不应误判 at 了机器人")
	}
	// 纯空白 qq 不计入 @ 名单（TrimSpace 后为空）
	ev = yyQQInboundBaseEvent([]any{map[string]any{"type": "at", "data": map[string]any{"qq": "  "}}}, "")
	blank, _ := yyQQParseInbound(ev)
	if len(blank.Mentions) != 0 || blank.Mentioned {
		t.Fatal("空白 qq 不应计入 @ 名单", blank.Mentions)
	}
}

// TestYYQQParseInboundPrivateGroupRouting 私聊/群聊分流：message_type 显式
// 声明优先；缺失时有 group_id 按群消息，否则按私聊；私聊声明清空 Group。
func TestYYQQParseInboundPrivateGroupRouting(t *testing.T) {
	yyQQInboundFixture(t)
	cases := []struct {
		name    string
		event   map[string]any
		private bool
		group   string
	}{
		{"显式群聊", map[string]any{"post_type": "message", "message_type": "group", "group_id": float64(12345), "user_id": "u"}, false, "12345"},
		{"显式私聊", map[string]any{"post_type": "message", "message_type": "private", "user_id": "u"}, true, ""},
		{"缺类型有群号按群聊", map[string]any{"post_type": "message", "group_id": float64(12345), "user_id": "u"}, false, "12345"},
		{"缺类型无群号按私聊", map[string]any{"post_type": "message", "user_id": "u"}, true, ""},
		{"私聊声明带群号也清空", map[string]any{"post_type": "message", "message_type": "private", "group_id": float64(12345), "user_id": "u"}, true, ""},
		{"群聊声明无群号", map[string]any{"post_type": "message", "message_type": "group", "user_id": "u"}, false, ""},
	}
	for _, c := range cases {
		in, ok := yyQQParseInbound(c.event)
		if !ok {
			t.Fatalf("%s: 应解析成功", c.name)
		}
		if in.IsPrivate != c.private || in.Group != c.group {
			t.Fatalf("%s: 分流错误 private=%v group=%q", c.name, in.IsPrivate, in.Group)
		}
	}
}

// TestYYQQInboundImagesAndTextViews 图片列表（url 优先、http(s) file 兜底、
// 本地 file 忽略）与人类可读纯文本（image/at/reply/face/未知段占位）。
func TestYYQQInboundImagesAndTextViews(t *testing.T) {
	yyQQInboundFixture(t)
	in := yyQQInbound{Segments: []yyQQInboundSegment{
		{Type: "text", Data: map[string]string{"text": "看这两张"}},
		{Type: "image", Data: map[string]string{"url": "https://example.com/1.png", "file": "1.png"}},
		{Type: "image", Data: map[string]string{"file": "https://example.com/2.png"}},
		{Type: "image", Data: map[string]string{"file": "local-only.jpg"}},
		{Type: "image", Data: map[string]string{"url": "   "}},
		{Type: "at", Data: map[string]string{"qq": "all"}},
		{Type: "at", Data: map[string]string{"qq": "42"}},
		{Type: "at", Data: map[string]string{"qq": "  "}},
		{Type: "reply", Data: map[string]string{"id": "r1"}},
		{Type: "face", Data: map[string]string{"id": "1"}},
		{Type: "json", Data: map[string]string{}},
	}}
	images := yyQQInboundImages(in)
	if !reflect.DeepEqual(images, []string{"https://example.com/1.png", "https://example.com/2.png"}) {
		t.Fatal("图片 URL 列表不符", images)
	}
	if got := yyQQInboundText(in); got != "看这两张[图片][图片][图片][图片]@全体成员@42[引用][表情][json]" {
		t.Fatalf("纯文本视图不符: %q", got)
	}
	empty := yyQQInbound{}
	if len(yyQQInboundImages(empty)) != 0 || yyQQInboundText(empty) != "" {
		t.Fatal("空入站应无图且文本为空")
	}
}

// TestYYQQParseCQSegmentsEscapeRoundTrip CQ 编解码：空串、纯文本、反转义、
// 双反转义保护、畸形码按文本保留、未转义逗号拼回、空 key/type 拒绝、
// escape/unescape 互逆、段序保持。
func TestYYQQParseCQSegmentsEscapeRoundTrip(t *testing.T) {
	yyQQInboundFixture(t)
	if segs := yyQQParseCQSegments(""); segs != nil {
		t.Fatal("空串应无段", segs)
	}
	segs := yyQQParseCQSegments("价格&amp;说明")
	if len(segs) != 1 || segs[0].Type != "text" || segs[0].Data["text"] != "价格&说明" {
		t.Fatal("纯文本段解析失败", segs)
	}
	if got := yyQQCQUnescape("&amp;#91;"); got != "&#91;" {
		t.Fatal("二次反转义保护失效", got)
	}
	segs = yyQQParseCQSegments("前[CQ:]后")
	if len(segs) != 1 || segs[0].Data["text"] != "前[CQ:]后" {
		t.Fatal("畸形 CQ 未按文本保留", segs)
	}
	segs = yyQQParseCQSegments("[CQ:image,file=http://example.com/a,b.jpg]")
	if len(segs) != 1 || segs[0].Data["file"] != "http://example.com/a,b.jpg" {
		t.Fatal("未转义逗号应拼回上一参数", segs)
	}
	segs = yyQQParseCQSegments("[CQ:at,=x,qq=]")
	if len(segs) != 1 || segs[0].Type != "at" || segs[0].Data["qq"] != "" || len(segs[0].Data) != 1 {
		t.Fatal("空 key 应拒绝、空参数值应保留", segs)
	}
	segs = yyQQParseCQSegments("[CQ:at,qq=1,qq=2]")
	if len(segs) != 1 || segs[0].Data["qq"] != "2" {
		t.Fatal("同名参数应后者覆盖", segs)
	}
	raw := "a&b[],c"
	escaped := yyQQCQEscape(raw)
	if escaped != "a&amp;b&#91;&#93;&#44;c" {
		t.Fatal("转义结果不符", escaped)
	}
	if yyQQCQUnescape(escaped) != raw {
		t.Fatal("escape/unescape 不互逆", escaped)
	}
	segs = yyQQParseCQSegments("甲[CQ:face,id=1]乙[CQ:face,id=2]丙")
	if len(segs) != 5 || segs[0].Data["text"] != "甲" || segs[1].Type != "face" || segs[2].Data["text"] != "乙" || segs[3].Type != "face" || segs[4].Data["text"] != "丙" {
		t.Fatal("交错段序错误", segs)
	}
	for _, s := range []string{"价格&说明", "a,b[c]d", "&amp;#91;"} {
		back := yyQQParseCQSegments(yyQQCQEscape(s))
		if len(back) != 1 || back[0].Data["text"] != s {
			t.Fatalf("往返失真: %q -> %q", s, back)
		}
	}
}

// TestYYQQParseSegmentsValueNormalization array 段 data 值归一成字符串
// （int/float64/bool/json.Number），无 type 或空 type 的段跳过，data 非
// map 时保留段且 data 为空 map。
func TestYYQQParseSegmentsValueNormalization(t *testing.T) {
	yyQQInboundFixture(t)
	list := []any{
		map[string]any{"type": "text", "data": map[string]any{"text": "hi"}},
		map[string]any{"type": "image", "data": map[string]any{"file": 123, "sub_type": float64(1), "cache": true, "big": json.Number("9007199254740993")}},
		map[string]any{"type": "face", "data": map[string]any{"id": "4"}},
		map[string]any{"data": map[string]any{"text": "没有 type"}},
		map[string]any{"type": "   ", "data": map[string]any{"text": "空 type"}},
		"不是对象",
		map[string]any{"type": "at", "data": "不是 map"},
	}
	segs := yyQQParseSegments(list)
	if len(segs) != 4 {
		t.Fatal("应保留 4 个有效段", segs)
	}
	img := segs[1]
	if img.Type != "image" || img.Data["file"] != "123" || img.Data["sub_type"] != "1" || img.Data["cache"] != "true" || img.Data["big"] != "9007199254740993" {
		t.Fatal("段值归一失败", img)
	}
	at := segs[3]
	if at.Type != "at" || at.Data == nil || len(at.Data) != 0 {
		t.Fatal("data 非 map 应为空 map 而非 nil", at)
	}
}

// TestYYQQInboundContextStoreLookupCurrentAndBounds 入站上下文：空 ID 不记、
// Raw 剥离、按 ID 查回、current 取最近、有界 64 条淘汰最旧。
func TestYYQQInboundContextStoreLookupCurrentAndBounds(t *testing.T) {
	yyQQInboundFixture(t)
	yyQQInboundStore(yyQQInbound{ID: "", Text: "无 ID"})
	if _, ok := yyQQCurrentInbound(); ok {
		t.Fatal("空 ID 不应入上下文")
	}
	yyQQInboundStore(yyQQInbound{ID: "m-1", Text: "你好", Raw: map[string]any{"post_type": "message"}})
	got, ok := yyQQInboundLookup("m-1")
	if !ok || got.Text != "你好" || got.Raw != nil {
		t.Fatal("查回异常或 Raw 未剥离", got, ok)
	}
	if cur, ok := yyQQCurrentInbound(); !ok || cur.ID != "m-1" {
		t.Fatal("current 应取最近一条", cur, ok)
	}
	if _, ok := yyQQInboundLookup(""); ok {
		t.Fatal("空 id 应查不到")
	}
	if _, ok := yyQQInboundLookup("不存在"); ok {
		t.Fatal("未知 id 应查不到")
	}
	time.Sleep(5 * time.Millisecond)
	yyQQInboundStore(yyQQInbound{ID: "m-2", Text: "第二条"})
	if cur, ok := yyQQCurrentInbound(); !ok || cur.ID != "m-2" {
		t.Fatal("current 应更新为最新", cur, ok)
	}
	for i := 0; i < 70; i++ {
		yyQQInboundStore(yyQQInbound{ID: fmt.Sprintf("b-%02d", i)})
	}
	yyQQInboundCtx.Lock()
	bound := len(yyQQInboundCtx.order)
	yyQQInboundCtx.Unlock()
	if bound != 64 {
		t.Fatal("上下文中应有界 64", bound)
	}
	if _, ok := yyQQInboundLookup("b-00"); ok {
		t.Fatal("最旧应被淘汰")
	}
	if _, ok := yyQQInboundLookup("b-69"); !ok {
		t.Fatal("最新应保留")
	}
}

// TestYYQQInboundContextTTLExpiry 超过 TTL（默认 10 分钟，测试临时调小）
// 后按 ID 查回与 current 都不可见。
func TestYYQQInboundContextTTLExpiry(t *testing.T) {
	yyQQInboundFixture(t)
	yyQQInboundCtxTTL = 20 * time.Millisecond
	yyQQInboundStore(yyQQInbound{ID: "ttl-1", Text: "会过期"})
	if _, ok := yyQQInboundLookup("ttl-1"); !ok {
		t.Fatal("刚存入应可查回")
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := yyQQInboundLookup("ttl-1"); ok {
		t.Fatal("超过 TTL 应查不到")
	}
	if _, ok := yyQQCurrentInbound(); ok {
		t.Fatal("超过 TTL 后 current 应为空")
	}
}

// TestYYQQInboundPrivateClaimAndDispatch 私聊去重（同 message_id 只处理
// 一次、表满淘汰最旧但新消息不丢）与分发边界（空 user/机器人回声不入上下文，
// 正常私聊入上下文并占用 claim）。
func TestYYQQInboundPrivateClaimAndDispatch(t *testing.T) {
	cf := yyQQInboundFixture(t)
	// 这里的断言依赖「从空表开始数到 256」，但去重表是包级变量，
	// 别的用例留下的记录会让计数偏移（全量跑偶发失败、单跑必过）。
	// 测试自己负责把这块全局状态清干净。
	yyQQInboundPrivateSeen.Lock()
	yyQQInboundPrivateSeen.items = map[string]time.Time{}
	yyQQInboundPrivateSeen.Unlock()
	if !yyQQInboundPrivateClaim("p-1") {
		t.Fatal("首次应认领")
	}
	if yyQQInboundPrivateClaim("p-1") {
		t.Fatal("重复 message_id 不应二次处理")
	}
	for i := 0; i < 256; i++ {
		yyQQInboundPrivateClaim(fmt.Sprintf("k-%d", i))
	}
	// 紧凑循环 Claim 的时间戳可能相同（时钟粒度），直接把 k-0 的记账时间
	// 调到明确最旧（仍在 TTL 内，避免走到 TTL 清扫分支），保证淘汰可预期。
	yyQQInboundPrivateSeen.Lock()
	yyQQInboundPrivateSeen.items["k-0"] = time.Now().Add(-5 * time.Minute)
	yyQQInboundPrivateSeen.Unlock()
	if !yyQQInboundPrivateClaim("fresh") {
		t.Fatal("表满后新消息不应被丢弃")
	}
	yyQQInboundPrivateSeen.Lock()
	kept := len(yyQQInboundPrivateSeen.items)
	_, k0gone := yyQQInboundPrivateSeen.items["k-0"]
	yyQQInboundPrivateSeen.Unlock()
	if kept != 256 {
		t.Fatal("淘汰后应维持 256", kept)
	}
	if k0gone {
		t.Fatal("最旧条目应被淘汰")
	}
	yyQQInboundPrivate(cf, yyQQInbound{ID: "x-1", User: "", Text: "空用户"})
	if _, ok := yyQQInboundLookup("x-1"); ok {
		t.Fatal("空 user 不应入上下文")
	}
	yyQQInboundPrivate(cf, yyQQInbound{ID: "x-2", User: cf.OneBot.BotID, Text: "机器人自己"})
	if _, ok := yyQQInboundLookup("x-2"); ok {
		t.Fatal("机器人回声不应入上下文")
	}
	cf.Assistant.Enabled = false
	yyQQInboundPrivate(cf, yyQQInbound{ID: "x-3", User: "u-9", Name: "小红", Text: "在吗"})
	if _, ok := yyQQInboundLookup("x-3"); !ok {
		t.Fatal("正常私聊应入上下文")
	}
	if yyQQInboundPrivateClaim("x-3") {
		t.Fatal("私聊 claim 应已占用")
	}
	if yyQQInboundRun(cf, yyQQInbound{ID: "r-1", User: ""}) {
		t.Fatal("空 user 应返回 false")
	}
	if yyQQInboundRun(cf, yyQQInbound{ID: "r-2", User: "u-1", Name: "小红", Text: "普通消息"}) {
		t.Fatal("无触发词应返回 false")
	}
}
