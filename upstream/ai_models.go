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

// AIModelRole 一个用途（语言/画图/语音）指向模型库里的哪条记录。
// 模型库只存一份地址与密钥，用途只存选择，避免同一套密钥在三个页面各配一遍。
type AIModelRole struct {
	Model string `json:"model"`           // 指向 models 里的 name；空 = 该用途未选模型
	Size  string `json:"size,omitempty"`  // 仅 image：出图尺寸
	Voice string `json:"voice,omitempty"` // 仅 speech：音色
	Mode  string `json:"mode,omitempty"`  // 仅 speech：off / auto / always
}

// aiModelRoles 三个用途的选择。
type aiModelRoles struct {
	Language AIModelRole `json:"language"`
	Image    AIModelRole `json:"image"`
	Speech   AIModelRole `json:"speech"`
}

// aiModelsFile 是 ai_models.json 的落盘结构。
// 旧版本这个文件是裸数组（只有模型列表），读的时候要兼容，否则升级后模型全丢。
type aiModelsFile struct {
	Version int          `json:"version"`
	Models  []*AIModel   `json:"models"`
	Roles   aiModelRoles `json:"roles"`
}

// aiModelStore 模型注册表：内存 + 落盘 <dataDir>/ai_models.json。
// 模型列表与三个用途的选择共用一个文件、一把锁，避免两处各写一半。
type aiModelStore struct {
	mu     sync.Mutex
	path   string
	models []*AIModel
	roles  aiModelRoles
}

// aiStore 全局模型注册表。路径由 StartServer 里 aiModelStore.setPath(dataDir) 初始化。
var aiStore = &aiModelStore{}

// setPath 设置持久化路径并加载已有模型（StartServer 时调用一次）。
func (s *aiModelStore) setPath(dataDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.path = dataDir + "/ai_models.json"
	// 重新指向另一个数据目录时，先清掉上一次的用途选择，避免把旧目录的选择带过去。
	s.roles = aiModelRoles{}
	if data, err := os.ReadFile(s.path); err == nil {
		// 新格式是 {models,roles}；老格式是裸数组。两种都要读得进来。
		var file aiModelsFile
		if err := json.Unmarshal(data, &file); err == nil && file.Models != nil {
			s.models = file.Models
			s.roles = file.Roles
		} else {
			_ = json.Unmarshal(data, &s.models)
		}
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

// saveLocked 把模型列表与用途选择一起写回磁盘（调用方需持锁）。
func (s *aiModelStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	models := s.models
	if models == nil {
		models = []*AIModel{}
	}
	data, err := json.MarshalIndent(aiModelsFile{Version: 2, Models: models, Roles: s.roles}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(s.path+".tmp", data, 0600); err != nil {
		return fmt.Errorf("模型配置写入失败: %w", err)
	}
	if err = os.Rename(s.path+".tmp", s.path); err != nil {
		return fmt.Errorf("模型配置替换失败: %w", err)
	}
	return nil
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
			before := *x
			*x = *m
			if err := s.saveLocked(); err != nil {
				*x = before
				return err
			}
			return nil
		}
	}
	copy := *m
	s.models = append(s.models, &copy)
	if err := s.saveLocked(); err != nil {
		s.models = s.models[:len(s.models)-1]
		return err
	}
	return nil
}

// Delete 按自定义名删除模型。内置模型不可删除。
func (s *aiModelStore) Delete(name string) error {
	if name == BuiltinModelName {
		return fmt.Errorf("内置模型不可删除")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before := append([]*AIModel(nil), s.models...)
	for i, m := range s.models {
		if m.Name == name {
			s.models = append(s.models[:i], s.models[i+1:]...)
			break
		}
	}
	if err := s.saveLocked(); err != nil {
		s.models = before
		return err
	}
	return nil
}

// Roles 返回三个用途选择的拷贝。
func (s *aiModelStore) Roles() aiModelRoles {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.roles
}

// SetRoles 写入三个用途的选择并落盘；写盘失败回滚，避免内存与磁盘不一致。
func (s *aiModelStore) SetRoles(next aiModelRoles) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := s.roles
	s.roles = next
	if err := s.saveLocked(); err != nil {
		s.roles = before
		return err
	}
	return nil
}
