//go:build !android

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

var yyDesktopAccounts = struct {
	sync.Mutex
	items map[int]*exec.Cmd
}{items: map[int]*exec.Cmd{}}

func yyAccountProcessStart(slot int, id, dir string) error {
	yyDesktopAccounts.Lock()
	defer yyDesktopAccounts.Unlock()
	if yyDesktopAccounts.items[slot] != nil {
		return nil
	}
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	cmd := exec.Command(executable)
	cmd.Env = append(os.Environ(), "YY_DATA_DIR="+dir, "YY_PORT="+strconv.Itoa(19500+slot), "YY_ACCOUNT_WORKER=1")
	yyHideAccountWindow(cmd)
	log, e := os.OpenFile(filepath.Join(dir, "engine.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		log.Close()
		return e
	}
	yyDesktopAccounts.items[slot] = cmd
	go func() {
		_ = cmd.Wait()
		_ = log.Close()
		yyDesktopAccounts.Lock()
		if yyDesktopAccounts.items[slot] == cmd {
			delete(yyDesktopAccounts.items, slot)
		}
		yyDesktopAccounts.Unlock()
	}()
	return nil
}
func yyAccountProcessStop(slot int) error {
	yyDesktopAccounts.Lock()
	cmd := yyDesktopAccounts.items[slot]
	yyDesktopAccounts.Unlock()
	if cmd != nil {
		return cmd.Process.Kill()
	}
	return nil
}
