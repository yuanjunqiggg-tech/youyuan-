package main

import "fmt"

func init() {
	yyExtraMethods["diagnostics.receiveTransport"] = "无参数；当前连接协商协议、真实注册包ID与解码前帧/字节/未知包计数。decode_panics是解码中断，trailing_bytes_frames是完成已知字段后剩余扩展字节，decoded_frames是完成解码的帧，returned_packets是返回上层的包数；decode_errors是旧版异常合计，不能当丢包数。只保存数字元数据，不保存认证或包内容。不依赖服务器组件。单批数量限制与业务留存容量分别报告，不冒充全服广播权限。"
}
func yyReceiveTransport(bm *BotManager, args []any) (any, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("receiveTransport()不接收参数")
	}
	if bm == nil {
		return map[string]any{"connected": false}, nil
	}
	bm.mu.Lock()
	conn, connected := bm.conn, bm.connected
	bm.mu.Unlock()
	if conn == nil {
		return map[string]any{"connected": false}, nil
	}
	result := conn.ReceiveDiagnostics()
	result["connected"] = connected
	return result, nil
}
