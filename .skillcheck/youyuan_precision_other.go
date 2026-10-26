//go:build !linux && !android

package main

import (
	"time"
)

// Keep portable platforms cooperative; measured Linux rates do not imply
// matching Windows timer precision. No artificial interval floor is applied.
func yyPrecisionWait(wait time.Duration) {
	if wait > 0 {
		time.Sleep(wait)
	}
}
