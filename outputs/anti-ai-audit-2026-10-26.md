# MCORT 反 AI 化审查（2026-10-26）

审查对象：`upstream/frontend/**`、`auth-service/public/**`、`upstream/youyuan_qq_help.go`、`youyuan_qq_actions.go`、`youyuan_qq_*.js`
审查口径：只判断"像不像人做的"，不判功能对错。每条给 `文件:行`、原文、可直接替换的文本或 CSS 值。
本文件只做建议，未改动任何源码。

---

## 总评

**整体 AI 味：轻微。**

核心工作台（首页改版后、账号页、群服互通控制台、互通端信息、触发词配置）已经明显是人手打磨过的：
空状态带下一步动作、提示语有具体代价（"群里刷屏时游戏也会刷屏，建议配定时打包"）、
`01/02` 这类无信息量编号也没有出现。残留主要集中三块：

1. **首页 hero 的英文小标 + 四字标语**——唯一一处"落地页模板"。
2. **群服互通面板的开发者口吻空状态**（"后端未返回…稍后重试"）与 **emoji 标题**。
3. **认证站把实现说明写给用户看**，以及破坏性按钮和普通按钮同权重。

---

## P0（用户第一眼就会觉得假）

### P0-1 首页 hero 是标准落地页模板

- 位置：`upstream/frontend/js/pages/home.js:87`
- 现：`YOUR CREATIVE SPACE` / `让创作，更有条理。` / `建筑、插件与服务器，在这里有序管理。`
- 改：
  - eyebrow → `MCORT · 疣螈`
  - h3 → `连服务器、盖楼、管群，一个 APK 全干完。`
  - p → `建筑导进导出、插件、群服互通都在本机跑，断网也能用大半功能。`
- 为什么假：全大写英文 eyebrow + 逗号收尾的四字 slogan + 一句正确的废话，是模板三件套。
  更要紧的是"创作"和产品真实内容（托管机器人、群服互通、公网远控）对不上，
  用户第一屏看到的不是他能干的事。

### P0-2 "玩法插件（骨架阶段）"——开发排期直接写在界面上

- 位置：`upstream/frontend/js/mcort_theme.js:548-551`
- 现：`玩法插件（骨架阶段）` / `当前只做「发现 + 列出 + 记录启用」，不会加载或执行任何插件 main.js。执行层要等显式白名单 + 二次确认，避免把任意 JS 直接放进工具箱进程。`
- 改：
  - 标题 → `玩法插件`
  - 正文 → `这里只列出已安装的插件、记住开关，不会执行插件代码。`
- 为什么假："骨架阶段""执行层""显式白名单""放进工具箱进程"是给同事看的排期与风控说明。
  用户读到的潜台词是"这功能没做完"，比不说更伤。

### P0-3 三处"后端没返回…"的日志口吻空状态

- 位置与现文：
  - `upstream/frontend/js/youyuan_qq_console.js:760` → `机器人接口未返回能力定义，稍后重试`
  - `upstream/frontend/js/youyuan_qq_console.js:798` → `后端未返回技能定义，稍后重试`
  - `upstream/frontend/js/youyuan_qq_console.js:848` → `后端未返回工具定义，稍后重试`
- 改：
  - 760 → `机器人那边没给能力清单。确认 NapCat 连上了、版本别太旧，再点重试。`
  - 798 → `没读到技能表。去「AI 助手」把提示词保存一次，再回来刷新。`
  - 848 → `没读到工具表。重启一次机器人；还是空，说明这版 APK 没带工具清单。`
- 为什么假：三行同一个句式，主语还换了三次（机器人接口 / 后端 / 后端）。
  用户既不知道"后端"是谁，也拿不到下一步。这三处是全站最像 Copilot 补全出来的文案。

---

## P1（明显模板 / 明显不是人写的）

### P1-1 六个模块标题带 emoji，和文件自己的规矩打架

- 位置：`upstream/frontend/js/youyuan_qq_console.js:157-168`
- 现：`title: "💬 聊天与账号"`、`"📋 查询与记录"`、`"✨ AI 助手"`、`"🤖 机器人能力"`、`"🛠 仅管理员"`、`"🔑 仅超级管理员"`
- 改：全部去掉 emoji，只留 `聊天与账号` / `查询与记录` / `AI 助手` / `机器人能力` / `仅管理员` / `仅超级管理员`
  （`icon:` 字段已经给了 SVG，`yyQQModuleShell` 会渲染）
- 为什么假：同一文件第 55 行写着"后端 Icon 字段是 emoji，前端一律不用"，
  结果标题里手打了 6 个 emoji。于是每个标题前面同时出现"彩色 emoji"和"线性 SVG"两套图形语言。

### P1-2 认证站把"为什么没有皮肤图"用三种说法讲了三遍

- 位置与现文：
  - `auth-service/public/console.js:192` → `头像按“网易官方头像 → 此账号当前玩家自己的 MC 头 → 像素占位”顺序取用；MC 头由真实皮肤纹理裁剪，不是剪影。`
  - `auth-service/public/console.js:200` → `网易未开放头像列表与更换接口，官方头像按账号资料自动显示；皮肤可按组件码设置。`
  - `auth-service/public/console.js:240` → `接口只能读取当前账号已选用皮肤的真实纹理：上方第一个预览是当前账号皮肤的真实头部；组件码逐项应用后预览随之更新，未应用的组件码不显示皮肤图。`
- 改：
  - 192 → `头像优先用网易官方图；取不到的，拿这个账号的皮肤裁一个头。`
  - 200 → `头像换不了（网易没开这个接口）。想变样只能换皮肤，填组件码即可。`
  - 240 → `只有正在用的皮肤能出图；下面填的组件码要先点应用，才会有预览。`
- 为什么假：这是典型的"怕用户不懂，于是把设计文档贴上去"。
  一段话里连出"真实纹理""接口""像素占位"，还带箭头流程，读起来像 changelog。

### P1-3 破坏性操作和"开个开关"长得完全一样

- 位置：`auth-service/public/console.js:246`
- 现：`authButton('重签游戏令牌', …)`，默认 `secondary` 样式，与「开启定时刷新」「使用此账号令牌」同色同尺寸，只靠 `confirm()` 拦一下
- 改：
  - 文案 → `重签令牌（旧的立即作废）`
  - 加类 `danger`，并在 `auth-service/public/console.css` 末尾补：
    ```css
    .auth-action-row .danger{color:#a1542a;border-color:rgba(216,124,62,.35);background:rgba(216,124,62,.08);font-weight:600}
    .auth-action-row .danger:hover:not(:disabled){background:rgba(216,124,62,.14)}
    ```
- 为什么假：整个认证站 css 里 `grep danger` 是 0 条。所有按钮一个绿、一个浅灰，
  于是"作废旧令牌"和"打开定时刷新"视觉权重完全相同——这是模板化组件库的典型症状。

### P1-4 登录页"一个入口""即可"，加一个"一键"

- 位置：`auth-service/public/index.html:532`
- 现：h1 `连接游戏，从账号开始。`；p `一个入口，管理你的游戏账号与机器人。登录游园后，即可添加账号、选择皮肤和获取连接令牌。`
- 改：h1 → `登录游园`；p → `添加游戏账号、选皮肤、取连接令牌，都在这之后。第一次用先注册并验证 QQ 邮箱。`
- 为什么假：句号结尾的口号 + "一个入口" + "即可"，是 SaaS 落地页通用填充。
  而"第一次用先注册"这句真正有用的信息，被压在下面 h2 里重复了一遍。

### P1-5 「一键创建游客账号」

- 位置：`auth-service/public/index.html:534`
- 现：`<option value="register_guest">一键创建游客账号</option>`
- 改：`新建游客账号（不用注册）`
- 为什么假："一键"是本轮要清的词；括号里写明代价（不注册 = 游客），比形容词有用。

### P1-6 认证站背景混进了蓝紫两块氛围色

- 位置：`auth-service/public/index.html:94-99`（同源：`auth-service/public/console.css:18-22`）
- 现：三个径向渐变 `#d7f2e9`（青绿）、`#e2eefb`（蓝）、`#efe8fb`（紫）
- 改：
  ```css
  background:radial-gradient(900px 620px at 8% -10%,#dff0eb,transparent 60%),#f5f8f8;
  background-attachment:fixed;
  ```
- 为什么假：`--accent` 是 `#08796c` 青绿，背景却拉了一块天蓝和一块淡紫进来，
  三个色相互不相干。多色极光渐变 + 毛玻璃是 AI 生成主题的固定配方。

### P1-7 「液态玻璃」主题：iOS 系统蓝 + 四色极光 + 20px 圆角

- 位置：`upstream/frontend/css/base.css:260-281`
- 现：`--phx-primary:#0a84ff`（iOS 系统蓝）、`--phx-radius:20px`、`--phx-radius-sm:14px`、
  背景四色径向（`rgba(10,132,255,.32)` 蓝 / `rgba(124,58,237,.28)` 紫 / `rgba(34,211,238,.24)` 青 / `rgba(255,80,128,.18)` 粉）
- 改：
  ```css
  --phx-primary:#0e8f83; --phx-primary-hover:#0b7d72; --phx-primary-active:#096a61;
  --phx-primary-bg:rgba(14,143,131,.12);
  --phx-radius:14px; --phx-radius-sm:10px;
  ```
  背景只留两色：`radial-gradient(60% 50% at 18% 8%,rgba(14,143,131,.18),transparent 70%),#eef2f2`
- 为什么假：`#0a84ff` 是苹果系统色，跟品牌 `#087f79` 撞得很难看；四色极光 + 20px 圆角 + 28px 模糊，
  这三样凑在一起就是"AI 生成一个好看的玻璃卡片"的标准答案。

### P1-8 群服互通下半页的模块容器类没有样式

- 位置：
  - `upstream/frontend/js/youyuan_qq_console.js:2605` 外壳 `<div class="yyq-mod-grid">`
  - `upstream/frontend/js/youyuan_qq_console.js:255` 模块 `<details class="yyq-mod">`
  - `upstream/frontend/js/youyuan_qq_console.js:218` 分区 `<section class="yyq-sec">`
- 现状：`grep -rn "yyq-mod-grid\|\.yyq-mod{\|\.yyq-sec{" upstream/frontend/css/` 结果为空，
  这三个类在样式表里没有一条规则（`.yyq-grid` 有，`.yyq-mod-grid` 没有）
- 改（补到 `upstream/frontend/css/youyuan_qq_console.css` 末尾）：
  ```css
  .yyq-mod-grid{display:flex;flex-direction:column;gap:12px;margin-top:16px}
  .yyq-mod{border:1px solid var(--yyq-line-2);border-radius:var(--yyq-radius);background:var(--yyq-surface);overflow:hidden}
  .yyq-mod>summary{list-style:none;padding:11px 14px;cursor:pointer}
  .yyq-mod>summary::-webkit-details-marker{display:none}
  .yyq-mod-body{padding:2px 14px 14px}
  .yyq-sec+.yyq-sec{margin-top:12px;padding-top:12px;border-top:1px solid var(--yyq-line-2)}
  ```
- 为什么假：不是"AI 味"，是"没做完"的观感。六个模块只剩字号差，
  和上面有卡片有分隔的状态带、能力矩阵一对比，下半页像草稿。

### P1-9 首页副标题还留着口号

- 位置：`upstream/frontend/js/youyuan_dashboard.js:86`
- 现：`yyHeading('工作台','从创作到管理，把所有工具放在顺手的位置。')`
- 改：`yyHeading('工作台','这台设备上正在跑什么，一眼看完。')`
- 为什么假：页面内容就是"状态 + 任务 + 工具"，副标题却在讲情怀。
  改成如实描述，页面结构立刻自证。

### P1-10 托管池页面标题连续重复两遍

- 位置：`upstream/frontend/js/youyuan_accounts.js:1110`
- 现：`openSubPage("机器人托管池", yyIntro("机器人托管池", "统一查看多台机器人，按时间、身份和玩家筛选。") + …)`
- 改：`openSubPage("机器人托管池", yyIntro("按在线、身份、时间筛", "多台机器人一张表看完；点一行展开它的连接与绑定。") + …)`
- 为什么假：标题栏"机器人托管池"，往下 40px 又一个大字"机器人托管池"。
  人写界面不会连写两遍同一个词。

---

## P2（细节，但一处一处都能闻出味）

### P2-1 三个页面的副标题是同一种句式

- `upstream/frontend/js/youyuan_dashboard.js:122` 现 `随时连接你的设备` → 改 `公网远控`
- `upstream/frontend/js/youyuan_dashboard.js:125` 现 `插件持续运行，状态持续保留` → 改 `常驻插件`
  （"持续…持续"是同义反复，等于没说）
- `upstream/frontend/js/youyuan_dashboard.js:136` 现 `你的云端资料库` → 改 `云端资料库`
- 三处都建议把 h2 换成名词，说明句放 `<p>`：
  122 → `手机不在同一局域网时，用这里给的地址连回工具箱。`
  125 → `Python / C++ / Lua 各跑各的实例，重启工具箱后状态还在。`
  136 → `插件、建筑、音乐都存在云上，换设备能直接取回。`

### P2-2 空状态只剩一句"暂无…"

- `upstream/frontend/js/youyuan_qq_console.js:264` 现 `<p class="yyq-mod-empty">暂无内容</p>`
  → 改 `这一栏还没加载出来，点右上角刷新试试`
- `upstream/frontend/js/youyuan_qq_console.js:200` 现 `"暂无动作"` → 改 `"今天还没动过手"`
- `upstream/frontend/js/youyuan_dashboard.js:125` 现 `<div class="yy-empty">暂无运行中的实例</div>`
  → 改 `没有正在跑的实例。在左边填账号和插件 ID，点「启动插件」。`
- 理由：同目录 `youyuan_qq_serverinfo.js:325` 的空状态写着"等机器人连上服务器后，这里会自动出现进过服的人"，
  说明了这个项目本来能做到什么水平——这三处是漏网的。

### P2-3 同一条提示复制了 3–4 份，措辞还不一致

- 现：`令牌已复制到剪贴板，认证地址已显示在卡片里。` 出现在
  `auth-service/public/index.html:564`、`:574`、`auth-service/public/console.js:164`
- 现：`浏览器不允许自动复制，令牌已显示在卡片里，请手动选中复制。` 出现在同三处 + `index.html` 另一处
- 改：统一抽成 `showTokenCopied(copied)` 一个函数，只留一句
  `已复制` / `复制被浏览器拦了：令牌在上面，手动选一下`
- 理由：三份拷贝意味着三次改文案的机会漏掉两次；用户在不同入口看到两种语气。

### P2-4 后台枚举直接抛给用户

- 位置：`upstream/youyuan_qq_actions.go:439`
- 现：`"action 支持 like/qzone/delete_qzone/avatar/long_nick/nick/sticker"`
- 改：`这个动作不认识。可用的有：点赞、发动态、删动态、改头像、改签名、改昵称、发表情。`
- 理由：`action`、下划线枚举只对调用方有意义；群友/面板用户看到只会一脸问号。
  同文件 322 行的群内帮助已经写得很人话，这里没跟上。

### P2-5 术语越界两处

- `upstream/frontend/js/youyuan_qq_serverinfo.js:317`
  现 `placeholder="搜玩家名或QQ号"` —— "玩家"（游戏内角色）和"QQ号"（QQ 侧）塞进同一个搜索框，
  改 `placeholder="搜游戏内名字，或 QQ 尾号"`（若只能搜一样，就删掉另一样）
- `upstream/frontend/js/youyuan_market.js:456`
  现 `还没有写简介，编辑资料补充后更容易被玩家找到。`
  改 `还没有写简介。补一句，别人在插件列表里更容易看懂它干什么。`
- 理由：项目铁律是 玩家=游戏内角色 / 群友=QQ 侧。这两处把两边混在一个句子里。

### P2-6 插件/主题页给用户的安装说明是路径和文件名

- `upstream/frontend/js/mcort_theme.js:483` 现 `把主题包放到数据目录的 themes/<id>/，需要 theme.json + theme.css。`
  → 改 `主题包解压到 数据目录/themes/，一个主题一个文件夹。`
- `upstream/frontend/js/mcort_theme.js:555` 现 `还没有插件。放到数据目录的 plugins/<id>/，需要 plugin.json + main.js。`
  → 改 `还没有插件。插件包解压到 数据目录/plugins/，一个插件一个文件夹。`
- 理由：`<id>` 这种占位符语法是写给读文档的人看的，不是写给正在用 App 的人看的。

### P2-7 「一键」还剩一处

- `upstream/frontend/js/features/ai_assist.js:37`
  现 `还没有模型。点右上角「模型设置」添加一个（填入供应商/请求地址/密钥，可一键获取模型列表）。`
  改 `还没有模型。点右上角「模型设置」填供应商、请求地址和密钥，点「获取模型」会把列表拉回来。`
- 理由：与 P1-5 同源。

### P2-8 11 套主题的名字像模板市集

- 位置：`upstream/frontend/js/theme.js:21`
- 现：`light:'浅色青绿', dark:'深色青绿', forest:'墨绿自然', amethyst:'暗紫优雅', ocean:'海洋蓝', sakura:'樱花粉', midnight:'午夜深蓝', sepia:'复古棕', glass:'液态玻璃', neon:'霓虹', cream:'奶油'`
- 改：留 4 套（浅色/深色/墨绿/复古棕），名字改成 `浅色` `深色` `墨绿` `暖棕`；
  其余下线，或改成 `高对比` `护眼` 这种按用途命名的
- 理由：`暗紫优雅` `樱花粉` `奶油` 是"氛围感命名"，和这个产品（连服务器、盖楼）不搭；
  11 套也超出任何人的选择成本。

### P2-9 后台口吻的"该用户"

- 位置：`auth-service/public/index.html:574`
- 现：`该用户尚无托管游戏账号。`
- 改：`这个用户名下还没有托管账号。`
- 理由：认证站不是工单系统，"该用户"是坐席视角。

---

## 明确建议保留（别顺手删了）

| 位置 | 内容 | 说明 |
| --- | --- | --- |
| `upstream/frontend/index.html:3-4` | `MCORT · Minecraft Open Robot Toolbox` + `<meta name="author" content="疣螈">` | 品牌与署名，必须留 |
| `upstream/frontend/js/youyuan_dashboard.js:71` | 侧栏 `MCORT` + `MINECRAFT OPEN ROBOT TOOLBOX` + `MCORT · 作者 疣螈` | 署名 |
| `upstream/frontend/js/youyuan_dashboard.js:139` | 页脚 `MCORT · 作者 疣螈 · v… · APK 内置运行环境` | 署名 + 环境，信息量足 |
| `upstream/frontend/js/mcort_theme.js:291/316/476` | `MCORT · 作者 疣螈`、`MCORT 标识与作者署名属于硬编码层` | 署名 |
| `upstream/youyuan_qq_help.go:62-106` | 群内帮助的分组 emoji（`💬 聊天与账号` 等） | QQ 场景里 emoji 合理，**不要**跟 P1-1 一起删 |
| `upstream/frontend/js/youyuan_qq_serverinfo.js:325` | `还没有玩家数据 / 等机器人连上服务器后…` | 空状态范文，保持 |
| `upstream/frontend/js/youyuan_qq_config.js:203` | `比如群里有人发「赞我」，机器人就去点赞——点上面的模板或「新建规则」开始` | 空状态范文，保持 |
| `upstream/frontend/js/youyuan_qq_config.js:927` | `群里刷屏时游戏也会刷屏，建议配「定时打包」` | 说人话的代价提示，保持 |

---

## 改法建议（按投入产出排序）

1. 先改 P0 三条 + P1-1（emoji 标题）+ P1-8（补模块样式）：改动量都在 10 行以内，第一屏和群服互通页立刻不一样。
2. 再改 P1-2 / P1-3 / P1-4（认证站）：认证站是外部用户第一站，"内部说明 + 同权重按钮"最伤可信度。
3. 最后扫 P2：`暂无…` / 重复提示 / 术语三处，机械替换即可，不动结构。

> 说明：本报告中 P1-8 属于"样式遗漏"而非审美判断，但它的表现（下半页没有块级分隔、没有视觉主次）
> 恰好落在"布局 AI 味"的判定口径里，所以放在 P1，且给了可直接粘贴的 CSS，不需要重构。
