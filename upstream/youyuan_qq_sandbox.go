package main

// 互通「沙盒试跑」：用真实模型跑一次人设，不经过 QQ、不写群记忆、不发群消息。
//
// 旧实现是前端把提示词原样回显成「群友说：xxx」，用户看不到模型的真实反应，
// 误以为 AI 不回答。这里补一个只读的试跑接口：给定人设提示词 + 一句话，
// 用当前选择（或默认）模型生成一次回复，把 reply 与 token 用量如实返回。

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func yyQQSandboxHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Text   string `json:"text"`
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&p); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
		return
	}
	text := strings.TrimSpace(p.Text)
	if text == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "先写一句要试的话"})
		return
	}
	cf := yyQQSnapshot()
	name := strings.TrimSpace(p.Model)
	if name == "" {
		name = strings.TrimSpace(cf.Assistant.Model)
	}
	if name == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "还没有选择模型：先选一个，或点「配置模型」添加"})
		return
	}
	model := aiStore.Get(name)
	if model == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "找不到模型 " + name + "，可能已被删除"})
		return
	}
	sys := strings.TrimSpace(p.Prompt)
	if sys == "" {
		sys = "你是群里的聊天助手，用简短自然的中文回复，不要解释自己是模型。"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	turn, err := model.complete(ctx, []aiTurn{{Role: "system", Text: sys}, {Role: "user", Text: text}}, nil, aiStreamHooks{})
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	reply := strings.TrimSpace(turn.Text)
	if reply == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "模型没有返回内容（可能是思考型模型只给了推理）"})
		return
	}
	writeJSON(w, map[string]any{
		"ok": true, "reply": reply, "model": name,
		"input_tokens": turn.InputTokens, "output_tokens": turn.OutputTokens,
	})
}
