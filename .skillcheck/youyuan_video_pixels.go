package main

import "math"

// OKLab compares perceptual lightness/chroma instead of treating dark blue and
// bright green as equivalent RGB errors. Palette values remain explicit, not
// a claim of photorealism with only sixteen vanilla concrete colours.
type yyLab [3]float64

func yyRGBLab(r, g, b int) yyLab {
	linear := func(v int) float64 {
		x := float64(v) / 255
		if x <= .04045 {
			return x / 12.92
		}
		return math.Pow((x+.055)/1.055, 2.4)
	}
	rr, gg, bb := linear(r), linear(g), linear(b)
	l := math.Cbrt(.4122214708*rr + .5363325363*gg + .0514459929*bb)
	m := math.Cbrt(.2119034982*rr + .6806995451*gg + .1073969566*bb)
	s := math.Cbrt(.0883024619*rr + .2817188376*gg + .6299787005*bb)
	return yyLab{.2104542553*l + .793617785*m - .0040720468*s, 1.9779984951*l - 2.428592205*m + .4505937099*s, .0259040371*l + .7827717662*m - .808675766*s}
}

var yyVideoPaletteLab = func() []yyLab {
	out := []yyLab{}
	for _, c := range yyVideoColors {
		out = append(out, yyRGBLab(c[0], c[1], c[2]))
	}
	return out
}()

func yyVideoNearest(r, g, b int) int {
	l := yyRGBLab(r, g, b)
	best := math.Inf(1)
	index := 0
	for i, c := range yyVideoPaletteLab {
		d := 0.
		for n := range l {
			delta := l[n] - c[n]
			d += delta * delta
		}
		if d < best {
			best = d
			index = i
		}
	}
	return index
}

type yyVideoRect struct{ X, Y, W, H, Color int }

func yyVideoRectangles(cells, previous []int, width, height int) []yyVideoRect {
	used := make([]bool, len(cells))
	out := []yyVideoRect{}
	changed := func(i int) bool { return !used[i] && (len(previous) != len(cells) || previous[i] != cells[i]) }
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			if !changed(i) {
				continue
			}
			color := cells[i]
			w := 1
			for x+w < width && changed(i+w) && cells[i+w] == color {
				w++
			}
			h := 1
			for y+h < height {
				ok := true
				for dx := 0; dx < w; dx++ {
					j := (y+h)*width + x + dx
					if !changed(j) || cells[j] != color {
						ok = false
						break
					}
				}
				if !ok {
					break
				}
				h++
			}
			// Respect vanilla fill volume without changing output pixels.
			maxRows := 32768 / w
			if h > maxRows {
				h = maxRows
			}
			for dy := 0; dy < h; dy++ {
				for dx := 0; dx < w; dx++ {
					used[(y+dy)*width+x+dx] = true
				}
			}
			out = append(out, yyVideoRect{x, y, w, h, color})
		}
	}
	return out
}
