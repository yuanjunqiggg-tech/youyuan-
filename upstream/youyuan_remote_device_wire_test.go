package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestRemoteDeviceSelectorOnWire(t *testing.T) {
	for _, tc := range []struct{ name, device, query string }{
		{"explicit header", "linux-server", ""},
		{"query fallback", "", "linux-server"},
		{"header wins conflict", "linux-server", "primary-device"},
		{"no selector", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := make(chan *http.Request, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen <- r.Clone(r.Context())
				conn, err := websocket.Accept(w, r, nil)
				if err == nil {
					defer conn.CloseNow()
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			address := "ws" + strings.TrimPrefix(server.URL, "http") + "/device/connect"
			if tc.query != "" {
				address += "?device=" + tc.query
			}
			conn, _, err := websocket.Dial(ctx, address, &websocket.DialOptions{HTTPHeader: yyRemoteDialHeaders("fixture-device-only", tc.device)})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.CloseNow()
			observed := <-seen
			if observed.Header.Get("X-YouYuan-Device") != tc.device || observed.URL.Query().Get("device") != tc.query || observed.Header.Get("Authorization") != "Bearer fixture-device-only" || observed.UserAgent() != "YouYuan-Toolbox/"+yyAppVersion {
				t.Fatal("native WebSocket changed the device selector or authentication headers")
			}
		})
	}
}
