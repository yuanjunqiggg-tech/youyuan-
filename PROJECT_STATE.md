# PROJECT_STATE.md — 游园工具箱唯一项目状态

> 本文件是项目状态的**唯一权威来源**。每次上下文压缩、换会话、换 AI 接手后，**先读本文件再动手**，不得凭聊天记忆行事。
> 与本文件冲突的聊天内容一律作废；发现记录缺失或互相矛盾时，**先停下向用户确认**，不得自行补脑。
> 更新规则：每完成一个模块或一个阶段 → 更新本文件 → 提交 GitHub，提交信息写明回滚点。

- 最后更新：2026-10-20 00:10（北京时间）
- 当前阶段：**0.10.75 收尾**（源码已改、未构建、未部署、未端到端验收）
- 线上基线：**0.10.74 / versionCode 1074**
- 整体完成：**否**（总账 `all_complete=false`）
- 最近回滚点：**`2aacd22` / tag `rb-0.10.75-body-radius`**（抓包四个缺陷已修 + 身体半径可配置，仅本地验证）

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

总账文件：`需求逐项总清单.json`（119 项，唯一计数来源），活动清单由 `项目待办.md` 呈现。

| 状态 | 数量 |
|---|---|
| 未完成 | 83 |
| 部分完成／待验收 | 30 |
| 已完成（验收归档） | 4 |
| 已修复（历史验收） | 1 |
| 已撤回／不执行 | 1 |
| **合计** | **119** |

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
├─ 需求逐项总清单.json       # 119 项唯一总账
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
| `2aacd22` / `rb-0.10.75-body-radius` | 提交 + tag | 身体接近半径可配置 + 界面文案纠正 | 当前 HEAD，仅本地验证层 |
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

0. **当前阻塞**：GitHub 推送缺少凭据。GitHub 连接器为应用级令牌，无 `contents:write` 与建仓权限（403）；本机无 `gh`、无 `~/.git-credentials`、无 SSH 密钥、Windows 凭据管理器无 GitHub 条目。需用户提供 PAT（`Contents: Read and write`，目标仓库 `youyuan-`）后执行首次推送。本地提交已就绪（`3a31201`）。
1. **待用户裁决 2 项**：`cmd/server` 结构性编译失败（可加 `//go:build ignore` 一行修掉，或删除该遗留目录）；`internal/playerrelay` UDP 回环偶发超时（是否加长等待/重试）。
2. **建立 GitHub 备份**（本文件 + 全部源码首次推送），推送 `rb-0.10.74-online` 与 `rb-0.10.75-capture-fix` tag。
3. **真机复验抓包四缺陷**：开启全局抓包看 `queue_dropped` 是否长期为 0 且不再断连；进地狱打生物确认血量与属性包；确认三个机器人全部进入末地值守；多人同服观察丢包。
4. **收尾 0.10.75**：构建 → 部署网页/Linux/APK → 核对线上版本与哈希 → 真机与真服验收（含房间识别三类候选、终端、插件启停删、编辑器）。
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
