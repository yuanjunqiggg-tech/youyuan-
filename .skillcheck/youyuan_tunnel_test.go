package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTunnelExposesMCPAndBlocksPrivateWebAPI(t *testing.T) {
	key := yyRemote.Key
	yyRemote.Key = "test-tunnel-secret"
	defer func() { yyRemote.Key = key }()
	for _, path := range []string{"/api/config", "/api/file/list", "/api/qq/config"} {
		w := httptest.NewRecorder()
		yyTunnelPublicHandler(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("private API exposed", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	yyTunnelPublicHandler(w, httptest.NewRequest("POST", "/mcp/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)))
	if w.Code != 401 {
		t.Fatal("missing MCP key accepted", w.Code)
	}
	if yyTunnelURL.FindString("Address https://abc-def.trycloudflare.com is ready") != "https://abc-def.trycloudflare.com" {
		t.Fatal("address parsing")
	}
}
