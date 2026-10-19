package g79client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FetchG79LinkServersContext reads the official release document and its link
// list through this client's HTTP transport. It does not reuse the unbounded
// global discovery lock/cache or accept caller-supplied endpoint overrides.
func (c *Client) FetchG79LinkServersContext(ctx context.Context) ([]G79LinkServerEntry, error) {
	if c == nil {
		return nil, fmt.Errorf("link discovery: nil client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	fetch := func(target string, result any) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "libhttpclient/1.0.0.0")
		req.Header.Set("Accept", "application/json")
		resp, err := resolveHTTPClient(c.httpClient).Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("link discovery: HTTP %d", resp.StatusCode)
		}
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result)
	}
	var release G79ReleaseJSON
	if err := fetch("https://g79.update.netease.com/serverlist/adr_release.0.17.json", &release); err != nil {
		return nil, err
	}
	if release.LinkServerURL == "" {
		return nil, fmt.Errorf("link discovery: official release has no LinkServerUrl")
	}
	var servers []G79LinkServerEntry
	if err := fetch(release.LinkServerURL, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}
