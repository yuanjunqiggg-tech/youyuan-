package main

import (
	"bot-apk/config"
	"encoding/json"
	"fmt"
	"time"
)

func init() {
	for k, v := range map[string]string{
		"observerBridge.assignDimension": "options={dimension:0|1|2,player?,landing?:{x,y,z},game_mode?:keep|survival|creative|adventure|spectator,invisible?:bool,auto_respawn?:bool}；仅控制本机器人，关闭auto_creative维护；未指定模式保持当前。跨维度优先靠近该维度目标玩家；无玩家时仅使用明确配置的落点，否则保留当前位置。导入导出期间暂停；ready按真实身体维度判断。需本账号OP。",
		"observerBridge.dimensionStatus": "读取本机器人实际维度、游戏模式、连接和OP；不能将摄像机位置当身体维度。",
	} {
		yyPatrolDocs[k] = v
		yyExtraMethods[k] = v
	}
}

type yyObserverDimensionOptions struct {
	Dimension   *int32            `json:"dimension"`
	Player      string            `json:"player"`
	Landing     *yyCaptureLanding `json:"landing,omitempty"`
	GameMode    string            `json:"game_mode,omitempty"`
	Invisible   *bool             `json:"invisible,omitempty"`
	AutoRespawn *bool             `json:"auto_respawn,omitempty"`
}

func yyObserverDimensionTransfer(name string, o yyObserverDimensionOptions) string {
	quoted, _ := json.Marshal(name)
	if o.Player != "" {
		target, _ := json.Marshal(o.Player)
		return fmt.Sprintf("execute in %s at @a[name=%s] run tp %s ~ ~6 ~", yyWatchDimension(*o.Dimension), target, quoted)
	}
	if o.Landing != nil {
		return fmt.Sprintf("execute in %s run tp %s %.3f %.3f %.3f", yyWatchDimension(*o.Dimension), quoted, o.Landing.X, o.Landing.Y, o.Landing.Z)
	}
	return ""
}

func yyObserverDimensionStatus(bm *BotManager) map[string]any {
	if bm == nil {
		return map[string]any{"connected": false, "ready": false}
	}
	bm.mu.Lock()
	status := map[string]any{"connected": bm.connected, "dimension": bm.dimension, "game_mode": bm.gameMode, "name": bm.display, "server": bm.serverCode, "is_op": bm.isOP || bm.opPacketConfirmed}
	bm.mu.Unlock()
	busy := false
	if pluginController != nil {
		pluginController.mu.Lock()
		for _, active := range pluginController.running {
			busy = busy || active
		}
		for _, active := range pluginController.loading {
			busy = busy || active
		}
		pluginController.mu.Unlock()
	}
	yyRuntimeMu.Lock()
	busy = busy || len(yyRuntimeInstances) > 0
	yyRuntimeMu.Unlock()
	status["business_plugins_running"] = busy
	return status
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
	var o yyObserverDimensionOptions
	if e = json.Unmarshal(raw, &o); e != nil || o.Dimension == nil || !yyDimensionIDValid(*o.Dimension) {
		return nil, fmt.Errorf("dimension需要0/1/2")
	}
	if o.Player != "" && !yyWatchName(o.Player) {
		return nil, fmt.Errorf("目标玩家名无效")
	}
	if !yyCaptureGameModeValid(o.GameMode) {
		return nil, fmt.Errorf("不支持的机器人游戏模式")
	}
	if o.Landing != nil {
		if err := yyCaptureLandingsValidate(map[string]yyCaptureLanding{fmt.Sprint(*o.Dimension): *o.Landing}); err != nil {
			return nil, err
		}
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
	if cfg != nil && (cfg.AutoCreative || o.AutoRespawn != nil && (cfg.AutoRespawn == nil || *cfg.AutoRespawn != *o.AutoRespawn)) {
		cfg.AutoCreative = false
		if o.AutoRespawn != nil {
			cfg.AutoRespawn = o.AutoRespawn
		}
		if e = config.SaveConfig(cfg); e != nil {
			return nil, e
		}
	}
	if o.AutoRespawn != nil {
		bm.mu.Lock()
		resources := bm.resources
		bm.mu.Unlock()
		if resources != nil {
			resources.SetAutoRespawn(*o.AutoRespawn)
		}
	}
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
	if err := yyCaptureApplyBehavior(bm, o, confirm); err != nil {
		return nil, err
	}
	if status["dimension"] != *o.Dimension {
		name, _ := status["name"].(string)
		command := yyObserverDimensionTransfer(name, o)
		if command == "" {
			status["ready"], status["submitted"], status["controls"] = false, false, controls
			status["reason"] = "目标维度暂无可用玩家，保留当前位置等待"
			return status, nil
		}
		if e = confirm(command); e != nil {
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
