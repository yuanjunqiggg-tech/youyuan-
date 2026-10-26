# 安全与缺陷审查（2026-10-23）

审查范围：`youyuan_bridge_api.go`、`youyuan_seed_api.go`、`youyuan_qq_serverinfo.go`、`youyuan_qq.go`、`youyuan_qq_login.go`、`youyuan_commands.go`、`youyuan_account_pool.go`、`youyuan_qq_caps.go`、`youyuan_qq_assistant.go`、`youyuan_qq_actions.go`，以及三个前端文件。
只读审查，未改任何业务代码（临时测试 `security_tmp_test.go` / `security_tmp2_test.go` 已删除）。

## 结论（能否上线）

**不建议按当前状态上线**，下面 3 条必须先修：

1. `/api/bridge/rules` + `/api/bridge/emit` 组成一条**完全不校验令牌**的"落规则 → 触发 → 服内执行指令"链路（高危 H1）。
2. `yyBridgeTokenOK` 的本机兜底在云中继后面**恒为真**（RemoteAddr 永远是 127.0.0.1），默认配置下 `game.command` / `game.chat` 等于没有鉴权（高危 H2）。
3. `/api/qq/server/say` 的黑名单可以用 `minecraft:` 命名空间前缀绕过，且名单本身缺 tp/give/setblock/gamemode/summon 等（高危 H3）。

修完这三条，中危项（cap 越界 panic、`"<nil>"` 字面量、无锁读 Seq、非原子写命令链）可以下个迭代处理。

---

## 高危

### H1 `/api/bridge/rules` + `/api/bridge/emit`：无令牌的指令执行链（RCE）

- 位置：`upstream/youyuan_bridge_api.go:485-552`（规则写入，**全程没有 `yyBridgeTokenOK`**）、`:436-457`（emit，同样没有）、`:219-227` → `:232-243`（`game_command` 直接走 `SendCommandNoWait`，**没有任何黑名单**）。
- 对比：`/api/bridge/call` 的 `game.command` 至少要求令牌（`:397-401`），但同一份规则文件里的 `game_command` 动作不需要。
- 攻击路径（三步，无需任何令牌）：
  1. `POST /api/bridge/rules`，body `{"rules":[{"id":"x","enabled":true,"event":"player.join","action":{"type":"game_command","command":"op attacker"}}]}` → 200，已落盘 `bridge-rules.json`；
  2. `POST /api/bridge/emit`，body `{"event":"player.join","player":"a"}` → 200；
  3. 规则命中 → `yyBridgeGameCommand("op attacker")` → `bm.SendCommandNoWait`。
- 实测（`TestTmpBridgeRulesNoAuth`）：两条请求都返回 200；规则原文被接受；emit 返回 `seq:1`（本机未连游戏，因此卡在"游戏机器人未连接"，链路已通）。
- 谁能打：任何能到达 `/api/*` 的调用方——面板会话/控制台密钥、共享节点上被限制为"只能管账号池"的订阅者（见 H2 第 2 段）、本机任意进程与插件。
- 修法（一句话）：`/api/bridge/rules`、`/api/bridge/emit`、`/api/bridge/events` 全部加上与 `game.command` 同一把令牌校验，并且 `yyBridgeRun` 里的 `game_command` 再走一次 `yySrvInfoCommandGuard`。

### H2 反代之后 `yyBridgeTokenOK` 的本机兜底等于"永远通过"

- 位置：`upstream/youyuan_bridge_api.go:258-273`（`want == ""` 时只信 `RemoteAddr`）。
- 事实链：
  - Cloudflare Worker 对所有 `/api/*` 做鉴权（`cloudflare/worker.mjs:86-98`，要求 `Authorization: Bearer <CONSOLE_TOKEN>` 或有效身份会话），然后整包转发给中继 DO（`:142`）；
  - 中继把请求发给设备，设备收到后**重新构造**一个本机请求：`upstream/youyuan_remote.go:583` `http.NewRequestWithContext(ctx, p.Method, "http://127.0.0.1:"+port+p.Path, ...)`，并把 `Authorization` 覆盖成设备自己的 key（`:593`）；
  - 于是所有经公网来的请求，`r.RemoteAddr` **恒为 `127.0.0.1:<随机端口>`**，`yyBridgeTokenOK` 直接返回 true。
- 实测（`TestTmpBridgeTokenFallback`）：`yyBridgeTokenOK(RemoteAddr="127.0.0.1:xxxx")` = **true**；同一请求改成 `8.8.8.8` 才为 false。
- 放大条件：`Assistant.MCPToken` 没有默认值（`youyuan_qq_assistant.go:42`，全仓库无生成逻辑），用户不填就一直为空 → `game.command` / `game.chat` 对**所有面板调用方**开放，只有 Worker 那层登录在挡。
- 第二段（租户策略绕过）：`upstream/youyuan_accounts.go:265-321` 把 `/api/youyuan/accounts/invoke/<id>/...` 转发给子内核；子内核收到的同样是 localhost 请求，且子内核的 `MCPToken` 也是空 → `POST /api/youyuan/accounts/invoke/<自己的账号>/api/bridge/call` 带 `capability:"game.command"` 不需要令牌。而 Worker 本意是共享节点上的非管理员**只能**用账号池接口（`cloudflare/pool-identity.mjs:16-31`，`sharedPoolAllowed` 对 `/api/bridge/*` 返回 false）。也就是说：受限订阅者可以借 bridge 在共享游戏服里执行任意指令。
- 修法（一句话）：删掉 RemoteAddr 兜底；`MCPToken` 为空时直接拒绝危险能力（或首次启动自动生成随机令牌写回配置），并且在 `yyBridge`/`yySrvInfo` 里用 `X-YouYuan-Actor` 判断"是否共享租户"。

### H3 `/api/qq/server/say` 黑名单可绕过 + 无令牌

- 位置：`upstream/youyuan_qq_serverinfo.go:538-593`（`yySrvInfoDangerRules` / `yySrvInfoCommandGuard`）、`:207-213`（注册路由，`say` 无任何令牌校验）、`:655-741`。
- 实测绕过（`TestTmpSrvInfoGuardBypass`，全部返回 nil = 放行）：

  | 输入 | 结果 |
  |---|---|
  | `/op Steve`、`/Op Steve`、`/  op Steve` | 拦住了（大小写/空格处理正确） |
  | `/stop`、`/stop\n/op Steve` | 拦住（换行由 `yySkillGuardCommand` 挡） |
  | `/execute as @a run op Steve` | 拦住（`execute ... run` 剥壳有效） |
  | **`/minecraft:op Steve`** | **放行** |
  | **`/execute as @a run minecraft:deop Steve`** | **放行** |
  | **`/give Steve diamond 64`** | **放行** |
  | **`/setblock 0 0 0 bedrock`** | **放行** |
  | **`/gamemode creative Steve`** | **放行** |
  | **`/function evil:op`** | **放行** |

- 原版 1.13+ 接受 `namespace:path` 形式的命令名，`minecraft:op` 与 `op` 等价。因此"禁止给人 op / 禁止停服"这两个最关键的保护可以一个前缀绕开。另外名单本身缺 `tp`（`/tp @a 0 -100 0` 可团灭）、`kill @e`、`summon`、`effect`、`whitelist add`、`setworldspawn`。
- 为什么算高危：这条接口的注释自己写着"面板调用者已经过设备令牌认证，等同于超级管理员"（`:535`），但实际**没有令牌校验**，且唯一的兜底（黑名单）可绕。
- 修法（一句话）：命令名先 `strings.TrimPrefix(body, "minecraft:")` 再判，并加 `tp/kill @e/summon/gamemode/effect/whitelist add/function/setblock/fill` 前缀；同时给 `/say` 的 `mode=command` 加 `yyBridgeTokenOK`。

---

## 中危

### M1 `CapLimits` 越界 → 面板持久 panic（index out of range）

- `upstream/youyuan_qq_caps.go:144`：`[]string{"成员","管理员","超级管理员"}[c.MinLevel]`；`upstream/youyuan_qq_actions.go:131` 同样写法。
- `yyQQCapLimitNormalize` 只在 `/api/qq/caps` POST 里调用（`:280`），而 `/api/qq/config`（`youyuan_qq.go:108-163`）是 `yyMergePatch` + `json.Unmarshal`，**不夹紧** `cap_limits`。写 `{"assistant":{"cap_limits":{"like":{"min_level":5}}}}` 之后，`GET /api/qq/caps`、`GET /api/qq/actions`、以及任何 `yyQQRunAction` 都会 panic。
- 实测：`yyQQCapsView` → `runtime error: index out of range [5] with length 3`。net/http 会吞掉 panic 只断这条连接，但面板会一直 500，必须手改 `qq-bridge.json` 才能恢复。
- 建议：在 `yyQQNormalizeConfig` 里也跑一遍 `yyQQCapLimitNormalize`，下标取值改成 `yyQQLevelText(n)` 带越界兜底。

### M2 `str()` 把缺失参数变成字符串 `"<nil>"`

- `upstream/youyuan_bridge_api.go:338-343`：`fmt.Sprint(p.Params[k])`；只有 `p.Params == nil` 才返回空串，`params:{}`（对象存在但没有这个键）会返回 `"<nil>"`。
- 实测：`{"capability":"qq.send","params":{}}` → **通过了 `text == ""` 校验**，进入发送流程（本机因"目标群未启用"才失败）。`game.command` 同理，会执行 `SendCommandNoWait("/<nil>")`。
- 建议：改成类型断言 `if v, ok := p.Params[k].(string); ok`，其它类型统一按空串处理。

### M3 并发与内存

- **无锁读 `Seq`**：`upstream/youyuan_bridge_api.go:481` 在锁外读 `yyBridge.Seq`，而 `:119` 在锁内自增。Android 32 位 ARM 上 int64 可能撕裂。本次无法用 `-race` 证实（见"竞态检测结果"），但静态即可确认是数据竞争。修法：在 `:468-470` 的加锁块里一起读出 `seq`。
- **`marks` / `lastFire` 只增不减**：`:163`、`:175-176` 只把 `marks[key]` 置 nil，key 永远留在 map 里；`lastFire` 从不清理。配 `per_player:true` 的规则时按 `rule|player` 线性增长，长期运行是内存泄漏。修法：清理时 `delete(yyBridge.marks, key)`，并按阈值整体重建 map。
- **动作在锁外执行**：`:154-179` 判断完就放锁再 `yyBridgeRun`，两条同 key 事件并发时（`threshold=1`、`CoolSeconds=0`）会重复执行同一动作。属于"多发一次"，不是越权，但限流目的落空。修法：在锁内打"正在执行"标记。
- **限流无竞态**：`yySrvInfoSayAllow`（`:638-653`）先查后写都在同一把锁内，**没有**竞态问题。
- **OneBot 心跳 vs 读循环**：`conn.Ping`（`:394`）与 `conn.Read`（`:405`）、`CloseNow`（`:398`/`:458`）并发是安全的——coder/websocket v1.8.14 `conn.go:32-34` 明确写 "All methods may be called concurrently except for Reader and Read"。结论：**不是 bug**，无需改。

### M4 `/api/qq/login/refresh`：无鉴权 + 打断式重启

- 位置：`upstream/youyuan_qq_login.go:155-175`。无令牌，任何人可刷；`exec.CommandContext(ctx, "sh", "-c", cmdLine)` 用的是 `r.Context()`（`:167`），**浏览器关页/超时会把 `systemctl restart napcat` 杀在半路**，可能把 NapCat 停在已停止状态；连续触发则是重启风暴。
- 命令注入本身：`YY_NAPCAT_RESTART_CMD` 只来自环境变量，全仓库没有 `os.Setenv` 写它（`grep` 确认），接口无法影响 → **不是注入**。但 Lua 插件可用 `os.setenv`（`third_party/gopher-lua/oslib.go:183`）改写环境变量，插件本来就有全权限，属同级信任，不额外升级。
- 建议：加令牌；把 `r.Context()` 换成 `context.WithTimeout(context.Background(), 20s)`，避免客户端断开截断重启。

### M5 命令链 `save` 非原子写、`delete` 无服务端二次确认

- `upstream/youyuan_commands.go:953` 直接 `os.WriteFile` 覆盖源文件，断电/磁盘满会留下半截链（bridge 的规则保存用了 `tmp+rename`，这里没有）。
- `delete`（`:973-1004`）只有前端 `window.confirm`（`youyuan_chain_center.js:234`），服务端一次调用即删；MCP/插件误调无法回滚。运行中删除先 `yyChainStop`，12 秒超时会返回错误并**保留文件**（`:987-992`），不会出现"文件删了进程还在跑"。
- 误删非命令链文件：**不会**。`yyChainName`（`:335-344`）只允许 base name + `.txt/.mcfunction`，且拒绝 `.` 开头；`bridge-rules.json` / `qq-bridge.json` 扩展名和目录都不同（命令链根目录是 `插件数据文件/ds_ai_agent/command_txt`）。
- 建议：保存改成 tmp+rename；delete 增加 `confirm:true` 字段。

### M6 令牌展示与身份降级

- `yyPoolMaskToken`（`upstream/youyuan_account_pool.go:170-178`）：<12 位整体打码（**安全**，不会等于明文）；但 12~13 位令牌会露出前 4 + 后 4，只剩 4 位未知，可穷举。建议 ≥16 位才露头尾，否则全掩。
- `/api/youyuan/accounts/pool/secret`（`:678-704`）：鉴权走 `yyPoolActorRequest`，逻辑上只能读自己或管理员的账号；但 `youyuan_pool_identity.go:50-56` 在**没有任何 actor 头**时直接返回本机已绑定账号（通常是 owner/admin）。而 Worker 对 legacy device token / console token 的请求会主动删掉 actor 头（`cloudflare/worker.mjs:138-141`），于是"谁拿着设备密钥"都等价于 owner，租户隔离在这条路径上不存在。
- 令牌不进日志：`grep` 确认 `yyQQLog`/`fmt.Printf` 均未打印 token / Authorization；响应带 `Cache-Control: no-store`。这点没问题。
- 建议：`pool/secret` 额外要求 `X-YouYuan-Actor` 必须存在（无 actor 头直接 403）。

---

## 低危 / 加固建议

1. **前端 `esc()` 的降级分支不安全**：`youyuan_qq_messenger.js:18-20`、`youyuan_qq_serverinfo.js:33-35` 在 `yyQQE` 不存在时退化为 `String(v)` 原文。当前 `index.html` 里 `youyuan_qq_console.js`(156) 先于两者(159/160) 加载，且 `yyQQE`→`yyEscape` 转义了 `&<>"'`，**现在没有 XSS**；但只要脚本顺序变化或某天改名，就是直接 XSS。建议降级分支也做一次本地转义。
2. 逐点核对过三个文件的插值：messenger 的 `<option value>`、`<textarea>` 内容、日志条目，serverinfo 的 `src`、`value`、`datetime`、`title` **全部走了 esc**；`chain_center.js` 的删除/启动按钮用 `data-name` + 事件委托（`:100`、`:174-185`），**没有**把文件名拼进 `onclick`，文件名含 `'` 也不会破字符串。这块结论：安全。
3. `/api/qq/login` 会返回服务器绝对路径 `qrcode.path`（`youyuan_qq_login.go:122`，如 `/opt/qqbot/napcat/cache/qrcode.png`），轻微信息泄露，建议只返不返。
4. seed 接口的整数溢出：`yySeedParamInt` 用 `Atoi` 收到接近 int64 上限的值后，`:584` 的 `toX-fromX` 会溢出成负数从而绕过 4096 检查（实测 `from_x=MaxInt, to_x=-MaxInt` 返回 200）。因为有 6 万区域 + 150ms 双重上限兜底，**没有实际 DoS**，但建议对单轴坐标先做绝对值上限校验。`x` 转 `int32` 会截断，给出错误坐标（不影响安全）。
5. `/say` 的 3 次/秒是**全局**计数（`yySrvInfoSayGate`），一个面板用户可以把其他人一起限流；改成按调用者更合理。
6. `yyBridgeGameChat` 的 `{player}` 由 emit 传入，可拼出以 `/` 开头的文本并塞进 `packet.Text{TextTypeChat}`（`bot_conn.go:1230`）。是否被服务端当命令执行取决于目标协议版本，**未能实测**，建议入口处拒绝以 `/` 开头的渲染结果。
7. 新增 handler 都没有 `recover`（bridge / serverinfo / login / caps）。单请求 panic 会被 net/http 兜住不会进程崩溃，但会断连接且无错误体；建议统一加一个包装。

---

## 竞态检测结果

- 本机无 C 编译器：`go test -race` 失败（`cgo: C compiler "gcc" not found`，`CGO_ENABLED=1` 同样失败），**未能跑数据竞争检测**。
- 普通模式 `go test -run 'TestTmp'` 全部通过，无 panic（除 M1 主动触发的那一次）。
- 静态确认的竞争点只有一处：`youyuan_bridge_api.go:481` 无锁读 `yyBridge.Seq`（见 M3）。其余计数器（`yySrvInfoSayGate`、`yyQQActions`、`yySrvInfoBot`）都在锁内读写；`coder/websocket` 的并发语义已按官方注释核对，心跳与读写并发是允许的。

## 未能验证的点

1. **真正的端到端 RCE**：本机没有连上游戏客户端，只验证到"规则落盘 → emit 200 → 进入 `SendCommandNoWait`"（那里返回"游戏机器人未连接"）。生产上这一步会真发指令。
2. **共享租户借 bridge 逃逸**（H2 第二段）：需要父子账号内核同时在线才能跑通，只做了代码路径核对（`youyuan_accounts.go:265-321` + `pool-identity.mjs:16-31`）。
3. **`minecraft:` 前缀在目标服务端是否被接受**：原版 1.13+ 接受，实测环境没有服务器；建议在测试服上跑一次 `/minecraft:op <bot>` 确认。
4. **`Text(TextTypeChat)` 里以 `/` 开头的文本是否会被当命令**：取决于服务端协议版本，未实测。
5. **Cloudflare 侧的真实 IP 头**：Worker**不注入** `CF-Connecting-IP` / `X-Forwarded-For` 到中继包（`worker.mjs:192` 只传固定几个头），设备端也不读这些头。所以"用真实 IP 替代 RemoteAddr"在当前架构下**不可行**，只能改成显式令牌。
