package main

import (
	"bot-apk/config"
	"strings"
)

// Called under yyAccountMutationMu and yyAccounts.Lock. Only an explicit new
// save may claim an unconfigured main; old children are never promoted by order.
func yyTryPrimaryAccountSave(a yyManagedAccount, existing, allowed bool) (bool, error) {
	if !allowed || a.Token == "" {
		return false, nil
	}
	yyConfigMutationMu.Lock()
	defer yyConfigMutationMu.Unlock()
	cfg := config.ReloadConfig()
	if cfg.Token != "" {
		// Exact credential idempotency only. A label/role/owner is not identity.
		return cfg.Token == a.Token && strings.TrimRight(cfg.AuthURL, "/") == strings.TrimRight(a.AuthURL, "/"), nil
	}
	// An old installation with saved children has no trustworthy creation order.
	// Preserve it instead of silently migrating an arbitrary child/workspace.
	if existing || len(yyAccounts.Items) != 0 {
		return false, nil
	}
	if bm := getBotManagerLocked(); bm != nil && bm.IsConnected() {
		return false, nil
	}
	taskMu.Lock()
	defer taskMu.Unlock()
	yyAutoConnection.Lock()
	defer yyAutoConnection.Unlock()
	if activeTask != nil || yyAutoConnection.busy {
		return false, nil
	}
	cfg.Token, cfg.AuthURL, cfg.ServerCode, cfg.ServerPass = a.Token, a.AuthURL, a.Server, a.Password
	cfg.MainAccountName = a.Name
	cfg.AutoConnect, cfg.AutoReconnect = false, a.KeepAlive
	if err := config.SaveConfig(cfg); err != nil {
		return true, err
	}
	// Saving credentials is not permission to open a game connection. Keep the
	// user's keep-alive preference, but require their explicit Connect to resume.
	yyAutoConnection.paused, yyAutoConnection.wanted = true, false
	return true, nil
}
