package main

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// 入站消息结构化（P0 基础能力）。
//
// 设计依据 docs/design/bot-capability-arch.md §1.6：WS 读循环原来只取 raw_message、
// 只接受 group 消息，导致引用图片（reference:"quoted"）、私聊触发、@ 解析全部做不了。
// 这里把 OneBot v11 消息事件解析成结构化的 yyQQInbound：
//   - 喂现有群消息链路用的 Text 与 raw_message 等价（行为兼容硬约束）；
//   - 段级信息（image/reply/at）供触发词引擎、生图参考图（§2.3）与私聊分发取用。
//
// 触发词引擎的扩展参数（ReplyID / 图片 / at 名单）后续由引擎侧接入，
// 本期先落地结构化与包级「当前入站上下文」。

// yyQQInboundSegment 是 OneBot 消息段。至少支持 text/image/reply/at 四种 type：
// text 段 data.text；image 段 data.url 与 data.file；reply 段 data.id；at 段 data.qq。
// face 等其余段原样保留 type 与 data，解析不丢信息。
type yyQQInboundSegment struct {
	Type string            `json:"type"`
	Data map[string]string `json:"data,omitempty"`
}

type yyQQInbound struct {
	Group     string               `json:"group"`
	User      string               `json:"user"`
	Name      string               `json:"name"`
	Role      string               `json:"role"`
	Text      string               `json:"text"` // 与 raw_message 等价（CQ 原文），喂现有只吃 text 的链路
	ID        string               `json:"id"`
	ReplyID   string               `json:"reply_id,omitempty"`
	ReplyText string               `json:"reply_text,omitempty"`
	ReplyUser string               `json:"reply_user,omitempty"`
	Mentioned bool                 `json:"mentioned"`
	Mentions  []string             `json:"mentions,omitempty"`
	Segments  []yyQQInboundSegment `json:"segments,omitempty"`
	IsPrivate bool                 `json:"is_private"`
	PostType  string               `json:"post_type"`
	Raw       map[string]any       `json:"-"`
}

// yyQQParseInbound 把 OneBot v11 消息事件解析成结构化入站消息。
// message 可能是 array（段数组）或 string（CQ 码或纯文本），三种都处理；
// 私聊（message_type=private，或 message_type 缺失且没有 group_id）解析为
// IsPrivate=true、Group 为空。返回 false 表示不是可处理的消息事件
// （post_type 不是 message、message_sent 回声等）。
func yyQQParseInbound(event map[string]any) (yyQQInbound, bool) {
	if event == nil {
		return yyQQInbound{}, false
	}
	postType, _ := event["post_type"].(string)
	if postType != "message" {
		return yyQQInbound{}, false
	}
	group := yyQQGroupString(event["group_id"])
	messageType, _ := event["message_type"].(string)
	switch messageType {
	case "group":
	case "private":
		group = ""
	default:
		// 少数实现不发 message_type：有 group_id 按群消息，否则按私聊。
		if group == "" {
			messageType = "private"
		} else {
			messageType = "group"
		}
	}
	sender, _ := event["sender"].(map[string]any)
	in := yyQQInbound{
		Group:     group,
		User:      yyQQStableID(event["user_id"]),
		Name:      yyQQSenderName(sender),
		ID:        yyQQStableID(event["message_id"]),
		IsPrivate: messageType == "private",
		PostType:  postType,
		Raw:       event,
	}
	if role, ok := sender["role"].(string); ok {
		in.Role = role
	}
	// 段解析：优先 message（array 或 string），缺失时退回 raw_message。
	switch message := event["message"].(type) {
	case []any:
		in.Segments = yyQQParseSegments(message)
	case string:
		in.Segments = yyQQParseCQSegments(message)
	default:
		if raw, ok := event["raw_message"].(string); ok {
			in.Segments = yyQQParseCQSegments(raw)
		}
	}
	// Text 与 raw_message 等价：raw_message 存在时原样采用（行为兼容硬约束，
	// 群消息链路拿到的 text 与改造前逐字节一致）；缺失时才按段重建成 CQ 形式，
	// 保证 Text 始终维持 raw_message 的形态。
	if raw, ok := event["raw_message"].(string); ok {
		in.Text = raw
	} else {
		in.Text = yyQQInboundCQ(in.Segments)
	}
	in.Mentions = yyQQInboundMentions(in.Segments)
	if self := yyQQStableID(event["self_id"]); self != "" {
		in.Mentioned = yyQQInboundMentionedBy(in.Segments, self)
	}
	in.ReplyID = yyQQInboundReplyID(event, in.Segments)
	return in, true
}

// yyQQParseSegments 解析 array 形态的 message 段。data 里的值统一归一成字符串
// （json.Number / float64 / bool 都转），段内不丢字段。
func yyQQParseSegments(list []any) []yyQQInboundSegment {
	out := make([]yyQQInboundSegment, 0, len(list))
	for _, item := range list {
		segment, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := segment["type"].(string)
		kind = strings.TrimSpace(kind)
		if kind == "" {
			continue
		}
		data := map[string]string{}
		if values, ok := segment["data"].(map[string]any); ok {
			for key, value := range values {
				data[key] = yyQQSegmentValue(value)
			}
		}
		out = append(out, yyQQInboundSegment{Type: kind, Data: data})
	}
	return out
}

// yyQQParseCQSegments 解析 CQ 码字符串（如 "[CQ:image,file=xxx]你好"）为段：
// 非 CQ 部分整体当 text 段（做反转义），CQ 码按 type 建段。
func yyQQParseCQSegments(raw string) []yyQQInboundSegment {
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "[CQ:") {
		return []yyQQInboundSegment{{Type: "text", Data: map[string]string{"text": yyQQCQUnescape(raw)}}}
	}
	var out []yyQQInboundSegment
	var text strings.Builder
	flush := func() {
		if text.Len() == 0 {
			return
		}
		out = append(out, yyQQInboundSegment{Type: "text", Data: map[string]string{"text": yyQQCQUnescape(text.String())}})
		text.Reset()
	}
	for i := 0; i < len(raw); {
		if raw[i] == '[' && i+4 <= len(raw) && raw[i:i+4] == "[CQ:" {
			end := strings.IndexByte(raw[i:], ']')
			if end > 4 {
				flush()
				if segment, ok := yyQQParseCQBody(raw[i+4 : i+end]); ok {
					out = append(out, segment)
				}
				i += end + 1
				continue
			}
		}
		text.WriteByte(raw[i])
		i++
	}
	flush()
	return out
}

// yyQQParseCQBody 解析 CQ 码体（"at,qq=123"）为段。参数值里未转义的逗号拼回
// 上一个参数，容忍部分实现不转义 URL 查询串的情况。
func yyQQParseCQBody(body string) (yyQQInboundSegment, bool) {
	kind, params := body, ""
	if comma := strings.IndexByte(body, ','); comma >= 0 {
		kind, params = body[:comma], body[comma+1:]
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return yyQQInboundSegment{}, false
	}
	data := map[string]string{}
	last := ""
	if params != "" {
		for _, piece := range strings.Split(params, ",") {
			if eq := strings.IndexByte(piece, '='); eq > 0 {
				key := yyQQCQUnescape(piece[:eq])
				if key == "" {
					last = ""
					continue
				}
				data[key] = yyQQCQUnescape(piece[eq+1:])
				last = key
				continue
			}
			if last != "" {
				data[last] += "," + yyQQCQUnescape(piece)
			}
		}
	}
	return yyQQInboundSegment{Type: kind, Data: data}, true
}

// CQ 码转义四个字符：& [ ] ,。反转义用 strings.Replacer 单趟扫描，
// 保证 "&amp;#91;" 只会还原成 "&#91;" 而不会被二次反转义成 "["。
var yyQQCQUnescaper = strings.NewReplacer("&#44;", ",", "&#93;", "]", "&#91;", "[", "&amp;", "&")
var yyQQCQEscaper = strings.NewReplacer("&", "&amp;", "[", "&#91;", "]", "&#93;", ",", "&#44;")

func yyQQCQUnescape(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	return yyQQCQUnescaper.Replace(s)
}
func yyQQCQEscape(s string) string {
	if !strings.ContainsAny(s, "&[],") {
		return s
	}
	return yyQQCQEscaper.Replace(s)
}

// yyQQSegmentValue 把段数据里的值归一成字符串（json.Number/float64/int/bool）。
func yyQQSegmentValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	}
	return yyQQStableID(value)
}

// yyQQInboundMentions 收集段内所有 at 目标（含 "all"）。
func yyQQInboundMentions(segments []yyQQInboundSegment) []string {
	var out []string
	for _, segment := range segments {
		if segment.Type != "at" {
			continue
		}
		if qq := strings.TrimSpace(segment.Data["qq"]); qq != "" {
			out = append(out, qq)
		}
	}
	return out
}

// yyQQInboundMentionedBy 判断段内是否 at 了指定账号（或 @全体）。
// 语义与官方适配器的 GROUP_AT_MESSAGE_CREATE 对齐：这条消息 @ 了机器人。
func yyQQInboundMentionedBy(segments []yyQQInboundSegment, botID string) bool {
	for _, segment := range segments {
		if segment.Type != "at" {
			continue
		}
		qq := strings.TrimSpace(segment.Data["qq"])
		if qq == "all" || (botID != "" && qq == botID) {
			return true
		}
	}
	return false
}

// yyQQInboundReplyID 取被引用消息 id：优先 message_reference（官方事件形状），
// 再看段内 reply 段（array 与 CQ 串两种来源都已解析成段）。
func yyQQInboundReplyID(event map[string]any, segments []yyQQInboundSegment) string {
	if ref, ok := event["message_reference"].(map[string]any); ok {
		return yyQQStableID(ref["message_id"])
	}
	for _, segment := range segments {
		if segment.Type == "reply" {
			if id := strings.TrimSpace(segment.Data["id"]); id != "" {
				return id
			}
		}
	}
	return ""
}

// yyQQInboundCQ 把段重建成 CQ 码字符串（raw_message 的形态），仅在事件缺
// raw_message 时兜底；参数键排序保证输出确定。
func yyQQInboundCQ(segments []yyQQInboundSegment) string {
	var b strings.Builder
	for _, segment := range segments {
		if segment.Type == "text" {
			b.WriteString(yyQQCQEscape(segment.Data["text"]))
			continue
		}
		b.WriteString("[CQ:")
		b.WriteString(segment.Type)
		keys := make([]string, 0, len(segment.Data))
		for key := range segment.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString(",")
			b.WriteString(key)
			b.WriteString("=")
			b.WriteString(yyQQCQEscape(segment.Data[key]))
		}
		b.WriteString("]")
	}
	return b.String()
}

// yyQQInboundImages 返回段内所有图片 URL 列表（供生图参考图用，见设计 §2.3）。
// 优先 data.url；拿不到 url 时退回 http(s) 形态的 data.file（部分实现只给 file）。
func yyQQInboundImages(in yyQQInbound) []string {
	var out []string
	for _, segment := range in.Segments {
		if segment.Type != "image" {
			continue
		}
		url := strings.TrimSpace(segment.Data["url"])
		if url == "" {
			if file := strings.TrimSpace(segment.Data["file"]); strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
				url = file
			}
		}
		if url != "" {
			out = append(out, url)
		}
	}
	return out
}

// yyQQInboundText 把段拼成纯文本：image 段占位 [图片]、at 段占位 @qq（@全体成员），
// 供只吃 text 的链路（AI 上下文、触发词干跑等）使用。注意与 Text 的区别：
// Text 与 raw_message 等价（CQ 原文，喂现有群消息链路），本函数是人类可读形态。
func yyQQInboundText(in yyQQInbound) string {
	var b strings.Builder
	for _, segment := range in.Segments {
		switch segment.Type {
		case "text":
			b.WriteString(segment.Data["text"])
		case "image":
			b.WriteString("[图片]")
		case "at":
			if qq := strings.TrimSpace(segment.Data["qq"]); qq == "all" {
				b.WriteString("@全体成员")
			} else if qq != "" {
				b.WriteString("@" + qq)
			}
		case "reply":
			b.WriteString("[引用]")
		case "face":
			b.WriteString("[表情]")
		default:
			b.WriteString("[" + segment.Type + "]")
		}
	}
	return b.String()
}

// yyQQInboundResolveReply 用本地群消息缓存补被引用消息的文本与发送者
// （机器人"自己去看聊天记录"的本地部分，不发起网络调用；取不到就留空，
// 后续可退回 get_msg，见设计 §2.3）。
func yyQQInboundResolveReply(cf yyQQConfig, in *yyQQInbound) {
	if in == nil || in.ReplyID == "" || in.Group == "" || in.ReplyText != "" {
		return
	}
	for _, row := range yyQQRecentMessages(cf, 80) {
		if row.ID == in.ReplyID {
			in.ReplyText = row.Text
			in.ReplyUser = row.User
			return
		}
	}
}

// 包级「当前入站上下文」：按 message_id 记住最近的结构化入站消息，
// 供生图 reference:"quoted"、引用解析等后续功能取用（设计 §2.3）。
// 有界（64 条）+ 10 分钟过期，避免长连接下无限增长。
var yyQQInboundCtx = struct {
	sync.Mutex
	items map[string]yyQQInboundEntry
	order []string
}{items: map[string]yyQQInboundEntry{}}

type yyQQInboundEntry struct {
	in yyQQInbound
	at time.Time
}

const yyQQInboundCtxMax = 64

var yyQQInboundCtxTTL = 10 * time.Minute

// yyQQInboundStore 记一条入站上下文。Raw 事件本体不囤内存，只留结构化字段。
func yyQQInboundStore(in yyQQInbound) {
	if in.ID == "" {
		return
	}
	in.Raw = nil
	yyQQInboundCtx.Lock()
	defer yyQQInboundCtx.Unlock()
	now := time.Now()
	yyQQInboundCtxSweepLocked(now)
	if _, exists := yyQQInboundCtx.items[in.ID]; !exists {
		yyQQInboundCtx.order = append(yyQQInboundCtx.order, in.ID)
	}
	yyQQInboundCtx.items[in.ID] = yyQQInboundEntry{in: in, at: now}
	for len(yyQQInboundCtx.order) > yyQQInboundCtxMax {
		oldest := yyQQInboundCtx.order[0]
		yyQQInboundCtx.order = yyQQInboundCtx.order[1:]
		delete(yyQQInboundCtx.items, oldest)
	}
}

func yyQQInboundCtxSweepLocked(now time.Time) {
	keep := make([]string, 0, len(yyQQInboundCtx.order))
	for _, id := range yyQQInboundCtx.order {
		entry, ok := yyQQInboundCtx.items[id]
		if !ok || now.Sub(entry.at) > yyQQInboundCtxTTL {
			delete(yyQQInboundCtx.items, id)
			continue
		}
		keep = append(keep, id)
	}
	yyQQInboundCtx.order = keep
}

// yyQQCurrentInbound 返回最近一条入站消息的结构化上下文。
func yyQQCurrentInbound() (yyQQInbound, bool) {
	yyQQInboundCtx.Lock()
	defer yyQQInboundCtx.Unlock()
	var best yyQQInboundEntry
	found := false
	for _, entry := range yyQQInboundCtx.items {
		if !found || entry.at.After(best.at) {
			best, found = entry, true
		}
	}
	if !found || time.Since(best.at) > yyQQInboundCtxTTL {
		return yyQQInbound{}, false
	}
	return best.in, true
}

// yyQQInboundLookup 按 message_id 取入站上下文（引用回复的生图参考图用）。
func yyQQInboundLookup(id string) (yyQQInbound, bool) {
	if id == "" {
		return yyQQInbound{}, false
	}
	yyQQInboundCtx.Lock()
	defer yyQQInboundCtx.Unlock()
	entry, ok := yyQQInboundCtx.items[id]
	if !ok || time.Since(entry.at) > yyQQInboundCtxTTL {
		return yyQQInbound{}, false
	}
	return entry.in, true
}

// yyQQInboundRun 是触发词引擎的结构化入口：先记入站上下文，再调用现有
// yyQQTriggerRun（签名与行为保持不变，群消息链路不受影响）。
// 触发词引擎扩展参数（ReplyID / 图片 / at 名单）后续由引擎侧接入，
// 本期先落地结构化与上下文。
func yyQQInboundRun(cf yyQQConfig, in yyQQInbound) bool {
	yyQQInboundStore(in)
	if in.User == "" {
		return false
	}
	return yyQQTriggerRun(cf, in.Name, in.User, in.Role, in.Text, in.ID, in.Mentioned)
}

// yyQQInboundPrivate 处理私聊入站：本期只做结构化记录与触发词，
// 不进游戏转发 / 助手 / 陪聊链路（那些链路现状只接群消息，不新造）。
// 私聊没有群：cf.GroupID 传空串，规则里的 Groups 限定自然不生效。
func yyQQInboundPrivate(cf yyQQConfig, in yyQQInbound) {
	if in.User == "" || in.User == cf.OneBot.BotID {
		return
	}
	if in.ID != "" && !yyQQInboundPrivateClaim(in.ID) {
		return
	}
	yyQQInboundStore(in)
	yyQQLog("QQ私聊消息 " + in.Name + "(" + in.User + ")：" + yyQQClip(yyQQInboundText(in), 200))
	private := cf
	private.GroupID = ""
	if private.Assistant.Enabled && yyQQInboundRun(private, in) {
		return
	}
}

// 私聊去重：适配器重推同一 message_id 时不重复处理（回复幂等）。
var yyQQInboundPrivateSeen = struct {
	sync.Mutex
	items map[string]time.Time
}{items: map[string]time.Time{}}

func yyQQInboundPrivateClaim(id string) bool {
	yyQQInboundPrivateSeen.Lock()
	defer yyQQInboundPrivateSeen.Unlock()
	now := time.Now()
	for key, at := range yyQQInboundPrivateSeen.items {
		if now.Sub(at) > 10*time.Minute {
			delete(yyQQInboundPrivateSeen.items, key)
		}
	}
	if _, seen := yyQQInboundPrivateSeen.items[id]; seen {
		return false
	}
	if len(yyQQInboundPrivateSeen.items) >= 256 {
		// 淘汰最旧一条，新消息永不因表满被丢弃。
		oldestKey, oldestAt := "", time.Now()
		for key, at := range yyQQInboundPrivateSeen.items {
			if oldestKey == "" || at.Before(oldestAt) {
				oldestKey, oldestAt = key, at
			}
		}
		delete(yyQQInboundPrivateSeen.items, oldestKey)
	}
	yyQQInboundPrivateSeen.items[id] = now
	return true
}
