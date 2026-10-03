package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// AIModel 一个配置好的 LLM 模型。
type AIModel struct {
	Name                  string  `json:"name"`                              // 本地自定义名（唯一，用户自己起）
	Provider              string  `json:"provider"`                          // "anthropic" | "openai"（openai 兼容 DeepSeek/Qwen/GLM/Ollama 等）
	BaseURL               string  `json:"base_url"`                          // 请求地址，如 https://api.openai.com / https://api.deepseek.com / http://localhost:11434
	APIKey                string  `json:"api_key"`                           // 密钥（明文存储，与 token 一致）
	ModelID               string  `json:"model_id"`                          // 供应商侧模型 id，如 gpt-4o / deepseek-chat / claude-sonnet-5
	Thinking              string  `json:"thinking"`                          // 思考强度：off | low | medium | high
	Streaming             bool    `json:"streaming"`                         // 是否流式输出
	InputPricePerMillion  float64 `json:"input_price_per_million,omitempty"` // 用户设置的每百万输入 token 单价
	OutputPricePerMillion float64 `json:"output_price_per_million,omitempty"`
	ContextWindow         int     `json:"context_window,omitempty"`
	MaxOutputTokens       int     `json:"max_output_tokens,omitempty"`
	CompressionPercent    int     `json:"compression_percent,omitempty"`
	Builtin               bool    `json:"builtin"` // 内置模型（不可删除/编辑，由代码运行时提供）
}

// 仅接入用户自行配置的模型，不使用游戏 Token 调用第三方代理。
const BuiltinModelName = "游园自定义"

// 供应商常量。
const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
)

// 思考强度 → anthropic 预算 / openai reasoning_effort 的映射。
const (
	ThinkingOff = "off"
)

var (
	thinkingBudget = map[string]int{"low": 2048, "medium": 8192, "high": 16384}
	thinkingEffort = map[string]string{"low": "low", "medium": "medium", "high": "high"}
)

// aiModelStore 模型注册表：内存 + 落盘 <dataDir>/ai_models.json。
type aiModelStore struct {
	mu     sync.Mutex
	path   string
	models []*AIModel
}

// aiStore 全局模型注册表。路径由 StartServer 里 aiModelStore.setPath(dataDir) 初始化。
var aiStore = &aiModelStore{}

// setPath 设置持久化路径并加载已有模型（StartServer 时调用一次）。
func (s *aiModelStore) setPath(dataDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.path = dataDir + "/ai_models.json"
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, &s.models)
	}
	if s.models == nil {
		s.models = []*AIModel{}
	}
	changed := false
	for _, model := range s.models {
		if !strings.HasPrefix(strings.TrimRight(model.BaseURL, "/"), "https://api.deepseek.com") {
			continue
		}
		if model.ModelID == "deepseek-chat" {
			model.ModelID = "deepseek-flash"
			model.Thinking = ThinkingOff
			changed = true
		} else if model.ModelID == "deepseek-reasoner" {
			model.ModelID = "deepseek-flash"
			model.Thinking = "high"
			changed = true
		}
	}
	if changed {
		s.saveLocked()
	}
}

// saveLocked 把模型列表写回磁盘（调用方需持锁）。
func (s *aiModelStore) saveLocked() {
	if s.path == "" {
		return
	}
	data, _ := json.MarshalIndent(s.models, "", "  ")
	if os.WriteFile(s.path+".tmp", data, 0600) == nil {
		_ = os.Rename(s.path+".tmp", s.path)
	}
}

// builtin 仅从用户自行设置的模型环境变量读取配置，不使用游戏 Token。
func (s *aiModelStore) builtin() *AIModel {
	return &AIModel{Name: BuiltinModelName, Provider: ProviderOpenAI, BaseURL: os.Getenv("YY_AI_BASE_URL"), APIKey: os.Getenv("YY_AI_API_KEY"), ModelID: os.Getenv("YY_AI_MODEL"), Streaming: true, Builtin: true}
}

// List 返回模型列表（拷贝，防外部改共享内存）。内置模型在前，随后是用户模型。
func (s *aiModelStore) List() []*AIModel {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*AIModel{}
	if model := s.builtin(); model.BaseURL != "" && model.ModelID != "" {
		out = append(out, model)
	}
	for _, m := range s.models {
		cp := *m
		out = append(out, &cp)
	}
	return out
}

// Get 按自定义名取模型。内置模型名直接返回内置模型。
func (s *aiModelStore) Get(name string) *AIModel {
	if name == BuiltinModelName {
		model := s.builtin()
		if model.BaseURL == "" || model.ModelID == "" {
			return nil
		}
		return model
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.models {
		if m.Name == name {
			copy := *m
			return &copy
		}
	}
	return nil
}

// Add 新增或更新模型（同名覆盖）。
func (s *aiModelStore) Add(m *AIModel) error {
	if m.Name == "" {
		return fmt.Errorf("模型名不能为空")
	}
	if m.Name == BuiltinModelName {
		return fmt.Errorf("内置模型「%s」不可编辑", BuiltinModelName)
	}
	if m.Provider != ProviderAnthropic && m.Provider != ProviderOpenAI {
		return fmt.Errorf("不支持的供应商: %s（仅 anthropic / openai）", m.Provider)
	}
	if m.BaseURL == "" {
		return fmt.Errorf("请求地址不能为空")
	}
	if m.ModelID == "" {
		return fmt.Errorf("模型ID不能为空")
	}
	if m.InputPricePerMillion < 0 || m.OutputPricePerMillion < 0 {
		return fmt.Errorf("Token 单价不能为负数")
	}
	if m.Thinking == "" {
		m.Thinking = ThinkingOff
	}
	if m.ContextWindow != 0 && (m.ContextWindow < 4096 || m.ContextWindow > 2000000) {
		return fmt.Errorf("上下文容量应为 4096..2000000 Token")
	}
	if m.MaxOutputTokens < 0 || (m.MaxOutputTokens != 0 && m.MaxOutputTokens >= m.contextWindow()) {
		return fmt.Errorf("输出预留必须小于上下文容量")
	}
	if m.CompressionPercent != 0 && (m.CompressionPercent < 50 || m.CompressionPercent > 95) {
		return fmt.Errorf("压缩阈值应为 50..95%%")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.models {
		if x.Name == m.Name {
			*x = *m
			s.saveLocked()
			return nil
		}
	}
	s.models = append(s.models, m)
	s.saveLocked()
	return nil
}

// Delete 按自定义名删除模型。内置模型不可删除。
func (s *aiModelStore) Delete(name string) {
	if name == BuiltinModelName {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, m := range s.models {
		if m.Name == name {
			s.models = append(s.models[:i], s.models[i+1:]...)
			break
		}
	}
	s.saveLocked()
}
