package g79client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type discoveryRoundTripper func(*http.Request) (*http.Response, error)

func (f discoveryRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func discoveryClientFixture(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	return &Client{ReleaseJSON: G79ReleaseJSON{LinkServerURL: "https://ignored.invalid/not-official-discovery"}, httpClient: &http.Client{Transport: discoveryRoundTripper(func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		u := *r.URL
		u.Scheme = target.Scheme
		u.Host = target.Host
		clone.URL = &u
		clone.Header.Set("X-Fixture-Transport", "used")
		return transport.RoundTrip(clone)
	})}}
}

func TestLinkDiscoveryUsesOfficialReleaseAndClientHTTPWire(t *testing.T) {
	var calls atomic.Int32
	cli := discoveryClientFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.Header.Get("X-Fixture-Transport") != "used" {
			t.Error("client HTTP transport or method lost")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/serverlist/adr_release.0.17.json":
			fmt.Fprint(w, `{"LinkServerUrl":"https://g79.update.netease.com/fixture-official-link.list"}`)
		case "/fixture-official-link.list":
			fmt.Fprint(w, `[{"ip":"127.0.0.1","port":10200,"status":0,"ServerType":"dockerA"},{"ip":"127.0.0.1","port":10201,"status":1,"ServerType":"dockerB"}]`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	list, err := cli.FetchG79LinkServersContext(context.Background())
	if err != nil || len(list) != 2 || calls.Load() != 2 {
		t.Fatalf("list=%v calls=%d err=%v", list, calls.Load(), err)
	}
	if list[1].Status.Int64() != 1 || list[1].ServerType != "dockerB" {
		t.Fatal("discovery must not interpret status or drop server type")
	}
}

func TestLinkDiscoveryCancellationClosesHTTP(t *testing.T) {
	for _, bodyBlocked := range []bool{false, true} {
		t.Run(fmt.Sprintf("body=%v", bodyBlocked), func(t *testing.T) {
			entered := make(chan struct{})
			closed := make(chan struct{})
			cli := discoveryClientFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if bodyBlocked {
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}
				close(entered)
				<-r.Context().Done()
				close(closed)
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := cli.FetchG79LinkServersContext(ctx); done <- err }()
			<-entered
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("%v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("HTTP discovery ignored cancellation")
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("HTTP server request leaked")
			}
		})
	}
}

func TestLinkDiscoveryDeadlineAndHTTPError(t *testing.T) {
	cli := &Client{httpClient: &http.Client{Transport: discoveryRoundTripper(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 12*time.Second {
			t.Error("missing discovery timeout")
		}
		return &http.Response{StatusCode: 503, Body: http.NoBody, Header: make(http.Header)}, nil
	})}}
	if _, err := cli.FetchG79LinkServersContext(context.Background()); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("%v", err)
	}
	entered := make(chan struct{})
	cli = discoveryClientFixture(t, func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := cli.FetchG79LinkServersContext(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%v", err)
	}
	select {
	case <-entered:
	default:
		t.Fatal("fixture HTTP was not reached")
	}
}
