package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var yyLibraryHTTP = &http.Client{Timeout: 140 * time.Second}

func yyLibraryProxy(w http.ResponseWriter, r *http.Request) {
	if getPlatformName() == "linux" {
		yyLocalLibrary(w, r)
		return
	}
	yyRemoteMu.RLock()
	cfg := yyRemote
	yyRemoteMu.RUnlock()
	base, err := url.Parse(cfg.URL)
	if err != nil || base.Host == "" || cfg.DeviceToken == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "先在公网远控页配置设备连接地址和密钥"})
		return
	}
	base.Scheme = "https"
	base.Path = "/api/library/" + strings.TrimPrefix(r.URL.Path, "/api/youyuan/library/")
	base.RawQuery = r.URL.RawQuery
	request, err := http.NewRequestWithContext(r.Context(), r.Method, base.String(), r.Body)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	request.Header.Set("Authorization", "Bearer "+cfg.DeviceToken)
	request.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	response, err := yyLibraryHTTP.Do(request)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": fmt.Sprint(err)})
		return
	}
	defer response.Body.Close()
	for _, name := range []string{"Content-Type", "Content-Disposition", "X-Content-SHA256"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}
