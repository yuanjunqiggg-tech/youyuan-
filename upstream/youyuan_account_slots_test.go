package main

import (
	"fmt"
	"testing"
)

func TestAccountProfilesAcquireSlotsOnlyForRunningProcesses(t *testing.T) {
	yyAccounts.Lock()
	defer yyAccounts.Unlock()
	previous := yyAccounts.Items
	defer func() { yyAccounts.Items = previous }()
	yyAccounts.Items = make([]yyManagedAccount, 80)
	for i := range yyAccounts.Items {
		yyAccounts.Items[i].ID = fmt.Sprintf("profile_%d", i)
	}
	for i := 0; i < yyAccountProcessSlots; i++ {
		if e := yyAccountAllocateLocked(&yyAccounts.Items[i]); e != nil {
			t.Fatal(e)
		}
		if yyAccounts.Items[i].Slot != i+1 {
			t.Fatal("slot collision")
		}
	}
	if yyAccounts.Items[60].Slot != 0 {
		t.Fatal("stored profile consumed a process slot")
	}
	if yyAccountAllocateLocked(&yyAccounts.Items[32]) == nil {
		t.Fatal("running limit ignored")
	}
	yyAccounts.Items[4].Slot = 0
	if e := yyAccountAllocateLocked(&yyAccounts.Items[60]); e != nil || yyAccounts.Items[60].Slot != 5 {
		t.Fatal("released slot not reusable", e)
	}
}
