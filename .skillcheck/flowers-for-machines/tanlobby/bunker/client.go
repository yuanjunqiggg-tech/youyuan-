package bunker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	AuthServer         string
	FBToken            string
	ProvidedPEAuthData string
	ProvidedSaAuthData string
}

func NewClient(authServer string, token string, peAuth string, saAuth string) *Client {
	return &Client{
		AuthServer:         authServer,
		FBToken:            token,
		ProvidedPEAuthData: peAuth,
		ProvidedSaAuthData: saAuth,
	}
}

func parseHttpResponse[T any](resp *http.Response) (result T, err error) {
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return result, fmt.Errorf("parseHttpResponse：HTTP 请求状态码不是 200 (code = %d)", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("解析 HTTP 响应失败：%v", err)
	}

	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return result, fmt.Errorf("解析 HTTP 响应失败：%v", err)
	}

	return result, nil
}
