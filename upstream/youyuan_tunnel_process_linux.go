//go:build linux || android

package main

import (
	"os/exec"
	"syscall"
)

func yyPrepareTunnelProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
