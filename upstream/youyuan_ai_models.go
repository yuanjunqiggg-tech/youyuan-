package main

// 统一模型配置（语言 / 画图 / 语音）。
//
// 背景：模型配置以前散在三处——
//   1) 模型库 ai_models.json：地址、密钥、模型 ID（唯一一份凭证）；
//   2) 群服互通 qq-bridge.json 的 assistant.model / image_model / voice_model：
//      三个用途各自指向模型库里的哪一条；
//   3) 生图 qq-image.json：另一套地址 + 密钥，和模型库重复。
// 用户的要求是「模型配置就搞一个统一的，包括语言模型、画图模型以及语音模型」。
//
// 做法：用途选择收进 ai_models.json 的 roles 段，模型库仍是唯一一份凭证。
// 保存后写穿（write-through）到两个老消费者：
//   - qq-bridge.json  assistant.model / image_model / voice_model / voice_voice / voice_mode
//   - qq-image.json   provider / model / base_url / api_key / size
// 这样群服互通、生图队列、AI 工具都不用改代码就能读到统一配置，
// 也不会出现「设置页改了、实际生图还用旧密钥」的假统一。

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// 三个用途的键名，前后端共用同一套字符串。
const (
	yyAIRoleLanguage = "language"
	yyAIRoleImage    = "image"
	yyAIRoleSpeech   = "speech"
)

// yyAIRoleValid 判断用途键是否合法。
func yyAIRoleValid(role string) bool {
	return role == yyAIRoleLanguage || role == yyAIRoleImage || role == yyAIRoleSpeech
}

// yyAIRolesView 返回给前端的用途选择（含解析后的模型 ID，方便直接显示）。
// 只暴露模型名与模型 ID，密钥一律不出这个函数。
func yyAIRolesView() map[string]any {
	roles := aiStore.Roles()
	out := map[string]any{}
	for _, role := range []string{yyAIRoleLanguage, yyAIRoleImage, yyAIRoleSpeech} {
		var r AIModelRole
		switch role {
		case yyAIRoleLanguage:
			r = roles.Language
		case yyAIRoleImage:
			r = roles.Image
		case yyAIRoleSpeech:
			r = roles.Speech
		}
		item := map[string]any{"model": r.Model, "size": r.Size, "voice": r.Voice, "mode": r.Mode}
		if m := aiStore.Get(r.Model); m != nil {
			item["model_id"] = m.ModelID
			item["provider"] = m.Provider
			item["base_url"] = m.BaseURL
			item["ready"] = true
		} else {
			item["ready"] = false
			if r.Model != "" {
				item["warning"] = "模型库里找不到「" + r.Model + "」，请重新选择"
			}
		}
		out[role] = item
	}
	return out
}

// yyAISeedRolesFromQQ 老用户迁移：roles 还是空的，就把群服互通里已经选过的模型搬过来。
// 只在第一次 GET 时执行（roles 为空才动手），不会覆盖用户后来在统一配置里做的选择。
func yyAISeedRolesFromQQ() bool {
	roles := aiStore.Roles()
	if roles.Language.Model != "" || roles.Image.Model != "" || roles.Speech.Model != "" {
		return false
	}
	as := yyQQSnapshot().Assistant
	if as.Model == "" && as.ImageModel == "" && as.VoiceModel == "" {
		return false
	}
	next := roles
	if as.Model != "" {
		next.Language.Model = as.Model
	}
	if as.ImageModel != "" {
		next.Image = AIModelRole{Model: as.ImageModel}
		if cfg := yyQQImageSnapshot(); cfg.Size != "" {
			next.Image.Size = cfg.Size
		}
	}
	if as.VoiceModel != "" {
		next.Speech = AIModelRole{Model: as.VoiceModel, Voice: as.VoiceVoice, Mode: as.VoiceMode}
	}
	if err := aiStore.SetRoles(next); err != nil {
		return false
	}
	return true
}

// yyAINormalizeRole 校验并归一化单个用途：模型必须真实存在，参数只收已知取值。
func yyAINormalizeRole(role string, in AIModelRole) (AIModelRole, error) {
	out := AIModelRole{Model: strings.TrimSpace(in.Model)}
	if out.Model != "" && aiStore.Get(out.Model) == nil {
		return out, fmt.Errorf("模型库里没有「%s」，请先在模型库里添加", out.Model)
	}
	switch role {
	case yyAIRoleImage:
		out.Size = strings.TrimSpace(in.Size)
		if out.Size != "" {
			var w, h int
			if _, err := fmt.Sscanf(out.Size, "%dx%d", &w, &h); err != nil || w < 64 || h < 64 || w > 4096 || h > 4096 {
				return out, fmt.Errorf("图片尺寸要写成 宽x高，例如 1024x1024（64..4096）")
			}
		}
	case yyAIRoleSpeech:
		out.Voice = strings.TrimSpace(in.Voice)
		if len([]rune(out.Voice)) > 40 {
			return out, fmt.Errorf("音色名不超过 40 个字符")
		}
		switch in.Mode {
		case "off", "auto", "always":
			out.Mode = in.Mode
		case "":
			out.Mode = "off"
		default:
			return out, fmt.Errorf("语音模式只支持 off / auto / always")
		}
	}
	return out, nil
}

// yyAISaveRoles 保存用途选择，并写穿到 qq-bridge.json 与 qq-image.json。
// 三个用途独立生效：只改语音时不会把语言模型也重置成空。
func yyAISaveRoles(in aiModelRoles) (aiModelRoles, error) {
	next := aiStore.Roles()
	var err error
	if next.Language, err = yyAINormalizeRole(yyAIRoleLanguage, in.Language); err != nil {
		return next, err
	}
	if next.Image, err = yyAINormalizeRole(yyAIRoleImage, in.Image); err != nil {
		return next, err
	}
	if next.Speech, err = yyAINormalizeRole(yyAIRoleSpeech, in.Speech); err != nil {
		return next, err
	}
	if err = aiStore.SetRoles(next); err != nil {
		return next, err
	}
	if err = yyAIApplyRoles(next); err != nil {
		return next, err
	}
	return next, nil
}

// yyAIApplyRoles 把用途选择写进真正干活的配置，保证「保存后立刻生效」。
func yyAIApplyRoles(roles aiModelRoles) error {
	lang := roles.Language.Model
	image := roles.Image.Model
	voice := roles.Speech.Model
	if err := yyAISyncQQAssistant(lang, image, voice, roles.Speech.Voice, roles.Speech.Mode); err != nil {
		return err
	}
	if image != "" {
		if err := yyAISyncQQImage(roles.Image); err != nil {
			return err
		}
	}
	return nil
}

// yyAISyncQQAssistant 写穿到 qq-bridge.json 的 assistant 段。
// 只改这 5 个字段，其它助手设置原样保留。
func yyAISyncQQAssistant(lang, image, voice, voiceName, voiceMode string) error {
	yyQQMu.Lock()
	defer yyQQMu.Unlock()
	next := yyQQ
	next.Assistant.Model = lang
	next.Assistant.ImageModel = image
	next.Assistant.VoiceModel = voice
	next.Assistant.VoiceVoice = voiceName
	next.Assistant.VoiceMode = voiceMode
	if yyQQPath == "" {
		yyQQ = next
		return nil
	}
	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(yyQQPath, raw, 0600); err != nil {
		return fmt.Errorf("保存群服互通模型选择失败: %w", err)
	}
	yyQQ = next
	return nil
}

// yyAISyncQQImage 把统一配置里的画图模型写成生图面板的配置。
// 生图队列实际跑的是 yyQQImageGenerate(qq-image.json)，不写这里就是假统一。
func yyAISyncQQImage(role AIModelRole) error {
	if yyQQImagePath == "" {
		return nil
	}
	m := aiStore.Get(role.Model)
	if m == nil {
		return fmt.Errorf("模型库里没有「%s」", role.Model)
	}
	cfg := yyQQImageSnapshot()
	cfg.Provider = yyAIImageProviderFor(m)
	cfg.Model = m.ModelID
	cfg.BaseURL = m.BaseURL
	cfg.APIKey = m.APIKey
	if role.Size != "" {
		cfg.Size = role.Size
	}
	// 把统一模型也放进可选列表，生图面板下拉才不会显示成「不在库里」。
	models := []string{m.ModelID}
	for _, x := range cfg.Models {
		if x != "" && x != m.ModelID {
			models = append(models, x)
		}
	}
	cfg.Models = models
	return yyQQImageSave(cfg)
}

// yyAIImageProviderFor 按地址与模型 ID 判断生图走哪条协议。
func yyAIImageProviderFor(m *AIModel) string {
	base := strings.ToLower(m.BaseURL)
	if strings.Contains(base, "api.cloudflare.com") || strings.HasPrefix(m.ModelID, "@cf/") {
		return "cloudflare"
	}
	return "openai"
}
