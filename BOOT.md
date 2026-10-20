# BOOT.md — 接手本项目先读这一份

> 用途：会话被压缩、换会话、换 AI 接手时，**只读本文件就能继续干活**，不必再逐个翻文档。
> 权威状态仍在 `PROJECT_STATE.md`；本文件负责告诉你去哪里、以及那些翻遍仓库才能重新找到的事实。
> 规则不变：与本文件或 `PROJECT_STATE.md` 冲突的记忆一律作废；记录矛盾时先停下问用户。

## 0. 先跑这三条（10 秒拿到当前状态）

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
git log --oneline -3 && git status --short     # 当前 HEAD 与工作区是否干净
python - <<'EOF'                                # 总账计数（唯一计数来源）
import json;d=json.load(open('需求逐项总清单.json'))
print(len(d.get('items',d)), sum(1 for v in (d.get('items') or d).values() if isinstance(v,dict) and not v.get('closed_at')))
EOF
ls ../agent-hub/TEST_RESULTS.md ../agent-hub/REQUIREMENTS_BACKLOG.md
```

然后读 `PROJECT_STATE.md` 第 1–3 节（目标 / 任务清单 / 已完成与未完成）。**不要完整读完**，只读这三节就够动手。

## 1. 路径速查（这些翻一遍要几分钟）

| 用途 | 路径 |
|---|---|
| 项目根（git 仓库） | `C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project` |
| 宿主 Go 源码（绝大部分后端逻辑） | `project/upstream/*.go` |
| 前端（APK 内 WebView） | `project/upstream/frontend/js/`、`.../frontend/css/`、`index.html` |
| 云端 Worker（网页控制台 + 身份 + 邮件） | `project/cloudflare/*.mjs` |
| 独立认证服务 | `project/auth-service/`（`worker.mjs`、`engine.go`、`public/`） |
| Linux 服务端部署脚本 | `project/linux/` |
| 多智能体中枢、派单记录、测试结果 | `../agent-hub/`（`AGENTS.md` / `TEST_RESULTS.md` / `REQUIREMENTS_BACKLOG.md` / `SUBAGENT_PROMPTS.md`） |
| 备份快照 | `../agent-hub/backups/`（工具 `python ../agent-hub/tools/backup.py snap`） |
| 交付证据 | `project/outputs/*.md` / `*.json` |
| 需求总账（120 项） | `project/需求逐项总清单.json` |
| 历史会话（Codex 提过的原始需求） | `C:/Users/Lenovo/.codex/sessions/` |

## 2. 命令速查（含踩过的坑）

```bash
# Go 宿主：改完 .go 必跑
cd project/upstream && go vet . && go test . ./config ./state ./building

# 前端 JS：改完必跑 node --check（不是 --test）
cd project && node --check upstream/frontend/js/你要改的文件.js

# 云端 / 认证全部测试
cd project && node --test cloudflare/*.test.mjs auth-service/*.test.mjs

# 脚本用的 Python（httpx 只在里面，系统 python 没有）
D:/YouYuan-Codex/.venv/Scripts/python.exe
```

| 环境事实 | 值 |
|---|---|
| Go | `go1.27.0 windows/amd64`，在 PATH 上 |
| 本地代理 | GitHub / Cloudflare 必须走 `http://127.0.0.1:9674`，直连被重置 |
| Linux 生产机 | `47.102.156.125`，SSH 22，主机密钥 pin `24bd2547…59bc` |
| Linux 服务 | systemd `youyuan.service`，端口 **19429**（不是 19439，旧脚本写错过） |
| 公网入口 | 控制台 `toolbox.youyuanqi.dpdns.org`、认证 `auth.youyuanqi.dpdns.org`、更新 `updates.youyuanqi.dpdns.org` |
| 拉 updates 更新清单 | 直连 urllib 会 403，必须带浏览器 UA |
| SSH 中继桥 | `ssh_https73.py` 报「插件没有启动」，不可用；用直连 SSH 22 + host pin |
| APK 签名 | `D:/YouYuan-APK/private/signing.json` + `youyuan-release.jks`；**证书与 versionCode 不可变** |
| 凭据位置 | `D:/YouYuan-APK/private/`（`linux-access.json`、`cloud-keys.json`）、`AppData/Roaming/xdg.config/.wrangler/config/default.toml` |

## 3. 结构性坑（别再花时间试）

1. **`cmd/server` 永远编译不过**：它是 `package main` 却 `import "bot-apk"`（同为 `package main`），结构上不可能成立，构建脚本也没用它。`go build ./...` 报它算预期，不是你的改动造成的。
2. **`internal/playerrelay` 的 UDP 回环测试偶发失败**（连跑 通过/失败/通过），与业务改动无调用关系。用户未裁决前不要动它。
3. **`wrangler deploy` 上传到 22/34 资产可能中断且日志无错误行**。中断不会激活部署、不污染线上。**先在线核查 latest.json 基线，再重试**；重试时资产按内容哈希差量，几秒就完成。
4. **`interval_seconds` 的 0.001 是合法值**（`youyuan_observer_patrol.go:67` `yyPatrolDuration`，两份既有测试专门断言）。它是「提交目标间隔」不是睡眠，不要加硬下限。
5. **`yyGlobalCaptureConfig` 新增数值字段必须用指针**：值类型会让旧 `global-capture.json` 解码成 0，升级后静默失效（`body_radius_blocks` 已经踩过）。
6. **前端兜底不能按「没有 content-type 就判非 JSON」写**：`api.js` 这样写过，导致 mock 测试 3 项失败。只有服务端明确声明非 JSON 才兜底。
7. **子 Agent 派单 429/500 频发**：连续 4 次全部失败是常态。不原地重试；一条 prompt 一件事、给路径不给方向、限输出 ≤900 字、要 `文件:行号` 证据。纪律见 `../agent-hub/SUBAGENT_PROMPTS.md` 末尾。
8. **`auth-service` 的 `runKernel`→`relay.fetch()`（`cloudflare/email-verification.mjs:51`）无服务端超时**，工具箱侧 `youyuan_identity.js` 原先也无客户端超时 → 中继一挂界面就永久转圈。客户端超时已补（80s），服务端仍未加。

## 4. 授权分级（`../agent-hub/AUTHORIZATION.md`）

- **A 本地只读 / 构建**：默认允许。
- **B 本地写 + 暂存**：需授权。
- **C 生产写**（`wrangler deploy`、重启 systemd、APK 上架、`git push`）：逐项授权，执行前再确认一次。
- 用户 2026-10-20 14:45 给过一次性全权（R5），条件是**文件提前备份并打时间戳**、多调子智能体、更新后给实际效果。
- 无论是否授权都不做：把凭据写进文件/日志/提交；降 versionCode；换签名证书；动 `cmd/server` 与 `playerrelay`；未授权合并主分支或 push。

## 5. 当前进度指针（2026-10-20 20:xx）

- HEAD `fd895df`（0.10.75 三平台发布完成），线上基线 **0.10.75 / versionCode 1075**。
- 本轮已修（**尚未提交、未构建、未部署**）：
  - 抓包机器人不上线 = **结构性死锁**，不是封号。`youyuan_capture_runtime.go` `yyCaptureRobotNamesWithStatus()` 把「停用账号的 slot 指向已死子内核」当成「身份不明」，`youyuan_capture_reconcile.go:119` 据此把 `short` 强制归零 → 其余机器人永远不上线。已改为先用 `GET /api/bot/status` 判在线，只有**在线但报不出名字**才暂停增派；并新增 `yyAccountsRoutes` 加载时清空停用账号残留 slot 的自愈。回归测试 `youyuan_capture_identity_test.go` 4 项。
  - 注册卡在「正在注册」= 客户端无超时（见第 3 节第 8 条）。已给 `youyuan_identity.js` 的 `yyIdentityRequest` 加 80s AbortController + 可操作中文提示。回归测试 `cloudflare/identity-request-timeout.test.mjs` 3 项。
  - 测试：`go test .` 全通过；`node --test cloudflare auth-service` **271/271**。
- 下一批待办与 11 条历史原始需求：`../agent-hub/REQUIREMENTS_BACKLOG.md`。

## 6. 汇报纪律

有 evidence 才写「完成」。区分「已运行（本地测试通过）」/「已构建」/「已部署」/「真机真服验收」四层——本项目过去最大的返工来自把这四层混为一谈。
