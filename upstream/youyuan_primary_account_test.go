package main

import (
	"bot-apk/config"
	"bot-apk/state"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func yyPrimaryFixture(t *testing.T) {
	t.Helper()
	yyPoolTestSetup(t)
	yyResetConnectionTest(t)
	yyAccounts.Items = nil
	config.InitConfig(yyAccounts.Root)
	if err := config.SaveConfig(config.LoadConfig()); err != nil {
		t.Fatal(err)
	}
	botMgrMu.Lock()
	previousBot := botMgr
	botMgr = nil
	botMgrMu.Unlock()
	taskMu.Lock()
	previousTask := activeTask
	activeTask = nil
	taskMu.Unlock()
	t.Cleanup(func() {
		botMgrMu.Lock()
		botMgr = previousBot
		botMgrMu.Unlock()
		taskMu.Lock()
		activeTask = previousTask
		taskMu.Unlock()
	})
}

func yyPrimaryPost(account yyManagedAccount, actor *yyPoolActor) map[string]any {
	raw, _ := json.Marshal(map[string]any{"action": "save", "defer_connect": true, "account": account})
	req := httptest.NewRequest("POST", "/api/youyuan/accounts", bytes.NewReader(raw))
	if actor != nil {
		req = req.WithContext(contextWithPoolActor(req, *actor))
	}
	out := httptest.NewRecorder()
	yyMux.ServeHTTP(out, req)
	var result map[string]any
	_ = json.Unmarshal(out.Body.Bytes(), &result)
	return result
}

func contextWithPoolActor(req *http.Request, actor yyPoolActor) context.Context {
	return context.WithValue(req.Context(), yyPoolActorContext{}, actor)
}

func yyPrimaryGet(t *testing.T) (map[string]any, string) {
	t.Helper()
	out := httptest.NewRecorder()
	yyMux.ServeHTTP(out, httptest.NewRequest("GET", "/api/youyuan/accounts?view=summary", nil))
	var result map[string]any
	if json.Unmarshal(out.Body.Bytes(), &result) != nil || result["ok"] != true {
		t.Fatal("account summary failed")
	}
	return result, out.Body.String()
}

func yyPrimaryAccountFixture(id, token string) yyManagedAccount {
	return yyManagedAccount{ID: id, Name: "same local nickname", Token: token, AuthURL: "https://auth.example.invalid", Server: "fixture-server", KeepAlive: true}
}

func TestPrimaryFirstActualSaveUsesMainWithoutProcessOrAutoConnect(t *testing.T) {
	yyPrimaryFixture(t)
	a := yyPrimaryAccountFixture("first", "fixture-first-credential")
	result := yyPrimaryPost(a, nil)
	if result["ok"] != true || result["id"] != "main" || result["connection_requested"] != false {
		t.Fatal("first explicit save did not return deferred main")
	}
	cfg := config.ReloadConfig()
	if cfg.Token != a.Token || cfg.ServerCode != a.Server || cfg.MainAccountName != a.Name || cfg.AutoConnect || !cfg.AutoReconnect {
		t.Fatal("primary credentials or preferences not saved")
	}
	if len(yyAccounts.Items) != 0 {
		t.Fatal("first primary also consumed a child profile")
	}
	if _, err := os.Stat(filepath.Join(yyAccounts.Root, "managed-accounts", a.ID)); !os.IsNotExist(err) {
		t.Fatal("first primary started/created a child workspace")
	}
	yyAutoConnection.Lock()
	paused, wanted, busy := yyAutoConnection.paused, yyAutoConnection.wanted, yyAutoConnection.busy
	yyAutoConnection.Unlock()
	if !paused || wanted || busy {
		t.Fatal("save implicitly requested a game connection")
	}
	second := yyPrimaryPost(yyPrimaryAccountFixture("second", "fixture-second-credential"), nil)
	if second["ok"] != true || second["id"] != "second" || len(yyAccounts.Items) != 1 || yyAccounts.Items[0].Slot != 0 || yyAccounts.Items[0].Enabled {
		t.Fatal("second real account did not remain an idle child")
	}
	summary, raw := yyPrimaryGet(t)
	main := summary["main"].(map[string]any)
	if main["id"] != "main" || main["configured"] != true || main["name"] != a.Name || main["game_id"] != nil {
		t.Fatal("stored unverified credential invented a game identity")
	}
	if strings.Contains(raw, a.Token) || strings.Contains(raw, yyGameCredentialBinding(a.Token, a.AuthURL)) {
		t.Fatal("account summary leaked private credential material")
	}
}

func TestPrimaryConcurrentFirstSavesHaveOneMain(t *testing.T) {
	yyPrimaryFixture(t)
	var wg sync.WaitGroup
	results := make(chan map[string]any, 2)
	for _, a := range []yyManagedAccount{yyPrimaryAccountFixture("one", "fixture-one"), yyPrimaryAccountFixture("two", "fixture-two")} {
		wg.Add(1)
		go func(a yyManagedAccount) { defer wg.Done(); results <- yyPrimaryPost(a, nil) }(a)
	}
	wg.Wait()
	close(results)
	mains := 0
	for r := range results {
		if r["ok"] != true {
			t.Fatal("concurrent account save failed")
		}
		if r["id"] == "main" {
			mains++
		}
	}
	if mains != 1 || len(yyAccounts.Items) != 1 || yyAccounts.Items[0].Token == config.ReloadConfig().Token {
		t.Fatal("concurrent first saves duplicated or replaced main")
	}
}

func TestPrimaryFailedFirstSaveRollsBackAndCanRetry(t *testing.T) {
	yyPrimaryFixture(t)
	path := filepath.Join(yyAccounts.Root, "config.json.tmp")
	if os.Mkdir(path, 0700) != nil {
		t.Fatal("fixture")
	}
	a := yyPrimaryAccountFixture("first", "fixture-first")
	if r := yyPrimaryPost(a, nil); r["ok"] != false {
		t.Fatal("failed main write reported success")
	}
	if config.ReloadConfig().Token != "" || len(yyAccounts.Items) != 0 {
		t.Fatal("failed first save left partial primary/child state")
	}
	yyAutoConnection.Lock()
	paused := yyAutoConnection.paused
	yyAutoConnection.Unlock()
	if paused {
		t.Fatal("failed write changed connection policy")
	}
	if os.Remove(path) != nil {
		t.Fatal("fixture cleanup")
	}
	if r := yyPrimaryPost(a, nil); r["ok"] != true || r["id"] != "main" {
		t.Fatal("first save could not recover after failed write")
	}
}

func TestPrimaryExactCredentialIsIdempotentButNameIsNotIdentity(t *testing.T) {
	yyPrimaryFixture(t)
	a := yyPrimaryAccountFixture("first", "fixture-first")
	if yyPrimaryPost(a, nil)["id"] != "main" {
		t.Fatal("fixture")
	}
	yyResumeAutoConnection()
	alias := a
	alias.ID = "another-request-id"
	alias.Name = "different local remark"
	if yyPrimaryPost(alias, nil)["id"] != "main" || len(yyAccounts.Items) != 0 || config.ReloadConfig().MainAccountName != a.Name {
		t.Fatal("exact credential was duplicated or silently changed main")
	}
	yyAutoConnection.Lock()
	paused := yyAutoConnection.paused
	yyAutoConnection.Unlock()
	if paused {
		t.Fatal("idempotent save paused an existing connection policy")
	}
	other := a
	other.ID = "same-name-other-account"
	other.Token = "fixture-distinct"
	if yyPrimaryPost(other, nil)["id"] != other.ID {
		t.Fatal("same name merged distinct credentials")
	}
}

func TestPrimarySubscriberAndLegacyChildrenCannotClaimMain(t *testing.T) {
	yyPrimaryFixture(t)
	actor := yyPoolActor{ID: "other-tenant", Scope: "control"}
	a := yyPrimaryAccountFixture("subscriber", "fixture-subscriber")
	if yyPrimaryPost(a, &actor)["id"] != "subscriber" || config.ReloadConfig().Token != "" {
		t.Fatal("tenant claimed host main")
	}
	before := yyAccounts.Items[0]
	owner := yyPrimaryAccountFixture("new-owner-account", "fixture-owner")
	if yyPrimaryPost(owner, nil)["id"] != "new-owner-account" || config.ReloadConfig().Token != "" || yyAccounts.Items[0] != before {
		t.Fatal("legacy children were guessed/migrated by order")
	}
}

func TestPrimarySavePreservesExistingConnectionAndTask(t *testing.T) {
	for _, guard := range []string{"connection", "task", "credentials"} {
		t.Run(guard, func(t *testing.T) {
			yyPrimaryFixture(t)
			bm := &BotManager{connected: true}
			task := &state.TaskController{}
			if guard == "connection" {
				botMgr = bm
			}
			if guard == "task" {
				activeTask = task
			}
			if guard == "credentials" {
				cfg := config.LoadConfig()
				cfg.Token = "fixture-existing"
				cfg.ServerCode = "existing-server"
				if config.SaveConfig(cfg) != nil {
					t.Fatal("fixture")
				}
			}
			old := config.ReloadConfig()
			a := yyPrimaryAccountFixture("added", "fixture-added")
			if yyPrimaryPost(a, nil)["id"] != "added" {
				t.Fatal("existing main resources were replaced")
			}
			now := config.ReloadConfig()
			if now.Token != old.Token || now.ServerCode != old.ServerCode {
				t.Fatal("adding child changed main config")
			}
			if guard == "connection" && (!bm.connected || botMgr != bm) {
				t.Fatal("existing main connection disturbed")
			}
			if guard == "task" && activeTask != task {
				t.Fatal("existing task disturbed")
			}
		})
	}
}

func TestPrimaryVerifiedUIDSurvivesOfflineAndDisappearsOnTokenChange(t *testing.T) {
	yyPrimaryFixture(t)
	a := yyPrimaryAccountFixture("first", "fixture-verified")
	if yyPrimaryPost(a, nil)["id"] != "main" {
		t.Fatal("fixture")
	}
	yyRememberMainAuthenticatedGameID(a.Token, a.AuthURL, 3079938027)
	config.InitConfig(yyAccounts.Root)
	result, raw := yyPrimaryGet(t)
	main := result["main"].(map[string]any)
	if main["game_id"] != "3079938027" || main["game_identity_source"] != "authenticated_cache" || main["game_identity_confirmed_at"] == nil {
		t.Fatal("offline/restart lost authenticated UID")
	}
	if strings.Contains(raw, a.Token) || strings.Contains(raw, "credential_binding") || strings.Contains(raw, yyGameCredentialBinding(a.Token, a.AuthURL)) {
		t.Fatal("offline API leaked binding")
	}
	cfg := config.LoadConfig()
	cfg.Token = "fixture-replaced"
	if config.SaveConfig(cfg) != nil {
		t.Fatal("fixture")
	}
	result, _ = yyPrimaryGet(t)
	if result["main"].(map[string]any)["game_id"] != nil {
		t.Fatal("replaced token retained old UID")
	}
}
