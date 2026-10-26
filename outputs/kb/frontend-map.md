# 前端模块图（frontend-map）

> 项目：游园工具箱 / MCORT（Minecraft Open Robot Toolbox）
> 源码：`C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend/`
> 部署副本：`C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/cloudflare/public/`
> 技术栈：原生 JS + 全局函数（无框架、无打包器、无模块系统），靠 `index.html` 的 `<script>` 顺序 + 全局变量覆盖实现"插件化"。
> 本文件只读源码生成，未修改任何 js/css/html。

---

## 0. 速查

| 指标 | 数值 | 口径 |
|---|---|---|
| js 文件总数 | **112** | `js/*.js` 76 + `js/features/*.js` 28 + `js/pages/*.js` 5 + `js/vendor/*.js` 3 |
| 被 index.html 加载的 js | **111** | 唯一未加载：`js/mcort_theme.js` |
| `<script src>` 出现次数 | **111** | 与上一致 |
| css 文件总数 | **43** | `css/*.css` |
| 被 index.html 引用的 css | **41** | 未引用：`css/mcort_theme.css`、`css/style.css` |
| `<link href="/css/...">` 出现次数 | **41** | 与上一致 |
| 一级导航 tab | **5** 固定 + **1** 运行时插入（互通） | 见 §4.1 |
| 工作台工具模块（`yyModules`） | 静态 **23** + 动态注册 **5** = 28 | 见 §4.2 / §4.3 |
| 全局入口函数（顶层 `function` / `window.X=`） | **1653** 个定义 | 见 §8 统计方法 |
| 零调用全局函数 | **45** 个确认零调用（53 个候选中排除 8 个外部回调/注册表） | 见 §8 |

---

## 1. 目录与容器结构

```
frontend/
├── index.html                 188 行；唯一入口页（SPA 壳）
├── draw.html / piano.html / cap_test.html   独立测试页（不被 index 引用）
├── css/    43 个
├── js/
│   ├── 76 个根级脚本（内核、页面、各业务面板）
│   ├── pages/     5 个：home / tasks / terminal / settings / plugins（一级 tab 渲染器）
│   ├── features/  28 个：二级功能页（导入导出、地图画、音乐、相机…）
│   └── vendor/    3 个：marked.min.js / three.min.js / youyuan_editor.js
└── *.js.esb   —— 构建中间产物（**不是**源码，浏览器不加载，勿改）
```

### 1.1 页面容器 id 命名规律

`index.html` 只提供 6 个空容器，所有内容靠 JS 注入：

| 容器 id | class | 用途 | 注入者 |
|---|---|---|---|
| `#tab-home` | `.content.show` | 一级页「首页/工作台」 | `Rhome()` |
| `#tab-tasks` | `.content` | 一级页「插件」 | `Rplg()` |
| `#tab-terminal` | `.content` | 一级页「终端」 | `Rconsole()` |
| `#tab-settings` | `.content` | 一级页「设置」 | `Rcfg()` |
| `#tab-accounts` | `.content` | 一级页「账号」 | `Raccounts()` |
| `#sub-page` | `.sub-page` | **所有二级页共用的唯一容器** | `openSubPage(title, html)` |

规律：`#tab-<name>` + `#tab-<name>-inner`（`index.html:60-64`）。二级页没有独立容器，全靠 `openSubPage()` 往 `#sub-page` 灌 HTML，并用 `subPageCache[title]` 缓存骨架（`app.js:110-140`）。因此**二级页之间是同容器替换关系**，标题字符串就是缓存 key——改标题等于换缓存槽。

### 1.2 全局状态（`app.js:1-16`）

| 变量 | 含义 | 注意 |
|---|---|---|
| `curTab` | 当前一级 tab，初始 `'home'` | 多处 `live()` 校验依赖它 |
| `TAB_ORDER` | `['home','tasks','terminal','settings']` | **不含 `accounts`**；`accounts.js:428` 运行时 `push` 进去 |
| `tabInited` | 一级 tab 是否已首渲染 | `tabTo` 只对未初始化 tab 调渲染器 |
| `activeRenderers` | 当前页注册的清理函数 | 切页/开子页时全部执行并清空 |
| `subPageOpen` / `subPageStack` / `subPageCache` | 二级页状态栈与 HTML 缓存 | 缓存 key = 标题 |
| `subPageInits` | `subPageInit(title, fn)` 注册表 | 缓存命中后必须重跑，否则是空壳（`app.js:13-16` 注释即历史事故） |

### 1.3 全局命名约定

| 前缀 | 含义 | 例 |
|---|---|---|
| `R<name>()` | **页面级渲染器**，往某个容器整块 innerHTML | `Rhome` `Rcfg` `RimpHTML` `Rqq` |
| `R<name>HTML()` | 返回 HTML 字符串（不自己开页） | `RimpHTML` `RflyHTML` `RcameraHTML` |
| `yy*Page()` | 打开一个二级页（内部调 `openSubPage`） | `yyQQPage` `yySeedPage` `yyConverterPage` |
| `yy*()` | 内核/业务函数 | `yyOpenModule` `yyNavigate` `yyPollDefine` |
| `do*()` | 表单动作（读 DOM → 调接口） | `doConn` `doImp` `doMa` |
| `plg*` / `pm*` / `bm*` / `qb*` / `ysm*` / `im*` / `dbg*` | 各功能域私有前缀 | `plgImport` `pmOpenMarket` `bmOpenMarket` `qbTplSave` `ysmDraw` `imAddItem` `dbgPollStatus` |

---

## 2. index.html 加载顺序（按行号）

加载顺序**就是**本项目的依赖注入顺序：后加载的脚本通过"保存旧函数引用 + 重新赋值"来包装前一个（见 §5 覆盖链）。改动任何一行的位置都可能改变最终行为。

### 阶段 A — 首帧主题 + 41 个 CSS（`index.html:6-43`）

- `index.html:6-7` 内联脚本：在 `<head>` 最早按 `localStorage['youyuan_theme']` 写 `data-theme`，避免深色用户首屏闪白。
- `index.html:10-43` 41 个 `<link rel="stylesheet">`。顺序上分三层：基础层（`base` → `layout` → `components`）→ 设计层（`youyuan_design`）→ 业务层（其余 37 个按模块排列）。

### 阶段 B — 内核（`index.html:80-89`）

| 行 | 脚本 | 作用 |
|---|---|---|
| 80 | 内联 `window.onerror` | 全局错误只打印不阻断 |
| 81 | `compat.js` | **必须最先**：补齐 `replaceChildren` / `Object.hasOwn` / `replaceAll` 等运行时 API（旧 WebView 没有） |
| 82 | `theme.js` | `initTheme/setTheme/syncStatusBarTheme` |
| 83 | `youyuan_console_token.js` | 包装全局 `fetch`，给所有同源 `/api/*` 注入 `X-YouYuan-Console-Token` |
| 84 | `youyuan_boot.js` | 云端同源口令 / device 参数归一 |
| 85 | `youyuan_account_boot.js` | 包装 `fetch`（多账号路由）+ `EventSource` 垫片（3s 长轮询替身） |
| 86 | `youyuan_poll_policy.js` | `yyPollDefine` 统一轮询注册表（额度护栏） |
| 87 | `api.js` | `A(m,p,b)` 统一请求 + 2s 读缓存 + `APrism` |
| 88 | `utils.js` | `T/L/U/I/C`、`escHtml`、文件选择、弹窗、`prismDropdown` |
| 89 | `sse.js` | `yyEventsChannel` 事件流 |

> 注意 83/85 两个脚本**都**包装了 `window.fetch`，顺序决定谁在外层：console_token 先包，account_boot 后包 → 最终调用链是 `account_boot → console_token → 原生 fetch`。所以账号路由前缀在 token 注入之前拼接。

### 阶段 C — 一级页渲染器（`index.html:90-94`）

`pages/home.js` → `pages/tasks.js` → `pages/terminal.js` → `pages/settings.js` → `pages/plugins.js`
（依次定义 `Rhome` / `Rtsk` / `Rconsole` / `Rcfg` / `Rplg`）

### 阶段 D — features 功能页（`index.html:95-121`，27 个）

`filepicker` → `helpers` → `building` → `mapart` → `drawing` → `music` → `skin` → `mapplayer` → `announcements` → `intro_video` → `notifications` → `building_mgmt` → `others` → `hq_query` → `seed_map` → `migrate_lite` → `cb_engineering` → `itemmaker` → `camera` → `mcfunction` → `fly` → `personalize` → `plugin_mgmt` → `plugin_market` → `ai_assist` → `app.js` → `preview`

> `app.js` 夹在 features 中间（第 120 行）——它必须晚于 `utils.js`/`api.js`，又早于 `preview.js`（`preview.js` 用到 `openSubPage` 的缓存语义）。

### 阶段 E — vendor（`index.html:123-125`）

`marked.min.js` → `three.min.js` → 内联 `console.log` 自检（`THREE` / `marked` / `RmusicHTML` / `openSubPage` / `tabTo` 是否就绪）。

### 阶段 F — 自建内核与业务（`index.html:127-184`，58 个）

| 行 | 脚本 | 备注 |
|---|---|---|
| 127 | `features/qq_bridge.js`、`features/qq_template.js`、`youyuan.js` | `youyuan.js` 首次定义 `yyRemotePage/yyLanguagePage/yyQQPage` 并**包装** `Rhome`/`Rplg`（见 §5 陷阱 1） |
| 128 | 内联：`yyOriginalPluginMarket=pmOpenMarket` 等 | 保存三个市场入口的原函数，供 `settings.js:20` 还原 |
| 129 | `youyuan_library.js` | |
| 130 | `vendor/youyuan_editor.js` | 代码编辑器（`YYEditor`） |
| 131 | `youyuan_dashboard.js` | **工作台核心**：`yyModules` 注册表、`yyShell()`、`Rhome` 重写、`tabTo` 包装 |
| 132 | `youyuan_settings.js` | **`yyModules` 被替换为 13 项白名单副本**（§5 陷阱 2）、`Rcfg` 重写 |
| 133-137 | `youyuan_chain_upgrade` / `chain_center` / `workspace` / `social` / `skin_media` | |
| 138 | `youyuan_accounts.js` | `Raccounts` + 账号池 + 皮肤库；`TAB_ORDER.push('accounts')` |
| 139-147 | `skin_library` / `appearance` / `agent_profiles` / `agent_attachments` / `skill_library` / `execution_options` / `upload` / `usability` / `sdk` | 多个在这里再包一层 `Rcfg`/`Rhome` |
| 148-150 | `sense` / `identity` / `identity_recovery` | `identity.js` 两次覆盖 `Rcfg`（345、438） |
| 151-152 | `exports` / `export_center` | 都覆盖 `Raccounts`/`Rplg` |
| 153-156 | `block_video` / `converters` / `qq_diagnostics` / `model_brand` | |
| 157-164 | `qq_assistant` / `qq_memory` / `qq_groups` / `qq_console` / `qq_config` / `qq_member_ui` / `qq_knowledge` / `qq_serverinfo` | `qq_console.js:3065` **覆盖 `yyQQPage`** |
| 165-175 | `skin_viewer` / `game_auth` / `host_files` / `tunnel` / `mcp_scope` / `mcp_tutorial` / `agent_layout` / `reference` / `connections` / `connection_policy` / `connection_diagnostics` | |
| 176-180 | `global_capture` / `seed_local` / `seed_page` / `seedmap_v2` / `room_discovery` | 前四个都会向 `yyModules` 动态注册 |
| 181 | `settings_groups` / `studio` / `terminal` / `agent_studio` / `agent_workspace_ui` / `plugin_studio` / `game_controls` / `market` / `navigation` + 内联 `initPersonalize();tabTo('home')` | **真正的首屏启动点**在这一行末尾 |
| 182-184 | `qq_assistant_tools` / `qq_guard` / `qq_moderation` | |
| 187 | `youyuan_boot_stable.js` | 首屏稳定层：必须最后，包住最终的 `Rhome`/`A`/`yyOpenModule` |

---

## 3. 模块总表（js 文件 → 页面/面板 → 全局入口 → 接口 → css）

> 说明：`字节` 取 `ls -l` 精确值；`主要全局入口` 只列页面级/可被 UI 触发的函数，域内私有函数不展开；`接口` 为该文件里出现的 `/api/*` 路径前缀（每文件最多 5 条）。

### 3.1 根目录 `js/*.js`（76 个）

| # | 文件（字节） | 负责的页面/面板 | 主要全局入口 | 依赖接口 | 对应 css |
|---|---|---|---|---|---|
| 1 | `api.js` (4312) | 全局请求层（无 UI） | `A` `APrism` `yyInvalidateReads` `yyReadScope` | 所有 `/api/*`；`/api/prism-proxy` | — |
| 2 | `app.js` (20543) | SPA 壳：tab 切换、二级页容器、导航滑片、返回键 | `tabTo` `openSubPage` `closeSubPage` `subPageInit` `refreshHomeTasks` `__onBackPressed__` | `/api/config` `/api/task/*` `/api/version/check` | `base` `layout` `components` |
| 3 | `compat.js` (3263) | 旧内核兼容垫片（最先加载） | 无导出，直接改写内置原型 | — | — |
| 4 | `mcort_theme.js` (24536) | MCORT 主题系统（整文件未被加载） | `window.mcortTheme` | `/api/mcort/theme*` `/api/mcort/plugins` | `mcort_theme.css`（同样未链接） |
| 5 | `sse.js` (19136) | 事件流通道 | `yyEventsChannel` | `/api/youyuan/events` `/api/bot/status` | — |
| 6 | `theme.js` (2722) | 明暗/配色切换、状态栏 | `initTheme` `setTheme` `syncStatusBarTheme` | — | `youyuan_design.css` |
| 7 | `utils.js` (24396) | 全局工具：toast、loading、连接状态、文件选择、确认框 | `T` `L` `U` `I` `C` `escHtml` `pick` `fpicker` `pickBySetting` `showPrompt` `showConfirm` `yyShowAlertModal` `prismDropdown` | `/api/file/upload` | `components.css` |
| 8 | `youyuan.js` (11305) | 自建内核旧版页：公网远控 / 常驻插件 / 群服互通 / API 目录 | `yyRemotePage` `yyLanguagePage` `yyQQPage` `yyCatalogPage` `yyQQSave` `yyImportPython` | `/api/youyuan/remote` `/api/youyuan/catalog` `/api/qq/*` | `youyuan_design.css` |
| 9 | `youyuan_account_boot.js` (2987) | 多账号路由（`fetch` 前缀改写 + `EventSource` 垫片） | `yyAccountPath`；改写 `window.fetch` / `window.EventSource` | `/api/youyuan/accounts/invoke/*` | — |
| 10 | `youyuan_accounts.js` (176413) | 一级 tab「账号」+ 账号池 + 皮肤库 + 巡检 | `Raccounts` `yyEnterAccount` `yyPoolPanel` `yyPoolDetail` `yySkinLibraryOpen` `yyStartPatrolUI` `yyAcctPickerOpen` | `/api/accounts` `/api/bot/*` `/api/youyuan/accounts*` | `youyuan_accounts.css` `youyuan_account_pool.css` |
| 11 | `youyuan_agent_attachments.js` (7510) | 工作区对话附件/粘贴 | `yyAttachAgentFiles` `yyPasteAgentClipboard` `yyAgentClipboardText` | `/api/youyuan/agent/upload` | `yy_agent_workspace.css` |
| 12 | `youyuan_agent_layout.js` (998) | 工作区打开时切到 Agent 版式 + 恢复钩子 | 包装 `yyOpenWorkspace`；`__subPageRestoreHooks` | — | `youyuan_agent_layout.css` |
| 13 | `youyuan_agent_profiles.js` (20955) | AI 助手档案、多标签会话、用量 | `yyAgentProfileModal` `yyAgentPreferences` `yyNewAgentSession` `yySavedAgentSessions` `yyRenderOpenAgentTabs` `yyUseCloudModel` | `/api/youyuan/agent/*` | `youyuan_agent_layout.css` |
| 14 | `youyuan_agent_studio.js` (11956) | 工作区 Studio 面板挂载、运行预算弹窗 | 包装 `yyOpenWorkspace` / `yyWorkspacePane`；`window.yyAgentRunBudgetDialog` | `/api/youyuan/agent/run-budget` | `youyuan_agent_studio.css` |
| 15 | `youyuan_agent_workspace_ui.js` (10376) | 工作区手机端版式（任务详情、输入区随键盘收缩） | 包装 `yyOpenWorkspace`（无新全局名） | — | `yy_agent_workspace.css` |
| 16 | `youyuan_appearance.js` (5118) | 「背景与封面」二级页 | `yyAppearancePage` `yyApplyAppearance` `yyUploadAppearance` `yyCoverShow` | `/api/youyuan/appearance*` | `youyuan_design.css` |
| 17 | `youyuan_block_video.js` (7706) | 「视频方块化」二级页 | `yyBlockVideoPage` `yyBlockVideoConvert` `yyBlockVideoCancel` `yyDownloadArtifact` | `/api/youyuan/media/block-video` `/api/youyuan/chains` | — |
| 18 | `youyuan_boot.js` (7969) | 云端同源口令 / 设备选择 / 云被拦提示 | `yyCloudBlocked` `yyDiscoverLinux` `yyLinuxDirectOrigin` `yyLinuxTransportStatus` | `/api/cloud/*` `/api/identity/*` | — |
| 19 | `youyuan_boot_stable.js` (6292) | 首屏稳定层：合并重复 `Rhome`/`A`/`yyOpenModule` 调用 | 包装 `Rhome` `A` `yyOpenModule` | `/api/qq/config` `/api/qq/status` | `youyuan_boot_stable.css` |
| 20 | `youyuan_chain_center.js` (14767) | 命令链中心（总控页） | `window.yyChainsPage` `yyChainsRefresh` `yyDrawChains` `yyChainCenterRefresh` `yyChainCenterStopAll` | `/api/youyuan/chains` | `youyuan_chain_center.css` |
| 21 | `youyuan_chain_upgrade.js` (14612) | 命令链编辑增强（插入/分支/等待） | `yyChainControlPanel` `yyChainInsertText` `yyChainInsertCall` `yyChainInsertWait` `yyChainInsertBranch` | `/api/youyuan/chains` | `youyuan_chain_center.css` |
| 22 | `youyuan_connection_diagnostics.js` (12536) | 连接诊断面板（掉线原因/重连） | `yyRefreshConnDiag` `yyConnDiagToggle` | `/api/youyuan/connection-diagnostics` | `youyuan_connection_diagnostics.css` |
| 23 | `youyuan_connection_policy.js` (14264) | 连接策略卡 + 服务器信息轮询；覆盖 `Rcfg`(179) | `yyChangeConnectionPolicy` `yyDrawConnectionPolicy` `yyRefreshServerInfoNow` `yyResumeReconnect` | `/api/youyuan/connection-policy` `/api/bot/status` | `youyuan_connection_policy.css` |
| 24 | `youyuan_connections.js` (7573) | 跨账号联通 / MCP 专用连接 / 平台密码 | `yyAccountBridgePage` `yyMCPLinksPage` `yyMCPLinkCreate` `yyPlatformPasswordPage` `yyConnectionRequest` | `/api/youyuan/mcp-links` `/api/youyuan/tunnel` | `youyuan_design.css` |
| 25 | `youyuan_console_token.js` (1597) | 给所有 `/api/*` 注入控制台 token（包装 `fetch`） | 无导出 | 全部 `/api/*` | — |
| 26 | `youyuan_converters.js` (21482) | 「结构与指令转换」二级页 | `yyConverterPage` `yyConvertStructure` `yyConvertCommands` `yyMakeCommandStructure` `yyConversionTab` | `/api/youyuan/convert/*` | `youyuan_design.css` |
| 27 | `youyuan_dashboard.js` (46905) | 工作台首页 + `yyModules` 注册表 + 侧边栏外壳 | `yyModules`(23 项) `yyOpenModule` `yyModule` `yyNavigate` `yyShell` `Rhome`(85) `yyDrawTools` `yyDrawFavorites` | `/api/youyuan/snapshot` `/api/bot/status` `/api/task/list` `/api/plugin/list` | `youyuan_design.css` `yy_home_workbench.css` |
| 28 | `youyuan_execution_options.js` (4675) | 插件执行参数面板 | `yyPluginCommandSettings` | `/api/youyuan/agent/file` | — |
| 29 | `youyuan_export_center.js` (16410) | 「导出中心」二级页；覆盖 `Rplg`(250) | `yyOpenExportCenter` `yyDownloadExport` `yyExportPluginZip` `yyLoadPluginOutputs` | `/api/youyuan/exports*` `/api/plugin/export` | `youyuan_design.css` |
| 30 | `youyuan_exports.js` (5657) | 观测/运行时插件导出；覆盖 `Raccounts`(41) `Rplg`(78) | `yyExportObservations` `yyExportRuntimePlugin` | `/api/youyuan/observations` `/api/youyuan/runtime/*` | — |
| 31 | `youyuan_game_auth.js` (28966) | 游戏账号注册 / 4399 登录 / 房间搜索；覆盖 `Rcfg`(263,380) | `yyOpenAccountRegistration` `yyGameAuthPage` `yyRoomSearchPage` `yyOpen4399Registration` | `/api/game-auth/accounts` `/api/youyuan/game-auth` | `youyuan_accounts.css` |
| 32 | `youyuan_game_controls.js` (12253) | 体感设置页 / 游戏出口设置页 | `yyExperiencePage` `yyGameEgressPage` | `/api/youyuan/game-egress` `/api/youyuan/player/experience` | `youyuan_game_controls.css` |
| 33 | `youyuan_global_capture.js` (42162) | 「全局抓包」页 + 多选/维度筛选；覆盖 `Rcfg`(74) | `yyGlobalCapturePage` `yyGlobalCaptureSimplePage` `yyGlobalMultiChoice` `yyServerPlayerLimitSettings` | `/api/youyuan/global-capture` `/api/players/list` | `youyuan_design.css` |
| 34 | `youyuan_host_files.js` (5569) | 「主机文件」浏览页 | `yyHostFilesPage` `yyRefreshHostFiles` `yyUploadHostFiles` | `/api/youyuan/files*` | `yy_file_browser.css` |
| 35 | `youyuan_identity.js` (35490) | 身份验证 / Turnstile+PoW / 订阅 / 平台账号；覆盖 `Rcfg`(345,438) | `yyShowIdentity` `yyIdentitySubmit` `yyPlatformPage` `yySubscriptionsPage` `yyResetPasswordPage` | `/api/identity/*` `/api/cloud/status` | `youyuan_design.css` |
| 36 | `youyuan_identity_recovery.js` (1436) | 云被拦时的身份兜底 | `yyCloudBlocked` | — | — |
| 37 | `youyuan_library.js` (3012) | 「云端资料库」二级页 | `yyLibraryPage` `yyLibraryUpload` `yyLibraryInstall` `yyLibraryDownload` | `/api/youyuan/library/*` | `youyuan_design.css` |
| 38 | `youyuan_market.js` (41761) | 「插件市场」页；动态注册 `market` 模块 | `yyMarketPage` `yyMarketInstall` `yyMarketUpload` `yyMarketView` `yyMarketTab` | `/api/market/*` | `youyuan_market.css` |
| 39 | `youyuan_mcp_scope.js` (14104) | MCP 工具权限范围面板 | `yyMcpScopeShow` `yyMcpScopeSave` `yyMcpScopeTest` `yyMcpScopeBoot` | `/api/youyuan/mcp/scope` | `youyuan_mcp_scope.css` |
| 40 | `youyuan_mcp_tutorial.js` (1117) | MCP 接入教程卡；包装 `yyRemotePage` | `yyMCPTutorial` | `/api/youyuan/docs` | `youyuan_design.css` |
| 41 | `youyuan_model_brand.js` (13212) | 模型品牌图标 + 自定义下拉（升级原生 select） | `yyBrandOf` `yyBrandRefresh` `yyBrandAttach` `yyBrandChipFallback` | `/api/ai/models` | `youyuan_model_brand.css` |
| 42 | `youyuan_navigation.js` (11474) | 侧边栏重建 / 手机二级导航条 / 「互通」一级入口 / 种子页懒加载 | `yySubNav` `yySubNavOpen` `yySubNavSync`；包装 `openSubPage` `tabTo` `yySyncNav` `Rcfg` | — | `youyuan_navigation.css` `youyuan_agent_mobile.css` |
| 43 | `youyuan_plugin_studio.js` (37332) | 插件工作室（在线写/调试插件） | `window.yyPluginStudioModel` | `/api/youyuan/runtime/list` `/api/youyuan/agent/workspaces` `/api/plugin/list` | `youyuan_plugin_studio.css` |
| 44 | `youyuan_poll_policy.js` (10697) | 轮询注册表（额度护栏：不可见即停） | `yyPollDefine` `yyPollStop` `yyPollKick` `yyApiHit` `yyPollGuardStats` | — | — |
| 45 | `youyuan_qq_assistant.js` (60579) | 群服互通「助手」折叠块（人格/模型/主动接话） | `yyQQAssistantSave` `yyQQAssistantDraft` `yyQQRefreshAll` `yyQQSaveModel` `yyQQResetCommandAliases` | `/api/qq/assistant*` `/api/ai/models` | `youyuan_qq_console.css` |
| 46 | `youyuan_qq_assistant_tools.js` (3582) | 助手工具与「领取」二维码 | `yyQQCreateClaim` | `/api/qq/assistant` `/api/qq/claim/qr` | `youyuan_qq_assistant_tools.css` |
| 47 | `youyuan_qq_config.js` (53415) | 触发词规则 + 生图配置（包装 `yyQQRenderAll`） | `__yyQQEditGuard` `__yyQQEditing` `__yyqCfgRegistered` | `/api/qq/triggers` `/api/qq/relay` `/api/qq/image` | `youyuan_qq_console.css` |
| 48 | `youyuan_qq_console.js` (151272) | 群服互通控制台主界面；`yyQQPage = yyQQConsolePage`(3065) | `yyQQConsolePage` `yyQQConsoleBody` `yyQQRender` `yyQQRenderAll` `yyQQOpenModule` `yyQQModuleShell` | `/api/qq/*` `/api/ai/models` | `youyuan_qq_console.css` `youyuan_qq_console_quicknav.css` |
| 49 | `youyuan_qq_diagnostics.js` (7563) | QQ 链路诊断 | `yyQQDiagnose` | `/api/qq/diagnostics` `/api/qq/verify` | `youyuan_qq_console.css` |
| 50 | `youyuan_qq_groups.js` (2916) | QQ 群列表/绑定 | `yyQQGroupRefresh` `yyQQGroupAdd` `yyQQGroupClaim` | `/api/qq/groups` | `youyuan_qq_console.css` |
| 51 | `youyuan_qq_guard.js` (7832) | 危险指令守卫（白名单/额外放行） | `yyGReload` `yyGSave` `yyGToggle` `yyGExtraAdd` `yyGExtraDel` `yyGFold` | `/api/qq/guard` | `youyuan_qq_console.css` |
| 52 | `youyuan_qq_knowledge.js` (14783) | 群知识库（树/检索/编辑） | `yyKOpen` `yyKSearch` `yyKPick` `yyKSave` `yyKDrag` `yyKDrop` | `/api/qq/kb/*` | `youyuan_qq_console.css` |
| 53 | `youyuan_qq_member_ui.js` (20852) | 群成员目录 UI | `YYQQDirectory` `YYQDHideAvatar` `yyQQRefreshMembers` | `/api/qq/assistant` `/api/qq/config` | `youyuan_qq_member_ui.css` |
| 54 | `youyuan_qq_memory.js` (13092) | 群记忆（读取/编辑/清理） | `yyQQMemoryMount` `yyQQMemorySave` `yyQQMemoryClear` `yyQQMemoryDraft` | `/api/qq/memory` | `youyuan_qq_console.css` |
| 55 | `youyuan_qq_moderation.js` (28360) | 管理模块：禁言词/规则 | `yyMEdit` `yyMSaveEdit` `yyMDel` `yyMTabooAdd` `yyMToggle` | `/api/qq/moderation` | `youyuan_qq_console.css` |
| 56 | `youyuan_qq_serverinfo.js` (33885) | 服务器信息面板（玩家卡/日志/事件） | `yySIPage` `yySIPlayer` `yySISearch` `yySISay` `yySIReload` | `/api/qq/server/*` `/api/qq/player-card` | `youyuan_qq_serverinfo.css` |
| 57 | `youyuan_reference.js` (8716) | 参考文档页 + 传输进度 | `yyReferenceRenderList` `yyReferenceMarkdown` `yyTransferProgress` `yyTransferClock` | `/api/youyuan/docs` | `youyuan_reference.css` |
| 58 | `youyuan_room_discovery.js` (17737) | 房间发现（局域网/租赁服扫描） | `window.yyRoomDiscovery`（`discover` / `render` / `renderCurrent`） | `/api/youyuan/rooms` `/api/bot/status` | `youyuan_room_discovery.css` |
| 59 | `youyuan_sdk.js` (2428) | 插件 SDK 面板；包装 `Rplg`(2) 与 `yyRuntimeAction`(3) | `yySDKOpenPanel` | `/api/youyuan/sdk/*` `/api/youyuan/runtime/stop` | — |
| 60 | `youyuan_seed_local.js` (16186) | 种子本机算法（由导航懒加载，无 UI） | 无 window 导出，供 `youyuan_seed_page.js` 调用 | — | `youyuan_seed_page.css` |
| 61 | `youyuan_seed_page.js` (44011) | 「种子查询」页；动态注册 `seed` 模块 | `yySeedPage` | `/api/youyuan/world/seed` | `youyuan_seed_page.css` |
| 62 | `youyuan_seedmap_v2.js` (91467) | 「种子地图」v2（拖拽缩放/结构叠加）；动态注册 `seedmap2` | `yySeedMapPage` | `/api/seed/*` `/api/seed/v2/*` | `youyuan_seedmap_v2.css` |
| 63 | `youyuan_sense.js` (35352) | 感知/观测/抓包视图/团队协作；覆盖 `Rcfg`(2) | `yySensePage` `yySensePoll` `yyCaptureView` `yyTeamUI` `yyObserverPlanUI` | `/api/youyuan/packets/*` `/api/youyuan/account-pool` | `youyuan_design.css` |
| 64 | `youyuan_settings.js` (40794) | 设置页主渲染（覆盖 `Rcfg`(25)）+ 反馈/命令链/机器人概览；**替换 `yyModules` 为 13 项白名单** | `yyFeedbackPage` `yyChainsPage` `yySendFeedback` `yyRenderBotOverview` `yyOpenModelLibrary` | `/api/config` `/api/bot/*` `/api/fleet/*` | `youyuan_design.css` |
| 65 | `youyuan_settings_groups.js` (5345) | 设置页卡片折叠分组（纯 DOM 后处理） | 无导出（IIFE + MutationObserver） | — | `youyuan_settings_groups.css` |
| 66 | `youyuan_skill_library.js` (7463) | 技能库面板 | `yyOpenSkillLibrary` | `/api/youyuan/agent/skills` `/api/youyuan/skills/*` | `youyuan_agent_studio.css` |
| 67 | `youyuan_skin_library.js` (14402) | 皮肤库选择器（组件） | `yySkinLibraryOpen` `yySkinLibraryRender` `yySkinLibraryApply` `yySkinLibraryUse` `yySkinTurnPaint` | `/api/skin/item-library` `/api/skin/item-texture` | `youyuan_skin_library.css` |
| 68 | `youyuan_skin_media.js` (5719) | 皮肤媒体 URL 解析（头像/材质代理） | `YYSkinMediaURL` `YYSkinMediaMount` `YYSkinMediaReset` | `/api/skin/*` `/api/qq/player-card` | `youyuan_accounts.css` |
| 69 | `youyuan_skin_viewer.js` (5817) | 皮肤 3D 预览（纯 CSS 六面，无 WebGL） | `YYSkinViewer` | — | `youyuan_accounts.css` |
| 70 | `youyuan_social.js` (9644) | 「服务器搜索 / 投放队列」页；覆盖 `Rcfg`(3) | `yySocialPage` `yySearchServers` `yyStartSocialBatch` `yyPollSocial` `yySaveNotices` | `/api/youyuan/social*` `/api/youyuan/notices` | `youyuan_design.css` |
| 71 | `youyuan_studio.js` (19308) | 插件市场发布 / 更新检查 | `yyCheckPluginUpdates` `yyPluginAutoCheck` `yyPluginSourceDialog` `yyStudioUploadFile` | `/api/plugin/list` `/api/plugin-market/files/*` | `youyuan_studio.css` |
| 72 | `youyuan_terminal.js` (30896) | 终端增强（清屏/复制/跟随/快捷键） | `yyTerminalClear` `yyTerminalCopy` `yyTerminalFollow` `yyTerminalKey` | `/api/bot/console` `/api/plugin/*` | `youyuan_terminal.css` |
| 73 | `youyuan_tunnel.js` (3775) | 公网隧道卡（包装 `yyRemotePage`） | `yyTunnelRefresh` `yyTunnelAction` `yyTunnelCopy` | `/api/youyuan/tunnel` | `youyuan_connection_policy.css` |
| 74 | `youyuan_upload.js` (10253) | 分片上传（SHA-256 校验 + 票据） | `yyUploadFile` | `/api/youyuan/files/transfer` | `yy_file_browser.css` |
| 75 | `youyuan_usability.js` (6868) | 配色选择器 / 开发文档 / 统计卡点击；覆盖 `Rcfg`(5) `Rhome`(9) `fpicker` `pickBySetting` `yyRemotePage`(18) | `yyChoosePalette` `yyRefreshPaletteLabel` `yyOpenLanguageDoc` `yyJobOverview` | `/api/youyuan/docs` `/api/task/list` | `youyuan_usability.css` |
| 76 | `youyuan_workspace.js` (59751) | 工作区（项目文件/编辑器/Agent 任务/MCP）；覆盖 `Rplg`(5) `yyLanguagePage`(54) | `yyOpenWorkspace` `yyWorkspacePane` `yyAgentTask` `yySaveProjectFile` `yyRiskConfirm` `yyCallMCP` | `/api/youyuan/agent/*` `/api/plugin/run` `/api/youyuan/agent/archive` | `yy_agent_workspace.css` `yy_file_browser.css` |

> **根目录 js 共 76 个，本表 76 个。**

### 3.2 `js/features/*.js`（28 个）

| # | 文件（字节） | 负责的页面/面板 | 主要全局入口 | 依赖接口 | 对应 css |
|---|---|---|---|---|---|
| 1 | `ai_assist.js` (42698) | 「AI 助手」对话面板（含会话/快照/技能/工具卡） | `openAIAssist` `aiSend` `aiOpenManage` `aiOpenSession` `aiOpenSkills` `aiNewChat` | `/api/ai/chat` `/api/ai/models` `/api/ai/answer` | `youyuan_design.css` |
| 2 | `announcements.js` (11536) | 公告 / 版本检查 / 完整性告警 | `fetchAnnouncements` `showAnnouncements` `handleVersionCheck` `showIntegrityAlert` `showImagePreview` | `/api/announcements` | `components.css` |
| 3 | `building.js` (56260) | 「建筑导入」`RimpHTML` + 「建筑导出」`RexpHTML` | `RimpHTML` `RexpHTML` `RsearchHTML` `doImp` `doGenVoxel` `doRepair` `showRegionPicker` | `/api/building/analyze` `/api/building/voxel` `/api/fleet/status` | `youyuan_design.css` |
| 4 | `building_mgmt.js` (45306) | 「建筑文件管理」+ 建筑市场（`bmOpenMarket`） | `RbuildingMgmtHTML` `bmOpenMarket` `bmOpenSub` `bmOpenDetail` `bmUploadDone` `bmRenderPreviewGrid` | `/api/market/categories` `/api/market-proxy/*` | `youyuan_design.css` `yy_building_deploy.css` |
| 5 | `camera.js` (30466) | 「相机工具」四步向导（选人→录制→镜头→输出） | `RcameraHTML` `camStartRec` `camStopRec` `camApplyShotEdits` `camExportMcfunction` | `/api/camera/*` | `youyuan_design.css` |
| 6 | `cb_engineering.js` (17953) | 「命令方块工程」（导入/扫描/导出） | `RcbHTML` `cbSwitchTab` `cbStartImport` `cbStartScan` `cbSaveExport` | `/api/cb/import/*` `/api/cb/export/*` | `youyuan_design.css` |
| 7 | `drawing.js` (30679) | 「像素画板」（Canvas 绘制 + 提交） | `RdrawHTML` `initDraw` `drawApply` `drawUndo` `drawRedo` `drawToggleFloat` | `/api/draw/*` | `youyuan_design.css` |
| 8 | `filepicker.js` (36983) | 内置文件选择器（浏览设备目录） | `showFilePicker` | `/api/files/scan` `/api/skin/*` | `components.css` |
| 9 | `fly.js` (25481) | 「飞行控制」摇杆/姿态 | `RflyHTML` `flyToggle` `flyJump` `flySneakStart` `flyPollStatus` | `/api/fly/*` | `youyuan_design.css` |
| 10 | `helpers.js` (10532) | THREE 体素 3D 预览（被导入/地图画调用） | `show3D` | — | `components.css` |
| 11 | `hq_query.js` (12939) | 「总部查询器」（包装成 IIFE，挂 `window.RhqQueryHTML`） | `window.RhqQueryHTML` `hqQueryFetchLive` `hqQueryRender` | `/api/youyuan/world/hq` | `youyuan_hq_query.css` |
| 12 | `intro_video.js` (2649) | 冷启动片头动画（一次） | 无导出（IIFE，`sessionStorage` 去重） | — | — |
| 13 | `itemmaker.js` (17674) | 「物品生成」 | `RitemmakerHTML` `doItemMake` `imAddItem` `imAddEnch` | `/api/itemmaker/generate` | `youyuan_design.css` |
| 14 | `mapart.js` (25550) | 「地图画」（图片→方块画） | `RmaHTML` `doMa` `doMaStop` `refreshMaPreview` `showDitherPicker` | `/api/mapart/preview` `/api/task/start` | `youyuan_design.css` |
| 15 | `mapplayer.js` (4235) | 「展示框显示器」（**无任何入口**，见 §8） | `RmapDisplayHTML` `doMapScan` `doMapDisplay` `doMapPlay` | `/api/map/*` | `youyuan_design.css` |
| 16 | `mcfunction.js` (25238) | 「长时指令执行器」（四步：选文件→解析→执行→进度） | `RmcfunctionHTML` `mcfnParseMTSX` `mcfnStartExec` `mcfnStopExec` `highlightMcfunction` | `/api/mcfunction/*` `/api/files/read` | `youyuan_design.css` |
| 17 | `migrate_lite.js` (4888) | 「区域迁移」轻量版 | `RmgrtLiteHTML` `yyMigrateExport` `yyMigrateImport` `yyMigrateCheck` | `/api/files/scan` `/api/task/start` | `youyuan_design.css` |
| 18 | `music.js` (28299) | 「音乐工坊」+「钢琴」 | `RmusicHTML` `RpianoHTML` `doMusicPlay` `uploadMidiFile` `renderPianoKeys` | `/api/music/*` `/api/music-proxy/*` | `youyuan_design.css` |
| 19 | `notifications.js` (7277) | 全局通知栈（toast/popup/banner/modal） | `ensureNotifStack` `showNotifToast` `showNotifPopup` `loadPendingNotifs` | `/api/push/*` `/api/config` | `components.css` |
| 20 | `others.js` (24611) | 「更多工具」`RothersHTML`（工具索引）+ 滚动字幕 + 调试台 | `RothersHTML` `RmarqueeHTML` `doMarqueeStart` `RdebugHTML` `yyToolsListHTML` | `/api/marquee/*` `/api/bot/status` | `yy_tools_browser.css` |
| 21 | `personalize.js` (17005) | 「个性化」弹层（背景/配色/封面） | `initPersonalize` `openPersonalizeSheet` `applyBackground` `pickThemeColors` | `/api/files/read` | `youyuan_usability.css` |
| 22 | `plugin_market.js` (30175) | 「插件市场」完整页（列表/详情/安装/评论/投稿） | `APluginMarket` `pmOpenMarket` `pmInstall` `pmDoInstall` `pmAddComment` | `/api/plugin-market/*` `/api/plugin-market-proxy` | `youyuan_design.css` |
| 23 | `plugin_mgmt.js` (26227) | 插件导入 / 代码与配置编辑器 | `plgImport` `plgCreate` `plgEdit` `openPluginEditor` `plgSaveConfig` `plgExportZip` | `/api/plugin/*` `/api/file/upload` | `youyuan_design.css` |
| 24 | `preview.js` (82165) | 「生成预览图」（四步：选目录→筛选→角度→生成） | `RpreviewHTML` `previewOpen` `previewStartGenerate` `previewDoZip` `previewBuildScene` | `/api/preview/*` `/api/building/voxel` | `youyuan_design.css` |
| 25 | `qq_bridge.js` (45404) | QQ 桥接状态面板（旧版配置向导） | `renderQQPanel` `qbOpenWizard` `qbWizFinish` `qbRefreshStatus` | `/api/qq/config` `/api/qq/status` | `youyuan_qq_console.css` |
| 26 | `qq_template.js` (26905) | QQ 消息模板库 | `qbOpenTemplates` `qbTplSave` `qbTplPreview` `qbTplCreate` | `/api/qq/template` `/api/qq/ai-prompt` | `youyuan_qq_console.css` |
| 27 | `seed_map.js` (53145) | 「种子地图」v1（`RseedMapHTML`，从「更多工具」进入） | `RseedMapHTML` `ysmInit` `ysmDraw` `ysmRunJob` `ysmReadWorldSeed` | `/api/seed/*` `/api/youyuan/world/seed` | `yy_seed_map.css` |
| 28 | `skin.js` (9777) | 「皮肤雕像」（本地/在线皮肤→雕像） | `RskinHTML` `doSkinBuild` `doSkinPickOnline` `yySkinRefreshOnline` | `/api/skin/*` | `youyuan_design.css` |

> **features 共 28 个，本表 28 个。**

### 3.3 `js/pages/*.js`（5 个，一级 tab 渲染器）

| # | 文件（字节） | 负责的页面/面板 | 主要全局入口 | 依赖接口 | 对应 css |
|---|---|---|---|---|---|
| 1 | `home.js` (14935) | 首页**原始版**（被 `dashboard.js:85` 整体覆盖，见 §5） | `Rhome`(1) `setScMode` `accountCard` `doSwitchAcc` `showAccountModal` | `/api/accounts` `/api/bot/status` `/api/task/list` | `youyuan_design.css` |
| 2 | `tasks.js` (12990) | 插件 tab**原始版** + 任务中心 | `Rtsk`(126，零调用) `yyTaskCenter` `yyTaskCenterPage` `yyOpenTask` `yyTaskDialog` `resT` `stopT` `delT` | `/api/task/*` | `youyuan_design.css` |
| 3 | `terminal.js` (4240) | 终端 tab 原始版 | `Rconsole` `appendConsoleLogs` `sendConsole` `addConsoleLine` | `/api/bot/console` | `youyuan_terminal.css` |
| 4 | `settings.js` (42834) | 设置 tab 原始版（连接/Token/机器人/主题） | `Rcfg` `refreshCfg` `doConn` `doDisc` `doFleetConn` `showConnModal` `doClearCache` | `/api/bot/*` `/api/config` `/api/cache/clear` | `youyuan_design.css` |
| 5 | `plugins.js` (13116) | 插件 tab 主体（`yyOriginalPlugins` 来源） | `Rplg` `yyPluginListHTML` `plgToggle` `plgEnable` `plgDisable` `plgReload` `plgRename` | `/api/plugin/*` | `youyuan_design.css` |

> **pages 共 5 个，本表 5 个。**

### 3.4 `js/vendor/*.js`（3 个）

| # | 文件（字节） | 作用 | 全局导出 | 依赖接口 | 对应 css |
|---|---|---|---|---|---|
| 1 | `marked.min.js` (35141) | Markdown 渲染 | `marked` | — | — |
| 2 | `three.min.js` (648280) | 3D 渲染（体素预览、预览图生成） | `THREE` / `window.__THREE__` | — | — |
| 3 | `youyuan_editor.js` (583929) | 代码编辑器（插件/工作区） | `YYEditor` | — | `youyuan_plugin_studio.css` |

> **vendor 共 3 个，本表 3 个。**
>
> ### 合计：js 共 112 个，本表 112 个。

---

## 4. 页面 → 入口 映射（点哪里 → 调什么 → 渲染到哪）

### 4.1 一级导航（`#bottom-nav`，`index.html:67-74`）

| 导航项 | `data-tab` | 点击链路 | 最终渲染函数 | 渲染容器 |
|---|---|---|---|---|
| 首页 / 工作台 | `home` | `button[data-tab]` click（`app.js:398 initNavDrag` 内绑定）→ `tabTo('home')` | `Rhome()`（`dashboard.js:85` 版，被 `boot_stable.js:40` 包裹） | `#tab-home`（整块 `innerHTML`） |
| 插件 | `tasks` | → `tabTo('tasks')` | `Rplg()`（`export_center.js:250` 是最终层） | `#tab-tasks` |
| 终端 | `terminal` | → `tabTo('terminal')` | `Rconsole()`（`dashboard.js:134` 包装 + `settings.js:40` 再包） | `#tab-terminal` |
| 设置 / 连接 | `settings` | → `tabTo('settings')` | `Rcfg()`（覆盖 11 次，最终为 `connection_policy.js:179`） | `#tab-settings` |
| 账号 | `accounts` | → `tabTo('accounts')`（`accounts.js:429` 的包装） | `Raccounts()`（`exports.js:41` 是最终层） | `#tab-accounts` |
| **互通**（运行时插入） | `hub` | `navigation.js:138-174` 动态 `insertBefore` 到「终端」与「连接」之间；click → `openHub()` | `yyOpenModule('qq')` → `yyQQPage()` = `yyQQConsolePage()` | `#sub-page` |

> `tabTo` 的关键行为（`app.js:477-521`）：切 tab 时先执行 `activeRenderers` 里的清理函数，再对**未初始化**的 tab 调一次渲染器（`tabInited[t]` 只记一次）；`tasks` 例外——每次进入都调 `Rplg()`。

### 4.2 工作台工具模块 `yyModules`（`dashboard.js:34-58`，静态 23 项）

用户在首页工具网格/常用工具/最近使用里点卡片 → `yyToolCard` 的 `onclick="yyOpenModule('<id>')"`（`dashboard.js:104`）→ `yyOpenModule`（`dashboard.js:63`，被 `boot_stable.js:98` 包裹）→ `m.open()`。

| id | 名称 | 分类 | `open()` 实际调用 | 落地容器 |
|---|---|---|---|---|
| `import` | 建筑导入 | building | `openSubPage('建筑导入', RimpHTML())` | `#sub-page` |
| `export` | 建筑导出 | building | `openSubPage('建筑导出', RexpHTML())` | `#sub-page` |
| `files` | 建筑文件 | building | `openSubPage('建筑文件管理', RbuildingMgmtHTML())` | `#sub-page` |
| `command` | 命令方块 | building | `openSubPage('命令方块工程', RcbHTML())` | `#sub-page` |
| `mapart` | 地图画 | create | `openSubPage('地图画', RmaHTML())` | `#sub-page` |
| `blockvideo` | 视频方块化 | create | `yyBlockVideoPage()` | `#sub-page` |
| `converter` | 结构与指令转换 | building | `yyConverterPage()` | `#sub-page` |
| `music` | 音乐工坊 | create | `openSubPage('音乐', RmusicHTML())` | `#sub-page` |
| `skin` | 皮肤雕像 | create | `openSubPage('皮肤雕像', RskinHTML())` | `#sub-page` |
| `draw` | 像素画板 | create | `openSubPage('画板', RdrawHTML())` + `initDraw()` | `#sub-page` |
| `camera` | 相机工具 | create | `openSubPage('相机工具', RcameraHTML())` | `#sub-page` |
| `item` | 物品生成 | create | `openSubPage('物品生成', RitemmakerHTML())` | `#sub-page` |
| `runtime` | 常驻插件 | automation | `yyLanguagePage()` | `#sub-page` |
| `plugins` | 插件管理 | automation | `tabTo('tasks')` | `#tab-tasks` |
| `mcfunction` | 指令执行器 | automation | `openSubPage('长时指令执行器', RmcfunctionHTML())` | `#sub-page` |
| `ai` | AI 助手 | automation | `openAIAssist('')`（`settings.js:12` 改写为 `tabTo('tasks')` + `yyCreateWorkspaceDialog()`） | 弹层 / 工作区 |
| `fly` | 飞行控制 | automation | `openSubPage('飞行控制', RflyHTML())` | `#sub-page` |
| `qq` | 群服互通 | connect | `yyQQPage()` → `yyQQConsolePage()` | `#sub-page` |
| `remote` | 公网远控 | connect | `yyRemotePage()`（被 4 个脚本包装） | `#sub-page` |
| `library` | 云端资料库 | connect | `yyLibraryPage()` | `#sub-page` |
| `settings` | 账号与连接 | connect | `tabTo('settings')` | `#tab-settings` |
| `api` | API 目录 | connect | `yyCatalogPage()`（`admin:true`，`yyModuleAllowed` 过滤） | `#sub-page` |
| `more` | 更多工具 | connect | `openSubPage('其他', RothersHTML())` | `#sub-page` |

### 4.3 运行时动态注册的模块（5 个，不在 `dashboard.js` 表里）

| id | 名称 | 注册位置 | `open()` | 触发条件 |
|---|---|---|---|---|
| `search` | 服务器搜索 | `settings.js:4`（`yyModules.push`） | `yySocialPage()` | 脚本加载即注册 |
| `market` | 插件市场 | `market.js:839` | `window.yyMarketPage()` | 加载即注册（带 `some()` 去重） |
| `seed` | 种子查询 | `seed_page.js:1091` | `yySeedPage()` | **由 `navigation.js:181-201` 懒加载后**才注册（先 `seed_local.js` 再 `seed_page.js`，版本戳 `2026102401`） |
| `seedmap2` | 种子地图 | `seedmap_v2.js:2382` | `yySeedMapPage()` | 加载即注册；若存在旧 `seedmap` 模块则改写其 `open` |
| `globalcapture` | 全局抓包 | `global_capture.js:493` | `yyGlobalCapturePage()` | 加载即注册，并自动加入 `yyPrefs.pins`（常用工具） |

### 4.4 二级页面（不经 `yyModules`，由其它页面里的按钮/卡片打开）

| 界面位置 | 按钮/触发 | 全局函数 | 容器 |
|---|---|---|---|
| 设置页「反馈」卡 | `yyFeedbackPage()` | `yyFeedbackPage`（`settings.js`） | `#sub-page` |
| 设置页「命令链」卡 | `yyChainsPage()` | `yyChainsPage`（`chain_center.js`） | `#sub-page` |
| 设置页「跨账号联通」卡 | `yyAccountBridgePage()` | `youyuan_connections.js` | `#sub-page` |
| 设置页「MCP 专用连接」卡 | `yyMCPLinksPage()` | `youyuan_connections.js` | `#sub-page` |
| 设置页「修改平台密码」 | `yyPlatformPasswordPage()` | `youyuan_connections.js` | `#sub-page` |
| 设置页「游园平台账号」卡 | `yyPlatformPage()` / `yySubscriptionsPage()` | `youyuan_identity.js` | `#sub-page` |
| 设置页「主机文件」卡 | `yyHostFilesPage()` | `youyuan_host_files.js` | `#sub-page` |
| 设置页「导出中心」卡 | `yyOpenExportCenter()` | `youyuan_export_center.js` | `#sub-page` |
| 设置页「配色」按钮 | `yyChoosePalette()`（覆盖了 `#cfg-theme-picker.onclick`） | `youyuan_usability.js` | `#sub-page` |
| 设置页「插件开发文档」卡 | `yyOpenLanguageDoc('python-runtime' / 'cpp-runtime' / 'lua-runtime' / 'command-chain')` | `youyuan_usability.js` | `#sub-page` |
| 设置页「背景与封面」卡 | `yyAppearancePage()` | `youyuan_appearance.js` | `#sub-page` |
| 设置页「体感 / 出口」 | `yyExperiencePage()` / `yyGameEgressPage()` | `youyuan_game_controls.js` | `#sub-page` |
| 设置页「游戏账号注册」 | `yyOpenAccountRegistration()` / `yyGameAuthPage()` / `yyRoomSearchPage()` | `youyuan_game_auth.js` | `#sub-page` |
| 账号 tab 皮肤按钮 | `yySkinLibraryOpen(row)` | `youyuan_skin_library.js` | 弹层 |
| 账号 tab 账号池 | `yyPoolPanel()` / `yyPoolDetail(i)` | `youyuan_accounts.js` | `#sub-page` |
| 工作区 | `yyOpenWorkspace(id, pane)` | `youyuan_workspace.js`（被 4 个脚本包装） | `#sub-page` |
| 首页统计卡 | 4 张卡分别 → `yyNavigate('settings')` / `yyRemotePage()` / `yyNavigate('tasks')` / `yyJobOverview()` | `youyuan_usability.js:9` | 混合 |
| 「更多工具」列表 | `RothersHTML()` 里的工具行 → `RhqQueryHTML()` / `RseedMapHTML()` / `RmgrtLiteHTML()` / `RdebugHTML()` 等 | `features/others.js:6-16` | `#sub-page` |
| 群服互通内各模块 | `yyQQOpenModule(id)` | `youyuan_qq_console.js` | `#sub-page` 内区块 |
| 插件市场（工作台内） | `APluginMarket()` / `pmOpenMarket()` / `bmOpenMarket()` | `plugin_market.js` / `building_mgmt.js` | `#sub-page` |
| 全局返回键 | `window.__onBackPressed__`（`app.js:522`） | 有弹层先关弹层，否则 `closeSubPage()` | — |

### 4.5 其它入口

| 入口 | 实现 |
|---|---|
| 顶部状态悬浮窗按钮 | `index.html:54` 内联 `onclick="window.android.toggleFloatingWindow()"`；`dashboard.js:76` 改成带兜底 `T()` 的版本 |
| 明暗主题按钮 | `dashboard.js:75` 动态插入 `.cb-right`，`onclick=yyToggleTheme` |
| 手机二级导航条 `#yy-sub-nav` | `navigation.js:29-125`，四格「首页/互通/工具/我的」→ `yySubNav('home'|'hub'|'tools'|'me')` |
| 桌面侧边栏 `.yy-sidebar` | `dashboard.js:71 yyShell()` 生成；`navigation.js:7-14` 重建为三组（工作空间 / 快捷操作 / 连接与资料） |
| 片头动画 | `features/intro_video.js` 自执行，`sessionStorage['yy-intro-played']` 控制只播一次 |
| 公告 / 版本检查 | `app.js:104-109` 启动 3s 后 `POST /api/version/check` → `handleVersionCheck` → `showAnnouncements` |
| 外部文件回调 | `utils.js:155 window.__onFilePicked__` / `159 window.__onExternalFile__`（Android 原生回调） |
| 存储权限回调 | `preview.js window.__onStoragePermissionResult__` |
| 导出结果回调 | `plugin_mgmt.js window.__onExportResult__` |

---

## 5. 覆盖链（本项目最危险的结构）

本项目的"插件化"完全依赖**后加载脚本改写全局函数**。同一个入口可能有 2~11 层包装，**最终生效的永远是最后一个赋值**。排查"按钮点了没反应 / 功能写了但没出现"时，第一步就是看这条链。

### 5.1 `Rcfg`（设置页）—— 11 次覆盖，最长链

| 顺序 | 位置 | 方式 |
|---|---|---|
| 1 | `pages/settings.js` | 定义原始 `Rcfg` |
| 2 | `dashboard.js:127` | **整体重写**（不调用原始版） |
| 3 | `settings.js:25` | 整体重写（当前设置页主体） |
| 4 | `identity.js:345` | 包装（追加平台账号卡） |
| 5 | `identity.js:438` | 再包装（追加订阅卡） |
| 6 | `game_auth.js:263` | 包装 |
| 7 | `game_auth.js:380` | 再包装 |
| 8 | `global_capture.js:74` | 包装 |
| 9 | `sense.js:2` | 包装 |
| 10 | `social.js:3` | 包装（追加提示与广播卡） |
| 11 | `usability.js:5` | 包装（改配色按钮 + 追加开发文档卡） |
| 12 | `appearance.js:12` | 包装（追加背景与封面卡） |
| 13 | `connection_policy.js:179` | 包装（**最终层**，追加连接策略与服务器信息） |
| 14 | `navigation.js:19` | 再包装（追加「打开终端 / 角色等级」快捷条） |

> 结论：`Rcfg` 有 **14 个赋值点**。任何新增设置卡都必须"包装"而不是"重写"，否则会静默抹掉下游 8 个模块的卡片。

### 5.2 `Rhome`（首页）—— 5 个赋值点，其中 2 个被整体覆盖

| 顺序 | 位置 | 方式 | 结果 |
|---|---|---|---|
| 1 | `pages/home.js:1` | 定义原始版 | 被 2 覆盖 |
| 2 | `youyuan.js:42` | 包装（追加「游园自建内核」入口卡 `#yy-entry-buttons`） | **被 3 整体覆盖 → 永不执行** |
| 3 | `dashboard.js:85` | 整体重写（工作台） | 生效 |
| 4 | `usability.js:9` | 包装（统计卡可点） | 生效 |
| 5 | `appearance.js:11` | 包装（`yyCoverShow()`） | 生效 |
| 6 | `boot_stable.js:40` | 包装（去重/防重复渲染） | 最终层 |

### 5.3 `Rplg`（插件 tab）

`pages/plugins.js`（原始，=`yyOriginalPlugins`）→ `youyuan.js:44`（追加常驻插件按钮，**被下一步覆盖**）→ `dashboard.js:132`（重写，但内部 `await yyBasePlugins()` = 原始版）→ `sdk.js:2`（空包装）→ `workspace.js:5` → `exports.js:78` → `export_center.js:250`（最终层）。

### 5.4 `Raccounts`（账号 tab）

`accounts.js:14`（原始）→ `exports.js:41`（最终层，追加导出入口）。渲染触发点是 `accounts.js:429` 对 `tabTo` 的包装（`TAB_ORDER` 也在 `accounts.js:428` 补 `'accounts'`）。

### 5.5 `Rconsole`（终端 tab）

`pages/terminal.js`（原始）→ `dashboard.js:134`（加标题、改高度）→ `settings.js:40`（改输入框 placeholder 与提示文案）。

### 5.6 `tabTo` / `openSubPage` / `yyOpenModule` / `yyRemotePage` / `yyQQPage`

| 函数 | 赋值链（按加载顺序） |
|---|---|
| `tabTo` | `app.js:477` → `dashboard.js:81`（同步面包屑）→ `accounts.js:431`（accounts 分支渲染）→ `navigation.js:168`（hub 分支）→ `boot_stable.js`（去重） |
| `openSubPage` | `app.js:110` → `navigation.js:115`（手机二级导航高亮） |
| `yyOpenModule` | `dashboard.js:63` → `boot_stable.js:98`（合并重复调用） |
| `yyRemotePage` | `youyuan.js:8` → `dashboard.js:122`（重写）→ `mcp_tutorial.js:2`（追加教程卡）→ `usability.js:18`（按账号显示密钥）→ `tunnel.js:2`（追加隧道卡） |
| `yyQQPage` | `youyuan.js:15` → `dashboard.js:126`（重写）→ **`qq_console.js:3065`（最终 = `yyQQConsolePage`）** |
| `yyLanguagePage` | `youyuan.js:9` → `dashboard.js:125` → `workspace.js:54` |
| `yyOpenWorkspace` | `workspace.js` → `agent_layout.js` → `agent_studio.js` → `agent_workspace_ui.js` |
| `A`（请求） | `api.js:22` → `boot_stable.js`（同参数合并） |

### 5.7 两个必须知道的陷阱

**陷阱 1：`youyuan.js` 的首页入口卡从未显示。**
`youyuan.js:41` 存了 `yyOriginalHome = Rhome`，`42` 行包装后往里插 `#yy-entry-buttons`（公网远控 / 常驻插件 / 群服互通 / 全部 API / 资料库 / 飞行控制…9 个按钮）。但 `dashboard.js:85` 直接把 `Rhome` 整体重写且**不调用** `yyOriginalHome`，于是 `youyuan.js` 那层永远不执行。
→ 这些入口现在只能从工作台工具网格或侧边栏进入，属于"功能在、入口没了"的典型。

**陷阱 2：`yyModules` 在 `settings.js:9` 被换成 13 项白名单副本。**
`settings.js:8` 先 `var yyAllModules = yyModules`，第 9 行 `yyModules = yyAllModules.filter(m => yyHomeIDs.indexOf(m.id) >= 0)`。`yyHomeIDs` 只有 13 项：
`import, export, mapart, blockvideo, converter, music, skin, search, files, more, library, mcfunction, globalcapture`。
后果：
- 首页工具网格只显示这 13 项（设计如此，"首页只展示创作与游戏工具"）；
- 但**任何读 `yyModules` 全量的代码都只拿到 13 项**；`yyModule(id)` 在第 10 行被改写为查 `yyAllModules`，所以 `yyOpenModule('qq')` 仍可用——两个数组的语义必须分清；
- 之后动态 `push` 到 `yyModules` 的模块（market / seedmap2 / seed / globalcapture）会进入"过滤后"的数组，而 `yyAllModules` 引用的是**同一个原始数组**，因此动态模块会同时被追加两份（代码里都写了 `yyAllModules.push(...)` 去重逻辑）。

---

## 6. 版本戳清单（部署验收基准）

`index.html` 共 **152 处** `?v=`，只有 **3 个不同值**。改动任何 js/css 都必须同步改这里，否则线上会读到旧缓存。

### 6.1 分布

| 版本戳 | 出现次数 | 构成 |
|---|---|---|
| `2026102824` | **148** | 40 个 css + 108 个 js（默认值，绝大多数文件用它） |
| `2026102825` | **3** | 1 个 css + 2 个 js（见 6.2） |
| `2026102826` | **1** | 1 个 js（见 6.2） |
| 合计 | **152** | 41 css + 111 js |

### 6.2 偏离默认值的 4 处（逐条）

| 行 | 类型 | 文件 | 版本戳 |
|---|---|---|---|
| 29 | css | `/css/youyuan_qq_console.css` | `2026102825` |
| 83 | js | `/js/youyuan_console_token.js` | `2026102825` |
| 138 | js | `/js/youyuan_accounts.js` | `2026102826`（**全站唯一最高版本**） |
| 176 | js | `/js/youyuan_global_capture.js` | `2026102825` |

### 6.3 不在 index.html 里的版本戳（动态注入，容易漏改）

`youyuan_navigation.js:181-201` 懒加载种子页时，把版本号写死为 **`2026102401`**：

| 位置 | 资源 |
|---|---|
| `navigation.js:186` | `/css/youyuan_seed_page.css?v=2026102401` |
| `navigation.js:190` | `/js/youyuan_seed_local.js?v=2026102401` |
| `navigation.js:195` | `/js/youyuan_seed_page.js?v=2026102401` |

> 注意：这三个文件在 `index.html:177-178` **也**被静态加载了一次，用的是 `2026102824`。也就是说同一份 `seed_local.js` / `seed_page.js` 存在两个版本号——静态那次已经执行并注册了 `seed` 模块，懒加载那次因 `#yy-seed-boot` 已存在会被 `if (document.getElementById('yy-seed-boot')) return;` 拦下。**验收时若只改 index.html 的版本戳，懒加载路径仍是旧的。**

### 6.4 默认版本戳（`2026102824`）覆盖的完整文件清单

**css（40 个）**
`base.css` `components.css` `layout.css` `youyuan_account_pool.css` `youyuan_accounts.css` `youyuan_agent_layout.css` `youyuan_agent_mobile.css` `youyuan_agent_scroll.css` `youyuan_agent_studio.css` `youyuan_boot_stable.css` `youyuan_chain_center.css` `youyuan_connection_diagnostics.css` `youyuan_connection_policy.css` `youyuan_design.css` `youyuan_game_controls.css` `youyuan_hq_query.css` `youyuan_market.css` `youyuan_mcp_scope.css` `youyuan_model_brand.css` `youyuan_navigation.css` `youyuan_plugin_studio.css` `youyuan_qq_assistant_tools.css` `youyuan_qq_console_quicknav.css` `youyuan_qq_member_ui.css` `youyuan_qq_serverinfo.css` `youyuan_reference.css` `youyuan_room_discovery.css` `youyuan_seed_page.css` `youyuan_seedmap_v2.css` `youyuan_settings_groups.css` `youyuan_skin_library.css` `youyuan_studio.css` `youyuan_terminal.css` `youyuan_usability.css` `yy_agent_workspace.css` `yy_building_deploy.css` `yy_file_browser.css` `yy_home_workbench.css` `yy_seed_map.css` `yy_tools_browser.css`

**js（108 个）**
`api.js` `app.js` `compat.js` `sse.js` `theme.js` `utils.js` `youyuan.js` `youyuan_account_boot.js` `youyuan_agent_attachments.js` `youyuan_agent_layout.js` `youyuan_agent_profiles.js` `youyuan_agent_studio.js` `youyuan_agent_workspace_ui.js` `youyuan_appearance.js` `youyuan_block_video.js` `youyuan_boot.js` `youyuan_boot_stable.js` `youyuan_chain_center.js` `youyuan_chain_upgrade.js` `youyuan_connection_diagnostics.js` `youyuan_connection_policy.js` `youyuan_connections.js` `youyuan_converters.js` `youyuan_dashboard.js` `youyuan_execution_options.js` `youyuan_export_center.js` `youyuan_exports.js` `youyuan_game_auth.js` `youyuan_game_controls.js` `youyuan_host_files.js` `youyuan_identity.js` `youyuan_identity_recovery.js` `youyuan_library.js` `youyuan_market.js` `youyuan_mcp_scope.js` `youyuan_mcp_tutorial.js` `youyuan_model_brand.js` `youyuan_navigation.js` `youyuan_plugin_studio.js` `youyuan_poll_policy.js` `youyuan_qq_assistant.js` `youyuan_qq_assistant_tools.js` `youyuan_qq_config.js` `youyuan_qq_console.js` `youyuan_qq_diagnostics.js` `youyuan_qq_groups.js` `youyuan_qq_guard.js` `youyuan_qq_knowledge.js` `youyuan_qq_member_ui.js` `youyuan_qq_memory.js` `youyuan_qq_moderation.js` `youyuan_qq_serverinfo.js` `youyuan_reference.js` `youyuan_room_discovery.js` `youyuan_sdk.js` `youyuan_seed_local.js` `youyuan_seed_page.js` `youyuan_seedmap_v2.js` `youyuan_sense.js` `youyuan_settings.js` `youyuan_settings_groups.js` `youyuan_skill_library.js` `youyuan_skin_library.js` `youyuan_skin_media.js` `youyuan_skin_viewer.js` `youyuan_social.js` `youyuan_studio.js` `youyuan_terminal.js` `youyuan_tunnel.js` `youyuan_upload.js` `youyuan_usability.js` `youyuan_workspace.js`
`features/`: `ai_assist.js` `announcements.js` `building.js` `building_mgmt.js` `camera.js` `cb_engineering.js` `drawing.js` `filepicker.js` `fly.js` `helpers.js` `hq_query.js` `intro_video.js` `itemmaker.js` `mapart.js` `mapplayer.js` `mcfunction.js` `migrate_lite.js` `music.js` `notifications.js` `others.js` `personalize.js` `plugin_market.js` `plugin_mgmt.js` `preview.js` `qq_bridge.js` `qq_template.js` `seed_map.js` `skin.js`
`pages/`: `home.js` `plugins.js` `settings.js` `tasks.js` `terminal.js`
`vendor/`: `marked.min.js` `three.min.js` `youyuan_editor.js`

### 6.5 未参与版本控制的文件（不在 index.html，无 `?v=`）

| 文件 | 状态 |
|---|---|
| `js/mcort_theme.js` | 未被任何 html/js 引用（§8 死文件） |
| `css/mcort_theme.css` | 未被 `<link>` 引用 |
| `css/style.css` | 112 字节，未被引用 |
| `*.js.esb`（31 个） | 构建中间产物，浏览器不加载 |

---

## 7. 部署副本 `cloudflare/public/` 说明（避免误判"线上是旧版"）

| 对比项 | 结果 |
|---|---|
| `index.html` | **与上游完全一致**（`diff -q` 无输出），版本戳相同 |
| `css/` | 43 个，**逐字节一致**（例：`youyuan_design.css` 两侧均 34119 字节） |
| `js/` | 112 个，**数量一致**，但 76 个与上游不同 |
| js 差异原因 | `public/js` 是 **esbuild 压缩产物**：单行、变量名压缩、中文转 `\uXXXX`（例：`public/js/theme.js` 2594B vs 上游 2722B） |
| 是否为旧版 | **不是**。抽查最新符号 `yyAcctCardWrapHTML` / `yyGlobalCapturePage` / `yySeedMapPage` / `yyMarketPage` 在 public 中均存在 |
| 逐字节相同的 js | 36 / 112（多为 vendor 与无需压缩的文件） |

**验收结论**：判断线上是否落后，**不能**用 `diff` 或文件大小，只能：
1) 比对 `index.html` 的 `?v=` 是否与源码一致；
2) 在 `public/js` 里 `grep` 目标函数名是否存在。

`public/` 还额外包含上游没有的 `framework-release.md`、`skill-library-guide.md`、`updates/`（发版说明与更新包，非前端源码）。

---

## 8. 疑似死代码（本项目历史高发问题）

> 历史事故模式：**函数写好了、样式也写了、就是没人调用**。因此本节用"零调用点"作为唯一判据，并给出可复现的检索方法。

### 8.1 检索方法（三步，可原样复现）

**第 1 步：抽取全部全局入口定义**（`^` 锚定行首，避免匹配到函数体内部）

```
cd <frontend>
grep -rnoE '^(async )?function [A-Za-z_$][A-Za-z0-9_$]*' js --include=*.js
grep -rnoE '^window\.[A-Za-z_$][A-Za-z0-9_$]* *='        js --include=*.js
```

**第 2 步：对每个名字统计"全库引用数"与"调用数"**

- 引用数：在 `index.html` + 全部 `js/**/*.js`（**排除 `*.esb`**）里数 `(?<![\w$])NAME(?![\w$])`
- 调用数：数 `(?<![\w$])NAME\s*\(`（`onclick="NAME(...)"` 这种字符串内调用同样计入）

**第 3 步：判定**

| 条件 | 含义 |
|---|---|
| 引用数 = 1 | 全库只出现一次 → **只有定义，零调用** |
| 引用数 > 1 但调用数 = 1 | 只被"定义行"自身匹配 → **零调用** |

复现脚本（Python 3，只读，不写文件）：

```python
import re, glob
from collections import Counter
files = ['index.html'] + [p for p in glob.glob('js/**/*.js', recursive=True) if not p.endswith('.esb')]
text = '\n'.join(open(p, encoding='utf-8', errors='ignore').read() for p in files)
tok = Counter(re.findall(r'[A-Za-z_$][\w$]*', text))
cal = Counter(re.findall(r'([A-Za-z_$][\w$]*)\s*\(', text))
# 再用上面的 grep 结果作为定义表，逐个比对 tok[n] / cal[n]
```

**第 4 步：人工排除**（见 8.2），避免把"外部回调/注册表"误判为死代码。

**已知局限**：`window['name']`、字符串拼接调用（`eval` 风格）不计入，因此"零调用"结论对动态派发不敏感；但本项目的调用几乎全是 `onclick="fn()"` 字面量，误判率很低。

### 8.2 人工排除清单（8 个，不算死代码）

| 名字 | 位置 | 排除理由 |
|---|---|---|
| `__onBackPressed__` | `app.js:522` | Android 原生返回键回调，由 APK 侧调用 |
| `__onFilePicked__` | `utils.js:155` | Android 文件选择回调 |
| `__onExternalFile__` | `utils.js:159` | Android 外部文件回调 |
| `__onExportResult__` | `plugin_mgmt.js:474` | Android SAF 导出结果回调 |
| `__subPageRestoreHooks` | `building_mgmt.js:52` | 全局钩子**数组**（`push` 使用，不是函数） |
| `__subPageDetachHooks` | `qq_template.js:33` | 同上 |
| `EventSource` | `youyuan_account_boot.js:15` | 全局 API 垫片，替换浏览器内置类 |
| `yyQQInteropModules` | `qq_console.js:140` | 模块互操作**注册表对象**，被其它脚本读写 |

### 8.3 零调用函数清单（45 个）

严重度：**高** = 整页/整块功能没有入口；**中** = 页内某个按钮/交互失效；**低** = 工具函数无人使用。

| # | 函数 | 位置 | 本应做什么 | 严重度 |
|---|---|---|---|---|
| 1 | `Rtsk` | `pages/tasks.js:126` | 插件 tab 的旧版渲染器（把任务表画进 `#tab-tasks`），已被 `Rplg` 取代 | 低（历史遗留） |
| 2 | `RmapDisplayHTML` | `features/mapplayer.js:1` | **「展示框显示器」整页**（扫描展示框→显示图片/播放视频），文件 4.2KB 完整实现，`yyModules` 无此条目，全库零调用 | **高** |
| 3 | `RdebugHTML` | `features/others.js:388` | 「调试台」页（轮询状态/日志清理），`RothersHTML` 的工具列表里没有这一项 | 中 |
| 4 | `dbgCleanup` | `features/others.js:412` | 调试台清日志按钮 | 低 |
| 5 | `showAccountModal` | `pages/home.js:160` | 首页账号切换弹窗（按 uid 去重、私有/共享分组）——首页改版后无人调用 | 中 |
| 6 | `doExp` | `features/building.js:675` | 「建筑导出」的开页入口（`openSubPage('建筑导出', RexpHTML())`），实际由 `yyModules.export` 直接调 `RexpHTML` 绕过 | 低（被等价实现替代） |
| 7 | `getOnlinePlayers` | `features/camera.js:600` | 相机工具「选人」步骤的在线玩家列表 | 中 |
| 8 | `getPlayerInfo` | `features/camera.js:611` | 相机工具单人信息 | 中 |
| 9 | `restoreCBState` | `features/cb_engineering.js:340` | 命令方块工程页返回时恢复表单状态 | 中 |
| 10 | `drawHideFloatBtn` | `features/drawing.js:462` | 画板隐藏悬浮按钮/关闭按钮 | 低 |
| 11 | `flyJump` | `features/fly.js:225` | 飞行「跳跃」按钮 | 中 |
| 12 | `flySneakStart` | `features/fly.js:242` | 飞行「潜行」按下 | 中 |
| 13 | `flySneakStop` | `features/fly.js:248` | 飞行「潜行」松开 | 中 |
| 14 | `flyDown` | `features/fly.js:268` | 飞行「下降」 | 中 |
| 15 | `flyIsMoving` | `features/fly.js:466` | 飞行状态判断（用于摇杆复位） | 低 |
| 16 | `restoreFlyState` | `features/fly.js:580` | 飞行页返回时恢复摇杆状态 | 中 |
| 17 | `imEnchOptions` | `features/itemmaker.js:101` | 物品生成「附魔」下拉选项 | 低 |
| 18 | `showMidiView` | `features/music.js:277` | 音乐工坊 MIDI 预览视图 | 中 |
| 19 | `plgExport` | `features/plugin_mgmt.js:441` | **插件「分享」弹窗**（导出 ZIP / 上传市场两个按钮），插件列表里没有分享按钮 | 中 |
| 20 | `previewGen2D` | `features/preview.js:1121` | 预览图 2D 渲染分支 | 中 |
| 21 | `previewDoZip` | `features/preview.js:1309` | 预览图打包 ZIP（`POST /api/preview/zip`），生成完成后没有调用 | 中 |
| 22 | `renderPreviewAngle` | `features/preview.js:1325` | 预览角度列表渲染 | 低 |
| 23 | `qbStatePush` | `features/qq_bridge.js:149` | QQ 桥接状态推送 | 低 |
| 24 | `refreshQQPanel` | `features/qq_bridge.js:158` | 旧 QQ 面板刷新（被 console 版取代） | 低 |
| 25 | `qbTplPreviewSaved` | `features/qq_template.js:453` | 模板「预览已保存版本」 | 低 |
| 26 | `ysmPxToBlockX` | `features/seed_map.js:637` | 种子地图像素→方块 X 换算 | 低 |
| 27 | `ysmPxToBlockZ` | `features/seed_map.js:638` | 同上 Z | 低 |
| 28 | `doSkinPickOnline` | `features/skin.js:92` | 皮肤雕像「在线皮肤」选择 | 中 |
| 29 | `updateConnStage` | `pages/settings.js:512` | 连接进度阶段文案（连接中弹窗） | 中 |
| 30 | `formatTime` | `utils.js:89` | 时间格式化工具 | 低 |
| 31 | `soundLabel` | `utils.js:149` | 音效名称映射 | 低 |
| 32 | `yyAcctUUID` | `youyuan_accounts.js:2346` | 账号 UUID 提取 | 低 |
| 33 | `yyAcctGameName` | `youyuan_accounts.js:2352` | 账号游戏名提取 | 低 |
| 34 | `yyAcctImgFail` | `youyuan_accounts.js:2443` | 头像加载失败兜底 | 低 |
| 35 | `yyAcctImgLoaded` | `youyuan_accounts.js:2459` | 头像加载完成回调 | 低 |
| 36 | `yyAcctCardWrapHTML` | `youyuan_accounts.js:2663` | **「调用身份卡」整块 UI**（可折叠 + 复制身份文字 + 取 Token），渲染函数与配套 CSS 都齐全，但没人拼进任何页面 | **高** |
| 37 | `yyAgentScrollPage` | `youyuan_agent_profiles.js:108` | Agent 页滚动位置保持 | 低 |
| 38 | `yyMcpScopeRoot` | `youyuan_mcp_scope.js:20` | MCP 权限面板根节点定位 | 低 |
| 39 | `yyQQToolsFold` | `youyuan_qq_console.js:877` | 群服互通「工具」区折叠 | 低 |
| 40 | `yyQQFoldToggle` | `youyuan_qq_console.js:1542` | 通用折叠开关（模块头部） | 中 |
| 41 | `yyQQPersona` | `youyuan_qq_console.js:1696` | 「人格」面板渲染 | 中 |
| 42 | `yyQQModelPicked` | `youyuan_qq_console.js:2027` | 模型选择回调 | 低 |
| 43 | `yyQQPromptCount` | `youyuan_qq_console.js:2029` | 提示词字数统计 | 低 |
| 44 | `yyQQLoadModels` | `youyuan_qq_console.js:2103` | 加载模型列表（AI 模块） | 中 |
| 45 | `yyObserverCooperationPage` | `youyuan_sense.js:387` | 「观测协作」页（多账号分工观测） | **高** |

### 8.4 四个最值得处理的案例

**① `RmapDisplayHTML` —— 整个「展示框显示器」功能没有入口（`features/mapplayer.js:1`）**
文件 4235 字节，页面 HTML、`doMapScan/doMapDisplay/doMapPlay/doMapStop` 四个动作、后端 `/api/map/*` 四条接口全部就绪，但：
- 不在 `yyModules`（23 项里没有）；
- 没有任何脚本 `push` 它；
- `RmapDisplayHTML` 全库只出现 1 次（定义行）。
→ 用户完全无法触达。若要恢复，最小改动是在 `dashboard.js` 的 `yyModules` 里加一条 `{id:'mapplayer', open:()=>openSubPage('展示框显示器', RmapDisplayHTML())}`。

**② `yyAcctCardWrapHTML` —— 新版账号「身份卡」写好了没接线（`youyuan_accounts.js:2663`）**
它拼出 `<details class="yy-acct-cardwrap">`，内含 `yyAcctIdentityCardHTML` + 复制身份卡文字 + 取 Token 三个按钮，配套 `yyAcctCardToggle` / `yyAcctCardCopy` / `yyAcctTokenToggle` 也都在。但这些配套函数**只被这段 HTML 里的 onclick 引用**，而这段 HTML 本身没人调用 → 整条链悬空。
注意：`cloudflare/public/js/youyuan_accounts.js` 里也有 `yyAcctCardWrapHTML`，说明它已经构建上线，只是永远不显示。

**③ `plgExport` —— 插件「分享」弹窗没有按钮（`features/plugin_mgmt.js:441`）**
弹窗里有「导出 ZIP」「上传市场」两个入口，依赖 `plgExportZip` / `pmOpenUpload`，两者都存在且可用。缺的只是插件卡片上的一个 `onclick="plgExport(id)"`。

**④ `yyObserverCooperationPage` —— 观测协作页零调用（`youyuan_sense.js:387`）**
同文件里 `yyTeamUI` / `yyObserverPlanUI` / `yyObserverHandoffUI` 都有人调用，唯独这个整页入口没有，属于"同一批功能里漏接了一个"。

### 8.5 结论

- 全库 **1653** 个全局函数定义，**45** 个确认零调用（占 2.7%），另有 **8** 个是外部回调/注册表不计入。
- 其中 **3 个是"整块 UI 级"缺失**（`RmapDisplayHTML`、`yyAcctCardWrapHTML`、`yyObserverCooperationPage`），是用户可感知的功能丢失，不只是代码洁癖问题。
- **建议的验收动作**：每次发版前跑一遍 8.1 的脚本，把新增的零调用函数当作"未接线告警"，而不是等到用户反馈"按钮不见了"。

---

## 9. 维护与复现命令

以下命令均为只读，可直接复制执行（路径按需替换）。

```bash
cd C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend

# 1) 文件清单（含字节数）
ls -l js/*.js js/*/*.js css/*.css

# 2) 哪些 js 没被 index.html 加载
for f in $(ls js/*.js js/*/*.js); do b=${f#js/}; grep -q "/js/$b" index.html || echo "NOT_LOADED $f"; done

# 3) 哪些 css 没被 index.html 引用
for f in css/*.css; do b=$(basename "$f"); grep -q "/css/$b" index.html || echo "NOT_LINKED $f"; done

# 4) 版本戳分布
grep -oE '\?v=[0-9]+' index.html | sort | uniq -c

# 5) 非默认版本戳（逐行）
grep -nE '\?v=202610282[5-9]' index.html

# 6) 函数覆盖链（谁重写了页面渲染器）
grep -rnE '^(Rhome|Rcfg|Rplg|Rconsole|Raccounts|tabTo|openSubPage|yyQQPage|yyRemotePage|yyOpenModule) *= *(async )?function' js --include=*.js | grep -v '\.esb'

# 7) 动态注册到工作台的工具
grep -rn 'yyModules.push' js --include=*.js | grep -v '\.esb'

# 8) 单个函数是否被调用（示例：plgExport）
grep -rn '\bplgExport\b' index.html js --include=*.js | grep -v '\.esb'
```

### 改动时的三条硬规则

1. **改 js/css 必须同步改 `index.html` 的 `?v=`**，且**同一文件在 `navigation.js:186-195` 还有第二个版本戳**（`2026102401`），种子相关文件要改两处。
2. **新增设置卡片/首页区块要"包装"现有渲染器**（`var base=Rcfg; Rcfg=async function(){await base(); ...}`），不要整体重写，否则会抹掉下游 8~13 层包装。
3. **新增工具入口要 `yyModules.push` 并同步 `yyAllModules` + `yyPrefs.order`**（参考 `global_capture.js:493-505` 的写法），否则在"全部工具"里排不到位置。

---

## 附录 A：css 清单（43 个，含未引用项）

| css | 字节 | 服务的模块 |
|---|---|---|
| `base.css` | 23177 | 全局基础（reset/变量） |
| `layout.css` | 13539 | 壳层布局、`.content`、`.sub-page` |
| `components.css` | 26974 | 通用组件（卡片/按钮/toast/弹窗） |
| `youyuan_design.css` | 34119 | 工作台设计系统（主视觉） |
| `youyuan_accounts.css` | 44085 | 账号 tab / 账号卡 |
| `youyuan_account_pool.css` | 8295 | 账号池 |
| `youyuan_agent_layout.css` | 8145 | Agent 版式 |
| `youyuan_agent_mobile.css` | 31493 | 手机端适配（含二级导航条） |
| `youyuan_agent_scroll.css` | 1926 | Agent 滚动 |
| `youyuan_agent_studio.css` | 12590 | 工作区 Studio |
| `youyuan_boot_stable.css` | 1734 | 首屏占位 |
| `youyuan_chain_center.css` | 6771 | 命令链中心 |
| `youyuan_connection_diagnostics.css` | 6874 | 连接诊断 |
| `youyuan_connection_policy.css` | 14214 | 连接策略/隧道 |
| `youyuan_game_controls.css` | 1301 | 体感/出口 |
| `youyuan_hq_query.css` | 3088 | 总部查询 |
| `youyuan_market.css` | 15243 | 插件市场 |
| `youyuan_mcp_scope.css` | 2282 | MCP 权限 |
| `youyuan_model_brand.css` | 4176 | 模型品牌下拉 |
| `youyuan_navigation.css` | 1708 | 侧边栏/二级导航 |
| `youyuan_plugin_studio.css` | 22231 | 插件工作室/编辑器 |
| `youyuan_qq_assistant_tools.css` | 4314 | QQ 助手工具 |
| `youyuan_qq_console.css` | 78115 | 群服互通控制台（最大） |
| `youyuan_qq_console_quicknav.css` | 2993 | 控制台快捷导航 |
| `youyuan_qq_member_ui.css` | 7903 | 群成员目录 |
| `youyuan_qq_serverinfo.css` | 18788 | 服务器信息 |
| `youyuan_reference.css` | 2917 | 参考文档 |
| `youyuan_room_discovery.css` | 6361 | 房间发现 |
| `youyuan_seed_page.css` | 8671 | 种子查询 |
| `youyuan_seedmap_v2.css` | 23954 | 种子地图 v2 |
| `youyuan_settings_groups.css` | 2926 | 设置页折叠分组 |
| `youyuan_skin_library.css` | 8495 | 皮肤库 |
| `youyuan_studio.css` | 8984 | 插件发布 |
| `youyuan_terminal.css` | 14077 | 终端 |
| `youyuan_usability.css` | 11734 | 配色/个性化 |
| `yy_agent_workspace.css` | 7401 | 工作区 |
| `yy_building_deploy.css` | 5843 | 建筑部署/市场卡 |
| `yy_file_browser.css` | 12722 | 文件浏览器 |
| `yy_home_workbench.css` | 3895 | 首页工作台 |
| `yy_seed_map.css` | 7672 | 种子地图 v1 |
| `yy_tools_browser.css` | 3996 | 更多工具列表 |
| `mcort_theme.css` | 11676 | **未引用**（配套 `mcort_theme.js`，整套未启用） |
| `style.css` | 112 | **未引用**（112 字节，疑似占位） |

> css 共 43 个，本表 43 个。

---

## 附录 B：本文档的核对结论

| 核对项 | 结论 |
|---|---|
| js 文件清单 | `ls` 112 条 = 本表 112 条 ✓ |
| css 文件清单 | `ls` 43 条 = 附录 A 43 条 ✓ |
| 版本戳 | 152 处，3 个值，4 处偏离默认 + 3 处动态注入 ✓ |
| 零调用函数 | 53 个候选，排除 8 个外部回调/注册表 → **45 个** |
| 未加载文件 | `js/mcort_theme.js`、`css/mcort_theme.css`、`css/style.css` |
| 部署副本 | `public/index.html` 与源码一致；`public/js` 为压缩产物，功能同步 |

