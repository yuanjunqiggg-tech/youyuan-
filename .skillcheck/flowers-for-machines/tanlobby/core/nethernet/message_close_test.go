package nethernet

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"
)

func TestHandleMessageUnblocksWhenConnectionCloses(t *testing.T) {
	c := &Conn{packets: make(chan []byte), closed: make(chan struct{}), message: &message{}}
	finished := make(chan error, 1)
	go func() { finished <- c.handleMessage([]byte{0, 'a'}) }()
	// A completed packet must wait for a reader rather than being discarded.
	select {
	case err := <-finished:
		t.Fatalf("delivery returned without a reader: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(c.closed) // First operation performed by Conn.Close, before transport shutdown.
	select {
	case err := <-finished:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("connection closure did not release the data-channel callback")
	}
}

func TestHandleMessageKeepsSegmentsAndPacketOrderWithSlowReader(t *testing.T) {
	c := &Conn{packets: make(chan []byte), closed: make(chan struct{}), message: &message{}}
	defer close(c.closed)
	finished := make(chan error, 1)
	go func() {
		for i := 0; i < 200; i++ {
			if err := c.handleMessage([]byte{1, byte(i)}); err != nil {
				finished <- err
				return
			}
			if err := c.handleMessage([]byte{0, byte(i + 1)}); err != nil {
				finished <- err
				return
			}
		}
		finished <- nil
	}()
	for i := 0; i < 200; i++ {
		select {
		case packet := <-c.packets:
			if !bytes.Equal(packet, []byte{byte(i), byte(i + 1)}) {
				t.Fatalf("packet %d out of order or truncated: %v", i, packet)
			}
		case <-time.After(time.Second):
			t.Fatalf("packet %d lost", i)
		}
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
