package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// QQ 机器人「发语音」能力。
//
// 用户原话（第 22 条）：「别人这个AI聊天模型搭配onebot可以发语音 这个我们肯定也要弄一个能力」。
//
// 三条路径共用同一份发送实现：
//  1. 群指令 `.语音 文本或音频URL`；
//  2. AI 工具 send_voice（模型自己决定什么时候开口）；
//  3. 技能「技能[语音] 文本」，以及 voice_mode=always 时每条文本回复额外补一条语音。
//
// 发送格式与图片/表情一致：send_group_msg + message 段数组，段类型 record，
// 地址放在 data.file。NapCat 认 http/https 音频直链，也认 base64://<base64>，
// 所以 TTS 合成出来的二进制统一转成 base64:// 再交给 OneBot，不依赖外链是否可达。

const (
	// yyVoiceTextMax TTS 单次文本上限（字）。超了截断并明确告知，不静默吞掉。
	yyVoiceTextMax = 300
	// yyVoiceAudioMax 单次合成的音频上限，防止供应商返回异常大文件把内存撑爆。
	yyVoiceAudioMax = 8 << 20
)

var (
	// 变量别名，只为测试能打桩；逻辑上仍是 OneBot 调用。
	yyVoiceCall = yyQQOneBotCall
	// 语音文本里残留的技能段 / CQ 码不该被念出来。
	yyVoiceSkillRe = regexp.MustCompile(`(?:使用)?\s*技能\s*[\[［][^\]］\n]{1,20}[\]］][^\n，。；;]*`)
	yyVoiceCQRe    = regexp.MustCompile(`\[CQ:[^\]]+\]`)
)

// yyVoiceSend 往群里发一条语音（OneBot record 段）。
func yyVoiceSend(groupID, file string) error {
	file = strings.TrimSpace(file)
	if file == "" {
		return fmt.Errorf("缺少语音内容")
	}
	if !yyVoiceURL(file) {
		return fmt.Errorf("语音只支持 http/https 音频直链或 base64:// 数据")
	}
	g, e := strconv.ParseInt(strings.TrimSpace(yyQQGroupString(groupID)), 10, 64)
	if e != nil {
		return fmt.Errorf("OneBot 群号无效")
	}
	msg := []map[string]any{{
		"type": "record",
		"data": map[string]any{"file": file},
	}}
	_, e = yyVoiceCall("send_group_msg", map[string]any{"group_id": g, "message": msg})
	return e
}

// yyVoiceURL 只认 http/https 或 base64:// 前缀，其余（file://、data:、javascript:）一律拒绝。
func yyVoiceURL(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "base64://")
}

// yyQQVoiceModeNormalize 语音模式归一化：只允许 off/auto/always，其余（含空）归一化成空串。
// 空串与 off 语义相同（不发语音），但面板要能区分「没设过」和「明确关闭」。
func yyQQVoiceModeNormalize(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "off":
		return "off"
	case "auto":
		return "auto"
	case "always":
		return "always"
	}
	return ""
}

// yyQQVoiceMode 取生效语音模式（脏数据按空处理，不 panic、不误发）。
func yyQQVoiceMode(cf yyQQConfig) string {
	return yyQQVoiceModeNormalize(cf.Assistant.VoiceMode)
}

// yyQQVoiceName 音色：留空回落到 alloy（OpenAI 兼容接口的默认音色）。
func yyQQVoiceName(cf yyQQConfig) string {
	if v := strings.TrimSpace(cf.Assistant.VoiceVoice); v != "" {
		return v
	}
	return "alloy"
}

// yyVoiceClip 截断到 yyVoiceTextMax 字；第二个返回值表示是否真的截过。
func yyVoiceClip(text string) (string, bool) {
	r := []rune(strings.TrimSpace(text))
	if len(r) <= yyVoiceTextMax {
		return string(r), false
	}
	return string(r[:yyVoiceTextMax]), true
}

// yyVoicePlain 把一段回复洗成适合朗读的纯文本：去掉残留技能段、CQ 码、换行。
// always 模式下 AI 的回复正文里可能还有 markdown/CQ 码，念出来会很怪。
func yyVoicePlain(text string) string {
	s := yyVoiceSkillRe.ReplaceAllString(text, "")
	s = yyVoiceCQRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "```", " ")
	s = strings.Join(strings.Fields(s), " ")
	s, _ = yyVoiceClip(s)
	return strings.TrimSpace(s)
}

// yyVoiceTTS 用配置的 TTS 接口把文本合成音频，返回 base64:// 或可播放 URL。
//
// 约定：OpenAI 兼容 POST <base_url>/audio/speech，
// body {model, input, voice, response_format:"mp3"}。
// 返回二进制 → base64://<b64>（NapCat 支持 base64:// 发送）；
// 若供应商返回 JSON（如 {"url":"https://..."}）则直接取 url。
//
// 未配置语音模型时不做任何网络请求，回一句可操作的错误。
func yyVoiceTTS(cf yyQQConfig, text string) (string, error) {
	text, truncated := yyVoiceClip(text)
	if text == "" {
		return "", fmt.Errorf("要念的内容是空的")
	}
	if truncated {
		yyQQLog("语音文本超过 " + strconv.Itoa(yyVoiceTextMax) + " 字，已截断后合成")
	}
	name := strings.TrimSpace(cf.Assistant.VoiceModel)
	if name == "" {
		return "", fmt.Errorf("还没配置语音模型：先在 QQ 控制台·人设里选语音模型")
	}
	model := aiStore.Get(name)
	if model == nil {
		return "", fmt.Errorf("语音模型「%s」不在模型库里：去 QQ 控制台·人设里重新选一个", name)
	}
	base := strings.TrimRight(strings.TrimSpace(model.BaseURL), "/")
	if base == "" {
		return "", fmt.Errorf("语音模型的请求地址为空，去模型配置里补上")
	}
	payload, _ := json.Marshal(map[string]any{
		"model":           model.ModelID,
		"input":           text,
		"voice":           yyQQVoiceName(cf),
		"response_format": "mp3",
	})
	req, e := http.NewRequest("POST", base+"/audio/speech", bytes.NewReader(payload))
	if e != nil {
		return "", fmt.Errorf("语音请求构造失败：%w", e)
	}
	req.Header.Set("Content-Type", "application/json")
	if model.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+model.APIKey)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, e := client.Do(req)
	if e != nil {
		return "", fmt.Errorf("语音接口连接失败：%w", e)
	}
	defer resp.Body.Close()
	body, e := io.ReadAll(io.LimitReader(resp.Body, yyVoiceAudioMax))
	if e != nil {
		return "", fmt.Errorf("读取语音响应失败：%w", e)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("语音接口返回 %d：%s", resp.StatusCode, yyQQCardClip(string(body), 160))
	}
	// JSON = 返回地址型供应商（没有二进制音频）。
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var out struct {
			URL   string `json:"url"`
			Audio string `json:"audio"`
			Error any    `json:"error"`
		}
		if e := json.Unmarshal(trimmed, &out); e == nil {
			if u := strings.TrimSpace(out.URL); u != "" {
				if !yyVoiceURL(u) {
					return "", fmt.Errorf("语音接口返回的地址不可播放")
				}
				return u, nil
			}
			if a := strings.TrimSpace(out.Audio); a != "" {
				return "base64://" + strings.TrimPrefix(a, "base64://"), nil
			}
			if out.Error != nil {
				return "", fmt.Errorf("语音接口报错：%s", yyQQCardClip(fmt.Sprint(out.Error), 160))
			}
		}
		return "", fmt.Errorf("语音接口返回了无法识别的 JSON")
	}
	if len(body) == 0 {
		return "", fmt.Errorf("语音接口没有返回音频")
	}
	return "base64://" + base64.StdEncoding.EncodeToString(body), nil
}

// yyQQVoicePrepare 把「文本或音频URL」统一成可直接发送的 file。
// clipped=true 表示文本超长被截断（调用方负责告诉用户）。
// 未配置语音模型时直接在本地报错，不会发任何网络请求。
func yyQQVoicePrepare(cf yyQQConfig, raw string) (file string, clipped bool, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, fmt.Errorf("要念的内容是空的")
	}
	if yyVoiceURL(raw) {
		if len(raw) > 2000 {
			return "", false, fmt.Errorf("音频链接太长（最多 2000 字符），换一个短链接")
		}
		return raw, false, nil
	}
	if aiStore.Get(cf.Assistant.VoiceModel) == nil {
		return "", false, fmt.Errorf("还没配置语音模型：先在 QQ 控制台·人设里选语音模型")
	}
	text, truncated := yyVoiceClip(raw)
	file, err = yyVoiceTTS(cf, text)
	return file, truncated, err
}

// yyQQCmdVoice .语音 文本或音频URL —— 受能力开关与限流约束。
func yyQQCmdVoice(cf yyQQConfig, user, name, arg string, level int) []string {
	reply := func(s string) []string { return []string{s} }
	raw := strings.TrimSpace(arg)
	if raw == "" {
		return reply("格式：" + yyQQCommandText(cf, "voice") + " 要念的话，或音频URL（http/https）")
	}
	group := yyQQGroupString(cf.GroupID)
	if group == "" {
		return reply("还没选群，无法发语音。")
	}
	// URL 直发只要能力开着就行，不要求配 TTS 模型；文本才需要模型。
	if !yyVoiceURL(raw) && aiStore.Get(cf.Assistant.VoiceModel) == nil {
		return reply("还没配置语音模型：先在 QQ 控制台·人设里选语音模型。")
	}
	target := yyQQCardClip(raw, 60)
	if yyVoiceURL(raw) {
		target = "音频URL " + yyQQCardClip(raw, 50)
	}
	clipped := false
	e := yyQQRunAction("voice", user, name, target, target, level, func() (string, error) {
		file, cut, err := yyQQVoicePrepare(cf, raw)
		clipped = cut
		if err != nil {
			return "", err
		}
		return "", yyVoiceSend(group, file)
	})
	if e != nil {
		return reply("发语音失败：" + e.Error())
	}
	if clipped {
		return reply("文本超过 " + strconv.Itoa(yyVoiceTextMax) + " 字，已截取前 " + strconv.Itoa(yyVoiceTextMax) + " 字合成语音。")
	}
	return reply("语音已发出。")
}

// yyQQVoiceAlwaysSend voice_mode=always：文本回复照发，另外异步补一条语音。
// 失败只记日志，绝不影响文本发送（语音是锦上添花，不是主通道）。
func yyQQVoiceAlwaysSend(cf yyQQConfig, answer string) {
	if yyQQVoiceMode(cf) != "always" {
		return
	}
	// 面板上把「发语音」关掉就真的不发，always 也不例外。
	if !yyQQCapEnabled(cf.Assistant, "voice") {
		return
	}
	if cf.Adapter != "onebot" {
		// 官方适配器没有 record 段，静默跳过，免得每个文本都写一条失败日志。
		return
	}
	text := yyVoicePlain(answer)
	if text == "" {
		return
	}
	group := yyQQGroupString(cf.GroupID)
	if group == "" {
		return
	}
	go func() {
		file, e := yyVoiceTTS(cf, text)
		if e != nil {
			yyQQLog("语音回复未合成：" + e.Error())
			return
		}
		if e := yyVoiceSend(group, file); e != nil {
			yyQQLog("语音回复发送失败：" + e.Error())
			return
		}
		yyQQLog("语音回复已发送（always 模式）")
	}()
}
