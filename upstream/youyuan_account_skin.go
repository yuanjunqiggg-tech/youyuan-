package main

// 账号保存时预热皮肤纹理。
//
// 为什么需要它：账号页的立体皮肤来自 playerInfoMgr（只有玩家进服后收到
// PlayerList/PlayerSkin 才有）与皮肤磁盘缓存。一个刚添加、还没进过服的机器人
// 两处都空，所以账号页长期停在「皮肤加载中」。
//
// 这里在保存账号时主动问一次认证服务，把它名下游戏的官方皮肤纹理落进同一个
// 磁盘缓存（yySkinCacheEnqueue），之后 /api/skin/online-texture?player=<名字>
// 不用等进服就能出图。
//
// 数据来源按可行性依次尝试：
//  1. 认证服务账号列表里的官方头像 URL（avatar）——网易只给头像，不是皮肤纹理；
//     只有尺寸符合皮肤规则时才会被采用，否则跳过（绝不放大/拼接造像素）。
//  2. 认证服务 skin_preview —— 由它用官方 res_url 下载并解出皮肤 PNG；
//     这条才是真正的皮肤纹理来源。
// 两条都拿不到就记日志跳过，磁盘缓存里不会出现伪造的图。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	yyAccountSkinMaxBytes = 8 << 20
	yyAccountSkinTimeout  = 30 * time.Second
)

type yyAuthAccountBrief struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Nickname      string `json:"nickname"`
	GameID        string `json:"game_id"`
	Avatar        string `json:"avatar"`
	AvatarSource  string `json:"avatar_source"`
	SkinItemID    string `json:"skin_item_id"`
	SkinSlim      bool   `json:"skin_slim"`
	SkinBodyKnown bool   `json:"skin_body_known"`
	TokenHint     string `json:"token_hint"`
}

// yyAccountSkinClient 是认证服务调用的 HTTP 客户端。
// 单独一个变量是为了测试：测试把它换成信任自签证书的客户端，指向本地假认证服务。
var yyAccountSkinClient = &http.Client{Timeout: yyAccountSkinTimeout}

// 会话刷新冷却：每个账号 60 秒内只 refresh 一次。
var (
	yyAccountSkinRefreshMu sync.Mutex
	yyAccountSkinRefreshAt = map[string]time.Time{}
)

func yyAccountSkinDeviceToken() string {
	yyRemoteMu.RLock()
	defer yyRemoteMu.RUnlock()
	return strings.TrimSpace(yyRemote.DeviceToken)
}

// yyAccountSkinOrigin 只接受 https 的认证服务地址，避免把设备密钥发到明文端点。
func yyAccountSkinOrigin(a yyManagedAccount) string {
	origin := strings.TrimRight(strings.TrimSpace(a.AuthURL), "/")
	if origin == "" {
		origin = yyGameAuthOrigin()
	}
	u, e := url.Parse(origin)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	return origin
}

func yyAccountSkinPost(ctx context.Context, origin, token string, body []byte) ([]byte, int, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, origin+"/api/accounts", bytes.NewReader(body))
	if e != nil {
		return nil, 0, e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "YouYuan-Toolbox/"+yyAppVersion)
	resp, e := yyAccountSkinClient.Do(req)
	if e != nil {
		return nil, 0, e
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, yyAccountSkinMaxBytes))
	if e != nil {
		return nil, resp.StatusCode, e
	}
	if resp.StatusCode != http.StatusOK {
		// 认证服务对内核类错误返回 400 + JSON 错误体。身体里的原因
		// （如「读取皮肤设置：请先登录(10)」）比状态码有用，一起带回去。
		return raw, resp.StatusCode, fmt.Errorf("认证服务返回 %s", resp.Status)
	}
	return raw, resp.StatusCode, nil
}

// yyAccountSkinCall 发一次账号动作请求并解析 JSON 响应。
// 400 也尝试解析：错误原因全在身体里，丢了就只剩一句「认证服务返回 400」。
func yyAccountSkinCall(ctx context.Context, origin, token string, body []byte) (map[string]any, error) {
	raw, status, e := yyAccountSkinPost(ctx, origin, token, body)
	if e != nil && status != http.StatusBadRequest {
		return nil, e
	}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		if e != nil {
			return nil, e
		}
		return nil, fmt.Errorf("认证服务响应无法解析")
	}
	return data, nil
}

// yyAccountSkinCallError 从认证服务的错误响应里取人话原因。
func yyAccountSkinCallError(data map[string]any) string {
	if data == nil {
		return ""
	}
	for _, key := range []string{"error", "error_info", "message"} {
		if text := strings.TrimSpace(fmt.Sprint(data[key])); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

// yyAccountSkinLoginExpired 判断这次失败是不是「认证服务的游戏会话过期」。
// 皮肤设置接口（pe-get-user-setting-list）只认有效会话，vault 里的 client
// 一过期就返回「读取皮肤设置：请先登录(10)」——2026-10-25 线上 5 个账号
// 全部命根于此。此时先 refresh 再问一次就能拿到官方皮肤。
func yyAccountSkinLoginExpired(data map[string]any) bool {
	if data == nil || data["ok"] == true {
		return false
	}
	text := yyAccountSkinCallError(data)
	return strings.Contains(text, "请先登录") || strings.Contains(text, "登录已过期") || strings.Contains(text, "未登录")
}

func yyAccountSkinAuthAccounts(ctx context.Context, origin, token string) []yyAuthAccountBrief {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/api/accounts", nil)
	if e != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "YouYuan-Toolbox/"+yyAppVersion)
	resp, e := yyAccountSkinClient.Do(req)
	if e != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var data struct {
		OK       bool                 `json:"ok"`
		Accounts []yyAuthAccountBrief `json:"accounts"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, yyAccountSkinMaxBytes)).Decode(&data) != nil {
		return nil
	}
	return data.Accounts
}

// yyAccountSkinAuthAccountsBackground 给批量预热用：一次取回整批账号列表。
// 返回非空 reason 表示这次根本没拿到列表（没登录 / 认证地址无效 / 请求失败），
// 调用方据此把整批记成失败，而不是逐个账号再打一遍。
func yyAccountSkinAuthAccountsBackground(origin string) ([]yyAuthAccountBrief, string) {
	if origin == "" {
		return nil, "认证地址不是 https，无法安全取官方皮肤"
	}
	token := yyAccountSkinDeviceToken()
	if token == "" {
		return nil, "未登录游园平台，拿不到设备令牌"
	}
	ctx, cancel := context.WithTimeout(context.Background(), yyAccountSkinTimeout)
	defer cancel()
	rows := yyAccountSkinAuthAccounts(ctx, origin, token)
	if len(rows) == 0 {
		return nil, "认证服务没有返回账号列表"
	}
	return rows, ""
}

// yyAccountSkinMatch 把本地账号对到认证服务上的那条游戏凭据。
// 前端保存凭据时用 "yy_"+认证账号id 作为本地 id，这是最强线索；
// 其次比对 token 提示（前 5 位），最后才退回昵称/游戏名。
func yyAccountSkinMatch(a yyManagedAccount, rows []yyAuthAccountBrief) (yyAuthAccountBrief, bool) {
	if len(rows) == 0 {
		return yyAuthAccountBrief{}, false
	}
	wanted := strings.TrimPrefix(a.ID, "yy_")
	for _, row := range rows {
		if wanted != "" && row.ID == wanted {
			return row, true
		}
	}
	if hint := strings.TrimSpace(a.Token); len(hint) >= 5 {
		prefix := hint[:5] + "…"
		for _, row := range rows {
			if row.TokenHint == prefix {
				return row, true
			}
		}
	}
	names := yyAccountSkinNames(a)
	for _, row := range rows {
		for _, name := range names {
			if name != "" && (name == row.Nickname || name == row.Name) {
				return row, true
			}
		}
	}
	return yyAuthAccountBrief{}, false
}

// yyAccountSkinNames 返回这个账号可能被前端用来查纹理的名字。
// 只放确认过的游戏名、认证服务昵称与账号备注名：纹理本身来自本账号，
// 按备注名命中不会拿到陌生人的皮肤。
func yyAccountSkinNames(a yyManagedAccount) []string {
	names := []string{}
	if name, ok := yyPoolLocalGameIdentity(a); ok && name != "" {
		names = append(names, name)
	}
	if name, _ := yyPoolRememberedGameIdentity(a.ID); name != "" {
		names = append(names, name)
	}
	for _, name := range []string{a.Name, a.ID} {
		if name = strings.TrimSpace(name); name != "" && name != "main" {
			names = append(names, name)
		}
	}
	seen := map[string]bool{}
	unique := names[:0]
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		unique = append(unique, name)
	}
	return unique
}

// yySkinTexturePNG 把 PNG 转成内核用的 RGBA 像素，尺寸必须符合皮肤规则。
// 不符合就返回 false —— 这是"不造像素"的最后一道闸。
func yySkinTexturePNG(raw []byte) ([]byte, uint32, uint32, bool) {
	cfg, format, e := image.DecodeConfig(bytes.NewReader(raw))
	if e != nil || format != "png" {
		return nil, 0, 0, false
	}
	w, h := cfg.Width, cfg.Height
	if w < 64 || w > 1024 || w%64 != 0 || (h != w && h*2 != w) {
		return nil, 0, 0, false
	}
	img, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil {
		return nil, 0, 0, false
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Src)
	return dst.Pix, uint32(w), uint32(h), true
}

func yySkinTextureDataURL(raw string) ([]byte, uint32, uint32, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, 0, 0, false
	}
	if idx := strings.Index(raw, "base64,"); idx >= 0 {
		raw = raw[idx+len("base64,"):]
	}
	data, e := base64.StdEncoding.DecodeString(raw)
	if e != nil || len(data) == 0 {
		return nil, 0, 0, false
	}
	return yySkinTexturePNG(data)
}

func yyAccountSkinOfficialURL(raw string) bool {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	// 与 /api/skin/proxy 的白名单保持一致：网易自有域名 + 它的 CDN 后缀。
	// 域名清单已收敛到 protect_secrets.go（XOR 编码），判定语义不变。
	return yySecIsAccountSkinHost(h)
}

func yyAccountSkinDownload(ctx context.Context, raw string) ([]byte, bool) {
	if !yyAccountSkinOfficialURL(raw) {
		return nil, false
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if e != nil {
		return nil, false
	}
	resp, e := yyAccountSkinClient.Do(req)
	if e != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, yyAccountSkinMaxBytes+1))
	if e != nil || len(data) == 0 || len(data) > yyAccountSkinMaxBytes {
		return nil, false
	}
	return data, true
}

// yyAccountSkinRefresh 问认证服务刷新一次这个账号的游戏会话。
//
// 为什么需要它：皮肤设置接口只认有效会话，vault 里的 client 过期时
// skin_preview 返回「读取皮肤设置：请先登录(10)」。refresh 就是认证服务
// 自己每 25 分钟做的那件事（UpdateToken，失败再用 cookie 重新登录），
// 不改变任何账号状态。每个账号 60 秒内只刷新一次，防止把认证服务打爆。
func yyAccountSkinRefresh(ctx context.Context, origin, token, accountID string) error {
	yyAccountSkinRefreshMu.Lock()
	if at, seen := yyAccountSkinRefreshAt[accountID]; seen && time.Since(at) < time.Minute {
		yyAccountSkinRefreshMu.Unlock()
		return nil // 冷却期内当作已刷新，直接重试
	}
	yyAccountSkinRefreshAt[accountID] = time.Now()
	yyAccountSkinRefreshMu.Unlock()
	body, _ := json.Marshal(map[string]any{"action": "refresh", "id": accountID})
	data, e := yyAccountSkinCall(ctx, origin, token, body)
	if e != nil {
		return e
	}
	if data["ok"] != true {
		return fmt.Errorf("认证服务刷新会话失败：%s", yyAccountSkinCallError(data))
	}
	return nil
}

// yyAccountSkinPreviewOnce 问一次认证服务的皮肤预览接口。
// 返回 (纹理, 宽, 高, 成功, 会话过期, 原因)。
func yyAccountSkinPreviewOnce(ctx context.Context, origin, token, accountID string) ([]byte, uint32, uint32, bool, bool, string) {
	body, _ := json.Marshal(map[string]any{"action": "skin_preview", "id": accountID})
	data, e := yyAccountSkinCall(ctx, origin, token, body)
	if e != nil {
		debugLog("[皮肤] 账号 %s 取认证皮肤失败: %v", accountID, e)
		return nil, 0, 0, false, false, e.Error()
	}
	if data["ok"] != true {
		reason := yyAccountSkinCallError(data)
		if reason == "" {
			reason = "认证服务没有给出皮肤"
		}
		debugLog("[皮肤] 账号 %s 认证服务没有给出皮肤: %s", accountID, reason)
		return nil, 0, 0, false, yyAccountSkinLoginExpired(data), reason
	}
	texture, _ := data["texture"].(string)
	if texture == "" {
		return nil, 0, 0, false, false, "认证服务没有返回皮肤纹理"
	}
	pix, w, h, ok := yySkinTextureDataURL(texture)
	if !ok {
		return nil, 0, 0, false, false, "皮肤纹理不是标准尺寸，无法套用标准骨骼"
	}
	return pix, w, h, true, false, ""
}

// yyAccountSkinPreview 取这个账号的官方皮肤纹理。
// 会话过期时先让认证服务 refresh，再问一次——不 refresh 就会永远停在
// 「请先登录(10)」，用户看到的就是「官方接口根本没有皮肤」。
func yyAccountSkinPreview(ctx context.Context, origin, token, accountID string) ([]byte, uint32, uint32, string) {
	pix, w, h, ok, expired, reason := yyAccountSkinPreviewOnce(ctx, origin, token, accountID)
	if ok || !expired {
		return pix, w, h, reason
	}
	if e := yyAccountSkinRefresh(ctx, origin, token, accountID); e != nil {
		debugLog("[皮肤] 账号 %s 刷新认证会话失败: %v", accountID, e)
		return nil, 0, 0, reason + "（刷新会话也没成功）"
	}
	debugLog("[皮肤] 账号 %s 会话已过期，刷新后重试取皮肤", accountID)
	pix, w, h, ok, _, reason = yyAccountSkinPreviewOnce(ctx, origin, token, accountID)
	if !ok && reason == "" {
		reason = "官方皮肤接口没有返回可用纹理"
	}
	return pix, w, h, reason
}

// yyAccountSkinResult 是一次官方皮肤获取的完整结果。
//
// 以前这里只返回 bool，失败时调用方只能记一句「跳过」，分不清是没登录、
// 没凭据、还是官方接口不给纹理——用户反馈「你也没把它提前下载了」时
// 日志里看不出到底卡在哪一步。现在把原因和实际落盘用的名字都带出来。
type yyAccountSkinResult struct {
	Data   []byte
	Width  uint32
	Height uint32
	Names  []string // 实际用来落盘的名字（含认证服务返回的官方昵称）
	Reason string   // 失败原因；成功时为空
}

func (r yyAccountSkinResult) OK() bool { return len(r.Data) > 0 && r.Width > 0 && r.Height > 0 }

// yyAccountSkinFetch 取这个账号的官方皮肤纹理（RGBA 原始像素 + 尺寸）。
//
// rows 传 nil 时自己去认证服务拉账号列表；批量预热时由调用方共享一份，
// 避免一个账号打一次 /api/accounts。来源按可行性依次尝试：
//  1. 账号已有的官方头像 URL —— 网易通常只给头像，尺寸不是皮肤规则就跳过；
//  2. 认证服务 skin_preview —— 由它用官方 res_url 下载并解包成 PNG，
//     这是真正的皮肤纹理来源（res_url 没有拼接规律，只能接口取）。
func yyAccountSkinFetch(a yyManagedAccount, rows []yyAuthAccountBrief) yyAccountSkinResult {
	names := yyAccountSkinNames(a)
	origin, token := yyAccountSkinOrigin(a), yyAccountSkinDeviceToken()
	if origin == "" {
		return yyAccountSkinResult{Names: names, Reason: "认证地址不是 https，无法安全取官方皮肤"}
	}
	if token == "" {
		return yyAccountSkinResult{Names: names, Reason: "未登录游园平台，拿不到设备令牌"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), yyAccountSkinTimeout)
	defer cancel()
	if rows == nil {
		rows = yyAccountSkinAuthAccounts(ctx, origin, token)
		if len(rows) == 0 {
			return yyAccountSkinResult{Names: names, Reason: "认证服务没有返回账号列表"}
		}
	}
	row, ok := yyAccountSkinMatch(a, rows)
	if !ok {
		return yyAccountSkinResult{Names: names, Reason: "认证服务里没有这个账号对应的游戏凭据"}
	}
	if row.Nickname != "" && !yyAccountSkinHasName(names, row.Nickname) {
		names = append(names, row.Nickname)
	}
	// 1) 账号已有的官方头像 URL：通常只是一张头像，尺寸不是皮肤纹理，
	//    此时直接跳过，绝不放大拼接当皮肤用。
	if row.Avatar != "" {
		if raw, ok := yyAccountSkinDownload(ctx, row.Avatar); ok {
			if data, w, h, ok := yySkinTexturePNG(raw); ok {
				return yyAccountSkinResult{Data: data, Width: w, Height: h, Names: names}
			}
		}
	}
	// 2) 认证服务的皮肤下载地址（skin_preview 由服务端取官方 res_url 并解包）。
	if data, w, h, reason := yyAccountSkinPreview(ctx, origin, token, row.ID); len(data) > 0 {
		return yyAccountSkinResult{Data: data, Width: w, Height: h, Names: names}
	} else if reason != "" {
		debugLog("[皮肤] 账号 %s 官方皮肤接口不可用: %s", a.ID, reason)
	}
	return yyAccountSkinResult{Names: names, Reason: "官方皮肤接口没有返回可用纹理"}
}

func yyAccountSkinHasName(names []string, name string) bool {
	for _, n := range names {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

// yyAccountSkinStore 把纹理按名字全部落盘，返回写入的名字数。
func yyAccountSkinStore(result yyAccountSkinResult) int {
	if !result.OK() {
		return 0
	}
	written := 0
	for _, name := range result.Names {
		yySkinCacheEnqueue(name, result.Data, result.Width, result.Height)
		written++
	}
	return written
}

// yyAccountPrimeSkin 保存账号后预热皮肤。必须在 goroutine 里调用：
// 认证服务可能慢，不能拖住建账号的请求。
func yyAccountPrimeSkin(a yyManagedAccount) {
	if yySkinDiskRoot == "" {
		debugLog("[皮肤] 磁盘缓存未初始化，账号 %s 无法预热", a.ID)
		return
	}
	names := yyAccountSkinNames(a)
	if len(names) == 0 {
		debugLog("[皮肤] 账号 %s 没有任何可用名字，无法预热", a.ID)
		return
	}
	if yySkinCacheHasAny(names) {
		return
	}
	result := yyAccountSkinFetch(a, nil)
	if !result.OK() {
		debugLog("[皮肤] 账号 %s 预热失败：%s", a.ID, result.Reason)
		return
	}
	written := yyAccountSkinStore(result)
	debugLog("[皮肤] 账号 %s 已落盘 %dx%d（%d 个名字）", a.ID, result.Width, result.Height, written)
}

func yySkinCacheHasAny(names []string) bool {
	for _, name := range names {
		if _, _, ok := yySkinCacheHas(name); ok {
			return true
		}
	}
	return false
}
