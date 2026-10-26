package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 生图子系统。
//
// 需求来自实际使用：群里有人发「画个 xxx」，机器人要能真的出图发回来，
// 而且最好是拿提问者本人的头像当参考，出来的东西才像"给这个人画的"。
// 因此这里不写死任何一家服务：provider / model / base_url / key 全部可配，
// 用户在面板里换模型不用等发版。

type yyQQImageConfig struct {
	Enabled      bool     `json:"enabled"`
	Provider     string   `json:"provider"` // openai | cloudflare
	Model        string   `json:"model"`
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	AccountID    string   `json:"account_id,omitempty"`
	Size         string   `json:"size"`
	Steps        int      `json:"steps,omitempty"`
	UseAvatar    bool     `json:"use_avatar"`
	PromptPrefix string   `json:"prompt_prefix,omitempty"`
	PromptSuffix string   `json:"prompt_suffix,omitempty"`
	ReplyOnStart string   `json:"reply_on_start,omitempty"`
	Models       []string `json:"models,omitempty"`
}

var (
	yyQQImageMu   sync.Mutex
	yyQQImageCfg  yyQQImageConfig
	yyQQImagePath string
	yyQQImageHTTP = &http.Client{Timeout: 150 * time.Second}
)

func yyQQImageSnapshot() yyQQImageConfig {
	yyQQImageMu.Lock()
	defer yyQQImageMu.Unlock()
	return yyQQImageCfg
}

func yyQQImageSave(c yyQQImageConfig) error {
	yyQQImageMu.Lock()
	yyQQImageCfg = c
	path := yyQQImagePath
	yyQQImageMu.Unlock()
	if path == "" {
		return fmt.Errorf("生图配置未初始化")
	}
	raw, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, raw, 0o600)
}

func yyQQImageLoad(dataDir string) {
	yyQQImagePath = filepath.Join(dataDir, "qq-image.json")
	raw, e := os.ReadFile(yyQQImagePath)
	if e == nil {
		_ = json.Unmarshal(raw, &yyQQImageCfg)
	}
	if yyQQImageCfg.Provider == "" {
		yyQQImageCfg.Provider = "openai"
	}
	if yyQQImageCfg.Size == "" {
		yyQQImageCfg.Size = "1024x1024"
	}
	if yyQQImageCfg.Models == nil {
		yyQQImageCfg.Models = []string{"Kwai-Kolors/Kolors", "black-forest-labs/FLUX.1-schnell", "cogview-3-flash", "gpt-image-1"}
	}
}

// yyQQImageEndpoint 归一化 OpenAI 兼容接口地址。
// 用户填 https://api.openai.com 时要补 /v1；填 https://api.siliconflow.cn/v1 时不能再补。
func yyQQImageEndpoint(base, suffix string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	low := strings.ToLower(b)
	for _, tail := range []string{"/v1", "/v2", "/v3", "/v4", "/v1beta", "/api/v1"} {
		if strings.HasSuffix(low, tail) {
			return b + suffix
		}
	}
	return b + "/v1" + suffix
}

// yyQQAvatarURL 取玩家 QQ 头像。s=640 足够做参考图。
func yyQQAvatarURL(uin string) string {
	if uin == "" {
		return ""
	}
	return "https://q1.qlogo.cn/g?b=qq&nk=" + uin + "&s=640"
}

func yyQQImageFetch(url string) ([]byte, string, error) {
	req, e := http.NewRequest("GET", url, nil)
	if e != nil {
		return nil, "", e
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, e := yyQQImageHTTP.Do(req)
	if e != nil {
		return nil, "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
	if e != nil {
		return nil, "", e
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	return raw, ct, nil
}

// yyQQImageGenerate 调外部服务出图，返回图片字节与 MIME。
func yyQQImageGenerate(c yyQQImageConfig, prompt string, avatar []byte) ([]byte, string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, "", fmt.Errorf("还没配置生图密钥，先去面板「生图」里填")
	}
	switch c.Provider {
	case "cloudflare":
		if c.AccountID == "" {
			return nil, "", fmt.Errorf("Cloudflare 生图需要 account_id")
		}
		model := c.Model
		if model == "" {
			model = "@cf/black-forest-labs/flux-1-schnell"
		}
		if !strings.HasPrefix(model, "@cf/") {
			model = "@cf/" + model
		}
		body, _ := json.Marshal(map[string]any{"prompt": prompt, "steps": yyQQImageSteps(c)})
		url := "https://api.cloudflare.com/client/v4/accounts/" + c.AccountID + "/ai/run/" + model
		req, e := http.NewRequest("POST", url, bytes.NewReader(body))
		if e != nil {
			return nil, "", e
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		resp, e := yyQQImageHTTP.Do(req)
		if e != nil {
			return nil, "", e
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 24<<20))
		if resp.StatusCode != 200 {
			return nil, "", fmt.Errorf("生图服务返回 %d：%s", resp.StatusCode, yyQQShort(string(raw), 160))
		}
		// Workers AI 的图片模型直接返回二进制；文本模型会返回 JSON，这里做兜底。
		if len(raw) > 8 && (string(raw[:8]) == "\x89PNG\r\n\x1a\n" || string(raw[:3]) == "\xff\xd8\xff") {
			return raw, "image/png", nil
		}
		var wrap struct {
			Result struct {
				Image string `json:"image"`
			} `json:"result"`
		}
		if json.Unmarshal(raw, &wrap) == nil && wrap.Result.Image != "" {
			if dec, e := base64.StdEncoding.DecodeString(wrap.Result.Image); e == nil {
				return dec, "image/png", nil
			}
		}
		return nil, "", fmt.Errorf("生图返回内容无法识别")
	default: // openai 兼容：硅基流动 / 智谱 / OpenAI / 自建
		base := strings.TrimRight(c.BaseURL, "/")
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		model := c.Model
		if model == "" {
			model = "gpt-image-1"
		}
		// 有头像参考图时优先走 images/edits（multipart），失败再退回纯文生图。
		if len(avatar) > 0 {
			if out, mime, e := yyQQImageEdit(c, base, model, prompt, avatar); e == nil {
				return out, mime, nil
			}
		}
		payload := map[string]any{"model": model, "prompt": prompt, "n": 1, "size": yyQQImageSize(c)}
		body, _ := json.Marshal(payload)
		req, e := http.NewRequest("POST", yyQQImageEndpoint(base, "/images/generations"), bytes.NewReader(body))
		if e != nil {
			return nil, "", e
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		resp, e := yyQQImageHTTP.Do(req)
		if e != nil {
			return nil, "", e
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 24<<20))
		if resp.StatusCode != 200 {
			return nil, "", fmt.Errorf("生图服务返回 %d：%s", resp.StatusCode, yyQQShort(string(raw), 160))
		}
		var parsed struct {
			Data []struct {
				B64 string `json:"b64_json"`
				URL string `json:"url"`
			} `json:"data"`
		}
		if e := json.Unmarshal(raw, &parsed); e != nil || len(parsed.Data) == 0 {
			return nil, "", fmt.Errorf("生图返回格式无法识别")
		}
		if parsed.Data[0].B64 != "" {
			dec, e := base64.StdEncoding.DecodeString(parsed.Data[0].B64)
			if e != nil {
				return nil, "", e
			}
			return dec, "image/png", nil
		}
		if parsed.Data[0].URL != "" {
			return yyQQImageFetch(parsed.Data[0].URL)
		}
		return nil, "", fmt.Errorf("生图没有返回图片")
	}
}

func yyQQImageEdit(c yyQQImageConfig, base, model, prompt string, avatar []byte) ([]byte, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("model", model)
	_ = mw.WriteField("prompt", prompt)
	_ = mw.WriteField("size", yyQQImageSize(c))
	part, e := mw.CreateFormFile("image", "avatar.png")
	if e != nil {
		return nil, "", e
	}
	if _, e = part.Write(avatar); e != nil {
		return nil, "", e
	}
	_ = mw.Close()
	req, e := http.NewRequest("POST", yyQQImageEndpoint(base, "/images/edits"), &buf)
	if e != nil {
		return nil, "", e
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, e := yyQQImageHTTP.Do(req)
	if e != nil {
		return nil, "", e
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 24<<20))
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("edits %d", resp.StatusCode)
	}
	var parsed struct {
		Data []struct {
			B64 string `json:"b64_json"`
			URL string `json:"url"`
		} `json:"data"`
	}
	if e := json.Unmarshal(raw, &parsed); e != nil || len(parsed.Data) == 0 {
		return nil, "", fmt.Errorf("edits 格式无法识别")
	}
	if parsed.Data[0].B64 != "" {
		dec, e := base64.StdEncoding.DecodeString(parsed.Data[0].B64)
		if e != nil {
			return nil, "", e
		}
		return dec, "image/png", nil
	}
	if parsed.Data[0].URL != "" {
		return yyQQImageFetch(parsed.Data[0].URL)
	}
	return nil, "", fmt.Errorf("edits 没有返回图片")
}

func yyQQImageSize(c yyQQImageConfig) string {
	if c.Size == "" {
		return "1024x1024"
	}
	return c.Size
}

func yyQQImageSteps(c yyQQImageConfig) int {
	if c.Steps > 0 && c.Steps <= 8 {
		return c.Steps
	}
	return 4
}

func yyQQShort(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// yyQQImageSend 把图片以 base64 直接发进群，不依赖外链有效期。
func yyQQImageSend(group string, img []byte, replyID string) error {
	g, e := strconv.ParseInt(yyQQGroupString(group), 10, 64)
	if e != nil {
		return fmt.Errorf("群号无效")
	}
	msg := []map[string]any{{
		"type": "image",
		"data": map[string]any{"file": "base64://" + base64.StdEncoding.EncodeToString(img)},
	}}
	params := map[string]any{"group_id": g, "message": msg}
	if replyID != "" {
		// OneBot v11 的 reply 段
		msg = append([]map[string]any{{"type": "reply", "data": map[string]any{"id": replyID}}}, msg...)
		params["message"] = msg
	}
	_, e = yyQQOneBotCall("send_group_msg", params)
	return e
}

// yyQQImageSendAs 以「提出要求的那个人」的名义发图。
//
// OneBot/官方接口都不允许真的冒充别人发言，所以「署名」落在消息内容上：
// 图片前是一条 @他 + 「要的图」的说明段，群里看到的是「这张图是 X 要的」，
// 而不是机器人自己在那儿说「我画好了」。这既是用户要的「以发图者的身份」，
// 也守住了「机器人不能冒充群成员」的底线（文案里不出现第一人称）。
func yyQQImageSendAs(cf yyQQConfig, img []byte, replyID, qq, name string) error {
	if len(img) == 0 {
		return fmt.Errorf("图片为空")
	}
	// 官方适配器只能发"纯图片"消息，拼不出 @ 段：退回原来的发图路径（没有署名，
	// 但至少发得出去）。署名能力目前只在 OneBot/NapCat 上生效。
	if cf.Adapter != "onebot" {
		return yyQQSendImage(cf, img, replyID)
	}
	g, e := strconv.ParseInt(strings.TrimSpace(yyQQGroupString(cf.GroupID)), 10, 64)
	if e != nil {
		return fmt.Errorf("群号无效")
	}
	who := strings.TrimSpace(name)
	if who == "" {
		who = strings.TrimSpace(qq)
	}
	if who == "" {
		who = "群友"
	}
	msg := make([]map[string]any, 0, 4)
	if replyID != "" {
		msg = append(msg, map[string]any{"type": "reply", "data": map[string]any{"id": replyID}})
	}
	// 数字 QQ 才能 @；官方适配器的 member_openid 拼不出 at 段，退化成纯文字署名。
	if yyQQNumericQQ(strings.TrimSpace(qq)) {
		msg = append(msg,
			map[string]any{"type": "at", "data": map[string]any{"qq": strings.TrimSpace(qq)}},
			map[string]any{"type": "text", "data": map[string]any{"text": " 要的图"}})
	} else {
		msg = append(msg, map[string]any{"type": "text", "data": map[string]any{"text": who + " 要的图"}})
	}
	msg = append(msg, map[string]any{"type": "image", "data": map[string]any{"file": "base64://" + base64.StdEncoding.EncodeToString(img)}})
	_, e = yyQQOneBotCall("send_group_msg", map[string]any{"group_id": g, "message": msg})
	return e
}

// yyQQDrawRate 画图比聊天贵得多，成员随手刷能把额度刷穿。按人限速：
// 20 秒一次、每人每天 20 张，超了直接告诉模型「别再画了」。
var yyQQDrawRate = struct {
	sync.Mutex
	Last map[string]time.Time
	Day  string
	Hits map[string]int
}{Last: map[string]time.Time{}, Hits: map[string]int{}}

const (
	yyQQDrawGap   = 20 * time.Second
	yyQQDrawDaily = 20
)

func yyQQDrawRateTake(user string) error {
	if strings.TrimSpace(user) == "" {
		user = "anonymous"
	}
	now := time.Now()
	day := now.Format("2006-01-02")
	yyQQDrawRate.Lock()
	defer yyQQDrawRate.Unlock()
	if yyQQDrawRate.Day != day {
		yyQQDrawRate.Day = day
		yyQQDrawRate.Hits = map[string]int{}
		yyQQDrawRate.Last = map[string]time.Time{}
	}
	if at, ok := yyQQDrawRate.Last[user]; ok && now.Sub(at) < yyQQDrawGap {
		return fmt.Errorf("画图太快了，%d 秒后再试", int(yyQQDrawGap.Seconds()-now.Sub(at).Seconds())+1)
	}
	if yyQQDrawRate.Hits[user] >= yyQQDrawDaily {
		return fmt.Errorf("今天的画图次数用完了（%d 张/天），明天再来", yyQQDrawDaily)
	}
	yyQQDrawRate.Last[user] = now
	yyQQDrawRate.Hits[user]++
	return nil
}

// yyQQImageRun 是触发词引擎里 `image` 动作的落点，异步执行不阻塞消息循环。
func yyQQImageRun(cf yyQQConfig, userID, name, msgID, prompt string, params map[string]string) {
	c := yyQQImageSnapshot()
	if !c.Enabled {
		return
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return
	}
	useAvatar := c.UseAvatar
	if v, ok := params["use_avatar"]; ok {
		useAvatar = v == "true" || v == "1"
	}
	full := strings.TrimSpace(c.PromptPrefix + " " + prompt + " " + c.PromptSuffix)

	if msg := strings.TrimSpace(c.ReplyOnStart); msg != "" {
		_ = yyQQSend(strings.NewReplacer("{user}", name, "{qq}", userID, "{arg}", prompt).Replace(msg), msgID)
	}

	var avatar []byte
	if useAvatar {
		if url := yyQQAvatarURL(userID); url != "" {
			if raw, _, e := yyQQImageFetch(url); e == nil && len(raw) > 0 {
				avatar = raw
			}
		}
	}
	start := time.Now()
	img, _, e := yyQQImageGenerate(c, full, avatar)
	if e != nil {
		yyQQLog("生图失败：" + e.Error())
		_ = yyQQSend("图没画出来："+e.Error(), msgID)
		return
	}
	// 署名归提出要求的人：图片消息里 @ 他 + 「要的图」。
	if e := yyQQImageSendAs(cf, img, msgID, userID, name); e != nil {
		yyQQLog("生图发送失败：" + e.Error())
		return
	}
	yyQQLog(fmt.Sprintf("生图完成：%s 用了 %s，%d KB", name, time.Since(start).Round(time.Second), len(img)/1024))
}

func yyQQImageRoutes(mux *http.ServeMux, dataDir string) {
	yyQQImageLoad(dataDir)
	mux.HandleFunc("/api/qq/image", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			c := yyQQImageSnapshot()
			// 密钥只回显是否已配置，不回显明文
			out := map[string]any{
				"enabled": c.Enabled, "provider": c.Provider, "model": c.Model,
				"base_url": c.BaseURL, "account_id": c.AccountID, "size": c.Size,
				"steps": c.Steps, "use_avatar": c.UseAvatar,
				"prompt_prefix": c.PromptPrefix, "prompt_suffix": c.PromptSuffix,
				"reply_on_start": c.ReplyOnStart, "models": c.Models,
				"has_key": strings.TrimSpace(c.APIKey) != "",
			}
			writeJSON(w, map[string]any{"ok": true, "config": out,
				"providers": []map[string]any{
					{"id": "openai", "label": "OpenAI 兼容（硅基流动 / 智谱 / 自建）", "need": []string{"base_url", "api_key", "model"}},
					{"id": "cloudflare", "label": "Cloudflare Workers AI", "need": []string{"account_id", "api_key", "model"}},
				}})
		case http.MethodPost:
			var patch map[string]any
			if e := json.NewDecoder(r.Body).Decode(&patch); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			c := yyQQImageSnapshot()
			for k, v := range patch {
				switch k {
				case "enabled":
					c.Enabled = v == true
				case "provider":
					c.Provider = yyQQStr(v)
				case "model":
					c.Model = yyQQStr(v)
				case "base_url":
					c.BaseURL = yyQQStr(v)
				case "account_id":
					c.AccountID = yyQQStr(v)
				case "size":
					c.Size = yyQQStr(v)
				case "prompt_prefix":
					c.PromptPrefix = yyQQStr(v)
				case "prompt_suffix":
					c.PromptSuffix = yyQQStr(v)
				case "reply_on_start":
					c.ReplyOnStart = yyQQStr(v)
				case "api_key":
					// 空字符串表示"不修改"，避免前端回显空值把 key 清掉
					if s := yyQQStr(v); s != "" {
						c.APIKey = s
					}
				case "steps":
					if n, e := strconv.Atoi(yyQQStr(v)); e == nil {
						c.Steps = n
					}
				case "use_avatar":
					c.UseAvatar = v == true
				}
			}
			if e := yyQQImageSave(c); e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSON(w, map[string]any{"ok": false, "error": "method"})
		}
	})
}

func yyQQStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// yyQQGenerateImage 复用「AI 模型」里已配置的那条记录去生图。
// 用户在「模型」里填好 BaseURL / APIKey / ModelID，把 ModelID 填成生图模型
// （如 Kwai-Kolors/Kolors、cogview-3-flash、gpt-image-1、flux-1-schnell），
// 然后在群服互通里把「生图模型」指向它即可，不需要再单独维护一套密钥。
func yyQQGenerateImage(ctx context.Context, model *AIModel, prompt string) ([]byte, error) {
	if model == nil {
		return nil, fmt.Errorf("还没有选择生图模型，先去「群服互通 · 生图」里挑一个")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, fmt.Errorf("缺少画面描述")
	}
	base := strings.TrimRight(strings.TrimSpace(model.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("模型 %s 缺少 BaseURL", model.Name)
	}
	// 生图比对话慢得多，这里给独立的超时，不受对话超时影响。
	runCtx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	if strings.Contains(base, "api.cloudflare.com") || strings.Contains(model.ModelID, "@cf/") {
		id := model.ModelID
		if !strings.HasPrefix(id, "@cf/") {
			id = "@cf/" + id
		}
		body, _ := json.Marshal(map[string]any{"prompt": prompt, "steps": 4})
		req, e := http.NewRequestWithContext(runCtx, "POST", base+"/"+id, bytes.NewReader(body))
		if e != nil {
			return nil, e
		}
		req.Header.Set("Authorization", "Bearer "+model.APIKey)
		req.Header.Set("Content-Type", "application/json")
		resp, e := yyQQImageHTTP.Do(req)
		if e != nil {
			return nil, e
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 24<<20))
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("生图服务返回 %d：%s", resp.StatusCode, yyQQShort(string(raw), 160))
		}
		if len(raw) > 8 && (bytes.HasPrefix(raw, []byte{0x89, 0x50, 0x4E, 0x47}) || bytes.HasPrefix(raw, []byte{0xFF, 0xD8, 0xFF})) {
			return raw, nil
		}
		return nil, fmt.Errorf("生图返回内容无法识别")
	}

	payload := map[string]any{"model": model.ModelID, "prompt": prompt, "n": 1, "size": "1024x1024"}
	body, _ := json.Marshal(payload)
	req, e := http.NewRequestWithContext(runCtx, "POST", yyQQImageEndpoint(base, "/images/generations"), bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+model.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, e := yyQQImageHTTP.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 24<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("生图服务返回 %d：%s", resp.StatusCode, yyQQShort(string(raw), 160))
	}
	var parsed struct {
		Data []struct {
			B64 string `json:"b64_json"`
			URL string `json:"url"`
		} `json:"data"`
	}
	if e := json.Unmarshal(raw, &parsed); e != nil || len(parsed.Data) == 0 {
		return nil, fmt.Errorf("生图返回格式无法识别")
	}
	if parsed.Data[0].B64 != "" {
		return base64.StdEncoding.DecodeString(parsed.Data[0].B64)
	}
	if parsed.Data[0].URL != "" {
		out, _, e := yyQQImageFetch(parsed.Data[0].URL)
		return out, e
	}
	return nil, fmt.Errorf("生图没有返回图片")
}
