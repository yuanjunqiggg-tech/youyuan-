package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	lua "github.com/yuin/gopher-lua"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func yyPoolTestSetup(t *testing.T) {
	t.Helper()
	root, items, worker, mux := yyAccounts.Root, yyAccounts.Items, yyAccounts.Worker, yyMux
	path, leases, keys := yyPool.Path, yyPool.Leases, yyPool.Keys
	user := yyIdentity.Data.User
	secret := yyPoolRelayIdentity.Secret
	dir := t.TempDir()
	yyAccounts.Root = dir
	yyAccounts.Worker = false
	yyIdentity.Data.User = map[string]any{"id": "platform-owner", "legacy_owner": true}
	yyAccounts.Items = []yyManagedAccount{{ID: "alice_bot", Name: "A", OwnerID: "alice", Token: "token-a", Server: "test"}, {ID: "bob_bot", Name: "B", OwnerID: "bob", Token: "token-b", Server: "test"}, {ID: "legacy_bot", Token: "token-owner", Server: "test"}}
	yyPool.Path = filepath.Join(dir, "account-pool.json")
	yyPool.Leases = map[string]yyPoolLease{}
	yyPool.Keys = map[string]string{}
	yyMux = http.NewServeMux()
	// Real account mutations; no process or network is started by these tests.
	yyAccounts.Worker = true
	yyAccountsRoutes(yyMux, dir)
	yyAccounts.Worker = false
	t.Cleanup(func() {
		yyAccounts.Root = root
		yyAccounts.Items = items
		yyAccounts.Worker = worker
		yyMux = mux
		yyPool.Path = path
		yyPool.Leases = leases
		yyPool.Keys = keys
		yyIdentity.Data.User = user
		yyPoolRelayIdentity.Secret = secret
	})
}
func TestPoolIdleExpiryAndRebindingCancelsExit(t *testing.T) {
	yyPoolTestSetup(t)
	actor := yyPoolActor{ID: "alice", Scope: "control"}
	_, e := yyPoolCall(actor, "accountsPool.bind", []any{"alice_bot", map[string]any{"player": "P"}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = yyPoolCall(actor, "accountsPool.release", []any{"alice_bot", map[string]any{"idle_seconds": 0.0}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = yyPoolCall(actor, "accountsPool.acquire", []any{map[string]any{"player": "Q", "server": "test"}})
	if e != nil {
		t.Fatal(e)
	}
	yyPoolExpire(time.Now().Add(time.Second).UnixMilli())
	if yyPool.Leases["alice_bot"].Player != "Q" {
		t.Fatal("old expiry destroyed new binding")
	}
	_, e = yyPoolCall(actor, "accountsPool.release", []any{"alice_bot", map[string]any{"idle_seconds": 0.0}})
	if e != nil {
		t.Fatal(e)
	}
	yyPoolExpire(time.Now().Add(time.Second).UnixMilli())
	if _, ok := yyPool.Leases["alice_bot"]; ok {
		t.Fatal("expired idle account was not released")
	}
	a, _ := yyManagedFind("alice_bot")
	if a.Enabled || a.Slot != 0 {
		t.Fatal("expired account kept running")
	}
}
func TestPoolLuaAndStructuredSDKBinding(t *testing.T) {
	yyPoolTestSetup(t)
	L := lua.NewState()
	defer L.Close()
	registerAccountPoolAPI(L)
	if e := L.DoString(`local v=accountsPool.list({all=true}); assert(#v.accounts==3); local b=accountsPool.acquire({all=true,id="alice_bot",player="P",server="test"}); assert(b.account=="alice_bot"); assert(b.phase=="bound"); local s=accountsPool.bindings({all=true}); assert(#s==1)`); e != nil {
		t.Fatal("Lua pool API did not execute", e)
	}
	s := sdkFixture(t, "pool_binding")
	v, e := s.call("accountsPool.list", []any{map[string]any{"all": true}})
	if e != nil || len(v.(map[string]any)["accounts"].([]any)) != 3 {
		t.Fatal("Python/C++ common SDK dispatch failed", v, e)
	}
}
func TestPoolAccountDeletionRevokesWorkerIdentity(t *testing.T) {
	yyPoolTestSetup(t)
	actor := yyPoolActor{ID: "alice", Scope: "control"}
	a, _ := yyManagedFind("alice_bot")
	if e := yyPoolWriteChildIdentity(a, t.TempDir()); e != nil {
		t.Fatal(e)
	}
	old := yyPool.Keys[a.ID]
	if _, e := yyPoolCall(actor, "accountsPool.bind", []any{a.ID, map[string]any{"player": "P"}}); e != nil {
		t.Fatal(e)
	}
	if _, e := yyPoolInvokeActor(actor, "POST", "/api/youyuan/accounts", map[string]any{"action": "delete", "id": a.ID}); e != nil {
		t.Fatal(e)
	}
	if yyPool.Keys[a.ID] != "" || yyPool.Leases[a.ID].Account != "" {
		t.Fatal("deleted account retained lease/key")
	}
	if _, e := yyPoolCall(actor, "accountsPool.add", []any{map[string]any{"id": a.ID, "server": "test", "auth_url": "https://auth.youyuanqi.dpdns.org"}}); e != nil {
		t.Fatal(e)
	}
	if yyPool.Keys[a.ID] == old {
		t.Fatal("recreated id inherited revoked worker key")
	}
}
func TestPoolTenantIsolationAndAdminAll(t *testing.T) {
	yyPoolTestSetup(t)
	alice := yyPoolActor{ID: "alice", Scope: "control"}
	rows, e := yyPoolVisible(alice, false)
	if e != nil || len(rows) != 1 || rows[0].ID != "alice_bot" {
		t.Fatal(rows, e)
	}
	if _, e = yyPoolVisible(alice, true); e == nil {
		t.Fatal("member enumerated all pools")
	}
	if _, e = yyPoolRequire(alice, "bob_bot"); e == nil {
		t.Fatal("member controlled another tenant")
	}
	rows, e = yyPoolVisible(yyPoolActor{ID: "owner", Admin: true}, true)
	if e != nil || len(rows) != 3 {
		t.Fatal(rows, e)
	}
	if yyPoolOwner(rows[2]) != "owner" {
		t.Fatal("legacy account ownership changed")
	}
	raw, _ := json.Marshal(rows[0])
	if !strings.Contains(string(raw), "token-a") {
		t.Fatal("fixture missing credential")
	}
	v, e := yyPoolCall(alice, "accountsPool.list", nil)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ = json.Marshal(v)
	if strings.Contains(string(raw), "token-a") {
		t.Fatal("pool list leaked credential")
	}
}
func TestPoolBindingReuseAndPersistence(t *testing.T) {
	yyPoolTestSetup(t)
	actor := yyPoolActor{ID: "alice", Scope: "control"}
	args := []any{map[string]any{"player": "疣螈", "server": "test"}}
	v, e := yyPoolCall(actor, "accountsPool.acquire", args)
	if e != nil {
		t.Fatal(e)
	}
	l := v.(yyPoolLease)
	if l.Account != "alice_bot" || l.Phase != "bound" {
		t.Fatal(l)
	}
	if _, e = yyPoolCall(actor, "accountsPool.bind", []any{"alice_bot", map[string]any{"player": "另一个玩家"}}); e == nil {
		t.Fatal("silent reassignment")
	}
	v, e = yyPoolCall(actor, "accountsPool.acquire", args)
	if e != nil || v.(yyPoolLease).Updated != l.Updated {
		t.Fatal("acquire not idempotent", v, e)
	}
	_, e = yyPoolCall(actor, "accountsPool.release", []any{"alice_bot", map[string]any{"idle_seconds": 60.0}})
	if e != nil {
		t.Fatal(e)
	}
	if yyPool.Leases["alice_bot"].IdleUntil-time.Now().UnixMilli() < 59000 {
		t.Fatal("idle grace period wrong")
	}
	args[0] = map[string]any{"player": "新玩家", "server": "test"}
	v, e = yyPoolCall(actor, "accountsPool.acquire", args)
	if e != nil || v.(yyPoolLease).Player != "新玩家" || v.(yyPoolLease).IdleUntil != 0 {
		t.Fatal("idle robot not reused", v, e)
	}
	yyPool.Leases = map[string]yyPoolLease{}
	yyPoolLoad(yyAccounts.Root)
	if yyPool.Leases["alice_bot"].Player != "新玩家" {
		t.Fatal("restart lost binding")
	}
}
func TestPoolReadonlyCannotMutateAndNoFabricatedCapacity(t *testing.T) {
	yyPoolTestSetup(t)
	actor := yyPoolActor{ID: "alice", Scope: "read"}
	for _, op := range []string{"acquire", "bind", "release", "start", "stop", "add", "invoke"} {
		if _, e := yyPoolCall(actor, "accountsPool."+op, nil); e == nil {
			t.Fatal("read token mutated", op)
		}
	}
	_, e := yyPoolCall(yyPoolActor{ID: "nobody", Scope: "control"}, "accountsPool.acquire", []any{map[string]any{"player": "P", "server": "test"}})
	if e == nil {
		t.Fatal("fabricated robot without an account")
	}
}
func TestPoolRejectsForgedOwnerAndForeignHTTPInvocations(t *testing.T) {
	yyPoolTestSetup(t)
	actor := yyPoolActor{ID: "alice", Scope: "control"}
	_, e := yyPoolCall(actor, "accountsPool.add", []any{map[string]any{"id": "alice_second", "owner_id": "bob", "token": "another-token", "server": "test", "auth_url": "https://auth.youyuanqi.dpdns.org"}})
	if e != nil {
		t.Fatal(e)
	}
	a, ok := yyManagedFind("alice_second")
	if !ok || a.OwnerID != "alice" {
		t.Fatal("forged owner accepted", a)
	}
	_, e = yyPoolCall(actor, "accountsPool.add", []any{map[string]any{"id": "bob_bot", "token": "stolen"}})
	if e == nil {
		t.Fatal("foreign account overwritten")
	}
	r := httptest.NewRequest("POST", yyAccountsPrefix+"bob_bot/api/bot/disconnect", nil)
	r = r.WithContext(context.WithValue(r.Context(), yyPoolActorContext{}, actor))
	w := httptest.NewRecorder()
	yyMux.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("HTTP prefix bypassed ownership", w.Code)
	}
	if _, e = yyPoolInvokeActor(actor, "POST", "/api/youyuan/accounts", map[string]any{"action": "profile", "id": "main", "name": "hijack"}); e == nil {
		t.Fatal("member changed main account")
	}
}
func TestPoolRelaySignatureExpiryAndChildOwner(t *testing.T) {
	yyPoolTestSetup(t)
	secret := yyNewKey()
	yyPoolRelayIdentity.Secret = secret
	sign := func(actor yyPoolActor) *http.Request {
		raw, _ := json.Marshal(actor)
		encoded := base64.RawURLEncoding.EncodeToString(raw)
		key, _ := hex.DecodeString(secret)
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(encoded))
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-YouYuan-Actor", encoded)
		r.Header.Set("X-YouYuan-Actor-Signature", hex.EncodeToString(mac.Sum(nil)))
		return r
	}
	actor := yyPoolActor{ID: "alice", Scope: "control", Expires: time.Now().Add(30 * time.Second).UnixMilli()}
	r := sign(actor)
	v, e := yyPoolActorRequest(r)
	if e != nil || v.ID != "alice" || v.Admin {
		t.Fatal(v, e)
	}
	r.Header.Set("X-YouYuan-Actor-Signature", strings.Repeat("0", 64))
	if _, e = yyPoolActorRequest(r); e == nil {
		t.Fatal("forged signature accepted")
	}
	actor.Expires = time.Now().Add(-time.Second).UnixMilli()
	if _, e = yyPoolActorRequest(sign(actor)); e == nil {
		t.Fatal("expired identity accepted")
	}
	a, _ := yyManagedFind("alice_bot")
	dir := t.TempDir()
	if e = yyPoolWriteChildIdentity(a, dir); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "pool-parent.json"))
	var child yyPoolChildIdentity
	_ = json.Unmarshal(raw, &child)
	r = httptest.NewRequest("POST", "/", nil)
	r.Header.Set("X-YouYuan-Pool-Source", child.Source)
	r.Header.Set("X-YouYuan-Pool-Secret", child.Secret)
	v, e = yyPoolActorRequest(r)
	if e != nil || v.ID != "alice" || v.Admin {
		t.Fatal("worker inherited administrator", v, e)
	}
	r.Header.Set("X-YouYuan-Pool-Source", "bob_bot")
	if _, e = yyPoolActorRequest(r); e == nil {
		t.Fatal("worker impersonated foreign account")
	}
}
