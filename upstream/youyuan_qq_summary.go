package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// 群消息总结：.总结 [条数]
//
// 用户要求：「总结谁QQ群里面发了什么消息」。
// 这里做**本地确定性汇总**（不走模型）：谁的发言最多、最近几条原话是什么。
// 理由：模型总结要几秒、可能被工具调用打断、还可能编造没说过的话；
// 而"谁说了什么"是事实性需求，本地算反而又快又准。
// 需要"提炼要点"时再让 AI 基于这份事实去润色，不在这里做。

const (
	yySummaryDefault = 30
	// yyQQRecentMessages 内部最多取 80 条，这里跟着夹住，免得用户以为能取 200。
	yySummaryMax     = 80
	yySummaryQuotes  = 8
)

func yyQQCmdSummary(cf yyQQConfig, arg string, level int) []string {
	n := yySummaryDefault
	if f := strings.Fields(arg); len(f) > 0 {
		if v, e := strconv.Atoi(f[0]); e == nil {
			n = v
		}
	}
	if n < 5 {
		n = 5
	}
	if n > yySummaryMax {
		n = yySummaryMax
	}

	rows := yyQQRecentMessages(cf, n)
	if len(rows) == 0 {
		return []string{"最近没有可总结的消息（机器人只统计自己看到过的群消息）。"}
	}

	first, last := rows[0], rows[len(rows)-1]
	head := fmt.Sprintf("✦ 最近 %d 条消息（%s–%s）", len(rows), yySummaryClock(first), yySummaryClock(last))
	lines := []string{head, "━━━━━━━━━━━━━━", "发言：" + yySummaryTop(rows)}

	// 原话：只取最后几条，每条截断，避免刷屏。
	start := len(rows) - yySummaryQuotes
	if start < 0 {
		start = 0
	}
	for _, m := range rows[start:] {
		text := strings.TrimSpace(strings.ReplaceAll(m.Text, "\n", " "))
		if text == "" {
			text = "（非文字消息）"
		}
		if runes := []rune(text); len(runes) > 60 {
			text = string(runes[:60]) + "…"
		}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = "未知"
		}
		lines = append(lines, yySummaryClock(m)+" "+name+"："+text)
	}
	if len(rows) > yySummaryQuotes {
		lines = append(lines, fmt.Sprintf("（另有 %d 条更早的消息未逐条列出）", len(rows)-yySummaryQuotes))
	}
	return []string{strings.Join(lines, "\n")}
}

// yySummaryTop 发言量排行：按人聚合条数倒序，同条数按名字排序保证输出稳定。
// .总结 指令和 AI 的 group_summary 工具共用，避免两处统计口径不一致。
func yySummaryTop(rows []yyQQGroupMessage) string {
	type stat struct {
		name string
		n    int
	}
	counts := map[string]int{}
	for _, m := range rows {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = "未知"
		}
		counts[name]++
	}
	stats := make([]stat, 0, len(counts))
	for name, c := range counts {
		stats = append(stats, stat{name: name, n: c})
	}
	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].n != stats[j].n {
			return stats[i].n > stats[j].n
		}
		return stats[i].name < stats[j].name
	})
	if len(stats) > 6 {
		stats = stats[:6]
	}
	parts := make([]string, 0, len(stats))
	for _, s := range stats {
		parts = append(parts, s.name+" "+strconv.Itoa(s.n)+" 条")
	}
	return strings.Join(parts, " · ")
}

// yySummaryClock 时间只到分钟：群里看"几点说的"就够了。
func yySummaryClock(m yyQQGroupMessage) string {
	if m.Timestamp.IsZero() {
		return "--:--"
	}
	return m.Timestamp.Format("15:04")
}
