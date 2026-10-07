# Chat OAuth 与 Linux MCP 接入 · 游园 0.10.13

连接网址：https://toolbox.youyuanqi.dpdns.org/mcp/oauth/linux-server
后台网址：https://toolbox.youyuanqi.dpdns.org/

Chat 的连接器/MCP 设置中创建自定义连接，填上面地址，认证选 OAuth。客户端ID/Secret不需要自行粘贴设备密钥，服务支持动态客户端注册、授权码与S256 PKCE。浏览器打开游园授权页后，使用自己的游园平台账号登录并同意权限。帐号“幽怨qi”的恢复密码单独交付私有文件，公共文档没有任何凭据。

只有管理员和未过期的订阅用户可以使用。普通订阅用户还需要被分配 linux-server 节点；每次调用都会复查。scope=mcp:read 只列出5个只读工具；mcp:control 包含真实管理工具，可以修改插件、运行命令。不要把管理授权给普通群成员。

资源发现：/.well-known/oauth-protected-resource/mcp/oauth/linux-server
授权服务器发现：/.well-known/oauth-authorization-server
授权/兑换/注册/撤销：/oauth/authorize、/oauth/token、/oauth/register、/oauth/revoke
访问令牌1小时，刷新令牌30天，刷新轮换；旧刷新令牌重复使用撤销整组授权。密码重置立即让此前OAuth令牌失效。完整URL绑定节点，参数或X-YouYuan-Device不能切换节点。

本电脑已经通过公网OAuth完成Linux实际握手、244工具目录和在线状态调用。文件读写另见验收JSON。Chat界面自身尚未完成授权：本轮浏览器控制报nodeRepl.fetch错误/超时，不能将服务端通过当成Chat客户端已连通。

403区别：游园JSON SUBSCRIPTION_REQUIRED/节点未授权是账号权限；Cloudflare HTML和1010是边缘拦截。本轮从本电脑使用浏览器、ChatGPT-User、OpenAI-MCP、httpx User-Agent读取发现接口均为200，没有复现1010；不是已证明OpenAI实际出口不会被拦截。如Chat仍报错，请记录时间、CF-Ray和错误全文，针对MCP/OAuth路径查规则，不关闭全站防护。

公共备用地址：https://youyuan-apk-v030.yuanjunqiggg.workers.dev/mcp/oauth/linux-server
它有自己的相同域名issuer/audience，需要独立注册授权；不能把toolbox域名令牌交给workers.dev。用于排查自定义域名WAF，不是匿名绕过权限。

插件多文件读写：api_plugin_file_read / api_plugin_file_write 的 scope=code,file=modules/core.lua；scope=data,file=ai_prompts/core.txt。file为空仍读写旧入口文件。拒绝越界/符号链接。写入后根据修改范围热重载；命令TXT只重编译被修改独立段。

进服就绪生命周期：常驻插件在机器人已就绪后加载或热重载，宿主补发它自己的onBotReady回调；同一个Runtime同一个handler只一次，源码不重跑。网络重新建立创建的新Runtime重新初始化。

`.help`目前测试证明15个无副作用菜单入口经过完整on_chat包装与真实存档逻辑可调用；不等于市场交易、领地、死亡、灵魂出窍等业务全部在真实玩家操作中通过。这些场景继续保留待办。
