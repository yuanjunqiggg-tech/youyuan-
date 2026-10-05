package main

import (
	"sync"
	"time"
)

// One clock drives all plugin timers. Each job has at most one outstanding
// callback, including time spent waiting in its resident runtime's event queue.
// These are host ticks; a remote Minecraft server does not share this clock.
var yyTickOnce sync.Once
var yyTickWake = make(chan struct{}, 1)

func yyWakeTickLoop() {
	yyTickOnce.Do(func() { go yyTickLoop() })
	select {
	case yyTickWake <- struct{}{}:
	default:
	}
}

func yyTickLoop() {
	for {
		now := time.Now()
		var due, exiting []*schedJob
		wait := time.Hour
		schedMu.Lock()
		for id, j := range schedJobs {
			select {
			case <-j.cancel:
				if !j.running {
					delete(schedJobs, id)
					exiting = append(exiting, j)
				}
				continue
			default:
			}
			if j.finished {
				if !j.running {
					delete(schedJobs, id)
					exiting = append(exiting, j)
				}
				continue
			}
			if j.nextDue.IsZero() {
				j.nextDue = now.Add(j.interval)
			}
			if !j.nextDue.After(now) {
				if j.oneshot {
					if !j.paused.Load() {
						j.finished = true
						j.running = true
						due = append(due, j)
					}
					j.nextDue = now.Add(time.Millisecond)
				} else {
					periods := uint64(now.Sub(j.nextDue)/j.interval) + 1
					j.nextDue = j.nextDue.Add(time.Duration(periods) * j.interval)
					if j.running || j.paused.Load() {
						j.missed.Add(periods)
					} else {
						j.missed.Add(periods - 1)
						j.running = true
						due = append(due, j)
					}
				}
			}
			if d := j.nextDue.Sub(now); d < wait {
				wait = d
			}
		}
		schedMu.Unlock()
		for _, j := range exiting {
			yySchedExit(j)
		}
		for _, j := range due {
			go func(j *schedJob) {
				defer func() { schedMu.Lock(); j.running = false; schedMu.Unlock(); yyWakeTickLoop() }()
				select {
				case <-j.cancel:
					return
				default:
				}
				runScheduledHandler(j)
			}(j)
		}
		if wait < time.Millisecond {
			wait = time.Millisecond
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-yyTickWake:
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
}

func yySchedExit(j *schedJob) {
	j.exitOnce.Do(func() {
		if j.onExit != nil {
			j.onExit()
		}
	})
}
