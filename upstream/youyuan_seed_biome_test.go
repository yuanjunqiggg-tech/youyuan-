package main

// 生物群系（/api/seed/biome）测试。只测纯本地算法与 HTTP 层，不依赖网络/存档。
// 运行：go test -run 'Biome' .

import (
	"math"
	"net/http"
	"testing"
)

// 同一输入必须稳定：两次请求结果逐字一致（算法无包级可变状态）。
func TestBiomeDeterministic(t *testing.T) {
	mux := yySeedTestMux()
	url := "/api/seed/biome?seed=12345&x=100&z=-200&dimension=overworld&version=1.20"
	code1, body1 := yySeedTestGet(t, mux, url)
	code2, body2 := yySeedTestGet(t, mux, url)
	if code1 != http.StatusOK || code2 != http.StatusOK {
		t.Fatalf("状态码异常：%d / %d", code1, code2)
	}
	if body1["ok"] != true {
		t.Fatalf("ok 不为 true：%v", body1)
	}
	if body1["biome"] != body2["biome"] {
		t.Fatalf("同输入结果不稳定：%v vs %v", body1["biome"], body2["biome"])
	}
	if body1["biome"] == "" || body1["biome"] == nil {
		t.Fatalf("biome 为空：%v", body1)
	}
	if body1["name"] == "" || body1["name"] == nil {
		t.Fatalf("name 为空：%v", body1)
	}
}

// 海洋占比必须落在合理区间。
//
// 注意：不能拿「远处坐标一定是海洋」来断言——原版跑到 30 万方块外也可能在陆地上，
// 那种断言只对「算法有偏」的旧实现成立。这里改成统计检验：
// 大样本里海洋类（含深海）占比应在 15%~50%，且不能一种群系吃掉八成面积。
func TestBiomeOceanSharePlausible(t *testing.T) {
	mux := yySeedTestMux()
	oceanSet := map[string]bool{
		"ocean": true, "deep_ocean": true, "frozen_ocean": true, "cold_ocean": true,
		"warm_ocean": true, "lukewarm_ocean": true, "deep_cold_ocean": true,
		"deep_frozen_ocean": true, "deep_lukewarm_ocean": true,
	}
	const n = 24
	total, ocean := 0, 0
	kinds := map[string]int{}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			// 步长取质数 1237 + 偏移：避开噪声整数格点。
			// 用 4096 这种 2 的幂会正好落在格点上（Perlin 在整数格点恒为 0），
			// 测出来是假分布——真实地图按 64~512 方块取样，不会这样。
			x := i*1237 + 37
			z := j*1237 + 53
			code, body := yySeedTestGet(t, mux, "/api/seed/biome?seed=7&x="+itoa(x)+"&z="+itoa(z)+"&version=1.20")
			if code != http.StatusOK || body["ok"] != true {
				t.Fatalf("查询失败：%v", body)
			}
			key, _ := body["biome"].(string)
			total++
			kinds[key]++
			if oceanSet[key] {
				ocean++
			}
		}
	}
	share := float64(ocean) / float64(total)
	if share < 0.15 || share > 0.50 {
		t.Fatalf("海洋占比 %.1f%% 不合理（应在 15%%~50%%），分布：%v", share*100, kinds)
	}
	// 不能有一种群系吃掉八成以上面积
	for k, c := range kinds {
		if float64(c)/float64(total) > 0.80 {
			t.Fatalf("群系 %s 占了 %.0f%%，分布明显退化成单一值：%v", k, float64(c)*100/float64(total), kinds)
		}
	}
	if len(kinds) < 6 {
		t.Fatalf("%d 个采样点只出现 %d 种群系，多样性不足：%v", total, len(kinds), kinds)
	}
}

// 前端取样步长是 2 的幂且常从 0 起，必须保证这种格点上群系仍然有变化。
//
// 回归用例：Perlin 在整数格点恒为 0，若不做偏移，512/1024 步长会让
// 湿度与怪异度整片变 0，地图上出现大片同色块。
func TestBiomePowerOfTwoGridHasDiversity(t *testing.T) {
	for _, step := range []int{64, 128, 256, 512, 1024} {
		seen := map[string]bool{}
		for i := 0; i < 12; i++ {
			for j := 0; j < 12; j++ {
				def, _, _ := yyBiomeAt(12345, int64(i*step), int64(j*step), "overworld", "1.20")
				seen[def.Key] = true
			}
		}
		if len(seen) < 3 {
			t.Fatalf("步长 %d 的 2 的幂格点上只出现 %d 种群系（%v），说明噪声落在格点上退化了",
				step, len(seen), seen)
		}
	}
	// 下界同理
	seen := map[string]bool{}
	for i := 0; i < 16; i++ {
		for j := 0; j < 16; j++ {
			def, _, _ := yyBiomeAt(12345, int64(i*128), int64(j*128), "nether", "1.20")
			seen[def.Key] = true
		}
	}
	if len(seen) < 3 {
		t.Fatalf("下界 128 步长格点上只出现 %d 种群系（%v），斑块退化", len(seen), seen)
	}
}

// 三维度必须给出不同的群系集合（末地恒 the_end；下界只在 5 种群系里）。
func TestBiomeDimensions(t *testing.T) {
	mux := yySeedTestMux()
	seen := map[string]map[string]bool{"overworld": {}, "nether": {}, "the_end": {}}
	for _, dim := range []string{"overworld", "nether", "the_end"} {
		for x := 0; x < 8; x++ {
			for z := 0; z < 8; z++ {
				url := "/api/seed/biome?seed=99&x=" + itoa(x*5000) + "&z=" + itoa(z*5000) +
					"&dimension=" + dim + "&version=1.20"
				code, body := yySeedTestGet(t, mux, url)
				if code != http.StatusOK || body["ok"] != true {
					t.Fatalf("%s 查询失败：%v", dim, body)
				}
				seen[dim][body["biome"].(string)] = true
			}
		}
	}
	// 末地必须全是 the_end
	if len(seen["the_end"]) != 1 || !seen["the_end"]["the_end"] {
		t.Fatalf("末地群系应为单一 the_end，实际 %v", seen["the_end"])
	}
	// 下界只允许 5 种
	allowed := map[string]bool{"nether_wastes": true, "soul_sand_valley": true,
		"crimson_forest": true, "warped_forest": true, "basalt_deltas": true}
	for k := range seen["nether"] {
		if !allowed[k] {
			t.Fatalf("下界出现非法群系 %q", k)
		}
	}
	// 主世界种类必须多于下界（64 个采样点）
	if len(seen["overworld"]) < 5 {
		t.Fatalf("主世界群系种类太少（%d 种），采样 64 点应有明显多样性：%v",
			len(seen["overworld"]), seen["overworld"])
	}
}

// 旧版本（1.12）与新版（1.20）走不同算法，同一坐标允许不同结果，
// 但都必须返回合法的群系 key（不能是空串或 unknown）。
func TestBiomeVersionClasses(t *testing.T) {
	mux := yySeedTestMux()
	for _, ver := range []string{"1.12", "1.16", "1.18", "1.20", "1.21"} {
		url := "/api/seed/biome?seed=2024&x=333&z=777&version=" + ver
		code, body := yySeedTestGet(t, mux, url)
		if code != http.StatusOK || body["ok"] != true {
			t.Fatalf("版本 %s 查询失败：%v", ver, body)
		}
		key, _ := body["biome"].(string)
		if key == "" || key == "unknown" {
			t.Fatalf("版本 %s 返回非法群系 %q", ver, key)
		}
		algo, _ := body["algorithm"].(string)
		if algo == "" {
			t.Fatalf("版本 %s 缺少 algorithm 标注", ver)
		}
	}
}

// 批量模式：点数 = 网格点数，且每点都带坐标与群系。
func TestBiomeBatch(t *testing.T) {
	mux := yySeedTestMux()
	url := "/api/seed/biome?seed=5&from_x=-64&from_z=-64&to_x=64&to_z=64&step=16&version=1.20"
	code, body := yySeedTestGet(t, mux, url)
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("批量查询失败：%v", body)
	}
	pts, ok := body["points"].([]any)
	if !ok || len(pts) == 0 {
		t.Fatalf("points 为空：%v", body)
	}
	// -64..64 步长 16 = 9 个点/轴 → 81 点
	if len(pts) != 81 {
		t.Fatalf("期望 81 点，实际 %d", len(pts))
	}
	for _, raw := range pts {
		p, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("points 元素不是对象：%v", raw)
		}
		if p["biome"] == nil || p["biome"] == "" {
			t.Fatalf("批量点缺少群系：%v", p)
		}
		if _, ok := p["x"]; !ok {
			t.Fatalf("批量点缺少 x：%v", p)
		}
	}
}

// 参数校验：缺 seed / 缺坐标 / 跨度过大 都必须 400，不能 500 也不能静默返回空。
func TestBiomeParamErrors(t *testing.T) {
	mux := yySeedTestMux()
	cases := []string{
		"/api/seed/biome?x=0&z=0",                                     // 缺 seed
		"/api/seed/biome?seed=1",                                      // 缺坐标
		"/api/seed/biome?seed=1&x=0",                                  // 缺 z
		"/api/seed/biome?seed=1&from_x=0&from_z=0&to_x=99999&to_z=1",  // 跨度超限
		"/api/seed/biome?seed=abc&x=0&z=0",                            // seed 非整数
	}
	for _, url := range cases {
		code, body := yySeedTestGet(t, mux, url)
		if code != http.StatusBadRequest {
			t.Fatalf("%s 期望 400，实际 %d（%v）", url, code, body)
		}
		if body["ok"] == true {
			t.Fatalf("%s 不应返回 ok=true", url)
		}
	}
}

// 噪声本身必须落在 [-1,1] 区间：八度叠加后也不该爆掉。
func TestBiomeNoiseRange(t *testing.T) {
	p := yyPerlinNew(42)
	for i := 0; i < 500; i++ {
		x := float64(i)*0.37 - 90
		z := float64(i)*0.11 + 13
		v := p.Noise2D(x, z)
		if math.Abs(v) > 1.0001 {
			t.Fatalf("Perlin 输出越界：%f at (%f,%f)", v, x, z)
		}
	}
	// 八度叠加：振幅总和为上限
	sum := 0.0
	for _, a := range yyContAmp {
		sum += a
	}
	for i := 0; i < 200; i++ {
		v := p.yyOctave(float64(i)*0.05, float64(i)*0.07, -9, yyContAmp)
		if math.Abs(v) > sum+0.001 {
			t.Fatalf("八度叠加越界：%f > %f", v, sum)
		}
	}
}

// 小工具：避免依赖 strconv 的重复引入（测试文件内自足）。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
