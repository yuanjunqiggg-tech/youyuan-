package service

import (
	"bytes"
	"context"
	"fmt"
	"net"

	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/encoding"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/packet"
)

// readPacketWithContext ..
func readPacketWithContext(ctx context.Context, conn net.Conn, decoder *packet.Decoder) (packet.Packet, error) {
	pkChannel := make(chan packet.Packet, 1)
	errChannel := make(chan error, 1)

	go func() {
		pk, err := readPacket(decoder)
		if err != nil {
			errChannel <- err
			return
		}
		pkChannel <- pk
	}()

	select {
	case <-ctx.Done():
		_ = conn.Close()
		return nil, fmt.Errorf("读取数据包（带上下文）：%v",ctx.Err())
	case err := <-errChannel:
		return nil, fmt.Errorf("读取数据包（带上下文）：%v",err)
	case pk := <-pkChannel:
		return pk, nil
	}
}

// readPacket ..
func readPacket(decoder *packet.Decoder) (pk packet.Packet, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("读取数据包：%v",r)
		}
	}()

	pkData, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("读取数据包：%v",err)
	}

	buf := bytes.NewBuffer(pkData)
	reader := encoding.NewReader(buf)

	header := packet.Header{}
	err = header.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("读取数据包：%v",err)
	}

	pk = packet.NewServerPool()[header.PacketID]
	if pk == nil {
		return nil, fmt.Errorf("读取数据包：不支持的数据包 %d；载荷 = %#v", header.PacketID, buf.Bytes())
	}

	pk.Marshal(reader)
	return pk, nil
}

// writePacket ..
func writePacket(encoder *packet.Encoder, pk packet.Packet) error {
	if pk == nil {
		return nil
	}

	buf := bytes.NewBuffer(nil)
	writer := encoding.NewWriter(buf, 0)
	pk.Marshal(writer)

	header := packet.Header{PacketID: pk.ID()}
	headerBuf := bytes.NewBuffer(nil)
	if err := header.Write(headerBuf); err != nil {
		return fmt.Errorf("写入数据包：%v", err)
	}

	err := encoder.Encode(append(headerBuf.Bytes(), buf.Bytes()...))
	if err != nil {
		return fmt.Errorf("写入数据包：%v", err)
	}

	return nil
}
