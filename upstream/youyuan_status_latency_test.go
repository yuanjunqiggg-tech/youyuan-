package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bot-apk/config"
)

type yyBlockingStatusWriter struct {
	header  http.Header
	started chan struct{}
	release chan struct{}
	bytes.Buffer
}

func (w *yyBlockingStatusWriter) Header() http.Header { return w.header }

func (w *yyBlockingStatusWriter) WriteHeader(int) {}

func (w *yyBlockingStatusWriter) Write(p []byte) (int, error) {
	select {
	case w.started <- struct{}{}:
	default:
	}
	<-w.release
	return w.Buffer.Write(p)
}

func TestHandleBotStatusDoesNotHoldManagerLockWhileWriting(t *testing.T) {
	config.InitConfig(t.TempDir())

	botMgrMu.Lock()
	old := botMgr
	botMgr = &BotManager{}
	botMgrMu.Unlock()
	t.Cleanup(func() {
		botMgrMu.Lock()
		botMgr = old
		botMgrMu.Unlock()
	})

	w := &yyBlockingStatusWriter{header: make(http.Header), started: make(chan struct{}, 1), release: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		handleBotStatus(w, httptest.NewRequest(http.MethodGet, "/api/bot/status", nil))
		close(done)
	}()
	select {
	case <-w.started:
	case <-time.After(2 * time.Second):
		close(w.release)
		t.Fatal("status handler did not reach response write")
	}

	if !botMgrMu.TryLock() {
		close(w.release)
		<-done
		t.Fatal("handleBotStatus held botMgrMu while the response writer was blocked")
	}
	botMgrMu.Unlock()
	close(w.release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("status handler did not finish after response writer was released")
	}
}

func TestHandleBotStatusReportsUnknownGameRTTWhenDisconnected(t *testing.T) {
	config.InitConfig(t.TempDir())

	botMgrMu.Lock()
	old := botMgr
	botMgr = nil
	botMgrMu.Unlock()
	t.Cleanup(func() {
		botMgrMu.Lock()
		botMgr = old
		botMgrMu.Unlock()
	})

	rr := httptest.NewRecorder()
	handleBotStatus(rr, httptest.NewRequest(http.MethodGet, "/api/bot/status", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status returned HTTP %d", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if got["connected"] != false {
		t.Fatalf("connected = %v, want false", got["connected"])
	}
	if got["game_rtt_ms"] != nil {
		t.Fatalf("game_rtt_ms = %v, want null when disconnected", got["game_rtt_ms"])
	}
	if got["game_rtt_source"] != "raknet_ack" {
		t.Fatalf("game_rtt_source = %v, want raknet_ack", got["game_rtt_source"])
	}
}
