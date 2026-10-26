# 「刷新连接」后服务器号/账号状态显示很慢 —— 链路实测报告

测量时间：2026-10-21 03:35–03:45 CST　测量人：性能分析智能体（只读，未改任何代码/配置）
线上入口：`https://toolbox.youyuanqi.dpdns.org`（Cloudflare Worker → RELAY Durable Object → WebSocket → Linux 47.102.156.125 → 127.0.0.1:19430 → 19429）

> **适用范围（请与 perf-plugin-list 那份报告区分阅读）**
> 本报告只解释「连接与设置」页的连接卡片（`#cfg-conn-card`）里**服务器号 / 账号状态显示慢**：瓶颈是 5 个 API 串行 3 波、单趟公网往返 0.5–1.05 s，静态资源已实测不是瓶颈（边缘入口整页 load 仅 1.53 s），所以**「聚合接口」排第一**。
> `perf-plugin-list-report.md` 解释的是「插件」标签页（`#tab-tasks`）打开慢：瓶颈是**首屏 139 个请求**（`index.html` 静态引用 103 个 = 85 JS + 18 CSS，另约 36 个由运行时动态拉取；两个口径都对，103 可对着代码数、139 是 `performance.getEntriesByType('resource')` 实测值）+ 插件面板多次 API 往返，所以**「合并 bundle」排第一**。
> **两份报告的排名不可合并成一张表**，但根因相同：公网每次 API 往返 0.5–1.1 s 太贵，凡是要连续打多个接口的视图都应合并成一个聚合接口。

---

## 一句话原理

**接口本身 1 毫秒就返回，慢的全在链路上**：浏览器每要一个字段都要单独走一趟「CF 边缘 → Worker → RELAY DO → WebSocket → Linux 本机 HTTP」，单趟 0.5–1.05 s；而「连接与设置」页首屏是 **3 波串行**（`/api/config` → `/api/bot/status` → `/api/qq/status`+`/api/youyuan/remote` → `/api/youyuan/connection-policy`），于是服务器号要等第 1 波（≈1.0 s）、"已连接"徽标要等第 2 波（≈1.5 s）、"正在读取连接策略…"那行要等第 3 波（≈2.6 s）才被替换。`server` 字段不是数据源慢——它在连上的同一瞬间就写好了。

> **两种"慢"要分开**：
> - **A. 页面渲染慢**（2.6 s）——链路 + 串行，见第一～四节，改聚合接口即可解决。
> - **B. 服务器号空白**（最长 78 s）——服务重启后机器人要 78 s 才重新进服，这期间 `server` 真的是空字符串。见第五节第 1 条。用户说的"刷新连接也一样"很可能同时撞上了 B。

---

## 一、实测数据表

### 1.1 同一接口：服务器本机直连 vs 线上经 Worker（各 3 次，秒）

| 场景 | #1 | #2 | #3 | 均值 |
|---|---|---|---|---|
| 服务器本机 `127.0.0.1:19429/api/bot/status` | 0.00093 | 0.00098 | 0.00118 | **0.0010** |
| 服务器本机 `127.0.0.1:19430/api/bot/status`（隧道入口） | 0.00168 | 0.00138 | 0.00131 | 0.0015 |
| 线上经 Worker，每次新建 TLS 连接 | 1.043 | 1.050 | 0.990 | **1.028** |
| 线上经 Worker，复用同一条连接 | 1.026 | 0.548 | 0.489 | **0.688** |
| 线上静态 `index.html`（复用连接，纯边缘基线，不经中继） | 0.796 | 0.310 | 0.488 | **0.531** |

- 新建连接拆解：`connect`=0.22–0.25 s（本机→CF 边缘 TCP），`tls`=0.46–0.49 s，业务响应再 0.5 s。
- **本机 1 ms → 线上 1000 ms，放大约 1000 倍**，与服务器负载无关。

### 1.2 线上各接口（每次新建连接，各 3 次，秒）

| 接口 | #1 | #2 | #3 | 响应体 |
|---|---|---|---|---|
| `/api/bot/status` | 1.046 | 1.095 | 1.064 | 114 B |
| `/api/config` | 1.394 | 0.925 | 1.046 | 791 B |
| `/api/youyuan/connection-policy` | 1.148 | 1.012 | 0.989 | 409 B |
| `/api/youyuan/remote` | 1.035 | 1.427 | 0.954 | 559 B |
| `/api/qq/status` | 1.009 | 1.020 | 1.076 | 177 B |
| `/api/youyuan/events?after=0` | 1.076 | 0.974 | 1.029 | **17 945 B** |

### 1.3 首屏瀑布（单连接，按前端真实顺序，秒）

```
/api/config              1.054   ← 服务器号输入框在这里才被填上
/api/bot/status          0.539   ← 顶部"已连接"徽标 / OP 状态
/api/qq/status   ∥      0.528   ← 前端 Promise.all 并行，只算一次
/api/youyuan/remote      0.524
/api/youyuan/connection-policy 0.488  ← "正在读取连接策略…"在这里才被替换
---------------------------------------------
合计 ≈ 2.61 s（冷启动首屏 3.1–3.5 s）
```

同一批接口在服务器本机合计 **≈ 6 ms**。

### 1.4 三条路径同接口对比（`/api/plugin/list`，热连接，同一会话）

| 路径 | #1 | #2 | #3 | 说明 |
|---|---|---|---|---|
| 边缘 401（不带令牌，**不经过任何跳**） | 0.856 | 0.316 | 0.294 | Worker `worker.mjs:93-95` 直接拒，`relay.fetch()` 根本没执行 |
| **中继全程**（带令牌，真的到 Linux，返回 4 KB 插件列表） | 1.096 | 0.523 | 0.531 | Worker→DO→WS→Linux→Go→回 |
| **直连隧道全程**（带令牌，同样返回 4 KB 插件列表） | 0.522 | 0.545 | 0.580 | CF→trycloudflare→gateway→Go→回 |
| 直连隧道 401（到网关为止，未打开上游） | 0.452 | 0.427 | — | `direct_gateway.py:69-71` 拒绝 |

**结论：中继 0.53 s ≈ 直连 0.55 s，两条路径同量级，差异在噪声内。** 直连并不更快，也不更慢。
网关自身开销约 **1.4 ms**（服务器上 `curl 127.0.0.1:19430/api/bot/status` = 1.68 / 1.38 / 1.31 ms，已含"每请求新建上游连接"），因此 `direct_gateway.py:86` 的 `urlopen` + `Connection: close` 在回环上不构成可测量成本。

> 注意区分"边缘 401"和"中继全程"：前者 0.29 s、后者 0.53 s，差的 0.24 s 才是中继那一跳。任何用 401 响应测出来的"中继耗时"都只是边缘 RTT，不能用来和直连做对比。

### 1.5 稳定性

经中继连续 12 次（间隔 2 s）请求 `/api/bot/status`：**12/12 成功，无 503、无超时**，耗时稳定在 0.95–1.19 s。→ 中继链路本身不丢包、不抖动，**纯耗时问题，不是故障**。

### 1.6 `server` 字段时序

12 次采样中 `connected` 与 `server` 同步翻转（第 4 次同时变为 `connected:true` / `server:"98014267"`），**没有任何一次出现"已连接但服务器号为空"**。→ `server` 不存在"重连后要等一会儿才有"的问题。

---

## 二、前端调用链路图

### 2.1 点「刷新状态」（连接卡片右上角那个按钮）

```
点击「刷新状态」
  └─ yyRefreshConnectionPolicy()                      youyuan_connection_policy.js:32
       └─ A('GET','/api/youyuan/connection-policy')   ← 只发这 1 个请求
            └─ 0.5–1.1 s
                 └─ yyDrawConnectionPolicy() 重绘那一行文字
```
**注意**：这个按钮**不会刷新服务器号**，服务器号来自 `/api/config`。用户点它看不到服务器号变化，只能等整页重渲染或 10 s 轮询。

### 2.2 进入/重载「连接与设置」（用户实际感受到的"刷新连接"）

```
tabTo('settings') → Rcfg()
│
├─ 渲染空壳 HTML（服务器号输入框空白、"读取中"徽标）      youyuan_settings.js:28
│
├─ 第 1 波：await refreshCfg(live)
│    └─ await A('GET','/api/config')                     pages/settings.js:23   ≈1.0 s
│         └─ 填 #bot-server.value = config.server_code   pages/settings.js:31   ← 服务器号出现
│    └─ 不 await：refreshCfgStatus() / refreshCfgAccount() / pollToolboxInfo()
│         └─ A('GET','/api/bot/status')                  pages/settings.js:55   ≈0.5 s
│              └─ U(connected,is_op,server) 顶部状态     pages/settings.js:56
│              └─ #cfg-conn-st = "已连接(OP)"            pages/settings.js:59-65 ← 账号状态出现
│              （refreshCfgAccount 又调一次 refreshCfgStatus，被 api.js:16 去重，不额外发请求）
│
├─ 第 2 波：await Promise.all([qq/status, youyuan/remote])  youyuan_settings.js:31  ≈0.5 s
│
└─ 第 3 波：await yyRefreshConnectionPolicy()            youyuan_connection_policy.js:53 ≈0.5 s
     └─ "正在读取连接策略…" 被替换成 "游戏已连接 · 自动重连开启 · …"
```

之后每 **10 s** 轮询一次：`setInterval(refreshCfgAccount,1e4)`（pages/settings.js:18 / youyuan_settings.js:31）→ 每次 2 个中继请求（`/api/bot/status` + `/api/youyuan/connection-policy`）。

另有常驻 **1.5 s** 事件轮询 `/api/youyuan/events`（17.9 KB/次），由前端替换掉原生 EventSource 实现（youyuan_boot.js:70-81）。

---

## 三、代码证据

### 前端

| 位置 | 内容 |
|---|---|
| `upstream/frontend/js/youyuan_connection_policy.js:50` | 页面里唯一的"刷新"按钮「刷新状态」，`onclick="yyRefreshConnectionPolicy()"` |
| `upstream/frontend/js/youyuan_connection_policy.js:32-35` | 该函数只 GET `/api/youyuan/connection-policy`，不碰服务器号 |
| `upstream/frontend/js/youyuan_connection_policy.js:43-53` | `Rcfg` 装饰器：`await` 完基类后才 `await yyRefreshConnectionPolicy()` → 策略文案被压到最后一波 |
| `upstream/frontend/js/youyuan_settings.js:25-32` | 设置页最终版 `Rcfg`；**30 行 `await refreshCfg(live)`，31 行 `await Promise.all([...])`** —— 串行根因 |
| `upstream/frontend/js/pages/settings.js:22-53` | `refreshCfg`：23 行 `await /api/config`；49–51 行三个未 await 的后续调用 |
| `upstream/frontend/js/pages/settings.js:54-66` | `refreshCfgStatus`：55 行 `await /api/bot/status`，56 行 `U(...,s.server)` 渲染服务器号 |
| `upstream/frontend/js/pages/settings.js:67-71` | `refreshCfgAccount` 内部又调一次 `refreshCfgStatus`（重复调用，靠 api.js 去重兜住） |
| `upstream/frontend/js/pages/settings.js:72-73` | `pollToolboxInfo` 是**空函数**（死代码），却仍被 setInterval 挂着 |
| `upstream/frontend/js/api.js:16,18` | GET 并发去重；**超时 300 s**（设备假死时页面可以空白 5 分钟） |
| `upstream/frontend/js/youyuan_boot.js:20-68` | 全局 fetch 包装：每个 `/api/*` 加 `Authorization` + `X-YouYuan-Device` |
| `upstream/frontend/js/youyuan_boot.js:70-81` | 用 `setTimeout(poll,1500)` 轮询 `/api/youyuan/events` 替代 SSE |
| `upstream/frontend/js/youyuan_boot.js:13-19` | 启动即发 `/api/cloud/linux-direct` 探测（当前恒返回 `use_relay:true`，白跑一趟） |
| `upstream/frontend/js/youyuan_dashboard.js:83` | 从子页返回 tab 时 `setTimeout(...,310)` 固定延迟 |

### 后端

| 位置 | 内容 |
|---|---|
| `upstream/main.go:1099-1120` | `handleBotStatus`：1100 行取**全局锁** `botMgrMu`，1107 行 `ServerCode()`；实测无锁竞争（本机 1 ms） |
| `upstream/bot_conn.go:1399-1403` | `ServerCode()` 只读 `bm.serverCode` |
| `upstream/bot_conn.go:287`、`516` | `serverCode` 在**建立连接的同一次加锁里**写入（与 `connected=true` 同一临界区）→ 不存在延迟填充 |
| `upstream/youyuan_remote.go:514-628` | 设备端 WS 客户端：539 行本地 HTTP 客户端 **140 s 超时** |
| `upstream/youyuan_remote_admission.go:166-169`、`25` | 中继请求 `go func` **并发**处理，上限 32 —— 后端不是串行瓶颈 |

### Cloudflare Worker

| 位置 | 内容 |
|---|---|
| `cloudflare/worker.mjs:45` | **每个**带会话令牌的 `/api/*` 请求都先做一次 `identityResolve`（Worker→identity DO 的额外往返） |
| `cloudflare/worker.mjs:140` | `await relay.fetch(request)` —— 转发到 RELAY DO |
| `cloudflare/worker.mjs:174` | 设备 WS 不在线时**立即**返回 503（不会让前端干等） |
| `cloudflare/worker.mjs:188` | 中继超时 **150 s** |
| `cloudflare/worker.mjs:167` | 新设备连接会 `close(1012)` 踢掉旧 socket（本次实测未发生抢占） |
| `cloudflare/worker.mjs:122` | `LINUX_DIRECT_MODE==='relay'` → 恒返回 `use_relay:true`，前端的直连通道（youyuan_boot.js:26）永远不启用 |

---

## 四、修复建议（按收益排序）

| # | 做法 | 收益 | 风险 | 改动面 |
|---|---|---|---|---|
| **1** | **新增聚合接口** `/api/youyuan/overview`，服务端一次返回 `{config, status, qq, remote, policy}`；前端 `Rcfg` 只发这一个请求 | **2.6 s → 0.5 s（-80%）**，且彻底消灭 3 波串行 | **低**：纯新增路由 + 改 1 个前端函数，不动现有接口 | 后端新增 handler（复用现有 5 个 handler 的内部函数）+ `youyuan_settings.js`/`pages/settings.js` |
| **1b** | **`/api/bot/status` 在未连接时回退返回配置里的服务器号**（`server` 空时填 `config.server_code`），前端显示"98014267 · 未连接"而不是空白 | 重启后 78 s 的空白窗口 → **立刻有号** | **低**：`main.go:1112-1119` 加 2 行；语义从"当前连接号"变"目标号"，需前端区分文案 | `main.go:handleBotStatus` + 设置页徽标文案 |
| **2** | **首屏 stale-while-revalidate**：把上次的 `status/config/policy` 存 localStorage，进页面**立即渲染**旧值（标注"上次数据·刷新中"），再异步校正 | **感知延迟 ≈ 0** | **低**：纯前端；只要文案区分"缓存/实时" | `pages/settings.js`、`youyuan_connection_policy.js` |
| **3** | **把 3 波压成 2 波**：`/api/config` 与 `/api/bot/status` 无依赖，改 `Promise.all([config, status])`；`connection-policy` 提前到与 `qq/remote` 同一批 | **-0.5～-1.0 s** | **低**：只改 `await` 结构；注意 `live()` 守卫仍在 | `youyuan_settings.js:30-31`、`youyuan_connection_policy.js:53` |
| **4** | **打开 Linux 直连**（`LINUX_DIRECT_MODE=direct`），前端已支持（`youyuan_boot.js:26`），浏览器直接打 trycloudflare 隧道，省掉 Worker→DO→WS 一跳 | **延迟收益 ≈ 0**（实测直连全程 0.52–0.58 s ≈ 中继 0.52–0.53 s，见 1.4）；只省 DO 计费与 Worker 一次转发，**不是性能优化** | **中高**：① 隧道健康检查反复失败（`journalctl -u youyuan-direct-tunnel` 见 `Linux direct HTTPS health failed (1/3)`）；② 直连只覆盖 `/api/*` 与 `/health`，静态资源仍走边缘（见第五节第 5 条，这点是好事）；③ 鉴权回退链路已写好但需复验 | `wrangler.jsonc` 环境变量 + 隧道保活 |
| **5** | **降频**：事件轮询 1.5 s → 5 s；10 s 轮询只保留 `/api/bot/status`（策略随页面刷新）；删掉空的 `pollToolboxInfo` | 常驻中继请求量 **-70%** | **低** | `youyuan_boot.js:79`、`pages/settings.js:18,72` |
| **6** | **首屏读接口超时收紧到 8–10 s**，超时显示"读取超时·重试"按钮（现在 `A()` 是 300 s、DO 是 150 s） | 假死时不再空白 2.5 分钟 | **低** | `api.js:18`（只对白名单读接口） |
| **7** | 首屏 **103 个静态资源**（85 个 `<script src>` + 18 个 `<link rel=stylesheet>`，JS 约 2.6 MB）合并/预加载 | 首次打开 <1 s（h2 下边缘入口整页 load 已只有 1.53 s，**不是本问题瓶颈**） | **低** | 构建产物，`index.html` |

**如果只做一件事**：做 1（聚合接口）。它把"要等 3 波"变成"只等 1 波"，服务器号、账号状态、连接策略会在同一次响应里一起出现。

---

## 五、附带发现（不是本次主因，但会影响"服务器号显示"）

1. **重启后服务器号会空白约 78 秒，不是"慢"而是"没有"**（与 perf-plugin-list 交叉验证后的更正）：
   - 重启时刻 03:35:19 → `/health` 200 仅 **+0.68 s**（端口立刻就绪）→ 首条 `[skin]` 日志 +11 s → **`/api/bot/status` 首次 `connected:true` 要到 +78 s**（03:36:37）。
   - 进程重启后 `botMgr=nil`，这段时间 `/api/bot/status` 恒返回 `connected:false`、`server:""`；`/api/config` 里的 `server_code` 仍有值，所以会出现"输入框有服务器号、顶部显示未连接"的割裂状态。
   - 我在 03:36:29 起的 12 次采样正好落在恢复点附近（第 4 次翻为 `connected:true`），先前据此估的"6–8 s"是错的，**真实掉线窗口 78 s**。任何"重启后刷新页面"的操作都会撞上这 78 秒。
   - `NRestarts=0` 且 `ActiveEnterTimestamp` 对得上 → 是外部 stop/start（部署脚本或人工），不是崩溃自愈。
2. `youyuan-update.service` 已连续失败：`/opt/youyuan/auto-update.sh: line 30: syntax error near unexpected token 'done'`（不影响主服务，但自动更新一直是坏的）。
3. **三个 youyuan 进程，但中继只有一个（已核实，先前的推断已撤回）**：
   - `19429` 主进程：`relay_connected:true`，`wss://…/device/connect?device=linux-server`，`account_id:"main"`。
   - `19501` / `19502` 两个账号进程：`account_id:"pool_3f5378b8-…"` / `"pool_d1f9f855-…"`，`/api/youyuan/remote` 返回 `enabled:false`、`url:""`、`relay_connected:false`，且两者 `connected:false`。
   - 结论：**子进程不持有中继 socket，不会与主进程抢 `worker.mjs:167` 的 device 槽**。我先前按 `ss` 看到三个进程各有到 `172.67.175.217:443` 的 ESTAB 就推断"三个都持 WSS"属过度推断——那是 TCP/443 连接，协议未核实（更可能是到 CF 的 HTTPS keep-alive，例如认证/身份请求）。该结论撤回。
   - 多机器人场景下"多账号同时跑"目前不成立（两个账号进程都未连接）。
4. `/api/youyuan/events` 每次返回 **17 945 B**，1.5 s 一次 ≈ 12 KB/s 常驻中继流量；该响应未压缩。
5. **直连网关每请求都新建 TCP 连接，但"代价多大"取决于入口**（perf-plugin-list 核实 + 我复核）：
   - `linux/direct_gateway.py:86,92` 用 `urllib.request.urlopen` 且强制 `Connection: close` / `Cache-Control: no-store`。
   - **只把 `/api/*` 送去隧道时无害**：本机回环 19430 实测 1.3–1.4 ms，可忽略；而前端的直连判定只对 `/api/` 和 `/health` 生效（`youyuan_boot.js:26` 显式排除 `/api/cloud/`、`/api/library/`），**静态资源永远走 CF 边缘**。
   - **如果整页资源也走隧道就变成主要成本**：perf-plugin-list 用 Playwright 实测同一份 103 个资源——边缘入口 `load 1.53 s`（h2×138），隧道入口 `load 7.62 s`（h2×118，多个 JS 排队 5.26 s），差 5 倍。原因正是每请求新建上游连接 + 32 并发上限（`direct_gateway.py:31`）。
   - 结论：**别把静态资源挂到隧道上**；做建议 4 时保持"只代理 `/api/*`"的现状即可。
   - 补实测：网关自身（含每请求新建上游连接 + `Connection: close`）在服务器上只花 **1.4 ms**（`127.0.0.1:19430/api/bot/status` = 1.68/1.38/1.31 ms）；而经隧道走完整路径的 `/api/plugin/list` 是 **0.52–0.58 s**，与中继全程的 0.52–0.53 s 相同。→ 网关不是瓶颈，**不能用 401 响应的耗时去指控 `:86`**（401 在 `:69-71` 就返回了，上游连接根本没建立）。
6. **HTTP/2 已确认可用**（perf-plugin-list 用 Playwright 读 `nextHopProtocol`：边缘入口 h2×138、隧道入口 h2×118；`openssl s_client -alpn h2` 两域名均返回 `h2`）。→ 浏览器侧的 6 连接上限**不成立**，所以建议 3 里"把请求并到同一波"不会被连接数卡住，并发代价很低。（我先前用 curl 看到 `http=1.1` 是本机 libcurl 不支持 `--http2` 的假象，已作废。）
7. **直连网关的 `denied` 指标主要是外部扫描噪声，不是鉴权坏了**：我实测 `/api/cloud/direct-metrics` = `{"requests":1090,"cloud_auth_checks":0,"denied":763}`（每请求 +1，含读取指标本身）。按 `direct_gateway.py:10,69-71`，`denied` 只在 `authenticate()` 返回 False 时累加，而 `cloud_auth_checks` 为 0 说明**从来没有请求带着非 console 令牌走到身份校验那一步**——即这 763 次是**完全没有 Authorization 头**的请求（公网 trycloudflare 域名被扫描）。真实前端每次都会带 `Authorization`（`youyuan_boot.js:24`），所以这不构成"开直连会被拒 70%"的证据。
8. **连接层发包缓冲与本次现象无关**（dig-codex-module 核实，我复核）：
   - `flowers-for-machines/core/minecraft/conn.go:336-338` `WritePacket`：由 **20 Hz（50 ms）定时器**统一 flush。
   - `conn.go:353-376` `WritePacketBounded`（player/console 通道）：**256 包 / 256 KiB** 阈值，AI 通道没有这个阈值，纯定时器驱动。
   - 缓冲粒度是**每账号一条 Conn**（`youyuan_commands.go:465-467` 按 `cfg.Account` 取独立 `BotManager`，各自持 `bm.conn`），不是每进程。
   - 关键澄清：命令链 fast 的 AI 通道（`youyuan_chain_fast_module.go:83-101`）直接写 **RakNet 连接**，**不经过 19429 的 HTTP 端口**，因此**不占用 `yyRemoteAdmission` 的 32 个中继槽**（`youyuan_remote_admission.go:166-169`）。→ 网页接口慢与命令链发包量无关；中继 429 只可能来自 MCP/网页请求本身。

---

## 六、复现命令

```bash
# 服务器本机（1 ms）
ssh root@47.102.156.125 'for i in 1 2 3; do curl -s -o /dev/null -w "%{time_total}\n" \
  http://127.0.0.1:19429/api/bot/status; done'

# 线上经 Worker（0.5–1.05 s），TOKEN 见 D:/YouYuan-APK/private/cloud-keys.json
TOK=...; for i in 1 2 3; do curl -s -o /dev/null -w "%{time_total}\n" \
  -H "Authorization: Bearer $TOK" -H "X-YouYuan-Device: linux-server" \
  https://toolbox.youyuanqi.dpdns.org/api/bot/status; done
```

测量脚本：`maintenance/perf_conn_measure.py`（只读，仅 curl + paramiko）。
