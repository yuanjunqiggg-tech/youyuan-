package minecraft

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

type coalescedWriterFunc func([]byte) (int, error)

func (f coalescedWriterFunc) Write(p []byte) (int, error) { return f(p) }

func TestCoalescedLongModuleLetsControlWriteBetweenTransportBatches(t *testing.T) {
	c := coalescedFixture()
	var out bytes.Buffer
	var once sync.Once
	var submitted, controlAt atomic.Int32
	packets := coalescedPackets(1024)
	controlAt.Store(-1)
	done := make(chan error, 1)
	c.SetOutboundPacketObserver(func(p packet.Packet) {
		if command, ok := p.(*packet.SettingsCommand); ok && command.CommandLine == "say control" {
			controlAt.Store(submitted.Load())
		} else {
			index := submitted.Add(1) - 1
			if p != packets[index] {
				t.Errorf("module order changed at %d", index)
			}
		}
	})
	c.enc = packet.NewEncoder(coalescedWriterFunc(func(data []byte) (int, error) {
		once.Do(func() {
			waiting := make(chan struct{})
			go func() {
				close(waiting)
				done <- c.WritePacket(&packet.SettingsCommand{CommandLine: "say control", SuppressOutput: true})
			}()
			<-waiting
			// Keep the first local flush occupied until the control writer is waiting.
			time.Sleep(3 * time.Millisecond)
		})
		return out.Write(data)
	}))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := c.WritePacketsBounded(ctx, packets); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("control writer starved")
	}
	if n := controlAt.Load(); n < 256 || n >= 1024 {
		t.Fatalf("control writer waited for the entire module: observed %d of 1024 commands", n)
	}
	if submitted.Load() != 1024 {
		t.Fatal("module commands were lost", submitted.Load())
	}
	t.Logf("control call entered after %d of 1024 commands", controlAt.Load())
}

func TestCoalescedBatchHandoffStopsWithoutReplay(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "encode_error"}[fail], func(t *testing.T) {
			c := coalescedFixture()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("local transport failure")
			seen, writes := 0, 0
			c.SetOutboundPacketObserver(func(packet.Packet) { seen++ })
			c.enc = packet.NewEncoder(coalescedWriterFunc(func(data []byte) (int, error) {
				writes++
				if fail {
					return 0, sentinel
				}
				cancel()
				return len(data), nil
			}))
			attempted, err := c.WritePacketsBounded(ctx, coalescedPackets(1024))
			want := error(context.Canceled)
			if fail {
				want = sentinel
			}
			if !attempted || !errors.Is(err, want) || seen != 256 || writes != 1 || len(c.bufferedSend) != 0 {
				t.Fatalf("partial batch replayed/continued: attempted=%v err=%v packets=%d writes=%d buffered=%d", attempted, err, seen, writes, len(c.bufferedSend))
			}
			if !c.sendMu.TryLock() {
				t.Fatal("send lock leaked")
			}
			c.sendMu.Unlock()
		})
	}
}

func TestCoalescedLargeModuleRetainsWireBytes(t *testing.T) {
	a, b := coalescedFixture(), coalescedFixture()
	var oldWire, newWire bytes.Buffer
	a.enc, b.enc = packet.NewEncoder(&oldWire), packet.NewEncoder(&newWire)
	packets := coalescedPackets(1025)
	for _, p := range packets {
		if err := a.WritePacketBounded(p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.WritePacketsBounded(context.Background(), packets); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(oldWire.Bytes(), newWire.Bytes()) || len(a.bufferedSend) != len(b.bufferedSend) || !bytes.Equal(a.bufferedSend[0], b.bufferedSend[0]) {
		t.Fatal("batch handoff changed wire bytes, order or tick buffer")
	}
}
