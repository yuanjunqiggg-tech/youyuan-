package main

// 结构注册表（全量）。
//
// 覆盖目标（用户要求「所有它的结构 / 不同维度 / 所有覆盖的版本」）：
//   - 主世界：14 种随机网格结构 + 要塞（三环同心）+ 废弃矿井（区块概率）
//   - 下界：下界要塞 / 堡垒遗迹 / 下界化石
//   - 末地：末地城
//
// 数据来源：Minecraft Wiki「Structure」各结构 placement 的
// spacing / separation / salt / spread_type 公开值，取 Java 1.13~1.21 主流参数。
// 不同版本会微调参数（例如 1.18、1.19 对部分结构改过 spacing），
// 因此网格类结果一律 candidate，必须进游戏实测确认——不许冒充精确。

import (
	"sort"
	"strings"
	"time"
)

// 维度常量。与 /api/seed/biome 的 dimension 取值保持一致。
const (
	yyStructDimOverworld = "overworld"
	yyStructDimNether    = "nether"
	yyStructDimEnd       = "the_end"
)

// 放置类型。
const (
	// yyStructKindSpread 区域网格 + 盐值（原版 RandomSpreadStructurePlacement，1.13+）
	yyStructKindSpread = "random_spread"
	// yyStructKindConcentric 同心环：要塞三环半径 + 每环 128 候选角
	yyStructKindConcentric = "concentric"
	// yyStructKindProbability 每区块独立概率，没有网格，给不出确定坐标
	yyStructKindProbability = "probability"
)

// yySeedStructureSpec 是一种结构的放置参数。
//
// Spacing/Separation/Salt 只在 Kind=random_spread 时有意义；
// Kind=concentric 时 Spacing/Separation 记录原版同心环的网格参数（仅作展示）；
// Kind=probability 时用 Probability 表示每区块概率。
type yySeedStructureSpec struct {
	ID          string
	Label       string
	Spacing     int
	Separation  int
	Salt        int64
	Dimension   string
	Kind        string
	Since       string
	Probability float64
	Note        string
}

// yySeedStructureAliases 历史别名 → 真实 ID。
//
// temple 是 1.13+ 共用同一套网格参数（32/8/14357617）的四种结构：
// 沙漠神殿 / 丛林神庙 / 女巫小屋 / 雪屋。它们候选区块完全相同，
// 差别只在生物群系过滤，因此别名到 desert_pyramid 不会丢点位，只影响显示名。
var yySeedStructureAliases = map[string]string{
	"temple":            "desert_pyramid",
	"pyramid":           "desert_pyramid",
	"desert_temple":     "desert_pyramid",
	"jungle_temple":     "jungle_pyramid",
	"witch_hut":         "swamp_hut",
	"swamp_hut_hut":     "swamp_hut",
	"outpost":           "pillager_outpost",
	"pillager":          "pillager_outpost",
	"woodland_mansion":  "mansion",
	"ocean_monument":    "monument",
	"sea_monument":      "monument",
	"nether_fortress":   "fortress",
	"fortress_nether":   "fortress",
	"bastion_remnant":   "bastion",
	"endcity":           "end_city",
	"end_cities":        "end_city",
	"ruin":              "ocean_ruin",
	"ocean_ruins":       "ocean_ruin",
	"treasure":          "buried_treasure",
	"portal":            "ruined_portal",
	"city":              "ancient_city",
	"deep_dark_city":    "ancient_city",
	"trail_ruin":        "trail_ruins",
	"trial_chamber":     "trial_chambers",
	"fossil":            "nether_fossil",
	"mine":              "mineshaft",
	"abandoned_mineshaft": "mineshaft",
}

// yySeedStructureSpecs 返回支持的全部结构（不含别名）。
//
// 顺序刻意按「维度 → 出现频率」排，前端下拉直接照这个顺序展示。
func yySeedStructureSpecs() []yySeedStructureSpec {
	return []yySeedStructureSpec{
		// ── 主世界：随机网格 ──
		{ID: "village", Label: "村庄", Spacing: 34, Separation: 8, Salt: 10387312,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.13",
			Note: "1.13+ 网格 34/8；1.18 前后参数未变，但村庄生物群系判定变了"},
		{ID: "desert_pyramid", Label: "沙漠神殿", Spacing: 32, Separation: 8, Salt: 14357617,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.13",
			Note: "与丛林神庙/女巫小屋/雪屋共用同一套网格，候选区块相同"},
		{ID: "jungle_pyramid", Label: "丛林神庙", Spacing: 32, Separation: 8, Salt: 14357617,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.13",
			Note: "与沙漠神殿共用网格，实际位置由丛林生物群系决定"},
		{ID: "swamp_hut", Label: "女巫小屋", Spacing: 32, Separation: 8, Salt: 14357617,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.13",
			Note: "与沙漠神殿共用网格，实际位置由沼泽生物群系决定"},
		{ID: "igloo", Label: "雪屋", Spacing: 32, Separation: 8, Salt: 14357617,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.13",
			Note: "与沙漠神殿共用网格，实际位置由积雪生物群系决定"},
		{ID: "ocean_ruin", Label: "海底废墟", Spacing: 20, Separation: 8, Salt: 14357621,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.13",
			Note: "分暖水/冷水两种变体，变体由生物群系决定"},
		{ID: "shipwreck", Label: "沉船", Spacing: 24, Separation: 4, Salt: 165745295,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.13",
			Note: "含搁浅/正常/补给三种变体"},
		{ID: "monument", Label: "海底神殿", Spacing: 32, Separation: 5, Salt: 10387313,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.13",
			Note: "需要深海生物群系，省略群系过滤后实际位置可能偏移"},
		{ID: "mansion", Label: "林地府邸", Spacing: 80, Separation: 20, Salt: 10387319,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.13",
			Note: "间距最大（80 区块），因此数量最少、离出生点最远"},
		{ID: "pillager_outpost", Label: "掠夺者前哨站", Spacing: 32, Separation: 8, Salt: 165745296,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.14",
			Note: "1.14 加入；参数在 1.18/1.19 未变"},
		{ID: "ruined_portal", Label: "废弃传送门", Spacing: 40, Separation: 15, Salt: 34222645,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.16",
			Note: "1.16 加入；海底/山地变体由生物群系决定"},
		{ID: "ancient_city", Label: "远古城市", Spacing: 24, Separation: 8, Salt: 20083232,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.19",
			Note: "1.19 加入；只在深暗之域生成，位于 y=-51 附近"},
		{ID: "trail_ruins", Label: "古迹废墟", Spacing: 34, Separation: 8, Salt: 83469867,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.20",
			Note: "1.20 加入；只生成在 7 种「考古」生物群系里"},
		{ID: "trial_chambers", Label: "试炼密室", Spacing: 34, Separation: 12, Salt: 94251327,
			Dimension: yyStructDimOverworld, Kind: yyStructKindSpread, Since: "1.21",
			Note: "1.21 加入；地下结构，从 -40 到 40 层随机"},
		// ── 主世界：特殊放置 ──
		{ID: "stronghold", Label: "要塞", Spacing: 32, Separation: 3, Salt: 0,
			Dimension: yyStructDimOverworld, Kind: yyStructKindConcentric, Since: "1.9",
			Note: "三环同心（1408/4352/7424 方块），每环 128 候选角；省略生物群系过滤"},
		{ID: "mineshaft", Label: "废弃矿井", Spacing: 1, Separation: 0, Salt: 0,
			Dimension: yyStructDimOverworld, Kind: yyStructKindProbability, Since: "1.13",
			Probability: 0.004,
			Note: "每区块 0.4% 概率独立生成，没有区域网格，本工具给不出确定坐标"},
		{ID: "buried_treasure", Label: "埋藏的宝藏", Spacing: 1, Separation: 0, Salt: 0,
			Dimension: yyStructDimOverworld, Kind: yyStructKindProbability, Since: "1.13",
			Probability: 0.01,
			Note: "每区块 1% 概率生成（仅沙滩/海洋），没有区域网格，给不出确定坐标"},
		// ── 下界 ──
		{ID: "fortress", Label: "下界要塞", Spacing: 27, Separation: 4, Salt: 30084232,
			Dimension: yyStructDimNether, Kind: yyStructKindSpread, Since: "1.13",
			Note: "下界结构，坐标是下界坐标系（主世界坐标 ÷ 8）"},
		{ID: "bastion", Label: "堡垒遗迹", Spacing: 27, Separation: 4, Salt: 30084232,
			Dimension: yyStructDimNether, Kind: yyStructKindSpread, Since: "1.16",
			Note: "1.16 加入；与下界要塞共用区域参数，原版靠结构集区分，候选会重叠"},
		{ID: "nether_fossil", Label: "下界化石", Spacing: 2, Separation: 1, Salt: 14357921,
			Dimension: yyStructDimNether, Kind: yyStructKindSpread, Since: "1.16",
			Note: "1.16 加入；生成在灵魂沙峡谷，间距很小所以数量多"},
		// ── 末地 ──
		{ID: "end_city", Label: "末地城", Spacing: 20, Separation: 11, Salt: 10387313,
			Dimension: yyStructDimEnd, Kind: yyStructKindSpread, Since: "1.13",
			Note: "末地外岛结构，坐标是末地坐标系；只在离主岛 1000 格外生成"},
	}
}

// yySeedStructureSpecByID 按 ID 或别名查结构；找不到返回 false。
func yySeedStructureSpecByID(id string) (yySeedStructureSpec, bool) {
	key := yyStructNormalizeID(id)
	if real, ok := yySeedStructureAliases[key]; ok {
		key = real
	}
	for _, s := range yySeedStructureSpecs() {
		if s.ID == key {
			return s, true
		}
	}
	return yySeedStructureSpec{}, false
}

// yyStructNormalizeID 统一小写、去首尾空格、把内部连字符/空格换成下划线。
// 前端下拉、AI 工具参数、手打 URL 三种来源的写法都要能吃下。
func yyStructNormalizeID(id string) string {
	id = strings.TrimSpace(id)
	out := make([]rune, 0, len(id))
	for _, r := range id {
		switch r {
		case ' ', '-', '\t':
			out = append(out, '_')
		default:
			if r >= 'A' && r <= 'Z' {
				r = r - 'A' + 'a'
			}
			out = append(out, r)
		}
	}
	return strings.Trim(string(out), "_")
}

// yySeedStructureSpecsByDim 返回某维度下的全部结构。
// dim 为空 / "all" 时返回全部维度。
func yySeedStructureSpecsByDim(dim string) []yySeedStructureSpec {
	d := yyStructNormalizeID(dim)
	if d == "" || d == "all" {
		return yySeedStructureSpecs()
	}
	if d == "end" {
		d = yyStructDimEnd
	}
	if d == "the_nether" {
		d = yyStructDimNether
	}
	out := make([]yySeedStructureSpec, 0, len(yySeedStructureSpecs()))
	for _, s := range yySeedStructureSpecs() {
		if s.Dimension == d {
			out = append(out, s)
		}
	}
	return out
}

// yySeedStructureIDsByDim 返回某维度下所有结构的 ID（给报错提示用）。
func yySeedStructureIDsByDim(dim string) string {
	out := yySeedStructureSpecsByDim(dim)
	parts := make([]string, 0, len(out))
	for _, s := range out {
		parts = append(parts, s.ID)
	}
	return strings.Join(parts, "/")
}

// yySeedStructurePrecision 返回该结构的精度标签（项目铁律：不许冒充精确）。
func yySeedStructurePrecision(spec yySeedStructureSpec) string {
	switch spec.Kind {
	case yyStructKindConcentric:
		return "approximate"
	case yyStructKindProbability:
		return "unsupported"
	default:
		return "candidate"
	}
}

// yySeedStructureCatalog 生成给 manifest / 前端用的结构清单。
func yySeedStructureCatalog() []map[string]any {
	specs := yySeedStructureSpecs()
	out := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		m := map[string]any{
			"id": s.ID, "label": s.Label, "dimension": s.Dimension,
			"kind": s.Kind, "since": s.Since,
			"precision": yySeedStructurePrecision(s),
			"note":      s.Note,
		}
		if s.Kind == yyStructKindSpread {
			m["spacing"] = s.Spacing
			m["separation"] = s.Separation
			m["salt"] = s.Salt
		}
		if s.Kind == yyStructKindProbability {
			m["probability"] = s.Probability
		}
		out = append(out, m)
	}
	return out
}

// yySeedStructureScanSpec 统一扫描入口：按放置类型分派。
//
// random_spread → yySeedStructureScanEx（LOD 分级）
// concentric    → yySeedStructureScanConcentric（要塞三环点，过滤到范围内）
// probability   → 空结果 + 中文说明（给不出确定坐标，不许编）
func yySeedStructureScanSpec(seed int64, spec yySeedStructureSpec, fromX, fromZ, toX, toZ, lod, limit int, deadline time.Time) yySeedScanResult {
	switch spec.Kind {
	case yyStructKindConcentric:
		return yySeedStructureScanConcentric(seed, fromX, fromZ, toX, toZ)
	case yyStructKindProbability:
		return yySeedScanResult{
			Chunks: [][2]int{}, LOD: yySeedClampLOD(lod), Stride: 1,
			Truncated: true, Reason: spec.Note,
		}
	default:
		return yySeedStructureScanEx(seed, spec, fromX, fromZ, toX, toZ, lod, limit, deadline)
	}
}

// yySeedStructureScanConcentric 把要塞三环候选点过滤到给定区块矩形内。
func yySeedStructureScanConcentric(seed int64, fromX, fromZ, toX, toZ int) yySeedScanResult {
	if toX < fromX {
		fromX, toX = toX, fromX
	}
	if toZ < fromZ {
		fromZ, toZ = toZ, fromZ
	}
	res := yySeedScanResult{Chunks: [][2]int{}, LOD: yySeedLODNear, Stride: 1}
	for _, p := range yySeedStrongholds(seed, yySeedMaxStrongholds) {
		if p.ChunkX < fromX || p.ChunkX > toX || p.ChunkZ < fromZ || p.ChunkZ > toZ {
			continue
		}
		res.Chunks = append(res.Chunks, [2]int{p.ChunkX, p.ChunkZ})
	}
	sort.Slice(res.Chunks, func(i, j int) bool {
		if res.Chunks[i][0] != res.Chunks[j][0] {
			return res.Chunks[i][0] < res.Chunks[j][0]
		}
		return res.Chunks[i][1] < res.Chunks[j][1]
	})
	return res
}

// yySeedDimensionLabel 维度 ID → 中文名。
func yySeedDimensionLabel(dim string) string {
	switch yyStructNormalizeID(dim) {
	case yyStructDimNether, "the_nether":
		return "下界"
	case yyStructDimEnd, "end":
		return "末地"
	case "all":
		return "全部维度"
	default:
		return "主世界"
	}
}

// yySeedGameVersions 返回前端版本下拉可选的真实游戏版本（从新到旧）。
//
// 只列「算法上有意义」的版本：1.18 世界生成重写、1.19/1.20/1.21 各加了一批结构，
// 这些边界必须能选到；中间的补丁版本（1.20.1/1.20.4）算法与主版本一致，不必逐个列。
func yySeedGameVersions() []string {
	return []string{
		"1.21.4", "1.21", "1.20.6", "1.20", "1.19.4", "1.19", "1.18.2", "1.18",
		"1.17", "1.16.5", "1.16", "1.15", "1.14", "1.13",
		"1.12", "1.11", "1.10", "1.9", "1.8", "1.7",
	}
}

// yySeedVersionMatrix 版本覆盖矩阵：明确写出「哪一段能算、哪一段不能算」。
//
// 用户要求「所有覆盖的版本」，这里的做法是诚实分档：
// 能算的标出算法与精度，不能算的直接标 unsupported，不做假。
func yySeedVersionMatrix() []map[string]any {
	return []map[string]any{
		{
			"id": "1.7-1.12", "label": "1.7 ~ 1.12", "biome_algo": "legacy_temp_humidity",
			"structure_algo": "none", "biome_precision": "approximate", "structure_precision": "unsupported",
			"note": "群系用旧温湿度双噪声近似可算；1.13 之前结构放置参数与现在不同，本工具不覆盖",
		},
		{
			"id": "1.13-1.17", "label": "1.13 ~ 1.17", "biome_algo": "legacy_temp_humidity",
			"structure_algo": "random_spread", "biome_precision": "approximate", "structure_precision": "candidate",
			"note": "结构进入区域网格 + 盐值时代；1.16 新增废弃传送门/堡垒遗迹/下界化石",
		},
		{
			"id": "1.18-1.18.2", "label": "1.18 ~ 1.18.2", "biome_algo": "climate_noise",
			"structure_algo": "random_spread", "biome_precision": "approximate", "structure_precision": "candidate",
			"note": "1.18 重写世界生成，群系改为五气候参数噪声栈；结构参数未大改",
		},
		{
			"id": "1.19-1.19.4", "label": "1.19 ~ 1.19.4", "biome_algo": "climate_noise",
			"structure_algo": "random_spread", "biome_precision": "approximate", "structure_precision": "candidate",
			"note": "1.19 新增远古城市；结构放置改为数据包注册表，参数表沿用",
		},
		{
			"id": "1.20-1.20.6", "label": "1.20 ~ 1.20.6", "biome_algo": "climate_noise",
			"structure_algo": "random_spread", "biome_precision": "approximate", "structure_precision": "candidate",
			"note": "1.20 新增古迹废墟",
		},
		{
			"id": "1.21+", "label": "1.21 及以后", "biome_algo": "climate_noise",
			"structure_algo": "random_spread", "biome_precision": "approximate", "structure_precision": "candidate",
			"note": "1.21 新增试炼密室；后续版本参数若再调整，以 manifest 的 since 字段为准",
		},
		{
			"id": "bedrock", "label": "基岩版", "biome_algo": "none",
			"structure_algo": "none", "biome_precision": "unsupported", "structure_precision": "unsupported",
			"note": "基岩版 RNG 与 Java 不同源，全部算法未实现；结果只代表 Java 语义",
		},
	}
}

// yySeedDimensionCatalog 三个维度的说明（前端做维度切换时直接用）。
func yySeedDimensionCatalog() []map[string]any {
	return []map[string]any{
		{"id": yyStructDimOverworld, "label": "主世界", "biome_precision": "approximate",
			"note": "坐标即方块坐标；群系按气候噪声栈（1.18+）或温湿度双噪声（1.7-1.17）"},
		{"id": yyStructDimNether, "label": "下界", "biome_precision": "approximate",
			"note": "坐标是下界坐标系，与主世界按 1:8 换算；群系按 y=64 切片二维近似"},
		{"id": yyStructDimEnd, "label": "末地", "biome_precision": "exact",
			"note": "主岛与外围岛屿统一末地群系（精确）；末地城只在离主岛 1000 格外生成"},
	}
}
