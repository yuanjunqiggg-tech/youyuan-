package main

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"bot-apk/building"
)

func TestMultiBotImportBudgetSharesRateAcrossWorkers(t *testing.T) {
	const speed, perWorker = 40, 6
	budget := yyNewMultiBotImportBudget(speed)
	var calls atomic.Int32
	start := time.Now()
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for worker := 0; worker < 2; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task := &building.ImportTask{Speed: speed, SetBlock: func(int32, int32, int32, string, string) error {
				calls.Add(1)
				return nil
			}}
			budget.wrap(task, nil)
			if task.Speed != 0 {
				errCh <- errors.New("per-worker speed delay was not disabled")
				return
			}
			for i := 0; i < perWorker; i++ {
				if err := task.SetBlock(0, 0, 0, "stone", "[]"); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if got := calls.Load(); got != 2*perWorker {
		t.Fatalf("submitted calls=%d, want %d", got, 2*perWorker)
	}
	if elapsed < 220*time.Millisecond {
		t.Fatalf("shared budget completed %d calls too quickly: %v", 2*perWorker, elapsed)
	}
}

func TestMultiBotImportBudgetStopCancelsWaiter(t *testing.T) {
	budget := yyNewMultiBotImportBudget(1)
	stop := make(chan struct{})
	calls := 0
	task := &building.ImportTask{Speed: 1, SetBlock: func(int32, int32, int32, string, string) error {
		calls++
		return nil
	}}
	budget.wrap(task, stop)
	if err := task.SetBlock(0, 0, 0, "stone", "[]"); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- task.SetBlock(1, 0, 0, "stone", "[]") }()
	time.Sleep(10 * time.Millisecond)
	close(stop)
	select {
	case err := <-result:
		if !errors.Is(err, errMultiBotImportStopped) {
			t.Fatalf("wait error=%v, want cancellation", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("stop did not cancel the shared pacing wait promptly")
	}
	if calls != 1 {
		t.Fatalf("callback calls=%d after stop, want only first call", calls)
	}
}

func TestMultiBotImportBudgetAccumulatesSubMillisecondIntervals(t *testing.T) {
	budget := yyNewMultiBotImportBudget(10000)
	for i := 0; i < 50; i++ {
		if err := budget.wait(nil); err != nil {
			t.Fatal(err)
		}
	}
	if budget.next.IsZero() {
		t.Fatal("shared absolute deadline lost")
	}
	// A long host pause must not produce an unbounded catch-up burst.
	budget.next = time.Now().Add(-time.Hour)
	if err := budget.wait(nil); err != nil {
		t.Fatal(err)
	}
	if time.Since(budget.next) > time.Millisecond {
		t.Fatal("old deadline retained an hour of send debt")
	}
}

func TestMultiBotImportBudgetUnlimitedAndErrors(t *testing.T) {
	for _, speed := range []int{-1, 0} {
		budget := yyNewMultiBotImportBudget(speed)
		calls := 0
		task := &building.ImportTask{Speed: speed, SetBlock: func(int32, int32, int32, string, string) error {
			calls++
			return nil
		}}
		budget.wrap(task, nil)
		start := time.Now()
		for i := 0; i < 100; i++ {
			if err := task.SetBlock(0, 0, 0, "stone", "[]"); err != nil {
				t.Fatal(err)
			}
		}
		if calls != 100 || task.Speed != speed || time.Since(start) >= 100*time.Millisecond {
			t.Fatalf("speed=%d calls=%d retained_speed=%d elapsed=%v", speed, calls, task.Speed, time.Since(start))
		}
	}

	want := errors.New("transport failed")
	task := &building.ImportTask{
		Speed:      1000,
		SetBlock:   func(int32, int32, int32, string, string) error { return want },
		FillRegion: func(int32, int32, int32, int32, int32, int32, string, string) error { return want },
	}
	yyNewMultiBotImportBudget(1000).wrap(task, nil)
	if err := task.SetBlock(0, 0, 0, "stone", "[]"); !errors.Is(err, want) {
		t.Fatalf("SetBlock error=%v, want original transport error", err)
	}
	if err := task.FillRegion(0, 0, 0, 1, 1, 1, "stone", "[]"); !errors.Is(err, want) {
		t.Fatalf("FillRegion error=%v, want original transport error", err)
	}
}
