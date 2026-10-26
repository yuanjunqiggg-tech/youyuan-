package main

// 按组件码取皮肤：/api/skin/item-texture?item_id=<组件码>、/api/skin/item-library
//
// 为什么需要它：皮肤库里每一张卡片都只认「组件码」（网易的 skin_item_id），
// 而本机原有的取图链路全都按**玩家名**或**账号**走：
//   /api/skin/online-texture?player=<玩家名>   内核收到过的纹理（按名字存）
//   /api/skin/account-texture?id=<账号>         账号**当前穿着**的纹理
// 于是皮肤库里只有「机器人正好穿着这一款」时才出图，其余全是组件码方块。
// 用户说的「皮肤库里的组件没有正常加载」就是这个断点。
//
// 官方链路（GitHub yeah-god/FunAuth modules/g79client/download_get_info.go）：
//   POST {ApiGatewayUrl}/pe-download-item/get-download-info  {"item_id":"<组件码>"}
//   → {"entity":{"res_url":"https://x19.gph.netease.com/..."}}
// res_url 没有拼接规律，只能接口现取；包里通常是一个 zip，里面的 skin.png 才是纹理。
//
// 这条接口必须登录态（实测无凭据返回 {"code":10,"message":"请先登录"}），所以凭据有两个来源：
//  1. 认证服务：action "skin_info" 会把该账号的 g79 client 快照回传给我们，
//     里面有 UserID / UserToken / ReleaseJSON.ApiGatewayUrl——不用改账号状态，只是读；
//  2. 本机配置文件：插件数据文件/皮肤组件凭据.json，供没有认证账号、或账号掉线时使用。
// 两者都没有时**明确告诉用户缺什么**，不拿别的皮肤冒充这一款。

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	yySkinItemDirName     = "皮肤组件缓存"
	yySkinItemCredName    = "皮肤组件凭据.json"
	yySkinItemLibName     = "皮肤组件库.json"
	yySkinItemMaxSize     = 8 << 20
	yySkinItemNegTTL      = 3 * time.Minute
	yySkinItemMaxCodes    = 200
	yySkinItemDefaultGate = "https://g79apigatewayobt.minecraft.cn"
	yySkinItemDownPath    = "/pe-download-item/get-download-info"
	yySkinItemUserAgent   = "WPFLauncher/0.0.0.0"
)

var (
	yySkinItemRoot     string
	yySkinItemLibPath  string
	yySkinItemCredPath string

	yySkinItemMu  sync.Mutex
	yySkinItemNeg = map[string]time.Time{} // 组件码 → 上次失败时间

	// 认证服务账号列表的短缓存：打开一次皮肤库会并发问 6~10 张卡片，
	// 不缓存就是 6~10 次 /api/accounts。只缓存成功结果，失败仍然立刻可见。
	yySkinItemRowsMu  sync.Mutex
	yySkinItemRows    []yyAuthAccountBrief
	yySkinItemRowsOrg string
	yySkinItemRowsAt  time.Time
)

const yySkinItemRowsTTL = 30 * time.Second

// yySkinItemRecord 是组件码缓存旁的元数据；PNG 本体是 <组件码>.png。
type yySkinItemRecord struct {
	ItemID  string `json:"item_id"`
	Width   uint32 `json:"width"`
	Height  uint32 `json:"height"`
	Source  string `json:"source"`
	ResURL  string `json:"res_url"`
	Updated int64  `json:"updated"`
}

// yySkinItemCred 是一次调用网易组件接口所需的登录态。
// UserToken 是 g79 的长期 token，请求时再由 yySkinItemToken 派生动态 token。
type yySkinItemCred struct {
	UserID     string `json:"user_id"`
	UserToken  string `json:"user_token"`
	APIGateway string `json:"api_gateway"`
	Note       string `json:"note,omitempty"`
}

func (c yySkinItemCred) Ready() bool {
	return strings.TrimSpace(c.UserID) != "" && strings.TrimSpace(c.UserToken) != ""
}

func yySkinItemInit(dataDir string) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return
	}
	base := filepath.Join(dataDir, "插件数据文件")
	yySkinItemRoot = filepath.Join(base, yySkinItemDirName)
	yySkinItemLibPath = filepath.Join(base, yySkinItemLibName)
	yySkinItemCredPath = filepath.Join(base, yySkinItemCredName)
	_ = os.MkdirAll(yySkinItemRoot, 0755)
}

// ---------- 缓存 ----------

func yySkinItemValidCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 32 {
		return false
	}
	return strings.Trim(code, "0123456789") == ""
}

func yySkinItemCached(itemID string) ([]byte, bool) {
	if yySkinItemRoot == "" || !yySkinItemValidCode(itemID) {
		return nil, false
	}
	png, err := os.ReadFile(filepath.Join(yySkinItemRoot, itemID+".png"))
	if err != nil || len(png) < 8 {
		return nil, false
	}
	return png, true
}

func yySkinItemStore(itemID string, png []byte, w, h uint32, source, resURL string) {
	if yySkinItemRoot == "" || len(png) == 0 || !yySkinItemValidCode(itemID) {
		return
	}
	path := filepath.Join(yySkinItemRoot, itemID+".png")
	if err := os.WriteFile(path, png, 0644); err != nil {
		return
	}
	rec, _ := json.Marshal(yySkinItemRecord{
		ItemID: itemID, Width: w, Height: h, Source: source, ResURL: resURL, Updated: time.Now().Unix(),
	})
	_ = os.WriteFile(filepath.Join(yySkinItemRoot, itemID+".json"), rec, 0644)
}

func yySkinItemBlocked(itemID string) bool {
	yySkinItemMu.Lock()
	defer yySkinItemMu.Unlock()
	at, seen := yySkinItemNeg[itemID]
	if !seen {
		return false
	}
	if time.Since(at) > yySkinItemNegTTL {
		delete(yySkinItemNeg, itemID)
		return false
	}
	return true
}

func yySkinItemBlock(itemID string) {
	yySkinItemMu.Lock()
	defer yySkinItemMu.Unlock()
	if len(yySkinItemNeg) > 512 {
		yySkinItemNeg = map[string]time.Time{}
	}
	yySkinItemNeg[itemID] = time.Now()
}

// ---------- 凭据 ----------

// yySkinItemCredFromFile 读本机配置的凭据；文件不存在就是「没配置」，不是错误。
func yySkinItemCredFromFile() (yySkinItemCred, bool) {
	if yySkinItemCredPath == "" {
		return yySkinItemCred{}, false
	}
	raw, err := os.ReadFile(yySkinItemCredPath)
	if err != nil {
		return yySkinItemCred{}, false
	}
	var cred yySkinItemCred
	if json.Unmarshal(raw, &cred) != nil || !cred.Ready() {
		return yySkinItemCred{}, false
	}
	return cred, true
}

// yySkinItemCredFromAuth 问认证服务要这个账号的 g79 client 快照。
// skin_info 只读：它内部是 GetUserSettingList，不会改动账号的皮肤选择。
// 会话过期时先让认证服务 refresh 再问一次——不刷新就会永远停在「请先登录(10)」。
func yySkinItemCredFromAuth(authID string) (yySkinItemCred, string) {
	authID = strings.TrimSpace(authID)
	if authID == "" {
		return yySkinItemCred{}, "缺少账号"
	}
	origin := yyGameAuthOrigin()
	if origin == "" {
		return yySkinItemCred{}, "认证地址不是 https，拿不到账号登录态"
	}
	token := yyAccountSkinDeviceToken()
	if token == "" {
		return yySkinItemCred{}, "未登录游园平台，拿不到设备令牌"
	}
	ctx, cancel := context.WithTimeout(context.Background(), yyAccountSkinTimeout)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"action": "skin_info", "id": authID})

	cred, expired, reason := yySkinItemCredAttempt(ctx, origin, token, body)
	if reason == "" || !expired {
		return cred, reason
	}
	if err := yyAccountSkinRefresh(ctx, origin, token, authID); err != nil {
		return yySkinItemCred{}, reason + "（刷新会话也没成功）"
	}
	cred, _, reason = yySkinItemCredAttempt(ctx, origin, token, body)
	return cred, reason
}

// yySkinItemCredAttempt 发一次 skin_info 并解析出登录态；
// 第二个返回值表示这次失败是不是「会话过期」，调用方据此决定要不要刷新。
func yySkinItemCredAttempt(ctx context.Context, origin, token string, body []byte) (yySkinItemCred, bool, string) {
	data, err := yyAccountSkinCall(ctx, origin, token, body)
	if err != nil {
		return yySkinItemCred{}, false, err.Error()
	}
	if data["ok"] != true {
		reason := yyAccountSkinCallError(data)
		if reason == "" {
			reason = "认证服务没有返回账号登录态"
		}
		return yySkinItemCred{}, yyAccountSkinLoginExpired(data), reason
	}
	raw, err := json.Marshal(data["client"])
	if err != nil {
		return yySkinItemCred{}, false, "认证服务返回的账号登录态无法解析"
	}
	var snapshot struct {
		UserID      string `json:"UserID"`
		UserToken   string `json:"UserToken"`
		ReleaseJSON struct {
			APIGateway string `json:"ApiGatewayUrl"`
		} `json:"ReleaseJSON"`
	}
	if json.Unmarshal(raw, &snapshot) != nil {
		return yySkinItemCred{}, false, "认证服务返回的账号登录态无法解析"
	}
	cred := yySkinItemCred{
		UserID:     strings.TrimSpace(snapshot.UserID),
		UserToken:  strings.TrimSpace(snapshot.UserToken),
		APIGateway: strings.TrimRight(strings.TrimSpace(snapshot.ReleaseJSON.APIGateway), "/"),
	}
	if !cred.Ready() {
		return yySkinItemCred{}, false, "认证服务返回的账号登录态里没有 user-id / user-token"
	}
	return cred, false, ""
}

// yySkinItemAuthRows 取认证服务的账号列表，30 秒内复用上一次的结果。
func yySkinItemAuthRows(origin string) ([]yyAuthAccountBrief, string) {
	yySkinItemRowsMu.Lock()
	cached, cachedOrg, cachedAt := yySkinItemRows, yySkinItemRowsOrg, yySkinItemRowsAt
	yySkinItemRowsMu.Unlock()
	if len(cached) > 0 && cachedOrg == origin && time.Since(cachedAt) < yySkinItemRowsTTL {
		return cached, ""
	}
	rows, reason := yyAccountSkinAuthAccountsBackground(origin)
	if len(rows) > 0 {
		yySkinItemRowsMu.Lock()
		yySkinItemRows, yySkinItemRowsOrg, yySkinItemRowsAt = rows, origin, time.Now()
		yySkinItemRowsMu.Unlock()
	}
	return rows, reason
}

// yySkinItemAuthAccount 把前端给的 id/cred 对到认证服务上的那条游戏凭据。
// 第二个返回值表示「这次请求给没给账号」——没给不是错误，可以走配置凭据。
func yySkinItemAuthAccount(r *http.Request) (yyAuthAccountBrief, bool, string) {
	query := r.URL.Query()
	if cred := strings.TrimSpace(query.Get("cred")); cred != "" {
		return yyAuthAccountBrief{ID: cred}, true, ""
	}
	id := strings.TrimSpace(query.Get("id"))
	if id == "" {
		id = strings.TrimSpace(query.Get("account"))
	}
	if id == "" {
		return yyAuthAccountBrief{}, false, ""
	}
	a, ok := yyManagedFind(id)
	if !ok {
		// 仅云端的记录没有本地账号，前端用 "yy_<凭据ID>" 作为 ID。
		if strings.HasPrefix(id, "yy_") && len(id) > 3 {
			return yyAuthAccountBrief{ID: strings.TrimPrefix(id, "yy_")}, true, ""
		}
		return yyAuthAccountBrief{}, true, "找不到这个账号"
	}
	rows, reason := yySkinItemAuthRows(yyAccountSkinOrigin(a))
	if len(rows) == 0 {
		if reason == "" {
			reason = "认证服务没有返回账号列表"
		}
		return yyAuthAccountBrief{}, true, reason
	}
	row, matched := yyAccountSkinMatch(a, rows)
	if !matched {
		return yyAuthAccountBrief{}, true, "认证服务里没有这个账号对应的游戏凭据"
	}
	if strings.TrimSpace(row.Nickname) == "" {
		row.Nickname = a.Name
	}
	return row, true, ""
}

// ---------- 网易组件接口 ----------

// yySkinItemToken 复刻 g79client.CalculateDynamicToken（third_party/funauth/.../crypto.go:170）。
// 必须逐字节一致：算法是 md5 → 左右移 6 位 → 逐字节位反转后 xor → base64 取前 16 位替换 +/ → 末尾补 "1"。
func yySkinItemToken(path, content, token string) string {
	tokenMD5 := fmt.Sprintf("%x", md5.Sum([]byte(token)))
	payload := tokenMD5 + content + "0eGsBkhl" + strings.TrimSuffix(path, "?")
	payloadMD5 := fmt.Sprintf("%x", md5.Sum([]byte(payload)))

	bits := make([]byte, 0, len(payloadMD5)*8)
	for _, b := range []byte(payloadMD5) {
		bits = append(bits, fmt.Sprintf("%08b", b)...)
	}
	bits = append(bits[6:], bits[:6]...)

	transformed := []byte(payloadMD5)
	for i := 0; i < len(transformed); i++ {
		section := bits[i*8 : i*8+8]
		var reversed byte
		for j := 0; j < 8; j++ {
			if section[7-j] == '1' {
				reversed |= 1 << j
			}
		}
		transformed[i] = reversed ^ transformed[i]
	}

	b64 := base64.StdEncoding.EncodeToString(transformed)
	if len(b64) < 16 {
		return ""
	}
	short := strings.ReplaceAll(strings.ReplaceAll(b64[:16], "+", "m"), "/", "o")
	return short + "1"
}

// yySkinItemDownloadInfo 用给定登录态换组件资源的下载地址。
func yySkinItemDownloadInfo(ctx context.Context, cred yySkinItemCred, itemID string) (string, string) {
	gateway := strings.TrimRight(strings.TrimSpace(cred.APIGateway), "/")
	if gateway == "" {
		gateway = yySkinItemDefaultGate
	}
	u, err := url.Parse(gateway)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", "组件接口地址必须是 https"
	}
	payload, _ := json.Marshal(map[string]any{"item_id": itemID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gateway+yySkinItemDownPath, bytes.NewReader(payload))
	if err != nil {
		return "", err.Error()
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", yySkinItemUserAgent)
	req.Header.Set("user-id", strings.TrimSpace(cred.UserID))
	req.Header.Set("user-token", yySkinItemToken(yySkinItemDownPath, string(payload), strings.TrimSpace(cred.UserToken)))

	resp, err := yyAccountSkinClient.Do(req)
	if err != nil {
		return "", "连不上网易组件接口：" + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Sprintf("网易组件接口返回 HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "读取网易组件接口响应失败"
	}
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Entity  *struct {
			ResURL string `json:"res_url"`
		} `json:"entity"`
	}
	if json.Unmarshal(raw, &out) != nil {
		return "", "网易组件接口响应无法解析"
	}
	if out.Code != 0 {
		// code=10 就是「请先登录」：账号会话过期。原样带回，方便用户重新登录。
		return "", fmt.Sprintf("网易组件接口：%s(%d)", strings.TrimSpace(out.Message), out.Code)
	}
	if out.Entity == nil || strings.TrimSpace(out.Entity.ResURL) == "" {
		return "", "网易没有返回这个组件码的下载地址"
	}
	return strings.TrimSpace(out.Entity.ResURL), ""
}

// yySkinItemTexture 从组件资源里取出皮肤纹理，返回 (PNG 字节, 宽, 高, 是否成功)。
// 资源可能是裸 PNG，也可能是一个 zip 包（里面 skin.png 才是纹理）。
func yySkinItemTexture(raw []byte) ([]byte, uint32, uint32, bool) {
	texture := raw
	if len(raw) > 4 && string(raw[:2]) == "PK" {
		z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return nil, 0, 0, false
		}
		var best *zip.File
		for _, f := range z.File {
			name := strings.ToLower(f.Name)
			if !strings.HasSuffix(name, ".png") {
				continue
			}
			if strings.Contains(name, "icon") || strings.Contains(name, "pack.png") {
				continue
			}
			// 同名的 skin.png 最可信，其余 PNG 只作为兜底。
			if best == nil || strings.HasSuffix(name, "skin.png") {
				best = f
			}
		}
		if best == nil || best.UncompressedSize64 > yySkinItemMaxSize {
			return nil, 0, 0, false
		}
		f, err := best.Open()
		if err != nil {
			return nil, 0, 0, false
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, yySkinItemMaxSize+1))
		if err != nil || len(data) == 0 || len(data) > yySkinItemMaxSize {
			return nil, 0, 0, false
		}
		texture = data
	}
	// yySkinTexturePNG 是「不造像素」的最后一道闸：尺寸不符合皮肤规则就整体拒绝。
	_, w, h, ok := yySkinTexturePNG(texture)
	if !ok {
		return nil, 0, 0, false
	}
	return texture, w, h, true
}

// ---------- 主链路 ----------

// yySkinItemLookup 组件码 → PNG 字节。第二个返回值是来源，用于诊断。
//
// 顺序：本地缓存 → 账号当前穿着的就是它（复用账号纹理，不打网易）→ 配置凭据 → 账号凭据。
func yySkinItemLookup(r *http.Request, itemID string) ([]byte, string, string) {
	if cached, ok := yySkinItemCached(itemID); ok {
		return cached, "cache", ""
	}
	if yySkinItemBlocked(itemID) {
		return nil, "", "刚刚取过一次没成功，请稍后再试"
	}

	row, hasAccount, accountReason := yySkinItemAuthAccount(r)
	if hasAccount && accountReason == "" && strings.TrimSpace(row.SkinItemID) == itemID {
		// 机器人穿的就是这一款：账号纹理链路已经有图，直接落进组件码缓存。
		if data, w, h, _ := yySkinAccountTexture(r); len(data) > 0 {
			var buf bytes.Buffer
			if png.Encode(&buf, yySkinAccountImage(data, w, h)) == nil {
				yySkinItemStore(itemID, buf.Bytes(), w, h, "worn", "")
				return buf.Bytes(), "worn", ""
			}
		}
	}

	cred, ok := yySkinItemCredFromFile()
	source := "config"
	if !ok {
		if !hasAccount {
			return nil, "", "这个组件码还没有图片：请先在上面选一个账号，或在 插件数据文件/皮肤组件凭据.json 里配置登录态"
		}
		if accountReason != "" {
			return nil, "", "拿不到账号登录态：" + accountReason
		}
		var reason string
		cred, reason = yySkinItemCredFromAuth(row.ID)
		if reason != "" {
			return nil, "", "拿不到账号登录态：" + reason
		}
		source = "account"
	}

	ctx, cancel := context.WithTimeout(context.Background(), yyAccountSkinTimeout)
	defer cancel()
	resURL, reason := yySkinItemDownloadInfo(ctx, cred, itemID)
	if reason != "" {
		yySkinItemBlock(itemID)
		return nil, "", reason
	}
	if !yyAccountSkinOfficialURL(resURL) {
		return nil, "", "组件下载地址不在官方域名白名单内，已拒绝"
	}
	raw, ok := yyAccountSkinDownload(ctx, resURL)
	if !ok {
		yySkinItemBlock(itemID)
		return nil, "", "下载组件资源失败"
	}
	pix, w, h, ok := yySkinItemTexture(raw)
	if !ok {
		yySkinItemBlock(itemID)
		return nil, "", "组件资源里没有可用的皮肤 PNG"
	}
	yySkinItemStore(itemID, pix, w, h, source, resURL)
	return pix, source, ""
}

// ---------- 共享组件码列表 ----------

// yySkinItemEntry 是群友分享的一条组件码。内置的几款仍由前端维护，
// 这里只存「贴进来」的那些，避免两边各写一份会漂移。
type yySkinItemEntry struct {
	Code string `json:"code"`
	Name string `json:"name"`
	By   string `json:"by,omitempty"`
	At   int64  `json:"at"`
}

func yySkinItemLibraryRead() []yySkinItemEntry {
	if yySkinItemLibPath == "" {
		return nil
	}
	raw, err := os.ReadFile(yySkinItemLibPath)
	if err != nil {
		return nil
	}
	var list []yySkinItemEntry
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	out := make([]yySkinItemEntry, 0, len(list))
	for _, item := range list {
		if yySkinItemValidCode(item.Code) {
			out = append(out, item)
		}
	}
	return out
}

func yySkinItemLibraryWrite(list []yySkinItemEntry) error {
	if yySkinItemLibPath == "" {
		return fmt.Errorf("数据目录未初始化")
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(yySkinItemLibPath, raw, 0644)
}

// yySkinItemCaptureCodes 把 SyncSkin 抓到的组件码并入共享组件码库（去重、限量）。
//
// 2026-10-26 第 31 条：SyncSkin.skin_item_ids 是网易协议里唯一显式携带组件码的字段，
// 过去只打日志丢弃。抓到的码并入库后，皮肤库会直接出现这一款，
// 点开走 /api/skin/item-texture 的 A 链（组件码 → 官方 get-download-info → PNG），
// 不要求机器人正好穿着同款。
func yySkinItemCaptureCodes(codes []string) {
	if yySkinItemLibPath == "" {
		return
	}
	list := yySkinItemLibraryRead()
	seen := make(map[string]bool, len(list))
	for _, it := range list {
		seen[it.Code] = true
	}
	now := time.Now().Unix()
	added := 0
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if !yySkinItemValidCode(code) || seen[code] {
			continue
		}
		list = append(list, yySkinItemEntry{Code: code, Name: "抓包 " + code, By: "bot", At: now})
		seen[code] = true
		added++
	}
	if added == 0 {
		return
	}
	if len(list) > 200 {
		list = list[len(list)-200:]
	}
	if err := yySkinItemLibraryWrite(list); err == nil {
		debugLog("[皮肤] SyncSkin 抓到 %d 个组件码，已并入组件码库", added)
	}
}

func handleSkinItemLibrary(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		writeJSON(w, map[string]any{"ok": true, "items": yySkinItemLibraryRead()})
	case http.MethodPost:
		var body struct {
			Code string `json:"code"`
			Name string `json:"name"`
			By   string `json:"by"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求体不是 JSON"})
			return
		}
		code := strings.TrimSpace(body.Code)
		if !yySkinItemValidCode(code) {
			writeJSON(w, map[string]any{"ok": false, "error": "组件码是纯数字，请检查"})
			return
		}
		name := strings.TrimSpace(body.Name)
		if len([]rune(name)) > 24 {
			name = string([]rune(name)[:24])
		}
		if name == "" {
			name = "组件码 " + code
		}
		list := yySkinItemLibraryRead()
		merged := make([]yySkinItemEntry, 0, len(list)+1)
		merged = append(merged, yySkinItemEntry{Code: code, Name: name, By: strings.TrimSpace(body.By), At: time.Now().Unix()})
		for _, item := range list {
			if item.Code == code {
				continue
			}
			merged = append(merged, item)
		}
		if len(merged) > yySkinItemMaxCodes {
			merged = merged[:yySkinItemMaxCodes]
		}
		if err := yySkinItemLibraryWrite(merged); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "写入组件码列表失败"})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "items": merged})
	default:
		writeJSON(w, map[string]any{"ok": false, "error": "仅支持 GET / POST"})
	}
}

// handleSkinItemTexture 返回组件码对应的整张皮肤纹理 PNG。
func handleSkinItemTexture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, map[string]any{"ok": false, "error": "仅支持 GET"})
		return
	}
	itemID := strings.TrimSpace(r.URL.Query().Get("item_id"))
	if !yySkinItemValidCode(itemID) {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少数字组件码（item_id）"})
		return
	}
	// 组件码对应的纹理是固定的，给长缓存；本地文件变了会换 ETag。
	data, source, reason := yySkinItemLookup(r, itemID)
	if len(data) == 0 {
		if reason == "" {
			reason = "这个组件码暂时取不到皮肤图"
		}
		writeJSON(w, map[string]any{"ok": false, "item_id": itemID, "error": reason})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Skin-Source", source)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func yySkinItemRoutes(mux *http.ServeMux, dataDir string) {
	yySkinItemInit(dataDir)
	mux.HandleFunc("/api/skin/item-texture", handleSkinItemTexture)
	mux.HandleFunc("/api/skin/item-library", handleSkinItemLibrary)
}
