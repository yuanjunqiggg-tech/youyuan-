package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maskKey 脱敏 api_key 展示：保留前4后4，中间打码。
func maskKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "****"
	}
	return k[:4] + "****" + k[len(k)-4:]
}

// handleAIModels 按方法分发：GET=列出（key 脱敏）+ 三个用途的选择、
// POST=新增/更新模型，或 body 里带 roles 时只更新用途选择、DELETE=删除。
func handleAIModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// 同一个入口两种 body：带 roles 的是「统一模型配置」里的用途选择，
		// 不带的仍是原来的模型增改。分成两个路由反而要多登记两份目录。
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "读取请求失败"})
			return
		}
		var body struct {
			Roles *aiModelRoles `json:"roles"`
			AIModel
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
			return
		}
		if body.Roles != nil {
			if _, err := yyAISaveRoles(*body.Roles); err != nil {
				writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "roles": yyAIRolesView()})
			return
		}
		yyAIUpsertModel(w, &body.AIModel)
	case http.MethodDelete:
		handleAIModelDelete(w, r)
	default:
		// 老用户第一次打开统一配置时，把群服互通里已选的模型迁过来。
		yyAISeedRolesFromQQ()
		list := aiStore.List()
		out := make([]map[string]any, 0, len(list))
		for _, m := range list {
			out = append(out, map[string]any{
				"name":                     m.Name,
				"provider":                 m.Provider,
				"base_url":                 m.BaseURL,
				"api_key":                  maskKey(m.APIKey),
				"model_id":                 m.ModelID,
				"thinking":                 m.Thinking,
				"streaming":                m.Streaming,
				"input_price_per_million":  m.InputPricePerMillion,
				"output_price_per_million": m.OutputPricePerMillion,
				"context_window":           m.ContextWindow,
				"max_output_tokens":        m.MaxOutputTokens,
				"compression_percent":      m.CompressionPercent,
				"builtin":                  m.Builtin,
			})
		}
		writeJSON(w, map[string]any{"ok": true, "models": out, "roles": yyAIRolesView()})
	}
}

// handleAIModelAdd 新增/更新模型。
func handleAIModelAdd(w http.ResponseWriter, r *http.Request) {
	var m AIModel
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
		return
	}
	yyAIUpsertModel(w, &m)
}

// yyAIUpsertModel 校验并写入一条模型记录；名称未变且 key 被脱敏回传时保留原 key。
func yyAIUpsertModel(w http.ResponseWriter, m *AIModel) {
	if m.Name == BuiltinModelName {
		writeJSON(w, map[string]any{"ok": false, "error": "内置模型「" + BuiltinModelName + "」不可编辑"})
		return
	}
	if cur := aiStore.Get(m.Name); cur != nil && (m.APIKey == "" || strings.Contains(m.APIKey, "****")) {
		m.APIKey = cur.APIKey
	}
	if err := aiStore.Add(m); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleAIModelDelete 删除模型。
func handleAIModelDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少模型名"})
		return
	}
	if req.Name == BuiltinModelName {
		writeJSON(w, map[string]any{"ok": false, "error": "内置模型「" + BuiltinModelName + "」不可删除"})
		return
	}
	if err := aiStore.Delete(req.Name); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleAISessions 会话管理：GET=列表、POST=保存、DELETE=删除。
func handleAISessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Name     string   `json:"name"`
			PluginID string   `json:"plugin_id"`
			Messages []aiTurn `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) == 0 {
			writeJSON(w, map[string]any{"ok": false, "error": "没有可保存的内容"})
			return
		}
		if err := sessionStore.Save(req.Name, req.PluginID, req.Messages); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "无效请求"})
			return
		}
		sessionStore.Delete(req.Name)
		writeJSON(w, map[string]any{"ok": true})
	default:
		pluginID := r.URL.Query().Get("plugin_id")
		writeJSON(w, map[string]any{"ok": true, "sessions": sessionStore.List(pluginID)})
	}
}

// handleAISessionRecent 读回最近一个会话（用于自动续接）；可按 plugin_id 隔离到单个插件。
func handleAISessionRecent(w http.ResponseWriter, r *http.Request) {
	pluginID := r.URL.Query().Get("plugin_id")
	sess, ok := sessionStore.Recent(pluginID)
	if !ok {
		writeJSON(w, map[string]any{"ok": true, "session": nil})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "session": sess})
}

// handleAISessionRename 重命名一个会话（AI 改标题）。
func handleAISessionRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldName string `json:"old_name"`
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OldName == "" || req.NewName == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少会话名"})
		return
	}
	if err := sessionStore.Rename(req.OldName, req.NewName); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "name": req.NewName})
}

// handleAISessionLoad 读档一个会话。
func handleAISessionLoad(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少会话名"})
		return
	}
	sess, err := sessionStore.Load(req.Name)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "读档失败: " + err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "session": sess})
}

// handleAISnapshot 改动存档：GET=列表、POST=存档、DELETE=删除。
func handleAISnapshot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "缺少存档名"})
			return
		}
		if err := snapshotStore.Save(req.Name); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "无效请求"})
			return
		}
		snapshotStore.Delete(req.Name)
		writeJSON(w, map[string]any{"ok": true})
	default:
		writeJSON(w, map[string]any{"ok": true, "snapshots": snapshotStore.List()})
	}
}

// handleAISnapshotRestore 读档（恢复）一个改动快照。
func handleAISnapshotRestore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少存档名"})
		return
	}
	n, err := snapshotStore.Restore(req.Name)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "读档失败: " + err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "restored": n})
}

// handleAIAnswer 接收用户对 AI 问题的选择，回传给等待中的聊天请求。
func handleAIAnswer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReqID  string `json:"req_id"`
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ReqID == "" || req.Answer == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "无效请求"})
		return
	}
	ch, ok := aiQuestions.pop(req.ReqID)
	if !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "问题不存在或已过期"})
		return
	}
	ch <- req.Answer
	writeJSON(w, map[string]any{"ok": true})
}

// handleAISkills 技能管理：GET=列出、POST=加载、DELETE=卸载。
func handleAISkills(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "缺少技能 id"})
			return
		}
		content, err := skillStore.Load(req.ID)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "chars": len(content)})
	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "缺少技能 id"})
			return
		}
		skillStore.Unload(req.ID)
		writeJSON(w, map[string]any{"ok": true})
	default:
		writeJSON(w, map[string]any{"ok": true, "skills": skillStore.List()})
	}
}

// handleAIModelFetch 凭 base_url+key 拉取该供应商的模型列表。
func handleAIModelFetch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无效JSON"})
		return
	}
	base := strings.TrimRight(req.BaseURL, "/")
	if req.APIKey == "" && req.Name != "" {
		if saved := aiStore.Get(req.Name); saved != nil && saved.BaseURL == req.BaseURL {
			req.APIKey = saved.APIKey
		}
	}
	// 候选模型列表接口：不同服务路径不同，逐个尝试直到拿到 200。
	var candidates []string
	switch req.Provider {
	case ProviderAnthropic:
		if strings.HasSuffix(base, "/v1") {
			candidates = []string{base + "/models"}
		} else {
			candidates = []string{base + "/v1/models", base + "/models"}
		}
	default:
		if strings.HasSuffix(base, "/v1") {
			candidates = []string{base + "/models"}
		} else {
			candidates = []string{base + "/v1/models", base + "/models"}
		}
	}
	var headers map[string]string
	if req.Provider == ProviderAnthropic {
		headers = map[string]string{"x-api-key": req.APIKey, "anthropic-version": "2023-06-01"}
	} else {
		headers = map[string]string{"Authorization": "Bearer " + req.APIKey}
	}
	var lastErr string
	for _, url := range candidates {
		status, _, text, err := httpRequestFull("GET", url, "", headers, 30*time.Second)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		if status != 200 {
			lastErr = "HTTP " + strconv.Itoa(status) + " " + truncateStr(text, 200)
			continue
		}
		var resp struct {
			Data []struct {
				ID              string `json:"id"`
				ContextWindow   int    `json:"context_window,omitempty"`
				MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			lastErr = "响应解析失败: " + err.Error()
			continue
		}
		ids := make([]string, 0, len(resp.Data))
		for _, d := range resp.Data {
			if d.ID != "" {
				ids = append(ids, d.ID)
			}
		}
		writeJSON(w, map[string]any{"ok": true, "models": ids, "metadata": resp.Data, "empty": len(ids) == 0})
		return
	}
	writeJSON(w, map[string]any{"ok": false, "error": "无法获取模型列表: " + lastErr})
}
