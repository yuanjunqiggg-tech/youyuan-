# Skill 与会话记录地图

> 这个项目的知识不在一个文件里，而在三处：**本地 skill**（可执行规范）、**项目内技能文档**（`upstream/skills/`，会被编进二进制给 AI 用）、**历史会话**（原始需求与踩坑现场）。
> 本文告诉你每一处在哪、什么时候该读、怎么检索。

---

## 1. 本地 Skill（`C:/Users/Lenovo/.workbuddy-ai/skills/`）

### 1.1 游园专用（8 个，开工先读）

| Skill | 路径 | 用途 | 什么时候读 |
|---|---|---|---|
| `youyuan-obey-execute` | `…/skills/youyuan-obey-execute/SKILL.md` | 项目铁律（含「思考用中文」「每句话必须有主」）；违反=事故 | **每次会话开始/压缩后第一条** |
| `youyuan-delivery-loop` | `…/skills/youyuan-delivery-loop/SKILL.md` | 交付闭环：清单总账 + 四类审查子智能体 + 构建发布命令 | 每次继续项目、准备发布前 |
| `youyuan-command-center` | `…/skills/youyuan-command-center/SKILL.md` | 总统筹工作台：必读清单、角色分工、验收纪律、自查「文档看了没」 | 多智能体派单、UI/安全审查、上下文恢复 |
| `youyuan-context` | `…/skills/youyuan-context/SKILL.md` | 上下文速查包（项目史蒸馏），避免每次重新考古 | 任何涉及 APK/内核/命令链/生产机/Worker/MCP/注册/QQ 的任务 |
| `youyuan-maintenance` | `…/skills/youyuan-maintenance/SKILL.md` | 维护与交接：总账、域名/故障区分、证据核对、归档 | 继续维护、交接进度 |
| `youyuan-ui-design` | `…/skills/youyuan-ui-design/SKILL.md` | Mineral Workspace 视觉体系、移动端优先、禁止假组件 | 改任何前端页面/组件/导航前 |
| `youyuan-requirements-memo` | `…/skills/youyuan-requirements-memo/SKILL.md` | 用户显式需求总账（先登记→再动手→完成打勾+证据） | 接到新需求、收尾对账 |
| `youyuan-plugin-runtime-perf` | `…/skills/youyuan-plugin-runtime-perf/SKILL.md` | 插件运行环境性能：批执行器、子进程 RPC、原生插件、保底调度 | 改 Lua/Python/C++ 运行器、命令吞吐 |

> Codex 侧还有一份 `C:/Users/Lenovo/.codex/skills/youyuan-maintenance/SKILL.md`（旧路径，内容可能滞后；以 WorkBuddy 这份为准）。

### 1.2 通用工程 Skill（本项目高频用到）

| Skill | 用途 | 本项目典型场景 |
|---|---|---|
| `android-apk-build` | Gradle + AGP、WebView 壳 + assets 打包、apksigner 验证 | `build.ps1` 出 APK、assets 源集路径坑、构建成功但内容缺失 |
| `android-cloudflare-tunnel` | Android 内自带 cloudflared、libcloudflared.so 交叉编译、DNS 注入 | APK 自带隧道、前台服务权限 |
| `android-host-backend-bridge` | 宿主内嵌独立后端进程 + 反向通道（长轮询/WebSocket） | Go 内核 ↔ Kotlin/Java 桥 |
| `apk-decompile-repatch` | apktool/smali/`//go:embed` 等长替换、五类必踩坑 | 参考上游 APK、patcher |
| `fullstack-feature-verification` | 「声称支持」后的落地验证 + 首屏体积守卫 | 新增运行器/导出格式/主题；白名单加了但 switch 漏了 |
| `go-embed-script-lang` | Go 常驻子进程嵌 Python/Node/Lua 的 JSON-RPC 六条死锁约束 | Python/C++ 插件运行器、`prelude` 注入 |
| `go-workspace-build-hang` | go.work + GOPROXY 联网导致构建挂起 | 离线构建 `GOPROXY=off`、多服务批量构建 |
| `latency-measurement` | 亚毫秒热路径分层、Windows 时钟粒度、P50 恒为 0 的识别 | 命令链吞吐、回调延迟、TPS |
| `mcp-builder` / `mcp-gateway-wrapper` | 写/包一层正规 MCP server（stdio/HTTP、工具 schema） | `/mcp` 工具暴露、把 REST 包成 MCP |
| `multi-agent-app-build` | 多 agent 并行建应用的契约先行、文件隔离、整合验收 | 本项目几乎每轮都在用 |
| `cloudflare-login-via-browser` | Playwright 登录 Cloudflare、Turnstile 必重置；推荐改用 API Token | wrangler 登录、后台操作 |
| `cloudflare-quick-tunnel` | 无账号把本机端口暴露成 trycloudflare 临时地址 | 临时联调、演示 |
| `ds-binary-pull` | 从被控端无损拉二进制（zip/图片） | Prism/云手机取文件 |
| `ponytail` | 「懒资深」模式：最小改动 + 一眼能读完的回复 | 日常改代码的默认工作方式 |
| `plan-grilling` / `grill-me` | 对计划/决策做压力测试（只做决策，不执行） | 大改前的方案评审 |
| `deep-research` | 多源检索 + 报告 | 协议/算法外部资料调研 |
| `prism-agent-fleet` / `prism-forge-*` | 多智能体舰队、铁律、监控 | 大规模并行派单 |

### 1.3 怎么加载

- WorkBuddy：会话里直接说「读 `youyuan-obey-execute`」或由系统按触发词自动加载；路径就是上表。
- 手工读：`Read` 上面任意 `SKILL.md`，前 10 行有 `name/description/触发词`。
- 自建/改 skill：用 `skill-creator`，别把项目临时结论写进通用 skill。

---

## 2. 项目内技能（`upstream/skills/`，30 个）

这些是**给宿主内 AI/Agent 用**的技能包，`ai_skills.go:14` 用 `//go:embed skills` 编进二进制，随 `GET /api/ai/skills` 暴露。

| 分类 | 目录 |
|---|---|
| 运行时 | `runtime-kernel`、`runtime-reference`、`lua-runtime`、`python-runtime`、`cpp-runtime` |
| 命令与游戏 | `command-chain`、`command-chain-api`、`command-execution`、`mcbe-commands`、`bedrock-commands`、`gameplay-events`、`physical-actions`、`player-actions` |
| 世界与建筑 | `world-seed`、`building-transfer`、`nbt-workbench`、`block-media`、`fmbe-display` |
| 连接与观测 | `mcp-connection`、`network-observation`、`observer-bridge`、`public-tunnel`、`account-bridge`、`account-pool`、`self-auth` |
| 其它 | `hq-query`、`qq-assistant`、`efficient-context`、`packet-reference`、`toolbox-live-development` |

种子查询的接口契约在 `upstream/skills/world-seed/index.md`（前面几份报告都引用它）。

---

## 3. 会话记录位置

### 3.1 WorkBuddy（本机主力）

```
C:/Users/Lenovo/.workbuddy-ai/projects/<路径编码>/<sessionId>.jsonl
```

- 本项目对应目录：`c-Users-Lenovo-Documents-Codex-2026-09-28-apk-project/`
- 全机共 **429** 个 `.jsonl`（跨多个工作区）
- 行格式（已实测）：每行一个 JSON 对象
  - `{"type":"session-meta", ...}` 元信息
  - `{"type":"message","role":"user|assistant","content":[{"type":"input_text","text":"..."}],"timestamp":<ms>}` 实际消息
- 同名子目录里可能有附件/临时文件

### 3.2 Codex

```
C:/Users/Lenovo/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl
```

- 共 **21** 个 rollout（2026-10 为主）
- 行格式：`{"timestamp":"ISO8601","type":"session_meta|response_item|event_msg|...","payload":{...}}`
- 早期会话可能压成 `.jsonl.zst`

### 3.3 项目内已蒸馏的会话

| 文件 | 内容 |
|---|---|
| `outputs/_user_messages_raw.jsonl` | 266 条用户原话（带来源 rollout 路径与时间） |
| `outputs/_user_messages_readable.txt` | 2074 行可读版，按时间排序，最适合全文搜索 |
| `新建文件4/00-交接总览.md` ~ `09-*.md` | 2026-10-21 事故期的交接快照 |
| `协作交接/维护入口.md` | **按时间倒序的最新状态**，顶部永远是最新 |
| `outputs/` 下按日期的 `*.md` | 每次交付/事故/审查的报告 |

### 3.4 其它 Agent 工作区

| 平台 | 路径 | 备注 |
|---|---|---|
| DeepSeek Harness | `D:/Users/Lenovo/Documents/deepseek-harness/default-workspace/` | `AGENTS.md`、`TASKS.md`、`handoff/tasks/`；GUI 应用，无 headless CLI |
| Antigravity | `D:/Users/Lenovo/Documents/antigravity-handoff/` | `AGENTS.md`、`TASKS.md`、`tasks/AG-01~03.md`、`results/` |
| 多智能体中枢 | `C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/agent-hub/` | `TASKS.md`、`AUTHORIZATION.md`、`DECISIONS.md`、`TEST_RESULTS.md`、`SUBAGENT_PROMPTS.md` |

---

## 4. 检索配方（直接复制）

### 4.1 在所有 WorkBuddy 会话里搜关键词

```bash
cd "C:/Users/Lenovo/.workbuddy-ai/projects"
grep -rl "种子查询" --include="*.jsonl" . | head -20
```

### 4.2 只搜本项目的用户原话（最提纯）

```bash
cd "C:/Users/Lenovo/.workbuddy-ai/projects/c-Users-Lenovo-Documents-Codex-2026-09-28-apk-project"

# 每条用户消息一行：时间 + 文本（需要 jq）
jq -r 'select(.type=="message" and .role=="user")
       | [((.timestamp/1000)|todate), ([.content[]?|select(.type=="input_text")|.text]|join(" "))]
       | @tsv' *.jsonl 2>/dev/null | grep "部署"
```

没有 `jq` 时用 Node（本机有 22.22.2）：

```bash
NODE="C:/Users/Lenovo/.workbuddy-ai/binaries/node/versions/22.22.2-6/node.exe"
"$NODE" -e '
const fs=require("fs"),path=require("path");
const dir="C:/Users/Lenovo/.workbuddy-ai/projects/c-Users-Lenovo-Documents-Codex-2026-09-28-apk-project";
for(const f of fs.readdirSync(dir).filter(x=>x.endsWith(".jsonl"))){
  const lines=fs.readFileSync(path.join(dir,f),"utf8").split(/\r?\n/);
  for(const l of lines){ if(!l.trim())continue; let o; try{o=JSON.parse(l)}catch(e){continue}
    if(o.type!=="message"||o.role!=="user")continue;
    const t=(o.content||[]).filter(c=>c.type==="input_text").map(c=>c.text).join(" ");
    if(t.includes(process.argv[1])) console.log(new Date(o.timestamp).toISOString(), t.slice(0,200));
  }
}' "缓存"
```

### 4.3 搜 Codex 历史

```bash
cd "C:/Users/Lenovo/.codex/sessions"
grep -rl "种子查询\|seed" --include="*.jsonl" . | tail -20
# 单条上下文
grep -o '.\{0,200\}种子查询.\{0,300\}' 2026/10/19/*.jsonl | head
```

### 4.4 搜已蒸馏的 266 条用户原话（最快）

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
grep -n -A6 -B2 "种子" outputs/_user_messages_readable.txt | head -80
```

### 4.5 找「谁在什么时候改了什么」

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
git log --oneline --since="2026-10-19" -- upstream/frontend/js/youyuan_seedmap_v2.js
git log -p -1 -- upstream/youyuan_seedv2_routes.go | head -80
```

---

## 5. 关键决策纪要（已被反复确认，别再重新讨论）

| # | 决策 | 出处 | 影响 |
|---|---|---|---|
| 1 | 不伪造令牌、不入侵第三方后台、不借用他人身份；只用获授权 API | `PROJECT_STATE.md` §4-1 | 所有认证/账号功能的边界 |
| 2 | 自动重连 / 创造维护开关**保持关闭**（用户明确要求） | §4-2 | 不要「帮着打开」 |
| 3 | 观测只用摄像机视角，不默认旁观/跟随传送；机器人身份必须排除，未知身份不投放 | §4-3 | 抓包/观察者相关改动 |
| 4 | 四层验收：静态核实 → 可构建 → 已运行 → 端到端通过；不许拿进程存活/网页 200 当验收 | §4-4、`BOOT.md` §6 | 所有「完成」声明 |
| 5 | 总账只增量更新，不重跑历史 `update/record_acceptance` 脚本 | §4-5 | 需求计数只以 `需求逐项总清单.json` 为准 |
| 6 | 旧 SSH 临时桥（`ssh_https58.py` / `yy_deploy58_tmp`）已删除，禁止调用 | §4-6 | 部署只用直连 SSH + host pin |
| 7 | GitHub 备份用现有 `yuanjunqiggg-tech/youyuan-`；源码入库，`build/`、`cloudflare/public/`、`private/` 不入库 | §4-7 | 提交范围 |
| 8 | `PROJECT_STATE.md` 为唯一状态源（**现已过期，见 §5.1**） | §4-9 | 读它之前先看日期 |
| 9 | 前端上线必须压缩混淆、禁 sourcemap（铁律第十一条） | `build_frontend.py:1-16` | 不能直接把源码目录传上去 |
| 10 | 签名证书 `cd823622…` 与 versionCode 不可变/不可降 | `build.ps1:88-100` | APK 覆盖安装的前提 |
| 11 | 凭据只放 `D:/YouYuan-APK/private/`，不进文档/日志/提交 | `BOOT.md` §0 | 任何文档都不能贴值 |
| 12 | 原雪球命令链保持停用，不以压测/复现代替修复 | `协作交接/维护入口.md` 2026-10-21 段 | 碰链前先问用户 |

### 5.1 「缓存死锁」这一课（为什么现在这么强调版本戳）

- **抓包机器人不上线**（2026-10-20）：`yyCaptureRobotNamesWithStatus()` 对停用账号的残留 slot 逐个 `bot.self`，任一失败就判定「身份不明」，`reconcile` 据此把可增派数强制归零 → 其余机器人永久不上线。根因不是封号，是**状态自锁**。
- **注册卡在「正在注册」**：客户端裸 `fetch` 无超时 + 服务端 `relay.fetch()` 无超时，中继一挂就永久转圈。已加 80s 客户端超时。
- **前端缓存死锁**：`index.html` 的 `?v=` 不变，浏览器永远拿旧 JS；现象是「代码明明改了，界面还是错的」。所以每次前端发布必须换版本戳，且 `youyuan_seedmap_v2.js:145` 的 CSS 版本要同步。

### 5.2 box 与 Worker 是双链路，别混

| 维度 | Worker（网页） | box（Linux） |
|---|---|---|
| 前端来源 | `cloudflare/public/` 静态资源 | 二进制内 `go:embed` |
| API 来源 | 反代到 Linux 中继 | 本机进程 |
| 更新方式 | `wrangler deploy` | `deploy_linux_bin.py` + systemd |
| 挂了的表现 | 网页打不开/白屏 | `/health` 502、游戏掉线 |
| 验证命令 | `curl toolbox…/health` | `curl 127.0.0.1:19429/health` |

「网页能开」永远不能证明「内核正常」。

### 5.3 「组件皮肤」相关决策

- **两件事不要混**：
  1. **UI 皮肤/主题**：`--phx-*` 令牌 + `[data-theme="dark"]`（`css/youyuan_design.css`）；深色/浅色对比度曾有 1.2:1、1.72:1 的阻断问题，已修为 `--phx-on-primary`、`--phx-text-secondary`。
  2. **游戏皮肤**：官方头像/皮肤库/中性占位（`youyuan_skin_*.go`、`js/youyuan_skin_library.js`）；云端与设备同 UID 合并、未知头像用中性占位，不伪造。
- 规则：**不伪造官方头像**；缓存可命中磁盘；换主题不能让任何页面白底浅字。

### 5.4 种子 v2 精度校准（本轮最值钱的一次修正）

| 阶段 | 指标 | 说明 |
|---|---|---|
| 修 depth 前 | 72.76% | 48 个 tile 里 7 个 0.00%，整片画成繁茂洞穴 |
| depth 改为 `1.0 + off` 后 | 96.26% | 采样点落在 voronoi 格「角」上（`chunk*16+8` ≡ 0 mod 4） |
| 偏移 +2 后 | **99.70%** | `yyV2RenderSampleOff(bpp)`，24503/24576，41.4ms/tile |
| 结构放置 | 13 类逐点 100% | 与 `D:/种子查询器自研/src/world.mjs` 对拍 |
| 要塞 | `ring_approx` | 环位候选，原版还会做群系回拉；**不再宣称 exact**；基岩版直接 501 |

来源：`outputs/review-seed/报告.md`、`seedv2_biome_golden_test.go`、`youyuan_seedv2_routes_test.go`。
外壳（布局/图例/密度）问题是另一回事，见 `seed-query/review.md`。

---

## 6. 交接模板（复制到 `协作交接/维护入口.md` 顶部）

```
> YYYY-MM-DDTHH:MM:SS+08:00 最新：[一句话主题](outputs/YYYY-MM-DD-主题.md)。
> 线上实际：网页 <version/version-id>、原生 <binary sha 前 8 位>、认证 <version>。
> 已做：<3 条以内，带证据文件>。
> 未做/未验：<明确列出，别写「基本完成」>。
> 回滚点：<commit/tag/备份目录>。
> 注意：<并行 Agent、停用状态、禁止操作>。
```
