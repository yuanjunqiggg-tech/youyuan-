package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// Capture must never block the packet-consumer goroutine. A full queue is a
// counted drop, not a stall that backs up into the RakNet receive window.
func TestCaptureEnqueueIsBoundedAndCountsDrops(t *testing.T) {
	bm := &BotManager{}
	s := yySenseFor(bm)
	s.Lock()
	s.Capture = yyCaptureState{
		Active:  true,
		Config:  yyCaptureConfig{Types: []string{"SetHealth"}, Capacity: 4, MaxBytes: 1024, TotalBytes: 1 << 20},
		Last:    map[string]time.Time{},
		pending: make(chan packet.Packet, 2),
		done:    make(chan struct{}),
		gen:     1,
	}
	s.Unlock()
	defer func() {
		s.Lock()
		if s.Capture.done != nil {
			close(s.Capture.done)
			s.Capture.done = nil
		}
		s.Capture.Active = false
		s.Unlock()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 6; i++ {
			yyCapturePacket(bm, &packet.SetHealth{Health: 20})
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("capture enqueue blocked the caller")
	}

	s.Lock()
	dropped, queued := s.Capture.QueueDropped, len(s.Capture.pending)
	s.Unlock()
	if queued != 2 {
		t.Fatalf("queue should hold exactly its capacity, got %d", queued)
	}
	if dropped != 4 {
		t.Fatalf("every rejected packet must be counted, got %d", dropped)
	}
}

func TestCaptureWorkerStoresWhatWasAccepted(t *testing.T) {
	bm := &BotManager{}
	if _, e := yyCaptureStart(bm, yyCaptureConfig{Types: []string{"SetHealth"}, Capacity: 8}); e != nil {
		t.Fatal(e)
	}
	defer func() {
		s := yySenseFor(bm)
		s.Lock()
		s.Capture.Active = false
		if s.Capture.done != nil {
			close(s.Capture.done)
			s.Capture.done = nil
		}
		s.Unlock()
	}()
	yyCapturePacket(bm, &packet.SetHealth{Health: 7})
	read := yyCaptureRead(bm, 0, 8)
	if read["retained_count"].(int) != 1 {
		t.Fatalf("accepted packet was not stored: %v", read["retained_count"])
	}
	if read["queue_dropped"].(uint64) != 0 {
		t.Fatalf("unexpected drops: %v", read["queue_dropped"])
	}
}

// A skipped target must stay visible in the watch status instead of silently
// ending observation.
func TestWatchStatusExposesSkipCounters(t *testing.T) {
	w := &yyWatch{Bot: &BotManager{}, Config: yyWatchConfig{Mode: "camera"}, DimensionSkips: 3, QueryFailures: 2, LastSkip: "相机不能跨维度"}
	snap := w.snapshot()
	if snap["dimension_skips"] != uint64(3) || snap["query_failures"] != uint64(2) {
		t.Fatal(snap)
	}
	if snap["last_skip"] != "相机不能跨维度" {
		t.Fatal(snap["last_skip"])
	}
}

// Camera-only observation still needs the body inside the broadcast radius;
// the server-side distance test keeps that cheap and never moves the target.
func TestWatchBodyCommandOnlyMovesTheObserver(t *testing.T) {
	command := yyWatchBodyCommand("observer", "player", 64)
	if !strings.Contains(command, "distance=64..") || !strings.Contains(command, "tp @s ~ ~ ~") {
		t.Fatal(command)
	}
	if strings.Contains(command, "player as") {
		t.Fatal("target must never be the teleport subject")
	}
	if yyWatchBodyCommand("observer", "player", 0) != "" {
		t.Fatal("radius 0 must disable body proximity")
	}
	if yyWatchBodyCommand("bad\nname", "player", 64) != "" {
		t.Fatal("unsafe name accepted")
	}
	// A fractional radius must not round down to "distance=0..", which would
	// match at any distance and teleport on every tick.
	if got := yyWatchBodyCommand("observer", "player", 0.5); !strings.Contains(got, "distance=1..") {
		t.Fatalf("fractional radius must clamp to 1, got %q", got)
	}
}

// Stopping capture must also retire the published matcher, otherwise the
// ingress filter keeps selecting packets that can never be stored.
func TestCaptureStopRetiresTheIngressMatcher(t *testing.T) {
	bm := &BotManager{}
	if _, e := yyCaptureStart(bm, yyCaptureConfig{Types: []string{"SetHealth"}, Capacity: 4}); e != nil {
		t.Fatal(e)
	}
	if !yyCaptureWants(bm, "SetHealth") {
		t.Fatal("active capture must select its configured type")
	}
	if yyCaptureWants(bm, "SetTime") {
		t.Fatal("capture must not select unconfigured types")
	}
	yyCaptureStop(bm)
	if yyCaptureWants(bm, "SetHealth") {
		t.Fatal("stopped capture must stop selecting packets")
	}
	if yyCaptureStatus(bm)["active"] != false {
		t.Fatal("stop must report inactive")
	}
}

// Every stop route has to share the same shutdown path.
func TestSenseCaptureStopRouteRetiresTheMatcher(t *testing.T) {
	bm := &BotManager{}
	if _, e := yyCaptureStart(bm, yyCaptureConfig{Types: []string{"SetHealth"}, Capacity: 4}); e != nil {
		t.Fatal(e)
	}
	if _, e := yySenseCall(bm, "events.captureStop", nil); e != nil {
		t.Fatal(e)
	}
	if yyCaptureWants(bm, "SetHealth") {
		t.Fatal("events.captureStop left the matcher published")
	}
}

// A global-capture.json written before body_radius_blocks existed must keep the
// safe default, otherwise upgrading silently disables entity packet reception.
func TestGlobalCaptureBodyRadiusKeepsDefaultForOldConfigs(t *testing.T) {
	if got := yyGlobalWatchOptions(yyGlobalCaptureConfig{})["body_radius_blocks"]; got != float64(64) {
		t.Fatalf("absent field must default to 64, got %v", got)
	}
	zero, twenty := float64(0), float64(20)
	if got := yyGlobalWatchOptions(yyGlobalCaptureConfig{BodyRadius: &zero})["body_radius_blocks"]; got != float64(0) {
		t.Fatalf("explicit 0 must disable proximity, got %v", got)
	}
	if got := yyGlobalWatchOptions(yyGlobalCaptureConfig{BodyRadius: &twenty})["body_radius_blocks"]; got != float64(20) {
		t.Fatalf("explicit radius lost, got %v", got)
	}
	var saved yyGlobalCaptureConfig
	if e := json.Unmarshal([]byte(`{"mode":"camera"}`), &saved); e != nil || saved.BodyRadius != nil {
		t.Fatalf("old config must decode to nil, got %v %v", saved.BodyRadius, e)
	}
	if got := yyGlobalBodyRadius(saved); got != 64 {
		t.Fatalf("old config must resolve to 64, got %v", got)
	}
}

func TestWatchParseBodyRadiusDefaultAndBounds(t *testing.T) {
	c, err := yyWatchParse(map[string]any{"player": "p"})
	if err != nil || c.BodyRadius != 64 {
		t.Fatalf("default body radius lost: %v %v", c.BodyRadius, err)
	}
	if _, err = yyWatchParse(map[string]any{"player": "p", "body_radius_blocks": -1}); err == nil {
		t.Fatal("negative body radius accepted")
	}
	if c, err = yyWatchParse(map[string]any{"player": "p", "body_radius_blocks": 0}); err != nil || c.BodyRadius != 0 {
		t.Fatal("explicit 0 must disable proximity")
	}
}
