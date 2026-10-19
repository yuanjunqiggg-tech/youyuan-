package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	auth "github.com/Yeah114/FunAuth/auth"
	"image"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxSkinResource = 8 << 20

func officialSkinURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "netease.com" || strings.HasSuffix(h, ".netease.com") || strings.HasSuffix(h, ".163.com")
}
func skinPreview(ctx context.Context, skin auth.SkinInfo) (map[string]any, error) {
	if !officialSkinURL(skin.SkinDownloadURL) {
		return nil, fmt.Errorf("当前皮肤没有受支持的官方资源地址")
	}
	req, e := http.NewRequestWithContext(ctx, "GET", skin.SkinDownloadURL, nil)
	if e != nil {
		return nil, e
	}
	client := &http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 4 || !officialSkinURL(r.URL.String()) {
			return fmt.Errorf("皮肤资源跳转地址不受支持")
		}
		return nil
	}}
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("皮肤资源HTTP %d", resp.StatusCode)
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, maxSkinResource+1))
	if e != nil {
		return nil, e
	}
	if len(raw) > maxSkinResource {
		return nil, fmt.Errorf("皮肤资源超过8MiB")
	}
	return decodeSkinPreview(raw)
}
func decodeSkinPreview(raw []byte) (map[string]any, error) {
	texture := raw
	custom := false
	if len(raw) > 4 && string(raw[:2]) == "PK" {
		z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if e != nil {
			return nil, e
		}
		var best *zip.File
		for _, f := range z.File {
			name := strings.ToLower(f.Name)
			if strings.HasSuffix(name, ".geo.json") || strings.Contains(name, "geometry") {
				custom = true
			}
			if strings.HasSuffix(name, ".png") && !strings.Contains(name, "icon") && !strings.Contains(name, "pack.png") && (best == nil || strings.HasSuffix(name, "skin.png")) {
				best = f
			}
		}
		if best == nil || best.UncompressedSize64 > maxSkinResource {
			return nil, fmt.Errorf("皮肤包没有受支持的PNG纹理")
		}
		f, e := best.Open()
		if e != nil {
			return nil, e
		}
		defer f.Close()
		texture, e = io.ReadAll(io.LimitReader(f, maxSkinResource+1))
		if e != nil {
			return nil, e
		}
	}
	if len(texture) > maxSkinResource {
		return nil, fmt.Errorf("皮肤纹理过大")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(texture))
	if e != nil || format != "png" {
		return nil, fmt.Errorf("皮肤不是有效PNG纹理")
	}
	if cfg.Width < 64 || cfg.Width > 1024 || cfg.Width%64 != 0 || (cfg.Height != cfg.Width && cfg.Height*2 != cfg.Width) {
		return nil, fmt.Errorf("非标准皮肤尺寸 %dx%d，无法套用标准骨骼", cfg.Width, cfg.Height)
	}
	return map[string]any{"texture": "data:image/png;base64," + base64.StdEncoding.EncodeToString(texture), "width": cfg.Width, "height": cfg.Height, "custom_geometry": custom, "preview_kind": "standard_humanoid"}, nil
}
