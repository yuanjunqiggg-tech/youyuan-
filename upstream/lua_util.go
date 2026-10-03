package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// ---- 后台定时任务 ----

// schedJob 一个后台定时任务（插件注册 util.schedule/setTimeout）。
type schedJob struct {
	plugin   string
	handler  string
	interval time.Duration // 重复间隔；0 = 一次性
	oneshot  bool
	bm       *BotManager
	cancel   chan struct{}
}

var (
	schedMu   sync.Mutex
	schedNext int64
	schedJobs = map[int64]*schedJob{}
)

// maxSchedIntervalSec 定时任务间隔/延时的上限（1 年）。防 duration 溢出成负数导致 panic。
const maxSchedIntervalSec = 365 * 24 * 3600

// newSchedJob 注册并启动一个定时任务，返回 id。
func newSchedJob(plugin, handler string, interval time.Duration, oneshot bool, bm *BotManager) int64 {
	schedMu.Lock()
	schedNext++
	id := schedNext
	schedJobs[id] = &schedJob{plugin: plugin, handler: handler, interval: interval, oneshot: oneshot, bm: bm, cancel: make(chan struct{})}
	schedMu.Unlock()
	go runSchedLoop(id)
	return id
}

func runSchedLoop(id int64) {
	schedMu.Lock()
	job := schedJobs[id]
	schedMu.Unlock()
	if job == nil {
		return
	}
	// 兜底 recover：任何 panic 都不能杀死整个进程（Go 中未恢复的 panic 会让 App 崩溃）。
	defer func() {
		if r := recover(); r != nil {
			debugLog("[定时] 定时任务崩溃 id=%d err=%v", id, r)
		}
		schedMu.Lock()
		delete(schedJobs, id)
		schedMu.Unlock()
	}()
	if job.oneshot {
		timer := time.NewTimer(job.interval)
		defer timer.Stop()
		select {
		case <-job.cancel:
		case <-timer.C:
			runScheduledHandler(job)
		}
		return
	}
	ticker := time.NewTicker(job.interval)
	defer ticker.Stop()
	for {
		select {
		case <-job.cancel:
			return
		case <-ticker.C:
			runScheduledHandler(job)
		}
	}
}

// runScheduledHandler 重跑插件源码并调用 handler（复用 callEventHandler）。
func runScheduledHandler(job *schedJob) {
	defer func() { recover() }()
	callEventHandler(job.bm, &luaEventH{plugin: job.plugin, handler: job.handler})
}

// cancelSchedJob 取消指定 id 的定时任务。
func cancelSchedJob(id int64) bool {
	schedMu.Lock()
	job := schedJobs[id]
	schedMu.Unlock()
	if job == nil {
		return false
	}
	select {
	case <-job.cancel:
	default:
		close(job.cancel)
	}
	return true
}

// RemovePluginSchedules 取消某插件的全部定时任务（插件停止时调用）。
func RemovePluginSchedules(plugin string) {
	schedMu.Lock()
	var toCancel []*schedJob
	for _, job := range schedJobs {
		if job.plugin == plugin {
			toCancel = append(toCancel, job)
		}
	}
	schedMu.Unlock()
	for _, job := range toCancel {
		select {
		case <-job.cancel:
		default:
			close(job.cancel)
		}
	}
}

// registerUtilAPI 注册纯工具函数到全局 util 表（时间/文件/JSON/文本/数字/坐标）。
func registerUtilAPI(L *lua.LState) {
	u := L.NewTable()
	L.SetGlobal("util", u)

	// ---- 时间 ----
	L.SetField(u, "now", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(time.Now().Format("2006-01-02 15:04:05")))
		return 1
	}))
	L.SetField(u, "timestamp", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Now().Unix()))
		return 1
	}))
	// util.time_fmt(ts?) → 格式化本地时间；缺省取当前时间
	L.SetField(u, "time_fmt", L.NewFunction(func(L *lua.LState) int {
		ts := int64(L.OptNumber(1, lua.LNumber(time.Now().Unix())))
		L.Push(lua.LString(time.Unix(ts, 0).Format("2006-01-02 15:04:05")))
		return 1
	}))

	// ---- 文件 / JSON ----
	// util.json_load(path) → table / nil
	L.SetField(u, "json_load", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		data, err := os.ReadFile(path)
		if err != nil {
			L.RaiseError("json_load: %v", err)
		}
		var v any
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.UseNumber()
		if err := dec.Decode(&v); err != nil {
			L.RaiseError("json_load: %v", err)
		}
		L.Push(goToLua(L, v))
		return 1
	}))
	// util.json_write(path, data) → true（自动建目录）
	L.SetField(u, "json_write", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		data := luaToGo(L.Get(2))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			L.RaiseError("json_write: %v", err)
		}
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			L.RaiseError("json_write: %v", err)
		}
		if err := os.WriteFile(path, b, 0644); err != nil {
			L.RaiseError("json_write: %v", err)
		}
		L.Push(lua.LTrue)
		return 1
	}))
	// util.read_file(path) → string
	L.SetField(u, "read_file", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		data, err := os.ReadFile(path)
		if err != nil {
			L.RaiseError("read_file: %v", err)
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	// util.write_file(path, content) → true
	L.SetField(u, "write_file", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		content := L.CheckString(2)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			L.RaiseError("write_file: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			L.RaiseError("write_file: %v", err)
		}
		L.Push(lua.LTrue)
		return 1
	}))

	// ---- 文本 / 颜色 ----
	// util.strip_color(text) → 去掉 §/& 颜色码
	L.SetField(u, "strip_color", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		var b strings.Builder
		for i := 0; i < len(s); i++ {
			if s[i] == '§' || s[i] == '&' {
				// 跳过颜色/格式码（& 后跟一个字符；§ 可能是 UTF-8 后跟一个字节码）
				if i+1 < len(s) {
					i++
					continue
				}
				continue
			}
			b.WriteByte(s[i])
		}
		L.Push(lua.LString(b.String()))
		return 1
	}))
	// util.split(str, sep) → 字符串数组
	L.SetField(u, "split", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		sep := L.CheckString(2)
		parts := strings.Split(s, sep)
		t := L.NewTable()
		for i, p := range parts {
			t.RawSetInt(i+1, lua.LString(p))
		}
		L.Push(t)
		return 1
	}))
	// util.join(tbl, sep) → string
	L.SetField(u, "join", L.NewFunction(func(L *lua.LState) int {
		t := L.CheckTable(1)
		sep := L.CheckString(2)
		var parts []string
		t.ForEach(func(_, v lua.LValue) { parts = append(parts, lua.LVAsString(v)) })
		L.Push(lua.LString(strings.Join(parts, sep)))
		return 1
	}))
	L.SetField(u, "trim", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(strings.TrimSpace(L.CheckString(1))))
		return 1
	}))

	// ---- 数字 ----
	L.SetField(u, "floor", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(math.Floor(float64(L.CheckNumber(1)))))
		return 1
	}))
	L.SetField(u, "ceil", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(math.Ceil(float64(L.CheckNumber(1)))))
		return 1
	}))
	L.SetField(u, "round", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(math.Round(float64(L.CheckNumber(1)))))
		return 1
	}))

	// ---- 坐标 ----
	// util.pos("x y z") → {x,y,z}；缺项补 0
	L.SetField(u, "pos", L.NewFunction(func(L *lua.LState) int {
		s := strings.Fields(L.CheckString(1))
		xyz := []float64{0, 0, 0}
		for i := 0; i < len(s) && i < 3; i++ {
			if f, err := strconv.ParseFloat(s[i], 64); err == nil {
				xyz[i] = f
			}
		}
		t := L.NewTable()
		t.RawSetString("x", lua.LNumber(xyz[0]))
		t.RawSetString("y", lua.LNumber(xyz[1]))
		t.RawSetString("z", lua.LNumber(xyz[2]))
		L.Push(t)
		return 1
	}))

	// ---- 后台定时任务 ----
	// util.schedule(intervalSec, handlerName) → id：每 intervalSec 调用一次 handler
	L.SetField(u, "schedule", L.NewFunction(func(L *lua.LState) int {
		if L.GetGlobal("__RE_RUN__") == lua.LTrue { // 重跑上下文跳过，避免重复注册
			L.Push(lua.LNumber(0))
			return 1
		}
		interval := float64(L.CheckNumber(1))
		// 校验间隔：≤0 会让 time.NewTicker panic（未恢复的 panic 会杀死整个进程）；
		// 太小会被转成 0 duration 同样 panic，过大则 duration 溢出。钳制在 [0.001s, 1年]。
		if interval < 0.001 || interval > maxSchedIntervalSec {
			L.RaiseError("schedule: 间隔需在 0.001~%d 秒之间", maxSchedIntervalSec)
		}
		handler := L.CheckString(2)
		plugin := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		id := newSchedJob(plugin, handler, time.Duration(interval*float64(time.Second)), false, schedBot(L))
		L.Push(lua.LNumber(id))
		return 1
	}))
	// util.setTimeout(sec, handlerName) → id：延时 sec 后调用一次 handler
	L.SetField(u, "setTimeout", L.NewFunction(func(L *lua.LState) int {
		if L.GetGlobal("__RE_RUN__") == lua.LTrue { // 重跑上下文跳过，避免重复注册
			L.Push(lua.LNumber(0))
			return 1
		}
		sec := float64(L.CheckNumber(1))
		// 0 允许（立即触发）；负数或过大（duration 溢出）非法。
		if sec < 0 || sec > maxSchedIntervalSec {
			L.RaiseError("setTimeout: 延时需在 0~%d 秒之间", maxSchedIntervalSec)
		}
		handler := L.CheckString(2)
		plugin := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		id := newSchedJob(plugin, handler, time.Duration(sec*float64(time.Second)), true, schedBot(L))
		L.Push(lua.LNumber(id))
		return 1
	}))
	// util.cancel(id) → 取消定时任务
	L.SetField(u, "cancel", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(cancelSchedJob(int64(L.CheckNumber(1)))))
		return 1
	}))
}

// schedBot 返回定时任务默认使用的机器人（当前活跃，缺省取舰队首个）。
func schedBot(L *lua.LState) *BotManager {
	if b := abot(L); b != nil {
		return b
	}
	if bots := GetFleetBots(); len(bots) > 0 {
		return bots[0]
	}
	return nil
}
