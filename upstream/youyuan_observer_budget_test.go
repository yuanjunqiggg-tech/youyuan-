package main

import (
	"testing"
	"time"
)

func TestObservationSendUsesSharedBudgetAndHonorsJobPause(t *testing.T) {
	bm := &BotManager{connected: true, stopCh: make(chan struct{})}
	defer func() { yySpatialJobs.Lock(); delete(yySpatialJobs.Items, bm); yySpatialJobs.Unlock() }()
	start := time.Now()
	for i := 0; i < 5; i++ {
		sent, err := yyObserverSend(bm, "camera @s clear", false)
		if !sent || err == nil {
			t.Fatal("expected disconnected transport after budget reservation", sent, err)
		}
	}
	if time.Since(start) < 190*time.Millisecond || bm.controlCommands.Load() != 5 {
		t.Fatal("observation bypassed shared 20-command/s budget")
	}
	next := bm.controlCommandNext
	count := bm.controlCommands.Load()
	done := yyBeginSpatialJob(bm, "import")
	sent, err := yyObserverSend(bm, "camera @s clear", false)
	if sent || err != nil || bm.controlCommands.Load() != count || !bm.controlCommandNext.Equal(next) {
		t.Fatal("paused observation consumed a send slot")
	}
	done()
	close(bm.stopCh)
	if _, err := yyObserverSend(bm, "camera @s clear", false); err == nil {
		t.Fatal("stopped connection accepted")
	}
}
