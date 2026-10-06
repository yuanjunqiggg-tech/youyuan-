package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

const yyAppVersion = "0.10.11"
const yyAppVersionCode = 1011
const yyUpdateManifest = "https://updates.youyuanqi.dpdns.org/updates/android/latest.json"

type yyReleaseManifest struct {
	Version     string `json:"version"`
	VersionCode int    `json:"version_code"`
	Package     string `json:"package"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	APKURL      string `json:"apk_url"`
	Notes       string `json:"release_notes"`
}

func yyCheckRelease(ctx context.Context) (yyReleaseManifest, error) {
	var release yyReleaseManifest
	req, err := http.NewRequestWithContext(ctx, "GET", yyUpdateManifest, nil)
	if err != nil {
		return release, err
	}
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return release, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return release, fmt.Errorf("更新服务返回 HTTP %d", resp.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&release); err != nil {
		return release, err
	}
	u, err := url.Parse(release.APKURL)
	if err != nil || u.Scheme != "https" || u.Host != "updates.youyuanqi.dpdns.org" || u.User != nil || u.Path != "/updates/android/latest.apk" || release.Package != "org.youyuan.toolbox" || release.VersionCode <= 0 || release.Size <= 0 || release.Size > 256*1024*1024 || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(release.SHA256) {
		return release, fmt.Errorf("更新清单校验失败")
	}
	return release, nil
}

var yyDataRoot string

func yyVersion(w http.ResponseWriter, r *http.Request) {
	release, err := yyCheckRelease(r.Context())
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error(), "current_version": yyAppVersion, "current_version_code": yyAppVersionCode})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "current_version": yyAppVersion, "current_version_code": yyAppVersionCode, "latest_version": release.Version, "latest_version_code": release.VersionCode, "update_url": release.APKURL, "update_sha256": release.SHA256, "update_size": release.Size, "force_update": false, "update_message": release.Notes})
}
func yyAnnouncements(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "announcements": []any{}})
}
func yyIndependentInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "product": "游园", "package": "org.youyuan.toolbox", "author_api": false, "quiz_required": false, "license": "AGPL-3.0"})
}
