package config

import "path/filepath"

// GameEgressProxyPath is inside the private runtime configuration directory.
func GameEgressProxyPath() string {
	configMu.Lock()
	defer configMu.Unlock()
	if cfgPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cfgPath), "game-egress-proxy.json")
}
