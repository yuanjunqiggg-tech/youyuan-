package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type yyIdentityTestTransport func(*http.Request) (*http.Response, error)

func (f yyIdentityTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func yyIdentityTestSetup(t *testing.T, respond yyIdentityTestTransport) string {
	t.Helper()
	dir := t.TempDir()
	oldData, oldPath, oldClient := yyIdentity.Data, yyIdentity.Path, yyIdentityHTTPClient
	oldRemote, oldRemotePath, oldCancel := yyRemote, yyRemotePath, yyRemoteCancel
	oldWorker := yyAccounts.Worker
	yyAccounts.Worker = false
	yyIdentity.Data = yyPlatformIdentity{}
	yyIdentity.Path = filepath.Join(dir, "platform-identity.json")
	yyRemote = yyRemoteConfig{}
	yyRemotePath = filepath.Join(dir, "youyuan-remote.json")
	yyRemoteCancel = nil
	yyIdentityHTTPClient = &http.Client{Transport: respond}
	t.Cleanup(func() {
		yyIdentity.Data = oldData
		yyIdentity.Path = oldPath
		yyIdentityHTTPClient = oldClient
		yyRemote = oldRemote
		yyRemotePath = oldRemotePath
		yyRemoteCancel = oldCancel
		yyAccounts.Worker = oldWorker
	})
	return dir
}
func yyIdentityTestResponse(status int, value any) *http.Response {
	body, _ := json.Marshal(value)
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}
}
func TestYYIdentityRegistrationPreflightAvoidsOrphanAccount(t *testing.T) {
	calls := []string{}
	dir := yyIdentityTestSetup(t, func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.Path)
		return yyIdentityTestResponse(200, map[string]any{"ok": true, "user": map[string]any{"id": "original", "username": "original_user"}}), nil
	})
	yyIdentity.Data = yyPlatformIdentity{User: map[string]any{"id": "original", "username": "original_user"}, Control: "private-control"}
	mux := http.NewServeMux()
	yyIdentityRoutes(mux, dir)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/youyuan/identity", strings.NewReader(`{"action":"register","username":"new_user","email":"123456@qq.com","email_code":"123456","password":"long-password"}`)))
	if !strings.Contains(w.Body.String(), "退出登录不会注销") {
		t.Fatal(w.Body.String())
	}
	if len(calls) != 1 || !strings.HasSuffix(calls[0], "/binding") {
		t.Fatal(calls)
	}
}
func TestYYIdentityDeletedBindingClearsTokensButRetainsProjects(t *testing.T) {
	dir := yyIdentityTestSetup(t, func(r *http.Request) (*http.Response, error) {
		return yyIdentityTestResponse(401, map[string]any{"ok": false, "error": "请先登录"}), nil
	})
	yyIdentity.Data = yyPlatformIdentity{User: map[string]any{"id": "deleted"}, Control: "control", Device: "revoked-device"}
	yyRemote = yyRemoteConfig{DeviceToken: "revoked-device", Enabled: true, Key: "local-mcp-unchanged"}
	resource := filepath.Join(dir, "project.lua")
	if e := os.WriteFile(resource, []byte("retained source"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := yyIdentityCheckBindingLocked(); e != nil {
		t.Fatal(e)
	}
	if yyIdentity.Data.User != nil || yyRemote.Enabled || yyRemote.DeviceToken != "" || yyRemote.Key != "local-mcp-unchanged" {
		t.Fatal("stale credentials not cleared")
	}
	if raw, e := os.ReadFile(resource); e != nil || string(raw) != "retained source" {
		t.Fatal("project removed")
	}
}
func TestYYIdentityNetworkFailureDoesNotUnbind(t *testing.T) {
	yyIdentityTestSetup(t, func(r *http.Request) (*http.Response, error) {
		return yyIdentityTestResponse(503, map[string]any{"ok": false, "error": "临时离线"}), nil
	})
	yyIdentity.Data = yyPlatformIdentity{User: map[string]any{"id": "existing"}, Control: "control"}
	if yyIdentityCheckBindingLocked() == nil {
		t.Fatal("failure swallowed")
	}
	if yyIdentity.Data.User == nil {
		t.Fatal("temporary failure erased binding")
	}
}
func TestYYIdentityDeleteFailureDoesNotClearLocalAccount(t *testing.T) {
	dir := yyIdentityTestSetup(t, func(r *http.Request) (*http.Response, error) {
		return yyIdentityTestResponse(403, map[string]any{"ok": false, "error": "密码不正确"}), nil
	})
	yyIdentity.Data = yyPlatformIdentity{User: map[string]any{"id": "existing"}, Session: "session", Control: "control"}
	mux := http.NewServeMux()
	yyIdentityRoutes(mux, dir)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/youyuan/identity", strings.NewReader(`{"action":"delete","password":"incorrect"}`)))
	if yyIdentity.Data.User == nil || yyIdentity.Data.Session != "session" {
		t.Fatal("failed deletion erased state")
	}
}
