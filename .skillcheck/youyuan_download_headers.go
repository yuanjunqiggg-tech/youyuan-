package main

import "net/http"

func yyDownloadCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-YouYuan-Device, X-YouYuan-Trace-ID, MCP-Protocol-Version")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, Content-Length, X-Content-SHA256, X-YouYuan-Trace-ID, Retry-After")
}
