# 命令链最底层下发机制 — 只读取证报告

> 取证对象：`upstream/`（工作区，含未提交的 `youyuan_chain_fast_module.go`）、`upstream/flowers-for-machines/`、`maintenance/chain-dump/雪球菜单_综合功能.txt`、`~/.codex/sessions/**/*.jsonl`。
> 取证方式：只读。未修改任何代码文件，未连接游戏、未发送命令。
> 生成时间：2026-10-21。
> 与既有报告的关系：`maintenance/codex-module-chain-report.md` 已查清"模块方式"的历史归属与回归点（commit `9cd26f7`），本报告不重复那部分，只往下追到 RakNet 字节与服务器执行模型。

---

## 零、五条核心结论（先看这里）

1. **一条命令 = 一个包，不能一个包塞多条。** `SettingsCommand` 的载荷只有 `command`(string) + `suppressOutput`(bool) 两个字段（`upstream/flowers-for-machines/core/minecraft/protocol/packet/settings_command.go:9-27`）。用户记忆里的 `commandOrigin / internal / version` 是**另一个包** `CommandRequest` 的字段，两者不是同一个东西（见 §二）。
2. **"批量"发生在包的上一层，不在命令这一层。** 多个包会被 `0xfe` 批次包按 `varuint32 长度 + 载荷` 串接，整体压缩加密后作为**一次 `net.Conn.Write`** 交给 RakNet（`protocol/packet/encoder.go:80-125`）。所以"一批"= 一个批次包 = N 个命令包。
3. **`1024` 包 / `512KiB` / `2MiB` 三个常量都没有协议依据。** 它们全部来自项目最初基线 `9f90893`（`git log -S` 只能追到这一条），而协议侧唯一有出处的数字是解码器的 `maximumInBatch = 812`（`packet/decoder.go:96`，与 gophertunnel 同名常量一致）。**1024 > 812**。
4. **MTU 才是真正的物理上限，而它按"压缩后字节"算，不按"命令文本"算。** 1492 MTU 下单分片载荷 1440 字节（`raknet/packet.go:193-213`），512KiB 文本 ≈ 365 个分片，2MiB ≈ 1457 个分片，而接收侧对单个 splitID 的上限是 512 分片（`raknet/conn.go:561`）→ **约 720KiB 是实际可达上限，`WritePacketBatch` 的 2MiB 分支永远到不了**。
5. **服务端没有可证的"每 tick 命令配额"，也没有生效的节流。** 官方只文档化了 `functionCommandLimit`(默认 10000，仅 `/function`) 与 `maxCommandChainLength`(65536，命令方块/函数链)。客户端包路径不受这两条约束；而 TPS 掉到个位数这一事实本身就反证了"没有生效的限额"——有限额的话超出的部分会被丢弃，TPS 不会崩（§四）。

---

## 一、从命令数组到字节的完整调用链

### 1.1 两条路径（必须先分清）

| 路径 | 提交单位 | 代码位置 | 现状 |
|---|---|---|---|
| **A. HEAD 逐条**（回归后） | 1 条命令 = 1 次 `SendCommandNoWaitRate` | `upstream/youyuan_commands.go:528-531` 之前的老分支；`command_dispatch.go:71-79` | 线上 0.10.78 |
| **B. 整模块**（工作区未提交补丁） | 1 模块 = 1 次 `WritePacketBatch` | `upstream/youyuan_chain_fast_module.go:47-81` | 未提交 |

两条路径**最终写出的字节布局完全相同**（都是 N 个 `SettingsCommand` 子包串在一个批次包里），差别只在"批次边界由谁决定"：

- A：批次边界由 50ms 定时器决定（`conn.go:197-210` + `dial.go:231` 默认 `FlushRate = time.Second/20`）。3322 条/秒 ÷ 20 次/秒 ≈ **每次 flush 166 个包**，且这 166 个包可能来自不同模块、不同段、甚至跨轮次（5 个段是 5 个 goroutine 并发写同一个 `bufferedSend`）。
- B：批次边界 = 模块边界。模块发完才发下一个（`yyChainFastGate` 串行），一次 `Encode` 一次 `Write`。

**这一点很重要**：从"服务器收到多少命令"看，A 和 B 完全一样（都是 3322 条/秒）；从"服务器在同一个 tick 里看到什么"看，A 是"约 166 条杂牌混批"，B 是"164 条同模块整批"。这就是用户说的"模块方式"的物理含义。

### 1.2 完整调用栈（自顶向下，工作区 B 路径）

```
① 链调度器按 Hz 起轮
   upstream/youyuan_commands.go:450-491        yyChainInstance.run 的一轮
   upstream/youyuan_commands.go:528-531        batch 回调 → callCfg.Mode=="fast" → fastModule()
   upstream/youyuan_commands.go:469-470        fastModule → bm.sendChainFastModule()

② 模块切分（哪一段算"一个模块"）
   upstream/youyuan_chain_buffered.go:62-64    fast 模式 window = intmax（整段不分块）
   upstream/youyuan_chain_buffered.go:105-125  从 i 向后吃，遇到 Control/Conditional/DelayMS/换通道就停
   upstream/youyuan_chain_buffered.go:126      rows, err := batch(chain.Nodes[i:end])   ← 整段一次性交出
   upstream/youyuan_chain_control.go:313-320   batch 包装 → 检查 fatal → 透传

③ 模块提交（同账号串行）
   upstream/youyuan_chain_fast_module.go:51-55   chainFast.acquire(ctx) 取账号唯一槽位
   upstream/youyuan_chain_fast_module.go:57-65   取 conn / 连接态 / 维度 / 显示名，bind 连接
   upstream/youyuan_chain_fast_module.go:66-76   yyChainFastChunks(..., send)
   upstream/youyuan_chain_fast_module.go:68      waitControlCommandRateContext(len,1024,RateCPS)（rate=0 时不睡）
   upstream/youyuan_chain_fast_module.go:75      conn.WritePacketBatch(packets)

④ 逐条构包（命令 → 包对象）
   upstream/youyuan_chain_fast_module.go:105-121 为每个 node 生成 packet，累加文本长度
   upstream/youyuan_chain_fast_module.go:83-102  yyChainFastPacket
   upstream/youyuan_chain_fast_module.go:91      wrapCommandInDimension(cmd, dim) 后去掉前导 "/"
   upstream/bot_conn.go:1701-1713                wrapCommandInDimension（非主世界则加 "execute in <dim> run"）
   upstream/bot_conn.go:1807-1813                yyFastSettingsPacket → SettingsCommand{CommandLine, SuppressOutput:true}
   upstream/bot_conn.go:1810                     ★ 每条命令被再包一层 execute as @a[name="<bot>"] at @s run <cmd>

⑤ 批次包编码（N 个包 → 1 个 0xfe 批次）
   core/minecraft/conn.go:428-462                WritePacketBatch：加锁 → 逐包 writePacketLocked → 统计字节 → Encode 一次 → 清空
   core/minecraft/conn.go:379-410                writePacketLocked：写包头 varuint32，Marshal 载荷，拷进 bufferedSend
   core/minecraft/protocol/packet/packet.go:31-33 包头 = varuint32(PacketID | sender<<10 | target<<12)
   core/minecraft/protocol/packet/settings_command.go:24-27  Marshal：io.String(command) + io.Bool(suppress)
   core/minecraft/protocol/packet/encoder.go:88-104  每个子包前置 varuint32 长度，串成一个 buffer
   core/minecraft/protocol/packet/encoder.go:105-118 网易 flate 压缩（无结束标记的单 flate 块）→ 加密 → prepend 0xfe
   core/minecraft/protocol/packet/encoder.go:120     encoder.w.Write(data)   ← 整个模块一次 Write

⑥ RakNet（1 次 Write → N 个 UDP 数据报）
   raknet/conn.go:251-253                        Write → writeWithReliability(b, reliableOrdered)
   raknet/conn.go:278-279                        fragments := split(b, effectiveMTU())
   raknet/conn.go:130-134                        effectiveMTU = mtu - 28
   raknet/packet.go:193-213                      split()：单分片 = mtu - 28 - 14（首片）/ - 24（分片）
   raknet/conn.go:289-316                        每个分片一个 packet，写 splitCount/splitID/splitIndex
   raknet/conn.go:680-697                        sendDatagram：加 datagram 头 + seq，一次 WriteTo（一个 UDP 报文）
```

**"一次 WritePacketBatch" 在线上实际等于**：1 个批次包 → 压缩 → 切成 F 个 RakNet 分片 → F 个 UDP 数据报，全部 `reliableOrdered`，带重传队列（`conn.go:687-691`）。

### 1.3 一条命令的真实字节（以 `2,0,0 20` 段为例）

主段 164 条命令，原始文本合计 **30,758 字节**（实测统计，平均 187.5 字节/条）。单条线上结构：

```
[0xfe]                                  批次头（1 字节）
[varuint32 压缩算法ID?]                 网易算法不写（compression.go:196 注释：只接受无结束标记的 flate 块）
<flate 压缩后的整段>                     包含下面全部子包
   └─ [varuint32 len_1][varuint32 0x8C][string "execute as @a[name=..."] [bool true]
   └─ [varuint32 len_2][varuint32 0x8C][string ...] [bool true]
   └─ ... × 164
```

- `0x8C` = 本仓库 `IDSettingsCommand` 的数值（按 `packet/id.go` 枚举算得 140；该表被网易改过，**不要硬编码，不同版本会变**）。
- 加密开启后还有 AES-CTR + 8 字节校验尾（`encoder.go:118-121`、`decoder.go:147-154`）。
- 所以"一条命令一个包"成立，但"一条命令一个 UDP 报文"**不成立**——一个 UDP 报文装 1440 字节的压缩流，能装几十到上百条命令。

---

## 二、SettingsCommand 包结构（字段级）

### 2.1 实际字段：只有两个

```go
// upstream/flowers-for-machines/core/minecraft/protocol/packet/settings_command.go:9-17
type SettingsCommand struct {
    CommandLine    string  // 完整命令行（不含前导 "/"）
    SuppressOutput bool    // 请求服务器不要回 CommandOutput
}
func (pk *SettingsCommand) Marshal(io protocol.IO) {
    io.String(&pk.CommandLine)   // varuint32 长度 + UTF-8
    io.Bool(&pk.SuppressOutput)  // 1 字节
}
```

外部交叉验证一致：`NetEasePE/Protocol` 的 `SettingsCommandPacket.java` 就是 `{String command; boolean suppressingOutput;}`，没有任何 origin/version 字段。

### 2.2 用户提到的 `commandOrigin / internal / version` 属于 `CommandRequest`

```go
// upstream/.../packet/command_request.go:10-33
type CommandRequest struct {
    CommandLine   string
    CommandOrigin protocol.CommandOrigin  // Origin(uint32) + UUID(16B) + RequestID(string) [+ PlayerUniqueID 仅 DevConsole/Test]
    Internal      bool
    Version       int32                   // 本仓库为 varint32
    UnLimited     bool                    // 网易/PhoenixBuilder 专有
}
```

`CommandOrigin` 结构见 `protocol/command.go:265-291`；`Origin` 取值枚举含 `PLAYER / AUTOMATION_PLAYER / DEV_CONSOLE / SCRIPT / ...`。

**这两个包在本项目里是两条不同通道**：
- `ChannelAI`（默认、命令链用的就是这个）→ `SettingsCommand`（`bot_conn.go:1786-1794`、`youyuan_chain_fast_module.go:92-95`）。
- `ChannelPlayer` / `ChannelConsole` → `CommandRequest`（`youyuan_chain_fast_module.go:96-101`）。

选 `SettingsCommand` 是有意的：它**不带 RequestID，也不会产生 `CommandOutput` 回包**（`bot_conn.go:1787` 注释明确写了这一点）。如果换成 `CommandRequest`，3322 条/秒会附带 3322 个/秒的 `CommandOutput` 回包，服务器和客户端都会被反向流量压死。**这一条不能改。**

> ⚠️ 顺带记录一个隐患（未验证）：`CommandRequest.Marshal` 写的是 `string, CommandOrigin, bool internal, varint32 version, bool unlimited`（`command_request.go:36-44`），而 PhoenixBuilder 的 NetEase 实现写的是 `string, CommandOrigin, bool internal, bool unlimited`（**没有 version**）。两者不可能同时对。走 player/console 通道时如果出现"包被丢弃/静默失败"，优先怀疑这里的字段布局与当前网易服务端版本不匹配。

### 2.3 一条命令一个包，一个包不能塞多条

- 协议层：`CommandLine` 是单个 string，没有数组/分隔符字段。
- 应用层：所有入口都显式拒绝换行（`youyuan_plugin_commands.go:156`、`youyuan_command_batch.go:107`），因为服务端命令解析器不接受多行。
- Bedrock 也没有 Java 那种 `;` 分隔或"一条命令串多条"的语法；`execute ... run execute ... run` 只是嵌套执行上下文，叶子仍然是一条命令，且每层 `execute` 都要重新求值选择器。

**结论：想减少"包数"只能减少"命令数"，包这一层没有压缩空间。**

### 2.4 我们的实现额外加了一层包装（重要，见 §五 L6）

`yyFastSettingsPacket`（`bot_conn.go:1807-1813`）对**每一条**命令做：

```
execute as @a[name="<机器人显示名>"] at @s run <原命令>
```

实测该 TXT：**172 条命令里 155 条本身就以 `/execute` 开头，且 0 条非 execute 命令引用 `@s` / `~` / `^`**（脚本统计）。也就是说这层包装对整份 TXT 是纯冗余开销：每条命令多一次 `@a` 全服玩家选择器求值 + 多一层 execute 栈，3322 条/秒就是 3322 次/秒的额外选择器求值。

---

## 三、批量上限与 MTU 约束

### 3.1 四个不同层次的"上限"（不要混为一谈）

| 层次 | 常量 | 位置 | 依据 | 约束对象 |
|---|---|---|---|---|
| 命令条数/次提交 | 1024 | `conn.go:429`；`youyuan_chain_fast_module.go:122`；`youyuan_command_burst.go:18` | **无协议依据**（基线 `9f90893` 自带） | 包对象个数 |
| 命令文本字节 | 512KiB | `youyuan_chain_fast_module.go:117,131`；`youyuan_command_burst.go:59`；`youyuan_command_stream.go:185` | **无协议依据**（同上） | 未压缩 UTF-8 文本 |
| 编码后字节 | 2MiB | `conn.go:449-455` | **无协议依据**（同上），且实际不可达（见 3.3） | 未压缩的子包串 |
| 批次内包数 | **812** | `packet/decoder.go:96,191` | 与 gophertunnel `maximumInBatch = 812` 完全一致，是协议侧唯一有出处的数字 | 对端解码 |

关于 812：它是 gophertunnel 客户端解码器的保护值（"a compressed batch has more than this amount, decoding will fail"），本仓库只在 `limits=false`（即客户端连服务器）时关闭该检查（`conn.go:189-192`）。**服务端是否对我们发去的 1024 包批次做同样的检查，未证实**；但既然参考实现里 812 是"正常流量不会超过"的量级，把 1024 当安全值是没有依据的。当前这份 TXT 单模块 164 条，离两个数都远，**不是当下瓶颈**，但属于"迟早踩到"的坑。

### 3.2 这三个常量的真实来历

```
$ git log --oneline -S "512*1024" --all -- upstream/
9f90893 chore: preserve existing framework source baseline before staged repairs
$ git log --oneline -S "must contain 1..1024 packets" --all -- upstream/
9f90893 ...
```

即：**全部来自接手时的基线快照，仓库内没有任何注释/文档/测试说明为什么是 1024、512KiB、2MiB。** 对照 §3.1 的 812 与 §3.3 的 MTU 算式，这三个数应视为"经验值"，需要重新标定。

### 3.3 MTU 与分片：真正的物理约束

```
raknet/dial.go:315      mtuSizes = []uint16{1492, 1200, 576}   // 逐级探测
raknet/conn.go:32-33    minMTUSize = 400, maxMTUSize = 1492
raknet/conn.go:130-134  effectiveMTU() = mtu - 28              // 扣 IP+UDP 头
raknet/packet.go:180-183 packetAdditionalSize = 1+3+1+2+3+3+1 = 14
raknet/packet.go:186-188 splitAdditionalSize  = 4+2+4 = 10
raknet/packet.go:193-213 split(b, mtu)：
        maxSize = mtu - 14
        若需要分片：maxSize -= 10
```

代入 mtu=1492（最常见，局域网/公网均能通过）：

| 量 | 计算 | 结果 |
|---|---|---|
| 单分片最大载荷 | 1492 − 28 − 14 − 10 | **1440 字节** |
| 164 条命令（30,758 B 文本）压缩后 | flate 后约 5–8 KB | **4–6 个分片** |
| 512KiB 文本 | 524288 / 1440 | **≈ 365 个分片** |
| 2MiB 编码后 | 2097152 / 1440 | **≈ 1457 个分片** |
| 接收侧单 splitID 上限 | `raknet/conn.go:561` maxSplitCount=512 | **512 个分片** |
| 因此实际可达上限 | 512 × 1440 | **≈ 720KiB** |

**三条直接推论：**

1. `WritePacketBatch` 的 `2MiB` 分支（`conn.go:449`）是**死代码级**的保护：真发到 2MiB，接收侧（若启用同样的 512 分片限制）会整批丢弃，而且是"整批"——不是丢几条。
2. `512KiB` 限的是**文本**，但 MTU 卡的是**压缩后**。文本 512KiB 的命令可能压到 30KB（很安全），也可能因为 NBT/JSON 压不动而接近上限。用文本量做安全阈值方向对，但阈值本身没有标定。
3. 一个批次越大，**分片越多，任一 UDP 分片丢失都会导致整批 head-of-line 阻塞重传**（`reliableOrdered`）。所以"把 5 个模块合成一个大批次"在弱网下比"每模块一批"更脆弱——这从侧面支持用户的"模块方式"，但理由不是"服务器负载"，而是"重传放大"。

### 3.4 关于 50ms 与 256 包阈值

```
core/minecraft/conn.go:156, 197-210   newConn(flushRate) 起一个 ticker，每 flushRate 调 Flush()
core/minecraft/dial.go:231            FlushRate 默认 time.Second/20 = 50ms
core/minecraft/conn.go:566-585        Flush()：加锁 → 有内容才 Encode → 清空
core/minecraft/conn.go:353-377        WritePacketBounded：攒够 256 包或 256KiB 提前 flush
```

- `WritePacket`（A 路径用的）**不检查阈值**，只等 50ms ticker。
- `WritePacketBounded`（player/console 用）才有 256 包 / 256KiB 提前 flush；164 条到不了阈值，所以行为跟 `WritePacket` 一样。
- `WritePacketBatch`（B 路径）**立即 Encode + Write，不等 ticker**。ticker 随后跑空转（`bufferedSend` 已清空），不会重复发。

所以 A/B 两路径的**发送时刻**其实都是"每个 50ms 窗口一次"——B 是模块自己的节拍（20Hz 模块 = 每 50ms 一次），A 是 ticker 的节拍。**在网络层，两者没有数量级差异**。这一点必须让决策者知道：改回模块方式不会降低服务器负载。

### 3.5 一个已知的传输层缺陷（与性能无关，但影响可观测性）

```
core/minecraft/conn.go:652-653   SetWriteDeadline 是空实现
youyuan_chain_fast_module.go:74  // shortcut: the transport write has no context deadline
```

`WritePacketBatch` 同步写，没有 ctx 截止时间。一旦底层 UDP 写阻塞，调用方无法取消，只能等。这与"模块串行 gate"叠加时，会让一个卡住的模块阻塞后续所有模块。

---

## 四、服务端每 tick 执行模型

### 4.1 能确认的（有代码或官方文档依据）

| 事实 | 依据 |
|---|---|
| 客户端把 N 个命令包串成**一个批次包**，一次 `Write` 交给 RakNet | `encoder.go:88-125` |
| RakNet 用 `reliableOrdered` 传输，分片按序重组后才交给上层 | `raknet/conn.go:252,278-316` |
| 上层解码时是**一次性把整批拆成 N 个包**再逐个交给 handler | `packet/decoder.go:182-193`（`for b.Len()!=0` 循环拆包） |
| 参考服务端实现对 `CommandRequest` 是**在收包 handler 里直接执行，没有任何配额/限速** | dragonfly `server/session/handler_command_request.go`：`c.ExecuteCommand(pk.CommandLine)` 直调 |
| 官方文档化的每 tick 上限只有两条：`functionCommandLimit`（默认 10000，**仅 `/function`**）、`maxCommandChainLength`（65536，**命令方块与函数链**） | MS Learn GameRule 枚举；Minecraft Wiki Game rule；1.9.0 更新日志"functioncommandlimit (default 10k)" |
| 这两条 gamerule 都**不覆盖**客户端直接发来的 `SettingsCommand` / `CommandRequest` | 上条的定义原文均为 "by the /function command" / "from one execution source ... Applies to command blocks and functions" |

### 4.2 未证实的（明确标注，不做推断）

1. **未证实**：网易租赁服（BDS 分支）是否把 `SettingsCommand` 排队到主线程、每 tick 处理一次队列，还是直接在网络线程执行。没有任何公开文档或反编译证据支持其中任一种。
2. **未证实**：网易是否在 `SettingsCommand` 路径上加了自己的"每 tick 命令数上限"或"发包速率保护"。没有找到任何可引用的实现或文档。
3. **未证实**：`CommandRequest.UnLimited` 字段（`command_request.go:27`）是否真的能绕过某种服务端限制。PhoenixBuilder 里该字段恒为 `false`（`minecraft/command/sender.go`），没有任何注释说明它做什么。
4. **未证实**：一个批次包内的 164 条命令是否**必然落在同一个 tick**。`conn.go:426-427` 的注释本身就写了 "not one server Tick"。只能确认它们**在同一次网络读取里被解码**，之后是否同 tick 执行取决于服务端队列实现。

### 4.3 用事故现象反推（这条是可靠的）

用户实测：3322 条/秒打进去后**租赁服帧率掉到个位数、新玩家进不去**。

这个现象本身排除"存在生效的每 tick 配额"：

- 如果服务端有 100 条/tick 的硬上限，164 条/tick 只会**被截断**（丢掉 64 条），TPS 应保持 20；玩家能进，只是菜单少执行几条。
- 实际是 TPS 崩到个位数 → 说明服务端**把收到的命令基本都执行了**，主线程时间被命令执行吃满。没有生效的限流。

**所以"3322 条/秒会不会触发某种保护"的答案是：不会。它不会触发保护，它会把服务器直接拖死。** 这也是为什么"限速"是唯一真正有效的方向——而用户否决了它。于是只剩"降低每条命令的执行成本"这一条路（§五 L10）。

### 4.4 这份 TXT 的负载到底在哪（关键旁证）

对 `maintenance/chain-dump/雪球菜单_综合功能.txt` 做静态统计：

- 全文件 172 条命令，其中 **112 条**是同一个模式（含 110 条完全重复）：

```
/execute as @a[tag=return_surface] at @s run execute as @n[type=armor_stand,tag=surface_tp,
   x=-2000000,dx=4000000,y=-114514,dy=320000000,z=-2000000,dz=4000000] at @s if block ~ ~-1 ~ air run tp @s ~ ~-1 ~
```

- 选择器包围盒 `dx=4000000, dy=320000000, dz=4000000` 覆盖**整个世界**。Bedrock 求值 `@n[...]` 必须遍历该 AABB 内的实体再过滤。
- 20Hz × 110 条 = **2200 次全维度实体遍历/秒**，且 `execute as @a[...] at @s` 还要先遍历所有玩家。

**这才是 TPS 崩塌的主因，不是 3322 个包。** 3322 个包的编解码成本在服务端大约是"每 tick 解 166 个小包"，量级远小于 2200 次实体遍历。

---

## 五、不改语义的优化杠杆清单

> 判定口径：**"不改语义" = 命令文本、顺序、频率、执行次数、执行者上下文全部不变，只改"怎么送到服务器"。**
> 任何需要改动 `雪球菜单_综合功能.txt` 内容的方案，一律标"需改 txt"。

| # | 杠杆 | 可行性 | 预期收益 | 需改 txt | 结论 |
|---|---|---|---|---|---|
| L1 | 恢复"一模块一批"（`WritePacketBatch`） | **可行**，代码已在工作区 | 中（客户端侧）：批次边界=模块边界、1 次 flate 覆盖整段（压缩率更好）、1 次 `sendMu`、不再跨模块/跨轮次混批 | 否 | **建议保留**，但要向用户说明它**不降服务器负载** |
| L2 | 同 tick 内重复命令去重/合并 | **不可行** | — | 否 | 本 TXT 的 110 条重复是**累积位移**（每条把盔甲架往下 1 格，110 条 = 110 格）。去重直接改行为。通用去重必须逐条判定幂等性，执行层无法自动判定 |
| L3 | 用 `execute` 把 N 条压成 1 条 | **不可行** | — | 否 | Bedrock 无 `;` 分隔；`execute ... run execute ... run` 只是嵌套上下文，叶子仍 1 条，且每层选择器都要重算。把 110 条 `tp ~ ~-1 ~` 合并成 1 条 = 只下移 1 格，语义变了 |
| L4 | 转 `/function` / 命令方块链 / tickingarea | **不可行（客户端侧）** | 高（若能落地） | **是** | `/function` 需要服务器侧行为包，bot 没有文件写入权限；本 TXT 已有一条 `setblock 1019 151 1023 repeating_command_block 3`，说明作者本来就在用命令方块，但要把 164 条搬进去必须重写 TXT + 改世界 |
| L5 | 增量下发（只在状态变化时发） | **不可行（不改 txt 的前提下）** | 极高 | **是** | 本 TXT 的绝大多数命令是"轮询探测"（`@e[type=snowball]`、`@r[scores=...]`、`if score ... matches`）。bot 无法在不查询服务器状态的情况下判断"这一轮可以跳过"。要做增量必须在 TXT 里加前置条件 |
| **L6** | **去掉每条命令的 `execute as @a[name=..] at @s run` 包装** | **可行**（需白名单校验） | **中**：省掉 3322 次/秒 `@a` 全服选择器求值 + 3322 层 execute 栈 | **否** | 本 TXT 实测：155/172 条本身以 `execute` 开头，其余 17 条无一引用 `@s`/`~`/`^` → 包装 100% 冗余。实现时按"命令不含 `@s`/`~`/`^` 且不以裸 `@p/@r` 依赖执行位置"白名单放行，其余保留包装。**风险点：执行位置**（`@p[r=3]` 之类依赖执行坐标的选择器），必须逐条核对 |
| L7 | 模块级节拍对齐（每 50ms 最多一个模块批次） | 可行 | 低 | 否 | 本质是"按模块限速"，用户已否决 rate_cps，同类方案不建议再提 |
| L8 | 保持 `SuppressOutput=true`（不回 CommandOutput） | **已在做，不可回退** | 关键 | 否 | 若改 false，3322 条/秒会带回 3322 个 `CommandOutput`/秒，双向流量翻倍 |
| L9 | 保持 `SettingsCommand`（不要换 `CommandRequest`） | **已在做，不可回退** | 关键 | 否 | 换包 = 每条带 RequestID + 回包；且 `CommandRequest` 的字段布局在网易各版本间不一致（§2.2 隐患） |
| L10 | 收窄世界级选择器包围盒 | 不可由执行层完成 | **最高（唯一能救 TPS 的）** | **是** | 110 条 × `dx=4000000,dy=320000000,dz=4000000` 的全维度遍历是真正的负载源。执行层无法改写命令文本 |
| L11 | 模块间串行 gate 的副作用评估 | 需复核 | — | 否 | `sendChainFastModule` 的 gate 是**账号级**（`youyuan_chain_fast_module.go:16-35`），而 5 个段是 5 个独立 worker（`youyuan_chain_segments.go:47-56`）。20Hz 段每 50ms 持锁，1Hz 段和 0.0167Hz 段会被排队——顺序正确，但**慢模块会拖住快模块**。建议记录为已知耦合，不是 bug |

### 5.1 一句话总结第五节

**在"不改 TXT"的红线内，唯一有实际意义的优化是 L6（去掉冗余包装），它省的是客户端/服务端的选择器求值，量级是"每 tick 164 次选择器"，对 TPS 的帮助是有限的。**

**真正能把 TPS 拉回来的只有 L10（收窄包围盒）或 L5（增量下发），两者都必须改 TXT。**

L1 应该做，但它的价值是"批次边界确定、压缩率更好、失败可感知"，**不是"服务器不卡"**。这一点如果不说清楚，改完 L1 复测还会卡，会再次误判为"没修好"。

---

## 六、参考网址与开源仓库

### 6.1 协议库（可直接对照字段）

| 仓库 | 地址 | 关键文件 |
|---|---|---|
| gophertunnel（Go，Bedrock 协议事实标准） | https://github.com/Sandertv/gophertunnel | `minecraft/protocol/packet/settings_command.go`（只有 command + suppressOutput）<br>`minecraft/protocol/packet/command_request.go`（command/origin/internal/version）<br>`minecraft/protocol/packet/decoder.go:107`（`maximumInBatch = 812`）<br>`minecraft/conn.go`（`WritePacket` / `Flush` / `FlushRate`） |
| CloudburstMC/Protocol（Java） | https://github.com/CloudburstMC/Protocol | `bedrock/bedrock-common/src/main/java/org/cloudburstmc/protocol/bedrock/packet/SettingsCommandPacket.java`<br>`.../CommandRequestPacket.java`<br>`.../data/CommandOriginData.java` |
| **NetEasePE/Protocol（网易版协议，本项目的直接参照）** | https://github.com/NetEasePE/Protocol | `bedrock/bedrock-common/src/main/java/com/nukkitx/protocol/bedrock/packet/SettingsCommandPacket.java`（`String command; boolean suppressingOutput`）<br>`.../CommandRequestPacket.java`（`command / commandOriginData / internal`）<br>`.../data/CommandOriginData.java`（含 `AUTOMATION_PLAYER` 枚举） |
| PrismarineJS/bedrock-protocol + minecraft-data | https://github.com/PrismarineJS/bedrock-protocol<br>https://github.com/PrismarineJS/minecraft-data | `data/bedrock/proto.yml`、`data/bedrock/1.x/protocol.json`（settings_command / command_request 字段表） |

### 6.2 服务端实现（对照"服务端怎么处理命令"）

| 仓库 | 地址 | 关键文件 |
|---|---|---|
| dragonfly（Go 服务端） | https://github.com/df-mc/dragonfly | `server/session/handler_command_request.go`（收到即执行，无配额）<br>`server/session/command.go` |
| PocketMine-MP（PHP 服务端） | https://github.com/pmmp/PocketMine-MP | `src/network/mcpe/handler/` 下的命令请求处理器 |
| BDS 官方下载 / 协议文档入口 | https://www.minecraft.net/en-us/download/server/bedrock | 附带的 `bedrock_server` 与协议变更说明 |

### 6.3 网易系机器人实现（同类工程做法）

| 仓库 | 地址 | 关键文件 / 可借鉴点 |
|---|---|---|
| **PhoenixBuilder**（网易租赁服机器人，最直接参照） | https://github.com/DreamOneX/PhoenixBuilder | `minecraft/command/sender.go`：`SendCommand`（CommandRequest + Player origin）/ `SendWSCommand`（AutomationPlayer）/ **`SendSizukanaCommand`（SettingsCommand + SuppressOutput=true）**——与我们的 `yyFastSettingsPacket` 同构<br>`minecraft/protocol/packet/command_request.go`：NetEase 的 `UnLimited` 字段（**无 version 字段**，与我们的布局不同） |
| flowers-for-machines（本项目所用框架的上游） | https://github.com/OmineDev/flowers-for-machines | `game_control/game_interface/commands.go`（`SendSettingsCommand` 的原始实现）<br>`core/minecraft/raknet/`（RakNet 客户端） |

### 6.4 官方文档 / Wiki（每 tick 上限的出处）

| 资料 | 地址 | 要点 |
|---|---|---|
| MS Learn — GameRule 枚举 | https://learn.microsoft.com/minecraft/creator/scriptapi/minecraft/server/gamerule | `FunctionCommandLimit`："maximum number of commands that can be executed simultaneously by the /function command"；`MaxCommandChainLength`："maximum number of chained commands that can execute per tick" |
| MS Learn — gamerule 命令 | https://learn.microsoft.com/minecraft/creator/commands/commands/gamerule | `MAX_FUNCTIONCOMMANDLIMIT = 10000` |
| Minecraft Wiki — Game rule | https://minecraft.wiki/w/Game_rule | `maxcommandchainlength` 默认 65536，"per tick from one execution source ... Applies to command blocks and functions" |
| 1.9.0 (Bedrock) 更新日志 | https://feedback.minecraft.net/hc/en-us/articles/360021523671 | "The number of commands run through functions can be limited with the new gamerule 'functioncommandlimit' (default 10k) to limit performance issues" |
| Bedrock Wiki（社区，命令/选择器语义） | https://wiki.bedrock.dev/ | `execute`、target selector、AABB 求值语义 |

### 6.5 本项目内的既有取证（避免重复劳动）

| 文件 | 内容 |
|---|---|
| `maintenance/codex-module-chain-report.md` | "模块方式"的完整历史取证：5 段/172 条、回归点 `9cd26f7`、`WritePacketBatch` 从未被 fast 链使用过、旧 fast 实际因 window 校验跑不起来 |
| `maintenance/chain-regression-report.md` | Git 时间线与"以前不卡"对应的版本推断 |
| `maintenance/chain-dump/雪球菜单_综合功能.txt` | 本次统计的原始命令链（197 行 / 172 条） |

---

## 七、本次取证的边界（诚实标注）

**已确认（有代码行号或官方文档支撑）**：§一全部、§二全部、§三全部、§四 4.1 与 4.3、§五里 L1/L2/L3/L6/L8/L9 的判定。

**未证实（不要当结论用）**：
1. 网易服务端对 `SettingsCommand` 的排队/执行时机与并发模型。
2. 网易服务端是否有自己的每 tick 命令配额或发包保护。
3. 服务端是否对 >812 包的批次做拒绝。
4. 服务端接收侧是否启用 512 分片上限（决定 720KiB 这个实际上限是否成立）。
5. `CommandRequest.UnLimited` 的语义。
6. 一个批次内的命令是否必然同 tick 执行。

**未做**：未连服务器、未抓包、未跑测试、未测真实 TPS。§4.3 的结论是从用户实测现象反推的，不是直接测量。

**建议的下一步验证（离线可做，零风险）**：
1. 用假 conn + 捕获器跑一遍雪球 TXT，断言：5 段命令数 `3/164/2/2/1`、每段恰好 1 次 `WritePacketBatch`、每批包数 = 段长度、包序与 TXT 行序一致。
2. 断言 `yyFastSettingsPacket` 输出的 `CommandLine` 长度分布，算出实际压缩前字节，对照 §3.3 的分片表，确认每段分片数（预期 4–6）。
3. 用 `git grep -n "512\*1024\|1024"` 的输出作为"常量无依据"的附件，提请重新标定为 812 / 720KiB。
