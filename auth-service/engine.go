package main

import (
	"context"
	"encoding/json"
	"fmt"
	auth "github.com/Yeah114/FunAuth/auth"
	g79 "github.com/Yeah114/g79client"
	"github.com/Yeah114/g79client/account/mpay"
	"net/http"
	"strings"
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
	ServerPassword string          `json:"server_password"`
	PublicKey      string          `json:"public_key"`
	Data           string          `json:"data"`
	UID            string          `json:"uid"`
	Engine         string          `json:"engine_version"`
	Patch          string          `json:"patch_version"`
	IsPC           bool            `json:"is_pc"`
	Keyword        string          `json:"keyword"`
	Offset         int             `json:"offset"`
}
type contextTransport struct{ ctx context.Context }

func (t contextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(req.Clone(t.ctx))
}
func kernel(raw string) (out map[string]any) {
	defer func() {
		if recover() != nil {
			out = map[string]any{"ok": false, "error": "认证计算失败"}
		}
	}()
	var p kernelInput
	if e := json.Unmarshal([]byte(raw), &p); e != nil {
		return map[string]any{"ok": false, "error": "请求格式错误"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		return result
	}
	switch p.Action {
	case "capabilities":
		return map[string]any{"ok": true, "provider": "direct-netease", "actions": []string{"device_create", "email_login", "guest_login", "authenticate", "refresh", "nickname", "game_login", "transfer_start_type", "transfer_check_num"}}
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
		for _, v := range list.Entities {
			rows = append(rows, map[string]any{"id": v.Sid, "name": v.Name, "type": "domain", "status": v.Status})
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
		return map[string]any{"ok": true, "client": json.RawMessage(snapshot), "result": result}
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
