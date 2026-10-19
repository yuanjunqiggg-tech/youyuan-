package main

import (
	"bot-apk/config"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Exercise the actual download entry points against a local server that refuses
// access. No market package is fetched or installed, and upstream authorization
// failures must remain failures rather than becoming successful downloads.
func TestMarketDownloadEntryPointsScopedCredentials(t *testing.T) {
	cases := []struct {
		name, marketToken, dedicatedHeader, want string
		sameProvider                             bool
	}{
		{name: "configured market token", marketToken: "market-test", want: "Bearer market-test"},
		{name: "explicit market credential", marketToken: "market-test", dedicatedHeader: "Bearer explicit-market", want: "Bearer explicit-market"},
		{name: "matching provider automatically bound", sameProvider: true, want: "Bearer game-test"},
		{name: "foreign game and device credentials excluded"},
		{name: "device credential excluded even without game token"},
	}
	for _, endpoint := range []string{"building", "plugin"} {
		for _, tc := range cases {
			t.Run(endpoint+"/"+tc.name, func(t *testing.T) {
				config.InitConfig(t.TempDir())
				seen, seenPath, calls := "", "", 0
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					seen, seenPath = r.Header.Get("Authorization"), r.URL.Path
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					io.WriteString(w, `{"ok":false,"error":"official permission required"}`)
				}))
				defer upstream.Close()
				cfg := &config.AppConfig{AuthURL: "https://auth.example.invalid", Token: "game-test", MarketURL: upstream.URL, MarketToken: tc.marketToken}
				if tc.sameProvider {
					cfg.AuthURL = upstream.URL + "/"
				}
				if tc.name == "device credential excluded even without game token" {
					cfg.Token = ""
				}
				if e := config.SaveConfig(cfg); e != nil {
					t.Fatal(e)
				}
				request := httptest.NewRequest(http.MethodGet, "/api/market/download?id=123", nil)
				if endpoint == "plugin" {
					request = httptest.NewRequest(http.MethodPost, "/api/plugin-market/install", strings.NewReader(`{"id":"123"}`))
				}
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer private-device-session")
				request.Header.Set("X-YouYuan-Market-Authorization", tc.dedicatedHeader)
				response := httptest.NewRecorder()
				wantPath := "/api/market/files/123/download"
				if endpoint == "plugin" {
					wantPath = "/api/plugin-market/files/123/download"
					handlePluginMarketInstall(response, request)
				} else {
					handleMarketDownload(response, request)
					if response.Code != http.StatusForbidden {
						t.Fatalf("upstream denial lost: HTTP %d", response.Code)
					}
				}
				if calls != 1 || seen != tc.want || seenPath != wantPath {
					t.Fatalf("credential routing: calls=%d authorization=%q path=%q", calls, seen, seenPath)
				}
				if !strings.Contains(response.Body.String(), "official permission required") || !strings.Contains(response.Body.String(), `"ok":false`) {
					t.Fatalf("official refusal was not preserved: %s", response.Body.String())
				}
			})
		}
	}
}
