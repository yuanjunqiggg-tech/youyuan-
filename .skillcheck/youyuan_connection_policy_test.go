package main

import (
	"bot-apk/config"
	"encoding/json"
	"errors"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func yyResetConnectionTest(t *testing.T) {
	t.Helper()
	yyAutoConnection.Lock()
	oldBusy, oldWanted, oldPaused, oldAttempted := yyAutoConnection.busy, yyAutoConnection.wanted, yyAutoConnection.paused, yyAutoConnection.attempted
	oldLast, oldAttempts, oldFailures, oldError := yyAutoConnection.last, yyAutoConnection.attempts, yyAutoConnection.failures, yyAutoConnection.lastError
	yyAutoConnection.busy, yyAutoConnection.wanted, yyAutoConnection.paused, yyAutoConnection.attempted = false, false, false, false
	yyAutoConnection.last = time.Time{}
	yyAutoConnection.attempts, yyAutoConnection.failures, yyAutoConnection.lastError = 0, 0, ""
	yyAutoConnection.Unlock()
	t.Cleanup(func() {
		yyAutoConnection.Lock()
		defer yyAutoConnection.Unlock()
		yyAutoConnection.busy, yyAutoConnection.wanted, yyAutoConnection.paused, yyAutoConnection.attempted = oldBusy, oldWanted, oldPaused, oldAttempted
		yyAutoConnection.last, yyAutoConnection.attempts, yyAutoConnection.failures, yyAutoConnection.lastError = oldLast, oldAttempts, oldFailures, oldError
	})
}

func TestReconnectContinuesAfterThirtyFailedRounds(t *testing.T) {
	yyResetConnectionTest(t)
	c := &config.AppConfig{Token: "test", ServerCode: "test", AuthURL: "https://example.invalid", AutoReconnect: true, ReconnectSeconds: 1}
	for round := 0; round < 30; round++ {
		if !yyBeginConnectionAttempt() {
			t.Fatal("gate stayed busy", round)
		}
		if yyMayAutoConnect(c, false, time.Now().Add(time.Hour)) {
			t.Fatal("overlapping connection allowed")
		}
		yyEndConnectionAttempt(errors.New("test login failure"))
		if yyMayAutoConnect(c, false, time.Now()) {
			t.Fatal("retry interval ignored")
		}
		if !yyMayAutoConnect(c, false, time.Now().Add(2*time.Second)) {
			t.Fatal("retries stopped after failure", round)
		}
	}
	s := yyConnectionState(c, nil)
	if s["consecutive_failures"] != uint64(30) || s["retry_limit"] != nil {
		t.Fatal(s)
	}
	yyPauseAutoConnection()
	if yyMayAutoConnect(c, false, time.Now().Add(time.Hour)) {
		t.Fatal("manual pause ignored")
	}
	yyResumeAutoConnection()
	if !yyMayAutoConnect(c, false, time.Now()) {
		t.Fatal("explicit resume did not reconnect")
	}
	c.AutoReconnect = false
	if yyMayAutoConnect(c, false, time.Now().Add(time.Hour)) {
		t.Fatal("disabled reconnect still active")
	}
}

func TestConnectionPolicyPersistsWithoutErasingCredentials(t *testing.T) {
	yyResetConnectionTest(t)
	config.InitConfig(t.TempDir())
	c := &config.AppConfig{Token: "private-test-token", ServerCode: "server", AuthURL: "https://example.invalid", AutoCreative: false, PacketQueueCapacity: 9000}
	if err := config.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	yyPauseAutoConnection()
	w := httptest.NewRecorder()
	yyConnectionPolicy(w, httptest.NewRequest("POST", "/api/youyuan/connection-policy", strings.NewReader(`{"auto_reconnect":true}`)))
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response["ok"] != true {
		t.Fatal(w.Body.String(), err)
	}
	saved := config.ReloadConfig()
	if !saved.AutoReconnect || saved.Token != c.Token || saved.ServerCode != c.ServerCode || saved.AutoCreative || saved.PacketQueueCapacity != 9000 {
		t.Fatal("partial update changed other settings")
	}
	if strings.Contains(w.Body.String(), c.Token) {
		t.Fatal("policy response exposed credentials")
	}
	if response["policy"].(map[string]any)["paused"] != false {
		t.Fatal("enabling reconnect left manual pause latched")
	}
	before := *saved
	w = httptest.NewRecorder()
	yyConnectionPolicy(w, httptest.NewRequest("POST", "/api/youyuan/connection-policy", strings.NewReader(`{"auto_reconnect":false,"reconnect_seconds":0}`)))
	if config.ReloadConfig().AutoReconnect != before.AutoReconnect {
		t.Fatal("invalid request partly committed")
	}
}

func TestChildCreativeDefaultPreservesExplicitOptOut(t *testing.T) {
	yyPoolTestSetup(t)
	a := yyAccounts.Items[0]
	if err := yyWriteChildConfig(a, true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(yyAccountDir(a.ID), "config.json")
	b, _ := os.ReadFile(path)
	var c config.AppConfig
	if err := json.Unmarshal(b, &c); err != nil || !c.AutoCreative {
		t.Fatal("new account maintenance not enabled", err)
	}
	c.AutoCreative = false
	b, _ = json.Marshal(c)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := yyWriteChildConfig(a, true); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	_ = json.Unmarshal(b, &c)
	if c.AutoCreative {
		t.Fatal("explicit survival-test opt out overwritten")
	}
}

func TestDirectConnectCredentialsSurviveRestartWithPolicy(t *testing.T) {
	config.InitConfig(t.TempDir())
	if err := config.SaveConfig(&config.AppConfig{AutoReconnect: true, AutoCreative: true, ReconnectSeconds: 7, ImportSpeed: 9876}); err != nil {
		t.Fatal(err)
	}
	if err := yySaveConnectionCredentials("test-credential", "test-server", "https://example.invalid", "test-pass", true); err != nil {
		t.Fatal(err)
	}
	c := config.ReloadConfig()
	if c.Token != "test-credential" || c.ServerCode != "test-server" || c.AuthURL != "https://example.invalid" || c.ServerPass != "test-pass" || !c.UseNewProtocol || !c.AutoReconnect || !c.AutoCreative || c.ReconnectSeconds != 7 || c.ImportSpeed != 9876 {
		t.Fatal("session inputs or existing policy not retained")
	}
}

func TestFlyingAbilityLayersDoNotInventConfirmation(t *testing.T) {
	cases := []struct {
		layers        []protocol.AbilityLayer
		flying, known bool
	}{
		{nil, false, false},
		{[]protocol.AbilityLayer{{Values: protocol.AbilityFlying}}, false, false},
		{[]protocol.AbilityLayer{{Abilities: protocol.AbilityFlying}}, false, true},
		{[]protocol.AbilityLayer{{Abilities: protocol.AbilityFlying}, {Abilities: protocol.AbilityFlying, Values: protocol.AbilityFlying}}, true, true},
		{[]protocol.AbilityLayer{{Abilities: protocol.AbilityFlying, Values: protocol.AbilityFlying}, {Abilities: protocol.AbilityFlying}}, false, true},
	}
	for i, c := range cases {
		f, k := yyFlyingAbility(c.layers)
		if f != c.flying || k != c.known {
			t.Fatal(i, f, k)
		}
	}
	bm := &BotManager{connected: true, gameMode: 0, isFlying: false, flyRequested: true}
	state := yyConnectionState(&config.AppConfig{AutoCreative: true}, bm)
	if state["creative_confirmed"] != false || state["flying_confirmed"] != false {
		t.Fatal("request presented as confirmed", state)
	}
}

func TestFlightRequestDoesNotInventServerState(t *testing.T) {
	bm := NewBotManager()
	if err := bm.StartFlying(); err == nil {
		t.Fatal("flight without connection succeeded")
	}
	if bm.isFlying {
		t.Fatal("outbound flight request presented as server confirmation")
	}
	bm.isFlying = true
	if err := bm.StopFlying(); err == nil {
		t.Fatal("stop without connection succeeded")
	}
	if !bm.isFlying {
		t.Fatal("failed stop erased last received flight state")
	}
}
