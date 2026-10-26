# Agent-C（工具箱前端两处 UI 修复）进度

## 2026-10-23 · 步骤 1：读取与根因定位（已完成）

### 修复点一：连接诊断面板（youyuan_connection_diagnostics.js/css）

现状代码（`js/youyuan_connection_diagnostics.js:132`）：面板 `class="yy-server-hero"`，插在 `#yy-connection-server-info` 之后（anchor 在时）或 `afterbegin`（anchor 不在时，抢设置页最顶部）。

根因（已 Locate 到证据）：
1. 「这么大」：`yy-server-hero` 是 `css/youyuan_room_discovery.css:5-6` 给服务器信息 hero 的宽幅样式——`border-radius:20px`、`box-shadow:0 20px 44px -34px`、`::before` radial-gradient 渐变底、`margin:0 0 24px`、`overflow:hidden`。`.content-inner`（`css/layout.css:38`）只是 `width:100%` 的普通块容器，`grid-column:1/-1` 被忽略，但阴影/圆角/渐变全部生效，所以诊断面板"这么大"。
2. 「错位」：`youyuan_room_discovery.css:26` `.yy-server-hero .yy-room-current-actions{order:3;margin-left:auto;justify-content:flex-end;flex-wrap:nowrap}`——诊断面板操作行（按钮 + 日志路径 span）被 nowrap，390px 窄屏不换行 → 横向溢出被卡片 `overflow:hidden` 裁掉 → 错位。
3. 插入位置：anchor 丢失时（如 `youyuan_connection_policy.js:135` `if(!yySettingsStillLive()) return;` 提前返回导致 hero 没插）回退 `afterbegin` → 抢设置页最顶部。脚本顺序：connection_policy.js(145) 先于 connection_diagnostics.js(146) 加载，正常情况下 anchor 就绪、面板在 hero 之后；但 fallback 分支是真实风险。

### 修复点二：进出记录头像（youyuan_qq_serverinfo.js/css）

现状：
- 玩家行（`js/youyuan_qq_serverinfo.js:327` personRow）：有 `<span class="yysi-face"><i>首字母</i>+img</span>` 结构，`avaImg`（:90）链 = 同源相对路径直出 / 外链走 `/api/skin/proxy?url=` + onerror 回退 data-orig 原图直连，再失败 `el.remove()` 露出底层首字母色块。这条链对玩家头像（后端给 `/api/skin/online-head?player=`，`upstream/youyuan_qq_serverinfo.go:140-145`）是通的。
- **进出记录（eventsPanel，:503）：完全没有渲染头像**——后端 `yySrvInfoHandleEvents`（`upstream/youyuan_qq_serverinfo.go:435-440`）的 event 结构体只有 `Time/Type/Player/QQ`，**没有 avatar 字段**；前端 eventsPanel 也只有 dot + 名字 + 时间。用户说的「原本都还有头像这些显示的 但是点进去了」= 玩家 tab 有头像、点进进出记录没有。→ 根因之一在**后端 events 接口不返回 avatar**（不越界改，前端用 players 列表同款地址兜底，证据已留档）。
- hero 区 bot 头像（:223）：`bot.avatar` 为空时才渲染 `.yysi-ava-txt` 首字母占位；**img 在时代理/直连全失败 → `el.remove()` → 位置只剩空白**（没有垫底占位）——这是"不留空白"要求的缺口。bot 头像源是外链 `https://q1.qlogo.cn/g?b=qq&nk=…&s=640`（`upstream/youyuan_qq_image.go:102-107`），走 `/api/skin/proxy`。
- CSS 无隐藏 img 的规则（已查 `youyuan_qq_serverinfo.css` 与 `youyuan_qq_console.css`，`.yysi-face img` 正常 absolute 覆盖）。

### 修复方案（下一步执行）
1. 诊断面板：`yy-server-hero` → 新 `.yy-conn-diag-card` 普通卡片；`<details>` 默认折叠（摘要 = 徽章 + 今日掉线次数 + 展开详情），展开才 load + 15s 定时，收起停表；fallback 插末尾（beforeend）不再抢顶；grid/hint/actions 全部用 `.yy-conn-diag-card` 前缀在自有 CSS 重写。
2. 进出记录：eventsPanel 加头像（players 映射优先 → 现场拼 `/api/skin/online-head` 兜底 → 首字母色块垫底）；hero bot 头像改「首字母永远垫底」结构，全失败不留空白；CSS 加 `.yysi-face-hero`/`.yysi-face-ev` 变体，390px 不错位。

## 2026-10-23 · 步骤 2：修复点一完成（连接诊断面板紧凑化）

改动文件：`upstream/frontend/js/youyuan_connection_diagnostics.js`、`upstream/frontend/css/youyuan_connection_diagnostics.css`

before → after：
- 样式：`<section class="yy-server-hero">`（20px 圆角 + 0 20px 44px 大阴影 + radial-gradient 渐变底 + `overflow:hidden`）→ `<details class="yy-conn-diag-card">`（14px 圆角、细边框、`--phx-bg-card`、无阴影无渐变）。
- 位置：anchor 在 → hero 之后（afterend，保留）；anchor 不在 → `afterbegin` 抢设置页最顶部 **改为** `beforeend` 沉到内容区末尾。
- 交互：整面板常开 → 默认折叠 `<details>`；摘要行 = 标题 + 在线/离线徽章 + 今日掉线次数 + 「展开详情▾」；展开才 `load(false)` + 15s 定时刷新，收起 `clearTimeout` 停表；`live()` 增加 `!!host.open` 判断；渲染时不再无条件 load/schedule。
- 错位：删掉对 hero 专属样式的依赖（不再用 `yy-server-state`/`yy-room-current-actions` 类）；grid/hint/actions 全部以 `#tab-settings #yy-conn-diag .yy-conn-diag-*` 前缀重写；操作行 `flex-wrap:wrap` + 日志路径 `flex:1 1 100%` 独立成行，390px 不再被 overflow:hidden 裁切。
- API 不变：`yyRefreshConnDiag(force)` 保留，折叠时被调用会先自动展开再刷新（否则折叠态调用会静默失效）；数据字段 id 全部保留，另加摘要行 `yy-diag-today-sum`。

验证：
- `node --check` 通过（SYNTAX_OK）。
- node vm + 假 DOM 冒烟（smoke_conn_diag.js，会话目录，非项目文件）全过：① 只插一次且在 anchor 之后(afterend)、details 无 open 属性、摘要含徽章/计数/展开详情、折叠态 0 请求；② 展开后请求 1 次且 draw 落全部字段（徽章在线+success、今日掉线 3、事件列表、日志路径）；③ 收起后 0 新请求；④ 折叠态调 yyRefreshConnDiag(true) 自动展开并刷新+toast；⑤ cleanup 正常。
- 390px 断点：CSS 静态核对——`@media(max-width:600px)` grid 2 列 + row 纵排（原有规则保留）；摘要行 flex-wrap:wrap；操作行 wrap + 日志 100% 换行；`.yy-conn-diag-count` min-width:0 + overflow-wrap:anywhere。浏览器实测由主 Agent 部署后统一做。

## 2026-10-23 · 步骤 3：复核回合（修复点一终验，无新代码改动）

背景：本回合重新派发时，两个文件已处于步骤 2 的修复后状态。本轮做逐条复核 + 运行时验证，未再改代码（避免破坏已验证状态）。

### 五项修改要求逐条核对（全部满足）
1. 样式类：HTML 为 `<details class="yy-conn-diag-card">`，CSS:7 = 14px 圆角 + `--phx-border` 细边 + `--phx-bg-card`，无 box-shadow、无渐变；HTML/CSS 中已无 `yy-server-hero` 引用（grep 证实仅剩 room_discovery.css 自身定义）。
2. 位置：js:172-173 anchor 在且 `parentElement===root` → afterend；否则 `beforeend` 沉底。已核实 anchor（`youyuan_connection_policy.js:142-148`）就是 `#tab-settings>.content-inner` 的直接子节点，正常路径必走 afterend；fallback 不再 afterbegin。
3. 默认折叠：`<details>` 无 open 属性；summary = 标题 + `yy-diag-badge`（在线/离线）+ 今日掉线 `yy-diag-today-sum` + 「展开详情▾」；js:134-138 toggle：展开 load+schedule、收起 clearTimeout；js:177 live() 含 `!!host.open`；draw/load 均经 live() 守卫，折叠态零请求零定时器。
4. 错位修复：grid/hint/actions 全部以 `#tab-settings #yy-conn-diag` 前缀重写在自有 CSS；操作行 flex-wrap:wrap + 日志 `flex:1 1 100%` 独立成行；`@media(max-width:600px)` grid 2 列 + row 纵排。
5. API 不变：`window.yyRefreshConnDiag(force)` 保留（折叠态调用先自动展开再刷新）；数据字段 id 全保留，新增摘要行 `yy-diag-today-sum`。

### 兼容性旁证（旧 WebView）
- `ontoggle` 是本项目既有量产模式：youyuan_accounts.js:1753、youyuan_qq_console.js:810/932/1221、youyuan_qq_config/knowledge/moderation/guard/serverinfo 等十余处均用 `<details ontoggle>`，settings_groups/plugin_studio 用 addEventListener('toggle')——旧内核可用已被现网功能验证。
- `.js.esb` 为历史遗留孤儿文件：index.html 与全部 Go 代码只引用 `.js`（grep `.esb` 零引用），`.js` 是唯一生效文件，无需同步 .esb。
- 脚本顺序 index.html:145(policy)→146(diagnostics)，Rcfg 包装链保证 anchor 先插入、诊断面板后插到其后。
- CSS 经 index.html:17 全局加载；面板复用共享类 yy-badge/yy-hint/yy-text-btn（设置页既有类）。
- color-mix 仅用于事件行 tone 底色，room_discovery.css:6 已在用同款函数，不支持时退化为无底色，不崩。

### 验证结果
- `node --check`（22.22.2-6）：JS_SYNTAX_OK。
- 禁用语法扫描（?. / ?? / .at( / replaceChildren / Object.fromEntries / flatMap）：0 匹配；文件保持 var/function/indexOf/slice 的 ES5 风格。
- CSS 括号平衡：花括号 32/32、圆括号 27/27。
- node vm + 假 DOM 冒烟（smoke_conn_diag.js，会话目录，非项目文件）：20/20 全过——① afterend 插入+默认折叠+折叠态 0 请求 0 定时器+摘要行三要素；② 展开 1 请求+15s 定时器+draw 全字段（徽章/计数/状态/事件/分布/日志）；③ 收起清真实定时器句柄；④ 折叠态 yyRefreshConnDiag(true) 自动展开+刷新+toast；⑤ cleanup 后 live 失效；⑥ anchor 丢失 beforeend 兜底且仍默认折叠。（过程中修正过 1 处测试断言：schedule 开头有防御性 clearTimeout(null)，收起时 cleared 计数应为 2 而非 1——代码行为正确，断言写错。）

### 390px 断点推演（静态）
视口 390px：`.content-inner` 满宽，面板 ≈358px。摘要行 flex-wrap，标题(nowrap)+徽章+计数(overflow-wrap:anywhere) 可换行不溢出；详情 body 左右各 14px padding → grid 可用 ≈330px，`repeat(auto-fit,minmax(150px,1fr))` 自然得 2 列（且 ≤600px 媒体查询强制 2 列）；stat 卡 12px 圆角、strong 18px overflow-wrap:anywhere 长数字可断行；事件行 ≤600px 纵排；操作行 wrap、日志路径 100% 独立成行——原 hero 的 overflow:hidden + nowrap 裁切路径已彻底移除。
