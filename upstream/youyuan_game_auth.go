package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"bot-apk/config"
)

// yyGameAuthOrigin returns the configured authentication origin, falling back
// to the product authentication host so a fresh install without a saved
// config still reaches the real service.
func yyGameAuthOrigin() string {
	if cfg := config.LoadConfig(); cfg != nil && strings.TrimSpace(cfg.AuthURL) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.AuthURL), "/")
	}
	return yySecAuthOriginFallback()
}

// ═══ 皮肤预览缓存 ═══
//
// 线上实测（2026-10-26，box 127.0.0.1:19429）：同一个账号连续两次 skin_preview
// 分别耗时 5.20s 和 2.33s —— 两次都在实时打网易（取设置 → 取下载地址 → 下载纹理
// → 解包 → base64），完全没有应用层缓存。用户点开「皮肤预览」就是盯着空舞台等
// 五秒，等不到就以为「皮肤不显示」。
//
// 皮肤纹理在一个账号换皮肤之前是不变的，所以按账号 id 缓存整份成功响应。
// 换皮肤（action:"skin"）成功后立刻清掉该账号的缓存，避免看到旧图。
// 只缓存 skin_preview，其余动作一律直通——缓存范围越小越不容易出诡异问题。
const (
	yyGameAuthPreviewTTL = 5 * time.Minute
	yyGameAuthPreviewMax = 32 // 5 个账号 × 十几 KB，32 条上限纯属防御
)

type yyGameAuthPreviewEntry struct {
	body []byte
	at   time.Time
}

var yyGameAuthPreviewCache = struct {
	sync.Mutex
	m map[string]yyGameAuthPreviewEntry
}{m: map[string]yyGameAuthPreviewEntry{}}

func yyGameAuthPreviewGet(id string) ([]byte, bool) {
	yyGameAuthPreviewCache.Lock()
	defer yyGameAuthPreviewCache.Unlock()
	e, ok := yyGameAuthPreviewCache.m[id]
	if !ok {
		return nil, false
	}
	if time.Since(e.at) > yyGameAuthPreviewTTL {
		delete(yyGameAuthPreviewCache.m, id)
		return nil, false
	}
	return e.body, true
}

func yyGameAuthPreviewPut(id string, body []byte) {
	yyGameAuthPreviewCache.Lock()
	defer yyGameAuthPreviewCache.Unlock()
	if len(yyGameAuthPreviewCache.m) >= yyGameAuthPreviewMax {
		// 淘汰最旧的一条。缓存很小，不值得上真正的 LRU。
		oldest, oldestAt := "", time.Time{}
		for k, v := range yyGameAuthPreviewCache.m {
			if oldest == "" || v.at.Before(oldestAt) {
				oldest, oldestAt = k, v.at
			}
		}
		if oldest != "" {
			delete(yyGameAuthPreviewCache.m, oldest)
		}
	}
	yyGameAuthPreviewCache.m[id] = yyGameAuthPreviewEntry{body: body, at: time.Now()}
}

func yyGameAuthPreviewDrop(id string) {
	yyGameAuthPreviewCache.Lock()
	delete(yyGameAuthPreviewCache.m, id)
	yyGameAuthPreviewCache.Unlock()
}

// yyGameAuthPreviewReset 只给测试用：清空全部缓存，避免用例之间互相污染。
func yyGameAuthPreviewReset() {
	yyGameAuthPreviewCache.Lock()
	yyGameAuthPreviewCache.m = map[string]yyGameAuthPreviewEntry{}
	yyGameAuthPreviewCache.Unlock()
}

// yyGameAuthUpstream 把一次请求原样转给认证服务，返回状态码与响应体。
// 抽出来是为了让测试能对着 httptest 服务器跑，不用真的连线上。
func yyGameAuthUpstream(ctx context.Context, origin, key, method string, body []byte) (int, []byte) {
	req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(origin, "/")+"/api/accounts", bytes.NewReader(body))
	if e != nil {
		return 500, []byte(`{"ok":false,"error":"请求失败"}`)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "YouYuan-Toolbox/"+yyAppVersion)
	client := &http.Client{Timeout: 75 * time.Second, Transport: http.DefaultTransport}
	resp, e := client.Do(req)
	if e != nil {
		return 200, []byte(`{"ok":false,"error":"独立认证服务连接失败，请检查网络后重试"}`)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 200000))
	return resp.StatusCode, respBody
}

// yyGameAuthServe 是 /api/youyuan/game-auth 的实际处理逻辑。
// origin / key 由调用方给，方便测试注入。
func yyGameAuthServe(ctx context.Context, origin, key, method string, body []byte, w http.ResponseWriter) {
	var op struct {
		Action string `json:"action"`
		ID     string `json:"id"`
	}
	_ = json.Unmarshal(body, &op)

	cacheable := method == http.MethodPost && op.Action == "skin_preview" && op.ID != ""
	if cacheable {
		if hit, ok := yyGameAuthPreviewGet(op.ID); ok {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(hit)
			return
		}
	}

	status, respBody := yyGameAuthUpstream(ctx, origin, key, method, body)
	okResp := status == http.StatusOK && bytes.Contains(respBody, []byte(`"ok":true`))

	if cacheable && okResp {
		yyGameAuthPreviewPut(op.ID, respBody)
	}
	// 换皮肤成功后必须让缓存失效，否则最多 5 分钟都在给人看旧皮肤。
	if method == http.MethodPost && op.Action == "skin" && op.ID != "" && okResp {
		yyGameAuthPreviewDrop(op.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(respBody)
}

// Credentials are managed through the owner's authenticated cloud vault.
// Device keys stay in the private configuration and never reach page markup.
func yyGameAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/game-auth", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "POST" {
			http.Error(w, "GET/POST required", 405)
			return
		}
		yyRemoteMu.RLock()
		key := yyRemote.DeviceToken
		yyRemoteMu.RUnlock()
		if key == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "请先登录游园平台账号，或在公网控制设置中配置自己的设备密钥"})
			return
		}
		body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 100000))
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求过大"})
			return
		}
		yyGameAuthServe(r.Context(), yyGameAuthOrigin(), key, r.Method, body, w)
	})
}
