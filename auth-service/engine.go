package main

import (
	"context"
	"encoding/json"
	"fmt"
	auth "github.com/Yeah114/FunAuth/auth"
	g79 "github.com/Yeah114/g79client"
	"github.com/Yeah114/g79client/account/com4399"
	"github.com/Yeah114/g79client/account/mpay"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type kernelInput struct {
	Action         string          `json:"action"`
	Email          string          `json:"email"`
	Password       string          `json:"password"`
	Cookie         string          `json:"cookie"`
	Device         *mpay.Device    `json:"device"`
	Client         json.RawMessage `json:"client"`
	Server         string          `json:"server"`
	ServerID       string          `json:"server_id"`
	Nickname       string          `json:"nickname"`
	ItemID         string          `json:"item_id"`
	ServerPassword string          `json:"server_password"`
	PublicKey      string          `json:"public_key"`
	Data           string          `json:"data"`
	UID            string          `json:"uid"`
	Engine         string          `json:"engine_version"`
	Patch          string          `json:"patch_version"`
	IsPC           bool            `json:"is_pc"`
	Keyword        string          `json:"keyword"`
	RoomID         string          `json:"room_id"`
	Offset         int             `json:"offset"`
	Username       string          `json:"username"`
	Device4399     *com4399.Device `json:"device_4399"`
	Captcha        string          `json:"captcha"`
	CaptchaID      string          `json:"captcha_id"`
}
type contextTransport struct{ ctx context.Context }

func (t contextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Preserve the operation's shorter deadline, while also respecting the
	// enclosing kernel request. Replacing req.Context lost discovery timeouts.
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(t.ctx, cancel)
	if t.ctx.Err() != nil {
		cancel()
	}
	response, err := http.DefaultTransport.RoundTrip(req.Clone(ctx))
	if err != nil {
		stop()
		cancel()
		return nil, err
	}
	response.Body = &kernelResponseBody{ReadCloser: response.Body, release: func() { stop(); cancel() }}
	return response, nil
}

type kernelResponseBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *kernelResponseBody) Close() error { defer b.once.Do(b.release); return b.ReadCloser.Close() }
func kernel(raw string) (out map[string]any) {
	return kernelWithContext(context.Background(), raw)
}

func kernelWithContext(parent context.Context, raw string) (out map[string]any) {
	defer func() {
		if recover() != nil {
			out = map[string]any{"ok": false, "error": "认证计算失败"}
		}
	}()
	var p kernelInput
	if e := json.Unmarshal([]byte(raw), &p); e != nil {
		return map[string]any{"ok": false, "error": "请求格式错误"}
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	fail := func(e error) map[string]any {
		message := e.Error()
		for _, secret := range []string{p.Password, p.Cookie} {
			if secret != "" {
				message = strings.ReplaceAll(message, secret, "[已隐藏]")
			}
		}
		result := map[string]any{"ok": false, "error": message}
		if v, ok := e.(*mpay.NeedVerifyError); ok {
			result["code"] = v.Code
			result["verification_url"] = v.VerifyURL
			result["verification_required"] = v.VerifyURL != ""
		}
		if v, ok := e.(*com4399.NeedCaptchaError); ok {
			result["verification_required"] = true
			result["verification_kind"] = "captcha_4399"
			result["captcha_id"] = v.CaptchaID
			result["captcha_url"] = v.CaptchaURL
		}
		if v, ok := e.(*com4399.LoginUnavailableError); ok {
			result["provider_status"] = v.StatusCode
			result["retryable"] = true
			result["phase"] = "channel_login"
		}
		return result
	}
	switch p.Action {
	case "capabilities":
		return map[string]any{"ok": true, "provider": "direct-netease", "actions": []string{"device_create", "email_login", "guest_login", "device_4399_create", "login_4399", "authenticate", "refresh", "nickname", "game_login", "tan_lobby_login", "room_discover", "transfer_start_type", "transfer_check_num"}, "official_bulk_sms_registration_guaranteed": false}
	case "device_4399_create":
		d, e := com4399.NewClient(&http.Client{Timeout: 12 * time.Second, Transport: contextTransport{ctx}}).RegisterDevice(ctx)
		if e != nil {
			return fail(e)
		}
		return map[string]any{"ok": true, "device_4399": d}
	case "login_4399":
		if p.Device4399 == nil {
			return fail(fmt.Errorf("缺少4399设备凭据"))
		}
		p.Device4399.SetHTTPClient(&http.Client{Timeout: 12 * time.Second, Transport: contextTransport{ctx}})
		u, e := p.Device4399.LoginWithPasswordOptions(ctx, p.Username, p.Password, com4399.LoginOptions{Captcha: p.Captcha, CaptchaID: p.CaptchaID, ManualCaptcha: true})
		if e != nil {
			return fail(e)
		}
		cookie, e := u.CookieString()
		if e != nil {
			return fail(e)
		}
		return map[string]any{"ok": true, "cookie": cookie, "sdk_uid": u.Sauth.SDKUID}
	case "device_create":
		d, e := mpay.NewClient(&http.Client{Timeout: 12 * time.Second, Transport: contextTransport{ctx}}).GenerateDevice(ctx)
		if e != nil {
			return fail(e)
		}
		return map[string]any{"ok": true, "device": d}
	case "email_login", "guest_login":
		if p.Device == nil {
			return fail(fmt.Errorf("缺少设备凭据"))
		}
		p.Device.SetHTTPClient(&http.Client{Timeout: 12 * time.Second, Transport: contextTransport{ctx}})
		var u *mpay.User
		var e error
		if p.Action == "guest_login" {
			u, e = p.Device.Guest(ctx)
		} else {
			u, e = p.Device.LoginEmail(ctx, p.Email, p.Password)
		}
		if e != nil {
			return fail(e)
		}
		cookie, e := u.CookieString()
		if e != nil {
			return fail(e)
		}
		return map[string]any{"ok": true, "cookie": cookie, "sdk_uid": u.Sauth.SDKUID}
	case "transfer_start_type":
		v, e := auth.TransferStartType(p.UID, p.Data)
		if e != nil {
			return fail(e)
		}
		return map[string]any{"ok": true, "data": v}
	case "transfer_check_num":
		v, e := auth.TransferCheckNum(ctx, p.IsPC, p.Data, p.Engine, p.Patch)
		if e != nil {
			return fail(e)
		}
		return map[string]any{"ok": true, "value": v}
	}
	cli, e := g79.NewClientWithHTTPClient(&http.Client{Timeout: 12 * time.Second, Transport: contextTransport{ctx}})
	if e != nil {
		return fail(e)
	}
	if len(p.Client) > 0 {
		if e = json.Unmarshal(p.Client, cli); e != nil {
			return fail(e)
		}
	}
	switch p.Action {
	case "skin", "skin_info", "skin_preview":
		if p.Action == "skin" {
			if len(p.ItemID) < 1 || len(p.ItemID) > 32 || strings.Trim(p.ItemID, "0123456789") != "" {
				return fail(fmt.Errorf("请输入本账号已有皮肤的数字组件码"))
			}
			// Selection only: do not purchase an item on behalf of the account.
			selected, err := cli.SetUserSettingList(p.ItemID)
			if err != nil {
				return fail(err)
			}
			if selected.Code != 0 {
				return fail(fmt.Errorf("皮肤选择失败：%s(%d)", selected.Message, selected.Code))
			}
		}
		skin, err := currentAccountSkin(cli)
		if err != nil && skin.ItemID == "" {
			return fail(err)
		}
		warning := ""
		if err != nil {
			warning = err.Error()
		}
		if p.Action == "skin_preview" {
			preview, e := skinPreview(ctx, skin)
			if e != nil {
				return fail(e)
			}
			preview["skin_info"] = skin
			preview["body_type_known"] = warning == ""
			preview["warning"] = warning
			preview["ok"] = true
			preview["client"] = p.Client
			return preview
		}
		snapshot, _ := json.Marshal(cli)
		return map[string]any{"ok": true, "skin_info": skin, "warning": warning, "body_type_known": warning == "", "client": json.RawMessage(snapshot)}
	case "authenticate":
		if e = cli.G79AuthenticateWithCookie(p.Cookie); e != nil {
			return fail(e)
		}
		detail, e := cli.GetUserDetail()
		if e != nil {
			return fail(e)
		}
		cli.UserDetail = &detail.Entity
	case "refresh":
		if cli.UserToken == "" {
			return fail(fmt.Errorf("缺少游戏登录状态"))
		}
		if _, e = cli.UpdateToken(); e != nil {
			return fail(e)
		}
	case "room_discover":
		return discoverRooms(cli, p.Keyword, p.Offset)
	case "room_list":
		if len([]rune(p.Keyword)) > 64 || p.Offset < 0 || p.Offset > 100000 {
			return fail(fmt.Errorf("房间搜索参数无效"))
		}
		list, err := cli.SearchOnlineLobbyRoomByKeyword(p.Keyword, 20, p.Offset)
		if err != nil {
			return fail(err)
		}
		if list.Code != 0 {
			return fail(fmt.Errorf("读取房间失败：%s(%d)", list.Message, list.Code))
		}
		rows := []map[string]any{}
		for _, v := range list.Entities {
			rows = append(rows, map[string]any{"room_id": v.ID(), "entity_id": v.EntityID.String(), "name": v.RoomName, "owner": v.OwnerName, "players": v.CurNum.Int64(), "max_players": v.MaxCount.Int64(), "password_required": v.Password.Int64() != 0})
		}
		return map[string]any{"ok": true, "rooms": rows, "total": list.Total.Int64(), "offset": p.Offset}
	case "domain_list":
		list, err := cli.GetOtherDomainServers()
		if err != nil {
			return fail(err)
		}
		if list.Code != 0 {
			return fail(fmt.Errorf("读取山头列表失败：%s(%d)", list.Message, list.Code))
		}
		rows := []map[string]any{}
		for i, v := range list.Entities {
			rows = append(rows, map[string]any{"index": i + 1, "id": v.Sid, "name": v.Name, "type": "domain", "status": v.Status})
		}
		return map[string]any{"ok": true, "servers": rows}
	case "domain_prepare":
		before, e := cli.GetOtherDomainServers()
		if e != nil {
			return fail(e)
		}
		joined, e := cli.JoinDomainServerWithInviteCode(strings.TrimPrefix(p.Server, "DomainGame:"))
		if e != nil {
			return fail(e)
		}
		after, e := cli.GetOtherDomainServers()
		if e != nil {
			return fail(e)
		}
		return map[string]any{"ok": true, "before": before, "joined": joined, "after": after}
	case "nickname":
		if len([]rune(strings.TrimSpace(p.Nickname))) < 2 || len([]rune(p.Nickname)) > 20 {
			return fail(fmt.Errorf("游戏昵称长度需为2–20个字符"))
		}
		if e = cli.UpdateNickname(strings.TrimSpace(p.Nickname)); e != nil {
			return fail(e)
		}
		detail, e := cli.GetUserDetail()
		if e != nil {
			return fail(e)
		}
		cli.UserDetail = &detail.Entity
	case "tan_lobby_login":
		if cli.UserDetail == nil || cli.UserDetail.Name == "" { return fail(fmt.Errorf("账号尚未设置游戏昵称，请先设置昵称")) }
		if strings.TrimSpace(p.RoomID) == "" || len(p.RoomID)>128 { return fail(fmt.Errorf("本地房间标识无效")) }
		result, err := auth.TanLobbyLogin(ctx, cli, auth.TanLobbyLoginParams{RoomID:p.RoomID})
		if err != nil { return fail(err) }
		snapshot, err := json.Marshal(cli); if err != nil { return fail(err) }
		skin, skinErr := currentAccountSkin(cli)
		response := map[string]any{"ok":true,"client":json.RawMessage(snapshot),"result":tanLobbyWireResult(result),"skin_info":skin,"engine_version":cli.EngineVersion,"patch_version":cli.G79LatestVersion}
		if skinErr != nil { response["skin_error"] = "未能读取当前账号皮肤" }
		return response
	case "game_login":
		if cli.UserDetail == nil || cli.UserDetail.Name == "" {
			return fail(fmt.Errorf("账号尚未设置游戏昵称，请先设置昵称"))
		}
		var result auth.LoginResult
		if invite, ok := strings.CutPrefix(p.Server, "DomainGame:"); ok {
			result, e = safeDomainLogin(cli, invite, p.ServerID, p.PublicKey)
		} else if sid, ok := strings.CutPrefix(p.Server, "DomainSID:"); ok {
			result, e = safeDomainLogin(cli, "", sid, p.PublicKey)
		} else if strings.HasPrefix(p.Server, "PC") {
			return fail(fmt.Errorf("此独立认证入口暂未开放PC大厅和PC山头协议"))
		} else {
			result, e = auth.Login(ctx, cli, auth.LoginParams{ServerCode: p.Server, ServerPassword: p.ServerPassword, ClientPublicKey: p.PublicKey})
		}
		if e != nil {
			return fail(e)
		}
		snapshot, e := json.Marshal(cli)
		if e != nil {
			return fail(e)
		}
		skin, skinErr := currentAccountSkin(cli)
		response := map[string]any{"ok": true, "client": json.RawMessage(snapshot), "result": result, "skin_info": skin}
		if skinErr != nil {
			response["skin_error"] = skinErr.Error()
		}
		return response
	default:
		return fail(fmt.Errorf("暂未开放的认证操作"))
	}
	snapshot, e := json.Marshal(cli)
	if e != nil {
		return fail(e)
	}
	return map[string]any{"ok": true, "client": json.RawMessage(snapshot), "game_id": cli.UserID, "nickname": func() string {
		if cli.UserDetail != nil {
			return cli.UserDetail.Name
		}
		return ""
	}(), "engine_version": cli.EngineVersion, "patch_version": cli.G79LatestVersion}
}

// Preserve the account's selected skin. Unlike the upstream fallback helper,
// this never changes the selection when it is missing or cannot be read.
func currentAccountSkin(cli *g79.Client) (auth.SkinInfo, error) {
	s, e := cli.GetUserSettingList()
	if e != nil {
		return auth.SkinInfo{}, e
	}
	if s.Code != 0 {
		return auth.SkinInfo{}, fmt.Errorf("读取皮肤设置：%s(%d)", s.Message, s.Code)
	}
	id := s.Entity.SkinData.ItemID
	if id == "" || strings.HasPrefix(id, "-") {
		// 这里原来返回 (空 SkinInfo, nil)。因为 err 是 nil，engine.go:200 的守卫
		// `if err != nil && skin.ItemID == ""` 判不出来，于是带着空的
		// SkinDownloadURL 一路进 skinPreview，最终报「当前皮肤没有受支持的官方
		// 资源地址」——这个文案看不出真正原因（账号压根没选过皮肤）。
		// 改成显式报错，让前端能直接说人话。
		// 注意：game_login / tan_lobby_login 只把 err 写进 skin_error 字段、
		// 不中断登录，所以这里改成报错不影响进服。
		return auth.SkinInfo{}, fmt.Errorf("这个账号还没选过皮肤：请先在游戏里选一款，或在下方填入已拥有的皮肤组件码")
	}
	d, e := cli.GetDownloadInfo(id)
	if e != nil {
		return auth.SkinInfo{}, e
	}
	if d.Code != 0 {
		return auth.SkinInfo{}, fmt.Errorf("读取皮肤资源：%s(%d)", d.Message, d.Code)
	}
	skin := auth.SkinInfo{ItemID: id, SkinDownloadURL: d.Entity.ResURL}
	if s.Entity.SkinData.IsSlim != nil {
		skin.SkinIsSlim = *s.Entity.SkinData.IsSlim
		return skin, nil
	}
	slim, bodyErr := cli.GetSkinBodyType(id)
	skin.SkinIsSlim = slim
	return skin, bodyErr
}
