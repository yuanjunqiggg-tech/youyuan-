package main

import (
	"os"
	"testing"
)

// 用真实官方 APK 验证：签名证书哈希应等于当前官方密钥 f4847ee3...
func TestApkSigningCertHash_Official(t *testing.T) {
	paths := []string{
		"/root/prism-toolbox/prism工具箱_v1.0.4-beta.50.apk",
		"/root/prism-toolbox/bot-apk/prism工具箱_v1.0.4-beta.50.apk",
	}
	apk := ""
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			apk = p
			break
		}
	}
	if apk == "" {
		t.Skip("no official APK present to test against")
	}

	got, err := apkSigningCertHash(apk)
	if err != nil {
		t.Fatalf("apkSigningCertHash: %v", err)
	}
	want := "f4847ee3c50e284abf9bf3a51c27237f1c8413450e0625ad832143d933f81d14"
	if got != want {
		t.Fatalf("cert hash mismatch: got %s, want %s", got, want)
	}
	t.Logf("official APK cert hash OK: %s", got)
}

// 验证白名单里的所有官方哈希都能被解析器识别为合法。
func TestApkSigningCertHash_InWhitelist(t *testing.T) {
	paths := []string{
		"/root/prism-toolbox/prism工具箱_v1.0.4-beta.50.apk",
		"/root/prism-toolbox/bot-apk/prism工具箱_v1.0.4-beta.50.apk",
	}
	apk := ""
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			apk = p
			break
		}
	}
	if apk == "" {
		t.Skip("no official APK present to test against")
	}
	got, err := apkSigningCertHash(apk)
	if err != nil {
		t.Fatalf("apkSigningCertHash: %v", err)
	}
	ok := false
	for _, h := range officialSigHashes {
		if got == h {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatalf("cert hash %s is NOT in whitelist", got)
	}
}
