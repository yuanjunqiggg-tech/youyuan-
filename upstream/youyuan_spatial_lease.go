package main

import (
	"fmt"
	"strings"
	"sync"
)

var yySpatialJobs = struct {
	sync.Mutex
	Items map[*BotManager]*yySpatialSlot
}{Items: map[*BotManager]*yySpatialSlot{}}

type yySpatialSlot struct {
	sync.RWMutex
	Reasons map[string]int
}

const yyObserverCommandRate = 20

func yySpatialSlotFor(bm *BotManager) *yySpatialSlot {
	yySpatialJobs.Lock()
	defer yySpatialJobs.Unlock()
	slot := yySpatialJobs.Items[bm]
	if slot == nil {
		slot = &yySpatialSlot{Reasons: map[string]int{}}
		yySpatialJobs.Items[bm] = slot
	}
	return slot
}
func yyObserverSend(bm *BotManager, command string, allowDuringJob bool) (bool, error) {
	slot := yySpatialSlotFor(bm)
	slot.RLock()
	defer slot.RUnlock()
	if len(slot.Reasons) > 0 && !allowDuringJob {
		return false, nil
	}
	// Bound all observation loops together; millisecond target intervals must not flood the server.
	return true, bm.SendCommandNoWaitRate(ChannelAI, command, yyObserverCommandRate)
}

func init() {
	yyExtraMethods["observerBridge.jobState"] = "返回当前账号导入导出等空间作业的paused/reasons；插件先检查paused再发传送，避免高频重复触发暂停错误。作业结束自动恢复，普通查询和非移动命令保持可用。"
}

// Reference counted per kernel, so finishing one of two jobs cannot resume
// a watcher while another job still owns the bot's position.
func yyBeginSpatialJob(bm *BotManager, reason string) func() {
	if bm == nil {
		return func() {}
	}
	slot := yySpatialSlotFor(bm)
	slot.Lock()
	slot.Reasons[reason]++
	slot.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			slot.Lock()
			defer slot.Unlock()
			rows := slot.Reasons
			rows[reason]--
			if rows[reason] <= 0 {
				delete(rows, reason)
			}
		})
	}
}
func yySpatialJobState(bm *BotManager) (bool, []string) {
	if bm == nil {
		return false, []string{}
	}
	slot := yySpatialSlotFor(bm)
	slot.RLock()
	defer slot.RUnlock()
	rows := slot.Reasons
	reasons := []string{}
	for k, n := range rows {
		if n > 0 {
			reasons = append(reasons, k)
		}
	}
	return len(reasons) > 0, reasons
}
func yyPluginMovementAllowed(bm *BotManager) error {
	if active, reasons := yySpatialJobState(bm); active {
		return fmt.Errorf("机器人空间作业正在进行，插件传送已暂停：%s", strings.Join(reasons, ","))
	}
	return nil
}
func yyPluginTeleportCommand(bm *BotManager, command string) bool {
	if bm == nil {
		return false
	}
	command = strings.TrimSpace(strings.TrimPrefix(command, "/"))
	tokens := strings.Fields(command)
	if len(tokens) == 0 {
		return false
	}
	bm.mu.Lock()
	name := bm.display
	bm.mu.Unlock()
	for i, token := range tokens {
		if (token == "tp" || token == "teleport") && i+1 < len(tokens) {
			target := strings.Trim(tokens[i+1], "\"")
			if target == "@s" || strings.HasPrefix(target, "@a") || target == name || strings.HasPrefix(target, "@e") {
				return true
			}
			if len(tokens) == i+4 {
				return true
			}
		}
	}
	return false
}
