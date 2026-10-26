package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AI 自主社交工具
//
// 用户的要求（原话）：
//   「拍一拍这个功能也让他自己来决定 / 发表情包也是他自己来发 不是我们来发 /
//     他是一个 AI，一个超强大脑，他可以自己来选择怎么做 /
//     群消息总结这个就直接叫他自己弄就好……相当于有这些工具，它可以去调用 /
//     包括他识图、发动态啊什么的全部」
//
// 所以这些能力的主入口是**模型自己决定调用**，不是人敲 `.拍` / `.发图`。
// 群指令仍然保留（手动兜底、也方便调试），但不再是唯一入口。
//
// 设计约束：
//  1. 有副作用的动作（拍人、发图、发动态）一律过 yyQQRunAction，
//     跟群指令共用同一套能力开关 + 冷却 + 间隔 + 每日上限，玩家在面板上
//     关掉某个能力，AI 也调不动。
//  2. 只读动作（看收藏表情、读群消息）不占每日额度，但 view_image 要调
//     视觉模型花钱，单独按人限速。
//  3. 工具返回值是给模型看的事实，不是发到群里的文本。AI 拿到结果后
//     自己组织语言回复。

// yyQQSocialTools 返回社交类工具定义。全员可用（成员也需要能自然互动），
// 但 post_qzone 只在 level>=1 时给出，避免成员借 AI 往空间发东西。
func yyQQSocialTools(level int) []ToolDef {
	str := func() map[string]any { return map[string]any{"type": "string"} }
	tools := []ToolDef{
		{Name: "poke", Description: "拍一拍群里的某个人。什么时候拍由你自己判断：打招呼、叫人、开玩笑、想引起某人注意。不要每句话都拍，也不要连拍同一个人。", Parameters: yySchema(map[string]any{
			"user_id": str(),
			"reason":  str(),
		})},
		{Name: "list_stickers", Description: "列出机器人 QQ 收藏的表情包（序号 + 名字）。想发表情但不确定有哪些时先调这个。", Parameters: yySchema(map[string]any{})},
		// 多图 + 图文解耦（用户原话：「谁告诉你只能发一张了 而且谁告诉你既要
		// 说话又要发表情包？如果我没说的话，你为什么要擅作主张？」）。
		// indexes / urls 是「发图」开关，text 是「说话」开关，两者互不牵连：
		//   - 只发图：只给 indexes/urls，text 留空，发完不要在回复里再补一句话；
		//   - 只说话：别调这个工具，正常回话就行。
		{Name: "send_sticker", Description: "从机器人收藏表情里挑 1~9 张发到当前群（indexes 为序号数组，或 urls 为图片链接数组）。发图和说话是两个独立开关：只发图就把 text 留空，发完不要再补一句话；只想说话就别调这个工具。用户没要表情包时不要自作主张加，也不要每句都配。", Parameters: yySchema(map[string]any{
			"indexes": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "minItems": 1, "maxItems": 9},
			"urls":    map[string]any{"type": "array", "items": str(), "minItems": 1, "maxItems": 9},
			"index":   map[string]any{"type": "integer"},
			"url":     str(),
			"text":    str(),
		})},
		{Name: "send_voice", Description: "想用语音说话时用。text 会转成语音；url 必须是音频直链（http/https）。别每条都发语音，只在情绪强烈、想强调、想卖萌的时候用。", Parameters: yySchema(map[string]any{
			"text": str(),
			"url":  str(),
		})},
		{Name: "group_summary", Description: "读取最近群消息的原始记录（谁在什么时间说了什么）。用户问「刚才聊了啥」「总结一下」时自己调这个，然后由你组织语言总结，不要照抄原始列表。", Parameters: yySchema(map[string]any{
			"limit": map[string]any{"type": "integer"},
		})},
		{Name: "view_image", Description: "用视觉模型看清一张图片的内容。用户发了图或贴了图片链接、你需要知道图里是什么时调它。当前消息里的图通常已经直接挂在本轮对话上，优先直接看；只有看别的图时才调它。", Parameters: yySchema(map[string]any{
			"url":  str(),
			"file": str(),
		}, "url")},
	}
	// 画图：署名权归「让它画的那个人」（用户原话：「画图的话，我还是希望他能
	// 做成那种意义发图者的身份来执行」）。谁提出要求，图就以谁的昵称/@ 发出，
	// 不是以机器人自己的口吻「我画的」。没配绘图模型时不下发这个工具。
	if strings.TrimSpace(yyQQSnapshot().Assistant.ImageModel) != "" && yyQQToolEnabled(yyQQSnapshot().Assistant, "image_generate") {
		tools = append(tools, ToolDef{Name: "draw_image", Description: "画一张图发到当前群，署名归提出要求的那个人（谁让你画就以谁的名义发）。用户明确要图时才用；prompt 只写画面内容，不要写「帮我画」「给我画一张」这类话。", Parameters: yySchema(map[string]any{
			"prompt": str(),
		}, "prompt")})
	}
	if level >= 1 {
		tools = append(tools, ToolDef{Name: "post_qzone", Description: "以机器人自己的身份发一条 QQ 空间动态。用户明确要求发动态、或者你觉得有值得分享的内容时用；内容自己写，不要复述聊天记录。", Parameters: yySchema(map[string]any{
			"text": str(),
		}, "text")})
		// 踢人：能力 MinLevel=管理员。极端情况（严重违规）才用。
		tools = append(tools, ToolDef{Name: "kick", Description: "把严重违规的群成员移出群（不加黑名单）。极端情况才用：有人严重违反群规、惹了众怒时。要有理由，别随便踢。", Parameters: yySchema(map[string]any{
			"user":   str(),
			"reason": str(),
		}, "user")})
	}
	if level >= 2 {
		// 禁言：能力 MinLevel=超级管理员。机器人自己是群管理，有人违规/刷屏/
		// 惹恼你时用；duration 分钟，默认10；要有理由、有分寸，别滥用。
		tools = append(tools, ToolDef{Name: "ban", Description: "禁言群里成员。你是群管理员，有人违规/刷屏/惹恼你时用。duration 分钟，默认10。要有理由、有分寸，别滥用。", Parameters: yySchema(map[string]any{
			"user":     str(),
			"duration": map[string]any{"type": "integer"},
			"reason":   str(),
		}, "user")})
	}
	return tools
}

// yyQQSocialToolNames 判断一个工具名是不是社交类，供执行分发快速路由。
func yyQQSocialToolNames(name string) bool {
	switch name {
	case "poke", "list_stickers", "send_sticker", "draw_image", "send_voice", "group_summary", "view_image", "post_qzone", "ban", "kick":
		return true
	}
	return false
}

// yyStickerCollect 把模型给的参数收成一组待发图片地址（1~9 张）。
//
// 兼容三种写法：
//   - urls / indexes 数组（新写法，一次 1~9 张）
//   - url / index 单值（旧写法，模型和旧提示词都还在用）
//
// 顺序按「先 urls 后 indexes」，不重排，保证模型说的「先发这张再发那张」不变。
func yyStickerCollect(args map[string]any) ([]string, []string, error) {
	urls := make([]string, 0, 9)
	labels := make([]string, 0, 9)
	for _, u := range yyArgStrList(args, "urls") {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		urls = append(urls, u)
		labels = append(labels, yyQQCardClip(u, 40))
	}
	if u := strings.TrimSpace(aiArgStr(args, "url")); u != "" {
		urls = append(urls, u)
		labels = append(labels, yyQQCardClip(u, 40))
	}
	idxs := yyArgIntList(args, "indexes")
	if n := aiArgInt(args, "index", 0); n > 0 {
		idxs = append(idxs, n)
	}
	if len(idxs) > 0 {
		rows, e := yyStickerFaces()
		if e != nil {
			return nil, nil, fmt.Errorf("读取收藏表情失败：%v", e)
		}
		for _, idx := range idxs {
			if idx <= 0 {
				continue
			}
			if idx > len(rows) {
				return nil, nil, fmt.Errorf("没有第 %d 个收藏表情（共 %d 个），先用 list_stickers 看有哪些", idx, len(rows))
			}
			u := strings.TrimSpace(fmt.Sprint(rows[idx-1]["url"]))
			if u == "" {
				return nil, nil, fmt.Errorf("第 %d 个收藏表情没有可用的图片地址", idx)
			}
			urls = append(urls, u)
			labels = append(labels, fmt.Sprintf("收藏表情#%d", idx))
		}
	}
	if len(urls) == 0 {
		return nil, nil, fmt.Errorf("要么给 indexes（1~9 个收藏表情序号，先用 list_stickers 看有哪些），要么给 urls（1~9 个 http/https 图片链接）")
	}
	if len(urls) > 9 {
		return nil, nil, fmt.Errorf("一次最多 9 张，收到 %d 张", len(urls))
	}
	for _, u := range urls {
		if !yyStickerURL(u) || len(u) > 2000 {
			return nil, nil, fmt.Errorf("图片链接必须是 http/https 且不超过 2000 字符")
		}
	}
	return urls, labels, nil
}

// yyStickerSendMany 一条群消息里发 1~9 张图。text 非空时在图片前加一段文字，
// 为空就只发图——文字和图片各走各的开关，谁也不绑定谁。
func yyStickerSendMany(groupID string, urls []string, text string) error {
	if len(urls) == 0 {
		return fmt.Errorf("缺少图片地址")
	}
	g, e := strconv.ParseInt(strings.TrimSpace(yyQQGroupString(groupID)), 10, 64)
	if e != nil {
		return fmt.Errorf("OneBot 群号无效")
	}
	msg := make([]map[string]any, 0, len(urls)+1)
	if s := strings.TrimSpace(text); s != "" {
		msg = append(msg, map[string]any{"type": "text", "data": map[string]any{"text": s}})
	}
	for _, u := range urls {
		msg = append(msg, map[string]any{"type": "image", "data": map[string]any{"file": u}})
	}
	_, e = yyStickerCall("send_group_msg", map[string]any{"group_id": g, "message": msg})
	return e
}

// yyArgIntList 读数组参数里的整数，兼容 []any(float64/json.Number)、[]int、单值。
func yyArgIntList(args map[string]any, key string) []int {
	out := []int{}
	push := func(v any) {
		switch n := v.(type) {
		case float64:
			out = append(out, int(n))
		case int:
			out = append(out, n)
		case json.Number:
			if i, e := n.Int64(); e == nil {
				out = append(out, int(i))
			}
		case string:
			if i, e := strconv.Atoi(strings.TrimSpace(n)); e == nil {
				out = append(out, i)
			}
		}
	}
	switch v := args[key].(type) {
	case []any:
		for _, x := range v {
			push(x)
		}
	case []int:
		out = append(out, v...)
	case []float64:
		for _, x := range v {
			out = append(out, int(x))
		}
	default:
		if v != nil {
			push(v)
		}
	}
	return out
}

// yyArgStrList 读数组参数里的字符串，兼容 []any、[]string、单值。
func yyArgStrList(args map[string]any, key string) []string {
	out := []string{}
	switch v := args[key].(type) {
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, v...)
	case string:
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

// yyQQSocialToolCall 执行社交工具。actor/actorName 是发起这次对话的群成员，
// 用于限流归属和「拍自己」的默认目标。
func yyQQSocialToolCall(ctx context.Context, cf yyQQConfig, name string, args map[string]any, level int, actor, actorName string) (string, bool) {
	fail := func(format string, a ...any) (string, bool) {
		return mustJSON(map[string]any{"ok": false, "error": fmt.Sprintf(format, a...)}), true
	}
	group := yyQQGroupString(cf.GroupID)
	switch name {
	case "poke":
		target := strings.TrimSpace(aiArgStr(args, "user_id"))
		if target == "" {
			target = strings.TrimSpace(actor)
		}
		if !yyQQNumericQQ(target) {
			return fail("user_id 必须是数字 QQ 号；想拍当前说话的人就留空")
		}
		if group == "" {
			return fail("当前没有绑定群，无法拍一拍")
		}
		reason := strings.TrimSpace(aiArgStr(args, "reason"))
		e := yyQQRunAction("poke", actor, actorName, target, "AI 拍 QQ"+target+yyQQCardClip(reason, 20), level, func() (string, error) {
			return "", yyPokeGroup(group, target)
		})
		if e != nil {
			return mustJSON(map[string]any{"ok": false, "error": e.Error()}), true
		}
		return mustJSON(map[string]any{"ok": true, "poked": target}), false

	case "list_stickers":
		rows, e := yyStickerFaces()
		if e != nil {
			return fail("读取收藏表情失败：%v", e)
		}
		out := make([]map[string]any, 0, len(rows))
		for i, r := range rows {
			if i >= 40 {
				break
			}
			row := map[string]any{"index": i + 1}
			if v := strings.TrimSpace(fmt.Sprint(r["name"])); v != "" && v != "<nil>" {
				row["name"] = yyQQCardClip(v, 30)
			}
			// URL 只给前 100 字符用于辨识，真正发送用 index，避免把
			// 几十条长链接塞进上下文把 token 烧光。
			if v := strings.TrimSpace(fmt.Sprint(r["url"])); v != "" && v != "<nil>" {
				row["url_hint"] = yyQQCardClip(v, 100)
			}
			out = append(out, row)
		}
		if len(out) == 0 {
			return mustJSON(map[string]any{"ok": true, "count": 0, "note": "机器人没有收藏表情"}), false
		}
		return mustJSON(map[string]any{"ok": true, "count": len(out), "stickers": out, "note": "发送时把序号传给 send_sticker（可一次给 indexes 数组，1~9 张）"}), false

	case "send_sticker":
		if group == "" {
			return fail("当前没有绑定群，无法发图")
		}
		urls, labels, e := yyStickerCollect(args)
		if e != nil {
			return fail("%v", e)
		}
		// text 是独立开关：留空 = 只发图，不额外说话（不再逼模型「发完再说一句」）。
		text := strings.TrimSpace(aiArgStr(args, "text"))
		detail := "AI 发表情包 " + strconv.Itoa(len(urls)) + " 张"
		if text != "" {
			detail += "（附一句：" + yyQQCardClip(text, 20) + "）"
		}
		if e := yyQQRunAction("sticker", actor, actorName, yyQQCardClip(strings.Join(labels, "、"), 60), detail, level, func() (string, error) {
			return "", yyStickerSendMany(group, urls, text)
		}); e != nil {
			return mustJSON(map[string]any{"ok": false, "error": e.Error()}), true
		}
		return mustJSON(map[string]any{
			"ok":        true,
			"count":     len(urls),
			"labels":    labels,
			"with_text": text != "",
			"note":      "已发完，不用再补一句「表情包发好了」之类的话",
		}), false

	case "draw_image":
		prompt := strings.TrimSpace(aiArgStr(args, "prompt"))
		if prompt == "" {
			return fail("缺少 prompt：要画的画面内容")
		}
		if group == "" {
			return fail("当前没有绑定群，无法发图")
		}
		cur := yyQQSnapshot()
		if strings.TrimSpace(cur.Assistant.ImageModel) == "" {
			return fail("还没配置绘图模型，先去群服互通面板挑一个")
		}
		// 面板「AI 工具 · 画图」关掉就真的画不了：老工具 image_generate 与新工具
		// draw_image 共用同一个开关，避免出现"关了一个另一个还能烧钱"。
		if !yyQQToolEnabled(cur.Assistant, "image_generate") {
			return fail("「画图」工具已在 AI 工具里关闭，先打开再画")
		}
		if e := yyQQDrawRateTake(actor); e != nil {
			return mustJSON(map[string]any{"ok": false, "error": e.Error()}), true
		}
		model := aiStore.Get(cur.Assistant.ImageModel)
		img, e := yyQQGenerateImage(ctx, model, prompt)
		if e != nil {
			return fail("画图失败：%v", e)
		}
		// 署名权归提出要求的人：图消息里 @ 他并注明「要的图」，不是机器人自夸。
		if e := yyQQImageSendAs(cf, img, "", actor, actorName); e != nil {
			return fail("图画好了但发送失败：%v", e)
		}
		return mustJSON(map[string]any{"ok": true, "by": actorName, "note": "已以 " + actorName + " 的名义发出，不用再说「我画好了」"}), false

	case "send_voice":
		if group == "" {
			return fail("当前没有绑定群，无法发语音")
		}
		text := strings.TrimSpace(aiArgStr(args, "text"))
		url := strings.TrimSpace(aiArgStr(args, "url"))
		if text == "" && url == "" {
			return fail("要么给 text（要念的话，≤300字），要么给 url（音频直链）")
		}
		if url != "" && !yyVoiceURL(url) {
			return fail("url 必须是 http/https 音频直链或 base64:// 数据")
		}
		label := "语音：" + yyQQCardClip(url, 50)
		if text != "" {
			label = "语音：" + yyQQCardClip(text, 40)
		}
		clipped := false
		e := yyQQRunAction("voice", actor, actorName, label, label, level, func() (string, error) {
			raw := url
			if raw == "" {
				raw = text
			}
			file, cut, err := yyQQVoicePrepare(cf, raw)
			clipped = cut
			if err != nil {
				return "", err
			}
			return "", yyVoiceSend(group, file)
		})
		if e != nil {
			return mustJSON(map[string]any{"ok": false, "error": e.Error()}), true
		}
		out := map[string]any{"ok": true, "sent": label}
		if clipped {
			out["note"] = fmt.Sprintf("文本超过 %d 字，只念了前 %d 字", yyVoiceTextMax, yyVoiceTextMax)
		}
		return mustJSON(out), false

	case "group_summary":
		n := aiArgInt(args, "limit", 0)
		if n <= 0 {
			n = 40
		}
		facts := yySummaryFacts(cf, n)
		return mustJSON(map[string]any{"ok": true, "facts": facts, "note": "这是原始记录，请用你自己的话总结，不要逐条复述"}), false

	case "view_image":
		url := strings.TrimSpace(aiArgStr(args, "url"))
		if url == "" {
			// 图片段只有 file（QQ 内部 ID）时，模型多半是从 CQ 码里抄的，
			// 后端负责调 get_image 换成可访问地址，不然表情包永远看不了。
			if file := strings.TrimSpace(aiArgStr(args, "file")); file != "" {
				url = yyVisionResolveFile(file)
			}
		}
		if url == "" {
			return fail("缺少 url；消息里是 CQ 码 image 段时，把它的 file 字段传给 file 参数，后端会自动换成可访问地址")
		}
		if _, e := yyVisionImagePart(url); e != nil {
			return fail("%v", e)
		}
		if e := yyVisionRateTake(actor); e != nil {
			return mustJSON(map[string]any{"ok": false, "error": e.Error()}), true
		}
		m := aiStore.Get(cf.Assistant.Model)
		if m == nil {
			return fail("没有配置可用的 AI 模型")
		}
		desc, e := yyVisionDescribeURL(ctx, m, url)
		if e != nil {
			return fail("看图失败：%v", e)
		}
		return mustJSON(map[string]any{"ok": true, "url": url, "description": desc}), false

	case "post_qzone":
		text := strings.TrimSpace(aiArgStr(args, "text"))
		if text == "" {
			return fail("动态内容不能为空")
		}
		if len([]rune(text)) > 500 {
			return fail("动态内容最长 500 字")
		}
		text = strings.Join(strings.Fields(text), " ")
		tid := ""
		e := yyQQRunAction("qzone", actor, actorName, "", yyQQCardClip(text, 40), level, func() (string, error) {
			out, err := yyQQActionQzone(text, 1)
			tid = out
			return "", err
		})
		if e != nil {
			return mustJSON(map[string]any{"ok": false, "error": e.Error()}), true
		}
		return mustJSON(map[string]any{"ok": true, "tid": tid}), false

	case "ban":
		ref := strings.TrimSpace(aiArgStr(args, "user"))
		if ref == "" {
			return fail("缺少 user：要禁言的成员（@某人、显示名或 QQ 号）")
		}
		user := yyQQResolveGroupUser(cf, ref)
		if user == "" {
			return fail("没找到这个人（%s），把 QQ 号或完整昵称给我", yyQQCardClip(ref, 30))
		}
		minutes := aiArgInt(args, "duration", 10)
		if minutes <= 0 {
			minutes = 10
		}
		if minutes > 43200 { // 30 天封顶
			minutes = 43200
		}
		if group == "" {
			return fail("当前没有绑定群，无法禁言")
		}
		reason := strings.TrimSpace(aiArgStr(args, "reason"))
		detail := "AI 禁言 " + yyQQCardClip(ref, 30) + " " + strconv.Itoa(minutes) + " 分钟"
		if reason != "" {
			detail += "（" + yyQQCardClip(reason, 40) + "）"
		}
		if e := yyQQRunAction("ban", actor, actorName, user, detail, level, func() (string, error) {
			return "", yyQQActionBan(group, user, minutes)
		}); e != nil {
			return mustJSON(map[string]any{"ok": false, "error": e.Error()}), true
		}
		return mustJSON(map[string]any{"ok": true, "banned": user, "minutes": minutes}), false

	case "kick":
		ref := strings.TrimSpace(aiArgStr(args, "user"))
		if ref == "" {
			return fail("缺少 user：要移出的成员（@某人、显示名或 QQ 号）")
		}
		user := yyQQResolveGroupUser(cf, ref)
		if user == "" {
			return fail("没找到这个人（%s），把 QQ 号或完整昵称给我", yyQQCardClip(ref, 30))
		}
		if group == "" {
			return fail("当前没有绑定群，无法移出成员")
		}
		reason := strings.TrimSpace(aiArgStr(args, "reason"))
		detail := "AI 踢人 " + yyQQCardClip(ref, 30)
		if reason != "" {
			detail += "（" + yyQQCardClip(reason, 40) + "）"
		}
		if e := yyQQRunAction("kick", actor, actorName, user, detail, level, func() (string, error) {
			return "", yyQQActionKick(group, user, false)
		}); e != nil {
			return mustJSON(map[string]any{"ok": false, "error": e.Error()}), true
		}
		return mustJSON(map[string]any{"ok": true, "kicked": user}), false
	}
	return mustJSON(map[string]any{"ok": false, "error": "未知社交工具"}), true
}

// yyQQAISelfRoutes 暴露「AI 现在到底能用什么」。
//
// 用户原话：「QQ机器人里面的AI能够自主发消息，现在都还没条件，跟他没存在一样」。
// 他看不到 AI 有哪些工具、模型支不支持看图、主动接话开没开——
// 没有这个视图，任何「已经做好了」都只能靠我嘴说。所以把它做成可查的。
func yyQQAISelfRoutes(mux *http.ServeMux) {
	// 超管认领码二维码（GET /api/qq/claim/qr）。跟 selfcheck 一样是助手面板的
	// 附属端点，挂在同一处注册，避免再改 youyuan_qq.go 的注册列表。
	yyQQClaimQRRoutes(mux)
	mux.HandleFunc("/api/qq/ai/selfcheck", func(w http.ResponseWriter, r *http.Request) {
		cf := yyQQSnapshot()
		model := aiStore.Get(cf.Assistant.Model)
		modelID, provider := "", ""
		if model != nil {
			modelID, provider = model.ModelID, string(model.Provider)
		}
		// 用最高权限看全量工具，这样用户能知道「AI 本来会什么」，
		// 再对照 level 看自己能用哪些。已被用户在「AI 工具」里关掉的项不出现
		// （selfcheck 说的是「现在真的给了什么」，不是「代码里写了什么」）。
		names := yyQQToolNames(true, 2)
		social := make([]string, 0, 6)
		for _, t := range yyQQSocialTools(2) {
			if yyQQToolEnabled(cf.Assistant, t.Name) {
				social = append(social, t.Name)
			}
		}
		writeJSON(w, map[string]any{
			"ok":               true,
			"model_name":       cf.Assistant.Model,
			"model_id":         modelID,
			"provider":         provider,
			"model_found":      model != nil,
			"supports_vision":  yyQQVisionEnabled(cf),
			"vision_mode":      yyQQVisionMode(cf),
			"tools":            names,
			"tool_count":       len(names),
			"social_tools":     social,
			"tool_groups":      yyQQToolGroupsView(cf.Assistant, 2),
			"tool_total":       len(yyQQToolDefs),
			"companion":        cf.Assistant.Companion,
			"companion_window": cf.Assistant.CompanionWindow,
			"image_model":      cf.Assistant.ImageModel != "",
			"enabled":          cf.Enabled,
		})
	})
}

// 视觉请求要额外花钱、还慢，AI 一次对话里连续看十张图会把预算烧穿。
// yyVisionRate 看图限速：每人每分钟 3 次。
// 视觉请求要额外花钱、还慢，AI 一次对话里连续看十张图会把预算烧穿。
var yyVisionRate = struct {
	sync.Mutex
	Hits map[string][]time.Time
}{Hits: map[string][]time.Time{}}

func yyVisionRateTake(user string) error {
	if user == "" {
		user = "anonymous"
	}
	now := time.Now()
	yyVisionRate.Lock()
	defer yyVisionRate.Unlock()
	if len(yyVisionRate.Hits) > 512 {
		for k, v := range yyVisionRate.Hits {
			if len(v) == 0 || now.Sub(v[len(v)-1]) > time.Minute {
				delete(yyVisionRate.Hits, k)
			}
		}
	}
	kept := yyVisionRate.Hits[user][:0]
	for _, at := range yyVisionRate.Hits[user] {
		if now.Sub(at) < time.Minute {
			kept = append(kept, at)
		}
	}
	if len(kept) >= 3 {
		yyVisionRate.Hits[user] = kept
		return fmt.Errorf("一分钟内看图次数已用完（3 次），稍后再试")
	}
	yyVisionRate.Hits[user] = append(kept, now)
	return nil
}

// yyVisionDescribeURL 让视觉模型描述一张图片。
// 只在「模型支持视觉」时才允许调用，否则退回一句明确说明，
// 免得把图片塞给纯文本模型导致请求报错。
func yyVisionDescribeURL(ctx context.Context, model *AIModel, url string) (string, error) {
	if model == nil {
		return "", fmt.Errorf("模型未配置")
	}
	if !yyQQVisionEnabled(yyQQSnapshot()) {
		return "", fmt.Errorf("当前群未开启识图（视觉模式 auto/forced_off 且模型名检测不到视觉能力）。在 QQ 助手设置里把「多模态识图」设为强制开启，或换一个模型名带 vision/vl/4o 等的模型")
	}
	turns := []aiTurn{
		{Role: "system", Text: "你在看群聊里的一张图。只用中文客观描述：主体是什么、图里有没有文字（照抄）、如果是表情包说明它表达的情绪或梗。看不清就说看不清，不要编造。"},
		{Role: "user", Text: "这张图里是什么？", Images: []string{url}},
	}
	turn, e := model.complete(ctx, turns, nil, aiStreamHooks{})
	if e != nil {
		return "", e
	}
	desc := strings.TrimSpace(turn.Text)
	if desc == "" {
		return "", fmt.Errorf("模型没有返回描述")
	}
	return yyQQClip(desc, 1200), nil
}

// yySummaryFacts 输出「最近 N 条消息」的原始事实（发言排行 + 原话），
// 供 AI 自己组织语言总结。与 .总结 指令共用，保证两边看到的是同一份数据。
func yySummaryFacts(cf yyQQConfig, n int) string {
	if n < 5 {
		n = 5
	}
	if n > yySummaryMax {
		n = yySummaryMax
	}
	rows := yyQQRecentMessages(cf, n)
	if len(rows) == 0 {
		return "最近没有可用的群消息记录（机器人只统计自己看到过的消息，重启前的历史不在）。"
	}
	head := []string{fmt.Sprintf("时间范围：%s–%s，共 %d 条", yySummaryClock(rows[0]), yySummaryClock(rows[len(rows)-1]), len(rows)), "发言量：" + yySummaryTop(rows), "原始消息（时间 发言人：内容）："}

	// 按字符预算从**最新**往回取。
	// 工具输出在宿主侧会被截到 5000 字符，如果顺着时间正序拼、再被截断，
	// 留下的恰好是最早的消息——用户问「刚才聊了啥」时最新的全丢了。
	// 所以这里自己先裁：最新的一定在里面，太早的按预算丢掉并明确告知。
	const budget = 4200
	picked := make([]string, 0, len(rows))
	used := 0
	for i := len(rows) - 1; i >= 0; i-- {
		m := rows[i]
		text := strings.TrimSpace(strings.ReplaceAll(m.Text, "\n", " "))
		if text == "" {
			text = "（非文字消息）"
		}
		if runes := []rune(text); len(runes) > 100 {
			text = string(runes[:100]) + "…"
		}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = "未知"
		}
		line := yySummaryClock(m) + " " + name + "：" + text
		if used+len([]rune(line)) > budget && len(picked) > 0 {
			break
		}
		picked = append(picked, line)
		used += len([]rune(line))
	}
	// picked 是倒序，翻回正序再输出。
	for i, j := 0, len(picked)-1; i < j; i, j = i+1, j-1 {
		picked[i], picked[j] = picked[j], picked[i]
	}
	out := append(head, picked...)
	if len(picked) < len(rows) {
		out = append(out, fmt.Sprintf("（另有 %d 条更早的消息未列出，需要的话把 limit 调小再看）", len(rows)-len(picked)))
	}
	return strings.Join(out, "\n")
}
