# 游园独立游戏认证

Cloudflare Workers + Durable Objects运行Go WebAssembly认证内核，直连网易。邮箱登录、游客创建、明确设置昵称、游戏令牌、刷新、撤销、按平台用户隔离的AES-GCM资料库。平台账号复用游园身份注册服务。

运行build.ps1生成kernel.wasm与匹配Go运行时。依赖Go1.27，随后用Wrangler部署wrangler.jsonc。需要配置私有secrets：AUTH_VAULT_KEY（64位十六进制随机AES密钥）、DEVICE_TOKEN和CONSOLE_TOKEN（兼容既有游园所有者）；公开源码不附带密钥。RELAY服务引用现有游园身份资料库。主工作台通过AUTH_SERVICE绑定本服务，不能再代理第三方验证API。

主要接口和使用方式见upstream/skills/self-auth/index.md。原生游客注册、不同服务器类型和长期刷新分别验收；HTTP200不能代替实际游戏进服测试。PC山头/PC大厅、本地联机认证尚未开放。游客可能受网易实名与风控限制。

修改点：山头流程保留已有加入记录，目标有歧义时要求server_id；只按用户请求设置昵称；MPay查询用户名来自本次输入，去除参考代码固定邮箱；恢复的设备使用有超时的HTTP客户端；每次WASM计算使用当前请求独立生命周期，结束后清理定时器；每批最多4个账号自动刷新。

参考组件源代码在third_party/funauth，保留其MIT以及g79client、unmcpk各自许可证。本服务及其修改提供对应源码；完整构建不能删除这些目录和许可证。源码下载在游园工作台的/source.zip。
