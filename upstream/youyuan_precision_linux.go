//go:build linux || android

package main

import (
	"golang.org/x/sys/unix"
	"time"
)

// Linux hrtimer sleep avoids a busy polling loop for sub-millisecond periods.
// A new registration/cancellation may wait at most this short sleep (<=1ms).
func yyPrecisionWait(wait time.Duration) {
	if wait <= 0 {
		return
	}
	req := unix.NsecToTimespec(int64(wait))
	var remaining unix.Timespec
	for unix.Nanosleep(&req, &remaining) == unix.EINTR {
		req = remaining
	}
}
