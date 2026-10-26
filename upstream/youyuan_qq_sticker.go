package main

import (
	"fmt"
	"strconv"
	"strings"
)

// 发表情包：用户显式发 `.发图 <URL>` 或 `.发图 <收藏表情序号>` 时，
// 以机器人身份往当前群发一条只含图片段的消息。
//
// 注意：机器人的收藏表情是给它自己「看」的（见 .表情包 / yyQQFetchFaces），
// 除非用户点名序号，否则绝不批量往群里刷图。

var (
	// 变量别名，只为测试能打桩；逻辑上仍是 OneBot 调用与收藏表情读取。
	yyStickerCall  = yyQQOneBotCall
	yyStickerFaces = yyQQFetchFaces
)

// yyStickerSend 往群里发一条只有图片段的消息。段格式与 yyQQImageSend 一致：
// send_group_msg + message 数组，图片地址放在 data.file。
func yyStickerSend(groupID, imageURL string) error {
	url := strings.TrimSpace(imageURL)
	if url == "" {
		return fmt.Errorf("缺少图片地址")
	}
	g, e := strconv.ParseInt(strings.TrimSpace(yyQQGroupString(groupID)), 10, 64)
	if e != nil {
		return fmt.Errorf("OneBot 群号无效")
	}
	msg := []map[string]any{{
		"type": "image",
		"data": map[string]any{"file": url},
	}}
	_, e = yyStickerCall("send_group_msg", map[string]any{"group_id": g, "message": msg})
	return e
}

// yyStickerURL 只认 http/https，其它（data:、file://、base64）一律拒绝。
func yyStickerURL(u string) bool {
	lower := strings.ToLower(strings.TrimSpace(u))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// yyQQCmdSticker .发图 <图片URL 或 收藏表情序号>：受能力开关与限流约束。
func yyQQCmdSticker(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	raw := strings.TrimSpace(arg)
	if raw == "" {
		return reply("格式：.发图 图片URL，或 .发图 序号（先发 .表情包 看序号）")
	}

	url := raw
	target := yyQQCardClip(raw, 60)

	// 纯数字当收藏表情序号（从 1 开始，与 .表情包 的排列一致）。
	if n, e := strconv.Atoi(raw); e == nil {
		if n < 1 {
			return reply("收藏表情序号从 1 开始，先发 .表情包 看机器人收藏了哪些图。")
		}
		rows, err := yyStickerFaces()
		if err != nil {
			return reply("读取收藏表情失败：" + err.Error())
		}
		if n > len(rows) {
			return reply(fmt.Sprintf("没有第 %d 个收藏表情（机器人现在收藏了 %d 个），先发 .表情包 查看序号。", n, len(rows)))
		}
		url = strings.TrimSpace(fmt.Sprint(rows[n-1]["url"]))
		if url == "" {
			return reply("这个收藏表情没有可用的图片地址，换一个序号试试。")
		}
		target = fmt.Sprintf("收藏表情#%d", n)
	} else {
		if !yyStickerURL(url) {
			return reply("只支持 http/https 图片链接，或收藏表情序号（先发 .表情包 看序号）。")
		}
		if len(url) > 2000 {
			return reply("图片链接太长（最多 2000 字符），换一个短链接。")
		}
	}

	group := yyQQGroupString(cf.GroupID)
	if group == "" {
		return reply("还没选群，无法发图。")
	}
	e := yyQQRunAction("sticker", user, name, target, target, level, func() (string, error) {
		return "", yyStickerSend(group, url)
	})
	if e != nil {
		return reply("发表情包失败：" + e.Error())
	}
	return reply("表情包已发出。")
}
