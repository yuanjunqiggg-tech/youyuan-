package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// 识图：把群消息里的图片变成 AI 能吃的多模态片段。
// 拼接与解析是纯函数；换链（get_image）需要过 OneBot，由 yyVisionResolveFile
// 单独封装，调用方自行把握时机。

// yyVisionUnescapeJSON 还原 JSON 字符串转义。从 json.Marshal 后的文本里
// 抽 URL 时，and 符号、尖括号会被转成反杠u开头的六位十六进制字面量，
// 不还原的话链接是坏的（QQ 多媒体链接必带签名参数，必然踩中）。
func yyVisionUnescapeJSON(s string) string {
	if !strings.ContainsRune(s, 92) {
		return s
	}
	// 用 strconv.Unquote 还原 JSON 转义，比自己写 Replacer 稳——手写
	// 转义字面量在这套改造链路里已经被坑过两次。反引号包一层让它变成
	// 合法 Go 字符串字面量；解析失败就原样返回，不影响主流程。
	if v, err := strconv.Unquote(`"` + s + `"`); err == nil {
		return v
	}
	return s
}

// yyVisionResolveFile 把 OneBot 图片段的 file 字段换成可访问的 http(s) URL。
// 群里的表情包/市场表情常常只带 file（QQ 内部文件 ID，如 "{ABC...}.jpg"）
// 不带 url，不换链视觉模型就永远拿不到这些图。走 OneBot get_image action，
// 8 秒超时；任何失败都返回空串，由调用方降级（当没这张图）。
func yyVisionResolveFile(file string) string {
	file = strings.TrimSpace(file)
	if file == "" || yyVisionHTTPURL(file) {
		return ""
	}
	data, err := yyQQOneBotCall("get_image", map[string]any{"file": file})
	if err != nil {
		return ""
	}
	obj, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"url", "file"} {
		if u := strings.TrimSpace(fmt.Sprint(obj[key])); yyVisionHTTPURL(u) {
			return u
		}
	}
	return ""
}

// yyVisionCollectURLs 收集一条群消息里所有能喂给视觉模型的图片 URL（上限 3）。
// 两级来源：
//  1. 段里直接带的 url / http 形态的 file（yyVisionExtractURLs）；
//  2. 只有 file（QQ 内部 ID）的段，调 get_image 换链——表情包和市场表情
//     基本都是这种形态，不换链就一张都看不到。
func yyVisionCollectURLs(cf yyQQConfig, message string) []string {
	urls := yyVisionExtractURLs(message)
	seen := map[string]bool{}
	for _, u := range urls {
		seen[u] = true
	}
	for _, seg := range yyQQParseCQSegments(message) {
		if len(urls) >= 3 {
			break
		}
		if seg.Type != "image" {
			continue
		}
		// 已经有直链的段跳过（get_image 是网络调用，能省则省）。
		if yyVisionHTTPURL(strings.TrimSpace(seg.Data["url"])) || yyVisionHTTPURL(strings.TrimSpace(seg.Data["file"])) {
			continue
		}
		if u := yyVisionResolveFile(seg.Data["file"]); u != "" && !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	if len(urls) > 3 {
		urls = urls[:3]
	}
	return urls
}

// yyQQVisionEnabled 是「这个群现在到底能不能看图」的唯一判据。
// 三态：cf.Assistant.Vision == nil → 按模型名自动检测（旧行为）；
// true → 强制开启；false → 强制关闭。
// 自动检测只认模型名里的 vision/vl/4o/claude/gemini 等字样，DeepSeek 这类
// 实际支持视觉但名字看不出来的模型会被漏判——用户反馈「多模态模型看不到
// 表情包和照片」就是这个根因，所以必须给强制档，并让面板可查。
func yyQQVisionEnabled(cf yyQQConfig) bool {
	if cf.Assistant.Vision != nil {
		return *cf.Assistant.Vision
	}
	m := aiStore.Get(cf.Assistant.Model)
	if m == nil {
		return false
	}
	return yyAIVisionModel(m.ModelID)
}

// yyQQVisionMode 返回当前视觉判定模式，供面板/自查接口显示：
// auto / forced_on / forced_off。
func yyQQVisionMode(cf yyQQConfig) string {
	if cf.Assistant.Vision == nil {
		return "auto"
	}
	if *cf.Assistant.Vision {
		return "forced_on"
	}
	return "forced_off"
}

// yyVisionImagePart 把图片 URL 转成 OpenAI 兼容的多模态 image_url 片段：
//
//	{"type":"image_url","image_url":{"url":"https://..."}}
//
// 只接受 http(s)；data:（内联 base64）和指向内网的地址一律拒绝，
// 否则模型侧或机器人侧会被当成跳板去读内网资源（SSRF）。
func yyVisionImagePart(rawURL string) (map[string]any, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("图片地址为空")
	}
	lower := strings.ToLower(rawURL)
	if strings.HasPrefix(lower, "data:") {
		return nil, fmt.Errorf("不支持 data: 内联图片，请提供 http(s) 链接")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("图片地址无效：%v", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("只支持 http/https 图片链接")
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("图片地址缺少主机名")
	}
	if yyVisionInternalHost(host) {
		return nil, fmt.Errorf("不允许访问内网地址")
	}
	return map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url": rawURL,
		},
	}, nil
}

// yyVisionInternalHost 判断主机名是否指向本机/内网。只做字面判断，
// 不做 DNS 解析——纯函数不能有网络副作用。
func yyVisionInternalHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if h == "" {
		return true
	}
	if h == "localhost" || strings.HasSuffix(h, ".localhost") ||
		strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	if ip == nil {
		// 普通域名：无法离线判断解析结果，放行由出网侧兜底。
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

// yyVisionExtractURLs 从一段消息文本里抽图片地址，支持两种来源：
//  1. OneBot 的 CQ 码图片段：`[CQ:image,file=abc.jpg,url=https://...]`，
//     取 url 参数；部分适配器把直链放在 file 里，file 是 http(s) 时也认。
//  2. 直接写在文本里的图片直链：以 .jpg/.jpeg/.png/.gif/.webp 结尾的 http(s) 地址。
//
// CQ 码交给现成的 yyQQParseCQSegments 解析，不自己用正则拆逗号与转义。
// 返回结果按出现顺序去重，非法或不带图片后缀的链接不会被当成图片。
func yyVisionExtractURLs(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	out := make([]string, 0, 2)
	seen := map[string]bool{}
	add := func(u string) {
		u = yyVisionUnescapeJSON(strings.TrimSpace(u))
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}

	// 来源一：CQ 码图片段。
	for _, seg := range yyQQParseCQSegments(text) {
		if seg.Type != "image" {
			continue
		}
		if u := strings.TrimSpace(seg.Data["url"]); yyVisionHTTPURL(u) {
			add(u)
		}
		if u := strings.TrimSpace(seg.Data["file"]); yyVisionHTTPURL(u) {
			add(u)
		}
	}

	// 来源二：裸链接。群里的常见写法是「看这个https://a.com/x.jpg，好看吗」——
	// 链接前后都直接粘中文，中间没有空白，Fields 不会在这里断开。所以先从
	// 第一个 http(s) 出现的位置开始取，再切掉「粘在链接后面的中文/右括号」。
	for _, field := range strings.Fields(text) {
		if field == "" || strings.Contains(field, "[CQ:") {
			continue
		}
		start := -1
		for _, mark := range []string{"http://", "https://"} {
			if i := strings.Index(field, mark); i >= 0 && (start < 0 || i < start) {
				start = i
			}
		}
		if start < 0 {
			continue
		}
		u := yyVisionCutURL(field[start:])
		if !yyVisionHTTPURL(u) {
			continue
		}
		if yyVisionImageURL(u) {
			add(u)
		}
	}
	return out
}

// yyVisionCutURL 从「链接+紧跟的中文」里切出链接本身。
// 群消息常见写法是「看这个https://a.com/x.jpg，好看吗」，链接后面直接粘着
// 中文或右括号，所以遇到非 ASCII 字符或闭合括号就在那里截断。
func yyVisionCutURL(s string) string {
	for i, r := range s {
		if r > 0x7e || strings.ContainsRune(")]}>\"'", r) {
			return strings.TrimRight(s[:i], ".,;:!?")
		}
	}
	return strings.TrimRight(s, ".,;:!?")
}

func yyVisionHTTPURL(u string) bool {
	lower := strings.ToLower(strings.TrimSpace(u))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// yyVisionImageURL 判断是不是「看起来是图片」的链接：路径（去掉查询串与锚点）
// 以图片后缀结尾。查询串里常带签名参数，所以只看路径。
func yyVisionImageURL(u string) bool {
	path := u
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = strings.ToLower(path)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// yyVisionDescribePrompt 给模型的看图提示：先描述看到的，再回答，别瞎猜。
func yyVisionDescribePrompt() string {
	return "用户在这条消息里发了图片，请先看清图片内容再回答；看不清就直接说明看不清，不要凭空猜测。"
}
