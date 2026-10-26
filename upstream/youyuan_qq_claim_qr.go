package main

// 超级管理员认领码 · 二维码
//
// 用户原话：「生成超级管理员认领码的话 可以变成用QQ去扫一张二维码」。
// 现在面板上生成的是一串 24 位码，得由操作者手打/复制到群里；改成二维码后，
// 手机 QQ「扫一扫」直接得到一条可复制的 `.认领 <码>` 指令，粘到群里就完事。
// 文本码保留作兜底（二维码接口不可用、或对方就想复制文本时照旧）。
//
// 为什么自己写 QR：项目 go.mod 里没有任何二维码库，引第三方依赖要走主控审批
// （见报告）。认领码只有 30 多字节，用版本 3（29×29，纠错 M，容量 42 字节）
// 和版本 4（33×33，容量 62 字节）就够，所以这里只实现这两档（外加更短的
// 1、2 档和 5、6 档，成本为零），不碰版本信息块（v≥7 才有），
// 代码量小、没有外部依赖、也不改 go.mod。
//
// 编码范围：字节模式（UTF-8）+ 纠错等级 M。够装「.认领 + 24 位码」，
// 也够装 url 形态的认领链接。超长内容直接报错，让调用方回退文本码。

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// ---------- GF(256) ----------

var (
	yyQRExpTab [512]byte
	yyQRLogTab [256]int
	yyQRGFOnce sync.Once
)

func yyQRGFInit() {
	yyQRGFOnce.Do(func() {
		x := 1
		for i := 0; i < 255; i++ {
			yyQRExpTab[i] = byte(x)
			yyQRLogTab[x] = i
			x <<= 1
			if x&0x100 != 0 {
				x ^= 0x11D
			}
		}
		for i := 255; i < 512; i++ {
			yyQRExpTab[i] = yyQRExpTab[i-255]
		}
	})
}

func yyQRMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return yyQRExpTab[int(yyQRLogTab[a])+int(yyQRLogTab[b])]
}

// ---------- 规格表（仅纠错等级 M） ----------

type yyQRSpec struct {
	version int
	ec      int   // 每个纠错块的纠错码字数
	data    []int // 每个数据块的数据码字数
}

// 只列 1~6：版本 7 起要写版本信息块，认领码用不到，省掉一段易错逻辑。
var yyQRSpecs = []yyQRSpec{
	{1, 10, []int{16}},
	{2, 16, []int{28}},
	{3, 26, []int{44}},
	{4, 18, []int{32, 32}},
	{5, 24, []int{43, 43}},
	{6, 16, []int{27, 27, 27, 27}},
}

// yyQRAlignCenters 各版本对齐图案中心坐标（1 版没有对齐图案）。
var yyQRAlignCenters = map[int][]int{
	2: {6, 18},
	3: {6, 22},
	4: {6, 26},
	5: {6, 30},
	6: {6, 34},
}

// yyQRMaxPayload 版本 6-M 的字节模式容量；超过就直接报错回退文本码。
const yyQRMaxPayload = 106

func yyQRPickSpec(n int) (yyQRSpec, bool) {
	for _, s := range yyQRSpecs {
		total := 0
		for _, d := range s.data {
			total += d
		}
		// 模式 4 位 + 字符计数 8 位 = 12 位，折算 1.5 字节，向下取整后
		// 可用字节数 = 数据码字数 - 2。
		if n <= total-2 {
			return s, true
		}
	}
	return yyQRSpec{}, false
}

// ---------- 位流 / 纠错 ----------

type yyQRBits struct{ bits []bool }

func (b *yyQRBits) put(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		b.bits = append(b.bits, (v>>uint(i))&1 == 1)
	}
}

func yyQRDataCodewords(payload []byte, s yyQRSpec) []byte {
	total := 0
	for _, d := range s.data {
		total += d
	}
	buf := &yyQRBits{}
	buf.put(4, 4) // 0100 = 字节模式
	buf.put(uint32(len(payload)), 8)
	for _, p := range payload {
		buf.put(uint32(p), 8)
	}
	for i := 0; i < 4 && len(buf.bits) < total*8; i++ { // 终止符
		buf.bits = append(buf.bits, false)
	}
	for len(buf.bits)%8 != 0 && len(buf.bits) < total*8 {
		buf.bits = append(buf.bits, false)
	}
	used := (len(buf.bits) + 7) / 8
	out := make([]byte, total)
	for i := 0; i < used && i < total; i++ {
		var v byte
		for j := 0; j < 8; j++ {
			k := i*8 + j
			if k < len(buf.bits) && buf.bits[k] {
				v |= 1 << uint(7-j)
			}
		}
		out[i] = v
	}
	pad := []byte{0xEC, 0x11}
	for i := used; i < total; i++ {
		out[i] = pad[(i-used)%2]
	}
	return out
}

func yyQRGenPoly(n int) []byte {
	g := []byte{1}
	for i := 0; i < n; i++ {
		ng := make([]byte, len(g)+1)
		for j := 0; j < len(g); j++ {
			ng[j] ^= g[j]
			ng[j+1] ^= yyQRMul(g[j], yyQRExpTab[i])
		}
		g = ng
	}
	return g
}

func yyQRRemainder(data, gen []byte) []byte {
	res := make([]byte, len(gen)-1)
	for _, b := range data {
		f := b ^ res[0]
		copy(res, res[1:])
		res[len(res)-1] = 0
		for i := range res {
			res[i] ^= yyQRMul(gen[i+1], f)
		}
	}
	return res
}

func yyQRInterleave(data []byte, s yyQRSpec) []byte {
	gen := yyQRGenPoly(s.ec)
	blocks := make([][]byte, len(s.data))
	ecs := make([][]byte, len(s.data))
	off := 0
	for i, n := range s.data {
		if off+n > len(data) {
			n = len(data) - off
		}
		blocks[i] = data[off : off+n]
		off += n
		ecs[i] = yyQRRemainder(blocks[i], gen)
	}
	out := make([]byte, 0, off+len(s.data)*s.ec)
	maxLen := 0
	for _, b := range blocks {
		if len(b) > maxLen {
			maxLen = len(b)
		}
	}
	for i := 0; i < maxLen; i++ {
		for _, b := range blocks {
			if i < len(b) {
				out = append(out, b[i])
			}
		}
	}
	for i := 0; i < s.ec; i++ {
		for _, e := range ecs {
			if i < len(e) {
				out = append(out, e[i])
			}
		}
	}
	return out
}

// ---------- 矩阵 ----------

func yyQRMatrix(text string) ([][]bool, error) {
	yyQRGFInit()
	payload := []byte(text)
	s, ok := yyQRPickSpec(len(payload))
	if !ok {
		return nil, fmt.Errorf("二维码内容太长（%d 字节，上限 %d）", len(payload), yyQRMaxPayload)
	}
	size := 17 + 4*s.version
	base := make([][]bool, size)
	fixed := make([][]bool, size)
	for i := range base {
		base[i] = make([]bool, size)
		fixed[i] = make([]bool, size)
	}
	set := func(r, c int, dark bool) {
		base[r][c] = dark
		fixed[r][c] = true
	}
	drawFinder := func(r0, c0 int) {
		for dr := -1; dr <= 7; dr++ {
			for dc := -1; dc <= 7; dc++ {
				r, c := r0+dr, c0+dc
				if r < 0 || c < 0 || r >= size || c >= size {
					continue
				}
				dark := false
				if dr >= 0 && dr <= 6 && dc >= 0 && dc <= 6 {
					dark = dr == 0 || dr == 6 || dc == 0 || dc == 6 ||
						(dr >= 2 && dr <= 4 && dc >= 2 && dc <= 4)
				}
				set(r, c, dark)
			}
		}
	}
	drawFinder(0, 0)
	drawFinder(0, size-7)
	drawFinder(size-7, 0)
	// 定时图案
	for i := 8; i < size-8; i++ {
		set(6, i, i%2 == 0)
		set(i, 6, i%2 == 0)
	}
	// 对齐图案
	if centers, ok := yyQRAlignCenters[s.version]; ok {
		last := centers[len(centers)-1]
		for _, r := range centers {
			for _, c := range centers {
				if (r == 6 && c == 6) || (r == 6 && c == last) || (r == last && c == 6) {
					continue
				}
				for dr := -2; dr <= 2; dr++ {
					for dc := -2; dc <= 2; dc++ {
						ar, ac := dr, dc
						if ar < 0 {
							ar = -ar
						}
						if ac < 0 {
							ac = -ac
						}
						// 5×5 对齐图案：外圈深、内圈（切比雪夫距离 1）浅、中心深。
						far := ar
						if ac > far {
							far = ac
						}
						set(r+dr, c+dc, far != 1)
					}
				}
			}
		}
	}
	// 预留格式信息区（先占位，选定掩码后再写真实值）
	for i := 0; i < 9; i++ {
		if !fixed[8][i] {
			set(8, i, false)
		}
		if !fixed[i][8] {
			set(i, 8, false)
		}
	}
	for i := 0; i < 8; i++ {
		if !fixed[8][size-1-i] {
			set(8, size-1-i, false)
		}
		if !fixed[size-1-i][8] {
			set(size-1-i, 8, false)
		}
	}
	// 数据：从右下角开始，两列一条带，在**同一行**里先右列后左列，
	// 逐行向上（下一条带向下），遇到功能模块跳过。
	data := yyQRInterleave(yyQRDataCodewords(payload, s), s)
	total := len(data) * 8
	bit := 0
	up := true
	for col := size - 1; col > 0; col -= 2 {
		if col == 6 { // 跳过定时列
			col--
		}
		for rowIdx := 0; rowIdx < size; rowIdx++ {
			r := rowIdx
			if up {
				r = size - 1 - rowIdx
			}
			for k := 0; k < 2; k++ {
				c := col - k
				if fixed[r][c] {
					continue
				}
				dark := false
				if bit < total {
					dark = (data[bit/8]>>uint(7-bit%8))&1 == 1
				}
				bit++
				base[r][c] = dark
			}
		}
		up = !up
	}

	best := base
	bestScore := math.MaxInt32
	for mask := 0; mask < 8; mask++ {
		cand := make([][]bool, size)
		for i := range cand {
			cand[i] = make([]bool, size)
			copy(cand[i], base[i])
		}
		yyQRApplyMask(cand, fixed, mask)
		yyQRPlaceFormat(cand, mask)
		cand[size-8][8] = true // 固定深色模块，任何掩码下都不参与
		if sc := yyQRPenalty(cand); sc < bestScore {
			bestScore, best = sc, cand
		}
	}
	return best, nil
}

func yyQRApplyMask(m, fixed [][]bool, mask int) {
	n := len(m)
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if fixed[r][c] {
				continue
			}
			var hit bool
			switch mask {
			case 0:
				hit = (r+c)%2 == 0
			case 1:
				hit = r%2 == 0
			case 2:
				hit = c%3 == 0
			case 3:
				hit = (r+c)%3 == 0
			case 4:
				hit = (r/2+c/3)%2 == 0
			case 5:
				hit = (r*c)%2+(r*c)%3 == 0
			case 6:
				hit = ((r*c)%2+(r*c)%3)%2 == 0
			case 7:
				hit = ((r+c)%2+(r*c)%3)%2 == 0
			}
			if hit {
				m[r][c] = !m[r][c]
			}
		}
	}
}

// yyQRFormatBits 15 位格式信息：等级 M = 00，后接 3 位掩码，BCH(15,5) + 掩码 0x5412。
func yyQRFormatBits(mask int) uint16 {
	d := uint32(mask) & 0x1F
	v := d << 10
	for i := 14; i >= 10; i-- {
		if v&(1<<uint(i)) != 0 {
			v ^= 0x537 << uint(i-10)
		}
	}
	return uint16((d<<10 | v) ^ 0x5412)
}

func yyQRPlaceFormat(m [][]bool, mask int) {
	size := len(m)
	f := yyQRFormatBits(mask)
	bit := func(i int) bool { return (f>>uint(i))&1 == 1 }
	for i := 0; i <= 5; i++ {
		m[i][8] = bit(i)
	}
	m[7][8] = bit(6)
	m[8][8] = bit(7)
	m[8][7] = bit(8)
	for i := 9; i < 15; i++ {
		m[8][14-i] = bit(i)
	}
	for i := 0; i < 8; i++ {
		m[8][size-1-i] = bit(i)
	}
	for i := 8; i < 15; i++ {
		m[size-15+i][8] = bit(i)
	}
}

// yyQRPenalty 标准四条罚分（只影响识别率，不影响合法性）。
func yyQRPenalty(m [][]bool) int {
	n := len(m)
	score := 0
	for a := 0; a < n; a++ {
		for _, horiz := range []bool{true, false} {
			run := 1
			for b := 1; b < n; b++ {
				var cur, prev bool
				if horiz {
					cur, prev = m[a][b], m[a][b-1]
				} else {
					cur, prev = m[b][a], m[b-1][a]
				}
				if cur == prev {
					run++
					continue
				}
				if run >= 5 {
					score += 3 + run - 5
				}
				run = 1
			}
			if run >= 5 {
				score += 3 + run - 5
			}
		}
	}
	for r := 0; r+1 < n; r++ {
		for c := 0; c+1 < n; c++ {
			if m[r][c] == m[r][c+1] && m[r][c] == m[r+1][c] && m[r][c] == m[r+1][c+1] {
				score += 3
			}
		}
	}
	patA := []bool{true, false, true, true, true, false, true, false, false, false, false}
	patB := []bool{false, false, false, false, true, false, true, true, true, false, true}
	match := func(vals []bool, at int, pat []bool) bool {
		for i, p := range pat {
			if vals[at+i] != p {
				return false
			}
		}
		return true
	}
	for a := 0; a < n; a++ {
		row := make([]bool, n)
		col := make([]bool, n)
		for b := 0; b < n; b++ {
			row[b] = m[a][b]
			col[b] = m[b][a]
		}
		for b := 0; b+11 <= n; b++ {
			if match(row, b, patA) || match(row, b, patB) {
				score += 40
			}
			if match(col, b, patA) || match(col, b, patB) {
				score += 40
			}
		}
	}
	dark := 0
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if m[r][c] {
				dark++
			}
		}
	}
	percent := dark * 100 / (n * n)
	dev := percent - 50
	if dev < 0 {
		dev = -dev
	}
	score += (dev / 5) * 10
	return score
}

// yyQRPNG 把文本编成二维码 PNG。scale 是每个模块的像素边长（2~12，默认 8），
// 四周留 4 个模块的静区。
func yyQRPNG(text string, scale int) ([]byte, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("二维码内容为空")
	}
	if scale < 2 {
		scale = 2
	}
	if scale > 12 {
		scale = 12
	}
	m, e := yyQRMatrix(text)
	if e != nil {
		return nil, e
	}
	n := len(m)
	const quiet = 4
	dim := (n + quiet*2) * scale
	img := image.NewGray(image.Rect(0, 0, dim, dim))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	black := color.Gray{Y: 0}
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if !m[r][c] {
				continue
			}
			x0 := (c + quiet) * scale
			y0 := (r + quiet) * scale
			for y := y0; y < y0+scale; y++ {
				for x := x0; x < x0+scale; x++ {
					img.SetGray(x, y, black)
				}
			}
		}
	}
	var buf bytes.Buffer
	if e := png.Encode(&buf, img); e != nil {
		return nil, e
	}
	return buf.Bytes(), nil
}

// ---------- 面板路由 ----------

// yyQQClaimQRText 把「指令 + 码」拼成二维码内容。cmd 为空时退回内置 .认领。
func yyQQClaimQRText(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	cmd := strings.TrimSpace(yyQQCommandText(yyQQSnapshot(), "claim"))
	if cmd == "" {
		cmd = ".认领"
	}
	if strings.HasPrefix(code, cmd) || strings.HasPrefix(code, "/") || strings.HasPrefix(code, "http") {
		return code
	}
	return cmd + " " + code
}

// yyQQClaimQRRoutes 注册 GET /api/qq/claim/qr：
//
//	?text=...     二维码内容（优先）；也接受 ?code=...（自动补成 .认领 码）
//	&scale=2..12  每个模块像素，默认 8
//	&format=json  返回 {ok,text,png:dataURL}，默认直接返回 image/png
//
// 只读、无副作用、内容不回显任何密钥；没有登记进 MCP 只读白名单，
// 外部只读连接拿不到它（认领码属于管理员凭据，不该经 MCP 外露）。
func yyQQClaimQRRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/claim/qr", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "只支持 GET"})
			return
		}
		q := r.URL.Query()
		text := strings.TrimSpace(q.Get("text"))
		if text == "" {
			text = yyQQClaimQRText(q.Get("code"))
		}
		if text == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "缺少 text 或 code"})
			return
		}
		if len([]byte(text)) > yyQRMaxPayload {
			writeJSON(w, map[string]any{"ok": false, "error": fmt.Sprintf("内容太长（%d 字节，上限 %d），请用文本码", len([]byte(text)), yyQRMaxPayload)})
			return
		}
		scale := 8
		if v := strings.TrimSpace(q.Get("scale")); v != "" {
			if n, e := strconv.Atoi(v); e == nil {
				scale = n
			}
		}
		data, e := yyQRPNG(text, scale)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if strings.EqualFold(strings.TrimSpace(q.Get("format")), "json") {
			writeJSON(w, map[string]any{"ok": true, "text": text,
				"png": "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(data)
	})
}
