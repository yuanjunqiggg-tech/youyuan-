//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func yyHideAccountWindow(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
