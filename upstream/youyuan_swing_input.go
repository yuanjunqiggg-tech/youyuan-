package main

import "fmt"

func init() {
	yyExtraMethods["bot.swingAir"] = "无参数；在下一正常PlayerAuthInput心跳提交一次MissedSwing，复用当前位置/视角与递增输入Tick，不发旧式Animate。重复未发送请求合并；submitted=false表示仅排队；服务器广播/处理另行观察。"
}
func yySwingAir(bm *BotManager, args []any) (any, error) {
	if len(args) != 0 || bm == nil {
		return nil, fmt.Errorf("bot.swingAir()需要连接账号")
	}
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.conn == nil || !bm.connected || bm.conn.Context().Err() != nil {
		return nil, fmt.Errorf("游戏未连接")
	}
	if !bm.heartbeatEnabled || bm.heartbeatStopCh == nil {
		return nil, fmt.Errorf("正常玩家输入心跳未启用")
	}
	coalesced := bm.missedSwingPending
	bm.missedSwingPending = true
	return map[string]any{"queued": true, "submitted": false, "confirmed": false, "coalesced": coalesced, "input": "PlayerAuthInput.MissedSwing", "next_input_tick": bm.tick + 1}, nil
}
