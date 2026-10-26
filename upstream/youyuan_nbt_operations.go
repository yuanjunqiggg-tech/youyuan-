package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/OmineDev/flowers-for-machines/mapping"
	nbt_parser_block "github.com/OmineDev/flowers-for-machines/nbt_parser/block"
	"github.com/google/uuid"
)

var yyNBTMethods = map[string]string{
	"building.placeSkull":      "x,y,z,SkullType(0..6),states?；异步通过真实头颅物品放置和结构缓存复制；返回job_id，使用nbtJob读取实际SkullType。会使用NBT工作台并移动机器人。",
	"building.nbtCapabilities": "无参数；实际底层支持的方块/特殊物品及制作行为目录。支持表不是全服实测保证。",
	"building.normalizeNBT":    "NBT对象；按Bedrock物品字段恢复Count/Slot/ench/Damage等NBT类型，拒绝越界，保留未知字段。",
	"building.containerData":   "容器名,Items数组,options?；生成经过类型/槽位检查的方块NBT，不连接游戏。options支持custom_name。",
	"building.placeContainer":  "x,y,z,容器名,Items数组,options?；异步使用真实NBT工作台制作、铁砧改名、附魔及结构复制；返回job_id，用nbtJob查验证结果。会移动机器人、初始化时清背包。",
	"building.nbtJob":          "job_id；读取当前账号的NBT任务状态；submitted与目标结构读回verified区分。任务内存留存，重启不恢复。",
	"building.planRegion":      "{x,y,z},{sx,sy,sz}；按16x128x16分块，返回不重叠完整覆盖的单元。规划不等于执行。",
	"building.exportRegion":    "起点,尺寸,{workers:1}?；异步按区块读取并保存mcstructure。只使用已配置的同服Fleet机器人，失败不输出缺块文件。返回job_id，用nbtJob查询本机文件路径。",
}

func init() {
	for k, v := range yyNBTMethods {
		yyExtraMethods[k] = v
	}
}

// JSON and Lua numbers have no NBT tag width. Restore schema-defined widths
// only; never cast all item tag numbers to int16 (colors and timestamps differ).
func yyNBTBoundedInteger(v any, lo, hi int64) (int64, error) {
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case float32:
		n = float64(x)
	case int:
		n = float64(x)
	case int8:
		n = float64(x)
	case uint8:
		n = float64(x)
	case int16:
		n = float64(x)
	case uint16:
		n = float64(x)
	case int32:
		n = float64(x)
	case uint32:
		n = float64(x)
	case int64:
		if x < lo || x > hi {
			return 0, fmt.Errorf("整数越界")
		}
		return x, nil
	case uint64:
		if x > uint64(hi) {
			return 0, fmt.Errorf("整数越界")
		}
		return int64(x), nil
	case json.Number:
		var e error
		n, e = x.Float64()
		if e != nil {
			return 0, e
		}
	default:
		return 0, fmt.Errorf("需要整数，得到%T", v)
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n || n < float64(lo) || n > float64(hi) {
		return 0, fmt.Errorf("整数应在%d..%d", lo, hi)
	}
	return int64(n), nil
}
func yyNormalizeNBT(v any) (any, error) { return yyNormalizeNBTAt(v, "", 0) }
func yyNormalizeNBTAt(v any, path string, depth int) (any, error) {
	if depth > 64 {
		return nil, fmt.Errorf("NBT嵌套超过64层")
	}
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		item := x["Name"] != nil && (x["Count"] != nil || x["Slot"] != nil)
		for k, value := range x {
			if value == nil {
				return nil, fmt.Errorf("%s.%s: NBT不支持null", path, k)
			}
			p := path + "." + k
			var a any
			var e error
			if item && (k == "Count" || k == "Slot") {
				var n int64
				n, e = yyNBTBoundedInteger(value, 0, 255)
				a = byte(n)
			} else if k == "facing" {
				var n int64
				n, e = yyNBTBoundedInteger(value, 0, 5)
				a = byte(n)
			} else if item && k == "Damage" || strings.Contains(path, ".ench[") && (k == "id" || k == "lvl") {
				var n int64
				n, e = yyNBTBoundedInteger(value, -32768, 32767)
				a = int16(n)
			} else if k == "Damage" && strings.HasSuffix(path, ".tag") {
				var n int64
				n, e = yyNBTBoundedInteger(value, 0, math.MaxInt32)
				a = int32(n)
			} else {
				a, e = yyNormalizeNBTAt(value, p, depth+1)
			}
			if e != nil {
				return nil, fmt.Errorf("%s: %w", p, e)
			}
			out[k] = a
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, value := range x {
			a, e := yyNormalizeNBTAt(value, fmt.Sprintf("%s[%d]", path, i), depth+1)
			if e != nil {
				return nil, e
			}
			out[i] = a
		}
		return out, nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, fmt.Errorf("NBT数字非有限")
		}
		if math.Trunc(x) == x && x >= math.MinInt32 && x <= math.MaxInt32 {
			return int32(x), nil
		}
		return x, nil
	case json.Number:
		if n, e := x.Int64(); e == nil {
			if n >= math.MinInt32 && n <= math.MaxInt32 {
				return int32(n), nil
			}
			return n, nil
		}
		n, e := x.Float64()
		if e != nil {
			return nil, e
		}
		return yyNormalizeNBTAt(n, path, depth+1)
	case int:
		if int64(x) >= math.MinInt32 && int64(x) <= math.MaxInt32 {
			return int32(x), nil
		}
		return int64(x), nil
	case nil:
		return nil, fmt.Errorf("NBT不支持null")
	default:
		return v, nil
	}
}
func yyContainerNBT(name string, rows []any, options map[string]any) (map[string]any, error) {
	name = "minecraft:" + normalizeBlockName(strings.ToLower(strings.TrimSpace(name)))
	if mapping.SupportBlocksPool[name] != mapping.SupportNBTBlockTypeContainer {
		return nil, fmt.Errorf("未支持该容器：%s", name)
	}
	capacity := 27
	switch {
	case strings.Contains(name, "furnace") || strings.Contains(name, "smoker"):
		capacity = 3
	case name == "minecraft:hopper":
		capacity = 5
	case name == "minecraft:dropper" || name == "minecraft:dispenser":
		capacity = 9
	}
	if len(rows) > capacity {
		return nil, fmt.Errorf("%s有%d个槽，不能静默截断%d项", name, capacity, len(rows))
	}
	used := map[int]bool{}
	items := make([]any, 0, len(rows))
	for i, v := range rows {
		item, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Items[%d]须为物品NBT对象", i)
		}
		copyItem := map[string]any{}
		for k, value := range item {
			copyItem[k] = value
		}
		nm, ok := copyItem["Name"].(string)
		if !ok || strings.TrimSpace(nm) == "" {
			return nil, fmt.Errorf("Items[%d].Name不能为空", i)
		}
		copyItem["Name"] = "minecraft:" + normalizeBlockName(strings.ToLower(strings.TrimSpace(nm)))
		if copyItem["Slot"] == nil {
			copyItem["Slot"] = i
		}
		if copyItem["Count"] == nil {
			copyItem["Count"] = 1
		}
		if copyItem["Damage"] == nil {
			copyItem["Damage"] = 0
		}
		slot, e := yyNBTBoundedInteger(copyItem["Slot"], 0, int64(capacity-1))
		if e != nil {
			return nil, e
		}
		if used[int(slot)] {
			return nil, fmt.Errorf("重复Slot %d", slot)
		}
		used[int(slot)] = true
		if _, e = yyNBTBoundedInteger(copyItem["Count"], 1, 255); e != nil {
			return nil, e
		}
		normal, e := yyNormalizeNBT(copyItem)
		if e != nil {
			return nil, e
		}
		items = append(items, normal)
	}
	out := map[string]any{"Items": items}
	if strings.Contains(name, "shulker") {
		facing := any(1)
		if v, ok := options["facing"]; ok {
			facing = v
		}
		n, e := yyNBTBoundedInteger(facing, 0, 5)
		if e != nil {
			return nil, fmt.Errorf("潜影箱朝向: %w", e)
		}
		out["facing"] = byte(n)
	}
	if s, ok := options["custom_name"].(string); ok && s != "" {
		out["CustomName"] = s
	}
	return out, nil
}

var yyBlockOperationLocks sync.Map

func yyBlockOperationLock(bm *BotManager) *sync.Mutex {
	v, _ := yyBlockOperationLocks.LoadOrStore(bm, &sync.Mutex{})
	return v.(*sync.Mutex)
}

type yyNBTJob struct {
	mu               sync.Mutex
	Bot              *BotManager
	ID, State, Error string
	Created          time.Time
	Result           any
}

var yyNBTJobs = struct {
	sync.Mutex
	Items map[string]*yyNBTJob
}{Items: map[string]*yyNBTJob{}}

func yyNBTJobSnapshot(j *yyNBTJob) map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	return map[string]any{"job_id": j.ID, "state": j.State, "error": j.Error, "completed": j.State == "done" || j.State == "failed", "result": j.Result, "created_at": j.Created}
}
func yyStartNBTJob(bm *BotManager, run func() (any, error)) (any, error) {
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("游戏账号未连接")
	}
	j := &yyNBTJob{Bot: bm, ID: uuid.NewString(), State: "queued", Created: time.Now()}
	yyNBTJobs.Lock()
	for id, old := range yyNBTJobs.Items {
		old.mu.Lock()
		finished := old.State == "done" || old.State == "failed"
		old.mu.Unlock()
		if finished && time.Since(old.Created) > time.Hour {
			delete(yyNBTJobs.Items, id)
		}
	}
	if len(yyNBTJobs.Items) >= yyObservationSettings().AsyncTickets {
		yyNBTJobs.Unlock()
		return nil, fmt.Errorf("NBT任务留存已满，请读取并consume已完成任务")
	}
	yyNBTJobs.Items[j.ID] = j
	yyNBTJobs.Unlock()
	releaseSpatial := yyBeginSpatialJob(bm, "nbt-job")
	go func() {
		defer releaseSpatial()
		defer func() {
			if p := recover(); p != nil {
				j.mu.Lock()
				j.State = "failed"
				j.Error = fmt.Sprint(p)
				j.mu.Unlock()
			}
		}()
		j.mu.Lock()
		j.State = "running"
		j.mu.Unlock()
		result, e := run()
		j.mu.Lock()
		defer j.mu.Unlock()
		j.Result = result
		if e != nil {
			j.State = "failed"
			j.Error = e.Error()
		} else {
			j.State = "done"
		}
	}()
	return map[string]any{"job_id": j.ID, "accepted": true, "completed": false}, nil
}
func yyNBTCall(bm *BotManager, method string, args []any) (any, error) {
	switch method {
	case "building.placeSkull":
		if len(args) < 4 {
			return nil, fmt.Errorf("x,y,z,SkullType,states?")
		}
		var pos [3]int
		for i := 0; i < 3; i++ {
			n, e := yyNBTBoundedInteger(args[i], math.MinInt32, math.MaxInt32)
			if e != nil {
				return nil, e
			}
			pos[i] = int(n)
		}
		typ, e := yyNBTBoundedInteger(args[3], 0, 6)
		if e != nil {
			return nil, fmt.Errorf("SkullType需要0..6整数")
		}
		states := `["facing_direction"=1]`
		if len(args) > 4 {
			var ok bool
			states, ok = args[4].(string)
			if !ok {
				return nil, fmt.Errorf("states需要方块状态字符串")
			}
		}
		return yyStartNBTJob(bm, func() (any, error) {
			if e := bm.PlaceNBTBlock(pos[0], pos[1], pos[2], "minecraft:skull", states, map[string]any{"SkullType": byte(typ)}); e != nil {
				return nil, e
			}
			actual, e := bm.GetBlockData(pos[0], pos[1], pos[2])
			if e != nil {
				return map[string]any{"submitted": true, "verified": false}, e
			}
			n, e := yyNBTBoundedInteger(actual.NBT["SkullType"], 0, 6)
			if e != nil {
				for i, name := range nbt_parser_block.SkullNames {
					if normalizeBlockName(actual.Name) == normalizeBlockName(name) {
						n = int64(i)
						e = nil
						break
					}
				}
			}
			verified := e == nil && n == typ
			result := map[string]any{"submitted": true, "verified": verified, "actual": actual}
			if !verified {
				return result, fmt.Errorf("头颅类型读回不一致")
			}
			return result, nil
		})
	case "building.nbtCapabilities":
		blocks := []string{}
		for k := range mapping.SupportBlocksPool {
			blocks = append(blocks, k)
		}
		sort.Strings(blocks)
		items := []string{}
		for k := range mapping.SupportItemsPool {
			items = append(items, k)
		}
		sort.Strings(items)
		return map[string]any{"blocks": blocks, "complex_items": items, "ordinary_item_enchant_and_rename": true, "behaviors": []string{"workspace_platform", "open_inventory", "open_container", "close_container", "inventory_move", "container_insert", "container_take", "anvil_rename", "command_enchant", "book_pages", "banner_loom", "shield_pattern", "armor_trim", "item_frame", "lectern_book", "jukebox_disc", "brewing_stand_slots", "crafter_slots", "nested_container_pick", "structure_cache_and_copy"}, "arbitrary_nbt_write": false, "clear_bot_inventory_on_workspace_init": true, "verified_on_every_server": false}, nil
	case "building.normalizeNBT":
		if len(args) != 1 {
			return nil, fmt.Errorf("需要NBT对象")
		}
		return yyNormalizeNBT(args[0])
	case "building.containerData", "building.placeContainer":
		at := 0
		var pos [3]int
		if method == "building.placeContainer" {
			if len(args) < 5 {
				return nil, fmt.Errorf("x,y,z,name,Items,options?")
			}
			for i := 0; i < 3; i++ {
				n, e := yyNBTBoundedInteger(args[i], math.MinInt32, math.MaxInt32)
				if e != nil {
					return nil, e
				}
				pos[i] = int(n)
			}
			at = 3
		}
		name, e := yySDKString(args, at)
		if e != nil {
			return nil, e
		}
		if len(args) <= at+1 {
			return nil, fmt.Errorf("需要Items数组")
		}
		rows, ok := args[at+1].([]any)
		if empty, isMap := args[at+1].(map[string]any); isMap && len(empty) == 0 {
			rows = []any{}
			ok = true
		}
		if !ok {
			return nil, fmt.Errorf("需要Items数组")
		}
		options := map[string]any{}
		if len(args) > at+2 {
			options, ok = args[at+2].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("options须为对象")
			}
		}
		data, e := yyContainerNBT(name, rows, options)
		if e != nil {
			return nil, e
		}
		if method == "building.containerData" {
			return data, nil
		}
		return yyStartNBTJob(bm, func() (any, error) {
			if e := bm.PlaceNBTBlock(pos[0], pos[1], pos[2], name, "", data); e != nil {
				return nil, e
			}
			actual, e := bm.GetBlockData(pos[0], pos[1], pos[2])
			if e != nil {
				return map[string]any{"submitted": true, "verified": false}, fmt.Errorf("已提交，但目标读回失败：%w", e)
			}
			verified := normalizeBlockName(actual.Name) == normalizeBlockName(name) && yyNBTContains(actual.NBT, data)
			result := map[string]any{"submitted": true, "verified": verified, "actual": actual}
			if !verified {
				return result, fmt.Errorf("目标NBT读回与请求不一致")
			}
			return result, nil
		})
	case "building.nbtJob":
		id, e := yySDKString(args, 0)
		if e != nil {
			return nil, e
		}
		yyNBTJobs.Lock()
		j := yyNBTJobs.Items[id]
		if j == nil || j.Bot != bm {
			yyNBTJobs.Unlock()
			return nil, fmt.Errorf("当前账号无此任务")
		}
		v := yyNBTJobSnapshot(j)
		if len(args) > 1 && args[1] == true && v["completed"] == true {
			delete(yyNBTJobs.Items, id)
		}
		yyNBTJobs.Unlock()
		return v, nil
	case "building.planRegion":
		p, s, e := yyRegionArgs(args)
		if e != nil {
			return nil, e
		}
		return yyRegionPlan(p, s), nil
	case "building.exportRegion":
		return yyExportRegionJob(bm, args)
	}
	return nil, fmt.Errorf("未知NBT接口")
}

// Comparison checks all requested data, allowing extra server-generated fields.
func yyNBTContains(actual, wanted any) bool {
	switch w := wanted.(type) {
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range w {
			if !yyNBTContains(a[k], v) {
				return false
			}
		}
		return true
	case []any:
		a, ok := actual.([]any)
		if !ok || len(a) != len(w) {
			return false
		}
		if len(w) > 0 {
			if m, ok := w[0].(map[string]any); ok && m["Slot"] != nil {
				bySlot := map[int64]any{}
				for _, v := range a {
					m, ok := v.(map[string]any)
					if !ok {
						return false
					}
					n, e := yyNBTBoundedInteger(m["Slot"], 0, 255)
					if e != nil {
						return false
					}
					bySlot[n] = m
				}
				for _, v := range w {
					m := v.(map[string]any)
					n, _ := yyNBTBoundedInteger(m["Slot"], 0, 255)
					if !yyNBTContains(bySlot[n], m) {
						return false
					}
				}
				return true
			}
		}
		for i, v := range w {
			if !yyNBTContains(a[i], v) {
				return false
			}
		}
		return true
	default:
		aa, e1 := json.Marshal(actual)
		bb, e2 := json.Marshal(wanted)
		return e1 == nil && e2 == nil && string(aa) == string(bb)
	}
}
