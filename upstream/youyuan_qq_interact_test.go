package main

import (
	"fmt"
	"strings"
	"testing"
)

// ---- 识图 ----

func TestYYVisionImagePart(t *testing.T) {
	part, e := yyVisionImagePart("https://img.example.com/a.jpg")
	if e != nil {
		t.Fatalf("公网图片应通过：%v", e)
	}
	if part["type"] != "image_url" {
		t.Fatalf("段类型应为 image_url，得到 %v", part["type"])
	}
	inner, ok := part["image_url"].(map[string]any)
	if !ok || inner["url"] != "https://img.example.com/a.jpg" {
		t.Fatalf("image_url.url 结构不对：%+v", part)
	}

	bad := []string{
		"",                                   // 空
		"data:image/png;base64,iVBORw0KGgo",  // 内联 base64
		"file:///etc/passwd",                 // 非 http
		"ftp://example.com/a.jpg",            // 非 http
		"http://127.0.0.1/a.jpg",             // 回环
		"http://localhost:8080/a.jpg",        // localhost
		"http://192.168.1.10/a.jpg",          // 私网
		"http://10.0.0.8/a.jpg",              // 私网
		"http://172.20.1.2/a.jpg",            // 私网
		"http://169.254.169.254/latest/meta", // 云元数据
		"http://[::1]/a.jpg",                 // IPv6 回环
		"http://0.0.0.0/a.jpg",               // 未指定
		"https://foo.internal/a.jpg",         // 内网后缀
		"https:///a.jpg",                     // 缺主机
	}
	for _, u := range bad {
		if _, e := yyVisionImagePart(u); e == nil {
			t.Fatalf("%q 应被拒绝", u)
		}
	}
}

func TestYYVisionExtractURLs(t *testing.T) {
	// 来源一：CQ 码图片段，url 参数是直链。
	got := yyVisionExtractURLs("看这个[CQ:image,file=abc.jpg,url=https://cdn.example.com/1.png]好看吗")
	if len(got) != 1 || got[0] != "https://cdn.example.com/1.png" {
		t.Fatalf("CQ 码里应抽出 1 个图片地址，得到 %v", got)
	}
	// file 直接是直链的适配器也要认。
	got = yyVisionExtractURLs("[CQ:image,file=https://cdn.example.com/2.gif]")
	if len(got) != 1 || got[0] != "https://cdn.example.com/2.gif" {
		t.Fatalf("file 直链应被识别，得到 %v", got)
	}
	// 来源二：裸链接，且忽略非图片链接与中文标点。
	got = yyVisionExtractURLs("这两张 https://a.example.com/x.jpg，还有 https://b.example.com/y.webp。另有 https://c.example.com/page.html")
	if len(got) != 2 {
		t.Fatalf("应抽出 2 个图片裸链接，得到 %v", got)
	}
	if got[0] != "https://a.example.com/x.jpg" || got[1] != "https://b.example.com/y.webp" {
		t.Fatalf("裸链接抽取结果不对：%v", got)
	}
	// 去重
	if got := yyVisionExtractURLs("https://a.example.com/x.jpg https://a.example.com/x.jpg"); len(got) != 1 {
		t.Fatalf("应去重，得到 %v", got)
	}
	if got := yyVisionExtractURLs("没有图片"); len(got) != 0 {
		t.Fatalf("纯文本不应抽出地址，得到 %v", got)
	}
	if yyVisionDescribePrompt() == "" {
		t.Fatal("识图提示词不应为空")
	}
}

// ---- 拍一拍 ----

func TestYYPokeGroupFallback(t *testing.T) {
	old := yyPokeCall
	defer func() { yyPokeCall = old }()

	var calls []string
	yyPokeCall = func(action string, params map[string]any) (any, error) {
		calls = append(calls, action)
		if action == "group_poke" {
			return nil, fmt.Errorf("OneBot 返回失败：{\"status\":\"failed\",\"retcode\":1404,\"message\":\"不支持的API\"}")
		}
		if params["group_id"] != "123456" || params["user_id"] != "10001" {
			t.Fatalf("参数不对：%+v", params)
		}
		return nil, nil
	}

	if e := yyPokeGroup("123456", "10001"); e != nil {
		t.Fatalf("回退后应成功：%v", e)
	}
	if len(calls) != 2 || calls[0] != "group_poke" || calls[1] != "send_group_poke" {
		t.Fatalf("应先 group_poke 再回退 send_group_poke，实际 %v", calls)
	}

	// 非「动作不存在」的错误不重试。
	calls = nil
	yyPokeCall = func(action string, params map[string]any) (any, error) {
		calls = append(calls, action)
		return nil, fmt.Errorf("OneBot 回执超时，结果未知")
	}
	if e := yyPokeGroup("123456", "10001"); e == nil {
		t.Fatal("超时应报错")
	}
	if len(calls) != 1 {
		t.Fatalf("超时不应换动作名重试，实际 %v", calls)
	}

	// 第一次就成功时不重试。
	calls = nil
	yyPokeCall = func(action string, params map[string]any) (any, error) {
		calls = append(calls, action)
		return nil, nil
	}
	if e := yyPokeGroup("123456", "10001"); e != nil {
		t.Fatalf("直接成功不应报错：%v", e)
	}
	if len(calls) != 1 || calls[0] != "group_poke" {
		t.Fatalf("不应回退，实际 %v", calls)
	}
}

func TestYYPokeFriendFallbackAndTarget(t *testing.T) {
	old := yyPokeCall
	defer func() { yyPokeCall = old }()

	var calls []string
	yyPokeCall = func(action string, params map[string]any) (any, error) {
		calls = append(calls, action)
		if action == "friend_poke" {
			return nil, fmt.Errorf("OneBot 返回失败：unknown action friend_poke")
		}
		return nil, nil
	}
	if e := yyPokeFriend("10001"); e != nil {
		t.Fatalf("私聊拍一拍回退后应成功：%v", e)
	}
	if len(calls) != 2 || calls[1] != "send_friend_poke" {
		t.Fatalf("应回退 send_friend_poke，实际 %v", calls)
	}
	if e := yyPokeFriend("  "); e == nil {
		t.Fatal("缺 QQ 号应报错")
	}

	if target, ok := yyPokeTargetArg("", "10001"); !ok || target != "10001" {
		t.Fatalf("空参数应拍自己，得到 %q %v", target, ok)
	}
	if target, ok := yyPokeTargetArg("@10002", "10001"); !ok || target != "10002" {
		t.Fatalf("@QQ号 解析失败，得到 %q %v", target, ok)
	}
	if target, ok := yyPokeTargetArg("[CQ:at,qq=10003]", "10001"); !ok || target != "10003" {
		t.Fatalf("CQ at 解析失败，得到 %q %v", target, ok)
	}
	if _, ok := yyPokeTargetArg("@张三", "10001"); ok {
		t.Fatal("无法识别的目标应返回 ok=false")
	}
}

// ---- 发表情包 ----

func TestYYQQCmdStickerIndexOutOfRange(t *testing.T) {
	old := yyStickerFaces
	defer func() { yyStickerFaces = old }()
	yyStickerFaces = func() ([]map[string]any, error) {
		return []map[string]any{
			{"url": "https://cdn.example.com/a.png"},
			{"url": "https://cdn.example.com/b.png"},
		}, nil
	}

	out := yyQQCmdSticker(yyQQConfig{}, "10001", "测试", "5", 2)
	if len(out) != 1 {
		t.Fatalf("应回一句提示，得到 %v", out)
	}
	msg := out[0]
	if !strings.Contains(msg, "没有第 5 个") || !strings.Contains(msg, "2 个") {
		t.Fatalf("越界提示要说清序号与总数，得到 %q", msg)
	}

	if out := yyQQCmdSticker(yyQQConfig{}, "10001", "测试", "0", 2); !strings.Contains(out[0], "从 1 开始") {
		t.Fatalf("0 应提示从 1 开始，得到 %v", out)
	}
	if out := yyQQCmdSticker(yyQQConfig{}, "10001", "测试", "", 2); !strings.Contains(out[0], "格式") {
		t.Fatalf("空参数应给格式提示，得到 %v", out)
	}
	if out := yyQQCmdSticker(yyQQConfig{}, "10001", "测试", "ftp://x/a.png", 2); !strings.Contains(out[0], "http") {
		t.Fatalf("非 http 链接应被拒，得到 %v", out)
	}
	long := "https://cdn.example.com/" + strings.Repeat("a", 2100)
	if out := yyQQCmdSticker(yyQQConfig{}, "10001", "测试", long, 2); !strings.Contains(out[0], "太长") {
		t.Fatalf("超长链接应被拒，得到 %v", out)
	}
}

func TestYYStickerSendBuildsImageSegment(t *testing.T) {
	old := yyStickerCall
	defer func() { yyStickerCall = old }()

	var action string
	var params map[string]any
	yyStickerCall = func(a string, p map[string]any) (any, error) {
		action, params = a, p
		return nil, nil
	}
	if e := yyStickerSend("123456", "https://cdn.example.com/a.png"); e != nil {
		t.Fatalf("发图应成功：%v", e)
	}
	if action != "send_group_msg" {
		t.Fatalf("应走 send_group_msg，实际 %s", action)
	}
	if params["group_id"] != int64(123456) {
		t.Fatalf("群号应为 int64，实际 %T %v", params["group_id"], params["group_id"])
	}
	msg, ok := params["message"].([]map[string]any)
	if !ok || len(msg) != 1 {
		t.Fatalf("消息应是单段数组，实际 %#v", params["message"])
	}
	if msg[0]["type"] != "image" {
		t.Fatalf("段类型应为 image，实际 %v", msg[0]["type"])
	}
	data, ok := msg[0]["data"].(map[string]any)
	if !ok || data["file"] != "https://cdn.example.com/a.png" {
		t.Fatalf("图片地址应放在 data.file，实际 %#v", msg[0]["data"])
	}

	if e := yyStickerSend("", "https://cdn.example.com/a.png"); e == nil {
		t.Fatal("群号无效应报错")
	}
	if e := yyStickerSend("123456", "  "); e == nil {
		t.Fatal("空图片地址应报错")
	}
}
