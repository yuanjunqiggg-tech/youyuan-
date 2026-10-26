package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

// 官方合法 APK 签名证书的 SHA-256 集合。
// 历史上更换过签名密钥，下列均为官方密钥签发的正式版本，全部放行。
// 被篡改/重打包的 APK 必须用新密钥重签，哈希不在该集合内 → 启动时被硬拦截。
var officialSigHashes = []string{
	"38083b09047454ef3d0623183b2ca5e716dfbb1ffe1c10893327f248e2bc4842", // 旧密钥 A（beta.11~32）
	"1a250d20e682bb5769aedf8326ac029324e6a82cb12141a636c3cdf0224959cf", // 旧密钥 B（beta.15/18/33~37）
	"f4847ee3c50e284abf9bf3a51c27237f1c8413450e0625ad832143d933f81d14", // 当前密钥（beta.38~）
}

// 官方包名。被改包名会在这里被拦。
// 必须与 android/app/build.gradle 的 applicationId 一致：原先写成旧包名
// com.prismtool.box，导致「包名被修改」这条检测实际上永远误判（安全审查 M2）。
const officialPkgName = "org.youyuan.toolbox"

// Build-time injected resource hashes.
// Set by: go build -ldflags "-X main.frontendHash=sha256:xxx -X main.soundsHash=sha256:xxx"
var (
	frontendHash = ""
	soundsHash   = ""
	apkSigHash   = "" // set from Android JNI at runtime (Java 上报，作兜底)
	apkPkgName   = "" // set from Android JNI at runtime
	apkPath      = "" // set from Android JNI at runtime（权威校验直接读这个 APK）
)

// 篡改状态（线程安全）。判定为被篡改后，HTTP 层会对所有请求返回拦截页。
var (
	tamperedMu   sync.Mutex
	apkTampered  bool
	tamperReason string
)

// IntegrityResult holds the result of a single integrity check.
type IntegrityResult struct {
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// SetApkSignatureHash stores the APK signature hash received from Android.
func SetApkSignatureHash(hash string) {
	apkSigHash = hash
}

// GetApkSignatureHash returns the stored APK signature hash.
func GetApkSignatureHash() string {
	return apkSigHash
}

// SetApkPackageName stores the APK package name received from Android.
func SetApkPackageName(pkg string) {
	apkPkgName = pkg
}

// SetApkPath stores the installed APK file path received from Android.
// Go 直接读该文件的签名证书做权威校验，不信任 Java 上报的哈希/包名。
func SetApkPath(path string) {
	apkPath = path
}

// hashInWhitelist 判断给定签名哈希是否命中任一官方密钥。
func hashInWhitelist(h string) bool {
	t := strings.TrimSpace(h)
	for _, x := range officialSigHashes {
		if strings.EqualFold(t, strings.TrimSpace(x)) {
			return true
		}
	}
	return false
}

// IsApkTampered reports whether startup verification decided the APK is not official.
func IsApkTampered() bool {
	tamperedMu.Lock()
	defer tamperedMu.Unlock()
	return apkTampered
}

// TamperReason returns the reason the APK was flagged as tampered ("" if clean).
func TamperReason() string {
	tamperedMu.Lock()
	defer tamperedMu.Unlock()
	return tamperReason
}

// MarkTampered flips the tampered flag on (e.g. from a server-authoritative verdict).
func MarkTampered(reason string) {
	tamperedMu.Lock()
	defer tamperedMu.Unlock()
	apkTampered = true
	if reason != "" {
		tamperReason = reason
	}
}

// VerifyApkIntegrity 做权威完整性校验，决定 apkTampered 标志。
// 优先：直接读 APK 文件的签名证书（不信任 Java，破解者改 Java 伪造也绕不过）。
// 兜底：拿不到 APK 路径时，退回 Java 上报的签名/包名校验。
// 在启动时调用一次（GoMain 之前 Java 已通过 JNI 上报 APK 路径/签名/包名）。
func VerifyApkIntegrity() {
	tamperedMu.Lock()
	defer tamperedMu.Unlock()

	// 1. 权威校验：直接读 APK 签名证书。
	if apkPath != "" {
		h, err := apkSigningCertHash(apkPath)
		if err != nil {
			apkTampered = true
			tamperReason = "无法校验 APK 签名（" + err.Error() + "）"
			return
		}
		if !hashInWhitelist(h) {
			apkTampered = true
			tamperReason = "APK 签名证书不匹配，可能已被重打包"
			return
		}
		// 权威分支原先校验完签名就 return，下面兜底分支的包名校验永远不执行，
		// 「包名被修改」形同虚设（安全审查 M2）。这里补上：Java 侧上报的是真实
		// getPackageName()（MainActivity.java:73），为空时跳过以免拿不到上报就误判。
		if apkPkgName != "" && !strings.EqualFold(strings.TrimSpace(apkPkgName), officialPkgName) {
			apkTampered = true
			tamperReason = "应用包名被修改"
			return
		}
		apkTampered = false
		tamperReason = ""
		return
	}

	// 2. 兜底：Java 上报的签名/包名校验（拿不到 APK 路径时）。
	if strings.TrimSpace(apkSigHash) == "" {
		apkTampered = true
		tamperReason = "未检测到 APK 签名"
		return
	}
	if !hashInWhitelist(apkSigHash) {
		apkTampered = true
		tamperReason = "APK 签名不匹配，可能已被重打包"
		return
	}
	if apkPkgName != "" && !strings.EqualFold(strings.TrimSpace(apkPkgName), officialPkgName) {
		apkTampered = true
		tamperReason = "应用包名被修改"
		return
	}
	apkTampered = false
	tamperReason = ""
}

// frontendResourceDigest 计算内嵌前端资源树的稳定摘要。
//
// 算法（build.ps1 的 Get-FrontendDigest 必须逐字节一致）：
//  1. 递归遍历 fsys 的 frontend/ 下所有文件，按路径做**字节序**排序；
//  2. 每个文件写一行 "路径\n<sha256 hex 小写>\n"；
//  3. 对整段文本求 sha256，输出 hex 小写。
//
// 为什么不用「只算 index.html」：真正会被改的是 js/ 下的逻辑代码，
// 只钉 index.html 等于没钉。整棵树一起算才能覆盖「改一个 JS 重签」这条路径。
func frontendResourceDigest(fsys fs.FS) (string, error) {
	var paths []string
	err := fs.WalkDir(fsys, "frontend", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("内嵌前端资源为空")
	}
	sort.Strings(paths)

	var sb strings.Builder
	for _, p := range paths {
		data, readErr := fs.ReadFile(fsys, p)
		if readErr != nil {
			return "", fmt.Errorf("读取 %s 失败: %w", p, readErr)
		}
		sum := sha256.Sum256(data)
		sb.WriteString(p)
		sb.WriteByte('\n')
		sb.WriteString(hex.EncodeToString(sum[:]))
		sb.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:]), nil
}

// trimSHA256Prefix 去掉 "sha256:" 前缀，并统一小写。
func trimSHA256Prefix(v string) string {
	t := strings.ToLower(strings.TrimSpace(v))
	if strings.HasPrefix(t, "sha256:") {
		t = t[len("sha256:"):]
	}
	return t
}

// shortHash 只截前 16 位用于日志/提示，避免把完整摘要写进事件流。
func shortHash(h string) string {
	if len(h) <= 16 {
		return h
	}
	return h[:16]
}

// RunIntegrityChecks verifies embedded resources haven't been tampered with.
// Returns a map of check name → result.
//
// 关于「没注入就静默通过」这个口子：构建脚本（build.ps1）每次都会同时注入
// buildStamp 和 frontendHash。所以「有 buildStamp 却没有 frontendHash」
// 只可能是构建流水线漏了参数 —— 这种情况直接判失败，绝不能当开发构建放过去。
// 反过来，普通 `go build` / `go test` 两者都为空，属于开发场景，跳过检查。
func RunIntegrityChecks() map[string]IntegrityResult {
	results := make(map[string]IntegrityResult)

	expected := trimSHA256Prefix(frontendHash)
	if expected == "" {
		if strings.TrimSpace(buildStamp) != "" {
			results["frontend"] = IntegrityResult{
				Passed:  false,
				Message: "构建时未注入前端资源摘要（frontendHash 为空），拒绝以未校验状态运行",
			}
		}
		return results
	}

	actual, err := frontendResourceDigest(frontendFS)
	if err != nil {
		results["frontend"] = IntegrityResult{Passed: false, Message: "无法读取前端资源: " + err.Error()}
		return results
	}
	if actual == expected {
		results["frontend"] = IntegrityResult{Passed: true, Message: "OK"}
		return results
	}
	results["frontend"] = IntegrityResult{
		Passed:  false,
		Message: fmt.Sprintf("前端资源被篡改 (期望:%s 实际:%s)", shortHash(expected), shortHash(actual)),
	}
	return results
}
