package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"bot-apk/building"
)

var yyImportTickingSequence atomic.Uint64

// 每次作业只清理自己成功创建的名称，不扫描或删除其他机器人/作业的区域。
func yyTickingCallbacks(bm *BotManager, prefix string, enabled bool) (
	preload func(name string, x1, z1, x2, z2 int) error,
	release func(name string) error,
	cleanup func(),
) {
	if !enabled || bm == nil {
		return nil, nil, nil
	}
	bm.mu.Lock()
	conn:=bm.conn
	bm.mu.Unlock()
	if conn==nil {return nil,nil,nil}
	if prefix == "" {
		prefix = building.TickingPreloadPrefix
	}
	prefix = fmt.Sprintf("%s%x_%x_", prefix, time.Now().UnixNano(), yyImportTickingSequence.Add(1))
	owned := map[string]bool{}
	preload = func(name string, x1, z1, x2, z2 int) error {
		name = prefix + name
		owned[name] = true
		if err := bm.TickingAreaAdd(name, x1, z1, x2, z2); err != nil {
			return err
		}
		owned[name] = true
		return nil
	}
	release = func(name string) error {
		name = prefix + name
		if !owned[name] {
			return nil
		}
		if err := bm.TickingAreaRemove(name); err != nil {
			return err
		}
		delete(owned, name)
		return nil
	}
	cleanup = func() {
		for name := range owned {
			if err := bm.TickingAreaRemove(name); err != nil {
				yyReceiveLog(bm.logCh, fmt.Sprintf("清理导入常加载区 %s: %v", name, err))
			}
		}
	}
	return
}

// yyTickingBotPrefix 多机器人按机器人名生成互不冲突的前缀。
func yyTickingBotPrefix(bot *BotManager) string {
	if bot == nil {
		return building.TickingPreloadPrefix
	}
	bot.mu.Lock()
	name := bot.display
	bot.mu.Unlock()
	var sb strings.Builder
	sb.WriteString(building.TickingPreloadPrefix)
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('_')
	return sb.String()
}

// 常加载区预加载（tickingarea preload）底层封装。
// 用户指定方案：导入/导出批次切换之前，先用 tickingarea 把即将传送的目标区块
// 提前加载好——机器人 TP 过去时区块已就绪，省去等待区块加载的时间；批次结束后
// 卸掉，为下一批让出名额（服务器常加载区数量有上限）。所有命令经 WS 自动化
// 通道发送并等待响应，SuccessCount>0 才视为成功。

// TickingAreaAdd 添加常加载区，覆盖 (x1,z1)-(x2,z2)（服务器自动按 16×16 区块对齐）。
// 同名残留或名额占满时，先移除同名区域再重试一次。
func (bm *BotManager) TickingAreaAdd(name string, x1, z1, x2, z2 int) error {
	if bm == nil || bm.conn == nil {
		return fmt.Errorf("tickingarea add %s: 未连接", name)
	}
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	y1, y2 := bm.tickingYRange()
	raw := fmt.Sprintf("tickingarea add %d %d %d %d %d %d %s true", x1, y1, z1, x2, y2, z2, name)
	cmd := bm.WrapCommandInDimension(raw)
	ok, err := bm.IsCmdSuccess(cmd)
	if err != nil {
		return fmt.Errorf("tickingarea add %s: %v", name, err)
	}
	if !ok {
		bm.IsCmdSuccess(bm.WrapCommandInDimension("tickingarea remove " + name))
		ok, err = bm.IsCmdSuccess(cmd)
		if err != nil {
			return fmt.Errorf("tickingarea add %s: %v", name, err)
		}
		if !ok {
			return fmt.Errorf("tickingarea add %s: 服务器未确认成功", name)
		}
	}
	return nil
}

// TickingAreaRemove 移除常加载区。区域不存在时忽略（视为已卸载）。
func (bm *BotManager) TickingAreaRemove(name string) error {
	if bm == nil || bm.conn == nil {
		return fmt.Errorf("tickingarea remove %s: 未连接", name)
	}
	_, err := bm.IsCmdSuccess(bm.WrapCommandInDimension("tickingarea remove " + name))
	return err
}

// TickingAreaCleanupPrefix 列出所有维度的常加载区，删除名称带指定前缀的残留。
func (bm *BotManager) TickingAreaCleanupPrefix(prefix string) {
	if bm == nil || bm.conn == nil {
		return
	}
	areas, err := bm.GetTickingAreaList()
	if err != nil {
		return
	}
	for name := range areas {
		if strings.HasPrefix(name, prefix) {
			_, _ = bm.IsCmdSuccess(bm.WrapCommandInDimension("tickingarea remove " + name))
		}
	}
}

// tickingYRange 按当前命令维度给出合理的 Y 范围（tickingarea 的 from/to 是三维坐标，
// Y 不影响区块加载范围，但超出维度边界可能导致命令失败）。
func (bm *BotManager) tickingYRange() (int, int) {
	bm.mu.Lock()
	dim := strings.ToLower(bm.commandDimension)
	bm.mu.Unlock()
	switch {
	case strings.Contains(dim, "nether"):
		return 0, 127
	case strings.Contains(dim, "end"):
		return 0, 255
	}
	return -64, 319
}
