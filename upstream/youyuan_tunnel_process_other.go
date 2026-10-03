//go:build !linux && !android

package main

import "os/exec"

func yyPrepareTunnelProcess(cmd *exec.Cmd) { yyHideAccountWindow(cmd) }
