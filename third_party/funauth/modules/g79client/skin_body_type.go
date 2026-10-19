package g79client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetSkinBodyType queries the selected official component; absence is unknown,
// never silently interpreted as a slim body.
func (c *Client) GetSkinBodyType(itemID string) (bool, error) {
	path := "/pe-item/query/search-by-id-list"
	payload, _ := json.Marshal(map[string]any{"item_id_list": []string{itemID}})
	req, err := http.NewRequest("POST", c.ReleaseJSON.ApiGatewayUrl+path, strings.NewReader(string(payload)))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "WPFLauncher/0.0.0.0")
	req.Header.Set("user-id", c.UserID)
	req.Header.Set("user-token", CalculateDynamicToken(path, string(payload), c.UserToken))
	resp, err := c.do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	raw, err := readResponseBody(resp)
	if err != nil {
		return false, err
	}
	var result struct {
		Response
		Entities []struct {
			Body *int `json:"skin_body_type"`
		} `json:"entities"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return false, err
	}
	if resp.StatusCode != 200 || result.Code != 0 || len(result.Entities) != 1 || result.Entities[0].Body == nil {
		return false, fmt.Errorf("官方皮肤体型未返回有效值")
	}
	switch *result.Entities[0].Body {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("官方皮肤体型未知：%d", *result.Entities[0].Body)
	}
}
