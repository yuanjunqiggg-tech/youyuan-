package main

import "testing"

func TestMusicNoteName(t *testing.T) {
	if musicNoteName(1.0) != "C" {
		t.Fatalf("pitch 1.0 should be C (MIDI 60), got %q", musicNoteName(1.0))
	}
}

func TestEmitForwardsToFloatingHub(t *testing.T) {
	h := NewSSEHub()
	fh := NewSSEHub()
	h.secondary = fh

	got := make(chan string, 4)
	fh.Add(got)

	h.Emit("music_progress", `{"current":1}`)

	select {
	case msg := <-got:
		if msg != "event: music_progress\ndata: {\"current\":1}\n\n" {
			t.Fatalf("unexpected forwarded msg: %q", msg)
		}
	default:
		t.Fatal("floatingHub did not receive forwarded message")
	}
}
