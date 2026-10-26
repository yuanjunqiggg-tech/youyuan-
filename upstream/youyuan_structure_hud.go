package main

import (
	"fmt"
	"strings"
	"time"
)

// yyStructureHud is the in-game actionbar panel for structure import/export.
// Every field is either measured or explicitly marked unknown. The previous
// single-line counter could not show TPS, RTT, rate, chunk span and remaining
// time at once, so operators had to open the toolbox to tell "slow" apart from
// "stuck".
type yyStructureHud struct {
	// Phase is the human label after the ● marker: 导入中 / 导出中 / 已暂停 / 已完成.
	Phase string
	// PhaseColour is the § colour digit for the phase dot (a green, e red, e yellow, 6 gold).
	PhaseColour string

	BlocksDone  int
	BlocksTotal int

	// ChunksDone is an estimate whenever the caller can only derive it from the
	// block ratio. ChunksExact marks the measured case so the renderer can add
	// "~" instead of implying a per-chunk counter exists.
	ChunksDone  int
	ChunksTotal int
	ChunksExact bool

	Speed     float64
	SpeedKnown bool

	TPS      float64
	TPSKnown bool

	RTT      time.Duration
	RTTKnown bool

	// Spark holds recent throughput samples for the ▂▄▆█ meter. Empty means
	// "not measured yet" and renders as a flat grey baseline.
	Spark []float64

	Elapsed        time.Duration
	Remaining      time.Duration
	RemainingKnown bool

	// Cursor is the coordinate being written, e.g. "X1024 Z-288". Empty falls
	// back to the structure name and size, which is measured rather than guessed.
	Cursor    string
	Structure string
	SizeX     int
	SizeY     int
	SizeZ     int

	// Archive is retained for existing callers; the player panel omits build labels.
	Archive string
}

// yyStructureHudPercent prefers the block ratio because it is the measured one.
func yyStructureHudPercent(h yyStructureHud) int {
	if h.BlocksTotal > 0 {
		return yyStructureClampPercent(h.BlocksDone * 100 / h.BlocksTotal)
	}
	if h.ChunksTotal > 0 {
		return yyStructureClampPercent(h.ChunksDone * 100 / h.ChunksTotal)
	}
	return 0
}

func yyStructureClampPercent(pct int) int {
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// yyStructureChunks converts a structure footprint into the chunk count the
// server actually loads. This is arithmetic on the requested size, not an
// observation of what has been written.
func yyStructureChunks(sizeX, sizeZ int) int {
	cx := (sizeX + 15) / 16
	cz := (sizeZ + 15) / 16
	if cx < 1 {
		cx = 1
	}
	if cz < 1 {
		cz = 1
	}
	return cx * cz
}

// yyHudClock renders mm:ss, growing to h:mm:ss past an hour so a long job does
// not read as 90:00.
func yyHudClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int64(d / time.Second)
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// yyHudSpark renders the last samples as a block meter. Values are scaled
// against the window maximum so a stable rate stays visibly steady instead of
// collapsing to the bottom row when absolute throughput is low.
func yyHudSpark(samples []float64, width int) string {
	ramp := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	if width <= 0 {
		width = 4
	}
	if len(samples) == 0 {
		return "§8" + strings.Repeat(string(ramp[0]), width)
	}
	start := 0
	if len(samples) > width {
		start = len(samples) - width
	}
	window := samples[start:]
	peak := 0.0
	for _, v := range window {
		if v > peak {
			peak = v
		}
	}
	var sb strings.Builder
	sb.WriteString("§b")
	for _, v := range window {
		idx := 0
		if peak > 0 && v > 0 {
			idx = int(v / peak * float64(len(ramp)-1))
			if idx < 1 {
				idx = 1
			}
		}
		if idx >= len(ramp) {
			idx = len(ramp) - 1
		}
		sb.WriteRune(ramp[idx])
	}
	for i := len(window); i < width; i++ {
		sb.WriteString("§8")
		sb.WriteRune(ramp[0])
	}
	return sb.String()
}

// yyStructureHudBar draws the filled/empty bar. Ten cells keeps the panel the
// same width as the reference layout on a phone.
func yyStructureHudBar(pct int) string {
	const cells = 10
	filled := pct * cells / 100
	if filled < 0 {
		filled = 0
	}
	if filled > cells {
		filled = cells
	}
	return "§b" + strings.Repeat("▰", filled) + "§8" + strings.Repeat("▱", cells-filled)
}

// Split metrics and bound names so a long structure filename cannot widen the panel.
func yyStructureHudRender(h yyStructureHud) string {
	const (
		top    = "§8╭━━§b ✦ 游园 §8━━━━╮"
		rule   = "§8┣━━━━━━━━━━━━━━┫"
		bottom = "§8╰━━━━━━━━━━━━━━╯"
	)
	phase := strings.TrimSpace(h.Phase)
	if phase == "" {
		phase = "导入中"
	}
	colour := strings.TrimSpace(h.PhaseColour)
	if colour == "" {
		colour = "a"
	}
	pct := yyStructureHudPercent(h)
	remainingBlocks := "§8—"
	if h.BlocksTotal > 0 {
		remainingBlocks = fmt.Sprintf("§f%d", max(0, h.BlocksTotal-h.BlocksDone))
	}

	lines := []string{
		top,
		fmt.Sprintf("§8┃ §%s● §f%s", colour, yyStructureHudLabel(phase, 12)),
		rule,
		fmt.Sprintf("§8┃ %s  §a%d%%", yyStructureHudBar(pct), pct),
		fmt.Sprintf("§8┃ §7区块 %s", yyStructureHudChunkText(h)),
		"§8┃ §7方块 " + yyStructureHudBlockText(h),
		yyStructureHudRateLine(h),
		"§8┃ §7剩余方块 " + remainingBlocks,
		yyStructureHudHealthLine(h),
		fmt.Sprintf("§8┃ §7已用 §f%s", yyHudClock(h.Elapsed)),
		"§8┃ §7预计剩余 " + yyStructureHudRemaining(h),
		rule,
		yyStructureHudCursorLine(h),
		bottom,
	}
	return strings.Join(lines, "\n")
}

func yyStructureHudLabel(text string, limit int) string {
	text = strings.NewReplacer("§", "", "\n", " ", "\r", " ").Replace(strings.TrimSpace(text))
	r := []rune(text)
	if len(r) > limit {
		return string(r[:limit-1]) + "…"
	}
	return text
}

func yyStructureHudBlockText(h yyStructureHud) string {
	if h.BlocksTotal <= 0 {
		return "§8—"
	}
	return fmt.Sprintf("§f%d§8/§f%d", max(0, h.BlocksDone), h.BlocksTotal)
}

func yyStructureHudChunkText(h yyStructureHud) string {
	if h.ChunksTotal <= 0 {
		return "§8—"
	}
	prefix := ""
	if !h.ChunksExact {
		prefix = "~"
	}
	return fmt.Sprintf("%s§f%d§8/§f%d", prefix, max(0, h.ChunksDone), h.ChunksTotal)
}

func yyStructureHudRateLine(h yyStructureHud) string {
	if !h.SpeedKnown || h.Speed <= 0 {
		return "§8┃ §7速率 §8—"
	}
	line := fmt.Sprintf("§8┃ §7速率 §b%s§7块/s %s", yyStructureHudSpeed(h.Speed), yyHudSpark(h.Spark, 4))
	return line
}

func yyStructureHudSpeed(speed float64) string {
	switch {
	case speed >= 100:
		return fmt.Sprintf("%.0f", speed)
	case speed >= 10:
		return fmt.Sprintf("%.1f", speed)
	default:
		return fmt.Sprintf("%.2f", speed)
	}
}

// yyStructureHudHealthLine keeps the honesty rule from yyTaskActionbarMetrics:
// an unavailable TPS is stated, never replaced by a configured tick rate or a
// local clock estimate.
func yyStructureHudHealthLine(h yyStructureHud) string {
	tps := "§7TPS §8不可用"
	if h.TPSKnown {
		tps = fmt.Sprintf("§7TPS §a%.1f", h.TPS)
	}
	rtt := "§7RTT §8—"
	if h.RTTKnown && h.RTT > 0 {
		rtt = fmt.Sprintf("§7RTT §a%dms", h.RTT.Round(time.Millisecond)/time.Millisecond)
	}
	return fmt.Sprintf("§8┃ %s  %s", tps, rtt)
}

func yyStructureHudRemaining(h yyStructureHud) string {
	if h.RemainingKnown {
		return "§e" + yyHudClock(h.Remaining)
	}
	if h.SpeedKnown && h.Speed > 0 && h.BlocksTotal > h.BlocksDone {
		left := time.Duration(float64(h.BlocksTotal-h.BlocksDone)/h.Speed) * time.Second
		return "§e" + yyHudClock(left)
	}
	return "§8—"
}

func yyStructureHudCursorLine(h yyStructureHud) string {
	if cursor := strings.TrimSpace(h.Cursor); cursor != "" {
		return fmt.Sprintf("§8┃ §7位置 §f%s\n§8┃ §7建筑 §f%s", yyStructureHudLabel(cursor, 20), yyStructureHudLabel(h.Structure, 12))
	}
	name := strings.TrimSpace(h.Structure)
	if name == "" {
		name = "结构"
	}
	if h.SizeX > 0 && h.SizeY > 0 && h.SizeZ > 0 {
		return fmt.Sprintf("§8┃ §7建筑 §f%s\n§8┃ §7尺寸 §f%d×%d×%d", yyStructureHudLabel(name, 12), h.SizeX, h.SizeY, h.SizeZ)
	}
	return fmt.Sprintf("§8┃ §7建筑 §f%s", yyStructureHudLabel(name, 12))
}

// yyStructureHudForImport maps the counters the import task already tracks onto
// the panel. Chunk progress is derived from the block ratio, so it is marked as
// an estimate unless the caller supplies a measured value.
func yyStructureHudForImport(phase, phaseColour, name string, sx, sy, sz int,
	blocks, totalSolid, realSpeed int, elapsed time.Duration, spark []float64) yyStructureHud {
	totalChunks := yyStructureChunks(sx, sz)
	doneChunks := 0
	if totalSolid > 0 {
		doneChunks = blocks * totalChunks / totalSolid
	}
	speed := float64(realSpeed)
	return yyStructureHud{
		Phase:       phase,
		PhaseColour: phaseColour,
		BlocksDone:  blocks,
		BlocksTotal: totalSolid,
		ChunksDone:  doneChunks,
		ChunksTotal: totalChunks,
		ChunksExact: false,
		Speed:       speed,
		SpeedKnown:  realSpeed > 0,
		Elapsed:     elapsed,
		Spark:       spark,
		Structure:   name,
		SizeX:       sx,
		SizeY:       sy,
		SizeZ:       sz,
	}
}

// yyStructureHudWithHealth fills the measured link figures. Kept separate so the
// pure renderer stays testable without a live connection.
func yyStructureHudWithHealth(h yyStructureHud) yyStructureHud {
	if bm := getBotManager(); bm != nil {
		if tps, known := bm.observedTPS(); known {
			h.TPS, h.TPSKnown = tps, true
		}
		if rtt, known := bm.observedRTT(); known {
			h.RTT, h.RTTKnown = rtt, true
		}
	}
	return h
}

// observedRTT reports the round trip from the RakNet resend statistics. Latency()
// is documented as half the round trip, so it is doubled rather than presented
// as a one-way figure.
func (bm *BotManager) observedRTT() (time.Duration, bool) {
	if bm == nil {
		return 0, false
	}
	bm.mu.Lock()
	conn, connected := bm.conn, bm.connected
	bm.mu.Unlock()
	if !connected || conn == nil {
		return 0, false
	}
	half := conn.Latency()
	if half <= 0 {
		return 0, false
	}
	return 2 * half, true
}
