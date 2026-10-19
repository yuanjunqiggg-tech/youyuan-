package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTanLobbyManualStopCancelsAuthAndRetryWait(t *testing.T) {
	for _, phase := range []string{"auth-request", "retry-wait"} {
		t.Run(phase, func(t *testing.T) {
			var calls atomic.Int32
			started, failed, release := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				select {
				case started <- struct{}{}:
				default:
				}
				if phase == "auth-request" {
					select {
					case <-r.Context().Done():
					case <-release:
					}
					return
				}
				http.Error(w, "fixture provider unavailable", http.StatusServiceUnavailable)
			}))
			defer server.Close()
			defer close(release)
			bm := NewBotManager()
			bm.onLoginFail = func(_ int, _ error) {
				select {
				case failed <- struct{}{}:
				default:
				}
			}
			finished := make(chan error, 1)
			go func() { finished <- bm.connectTanLobby("fixture-token", "123", server.URL, "") }()
			wait := started
			if phase == "retry-wait" {
				wait = failed
			}
			select {
			case <-wait:
			case <-time.After(3 * time.Second):
				t.Fatal("room login did not reach stop test phase")
			}
			bm.Disconnect()
			select {
			case err := <-finished:
				if err == nil || !strings.Contains(err.Error(), "取消") {
					t.Fatal("manual stop did not end room login", err)
				}
			case <-time.After(time.Second):
				t.Fatal("manual stop left room login blocked")
			}
			if calls.Load() != 1 || bm.IsConnected() {
				t.Fatalf("cancelled room login retried or stayed online: calls=%d", calls.Load())
			}
		})
	}
}
