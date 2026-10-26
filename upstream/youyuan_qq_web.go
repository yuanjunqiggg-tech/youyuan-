package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// 机器人联网读取网页。
//
// 安全边界（这是对外开放的能力，必须写死）：
//   - 只允许 http/https；
//   - 解析出的每个 IP 都必须是公网地址（拒绝回环/内网/链路本地/保留段），
//     并且在**拨号层**再校验一次，防 DNS 重绑定；
//   - 不跟随跨协议跳转，最多 3 跳，每跳重新校验；
//   - 单次最多读 512KB、超时 8 秒；
//   - 返回值做换行归一 + 长度截断，避免把整页 HTML 灌进上下文。

const (
	yyWebFetchLimitBytes = 512 << 10
	yyWebFetchTimeout    = 8 * time.Second
	yyWebFetchMaxChars   = 6000
)

func yyWebFetchToolDef() ToolDef {
	return ToolDef{
		Name: "web_fetch",
		Description: "读取一个公开网页的文本内容（只读，最多 6000 字）。当用户明确要你查资料、看链接、核实最新信息时使用。" +
			"只能访问公网 http/https，不能访问内网、本机地址或需要登录的页面。",
		Parameters: yySchema(map[string]any{
			"url":   map[string]any{"type": "string"},
			"chars": map[string]any{"type": "integer"},
		}, "url"),
	}
}

// yyWebIPAllowed 判断一个 IP 是否允许访问（必须是公网单播）。
func yyWebIPAllowed(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	// 云厂商元数据地址段（169.254 已被 IsLinkLocalUnicast 覆盖，这里补常见保留段）
	if ip4 := ip.To4(); ip4 != nil {
		switch {
		case ip4[0] == 0, ip4[0] == 127:
			return false
		case ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127: // CGNAT
			return false
		case ip4[0] == 169 && ip4[1] == 254:
			return false
		}
	}
	return true
}

// yyWebCheckURL 校验协议与主机解析结果。
func yyWebCheckURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("链接不能为空")
	}
	u, e := url.Parse(raw)
	if e != nil {
		return nil, fmt.Errorf("链接格式不正确")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("只支持 http/https 链接")
	}
	if u.User != nil {
		return nil, fmt.Errorf("链接里不能带用户名密码")
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("链接缺少域名")
	}
	// 直接写 IP 的，先判一次；写域名的，解析后逐个判。
	if ip := net.ParseIP(host); ip != nil {
		if !yyWebIPAllowed(ip) {
			return nil, fmt.Errorf("不允许访问内网或本机地址")
		}
		return u, nil
	}
	ips, e := net.LookupIP(host)
	if e != nil || len(ips) == 0 {
		return nil, fmt.Errorf("域名解析失败")
	}
	for _, ip := range ips {
		if !yyWebIPAllowed(ip) {
			return nil, fmt.Errorf("不允许访问内网或本机地址")
		}
	}
	return u, nil
}

// yyWebFetch 抓取并返回纯文本（已截断）。
func yyWebFetch(ctx context.Context, raw string, maxChars int) (string, error) {
	if maxChars <= 0 || maxChars > yyWebFetchMaxChars {
		maxChars = yyWebFetchMaxChars
	}
	cur := strings.TrimSpace(raw)
	client := &http.Client{
		Timeout: yyWebFetchTimeout,
		// 手动跳转：每一跳都要重新过 IP 校验，避免 302 到内网。
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("跳转次数过多")
			}
			_, e := yyWebCheckURL(req.URL.String())
			return e
		},
		Transport: &http.Transport{
			// 拨号层再校验一次：防 DNS 重绑定（解析时是公网、连接时变内网）。
			DialContext: func(dctx context.Context, network, addr string) (net.Conn, error) {
				host, port, e := net.SplitHostPort(addr)
				if e != nil {
					return nil, e
				}
				// 域名要在这一层自己解析：拿到 IP 再校验，最后拨 IP。
				// 这样即使 DNS 在两次查询之间换了答案（DNS 重绑定），也拦得住。
				ips := []net.IP{}
				if ip := net.ParseIP(host); ip != nil {
					ips = append(ips, ip)
				} else {
					resolved, e := net.LookupIP(host)
					if e != nil || len(resolved) == 0 {
						return nil, fmt.Errorf("域名解析失败")
					}
					ips = resolved
				}
				d := net.Dialer{Timeout: 5 * time.Second}
				var lastErr error
				for _, ip := range ips {
					if !yyWebIPAllowed(ip) {
						lastErr = fmt.Errorf("不允许连接内网地址")
						continue
					}
					conn, e := d.DialContext(dctx, network, net.JoinHostPort(ip.String(), port))
					if e == nil {
						return conn, nil
					}
					lastErr = e
				}
				if lastErr == nil {
					lastErr = fmt.Errorf("没有可用的地址")
				}
				return nil, lastErr
			},
			// 自定义 DialContext 后 Go 不会自己解析域名，这里用带校验的解析器兜住。
			Proxy: nil,
		},
	}
	u, e := yyWebCheckURL(cur)
	if e != nil {
		return "", e
	}
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return "", fmt.Errorf("请求构造失败")
	}
	req.Header.Set("User-Agent", "YouYuanBot/1.0 (+"+yySecPlatformOrigin()+")")
	req.Header.Set("Accept", "text/html,text/plain,application/json;q=0.9,*/*;q=0.5")
	resp, e := client.Do(req)
	if e != nil {
		return "", fmt.Errorf("抓取失败：%v", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("目标返回 HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, yyWebFetchLimitBytes))
	text := yyWebStrip(string(body))
	if utf8.RuneCountInString(text) > maxChars {
		runes := []rune(text)
		text = string(runes[:maxChars]) + "\n…（已截断）"
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("页面没有可读文本（可能需要登录或是纯脚本页面）")
	}
	return text, nil
}

// yyWebStrip 去掉标签与多余空白，保留可读文本。
func yyWebStrip(s string) string {
	// 先干掉脚本/样式块，避免把 JSON 和 CSS 灌进上下文。
	lower := strings.ToLower(s)
	for _, tag := range []string{"script", "style", "noscript", "svg"} {
		open := "<" + tag
		close := "</" + tag + ">"
		for {
			i := strings.Index(lower, open)
			if i < 0 {
				break
			}
			j := strings.Index(lower[i:], close)
			if j < 0 {
				s = s[:i]
				lower = strings.ToLower(s)
				break
			}
			s = s[:i] + " " + s[i+j+len(close):]
			lower = strings.ToLower(s)
		}
	}
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
			}
			b.WriteRune(' ')
		case depth == 0:
			b.WriteRune(r)
		}
	}
	// 归一空白：连续空白压成一个，最多保留一个空行。
	text := strings.Join(strings.Fields(b.String()), " ")
	for _, ent := range [][2]string{{"&nbsp;", " "}, {"&amp;", "&"}, {"&lt;", "<"}, {"&gt;", ">"}, {"&quot;", "\""}, {"&#39;", "'"}} {
		text = strings.ReplaceAll(text, ent[0], ent[1])
	}
	return text
}
