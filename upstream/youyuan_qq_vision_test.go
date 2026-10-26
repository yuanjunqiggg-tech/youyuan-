package main

import (
	"os"
	"path/filepath"
	"testing"
)

// 用户实际踩的坑：模型叫 "Deep seek"，名字里没有 vision/vl/4o 等标记，
// 旧逻辑（裸看模型名）判为「不支持视觉」→ 群里的图片/表情包 AI 全都看不到。
// 三态开关就是为这种「实际支持但名看不出来」的模型开的强制档。
func TestVisionThreeState(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "vision_three_state_dir")
	os.MkdirAll(dir, 0o755)
	aiStore.setPath(dir)
	defer aiStore.setPath(filepath.Join(os.TempDir(), "vision_three_state_dir"))
	if e := aiStore.Add(&AIModel{Name: "vv-plain", Provider: "openai", BaseURL: "http://x", ModelID: "deepseek-chat"}); e != nil {
		t.Fatal(e)
	}
	if e := aiStore.Add(&AIModel{Name: "vv-vl", Provider: "openai", BaseURL: "http://x", ModelID: "qwen-vl-max"}); e != nil {
		t.Fatal(e)
	}
	cf := yyQQConfig{}
	cf.Assistant.Model = "vv-plain"

	cf.Assistant.Vision = nil
	if yyQQVisionEnabled(cf) {
		t.Fatal("auto + 非视觉模型名应为 false")
	}
	if got := yyQQVisionMode(cf); got != "auto" {
		t.Fatalf("mode 应为 auto，got %s", got)
	}
	on := true
	cf.Assistant.Vision = &on
	if !yyQQVisionEnabled(cf) {
		t.Fatal("forced_on 必须开启，即使模型名没有视觉标记")
	}
	if got := yyQQVisionMode(cf); got != "forced_on" {
		t.Fatalf("mode 应为 forced_on，got %s", got)
	}
	off := false
	cf.Assistant.Vision = &off
	if yyQQVisionEnabled(cf) {
		t.Fatal("forced_off 必须关闭")
	}
	if got := yyQQVisionMode(cf); got != "forced_off" {
		t.Fatalf("mode 应为 forced_off，got %s", got)
	}
	// 视觉名模型在 auto 下仍然要开（旧行为不能退化）。
	cf.Assistant.Model = "vv-vl"
	cf.Assistant.Vision = nil
	if !yyQQVisionEnabled(cf) {
		t.Fatal("auto + 视觉模型名应为 true")
	}
}

// json.Marshal 会把 URL 里的 and 符号转成反杠u0026 六位字面量，视觉请求拿到
// 这种链接直接失败——「看不到图」的隐蔽根因。Unescape 必须还原。
func TestVisionUnescapeRestoresSig(t *testing.T) {
	raw := "https://multimedia.nt.qq.com/p.jpg?sig=abc\\u0026exp=123"
	want := "https://multimedia.nt.qq.com/p.jpg?sig=abc&exp=123"
	if got := yyVisionUnescapeJSON(raw); got != want {
		t.Fatalf("未还原 and 符号：%q", got)
	}
	if got := yyVisionUnescapeJSON("https://a.com/b.jpg"); got != "https://a.com/b.jpg" {
		t.Fatalf("无转义串不应被改动：%q", got)
	}
}

func TestVisionExtractCQImageURL(t *testing.T) {
	msg := "[CQ:image,file=abc.jpg,url=https://multimedia.nt.qq.com/p.jpg?sig=x&y=1]看这个"
	urls := yyVisionExtractURLs(msg)
	if len(urls) != 1 || urls[0] != "https://multimedia.nt.qq.com/p.jpg?sig=x&y=1" {
		t.Fatalf("CQ 段 url 抽取错误：%v", urls)
	}
	// 裸链接（后面粘中文）也要能抽到。
	if got := yyVisionExtractURLs("看这个https://a.com/b.jpg，好看吗"); len(got) != 1 || got[0] != "https://a.com/b.jpg" {
		t.Fatalf("裸链接抽取错误：%v", got)
	}
	// 没有图片的文本不能凭空产出链接。
	if got := yyVisionExtractURLs("今天天气不错"); len(got) != 0 {
		t.Fatalf("纯文本不应有图：%v", got)
	}
}

// 只有 file 没有 url 的段（表情包/市场表情常态）：无 OneBot 连接时
// 必须静默返回空，不能 panic、不能把请求打挂。
func TestVisionCollectFileOnlyNoConn(t *testing.T) {
	urls := yyVisionCollectURLs(yyQQConfig{}, "[CQ:image,file={ABC-123}.jpg]")
	if len(urls) != 0 {
		t.Fatalf("无连接时应降级为空，got %v", urls)
	}
	// 有直链的段不受影响。
	urls = yyVisionCollectURLs(yyQQConfig{}, "[CQ:image,url=https://a.com/b.jpg]")
	if len(urls) != 1 {
		t.Fatalf("直链应保留，got %v", urls)
	}
	// 多图上限 3。
	urls = yyVisionCollectURLs(yyQQConfig{}, "[CQ:image,url=https://a.com/1.jpg][CQ:image,url=https://a.com/2.jpg][CQ:image,url=https://a.com/3.jpg][CQ:image,url=https://a.com/4.jpg]")
	if len(urls) != 3 {
		t.Fatalf("多图应截到 3，got %d", len(urls))
	}
}

func TestVisionImagePartRejectsInternal(t *testing.T) {
	for _, u := range []string{"data:image/png;base64,xxxx", "http://127.0.0.1/a.jpg", "http://localhost/a.jpg", "http://192.168.1.2/a.jpg"} {
		if _, e := yyVisionImagePart(u); e == nil {
			t.Fatalf("应拒绝 %s", u)
		}
	}
	if _, e := yyVisionImagePart("https://multimedia.nt.qq.com/p.jpg"); e != nil {
		t.Fatalf("公网 https 图应放行：%v", e)
	}
}
