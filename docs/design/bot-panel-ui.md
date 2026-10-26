# 群服互通 · 机器人能力面板 重设计方案

目标：`upstream/frontend/js/youyuan_qq_console.js`、`upstream/frontend/css/youyuan_qq_console.css`
约束：只复用现有 token（`--phx-*`）、组件（`.card` / `.btn` / `.yy-hint`）、图标系统（`yyIcon()`）。不引入构建步骤、图标字体、第三方库。

## 0. 诊断
现在是「5 张等高卡片，每张里一个标题、一段解释、几个输入框」。它把**配置表单**当成了**控制台**。
控制台不是把所有设置摆出来，而是**先回答"现在什么状态"，再让人做最小的动作改变状态**。

## 1. 调研结论

| 项目 | 做法 | 借 | 不借 |
|---|---|---|---|
| **Koishi 控制台** | 左栏切页；插件配置是**行式列表**（图标+名称+右侧开关），点开才展开表单；沙盒独立 | 开关用**行**不用卡片；配置默认折叠 | 树形多级分组（我们只有 10 个能力） |
| **AstrBot WebUI** | 导航：仪表盘/插件市场/配置管理/日志/Agent；仪表盘是**指标**不是表单 | 顶部放状态与诊断数字；日志独立成块 | 每个模块做成一整页 |
| **NapCat WebUI** | 未登录时**全屏单任务**（Token 或扫码），登录后才出面板 | 「未连接」收敛为接入引导，不露能力/提示词 | 纯文字登录页的简陋视觉 |
| **LangBot WebUI** | 顶部常驻状态（处理量/成功率）；实体卡片化；WebUI 取代 JSON | 顶部状态带；一个群=一张卡 | 流水线编排画布 |
| **nonebot-htmlrender** | 消息卡片层级：头像→昵称→时间→正文→操作 | 实时流单条消息照此排 | 图片渲染 |

**共同点**：这五个面板里**没有一个**把「开关」画成 230px 大卡片，全是行。卡片只用于**实体**（插件包、群、机器人）。

## 2. 信息架构

| 场景 | 用户在想 | 需要的界面 | 频次 |
|---|---|---|---|
| A 首次接入 | 「装好 NapCat 了，然后呢」 | 全屏引导，不出别的模块 | 一次 |
| B 临时收手 | 「有人在用 .踢 乱踢，先关掉」 | 能力一屏可见、一击生效、行内反馈 | 高 |
| C 调人设 | 「说话太像客服了」 | 大编辑区 + 变量插入 + 就地试跑 | 低 |
| D 排障 | 「它为什么不回话」 | 链路状态 + 最近消息 + 最近失败，同屏 | 高 |

B、D 必须同屏；A、C 必须能收起。故：

```
┌──────────────────────────────────────────────────────┐
│ 0 状态带 sticky 56px  ● 已连接 NapCat·3 群·12s 前 [刷新]│
├──────────────────────────────────────────────────────┤
│ 1 身份卡 18px                                         │
│  [头像] 布汲岛小助手 QQ 3921…        ┌──────────────┐ │
│  互通群│游戏身份│好友/群│今日动作      │NapCat→网关→3 群│ │
├──────────────────────────────┬───────────────────────┤
│ 2 能力矩阵 1fr                │ 3 实时流 336px sticky  │
│  全部/互动/形象/管理           │   [消息][动作]         │
│  [icon]点赞  2分/3次      [○] │  12:04 张三:开服了吗   │
│  [icon]收藏表情 不限      [●] │  12:04 机器人→已回复   │
├──────────────────────────────┴───────────────────────┤
│ 4 接入与群（折叠·未连接时展开）│ 5 人设与试跑（折叠）   │
│ 6 收藏表情（折叠·默认收起）                            │
└──────────────────────────────────────────────────────┘
```

**理由**：0 必须 sticky——D 场景下滚到哪都要看见「连没连」；1 是"我是谁"、2 是"能做什么"、3 是"刚做了什么"，三者成闭环放一屏；4/5/6 配一次就不动，折叠成 56px 横条，把首屏让给 2 和 3。4 的展开条件写成 `state !== "open"` 时 `open`，A 场景自动进引导，不必单独做页面。

**删掉**：`yyIntro()` 那句「OneBot 机器人的身份、能力开关、表情包与提示词，都在这里管理。」——功能罗列等于没说。换动态状态句：「正在运行 · NapCat · 3 个群」。

## 3. DOM 结构与函数对应

### 3.1 渲染入口：统一 render，不再局部 `outerHTML`

现状 `yyQQConsoleProfile()` 用 `host.outerHTML = yyQQConsoleHero()`（`js:176`）替换节点，会丢焦点、跳滚动。改为按 id 局部更新：

```js
var yyQQC = { cfg:{}, status:{}, caps:[], profile:null, faces:[], actions:[], messages:[],
              tab:"caps", stream:"msg", group:"", loading:{caps:1,stream:1} };
function yyQQRender(part){                    // status|hero|caps|stream|access|persona|faces
  var el = document.getElementById("yyq-" + part);
  if (el) el.outerHTML = yyQQParts[part]();   // 每个 id 只由自己的函数产出
}
```

挂载点：`yyq-status` / `yyq-hero` / `yyq-caps` / `yyq-stream` / `yyq-access` / `yyq-persona` / `yyq-faces`。

### 3.2 状态带 `yyQQStatusBar()`

```html
<div class="yyq-status" id="yyq-status" data-state="open">
  <span class="yyq-dot"></span><span class="yyq-status-text">已连接</span>
  <span class="yyq-status-meta">NapCat · 3 群 · 最近消息 12 秒前</span>
  <button class="yyq-iconbtn" onclick="yyQQRefresh()" aria-label="刷新">SVG(refresh)</button>
  <button class="yyq-iconbtn" onclick="yyQQMenu()" aria-label="更多">SVG(more)</button>
</div>
```

`data-state` 对齐后端 `yyQQState`：`open|connecting|reconnecting|stopped|error|waiting_callback|unconfigured`。

```css
.yyq-status[data-state="open"] .yyq-dot{background:var(--phx-success,#13876a)}
.yyq-status[data-state="connecting"] .yyq-dot,.yyq-status[data-state="reconnecting"] .yyq-dot{background:var(--phx-warning,#c48424);animation:yyqPulse 1.6s infinite}
.yyq-status[data-state="error"] .yyq-dot{background:var(--phx-error,#d25050)}
.yyq-status[data-state="unconfigured"] .yyq-dot,.yyq-status[data-state="stopped"] .yyq-dot{background:var(--phx-text-disabled,#97a4a8)}
```

`yyQQStateText()`（`js:13`）漏了 `error` 与 `waiting_callback`，会直接显示英文，需补「连接失败」「等待回调」。

### 3.3 身份卡 `yyQQHero()`

```html
<section class="yyq-hero" id="yyq-hero" data-state="open">
  <div class="yyq-hero-id">
    <img class="yyq-avatar" ...>                        <!-- 空则 .is-empty + SVG(robot) -->
    <div class="yyq-hero-name"><strong>布汲岛小助手</strong>
      <div class="yyq-hero-sub"><span class="yyq-statechip" data-state="open">在线</span><span>QQ 3921000123</span></div>
    </div>
    <button class="yyq-ghost" onclick="yyQQPickGroup()">切换群</button>
  </div>
  <dl class="yyq-facts">
    <div><dt>互通群</dt><dd>布汲岛生存服</dd></div><div><dt>游戏身份</dt><dd>Stev</dd></div>
    <div><dt>好友 / 群</dt><dd>128 / 3</dd></div><div><dt>今日动作</dt><dd>47 <small>失败 2</small></dd></div>
  </dl>
  <ol class="yyq-chain" aria-label="链路">
    <li data-ok="1">NapCat<span>ws://127.0.0.1:3001</span></li><li data-ok="1">OneBot v11</li><li data-ok="1">3 个群</li>
  </ol>
</section>
```

- 三个灰 chip → **状态点+文字**，只有「在线」带色，其余降为普通文本。现状三个 `yy-bot-tag` 权重相同（`css:13`），读者分不清哪个是状态。
- 新增 `.yyq-chain`：D 场景核心，一眼看出断在哪环。`data-ok="0"` 变灰+虚线边框，其后节点同灰。
- `facts` 去掉每格独立边框背景，改**竖线分隔**（`border-left:1px solid var(--phx-border-light)`，首格无），减少「4 个小框」的廉价感。

### 3.4 能力矩阵 `yyQQCaps()`（改动最大）
```html
<div class="yyq-caps" id="yyq-caps">
  <div class="yyq-tabs" role="tablist">
    <button class="is-on" data-group="all">全部 <small>10</small></button>
    <button data-group="interact">互动 <small>3</small></button>
    <button data-group="identity">形象 <small>4</small></button>
    <button data-group="manage">管理 <small>3</small></button>
  </div>
  <ul class="yyq-cap-list">
    <li class="yyq-cap" data-kind="like" data-on="1">
      <span class="yyq-cap-ico">SVG(thumb)</span>
      <span class="yyq-cap-body"><strong>点赞</strong><small>给群友的名片点赞，每人每天 3 次</small></span>
      <span class="yyq-cap-meta">冷却 2 分 · 每日 3</span>
      <label class="yyq-sw"><input type="checkbox" checked onchange="yyQQToggle(this)"><span></span></label>
    </li>
  </ul>
  <p class="yyq-cap-foot">成员及以上可用 · 所有动作均留痕</p>
</div>
```

- 行高 60px（移动 68px），`grid-template-columns:40px minmax(0,1fr) auto 44px`。`data-on="0"` 整行 `opacity:.55` + 图标底色变灰；**不隐藏**，用户要看见"我关了什么"。
- 指令码不再常显（现状 10 个虚线框=噪音），收进行尾「?」展开的详情。
- 开关用 `label>input+span`，44×26，`span` 做轨道、`::after` 做滑块，避免现状 `.yy-cap-toggle::before` 在部分 WebView 的定位偏差。
- **整行点击 ≠ 切开关**（现状整行 `label` 包住 checkbox，点文字就误触）。整行点击开详情，只有开关响应。

`yyQQToggle()` 需改：现状 `card.querySelector("strong")` 取名（`js:260`），新结构改 `input.closest(".yyq-cap").querySelector(".yyq-cap-body strong")`。保存中给开关加 `data-busy`（轨道内 12px 转圈），而非静默 `disabled`。

### 3.5 实时流 `yyQQStream()`（新模块）

数据来自已有接口：`GET /api/qq/messages?group=`（`{messages:[{id,sender_id,name,text,timestamp,is_bot,reply_to_message_id}]}`）与 `GET /api/qq/actions`（`{actions:[{time,kind,caller,name,target,detail,ok,error}]}`）。

**合并成一条时间线**，不做两个列表——排障要回答的是「他 12:04 发了消息，机器人 12:04 报了错」，分开的列表无法对齐时间。

```html
<aside class="yyq-stream" id="yyq-stream">
  <div class="yyq-stream-head"><div class="yyq-seg"><button class="is-on">消息</button><button>动作</button></div>
    <span class="yyq-live"><i></i>实时</span></div>
  <ol class="yyq-tl">
    <li class="yyq-tl-item" data-kind="in"><span class="yyq-tl-ico">SVG(chat)</span>
      <div><b>张三</b><time>12:04</time><p>今天开服了吗</p></div></li>
    <li class="yyq-tl-item" data-kind="out" data-ok="1"><span class="yyq-tl-ico">SVG(fly)</span>
      <div><b>机器人</b><time>12:04</time><p>已回复 · 320ms</p></div></li>
    <li class="yyq-tl-item" data-kind="act" data-ok="0"><span class="yyq-tl-ico">SVG(ban)</span>
      <div><b>点赞</b><time>12:05</time><p>失败：目标 QQ 不存在</p></div></li>
  </ol>
  <div class="yyq-stream-foot"><button class="yyq-ghost">加载更早</button></div>
</aside>
```

- 左侧 2px 竖线（`.yyq-tl::before`），图标压线并 `background:var(--phx-bg-card)` 打底形成断口。
- `data-kind`：`in` 灰 / `out`·`act` 主色 / `fail` 红。**只染图标，不整行染色**。
- 桌面 `position:sticky;top:56px;max-height:calc(100vh - 96px);overflow:auto`；新消息只在流内滚动，且仅当用户已在底部（`scrollHeight-scrollTop-clientHeight<24`）才自动跟随。

### 3.6 接入与群 `yyQQAccess()`

```html
<details class="yyq-fold" id="yyq-access" open>
  <summary><span class="yyq-fold-ico">SVG(connection)</span><span class="yyq-fold-t">接入与互通群</span>
    <span class="yyq-fold-sum">NapCat · ws://127.0.0.1:3001 · 3 群</span><span class="yyq-fold-caret">SVG(arrow)</span></summary>
  <div class="yyq-fold-body">
    <div class="yyq-seg yyq-seg-wide"><button class="is-on">OneBot / NapCat</button><button>QQ 官方</button></div>
    <div class="yyq-fields">
      <label class="yyq-field"><span>WebSocket 地址</span><input id="yyq-ws"></label>
      <label class="yyq-field"><span>Token</span><input type="password" placeholder="已保存"></label>
    </div>
    <div class="yyq-group-list">…群卡网格…</div>
    <label class="yyq-switchrow">…转发群聊进游戏…</label>
    <div class="yyq-fold-actions"><button class="btn">保存并重连</button><button class="btn-s">测试连接</button></div>
  </div>
</details>
```

- 适配器从 `<select>` 改两段式 segmented：只有两个选项，select 要两次点击。
- 群从「输入框 + chip 流」改**群卡网格**（群名/人数/勾选角标）。现状 `yyQQConsolePickGroup()` 只把 id 塞进输入框（`js:211`），看不出选了哪个群。
- 按钮「保存」→「保存并重连」：现状 toast 说"正在重新接入"（`js:240`），按钮只说"保存"，承诺与行为不一致。

### 3.7 人设与试跑 `yyQQPersona()`

把「系统提示词卡」与「试一句」合并成**沙盒**：
```html
<details class="yyq-fold" id="yyq-persona">
  <summary>…人设与试跑…<span class="yyq-fold-sum">简短 · 先给结论</span></summary>
  <div class="yyq-fold-body">
    <div class="yyq-prompt-bar">
      <div class="yyq-var-chips"><button>{玩家}</button><button>{群名}</button><button>{时间}</button></div>
      <span class="yyq-count" id="yyq-prompt-count">0 / 2000</span>
    </div>
    <textarea id="yyq-prompt" rows="6"></textarea>
    <label class="yyq-field"><span>使用模型</span><input id="yyq-model" placeholder="默认模型"></label>
    <div class="yyq-sandbox"><div class="yyq-sandbox-log" id="yyq-sandbox-log"></div>
      <div class="yyq-sandbox-input"><input id="yyq-try" placeholder="在沙盒里试一句，不会发到群里">
        <button class="btn" onclick="yyQQAsk()">试跑</button></div></div>
    <div class="yyq-fold-actions"><button class="btn">保存人设</button></div>
  </div>
</details>
```

- 变量 chip 点击插入到光标处，别让人手打 `{玩家}`。
- 「试一句」改沙盒：就地渲染问答气泡。现状 `T("已发到互通群","o")`（`js:300`）其实调 `POST /api/qq/send` 真发群——语义欺骗，用户会把测试消息发进群。
- 落库仍用 `POST /api/qq/config {assistant:{prompt,model}}`。

### 3.8 收藏表情 `yyQQFacesView()`

保留网格，改三处：`minmax(88px,1fr)`→`minmax(96px,1fr)`；`height:88px`→`aspect-ratio:1` + `object-fit:cover`（现状长图被压扁）；`figcaption` 常显→hover/focus-within 显示（移动端常显但截断）。补 `decoding="async"`。
## 4. 视觉规范

**圆角 3 档**：页面级容器/hero/折叠块 `18px`（移动 `16px`）；卡内子块（群卡、沙盒、气泡）`12px`；输入框/按钮/能力行 `10px`；chip/状态点/开关 `999px`。现状 20/14/12/11 四档无规律（`css:4,17,31,47,64`）。

**间距（8pt）**：区块间距 `20px`；主区两栏 gap `20px`；卡内 padding `20px`（移动 `16px`）；能力行 `padding:0 14px` 行高 `60px`；行内 gap `12px`/`8px`；折叠 summary 高 `56px`。

**字号（只允许 6 档）**：标题 `22/1.3/650`（hero 昵称）；区块 `14/1.4/600`（折叠块标题）；正文 `13/1.6/400`（输入值、能力名）；辅助 `12/1.6/400`（描述、群名）；微 `11/1.5/400`（时间、元信息）；标 `10/1.4/600` + `letter-spacing:.06em`（`dt` 标签）。现状 20/16/14/13/11/10，16 与 14 太近。

**阴影（只给 hero）**：

```css
--yyq-shadow:0 1px 2px rgba(20,50,50,.04),0 12px 32px -24px rgba(20,50,50,.28);
--yyq-shadow-lift:0 2px 4px rgba(20,50,50,.05),0 16px 40px -24px rgba(20,50,50,.34);
```

能力行、群卡**不用阴影**，只 1px 边框——阴影泛滥是"廉价"第一来源。现状 hero 是 `0 20px 44px -34px rgba(15,32,40,.6)`，透明度 .6 在暗色主题下发脏。

**颜色**：全走 token——`--phx-primary` / `--phx-border(-light)` / `--phx-bg-card` / `--phx-bg-secondary`；语义 `--phx-success|warning|error|text-disabled`；强调底 `color-mix(in srgb,var(--phx-primary) 8%,var(--phx-bg-card))`（项目已有用法）。**禁止** `#d9534f` 这类裸色（`css:56` 在用）。

**动效**：

```css
--yyq-ease:cubic-bezier(.22,1,.36,1);
.yyq-cap{transition:background .15s var(--yyq-ease),opacity .15s}
.yyq-sw span::after{transition:transform .18s var(--yyq-ease)}
@media(prefers-reduced-motion:reduce){*{animation:none!important;transition:none!important}}
```

## 5. 图标方案

**复用 `yyIcon()`（`youyuan_dashboard.js:32`）与 `yyIconPaths`（`:2`）**。该文件在 `index.html:127` 加载，早于 `youyuan_qq_console.js`（`:152`），全局可用，无需新 sprite 机制。
可直接复用：`chat` `image` `user` `camera` `pen` `list` `connection` `spark` `shield` `clock` `search` `arrow` `fly` `settings`。

新增 12 个（统一 24×24、`fill:none`、`stroke:currentColor`、`stroke-width:1.6`，与 `.yy-icon` 一致）：

```js
robot:'<rect x="4" y="8" width="16" height="11" rx="3"/><path d="M12 4v4"/><circle cx="12" cy="3" r="1"/><circle cx="9.5" cy="13" r="1.3"/><circle cx="14.5" cy="13" r="1.3"/><path d="M9.5 16.5h5"/>',
power:'<path d="M12 3v9"/><path d="M6.5 6.6a8 8 0 1 0 11 0"/>',
qr:'<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><path d="M14 14h3v3h-3zM20 14v2M14 20h2M20 19v2"/>',
activity:'<path d="M3 12h4l2.5-7 4 14 2.5-7H21"/>',
sliders:'<path d="M4 8h9M18 8h2M4 16h3M12 16h8"/><circle cx="15.5" cy="8" r="2.2"/><circle cx="9.5" cy="16" r="2.2"/>',
face:'<circle cx="12" cy="12" r="9"/><path d="M8.5 14.5a4.5 4.5 0 0 0 7 0M9 9.6v.01M15 9.6v.01"/>',
refresh:'<path d="M20 12a8 8 0 1 1-2.4-5.7"/><path d="M20 4.5V9h-4.5"/>',
ban:'<circle cx="12" cy="12" r="9"/><path d="m6 6 12 12"/>',
key:'<circle cx="8" cy="12" r="4"/><path d="M12 12h9M17.5 12v3M15 12v2"/>',
thumb:'<path d="M7 11v9H4.5A1.5 1.5 0 0 1 3 18.5v-6A1.5 1.5 0 0 1 4.5 11H7Z"/><path d="M7 11l3.6-7.2A2 2 0 0 1 12.6 4v4h4.6a2 2 0 0 1 2 2.4l-1.3 6.4A2 2 0 0 1 15.9 20H7"/>',
tag:'<path d="M3 12.4V5a2 2 0 0 1 2-2h7.4a2 2 0 0 1 1.4.6l7.6 7.6a2 2 0 0 1 0 2.8l-6 6a2 2 0 0 1-2.8 0L3.6 13.8a2 2 0 0 1-.6-1.4Z"/><circle cx="8" cy="8" r="1.4"/>',
trash:'<path d="M4 7h16M9.5 7V4.8h5V7M6.5 7l.9 13h9.2l.9-13"/>',
```

**能力 kind → 图标（前端决定，忽略后端 emoji）**：后端 `yyQQCapability.Icon` 是 emoji（`youyuan_qq_caps.go:35-46`：`👍 😄 🖼 📮 🗑 🧑 ✒️ 🏷 👢 📋`）。前端不用 `c.icon`，改本地映射；后端字段保留（群指令 `.能力` 的 emoji 排版不在本次范围）。

```js
var yyQQCapIcon = { like:"thumb", emoji_like:"face", face:"image", qzone:"fly",
  delete_qzone:"trash", avatar:"camera", long_nick:"pen", nick:"tag",
  kick:"ban", relay_history:"list" };   // 兜底 "spark"
```

**尺寸**：能力行/折叠块 18px，状态带 16px，空态插图 20px（容器 40px）。现状 `.yy-cap-icon{font-size:16px}`（`css:34`）承载 emoji，整块删除。

## 6. 状态呈现

### 6.1 未连接 / 未登录（A 场景）

三层，不要堆在一张卡：
① **状态带** `data-state="unconfigured"` → 灰点 +「未配置」+ 右侧主按钮「去接入」，其余按钮隐藏。
② **hero** `.yyq-chain` 三节点全 `data-ok="0"`，`dd` 显示 `—`，头像位放 56px 灰底 `SVG(robot)`。
③ **能力区** `aria-disabled="true"` + `filter:grayscale(1);opacity:.45;pointer-events:none`，上方一条：

```html
<p class="yyq-notice" data-tone="info"><span class="yyq-notice-ico">SVG(key)</span>
  还没有连上 QQ。<a href="#" onclick="yyQQOpenAccess();return false">填写 NapCat 地址</a>后能力开关才会生效。</p>
```

**扫码态**：后端目前**没有**二维码接口（`/api/qq/*` 无 qr/login 路由），需补 `GET /api/qq/login/qr` → `{ok,qr_url,expires_in}`。接口就绪前此块只做静态占位：

```html
<div class="yyq-qr" data-state="waiting">
  <div class="yyq-qr-box"><img alt="登录二维码"><span class="yyq-qr-mask">二维码已过期<button>刷新</button></span></div>
  <ol class="yyq-qr-steps"><li class="is-done">手机 QQ 扫码</li><li class="is-on">在手机上确认</li><li>机器人自动上线</li></ol>
</div>
```

### 6.2 加载态

禁止纯文字「能力列表读取中…」（`js:81`）。用与真实行**等高**的骨架：

```css
.yyq-skel{height:60px;border-radius:10px;margin-bottom:8px;background:linear-gradient(90deg,var(--phx-bg-secondary) 25%,color-mix(in srgb,var(--phx-border-light) 60%,var(--phx-bg-secondary)) 37%,var(--phx-bg-secondary) 63%);background-size:400% 100%;animation:yyqShimmer 1.4s infinite}
@keyframes yyqShimmer{from{background-position:100% 0}to{background-position:-100% 0}}
```

能力 6 条、时间线 4 条。数量须 ≥ 首屏可见行数，否则二次跳动。

### 6.3 空态（统一三行组件）

```html
<div class="yyq-empty"><span class="yyq-empty-ico">SVG(chat)</span>
  <p>机器人还没收到群消息</p><small>在群里 @ 它一句，这里会立刻出现</small>
  <button class="btn-s">刷新</button></div>
```
```css
.yyq-empty{border:1px dashed var(--phx-border);border-radius:12px;padding:28px 18px;text-align:center}
.yyq-empty-ico{display:grid;place-items:center;width:40px;height:40px;margin:0 auto 12px;border-radius:50%;background:var(--phx-bg-secondary);color:var(--phx-text-disabled)}
.yyq-empty-ico .yy-icon{width:20px;height:20px}
.yyq-empty p{margin:0;font-size:13px;color:var(--phx-text)}
.yyq-empty small{display:block;margin:6px 0 14px;font-size:11px;color:var(--phx-text-secondary)}
```

空态四处文案（避免"还没有…"连排）：实时流·消息「机器人还没收到群消息 / 在群里 @ 它一句，这里会立刻出现」；实时流·动作「今天还没有执行过动作 / 群里发 `.点赞 123456` 试试」；收藏表情「还没读取表情包 / 读取后能看清机器人收藏了哪些图」；群列表「机器人还没加入任何群 / 先在 QQ 里把它拉进一个群」。

### 6.4 错误态与保存反馈

- **行内**（单接口失败）：`.yyq-inline-err`，红左边框 3px + 图标 + 原因 +「重试」文本按钮，占位在原处，不弹 toast。
- **整页**（`/api/qq/config` 失败）：hero 换成 `.yyq-empty` 的 `data-tone="error"` 变体，状态带保留可用。
- **动作失败**：时间线节点 `data-kind="fail"`，图标换 `ban`，正文显示 `a.error`；完整错误放 `<details>`，折叠行只留前 40 字。
- **保存反馈**：按钮进 `data-busy`（文案→「保存中」+ 12px 转圈），成功后 `data-done`（✓ 已保存，1.4s 复原）。**不只依赖 toast**——toast 2.2s 就消失（`js/utils.js:40`）。

## 7. 移动端（390px）

```css
@media(max-width:900px){.yyq-grid{grid-template-columns:1fr}.yyq-stream{position:static;max-height:340px}.yyq-fold-grid{grid-template-columns:1fr}}
@media(max-width:560px){
  .yyq-status{height:52px;padding:0 12px;gap:8px}.yyq-status-meta{display:none}  /* 只留 点+文案 */
  .yyq-hero{padding:16px;border-radius:16px}.yyq-avatar{width:48px;height:48px;border-radius:14px}
  .yyq-hero-name strong{font-size:17px}
  .yyq-facts{grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.yyq-facts>div{border-left:0;padding-left:0}
  .yyq-chain{flex-direction:column;align-items:flex-start;gap:6px}
  .yyq-cap{grid-template-columns:36px minmax(0,1fr) 44px;min-height:68px;padding:10px 12px}
  .yyq-cap-meta{grid-column:2;font-size:10px;margin-top:2px}   /* 元信息换第二行 */
  .yyq-cap-body small{-webkit-line-clamp:1}
  .yyq-tabs{overflow-x:auto;scrollbar-width:none;margin:0 -16px 12px;padding:0 16px}.yyq-tabs::-webkit-scrollbar{display:none}
  .yyq-fold summary{height:52px}
  .yyq-fold-actions{position:sticky;bottom:0;z-index:2;background:var(--phx-bg-card);padding:10px 0 max(10px,env(safe-area-inset-bottom));border-top:1px solid var(--phx-border-light)}
  .yyq-fold-actions .btn,.yyq-fold-actions .btn-s{flex:1 1 auto;min-height:44px}
}
```

- **触控 ≥44×44**：开关本体 44×26，靠 `label` 的 padding 撑到 44 高。
- **不横向滚动**：能力行固定 3 列，全链路 `min-width:0` + `overflow-wrap:anywhere`。
- **状态带压缩**：390px 只留 `● 已连接`，`NapCat·3 群·12s` 移进 hero 的 facts。
- **sticky 保存条**：折叠块展开可能超两屏，保存按钮必须常驻底部。
- **时间线** `overscroll-behavior:contain`，避免滚到底带动整页。
- `.sp-body` 在 600px 下已 `padding:22px 18px 35px`（`youyuan_design.css:30`），页面根容器**不要再加左右 padding**，否则 390px 可用宽度只剩 318px。

## 8. 反例清单

| # | 位置 | 现在 | 为什么廉价 | 换成 |
|---|---|---|---|---|
| 1 | `js:88` | `<span class="yy-cap-icon">` 塞后端 emoji | emoji 跨系统字形/基线/大小全不一致，带"聊天软件"气质，不像控制台 | `yyIcon(yyQQCapIcon[kind])` 18px 描边 |
| 2 | `css:30` | 能力是 `minmax(230px,1fr)` 卡片网格 | 10 个能力→3~4 列大卡，每卡 2 行字，留白比内容多；扫视成本高 | 60px 行式列表 |
| 3 | `js:121-130` | 接入 7 个控件平铺一张卡 | 无分组无主次，用户不知道必须填哪几个 | 折叠块 + segmented + 2 列字段网格 |
| 4 | `js:133` | 「关掉的能力…会直接拒绝；面板也会同步置灰。」 | 用解释文字弥补设计缺陷，读完仍不知后果 | 删掉；改 `title` + 关闭态行内「已停用」标签 |
| 5 | `js:146` | `<label>试一句</label><input>` 在按钮**之后** | 阅读顺序错乱：按钮在上、输入在下 | 输入+按钮同一行，按钮在右 |
| 6 | `js:300` | 「试一句」实际 `POST /api/qq/send` 真发群 | 语义欺骗，测试消息会真进群 | 沙盒本地渲染 |
| 7 | `js:81` | `能力列表读取中…` 纯文字 | 加载时塌成一行字，随后整块跳变 | 6 条等高骨架 |
| 8 | `js:100` | 表情空态一句话 | 大空白+一句话=未完成感 | `.yyq-empty` 三行组件 |
| 9 | `css:25` | `:nth-child(3/4/5){grid-column:1/-1}` | 依赖 DOM 顺序，插一个 section 全乱 | 显式 `.yyq-span-2` |
| 10 | `css:54-56` | 动作 4 列 grid，`.ok` 只加 `border-left:3px` | 390px 挤成 2 行；左边框是最弱信号 | 时间线 + 图标着色 |
| 11 | `js:176` | `host.outerHTML = yyQQConsoleHero()` | 替换节点丢焦点、跳滚动、重触发 `onerror` | 统一 `yyQQRender(part)` 按 id 更新 |
| 12 | `js:13` | 状态文案漏 `error`/`waiting_callback` | 直接显示英文 `error` | 补全 7 个状态中文 |
| 13 | `css:13` | 三个 `yy-bot-tag` 同权重灰 chip | 分不清「在线」和「OneBot」哪个是状态 | 状态点+文字（带色），其余降普通文本 |
| 14 | `css:4` | `0 20px 44px -34px rgba(15,32,40,.6)` | .6 透明度深色大阴影，暗色主题发脏 | 双层阴影，最大 .28 |
| 15 | `css:43` | 每个能力挂一个虚线框指令码 | 10 个虚线框=噪音，指令不是每行都要看 | 收进详情 / hover 显示 |
| 16 | `js:155` | `yyIntro(..., "…身份、能力开关、表情包与提示词，都在这里管理。")` | 功能罗列等于没说 | 动态状态句「正在运行 · NapCat · 3 个群」 |
| 17 | `css:47-49` | 表情 `height:88px` 固定 | 长图被压扁，宽图两侧留白 | `aspect-ratio:1` + `object-fit:cover` |
| 18 | `js:211` | 选群只把 id 写进输入框 | 看不出"选了哪个群"，也看不到群名 | 群卡网格 + 勾选态 |

## 9. 落地顺序

1. **换图标**（#1）——加 12 个 path + 映射表，`c.icon` 换 `yyIcon(...)`。最小改动，提升最大。
2. **能力改行式**（#2/#15）——重写 `yyQQConsoleCaps()` + CSS。
3. **状态带 + 链路**——新 `yyQQStatusBar()` / `.yyq-chain`，解决 D 场景。
4. **接入/人设折叠**（#3/#5/#6）——`<details>` 包裹，改动量小。
5. **实时流**（新功能）→ 6. 骨架屏 / 空态 / 移动端收尾。

CSS 按 `.yyq-` 前缀全量重写 `youyuan_qq_console.css`（现仅 79 行），完成后删除旧 `.yy-bot-*` / `.yy-cap-*` / `.yy-act-*`，避免两套样式共存。
