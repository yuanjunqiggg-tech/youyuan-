package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthRequestsUseProductIdentityAndKeepBearer(t *testing.T) {
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if !strings.HasPrefix(r.UserAgent(), "YouYuan-Toolbox/") {
			t.Error("missing client identity", r.UserAgent())
		}
		if r.URL.Path == "/api/new" {
			w.Write([]byte("test-secret"))
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("lost auth bearer")
		}
		w.Write([]byte("{}"))
	}))
	defer s.Close()
	c, e := CreateClient(&ClientOptions{AuthServer: s.URL})
	if e != nil {
		t.Fatal(e)
	}
	r, e := c.client.Get(s.URL + "/api/example")
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if n != 2 {
		t.Fatal(n)
	}
	q, _ := http.NewRequest("GET", s.URL, nil)
	q.Header.Set("User-Agent", "OwnerConfiguredClient/1")
	yyAttachDeviceKey(q)
	if q.UserAgent() != "OwnerConfiguredClient/1" {
		t.Fatal("overwrote explicit client identity")
	}
}

func TestAuthJSONFailureRetainsReason(t *testing.T) {
	r := &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{"success":false,"message":"刷新失败 (code=10): 请先登录"}`))}
	_, e := assertAndParse[map[string]any](r)
	if e == nil || !strings.Contains(e.Error(), "HTTP 400") || !strings.Contains(e.Error(), "请先登录") {
		t.Fatal(e)
	}
}
