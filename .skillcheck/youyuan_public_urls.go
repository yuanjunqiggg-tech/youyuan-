package main

import "net/url"

func yyPublicURLs(raw string) map[string]string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "wss" || u.Host == "" || u.User != nil {
		return map[string]string{}
	}
	query := url.Values{}
	if device := u.Query().Get("device"); device != "" {
		query.Set("device", device)
	}
	result := map[string]string{}
	for name, path := range map[string]string{"console": "/", "mcp": "/mcp/", "api": "/api/youyuan/catalog"} {
		result[name] = (&url.URL{Scheme: "https", Host: u.Host, Path: path, RawQuery: query.Encode()}).String()
	}
	return result
}
