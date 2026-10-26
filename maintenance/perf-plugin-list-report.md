# 插件列表面板"刚打开要等半天"性能分析报告（v4，已撤掉无效 A/B）

- 目标环境：Linux 47.102.156.125，`youyuan.service`:19429
- 两个对外入口：
  - 页面/静态资源：`https://toolbox.youyuanqi.dpdns.org`（Cloudflare Worker，`env.ASSETS` 边缘直出）
  - API 直连：`https://discussed-photographs-ten-chosen.trycloudflare.com` → `direct_gateway.py`:19430 → Go:19429
- 实测时间：2026-10-21 03:32 ~ 04:15（CST）

**适用范围（避免与 perf-conn-info 报告混排）**：本报告只解释**「插件」标签页（`#tab-tasks`）打开慢**——瓶颈是首屏 139 个请求（`index.html` 静态引用 103 个 + 运行时动态拉取约 36 个）+ 插件面板的多次 API 往返，所以"合并 bundle"排第一。perf-conn-info 那份解释的是**「连接」卡片服务器号显示慢**——瓶颈是 5 个 API 串行 3 波、静态资源已确认不是瓶颈，所以"聚合接口"排第一。两份排名不可合并成一张表，但**根因相同**：公网每次 API 往返 0.5~1.1s，减少往返次数是共同落点。

**版本修正轨迹**
- v1：报"整页 16 秒" —— HTTP/1.1 测量产物，**作废**。
- v2：h2 确认后改报"边缘 1.5s / 隧道 7.6s" —— 隧道入口整页**不是真实路径**（见下），降级。
- v3：报"直连比中继慢 3~12 倍" —— **A/B 方法错误，作废**（见 2.4）。我用的是无令牌 401 响应，两条都在进入中继/上游之前就返回了，测到的只是"谁的 401 先回"。
- v4（本版）：带令牌重测三路径，**中继与直连同量级**（1.0~1.1s vs 0.8~1.5s）；网关回环开销仅 3~5ms。真实成本是"每一次 API 都要付 0.5~1.1s 的公网往返"，与走哪条路无关。

---

## 一、一句话原理

**`/api/plugin/list` 服务端 1.3 毫秒、4.3KB，后端不是瓶颈。** 用户感知的"半天"由两段叠加而成：

1. **首屏 1.5 秒**：`index.html` 静态引用 103 个 JS/CSS（2.76MB，无 gzip），运行时实测共 139 个请求，全部由 Cloudflare 边缘提供；桌面浏览器 load 1527ms，手机上还要加上这 139 个请求在 4G 上的往返与 2.76MB 的 JS 解析时间。
2. **每一次 API 0.5~1.1 秒**：这是公网往返的固有成本，**中继和直连同量级**（带令牌实测：中继 1.0~1.1s、直连 0.8~1.5s）。网关本身只占 3~5ms，不是成本来源。

**结论：后端零优化价值，网关也不是瓶颈；真正该动的是"首屏资源体积/数量"和"减少 API 往返次数"。**

---

## 二、实测数据

### 2.1 服务端接口耗时（服务器上 curl 127.0.0.1）

| 接口 | 第 1 次 | 第 2 次 | 第 3 次 | 响应体 |
|---|---|---|---|---|
| `/api/plugin/list` | 1.474 ms | 1.338 ms | 1.247 ms | 4288 B |
| `/api/bot/status` | 1.048 ms | 0.893 ms | 0.916 ms | 114 B |
| `/api/task/list` | 2.796 ms | 1.052 ms | 1.173 ms | 25 B |
| `/api/youyuan/chains` | 1.184 ms | 1.275 ms | 1.052 ms | 3285 B |
| `/api/youyuan/runtime/list` | 1.054 ms | 1.004 ms | 1.032 ms | 143 B |
| `/api/youyuan/agent/workspaces` | 0.968 ms | 0.954 ms | 1.014 ms | 191 B |
| `/health` | 1.553 ms | 1.039 ms | 0.912 ms | 201 B |
| `/api/plugin/list` × 6 并发 | 2.54 / 5.43 / 1.89 / 1.69 / 1.58 / 1.43 ms | — | — | 无锁竞争 |

### 2.2 冷启动（03:35:19 `systemctl restart youyuan.service`）

| 指标 | 数值 |
|---|---|
| 进程启动 → 端口可连（`/health` 200） | **0.68 s** |
| 冷启动后第 1 次 `/api/plugin/list` | **1.353 ms** |
| 第 2 / 3 / 4 / 5 次 | 1.155 / 1.106 / 1.345 / 1.568 ms |
| 冷启动后对照接口 | 0.95~1.39 ms |
| 机器人恢复 `connected:true` | 重启后 **78 秒**（03:36:37，期间掉线） |

**无冷启动惩罚**：第一次请求与第五次同量级。

### 2.3 真实浏览器整页加载（Playwright Chromium，读 `nextHopProtocol`）

| 入口 | TTFB | DOMContentLoaded | load | 资源数 | 协议 | 最慢单资源 |
|---|---|---|---|---|---|---|
| `toolbox.youyuanqi.dpdns.org`（**真实路径**） | 646 ms | 1409 ms | **1527 ms** | 139 | h2 ×138 | 573 ms |
| `discussed-photographs-ten-chosen.trycloudflare.com`（**非真实路径**，仅手开隧道 URL 时） | 1570 ms | 7026 ms | 7621 ms | 118 | h2 ×118 | 5259 ms |

> 隧道入口整页 7.6s 只用于说明网关的承载力：同一份资源走边缘 1.5s、穿隧道 7.6s，差 5 倍。**正常使用不会走到这条路**，因为 `youyuan_boot.js:26-28` 只把 `/api/`、`/mcp`、`/health` 改写到直连源，`<script src>` 由 HTML 解析器发起，永远落在页面自身的源（边缘）。

### 2.4 三路径 A/B（**必须都带令牌**，同接口 `/api/plugin/list`，热连接）

| 路径 | #1 | #2 | #3 | 状态 / 响应体 | 是否真的到 Go |
|---|---|---|---|---|---|
| A 边缘 401（无令牌） | 572 ms | 547 ms | 682 ms | 401 / 85 B | 否（`worker.mjs:93-95` 直接返回） |
| **B 中继全程（带令牌）** | 1133 ms | 996 ms | 1077 ms | 200 / 4288 B | **是** |
| **C 直连全程（带令牌）** | 1467 ms | 809 ms | 1045 ms | 200 / 4288 B | **是** |
| D 直连 401（无令牌） | 834 ms | 1043 ms | 796 ms | 401 / 184 B | 否（`direct_gateway.py:69-71` 直接返回） |

**结论：中继 1.0~1.1s ≈ 直连 0.8~1.5s，差异在噪声内，直连不带来延迟收益。**

> **v3 的方法错误（记录在此以免重犯）**：v3 用"中继 127ms vs 直连 411ms"得出"直连慢 3~12 倍"，但两条都是**无令牌 401**——401 在 `cloudflare/worker.mjs:93-95`（`relay.fetch()` 在 `:140` 之后）和 `direct_gateway.py:69-71`（`:86` 之前）就返回了，**中继那一跳和上游连接都没有发生**。测到的只是"谁的 401 先回"。

### 2.5 网关开销（回环，排除公网）

| 测量 | #1 | #2 | #3 |
|---|---|---|---|
| `127.0.0.1:19430`（网关，含新建上游连接 + `Connection: close` + Go 处理） | 3.451 ms | 2.982 ms | 5.206 ms |
| `127.0.0.1:19429`（直连 Go） | 1.064 ms | 1.258 ms | 1.068 ms |

**网关净开销 2~4 ms，`direct_gateway.py:86/:92` 在回环上不可测量，不是成本来源。**

### 2.5 公网链路与网关指标

| 测量项 | 数值 |
|---|---|
| 隧道 `/api/plugin/list`（curl，401） | 0.94 s TTFB |
| 固定域名 `/js/api.js` 响应头 | `CF-Cache-Status: HIT`、`ETag`、`Cache-Control: public, max-age=0, must-revalidate` |
| 服务器出网下载（5MB） | 2.53 MB/s（**不是带宽瓶颈**） |
| 网关 `/api/cloud/direct-metrics` | `requests: 944`、`denied: 726`、`cloud_auth_checks: 0` |

> `denied: 726` **不是鉴权故障**：`authenticate()` 在 `if not token: return False` 处即返回（`direct_gateway.py:10`），`cloud_auth_checks` 恒为 0 说明这 726 次是**完全没带 Authorization 头**的请求——公网 trycloudflare 域名被扫描是常态，真实前端每次都会带（`youyuan_boot.js:24`）。

---

## 三、代码证据

### 3.1 后端接口：纯内存读

`upstream/main.go:642`
```go
func handlePluginList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "plugins": pluginController.List(), "market_origin": yyMarketOrigin(config.ReloadConfig())})
}
```
`upstream/plugin_controller.go:249`
```go
func (pc *PluginController) List() []PluginStatus {
	pc.mu.Lock(); defer pc.mu.Unlock()
	out := make([]PluginStatus, 0, len(pc.defs))
	for _, def := range pc.defs { st := pc.defStatus(def); out = append(out, *st) }
	return out
}
```
- 只遍历内存 map：不读磁盘、不起 Lua/Python、不解析词库、不做语法检查。
- `ScanPlugins()`（`plugin_controller.go:157`）只在启动（`main.go:4834`）和导入（`main.go:751`、`2460`）调用，不在请求路径。
- 唯一全局锁 `pc.mu` 微秒级持有；6 并发最慢 5.4ms。

### 3.2 直连路由规则（决定"哪些请求会变慢"）

`upstream/frontend/js/youyuan_boot.js:26-28`
```js
if (path.startsWith('/api/')||path.startsWith('/mcp')||path==='/health'){ ... headers.set('Authorization','Bearer '+key) ... }
var direct = window.yyLinuxDirectOrigin && (path.startsWith('/api/')||path==='/health')
          && !path.startsWith('/api/identity/') && !path.startsWith('/api/cloud/') && !path.startsWith('/api/library/')
          && new URLSearchParams(location.search).get('device')==='linux-server';
```
- 只有 `fetch()` 发出的 `/api/*`、`/health` 会被改写到直连源；**静态资源不受影响**。
- `discoverLinux()` 有 **8 秒 abort 上限**（`:20`），失败则 60s 后重试，期间全部走中继。

### 3.3 直连网关：每请求新建连接（**回环实测 2~4ms，不是瓶颈，仅记录**）

`linux/direct_gateway.py:31`
```python
daemon_threads=True; slots=threading.BoundedSemaphore(32)
```
`linux/direct_gateway.py:86`
```python
response=urllib.request.urlopen(req,timeout=155)   # 每个请求新建到 127.0.0.1:19429 的连接，无连接池
```
`linux/direct_gateway.py:92`
```python
self.send_header('Cache-Control','no-store'); ... self.send_header('Connection','close'); self.close_connection=True
```
> 这三处在 v3 被我当成"直连慢 3~12 倍"的原因，**是错的**。回环实测网关全程 3.0~5.2ms（含新建上游连接），Go 本身 1.1~1.3ms，净开销 2~4ms。公网那 0.8~1.5s 是网络往返，与网关无关。`no-store` 只影响"手开隧道 URL 时的静态资源缓存"，真实路径下资源走边缘。

### 3.4 静态资源（Go 侧）：无压缩、无 ETag

`upstream/main.go:253`
```go
w.Header().Set("Content-Type", ct)
w.Header().Set("Cache-Control", "no-cache")   // 无 ETag / Last-Modified → 无法 304
w.Write(data)                                  // 无 gzip / br
```
> 但真实路径下资源由 Worker 边缘提供，边缘已带 `ETag` + `max-age=0, must-revalidate`，所以这处只在"手开隧道 URL"时才是问题。

### 3.5 前端首屏：103 个资源

`upstream/frontend/index.html:81-161`：**85 个 `<script src>` + 18 个 `<link rel=stylesheet>` = 103 个唯一资源**（`cloudflare/public/index.html` 与之逐字节相同）。含 `three.min.js` 636KB、`youyuan_editor.js` 572KB，均首屏同步加载；末尾才 `initPersonalize();tabTo('home')`。
口径提示：`grep -c '<script src='` 只数到 77 行（第 161 行一行 3 个 `<script>`），必须用 `grep -o` 数出现次数。

---

## 四、前端请求链路（谁等谁）

```
打开程序（真实路径）
  ├─ GET /                        边缘，TTFB 646ms
  ├─ 139 × (JS/CSS)               边缘 h2，全部到位 load = 1527ms
  │    └─ 资源永远走边缘：youyuan_boot.js:26 只改写 /api/*、/mcp、/health
  ├─ 脚本到位 → initPersonalize(); tabTo('home')
  └─ 点"插件" → app.js:480 → Rplg()
       └─ plugins.js:39  GET /api/plugin/list
            ├─ 中继全程：1.0~1.1s
            └─ 直连全程：0.8~1.5s（同量级，无收益）
```

- 无串行依赖：`api.js:12` 的 `A()` 只对同 URL 去重，不全局串行；`sse.js:12` 在插件页收到事件会重刷 `Rplg()`，2 秒节流且被去重。
- 真实等待 = 首屏 1.5s（桌面）+ 一次 API 0.5~1.1s。手机端还会叠加 139 个条件请求的往返与 2.76MB JS 的解析。
- 放大项：`api.js:18` 超时 **300000ms**，链路抖动时界面静默卡住 5 分钟才报错；直连失败会 `directFailed()` 后**自动重放一次到中继**（`youyuan_boot.js:38-45`），一次操作可能变成 2 倍耗时。

---

## 五、修复建议（按收益排序）

| # | 措施 | 预期收益 | 风险 | 说明 |
|---|---|---|---|---|
| 1 | **合并 bundle**：85 JS → 3~5 个，把 `three.min.js`(636KB)/`youyuan_editor.js`(572KB) 改懒加载 | 139 请求 → ~10，手机端首屏显著缩短 | 中（需构建步骤，改发布流程） | 对"首次打开"唯一的结构性解法 |
| 2 | **插件面板先出骨架**：`Rplg()` 立即画空壳+"加载中"再回填 | 用户感知延迟 → 0 | 低 | 成本最低，立刻改善手感 |
| 3 | **减少 API 往返次数**：`Rplg()` 只打 1 个接口（现状已是），但 workspace/agent 视图是 3 个 `Promise.all`（`youyuan_workspace.js:12`），可合并为 1 个聚合接口 | 每次面板打开省 0.5~1.0s | 低-中（新增聚合接口） | 公网往返 0.5~1.1s/次，减少次数是唯一能砍掉这段的办法 |
| 4 | **开 gzip/br** | 2.76MB → 约 0.7MB | 低-中 | 边缘入口默认已压缩，此项主要针对直出场景 |
| 5 | 直连健康检查加重试/退避（`health failed (1/3)` 会让 `directFailed()` 重放到中继） | 消除偶发的 2 倍耗时 | 中 | 与 perf-conn-info 的建议重叠 |
| 6 | 护栏：静态资源永远走 Worker 边缘，**不要挂隧道** | 避免 7.6s 的最坏情况 | 低 | 现状已正确，属于"不要退步" |

**明确不建议做**：
- 优化 `handlePluginList`、给 `List()` 加缓存、把 `ScanPlugins` 改异步（冷启动 0.68s 就绪、首请求 1.35ms，零收益）。
- **改直连网关的连接池/`Connection: close`**（`direct_gateway.py:86,92`）：回环实测净开销 2~4ms，中继与直连公网延迟同量级（1.0~1.1s vs 0.8~1.5s），改了不会变快。v3 曾把它列为建议 1，基于错误的 A/B，**已撤销**。

**测量缺口（需要用户侧配合）**：我在服务端与公网侧都无法复现"半天"。若用户仍感知明显延迟，最可能的剩余原因是手机端：WebView 冷启动 + 139 个条件请求在 4G 上的往返 + 2.76MB JS 解析。建议让用户在手机上开一次远程调试，抓 `Performance` 面板的 `DOMContentLoaded`、以及 `/api/plugin/list` 那条请求的 `Waiting(TTFB)`——前者大就是建议 1/4，后者大就是建议 3。

---

## 六、对线上环境的影响说明

- 为取得冷启动数据，**2026-10-21 03:35:19** 执行了一次 `systemctl restart youyuan.service`。
- 机器人**掉线约 78 秒**，于 **03:36:37** 恢复 `connected:true, is_op:true, server=98014267`；服务全程 `active`。
- 除该次重启外全程只读探测，未修改任何代码或配置。

## 七、复现脚本

- `maintenance/perf_probe_plugin_list.py` — 接口耗时/并发/对照
- `maintenance/perf_probe_assets.py` — 响应头、资源体积、出网带宽
- `maintenance/perf_probe_startup_log.py` — 启动日志时间线
- `maintenance/perf_probe_coldstart.py` — 冷启动计时（**含重启，谨慎执行**）
- `maintenance/perf_probe_public_api.py` — 公网入口接口探测（非浏览器 UA 会被 CF WAF 1010 拦）
- `maintenance/perf_probe_three_paths.py` — **三路径带令牌 A/B（唯一有效的对比方法）**
