package main

import "testing"

// TestSampleThemeAndPluginValid 用真实校验器跑 samples/，防止示例 manifest 漂移。
func TestSampleThemeAndPluginValid(t *testing.T) {
	themes := yyThemeScan("samples")
	found := false
	for _, th := range themes {
		if th.ID != "mcort-aurora" {
			continue
		}
		found = true
		if !th.OK {
			t.Fatalf("示例主题未通过校验：%s", th.Error)
		}
		if th.Name == "" || th.Version == "" || th.API != yyThemeAPIVersion {
			t.Fatalf("示例主题字段不完整：%+v", th)
		}
	}
	if !found {
		t.Fatal("samples/themes 下没有找到 mcort-aurora")
	}

	plugins := yyThemeScanPlugins("samples", map[string]bool{"seed-query": true})
	pf := false
	for _, p := range plugins {
		if p.ID != "seed-query" {
			continue
		}
		pf = true
		if !p.OK {
			t.Fatalf("示例插件未通过校验：%s", p.Error)
		}
		if !p.Enabled {
			t.Fatal("示例插件应记录为 enabled")
		}
		if p.Executed {
			t.Fatal("插件执行层默认关闭，executed 必须为 false")
		}
	}
	if !pf {
		t.Fatal("samples/plugins 下没有找到 seed-query")
	}
}
