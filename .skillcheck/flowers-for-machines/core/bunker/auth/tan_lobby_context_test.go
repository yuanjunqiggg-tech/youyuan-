package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestTanLobbyContextCancelsHTTPAndAdapterWithoutReplay(t *testing.T) {
	for _, mode := range []string{"session", "session-body", "room", "room-body", "wrapper", "adapter"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			started, release := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				if mode == "session-body" || mode == "room-body" {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				close(started)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				client := &Client{client: *server.Client(), ClientOptions: &ClientOptions{AuthServer: server.URL}}
				wrapper := NewAccessWrapper(client, "fixture-room", "", "fixture-token", "", "")
				var err error
				switch mode {
				case "session", "session-body":
					_, err = CreateClientContext(ctx, &ClientOptions{AuthServer: server.URL})
				case "wrapper":
					_, err = wrapper.TanLobbyGetAccessContext(ctx, "fixture-room")
				case "adapter":
					_, err = NewTanLobbyAuthenticatorContext(ctx, wrapper).GetAccess("fixture-room")
				default:
					_, err = client.TanLobbyAuthContext(ctx, "fixture-room", "fixture-token")
				}
				finished <- err
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("HTTP request did not start")
			}
			cancel()
			select {
			case err := <-finished:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation cause lost: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled room authentication stayed blocked")
			}
			if calls.Load() != 1 {
				t.Fatalf("authentication replayed: %d requests", calls.Load())
			}
		})
	}
}

func TestTanLobbyLegacyEntryPointsKeepRequestAndResponseContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/phoenix/tan_lobby_login" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("unexpected legacy request", r.Method, r.URL.Path, r.Header)
		}
		_, _ = io.WriteString(w, `{"success":true,"user_unique_id":123,"user_player_name":"fixture-player"}`)
	}))
	defer server.Close()
	client := &Client{client: *server.Client(), ClientOptions: &ClientOptions{AuthServer: server.URL}}
	if response, err := client.TanLobbyAuth("fixture-room", "fixture-token"); err != nil || response.UserUniqueID != 123 {
		t.Fatal(response, err)
	}
	wrapper := NewAccessWrapper(client, "fixture-room", "", "fixture-token", "", "")
	if response, err := wrapper.TanLobbyGetAccess("fixture-room"); err != nil || response.UserUniqueID != 123 {
		t.Fatal(response, err)
	}
	if response, err := NewTanLobbyAuthenticator(wrapper).GetAccess("fixture-room"); err != nil || response.UserUniqueID != 123 {
		t.Fatal(response, err)
	}
}
