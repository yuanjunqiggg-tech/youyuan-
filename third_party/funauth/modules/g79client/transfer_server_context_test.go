package g79client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transferFixtureTransport func(*http.Request) (*http.Response, error)

func (f transferFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTransferQueriesHonorCancellation(t *testing.T) {
	for _, list := range []bool{false, true} {
		t.Run(map[bool]string{false: "room", true: "servers"}[list], func(t *testing.T) {
			entered := make(chan struct{})
			cli := &Client{UserID: "123", ReleaseJSON: G79ReleaseJSON{TransferServerNewHttpUrl: "https://fixture.invalid"}, httpClient: &http.Client{Transport: transferFixtureTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "g79.update.netease.com" {
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"TransferServerUrl":"https://fixture.invalid/list"}`)), Header: make(http.Header)}, nil
				}
				close(entered)
				<-r.Context().Done()
				return nil, r.Context().Err()
			})}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if list {
					_, err := cli.FetchG79TransferServersContext(ctx)
					done <- err
				} else {
					_, err := cli.GetTransferRoomWithNameContext(ctx, "123")
					done <- err
				}
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("query did not start")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("canceled transfer request remained blocked")
			}
		})
	}
}
