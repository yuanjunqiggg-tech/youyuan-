package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_rand_ref_test.go —— 与 Node 参考实现（D:/种子查询器自研/src）逐项对拍
//
// 基准文件：testdata/yy_v2_rand_ref.json
//   由 scripts 里的一次性生成脚本产出（Node 直接跑参考 .mjs，导出 JSON）。
//   覆盖：Java LCG 全部方法 / Xoroshiro128++ / Perlin / DoublePerlin /
//         climate 六轴（含 Large Biomes）/ Voronoi SHA + 3D 访问。
//
// 跑法（只用跑这一个，别跑全量）：
//   go test -count=1 -run TestSeedV2Rand .
// ═══════════════════════════════════════════════════════════════════════

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

type yyV2RandRefFile struct {
	Anchor struct {
		Seed     string `json:"seed"`
		NextLong string `json:"nextLong"`
	} `json:"anchor"`
	RNG []struct {
		Seed  string              `json:"seed"`
		Ops   [][]json.RawMessage `json:"ops"`
		Vals  []json.RawMessage   `json:"vals"`
		State string              `json:"state"`
	} `json:"rng"`
	Xoro []struct {
		Seed  string              `json:"seed"`
		Ops   [][]json.RawMessage `json:"ops"`
		Vals  []json.RawMessage   `json:"vals"`
		State []string            `json:"state"`
	} `json:"xoro"`
	Perlin []struct {
		Seed   string      `json:"seed"`
		Points [][]float64 `json:"points"`
		Amp    float64     `json:"amp"`
		Lac    float64     `json:"lac"`
	} `json:"perlin"`
	DoublePerlin []struct {
		Seed      string      `json:"seed"`
		Amp       []float64   `json:"amp"`
		Omin      int         `json:"omin"`
		Len       int         `json:"len"`
		Amplitude float64     `json:"amplitude"`
		Points    [][]float64 `json:"points"`
	} `json:"doubleperlin"`
	Climate []struct {
		Seed   string              `json:"seed"`
		Large  bool                `json:"large"`
		Points [][]json.RawMessage `json:"points"`
	} `json:"climate"`
	Voronoi []struct {
		Seed   string              `json:"seed"`
		SHA    string              `json:"sha"`
		Access [][]json.RawMessage `json:"access"`
	} `json:"voronoi"`
}

func yyV2RandLoadRef(t *testing.T) *yyV2RandRefFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/yy_v2_rand_ref.json")
	if err != nil {
		t.Fatalf("读基准文件失败：%v", err)
	}
	var f yyV2RandRefFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("解析基准文件失败：%v", err)
	}
	return &f
}

func yyV2RandRefStr(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func yyV2RandRefInt64(t *testing.T, raw json.RawMessage) int64 {
	t.Helper()
	v, err := strconv.ParseInt(yyV2RandRefStr(t, raw), 10, 64)
	if err != nil {
		t.Fatalf("基准值不是 int64：%s", string(raw))
	}
	return v
}

func yyV2RandRefUint64(t *testing.T, raw json.RawMessage) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(yyV2RandRefStr(t, raw), 10, 64)
	if err != nil {
		t.Fatalf("基准值不是 uint64：%s", string(raw))
	}
	return v
}

func yyV2RandRefInt(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	v, err := strconv.Atoi(yyV2RandRefStr(t, raw))
	if err != nil {
		t.Fatalf("基准值不是 int：%s", string(raw))
	}
	return v
}

func yyV2RandRefFloat(t *testing.T, raw json.RawMessage) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(yyV2RandRefStr(t, raw), 64)
	if err != nil {
		t.Fatalf("基准值不是 float：%s", string(raw))
	}
	return v
}

// yyV2RandSameFloat 要求逐位相同（NaN 不算相同，这里不会出现）。
func yyV2RandSameFloat(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

// ── 锚点 ────────────────────────────────────────────────────────────────

func TestSeedV2RandAnchor(t *testing.T) {
	const want = int64(-4962768465676381896)
	if got := yyV2NewRNG(0).NextLong(); got != want {
		t.Fatalf("new JRandom(0).nextLong() = %d，应为 %d", got, want)
	}
	f := yyV2RandLoadRef(t)
	if got := strconv.FormatInt(want, 10); got != f.Anchor.NextLong {
		t.Fatalf("基准文件锚点不一致：%s vs %s", got, f.Anchor.NextLong)
	}
}

// ── Java LCG ────────────────────────────────────────────────────────────

func TestSeedV2RandJavaLCG(t *testing.T) {
	f := yyV2RandLoadRef(t)
	if len(f.RNG) == 0 {
		t.Fatal("基准文件没有 rng 用例")
	}
	var nLong, nInt, nDouble int
	for _, c := range f.RNG {
		seed, err := strconv.ParseInt(c.Seed, 10, 64)
		if err != nil {
			t.Fatalf("种子 %q 解析失败", c.Seed)
		}
		r := yyV2NewRNG(seed)
		if len(c.Ops) != len(c.Vals) {
			t.Fatalf("seed=%s: ops/vals 长度不一致", c.Seed)
		}
		for i, op := range c.Ops {
			kind := yyV2RandRefStr(t, op[0])
			switch kind {
			case "l":
				want := yyV2RandRefInt64(t, c.Vals[i])
				if got := r.NextLong(); got != want {
					t.Fatalf("seed=%s op#%d nextLong = %d，应为 %d", c.Seed, i, got, want)
				}
				nLong++
			case "i":
				bound := int32(yyV2RandRefInt(t, op[1]))
				want := int32(yyV2RandRefInt(t, c.Vals[i]))
				if got := r.NextInt(bound); got != want {
					t.Fatalf("seed=%s op#%d nextInt(%d) = %d，应为 %d", c.Seed, i, bound, got, want)
				}
				nInt++
			case "d":
				want := yyV2RandRefFloat(t, c.Vals[i])
				if got := r.NextDouble(); !yyV2RandSameFloat(got, want) {
					t.Fatalf("seed=%s op#%d nextDouble = %.20g，应为 %.20g", c.Seed, i, got, want)
				}
				nDouble++
			default:
				t.Fatalf("未知 op：%q", kind)
			}
		}
		wantState := yyV2RandRefUint64(t, json.RawMessage(strconv.Quote(c.State)))
		if got := r.State(); got != wantState {
			t.Fatalf("seed=%s 终态 = %d，应为 %d", c.Seed, got, wantState)
		}
	}
	t.Logf("Java LCG 对拍：%d 组种子，nextLong %d 次 / nextInt %d 次 / nextDouble %d 次",
		len(f.RNG), nLong, nInt, nDouble)
}

// ── Xoroshiro128++ ─────────────────────────────────────────────────────

func TestSeedV2RandXoroshiro(t *testing.T) {
	f := yyV2RandLoadRef(t)
	var nLong, nInt, nDouble int
	for _, c := range f.Xoro {
		seed, err := strconv.ParseUint(c.Seed, 10, 64)
		if err != nil {
			// 负数种子：先按 int64 解析再转 uint64
			signed, err2 := strconv.ParseInt(c.Seed, 10, 64)
			if err2 != nil {
				t.Fatalf("种子 %q 解析失败", c.Seed)
			}
			seed = uint64(signed)
		}
		xr := yyV2RandXSetSeed(seed)
		for i, op := range c.Ops {
			kind := yyV2RandRefStr(t, op[0])
			switch kind {
			case "l":
				want := yyV2RandRefUint64(t, c.Vals[i])
				if got := xr.nextLong(); got != want {
					t.Fatalf("seed=%s op#%d xNextLong = %d，应为 %d", c.Seed, i, got, want)
				}
				nLong++
			case "i":
				bound := uint32(yyV2RandRefInt(t, op[1]))
				want := uint32(yyV2RandRefInt(t, c.Vals[i]))
				if got := xr.nextInt(bound); got != want {
					t.Fatalf("seed=%s op#%d xNextInt(%d) = %d，应为 %d", c.Seed, i, bound, got, want)
				}
				nInt++
			case "d":
				want := yyV2RandRefFloat(t, c.Vals[i])
				if got := xr.nextDouble(); !yyV2RandSameFloat(got, want) {
					t.Fatalf("seed=%s op#%d xNextDouble = %.20g，应为 %.20g", c.Seed, i, got, want)
				}
				nDouble++
			}
		}
		if len(c.State) == 2 {
			if got, want := xr.lo, yyV2RandRefUint64(t, json.RawMessage(strconv.Quote(c.State[0]))); got != want {
				t.Fatalf("seed=%s lo 终态 = %d，应为 %d", c.Seed, got, want)
			}
			if got, want := xr.hi, yyV2RandRefUint64(t, json.RawMessage(strconv.Quote(c.State[1]))); got != want {
				t.Fatalf("seed=%s hi 终态 = %d，应为 %d", c.Seed, got, want)
			}
		}
	}
	t.Logf("Xoroshiro 对拍：%d 组种子，nextLong %d 次 / nextInt %d 次 / nextDouble %d 次",
		len(f.Xoro), nLong, nInt, nDouble)
}

// ── Perlin ──────────────────────────────────────────────────────────────

func TestSeedV2RandPerlin(t *testing.T) {
	f := yyV2RandLoadRef(t)
	var n int
	for _, c := range f.Perlin {
		seed, _ := strconv.ParseInt(c.Seed, 10, 64)
		xr := yyV2RandXSetSeed(uint64(seed))
		p := yyV2RandPerlinInit(&xr)
		if !yyV2RandSameFloat(p.amplitude, c.Amp) || !yyV2RandSameFloat(p.lacunarity, c.Lac) {
			t.Fatalf("seed=%s perlin amp/lac 不一致：%v/%v", c.Seed, p.amplitude, p.lacunarity)
		}
		for _, pt := range c.Points {
			got := yyV2RandSamplePerlin(p, pt[0], pt[1], pt[2], 0, 0)
			if !yyV2RandSameFloat(got, pt[3]) {
				t.Fatalf("seed=%s perlin(%.6g,%.6g,%.6g) = %.20g，应为 %.20g",
					c.Seed, pt[0], pt[1], pt[2], got, pt[3])
			}
			n++
		}
	}
	t.Logf("Perlin 对拍：%d 组种子 / %d 个采样点", len(f.Perlin), n)
}

// ── DoublePerlin ────────────────────────────────────────────────────────

func TestSeedV2RandDoublePerlin(t *testing.T) {
	f := yyV2RandLoadRef(t)
	var n int
	for _, c := range f.DoublePerlin {
		seed, _ := strconv.ParseInt(c.Seed, 10, 64)
		xr := yyV2RandXSetSeed(uint64(seed))
		xr.nextLong() // buildClimate / 参考脚本都先消耗两次 nextLong
		xr.nextLong()
		dp := yyV2RandDoublePerlinInit(&xr, c.Amp, c.Omin, c.Len, -1)
		if !yyV2RandSameFloat(dp.amplitude, c.Amplitude) {
			t.Fatalf("seed=%s amp 不一致：%.20g 应为 %.20g", c.Seed, dp.amplitude, c.Amplitude)
		}
		for _, pt := range c.Points {
			got := yyV2RandSampleDoublePerlin(dp, pt[0], pt[1], pt[2])
			if !yyV2RandSameFloat(got, pt[3]) {
				t.Fatalf("seed=%s doublePerlin(%.6g,%.6g,%.6g) = %.20g，应为 %.20g",
					c.Seed, pt[0], pt[1], pt[2], got, pt[3])
			}
			n++
		}
	}
	t.Logf("DoublePerlin 对拍：%d 组噪声 / %d 个采样点", len(f.DoublePerlin), n)
}

// ── climate 六轴 ────────────────────────────────────────────────────────

func TestSeedV2RandClimateAxes(t *testing.T) {
	f := yyV2RandLoadRef(t)
	var n int
	for _, c := range f.Climate {
		seed, _ := strconv.ParseInt(c.Seed, 10, 64)
		cl := yyV2NewClimate(seed, c.Large)
		for _, pt := range c.Points {
			x := yyV2RandRefFloat(t, pt[0])
			z := yyV2RandRefFloat(t, pt[1])
			var want [6]int32
			var raw []json.RawMessage
			if err := json.Unmarshal(pt[2], &raw); err != nil {
				t.Fatalf("climate 期望值解析失败：%v", err)
			}
			for i := 0; i < 6; i++ {
				want[i] = int32(yyV2RandRefInt(t, raw[i]))
			}
			got := cl.Axes(x, z)
			if got != want {
				t.Fatalf("seed=%s large=%v Axes(%.6g,%.6g) = %v，应为 %v",
					c.Seed, c.Large, x, z, got, want)
			}
			n++
		}
	}
	t.Logf("climate 对拍：%d 组（种子×large）/ %d 个坐标，6 轴全部一致", len(f.Climate), n)
}

// ── Voronoi ─────────────────────────────────────────────────────────────

func TestSeedV2RandVoronoi(t *testing.T) {
	f := yyV2RandLoadRef(t)
	var n int
	for _, c := range f.Voronoi {
		seed, err := strconv.ParseInt(c.Seed, 10, 64)
		if err != nil {
			t.Fatalf("种子 %q 解析失败", c.Seed)
		}
		sha := yyV2VoronoiSHA(seed)
		wantSHA, err := strconv.ParseUint(c.SHA, 10, 64)
		if err != nil {
			t.Fatalf("sha 解析失败：%v", err)
		}
		if sha != wantSHA {
			t.Fatalf("seed=%s voronoiSHA = %d，应为 %d", c.Seed, sha, wantSHA)
		}
		for _, a := range c.Access {
			x := yyV2RandRefInt(t, a[0])
			y := yyV2RandRefInt(t, a[1])
			z := yyV2RandRefInt(t, a[2])
			var raw []json.RawMessage
			if err := json.Unmarshal(a[3], &raw); err != nil {
				t.Fatalf("access 期望值解析失败：%v", err)
			}
			gx, gy, gz := yyV2VoronoiAccess3D(sha, x, y, z)
			wx, wy, wz := yyV2RandRefInt(t, raw[0]), yyV2RandRefInt(t, raw[1]), yyV2RandRefInt(t, raw[2])
			if gx != wx || gy != wy || gz != wz {
				t.Fatalf("seed=%s access3D(%d,%d,%d) = (%d,%d,%d)，应为 (%d,%d,%d)",
					c.Seed, x, y, z, gx, gy, gz, wx, wy, wz)
			}
			n++
		}
	}
	t.Logf("Voronoi 对拍：%d 组种子 SHA / %d 个 3D 格点", len(f.Voronoi), n)
}
