# 雪球菜单命令链卡顿回归 — Git 取证报告

调查人：dig-chain-regression ｜ 日期：2026-10-21 ｜ 方法：只读 git（git log/show/grep/diff），未修改任何 upstream 文件。

## 提交时间线（youyuan_commands.go 及链执行文件）

```
9f90893  最早基线（preserve framework source baseline）
9cd26f7  fix: restore fast command submission and condition-only chain barriers   ← 引入 fast 模式
729c59c  fix: reject writes after closure and distinguish fast submissions
19ba43c  feat: complete resident framework（与命令提交路径无关，仅 structure 导出）
0049249  fix: stabilize receive dispatch, chain control（收包顺序化 + @branch）
ed3a0cf  feat: deliver R21A resident chains ← 引入 youyuan_chain_segments.go 分段并行 + per-chain Hz
6f703ec  baseline: 接手 0.10.75 阶段（链执行代码与 ed3a0cf 行为等价，仅 Hz 配置统一化）
HEAD(71ba734)  0.10.78 —— 链执行核心文件相对 6f703ec 零改动
```

## 一、"一轮"如何执行（新旧对比）

**9f90893（以前）**：
- 前端 `youyuan_chain_upgrade.js:18` 默认 mode 为 `'remote'`（无 fast 选项）："严格顺序 · 每条确认后继续"。
- mode 无 fast 时走 `yyChainExecuteDetailed`（youyuan_chain_buffered.go:39-42），单节点经 `yyCommandBatchDefaultsContext(Mode:"ordered", Window:1, TimeoutMS:8000)`（youyuan_commands.go:432）**逐条提交并阻塞等待服务器 CommandOutput 确认**；未 confirmed 即报错（youyuan_commands.go:444）。
- 多段链在单 worker 的 `yyChainExecuteGraph` 中**串行**执行；整轮结束后 `yyChainRoundWait`（youyuan_commands.go:541）补足周期，超时轮不排队（"Overruns continue without queued rounds"）。
- 存在 `failures >= 3 停轮`（youyuan_commands.go:477-481）熔断。

**ed3a0cf 及以后（现在）**：
- 前端默认 mode 为 `'fast'`（`row?'remote':'fast'`，youyuan_chain_upgrade.js:18）。
- fast 批量路径（youyuan_commands.go:450-464，9cd26f7 引入）循环调用 `SendCommandNoWaitRate(ch, cmd, callCfg.RateCPS)`，**提交后不等回包**，只记 `submitted`。
- ChannelAI 走 `conn.WritePacket(yyFastSettingsPacket(...))`（bot_conn.go:1647-1670，9cd26f7 引入），fire-and-forget。
- RateCPS 默认 0 = **无任何限速**（command_dispatch.go:72-80："Zero means no artificial dispatch delay"）；729c59c 在 bot_conn.go:1643 注释确认 "neither adds a per-command sleep"。
- fast 窗口固定 1024（youyuan_chain_buffered.go:54），一段 80 条一次循环全速写出。
- ed3a0cf 引入 `runSegments`（youyuan_chain_segments.go，新增 187 行）：**每个 chain 段一个独立 worker goroutine、独立时钟并行运行**；引入前所有段串行于一个 run 循环。

## 二、"以前不卡"对应哪个版本

**9f90893**（或其代表的 0.10.7x 更早期线上版本）。证据：该版本无 fast 通道，链默认 remote 逐条等确认，提交速率天然被"每条一个响应往返"压制；20Hz 段 80 条命令实际只能以受 RTT 限制的速率（约几十条/秒）提交，且超时轮被 `yyChainRoundWait` 丢弃而非排队累积。

## 三、fast 提交路径上的行为变化（按嫌疑排序）

1. **9cd26f7 — 提交语义从"等确认"变"fire-and-forget 全速"**
   - youyuan_commands.go:450-464：fast 批量逐条 `SendCommandNoWaitRate(..., RateCPS)`，不等待、不合并、不去重。
   - bot_conn.go:1647-1670：ChannelAI 直写 SettingsCommand 包，绕过逐条 timer。
   - command_dispatch.go:72-80：`rate==0` 直接 `controlCommands.Add(count)` 后返回，无等待。默认 RateCPS=0（前端 youyuan_chain_upgrade.js:21 强制归零）→ 20Hz×160 条理论可达 3200 条/s 的命令风暴。

2. **ed3a0cf — 分段并行调度**
   - youyuan_chain_segments.go:16-100：每段独立 worker 并行、独立 50ms 时钟；引入前多段串行于单 worker（ed3a0cf 之前的 run + `yyChainExecuteGraph`），整链一轮的总耗时天然拉长轮间隔。

3. **9cd26f7 — fast 窗口 1024**
   - youyuan_chain_buffered.go:54：`if cfg.Mode == "fast" { window = 1024 }`，一段 80 条不再被旧 Window≤16 分批，一拍内全部写出。

## 四、20Hz×160 条曾被压制的证据

- 9f90893 youyuan_commands.go:430-445：ordered/Window:1/TimeoutMS:8000 + `confirmed!=true 即失败`——每条命令阻塞等 CommandOutput，有效速率 = 1/RTT 每条，160 条链轮被拉长到秒级。
- youyuan_commands.go:541（9f90893）：`yyChainRoundWait` 超时轮直接丢弃，无排队。
- 729c59c youyuan_commands.go:641：新增 `results_confirmed: mode != "fast"` 与 "counter_semantics" 字段，明确承认 fast 的 submitted 只是"宿主接收"，非服务器执行证明——即 fast 上线时官方确认了语义变化。

## 五、6f703ec → HEAD 无链执行代码改动

`git diff 6f703ec HEAD --stat` 在 youyuan_commands.go / youyuan_chain_segments.go / youyuan_chain_buffered.go / youyuan_chain_control.go / command_dispatch.go / youyuan_chain_api.go / youyuan_plugin_commands.go / flowers-for-machines/ 上**全部为空**（bot_conn.go 仅 +1 行）。

**结论**：卡顿不是 0.10.76-0.10.78 的提交直接改坏链代码；命令风暴行为在 6f703ec（0.10.75 baseline）时已由 9cd26f7（fast）+ ed3a0cf（segments）定型。"更新协议后卡"更可能是链文件从旧实例迁移/重建时 mode 落到默认 fast（旧数据 mode 为 remote/pipeline），或服务器负载放大了既有风暴。

## 缓解建议（供团队参考，未实施）

- 对 fast 模式给 RateCPS 一个安全默认值（如 200）而非 0；或将 20Hz 段的批量提交按服务器 tick 分帧。
- 短期把该链的执行方式改为"严格顺序 remote"即可复现旧行为。
