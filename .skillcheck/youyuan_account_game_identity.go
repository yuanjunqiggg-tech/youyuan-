package main

import (
	"bot-apk/config"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

func yyGameCredentialBinding(token, auth string) string {
	if token == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(strings.TrimRight(strings.TrimSpace(auth), "/") + "\x00" + token))
	return hex.EncodeToString(digest[:])
}

func yyRememberMainAuthenticatedGameID(token, auth string, uid int64) {
	if uid <= 0 {
		return
	}
	if err := config.RecordAuthenticatedGameID(token, auth, strconv.FormatInt(uid, 10)); err != nil {
		debugLog("[账号身份] 未持久保存本次已认证身份: %v", err)
	}
}

// A game UID comes only from the current authenticated connection. Saved labels,
// platform identity, entity IDs, XUIDs and bearer contents are not substitutes.
// The frontend may join this public UID to its already-authorized official
// profile list; this function performs no network request or portrait inference.
func yyMainLiveAuthenticatedGameID() string {
	cfg := config.ReloadConfig()
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		return ""
	}
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if !bm.connected || bm.conn == nil || bm.conn.Context().Err() != nil {
		return ""
	}
	if bm.gameCredentialBinding == "" || bm.gameCredentialBinding != yyGameCredentialBinding(cfg.Token, cfg.AuthURL) {
		return ""
	}
	uid := bm.conn.IdentityData().Uid
	if uid <= 0 {
		return ""
	}
	return strconv.FormatInt(uid, 10)
}

func yyMainGameIdentityView() map[string]any {
	if uid := yyMainLiveAuthenticatedGameID(); uid != "" {
		return map[string]any{"game_id": uid, "game_identity_source": "authenticated_connection"}
	}
	uid, confirmedAt := config.AuthenticatedGameID()
	if uid == "" {
		return nil
	}
	return map[string]any{"game_id": uid, "game_identity_source": "authenticated_cache", "game_identity_confirmed_at": confirmedAt}
}

func yyMainAuthenticatedGameID() string {
	uid, _ := yyMainGameIdentityView()["game_id"].(string)
	return uid
}
