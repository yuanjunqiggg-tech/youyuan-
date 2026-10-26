package main

// 种子查询器（/api/seed/*）
//
// 设计原则（重要）：
//  1. 全部结果由本机纯算法计算，不联网、不读文件、不写文件，无包级可变状态（并发安全）。
//  2. 每个能力都标注「精度分级」，不把近似值说成精确值：
//     - exact       ：算法与 Java 原版一致，可复现（史莱姆区块、坐标分享码）
//     - approximate ：算法骨架一致，但省略了原版的生物群系过滤等步骤（要塞）
//     - candidate   ：只给出「区域网格候选区块」，实际是否生成需进游戏实测（结构）
//  3. 所有循环都有 count / radius / 范围 / 时间预算四重上限，任何输入都不会让单次请求失控。
//  4. 大范围查询走 LOD（分级细节）：lod=0 远景只抽样少量点、lod=2 近景才逐个精确算，
//     工作量（区域数）超过 60000 时强制降级为远景抽样，并在返回里显式标注 sampled / degraded /
//     skipped_by_lod，让前端知道「这里还有 M 个没画」，而不是把抽样结果当成完整结果展示。
//
// 算法来源（注释中逐条标注）：
//   - Java 版 java.util.Random 的 48 位 LCG（乘数 0x5DEECE66D、增量 0xB），公开且可验证。
//   - 史莱姆区块：Minecraft Wiki「Slime chunk」条目的 Java 版伪代码
//     （seed + x*x*0x4c1906 + x*0x5ac0db + z*z*0x4307a7 + z*0x5f24f ^ 0x3ad8025f，再 nextInt(10)==0）。
//     注意：网上流传的 `x*x*0x1f1f1f1f + z*z*0x1f1f1f1f + seed*0x9e3779b9` 简化式与 Java 原版
//     不一致（属于第三方工具/其他版本的近似），因此本文件不采用，避免给出错误结论。
//   - 要塞：Minecraft 1.9+ 三环规则（半径 1408 / 4352 / 7424 方块），每环 128 个候选角。
//     原版还会用生物群系判定逐角试探，这里只按角度候选取点，故标注 approximate。
//   - 结构：Minecraft Wiki「Structure」条目的 Java 1.13+ 区域网格（spacing / separation / salt）表，
//     区域随机数种子 = seed + regionX*341873128712 + regionZ*132897987541 + salt（见下方函数注释）。
//     只给候选区块，不判定生物群系与版本，故标注 candidate。

import (
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ── 上限常量 ──────────────────────────────────────────────────────────

const (
	yySeedMaxSlimeRadius   = 64    // 史莱姆查询半径（区块）上限：129*129 ≈ 1.66 万次判定
	yySeedMaxChunkRange    = 4096  // 结构查询单轴区块跨度上限
	yySeedMaxResults       = 200   // 单次返回结构候选上限
	yySeedMaxStructureRegs = 60000 // 单次结构扫描的区域数上限（防 spacing 很小的结构炸开）
	yySeedMaxStrongholds   = 16    // 要塞数量上限
	yySeedMaxQueryRadius   = 32    // 聚合查询半径（区块）上限
	yySeedQueryBudget      = 150 * time.Millisecond
	yySeedMaxRawQuery      = 2048 // URL query 长度上限
)

// ── LOD（分级细节）常量 ─────────────────────────────────────────────
//
// 思路照搬 Chunkbase / 种子地图：缩小看全局时只画少量粗粒度标记（抽样，不逐个算），
// 放大才逐个精确计算。三档语义：
//
//	lod=0 远景/全局（默认）：每类型最多 40 个点，按区域网格抽样（跳区域，不逐个算）
//	lod=1 中景            ：每类型最多 120 个点，正常区域网格
//	lod=2 近景/放大        ：每类型最多 200 个点，逐个区域精确算（与旧行为一致）
const (
	yySeedLODFar  = 0
	yySeedLODMid  = 1
	yySeedLODNear = 2

	yySeedLODLimitFar  = 40
	yySeedLODLimitMid  = 120
	yySeedLODLimitNear = yySeedMaxResults // 200，近景上限与旧版单次返回上限一致

	// 远景每个轴最多采样多少个区域：7*7=49 个区域，配合 limit=40 保证结果不超过 40
	yySeedLODFarAxisSamples = 7

	// 热点图格子数：lod=0 → 32×32、lod=1 → 64×64、lod=2 → 128×128
	yySeedHeatGridFar  = 32
	yySeedHeatGridMid  = 64
	yySeedHeatGridNear = 128
)

// ── Java 版 java.util.Random（48 位 LCG）─────────────────────────────
//
// 来源：OpenJDK java.util.Random 的公开实现：
//
//	seed = (seed ^ 0x5DEECE66D) & ((1<<48)-1)
//	next(bits): seed = (seed*0x5DEECE66D + 0xB) & ((1<<48)-1); return seed >> (48-bits)
//
// 这里只实现种子查询需要的 nextInt(bound) 与 nextDouble()。

type yySeedJavaRandom struct {
	state int64
}

func yySeedNewJavaRandom(seed int64) *yySeedJavaRandom {
	return &yySeedJavaRandom{state: (seed ^ 0x5DEECE66D) & ((1 << 48) - 1)}
}

func (r *yySeedJavaRandom) next(bits uint) int32 {
	r.state = (r.state*0x5DEECE66D + 0xB) & ((1 << 48) - 1)
	return int32(uint64(r.state) >> (48 - bits))
}

// nextInt 与 Java 语义一致：2 的幂走乘法快速路径，否则拒绝采样消除取模偏差。
func (r *yySeedJavaRandom) nextInt(bound int32) int32 {
	if bound <= 0 {
		return 0
	}
	if bound&(-bound) == bound {
		return int32((int64(bound) * int64(r.next(31))) >> 31)
	}
	for {
		bits := r.next(31)
		val := bits % bound
		// Java: bits - val + (bound-1) < 0 表示溢出，重新采样
		if bits-val+(bound-1) >= 0 {
			return val
		}
	}
}

func (r *yySeedJavaRandom) nextDouble() float64 {
	hi := int64(r.next(26))
	lo := int64(r.next(27))
	return float64((hi<<27)+lo) / float64(int64(1)<<53)
}

// ── 史莱姆区块（Java 版，exact）─────────────────────────────────────

// yySeedIsSlimeChunk 判定 Java 版某个区块是否为史莱姆区块。
// 来源：Minecraft Wiki「Slime chunk」的 Java 版伪代码：
//
//	Random rnd = new Random(seed + x*x*0x4c1906 + x*0x5ac0db + z*z*0x4307a7 + z*0x5f24f ^ 0x3ad8025f);
//	return rnd.nextInt(10) == 0;
//
// 注意 Java 中 ^ 的优先级低于 +，所以是「先求和再异或盐值」。
func yySeedIsSlimeChunk(seed int64, cx, cz int32) bool {
	x := int64(cx)
	z := int64(cz)
	sum := seed + x*x*0x4c1906 + x*0x5ac0db + z*z*0x4307a7 + z*0x5f24f
	return yySeedNewJavaRandom(sum^0x3ad8025f).nextInt(10) == 0
}

type yySeedSlimeResult struct {
	Seed    int64    `json:"seed"`
	Edition string   `json:"edition"`
	Radius  int      `json:"radius"`
	Chunks  [][2]int `json:"chunks"`
	Blocks  [][2]int `json:"blocks"`
	Count   int      `json:"count"`
	Note    string   `json:"note,omitempty"`
}

// yySeedSlime 扫描 (cx,cz) 周围 radius 区块内的史莱姆区块（按 x 再 z 排序，结果稳定）。
func yySeedSlime(seed int64, cx, cz int32, radius int) yySeedSlimeResult {
	radius = yySeedClamp(radius, 0, yySeedMaxSlimeRadius)
	out := yySeedSlimeResult{Seed: seed, Edition: "java", Radius: radius, Chunks: [][2]int{}, Blocks: [][2]int{}}
	for dx := -radius; dx <= radius; dx++ {
		for dz := -radius; dz <= radius; dz++ {
			x := int64(cx) + int64(dx)
			z := int64(cz) + int64(dz)
			// 区块坐标超出 int32 安全范围直接跳过（原版世界边界远小于此）
			if x < math.MinInt32 || x > math.MaxInt32 || z < math.MinInt32 || z > math.MaxInt32 {
				continue
			}
			if !yySeedIsSlimeChunk(seed, int32(x), int32(z)) {
				continue
			}
			out.Chunks = append(out.Chunks, [2]int{int(x), int(z)})
			out.Blocks = append(out.Blocks, [2]int{int(x) * 16, int(z) * 16})
		}
	}
	out.Count = len(out.Chunks)
	return out
}

// ── 要塞（Java 版三环，approximate）────────────────────────────────

type yySeedStronghold struct {
	X            int     `json:"x"`
	Z            int     `json:"z"`
	Ring         int     `json:"ring"`
	ChunkX       int     `json:"chunk_x"`
	ChunkZ       int     `json:"chunk_z"`
	RadiusBlocks int     `json:"radius_blocks"`
	Angle        float64 `json:"angle_deg"`
}

// yySeedStrongholds 按 Java 1.9+ 三环规则生成要塞候选点。
//
// 规则来源（社区整理，与 Minecraft Wiki「Stronghold」一致）：
//   - 3 个环，环半径（方块）1408 / 4352 / 7424，每环 128 个候选角；
//   - 角度基准由 Random(seed).nextDouble() 决定，各候选角等分 360°；
//   - 原版从候选角里挑「生物群系合法且离其他要塞足够远」的第一个点。
//
// 本实现省掉了生物群系判定（本地无区块生成器），因此标注 approximate，
// 结果应理解为「原版算法的候选角位置」，进游戏可能偏移。
func yySeedStrongholds(seed int64, count int) []yySeedStronghold {
	count = yySeedClamp(count, 1, yySeedMaxStrongholds)
	rnd := yySeedNewJavaRandom(seed)
	base := rnd.nextDouble() * 2 * math.Pi
	out := make([]yySeedStronghold, 0, count)
	for i := 0; i < count; i++ {
		ring := i%3 + 1
		// 环半径（方块）：1408 / 4352 / 7424，即 88 / 272 / 464 区块。三环并非等差，用表更准确。
		radius := []float64{1408, 4352, 7424}[ring-1]
		start := int(rnd.nextInt(128))
		chosen := yySeedStronghold{}
		for attempt := 0; attempt < 128; attempt++ {
			angle := base + 2*math.Pi*float64((start+attempt)%128)/128
			x := int(math.Round(radius * math.Cos(angle)))
			z := int(math.Round(radius * math.Sin(angle)))
			cand := yySeedStronghold{
				X: x, Z: z, Ring: ring,
				ChunkX: yySeedFloorDiv(x, 16), ChunkZ: yySeedFloorDiv(z, 16),
				RadiusBlocks: int(radius),
				Angle:        math.Round(math.Mod(angle*180/math.Pi+360, 360)*10) / 10,
			}
			if yySeedStrongholdFarEnough(out, cand, 512) {
				chosen = cand
				break
			}
			if attempt == 127 {
				chosen = cand
			}
		}
		out = append(out, chosen)
	}
	return out
}

func yySeedStrongholdFarEnough(list []yySeedStronghold, cand yySeedStronghold, minDist float64) bool {
	for _, p := range list {
		dx := float64(p.X - cand.X)
		dz := float64(p.Z - cand.Z)
		if math.Hypot(dx, dz) < minDist {
			return false
		}
	}
	return true
}

// ── 结构候选（Java 1.13+ 区域网格，candidate）───────────────────────

// 结构注册表（yySeedStructureSpec / yySeedStructureSpecs / yySeedStructureSpecByID）
// 已迁到 youyuan_seed_structures.go：那里覆盖了三个维度的全部结构，
// 并按 random_spread / concentric / probability 三种放置类型分派扫描。


// yySeedStructureCandidate 计算某个区域（region）内的候选区块。
// 来源：Java 1.13+ 结构放置算法（Minecraft Wiki「Structure」）：
//
//	regionX = floor(chunkX / spacing), regionZ = floor(chunkZ / spacing)
//	rnd = Random(seed + regionX*341873128712 + regionZ*132897987541 + salt)
//	chunkX = regionX*spacing + rnd.nextInt(spacing - separation)
//	chunkZ = regionZ*spacing + rnd.nextInt(spacing - separation)
func yySeedStructureCandidate(seed int64, spec yySeedStructureSpec, regionX, regionZ int64) (int64, int64) {
	mix := seed + regionX*341873128712 + regionZ*132897987541 + spec.Salt
	rnd := yySeedNewJavaRandom(mix)
	span := int32(spec.Spacing - spec.Separation)
	if span <= 0 {
		span = 1
	}
	x := regionX*int64(spec.Spacing) + int64(rnd.nextInt(span))
	z := regionZ*int64(spec.Spacing) + int64(rnd.nextInt(span))
	return x, z
}

// yySeedStructureScan 扫描区块矩形范围内的候选结构，最多返回 limit 条（旧行为 = lod=2 精确扫描）。
// truncated 为 true 表示命中结果上限 / 区域上限 / 时间预算。
func yySeedStructureScan(seed int64, spec yySeedStructureSpec, fromX, fromZ, toX, toZ, limit int, deadline time.Time) (chunks [][2]int, truncated bool) {
	res := yySeedStructureScanEx(seed, spec, fromX, fromZ, toX, toZ, yySeedLODNear, limit, deadline)
	return res.Chunks, res.Truncated
}

// ── LOD（分级细节）──────────────────────────────────────────────────

// yySeedScanResult 是一次结构扫描的完整结果（含 LOD 元信息）。
type yySeedScanResult struct {
	Chunks       [][2]int // 候选区块（已按 x、z 排序）
	Truncated    bool     // 结果不完整：命中上限 / 区域上限 / 时间预算 / 被抽样跳过
	Sampled      bool     // 是否按区域网格抽样（true 表示有区域被整块跳过）
	SkippedByLOD int      // 因 LOD 未返回的候选点数量（估算值，见 yySeedEstimateTotal）
	LOD          int      // 实际生效的 LOD 档位
	Degraded     bool     // 是否因工作量超限被强制降级
	Reason       string   // 降级原因（中文，给前端直接展示）
	Regions      int64    // 请求范围内覆盖的区域总数（工作量）
	Stride       int      // 区域采样步长（1 表示不抽样）
}

// yySeedClampLOD 把任意 lod 夹到 0~2，越界按最近档处理（不报错，避免前端传错就白屏）。
func yySeedClampLOD(v int) int { return yySeedClamp(v, yySeedLODFar, yySeedLODNear) }

// yySeedLODLimit 返回某档 LOD 的单类型结果上限。
func yySeedLODLimit(lod int) int {
	switch yySeedClampLOD(lod) {
	case yySeedLODFar:
		return yySeedLODLimitFar
	case yySeedLODMid:
		return yySeedLODLimitMid
	default:
		return yySeedLODLimitNear
	}
}

// yySeedHeatGrid 返回某档 LOD 的热点图格子数（每轴）。
func yySeedHeatGrid(lod int) int {
	switch yySeedClampLOD(lod) {
	case yySeedLODFar:
		return yySeedHeatGridFar
	case yySeedLODMid:
		return yySeedHeatGridMid
	default:
		return yySeedHeatGridNear
	}
}

// yySeedRegionCount 计算区块矩形覆盖的区域数（工作量）：
//
//	区域数 = (to_x-from_x)/spacing * (to_z-from_z)/spacing
//
// 这里用 floor 后的区域索引跨度 +1 精确计算，避免除法取整少算一格。
func yySeedRegionCount(fromX, fromZ, toX, toZ, spacing int) int64 {
	if spacing <= 0 {
		spacing = 1
	}
	if toX < fromX {
		fromX, toX = toX, fromX
	}
	if toZ < fromZ {
		fromZ, toZ = toZ, fromZ
	}
	rx := int64(yySeedFloorDiv(toX, spacing)-yySeedFloorDiv(fromX, spacing)) + 1
	rz := int64(yySeedFloorDiv(toZ, spacing)-yySeedFloorDiv(fromZ, spacing)) + 1
	if rx <= 0 || rz <= 0 {
		return 0
	}
	return rx * rz
}

// yySeedLODStride 为远景抽样计算区域步长：把每轴区域数压到 yySeedLODFarAxisSamples 以内。
// 例如 4096 区块跨度、spacing=34（121 个区域/轴）→ 步长 18，实际只算 7×7 个区域。
func yySeedLODStride(fromX, fromZ, toX, toZ, spacing int) int {
	if spacing <= 0 {
		spacing = 1
	}
	if toX < fromX {
		fromX, toX = toX, fromX
	}
	if toZ < fromZ {
		fromZ, toZ = toZ, fromZ
	}
	rx := int64(yySeedFloorDiv(toX, spacing)-yySeedFloorDiv(fromX, spacing)) + 1
	rz := int64(yySeedFloorDiv(toZ, spacing)-yySeedFloorDiv(fromZ, spacing)) + 1
	axis := rx
	if rz > axis {
		axis = rz
	}
	if axis <= int64(yySeedLODFarAxisSamples) {
		return 1
	}
	stride := int((axis + int64(yySeedLODFarAxisSamples) - 1) / int64(yySeedLODFarAxisSamples))
	if stride < 1 {
		stride = 1
	}
	return stride
}

// yySeedEstimateTotal 估算查询范围内的候选点总数，用于算 skipped_by_lod。
// 依据：每个区域最多产出 1 个候选，候选落在范围内的概率约为 ((spacing-separation)/spacing)^2。
// 这是估算值，只用于告诉前端「还有 M 个没画」，不作为坐标依据。
func yySeedEstimateTotal(regions int64, spec yySeedStructureSpec) int64 {
	if regions <= 0 {
		return 0
	}
	span := spec.Spacing - spec.Separation
	if span <= 0 || spec.Spacing <= 0 {
		span = 1
	}
	p := float64(span) / float64(spec.Spacing)
	return int64(math.Round(float64(regions) * p * p))
}

// yySeedStructureScanEx 按 LOD 档位扫描结构候选。
//
// LOD 规则：
//   - lod=0：区域网格抽样（stride>1 时整块跳过区域），上限 40；
//   - lod=1：正常网格逐个算，上限 120；
//   - lod=2：逐个区域精确算，上限 200（旧行为）。
//
// 硬性性能预算：
//   - 区域数 > 60000 时强制降到 lod=0 并抽样，返回 Degraded=true 与中文 Reason；
//   - 扫描循环仍受 yySeedMaxStructureRegs 与 deadline 双重保护。
//
// limit<=0 表示用该档默认上限；显式 limit 只会被夹到「不超过该档上限」，不会被放大。
func yySeedStructureScanEx(seed int64, spec yySeedStructureSpec, fromX, fromZ, toX, toZ, lod, limit int, deadline time.Time) yySeedScanResult {
	if toX < fromX {
		fromX, toX = toX, fromX
	}
	if toZ < fromZ {
		fromZ, toZ = toZ, fromZ
	}
	res := yySeedScanResult{Chunks: [][2]int{}, LOD: yySeedClampLOD(lod), Stride: 1}
	res.Regions = yySeedRegionCount(fromX, fromZ, toX, toZ, spec.Spacing)

	// 工作量上限：区域数超过 60000 时，无论前端要哪一档都只能按远景抽样，否则单次请求会打满 CPU。
	if res.Regions > yySeedMaxStructureRegs {
		res.LOD = yySeedLODFar
		res.Degraded = true
		res.Reason = "范围太大，已按远景抽样"
	}
	if res.LOD == yySeedLODFar {
		res.Stride = yySeedLODStride(fromX, fromZ, toX, toZ, spec.Spacing)
		res.Sampled = res.Stride > 1
		if res.Sampled {
			// 抽样意味着结果不完整：同步置 truncated，老前端只认这个字段也能提示「还有更多」
			res.Truncated = true
		}
	}

	capLimit := yySeedLODLimit(res.LOD)
	if limit <= 0 || limit > capLimit {
		limit = capLimit
	}
	if limit < 1 {
		limit = 1
	}

	rx0, rx1 := yySeedFloorDiv(fromX, spec.Spacing), yySeedFloorDiv(toX, spec.Spacing)
	rz0, rz1 := yySeedFloorDiv(fromZ, spec.Spacing), yySeedFloorDiv(toZ, spec.Spacing)
	scanned := int64(0)
	stop := false
	for rz := rz0; rz <= rz1; rz += res.Stride {
		for rx := rx0; rx <= rx1; rx += res.Stride {
			scanned++
			if scanned > yySeedMaxStructureRegs {
				res.Truncated = true
				stop = true
				break
			}
			if scanned&1023 == 0 && time.Now().After(deadline) {
				res.Truncated = true
				stop = true
				break
			}
			cx, cz := yySeedStructureCandidate(seed, spec, int64(rx), int64(rz))
			if cx < int64(fromX) || cx > int64(toX) || cz < int64(fromZ) || cz > int64(toZ) {
				continue
			}
			res.Chunks = append(res.Chunks, [2]int{int(cx), int(cz)})
			if len(res.Chunks) >= limit {
				res.Truncated = true
				stop = true
				break
			}
		}
		if stop {
			break
		}
	}
	sort.Slice(res.Chunks, func(i, j int) bool {
		if res.Chunks[i][0] != res.Chunks[j][0] {
			return res.Chunks[i][0] < res.Chunks[j][0]
		}
		return res.Chunks[i][1] < res.Chunks[j][1]
	})

	// 抽样或截断时，告诉前端「还有多少个没画」（估算，不是精确缺失数）。
	if res.Sampled || res.Truncated {
		if est := yySeedEstimateTotal(res.Regions, spec); est > int64(len(res.Chunks)) {
			res.SkippedByLOD = int(est - int64(len(res.Chunks)))
		}
	}
	return res
}

// ── 热点图（粗粒度聚合）────────────────────────────────────────────

// yySeedHeatCell 把区块坐标映射到热点图格子（0..grid-1）。
func yySeedHeatCell(grid, fromX, fromZ, toX, toZ, cx, cz int) (int, int) {
	spanX := toX - fromX
	spanZ := toZ - fromZ
	if spanX <= 0 {
		spanX = 1
	}
	if spanZ <= 0 {
		spanZ = 1
	}
	gx := int(int64(cx-fromX) * int64(grid) / int64(spanX+1))
	gz := int(int64(cz-fromZ) * int64(grid) / int64(spanZ+1))
	if gx < 0 {
		gx = 0
	}
	if gx >= grid {
		gx = grid - 1
	}
	if gz < 0 {
		gz = 0
	}
	if gz >= grid {
		gz = grid - 1
	}
	return gx, gz
}

// yySeedHeat 生成结构热点图数据：把范围切成 grid×grid 格，每格统计结构候选数。
//
// 格子规格：lod=0 → 32×32、lod=1 → 64×64、lod=2 → 128×128。
// 只返回 count>0 的格子，格式 [gx,gz,count]；count 为 candidate 结构的数量（需实测确认）。
// 总量超过 60000 个区域时降为远景网格并标记 degraded，扫描循环仍受 150ms 预算约束。
func yySeedHeat(seed int64, specs []yySeedStructureSpec, fromX, fromZ, toX, toZ, lod int) map[string]any {
	if toX < fromX {
		fromX, toX = toX, fromX
	}
	if toZ < fromZ {
		fromZ, toZ = toZ, fromZ
	}
	lod = yySeedClampLOD(lod)

	totalRegions := int64(0)
	for _, spec := range specs {
		// 概率型结构没有区域网格，不计入工作量（spacing=1 会算出天文数字）
		if spec.Kind == yyStructKindProbability {
			continue
		}
		totalRegions += yySeedRegionCount(fromX, fromZ, toX, toZ, spec.Spacing)
	}
	degraded := false
	reason := ""
	if totalRegions > yySeedMaxStructureRegs {
		lod = yySeedLODFar
		degraded = true
		reason = "范围太大，已按远景抽样"
	}
	grid := yySeedHeatGrid(lod)

	deadline := time.Now().Add(yySeedQueryBudget)
	counts := make(map[[2]int]int, grid*grid/4)
	total := 0
	maxCount := 0
	truncated := false
	scanned := int64(0)

	for _, spec := range specs {
		// 同心环结构（要塞）走三环候选点，不用区域网格
		if spec.Kind == yyStructKindConcentric {
			for _, p := range yySeedStrongholds(seed, yySeedMaxStrongholds) {
				if p.ChunkX < fromX || p.ChunkX > toX || p.ChunkZ < fromZ || p.ChunkZ > toZ {
					continue
				}
				gx, gz := yySeedHeatCell(grid, fromX, fromZ, toX, toZ, p.ChunkX, p.ChunkZ)
				counts[[2]int{gx, gz}]++
				total++
				if counts[[2]int{gx, gz}] > maxCount {
					maxCount = counts[[2]int{gx, gz}]
				}
			}
			continue
		}
		// 概率型结构（矿井/宝藏）没有网格，热点图无意义，直接跳过
		if spec.Kind == yyStructKindProbability {
			continue
		}
		// 单类型区域数仍然过大时，按步长抽样（计数按 stride² 放大为估算值），保证有界。
		stride := 1
		if regions := yySeedRegionCount(fromX, fromZ, toX, toZ, spec.Spacing); regions > yySeedMaxStructureRegs {
			stride = int(math.Ceil(math.Sqrt(float64(regions) / float64(yySeedMaxStructureRegs))))
			if stride < 1 {
				stride = 1
			}
			truncated = true
		}
		rx0, rx1 := yySeedFloorDiv(fromX, spec.Spacing), yySeedFloorDiv(toX, spec.Spacing)
		rz0, rz1 := yySeedFloorDiv(fromZ, spec.Spacing), yySeedFloorDiv(toZ, spec.Spacing)
		stop := false
		for rz := rz0; rz <= rz1; rz += stride {
			for rx := rx0; rx <= rx1; rx += stride {
				scanned++
				if scanned&4095 == 0 && time.Now().After(deadline) {
					truncated = true
					stop = true
					break
				}
				cx, cz := yySeedStructureCandidate(seed, spec, int64(rx), int64(rz))
				if cx < int64(fromX) || cx > int64(toX) || cz < int64(fromZ) || cz > int64(toZ) {
					continue
				}
				gx, gz := yySeedHeatCell(grid, fromX, fromZ, toX, toZ, int(cx), int(cz))
				counts[[2]int{gx, gz}] += stride * stride
			}
			if stop {
				break
			}
		}
		if stop {
			break
		}
	}

	cells := make([][3]int, 0, len(counts))
	for key, n := range counts {
		if n <= 0 {
			continue
		}
		cells = append(cells, [3]int{key[0], key[1], n})
		total += n
		if n > maxCount {
			maxCount = n
		}
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i][2] != cells[j][2] {
			return cells[i][2] > cells[j][2]
		}
		if cells[i][0] != cells[j][0] {
			return cells[i][0] < cells[j][0]
		}
		return cells[i][1] < cells[j][1]
	})

	// types 只列真正参与统计的结构：概率型没有网格、不会产生任何格子，
	// 列进去会让前端以为「这一类算过了但一个点都没有」，属于误导。
	types := make([]string, 0, len(specs))
	for _, spec := range specs {
		if spec.Kind == yyStructKindProbability {
			continue
		}
		types = append(types, spec.ID)
	}

	return map[string]any{
		"ok": true, "seed": seed, "lod": lod, "grid": grid,
		"from_x": fromX, "from_z": fromZ, "to_x": toX, "to_z": toZ,
		"cells": cells, "cell_count": len(cells), "total": total, "max_count": maxCount,
		"types": types, "degraded": degraded, "reason": reason, "truncated": truncated,
		"workload_regions": totalRegions,
		"precision":        "candidate",
		"note":             "每格 [gx,gz,count]：count 为该格内结构候选数（candidate，需实测）；只返回 count>0 的格子；lod 越小格子越粗",
	}
}

// ── 坐标分享码（exact）──────────────────────────────────────────────

// yySeedEncodeCode 把 seed + 方块坐标编码成带校验和的 base36 短串。
// 布局：8 字节 seed + 4 字节 x + 4 字节 z（大端），追加 4 字节 CRC32，整体 base36。
// 前缀 YY1 表示格式版本；解码时先验长度再验校验和，防止群里传串号。
func yySeedEncodeCode(seed int64, x, z int32) string {
	buf := make([]byte, 20)
	binary.BigEndian.PutUint64(buf[0:8], uint64(seed))
	binary.BigEndian.PutUint32(buf[8:12], uint32(x))
	binary.BigEndian.PutUint32(buf[12:16], uint32(z))
	binary.BigEndian.PutUint32(buf[16:20], crc32.ChecksumIEEE(buf[0:16]))
	n := new(big.Int).SetBytes(buf)
	return "YY1" + strings.ToUpper(n.Text(36))
}

func yySeedDecodeCode(code string) (seed int64, x, z int32, err error) {
	raw := strings.ToUpper(strings.TrimSpace(code))
	if !strings.HasPrefix(raw, "YY1") {
		return 0, 0, 0, errSeedCode("分享码必须以 YY1 开头")
	}
	body := strings.TrimPrefix(raw, "YY1")
	if body == "" || len(body) > 40 {
		return 0, 0, 0, errSeedCode("分享码长度不合法")
	}
	n, ok := new(big.Int).SetString(body, 36)
	if !ok {
		return 0, 0, 0, errSeedCode("分享码包含非法字符（只允许 0-9 A-Z）")
	}
	b := n.Bytes()
	if len(b) > 20 {
		return 0, 0, 0, errSeedCode("分享码内容超出预期长度")
	}
	buf := make([]byte, 20)
	copy(buf[20-len(b):], b)
	want := binary.BigEndian.Uint32(buf[16:20])
	if crc32.ChecksumIEEE(buf[0:16]) != want {
		return 0, 0, 0, errSeedCode("分享码校验失败，可能被改动或复制不完整")
	}
	seed = int64(binary.BigEndian.Uint64(buf[0:8]))
	x = int32(binary.BigEndian.Uint32(buf[8:12]))
	z = int32(binary.BigEndian.Uint32(buf[12:16]))
	// 规范化校验：同一组数据只允许一种写法，避免前导零造成的多重编码
	if yySeedEncodeCode(seed, x, z) != raw {
		return 0, 0, 0, errSeedCode("分享码不是规范编码")
	}
	return seed, x, z, nil
}

type yySeedCodeError string

func (e yySeedCodeError) Error() string { return string(e) }

func errSeedCode(msg string) error { return yySeedCodeError(msg) }

// ── 通用小工具 ──────────────────────────────────────────────────────

func yySeedClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func yySeedFloorDiv(a, b int) int {
	if b == 0 {
		return 0
	}
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func yySeedFail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": msg})
}

// yySeedGuard 统一做请求防护：仅 GET、query 长度上限、body 上限、panic 兜底。
func yySeedGuard(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				yySeedFail(w, http.StatusBadRequest, "参数导致计算异常，请检查输入")
			}
		}()
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			yySeedFail(w, http.StatusMethodNotAllowed, "种子查询接口仅支持 GET")
			return
		}
		if len(r.URL.RawQuery) > yySeedMaxRawQuery {
			yySeedFail(w, http.StatusBadRequest, "查询参数过长")
			return
		}
		fn(w, r)
	}
}

func yySeedParamInt64(r *http.Request, key string) (int64, bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, false, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, true, errSeedCode("参数 " + key + " 必须是 64 位整数")
	}
	return v, true, nil
}

func yySeedParamInt(r *http.Request, key string, def int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errSeedCode("参数 " + key + " 必须是整数")
	}
	return v, nil
}

// yySeedParamLOD 读取 lod 参数（0/1/2），缺省 0；越界按最近档夹紧，避免前端传错就白屏。
func yySeedParamLOD(r *http.Request) (int, error) {
	lod, err := yySeedParamInt(r, "lod", yySeedLODFar)
	if err != nil {
		return 0, err
	}
	return yySeedClampLOD(lod), nil
}

// ── 能力清单 ────────────────────────────────────────────────────────

func yySeedManifest() map[string]any {
	return map[string]any{
		"ok":               true,
		"editions":         []string{"java", "bedrock"},
		"computed_edition": "java",
		"capabilities": []map[string]any{
			{"id": "slime", "label": "史莱姆区块", "precision": "exact", "note": "Java 版原版算法（java.util.Random + 盐值 0x3ad8025f），可复现；基岩版算法不同，未实现"},
			{"id": "stronghold", "label": "要塞坐标", "precision": "approximate", "note": "Java 1.9+ 三环半径 1408/4352/7424 方块、每环 128 候选角；省略原版生物群系过滤，实际位置可能偏移"},
			{"id": "structure", "label": "结构候选", "precision": "candidate", "note": "按 Java 1.13+ 区域网格 + 盐值算候选区块，需进游戏实测确认；覆盖主世界 17 种 / 下界 3 种 / 末地 1 种，支持 dimension 与 lod=0/1/2 分级细节"},
			{"id": "heat", "label": "结构热点图", "precision": "candidate", "note": "粗粒度聚合：范围切成 32×32 / 64×64 / 128×128 格，只返回 count>0 的格子，供前端远景画密度、放大再取精确点"},
			{"id": "coordinates", "label": "坐标分享码", "precision": "exact", "note": "种子+方块坐标编码为 base36，带 CRC32 校验，可反解"},
			{"id": "biome", "label": "生物群系", "precision": "approximate", "note": "群系地图：1.18+ 气候噪声栈 / 1.7-1.17 温湿度双噪声两套算法；支持 overworld/nether/the_end；末地精确、下界为 y=64 切片近似、基岩版只代表 Java 语义"},
		},
		"limits": map[string]any{
			"max_slime_radius":        yySeedMaxSlimeRadius,
			"max_range_chunks":        yySeedMaxChunkRange,
			"max_results":             yySeedMaxResults,
			"max_strongholds":         yySeedMaxStrongholds,
			"max_query_radius_chunks": yySeedMaxQueryRadius,
			"request_budget_ms":       int(yySeedQueryBudget / time.Millisecond),
			"max_workload_regions":    yySeedMaxStructureRegs,
		},
		"lod": map[string]any{
			"levels": []map[string]any{
				{"lod": 0, "label": "远景/全局（默认）", "max_points": yySeedLODLimitFar, "sampled": true, "heat_grid": yySeedHeatGridFar},
				{"lod": 1, "label": "中景", "max_points": yySeedLODLimitMid, "sampled": false, "heat_grid": yySeedHeatGridMid},
				{"lod": 2, "label": "近景/放大", "max_points": yySeedLODLimitNear, "sampled": false, "heat_grid": yySeedHeatGridNear},
			},
			"degrade_regions": yySeedMaxStructureRegs,
			"note":            "缩小看全局用 lod=0（抽样，只画少量点），放大后用 lod=2（逐个精确算）；工作量超限会自动降到 lod=0 并返回 degraded=true",
		},
		"dimensions":     yySeedDimensionCatalog(),
		"structures":     yySeedStructureCatalog(),
		"game_versions":  yySeedGameVersions(),
		"version_matrix": yySeedVersionMatrix(),
		"note":           "全部结果在本机离线计算，不联网、不读档；已覆盖三维度群系与 21 种结构，未实现出生点估算、要塞内部结构与实时渲染",
	}
}

// ── HTTP 路由 ───────────────────────────────────────────────────────

// yySeedRoutes 注册 /api/seed/* 全部 GET 接口。
func yySeedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/seed/manifest", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, yySeedManifest())
	}))

	mux.HandleFunc("/api/seed/slime", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		seed, ok, err := yySeedParamInt64(r, "seed")
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if !ok {
			yySeedFail(w, http.StatusBadRequest, "缺少 seed 参数")
			return
		}
		if ed := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("edition"))); ed != "" && ed != "java" {
			yySeedFail(w, http.StatusBadRequest, "史莱姆区块目前只支持 Java 版；基岩版算法不同，未实现")
			return
		}
		cx, err := yySeedParamInt(r, "x", 0)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		cz, err := yySeedParamInt(r, "z", 0)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		radius, err := yySeedParamInt(r, "radius", 8)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		res := yySeedSlime(seed, int32(cx), int32(cz), radius)
		writeJSON(w, map[string]any{
			"ok": true, "seed": res.Seed, "edition": res.Edition, "radius": res.Radius,
			"chunks": res.Chunks, "blocks": res.Blocks, "count": res.Count,
			"note": "radius 超过 64 会被夹到 64；区块坐标乘以 16 即方块坐标",
		})
	}))

	mux.HandleFunc("/api/seed/stronghold", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		seed, ok, err := yySeedParamInt64(r, "seed")
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if !ok {
			yySeedFail(w, http.StatusBadRequest, "缺少 seed 参数")
			return
		}
		count, err := yySeedParamInt(r, "count", 3)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		points := yySeedStrongholds(seed, count)
		writeJSON(w, map[string]any{
			"ok": true, "seed": seed, "edition": "java", "count": len(points),
			"points": points, "precision": "approximate",
			"note": "Java 版三环算法（半径 1408/4352/7424 方块，每环 128 候选角）；省略生物群系过滤，坐标可能偏移；基岩版算法不同，不支持",
		})
	}))

	mux.HandleFunc("/api/seed/structure", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		seed, ok, err := yySeedParamInt64(r, "seed")
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if !ok {
			yySeedFail(w, http.StatusBadRequest, "缺少 seed 参数")
			return
		}
		typ := yyStructNormalizeID(r.URL.Query().Get("type"))
		dim := yyStructNormalizeID(r.URL.Query().Get("dimension"))
		dimProvided := dim != ""
		if !dimProvided {
			dim = yyStructDimOverworld
		}
		if typ == "" {
			yySeedFail(w, http.StatusBadRequest, "缺少 type 参数（可选："+yySeedStructureIDsByDim(dim)+"/all；dimension 可选 overworld/nether/the_end）")
			return
		}
		// type=all：一次请求返回该维度全部结构的点位。
		// 这样前端每屏只花 1 次配额，而不是 15 次——直接对应用户要求的「整体刷新、省额度」。
		isAll := typ == "all"
		var spec yySeedStructureSpec
		if !isAll {
			found := false
			spec, found = yySeedStructureSpecByID(typ)
			if !found {
				yySeedFail(w, http.StatusBadRequest, "不支持的结构类型："+typ+"（可选："+yySeedStructureIDsByDim(dim)+"/all）")
				return
			}
			// 没显式给 dimension 时按结构自身维度推断（要塞/下界要塞/末地城都能直接查）；
			// 显式给了但和结构维度冲突才报错，避免拿主世界坐标去算下界要塞这种明显错误。
			if !dimProvided {
				dim = spec.Dimension
			}
			if dim != "all" && spec.Dimension != dim {
				yySeedFail(w, http.StatusBadRequest, spec.Label+" 属于 "+
					yySeedDimensionLabel(spec.Dimension)+" 维度，请把 dimension 设为 "+spec.Dimension+" 或去掉该参数")
				return
			}
		}
		fromX, err := yySeedParamInt(r, "from_x", -256)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		fromZ, err := yySeedParamInt(r, "from_z", -256)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		toX, err := yySeedParamInt(r, "to_x", 256)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		toZ, err := yySeedParamInt(r, "to_z", 256)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if toX-fromX > yySeedMaxChunkRange || toZ-fromZ > yySeedMaxChunkRange || fromX-toX > yySeedMaxChunkRange || fromZ-toZ > yySeedMaxChunkRange {
			yySeedFail(w, http.StatusBadRequest, "查询范围过大：单轴区块跨度不能超过 4096")
			return
		}
		lod, err := yySeedParamLOD(r)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		deadline := time.Now().Add(yySeedQueryBudget)
		if isAll {
			specs := yySeedStructureSpecsByDim(dim)
			list := make([]map[string]any, 0, len(specs))
			total := 0
			truncated := false
			skipped := 0
			for _, sp := range specs {
				res := yySeedStructureScanSpec(seed, sp, fromX, fromZ, toX, toZ, lod, 0, deadline)
				if res.Truncated {
					truncated = true
				}
				skipped += res.SkippedByLOD
				total += len(res.Chunks)
				item := map[string]any{
					"type": sp.ID, "label": sp.Label, "kind": sp.Kind, "since": sp.Since,
					"chunks": res.Chunks, "count": len(res.Chunks),
					"precision": yySeedStructurePrecision(sp),
					"truncated": res.Truncated, "sampled": res.Sampled,
					"skipped_by_lod": res.SkippedByLOD,
				}
				if sp.Kind == yyStructKindProbability {
					item["probability"] = sp.Probability
					item["note"] = sp.Note
				}
				list = append(list, item)
			}
			writeJSON(w, map[string]any{
				"ok": true, "seed": seed, "type": "all", "edition": "java",
				"dimension": dim, "dimension_label": yySeedDimensionLabel(dim),
				"structures": list, "type_count": len(list), "total": total,
				"lod": lod, "truncated": truncated, "skipped_by_lod": skipped,
				"note": "一次返回该维度全部结构的候选区块；概率型结构（矿井/宝藏）只给概率不给坐标",
			})
			return
		}
		res := yySeedStructureScanSpec(seed, spec, fromX, fromZ, toX, toZ, lod, 0, deadline)
		blocks := make([][2]int, 0, len(res.Chunks))
		for _, c := range res.Chunks {
			blocks = append(blocks, [2]int{c[0] * 16, c[1] * 16})
		}
		out := map[string]any{
			"ok": true, "seed": seed, "type": spec.ID, "label": spec.Label, "edition": "java",
			"dimension": spec.Dimension, "dimension_label": yySeedDimensionLabel(spec.Dimension),
			"kind": spec.Kind, "since": spec.Since,
			"chunks": res.Chunks, "blocks": blocks, "count": len(res.Chunks), "truncated": res.Truncated,
			"lod": res.LOD, "sampled": res.Sampled, "skipped_by_lod": res.SkippedByLOD,
			"degraded": res.Degraded, "reason": res.Reason,
			"workload_regions": res.Regions, "region_stride": res.Stride,
			"precision": yySeedStructurePrecision(spec),
			"note":      "候选区块，实际是否生成取决于生物群系与版本，需实测确认；lod=0 为远景抽样（最多 40 点），lod=2 为近景精确（最多 200 点）",
		}
		if spec.Kind == yyStructKindSpread {
			out["spacing"] = spec.Spacing
			out["separation"] = spec.Separation
			out["salt"] = spec.Salt
		}
		if spec.Kind == yyStructKindProbability {
			out["probability"] = spec.Probability
			out["note"] = spec.Note + "；本工具给不出确定坐标，只给概率"
		}
		writeJSON(w, out)
	}))

	mux.HandleFunc("/api/seed/query", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		seed, ok, err := yySeedParamInt64(r, "seed")
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if !ok {
			yySeedFail(w, http.StatusBadRequest, "缺少 seed 参数")
			return
		}
		cx, err := yySeedParamInt(r, "x", 0)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		cz, err := yySeedParamInt(r, "z", 0)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		radius, err := yySeedParamInt(r, "radius", 16)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		lod, err := yySeedParamLOD(r)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, yySeedQueryDim(seed, cx, cz, radius, lod, yyStructNormalizeID(r.URL.Query().Get("dimension"))))
	}))

	mux.HandleFunc("/api/seed/heat", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		seed, ok, err := yySeedParamInt64(r, "seed")
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if !ok {
			yySeedFail(w, http.StatusBadRequest, "缺少 seed 参数")
			return
		}
		typ := yyStructNormalizeID(r.URL.Query().Get("type"))
		dim := yyStructNormalizeID(r.URL.Query().Get("dimension"))
		dimProvided := dim != ""
		if !dimProvided {
			dim = yyStructDimOverworld
		}
		if typ == "" {
			yySeedFail(w, http.StatusBadRequest, "缺少 type 参数（可选："+yySeedStructureIDsByDim(dim)+"，或 all 表示该维度全部）")
			return
		}
		var specs []yySeedStructureSpec
		if typ == "all" {
			specs = yySeedStructureSpecsByDim(dim)
		} else {
			spec, found := yySeedStructureSpecByID(typ)
			if !found {
				yySeedFail(w, http.StatusBadRequest, "不支持的结构类型："+typ+"（可选："+yySeedStructureIDsByDim(dim)+"/all）")
				return
			}
			if !dimProvided {
				dim = spec.Dimension
			}
			if dim != "all" && spec.Dimension != dim {
				yySeedFail(w, http.StatusBadRequest, spec.Label+" 属于 "+
					yySeedDimensionLabel(spec.Dimension)+" 维度，请把 dimension 设为 "+spec.Dimension)
				return
			}
			specs = []yySeedStructureSpec{spec}
		}
		fromX, err := yySeedParamInt(r, "from_x", -256)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		fromZ, err := yySeedParamInt(r, "from_z", -256)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		toX, err := yySeedParamInt(r, "to_x", 256)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		toZ, err := yySeedParamInt(r, "to_z", 256)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if toX-fromX > yySeedMaxChunkRange || toZ-fromZ > yySeedMaxChunkRange || fromX-toX > yySeedMaxChunkRange || fromZ-toZ > yySeedMaxChunkRange {
			yySeedFail(w, http.StatusBadRequest, "查询范围过大：单轴区块跨度不能超过 4096")
			return
		}
		lod, err := yySeedParamLOD(r)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, yySeedHeat(seed, specs, fromX, fromZ, toX, toZ, lod))
	}))

	mux.HandleFunc("/api/seed/coordinates", yySeedGuard(func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code != "" {
			seed, x, z, err := yySeedDecodeCode(code)
			if err != nil {
				yySeedFail(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, map[string]any{"ok": true, "action": "decode", "code": strings.ToUpper(strings.TrimSpace(code)), "seed": seed, "x": x, "z": z})
			return
		}
		seed, ok, err := yySeedParamInt64(r, "seed")
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if !ok {
			yySeedFail(w, http.StatusBadRequest, "缺少 seed 参数（或提供 code 反解）")
			return
		}
		x, err := yySeedParamInt(r, "x", 0)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		z, err := yySeedParamInt(r, "z", 0)
		if err != nil {
			yySeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if x < math.MinInt32 || x > math.MaxInt32 || z < math.MinInt32 || z > math.MaxInt32 {
			yySeedFail(w, http.StatusBadRequest, "坐标超出可编码范围")
			return
		}
		writeJSON(w, map[string]any{
			"ok": true, "action": "encode", "seed": seed, "x": x, "z": z,
			"code": yySeedEncodeCode(seed, int32(x), int32(z)),
			"note": "分享码带 CRC32 校验，改动任意一位都会解码失败",
		})
	}))
	// 生物群系（群系地图）：单点 ?x=&z= 与批量 ?from_x=...&to_x=... 两种形态，
	// 支持 overworld/nether/the_end 三维度与 1.7-1.17 / 1.18+ 两套算法。实现见 youyuan_seed_biome.go。
	mux.HandleFunc("/api/seed/biome", yySeedGuard(yySeedBiomeHandler))
}

// yySeedQuery 一次性返回附近要塞、史莱姆区块数量与各类结构候选，并给一句中文总结。
// 工作量上限：半径 ≤ 32 区块、总预算 150ms；LOD 默认远景（lod=0）。
func yySeedQuery(seed int64, cx, cz, radius int) map[string]any {
	return yySeedQueryDim(seed, cx, cz, radius, yySeedLODFar, "")
}

// yySeedQueryLOD 是带 LOD 档位的聚合查询：每个结构类型独立套用 LOD 规则
// （lod=0 抽样 ≤40、lod=1 正常网格 ≤120、lod=2 精确 ≤200），互不影响。
func yySeedQueryLOD(seed int64, cx, cz, radius, lod int) map[string]any {
	return yySeedQueryDim(seed, cx, cz, radius, lod, "")
}

// yySeedQueryDim 带维度的聚合查询：dim 为空按主世界。
//
// 维度差异：
//   - 主世界：史莱姆区块 + 要塞 + 17 种结构
//   - 下界：无史莱姆区块/要塞，3 种结构（坐标是下界坐标系）
//   - 末地：无史莱姆区块/要塞，末地城（坐标是末地坐标系）
func yySeedQueryDim(seed int64, cx, cz, radius, lod int, dim string) map[string]any {
	if dim == "" {
		dim = yyStructDimOverworld
	}
	if dim == "end" {
		dim = yyStructDimEnd
	}
	if dim == "the_nether" {
		dim = yyStructDimNether
	}
	radius = yySeedClamp(radius, 0, yySeedMaxQueryRadius)
	lod = yySeedClampLOD(lod)
	deadline := time.Now().Add(yySeedQueryBudget)

	// 史莱姆区块与要塞只在主世界成立，其它维度返回空，避免给出错误数字
	slime := yySeedSlime(seed, int32(cx), int32(cz), radius)
	strongholds := yySeedStrongholds(seed, 3)
	if dim != yyStructDimOverworld {
		slime = yySeedSlimeResult{Seed: seed, Edition: "java", Radius: radius, Chunks: [][2]int{}, Blocks: [][2]int{}}
		strongholds = []yySeedStronghold{}
	}

	structures := []map[string]any{}
	totalCandidates := 0
	truncated := false
	sampledAny := false
	degraded := false
	reason := ""
	skippedTotal := 0
	for _, spec := range yySeedStructureSpecsByDim(dim) {
		res := yySeedStructureScanSpec(seed, spec, cx-radius, cz-radius, cx+radius, cz+radius, lod, 0, deadline)
		if res.Truncated {
			truncated = true
		}
		if res.Sampled {
			sampledAny = true
		}
		if res.Degraded {
			degraded = true
			reason = res.Reason
		}
		skippedTotal += res.SkippedByLOD
		totalCandidates += len(res.Chunks)
		item := map[string]any{
			"type": spec.ID, "label": spec.Label, "count": len(res.Chunks),
			"chunks": res.Chunks, "lod": res.LOD, "sampled": res.Sampled,
			"skipped_by_lod": res.SkippedByLOD,
			"precision":      yySeedStructurePrecision(spec),
			"kind":           spec.Kind,
		}
		if spec.Kind == yyStructKindProbability {
			item["probability"] = spec.Probability
			item["note"] = spec.Note
		}
		structures = append(structures, item)
	}
	if time.Now().After(deadline) {
		truncated = true
	}

	villageCount := 0
	for _, s := range structures {
		if s["type"] == "village" {
			villageCount = s["count"].(int)
		}
	}

	summary := "种子 " + strconv.FormatInt(seed, 10) + "（" + yySeedDimensionLabel(dim) + "）："
	if dim == yyStructDimOverworld {
		summary += "附近有 " + strconv.Itoa(len(strongholds)) + " 个要塞、" +
			strconv.Itoa(slime.Count) + " 个史莱姆区块、" + strconv.Itoa(villageCount) + " 个村庄候选"
	} else {
		summary += "附近共 " + strconv.Itoa(totalCandidates) + " 个结构候选（该维度没有史莱姆区块与要塞）"
	}
	if slime.Count == 0 && dim == yyStructDimOverworld {
		summary += "（该范围没有史莱姆区块，可扩大半径）"
	}
	if sampledAny {
		summary += "（当前为远景抽样，放大到更小范围可看全部点位）"
	}

	return map[string]any{
		"ok": true, "seed": seed, "x": cx, "z": cz, "radius": radius, "edition": "java",
		"dimension":       dim,
		"dimension_label": yySeedDimensionLabel(dim),
		"strongholds":     strongholds,
		"slime":           map[string]any{"count": slime.Count, "chunks": slime.Chunks, "precision": "exact"},
		"structures":      structures,
		"structure_total": totalCandidates,
		"truncated":       truncated,
		"lod":             lod,
		"sampled":         sampledAny,
		"skipped_by_lod":  skippedTotal,
		"degraded":        degraded,
		"reason":          reason,
		"summary":         summary,
		"note":            "要塞为 approximate（省略生物群系过滤），网格结构为 candidate（需实测），概率型结构为 unsupported（只给概率），史莱姆为 exact（Java 版）；lod=0 远景抽样、lod=2 近景精确",
	}
}

// ── 内置机器人工具定义 ──────────────────────────────────────────────

// yySeedAITools 返回给内置 AI 机器人注册的种子查询工具（与 yyQQAITools 同类型 ToolDef）。
// 返回 nil 表示该能力未启用；本实现始终可用（纯本地计算，无外部依赖）。
func yySeedAITools() []ToolDef {
	str := func() map[string]any { return map[string]any{"type": "string"} }
	integer := func(lo, hi int) map[string]any {
		m := map[string]any{"type": "integer"}
		if hi > lo {
			m["minimum"] = lo
			m["maximum"] = hi
		}
		return m
	}
	return []ToolDef{
		{Name: "seed_manifest", Description: "查看本机种子查询器支持的能力与精度分级（Java 版算法，离线计算）", Parameters: yySchema(map[string]any{})},
		{Name: "seed_slime", Description: "查询 Java 版史莱姆区块（精确）。x/z 为区块坐标，radius 为区块半径（最大 64）", Parameters: yySchema(map[string]any{
			"seed": integer(0, 0), "x": integer(0, 0), "z": integer(0, 0), "radius": integer(0, yySeedMaxSlimeRadius),
		}, "seed")},
		{Name: "seed_stronghold", Description: "查 Java 版要塞坐标。128 个环位按 cubiomes 逐位复刻，精确，可直接照着传送", Parameters: yySchema(map[string]any{
			"seed": integer(0, 0), "count": integer(1, yySeedMaxStrongholds),
		}, "seed")},
		{Name: "seed_structure", Description: "查询结构坐标（村庄/府邸/海底神殿/要塞/远古城市/末地城/末地船/下界要塞/堡垒遗迹等 28 种）。同一版本同一维度内位置算法与游戏原版一致；古迹废墟、试炼密室、沙漠水井等缺生物群系过滤的会标 candidate，需进游戏确认。dimension：overworld/nether/end", Parameters: yySchema(map[string]any{
			"seed": integer(0, 0), "type": str(), "dimension": str(), "from_x": integer(0, 0), "from_z": integer(0, 0), "to_x": integer(0, 0), "to_z": integer(0, 0),
			"lod": integer(yySeedLODFar, yySeedLODNear),
		}, "seed", "type")},
		{Name: "seed_heat", Description: "结构热点图：把范围切成 32×32 / 64×64 / 128×128 格，返回 [gx,gz,count]，只含 count>0 的格子（远景看密度，放大再取精确点）", Parameters: yySchema(map[string]any{
			"seed": integer(0, 0), "type": str(), "dimension": str(), "from_x": integer(0, 0), "from_z": integer(0, 0), "to_x": integer(0, 0), "to_z": integer(0, 0),
			"lod": integer(yySeedLODFar, yySeedLODNear),
		}, "seed", "type")},
		{Name: "seed_query", Description: "一句话聚合查询：附近最近的结构、地形概况，返回可直接念给群友的中文 summary。dimension 可选 overworld/nether/end", Parameters: yySchema(map[string]any{
			"seed": integer(0, 0), "x": integer(0, 0), "z": integer(0, 0), "radius": integer(0, yySeedMaxQueryRadius),
			"lod": integer(yySeedLODFar, yySeedLODNear), "dimension": str(),
		}, "seed")},
		{Name: "seed_biome", Description: "查生物群系（1.18+ 气候噪声 + 参数查表，与本机地图同源）。给 x/z 查单点，或给 from_x/from_z/to_x/to_z 批量查。dimension 目前只支持 overworld 精确；nether/end 只给代表群系。version 如 1.21.4 / 1.16.5", Parameters: yySchema(map[string]any{
			"seed": integer(0, 0), "x": integer(0, 0), "z": integer(0, 0),
			"from_x": integer(0, 0), "from_z": integer(0, 0), "to_x": integer(0, 0), "to_z": integer(0, 0),
			"step": integer(0, 0), "dimension": str(), "version": str(),
		}, "seed")},
		{Name: "seed_coordinates", Description: "生成或反解坐标分享码（带校验和）。提供 code 表示反解，否则用 seed+x+z 生成", Parameters: yySchema(map[string]any{
			"seed": integer(0, 0), "x": integer(0, 0), "z": integer(0, 0), "code": str(),
		})},
	}
}

// yySeedToolCall 执行 yySeedAITools 中定义的工具，供内置机器人工具循环调用。
// 与 HTTP 接口共用同一套核心函数，避免两套逻辑漂移。
func yySeedToolCall(name string, args map[string]any) (map[string]any, error) {
	// 2026-10-26 第 28 条：除「坐标分享码」外，全部切到 v2 引擎
	//（youyuan_seedv2_aitools.go）。参数名保持兼容，机器人既有调用不用改。
	if out, handled, err := yySeedV2ToolCall(name, args); handled {
		return out, err
	}
	argInt := func(key string, def int) int {
		if v, ok := args[key]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			case int64:
				return int(n)
			case json.Number:
				if x, e := n.Int64(); e == nil {
					return int(x)
				}
			case string:
				if x, e := strconv.Atoi(strings.TrimSpace(n)); e == nil {
					return x
				}
			}
		}
		return def
	}
	argStr := func(key string) string {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
		return ""
	}
	needSeed := func() (int64, error) {
		v, ok := args["seed"]
		if !ok {
			return 0, errSeedCode("缺少 seed 参数")
		}
		switch n := v.(type) {
		case float64:
			return int64(n), nil
		case int64:
			return n, nil
		case json.Number:
			return n.Int64()
		case string:
			return strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		}
		return 0, errSeedCode("seed 必须是整数")
	}

	switch name {
	case "seed_manifest":
		return yySeedManifest(), nil
	case "seed_slime":
		seed, err := needSeed()
		if err != nil {
			return nil, err
		}
		res := yySeedSlime(seed, int32(argInt("x", 0)), int32(argInt("z", 0)), argInt("radius", 8))
		return map[string]any{"ok": true, "seed": res.Seed, "edition": res.Edition, "radius": res.Radius,
			"chunks": res.Chunks, "blocks": res.Blocks, "count": res.Count}, nil
	case "seed_stronghold":
		seed, err := needSeed()
		if err != nil {
			return nil, err
		}
		points := yySeedStrongholds(seed, argInt("count", 3))
		return map[string]any{"ok": true, "seed": seed, "edition": "java", "count": len(points), "points": points, "precision": "approximate"}, nil
	case "seed_structure":
		seed, err := needSeed()
		if err != nil {
			return nil, err
		}
		dim := yyStructNormalizeID(argStr("dimension"))
		dimProvided := dim != ""
		if !dimProvided {
			dim = yyStructDimOverworld
		}
		spec, ok := yySeedStructureSpecByID(argStr("type"))
		if !ok {
			return nil, errSeedCode("不支持的结构类型，可选：" + yySeedStructureIDsByDim(dim))
		}
		if !dimProvided {
			dim = spec.Dimension
		}
		if dim != "all" && spec.Dimension != dim {
			return nil, errSeedCode(spec.Label + " 属于 " + yySeedDimensionLabel(spec.Dimension) + " 维度，请把 dimension 设为 " + spec.Dimension)
		}
		fromX, fromZ := argInt("from_x", -256), argInt("from_z", -256)
		toX, toZ := argInt("to_x", 256), argInt("to_z", 256)
		if toX-fromX > yySeedMaxChunkRange || toZ-fromZ > yySeedMaxChunkRange {
			return nil, errSeedCode("查询范围过大：单轴区块跨度不能超过 4096")
		}
		res := yySeedStructureScanSpec(seed, spec, fromX, fromZ, toX, toZ, argInt("lod", yySeedLODFar), 0, time.Now().Add(yySeedQueryBudget))
		out := map[string]any{"ok": true, "seed": seed, "type": spec.ID, "label": spec.Label,
			"dimension": spec.Dimension, "kind": spec.Kind, "since": spec.Since,
			"chunks": res.Chunks, "count": len(res.Chunks), "truncated": res.Truncated,
			"lod": res.LOD, "sampled": res.Sampled, "skipped_by_lod": res.SkippedByLOD,
			"degraded": res.Degraded, "reason": res.Reason, "precision": yySeedStructurePrecision(spec)}
		if spec.Kind == yyStructKindProbability {
			out["probability"] = spec.Probability
			out["note"] = spec.Note
		}
		return out, nil
	case "seed_heat":
		seed, err := needSeed()
		if err != nil {
			return nil, err
		}
		typ := yyStructNormalizeID(argStr("type"))
		dim := yyStructNormalizeID(argStr("dimension"))
		dimProvided := dim != ""
		if !dimProvided {
			dim = yyStructDimOverworld
		}
		var specs []yySeedStructureSpec
		if typ == "all" {
			specs = yySeedStructureSpecsByDim(dim)
		} else {
			spec, ok := yySeedStructureSpecByID(typ)
			if !ok {
				return nil, errSeedCode("不支持的结构类型，可选：" + yySeedStructureIDsByDim(dim) + "/all")
			}
			if !dimProvided {
				dim = spec.Dimension
			}
			if dim != "all" && spec.Dimension != dim {
				return nil, errSeedCode(spec.Label + " 属于 " + yySeedDimensionLabel(spec.Dimension) + " 维度，请把 dimension 设为 " + spec.Dimension)
			}
			specs = []yySeedStructureSpec{spec}
		}
		fromX, fromZ := argInt("from_x", -256), argInt("from_z", -256)
		toX, toZ := argInt("to_x", 256), argInt("to_z", 256)
		if toX-fromX > yySeedMaxChunkRange || toZ-fromZ > yySeedMaxChunkRange {
			return nil, errSeedCode("查询范围过大：单轴区块跨度不能超过 4096")
		}
		return yySeedHeat(seed, specs, fromX, fromZ, toX, toZ, argInt("lod", yySeedLODFar)), nil
	case "seed_query":
		seed, err := needSeed()
		if err != nil {
			return nil, err
		}
		return yySeedQueryDim(seed, argInt("x", 0), argInt("z", 0), argInt("radius", 16), argInt("lod", yySeedLODFar), argStr("dimension")), nil
	case "seed_biome":
		seed, err := needSeed()
		if err != nil {
			return nil, err
		}
		dim := argStr("dimension")
		if dim == "" {
			dim = yyStructDimOverworld
		}
		ver := argStr("version")
		// 给了 from_x 就走批量，否则单点
		if _, hasFrom := args["from_x"]; hasFrom {
			fromX, fromZ := argInt("from_x", 0), argInt("from_z", 0)
			toX, toZ := argInt("to_x", fromX), argInt("to_z", fromZ)
			step := argInt("step", 16)
			pts, usedStep, truncated := yyBiomeBatchPoints(seed, int64(fromX), int64(fromZ), int64(toX), int64(toZ), step, dim, ver)
			return map[string]any{"ok": true, "seed": seed, "dimension": dim, "version": ver,
				"step": usedStep, "points": pts, "count": len(pts), "truncated": truncated,
				"precision": "approximate"}, nil
		}
		def, prec, algo := yyBiomeAt(seed, int64(argInt("x", 0)), int64(argInt("z", 0)), dim, ver)
		return map[string]any{"ok": true, "seed": seed, "x": argInt("x", 0), "z": argInt("z", 0),
			"dimension": dim, "version": ver, "biome": def.Key, "biome_id": def.ID,
			"name": def.Name, "precision": prec, "algorithm": algo}, nil
	case "seed_coordinates":
		if code := argStr("code"); code != "" {
			seed, x, z, err := yySeedDecodeCode(code)
			if err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "action": "decode", "seed": seed, "x": x, "z": z}, nil
		}
		seed, err := needSeed()
		if err != nil {
			return nil, err
		}
		x, z := argInt("x", 0), argInt("z", 0)
		return map[string]any{"ok": true, "action": "encode", "seed": seed, "x": x, "z": z,
			"code": yySeedEncodeCode(seed, int32(x), int32(z))}, nil
	}
	return nil, errSeedCode("未知的种子工具：" + name)
}
