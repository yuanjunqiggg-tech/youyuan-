# 「模块方式发」取证报告

> 取证范围：`C:/Users/Lenovo/.codex/sessions/**/*.jsonl`（25 个会话，442MB）、项目 git 历史、`upstream/` 源码、`upstream/skills/` 文档。
> 取证方式：只读。未修改 `upstream/` 任何文件。
> 生成时间：2026-10-21

---

## 一、一句话结论

**「模块方式发」= TXT 里以 `.` 分隔的每个独立段（本例 5 个模块 / 172 条）各自攒成一个完整数组，一次宿主调用（Lua `CommandBuffer():submitFast()` → `game.sendCommandsFast(array)`）把**整段**交下去，模块与模块之间串行、不交叉；而不是 fast 链现在这样由 Go worker 逐条 `SendCommandNoWaitRate` 一条一条塞进共享的 50ms / 256 包发送缓冲。**

证据强度分级：
- **确凿**：用户原话、「5 个模块 / 172 条 / 3322 条每秒」的核实结论、「旧缓冲没有按模块划分边界」的诊断、`CommandBuffer.submitFast` 的实现与文档。
- **未确凿**：旧版 fast 链是否真的走过 `WritePacketBatch`（单网络批次）。git 全历史中 `WritePacketBatch` 只被 `burst`/`stream` 使用，fast 链从未用过（见 §四）。

---

## 二、证据清单

### E1 用户原话（唯一一次明确提出「模块」）
- 来源：`.codex/sessions/2026/10/20/rollout-2026-10-20T16-47-36-01a128b3-...01a1587e-....jsonl` **第 500 行**
- 时间：2026-10-20T17:41:23Z（北京时间 10-21 01:41）
- 原文：

> 1.我把你的项目接给了别人 别人乱改命令链 导致现在他使用170命令的那个雪球菜单相关txt文件 会导致服务器直接卡死 不仅仅是 机器人 卡死 服务器那端也会收到同样影响
> 也就是我们这个，**它原本是一个模块一个模块的快速打出去对吧？**
> 我看他这种情况，糟糕的，有点像 **一瞬间3200多行命令 依次发出**

关键：用户自己把「模块方式」和「依次发出」当成**对立面**。他记的「模块」不是插件模块，是 TXT 的分段。

### E2 Codex 核实：5 个模块 / 172 条 / 3322 条每秒
- 来源：同文件 **第 1222 行**，时间 2026-10-20T18:34:19Z
- 原文：

> 原文件已核实：**它是 5 个模块、共 172 条命令**；按原定频率连续运行，理论提交量约为每秒 3322 条，并非一轮展开成 3200 多行。接手前后的 Git 差异也没有显示这次改动碰过底层网络协议。

- 同文件 **第 610 行**（17:47:34Z）：

> 已确认两点：这份 TXT 是 172 条命令，按其中各模块的频率计算，持续运行时理论约提交 3322 条／秒；目前证据不支持"一轮发了 3200 条"。

**这条同时否掉了两个误判**：① 不是「一轮 3200 行」；② 不是「接手方改了网络协议」。

### E3 Codex 定位到的真实缺口：缓冲没有模块边界
- 来源：同文件 **第 1498 行**，时间 2026-10-20T18:48:24Z
- 原文：

> 调度器核对完了：没有发现"漏掉的轮次集中补发"或正常启停后旧 worker 叠加；但**旧发送缓冲没有按模块划分边界，50ms 刷新可能拆开一段，也可能合并相邻轮次和其他模块**。
> 我会收敛修复范围：**同一账号的 Fast 模块串行提交，每批限制包数和字节量**，写入失败立即停止、不自动重发。

- 同文件 **第 657 行**（17:50:33Z）：

> 快发路径确实缺少服务器反馈：它**按模块周期继续提交**，连接还能写入就记作成功。

### E4 代码证据：模块 = 一次整批提交（测试断言 batches == 1）
- 文件：`upstream/youyuan_snowball_incident_test.go:11-81`（该文件为 10/21 01:54 新建、尚未提交）
- 原文（节选）：

```go
segments := []struct{ header string; count int; hz float64 }{
    {"1,0,0", 3, 0}, {"2,0,0 20", 164, 20}, {"2,0,0 1", 2, 1},
    {"2,0,0 20", 2, 20}, {"2,0,0 0.016666667", 1, 0.016666667},
}
...
if plan.Commands != 172 || len(plan.Chains) != 5 ... 
if !reflect.DeepEqual(got, want[i]) || **batches != 1** || records != segment.count {
    t.Fatalf("segment %d changed order or repeated a single round: ...")
}
```

**这就是雪球菜单 TXT 的真实形状**：5 段、172 条，且测试强制「每个模块恰好 1 个批次、顺序不变、一轮不重复」。

### E5 代码证据：模块级串行 + 整批写入
- 文件：`upstream/youyuan_chain_fast_module.go:47-81`（10/21 02:54 新建，未提交）

```go
release, err := bm.chainFast.acquire(ctx)   // 同账号模块串行，一个模块发完才发下一个
...
rows, unknown, err := yyChainFastChunks(ctx, nodes, cfg, dim, name,
    func(packets []packet.Packet) (bool, error) { ... return true, conn.WritePacketBatch(packets) })
if unknown { bm.chainFast.poisoned = err }   // 结果未知即停本连接后续 Fast
```

- 同文件 `:122-151`：`limit := 1024`（RateCPS==0 时），整段 `from→to` 一次 `send(packets[from:to])`，即**一个模块 = 一次 `WritePacketBatch`**。

### E6 代码证据：fast 模式的窗口就是「整段」
- 文件：`upstream/youyuan_chain_buffered.go:62-64`

```go
if cfg.Mode == "fast" {
    window = int(^uint(0) >> 1) // A whole uninterrupted chain segment; no artificial 1024-node chunking.
}
```

注释原文即「整段不间断的链」。批处理回调 `batch(chain.Nodes[i:end])` 一次拿到整段（`:126`）。

### E7 回归点：fast 从「整段数组」退化为「逐条循环」
- 文件：`upstream/youyuan_commands.go` 工作区未提交 diff（相对 HEAD）
- 旧代码（HEAD / 线上 0.10.78）：

```go
if callCfg.Mode == "fast" {
    rows := make([]map[string]any, len(nodes))
    for i, n := range nodes {
        e := bm.SendCommandNoWaitRate(CommandChannel(yyChainNodeChannel(n, callCfg)), n.Command, callCfg.RateCPS)
        ...
    }
}
```

- 新代码（工作区）：

```go
if callCfg.Mode == "fast" { return fastModule(callCtx, nodes, callCfg) }
```

**这就是「依次发出」的代码本体**：172 条 → 172 次 `SendCommandNoWaitRate`，5 个模块 worker 并发写进同一个缓冲。

### E8 引入时间：2026-10-03 commit `9cd26f7`
- `git show 9cd26f7 -- upstream/youyuan_commands.go`，commit message：`fix: restore fast command submission and condition-only chain barriers`
- 该 commit **新增**了上面那段逐条循环。其父版本（`9cd26f7^`）的 fast 走的是：

```go
value, err := yyCommandBatchDefaultsContext(callCtx, bm, []any{values}, yyCommandOptions{Mode: mode, Channel: channel, Window: window, ...})
```

即 **一次宿主调用交整个数组**（mode=pipeline/burst/stream，window=len(nodes)）。

### E9 「模块方式」的正式 API：`CommandBuffer.submitFast`
- `upstream/youyuan_plugin_commands.go:272-279`（注释与实现）：

```go
// Build in Lua; submitFast crosses the Lua/Go boundary only once per buffer.
function CommandBuffer() ... submitFast=function(self,options)return game.sendCommandsFast(list,options)end
```

- `upstream/skills/cpp-runtime/index.md:133-141`：

> 高频命令使用 `CommandBuffer.submitFast`，**一轮整体交给宿主**，不为每个节点建立定时器。
> `local r=b:submitFast() -- 一次宿主边界，整数组按序提交；没有1024行限制`

- `upstream/skills/cpp-runtime/communication.md:76`：

> 业务模块先准备命令数组，再通过 **CommandBuffer 或 sendCommandsFast 整体提交**。

**这是全项目唯一一处把「模块」和「整体提交」写在一起的官方措辞。**

### E10 逐条路径的物理代价（为什么「依次发出」会打崩服务器）
- `upstream/command_dispatch.go:71-79`：`SendCommandNoWaitRate` → `sendCommandBuffered`
- `upstream/bot_conn.go:1777-1810`：AI 通道每命令一个 `SettingsCommand`，走 `WritePacket`
- `upstream/flowers-for-machines/core/minecraft/conn.go:336-338`（`WritePacket` 注释）：

> The encoded data is buffered until the next **20th of a second**, after which the data is flushed.

- 同文件 `:426-428`（`WritePacketBatch` 注释）：

> queues an entire bounded command burst under one send lock and flushes it once. This guarantees packet order and **one network batch**, not one server Tick.

结论：旧路径下 3322 条/秒 = 3322 个独立 `SettingsCommand`，按 50ms 定时器凑成 ~166 包一批冲出去，**批次边界由定时器决定，与模块边界无关**；新路径下 = 20 批/秒 × 164 包，一个模块一批，顺序与边界固定。

### E11 文档侧的一致性
- `命令链插件API.md:7`：「无条件、同通道、无延迟的**连续段一次交给底层**，按原文順序发送」
- `命令链插件API.md:57`：「32条以内且可装入用户窗口的短链**一次提交**」
- `命令链插件API.md:33`：「**模块的状态与启停**可由插件 chains.start/stop/configure 管理」（此处「模块」= 链/段）

---

## 三、旧实现 vs 现实现对比表

| 维度 | 旧（用户认可「模块方式」） | 现（fast 20Hz 全速，出事故） | 10/21 补丁（工作区未提交） |
|---|---|---|---|
| 提交单位 | 一个模块 = 一次宿主调用交整个数组 | 一条命令 = 一次 `SendCommandNoWaitRate` | 一个模块 = 一次 `WritePacketBatch` |
| 边界 | 模块边界 = 缓冲边界 | 缓冲边界由 50ms / 256 包定时器决定，可拆段、可跨模块合并（E3） | 模块边界 = 网络批次边界，串行不交叉 |
| 模块间并发 | 串行（`chainFast` 门） | 5 个 segment worker 并发写同一缓冲 | 串行（`yyChainFastGate`） |
| 写入失败 | 停止、不重发 | 不感知，继续按周期提交（E3/E4） | 立即停本连接后续 Fast（poison），重连恢复 |
| 单批上限 | 无 1024 行限制（文档口径） | 无（但拆成 1 条/批） | 1024 包 / 512KiB |
| 每秒命令量 | **同样是 3322 条/秒** | 3322 条/秒 | **同样是 3322 条/秒** |
| 每秒网络批次 | ~20 批 × 166 包（定时器驱动） | 同上，但内容跨模块交错 | 20 批 × 164 包（模块驱动） |

### ⚠️ 必须让 team-lead 知道的一句话
**「改回模块方式」只改客户端的提交形态（批次边界、顺序、串行度），不降低服务器每秒执行 3322 条命令的负载。** 用户描述的「帧率个位数、新玩家进不去」是服务端主线程被命令执行吃满的症状，与客户端怎么打包关系有限。旧版之所以「不卡」，最可能的原因是当时链**没有**跑在 20Hz 全速（旧 fast 实际不可用：见 §四线索 1），或跑在 pipeline/remote 有回包等待的自然限速下。这一点 Codex 在会话里也留了保留意见：

> L1591（18:53:01Z）：原菜单是否仍拖低服务器性能还需要单独验证。

---

## 四、未找到 / 不确定的地方（诚实标注）

### 未找到
1. **「旧版 fast 用单个网络批次把整模块打出去」的代码证据 —— 未找到。**
   `git log -S "WritePacketBatch"` 全历史只有一次（`9f90893` 引入），且只被 `youyuan_command_burst.go:108`、`youyuan_command_stream.go:223` 调用。fast 链在任何历史版本中**都没有**用过 `WritePacketBatch`。
2. **用户 10/20 之前针对雪球菜单卡顿的任何原话 —— 未找到。** 10/11、10/14、10/16、10/17 会话中「卡」的反馈全部指向抓包/UI/导入，与命令链无关。
3. **WorkBuddy 改动网络协议的证据 —— 未找到。** Codex 两次核对（L610、L1222）结论一致：接手前后 git 差异未触及底层协议；线上 0.10.78 二进制哈希与部署记录一致。

### 最接近的 3 个线索
1. **旧 fast 模式当时是坏的（最可能解释「以前不卡」）**
   `9cd26f7^` 的 fast 走 `yyCommandBatchDefaultsContext` 且 `window=len(nodes)`，而 `youyuan_command_batch.go:124` 校验 `pipeline/submit window=1..16` —— **超过 16 条的模块必然报错**。所以 2026-10-03 之前，20Hz×164 条的 fast 链根本跑不起来；用户记忆里的「一直跑起来」，要么是更早的 Lua `CommandBuffer` 插件版，要么是 mode 不是 fast。
2. **`CommandBuffer.submitFast` 是唯一被官方文档称为「模块整体提交」的入口**
   `youyuan_plugin_commands.go:272` + `skills/cpp-runtime/index.md:133`。如果用户要的是「插件模块自己发」而不是「链运行器发」，这就是原实现。
3. **Codex 自己的修法（已落盘未提交）就是「模块方式」的现代版**
   `youyuan_chain_fast_module.go` + `youyuan_snowball_incident_test.go`（10/21 01:54 / 02:54）。它把 fast 重新接回「一个模块一次整批、模块间串行、失败即停」，正是用户描述的形态；但**尚未在真服验收**（会话内前两轮回执方案都失败并回滚）。

---

## 五、时间线

| 时间（北京） | 事件 | 来源 |
|---|---|---|
| 2026-09-27 ~ 10-02 | 命令链/独立段（segments）设计落地，R20G 雪球菜单 TXT 成型（5 段 172 条） | `samples/ds_ai_agent_r21b/command_txt/雪球菜单_综合功能.txt` |
| 2026-10-03 10:14 | commit `9cd26f7`：fast 改为**逐条** `SendCommandNoWaitRate`（「恢复 fast 提交」） | git |
| 2026-10-12 | R21A 独立段（segments）+ 本机 MCP 文档 | `命令链插件API.md:175+` |
| 2026-10-19 23:30 | commit `6f703ec`「接手 0.10.75 阶段」，含 Hz 语义改动（未碰发送路径） | git |
| 2026-10-20 17:41 | **用户报告**：雪球 TXT 把服务器打崩，「原本是一个模块一个模块地打出去」 | 会话 L500 |
| 2026-10-20 17:47 | Codex 核实：172 条 / 5 模块 / 3322 条每秒；非一轮 3200 行；非协议改动 | 会话 L610 |
| 2026-10-20 18:34 | 再次核实「5 个模块 172 条」，定位「Fast 不等响应 + 结果未知无保护」 | 会话 L1222 |
| 2026-10-20 18:48 | 定位真正缺口：**旧缓冲没有模块边界**，决定改「模块串行 + 每批限包限字节」 | 会话 L1498 |
| 2026-10-20 18:53 | 最终候选通过测试与 go vet，开始构建 | 会话 L1591 |
| 2026-10-21 01:54 / 02:54 | `youyuan_snowball_incident_test.go` / `youyuan_chain_fast_module.go` 落盘（**未 commit**） | 文件 mtime |

---

## 六、两条容易踩错的边界（与 perf-conn-info 对齐）

1. **命令链 fast 不走本机 HTTP（19429），不存在与网页请求抢中继槽位。**
   `youyuan_chain_fast_module.go:83-101` 直接构造 `packet.SettingsCommand`，由 `:75` 的 `conn.WritePacketBatch` 写入 **RakNet 连接**（网易游戏服），全程不经过本机端口，也不进 `youyuan_remote_admission.go` 的 32 槽（`:25`、`:166-169`）。`REMOTE_REQUEST_LIMIT` 只可能来自 MCP/网页/中继侧调用。
2. **缓冲粒度是「每账号一条 Conn」，不是每进程。**
   `youyuan_commands.go:465-467` 按 `cfg.Account` 取 `yyAccountBot(...)`，每个 `BotManager` 持独立 `bm.conn`；`bufferedSend` 与 50ms ticker 都挂在 `Conn` 上（`conn.go:336`）。因此**多账号同时跑雪球菜单 = 服务端承受 N × 3322 条/秒**，客户端侧没有任何跨账号合并或节流。

   **但该场景当前未发生**（perf-conn-info 实测，2026-10-21）：
   ```
   19429  account_id=main                     connected:true   server=98014267
   19501  account_id=pool_3f5378b8-...        connected:false  relay: enabled=false
   19502  account_id=pool_d1f9f855-...        connected:false  relay: enabled=false
   ```
   两个子进程均未连接游戏、中继关闭。所以本次事故**只可能是 main 单账号**造成的，`N ×` 属于潜在放大项而非已发生事实。

---

## 七、给 team-lead 的建议（一句话版）

如果目标是「恢复用户认可的形态」：工作区里 Codex 的 `sendChainFastModule`（模块串行 + 每模块一次 `WritePacketBatch` + 失败即停）就是它，直接验收即可；但**不要向用户承诺这会解决 TPS**——3322 条/秒的执行量没变，真正要谈的仍是「模块频率 × 命令条数」这个乘积本身。
