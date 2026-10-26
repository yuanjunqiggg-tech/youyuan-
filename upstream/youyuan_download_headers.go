package main

import "net/http"

func yyDownloadCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, DELETE, OPTIONS")
	// DSH-04：同源请求不需要 Authorization；跨站预检在 CORS 层已被 403。
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-YouYuan-Console-Token, X-YouYuan-Trace-ID, MCP-Protocol-Version")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, Content-Length, X-Content-SHA256, X-YouYuan-Trace-ID, Retry-After")
}
