package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPProtocolCompatibilityAndRealInvocation(t *testing.T) {
	oldRemote, oldMux := yyRemote, yyMux
	defer func() { yyRemote = oldRemote; yyMux = oldMux }()
	yyRemote.Key = "mcp-fixture-secret"
	yyMux = http.NewServeMux()
	calls := 0
	yyMux.HandleFunc("/api/bot/status", func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w, map[string]any{"ok": true, "actual_handler": true})
	})
	invoke := func(method, body string, authorized bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/mcp/", strings.NewReader(body))
		if authorized {
			r.Header.Set("Authorization", "Bearer "+yyRemote.Key)
		}
		w := httptest.NewRecorder()
		yyMCP(w, r)
		return w
	}
	for _, v := range []string{"2025-03-26", "2025-06-18", "unknown-future"} {
		t.Run("negotiate_"+v, func(t *testing.T) {
			w := invoke("POST", `{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"`+v+`"}}`, true)
			var p map[string]any
			if e := json.Unmarshal(w.Body.Bytes(), &p); e != nil {
				t.Fatal(e)
			}
			actual := p["result"].(map[string]any)["protocolVersion"]
			expected := v
			if !yyMCPVersionSupported(v) {
				expected = "2025-06-18"
			}
			if actual != expected {
				t.Fatal(actual)
			}
		})
	}
	cases := []struct {
		name, body string
		code       float64
	}{
		{"parse", `{`, -32700}, {"trailing", `{"jsonrpc":"2.0","id":1,"method":"ping"}{}`, -32700},
		{"batch", `[]`, -32600}, {"version", `{"id":1,"method":"ping"}`, -32600},
		{"null_id", `{"jsonrpc":"2.0","id":null,"method":"ping"}`, -32600},
		{"object_id", `{"jsonrpc":"2.0","id":{},"method":"ping"}`, -32600},
		{"mixed_response", `{"jsonrpc":"2.0","id":1,"method":"ping","result":{}}`, -32600},
		{"params", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":[]}`, -32602},
		{"unknown_method", `{"jsonrpc":"2.0","id":1,"method":"not/a/method"}`, -32601},
		{"unknown_tool", `{"jsonrpc":"2.0","id":"tool","method":"tools/call","params":{"name":"not_a_tool"}}`, -32602},
		{"arguments", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"api_bot_status","arguments":[]}}`, -32602},
		{"argument_type", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"api_bot_status","arguments":{"account":1}}}`, -32602},
		{"query_type", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"api_bot_status","arguments":{"query":[]}}}`, -32602},
		{"account", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"api_bot_status","arguments":{"account":"../other"}}}`, -32602},
		{"verb", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"api_bot_status","arguments":{"method":"TRACE"}}}`, -32602},
		{"resource", `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{}}`, -32602},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := invoke("POST", c.body, true)
			var p map[string]any
			if e := json.Unmarshal(w.Body.Bytes(), &p); e != nil {
				t.Fatalf("non-protocol response: %s", w.Body)
			}
			err, ok := p["error"].(map[string]any)
			if p["jsonrpc"] != "2.0" || !ok || err["code"] != c.code {
				t.Fatal(p)
			}
		})
	}
	if calls != 0 {
		t.Fatal("invalid request executed tool", calls)
	}
	for _, b := range []string{`{"jsonrpc":"2.0","method":"notifications/initialized"}`, `{"jsonrpc":"2.0","id":1,"result":{}}`} {
		w := invoke("POST", b, true)
		if w.Code != 202 || w.Body.Len() != 0 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if invoke("PUT", `{}`, true).Code != 405 || invoke("POST", `{"jsonrpc":"2.0","id":1,"method":"ping"}`, false).Code != 401 {
		t.Fatal("transport/auth")
	}
	w := invoke("POST", `{"jsonrpc":"2.0","id":"actual","method":"tools/call","params":{"name":"api_bot_status","arguments":{}}}`, true)
	var p map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if calls != 1 || p["id"] != "actual" || !strings.Contains(w.Body.String(), "actual_handler") || p["result"].(map[string]any)["isError"] != false {
		t.Fatal(w.Body, calls)
	}
}
