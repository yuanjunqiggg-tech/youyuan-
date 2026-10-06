package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const yyPlatformOrigin = "https://toolbox.youyuanqi.dpdns.org"

type yyPlatformIdentity struct {
	User    map[string]any `json:"user"`
	Session string         `json:"session_token"`
	Device  string         `json:"device_token"`
	Control string         `json:"control_token"`
}

var yyIdentity struct {
	sync.Mutex
	Data yyPlatformIdentity
	Path string
}

func yyIdentityCall(action string, body any, token string) (map[string]any, error) {
	data, _ := json.Marshal(body)
	request, e := http.NewRequest("POST", yyPlatformOrigin+"/api/identity/"+action, bytes.NewReader(data))
	if e != nil {
		return nil, e
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, e := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	var result map[string]any
	if e = json.NewDecoder(response.Body).Decode(&result); e != nil {
		return nil, e
	}
	if result["ok"] != true {
		return nil, fmt.Errorf("%v", result["error"])
	}
	return result, nil
}
func yyIdentitySaveLocked() error {
	data, _ := json.MarshalIndent(yyIdentity.Data, "", "  ")
	return os.WriteFile(yyIdentity.Path, data, 0600)
}
func yyIdentityRoutes(mux *http.ServeMux, dataDir string) {
	yyIdentity.Path = filepath.Join(dataDir, "platform-identity.json")
	if raw, e := os.ReadFile(yyIdentity.Path); e == nil {
		_ = json.Unmarshal(raw, &yyIdentity.Data)
	}
	mux.HandleFunc("/api/youyuan/identity", func(w http.ResponseWriter, r *http.Request) {
		if yyAccounts.Worker {
			writeJSON(w, map[string]any{"ok": false, "error": "平台账号在主控制端登录"})
			return
		}
		yyIdentity.Lock()
		defer yyIdentity.Unlock()
		if r.Method == "GET" {
			authenticated := false
			var detail string
			if yyIdentity.Data.Session != "" {
				_, e := yyIdentityCall("me", nil, yyIdentity.Data.Session)
				authenticated = e == nil
				if e != nil {
					detail = e.Error()
				}
			}
			writeJSON(w, map[string]any{"ok": true, "authenticated": authenticated, "user": yyIdentity.Data.User, "control_token": yyIdentity.Data.Control, "domain": yyPlatformOrigin, "detail": detail})
			return
		}
		var input struct {
			Action          string `json:"action"`
			Username        string `json:"username"`
			QQ              string `json:"qq"`
			Password        string `json:"password"`
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "输入无效"})
			return
		}
		if input.Action == "logout" {
			_, _ = yyIdentityCall("logout", nil, yyIdentity.Data.Session)
			yyIdentity.Data.Session = ""
			_ = yyIdentitySaveLocked()
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if input.Action == "password" {
			result, e := yyIdentityCall("password", map[string]any{"current_password": input.CurrentPassword, "new_password": input.NewPassword}, yyIdentity.Data.Session)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, result)
			return
		}
		if input.Action != "login" && input.Action != "register" {
			writeJSON(w, map[string]any{"ok": false, "error": "未知账号操作"})
			return
		}
		result, e := yyIdentityCall(input.Action, map[string]any{"username": input.Username, "qq": input.QQ, "password": input.Password}, "")
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		user, _ := result["user"].(map[string]any)
		if yyIdentity.Data.User != nil && yyIdentity.Data.User["id"] != user["id"] {
			writeJSON(w, map[string]any{"ok": false, "error": "本机数据已属于另一平台账号，请使用原账号登录"})
			return
		}
		session, _ := result["session_token"].(string)
		device, _ := result["device_token"].(string)
		control, _ := result["control_token"].(string)
		yyRemoteMu.RLock()
		existingDevice := yyRemote.DeviceToken
		yyRemoteMu.RUnlock()
		if existingDevice != "" && yyIdentity.Data.User == nil {
			if claim, err := yyIdentityCall("claim", map[string]any{"device_key": existingDevice}, session); err == nil {
				user, _ = claim["user"].(map[string]any)
				device = existingDevice
			}
		}
		yyIdentity.Data = yyPlatformIdentity{User: user, Session: session, Device: device, Control: control}
		if e = yyIdentitySaveLocked(); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		yyRemoteMu.Lock()
		yyRemote.URL = "wss://toolbox.youyuanqi.dpdns.org/device/connect"
		yyRemote.DeviceToken = device
		yyRemote.Enabled = true
		data, _ := json.MarshalIndent(yyRemote, "", "  ")
		e = os.WriteFile(yyRemotePath, data, 0600)
		if yyRemoteCancel != nil {
			yyRemoteCancel()
		}
		yyRemoteMu.Unlock()
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "authenticated": true, "user": user, "control_token": control})
	})
}
