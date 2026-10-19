package main

import (
	"testing"
	"time"
)

func TestAccountIdentityDoesNotPollOfflineWorker(t *testing.T) {
	yyPoolTestSetup(t)
	start := time.Now()
	row := yyAccountIdentitySummary(yyManagedAccount{ID: "offline-profile", Name: "saved name", Token: "private-token", Slot: 32})
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("identity waited for worker")
	}
	if row["name"] != "saved name" || row["has_token"] != true || row["connected"] != nil || row["connection_known"] != false || row["telemetry_loaded"] != false {
		t.Fatal(row)
	}
	if _, ok := row["token"]; ok {
		t.Fatal("identity leaked credential")
	}
}
