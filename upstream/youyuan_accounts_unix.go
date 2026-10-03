//go:build !windows && !android

package main

import "os/exec"

func yyHideAccountWindow(cmd *exec.Cmd) {}
