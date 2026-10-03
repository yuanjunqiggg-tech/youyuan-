package main

import "testing"

// 验证内置插件注册表结构（当前为空：插件由用户安装后 AddDef 注册）。
func TestBuiltinPluginDefs(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range builtinPlugins {
		if p.ID == "" || p.Name == "" || p.Type == "" || p.Description == "" || p.Load == nil {
			t.Errorf("插件 %q 字段缺失", p.ID)
		}
		if seen[p.ID] {
			t.Errorf("重复 id: %s", p.ID)
		}
		seen[p.ID] = true
		if p.Type != "lua" && p.Type != "wordbank" {
			t.Errorf("插件 %s 类型非法: %s", p.ID, p.Type)
		}
	}
}
