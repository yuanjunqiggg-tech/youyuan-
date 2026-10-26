# 种子查询器 UI 现状审计（seed_map.js + yy_seed_map.css）

> 审计对象：`upstream/frontend/js/features/seed_map.js`（文件头自称「种子查询器 — 群系地图 · 结构定位 · 全维度（Canvas 本机渲染）」，`YSM_TITLE = '种子查询器'`）
> 配套样式：`upstream/frontend/css/yy_seed_map.css`（由 `index.html:28` 引入。**已确认**：seed_map.js 内部不加载任何 css，全文件只有 `RseedMapHTML()` 返回内联 HTML、无 `<link>`；`index.html` 全库仅此一处引用 `yy_seed_map.css`）
> 审计方式：全量通读 1227 行 JS + 295 行 CSS，正则统计 + 交叉核对。**未改动任何源码，未部署，未实测渲染**（尺寸结论均标注「CSS 推断」）
> 审计时间：2026-10-26

---

## 0. 文件指纹

| 文件 | 字节数 | 行数 | LF | CRLF | BOM |
|---|---|---|---|---|---|
| `js/features/seed_map.js` | **53,145** | **1,227**（以单个 `\n` 结尾） | 1,227 | **0** | 无 |
| `css/yy_seed_map.css` | **7,672** | **295**（以单个 `\n` 结尾） | 295 | **0** | 无 |

统计口径（Python 读 bytes 计数）：
```python
b = open(path,'rb').read()
len(b)            # 字节数
b.count(b'\n')    # LF
b.count(b'\r\n')  # CRLF
```

**关键结论：两个文件都是纯 LF、无 BOM、无 CRLF。** 后续「在此基础上优化」的追加写入必须保持 LF + 无 BOM，否则会污染整文件 diff。

入口链（已核对）：
- `js/features/others.js:15-16` — 工具卡片，`cat:'建筑与世界'`，`act` 里 `delete subPageCache['种子查询器']; openSubPage('种子查询器', RseedMapHTML()); setTimeout(function(){try{ysmInit()}catch(e){}},30)`
- `index.html:109` 加载 `seed_map.js`；`index.html:120` 加载 `app.js`（**在 seed_map.js 之后**，故 seed_map.js 末尾用 `setTimeout(...,0)` 等 `subPageInit`）
- 页面容器由 `js/app.js:110 openSubPage()` 生成：`#sub-page.sub-page.show > .sp-header + .sp-body`，业务 HTML 塞进 `.sp-body`

---

## 1. 结构契约（以下任何一项都不能改）

### 1.1 元素 id 清单（13 个，HTML 侧与 JS 侧集合完全相等）

统计方式：`grep -o 'id="[^"]*"'` 去重 = 13；`grep -o "ysm\$('[^']*')"` 去重 = 13；两侧**完全相等**，无孤立 id、无缺失 id。

| # | id | 标签/类 | 行号(HTML) | 读取处（行号） | 作用 |
|---|---|---|---|---|---|
| 1 | `ysm-seed` | `input.ysm-input` | 221 | 1015、1107、1189 | 种子输入框，`inputmode="numeric"` |
| 2 | `ysm-read` | `button.btn-s.ysm-readbtn` | 222 | 1040 | 「读当前种子」按钮 |
| 3 | `ysm-version` | `select.ysm-select` | 225 | 1007、1156、1170、1173 | 版本下拉 |
| 4 | `ysm-verbadge` | `span.ysm-badge` | 226 | 1157、1174 | 版本来源徽章（「版本区间矩阵」/「默认」） |
| 5 | `ysm-dims` | `div.ysm-tabs` | 227 | 994 | 维度 tab 组（`role="group"`） |
| 6 | `ysm-note` | `p.ysm-hint` | 233 | 1031、1093、1095、1100、1117、1120、1195 | 状态提示文案 |
| 7 | `ysm-wrap` | `div.ysm-mapwrap` | 236 | 862、984、1047 | 地图容器（ResizeObserver 观测目标） |
| 8 | `ysm-canvas` | `canvas` | 237 | 622、648、861、951-982 | 唯一渲染面 |
| 9 | `ysm-overlay` | `div.ysm-overlay` | 238 | 825、832 | 诚实空态叠层 |
| 10 | `ysm-pincard` | `div.ysm-pincard` | 244 | 838、882 | 图钉详情卡 |
| 11 | `ysm-readout` | `div.ysm-readout` | 246 | 805 | 底部读数条 |
| 12 | `ysm-biolegend` | `div.ysm-legend` | 250 | 765、1080 | 群系图例 |
| 13 | `ysm-strlegend` | `div.ysm-legend.ysm-slegend` | 251 | 783 | 结构图例 |

**共 13 个 id**（命令：`grep -o 'id="[^"]*"' seed_map.js | sort -u | wc -l`）。

### 1.2 全局函数清单（顶层 50 个 + 内嵌闭包 7 个 = 57 个函数定义）

统计方式：`grep -o 'function [A-Za-z0-9_$]*(' seed_map.js`（去重 57）。**共 50 个顶层全局函数**，全部挂在 `window` 上（无 IIFE 包裹）。其中只有 `RseedMapHTML` 与 `ysmInit` 被外部（others.js）调用。

| 分组 | 函数（行号） | 说明 |
|---|---|---|
| 工具 | `ysmEsc`(184) `ysmClamp`(189) `ysmToast`(190) `ysm$`(191) `ysmErrText`(192) `ysmPrecLabel`(195) | `ysm$` = `getElementById` 包装；`ysmToast` 转发全局 `T()`（`js/utils.js:31`） |
| 取色 | `ysmBiomeColor`(205) | 先精确表、再关键词表，未命中返回 `null`（不猜色） |
| 骨架 | **`RseedMapHTML`**(216) | 外部调用（others.js:16），返回整页 HTML 字符串 |
| 状态 | `ysmNewState`(257) `ysmSyncCell`(1085) | 每次打开重建状态对象 |
| 步长 | `ysmPickCell`(281) | 按 `YSM_CELLS` 选每格 18–40px 的步长 |
| 队列 | `ysmPump`(289) `ysmRunJob`(299) `ysmEnqueue`(317) `ysmFlushQueue`(322) | 并发上限 2，优先级小者先出队 |
| URL | `ysmCommonParams`(330) | 拼 `seed/version/edition=bedrock` |
| 群系 | `ysmEnsureBiome`(337) `ysmEnqueueBatch`(421) `ysmEnqueuePoints`(446) `ysmAbsorbPoints`(460) `ysmAbsorbPoint`(471) | 三级探测 batch → point → dead |
| 结构 | `ysmEnsureStructs`(489) `ysmFetchStruct`(514) `ysmStructRowsToBlocks`(541) `ysmStructFallbackAll`(553) `ysmStructFallback`(564) `ysmChunkXY`(585) `ysmAbsorbStructs`(598) | 新接口优先，整桶失败逐类型回退 |
| 几何 | `ysmCellRange`(620) `ysmBlockToPx`(635) `ysmBlockToPy`(636) `ysmPxToBlockX`(637) `ysmPxToBlockZ`(638) | 后两个**从未被调用**（见 §6） |
| 绘制 | `ysmSchedule`(641) `ysmDraw`(645) `ysmDrawPin`(745) | rAF 合并重绘 |
| DOM 更新 | `ysmDrawLegend`(763) `ysmRenderStrLegend`(781) `ysmUpdateReadout`(803) `ysmShowOverlay`(824) `ysmHideOverlay`(831) | 图例 / 读数 / 叠层 |
| 卡片 | `ysmShowPinCard`(837) | 图钉详情，含 `left/top` 内联定位 |
| 绑定 | `ysmBind`(859) | 一次性绑定全部交互 |
| 生命周期 | `ysmOnVis`(1054) `ysmResetData`(1067) **`ysmInit`**(1182) `ysmReinit`(1218) | `ysmInit` 被 others.js:16 与 `ysmReinit` 调用 |
| 种子/版本 | `ysmReadWorldSeed`(1092) `ysmParseManifest`(1129) `ysmLoadManifest`(1155) | 预填种子 + manifest 解析 |
| 内嵌闭包（外部不可引用） | `staleFn`(388) `ysmSeedSig`(389) `localPos`(866) `onDown`(870) `onMove`(884) `onUp`(908) `zoomAt`(935) | 7 个 |

### 1.3 `YSM_` 前缀标识符（9 个）

统计方式：`grep -o '\bYSM_[A-Za-z0-9_]*' seed_map.js | sort -u` → 9 个。

| 标识符 | 行号 | 类型 | 内容规模 |
|---|---|---|---|
| `YSM_TITLE` | 34 | 字符串常量 | `'种子查询器'`（也被 `subPageInit` 用，见 1226） |
| `YSM_BIOME_EXACT` | 41 | 对象 | **23** 条 `[颜色, 中文名]` |
| `YSM_BIOME_MATCH` | 67 | 数组 | **53** 条 `[关键词, 颜色, 中文名]` |
| `YSM_STRUCT_META` | 129 | 对象 | **21** 条结构元数据（`label/color/dim`，2 条带 `prob`） |
| `YSM_STRUCT_BY_DIM` | 158 | 对象 | 从 META 派生：`overworld` 15 / `nether` 3 / `the_end` 1（概率型不入组） |
| `YSM_STRUCT_PROB` | 159 | 数组 | 2 条（`mineshaft` 0.4%、`buried_treasure` 1%） |
| `YSM_DIM_LABEL` | 167 | 对象 | 主世界 / 下界 / 末地 |
| `YSM_DEFAULT_VERSIONS` | 170 | 数组 | 7 个版本 `1.21 … 1.12` |
| `YSM_CELLS` | 280 | 数组 | 9 档 `4 … 1024` |

**共 9 个 `YSM_` 标识符**（命令：`grep -o '\bYSM_[A-Za-z0-9_]*' seed_map.js | sort -u | wc -l`）。

### 1.4 `data-*` 属性（2 个）

统计方式：`grep -o 'data-[a-z-]*' seed_map.js | sort -u` → 2 个。

| 属性 | 取值 | 出现位置 | JS 读取处 |
|---|---|---|---|
| `data-dim` | `overworld` / `nether` / `the_end` | 228、229、230（3 个 button） | 997、998、1000（`closest('button[data-dim]')`） |
| `data-act` | `in` / `out` / `reset` | 240、241、242（3 个 button） | 984（选择器）、986、987、988 |

### 1.5 `RseedMapHTML()` 返回的骨架层级（216-254）

```
div.ysm                                    <- 根，flex column, gap 12
├─ section.ysm-panel                       <- 参数面板
│  ├─ div.ysm-row
│  │  ├─ input#ysm-seed.ysm-input
│  │  └─ button#ysm-read.btn-s.ysm-readbtn
│  ├─ div.ysm-row.ysm-row-ver              <- flex-wrap
│  │  ├─ select#ysm-version.ysm-select
│  │  ├─ span#ysm-verbadge.ysm-badge[hidden]
│  │  └─ div.ysm-tabs#ysm-dims[role=group]
│  │     └─ button[data-dim] x3（第 1 个带 .on）
│  └─ p.ysm-hint#ysm-note
├─ section.ysm-mapcard
│  ├─ div.ysm-mapwrap#ysm-wrap             <- position:relative, touch-action:none
│  │  ├─ canvas#ysm-canvas
│  │  ├─ div.ysm-overlay#ysm-overlay[hidden]
│  │  ├─ div.ysm-tools
│  │  │  └─ button[data-act] x3
│  │  └─ div.ysm-pincard#ysm-pincard[hidden]
│  └─ div.ysm-readout#ysm-readout
└─ section.ysm-panel                       <- 图例面板（复用 .ysm-panel）
   ├─ h3.ysm-legend-title
   ├─ div.ysm-legend#ysm-biolegend（初始含 span.ysm-dim）
   └─ div.ysm-legend.ysm-slegend#ysm-strlegend（初始为空）
```

注意：图例区**复用** `.ysm-panel`（没有独立类），任何对 `.ysm-panel` 的改动会同时影响参数面板与图例面板 —— 这是「不动原有规则」时最容易被忽略的耦合点。

### 1.6 外部全局依赖（改名/删除会连带炸）

| 符号 | 定义处 | 本模块用法 |
|---|---|---|
| `T(msg, kind)` | `js/utils.js:31` | `ysmToast` 转发 |
| `openSubPage` / `subPageCache` | `js/app.js:110` / `js/app.js:11` | 由 others.js 调用，非本模块 |
| `subPageInit` | `js/app.js:17` | 1224-1227 注册 `ysmReinit` |
| `activeRenderers` | `js/app.js:3` | 1202-1214 push 清理函数 |
| `--phx-*` CSS 变量 | 见 §2.2 | 全部视觉令牌 |

---

## 2. 现有设计语言（「在它基础上优化」的基准）

### 2.1 CSS 类清单（26 个 `ysm-*` 类，**0 个未被使用**）

统计方式：`grep -o '\.[a-zA-Z][a-zA-Z0-9_-]*' yy_seed_map.css | sort -u` → 26 个 `ysm-*` 唯一类名，另有 `.on`（状态类）与 `.js`（注释中文件名误命中）。

| # | 类名 | 行号 | 职责 | 关键声明 |
|---|---|---|---|---|
| 1 | `.ysm` | 9 | 根容器 | `flex column; gap:12px; padding:2px 0 24px` |
| 2 | `.ysm-panel` | 12 | 卡片面板（参数 + 图例共用） | `padding:12px 13px; border:1px --phx-border; radius:12px; bg:--phx-bg-card` |
| 3 | `.ysm-row` | 21 | 横向行 | `flex; align-items:center; gap:8px; min-width:0` |
| 4 | `.ysm-row-ver` | 22 | 版本行修饰 | `flex-wrap:wrap` |
| 5 | `.ysm-input` | 24 | 种子输入 | `min-height:44px; padding:10px 12px; radius:10px; font-size:15px` |
| 6 | `.ysm-select` | 24/43 | 版本下拉 | 同 input + `flex:0 1 132px` + 双 linear-gradient 三角箭头 |
| 7 | `.ysm-readbtn` | 52 | 读种子按钮修饰 | `flex:0 0 auto; min-height:44px`（外观继承 `.btn-s`） |
| 8 | `.ysm-badge` | 55 | 版本徽章 | `padding:2px 7px; radius:6px; bg:rgba(245,166,35,.14); 10.5px/650` |
| 9 | `.ysm-tabs` | 67 | 维度 tab 容器 | `flex; gap:6px; flex:1 1 100%` |
| 10 | `.ysm-hint` | 95 | 提示段落 | `11.5px/1.5; --phx-text-secondary` |
| 11 | `.ysm-dim` | 101 | 弱化文字（多处复用） | `11.5px; --phx-text-secondary` |
| 12 | `.ysm-mapcard` | 104 | 地图卡外壳 | `flex column; gap:8px` |
| 13 | `.ysm-mapwrap` | 109 | 地图视口 | `position:relative; height:clamp(320px,60vh,520px); radius:12px; overflow:hidden; touch-action:none; user-select:none` |
| 14 | `.ysm-mapwrap canvas` | 123 | 画布 | `display:block; width/height:100%; cursor:grab` |
| 15 | `.ysm-overlay` | 133 | 顶部叠层 | `absolute; left/right/top:8px; display:flex; justify-content:center; pointer-events:none` |
| 16 | `.ysm-overlay-box` | 142 | 叠层气泡 | `padding:7px 11px; radius:9px; bg:rgba(18,22,25,.82); color:#f2f5f7; 11.5px; blur(6px)` |
| 17 | `.ysm-tools` | 158 | 右侧浮动工具 | `absolute; right:8px; bottom:8px; flex column; gap:6px` |
| 18 | `.ysm-tools button` | 166 | 缩放/复位按钮 | `44x44; radius:10px; bg:rgba(18,22,25,.78); color:#f2f5f7` |
| 19 | `.ysm-pincard` | 187 | 图钉详情卡 | `absolute; left:8; bottom:8; right:64; max-width:300; radius:11px; shadow 0 8px 22px rgba(0,0,0,.22)` |
| 20 | `.ysm-pincard-head` | 203 | 卡片标题行 | `flex; gap:7px; 13px/650` |
| 21 | `.ysm-pincard-x` | 218 | 卡片关闭按钮 | `28x28; radius:7px; 透明底` |
| 22 | `.ysm-pincard-row` | 233 | 卡片信息行 | `--phx-text-secondary`；`b` 用 `--phx-text`/650 |
| 23 | `.ysm-readout` | 237 | 底部读数条 | `padding:7px 10px; radius:9px; 11.5px; tabular-nums; overflow-wrap:anywhere` |
| 24 | `.ysm-legend-title` | 250 | 图例小标题 | `12px/650; --phx-text` |
| 25 | `.ysm-legend` | 256 | 图例容器 | `flex wrap; gap:5px 10px` |
| 26 | `.ysm-lgi` | 261 | 图例单项 | `inline-flex; gap:5px; 11.5px; white-space:nowrap` |
| 27 | `.ysm-lgi i` | 270 | 图例色块 | `11x11; radius:3px; border:1px solid rgba(0,0,0,.12)` |
| 28 | `.ysm-slegend` | 277 | 结构图例修饰 | `margin-top:2px` |
| 29 | `.ysm-pin-dot` | 278 | 结构图例圆点 | `border-radius:50% !important; border:none !important` |

（上表 29 行含派生选择器；`ysm-*` 唯一类名共 26 个：ysm / panel / row / row-ver / input / select / readbtn / badge / tabs / hint / dim / mapcard / mapwrap / overlay / overlay-box / tools / pincard / pincard-head / pincard-x / pincard-row / readout / legend-title / legend / lgi / slegend / pin-dot。）

**死类检查结果：0 个未使用。** 每个 `ysm-*` 类都能在 `seed_map.js` 的 HTML 字符串或 `innerHTML` 拼装中找到使用点（`ysm-pin-dot`→788、`ysm-slegend`→251、`ysm-row-ver`→224、`ysm-badge`→226 等）。

### 2.2 色板：**全部走 `--phx-*` 变量 + 少量硬编码**（这是能跟随 MCORT 主题的关键）

**变量实际生效值**（级联已核对：`base.css:5 :root` 被 `youyuan_design.css:2 :root,[data-theme="light"]` 覆盖，后者在 `index.html:14` 晚于 `base.css`(10) 加载；挂载 MCORT 主题时 `html[data-mcort-theme]`（`mcort_theme.css:59-75`）再覆盖一次）。

| 变量 | 用在本模块哪里 | 浅色生效值 | 深色生效值 | 本模块写的 fallback |
|---|---|---|---|---|
| `--phx-primary` | 输入聚焦边框、tab hover/选中、tools hover、focus-visible | `#087f79` | `#68d8c5` | **无** |
| `--phx-primary-bg` | tab 选中底色 | `#e8f6f3` | `#203c3a` | `rgba(20,126,130,.12)` |
| `--phx-border` | panel/input/mapwrap/tools/pincard/readout 边框、pincard-x hover 底 | `#dae3e5` | `#33494b` | **无** |
| `--phx-border-light` | `.btn-s` 边框；**canvas 网格线（JS 663-665 读）** | `#e7ecee` | `#2a4042` | `#e2e8ea` |
| `--phx-bg-card` | panel/mapwrap/readout/pincard 底 | `#fff` | `#1b3033` | **无** |
| `--phx-bg` | input/select 底；**canvas 底色（JS 664）** | `#f5f7f8` | `#142427` | `#f5f7f8` |
| `--phx-text` | input 值、legend-title、pincard 正文与 `b` | `#233338` | `#e2ecec` | **无** |
| `--phx-text-secondary` | hint/dim/readout/lgi/pincard-row/tab 未选中/select 箭头/placeholder | `#5f6f73` | `#93a7aa` | `#8a949c` |
| `--phx-warning` | 版本徽章文字 | `#f5a623`（回落 base.css） | `#fbbf24`（回落 base.css dark） | **无** |

**写死的颜色（不跟随主题，全部列出）**：

| 值 | 位置 | 用途 | 判定 |
|---|---|---|---|
| `rgba(245,166,35,.14)` | css:59 | 徽章底色 | 与 `--phx-warning` 手配的 14% 透明版，主题改 warning 时底色不跟 |
| `rgba(18,22,25,.82)` / `#f2f5f7` | css:147-148 | overlay 气泡（永远深色） | 有意：浮在地图上保对比 |
| `rgba(18,22,25,.78)` / `#f2f5f7` / `#7fd7d9` | css:172-182 | 浮动工具按钮 + hover 青 | 有意，但 `#7fd7d9` 是唯一「独立于主题的强调色」 |
| `rgba(0,0,0,.12)` | css:275 | 图例色块描边 | 浅色下有效，深色下几乎不可见 |
| `rgba(0,0,0,.22)` | css:198 | pincard 阴影 | 深色下无意义 |
| `#8a949c55` | js:692 | 有数据但无识别色时的格子填充 | 中性灰 33% |
| `#ffffff` | js:746、756 | 图钉描边与白心 | 有意的「白描边」画法 |
| `#f5f7f8` / `#e2e8ea` / `#6b7a83` | js:664-666 | canvas 取变量的兜底 | 与 CSS fallback 不一致（CSS 用 `#8a949c`） |

**群系色板（写死在 JS，不在 CSS）**：23 条精确 + 53 条关键词。代表色：平原 `#91bd59`、沙漠 `#e0c860`、丛林 `#2f8f1f`、深海 `#12306e`、雪原 `#f0f6f6`、恶地 `#d97718`、蘑菇岛 `#a945a9`、深暗之域 `#1b2430`、下界荒地 `#7a2f22`、末地 `#e5e08a`，未识别灰 `#8a949c`。
**结构图钉色（19 种可画点）**：村庄 `#2f9e5f`、要塞 `#4a8fb8`、远古城市 `#2b3f5c`、试炼密室 `#c07a3a`、末地城 `#d8c747` 等，见 131-154。

### 2.3 圆角档位（7 档，全部写死，未用 `--phx-radius`）

| 值 | 用在哪 |
|---|---|
| `12px` | `.ysm-panel`(18)、`.ysm-mapwrap`(115) |
| `11px` | `.ysm-pincard`(196) |
| `10px` | `.ysm-input`/`.ysm-select`(32)、`.ysm-tabs button`(79)、`.ysm-tools button`(171) |
| `9px` | `.ysm-overlay-box`(146)、`.ysm-readout`(239) |
| `7px` | `.ysm-pincard-x`(225) |
| `6px` | `.ysm-badge`(58) |
| `3px` / `50%` | `.ysm-lgi i`(274) / `.ysm-pincard-head i`(215)、`.ysm-pin-dot`(279) |

**`--phx-radius`(14px) 与 `--phx-radius-sm`(9px) 完全没被使用** —— 这是与 MCORT 主题体系唯一的「脱钩点」。

### 2.4 间距档位

- 纵向节奏：根 `gap:12px`；面板内 `gap:9px`；地图卡 `gap:8px`；行内 `gap:8px`；tab 与工具按钮 `gap:6px`；图例 `gap:5px 10px`。
- 内边距：面板 `12px 13px`；输入 `10px 12px`；tab `8px 4px`；overlay `7px 11px`；pincard `10px 12px`；readout `7px 10px`；badge `2px 7px`；根 `2px 0 24px`。
- 叠层偏移统一 `8px`（overlay 三边、tools 右下、pincard 左/下）；`right:64px` 约等于 8 + 44 + 12 的视觉留白。

### 2.5 字号 / 字重档位

| 字号 | 用途 |
|---|---|
| `15px` | 输入框、下拉（**≥15px 防 iOS 聚焦缩放，注释明写**） |
| `13px` | tab 文字、pincard 标题 |
| `12px` | 图例小标题、pincard 正文 |
| `11.5px` | hint / dim / overlay / readout / lgi（出现 5 次，是正文基准） |
| `11px` | `.ysm-tools button[data-act="reset"]` |
| `10.5px` | 版本徽章 |

字重：`650`（badge、legend-title、pincard-head、`b`）、`600`（tab、tools 按钮）、`700`（继承全局 `button`）。

### 2.6 Canvas 尺寸策略（CSS 定高 + JS 按 DPR 重设位图，**不是固定 px、也不是纯百分比**）

1. CSS：`.ysm-mapwrap{height:clamp(320px,60vh,520px)}`，`canvas{width:100%;height:100%}` → 高度跟视口，宽度跟容器（`.sp-body` 内容宽）。
2. JS(654-657)：每次 `ysmDraw` 比较 `cv.width !== round(W*dpr)`，不一致才重设位图尺寸；`dpr = min(2, devicePixelRatio)`（上限 2，避免高分屏内存爆）。
3. 变换：`g.setTransform(dpr,0,0,dpr,0,0)` 后全部按 CSS px 作图。
4. 重绘触发：`ResizeObserver` 观测 `#ysm-wrap`（1045-1048，无 RO 时回落 `window.resize`）→ `ysmSchedule()`；`requestAnimationFrame` 合并。
5. 三档高度：`<360px` → `clamp(280px,52vh,420px)`；默认 → `clamp(320px,60vh,520px)`；`≥900px` → `clamp(420px,62vh,640px)`。

### 2.7 交互态（写新样式必须对齐这四态）

- hover：tab 变 `--phx-primary` 边框与字色；tools 边框 primary + 字色 `#7fd7d9`；pincard-x 底 `--phx-border`。
- `:focus-visible`：tab 与 tools 用 `outline:2px solid var(--phx-primary); outline-offset:2px`。**input/select 只有 `:focus` 改边框色，没有 outline**。
- `:active`：tools 按钮 `transform:scale(.95)`（与全局 `button:active` 同值）；canvas `cursor:grabbing`。
- `:disabled`：无自定义，继承全局 `button:disabled{opacity:.5;pointer-events:none}`（读种子按钮请求中会 disabled，1094/1122）。
- 过渡：仅 tab 有 `transition: background .15s, color .15s, border-color .15s`；tools 按钮无 transition。

---

## 3. 功能面

### 3.1 能查什么

| 能力 | 实现 | 精度声明 |
|---|---|---|
| 群系地图 | 按 `cell` 步长网格取点，命中缓存上色，未命中画 1px 空网格 | 三种模式 `batch`/`point`/`dead`，读数条如实显示「批量模式 / 单点模式 / 群系接口不可用」 |
| 结构定位 | 19 种可画点结构（主世界 15 / 下界 3 / 末地 1） | `exact`/`approx`/`candidate` 经 `ysmPrecLabel` 归一为「精确 / 近似（试算）/ 不支持 / 原样」 |
| 概率型结构说明 | `mineshaft` 0.4%、`buried_treasure` 1% 只写进图例、不画点 | 注释明确「不假装扫过了但一个都没有」 |
| 三维度 | `overworld`/`nether`/`the_end`，切换即 `ysmResetData()` 清缓存 | — |
| 版本切换 | `manifest` 解析，失败用 7 个内置版本 + 「默认」徽章 | — |
| 当前种子预填 | 打开即 `ysmReadWorldSeed(null)`，优先 `seed_text`（防 2^53 丢精度），`0` 会提示「别当成真实种子」 | — |

**没有**的功能：史莱姆区块、要塞环定位（只有 `stronghold` 结构点）、矿洞分布、群系过渡带、坐标跳转输入框、图钉列表/导出。优化时不要假设这些已存在。

### 3.2 输入

- 种子：`input` + `inputmode="numeric"`，仅接受 `/^[+-]?\d{1,20}$/`，`BigInt` 校验 64 位范围（1023-1026）。`Enter` 触发 `blur` + apply。
- 版本：`<select>`，值来自 manifest 或默认列表。
- 维度：3 个 button（`data-dim`）。
- 手势：单指拖动平移、双指捏合缩放（`clamp(0.01, 8)`）、滚轮缩放（±25%）、三个按钮（x1.6 / ÷1.6 / 复位到 0,0）。
- 缩放与拖动停止后 **260ms** 防抖再补拉（916、944、976）。

### 3.3 后端接口（4 条路径，6 种调用形态）

| 方法/路径 | 参数 | 调用处 | 失败策略 |
|---|---|---|---|
| `GET /api/seed/biome` | `seed&version&edition=bedrock&dimension&lod=1&from_x&from_z&to_x&to_z` | 371（探测）、437（8x8 格分片） | 失败 → 试单点 → 再失败 2 次进 `dead` |
| `GET /api/seed/biome` | `...&x&z` | 395（探测）、450（逐格） | 单点模式一次最多 160 格 |
| `GET /api/seed/manifest` | 无 | 1162 | 失败 → 内置 7 版本 + 「默认」徽章 |
| `GET /api/seed/structure` | `...&dimension&type=all&from_x&from_z&to_x&to_z`（**区块坐标**，`/16`） | 519 | 整桶失败 → 逐类型回退 |
| `GET /api/youyuan/world/seed` | 无 | 1097（预填种子） | 提示手输 |
| `GET /api/youyuan/world/seed` | `seed&structure=<type>&edition=java&from_x...` | 566（结构回退试算） | 静默，图例注明 |

配额纪律（注释 26-31 与实现一致）：同屏并发 2（291）、优先级 0=群系 / 1=结构（293）、单屏格数 >320 直接放弃取点（344）、缓存 >4000 清空（485）、`visibilitychange` 隐藏时 `ysmFlushQueue()` 掐断（1058）、缓存键 = `cell:gx,gz`（349）。

### 3.4 Canvas 上画了什么（`ysmDraw` 645-743，绘制顺序）

1. 整屏填 `--phx-bg`（667-668）。
2. 群系格子：有 `color` → 填色块（`ceil(cellPx)` 取整避免缝）；有数据无颜色 → `#8a949c55`；无数据 → `--phx-border-light` 1px 描边空网格（684-701）。
3. 原点十字轴：x=0 / z=0 两条 0.5px 对齐线（703-711）。
4. 结构图钉：水滴形 `moveTo(x,y) → lineTo(x+4,y-7) → arc(x,y-13,6.5,...) → fill + stroke(白 1.5px) → 白心 r2.2`（745-760）。视口外 ±30px 的图钉被 filter 掉（714-718）。
5. 取样中提示：底部居中 `11px system-ui`，文案 `群系取样中…（并发 2 · 已缓存 N 格）`（723-729）。
6. 同时更新 DOM：`ysmDrawLegend()` + `ysmUpdateReadout()`（734-735）—— **每帧都重写 innerHTML**，见 §6。

---

## 4. 移动端适配现状

### 4.1 断点（**只有 2 个**，全部在本文件）

| 断点 | 行号 | 改了什么 |
|---|---|---|
| `max-width: 359px` | 284-290 | panel 内边距 12/13→10/11；select 独占整行 `flex:1 1 100%`；tab 字 13→12；地图高 `clamp(280px,52vh,420px)`；pincard `right:8px; max-width:none` |
| `min-width: 900px` | 293-295 | 地图高 `clamp(420px,62vh,640px)` |

**360–899px 区间没有任何专门适配**，390px 完全走默认样式。父级 `.sp-body` 内边距来自 `youyuan_design.css:30`（≤600px）：`22px 18px 35px` → **390px 视口下内容宽 = 390 − 36 = 354px**；面板内宽 = 354 − 26 = **328px**。

### 4.2 触摸事件支持

| 事件 | 是否支持 | 位置 |
|---|---|---|
| `pointerdown/move/up/cancel` | 支持，主路径（含 `setPointerCapture`） | 951-954 |
| 双指捏合 | 支持，通过 `pointers` 表 + `Math.hypot` 自行实现 | 876-900 |
| `touchstart/move/end` | **仅当 `!window.PointerEvent` 时注册，且只做单指平移、没有捏合** | 961-982 |
| `wheel` | 支持缩放（`passive:false` + preventDefault） | 955-959 |
| 双击缩放 | 无 | — |
| 长按 | 无 | — |
| `touch-action` | `none`（mapwrap + canvas），阻止浏览器抢手势 | css:119、128 |

### 4.3 可点元素实际尺寸（CSS 推断，未实测）

| 元素 | 尺寸 | 是否 ≥44px |
|---|---|---|
| `#ysm-seed` input | 高 `min-height:44px` | 是 |
| `#ysm-read` 按钮 | 高 44；宽约 5x14px + 2x18px padding + 4px border ≈ **110px**（继承全局 `button{padding:11px 18px;font-size:14px}`） | 是 |
| `#ysm-version` select | 高 44；宽 `flex:0 1 132px`（390px 下与 input 同行时被压到约 132） | 是 |
| 维度 tab x3 | 高 44；宽 `(328 − 2x6)/3 ≈ 105px` | 是 |
| 浮动工具 x3 | **44x44** 固定 | 是 |
| 图钉关闭 `x` | **28x28** | **否，不达标** |
| Canvas 上的图钉 | 命中半径 **26px**（927 行 `bestD=26`）→ 直径 52px 判定圈 | 是（但判定圆心是锚点，不是钉头） |

### 4.4 390px 下可能溢出/拥挤的元素

1. **`#ysm-pincard` 详情卡（最严重）**：CSS 同时给了 `left:8; bottom:8; right:64`（187-202），JS 又内联写 `left/top`（852-853）。按 CSS 规范，`left`+`right` 同时非 auto 且 `width:auto` → **宽度 = 容器宽 − left − 64**；`top`+`bottom` 同时非 auto 且 `height:auto` → **高度 = 容器高 − top − 8**。JS 的 clamp 上限是 `容器宽 − 190`（850 行），故 390px 下卡片宽度最窄可到 **354 − 164 − 64 = 126px**；若图钉靠上，卡片高度会被拉到接近整屏（约 `520 − 6 − 8 ≈ 500px`）却只有 3 行字。**结论：卡片宽高被双重约束压扁/拉长，390px 下可读性会明显崩坏（CSS 推断，未实测，建议优先复现验证）。**
2. `.ysm-lgi` 用 `white-space:nowrap`，最长项「掠夺者前哨站」6 字约 69px + 11px 色块 + 5px gap ≈ 85px，一行约放 3 个，不溢出但换行密集。
3. `.ysm-tools` 3x44 + 2x6 = **144px 高**，在 320px 高地图上占 45%，会压住右下角群系格子。
4. `.ysm-select` 的背景箭头用 `background-position: ... 20px` 固定 20px，若主题改小 `min-height`，箭头会偏。
5. `.ysm-readout` 有 `overflow-wrap:anywhere` + `tabular-nums`，长文案（「precision 未标注（后端没给，不冒充精确）」）能换行，不溢出。
6. 首屏总高（390x844 推断）：header 62 + padding 22 + 参数面板约 150 + gap 12 + 地图 506 + gap 8 + readout 约 34 + gap 12 + 图例面板约 110 + padding 35 ≈ **950px > 844**，**必然需要滚动**，地图无法一屏看全。

### 4.5 其它移动端事实

- 图钉命中判定用锚点 `(px, py)`，而视觉钉头画在 `py − 13`；半径 26 能覆盖，但点在钉头正上方 26px 外会失效。
- 轻点判定：`moved < 8 && Date.now()-downAt < 500ms`（924）→ 长按超过 500ms 不弹卡片。
- `clamp(320px,60vh,520px)` 用 `vh`：移动端键盘弹出时 `vh` 不变（iOS），地图不会被压扁，但也意味着输入框聚焦后地图仍在视口下方。
- 无 `env(safe-area-inset-*)` 处理（父级 `.sp-body` 在 `youyuan_agent_mobile.css:810` 已有底部安全区，本模块根 padding 24px 足够）。

---

## 5. 可增量优化的点（全部为「追加规则」，不改 id / 不改函数名 / 不重写原有规则）

> 纪律说明：以下每一条都只是在 `yy_seed_map.css` **末尾追加新规则**（同选择器靠后者覆盖），原有规则文本一字不动；JS 一行不改。风险等级 = 「改坏了会不会让页面不可用」。

| # | 改什么（追加内容） | 为什么 | 风险 | 纯追加 |
|---|---|---|---|---|
| A1 | `.ysm-pincard{bottom:auto}` | 解掉 §4.4-1 的 `top`+`bottom` 双重约束，让卡片按内容高度收缩；JS 内联的 `top` 继续生效，视觉位置不变 | **中**（若某次 JS 未写 `top` 会贴顶，但当前 852 行必写） | 是 |
| A2 | `.ysm-pincard{max-height:calc(100% - 16px);overflow:auto}` | 内容多（长结构名 + 试算说明）时不被裁掉 | 低 | 是 |
| A3 | `.ysm-pincard-x{width:44px;height:44px;display:inline-flex;align-items:center;justify-content:center}` | 把 28x28 补到 44x44 触控标准（§4.3 唯一不达标项） | 低 | 是 |
| A4 | `.ysm-slegend .ysm-dim{flex:1 1 100%}` | 「按区块概率生成、地图无法给点：废弃矿井 0.4%、埋藏的宝藏 1.0%」在 390px 下与图例项挤同一行，独占一行后清晰 | 低 | 是 |
| A5 | `#ysm-overlay[hidden]{display:none}` | 修 §6.2-1 的 `[hidden]` 被 `display:flex` 覆盖（现靠 innerHTML 清空掩盖） | 低 | 是 |
| A6 | `.ysm-tabs button.on{box-shadow:inset 0 0 0 1px var(--phx-primary)}` | 浅色下 `--phx-primary-bg #e8f6f3` 与 `--phx-bg-card #fff` 对比过弱，选中态靠边框单薄 | 低 | 是 |
| A7 | `.ysm-input:focus,.ysm-select:focus{box-shadow:0 0 0 3px var(--phx-primary-bg)}` | 与 A6 同一套「柔光环」语言，补足输入聚焦反馈（现只有 1px 边框变色） | 低 | 是 |
| A8 | `@media (pointer:coarse){.ysm-tools button{width:48px;height:48px}}` | 触屏上浮动按钮 44 偏紧，48 更好按（桌面鼠标不受影响） | 低 | 是 |
| A9 | `@media (min-width:360px) and (max-width:899px){.ysm-mapwrap{height:clamp(300px,54vh,480px)}}` | 390px 首屏要滚约 950px 才能看完，降 6vh 能少滚一屏 | **中**（地图变矮，用户可能觉得「图变小了」） | 是 |
| A10 | `.ysm-overlay-box{max-width:min(100%,360px)}` | 叠层文案（含后端 error 原文）在桌面 1090px 宽下会拉成一整条 | 低 | 是 |
| A11 | `.ysm-lgi i{box-shadow:0 0 0 1px rgba(127,127,127,.28)}` | 深色主题下 `rgba(0,0,0,.12)` 描边几乎看不见，图例色块与背景糊在一起 | 低 | 是 |
| A12 | `.ysm-legend:empty::after{content:'—';color:var(--phx-text-secondary,#8a949c);font-size:11.5px}` | `#ysm-strlegend` 在结构未返回前是空 div，视觉上是一块空白 | 低 | 是 |
| A13 | `@media (max-width:359px){.ysm-pincard{max-height:calc(100% - 16px)}}` | 320px 下卡片更矮，配 A2 一起用 | 低 | 是 |
| A14 | `.ysm-readbtn{font-size:13px;padding-left:12px;padding-right:12px}` | 390px 下「读当前种子」按钮约 110px 宽，占掉首行 1/3，收窄可给种子输入多留约 14px | 低 | 是 |

**明确不做（会被判「推倒重来」）**：换框架、把 canvas 换成 SVG/DOM、重命名 `ysm-*` 前缀、改 `RseedMapHTML` 结构、合并三个 section、把群系色板搬到 CSS 变量。

**建议落地顺序**：A1 + A3（修真实布局缺陷与触控不达标）→ A4 + A5（低成本正确性）→ A6 + A7（视觉一致性）→ 其余按需。
