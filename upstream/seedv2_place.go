package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_place.go —— 结构放置 / 要塞环 / 史莱姆 / 出生点
//
// 【归属】agent: seed-place
// 【来源】D:/种子查询器自研/src/world.mjs（已 274 组黄金基准 + 24 结构
//         召回 100% 验证，逐行照搬，公式一个字符都不许改）
//
// 【必须复刻的 5 个坑】
//   1. regionSeed = regionX*341873128712 + regionZ*132897987541 + worldSeed + salt
//      （全部 64 位回绕无符号加法）
//   2. `xi()` 是**向零取整**（Math.trunc），不是 floor。负数下 floor 会差 1。
//   3. largeSpread=true 走三角分布：每轴 2 次 nextInt 取平均再 xi()；
//      否则线性：每轴 1 次 nextInt。oceanMonument / endCity / woodlandMansion
//      以及**所有 Bedrock 变体**是三角分布 —— 搞错会导致府邸召回 0。
//   4. 下界要塞与堡垒遗迹同 salt 同 spacing，一个 region 最多出一个；
//      1.16–1.17 用同 region RNG 的第三次 nextInt(5) 分流。
//   5. netherFortress / bastionRemnant 的 separation 是 **4**，不是 23
//      （23 是 cubiomes 的 chunkRange = spacing - separation，别搞反）。
//
// 【与参考实现的两处有据可查的偏差】
//   a. nextDouble：seedv2_rand.go 的 (*yyV2RNG).NextDouble 目前漏了「除以 2^53」
//      （见该文件第 74-77 行注释），要塞环依赖它。本文件用私有
//      yyV2PlaceNextDouble 按 Java 原式 (next(26)*2^27+next(27))/2^53 计算，
//      不改别人的文件；等 seed-rand-noise 修好后两边结果一致，可无痛替换。
//   b. endCityHeightAt：world.mjs 现版本把 s/c 轴写反了（docs/structures/
//      END_CITY.md §Bug 2 有定量证据：正确索引是 119+c，world.mjs 算的是
//      119+16c）。本文件按文档修正版实现，见 yyV2EndCityHeight。
// ═══════════════════════════════════════════════════════════════════════

import (
	"math"
	"sort"
)

// ── 结构配置表（spacing/separation/salt，chunkbase 观测值）─────────────
//
// 与 world.mjs 的 STRUCTURE_CONFIG 逐条对应，一条都不许少：
// legacy 变体是版本分支用的，不参与默认放置。

type yyV2PlaceCfg struct {
	Salt        int64
	RegionSize  int
	Separation  int
	LargeSpread bool
}

var yyV2PlaceConfigs = map[string]yyV2PlaceCfg{
	"village":                  {Salt: 10387312, RegionSize: 34, Separation: 8},
	"village_legacy":           {Salt: 10387312, RegionSize: 32, Separation: 8},  // Java <1.18
	"village_bedrock_legacy":   {Salt: 10387312, RegionSize: 27, Separation: 10}, // Bedrock <1.18
	"woodlandMansion":          {Salt: 10387319, RegionSize: 80, Separation: 20, LargeSpread: true},
	"oceanMonument":            {Salt: 10387313, RegionSize: 32, Separation: 5, LargeSpread: true},
	"endCity":                  {Salt: 10387313, RegionSize: 20, Separation: 11, LargeSpread: true},
	"desertTemple":             {Salt: 14357617, RegionSize: 32, Separation: 8},
	"jungleTemple":             {Salt: 14357619, RegionSize: 32, Separation: 8},
	"igloo":                    {Salt: 14357618, RegionSize: 32, Separation: 8},
	"witchHut":                 {Salt: 14357620, RegionSize: 32, Separation: 8},
	"pillagerOutpost":          {Salt: 165745296, RegionSize: 32, Separation: 8}, // Java
	"pillagerOutpost_bedrock":  {Salt: 165745296, RegionSize: 80, Separation: 24},
	"shipwreck":                {Salt: 165745295, RegionSize: 24, Separation: 4},
	"shipwreck_legacy":         {Salt: 165745295, RegionSize: 16, Separation: 8}, // Java 1.13-1.15
	"shipwreck_bedrock_legacy": {Salt: 165745295, RegionSize: 10, Separation: 5}, // Bedrock <1.18
	"oceanRuin":                {Salt: 14357621, RegionSize: 20, Separation: 8},
	"oceanRuin_legacy":         {Salt: 14357617, RegionSize: 16, Separation: 8}, // Java <1.16
	"oceanRuin_bedrock_legacy": {Salt: 14357621, RegionSize: 12, Separation: 7}, // Bedrock <1.18
	"ancientCity":              {Salt: 20083232, RegionSize: 24, Separation: 8},
	"trailRuin":                {Salt: 83469867, RegionSize: 34, Separation: 8},
	"trialChamber":             {Salt: 94251327, RegionSize: 34, Separation: 12},
	"buriedTreasure":           {Salt: 10387320, RegionSize: 1, Separation: 1},
	// separation 是「留边」，不是 chunkRange。cubiomes 的 chunkRange=27-4=23，别搞反
	"netherFortress":             {Salt: 30084232, RegionSize: 27, Separation: 4},
	"bastionRemnant":             {Salt: 30084232, RegionSize: 27, Separation: 4},
	"ruinedPortalOW":             {Salt: 34222645, RegionSize: 40, Separation: 15},
	"ruinedPortalOW_legacy":      {Salt: 34222645, RegionSize: 25, Separation: 10}, // <1.18
	"ruinedPortalNether":         {Salt: 34222645, RegionSize: 40, Separation: 15}, // Java 1.18+；1.16-1.17 用 25/10
	"ruinedPortalNether_bedrock": {Salt: 40552231, RegionSize: 25, Separation: 10},
	"abandonedCamp":              {Salt: 16842397, RegionSize: 4, Separation: 2},
	"desertWell":                 {Salt: 40002, RegionSize: 1, Separation: 1},
	"desertWell_117":             {Salt: 40013, RegionSize: 1, Separation: 1},
	"desertWell_115":             {Salt: 30010, RegionSize: 1, Separation: 1},
}

// yyV2PlaceVersionRange 结构可用版本区间（版本码，闭区间）。
type yyV2PlaceVersionRange struct {
	Java    [2]int32
	Bedrock [2]int32
}

// 与 world.mjs 的 STRUCTURE_VERSIONS 逐条对应。
var yyV2PlaceVersions = map[string]yyV2PlaceVersionRange{
	"village":            {Java: [2]int32{100700, 260400}, Bedrock: [2]int32{101400, 267000}},
	"woodlandMansion":    {Java: [2]int32{101100, 260400}, Bedrock: [2]int32{101400, 267000}},
	"oceanMonument":      {Java: [2]int32{100800, 260400}, Bedrock: [2]int32{101400, 267000}},
	"endCity":            {Java: [2]int32{101300, 260400}, Bedrock: [2]int32{101400, 267000}},
	"endShip":            {Java: [2]int32{101300, 260400}, Bedrock: [2]int32{101400, 267000}},
	"desertTemple":       {Java: [2]int32{100700, 260400}, Bedrock: [2]int32{101400, 267000}},
	"jungleTemple":       {Java: [2]int32{100700, 260400}, Bedrock: [2]int32{101400, 267000}},
	"igloo":              {Java: [2]int32{100900, 260400}, Bedrock: [2]int32{101400, 267000}},
	"witchHut":           {Java: [2]int32{100700, 260400}, Bedrock: [2]int32{101400, 267000}},
	"pillagerOutpost":    {Java: [2]int32{101400, 260400}, Bedrock: [2]int32{101400, 267000}},
	"shipwreck":          {Java: [2]int32{101300, 260400}, Bedrock: [2]int32{101400, 267000}},
	"oceanRuin":          {Java: [2]int32{101600, 260400}, Bedrock: [2]int32{101400, 267000}},
	"ancientCity":        {Java: [2]int32{101900, 260400}, Bedrock: [2]int32{101900, 267000}},
	"trailRuin":          {Java: [2]int32{102000, 260400}, Bedrock: [2]int32{102000, 267000}},
	"trialChamber":       {Java: [2]int32{102100, 260400}, Bedrock: [2]int32{102100, 267000}},
	"buriedTreasure":     {Java: [2]int32{101300, 260400}, Bedrock: [2]int32{101400, 267000}},
	"netherFortress":     {Java: [2]int32{100700, 260400}, Bedrock: [2]int32{101400, 267000}},
	"bastionRemnant":     {Java: [2]int32{101600, 260400}, Bedrock: [2]int32{101400, 267000}},
	"ruinedPortalOW":     {Java: [2]int32{101600, 260400}, Bedrock: [2]int32{101400, 267000}},
	"ruinedPortalNether": {Java: [2]int32{101600, 260400}, Bedrock: [2]int32{101400, 267000}},
	"abandonedCamp":      {Java: [2]int32{260300, 260400}, Bedrock: [2]int32{265000, 267000}},
	"desertWell":         {Java: [2]int32{101300, 260400}, Bedrock: [2]int32{101400, 267000}},
	"stronghold":         {Java: [2]int32{100700, 260400}, Bedrock: [2]int32{101400, 267000}},
	"spawn":              {Java: [2]int32{100700, 260400}, Bedrock: [2]int32{101400, 267000}},
	"slimeChunk":         {Java: [2]int32{100700, 260400}, Bedrock: [2]int32{101400, 267000}},
	"mineshaft":          {Java: [2]int32{100700, 260400}, Bedrock: [2]int32{101400, 267000}},
	"fossil":             {Java: [2]int32{101600, 260400}, Bedrock: [2]int32{101400, 267000}},
	"amethystGeode":      {Java: [2]int32{101700, 260400}, Bedrock: [2]int32{101700, 267000}},
}

// yyV2LegacySalt 1.13 前统一盐。
const yyV2LegacySalt = 14357617

// 常量：与 world.mjs 顶部完全一致
const (
	yyV2RegionMulX = 341873128712
	yyV2RegionMulZ = 132897987541
)

// ── 版本分支 ──────────────────────────────────────────────────────────

// yyV2PlaceConfigFor 按版本取结构配置；不可用返回 (零值,false)。
// 逐行对应 world.mjs 的 configForVersion：版本区间 → legacy 分支 → 1.13 统一盐。
// versionCode<=0 视为「不限版本」（内部/未知版本调用，避免整表被过滤空）。
func yyV2PlaceConfigFor(name string, versionCode int32, edition string) (yyV2PlaceCfg, bool) {
	ed := yyV2NormalizeEdition(edition)
	if vr, ok := yyV2PlaceVersions[name]; ok && versionCode > 0 {
		rng := vr.Java
		if ed == yyV2EditionBedrock {
			rng = vr.Bedrock
		}
		if versionCode < rng[0] || versionCode > rng[1] {
			return yyV2PlaceCfg{}, false
		}
	}

	// endShip 是 endCity 的 hasShip 子集，复用 endCity 的放置配置
	cfgName := name
	if name == "endShip" {
		cfgName = "endCity"
	}
	cfg, ok := yyV2PlaceConfigs[cfgName]
	if !ok {
		return yyV2PlaceCfg{}, false
	}

	switch cfgName {
	case "village":
		if ed == yyV2EditionBedrock && versionCode < 101800 {
			cfg = yyV2PlaceConfigs["village_bedrock_legacy"]
		} else if ed == yyV2EditionJava && versionCode < 101800 {
			cfg = yyV2PlaceConfigs["village_legacy"]
		}
	case "shipwreck":
		if ed == yyV2EditionBedrock && versionCode < 101800 {
			cfg = yyV2PlaceConfigs["shipwreck_bedrock_legacy"]
		} else if ed == yyV2EditionJava && versionCode < 101600 {
			cfg = yyV2PlaceConfigs["shipwreck_legacy"]
		}
	case "oceanRuin":
		if ed == yyV2EditionBedrock && versionCode < 101800 {
			cfg = yyV2PlaceConfigs["oceanRuin_bedrock_legacy"]
		} else if ed == yyV2EditionJava && versionCode < 101600 {
			cfg = yyV2PlaceConfigs["oceanRuin_legacy"]
		}
	case "pillagerOutpost":
		if ed == yyV2EditionBedrock {
			cfg = yyV2PlaceConfigs["pillagerOutpost_bedrock"]
		}
	case "ruinedPortalOW":
		// 注意：参考实现没区分版本类型，Java/Bedrock 都按 1.18 分界
		if versionCode < 101800 {
			cfg = yyV2PlaceConfigs["ruinedPortalOW_legacy"]
		}
	case "ruinedPortalNether":
		if ed == yyV2EditionBedrock {
			cfg = yyV2PlaceConfigs["ruinedPortalNether_bedrock"]
		}
	case "desertWell":
		if versionCode < 101500 {
			cfg = yyV2PlaceConfigs["desertWell_115"]
		} else if versionCode < 101800 {
			cfg = yyV2PlaceConfigs["desertWell_117"]
		}
	}

	// 1.13 前的盐统一为 14357617
	if ed == yyV2EditionJava && versionCode < 101300 {
		switch cfgName {
		case "desertTemple", "jungleTemple", "igloo", "witchHut", "oceanRuin":
			cfg.Salt = yyV2LegacySalt
		}
	}
	return cfg, true
}

// yyV2PlaceAvailable 该版本+版本类型下结构是否可用。
// 与 world.mjs 的 isStructureAvailable 一致：先看版本表（有区间就算可用，
// 即使没有 region 配置——要塞/矿井/化石靠各自的算法），再看配置是否存在。
func yyV2PlaceAvailable(name string, versionCode int32, edition string) bool {
	ed := yyV2NormalizeEdition(edition)
	if vr, ok := yyV2PlaceVersions[name]; ok {
		rng := vr.Java
		if ed == yyV2EditionBedrock {
			rng = vr.Bedrock
		}
		if versionCode > 0 && (versionCode < rng[0] || versionCode > rng[1]) {
			return false
		}
		return true
	}
	_, ok := yyV2PlaceConfigFor(name, versionCode, ed)
	return ok
}

// ── RNG 辅助 ──────────────────────────────────────────────────────────

// yyV2PlaceNextDouble = Java Random.nextDouble()：(next(26)*2^27 + next(27)) / 2^53。
// seedv2_rand.go 的 NextDouble 目前漏了除法（别人的文件，不动），这里私有实现兜底；
// 两位对齐后可直接换成 r.NextDouble()。
func yyV2PlaceNextDouble(r *yyV2RNG) float64 {
	return (float64(r.Next(26))*134217728.0 + float64(r.Next(27))) / 9007199254740992.0
}

// yyV2PlaceRegionSeed regionSeed = rx*341873128712 + rz*132897987541 + worldSeed + salt
// （uint64 回绕；负 region 走二进制补码，与 BigInt.asUintN(64, ...) 等价）
func yyV2PlaceRegionSeed(worldSeed int64, salt int64, rx, rz int) uint64 {
	return uint64(worldSeed) + uint64(salt) +
		uint64(int64(rx))*yyV2RegionMulX + uint64(int64(rz))*yyV2RegionMulZ
}

// yyV2PlaceXi cubiomes 的 xi()：向零取整（Go 的 float64→int 就是向零截断）。
func yyV2PlaceXi(e float64) int { return int(e) }

// yyV2PlaceFloorDiv 向下取整除（负数时 Go 的 / 是向零截断，不能直接用）。
func yyV2PlaceFloorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// yyV2PlacePos 取 region 内位置（不含 RNG）。
func yyV2PlacePos(worldSeed int64, cfg yyV2PlaceCfg, rx, rz int) (int, int) {
	x, z, _ := yyV2PlacePosWithRng(worldSeed, cfg, rx, rz)
	return x, z
}

// yyV2PlacePosWithRng 取 region 内位置并把 RNG 交出去（下界要塞/堡垒的第三次
// nextInt(5) 就在同一个 rng 上抽，不能重新播种）。
func yyV2PlacePosWithRng(worldSeed int64, cfg yyV2PlaceCfg, rx, rz int) (int, int, *yyV2RNG) {
	r := yyV2NewRNG(int64(yyV2PlaceRegionSeed(worldSeed, cfg.Salt, rx, rz)))
	span := cfg.RegionSize - cfg.Separation
	var ox, oz int
	if cfg.LargeSpread {
		// 三角分布：每轴 2 次 nextInt 取平均再向零取整（顺序：x 两次、z 两次）
		ox = yyV2PlaceXi(float64(r.NextInt(int32(span))+r.NextInt(int32(span))) / 2)
		oz = yyV2PlaceXi(float64(r.NextInt(int32(span))+r.NextInt(int32(span))) / 2)
	} else {
		ox = int(r.NextInt(int32(span)))
		oz = int(r.NextInt(int32(span)))
	}
	return rx*cfg.RegionSize + ox, rz*cfg.RegionSize + oz, r
}

// ── 结构元素信息表 ────────────────────────────────────────────────────

type yyV2PlaceMeta struct {
	ID        string
	Name      string
	Dim       string
	Color     [3]uint8
	Precision string // exact / candidate
	Placement string // region / ring / per_chunk / biome_scan
	Default   bool
}

// 顺序即前端展示顺序（与参考实现 STRUCT_META / 任务书一致）。
var yyV2PlaceMetas = []yyV2PlaceMeta{
	{"village", "村庄", yyV2DimOverworld, [3]uint8{255, 60, 60}, "exact", "region", true},
	{"woodlandMansion", "林地府邸", yyV2DimOverworld, [3]uint8{160, 82, 45}, "exact", "region", true},
	{"oceanMonument", "海底神殿", yyV2DimOverworld, [3]uint8{60, 200, 255}, "exact", "region", true},
	{"endCity", "末地城", yyV2DimEnd, [3]uint8{200, 120, 255}, "exact", "region", true},
	{"endShip", "末地船", yyV2DimEnd, [3]uint8{237, 231, 246}, "candidate", "region", true},
	{"desertTemple", "沙漠神殿", yyV2DimOverworld, [3]uint8{255, 180, 60}, "exact", "region", true},
	{"jungleTemple", "丛林神庙", yyV2DimOverworld, [3]uint8{120, 255, 80}, "exact", "region", true},
	{"igloo", "雪屋", yyV2DimOverworld, [3]uint8{200, 240, 255}, "exact", "region", true},
	{"witchHut", "女巫小屋", yyV2DimOverworld, [3]uint8{80, 255, 120}, "exact", "region", true},
	{"pillagerOutpost", "掠夺者前哨", yyV2DimOverworld, [3]uint8{200, 120, 255}, "exact", "region", true},
	{"shipwreck", "沉船", yyV2DimOverworld, [3]uint8{200, 200, 80}, "exact", "region", true},
	{"oceanRuin", "海底废墟", yyV2DimOverworld, [3]uint8{80, 180, 220}, "exact", "region", true},
	{"ancientCity", "远古城市", yyV2DimOverworld, [3]uint8{90, 200, 200}, "exact", "region", true},
	{"trailRuin", "古迹废墟", yyV2DimOverworld, [3]uint8{200, 170, 120}, "candidate", "region", true},
	{"trialChamber", "试炼密室", yyV2DimOverworld, [3]uint8{255, 150, 200}, "candidate", "region", true},
	// 埋藏宝藏：配置是 spacing/separation 都 1（range=0），参考实现枚举解出 NaN →
	// 候选恒空。要与 chunkbase 对齐得靠 biome 过滤的 oracle，故标 candidate。
	{"buriedTreasure", "埋藏的宝藏", yyV2DimOverworld, [3]uint8{255, 220, 60}, "candidate", "region", true},
	{"netherFortress", "下界要塞", yyV2DimNether, [3]uint8{180, 60, 60}, "exact", "region", true},
	{"bastionRemnant", "堡垒遗迹", yyV2DimNether, [3]uint8{220, 100, 100}, "exact", "region", true},
	{"ruinedPortalOW", "废弃传送门(主世界)", yyV2DimOverworld, [3]uint8{180, 120, 220}, "exact", "region", true},
	{"ruinedPortalNether", "废弃传送门(下界)", yyV2DimNether, [3]uint8{140, 90, 200}, "exact", "region", true},
	{"stronghold", "要塞", yyV2DimOverworld, [3]uint8{255, 255, 0}, "exact", "ring", true},
	// 矿井/化石/晶洞在原版是逐区块判定（走 WorldGen 的 chance），
	// 参考实现没有对应算法，也不在 STRUCTURE_CONFIG 里 → 只列出来，扫描返回空，
	// 精度标 candidate，别让前端以为能精确出点。
	{"mineshaft", "废弃矿井", yyV2DimOverworld, [3]uint8{150, 150, 150}, "candidate", "per_chunk", false},
	{"fossil", "化石", yyV2DimOverworld, [3]uint8{200, 200, 180}, "candidate", "per_chunk", false},
	{"amethystGeode", "紫水晶晶洞", yyV2DimOverworld, [3]uint8{180, 130, 220}, "candidate", "per_chunk", false},
	{"desertWell", "沙漠水井", yyV2DimOverworld, [3]uint8{255, 200, 120}, "candidate", "region", false},
	{"abandonedCamp", "废弃营地", yyV2DimOverworld, [3]uint8{220, 160, 90}, "candidate", "region", false},
	{"spawn", "出生点", yyV2DimOverworld, [3]uint8{255, 255, 255}, "candidate", "biome_scan", false},
	{"slimeChunk", "史莱姆区块", yyV2DimOverworld, [3]uint8{120, 220, 120}, "exact", "per_chunk", false},
}

// ── 别名归一 ──────────────────────────────────────────────────────────
//
// 参考实现的 STRUCT_ALIAS 一张不少，另外补上「下划线写法 / minecraft: 前缀 /
// 中文名」，前端传中文或简写也能查到。

var yyV2PlaceAliasRaw = map[string]string{
	// world.mjs / server.mjs STRUCT_ALIAS
	"mansion":               "woodlandMansion",
	"monument":              "oceanMonument",
	"outpost":               "pillagerOutpost",
	"fortress":              "netherFortress",
	"bastion":               "bastionRemnant",
	"ruinedportal":          "ruinedPortalOW",
	"ruinedportaloverworld": "ruinedPortalOW",
	"camp":                  "abandonedCamp",
	"geode":                 "amethystGeode",
	"slime":                 "slimeChunk",
	"oceanruin":             "oceanRuin",
	"endcity":               "endCity",
	"endship":               "endShip",
	// 中文名
	"村庄":         "village",
	"林地府邸":       "woodlandMansion",
	"府邸":         "woodlandMansion",
	"海底神殿":       "oceanMonument",
	"海底遗迹":       "oceanRuin",
	"海底废墟":       "oceanRuin",
	"末地城":        "endCity",
	"末地船":        "endShip",
	"沙漠神殿":       "desertTemple",
	"丛林神庙":       "jungleTemple",
	"雪屋":         "igloo",
	"冰屋":         "igloo",
	"女巫小屋":       "witchHut",
	"掠夺者前哨":      "pillagerOutpost",
	"前哨":         "pillagerOutpost",
	"沉船":         "shipwreck",
	"远古城市":       "ancientCity",
	"古迹废墟":       "trailRuin",
	"考古遗迹":       "trailRuin",
	"试炼密室":       "trialChamber",
	"埋藏的宝藏":      "buriedTreasure",
	"埋藏宝藏":       "buriedTreasure",
	"下界要塞":       "netherFortress",
	"堡垒遗迹":       "bastionRemnant",
	"废弃传送门":      "ruinedPortalOW",
	"废弃传送门(主世界)": "ruinedPortalOW",
	"废弃传送门(下界)":  "ruinedPortalNether",
	"要塞":         "stronghold",
	"废弃矿井":       "mineshaft",
	"矿井":         "mineshaft",
	"化石":         "fossil",
	"紫水晶晶洞":      "amethystGeode",
	"紫水晶洞":       "amethystGeode",
	"沙漠水井":       "desertWell",
	"水井":         "desertWell",
	"废弃营地":       "abandonedCamp",
	"出生点":        "spawn",
	"史莱姆区块":      "slimeChunk",
}

// yyV2PlaceAlias 归一化后的查表：key 已去分隔符+小写。
var yyV2PlaceAlias = func() map[string]string {
	m := make(map[string]string, len(yyV2PlaceMetas)+len(yyV2PlaceAliasRaw))
	for _, meta := range yyV2PlaceMetas {
		m[yyV2PlaceAliasKey(meta.ID)] = meta.ID
	}
	// 数组里出现的每个 meta 的中文名也当别名
	for _, meta := range yyV2PlaceMetas {
		if _, ok := m[yyV2PlaceAliasKey(meta.Name)]; !ok {
			m[yyV2PlaceAliasKey(meta.Name)] = meta.ID
		}
	}
	for k, v := range yyV2PlaceAliasRaw {
		m[yyV2PlaceAliasKey(k)] = v
	}
	return m
}()

// yyV2PlaceAliasKey 去 "minecraft:" 前缀、去掉 _ - 空格、转小写。
func yyV2PlaceAliasKey(raw string) string {
	s := yyV2Lower(raw)
	if len(s) > 10 && s[:10] == "minecraft:" {
		s = s[10:]
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || c == '-' || c == ' ' || c == '\t' {
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

// yyV2PlaceCanonical 别名 → 规范 id；未知返回 ""。
func yyV2PlaceCanonical(id string) string {
	if id == "" {
		return ""
	}
	return yyV2PlaceAlias[yyV2PlaceAliasKey(id)]
}

func yyV2PlaceMetaByID(id string) (yyV2PlaceMeta, bool) {
	for _, meta := range yyV2PlaceMetas {
		if meta.ID == id {
			return meta, true
		}
	}
	return yyV2PlaceMeta{}, false
}

// ── 对外：结构清单 / 元信息 ────────────────────────────────────────────

// yyV2StructureList 该版本+版本类型下可用的结构清单（顺序即前端展示顺序）。
// 不可用的结构不要出现在结果里 —— 让前端能区分"这个版本没有这个结构"。
func yyV2StructureList(versionCode int32, edition string) []yyV2StructureInfo {
	ed := yyV2NormalizeEdition(edition)
	out := make([]yyV2StructureInfo, 0, len(yyV2PlaceMetas))
	for _, meta := range yyV2PlaceMetas {
		if !yyV2PlaceAvailable(meta.ID, versionCode, ed) {
			continue
		}
		out = append(out, yyV2PlaceInfo(meta))
	}
	return out
}

func yyV2PlaceInfo(meta yyV2PlaceMeta) yyV2StructureInfo {
	info := yyV2StructureInfo{
		ID:        meta.ID,
		Name:      meta.Name,
		Dim:       meta.Dim,
		Color:     meta.Color,
		Precision: meta.Precision,
		Placement: meta.Placement,
		Default:   meta.Default,
	}
	if vr, ok := yyV2PlaceVersions[meta.ID]; ok {
		info.Java = vr.Java
		info.Bedrock = vr.Bedrock
	}
	return info
}

// yyV2StructureInfoByID 按 id 取结构元信息（含别名归一）。
// 已知 id 一律返回 true（不管版本是否支持），版本可用性由 Java/Bedrock 字段表达 ——
// 渲染层拿颜色时不该因为版本分支拿不到配置。
func yyV2StructureInfoByID(id string, versionCode int32, edition string) (yyV2StructureInfo, bool) {
	_ = versionCode
	_ = edition
	canon := yyV2PlaceCanonical(id)
	if canon == "" {
		return yyV2StructureInfo{}, false
	}
	meta, ok := yyV2PlaceMetaByID(canon)
	if !ok {
		return yyV2StructureInfo{}, false
	}
	return yyV2PlaceInfo(meta), true
}

// ── 对外：扫描 ────────────────────────────────────────────────────────

const (
	yyV2PlaceDefaultLimit = 20000
	// 单个请求最多枚举的 region 数，防止 ±百万区块的入参把进程拖死
	yyV2PlaceMaxRegions = 1 << 20
)

// yyV2ScanStructures 扫描区块矩形内的结构点。
// 返回的 X/Z 一律是**区块坐标**。limit<=0 时用默认上限（20000）。
// 越界/超量必须截断而不是报错 —— 前端滑动地图不能因为范围大就 500。
//
// 已实现：region 放置类全部结构 + 要塞环 + 史莱姆区块 + 出生点粗估。
// 未实现（返回空，不伪造）：mineshaft / fossil / amethystGeode。
func yyV2ScanStructures(seed int64, id string, versionCode int32, edition string, cx0, cz0, cx1, cz1, limit int) []yyV2StructPos {
	canon := yyV2PlaceCanonical(id)
	if canon == "" {
		return nil
	}
	if cx0 > cx1 {
		cx0, cx1 = cx1, cx0
	}
	if cz0 > cz1 {
		cz0, cz1 = cz1, cz0
	}
	if limit <= 0 {
		limit = yyV2PlaceDefaultLimit
	}
	ed := yyV2NormalizeEdition(edition)

	switch canon {
	case "stronghold":
		return yyV2PlaceScanStrongholds(seed, versionCode, cx0, cz0, cx1, cz1, limit)
	case "spawn":
		sx, sz := yyV2SpawnPointV2(seed)
		if sx < cx0 || sx > cx1 || sz < cz0 || sz > cz1 {
			return nil
		}
		return []yyV2StructPos{{X: sx, Z: sz, Type: "spawn", Variant: "not_vanilla_algorithm"}}
	case "slimeChunk":
		return yyV2PlaceScanSlime(seed, cx0, cz0, cx1, cz1, limit)
	case "mineshaft", "fossil", "amethystGeode":
		// 逐区块概率判定，参考实现没有算法 → 不伪造
		return nil
	}

	cfg, ok := yyV2PlaceConfigFor(canon, versionCode, ed)
	if !ok {
		return nil
	}
	// 参考实现里 buriedTreasure / desertWell 是 regionSize=1、separation=1 →
	// nextInt(0) 在 JS 里得 NaN，结构候选恒为空。这里保持一致（不擅自改成每区块
	// 一个候选），它们本来就是 biome 过滤型的 candidate。
	if cfg.RegionSize-cfg.Separation <= 0 {
		return nil
	}

	// 末地船：位置与末地城共用；hasShip 需要区块高度图（高度图=地形，纯放置层
	// 拿不到），这里只返回「可能有船」的城，Variant 显式标注未判定。
	isShip := canon == "endShip"

	netherPair := canon == "netherFortress" || canon == "bastionRemnant"

	out := make([]yyV2StructPos, 0, 64)
	iter := 0
	rx0 := yyV2PlaceFloorDiv(cx0, cfg.RegionSize)
	rx1 := yyV2PlaceFloorDiv(cx1, cfg.RegionSize)
	rz0 := yyV2PlaceFloorDiv(cz0, cfg.RegionSize)
	rz1 := yyV2PlaceFloorDiv(cz1, cfg.RegionSize)

	for rx := rx0; rx <= rx1; rx++ {
		for rz := rz0; rz <= rz1; rz++ {
			iter++
			if iter > yyV2PlaceMaxRegions {
				goto done
			}
			x, z, rng := yyV2PlacePosWithRng(seed, cfg, rx, rz)
			if x < cx0 || x > cx1 || z < cz0 || z > cz1 {
				continue
			}
			typ := canon
			if netherPair {
				// 同一 region 第三次 nextInt(5) 分流：<2 要塞，>=2 堡垒
				// （1.16–1.17 已校准；1.18+ 是近似，DEV_NOTES §3.9）
				typ = "netherFortress"
				if rng.NextInt(5) >= 2 {
					typ = "bastionRemnant"
				}
				if typ != canon {
					continue
				}
			}
			p := yyV2StructPos{X: x, Z: z, Type: typ}
			if isShip {
				p.Type = "endShip"
				p.Variant = "hasShip_unknown"
			}
			out = append(out, p)
		}
	}
done:
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Z != out[j].Z {
			return out[i].Z < out[j].Z
		}
		return out[i].X < out[j].X
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func yyV2PlaceScanStrongholds(seed int64, versionCode int32, cx0, cz0, cx1, cz1, limit int) []yyV2StructPos {
	all := yyV2StrongholdsV2(seed, versionCode)
	out := make([]yyV2StructPos, 0, len(all))
	for _, p := range all {
		if p[0] < cx0 || p[0] > cx1 || p[1] < cz0 || p[1] > cz1 {
			continue
		}
		out = append(out, yyV2StructPos{X: p[0], Z: p[1], Type: "stronghold"})
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func yyV2PlaceScanSlime(seed int64, cx0, cz0, cx1, cz1, limit int) []yyV2StructPos {
	out := make([]yyV2StructPos, 0, 64)
	for z := cz0; z <= cz1; z++ {
		for x := cx0; x <= cx1; x++ {
			if !yyV2IsSlimeChunkV2(seed, int32(x), int32(z)) {
				continue
			}
			out = append(out, yyV2StructPos{X: x, Z: z, Type: "slimeChunk"})
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

// ── 要塞环 ────────────────────────────────────────────────────────────

// yyV2StrongholdsV2 要塞位置（cubiomes initFirstStronghold + nextStronghold
// 逐位复刻，128 个）。返回区块坐标。这比"三环近似"准得多，是 exact 级。
//
// 注意：环算法只是「大致位置」。1.18+ 原版还会在允许生物群系里搜最近点，
// 最终坐标会被 biome 拉走；要精确坐标得配合 biome 引擎再看。
func yyV2StrongholdsV2(seed int64, versionCode int32) [][2]int {
	rng := yyV2NewRNG(seed)
	out := make([][2]int, 0, 128)

	angle := 2 * math.Pi * yyV2PlaceNextDouble(rng)
	var dist float64
	if versionCode >= 100900 {
		dist = (4.0 * 32.0) + (yyV2PlaceNextDouble(rng)-0.5)*32*2.5 // 1.9+
	} else {
		dist = (1.25 + yyV2PlaceNextDouble(rng)) * 32.0 // 1.8 及更早
	}
	ringnum := 0
	ringmax := 3
	ringidx := 0

	push := func() {
		// 返回**区块**坐标（与 API 其他结构一致）：
		// cubiomes 内部是 round(cos*dist)*16+8（方块），这里回到区块。
		// Math.round 是「x.5 向 +∞」，Go 用 floor(v+0.5) 复刻。
		out = append(out, [2]int{
			int(math.Floor(math.Cos(angle)*dist + 0.5)),
			int(math.Floor(math.Sin(angle)*dist + 0.5)),
		})
	}
	push() // 第 1 个

	for i := 1; i < 128; i++ {
		ringidx++
		if ringidx == ringmax {
			ringidx = 0
			ringnum++
			angle += yyV2PlaceNextDouble(rng) * math.Pi * 2
			ringmax += (2 * ringmax) / (ringnum + 1)
		}
		angle += (2 * math.Pi) / float64(ringmax)
		// 注意：(ringnum*6 + nextDouble() - 0.5) * 80 —— ring 的系数是 480，不是 192
		dist = (4.0 * 32.0) + (float64(ringnum*6)+yyV2PlaceNextDouble(rng)-0.5)*32*2.5
		push()
	}
	return out
}

// ── 史莱姆 ────────────────────────────────────────────────────────────

// yyV2IsSlimeChunkV2 Java 版史莱姆区块判定（exact）。
//
// 公式：new JRandom(seed + x²*0x4c1906 + x*0x5ac0db + z²*0x4307a7 + z*0x5f24f
//
//	^ 0x3ad8025f).nextInt(10) === 0
//
// Java 的 `^` 优先级低于 `+`：**先求和再异或**，不是逐项异或。
// 全部按 64 位回绕算（JS 端是 BigInt 精确值再 asUintN(64)，模 2^64 等价）。
func yyV2IsSlimeChunkV2(seed int64, cx, cz int32) bool {
	x := int64(cx)
	z := int64(cz)
	sum := uint64(seed)
	sum += uint64(x*x) * 0x4c1906
	sum += uint64(x) * 0x5ac0db
	sum += uint64(z*z) * 0x4307a7
	sum += uint64(z) * 0x5f24f
	sum ^= 0x3ad8025f
	r := yyV2NewRNG(int64(sum))
	return r.NextInt(10) == 0
}

// ── 末地城高度 ────────────────────────────────────────────────────────

// yyV2EndCityHeight 末地城「能否生成 / 生成高度基准」（chunkbase 的 wo()）。
//
// 【依赖 heightmap】参数 heightmap 必须是该区块的 16×16 高度列，行主序
// （idx = z*16 + x，长度至少 256）。Go 侧没有区块高度数据，调用方要自己从
// 地形/存档拿；没有高度图就不要调它，别用 y=0 冒充。
//
// 返回 (高度, true)；heightmap 长度不足返回 (0,false)。
//
// 播种：
//
//	Java ≥ 1.19 → seed = (cx*l) ^ (cz*m) ^ worldSeed，l/m = JavaRandom(worldSeed)
//	              的前两个 nextLong()（低 32 位按 i32 符号扩展，yyV2RNG.NextLong 已对齐）
//	Java ≤ 1.18 → seed = 10387313*cz + cx（注意顺序是 z 先乘）
//
// 采样点（docs/structures/END_CITY.md §3.2 (b)，s 作用在 z、c 作用在 x）：
//
//	(7,7) (7+c,7) (7,7+s) (7+c,7+s) 取 min。
//
// world.mjs 现版本把 s/c 写反（§Bug 2 有定量证据），本函数按修正版实现。
// Bedrock 的 RNG 是 MT19937，Go 侧未实现 → 只在 Java 版可用。
func yyV2EndCityHeight(chunkX, chunkZ int, heightmap []int16, worldSeed int64, versionCode int32) (int, bool) {
	if len(heightmap) < 256 {
		return 0, false
	}
	var seed int64
	if versionCode >= 101900 {
		r := yyV2NewRNG(worldSeed)
		l := r.NextLong()
		m := r.NextLong()
		seed = int64(uint64(int64(chunkX))*uint64(l) ^ uint64(int64(chunkZ))*uint64(m) ^ uint64(worldSeed))
	} else {
		seed = int64(10387313)*int64(chunkZ) + int64(chunkX)
	}
	rng := yyV2NewRNG(seed)
	o := rng.NextInt(4)
	s, c := 5, 5
	switch o {
	case 1:
		s = -5
	case 2:
		s = -5
		c = -5
	case 3:
		c = -5
	}
	idx := func(x, z int) int16 { return heightmap[z*16+x] }
	h := idx(7, 7)
	if v := idx(7+c, 7); v < h {
		h = v
	}
	if v := idx(7, 7+s); v < h {
		h = v
	}
	if v := idx(7+c, 7+s); v < h {
		h = v
	}
	return int(h), true
}

// ── 出生点 ────────────────────────────────────────────────────────────

// yyV2SpawnPointV2 出生点区块坐标（弱精度，标注 candidate）。
//
// 【不是原版算法】原版出生点是在 (0,0) 附近按 biome 权重 + 螺旋搜索 + 高度
// 修正选出来的，参考实现里也没有。这里只做「以 (0,0) 为中心、按区块螺旋
// 找第一个非海洋/非河流群系区块」的粗估，且依赖 yyV2NewBiomeGen
// （生物群系引擎就位后才有意义）。找不到就退回 (0,0)。
// 前端必须把它当候选，不能当精确出生点宣传。
func yyV2SpawnPointV2(seed int64) (int, int) {
	gen := yyV2NewBiomeGen(seed, 102104, false)
	// 螺旋：r=0 是原点，每圈按 z 行再 x 列扫，最多 64 区块半径
	for r := 0; r <= 64; r++ {
		for z := -r; z <= r; z++ {
			for x := -r; x <= r; x++ {
				// 只扫当前圈（内圈已扫过）
				if r > 0 && x > -r && x < r && z > -r && z < r {
					continue
				}
				if !yyV2PlaceIsOceanBiome(gen.BiomeAtChunk(x, z)) {
					return x, z
				}
			}
		}
	}
	return 0, 0
}

// yyV2PlaceIsOceanBiome 判定是否「不适合出生」的群系（海洋/河流/沙滩）。
func yyV2PlaceIsOceanBiome(id int32) bool {
	switch id {
	case 0, 7, 10, 11, 16, 24, 26, 44, 45, 46, 47, 48, 49, 50:
		return true
	}
	return false
}
