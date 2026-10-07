package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
)

func init() {
	for method, doc := range map[string]string{
		"mcp.connection":   "无参数，读取当前实例真实本机MCP地址、远端状态与账号。密钥由宿主内部使用，不返回给模型或聊天。",
		"mcp.tools":        "无参数，读取本机实际MCP工具列表，不自动把全部说明塞进AI上下文。",
		"mcp.resources":    "无参数，读取本机MCP资源目录。",
		"mcp.readResource": "uri，按需读取真实MCP资源。",
		"mcp.call":         "tool_name,arguments对象；使用本机真实MCP工具分发与权限，自动绑定当前账号，不允许指定别的账号。不访问旧固定云桥，不导出密钥。",
	} {
		yyExtraMethods[method] = doc
	}
	yySDKNames = append(yySDKNames, "mcp")
}
func yyPluginMCP(bm *BotManager, method string, args []any) (any, error) {
	account := "main"
	if bm != nil && bm != getBotManagerLocked() {
		account = bm.idStr
	}
	if method == "mcp.connection" {
		yyRemoteMu.RLock()
		defer yyRemoteMu.RUnlock()
		return map[string]any{"account": account, "local_mcp_url": "http://127.0.0.1:" + yyCorePort + "/mcp/", "public_urls": yyPublicURLs(yyRemote.URL), "relay_connected": yyRemote.Enabled && yyRemoteState == "已连接", "authentication": "host-managed", "configured": yyRemote.Key != ""}, nil
	}
	rpc := map[string]any{"jsonrpc": "2.0", "id": 1}
	params := map[string]any{}
	switch method {
	case "mcp.tools":
		rpc["method"] = "tools/list"
	case "mcp.resources":
		rpc["method"] = "resources/list"
	case "mcp.readResource":
		uri, e := yyArgString(args, 0, "")
		if e != nil {
			return nil, e
		}
		rpc["method"] = "resources/read"
		params["uri"] = uri
	case "mcp.call":
		name, e := yyArgString(args, 0, "")
		if e != nil {
			return nil, e
		}
		arguments := map[string]any{}
		if len(args) > 1 {
			given, ok := args[1].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("MCP arguments需要对象")
			}
			for k, v := range given {
				arguments[k] = v
			}
		}
		if requested, _ := arguments["account"].(string); requested != "" && requested != account {
			return nil, fmt.Errorf("跨账号请使用accounts定向授权API")
		}
		arguments["account"] = account
		rpc["method"] = "tools/call"
		params["name"] = name
		params["arguments"] = arguments
	default:
		return nil, fmt.Errorf("未知MCP方法")
	}
	rpc["params"] = params
	b, e := json.Marshal(rpc)
	if e != nil {
		return nil, e
	}
	req := httptest.NewRequest("POST", "/mcp/", bytes.NewReader(b))
	yyRemoteMu.RLock()
	req.Header.Set("Authorization", "Bearer "+yyRemote.Key)
	yyRemoteMu.RUnlock()
	recorder := httptest.NewRecorder()
	yyMCP(recorder, req)
	if recorder.Code != 200 {
		return nil, fmt.Errorf("本机MCP HTTP %d", recorder.Code)
	}
	var result map[string]any
	if e = json.Unmarshal(recorder.Body.Bytes(), &result); e != nil {
		return nil, e
	}
	if failure := result["error"]; failure != nil {
		return nil, fmt.Errorf("MCP: %v", failure)
	}
	if result["result"] == nil {
		return nil, fmt.Errorf("MCP响应缺少result: %s", strings.TrimSpace(recorder.Body.String()))
	}
	return result["result"], nil
}
