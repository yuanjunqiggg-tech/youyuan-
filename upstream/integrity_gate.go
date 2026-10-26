package main

// 完整性闸门（integrity_gate.go）
//
// 背景：integrity.go / apk_signing.go 里那套签名校验本身是能用的，但一直缺三样东西：
//   1. 从来没人在启动时调用 VerifyApkIntegrity —— 校验结果永远是 false；
//   2. 即便校验出 true，也没有任何地方读 IsApkTampered —— 没人拦；
//   3. 内嵌前端资源的摘要（frontendHash）构建时没注入 —— RunIntegrityChecks 空转。
//
// 这个文件补的就是「调用点 + HTTP 闸门 + 状态出口」：
//   - yyRunStartupIntegrity()：在端口监听之前同步跑一次自校验，结果写进包级状态；
//   - yyIntegrityGate()：包住整个 mux。判定被篡改时，/api/* 回 403 JSON，
//     其余（HTML/静态资源/字体/logo 一律不例外）回中文拦截页；
//   - yyIntegrityReport()：给 /api/youyuan/integrity 用的可观测快照。
//
// 三条硬约束（都很重要，别为了「看起来狠」而改）：
//   - **取不到校验依据就降级为「未知」，不拦截**。Linux 上根本没有 APK，
//     子进程（AccountProcessService）也不会上报 APK 路径。宁可漏判，
//     也不能把正常用户锁在门外——一次误判 = 整台机器的工具箱全挂。
//   - **不暴露 hash 值本身**。状态接口只给 checked/ok/reason/frontend_hash_present。
//   - 默认开启拦截；YY_INTEGRITY_ENFORCE=0 可临时关掉（开发调试）。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// 判定结果三态。空串表示「还没校验过」。
const (
	yyIntegrityClean    = "clean"    // 校验通过
	yyIntegrityTampered = "tampered" // 校验失败，拦截
	yyIntegrityUnknown  = "unknown"  // 不具备校验条件，放行但记录
)

type yyIntegrityStatus struct {
	mu      sync.RWMutex
	checked bool
	verdict string
	reason  string
}

var yyIntegrity yyIntegrityStatus

// yyIntegritySet 写入一次判定结果。
func yyIntegritySet(verdict, reason string) {
	yyIntegrity.mu.Lock()
	yyIntegrity.checked = true
	yyIntegrity.verdict = verdict
	yyIntegrity.reason = reason
	yyIntegrity.mu.Unlock()
}

// yyIntegrityReset 清空状态（单测用）。
func yyIntegrityReset() {
	yyIntegrity.mu.Lock()
	yyIntegrity.checked = false
	yyIntegrity.verdict = ""
	yyIntegrity.reason = ""
	yyIntegrity.mu.Unlock()
}

// yyIntegrityEnforceOn 是否启用拦截。默认开；YY_INTEGRITY_ENFORCE=0 关闭。
func yyIntegrityEnforceOn() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("YY_INTEGRITY_ENFORCE"))) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// yyIntegrityShouldBlock 判定当前请求是否应当被拦。
// 只有「已校验 + 判定篡改 + 未关闭强制」三个条件同时成立才拦。
func yyIntegrityShouldBlock() (bool, string) {
	yyIntegrity.mu.RLock()
	checked, verdict, reason := yyIntegrity.checked, yyIntegrity.verdict, yyIntegrity.reason
	yyIntegrity.mu.RUnlock()
	if !checked || verdict != yyIntegrityTampered {
		return false, ""
	}
	if !yyIntegrityEnforceOn() {
		return false, ""
	}
	return true, reason
}

// yyIntegrityReport 生成状态快照。刻意不含任何 hash 值。
func yyIntegrityReport() map[string]any {
	yyIntegrity.mu.RLock()
	checked, verdict, reason := yyIntegrity.checked, yyIntegrity.verdict, yyIntegrity.reason
	yyIntegrity.mu.RUnlock()
	blocked := checked && verdict == yyIntegrityTampered && yyIntegrityEnforceOn()
	return map[string]any{
		"checked":              checked,
		"ok":                   checked && verdict == yyIntegrityClean,
		"verdict":              verdict,
		"reason":               reason,
		"blocked":              blocked,
		"enforced":             yyIntegrityEnforceOn(),
		"platform":             runtime.GOOS,
		"frontend_hash_present": strings.TrimSpace(frontendHash) != "",
	}
}

// yyIntegrityStatusHandler 是 /api/youyuan/integrity 的处理器。
func yyIntegrityStatusHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "integrity": yyIntegrityReport()})
}

// yyIntegrityGate 把整棵 mux 包起来。被判定篡改时一律拦截，静态资源也不例外。
func yyIntegrityGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if block, _ := yyIntegrityShouldBlock(); block {
			yyWriteTamperedResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// yyIntegrityIsAPIRequest 判断请求要 JSON 还是 HTML。
func yyIntegrityIsAPIRequest(r *http.Request) bool {
	p := r.URL.Path
	if strings.HasPrefix(p, "/api/") || p == "/api" {
		return true
	}
	// MCP 是 JSON-RPC，远端中继与插件都按 JSON 解析，不能喂 HTML。
	return p == "/mcp" || strings.HasPrefix(p, "/mcp/") || p == "/health"
}

func yyWriteTamperedResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-YouYuan-Integrity", "tampered")
	if yyIntegrityIsAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "客户端完整性校验未通过",
			"code":  "tampered",
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(yyIntegrityBlockPage))
}

// 拦截页内联在 Go 里：不依赖任何外部文件，避免「删掉拦截页资源就绕过」。
const yyIntegrityBlockPage = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>客户端完整性校验未通过</title>
<style>
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;
background:#0f1115;color:#e8eaed;font-family:-apple-system,"PingFang SC","Microsoft YaHei",sans-serif;padding:24px}
.card{max-width:560px;width:100%;background:#171a21;border:1px solid #262b36;border-radius:14px;
padding:32px 28px;box-shadow:0 12px 40px rgba(0,0,0,.45)}
.icon{width:46px;height:46px;border-radius:50%;background:#3a1d20;color:#ff6b6b;
display:flex;align-items:center;justify-content:center;font-size:26px;font-weight:700;margin-bottom:18px}
h1{font-size:21px;margin:0 0 14px;line-height:1.4}
p{font-size:14px;line-height:1.75;color:#b6bcc8;margin:0 0 12px}
.hint{font-size:13px;color:#8b93a3;border-top:1px solid #262b36;padding-top:14px;margin-top:18px}
.code{margin:16px 0 0;font-family:ui-monospace,Consolas,monospace;font-size:12px;color:#6b7280}
</style>
</head>
<body>
<div class="card">
<div class="icon">!</div>
<h1>客户端完整性校验未通过</h1>
<p>当前程序的文件签名与本工具箱的官方发布版本不一致，可能已被修改、注入或重新打包。</p>
<p>为保护你的账号与设备，面板已停止提供服务。</p>
<p class="hint">请从官方渠道重新下载安装包后再使用。开发者如需临时调试，可设置环境变量 YY_INTEGRITY_ENFORCE=0 关闭校验（仅本机生效）。</p>
<p class="code">错误码：tampered</p>
</div>
</body>
</html>`

// yyRunStartupIntegrity 在端口监听之前同步执行一次自校验。
// 必须是同步的：闸门一旦上线就只能依据这里的结果判断，异步跑会留出
// 「先发请求再被拦」的时间窗。
func yyRunStartupIntegrity(dataDir string) {
	verdict, reason := yyVerifySelfIntegrity()
	yyIntegritySet(verdict, reason)

	switch verdict {
	case yyIntegrityClean:
		yyIntegrityLog(dataDir, "完整性校验通过")
	case yyIntegrityTampered:
		yyIntegrityLog(dataDir, "完整性校验未通过，HTTP 已进入拦截模式："+reason)
	default:
		// 关键：未知 = 放行。取不到校验依据不是用户的错。
		yyIntegrityLog(dataDir, "完整性校验降级为未知（不拦截）："+reason)
	}
}

// yyVerifySelfIntegrity 汇总两条校验线，返回 (verdict, reason)。
func yyVerifySelfIntegrity() (string, string) {
	// 1) 内嵌资源摘要。前端 JS/CSS 是被编进二进制里的，
	//    改 JS 重打包会让这里的摘要对不上。
	for name, r := range RunIntegrityChecks() {
		if !r.Passed {
			return yyIntegrityTampered, "内嵌资源校验失败（" + name + "）：" + r.Message
		}
	}

	// 2) 自身签名 / 封签。
	switch runtime.GOOS {
	case "android":
		return yyVerifyAndroidApk()
	case "linux", "windows":
		// ELF/PE 的封印槽由 maintenance/seal_binary.py 回填，见 protect_selfcheck.go。
		return yyVerifySelfSeal()
	default:
		return yyIntegrityUnknown, "平台 " + runtime.GOOS + " 不具备自校验条件"
	}
}

// yyVerifyAndroidApk 用 Java 上报的 APK 路径/签名做权威校验。
// 两者都为空说明「拿不到依据」（子进程、非官方启动方式），降级为未知。
func yyVerifyAndroidApk() (string, string) {
	if strings.TrimSpace(apkPath) == "" && strings.TrimSpace(apkSigHash) == "" {
		return yyIntegrityUnknown, "未收到 APK 路径与签名上报（可能是账号子进程），按未知放行"
	}
	VerifyApkIntegrity()
	if IsApkTampered() {
		reason := TamperReason()
		if reason == "" {
			reason = "APK 签名校验未通过"
		}
		return yyIntegrityTampered, reason
	}
	return yyIntegrityClean, ""
}

// yyVerifySelfSeal 校验自身二进制的封印槽。
// sealed → 通过；mismatch → 拦截；unsealed/not_found/read_error → 未知放行
// （开发构建没跑封印脚本，属于正常情况）。
func yyVerifySelfSeal() (string, string) {
	exe, err := os.Executable()
	if err != nil {
		return yyIntegrityUnknown, "取自身路径失败：" + err.Error()
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return yyIntegrityUnknown, "读自身失败：" + err.Error()
	}
	status, detail, _ := yySealVerify(data)
	switch status {
	case "sealed":
		return yyIntegrityClean, ""
	case "mismatch":
		// detail 里带 hash，不往外给，只落日志。
		return yyIntegrityTampered, "内核二进制封签不符，可能已被修改"
	default:
		return yyIntegrityUnknown, "封签状态 " + status + "（" + detail + "）"
	}
}

// yyIntegrityLog 把结果同时打到 stdout 和 <dataDir>/security.log。
// 写日志失败不影响判定（日志不是安全边界）。
func yyIntegrityLog(dataDir, msg string) {
	stamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[integrity] %s %s\n", stamp, msg)
	if strings.TrimSpace(dataDir) == "" {
		return
	}
	f, err := os.OpenFile(filepath.Join(dataDir, "security.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s [integrity] %s\n", stamp, msg)
}
