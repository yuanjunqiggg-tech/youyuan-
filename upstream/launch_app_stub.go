//go:build !android

package main

// 桌面构建的 launchAndroidApp 空实现（启动应用仅 Android 支持）。
func launchAndroidApp(pkg string) {}
