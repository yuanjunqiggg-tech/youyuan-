package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"
)

// A separate operator-selected image provider; text-only models are never
// presented as drawing providers and no user key is embedded in the app.
func yyQQGenerateImage(ctx context.Context, model *AIModel, prompt string) ([]byte, error) {
	if model == nil {
		return nil, fmt.Errorf("尚未配置绘图模型，请在群服设置选择支持 images/generations 的模型")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || len([]rune(prompt)) > 2000 {
		return nil, fmt.Errorf("绘图描述需1–2000个字符")
	}
	base := strings.TrimRight(model.BaseURL, "/")
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	raw, _ := json.Marshal(map[string]any{"model": model.ModelID, "prompt": prompt, "n": 1, "size": "1024x1024", "response_format": "b64_json"})
	request, e := http.NewRequestWithContext(ctx, "POST", base+"/images/generations", bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+model.APIKey)
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Do(request)
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	data, e := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
	if e != nil {
		return nil, e
	}
	if len(data) > 4*1024*1024 {
		return nil, fmt.Errorf("绘图接口返回超过4MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("绘图接口HTTP %d，请检查所选模型是否支持图像生成", response.StatusCode)
	}
	var result struct {
		Data []struct {
			B64 string `json:"b64_json"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &result) != nil || len(result.Data) == 0 || result.Data[0].B64 == "" {
		return nil, fmt.Errorf("绘图接口未返回b64_json图片；当前适配器需要该格式")
	}
	picture, e := base64.StdEncoding.DecodeString(result.Data[0].B64)
	if e != nil {
		return nil, fmt.Errorf("绘图接口返回无效图片编码")
	}
	if len(picture) > 2*1024*1024 {
		return nil, fmt.Errorf("图片超过当前QQ图片上传2MiB预算")
	}
	info, _, e := image.DecodeConfig(bytes.NewReader(picture))
	if e != nil || info.Width < 1 || info.Height < 1 || info.Width > 4096 || info.Height > 4096 {
		return nil, fmt.Errorf("绘图接口返回无效或过大图像")
	}
	return picture, nil
}
