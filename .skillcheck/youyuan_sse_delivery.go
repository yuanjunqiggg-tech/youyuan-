package main

// Keep terminal task state deliverable when a slow client has queued progress.
func yyEnqueueSSE(ch chan string, msg string, terminal bool) {
	select {
	case ch <- msg:
		return
	default:
	}
	if !terminal || cap(ch) == 0 {
		return
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- msg:
	default:
	}
}
