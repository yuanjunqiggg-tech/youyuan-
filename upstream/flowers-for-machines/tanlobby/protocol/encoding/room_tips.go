package encoding

// RoomTips ..
type RoomTips struct {
	LevelID            string
	GameType           uint8
	ConstantTestString string // Always is `Test`
	Vioce              int16
	ProtocolID         uint8
	GameVersion        string // 房主游戏版本，如 "1.21.120.0"；客户端据此校验"和房主版本不一致"
}
