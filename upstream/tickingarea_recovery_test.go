package main

import (
	"errors"
	"testing"

	"bot-apk/state"
	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestTickingAreaReceiptActualAndLegacy(t *testing.T) {
	for _, text := range []string{
		"%dimension.dimensionName0: \n- yy_test: 10288 0 10928 %commands.tickingarea-list.to 10383 0 11023 %commands.tickingarea-list.preload",
		"%dimension.dimensionName0: \n- yy_test: (10288, 10928) to (10383, 11023)",
	} {
		v, err := yyParseTickingAreas(&packet.CommandOutput{SuccessCount: 1, OutputMessages: []protocol.CommandOutputMessage{{Success: true, Message: text}}})
		if err != nil || len(v) != 1 || v["yy_test"].EndZ != 11023 || v["yy_test"].Dimension != "0" {
			t.Fatal(v, err)
		}
	}
	for _, text := range []string{"", "permission denied", "%dimension.dimensionName0:\n- yy_test: malformed"} {
		if _, err := yyParseTickingAreas(&packet.CommandOutput{OutputMessages: []protocol.CommandOutputMessage{{Message: text}}}); err == nil {
			t.Fatalf("unrecognised receipt treated as empty: %q", text)
		}
	}
	v, err := yyParseTickingAreas(&packet.CommandOutput{OutputMessages: []protocol.CommandOutputMessage{{Message: "commands.tickingarea-list.none"}}})
	if err != nil || len(v) != 0 {
		t.Fatal(v, err)
	}
	if cmd := yyTickingCommand("tickingarea remove yy_test", "overworld"); cmd != "execute in overworld run tickingarea remove yy_test" {
		t.Fatal("cleanup could follow executor into another dimension", cmd)
	}
}

func TestTickingAreaOwnershipSurvivesRestartAndSkipsLiveWork(t *testing.T) {
	dir := t.TempDir()
	if err := state.InitCheckpointDB(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { state.CloseDB() })
	old := state.TickingAreaLease{Server: "A", Dimension: "overworld", Name: "old"}
	active := state.TickingAreaLease{Server: "A", Dimension: "overworld", Name: "active"}
	otherDim := state.TickingAreaLease{Server: "A", Dimension: "the_nether", Name: "nether"}
	otherServer := state.TickingAreaLease{Server: "B", Dimension: "overworld", Name: "foreign"}
	for _, v := range []state.TickingAreaLease{old, active, otherDim, otherServer} {
		if err := state.RememberTickingArea(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := state.InitCheckpointDB(dir); err != nil {
		t.Fatal(err)
	}
	yyActiveTickingAreas.Lock()
	yyActiveTickingAreas.leases[active] = true
	yyActiveTickingAreas.Unlock()
	t.Cleanup(func() {
		yyActiveTickingAreas.Lock()
		delete(yyActiveTickingAreas.leases, active)
		yyActiveTickingAreas.Unlock()
	})
	var removed []string
	remove := func(v state.TickingAreaLease) error {
		removed = append(removed, v.Name)
		return errors.New("connection lost after command submission")
	}
	if err := yyRecoverTickingAreas("A", "overworld", remove); err == nil {
		t.Fatal("failed cleanup discarded ownership")
	}
	if v, _ := state.ListTickingAreas("A"); len(v) != 3 {
		t.Fatal("failed deletion lost record", v)
	}
	err := yyRecoverTickingAreas("A", "overworld", func(v state.TickingAreaLease) error {
		if v != old {
			t.Fatalf("removed live, foreign, or other-dimension area: %+v", v)
		}
		return nil
	})
	if err != nil || len(removed) != 1 || removed[0] != "old" {
		t.Fatal(err, removed)
	}
	if v, _ := state.ListTickingAreas("A"); len(v) != 2 {
		t.Fatal(v)
	}
	if v, _ := state.ListTickingAreas("B"); len(v) != 1 {
		t.Fatal(v)
	}
}

func TestTickingAreaFailedSubmissionRetainsOwnershipAndDoesNotUseReplacementConnection(t *testing.T) {
	if err := state.InitCheckpointDB(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { state.CloseDB() })
	bm := &BotManager{conn: &minecraft.Conn{}, connected: true, serverCode: "original", dimension: 1}
	preload, _, cleanup := yyTickingCallbacks(bm, "yy_test_", true)
	// No game interface: submission fails locally without contacting any server.
	if err := preload("region", 0, 0, 15, 15); err == nil {
		t.Fatal("missing interface reported success")
	}
	v, err := state.ListTickingAreas("original")
	if err != nil || len(v) != 1 || v[0].Dimension != "the_nether" {
		t.Fatal("failed submission discarded ownership", v, err)
	}
	bm.mu.Lock()
	bm.conn = &minecraft.Conn{}
	bm.serverCode = "replacement"
	bm.mu.Unlock()
	cleanup()
	if v, _ := state.ListTickingAreas("original"); len(v) != 1 {
		t.Fatal("cleanup discarded record after connection replacement", v)
	}
	if err := yyRecoverTickingAreas("original", "the_nether", func(state.TickingAreaLease) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if v, _ := state.ListTickingAreas("original"); len(v) != 0 {
		t.Fatal("ended job still marked active", v)
	}
}

func TestTickingCleanupOnlyForgetsConfirmedAbsentOwnedNames(t *testing.T) {
	if err := state.InitCheckpointDB(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { state.CloseDB() })
	for _, name := range []string{"gone", "present", "foreign"} {
		if err := state.RememberTickingArea(state.TickingAreaLease{Server: "A", Dimension: "the_end", Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	if err := yyForgetAbsentTickingAreas("A", "the_end", []string{"gone", "present"}, map[string]*TickingAreaInfo{"present": {Dimension: "2"}}); err != nil {
		t.Fatal(err)
	}
	remaining, err := state.ListTickingAreas("A")
	if err != nil || len(remaining) != 2 {
		t.Fatal(remaining, err)
	}
	for _, area := range remaining {
		if area.Name == "gone" {
			t.Fatal("confirmed absence was not released")
		}
	}
	if got := yyTickingAddCommand("own", 15, 31, 0, 16, "the_end"); got != "execute in the_end run tickingarea add 0 0 16 15 255 31 own true" {
		t.Fatal(got)
	}
}
