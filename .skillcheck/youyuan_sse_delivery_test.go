package main

import "testing"

func TestTerminalTaskEventSurvivesQueuedProgress(t *testing.T) {
	c := make(chan string, 3)
	for range 3 {
		c <- "progress"
	}
	yyEnqueueSSE(c, "done", true)
	found := false
	for range 3 {
		found = <-c == "done" || found
	}
	if !found {
		t.Fatal("completion dropped behind progress")
	}
	u := make(chan string)
	yyEnqueueSSE(u, "done", true) // Never block a game/task thread for a stalled UI.
}
