package main

import (
	"strings"
	"testing"
	"time"
)

func TestConnectionExpiredServerDoesNotBecomeCredentialFailure(t *testing.T) {
	message := "认证 HTTP 400: 进入山头失败：山头服已过期(1005026)"
	if got := yyConnectionErrorKind(message); got != "server_expired" {
		t.Fatalf("kind=%s", got)
	}
	if !strings.Contains(yyConnectionActionHint("server_expired"), "服务器已过期") {
		t.Fatal("missing actionable reason")
	}
	if got := yyConnectionRetryDelay(10*time.Second, 245, message); got != 5*time.Minute {
		t.Fatalf("existing bounded retry changed: %s", got)
	}
	if got := yyConnectionErrorKind("认证 HTTP 400: 请先登录"); got != "credential_rejected" {
		t.Fatalf("credential kind=%s", got)
	}
	if got := yyConnectionActionHint("connection_error"); got != "" {
		t.Fatal("unknown failures must not claim server expired")
	}
}
