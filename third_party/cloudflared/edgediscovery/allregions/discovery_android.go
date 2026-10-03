//go:build android

// YouYuan integration: Android has no resolv.conf for Go's SRV resolver.
// Resolve discovery SRV over HTTPS using public HTTPS resolvers with bounded fallback.
package allregions

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func init() { netLookupSRV = androidLookupSRV; fallbackLookupSRV = androidLookupSRV }
func androidLookupSRV(service, proto, name string) (string, []*net.SRV, error) {
	query := "_" + service + "._" + proto + "." + name
	var failures []string
	for _, endpoint := range []string{"https://dns.alidns.com/resolve", "https://cloudflare-dns.com/dns-query", "https://dns.google/resolve"} {
		result, e := androidHTTPSLookupSRV(endpoint, query)
		if e == nil {
			return query + ".", result, nil
		}
		failures = append(failures, e.Error())
	}
	return "", nil, fmt.Errorf("SRV HTTPS discovery failed: %s", strings.Join(failures, "; "))
}
func androidHTTPSLookupSRV(endpoint, query string) ([]*net.SRV, error) {
	request, e := http.NewRequest("GET", endpoint+"?name="+url.QueryEscape(query)+"&type=SRV", nil)
	if e != nil {
		return nil, e
	}
	request.Header.Set("Accept", "application/dns-json")
	response, e := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if e != nil {
		return nil, e
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("SRV HTTPS HTTP %d", response.StatusCode)
	}
	var answer struct {
		Status int
		Answer []struct {
			Type int
			Data string
		}
	}
	if e = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&answer); e != nil {
		return nil, e
	}
	result := []*net.SRV{}
	for _, item := range answer.Answer {
		if item.Type != 33 {
			continue
		}
		parts := strings.Fields(item.Data)
		if len(parts) != 4 {
			continue
		}
		priority, e1 := strconv.ParseUint(parts[0], 10, 16)
		weight, e2 := strconv.ParseUint(parts[1], 10, 16)
		port, e3 := strconv.ParseUint(parts[2], 10, 16)
		if e1 != nil || e2 != nil || e3 != nil {
			continue
		}
		result = append(result, &net.SRV{Target: parts[3], Port: uint16(port), Priority: uint16(priority), Weight: uint16(weight)})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("SRV HTTPS DNS status=%d returned no records", answer.Status)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Priority < result[j].Priority })
	return result, nil
}
