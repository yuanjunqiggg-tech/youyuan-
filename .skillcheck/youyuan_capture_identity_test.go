package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A slot that points at a kernel which is no longer running used to poison
// yyCaptureRobotNamesWithStatus, because the code asked the dead kernel for
// bot.self and treated the failed call as "identity unknown". That cleared
// desired observers to zero and left every other robot offline for good.
//
// The guard still has to exist for the opposite case: an account that really
// is online but cannot report the name it plays under. Only that may hold back
// provisioning, because an unidentified online entity could be a real player.
func TestStaleAccountSlotDoesNotPoisonRobotIdentities(t *testing.T) {
	yyPrimaryFixture(t)
	// Slot 32 is past any account this fixture starts; the kernel is absent,
	// which is exactly the "stopped but slot not cleared" state in the field.
	yyAccounts.Items = []yyManagedAccount{{ID: "pool_stale", Name: "离线账号", Slot: 32}}
	t.Cleanup(func() { yyAccounts.Items = nil })

	names, known := yyCaptureRobotNamesWithStatus()
	if !known {
		t.Fatal("a stopped kernel must not report robot identities as unverifiable")
	}
	if len(names) == 0 || !containsString(names, "离线账号") {
		t.Fatal("the saved account name is still a valid robot exclusion", names)
	}
	stale, unverified := yyCaptureAuditLog.snapshot()
	if len(stale) != 1 || stale[0] != "pool_stale" {
		t.Fatal("the offline account should be recorded as a skipped slot", stale)
	}
	if len(unverified) != 0 {
		t.Fatal("nothing online went unverified", unverified)
	}
	summary := map[string]any{}
	summary["robot_identities_verified"] = known
	if !summary["robot_identities_verified"].(bool) {
		t.Fatal("reconcile summary must keep provisioning enabled")
	}
}

// Accounts that were never started carry no slot at all. They must stay
// invisible to the identity check instead of being reported as broken.
func TestSlotlessAccountIsNotAnIdentityFailure(t *testing.T) {
	yyPrimaryFixture(t)
	yyAccounts.Items = []yyManagedAccount{{ID: "pool_released", Name: "未启动账号", Slot: 0}}
	t.Cleanup(func() { yyAccounts.Items = nil })

	if _, known := yyCaptureRobotNamesWithStatus(); !known {
		t.Fatal("an account without a slot cannot fail identity verification")
	}
	stale, unverified := yyCaptureAuditLog.snapshot()
	if len(stale) != 0 || len(unverified) != 0 {
		t.Fatal("slotless accounts should not be audited at all", stale, unverified)
	}
}

// The diagnostic field the reconcile loop exposes has to be derived from the
// same snapshot, so an operator can tell a stale slot from an online account
// that refused to identify.
func TestIdentityAuditFieldsDistinguishOfflineFromUnverified(t *testing.T) {
	yyPrimaryFixture(t)
	yyAccounts.Items = []yyManagedAccount{
		{ID: "pool_offline", Name: "停用未清槽", Slot: 32},
	}
	t.Cleanup(func() { yyAccounts.Items = nil })
	_, _ = yyCaptureRobotNamesWithStatus()
	stale, unverified := yyCaptureAuditLog.snapshot()
	if len(stale) != 1 || stale[0] != "pool_offline" {
		t.Fatal("offline account missing from audit", stale)
	}
	if len(unverified) != 0 {
		t.Fatal("offline account must not be listed as unverified online", unverified)
	}
	rendered := strings.Join(stale, ",") + "|" + strings.Join(unverified, ",")
	if rendered != "pool_offline|" {
		t.Fatal("audit rendering must stay addressable by account id", rendered)
	}
}

func containsString(items []string, want string) bool {
	for _, v := range items {
		if v == want {
			return true
		}
	}
	return false
}

// The on-disk state repaired at load time. Without this, a disabled account
// that kept its slot keeps describing a process that no longer exists, and any
// future re-check of that slot can poison the capture roster again.
func TestDisabledAccountSlotIsHealedOnLoad(t *testing.T) {
	yyPrimaryFixture(t)
	dir := t.TempDir()
	raw := `[{"id":"pool_stale","name":"停用未清槽","enabled":false,"slot":1},{"id":"pool_live","name":"在用账号","enabled":true,"slot":2}]`
	if e := os.WriteFile(filepath.Join(dir, "managed-accounts.json"), []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	previousRoot, previousWorker, previousItems := yyAccounts.Root, yyAccounts.Worker, yyAccounts.Items
	t.Cleanup(func() {
		yyAccounts.Root, yyAccounts.Worker, yyAccounts.Items = previousRoot, previousWorker, previousItems
	})

	yyAccountsRoutes(http.NewServeMux(), dir)

	byID := map[string]yyManagedAccount{}
	for _, a := range yyAccounts.Items {
		byID[a.ID] = a
	}
	if got := byID["pool_stale"].Slot; got != 0 {
		t.Fatal("a disabled account must not keep a slot on disk", got)
	}
	if got := byID["pool_live"].Slot; got != 2 {
		t.Fatal("an enabled account must keep its slot", got)
	}
	saved, e := os.ReadFile(filepath.Join(dir, "managed-accounts.json"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(saved), "\"slot\": 1") {
		t.Fatal("healed state must be written back", string(saved))
	}
}
