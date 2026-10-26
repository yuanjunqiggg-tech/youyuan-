//go:build !android

package main

import "fmt"

// callAndroidSystem 桌面空实现：系统交互(Shizuku ADB shell)仅 Android 支持。
// Android 实现位于 libmain.go（//go:build android）。
func callAndroidSystem(method, paramsJSON string) (string, error) {
	return "", fmt.Errorf("桌面环境不支持系统交互（method=%s）", method)
}
