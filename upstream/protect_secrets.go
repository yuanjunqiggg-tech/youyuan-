package main

// 敏感常量收敛层（protect_secrets.go）
//
// 目的：把「拿到二进制就等于拿到我们的配置」这件事打断。
//
// 原状态：网易皮肤图源白名单、自有云端地址（auth / toolbox / updates）、
// QQ 助手的管理员提示词，全部以明文形式躺在 .rodata 里，`strings youyuan`
// 一条命令就能捞出来——等于把「我们跟哪些域通信、什么参数」直接送人。
//
// 本层的做法是**只做存储形态的变换，不做任何逻辑改动**：
//   - 常量在源码里以 XOR 编码的 hex 形式存在，运行时在 yySecLoad 里还原；
//   - 对外仍然是原来那些标识符（yySkinAllowedSuffixes / yySkinAllowedHosts），
//     调用点一行都不用改，行为逐字节等价；
//   - 还原结果落地在一份 sync.Once 缓存里，热路径（每个请求都过的
//     yySkinHostAllowed）不会重复解码。
//
// 这不是密码学意义上的加密——密钥就在同一个二进制里，铁了心逆向的人仍然
// 可以还原。它的价值是提高「strings + grep」这条最低成本路径的门槛，把
// 「随手一看就能拿到」变成「必须读代码 + 写脚本」。真正的强度要靠：核心逻辑
// 上云 + 服务端收权 + 外购加固（见报告第四节）。这一点必须在报告里说清楚，
// 不能对用户吹成「绝对拿不到」。

import (
	"encoding/hex"
	"strings"
	"sync"
)

// yySecKey 是编码密钥。故意写成短字符串，避免被当作 base64 高熵串一眼认出。
const yySecKey = "yy:2026:prism:toolbox:k9"

// yySecDecode 还原一段 hex 编码的常量。
// 变换：raw[i] ^= key[i%len(key)] ^ byte(i*37+11)
// 第二个异或项让相同字符在不同位置呈现不同字节，避免「空格 / 换行」在
// hex 里反复出现同一个值而形成可识别的纹理。
func yySecDecode(blob string) string {
	raw, err := hex.DecodeString(strings.TrimSpace(blob))
	if err != nil {
		// 编码损坏时返回空串而不是 panic：调用方都是「白名单 / 兜底地址」，
		// 空串会走拒绝分支，是安全侧失败，不会把业务带崩。
		return ""
	}
	key := yySecKey
	for i := range raw {
		raw[i] ^= key[i%len(key)] ^ byte(i*37+11)
	}
	return string(raw)
}

// 编码后的常量。hex 每行都短，便于 review 时确认没有夹带别的东西。
const (
	yySecBlobSkinSuffixes     = "5c270a3cca97ac516d497bbcd6f8546f07c2a4cafa527c56c8e7d9ae427a"
	yySecBlobSkinHosts        = "113b0e2ece82be466d497bbcd6bb0c375b98a6d7b940371386bc94ed4f6b36d8a88cf20c26"
	yySecBlobAccountSkinHosts = "1c2c1b2dce85ba1a204579ad84b8002d518db4c0b94d3d0a86e0c1f3096d38d1a78cad5f61709eaed1e8511c2d80f481e72d"
	yySecBlobPlatformOrigin   = "1a3d1b38dcccf01b37457bbdc8b91d774d83b2dce24f3c1693ff93b043602492b4d0fb"
	yySecBlobAuthFallback     = "1a3d1b38dcccf01b225f60b984af0a2c4d99a6cbe6477c038ab599b3096125db"
	yySecBlobUpdateManifest   = "1a3d1b38dcccf01b365a70b0deb316774d83b2dce24f3c1693ff93b043602492b4d0fb46272e99a0c8f10c027ed8be9ded301b77d3e79ba1204e2a8ba9891b"
	yySecBlobUpdateHost       = "07390b29db93ac1a3a4561a8dfb70b285dc2a3d5f340214995a390"
	yySecBlobUpdatePath       = "5d3c1f2cce82ba476c4b7ab5d8b90c3d1b80a6d1f25d26499ba19c"
	yySecBlobQQAdminPersona   = "78add2e8496e70dcfcb3f0690031dbfdd3764342398fb5f77c346658c3b7dc5863227fe9d0b943655513f3c890273d7b1dbee5dc5b3c6420ee9aed626701e9c2c1805f323b82abf0860e3b4ad182eea8510864c6df86aa754839cfb8a0e92e44378ea9e9a73e7f3bdfc6d26d43123fc3b1845020064196baba14184f2395d2ba7452550fe091a16d3a372ca58fd51205209fafecb9161401fcf5c0ba5f631af8ac9dbf13445b879bafe2277131b58a88b3524a12efedad902b263695faf8091f306189898a3510103beab0a940713906918299024d6a75999ef6192101529c999469331508f8a1881a543e88e5d0f92f437fa3919ace29613cc2a7a19b420d0482e792f2735664aac9c4cd0b1862bce8937f2b223acbc8820b5e2b7bd98eec3e7d4d77a5e8c217720c0592acab5f460472c282ed58135147d8"
	yySecBlobQQAdminTools     = "52acd2db4a7f52d3d782f2591d30fdf6d15b754319a6b4fa79365961c09ed1594a3a73d5debb726e582fdac9a2093d7b2abfc7e0590e6021e49fe1646d03e7c5cd997a321a90a5ed8e042346de9dcf01a8e268c6ec8899484a18e2b8a5e72f663a8a8feaaa215a31e6fdd1557b1e31dcb284712d014b9490ac152f6d2f88f6b47c685903da979a6c393b0ea8b2e713160b9c8ec5bb1d2b07f3d8c0bd626310e1a081a8125376848baae4176132818284ad53492ec7eba8a623181498ebf0031c1ac46c588a3935111fc2bd8d6a7e031892bbb189be9de6c8869d52066aaa720f7890b5cbf7f8a291186c1588e8d3ff2a757683949ac022e7f21628cea63de6e20d305bf27356ee582b4518e39cb74b60ce7a4a50239ba59e5721200b8c86885a5d0628bca705d4fdcfe69b8d90c7b494f53f546c84fad3a92ed2d721e58b536e546197e4d151304c7d9abbfe9c96bb2c8dd4b259710262c1cbff07344e2fbac767fcd65668f3cfb604254ad3ff9fae4266739cb0414eec21649dbcd4ec47467293aa9ba140182c57432be03e453a87b6cc412b4c61cbff831f27122cc8feba593556ff69503ac0e1d4ab67451c8bbdfbaaf98fb74c723035a9d5ea047a824e6c4c1bddb0900f2a4d2185bced5211762bc288e56e760b07d9ca846e32134eaff9970c21512791d5b1547b51f7d399b9753034eaa093c31a382a9996eaa20a5e34d5f2d9b4671121e8bfb49d"
)

var (
	yySecOnce            sync.Once
	yySecSkinSuffixes    []string
	yySecSkinHosts       map[string]bool
	yySecAccountSkinList []string
	yySecPlatform        string
	yySecAuthFallback    string
	yySecUpdateManifest  string
	yySecUpdateHost      string
	yySecUpdatePath      string
	yySecQQAdminPersona  string
	yySecQQAdminTools    string
)

// yySecLoad 只跑一次，把全部常量还原进包级缓存。
func yySecLoad() {
	yySecOnce.Do(func() {
		yySecSkinSuffixes = yySecSplitNonEmpty(yySecDecode(yySecBlobSkinSuffixes))
		yySecAccountSkinList = yySecSplitNonEmpty(yySecDecode(yySecBlobAccountSkinHosts))
		yySecSkinHosts = make(map[string]bool, 4)
		for _, h := range yySecSplitNonEmpty(yySecDecode(yySecBlobSkinHosts)) {
			yySecSkinHosts[h] = true
		}
		yySecPlatform = yySecDecode(yySecBlobPlatformOrigin)
		yySecAuthFallback = yySecDecode(yySecBlobAuthFallback)
		yySecUpdateManifest = yySecDecode(yySecBlobUpdateManifest)
		yySecUpdateHost = yySecDecode(yySecBlobUpdateHost)
		yySecUpdatePath = yySecDecode(yySecBlobUpdatePath)
		yySecQQAdminPersona = yySecDecode(yySecBlobQQAdminPersona)
		yySecQQAdminTools = yySecDecode(yySecBlobQQAdminTools)
	})
}

func yySecSplitNonEmpty(s string) []string {
	parts := strings.Split(s, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---- 对外访问器：名字刻意保持「读起来就是原来那个常量」----

// yySecSkinSuffixesList 对应原 yySkinAllowedSuffixes（精确 + 后缀放行）。
func yySecSkinSuffixesList() []string {
	yySecLoad()
	return yySecSkinSuffixes
}

// yySecSkinHostsMap 对应原 yySkinAllowedHosts（精确放行的第三方头像站）。
func yySecSkinHostsMap() map[string]bool {
	yySecLoad()
	return yySecSkinHosts
}

// yySecIsAccountSkinHost 对应原 yyAccountSkinOfficialURL 里的那段域名判断。
// 语义逐字保留：不带点的条目只做精确匹配，带点的条目允许「本域 + 任意子域」，
// 因此 ".126.net" 不会误放行裸 "126.net"。
func yySecIsAccountSkinHost(host string) bool {
	yySecLoad()
	for _, d := range yySecAccountSkinList {
		if strings.HasPrefix(d, ".") {
			if strings.HasSuffix(host, d) {
				return true
			}
			continue
		}
		if host == d {
			return true
		}
	}
	return false
}

// yySecDeviceSocketURL 由平台地址派生设备长连地址（https → wss）。
// 派生而不是再存一份，既少一处明文，也保证两条链永远指向同一个域。
func yySecDeviceSocketURL() string {
	return "wss://" + strings.TrimPrefix(yySecPlatformOrigin(), "https://") + "/device/connect"
}

func yySecPlatformOrigin() string    { yySecLoad(); return yySecPlatform }
func yySecAuthOriginFallback() string { yySecLoad(); return yySecAuthFallback }
func yySecUpdateManifestURL() string { yySecLoad(); return yySecUpdateManifest }
func yySecUpdateHostOnly() string    { yySecLoad(); return yySecUpdateHost }
func yySecUpdateAPKPath() string     { yySecLoad(); return yySecUpdatePath }
func yySecQQAdminPersonaText() string { yySecLoad(); return yySecQQAdminPersona }
func yySecQQAdminToolsText() string   { yySecLoad(); return yySecQQAdminTools }
