package game_interface

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// TargetQueryingPos 是单个 querytarget 结果中的位置信息
type TargetQueryingPos struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

// 用于描述单个 querytarget 结果的结构体
type TargetQueryingInfo struct {
	Dimension      uint8             `json:"dimension"`
	PlayerUniqueID int64             `json:"id"`
	Position       TargetQueryingPos `json:"position"`
	PlayerUUID     uuid.UUID         `json:"uniqueId"`
	YRot           float32           `json:"yRot"`
}

// Querytarget 是基于 Commands 包装的 Querytarget 实现
type Querytarget struct {
	api *Commands
}

// NewQuerytarget 根据 api 返回并创建一个新的 Querytarget
func NewQuerytarget(api *Commands) *Querytarget {
	return &Querytarget{api: api}
}

// DoQuerytarget 查询目标选择器为 target 的坐标信息。
// 如果返回的切片为空，则说明目标不存在
func (q *Querytarget) DoQuerytarget(target string) ([]TargetQueryingInfo, error) {
	api := q.api

	// 授权后网易大部分命令不返回结果；querytarget 用短超时，超时返回错误，
	// 由调用方（如 getBotPos）兜底（本地数据包坐标），避免永久卡死。
	resp, isTimeout, err := api.SendWSCommandWithTimeout(
		fmt.Sprintf("querytarget %s", target),
		DefaultTimeoutCommandRequest,
	)
	if err != nil && !isTimeout {
		return nil, fmt.Errorf("执行查询目标失败: %v", err)
	}
	if isTimeout {
		return nil, fmt.Errorf("执行查询目标: querytarget 命令超时")
	}

	result := make([]TargetQueryingInfo, 0)
	if resp.SuccessCount <= 0 || len(resp.OutputMessages[0].Parameters) <= 0 {
		return nil, nil
	}

	err = json.Unmarshal([]byte(resp.OutputMessages[0].Parameters[0]), &result)
	if err != nil {
		return nil, fmt.Errorf("执行查询目标失败: %v", err)
	}

	return result, nil
}
