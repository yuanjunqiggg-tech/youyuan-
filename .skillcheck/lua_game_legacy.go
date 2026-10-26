package main

import (
	"bot-apk/building"
	"context"
	"encoding/json"
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"sort"
	"strings"
	"time"
)

type yyLegacyImportJob struct {
	done chan struct{}
	err  error
}

// Keep the original plugin API names connected to real host implementations.
func registerLegacyGameAPI(L *lua.LState) {
	g := L.GetGlobal("game")
	L.SetField(g, "sendCommandWithResp", L.NewFunction(func(L *lua.LState) int {
		cmd := L.CheckString(1)
		channel, e := normalizeCommandChannel(L.OptString(2, "ai"))
		if e != nil {
			L.RaiseError("%v", e)
			return 0
		}
		seconds := L.OptNumber(3, 5)
		if seconds <= 0 {
			L.RaiseError("timeout must be positive")
			return 0
		}
		bm := abot(L)
		value, e := yyLuaWaitValue(L, func(_ context.Context) (map[string]any, error) {
			resp, timeout, err := bm.SendCommandCh(channel, cmd, time.Duration(float64(seconds)*float64(time.Second)))
			if err != nil {
				return nil, err
			}
			if timeout || resp == nil {
				return nil, fmt.Errorf("等待服务器回包超时，命令可能已执行")
			}
			return packetToMap(resp), nil
		})
		if e != nil {
			L.RaiseError("sendCommandWithResp: %v", e)
			return 0
		}
		L.Push(goToLua(L, value))
		return 1
	}))
	L.SetField(g, "getOnlinePlayers", L.NewFunction(func(L *lua.LState) int {
		names, e := yyLuaWait1(L, abot(L).GetTarget, "@a")
		if e != nil {
			L.RaiseError("getOnlinePlayers: %v", e)
			return 0
		}
		L.Push(goToLua(L, names))
		return 1
	}))
	L.SetField(g, "importBlocks", L.NewFunction(func(L *lua.LState) int {
		list := L.CheckTable(1)
		options := L.OptTable(2, L.NewTable())
		bm := abot(L)
		if bm == nil {
			L.RaiseError("importBlocks: 游戏账号未连接")
			return 0
		}
		bm.mu.Lock()
		connected := bm.connected
		bm.mu.Unlock()
		if !connected {
			L.RaiseError("importBlocks: 游戏账号未连接")
			return 0
		}
		data, e := yyLegacyImportData(list)
		if e != nil {
			L.RaiseError("importBlocks: %v", e)
			return 0
		}
		number := func(k string, d int) int {
			v := options.RawGetString(k)
			if v == lua.LNil {
				return d
			}
			return int(lua.LVAsNumber(v))
		}
		dimension := lua.LVAsString(options.RawGetString("dimension"))
		if dimension == "" {
			dimension = "overworld"
		}
		if dimension != "overworld" && dimension != "nether" && dimension != "the_end" {
			L.RaiseError("importBlocks: dimension 应为 overworld/nether/the_end")
			return 0
		}
		region := number("region_mode", 1)
		if region < 1 || region > 3 {
			L.RaiseError("importBlocks: region_mode 应为1/2/3")
			return 0
		}
		parent := L.Context()
		if parent == nil {
			parent = context.Background()
		}
		if c := L.Context(); c != nil {
			if bc, ok := c.Value(luaBotCtxKey{}).(*luaBotCtx); ok && bc.parent != nil && bc.parent.Done() != nil {
				parent = bc.parent
			}
		}
		ctx, cancel := context.WithCancel(parent)
		task := &building.ImportTask{Data: data, BaseX: number("x", 0), BaseY: number("y", 64), BaseZ: number("z", 0), Speed: number("speed", 0), RegionMode: region, Dimension: dimension, StopCh: ctx.Done(), StopOnError: true, ImportCommands: true, SkipPreClear: true}
		if task.Speed < 0 {
			task.Speed = 0
		}
		task.Teleport = func(x, y, z int) error { return bm.TP(x, y, z) }
		if options.RawGetString("anti_loss") == lua.LTrue {
			task.ChunkLoaded = func(cx, cz int) bool {
				select {
				case <-ctx.Done():
					return false
				default:
				}
				prefix := ""
				if dimension != "overworld" {
					prefix = "execute in " + dimension + " run "
				}
				_, timeout, err := bm.SendWSCommandWithTimeout(prefix+fmt.Sprintf("testforblock %d 1 %d minecraft:air", cx*16+8, cz*16+8), time.Second)
				return !timeout && err == nil
			}
		}
		dimensionPrefix := ""
		if dimension != "overworld" {
			dimensionPrefix = "execute in " + dimension + " run "
		}
		task.SetBlock = func(x, y, z int32, name, states string) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			command := fmt.Sprintf("setblock %d %d %d %s", x, y, z, normalizeBlockName(name))
			if states != "" && states != "[]" {
				command += " " + normalizeBlockStates(name, states)
			}
			return bm.SendBlockCmd(dimensionPrefix + command)
		}
		var nbtErr error
		task.OnNBT = func(x, y, z int32, nbt map[string]any, name, states string) {
			if nbtErr != nil {
				return
			}
			nbtErr = bm.PlaceNBTBlock(int(x), int(y), int(z), name, states, nbt)
			if nbtErr != nil {
				cancel()
			}
		}
		job := &yyLegacyImportJob{done: make(chan struct{})}
		releaseSpatial := yyBeginSpatialJob(bm, "plugin-importBlocks")
		go func() {
			defer releaseSpatial()
			defer close(job.done)
			defer cancel()
			defer func() {
				if p := recover(); p != nil {
					job.err = fmt.Errorf("importBlocks: %v", p)
				}
			}()
			job.err = task.Run()
			if nbtErr != nil {
				job.err = nbtErr
			}
		}()
		ud := L.NewUserData()
		ud.Value = job
		L.Push(ud)
		return 1
	}))
	L.SetField(g, "importWait", L.NewFunction(func(L *lua.LState) int {
		job, ok := L.CheckUserData(1).Value.(*yyLegacyImportJob)
		if !ok {
			L.RaiseError("importWait: 不是本机导入任务")
			return 0
		}
		ctx := L.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		select {
		case <-ctx.Done():
			L.RaiseError("importWait: %v", ctx.Err())
			return 0
		case <-job.done:
		}
		if job.err != nil {
			L.RaiseError("importWait: %v", job.err)
			return 0
		}
		L.Push(lua.LTrue)
		return 1
	}))
}

func yyLegacyImportData(list *lua.LTable) (*building.StructureData, error) {
	data := &building.StructureData{Blocks: map[uint64]uint32{}}
	var first error
	list.ForEach(func(_, v lua.LValue) {
		if first != nil {
			return
		}
		row, ok := v.(*lua.LTable)
		if !ok {
			first = fmt.Errorf("方块条目必须是table")
			return
		}
		coord := func(k string) int { return int(lua.LVAsNumber(row.RawGetString(k))) }
		x, y, z := coord("x"), coord("y"), coord("z")
		if x < 0 || y < 0 || z < 0 || x >= 1<<20 || y >= 1<<20 || z >= 1<<20 {
			first = fmt.Errorf("结构相对坐标超出本机位置编码范围")
			return
		}
		name := lua.LVAsString(row.RawGetString("name"))
		if strings.TrimSpace(name) == "" {
			first = fmt.Errorf("方块名称为空")
			return
		}
		states := lua.LVAsString(row.RawGetString("states"))
		if t, ok := row.RawGetString("states").(*lua.LTable); ok {
			var e error
			states, e = yyLegacyBlockStates(t)
			if e != nil {
				first = e
				return
			}
		}
		var nbt map[string]any
		if t, ok := row.RawGetString("nbt").(*lua.LTable); ok {
			nbt, _ = luaToGo(t).(map[string]any)
		}
		data.Set(x, y, z, &building.BlockInfo{Name: name, States: states, NBT: nbt})
		if x+1 > data.SizeX {
			data.SizeX = x + 1
		}
		if y+1 > data.SizeY {
			data.SizeY = y + 1
		}
		if z+1 > data.SizeZ {
			data.SizeZ = z + 1
		}
	})
	if first != nil {
		return nil, first
	}
	if data.SizeX == 0 {
		return nil, fmt.Errorf("方块列表为空")
	}
	return data, nil
}

func yyLegacyBlockStates(t *lua.LTable) (string, error) {
	values := map[string]string{}
	var err error
	t.ForEach(func(k, v lua.LValue) {
		if err != nil {
			return
		}
		key, ok := k.(lua.LString)
		if !ok {
			err = fmt.Errorf("方块状态键必须是字符串")
			return
		}
		switch v.(type) {
		case lua.LString, lua.LNumber, lua.LBool:
		default:
			err = fmt.Errorf("方块状态 %s 必须是字符串、数字或布尔值", key)
			return
		}
		encoded, e := json.Marshal(luaToGo(v))
		if e != nil {
			err = e
			return
		}
		values[string(key)] = string(encoded)
	})
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		encoded, _ := json.Marshal(k)
		parts = append(parts, string(encoded)+"="+values[k])
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}
