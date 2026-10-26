# 种子查询器 / 种子地图 v2 · 现状报告

> 审计对象：`upstream/frontend/js/youyuan_seedmap_v2.js` + `upstream/frontend/css/youyuan_seedmap_v2.css`
> 方式：全文件通读（非抽样）+ Grep 计数。**本次只读，未改任何源码、未部署、未动版本戳。**
> 日期：2026-10-26

---

## 0. 结论速览

1. v2 是**服务端瓦片拼图**方案：前端不算生物群系，只摆瓦片、叠图标、报坐标，全部数据来自 `/api/seed/v2/*`。
2. 设计语言**完全寄生在 `--phx-*` 令牌**上（87 处引用 / 12 个变量），自带变量只有 3 个。改主题不需要动这个文件。
3. 但**地图画布本身是写死的深色**（`--sm2-map-bg: #0b1218` + 一批写死的 `rgba(9,15,21,…)` 浮层），不跟浅色主题走。这是有意的，也是后面做增量的边界。
4. 结构上非常"契约化"：**47 个 id、1 个全局函数、0 个内联 onclick**，交互全部走 `addEventListener` + `data-*` 属性。这对"增量优化"是极大利好——加按钮不用碰既有逻辑。
5. **重大发现：项目里同时存在 3 套种子模块**，而且标题叫「种子查询器」的那个**不是** v2（详见第 2 节）。改哪个必须先跟用户对齐，否则会改错文件。

---

## 1. 文件与规模

| 文件 | 字节 | 行数 | CRLF | 换行 | 末尾换行 |
|---|---:|---:|---:|---|---|
| `js/youyuan_seedmap_v2.js` | **91,467** | **2,403** | **0** | 纯 LF | 有（`})();` + 换行） |
| `css/youyuan_seedmap_v2.css` | **23,954** | **833** | **0** | 纯 LF | 有 |

- 编码：UTF-8（中文注释正常）。
- JS 是一个 `(function(){ 'use strict'; … })()` IIFE，**不污染全局**，只在末尾挂了 `window.yySeedMapPage`。
- CSS 内注释用 `/* ── 区块名 ── */` 分节，共 8 个语义分区：变量 / 顶部工具条 / 地图卡片 / 侧栏 / 移动端 / 抽屉头 / 减少动画。
- 引入方式（`index.html`）：
  - 第 26 行 `<link id="yy-seedmap-v2-css" href="/css/youyuan_seedmap_v2.css?v=2026102824">`
  - 第 179 行 `<script src="/js/youyuan_seedmap_v2.js?v=2026102824"></script>`
  - JS 内 `ensureCss()` 还会兜底注入一次（用 `#yy-seedmap-v2-css` 去重，不会双载），但它写的版本是 `?v=2026102601`，**与 index.html 的 2026102824 不一致**（见第 9 节）。

---

## 2. 三套并存的种子模块（必须先看，决定改哪里）

| # | 文件 | 入口标题 | 入口位置 | 渲染方式 | 数据来源 | 结构数 |
|---|---|---|---|---|---|---|
| A | `js/youyuan_seed_page.js`（1111 行 / 44,011 B）+ `css/youyuan_seed_page.css` | 「种子查询」 | 工具箱模块（`yyModules` id=`seed`，category=`seed`） | `<canvas>` 手绘网格 | 纯本机 `YYSeedLocal` 算法 + 读内核种子 | 8 类（村庄/要塞/神殿/府邸/下界要塞/堡垒/神庙/史莱姆） |
| B | `js/youyuan_seedmap_v2.js`（2403 行 / 91,467 B）+ `css/youyuan_seedmap_v2.css` | 「**种子地图**」 | **劫持 A 的入口**（见下） | 服务端 PNG 瓦片 + DOM 图标层 | `/api/seed/v2/*` | 28 种（manifest 驱动） |
| C | `js/features/seed_map.js`（1227 行 / 53,145 B）+ `css/yy_seed_map.css` | 「**种子查询器**」 | 更多工具 → 建筑与世界（`yyToolsCards`） | `<canvas>` 本机绘群系色块 | `/api/seed/biome`（批量优先）、`/api/seed/structure`、`/api/youyuan/world/seed` | 图钉式，按维度 |

**B 如何顶掉 A**（`youyuan_seedmap_v2.js` 第 2369–2379 行）：

```js
// 旧页面（id='seed'，youyuan_seed_page.js）走的是 /api/seed/* 那套近似实现。
// 玩家不该看到两个「种子」入口，所以：有旧入口就把它的 open 指到新页，
var legacy = null;   // 在 yyModules 里找 id==='seed'
if (legacy) {
  legacy.name = '种子地图';
  legacy.desc = '拖动缩放看地形与结构，点一下读坐标';
  legacy.open = function () { return yySeedMapPage(); };
}
```

所以：**A 已经不可达**（入口被改写，但文件保留当后备），**B 和 C 同时在线上，且都叫"种子××"**。

> ⚠️ 用户口中「种子查询器」的**字面标题是 C**（`js/features/seed_map.js`，`YSM_TITLE = '种子查询器'`）。
> 本次任务指派的是 **B**。两者设计语言不同（C 是 44px 点击区 / 12px 圆角 / 手机优先；B 是 27~38px 控件 / 16px 圆角 / 桌面优先）。**动手前必须确认改的是哪一个**，否则会优化到用户看不见的页面上。

---

## 3. 现有设计语言（增量基准，可照此写同风格样式）

### 3.1 色板

#### (1) 本文件自带的 3 个变量（`.sm2` 作用域内）

| 变量 | 值 | 用途 | 实际使用 |
|---|---|---|---|
| `--sm2-map-bg` | `#0b1218` | 地图画布底色（恒定深色） | 2 处（`.sm2-map` 背景 + 径向渐变终点） |
| `--sm2-map-line` | `rgba(255,255,255,.06)` | 预留的瓦片网格线 | **0 处，死变量** |
| `--sm2-radius` | `16px` | 工具条 / 地图卡外框圆角 | 2 处 |

#### (2) 引用的 `--phx-*` 令牌（12 个 / 87 处）

| 变量 | 引用次数 | 在本文件里承担什么 |
|---|---:|---|
| `--phx-text-secondary` | 20 | 次要文字：副标题、chip 未选、行距离、说明段 |
| `--phx-text` | 16 | 主文字：h3、选中 chip、读数数值 |
| `--phx-border` | 16 | 所有 1px 描边（卡片/输入/分隔/圆点边框） |
| `--phx-primary` | 13 | 主色：查看按钮、维度选中、聚焦环、pin、滑杆、强调行 |
| `--phx-bg-card` | 7 | 卡片底：工具条、地图卡、mini 按钮、侧栏卡、info 面板 |
| `--phx-bg` | 7 | 内凹底：种子输入框、版本框、维度条、chip、汇总胶囊 |
| `--phx-shadow` | 2 | 工具条 / 侧栏卡投影 |
| `--phx-on-primary` | 2 | 主色上的文字（**base.css 未定义，见第 9 节**） |
| `--phx-text-disabled` | 1 | 种子输入 placeholder |
| `--phx-success` | 1 | `.sm2-kv .ok`（预留，JS 未用该类） |
| `--phx-shadow-lg` | 1 | 地图卡投影 |
| `--phx-primary-hover` | 1 | `.sm2-go:hover` |

> 结论：**要新写同风格样式，只准用这 12 个 `--phx-*`**，不要新增颜色字面量。

#### (3) 写死的颜色（不跟主题，全部集中在地图区）

| 值 | 出现在 |
|---|---|
| `#0b1218` / `#16212b` | 地图画布底色 / 径向渐变起点 |
| `#121c24` / `#111a22` | 瓦片占位底 / 瓦片加载失败底 |
| `rgba(9,15,21,.74/.82/.86/.9)` | 读数条 / 操作坞 / 抽屉按钮 / hover 气泡底 |
| `#cfd8e0` `#93a2ad` `#56656f` `#e2e8ef` `#fff` `#e6edf3` `#b9c6cf` `#c4d1da` `#4b6472` `#e8eff4` | 深色浮层上的文字阶梯（主/次/分隔点） |
| `rgba(45,212,191,…)` + `#2dd4bf` | 青色光晕：输入聚焦环、pin 外圈、滑杆轨道/滑块边、spinner |
| `rgba(74,222,128,.2/.45)` | 史莱姆区块填充 / 边框 |
| `rgba(40,18,18,.92)` `rgba(248,113,113,.45)` `#ffd9d9` | 错误提示条 |
| `rgba(148,163,184,.26)` | 种子输入框的 × 清空钮底 |
| `rgba(255,255,255,.028/.05/.08/.1/.12/.16/.22)` | 各种深色浮层的白色叠加 |

> 设计规则一句话：**"卡片跟随主题，地图永远是暗房"**。地图内所有浮层（readout / dock / hover / loading / empty / alert）都用 `rgba(9,15,21,α)` + `rgba(255,255,255,α)` 描边，不引用 `--phx-bg-card`；只有 `.sm2-info` 是例外（用 `var(--phx-bg-card)` + `var(--phx-border)`，所以点击信息面板在浅色主题下是浅色的）。

### 3.2 圆角阶梯（按出现频次）

| 值 | 次数 | 用在哪 |
|---|---:|---|
| `50%` | 14 | 圆形：清空钮、圆点、pin、dock 按钮、图标徽标、关闭钮、spinner |
| `999px` | 8 | 药丸：种子输入框、版本框、维度条、查看按钮、chip、dock 容器、抽屉按钮 |
| `8px` | 3 | mini 按钮、开关、读数条 |
| `var(--sm2-radius)` = `16px` | 2 | 工具条、地图卡外框 |
| `7px` | 2 | 汇总胶囊、alert 内按钮 |
| `6px` | 2 | hover 气泡、列表行 |
| `4px` | 2 | 骨架块、emptyline 内 code |
| `2px` | 2 | 滑杆轨道 |
| `14px` | 2 | 侧栏卡片（`.sm2-card`）、640 断点下的工具条 |
| `12px` | 1 | info 面板 |
| `10px` | 1 | alert 条 |
| `16px 16px 0 0` | 1 | 移动端底部抽屉 |
| `50% 50% 50% 2px` | 1 | 要塞图标的"水滴"形状 |

**规律**：容器 16 / 卡片 14 / 面板 12 / 浮条 10 / 控件 8 / 小件 6~7 / 药丸 999 / 圆点 50%。新加东西照这个阶梯挑，不要出现 5px、9px、11px。

### 3.3 间距

| 场景 | 档位 | 说明 |
|---|---|---|
| 卡片内 gap | `8px`(9 次) `6px`(7 次) `10px`(5 次) | 8 是默认，6 是密集控件组，10 是卡片之间 |
| 页面级 gap | `.sm2` = `14px`（列间距） / `.sm2-main` = `12px` / `.sm2-side` = `10px` | 三档递减 |
| 卡片 padding | `11px 12px`（`.sm2-card`）、`10px 12px`（`.sm2-bar`）、`8px 12px`（info body）、`8px 10px`（info head） | 上下 8~11、左右 12 为主 |
| 小控件 padding | `0 10px`（mini/chip/switch）、`0 12px`（维度钮）、`0 18px`（查看） | 横向随体量递增 |
| 浮层 padding | `5px 10px`（readout）、`5px`（dock）、`7px 9px 7px 12px`（alert） | |
| 地图内边距 | `12px`（readout/dock/drawerbtn/info 距边），640 下退到 `8px` | |
| 行高 | 根 `1.5`；说明段 `1.6` | |

### 3.4 字号（px，共 7 档）

| 值 | 次数 | 语义 | 用在哪 |
|---|---:|---|---|
| `16px` | 1 | 图标字 | `.sm2-dock button` 的 ＋/− |
| `15px` | 3 | 空态标题 / 关闭钮 | `.sm2-empty strong`、info 关闭、抽屉关闭 |
| `14px` | 2 | 卡片标题 | `.sm2-card-head h3`、种子输入 `input` |
| `13.5px` | 2 | **正文基准** | `.sm2` 根、`.sm2-go` |
| `13px` | 10 | 控件文字 | 维度钮、chip 容器、行、info、loading、抽屉按钮 |
| `12.5px` | 18 | **辅助文字主力** | 版本标签、chip、mini、switch、读数、hover、行、tip、alert |
| `12px` | 2 | 最小字 | `.sm2-dots small`、`.sm2-emptyline code` |

**规律**：13.5 正文 → 13 控件 → 12.5 辅助 → 12 极小；标题 14；不要用 11px 以下。

### 3.5 卡片结构

本文件有**两种**卡片，别混：

```
① 侧栏卡 .sm2-card（有 head）
   <div class="sm2-card">
     <div class="sm2-card-head">
       <h3>标题</h3>            ← 14px / 600 / --phx-text
       <small>右侧计数</small>   ← 12.5px / --phx-text-secondary / margin-left:auto / 等宽数字
       <button class="sm2-close-drawer">×</button>   ← 仅移动端显示（display:none → 1023 下 inline-flex）
     </div>
     <div class="sm2-sum">…</div>        ← body，无固定 class，各卡自定
     <p class="sm2-emptyline">…</p>
   </div>
   样式：bg-card / 1px border / radius 14 / padding 11px 12px / shadow

② 地图卡 .sm2-mapcard（无 head，纯容器 + 叠层）
   .sm2-mapcard > .sm2-map > { .sm2-plane, .sm2-slimes, .sm2-icons, .sm2-pin,
                               .sm2-hover, .sm2-readout, .sm2-dock, .sm2-drawerbtn,
                               .sm2-loading, .sm2-empty, .sm2-alert, .sm2-info }
   样式：bg-card / 1px border / radius 16 / shadow-lg / overflow:hidden
   内部高度 clamp(430px, 63vh, 760px)
```

> **没有 footer 概念**。唯一的"foot"是 `.sm2-info-foot`（两个 mini 按钮 flex:1）。

### 3.6 主题机制

- **主题是全局的，本文件零参与**。内置 10 套 + 自定义，全部定义在 `css/base.css`：
  `light`（默认 `:root`）、`dark`、`forest`、`amethyst`、`ocean`、`sakura`、`midnight`、`sepia`、`glass`、`neon`、`cream`，外加 `custom`（内联变量）。
- 切换方式：**`data-theme` 属性写在 `<html>` 上**，不是 class、不是覆盖变量：

```js
// js/theme.js
document.documentElement.setAttribute('data-theme', appTheme);
localStorage.setItem('youyuan_theme', appTheme);   // key = youyuan_theme
```

  另有 `index.html` 第 6~7 行的 head 内联脚本，在首帧前抢先落 `data-theme`，避免闪一下浅色。
- 自定义主题走 `setTheme('custom')` 时不清内联变量（`js/theme.js` 第 16~20 行），即**内联 `--phx-*` 优先级最高**。
- 本文件对主题的唯一反应：`--phx-*` 自动跟着变；地图画布不变。
- 另有 `prefers-reduced-motion: reduce` 一条：把 `.sm2 *` 的 animation/transition 压到 0.001ms。

### 3.7 可直接复用的组件配方（照抄即同风格）

```css
/* 主按钮：查看 */
height:38px; padding:0 18px; border:0; border-radius:999px;
background:var(--phx-primary); color:var(--phx-on-primary);
font-size:13.5px; font-weight:600;

/* 次按钮：mini */
height:27px; padding:0 10px; border:1px solid var(--phx-border);
border-radius:8px; background:var(--phx-bg-card);
color:var(--phx-text-secondary); font-size:12.5px;
/* hover: color:--phx-text; border-color:--phx-primary */

/* 药丸切换组 */
height:38px; padding:3px; gap:2px; border-radius:999px;
background:var(--phx-bg); border:1px solid var(--phx-border);
/* 子按钮 height:100%; padding:0 12px; radius 999px;
   选中: bg=--phx-primary, color=--phx-on-primary */

/* 多选 chip（带色点） */
height:29px; padding:0 10px; gap:6px; border-radius:999px;
border:1px solid var(--phx-border); background:var(--phx-bg);
font-size:12.5px; opacity:.62;   /* 未选 */
/* 选中: opacity:1; border-color:var(--c); background:rgba(255,255,255,.05) */

/* 深色浮层（地图内） */
background:rgba(9,15,21,.82); border:1px solid rgba(255,255,255,.1);
border-radius:8px; backdrop-filter:blur(8px); color:#cfd8e0;
```

**动态色约定**：每个结构自带颜色，通过内联 `style="--c:#xxxxxx"` 注入，CSS 侧一律写 `var(--c, 兜底色)`。新加带色元素必须沿用 `--c` 这个名字（已在 chip / dots / sum / group-head / icon / badge 六处使用）。

---

## 4. 结构

### 4.1 DOM 树（`pageHtml()` 第 454–552 行）

```
#sm2 .sm2                                  ← 根，grid: minmax(0,1fr) 344px
├─ .sm2-main                               ← 左列，flex column, gap 12
│  ├─ .sm2-bar                             ← 顶部工具条（卡片）
│  │  ├─ .sm2-seedrow                      ← flex wrap, gap 8
│  │  │  ├─ .sm2-field#sm2-seedfield       ← 药丸输入框
│  │  │  │  ├─ svg（放大镜）
│  │  │  │  ├─ input#sm2-seed
│  │  │  │  └─ button.sm2-clear#sm2-clear
│  │  │  ├─ label.sm2-ver > span + select#sm2-version
│  │  │  ├─ .sm2-dims#sm2-dims             ← 维度药丸组（JS 填充 button[data-dim]）
│  │  │  └─ button.sm2-go#sm2-go
│  │  └─ .sm2-layers                       ← 虚线分隔 + 可折叠
│  │     ├─ button.sm2-layertoggle#sm2-layertoggle
│  │     │  ├─ b「结构图层」 + span.sm2-dots#sm2-dots + svg.sm2-caret
│  │     └─ .sm2-layerbody#sm2-layerbody
│  │        ├─ .sm2-chips#sm2-chips        ← JS 填充 .sm2-chip[data-id]
│  │        └─ .sm2-layeracts              ← 3 个 button.sm2-mini[data-act] + label.sm2-switch#sm2-slimewrap
│  └─ .sm2-mapcard
│     └─ .sm2-map#sm2-map                  ← height clamp(430px,63vh,760px)
│        ├─ .sm2-plane#sm2-plane           ← 瓦片层（img.sm2-tile + img.sm2-preview）
│        ├─ .sm2-slimes#sm2-slimes         ← 史莱姆方块层
│        ├─ .sm2-icons#sm2-icons           ← 结构图标层（button.sm2-icon > span.sm2-badge > img/svg）
│        ├─ .sm2-pin#sm2-pin
│        ├─ .sm2-hover#sm2-hover [hidden]
│        ├─ .sm2-readout#sm2-readout
│        ├─ .sm2-dock                      ← − ／ input#sm2-zoom[type=range] ／ ＋ ／ ⌂
│        ├─ button.sm2-drawerbtn#sm2-drawer
│        ├─ .sm2-loading#sm2-loading [hidden] > .sm2-spinner + span#sm2-loadingtext + .sm2-skel>i×3
│        ├─ .sm2-empty#sm2-empty [hidden] > svg + strong#sm2-empty-title + p#sm2-empty-desc + button#sm2-empty-btn
│        ├─ .sm2-alert#sm2-alert [hidden] > p#sm2-alert-text + button#sm2-alert-btn
│        └─ .sm2-info#sm2-info [hidden]
│           ├─ .sm2-info-head > b#sm2-info-title + button[data-act=close]
│           ├─ .sm2-info-body#sm2-info-body   （JS 写入 .sm2-kv 行，内含 #sm2-info-biome / #sm2-info-near）
│           └─ .sm2-info-foot > button[data-act=center] + button[data-act=copy]
├─ aside.sm2-side#sm2-side                 ← 右列，sticky，1023 下变底部抽屉
│  ├─ .sm2-card（结构概览）> head(h3 + small#sm2-total + button#sm2-closedrawer) + .sm2-sum#sm2-sum + p#sm2-summary
│  ├─ .sm2-card（结构列表）> head(h3 + small#sm2-listnote) + .sm2-listbox#sm2-list
│  ├─ .sm2-card（要塞）   > head(h3 + small#sm2-shnote) + .sm2-btnrow > button#sm2-shbtn + .sm2-shlist#sm2-shlist
│  └─ .sm2-card（说明）   > .sm2-tip > svg + p
└─ .sm2-scrim#sm2-scrim                    ← 抽屉遮罩，仅 ≤1023 生效
```

### 4.2 元素 id 清单（共 47 个，每个仅出现 1 次）

**根 / 遮罩（2）**
`sm2`、`sm2-scrim`

**顶部工具条（12）**
`sm2-seedfield`、`sm2-seed`、`sm2-clear`、`sm2-version`、`sm2-dims`、`sm2-go`、`sm2-layertoggle`、`sm2-dots`、`sm2-layerbody`、`sm2-chips`、`sm2-slimewrap`、`sm2-slime`

**地图区（23）**
`sm2-map`、`sm2-plane`、`sm2-slimes`、`sm2-icons`、`sm2-pin`、`sm2-hover`、`sm2-readout`、`sm2-zoom`、`sm2-drawer`、`sm2-loading`、`sm2-loadingtext`、`sm2-empty`、`sm2-empty-title`、`sm2-empty-desc`、`sm2-empty-btn`、`sm2-alert`、`sm2-alert-text`、`sm2-alert-btn`、`sm2-info`、`sm2-info-title`、`sm2-info-body`、`sm2-info-biome`、`sm2-info-near`

**侧栏（10）**
`sm2-side`、`sm2-total`、`sm2-closedrawer`、`sm2-sum`、`sm2-summary`、`sm2-listnote`、`sm2-list`、`sm2-shnote`、`sm2-shbtn`、`sm2-shlist`

**通过 `$()` 读取的有 45 个**。剩下 2 个不参与 JS 读写：

- `sm2-side` —— 只用 CSS 类 `.sm2-side` 控制，JS 用 `$('sm2')` 加 `drawer` 类来开关。
- `sm2-info-near` —— 在 `showInfo()` 的 innerHTML 里生成，但**从不被查询或更新**（"最近结构"是渲染时一次算好的，后续不会刷新）。

> **共 47 个 id。**

### 4.3 全局函数与事件契约

**全局函数：只有 1 个。**

```js
window.yySeedMapPage = yySeedMapPage;   // 第 2359 行，唯一出口
```

**内联 `onclick=` / `onchange=` / `oninput=`：0 个。** 全文件无内联事件属性（Grep 结果为 0）。
所以"改名就废界面"的风险在这里**不存在**；真正的耦合是 **`data-*` 属性契约**：

| 属性 | 取值 | 绑定位置 | 作用 |
|---|---|---|---|
| `data-act` | `all` / `none` / `default` | `.sm2-layerbody` 委托 | 结构图层全选 / 全不选 / 本维度默认 |
| `data-act` | `in` / `out` / `home` | `.sm2-dock`（`document.querySelector`） | 放大 / 缩小 / 回原点 |
| `data-act` | `close` / `center` / `copy` | `#sm2-info` 委托 | 关面板 / 居中 / 复制坐标 |
| `data-dim` | `overworld` / `nether` / `end` | `#sm2-dims` 委托 | 切维度 |
| `data-id` | 结构 id | `.sm2-chip` | 切单个结构 |
| `data-jump` | `"bx,bz"` | `#sm2-list` / `#sm2-shlist` 委托 | 跳转坐标 |
| `data-more` | 结构 id | `#sm2-list` | 展开更多（8 → 40） |
| `data-group` | 结构 id | `.sm2-group` | 分组折叠 |
| `data-strong` | `"1"` | `.sm2-icon` | 要塞图标换成水滴形 |
| `data-filled` | `"0"` / `"1"` | `#sm2-seedfield` | 控制 × 清空钮显隐（纯 CSS） |

**动态生成的 class（新增样式时要照顾到）**：
`.sm2-tile`（+`.pixel` `.fail`）、`.sm2-preview`（+`.gone`）、`.sm2-icon`、`.sm2-badge`、`.sm2-slime`、`.sm2-chip`（+`.on`）、`.sm2-group`（+`.closed`）、`.sm2-group-head`、`.sm2-rows`、`.sm2-row`（+`.more`）、`.sm2-kv`、`.sm2-emptyline`、`.sm2-sum > span`、`.on`（维度钮 / 开关 / mini）。

**CSS 类名共 62 个**（`.sm2*` 去重计数），全部以 `sm2-` 为前缀，**没有任何跨文件的 `.sm2` 覆盖**（Grep 全 `css/` 目录，只有本文件出现 `sm2`）。命名风格是"扁平的 `前缀-块-元素`"（如 `sm2-card-head`、`sm2-info-foot`），没有 BEM 双下划线。

### 4.4 外部全局依赖（同样不能改名，改了本模块直接白屏）

| 名称 | 来源 | 用法 | 有无兜底 |
|---|---|---|---|
| `openSubPage(title, html)` | `app.js` | 渲染页面 | **无**（裸调用，第 2329 行） |
| `subPageCache` | `app.js` | `delete subPageCache['种子地图']` 强制重建 | **无**（裸调用） |
| `activeRenderers` | `app.js` | `push(teardown)` 注册销毁 | 有 `typeof` 判断 |
| `T(msg, kind)` | `app.js` toast | 所有提示 | 有（退化为 `console.warn`） |
| `yyRootFetch` | `youyuan_account_boot.js` | 所有请求 | 有（退化 `window.fetch`） |
| `yyCategories` | `app.js` | 补 `seed: '种子与地图'` | 有 |
| `yyModules` / `yyAllModules` / `yyPrefs` / `yySavePrefs` | `app.js` | 注册入口 | 有 |
| `yyDrawTools` / `curTab` / `subPageOpen` | `app.js` | 首页重绘工具网格 | 有 |
| `localStorage` key `yy-seedmap-v2` | 自己 | 记住 seed/edition/version/dim | try/catch |

> **共 1 个全局函数**（`yySeedMapPage`），**0 个内联 onclick**，**10 个 `data-*` 契约属性**。

---

## 5. 功能

### 5.1 能查什么

| 能力 | 说明 |
|---|---|
| 地形图 | 服务端渲染 PNG 瓦片，按生物群系上色，可拖动/缩放 |
| 结构点 | manifest 驱动的 28 种（村庄/前哨/神殿/神庙/女巫小屋/雪屋/沉船/海底神殿/府邸/远古城市/试炼密室/废弃传送门/埋藏宝藏/矿井/下界要塞/堡垒/末地城/末地船/要塞…），分 `exact` / `candidate` 两档精度 |
| 史莱姆区块 | 仅主世界，逐区块绿块 |
| 要塞 | 单独接口，列出全部并算到当前视口中心的距离 |
| 坐标查询 | 点地图任意位置 → 方块坐标 + 区块坐标 + 生物群系 + 最近结构 |
| 版本/维度切换 | Java & 基岩、主世界/下界/末地 |

### 5.2 输入

- **种子**：`parseSeed()`（第 722 行）
  - 去首尾空白；删所有空白、半角逗号、全角逗号；全角减号 `−–—` 转 `-`
  - 校验 `/^[-+]?\d{1,19}$/`，通过后去掉前导 `+`
  - 不合法 → toast「种子要填十进制整数，可以是负数」；空 → 回到"先填一个种子"空态
  - 上限 19 位（超出 `Number` 安全整数范围，但后端按字符串解析，前端只透传）
- **版本**：`<select>` 的 option value 是 `"edition|label"`（如 `java|1.21.4`），按 edition 分 `optgroup`
- **维度**：manifest 给的 `dimensions`，按钮文字用 `d.name`（中文由后端给）
- **结构**：chips 多选；「全选 / 全不选 / 本维度默认」；默认选中规则 `defaultSelection()`：`def===true` → 退而取 `precision==='exact'` → 再退而取前 4 个

### 5.3 输出

- 地图瓦片：256×256 px，`bpp` 自适应 1/2/4/8/16/32（`computeBpp`：让一张瓦片约占 512 屏幕 px）
- 图标：19×19 圆形徽标，图来自 `/api/seed/v2/icon.svg?id=<id>&size=20`；加载失败退化为内联 `<svg>` 圆点
- 侧栏：结构概览（按类型计数胶囊 + 总数）、结构列表（按到视口中心距离排序、分组折叠、每组先 8 条再展开到 40）、要塞列表
- 读数条：种子 / 中心方块坐标 / 区块坐标 / 每像素多少方块
- 坐标详情：方块坐标、区块坐标、生物群系（带群系自带颜色）、最近结构 + 距离

### 5.4 交互

- 拖动平移（Pointer Events + `setPointerCapture`）
- 滚轮缩放（**不需要按 Ctrl**，`preventDefault` + `{passive:false}`）
- 双指缩放 + 双指平移
- 双击放大 1.9×
- 键盘：`←→↑↓` 平移、`+`/`=` 放大、`-`/`_` 缩小、`0` 回原点
- 缩放滑块：对数映射，`MIN_SCALE=0.04` ~ `MAX_SCALE=32`，滑块 0~100
- 悬停读数（仅鼠标，`pointerType==='mouse'`）

### 5.5 接口（全部 `GET`，前缀 `const API = '/api/seed/v2'`）

| 端点 | 调用点 | 参数 | 用途 |
|---|---|---|---|
| `/manifest` | `bootManifest()` | — | 版本表 / 维度 / 结构清单 / 引擎自检 |
| `/meta` | `readMeta()` | `version, edition` | 该版本不支持的结构 → chip 置灰禁用 |
| `/tile.png` | `tileURLOf()` / `loadPreview()` | `seed,version,edition,dim,large,cx,cz,w,h,bpp,layer=terrain` | 地形瓦片；**必须带 `layer=terrain`**，否则后端把图标烘进 PNG 会和 DOM 徽标叠成双图标 |
| `/structures` | `loadStructures()` | `seed,version,edition,dim,types,cx,cz,r,limit=20000` | 结构点（区块坐标） |
| `/stronghold` | `loadStrongholds()` | `seed,version,edition` | 要塞列表 |
| `/slime` | `loadSlimes()` | `seed,cx,cz,r` | 史莱姆区块 |
| `/biome` | `loadBiome()` | `seed,version,edition,dim,x,z`（区块坐标） | 单点生物群系 |
| `/icon.svg` | `renderIcons()` | `id,size=20` | 结构图标（不是 JSON，直接塞进 `<img src>`） |

- 所有 JSON 请求走 `getJSON()`，用 `AbortController`；`res.ok===false` / `data.ok===false` / 非 JSON 都转成中文 Error。
- 文档里还有 `/summary` 端点，**前端未使用**。
- `YY_SEEDMAP_MOCK` 开关（第 25 行，当前 `false`）可让全部请求走内置假数据 `MOCK`。

### 5.6 性能机制（改样式时别踩）

- 瓦片 LRU 缓存 60 张（`TILE_CACHE_MAX`），DOM 里同时最多 64 个 `img`（`MAX_TILE_ELEMS`）
- 图标上限按缩放分 4 档：<0.09 → 70 个且只画 exact；<0.22 → 170 且只 exact；<0.6 → 280；其余 420（`MAX_ICONS`）
- 史莱姆上限 900 块（`MAX_SLIMES`）
- 平移/缩放用 `requestAnimationFrame` 合并（`schedulePaint`）；重排用 `later()` 防抖（缩放 170ms、平移 90ms、数据 480ms）
- 首屏先拉一张 `bpp=16 / 512px` 粗图垫底，细瓦片到齐后 260ms 淡出
- 瓦片请求并发上限 4（`pumpTiles`）

---

## 6. 与 v1（`youyuan_seed_page.js`）对比

### 6.1 v1 有、v2 没有的

| 能力 | v1 做法 | v2 现状 |
|---|---|---|
| **「读取当前世界种子」按钮** | `#yy-seed-read`，读本机内核里的世界种子，并把 `seedFromKernel` 状态回写到界面 | **完全没有**。v2 只能手输，或从 `localStorage['yy-seedmap-v2']` 恢复上次输入 |
| **基岩版"按 Java 算法试算"** | 有 `preview` 开关 + 横幅说明"这是 Java 试算，不是真实位置" + 地图角标提示 | 没有。v2 的基岩版直接调后端 `edition=bedrock`，不做试算也不做"仅供参考"提示 |
| **版本支持范围写在选项里** | `<option>` 文本带后缀「（算法未实现）」「（仅史莱姆区块）」「（不支持）」 | 没有。v2 靠 `/meta` 的 `unsupported` 事后置灰 chip |
| **「精度与边界（必读）」折叠说明** | `<details>` 逐条列出哪些是精确、哪些是近似、哪些没做 | 只有侧栏一句 `.sm2-tip`（区块×16+8 的换算说明），**没有精度分级说明** |
| **能见度提示（`.yy-seed-banner`）** | 顶部横幅解释当前状态 | 无（只有 error 用的 `.sm2-alert`） |
| **离线可用** | 纯本机算法，断网也能算 | 依赖本机 HTTP 服务，服务没起就只有空态 |

### 6.2 v2 新增的能力

- 真实地形渲染（生物群系上色瓦片），v1 只有网格
- 结构从 8 类扩到 28 类，且有 `exact` / `candidate` 精度标记
- 生物群系查询（点哪读哪）
- 要塞独立列表 + 距离
- 结构列表按距离排序 / 分组折叠 / 展开更多
- 地图交互完整化：滚轮缩放、双指、双击、键盘、缩放滑块、悬停读数
- 移动端底部抽屉 + 遮罩
- 坐标详情面板（含"居中到此处""复制坐标"）
- 状态记忆（seed/edition/version/dim 存 localStorage）
- 引擎未就绪（503）时的降级：地图画不出来，但结构/要塞/史莱姆照常可用

### 6.3 与第三套（`features/seed_map.js`，标题就是「种子查询器」）的差异

| 维度 | v2（种子地图） | 第三套（种子查询器） |
|---|---|---|
| 入口 | 工具箱模块 `seed`（劫持 v1） | 更多工具 → 建筑与世界 |
| 地形来源 | 后端 PNG 瓦片 | 前端按格取 `/api/seed/biome` 批量点，Canvas 上色 |
| 精度表达 | 图标 + `precision` 字段 | 图钉 + `precision` 徽章 + 图例 |
| 图例 | **无** | 有（群系图例 + 结构图例） |
| 移动端 | 断点 1023 变抽屉，控件 27~38px | 手机优先，**点击区 ≥44px**，320px 不横向溢出，15px 输入字号防 iOS 缩放 |
| 圆角 | 16 / 14 / 12 / 10 / 8 | 12 / 11 / 10 / 9 / 6 |
| 字号 | 13.5 基准 | 15 输入 / 13 按钮 / 11.5 辅助 |
| 读当前种子 | 无 | 有（`#ysm-read`） |

> 一句话：**v2 是"桌面优先的地图查看器"，第三套是"手机优先的查询器"**。用户说"在他现有设计基础上优化"，如果指手机体验，第三套的设计约束（44px / 320px）反而更接近用户诉求。

---

## 7. 移动端适配现状

### 7.1 断点（CSS 3 条 + JS 2 处）

| 位置 | 条件 | 变化 |
|---|---|---|
| `youyuan_seedmap_v2.css:749` | `max-width: 1180px` | 侧栏 344px → **306px** |
| `youyuan_seedmap_v2.css:753` | `max-width: 1023px` | 单列；侧栏变**固定底部抽屉**（`translateY(103%)`，`max-height:74vh`，`border-radius:16px 16px 0 0`，`padding-bottom: calc(12px + env(safe-area-inset-bottom))`）；`.sm2-scrim` 显示（z-index 60，抽屉 61）；`.sm2-drawerbtn` 显示；`.sm2-dock` 从居中改左对齐；地图高度 `clamp(400px,62vh,640px)`；`.sm2-listbox` 34vh；`.sm2-close-drawer` 显示 |
| `youyuan_seedmap_v2.css:791` | `max-width: 640px` | 工具条 padding 9/10、圆角 14；种子框 `flex:1 1 100%` 独占一行；版本框 `flex:1 1 auto` + select `max-width:120px`；维度钮 `flex:1` + `padding:0 6px` + 12.5px；地图 `height:56vh; min-height:360px`；读数条退到 8px 且**隐藏 `.sm2-hide-sm`（区块坐标那段）**；dock 改 `left/right:8px` + 滑杆 `flex:1`；drawerbtn 提到 `bottom:58px`；info 面板 `left/right:8px; width:auto` |
| `youyuan_seedmap_v2.css:828` | `prefers-reduced-motion: reduce` | 动画/过渡压到 0.001ms |
| `youyuan_seedmap_v2.js:2341` | `window.innerWidth <= 640` | 打开页面时**自动收起结构图层**（`aria-expanded=false` + `layerbody.hidden=true`） |
| `youyuan_seedmap_v2.js:2293` | `S.size.w > 1023`（ResizeObserver） | 视口变宽时自动关抽屉 |

抽屉开关是**类切换**：`#sm2` 上加/去 `drawer` 类，CSS 用 `.sm2.drawer .sm2-side { transform:none }` 和 `.sm2.drawer .sm2-scrim { opacity:1; pointer-events:auto }`。

### 7.2 390px 宽下的溢出风险（**以下均为按 CSS 推断，未实测**）

| # | 位置 | 推断 | 依据 |
|---|---|---|---|
| 1 | `.sm2-seedrow` 第二行（版本 + 维度） | **不溢出但会挤**：版本框最小宽 ≈ 12(padding-left) + 34(「版本」12.5px×2字) + 4(gap) + 120(select) + 8(padding-right) ≈ **178px**；维度组 3 钮 ×(中文 3 字≈40 + 12 padding) + 2×2 gap + 6 padding ≈ **148px**；178+148+8(gap) = **334px**，容器可用 390−2×10(bar padding) = **370px** → 余 36px，**勉强放下**。若系统字体放大到 1.15×，大概率换行。 |
| 2 | 同上，**320px** 宽 | 334 > 300 → **换行成两行**，不横向溢出（`flex-wrap:wrap` 已设），但工具条会变高约 46px。 |
| 3 | `.sm2-readout` | **安全**：`max-width:calc(100% - 24px)`，第三段已隐藏，19 位种子 + 两组 7 位坐标 ≈ 40 字符 ×6.6px ≈ 264px + padding 16 ≈ 280px < 374px。 |
| 4 | `.sm2-dock` | **安全**：`left/right:8px` + 滑杆 `flex:1`，4 个按钮 + 滑杆最小 0 仍有余量。 |
| 5 | `.sm2-info` | **安全**：640 下 `left/right:8px; width:auto; max-width:none`。 |
| 6 | **手机横屏（844×390）** | **会溢出**：`.sm2-map` 在 640 断点是 `height:56vh; min-height:360px` → 56vh=218，被 min-height 顶到 **360px**；加上工具条（≥110px）和页面内边距，总高 > 390px 视口 → 必须纵向滚动，且 `.sm2-dock` 固定在 map 底部（`bottom:8px`），滚动时会被工具条遮住。**推断，未实测。** |
| 7 | 触摸目标尺寸 | **普遍偏小**：`.sm2-mini` 27px、`.sm2-chip` 29px、`.sm2-dock button` 30px、`.sm2-info-head button` 22px、`.sm2-clear` 20px —— 均低于移动端 44px 建议值。这是与第三套（硬性 44px）最明显的差距。**推断，未实测。** |
| 8 | `.sm2-chips` 展开 | 640 下图层默认收起，用户点开后有 `max-height:132px` + `overflow-y:auto`，**不撑高页面**。安全。 |
| 9 | `.sm2-side` 抽屉内容 | `max-height:74vh` + `overflow:auto`；内部 `.sm2-listbox` 34vh，4 张卡片总高可能超 74vh → **抽屉内滚动**，不会溢出页面。可接受。 |

---

## 8. 优化机会清单（只做**增量追加**，不动原有规则）

> 原则：**只新增选择器 / 只新增媒体查询块 / 只在末尾追加声明**。不重写既有规则，不改 id，不改 `data-*` 取值，不动 JS 里的状态机 `S`。

| # | 改什么 | 为什么 | 风险 |
|---|---|---|---|
| **1** | 在 CSS 末尾追加一个 `@media (max-width:640px)` 块，把 `.sm2-mini`、`.sm2-chip`、`.sm2-dock button` 的 `height` 提到 `40px`，`.sm2-clear`、`.sm2-info-head button` 提到 `32px`，并给 `.sm2-chips` 在同一块内补 `max-height:168px` | 触摸目标只有 20~30px，手机上误触率高；这是用户最能直接感知的"变好用了"。第三套已用 44px 验证过方向 | **低**。纯追加媒体查询，桌面端零影响。副作用：chips 变高 → 必须同步追加 `max-height`，否则折叠区被压。 |
| **2** | 追加 `@media (max-height:480px) and (orientation:landscape)` 块：`.sm2-map { height: calc(100vh - 132px); min-height:220px; }`，`.sm2-dock { bottom:6px; }` | 横屏时 `min-height:360px` 会顶破视口（7.2 #6），地图底部控件被遮 | **低**。纯追加，竖屏不受影响。`100vh` 在部分 WebView 含地址栏可能差 ~56px，只影响横屏观感。 |
| **3** | 追加 `:focus-visible` 样式：`.sm2-chip:focus-visible, .sm2-mini:focus-visible, .sm2-row:focus-visible, .sm2-group-head:focus-visible, .sm2-dock button:focus-visible { outline:2px solid var(--phx-primary); outline-offset:2px; }` | 目前只有 `.sm2-map` 和 `.sm2-icon` 有焦点环，键盘/无障碍用户看不到焦点在哪 | **0**。只新增伪类选择器，不影响鼠标用户。 |
| **4** | 追加 `[data-theme="light"] .sm2-mapcard { box-shadow: 0 6px 18px rgba(29,55,58,.10); }` 与 `[data-theme="light"] .sm2-bar { box-shadow: 0 2px 6px rgba(29,55,58,.06); }` | 浅色主题下 `--phx-shadow` 很淡，深色地图卡贴在浅底上边界不清，看着"浮不起来" | **低**。属性选择器 + 只覆盖 shadow，不改布局。色值取自 `youyuan_design.css` 浅色阴影基色，保持同一套视觉。 |
| **5** | 在 1023 媒体块内追加 `.sm2-side::before { content:''; display:block; width:36px; height:4px; margin:0 auto 8px; border-radius:999px; background:var(--phx-border); }` 并给 `.sm2-side` 补 `position:relative` | 底部抽屉没有任何"可拖拽/可下滑"的视觉暗示，用户不知道能关 | **低**。伪元素不参与文档流；`position:relative` 对 `position:fixed` 的抽屉无副作用。 |
| **6** | 在 640 媒体块内追加 `.sm2-readout { flex-wrap: wrap; }` | 19 位种子 + 大坐标时读数条单行可能撑满，换行更稳 | **低**。`max-width` 已有，换行不会溢出。 |
| **7** | 追加 `@media (max-width:640px) { .sm2-ver select { max-width:100%; } }` | 当前 `max-width:120px` 会把长版本号截断 | **低**。只影响移动端下拉宽度。 |
| **8** | 追加浅色主题下的画布底色覆盖：`[data-theme="light"] .sm2 { --sm2-map-bg:#e8eef2; }`（**仅当**用户希望地图也变浅时） | 现在地图恒定深色，浅色主题下这块深色很突兀 | **中**。会让 `rgba(9,15,21,.74)` 的读数条、hover 气泡在浅底上对比度反转，**必须同步追加**这些浮层的浅色覆盖，否则文字看不清。建议先跟用户确认。 |
| **9** | 把已定义未使用的 `--sm2-map-line` 用起来（如 `.sm2-tile { outline:1px solid var(--sm2-map-line); }`），或原样留着 | 消除死变量；瓦片拼接处加极淡描边可掩盖 1px 缝隙 | **0 / 可选**。`outline` 不占布局。 |
| **10** | 在侧栏第 4 张卡（`.sm2-tip` 之后）追加一张新的 `.sm2-card`，写「精度说明」 | v1 有「精度与边界（必读）」，v2 丢了。用户看到 `candidate` 结构会误以为是精确位置 | **低**（纯 HTML 追加，无 JS）。若要做成按版本动态显示，就需要 JS 增量，风险升到中。 |

**明确不建议的**（属于推倒重做，不在本次范围）：换掉瓦片方案、把地图改成 Canvas、重构状态机 `S`、改 `data-*` 契约、合并三套种子模块。

---

## 9. 存疑点 / 需要在真机上确认的

1. **`--phx-on-primary` 在 `base.css` 里没有定义**（Grep `base.css` 命中 0 次）。它只出现在 `youyuan_design.css`（`:root` 与 `[data-theme="dark"]`）和 `mcort_theme.css`。而 `.sm2-go`、`.sm2-dims button.on` 都依赖它当文字色。**如果某个主题没覆盖这个变量，主色按钮上的文字会继承 `--phx-text`**（浅色下是深褐 `#233338` 压在主色 `#087f79` 上），对比度偏低。推断，未实测。
2. **主题级联顺序存疑**：`base.css` 定义了 10 套主题（`[data-theme="..."]`，特异度 0,1,0），但 `index.html` 第 14 行的 `youyuan_design.css` 用 `:root,[data-theme="light"]` 又重定义了整套 `--phx-*`（同样 0,1,0，但**位置更靠后**）。按 CSS 规则后者会胜出 → 除 `light` / `dark` 外的 8 套主题是否真的生效，需要用实际切主题截图确认。**这与本模块无关，但会直接影响"种子地图在森林/霓虹主题下长什么样"。**
3. **CSS 版本戳不一致**：`index.html` 用 `?v=2026102824`，`ensureCss()` 内部用 `?v=2026102601`。目前因为有 `#yy-seedmap-v2-css` 去重不会双载；但如果哪天从 HTML 里删掉那个 `<link>`，就会走 2601 那个旧戳，缓存行为不可预期。
4. **`sm2-info-near` 是死 id**：`showInfo()` 写入后从不更新，结构数据晚到时"最近结构"不会刷新。
5. **`--sm2-map-line` 是死变量**（定义 1 处、使用 0 处）。
6. **v1 脚本可能被加载两次**：`index.html:178` 已引入 `youyuan_seed_page.js`，而 `youyuan_navigation.js:181-201` 又会按需注入一份（它检查的 id 是 `yy-seed-boot`，HTML 里没有这个 id）。好在 `youyuan_seed_page.js` 第 1090 行有 `!yyModules.some(id==='seed')` 幂等保护，重复执行不会重复注册入口。
7. 第 7 节所有溢出判断均为**按 CSS 推算，未在 390px 真机 / DevTools 实测**。

---

*本报告仅审计，未修改 `youyuan_seedmap_v2.js` / `.css` 或任何源码。*
