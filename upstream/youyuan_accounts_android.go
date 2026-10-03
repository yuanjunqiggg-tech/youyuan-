//go:build android

package main

import (
	"encoding/json"
	"fmt"
)

func yyAccountProcessStart(slot int, id, dir string) error {
	return yyAccountProcessCall("start", slot, id, dir)
}
func yyAccountProcessStop(slot int) error { return yyAccountProcessCall("stop", slot, "", "") }
func yyAccountProcessCall(action string, slot int, id, dir string) error {
	body, _ := json.Marshal(map[string]any{"action": action, "slot": slot, "id": id, "dir": dir})
	text, e := callAndroidSystem("accountProcess", string(body))
	if e != nil {
		return e
	}
	var response map[string]any
	if e = json.Unmarshal([]byte(text), &response); e != nil {
		return e
	}
	if response["ok"] != true {
		return fmt.Errorf("账号进程: %v", response["error"])
	}
	return nil
}
