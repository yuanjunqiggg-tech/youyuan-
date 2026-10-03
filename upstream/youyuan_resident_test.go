package main

import (
	"bot-apk/building"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestYouYuanResidentKeepsStateAndSeparatesBots(t *testing.T) {
	a, b := &BotManager{}, &BotManager{}
	id := "yy-persist-test"
	source := `counter=0; function next() counter=counter+1;return counter end; plugin_api={next=next}`
	if err := LoadLuaPlugin(a, id, source); err != nil {
		t.Fatal(err)
	}
	defer yyStopResident(a, id)
	defer yyStopResident(b, id)
	for index := 1; index <= 20; index++ {
		value, err := yyCallResident(a, id, "next", nil, false, false)
		if err != nil || value != float64(index) {
			t.Fatalf("变量被重置：%v %v", value, err)
		}
	}
	other, err := yyCallResident(b, id, "next", nil, false, false)
	if err != nil || other != float64(1) {
		t.Fatalf("机器人状态混用：%v %v", other, err)
	}
	value, err := callPluginFunction(a, id, "next", "[]")
	if err != nil || value != "21" {
		t.Fatalf("跨插件调用重跑源码：%s %v", value, err)
	}
}
func TestYouYuanUnlimitedSpeed(t *testing.T) {
	for _, speed := range []int{-1, 0} {
		if building.UnlimitedSpeedDelay(speed) != 0 {
			t.Fatal("不限速仍产生延时")
		}
	}
	if building.UnlimitedSpeedDelay(100) != 10*time.Millisecond {
		t.Fatal("自定义速率失效")
	}
}
func TestYouYuanOfficialSignatureAndChallenge(t *testing.T) {
	saved := yyQQ
	defer func() { yyQQ = saved }()
	yyQQ.Enabled = true
	yyQQ.Adapter = "qqofficial"
	yyQQ.Official.Secret = "test-app-secret"
	request := httptest.NewRequest("POST", "/api/qq/webhook", strings.NewReader(`{"op":13,"d":{"plain_token":"challenge","event_ts":"123"}}`))
	response := httptest.NewRecorder()
	yyQQWebhook(response, request)
	if response.Code != 401 {
		t.Fatal("未签名回调被接受")
	}
	raw := `{"op":13,"d":{"plain_token":"challenge","event_ts":"123"}}`
	key := yyQQPrivateKey(yyQQ.Official.Secret)
	request = httptest.NewRequest("POST", "/api/qq/webhook", strings.NewReader(raw))
	request.Header.Set("X-Signature-Timestamp", "123")
	request.Header.Set("X-Signature-Ed25519", hex.EncodeToString(ed25519.Sign(key, []byte("123"+raw))))
	response = httptest.NewRecorder()
	yyQQWebhook(response, request)
	if response.Code != 200 {
		t.Fatal("有效官方签名被拒绝", response.Body.String())
	}
	var challenge map[string]string
	_ = json.Unmarshal(response.Body.Bytes(), &challenge)
	signature, err := hex.DecodeString(challenge["signature"])
	if err != nil || challenge["plain_token"] != "challenge" || !ed25519.Verify(key.Public().(ed25519.PublicKey), []byte("123challenge"), signature) {
		t.Fatal("挑战响应签名无效")
	}
}
func TestYouYuanNumericGroupID(t *testing.T) {
	var cfg yyQQConfig
	_ = json.Unmarshal([]byte(`{"group_id":123456789}`), &cfg)
	if yyQQGroupString(cfg.GroupID) != "123456789" {
		t.Fatal("群号被转换为科学计数法")
	}
}
func BenchmarkYouYuanResidentCallback(b *testing.B) {
	bm := &BotManager{}
	id := "yy-benchmark"
	_ = LoadLuaPlugin(bm, id, `counter=0;function next() counter=counter+1;return counter end`)
	defer yyStopResident(bm, id)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = yyCallResident(bm, id, "next", nil, false, false)
	}
}
func BenchmarkYouYuanLegacyCallback(b *testing.B) {
	bm := &BotManager{}
	for i := 0; i < b.N; i++ {
		L := newLuaState(bm)
		_ = L.DoString(`counter=0;function next() counter=counter+1;return counter end; next()`)
		cancelLuaCtx(L)
		L.Close()
	}
}
