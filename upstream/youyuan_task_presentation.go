package main

import (
	"fmt"
	"strings"
	"time"
)

// yyTaskActionbar presents real counters. Submission progress is not a claim
// that the server has acknowledged every block operation.
func yyTaskActionbar(kind, name, colour string, done, total, speed int, elapsed time.Duration) string {
	if total < 0 {
		total = 0
	}
	if done < 0 {
		done = 0
	}
	tps, known := getBotManager().observedTPS()
	return yyTaskActionbarMetrics(kind, name, colour, done, total, speed, elapsed, tps, known)
}

func yyTaskActionbarMetrics(kind, name, colour string, done, total, speed int, elapsed time.Duration, tps float64, tpsKnown bool) string {
	if total < 0 {
		total = 0
	}
	if done < 0 {
		done = 0
	}
	filled := 0
	if total > 0 {
		filled = int(float64(done) / float64(total) * 10)
	}
	if filled > 10 {
		filled = 10
	}
	bar := "§" + colour + strings.Repeat("┃", filled) + "§8" + strings.Repeat("┃", 10-filled)
	name = strings.NewReplacer("§", "", "\n", " ", "\r", " ").Replace(name)
	if r := []rune(name); len(r) > 12 {
		name = string(r[:11]) + "…"
	}
	lines := []string{fmt.Sprintf("§r§%s游园 · %s", colour, kind), "§r§f" + name, "§r" + bar, fmt.Sprintf("§r§7数量 §f%d §7/ %d", done, total)}
	if speed > 0 && done < total {
		lines = append(lines, fmt.Sprintf("§r§7速率 §f%d§7/秒", speed), "§r§7预计剩余 §f"+yyHudClock((time.Duration(total-done)*time.Second)/time.Duration(speed)))
	}
	if elapsed > 0 {
		lines = append(lines, "§r§7已用 §f"+yyHudClock(elapsed))
	}
	tpsText := "§7TPS 暂不可用"
	if tpsKnown {
		tpsText = fmt.Sprintf("§7TPS §f%.1f", tps)
	}
	return strings.Join(append(lines, "§r"+tpsText), "\n")
}
