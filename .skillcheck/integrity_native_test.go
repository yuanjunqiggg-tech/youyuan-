package main

import (
	"os"
	"testing"
)

func officialAPKPath() string {
	for _, p := range []string{
		"/root/prism-toolbox/prism工具箱_v1.0.4-beta.50.apk",
		"/root/prism-toolbox/bot-apk/prism工具箱_v1.0.4-beta.50.apk",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// 官方 APK：权威校验应判定为未被篡改。
func TestVerifyApkIntegrity_NativeOfficial(t *testing.T) {
	apk := officialAPKPath()
	if apk == "" {
		t.Skip("no official APK present")
	}
	SetApkPath(apk)
	VerifyApkIntegrity()
	if IsApkTampered() {
		t.Fatalf("官方 APK 被误判为篡改: %s", TamperReason())
	}
	t.Logf("official APK verified OK (reason=%q)", TamperReason())
}

// 破解 APK（签名 a40da80a，非官方密钥）：权威校验应判定为被篡改。
func TestVerifyApkIntegrity_NativeTampered(t *testing.T) {
	apk := "/root/prism-toolbox/小果[万能导入器]_3.0.5破解版.apk.1"
	if _, err := os.Stat(apk); err != nil {
		t.Skip("no cracked APK present")
	}
	SetApkPath(apk)
	VerifyApkIntegrity()
	if !IsApkTampered() {
		t.Fatal("破解 APK 竟被判为合法")
	}
	t.Logf("cracked APK correctly blocked (reason=%q)", TamperReason())
}
