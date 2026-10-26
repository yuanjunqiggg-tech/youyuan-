package main

import (
	"errors"
	"testing"
)

func TestTickingMutationReceiptLossUsesReadbackWithoutReplaying(t *testing.T) {
	timeout := errors.New("reply lost")
	for _, tc := range []struct {
		ok        bool
		writeErr  error
		confirmed bool
		readErr   error
		pass      bool
	}{
		{true, nil, false, nil, true}, {false, timeout, true, nil, true}, {false, nil, true, nil, true},
		{false, timeout, false, nil, false}, {false, nil, false, nil, false}, {false, timeout, false, timeout, false},
	} {
		writes, reads := 0, 0
		e := yyConfirmTickingChange(func() (bool, error) { writes++; return tc.ok, tc.writeErr }, func() (bool, error) { reads++; return tc.confirmed, tc.readErr })
		if (e == nil) != tc.pass || writes != 1 || (tc.ok && tc.writeErr == nil && reads != 0) {
			t.Fatal(tc, e, writes, reads)
		}
		if !tc.pass && tc.writeErr != nil && !errors.Is(e, timeout) {
			t.Fatal("lost original failure", e)
		}
	}
}

func TestTickingAreaReadbackRequiresDimensionAndAllRequestedChunks(t *testing.T) {
	a := &TickingAreaInfo{Dimension: "1", StartX: -32, StartZ: 0, EndX: -1, EndZ: 31}
	if !yyTickingAreaCovers(a, "the_nether", -31, 1, -2, 30) {
		t.Fatal("aligned area rejected")
	}
	for _, tc := range []struct {
		dim            string
		x1, z1, x2, z2 int
	}{
		{"overworld", -31, 1, -2, 30}, {"the_nether", -33, 1, -2, 30}, {"nether", -31, 1, 0, 30}, {"1", -31, 1, -2, 32},
	} {
		if yyTickingAreaCovers(a, tc.dim, tc.x1, tc.z1, tc.x2, tc.z2) {
			t.Fatal("wrong dimension/partial cover accepted", tc)
		}
	}
	if yyTickingAreaCovers(nil, "1", 0, 0, 1, 1) {
		t.Fatal("missing area accepted")
	}
}
