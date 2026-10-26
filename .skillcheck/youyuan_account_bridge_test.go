package main

import (
	"encoding/json"
	lua "github.com/yuin/gopher-lua"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAccountBridgeDirectionalRules(t *testing.T) {
	old := yyAccountBridge.Rules
	defer func() { yyAccountBridge.Rules = old }()
	yyAccountBridge.Rules = []yyAccountBridgeRule{{Source: "main", Plugin: "source", Target: "second", Operations: []string{"status"}}}
	for _, c := range []yyAccountBridgeCall{{Source: "second", Plugin: "source", Target: "main", Operation: "status"}, {Source: "main", Plugin: "other", Target: "second", Operation: "status"}, {Source: "main", Plugin: "source", Target: "second", Operation: "sendCommandsFast"}} {
		if yyBridgeAllowed(c) {
			t.Fatal("unexpected cross-account grant", c)
		}
	}
	if !yyBridgeAllowed(yyAccountBridgeCall{Source: "main", Plugin: "source", Target: "second", Operation: "status"}) {
		t.Fatal("explicit rule not accepted")
	}
	if yyBridgeValidateRules([]yyAccountBridgeRule{{Source: "main", Plugin: "*", Target: "second", Operations: []string{"invoke"}}}) == nil {
		t.Fatal("arbitrary API proxy accepted")
	}
}
func TestAccountBridgeLuaRegistration(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	registerAccountBridgeAPI(L)
	v := L.GetGlobal("accounts").(*lua.LTable)
	for name := range yyAccountBridgeMethods {
		if v.RawGetString(name[len("accounts."):]).Type() != lua.LTFunction {
			t.Fatal(name)
		}
	}
}
func TestMCPLinkReadAndRevoke(t *testing.T) {
	oldItems, oldPath := yyMCPLinks.Items, yyMCPLinks.Path
	defer func() { yyMCPLinks.Items = oldItems; yyMCPLinks.Path = oldPath }()
	yyMCPLinks.Items = nil
	mux := http.NewServeMux()
	yyMCPLinksRoutes(mux, t.TempDir())
	create := httptest.NewRecorder()
	mux.ServeHTTP(create, httptest.NewRequest("GET", "/api/youyuan/mcp-links", nil))
	if create.Code != 200 {
		t.Fatal(create.Code)
	}
	yyMCPLinks.Items = []yyMCPLink{{Token: "fixture", Scope: "read", Expires: time.Now().Add(time.Hour).UnixMilli()}, {Token: "expired", Expires: 1}}
	if _, ok := yyMCPLinkLookup("expired"); ok {
		t.Fatal("expired link usable")
	}
	if _, ok := yyMCPLinkLookup("fixture"); !ok {
		t.Fatal("active link missing")
	}
	for _, p := range []string{`{"name":"api_bot_console","arguments":{"method":"POST"}}`, `{"name":"api_call","arguments":{"path":"/api/bot/console"}}`, `{"name":"api_bot_status","arguments":{"method":"DELETE"}}`} {
		if yyMCPReadCheck("tools/call", json.RawMessage(p)) == nil {
			t.Fatal("read token permitted mutation", p)
		}
	}
	if err := yyMCPReadCheck("tools/call", json.RawMessage(`{"name":"api_bot_status"}`)); err != nil {
		t.Fatal(err)
	}
	if err := yyMCPReadCheck("resources/read", json.RawMessage(`{"uri":"youyuan://account/main/project/w/config.json"}`)); err == nil {
		t.Fatal("read-only status link read project files")
	}
	yyMCPLinks.Items = nil
	if _, ok := yyMCPLinkLookup("fixture"); ok {
		t.Fatal("revoked token usable")
	}
}
