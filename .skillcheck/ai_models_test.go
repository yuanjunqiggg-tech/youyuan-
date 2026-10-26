package main

import "testing"

func TestYouYuanNoAuthorDefaultAI(t *testing.T) {
	t.Setenv("YY_AI_BASE_URL", "")
	t.Setenv("YY_AI_MODEL", "")
	t.Setenv("YY_AI_API_KEY", "")
	s := &aiModelStore{}
	if len(s.List()) != 0 || s.Get(BuiltinModelName) != nil || s.builtin().APIKey != "" {
		t.Fatal("无配置时出现原作者代理或游戏 Token 被当作模型密钥")
	}
}

// 内置模型「prism自动」在 List 中排在首位，且标记 Builtin。
func TestBuiltinModelList(t *testing.T) {
	t.Setenv("YY_AI_BASE_URL", "https://model.example.invalid")
	t.Setenv("YY_AI_MODEL", "own-model")
	s := &aiModelStore{models: []*AIModel{}}
	list := s.List()
	if len(list) == 0 {
		t.Fatal("List 应至少含内置模型")
	}
	if list[0].Name != BuiltinModelName {
		t.Errorf("首个模型=%q, want %q", list[0].Name, BuiltinModelName)
	}
	if !list[0].Builtin {
		t.Error("内置模型应标记 Builtin=true")
	}
	if list[0].BaseURL != "https://model.example.invalid" {
		t.Errorf("内置模型 BaseURL=%q, want prism 代理", list[0].BaseURL)
	}
}

// Get 能取到内置模型。
func TestBuiltinModelGet(t *testing.T) {
	t.Setenv("YY_AI_BASE_URL", "https://model.example.invalid")
	t.Setenv("YY_AI_MODEL", "own-model")
	s := &aiModelStore{models: []*AIModel{}}
	if m := s.Get(BuiltinModelName); m == nil {
		t.Fatal("Get 内置名应返回内置模型")
	} else if !m.Builtin {
		t.Error("内置模型应标记 Builtin")
	}
}

// 内置模型不可删除。
func TestBuiltinModelDeleteRejected(t *testing.T) {
	s := &aiModelStore{models: []*AIModel{{Name: "user-model"}}}
	s.Delete(BuiltinModelName) // 应 no-op
	if len(s.models) != 1 || s.models[0].Name != "user-model" {
		t.Errorf("内置模型被误删: %+v", s.models)
	}
}

// 内置模型不可编辑（Add 同名应报错）。
func TestBuiltinModelAddRejected(t *testing.T) {
	s := &aiModelStore{models: []*AIModel{}}
	err := s.Add(&AIModel{Name: BuiltinModelName, Provider: ProviderOpenAI, BaseURL: "x", ModelID: "y"})
	if err == nil {
		t.Fatal("Add 内置名应报错")
	}
}

// 普通用户模型仍可正常增删。
func TestUserModelAddDelete(t *testing.T) {
	s := &aiModelStore{models: []*AIModel{}}
	m := &AIModel{Name: "deepseek", Provider: ProviderOpenAI, BaseURL: "https://api.deepseek.com", ModelID: "deepseek-chat"}
	if err := s.Add(m); err != nil {
		t.Fatalf("Add 用户模型失败: %v", err)
	}
	if s.Get("deepseek") == nil {
		t.Fatal("应能取到用户模型")
	}
	s.Delete("deepseek")
	if s.Get("deepseek") != nil {
		t.Error("用户模型删除失败")
	}
}
