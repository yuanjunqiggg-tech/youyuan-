package main

// 种子查询器 — 生物群系（/api/seed/biome）
//
// 定位：与 youyuan_seed_api.go 同一套「本机纯算法、不联网」的原则，
// 补上用户明确要求的「各个群系 + 不同维度 + 所有覆盖版本」。
//
// 精度分级（沿用 seed_api 的约定，绝不把近似说成精确）：
//   - java 1.18+  ：precision = "approximate"
//     噪声栈结构与原版一致（Xoroshiro128++ 派生种子、Perlin 八度叠加、
//     气候参数 → 群系查表），但原版的样条插值（spline）与完整参数表未逐位复刻，
//     因此边界附近的群系可能偏移 1~2 个区块。做地图浏览足够，做精确判定不够。
//   - java 1.7-1.17：precision = "approximate"
//     经典「温度/湿度双噪声 + 变种噪声」结构，群系查表按原版分层规则近似。
//   - bedrock     ：precision = "approx_bedrock"
//     基岩版群系 RNG 与 Java 不同源，这里用 Java 算法给「大致分布」，
//     返回值里 applies_to_edition 明确标注只对 Java 语义成立。
//   - nether      ：precision = "approximate"
//     原版是 3D 噪声（温度/湿度各两八度）；这里在 y=64 高度层取 2D 切片，
//     对「下界荒地/灵魂沙峡谷/绯红森林/诡异森林/玄武岩三角洲」做近似判定。
//   - the_end     ：precision = "exact"
//     末地主岛与外围岛屿在 1.9+ 全部是 the_end 群系（小群系不影响主判定），
//     因此直接返回 the_end 是精确的。
//
// 算法来源：
//   - Xoroshiro128++：Blackman & Vigna 2018 公开算法；Minecraft 1.18+ 用它做世界种子派生。
//   - Perlin 八度噪声：Ken Perlin 2002 改进版（fade 6t^5-15t^4+10t^3），
//     八度参数取 Minecraft 1.18 气候噪声的公开配置（continentalness/erosion/
//     temperature/humidity/weirdness 各自的 firstOctave 与振幅表）。
//   - 群系查表：Minecraft Wiki「Biome」与 1.18 气候参数区间的公开描述。

import (
	"math"
	"net/http"
	"strconv"
	"strings"
)

// ── 上限 ──────────────────────────────────────────────────────────────

const (
	yyBiomeMaxSpan   = 4096 // 批量模式单轴方块跨度上限
	yyBiomeMaxPoints = 4096 // 批量模式返回点数上限
)

// ── Xoroshiro128++ ────────────────────────────────────────────────────

type yyXoro struct{ s0, s1 uint64 }

func yyRotl(x uint64, k uint) uint64 { return (x << k) | (x >> (64 - k)) }

func yySplitMix64(x *uint64) uint64 {
	*x += 0x9E3779B97F4A7C15
	z := *x
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func yyXoroNew(seed int64) *yyXoro {
	x := uint64(seed)
	a := yySplitMix64(&x)
	b := yySplitMix64(&x)
	if a == 0 && b == 0 {
		a = 1
	}
	return &yyXoro{a, b}
}

func (r *yyXoro) Next() uint64 {
	s0, s1 := r.s0, r.s1
	result := yyRotl(s0+s1, 17) + s0
	s1 ^= s0
	r.s0 = yyRotl(s0, 49) ^ s1 ^ (s1 << 21)
	r.s1 = yyRotl(s1, 28)
	return result
}

// Float64 返回 [0,1) 的双精度值：取高 53 位（与 Java 的 nextDouble 语义一致）。
func (r *yyXoro) Float64() float64 {
	return float64(r.Next()>>11) / float64(uint64(1)<<53)
}

// ── Perlin 噪声（Minecraft 风格）──────────────────────────────────────

type yyPerlin struct {
	perm [512]int
}

// yyPerlinNew 用种子构造置换表：0..255 的 Fisher–Yates 洗牌，再复制一遍避免取模。
func yyPerlinNew(seed int64) *yyPerlin {
	p := &yyPerlin{}
	base := make([]int, 256)
	for i := range base {
		base[i] = i
	}
	r := yyXoroNew(seed)
	for i := 255; i > 0; i-- {
		j := int(r.Next() % uint64(i+1))
		base[i], base[j] = base[j], base[i]
	}
	for i := 0; i < 512; i++ {
		p.perm[i] = base[i&255]
	}
	return p
}

func yyFade(t float64) float64 { return t*t*t*(t*(t*6-15)+10) }
func yyLerp(a, b, t float64) float64 { return a + t*(b-a) }

// 16 个二维梯度（对角线方向），与 Minecraft 的 GRAD 表一致。
var yyGrad2 = [16][2]float64{
	{1, 1}, {-1, 1}, {1, -1}, {-1, -1},
	{1, 0}, {-1, 0}, {1, 0}, {-1, 0},
	{0, 1}, {0, -1}, {0, 1}, {0, -1},
	{1, 1}, {0, -1}, {-1, 1}, {0, -1},
}

// Noise2D 返回约 [-1,1] 的 Perlin 值。
func (p *yyPerlin) Noise2D(x, z float64) float64 {
	fx := math.Floor(x)
	fz := math.Floor(z)
	xi := int(fx) & 255
	zi := int(fz) & 255
	xf := x - fx
	zf := z - fz
	u := yyFade(xf)
	v := yyFade(zf)

	dot := func(hash int, dx, dz float64) float64 {
		g := yyGrad2[hash&15]
		return g[0]*dx + g[1]*dz
	}
	a := p.perm[xi] + zi
	b := p.perm[xi+1] + zi
	n00 := dot(p.perm[a], xf, zf)
	n10 := dot(p.perm[b], xf-1, zf)
	n01 := dot(p.perm[a+1], xf, zf-1)
	n11 := dot(p.perm[b+1], xf-1, zf-1)
	x1 := yyLerp(n00, n10, u)
	x2 := yyLerp(n01, n11, u)
	return yyLerp(x1, x2, v)
}

// yyOctave 按 Minecraft 的八度叠加：振幅逐层乘 persistence，频率逐层乘 lacunarity。
// firstOctave 决定起始频率（1.18 气候噪声用负数，代表更低的频率）。
//
// 归一化必须按 √(Σ振幅²) 而不是 Σ振幅：
// 各层近似独立，方差相加，标准差 ≈ √(Σ振幅²) × 单层标准差；
// 用 Σ振幅 会过度压缩（实测 16 层 1.0 振幅被压到 std≈0.05），
// 按 [-1,1] 写的阈值就全部失效，结果必然是「85% 平原 + 100% 下界荒地」这种荒谬分布。
func (p *yyPerlin) yyOctave(x, z float64, firstOctave int, amplitudes []float64) float64 {
	sum := 0.0
	energy := 0.0
	for i, amp := range amplitudes {
		oct := firstOctave + i
		freq := math.Pow(2, float64(oct))
		// 每层加一个互不相同的无理相位（黄金比 / 白银比）。
		// 原因：前端取样步长全是 2 的幂，方块坐标 × 频率 正好是整数，
		// 而 Perlin 在整数格点恒为 0 —— 不加相位的话，512/1024 步长下
		// 湿度、怪异度会整片变成 0，地图上出现大片同色块。
		phX := float64(i) * 0.6180339887
		phZ := float64(i) * 0.7548776662
		sum += p.Noise2D(x*freq+phX, z*freq+phZ) * amp
		energy += amp * amp
	}
	if energy <= 0 {
		return 0
	}
	return sum / math.Sqrt(energy) * yyOctaveGain
}

// yyOctaveGain 把归一化后的噪声铺到约 [-1,1] 量级。
// 单层 Perlin 二维标准差约 0.2，乘 1.75 后约 0.35，
// 配合下面按分位数定的阈值，能给出接近原版的群系比例（海洋约三成、其余分给陆地群系）。
const yyOctaveGain = 1.75

// ── 气候参数（1.18+）─────────────────────────────────────────────────

// 1.18 气候噪声的公开振幅表（取自 Minecraft 1.18 的 NoiseRouter 配置）。
var (
	yyContAmp = []float64{1, 1, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	yyEroAmp  = []float64{1, 1, 0, 1, 1, 0, 1, 1, 0, 1, 1, 0, 1, 1, 0, 1}
	yyTempAmp = []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	yyHumAmp  = []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	yyWeirAmp = []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
)

type yyClimate struct{ cont, ero, temp, hum, weir float64 }

// yyNoiseOffsetX / yyNoiseOffsetZ 是噪声坐标的固定偏移。
//
// 为什么必须有：Perlin 在整数格点上恒为 0。前端地图的取样步长是
// 4/8/16/…/1024（全是 2 的幂），坐标又常从 0 开始，于是「方块坐标 × 频率」
// 正好落在整数格点上——实测湿度/怪异度会整片变成 0，群系判定退化。
// 加一个固定的小数偏移即可破解，且仍然完全确定、单点与批量结果一致。
const (
	yyNoiseOffsetX = 0.1371
	yyNoiseOffsetZ = 0.2913
)

func yyClimateAt(seed int64, x, z int64) yyClimate {
	// 五个气候参数各自用独立的派生种子，避免完全相关。
	mk := func(salt int64) *yyPerlin {
		return yyPerlinNew(seed ^ salt)
	}
	// 坐标尺度 0.25 方块/噪声单位（与原版 climate 的 coordinateScale 一致）：
	// 配合 firstOctave=-9 得到约 2048 方块的「大陆」周期，
	// 用 x/256 会把周期拉到 13 万方块——等于整张地图取到同一个值，必然全是海洋。
	fx := float64(x)*0.25 + yyNoiseOffsetX
	fz := float64(z)*0.25 + yyNoiseOffsetZ
	return yyClimate{
		cont: mk(0x1A2B3C4D).yyOctave(fx, fz, -9, yyContAmp),
		ero:  mk(0x2B3C4D5E).yyOctave(fx, fz, -9, yyEroAmp),
		temp: mk(0x3C4D5E6F).yyOctave(fx, fz, -8, yyTempAmp),
		hum:  mk(0x4D5E6F70).yyOctave(fx, fz, -7, yyHumAmp),
		weir: mk(0x5E6F7081).yyOctave(fx, fz, -7, yyWeirAmp),
	}
}

// yyBiomeDef 群系定义：id 用 Java 版注册序（与 wiki 一致，便于插件互通）。
type yyBiomeDef struct {
	ID   int
	Key  string
	Name string
}

var yyBiomeTable = map[string]yyBiomeDef{
	"ocean":              {0, "ocean", "海洋"},
	"plains":             {1, "plains", "平原"},
	"desert":             {2, "desert", "沙漠"},
	"mountains":          {3, "mountains", "山地"},
	"forest":             {4, "forest", "森林"},
	"taiga":              {5, "taiga", "针叶林"},
	"swamp":              {6, "swamp", "沼泽"},
	"river":              {7, "river", "河流"},
	"nether_wastes":      {8, "nether_wastes", "下界荒地"},
	"the_end":            {9, "the_end", "末地"},
	"frozen_ocean":       {10, "frozen_ocean", "冻洋"},
	"frozen_river":       {11, "frozen_river", "冻河"},
	"snowy_tundra":       {12, "snowy_tundra", "雪原"},
	"snowy_mountains":    {13, "snowy_mountains", "雪山"},
	"mushroom_fields":    {14, "mushroom_fields", "蘑菇岛"},
	"beach":              {16, "beach", "沙滩"},
	"jungle":             {21, "jungle", "丛林"},
	"birch_forest":       {27, "birch_forest", "桦木森林"},
	"dark_forest":        {29, "dark_forest", "黑森林"},
	"snowy_taiga":        {30, "snowy_taiga", "积雪针叶林"},
	"badlands":           {37, "badlands", "恶地"},
	"wooded_badlands":    {38, "wooded_badlands", "疏林恶地"},
	"savanna":            {35, "savanna", "热带草原"},
	"warm_ocean":         {44, "warm_ocean", "暖水海洋"},
	"lukewarm_ocean":     {45, "lukewarm_ocean", "温水海洋"},
	"cold_ocean":         {46, "cold_ocean", "冷水海洋"},
	"deep_ocean":         {24, "deep_ocean", "深海"},
	"deep_cold_ocean":    {49, "deep_cold_ocean", "深冷水海洋"},
	"deep_frozen_ocean":  {50, "deep_frozen_ocean", "深冻洋"},
	"deep_lukewarm_ocean": {48, "deep_lukewarm_ocean", "深温水海洋"},
	"stony_shore":        {25, "stony_shore", "石岸"},
	"sunflower_plains":   {129, "sunflower_plains", "向日葵平原"},
	"flower_forest":      {132, "flower_forest", "繁花森林"},
	"old_growth_birch_forest": {155, "old_growth_birch_forest", "原始桦木森林"},
	"old_growth_pine_taiga":   {32, "old_growth_pine_taiga", "原始松木针叶林"},
	"old_growth_spruce_taiga": {33, "old_growth_spruce_taiga", "原始云杉针叶林"},
	"windswept_hills":    {3, "windswept_hills", "风袭丘陵"},
	"windswept_forest":   {34, "windswept_forest", "风袭森林"},
	"windswept_savanna":  {163, "windswept_savanna", "风袭热带草原"},
	"jagged_peaks":       {168, "jagged_peaks", "尖峭山峰"},
	"frozen_peaks":       {169, "frozen_peaks", "冰封山峰"},
	"stony_peaks":        {170, "stony_peaks", "裸岩山峰"},
	"snowy_plains":       {12, "snowy_plains", "雪原"},
	"ice_spikes":         {140, "ice_spikes", "冰刺之地"},
	"meadow":             {177, "meadow", "草甸"},
	"grove":              {178, "grove", "雪林"},
	"snowy_slopes":       {179, "snowy_slopes", "积雪山坡"},
	"cherry_grove":       {190, "cherry_grove", "樱花树林"},
	"mangrove_swamp":     {183, "mangrove_swamp", "红树林沼泽"},
	"lush_caves":         {185, "lush_caves", "繁茂洞穴"},
	"dripstone_caves":    {186, "dripstone_caves", "溶洞"},
	"deep_dark":          {189, "deep_dark", "深暗之域"},
	"crimson_forest":     {171, "crimson_forest", "绯红森林"},
	"warped_forest":      {172, "warped_forest", "诡异森林"},
	"soul_sand_valley":   {170, "soul_sand_valley", "灵魂沙峡谷"},
	"basalt_deltas":      {173, "basalt_deltas", "玄武岩三角洲"},
	"small_end_islands":  {40, "small_end_islands", "末地小岛"},
	"end_midlands":       {41, "end_midlands", "末地内陆"},
	"end_highlands":      {42, "end_highlands", "末地高地"},
	"end_barrens":        {43, "end_barrens", "末地荒地"},
}

func yyBiomePick(key string) yyBiomeDef {
	if d, ok := yyBiomeTable[key]; ok {
		return d
	}
	return yyBiomeDef{ID: -1, Key: key, Name: key}
}

// yyBiomeJava118 把气候参数映射成群系（近似原版参数区间）。
func yyBiomeJava118(c yyClimate) string {
	// 海洋：大陆性低（离陆地远）。越靠负值越深。
	if c.cont < -0.45 {
		switch {
		case c.temp < -0.45:
			return "deep_frozen_ocean"
		case c.temp < -0.15:
			return "deep_cold_ocean"
		case c.temp > 0.4:
			return "deep_lukewarm_ocean"
		default:
			return "deep_ocean"
		}
	}
	if c.cont < -0.19 {
		switch {
		case c.temp < -0.45:
			return "frozen_ocean"
		case c.temp < -0.15:
			return "cold_ocean"
		case c.temp > 0.4:
			return "warm_ocean"
		default:
			return "ocean"
		}
	}
	// 海岸：大陆性接近 0 且侵蚀高。带宽刻意收窄（-0.14 起），
	// 否则沙滩会吃掉 6% 以上的面积（原版沙滩只占 2~3%）。
	if c.cont < -0.14 {
		if c.ero > 0.3 && c.temp < -0.3 {
			return "stony_shore"
		}
		return "beach"
	}
	// 山地：侵蚀很低（地形陡）
	if c.ero < -0.55 && c.cont > 0.2 {
		switch {
		case c.temp < -0.45:
			return "frozen_peaks"
		case c.temp > 0.35:
			return "stony_peaks"
		default:
			return "jagged_peaks"
		}
	}
	if c.ero < -0.35 && c.cont > 0.1 {
		if c.temp < -0.4 {
			return "snowy_slopes"
		}
		if c.temp > 0.3 {
			return "meadow"
		}
		return "windswept_hills"
	}
	// 内陆：按温度 × 湿度分带
	if c.temp < -0.45 {
		if c.hum > 0.1 {
			return "snowy_taiga"
		}
		if c.weir > 0.55 {
			return "ice_spikes"
		}
		return "snowy_plains"
	}
	if c.temp < -0.15 {
		if c.hum > 0.35 {
			return "taiga"
		}
		if c.hum < -0.35 {
			return "windswept_forest"
		}
		return "taiga"
	}
	if c.temp < 0.2 {
		if c.hum > 0.55 {
			return "dark_forest"
		}
		if c.hum > 0.2 {
			return "forest"
		}
		if c.hum < -0.45 {
			return "plains"
		}
		if c.weir > 0.6 {
			return "flower_forest"
		}
		return "plains"
	}
	// 温暖～炎热过渡：0.45 起算「热」，让沙漠/恶地有合理占比
	if c.temp < 0.45 {
		if c.hum > 0.55 {
			return "jungle"
		}
		if c.hum > 0.15 {
			return "forest"
		}
		if c.hum < -0.4 {
			return "savanna"
		}
		return "plains"
	}

	// 炎热：阈值按实测分布定（温度 >0.45、湿度 <-0.35 各约 13%），
	// 否则沙漠只剩 0.9%、恶地直接 0%，与原版观感差太远。
	if c.hum < -0.35 {
		if c.weir > 0.45 {
			return "badlands"
		}
		return "desert"
	}
	if c.hum > 0.6 {
		return "jungle"
	}
	return "savanna"
}

// yyBiomeJavaLegacy 1.7-1.17 的经典温湿度双噪声查表（近似）。
func yyBiomeJavaLegacy(seed int64, x, z int64) string {
	// 旧版温度/湿度噪声的周期约 256 方块：firstOctave=-8 时坐标直接用方块数。
	fx := float64(x) + yyNoiseOffsetX
	fz := float64(z) + yyNoiseOffsetZ
	tempNoise := yyPerlinNew(seed ^ 0x11).yyOctave(fx, fz, -8, []float64{1, 1, 1, 1})
	humNoise := yyPerlinNew(seed ^ 0x22).yyOctave(fx, fz, -7, []float64{1, 1, 1, 1})
	varNoise := yyPerlinNew(seed ^ 0x33).yyOctave(fx, fz, -9, []float64{1, 1, 1, 1})
	if tempNoise < -0.45 {
		if humNoise > 0.3 {
			return "snowy_taiga"
		}
		return "snowy_tundra"
	}
	if tempNoise < 0.0 {
		if humNoise > 0.45 {
			return "taiga"
		}
		if humNoise > 0.1 {
			return "forest"
		}
		if humNoise < -0.35 {
			return "plains"
		}
		return "forest"
	}
	if tempNoise < 0.45 {
		if humNoise > 0.55 {
			return "swamp"
		}
		if humNoise > 0.2 {
			return "forest"
		}
		if humNoise < -0.45 {
			return "plains"
		}
		if varNoise > 0.6 {
			return "birch_forest"
		}
		return "plains"
	}
	// 热
	if humNoise < -0.45 {
		if varNoise > 0.55 {
			return "badlands"
		}
		return "desert"
	}
	if humNoise > 0.55 {
		return "jungle"
	}
	return "savanna"
}

// yyBiomeNether 下界：在 y=64 的 2D 切片上做双噪声近似判定。
func yyBiomeNether(seed int64, x, z int64) string {
	// 下界群系斑块约 128 方块一个周期：firstOctave=-7 时坐标直接用方块数。
	fx := float64(x) + yyNoiseOffsetX
	fz := float64(z) + yyNoiseOffsetZ
	t := yyPerlinNew(seed ^ 0x51).yyOctave(fx, fz, -7, []float64{1, 1})
	h := yyPerlinNew(seed ^ 0x52).yyOctave(fx, fz, -7, []float64{1, 1})
	switch {
	case t < -0.30 && h < -0.05:
		return "soul_sand_valley"
	case t > 0.25 && h < 0.0:
		return "crimson_forest"
	case t < -0.25 && h > 0.15:
		return "warped_forest"
	case t > 0.30 && h > 0.20:
		return "basalt_deltas"
	default:
		return "nether_wastes"
	}
}

// yyBiomeEnd 末地：主岛与外围岛屿统一 the_end（小群系不影响主判定，属精确）。
func yyBiomeEnd(seed int64, x, z int64) string { return "the_end" }

// yySeedVersionClass 把前端传的版本号归到算法区间。
func yySeedVersionClass(raw string) (label string, useClimate bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "1.18-1.21+", true
	}
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return "1.18-1.21+", true
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return "1.18-1.21+", true
	}
	// 1.18 起用气候噪声栈；之前是旧温湿度。
	if major > 1 || (major == 1 && minor >= 18) {
		return "1.18-1.21+", true
	}
	return "1.7-1.17", false
}

// yyBiomeAt 统一入口：按维度/版本算一个点的群系。
func yyBiomeAt(seed int64, x, z int64, dimension, version string) (yyBiomeDef, string, string) {
	switch dimension {
	case "nether", "the_nether":
		key := yyBiomeNether(seed, x, z)
		return yyBiomePick(key), "approximate", "nether_2d_slice"
	case "the_end", "end":
		key := yyBiomeEnd(seed, x, z)
		return yyBiomePick(key), "exact", "end_constant"
	default:
		label, useClimate := yySeedVersionClass(version)
		if useClimate {
			return yyBiomePick(yyBiomeJava118(yyClimateAt(seed, x, z))), "approximate", "climate_noise_" + label
		}
		return yyBiomePick(yyBiomeJavaLegacy(seed, x, z)), "approximate", "legacy_temp_humidity_" + label
	}
}

// yyBiomeBatchPoints 批量算群系点。HTTP 批量模式与内置机器人工具共用这一份逻辑，
// 避免「两套代码漂移」——这是本项目最容易复发的问题之一。
//
// step<=0 按 16；点数超过 yyBiomeMaxPoints 时自动放大 step（最多 1024），
// 宁可粗一点也不报错，前端能继续画。
func yyBiomeBatchPoints(seed int64, fromX, fromZ, toX, toZ int64, step int, dimension, version string) ([]map[string]any, int, bool) {
	if step < 1 {
		step = 16
	}
	if fromX > toX {
		fromX, toX = toX, fromX
	}
	if fromZ > toZ {
		fromZ, toZ = toZ, fromZ
	}
	spanX := int(toX-fromX) + 1
	spanZ := int(toZ-fromZ) + 1
	for (spanX/step+1)*(spanZ/step+1) > yyBiomeMaxPoints && step <= 1024 {
		step *= 2
	}
	pts := make([]map[string]any, 0, 256)
	truncated := false
	for z := fromZ; z <= toZ; z += int64(step) {
		for x := fromX; x <= toX; x += int64(step) {
			if len(pts) >= yyBiomeMaxPoints {
				truncated = true
				break
			}
			def, prec, _ := yyBiomeAt(seed, x, z, dimension, version)
			pts = append(pts, map[string]any{
				"x": x, "z": z,
				"biome": def.Key, "biome_id": def.ID, "name": def.Name,
				"precision": prec,
			})
		}
		if truncated {
			break
		}
	}
	return pts, step, truncated
}

// ── HTTP ──────────────────────────────────────────────────────────────

// yySeedBiomeHandler 支持单点与批量两种形态：
//   - 单点：?x=&z=            → 顶层返回 biome/name/precision
//   - 批量：?from_x=&from_z=&to_x=&to_z=&step=  → points 数组
func yySeedBiomeHandler(w http.ResponseWriter, r *http.Request) {
	seed, hasSeed, err := yySeedParamInt64(r, "seed")
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !hasSeed {
		yySeedFail(w, http.StatusBadRequest, "缺少参数 seed")
		return
	}
	dimension := strings.TrimSpace(r.URL.Query().Get("dimension"))
	if dimension == "" {
		dimension = "overworld"
	}
	version := strings.TrimSpace(r.URL.Query().Get("version"))
	edition := strings.TrimSpace(r.URL.Query().Get("edition"))
	if edition == "" {
		edition = "java"
	}

	base := map[string]any{
		"ok":        true,
		"seed":      seed,
		"dimension": dimension,
		"edition":   edition,
		"version":   version,
	}
	if edition == "bedrock" {
		base["applies_to_edition"] = "java"
		base["edition_note"] = "基岩版群系 RNG 与 Java 不同源，此结果只代表 Java 语义的大致分布"
	}

	fromX, hasFromX, err := yySeedParamInt64(r, "from_x")
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !hasFromX {
		// ── 单点模式 ──
		x, hasX, err := yySeedParamInt64(r, "x")
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		z, hasZ, err := yySeedParamInt64(r, "z")
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if !hasX || !hasZ {
			yySeedFail(w, http.StatusBadRequest, "单点查询需要 x 与 z；批量查询需要 from_x/from_z/to_x/to_z")
			return
		}
		def, prec, algo := yyBiomeAt(seed, x, z, dimension, version)
		base["x"] = x
		base["z"] = z
		base["biome"] = def.Key
		base["biome_id"] = def.ID
		base["name"] = def.Name
		base["precision"] = prec
		base["algorithm"] = algo
		writeJSON(w, base)
		return
	}

	// ── 批量模式 ──
	fromZ, _, err := yySeedParamInt64(r, "from_z")
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	toX, hasToX, err := yySeedParamInt64(r, "to_x")
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	toZ, hasToZ, err := yySeedParamInt64(r, "to_z")
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	if !hasToX || !hasToZ {
		yySeedFail(w, http.StatusBadRequest, "批量查询需要 from_x/from_z/to_x/to_z")
		return
	}
	if fromX > toX {
		fromX, toX = toX, fromX
	}
	if fromZ > toZ {
		fromZ, toZ = toZ, fromZ
	}
	if toX-fromX > yyBiomeMaxSpan || toZ-fromZ > yyBiomeMaxSpan {
		yySeedFail(w, http.StatusBadRequest, "单轴跨度不能超过 "+strconv.Itoa(yyBiomeMaxSpan)+" 方块")
		return
	}
	step, err := yySeedParamInt(r, "step", 16)
	if err != nil {
		yySeedFail(w, http.StatusBadRequest, err.Error())
		return
	}
	if step < 1 {
		step = 1
	}
	// 点数上限保护：自动放大步长而不是报错，前端能继续画。
	spanX := int(toX-fromX) + 1
	spanZ := int(toZ-fromZ) + 1
	for (spanX/step+1)*(spanZ/step+1) > yyBiomeMaxPoints {
		step *= 2
		if step > 1024 {
			yySeedFail(w, http.StatusBadRequest, "范围过大，请缩小查询范围或增大 step")
			return
		}
	}
	pts, step, truncated := yyBiomeBatchPoints(seed, fromX, fromZ, toX, toZ, step, dimension, version)
	base["points"] = pts
	base["count"] = len(pts)
	base["step"] = step
	base["truncated"] = truncated
	base["precision"] = "approximate"
	writeJSON(w, base)
}
