package minecraft

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// packetData holds the data of a Minecraft packet.
type packetData struct {
	h       *packet.Header
	full    []byte
	payload *bytes.Buffer
}

// parseData parses the packet data slice passed into a packetData struct.
func parseData(data []byte, conn *Conn) (*packetData, error) {
	buf := bytes.NewBuffer(data)
	header := &packet.Header{}
	if err := header.Read(buf); err != nil {
		conn.receiveAudit.headerErrors.Add(1)
		// We don't return this as an error as it's not in the hand of the user to control this. Instead,
		// we return to reading a new packet.
		return nil, fmt.Errorf("读取数据包头失败：%w", err)
	}
	_, known := conn.pool[header.PacketID]
	conn.receiveAudit.parsed(header.PacketID, buf.Len(), known)
	if conn.packetFunc != nil {
		// The packet func was set, so we call it.
		conn.packetFunc(*header, buf.Bytes(), conn.RemoteAddr(), conn.LocalAddr())
	}
	return &packetData{h: header, full: data, payload: buf}, nil
}

type unknownPacketError struct {
	id uint32
}

func (err unknownPacketError) Error() string {
	return fmt.Sprintf("unexpected packet (ID=%v)", err.id)
}

// decode decodes the packet payload held in the packetData and returns the packet.Packet decoded.
func (p *packetData) decode(conn *Conn) (pks []packet.Packet, err error) {
	decodeFailed := false
	defer func() {
		conn.receiveAudit.entry(p.h.PacketID).returnedPackets.Add(uint64(len(pks)))
		if decodeFailed || err != nil {
			conn.receiveAudit.entry(p.h.PacketID).decodeErrors.Add(1)
		}
	}()
	// Attempt to fetch the packet with the right packet ID from the pool.
	pkFunc, ok := conn.pool[p.h.PacketID]
	var pk packet.Packet
	if !ok {
		// No packet with the ID. This may be a custom packet of some sorts.
		pk = &packet.Unknown{PacketID: p.h.PacketID}
		if conn.disconnectOnUnknownPacket {
			_ = conn.Close()
			return nil, unknownPacketError{id: p.h.PacketID}
		}
	} else {
		pk = pkFunc()
	}

// PhoenixBuilder specific defer func.
// Author: LNSSPsd, Happy2018new
defer func() {
    if recoveredErr := recover(); recoveredErr != nil {
        conn.receiveAudit.entry(p.h.PacketID).decodePanics.Add(1)
        err = fmt.Errorf("解码数据包 %T 失败：%v", pk, recoveredErr)
        conn.log.Warn("数据包解码失败", "packet", fmt.Sprintf("%T", pk), "error", err)
        // 协议诊断：每种包前 2 次解码 panic 时 dump 原始字节（header+payload）。
        // 用于定位网易租赁服 GameRulesChanged(gamerule type=100)/BiomeDefinitionList
        // 的字段错位。payload 仅含房间/世界元数据，不含账号敏感数据。
        if conn.receiveAudit.entry(p.h.PacketID).decodePanics.Load() <= 2 {
            conn.log.Warn("解码失败原始字节", "packet", fmt.Sprintf("%T", pk),
                "len", len(p.full), "hex", hex.EncodeToString(p.full))
        }
        // fmt.Printf("%v\n", err)
    }
		/*
			if recoveredErr := recover(); recoveredErr != nil {
				err = fmt.Errorf("解码数据包 %T 失败：%w", pk, recoveredErr.(error))
			}
			if err != nil && !errors.Is(err, unknownPacketError{}) && conn.disconnectOnInvalidPacket {
				_ = conn.Close()
			}
		*/
	}()

	r := conn.proto.NewReader(p.payload, conn.shieldID.Load(), conn.readerLimits)
	pk.Marshal(r)
	conn.receiveAudit.entry(p.h.PacketID).decodedFrames.Add(1)
	if p.payload.Len() != 0 {
		conn.receiveAudit.entry(p.h.PacketID).trailingFrames.Add(1)
		decodeFailed = true
		err = fmt.Errorf("解码数据包 %T 失败：剩余 %v 字节未读取：0x%x", pk, p.payload.Len(), p.payload.Bytes())
	}
	if conn.disconnectOnInvalidPacket && err != nil {
		return nil, err
	}

	// PhoenixBuilder specific.
	// Author: LNSSPsd, Happy2018new
	{
		return conn.proto.ConvertToLatest(pk, conn), nil
		// return conn.proto.ConvertToLatest(pk, conn), err
	}
}
