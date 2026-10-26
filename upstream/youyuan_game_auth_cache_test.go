package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 皮肤预览缓存的行为约束。守的是「点开预览要等 5 秒」这件事：
// 同一个账号第二次必须直接命中，换皮肤后必须重新回源。

// yyAuthTestServer 起一个假认证服务，统计被真实调用的次数。
func yyAuthTestServer(t *testing.T, respBody string, status int) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func yyAuthCall(t *testing.T, origin, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	yyGameAuthServe(context.Background(), origin, "test-key", http.MethodPost, []byte(body), rec)
	return rec
}

func TestYYGameAuthSkinPreviewCacheHit(t *testing.T) {
	yyGameAuthPreviewReset()
	srv, hits := yyAuthTestServer(t, `{"ok":true,"texture":"data:image/png;base64,AAAA","width":128,"height":128}`, 200)

	const req = `{"action":"skin_preview","id":"acc-1"}`

	first := yyAuthCall(t, srv.URL, req)
	if first.Code != 200 || !strings.Contains(first.Body.String(), `"texture"`) {
		t.Fatalf("首次请求异常：%d %s", first.Code, first.Body.String())
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("首次应回源 1 次，实际 %d", got)
	}

	second := yyAuthCall(t, srv.URL, req)
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("第二次应命中缓存不再回源，实际回源 %d 次", got)
	}
	if second.Body.String() != first.Body.String() {
		t.Fatalf("缓存响应与首次不一致：\n首次 %s\n缓存 %s", first.Body.String(), second.Body.String())
	}
}

func TestYYGameAuthSkinCacheDroppedAfterSkinChange(t *testing.T) {
	yyGameAuthPreviewReset()
	srv, hits := yyAuthTestServer(t, `{"ok":true,"texture":"data:image/png;base64,AAAA"}`, 200)

	const preview = `{"action":"skin_preview","id":"acc-2"}`
	yyAuthCall(t, srv.URL, preview) // 回源 1
	yyAuthCall(t, srv.URL, preview) // 命中，仍 1
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("缓存未生效，回源 %d 次", got)
	}

	// 换皮肤成功 → 缓存必须失效
	yyAuthCall(t, srv.URL, `{"action":"skin","id":"acc-2","item_id":"4676202654076007710"}`) // 回源 2
	yyAuthCall(t, srv.URL, preview)                                                         // 回源 3
	if got := atomic.LoadInt32(hits); got != 3 {
		t.Fatalf("换皮肤后应重新回源，期望 3 次，实际 %d", got)
	}
}

func TestYYGameAuthNonPreviewNeverCached(t *testing.T) {
	yyGameAuthPreviewReset()
	srv, hits := yyAuthTestServer(t, `{"ok":true,"accounts":[]}`, 200)

	// 账号列表不是 skin_preview，两次都必须回源（列表随时在变，不能缓存）
	yyAuthCall(t, srv.URL, `{"action":"list"}`)
	yyAuthCall(t, srv.URL, `{"action":"list"}`)
	if got := atomic.LoadInt32(hits); got != 2 {
		t.Fatalf("非 skin_preview 不应缓存，回源 %d 次（期望 2）", got)
	}
}

func TestYYGameAuthFailedPreviewNotCached(t *testing.T) {
	yyGameAuthPreviewReset()
	srv, hits := yyAuthTestServer(t, `{"ok":false,"error":"这个账号还没选过皮肤"}`, 200)

	const req = `{"action":"skin_preview","id":"acc-3"}`
	yyAuthCall(t, srv.URL, req)
	yyAuthCall(t, srv.URL, req)
	if got := atomic.LoadInt32(hits); got != 2 {
		t.Fatalf("失败响应不能进缓存，回源 %d 次（期望 2）", got)
	}
}

func TestYYGameAuthCacheExpires(t *testing.T) {
	yyGameAuthPreviewReset()
	// 直接验证 TTL 逻辑，不去等 5 分钟：手工塞一条过期条目。
	yyGameAuthPreviewCache.Lock()
	yyGameAuthPreviewCache.m["acc-4"] = yyGameAuthPreviewEntry{
		body: []byte(`{"ok":true}`),
		at:   time.Now().Add(-yyGameAuthPreviewTTL - time.Second),
	}
	yyGameAuthPreviewCache.Unlock()

	if _, ok := yyGameAuthPreviewGet("acc-4"); ok {
		t.Fatalf("过期条目不应命中")
	}
	yyGameAuthPreviewCache.Lock()
	_, still := yyGameAuthPreviewCache.m["acc-4"]
	yyGameAuthPreviewCache.Unlock()
	if still {
		t.Fatalf("过期条目应被顺手删掉，避免内存里堆垃圾")
	}
}
