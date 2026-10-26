package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
	"github.com/OmineDev/flowers-for-machines/game_control/resources_control"
	"github.com/OmineDev/flowers-for-machines/mapping"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_cache"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_console"
	"github.com/OmineDev/flowers-for-machines/utils"
	"github.com/TriM-Organization/bedrock-world-operator/block"
	"github.com/go-gl/mathgl/mgl32"
)

// init 包装 block.StateToRuntimeID 以处理告示牌名称映射。
// block_states.bin 使用旧格式名称（standing_sign、wall_sign），
// 但建筑文件可能使用新格式名称（oak_sign、oak_wall_sign）。
func init() {
	orig := block.StateToRuntimeID
	block.StateToRuntimeID = func(name string, properties map[string]any) (uint32, bool) {
		rid, found := orig(name, properties)
		if found {
			return rid, true
		}
		// 告示牌名称映射：新格式 → 旧格式
		// 站立牌：oak_sign → standing_sign
		// 墙壁牌：oak_wall_sign → wall_sign
		// 悬挂牌已经在 block_states.bin 中
		if strings.HasSuffix(name, "_wall_sign") {
			// 墙壁牌：用 wall_sign 代替
			rid, found = orig("minecraft:wall_sign", properties)
			if found {
				return rid, true
			}
		} else if strings.HasSuffix(name, "_sign") && !strings.HasSuffix(name, "_hanging_sign") {
			// 站立牌：用 standing_sign 代替
			// 注意：standing_sign 使用 ground_sign_direction 属性，
			// 而新格式使用 facing_direction，需要转换
			mappedProps := make(map[string]any, len(properties))
			for k, v := range properties {
				if k == "facing_direction" {
					// facing_direction (2-5) → ground_sign_direction (0,4,8,12)
					// 2=北→8, 3=南→0, 4=西→4, 5=东→12
					if fd, ok := v.(int32); ok {
						gsd := map[int32]int32{2: 8, 3: 0, 4: 4, 5: 12}[fd]
						mappedProps["ground_sign_direction"] = gsd
					} else if fd, ok := v.(byte); ok {
						gsd := map[byte]byte{2: 8, 3: 0, 4: 4, 5: 12}[fd]
						mappedProps["ground_sign_direction"] = gsd
					} else {
						mappedProps[k] = v
					}
				} else {
					mappedProps[k] = v
				}
			}
			rid, found = orig("minecraft:standing_sign", mappedProps)
			if found {
				return rid, true
			}
			// 如果属性映射后仍找不到，尝试默认属性
			rid, found = orig("minecraft:standing_sign", map[string]any{})
			if found {
				return rid, true
			}
		}
		return 0, false
	}
}

// getGameInterface 懒加载 flowers 高层动作接口。
// 仅在设置 auto_creative 显式开启时保持创造模式 + 飞行。
// 只在第一次需要时创建一次，后续复用。
//
// 初始化会注册监听器和维护器；独立串行化，避免重复注册，也不占用心跳状态锁。
func (bm *BotManager) getGameInterface() (*game_interface.GameInterface, error) {
	gi, created, err := bm.loadGameInterface(func(resources *resources_control.Resources, conn *minecraft.Conn) (*game_interface.GameInterface, error) {
		maintainer := &yyBotMaintainer{bm: bm, conn: conn}
		gi, err := game_interface.NewGameInterface(resources, maintainer)
		if err != nil {
			return nil, err
		}
		gi.Movement().SetLocalPositionProvider(func() ([3]float32, bool) {
			if !maintainer.currentSession() {
				return [3]float32{}, false
			}
			return bm.querySelfFallback()
		})
		gi.SetStructureReader(func(pos, size protocol.BlockPos) (map[string]any, error) {
			if !maintainer.currentSession() {
				return nil, fmt.Errorf("区域读取所属连接已结束")
			}
			return bm.requestStructureContext(conn.Context(), [3]int32(pos), [3]int32(size), 5*time.Second)
		})
		return gi, nil
	})
	if err != nil || !created {
		return gi, err
	}
	if m, ok := gi.Maintainer().(*yyBotMaintainer); ok {
		m.startWatchdog(gi)
	}
	go func() {
		if m := gi.Maintainer(); m != nil {
			if err := m.TouchMaintainer(gi); err != nil {
				debugLog("[模式维护] %v", err)
			}
		}
	}()
	return gi, nil
}

func (bm *BotManager) loadGameInterface(create func(*resources_control.Resources, *minecraft.Conn) (*game_interface.GameInterface, error)) (*game_interface.GameInterface, bool, error) {
	bm.mu.Lock()
	gi := bm.gameInterface
	bm.mu.Unlock()
	if gi != nil {
		return gi, false, nil
	}
	bm.gameInterfaceInitMu.Lock()
	defer bm.gameInterfaceInitMu.Unlock()
	bm.mu.Lock()
	gi, resources, conn := bm.gameInterface, bm.resources, bm.conn
	connected := bm.connected
	bm.mu.Unlock()
	if gi != nil {
		return gi, false, nil
	}
	if resources == nil || conn == nil || !connected {
		return nil, false, fmt.Errorf("当前连接的 resources 未就绪")
	}
	gi, err := create(resources, conn)
	if err != nil {
		return nil, false, fmt.Errorf("创建 game_interface: %v", err)
	}
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.conn != conn || bm.resources != resources || !bm.connected {
		return nil, false, fmt.Errorf("连接已更换，丢弃旧 game_interface 初始化结果")
	}
	bm.gameInterface = gi
	return gi, true, nil
}

// Called under bm.mu on disconnect and both rental/room connection replacement paths.
func (bm *BotManager) resetGameInterfacesLocked() {
	bm.gameInterface, bm.resources = nil, nil
	bm.nbtAssigner, bm.nbtConsole = nil, nil
	bm.nbtWorkspaceEpoch++
}

// localSelfPos 返回本地数据包追踪的自身坐标，作为 getBotPos 里 querytarget 失败的兜底。
// bm.pos 由 bot_conn.go 从 Respawn/PlayerLocation/MovePlayer/AddPlayer/ChangeDimension 等数据包实时更新；
// 默认值 (0,64,0) 视为尚未收到真实坐标，返回无效。
func (bm *BotManager) localSelfPos() ([3]float32, bool) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.pos[0] == 0 && bm.pos[1] == 64 && bm.pos[2] == 0 {
		return [3]float32{}, false
	}
	return [3]float32{bm.pos[0], bm.pos[1], bm.pos[2]}, true
}

// querySelfViaFleet 多机兜底：自己 querytarget 失败时，委托兄弟机器人代为查询。
// 依次让 B、C、D… 用它们各自的 querytarget @a 查所有玩家，挑出"自己"的位置返回；
// 全部兄弟都失败返回 false。单机时 fleet 只有自己，直接返回 false。
func (bm *BotManager) querySelfViaFleet() ([3]float32, bool) {
	selfEID := bm.eid
	for _, other := range GetFleetBots() {
		if other == bm || other == nil {
			continue
		}
		// 每个兄弟带 3s 超时，避免其命令无响应时卡死导入
		ch := make(chan [3]float32, 1)
		go func(ob *BotManager) {
			gi, err := ob.getGameInterface()
			if err != nil {
				ch <- [3]float32{}
				return
			}
			infos, err := gi.Querytarget().DoQuerytarget("@a")
			if err != nil {
				ch <- [3]float32{}
				return
			}
			for _, it := range infos {
				if it.PlayerUniqueID == selfEID {
					ch <- [3]float32{it.Position.X, it.Position.Y, it.Position.Z}
					return
				}
			}
			ch <- [3]float32{}
		}(other)
		select {
		case p := <-ch:
			if !(p[0] == 0 && p[1] == 0 && p[2] == 0) {
				return p, true
			}
		case <-time.After(3 * time.Second):
			// 超时，交给下一个兄弟
		}
	}
	return [3]float32{}, false
}

// querySelfFallback 兜底链：委托兄弟机器人 → 本地数据包坐标。
func (bm *BotManager) querySelfFallback() ([3]float32, bool) {
	if p, ok := bm.querySelfViaFleet(); ok {
		return p, true
	}
	return bm.localSelfPos()
}

// ensureGameInterfaceWithRetry 确保动作接口就绪。getGameInterface 首次失败时带重试
// （幂等：成功后缓存，下次直接返回），避免授权后首次初始化竞态导致动作接口缺失、
// NBT/告示牌被静默跳过。间隔递增最多 5 次。无论成败都放行（不阻塞连接/前端），
// 返回失败次数；仍失败仅影响需动作接口的方块（告示牌/NBT），普通导入不受影响。
func (bm *BotManager) ensureGameInterfaceWithRetry() int {
	failures := 0
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := bm.getGameInterface(); err == nil {
			if attempt > 0 {
				yyReceiveLog(bm.logCh, fmt.Sprintf("[动作接口] 重试第%d次后恢复", attempt+1))
			} else {
				yyReceiveLog(bm.logCh, "[动作接口] 机器人动作接口就绪，游戏模式按设置保留或维护")
			}
			return failures
		} else {
			failures++
			yyReceiveLog(bm.logCh, fmt.Sprintf("[动作接口] 初始化失败(第%d次): %v", attempt+1, err))
		}
		// 间隔递增重试，给网络/通道一个恢复窗口
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	yyReceiveLog(bm.logCh, "[动作接口] 多次初始化仍失败，将跳过需动作接口的方块(告示牌/NBT)")
	return failures
}

// getNBTAssigner 返回 flowers NBT 方块放置系统。
// 同一个 import 任务复用一个 NBTAssigner，避免每次重建 Console 和 Cache
// 导致大量网络操作（TP、fill、clear）和内存分配。
func (bm *BotManager) getNBTAssigner(center protocol.BlockPos) (*nbt_assigner.NBTAssigner, error) {
	bm.nbtAssignerInitMu.Lock()
	defer bm.nbtAssignerInitMu.Unlock()
	bm.mu.Lock()
	if bm.nbtAssigner != nil {
		assigner := bm.nbtAssigner
		bm.mu.Unlock()
		return assigner, nil
	}
	conn, epoch := bm.conn, bm.nbtWorkspaceEpoch
	bm.mu.Unlock()

	console, err := bm.getNBTConsole(center)
	if err != nil {
		return nil, fmt.Errorf("创建NBT操作台: %w", err)
	}

	cache := nbt_cache.NewNBTCacheSystem(console)
	assigner := nbt_assigner.NewNBTAssigner(console, cache)

	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.conn != conn || bm.nbtWorkspaceEpoch != epoch || bm.nbtConsole != console {
		return nil, fmt.Errorf("连接或NBT作业已更换，丢弃旧分配器初始化结果")
	}
	bm.nbtAssigner = assigner
	return assigner, nil
}

// getNBTConsole 返回 NBT 操作台，需要时创建。
func (bm *BotManager) getNBTConsole(center protocol.BlockPos) (*nbt_console.Console, error) {
	return bm.loadNBTConsole(center, nbt_console.NewConsole)
}

func (bm *BotManager) loadNBTConsole(center protocol.BlockPos, create func(*game_interface.GameInterface, uint8, protocol.BlockPos) (*nbt_console.Console, error)) (*nbt_console.Console, error) {
	bm.nbtConsoleInitMu.Lock()
	defer bm.nbtConsoleInitMu.Unlock()
	if center == (protocol.BlockPos{}) {
		center = protocol.BlockPos{9999, 100, 9999}
	}
	bm.mu.Lock()
	if bm.nbtConsole != nil {
		console := bm.nbtConsole
		bm.mu.Unlock()
		return console, nil
	}
	conn, epoch, dim := bm.conn, bm.nbtWorkspaceEpoch, bm.commandDimension
	bm.mu.Unlock()

	gi, err := bm.getGameInterface()
	if err != nil {
		return nil, err
	}
	bm.mu.Lock()
	current := bm.conn == conn && bm.nbtWorkspaceEpoch == epoch && bm.gameInterface == gi
	bm.mu.Unlock()
	if !current {
		return nil, fmt.Errorf("连接或NBT作业已更换，未创建旧操作台")
	}

	dimID := uint8(dimNameToID(dim))

	console, err := create(gi, dimID, center)
	if err != nil {
		return nil, err
	}
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.conn != conn || bm.nbtWorkspaceEpoch != epoch || bm.gameInterface != gi {
		return nil, fmt.Errorf("连接或NBT作业已更换，丢弃旧操作台初始化结果")
	}
	bm.nbtConsole = console
	return console, nil
}

// signTextOf 从告示牌 NBT 里取某一面的文字（FrontText/BackText → Text）。
func signTextOf(nbt map[string]any, face string) string {
	m, ok := nbt[face].(map[string]any)
	if !ok {
		return ""
	}
	t, _ := m["Text"].(string)
	return t
}

// signGlowOf 判断告示牌某一面是否发光（IgnoreLighting==1）。
func signGlowOf(nbt map[string]any, face string) bool {
	m, ok := nbt[face].(map[string]any)
	if !ok {
		return false
	}
	switch v := m["IgnoreLighting"].(type) {
	case byte:
		return v == 1
	case int8:
		return v == 1
	case int:
		return v == 1
	case int32:
		return v == 1
	case int64:
		return v == 1
	case bool:
		return v
	case float64:
		return v == 1
	}
	return false
}

// signIsWaxed 检查告示牌 NBT 是否有涂蜡标记。
func signIsWaxed(nbt map[string]any) bool {
	switch v := nbt["IsWaxed"].(type) {
	case byte:
		return v == 1
	case int8:
		return v == 1
	case int:
		return v == 1
	case int32:
		return v == 1
	case int64:
		return v == 1
	case float64:
		return v == 1
	}
	return false
}

// signColorToDye 从 FrontText/BackText 中提取颜色信息，返回对应的染料物品名。
// 优先取 SignTextColor（Bedrock 格式 int32 RGBA），次选 Color（Java 格式颜色名）。
// 返回 "" 表示不需要染色（黑色默认）。
func signColorToDye(nbt map[string]any, face string) string {
	m, ok := nbt[face].(map[string]any)
	if !ok {
		return ""
	}

	// 1) Bedrock 格式：SignTextColor (int32 RGBA)
	if raw, ok := m["SignTextColor"]; ok {
		var sc int32
		switch v := raw.(type) {
		case int32:
			sc = v
		case float64:
			sc = int32(v)
		case int:
			sc = int32(v)
		case int64:
			sc = int32(v)
		}
		rgb, _ := utils.DecodeVarRGBA(sc)
		if rgb != [3]uint8{0, 0, 0} {
			best := utils.SearchForBestColor(rgb, mapping.DefaultDyeColor)
			if dye, ok := mapping.RGBToDyeItemName[best]; ok {
				return dye
			}
		}
	}

	// 2) Java 格式：Color (string, 如 "black"/"red"/"dark_blue"...)
	if c, ok := m["Color"].(string); ok && c != "" {
		lower := strings.ToLower(c)
		if dye, ok := javaSignColorToDye[lower]; ok && lower != "black" {
			return dye
		}
	}
	if c, ok := m["color"].(string); ok && c != "" {
		lower := strings.ToLower(c)
		if dye, ok := javaSignColorToDye[lower]; ok && lower != "black" {
			return dye
		}
	}
	return ""
}

// signCopyFace 从原始 NBT 复制某一面（FrontText/BackText）的所有字段。
func signCopyFace(nbt map[string]any, face string) map[string]any {
	m, ok := nbt[face].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	out["TextIgnoreDeletion"] = byte(1)
	return out
}

// handleContainerItems 处理容器内的物品放置。
// 简单物品（无 tag）通过 /replaceitem 命令直接放入目标容器。
// 改名/附魔物品先通过 Console 铁砧制作，再移动至目标容器。
func (bm *BotManager) handleContainerItems(x, y, z int32, blockName, blockStates string, nbtData map[string]any, consoleCenter protocol.BlockPos) {
	items, ok := nbtData["Items"].([]any)
	if !ok || len(items) == 0 {
		return
	}

	gi, err := bm.getGameInterface()
	if err != nil {
		return
	}

	// 先找出需要改名/附魔的复杂物品
	type complexItem struct {
		slot        int
		name        string
		count       int
		meta        int
		displayName string
		enchants    [][2]int32
	}
	var complexItems []complexItem

	// 第一遍：简单物品直接 replaceitem，复杂物品记下来
	for _, raw := range items {
		im, ok2 := raw.(map[string]any)
		if !ok2 {
			continue
		}
		slot := int(nbtInt(im, "Slot"))
		itemName, _ := im["Name"].(string)
		if itemName == "" || strings.Contains(itemName, "air") {
			continue
		}
		cnt := int(nbtInt(im, "Count"))
		if cnt < 1 {
			cnt = 1
		}
		meta := int(itemDamage(im))

		tag, hasTag := im["tag"].(map[string]any)
		if !hasTag {
			// 无 tag → 简单物品，直接 replaceitem
			cmd := fmt.Sprintf("replaceitem block %d %d %d slot.container %d %s %d %d", x, y, z, slot, itemName, cnt, meta)
			bm.SendWOCmd(cmd)
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// 有 tag → 检查是否需要改名/附魔
		displayName := ""
		if d, ok2 := tag["display"].(map[string]any); ok2 {
			displayName, _ = d["Name"].(string)
		}
		var enchList [][2]int32
		for _, src := range []any{tag["ench"], tag["Enchantments"]} {
			if list, ok2 := src.([]any); ok2 {
				for _, e := range list {
					if em, ok2 := e.(map[string]any); ok2 {
						id := int32(nbtInt(em, "id"))
						lvl := int32(nbtInt(em, "lvl"))
						if lvl < 1 {
							lvl = 1
						}
						enchList = append(enchList, [2]int32{id, lvl})
					}
				}
			}
		}
		if displayName == "" && len(enchList) == 0 {
			// 有 tag 但不需要改名/附魔（如 keep_on_death）→ 走命令
			components := buildSimpleItemComponents(tag)
			if components != "" {
				bm.SendWOCmd(fmt.Sprintf("replaceitem block %d %d %d slot.container %d %s %d %d %s", x, y, z, slot, itemName, cnt, meta, components))
			} else {
				bm.SendWOCmd(fmt.Sprintf("replaceitem block %d %d %d slot.container %d %s %d %d", x, y, z, slot, itemName, cnt, meta))
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}

		complexItems = append(complexItems, complexItem{
			slot: slot, name: itemName, count: cnt, meta: meta,
			displayName: displayName, enchants: enchList,
		})
	}

	if len(complexItems) == 0 {
		return
	}

	// 第二遍：用 Console 处理改名/附魔物品
	console, cErr := bm.getNBTConsole(consoleCenter)
	if cErr != nil {
		hub.Emit("progress", mustJSON(map[string]any{
			"msg": fmt.Sprintf("容器复杂物品处理失败（无法创建操作台）: %v", cErr),
		}))
		return
	}

	const hotbar = resources_control.SlotID(0)

	for _, ci := range complexItems {
		// 把基础物品放入快捷栏
		_ = gi.BotClick().ChangeSelectedHotbarSlot(hotbar)
		if err := gi.Replaceitem().ReplaceitemInInventory(
			"@s", game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name: ci.name, Count: uint8(ci.count), MetaData: int16(ci.meta), Slot: hotbar,
			},
			"", true,
		); err != nil {
			continue
		}

		// 改名
		if ci.displayName != "" {
			index, aErr := console.FindOrGenerateNewAnvil()
			if aErr != nil {
				continue
			}
			ok2, _ := console.OpenContainerByIndex(index)
			if !ok2 {
				continue
			}
			_, _, _, _ = gi.ItemStackOperation().OpenTransaction().
				RenameInventoryItem(hotbar, ci.displayName).
				Commit()
			_ = gi.ContainerOpenAndClose().CloseContainer()
		}

		// 附魔
		for _, ench := range ci.enchants {
			gi.Commands().SendSettingsCommand(
				fmt.Sprintf("enchant @s %d %d", ench[0], ench[1]), true,
			)
			time.Sleep(10 * time.Millisecond)
		}

		// 把物品放入目标容器
		bm.TP(int(x), int(y+1), int(z))
		time.Sleep(100 * time.Millisecond)

		blockAction := game_interface.UseItemOnBlocks{
			HotbarSlotID: hotbar,
			BotPos:       mgl32.Vec3{float32(x) + 0.5, float32(y) + 1.5, float32(z) + 0.5},
			BlockPos:     protocol.BlockPos{x, y, z},
			BlockName:    blockName,
			BlockStates:  utils.ParseBlockStatesString(blockStates),
		}
		if _, err := gi.ContainerOpenAndClose().OpenContainer(blockAction, true); err != nil {
			continue
		}
		_, _, _, _ = gi.ItemStackOperation().OpenTransaction().
			MoveToContainer(hotbar, resources_control.SlotID(ci.slot), uint8(ci.count)).
			Commit()
		_ = gi.ContainerOpenAndClose().CloseContainer()
	}
}

// cleanupNBTWorkspace 清理工作区，释放资源

func (bm *BotManager) cleanupNBTWorkspace() {
	if bm == nil {
		return
	}
	bm.mu.Lock()
	bm.nbtAssigner = nil
	bm.nbtConsole = nil
	bm.nbtWorkspaceEpoch++
	bm.mu.Unlock()
}

// javaSignColorToDye 映射 Java 文本颜色名到 Bedrock 染料物品名。
var javaSignColorToDye = map[string]string{
	"black":        "minecraft:black_dye",
	"dark_blue":    "minecraft:blue_dye",
	"dark_green":   "minecraft:green_dye",
	"dark_aqua":    "minecraft:cyan_dye",
	"dark_red":     "minecraft:red_dye",
	"dark_purple":  "minecraft:purple_dye",
	"gold":         "minecraft:orange_dye",
	"gray":         "minecraft:light_gray_dye",
	"dark_gray":    "minecraft:gray_dye",
	"blue":         "minecraft:blue_dye",
	"green":        "minecraft:lime_dye",
	"aqua":         "minecraft:light_blue_dye",
	"red":          "minecraft:red_dye",
	"light_purple": "minecraft:magenta_dye",
	"yellow":       "minecraft:yellow_dye",
	"white":        "minecraft:white_dye",
}

// replaceHotbar 切到指定格并替换物品，block=true 等待变更完成。
func (bm *BotManager) replaceHotbar(slot resources_control.SlotID, itemName string, block bool) error {
	gi, err := bm.getGameInterface()
	if err != nil {
		return err
	}
	_ = gi.BotClick().ChangeSelectedHotbarSlot(slot)
	return gi.Replaceitem().ReplaceitemInInventory(
		"@s", game_interface.ReplacePathHotbarOnly,
		game_interface.ReplaceitemInfo{Name: itemName, Count: 1, MetaData: 0, Slot: slot},
		"", block,
	)
}

// PlaceSign 在目标坐标就地放置一个带文字的告示牌。
// 完整流程：清手持 → 放正确的告示牌 → 写文字 → 染色 → 发光 → 涂蜡。
// 直接在原本的告示牌方块上操作，不放辅助牌也不覆写，避免干扰方块流程。
func (bm *BotManager) PlaceSign(x, y, z int32, blockName, blockStates string, nbt map[string]any) error {
	gi, err := bm.getGameInterface()
	if err != nil {
		return err
	}

	pos := protocol.BlockPos{x, y, z}
	const slot = resources_control.SlotID(0)

	frontText := signTextOf(nbt, "FrontText")
	backText := signTextOf(nbt, "BackText")
	frontGlow := signGlowOf(nbt, "FrontText")
	backGlow := signGlowOf(nbt, "BackText")
	waxed := signIsWaxed(nbt)
	frontDye := signColorToDye(nbt, "FrontText")
	backDye := signColorToDye(nbt, "BackText")

	// 无文字 + 无发光 + 未涂蜡 → 纯装饰，直接 setblock 跳过
	if frontText == "" && backText == "" && !frontGlow && !backGlow && !waxed {
		return gi.SetBlock().SetBlock(pos, blockName, blockStates)
	}

	// 切槽 + 清空手持
	_ = gi.BotClick().ChangeSelectedHotbarSlot(slot)
	if err := bm.replaceHotbar(slot, "minecraft:air", false); err != nil {
		return fmt.Errorf("清空手持: %v", err)
	}

	// 传送到目标
	_ = gi.Commands().SendSettingsCommand(fmt.Sprintf("tp %d %d %d", x, y, z), true)
	_ = gi.Commands().AwaitChangesGeneral()

	// 同步本地位置，确保心跳发送的坐标与后续点击操作一致
	bm.mu.Lock()
	bm.pos = mgl32.Vec3{float32(x) + 0.5, float32(y) + 1.5, float32(z) + 0.5}
	bm.mu.Unlock()
	// 直接放置正确的告示牌
	if err := gi.SetBlock().SetBlock(pos, "minecraft:air", "[]"); err != nil {
		return fmt.Errorf("清空目标格: %v", err)
	}
	if err := gi.SetBlock().SetBlock(pos, blockName, blockStates); err != nil {
		return fmt.Errorf("放置告示牌: %v", err)
	}

	blockAction := game_interface.UseItemOnBlocks{
		HotbarSlotID: slot,
		BotPos:       mgl32.Vec3{float32(x) + 0.5, float32(y) + 1.5, float32(z) + 0.5},
		BlockPos:     pos,
		BlockName:    blockName,
		BlockStates:  utils.ParseBlockStatesString(blockStates),
	}

	// ── 第 1 步：写文字 ──
	nbtMap := map[string]any{}
	if ft := signCopyFace(nbt, "FrontText"); ft != nil {
		nbtMap["FrontText"] = ft
	}
	if bt := signCopyFace(nbt, "BackText"); bt != nil {
		nbtMap["BackText"] = bt
	}
	if len(nbtMap) > 0 {
		if err := gi.BotClick().ClickBlock(blockAction); err != nil {
			return fmt.Errorf("点击打开告示牌: %v", err)
		}
		if err := gi.Resources().WritePacket(&packet.BlockActorData{Position: pos, NBTData: nbtMap}); err != nil {
			return fmt.Errorf("写告示牌文字: %v", err)
		}
	}

	// ── 第 2 步：染色（正面/背面分别用对应染料点击） ──
	for _, op := range [][2]any{
		{frontDye, 0},
		{backDye, 1},
	} {
		dye, _ := op[0].(string)
		side, _ := op[1].(int)
		if dye == "" {
			continue
		}
		if err := bm.replaceHotbar(slot, dye, true); err != nil {
			return fmt.Errorf("取染料 %s: %v", dye, err)
		}
		cx := x
		if side != 0 {
			cx++
		}
		if _, err := gi.BotClick().ClickBlockWithPosition(
			blockAction,
			mgl32.Vec3{float32(cx) + 0.5, float32(y), float32(z) + 0.5},
		); err != nil {
			return fmt.Errorf("染色点击 (%s): %v", dye, err)
		}
	}

	// ── 第 3 步：发光 ──
	if frontGlow || backGlow {
		if err := bm.replaceHotbar(slot, "minecraft:glow_ink_sac", true); err != nil {
			return fmt.Errorf("取发光墨囊: %v", err)
		}
		for _, op := range [][2]any{
			{frontGlow, 0},
			{backGlow, 1},
		} {
			glow, _ := op[0].(bool)
			side, _ := op[1].(int)
			if !glow {
				continue
			}
			cx := x
			if side != 0 {
				cx++
			}
			if _, err := gi.BotClick().ClickBlockWithPosition(
				blockAction,
				mgl32.Vec3{float32(cx) + 0.5, float32(y), float32(z) + 0.5},
			); err != nil {
				return fmt.Errorf("发光点击: %v", err)
			}
		}
	}

	// ── 第 4 步：涂蜡 ──
	if waxed {
		if err := bm.replaceHotbar(slot, "minecraft:honeycomb", true); err != nil {
			return fmt.Errorf("取蜜脾: %v", err)
		}
		if err := gi.BotClick().ClickBlock(blockAction); err != nil {
			return fmt.Errorf("涂蜡点击: %v", err)
		}
	}

	return nil
}
