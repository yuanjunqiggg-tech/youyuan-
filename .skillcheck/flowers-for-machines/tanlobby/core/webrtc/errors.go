// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"errors"
)

var (
	// ErrUnknownType indicates an error with Unknown info.
	ErrUnknownType = errors.New("未知")

	// ErrConnectionClosed indicates an operation executed after connection
	// has already been closed.
	ErrConnectionClosed = errors.New("连接已关闭")

	// ErrDataChannelNotOpen indicates an operation executed when the data
	// channel is not (yet) open.
	ErrDataChannelNotOpen = errors.New("数据通道未打开")

	// ErrCertificateExpired indicates that an x509 certificate has expired.
	ErrCertificateExpired = errors.New("x509 证书已过期")

	// ErrNoTurnCredentials indicates that a TURN server URL was provided
	// without required credentials.
	ErrNoTurnCredentials = errors.New("需要 TURN 服务器凭据")

	// ErrTurnCredentials indicates that provided TURN credentials are partial
	// or malformed.
	ErrTurnCredentials = errors.New("无效的 TURN 服务器凭据")

	// ErrExistingTrack indicates that a 轨道已存在.
	ErrExistingTrack = errors.New("轨道已存在")

	// ErrPrivateKeyType indicates that a particular private key encryption
	// chosen to generate a certificate is not supported.
	ErrPrivateKeyType = errors.New("不支持的私钥类型")

	// ErrModifyingPeerIdentity indicates that an attempt to modify
	// PeerIdentity was made after PeerConnection has been initialized.
	ErrModifyingPeerIdentity = errors.New("无法修改 peerIdentity")

	// ErrModifyingCertificates indicates that an attempt to modify
	// Certificates was made after PeerConnection has been initialized.
	ErrModifyingCertificates = errors.New("无法修改证书")

	// ErrModifyingBundlePolicy indicates that an attempt to modify
	// BundlePolicy was made after PeerConnection has been initialized.
	ErrModifyingBundlePolicy = errors.New("无法修改 bundle 策略")

	// ErrModifyingRTCPMuxPolicy indicates that an attempt to modify
	// RTCPMuxPolicy was made after PeerConnection has been initialized.
	ErrModifyingRTCPMuxPolicy = errors.New("无法修改 RTCP mux 策略")

	// ErrModifyingICECandidatePoolSize indicates that an attempt to modify
	// ICECandidatePoolSize was made after PeerConnection has been initialized.
	ErrModifyingICECandidatePoolSize = errors.New("无法修改 ICE 候选池大小")

	// ErrStringSizeLimit indicates that the character size limit of string is
	// exceeded. The limit is hardcoded to 65535 according to specifications.
	ErrStringSizeLimit = errors.New("数据通道标签超出大小限制")

	// ErrMaxDataChannelID indicates that the maximum number ID that could be
	// specified for a data channel has been exceeded.
	ErrMaxDataChannelID = errors.New("已指定数据通道的最大 ID 数量")

	// ErrNegotiatedWithoutID indicates that an attempt to create a data channel
	// was made while setting the negotiated option to true without providing
	// the negotiated channel ID.
	ErrNegotiatedWithoutID = errors.New("未设置 channel id 却已协商")

	// ErrRetransmitsOrPacketLifeTime indicates that an attempt to create a data
	// channel was made with both options MaxPacketLifeTime and MaxRetransmits
	// set together. Such configuration is not supported by the specification
	// and is mutually exclusive.
	ErrRetransmitsOrPacketLifeTime = errors.New("同时设置了 MaxPacketLifeTime 与 MaxRetransmits")

	// ErrCodecNotFound is returned when a codec search to the Media Engine fails.
	ErrCodecNotFound = errors.New("未找到编解码器")

	// ErrNoRemoteDescription indicates that an operation was rejected because
	// the 未设置远端描述.
	ErrNoRemoteDescription = errors.New("未设置远端描述")

	// ErrIncorrectSDPSemantics indicates that the PeerConnection was configured to
	// generate SDP Answers with different SDP Semantics than the received Offer.
	ErrIncorrectSDPSemantics = errors.New("远端 SessionDescription 语义与配置不匹配")

	// ErrIncorrectSignalingState indicates that the signaling state of PeerConnection is not correct.
	ErrIncorrectSignalingState = errors.New("当前信令状态下无法执行该操作")

	// ErrProtocolTooLarge indicates that value given for a DataChannelInit protocol is
	// longer then 65535 bytes.
	ErrProtocolTooLarge = errors.New("协议数据大于 65535 字节")

	// ErrSenderNotCreatedByConnection indicates RemoveTrack was called with a RtpSender not created
	// by this PeerConnection.
	ErrSenderNotCreatedByConnection = errors.New("RtpSender 不是由当前 PeerConnection 创建的")

	// ErrSessionDescriptionNoFingerprint indicates SetRemoteDescription was called with a SessionDescription that has no
	// fingerprint.
	ErrSessionDescriptionNoFingerprint = errors.New("调用 SetRemoteDescription 时缺少 fingerprint")

	// ErrSessionDescriptionInvalidFingerprint indicates SetRemoteDescription was called with a SessionDescription that
	// has an invalid fingerprint.
	ErrSessionDescriptionInvalidFingerprint = errors.New("调用 SetRemoteDescription 时 fingerprint 无效")

	// ErrSessionDescriptionConflictingFingerprints indicates SetRemoteDescription was called with a SessionDescription
	// that has an conflicting fingerprints.
	ErrSessionDescriptionConflictingFingerprints = errors.New(
		"调用 SetRemoteDescription 时存在多个冲突的 fingerprint",
	)

	// ErrSessionDescriptionMissingIceUfrag indicates SetRemoteDescription was called with a SessionDescription that
	// is missing an ice-ufrag value.
	ErrSessionDescriptionMissingIceUfrag = errors.New("调用 SetRemoteDescription 时缺少 ice-ufrag")

	// ErrSessionDescriptionMissingIcePwd indicates SetRemoteDescription was called with a SessionDescription that
	// is missing an ice-pwd value.
	ErrSessionDescriptionMissingIcePwd = errors.New("调用 SetRemoteDescription 时缺少 ice-pwd")

	// ErrSessionDescriptionConflictingIceUfrag  indicates SetRemoteDescription was called with a SessionDescription
	// that contains multiple conflicting ice-ufrag values.
	ErrSessionDescriptionConflictingIceUfrag = errors.New(
		"调用 SetRemoteDescription 时存在多个冲突的 ice-ufrag 值",
	)

	// ErrSessionDescriptionConflictingIcePwd indicates SetRemoteDescription was called with a SessionDescription
	// that contains multiple conflicting ice-pwd values.
	ErrSessionDescriptionConflictingIcePwd = errors.New(
		"调用 SetRemoteDescription 时存在多个冲突的 ice-pwd 值",
	)

	// ErrNoSRTPProtectionProfile indicates that the DTLS handshake completed and no SRTP Protection Profile was chosen.
	ErrNoSRTPProtectionProfile = errors.New("DTLS 握手已完成，但未选择 SRTP 保护配置")

	// ErrFailedToGenerateCertificateFingerprint indicates that we failed to generate the fingerprint
	// used for comparing certificates.
	ErrFailedToGenerateCertificateFingerprint = errors.New("生成证书指纹失败")

	// ErrNoCodecsAvailable indicates that operation isn't possible because the MediaEngine has no codecs available.
	ErrNoCodecsAvailable = errors.New("操作失败，没有可用的编解码器")

	// ErrUnsupportedCodec indicates the remote peer doesn't support the requested codec.
	ErrUnsupportedCodec = errors.New("无法启动轨道，远端不支持该编解码器")

	// ErrSenderWithNoCodecs indicates that a RTPSender was created without any codecs. To send media the MediaEngine
	//  needs at least one configured codec.
	ErrSenderWithNoCodecs = errors.New("无法填充媒体段，RTPSender 创建时没有编解码器")

	// ErrCodecAlreadyRegistered indicates that a codec has already been registered for the same payload type.
	ErrCodecAlreadyRegistered = errors.New("相同的负载类型已注册过编解码器")

	// ErrRTPSenderNewTrackHasIncorrectKind indicates that the new track is of a different kind than the previous/original.
	ErrRTPSenderNewTrackHasIncorrectKind = errors.New("新轨道必须与之前轨道的类型相同")

	// ErrRTPSenderNewTrackHasIncorrectEnvelope indicates that the new track has a different envelope
	//  than the previous/original.
	ErrRTPSenderNewTrackHasIncorrectEnvelope = errors.New("新轨道必须与之前轨道的封套相同")

	// ErrUnbindFailed indicates that a TrackLocal was not able to be unbind.
	ErrUnbindFailed = errors.New("将 TrackLocal 从 PeerConnection 解绑失败")

	// ErrNoPayloaderForCodec indicates that 请求的编解码器没有 payloader.
	ErrNoPayloaderForCodec = errors.New("请求的编解码器没有 payloader")

	// ErrRegisterHeaderExtensionInvalidDirection indicates that a extension was
	// registered with a direction besides `sendonly` or `recvonly`.
	ErrRegisterHeaderExtensionInvalidDirection = errors.New(
		"头部扩展必须注册为 'recvonly'、'sendonly' 或两者",
	)

	// ErrSimulcastProbeOverflow indicates that too many Simulcast probe streams are in flight
	// and the requested SSRC was ignored.
	ErrSimulcastProbeOverflow = errors.New("已达到 simulcast 探测上限，新的 SSRC 已被丢弃")

	errDetachNotEnabled                 = errors.New("通过调用 webrtc.DetachDataChannels() 启用分离")
	errDetachBeforeOpened               = errors.New("数据通道尚未打开，请尝试在 OnOpen 中调用 Detach")
	errDtlsTransportNotStarted          = errors.New("DTLS 传输尚未启动")
	errDtlsKeyExtractionFailed          = errors.New("从 DTLS 中提取 SRTP 密钥失败")
	errFailedToStartSRTP                = errors.New("启动 SRTP 失败")
	errFailedToStartSRTCP               = errors.New("启动 SRTCP 失败")
	errInvalidDTLSStart                 = errors.New("尝试启动不在 new 状态的 DTLSTransport")
	errNoRemoteCertificate              = errors.New("对端未通过 DTLS 提供证书")
	errIdentityProviderNotImplemented   = errors.New("身份提供者尚未实现")
	errNoMatchingCertificateFingerprint = errors.New("远端证书与任何 fingerprint 都不匹配")

	errICEConnectionNotStarted        = errors.New("ICE 连接尚未启动")
	errICECandidateTypeUnknown        = errors.New("未知的候选类型")
	errICEInvalidConvertCandidateType = errors.New(
		"无法将 ice.CandidateType 转换为 webrtc.ICECandidateType：类型无效",
	)
	errICEAgentNotExist            = errors.New("ICEAgent 不存在")
	errICECandiatesCoversionFailed = errors.New("无法将 ICE 候选转换为 ICECandidates")
	errICERoleUnknown              = errors.New("未知的 ICE 角色")
	errICEProtocolUnknown          = errors.New("未知的协议")
	errICEGathererNotStarted       = errors.New("收集器尚未启动")

	errNetworkTypeUnknown = errors.New("未知的网络类型")

	errSDPDoesNotMatchOffer        = errors.New("新的 SDP 与之前的 offer 不匹配")
	errSDPDoesNotMatchAnswer       = errors.New("新的 SDP 与之前的应答不匹配")
	errPeerConnSDPTypeInvalidValue = errors.New(
		"提供的值不是有效的 SDPType 枚举值",
	)
	errPeerConnStateChangeInvalid                     = errors.New("无效的状态变更操作")
	errPeerConnStateChangeUnhandled                   = errors.New("未处理的状态变更操作")
	errPeerConnSDPTypeInvalidValueSetLocalDescription = errors.New("提供给 SetLocalDescription() 的 SDP 类型无效")
	errPeerConnRemoteDescriptionWithoutMidValue       = errors.New(
		"远端描述中的媒体段缺少 mid 值",
	)
	errPeerConnRemoteDescriptionNil                  = errors.New("尚未设置远端描述")
	errMediaSectionHasExplictSSRCAttribute           = errors.New("媒体段含有显式 SSRC")
	errPeerConnRemoteSSRCAddTransceiver              = errors.New("无法为远端 SSRC 添加收发器")
	errPeerConnSimulcastMidRTPExtensionRequired      = errors.New("Simulcast 需要 mid RTP 扩展")
	errPeerConnSimulcastStreamIDRTPExtensionRequired = errors.New("Simulcast 需要 stream id RTP 扩展")
	errPeerConnSimulcastIncomingSSRCFailed           = errors.New("入站 SSRC 的 Simulcast 探测失败")
	errPeerConnAddTransceiverFromKindOnlyAcceptsOne  = errors.New(
		"AddTransceiverFromKind 仅接受一个 RTPTransceiverInit",
	)
	errPeerConnAddTransceiverFromTrackOnlyAcceptsOne = errors.New(
		"AddTransceiverFromTrack 仅接受一个 RTPTransceiverInit",
	)
	errPeerConnAddTransceiverFromKindSupport = errors.New(
		"AddTransceiverFromKind 当前仅支持 recvonly",
	)
	errPeerConnAddTransceiverFromTrackSupport = errors.New(
		"AddTransceiverFromTrack 当前仅支持 sendonly 和 sendrecv",
	)
	errPeerConnSetIdentityProviderNotImplemented = errors.New("TODO：SetIdentityProvider 尚未实现")
	errPeerConnWriteRTCPOpenWriteStream          = errors.New("WriteRTCP 打开 WriteStream 失败")
	errPeerConnTranscieverMidNil                 = errors.New("找不到与 mid 匹配的收发器")
	errPeerConnEarlyMediaWithoutAnswer           = errors.New(
		"没有 SDP 应答，无法处理早期媒体，" +
			"use SettingEngine.SetHandleUndeclaredSSRCWithoutAnswer(true) to process without answer",
	)

	errRTPReceiverDTLSTransportNil            = errors.New("DTLSTransport 不能为 nil")
	errRTPReceiverReceiveAlreadyCalled        = errors.New("已调用过 Receive")
	errRTPReceiverWithSSRCTrackStreamNotFound = errors.New("无法为具有 SSRC 的轨道找到流")
	errRTPReceiverForRIDTrackStreamNotFound   = errors.New("未找到该 RID 的轨道流")

	errRTPSenderTrackNil             = errors.New("Track 不能为 nil")
	errRTPSenderDTLSTransportNil     = errors.New("DTLSTransport 不能为 nil")
	errRTPSenderSendAlreadyCalled    = errors.New("已调用过 Send")
	errRTPSenderSendNotCalled        = errors.New("尚未调用 Send")
	errRTPSenderStopped              = errors.New("Sender 已停止")
	errRTPSenderTrackRemoved         = errors.New("Sender 的轨道已被移除或替换为 nil")
	errRTPSenderRidNil               = errors.New("rid 为空，Sender 无法添加编码")
	errRTPSenderNoBaseEncoding       = errors.New("没有基础轨道，Sender 无法添加编码")
	errRTPSenderBaseEncodingMismatch = errors.New("提供的轨道与基础轨道不匹配，Sender 无法添加编码")
	errRTPSenderRIDCollision         = errors.New("RID 冲突，Sender 无法编码")
	errRTPSenderNoTrackForRID        = errors.New("Sender 没有该 RID 的轨道")

	errRTPTransceiverCannotChangeMid        = errors.New("无法更改收发器的 mid")
	errRTPTransceiverSetSendingInvalidState = errors.New("RTPTransceiver.setSending 中的状态变更无效")
	errRTPTransceiverCodecUnsupported       = errors.New("此收发器不支持的编解码器类型")

	errSCTPTransportDTLS = errors.New("DTLS 未建立")

	errSDPZeroTransceivers                 = errors.New("addTransceiverSDP() 调用时没有收发器")
	errSDPMediaSectionMediaDataChanInvalid = errors.New("无效的媒体段：Media 与 DataChannel 同时启用")
	errSDPMediaSectionMultipleTrackInvalid = errors.New(
		"无效的媒体段：UnifiedPlan 中一个 MediaSection 不能包含多个轨道",
	)

	errSettingEngineSetAnsweringDTLSRole = errors.New("SetAnsweringDTLSRole 必须是 DTLSRoleClient 或 DTLSRoleServer")

	errSignalingStateCannotRollback            = errors.New("无法从 stable 状态回滚")
	errSignalingStateProposedTransitionInvalid = errors.New("无效的信令状态转换")

	errStatsICECandidateStateInvalid = errors.New(
		"无法转换为 StatsICECandidatePairStateSucceeded：无效的 ICE 候选状态",
	)

	errInvalidICECredentialTypeString = errors.New("无效的 ICECredentialType")
	errInvalidICEServer               = errors.New("无效的 ICEServer")

	errICETransportNotInNew = errors.New("仅可在 ICETransportStateNew 状态下调用 ICETransport")
	errICETransportClosed   = errors.New("ICETransport 已关闭")

	errCertificatePEMMultipleCert = errors.New("解析证书失败：pems 中含有多于 1 个 CERTIFICATE 块")
	errCertificatePEMMultiplePriv = errors.New("解析证书失败：pems 中含有多于 1 个 PRIVATE KEY 块")
	errCertificatePEMMissing      = errors.New("解析证书失败：pems 必须同时包含 CERTIFICATE 块与 PRIVATE KEY 块") // nolint: lll

	errRTPTooShort = errors.New("长度不足以构成 RTP 数据包")

	errExcessiveRetries = errors.New("CreateOffer 中重试次数过多")
)
