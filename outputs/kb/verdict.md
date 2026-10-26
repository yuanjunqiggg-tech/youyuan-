# 验收报告 2026-10-26

> 验收人：verifier（独立挑刺，立场是找「看起来做完了其实没做完」）
> 方法：**只读**。所有结论来自 Grep / Read / 只读 git log / 只读快照比对；未改任何源码、文档、配置，未部署、未重启。
> 术语纪律按用户明令执行：**玩家**=游戏内角色，**用户/群友**=QQ 侧的人，**账号**=登录工具箱的账号。
> 声明：本轮**未实际渲染页面**。移动端尺寸均为 **CSS 推断，未实测**，已逐条标注。

---

## 一、功能真实性核查

| 功能 | 声称状态 | 实际证据 | 结论 |
|---|---|---|---|
| 头像选择 · 三个标识符是否存在 | 已完成 | yyPickAvatarDialog 定义在 upstream/frontend/js/youyuan_accounts.js:851；#yy-profile-preview 在 js:836（HTML 串）与 js:841（getElementById）；#yy-profile-avatar 在 js:836 与 js:852。三处全部存在 | **真** |
| 头像选择 · 是否有调用点（关键：防「零调用」） | 已完成 | js:836 同一段 HTML 里有 onclick="yyPickAvatarDialog()"，且 js:853 执行 el.click() 触发隐藏的 input type=file（onchange="yyPickAccountAvatar.call(this)"）。**有调用点，不是死函数** | **真** |
| 头像选择 · 端到端链路是否闭合 | 已完成 | 选图 js:855-881 → 校验 ≤5MB / PNG·JPEG js:859 → canvas 裁 144×144 js:869-872 → 预览 js:874 → 提交 js:887-888 → 后端 yyValidateAccountAvatar（upstream/youyuan_account_profiles.go:73）→ 落盘 :102-116 → 合并进账号行（upstream/youyuan_accounts.go:218）→ 展示时 data:image 优先（js:2396-2400）→ 渲染点 js:2615、js:2654。**全链闭合，不是半截代码** | **真** |
| 头像入口可发现性 | 未声称 | 入口埋两层：账号行操作菜单「设备备注 / 分类」（js:646）→ 二级页「账号资料」（js:836）。玩家要点 2 次才看到「选择头像」按钮 | **真但深**（不是假，是可发现性偏弱） |
| 种子地图 CSS 增量标记 | 已完成 | 起始标记 css/youyuan_seedmap_v2.css:836「2026-10-26 增量 · 移动端可用性与浅色主题边界」，结束标记 css:928。两处都在文件**末尾** | **真** |
| 种子地图 CSS「只追加不改」 | 已完成 | 行数 833→928（+95 行）；字节 23,954→27,417（+3,463 B）；实测增量块内容 3,367 B + 95 个 CRLF = 3,462 B，与 +3,463 差 1 B。**纯追加成立**，未改写既有规则 | **真** |
| 种子地图版本戳 2026102826 | 已完成 | index.html:26 CSS = ?v=2026102826 **对**；index.html:138 accounts.js = ?v=2026102826 **对**；index.html:179 **seedmap_v2.js = ?v=2026102824 错** | **部分**（JS 没升） |
| 增量是否真的上线 | 已部署 | cloudflare/public/css/youyuan_seedmap_v2.css 18,548 B（已压缩）。5 条增量特征在压缩文本中逐条命中：sm2-side:before ×1、orientation:landscape ×1、max-height:480px ×1、outline-offset ×2、data-theme=light ×2、168px ×1 | **真**（部署副本与源码同步） |
| 部署副本头像入口 | — | cloudflare/public/js/youyuan_accounts.js 含 onclick="yyPickAvatarDialog ×1、yy-profile-avatar ×1 | **真** |
| 移动端触摸目标提升 | 已优化（增量 ①） | 见 1.1 节表格，8 个目标里只改了 3 个 | **部分** |

### 1.1 移动端断点与尺寸（CSS 推断，未实测）

css/youyuan_seedmap_v2.css 全部 @media 断点（7 处）：

| 行号 | 条件 |
|---|---|
| 749 | (max-width: 1180px) |
| 753 | (max-width: 1023px) |
| 791 | (max-width: 640px)（原有） |
| 828 | (prefers-reduced-motion: reduce) |
| 847 | (max-width: 640px)（本次增量 ①） |
| 880 | (max-height: 480px) and (orientation: landscape)（本次增量 ②） |
| 915 | (max-width: 1023px)（本次增量 ⑤） |

640px 以下各触摸目标实际尺寸：

| 元素 | 原尺寸（行号） | 640px 以下 | 是否被增量覆盖 |
|---|---|---|---|
| .sm2-mini | 27px（:229） | **40px**（:848-851） | 是 |
| .sm2-chip | 29px（:196） | **40px**（:852-855） | 是 |
| .sm2-dock button | 30×30（:412） | **40×40**（:856-859） | 是 |
| .sm2-clear | 20×20（:72） | 32×32（:860-863） | 是，但 32 < 44 |
| .sm2-info-head button | — | 32×32（:864-867） | 是，但 32 < 44 |
| .sm2-empty .sm2-mini（空态「重试」） | 31px（:533） | **仍是 31px** | **否**（特异性 0,2,0 压过 0,1,0） |
| .sm2-close-drawer（抽屉关闭） | 26×26（:813-814） | **仍是 26px** | **否**（增量完全没碰） |
| .sm2-drawerbtn（打开抽屉） | 36px 高（:465） | **仍是 36px** | **否** |
| .sm2-go（查询按钮） | 38px 高（:135） | **仍是 38px** | **否** |
| .sm2-dims button（维度切换） | 容器 38px（:109） | **仍是 38px** | **否** |

结论：增量 ① 的注释自己写着「原 20~30px，低于 44px 建议线」，但改完是 **40px，仍在 44px 线以下**；且 .sm2-empty .sm2-mini（31px）与 .sm2-close-drawer（26px，移动端唯一可见的抽屉关闭按钮）**根本没被覆盖**。

---

## 二、编造抽查

抽 14 个标识符 / 行号断言去源码 Grep + Read 逐条对位：**命中 14 个，命中率 14/14（100%），未命中 0 个**。

| # | 文档断言 | 出处 | 实测 | 结果 |
|---|---|---|---|---|
| 1 | 版本 0.10.78 @ youyuan_product.go:14 | source-map.md:3 | const yyAppVersion = "0.10.78" 在第 14 行 | 命中 |
| 2 | 只绑 127.0.0.1 @ main.go:5289 | source-map.md:19 | net.Listen("tcp", "127.0.0.1:"+port) 在 5289 | 命中 |
| 3 | //go:embed frontend/** @ main.go:52-53 | source-map.md:24 | 52=注释、53=var frontendFS embed.FS | 命中 |
| 4 | 中间件 yyConsoleAuth(yyIntegrityGate(corsHandler)) @ main.go:5280 | source-map.md:21 | 5280 行完全一致 | 命中 |
| 5 | v2 劫持旧入口 @ youyuan_seedmap_v2.js:2375-2379 | gaps.md:29 | 2377 行 if (yyModules[i].id === 'seed')，2379 legacy.open = function(){ return yySeedMapPage(); } | 命中 |
| 6 | 种子查询器入口 @ js/features/others.js:16 | gaps.md:22 | 16 行 t: '种子查询器'，act 调 RseedMapHTML() | 命中 |
| 7 | --sm2-map-line 定义 1 处、使用 0 处 @ css:9 | gaps.md:54-55 | 全文仅 9 行 1 处定义，无任何 var(--sm2-map-line) | 命中 |
| 8 | sm2-info-near 是死 id | gaps.md:50-51 | 全文仅 js:1934 1 处（写入），无读取 | 命中 |
| 9 | /api/skin/online-head @ main.go:5194 | api-inventory.md:363 | 5194 行完全一致 | 命中 |
| 10 | /api/skin/online-texture @ main.go:5195 | api-inventory.md:366 | 5195 行完全一致 | 命中 |
| 11 | /api/youyuan/accounts @ youyuan_accounts.go:375 | api-inventory.md:381 | 375 行 mux.HandleFunc("/api/youyuan/accounts", ...) | 命中 |
| 12 | bridge 三端点 @ youyuan_account_bridge.go:335/342/357 | api-inventory.md:382-384 | 335=profiles、342=execute、357=local，全部对位 | 命中 |
| 13 | 头像校验函数 yyValidateAccountAvatar | source-map.md:251 | youyuan_account_profiles.go:65 定义，73 行做 data:image 前缀判定 | 命中 |
| 14 | yyAccountProfileView 供账号行合并 | source-map.md:251 | 定义 youyuan_account_profiles.go:51，调用 youyuan_accounts.go:218 | 命中 |

**未发现编造。** 这批文档的「文件:行」是可复核的真值，不是凭印象写的。

---

## 三、反 AI 化问题

**核查结论：编写型文档里没有 AI 套话，未发现问题。** 逐项给出核对范围与数字，不凑指标。

| 文件 | 赋能 | 闭环 | 一站式 | 全方位 | 深度融合 | 打造 | 助力 | 强大 | 高效 | 流畅 | 极致 | 精心设计 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| api-inventory.md | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| frontend-map.md | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| gaps.md | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| mcp-surface.md | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| plugin-runtime.md | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| seed-v2.md | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| source-map.md | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| Lua插件开发说明.md | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| Python插件开发说明.md | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| C++插件开发说明.md | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| D:/游园/README.md + LOG.md | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| 前端新增文案（accounts.js / seedmap_v2.js） | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

关于三份插件文档那 3 处「闭环」：Lua插件开发说明.md:2555、Python插件开发说明.md:2555、C++插件开发说明.md:2554，标题均为「## 用AI形成操作闭环」，指 AI 调用链的收尾，是技术表述而非营销套话。**不计为问题。**

关于全库统计出的 220 次「智能」、17 次「闭环」、5 次「强大」、14 次「高效」、2 次「流畅」：**逐条回查上下文，全部是用户原话的逐字转写或引用**——「智能」全库共 235 次，其中 220 次在 longest-session-user-words.md、4 次在 conversations.md、4 次在 longest-single-message.md、7 次在 user-words-index.md，逐条查看**全部**是「子智能体 / 智能体」这一用户自造词；17 次「闭环」全在 longest-session-user-words.md，指「显示闭环 / 功能闭环」；5 次「强大」是用户原话「子智能体的作用还是很强大的」「一个超强大脑」；14 次「高效」中 12 次在转写稿、2 次在 conversations.md:292 与 user-words-index.md:281 且都是同一句用户原话「这样才能高效的完成」；2 次「流畅」是用户原话「测试一下流畅度」。**这些不是 AI 写的文案，对转写稿做反 AI 化打分是错靶，不计入问题，也不作为凑数。**

自我表扬类（「精心设计」「我们实现了」「完美」「无缝」「彻底解决」「优雅地」）全库 0 命中。

---

## 四、术语混用

| 文件:行 | 原文 | 应该是 | 说明 |
|---|---|---|---|
| upstream/frontend/js/youyuan_seedmap_v2.js:972 | 「粗图只发一个请求，网络慢的时候用户至少能看见地形轮廓。」 | 「操作者」或「玩家」 | 这里「用户」指操作网页控制台的人，按团队纪律「用户」=QQ 侧的人，语义漂移。**属代码注释，不影响玩家所见**，列为最低优先级 |
| 其余 | — | — | 未发现「玩家/用户/账号」三者互换。youyuan_accounts.js 里 29 处「玩家」全部指游戏内角色（:1096 搜索占位、:1239 未绑定玩家、:1376 "玩家 " + bound），20 处「用户」全部在代码注释里指操作者。**账号页面向玩家的文案没有一处把「账号」写成「玩家」或把「玩家」写成「账号」** |

结论：**术语基本干净，只有 1 条注释级漂移。**

---

## 五、结论

### 5.1 「假」的有几项

**功能层面「假」（声称做完但没真做）：0 项。**
头像选择、种子地图 CSS 增量、部署副本三项都经得起查：有定义、有调用点、有端到端链路、线上副本与源码一致。

**「部分」2 项**（做了，但没做到声称的程度）：
1. 种子地图版本戳——CSS / accounts 是 2026102826，index.html:179 的 seedmap_v2.js 还是 2026102824。
2. 移动端触摸目标——8 个目标只改了 3 个；改到的 3 个也只到 40px（自称要修 44px 线）；.sm2-empty .sm2-mini 因特异性未被覆盖仍是 31px，.sm2-close-drawer 仍是 26px。

**文档层面「假」：1 项，且这是本轮最严重的问题。**

### 5.2 最严重的 1 条（必须改）

**outputs/kb/gaps.md:64-67 声称三份知识库「未产出」，实际它们在 20 分钟前就已落盘。**

gaps.md 原文：
- gaps.md:64  第 1 项 源码地图   「429 中断，未产出」  依据栏写「无 outputs/kb/source-map.md」
- gaps.md:65  第 2 项 API 清单   「429 中断，未产出」  依据栏写「无 outputs/kb/api-inventory.md」
- gaps.md:67  第 4 项 插件运行时 「429 中断，未产出」  依据栏写「无 outputs/kb/plugin-runtime.md」

反证：
- outputs/kb/source-map.md 52,557 B，mtime 2026-10-26 15:33:47
- outputs/kb/api-inventory.md 60,926 B，mtime 2026-10-26 15:35:37
- outputs/kb/plugin-runtime.md 39,883 B，mtime 2026-10-26 15:35:53
- outputs/kb/gaps.md mtime 2026-10-26 15:54:50，文件头自述「生成：2026-10-26 15:55」
- git log：三份文件随 8cad63a「docs(kb): 知识库首批产出」一起入库，**gaps.md 自己也在同一个提交里**

即：写 gaps.md 的那一刻，这三个文件已经在同一个目录里躺了 20 分钟，而且会被同一个 commit 提交。这不是「口径分歧」，是**对自身目录状态的事实性错误陈述**。

**连带影响（必须让 team-lead 知道）**：任务 #2「补齐被 429 打掉的 4 份知识库（源码地图/API清单/插件运行时/经验库）」的前提是假的——4 份里 **3 份已存在**，只有 experience-log.md 真缺（ls 确认不存在）。kb-filler 若按原任务书重写，有覆盖已有产出的风险。

### 5.3 其他真实缺陷（B 级，建议排期）

| # | 文件:行 | 问题 | 证据 |
|---|---|---|---|
| B-1 | upstream/frontend/index.html:179 | seedmap_v2.js 版本戳 2026102824，与声称的 2026102826 不一致 | 见第一节表格 |
| B-2 | upstream/frontend/js/youyuan_seedmap_v2.js:145 | ensureCss() 回落到旧戳 ?v=2026102601，同一 CSS 存在两个版本戳 | link.href = '/css/youyuan_seedmap_v2.css?v=2026102601'。gaps.md B-2 已自行披露，未修 |
| B-3 | upstream/frontend/css/youyuan_seedmap_v2.css:533 vs :848 | .sm2-empty .sm2-mini { height: 31px }（特异性 0,2,0）压过 @media 内 .sm2-mini { height: 40px }（0,1,0），空态「重试」按钮在手机上仍是 31px | 两条规则逐字比对 |
| B-4 | upstream/frontend/css/youyuan_seedmap_v2.css:813-814 | .sm2-close-drawer 26×26，且在 :788 被 display: inline-flex !important 在移动端强制显示——这是手机上唯一的抽屉关闭按钮，26px 严重偏小，增量 ① 完全没覆盖 | 逐条比对 |
| B-5 | upstream/youyuan_capture_runtime.go:349 | profile["nickname"] 是死分支：yyAccountProfileView（youyuan_account_profiles.go:51-63）返回的键是 id/name/avatar/profile_role/role_source/game_server_owner_known，**没有 nickname**，该分支永不进入 | 逐键比对。功能未断（下一行 a.Name 兜底），属死代码 |
| B-6 | outputs/kb/gaps.md:112、D:/游园/README.md:35-36 | RAG 规模三处数字互不相同且全部过期：gaps.md 写「11,643 块 / 16.5MB」，README 写「12,182 块 / 19MB」，实测 index.jsonl **15,062 行 / 13,940,364 B**、youyuan.db **24,342,528 B（23.2MB）** | wc -l + ls -la |
| B-7 | upstream/youyuan_account_profiles.go:68 vs :75 | 头像限制口径不一致：首检 len(value) > 140000 报「小于96KB」，实际解码后再检 len(b) > 96*1024。140000 个 base64 字符约 105KB，两个阈值不是同一个数 | 读函数体 |
| B-8 | outputs/kb/source-map.md:19 | 「109 个 script」实为 grep -c 的**行数**，不是标签数。真实 script 开标签 118 个，带 src 的 .js 111 个（frontend-map.md:17 写的 111 才是对的）。同一事实两份文档给了 109 和 111 | grep -o "<script" 得 118；带 src 的 .js 得 111 |

### 5.4 明确「不是问题」的（避免误伤，逐条排除）

- cloudflare/public/css/youyuan_seedmap_v2.css 只有 18,548 B 且搜不到中文注释标记 → **不是没部署**，是压缩产物；5 条增量特征在压缩文本里逐条命中。
- frontend-map.md 的 111 / 41 / 112 / 43 四个数字 → **全部核对正确**（按 grep -o 而非 grep -c 复算）。
- plugin-runtime.md 声明基线 3a11d92 → **该 commit 真实存在**，且正是 KB 提交前一个版本，声明准确。
- D:/游园/备份库 的目录结构 → **真实存在**：4 个快照（20261026-1606/1617/1632/1712）、LATEST、LOG.md、manifest.json 齐全；maintenance/backup_snapshot.py、backup_restore.py 均在。
- D:/游园/LOG.md → 主动写了「未完成」（GitHub push 失败）、「遗留」（Cloudflare 隧道已删）、「红线」（youyuan 全程未动），**没有粉饰**。
- 三份插件文档各 1 处「闭环」、转写稿里的 220 次「智能」→ 均属技术表述或用户原话，**不判为 AI 化**。

### 5.5 一句话总结

**代码是真的（14/14 抽查命中，头像与种子地图链路闭合，线上副本同步），文档的反 AI 化也是真的（编写型文档 0 处套话）。唯一「假」的是一份验收文档自己——gaps.md 把三份已经存在的知识库说成不存在，而它和那三份文件还在同一个 git 提交里。**

---

## 六、附：三份插件文档补写（任务 #1）复核

> 追加于 api-doc 报完成后，应其要求对齐口径。复核对象：`Lua插件开发说明.md` / `Python插件开发说明.md` / `C++插件开发说明.md`。

| 声称 | 实测 | 结论 |
|---|---|---|
| 每份 333 条方法（`^### \`ns.method\``） | 三份各 **333** | 真 |
| 三份集合与顺序完全一致 | `diff` 三份方法清单，**零差异** | 真 |
| 漏写 0、过期 0 | 以 `outputs/kb/api-registered.json`（25 个命名空间）反算：文档 333、注册 333，**双向差集均为空** | 真 |
| 补了 129 个方法（非任务书的 136） | 行多重集比对：净新增 `### ` 标题 **129** | 真 |
| 130 处补写标记 | 每份 `2026-10-26 补写：源码注册面同步` **130** 处（129 小节 + api 表） | 真 |
| 9 个新命名空间章节 | 一级标题真实存在：`# accounts · 跨账号调用`(3119)、`accountsPool`(3179)、`diagnostics`(3349)、`entities`(3431)、`entity`(3480)、`hud`(3508)、`mcp`(3546)、`observerBridge`(3606)、`scoreboard`(3897)；目录由 16 行增至 **25** 行 | 真 |
| 7 个 `api.call` 附加方法有真实实现 | `seed.manifest/structures/biome/slime/stronghold/summary` 在 `upstream/youyuan_seedv2_sdk.go`；`packet.catalog` 在 `upstream/youyuan_observation_summary.go:21`，均登记进 `yyExtraMethods` | 真 |
| getTPS「当前宿主无 TPS 数据源，返回 nil」 | `bot_conn.go:710-713` `observedTPS()` 无条件 `return 0, false`；`youyuan_game_dispatch.go:25-33` 插件侧 `!known` 时 `return nil, nil`。**插件侧确实是 nil，说法正确** | 真 |
| 长描述是源码原样搬运 | `accountsPool.captureConfigure` 与 `youyuan_capture_runtime.go:16-23` 逐字一致（含「自动1/2/3、>10为5、>=15为6台」这句别扭表述，源码本身如此） | 真 |

### 6.1 一处口径已整改（原措辞会被 git 打脸，非造假）

api-doc 自述「改动是纯插入，diff 里没有删除或替换」。实测：

```
git diff --numstat  →  每份 1614 新增 / 106 删除
git diff | grep -c '^-[^-]'  →  每份 42 行删除
```

**但净内容零丢失**：行多重集比对（`collections.Counter(旧) - Counter(新)`）结果为 **真丢失 0 行**，净新增 1,508 行。删除的 42 行是 `packet` / `bot` / `chains` / `media` 等整块因插入新章节而位移，属**块移动**，不是改写。

问题在于「diff 里没有删除」这句话会被任何人跑一次 `git diff` 直接推翻。

**api-doc 已接受整改并撤回原话，改后口径如下（本轮已逐项复验，措辞准确）：**

> 内容零丢失（旧行多重集包含于新行，实测丢失 0 行）；`git diff` 显示每份 1614 增 / 106 删，106 行全部是 packet/bot/chains/media 等章节块位移被 diff 判成删除后重加，无改写、无替换。

其补充的两个数据点我也独立复验通过：

| 数据点 | 复验结果 |
|---|---|
| `git show HEAD:` 是 LF，`.bak` 是 CRLF，只差 3,198 个 CR | **成立**：HEAD blob 176,169 B / 3,198 行 / CRLF 0；`.bak-20261026` 179,367 B / 3,198 行 / CRLF 3,198；差 3,198 B 恰等于行数 |
| 工作区保持 CRLF，故 diff 无整文件换行噪声 | **成立**：现文件 4,706 行 / CRLF 4,706 / 裸 CR 0，无混行尾。`core.autocrlf=true`（仓库未设 `.gitattributes`）使 git 比对前归一化 CRLF→LF，故不出现 3,198 删 + 4,706 增 |
| 1614 − 106 = 1508 = 净新增行 | **成立**：4706−3198 = 4777−3269 = 4776−3268 = 1,508，三份一致 |

### 6.2 已知问题（判定为不修，理由成立）

**`accountsPool.captureConfigure` 可读性未优化**——单段约 700 字、20+ 个分号从句连成一气。事实无误（与 `upstream/youyuan_capture_runtime.go:16-23` 逐字一致），但这是玩家唯一能看到该说明的地方。

我原建议加分号换行，api-doc 拒绝，理由我复核后**认可**：

1. Markdown 列表项内的单个换行是软换行，渲染仍并成一段，视觉上与分号无差别——改了等于没改。
2. 真要可读必须拆成子列表，那会破坏「一节一个要点列表」在 333 节里的统一形态，代价大于收益。
3. 该段是源码字符串逐字搬运，为排版改动就得改源码文字，反而制造「文档与注册表不同步」的新风险。

**最终判定：不修。记录为「可读性未优化，内容与源码逐字一致」。** 若日后要改，正确做法是先改 `youyuan_capture_runtime.go` 的 doc 字符串，再同步三份文档，不要只改文档。

### 6.3 提醒（非错误）

TPS 的「返回 nil」对插件侧成立；`upstream/ai_game.go:215-219` 的 AI 工具调用路径返回的是字符串「暂不可用」。三份文档目前均未出现「暂不可用」（实测 0 处），**没有混用**，仅提醒后续别把两条路径的返回值写串。
