package main

import (
	"bot-apk/config"
	"encoding/json"
	"fmt"
	"time"
)

func init() {
	for k, v := range map[string]string{
		"observerBridge.assignDimension": "options={dimension:0|1|2}；仅控制本机器人，持久关闭auto_creative，不改变当前游戏模式。身体不在分工维度时请求正常TP回该维度；正在导入导出则暂停，ready仅按真实身体维度判断，submitted不是收到动作包。需本账号OP。",
		"observerBridge.dimensionStatus": "读取本机器人实际维度、游戏模式、连接和OP；不能将摄像机位置当身体维度。",
	} {
		yyPatrolDocs[k] = v
		yyExtraMethods[k] = v
	}
}

func yyObserverDimensionStatus(bm *BotManager) map[string]any {
	if bm == nil {
		return map[string]any{"connected": false, "ready": false}
	}
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return map[string]any{"connected": bm.connected, "dimension": bm.dimension, "game_mode": bm.gameMode, "name": bm.display, "server": bm.serverCode, "is_op": bm.isOP || bm.opPacketConfirmed}
}
func yyObserverDimensionCall(bm *BotManager, method string, args []any) (any, error) {
	if method == "observerBridge.dimensionStatus" {
		return yyObserverDimensionStatus(bm), nil
	}
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("观察机器人未连接")
	}
	if !bm.IsOP() {
		return nil, fmt.Errorf("维度分工需要本机器人OP")
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("需要dimension参数")
	}
	raw, e := json.Marshal(args[0])
	if e != nil {
		return nil, e
	}
	var o struct {
		Dimension *int32 `json:"dimension"`
	}
	if e = json.Unmarshal(raw, &o); e != nil || o.Dimension == nil || *o.Dimension < 0 || *o.Dimension > 2 {
		return nil, fmt.Errorf("dimension需要0/1/2")
	}
	status := yyObserverDimensionStatus(bm)
	status["assigned_dimension"] = *o.Dimension
	if !bm.dimensionMaintenanceMu.TryLock() {
		status["ready"] = false
		status["reason"] = "上一轮维度维护尚未结束，不叠加请求"
		return status, nil
	}
	defer bm.dimensionMaintenanceMu.Unlock()
	if busy, reasons := yySpatialJobState(bm); busy {
		status["ready"] = false
		status["paused_for_job"] = true
		status["pause_reasons"] = reasons
		return status, nil
	}
	cfg := config.LoadConfig()
	if cfg != nil && cfg.AutoCreative {
		cfg.AutoCreative = false
		if e = config.SaveConfig(cfg); e != nil {
			return nil, e
		}
	}
	name, _ := json.Marshal(status["name"])
	controls := []any{}
	confirm := func(command string) error {
		p, timedOut, err := bm.SendWSCommandWithTimeout(command, 3*time.Second)
		entry := map[string]any{"command": command, "confirmed": !timedOut && err == nil && p != nil, "success": p != nil && p.SuccessCount > 0}
		controls = append(controls, entry)
		if err != nil {
			return err
		}
		if timedOut || p == nil {
			return fmt.Errorf("维度维护等待服务器确认超时；请求可能已执行，需读取实际状态")
		}
		if p.SuccessCount == 0 {
			return fmt.Errorf("服务器拒绝维度/模式维护命令: %v", p.OutputMessages)
		}
		return nil
	}
	if status["dimension"] != *o.Dimension {
		if e = confirm(fmt.Sprintf("execute in %s run tp %s 20 120 20", yyWatchDimension(*o.Dimension), name)); e != nil {
			return nil, e
		}
	}
	// CommandOutput acceptance is not proof of ChangeDimension.
	status = yyObserverDimensionStatus(bm)
	status["assigned_dimension"] = *o.Dimension
	status["controls"] = controls
	status["ready"] = status["dimension"] == *o.Dimension
	status["submitted"] = len(controls) > 0
	if status["ready"] != true {
		status["reason"] = "等待连接层确认真实身体维度，不能以命令成功代替"
	}
	return status, nil
}
