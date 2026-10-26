# 游园工具箱（MCORT）统一工作区索引

> 本目录 `project/workspace/` 是**纯文档目录**：不参与 `go:embed`、不进入 Worker 静态资源、不被 `build.ps1` / `build_frontend.py` 读取。
> 目的：把散落在 639 个 Go 文件、111 个前端 JS、三套种子页面、两套部署链路里的东西收成一张能查的地图。
> 维护规则：**只在本目录写文档，不移动、不重命名、不删除任何既有文件**；源码事实一律用 `文件:行` 指回原处。

生成时间：2026-10-26　　项目根：`C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project`

---

## 0. 三句话看懂这个项目

1. **一个 Go 宿主**（`upstream/`，639 个 `.go`）同时编出 Android `.so`、Windows `.exe`、Linux ELF；前端（111 个 JS：`js/` 75 + `features/` 28 + `pages/` 5 + `vendor/` 3）通过 `//go:embed frontend/**` 编进二进制（`upstream/main.go:52`）。
2. **三个线上入口**：控制台 `toolbox.youyuanqi.dpdns.org`、认证 `auth.youyuanqi.dpdns.org`、更新 `updates.youyuanqi.dpdns.org`。前两者是两个独立 Worker（`youyuan-apk-v030`、`youyuan-game-auth`）。
3. **一条 Linux 生产链路**：`47.102.156.125` → systemd `youyuan.service` → `/opt/youyuan/youyuan` → 端口 **19429** → 数据目录 `/var/lib/youyuan`。

---

## 1. 目录说明：什么是什么

| 路径 | 性质 | 说明 | 能不能手改 |
|---|---|---|---|
| `upstream/` | **源码** | Go 宿主 + 内嵌前端 + Android 壳工程 + 30 个技能文档 | 能，改完要构建 |
| `upstream/frontend/` | **源码（唯一可读前端）** | `index.html`、`js/`、`css/`、`assets/`、`blocks/` | 能 |
| `upstream/plugins/` | 空/预留 | 插件源码不放这里；运行时插件在数据目录的 `插件文件/` | — |
| `upstream/skills/` | **源码（文档）** | 30 个内置技能目录，被 `ai_skills.go:14` `//go:embed skills` 编进二进制 | 能 |
| `upstream/android/` | **源码** | APK 壳、`build.gradle`、`jniLibs` | 能 |
| `cloudflare/` | **源码 + 构建产物** | `*.mjs` 是 Worker 源码；`public/` 是 `build_frontend.py` 生成的部署快照 | `public/` **不要手改** |
| `auth-service/` | **源码 + 构建产物** | `worker.mjs`、`engine.go`、`public/`；`kernel.wasm`、`wasm_exec.js` 是 `build.ps1` 产物 | wasm 不要手改 |
| `maintenance/` | **脚本** | 构建前端、部署 Linux、发布 APK、总账渲染、诊断探针 | 能，但先读脚本头注释 |
| `linux/` | **部署脚本** | systemd 单元、`start.sh`、`install-bundle.sh`、直连网关 | 能 |
| `build/` | **构建产物** | `youyuan-windows.exe`、`youyuan-linux-x86_64`、`youyuan-v*.apk`、`cloudflared*` | 不要手改 |
| `outputs/` | **证据** | 验收报告、截图、探针 JSON、审查报告 | 只增不改历史 |
| `docs/design/` | 设计文档 | 机器人能力/健康/面板/人格 4 篇 | 能 |
| `preview/` | 预览页 | `live_console.html`、`qq-console.html` 等本地预览 | 能 |
| `private/` | 私有 | 浏览器 profile、Turnstile secret；**已被 git 忽略** | 不要把值写进任何文档 |
| `新建文件4/` | **交接快照** | 2026-10-21 事故期的 10 篇交接（部署位置、根因、未完成清单） | 只读参考 |
| `incident-fixes/` | 事故补丁 | 按日期分的补丁与说明 | 只读参考 |
| `协作交接/维护入口.md` | **当前交接总入口** | 顶部是按时间倒序的最新状态，越靠前越新 | 只读，别覆盖 |
| `workspace/` | **本目录** | 统一索引（你正在看的） | 能，只增文档 |
| `../work/`（项目外） | 临时/发布脚本 | 历史上传、暂存、验证脚本 | 能；**历史脚本不可照搬** |
| `../agent-hub/`（项目外） | 多智能体中枢 | `TASKS.md`、`AUTHORIZATION.md`、`TEST_RESULTS.md` | 只读 |

### 构建产物黑名单（不要当源码改）

`build/*`、`cloudflare/public/**`、`auth-service/kernel.wasm`、`auth-service/wasm_exec.js`、`upstream/android/app/build/**`、`upstream/frontend/js/*.esb`（31 个历史压缩副本，`index.html` 一个都没引用）。

### 1.1 关于「把所有东西统一到一个目录」

本轮**没有物理搬动任何文件**，原因有三：

1. 项目里有 `//go:embed frontend/**`、`assets.directory = public`、`wrangler.jsonc` 的相对路径、systemd 的 `/opt/youyuan`、脚本里写死的绝对路径（`build_frontend.py:35-36`、`deploy_linux_bin.py:18-21`）。搬目录不是「整理」，是一次跨 4 条链路的路径重构。
2. 当前工作树有多个并行 Agent 的未提交改动（`git status`），搬动会直接冲掉别人的改动，且 `git mv` 后历史定位困难。
3. 用户当前最痛的是「找不到、看不懂、部署靠记忆」，不是「文件不在同一个文件夹」。先给索引，收益立刻可见；物理搬迁应单独立项、先备份、再改引用。

如果之后要做物理统一，建议目标形态（**未执行，需授权**）：

```
project/
├─ src/            # upstream/（Go + frontend + android + skills）
├─ services/       # cloudflare/、auth-service/
├─ ops/            # linux/、maintenance/、build 脚本
├─ docs/           # 现有 md + workspace/
├─ artifacts/      # build/、cloudflare/public/（gitignore）
└─ records/        # outputs/、新建文件4/、协作交接/
```

前置条件：先把 `go:embed`、`wrangler.jsonc`、`build.ps1`、`deploy_linux_bin.py`、`build_frontend.py` 里的路径集中到一个配置文件，再动目录；否则必然出现「网页好了、APK 白屏」「Linux 起来了、数据目录空了」。

---

## 2. 全部项目清单

> 状态口径按四层：**本地通过** → **已构建** → **已部署** → **真机/真服验收**。同一个人说「好了」不算，要有证据文件。
> 版本提醒：源码常量已是 `0.10.78 / 1078`（`upstream/youyuan_product.go:14`、`upstream/android/app/build.gradle:6`），但 `PROJECT_STATE.md` 头部仍停在 0.10.75，**已落后**。当前状态请以 `协作交接/维护入口.md` 顶部 + `需求逐项总清单.json` 为准。

### 2.1 主干

| 项目 | 位置 | 入口文件 | 部署方式 | 当前状态 |
|---|---|---|---|---|
| **游园工具箱 Go 内核** | `upstream/` | `upstream/main.go`、`main_native.go` | `build.ps1` 三平台构建 | 源码 0.10.78；线上据巡检为 0.10.78；工作树有大量并行未提交改动（HEAD `3a11d92`），**不要整树提交** |
| **网页控制台（前端）** | `upstream/frontend/` | `index.html`、`js/app.js` | `maintenance/build_frontend.py` → `cloudflare/public/` → `wrangler deploy`；APK/Linux 需重新 `go build` 才更新内嵌副本 | 可构建；`cloudflare/public/updates/android/latest.json` 仍是 0.10.77，快照**已过期** |
| **APK** | `upstream/android/` | `android/app/build.gradle` | `build.ps1` → `maintenance/publish_apk.py` → `wrangler deploy` | 本地更新清单为 0.10.77/1077；证书 `cd823622…` 不可变 |
| **box / Linux 服务端** | `linux/` + `build/youyuan-linux-x86_64` | `linux/youyuan.service` | `GOOS=linux go build` → `maintenance/deploy_linux_bin.py` | 线上 47.102.156.125 端口 19429；自动更新定时器是已知风险（§5） |
| **独立认证站** | `auth-service/` | `worker.mjs` + `engine.go` | `auth-service/build.ps1`（wasm）→ `wrangler deploy`（`youyuan-game-auth`） | 已上线 `auth.youyuanqi.dpdns.org`；独立版本 0.10.70，热修 `link-tcp-20261019.4` |
| **Cloudflare Worker（中继/主站）** | `cloudflare/` | `worker.mjs`、`wrangler.jsonc` | `wrangler deploy`（脚本名 `youyuan-apk-v030`） | 已上线；含 RELAY DO、OAuth/MCP 发现、更新分发 |

### 2.2 种子查询（当前要修的）

| 项目 | 位置 | 入口 | 部署 | 状态 |
|---|---|---|---|---|
| **种子查询 v2（服务端）** | `upstream/seedv2_*.go`（14）+ `youyuan_seedv2_*.go` | `youyuan_seedv2_routes.go:70` | 随宿主二进制/网页 | 内核已接好：结构逐点对拍、群系 99.7%（`outputs/review-seed/报告.md`）；**外壳布局待改**，见 `seed-query/review.md` |
| **种子地图 v2（前端）** | `upstream/frontend/js/youyuan_seedmap_v2.js`（2403 行）+ `css/youyuan_seedmap_v2.css`（833 行） | `yySeedMapPage()`（js:2325） | `build_frontend.py` / `go build` | 可用但密度与层级问题多；与另两套并存 |
| **种子查询 v1（旧）** | `upstream/youyuan_seed_api.go` + `js/youyuan_seed_page.js`（1111 行） | `/api/seed/*` | 同上 | 保留兼容，不再演进（路由注释已写死） |
| **种子查询器（Canvas 版）** | `js/features/seed_map.js` + `css/yy_seed_map.css` | 「更多工具 → 建筑与世界」 | 同上 | 第三套入口；`/api/seed/*` 降级实现，建议收敛 |
| API 速查 | `workspace/seed-query/api.md` | — | — | 本轮新增 |

### 2.3 子系统

| 项目 | 位置 | 入口 | 部署 | 状态 |
|---|---|---|---|---|
| **插件系统（Lua/Python/C++）** | `upstream/plugin_*.go`、`lua_*`、`samples/`、`upstream/skills/*-runtime` | `plugin_controller.go`、`plugin_scan.go` | 随宿主；插件放数据目录 `插件文件/<id>/` | 本地通过；真机长跑未全验 |
| **插件市场** | `upstream/youyuan_market.go` + `js/features/plugin_market.js`、`js/youyuan_market.js` | `/api/market/plugins` | 随宿主/网页 | 有失败态修复；真实市场下载需凭据 |
| **MCP** | `upstream/youyuan_mcp_*.go`、`youyuan_agent_mcp_catalog.go`、`cloudflare/mcp-*.mjs` | `/mcp`（Bearer/专用链接） | Worker + 宿主 | 可用；只读白名单在 `youyuan_mcp_links.go:27` |
| **QQ 群服互通** | `upstream/youyuan_qq*.go`（93）+ `js/youyuan_qq*.js`（12） | `youyuan_qq.go`、`/qq/webhook` | Worker 回调 + 宿主 | 双向群消息未完成真群回执验收 |
| **抓包 / 观察者** | `upstream/youyuan_capture*.go`（17）、`youyuan_observer_*.go`、`observer-bridge/` | `youyuan_capture.go`、`/api/youyuan/capture/*` | 随宿主 | 异步队列/身体接近已修；多人峰值未验 |
| **建筑导入导出 / NBT** | `upstream/youyuan_import*.go`、`export_center.go`、`building/`、`nbt_world_api.go` | `/api/youyuan/import/*`、`/api/youyuan/exports/*` | 随宿主 | 有真服导入样本；大范围/断点未全验 |
| **地图画 / 音乐 / 绘图** | `youyuan_map_video.go`、`music_player.go`、`draw_handlers.go` + `features/mapart.js`、`music.js`、`drawing.js` | 首页工具区 | 随宿主 | 可用；动画播放待真机验 |
| **命令链** | `youyuan_chain*.go`、`youyuan_commands.go` | `/api/youyuan/chains` | 随宿主；TXT 在 `插件数据文件/ds_ai_agent/command_txt/` | 事故后已多次修复；原雪球主链停用状态需保持，除非用户明确要求 |
| **AI 工作区 / Agent** | `youyuan_agent*.go`、`youyuan_ai_*.go` + `js/youyuan_agent_*.js` | `/ai/*` | Worker 代理 + 宿主 | 可用；外部 LLM 长任务/手机键盘未验 |
| **账号 / 皮肤** | `youyuan_account*.go`、`youyuan_skin*.go` | `/api/youyuan/accounts/*` | 随宿主 | 云端/设备同 UID 合并已上线；真实头像缓存可用 |
| **内置编辑器** | `editor/` + `js/vendor/youyuan_editor.js` | `/editor/*` | Worker 静态 + 宿主 | 可用；文件顺序有测试 |
| **终端** | `youyuan_terminal.go` + `js/youyuan_terminal.js` | `/api/youyuan/terminal/*` | 随宿主 | 可用 |
| **聊天插件（QQ 机器人侧）** | `chat-plugin/youyuan-toolbox/` | 插件入口 | 随宿主/外部宿主 | 参考实现，非主线 |
| **观测桥（第三方客户端）** | `observer-bridge/` | `agent.py`、`README.md` | 单独进程 | 实验性，未纳入主发布 |

---

## 3. 线上事实速查

| 项 | 值 | 备注 |
|---|---|---|
| 控制台 | `https://toolbox.youyuanqi.dpdns.org` | Worker `youyuan-apk-v030` |
| 认证 | `https://auth.youyuanqi.dpdns.org` | Worker `youyuan-game-auth` |
| 更新 | `https://updates.youyuanqi.dpdns.org` | 同一 Worker 的另一条自定义域路由 |
| Linux 生产机 | `47.102.156.125:22`（root） | 凭据 `D:/YouYuan-APK/private/linux-access.json` |
| 服务/端口 | `youyuan.service` / **19429** | 旧文档里的 19439 是笔误 |
| 数据目录 | `/var/lib/youyuan` | systemd 里 `YY_DATA_DIR` 显式指定 |
| 二进制 | `/opt/youyuan/youyuan` | 回滚副本 `/var/backups/youyuan/*` |
| Cloudflare 账号 | `7086578ad87b302bbc65bf198f21a75c` | 见两个 `wrangler.jsonc` |
| wrangler | 4.149.0（npx 缓存） | 凭据 `AppData/Roaming/xdg.config/.wrangler/config/default.toml` |
| Node | `C:/Users/Lenovo/.workbuddy-ai/binaries/node/versions/22.22.2-6/node.exe` | `build_frontend.py:20` 写死 |
| Go | `C:/Program Files/Go/bin/go.exe`（1.27） | — |
| Python（带 httpx/paramiko） | `D:/YouYuan-Codex/.venv/Scripts/python.exe` | 系统 python 不够用 |
| 本地代理 | `http://127.0.0.1:9674` | GitHub/Cloudflare 直连会被重置 |

---

## 4. 文档怎么读（省时间版）

```
要部署            → workspace/deploy-cloud.md
要改前端/插件/链  → workspace/module-update.md
要找 skill/会话   → workspace/skills.md、workspace/conversations.md
要修种子查询      → workspace/seed-query/review.md（外壳）、seed-query/api.md（接口）
要知道现在做到哪  → 协作交接/维护入口.md 顶部 → 需求逐项总清单.md
要接手但只有 5 分钟 → BOOT.md → 本文件 §0/§3
```

`PROJECT_STATE.md` 仍可读架构与决策（§1/§4 很值钱），但**它的版本号与任务计数已过期**，不要拿它当现状。

---

## 5. 当前已知风险（部署前必看）

1. **`cloudflare/public/` 是过期快照**：本地 `latest.json` 写 0.10.77，源码 0.10.78。每次部署必须先跑 `build_frontend.py`，再核对线上 `latest.json`。
2. **Linux 自动更新定时器**：`youyuan-update.timer` 每 6h 拉更新，但没有 Linux 专用 manifest（`新建文件4/02-部署位置.md:17`）。在没确认它拉什么之前，**不要把它当无害**；可用 `systemctl list-timers` / `systemctl cat youyuan-update.timer` 先看。
3. **三套种子页面并存**：`种子查询器` / `种子地图`（v2）/ `种子查询`，两套坐标语义（区块 vs 方块）。这是「结构显示像一滩」的产品层根因之一，见 `seed-query/review.md`。
4. **`.esb` 是历史压缩副本**：31 个，没有任何页面引用；改源码时别改它们，也别以为改了会生效。
5. **并行 Agent 正在改同一棵树**：`git status` 里的 dirty 不是垃圾。禁止 `git checkout -- .`、`git reset --hard`、整树提交。
6. **凭据红线**：不把 `D:/YouYuan-APK/private/` 的值写进文档、日志、提交；`wrangler` token 同理。

---

## 6. 本目录文件清单

| 文件 | 内容 |
|---|---|
| `README.md` | 本索引：目录性质、项目清单、线上事实、风险 |
| `deploy-cloud.md` | 三条部署链路：Worker 前端、认证站、box Linux；命令/验证点/坑 |
| `module-update.md` | 前端模块、插件、链 TXT、MCP/catalog 的更新步骤 |
| `skills.md` | 本地 skill 路径与用途、会话记录位置 |
| `conversations.md` | 历史对话检索方法 + 关键决策纪要 |
| `seed-query/review.md` | 种子地图 v2 结构显示问题清单 + 改进方案（不改代码） |
| `seed-query/api.md` | `/api/seed/*`、`/api/seed/v2/*`、`/api/youyuan/world/seed` 参数与返回 |
