package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 验证 http.* 详细网络 API：GET/POST 返回 {status, body, headers}，非 200 也拿 body。
func TestHTTPAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/data":
			w.Header().Set("X-Custom", "hello")
			fmt.Fprint(w, `{"k":"v"}`)
		case "/echo":
			body := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(body)
			w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
			fmt.Fprintf(w, "method=%s body=%s", r.Method, string(body))
		case "/notfound":
			http.Error(w, "boom", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	bm := &BotManager{}
	L := newLuaState(bm)
	defer L.Close()

	var got []string
	old := sendTargetedHook
	setSendTargetedHook(func(p, txt string) error { got = append(got, txt); return nil })
	defer func() { setSendTargetedHook(old) }()

	// GET → 状态码/body/响应头
	if err := L.DoString(`local r = http.get("` + srv.URL + `/data"); game.sendTargeted("a", r.status .. "|" .. r.body .. "|" .. r.headers["X-Custom"])`); err != nil {
		t.Fatalf("http.get: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] != `200|{"k":"v"}|hello` {
		t.Fatalf("http.get 结果错误: %v", got)
	}
	// POST → 方法+body 回显 + 默认 Content-Type
	got = nil
	if err := L.DoString(`local r = http.post("` + srv.URL + `/echo", "abc"); game.sendTargeted("a", r.body .. "|ct=" .. r.headers["Content-Type"])`); err != nil {
		t.Fatalf("http.post: %v", err)
	}
	if len(got) == 0 || !strings.Contains(got[len(got)-1], "method=POST body=abc") || !strings.Contains(got[len(got)-1], "ct=application/json") {
		t.Fatalf("http.post 结果错误: %v", got)
	}
	// 非 2xx → ok=true 但 status=404、body=boom
	got = nil
	if err := L.DoString(`local r = http.get("` + srv.URL + `/notfound"); game.sendTargeted("a", tostring(r.ok) .. "|" .. r.status .. "|" .. r.body)`); err != nil {
		t.Fatalf("http.get notfound: %v", err)
	}
	if len(got) == 0 || !strings.Contains(got[len(got)-1], "true|404|") {
		t.Fatalf("非200结果错误: %v", got)
	}
	// querify
	got = nil
	if err := L.DoString(`local q = http.querify({a="1", b="hello world"}); game.sendTargeted("a", q)`); err != nil {
		t.Fatalf("querify: %v", err)
	}
	if len(got) == 0 || !strings.Contains(got[len(got)-1], "a=1") || !strings.Contains(got[len(got)-1], "hello+world") {
		t.Fatalf("querify 结果错误: %v", got)
	}
	// http 表绑定齐全
	for _, fn := range []string{"request", "get", "post", "put", "delete", "json_encode", "json_decode", "querify"} {
		if L.GetField(L.GetGlobal("http"), fn).Type() == 0 {
			t.Fatalf("http.%s 未绑定", fn)
		}
	}
}
