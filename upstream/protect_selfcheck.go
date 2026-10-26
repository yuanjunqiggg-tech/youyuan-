package main

// 启动自检（protect_selfcheck.go）
//
// 三件事，都很便宜、都不会改变业务行为：
//
//  1. 自身完整性自校验（封印槽）
//     二进制里预留了一段固定长度的槽位（yySealRecord），构建完成后由
//     maintenance/seal_binary.py 把「把槽位清零后的整个文件」的 sha256 写进去。
//     运行时就地重算并比对——任何对二进制的字节级修改（改字符串、插 jump、
//     打补丁做去授权）都会让它对不上。
//
//     为什么不用 -X 注入：`go build -ldflags -X main.x=<hash>` 注入的字符串
//     本身会改变二进制，注入的 hash 永远不可能是产物的 hash，这是个死循环。
//     预置固定槽位 + 构建后回填是唯一自洽的做法。
//
//  2. 调试器检测（TracerPid）
//     读 /proc/self/status 的 TracerPid。非 0 说明有人 ptrace 上了本进程。
//     **只记日志告警，不退出、不自杀**。理由：这套东西跑在用户的
//     systemd 里，一次误报就是整台机子的工具箱挂掉；而 systemd 本身、
//     部分监控/备份代理确实会短暂 ptrace。宁可漏报也不要自伤。
//
//  3. 构建指纹
//     -X main.buildStamp=<随机值>，用来把「哪次构建」写进日志，
//     配合 `-buildid=` 让两份构建产物无法从内部指纹互相定位。
//
// 自检结果同时写 <dataDir>/security.log；设置 YY_SELFTEST=1 时改为
// 打印到 stdout 并直接退出（os.Exit(0)），供人工/CI 验证，不启动服务。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// buildStamp 由构建脚本注入：-X main.buildStamp=<时间戳/随机串>。
var buildStamp = ""

// yySealSlotHexLen 是封印槽里 sha256 十六进制串的长度。
const yySealSlotHexLen = 64

// yySealRecord 是二进制里的封印记录：魔数 + 64 字节槽位 + 结束标记。
// 三段必须写在同一个表达式里——Go 会把常量拼接折叠成一个连续的字符串
// 字面量，这样它在 .rodata 里才是一段连续字节，seal_binary.py 才能定位。
// 槽位初始值必须是 64 个 '0'，长度写死，绝不能改动。
const yySealRecord = "YYSEALSC1" + "0000000000000000000000000000000000000000000000000000000000000000" + "YYSEALEND"

const (
	yySealPrefixStr = "YYSEALSC1"
	yySealSuffixStr = "YYSEALEND"
)

// yySealRecordData 把整段记录以字节切片的形式带进 .rodata 并保持可达。
//
// 这一层不能省：`-s -w` 构建下链接器会删掉「只被常量表达式使用」的字面量，
// 第一版就是因为只用了 yySealPrefix / yySealSuffix 两个小常量，导致
// yySealRecord 被整段优化掉，自检永远报 not_found。
// 只要有人真的读这段字节，链接器就必须把它留在文件里。
var yySealRecordData = []byte(yySealRecord)

var (
	yySealPrefix = yySealRecordData[:len(yySealPrefixStr)]
	yySealSuffix = yySealRecordData[len(yySealRecordData)-len(yySealSuffixStr):]
)

// yySelfCheckReport 是一次自检的完整结果。
type yySelfCheckReport struct {
	BinaryPath string   `json:"binary_path"`
	BuildStamp string   `json:"build_stamp"`
	SealStatus string   `json:"seal_status"` // sealed / unsealed / mismatch / not_found / unsupported / read_error
	SealDetail string   `json:"seal_detail"`
	SelfSHA256 string   `json:"self_sha256"`
	SelfSize   int64    `json:"self_size"`
	TracerPid  int      `json:"tracer_pid"`
	Debugger   bool     `json:"debugger"`
	Warnings   []string `json:"warnings"`
}

// yySealVerify 对一段二进制内容做封印校验，返回 (状态, 说明, 实际sha256)。
// 抽成纯函数是为了能在单测里用内存数据验证，不必真的去读磁盘上的自己。
//
// 注意：不能拿 yySealRecord（全零槽位）去搜——槽位一旦回填就不再是全零。
// 必须按「魔数 + 64 字节 + 结束标记」的结构去定位。
func yySealVerify(data []byte) (string, string, string) {
	starts := yySealSlots(data)
	if len(starts) == 0 {
		return "not_found", "二进制里找不到封印记录（未经 seal_binary.py 处理，或已被整体替换）", ""
	}
	if len(starts) > 1 {
		// 多于一处说明有人往里面塞了一份伪造记录，想让我们校验到假槽位上。
		return "mismatch", fmt.Sprintf("封印记录出现 %d 次，疑似伪造", len(starts)), ""
	}
	idx := starts[0]

	slotStart := idx + len(yySealPrefix)
	slotEnd := slotStart + yySealSlotHexLen
	slot := string(data[slotStart:slotEnd])

	// 槽位还是全 0：开发构建（没跑封印脚本）。不算失败，但要如实报出来。
	if strings.Trim(slot, "0") == "" {
		sum := sha256.Sum256(data)
		return "unsealed", "槽位为空，开发构建未封印", hex.EncodeToString(sum[:])
	}

	// 重算：把槽位当成 64 个 '0' 再由整个文件求 sha256。
	buf := make([]byte, len(data))
	copy(buf, data)
	for i := slotStart; i < slotEnd; i++ {
		buf[i] = '0'
	}
	sum := sha256.Sum256(buf)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(slot, actual) {
		return "mismatch", "封签不符：二进制已被修改（" + actual + " != " + slot + "）", actual
	}
	return "sealed", "封签一致", actual
}

// yySealSlots 返回所有「魔数 + 64 字节槽位 + 结束标记」结构的起始下标。
// 正常情况下应当恰好一个；多个即为伪造。
func yySealSlots(data []byte) []int {
	needle := yySealPrefix
	tail := yySealSuffix
	total := len(needle) + yySealSlotHexLen + len(tail)
	var starts []int
	for from := 0; ; {
		at := indexBytes(data[from:], needle)
		if at < 0 {
			break
		}
		pos := from + at
		from = pos + 1
		if pos+total > len(data) {
			continue
		}
		if indexBytes(data[pos+total-len(tail):pos+total], tail) != 0 {
			continue
		}
		starts = append(starts, pos)
	}
	return starts
}

// indexBytes 是 bytes.Index 的最小替代，避免为了一个调用多导一个包名。
func indexBytes(hay, needle []byte) int {
	if len(needle) == 0 || len(hay) < len(needle) {
		return -1
	}
	limit := len(hay) - len(needle)
	for i := 0; i <= limit; i++ {
		if hay[i] != needle[0] {
			continue
		}
		match := true
		for j := 1; j < len(needle); j++ {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// yyTracerPID 读 /proc/self/status 的 TracerPid。
// 非 Linux 平台返回 (-1, nil)，表示「本平台不支持」，不是错误。
func yyTracerPID() (int, error) {
	if runtime.GOOS != "linux" {
		return -1, nil
	}
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return -1, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "TracerPid:") {
			continue
		}
		pid, convErr := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "TracerPid:")))
		if convErr != nil {
			return -1, convErr
		}
		return pid, nil
	}
	return -1, nil
}

// yySelfCheck 执行全部检查并返回报告。
func yySelfCheck() yySelfCheckReport {
	rep := yySelfCheckReport{BuildStamp: buildStamp}

	// 调试器检测：只告警。
	if pid, err := yyTracerPID(); err != nil {
		rep.Warnings = append(rep.Warnings, "TracerPid 读取失败："+err.Error())
	} else {
		rep.TracerPid = pid
		rep.Debugger = pid > 0
		if pid > 0 {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("检测到调试器（TracerPid=%d）", pid))
		}
	}

	// Android 上 os.Executable() 指向的是 zygote 的 app_process，不是本库，
	// 拿它算哈希没有意义，所以整段跳过（APK 侧的完整性由 integrity.go 负责）。
	if runtime.GOOS == "android" {
		rep.SealStatus = "unsupported"
		rep.SealDetail = "Android 平台不校验宿主进程（APK 完整性由签名校验负责）"
		return rep
	}

	exe, err := os.Executable()
	if err != nil {
		rep.SealStatus = "read_error"
		rep.SealDetail = "取自身路径失败：" + err.Error()
		rep.Warnings = append(rep.Warnings, rep.SealDetail)
		return rep
	}
	rep.BinaryPath = exe
	data, err := os.ReadFile(exe)
	if err != nil {
		rep.SealStatus = "read_error"
		rep.SealDetail = "读自身失败：" + err.Error()
		rep.Warnings = append(rep.Warnings, rep.SealDetail)
		return rep
	}
	rep.SelfSize = int64(len(data))
	status, detail, sum := yySealVerify(data)
	rep.SealStatus = status
	rep.SealDetail = detail
	rep.SelfSHA256 = sum
	if status == "mismatch" {
		rep.Warnings = append(rep.Warnings, "完整性自校验失败："+detail)
	}
	return rep
}

// yySelfCheckLines 把报告渲染成便于人读的若干行。
func yySelfCheckLines(rep yySelfCheckReport) []string {
	tracer := "n/a"
	if rep.TracerPid >= 0 {
		tracer = strconv.Itoa(rep.TracerPid)
	}
	lines := []string{
		"游园内核启动自检",
		"  构建指纹   : " + orDash(rep.BuildStamp),
		"  自身路径   : " + orDash(rep.BinaryPath),
		fmt.Sprintf("  自身大小   : %d 字节", rep.SelfSize),
		"  完整性     : " + rep.SealStatus + "（" + rep.SealDetail + "）",
		"  自身 sha256: " + orDash(rep.SelfSHA256),
		"  TracerPid  : " + tracer,
	}
	if len(rep.Warnings) == 0 {
		lines = append(lines, "  告警       : 无")
	} else {
		for i, w := range rep.Warnings {
			prefix := "  告警       : "
			if i > 0 {
				prefix = "               "
			}
			lines = append(lines, prefix+w)
		}
	}
	return lines
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// yySelfCheckStartup 是启动入口，由 StartServer 调用（唯一接线点，同时覆盖
// Linux/Windows 原生启动和 Android JNI 启动）。
//
//   - YY_SELFTEST=1：打印报告后 os.Exit(0)，不启动服务，供人工/CI 验证；
//   - 否则：把报告追加到 <dataDir>/security.log，然后正常返回。
//
// 注意这里**没有任何 return error 的分支**：自检永远不阻断启动。被篡改的
// 二进制该不该拒绝服务，是产品/授权层面的决定（需要服务端参与），不是一行
// 本地判断能承担的责任——误判一次就是用户的线上工具全挂。
func yySelfCheckStartup(dataDir string) {
	rep := yySelfCheck()
	lines := yySelfCheckLines(rep)

	if strings.TrimSpace(os.Getenv("YY_SELFTEST")) == "1" {
		for _, l := range lines {
			fmt.Println(l)
		}
		os.Exit(0)
	}

	if dataDir == "" {
		return
	}
	f, err := os.OpenFile(filepath.Join(dataDir, "security.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	stamp := time.Now().Format("2006-01-02 15:04:05")
	for _, l := range lines {
		fmt.Fprintf(f, "%s %s\n", stamp, l)
	}
}
