package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCancelledLoginDoesNotContactAuthService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	c, err := LoginRentalServer(Config{LoginContext: ctx, AuthServerAddress: "http://192.0.2.1:1"})
	if c != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(c, err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancelled login waited for network")
	}
}
