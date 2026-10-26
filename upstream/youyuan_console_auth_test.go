package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestYYConsoleAuth 锁定 DSH-04 的四条边界：
// 无 token → 401；控制台 token → 放行；远控 key → 放行；导出能力 URL → 放行。
func TestYYConsoleAuth(t *testing.T) {
	yyRemoteMu.Lock()
	oldKey := yyRemote.Key
	yyRemote.Key = "remote-key-for-test"
	yyRemoteMu.Unlock()
	defer func() {
		yyRemoteMu.Lock()
		yyRemote.Key = oldKey
		yyRemoteMu.Unlock()
	}()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := yyConsoleAuth(next)

	cases := []struct {
		name   string
		path   string
		method string
		header map[string]string
		query  string
		want   int
	}{
		{name: "无 token 拒绝", path: "/api/config", method: "GET", want: 401},
		{name: "控制台 header 放行", path: "/api/config", method: "GET",
			header: map[string]string{"X-YouYuan-Console-Token": yyConsoleTokenValue()}, want: 200},
		{name: "控制台 bearer 放行", path: "/api/config", method: "GET",
			header: map[string]string{"Authorization": "Bearer " + yyConsoleTokenValue()}, want: 200},
		{name: "远控 key 放行", path: "/api/config", method: "GET",
			header: map[string]string{"Authorization": "Bearer remote-key-for-test"}, want: 200},
		{name: "query token 放行", path: "/api/youyuan/events", method: "GET",
			query: "?token=" + yyConsoleTokenValue(), want: 200},
		{name: "导出能力 URL 放行", path: "/api/youyuan/exports/download", method: "GET", want: 200},
		{name: "头像图片 GET 放行", path: "/api/skin/online-head", method: "GET", want: 200},
		{name: "市场预览图 GET 放行", path: "/api/market-proxy/api/market/files/1/gif", method: "GET", want: 200},
		{name: "静态资源不拦", path: "/js/api.js", method: "GET", want: 200},
		{name: "OPTIONS 交给 CORS 层", path: "/api/config", method: "OPTIONS", want: 200},
		{name: "错 token 拒绝", path: "/api/config", method: "GET",
			header: map[string]string{"X-YouYuan-Console-Token": "wrong"}, want: 401},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path+tc.query, nil)
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s: got %d want %d body=%s", tc.name, rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

// TestYYConsoleInjectToken 必须幂等：重复注入只留一个 meta。
func TestYYConsoleInjectToken(t *testing.T) {
	in := []byte("<html><head><title>x</title></head><body></body></html>")
	out := yyConsoleInjectToken(in)
	if got := countSubstring(string(out), `name="yy-console-token"`); got != 1 {
		t.Fatalf("meta count = %d, want 1", got)
	}
	out2 := yyConsoleInjectToken(out)
	if got := countSubstring(string(out2), `name="yy-console-token"`); got != 1 {
		t.Fatalf("second inject meta count = %d, want 1", got)
	}
}

func countSubstring(s, sub string) int {
	n := 0
	for {
		i := indexOf(s, sub)
		if i < 0 {
			return n
		}
		n++
		s = s[i+len(sub):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
