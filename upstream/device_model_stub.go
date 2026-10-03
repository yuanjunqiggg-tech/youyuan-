//go:build !android

package main
import "runtime"

func getDeviceModel() string {
	return "desktop"
}

// getPlatformName 返回运行平台名（Lua system.platform() 用）。
func getPlatformName() string {
	return runtime.GOOS
}
