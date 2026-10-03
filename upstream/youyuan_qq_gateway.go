package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"io"
	"net/http"
	"os"
	"regexp"
	"sync/atomic"
	"time"
)

var yyQQNumericGroup = regexp.MustCompile(`^[0-9]+$`)

func yyQQNormalizeConfig(next *yyQQConfig, old yyQQConfig) {
	if next.Adapter != "qqofficial" {
		return
	}
	next.Official.AppID = regexp.MustCompile(`\s+`).ReplaceAllString(next.Official.AppID, "")
	if next.Official.AppID != old.Official.AppID && old.Official.AppID != "" {
		next.GroupID = nil
		next.Groups = nil
	}
	group := yyQQGroupString(next.GroupID)
	if yyQQNumericGroup.MatchString(group) && group != "0" {
		next.GroupNumber = group
		next.GroupID = nil
		if old.Adapter == "qqofficial" && old.Official.AppID == next.Official.AppID && yyQQGroupString(old.GroupID) != "" && !yyQQNumericGroup.MatchString(yyQQGroupString(old.GroupID)) {
			next.GroupID = old.GroupID
		}
	}
}

func yyQQOfficialEvent(cf yyQQConfig, eventType, eventID string, data map[string]any) {
	if eventType != "GROUP_AT_MESSAGE_CREATE" && eventType != "GROUP_MESSAGE_CREATE" {
		return
	}
	group, _ := data["group_openid"].(string)
	if group == "" {
		return
	}
	current := yyQQSnapshot()
	if !current.Enabled || current.Adapter != "qqofficial" || current.Official.AppID != cf.Official.AppID {
		return
	}
	text, _ := data["content"].(string)
	if eventType == "GROUP_AT_MESSAGE_CREATE" {
		text = yyQQOfficialText(text)
	}
	if !yyQQGroupEnabled(yyQQSnapshot(), group) && yyQQHasPrefix(text, "/加群") {
		if ok, e := yyQQEnrollOfficialGroup(group, text); e != nil {
			yyQQLog("加群失败：" + e.Error())
			return
		} else if ok {
			text = "帮助"
		}
	}
	yyQQMu.Lock()
	if yyQQ.Official.AppID != cf.Official.AppID || !yyQQ.Enabled {
		yyQQMu.Unlock()
		return
	}
	if yyQQ.Groups == nil && (yyQQGroupString(yyQQ.GroupID) == "" || yyQQGroupString(yyQQ.GroupID) == "0") {
		yyQQ.GroupID = group
		raw, _ := json.MarshalIndent(yyQQ, "", "  ")
		if e := os.WriteFile(yyQQPath, raw, 0600); e != nil {
			yyQQDetail = "已收到群事件，但保存群绑定失败：" + e.Error()
		}
	}
	cf = yyQQ
	yyQQMu.Unlock()
	if !yyQQGroupEnabled(cf, group) {
		return
	}
	cf.GroupID = group
	author, _ := data["author"].(map[string]any)
	name, _ := author["member_openid"].(string)
	if name == "" {
		yyQQLog("官方群事件缺少member_openid，未处理身份或权限操作")
		return
	}
	id, _ := data["id"].(string)
	if id == "" {
		id = eventID
	}
	yyQQRememberReply(group, id)
	if eventType == "GROUP_AT_MESSAGE_CREATE" {
		text = yyQQOfficialText(text)
	}
	go yyQQToGameMention(cf, yyQQDisplayName(cf, name, ""), name, "member", text, id, eventType == "GROUP_AT_MESSAGE_CREATE")
}

// An outbound official connection removes the requirement for a reachable webhook.
// One receiver, one heartbeat, bounded messages and reconnect backoff; no busy loop.
func yyQQOfficialConnect(ctx context.Context, cf yyQQConfig) error {
	token, e := yyQQAccessToken(cf)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "GET", "https://api.sgroup.qq.com/gateway/bot", nil)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("X-Union-Appid", cf.Official.AppID)
	response, e := yyQQHTTP.Do(req)
	if e != nil {
		return e
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, 65536))
	response.Body.Close()
	if e != nil {
		return e
	}
	var gateway struct {
		URL     string
		Code    int
		Message string
	}
	if e = json.Unmarshal(raw, &gateway); e != nil {
		return e
	}
	if response.StatusCode != 200 || gateway.URL == "" {
		return fmt.Errorf("QQ 网关 HTTP %d code=%d：%s", response.StatusCode, gateway.Code, gateway.Message)
	}
	return yyQQGatewaySession(ctx, cf, token, gateway.URL)
}
func yyQQGatewaySession(ctx context.Context, cf yyQQConfig, token, address string) error {
	dialCtx, cancelDial := context.WithTimeout(ctx, 20*time.Second)
	conn, _, e := websocket.Dial(dialCtx, address, nil)
	cancelDial()
	if e != nil {
		return e
	}
	defer conn.CloseNow()
	conn.SetReadLimit(2 << 20)
	readCtx, cancelHello := context.WithTimeout(ctx, 20*time.Second)
	_, raw, e := conn.Read(readCtx)
	cancelHello()
	if e != nil {
		return e
	}
	var hello struct {
		Op int
		D  struct {
			Interval int `json:"heartbeat_interval"`
		}
	}
	if e = json.Unmarshal(raw, &hello); e != nil {
		return e
	}
	if hello.Op != 10 || hello.D.Interval < 1000 || hello.D.Interval > 120000 {
		return fmt.Errorf("QQ 网关心跳参数无效")
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var sequence atomic.Int64
	var ack atomic.Bool
	ack.Store(true)
	write := func(value any) error {
		raw, _ := json.Marshal(value)
		wctx, wcancel := context.WithTimeout(sessionCtx, 10*time.Second)
		defer wcancel()
		return conn.Write(wctx, websocket.MessageText, raw)
	}
	if e = write(map[string]any{"op": 2, "d": map[string]any{"token": "QQBot " + token, "intents": 1 << 25, "shard": []int{0, 1}, "properties": map[string]string{"$os": "android", "$device": "youyuan"}}}); e != nil {
		return e
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Duration(hello.D.Interval) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-ticker.C:
				if !ack.Swap(false) {
					cancel()
					return
				}
				if write(map[string]any{"op": 1, "d": sequence.Load()}) != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-done }()
	ready := false
	for {
		rctx, rcancel := context.WithTimeout(sessionCtx, time.Duration(hello.D.Interval)*2*time.Millisecond+20*time.Second)
		_, raw, e = conn.Read(rctx)
		rcancel()
		if e != nil {
			return fmt.Errorf("QQ 长连接中断，将重连：%w", e)
		}
		var event struct {
			Op    int
			S     *int64
			T, ID string
			D     map[string]any
		}
		if e = json.Unmarshal(raw, &event); e != nil {
			return e
		}
		if event.S != nil {
			sequence.Store(*event.S)
		}
		switch event.Op {
		case 11:
			ack.Store(true)
		case 1:
			if e = write(map[string]any{"op": 1, "d": sequence.Load()}); e != nil {
				return e
			}
		case 7, 9:
			return fmt.Errorf("QQ 网关要求重新登录 op=%d", event.Op)
		case 0:
			if event.T == "READY" {
				ready = true
				yyQQMu.Lock()
				yyQQState = "open"
				yyQQDetail = "QQ 官方长连接已鉴权，等待群消息"
				yyQQMu.Unlock()
			}
			if ready {
				yyQQOfficialEvent(cf, event.T, event.ID, event.D)
			}
		}
	}
}
