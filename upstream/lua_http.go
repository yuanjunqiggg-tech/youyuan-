package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxHTTPBody 插件 HTTP 调用响应体上限（10MB），防异常大响应撑爆内存。
const maxHTTPBody = 10 << 20

// readHTTPBody 读取响应体，超过 maxHTTPBody 返回错误。
func readHTTPBody(resp *http.Response) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxHTTPBody {
		return nil, fmt.Errorf("响应体超过 %dMB 上限", maxHTTPBody>>20)
	}
	return data, nil
}

// httpGetText 发起 GET 请求，返回响应体文本。
func httpGetText(url string) (string, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := readHTTPBody(resp)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 300))
	}
	return string(data), nil
}

// httpPostText 发起 POST 请求（body 为任意字符串，headers 为 header 表），返回响应体文本。
func httpPostText(url, body string, headers map[string]string) (string, error) {
	return httpPostTextContext(context.Background(), url, body, headers)
}
func httpPostTextContext(ctx context.Context, url, body string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := readHTTPBody(resp)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 300))
	}
	return string(data), nil
}

// httpRequestFull 发起任意方法 HTTP 请求，返回 (状态码, 响应头, 响应体, 错误)。
// 传输错误（连不上/超时）才返回 err；非 2xx 状态码也正常返回，由调用方检查 status。
func httpRequestFull(method, url, body string, headers map[string]string, timeout time.Duration) (int, map[string]string, string, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return 0, nil, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if _, ok := headers["Content-Type"]; !ok && (method == "POST" || method == "PUT" || method == "PATCH") {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	data, err := readHTTPBody(resp)
	if err != nil {
		return 0, nil, "", err
	}
	h := map[string]string{}
	for k, v := range resp.Header {
		if len(v) > 0 {
			h[k] = v[0]
		}
	}
	return resp.StatusCode, h, string(data), nil
}

// jsonEncodeGo 把 Go 值编码为 JSON 字符串。
func jsonEncodeGo(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// jsonDecodeGo 把 JSON 字符串解码为 Go 值（map[string]any / []any / 基本类型）。
func jsonDecodeGo(s string) (any, error) {
	var v any
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}
