/*
PhoenixBuilder specific.
Author: CoozillaX
*/
package minecraft

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"
)

var pharosClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Resolver: &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "udp", "8.8.8.8:53")
				},
			},
		}).DialContext,
	},
	Timeout: 10 * time.Second,
}

// pharosResponse ..
type pharosResponse struct {
	Harbor struct {
		Servers map[string]map[string][][]string `json:"servers"`
		Warning string                           `json:"warning"`
	} `json:"harbor"`
}

// getPharosSpeedUpIP ..
func getPharosSpeedUpIP(addr string) (string, error) {
	return getPharosSpeedUpIPContext(context.Background(), addr)
}

func getPharosSpeedUpIPContext(ctx context.Context, addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.New("解析地址失败")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://dual.pharos.netease.com/api/v1/harbor/mcrealms?ips=%s", addr), nil)
	if err != nil {
		return "", err
	}
	respReader, err := pharosClient.Do(req)
	if err != nil {
		return "", err
	}
	defer respReader.Body.Close()
	if respReader.StatusCode != http.StatusOK {
		return "", errors.New("官方出口候选查询被拒绝")
	}
	resp := &pharosResponse{}
	if err := json.NewDecoder(io.LimitReader(respReader.Body, 1<<20)).Decode(resp); err != nil {
		return "", err
	}
	if resp.Harbor.Warning != "" {
		return "", errors.New(resp.Harbor.Warning)
	}
	if ports, ok := resp.Harbor.Servers[host]; ok {
		if proxyList, ok := ports[port]; ok && len(proxyList) > 0 {
			randomIndex := rand.Intn(len(proxyList))
			proxyInfo := proxyList[randomIndex]
			if len(proxyInfo) >= 2 {
				candidatePort, e := strconv.Atoi(proxyInfo[1])
				if net.ParseIP(proxyInfo[0]) == nil || e != nil || candidatePort < 1 || candidatePort > 65535 {
					return "", errors.New("官方出口候选地址无效")
				}
				speedUpAddr := net.JoinHostPort(proxyInfo[0], proxyInfo[1])
				return speedUpAddr, nil
			}
		}
	}
	return "", errors.New("未找到给定地址的加速节点")
}
