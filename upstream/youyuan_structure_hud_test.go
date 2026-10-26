package main

import (
	"strings"
	"testing"
	"time"
)

// The actionbar panel is the only place an operator can see progress without
// opening the toolbox, so its layout and its honesty rules are both load-bearing.
func TestStructureHudMatchesHouseLayout(t *testing.T) {
	hud := yyStructureHud{
		Phase:          "导入中",
		PhaseColour:    "a",
		BlocksDone:     1290240,
		BlocksTotal:    1843200,
		ChunksDone:     56,
		ChunksTotal:    80,
		ChunksExact:    true,
		Speed:          8600,
		SpeedKnown:     true,
		TPS:            19.6,
		TPSKnown:       true,
		RTT:            46 * time.Millisecond,
		RTTKnown:       true,
		Spark:          []float64{4, 6, 8, 10},
		Elapsed:        150 * time.Second,
		Remaining:      64 * time.Second,
		RemainingKnown: true,
		Cursor:         "X1024 Z-288",
	}
	text := yyStructureHudRender(hud)
	for _, want := range []string{
		"§8╭━━§b ✦ 游园 §8━━━━╮",
		"§8┃ §a● §f导入中",
		"§8┃ §7区块 §f56§8/§f80",
		"§8┃ §b▰▰▰▰▰▰▰§8▱▱▱  §a70%",
		"§8┃ §7方块 §f1290240§8/§f1843200",
		"§8┃ §7速率 §b8600§7块/s",
		"§8┃ §7剩余方块 §f552960",
		"§8┃ §7TPS §a19.6  §7RTT §a46ms",
		"§8┃ §7已用 §f02:30\n§8┃ §7预计剩余 §e01:04",
		"§8┃ §7位置 §fX1024 Z-288",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "STRUCTURE I/O") || strings.Contains(text, "ARCHIVE v") {
		t.Fatal("internal filler made the player panel wider", text)
	}
}

// Same rule as yyTaskActionbarMetrics: never invent TPS. An unknown figure is
// stated as unavailable instead of falling back to 20 or to the tick rate.
func TestStructureHudNeverInventsTPSOrRTT(t *testing.T) {
	text := yyStructureHudRender(yyStructureHud{
		Phase: "导入中", BlocksDone: 1, BlocksTotal: 10, ChunksTotal: 4,
	})
	if !strings.Contains(text, "§7TPS §8不可用") {
		t.Fatal("unknown TPS must be stated", text)
	}
	if strings.Contains(text, "TPS §a20.0") || strings.Contains(text, "§a19.") {
		t.Fatal("TPS was invented", text)
	}
	if !strings.Contains(text, "§7RTT §8—") {
		t.Fatal("unknown RTT must be a dash", text)
	}
	if strings.Contains(text, "速率 §b") {
		t.Fatal("unknown speed must not be shown as a measured rate", text)
	}
}

// A derived chunk figure must be marked so nobody reads it as a per-chunk
// counter the host does not actually keep.
func TestStructureHudMarksDerivedChunkProgress(t *testing.T) {
	hud := yyStructureHudForImport("导入中", "a", "房子", 80, 20, 80,
		700, 1000, 100, 5*time.Second, nil)
	if hud.ChunksTotal != 25 {
		t.Fatalf("80x80 footprint spans 25 chunks, got %d", hud.ChunksTotal)
	}
	if hud.ChunksExact {
		t.Fatal("a block-ratio derivation is not an exact chunk count")
	}
	text := yyStructureHudRender(hud)
	if !strings.Contains(text, "§7区块 ~§f17§8/§f25") {
		t.Fatalf("derived chunk progress must carry the ~ marker:\n%s", text)
	}
}

func TestImportPartitionModesNeverBecomeExportInGame(t *testing.T) {
	for mode := 0; mode <= 4; mode++ {
		var text string
		buildV5Actionbar(func(s string) { text = s }, "房子", 80, 20, 80,
			mode, 100, 9500, 10, 1, 1, 1, 1, 4, 0, 25, 1, 2, time.Now(), "游园")
		if !strings.Contains(text, "导入中") || strings.Contains(text, "导出中") {
			t.Fatalf("partition mode %d mislabeled import: %s", mode, text)
		}
	}
}

func TestStructureHudPreparationAndLongNameStayCompact(t *testing.T) {
	name := strings.Repeat("海牛的家", 20) + "@[178788 206 -183864]"
	text := yyStructureHudRender(yyStructureHud{Phase: "导入准备", Structure: name})
	if strings.Contains(text, name) || strings.Contains(text, "0§8/§f0") || !strings.Contains(text, "海牛的家海牛的家海牛的…") {
		t.Fatal("preparation must clip long names and keep unknown totals unknown", text)
	}
	if len(strings.Split(text, "\n")) < 14 {
		t.Fatal("panel collapsed back to a wide short strip", text)
	}
}

func TestStructureChunkSpan(t *testing.T) {
	cases := []struct{ x, z, want int }{
		{1, 1, 1}, {16, 16, 1}, {17, 16, 2}, {32, 32, 4}, {80, 80, 25}, {0, 0, 1},
	}
	for _, c := range cases {
		if got := yyStructureChunks(c.x, c.z); got != c.want {
			t.Fatalf("%dx%d want %d chunks, got %d", c.x, c.z, c.want, got)
		}
	}
}

func TestHudClockGrowsPastAnHour(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "00:00"},
		{150 * time.Second, "02:30"},
		{64 * time.Second, "01:04"},
		{time.Hour + 2*time.Minute + 5*time.Second, "1:02:05"},
		{-5 * time.Second, "00:00"},
	}
	for _, c := range cases {
		if got := yyHudClock(c.in); got != c.want {
			t.Fatalf("%v want %s, got %s", c.in, c.want, got)
		}
	}
}

func TestHudSparkDegradesWithoutSamples(t *testing.T) {
	flat := yyHudSpark(nil, 4)
	if !strings.HasPrefix(flat, "§8") {
		t.Fatalf("no samples must render a grey baseline, got %q", flat)
	}
	if got := len([]rune(strings.TrimPrefix(flat, "§8"))); got != 4 {
		t.Fatalf("meter width must stay fixed, got %d", got)
	}
	full := yyHudSpark([]float64{1, 1, 1, 1}, 4)
	if !strings.Contains(full, "█") {
		t.Fatalf("a steady rate should fill the meter, got %q", full)
	}
}
