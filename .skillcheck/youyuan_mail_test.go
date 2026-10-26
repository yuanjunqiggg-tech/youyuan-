package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestYYMailBridgeRejectsForgedExpiredAndNonRegistrationRequests(t *testing.T) {
	dir := t.TempDir()
	cfg := yyMailConfig{RelaySecret: strings.Repeat("secret", 8)}
	raw, _ := json.Marshal(cfg)
	if e := os.WriteFile(filepath.Join(dir, "mail-config.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	yyMailRoutes(mux, dir)
	valid := `{"to":"123456@qq.com","code":"123456","purpose":"registration"}`
	for _, tc := range []struct {
		body   string
		age    time.Duration
		signed bool
		want   int
	}{
		{valid, 0, false, 403}, {valid, -10 * time.Minute, true, 403}, {`{"to":"victim@evil.test","code":"123456","purpose":"registration"}`, 0, true, 400}, {`{"to":"123456@qq.com","code":"123456","purpose":"anything"}`, 0, true, 400}, {valid, 0, true, 503},
	} {
		stamp := strconv.FormatInt(time.Now().Add(tc.age).UnixMilli(), 10)
		r := httptest.NewRequest("POST", "/api/youyuan/mail/send", strings.NewReader(tc.body))
		r.Header.Set("X-YouYuan-Mail-Timestamp", stamp)
		if tc.signed {
			mac := hmac.New(sha256.New, []byte(cfg.RelaySecret))
			mac.Write([]byte(stamp + "\n" + tc.body))
			r.Header.Set("X-YouYuan-Mail-Signature", hex.EncodeToString(mac.Sum(nil)))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
		}
	}
}
