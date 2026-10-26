# 群服互通（MCORT）体验与结构审查 · 2026-10-26

审查范围：`upstream/frontend/js/youyuan_qq*.js`、`upstream/youyuan_qq*.go`、`upstream/frontend/css/youyuan_qq*.css`。
线上站点 `https://toolbox.youyuanqi.dpdns.org/` 与本地同版本（`index.html` 引用 `?v=2026102824`，线上 12 个互通脚本齐全），`/api` 401 属正常，未做登录态操作。
**本次只写报告，未改任何源码。**

证据口径：
- `文件:行` 指本地工作区当前内容（`youyuan_qq_console.js` 为未跟踪文件，因此无 git 历史可比对；涉及"退化"的判断以代码现状 + 线上同版本产物为准）。
- 结论只有四种：**已做**（行为符合诉求）、**部分**（后端或前端做了一半）、**没做**、**做错**（做了但与诉求相反/位置错）。

---

## 〇、一句话结论

用户 17 条里，**5 条真正做完**（转发四模式、认领二维码、表情包多图、知识库三目录+压缩包、统一模型配置）；**9 条"后端做完但前端没入口 / 入口放错位置"**（提示词、触发词分层、模型选择、掉线自愈、自动接话 interval、MCP 作用域、终端多群、主动搭话窗口、助手旧面板）；**2 条是架构性做错**（首页分组：源码里是 12 个平铺折叠块 + 6 个脚本各自"包装渲染函数"往里插，而不是 6 个模块；界面厚度/文案）；**1 条真退化**（游戏头像变文字）。

---

## 一、17 条逐条现状核对

### 1. 互通首页折叠模块不是 6 组

| 项 | 内容 |
|---|---|
| 现状 | `yyQQShellHTML()` 只拼 3 个折叠块 + 1 个表情块 = 4 个：`yyQQAccess()`（接入与互通群）、`yyQQPersona()`（人设与试跑）、`yyQQTools()`（AI 工具）、`yyQQFaces()`（收藏表情）。其余 8 个折叠块由 6 个独立脚本"包装 `window.yyQQRenderAll`、往 `.yyq-fold-grid` 里 `insertAdjacentHTML`"注入：群服互通助手、触发词规则、生图配置、群→游戏转发、知识库、互通端信息、群规、危险指令。 |
| 证据 | `youyuan_qq_console.js:2344-2351`（shell 只有 3 块）、`:2233`（`yyQQPartList` 只认 8 个 console 区块）、`:2210-2219`、`:2239-2247`（快捷跳转 chip 硬编码 8 个旧 id）<br>`youyuan_qq_assistant.js:776-788`、`youyuan_qq_config.js:1052-1063`（一次插 3 块）、`youyuan_qq_knowledge.js:393-409`、`youyuan_qq_serverinfo.js:723-733`、`youyuan_qq_moderation.js:585-596`、`youyuan_qq_guard.js:188-198` |
| 结论 | **做错（架构）**。不是"少写了 2 个块"，而是根本没有"模块"这一层：12 个折叠块平铺在一个 2 列 grid 里（`youyuan_qq_console.css:780-786`），顺序由脚本加载顺序决定，折叠状态由 8 个各自为政的全局变量维护（`accessOpen/personaOpen/toolsOpen/facesOpen` + 各模块私有 `open`）。用户只看到 4 个，说明这层"包装注入"在他们那条链路上没跑通——它依赖 6 次函数替换按序成功，任一环断掉就是"其余模块全消失"，与现象吻合。 |
| 附带发现 | 帮助菜单实际是 **7 组**不是 6 组：`chat/query/ai/cap/game/admin/root`（`youyuan_qq_help.go:62-94`、`:98`），🎮 游戏技能被单列；且标题后缀重复——`"🔑 超级管理员" + " · 仅超级管理员"` = 「🔑 超级管理员 · 仅超级管理员」，`"🛠 管理操作" + " · 仅管理员"`（`youyuan_qq_help.go:110-117`）。要"统一成 6 个"，web 与群内帮助两边都得改。 |

### 2. 全服互通原本显示「游戏头像」，现在变成一个文字

| 项 | 内容 |
|---|---|
| 现状 | 头像位置只有两层：底层是「昵称首字」文本占位 `<i>机</i>`，上层才是 QQ 头像 `<img>`；`img` 用 `onerror` 直接 `remove()`，失败即只剩一个字。互通首页身份卡同理：只有 QQ 头像（`p.avatar`），失败后露出机器人图标；**游戏皮肤头像从头到尾没有出现过**，"游戏身份"只是一行纯文本。 |
| 证据 | `youyuan_qq_serverinfo.js:232-236`（首字母占位 + `avaImg`）、`:515-527`（同源 `/api/skin/online-head` 已在用）<br>`youyuan_qq_console.js:370-376`（`yyq-avatar-fb` 机器人图标兜底）、`:383`（`["游戏身份", bot_player \|\| "未绑定"]`：一行字）<br>`youyuan_qq_image.go:102-107` + `youyuan_qq_serverinfo.go:132`：头像 URL 固定为 `https://q1.qlogo.cn/...`（境外外链，无同源代理、无缓存） |
| 结论 | **没做 + 做错**。①要求的"游戏头像"没做：没有把 `assistant.bot_player` 走 `/api/skin/online-head?player=` 渲染成皮肤头；②QQ 头像用的是境外 qlogo 直链，弱网/离线必失败，退化成"一个字"，这正是用户看到的现象。项目里已经有 `youyuan_skin_cache.go` 与 `/api/skin/proxy`，复用即可，不要再新增外链。 |

### 3. 违规词不能修改/删除；提示词保存后回来就没了

| 项 | 内容 |
|---|---|
| 现状（违规词） | 前端已支持"改 / 删 / 启停 / 新增"，删除有二次确认；后端 `POST`（整表替换 + `taboos` 指针语义）与 `DELETE ?pattern=&kind=` 都在，长期禁令单独走人格表。 |
| 证据 | `youyuan_qq_moderation.js:216-300`（`yyMEdit` / `yyMDel` / 行内编辑 `ruleEditRow`）、`:591-596`<br>`youyuan_qq_moderation.go:392-470`（GET/POST/DELETE；`:437` 起 kind=taboo 单独处理） |
| 现状（提示词） | 前端改成了"用户真改过才提交"的草稿机制：`yyQQPromptDraft` + `yyQQPromptPatch()`，两个面板（人设块 / 旧助手面板）镜像同一份草稿；后端 `yyMergePatch` 深合并 + 空串=显式清空，且有单测锁定。 |
| 证据 | `youyuan_qq_console.js:1488-1496`（注释即事故复盘）、`:1858-1861`（`yyQQPromptPatch`）、`:2050-2067`（保存只发改动字段）<br>`youyuan_qq_persona_test.go:303-330`（`TestYYConfigPromptPatchKeepsAssistantSiblings`） |
| 结论 | **源码层=已做；体验层=做错位置**。用户在线上仍然"改不了/存不住"，最可能的解释与第 1 条同源：群规、触发词这些折叠块根本没渲染出来，用户找不到入口；而提示词虽然有草稿机制，但**两个面板 + 三个保存按钮（保存人设 / 保存接话设置 / 保存助手设置）共存**，只要有一个旧入口被点到，观感仍然是"存了又没了"。→ 修复要收敛成"一个提示词只有一个保存按钮、一个写入者"。 |

### 4. 给 AI 的工具只有几个，要求 40~50 个；触发词缺管理员/成员分层

| 项 | 内容 |
|---|---|
| 现状（工具数） | 注册表共 **28 项**（游戏操作 10 / 服务器查询 3 / 群管理 2 / 内容生成 6 / 其它 7）。实际下发：普通成员 9~10 项（3 查询 + `game_skill` + 社交 6），管理员约 30 项（再加 `game_call`、`load_skill`、`web_fetch`、种子 8 项、MCP 3 项、`post_qzone`、`kick`、`ban`）。面板可逐项开关，关掉就真的不下发。 |
| 证据 | `youyuan_qq_toolbox.go:55-123`（28 条定义）、`:136-146`（开关）、`:249-255`（实际下发名单）<br>`youyuan_qq_ai.go:82-141`（成员/管理员分支）、`youyuan_qq_social_tools.go:39-92`（社交工具） |
| 现状（触发词分层） | 后端已有 `MinRole`（0 全员/1 管理员/2 超管），匹配时会拦（`level < t.MinRole` 直接跳过）。前端触发词表单**完全没有这个字段**（只有名称/匹配/冷却/日限）。 |
| 证据 | `youyuan_qq_triggers.go:38-41`、`:168-180`、`:623-628`<br>`youyuan_qq_config.js:171-260`（`yyTrigBlock` 表单）、`grep min_role frontend/js/*.js` 仅命中 `youyuan_qq_console.js:696`（工具行的角色标签，不是触发词） |
| 结论 | 工具数**部分（28 < 45）**，但方向对且可扩展；触发词**部分（后端有、前端无）**，属于"用户点不到"的典型。补到 45~50 项不等于把危险工具全给成员，应按"能力域"补齐只读/低危工具（见修复清单 P1-4）。 |

### 5. 生图配置没有模型选择，互通里却有；要求互通只留模板

| 项 | 内容 |
|---|---|
| 现状 | 三套并存：①**统一模型配置**（语言/画图/语音）已在设置页，且写穿到 `qq-bridge.json` 与 `qq-image.json`；②**生图配置折叠块**有自己的 provider/model/key；③**互通页仍保留模型下拉**——人设块里"使用模型"、自检块里"绘图模型"，旧助手面板里还有一整套"AI 模型 / 绘图模型 / 添加模型接口"。 |
| 证据 | `youyuan_settings.js:28`（设置页已写明"AI 模型不在这里配…统一到下面「AI 模型统一配置」"）、`:187-256`（`yyLoadModelRoles` / `yySaveModelRoles`）<br>`ai_handlers.go:37-52`（`POST /api/ai/models {roles}`）、`youyuan_ai_models.go:1-30`（写穿注释）<br>`youyuan_qq_console.js:1516`（`<select id="yyq-model">使用模型`）、`:1776`（自检"绘图模型"）<br>`youyuan_qq_config.js:636+`（生图块自带模型）、`youyuan_qq_assistant.js:411/600/740`（旧面板模型 + `image_model` 保存） |
| 结论 | **部分 + 做错位置**。统一配置已存在（诉求 15 已完成），但互通页没删干净，用户看到的是"同一件事有 3 个入口、其中 2 个是旧快照"，这正是"改了不生效/存了又没"的另一来源。互通应改为**只读模板**：显示“当前语言模型 = X（在设置 → AI 模型统一配置 修改）”。 |

### 6. 掉线智联的设置不能修改

| 项 | 内容 |
|---|---|
| 现状 | 设置页的"掉线智联（掉线自愈）"卡已开放：开关 / 探测间隔 10–3600 秒 / 每小时重启上限 1–100，`POST /api/qq/watchdog` 一直支持这两个字段并落盘。互通页那条掉线自愈条也能改，但它被放在英雄卡右侧，**只在已登录时渲染**；一旦 `need_login / need_scan`，整块被二维码替换 → 掉线状态下根本无法改设置（用户感知就是"不能修改"）。另外互通页的开关按钮失败时静默返回，没有任何提示。 |
| 证据 | `youyuan_settings.js:263-307`（可读可写）<br>`youyuan_qq_console.js:460-502`（两个输入框 + 保存）、`:399`（`need_login/need_scan` 时换成 `yyQQQRCode`，不渲染 watchdog）、`:519-526`（toggle 失败直接 `return`，无 `T(...)`） |
| 结论 | **部分 + 做错位置**。能力有了，但放在"掉线时看不见"的位置，等于没做。 |

### 7. 群聊消息转发到游戏：触发词 / 自动 / 定时 / 可配

| 项 | 内容 |
|---|---|
| 现状 | 四种模式齐全：`off` / `keyword`（命中任意触发词转这一条）/ `auto`（每条非指令消息都转）/ `interval`（每 N 分钟打包一次，单次上限可设）。后端有 ticker 跑定时打包，超限留到下一轮。 |
| 证据 | `youyuan_qq_config.js:845-910`（`yyRelayBlock` / `yyRelayBody`，模式按钮 + 触发词 + 分钟/条数）<br>`youyuan_qq_relay.go:20-32`、`:383`（ticker）、`:444-460`（状态回显）、`:500-510`（字段校验） |
| 结论 | **已做**。唯一体验问题是它也是一个"注入的折叠块"，受第 1 条连坐；且"转发开关"在两处（接入块的 `yyq-relay` 勾选框 + 转发块的模式），两个真值来源易打架（`youyuan_qq_console.js:1378` 的 `as.relay_chat` vs relay 的 `mode`）。 |

### 8. AI 自动接话：每隔 N 秒看一次聊天记录、自己挑想回的

| 项 | 内容 |
|---|---|
| 现状 | 三种接话模式：`mention`（默认）/ `interval`（每 N 分钟把"上次自检后的新消息"交给 AI，回不回、回哪条由模型决定，不回输出 `[SKIP]`）/ `auto`（每条都判，带 60 秒/人成本闸门）。自检轮询粒度 20 秒，单轮最多 40 条，间隔下限 1 分钟。 |
| 证据 | `youyuan_qq_companion_aimode.go:8-45`（三种模式 + 常量 + 两段任务提示词）、`:189`（ticker）<br>前端入口 `youyuan_qq_assistant.js:411` 的 `aimodeBlock(c)`；互通新面板里对应 `youyuan_qq_console.js:1539` 的 `yyQQCompanion()`（但只有"被@/触发词/每条都接"三档 + 窗口秒数，**没有 interval 的"每 N 分钟"**） |
| 结论 | **部分**。后端做全了，新面板（console 人设块）没暴露 `interval` 模式与分钟数，用户只能去旧助手面板找 → 又回到"能力存在但看不见"。 |

### 9. 超级管理员认领码改成 QQ 扫二维码

| 项 | 内容 |
|---|---|
| 现状 | 已实现：自研 QR 编码（版本 3/4，纠错 M，无第三方依赖）+ `GET /api/qq/claim/qr?code=` 返回 PNG；前端在 `yyQQCreateClaim` 外面包了一层，生成二维码并把文本码保留作兜底。 |
| 证据 | `youyuan_qq_claim_qr.go:1-30`（设计说明）、`:549-590`（`yyQQClaimQRText` / routes）、`youyuan_qq_social_tools.go:516-518`（注册）<br>`youyuan_qq_assistant_tools.js:13/59-70`（重写 `window.yyQQCreateClaim`）<br>`youyuan_catalog.go:164`（登记 `/api/qq/claim/qr`） |
| 结论 | **已做**。风险：只编码 `.认领 + 码` 文本，手机 QQ 扫出来是"可复制文本"而非自动跳转，符合原话，但要在 UI 上写明"扫码后复制到群里发"，否则用户会以为扫完就自动认领。 |

### 10. MCP 是"广泛性"还是"专一性（绑定本机 MCP）"要可配置

| 项 | 内容 |
|---|---|
| 现状 | 后端已有完整的二维准入：`scope`（private/public）× `access`（all/admin/owner_only）× 角色，唯一判定入口 `yyMCPScopeGuard`；QQ 群侧还有一层 `yyQQMCPScopeGuard`。面板是独立页 `youyuan_mcp_scope.js`（增删改、测试连通、启停、改归属人）。 |
| 证据 | `youyuan_mcp_scope.go:35-40`（枚举）、`:102-124`（规范化默认值）、`:250-290`（guard）、`:294-330`（列表视图）<br>`youyuan_mcp_scope.js:130/226/243/255/265`<br>互通页只有一排 `MCP 地址 + Token`（`youyuan_qq_assistant.js:411` 的 `yy-qq-mcp-url/mcp-token`） |
| 结论 | **部分 + 做错位置**。用户问的"广泛 vs 专一"就是 `scope=public + access=all` 与 `scope=private + owner_only`，已可配，但入口在"公网远控/MCP"里，互通页看不到；且"绑定本机 MCP（APK 内置）"与"外部 HTTP MCP"在同一个列表里没有类型标签（`yyMCPConn` 只有 `transport`），用户没法一眼区分。 |

### 11. 发表情包不该限定一张、也不该擅自"又说话又发表情"

| 项 | 内容 |
|---|---|
| 现状 | 已按原话改：`indexes`/`urls` 支持 1~9 张；`text` 是独立的"说话开关"，留空=只发图，返回里还明确告诉模型"已发完，不用再补一句"；提示词里也写了"只发图就不要补话，只想说话就别调这个工具"。 |
| 证据 | `youyuan_qq_social_tools.go:44-56`（工具 schema + 注释即用户原话）、`:300-325`（执行：`with_text` 判定 + note） |
| 结论 | **已做**。遗留：`yyStickerCollect` 若同时给 `indexes` 和 `urls` 的优先级要在 UI/文档里说清；另外"收藏表情"折叠块里点一下直发是单张（`youyuan_qq_console.js:2087-2091`），与工具能力不对等，容易让用户以为还是只能一张。 |

### 12. AI 消息终端只能绑一个群 → 要多群、简洁、显示自己的回复

| 项 | 内容 |
|---|---|
| 现状 | 终端固定在消息流底部，发送目标 = 当前选中的群（`yyQQC.picked`），单群；后端 `/api/qq/send` 也只收一个 `group` 字符串。自己的回复**能显示**：服务端 `is_bot` 消息 + 本地回声 `yyQQC.sent[group]`（10 秒内去重）都会进时间线，并打「机器人」标签。 |
| 证据 | `youyuan_qq_console.js:1258-1272`（`yyQQTerm`，`label = 当前群`）、`:1274-1290`（`yyQQTermSend` 只发 `group`）、`:1180-1212`（时间线合并 `messages` + `sent` + `actions`）、`:1223-1227`（`data-mine` + 「机器人」标签）<br>`youyuan_qq.go:255-300`（`/api/qq/send`：`Group string`，要求已启用） |
| 结论 | **部分 + 没做多群**。"显示自己的回复"已做；"多群"没做——前端是单群、后端是单群字段，要改两边（`groups []string` + 逐个发送 + 聚合结果）。"简洁"是主观项，现状终端还叠了"实时"灯、刷新按钮、错误行，可压成一行。 |

### 13. 主动搭话 / 静默窗口 / 深夜自动降权要可配

| 项 | 内容 |
|---|---|
| 现状 | 主动搭话可配 4 个数值：冷场秒数、两次最小间隔、每群每日上限、开口阈值；另有 weights（silence/topic/activity/night）。但**深夜窗口写死 23:00–07:00**（常量），`night` 权重没暴露，"静默时段完全不发"也没有独立开关。 |
| 证据 | `youyuan_qq_console.js:1584-1650`（`yyQQProactive` 四个输入框 + 保存）、`:1652-1700`（`yyQQSaveProactive`，`weights` 直接沿用缓存值）<br>`youyuan_qq_proactive.go:27`（Night 权重）、`:61-62`（`yyProactiveNightStart/End` 常量）、`:398-402`（按小时判深夜） |
| 结论 | **部分**。四个"度"能调，但"什么时候算深夜"和"静默时段"不可调，`weights` 对用户是不可见字段 → 用户仍会觉得"它半夜乱说话我管不了"。 |

### 14. 知识库默认三个目录、可上传文档与压缩包

| 项 | 内容 |
|---|---|
| 现状 | 三类目录 `memo`（AI 备忘录）/ `kb`（知识库）/ `skill`（Skill）真实存在并注册；上传支持 `.md/.markdown/.txt/.json/.csv/.zip`，zip 只抽文本、跳过二进制、有 200 条目/8 MiB 总量/1 MiB 单条上限；单文件 4 MiB；支持手写保存、预览、删除（内置条目不可删）、检索试跑。 |
| 证据 | `youyuan_qq_knowledge.go:62-66`（三目录）、`:39-46`（限制）、`:411-440`（`yyKBUnzipText`）、`:466-469`（路由清单）<br>`youyuan_qq_knowledge.js:24-33`（状态与 tab）、`:88-140`（折叠块 + 上传区，`accept` 明确含 zip）、`:300-380`（预览/删除/保存/检索） |
| 结论 | **已做**。注意点：`/api/qq/kb/tree` 返回 `persistence_error` 时前端只提示不阻断，磁盘不可写会"看起来能存、重启就丢"（`:75`、`:172-176`），需要在 P1 里做成显著阻断。 |

### 15. 统一模型配置（语言 / 画图 / 语音）

| 项 | 内容 |
|---|---|
| 现状 | 已实现且是"唯一凭证 + 三个用途指针"：模型库 `ai_models.json` 存地址/密钥；`roles.language/image/speech` 只存"用哪一条"；保存时写穿到 `qq-bridge.json`（assistant.model/image_model/voice_model/voice_voice/voice_mode）与 `qq-image.json`，老消费者无需改代码。设置页有完整的三个下拉 + 出图尺寸 + 音色 + 语音模式。 |
| 证据 | `youyuan_ai_models.go:1-30`、`:78-100`（`Roles()` 视图）、`:137`（规范化）、`:178`（写回 assistant）<br>`ai_handlers.go:37-52`、`youyuan_settings.js:187-256` |
| 结论 | **已做**。与本轮诉求 5 冲突的地方：互通页、生图块、旧助手面板仍各自保留模型下拉（见第 5 条），"统一"还没统一干净。 |

### 16. 一大段"全服互通助手"帮助菜单默认就有，要求不默认弹出、可配置

| 项 | 内容 |
|---|---|
| 现状 | 存在两份：①**旧版巨型面板**——`youyuan_qq_assistant.js:411` 一整行约 8 KB 的 HTML 字符串（`yy-qq-assistant-panel`），含"高级权限标识 / 生成认领码 / 同步死亡·进服·退服 / 公开计分项 / 自然群聊 / 绘图模型 / MCP / 指令帮助 `<pre>` / 从真实群成员授权 / 绑定用户"十几个区块，**没有任何开关能关掉或折叠整体**；②新版控制台里同名折叠块 `yyq-assistant`（`youyuan_qq_assistant.js:415-422`），默认 `open:0`，但内容同样是"把设置项全摊开"，且摘要里就写着"群服互通助手"，用户会以为是帮助。 |
| 证据 | `youyuan_qq_assistant.js:411`（旧面板）、`:413-422`（新折叠块，`open:0`）、`:772-788`（注入）；`youyuan_qq_groups.js:3`（旧页面 `yyQQPage` 会先 `await base()` 再插"多个互通群"卡，说明旧页面链路仍在）<br>`youyuan_qq_console.js:2790-2791`（`yyQQPage = yyQQConsolePage` 覆盖旧页） |
| 结论 | **部分 + 做错形态**。新页已折叠（不算默认弹出），但旧面板整段仍在（死代码但体积巨大、易被别的页面 `yyQQPage` 覆盖顺序复活），且**没有任何"这是插件、可停用"的开关**。诉求本质是"插件化"，不是"再折一层"。 |

### 17. 界面"很厚"、乱排列、混入写给 AI 自己看的说明文案

| 项 | 内容 |
|---|---|
| 现状 | 一个"人设与试跑"折叠块里塞进了 6 个面板：提示词 + 模型 + AI 自检 + 主动接话 + 主动搭话 + 多模态识图 + 语音 + 沙盒（8 段），展开后是数屏长度；12 个折叠块 2 列平铺，快捷跳转 chip 仍是 8 个旧锚点；多处说明文案是"实现说明"而不是"操作指引"（如"自动检测只认模型名里的 vision / vl / 4o 等字样""换行、逗号、顿号、分号都算分隔符；不区分大小写""首次触发先等一小段（接话窗口的40%，最多10秒）"），还有直接面向模型的规则句（"不要问用户「要不要我拍你」"这类本体在提示词里，但面板注释性长文外泄到 UI）。 |
| 证据 | `youyuan_qq_console.js:1500-1537`（人设块 body：`yyQQAISelf()` + `yyQQCompanion()` + `yyQQProactive()` + `yyQQVisionPanel()` + `yyQQVoicePanel()` + 沙盒 + 保存，共 8 段）<br>`youyuan_qq_console.js:1703-1720`（识图说明）、`youyuan_qq_config.js:891/902`（分隔符/上限说明）、`youyuan_qq_knowledge.js:196`（"机器人回答前会先查这里…"）<br>`youyuan_qq_console.js:2239-2247`（8 个 chip）、`youyuan_qq_console.css:780-786`（2 列 grid） |
| 结论 | **做错**。这不是"样式再调一调"的问题：**信息架构缺一层**（模块 → 区块 → 字段），所有东西都挂在"折叠块"这一层，所以必然又厚又乱。修法见第二节。 |

### 复核小结（17 条计数）

| 结论 | 条数 | 条目 |
|---|---|---|
| 已做 | 5 | 7、9、11、14、15 |
| 部分 | 9 | 3、4、5、6、8、10、12、13、16 |
| 做错 | 2 | 1、17 |
| 没做 | 1 | 2（游戏皮肤头像从未实现，只退化成文字） |

---

## 二、6 模块重分组方案

### 2.1 目标形态（与帮助菜单 6 组同名同序）

首页结构固定为：

```
[状态带] [快捷跳转 chips：6 个模块锚点]
[身份卡 Hero]                        ← 头像（游戏皮肤优先）+ 昵称 + 状态 + 掉线自愈
[能力矩阵 caps] [消息时间线 stream + 终端]   ← 保留现状，属于"运行时"，不进折叠区
[六个模块]
  ① 💬 聊天与账号      ② 📋 查询与记录    ③ ✨ AI 助手
  ④ 🤖 机器人能力      ⑤ 🛠 仅管理员      ⑥ 🔑 仅超级管理员
```

**关键变化**：折叠层从"12 个平铺块"变成"6 个模块（默认折叠）"，模块内部改用 `<section class="yyq-sec">` 分区（不用嵌套 `<details>`，避免折叠套折叠）；每模块有且只有一个"保存"入口（或在字段级即时保存），并有统一的一句话摘要（`3 个群 · NapCat · 已连接`）。

### 2.2 分组映射表

| 模块 | 装什么（现有内容） | 承接函数 | 默认 |
|---|---|---|---|
| ① 💬 聊天与账号 | 接入方式（OneBot/官方 seg）、WS 地址、Token/AppID/Secret、多群勾选、手工群号；**掉线自愈**（开关 + 间隔 + 上限）；**群→游戏转发**（off/keyword/auto/interval + 触发词 + 分钟/条数）；机器人身份与头像 | `yyQQAccess()` + `yyQQWatchdogBox()` + `yyRelayBlock()` 合并为 `yyQQModChat()` | 折叠，未接入时**自动展开** |
| ② 📋 查询与记录 | 互通端信息（服务器/群号/在线/已绑定/适配器 + 机器人卡）；消息时间线 + 终端；操作留痕（`yyQQC.actions` 的今日/失败）；危险指令记录入口 | `yyQQStream()` 保留在运行时区；`yyQQModRecords()` = serverinfo 块 + 留痕摘要（薄封装） | 折叠 |
| ③ ✨ AI 助手 | 提示词（唯一输入框 + 唯一保存按钮 + 变量 chips + 计数）；**模型只读行**（"当前语言模型：X · 去设置里改"）；接话模式（mention/interval/auto + 分钟数）；主动搭话（4 个数值 + 深夜窗口 + 静默时段）；识图三态；语音模式；沙盒试跑；**知识库**（三目录 + 上传 + 检索）；**AI 工具开关**（28→50 项，按 5 组） | `yyQQPersona()` 拆成 `yyQQModAIPrompt()` / `yyQQModAIBehavior()` / `yyQQModAIKnowledge()` / `yyQQModAITools()`，由 `yyQQModAI()` 聚合 | 折叠 |
| ④ 🤖 机器人能力 | 能力矩阵（caps，运行时区保留）；技能（`yyQQSkills()`）；收藏表情；**触发词规则**（含 `min_role` 下拉：所有人/管理员/超管）；**群规/违规词**（规则表 + 长期禁令）；**生图配置**（含模型选择——诉求 5 的落点） | `yyQQCaps()` 保留；`yyQQModRobot()` = `yyQQSkills()` + `yyQQFaces()` + `yyTrigBlock()` + `yyImgBlock()` + `yyM*` 块 | 折叠 |
| ⑤ 🛠 仅管理员 | 群管理动作（禁言/踢人/同意危险指令）与对应工具开关；触发词"仅管理员"层的批量视图；MCP 作用域（每台 MCP 的 `scope × access`，带"本机 MCP / 外部 MCP"类型标签）；危险指令（guard）白名单 | `yyQQModAdmin()` 聚合 `yyQQGuard` 块 + MCP 作用域视图；数据复用 `yyQQToolDefs`（`min_role>=1`）与 `/api/youyuan/mcp/scope` | 折叠，非管理员**整块不渲染** |
| ⑥ 🔑 仅超级管理员 | 超管/管理员名单（成员标识）；**认领二维码**（扫码 → 复制 `.认领 <码>`）；危险指令最终授权与留痕；**模型库**（地址/密钥/模型 ID，语言/画图/语音的凭证都在这）；掉线自愈的高级参数（间隔/上限） | `yyQQModRoot()` 聚合认领 + 名单 + 模型库入口 + watchdog 高级项 | 折叠，非超管**整块不渲染** |

> 生图、知识库、触发词、群规这 4 个原本"注入块"的归属如上；`互通端信息` 归 ②；`群服互通助手` 旧面板**不再作为模块**，降级为"①模块底部 → 高级设置 → 旧面板（默认 h-0 收起 + 可删除）"。

### 2.3 函数级改动清单

| # | 文件 / 函数 | 动作 | 说明 |
|---|---|---|---|
| A1 | `youyuan_qq_console.js` `yyQQShellHTML()` :2344 | **改写** | 返回 `Status + QuickNav + Hero + Grid(caps,stream) + 6×Module()`，不再拼 3 个旧块 |
| A2 | 新增 `yyQQModChat / ModRecords / ModAI / ModRobot / ModAdmin / ModRoot`（建议同文件，命名 `yyQQMod*`） | 新增 | 每个返回一个 `<section class="yyq-mod" id="yyq-mod-xxx">`，内部用 `.yyq-sec` 分区 |
| A3 | `yyQQAccess()` :1324 / `yyQQPersona()` :1488 / `yyQQTools()` :661 / `yyQQFaces()` :2071 | **拆** | 去掉外层 `<details>`，只返回 body；折叠壳交给 `yyQQMod*`。折叠状态字段从 `accessOpen/personaOpen/...` 迁到 `yyQQC.modOpen = {chat:0, ai:0, ...}` |
| A4 | `yyQQPersona()` 内部（:1516-1537） | **拆 5 份** | `yyQQAISelf` 只留"只读摘要行"；`yyQQCompanion/Proactive/Vision/Voice` 进 ③ 的"行为"分区；沙盒留在 ③ 底部 |
| A5 | `yyQQParts` :2210 / `yyQQPartList` :2233 | **改** | 键改为 `status/hero/caps/stream/mod-chat/mod-records/mod-ai/mod-robot/mod-admin/mod-root`，增量渲染按模块走 |
| A6 | `yyQQQuickNavItems` :2239 | **改** | 8 个旧锚点 → 6 个（`yyq-mod-chat/records/ai/robot/admin/root`）；`yyQQQuickNavSync()` 的"元素不存在就隐藏"逻辑正好适配权限隐藏 |
| A7 | **新增模块注册表** `window.yyQQInteropModules = []` + `yyQQRegisterModule({id,group,order,minRole,render,load})` | 新增 | 取代 6 处"包装 `yyQQRenderAll`"（`assistant/config/knowledge/serverinfo/moderation/guard`）。shell 每帧按注册表渲染，模块不再碰 DOM 顺序 |
| A8 | `youyuan_qq_config.js` `yyCfgMount()` :1052 | **删包装，改注册** | `yyTrigBlock/yyImgBlock/yyRelayBlock` 减去 `<details>` 外壳，注册到 `robot`/`chat` 两个 group |
| A9 | `youyuan_qq_knowledge.js` `mount()` :393 | **同上** | `block()` 去掉 `<details>`，注册到 `ai` group；`render()` 改为由模块 render 调用 |
| A10 | `youyuan_qq_moderation.js` `mount()` :585 | **同上** | 注册到 `robot`；`min_role` 列补进规则表（见 P1-5） |
| A11 | `youyuan_qq_serverinfo.js` `mount()` :723 | **同上** | 注册到 `records`，默认折叠、懒加载 |
| A12 | `youyuan_qq_guard.js` `mount()` :188 | **同上** | 注册到 `admin`，`minRole:1` |
| A13 | `youyuan_qq_assistant.js` `mount()` :772 / 旧面板 :411 | **拆** | 新折叠块并入 ③ 的"高级"；旧面板改成独立页 `yyQQAdvancedPage()`（从 ① 底部链接进入），不再进首页 DOM |
| A14 | `youyuan_qq_settings.js:28` | **改** | 加一行"互通页模型为只读展示"的说明，并给 ③ 的只读行提供跳转锚点 |
| A15 | `youyuan_qq_console.css` :780 | **改** | `.yyq-fold-grid` 由 2 列改 1 列（模块级）；`.yyq-mod` 内部 `.yyq-sec` 用 2 列；`@media` 保持单列 |
| A16 | `youyuan_qq_help.go:81/88/98-106` | **改** | 组名改 `🛠 仅管理员` / `🔑 仅超级管理员`，把 `game` 并入 `cap`，`skill` 保持不进正文；`yyQQHelpGroupTitle()` :110 对 `admin/root` 不再拼后缀 |

### 2.4 为什么必须去掉"包装 yyQQRenderAll"

现状是 6 个脚本依次把 `window.yyQQRenderAll` 包一层（`assistant → config → knowledge → serverinfo → guard → moderation`，见各文件 `wrapped.__xxWrapped = 1`）。这条链有 4 个硬伤，全都直接对应本次投诉：

1. **顺序耦合**：`index.html` 里 `assistant.js`(:156) 早于 `console.js`(:159)，`config/knowledge/serverinfo/moderation` 更晚；任何一个脚本抛错，后面全断，用户看到的就是"只有 console 自带的 4 块"。
2. **首屏漏渲染**：`yyQQConsolePage()` 用 `openSubPage()` 直接灌 HTML，不走 `yyQQRenderAll`，所以 :2773-2779 不得不手动补一次 `yyQQRenderAll()`；注释里已记录过一次事故（"RAG 和相关设置全没了"）。
3. **整壳重建会清空**：`yyQQRenderAll()` 在 `!yyQQShellReady()` 时 `root.innerHTML = yyQQShellHTML()`（:2424），所有注入块被连根拔掉，只能靠各 wrapper 再插一次；只要某个 wrapper 已经执行过 `return`（如 `if (window.yyQQRenderAll.__yykWrapped) return` 在错误时机命中），就永久缺失。
4. **状态各自为政**：8 份 `open` 变量 + `yyQQFp` 指纹缓存 + `__yyQQEditing` 焦点守卫，三者叠加导致"点了没反应/展开又被收回"（:2416-2419、:2400-2403）。

注册表方案把这四条一次消掉：**顺序由 `order` 决定**、**首屏与增量共用同一渲染入口**、**整壳重建后仍从注册表取内容**、**折叠状态集中在 `yyQQC.modOpen`**。

### 2.5 迁移风险（按严重度）

| 风险 | 表现 | 缓解 |
|---|---|---|
| R1 折叠状态丢失 | 用户原先展开的块重进后是收起 | 迁移期同时读旧字段（`accessOpen` 等）做一次性映射；`localStorage` 写 `yyq.modOpen.v1` |
| R2 表单失焦（老问题复发） | 输入到一半被定时刷新冲掉 | 保留 `__yyQQEditing` 焦点守卫，并把"模块级指纹"粒度从整块改到分区；**保存按钮仍走"只发改动字段"** |
| R3 权限隐藏导致"功能消失" | 非管理员看不到 ⑤⑥，会以为功能被删 | ⑤⑥ 在非授权角色下渲染为**一行灰条**「仅管理员可见 · 当前身份：普通成员」，而不是不渲染（与 `yyQQToolRow` 的 `available` 语义一致） |
| R4 旧面板入口被别的页面覆盖 | `yyQQPage` 被 `groups.js` / `console.js` 依次覆盖（`youyuan_qq_groups.js:3`、`youyuan_qq_console.js:2791`） | 旧面板改为独立 `openSubPage` 页面，不再依赖 `yyQQPage` 覆盖顺序 |
| R5 群内帮助与网页分组不一致 | 群里 7 组、网页 6 组 | 同一份 `yyQQHelpGroupBase` + 网页 `MODULE_LABELS` 从一个常量导出；加一条测试断言"网页模块数 == 帮助组数" |
| R6 生图/统一模型双写 | 生图块改模型，`roles.image` 没跟着变 | 生图块的 model 字段改为**只读 + 跳转**，或保存时同步 `POST /api/ai/models {roles.image}`（二选一，不能两套都写） |
| R7 终端多群改动波及后端 | `/api/qq/send` 加 `groups[]` 后老调用方（群指令、提醒）行为变化 | 保持 `group` 字段兼容：`group` 优先，`groups` 为新增可选；逐群发送并返回 `{sent:[], failed:[]}` |

---

## 三、修复清单（P0 / P1 / P2）

### P0（不修就无法验收，必须先做）

| # | 问题 | 动作 | 验收 |
|---|---|---|---|
| P0-1 | 首页只有 4 个折叠块，其余 8 个靠注入极易丢（①） | 落 `2.3 A1/A2/A7`：6 模块 + 注册表；删除 6 处 `yyQQRenderAll` 包装 | 冷启动 / 强刷 / 切页面回来三次，6 个模块都在，且顺序恒定；DevTools 里 `.yyq-fold` 数量 == 6 |
| P0-2 | 头像退化成"一个字"，且从来没有游戏头像（②） | Hero 与互通端信息头像：`bot_player` 优先 `/api/skin/online-head?player=`，QQ 头像走 `/api/skin/proxy`（禁直连 qlogo）；占位改成`皮肤头灰阶`而不是昵称首字 | 断网/外链失败时仍显示游戏皮肤头（本地缓存）；未绑定 `bot_player` 时才回落到 QQ 头像 |
| P0-3 | 用户找不到群规/触发词/生图/知识库入口（③ 的根因） | 随 P0-1 一并解决；⑤⑥ 非授权渲染灰条而非隐藏 | 普通成员能看到 6 个模块标题，⑤⑥ 有明确"权限不足"说明 |
| P0-4 | 掉线时看不到/改不了掉线自愈（⑥） | `yyQQHero()` :399 改为"二维码 + 掉线自愈条**同时**渲染"；`yyQQWatchdogToggle` 失败必须 `T(err,"e")` | 未登录状态下仍能改间隔/上限并保存成功；关闭失败有红色提示 |
| P0-5 | 互通页有 3 处模型选择，与统一配置打架（⑤） | 删 `yyQQConsole.js:1516` 的 `<select id="yyq-model">`、`:1776` 的绘图模型行、旧面板的模型区块 → 改只读行 + 「去设置」按钮 | 互通页搜索不到模型下拉；设置页改语言/画图模型后，互通页只读行即时更新 |

### P1（体验与能力对齐）

| # | 问题 | 动作 | 验收 |
|---|---|---|---|
| P1-1 | 提示词"存了又没"（③） | 收敛为**唯一写入者**：只保留 ③ 里的提示词框与保存按钮；旧面板的 `yy-qq-ai-prompt` 与 `yyQQAssistantSave` 里的 prompt 路径删除；保存成功后强制 `GET /api/qq/config` 回读 | 改 → 保存 → 切换页面 → 回来 → 刷新浏览器，四步后文本一致（人工 + `yyQQPromptSync` 单测） |
| P1-2 | 触发词缺角色分层（④） | `yyTrigBlock()` 表单加 `min_role` 下拉（所有人/管理员/超管），列表行显示徽章；后端已支持，只需透传 | 设"仅管理员"的规则，普通成员触发无反应、管理员触发有反应 |
| P1-3 | AI 工具只有 28 项，成员仅 9~10 项（④） | 补到 48~50 项，优先只读/低危：`query_*`（在线/绑定/分数/日志/服务器信息/玩家资料/皮肤）、`kb_search/kb_write`、`trigger_list`、`sticker_more`、`group_members`、`moderation_query`、`relay_status`、`help_search` 等；危险项仍 `Default:false` + `MinRole` | `/api/qq/ai/tools` 返回 `total >= 48`；成员集增加但危险项仍不可见（有单测断言 `ban/kick` 不在成员集） |
| P1-4 | 自动接话 interval 模式在新面板没有（⑧） | ③"行为"分区补 `mention/interval/auto` 三档 + "每 N 分钟"输入；复用 `assistant.ai_mode` + `interval_minutes` | 选 interval=3 分钟，观察 3 轮自检日志，且 `[SKIP]` 时不发消息 |
| P1-5 | 群规/违规词"改不了"（③） | ①规则表补"编辑"列已存在，问题在入口；②补 `min_role`/`source` 过滤与"仅超管可改自然语言禁令"的提示；③`writeRules` 改为**按 id 差量**提交，避免整表覆盖并发丢失 | 并发场景：另一人加规则的同时你改一条，两条都在 |
| P1-6 | 终端只能单群（⑫） | 前端：`yyQQTerm` 支持"发送到：当前群 / 勾选的 N 个群"；后端 `/api/qq/send` 增 `groups[]`（与 `group` 兼容），返回逐群结果 | 勾 3 个群发一条，3 个群都收到，时间线出现 3 条 `data-mine` |
| P1-7 | 主动搭话的深夜/静默不可配（⑬） | 补 `night_start/night_end`（默认 23/7）与 `quiet_ranges`（可多段）到 `yyQQProactiveConfig`；面板加"静默时段"两个时间选择器 | 设静默 00:00–08:00，期间 tick 不产生发送 |
| P1-8 | MCP 广泛/专一不可见（⑩） | ⑤ 内嵌"MCP 准入"表：每台 MCP 一行（类型：本机/外部 · scope · access · 归属人 · 启停）；数据走 `/api/youyuan/mcp/scope`；`yyMCPConn` 增 `kind:"local"/"external"` | 在互通页即可把某台 MCP 从"仅管理员"改成"全员可用"，改完群内成员工具列表变化 |
| P1-9 | 帮助菜单 7 组且标题重复（①） | `youyuan_qq_help.go`：`game` 并入 `cap`；`admin/root` 标题去掉重复后缀；`yyQQHelpGroupOrder` 同步 | 群里发"帮助"只出现 6 个分组标题 |
| P1-10 | 旧"群服互通助手"巨型面板默认存在（⑯） | 移到独立页 `yyQQAdvancedPage()`，首页不留 DOM；加 `assistant.helper_panel` 开关（默认关）；`指令帮助 <pre>` 改为按需 `GET /api/qq/help` 拉取 | 首页 DOM 里搜不到 `yy-qq-assistant-panel`；开关关闭时群内 `帮助` 也不推长文 |
| P1-11 | 知识库落盘失败只提示不阻断（⑭） | `persistence_error` 时上传/保存按钮置灰并红字说明 | 模拟只读目录，按钮不可点 |
| P1-12 | 收藏表情"点一下直发"只发一张（⑪） | 表情块支持多选 + "发选中 N 张"；与 `send_sticker` 能力对齐 | 选 3 张一次发出，时间线一条动作，`count:3` |

### P2（结构与文案）

| # | 问题 | 动作 |
|---|---|---|
| P2-1 | 界面"厚"（⑰） | 6 模块内统一 `.yyq-sec` 三级结构：`模块 → 分区（≤4 个）→ 字段`；每个分区一行摘要，展开才出字段；人设块从 8 段压到 3 段（提示词 / 行为 / 高级） |
| P2-2 | "写给 AI 看的"说明文案外泄（⑰） | 建立文案规范：UI 只写"操作者要做什么 + 后果"；实现细节（模型名匹配规则、分隔符、40% 窗口）移到 `title` / 「说明」折叠；模型行为规则只留在提示词（`youyuan_qq_ai.go:209-218`），不进 DOM |
| P2-3 | 快捷跳转 8 个旧 chip（①） | 改 6 个模块锚点，chip 文案与模块同名 |
| P2-4 | 转发开关双真值（⑦） | `assistant.relay_chat` 与 `relay.mode` 二选一：保留 `mode`，`relay_chat` 只做迁移读入，UI 只显示一个开关 |
| P2-5 | 认领二维码文案（⑨） | 图下加一行"手机 QQ 扫码 → 复制 → 发到群里"，并保留文本码折叠 |
| P2-6 | 统一模型与生图双写（⑤/⑮） | 生图块模型字段只读；或保存时同步 `roles.image`，二选一并写进注释 |
| P2-7 | 折叠状态持久化 | `localStorage: yyq.modOpen.v1`，跨刷新记住 6 个模块开合 |

---

## 四、给主控的三条判断

1. **本轮的核心不是"补功能"，是"补一层信息架构"**。17 条里 9 条的表象都是"功能在、用户点不到"，根因只有一个：所有东西都挂在同一个折叠层，靠 6 个脚本抢着往 DOM 里插。先做 P0-1（6 模块 + 注册表），其余 8 条会自动消失一半。
2. **两处"退化"要单独盯**：头像（外链 qlogo + 从来没有游戏皮肤头）和掉线自愈（掉线时被二维码顶掉）。这两个是"越修越差"的典型，改动小、观感收益大，建议与 P0-1 并行。
3. **"统一模型"已经是既成事实（`youyuan_ai_models.go` 写穿 + 设置页 UI）**，剩下的只是把互通里的 3 个旧下拉删掉。这件事不要重做，只做减法。
