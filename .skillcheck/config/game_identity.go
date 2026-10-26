package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Kept outside AppConfig: public config responses and caller-supplied settings
// never serialize or populate the private credential binding/cache.
type authenticatedGameIdentity struct {
	GameID            string `json:"game_id"`
	ConfirmedAt       string `json:"confirmed_at"`
	CredentialBinding string `json:"credential_binding"`
}
type diskConfig struct {
	*AppConfig
	GameIdentity *authenticatedGameIdentity `json:"authenticated_game_identity,omitempty"`
}

func gameCredentialBinding(token, authURL string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.TrimRight(strings.TrimSpace(authURL), "/") + "\x00" + token))
	return hex.EncodeToString(sum[:])
}

// RecordAuthenticatedGameID must only be called with the UID obtained from a
// successful game connection, never a nickname, request field or bearer decode.
func RecordAuthenticatedGameID(token, authURL, uid string) error {
	parsed, err := strconv.ParseInt(uid, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != uid {
		return fmt.Errorf("无效的已认证游戏UID")
	}
	configMu.Lock()
	defer configMu.Unlock()
	cfg := loadLocked(true)
	binding := gameCredentialBinding(token, authURL)
	if binding == "" || binding != gameCredentialBinding(cfg.Token, cfg.AuthURL) {
		return fmt.Errorf("游戏凭据已更改，未保存旧连接身份")
	}
	identity := &authenticatedGameIdentity{GameID: uid, ConfirmedAt: time.Now().UTC().Format(time.RFC3339Nano), CredentialBinding: binding}
	return saveLocked(cfg, identity)
}

// AuthenticatedGameID exposes only the public UID and confirmation time, and
// revalidates its private binding against the current saved credential.
func AuthenticatedGameID() (uid, confirmedAt string) {
	configMu.Lock()
	defer configMu.Unlock()
	cfg := loadLocked(true)
	if currentGameIdentity == nil || cfg.Token == "" || currentGameIdentity.CredentialBinding != gameCredentialBinding(cfg.Token, cfg.AuthURL) {
		return "", ""
	}
	return currentGameIdentity.GameID, currentGameIdentity.ConfirmedAt
}
