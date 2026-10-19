package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQQRecentContextIsBoundedAndGroupIsolated(t *testing.T) {
	old := yyQQMessages.Groups
	defer func() { yyQQMessages.Groups = old }()
	yyQQMessages.Groups = map[string][]yyQQGroupMessage{}
	cf := yyQQConfig{GroupID: "g", Groups: []yyQQGroup{{ID: "g", Enabled: true}, {ID: "other", Enabled: true}}}
	cf.Assistant.Nicknames = map[string]string{"opaque": "疣"}
	for i := 0; i < 100; i++ {
		yyQQRememberGroupMessage(cf, "opaque", "opaque", "公开积分80，位置 1, 2, 3", fmt.Sprint(i))
	}
	rows := yyQQRecentMessages(cf, 100)
	if len(rows) != 80 || rows[0].Name != "疣" {
		t.Fatal(len(rows), rows[0].Name)
	}
	prompt := yyQQGroupContext(cf, false)
	if strings.Contains(prompt, "1, 2, 3") || !strings.Contains(prompt, "公开积分80") || strings.Contains(prompt, "opaque") {
		t.Fatal("unsafe context", prompt)
	}
	cf.GroupID = "other"
	if len(yyQQRecentMessages(cf, 80)) != 0 {
		t.Fatal("group messages leaked")
	}
	cf.GroupID = "g"
	cf.Official.AppID = "different-bot"
	if len(yyQQRecentMessages(cf, 80)) != 0 {
		t.Fatal("another bot's context leaked")
	}
	if yyQQSenderName(map[string]any{"card": "群名片", "nickname": "昵称"}) != "群名片" || yyQQSenderName(map[string]any{}) != "" {
		t.Fatal("sender name")
	}
}

func TestQQGroupLogUsesReceivedHistoryAndHidesMemberCoordinates(t *testing.T) {
	old := yyQQMessages.Groups
	defer func() { yyQQMessages.Groups = old }()
	yyQQMessages.Groups = map[string][]yyQQGroupMessage{}
	cf := yyQQConfig{GroupID: "g", Groups: []yyQQGroup{{ID: "g", Enabled: true}}}
	yyQQRememberGroupMessage(cf, "member", "群昵称", "我在 123, 64, 456，积分9", "1")
	yyQQRememberGroupMessage(cf, "member", "群昵称", "最近的消息", "2")
	reply := strings.Join(yyQQAssistantCommand(cf, "群昵称", "member", ".群日志 2", "id"), "\n")
	if !strings.Contains(reply, "最近的消息") || !strings.Contains(reply, "群昵称") || strings.Contains(reply, "123, 64, 456") {
		t.Fatal(reply)
	}
	if !strings.Contains(strings.Join(yyQQAssistantCommand(cf, "群昵称", "member", ".群日志 1000", "id"), ""), "1..80") {
		t.Fatal("unbounded history")
	}
}

func TestQQMessageRouteRejectsUnknownLegacyGroup(t *testing.T) {
	old := yyQQ
	defer func() { yyQQ = old }()
	yyQQ = yyQQConfig{GroupID: "allowed"}
	mux := http.NewServeMux()
	yyQQMessageRoutes(mux)
	r := httptest.NewRecorder()
	mux.ServeHTTP(r, httptest.NewRequest("GET", "/api/qq/messages?group=foreign", nil))
	var result map[string]any
	json.Unmarshal(r.Body.Bytes(), &result)
	if result["ok"] != false {
		t.Fatal("unconfigured group accepted")
	}
}

func TestQQNormalConversationStillStopsForOwner(t *testing.T) {
	oldSessions, oldEpoch := yyQQCompanions.Sessions, yyQQCompanions.Epoch
	defer func() { yyQQCompanions.Sessions = oldSessions; yyQQCompanions.Epoch = oldEpoch }()
	yyQQCompanions.Sessions = map[string]yyQQCompanionSession{}
	yyQQCompanions.Epoch = map[string]uint64{}
	cf := yyQQConfig{GroupID: "normal"}
	cf.Assistant.Companion = true
	cf.Assistant.Model = "test"
	cf.Assistant.CompanionAllMessages = true
	cf.Assistant.CompanionOwner = "owner"
	now := time.Now()
	key, epoch, seq, _, ok := yyQQCompanionBegin(cf, "member", "晚上好", false, now)
	if !ok {
		t.Fatal("ordinary message not accepted")
	}
	yyQQCompanionBegin(cf, "owner", "我来了", false, now.Add(time.Second))
	if yyQQCompanionCurrent(cf, key, epoch, seq, now.Add(time.Second)) {
		t.Fatal("owner response failed to stop")
	}
	if _, _, _, _, ok := yyQQCompanionBegin(cf, "member", "/命令 kill @a", false, now); ok {
		t.Fatal("command treated as chat")
	}
}

func TestQQDrawingUsesActualProviderResponse(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" || r.Header.Get("Authorization") != "Bearer test-image-key" {
			t.Error("wrong image route or credentials")
		}
		var request map[string]any
		json.NewDecoder(r.Body).Decode(&request)
		if request["model"] != "image-model" || request["prompt"] != "画一只羊" {
			t.Error(request)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"b64_json": base64.StdEncoding.EncodeToString(buf.Bytes())}}})
	}))
	defer server.Close()
	got, e := yyQQGenerateImage(context.Background(), &AIModel{BaseURL: server.URL, ModelID: "image-model", APIKey: "test-image-key"}, "画一只羊")
	if e != nil || !bytes.Equal(got, buf.Bytes()) {
		t.Fatal(e)
	}
	if _, e := yyQQGenerateImage(context.Background(), nil, "画羊"); e == nil {
		t.Fatal("missing provider pretended success")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"url":"http://localhost/private"}]}`))
	}))
	defer bad.Close()
	if _, e := yyQQGenerateImage(context.Background(), &AIModel{BaseURL: bad.URL}, "画羊"); e == nil {
		t.Fatal("unsupported URL accepted")
	}
}
