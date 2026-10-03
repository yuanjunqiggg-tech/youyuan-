package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var yyQQAccess = struct {
	sync.Mutex
	AppID, Secret, Token string
	Expires              time.Time
}{}

type yyQQReplyWindow struct {
	Group string
	Until time.Time
	Next  int
}

var yyQQReplies = struct {
	sync.Mutex
	Items          map[string]*yyQQReplyWindow
	Last, Callback time.Time
}{Items: map[string]*yyQQReplyWindow{}}

func yyQQAccessToken(cf yyQQConfig) (string, error) {
	yyQQAccess.Lock()
	defer yyQQAccess.Unlock()
	if yyQQAccess.AppID == cf.Official.AppID && yyQQAccess.Secret == cf.Official.Secret && time.Until(yyQQAccess.Expires) > 60*time.Second {
		return yyQQAccess.Token, nil
	}
	if cf.Official.AppID == "" || cf.Official.Secret == "" {
		return "", fmt.Errorf("QQ AppID/AppSecret 尚未配置")
	}
	body, _ := json.Marshal(map[string]any{"appId": cf.Official.AppID, "clientSecret": cf.Official.Secret})
	response, e := yyQQHTTP.Post("https://bots.qq.com/app/getAppAccessToken", "application/json", bytes.NewReader(body))
	if e != nil {
		return "", e
	}
	defer response.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(response.Body, 65536))
	if e != nil {
		return "", e
	}
	var token struct {
		AccessToken string      `json:"access_token"`
		Expires     json.Number `json:"expires_in"`
		Code        int         `json:"code"`
		Message     string      `json:"message"`
	}
	if e = json.Unmarshal(raw, &token); e != nil {
		return "", e
	}
	if response.StatusCode != 200 || token.AccessToken == "" {
		return "", fmt.Errorf("QQ 认证失败 HTTP %d code=%d：%s", response.StatusCode, token.Code, token.Message)
	}
	expires, err := token.Expires.Int64()
	if err != nil || expires < 60 {
		expires = 60
	}
	yyQQAccess.AppID = cf.Official.AppID
	yyQQAccess.Secret = cf.Official.Secret
	yyQQAccess.Token = token.AccessToken
	yyQQAccess.Expires = time.Now().Add(time.Duration(expires) * time.Second)
	return token.AccessToken, nil
}
func yyQQRememberReply(group, id string) {
	if group == "" || id == "" {
		return
	}
	yyQQReplies.Lock()
	defer yyQQReplies.Unlock()
	now := time.Now()
	for k, v := range yyQQReplies.Items {
		if now.After(v.Until) {
			delete(yyQQReplies.Items, k)
		}
	}
	if _, ok := yyQQReplies.Items[id]; ok {
		return
	}
	if len(yyQQReplies.Items) >= 256 {
		return
	}
	yyQQReplies.Items[id] = &yyQQReplyWindow{Group: group, Until: now.Add(290 * time.Second), Next: 1}
	yyQQReplies.Last = now
}
func yyQQReserveReply(group, id string) (string, int, error) {
	yyQQReplies.Lock()
	defer yyQQReplies.Unlock()
	now := time.Now()
	if id == "" {
		var newest time.Time
		for k, v := range yyQQReplies.Items {
			if v.Group == group && v.Next <= 5 && v.Until.After(now) && v.Until.After(newest) {
				id, newest = k, v.Until
			}
		}
	}
	v := yyQQReplies.Items[id]
	if v == nil || v.Group != group || !v.Until.After(now) {
		return "", 0, fmt.Errorf("官方群机器人不能主动推送：请在群中 @机器人，获得五分钟回复窗口")
	}
	if v.Next > 5 {
		return "", 0, fmt.Errorf("该群消息五次回复额度已用完，请再次 @机器人")
	}
	seq := v.Next
	v.Next++
	return id, seq, nil // Never reuse a sequence after a timeout: the server may have accepted it.
}
func yyQQSendOfficialWindow(cf yyQQConfig, text, id string) error {
	group := yyQQGroupString(cf.GroupID)
	if group == "" {
		return fmt.Errorf("尚未收到本群真实回调，不能用数字群号代替 group_openid")
	}
	id, seq, e := yyQQReserveReply(group, id)
	if e != nil {
		return e
	}
	token, e := yyQQAccessToken(cf)
	if e != nil {
		return e
	}
	return yyQQOfficialPost(cf, token, "/messages", map[string]any{"content": text, "msg_type": 0, "msg_id": id, "msg_seq": seq}, nil)
}
func yyQQOfficialPost(cf yyQQConfig, token, suffix string, payload any, result any) error {
	group := yyQQGroupString(cf.GroupID)
	body, _ := json.Marshal(payload)
	req, e := http.NewRequest("POST", "https://api.sgroup.qq.com/v2/groups/"+url.PathEscape(group)+suffix, bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Union-Appid", cf.Official.AppID)
	response, e := yyQQHTTP.Do(req)
	if e != nil {
		return e
	}
	defer response.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(response.Body, 65536))
	if e != nil {
		return e
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("QQ 发送失败 HTTP %d：%s", response.StatusCode, raw)
	}
	if result != nil {
		return json.Unmarshal(raw, result)
	}
	return nil
}
func yyQQSendImage(cf yyQQConfig, image []byte, id string) error {
	if len(image) > 2*1024*1024 {
		return fmt.Errorf("资料卡图片过大")
	}
	if cf.Adapter == "onebot" {
		return yyQQSendOneBot(cf, []any{map[string]any{"type": "image", "data": map[string]any{"file": "base64://" + base64.StdEncoding.EncodeToString(image)}}}, false)
	}
	token, e := yyQQAccessToken(cf)
	if e != nil {
		return e
	}
	var upload struct {
		FileInfo string `json:"file_info"`
	}
	if e = yyQQOfficialPost(cf, token, "/files", map[string]any{"file_type": 1, "file_data": base64.StdEncoding.EncodeToString(image), "srv_send_msg": false}, &upload); e != nil {
		return e
	}
	if upload.FileInfo == "" {
		return fmt.Errorf("QQ图片上传没有返回file_info")
	}
	id, seq, e := yyQQReserveReply(yyQQGroupString(cf.GroupID), id)
	if e != nil {
		return e
	}
	return yyQQOfficialPost(cf, token, "/messages", map[string]any{"content": "玩家资料", "msg_type": 7, "media": map[string]any{"file_info": upload.FileInfo}, "msg_id": id, "msg_seq": seq}, nil)
}
func yyQQOfficialRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/qq/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		cf := yyQQSnapshot()
		_, e := yyQQAccessToken(cf)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "app_id": cf.Official.AppID, "credentials_valid": true, "scope": "仅验证官方凭据，不代表收到群回调/群消息往返成功"})
	})
	mux.HandleFunc("/api/qq/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		cf := yyQQSnapshot()
		yyQQReplies.Lock()
		slots := 0
		for _, v := range yyQQReplies.Items {
			if v.Group == yyQQGroupString(cf.GroupID) && v.Until.After(time.Now()) && v.Next <= 5 {
				slots += 6 - v.Next
			}
		}
		last, callback := yyQQReplies.Last, yyQQReplies.Callback
		yyQQReplies.Unlock()
		writeJSON(w, map[string]any{"ok": true, "enabled": cf.Enabled, "transport": cf.Official.Transport, "secret_saved": cf.Official.Secret != "", "group_number": cf.GroupNumber, "group_bound": yyQQGroupString(cf.GroupID) != "", "last_verified_callback": callback, "last_group_message": last, "reply_slots_remaining": slots, "official_active_push": false, "note": "真实群事件会自动绑定 group_openid；官方指令权限使用成员 member_openid，支持指定成员与指令前缀白名单。游戏转群需在五分钟、每条消息最多五次的官方回复窗口内。"})
	})
}
