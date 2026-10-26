package main

import (
	"context"
	"net"
	"strings"
	"testing"
)

// 联网能力对外开放，这几条守住「不能打内网」这条线。
func TestWebCheckURLRejectsInternal(t *testing.T) {
	bad := []string{
		"http://127.0.0.1/",
		"http://localhost:19429/api/qq/config",
		"http://10.0.0.5/",
		"http://192.168.1.1/",
		"http://169.254.169.254/latest/meta-data/", // 云厂商元数据
		"http://[::1]/",
		"ftp://example.com/",
		"file:///etc/passwd",
		"http://user:pass@example.com/",
	}
	for _, u := range bad {
		if _, e := yyWebCheckURL(u); e == nil {
			t.Fatalf("应当拒绝：%s", u)
		}
	}
}

func TestWebIPAllowed(t *testing.T) {
	allow := []string{"1.1.1.1", "8.8.8.8", "223.5.5.5"}
	deny := []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.0.1", "169.254.1.1", "0.0.0.0"}
	for _, s := range allow {
		if !yyWebIPAllowed(parseIPForTest(s)) {
			t.Fatalf("公网地址应放行：%s", s)
		}
	}
	for _, s := range deny {
		if yyWebIPAllowed(parseIPForTest(s)) {
			t.Fatalf("内网地址应拒绝：%s", s)
		}
	}
}

func TestWebStripRemovesScriptAndTags(t *testing.T) {
	html := `<html><head><style>.a{color:red}</style><script>var x=1;</script></head>` +
		`<body><h1>标题</h1><p>正文&nbsp;内容</p></body></html>`
	out := yyWebStrip(html)
	if strings.Contains(out, "var x") || strings.Contains(out, "color:red") {
		t.Fatalf("脚本/样式没清掉：%s", out)
	}
	if !strings.Contains(out, "标题") || !strings.Contains(out, "正文") {
		t.Fatalf("正文丢了：%s", out)
	}
	if strings.Contains(out, "<") || strings.Contains(out, ">") {
		t.Fatalf("标签没清干净：%s", out)
	}
}

// 内网目标在真正发请求前就应该被拒（不依赖网络）。
func TestWebFetchRejectsBeforeNetwork(t *testing.T) {
	if _, e := yyWebFetch(context.Background(), "http://127.0.0.1:19429/api/qq/config", 100); e == nil {
		t.Fatal("内网地址必须拒绝")
	}
}

func parseIPForTest(s string) net.IP { return net.ParseIP(s) }
