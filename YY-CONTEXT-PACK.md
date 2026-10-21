# 游园项目 · AI 上下文速查包（蒸馏版）

> 本文件是给 AI 的「开工必读」。每次会话开始先读完本文件再动手。
> 维护：每次重要进展后更新本文件；改动随 git 提交。

## 0. 铁律（用户红线）

1. **雪球菜单命令链 txt 一个字节都不许改**（用户自己测过的模板）。要改先问。
2. **不许用理论搪塞**。每个结论给实测证据（HTTP 实测/日志/抓包/测试结果）。
3. 用户说「没做好」就是没做好，先复现再下结论，不辩解。
4. 中文回复，思考过程也用中文。

## 1. 项目是什么（30 秒版）

- **项目根**：`C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project`（git 仓库，main 分支）
- **产品三件套**：
  - `upstream/` Go 内核（嵌套 module `flowers-for-machines`），机器人 + 命令链 + 插件运行时 + HTTP API(19429)
  - `upstream/android/` Android APK（Python 插件运行时）
  - `cloudflare/` + `auth-service/` Cloudflare Workers（认证/身份/MCP 中继/DO 存储）
- **生产机**：47.102.156.125（阿里云 ECS），`youyuan.service`，端口 19429 仅绑 127.0.0.1
- **SSH**：paramiko + `D:/YouYuan-APK/private/linux-access.json`（root + 密码 + host key pin）
- **Python**：`C:/Users/Lenovo/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe`
- **构建**：`cd upstream && GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o ../build/youyuan-linux-x86_64 .`

## 2. 当前进行中的事故（2026-10-21，box 已被重置）

- **用户重启无效后误触「重置系统」**：系统盘已还原为全新 Ubuntu 24.04，SSH banner 恢复但
  host key 已变（，pin 已更新）；**root 仅接受 publickey**，密码认证被禁。
- box 上原数据（/opt/youyuan、/var/lib/youyuan）随系统盘丢失；**数据盘（若有）待确认**。
- Cloudflare 侧不受影响：用户账号 DO、游戏账号 vault、隧道 ingress 规则都在 CF，未丢。
- 恢复套件已全部备好：（一键重建）+ 本地资产：
  二进制 、cloudflared、、
  隧道令牌 、发信 、
  配置快照 + 、插件源码 （ds-r20a 两个包）、
  命令链 dump （含雪球菜单_综合功能.txt）。
- **唯一卡点：SSH 进入方式**。等用户在控制台开密码认证（或绑私钥给我），先
   盘点数据盘，再全量重建。

## 3. 部署事实（别再猜）

| 部署物 | 要重启 box 吗 | 生效方式 |
|---|---|---|
| Go 二进制（upstream） | **要** | `systemctl restart youyuan.service`，PID 会变 |
| Cloudflare Worker | 不要 | `wrangler deploy` 即时生效 |
| 前端静态页 | 不要 | 随 Worker 部署 |
| config.json | 不要 | `/api/youyuan/connection-policy` 等 API 热生效 |
| Lua/Python 插件 | 不要 | 运行时热加载 |
| 命令链 txt | 不要 | 导入即生效（但雪球菜单禁改，见铁律 1） |

## 4. 卡顿/握手风暴事故的完整真相（用户已指认）

- **起源**：`9cd26f7`（2026-10-03 10:14，Codex 会话提交）为一次偶发进服失败，同时改了：
  - 传输层：`bot_conn.go`（AI 通道 CommandRequest→SettingsCommand+WritePacket+每条 execute 包装，丢失 256 包攒发）
  - `raknet/conn.go` idle timeout 5s→30s
  - **命令链执行方法**：`youyuan_chain_buffered.go`、`youyuan_commands.go`、`youyuan_plugin_commands.go`、`youyuan_sense_api.go`
- 后续 `729c59c→19ba43c→0049249→ed3a0cf→6f703ec` 在坏传输层上叠补丁，问题反复。
- **已修（`5d64743`）**：回退传输层（WritePacketBounded+条件包装）、RakNet 握手 10s 上限（原来 request2 无界重发，单次拨号最多 ~300 个握手包）、idle timeout 回退 30s、死亡 20s 豁免窗口。
- **仍待办**：逐项复核 9cd26f7 对命令链执行方法的改动（chain_buffered/commands/plugin_commands/sense_api），列出回退清单交用户确认。**未列清单前不准再动这些文件。**

## 5. 常用工具与入口（别重新找）

| 用途 | 路径 |
|---|---|
| SSH 内存/磁盘诊断 | `maintenance/diag_memory.py` |
| SSH 握手/注册取证 | `maintenance/diag_handshake_reg.py` |
| 恢复守护 | `maintenance/server_recovery_daemon.py`（日志 `maintenance/server-recovery.log`） |
| 部署（带回滚点） | `maintenance/deploy_linux_bin.py` |
| MCP 专用连接令牌 | `D:/YouYuan-APK/private/Grok专用连接网址-0.10.11.txt` |
| CDP 探针（Chrome 9333） | `maintenance/cdp-identity-probe.js`、`cdp-mcp-probe.js` |
| 用户历史消息蒸馏 | `outputs/_user_messages_readable.txt`、`outputs/codex-历史要求提炼.md` |
| 项目状态/待办 | `BOOT.md`、`未解决清单.md`、`任务备忘录.md`、`PROJECT_STATE.md` |

公网入口：`auth.youyuanqi.dpdns.org`（认证页+Worker）、`toolbox.youyuanqi.dpdns.org`（工具前台+MCP 链接）。MCP 链接格式 `/mcp/link/<64hex>`。

## 6. 需求进度（10 条清单，2026-10-21 状态）

- **已上线**：命令通道修复 + 死亡误报修复（`5d64743`，待 box 恢复后部署）。
- **未做**：房间发现三分类正方形卡片（"多维度"）、终端富配色、视角观察不开旁观、UI 重构若干。
- **群服互通缺口**：RAG 向量库、表情包收发、聊天记录导出。
- QQ 官方机器人能力已存在（`upstream/youyuan_qq*.go` 13 个文件）。

## 7. QQ 群机器人（非官方，进行中）

- 选型（等用户最终确认）：**LangBot**（AGPL-3.0，Go 栈同源，WebUI/插件/知识库开箱）+ **NapCat**（OneBot v11 QQ 协议端）+ **Qdrant**（向量库）+ 自研群管/权限/群主指令/记忆插件。
- AstrBot（41.5k★ Python 栈）为备选；NoneBot/Koishi 否决（WebUI/权限/RAG 都要自建）。
- **部署目标**：Linux 云服务器；Cloudflare Worker 做前置（MCP/webhook/反代）。用户可提供 QQ 号。
- 产出必须落库：/src /config /prompts /plugins /skills /memo /docs /scripts /vendor，全 Git，AGPL 义务记 /docs/vendor。

## 8. 事故速查（症状→原因→处置）

| 症状 | 第一嫌疑 | 处置 |
|---|---|---|
| 注册卡「正在注册」 | box 邮件中继挂 | 恢复 box；线上 auth 页已有 80s 超时，不会永久转圈 |
| MCP 503「执行端未连接」 | box↔CF 中继断 | 恢复 box 后用 cdp-mcp-probe.js 复测 |
| 机器人卡顿/掉线 | 重连风暴→握手洪水 | 已堵（5d64743）；部署后真服 A/B 复测 |
| SSH banner 超时 | box sshd 失联 | 控制台重启；server_recovery_daemon.py 守着 |
