package main

// ═══════════════════════════════════════════════════════════════════════
// youyuan_qq_quote.go —— QQ「引用回复」的原文解析（2026-10-26 第 29 条）
//
// 【用户反馈原话】「机器人在QQ群里面告诉我，他看不到引用的原文」
//
// 【根因：写了没接线】入站解析早就把被引用消息塞进了
// yyQQInbound.ReplyText / ReplyUser（youyuan_qq_inbound.go 的
// yyQQInboundResolveReply），但整条 AI 链路读的是 yyQQGroupMessage，
// 它只有 ReplyID 没有正文；ReplyText 全仓**没有任何一处被读过**。
// 于是模型只拿到一个 reply_to_message_id，按提示词里那句
// 「缺少引用原文时明确未知，不补造对话」老老实实回答「我看不到原文」。
//
// 【修法】三层，从便宜到贵：
//   1. 本地群消息窗口（80 条）里直查 —— 零网络调用，覆盖绝大多数场景
//   2. 窗口里没有就调 OneBot get_msg 按 id 拉一次，结果进有界缓存（5 分钟）
//   3. 都拿不到才留空，这时模型说「看不到」才是诚实的
//
// 权限纪律：引用的正文同样走 yyQQHideCoordinates（非管理员看不到坐标），
// 不能因为它是「引用」就绕开脱敏。
// ═══════════════════════════════════════════════════════════════════════

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

const (
	yyQQQuoteCacheMax = 256
	yyQQQuoteTextMax  = 400 // 进上下文的引用正文上限（字符）
)

type yyQQQuoteEntry struct {
	text string
	user string
	name string
	ok   bool
	at   time.Time
}

var yyQQQuoteCache = struct {
	sync.Mutex
	items map[string]yyQQQuoteEntry
	order []string
}{items: map[string]yyQQQuoteEntry{}}

var yyQQQuoteTTL = 5 * time.Minute

// yyQQQuoteStripCQ 把 CQ 码串转成人能读的形态（图片/表情/at 用占位符）。
// 群消息缓存里的 Text 是 CQ 原文，直接丢给模型会看到一大堆 [CQ:image,file=...]。
func yyQQQuoteStripCQ(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "[CQ:") {
		// 没有 CQ 码也要限长：纯文本同样可能是一整段长文
		if runes := []rune(raw); len(runes) > yyQQQuoteTextMax {
			return string(runes[:yyQQQuoteTextMax]) + "…"
		}
		return raw
	}
	var b strings.Builder
	for i := 0; i < len(raw); {
		if strings.HasPrefix(raw[i:], "[CQ:") {
			end := strings.Index(raw[i:], "]")
			if end < 0 {
				b.WriteString(raw[i:])
				break
			}
			body := raw[i+4 : i+end]
			i += end + 1
			kind := body
			if comma := strings.IndexAny(body, ","); comma >= 0 {
				kind = body[:comma]
			}
			switch kind {
			case "image":
				b.WriteString("[图片]")
			case "face", "mface":
				b.WriteString("[表情]")
			case "at":
				if qq := yyQQCQParam(body, "qq"); qq != "" {
					if qq == "all" {
						b.WriteString("@全体成员")
					} else {
						b.WriteString("@" + qq)
					}
				}
			case "reply":
				// 引用段本身不产出正文
			case "record":
				b.WriteString("[语音]")
			case "video":
				b.WriteString("[视频]")
			case "file":
				b.WriteString("[文件]")
			default:
				b.WriteString("[" + kind + "]")
			}
			continue
		}
		b.WriteByte(raw[i])
		i++
	}
	out := strings.TrimSpace(strings.ReplaceAll(b.String(), "&#91;", "["))
	// 截断放在这里而不是各调用点：只要正文从这条管道出来就一定有界，
	// 不会因为某个新调用方忘了裁剪就把 2000 字灌进上下文。
	if runes := []rune(out); len(runes) > yyQQQuoteTextMax {
		out = string(runes[:yyQQQuoteTextMax]) + "…"
	}
	return out
}

// yyQQCQParam 从 CQ 段体里取参数值（body 形如 image,file=abc,url=http://...）。
func yyQQCQParam(body, key string) string {
	for _, part := range strings.Split(body, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, key+"=") {
			return strings.TrimPrefix(part, key+"=")
		}
	}
	return ""
}

// yyQQQuoteFromRows 在本地群消息窗口里按 message_id 找被引用消息。
func yyQQQuoteFromRows(rows []yyQQGroupMessage, id string) (yyQQGroupMessage, bool) {
	if id == "" {
		return yyQQGroupMessage{}, false
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].ID == id {
			return rows[i], true
		}
	}
	return yyQQGroupMessage{}, false
}

// yyQQQuoteCached 读缓存。
func yyQQQuoteCached(id string) (yyQQQuoteEntry, bool) {
	yyQQQuoteCache.Lock()
	defer yyQQQuoteCache.Unlock()
	entry, ok := yyQQQuoteCache.items[id]
	if !ok {
		return yyQQQuoteEntry{}, false
	}
	if time.Since(entry.at) > yyQQQuoteTTL {
		delete(yyQQQuoteCache.items, id)
		return yyQQQuoteEntry{}, false
	}
	return entry, true
}

// yyQQQuoteStore 写缓存（有界 + 按插入顺序淘汰）。
func yyQQQuoteStore(id string, entry yyQQQuoteEntry) {
	if id == "" {
		return
	}
	entry.at = time.Now()
	yyQQQuoteCache.Lock()
	defer yyQQQuoteCache.Unlock()
	if _, exists := yyQQQuoteCache.items[id]; !exists {
		yyQQQuoteCache.order = append(yyQQQuoteCache.order, id)
	}
	yyQQQuoteCache.items[id] = entry
	for len(yyQQQuoteCache.order) > yyQQQuoteCacheMax {
		oldest := yyQQQuoteCache.order[0]
		yyQQQuoteCache.order = yyQQQuoteCache.order[1:]
		delete(yyQQQuoteCache.items, oldest)
	}
}

// yyQQQuoteResolve 解析被引用消息的正文与发送者。
//
// 顺序：缓存 → 本地 80 条窗口 → OneBot get_msg。
// 返回的 text 已去掉 CQ 码、已裁剪长度；拿不到时 ok=false。
// 注意：**不在这里做坐标脱敏**，脱敏由调用方按当前查看者的权限决定。
func yyQQQuoteResolve(cf yyQQConfig, id string) (text, user, name string, ok bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", "", false
	}
	if entry, hit := yyQQQuoteCached(id); hit {
		return entry.text, entry.user, entry.name, entry.ok
	}

	// ① 本地窗口
	if row, hit := yyQQQuoteFromRows(yyQQRecentMessages(cf, 80), id); hit {
		text = yyQQQuoteStripCQ(row.Text)
		if len([]rune(text)) > yyQQQuoteTextMax {
			text = string([]rune(text)[:yyQQQuoteTextMax]) + "…"
		}
		user, name = row.User, row.Name
		if name == "" {
			name = yyQQDisplayName(cf, row.User, "")
		}
		entry := yyQQQuoteEntry{text: text, user: user, name: name, ok: text != ""}
		yyQQQuoteStore(id, entry)
		return text, user, name, entry.ok
	}

	// ② OneBot get_msg（只有 onebot 适配器能按 id 拉；官方接口没有这个能力）
	if cf.Adapter == "onebot" && cf.Enabled {
		if data, err := yyQQOneBotCall("get_msg", map[string]any{"message_id": yyQQMessageIDArg(id)}); err == nil {
			text, user, name = yyQQQuoteFromGetMsg(data)
			if text != "" {
				entry := yyQQQuoteEntry{text: text, user: user, name: name, ok: true}
				yyQQQuoteStore(id, entry)
				return text, user, name, true
			}
		}
	}

	// ③ 拿不到：记一个负缓存，避免同一 id 被反复打网络
	yyQQQuoteStore(id, yyQQQuoteEntry{ok: false})
	return "", "", "", false
}

// yyQQMessageIDArg OneBot 的 message_id 多数实现要整数，纯数字就转 int。
func yyQQMessageIDArg(id string) any {
	allDigits := id != ""
	for i := 0; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			allDigits = false
			break
		}
	}
	if allDigits && len(id) <= 18 {
		var n int64
		for i := 0; i < len(id); i++ {
			n = n*10 + int64(id[i]-'0')
		}
		return n
	}
	return id
}

// yyQQQuoteFromGetMsg 从 OneBot get_msg 的 data 里取正文与发送者。
// message 可能是 CQ 字符串，也可能是 segment 数组，两种都吃。
func yyQQQuoteFromGetMsg(data any) (text, user, name string) {
	obj, ok := data.(map[string]any)
	if !ok {
		return "", "", ""
	}
	switch raw := obj["message"].(type) {
	case string:
		text = yyQQQuoteStripCQ(raw)
	case []any:
		var b strings.Builder
		for _, item := range raw {
			seg, ok := item.(map[string]any)
			if !ok {
				continue
			}
			kind, _ := seg["type"].(string)
			segData, _ := seg["data"].(map[string]any)
			switch kind {
			case "text":
				if s, ok := segData["text"].(string); ok {
					b.WriteString(s)
				}
			case "image":
				b.WriteString("[图片]")
			case "face", "mface":
				b.WriteString("[表情]")
			case "at":
				if qq, ok := segData["qq"].(string); ok && qq != "" {
					if qq == "all" {
						b.WriteString("@全体成员")
					} else {
						b.WriteString("@" + qq)
					}
				}
			case "reply":
				// 不产出正文
			default:
				if kind != "" {
					b.WriteString("[" + kind + "]")
				}
			}
		}
		text = b.String()
	}
	if sender, ok := obj["sender"].(map[string]any); ok {
		if v, ok := sender["user_id"]; ok {
			user = yyQQStableID(v)
		}
		if s, ok := sender["card"].(string); ok && s != "" {
			name = s
		} else if s, ok := sender["nickname"].(string); ok {
			name = s
		}
	}
	if user == "" {
		user = yyQQStableID(obj["user_id"])
	}
	text = strings.TrimSpace(text)
	if len([]rune(text)) > yyQQQuoteTextMax {
		text = string([]rune(text)[:yyQQQuoteTextMax]) + "…"
	}
	return text, user, name
}

// yyQQQuoteFields 给提示词用的字段：把引用信息塞进一条消息的 map 里。
// admin=false 时正文同样要脱敏（不能因为它是引用就漏坐标）。
func yyQQQuoteFields(cf yyQQConfig, out map[string]any, replyID string, admin bool) {
	if replyID == "" || out == nil {
		return
	}
	text, user, name, ok := yyQQQuoteResolve(cf, replyID)
	if !ok || text == "" {
		out["reply_to_text_available"] = false
		return
	}
	if !admin {
		text = yyQQHideCoordinates(text)
	}
	out["reply_to_text"] = text
	out["reply_to_text_available"] = true
	if user != "" {
		out["reply_to_sender"] = yyQQIdentityRef(cf, user)
	}
	if name != "" {
		out["reply_to_sender_name"] = yyQQClip(name, 64)
	}
}

// yyQQQuoteDebugJSON 给测试/排错用：把一次解析过程的结果原样吐出。
func yyQQQuoteDebugJSON(cf yyQQConfig, id string) string {
	text, user, name, ok := yyQQQuoteResolve(cf, id)
	raw, _ := json.Marshal(map[string]any{"id": id, "ok": ok, "text": text, "user": user, "name": name})
	return string(raw)
}
