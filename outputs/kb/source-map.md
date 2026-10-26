# 游园工具箱 / MCORT · 源码地图

> 审计对象：`C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project`（源码版本 **0.10.78**，`upstream/youyuan_product.go:14`）
> 生成时间：2026-10-26 · **只读审计**：未改任何 .go/.js/.css/.html/.json，未部署、未重启服务、未动版本戳、未做 git 操作
> 口径：行数为 `wc -l` 实测值；符号名取自 `grep "^func "` / `grep "^type "` 原文，不靠文件名推测

---

## 0. 一页结论

| 项 | 事实 | 依据 |
|---|---|---|
| 内核形态 | Go 单模块 `bot-apk`、`go 1.26`；**646 个 .go 全在同一目录、同一个 `package main`** | `upstream/go.mod:1,3`；`grep -h "^package " upstream/*.go` 结果只有 `package main` × 646 |
| 代码量 | 非测试 352 个 / 112285 行；`_test.go` 294 个 / 40516 行 | `ls *.go | wc -l`=646；`wc -l` 实测 |
| 命名家族 | 478 个 `youyuan_*`（含 228 个测试）；QQ 域独占 92 个文件；种子域 15 个实现文件合计 8039 行 | `ls youyuan_*.go`、`ls youyuan_qq_*.go`、`wc -l youyuan_seed*.go seedv2_*.go` |
| HTTP 面 | `main.go` 内 183 处 `mux.Handle*`，`yyRegisterAPI` 再挂 30+ 组子路由 | `grep -c "mux.HandleFunc|mux.Handle(" main.go`=183；`youyuan_remote.go:161-198` |
| 监听范围 | 只绑 `127.0.0.1`，局域网/外网进不来；对外靠云端 WS 反连 | `main.go:5289`（`main.go:5284-5288` 注释明确纠正了「绑 0.0.0.0」的旧说法） |
| 中间件栈 | `yyConsoleAuth(yyIntegrityGate(corsHandler))`，完整性闸门在 CORS 之内、console 鉴权之外 | `main.go:5280` |
| 前端 | 单页 `index.html` + **118 个 `<script>` 开标签**（其中 111 个带 `src`，7 个内联）；`frontend/js` 76 个 .js + 26 个 .js.esb + 3 个子目录 | `grep -o '<script' frontend/index.html \| wc -l`=118；`grep -o '<script[^>]*src=' … \| wc -l`=111；`ls frontend/js` |
| 最大文件 | `main.go` 5326 行 > `task_dispatch.go` 2818 > `bot_conn.go` 2030 | `wc -l` |

---

## 1. 目录树（2~3 层；前端 js 全列）

```
project/
├── upstream/                     # ★ Go 内核（646 个 .go 平铺在此，含 294 个 _test.go）
│   ├── main.go  bot_conn.go  task_dispatch.go  youyuan_*.go …（见第 3 节）
│   ├── frontend/                 # 内核内嵌前端（//go:embed frontend/**，main.go:52-53）
│   │   ├── index.html            # 唯一 SPA 壳，118 个 <script>（111 个带 src）
│   │   ├── draw.html  piano.html  cap_test.html  mcf-syntax.mtsx  _headers
│   │   ├── css/                  # 621K：base/components/layout/style + 30 余个 youyuan_*.css
│   │   ├── js/                   # 4.6M
│   │   │   ├── 核心：app.js(550) api.js(70) sse.js(455) utils.js(395) theme.js(44)
│   │   │   │        compat.js youyuan.js youyuan_boot.js youyuan_boot_stable.js
│   │   │   │        youyuan_console_token.js youyuan_account_boot.js youyuan_poll_policy.js
│   │   │   │        mcort_theme.js youyuan_navigation.js youyuan_model_brand.js
│   │   │   ├── 业务：youyuan_dashboard.js youyuan_settings.js youyuan_settings_groups.js
│   │   │   │        youyuan_workspace.js youyuan_studio.js youyuan_plugin_studio.js
│   │   │   │        youyuan_skill_library.js youyuan_market.js youyuan_library.js
│   │   │   │        youyuan_reference.js youyuan_usability.js youyuan_exports.js
│   │   │   │        youyuan_export_center.js youyuan_upload.js youyuan_host_files.js
│   │   │   │        youyuan_converters.js youyuan_chain_center.js youyuan_chain_upgrade.js
│   │   │   │        youyuan_execution_options.js youyuan_global_capture.js youyuan_sense.js
│   │   │   │        youyuan_connections.js youyuan_connection_policy.js
│   │   │   │        youyuan_connection_diagnostics.js youyuan_room_discovery.js
│   │   │   │        youyuan_identity.js youyuan_identity_recovery.js youyuan_game_auth.js
│   │   │   │        youyuan_game_controls.js youyuan_mcp_scope.js youyuan_mcp_tutorial.js
│   │   │   │        youyuan_social.js youyuan_terminal.js youyuan_tunnel.js youyuan_sdk.js
│   │   │   │        youyuan_theme.js youyuan_appearance.js youyuan_block_video.js
│   │   │   │        youyuan_skin_library.js youyuan_skin_media.js youyuan_skin_viewer.js
│   │   │   │        youyuan_seed_local.js youyuan_seed_page.js youyuan_seedmap_v2.js
│   │   │   │        youyuan_accounts.js youyuan_agent_attachments.js youyuan_agent_layout.js
│   │   │   │        youyuan_agent_profiles.js youyuan_agent_studio.js
│   │   │   │        youyuan_agent_workspace_ui.js
│   │   │   ├── QQ 域：youyuan_qq_assistant.js youyuan_qq_assistant_tools.js youyuan_qq_config.js
│   │   │   │        youyuan_qq_console.js youyuan_qq_diagnostics.js youyuan_qq_groups.js
│   │   │   │        youyuan_qq_guard.js youyuan_qq_knowledge.js youyuan_qq_member_ui.js
│   │   │   │        youyuan_qq_memory.js youyuan_qq_moderation.js youyuan_qq_serverinfo.js
│   │   │   ├── *.js.esb ×26（构建副本/回退）：app sse youyuan_accounts youyuan_agent_layout
│   │   │   │        youyuan_agent_profiles youyuan_agent_studio youyuan_converters
│   │   │   │        youyuan_export_center youyuan_exports youyuan_game_auth
│   │   │   │        youyuan_game_controls youyuan_global_capture youyuan_host_files
│   │   │   │        youyuan_identity youyuan_identity_recovery youyuan_plugin_studio
│   │   │   │        youyuan_qq_assistant youyuan_qq_diagnostics youyuan_qq_memory
│   │   │   │        youyuan_room_discovery youyuan_sense youyuan_settings_groups
│   │   │   │        youyuan_skill_library youyuan_studio youyuan_upload youyuan_workspace
│   │   │   ├── features/ (30) ai_assist announcements building building_mgmt camera
│   │   │   │        cb_engineering drawing filepicker fly helpers hq_query intro_video
│   │   │   │        itemmaker mapart mapplayer mcfunction migrate_lite music notifications
│   │   │   │        others personalize plugin_market plugin_mgmt preview qq_bridge
│   │   │   │        qq_template seed_map skin（另有 building/mapart/preview 三个 .js.esb）
│   │   │   ├── pages/ (5 + 2 .esb) home.js plugins.js settings.js tasks.js terminal.js（.esb: settings tasks）
│   │   │   └── vendor/ (3) marked.min.js three.min.js youyuan_editor.js
│   │   ├── blocks/  images/  sounds/  assets/(brands, intro.mp4, default-head.*)  licenses/
│   │   └── _uifx/                # UI 效果/实验素材
│   ├── building/                 # 结构导入导出子包（bdx/importer/exporter/classify/isolation）
│   ├── config/                   # 配置 + 出口 + 游戏身份（config.go, game_egress.go, game_identity.go）
│   ├── state/                    # 任务与断点（task.go, checkpoint.go, tickingarea.go）
│   ├── media/                    # 地图色/视频/MIDI/皮肤合成（map_color.go, video.go, midi*.go）
│   ├── prism/                    # 第三方登录客户端（api.go, client.go, toolbox.go）
│   ├── internal/playerrelay/     # 玩家中继（内部包，外部不可 import）
│   ├── cmd/                      # 三个旁路可执行：parse_mcworld / server / youyuan-player-gateway
│   ├── sdk/                      # 原生插件 SDK 样例（cpp_observer.cpp, python_*.py, cpp/）
│   ├── skills/                   # 20 个内置 AI 技能包（command-chain, nbt-workbench, …）
│   ├── flowers-for-machines/     # vendored MC 协议库（core/client/nbt_parser/tanlobby/…）
│   ├── android/                  # Android 工程（app/, gradle/）
│   ├── assets/qq-assistant/      # QQ 助手资源
│   ├── docs/                     # 10 项：SEED_MAP_V2.md、review-*.md、shots/、tools/、ui-harness/
│   ├── samples/ testdata/ third_party/ work/ build/ 插件数据文件/ 插件配置文件/
│   └── 插件开发文档.md 词库开发文档.md  libprism.h  fix_frontend.py  bot-apk.exe
│
├── cloudflare/                   # ★ 云端 Worker（60 项 / 57 个 .mjs）：worker.mjs identity.mjs oauth.mjs
│   │                             #   oauth-consent-page.mjs mcp-discovery.mjs mcp-link-policy.mjs
│   │                             #   pool-identity.mjs updates.mjs turnstile.mjs
│   │                             #   email-verification.mjs private-distribution.mjs
│   │                             #   + 30 余个 *.test.mjs + public/ + fixtures/ + wrangler.jsonc
├── auth-service/                 # ★ 登录/注册内核（Go → CLI 与 WASM 双目标）
│   │                             #   domain.go engine.go room_discovery.go skin_preview.go
│   │                             #   tan_lobby_contract.go kernel_cli.go(!js) kernel_js.go(js&&wasm)
│   │                             #   tcp_bridge_js.go + public/(console.js, skin-viewer.js)
│   │                             #   产物：kernel.wasm / auth-service.exe / GO-RUNTIME-LICENSE
├── maintenance/                  # 66 项运维脚本：diag_*.py perf_*.py deploy_*.py chain_*.py
│   │                             #   seal_binary.py publish_apk.py rebuild_box.py patches/ 若干 *.md
├── chat-plugin/youyuan-toolbox/  # 聊天侧插件包：plugin.json + mcp.json + skills/
├── linux/                        # 云主机侧：direct_gateway.py direct_tunnel.py start.sh
│                                 #   install-bundle.sh youyuan*.service validate-plugin-install.py
├── observer-bridge/              # 观测桥：agent.py + netease/YouyuanObserverBP
│                                 #   netallay/(Java 插件 6 文件) + 2 篇协议文档
├── native/counter.cpp            # 原生插件 C ABI 样例（yy_create/yy_event/yy_destroy）
├── editor/editor.js              # CodeMirror 6 编辑器 + package.json + third-party/
├── docs/                         # HARDENING.md + design/
├── outputs/                      # 交付物（本文件在 outputs/kb/）
├── preview/ private/ samples/ third_party/ workspace/ incident-fixes/ 协作交接/ 新建文件4/
└── build/                        # 构建产物
```

---

## 2. 子项目定位表

| 目录 | 定位 | 关键证据 |
|---|---|---|
| `upstream/` | Go 内核本体；Android(JNI)/桌面/Linux 三形态共用一份代码，靠 `//go:build` 分叉 | `libmain.go:1`(android)、`main_native.go:1`(!android)、`youyuan_accounts_{android,desktop,unix,windows}.go` |
| `upstream/frontend/` | 被 `//go:embed frontend/**` 打进二进制的 SPA，`handleIndex` 从 embed FS 读 | `main.go:52-53`、`main.go:268` |
| `upstream/docs/` | 内核内部技术档案（非用户文档）：SEED_MAP_V2、三份 2026-10-23 review、截图与 UI 试验台 | `ls upstream/docs` |
| `upstream/flowers-for-machines/` | vendored 的 MC 基岩协议库，`go.mod` 里是 `v0.0.0` | `upstream/go.mod:6` |
| `upstream/cmd/` | 三个旁路可执行：`parse_mcworld`、`server`、`youyuan-player-gateway` | `ls upstream/cmd` |
| `cloudflare/` | 账号/授权/设备/插件市场云端（Cloudflare Worker ESM），一能力一 `.mjs` + 同名 `.test.mjs` | `cloudflare/worker.mjs`、`wrangler.jsonc` |
| `auth-service/` | 登录内核，同一份 Go 编译成 CLI 与 WASM 两个目标，给 Worker 调用 | `kernel_cli.go:1`(!js) / `kernel_js.go:1`(js&&wasm)、`kernel.wasm` |
| `maintenance/` | 开发期运维/诊断/部署/压测脚本仓库（Python 为主），不参与运行时 | `ls maintenance`（diag_/perf_/deploy_ 前缀） |
| `chat-plugin/` | 聊天平台侧插件包，与内核通过 MCP 通信 | `chat-plugin/youyuan-toolbox/mcp.json` |
| `linux/` | 云主机直连网关/隧道 + systemd 单元 + 安装脚本 | `linux/direct_gateway.py:1-8`（只反代本机 19429、固定 Origin 白名单） |
| `observer-bridge/` | 第三方观测端接入：Python 发送器 + 网易/NetAllay 两端 Java 插件 | `observer-bridge/agent.py:1-4`、`netallay/YouyuanObserverPlugin.java` |
| `native/` | 原生插件 ABI 样例（`yy_create/yy_event/yy_destroy` + host 回调） | `native/counter.cpp:6-12` |
| `editor/` | 前端代码编辑器（CodeMirror 6 + lua/python/cpp/json + mcfunction 自定义高亮） | `editor/editor.js:1-12` |

---

## 3. `upstream/*.go` 职责表（352 个非测试文件，按 `ls` 顺序）

| 文件 | 一句话职责 | 关键导出/入口 | 行数 |
|---|---|---|---|
| `ai_chat.go` | AI 对话主链路：装配系统提示与工具表、把结果以 SSE 流式回传前端 | `handleAIChat`, `buildAISystem`, `newAISSE` | 229 |
| `ai_filetools.go` | AI 的文件读写工具：沙箱白名单、改动记录、插件路径判定 | `resolveAIFilePath`, `aiNoteModified`, `isPluginFilePath` | 326 |
| `ai_game.go` | 把 AI 的 `game.*` 工具调用映射到 BotManager 方法（完整 game API 桥） | `toolGameCall` | 296 |
| `ai_handlers.go` | `/api/ai/models`、`/skills`、`/sessions`、`/snapshot` 等 handler 与 api_key 脱敏 | `handleAIModels`, `maskKey`, `yyAIUpsertModel` | 376 |
| `ai_models.go` | AI 模型注册表（`ai_models.json`）与角色配置 | `AIModel`, `AIModelRole`, `aiModelRoles` | 278 |
| `ai_observe.go` | AI 观察缓冲：`wait_chat` / `listen_packet` 共用订阅骨架 | `newAIObserver`, `feed`, `wait`, `chatHubSubscribe` | 118 |
| `ai_provider.go` | LLM Provider 调用层：OpenAI/Anthropic 流式与工具调用回合 | `complete`, `completeOnce`, `completeAnthropic`, `aiHTTPStream` | 754 |
| `ai_questions.go` | `user_question` 工具：AI 向前端弹带选项的问题并等答复 | `aiQuestionRegistry`, `askUser` | 79 |
| `ai_session.go` | AI 会话与改动存档持久化（`ai_sessions/`、`ai_snapshots/`） | `aiSessionStore`, `aiSnapshotStore`, `Save` | 296 |
| `ai_skills.go` | AI 技能库（内嵌 + 磁盘 `ai_skills.json`），按 id 解析 | `AISkill`, `aiSkillStore`, `resolveSkill` | 221 |
| `ai_tools.go` | AI 工具注册表与执行器（工具名 → handler） | `aiToolDefs`, `execTool`, `aiArgStr` | 597 |
| `apk_signing.go` | 从 APK 解析 v2/v3 签名块并取证书 DER 哈希 | `apkSigningCertHash`, `v2BlockCertHash` | 156 |
| `bot.go` | 单连接封装 `botConn`：收包循环、`sendChat`（2s/条限流 + 50 字截断） | `botConn`, `runBot`, `sendChat`, `processPacket` | 195 |
| `bot_action.go` | 解析 `!action:key=val,key=val` 形式的动作指令 | `handleBotAction`, `parseAction` | 94 |
| `bot_conn.go` | 内核连接核心 BotManager：登录、包分发 switch、断线、SendCommand | `BotManager`, `Connect`(153), `registerPacketHandlers`(627), `Disconnect`(1336) | 2030 |
| `bot_op.go` | OP 权限探测：周围方块/命令回执判断是否拿到 OP | `opInfo`, `checkSurroundingOP`, `HandleOPPacket` | 197 |
| `cache.go` | 结构文件缓存的薄包装，转调 `building` 包 | `loadStructureFileCached`, `clearBuildingCache` | 14 |
| `camera_handler.go` | 相机/录制/玩家列表的 HTTP handler | `handleCameraStart`, `handleCameraStop`, `handlePlayersList` | 406 |
| `camera_recorder.go` | 相机采样、镜头切分、导出为命令序列 | `CameraSample`, `CameraShot`, `StartRecording`, `GenerateCommands` | 704 |
| `cb_export.go` | 命令方块工程导出：扫描结构 → 还原 CB 链与方块 | `CBSimpleBlock`, `CBlockChain`, `handleCBExportScan` | 724 |
| `cb_import.go` | 命令方块工程导入：解析 `.cb` 工程文本/属性/坐标 | `CBParsedProject`, `parseCBProject`, `parseCBLineComment` | 678 |
| `cb_stats.go` | 命令方块统计（全量/概览/资源）与链预览 | `CBFullStats`, `handleCBStats`, `computeCBFullStats` | 209 |
| `chat_menu.go` | 聊天栏菜单：选项渲染与玩家序号选择 | `ChatMenu`, `ShowMenu`, `parseMenuChoice` | 61 |
| `chat_trigger.go` | 触发词匹配、参数解析、命中后调用插件 Func（含并发闸门） | `ChatTrigger`, `matchTriggerWord`(45), `handleChat`(245) | 359 |
| `cmd_response.go` | 命令同步等待封装（WS 自动化通道 / 玩家通道） | `SendWSCommandWithTimeout`, `SendPlayerCommandWithTimeout` | 85 |
| `command_dispatch.go` | 命令通道抽象（player/console/ai）与限速、配额 | `CommandChannel`, `SendCommandCh`, `waitControlCommandSlots` | 170 |
| `device_model_android.go` | Android 构建下读设备型号与平台名 | `getDeviceModel`, `getPlatformName` | 26 |
| `device_model_stub.go` | 非 Android 构建的空实现 | `getDeviceModel`, `getPlatformName` | 13 |
| `draw_handlers.go` | 像素画板 HTTP 接口（创建/落笔/撤销/导出）与方块取色 | `handleDrawCreate`, `handleDrawStroke`, `drawTextureColor` | 439 |
| `draw_session.go` | 画板会话唯一真源（网页画布与悬浮窗原生画布共用） | `DrawSession`, `newDrawSession`, `RegionFromOrigin` | 553 |
| `export_subchunk.go` | 用 SubChunkRequest 协议导出区域方块，写 LevelDB / `.mcworld` | `SubChunkExportTask`, `RunSubChunkExport` | 559 |
| `fleet.go` | 同进程多机器人舰队（旧版 fleet，与账号池不是一回事） | `Fleet`, `FleetConnect`, `FleetStatus` | 348 |
| `floating.go` | 悬浮窗事件分发：SSEHub → WS 客户端 + JNI 回调 | `initFloatingHub`, `handleFloatingWS`, `handleFloatingCommand` | 141 |
| `floating_jni.go` | 后台协程把 floatingHub 消息经 JNI 推给 Java | `init` | 27 |
| `fly.go` | 心跳保活 + 飞行/移动/传送/按键注入的底层动作 | `startHeartbeat`, `MoveRelative`, `TeleportTo` | 829 |
| `game_data.go` | 玩家位置/背包/计分板等游戏数据读取 | `PlayerPos`, `GetPlayerPos`, `GetScore`, `GetPlayerInventory` | 520 |
| `heartbeat.go` | 心跳数据结构与上报骨架（原作者上报已在 `main.go:5001` 注释移除） | `HeartbeatRequest`, `sendHeartbeat`, `recordHeartbeatIncrement` | 250 |
| `integrity.go` | 官方签名证书 SHA-256 白名单与完整性判定 | `IntegrityResult`, `SetApkSignatureHash`, `hashInWhitelist` | 257 |
| `integrity_gate.go` | 完整性闸门中间件：判定被篡改则所有请求返回拦截页 | `yyIntegrityGate`(117), `yyIntegritySet` | 283 |
| `itemmaker_handler.go` | 生成含木桶 + 物品的 `.mcstructure` | `ItemMakerItem`, `handleItemMakerGenerate` | 171 |
| `launch_app_stub.go` | 桌面构建的 `launchAndroidApp` 空实现 | `launchAndroidApp` | 6 |
| `libmain.go` | Android JNI 入口：setDataDir/GoMain/账号进程 startCore，最终调 StartServer | `Java_..._GoMain`(133), `startCore`(212) | 237 |
| `lua_api.go` | Lua 全局 API 注册总表（game/system/api/plugin/util/bot/building/media…）与插件加载入口 | `registerAllLuaAPI`, `LoadLuaPlugin`(926) | 929 |
| `lua_api_cross.go` | 跨插件调用：复用常驻 Runtime，不重跑源码 | `registerCrossPluginAPI`, `rememberPluginSource` | 177 |
| `lua_bot.go` | Lua 侧 `bot.*` 只读舰队查询 | `registerBotAPI`, `botName` | 107 |
| `lua_botctx.go` | Lua 执行上下文：绑定 bot、执行超时、父 ctx 继承 | `luaBotCtx`, `selectBot`, `cancelLuaCtx` | 85 |
| `lua_building.go` | Lua `building.*`：结构解析/分类纯函数 | `registerBuildingAPI` | 77 |
| `lua_events.go` | Lua `events.*`、玩家进/退 diff、包订阅注册 | `registerEventAPI`, `registerPacketSub`, `diffPlayerEvents` | 364 |
| `lua_game_legacy.go` | 旧版 game API 名到新实现的兼容接线 | `registerLegacyGameAPI`, `yyLegacyImportData` | 298 |
| `lua_http.go` | 插件 HTTP 客户端底层（10MB 响应上限） | `httpGetText`, `httpPostText`, `readHTTPBody` | 135 |
| `lua_http_api.go` | Lua `http.*` 表（headers/timeout 等选项） | `registerHTTPAPI`, `pushHTTPResult` | 116 |
| `lua_media.go` | Lua `media.*`：图片转 ASCII、MIDI 解析、音高换算 | `registerMediaAPI` | 84 |
| `lua_player.go` | Lua `player.*` 查询玩家信息 | `registerPlayerAPI`, `playerInfoToTable` | 77 |
| `lua_runtime.go` | 执行一段 Lua 片段并取返回值（单测/简单调用） | `runLuaSnippet` | 21 |
| `lua_system.go` | Lua `system.*`：调 Android 系统能力（call-return 桥） | `registerSystemAPI`, `callSystemToLua` | 94 |
| `lua_util.go` | Lua `util.*`：定时任务（schedule/setTimeout）与插件任务清理 | `schedJob`, `newSchedJob`, `RemovePluginSchedules` | 328 |
| `lua_world.go` | Lua `world.*` 方块查询（需已连接） | `registerWorldAPI` | 15 |
| `main.go` | 进程装配与 HTTP 总路由：183 处 mux 注册 + 任务/插件/机器人 handler + StartServer | `StartServer`(4922), `handleBotConnect`(1039), `handlePluginRun`(669) | 5326 |
| `main_native.go` | 非 Android 入口 `main()`：读 YY_DATA_DIR/YY_PORT 后调 StartServer | `main`(11) | 26 |
| `map_player.go` | 地图画（物品展示框）扫描与像素映射 | `MapPlayer`, `ScanItemFrames`, `parseFramesFromNBT` | 590 |
| `marquee.go` | 7x7 ASCII 大字滚动字幕生成 | `MarqueeConfig`, `buildMarqueeCells` | 346 |
| `mcfunction.go` | mcfunction 解析/分组/执行计划 | `ParsedFile`, `parseMcfunction`, `parseGroupConfig` | 683 |
| `mcort_brand.go` | MCORT 品牌横幅与署名（硬编码身份层） | `yyBrandBanner`, `yyBrandSignature` | 44 |
| `multi_bot.go` | 多机器人并发放置 NBT 方块（导入加速） | `MultiBotManager`, `placeOneNBTBlock`, `buildNormalUnitTask` | 1235 |
| `music_player.go` | 音乐播放器与播放列表（音符调度） | `MusicPlayer`, `PlaylistManager`, `NewMusicPlayer` | 766 |
| `nbt_world_api.go` | 方块 NBT 读写统一入口（Lua `game.*` 与 AI `game_call` 共用） | `GetBlockData`, `GetBlockNBT`, `yyWithRegionReadSource` | 412 |
| `notification.go` | 服务端 `pending_messages` 推送的分发与游戏内执行 | `dispatchNewPushMessages`, `executeInGame`, `resolveDisplayMode` | 210 |
| `packet_hub.go` | 通用数据包订阅（插件 `events.subscribe` + AI `listen_packet`） | `packetHubSubscribe`, `yyPublishPacketSubscribers` | 230 |
| `pathfinder.go` | A* 寻路与移动控制（ControlState 输入位） | `AStar`, `NewPathfinder`, `MoveVector` | 905 |
| `player_info.go` | 玩家信息管理：皮肤/UUID/死亡宽限/包处理 | `PlayerInfo`, `NewPlayerInfoManager`, `MarkDeath` | 841 |
| `plugin_backup.go` | 插件全量备份/还原（保留空目录与私有文件） | `ExportPackage`, `importFullPluginBackup` | 304 |
| `plugin_config.go` | 插件配置与数据目录路径约定 | `ConfigPath`, `DataDir`, `ReadConfig` | 108 |
| `plugin_controller.go` | 插件生命周期控制器：扫描/运行/停止/重载/依赖 | `PluginController`, `Run`(313), `ScanPlugins`(165) | 614 |
| `plugin_defs.go` | `PluginDef` 定义；内置插件表当前为空 | `PluginDef` | 18 |
| `plugin_edit.go` | 新建插件（Lua 模板）与插件文件读写 | `CreatePlugin`, `ReadPluginFile`, `CodeFileName` | 210 |
| `plugin_pack.go` | 插件包导入/导出 zip，含 zipSlip 路径穿越防护 | `ImportPackage`, `zipSlipSafe`, `ExportMarketPackage` | 373 |
| `plugin_paths.go` | 插件代码目录归属（CodeDir）与松散文件隔离 | `CodeDir`, `isolatedCodeDir` | 84 |
| `plugin_scan.go` | 扫描插件目录、读 manifest 与文档，构造 PluginDef | `BuildDefFromFile`, `readPluginManifest` | 127 |
| `preview_handler.go` | 建筑预览：扫描区域、生成预览、状态查询 | `ScanRequest`, `handlePreviewScan`, `handlePreviewGenerate` | 732 |
| `protect_secrets.go` | 敏感常量（URL/密钥）收敛与运行时解码 | `yySecLoad`, `yySecDecode` | 154 |
| `protect_selfcheck.go` | 启动自检：封签校验、TracerPid、构建指纹 | `yySelfCheck`, `yyTracerPID`, `yySealVerify` | 314 |
| `seedv2_biome.go` | 种子地图 v2：生物群系生成（Voronoi/噪声） | `yyV2BiomeGen`, `BiomeAtBlock` | 471 |
| `seedv2_btree.go` | v2 结构 BTree 索引加载与查询 | `yyV2BTreeLoad`, `yyV2BTreeByName` | 236 |
| `seedv2_icons.go` | v2 图标图元绘制（矩形/圆/环/线/多边形） | `yyV2IconPrim`, `yyV2Circle` | 546 |
| `seedv2_place.go` | v2 结构放置位置推算（版本区间/元数据） | `yyV2PlaceConfigFor`, `yyV2PlaceRegionSeed` | 806 |
| `seedv2_rand.go` | v2 随机数实现（Xoroshiro/Perlin，含状态序列化） | `yyV2RNG`, `yyV2NewRNG`, `NextInt` | 845 |
| `seedv2_render.go` | v2 瓦片 PNG 渲染 | `yyV2RenderTilePNG`, `yyV2RenderNormalizeOptions` | 619 |
| `seedv2_render_shim.go` | 渲染层与图标层之间的过渡垫片 | `yyV2RenderDefaultIconIDs` | 43 |
| `seedv2_types.go` | v2 版本/维度/结构信息等基础类型与归一化 | `yyV2StructureInfo`, `yyV2ParseVersion` | 205 |
| `sign_place.go` | 告示牌放置与 GameInterface 载入（block_states 旧名映射） | `getGameInterface`, `loadGameInterface` | 783 |
| `subchunk.go` | 子区块方块存储解析（GetBlock / 调色板） | `SubChunk`, `ChunkColumn`, `GetBlock` | 529 |
| `system_stub.go` | 桌面 `callAndroidSystem` 空实现 | `callAndroidSystem` | 11 |
| `task_dispatch.go` | 长任务分发：导入/修复/导出/地图画/皮肤，含断点续跑 | `dispatchTask`, `runImportTask`(226), `runMapArtTask`(1486) | 2818 |
| `tickingarea_confirmation.go` | 常加载区变更确认与维度键 | `yyConfirmTickingChange` | 45 |
| `tickingarea_list.go` | 解析 `tickingarea list` 输出 | `yyParseTickingAreas` | 62 |
| `tickingarea_preload.go` | 常加载区预加载命令与回调 | `TickingAreaAdd`, `yyTickingCallbacks` | 346 |
| `tickingarea_recovery.go` | 启动/断线后恢复常加载区 | `yyRecoverTickingAreas` | 40 |
| `vps_handlers.go` | 非 Android 下接管「重启进程」handler | `init` | 19 |
| `web_api.go` | Lua `web.*`：page/serve/action/emit 注册到全局表 | `registerWebAPI`, `callWebAction` | 152 |
| `web_http.go` | 插件网页总入口 `/web/*`（门户/页面/动作/SSE/静态） | `handleWeb`, `webGate`, `handleWebAPI` | 416 |
| `web_registry.go` | 插件网页注册表（页面/控件/动作/订阅者） | `registerWebPage`, `pushWebEvent`, `RemovePluginWeb` | 189 |
| `wordbank.go` | 词库文件加载与命令执行 | `loadWordBank`, `execCommand`, `argTypeFromWord` | 95 |
| `youyuan_account.go` | 账号的 bot 视图与动作封装 | `yyAccountBot`, `yyAccountAction` | 63 |
| `youyuan_account_bridge.go` | 账号桥：规则校验、按规则把调用转发到子账号 | `yyAccountBridgeRoutes`(283), `yyBridgeAllowed` | 375 |
| `youyuan_account_game_identity.go` | 主账号与游戏 ID 的绑定记忆 | `yyGameCredentialBinding`, `yyMainGameIdentityView` | 67 |
| `youyuan_account_pool.go` | 账号池：租约、子进程身份、池路由 | `yyPoolLease`, `yyAccountPoolRoutes`(644) | 796 |
| `youyuan_account_profiles.go` | 账号资料（头像/昵称）读写与校验 | `yyAccountProfile`, `yySaveAccountProfile` | 133 |
| `youyuan_account_skin.go` | 账号皮肤：云端拉取/上传/代理 | `yyAccountSkinDeviceToken`, `yyAccountSkinCall` | 515 |
| `youyuan_account_skin_prime.go` | 皮肤预热与缓存命中判定 | `yyAccountSkinPrimeDue`, `yyAccountSkinCached` | 140 |
| `youyuan_accounts.go` | 托管账号存储与分配（目录/地址/保存） | `yyManagedAccount`, `yyAccountsRoutes`(293) | 709 |
| `youyuan_accounts_android.go` | Android 下账号子进程启停与 IPC | `yyAccountProcessStart`, `yyAccountProcessCall` | 28 |
| `youyuan_accounts_desktop.go` | 桌面下账号进程启停（无进程时降级） | `yyAccountProcessStart` | 61 |
| `youyuan_accounts_unix.go` | 隐藏账号窗口的 Unix 实现 | `yyHideAccountWindow` | 7 |
| `youyuan_accounts_windows.go` | 隐藏账号窗口的 Windows 实现 | `yyHideAccountWindow` | 10 |
| `youyuan_admission.go` | 远端准入状态机（握手/配额/快照） | `yyAdmissionState`, `step` | 128 |
| `youyuan_admission_roster.go` | 准入白名单名单枚举 | `yyAdmissionListNames` | 61 |
| `youyuan_agent.go` | AI Agent 工作区总控：事件、MCP 服务器、会话保存 | `yyWorkspace`, `yyAgentRoutes`(1250), `event` | 1753 |
| `youyuan_agent_attachments.go` | Agent 附件文本抽取与分页 | `yyAttachmentText`, `yyAgentAttachmentPage` | 48 |
| `youyuan_agent_binding.go` | Lua 插件与 Agent 工作区绑定/发布 | `yyBindLuaPlugin`, `yyPublishBoundLua` | 197 |
| `youyuan_agent_budget.go` | 单次 Agent 运行的轮次/工具预算 | `yyAgentRunBudget`, `yyEffectiveRunBudget` | 78 |
| `youyuan_agent_cleanup.go` | 工作区归档与迁移、维护路由 | `yyArchiveWorkspace`, `yyAgentMaintenanceRoutes` | 122 |
| `youyuan_agent_cloud.go` | Agent 云端能力开关与密钥 | `yyAgentCloudKey`, `yyCheckCloudCapability` | 81 |
| `youyuan_agent_context.go` | 上下文装配：历史摘要、绑定文本、技能文本 | `summarizeHistory`, `yyAgentContextTurns` | 145 |
| `youyuan_agent_fork.go` | 工作区分叉路由 | `yyAgentForkRoutes` | 152 |
| `youyuan_agent_load.go` | 工作区插件装载与 Agent 槽位申请 | `yyManageWorkspacePlugin`, `yyTryAgentSlot` | 89 |
| `youyuan_agent_mcp_catalog.go` | Agent 可见的 MCP 工具目录/描述分页 | `yyMCPToolDirectory`, `yyMCPToolPage` | 64 |
| `youyuan_agent_preferences.go` | Agent 默认技能/工具集与偏好 | `yyDefaultAgentSkills`, `yyWorkspaceToolDefs` | 228 |
| `youyuan_agent_sessions.go` | Agent 会话路由 | `yyAgentSessionRoutes` | 150 |
| `youyuan_agent_skill_search.go` | 工作区技能检索 | `yySearchWorkspaceSkills` | 63 |
| `youyuan_ai_models.go` | AI 角色（role）视图与保存、从 QQ 配置种子化 | `yyAIRolesView`, `yyAISaveRoles` | 233 |
| `youyuan_api_contract.go` | 关键 API 的「契约说明」文本（给 AI/MCP 的用法提示） | `yyAPIContract` | 29 |
| `youyuan_appearance.go` | 外观配置（主题/皮肤）与路由 | `yyAppearance`, `yyAppearanceRoutes` | 139 |
| `youyuan_artifact.go` | 下载外部编译产物并按 hash 校验后走原生插件通道 | `yyFetchArtifact` | 24 |
| `youyuan_benchmark.go` | 性能基准接口（中位数统计） | `yyRunBenchmark`, `yyBenchmarkRoutes` | 159 |
| `youyuan_block_video.go` | 方块视频（逐帧方块成像）任务与路由 | `yyBlockVideoConfig`, `yyBlockVideoRoutes` | 298 |
| `youyuan_bot_mode.go` | 机器人维护模式：会话保活/看门狗/飞行能力包 | `yyBotMaintainer`, `startWatchdog` | 198 |
| `youyuan_bridge_api.go` | 通用桥：规则、事件、发布（插件 ↔ 外部） | `yyBridgeAction`, `yyBridgeFire`, `yyBridgeRoutes`(332) | 700 |
| `youyuan_building_jobs.go` | 建筑任务（导入区域校验、异步 job） | `yyBuildingJobCall`, `yyValidateImportRegionParams` | 141 |
| `youyuan_capabilities.go` | 能力清单（语言/模块）与查询 | `yyCapabilityList`, `yyHasCapability` | 73 |
| `youyuan_capture.go` | 抓包：包名/模式/容量配置与 Lua sense 路由 | `yyPacketFilter`, `yyCaptureConfig`, `yySenseRoutes`(496) | 704 |
| `youyuan_capture_admission.go` | 抓包准入（并发/容量）应用 | `yyCaptureApplyAdmission` | 63 |
| `youyuan_capture_behavior.go` | 抓包行为模式（游戏模式相关命令） | `yyCaptureBehaviorCommands` | 57 |
| `youyuan_capture_landing.go` | 抓包落点校验（目标位置/维度） | `yyCaptureLanding`, `yyCaptureLandingMatches` | 227 |
| `youyuan_capture_plan.go` | 抓包编队计划（机器人分配/常驻数） | `yyCaptureFleetStep`, `yyCapturePlan` | 214 |
| `youyuan_capture_query.go` | 抓包结果行查询 | `yyCaptureQueryRows` | 55 |
| `youyuan_capture_reconcile.go` | 全局抓包对账：运行态与名册核对 | `yyGlobalCaptureRun`, `yyCaptureRoster` | 336 |
| `youyuan_capture_runtime.go` | 抓包运行时：启动、身份审计、OP 授予 | `yyCaptureLaunch`, `yyCaptureGrantOP` | 518 |
| `youyuan_catalog.go` | API 名 → 路径清单表（MCP 工具目录真源） | `yyAPIRoute`, `yyAPIRoutes` | 317 |
| `youyuan_chain_api.go` | 命令链插件调用入口与结果包装 | `yyChainPluginCall`(73), `yyChainRowsFor` | 317 |
| `youyuan_chain_buffered.go` | 链节点缓冲执行与激活键校验 | `yyChainActivationKey`, `yyChainExecuteBuffered` | 201 |
| `youyuan_chain_control.go` | 命令链图执行器（sleep/查找/切片/图遍历） | `yyChainExecuteGraph`(174), `yyParseChainControl` | 353 |
| `youyuan_chain_fast_module.go` | 快速链模块通道（合并发送/结果回写） | `yyChainFastGate`, `sendChainFastModule` | 279 |
| `youyuan_chain_gate.go` | 链闸门：只读判定、路径键、插件调用 | `yyChainGateRows`, `yyChainInvokePlugin` | 188 |
| `youyuan_chain_segments.go` | 链片段执行 | `runSegments`, `runSegmentsWith` | 195 |
| `youyuan_chain_watch.go` | 链资源文件变更监视与热重载 | `yyStartChainWatcher`, `yyReloadChainResource` | 76 |
| `youyuan_command_async.go` | 异步命令票据（提交-回收） | `yyAsyncCommand`, `yyAsyncCommandCall` | 169 |
| `youyuan_command_batch.go` | 命令批处理：窗口执行、回复等待、默认选项 | `yyCommandBatch`, `yyAwaitCommandReply`, `yyExecuteWindowed` | 252 |
| `youyuan_command_batch_gate.go` | 批处理并发闸门 | `acquireControlBatch`, `writeControlBatch` | 29 |
| `youyuan_command_burst.go` | 突发命令窗口（限速补偿） | `yyCommandBurst`, `yyCommandBurstWindow` | 146 |
| `youyuan_command_stream.go` | 命令流式执行（滑动窗口） | `yyCommandStream`, `yyExecuteSliding` | 254 |
| `youyuan_command_structure.go` | 命令结构化解析（自然排序/文本帧/放置文本） | `yyCommandStructure`, `yyTextFrames` | 462 |
| `youyuan_commands.go` | 命令链定义与解析：`yyChain`/`yyChainNode`、控制台拆分、保存 | `yyChain`, `yyParseChains`, `yyCommandRoutes`(721) | 1059 |
| `youyuan_connection.go` | 连接凭据保存、服务器地址归一化、连接尝试闸门、暂停标记 | `yySaveConnectionCredentials`, `yyBeginConnectionAttempt`(92) | 272 |
| `youyuan_connection_diagnostics.go` | 连接错误分类与用户提示/重试延迟 | `yyConnectionErrorKind`, `yyConnectionActionHint` | 62 |
| `youyuan_connection_report.go` | 连接质量采样与丢包/延迟报告 | `yyNoteConnectionSample`, `yyDropReasonKind` | 585 |
| `youyuan_console_auth.go` | 控制台会话 token 中间件（防同机网页直连） | `yyConsoleAuth`(87), `yyConsoleRequestAllowed` | 129 |
| `youyuan_conversion_options.go` | 结构转换选项（方块 JSON / 命令生成） | `yyConvertControl`, `yyStructureCommands` | 128 |
| `youyuan_converters.go` | 结构格式互转（bdx/schem/nbt）与路由 | `yyStructureEncode`, `yyConverterRoutes`(142) | 347 |
| `youyuan_death_rpc.go` | 死亡事件 RPC 与事件匹配 | `yyDeathRPC`, `yySenseEventMatches` | 62 |
| `youyuan_doc_catalog.go` | 文档目录：包/章节结构化文档 | `yyDocPacket`, `yyDocSection` | 119 |
| `youyuan_docs.go` | `/api/docs` 文档路由 | `yyDocumentationRoutes` | 69 |
| `youyuan_download_headers.go` | 下载类响应的 CORS 与暴露头 | `yyDownloadCORS` | 10 |
| `youyuan_egress.go` | 出口代理配置（模式/校验/保存）与拨号器 | `yyEgressProxyConfig`, `yyEgressDialer` | 340 |
| `youyuan_egress_socks.go` | 手写 SOCKS5 拨号器（含 UDP） | `yySOCKS5Dialer`, `yySOCKSUDPConn` | 279 |
| `youyuan_entity_observations.go` | 实体观测缓存（位置/属性/身份，含过期判定） | `yyObservedIdentity`, `yyObservationAge` | 473 |
| `youyuan_export_center.go` | 导出中心：插件产物记录、保留、下发 | `yyPluginExport`, `yyExportCenterRoutes`(216) | 329 |
| `youyuan_file_upload_sessions.go` | 分片上传会话（票据/落盘/校验） | `yyUploadTicket`, `yyUploadStore` | 452 |
| `youyuan_fleet_panel.go` | 舰队连接面板文案渲染 | `yyFleetConnectionPanel` | 23 |
| `youyuan_game_auth.go` | 游戏登录态路由与来源校验 | `yyGameAuthOrigin`, `yyGameAuthRoutes` | 63 |
| `youyuan_game_dispatch.go` | `game.*` 方法名规范化与公共调用入口 | `yyCanonicalGameMethod`, `yyPublicGameCall` | 62 |
| `youyuan_global_capture.go` | 全局抓包配置：范围/维度/半径/告警 | `yyGlobalCaptureConfig`, `yyGlobalCaptureRoutes`(346) | 445 |
| `youyuan_host_files.go` | 宿主文件浏览路由 | `yyHostFilesRoutes` | 103 |
| `youyuan_hud_scoreboard.go` | HUD 计分板渲染与归属清理 | `yyHudEntry`, `yyHudCall` | 271 |
| `youyuan_identity.go` | 平台身份：绑定/校验/清除与 HTTP 错误映射 | `yyPlatformIdentity`, `yyIdentityRoutes`(107) | 259 |
| `youyuan_import.go` | 插件解包与运行时导入 | `yyUnpackPlugin`, `yyRuntimeImport` | 133 |
| `youyuan_import_checkpoint.go` | 断点接管（保留已恢复位置，避免回退到原点重清） | `yyAdoptImportCheckpoint` | 16 |
| `youyuan_import_probe.go` | 用 `testforblock` 回执判断区块是否已加载 | `yyImportChunkProbeLoaded` | 31 |
| `youyuan_legacy_files.go` | 旧版文件路径兼容与绑定 | `yyLegacyFilePath`, `yyLegacyFileBindings` | 82 |
| `youyuan_library.go` | 库下载代理（140s 超时） | `yyLibraryProxy` | 50 |
| `youyuan_linux_library.go` | Linux 本地库条目与列表 | `yyLinuxLibraryItem`, `yyLocalLibrary` | 136 |
| `youyuan_locator.go` | 实体定位观测快照 | `yyLocatorObservation`, `yyLocatorSnapshot` | 41 |
| `youyuan_log_channel.go` | 收包日志通道与日志统计 | `yyReceiveLog`, `yyDisplayLogStats` | 21 |
| `youyuan_lua_wait.go` | Lua 等待原语（等待键 / 泛型等待） | `yyLuaWait`, `yyLuaWaitValue` | 66 |
| `youyuan_mail.go` | 注册邮件配置与发送 | `yyMailConfig`, `yySendRegistrationMail` | 152 |
| `youyuan_map_video.go` | 地图画视频会话（帧差/滤镜/画布） | `mapVideoSession`, `mapFrameDifference` | 292 |
| `youyuan_market.go` | 插件市场：版本/订单/仓库读写与路由 | `yyMarketVersion`, `yyMarketStore`, `yyMarketRoutes`(383) | 1150 |
| `youyuan_mcp_links.go` | MCP 链接（工具 ↔ API）读取与校验 | `yyMCPLink`, `yyMCPReadTool` | 201 |
| `youyuan_mcp_resources.go` | MCP resources 列表与读取 | `yyMCPResources`, `yyMCPReadResource` | 140 |
| `youyuan_mcp_scope.go` | MCP 连接作用域：授权/白名单/快照 | `yyMCPConn`, `yyMCPScopeRoutes`(643) | 747 |
| `youyuan_mcp_telemetry.go` | MCP 调用轨迹记录 | `yyMCPTraceRecord`, `yyMCPTraceRoutes` | 104 |
| `youyuan_model_context.go` | 上下文窗口估算与压缩阈值 | `yyEstimateContextTokens`, `contextWindow` | 84 |
| `youyuan_module_combat.go` | 模块战斗事件（UID/身份/位置） | `yyModuleCombatEvents`, `yyModuleIdentity` | 244 |
| `youyuan_multi_budget.go` | 多机器人导入的并发预算 | `yyMultiBotImportBudget`, `wait` | 95 |
| `youyuan_multi_checkpoint.go` | 多机器人导入断点（worker 选择 / 源哈希） | `yySelectImportWorkers`, `yyLoadMultiResume` | 155 |
| `youyuan_native_desktop.go` | 桌面原生插件宿主进程（启动/请求/事件） | `yyNativeProcess`, `yyNativeStart` | 180 |
| `youyuan_native_library.go` | 原生插件库校验 | `yyValidateNative` | 32 |
| `youyuan_nbt_operations.go` | NBT 作业：归一化 / 有界整数 / 容器 NBT | `yyNBTJob`, `yyNormalizeNBT` | 523 |
| `youyuan_notices.go` | 公告与任务提示快照、文案 | `yyNoticeConfig`, `yyNoticeSnapshot` | 135 |
| `youyuan_observation_limits.go` | 观测容量设置校验与加载 | `yyObservationLimits`, `yyLoadObservationSettings` | 86 |
| `youyuan_observation_summary.go` | 观测摘要（按类型/实体聚合） | `yySummaryOptions`, `yyObservationSummary` | 142 |
| `youyuan_observations.go` | 聊天记录/在线采样与观测路由 | `yyChatRecord`, `yyObservationRoutes`(56) | 138 |
| `youyuan_observed_events.go` | 可观测事件目录与定义解析 | `yyEventDefinition`, `yyObservedCatalog` | 135 |
| `youyuan_observer_bridge.go` | 观测桥：外部源接入 / 批次入库 / 调用 | `yyObserverBridge`, `yyObserverBridgeRoutes`(236) | 281 |
| `youyuan_observer_dimension.go` | 观测者跨维度传送与状态 | `yyObserverDimensionOptions`, `yyObserverDimensionTransfer` | 170 |
| `youyuan_observer_handoff.go` | 观测交接：账号选择 / 观测计划 / 机器人名单 | `yyObservationPlan`, `yyObserverAccountCall` | 299 |
| `youyuan_observer_input.go` | 观测者自身状态与输入包注入 | `yyObserverInputSession`, `yyObserverAttachSelf` | 184 |
| `youyuan_observer_patrol.go` | 巡逻观测：配置、命令、状态 | `yyPatrolConfig`, `yyPatrolCommand` | 399 |
| `youyuan_observer_rpc.go` | 观测 RPC 等待器（请求-回包配对） | `yyObserverRPCWaiter`, `yyObserverRPCCall` | 225 |
| `youyuan_observer_watch.go` | 定点看守：相机命令、命名、状态 | `yyWatchConfig`, `yyWatchCameraCommand` | 559 |
| `youyuan_packet_api.go` | 构造并发送任意包（字段填充/校验） | `yyMakePacket`, `yySendPacket` | 400 |
| `youyuan_packet_ingress.go` | 入站包分流：观测/登录/投递选择 | `yyPacketIngress`, `yySelectIncomingPacket` | 128 |
| `youyuan_permission_probe.go` | 命令权限回执确认 | `confirmCommandPermission` | 25 |
| `youyuan_physical_actions.go` | 物理动作/输入注入（挥臂、攻击等） | `yyPhysicalAction`, `yyPhysicalInput` | 197 |
| `youyuan_player_experience.go` | 玩家体验控制（模式/权限/命令下发） | `yyPlayerExperience`, `yyExperienceCommand` | 306 |
| `youyuan_plugin_commands.go` | 插件命令调用：选项合并、默认值、路由 | `yyPluginCommandCall`(119), `yyMergeCommandOptions` | 340 |
| `youyuan_plugin_mcp.go` | 插件向 MCP 暴露工具 | `yyPluginMCP` | 97 |
| `youyuan_plugin_outputs.go` | 插件产物列表与下载路由 | `yyListOutputs`, `yyPluginOutputRoutes`(124) | 179 |
| `youyuan_pool_identity.go` | 账号池身份与转发头 | `yyPoolActor`, `yyPoolForwardHeaders` | 118 |
| `youyuan_precision_linux.go` | Linux/Android 高精度睡眠（hrtimer，免忙轮询） | `yyPrecisionWait` | 21 |
| `youyuan_precision_other.go` | 其它平台高精度睡眠降级实现 | `yyPrecisionWait` | 15 |
| `youyuan_primary_account.go` | 主账号保存尝试（多进程竞争处理） | `yyTryPrimaryAccountSave` | 46 |
| `youyuan_product.go` | 版本号/更新清单/公告/独立版信息 | `yyAppVersion`(14), `yyCheckRelease` | 69 |
| `youyuan_profiler.go` | Lua 性能采样与 profiler 路由 | `yyLuaStats`, `yyProfilerRoute` | 70 |
| `youyuan_protocol_samples.go` | 协议元数据采样 | `yyProtocolMetadataSampler` | 57 |
| `youyuan_provider_routing.go` | Provider URL 与市场凭据路由 | `yyProviderURL`, `yyMarketCredential` | 41 |
| `youyuan_public_urls.go` | 由 wss 地址派生公开 URL（带 device 参数） | `yyPublicURLs` | 19 |
| `youyuan_publish_manifest.go` | 工作区发布清单 | `yyWorkspaceRuntimeManifest` | 31 |
| `youyuan_qq.go` | QQ 机器人总控：配置、连接循环、快照、路由注册 | `yyQQConfig`, `yyRegisterQQ`, `yyQQConnectLoop` | 969 |
| `youyuan_qq_actions.go` | QQ 动作策略：频率/权限/成功判定与日志 | `yyQQActionPolicy`, `yyQQActionRecord` | 479 |
| `youyuan_qq_affinity.go` | 好感度/情绪与互动预算 | `yyAffState`, `yyAffMoodOf`, `yyAffChatBudget` | 637 |
| `youyuan_qq_ai.go` | QQ 场景的 AI 会话与历史存储 | `yyQQAISessionKey`, `yyQQStoreAIHistory` | 694 |
| `youyuan_qq_approval.go` | 审批单：创建/过期/裁剪/保存 | `yyApprItem`, `yyApprNewIDLocked` | 630 |
| `youyuan_qq_assistant.go` | QQ 助手配置与技能开关、玩家统计 | `yyQQAssistantConfig`, `yyQQAssistantRoutes`(244) | 1165 |
| `youyuan_qq_caps.go` | QQ 能力等级与限额（等级 → 上限） | `yyQQCapability`, `yyQQCapEffective` | 584 |
| `youyuan_qq_card.go` | QQ 玩家卡片/统计摘要渲染 | `yyQQPlayerCard`, `yyQQStatsFor` | 233 |
| `youyuan_qq_claim.go` | QQ 认领（claim）存储与消息 | `yyQQClaimStore`, `yyQQClaimMessage` | 203 |
| `youyuan_qq_claim_qr.go` | 自实现 QR 码生成（GF 运算 / 码字 / 掩码） | `yyQRSpec`, `yyQRDataCodewords` | 613 |
| `youyuan_qq_commands.go` | QQ 命令文本拼装 | `yyQQCommandText` | 57 |
| `youyuan_qq_companion.go` | 陪伴会话：窗口/初始等待/存活判定 | `yyQQCompanionSession`, `yyQQCompanionBegin` | 225 |
| `youyuan_qq_companion_aimode.go` | 陪伴 AI 模式：任务、间隔、夜间系数 | `yyQQAIWithTask`, `yyQQAIMode` | 379 |
| `youyuan_qq_context.go` | QQ 消息上下文：键/稳定 ID/回复 ID | `yyQQGroupMessage`, `yyQQMessageKey` | 288 |
| `youyuan_qq_forward.go` | 合并转发节点构造与降级 | `yyForwardNode`, `yyForwardBuild` | 137 |
| `youyuan_qq_gateway.go` | QQ 官方网关连接与会话 | `yyQQOfficialConnect`, `yyQQGatewaySession` | 237 |
| `youyuan_qq_groups.go` | 群配置校验与回复群选择 | `yyQQGroup`, `yyQQReplyGroup` | 209 |
| `youyuan_qq_help.go` | QQ 帮助条目注册与排序 | `yyQQHelpEntry`, `yyQQHelpRegistry` | 322 |
| `youyuan_qq_identity.go` | QQ 管理员校验与成员解析 | `yyQQValidateAdminIDs`, `yyQQResolveMember` | 109 |
| `youyuan_qq_image.go` | QQ 图片能力：端点/头像/配置 | `yyQQImageConfig`, `yyQQAvatarURL` | 618 |
| `youyuan_qq_image_queue.go` | 图片生成任务队列（提交/循环/完成） | `yyImgQueue`, `Submit`, `runTask` | 294 |
| `youyuan_qq_inbound.go` | 入站消息解析（CQ 码 / 段 / 事件） | `yyQQParseInbound`, `yyQQParseCQSegments` | 522 |
| `youyuan_qq_knowledge.go` | QQ 知识库：分类/条目/导入导出 | `yyKBCategory`, `yyKBItem`, `yyKBInit` | 704 |
| `youyuan_qq_level.go` | QQ 等级系统：经验/冷却/上限 | `yyLevelConfig`, `yyLevelNeedOf` | 786 |
| `youyuan_qq_liveness.go` | QQ 在线探活与快照 | `yyQQLiveness`, `yyQQLiveProbeOnce` | 187 |
| `youyuan_qq_login.go` | QQ 登录（二维码 / WS 监听 / 登录态） | `yyQQQRCodeView`, `yyQQLoginSnapshot` | 290 |
| `youyuan_qq_member.go` | 群成员档案：初始化/表情缓存/消息钩子 | `yyQQMember`, `yyMemberOnMessage` | 1005 |
| `youyuan_qq_memory.go` | QQ 记忆：TTL/维护/上下文预算/嵌入档位 | `yyQQMemoryConfig`, `yyQQMemoryTTL` | 577 |
| `youyuan_qq_messages_multi.go` | 多账号消息汇总行 | `yyQQMessageSummary`, `yyQQMessageSummaryRoutes` | 64 |
| `youyuan_qq_moderation.go` | 内容审核规则与命中处理 | `yyQQModerationRule`, `yyQQModerationMatchLocked` | 493 |
| `youyuan_qq_moderation_nl.go` | 自然语言封禁与禁忌词增删 | `yyQQBanExtract`, `yyQQNaturalBan` | 261 |
| `youyuan_qq_official.go` | 官方 API：access_token / 回复窗口 / 发送 | `yyQQAccessToken`, `yyQQSendOfficialWindow` | 213 |
| `youyuan_qq_online_sync.go` | 在线状态同步循环 | `yyOnlineSyncStart`, `yyOnlineSyncOnce` | 76 |
| `youyuan_qq_persona.go` | 人格/文风配置与样例 | `yyQQPersona`, `yyStyleConfig` | 663 |
| `youyuan_qq_poke.go` | 拍一拍（群/友）与缺失参数提示 | `yyPokeGroup`, `yyPokeFriend` | 140 |
| `youyuan_qq_proactive.go` | 主动发言：权重/配置/群状态 | `yyProactiveConfig`, `yyProactiveNormalize` | 558 |
| `youyuan_qq_proactive_wire.go` | 主动发言调度接线（扫描/发言） | `yyProactiveStart`, `yyProactiveSweep` | 121 |
| `youyuan_qq_quote.go` | 引用回复：CQ 码剥离/缓存/存储 | `yyQQQuoteEntry`, `yyQQQuoteStore` | 336 |
| `youyuan_qq_rag.go` | QQ RAG：分块 / 索引 / 检索设置 | `yyRagDoc`, `yyRagChunk`, `yyRagInit` | 1371 |
| `youyuan_qq_relay.go` | QQ 群服互通中继（关键词/间隔/上限） | `yyQQRelayItem`, `yyQQRelayClamp` | 657 |
| `youyuan_qq_sandbox.go` | QQ 沙箱 handler | `yyQQSandboxHandler` | 70 |
| `youyuan_qq_selection.go` | 群/游戏选择与展示名 | `yyQQGroupChoice`, `yyQQSelection` | 161 |
| `youyuan_qq_serverinfo.go` | 服务器信息面板：机器人档案/在线集合/反查绑定 | `yySrvInfoBotProfile`, `yySrvInfoOnlineSet` | 1023 |
| `youyuan_qq_skill.go` | 游戏技能：内置技能、合并、日志 | `yyGameSkill`, `yyGameSkillBuiltins` | 1088 |
| `youyuan_qq_skills.go` | QQ 技能定义与启用判定 | `yyQQSkillDef`, `yyQQSkillEnabled` | 618 |
| `youyuan_qq_social_tools.go` | 社交工具集（贴纸/多目标发送/参数解析） | `yyQQSocialTools`, `yyStickerSendMany` | 669 |
| `youyuan_qq_sticker.go` | 贴纸发送与 URL 解析 | `yyStickerSend`, `yyQQCmdSticker` | 94 |
| `youyuan_qq_summary.go` | 群聊摘要命令与统计 | `yyQQCmdSummary`, `yySummaryTop` | 114 |
| `youyuan_qq_toolbox.go` | QQ 工具开关与分组视图 | `yyQQToolDef`, `yyQQToolRoutes` | 266 |
| `youyuan_qq_triggers.go` | QQ 触发词：匹配/渲染/等级限制 | `yyQQTrigger`, `yyQQTriggerMatch` | 692 |
| `youyuan_qq_usage.go` | 用量统计（ticker / 成本 / 聚合） | `yyUsageRecord`, `yyUsageAdd`, `yyUsageCost` | 733 |
| `youyuan_qq_vision.go` | 视觉理解：URL 收集/文件解析/模式 | `yyVisionCollectURLs`, `yyQQVisionEnabled` | 275 |
| `youyuan_qq_voice.go` | 语音发送与模式/音色 | `yyVoiceSend`, `yyQQVoiceMode` | 298 |
| `youyuan_qq_watchdog.go` | QQ 连接看门狗（调参/探活） | `yyQQWatchdogStart`, `yyQQWatchdogProbe` | 337 |
| `youyuan_qq_web.go` | QQ 侧网页抓取工具（URL 白名单/IP 校验） | `yyWebFetch`, `yyWebCheckURL` | 236 |
| `youyuan_receive_transport.go` | 收包传输诊断方法（帧/字节/未知包计数） | `yyReceiveTransport` | 24 |
| `youyuan_region_pipeline.go` | 区域读取并行流水线（瓦片 / worker） | `yyRegionTile`, `yyRegionPlan` | 303 |
| `youyuan_region_read.go` | 读取 region 子区块并转结构 | `yyReadRegionSubChunks`, `yySubChunksToStructure` | 133 |
| `youyuan_remote.go` | 云端远控：配置持久化、WS 接入、MCP 端点、**API 注册总表** | `yyRegisterAPI`(71), `yyInvoke`(205), `yyStartRemote`(538) | 652 |
| `youyuan_remote_admission.go` | 远端准入：包排队/背压/失败响应 | `yyRelayPacket`, `yyRemoteAdmission` | 171 |
| `youyuan_resident.go` | Lua 常驻运行时与感知投递队列 | `yyLoadResident`(133), `yyQueueSenseLua` | 362 |
| `youyuan_resource.go` | 资源目录与 Lua resource API | `yyResourceRoots`, `registerResourceAPI` | 212 |
| `youyuan_room_discovery.go` | 房间发现与选择记忆 | `yyRoomCandidate`, `yyRoomRemember` | 252 |
| `youyuan_rpc_ingress.go` | RPC 入口目录 / 幂等 / 淘汰 | `yyRPCIngressCatalog`, `yyRPCIngressSnapshot` | 157 |
| `youyuan_runtime.go` | Python/原生运行时清单与实例管理路由 | `yyRuntimeManifest`, `yyRuntimeRoutes` | 434 |
| `youyuan_runtime_android.go` | Android 侧 Python worker 派发 | `yyPythonDispatchTask`, `yyPythonWorker` | 210 |
| `youyuan_runtime_dependencies.go` | 运行时版本校验与依赖检查 | `yyRuntimeCheckDependencies` | 43 |
| `youyuan_runtime_desktop.go` | 桌面侧 Python 进程管理 | `yyPythonProcess`, `yyPythonRuntime` | 134 |
| `youyuan_runtime_export.go` | 运行时导出打包 | `yyRuntimeExport` | 119 |
| `youyuan_runtime_management.go` | 运行时目录与管理路由 | `yyRuntimeDirectory`, `yyRuntimeManagementRoutes` | 186 |
| `youyuan_runtime_restore.go` | 运行时选择记忆与恢复循环 | `yyRuntimeChoice`, `yyRestoreRuntimeLoop` | 95 |
| `youyuan_sample_cleanup.go` | 内置示例归档清理 | `yyArchiveBundledSamples` | 53 |
| `youyuan_scheduler.go` | 定时调度 Lua API 与调度控制 | `yySchedulerControl`, `registerSchedulerAPI` | 140 |
| `youyuan_sdk.go` | SDK 沙箱会话（web id / prepare / get / close / 入队） | `yySDKListener`, `yySDKPrepare`, `yySDKContext` | 901 |
| `youyuan_sdk_assets.go` | SDK 示例/观测器/头文件安装 | `yySDKInstallSamples`, `yySDKSeed` | 95 |
| `youyuan_seed_api.go` | 种子查询 API：JavaRandom/史莱姆/要塞/结构定位 | `yySeedJavaRandom`, `yySeedIsSlimeChunk` | 1522 |
| `youyuan_seed_biome.go` | 种子群系推算（Xoroshiro/Perlin/气候） | `yyXoro`, `yyPerlin`, `yyClimate` | 701 |
| `youyuan_seed_structures.go` | 结构规格表（维度 / ID 归一化） | `yySeedStructureSpec`, `yySeedStructureSpecs` | 392 |
| `youyuan_seed_world.go` | 世界种子读写与路由 | `yyWorldSeedRead`, `yyWorldSeedRoutes` | 226 |
| `youyuan_seedv2_aitools.go` | v2 种子地图的 AI 工具（调用/热度/seed 解析） | `yySeedV2ToolCall`, `yySeedV2ToolRun` | 422 |
| `youyuan_seedv2_routes.go` | v2 路由：查询参数解析与瓦片接口 | `yySeedV2Query`, `yySeedV2Routes` | 653 |
| `youyuan_seedv2_sdk.go` | v2 的 SDK / 脚本调用入口 | `yySeedV2Call`, `yySeedV2OptString` | 352 |
| `youyuan_selector_camera.go` | 目标选择器相机（@e 选择器 → 镜头） | `yySelectorCameraCommand`, `runSelectorCamera` | 194 |
| `youyuan_self_position.go` | 用 StartGame 数据校正自身位置 | `yyApplySelfPosition` | 40 |
| `youyuan_sense.go` | 感知：事件聚合、战斗近期、角色观测 | `yyActorObservation`, `yySenseState`, `yySenseHandle` | 1016 |
| `youyuan_sense_api.go` | 感知 Lua API：`events.on*` 钩子注册、容器变化、`sense.call` 分发与参数解析 | `registerSenseAPI`(128), `yySenseCall`(337), `yySenseLuaEmit`(111) | 867 |
| `youyuan_server_observation.go` | 服务器级观测：按 session/kind 聚合快照、健康度换算、事件上报 | `yyServerSnapshotRecord`(23), `yyServerHealth`(120), `yyServerObservationEmit`(146) | 281 |
| `youyuan_skill_library.go` | AI 技能库：按语言列 catalog、打包 zip、`/api/youyuan/skills/export` 与 `/read` | `yySkillCatalog`(15), `yySkillArchive`(43), `yySkillLibraryRoutes`(94) | 219 |
| `youyuan_skin_account_media.go` | 账号皮肤/头像出图：解析账号名 → 取贴图 → 裁剪头像 | `yySkinAccountResolve`(26), `handleSkinAccountTexture`(88), `handleSkinAccountHead`(105) | 117 |
| `youyuan_skin_cache.go` | 皮肤 HTTP 缓存与 SSRF 防护（域名白名单、私网 IP 拦截、负缓存） | `yySkinCache`(75), `yySkinCheckURL`(184), `yySkinDialGuard`(213) | 416 |
| `youyuan_skin_disk_cache.go` | 皮肤磁盘缓存：异步写盘队列、sha1 key、容量裁剪 | `yySkinCacheEnqueue`(68), `yySkinDiskTrim`(112), `yySkinCacheGet`(167) | 235 |
| `youyuan_skin_item.go` | 皮肤道具：兑换码/凭证校验、拉取落盘、屏蔽名单 | `yySkinItemCred`(84), `yySkinItemStore`(128), `yySkinItemInit`(95) | 666 |
| `youyuan_snapshot.go` | 首页统一快照：进程内直调既有 handler 把 7 条 GET 合成 1 条，减少 Worker/DO 双扣 | `yySnapshotHandler`(19) | 63 |
| `youyuan_social.go` | 社交访问（点赞/关注）批处理、访问队列、结果回传与 `/api/youyuan/social*` 路由 | `yyQueueConnect`(115), `yyRunSocialBatch`(188), `yySocialRoutes`(236) | 352 |
| `youyuan_spatial_lease.go` | 空间租约：同一时刻只允许一个位移型作业占用观察者指令通道 | `yyBeginSpatialJob`(48), `yyObserverSend`(31), `yyPluginMovementAllowed`(85) | 115 |
| `youyuan_spawn_state.go` | 用 StartGame 包整体重置出生状态（eid/runtimeID/gameMode/pos/dimension） | `yyApplySpawnStateLocked`(8) | 15 |
| `youyuan_sse_delivery.go` | SSE 投递兜底：慢客户端队列满时让终止消息挤掉最旧一条，保证送达 | `yyEnqueueSSE`(4) | 21 |
| `youyuan_storage_cache.go` | 插件 `storage` 命名空间：按 dataDir 分桶、JSON 落盘、深拷贝读写 | `yyStorageFor`(54), `yyStorageCall`(130), `registerStorageAPI`(261) | 285 |
| `youyuan_structure_chains.go` | 结构 → 命令链：把 StructureData 翻译成连锁命令块 | `yyStructureChains`(53), `yyStructureCommand`(12) | 197 |
| `youyuan_structure_export.go` | 结构导出为 BDX / .schem 两种格式 | `yyStructureBDX`(15), `yyStructureSchem`(104) | 121 |
| `youyuan_structure_hud.go` | 建造进度 HUD 文本渲染（进度条、火花线、ETA、标签截断） | `yyStructureHudRender`(168), `yyStructureHudBar`(155), `yyHudSpark`(112) | 355 |
| `youyuan_swing_input.go` | `bot.swingAir` 空挥手臂 API 注册 | `yySwingAir`(8) | 23 |
| `youyuan_task_presentation.go` | 任务进度 actionbar 文案（TPS 已知/未知两套格式） | `yyTaskActionbar`(11), `yyTaskActionbarMetrics`(22) | 53 |
| `youyuan_terminal.go` | 只读识别玩家聊天里的 say/tellraw 提交（不落库、不逐条记日志） | `yyReadableChatCommand`(9), `yyChatSubmitted`(41) | 51 |
| `youyuan_theme.go` | 主题与插件清单体系：扫描/启用/激活 + `/api/mcort/themes|theme|theme/asset|plugins|announcements` | `yyThemeRoutes`(712), `yyThemeScan`(209), `yyPluginManifest`(92) | 1009 |
| `youyuan_tick_loop.go` | 全局 tick 循环：驱动调度任务队列、唤醒与退出 | `yyTickLoop`(22), `yyWakeTickLoop`(14), `yySchedExit`(120) | 126 |
| `youyuan_timezone.go` | 启动时设定进程时区 | `yyInitTimeZone`(11) | 22 |
| `youyuan_tunnel.go` | 内网穿透进程管理：找二进制、启停、状态、重试退避、公开入口 handler（cloudflared quick tunnel） | `yyTunnelStart`(62), `yyTunnelRun`(131), `yyTunnelPublicHandler`(117) | 336 |
| `youyuan_tunnel_process_linux.go` | 构建标签 linux：穿透子进程放进独立进程组，避免随父进程被信号带走 | `yyPrepareTunnelProcess`(10) | 12 |
| `youyuan_tunnel_process_other.go` | 构建标签 !linux：穿透子进程隐藏窗口 | `yyPrepareTunnelProcess`(7) | 7 |
| `youyuan_tunnel_ssh.go` | Pinggy SSH 反向隧道：主机指纹校验、循环重连 | `yyTunnelPinggy`(64), `yyPinggyHostKey`(47) | 170 |
| `youyuan_video_batch.go` | 方块视频分帧批量追加（base64 图片 → 帧序列） | `yyBlockVideoJob.appendImages`(15) | 61 |
| `youyuan_video_pixels.go` | 视频像素量化：RGB→Lab、最近调色板匹配、帧间矩形差分 | `yyVideoNearest`(33), `yyVideoRectangles`(53), `yyRGBLab`(10) | 97 |
| `youyuan_visibility.go` | 可见性协商：区块半径请求与 `diagnostics.visibility` 采集（含方法说明表） | `yyVisibilityCall`(83), `yyVisibilityRecordRequest`(24), `yyVisibilitySnapshot`(31) | 152 |
| `youyuan_world_observer.go` | 世界观测：按模式收集实体/方块视图、维度解析、Lua 侧 `world.*` 出口 | `yyWorldCollect`(222), `yyWorldView`(215), `yyWorldCall`(441) | 515 |

---

## 4. 内核主链路：一次玩家操作从 HTTP 到返回

以最典型的一条 —— 前端点「连接服务器」（`POST /api/bot/connect`）为例，逐跳给出文件与行号。
这条链路串起了**鉴权层 → 路由层 → handler → 连接内核 → 包分发 → 业务分支 → SSE/JSON 回程**，
其它 383 条 `/api/*` 走的是同一套骨架，只是第 4 跳不同。

| 跳 | 位置 | 做了什么 |
|---|---|---|
| ① 前端发起 | `frontend/js/youyuan_accounts.js:463`（`yyRootAPI` 定义在 `:2`） | `yyRootAPI("POST","/api/bot/connect",{token,server,auth,password})` |
| ② 自动加凭证 | `frontend/js/youyuan_console_token.js:18-30` | 包装 `window.fetch`，对同源请求补 `X-YouYuan-Console-Token`；token 取自 `:12` 读 `<meta name="yy-console-token">`（由 `main.go:268 handleIndex` 注入） |
| ③ 鉴权中间件 | `main.go:5280` | `yyConsoleAuth(yyIntegrityGate(corsHandler))`：会话鉴权在最外，完整性闸门居中，CORS 在内 |
| ④ 会话校验 | `youyuan_console_auth.go:86`（判定 `:90-120`） | 非 `/api` 直接放行；`OPTIONS` 放行；能力型下载与 `GET /api/skin/*`、`/api/market-proxy/*` 放行；其余走 `yyConsoleRequestAllowed`（`:75-79`，console token **或** 远控 key 皆可），失败 401 `{"ok":false,"error":"控制台会话已失效，请重新打开工具箱"}`（`:118`） |
| ⑤ 完整性闸门 | `integrity_gate.go:117` | 篡改判定为真时统一拦截；取不到校验依据则降级放行（不误伤） |
| ⑥ 同源限制 | `main.go:5247-5274` | 只回显 `127.0.0.1` / `localhost` 的 Origin；写操作遇 `Sec-Fetch-Site: cross-site` 直接 403 |
| ⑦ 路由表 | `main.go:5102` | `mux.HandleFunc("/api/bot/connect", handleBotConnect)`；mux 在 `main.go:5010` 由 `yyRegisterAPI`（`youyuan_remote.go:71`）挂 33 组 `yy*Routes`，`main.go` 自身另有 183 条 |
| ⑧ handler | `main.go:1039 handleBotConnect` | 解 body → `yyBeginConnectionAttempt` 并发闸 → `yyNormalizeServer` → `yySaveConnectionCredentials` 落盘 → 断开旧实例并 `yyAwaitBotClosed(3s)` → `NewBotManager()`（`bot_conn.go:144`）→ 锁协议版本 `protocol.CurrentProtocol=860` → 起 goroutine 调 `Connect` |
| ⑨ 连接内核 | `bot_conn.go:153 Connect` | 走 vendored `flowers-for-machines` 客户端完成握手/登录；失败回 `resultErr` 交给 `yyEndConnectionAttempt` |
| ⑩ 包分发 | `bot_conn.go:627 registerPacketHandlers` | 每类包一个分支，写入 `bm` 状态；`StartGame` 由 `youyuan_spawn_state.go:8 yyApplySpawnStateLocked` **整体覆盖** eid/runtimeID/gameMode/pos/dimension（重连也覆盖） |
| ⑪ 业务分支 | 例如 `youyuan_tick_loop.go:22`、`youyuan_command*.go`、`youyuan_plugins*.go`、`youyuan_world_observer.go:222` | 任务/命令链/插件/观测各自订阅包事件或定时循环 |
| ⑫ 回程 A：立即 JSON | `main.go:160 writeJSON` | POST 不等连接完成就返回 `{"ok":true}`，避免前端超时 |
| ⑬ 回程 B：进度流 | `main.go:4955`（`hub = NewSSEHub()`）→ `main.go:296 handleSSE` | `onReady` 里 `hub.Emit("conn_ready","{}")`；前端订阅 `/stream`（SSE），慢客户端由 `youyuan_sse_delivery.go:4 yyEnqueueSSE` 兜底丢旧保终态 |
| ⑭ 回程 C：状态轮询 | `main.go:1164 handleBotStatus` | `GET /api/bot/status` 读 `botMgr` 快照返回 `{ok,connected,is_op,server,server_target}` |

### 4.1 第二条链路：进程内直调（不经 TCP）

`yyInvoke`（`youyuan_remote.go:205`）用 `httptest.NewRecorder()` 直接把请求喂给同一个 `yyMux`，
**不复制业务逻辑、不走网络**。使用方：

- `/api/youyuan/snapshot`（`youyuan_snapshot.go:19`）：把首页 7 条 GET 合成 1 条，专门为了少扣 Worker + DO 的双次计数；
- MCP 端点 `/mcp`（`youyuan_remote.go:234 yyMCP`）：JSON-RPC 方法名映射到内部 API 路径。

### 4.2 第三条链路：插件 → 宿主

Lua/Python/C++ 插件调 `game.*` / `bot.*` 等宿主 API 时，入口是 `lua_api.go:15 registerAllLuaAPI`
与各域 `register*API`（如 `youyuan_sense_api.go:128`、`youyuan_storage_cache.go:261`），
最终仍落到同一个 `bm` 状态或同一条内部 API，**不另开一条旁路**。详见 `plugin-runtime.md` 第 5 节。

---

## 5. 测试文件附录（294 个 `_test.go`，按 `ls` 顺序）

测试文件不承担运行时职责，但决定了「改哪里会被测试挡住」。第三列是文件内的前两个 `Test*` 函数，
即该文件实际在守的行为面。

| 文件 | 首个/次个测试函数 | 行数 |
|---|---|---|
| `ai_game_test.go` | TestGameCallNewFunctionsRecognized | 53 |
| `ai_lua_call_test.go` | TestToolLuaCallPlatform,TestToolLuaCallUnknownModule | 39 |
| `ai_models_test.go` | TestYouYuanNoAuthorDefaultAI,TestBuiltinModelList | 79 |
| `ai_observe_test.go` | TestAIObserverMaxReturnsEarly,TestAIObserverTimeoutReturnsPartial | 95 |
| `ai_test.go` | TestAIModelStoreCRUD,TestMaskKey | 349 |
| `apk_signing_test.go` | TestApkSigningCertHash_Official,TestApkSigningCertHash_InWhitelist | 66 |
| `bot_op_fix_test.go` | TestOPPacketConfirmedSurvivesCommandFailure,TestOPUnconfirmedClearsOnCommandFailure | 49 |
| `cb_export_test.go` | TestExtractCommandBlocks_ZFastest,TestExtractCommandBlocks_ArrayIndices | 120 |
| `cb_import_lenient_test.go` | TestParseCBProject_PlainText,TestParseCBProject_EmptyLineNewChain | 89 |
| `cb_import_test.go` | TestCBProjectRoundTrip | 49 |
| `cb_trace_test.go` | TestTraceChains_WithVertical,TestFacingNames | 40 |
| `chat_menu_test.go` | TestParseMenuChoice | 22 |
| `chat_trigger_test.go` | TestHasWaitMsg,TestParseArgValue | 141 |
| `command_dispatch_test.go` | TestNormalizeCommandChannel,TestCommandChannelValid | 43 |
| `draw_handlers_test.go` | TestHandlerCreateAndStroke,TestHandlerCreateMissing | 123 |
| `draw_session_test.go` | TestRegionFromOriginRect,TestRegionFromOriginMax512 | 263 |
| `floating_test.go` | TestMusicNoteName,TestEmitForwardsToFloatingHub | 29 |
| `fly_rotate_test.go` | TestNormalizeAngle,TestClampPitch | 111 |
| `format_command_message_test.go` | TestFormatCommandMessage | 24 |
| `full_verify_test.go` | TestFullVerifyNewApk | 24 |
| `game_data_test.go` | TestCleanTileName | 18 |
| `integrity_gate_test.go` | TestIntegrityGatePassesWhenClean,TestIntegrityGateBlocksAPIWhenTampered | 289 |
| `integrity_native_test.go` | TestVerifyApkIntegrity_NativeOfficial,TestVerifyApkIntegrity_NativeTampered | 46 |
| `lua_api_test.go` | TestNewLuaState,TestLuaSendCommandHook | 34 |
| `lua_api_test2_test.go` | TestLuaGameAPIBound,TestLuaShowMenuCallable | 90 |
| `lua_botctx_test.go` | TestBotCurrentDefault,TestBotSelectMissingRaises | 51 |
| `lua_cross_call_test.go` | TestCallPluginFunctionAnyGlobal,TestCallPluginFunctionAnyPluginAPI | 71 |
| `lua_cross_test.go` | TestCrossPluginCall,TestCrossPluginMissing | 257 |
| `lua_events_test.go` | TestNewCapabilityTablesBound,TestEventOnChat | 183 |
| `lua_game_legacy_test.go` | TestLegacyImportPreservesStatesNBTAndLargeList,TestLegacyImportRejectsInvalidStatesInsteadOfDropping | 39 |
| `lua_http_api_test.go` | TestHTTPAPI | 77 |
| `lua_plugin_test.go` | TestLuaRegisterTriggerCallback,TestLuaRegisterTriggerMultiArgs | 62 |
| `lua_runtime_test.go` | TestRunLuaSnippet | 14 |
| `lua_system_test.go` | TestLuaSystemAPIBound,TestLuaSystemAvailableInTriggerHandler | 80 |
| `nbt_world_api_test.go` | TestParseStructureData_CommandBlock,TestParseStructureData_RegionWithContainer | 214 |
| `normalize_states_test.go` | TestNormalizeBlockStates | 35 |
| `notification_test.go` | TestDispatchNewPushMessages,TestResolveDisplayMode | 78 |
| `packet_hub_test.go` | TestPacketToMapBasic,TestPacketToMapMovePlayerNested | 110 |
| `player_info_death_grace_test.go` | TestDeathGraceWindowSuppressesJoinLeavePair,TestMarkDeathIgnoresEmptyNameAndPrunesOldEntries | 58 |
| `plugin_autoload_test.go` | TestDeleteRemovesFilesAndDisabled,TestSetDisabledPersist | 142 |
| `plugin_backup_test.go` | TestFullPluginBackupPreservesNestedFilesAndData,TestFullPluginBackupRejectsCrossPluginPath | 102 |
| `plugin_config_test.go` | TestConfigReadWrite,TestImportExportRoundTrip | 225 |
| `plugin_controller_test.go` | TestPluginReloadPreservesExplicitDisabledState,TestPluginControllerLifecycle | 164 |
| `plugin_defs_test.go` | TestBuiltinPluginDefs | 20 |
| `plugin_edit_test.go` | TestCreatePluginLua,TestCreatePluginWordbank | 167 |
| `plugin_http_test.go` | TestHandlePluginCreate,TestHandlePluginFileReadWriteCode | 165 |
| `plugin_pack_test.go` | TestImportPackageRejectsTraversalID,TestImportPackageAllowsChineseID | 153 |
| `plugin_scan_test.go` | TestScanPlugins,TestBuildDefFromFileWordBank | 144 |
| `plugin_source_dir_test.go` | TestScannedMainKeepsActualSourceDirAndExistingDataIdentity,TestScannedTwoMainEntriesReportCollisionWithoutOverwritingOwner | 276 |
| `plugins_verify_test.go` | TestPluginPerFolderStructure,TestPluginSuiteVerification | 117 |
| `protect_secrets_test.go` | TestProtectSecretsDecodePlaintext,TestProtectSecretsQQPrompts | 124 |
| `protect_selfcheck_test.go` | TestSealVerifyUnsealedSealedMismatch,TestSealVerifyRejectsDuplicateRecord | 150 |
| `schematic_roundtrip_test.go` | TestSchematicExportRoundTrip | 57 |
| `seedv2_biome_golden_test.go` | TestSeedV2BiomeGoldenTile,TestSeedV2BiomeGoldenSweep | 368 |
| `seedv2_biome_test.go` | TestSeedV2BiomeGolden,TestSeedV2BiomeAxes | 407 |
| `seedv2_btree_test.go` | TestSeedV2BTreeDatSize,TestSeedV2BTreeLookup | 320 |
| `seedv2_place_test.go` | TestSeedV2PlaceAnchors,TestSeedV2PlaceConfigBranches | 599 |
| `seedv2_rand_ref_test.go` | TestSeedV2RandAnchor,TestSeedV2RandJavaLCG | 361 |
| `seedv2_render_test.go` | TestSeedV2RenderPNGDecode,TestSeedV2RenderSeedVariation | 429 |
| `structure_block_test.go` | TestBuildStructureBlockUpdate_BedrockLowercase,TestBuildStructureBlockUpdate_JavaFallback | 80 |
| `test_util_test.go` | — | 28 |
| `tickingarea_confirmation_test.go` | TestTickingMutationReceiptLossUsesReadbackWithoutReplaying,TestTickingAreaReadbackRequiresDimensionAndAllRequestedChunks | 49 |
| `tickingarea_empty_test.go` | TestTickingEmptyReplyIsNotACleanupFailure | 21 |
| `tickingarea_recovery_test.go` | TestTickingAreaReceiptActualAndLegacy,TestTickingAreaOwnershipSurvivesRestartAndSkipsLiveWork | 150 |
| `web_test.go` | TestWebRegistryRegisterAndRemove,TestWebPushAndChannelClose | 565 |
| `wordbank_test.go` | TestLoadWordBank,TestExecCommandReplacesPlayer | 83 |
| `youyuan_account_bridge_test.go` | TestAccountBridgeDirectionalRules,TestAccountBridgeLuaRegistration | 72 |
| `youyuan_account_identity_test.go` | TestAccountIdentityDoesNotPollOfflineWorker | 21 |
| `youyuan_account_pool_test.go` | TestPoolIdleExpiryAndRebindingCancelsExit,TestPoolLuaAndStructuredSDKBinding | 278 |
| `youyuan_account_profiles_test.go` | TestAccountProfilePreservesCredentialsAndUniqueOwner,TestAccountAvatarRejectsExecutableAndMalformedInput | 48 |
| `youyuan_account_slots_test.go` | TestAccountProfilesAcquireSlotsOnlyForRunningProcesses | 35 |
| `youyuan_admission_roster_test.go` | TestAdmissionAcceptsOnlySuccessfulNonemptyCompleteList | 55 |
| `youyuan_admission_test.go` | TestAdmissionRunsWithObservationOffAndOnlyRejects23rdNewHuman,TestAdmissionNeverRejectsExistingRosterWhenEnabledLoweredOrReconnected | 142 |
| `youyuan_agent_attachments_test.go` | TestAttachmentTextAndCharacterPaging,TestConfigurableBurstBudget | 43 |
| `youyuan_agent_binding_test.go` | TestBoundLuaSourceReadsModulesAndProtectsConcurrentEdits,TestValidationWorkspacesArchivedWithoutUserProjectLoss | 111 |
| `youyuan_agent_budget_test.go` | TestAgentContinuesPastThreeFailuresThirtyTwoToolsAndTwentyFourRounds,TestAgentExplicitToolBudgetAndRoundBudgetStillCheckpoint | 203 |
| `youyuan_agent_commands_test.go` | TestYouYuanTerminalChainPreservesDecimalsAndJSON,TestYouYuanCommandChainActualConditions | 208 |
| `youyuan_agent_context_usage_test.go` | TestContextPercentAndProviderCalibration,TestAgentContextDoesNotSilentlyCutRequestedFilesOrTaskHistory | 164 |
| `youyuan_agent_knowledge_test.go` | TestWorkspaceKnowledgePreservesToolRestrictions,TestAgentKnowledgeSections | 58 |
| `youyuan_agent_mcp_catalog_test.go` | TestMCPToolCatalogLoadsOnlySelectedSchema | 19 |
| `youyuan_agent_plugin_call_test.go` | TestAgentCanCallResidentPluginWithoutLosingOptOut | 28 |
| `youyuan_agent_sessions_test.go` | TestAgentDefaultSkillsRemainLazyAndLanguageScoped,TestAgentConversationPersistenceAndSessionIsolation | 127 |
| `youyuan_ai_delivery_test.go` | TestFishingHookFiltersBeforeVMAndPreservesFallback,TestAIPatchedActualSourcesCompile | 160 |
| `youyuan_ai_provider_test.go` | TestDeepSeekToolTurnKeepsReasoningAndSelectedSkill | 36 |
| `youyuan_api_inventory_test.go` | TestDocumentationInventory | 25 |
| `youyuan_block_cache_test.go` | TestBlockCacheOverflowKeepsRecentContainers,TestBlockCacheTrimLeavesSmallCachesAlone | 52 |
| `youyuan_bridge_security_test.go` | TestBridgeRejectsWithoutToken,TestSrvInfoGuardBlocksNamespaceBypass | 176 |
| `youyuan_bugfix_01015_test.go` | TestAgentForkPreservesExplicitDisabledToolsAcrossReload,TestModelCapacitySurvivesAPIAndDisk | 177 |
| `youyuan_building_jobs_test.go` | TestBuildingTaskAPIReadsRealCheckpoint | 36 |
| `youyuan_callback_io_pressure_test.go` | TestLiveInventoryMonitorUsesYieldingQueryWithoutDroppingChecks,TestPlayerInventoryUsesCooperativeGameBinding | 139 |
| `youyuan_callback_queue_test.go` | TestCallbackQueueConfigurationRequiresReloadAndPersists | 51 |
| `youyuan_capture48_test.go` | TestLegacyRegistrationsAcceptNamesAndFunctionsWithoutResetOrCrossTalk,TestTransferSpatialLeaseIsNestedAndOnlyBlocksSelfMovement | 189 |
| `youyuan_capture_behavior_test.go` | TestCaptureBehaviorOnlyChangesExplicitSettings,TestCaptureBehaviorRefreshIsBoundedAndFailedCommandsRetry | 62 |
| `youyuan_capture_hotpath_test.go` | TestCameraExclusionsNeverProbeChildHTTP,TestCaptureOnlyKernelRefusesPluginRun | 171 |
| `youyuan_capture_identity_test.go` | TestStaleAccountSlotDoesNotPoisonRobotIdentities,TestSlotlessAccountIsNotAnIdentityFailure | 129 |
| `youyuan_capture_landing_test.go` | TestCaptureLandingRequiresRealDimensionAndFeetPosition,TestCaptureLandingValidationAndNoImplicitLanding | 56 |
| `youyuan_capture_plan_test.go` | TestCaptureHumanThresholdsAndManualDeployment,TestCaptureIdleDimensionsHelpAndReturnHome | 148 |
| `youyuan_capture_query_test.go` | TestCaptureQueryDoesNotTreatEmptyExecuteSuccessAsDimension | 44 |
| `youyuan_capture_queue_test.go` | TestCaptureEnqueueIsBoundedAndCountsDrops,TestCaptureWorkerStoresWhatWasAccepted | 194 |
| `youyuan_capture_residents_test.go` | TestCaptureKeepsThreeIdleDimensionResidents,TestCaptureOverheadCameraDoesNotTiltWithTarget | 67 |
| `youyuan_chain_activation_test.go` | TestFastChainDoesNotReplayPulseOrUnpoweredCommands,TestRedstonePulseWaitDoesNotBlockIndependentBootstrap | 88 |
| `youyuan_chain_alloc_test.go` | TestFastSettingsDoesNotRebuildNameEscaper | 58 |
| `youyuan_chain_buffered_test.go` | TestFastChainOnlyConfirmsTrueDependencies,TestBufferedChainBarriersAndFailure | 95 |
| `youyuan_chain_cadence_test.go` | TestExplicitChainCadenceCapsModulesWithoutSpeedingSlowSegments,TestConfiguredHzOverridesMainModuleWithoutSpeedingClocks | 91 |
| `youyuan_chain_coalesced_test.go` | TestChainCoalescedPreservesOrderWithoutExtraPackets,TestChainCoalescedStopsAtFirstUnknownWrite | 54 |
| `youyuan_chain_control_test.go` | TestChainCallRangeOrderAndBarrier,TestChainRejectRecursiveAndCancelPersistent | 137 |
| `youyuan_chain_fast_module_test.go` | TestChainFastGateSerializesAndCancelsWaiting,TestChainFastWriteFailurePoisonsConnectionWithoutReplay | 318 |
| `youyuan_chain_fast_stop_test.go` | TestFastChainChildDeadlineOnlyResumesBeforeUnknownWrite,TestFastChainFirstModuleFailureStopsShortSegmentsWithoutAnotherRound | 48 |
| `youyuan_chain_fast_summary_test.go` | TestFastNativeSummaryKeepsConditionalBarrierWithoutResultTables,TestFastNativeSummaryReachesChildGraphs | 90 |
| `youyuan_chain_gate_recovery_test.go` | TestChainGateRetriesOnlyProvenReadConditions,TestChainGateReadOutageJoinsChildAndRecoversWithoutFalseBranch | 75 |
| `youyuan_chain_gate_test.go` | TestChainGateLoadsOnConditionAndJoinsBeforeNextActivation,TestChainGateSlowProbeDoesNotThrottleChildAndRejectsRecursion | 107 |
| `youyuan_chain_ground_entry_test.go` | TestGroundReturnEntryOwnsExactlyOneConditionalWorker,TestGroundReturnChecksPendingRequestOnceBeforeWholeBatch | 66 |
| `youyuan_chain_menu_bridge_test.go` | TestNativeMenuUsesRunnerParserAndOneToggleOperation | 70 |
| `youyuan_chain_module_stats_test.go` | TestFastModuleRecordsOneSummaryWithoutPerCommandCallbacks,TestFastModuleKeepsErrorsAndCancellationOutOfSuccessSummary | 54 |
| `youyuan_chain_multi_module_test.go` | TestChainMultipleModulesStayPreparedAcrossAlternatingSpans,TestChainModuleReloadAndReconnectEvictOldBuffers | 113 |
| `youyuan_chain_prepared_test.go` | TestChainPreparedReusesOnlyUnchangedImmutableModule,TestChainPreparedWireBytesStayIdenticalAcrossRounds | 167 |
| `youyuan_chain_segments_test.go` | TestSegmentHzIsPerIndependentChainAndPulseIsOnce,TestSegmentResourceReloadRearmsChangedPulseAndKeepsSibling | 127 |
| `youyuan_chain_stability_test.go` | TestFastDelayDoesNotCreateResponseDependency,TestBranchChoosesOneModuleAndReturns | 88 |
| `youyuan_chain_vm_pressure_test.go` | TestChainStopWaitingForWorkerDoesNotHoldPluginVM | 53 |
| `youyuan_command_batch_gate_test.go` | TestCommandBatchCanceledWaiterDoesNotHoldOrStealSlot,TestCommandBatchContentionKeepsSingleOwnerAndAllWaitersProgress | 115 |
| `youyuan_command_context_compat_test.go` | TestFastSettingsLegacyContextForExecuteCommands,TestFastSettingsLegacyContextWithoutBotName | 40 |
| `youyuan_command_rates_test.go` | TestExplicitRateOverridesLegacySingleAndBatchPace,TestChainFrequencyAppliesToWholeRound | 45 |
| `youyuan_command_stream_test.go` | TestSlidingRefillsBeforeFirstReply,TestSlidingTimeoutDoesNotResendOrGrowQueue | 163 |
| `youyuan_command_structure_test.go` | TestBuildingConvertersPreserveActualNBTFixtureAndEmptyBorders,TestCommandStructurePhysicalChain | 174 |
| `youyuan_connection_diagnostic_test.go` | TestConnectionHistoryKeepsFirstError,TestConnectionHistoryResetsFirstErrorPerAttempt | 64 |
| `youyuan_connection_error_hint_test.go` | TestConnectionExpiredServerDoesNotBecomeCredentialFailure | 26 |
| `youyuan_connection_generation_test.go` | TestOldConnectionCannotDisconnectReplacement | 22 |
| `youyuan_connection_policy_test.go` | TestReconnectContinuesAfterThirtyFailedRounds,TestConnectionPolicyPersistsWithoutErasingCredentials | 181 |
| `youyuan_connection_report_test.go` | TestDropReasonKindOnRealLogLines,TestLogFieldAndTimeOnRealLines | 223 |
| `youyuan_console_auth_test.go` | TestYYConsoleAuth,TestYYConsoleInjectToken | 99 |
| `youyuan_converter_preview_test.go` | TestConverterPreviewServer | 47 |
| `youyuan_converters_test.go` | TestConversionStructureRoundTripAndMerge,TestConversionCommandsExecutionAndLossDetection | 69 |
| `youyuan_delivery_test.go` | TestYouYuanDisabledAgentToolCannotModifyProject,TestYouYuanBinaryRevisionsAndMCPDocs | 100 |
| `youyuan_display_io_pressure_test.go` | TestRichChatBackpressureReleasesHostVM | 36 |
| `youyuan_egress_test.go` | TestYYEgressSOCKS5RealUDPRoundTrip,TestYYEgressSOCKS5RakNetConnection | 393 |
| `youyuan_entity_snapshot_test.go` | TestHealthSnapshotIncludesArmorWhenSynced,TestHealthSnapshotArmorStaysUnknownWithoutSync | 97 |
| `youyuan_execution_options_test.go` | TestAgentSchemasHaveArrayRequiredEvenForNoArguments,TestPluginCommandOptionsAreIsolatedAndValidated | 87 |
| `youyuan_export_center_test.go` | TestExportCenterCompleteSnapshotDownloadsAndTenantIsolation,TestRuntimeExportRetainedZipHasCompleteResourceTree | 182 |
| `youyuan_fast_settings_test.go` | TestFastArrayDoesNotIgnoreExplicitTransport,TestFastSettingsPreservesBotContext | 29 |
| `youyuan_file_upload_sessions_test.go` | TestResumableUploadLostAcknowledgementAndRestart,TestUploadRejectsCorruptionAndUncommittedTail | 261 |
| `youyuan_fleet_panel_test.go` | TestFleetConnectionPanelIsCompactAndNotImport | 15 |
| `youyuan_framework_0109_test.go` | TestResidentStopCancelsActiveLuaBeforeWaitingForVMLock,TestRegionFallbackNegativeCoordinatesNBTAndMissing | 327 |
| `youyuan_game_dispatch_test.go` | TestUnknownAPIReturnsJSON,TestPublicTPSUnknownAndStatefulImportAreExplicit | 41 |
| `youyuan_health_snapshot_test.go` | TestHealthDisplaySeedsOnlyObservedPresentSessionPlayers | 37 |
| `youyuan_host_files_test.go` | TestHostFileUploadDownloadAndCollision,TestLinuxLibraryUsesLocalDiskAndSurvivesRestart | 86 |
| `youyuan_hotfix45_test.go` | TestLiveCombatPatchOnlyMatchedWithin18AndNoFullScene,TestContainerGuardRapidRepeatDoesNotWaitOrDuplicate | 120 |
| `youyuan_identity_test.go` | TestYYIdentityRegistrationPreflightAvoidsOrphanAccount,TestYYIdentityDeletedBindingClearsTokensButRetainsProjects | 108 |
| `youyuan_import_checkpoint_test.go` | TestImportResumeHandoffSurvivesRestartBeforeFirstProgress | 34 |
| `youyuan_import_limits_test.go` | TestImportLimitsPublicCallAndValidation | 34 |
| `youyuan_import_probe_test.go` | TestImportChunkProbeRejectsUnloadedAndUnconfirmedReplies | 35 |
| `youyuan_import_resume_guard_test.go` | TestImportResumeDataRejectsCorruptionAndMissingPosition,TestImportResumeFailureCannotStartFreshTask | 82 |
| `youyuan_import_test.go` | TestYouYuanPluginImport | 43 |
| `youyuan_interaction_queue_test.go` | TestQueuedChatIOCannotBlockEntityFIFO,TestNativeTextAndPriorityEventsUseSeparateLanes | 77 |
| `youyuan_interface_session_test.go` | TestGameInterfaceConcurrentFirstUseCreatesOneInstance,TestGameInterfaceDoesNotPublishAfterConnectionReplacement | 168 |
| `youyuan_latency_01044_test.go` | TestContainerCacheInvalidatesWithoutInventoryExposure,TestActionHistorySurvivesMovementFlood | 150 |
| `youyuan_legacy_files_test.go` | TestLegacyRelativeFilesNeedNoPluginRewrite | 39 |
| `youyuan_load_limits_test.go` | TestAgentDefaultsOpenToolsButPreservePlayerRestrictions,TestBundledSampleMigrationPreservesFilesAndUserPlugins | 80 |
| `youyuan_log_channel_test.go` | TestDisplayLogFloodNeverWaitsForConsumer,TestNoProductionBlockingDisplayWrites | 82 |
| `youyuan_lua_protocol_values_test.go` | TestLuaProtocolHealthNumbersAndOptionalValues,TestLuaProtocolVectorsAndTypedNBT | 43 |
| `youyuan_lua_wait_test.go` | TestLuaCallerFindsLuaFrameThroughPcall,TestResidentTypedQueryWaitKeepsHealthCallbackResponsive | 148 |
| `youyuan_lua_wide_test.go` | TestWideRegistersExecuteAndKeepState,TestWideRegistersNestedFramesAndArguments | 72 |
| `youyuan_mail_test.go` | TestYYMailBridgeRejectsForgedExpiredAndNonRegistrationRequests | 50 |
| `youyuan_many_bots_test.go` | TestCaptureManyAccountPolicyAndFailover,TestCapturePolicyRejectsAmbiguousOrUnboundedSteps | 86 |
| `youyuan_map_video_test.go` | TestMapMCPJSONPreservesValues,TestMapVideoReadsOutputCanvasAndEnds | 178 |
| `youyuan_market_download_credentials_test.go` | TestMarketDownloadEntryPointsScopedCredentials | 76 |
| `youyuan_market_live_test.go` | TestMarketFree201OriginalPackageFunctions | 106 |
| `youyuan_market_test.go` | TestMarketLifecycleListMineAndFreeInstall,TestMarketPaidInstallRefusesWithoutPayment | 341 |
| `youyuan_mcp_doc_discovery_test.go` | TestYYMCPDocumentDiscovery | 41 |
| `youyuan_mcp_engineering_test.go` | TestMCPWriteTimeoutNeverSuggestsBlindReplay,TestMCPTraceDoesNotLeakPayloadAndIsOwnerScoped | 102 |
| `youyuan_mcp_protocol_test.go` | TestMCPProtocolCompatibilityAndRealInvocation | 97 |
| `youyuan_mixed_live_test.go` | TestLinuxMixedResidentCommunication,TestLinuxServerObservationRuntime | 212 |
| `youyuan_module_combat_test.go` | TestModuleCombatRecordedFarPlayerAndNoHealthFabrication,TestModuleCombatRejectsForgedSourceAndFloatIDs | 191 |
| `youyuan_multi_budget_test.go` | TestMultiBotImportBudgetSharesRateAcrossWorkers,TestMultiBotImportBudgetStopCancelsWaiter | 137 |
| `youyuan_native_desktop_test.go` | TestNativeWorkerTransportHelper,TestNativeWorkerTransportCrashAndTimeout | 83 |
| `youyuan_native_tps_test.go` | TestNativeTPSDoesNotEstimateWorldTimeChanges,TestNativeTPSLuaAliasesReturnNil | 78 |
| `youyuan_nbt_pipeline_test.go` | TestYYNBTContainerTypedRoundTrip,TestYYNBTRejectInvalidItems | 261 |
| `youyuan_observations_01023_test.go` | TestObservationHealthTruthAndReuse,TestObservationRotationWithoutPosition | 326 |
| `youyuan_observer_bindings_test.go` | TestObserverBridgeLuaBindingsAndPublicSchema | 29 |
| `youyuan_observer_bridge_test.go` | TestObserverBridgeKeyProvenanceReplayAndRevoke,TestObserverBridgePersistenceCursorAndCallerIsolation | 96 |
| `youyuan_observer_budget_test.go` | TestObservationSendUsesSharedBudgetAndHonorsJobPause | 33 |
| `youyuan_observer_dimension_test.go` | TestEmptyDimensionUsesOnlyExplicitLanding,TestDimensionAssignmentRejectsInvalidLandingBeforeSending | 149 |
| `youyuan_observer_input_test.go` | TestObserverInputNormalizesSwingWithoutInventingHit,TestObserverInputAttackTargetAndDirection | 38 |
| `youyuan_observer_method_docs_test.go` | TestObservationAndAccountPoolMethodDocsReadableAllLanguages | 28 |
| `youyuan_observer_patrol_test.go` | TestPatrolRestoreUsesCommandFeetInsteadOfInputEye,TestMappedEventRealPacketFilterIsolationAndUnload | 116 |
| `youyuan_observer_rpc_test.go` | TestObserverRPCRequiresMatchingOwnerRequestAndEnvelope,TestObserverRPCObservationProvenanceAndReset | 69 |
| `youyuan_observer_watch_test.go` | TestWatchCameraHeightPitchAndSelectorWithoutTeleport,TestWatchRejectsInvalidModesAndIntervals | 209 |
| `youyuan_op_revoke_test.go` | TestPermissionRevocationCannotReuseOldOperatorCache | 47 |
| `youyuan_packet_api_test.go` | TestPacketConstructionPrecisionAndValidation,TestObservedSwingSpawnPickupDoesNotInventDeath | 104 |
| `youyuan_packet_ingress_test.go` | TestIngressDistinguishesFilteredFromUnreceived,TestIngressCancellationAndReconnectIsolation | 109 |
| `youyuan_perf50_bench_test.go` | — | 62 |
| `youyuan_perf50_compat_test.go` | TestPerf50EveryRegisteredPacketKeepsFullFieldShape,TestPerf50SubscriberChangesDuringDispatchKeepOriginalOrder | 114 |
| `youyuan_permission_probe_test.go` | TestPermissionProbeIgnoresUnrelatedCommandReceipt,TestExplicitMemberPacketOutranksCommandFallback | 57 |
| `youyuan_physical_input_test.go` | TestPhysicalInputContinuesHeartbeatTickAndRejectsReplacedConnection | 21 |
| `youyuan_player_actions_docs_test.go` | TestPlayerActionsResourceAndSkillMatch | 35 |
| `youyuan_player_experience_test.go` | TestPlayerExperienceUsesRealAttributesAndZeroIsKnown,TestPlayerExperienceValidatesBeforeWriting | 146 |
| `youyuan_plugin_delete_lifecycle_test.go` | TestDeletePluginClosesResidentBeforeRemovingFiles,TestDeleteReportsFilesystemFailureAndDoesNotPretendSuccess | 161 |
| `youyuan_plugin_mcp_test.go` | TestPluginMCPUsesActualServiceAndNoCredentials,TestR21AThinkingNoticePrecedesSlowWrapperAndDoesNotDuplicate | 221 |
| `youyuan_plugin_offline_stop_test.go` | TestOfflinePluginCanBeStopped | 33 |
| `youyuan_plugin_outputs_test.go` | TestPluginGeneratedOutputsPreserveBinaryAndRejectOtherAccounts,TestResourceOutputAPIUsesPluginDataAndRuntimeAccount | 148 |
| `youyuan_primary_account_test.go` | TestPrimaryFirstActualSaveUsesMainWithoutProcessOrAutoConnect,TestPrimaryConcurrentFirstSavesHaveOneMain | 270 |
| `youyuan_product_user_agent_test.go` | TestProductUserAgentOnOwnedHTTPAndRemoteHandshake,TestRemoteDialHeadersDevice | 138 |
| `youyuan_protocol_samples_test.go` | TestProtocolSamplesWhitelistOnceAndSizeLimit | 54 |
| `youyuan_public_urls_test.go` | TestPublicURLsKeepDeviceButNeverCredentials | 21 |
| `youyuan_publish_manifest_test.go` | TestSDKPublishPreservesMetadata | 28 |
| `youyuan_python_test.go` | TestPythonObservationNamespaceInjectedBeforeSourceRuns,TestYouYuanPythonGameEventDispatch | 83 |
| `youyuan_qq_actions_test.go` | TestYYQQNumericQQ,TestYYQQActionPolicy | 71 |
| `youyuan_qq_affinity_test.go` | TestQQAffPraiseRaisesAndChatCapped,TestQQAffInsultDropsToAngry | 284 |
| `youyuan_qq_ai_live_test.go` | TestQQCasualChatRealProvider | 61 |
| `youyuan_qq_approval_test.go` | TestApprNonSuperAdminCannotApprove,TestApprSuperAdminApproveExecutes | 311 |
| `youyuan_qq_assistant_test.go` | TestQQAssistantAuthorizationAndPersistence,TestQQAssistantWrongPlayerCannotConfirmBinding | 230 |
| `youyuan_qq_caps_limit_test.go` | TestCapLimitDefaultsWhenNoOverride,TestCapLimitOverrideOnlyTouchesGivenField | 102 |
| `youyuan_qq_claim_test.go` | TestQQInboundClaimSurvivesRestart,TestQQTriggerActionNotReplayedAfterReconnect | 171 |
| `youyuan_qq_commands_test.go` | TestCommandAliasOverride,TestCommandAliasEmptyFallsBack | 240 |
| `youyuan_qq_companion_test.go` | TestQQCompanionAliveSurvivesGlobalSweep,TestQQCompanionAliveRejectsEpochReset | 82 |
| `youyuan_qq_context_test.go` | TestQQRecentContextIsBoundedAndGroupIsolated,TestQQGroupLogUsesReceivedHistoryAndHidesMemberCoordinates | 163 |
| `youyuan_qq_forward_test.go` | TestForwardBuildNodeStructure,TestForwardBuildRejectsEmptyInput | 253 |
| `youyuan_qq_gateway_test.go` | TestQQOfficialNumericGroupAndPreservedBinding,TestQQOfficialUnsignedValidationAndSignedEvents | 114 |
| `youyuan_qq_groups_test.go` | TestQQMultipleGroupIsolationAndExplicitEmpty,TestQQGroupEnrollmentWriteFailureAndOneUse | 66 |
| `youyuan_qq_guard_test.go` | TestDangerDefaultBlocksLocked,TestDangerDisabledRuleAllows | 137 |
| `youyuan_qq_help_test.go` | TestHelpRegistryDefaultsAllOn,TestHelpSwitchPersistsAndHidesEntry | 297 |
| `youyuan_qq_identity_test.go` | TestOfficialQQIdentityValidation,TestAuthorizeOnlySeenOfficialMember | 44 |
| `youyuan_qq_image_queue_test.go` | TestYYImgQueueFIFOOrder,TestYYImgQueueCoalesceSameUser | 351 |
| `youyuan_qq_inbound_test.go` | TestYYQQParseInboundThreeFormsTextEquivalence,TestYYQQParseInboundRejectsNonMessageEvents | 592 |
| `youyuan_qq_interact_test.go` | TestYYVisionImagePart,TestYYVisionExtractURLs | 239 |
| `youyuan_qq_level_test.go` | TestQQLevelOnMessageXPCooldownLevelUpAnnounceAndSync,TestQQLevelCurveMathWithFractionalFactor | 467 |
| `youyuan_qq_liveness_test.go` | TestQQLivenessNeverProbed,TestQQLivenessFreshSuccess | 178 |
| `youyuan_qq_member_test.go` | TestYYMemberOnMessageUpsert,TestYYMemberAliasLearning | 534 |
| `youyuan_qq_memory_test.go` | TestQQStructuredIdentitySurvivesSameNameAndReply,TestQQStableIDsPreserveLargeOneBotNumbers | 394 |
| `youyuan_qq_moderation_test.go` | TestModerationEmptyRulesDoNotBlock,TestModerationNormalizationVariants | 496 |
| `youyuan_qq_permissions_test.go` | TestQQPermissionChecksEveryCommand,TestQQUserIDsAndMessageTemplates | 53 |
| `youyuan_qq_persona_test.go` | TestYYPersonaPromptBlockHasAllDefaultRules,TestYYPersonaPromptBlockTruncate | 413 |
| `youyuan_qq_proactive_test.go` | TestProactiveSilenceScoreMonotonic,TestProactiveCooldownHardBlock | 415 |
| `youyuan_qq_quote_test.go` | TestQQQuoteStripCQ,TestQQQuoteFromRows | 166 |
| `youyuan_qq_rag_test.go` | TestYYRagIngestPersistReloadAndSearch,TestYYRagChatIngestScopeIsolationAndPerDocCap | 506 |
| `youyuan_qq_selection_test.go` | TestQQPlayerSelectionIsolation,TestQQCompanionTimeoutAndOwner | 80 |
| `youyuan_qq_serverinfo_test.go` | TestSrvInfoInfoWithoutConfig,TestSrvInfoPlayersFromStats | 337 |
| `youyuan_qq_skill_test.go` | TestYYGameSkillBuiltinTableComplete,TestYYGameSkillParamValidation | 674 |
| `youyuan_qq_skills_test.go` | TestSkillPromptBlockContainsWord,TestSkillDisabledHidden | 408 |
| `youyuan_qq_triggers_test.go` | TestYYQQTriggerMatchKeyword,TestYYQQTriggerMatchRegex | 168 |
| `youyuan_qq_usage_test.go` | TestYYUsageRealAndEstimatedSplitAndPersist,TestYYUsageDayAggregationRingAndFlushThreshold | 350 |
| `youyuan_qq_vision_test.go` | TestVisionThreeState,TestVisionUnescapeRestoresSig | 114 |
| `youyuan_qq_voice_test.go` | TestVoiceURLAllow,TestVoiceModeNormalize | 202 |
| `youyuan_qq_web_test.go` | TestWebCheckURLRejectsInternal,TestWebIPAllowed | 67 |
| `youyuan_receive_continuity_test.go` | TestReceiveChatContinuesWhenTerminalQueueIsFull | 49 |
| `youyuan_received_plugin_test.go` | TestReceivedR20BCompileAndTimezone | 40 |
| `youyuan_received_r21b_test.go` | TestReceivedR21BDeathAndThinking | 74 |
| `youyuan_remote_admission_test.go` | TestRemoteAdmissionSaturationRejectsWithoutWaitingAndReleaseRestoresCapacity,TestRemoteAdmissionRejectionBacklogIsBoundedAndClosesConnection | 348 |
| `youyuan_remote_device_wire_test.go` | TestRemoteDeviceSelectorOnWire | 48 |
| `youyuan_resident_test.go` | TestYouYuanResidentKeepsStateAndSeparatesBots,TestBotReadyReplayedAfterLateLoadAndOncePerRuntime | 135 |
| `youyuan_resource_test.go` | TestReimportRefreshesInstalledMetadata,TestResourceImportModuleAndFullRoundTrip | 104 |
| `youyuan_review_reply_test.go` | TestReviewReplySplitHandlesBlankLinesAndNestedUTF8Iteration,TestReviewLuaConcatReceiver | 102 |
| `youyuan_room_discovery_test.go` | TestRoomDiscoverySelectionBoundToCredentialAndInput,TestRoomCandidateRejectsCrossTypeAndPrecisionLoss | 79 |
| `youyuan_rpc_ingress_test.go` | TestRPCIngressCaptureOptInAndPayloadExclusion,TestRPCIngressUnknownShapesDoNotInventEvents | 115 |
| `youyuan_runtime_contention_test.go` | TestRuntimeListSlowClientCannotBlockEventDelivery,TestRuntimeListPreservesCatalogCountersAndQueueCapacity | 180 |
| `youyuan_runtime_export_test.go` | TestRuntimeExportKeepsNestedFilesImportable | 61 |
| `youyuan_runtime_management_test.go` | TestRuntimeDeletionCannotEscapeAndLeavesOtherPlugin | 35 |
| `youyuan_scheduler_live_test.go` | TestLinuxSchedulerLiveMeasurement | 135 |
| `youyuan_scheduler_test.go` | TestSchedulerAcceptsSubMillisecondWithoutArtificialHzCeiling,TestChainFrequencyKeepsSubMillisecondWholeRoundPeriod | 142 |
| `youyuan_sdk_test.go` | TestSDKRelativeFilesUseActualRuntimeDirectories,TestSDKBindingPersistenceAndErrors | 165 |
| `youyuan_security_hardening_test.go` | TestValidPluginIDRejectsScriptInjectionChars,TestPreviewSaveImageRejectsTraversalAndNonImage | 131 |
| `youyuan_seed_api_test.go` | TestSeedJavaRandomVector,TestSeedSlimeDeterministic | 317 |
| `youyuan_seed_biome_diag_test.go` | TestDiagClimate | 104 |
| `youyuan_seed_biome_test.go` | TestBiomeDeterministic,TestBiomeOceanSharePlausible | 268 |
| `youyuan_seed_lod_test.go` | TestSeedLODFarSampled,TestSeedLODMidLimit | 264 |
| `youyuan_seed_structures_test.go` | TestStructureRegistryComplete,TestStructureAliases | 460 |
| `youyuan_seed_world_test.go` | TestWorldSeedExplicitSeed,TestWorldSeedNoConnection | 152 |
| `youyuan_seedv2_routes_test.go` | TestSeedV2ManifestRoutes,TestSeedV2VillageAnchor | 418 |
| `youyuan_sense_test.go` | TestCommandAwaitKeepsBufferedReplyAfterDeadline,TestSenseUnknownHealthDeathDedupRespawn | 258 |
| `youyuan_server_observation_test.go` | TestServerObservationHealthSessionAndProvenance,TestServerObservationHealthRejectsDisplayAndInvalid | 137 |
| `youyuan_skill_export_regression_test.go` | TestEveryPublishedSkillExportsCompleteZIP | 23 |
| `youyuan_skill_library_test.go` | TestRobotCooperationGuidesIncludedInSelectableLanguageSkills,TestSkillLibraryPlayerChoicePersistenceAndIsolation | 180 |
| `youyuan_skin_account_media_test.go` | TestSkinAccountTextureServesCachedByAccountID,TestSkinAccountResolveCredentialFallback | 166 |
| `youyuan_skin_cache_test.go` | TestSkinRejectsNonWhitelistedHost,TestSkinRejectsInternalAddress | 276 |
| `youyuan_skull_test.go` | TestSkullParserAndImportDispatch | 31 |
| `youyuan_snowball_incident_test.go` | TestSnowballIncidentShapeAndSingleRoundOrder,TestSnowballIncidentFastConditionalBarrier | 129 |
| `youyuan_snowball_owner_test.go` | TestSnowballStartupPreservesNativeOwnerAndStoppedState,TestSnowballLuaUnknownProbeDoesNotRunMenuOrReplayFailedBatch | 63 |
| `youyuan_spawn_state_test.go` | TestReconnectRestoresSpawnDimensionAndRotationWithoutMovement | 23 |
| `youyuan_sse_delivery_test.go` | TestTerminalTaskEventSurvivesQueuedProgress | 20 |
| `youyuan_status_latency_test.go` | TestHandleBotStatusDoesNotHoldManagerLockWhileWriting,TestHandleBotStatusReportsUnknownGameRTTWhenDisconnected | 105 |
| `youyuan_structure_hud_test.go` | TestStructureHudMatchesHouseLayout,TestStructureHudNeverInventsTPSOrRTT | 154 |
| `youyuan_surface_positioned_test.go` | TestSurfacePositionedCandidatePreserves111ChecksAndClock,TestSurfacePositionedStableColumnsMatchOriginalAndMoveOnce | 81 |
| `youyuan_tanlobby_cancel_test.go` | TestTanLobbyManualStopCancelsAuthAndRetryWait | 68 |
| `youyuan_task_recovery_test.go` | TestImportStopAndDisconnectRemainResumable,TestStoppingTaskHoldsSlotUntilRunnerExits | 113 |
| `youyuan_terminal_test.go` | TestOutgoingChatReadableAndBounded | 22 |
| `youyuan_theme_samples_test.go` | TestSampleThemeAndPluginValid | 45 |
| `youyuan_theme_test.go` | TestYYThemeGuardRejectsBrandRemoval,TestYYThemeValidateManifest | 395 |
| `youyuan_timezone_test.go` | TestChatLogHistoricalUTCDisplaysBeijing | 24 |
| `youyuan_tunnel_test.go` | TestTunnelExposesMCPAndBlocksPrivateWebAPI | 28 |
| `youyuan_v097_test.go` | TestBlockVideoDeltaAndPlanes,TestStructureConvertRoundTrip | 150 |
| `youyuan_version_alignment_test.go` | TestProductVersionMatchesAndroidUpdateCode | 26 |
| `youyuan_video_pixels_test.go` | TestVideoBatchSequenceAndAtomicRollback,TestVideoRectanglesPreserveEveryChangedPixel | 123 |
| `youyuan_visibility_test.go` | TestVisibilityDefaultCaptureKeepsConsecutiveSwings,TestVisibilityFarPacketsHaveNoHostDistanceFilter | 78 |
| `youyuan_world_combat_consumer_test.go` | TestWorldCombatRetainsHitsDuringCameraSpawnChurn,TestWorldCombatLuaConsumesEndHitsOnceWithoutLocalCacheMixing | 81 |
| `youyuan_world_observer_test.go` | TestWorldFarHealthMergePreservesLocalUnknown,TestWorldIsolationRemovalAndAmbiguity | 200 |
---

## 6. 覆盖对账

- `ls upstream/*.go | wc -l` = **646**
- 其中 `_test.go` = **294**（见第 5 节，逐条列出），非测试实现文件 = **352**（见第 3 节，逐条列出）
- `grep -h "^package " upstream/*.go | sort -u` 只有 `package main` 一行，即 646 个文件同属一个包
- 非测试代码合计 112285 行，测试代码合计 40516 行（`wc -l` 实测）

**共 646 个 .go，本表覆盖 646 个**（第 3 节 352 个非测试文件 + 第 5 节 294 个测试文件）。
