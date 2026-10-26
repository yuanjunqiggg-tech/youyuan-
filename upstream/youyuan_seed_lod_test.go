package main

// LOD（分级细节）与热点图测试。
// 运行：go test -run 'Seed' .

import (
	"net/http"
	"sort"
	"testing"
	"time"
)

// 远景（lod=0）：大范围必须走抽样，每个结构类型最多 40 个点，并如实标注 sampled / skipped_by_lod。
func TestSeedLODFarSampled(t *testing.T) {
	mux := yySeedTestMux()
	url := "/api/seed/structure?seed=0&type=village&from_x=-2048&from_z=-2048&to_x=2048&to_z=2048&lod=0"
	code, body := yySeedTestGet(t, mux, url)
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("状态码 %d，body %v", code, body)
	}
	if int(body["lod"].(float64)) != yySeedLODFar {
		t.Fatalf("lod 回显应为 0，实际 %v", body["lod"])
	}
	if body["sampled"] != true {
		t.Fatalf("大范围 lod=0 必须标记 sampled=true：%v", body["sampled"])
	}
	if body["degraded"] != false {
		t.Fatalf("4096 跨度未超工作量上限，不应 degraded：%v", body["degraded"])
	}
	count := int(body["count"].(float64))
	if count > yySeedLODLimitFar {
		t.Fatalf("lod=0 结果数 %d 超过上限 %d", count, yySeedLODLimitFar)
	}
	if count != len(body["chunks"].([]any)) {
		t.Fatal("count 与 chunks 长度不一致")
	}
	if int(body["skipped_by_lod"].(float64)) <= 0 {
		t.Fatalf("抽样时必须告知还有多少没画，实际 %v", body["skipped_by_lod"])
	}
	if body["precision"] != "candidate" {
		t.Fatalf("抽样不改精度标注，仍应为 candidate，实际 %v", body["precision"])
	}
	if body["truncated"] != true {
		t.Fatalf("抽样即有遗漏，truncated 应为 true：%v", body["truncated"])
	}

	// 不传 lod 时默认就是远景（lod=0），行为必须与显式 lod=0 一致
	_, def := yySeedTestGet(t, mux, "/api/seed/structure?seed=0&type=village&from_x=-2048&from_z=-2048&to_x=2048&to_z=2048")
	if int(def["lod"].(float64)) != yySeedLODFar || def["sampled"] != true {
		t.Fatalf("缺省 lod 应等价于远景抽样，实际 lod=%v sampled=%v", def["lod"], def["sampled"])
	}
}

// 中景（lod=1）：上限 120，正常网格不抽样。
func TestSeedLODMidLimit(t *testing.T) {
	mux := yySeedTestMux()
	code, body := yySeedTestGet(t, mux, "/api/seed/structure?seed=0&type=village&from_x=-2048&from_z=-2048&to_x=2048&to_z=2048&lod=1")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("状态码 %d，body %v", code, body)
	}
	if body["sampled"] != false {
		t.Fatalf("lod=1 不应抽样：%v", body["sampled"])
	}
	if count := int(body["count"].(float64)); count > yySeedLODLimitMid {
		t.Fatalf("lod=1 结果数 %d 超过上限 %d", count, yySeedLODLimitMid)
	}
}

// 工作量超限：区域数 > 60000 时强制降到 lod=0 抽样，并给出中文原因。
// 注意 HTTP 层单轴跨度上限 4096（6 类结构合计最多约 9.6 万区域），所以这里直接调用核心函数，
// 覆盖「未来放宽范围上限 / 内部调用」时的安全网。
func TestSeedLODDegradeByWorkload(t *testing.T) {
	spec, ok := yySeedStructureSpecByID("village")
	if !ok {
		t.Fatal("缺少 village 结构定义")
	}
	start := time.Now()
	res := yySeedStructureScanEx(0, spec, -40000, -40000, 40000, 40000, yySeedLODNear, 0, time.Now().Add(yySeedQueryBudget))
	elapsed := time.Since(start)

	if res.Regions <= yySeedMaxStructureRegs {
		t.Fatalf("构造的范围区域数 %d 未超过阈值 %d", res.Regions, yySeedMaxStructureRegs)
	}
	if !res.Degraded || res.Reason == "" {
		t.Fatalf("工作量超限必须降级并给出原因，实际 degraded=%v reason=%q", res.Degraded, res.Reason)
	}
	if res.LOD != yySeedLODFar || !res.Sampled {
		t.Fatalf("降级后应为 lod=0 且抽样，实际 lod=%d sampled=%v", res.LOD, res.Sampled)
	}
	if len(res.Chunks) > yySeedLODLimitFar {
		t.Fatalf("降级后结果数 %d 超过远景上限 %d", len(res.Chunks), yySeedLODLimitFar)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("超工作量范围耗时 %v，超过 200ms", elapsed)
	}

	// 聚合查询同样逐类型套用 LOD，且能透出 degraded/sampled 字段
	_, body := yySeedTestGet(t, yySeedTestMux(), "/api/seed/query?seed=7&x=0&z=0&radius=32&lod=2")
	if _, ok := body["lod"]; !ok {
		t.Fatalf("query 缺少 lod 字段：%v", body)
	}
	if _, ok := body["sampled"]; !ok {
		t.Fatalf("query 缺少 sampled 字段：%v", body)
	}
	for _, raw := range body["structures"].([]any) {
		s := raw.(map[string]any)
		if _, ok := s["lod"]; !ok {
			t.Fatalf("%v 缺少 lod 字段", s["type"])
		}
		if _, ok := s["skipped_by_lod"]; !ok {
			t.Fatalf("%v 缺少 skipped_by_lod 字段", s["type"])
		}
	}
}

// 近景（lod=2）：小范围不抽样，结果与逐区域精确枚举完全一致（等于旧行为）。
func TestSeedLODNearExact(t *testing.T) {
	spec, ok := yySeedStructureSpecByID("village")
	if !ok {
		t.Fatal("缺少 village 结构定义")
	}
	const seed int64 = 20261023
	fromX, fromZ, toX, toZ := -200, -200, 200, 200

	res := yySeedStructureScanEx(seed, spec, fromX, fromZ, toX, toZ, yySeedLODNear, 0, time.Now().Add(yySeedQueryBudget))
	if res.Sampled || res.Degraded || res.Truncated {
		t.Fatalf("小范围近景不应抽样/降级/截断：%+v", res)
	}

	want := make([][2]int, 0, 128)
	rx0, rx1 := yySeedFloorDiv(fromX, spec.Spacing), yySeedFloorDiv(toX, spec.Spacing)
	rz0, rz1 := yySeedFloorDiv(fromZ, spec.Spacing), yySeedFloorDiv(toZ, spec.Spacing)
	for rz := rz0; rz <= rz1; rz++ {
		for rx := rx0; rx <= rx1; rx++ {
			cx, cz := yySeedStructureCandidate(seed, spec, int64(rx), int64(rz))
			if cx < int64(fromX) || cx > int64(toX) || cz < int64(fromZ) || cz > int64(toZ) {
				continue
			}
			want = append(want, [2]int{int(cx), int(cz)})
		}
	}
	sort.Slice(want, func(i, j int) bool {
		if want[i][0] != want[j][0] {
			return want[i][0] < want[j][0]
		}
		return want[i][1] < want[j][1]
	})
	if len(want) != len(res.Chunks) {
		t.Fatalf("精确枚举 %d 个，LOD 近景 %d 个", len(want), len(res.Chunks))
	}
	for i := range want {
		if want[i] != res.Chunks[i] {
			t.Fatalf("第 %d 个点不一致：%v vs %v", i, want[i], res.Chunks[i])
		}
	}

	// HTTP 层同样要标注 sampled=false
	_, body := yySeedTestGet(t, yySeedTestMux(), "/api/seed/structure?seed=20261023&type=village&from_x=-200&from_z=-200&to_x=200&to_z=200&lod=2")
	if body["sampled"] != false {
		t.Fatalf("近景不应标记 sampled：%v", body["sampled"])
	}
	if int(body["lod"].(float64)) != yySeedLODNear {
		t.Fatalf("lod 回显异常：%v", body["lod"])
	}
}

// 热点图：格子数 ≤ 32×32（lod=0），且只返回 count>0 的格子。
func TestSeedHeatCells(t *testing.T) {
	mux := yySeedTestMux()
	code, body := yySeedTestGet(t, mux, "/api/seed/heat?seed=0&type=village&from_x=-1024&from_z=-1024&to_x=1024&to_z=1024&lod=0")
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("状态码 %d，body %v", code, body)
	}
	grid := int(body["grid"].(float64))
	if grid != yySeedHeatGridFar {
		t.Fatalf("lod=0 格子数应为 %d，实际 %d", yySeedHeatGridFar, grid)
	}
	cells := body["cells"].([]any)
	if len(cells) > grid*grid {
		t.Fatalf("格子数 %d 超过 %d", len(cells), grid*grid)
	}
	if len(cells) == 0 {
		t.Fatal("该范围应至少有一个热点格子")
	}
	sum := 0
	for _, raw := range cells {
		c := raw.([]any)
		if len(c) != 3 {
			t.Fatalf("格子格式应为 [gx,gz,count]，实际 %v", c)
		}
		gx, gz, n := int(c[0].(float64)), int(c[1].(float64)), int(c[2].(float64))
		if gx < 0 || gx >= grid || gz < 0 || gz >= grid {
			t.Fatalf("格子坐标越界：%d,%d（grid=%d）", gx, gz, grid)
		}
		if n <= 0 {
			t.Fatalf("不应返回 count<=0 的格子：%v", c)
		}
		sum += n
	}
	if sum != int(body["total"].(float64)) {
		t.Fatalf("total=%v 与格子求和 %d 不一致", body["total"], sum)
	}
	if body["precision"] != "candidate" {
		t.Fatalf("热点图统计的是候选结构，精度应为 candidate，实际 %v", body["precision"])
	}

	// lod=2 用 128×128 网格；type=all 覆盖该维度（默认主世界）全部结构类型
	_, near := yySeedTestGet(t, mux, "/api/seed/heat?seed=0&type=all&from_x=-512&from_z=-512&to_x=512&to_z=512&lod=2")
	if int(near["grid"].(float64)) != yySeedHeatGridNear {
		t.Fatalf("lod=2 格子数应为 %d，实际 %v", yySeedHeatGridNear, near["grid"])
	}
	// 概率型结构（矿井/宝藏）没有区域网格，不参与热点图统计，所以不计入 types
	want := 0
	for _, s := range yySeedStructureSpecsByDim(yyStructDimOverworld) {
		if s.Kind != yyStructKindProbability {
			want++
		}
	}
	if len(near["types"].([]any)) != want {
		t.Fatalf("type=all 应覆盖 %d 种结构，实际 %v", want, near["types"])
	}

	// 缺 type 必须拒绝
	code, bad := yySeedTestGet(t, mux, "/api/seed/heat?seed=0")
	if code == http.StatusInternalServerError || bad["ok"] != false {
		t.Fatalf("缺 type 应返回 ok:false，实际 %d %v", code, bad)
	}
}

// 热点图工作量超限：type=all + 4096 跨度时区域数 > 60000，必须降为远景网格并标记 degraded。
func TestSeedHeatDegrade(t *testing.T) {
	_, body := yySeedTestGet(t, yySeedTestMux(), "/api/seed/heat?seed=0&type=all&from_x=0&from_z=0&to_x=4096&to_z=4096&lod=2")
	if body["degraded"] != true {
		t.Fatalf("type=all 大范围应触发降级，实际 %v（workload=%v）", body["degraded"], body["workload_regions"])
	}
	if msg, _ := body["reason"].(string); msg == "" {
		t.Fatal("降级必须给出中文原因")
	}
	if int(body["grid"].(float64)) != yySeedHeatGridFar {
		t.Fatalf("降级后应使用 %d 格远景网格，实际 %v", yySeedHeatGridFar, body["grid"])
	}
}

// 最坏范围耗时：4096 跨度 + 最密结构（fortress spacing=27）+ lod=2，必须远低于 200ms。
func TestSeedLODWorstCaseLatency(t *testing.T) {
	mux := yySeedTestMux()
	cases := []string{
		"/api/seed/structure?seed=12345&type=fortress&from_x=-4096&from_z=-4096&to_x=0&to_z=0&lod=2",
		"/api/seed/structure?seed=12345&type=fortress&from_x=-4096&from_z=-4096&to_x=0&to_z=0&lod=0",
		"/api/seed/heat?seed=12345&type=all&from_x=-4096&from_z=-4096&to_x=0&to_z=0&lod=2",
		"/api/seed/query?seed=12345&x=0&z=0&radius=32&lod=2",
	}
	for _, url := range cases {
		start := time.Now()
		code, body := yySeedTestGet(t, mux, url)
		elapsed := time.Since(start)
		if code != http.StatusOK || body["ok"] != true {
			t.Fatalf("%s 失败：%d %v", url, code, body)
		}
		if elapsed > 200*time.Millisecond {
			t.Fatalf("%s 耗时 %v，超过 200ms", url, elapsed)
		}
	}
}
