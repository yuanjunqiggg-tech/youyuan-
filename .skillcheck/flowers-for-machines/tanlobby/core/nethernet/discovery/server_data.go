package discovery

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type ServerData struct {
	Version        uint8
	ServerName     string
	LevelName      string
	GameType       int32
	PlayerCount    int32
	MaxPlayerCount int32
	EditorWorld    bool
	Hardcore       bool
	TransportLayer int32
}

func (d *ServerData) MarshalBinary() ([]byte, error) {
	buf := &bytes.Buffer{}

	_ = binary.Write(buf, binary.LittleEndian, d.Version)
	writeBytes[uint8](buf, []byte(d.ServerName))
	writeBytes[uint8](buf, []byte(d.LevelName))
	_ = binary.Write(buf, binary.LittleEndian, d.GameType)
	_ = binary.Write(buf, binary.LittleEndian, d.PlayerCount)
	_ = binary.Write(buf, binary.LittleEndian, d.MaxPlayerCount)
	_ = binary.Write(buf, binary.LittleEndian, d.EditorWorld)
	_ = binary.Write(buf, binary.LittleEndian, d.Hardcore)
	_ = binary.Write(buf, binary.LittleEndian, d.TransportLayer)

	return buf.Bytes(), nil
}

func (d *ServerData) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)

	if err := binary.Read(buf, binary.LittleEndian, &d.Version); err != nil {
		return fmt.Errorf("读取版本失败：%w", err)
	}
	serverName, err := readBytes[uint8](buf)
	if err != nil {
		return fmt.Errorf("读取服务器名称失败：%w", err)
	}
	d.ServerName = string(serverName)
	levelName, err := readBytes[uint8](buf)
	if err != nil {
		return fmt.Errorf("读取关卡名称失败：%w", err)
	}
	d.LevelName = string(levelName)
	if err := binary.Read(buf, binary.LittleEndian, &d.GameType); err != nil {
		return fmt.Errorf("读取游戏类型失败：%w", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &d.PlayerCount); err != nil {
		return fmt.Errorf("读取玩家数失败：%w", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &d.MaxPlayerCount); err != nil {
		return fmt.Errorf("读取最大玩家数失败：%w", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &d.EditorWorld); err != nil {
		return fmt.Errorf("读取编辑器世界失败：%w", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &d.Hardcore); err != nil {
		return fmt.Errorf("读取硬核模式失败：%w", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &d.TransportLayer); err != nil {
		return fmt.Errorf("读取传输层失败：%w", err)
	}

	return nil
}
