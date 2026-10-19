package g79client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkinBodyTypeOfficialMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, body  string
		slim, valid bool
	}{
		{"classic", `{"code":0,"entities":[{"skin_body_type":0}]}`, false, true},
		{"slim", `{"code":0,"entities":[{"skin_body_type":1}]}`, true, true},
		{"missing", `{"code":0,"entities":[{}]}`, false, false},
		{"unknown", `{"code":0,"entities":[{"skin_body_type":9}]}`, false, false},
		{"refused", `{"code":403,"entities":[{"skin_body_type":1}]}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/pe-item/query/search-by-id-list" || r.Header.Get("user-id") != "test-user" || r.Header.Get("user-token") == "" {
					t.Error("missing official method/path or signed credentials")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := &Client{UserID: "test-user", UserToken: "test-token", ReleaseJSON: G79ReleaseJSON{ApiGatewayUrl: srv.URL}, httpClient: srv.Client()}
			slim, err := c.GetSkinBodyType("123")
			if (err == nil) != tc.valid || slim != tc.slim {
				t.Fatalf("slim=%v err=%v", slim, err)
			}
		})
	}
}
