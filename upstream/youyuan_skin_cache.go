package main

// 皮肤图片代理缓存（/api/skin/proxy、/api/skin/warm）。
//
// 前端按名字取头像时会走这里，而不是让浏览器直连 crafatar / minotar：
//   - 只允许白名单域名，且必须 https；
//   - 解析到内网 / 回环 / 链路本地 / CGNAT 的地址一律拒绝（防 SSRF）；
//   - 成功结果落盘，带 ETag 与 7 天缓存头；失败走短期负缓存，不反复打远端。
//
// 注意：这里只代理「按玩家名/UUID 查公共头像」这类第三方图片。
// 账号页的立体皮肤不走这条路——那条链由内核按玩家名给出本人纹理
// （/api/skin/online-texture），不会按名字撞到同名陌生人的皮肤。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	yySkinTimeout    = 12 * time.Second
	yySkinMaxBytes   = 4 << 20
	yySkinNegTTL     = 5 * time.Minute
	yySkinCacheLife  = 7 * 24 * time.Hour
	yySkinCacheHead  = "public, max-age=604800, immutable"
	yySkinUserAgent  = "YouYuanToolbox/1.0 (+skin-proxy)"
	yySkinCacheDirNm = "皮肤代理缓存"
)

// 白名单本体已收敛到 protect_secrets.go（XOR 编码，运行时还原）：
//   - 精确放行的第三方头像站（Java 版公共头像）：yySecSkinHostsMap()
//   - 后缀放行的官方图源：yySecSkinSuffixesList()
//
// 后缀放行是因为网易的头像/皮肤地址形如 https://x19.gph.netease.com/xxxx，
// 是接口返回的完整 URL，没有可拼接规律，只能按域名后缀放行。仍然强制 https、
// 禁止携带凭据，拨号阶段还会再拦一次内网地址（防 DNS rebinding）。
//
// yySkinHostAllowed 先精确匹配，再按后缀匹配官方域名。
//
// 之前只有精确匹配、且只列了三个境外站，导致前端把网易头像 URL 包成
// /api/skin/proxy?url= 之后**全部被拒**，账号页和玩家列表的头像整片不显示
// （用户反馈「头像完全不会显示，永远只显示一个主账号」）。
func yySkinHostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return false
	}
	if yySecSkinHostsMap()[host] {
		return true
	}
	for _, suf := range yySecSkinSuffixesList() {
		// 必须是 xxx.netease.com 或 netease.com 本身，不能是 evilnetease.com
		if host == strings.TrimPrefix(suf, ".") || strings.HasSuffix(host, suf) {
			return true
		}
	}
	return false
}

type yySkinNegEntry struct {
	expires time.Time
}

type yySkinCache struct {
	dir    string
	mu     sync.Mutex
	neg    map[string]yySkinNegEntry
	client *http.Client
}

type yySkinCacheEntry struct {
	url   string
	body  []byte
	ctype string
	etag  string
}

func yyNewSkinCache(dir string) *yySkinCache {
	_ = os.MkdirAll(dir, 0755)
	cache := &yySkinCache{dir: dir, neg: map[string]yySkinNegEntry{}}
	cache.client = &http.Client{
		Timeout: yySkinTimeout,
		Transport: &http.Transport{
			DialContext:           yySkinDialContext,
			Proxy:                 nil,
			MaxIdleConns:          16,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   8 * time.Second,
			ResponseHeaderTimeout: yySkinTimeout,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("跳转过多")
			}
			return yySkinCheckURL(req.URL)
		},
	}
	return cache
}

// yySkinDialContext 自己解析域名、逐个校验解析结果，再直接拨 IP。
//
// 为什么不能只在 Control 里校验：http.Transport 的 DialContext 拿到的 address
// 是 "域名:端口"（如 mc-heads.net:443），域名还没解析，net.ParseIP 必然返回 nil。
// 旧实现在这里直接 `return errors.New("拨号地址无法解析")`，于是**每一个**代理请求
// 都被自己拒掉（实测连 mc-heads.net 都 400），账号页头像整片不显示。
//
// 现在改成：LookupIPAddr → 任一解析结果是内网/回环/CGNAT 就拒（防 DNS rebinding）
// → 用解析出的 IP 直接拨号，保证「校验的 IP」和「真正连的 IP」是同一个。
func yySkinDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	host = strings.Trim(host, "[]")

	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		addrs, lookupErr := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
		if lookupErr != nil {
			return nil, fmt.Errorf("解析 %s 失败：%w", host, lookupErr)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("解析 %s 得到空地址", host)
		}
		for _, a := range addrs {
			ips = append(ips, a.IP)
		}
	}

	// 全部解析结果都要过检查：只要有一条指向内网就整体拒绝，
	// 否则攻击者可以让 DNS 轮询返回「公网 + 内网」两条来绕过。
	for _, ip := range ips {
		if yySkinPrivateIP(ip) {
			return nil, fmt.Errorf("拒绝连接内网地址：%s -> %s", host, ip)
		}
	}

	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = errors.New("没有可用地址")
	}
	return nil, lastErr
}

func yySkinPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	// 100.64.0.0/10（运营商级 NAT）不是 IsPrivate，但同样不该被代理访问。
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}

// yySkinCheckURL 是入口处唯一的准入判断：协议、白名单、字符串层面的地址检查。
func yySkinCheckURL(u *url.URL) error {
	if u == nil {
		return errors.New("缺少地址")
	}
	if u.Scheme != "https" {
		return errors.New("只允许 https")
	}
	if u.User != nil {
		return errors.New("地址不允许携带凭据")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return errors.New("缺少主机名")
	}
	if !yySkinHostAllowed(host) {
		return fmt.Errorf("不在白名单：%s", host)
	}
	if ip := net.ParseIP(host); ip != nil && yySkinPrivateIP(ip) {
		return errors.New("不允许访问内网地址")
	}
	return nil
}

// yySkinDialGuard 只做「这个地址能不能拨」的判定，不负责拨号。
//
// 兼容两种输入：纯 IP（如 1.1.1.1:443）直接判；域名（如 mc-heads.net:443）
// 先解析再判。**绝不能因为「ParseIP 返回 nil」就拒绝**——那会把所有域名请求
// 一起打死（旧实现就是这个 bug，导致头像代理 100% 失败）。
// 真正的拨号在 yySkinDialContext 里，它复用同样的检查并直接连已校验的 IP。
func yySkinDialGuard(network, address string, _ any) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		if yySkinPrivateIP(ip) {
			return fmt.Errorf("拒绝连接内网地址：%s", ip)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, lookupErr := net.DefaultResolver.LookupIPAddr(ctx, host)
	if lookupErr != nil {
		return fmt.Errorf("解析 %s 失败：%w", host, lookupErr)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("解析 %s 得到空地址", host)
	}
	for _, a := range addrs {
		if yySkinPrivateIP(a.IP) {
			return fmt.Errorf("拒绝连接内网地址：%s -> %s", host, a.IP)
		}
	}
	return nil
}

func yySkinCachePath(dir, target string) string {
	sum := sha256.Sum256([]byte(target))
	return filepath.Join(dir, hex.EncodeToString(sum[:16])+".png")
}

func (c *yySkinCache) negative(target string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.neg[target]
	if !ok {
		return false
	}
	if time.Now().After(entry.expires) {
		delete(c.neg, target)
		return false
	}
	return true
}

func (c *yySkinCache) markNegative(target string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.neg) > 512 {
		c.neg = map[string]yySkinNegEntry{}
	}
	c.neg[target] = yySkinNegEntry{expires: time.Now().Add(yySkinNegTTL)}
}

// load 取图：磁盘命中直接返回，否则请求远端并落盘；失败不落盘。
func (c *yySkinCache) load(u *url.URL) ([]byte, string, string, error) {
	if err := yySkinCheckURL(u); err != nil {
		return nil, "", "", err
	}
	target := u.String()
	path := yySkinCachePath(c.dir, target)
	if body, err := os.ReadFile(path); err == nil && len(body) > 0 {
		return body, yySkinSniffType(body), yySkinETag(target, body), nil
	}
	if c.negative(target) {
		return nil, "", "", errors.New("该头像暂时取不到，稍后重试")
	}
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, "", "", err
	}
	request.Header.Set("User-Agent", yySkinUserAgent)
	request.Header.Set("Accept", "image/png,image/jpeg,image/webp,*/*")
	client := c.client
	if client == nil {
		return nil, "", "", errors.New("皮肤代理未初始化")
	}
	response, err := client.Do(request)
	if err != nil {
		c.markNegative(target)
		return nil, "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		c.markNegative(target)
		return nil, "", "", fmt.Errorf("远端返回 %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, yySkinMaxBytes+1))
	if err != nil {
		c.markNegative(target)
		return nil, "", "", err
	}
	if len(body) == 0 || len(body) > yySkinMaxBytes {
		c.markNegative(target)
		return nil, "", "", errors.New("图片大小异常")
	}
	ctype := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	if ctype == "" || !strings.HasPrefix(ctype, "image/") {
		ctype = yySkinSniffType(body)
	}
	_ = os.WriteFile(path, body, 0644)
	return body, ctype, yySkinETag(target, body), nil
}

func yySkinSniffType(body []byte) string {
	if len(body) > 8 && string(body[1:4]) == "PNG" {
		return "image/png"
	}
	if len(body) > 3 && body[0] == 0xFF && body[1] == 0xD8 {
		return "image/jpeg"
	}
	if len(body) > 12 && string(body[0:4]) == "RIFF" && string(body[8:12]) == "WEBP" {
		return "image/webp"
	}
	return "image/png"
}

func yySkinETag(target string, body []byte) string {
	sum := sha256.Sum256(append([]byte(target), body...))
	return `"` + hex.EncodeToString(sum[:12]) + `"`
}

func (c *yySkinCache) serve(w http.ResponseWriter, r *http.Request, allow304 bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "仅支持 GET", http.StatusMethodNotAllowed)
		return
	}
	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	if raw == "" {
		http.Error(w, "缺少 url", http.StatusBadRequest)
		return
	}
	u, err := url.Parse(raw)
	if err != nil {
		http.Error(w, "url 无效", http.StatusBadRequest)
		return
	}
	body, ctype, etag, err := c.load(u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", yySkinCacheHead)
	w.Header().Set("ETag", etag)
	if allow304 && etag != "" && strings.Contains(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (c *yySkinCache) handleProxy(w http.ResponseWriter, r *http.Request) {
	c.serve(w, r, true)
}

// handleWarm 批量预热：前端进账号页时把当前页的玩家名一次性丢过来。
func (c *yySkinCache) handleWarm(w http.ResponseWriter, r *http.Request) {
	warmed, failed := 0, 0
	var targets []string
	for _, name := range strings.Split(r.URL.Query().Get("names"), ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		targets = append(targets, "https://minotar.net/helm/"+url.PathEscape(name)+"/64.png")
	}
	for _, uuid := range strings.Split(r.URL.Query().Get("uuids"), ",") {
		uuid = strings.TrimSpace(uuid)
		if uuid == "" {
			continue
		}
		targets = append(targets, "https://crafatar.com/avatars/"+url.PathEscape(uuid)+"?size=64&overlay")
	}
	for _, target := range targets {
		u, err := url.Parse(target)
		if err != nil {
			failed++
			continue
		}
		if _, _, _, err := c.load(u); err != nil {
			failed++
			continue
		}
		warmed++
	}
	writeJSON(w, map[string]any{"ok": true, "warmed": warmed, "failed": failed})
}

// yySkinRoutes 注册代理与预热接口。
func yySkinRoutes(mux *http.ServeMux, dataDir string) {
	cacheDir := filepath.Join(dataDir, "插件数据文件", yySkinCacheDirNm)
	cache := yyNewSkinCache(cacheDir)
	mux.HandleFunc("/api/skin/proxy", cache.handleProxy)
	mux.HandleFunc("/api/skin/warm", cache.handleWarm)
	// 组件码取皮肤：皮肤库里每一张卡片只给组件码（skin_item_id），
	// 按名字/按账号的链路都答不了它，必须单独一条路由。见 youyuan_skin_item.go。
	yySkinItemRoutes(mux, dataDir)
}

