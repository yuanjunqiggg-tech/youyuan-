// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package datachannel

import "errors"

var (
	// ErrDataChannelMessageTooShort means that the data isn't long enough to be a valid DataChannel message.
	ErrDataChannelMessageTooShort = errors.New("DataChannel 消息长度不足以判断其类型")

	// ErrInvalidPayloadProtocolIdentifier means that we got a DataChannel messages with a Payload Protocol Identifier
	// we don't know how to handle.
	ErrInvalidPayloadProtocolIdentifier = errors.New(
		"DataChannel 消息的 Payload 协议标识符是我们无法处理的值",
	)

	// ErrInvalidChannelType means that the remote requested a channel type that we don't support.
	ErrInvalidChannelType = errors.New("无效的 Channel 类型")

	// ErrInvalidMessageType is returned when a DataChannel Message has a type we don't support.
	ErrInvalidMessageType = errors.New("无效的 Message 类型")

	// ErrExpectedAndActualLengthMismatch is when the declared length and actual length don't match.
	ErrExpectedAndActualLengthMismatch = errors.New("声明的长度与实际长度不匹配")

	// ErrUnexpectedDataChannelType is when a message type does not match the expected type.
	ErrUnexpectedDataChannelType = errors.New("期望的消息类型与实际消息类型不匹配")
)
