package main

// 按账号取皮肤：/api/skin/account-texture、/api/skin/account-head
//
// 为什么需要它：账号页取立体皮肤走的是 /api/skin/online-texture?player=<玩家名>，
// 只认「游戏内玩家名」。而账号预热落盘用的是 yyAccountSkinNames() 里的
// 备注名 / 账号 ID / 认证昵称——两边名字对不上时，磁盘里明明有纹理，
// 前端却还是 404，用户看到的就是「打开了账号信息，能看见头像，看不见皮肤」。
//
// 这里按账号（或云端凭据）直接查同一份磁盘缓存，查不到就当场问认证服务的
// 官方皮肤接口（skin_preview → 官方 res_url），取到后落盘再返回。
// 因此第一次点开可能会慢 1~3 秒，之后每次都是本地命中。

import (
	"image"
	"image/png"
	"net/http"
	"strings"
)

// yySkinAccountResolve 把请求参数解析成「一个账号 + 一组可以用来查缓存的名字」。
//
//	id   = 本地托管账号 ID（yy_xxx / pool_xxx / 自定义 ID）
//	cred = 云端游戏凭据 ID（没有本地账号时的仅云端记录）
//	name = 玩家名兜底（前端已经从别处知道游戏名时可以直接给）
func yySkinAccountResolve(r *http.Request) (yyManagedAccount, []string, bool) {
	query := r.URL.Query()
	name := strings.TrimSpace(query.Get("name"))
	if id := strings.TrimSpace(query.Get("id")); id != "" {
		if a, ok := yyManagedFind(id); ok {
			names := yyAccountSkinNames(a)
			if name != "" && !yyAccountSkinHasName(names, name) {
				names = append(names, name)
			}
			return a, names, len(names) > 0
		}
	}
	cred := strings.TrimSpace(query.Get("cred"))
	if cred == "" {
		return yyManagedAccount{}, nil, false
	}
	// 仅云端的账号没有本地记录：用 "yy_<凭据ID>" 这个约定去匹配认证服务，
	// 与前端保存凭据时生成的本地 ID 一致。
	a := yyManagedAccount{ID: "yy_" + cred, AuthURL: yyGameAuthOrigin()}
	names := yyAccountSkinNames(a)
	if name != "" && !yyAccountSkinHasName(names, name) {
		names = append(names, name)
	}
	return a, names, len(names) > 0
}

// yySkinAccountLookup 先查磁盘缓存（预热过就立刻命中）。
func yySkinAccountLookup(names []string) ([]byte, uint32, uint32, bool) {
	for _, name := range names {
		if data, w, h, ok := yySkinCacheGet(name); ok {
			return data, w, h, true
		}
	}
	return nil, 0, 0, false
}

// yySkinAccountTexture 返回账号当前皮肤的 RGBA 与尺寸：缓存优先，其次官方接口。
func yySkinAccountTexture(r *http.Request) ([]byte, uint32, uint32, string) {
	a, names, ok := yySkinAccountResolve(r)
	if !ok {
		return nil, 0, 0, "缺少账号标识或可用名字"
	}
	if data, w, h, ok := yySkinAccountLookup(names); ok {
		return data, w, h, ""
	}
	result := yyAccountSkinFetch(a, nil)
	if !result.OK() {
		return nil, 0, 0, result.Reason
	}
	// 取到就落盘：下一次（包括 online-texture）直接命中，不用再问认证服务。
	yyAccountSkinStore(result)
	yyAccountSkinPrimeMark(a.ID, true, "")
	return result.Data, result.Width, result.Height, ""
}

func yySkinAccountImage(data []byte, w, h uint32) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	copy(img.Pix, data)
	return img
}

// handleSkinAccountTexture 返回整张皮肤纹理 PNG。
func handleSkinAccountTexture(w http.ResponseWriter, r *http.Request) {
	data, width, height, reason := yySkinAccountTexture(r)
	if len(data) == 0 {
		if reason == "" {
			reason = "这个账号还没有可用的皮肤纹理"
		}
		writeJSON(w, map[string]any{"ok": false, "error": reason})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	// 皮肤会换，缓存短一点；同一账号连续请求由浏览器缓存兜住。
	w.Header().Set("Cache-Control", "private, max-age=120")
	_ = png.Encode(w, yySkinAccountImage(data, width, height))
}

// handleSkinAccountHead 返回从同一张纹理裁出的头像（16x16 放大前）。
// 头像走这条路的好处：不需要玩家名，也不依赖内核是否在线。
func handleSkinAccountHead(w http.ResponseWriter, r *http.Request) {
	data, width, height, reason := yySkinAccountTexture(r)
	if len(data) == 0 {
		if reason == "" {
			reason = "这个账号还没有可用的皮肤纹理"
		}
		writeJSON(w, map[string]any{"ok": false, "error": reason})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=120")
	_ = png.Encode(w, extractSkinHead(yySkinAccountImage(data, width, height)))
}
