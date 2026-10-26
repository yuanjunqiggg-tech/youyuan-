package main

import (
	"fmt"
	"strings"
)

func yyFleetConnectionPanel(brand string, ready, total int) string {
	if brand == "" {
		brand = "游园"
	}
	name := []rune(brand)
	if len(name) > 8 {
		brand = string(name[:8])
	}
	total = max(0, total)
	ready = max(0, min(ready, total))
	filled := 0
	if total > 0 {
		filled = ready * 12 / total
	}
	return fmt.Sprintf("§r§b%s §8· §f连接机器人\n§a%s§8%s\n§7已就绪 §f%d/%d §8· §7等待 §f%d", brand, strings.Repeat("━", filled), strings.Repeat("━", 12-filled), ready, total, total-ready)
}
