package minecraft

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

type egressRouteRoundTripper func(*http.Request) (*http.Response, error)

func (f egressRouteRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type egressRouteBody struct {
	io.Reader
	closed bool
}

func (b *egressRouteBody) Close() error { b.closed = true; return nil }

func TestOfficialGameEgressCandidatesAndCancellation(t *testing.T) {
	original := pharosClient
	defer func() { pharosClient = original }()
	body := &egressRouteBody{Reader: strings.NewReader(`{"harbor":{"servers":{"127.0.0.1":{"19132":[["127.0.0.2","19133"]]}}}}`)}
	pharosClient = &http.Client{Transport: egressRouteRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.Context().Err() != nil {
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
	})}
	got, e := getPharosSpeedUpIPContext(context.Background(), "127.0.0.1:19132")
	if e != nil || got != net.JoinHostPort("127.0.0.2", "19133") || !body.closed {
		t.Fatalf("candidate=%s error=%v body closed=%v", got, e, body.closed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = getPharosSpeedUpIPContext(ctx, "127.0.0.1:19132")
	if !errors.Is(e, context.Canceled) {
		t.Fatalf("cancelled candidate query continued: %v", e)
	}
	t.Log("Owned HTTP fixture validates official candidate parsing/body closure and propagates login cancellation; no external candidate query")
}
