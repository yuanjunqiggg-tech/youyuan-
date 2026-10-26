package main

import (
	"errors"
	"sync"
	"time"

	"bot-apk/building"
)

var errMultiBotImportStopped = errors.New("导入已取消")

type yyMultiBotImportBudget struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func yyNewMultiBotImportBudget(speed int) *yyMultiBotImportBudget {
	if speed <= 0 {
		return nil
	}
	interval := time.Second / time.Duration(speed)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	return &yyMultiBotImportBudget{interval: interval}
}

func (b *yyMultiBotImportBudget) wait(stop <-chan struct{}) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case <-stop:
		return errMultiBotImportStopped
	default:
	}
	now := time.Now()
	// Keep timer oversleep in the next deadline, but never catch up a long stall.
	if b.next.IsZero() {
		b.next = now
		return nil
	}
	if b.next.Before(now.Add(-time.Millisecond)) {
		b.next = now
	}
	b.next = b.next.Add(b.interval)
	delay := time.Until(b.next)
	if b.interval < time.Millisecond && delay < time.Millisecond {
		return nil
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-stop:
			return errMultiBotImportStopped
		case <-timer.C:
		}
	}
	select {
	case <-stop:
		return errMultiBotImportStopped
	default:
	}
	return nil
}

func (b *yyMultiBotImportBudget) wrap(task *building.ImportTask, stop <-chan struct{}) {
	if b == nil || task == nil {
		return
	}
	setBlock, fillRegion := task.SetBlock, task.FillRegion
	if setBlock != nil {
		task.SetBlock = func(x, y, z int32, name, states string) error {
			if err := b.wait(stop); err != nil {
				return err
			}
			return setBlock(x, y, z, name, states)
		}
	}
	if fillRegion != nil {
		task.FillRegion = func(x1, y1, z1, x2, y2, z2 int32, name, states string) error {
			if err := b.wait(stop); err != nil {
				return err
			}
			return fillRegion(x1, y1, z1, x2, y2, z2, name, states)
		}
	}
	// The shared budget replaces ImportTask's per-worker positive-speed sleep.
	task.Speed = 0
}
