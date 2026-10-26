package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQQOfficialNumericGroupAndPreservedBinding(t *testing.T) {
	var old yyQQConfig
	old.Adapter = "qqofficial"
	old.Official.AppID = "123"
	old.GroupID = "group-openid"
	next := old
	next.GroupID = "1121432132"
	yyQQNormalizeConfig(&next, old)
	if next.GroupID != "group-openid" || next.GroupNumber != "1121432132" {
		t.Fatal(next)
	}
	next = old
	next.Official.AppID = "456"
	yyQQNormalizeConfig(&next, old)
	if next.GroupID != nil {
		t.Fatal("other bot inherited binding")
	}
}
func TestQQOfficialUnsignedValidationAndSignedEvents(t *testing.T) {
	saved := yyQQ
	defer func() { yyQQ = saved }()
	yyQQ.Enabled = true
	yyQQ.Adapter = "qqofficial"
	yyQQ.Official.AppID = "123"
	yyQQ.Official.Secret = "test-secret-for-qq"
	raw := `{"op":13,"d":{"plain_token":"hello","event_ts":"123"}}`
	req := httptest.NewRequest("POST", "/api/qq/webhook", strings.NewReader(raw))
	req.Header.Set("X-Bot-Appid", "123")
	w := httptest.NewRecorder()
	yyQQWebhook(w, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var reply map[string]string
	json.Unmarshal(w.Body.Bytes(), &reply)
	signature, _ := hex.DecodeString(reply["signature"])
	key := yyQQPrivateKey(yyQQ.Official.Secret)
	if !ed25519.Verify(key.Public().(ed25519.PublicKey), []byte("123hello"), signature) {
		t.Fatal("invalid challenge response")
	}
	req = httptest.NewRequest("POST", "/api/qq/webhook", strings.NewReader(`{"op":0,"t":"GROUP_AT_MESSAGE_CREATE","d":{}}`))
	req.Header.Set("X-Bot-Appid", "123")
	w = httptest.NewRecorder()
	yyQQWebhook(w, req)
	if w.Code != 401 {
		t.Fatal("unsigned group event accepted", w.Code)
	}
}
func TestQQOfficialGatewayAuthenticationAndCancellation(t *testing.T) {
	got := make(chan bool, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, e := websocket.Accept(w, r, nil)
		if e != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		hello := []byte(`{"op":10,"d":{"heartbeat_interval":1000}}`)
		conn.Write(ctx, websocket.MessageText, hello)
		_, raw, e := conn.Read(ctx)
		if e != nil {
			return
		}
		var payload struct {
			Op int
			D  struct {
				Token   string
				Intents int
			}
		}
		json.Unmarshal(raw, &payload)
		got <- payload.Op == 2 && payload.D.Token == "QQBot test" && payload.D.Intents == 1<<25
		conn.Write(ctx, websocket.MessageText, []byte(`{"op":0,"s":1,"t":"READY","d":{}}`))
		for {
			_, raw, e = conn.Read(ctx)
			if e != nil {
				return
			}
			conn.Write(ctx, websocket.MessageText, []byte(`{"op":11}`))
		}
	}))
	defer server.Close()
	saved := yyQQState
	defer func() { yyQQState = saved }()
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	err := yyQQGatewaySession(ctx, yyQQConfig{}, "test", "ws"+strings.TrimPrefix(server.URL, "http"))
	if err == nil {
		t.Fatal("session did not stop")
	}
	select {
	case ok := <-got:
		if !ok {
			t.Fatal("wrong identify")
		}
	default:
		t.Fatal("identify missing")
	}
}
