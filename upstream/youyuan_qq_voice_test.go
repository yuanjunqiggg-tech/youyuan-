package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVoiceURLAllow(t *testing.T) {
	allow := []string{
		"http://example.com/a.mp3",
		"https://example.com/a.mp3",
		"HTTPS://example.com/a.mp3",
		"base64://SUQzBA==",
	}
	for _, v := range allow {
		if !yyVoiceURL(v) {
			t.Fatalf("应当允许：%q", v)
		}
	}
	deny := []string{
		"", "   ",
		"javascript:alert(1)",
		"file:///c:/a.mp3",
		"data:audio/mp3;base64,SUQz",
		"ftp://example.com/a.mp3",
		"C:\\a.mp3",
	}
	for _, v := range deny {
		if yyVoiceURL(v) {
			t.Fatalf("应当拒绝：%q", v)
		}
	}
}

func TestVoiceModeNormalize(t *testing.T) {
	keep := map[string]string{
		"off": "off", "auto": "auto", "always": "always",
		" OFF ": "off", "Auto": "auto", "ALWAYS": "always",
	}
	for in, want := range keep {
		if got := yyQQVoiceModeNormalize(in); got != want {
			t.Fatalf("归一化 %q = %q，期望 %q", in, got, want)
		}
	}
	for _, in := range []string{"", "on", "every", "auto2", "关闭"} {
		if got := yyQQVoiceModeNormalize(in); got != "" {
			t.Fatalf("非法值 %q 应归一化成空串，got %q", in, got)
		}
	}
	// 脏配置不能被当成 always 误发。
	cf := yyQQConfig{}
	cf.Assistant.VoiceMode = "every"
	if yyQQVoiceMode(cf) != "" {
		t.Fatal("脏 voice_mode 不应生效")
	}
}

func TestVoiceSkillRegistered(t *testing.T) {
	var found *yyQQSkillDef
	for i := range yyQQSkillDefs {
		if yyQQSkillDefs[i].ID == "voice" {
			found = &yyQQSkillDefs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("技能注册表缺少 voice")
	}
	if found.Word != "语音" {
		t.Fatalf("voice 默认功能词 = %q，期望 语音", found.Word)
	}
	if found.Tool != "send_voice" {
		t.Fatalf("voice 对应工具 = %q，期望 send_voice", found.Tool)
	}
	if len(found.Cap) != 1 || found.Cap[0] != "voice" {
		t.Fatalf("voice 技能应绑定 voice 能力，got %v", found.Cap)
	}
	if yyQQSkillByWord(yyQQConfig{}, "语音") == nil {
		t.Fatal("按功能词「语音」找不到技能")
	}
}

func TestVoiceCapRegistered(t *testing.T) {
	rows := yyQQCapsView(yyQQAssistantConfig{})
	var row map[string]any
	for _, r := range rows {
		if r["kind"] == "voice" {
			row = r
			break
		}
	}
	if row == nil {
		t.Fatal("能力视图缺少 voice")
	}
	if row["default"] != true {
		t.Fatalf("voice 应默认开启，got %v", row["default"])
	}
	if row["onebot_only"] != true {
		t.Fatalf("voice 应只支持 OneBot，got %v", row["onebot_only"])
	}
	if row["enabled"] != true {
		t.Fatalf("未改配置时 voice 应 enabled=true，got %v", row["enabled"])
	}
	cmd, _ := row["command"].(string)
	if !strings.Contains(cmd, "语音") {
		t.Fatalf("能力指令展示应含「语音」，got %q", cmd)
	}
}

func TestVoiceCommandAlias(t *testing.T) {
	cf := yyQQConfig{}
	if got := yyQQCommandText(cf, "voice"); got != ".语音" {
		t.Fatalf("默认语音指令 = %q，期望 .语音", got)
	}
	cf.Assistant.CommandAliases = map[string]string{"voice": ".说话"}
	if got := yyQQCommandText(cf, "voice"); got != ".说话" {
		t.Fatalf("别名未生效，got %q", got)
	}
	// 空白别名回落到默认值。
	cf.Assistant.CommandAliases["voice"] = "   "
	if got := yyQQCommandText(cf, "voice"); got != ".语音" {
		t.Fatalf("空白别名应回默认，got %q", got)
	}
}

func TestVoiceTTSBase64Prefix(t *testing.T) {
	var gotPath, gotAuth string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != "POST" {
			t.Errorf("方法 = %s，期望 POST", r.Method)
		}
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("ID3"))
	}))
	defer srv.Close()

	name := "voice-tts-test"
	if e := aiStore.Add(&AIModel{
		Name: name, Provider: ProviderOpenAI,
		BaseURL: srv.URL + "/v1", APIKey: "sk-test", ModelID: "tts-1",
	}); e != nil {
		t.Fatalf("准备模型失败：%v", e)
	}
	t.Cleanup(func() { _ = aiStore.Delete(name) })

	cf := yyQQConfig{}
	cf.Assistant.VoiceModel = name
	out, e := yyVoiceTTS(cf, "你好呀")
	if e != nil {
		t.Fatalf("TTS 失败：%v", e)
	}
	if !strings.HasPrefix(out, "base64://") {
		t.Fatalf("二进制音频应转成 base64://，got %q", out)
	}
	b64 := strings.TrimPrefix(out, "base64://")
	rawBytes, e := base64.StdEncoding.DecodeString(b64)
	if e != nil {
		t.Fatalf("base64 解码失败：%v", e)
	}
	if string(rawBytes) != "ID3" {
		t.Fatalf("音频内容 = %q，期望 ID3", rawBytes)
	}
	if gotPath != "/v1/audio/speech" {
		t.Fatalf("请求路径 = %q，期望 /v1/audio/speech", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if body["response_format"] != "mp3" {
		t.Fatalf("response_format = %v，期望 mp3", body["response_format"])
	}
	if body["voice"] != "alloy" {
		t.Fatalf("默认音色 = %v，期望 alloy", body["voice"])
	}
	if body["input"] != "你好呀" {
		t.Fatalf("input = %v", body["input"])
	}
}

func TestVoiceTTSUnconfigured(t *testing.T) {
	cf := yyQQConfig{}
	if _, e := yyVoiceTTS(cf, "测试"); e == nil {
		t.Fatal("没配语音模型时应报错，而不是发请求")
	} else if !strings.Contains(e.Error(), "语音模型") {
		t.Fatalf("错误信息应可操作，got %q", e.Error())
	}
	// URL 直发不需要 TTS 模型。
	file, clipped, e := yyQQVoicePrepare(cf, "https://example.com/a.mp3")
	if e != nil || clipped || file == "" {
		t.Fatalf("URL 直发不应依赖语音模型：file=%q clipped=%v err=%v", file, clipped, e)
	}
}
