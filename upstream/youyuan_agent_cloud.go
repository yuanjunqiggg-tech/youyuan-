package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func yyAgentCloudKey() string {
	yyRemoteMu.RLock()
	key := yyRemote.DeviceToken
	yyRemoteMu.RUnlock()
	if key == "" && yyAccounts.Worker {
		data, e := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(yyAccounts.Root)), "youyuan-remote.json"))
		if e == nil {
			var parent yyRemoteConfig
			if json.Unmarshal(data, &parent) == nil {
				key = parent.DeviceToken
			}
		}
	}
	return key
}

var yyCloudCapability = struct {
	sync.Mutex
	Key     [32]byte
	Until   time.Time
	Allowed bool
}{}

func yyCheckCloudCapability(ctx context.Context, key, origin string) bool {
	if key == "" || len(key) > 256 {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/api/cloud/ai-capability", nil)
	if e != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "YouYuan-Toolbox/"+yyAppVersion)
	// Credentials never follow redirects away from the trusted capability service.
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	res, e := client.Do(req)
	if e != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		OK      bool `json:"ok"`
		Allowed bool `json:"allowed"`
	}
	return json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&result) == nil && result.OK && result.Allowed
}
func yyAgentCloudAllowed(ctx context.Context, key string) bool {
	if key == "" {
		return false
	}
	digest := sha256.Sum256([]byte(key))
	yyCloudCapability.Lock()
	defer yyCloudCapability.Unlock()
	if yyCloudCapability.Key == digest && time.Now().Before(yyCloudCapability.Until) {
		return yyCloudCapability.Allowed
	}
	allowed := yyCheckCloudCapability(ctx, key, yySecPlatformOrigin())
	yyCloudCapability.Key = digest
	yyCloudCapability.Allowed = allowed
	yyCloudCapability.Until = time.Now().Add(30 * time.Second)
	return allowed
}
