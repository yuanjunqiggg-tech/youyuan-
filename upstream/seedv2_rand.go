package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_rand.go —— Java LCG / Xoroshiro128++ / Perlin / DoublePerlin / Voronoi
//
// 【归属】agent: seed-rand-noise
// 【来源】D:/种子查询器自研/src/world.mjs（JRandom）+ src/noise.mjs（全套）
//         + src/biomegen.mjs（CLIMATE_DEF / buildClimate / sampleClimate / 样条）
// 【纪律】逐行对照移植，不许"优化"、不许改公式。JS 用 BigInt 的地方 Go 直接用
//         uint64 / int64，语义更容易对齐，但注意 JS 的 `| 0` 是**有符号 32 位**，
//         对应 Go 的 int32(...)。
//
// 本文件对外只暴露下列符号，禁止改名：
//   yyV2NewRNG / yyV2NewRNGFromState / (*yyV2RNG).Next/NextInt/NextLong/NextDouble/State
//   yyV2NewClimate / (*yyV2Climate).Axes
//   yyV2VoronoiSHA / yyV2VoronoiAccess3D
//
// 所有内部辅助符号统一带 yyV2Rand 前缀，避免和同包其它文件撞名。
// ═══════════════════════════════════════════════════════════════════════

import "math"

// ── Java 版 java.util.Random（48 位 LCG）───────────────────────────────
//
// 这个 RNG 不是"通用随机数"：结构放置、要塞环、史莱姆区块全部靠它，
// 差一位就整张图错。JS 参考实现踩过两个坑，必须一起复刻：
//   1. nextLong() = ((long)next(32) << 32) + next(32)，两次 next(32) 都是
//      **有符号 int32**，低位为负时要借位相加，不能用位或拼。
//   2. nextInt(n) 对 2 的幂走乘法快速路径，否则拒绝采样消除取模偏差。

const yyV2RNGMask48 = (uint64(1) << 48) - 1

type yyV2RNG struct {
	s uint64 // 48 位有效
}

// yyV2NewRNG 与 JS 的 new JRandom(seed) 一致：state = (uint64(seed) ^ 0x5deece66d) & mask48
func yyV2NewRNG(seed int64) *yyV2RNG {
	return &yyV2RNG{s: (uint64(seed) ^ 0x5deece66d) & yyV2RNGMask48}
}

// yyV2NewRNGFromState 直接用已初始化的 48 位状态构造（region seed 用）。
func yyV2NewRNGFromState(state uint64) *yyV2RNG {
	return &yyV2RNG{s: state & yyV2RNGMask48}
}

// State 返回当前 48 位状态（用于需要"把 RNG 交出去"的场景，如下界要塞/堡垒分流）。
func (r *yyV2RNG) State() uint64 { return r.s }

// Next 与 Java Random.next(bits) 一致。返回 int32（bits==32 时可能是负数，
// 这正是 nextLong 需要的"有符号扩展"语义）。
func (r *yyV2RNG) Next(bits uint) int32 {
	r.s = (r.s*0x5deece66d + 0xb) & yyV2RNGMask48
	return int32(r.s >> (48 - bits))
}

// NextInt 与 Java Random.nextInt(bound) 一致。
func (r *yyV2RNG) NextInt(bound int32) int32 {
	if bound <= 0 {
		return 0
	}
	if bound&(bound-1) == 0 { // 2 的幂
		return int32((int64(bound) * int64(r.Next(31))) >> 31)
	}
	for {
		bits := r.Next(31)
		val := bits % bound
		if bits-val+(bound-1) >= 0 {
			return val
		}
	}
}

// NextLong 与 Java Random.nextLong() 一致（有符号扩展 + 相加，不是位或）。
func (r *yyV2RNG) NextLong() int64 {
	hi := int64(r.Next(32)) // int32 有符号扩展
	lo := int64(r.Next(32))
	return (hi << 32) + lo
}

// NextDouble 与 Java Random.nextDouble() 一致：
// (next(26)*2^27 + next(27)) / 2^53
func (r *yyV2RNG) NextDouble() float64 {
	return (float64(r.Next(26))*134217728.0 + float64(r.Next(27))) / 9007199254740992.0
}

// ── Xoroshiro128++（MC 的 XoroshiroRandomSource）────────────────────────

type yyV2RandXoro struct {
	lo, hi uint64
}

func yyV2RandRotl64(x uint64, b uint) uint64 {
	return (x << b) | (x >> (64 - b))
}

// yyV2RandXSetSeed 对应 noise.mjs 的 xSetSeed。
func yyV2RandXSetSeed(value uint64) yyV2RandXoro {
	const (
		xl = uint64(0x9e3779b97f4a7c15)
		xh = uint64(0x6a09e667f3bcc909)
		a  = uint64(0xbf58476d1ce4e5b9)
		b  = uint64(0x94d049bb133111eb)
	)
	l := value ^ xh
	h := l + xl
	l = (l ^ (l >> 30)) * a
	h = (h ^ (h >> 30)) * a
	l = (l ^ (l >> 27)) * b
	h = (h ^ (h >> 27)) * b
	l = l ^ (l >> 31)
	h = h ^ (h >> 31)
	return yyV2RandXoro{lo: l, hi: h}
}

// nextLong 对应 noise.mjs 的 xNextLong。
func (x *yyV2RandXoro) nextLong() uint64 {
	l, h := x.lo, x.hi
	n := yyV2RandRotl64(l+h, 17) + l
	h2 := h ^ l
	x.lo = yyV2RandRotl64(l, 49) ^ h2 ^ (h2 << 21)
	x.hi = yyV2RandRotl64(h2, 28)
	return n
}

// nextInt 对应 noise.mjs 的 xNextInt（Java nextInt(n) 语义）。
func (x *yyV2RandXoro) nextInt(n uint32) uint32 {
	bn := uint64(n)
	// ((xNextLong & M32) * bn) & M64 —— Go 的 uint64 乘法天然按 2^64 回绕
	r := (x.nextLong() & 0xffffffff) * bn
	low := uint32(r)
	if uint64(low) < bn {
		limit := (uint64(0xffffffff) - bn) % bn
		for uint64(low) < limit {
			r = (x.nextLong() & 0xffffffff) * bn
			low = uint32(r)
		}
	}
	return uint32(r >> 32)
}

// nextDouble 对应 noise.mjs 的 xNextDouble。
func (x *yyV2RandXoro) nextDouble() float64 {
	return float64(x.nextLong()>>11) * 1.1102230246251565e-16
}

// ── PerlinNoise ─────────────────────────────────────────────────────────
//
// y=0 时走缓存快路径（d2/h2/t2 在 init 时算好），气候噪声全部 y=0，
// 这条路径是热路径，别删。

type yyV2RandPerlin struct {
	d          [257]uint8 // 置换表，d[256]=d[0]
	a, b, c    float64    // x/y/z 偏移
	amplitude  float64
	lacunarity float64
	d2, t2     float64
	h2         int
}

// yyV2RandPerlinInit 对应 noise.mjs 的 xPerlinInit：
// 从 Xoroshiro 取 3 个 double 偏移 + Fisher-Yates 置换表。
func yyV2RandPerlinInit(x *yyV2RandXoro) *yyV2RandPerlin {
	p := &yyV2RandPerlin{amplitude: 1, lacunarity: 1}
	p.a = x.nextDouble() * 256.0
	p.b = x.nextDouble() * 256.0
	p.c = x.nextDouble() * 256.0
	for i := 0; i < 256; i++ {
		p.d[i] = uint8(i)
	}
	for i := 0; i < 256; i++ {
		j := int(x.nextInt(uint32(256-i))) + i
		p.d[i], p.d[j] = p.d[j], p.d[i]
	}
	p.d[256] = p.d[0]
	i2 := math.Floor(p.b)
	p.h2 = int(i2)
	p.d2 = p.b - i2
	p.t2 = p.d2 * p.d2 * p.d2 * (p.d2*(p.d2*6.0-15.0) + 10.0)
	return p
}

func yyV2RandIndexedLerp(idx int, a, b, c float64) float64 {
	switch idx & 0xf {
	case 0:
		return a + b
	case 1:
		return -a + b
	case 2:
		return a - b
	case 3:
		return -a - b
	case 4:
		return a + c
	case 5:
		return -a + c
	case 6:
		return a - c
	case 7:
		return -a - c
	case 8:
		return b + c
	case 9:
		return -b + c
	case 10:
		return b - c
	case 11:
		return -b - c
	case 12:
		return a + b
	case 13:
		return -b + c
	case 14:
		return -a + b
	case 15:
		return -b - c
	}
	return 0
}

func yyV2RandLerp(part, from, to float64) float64 { return from + part*(to-from) }

func yyV2RandSmootherstep(t float64) float64 {
	return t * t * t * (t*(t*6.0-15.0) + 10.0)
}

// yyV2RandSamplePerlin 对应 cubiomes samplePerlin。yamp/ymin 仅 3D 地形噪声使用，
// 气候噪声传 0。
func yyV2RandSamplePerlin(p *yyV2RandPerlin, x, y, z, yamp, ymin float64) float64 {
	d1, d2, d3 := x, y, z
	var h1, h2, h3 int
	var t1, t2, t3 float64

	if d2 == 0.0 {
		d2, h2, t2 = p.d2, p.h2, p.t2
	} else {
		d2 += p.b
		i2 := math.Floor(d2)
		d2 -= i2
		h2 = int(i2)
		t2 = yyV2RandSmootherstep(d2)
	}

	d1 += p.a
	d3 += p.c

	i1 := math.Floor(d1)
	i3 := math.Floor(d3)
	d1 -= i1
	d3 -= i3
	h1, h3 = int(i1), int(i3)
	t1 = yyV2RandSmootherstep(d1)
	t3 = yyV2RandSmootherstep(d3)

	if yamp != 0 {
		yclamp := d2
		if ymin < d2 {
			yclamp = ymin
		}
		d2 -= math.Floor(yclamp/yamp) * yamp
	}

	idx := &p.d
	a1 := int(idx[h1&0xff]) + h2
	b1 := int(idx[(h1+1)&0xff]) + h2
	a2 := int(idx[a1&0xff]) + h3
	b2 := int(idx[b1&0xff]) + h3
	a3 := int(idx[(a1+1)&0xff]) + h3
	b3 := int(idx[(b1+1)&0xff]) + h3

	l1 := yyV2RandIndexedLerp(int(idx[a2&0xff]), d1, d2, d3)
	l2 := yyV2RandIndexedLerp(int(idx[b2&0xff]), d1-1, d2, d3)
	l3 := yyV2RandIndexedLerp(int(idx[a3&0xff]), d1, d2-1, d3)
	l4 := yyV2RandIndexedLerp(int(idx[b3&0xff]), d1-1, d2-1, d3)
	l5 := yyV2RandIndexedLerp(int(idx[(a2+1)&0xff]), d1, d2, d3-1)
	l6 := yyV2RandIndexedLerp(int(idx[(b2+1)&0xff]), d1-1, d2, d3-1)
	l7 := yyV2RandIndexedLerp(int(idx[(a3+1)&0xff]), d1, d2-1, d3-1)
	l8 := yyV2RandIndexedLerp(int(idx[(b3+1)&0xff]), d1-1, d2-1, d3-1)

	m1 := yyV2RandLerp(t1, l1, l2)
	m3 := yyV2RandLerp(t1, l3, l4)
	m5 := yyV2RandLerp(t1, l5, l6)
	m7 := yyV2RandLerp(t1, l7, l8)
	m1 = yyV2RandLerp(t2, m1, m3)
	m5 = yyV2RandLerp(t2, m5, m7)
	return yyV2RandLerp(t3, m1, m5)
}

// ── OctaveNoise ─────────────────────────────────────────────────────────

// md5_octave_n：cubiomes 里 md5("minecraft:<axis>") 的 128 位盐，
// 下标 = 12 + omin + i（omin 为负，故范围 0..12）。
var yyV2RandMD5OctaveN = [13][2]uint64{
	{0xb198de63a8012672, 0x7b84cad43ef7b5a8},
	{0x0fd787bfbc403ec3, 0x74a4a31ca21b48b8},
	{0x36d326eed40efeb2, 0x5be9ce18223c636a},
	{0x082fe255f8be6631, 0x4e96119e22dedc81},
	{0x0ef68ec68504005e, 0x48b6bf93a2789640},
	{0xf11268128982754f, 0x257a1d670430b0aa},
	{0xe51c98ce7d1de664, 0x5f9478a733040c45},
	{0x6d7b49e7e429850a, 0x2e3063c622a24777},
	{0xbd90d5377ba1b762, 0xc07317d419a7548d},
	{0x53d39c6752dac858, 0xbcd1c5a80ab65b3e},
	{0xb4a24d7a84e7677b, 0x023ff9668e89b5c4},
	{0xdffa22b534c5f608, 0xb9b67517d3665ca9},
	{0xd50708086cef4d7c, 0x6e1651ecc7f43309},
}

// lacuna_ini 下标 = -omin（3..12）
var yyV2RandLacunaIni = [13]float64{
	1, 1.0 / 2, 1.0 / 4, 1.0 / 8, 1.0 / 16, 1.0 / 32, 1.0 / 64,
	1.0 / 128, 1.0 / 256, 1.0 / 512, 1.0 / 1024, 1.0 / 2048, 1.0 / 4096,
}

// persist_ini 下标 = len（4..9）：2^(len-1)/(2^len-1)
var yyV2RandPersistIni = [10]float64{
	0, 1, 2.0 / 3, 4.0 / 7, 8.0 / 15, 16.0 / 31, 32.0 / 63, 64.0 / 127, 128.0 / 255, 256.0 / 511,
}

// yyV2RandOctaveInit 对应 cubiomes xOctaveInit：按 amplitudes/omin/len 构造 octave 链。
func yyV2RandOctaveInit(x *yyV2RandXoro, amps []float64, omin, length, nmax int) []*yyV2RandPerlin {
	lacuna := yyV2RandLacunaIni[-omin]
	persist := yyV2RandPersistIni[length]
	xlo := x.nextLong()
	xhi := x.nextLong()

	octs := make([]*yyV2RandPerlin, 0, length)
	n := 0
	for i := 0; i < length; i++ {
		if n == nmax {
			break
		}
		if amps[i] != 0 {
			salt := yyV2RandMD5OctaveN[12+omin+i]
			sub := yyV2RandXoro{lo: xlo ^ salt[0], hi: xhi ^ salt[1]}
			p := yyV2RandPerlinInit(&sub)
			p.amplitude = amps[i] * persist
			p.lacunarity = lacuna
			octs = append(octs, p)
			n++
		}
		lacuna *= 2.0
		persist *= 0.5
	}
	return octs
}

func yyV2RandSampleOctave(octs []*yyV2RandPerlin, x, y, z float64) float64 {
	var v float64
	for _, p := range octs {
		lf := p.lacunarity
		v += p.amplitude * yyV2RandSamplePerlin(p, x*lf, y*lf, z*lf, 0, 0)
	}
	return v
}

// ── DoublePerlinNoise ───────────────────────────────────────────────────

// DP_F 是两个 octave 组的频率比（cubiomes 的 337/331）。
const yyV2RandDPF = 337.0 / 331.0

// amp_ini 下标 = trim 前导/尾随 0 之后的长度。
var yyV2RandAmpIni = [10]float64{
	0, 5.0 / 6, 10.0 / 9, 15.0 / 12, 20.0 / 15, 25.0 / 18, 30.0 / 21, 35.0 / 24, 40.0 / 27, 45.0 / 30,
}

type yyV2RandDoublePerlin struct {
	amplitude  float64
	octA, octB []*yyV2RandPerlin
}

// yyV2RandDoublePerlinInit 对应 cubiomes xDoublePerlinInit。
func yyV2RandDoublePerlinInit(x *yyV2RandXoro, amps []float64, omin, length, nmax int) *yyV2RandDoublePerlin {
	na, nb := -1, -1
	if nmax > 0 {
		na = (nmax + 1) >> 1
		nb = nmax - na
	}
	n := &yyV2RandDoublePerlin{}
	n.octA = yyV2RandOctaveInit(x, amps, omin, length, na)
	n.octB = yyV2RandOctaveInit(x, amps, omin, length, nb)

	// trim 前导/尾随 0 后的长度
	l := length
	for i := length - 1; i >= 0 && amps[i] == 0; i-- {
		l--
	}
	for i := 0; i < length && amps[i] == 0; i++ {
		l--
	}
	if l < 0 {
		l = 0
	}
	if l >= len(yyV2RandAmpIni) {
		l = len(yyV2RandAmpIni) - 1
	}
	n.amplitude = yyV2RandAmpIni[l]
	return n
}

func yyV2RandSampleDoublePerlin(n *yyV2RandDoublePerlin, x, y, z float64) float64 {
	var v float64
	v += yyV2RandSampleOctave(n.octA, x, y, z)
	v += yyV2RandSampleOctave(n.octB, x*yyV2RandDPF, y*yyV2RandDPF, z*yyV2RandDPF)
	return v * n.amplitude
}

// ── Voronoi（1.15+ 生物群系 1:4 缩放）──────────────────────────────────

// yyV2RandMcStepSeed 对应 cubiomes mcStepSeed：64 位 LCG 步进。
func yyV2RandMcStepSeed(s uint64, salt int) uint64 {
	return s*(s*6364136223846793005+1442695040888963407) + uint64(int64(salt))
}

// yyV2RandMcStepSeedU 同上，但 salt 是 uint64（getVoronoiCell 里用 sha 当盐）。
func yyV2RandMcStepSeedU(s, salt uint64) uint64 {
	return s*(s*6364136223846793005+1442695040888963407) + salt
}

var yyV2RandSHAK = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xb24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}

func yyV2RandRotr32(a uint32, b uint) uint32 { return (a >> b) | (a << (32 - b)) }

func yyV2RandBswap32(x uint32) uint32 {
	return (x&0x000000ff)<<24 | (x&0x0000ff00)<<8 | (x&0x00ff0000)>>8 | (x&0xff000000)>>24
}

// yyV2VoronoiSHA 对 worldSeed 做单块 SHA-256（消息 = 8 字节种子，大端），取前 8 字节。
// 逐行照抄 cubiomes getVoronoiSHA 的单块实现，不要换成 crypto/sha256：
// 这里只需要一个 block，且字节序处理必须和参考实现一致。
func yyV2VoronoiSHA(seed int64) uint64 {
	s := uint64(seed)
	var m [64]uint32
	m[0] = yyV2RandBswap32(uint32(s & 0xffffffff))
	m[1] = yyV2RandBswap32(uint32((s >> 32) & 0xffffffff))
	m[2] = 0x80000000
	// m[3..14] 已是 0（数组零值）
	m[15] = 0x00000040
	for i := 16; i < 64; i++ {
		m[i] = m[i-7] + m[i-16]
		x := m[i-15]
		m[i] += yyV2RandRotr32(x, 7) ^ yyV2RandRotr32(x, 18) ^ (x >> 3)
		x = m[i-2]
		m[i] += yyV2RandRotr32(x, 17) ^ yyV2RandRotr32(x, 19) ^ (x >> 10)
	}

	a0, a1, a2, a3 := uint32(0x6a09e667), uint32(0xbb67ae85), uint32(0x3c6ef372), uint32(0xa54ff53a)
	a4, a5, a6, a7 := uint32(0x510e527f), uint32(0x9b05688c), uint32(0x1f83d9ab), uint32(0x5be0cd19)
	for i := 0; i < 64; i++ {
		x := a7 + yyV2RandSHAK[i] + m[i] +
			(yyV2RandRotr32(a4, 6) ^ yyV2RandRotr32(a4, 11) ^ yyV2RandRotr32(a4, 25)) +
			((a4 & a5) ^ (^a4 & a6))
		y := (yyV2RandRotr32(a0, 2) ^ yyV2RandRotr32(a0, 13) ^ yyV2RandRotr32(a0, 22)) +
			((a0 & a1) ^ (a0 & a2) ^ (a1 & a2))
		a7, a6, a5 = a6, a5, a4
		a4 = a3 + x
		a3, a2, a1 = a2, a1, a0
		a0 = x + y
	}
	a0 += 0x6a09e667
	a1 += 0xbb67ae85
	return uint64(yyV2RandBswap32(a0)) | (uint64(yyV2RandBswap32(a1)) << 32)
}

// yyV2RandVoronoiCell 对应 cubiomes getVoronoiCell：格点内的随机偏移（1/36 方块精度）。
func yyV2RandVoronoiCell(sha uint64, a, b, c int) (int, int, int) {
	s := sha
	s = yyV2RandMcStepSeed(s, a)
	s = yyV2RandMcStepSeed(s, b)
	s = yyV2RandMcStepSeed(s, c)
	s = yyV2RandMcStepSeed(s, a)
	s = yyV2RandMcStepSeed(s, b)
	s = yyV2RandMcStepSeed(s, c)
	x := (int((s>>24)&1023) - 512) * 36
	s = yyV2RandMcStepSeedU(s, sha)
	y := (int((s>>24)&1023) - 512) * 36
	s = yyV2RandMcStepSeedU(s, sha)
	z := (int((s>>24)&1023) - 512) * 36
	return x, y, z
}

// yyV2VoronoiAccess3D 方块坐标 → 所属 1:4 气候格（格点坐标，不是方块坐标）。
// dmin 初值是"无穷大"（cubiomes 的 uint64(-1) 语义），不能用 0。
func yyV2VoronoiAccess3D(sha uint64, x, y, z int) (int, int, int) {
	x -= 2
	y -= 2
	z -= 2
	pX, pY, pZ := x>>2, y>>2, z>>2
	dx, dy, dz := (x&3)*10240, (y&3)*10240, (z&3)*10240
	ax, ay, az := 0, 0, 0
	dmin := int(^uint(0) >> 1) // MaxInt
	for i := 0; i < 8; i++ {
		bx, by, bz := 0, 0, 0
		if i&4 != 0 {
			bx = 1
		}
		if i&2 != 0 {
			by = 1
		}
		if i&1 != 0 {
			bz = 1
		}
		cx, cy, cz := pX+bx, pY+by, pZ+bz
		rx0, ry0, rz0 := yyV2RandVoronoiCell(sha, cx, cy, cz)
		rx := rx0 + dx - 40*1024*bx
		ry := ry0 + dy - 40*1024*by
		rz := rz0 + dz - 40*1024*bz
		d := rx*rx + ry*ry + rz*rz
		if d < dmin {
			dmin = d
			ax, ay, az = cx, cy, cz
		}
	}
	return ax, ay, az
}

// ── 气候（climate）六轴 ────────────────────────────────────────────────
//
// 对应 biomegen.mjs 的 CLIMATE_DEF / buildClimate / sampleClimate。
// 6 条 DoublePerlin 必须按 shift → temperature → vegetation → continentalness
// → erosion → weirdness 的顺序从**同一个** Xoroshiro 上连续消耗，
// 顺序错了整张生物群系图就错。

// 样条节点类型：**不是** NP_* 轴下标，而是 sampleClimate 里 npParam[4] 的下标
// （[continentalness, erosion, ridges, weirdness]）。别和 NP_* 混用。
const (
	yyV2RandSpCont  = 0
	yyV2RandSpEro   = 1
	yyV2RandSpRidge = 2
	yyV2RandSpWeird = 3
)

type yyV2RandClimateDef struct {
	lo, hi           uint64
	loLarge, hiLarge uint64
	hasLarge         bool
	amp              []float64
	omin             int
	ominLarge        int
	length           int
}

// yyV2RandClimateDefs 顺序 = 参考实现 CLIMATE_DEF 的插入顺序（RNG 消耗顺序）。
var yyV2RandClimateDefs = []yyV2RandClimateDef{
	{
		lo: 0x080518cf6af25384, hi: 0x3f3dfb40a54febd5,
		amp: []float64{1, 1, 1, 0}, omin: -3, length: 4,
	},
	{
		lo: 0x5c7e6b29735f0d7f, hi: 0xf7d86f1bbc734988,
		loLarge: 0x944b0073edf549db, hiLarge: 0x4ff44347e9d22b96, hasLarge: true,
		amp: []float64{1.5, 0, 1, 0, 0, 0}, omin: -10, ominLarge: -12, length: 6,
	},
	{
		lo: 0x81bb4d22e8dc168e, hi: 0xf1c8b4bea16303cd,
		loLarge: 0x71b8ab943dbd5301, hiLarge: 0xbb63ddcf39ff7a2b, hasLarge: true,
		amp: []float64{1, 1, 0, 0, 0, 0}, omin: -8, ominLarge: -10, length: 6,
	},
	{
		lo: 0x83886c9d0ae3a662, hi: 0xafa638a61b42e8ad,
		loLarge: 0x9a3f51a113fce8dc, hiLarge: 0xee2dbd157e5dcdad, hasLarge: true,
		amp: []float64{1, 1, 2, 2, 2, 1, 1, 1, 1}, omin: -9, ominLarge: -11, length: 9,
	},
	{
		lo: 0xd02491e6058f6fd8, hi: 0x4792512c94c17a80,
		loLarge: 0x8c984b1f8702a951, hiLarge: 0xead7b1f92bae535f, hasLarge: true,
		amp: []float64{1, 1, 0, 1, 1}, omin: -9, ominLarge: -11, length: 5,
	},
	{
		lo: 0xefc8ef4d36102b34, hi: 0x1beeeb324a0f24ea,
		amp: []float64{1, 2, 1, 0, 0, 0}, omin: -7, length: 6,
	},
}

// yyV2Climate 一个种子 + large 标志下的全套气候噪声。
// 构造一次反复用（渲染时是热路径，别每次采样都重建）。
type yyV2Climate struct {
	seed                                int64
	large                               bool
	shift, temperature, vegetation      *yyV2RandDoublePerlin
	continentalness, erosion, weirdness *yyV2RandDoublePerlin
}

// yyV2NewClimate 构造某个种子的全套气候噪声（shift/temperature/vegetation/
// continentalness/erosion/weirdness 六轴）。
func yyV2NewClimate(seed int64, large bool) *yyV2Climate {
	c := &yyV2Climate{seed: seed, large: large}
	xr := yyV2RandXSetSeed(uint64(seed))
	xlo := xr.nextLong()
	xhi := xr.nextLong()

	built := make([]*yyV2RandDoublePerlin, 0, len(yyV2RandClimateDefs))
	for _, d := range yyV2RandClimateDefs {
		lo, hi, omin := d.lo, d.hi, d.omin
		if large && d.hasLarge {
			lo, hi, omin = d.loLarge, d.hiLarge, d.ominLarge
		}
		sub := yyV2RandXoro{lo: xlo ^ lo, hi: xhi ^ hi}
		built = append(built, yyV2RandDoublePerlinInit(&sub, d.amp, omin, d.length, -1))
	}
	c.shift = built[0]
	c.temperature = built[1]
	c.vegetation = built[2]
	c.continentalness = built[3]
	c.erosion = built[4]
	c.weirdness = built[5]
	return c
}

// Axes 采样 6 个气候参数，顺序必须是 [temperature, vegetation, continentalness,
// erosion, depth, weirdness]，且**已乘 10000 取整**（int32 截断，同 Math.trunc）。
func (c *yyV2Climate) Axes(x, z float64) [6]int32 {
	// 注意 pz 那一行参数顺序是 (z, x, 0) —— 参考实现就是这样，别"修正"。
	px := x + yyV2RandSampleDoublePerlin(c.shift, x, 0, z)*4.0
	pz := z + yyV2RandSampleDoublePerlin(c.shift, z, x, 0)*4.0

	cont := yyV2RandSampleDoublePerlin(c.continentalness, px, 0, pz)
	ero := yyV2RandSampleDoublePerlin(c.erosion, px, 0, pz)
	w := yyV2RandSampleDoublePerlin(c.weirdness, px, 0, pz)

	npParam := [4]float64{
		cont,
		ero,
		-3.0 * (math.Abs(math.Abs(w)-0.6666667) - 0.33333334),
		w,
	}
	off := yyV2RandGetSpline(yyV2RandDepthSpline, npParam) + 0.015
	// cubiomes: y 恒为 0（不是查询高度）
	// ↳ 这里的 83 与参考实现逐位一致，别单独改（改 0 会让 d=1.0+off 顶进
	//   深暗之域的 depth 盒，实测地表图 9.5% 变深暗之域）。
	//   地表地图要的是「depth 钉在参数表地表点」，那一步在
	//   seedv2_biome.go 的 yyV2SurfaceDepthNP 里做，不在这里。
	d := 1.0 - 0 - 83.0/160.0 + off

	t := yyV2RandSampleDoublePerlin(c.temperature, px, 0, pz)
	h := yyV2RandSampleDoublePerlin(c.vegetation, px, 0, pz)

	return [6]int32{
		int32(math.Trunc(10000.0 * t)),
		int32(math.Trunc(10000.0 * h)),
		int32(math.Trunc(10000.0 * cont)),
		int32(math.Trunc(10000.0 * ero)),
		int32(math.Trunc(10000.0 * d)),
		int32(math.Trunc(10000.0 * w)),
	}
}

// ── depth 样条（MC OverworldBiomeBuilder / cubiomes initBiomeNoise）──────

// yyV2RandSpline 一个样条节点：fixed=true 时是常数叶，否则是按 typ 轴索引的插值样条。
type yyV2RandSpline struct {
	fixed bool
	v     float64
	typ   int
	loc   []float64
	val   []*yyV2RandSpline
	der   []float64
}

func yyV2RandNewSpline(typ int) *yyV2RandSpline { return &yyV2RandSpline{typ: typ} }

func yyV2RandFix(v float64) *yyV2RandSpline { return &yyV2RandSpline{fixed: true, v: v} }

func yyV2RandAddVal(sp *yyV2RandSpline, loc float64, val *yyV2RandSpline, der float64) {
	sp.loc = append(sp.loc, loc)
	sp.val = append(sp.val, val)
	sp.der = append(sp.der, der)
}

// yyV2RandGetOffsetValue 对应 biomegen.mjs getOffsetValue。
func yyV2RandGetOffsetValue(weirdness, continentalness float64) float64 {
	f0 := 1.0 - (1.0-continentalness)*0.5
	f1 := 0.5 * (1.0 - continentalness)
	f2 := (weirdness + 1.17) * 0.46082947
	off := f2*f0 - f1
	if weirdness < -0.7 {
		if off > -0.2222 {
			return off
		}
		return -0.2222
	}
	if off > 0 {
		return off
	}
	return 0
}

func yyV2RandCreateSpline38219(f float64, bl bool) *yyV2RandSpline {
	sp := yyV2RandNewSpline(yyV2RandSpRidge)
	i := yyV2RandGetOffsetValue(-1.0, f)
	k := yyV2RandGetOffsetValue(1.0, f)
	l := 1.0 - (1.0-f)*0.5
	u := 0.5 * (1.0 - f)
	l = u/(0.46082947*l) - 1.17
	if -0.65 < l && l < 1.0 {
		u = yyV2RandGetOffsetValue(-0.65, f)
		p := yyV2RandGetOffsetValue(-0.75, f)
		q := (p - i) * 4.0
		r := yyV2RandGetOffsetValue(l, f)
		s := (k - r) / (1.0 - l)
		yyV2RandAddVal(sp, -1.0, yyV2RandFix(i), q)
		yyV2RandAddVal(sp, -0.75, yyV2RandFix(p), 0)
		yyV2RandAddVal(sp, -0.65, yyV2RandFix(u), 0)
		yyV2RandAddVal(sp, l-0.01, yyV2RandFix(r), 0)
		yyV2RandAddVal(sp, l, yyV2RandFix(r), s)
		yyV2RandAddVal(sp, 1.0, yyV2RandFix(k), s)
	} else {
		u = (k - i) * 0.5
		if bl {
			first := i
			if !(i > 0.2) {
				first = 0.2
			}
			yyV2RandAddVal(sp, -1.0, yyV2RandFix(first), 0)
			yyV2RandAddVal(sp, 0.0, yyV2RandFix(yyV2RandLerp(0.5, i, k)), u)
		} else {
			yyV2RandAddVal(sp, -1.0, yyV2RandFix(i), u)
		}
		yyV2RandAddVal(sp, 1.0, yyV2RandFix(k), u)
	}
	return sp
}

func yyV2RandCreateFlatOffsetSpline(f, g, h, i, j, k float64) *yyV2RandSpline {
	sp := yyV2RandNewSpline(yyV2RandSpRidge)
	l := 0.5 * (g - f)
	if l < k {
		l = k
	}
	m := 5.0 * (h - g)
	der2 := l
	if !(l < m) {
		der2 = m
	}
	yyV2RandAddVal(sp, -1.0, yyV2RandFix(f), l)
	yyV2RandAddVal(sp, -0.4, yyV2RandFix(g), der2)
	yyV2RandAddVal(sp, 0.0, yyV2RandFix(h), m)
	yyV2RandAddVal(sp, 0.4, yyV2RandFix(i), 2.0*(i-h))
	yyV2RandAddVal(sp, 1.0, yyV2RandFix(j), 0.7*(j-i))
	return sp
}

func yyV2RandCreateLandSpline(f, g, h, i, j, k float64, bl bool) *yyV2RandSpline {
	sp1 := yyV2RandCreateSpline38219(yyV2RandLerp(i, 0.6, 1.5), bl)
	sp2 := yyV2RandCreateSpline38219(yyV2RandLerp(i, 0.6, 1.0), bl)
	sp3 := yyV2RandCreateSpline38219(i, bl)
	ih := 0.5 * i
	sp4 := yyV2RandCreateFlatOffsetSpline(f-0.15, ih, ih, ih, i*0.6, 0.5)
	sp5 := yyV2RandCreateFlatOffsetSpline(f, j*i, g*i, ih, i*0.6, 0.5)
	sp6 := yyV2RandCreateFlatOffsetSpline(f, j, j, g, h, 0.5)
	sp7 := yyV2RandCreateFlatOffsetSpline(f, j, j, g, h, 0.5)
	sp8 := yyV2RandNewSpline(yyV2RandSpRidge)
	yyV2RandAddVal(sp8, -1.0, yyV2RandFix(f), 0.0)
	yyV2RandAddVal(sp8, -0.4, sp6, 0.0)
	yyV2RandAddVal(sp8, 0.0, yyV2RandFix(h+0.07), 0.0)
	sp9 := yyV2RandCreateFlatOffsetSpline(-0.02, k, k, g, h, 0.0)
	sp := yyV2RandNewSpline(yyV2RandSpEro)
	yyV2RandAddVal(sp, -0.85, sp1, 0.0)
	yyV2RandAddVal(sp, -0.7, sp2, 0.0)
	yyV2RandAddVal(sp, -0.4, sp3, 0.0)
	yyV2RandAddVal(sp, -0.35, sp4, 0.0)
	yyV2RandAddVal(sp, -0.1, sp5, 0.0)
	yyV2RandAddVal(sp, 0.2, sp6, 0.0)
	if bl {
		yyV2RandAddVal(sp, 0.4, sp7, 0.0)
		yyV2RandAddVal(sp, 0.45, sp8, 0.0)
		yyV2RandAddVal(sp, 0.55, sp8, 0.0)
		yyV2RandAddVal(sp, 0.58, sp7, 0.0)
	}
	yyV2RandAddVal(sp, 0.7, sp9, 0.0)
	return sp
}

func yyV2RandBuildDepthSpline() *yyV2RandSpline {
	sp := yyV2RandNewSpline(yyV2RandSpCont)
	sp1 := yyV2RandCreateLandSpline(-0.15, 0.00, 0.0, 0.1, 0.00, -0.03, false)
	sp2 := yyV2RandCreateLandSpline(-0.10, 0.03, 0.1, 0.1, 0.01, -0.03, false)
	sp3 := yyV2RandCreateLandSpline(-0.10, 0.03, 0.1, 0.7, 0.01, -0.03, true)
	sp4 := yyV2RandCreateLandSpline(-0.05, 0.03, 0.1, 1.0, 0.01, 0.01, true)
	yyV2RandAddVal(sp, -1.10, yyV2RandFix(0.044), 0.0)
	yyV2RandAddVal(sp, -1.02, yyV2RandFix(-0.2222), 0.0)
	yyV2RandAddVal(sp, -0.51, yyV2RandFix(-0.2222), 0.0)
	yyV2RandAddVal(sp, -0.44, yyV2RandFix(-0.12), 0.0)
	yyV2RandAddVal(sp, -0.18, yyV2RandFix(-0.12), 0.0)
	yyV2RandAddVal(sp, -0.16, sp1, 0.0)
	yyV2RandAddVal(sp, -0.15, sp1, 0.0)
	yyV2RandAddVal(sp, -0.10, sp2, 0.0)
	yyV2RandAddVal(sp, 0.25, sp3, 0.0)
	yyV2RandAddVal(sp, 1.00, sp4, 0.0)
	return sp
}

// yyV2RandDepthSpline 全局唯一一份（样条只依赖常量表，与种子无关）。
var yyV2RandDepthSpline = yyV2RandBuildDepthSpline()

// yyV2RandGetSpline 对应 biomegen.mjs getSpline。
// vals 顺序 = [continentalness, erosion, ridges, weirdness]。
func yyV2RandGetSpline(sp *yyV2RandSpline, vals [4]float64) float64 {
	if sp == nil {
		return 0
	}
	if sp.fixed {
		return sp.v
	}
	if len(sp.loc) <= 0 {
		return 0
	}
	if len(sp.loc) == 1 {
		if sp.val[0] == nil {
			return 0
		}
		return sp.val[0].v
	}
	f := vals[sp.typ]
	i := 0
	for ; i < len(sp.loc); i++ {
		if sp.loc[i] >= f {
			break
		}
	}
	if i == 0 || i == len(sp.loc) {
		if i != 0 {
			i--
		}
		return yyV2RandGetSpline(sp.val[i], vals) + sp.der[i]*(f-sp.loc[i])
	}
	sp1, sp2 := sp.val[i-1], sp.val[i]
	g, h := sp.loc[i-1], sp.loc[i]
	k := (f - g) / (h - g)
	l, m := sp.der[i-1], sp.der[i]
	n := yyV2RandGetSpline(sp1, vals)
	o := yyV2RandGetSpline(sp2, vals)
	p := l*(h-g) - (o - n)
	q := -m*(h-g) + (o - n)
	return yyV2RandLerp(k, n, o) + k*(1.0-k)*yyV2RandLerp(k, p, q)
}
