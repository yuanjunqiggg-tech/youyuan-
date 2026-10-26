package minecraft

import (
	"context"
	"encoding/json"
	"io"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestLocalTenThousandCommandsAtTwentyHz(t *testing.T) {
	testLocalModuleLoad(t, 10000, 40, 20)
}

func TestLocalOriginalScale170CommandsAtEightyHz(t *testing.T) {
	testLocalModuleLoad(t, 170, 160, 80)
}

func testLocalModuleLoad(t *testing.T, commands, rounds, hz int) {
	t.Helper()
	period := time.Second / time.Duration(hz)
	c := coalescedFixture()
	c.enc = packet.NewEncoder(io.Discard)
	c.enc.EnableCompression(packet.CompressionAlgorithmNetEase)
	c.enc.EnableEncryption([32]byte{1, 2, 3})
	packets := coalescedPackets(commands)
	var observed, queries, maxQueryNS atomic.Int64
	c.SetOutboundPacketObserver(func(packet.Packet) { observed.Add(1) })
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				done <- nil
				return
			case <-tick.C:
				start := time.Now()
				_, err := c.WritePacketsBounded(ctx, packets[:1])
				if err != nil {
					done <- err
					return
				}
				queries.Add(1)
				ns := time.Since(start).Nanoseconds()
				for old := maxQueryNS.Load(); ns > old && !maxQueryNS.CompareAndSwap(old, ns); old = maxQueryNS.Load() {
				}
			}
		}
	}()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	late := 0
	maxRound := time.Duration(0)
	for i := 0; i < rounds; i++ {
		round := time.Now()
		if attempted, err := c.WritePacketsBounded(ctx, packets); !attempted || err != nil {
			cancel()
			<-done
			t.Fatal("local module failed", i, attempted, err)
		}
		elapsed := time.Since(round)
		if elapsed > maxRound {
			maxRound = elapsed
		}
		if elapsed > period {
			late++
		} else {
			time.Sleep(period - elapsed)
		}
	}
	elapsed := time.Since(start)
	cancel()
	if err := <-done; err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	if observed.Load() != int64(rounds*commands)+queries.Load() || queries.Load() == 0 {
		t.Fatal("local submission lost, duplicated, or starved", observed.Load(), queries.Load())
	}
	report, _ := json.Marshal(map[string]any{
		"scope":              "local encode/compress/encrypt only; not network or game execution",
		"commands_per_round": commands, "rounds": rounds, "target_hz": hz,
		"actual_hz": float64(rounds) / elapsed.Seconds(), "late_rounds": late,
		"max_round_ms":      float64(maxRound) / float64(time.Millisecond),
		"interleaved_calls": queries.Load(), "max_interleaved_wait_ms": float64(maxQueryNS.Load()) / 1e6,
		"allocated_bytes": after.TotalAlloc - before.TotalAlloc, "gc_count": after.NumGC - before.NumGC,
		"observed_commands": observed.Load(),
	})
	t.Log(string(report))
}
