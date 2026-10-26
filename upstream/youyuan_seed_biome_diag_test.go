package main

// 群系算法分布诊断：只用来量「气候参数落在 [-1,1] 里的实际分布」，
// 免得阈值凭感觉拍。默认不参与常规跑测（名字以 Diag 开头，跑 -run Diag 才执行）。
//
// 用法：go test -run DiagClimate -v .

import (
	"fmt"
	"math"
	"sort"
	"testing"
)

func yyDiagPct(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	i := int(p * float64(len(s)-1))
	return s[i]
}

func yyDiagReport(t *testing.T, name string, v []float64) {
	mean := 0.0
	for _, x := range v {
		mean += x
	}
	mean /= float64(len(v))
	var sum float64
	for _, x := range v {
		sum += (x - mean) * (x - mean)
	}
	std := math.Sqrt(sum / float64(len(v)))
	t.Log(fmt.Sprintf("%-6s min=%.3f p05=%.3f p25=%.3f p50=%.3f p75=%.3f p95=%.3f max=%.3f mean=%.3f std=%.3f",
		name, yyDiagPct(v, 0), yyDiagPct(v, 0.05), yyDiagPct(v, 0.25), yyDiagPct(v, 0.5),
		yyDiagPct(v, 0.75), yyDiagPct(v, 0.95), yyDiagPct(v, 1), mean, std))
}

func TestDiagClimate(t *testing.T) {
	var cont, ero, temp, hum, weir []float64
	// 150×150 网格、横纵用互质的质数步长（1237 / 1499），覆盖约 18 万 × 22 万方块。
	// 两个坑都要避开：
	//   1) 范围太小 → 只覆盖几个大陆格，占比全是采样噪声；
	//   2) 步长用 2 的幂（512/8192）→ 取样点落在噪声稀疏格点上，
	//      实测会把 cont 的均值从 0 拉到 -0.38，凭空多出 40% 海洋。
	for i := 0; i < 150; i++ {
		for j := 0; j < 150; j++ {
			c := yyClimateAt(12345, int64(i*1237+37), int64(j*1499+53))
			cont = append(cont, c.cont)
			ero = append(ero, c.ero)
			temp = append(temp, c.temp)
			hum = append(hum, c.hum)
			weir = append(weir, c.weir)
		}
	}
	yyDiagReport(t, "cont", cont)
	yyDiagReport(t, "ero", ero)
	yyDiagReport(t, "temp", temp)
	yyDiagReport(t, "hum", hum)
	yyDiagReport(t, "weir", weir)

	// 群系分布
	cnt := map[string]int{}
	for i := 0; i < 150; i++ {
		for j := 0; j < 150; j++ {
			def, _, _ := yyBiomeAt(12345, int64(i*1237+37), int64(j*1499+53), "overworld", "")
			cnt[def.Key]++
		}
	}
	total := 150 * 150
	type kv struct {
		k string
		n int
	}
	var list []kv
	for k, n := range cnt {
		list = append(list, kv{k, n})
	}
	sort.Slice(list, func(a, b int) bool { return list[a].n > list[b].n })
	t.Logf("主世界群系分布（%d 点，%d 种）：", total, len(list))
	for _, it := range list {
		t.Logf("  %-24s %5.1f%%", it.k, float64(it.n)*100/float64(total))
	}

	// 下界分布
	nCnt := map[string]int{}
	for i := 0; i < 64; i++ {
		for j := 0; j < 64; j++ {
			def, _, _ := yyBiomeAt(12345, int64(i*128+11), int64(j*128+17), "nether", "")
			nCnt[def.Key]++
		}
	}
	t.Logf("下界群系分布（%d 点，%d 种）：", 64*64, len(nCnt))
	var nl []kv
	for k, n := range nCnt {
		nl = append(nl, kv{k, n})
	}
	sort.Slice(nl, func(a, b int) bool { return nl[a].n > nl[b].n })
	for _, it := range nl {
		t.Logf("  %-24s %5.1f%%", it.k, float64(it.n)*100/float64(64*64))
	}
}
