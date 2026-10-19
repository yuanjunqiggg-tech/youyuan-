package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func identityConfigFixture(t *testing.T) (string, *AppConfig) {
	t.Helper()
	oldPath, oldCurrent, oldIdentity := cfgPath, current, currentGameIdentity
	t.Cleanup(func() {
		configMu.Lock()
		cfgPath, current, currentGameIdentity = oldPath, oldCurrent, oldIdentity
		configMu.Unlock()
	})
	dir := t.TempDir()
	InitConfig(dir)
	cfg := LoadConfig()
	cfg.Token = "fixture-credential-A"
	cfg.AuthURL = "https://auth.example.invalid"
	cfg.MainAccountName = "local remark"
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return dir, cfg
}

func TestAuthenticatedIdentityPersistsWithoutPublicBinding(t *testing.T) {
	dir, cfg := identityConfigFixture(t)
	const uid = "9007199254740993"
	if err := RecordAuthenticatedGameID(cfg.Token, cfg.AuthURL, uid); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var disk diskConfig
	if json.Unmarshal(raw, &disk) != nil || disk.GameIdentity == nil || disk.GameIdentity.CredentialBinding == "" {
		t.Fatal("private identity not atomically persisted")
	}
	public, err := json.Marshal(LoadConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"credential_binding", "authenticated_game_identity", disk.GameIdentity.CredentialBinding, uid} {
		if strings.Contains(string(public), secret) {
			t.Fatal("public config exposed private identity/binding")
		}
	}
	// Public settings cannot populate or replace the trusted disk-only cache.
	var posted map[string]any
	_ = json.Unmarshal(public, &posted)
	posted["authenticated_game_identity"] = map[string]any{"game_id": "1234", "credential_binding": "forged-fixture"}
	data, _ := json.Marshal(posted)
	copy := LoadConfig()
	if json.Unmarshal(data, copy) != nil {
		t.Fatal("fixture")
	}
	if err := SaveConfig(copy); err != nil {
		t.Fatal(err)
	}
	InitConfig(dir)
	got, at := AuthenticatedGameID()
	if got != uid || at == "" {
		t.Fatal("restart lost the exact verified UID")
	}
}

func TestAuthenticatedIdentityCredentialChangesInvalidate(t *testing.T) {
	_, cfg := identityConfigFixture(t)
	if err := RecordAuthenticatedGameID(cfg.Token, cfg.AuthURL, "3079938027"); err != nil {
		t.Fatal(err)
	}
	cfg.ImportSpeed = 12345
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if uid, _ := AuthenticatedGameID(); uid == "" {
		t.Fatal("unrelated settings invalidated identity")
	}
	original := cfg.Token
	cfg.Token = "fixture-credential-B"
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if uid, _ := AuthenticatedGameID(); uid != "" {
		t.Fatal("changed token retained old identity")
	}
	if RecordAuthenticatedGameID(original, cfg.AuthURL, "3079938027") == nil {
		t.Fatal("late old-session confirmation accepted")
	}
	cfg.Token = original
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if uid, _ := AuthenticatedGameID(); uid != "" {
		t.Fatal("A to B to A resurrected stale identity")
	}
	if err := RecordAuthenticatedGameID(cfg.Token, cfg.AuthURL, "3079938027"); err != nil {
		t.Fatal(err)
	}
	cfg.AuthURL = "https://other.example.invalid"
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if uid, _ := AuthenticatedGameID(); uid != "" {
		t.Fatal("different provider retained identity")
	}
}

func TestAuthenticatedIdentityWriteFailureKeepsPreviousState(t *testing.T) {
	dir, cfg := identityConfigFixture(t)
	if err := RecordAuthenticatedGameID(cfg.Token, cfg.AuthURL, "3079938027"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "config.json.tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	replacement := *cfg
	replacement.Token = "fixture-other"
	if SaveConfig(&replacement) == nil {
		t.Fatal("fixture write failure not returned")
	}
	if LoadConfig().Token != cfg.Token {
		t.Fatal("failed write changed in-memory credentials")
	}
	if uid, _ := AuthenticatedGameID(); uid != "3079938027" {
		t.Fatal("failed write lost previous trusted identity")
	}
	if RecordAuthenticatedGameID(cfg.Token, cfg.AuthURL, "1234") == nil {
		t.Fatal("failed identity write not returned")
	}
	if uid, _ := AuthenticatedGameID(); uid != "3079938027" {
		t.Fatal("failed identity write changed trusted UID")
	}
}

func TestAuthenticatedIdentityRejectsUntrustedUIDShapes(t *testing.T) {
	_, cfg := identityConfigFixture(t)
	for _, uid := range []string{"", "0", "-1", "nickname", "00123", "9223372036854775808"} {
		if RecordAuthenticatedGameID(cfg.Token, cfg.AuthURL, uid) == nil {
			t.Fatal("invalid UID accepted")
		}
	}
	if uid, _ := AuthenticatedGameID(); uid != "" {
		t.Fatal("invalid input produced identity")
	}
}
