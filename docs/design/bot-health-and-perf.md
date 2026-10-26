# QQ 桥接健康检查、自愈与性能设计

> 目标：把「连不上就永远连不上」变成「分层可见 + 有限自愈 + 用户一键修复」。
> 范围：NapCat(OneBot) 适配器；QQ 官方适配器只在差异处标注。
> 本文档基于 2026-10-22 13:20–13:24 生产机只读实测与源码静态分析，不含任何变更。

---

## 0. 实测结论（先纠正两个前提）

| 观测项 | 实测值 | 结论 |
|---|---|---|
| `systemctl is-active napcat / youyuan` | `active` / `active` | 进程层正常 |
| 监听端口 | `19429` ✅、`6099`(WebUI) ✅、`3001` ❌ | **WS 未监听** |
| `/api/qq/status` | `state=reconnecting`，`detail="failed to WebSocket dial: ... connection refused"`，`group=null` | 内核侧判定正确，但只报底层错误 |
| `napcat/cache/qrcode.png` | 147×147 1-bit PNG，594B，mtime `13:22:11` | **二维码仍在刷新** |
| 登录周期 | `13:20:09` 出码 → `13:22:11` `Login Error ErrType:1 ErrCode:3` → 立刻出下一张码 | 周期 **≈122s**，不是「11:02 后不再刷新」 |
| journal 每 2s 一行 sha256 | 三次采样 `qrcode.png` 的 md5 完全不变 | 该 sha256 **与二维码无关**，是启动器噪声，不可作为出码信号 |
| 服务定义 | `Restart=always`，`RestartSec=15` | systemd 只在进程退出时重启，**登录失败不触发** |
| `bridge-watch.sh` | 轮询到 3001 出现 → POST 配置 → 打印 status → `break` | 一次性守候，**不覆盖 NapCat 侧故障** |

**故障链**：QQ 扫码登录从未成功（ErrType 1 / ErrCode 3）→ NapCat 不启动 OneBot WS → 3001 不监听 → 内核 `reconnecting` → 前端只看到 `connection refused`。
**关键推论**：`connection refused` 是**下游症状**。健康模型必须先报「QQ 未登录（等待扫码）」，否则用户永远在修一个没坏的东西。

---

## 1. 分层健康模型（L0–L6）

自下而上，**任一层失败，上层一律标 `blocked` 而不是 `failed`**，避免误导。

| 层 | 名称 | 检测方法（具体命令/调用） | 判定阈值 | 超时 |
|---|---|---|---|---|
| L0 | 进程存活 | `systemctl is-active napcat`；`pgrep -f /opt/QQ/qq`；`systemctl show -p ActiveEnterTimestamp napcat` | active 且 uptime>30s 为 `ok`；active 但 uptime<30s 为 `starting` | 2s |
| L1 | WebUI 可达 | `curl -s -m 2 -o /dev/null -w '%{http_code}' http://127.0.0.1:6099/` | `200`/`401`/`403` 均算 `ok`（401 只是要 token）；连接拒绝= `failed` | 2s |
| L2 | QQ 已登录 | ① `get_login_info`（需 L4）；② 无 WS 时退化为**二维码新鲜度 + 日志**：`stat -c %Y qrcode.png`、`journalctl -u napcat --since -10min \| grep -c 'Login Error'` | `now-mtime < 150s` → `waiting_scan`；`>=150s` 且 10min 内无新码 → `login_stalled`；无码文件且无 `Login Error` → `unknown` | 3s |
| L3 | OneBot WS 监听 | `ss -H -ltn 'sport = :3001' \| grep -q 3001`（比 dial 便宜，不产生连接） | 有 LISTEN → `ok`；无 → `failed` | 1s |
| L4 | WS 握手 | 内核 `yyQQState == "open"`（`youyuan_qq.go:286`）+ 连接建立时间戳 | `open` 持续 >5s → `ok`；`connecting` >15s → `hung`（见 R3） | — |
| L5 | 心跳 | OneBot `meta_event`/`heartbeat`，`heartInterval=30000`（`onebot11_*.json`）。**当前被丢弃**：`youyuan_qq.go:314` 只放行 `post_type=="message"` | 距上次心跳 >90s（3×interval）→ `silent` | — |
| L6 | 游戏端 | `botMgr.IsConnected()`（`youyuan_qq.go:468`） | true → `ok` | 1s |

**心跳缺失是当前最大的盲区**：TCP 半死（对端进程活着但不发数据）时 `conn.Read` 永久阻塞（`youyuan_qq.go:291`，无 deadline），状态会**永远停在 `open`**，前端显示「已连接」而实际一条消息都收不到。L5 必须补。

### 1.1 分层状态语义

```
ok        本层正常
starting  进程/连接刚起，宽限期内不告警
blocked   上游层失败导致本层无法判定（不告警，不重试）
failed    本层自身失败（触发自愈）
waiting   等待人工动作（扫码/授权），只提示不重试
```

`blocked` 是核心：L3 失败但 L2 是 `waiting_scan` 时，L3 标 `blocked`，**不得触发任何重启**。

---

## 2. 自愈动作矩阵

| 失败层 | 前置条件 | 动作 | 是否自动 | 说明 |
|---|---|---|---|---|
| L4 握手 | L3 `ok` 且 `connecting`>15s | 取消 ctx 重拨（指数退避） | 自动 | 现有 3s 固定重试见 R3 |
| L4 握手 | L3 `ok` 且连续失败 ≥5 次 | 重启 napcat | 自动（限流） | 只有 WS 在监听却握不上，才是 NapCat 侧问题 |
| L5 心跳 | `open` 且 90s 无心跳 | 主动 `conn.Ping` → 失败则断开重连 | 自动 | 见 §2.2 |
| L2 登录 | `qrcode.png` mtime >150s 且 10min 内无新码 | 重启 napcat（重新出码） | 自动（限流） | 出码循环卡死才重启 |
| L2 登录 | 二维码新鲜（<150s） | **只通知，不重启** | 否 | 重启只会让用户扫到一半的码失效 |
| L0 进程 | inactive | `systemctl start napcat` | 自动 | systemd 已有 `Restart=always`，仅在 failed 终态时补 |
| L1 WebUI | 不可达但 L0 `ok` | 仅记录，不动作 | 否 | 6099 挂了通常意味着 3001 也不会好，等 L0 判据 |
| L6 游戏 | 未连接 | 仅前端提示 | 否 | 与 QQ 无关 |
| 全部 `ok` | — | 重置退避计数器 | 自动 | 需连续 `open` ≥10min 才重置，防抖动 |

### 2.1 退避策略（防「疯狂重启」）

```
delay(n) = min(5s * 2^n, 300s) * (0.8 + 0.4*rand())      # n = 连续失败次数
n >= 6  → 进入 cooldown 30min，期间只告警不再动作
重启配额：≤3 次/小时，≤8 次/天（滑动窗口，落盘持久化，进程重启不清零）
```

- 现有代码 `time.Sleep(3 * time.Second)`（`youyuan_qq.go:347`）是**固定 3s**，配合 `Dial` 无超时（`:282`），连接拒绝场景下每分钟约 20 次拨号 + 20 条日志，属于无退避的忙等。
- 配额持久化必须落盘（`dataDir/bot-heal.json`），否则内核重启会绕过配额。
- **禁止**在 `L2=waiting_scan` 时重启：今天 13:20 那次重启直接把用户可能正在扫的码作废。

### 2.2 心跳与读超时（必须新增）

```
conn.SetReadLimit(2<<20)                      // 已有 :289
每 30s: conn.Ping(ctx_with_5s_timeout)
       若 ping 失败 → CloseNow → 进入重连
读循环: ctx = context.WithTimeout(background, 90s)
       每次 Read 成功重置 deadline；超时视为 silent → 重连
```

`coder/websocket` 的 `Ping` 会等对端 `Pong`，是判定半死连接最便宜的手段，且不污染业务消息。

---

## 3. 二维码状态的对外暴露

现状：`qrcode.png` 每 ≈122s 被覆盖一次；NapCat 同时把**解码 URL** 写进日志（`二维码解码URL: https://txz.qq.com/p?k=...&f=...`），比 147px 1-bit PNG 更适合前端展示。

**不要用 mtime 单独判活**，用三元组：

| 字段 | 取值 | 用途 |
|---|---|---|
| `qr_available` | `now-mtime <= 150s` | 能否展示 |
| `qr_age_seconds` | `now-mtime` | 前端倒计时（剩余 = 150-age） |
| `qr_cycle_seconds` | 相邻两次 mtime 变化差（滚动 3 次中位数，缺省 122） | 预测下次刷新 |
| `qr_url` | 从 `journalctl -u napcat --since -5min` 正则 `二维码解码URL:\s*(\S+)` 提取最后一条 | 手机直接打开 |
| `qr_png` | `GET /api/qq/qrcode.png`（服务端读文件直出，`Cache-Control: no-store` + `?t=mtime`） | 面板展示 |

**过期判定**：`qr_age > qr_cycle*1.3` 即视为「本轮码已死、下一张未到」→ 前端置灰 + 显示「正在重新获取二维码」，**不报错**。
**避免误判**：journal 里每 2s 的 sha256 行与二维码无关（已实测），提取逻辑必须锚定 `二维码解码URL` 关键字，禁止用「日志有新行」当作出码信号。

---

## 4. 后端接口：`GET /api/qq/health`

新增，替代「只有一个扁平 `state`」的 `/api/qq/status`（`youyuan_qq.go:167-171`）。`/api/qq/status` 保留兼容，内部改为读同一份快照。

```json
{
  "ok": true,
  "checked_at": "2026-10-22T13:24:05+08:00",
  "verdict": "waiting_scan",
  "headline": "QQ 未登录，请扫码",
  "layers": [
    {"id":"L0","name":"进程","status":"ok","detail":"active since 13:20:09","ms":12},
    {"id":"L1","name":"WebUI","status":"ok","detail":"HTTP 200","ms":8},
    {"id":"L2","name":"QQ登录","status":"waiting","detail":"二维码 68s 前刷新，等待扫码","ms":3},
    {"id":"L3","name":"WS监听","status":"blocked","detail":"登录后才会监听 3001","ms":1},
    {"id":"L4","name":"WS握手","status":"blocked","detail":"-","ms":0},
    {"id":"L5","name":"心跳","status":"blocked","detail":"-","ms":0},
    {"id":"L6","name":"游戏端","status":"ok","detail":"已连接","ms":4}
  ],
  "qr": {"available":true,"age_seconds":68,"cycle_seconds":122,"url":"https://txz.qq.com/p?k=...","png_url":"/api/qq/qrcode.png?t=1759"},
  "actions": [
    {"id":"show_qr","label":"显示二维码","kind":"manual","primary":true},
    {"id":"restart_napcat","label":"重启 NapCat 并重新出码","kind":"heal","enabled":true,"cooldown_seconds":0},
    {"id":"reconnect_ws","label":"重连 WebSocket","kind":"heal","enabled":false,"reason":"登录未完成"}
  ],
  "heal": {"fail_count":2,"next_allowed_at":"2026-10-22T13:24:35+08:00","restarts_1h":1,"restarts_24h":1}
}
```

设计要点：
- `verdict` 是**唯一**给前端做判断的字段，取值：`healthy / waiting_scan / ws_down / login_stalled / silent / game_down / disabled`。
- `actions[]` 由后端算好「哪个按钮可点」，前端不做业务判断（避免前后端规则漂移）。
- 单次请求总预算 **1.5s**：L0/L3 用 `exec.CommandContext` + 超时；L2 只 `stat` + 增量读 journal 尾 200 行，**不整文件扫**；结果缓存 3s，防前端多标签页打爆。
- L4/L5 直接读内存原子变量（见 §5.2），零 IO。
- 建议同时提供 `POST /api/qq/heal {"action":"restart_napcat"}`，服务端二次校验配额与前置层，返回 `{ok, retry_after}`。

---

## 5. 性能：现状判断与目标模型

### 5.1 结论先行

- `yyQQActionGate`（`youyuan_qq_actions.go:28-35`）**不是全局锁**，`run()` 在锁外执行（`:136`），正确；影响面仅限能力动作（低频）。
- 读循环**不会**被消息处理阻塞：`youyuan_qq.go:335` 用了 `go yyQQToGameMentionMeta(...)`。
- 真正的风险在**锁争用、无界 goroutine、无退避重连、无心跳**四处，下面逐条给行号。

### 5.2 风险清单

| # | 位置 | 问题 | 后果 | 建议 |
|---|---|---|---|---|
| R1 | `youyuan_qq.go:429-439` | 每条群消息在 **`yyQQMu` 全局锁内**遍历整个 `yyQQRecent`（10 分钟窗口）做过期清理 | 消息速率 ↑ → 持锁时间 ↑；与 `yyQQOneBotCall`(`:614`)、`/api/qq/status`(`:168`)、`yyQQSnapshot`(`:314`,`:321`) 争同一把锁 → 发送 ack 延迟 P99 恶化 | 去重表分片（16 shard，各自 mutex）；过期清理改为独立 goroutine 每 30s 一次，不在热路径 |
| R2 | `youyuan_qq.go:335` | 每条群消息 `go` 一个 goroutine，**无上限** | 洪水/刷屏时 goroutine 数 ≈ 速率 × 处理时长；每条持有 event 副本（含 raw_message ≤2MB 上限） | 有界队列（4096）+ 固定 worker（8）+ 满则丢弃并计数 `qq_msg_dropped_total` |
| R3 | `youyuan_qq.go:282` + `:347` | `Dial` 的 ctx 无超时；重连固定 `Sleep(3s)` | 对端半死时 dial 挂约 2 分钟且状态停在 `connecting`；拒绝时每分钟 20 次无效拨号 | `context.WithTimeout(10s)`；指数退避（§2.1） |
| R4 | `youyuan_qq.go:291` | `conn.Read(ctx)` 无 deadline、无 ping | 僵尸连接永不自愈，`state` 假 `open` | §2.2 |
| R5 | `youyuan_qq.go:190-213` + `:577-587` + `:625` | 出站单消费者；`yyQQSend` 串行遍历多群；单次调用 15s 超时 | 队头阻塞：一个群卡住 15s，后面全等；队列 256 满后**静默丢弃**（`:556-560`） | 按群分片 worker；超时降到 8s；丢弃计入指标并在前端显示「有 N 条未送达」 |
| R6 | `youyuan_qq_actions.go:135` | `stamp()` 在 `run()` **之前** | 长动作（如改头像）执行期间，同类请求被报「正在处理」而非排队 | 语义可接受，但错误文案要区分「正在执行」与「冷却中」 |
| R7 | `main.go:103-110` + `youyuan_sse_delivery.go:4-16` | `Emit` 持 `h.mu` 遍历所有 client；channel(50) 满即丢 | 高频 `qq_log` 会挤掉 `task_done` 等关键事件 | 日志类事件走独立 hub 或降采样（1 条/秒）；关键事件单独 channel |
| R8 | `youyuan_qq.go:314`,`:321` | 每条消息至少 2 次 `yyQQSnapshot()`（整结构体浅拷贝 + 加解锁） | 高频下纯开销；`Groups` 切片头部复制 | 读循环开头取一次快照复用；配置变更用 atomic.Pointer 整体替换 |

### 5.3 目标并发模型

```
readLoop (1 goroutine, 只做解析+入队)
   │  conn.Read → json.Decode → 轻量过滤(群/自身/去重)
   ▼
inbound chan yyEvent (cap 4096, 满 → drop + counter)      // 绝不阻塞 Read
   ▼
worker × 8  (按 group_id 哈希固定到 worker → 同群保序，跨群并行)
   │  单条处理预算 20s；超时打点 qq_handler_slow_total
   ▼
outbound: per-group queue (cap 128) → sender × 4 → conn
   │  写前 writeMu.Lock()（不依赖库内部串行化）
   ▼
重连器 (单独 goroutine)：backoff + ping + 心跳看门狗
```

要点：读循环**永不**做 IO、不取全局锁、不 spawn 无界 goroutine；同群顺序由哈希固定保证；跨群并行度 = worker 数。

### 5.4 内存 / 上下文膨胀

现状已有较好上界，不必重做：
- 消息上下文：`youyuan_qq_context.go:140-150` — 每 scope ≤80 条且 ≤32000 字符；scope ≤64（`:112-124`）。
- RAG 记忆：`youyuan_qq_memory.go:299-301` — 每 scope ≤160 条；≤32 scope；落盘 ≤24MB（`:201`）；TTL 默认 24h（`:95-104`）。
- AI 历史：≤128 会话（`youyuan_qq_ai.go:131`）。

需要补的只有两处：
1. `yyQQRecent`（`youyuan_qq.go:63`）在 10 分钟窗口内**无条数上限**——按 20 msg/s 可达 12000 条 map。加硬上限 4096 + 分片（同 R1）。
2. `yyQQHelper.Seen` 用 `len<256 || 已存在` 判定（`youyuan_qq_assistant.go:445`），语义正确但会让「第 257 个新成员」永远不被记录；建议改成 LRU 512。

---

## 6. 可观测性

### 6.1 必录指标

| 指标 | 类型 | 说明 |
|---|---|---|
| `qq_msg_in_total{group}` / `qq_msg_dropped_total{reason}` | counter | 吞吐与丢弃（reason=queue_full/dedup/disabled/self） |
| `qq_msg_handle_ms` | histogram(p50/p90/p99) | 单条处理耗时 |
| `qq_action_total{kind,ok}` / `qq_action_limited_total{kind,reason}` | counter | 动作成功率、限流命中 |
| `qq_ai_ms{model}` / `qq_ai_fail_total{reason}` | histogram/counter | AI 延迟与失败（含 slot 满） |
| `qq_ai_tokens_total{model,in,out}` | counter | 从 provider 响应 usage 取 |
| `qq_ws_state` | gauge(枚举) | 0 stopped / 1 connecting / 2 open / 3 reconnecting |
| `qq_ws_reconnect_total{reason}` | counter | 区分 refused / eof / ping_timeout / silent |
| `qq_ws_uptime_seconds` | gauge | 当前连接存活时长 |
| `qq_heartbeat_age_seconds` | gauge | 心跳新鲜度（L5） |
| `qq_heal_total{action,result}` | counter | 自愈动作与结果 |
| `qq_out_queue_depth{group}` | gauge | 出站积压 |

### 6.2 落盘格式

```
dataDir/metrics/qq-2026-10-22.jsonl       # 按天，UTF-8，一行一 JSON
{"t":"2026-10-22T13:24:05.123+08:00","m":"qq_msg_in_total","v":1,"g":"1121432132"}
{"t":"...","m":"qq_msg_handle_ms","v":42,"g":"1121432132"}
{"t":"...","m":"qq_ws_state","v":3,"r":"refused"}
```

- 聚合写入：内存累加，**每 10s 刷一次**；每行 ≤512B；单文件上限 **32MB**，超限即切 `qq-2026-10-22.1.jsonl`。
- 保留 **7 天**，启动时清理过期文件。
- 崩溃安全：`O_APPEND` 单写者（专用 goroutine 独占 fd），不 fsync（可接受丢最后 10s）。
- **禁止**把原始群消息文本写入指标文件（隐私）；文本只在内存上下文与既有 `qq-group-memory.json` 中。

### 6.3 查询接口

`GET /api/qq/metrics?range=24h&step=5m`

```json
{"ok":true,"range":"24h","step":"5m","series":{
  "msg_in":[["13:00",12],["13:05",31]],
  "msg_dropped":[["13:00",0]],
  "handle_p99_ms":[["13:00",88]],
  "ws_state":[["13:00",3]],
  "reconnects":[["13:00",2]],
  "ai_p95_ms":[["13:00",2100]],
  "tokens":[["13:00",1840]]
},"summary":{"msg_in":1820,"msg_dropped":3,"reconnects":7,"uptime_ratio":0.83}}
```

实现：只扫最近 N 天 jsonl（`range` 上限 7d），流式解析不整体载入；单次 ≤200ms；结果缓存 15s。

---

## 7. 给用户的「自动检测」落地清单（前端）

现状：`youyuan_qq_console.js:152-158` 只在进页面时 `await yyQQConsoleLoad()`，**无任何轮询**；`yyQQConsoleRefresh` 是整页 `openSubPage` 重绘（`:179-188`），会丢滚动位置与输入框内容。这正是「卡在这里之后一直没用」的体感来源。

必须做：

1. **页面打开即检测**：进「群服互通」立即 `GET /api/qq/health`，1.5s 内出结论；失败（超时/500）显示「内核未响应」而不是空白。
2. **自动轮询**：`setInterval(4000)` 拉 `/api/qq/health`；`document.hidden` 时降频到 30s；`verdict` 变化立即触发一次；离开页面 `clearInterval`。
3. **失败自动重试**：请求失败按 2s→4s→8s→15s 退避，连续 5 次后停止并显示「已停止自动重试 · 点击重试」。
4. **局部刷新**：只重绘 `[data-yy-health]` 区块与 hero 的状态标签，**不调用 `openSubPage`**；保留滚动与表单输入。状态未变化时不触发任何 DOM 写入（对比 `verdict` + `qr.age_seconds` 秒级字段）。
5. **顶部横幅**：`verdict != healthy` 时在面板顶部固定横幅——颜色按 `waiting_scan`(黄)/`ws_down`(橙)/`login_stalled`(红)/`silent`(橙)；文案用 `headline`；右侧放 `actions[]` 里 `primary` 的按钮。
6. **一键修复**：点 `restart_napcat` → `POST /api/qq/heal` → 按钮进入 loading（最长 30s）→ 轮询 health 直到 `waiting_scan` 或超时 → 自动展开二维码卡片。按钮在 `heal.next_allowed_at` 未到前禁用并显示倒计时。
7. **二维码卡片**：`<img src="/api/qq/qrcode.png?t=...">` + 倒计时环（`150-age`）；`age>150` 自动换成「正在获取新二维码…」并 2s 轮询，不报错；同时提供 `qr.url` 的「手机打开」链接（PNG 只有 147px，扫码成功率低）。
8. **掉线可感知**：`verdict` 从 `healthy` 变其他时，`T()` 一条 toast + 横幅；从其他变 `healthy` 时 toast「QQ 已恢复」，并清空失败计数。
9. **状态时间戳**：横幅副标题显示「最后正常：13:05（19 分钟前）」，数据来自 `heal`/`checked_at`，让用户知道坏了多久。
10. **不要**用 SSE(`handleSSE`) 作为唯一数据源——它满 50 条即丢事件（`youyuan_sse_delivery.go:4-16`），只可作为「立即刷新」的触发器。

---

## 8. 实施顺序与验收

| 阶段 | 内容 | 验收标准 |
|---|---|---|
| P0 | L5 心跳 + 读超时 + 指数退避 + dial 超时（R3/R4） | 手动 `iptables` 丢弃 3001 流量后，90s 内 `state` 变 `reconnecting` 且 `qq_ws_reconnect_total{reason="silent"}` +1 |
| P1 | `GET /api/qq/health`（分层 + actions + qr） | 当前故障下返回 `verdict=waiting_scan`、`L3=blocked`，且**不触发**任何重启 |
| P2 | 自愈矩阵 + 配额落盘 | 连续杀掉 napcat 10 次，1 小时内实际重启 ≤3 次，第 4 次返回 `retry_after` |
| P3 | 前端横幅 + 4s 轮询 + 局部刷新 + 一键修复 | 断网/掉登录时页面 5s 内出现横幅；点修复后二维码自动出现；全程无整页重载、输入框不丢 |
| P4 | 指标 JSONL + `/api/qq/metrics` | 24h 数据可在面板画出吞吐/重连曲线；单文件不超 32MB |

**P0 的 R1/R2 可以延后**：当前群规模（单群、低并发）下不会触发；但在做「多群/高频」前必须先落地，否则 `yyQQMu` 会成为全局瓶颈。

---

## 9. 附：本次只读诊断命令（可复用）

```bash
systemctl is-active napcat youyuan
ss -tlnp | grep -E ':3001|:6099|:19429'
curl -s -m5 http://127.0.0.1:19429/api/qq/status
stat -c '%y %s' /opt/qqbot/napcat/cache/qrcode.png
journalctl -u napcat --since '-10min' | grep -aE 'Login Error|二维码解码URL'
cat /opt/qqbot/bridge-watch.log
```

判读口诀：**3001 不在 = 没登录；码新鲜 = 等扫码；码不新鲜 = 出码卡死；码新鲜但 WS 在 = 握手段问题。**
