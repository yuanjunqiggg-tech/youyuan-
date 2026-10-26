# 游园 MCP 接入教程

## 0.10.21：普通 Chat 实际接入与文档发现

2026-10-08 在普通 Chat 的“游园 Linux 工具箱”应用中完成真实 OAuth 登录，页面显示 Primary 已连接。当前 Chat 已实际调用 api_bot_status 和 api_plugin_api_doc，返回真实游戏连接状态及 Lua 开发手册；这不是只通过 SDK 或网页说明的结果。

应用详情中的“连接”完成账号绑定，随后“在聊天中试用”会附加应用。授权页仅放行本次已登记的 ChatGPT/本机回调；密码仍只提交到游园，S256 PKCE、resource、订阅和节点权限仍逐项检查。正式客户端若等待超时，必须回到应用重新连接，不使用旧本机 callback。

官方稳定客户端 https://chatgpt.com/oauth/client.json 已按2026-10-08官方元数据预登记，避免服务器出网不可达造成发现失败；仅精确官方 ID 和回调被接受，不因此自动授予任何权限。动态注册 DCR 仍保留。

api_youyuan_docs 不传参数时返回真实文档资源目录。读取一份文档时参数为 {"query":{"language":"mcp-connection"}}，Python/C++/Lua 分别用 python-runtime/cpp-runtime/lua-runtime。读取单个章节可传 query.section；接口可传 query.method。不要把 language 或完整 URI 填到顶层。工具的 inputSchema 已同步这些参数。支持 MCP Resources 的客户端也可直接读取 youyuan://docs/mcp-connection。

先通过 api_plugin_list 取得真实插件 ID，再读取文件：query={"id":"实际ID","scope":"code","file":"modules/core.lua"}。main.lua 可能只是模块加载器；源码读取没有把 modules/ 删掉。测试插件实际ID ds_ai_agent_r21b_test 与自定义插件名称并不相同。修改用 api_plugin_file_write 的 POST/body，并在修改后读回和按需求热重载；不要凭名称假定目录。


## 0.10.20：OAuth 客户端兼容与协议错误

普通 Chat / Codex 推荐地址：`https://toolbox.youyuanqi.dpdns.org/mcp/oauth/linux-server`，选择 OAuth，以自己的游园平台账号登录。账号不等于游戏令牌，授权只作用于指定节点；管理员和有效订阅、节点分配、只读/管理权限逐请求检查。

服务支持两条真实 OAuth 路径：动态客户端注册（DCR，供桌面本机回调等使用），以及 ChatGPT 官方 HTTPS 客户端元数据身份（CIMD）。后者只从 chatgpt.com 的官方 client.json 路径读取并校验 client_id、回调和 none 公共客户端方式；不获取任意用户提供的网址，不跟随重定向。二者都使用 S256 PKCE 和完整 resource 绑定。

密码错误会在授权页面显示错误，并保留当前授权事务与重试/取消按钮，不回显密码。页面事务20分钟，不能恢复已经超时退出的客户端；客户端退出后必须从客户端重新连接。

Streamable HTTP 支持 POST；不提供 GET SSE 流时返回405，合法通知返回202且无响应体。initialize 仅选择已支持的 2025-03-26 / 2025-06-18；不能盲目原样回传不支持的版本。无效 JSON、方法、工具、参数、账号会返回标准 JSON-RPC error；工具实际执行失败仍由 result.isError 表示。

Tools 可见 ≠ OAuth 已完成 ≠ 当前普通 Chat 已挂载工具。请分开验收：initialize / tools/list → OAuth → api_bot_status → 真实文件读写 → 当前普通 Chat 实际操作。若当前 Chat 没有工具，不能以网页工具说明冒充执行成功。


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


# QQ邮箱注册与账号注销 · 0.10.22

注册平台账号：填写3–32字符账号名称、自己的QQ邮箱（@qq.com）和10–128字符密码，点击发送验证码，在邮箱收件箱/垃圾箱查收6位验证码，输入后注册。验证码只对应本次账号名和邮箱；10分钟过期、一次有效，错误5次必须重发，60秒后可重发。真实SMTP未接受时不显示发送成功，也不会创建账号。

`POST /api/identity/register-code`：`{username,email}`；成功返回expires_in、retry_after，不返回验证码。`POST /api/identity/register`：`{username,email,email_code,password}`。APK/Windows本机通过`POST /api/youyuan/identity`加上`action`调用相同流程。旧账号名/密码登录保留，旧QQ号不会被当作已经验证的邮箱。

退出登录只结束当前会话，平台账号仍然存在。真正注销在设置→游园平台账号→注销平台账号，输入当前密码并确认；云端撤销所有会话、设备和控制凭证、OAuth和MCP授权并释放用户名/邮箱。APK清除对应平台绑定；本机游戏账号、插件源码和数据不删除。注册后是新用户，原订阅和云端资料不会自动迁移。项目所有者需要先交接，不能自助注销。云端暂时离线不会自动清除本机归属。

邮箱发信运行在Linux执行端，Cloudflare仅校验验证码和签名转发。配置文件`/var/lib/youyuan/mail-config.json`须由youyuan用户拥有并设0600：smtp_host、smtp_port（465隐式TLS或587强制STARTTLS）、smtp_username、smtp_authorization_code、relay_secret。relay_secret对应本站私有CONSOLE_TOKEN。配置只在服务器读取，不打入APK、不公开、不返回给MCP。QQ邮箱开启SMTP后使用授权码，而非QQ登录密码。应用接口不能用来任意发邮件：只接受本站签名、限定QQ收件邮箱和6位注册验证码。

AI或MCP不得把退出描述成注销，不得在注册冲突时直接删除已有用户，不得以未验证的号码声称邮箱归属。管理员可以重置账号密码；密码只保存哈希，不支持反查原密码。


# MCP联调约定与真实状态 · 0.10.57

宿主提供连接、原始包、账号进程、文件、回调队列和重载；插件实现血条/HUD、业务与跟随策略。API存在不是服务器全局广播保证。摄像机、OP也不保证跨维度目标包。三个常驻旁观按实际连接维度0/1/2分工；有受控下界远距无OP来源20/20记录，不承诺任意真人零丢包。

## 调用格式
MCP工具顶层account为空/main是主号，独立账号用账号池id。method是HTTP方法，query是GET查询参数，body是POST原始对象；游戏方法放body.method，参数放body.args。
例：api_call({path:"/api/youyuan/game/call",method:"POST",account:"main",body:{method:"bot.self",args:[]}})。
game.execute / sendCommandWithResp是确认通道；sendCommandFast只发不等，不代表执行成功。高频FUNCTION不要等待逐条确认；有真实成功依赖的条件才设置确认边界。

## 摘要与原始事件
MCP GET sense/snapshot、sense/events、packets/capture默认view=summary，返回bot简要位置/血量、计数和最近20条元数据。query支持limit、types逗号列表、runtime_id、after_sequence；规范事件还支持entity_type。过滤不创造未收到数据；counts来自保留窗口。完整内容明确query.view=raw；大文件用packets/export下载，不放AI上下文。
Lua events.summary({types={"swing","health"},limit=20})；Python host.events.summary({"types":["swing","health"],"limit":20})；C++ client.events.summary(yy::json{{"types",{"swing","health"}},{"limit",20}})。抓包摘要用events.captureSummary同样参数。
packet.catalog()列所有注册包ID/名称/字段数/是否能抓包。每个普通已解码包可用events.subscribe("包名",handler)；自定义转换events.define({name="my_event",packet="Animate",fields={target="entity_runtime_id"}},handler)。登录/握手隐私数据不进入抓包；大区块包不是小动作事件。UpdateAttributes是真实health来源；Animate只代表实际动作包，不用伤害伪造空挥；ActorEvent不保证带可靠攻击者。所有字段用packet.schema或/api/youyuan/packets/schema?name=Animate按需读取。

## trace与失败恢复
每次MCP工具调用structuredContent.mcp_meta含trace_id、http_status、elapsed_ms、outcome、retryable、execution_confirmed、auto_retried=false。公网中继同trace ID跟随到宿主；GET /api/youyuan/mcp/traces?trace_id=...读取最近元数据。保留256条内存记录，只含本人/管理员可读范围，不存密钥、命令正文、源码或聊天提示词。
not_sent表示中继未提交，可重试；timeout_unknown/write_result_unknown表示写操作可能已执行，必须读任务/状态后再决定，不盲目重复放置、计分或兑换。server_rejected表示收到明确失败回包，submitted_unconfirmed不是执行成功。云端503/504会带submitted/confirmed/retryable；不偷偷重试非幂等写。

## 长NBT操作
优先building.placeContainer(x,y,z,"minecraft:barrel",Items,options)返回job_id，再building.nbtJob(job_id)查询状态与verified。会占用真实作业位置、可能清理制作背包，先检查说明并暂停观察运动；不能在5秒同步调用里假装完成整条NBT自测。placeNBTBlock旧同步接口仍保留兼容，超时结果可能未知。当前没有统一覆盖所有旧同步方法的持久异步任务；重启后内存NBT任务不恢复。

## 账号池最小成功路径
1. accountsPool.list({all=false})获得自己账号id；has_token只证明存了凭据。
2. accountsPool.start(id,{server="服务器标识",connect=true})启动；不是“请求成功就进服成功”。
3. accountsPool.status(id)轮询engine_ready/connected/phase/fleet_ready。只有connected=true才验证当前server，credential_state=verified_for_current_server。
4. accountsPool.invoke(id,{path="/api/youyuan/game/call",method="POST",body={method="bot.self",args={}}})调用该账号。
/api/fleet是旧bot_tokens同进程池，空[]不代表账号管理池没有机器人。4399平台注册/启动器绑定/能进入网易游戏是不同步骤；宿主不能根据存有token就声称全部注册通过。

## 群服互通
死亡和重生不是退出/进入。名单以成功/list的名字为准，坐标查询失败不注销在线状态；周期事件只刷新名单，不逐玩家同步坐标。真正死亡/重生的事件仍由实际包与缓存处理。
.日志查游戏最近聊天；.群日志 [1..80]查机器人真实收到的近期QQ群消息，默认20，可分段。不是QQ全部历史或原生合并转发卡。普通成员仍隐藏坐标。
能发送图片不等于能理解表情包；当前图片发送/绘图配置可用，图像语义识别和表情图库尚未实现，不能让文字模型假装看图。QQ官方窗口/权限仍有效。

## 源码边界
公网宿主源码、SDK压缩包、Git目录、source map等下载路径已拦截；MCP资源来自本机已部署文档，不自动连接GitHub。玩家自己的插件项目仍可读写/导出，不能为隐藏宿主而砍掉开发功能。网页HTML/JS必须发给浏览器所以可查看；APK二进制也不能承诺无法逆向。源码审计不能替代多用户越权与部署目录完整验证。
