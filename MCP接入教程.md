# 游园 MCP 接入教程

先分清方向：**外部AI连接游园**，可以调用你的APK或Linux程序中的工具；**游园AI连接外部MCP**，是在AI工作区添加其他工具服务器。这两种连接不是同一个设置。

## 外部AI连接游园

### 使用自己的临时公网地址

1. 打开设置 → 公网远控与MCP → 临时公网MCP隧道。
2. 点击“开启临时MCP隧道”，等状态显示实际连接成功。
3. 复制界面显示的**实际公网MCP地址**和**连接密钥**。域名由服务实际分配，不要照抄旧截图里的域名。
4. 在支持URL接入的MCP客户端中添加服务器：名称随意；传输选择**Streamable HTTP**；URL填刚复制的完整地址，包含`/mcp/`。
5. 设置请求头：`Authorization: Bearer 你的连接密钥`。有独立Bearer密钥输入框时只填密钥，避免重复写Bearer。
6. 保存并连接，查看Tools和Resources。先调用只读的游戏连接状态，确认连接到了自己的账号，再调用游戏操作。

手机/程序与隧道进程都需要保持运行。临时地址会过期或变更，重连后要核对新地址。免费临时服务不保证持续可用。不要将MCP密钥放进QQ群、插件源码或URL查询参数。

### 使用游园的稳定地址

地址：`https://toolbox.youyuanqi.dpdns.org/mcp/`。密钥使用**游园平台账号的云端MCP控制密钥**，不是游戏令牌，也不是本机MCP密钥。

先在APK或Linux程序登录同一个游园平台账号，让设备公网连接显示“已连接”；外部客户端再使用稳定地址与平台控制密钥。云端是中继，设备没上线时不能执行游戏工具。各平台用户只能访问自己的设备。

### 在同一台电脑或Linux服务器连接

程序运行后，本机地址通常为`http://127.0.0.1:19429/mcp/`，使用界面显示的本机MCP密钥；端口以设置中的实际值为准。

`127.0.0.1`只代表客户端所在的那台机器。你电脑上的AI不能通过这个地址直接访问另一台云服务器。云服务器可以使用公网隧道、稳定中继，或SSH端口转发。主面板只监听本机回环地址，不要把私有设置页面作为无认证公网服务发布。

## 游园AI连接外部MCP

进入对应语言的AI帮写工作区 → MCP服务器管理 → 添加服务器；填写外部服务URL和它要求的令牌，启用后查看Tools/Resources。命令链、Python、C++、Lua工作区分别管理上下文和可用工具。

按需操作：`mcp_list`查看简短目录 → `mcp_describe`读取需要的一个工具的参数 → `mcp_call`调用。需要资料时才读取指定Resource。不要把全部工具手册一次加载到提示词里。群助手的外部MCP在群服互通助手设置里配置；成员不拥有管理员的操作工具。

MCP不负责模型推理。DeepSeek等模型需要单独配置API地址、密钥和模型ID；模型不能连接或超时，要先测试模型接口，再检查MCP。工具参数和结果仍会占用模型Token，按需文档只是减少不必要上下文。

## 最小检查顺序

客户端正常时会依次进行`initialize`、`notifications/initialized`、`tools/list`。再用工具`api_call`，参数为`{"path":"/api/bot/status","method":"GET"}`查询游戏连接状态。看到工具列表只代表MCP连通，不代表机器人已经进服。

Resources中可以按需读取`youyuan://docs/python-runtime`、`youyuan://docs/cpp-runtime`、`youyuan://docs/command-chain-api`、`youyuan://docs/physical-actions`等内置开发文档。

## 常见问题

| 现象 | 检查方式 |
| --- | --- |
| 401 | 地址对应的密钥是否正确；有没有重复Bearer；是否混用了平台/本机/游戏密钥 |
| 404 | 是否保留`/mcp/`；临时域名是否已经失效 |
| 浏览器打开MCP地址出现405 | 当前服务不提供GET事件流；应由客户端通过HTTP POST调用，不能把打开网页当作工具测试 |
| 503设备未连接 | 查看APK/Linux程序的公网设备连接状态与网络 |
| Tools可见，游戏操作失败 | 检查游戏是否连接、账号权限、目标是否已观测、距离限制与工具实际错误 |
| 客户端只支持旧式SSE | 选择支持Streamable HTTP的连接方式；本临时通道不提供旧式SSE双端点 |
| 临时域名变了 | 从运行中的设备复制新地址；不能认为上次保存的地址仍有效 |

危险命令、方块修改、重载插件都应限定到用户要求的对象与范围。超时后先查状态，不要自动重复发送可能已经执行过的操作。


## 0.10.11：Grok / 只有网址输入框的客户端

设置 → 账号联通与连接授权 → MCP专用连接。选择“查询状态和开发文档”或“管理”，有效期默认720小时，可设1–8760小时；每个平台账号/本机执行端最多128个同时有效的专用连接。这是游园自己的管理上限，不是Cloudflare每天创建域名的限额。

生成 `https://域名/mcp/link/独立随机凭证`，客户端无需单独填密钥；网址本身就是凭证，不应公开。云端链接绑定创建时的设备，不允许用查询参数切换；权限与订阅到期每次检查。列表可撤销；云端完整链接只在创建时显示一次。原始 /mcp/ + Bearer 方式仍保留。

本机API `GET/POST /api/youyuan/mcp-links`，云端 `POST /api/identity/mcp-links`：`{op:"create",scope:"read"或"control",hours:720,name:"Grok",device:"linux-server"}`；云端撤销 `{op:"revoke",id:"记录ID"}`；本机撤销 `{op:"revoke",token:"专用凭证"}`。本机创建不启动隧道；若尚无公网域名，先开启临时隧道，再把返回路径拼到真实临时域名下。

只读连接仅提供连接状态、在线观测、插件运行状态、SDK目录与开发文档；不读取项目文件、不运行命令、不修改设置。“管理”连接开放完整当前节点工具，包含危险命令，交给AI前请明确选择该权限。

Grok → Connectors → New Connector → Custom，粘贴完整专用网址，使用 Streamable HTTP。游园返回JSON，无旧版SSE依赖。若客户端强制要求OAuth而不接受已认证URL，则需另做OAuth接入；本版没有伪称实现OAuth。服务端握手与工具调用可以验收，Grok界面最终兼容仍需该客户端实际添加测试。

Cloudflare Quick Tunnel 官方公开的限制是200个同时在途请求、超限429、不支持SSE、无在线SLA。未公布固定每天/历史创建数量，因此不能承诺无限；频繁新建可能被申请429限流。保持已有隧道、不反复重建，长期挂机选择固定域名/命名隧道。Pinggy免费会话另有时长限制，以提供商为准。

参考核对来源（供维护者，不自动加载AI全部内容）：https://docs.x.ai/grok/connectors 、https://docs.x.ai/grok/connectors/custom-mcp-tunneling 、https://developers.cloudflare.com/tunnel/get-started/quick-tunnels/ 。


## 0.10.13 Chat OAuth 与 Linux
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
