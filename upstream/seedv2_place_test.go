package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_place_test.go —— 结构放置对拍（真值 = D:/种子查询器自研/src/world.mjs）
//
// 【归属】agent: seed-place
//
// 两类测试：
//   1. 锚点测试（不需要 node）：写死几个已验证的坐标/常量，跑在任何机器上都生效。
//   2. 对拍测试（需要 node + 参考实现目录）：把同一批请求同时喂给 Go 和
//      world.mjs，逐点比对，要求 100% 相等。找不到 node 就 t.Skip。
//
// 纪律：对不上就是抄错了，回去读 world.mjs；不许改常量去凑。
// ═══════════════════════════════════════════════════════════════════════

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// 参考实现目录（可用 YY_SEED_REF 覆盖）
const yyV2PlaceRefDefault = "D:/种子查询器自研"

// ── 锚点测试 ──────────────────────────────────────────────────────────

func TestSeedV2PlaceAnchors(t *testing.T) {
	// Java Random(0).nextLong() = -4962768465676381896
	if got := yyV2NewRNG(0).NextLong(); got != -4962768465676381896 {
		t.Fatalf("RNG nextLong 锚点不符: got %d", got)
	}

	// seed=0，region(0,0)：village = (15,2)，mansion = (36,24)
	village, _ := yyV2PlaceConfigFor("village", 102104, yyV2EditionJava)
	if x, z := yyV2PlacePos(0, village, 0, 0); x != 15 || z != 2 {
		t.Fatalf("village(0,0) 应为 (15,2)，got (%d,%d)", x, z)
	}
	mansion, _ := yyV2PlaceConfigFor("woodlandMansion", 102104, yyV2EditionJava)
	if x, z := yyV2PlacePos(0, mansion, 0, 0); x != 36 || z != 24 {
		t.Fatalf("mansion(0,0) 应为 (36,24)，got (%d,%d)", x, z)
	}

	// 史莱姆：world.mjs 实测的正/负样本
	mustSlime := [][3]int64{{0, -12, -20}, {0, -2, -20}, {0, 16, -20}, {12345, 0, -20}, {12345, -17, -19}, {12345, 14, -20}}
	for _, c := range mustSlime {
		if !yyV2IsSlimeChunkV2(c[0], int32(c[1]), int32(c[2])) {
			t.Fatalf("seed=%d (%d,%d) 应为史莱姆区块", c[0], c[1], c[2])
		}
	}
	for _, c := range [][3]int64{{0, 0, 0}, {0, 1, 1}, {12345, 0, 0}, {12345, 1, 0}, {-7, -3, 5}} {
		if yyV2IsSlimeChunkV2(c[0], int32(c[1]), int32(c[2])) {
			t.Fatalf("seed=%d (%d,%d) 不应是史莱姆区块", c[0], c[1], c[2])
		}
	}

	// 要塞环：seed=0，1.21.4 前三点
	sh := yyV2StrongholdsV2(0, 102104)
	if len(sh) != 128 {
		t.Fatalf("要塞数量应为 128，got %d", len(sh))
	}
	want := [][2]int{{-13, -106}, {128, 55}, {-106, 79}}
	for i, w := range want {
		if sh[i] != w {
			t.Fatalf("要塞[%d] 应为 %v，got %v", i, w, sh[i])
		}
	}
	// 1.8 及更早分支（第一个点的初始距离公式不同）
	sh18 := yyV2StrongholdsV2(0, 100800)
	if len(sh18) != 128 || sh18[0] != [2]int{-6, -47} {
		t.Fatalf("1.8 要塞首点应为 (-6,-47)，got %v", sh18[0])
	}
	if sh18[1] != sh[1] {
		t.Fatalf("1.8 与 1.9+ 从第二个点起应一致: %v vs %v", sh18[1], sh[1])
	}
}

func TestSeedV2PlaceConfigBranches(t *testing.T) {
	chk := func(name string, vc int32, ed string, salt int64, span, sep int, large bool) {
		t.Helper()
		cfg, ok := yyV2PlaceConfigFor(name, vc, ed)
		if !ok {
			t.Fatalf("%s @%d/%s 应可用", name, vc, ed)
		}
		if cfg.Salt != salt || cfg.RegionSize != span || cfg.Separation != sep || cfg.LargeSpread != large {
			t.Fatalf("%s @%d/%s 配置错: got {%d %d %d %v} want {%d %d %d %v}",
				name, vc, ed, cfg.Salt, cfg.RegionSize, cfg.Separation, cfg.LargeSpread,
				salt, span, sep, large)
		}
	}
	// 1.18 分界（village 34/8 ↔ legacy 32/8）
	chk("village", 101705, yyV2EditionJava, 10387312, 32, 8, false)
	chk("village", 101800, yyV2EditionJava, 10387312, 34, 8, false)
	chk("village", 101705, yyV2EditionBedrock, 10387312, 27, 10, false)
	chk("village", 101800, yyV2EditionBedrock, 10387312, 34, 8, false)
	// 沉船 1.13-1.15 legacy
	chk("shipwreck", 101505, yyV2EditionJava, 165745295, 16, 8, false)
	chk("shipwreck", 101600, yyV2EditionJava, 165745295, 24, 4, false)
	// 海底废墟 1.16 分界
	chk("oceanRuin", 101600, yyV2EditionJava, 14357621, 20, 8, false)
	// 前哨 Bedrock 变体
	chk("pillagerOutpost", 102000, yyV2EditionBedrock, 165745296, 80, 24, false)
	chk("pillagerOutpost", 102000, yyV2EditionJava, 165745296, 32, 8, false)
	// 废弃传送门
	chk("ruinedPortalOW", 101700, yyV2EditionJava, 34222645, 25, 10, false)
	chk("ruinedPortalOW", 101800, yyV2EditionJava, 34222645, 40, 15, false)
	chk("ruinedPortalNether", 102000, yyV2EditionBedrock, 40552231, 25, 10, false)
	// 沙漠水井三段盐
	chk("desertWell", 101400, yyV2EditionJava, 30010, 1, 1, false)
	chk("desertWell", 101600, yyV2EditionJava, 40013, 1, 1, false)
	chk("desertWell", 101800, yyV2EditionJava, 40002, 1, 1, false)
	// 1.13 前统一盐
	chk("desertTemple", 101200, yyV2EditionJava, yyV2LegacySalt, 32, 8, false)
	chk("jungleTemple", 101200, yyV2EditionJava, yyV2LegacySalt, 32, 8, false)
	chk("witchHut", 101200, yyV2EditionJava, yyV2LegacySalt, 32, 8, false)
	// 末地城三角分布 + 末地船复用配置
	chk("endCity", 102104, yyV2EditionJava, 10387313, 20, 11, true)
	chk("endShip", 102104, yyV2EditionJava, 10387313, 20, 11, true)
	// 下界 separation 必须是 4
	chk("netherFortress", 102104, yyV2EditionJava, 30084232, 27, 4, false)
	chk("bastionRemnant", 102104, yyV2EditionJava, 30084232, 27, 4, false)

	// 版本不可用
	if _, ok := yyV2PlaceConfigFor("ancientCity", 101800, yyV2EditionJava); ok {
		t.Fatal("1.18 不应有远古城市")
	}
	if _, ok := yyV2PlaceConfigFor("abandonedCamp", 102104, yyV2EditionJava); ok {
		t.Fatal("1.21 不应有废弃营地")
	}
	if _, ok := yyV2PlaceConfigFor("mineshaft", 102104, yyV2EditionJava); ok {
		t.Fatal("废弃矿井没有 region 配置")
	}
	if !yyV2PlaceAvailable("mineshaft", 102104, yyV2EditionJava) {
		t.Fatal("废弃矿井版本上可用（只是没有放置配置）")
	}
	if !yyV2PlaceAvailable("abandonedCamp", 260300, yyV2EditionJava) {
		t.Fatal("26.3 应有废弃营地")
	}
}

func TestSeedV2PlaceListAndAlias(t *testing.T) {
	list := yyV2StructureList(102104, yyV2EditionJava)
	if len(list) != 27 {
		t.Fatalf("1.21.4 Java 结构数应为 27（28 项减去 26.3 才有的废弃营地），got %d", len(list))
	}
	ids := map[string]bool{}
	for _, info := range list {
		ids[info.ID] = true
		if info.Name == "" || info.Dim == "" || info.Placement == "" || info.Precision == "" {
			t.Fatalf("%s 元信息不完整: %+v", info.ID, info)
		}
	}
	if !ids["endShip"] || !ids["stronghold"] || ids["abandonedCamp"] {
		t.Fatalf("结构清单不对: %v", ids)
	}
	// 26.3 才有废弃营地
	if !containsID(yyV2StructureList(260300, yyV2EditionJava), "abandonedCamp") {
		t.Fatal("26.3 应列出废弃营地")
	}
	// 1.12 没有远古城市/试炼密室
	for _, id := range []string{"ancientCity", "trailRuin", "trialChamber", "amethystGeode"} {
		if containsID(yyV2StructureList(101200, yyV2EditionJava), id) {
			t.Fatalf("1.12 不应列出 %s", id)
		}
	}

	// 别名归一
	aliases := map[string]string{
		"mansion":                  "woodlandMansion",
		"monument":                 "oceanMonument",
		"outpost":                  "pillagerOutpost",
		"fortress":                 "netherFortress",
		"bastion":                  "bastionRemnant",
		"camp":                     "abandonedCamp",
		"geode":                    "amethystGeode",
		"slime":                    "slimeChunk",
		"ruinedPortal":             "ruinedPortalOW",
		"ruinedPortalOverworld":    "ruinedPortalOW",
		"endShip":                  "endShip",
		"woodland_mansion":         "woodlandMansion",
		"minecraft:ocean_monument": "oceanMonument",
		"村庄":                       "village",
		"要塞":                       "stronghold",
		"废弃传送门(下界)":                "ruinedPortalNether",
		"紫水晶洞":                     "amethystGeode",
		"史莱姆区块":                    "slimeChunk",
	}
	for in, want := range aliases {
		info, ok := yyV2StructureInfoByID(in, 102104, yyV2EditionJava)
		if !ok || info.ID != want {
			t.Fatalf("别名 %q 应归一为 %q，got %q ok=%v", in, want, info.ID, ok)
		}
	}
	if _, ok := yyV2StructureInfoByID("totally_bogus", 102104, yyV2EditionJava); ok {
		t.Fatal("未知 id 应返回 false")
	}
}

func containsID(list []yyV2StructureInfo, id string) bool {
	for _, info := range list {
		if info.ID == id {
			return true
		}
	}
	return false
}

func TestSeedV2PlaceScanBasics(t *testing.T) {
	// 负坐标矩形：floorDiv 不能退化成向零截断
	pts := yyV2ScanStructures(12345, "village", 102104, yyV2EditionJava, -34, -34, -1, -1, 0)
	if len(pts) != 1 {
		t.Fatalf("village region(-1,-1) 应出 1 个点，got %d: %v", len(pts), pts)
	}
	// 排序 + 截断
	all := yyV2ScanStructures(12345, "village", 102104, yyV2EditionJava, -170, -170, 170, 170, 0)
	if len(all) == 0 {
		t.Fatal("village 扫描为空")
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool {
		if all[i].Z != all[j].Z {
			return all[i].Z < all[j].Z
		}
		return all[i].X < all[j].X
	}) {
		t.Fatal("扫描结果未按 (z,x) 排序")
	}
	limited := yyV2ScanStructures(12345, "village", 102104, yyV2EditionJava, -170, -170, 170, 170, 7)
	if len(limited) != 7 {
		t.Fatalf("limit=7 应返回 7 个，got %d", len(limited))
	}
	for i := range limited {
		if limited[i] != all[i] {
			t.Fatalf("截断后第 %d 个点应与全量一致", i)
		}
	}
	// 反序入参自动纠正
	reversed := yyV2ScanStructures(12345, "village", 102104, yyV2EditionJava, 170, 170, -170, -170, 0)
	if len(reversed) != len(all) {
		t.Fatalf("反序入参结果数应一致: %d vs %d", len(reversed), len(all))
	}
	// 未实现的结构返回空（不伪造）
	if p := yyV2ScanStructures(1, "mineshaft", 102104, yyV2EditionJava, -10, -10, 10, 10, 0); len(p) != 0 {
		t.Fatal("mineshaft 无算法，应返回空")
	}
	// 参考实现 range=0（spacing==separation）的结构枚举解为 NaN，候选恒空；
	// Go 侧保持一致，别偷偷给「每区块一个候选」。
	for _, id := range []string{"buriedTreasure", "desertWell"} {
		if p := yyV2ScanStructures(42, id, 102104, yyV2EditionJava, -20, -20, 20, 20, 0); len(p) != 0 {
			t.Fatalf("%s 在参考实现里候选恒空，Go 也不该出点（got %d）", id, len(p))
		}
	}
	// 下界要塞 / 堡垒互斥：同一 region 不会同时出现
	f := yyV2ScanStructures(12345, "fortress", 102104, yyV2EditionJava, -135, -135, 135, 135, 0)
	b := yyV2ScanStructures(12345, "bastion", 102104, yyV2EditionJava, -135, -135, 135, 135, 0)
	set := map[[2]int]bool{}
	for _, p := range f {
		set[[2]int{p.X, p.Z}] = true
	}
	for _, p := range b {
		if set[[2]int{p.X, p.Z}] {
			t.Fatalf("要塞与堡垒重叠: (%d,%d)", p.X, p.Z)
		}
	}
	if len(f) == 0 || len(b) == 0 {
		t.Fatalf("要塞/堡垒分流不应为空: %d/%d", len(f), len(b))
	}
}

func TestSeedV2PlaceEndCityHeight(t *testing.T) {
	// 全平高度图：任意播种都应返回 100
	flat := make([]int16, 256)
	for i := range flat {
		flat[i] = 100
	}
	for _, vc := range []int32{101800, 102104} {
		for cx := 0; cx < 5; cx++ {
			h, ok := yyV2EndCityHeight(cx, cx*3, flat, 12345, vc)
			if !ok || h != 100 {
				t.Fatalf("平地图应返回 (100,true)，got (%d,%v)", h, ok)
			}
		}
	}
	// 高度图不够长 → 明确返回 false，不猜
	if _, ok := yyV2EndCityHeight(0, 0, make([]int16, 10), 0, 102104); ok {
		t.Fatal("heightmap 长度不足时应返回 false")
	}
	// 四角取 min：把一个角压低，结果必须跟着变
	corner := make([]int16, 256)
	for i := range corner {
		corner[i] = 90
	}
	corner[119] = 70 // (x=7,z=7)
	h, ok := yyV2EndCityHeight(0, 0, corner, 0, 102104)
	if !ok || h != 70 {
		t.Fatalf("应取四角最小值 70，got (%d,%v)", h, ok)
	}
	// 1.18 播种 = 10387313*z + x，与末地城位置链同盐
	if h, ok := yyV2EndCityHeight(3, 7, corner, 999, 101800); !ok || h != 70 {
		t.Fatalf("旧版播种结果错: (%d,%v)", h, ok)
	}
}

func TestSeedV2PlaceSpawnAndSlimeScan(t *testing.T) {
	x, z := yyV2SpawnPointV2(12345)
	if x < -64 || x > 64 || z < -64 || z > 64 {
		t.Fatalf("出生点粗估越界: (%d,%d)", x, z)
	}
	pts := yyV2ScanStructures(12345, "spawn", 102104, yyV2EditionJava, -64, -64, 64, 64, 0)
	if len(pts) != 1 || pts[0].X != x || pts[0].Z != z {
		t.Fatalf("spawn 扫描应返回粗估点，got %v", pts)
	}
	if pts[0].Variant == "" {
		t.Fatal("spawn 必须标注不是原版算法")
	}
	sl := yyV2ScanStructures(12345, "slime", 102104, yyV2EditionJava, -20, -20, 20, 20, 0)
	if len(sl) == 0 {
		t.Fatal("史莱姆扫描为空")
	}
	for _, p := range sl {
		if !yyV2IsSlimeChunkV2(12345, int32(p.X), int32(p.Z)) {
			t.Fatalf("史莱姆扫描混入非史莱姆区块 (%d,%d)", p.X, p.Z)
		}
	}
}

// ── 与 world.mjs 对拍 ─────────────────────────────────────────────────

type placeSpec struct {
	Key     string `json:"key"`
	Kind    string `json:"kind"` // regionv / region / nether / stronghold / slime / config
	Name    string `json:"name,omitempty"`
	Edition string `json:"edition,omitempty"`
	Cfg     string `json:"cfg,omitempty"`
	Seed    string `json:"seed,omitempty"`
	Version int32  `json:"version,omitempty"`
	Type    string `json:"type,omitempty"`
	X0      int    `json:"x0"`
	Z0      int    `json:"z0"`
	X1      int    `json:"x1"`
	Z1      int    `json:"z1"`
}

// 真值脚本：读参考实现，把每个 case 的点集打成 JSON。
const yyV2PlaceNodeScript = `
import {pathToFileURL} from 'node:url';
import path from 'node:path';

const dir = process.argv[2];
const spec = JSON.parse(process.argv[3]);
const w = await import(pathToFileURL(path.join(dir, 'src', 'world.mjs')).href);
const out = {};

for (const c of spec) {
  if (c.kind === 'config') {
    const cfg = w.configForVersion(c.name, c.version, c.edition);
    out[c.key] = cfg ? [cfg.salt, cfg.regionSize, cfg.separation, cfg.largeSpread ? 1 : 0] : null;
    continue;
  }
  let pts = [];
  if (c.kind === 'region') {
    pts = [...w.structureCandidates(BigInt(c.seed), w.STRUCTURE_CONFIG[c.cfg], c.x0, c.z0, c.x1, c.z1)];
  } else if (c.kind === 'regionv') {
    const cfg = w.configForVersion(c.name, c.version, c.edition);
    if (!cfg) throw new Error('config missing: ' + c.name);
    pts = [...w.structureCandidates(BigInt(c.seed), cfg, c.x0, c.z0, c.x1, c.z1)];
  } else if (c.kind === 'nether') {
    pts = [...w.netherStructureCandidates(BigInt(c.seed), c.version, c.x0, c.z0, c.x1, c.z1)]
      .filter(p => p.type === c.type).map(p => [p.x, p.z]);
  } else if (c.kind === 'stronghold') {
    pts = w.strongholdRingPositions(BigInt(c.seed), c.version);
  } else if (c.kind === 'slime') {
    for (let z = c.z0; z <= c.z1; z++)
      for (let x = c.x0; x <= c.x1; x++)
        if (w.isSlimeChunkJava(BigInt(c.seed), x, z)) pts.push([x, z]);
  }
  out[c.key] = pts;
}
process.stdout.write(JSON.stringify(out));
`

func TestSeedV2PlaceGoldenAgainstNode(t *testing.T) {
	refDir := os.Getenv("YY_SEED_REF")
	if refDir == "" {
		refDir = yyV2PlaceRefDefault
	}
	if _, err := os.Stat(filepath.Join(refDir, "src", "world.mjs")); err != nil {
		t.Skipf("参考实现不存在，跳过对拍: %v", err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("没有 node，跳过对拍")
	}

	// 每个 case 都配了 Go 侧的调用参数；node 侧按同样的参数算真值。
	type vcase struct {
		spec    placeSpec
		goID    string // 空 = 用 spec.Name
		limit   int
		sortOut bool
	}
	cases := []vcase{
		{spec: placeSpec{Key: "village", Kind: "regionv", Name: "village", Edition: "Java", Seed: "12345", Version: 102104, X0: -170, Z0: -170, X1: 170, Z1: 170}, sortOut: true},
		{spec: placeSpec{Key: "mansion", Kind: "regionv", Name: "woodlandMansion", Edition: "Java", Seed: "12345", Version: 102104, X0: -400, Z0: -400, X1: 400, Z1: 400}, sortOut: true},
		{spec: placeSpec{Key: "monument", Kind: "regionv", Name: "oceanMonument", Edition: "Java", Seed: "12345", Version: 102104, X0: -160, Z0: -160, X1: 160, Z1: 160}, sortOut: true},
		{spec: placeSpec{Key: "endcity", Kind: "regionv", Name: "endCity", Edition: "Java", Seed: "12345", Version: 102104, X0: -100, Z0: -100, X1: 100, Z1: 100}, sortOut: true},
		{spec: placeSpec{Key: "outpost", Kind: "regionv", Name: "pillagerOutpost", Edition: "Java", Seed: "12345", Version: 102104, X0: -160, Z0: -160, X1: 160, Z1: 160}, sortOut: true},
		{spec: placeSpec{Key: "shipwreck", Kind: "regionv", Name: "shipwreck", Edition: "Java", Seed: "-4827193", Version: 102104, X0: -120, Z0: -120, X1: 120, Z1: 120}, sortOut: true},
		{spec: placeSpec{Key: "deserttemple", Kind: "regionv", Name: "desertTemple", Edition: "Java", Seed: "42", Version: 102104, X0: -160, Z0: -160, X1: 160, Z1: 160}, sortOut: true},
		{spec: placeSpec{Key: "ancientcity", Kind: "regionv", Name: "ancientCity", Edition: "Java", Seed: "42", Version: 102104, X0: -120, Z0: -120, X1: 120, Z1: 120}, sortOut: true},
		{spec: placeSpec{Key: "trailruin", Kind: "regionv", Name: "trailRuin", Edition: "Java", Seed: "42", Version: 102104, X0: -170, Z0: -170, X1: 170, Z1: 170}, sortOut: true},
		{spec: placeSpec{Key: "trialchamber", Kind: "regionv", Name: "trialChamber", Edition: "Java", Seed: "42", Version: 102104, X0: -170, Z0: -170, X1: 170, Z1: 170}, sortOut: true},
		{spec: placeSpec{Key: "treasure", Kind: "regionv", Name: "buriedTreasure", Edition: "Java", Seed: "42", Version: 102104, X0: -5, Z0: -5, X1: 5, Z1: 5}, sortOut: true},
		{spec: placeSpec{Key: "desertwell", Kind: "regionv", Name: "desertWell", Edition: "Java", Seed: "42", Version: 102104, X0: -12, Z0: -12, X1: 12, Z1: 12}, sortOut: true},
		{spec: placeSpec{Key: "camp", Kind: "regionv", Name: "abandonedCamp", Edition: "Java", Seed: "42", Version: 260300, X0: -20, Z0: -20, X1: 20, Z1: 20}, sortOut: true},
		// 版本分支：1.12 的村庄（legacy 32/8）与 1.13 前统一盐
		{spec: placeSpec{Key: "village112", Kind: "regionv", Name: "village", Edition: "Java", Seed: "12345", Version: 101200, X0: -160, Z0: -160, X1: 160, Z1: 160}, sortOut: true},
		{spec: placeSpec{Key: "temple112", Kind: "regionv", Name: "desertTemple", Edition: "Java", Seed: "12345", Version: 101200, X0: -160, Z0: -160, X1: 160, Z1: 160}, sortOut: true},
		// Bedrock 线性/三角分布 + legacy spacing
		{spec: placeSpec{Key: "villageBE", Kind: "regionv", Name: "village", Edition: "Bedrock", Seed: "12345", Version: 101600, X0: -135, Z0: -135, X1: 135, Z1: 135}, sortOut: true},
		{spec: placeSpec{Key: "monumentBE", Kind: "regionv", Name: "oceanMonument", Edition: "Bedrock", Seed: "12345", Version: 101600, X0: -160, Z0: -160, X1: 160, Z1: 160}, sortOut: true},
		// 下界要塞 / 堡垒分流
		{spec: placeSpec{Key: "fortress117", Kind: "nether", Type: "netherFortress", Seed: "12345", Version: 101700, X0: -135, Z0: -135, X1: 135, Z1: 135}, goID: "netherFortress", sortOut: true},
		{spec: placeSpec{Key: "bastion117", Kind: "nether", Type: "bastionRemnant", Seed: "12345", Version: 101700, X0: -135, Z0: -135, X1: 135, Z1: 135}, goID: "bastionRemnant", sortOut: true},
		{spec: placeSpec{Key: "fortress121", Kind: "nether", Type: "netherFortress", Seed: "12345", Version: 102104, X0: -135, Z0: -135, X1: 135, Z1: 135}, goID: "netherFortress", sortOut: true},
		// 三角分布：大跨度，同时验证 limit 截断
		{spec: placeSpec{Key: "villageBig", Kind: "regionv", Name: "village", Edition: "Java", Seed: "12345", Version: 102104, X0: -2000, Z0: -2000, X1: 2000, Z1: 2000}, limit: 500, sortOut: true},
		// 要塞环
		{spec: placeSpec{Key: "sh", Kind: "stronghold", Seed: "12345", Version: 102104}},
		{spec: placeSpec{Key: "sh0", Kind: "stronghold", Seed: "0", Version: 100800}},
		// 史莱姆 ±50 区块
		{spec: placeSpec{Key: "slime", Kind: "slime", Seed: "12345", X0: -50, Z0: -50, X1: 50, Z1: 50}},
		{spec: placeSpec{Key: "slime0", Kind: "slime", Seed: "0", X0: -50, Z0: -50, X1: 50, Z1: 50}},
	}
	configs := []placeSpec{
		{Key: "cfg_village_117", Kind: "config", Name: "village", Edition: "Java", Version: 101705},
		{Key: "cfg_village_118", Kind: "config", Name: "village", Edition: "Java", Version: 101800},
		{Key: "cfg_village_be", Kind: "config", Name: "village", Edition: "Bedrock", Version: 101705},
		{Key: "cfg_ruinedOW_117", Kind: "config", Name: "ruinedPortalOW", Edition: "Java", Version: 101705},
		{Key: "cfg_ruinedNether_be", Kind: "config", Name: "ruinedPortalNether", Edition: "Bedrock", Version: 102000},
		{Key: "cfg_well_114", Kind: "config", Name: "desertWell", Edition: "Java", Version: 101400},
		{Key: "cfg_well_116", Kind: "config", Name: "desertWell", Edition: "Java", Version: 101600},
		{Key: "cfg_well_118", Kind: "config", Name: "desertWell", Edition: "Java", Version: 101800},
		{Key: "cfg_temple_112", Kind: "config", Name: "desertTemple", Edition: "Java", Version: 101200},
		{Key: "cfg_ship_115", Kind: "config", Name: "shipwreck", Edition: "Java", Version: 101505},
		{Key: "cfg_ruin_115", Kind: "config", Name: "oceanRuin", Edition: "Java", Version: 101505},
		{Key: "cfg_outpost_be", Kind: "config", Name: "pillagerOutpost", Edition: "Bedrock", Version: 102000},
		{Key: "cfg_endship", Kind: "config", Name: "endShip", Edition: "Java", Version: 102104},
		{Key: "cfg_ancient_118", Kind: "config", Name: "ancientCity", Edition: "Java", Version: 101800},
		{Key: "cfg_camp_121", Kind: "config", Name: "abandonedCamp", Edition: "Java", Version: 102104},
	}

	specs := make([]placeSpec, 0, len(cases)+len(configs))
	for _, c := range cases {
		specs = append(specs, c.spec)
	}
	specs = append(specs, configs...)

	raw, err := json.Marshal(specs)
	if err != nil {
		t.Fatalf("序列化 spec 失败: %v", err)
	}
	script := filepath.Join(t.TempDir(), "yy_place_dump.mjs")
	if err := os.WriteFile(script, []byte(yyV2PlaceNodeScript), 0o644); err != nil {
		t.Fatalf("写临时脚本失败: %v", err)
	}
	out, err := exec.Command("node", script, refDir, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatalf("node 执行失败: %v\n%s", err, out)
	}
	var truth map[string]json.RawMessage
	if err := json.Unmarshal(out, &truth); err != nil {
		t.Fatalf("解析 node 输出失败: %v\n%s", err, out)
	}

	// 点集比对
	totalPts := 0
	for _, c := range cases {
		t.Run(c.spec.Key, func(t *testing.T) {
			var want [][2]int
			if err := json.Unmarshal(truth[c.spec.Key], &want); err != nil {
				t.Fatalf("解析 %s 真值失败: %v", c.spec.Key, err)
			}
			goID := c.goID
			if goID == "" {
				goID = c.spec.Name
			}
			ed := "java"
			if c.spec.Edition == "Bedrock" {
				ed = "bedrock"
			}
			var got [][2]int
			switch c.spec.Kind {
			case "stronghold":
				sh := yyV2StrongholdsV2(int64(atoi64(c.spec.Seed)), c.spec.Version)
				n := 20
				if len(sh) < n {
					n = len(sh)
				}
				for i := 0; i < n; i++ {
					got = append(got, sh[i])
				}
				if len(want) > n {
					want = want[:n]
				}
			case "slime":
				pts := yyV2ScanStructures(int64(atoi64(c.spec.Seed)), "slimeChunk", c.spec.Version, ed, c.spec.X0, c.spec.Z0, c.spec.X1, c.spec.Z1, 0)
				for _, p := range pts {
					got = append(got, [2]int{p.X, p.Z})
				}
			default:
				pts := yyV2ScanStructures(int64(atoi64(c.spec.Seed)), goID, c.spec.Version, ed, c.spec.X0, c.spec.Z0, c.spec.X1, c.spec.Z1, c.limit)
				for _, p := range pts {
					got = append(got, [2]int{p.X, p.Z})
				}
				if c.sortOut {
					sort.Slice(want, func(i, j int) bool {
						if want[i][1] != want[j][1] {
							return want[i][1] < want[j][1]
						}
						return want[i][0] < want[j][0]
					})
					if c.limit > 0 && len(want) > c.limit {
						want = want[:c.limit]
					}
				}
			}
			if c.spec.Kind != "stronghold" && c.spec.Kind != "slime" {
				sort.Slice(got, func(i, j int) bool {
					if got[i][1] != got[j][1] {
						return got[i][1] < got[j][1]
					}
					return got[i][0] < got[j][0]
				})
			}
			if len(got) != len(want) {
				t.Fatalf("点数不一致: go=%d node=%d", len(got), len(want))
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("第 %d 个点不一致: go=%v node=%v", i, got[i], want[i])
				}
			}
			totalPts += len(got)
			if len(got) < 20 && c.spec.Kind != "slime" {
				t.Logf("提示: %s 只有 %d 个点", c.spec.Key, len(got))
			}
		})
	}

	// 配置分支比对
	for _, cfgSpec := range configs {
		t.Run(cfgSpec.Key, func(t *testing.T) {
			var want []float64
			if err := json.Unmarshal(truth[cfgSpec.Key], &want); err != nil {
				// null（不可用）时 Unmarshal 成功且 want 为 nil
				want = nil
			}
			ed := yyV2NormalizeEdition(cfgSpec.Edition)
			cfg, ok := yyV2PlaceConfigFor(cfgSpec.Name, cfgSpec.Version, ed)
			if len(want) == 0 {
				if ok {
					t.Fatalf("%s 在 node 侧不可用，Go 却返回了配置", cfgSpec.Name)
				}
				return
			}
			if !ok {
				t.Fatalf("%s 在 node 侧可用，Go 说不可用", cfgSpec.Name)
			}
			large := 0.0
			if cfg.LargeSpread {
				large = 1
			}
			if want[0] != float64(cfg.Salt) || want[1] != float64(cfg.RegionSize) ||
				want[2] != float64(cfg.Separation) || want[3] != large {
				t.Fatalf("%s 配置不一致: node=%v go={%d %d %d %v}",
					cfgSpec.Name, want, cfg.Salt, cfg.RegionSize, cfg.Separation, cfg.LargeSpread)
			}
		})
	}

	t.Logf("对拍通过：%d 个结构 case，共 %d 个点", len(cases), totalPts)
}

func atoi64(s string) int64 {
	var v int64
	neg := false
	for i := 0; i < len(s); i++ {
		if i == 0 && s[i] == '-' {
			neg = true
			continue
		}
		v = v*10 + int64(s[i]-'0')
	}
	if neg {
		return -v
	}
	return v
}
