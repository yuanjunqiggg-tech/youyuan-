package main

// 启动自检的回归测试。
//
// 封印校验必须在「内存数据」上可测，否则这条防护永远只能靠上线后再发现。
// 这里模拟封印脚本的完整流程：造数据 → 确认未封印 → 回填 → 确认通过 →
// 改一个字节 → 确认报篡改；另外覆盖「伪造第二份记录」这个绕过手法。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime"
	"strings"
	"testing"
)

// yyTestSeal 复刻 maintenance/seal_binary.py 的算法。
func yyTestSeal(data []byte) []byte {
	out := make([]byte, len(data))
	copy(out, data)
	slots := yySealSlots(out)
	if len(slots) != 1 {
		panic("测试数据里封印记录数量不对")
	}
	idx := slots[0]
	slotStart := idx + len(yySealPrefix)
	for i := slotStart; i < slotStart+yySealSlotHexLen; i++ {
		out[i] = '0'
	}
	sum := sha256.Sum256(out)
	copy(out[slotStart:slotStart+yySealSlotHexLen], hex.EncodeToString(sum[:]))
	return out
}

func yyTestBinary() []byte {
	head := bytes.Repeat([]byte("HEAD"), 64)
	tail := bytes.Repeat([]byte("TAIL"), 64)
	return append(append(append([]byte{}, head...), yySealRecordData...), tail...)
}

func TestSealVerifyUnsealedSealedMismatch(t *testing.T) {
	raw := yyTestBinary()

	status, _, _ := yySealVerify(raw)
	if status != "unsealed" {
		t.Fatalf("未回填槽位应报 unsealed，得到 %s", status)
	}

	sealed := yyTestSeal(raw)
	status, detail, sum := yySealVerify(sealed)
	if status != "sealed" {
		t.Fatalf("回填后应报 sealed，得到 %s（%s）", status, detail)
	}
	if len(sum) != 64 {
		t.Fatalf("sha256 长度不对：%q", sum)
	}

	// 改一个业务字节（不是槽位），封签必须失效。
	tampered := append([]byte{}, sealed...)
	tampered[3] ^= 0xFF
	status, _, _ = yySealVerify(tampered)
	if status != "mismatch" {
		t.Fatalf("篡改后应报 mismatch，得到 %s", status)
	}

	// 只改槽位里的一个字符，同样必须失效。
	tampered2 := append([]byte{}, sealed...)
	slot := yySealSlots(tampered2)[0] + len(yySealPrefix)
	if tampered2[slot] == 'a' {
		tampered2[slot] = 'b'
	} else {
		tampered2[slot] = 'a'
	}
	status, _, _ = yySealVerify(tampered2)
	if status != "mismatch" {
		t.Fatalf("槽位被改写后应报 mismatch，得到 %s", status)
	}
}

func TestSealVerifyRejectsDuplicateRecord(t *testing.T) {
	// 攻击者往文件里再塞一份「自洽的假记录」，诱导校验命中假槽位。
	raw := yyTestBinary()
	sealed := yyTestSeal(raw)
	fake := yyTestSeal(yyTestBinary())
	doubled := append(append([]byte{}, sealed...), fake...)
	status, detail, _ := yySealVerify(doubled)
	if status != "mismatch" {
		t.Fatalf("双记录应报 mismatch，得到 %s", status)
	}
	if !strings.Contains(detail, "2 次") {
		t.Fatalf("说明里应指出记录次数：%s", detail)
	}
}

func TestSealVerifyMissingRecord(t *testing.T) {
	status, _, _ := yySealVerify([]byte("完全没有封印记录的二进制"))
	if status != "not_found" {
		t.Fatalf("无记录应报 not_found，得到 %s", status)
	}
}

func TestTracerPIDDoesNotPanic(t *testing.T) {
	pid, err := yyTracerPID()
	if runtime.GOOS != "linux" {
		if err != nil || pid != -1 {
			t.Fatalf("非 Linux 应返回 (-1, nil)，得到 (%d, %v)", pid, err)
		}
		return
	}
	if err != nil {
		// 容器里可能没有 /proc，允许报错，但不允许 panic 或返回负数以外的怪值。
		return
	}
	if pid < 0 {
		t.Fatalf("Linux 下 TracerPid 不应为负：%d", pid)
	}
}

func TestSelfCheckReportRenders(t *testing.T) {
	rep := yySelfCheck()
	lines := yySelfCheckLines(rep)
	if len(lines) < 6 {
		t.Fatalf("自检输出太少：%v", lines)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"游园内核启动自检", "完整性", "TracerPid"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("自检输出缺少 %q：\n%s", want, joined)
		}
	}
	if rep.SealStatus == "" {
		t.Fatalf("SealStatus 不能为空")
	}
}

func TestSelfCheckStartupWritesLog(t *testing.T) {
	// YY_SELFTEST 未置位时，应当只落 security.log 并正常返回（绝不退出）。
	dir := t.TempDir()
	t.Setenv("YY_SELFTEST", "")
	yySelfCheckStartup(dir)
	raw, err := os.ReadFile(dir + string(os.PathSeparator) + "security.log")
	if err != nil {
		t.Fatalf("security.log 未写入：%v", err)
	}
	if !strings.Contains(string(raw), "游园内核启动自检") {
		t.Fatalf("security.log 内容不对：%s", raw)
	}
}
