package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGameAuthHonorsContextWithoutReplaying(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		started <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	client := &Client{client: *server.Client(), ClientOptions: &ClientOptions{AuthServer: server.URL}}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		_, e := client.Auth(ctx, "fixture-server", "", "fixture-key", "fixture-token", "", "")
		finished <- e
	}()
	<-started
	cancel()
	select {
	case e := <-finished:
		if !errors.Is(e, context.Canceled) || !strings.Contains(e.Error(), "phase=game_login") {
			t.Fatalf("wrong cancellation: %v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("game auth ignored cancellation")
	}
	if calls.Load() != 1 {
		t.Fatalf("authentication replayed %d times", calls.Load())
	}
}

func TestAuthTransportTimeoutDistinguishesSessionAndSubmittedLogin(t *testing.T) {
	for _, tc := range []struct{ phase, outcome string }{{"new_session", "no_game_login_submitted"}, {"game_login", "unknown"}} {
		err := authTransportError(tc.phase, &url.Error{Op: "Post", URL: "https://provider/delegated/private-grant", Err: context.DeadlineExceeded})
		if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "outcome="+tc.outcome) || strings.Contains(err.Error(), "private-grant") {
			t.Fatal(err)
		}
	}
}
