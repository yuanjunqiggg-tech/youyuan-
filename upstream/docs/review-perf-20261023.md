# 性能分析（2026-10-23）

测量环境：Windows 11 / i9-13900H，Go 1.27，`upstream/` 包；基准为临时文件
`perf_tmp_test.go`（`go test -bench . -benchmem -run '^$'`，已删除）。前端用真实
`youyuan_qq_console.js` 的定时器代码在 Node 里跑 60 秒（DOM/`A()` 打桩，按各接口真实响应体积计字节）。
线上机（47.102.156.125）未触碰。

## 结论（能不能上线）

1. **能上线，但 `/api/qq/login` 这条轮询必须先处理**：未登录态下每 20 秒把二维码 base64 重新算一遍（磁盘读 + ≤683 KB 序列化），实测 60 秒 176 KB，折合 **约 10 MB/小时**手机流量；判定用的还是「游戏连接」而不是 QQ 登录态，游戏一停就永久掉进这条路径。
2. **`yyBridge` 的窗口索引会无界长**：200 条 per-player 规则 × 500 个玩家 = **10 万个 key、+10.1 MB 堆，且永不回收**（`marks[key]=nil` 不删 key）。这是唯一的真泄漏，改动量最小。
3. 其余（种子查询、玩家列表、`yyQQActionPolicy` 锁、10 秒状态轮询）实测都远低于风险线，**不需要为它们做优化**。特别注意：`yyQQActionPolicy` 不是每条消息都调，10 秒轮询也不拨 TCP（见下）。

## 实测数据

### 后端 benchmark（ns/op / B/op / allocs/op）

| 基准 | ns/op | B/op | allocs/op | 说明 |
|---|---:|---:|---:|---|
| BridgePublish 0 规则 | 169 | 152 | 1 | 只有环 append |
| BridgePublish 0 规则 + 环已满 300 | 187 | 152 | 1 | 环不搬迁（切片保留富余 cap），无异常 |
| BridgePublish 200 规则全不命中 | 19,504 | 32,921 | 2 | **纯复制规则切片**，32.9 KB/事件 |
| BridgeFire 1 规则（窗口空） | 342 | 432 | 4 | 342 ns 里含我这边每次重建 map 的开销，净成本约 250 ns |
| BridgeFire 1 规则（窗口 20 条） | 1,433 | 1,920 | 6 | 窗口内条数线性 |
| BridgePublish 50 规则全命中 | 1,208,084 | 13,951 | 52 | 窗口内条数随迭代堆积，见下注 |
| BridgePublish 200 规则全命中 | 2,450,847 | 52,752 | 202 | 同上 |
| BridgePlayerRows 100 玩家 | 54,732 | 47,488 | 802 | limit=100 |
| BridgePlayerRows 1000 玩家 | 189,845 | 62,080 | 802 | 仍只返回 100 行，排序+建 map 全量 |
| QQSnapshot（含 yyQQMu） | 77 | 0 | 0 | 浅拷贝，0 分配 |
| ActionPolicy（一次） | 130 | 0 | 0 | = snapshot + 能力表线性扫描 |
| ActionPolicy（并发 20G） | 309 | 0 | 0 | 锁竞争后 2.4 倍，绝对值仍 <0.4 µs |
| SeedSlime r=8 | 4,087 | 2,016 | 12 | |
| SeedSlime r=64（16641 区块） | 179,982 | 158,176 | 26 | 0.18 ms，预算 150 ms |
| SeedQuery r=16 | 23,427 | 15,304 | 79 | |
| SeedQuery r=32 | 59,273 | 40,136 | 92 | 0.06 ms |
| SeedStructureScan 单轴 512 区块 | 20,320 | 7,768 | 7 | |
| SeedStructureScan 单轴 4096 区块（village） | 32,506 | 7,768 | 7 | 上限 60000 region，最坏 0.03 ms |

> 注：命中类基准的 marks 切片会随迭代次数堆积（跑了几千次），所以 50/200 规则那两个数
> 是「窗口内几千条」的极端值。按窗口 20 条的单规则 1.43 µs 外推，200 条规则 ≈ **286 µs/事件**，
> 这才是 10 秒窗口 + 20 事件/秒时的合理值。

### 前端 60 秒实测（真实定时器代码）

| 页面状态 | 请求数/60s | 估算流量/60s | 明细 |
|---|---:|---:|---|
| 已登录、正常 | 13 | 41.9 KB | `/api/qq/status`×6（1.1 KB）、`/api/qq/messages`×4（40 KB）、`/api/qq/login`×3（0.8 KB） |
| 未登录 / 游戏未连接 | 13 | 216.9 KB | 同上，但 3 次 login 每次带 45 KB 二维码 → 175.8 KB |

- 10 秒那次轮询**只打 `/api/qq/status`，一次 TCP 都不拨**。`yyQQWSListening()` 的
  `net.DialTimeout` 只在 `/api/qq/login` 里（`youyuan_qq_login.go:101`），全仓库只有这一个调用点。
- 但 `/api/qq/login` 被 20 秒定时器拉（`youyuan_qq_console.js:1154-1157`），条件是
  `login.state !== "logged_in"`。而后端 `logged_in` 的判据是
  `WSListening && 游戏bot.IsConnected()`（`youyuan_qq_login.go:95-104`）——**掺了游戏连接**。
  所以「QQ 好着、游戏没开」时永远不是 logged_in，于是每 20 秒：1 次 TCP dial + 1 次
  `qrcode.png` 磁盘读 + 1 次 base64（≤512 KB 原文 → ≤683 KB）。
- 换算：未登录态 **3 次/分钟 = 4320 次/天**，按 45 KB png 计约 **10.5 MB/小时**；
  `docs/onebot-drop-and-qr-diagnosis.md:244` 记录过实测 700 KB 响应，那就是 **120 MB/小时**量级。

## 热点与根因

### 1. `yyBridge.marks` / `lastFire` 的 key 永不回收（真泄漏）

`upstream/youyuan_bridge_api.go:147-177`：key 是 `规则ID|玩家名`（per_player）或 `规则ID`。
触发后 `yyBridge.marks[key] = nil`（`:176`）只是清空值，**key 一直留在 map 里**；
`lastFire[key]`（`:175`）同理，且没有 TTL。

实测（`TestPerfBridgeMarksGrowth`）：200 条 per-player 规则 × 500 个玩家
→ `marks` 键数 **100,000**、堆 **+10,079 KB**。key 数 = 规则数 × 历史玩家数，
与运行时长同向增长；玩家改名/小号越多涨得越快。

### 2. `/api/qq/login` 的登录态判据混了游戏连接

`upstream/youyuan_qq_login.go:91-137`：`view.LoggedIn = view.WSListening && view.Connected`，
而 `view.Connected` 取的是 `botMgr.IsConnected()`（游戏连接，`:95-100`）。
不是 logged_in 就走 `:127-131` 的读图 + base64，**没有任何缓存**——同一个文件每 20 秒
重新读一遍、重新编码一遍。对应前端 `:1154-1157`。

### 3. `yyBridgePublish` 每个事件复制整份规则切片

`upstream/youyuan_bridge_api.go:125`：`rules := append([]yyBridgeRule(nil), yyBridge.Rules...)`
在持锁状态下 memcpy 整份配置。实测 200 条 = **19.5 µs / 32.9 KB / 事件**，而且是在
`yyBridge` 锁内拷贝，把 `/api/bridge/events`、`/api/bridge/manifest` 的读也一起拖住。
即使一条规则都不命中也要付这份钱（`:128-133` 遍历前就已经付了）。

### 4. `yyBridgeFire` 每条规则一次加锁 + 窗口切片线性扫描

`upstream/youyuan_bridge_api.go:154-177`。单规则空窗口 342 ns、窗口 20 条约 1.43 µs。
200 条规则全命中 ≈ 286 µs/事件，全部串行在 `yyBridge.Lock()` 上（每规则一锁一放，
不是一次锁遍历完）。同时 `:128-133` 是 O(规则数) 全表扫描，没有按 event 建索引。

### 5. `yyBridgePlayerRows` 全量排序后截断

`upstream/youyuan_bridge_api.go:584-608`：为全部玩家建 names 切片、`sort.Strings`、
再逐个建 `map[string]any`（802 allocs）。1000 玩家 190 µs，其中大部分浪费在
「排序 1000 个但只返回 100 行」。同构的 `/api/qq/server/players`
（`youyuan_qq_serverinfo.go:318-381`）也是全量建行后排序再截断。

### 6. 写配置时持 `yyQQMu` 做磁盘 IO（预存在，非本轮新增）

`upstream/youyuan_qq.go:115-157`：`json.MarshalIndent` + `os.WriteFile` 全程持 `yyQQMu`。
这段时间里所有 `yyQQSnapshot()`（77 ns 那次）都阻塞，包括动作策略和状态轮询。
`/api/qq/status`（`:180-184`）也持锁调 `writeJSON`，但响应只有 ~190 B，落在 net/http
的 2 KB bufio 里，handler 返回后才真正 flush，**当前不会卡在网络上**；只是超过 2 KB 的
接口（如 `/api/qq/caps` 4 KB）照这个写法写就会在锁内做 socket 写。

## 必须修的（按收益排序，含预期改善量）

| # | 修什么 | 怎么修 | 预期 |
|---|---|---|---|
| 1 | `yyBridge.marks`/`lastFire` 无界 | 每次 publish 后顺手清过期 key；或给 map 加条数上限（如 4096）+ 最旧淘汰；或把 per-player key 改成 `规则ID|玩家槽位` 并在玩家离线时删 | 内存从 **+10.1 MB / 10 万 key** 回到 O(活跃玩家×命中规则)，约 **-90%** |
| 2 | `/api/qq/login` 每 20 秒重算二维码 | ① `logged_in` 只用 QQ 侧判据（WS 已连 or 状态为 open），别掺游戏连接；② 二维码拆成 `/api/qq/login/qr.png?t=<mtime>`，用 ETag/`If-None-Match` 让浏览器缓存，JSON 里只回 URL 和 age；③ 未登录时的轮询间隔放宽到 60 s | 轮询流量 **-99%**（10 MB/h → <0.2 MB/h），TCP dial 从 4320 次/天降到必要时才拨 |
| 3 | `yyBridgePublish` 每次复制规则 | 换成 `atomic.Pointer[[]yyBridgeRule]`（写时整体替换）或持读锁遍历；再建 `map[event][]yyBridgeRule` 只遍历命中的 | 单事件 **19.5 µs/32.9 KB → <1 µs**；200 规则命中 **286 µs → 十几 µs** |
| 4 | `yyBridgeFire` 每规则一锁 | 一次加锁内遍历完所有命中规则，只在真正要执行动作时才放锁调外部 | 200 规则命中再降 3~5 倍锁开销 |

## 可接受但要知道的

- **`yyQQActionPolicy` 不是高频路径**：全仓库只有 `yyQQRunAction`（`:129`）和
  `allow()`（`:64`）调它，而 `yyQQRunAction` 只在「真的执行一次能力」时进
  （点赞/动态/转发/触发器的开火），**不是每条入站消息都走**。实测 130 ns、0 分配、
  并发 20 goroutine 也才 309 ns。一次动作里有 3 次 `yyQQSnapshot()`（`:115`、`:129`、`allow` 内），
  合计 ~390 ns，相对一次 OneBot WebSocket 往返（毫秒级）可忽略。**这里不需要优化。**
- **10 秒状态轮询很便宜**：6 次/分钟、1.1 KB/分钟、不拨 TCP、不进游戏、只在 state 变化时重绘。
- **`/api/qq/server/say` 限流是原子的**：`youyuan_qq_serverinfo.go:638-653`
  在同一把锁内完成「裁剪窗口 → 判断 → 追加」，check 和 append 不可分割，没有 TOCTOU。
  语义上它是全局 3 次/秒（不区分调用者），符合「防连点」的定位。
- **种子查询全部安全**：最坏 slime r=64 0.18 ms、query r=32 0.06 ms、structure 单轴 4096
  0.03 ms，都远低于 150 ms 预算，且有 region 数/结果数/时间预算三重上限；纯 CPU、
  无共享状态，并发安全。
- **`/api/qq/status` 持锁写响应**：当前 ~190 B 落在 bufio 里，实测无阻塞；但这是个
  坏范式，写 >2 KB 的接口别照抄。
- **玩家列表类接口**：1000 玩家 190 µs / 802 次分配，只在面板点开时调一次，可接受；
  真要优化就直接用堆选择前 N 个，不要全量排序。
- 前端 15 秒的 `/api/qq/messages`（40 KB/分钟）是已登录态下最大的流量项，比状态轮询
  大 36 倍；要省流量就改成 SSE 推送（项目里已经有 sse.js）。

## 数据竞争检查结果

- 本机 **无法跑 `go test -race`**：`-race` 需要 cgo，本机 PATH 里没有 gcc/clang
  （`CGO_ENABLED=1 go test -race` 报 `C compiler "gcc" not found`）。没有在服务器上跑。
- 替代手段：写并发压测（`TestPerfStressConcurrentMaps`，3 秒，18 个 goroutine 同时打
  `yyBridgePublish` / `yyBridgePlayerRows` / `yyQQActionPolicy` / `yyQQActions.allow+stamp+succeed` /
  `yyQQCapsView` / `yyQQHelperMutate`（真实写盘）/ 遍历 `Assistant.Caps`、`CapLimits` /
  `yySrvInfoSayAllow` / `yySeedQuery` / 快照 `yyBridge.Events`）。**结果 PASS，没有触发
  Go 运行时的 concurrent map read/write fatal。**
- 人工审查结论：
  - `yyQQSnapshot()` 返回的是 `yyQQConfig` 的**浅拷贝**，但所有写路径都用
    `yyQQHelperMutate`（`youyuan_qq_assistant.go:75-152`）先深拷贝 `SuperAdmins/Admins/Triggers/
    Caps/CapLimits`，再在锁内整体替换 `yyQQ`，属于 copy-on-write，读侧安全。
  - `yyBridge.Rules` 是整体替换（`youyuan_bridge_api.go:541`），`Events` 读写在锁内，
    `publish` 先复制再放锁，安全。
  - `yySrvInfoSayGate`、`yyQQActionGate`、`yySrvInfoBot` 都在自己的锁内读写。
  - 未覆盖：`yyQQPending`（OneBot echo 表）等本轮范围外的状态没审。
- 建议：上线前在 Linux 生产机跑一次
  `CGO_ENABLED=1 go test -race -run 'Bridge|Seed|Cap' .`（那台有 gcc），这是唯一能补上的一环。
