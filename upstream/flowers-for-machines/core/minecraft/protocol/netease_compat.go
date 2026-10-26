package protocol

// netease_compat.go —— 网易租赁服 1.21.120 协议自适应兼容层。
//
// 背景：本地协议库（819/1.21.90 时代的 Netease 适配）连 1.21.120 租赁服时，
// BiomeDefinitionList 的 Netease 定制段（Rain 与 Tags 之间的 Dimension+Vanilla）
// 与 GameRulesChanged 的值类型可能已变更，逐字段猜解不可靠。
//
// 方法：对同一 payload 按多个候选布局（Netease 段的不同形态、GameRule 值类型
// 的不同分支）逐一重放解析。只接受精确耗尽 payload 且通过边界校验的候选，
// 避免提前越界或留下残余字节；这只是结构筛选，不证明候选字段语义正确。
//
// shortcut: 多候选只做结构兼容；获得确定版本的原生布局和真实回包后应固定对应解码器。

import (
	"bytes"
	"io"
)

// Rest returns a snapshot of the unread bytes in the underlying buffer without
// consuming them. Only available when the Reader is backed by a *bytes.Buffer.
func (r *Reader) Rest() []byte {
	if b, ok := r.r.(*bytes.Buffer); ok {
		return append([]byte(nil), b.Bytes()...)
	}
	return nil
}

// BufferedLen returns the number of unread bytes in the underlying buffer, or -1
// when the reader is not backed by a *bytes.Buffer.
func (r *Reader) BufferedLen() int {
	if b, ok := r.r.(*bytes.Buffer); ok {
		return b.Len()
	}
	return -1
}

// Skip consumes exactly n bytes from the underlying buffer without interpreting
// them. It reports whether the skip succeeded.
func (r *Reader) Skip(n int) bool {
	if n <= 0 {
		return true
	}
	if b, ok := r.r.(*bytes.Buffer); ok {
		if b.Len() < n {
			return false
		}
		b.Next(n)
		return true
	}
	if _, ok := r.r.(io.Reader); ok {
		if _, err := io.CopyN(io.Discard, r.r, int64(n)); err != nil {
			return false
		}
		return true
	}
	return false
}

// biomeNeteaseSegment parses the Netease-specific segment that sits between
// Rain and Tags inside a BiomeDefinition. Variants differ per server version.
type biomeNeteaseSegment func(io IO, d *BiomeDefinition)

// segNeteaseFull is the layout this fork historically used on Netease rental
// servers: int32 Dimension followed by a plain string Vanilla.
func segNeteaseFull(io IO, d *BiomeDefinition) {
	io.Int32(&d.Dimension)
	io.String(&d.Vanilla)
}

// segVanillaOnly is the same but without the Dimension int32.
func segVanillaOnly(io IO, d *BiomeDefinition) {
	io.String(&d.Vanilla)
}

// segNone matches the vanilla (non-Netease) v800/v827 layout.
func segNone(io IO, d *BiomeDefinition) {}

// segDimensionOnly keeps only the Dimension int32.
func segDimensionOnly(io IO, d *BiomeDefinition) {
	io.Int32(&d.Dimension)
}

// tryBiomeDefinitionList replays the BiomeDefinitionList payload with the given
// Netease segment. It succeeds only when the whole payload is consumed exactly.
func tryBiomeDefinitionList(full []byte, shieldID int32, limits bool, seg biomeNeteaseSegment) (defs []BiomeDefinition, strs []string, ok bool) {
	r := NewReader(bytes.NewBuffer(full), shieldID, limits)
	defer func() {
		if recovered := recover(); recovered != nil {
			defs, strs, ok = nil, nil, false
		}
	}()
	var count uint32
	r.Varuint32(&count)
	if count > 8192 {
		return nil, nil, false
	}
	for i := uint32(0); i < count; i++ {
		var d BiomeDefinition
		r.Int16(&d.NameIndex)
		OptionalFunc(r, &d.BiomeID, r.Uint16)
		r.Float32(&d.Temperature)
		r.Float32(&d.Downfall)
		r.Float32(&d.RedSporeDensity)
		r.Float32(&d.BlueSporeDensity)
		r.Float32(&d.AshDensity)
		r.Float32(&d.WhiteAshDensity)
		r.Float32(&d.Depth)
		r.Float32(&d.Scale)
		r.Int32(&d.MapWaterColour)
		r.Bool(&d.Rain)
		if seg != nil {
			seg(r, &d)
		}
		OptionalFunc(r, &d.Tags, func(s *[]uint16) {
			FuncSlice(r, s, r.Uint16)
		})
		OptionalMarshaler(r, &d.ChunkGeneration)
		defs = append(defs, d)
	}
	var strCount uint32
	r.Varuint32(&strCount)
	if strCount > 1<<20 {
		return nil, nil, false
	}
	for i := uint32(0); i < strCount; i++ {
		var s string
		r.String(&s)
		strs = append(strs, s)
	}
	if r.BufferedLen() != 0 {
		return nil, nil, false
	}
	return defs, strs, true
}

// ReadBiomeDefinitionListAdaptive tries the known candidate layouts in order and
// returns the first that consumes the payload exactly. ok=false means none of
// the candidates matched; the caller keeps the legacy behaviour (panic path is
// recovered upstream, the packet is skipped without dropping the connection).
func ReadBiomeDefinitionListAdaptive(r *Reader) (defs []BiomeDefinition, strs []string, ok bool) {
	full := r.Rest()
	if full == nil {
		return nil, nil, false
	}
	// 常见形态优先：历史 Netease 全段 → 仅 string → 原版无 Netease → 仅 Dimension。
	segments := []biomeNeteaseSegment{segNeteaseFull, segVanillaOnly, nil, segDimensionOnly}
		for _, seg := range segments {
			if defs, strs, ok = tryBiomeDefinitionList(full, r.shieldID, r.limitsEnabled, seg); ok {
				// 精确耗尽判据保证成功候选吃掉了 full 的全部字节；自适应解析
				// 走的是字节快照，此处把原 payload 同步消费掉，否则上层
				// packetData.decode 的 trailing 检查会把成功解析误判为失败。
				r.Skip(len(full))
				return defs, strs, true
			}
		}
	return nil, nil, false
}

// tryGameRules replays a GameRulesChanged payload. candidateNetease100 adds a
// Netease-specific value type 100 (varuint32 payload) next to the vanilla
// types 1/2/3.
func tryGameRules(full []byte, shieldID int32, limits bool, netease100, fixedInts bool) (rules []GameRule, ok bool) {
	r := NewReader(bytes.NewBuffer(full), shieldID, limits)
	defer func() {
		if recovered := recover(); recovered != nil {
			rules, ok = nil, false
		}
	}()
	var count uint32
	r.Varuint32(&count)
	if count > 4096 {
		return nil, false
	}
	for i := uint32(0); i < count; i++ {
		var g GameRule
		r.String(&g.Name)
		r.Bool(&g.CanBeModifiedByPlayer)
		var t uint32
		r.Varuint32(&t)
		switch t {
		case 1:
			var v bool
			r.Bool(&v)
			g.Value = v
			rules = append(rules, g)
		case 2:
			var v int32
			if fixedInts {
				r.Int32(&v)
			} else {
				r.Varint32(&v)
			}
			g.Value = uint32(v)
			rules = append(rules, g)
		case 3:
			var v float32
			r.Float32(&v)
			g.Value = v
			rules = append(rules, g)
		default:
			if netease100 && t == 100 {
				// 网易定制值类型 100：varuint32 载荷（候选布局）。
				var v uint32
				r.Varuint32(&v)
				g.Value = v
				rules = append(rules, g)
				continue
			}
			return nil, false
		}
	}
	if r.BufferedLen() != 0 {
		return nil, false
	}
	return rules, true
}

// ReadGameRulesAdaptive tries the vanilla value types first and the Netease
// type-100 variant as a fallback, again accepting only an exact consumption.
func ReadGameRulesAdaptive(r *Reader) (rules []GameRule, ok bool) {
	full := r.Rest()
	if full == nil {
		return nil, false
	}
	if rules, ok = tryGameRules(full, r.shieldID, r.limitsEnabled, false, false); ok {
		r.Skip(len(full))
		return rules, true
	}
	// The actual rental-server frame uses fixed little-endian int32 values.
	// Reading them as varints makes the next rule's 'd' look like type 100.
	if rules, ok = tryGameRules(full, r.shieldID, r.limitsEnabled, false, true); ok {
		r.Skip(len(full))
		return rules, true
	}
	if rules, ok = tryGameRules(full, r.shieldID, r.limitsEnabled, true, false); ok {
		r.Skip(len(full))
		return rules, true
	}
	return nil, false
}
