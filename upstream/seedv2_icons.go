package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_icons.go —— 结构图标（原创矢量几何，两套出口）
//
//   1. 栅格化：烘焙进 PNG 瓦片（yyV2RenderBakeIcons）
//   2. SVG   ：给前端 DOM 图标层用（GET /api/seed/v2/icon.svg?id=village）
//
// 【版权】全部是自绘几何图形（圆/方/三角/菱形/环/线段），
// 不含任何 chunkbase 素材，可以随便分发改。
//
// 【坐标】每个图标在 0..1 的单位方框里定义，绘制时按 size 缩放。
// ═══════════════════════════════════════════════════════════════════════

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strings"
)

// yyV2IconPrim 一个图元。Pts 的含义按 Kind 不同：
//
//	rect   → x, y, w, h
//	circle → cx, cy, r
//	ring   → cx, cy, r, 线宽
//	line   → x0, y0, x1, y1, 线宽
//	poly   → x0, y0, x1, y1, ...（至少 3 个点）
type yyV2IconPrim struct {
	Kind string
	Pts  []float64
}

func yyV2Rect(x, y, w, h float64) yyV2IconPrim {
	return yyV2IconPrim{"rect", []float64{x, y, w, h}}
}
func yyV2Circle(cx, cy, r float64) yyV2IconPrim {
	return yyV2IconPrim{"circle", []float64{cx, cy, r}}
}
func yyV2Ring(cx, cy, r, w float64) yyV2IconPrim {
	return yyV2IconPrim{"ring", []float64{cx, cy, r, w}}
}
func yyV2Line(x0, y0, x1, y1, w float64) yyV2IconPrim {
	return yyV2IconPrim{"line", []float64{x0, y0, x1, y1, w}}
}
func yyV2Poly(pts ...float64) yyV2IconPrim {
	return yyV2IconPrim{"poly", pts}
}

// yyV2IconRecipes 28 种结构的图形配方。
//
// 设计原则：小尺寸（8~16px）下必须一眼能分出「房子 / 塔 / 圆环 / 三角 / 宝箱」，
// 所以每个图标最多 3 个图元、不用细节。颜色由结构自身的 Color 提供，
// 这里只定义白色前景图形的形状。
var yyV2IconRecipes = map[string][]yyV2IconPrim{
	// 房子系
	"village": {
		yyV2Poly(0.5, 0.18, 0.20, 0.48, 0.80, 0.48), // 屋顶
		yyV2Rect(0.28, 0.48, 0.44, 0.34),            // 屋身
	},
	"witchHut": {
		yyV2Poly(0.5, 0.20, 0.24, 0.46, 0.76, 0.46),
		yyV2Rect(0.34, 0.46, 0.32, 0.24),
		yyV2Line(0.38, 0.70, 0.38, 0.84, 0.06), // 吊脚
		yyV2Line(0.62, 0.70, 0.62, 0.84, 0.06),
	},
	"pillagerOutpost": {
		yyV2Rect(0.40, 0.34, 0.20, 0.50),
		yyV2Poly(0.5, 0.16, 0.32, 0.34, 0.68, 0.34), // 尖顶
		yyV2Line(0.40, 0.52, 0.60, 0.52, 0.05),
	},
	"abandonedCamp": { // 帐篷
		yyV2Poly(0.5, 0.16, 0.16, 0.82, 0.84, 0.82),
		yyV2Line(0.5, 0.16, 0.5, 0.82, 0.04),
	},
	// 多层建筑
	"woodlandMansion": {
		yyV2Rect(0.18, 0.42, 0.64, 0.42),
		yyV2Rect(0.30, 0.22, 0.40, 0.22),
		yyV2Line(0.18, 0.42, 0.82, 0.42, 0.05),
	},
	"ancientCity": { // 方尖碑
		yyV2Poly(0.5, 0.14, 0.36, 0.30, 0.36, 0.84, 0.64, 0.84, 0.64, 0.30),
		yyV2Line(0.42, 0.46, 0.58, 0.46, 0.04),
	},
	// 塔系
	"endCity": {
		yyV2Rect(0.38, 0.30, 0.24, 0.54),
		yyV2Poly(0.5, 0.10, 0.34, 0.30, 0.66, 0.30),
	},
	"bastionRemnant": {
		yyV2Rect(0.24, 0.38, 0.52, 0.46),
		yyV2Line(0.24, 0.38, 0.24, 0.24, 0.06), // 雉堞
		yyV2Line(0.76, 0.38, 0.76, 0.24, 0.06),
		yyV2Line(0.50, 0.38, 0.50, 0.26, 0.06),
	},
	"netherFortress": {
		yyV2Line(0.16, 0.62, 0.84, 0.62, 0.10), // 桥
		yyV2Rect(0.16, 0.30, 0.18, 0.34),
		yyV2Rect(0.66, 0.30, 0.18, 0.34),
	},
	// 三角/金字塔系
	"desertTemple": {
		yyV2Poly(0.5, 0.16, 0.14, 0.84, 0.86, 0.84),
		yyV2Line(0.5, 0.44, 0.5, 0.84, 0.05),
	},
	"jungleTemple": {
		yyV2Poly(0.5, 0.14, 0.30, 0.42, 0.70, 0.42),
		yyV2Rect(0.22, 0.42, 0.56, 0.42),
	},
	// 圆顶/环
	"igloo": {
		yyV2Poly(0.5, 0.30, 0.20, 0.80, 0.80, 0.80),
		yyV2Rect(0.42, 0.56, 0.16, 0.24),
	},
	"oceanMonument": {
		yyV2Rect(0.22, 0.40, 0.56, 0.44),
		yyV2Circle(0.5, 0.34, 0.14),
	},
	"oceanRuin": {
		yyV2Poly(0.24, 0.82, 0.24, 0.40, 0.50, 0.28, 0.76, 0.40, 0.76, 0.82),
		yyV2Line(0.50, 0.28, 0.50, 0.50, 0.05),
	},
	"stronghold": {
		yyV2Ring(0.5, 0.5, 0.34, 0.12),
		yyV2Rect(0.42, 0.16, 0.16, 0.12),
	},
	"desertWell": {
		yyV2Ring(0.5, 0.54, 0.30, 0.11),
		yyV2Line(0.5, 0.12, 0.5, 0.30, 0.05),
	},
	// 船
	"shipwreck": {
		yyV2Poly(0.14, 0.56, 0.86, 0.56, 0.70, 0.82, 0.30, 0.82),
		yyV2Line(0.5, 0.20, 0.5, 0.56, 0.05),
		yyV2Line(0.50, 0.22, 0.74, 0.46, 0.05),
	},
	"endShip": {
		yyV2Poly(0.12, 0.58, 0.88, 0.58, 0.72, 0.84, 0.28, 0.84),
		yyV2Poly(0.5, 0.14, 0.30, 0.58, 0.70, 0.58),
	},
	// 宝藏/宝箱
	"buriedTreasure": {
		yyV2Rect(0.20, 0.40, 0.60, 0.44),
		yyV2Line(0.20, 0.52, 0.80, 0.52, 0.06),
	},
	// 废墟
	"ruinedPortalOW": {
		yyV2Poly(0.24, 0.84, 0.24, 0.36, 0.50, 0.20, 0.76, 0.36, 0.76, 0.84),
		yyV2Line(0.24, 0.36, 0.50, 0.20, 0.05),
	},
	"ruinedPortalNether": {
		yyV2Poly(0.26, 0.84, 0.26, 0.38, 0.50, 0.24, 0.74, 0.38, 0.74, 0.84),
		yyV2Line(0.74, 0.38, 0.74, 0.62, 0.05),
	},
	"trailRuin": {
		yyV2Rect(0.30, 0.46, 0.18, 0.38),
		yyV2Rect(0.56, 0.30, 0.16, 0.54),
	},
	"trialChamber": {
		yyV2Rect(0.18, 0.24, 0.64, 0.56),
		yyV2Rect(0.36, 0.42, 0.28, 0.22),
	},
	// 其它
	"mineshaft": {
		yyV2Line(0.16, 0.30, 0.84, 0.70, 0.08),
		yyV2Line(0.16, 0.70, 0.84, 0.30, 0.08),
	},
	"fossil": {
		yyV2Line(0.20, 0.50, 0.80, 0.50, 0.10),
		yyV2Circle(0.24, 0.34, 0.08),
		yyV2Circle(0.76, 0.66, 0.08),
	},
	"amethystGeode": {
		yyV2Poly(0.5, 0.14, 0.82, 0.50, 0.5, 0.86, 0.18, 0.50),
		yyV2Line(0.36, 0.50, 0.64, 0.50, 0.04),
	},
	"spawn": {
		yyV2Ring(0.5, 0.5, 0.30, 0.09),
		yyV2Poly(0.5, 0.30, 0.40, 0.56, 0.60, 0.56),
	},
	"slimeChunk": {
		yyV2Rect(0.20, 0.34, 0.60, 0.46),
		yyV2Circle(0.36, 0.48, 0.06),
		yyV2Circle(0.64, 0.48, 0.06),
	},
}

// yyV2IconFallback 未知结构 id 用的通用图钉，别返回空。
var yyV2IconFallback = []yyV2IconPrim{
	yyV2Circle(0.5, 0.5, 0.30),
}

// yyV2IconRecipe 取配方（带 nil 兜底）。
func yyV2IconRecipe(id string) []yyV2IconPrim {
	if p, ok := yyV2IconRecipes[id]; ok && len(p) > 0 {
		return p
	}
	return yyV2IconFallback
}

// yyV2IconHasRecipe 给测试/自检用。
func yyV2IconHasRecipe(id string) bool {
	p, ok := yyV2IconRecipes[id]
	return ok && len(p) > 0
}

// ── 栅格化 ────────────────────────────────────────────────────────────

// yyV2DrawIcon 在 img 的 (left, top) 起 size×size 区域画一个图标：
// 圆形底衬（结构色）+ 深色描边 + 白色前景图形。
func yyV2DrawIcon(img *image.RGBA, left, top, size int, fill [3]uint8, prims []yyV2IconPrim) {
	if size <= 0 {
		return
	}
	bounds := img.Bounds()
	cx := float64(left) + float64(size)/2
	cy := float64(top) + float64(size)/2
	radius := float64(size)/2 - 0.5
	if radius <= 0.5 {
		radius = 0.5
	}
	base := color.RGBA{fill[0], fill[1], fill[2], 255}
	edge := color.RGBA{uint8(int(fill[0]) / 3), uint8(int(fill[1]) / 3), uint8(int(fill[2]) / 3), 255}
	white := color.RGBA{255, 255, 255, 255}

	// 2× 超采样抗锯齿：每个像素取 4 个子采样点
	const ss = 2
	inv := 1.0 / float64(size)
	sub := 0.5 / float64(ss)
	for py := 0; py < size; py++ {
		y0 := top + py
		if y0 < bounds.Min.Y || y0 >= bounds.Max.Y {
			continue
		}
		for px := 0; px < size; px++ {
			x0 := left + px
			if x0 < bounds.Min.X || x0 >= bounds.Max.X {
				continue
			}
			var coverBase, coverEdge, coverFg int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/ss
					fy := float64(py) + (float64(sy)+0.5)/ss
					dx := fx - float64(size)/2
					dy := fy - float64(size)/2
					dist := math.Hypot(dx, dy)
					switch {
					case dist > radius:
						continue
					case dist > radius-1.2:
						coverEdge++
						continue
					default:
						coverBase++
					}
					// 前景：单位坐标
					ux := (fx - 0.5) * inv / 0.84
					uy := (fy - 0.5) * inv / 0.84
					ux += 0.5
					uy += 0.5
					if yyV2IconHit(prims, ux, uy, 1.0/math.Max(1, float64(size)*0.84)) {
						coverFg++
					}
				}
			}
			total := float64(ss * ss)
			if coverBase+coverEdge == 0 {
				continue
			}
			c := base
			if coverEdge > 0 && coverFg == 0 {
				c = edge
			}
			if coverFg > 0 {
				c = yyV2MixColor(base, white, float64(coverFg)/total*2.2)
			}
			alpha := (float64(coverBase+coverEdge) / total) * 255
			if alpha < 8 {
				continue
			}
			if alpha >= 250 {
				img.SetRGBA(x0, y0, c)
			} else {
				img.SetRGBA(x0, y0, yyV2Blend(img.RGBAAt(x0, y0), c, alpha/255))
			}
		}
	}
	_ = cx
	_ = cy
	_ = sub
}

func yyV2MixColor(a, b color.RGBA, t float64) color.RGBA {
	if t > 1 {
		t = 1
	}
	return color.RGBA{
		uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		255,
	}
}

func yyV2Blend(dst, src color.RGBA, a float64) color.RGBA {
	return color.RGBA{
		uint8(float64(src.R)*a + float64(dst.R)*(1-a)),
		uint8(float64(src.G)*a + float64(dst.G)*(1-a)),
		uint8(float64(src.B)*a + float64(dst.B)*(1-a)),
		255,
	}
}

// yyV2IconHit 判断单位坐标 (x,y) 是否落在任一图元内。
// eps 是线宽补偿（图标越小线要越粗，否则糊掉）。
func yyV2IconHit(prims []yyV2IconPrim, x, y, eps float64) bool {
	for _, p := range prims {
		switch p.Kind {
		case "rect":
			if x >= p.Pts[0] && x <= p.Pts[0]+p.Pts[2] && y >= p.Pts[1] && y <= p.Pts[1]+p.Pts[3] {
				return true
			}
		case "circle":
			if math.Hypot(x-p.Pts[0], y-p.Pts[1]) <= p.Pts[2] {
				return true
			}
		case "ring":
			d := math.Hypot(x-p.Pts[0], y-p.Pts[1])
			if math.Abs(d-p.Pts[2]) <= p.Pts[3]/2+eps {
				return true
			}
		case "line":
			if yyV2DistToSegment(x, y, p.Pts[0], p.Pts[1], p.Pts[2], p.Pts[3]) <= p.Pts[4]/2+eps {
				return true
			}
		case "poly":
			if yyV2PointInPoly(x, y, p.Pts) {
				return true
			}
		}
	}
	return false
}

func yyV2DistToSegment(px, py, x0, y0, x1, y1 float64) float64 {
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(px-x0, py-y0)
	}
	t := ((px-x0)*dx + (py-y0)*dy) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(px-(x0+t*dx), py-(y0+t*dy))
}

// yyV2PointInPoly 射线法（点在图元内部返回 true）。
func yyV2PointInPoly(x, y float64, pts []float64) bool {
	n := len(pts) / 2
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := pts[i*2], pts[i*2+1]
		xj, yj := pts[j*2], pts[j*2+1]
		if (yi > y) != (yj > y) {
			xCross := (xj-xi)*(y-yi)/(yj-yi) + xi
			if x < xCross {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

// ── SVG 出口 ──────────────────────────────────────────────────────────

// yyV2StructureIconSVG 给前端 DOM 图标层用。
// 返回完整 <svg ...>...</svg>，viewBox 0 0 size size，不引外部字体/图片。
func yyV2StructureIconSVG(id string, size int) string {
	if size <= 0 {
		size = 20
	}
	if size > 128 {
		size = 128
	}
	fill := [3]uint8{120, 120, 120}
	if info, ok := yyV2StructureInfoByID(id, 1<<30, yyV2EditionJava); ok {
		fill = info.Color
	}
	hex := fmt.Sprintf("#%02x%02x%02x", fill[0], fill[1], fill[2])
	edge := fmt.Sprintf("#%02x%02x%02x", fill[0]/3, fill[1]/3, fill[2]/3)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, size, size, size, size)
	r := float64(size)/2 - 0.5
	fmt.Fprintf(&b, `<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s" stroke="%s" stroke-width="%.2f"/>`,
		float64(size)/2, float64(size)/2, r, hex, edge, math.Max(1, float64(size)*0.06))

	// 前景图形画在中间 84% 的区域
	inner := float64(size) * 0.84
	off := (float64(size) - inner) / 2
	sc := func(v float64) float64 { return off + v*inner }
	sw := math.Max(1, float64(size)*0.09)

	for _, p := range yyV2IconRecipe(id) {
		switch p.Kind {
		case "rect":
			fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="#fff"/>`,
				sc(p.Pts[0]), sc(p.Pts[1]), p.Pts[2]*inner, p.Pts[3]*inner)
		case "circle":
			fmt.Fprintf(&b, `<circle cx="%.2f" cy="%.2f" r="%.2f" fill="#fff"/>`,
				sc(p.Pts[0]), sc(p.Pts[1]), p.Pts[2]*inner)
		case "ring":
			fmt.Fprintf(&b, `<circle cx="%.2f" cy="%.2f" r="%.2f" fill="none" stroke="#fff" stroke-width="%.2f"/>`,
				sc(p.Pts[0]), sc(p.Pts[1]), p.Pts[2]*inner, math.Max(1, p.Pts[3]*inner))
		case "line":
			fmt.Fprintf(&b, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#fff" stroke-width="%.2f" stroke-linecap="round"/>`,
				sc(p.Pts[0]), sc(p.Pts[1]), sc(p.Pts[2]), sc(p.Pts[3]), math.Max(1, p.Pts[4]*inner))
		case "poly":
			var pts []string
			for i := 0; i+1 < len(p.Pts); i += 2 {
				pts = append(pts, fmt.Sprintf("%.2f,%.2f", sc(p.Pts[i]), sc(p.Pts[i+1])))
			}
			fmt.Fprintf(&b, `<polygon points="%s" fill="#fff"/>`, strings.Join(pts, " "))
		}
	}
	_ = sw
	b.WriteString(`</svg>`)
	return b.String()
}

// ── 把图标烘焙进瓦片 ──────────────────────────────────────────────────

// yyV2IconPriority 图标过多时保留谁：数越小越优先。
func yyV2IconPriority(id string) int {
	switch id {
	case "woodlandMansion", "stronghold", "oceanMonument", "endCity", "ancientCity":
		return 0
	case "village", "pillagerOutpost", "netherFortress", "bastionRemnant", "trialChamber", "endShip":
		return 1
	}
	return 2
}

// yyV2IconSizeForBPP 图标像素尺寸：视野越粗，图标越小，避免糊成一片。
func yyV2IconSizeForBPP(bpp int) int {
	switch {
	case bpp <= 1:
		return 16
	case bpp <= 2:
		return 15
	case bpp <= 4:
		return 14
	case bpp <= 8:
		return 12
	case bpp <= 16:
		return 10
	}
	return 8
}

// yyV2RenderBakeIcons 把结构图标画到瓦片上。
//
// 图标锚点：结构所在**区块**的中心方块 → 像素。
//
//	px = (chunkX*16 + 8 - CX*16) / bpp = (chunkX-CX)*16/bpp + 4/bpp
func yyV2RenderBakeIcons(img *image.RGBA, opt yyV2RenderOptions) {
	ids := opt.StructureIDs
	if len(ids) == 0 {
		ids = append(ids, yySeedV2DefaultTypes[opt.Dimension]...)
	}
	if len(ids) == 0 {
		return
	}

	spanBlocks := opt.W * opt.BlocksPerPixel
	cx0 := opt.CX
	cz0 := opt.CZ
	cx1 := opt.CX + spanBlocks/16
	cz1 := opt.CZ + opt.H*opt.BlocksPerPixel/16

	type placed struct {
		px, py int
		color  [3]uint8
		prims  []yyV2IconPrim
		prio   int
		seq    int
	}
	var all []placed
	seq := 0
	for _, id := range ids {
		limit := yyV2RenderScanHardLimit
		pts := yyV2ScanStructures(opt.Seed, id, opt.VersionCode, opt.Edition, cx0, cz0, cx1, cz1, limit)
		if len(pts) == 0 {
			continue
		}
		info, ok := yyV2StructureInfoByID(id, opt.VersionCode, opt.Edition)
		if !ok {
			continue
		}
		prims := yyV2IconRecipe(info.ID)
		prio := yyV2IconPriority(info.ID)
		for _, p := range pts {
			px := (p.X-opt.CX)*16/opt.BlocksPerPixel + 8/opt.BlocksPerPixel
			py := (p.Z-opt.CZ)*16/opt.BlocksPerPixel + 8/opt.BlocksPerPixel
			if px < 0 || py < 0 || px >= opt.W || py >= opt.H {
				continue
			}
			all = append(all, placed{px, py, info.Color, prims, prio, seq})
			seq++
		}
	}
	if len(all) == 0 {
		return
	}

	// 超上限时按优先级裁剪（同优先级保留先出现的，结果稳定）
	if len(all) > opt.MaxIcons {
		sort.SliceStable(all, func(i, j int) bool {
			if all[i].prio != all[j].prio {
				return all[i].prio < all[j].prio
			}
			return all[i].seq < all[j].seq
		})
		all = all[:opt.MaxIcons]
		sort.SliceStable(all, func(i, j int) bool { return all[i].seq < all[j].seq })
	}

	size := yyV2IconSizeForBPP(opt.BlocksPerPixel)
	for _, it := range all {
		left := it.px - size/2
		top := it.py - size/2
		yyV2DrawIcon(img, left, top, size, it.color, it.prims)
	}
}
