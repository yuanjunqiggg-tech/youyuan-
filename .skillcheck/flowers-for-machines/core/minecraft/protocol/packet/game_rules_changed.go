package packet

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
)

// GameRulesChanged is sent by the server to the client to update client-side game rules, such as game rules
// like the 'showCoordinates' game rule.
type GameRulesChanged struct {
	// GameRules defines game rules changed with their respective values. The value of these game rules may be
	// either 'bool', 'int32' or 'float32'.
	// Note that some game rules are server side only, and don't necessarily need to be sent to the client.
	// Only changed game rules need to be sent in this packet. Game rules that were not changed do not need to
	// be sent if the client is already updated on them.
	GameRules []protocol.GameRule
}

// ID ...
func (*GameRulesChanged) ID() uint32 {
	return IDGameRulesChanged
}

func (pk *GameRulesChanged) Marshal(io protocol.IO) {
	// 读取方向：网易 1.21.120 自适应解析（值类型 1/2/3 + 网易定制 100 候选，
	// 以 payload 精确耗尽为判据）。失败退回原解析，行为不比现状差。
	if r, ok := io.(*protocol.Reader); ok {
		if rules, ok := protocol.ReadGameRulesAdaptive(r); ok {
			pk.GameRules = rules
			return
		}
	}
	protocol.FuncSlice(io, &pk.GameRules, io.GameRule)
}
