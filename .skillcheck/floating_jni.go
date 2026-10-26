//go:build android

package main

import "encoding/json"

// 启动一个后台协程订阅 floatingHub，把每条消息经 JNI 推给 Java。
// JNI 回调是进程内直连，SELinux 严格模式下仍可用（不依赖任何网络 socket）。
// 本文件带 //go:build android，桌面构建自动排除，notifyFloating 仅在 android 存在，无冲突。
func init() {
	go func() {
		ch := make(chan string, 500)
		floatingHub.Add(ch)
		defer floatingHub.Remove(ch)
		for msg := range ch {
			event, data := splitSSEMessage(msg)
			if event == "" {
				continue
			}
			payload, err := json.Marshal(map[string]any{"type": event, "data": json.RawMessage(data)})
			if err != nil {
				continue
			}
			notifyFloating(string(payload))
		}
	}()
}
