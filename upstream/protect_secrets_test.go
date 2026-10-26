package main

// 敏感常量收敛层的回归测试。
//
// 这里故意把期望值写成明文——测试二进制不会随产品发布，泄露面为零；
// 而一旦有人手改 protect_secrets.go 里的编码串（改错一位、换行、大小写），
// 或者改了 yySecDecode 的变换公式，这些用例会立刻炸掉，而不是等到线上
// 头像全不显示 / 更新检查全失败才发现。
//
// 覆盖点：
//  1. 十段常量逐字还原正确；
//  2. 白名单判定语义与改造前逐条等价（尤其 ".126.net" 不能放行裸 "126.net"）；
//  3. QQ 助手提示词块的前导换行/空格没丢（拼接依赖它）；
//  4. 解码失败时安全侧失败（空串），不 panic。

import (
	"strings"
	"testing"
)

func TestProtectSecretsDecodePlaintext(t *testing.T) {
	cases := []struct {
		name     string
		got      string
		expected string
	}{
		{"皮肤代理后缀", strings.Join(yySecSkinSuffixesList(), "|"), ".netease.com|.163.com|.126.net"},
		{"皮肤代理精确域名", mapKeysSorted(yySecSkinHostsMap()), "crafatar.com|mc-heads.net|minotar.net"},
		{"账户皮肤域名", strings.Join(yySecAccountSkinList, "|"), "netease.com|.netease.com|163.com|.163.com|.126.net"},
		{"云端控制面", yySecPlatformOrigin(), "https://toolbox.youyuanqi.dpdns.org"},
		{"认证兜底", yySecAuthOriginFallback(), "https://auth.youyuanqi.dpdns.org"},
		{"更新清单", yySecUpdateManifestURL(), "https://updates.youyuanqi.dpdns.org/updates/android/latest.json"},
		{"更新主机", yySecUpdateHostOnly(), "updates.youyuanqi.dpdns.org"},
		{"更新路径", yySecUpdateAPKPath(), "/updates/android/latest.apk"},
		{"设备长连", yySecDeviceSocketURL(), "wss://toolbox.youyuanqi.dpdns.org/device/connect"},
	}
	for _, c := range cases {
		if c.got != c.expected {
			t.Fatalf("%s 还原不一致\n got=%q\nwant=%q", c.name, c.got, c.expected)
		}
	}
}

func TestProtectSecretsQQPrompts(t *testing.T) {
	persona := yySecQQAdminPersonaText()
	if !strings.HasPrefix(persona, "\n") {
		t.Fatalf("群管文案丢了前导换行，拼接后会跟前一句粘在一起：%q", firstN(persona, 20))
	}
	if !strings.Contains(persona, "你是这个群的管理员之一") {
		t.Fatalf("群管文案内容不对：%q", firstN(persona, 40))
	}
	tools := yySecQQAdminToolsText()
	if !strings.HasPrefix(tools, " ") {
		t.Fatalf("管理员工具文案丢了前导空格：%q", firstN(tools, 20))
	}
	if !strings.Contains(tools, "post_qzone") {
		t.Fatalf("管理员工具文案内容不对：%q", firstN(tools, 40))
	}
}

func TestProtectSecretsAccountSkinHostSemantics(t *testing.T) {
	// 与改造前 yyAccountSkinOfficialURL 的判断逐条对齐。
	cases := map[string]bool{
		"netease.com":         true,
		"x19.gph.netease.com": true,
		"163.com":             true,
		"a.b.163.com":         true,
		"x.126.net":           true,
		"126.net":             false, // 原实现只判后缀，裸 126.net 不放行
		"evilnetease.com":     false,
		"netease.com.evil.io": false,
		"":                    false,
	}
	for host, want := range cases {
		if got := yySecIsAccountSkinHost(host); got != want {
			t.Fatalf("yySecIsAccountSkinHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestProtectSecretsSkinProxyWhitelistSemantics(t *testing.T) {
	// 与 yySkinCache 的 yySkinHostAllowed 语义对齐（这条链允许裸 126.net）。
	yes := []string{"crafatar.com", "minotar.net", "mc-heads.net", "netease.com", "x19.gph.netease.com", "s.163.com", "126.net", "img.126.net"}
	for _, h := range yes {
		if !yySkinHostAllowed(h) {
			t.Fatalf("应放行：%s", h)
		}
	}
	no := []string{"evilnetease.com", "netease.com.evil.io", "crafatar.com.evil.io", "1.1.1.1", ""}
	for _, h := range no {
		if yySkinHostAllowed(h) {
			t.Fatalf("应拒绝：%s", h)
		}
	}
}

func TestProtectSecretsDecodeBrokenIsSafe(t *testing.T) {
	if got := yySecDecode("这不是hex"); got != "" {
		t.Fatalf("损坏编码应返回空串，得到 %q", got)
	}
	if got := yySecDecode("zz"); got != "" {
		t.Fatalf("非法 hex 应返回空串，得到 %q", got)
	}
}

func mapKeysSorted(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return strings.Join(keys, "|")
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
