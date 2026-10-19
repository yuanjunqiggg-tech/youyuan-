package main

import "sync/atomic"

var yyDisplayLogSubmitted atomic.Uint64
var yyDisplayLogDropped atomic.Uint64

// Display logs never block game work. Game packets, command results and event
// delivery do not use this queue and retain their existing delivery semantics.
func yyReceiveLog(ch chan string, message string) {
	yyDisplayLogSubmitted.Add(1)
	select {
	case ch <- message:
	default:
		yyDisplayLogDropped.Add(1)
	}
}

func yyDisplayLogStats() map[string]any {
	return map[string]any{"scope": "host display logs only; not game packets or plugin events", "submitted": yyDisplayLogSubmitted.Load(), "dropped": yyDisplayLogDropped.Load(), "policy": "nonblocking_display"}
}
