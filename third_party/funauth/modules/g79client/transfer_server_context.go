package g79client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Resolve the official transfer list with this request's transport/deadline.
// This avoids an uncancellable global cache lock and default HTTP transport.
func (c *Client) FetchG79TransferServersContext(ctx context.Context) ([]G79TransferServerEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	fetch := func(target string, out any) error {
		req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "libhttpclient/1.0.0.0")
		resp, err := resolveHTTPClient(c.httpClient).Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("transfer discovery: HTTP %d", resp.StatusCode)
		}
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
	}
	var release G79ReleaseJSON
	if err := fetch("https://g79.update.netease.com/serverlist/adr_release.0.17.json", &release); err != nil {
		return nil, err
	}
	if release.TransferServerUrl == "" {
		return nil, fmt.Errorf("official release has no transfer server URL")
	}
	var list []G79TransferServerEntry
	if err := fetch(release.TransferServerUrl, &list); err != nil {
		return nil, err
	}
	return list, nil
}
