package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCancelledTanLobbyLoginDoesNotContactAuthService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := LoginTanLobbyRoom(Config{LoginContext: ctx, AuthServerAddress: "http://192.0.2.1:1"})
	if client != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(client, err)
	}
}

func TestTanLobbyLoginCancellationReachesInitialAndDialAuthentication(t *testing.T) {
	for _, stage := range []int{0, 1, 2} {
		t.Run([]string{"new-session", "initial-room-auth", "dial-room-auth"}[stage], func(t *testing.T) {
			var calls atomic.Int32
			started, release := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := int(calls.Add(1)) - 1
				_, _ = io.Copy(io.Discard, r.Body)
				if call == stage {
					close(started)
					select {
					case <-r.Context().Done():
					case <-release:
					}
					return
				}
				if r.URL.Path == "/api/new" {
					_, _ = io.WriteString(w, "fixture-session")
				} else {
					_, _ = io.WriteString(w, `{"success":true,"user_unique_id":123,"user_player_name":"fixture-player"}`)
				}
			}))
			defer server.Close()
			defer close(release)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				_, err := LoginTanLobbyRoom(Config{LoginContext: ctx, AuthServerAddress: server.URL, RentalServerCode: "123", AuthServerToken: "fixture-token"})
				finished <- err
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("room auth phase did not start")
			}
			cancel()
			select {
			case err := <-finished:
				if !errors.Is(err, context.Canceled) {
					t.Fatal("lost cancellation", err)
				}
			case <-time.After(time.Second):
				t.Fatal("login ignored manual cancellation")
			}
			if calls.Load() != int32(stage+1) {
				t.Fatal("unexpected authentication retry", calls.Load())
			}
		})
	}
}

func TestTanLobbyDebugLogDoesNotIncludeRoomPassword(t *testing.T) {
	debugMu.Lock()
	oldFile, oldPath := debugFile, prismDebugLogPath
	debugFile = nil
	prismDebugLogPath = filepath.Join(t.TempDir(), "login.log")
	path := prismDebugLogPath
	debugMu.Unlock()
	defer func() {
		debugMu.Lock()
		defer debugMu.Unlock()
		if debugFile != nil {
			_ = debugFile.Close()
		}
		debugFile, prismDebugLogPath = oldFile, oldPath
	}()
	const password = "private-room-password-must-not-be-logged"
	_, _ = LoginTanLobbyRoom(Config{AuthServerAddress: "invalid-address", RentalServerCode: "123", RentalServerPasscode: password})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "房间号: 123") || strings.Contains(string(data), password) {
		t.Fatal("missing room diagnostics or exposed room password")
	}
}
