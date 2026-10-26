package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ========== 7x7 ASCII 大字字符映射表 ==========

var zd = map[string][]string{
	"A": {"  ███  ", " █   █ ", "█     █", "███████", "█     █", "█     █", "█     █"},
	"B": {"██████ ", "█     █", "█     █", "██████ ", "█     █", "█     █", "██████ "},
	"C": {" █████ ", "█     █", "█      ", "█      ", "█      ", "█     █", " █████ "},
	"D": {"██████ ", "█     █", "█     █", "█     █", "█     █", "█     █", "██████ "},
	"E": {"███████", "█      ", "█      ", "██████ ", "█      ", "█      ", "███████"},
	"F": {"███████", "█      ", "█      ", "██████ ", "█      ", "█      ", "█      "},
	"G": {" █████ ", "█     █", "█      ", "█  ████", "█     █", "█     █", " █████ "},
	"H": {"█     █", "█     █", "█     █", "███████", "█     █", "█     █", "█     █"},
	"I": {"███████", "   █   ", "   █   ", "   █   ", "   █   ", "   █   ", "███████"},
	"J": {"      █", "      █", "      █", "      █", "█     █", "█     █", " █████ "},
	"K": {"█    █ ", "█   █  ", "█  █   ", "███    ", "█  █   ", "█   █  ", "█    █ "},
	"L": {"█      ", "█      ", "█      ", "█      ", "█      ", "█      ", "███████"},
	"M": {"█     █", "██   ██", "█ █ █ █", "█  █  █", "█     █", "█     █", "█     █"},
	"N": {"█     █", "██    █", "█ █   █", "█  █  █", "█   █ █", "█    ██", "█     █"},
	"O": {" █████ ", "█     █", "█     █", "█     █", "█     █", "█     █", " █████ "},
	"P": {"██████ ", "█     █", "█     █", "██████ ", "█      ", "█      ", "█      "},
	"Q": {" █████ ", "█     █", "█     █", "█     █", "█   █ █", "█    ██", " ██████"},
	"R": {"██████ ", "█     █", "█     █", "██████ ", "█   █  ", "█    █ ", "█     █"},
	"S": {" █████ ", "█     █", " █     ", "  ███  ", "     █ ", "█     █", " █████ "},
	"T": {"███████", "   █   ", "   █   ", "   █   ", "   █   ", "   █   ", "   █   "},
	"U": {"█     █", "█     █", "█     █", "█     █", "█     █", "█     █", " █████ "},
	"V": {"█     █", "█     █", "█     █", "█     █", " █   █ ", "  █ █  ", "   █   "},
	"W": {"█     █", "█     █", "█  █  █", "█  █  █", "█ █ █ █", "██   ██", "█     █"},
	"X": {"█     █", " █   █ ", "  █ █  ", "   █   ", "  █ █  ", " █   █ ", "█     █"},
	"Y": {"█     █", " █   █ ", "  █ █  ", "   █   ", "   █   ", "   █   ", "   █   "},
	"Z": {"███████", "      █", "     █ ", "    █  ", "   █   ", "  █    ", "███████"},
	"0": {" █████ ", "█   █ █", "█  █  █", "█ █   █", "██    █", "█     █", " █████ "},
	"1": {"   █   ", "  ██   ", " █ █   ", "   █   ", "   █   ", "   █   ", "███████"},
	"2": {" █████ ", "█     █", "      █", " █████ ", "█      ", "█      ", "███████"},
	"3": {" █████ ", "█     █", "      █", " █████ ", "      █", "█     █", " █████ "},
	"4": {"█      ", "█     █", "█     █", "███████", "      █", "      █", "      █"},
	"5": {"███████", "█      ", "██████ ", "      █", "      █", "█     █", " █████ "},
	"6": {" █████ ", "█     █", "█      ", "██████ ", "█     █", "█     █", " █████ "},
	"7": {"███████", "      █", "     █ ", "    █  ", "   █   ", "  █    ", "  █    "},
	"8": {" █████ ", "█     █", "█     █", " █████ ", "█     █", "█     █", " █████ "},
	"9": {" █████ ", "█     █", "█     █", " ██████", "      █", "█     █", " █████ "},
	":": {"       ", "   █   ", "   █   ", "       ", "   █   ", "   █   ", "       "},
	".": {"       ", "       ", "       ", "       ", "       ", "   █   ", "   █   "},
	"!": {"   █   ", "   █   ", "   █   ", "   █   ", "   █   ", "       ", "   █   "},
	"?": {" █████ ", "█     █", "      █", "    █  ", "   █   ", "       ", "   █   "},
	" ": {"       ", "       ", "       ", "       ", "       ", "       ", "       "},
}

// ========== 配置 ==========

type MarqueeConfig struct {
	Text      string `json:"text"`
	Speed     int    `json:"speed"`
	Width     int    `json:"width"`
	Color     string `json:"color"`
	TextChar  string `json:"text_char"`
	FillChar  string `json:"fill_char"`
	Direction string `json:"direction"`
	Rainbow   bool   `json:"rainbow"`
}

func (c *MarqueeConfig) fillDefaults() {
	if c.Speed <= 0 {
		c.Speed = 200
	}
	if c.Width <= 0 {
		c.Width = 40
	}
	if c.Color == "" {
		c.Color = "a"
	}
	if c.TextChar == "" {
		c.TextChar = "█"
	}
	if c.Direction == "" {
		c.Direction = "left"
	}
}

const mcColorPrefix = "§"

var rainbowColors = []string{"c", "6", "e", "a", "b", "d", "5"}

// ========== 状态 ==========

var (
	marqueeMu      sync.Mutex
	marqueeRunning bool
	marqueeStopCh  chan struct{}
	marqueeCfg     MarqueeConfig
)

// ========== ASCII 大字构建 ==========

// mcWidth 返回字符在 Minecraft 中相对空格的宽度（空格/ASCII=1，方块类=2）。
// 用于把填充格补空格到与文字格等宽，使列对齐、文字不再割裂。
// 不同字体/平台宽度可能不同，可在 switch 里调整。
func mcWidth(ch rune) int {
	switch ch {
	case '█', '■', '▀', '▄', '▌', '▐', '░', '▒', '▓', '▬', '▪', '◼', '◆', '●', '▲', '▼', '▮', '▚', '▞', '▖', '▗', '▘', '▝':
		return 2
	default:
		return 1
	}
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return ' '
}

// buildMarqueeCells 构建 7 行 ASCII 大字画布，每行是"格子"数组。
// 每个格子的视觉宽度统一为 target 个空格宽：文字格(如 ■)保持原样，
// 填充格(如空格)后面补空格补齐，这样列在游戏内对齐、文字不再割裂。
// 返回 rows(每行格子数组)、总格数、target(每格空格宽)。
func buildMarqueeCells(text string, textChar, fillChar string) ([][]string, int, int) {
	upper := strings.ToUpper(text)
	if fillChar == "" {
		fillChar = " "
	}
	if textChar == "" {
		textChar = "█"
	}
	target := mcWidth(firstRune(textChar))
	if fw := mcWidth(firstRune(fillChar)); fw > target {
		target = fw
	}
	textCell := textChar
	fillCell := fillChar + strings.Repeat(" ", target-mcWidth(firstRune(fillChar)))
	rows := make([][]string, 7)
	for _, ch := range upper {
		art, ok := zd[string(ch)]
		if !ok {
			art = zd[" "]
		}
		for i := 0; i < 7; i++ {
			var cells []string
			for _, c := range art[i] {
				if c == '█' {
					cells = append(cells, textCell)
				} else {
					cells = append(cells, fillCell)
				}
			}
			if len(rows[i]) > 0 {
				rows[i] = append(rows[i], fillCell, fillCell)
			}
			rows[i] = append(rows[i], cells...)
		}
	}
	n := 0
	if len(rows) > 0 {
		n = len(rows[0])
	}
	return rows, n, target
}

// scrollCellRow 从起始格 pos 取 width 格，前后用 fillCell 补齐，形成一帧滚动行。
func scrollCellRow(row []string, pos, width int, fillCell string) string {
	padded := make([]string, 0, 2*width+len(row))
	for i := 0; i < width; i++ {
		padded = append(padded, fillCell)
	}
	padded = append(padded, row...)
	for i := 0; i < width; i++ {
		padded = append(padded, fillCell)
	}
	var b strings.Builder
	for k := 0; k < width; k++ {
		b.WriteString(padded[pos+k])
	}
	return b.String()
}

func applyColor(line string, color string) string {
	return mcColorPrefix + color + line
}

func applyRainbow(line string, offset int) string {
	var b strings.Builder
	for i, ch := range line {
		ci := (i + offset) % len(rainbowColors)
		b.WriteString(mcColorPrefix + rainbowColors[ci] + string(ch))
	}
	return b.String()
}

// ========== 滚动循环 ==========

func marqueeLoop() {
	marqueeMu.Lock()
	cfg := marqueeCfg
	stopCh := marqueeStopCh
	marqueeMu.Unlock()

	rows, cellCount, target := buildMarqueeCells(cfg.Text, cfg.TextChar, cfg.FillChar)
	if cellCount <= 0 {
		return
	}

	width := cfg.Width
	if width < 10 {
		width = 10
	}
	if width > 120 {
		width = 120
	}

	// 跑马灯：填充 + 文字 + 填充。填充格补空格到与文字格等宽(target)。
	fillCell := cfg.FillChar
	if fillCell == "" {
		fillCell = " "
	}
	fillCell += strings.Repeat(" ", target-mcWidth(firstRune(fillCell)))

	maxFrame := cellCount + width
	frame := 0
	ticker := time.NewTicker(time.Duration(cfg.Speed) * time.Millisecond)
	defer ticker.Stop()

	rainbowOffset := 0

	for {
		select {
		case <-stopCh:
			botMgrMu.Lock()
			if botMgr != nil && botMgr.IsConnected() {
				botMgr.SendWOCmd(`titleraw @a actionbar {"rawtext":[{"text":""}]}`)
			}
			botMgrMu.Unlock()
			return

		case <-ticker.C:
			pos := frame % maxFrame

			var lines []string
			for i := 0; i < 7; i++ {
				line := scrollCellRow(rows[i], pos, width, fillCell)
				if cfg.Rainbow {
					line = applyRainbow(line, rainbowOffset)
				} else {
					line = applyColor(line, cfg.Color)
				}
				lines = append(lines, line)
			}

			marqueeText := strings.Join(lines, "\n")
			cmd := fmt.Sprintf(`titleraw @a actionbar {"rawtext":[{"text":%s}]}`, mustJSON(marqueeText))

			botMgrMu.Lock()
			if botMgr != nil && botMgr.IsConnected() {
				botMgr.SendWOCmd(cmd)
			}
			botMgrMu.Unlock()

			frame++
			rainbowOffset++
		}
	}
}

// ========== API 处理函数 ==========

// startMarquee 启动一个跑马灯(游戏内 actionbar 顶部滚动)。text 为空则忽略。
// 被 HTTP 处理函数与推送通知分发共用。
func startMarquee(req MarqueeConfig) {
	if req.Text == "" {
		return
	}
	req.fillDefaults()

	marqueeMu.Lock()
	if marqueeRunning {
		close(marqueeStopCh)
	}
	marqueeCfg = req
	marqueeStopCh = make(chan struct{})
	marqueeRunning = true
	marqueeMu.Unlock()

	go marqueeLoop()
}

func handleMarqueeStart(w http.ResponseWriter, r *http.Request) {
	var req MarqueeConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无效JSON: " + err.Error()})
		return
	}
	startMarquee(req)

	writeJSON(w, map[string]any{"ok": true})
}

func handleMarqueeStop(w http.ResponseWriter, r *http.Request) {
	marqueeMu.Lock()
	if marqueeRunning {
		close(marqueeStopCh)
		marqueeRunning = false
	}
	marqueeMu.Unlock()

	writeJSON(w, map[string]any{"ok": true})
}

func handleMarqueeStatus(w http.ResponseWriter, r *http.Request) {
	marqueeMu.Lock()
	running := marqueeRunning
	cfg := marqueeCfg
	marqueeMu.Unlock()

	writeJSON(w, map[string]any{
		"ok":      true,
		"running": running,
		"config":  cfg,
	})
}

func handleMarqueePreview(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	textChar := r.URL.Query().Get("text_char")
	fillChar := r.URL.Query().Get("fill_char")
	if text == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少 text 参数"})
		return
	}
	if textChar == "" {
		textChar = "█"
	}
	if fillChar == "" {
		fillChar = " "
	}
	// 返回"格子"数组（填充格已补空格到与文字格等宽），JS 按格子滑动预览，与游戏内一致
	rows, _, target := buildMarqueeCells(text, textChar, fillChar)
	writeJSON(w, map[string]any{"ok": true, "rows": rows, "cell_width": target})
}