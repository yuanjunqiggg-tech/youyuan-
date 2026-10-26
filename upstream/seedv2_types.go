package main

// ═══════════════════════════════════════════════════════════════════════
// 种子查询 v2 · 跨文件契约（2026-10-26 第 28 条）
//
// 本文件只放「多个实现文件都要用」的类型 + 几个纯工具函数。
// 各实现文件内部的私有类型/函数放各自文件，不要往这里堆。
//
// 【禁止改名/删除】下列符号被其它文件引用，改名会导致整包编译失败：
//   yyV2StructPos / yyV2StructureInfo / yyV2BTree
//   yyV2NewRNG / yyV2RNG.Next / .NextInt / .NextLong / .NextDouble
//   yyV2NewClimate / yyV2Climate.Axes
//   yyV2VoronoiSHA / yyV2VoronoiAccess3D
//   yyV2BTreeByName
//   yyV2NewBiomeGen / yyV2BiomeGen.BiomeAtBlock / .BiomeAtChunk
//   yyV2StructureList / yyV2StructureInfoByID
//   yyV2ScanStructures / yyV2StrongholdsV2 / yyV2IsSlimeChunkV2 / yyV2SpawnPointV2
//   yyV2RenderTilePNG
//
// 版本码（versionCode）约定，与参考实现 src/world.mjs 完全一致：
//   java    1.21.4 → 102104 ；Bedrock 26.50 → 265000
//   1.7~1.21 段：major*100000 + minor*100 + patch
//   26.x 段    ：260000 + minor*100 + patch，且 260000 归一为 260100
// ═══════════════════════════════════════════════════════════════════════

const (
	yyV2EditionJava    = "java"
	yyV2EditionBedrock = "bedrock"
)

// 三个维度的内部键（与前端、MCP、插件参数统一）
const (
	yyV2DimOverworld = "overworld"
	yyV2DimNether    = "nether"
	yyV2DimEnd       = "end"
)

// yyV2StructPos 一个结构点。X/Z 一律是【区块坐标】。
type yyV2StructPos struct {
	X       int    `json:"x"`
	Z       int    `json:"z"`
	Type    string `json:"type"`
	Variant string `json:"variant,omitempty"`
	// HasShip 仅末地城有意义：该末地城是否带末地船
	HasShip bool `json:"has_ship,omitempty"`
	// Zombie 仅村庄有意义：是否为僵尸村
	Zombie bool `json:"zombie,omitempty"`
}

// yyV2StructureInfo 结构元信息（中文名、维度、版本区间、精度分级）。
type yyV2StructureInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Dim  string `json:"dim"`
	// Color 渲染颜色（图标底衬）
	Color [3]uint8 `json:"-"`
	// Java/Bedrock 版本可用区间 [lo, hi]，0 表示不限
	Java    [2]int32 `json:"java"`
	Bedrock [2]int32 `json:"bedrock"`
	// Precision: exact（逐位可复现）/ candidate（区域候选，需进游戏确认）
	Precision string `json:"precision"`
	// Placement: region（区域网格）/ ring（要塞环）/ per_chunk（逐区块）/ biome_scan
	Placement string `json:"placement"`
	// Default 是否默认勾选
	Default bool `json:"default"`
}

// yyV2BTree 生物群系 climate 参数检索树（由 seedv2_btree.dat 反序列化）。
type yyV2BTree struct {
	Name  string
	Order int
	Steps []int32
	// Param[i] = [lo, hi]
	Param [][2]int32
	// Nodes 每个 uint64：低 48 位 = 6 个参数下标（每字节一个），高 16 位 = 子节点下标或叶生物群系 id
	Nodes []uint64
}

// yyV2RenderOptions 瓦片渲染参数。
type yyV2RenderOptions struct {
	Seed        int64
	VersionCode int32
	Edition     string
	Dimension   string
	LargeBiomes bool
	// CX/CZ 瓦片左上角【区块坐标】
	CX, CZ int
	// W/H 输出像素尺寸
	W, H int
	// BlocksPerPixel 每像素代表多少方块（1/2/4/8/16/32），渲染瓦片时用
	BlocksPerPixel int
	// Layer: "terrain" 只画地形，"structures" 只画结构，"" 两者都画
	Layer string
	// StructureIDs 参与绘制的结构 ID（空 = 该维度全部默认结构）
	StructureIDs []string
	// MaxIcons 图标数量上限
	MaxIcons int
}

// ── 版本工具（多文件共用，实现在本文件）─────────────────────────────

// yyV2ParseVersion 把 "1.21.4" / "26.50" 解析成版本码。
// 与参考实现 parseVersion / parseBedrockVersion 完全一致：两版公式相同但不许合并，
// 因为历史上两边漂过（Bedrock 曾用 100000+minor*10，导致 1.18~1.21 全部渲染失败）。
func yyV2ParseVersion(raw string, edition string) int32 {
	major, minor, patch, ok := yyV2SplitVersion(raw)
	if !ok {
		if edition == yyV2EditionBedrock {
			return 265000
		}
		return 102104
	}
	var code int32
	if major >= 26 {
		code = 260000 + int32(minor)*100 + int32(patch)
		// 引擎不接受 260000（26.0），用 26.1 代表「26.0 - 26.23」这一档
		if code == 260000 {
			code = 260100
		}
		return code
	}
	code = int32(major)*100000 + int32(minor)*100 + int32(patch)
	if code == 100000 {
		// "1.0" 之类明显非法的输入
		return 102104
	}
	return code
}

// yyV2SplitVersion 拆 "1.21.4" → (1,21,4,true)。
func yyV2SplitVersion(raw string) (major, minor, patch int, ok bool) {
	var nums [3]int
	var filled int
	cur := -1
	seenDot := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c >= '0' && c <= '9' {
			if cur < 0 {
				cur = 0
			}
			cur = cur*10 + int(c-'0')
			continue
		}
		if c == '.' {
			if cur < 0 || filled >= 3 {
				return 0, 0, 0, false
			}
			nums[filled] = cur
			filled++
			cur = -1
			seenDot = true
			continue
		}
		// 出现非数字非点（例如 "1.21.4-beta"）：截断，按已读到的部分解析
		break
	}
	if cur >= 0 {
		nums[filled] = cur
		filled++
	}
	if filled == 0 || !seenDot {
		// 没有小版本号的一律视为非法（避免 "21" 被当成 21.0.0）
		return 0, 0, 0, false
	}
	return nums[0], nums[1], nums[2], true
}

// yyV2NormalizeEdition 归一版本类型；空值默认 java。
func yyV2NormalizeEdition(raw string) string {
	switch yyV2Lower(raw) {
	case "bedrock", "be", "基岩", "基岩版":
		return yyV2EditionBedrock
	default:
		return yyV2EditionJava
	}
}

// yyV2NormalizeDimension 归一维度键；接受 overworld/主世界/0、nether/下界/1、end/末地/2。
func yyV2NormalizeDimension(raw string) string {
	switch yyV2Lower(raw) {
	case "nether", "the_nether", "minecraft:nether", "the nether", "下界", "地狱", "1":
		return yyV2DimNether
	case "end", "the_end", "minecraft:end", "the end", "末地", "末路之地", "2":
		return yyV2DimEnd
	default:
		return yyV2DimOverworld
	}
}

// yyV2Lower 只处理 ASCII 的小写化，不引入 strings 依赖以保持本文件零 import。
func yyV2Lower(raw string) string {
	b := []byte(raw)
	changed := false
	for i := 0; i < len(b); i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
			changed = true
		}
	}
	if !changed {
		return raw
	}
	return string(b)
}
