# PROJECT_STATE.md — 游园工具箱唯一项目状态

> 本文件是项目状态的**唯一权威来源**。每次上下文压缩、换会话、换 AI 接手后，**先读本文件再动手**，不得凭聊天记忆行事。
> 与本文件冲突的聊天内容一律作废；发现记录缺失或互相矛盾时，**先停下向用户确认**，不得自行补脑。
> 更新规则：每完成一个模块或一个阶段 → 更新本文件 → 提交 GitHub，提交信息写明回滚点。

- 最后更新：2026-10-20 14:45（北京时间）
- 当前阶段：**0.10.75 三平台发布完成 + 待真机验收**（Cloudflare 主站 / 认证服务 / Linux 服务端均已上线 0.10.75）
- 线上基线：**0.10.75 / versionCode 1075**（原 0.10.74 已被替代；Linux 二进制 `948650bd…93511`，认证服务独立版本仍为 0.10.70）
- 整体完成：**否**（总账 `all_complete=false`；真机覆盖升级与端到端验收未做）
- 最近回滚点：**`rb-0.10.75-cloud-version`**（云端 worker 版本号 + 状态文档；Linux 二进制可用 `20261020-140806_0.10.75-linux-binary替换前` 或服务端 `/var/backups/youyuan/youyuan-before-01075`）

---

## 1. 项目目标

游园（Minecraft 网易版）工具箱，一套自带网页控制台 + Linux 服务 + Android APK 的宿主系统：

1. **原创 UI 体系**：导航、终端、插件中心、AI 工作区、账号、设置均为自有设计，功能不少于原 APK，且能让用户看出是成熟体系而非改版。
2. **插件与运行器**：Lua / Python / C++ / 命令链多语言插件，启停删有状态、有确认、有结果；运行器性能与稳定性可量化（用户目标：整体约 200 倍，追加专项至少 20 倍）。
3. **观测能力**：多机器人按维度常驻，仅用原生摄像机视角观察真人，排除机器人，不递归投放，不暴露内部原理。
4. **连接与出口**：租赁服 / 联机房间 / 本地房间自动识别与选择；默认用户出口与官方出口可选，并有真实路由证据。
5. **账号体系**：真实官方头像、账号合并、托管策略、权限隔离，不伪造令牌、不借用他人身份。
6. **导入导出与建筑**：断点续跑、多机协作、NBT 兼容、进度面板。
7. **QQ 群互通与 AI 工作区**：群消息双向、身份可辨、上下文有界；AI 工作台具备技能 / MCP / 预算 / 停止恢复。
8. **交付可回滚**：源码、需求总账、证据、发布产物分层留存，任何阶段可回到已知良好状态。

## 2. 当前任务清单

总账文件：`需求逐项总清单.json`（120 项，唯一计数来源，`updated_at=2026-10-20T01:17:59`），活动清单由 `项目待办.md` 呈现。

| 状态 | 数量 |
|---|---|
| 未完成 | 78 |
| 部分完成／待验收 | 35 |
| 已完成（验收归档） | 5 |
| 已修复（历史验收） | 1 |
| 已撤回／不执行 | 1 |
| **合计** | **120** |

> 口径说明（2026-10-20 01:45 校正）：本节此前写「119 项 / 83·30·4·1·1」，落后于总账。总账是唯一计数来源，故以总账实际统计为准；`closed_at` 非空 5 项与「已完成 5」一致。如用户要求保留 119 口径，须先说明合并了哪一项。

**本阶段（0.10.75）聚焦的条目**：D13（原创 UI 与入口）、D14（插件检查更新 / 市场）、B04（插件启停删状态）、B08（运行器性能）、Q01（AI 工作区）、E01/E05（房间识别与连接）、F02/F14（观测方式与原理私有）、O02（OpenCode 统筹）。

## 3. 已完成 / 未完成

### 3.1 已确认完成（有证据，可对外说）

- 0.10.74 已发布：网页 / Linux / APK 三端版本一致，原签名保留，公网整包 SHA 与 Linux 二进制 SHA 已核对（`outputs/2026-10-19-目录插件修复与最终交付验收.md`）。
- 插件目录修复：原 `main.lua` 源码根丢失问题已修，两个原 Lua 插件运行无加载错误；完整备份含 23 模块且排除邻居文件。
- 认证服务独立版本 0.10.70 `link-tcp-20261019.4`：真实 Workers TCP 桥可用；R2 密钥 `TrimRight` 误删末尾零的缺陷已修（**不得回退**）。
- 账号界面：云端与设备同 UID 合并、官方头像显示、未知头像中性占位已上线。
- Ponytail 5.3.2 已安装并读回启用（节省比例未量化）。
- 268 项 web/auth 测试、`go test bot-apk` 在本阶段改动后通过（本地层，不等于端到端）。

### 3.2 未完成（明确未通过，不得宣称完成）

- **0.10.75 全部改动未构建、未部署、未真机/真服验收、未更新总账**。
- **抓包四个缺陷只到「本地可构建 + 单测通过」层**：未打安装包、未部署、未在真机真服上复现原症状与确认消失；race 检测本机不可用（无 gcc，cgo 关闭）。
- 用户第 8 条：仅视角观察时不应要求旁观模式；普通界面仍写「采集账号需OP旁观」，F14「原理私有」未收口。
- B08 整体 20 倍 / 200 倍目标未达到，仅列表争锁局部改善约 290.7 倍，整对耗时无提升。
- E06 默认用户出口仍为 Linux 系统 UDP，**不是玩家手机出口**。
- 手机覆盖升级、真实外部 LLM 长任务、RSS/Token 对照、真 QQ 群回执、跨维度多机观测均未验收。
- O02 OpenCode 实际部署未落地。
- **既有测试失败 2 项（非本次引入，未处理，需用户裁决）**：
  - `bot-apk/cmd/server`：`import "bot-apk" is a program` —— `cmd/server` 是 `package main` 却导入同为 `package main` 的根包，结构上不可能编译；仅文档引用，构建脚本未用；本次 diff 未触碰。
  - `bot-apk/internal/playerrelay` `TestEncryptedLoginOnlyPreservesOpaqueWorldAndInput`：连跑 3 次结果为 通过 / 失败（`dial udp 127.0.0.1:58449: i/o timeout`）/ 通过 —— UDP 回环偶发，与本次改动无调用关系。

### 3.3 上一阶段（0.10.75）改动清单

| 类别 | 文件 |
|---|---|
| 前端 | `youyuan_navigation.js/.css`、`youyuan_terminal.js/.css`、`youyuan_plugin_studio.js/.css`、`youyuan_room_discovery.js/.css`、`youyuan_agent_studio.js/.css`、`youyuan_workspace.js`、`index.html`、`pages/plugins.js` |
| 宿主 Go | `youyuan_room_discovery.go`、`youyuan_observer_dimension.go`、`respawn_responder.go`、`packet_listener.go`、`packet_handler.go`、`main.go` |
| 认证 | `auth-service/room_discovery.go`、`tan_lobby_contract.go`、`engine.go`、`worker.mjs` |
| 测试 | `room-discovery.test.mjs`、`terminal-workbench.test.mjs`、`plugin-studio*.test.mjs`、`editor-file-order.test.mjs`、`agent-studio-lifecycle.test.mjs` 等 |

### 3.4 本阶段模块：抓包四个缺陷（已修，仅本地验证）

提交 `3a31201`，tag `rb-0.10.75-capture-fix`。

| 缺陷 | 根因 | 处理 | 文件 |
|---|---|---|---|
| 全局抓包变全局丢包 | `yyCapturePacket` 在唯一 `consumePackets` goroutine 上同步 `packetToMap`+`json.Marshal`，整条收包链路被阻塞，RakNet 窗口超限后真实丢包；`yyCaptureWants` 在接收线程抢 `yySenseState` 锁 | 2048 有界队列 + 专用 `yyCaptureDrain`/`yyCaptureStore` goroutine；消费线程只做廉价过滤与非阻塞入队，满队计 `queue_dropped`；不可变 `captureMatcher`（`atomic.Pointer`）无锁预筛；`yyCaptureFlush` 读前排空 | `youyuan_capture.go`、`bot_conn.go` |
| 地狱抓不到包 / 无血量 | camera 模式只发 `camera @s set`，机器人身体留原地，服务端不向其所在区块外广播 `UpdateAttributes`/`ActorEvent` | `body_radius_blocks`（默认 64）；`execute at <player> as <bot> if entity @s[distance=64..] run tp @s ~ ~ ~` 由服务端判定后只移动机器人身体；selector 模式每 10 次节流一次 | `youyuan_observer_watch.go`、`youyuan_selector_camera.go` |
| 三机器人有一个不去末地 | `yyCaptureGrantOP` 要求 `a.CaptureOnly`，手动加入账号池的机器人被跳过 → 永不 ready | 去掉该限制，手动与自动账号一视同仁 | `youyuan_capture_runtime.go` |
| 多人丢包严重 | `yyWatch.run`/`runSelectorCamera` 查询失败、跨维度、发送失败时 `return`，一次偶发即永久停摆 | 计数 + 最多 5 次重试；`snapshot` 暴露 `query_failures`/`dimension_skips`/`last_skip` | `youyuan_observer_watch.go`、`youyuan_selector_camera.go` |

统一停止入口 `yyCaptureStop`：`/api/youyuan/game/call` 与 `events.captureStop` 两条路径共用，避免 matcher 与 `Active` 状态不一致（第二处入口 `youyuan_sense_api.go:599` 原先直写 `Active=false`，已改）。

界面文案修正：`youyuan_global_capture.js` 由「采集账号需OP旁观」改为「采集账号需OP；默认只用摄像机视角、不切旁观模式，为接收实体与属性包会把身体保持在目标附近」。

验证层级：
- 静态核实：`go vet .` 通过。
- 可构建：`go build ./...` 通过（`cmd/server` 为既有结构性失败）。
- 已运行：`go test . ./config ./state ./building` 全通过（19.5s / 3.8s / 0.5s / 13.8s）；新增回归 5 项见 `youyuan_capture_queue_test.go`。
- 未执行：race 检测（本机无 gcc）；安装包构建、部署、真机真服端到端。

### 3.5 本阶段模块：身体接近半径可配置（已修，仅本地验证）

提交 `2aacd22`，tag `rb-0.10.75-body-radius`。

补 3.4 的可用性缺口：全局抓包页原先没有 `body_radius_blocks`，服务器禁止 tp 时用户无法关闭；观察方式选项仍写「身体不跟随」，与新行为矛盾。

| 改动 | 要点 |
|---|---|
| `yyGlobalCaptureConfig.BodyRadius *float64` | **必须用指针**：旧 `global-capture.json` 无此字段，值类型会解码成 0，等于升级后静默关闭身体接近、重新收不到血量包 |
| `yyGlobalBodyRadius(c)` | 缺字段 → 64；显式 0 → 关闭 |
| `yyGlobalWatchOptions` | 透传给 `yyWatchParse`，沿用 0..4096 校验 |
| 界面 | 新增「身体接近半径 / 格 · 0关闭」；观察方式文案改为「身体保持在目标附近」并说明只移动机器人身体 |

验证：`go vet .` 通过；`go test . ./config ./state ./building` 全通过（19.2s / 3.6s / 0.5s / 13.4s）；新增 `TestGlobalCaptureBodyRadiusKeepsDefaultForOldConfigs`；`node --test` 268/268 通过。未构建安装包、未部署、未真机真服验收。

### 3.6 产物级构建探针（不落正式产物）

源码版本号仍是 `0.10.74`（`youyuan_product.go:14`、`android/app/build.gradle:6`），直接构建会产出「0.10.74 文件名 + 0.10.75 行为」的版本撞车产物，故**未写入 `build/`**，只在 `../work/probe/` 验证目标平台可编译：

| 目标 | 结果 |
|---|---|
| `linux/amd64`（CGO 关闭） | 通过，51.6 MB |
| `linux/arm64`（CGO 关闭） | 通过，49.0 MB |
| `windows/amd64`（CGO 关闭） | 通过，52.3 MB |
| `android/arm64` `-buildmode=c-shared`（NDK 26.1.10909125） | 通过，52.6 MB `libyouyuan-arm64.so` |

结论：抓包两模块的改动可构建到全部目标平台；**版本号未提升，尚未生成可发布的 0.10.75 安装包**。

### 3.7 本阶段模块：夜间多智能体推进（2026-10-20 01:46–02:05）

背景：用户要求「今晚统领其余两个智能体完成 100 多项任务」。实际盘点后确认 113 项待办里绝大多数是子系统级功能且需要真机真服，**不可能一夜端到端完成**；因此本轮定位为「逐项核实真实状态 + 修掉能在本地闭环的真缺陷」，验收层级如实标注。

执行方式：1 主控 + 3 个核实智能体并行（verify-observe 完成 29 项；verify-platform 42 项、verify-feature 49 项因 429 频率超限中断，未产出，任务已转 DSH 侧文件交接）。

| 提交 | 修复内容 | 验收层级 |
|---|---|---|
| `51c1ca7` | 抓包突发仍丢包：`yyCaptureStore` 锁内序列化 → 改为锁内取快照、锁外序列化、锁内 O(1) 写入；队列 2048→4096；满队先让出重试再计数丢弃 | 已运行（vet + TestCapture* 3 轮 + 全量） |
| `46bf58a` | 序列化移出锁后 `yyCaptureFlush` 误判已排空（读到 0 条）→ 新增 `inflight` 计数，flush 要求 inflight==0；defer 兜底防 panic 泄漏 | 已运行 |
| `62f75f6` | D10/F14 内部 API 目录堆在普通界面 → `yyModuleAllowed` 统一按管理员过滤，首页重复入口隐藏，设置页保留一处 | 已运行（225 项前端测试） |
| `e37880f` | F13 踢人失败仍记 `Sent` 并静默 30s；I01 容器缓存满 4096 整表清空 → 改为按 timestamp 淘汰最旧一半 + `BlocksEvicted` 计数 | 已运行（新增 2 项单测，其中 1 项先失败暴露 trim 缺下限） |
| `c9a5eb0` | F03 selector 路径 `missed_intervals` 恒为 0 → 改为定速调度并累加 Missed | 已运行 |
| `f619aaa` | G10 断线只记最后一条 EOF → 新增 attempt 与 first_error/first_error_phase，三处写入统一 | 已运行（新增 3 项单测） |

DSH 侧协作通道（新建）：`D:/Users/Lenovo/Documents/deepseek-harness/default-workspace/handoff/`，含 `TASKS.md`、`tasks/DSH-01..03.md`、`results/REF-observe-29.md`。实测结论：**DSH 是 Electron 桌面应用，无 headless CLI，外部无法程序化派活**，只能文件交接 + 用户在 GUI 触发。

未验证层级（不得越级宣称）：全部 6 项均未构建安装包、未部署、未真机真服；race 检测仍不可用（本机无 gcc）。

### 3.8 多角色审核与安全加固（2026-10-20 02:20–03:10）

背景：用户明确要求「派独立 Agent 挑刺」「站在极端 UI 设计者角度审核」「要有安全负责人」「不能空想」。本轮派出两个独立 Agent（非实现者本人），两份报告落在 `outputs/reviews/`。

| 角色 | 交付 | 结论 |
|---|---|---|
| UI 极端审核官 | `outputs/reviews/ui-audit-2026-10-20.md` | 覆盖 18 屏，Chrome 实测 1280/768/390 三档 + 深色；**39 条问题**（阻断 9 / 严重 16 / 一般 10 / 建议 4） |
| 安全负责人 | `outputs/reviews/security-audit-2026-10-20.md` | **高危 6 / 中危 9 / 低危 7 / 已确认安全 13**；另列出 **6 处「看起来修了其实没修」** |

**安全侧根因**：云端中继与 Linux 网关的鉴权是可靠的，但**本机控制台 `/api/*` 零鉴权且 `ACAO:*`**，任意网页可读凭据、执行任意 MC 命令。这是 H1/H2/M1/M5 的共同根因。

本轮已修 12 处（全部落在「已运行」层）：

| # | 问题 | 来源 | 改动 | 验证 |
|---|---|---|---|---|
| 1 | 任意文件写入（可写插件代码→代码执行） | H3 | `preview_handler.go` 限图片扩展名 + 拒绝 `..` + 64MB 上限；`main.go` 上传文件名取 `Base`、`output_dir` 拒 `..` | 新增 4 项单测 |
| 2 | 插件 id 可注入内联事件（DOM XSS + WebView 原生桥读文件） | H4 | `plugin_pack.go` `validPluginID` 拒 `' " \` < > ( )`；`utils.js` 新增 `yyJSStr()`；`plugins.js` 全部 `p.id` 拼接改用它 | 新增 1 项单测 |
| 3 | 观测目标不过滤机器人（红线第 9 条在 API 层未闭环） | H5 | `youyuan_selector_camera.go` `targets()` 追加 `yyCaptureRobotNames()` 二次排除 | 代码核实（reconcile 路径已有覆盖） |
| 4 | 包名常量写成旧包名 `com.prismtool.box`，且权威分支永不校验包名 | M2 | `integrity.go` 改为 `org.youyuan.toolbox`；权威分支补包名校验 | 代码核实（Java 上报真实 `getPackageName()`） |
| 5 | 非 JSON 响应把 `Unexpected token '<'` 当错误文案（6 个屏） | UI B3 | `api.js` 加 content-type 判定与人话兜底，原始文本只进 console | 225 项 Worker 测试 |
| 6 | 深色模式设置卡白底浅字（对比度 1.2:1） | UI B1 | `youyuan_settings_groups.css` `--phx-bg-primary`（全项目未定义）→ `--phx-bg-card` | 渲染实测待复验 |
| 7 | 主按钮白字配浅青底（1.72:1） | UI B2 | 新增 `--phx-on-primary`（深色 `#0d2422`），`.yy-primary`/`.btn`/`.yy-brand-mark` 改用它 | 同上 |
| 8 | 次要文字对比度不足（3.63:1 / 2.39:1） | UI S5 | `--phx-text-secondary` `#7a898d` → `#5f6f73`（≈5.1:1） | 同上 |
| 9 | 观测页「自动刷新」渲染成空白药丸 | UI B6 | `youyuan_sense.js` 裸 checkbox → `yySwitch()` | 语法检查 |
| 10 | 群服互通正文印 `状态：undefined` | UI B5 | `youyuan_qq_diagnostics.js` 加末位兜底文案 | 语法检查 |
| 11 | 插件市场失败后永久停在「加载中…」 | UI B4 | `plugin_market.js` 新增 `error` 态 + 页面内错误块 + 重试 | 语法检查 |
| 12 | 账号页标题被折成两行；768 宽设置页两栏留 500px 空白 | UI B8/S3 | `.yy-form-head h3` 加 `nowrap`、`select` 限宽；设置页断点 760→900 | 同上 |

**修的过程中自己踩的坑（已修）**：`api.js` 第一版兜底在「拿不到 content-type」时也走非 JSON 分支，导致 `navigation-race` 3 项测试失败（mock 响应无该头）。改为「只有服务端明确声明非 JSON 才兜底」后 225/225 通过。这条记录下来，避免以后再用「没有头就判坏」的写法。

**仍未修（明确列出，不得当作已解决）**：

- **H1/H2（本机 API 零鉴权 + 凭据明文下发）**：架构级改动，涉及全部前端 `fetch`、SSE、WebView 首屏注入与 CORS 白名单，改错会让 APK 网页直接不可用。已写成自包含任务书 `handoff/tasks/DSH-04.md`，等 DSH 侧实现或用户授权我在下一轮做。
- **M1（catalog 后端无角色校验）**：已确认是纯前端隐藏、`curl` 可绕过；未改，随 H1 一起做。
- **H6（对第三方服务器玩家自动 kick）**：合规问题，是否上架版本移除需用户裁决。
- **UI 剩余 27 条**：S1/S2（命名与面包屑）、S7/S8/S9（20 种圆角 / 6 套按钮 / 4 种开关）、S10（内部术语外露）、S11（危险操作无确认）、S12（移动端触控区 <44px）、S15（11 个主题里 2 个无效）。
- **跨家族审核缺口**：本轮 UI 审核由 WorkBuddy 内部另起的独立 Agent 完成，仍属 DeepSeek 家族；按协定应由 Codex（OpenAI）或 Antigravity（Google）终审。Codex 额度未恢复，Antigravity 未实测。

### 3.9 三平台派单落地（2026-10-20 10:05–10:15）

**起因**：用户指出「DeepSeek Harness 以及谷歌的反重力那边一点显示都没有，任务列表也没做」。

**核实结果（如实）**：

| 平台 | 派单前状态 | 问题定性 |
|---|---|---|
| WorkBuddy | 无任务列表 | 未调用任务工具，面板空白 |
| DeepSeek Harness | `handoff/tasks/DSH-01~04.md` 存在，`results/` 仅 1 份历史结果 | 文件藏在子目录，GUI 不自动展示；4 个任务零交付 |
| Antigravity | **零文件** | 只在计划里提过，从未落盘，等于没派 |

**本轮落盘**：

| 位置 | 文件 | 作用 |
|---|---|---|
| `D:/.../deepseek-harness/default-workspace/` | `AGENTS.md`（新） | DSH 开工必读：项目路径、开工顺序、硬约束、结果格式、禁止重复项 |
| 同上 | `TASKS.md`（新） | 顶层看板，4 个任务 + 状态图例 + 可复制开工指令 |
| `D:/Users/Lenovo/Documents/antigravity-handoff/` | `AGENTS.md`、`TASKS.md`、`tasks/AG-01~03.md`、`results/`（全新） | Antigravity 独立工作区；AG-03 复核 12 处修复（P0）、AG-01 UI 终审、AG-02 安全终审 |
| `project/outputs/` | `2026-10-20-三平台开工指令.md`（新） | 三平台各自的可复制指令 + 平台能力限制说明 |
| WorkBuddy 任务面板 | #8–#18 共 11 项 | 覆盖看板、UI 阻断、鉴权、91 项核实、构建、部署、云服务器、上架、小黄鱼、GitHub、跨家族终审 |

**平台能力（实测，写入交接包）**：

- DSH：Electron GUI，进程在跑（6 个），**无 headless CLI**，无法程序化派单。
- Antigravity：进程在跑（6 个），安装目录只有 `Antigravity.exe`，`Roaming/Antigravity/bin/` 只有 `agy-node.cmd`，**无 `antigravity` 命令入口**，无法程序化打开工作区。
- 结论：两者只能「写文件 + 用户在 GUI 粘贴指令」，这是硬限制，不是未做。

**仍未解决**：Antigravity 当前工作区为 `outside-of-project`（`app_storage.json` 记录），需用户手动 Open Folder 到 `D:/Users/Lenovo/Documents/antigravity-handoff`。

## 4. 关键决策（已定，不再重复讨论）

1. **不伪造令牌、不入侵第三方后台、不借用他人身份**；只用获授权 API 与资源。此前「入侵网易后台」方向已撤回（P01）。
2. **自动重连 / 创造维护开关保持关闭**（用户明确要求），不得擅自打开。
3. **观测只用摄像机视角**，不默认开旁观/跟随传送；机器人身份必须排除，未知身份不投放。
4. **不把构建、进程存活、网页可达、单场样本当作端到端通过**；验收分层：静态核实 → 可构建 → 已运行 → 端到端通过。
5. **总账只增量更新**，不重跑历史 `update/record_acceptance` 脚本（会写回旧版本状态）。
6. **旧 SSH 临时桥（`ssh_https58.py` / `yy_deploy58_tmp`）已删除，禁止调用**；下次发布自建明确命名的临时桥，保留原主机钉住与认证。
7. **GitHub 备份使用现有仓库 `yuanjunqiggg-tech/youyuan-`（用户 2026-10-19 决定，公开仓库）+ 每模块提交 + 阶段 tag**；源码入库，构建产物（`build/`、`cloudflare/public/`、`private/`）不入库。
8. **测试用自签私钥 `linux/test_direct_tunnel_fixtures/localhost-key.pem` 入库**（用户已确认，仅本地回环测试用途）。
9. **PROJECT_STATE.md 为唯一状态源**，聊天记忆不作数。

## 5. 文件结构

```
project/
├─ PROJECT_STATE.md          # 本文件：唯一项目状态
├─ AGENTS.md                 # 工作入口与红线
├─ 项目待办.md               # 活动清单（由总账渲染）
├─ 需求逐项总清单.json       # 120 项唯一总账
├─ 已完成与撤回归档.md       # 关闭项归档
├─ 协作交接/维护入口.md      # 交接细节、发布流程、代码地图
├─ upstream/                 # 原生宿主 Go 代码 + 内嵌前端
│  ├─ frontend/js|css|pages # 网页控制台（独立网页与原生嵌入共用）
│  ├─ flowers-for-machines/ # 协议与包处理底层
│  ├─ android/              # APK 壳工程
│  └─ skills/               # 插件开发文档与包参考
├─ auth-service/            # 独立游戏认证服务（Worker + Go/WASM）
├─ cloudflare/              # 授权中继 Worker 与测试
├─ maintenance/             # 总账渲染/归档脚本与测试
├─ linux/                   # 直连网关与隧道
├─ observer-bridge/         # 观测桥
├─ editor/                  # 内置编辑器
├─ samples/                 # 示例插件
├─ third_party/             # 第三方依赖
├─ outputs/                 # 验收证据（json/md/png）
└─ private/                 # 私有配置（git 忽略）
../work/                    # 构建、发布、临时脚本与日志（git 忽略）
../outputs/                 # 交付产物与预览图
```

## 6. 常用命令

```bash
# 工作目录
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"

# Go 离线构建/测试（避免联网拉依赖）
export GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false
export GOCACHE=../work/go-cache GOTMPDIR=../work/tmp
"C:/Program Files/Go/bin/go.exe" test ./... -count=1 -timeout 120s

# 前端与 Worker 测试
node --test cloudflare/*.test.mjs
node --test auth-service/*.test.mjs

# 总账校验与渲染（只增量，不重置）
python -m unittest discover -s maintenance -p test_requirements.py
python maintenance/requirements.py --project . --render

# 发布（每次先读脚本，历史脚本不可照搬）
../work/rebuild_v01074.ps1   # 构建
../work/stage_web_01074.py   # 暂存网页
../work/upload74_final.py    # 上传
../work/activate74_final.py  # 激活
../work/verify74_remote.py   # 远端核验
```

- Python 环境：`D:/YouYuan-Codex/.venv/Scripts/python.exe -u -X utf8`
- 网络：GitHub 需走本地代理 `http://127.0.0.1:9674`，直连会被重置。

## 7. 回滚点

| 回滚点 | 类型 | 内容 | 说明 |
|---|---|---|---|
| `70a277a` | 提交 | 2026-10-07 历史提交 | git 中最后一个真实提交，早于 0.10.8–0.10.74 |
| `rb-0.10.74-online` | tag | 接手基线（0.10.74 线上 + 0.10.75 未验收改动） | 本文件首次提交时创建 |
| `3a31201` / `rb-0.10.75-capture-fix` | 提交 + tag | 抓包四个缺陷修复（队列异步化、身体接近、末地授权、观察者重试） | 仅本地验证层，未部署 |
| `2aacd22` / `rb-0.10.75-body-radius` | 提交 + tag | 身体接近半径可配置 + 界面文案纠正 | 仅本地验证层 |
| `f619aaa` | 提交 | 夜间多智能体推进：抓包丢包/flush 误判/踢人计数/容器缓存/selector 定速/断线诊断/API 目录收敛（共 6 提交） | 上一阶段 HEAD |
| `rb-0.10.75-security-ui` | 提交 + tag | 多角色审核（UI 39 条 / 安全 22 条）+ 12 处修复：任意文件写入、插件 id 注入、观测排除机器人、包名常量、非 JSON 兜底、深色对比度、空白开关、undefined、市场失败态、标题折行 | 当前 HEAD，仅本地验证层 |
| `rb-0.10.75-stage` | tag | 0.10.75 构建部署并验收后 | 待创建 |

回滚方式：

```bash
git checkout rb-0.10.74-online -- .               # 回到接手基线源码
git checkout rb-0.10.75-capture-fix -- upstream/  # 只回滚抓包异步化
git checkout rb-0.10.75-body-radius -- upstream/  # 回滚到当前 HEAD 的 upstream
git checkout rb-0.10.75-stage                     # 整树切换（待创建）
```

构建产物不入库：回滚源码后需用对应版本的 `work/rebuild_v0107x.ps1` 重新构建。

## 8. 下一步（按顺序）

> **2026-10-20 10:12 多平台派单修正**：此前任务只写在 `handoff/` 子目录，DSH 与 Antigravity 的 GUI 均看不到，等于没派单；Antigravity 侧**从未建过任何文件**。现已补齐：
> - DSH 工作区根新增 `AGENTS.md` + `TASKS.md`（顶层入口，不再依赖翻子目录）
> - 新建 `D:/Users/Lenovo/Documents/antigravity-handoff/`：`AGENTS.md` + `TASKS.md` + `tasks/AG-01~03.md`
> - 三平台可复制的开工指令：`outputs/2026-10-20-三平台开工指令.md`
> - WorkBuddy 任务面板建 11 项任务（#8–#18）
>
> **平台限制（如实记录）**：DSH 为 Electron GUI 无 headless CLI；Antigravity 仅有 `agy-node.cmd`、无 `antigravity` 命令入口。两者均无法程序化调用，只能「写文件 + 用户在 GUI 粘贴指令」。这填上了交接包 `api_or_cli_available` 的关键未知项。
>
> **2026-10-20 03:10 追加的优先级调整**：安全审查给出的 H1/H2（本机 `/api/*` 零鉴权 + 凭据明文下发）是**上架阻断项**，优先级高于下面原有 0–7。任务书已写成自包含的 `handoff/tasks/DSH-04.md`（含 5 步实现与验收方法）；若 DSH 侧 24 小时内无产出，由主控下一轮直接实现。
>
> **待用户确认（不得自行补脑）**：「小黄鱼」在项目文档 grep 与两次历史对话检索中**均为 0 结果**。需要用户说明：它指什么平台（闲鱼店铺？商品？）、要更新哪些字段（版本号/更新说明/截图/下载链接）、入口在哪。确认后才能写进第 5 节发布流程。

0. **当前阻塞**：GitHub 推送缺少凭据。远端 `origin` 已配好为 `https://github.com/yuanjunqiggg-tech/youyuan-.git`，`ls-remote` 匿名可读且仓库为空（首次推送不会冲突），但 `git push` 返回 `remote: No anonymous write access.`。GitHub 连接器为应用级令牌，无 `contents:write` 与建仓权限（403）；本机无 `gh`、无 `~/.git-credentials`、无 SSH 密钥、Windows 凭据管理器无 GitHub 条目。需用户提供 PAT（`Contents: Read and write`，目标仓库 `youyuan-`）。本地已就绪 3 个提交（`3a31201`、`2aacd22`、`45a2a92`）+ 2 个 tag，凭据到位后一条命令即可推送。
1. **待用户裁决 2 项**：`cmd/server` 结构性编译失败（可加 `//go:build ignore` 一行修掉，或删除该遗留目录）；`internal/playerrelay` UDP 回环偶发超时（是否加长等待/重试）。
2. **建立 GitHub 备份**（本文件 + 全部源码首次推送），推送 `rb-0.10.74-online` 与 `rb-0.10.75-capture-fix` tag。
3. **真机复验抓包四缺陷**：开启全局抓包看 `queue_dropped` 是否长期为 0 且不再断连；进地狱打生物确认血量与属性包；确认三个机器人全部进入末地值守；多人同服观察丢包。
4. **收尾 0.10.75**：构建 → 部署网页/Linux/APK → 核对线上版本与哈希 → 真机与真服验收（含房间识别三类候选、终端、插件启停删、编辑器）。**构建与三平台部署已于 2026-10-20 14:45 完成（见第 9 节），只剩真机验收。**
5. **补第 8 条**：去掉「采集账号需OP旁观」表述（文案已改，需界面核对），确认纯视角观察不切换旁观模式，收敛普通界面的抓包原理描述（F02/F14）。
6. 更新总账对应编号与 `项目待办.md`，写阶段验收报告到 `outputs/`。
7. 每完成一项即更新本文件并提交 GitHub（提交信息带 `rollback:` 标记）。

## 9. 变更记录

| 时间 | 变更 | 回滚点 |
|---|---|---|
| 2026-10-19 23:35 | 创建本文件；确立唯一状态源与提交规则 | `rb-0.10.74-online` |
| 2026-10-19 23:50 | 备份仓库改为现有 `youyuan-`；记录推送凭据阻塞 | `rb-0.10.74-online` |
| 2026-10-19 23:58 | 修复抓包四缺陷（异步队列、身体接近、末地授权、观察者重试）；补 5 项回归；记录 2 项既有测试失败 | `3a31201` / `rb-0.10.75-capture-fix` |
| 2026-10-20 00:10 | 身体接近半径可配置（指针字段防旧配置静默失效）；界面文案纠正；补 1 项迁移回归；268 项 web 测试复跑通过 | `2aacd22` / `rb-0.10.75-body-radius` |
| 2026-10-20 01:02 | 产物级构建探针：linux/amd64、linux/arm64、windows/amd64、android/arm64 c-shared 全部通过（写入 `work/probe/`，未覆盖 `build/`）；GitHub 远端已配好但推送被凭据阻塞（`No anonymous write access`） | `rb-0.10.75-body-radius` |
| 2026-10-20 01:20 | 复核发现小数半径会被 `%.0f` 渲染成 `distance=0..`（每 tick 传送），改为四舍五入下限 1；补断言 | `0246f5d` / `rb-0.10.75-body-radius` |
| 2026-10-20 01:45 | 校正任务清单口径：总账实为 120 项（78/35/5/1/1），本节原写 119 项（83/30/4/1/1）已落后；按「总账为唯一计数来源」同步，并提交 4 个未提交的总账文件 | `rb-0.10.75-body-radius` |
| 2026-10-20 02:05 | 夜间多智能体推进：29 项核实完成（14 已实现/13 部分/2 未实现），修 6 处可本地闭环缺陷（抓包丢包、flush 误判、踢人计数、容器缓存、selector 定速、断线首次原因）+ 1 处 UI 收敛；新建 DSH 文件交接通道；verify-platform/verify-feature 因 429 中断，任务转 DSH | `f619aaa` |
| 2026-10-20 03:10 | 多角色审核落地：UI 极端审核 39 条（阻断 9）、安全审查高危 6 条并揪出 6 处假修复；本轮修 12 处（H3/H4/H5/M2 + 7 处 UI 阻断/严重），新增 6 项安全单测；Go 全量 + 225 项 Worker + 43 项 auth 全通过；H1/H2 架构级鉴权写成 DSH-04 任务书 | `rb-0.10.75-security-ui` |
| 2026-10-20 10:15 | 三平台派单落地：DSH 工作区根新增 `AGENTS.md`+`TASKS.md`；新建 `D:/Users/Lenovo/Documents/antigravity-handoff/`（AGENTS/TASKS/AG-01~03/AG-03 复核 12 处修复）；新增 `outputs/2026-10-20-三平台开工指令.md`；WorkBuddy 任务面板建 11 项；实测两平台均无 CLI 入口（只能 GUI 粘贴指令） | `1e2bc07` / `rb-0.10.75-cloud-version` |
| 2026-10-20 14:45 | **0.10.75 三平台发布完成**：①Cloudflare `youyuan-apk-v030` 部署（Version `e42572e2…8536`），在线清单 0.10.75/1075、10 分片在线下载拼接 SHA-256 `8841ef82…` 与本地 APK 一致、证书未变；②`youyuan-game-auth` 部署（Version `41d78964…1847`，auth 独立版本仍 0.10.70，43+4 测试先过）；③Linux 47.102.156.125 直连 SSH 更新到二进制 `948650bd…93511`，服务 active、bot connected/is_op、中继 `/health` 报 `0.10.75`；本地时间戳备份 `20261020-140806_0.10.75-linux-binary替换前`，服务端备份 `/var/backups/youyuan/youyuan-before-01075`；首次 wrangler 上传中断未污染线上（失败后在线仍为 0.10.74） | `1e87529` / `1e2bc07` / `rb-0.10.75-cloud-version` |
| 2026-10-20 19:30 | **0.10.76 两个 P0 修复（本地验证层，未构建未部署）**：①抓包「其余机器人不上线」实锤为**结构性死锁、非封号**——`youyuan_capture_runtime.go` `yyCaptureRobotNamesWithStatus()` 对 `Slot>=1` 的托管账号逐个 `bot.self`，任一失败即 `identitiesKnown=false`，`youyuan_capture_reconcile.go:119` 据此强制 `short=0` 永久暂停增派；线上 `managed-accounts.json` 里 `pool_5b2e`(slot=1) 与 `account_1792`(slot=2) **停用但 slot 未清零**，slot 指向已死子内核 → 永久毒化名单。修法：先用 `GET /api/bot/status` 判子内核在跑且 connected，才要求它报身份；离线/未启动账号只记录进 `identity_skipped_offline_accounts` 不毒化；`yyAccountsRoutes` 加载时自愈清空停用账号残留 slot；reconcile summary 补 `identity_unverified_online_accounts`。新增 `youyuan_capture_identity_test.go` 4 项。②注册卡在「正在注册」= 客户端无超时：`youyuan_identity.js` `yyIdentityRequest` 用裸 `window.fetch`，而服务端 `cloudflare/email-verification.mjs:51` 向 Linux 中继 `relay.fetch()` 发验证码邮件也无超时，中继一挂界面永久转圈且按钮禁用。已加 80s AbortController + 可操作中文提示（说明任务不重复创建、可改试登录）。新增 `cloudflare/identity-request-timeout.test.mjs` 3 项。验证：`go vet .` + `go test . ./config ./state ./building` 全通过；`node --test cloudflare auth-service` **271/271**。③新建 `BOOT.md`（会话压缩/换 AI 接手时唯一入口，根治「每次重读文档」） | 待提交 |
| 2026-10-20 19:20 | **未解决清单落地 + 两处 UI/输入修复**：①新建 `未解决清单.md`（361 行）——合并总账 120 项 / `项目待办.md` / `当前未完成清单.md` / `agent-hub/REQUIREMENTS_BACKLOG.md`，按四层验证等级（A 本地闭环 / B 已构建 / C 已部署 / D 真机真服）重排；表尾 114 项全量表格由 `maintenance/gen_unresolved.py` 从总账生成，改总账后重跑即可。②「运行与维护」5 按钮平铺 → 三分组（运行 / 数据留存 / 危险操作红框区）+ 状态行读真实 `running`/`disabled` 字段，`youyuan_plugin_studio.js` 新增 `runStateLine`/`maintenanceGroups`。③全局抓包切换间隔 `min="0"` 允许提交 0 而 `yyPatrolDuration` 拒绝 `<=0`（用户只看到「间隔需要正有限秒数」）→ 改 `min="1"` + 保存 `Math.max(1,…)`，1ms 仍合法。验证：`go vet` + `go test . ./config ./state ./building` 四包通过；`node --test cloudflare auth-service` **278/278**（新增 6+1 项）。**仍未构建、未部署，线上 0.10.75** | 待提交 |EOF

## 2026-10-20 20:10 — 0.10.76 发布（UI + APK + 认证收敛）

- **云端已上线**：前端 8 文件（KPI 面板、运行维护三分组、设置修复）+ 0.10.76 分片 10 片 + latest.json；Worker Version bf5a2b92。线上验收：latest.json=0.10.76/1076，latest.apk Range 206 且全量 sha256=f9329c3d… 一致。
- **APK 已发布**：build/youyuan-v0.10.76.apk（83,297,109 B，证书 SHA-256 cd823622… 与历史一致，可覆盖安装）。versionName 0.10.76 / versionCode 1076。
- **三平台产物**：youyuan-windows.exe、youyuan-linux-x86_64（20:00 构建）、youyuan-v0.10.76.apk。Linux 云服务器二进制已就绪，等待服务器侧替换重启。
- **修复**：A-03（market_url 硬写已删）、A-05（auth_url 三处收敛到 auth.youyuanqi.dpdns.org，game_auth 改读配置）；A-02/A-04/A-06 复核为无需改（清单已更新）。
- **构建链修复**：build.ps1 加 UTF-8 BOM（PS5.1 中文解析）；注意 module 名是 bot-apk，import 配置包必须写 "bot-apk/config"。
- **新工具**：maintenance/publish_apk.py（APK→8MiB 分片+latest.json，签名指纹沿用上一版）。
- 测试：node 278/278；go test ./config ./state 通过；发布后提交回滚点。
