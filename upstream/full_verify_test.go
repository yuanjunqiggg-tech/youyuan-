package main

import (
	"os"
	"testing"
)

func TestFullVerifyNewApk(t *testing.T) {
	apk := "/root/prism-toolbox/bot-apk/prism工具箱.apk"
	if _, err := os.Stat(apk); err != nil {
		t.Skipf("该测试仅针对原作者发行包；新版由 apksigner 验证自己的发行签名: %v",err)
	}
	// 权威路径（运行时路径）
	SetApkPath(apk)
	VerifyApkIntegrity()
	t.Logf("权威路径: tampered=%v reason=%q", IsApkTampered(), TamperReason())

	// 兜底路径（模拟 apkPath 为空，Java 上报）
	apkPath = ""
	SetApkSignatureHash("f4847ee3c50e284abf9bf3a51c27237f1c8413450e0625ad832143d933f81d14")
	SetApkPackageName("com.prismtool.box")
	VerifyApkIntegrity()
	t.Logf("兜底路径: tampered=%v reason=%q", IsApkTampered(), TamperReason())
}
