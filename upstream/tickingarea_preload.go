package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bot-apk/building"
	"bot-apk/state"
)

var yyImportTickingSequence atomic.Uint64

var yyActiveTickingAreas = struct {
	sync.Mutex
	leases map[state.TickingAreaLease]bool
}{leases: make(map[state.TickingAreaLease]bool)}

// 只清理已记录归属的区域；超时的 add 也可能已生效，必须保留待清理记录。
func yyTickingCallbacks(bm *BotManager, prefix string, enabled bool) (
	preload func(name string, x1, z1, x2, z2 int) error,
	release func(name string) error,
	cleanup func(),
) {
	if !enabled || bm == nil {
		return nil, nil, nil
	}
	bm.mu.Lock()
	conn := bm.conn
	server, dim := bm.serverCode, bm.commandDimension
	if dim == "" {
		dim = map[int32]string{0: "overworld", 1: "the_nether", 2: "the_end"}[bm.dimension]
		if dim == "" {
			dim = fmt.Sprintf("dm%d", bm.dimension)
		}
	}
	bm.mu.Unlock()
	if conn == nil {
		return nil, nil, nil
	}
	if prefix == "" {
		prefix = building.TickingPreloadPrefix
	}
	prefix = fmt.Sprintf("%s%x_%x_", prefix, time.Now().UnixNano(), yyImportTickingSequence.Add(1))
	owned := map[string]bool{}
	var mu sync.Mutex
	recovered := false
	lease := func(name string) state.TickingAreaLease {
		return state.TickingAreaLease{Server: server, Dimension: dim, Name: name}
	}
	check := func() error {
		bm.mu.Lock()
		defer bm.mu.Unlock()
		if bm.conn != conn || bm.serverCode != server || !bm.connected {
			return fmt.Errorf("常加载区域所属连接已结束，保留记录待原服务器恢复后清理")
		}
		return nil
	}
	// Bind command I/O to the original interface, even if the UI reconnects mid-command.
	var commandBot *BotManager
	bindCommands := func() error {
		if commandBot != nil {
			return nil
		}
		gi, err := bm.getGameInterface()
		if err != nil {
			return err
		}
		if err := check(); err != nil {
			return err
		}
		commandBot = &BotManager{conn: conn, gameInterface: gi}
		return nil
	}
	remove := func(v state.TickingAreaLease) error {
		if err := check(); err != nil {
			return err
		}
		if err := bindCommands(); err != nil {
			return err
		}
		return commandBot.gameInterface.Commands().SendWSCommand(yyTickingCommand("tickingarea remove "+v.Name, v.Dimension))
	}
	preload = func(name string, x1, z1, x2, z2 int) error {
		mu.Lock()
		defer mu.Unlock()
		if err := building.ValidateTickingArea(x1, z1, x2, z2); err != nil {
			return err
		}
		if err := check(); err != nil {
			return err
		}
		if !recovered {
			// Submit owned stale removals before the add; receipt/list latency must not gate placement.
			old, err := state.ListTickingAreas(server)
			if err != nil {
				return err
			}
			for _, v := range old {
				if v.Dimension != dim {
					continue
				}
				yyActiveTickingAreas.Lock()
				active := yyActiveTickingAreas.leases[v]
				if !active {
					yyActiveTickingAreas.leases[v] = true
				}
				yyActiveTickingAreas.Unlock()
				if active {
					continue
				}
				owned[v.Name] = false
				if err := remove(v); err != nil {
					yyReceiveLog(bm.logCh, fmt.Sprintf("旧常加载区域清理提交失败，保留记录: %v", err))
					if err := check(); err != nil {
						return err
					}
				}
			}
			recovered = true
		}
		name = prefix + name
		v := lease(name)
		yyActiveTickingAreas.Lock()
		err := state.RememberTickingArea(v)
		if err == nil {
			yyActiveTickingAreas.leases[v] = true
		}
		yyActiveTickingAreas.Unlock()
		if err != nil {
			return err
		}
		owned[name] = true
		if err := bindCommands(); err != nil {
			return err
		}
		return commandBot.gameInterface.Commands().SendWSCommand(yyTickingAddCommand(name, x1, z1, x2, z2, dim))
	}
	releaseOwned := func(name string) error {
		v := lease(name)
		if err := remove(v); err != nil {
			return err
		}
		// Submission is not confirmation: retain persistent ownership until the end-of-job readback.
		owned[name] = false
		return nil
	}
	release = func(name string) error {
		mu.Lock()
		defer mu.Unlock()
		name = prefix + name
		if !owned[name] {
			return nil
		}
		return releaseOwned(name)
	}
	cleanup = func() {
		mu.Lock()
		defer mu.Unlock()
		names := make([]string, 0, len(owned))
		for name, active := range owned {
			if active {
				if err := releaseOwned(name); err != nil {
					yyReceiveLog(bm.logCh, fmt.Sprintf("清理导入常加载区 %s: %v", name, err))
				}
			}
			names = append(names, name)
			yyActiveTickingAreas.Lock()
			delete(yyActiveTickingAreas.leases, lease(name))
			yyActiveTickingAreas.Unlock()
		}
		if len(names) > 0 && commandBot != nil && check() == nil {
			go func() {
				areas, err := commandBot.GetTickingAreaList()
				if err == nil {
					err = yyForgetAbsentTickingAreas(server, dim, names, areas)
				}
				if err != nil {
					yyReceiveLog(bm.logCh, fmt.Sprintf("常加载区清理待确认，作业继续保留归属记录: %v", err))
				}
			}()
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

// Job callbacks submit add/remove in transport order without waiting for chunk readiness.
// Manual CRUD below still confirms receipts; cleanup ownership is never discarded on submission alone.

// TickingAreaAdd 添加常加载区，覆盖 (x1,z1)-(x2,z2)（服务器自动按 16×16 区块对齐）。
// 同名残留或名额占满时，先移除同名区域再重试一次。
func (bm *BotManager) TickingAreaAdd(name string, x1, z1, x2, z2 int) error {
	if bm == nil {
		return fmt.Errorf("tickingarea add %s: 未连接", name)
	}
	bm.mu.Lock()
	dim := bm.commandDimension
	bm.mu.Unlock()
	return bm.tickingAreaAddInDimension(name, x1, z1, x2, z2, dim)
}

func (bm *BotManager) tickingAreaAddInDimension(name string, x1, z1, x2, z2 int, dim string) error {
	if err := building.ValidateTickingArea(x1, z1, x2, z2); err != nil {
		return err
	}
	if bm == nil || bm.conn == nil {
		return fmt.Errorf("tickingarea add %s: 未连接", name)
	}
	if dim == "" {
		bm.mu.Lock()
		dim = fmt.Sprint(bm.dimension)
		bm.mu.Unlock()
	}
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	cmd := yyTickingAddCommand(name, x1, z1, x2, z2, dim)
	return yyConfirmTickingChange(func() (bool, error) { return bm.IsCmdSuccess(cmd) }, func() (bool, error) {
		areas, err := bm.GetTickingAreaList()
		if err != nil {
			return false, err
		}
		return yyTickingAreaCovers(areas[name], dim, x1, z1, x2, z2), nil
	})
}

func yyTickingAddCommand(name string, x1, z1, x2, z2 int, dim string) string {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	y1, y2 := tickingYRangeForDimension(dim)
	return yyTickingCommand(fmt.Sprintf("tickingarea add %d %d %d %d %d %d %s true", x1, y1, z1, x2, y2, z2, name), dim)
}

func yyForgetAbsentTickingAreas(server, dim string, names []string, areas map[string]*TickingAreaInfo) error {
	for _, name := range names {
		if a := areas[name]; a == nil || yyTickingDimensionKey(a.Dimension) != yyTickingDimensionKey(dim) {
			if err := state.ForgetTickingArea(state.TickingAreaLease{Server: server, Dimension: dim, Name: name}); err != nil {
				return err
			}
		}
	}
	return nil
}

// TickingAreaRemove 移除常加载区。区域不存在时忽略（视为已卸载）。
func (bm *BotManager) TickingAreaRemove(name string) error {
	if bm == nil {
		return fmt.Errorf("tickingarea remove %s: 未连接", name)
	}
	bm.mu.Lock()
	dim := bm.commandDimension
	bm.mu.Unlock()
	return bm.tickingAreaRemoveInDimension(name, dim)
}

func (bm *BotManager) tickingAreaRemoveInDimension(name, dim string) error {
	if bm == nil || bm.conn == nil {
		return fmt.Errorf("tickingarea remove %s: 未连接", name)
	}
	if dim == "" {
		bm.mu.Lock()
		dim = fmt.Sprint(bm.dimension)
		bm.mu.Unlock()
	}
	return yyConfirmTickingChange(func() (bool, error) {
		return bm.IsCmdSuccess(yyTickingCommand("tickingarea remove "+name, dim))
	}, func() (bool, error) {
		areas, err := bm.GetTickingAreaList()
		if err != nil {
			return false, err
		}
		a := areas[name]
		return a == nil || yyTickingDimensionKey(a.Dimension) != yyTickingDimensionKey(dim), nil
	})
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
	return tickingYRangeForDimension(dim)
}

func tickingYRangeForDimension(dim string) (int, int) {
	dim = strings.ToLower(dim)
	switch {
	case strings.Contains(dim, "nether"):
		return 0, 127
	case strings.Contains(dim, "end"):
		return 0, 255
	}
	return -64, 319
}

func yyTickingCommand(cmd, dim string) string {
	if dim == "" {
		return cmd
	}
	return fmt.Sprintf("execute in %s run %s", normalizeDimension(dim), cmd)
}
