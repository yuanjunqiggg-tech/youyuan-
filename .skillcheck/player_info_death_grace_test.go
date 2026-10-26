package main

import (
	"testing"
	"time"
)

// A death makes the server drop the player from the online list and add them
// back on respawn. diffPlayerEvents diffs that set every 5s, so without a grace
// window the QQ bridge reports "离开服务器" then "进入服务器" instead of "死亡".
func TestDeathGraceWindowSuppressesJoinLeavePair(t *testing.T) {
	pm := NewPlayerInfoManager()

	if pm.RecentlyDied("Steve") {
		t.Fatal("a player who never died must not be treated as recently dead")
	}

	pm.MarkDeath("Steve")
	if !pm.RecentlyDied("Steve") {
		t.Fatal("death must open the grace window")
	}
	// Other players are unaffected by Steve's death.
	if pm.RecentlyDied("Alex") {
		t.Fatal("grace window leaked to an unrelated player")
	}

	// Expire the record by hand: the window is 20s, too long to sleep in a test.
	pm.mu.Lock()
	pm.deathAt["Steve"] = time.Now().Add(-yyDeathGraceWindow - time.Second)
	pm.mu.Unlock()
	if pm.RecentlyDied("Steve") {
		t.Fatal("grace window must close after the configured duration")
	}
}

func TestMarkDeathIgnoresEmptyNameAndPrunesOldEntries(t *testing.T) {
	pm := NewPlayerInfoManager()
	pm.MarkDeath("")
	if len(pm.deathAt) != 0 {
		t.Fatalf("empty name must not be recorded: %v", pm.deathAt)
	}

	pm.mu.Lock()
	pm.deathAt["Ancient"] = time.Now().Add(-10 * yyDeathGraceWindow)
	pm.mu.Unlock()
	pm.MarkDeath("Fresh")

	pm.mu.RLock()
	_, keptOld := pm.deathAt["Ancient"]
	_, keptNew := pm.deathAt["Fresh"]
	pm.mu.RUnlock()
	if keptOld {
		t.Fatal("stale death records must be pruned so the map cannot grow forever")
	}
	if !keptNew {
		t.Fatal("the new death record must survive pruning")
	}
}
