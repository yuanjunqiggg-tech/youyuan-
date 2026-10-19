package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type yyUAAuditTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (a yyUAAuditTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	if r.URL.Hostname() == "auth.youyuanqi.dpdns.org" || r.URL.Hostname() == "toolbox.youyuanqi.dpdns.org" {
		copied := *r.URL
		copied.Scheme, copied.Host = a.target.Scheme, a.target.Host
		clone.URL = &copied
	} else if r.URL.Hostname() != "localhost" && r.URL.Hostname() != "127.0.0.1" {
		return nil, fmt.Errorf("UA audit must stay on localhost")
	}
	return a.base.RoundTrip(clone)
}

func TestProductUserAgentOnOwnedHTTPAndRemoteHandshake(t *testing.T) {
	seen := make(chan map[string]string, 16)
	observe := func(r *http.Request) {
		seen <- map[string]string{"ua": r.UserAgent(), "authorization": r.Header.Get("Authorization"), "content_type": r.Header.Get("Content-Type"), "method": r.Method, "path": r.URL.Path, "audit": r.Header.Get("X-YouYuan-Audit")}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observe(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"allowed":true}`))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = yyUAAuditTransport{base: oldTransport, target: target}
	defer func() { http.DefaultTransport = oldTransport }()
	rows := []map[string]any{}
	record := func(name, file string, line int, method, path, authorization, contentType, audit string) {
		t.Helper()
		select {
		case observed := <-seen:
			if observed["ua"] != "YouYuan-Toolbox/"+yyAppVersion {
				t.Fatalf("%s product User-Agent missing or stale: %q", name, observed["ua"])
			}
			if observed["authorization"] != authorization || observed["content_type"] != contentType || observed["method"] != method || observed["path"] != path || observed["audit"] != audit {
				t.Fatalf("%s changed business headers or route: auth_matches=%v content_type=%q method=%q path=%q audit=%q", name, observed["authorization"] == authorization, observed["content_type"], observed["method"], observed["path"], observed["audit"])
			}
			rows = append(rows, map[string]any{"path": name, "file": file, "line": line, "wire_user_agent": observed["ua"], "authorization_preserved": true, "business_headers_preserved": true, "passed": true, "scope": "actual localhost wire capture of production builder; fixture credentials only"})
			t.Logf("%s actual wire User-Agent=%q; authorization and business headers preserved", name, observed["ua"])
		case <-time.After(time.Second):
			t.Fatal("no local HTTP request observed", name)
		}
	}
	oldIdentity := yyIdentityHTTPClient
	yyIdentityHTTPClient = &http.Client{Timeout: time.Second, Transport: http.DefaultTransport}
	defer func() { yyIdentityHTTPClient = oldIdentity }()
	if _, err := yyIdentityCall("me", nil, "fixture-platform-session"); err != nil {
		t.Fatal(err)
	}
	record("platform identity", "upstream/youyuan_identity.go", 41, "POST", "/api/identity/me", "Bearer fixture-platform-session", "application/json", "")
	if _, err := yyIdentityCall("login", nil, ""); err != nil {
		t.Fatal(err)
	}
	record("anonymous platform identity", "upstream/youyuan_identity.go", 41, "POST", "/api/identity/login", "", "application/json", "")
	if !yyCheckCloudCapability(context.Background(), "fixture-only", server.URL) {
		t.Fatal("local capability fixture failed")
	}
	record("cloud AI capability", "upstream/youyuan_agent_cloud.go", 42, "GET", "/api/cloud/ai-capability", "Bearer fixture-only", "", "")
	yyRemoteMu.Lock()
	oldDevice := yyRemote.DeviceToken
	yyRemote.DeviceToken = "fixture-only"
	yyRemoteMu.Unlock()
	defer func() { yyRemoteMu.Lock(); yyRemote.DeviceToken = oldDevice; yyRemoteMu.Unlock() }()
	mux := http.NewServeMux()
	yyGameAuthRoutes(mux)
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/youyuan/game-auth", nil))
	record("game auth vault proxy", "upstream/youyuan_game_auth.go", 30, "GET", "/api/accounts", "Bearer fixture-only", "application/json", "")
	socketDone := make(chan struct{})
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observe(r)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		<-socketDone
	}))
	defer wsServer.Close()
	defer close(socketDone)
	headers := yyRemoteDialHeaders("fixture-device-session")
	headers.Set("X-YouYuan-Audit", "fixture-header")
	conn, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(wsServer.URL, "http")+"/device/connect", &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	record("remote WebSocket handshake", "upstream/youyuan_remote.go", 507, "GET", "/device/connect", "Bearer fixture-device-session", "", "fixture-header")
	if yyRemoteDialHeaders("second-fixture").Get("X-YouYuan-Audit") != "" {
		t.Fatal("WebSocket handshake headers leaked between sessions")
	}
	if output := os.Getenv("YY_UA_AUDIT_OUT"); output != "" {
		data, _ := json.MarshalIndent(rows, "", "  ")
		if err := os.WriteFile(output, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
