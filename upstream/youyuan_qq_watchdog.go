package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// 掉线自愈守护
//
// 用户的核心抱怨：「这个 QQ 号一直在掉」「经常登录容易掉」。
//
// 掉线的真实形态（已用生产日志确认）：
//   - NapCat 的 QQ 进程还活着，但登录态没了；
//   - 未登录时 NapCat **不会监听** OneBot 的 WS 端口（3001），
//     于是桥接怎么重连都连不上，面板上只看到 connection refused。
//
// 过去唯一的出路是人去扫码。但 NapCat 的登录态是落在
// /root/.config/QQ 下的（NTQQ 的设备信息 + 票据），只要这个目录不删，
// 重启协议端就能「快速登录」，不需要扫码。
// 所以我们之前每次排查都 rm qrcode.png + restart 反而是最坏的做法——
// 它把用户推回扫码流程。
//
// 这个守护做的事：端口连续几分钟不监听 → 重启协议端（**不动登录态文件**），
// 让它自己快速登录回来。带三重刹车，避免把服务重启成风暴。

const (
	yyWatchdogDefaultInterval    = 60               // 默认探测间隔（秒）
	yyWatchdogDefaultMaxRestarts = 3                // 默认一小时内最多重启次数
	yyWatchdogMinInterval        = 10               // 间隔下限（秒）
	yyWatchdogMaxInterval        = 3600             // 间隔上限（秒）
	yyWatchdogMaxRestartsLimit   = 100              // 次数上限
	yyWatchdogFailsToAct         = 3                // 连续 3 次不可用才动手
	yyWatchdogCooldown           = 30 * time.Minute // 用完额度后静默 30 分钟
)

// yyWatchdogTuning 读面板配置（qq-bridge.json 的 watchdog 段），非法值回落到默认。
// 旧配置只有 max_per_hour 时仍按次数生效。
func yyWatchdogTuning() (time.Duration, int) {
	wd := yyQQSnapshot().Watchdog
	interval := time.Duration(wd.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = yyWatchdogDefaultInterval * time.Second
	}
	if interval < yyWatchdogMinInterval*time.Second {
		interval = yyWatchdogMinInterval * time.Second
	}
	if interval > yyWatchdogMaxInterval*time.Second {
		interval = yyWatchdogMaxInterval * time.Second
	}
	maxRestarts := wd.MaxRestarts
	if maxRestarts <= 0 {
		maxRestarts = wd.MaxPerHour
	}
	if maxRestarts <= 0 {
		maxRestarts = yyWatchdogDefaultMaxRestarts
	}
	if maxRestarts > yyWatchdogMaxRestartsLimit {
		maxRestarts = yyWatchdogMaxRestartsLimit
	}
	return interval, maxRestarts
}

// yyQQWatchdogConfigure 把新参数写进配置并落盘。
func yyQQWatchdogConfigure(intervalSeconds, maxRestarts int) error {
	yyQQMu.Lock()
	defer yyQQMu.Unlock()
	yyQQ.Watchdog.IntervalSeconds = intervalSeconds
	yyQQ.Watchdog.MaxRestarts = maxRestarts
	yyQQ.Watchdog.MaxPerHour = 0 // 已被 max_restarts 取代，避免两个值互相覆盖
	if yyQQPath == "" {
		return nil
	}
	raw, e := json.MarshalIndent(yyQQ, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(yyQQPath, raw, 0600)
}

var yyWatchdog = struct {
	sync.Mutex
	Enabled  bool
	Fails    int
	Restarts []time.Time
	LastAt   time.Time
	LastErr  string
	LastOK   time.Time
	CoolTill time.Time
	// NapCatRunning 最近一次探测时协议端进程是否在跑。
	// 进程都没了（被人 stop / OOM 杀掉）也值得拉起来。
	NapCatRunning bool
}{Enabled: true}

// yyQQWatchdogStart 启动守护。只跑一次。
var yyQQWatchdogOnce sync.Once

func yyQQWatchdogStart() {
	yyQQWatchdogOnce.Do(func() { go yyQQWatchdogLoop() })
}

func yyQQWatchdogLoop() {
	// 启动后先等一会儿，别和主流程的首次连接抢时间。
	time.Sleep(20 * time.Second)
	for {
		yyQQWatchdogProbe()
		// 每轮都重新读配置：面板改完间隔不用重启进程。
		interval, _ := yyWatchdogTuning()
		time.Sleep(interval)
	}
}

// yyQQWatchdogProbe 一次探测 + 必要时自愈。
func yyQQWatchdogProbe() {
	cf := yyQQSnapshot()
	if !cf.Enabled || cf.Adapter != "onebot" {
		return
	}
	// 先问一句「你还活着吗」：socket 是 open 但 QQ 会话已经没了这种情况，
	// 光看端口和 socket 状态都发现不了（见 youyuan_qq_liveness.go 文件头）。
	live, _ := yyQQLiveProbeOnce()

	// 已登录：清零计数。
	// 注意这里也要刷新 NapCatRunning——之前提前 return 没更新，
	// 导致面板上一直显示「协议端未运行」，而它其实好好的。
	if yyQQStateNow() == "open" && yyQQWSListening(cf.OneBot.URL) && live.OK {
		running := yyNapCatRunning()
		yyWatchdog.Lock()
		yyWatchdog.Fails = 0
		yyWatchdog.LastOK = time.Now()
		yyWatchdog.NapCatRunning = running
		yyWatchdog.Unlock()
		return
	}

	listening := yyQQWSListening(cf.OneBot.URL)
	running := yyNapCatRunning()

	yyWatchdog.Lock()
	yyWatchdog.NapCatRunning = running
	if listening && !running {
		// 端口在听但没有进程，属于探测抖动，不算失败。
		yyWatchdog.Unlock()
		return
	}
	yyWatchdog.Fails++
	fails := yyWatchdog.Fails
	enabled := yyWatchdog.Enabled
	cool := time.Now().Before(yyWatchdog.CoolTill)
	yyWatchdog.Unlock()

	if !enabled || fails < yyWatchdogFailsToAct || cool {
		return
	}
	yyQQWatchdogRecover(listening)
}

// yyQQWatchdogRecover 重启协议端让它快速登录回来。
func yyQQWatchdogRecover(listening bool) {
	cmdLine := strings.TrimSpace(os.Getenv("YY_NAPCAT_RESTART_CMD"))
	if cmdLine == "" {
		yyWatchdog.Lock()
		yyWatchdog.LastErr = "没有配置 YY_NAPCAT_RESTART_CMD，无法自动恢复"
		yyWatchdog.Fails = 0
		yyWatchdog.CoolTill = time.Now().Add(yyWatchdogCooldown)
		yyWatchdog.Unlock()
		return
	}

	now := time.Now()
	_, maxRestarts := yyWatchdogTuning()
	yyWatchdog.Lock()
	kept := yyWatchdog.Restarts[:0]
	for _, at := range yyWatchdog.Restarts {
		if now.Sub(at) < time.Hour {
			kept = append(kept, at)
		}
	}
	yyWatchdog.Restarts = kept
	if len(kept) >= maxRestarts {
		// 一小时内已经重启够多次了：说明不是「进程卡住」而是「登录态真失效」，
		// 再重启也没用，必须人工扫码。静默半小时，并把原因写进状态给前端显示。
		yyWatchdog.LastErr = fmt.Sprintf("一小时内已自动重启 %d 次仍未恢复，登录态可能已失效，需要人工扫码登录", len(kept))
		yyWatchdog.CoolTill = now.Add(yyWatchdogCooldown)
		yyWatchdog.Fails = 0
		yyWatchdog.Unlock()
		yyQQLog("掉线自愈：额度用尽，暂停 30 分钟，等待人工扫码")
		return
	}
	yyWatchdog.Restarts = append(yyWatchdog.Restarts, now)
	yyWatchdog.Fails = 0
	yyWatchdog.Unlock()

	yyQQLog(fmt.Sprintf("掉线自愈：OneBot 端口%s，重启协议端（保留登录态，走快速登录）", map[bool]string{true: "在听但桥接未连上", false: "不监听"}[listening]))

	// 独立 ctx：守护不能因为某次 exec 卡住就永远停摆。
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	out, e := exec.CommandContext(ctx, "sh", "-c", cmdLine).CombinedOutput()

	yyWatchdog.Lock()
	yyWatchdog.LastAt = time.Now()
	if e != nil {
		yyWatchdog.LastErr = e.Error() + "：" + strings.TrimSpace(string(out))
	} else {
		yyWatchdog.LastErr = ""
	}
	yyWatchdog.Unlock()
	if e != nil {
		yyQQLog("掉线自愈失败：" + e.Error())
	}
}

// yyNapCatRunning 协议端进程是否存活。用 pgrep 精确匹配 QQ 主程序，
// 不用 `ps | grep`（会把 grep 自己算进去）。
// 先按进程名精确匹配；有些环境里进程名会被截断/改写，再用完整命令行兜一次。
func yyNapCatRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if exec.CommandContext(ctx, "pgrep", "-x", "qq").Run() == nil {
		return true
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	return exec.CommandContext(ctx2, "pgrep", "-f", "/opt/QQ/qq").Run() == nil
}

// yyQQWatchdogStatus 给面板看的自愈状态。
func yyQQWatchdogStatus() map[string]any {
	// 先读配置再锁守望状态：配置锁与状态锁的获取顺序保持一致，避免与探测线程互锁。
	interval, maxRestarts := yyWatchdogTuning()
	yyWatchdog.Lock()
	defer yyWatchdog.Unlock()
	now := time.Now()
	restarts := make([]string, 0, len(yyWatchdog.Restarts))
	for _, at := range yyWatchdog.Restarts {
		if now.Sub(at) < time.Hour {
			restarts = append(restarts, at.Format("15:04:05"))
		}
	}
	out := map[string]any{
		"ok":               true,
		"enabled":          yyWatchdog.Enabled,
		"fails":            yyWatchdog.Fails,
		"restarts_1h":      len(restarts),
		"restart_times":    restarts,
		"interval_seconds": int(interval.Seconds()),
		"max_restarts":     maxRestarts,
		// 旧字段：等于 max_restarts，保留给老前端。
		"max_per_hour":    maxRestarts,
		"napcat_running":  yyWatchdog.NapCatRunning,
		"has_restart_cmd": strings.TrimSpace(os.Getenv("YY_NAPCAT_RESTART_CMD")) != "",
	}
	if !yyWatchdog.LastAt.IsZero() {
		out["last_restart_at"] = yyWatchdog.LastAt.Format("01-02 15:04:05")
	}
	if !yyWatchdog.LastOK.IsZero() {
		out["last_ok_at"] = yyWatchdog.LastOK.Format("01-02 15:04:05")
	}
	if !yyWatchdog.CoolTill.IsZero() && now.Before(yyWatchdog.CoolTill) {
		out["cool_until"] = yyWatchdog.CoolTill.Format("15:04:05")
		out["cool_seconds"] = int(yyWatchdog.CoolTill.Sub(now).Seconds())
	}
	if yyWatchdog.LastErr != "" {
		out["last_error"] = yyWatchdog.LastErr
	}
	return out
}

// yyQQWatchdogRoutes 开关与状态。
func yyQQWatchdogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/watchdog", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			writeJSON(w, yyQQWatchdogStatus())
		case "POST":
			var body struct {
				Enabled         *bool `json:"enabled"`
				Reset           bool  `json:"reset"`
				IntervalSeconds *int  `json:"interval_seconds"`
				MaxRestarts     *int  `json:"max_restarts"`
				MaxPerHour      *int  `json:"max_per_hour"` // 旧参数名，等同 max_restarts
			}
			if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			if body.IntervalSeconds != nil && (*body.IntervalSeconds < yyWatchdogMinInterval || *body.IntervalSeconds > yyWatchdogMaxInterval) {
				writeJSON(w, map[string]any{"ok": false, "error": fmt.Sprintf("interval_seconds 需在 %d..%d 秒之间", yyWatchdogMinInterval, yyWatchdogMaxInterval)})
				return
			}
			maxRestarts := body.MaxRestarts
			if maxRestarts == nil {
				maxRestarts = body.MaxPerHour
			}
			if maxRestarts != nil && (*maxRestarts < 1 || *maxRestarts > yyWatchdogMaxRestartsLimit) {
				writeJSON(w, map[string]any{"ok": false, "error": fmt.Sprintf("max_restarts 需在 1..%d 之间", yyWatchdogMaxRestartsLimit)})
				return
			}
			if body.IntervalSeconds != nil || maxRestarts != nil {
				interval, currentMax := yyWatchdogTuning()
				nextInterval, nextMax := int(interval.Seconds()), currentMax
				if body.IntervalSeconds != nil {
					nextInterval = *body.IntervalSeconds
				}
				if maxRestarts != nil {
					nextMax = *maxRestarts
				}
				if e := yyQQWatchdogConfigure(nextInterval, nextMax); e != nil {
					writeJSON(w, map[string]any{"ok": false, "error": "保存自愈参数失败：" + e.Error()})
					return
				}
			}
			yyWatchdog.Lock()
			if body.Enabled != nil {
				yyWatchdog.Enabled = *body.Enabled
			}
			if body.Reset {
				yyWatchdog.Fails = 0
				yyWatchdog.CoolTill = time.Time{}
				yyWatchdog.LastErr = ""
				yyWatchdog.Restarts = nil
			}
			yyWatchdog.Unlock()
			writeJSON(w, yyQQWatchdogStatus())
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
		}
	})
}
