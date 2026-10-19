package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
var yyIdentityHTTPClient = &http.Client{Timeout: 35 * time.Second}

type yyIdentityHTTPError struct {
	Status  int
	Message string
}

func (e *yyIdentityHTTPError) Error() string { return e.Message }

func yyIdentityCall(action string, body any, token string) (map[string]any, error) {
	data, _ := json.Marshal(body)
	request, e := http.NewRequest("POST", yyPlatformOrigin+"/api/identity/"+action, bytes.NewReader(data))
	if e != nil {
		return nil, e
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "YouYuan-Toolbox/"+yyAppVersion)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, e := yyIdentityHTTPClient.Do(request)
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	var result map[string]any
	if e = json.NewDecoder(response.Body).Decode(&result); e != nil {
		return nil, e
	}
	if result["ok"] != true {
		return nil, &yyIdentityHTTPError{Status: response.StatusCode, Message: fmt.Sprint(result["error"])}
	}
	return result, nil
}

// Clear only a revoked platform binding. User projects and game accounts stay
// on disk; temporary network failure or an expired session never erases them.
func yyIdentityClearLocked() error {
	oldDevice := yyIdentity.Data.Device
	yyIdentity.Data = yyPlatformIdentity{}
	if err := yyIdentitySaveLocked(); err != nil {
		return err
	}
	yyRemoteMu.Lock()
	defer yyRemoteMu.Unlock()
	if oldDevice != "" && yyRemote.DeviceToken == oldDevice {
		yyRemote.DeviceToken = ""
		yyRemote.Enabled = false
		data, _ := json.MarshalIndent(yyRemote, "", "  ")
		if err := os.WriteFile(yyRemotePath, data, 0600); err != nil {
			return err
		}
		if yyRemoteCancel != nil {
			yyRemoteCancel()
		}
	}
	return nil
}
func yyIdentityCheckBindingLocked() error {
	if yyIdentity.Data.User == nil {
		return nil
	}
	if yyIdentity.Data.Control == "" {
		return fmt.Errorf("本机仍绑定原平台账号，请先登录原账号办理注销")
	}
	_, err := yyIdentityCall("binding", nil, yyIdentity.Data.Control)
	var remote *yyIdentityHTTPError
	if errors.As(err, &remote) && remote.Status == http.StatusUnauthorized {
		return yyIdentityClearLocked()
	}
	return err
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
				result, e := yyIdentityCall("me", nil, yyIdentity.Data.Session)
				authenticated = e == nil
				if e == nil {
					yyIdentity.Data.User, _ = result["user"].(map[string]any)
					_ = yyIdentitySaveLocked()
				}
				if e != nil {
					detail = e.Error()
					var remote *yyIdentityHTTPError
					if errors.As(e, &remote) && remote.Status == http.StatusUnauthorized {
						_ = yyIdentityCheckBindingLocked()
					}
				}
			}
			writeJSON(w, map[string]any{"ok": true, "authenticated": authenticated, "user": yyIdentity.Data.User, "control_token": yyIdentity.Data.Control, "domain": yyPlatformOrigin, "detail": detail})
			return
		}
		var input struct {
			Action          string `json:"action"`
			Username        string `json:"username"`
			Email           string `json:"email"`
			EmailCode       string `json:"email_code"`
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
		if input.Action == "delete" {
			result, e := yyIdentityCall("delete", map[string]any{"password": input.Password}, yyIdentity.Data.Session)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			if e = yyIdentityClearLocked(); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "云端账号已注销，本机解绑写入失败，请重启后重试：" + e.Error()})
				return
			}
			writeJSON(w, result)
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
		if input.Action != "login" && input.Action != "register" && input.Action != "register-code" {
			writeJSON(w, map[string]any{"ok": false, "error": "未知账号操作"})
			return
		}
		if yyIdentity.Data.User != nil {
			if e := yyIdentityCheckBindingLocked(); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": "无法确认本机账号绑定，请稍后重试：" + e.Error()})
				return
			}
			if yyIdentity.Data.User != nil && (input.Action != "login" || !strings.EqualFold(strings.TrimSpace(input.Username), fmt.Sprint(yyIdentity.Data.User["username"]))) {
				writeJSON(w, map[string]any{"ok": false, "error": "本机数据仍绑定账号「" + fmt.Sprint(yyIdentity.Data.User["username"]) + "」。退出登录不会注销；请登录该账号，在平台账号中办理注销后再注册。"})
				return
			}
		}
		result, e := yyIdentityCall(input.Action, map[string]any{"username": input.Username, "email": input.Email, "email_code": input.EmailCode, "password": input.Password}, "")
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		if input.Action == "register-code" {
			writeJSON(w, result)
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
		// A platform login must not move a Linux node onto the APK slug.
		if yyRemote.URL == "" {
			if getPlatformName() == "linux" {
				yyRemote.URL = "wss://toolbox.youyuanqi.dpdns.org/device/connect?device=linux-server"
			} else {
				yyRemote.URL = "wss://toolbox.youyuanqi.dpdns.org/device/connect"
			}
		}
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
