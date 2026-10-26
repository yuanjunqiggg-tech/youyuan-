package main

import (
	"fmt"
	"strconv"
	"strings"
)

func yyRuntimeVersionValid(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) > 3 || len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		n, e := strconv.Atoi(part)
		if e != nil || n < 0 {
			return false
		}
	}
	return true
}
func yyRuntimeCheckDependencies(manifest yyRuntimeManifest, account string) error {
	yyRuntimeMu.Lock()
	defer yyRuntimeMu.Unlock()
	for id, minimum := range manifest.Dependencies {
		if id == manifest.ID {
			return fmt.Errorf("插件不能依赖自己")
		}
		dependency := yyRuntimeInstances[account+":"+id]
		if dependency == nil {
			return fmt.Errorf("前置插件 %s 未启动，请先启动前置插件", id)
		}
		if minimum != "" && minimum != "*" {
			if !yyRuntimeVersionValid(minimum) || !yyRuntimeVersionValid(dependency.Manifest.Version) {
				return fmt.Errorf("依赖版本必须是数字版本，如 1.0.0")
			}
			if !versionAtLeast(dependency.Manifest.Version, minimum) {
				return fmt.Errorf("前置插件 %s 版本 %s 不满足最低 %s", id, dependency.Manifest.Version, minimum)
			}
		}
	}
	return nil
}
