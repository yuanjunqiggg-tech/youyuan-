package main

// 皮肤缓存代理的回归测试：白名单、内网拦截、缓存命中不重复打远端、预热与接口行为。

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

type yySkinStubTransport struct {
	calls int32
	body  []byte
	code  int
	ctype string
}

func (s *yySkinStubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&s.calls, 1)
	code := s.code
	if code == 0 {
		code = http.StatusOK
	}
	ctype := s.ctype
	if ctype == "" {
		ctype = "image/png"
	}
	return &http.Response{
		StatusCode: code,
		Header:     http.Header{"Content-Type": []string{ctype}},
		Body:       io.NopCloser(bytes.NewReader(s.body)),
		Request:    req,
	}, nil
}

func yySkinTestCache(t *testing.T, transport http.RoundTripper) *yySkinCache {
	t.Helper()
	c := yyNewSkinCache(t.TempDir())
	c.client = &http.Client{Transport: transport, Timeout: yySkinTimeout}
	return c
}

// 1. 非白名单域名必须拒绝。
func TestSkinRejectsNonWhitelistedHost(t *testing.T) {
	for _, raw := range []string{
		"https://evil.example.com/avatars/x.png",
		"https://crafatar.com.evil.example.com/x.png",
		"https://sub.crafatar.com/x.png",
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("解析失败 %s: %v", raw, err)
		}
		if err := yySkinCheckURL(u); err == nil {
			t.Fatalf("应拒绝非白名单域名：%s", raw)
		}
	}
	for _, raw := range []string{
		"https://crafatar.com/avatars/069a79f4-44e9-4726-a5be-fca90e38aaf5?size=64",
		"https://minotar.net/helm/Notch/64.png",
		"https://mc-heads.net/avatar/Notch/64",
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("解析失败 %s: %v", raw, err)
		}
		if err := yySkinCheckURL(u); err != nil {
			t.Fatalf("白名单域名不应被拒：%s -> %v", raw, err)
		}
	}
}

// 2. 内网 / 回环地址必须拒绝（URL 直连与拨号两个环节）。
func TestSkinRejectsInternalAddress(t *testing.T) {
	for _, raw := range []string{
		"https://127.0.0.1/x.png",
		"https://10.0.0.8/x.png",
		"https://192.168.1.1:443/x.png",
		"https://[::1]/x.png",
		"https://169.254.169.254/latest/meta-data/",
		"https://crafatar.com@127.0.0.1/x.png",
		"http://crafatar.com/x.png",
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("解析失败 %s: %v", raw, err)
		}
		if err := yySkinCheckURL(u); err == nil {
			t.Fatalf("应拒绝内网或非法地址：%s", raw)
		}
	}
	for _, address := range []string{"127.0.0.1:443", "10.1.2.3:443", "192.168.0.5:80", "[::1]:443", "169.254.169.254:80", "100.64.0.1:80"} {
		if err := yySkinDialGuard("tcp", address, nil); err == nil {
			t.Fatalf("拨号阶段应拒绝 %s", address)
		}
	}
	if err := yySkinDialGuard("tcp", "1.1.1.1:443", nil); err != nil {
		t.Fatalf("公网地址不应被拒：%v", err)
	}
}

// 2b. 域名不能被「ParseIP 返回 nil」误杀。
//
// 这是线上头像整片不显示的根因：DialContext 拿到的是 "域名:端口"，
// 旧实现 net.ParseIP 必然 nil → 直接 "拨号地址无法解析" → 所有代理请求 400。
func TestSkinDialGuardAcceptsPublicDomain(t *testing.T) {
	err := yySkinDialGuard("tcp", "mc-heads.net:443", nil)
	if err != nil {
		if strings.Contains(err.Error(), "解析") {
			t.Skipf("本机 DNS 不可用，跳过：%v", err)
		}
		t.Fatalf("公网域名不应被拒：%v", err)
	}
}

// 2c. 网易官方图源必须在白名单里（含子域），否则前端把 URL 包成
// /api/skin/proxy?url= 之后全部被拒，账号页头像一张都出不来。
func TestSkinHostAllowedNetease(t *testing.T) {
	allow := []string{
		"x19.gph.netease.com",
		"g79.gph.netease.com",
		"netease.com",
		"some.163.com",
		"p1.music.126.net",
		"crafatar.com",
		"mc-heads.net",
	}
	for _, h := range allow {
		if !yySkinHostAllowed(h) {
			t.Fatalf("应放行 %s", h)
		}
	}
	deny := []string{
		"evilnetease.com",     // 后缀伪装
		"netEase.com.attacker", // 前缀伪装
		"example.com",
		"127.0.0.1",
		"",
	}
	for _, h := range deny {
		if yySkinHostAllowed(h) {
			t.Fatalf("不该放行 %s", h)
		}
	}
}

// 3. 缓存命中后第二次不再请求远端。
func TestSkinCacheHitSkipsUpstream(t *testing.T) {
	stub := &yySkinStubTransport{body: []byte("fake-png-bytes")}
	c := yySkinTestCache(t, stub)
	u, _ := url.Parse("https://crafatar.com/avatars/069a79f4-44e9-4726-a5be-fca90e38aaf5?size=64&overlay")

	first, ctype, etag, err := c.load(u)
	if err != nil {
		t.Fatalf("首次加载失败：%v", err)
	}
	if string(first) != "fake-png-bytes" || ctype != "image/png" || etag == "" {
		t.Fatalf("首次返回异常：%q %q %q", first, ctype, etag)
	}
	second, _, etag2, err := c.load(u)
	if err != nil {
		t.Fatalf("二次加载失败：%v", err)
	}
	if !bytes.Equal(first, second) || etag != etag2 {
		t.Fatalf("缓存内容不一致")
	}
	if got := atomic.LoadInt32(&stub.calls); got != 1 {
		t.Fatalf("远端应只被请求 1 次，实际 %d 次", got)
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("缓存目录应有 1 个文件，实际 %v %v", entries, err)
	}
	if !strings.HasSuffix(entries[0].Name(), ".png") {
		t.Fatalf("缓存文件名应带扩展名，实际 %s", entries[0].Name())
	}
}

// 4. 上游失败时不写缓存，并返回明确错误（前端据此换默认头像）。
func TestSkinUpstreamFailureIsNotCached(t *testing.T) {
	stub := &yySkinStubTransport{body: []byte("oops"), code: http.StatusInternalServerError}
	c := yySkinTestCache(t, stub)
	u, _ := url.Parse("https://crafatar.com/avatars/069a79f4-44e9-4726-a5be-fca90e38aaf5")
	if _, _, _, err := c.load(u); err == nil {
		t.Fatal("上游 500 时不应成功")
	}
	if names, _ := os.ReadDir(c.dir); len(names) != 0 {
		t.Fatalf("失败结果不应落盘，实际 %v", names)
	}
	// 负缓存生效：第二次直接失败，不再打远端。
	if _, _, _, err := c.load(u); err == nil {
		t.Fatal("负缓存期内仍应失败")
	}
	if got := atomic.LoadInt32(&stub.calls); got != 1 {
		t.Fatalf("负缓存应避免重复请求，实际请求 %d 次", got)
	}
}

// 5. /api/skin/proxy 端到端：状态码、缓存头、ETag 与 304。
func TestSkinProxyHandlerServesCachedImage(t *testing.T) {
	stub := &yySkinStubTransport{body: []byte("pngdata")}
	c := yySkinTestCache(t, stub)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/skin/proxy", c.handleProxy)
	target := "https://minotar.net/helm/Notch/64.png"

	req := httptest.NewRequest(http.MethodGet, "/api/skin/proxy?url="+url.QueryEscape(target), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d：%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=604800") {
		t.Fatalf("缓存头缺失：%q", got)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("缺少 ETag")
	}
	if rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("Content-Type 异常：%q", rec.Header().Get("Content-Type"))
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/skin/proxy?url="+url.QueryEscape(target), nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("期望 304，实际 %d", rec2.Code)
	}
	if got := atomic.LoadInt32(&stub.calls); got != 1 {
		t.Fatalf("远端应只请求 1 次，实际 %d", got)
	}
}

// 6. 非白名单地址经接口访问必须 400。
func TestSkinProxyHandlerRejectsBadURL(t *testing.T) {
	c := yySkinTestCache(t, &yySkinStubTransport{body: []byte("x")})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/skin/proxy", c.handleProxy)
	for _, raw := range []string{"https://evil.example.com/a.png", "https://127.0.0.1/a.png", ""} {
		req := httptest.NewRequest(http.MethodGet, "/api/skin/proxy?url="+url.QueryEscape(raw), nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%q 应返回 400，实际 %d", raw, rec.Code)
		}
	}
}

// 7. 预热接口按 names / uuids 批量取图并计数。
func TestSkinWarmHandlerFetchesAll(t *testing.T) {
	stub := &yySkinStubTransport{body: []byte("warm")}
	c := yySkinTestCache(t, stub)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/skin/warm", c.handleWarm)
	req := httptest.NewRequest(http.MethodGet, "/api/skin/warm?names=Notch,Herobrine&uuids=069a79f4-44e9-4726-a5be-fca90e38aaf5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"warmed":3`) {
		t.Fatalf("期望预热 3 张，实际 %s", body)
	}
	if got := atomic.LoadInt32(&stub.calls); got != 3 {
		t.Fatalf("远端应被请求 3 次，实际 %d", got)
	}
}
