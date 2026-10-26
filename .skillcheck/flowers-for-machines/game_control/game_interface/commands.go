package game_interface

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/core/py_rpc"
	cts "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server"
	cts_mc "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server/minecraft"
	cts_mc_a "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/client_to_server/minecraft/ai_command"
	mei "github.com/OmineDev/flowers-for-machines/core/py_rpc/mod_event/interface"

	"github.com/google/uuid"
)

// ------------------------- Define -------------------------

const (
	// DefaultTimeoutCommandRequest 是默认的指令超时设置
	DefaultTimeoutCommandRequest = time.Second * 1
	// DefaultAwaitChangesCount 是 Await Chanegs 需要等待的游戏刻数
	DefaultAwaitChangesCount = 2
)

// Commands 是基于 ResourcesWrapper
// 实现的 MC 指令操作器，例如发送命令
// 并得到其响应体。
//
// 另外，出于对旧时代的尊重和可能的兼容性，
// 一些遗留实现也被同时迁移到此处
type Commands struct {
	api *ResourcesWrapper
}

// ------------------------- Basic function -------------------------

// NewCommands 基于 api 创建并返回一个新的 Commands
func NewCommands(api *ResourcesWrapper) *Commands {
	return &Commands{api: api}
}

// packCommandRequest 根据给定的命令 command，
// 命令来源 origin 和命令请求 ID 包装一个命令请求体
func packCommandRequest(command string, origin uint32, requestID uuid.UUID) *packet.CommandRequest {
	return &packet.CommandRequest{
		CommandLine: command,
		CommandOrigin: protocol.CommandOrigin{
			Origin:    origin,
			UUID:      requestID,
			RequestID: "96045347-a6a3-4114-94c0-1bc4cc561694",
		},
		Internal:  false,
		UnLimited: false,
		Version:   44,
	}
}

// ------------------------- Send settings command -------------------------

// 向租赁服发送 Sizukana 命令且无视返回值。
// 当 dimensional 为真时，
// 将使用 execute 更换命令执行环境为机器人所在的环境
func (c *Commands) SendSettingsCommand(command string, dimensional bool) error {
	api := c.api

	if dimensional {
		command = fmt.Sprintf(
			`execute as @a[name="%s"] at @s run %s`,
			api.BotName,
			command,
		)
	}

	err := api.WritePacket(&packet.SettingsCommand{
		CommandLine:    command,
		SuppressOutput: true,
	})
	if err != nil {
		return fmt.Errorf("发送设置命令失败: %v", err)
	}

	return nil
}

// ------------------------- Send command with no response -------------------------

// sendCommand 以 origin 的身份向租赁服发送命令 command 并无视返回值
func (c *Commands) sendCommand(command string, origin uint32) error {
	err := c.api.WritePacket(
		packCommandRequest(
			command, origin, uuid.New(),
		),
	)
	if err != nil {
		return fmt.Errorf("发送命令失败: %v", err)
	}
	return nil
}

// SendPlayerCommand 以玩家的身份向租赁服发送命令 command 并无视返回值
func (c *Commands) SendPlayerCommand(command string) error {
	err := c.sendCommand(command, protocol.CommandOriginPlayer)
	if err != nil {
		return fmt.Errorf("发送玩家命令失败: %v", err)
	}
	return nil
}

// SendPlayerCommand 以 Websocket 的身份向租赁服发送命令 command 并无视返回值
func (c *Commands) SendWSCommand(command string) error {
	err := c.sendCommand(command, protocol.CommandOriginAutomationPlayer)
	if err != nil {
		return fmt.Errorf("发送WS命令失败: %v", err)
	}
	return nil
}

// ------------------------- Send command with response and timeout -------------------------

// sendCommandWithResp 以 origin 的身份向租赁服发送命令 command 并获取响应体。
// timeout 指示超时处理；如果为负数则不考虑超时因素；如果为 0 则使用默认超时设置。
// 需要注意的是，如果命令请求超时，则返回的 err 不为空
func (c *Commands) sendCommandWithResp(command string, origin uint32, timeout time.Duration) (
	resp *packet.CommandOutput,
	isTimeOut bool,
	err error,
) {
	var terminalErr error

	api := c.api
	doOnce := new(sync.Once)
	requestID := uuid.New()
	channel := make(chan struct{})

	api.Resources.Commands().SetCommandRequestCallback(
		requestID,
		func(p *packet.CommandOutput, connCloseErr error) {
			doOnce.Do(func() {
				resp, terminalErr = p, connCloseErr
				close(channel)
			})
		},
	)
	defer api.Resources.Commands().DeleteCommandRequestCallback(requestID)

	err = api.WritePacket(
		packCommandRequest(
			command, origin, requestID,
		),
	)
	if err != nil {
		return nil, false, fmt.Errorf("发送命令(带响应)失败: %v", err)
	}

	if timeout == 0 {
		timeout = DefaultTimeoutCommandRequest
	}
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-channel:
		case <-timer.C:
			// A timeout is unknown, not proof that the server did not execute. Never resend here.
			return nil, true, fmt.Errorf(
				"发送命令(带响应): 命令请求%#v(来源=%d)超时(超时时间=%v秒)",
				command, origin, float64(timeout)/float64(time.Second),
			)
		}
	}
	<-channel

	if terminalErr != nil {
		return nil, false, fmt.Errorf("发送命令(带响应)失败: %v", terminalErr)
	}
	return resp, false, nil
}

// SendPlayerCommandWithTimeout 以玩家的身份向租赁服发送命令 command 并获取响应体。
// timeout 指示当请求发出后，若时间超过 timeout 是否应当返回错误。
// 如果 timeout 为负数或 0，则使用默认超时设置。
// 需要注意的是，如果命令请求超时，则返回的 err 不为空
func (c *Commands) SendPlayerCommandWithTimeout(command string, timeout time.Duration) (
	resp *packet.CommandOutput,
	isTimeOut bool,
	err error,
) {
	resp, isTimeOut, err = c.sendCommandWithResp(command, protocol.CommandOriginPlayer, max(0, timeout))
	if err != nil {
		return nil, isTimeOut, fmt.Errorf("发送玩家命令(带超时)失败: %v", err)
	}
	return
}

// SendWSCommandWithTimeout 以 Websocket 的身份向租赁服发送命令 command 并获取响应体。
// timeout 指示当请求发出后，若时间超过 timeout 是否应当返回错误。
// 如果 timeout 为负数或 0，则使用默认超时设置。
// 需要注意的是，如果命令请求超时，则返回的 err 不为空
func (c *Commands) SendWSCommandWithTimeout(command string, timeout time.Duration) (
	resp *packet.CommandOutput,
	isTimeOut bool,
	err error,
) {
	resp, isTimeOut, err = c.sendCommandWithResp(command, protocol.CommandOriginAutomationPlayer, max(0, timeout))
	if err != nil {
		return nil, isTimeOut, fmt.Errorf("发送WS命令(带超时)失败: %v", err)
	}
	return
}

// ------------------------- Send command with response and no timeout -------------------------

// sendCommandWithRespNoTimeout 以 origin 的身份向租赁服发送命令 command 并获取响应体。
// 区别于 sendCommandWithResp，此函数不考虑超时因素
func (c *Commands) sendCommandWithRespNoTimeout(command string, origin uint32) (resp *packet.CommandOutput, err error) {
	resp, _, err = c.sendCommandWithResp(command, origin, -1)
	if err != nil {
		return nil, fmt.Errorf("发送命令(带响应,无超时)失败: %v", err)
	}
	return
}

// SendPlayerCommandWithResp 以玩家的身份向租赁服发送命令 command 并获取响应体。
// 值得说明的是，此过程中不考虑超时因素
func (c *Commands) SendPlayerCommandWithResp(command string) (resp *packet.CommandOutput, err error) {
	resp, err = c.sendCommandWithRespNoTimeout(command, protocol.CommandOriginPlayer)
	if err != nil {
		return nil, fmt.Errorf("发送玩家命令(带响应)失败: %v", err)
	}
	return
}

// SendAICommand 通过网易特有的 AICommand（PyRpc ModEvent）通道执行命令。
// 普通导入的 setblock 走的正是这条通道，在网易服务器上可靠；
// 而标准 SettingsCommand 包在网易上可能静默失败（如 replaceitem/enchant）。
// fire-and-forget，不等待响应。
func (c *Commands) SendAICommand(cmd string) error {
	event := cts_mc_a.ExecuteCommandEvent{
		CommandLine:      "execute run " + cmd,
		CommandRequestID: uuid.New(),
	}
	module := cts_mc.AICommand{Module: &mei.DefaultModule{Event: &event}}
	park := cts.Minecraft{Default: mei.Default{Module: &module}}

	return c.api.WritePacket(&packet.PyRpc{
		Value: py_rpc.Marshal(&py_rpc.ModEvent{
			Package: &park,
			Type:    py_rpc.ModEventClientToServer,
		}),
		OperationType: packet.PyRpcOperationTypeSend,
	})
}

// SendWSCommandWithResp 以 Websocket 的身份向租赁服发送命令 command 并获取响应体。
// 值得说明的是，此过程中不考虑超时因素
func (c *Commands) SendWSCommandWithResp(command string) (resp *packet.CommandOutput, err error) {
	resp, err = c.sendCommandWithRespNoTimeout(command, protocol.CommandOriginAutomationPlayer)
	if err != nil {
		return nil, fmt.Errorf("发送WS命令(带响应)失败: %v", err)
	}
	return
}

// ------------------------- Other legacy function -------------------------

// AwaitChangesGeneral 通过发送空指令以等待租赁服更改。
// 它曾被广泛使用而难以替代，但此处出于语义兼容性而保留
func (c *Commands) AwaitChangesGeneral() error {
	for range DefaultAwaitChangesCount {
		// 带超时等待空命令响应：网易可能不响应空命令，2s 超时避免永久卡死（书导入等），
		// 同时给附魔等操作留出同步时间——纯 fire-and-forget 会让附魔后不等同步、
		// 移回物品时被服务器拒绝（Ench stage 2）。
		_, isTimeout, err := c.SendWSCommandWithTimeout("", DefaultTimeoutCommandRequest)
		if err != nil && !isTimeout {
			return fmt.Errorf("等待通用变更失败: %v", err)
		}
	}
	return nil
}

// SendChat 使机器人在聊天栏说出 content 的内容
func (c *Commands) SendChat(content string) error {
	api := c.api

	err := api.WritePacket(
		&packet.Text{
			TextType:         packet.TextTypeChat,
			NeedsTranslation: false,
			SourceName:       api.BotName,
			Message:          content,
			XUID:             api.XUID,
			PlatformChatID:   "",
			Unknown1:         []string{"PlayerId", fmt.Sprintf("%d", api.EntityRuntimeID)},
		},
	)
	if err != nil {
		return fmt.Errorf("发送聊天失败: %v", err)
	}

	return nil
}

// 以 actionbar 的形式向所有在线玩家显示 message
func (c *Commands) Title(message string) error {
	title := map[string]any{
		"rawtext": []any{
			map[string]any{
				"text": message,
			},
		},
	}
	jsonBytes, _ := json.Marshal(title)

	err := c.SendSettingsCommand(fmt.Sprintf("titleraw @a actionbar %s", jsonBytes), false)
	if err != nil {
		return fmt.Errorf("发送标题失败: %v", err)
	}

	return nil
}

// ------------------------- End -------------------------
