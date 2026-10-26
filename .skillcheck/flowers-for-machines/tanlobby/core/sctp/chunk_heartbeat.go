// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package sctp

import (
	"errors"
	"fmt"
)

/*
chunkHeartbeat represents an SCTP Chunk of type HEARTBEAT

An endpoint should send this chunk to its peer endpoint to probe the
reachability of a particular destination transport address defined in
the present association.

The parameter field contains the Heartbeat Information, which is a
variable-length opaque data structure understood only by the sender.

	 0                   1                   2                   3
	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|   Type = 4    | Chunk  Flags  |      Heartbeat Length         |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|                                                               |
	|            Heartbeat Information TLV (Variable-Length)        |
	|                                                               |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

Defined as a variable-length parameter using the format described
in Section 3.2.1, i.e.:

Variable Parameters                  Status     Type Value
-------------------------------------------------------------
heartbeat Info                       Mandatory   1
.
*/
type chunkHeartbeat struct {
	chunkHeader
	params []param
}

// Heartbeat chunk errors.
var (
	ErrChunkTypeNotHeartbeat      = errors.New("ChunkType 不是 HEARTBEAT 类型")
	ErrHeartbeatNotLongEnoughInfo = errors.New("Heartbeat 长度不足以容纳 Heartbeat Info")
	ErrParseParamTypeFailed       = errors.New("解析参数类型失败")
	ErrHeartbeatParam             = errors.New("Heartbeat 应只包含 HEARTBEAT 参数")
	ErrHeartbeatChunkUnmarshal    = errors.New("反序列化 Heartbeat Chunk 中的参数失败")
)

func (h *chunkHeartbeat) unmarshal(raw []byte) error {
	if err := h.chunkHeader.unmarshal(raw); err != nil {
		return err
	} else if h.typ != ctHeartbeat {
		return fmt.Errorf("%w：实际是 %s", ErrChunkTypeNotHeartbeat, h.typ.String())
	}

	if len(raw) <= chunkHeaderSize {
		return fmt.Errorf("%w: %d", ErrHeartbeatNotLongEnoughInfo, len(raw))
	}

	pType, err := parseParamType(raw[chunkHeaderSize:])
	if err != nil {
		return fmt.Errorf("%w: %v", ErrParseParamTypeFailed, err) //nolint:errorlint
	}
	if pType != heartbeatInfo {
		return fmt.Errorf("%w：而是 %s", ErrHeartbeatParam, pType.String())
	}

	p, err := buildParam(pType, raw[chunkHeaderSize:])
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHeartbeatChunkUnmarshal, err) //nolint:errorlint
	}
	h.params = append(h.params, p)

	return nil
}

func (h *chunkHeartbeat) Marshal() ([]byte, error) {
	return nil, ErrUnimplemented
}

func (h *chunkHeartbeat) check() (abort bool, err error) {
	return false, nil
}
