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
		filled = int(float64(done) / float64(total) * 18)
	}
	if filled > 18 {
		filled = 18
	}
	bar := "§" + colour + strings.Repeat("┃", filled) + "§8" + strings.Repeat("┃", 18-filled)
	name = strings.NewReplacer("§", "", "\n", " ", "\r", " ").Replace(name)
	if r := []rune(name); len(r) > 24 {
		name = string(r[:23]) + "…"
	}
	metric := fmt.Sprintf("§f%d §7/ %d", done, total)
	if speed > 0 && done < total {
		metric += fmt.Sprintf("  ·  §f%d§7/秒  ·  剩余约 %s", speed, ((time.Duration(total-done) * time.Second) / time.Duration(speed)).Round(time.Second))
	} else if elapsed > 0 {
		metric += "  ·  已用 " + elapsed.Round(time.Second).String()
	}
	tpsText := "§7TPS 暂不可用"
	if tpsKnown {
		tpsText = fmt.Sprintf("§7TPS §f%.1f", tps)
	}
	return fmt.Sprintf("§r§%s游园 · %s §7| §f%s\n§r%s  %s\n§r%s", colour, kind, name, bar, tpsText, metric)
}
