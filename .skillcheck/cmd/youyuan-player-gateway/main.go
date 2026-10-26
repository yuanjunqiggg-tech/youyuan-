package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bot-apk/internal/playerrelay"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
)

func main() {
	// Match the deployed toolbox's NetEase protocol rather than the vendor
	// library's older standalone default. Raw application frames stay unchanged.
	protocol.CurrentProtocol = 860
	protocol.CurrentVersion = "1.21.120"
	p := flag.String("config", "", "私有配置 JSON 路径（不在命令行填写令牌）")
	probe := flag.Bool("probe", false, "用真实认证协议客户端验收代理进服（不等于真人游戏界面验收）")
	flag.Parse()
	c, e := playerrelay.ReadPrivateConfig(*p)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *probe {
		ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
		defer cancel()
		r, e := playerrelay.RunLiveProbe(ctx, c, nil)
		if e != nil {
			r["error"] = e.Error()
		}
		b, _ := json.Marshal(r)
		fmt.Println(string(b))
		if e != nil {
			os.Exit(1)
		}
		return
	}
	g := &playerrelay.Gateway{Config: c}
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				b, _ := json.Marshal(g.Stats.Snapshot())
				fmt.Println(string(b))
			}
		}
	}()
	e = g.Serve(ctx, func(addr string) {
		b, _ := json.Marshal(map[string]any{"listening": addr, "authentication_required": true, "scope": "only the configured player routed through this gateway"})
		fmt.Println(string(b))
	})
	if e != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
