package packet

import "github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"

// BiomeDefinitionList is sent by the server to let the client know all biomes that are available and
// implemented on the server side. When enabled, it also includes information for the client to
// accurately recreate the server-side generation in vanilla worlds/servers for increased performance.
type BiomeDefinitionList struct {
	// BiomeDefinitions is a list of biomes that are available on the server.
	BiomeDefinitions []protocol.BiomeDefinition
	// StringList is a makeshift dictionary implementation Mojang created to try and reduce the size of the
	// overall packet. It is a list of common strings that are used in the biome definitions, such as
	// biome names, float values or query expressions.
	StringList []string
}

// ID ...
func (*BiomeDefinitionList) ID() uint32 {
	return IDBiomeDefinitionList
}

func (pk *BiomeDefinitionList) Marshal(io protocol.IO) {
	// 读取方向：优先走网易 1.21.120 自适应解析（多候选布局 + payload 精确
	// 耗尽判据）。全部候选失配时退回原解析（panic 由上层 recover 容错，
	// 连接保持，包被跳过）——行为不比现状差，成功则真正修复 StartGame 序列。
	if r, ok := io.(*protocol.Reader); ok {
		if defs, strs, ok := protocol.ReadBiomeDefinitionListAdaptive(r); ok {
			pk.BiomeDefinitions = defs
			pk.StringList = strs
			return
		}
	}
	protocol.Slice(io, &pk.BiomeDefinitions)
	protocol.FuncSlice(io, &pk.StringList, io.String)
}
