# 缺口与不符处（gaps）

> 生成：2026-10-26 15:55 · 由主 Agent 直接核实（12 Agent 批次中 #1/#2/#4/#12 被 429 打断，本文件为兜底产出）
> 纪律：**每条都带 `文件:行` 或可复现命令**。没验证的标「未验证」。

---

## A. 已证伪的说法（文档/报告写错了）

### A-1 审计报告说 CSS 行尾「CRLF 均为 0」——**错**
- 报告原文：`outputs/kb/seed-v2.md` 第 1 节「js 91,467 B / 2,403 行、css 23,954 B / 833 行，CRLF 均为 0」
- 实测：
  ```
  js/youyuan_seedmap_v2.js  91467 bytes  CRLF=0    LF-only=2403
  css/youyuan_seedmap_v2.css 23954 bytes CRLF=833  LF-only=0
  ```
- 结论：**js 是 LF，css 是 CRLF**，两者不一致。字节数与行数都对得上，说明报告作者只测了 js 就下了「均为」的结论。
- 影响：如果按「都是 LF」去做增量追加，会把 css 变成混行尾（LR 混 CRLF），diff 会变成整文件重写。
- 本次处理：追加时**按 css 现有 CRLF 写入**，追加后 `CRLF 833→928`、`LF-only=0`，行尾保持纯净单一。

### A-2 作战程序把「种子查询器」的目标文件写成了 v2——**先错后对，已澄清**
- 初判（我自己的错）：以为「种子查询器」逐字对应 `js/features/seed_map.js`（该文件 `YSM_TITLE = '种子查询器'`，入口 `js/features/others.js:16`）。
- 实证：`第28条-种子查询器-作战地图.md:57` 明确写 `seed-ui` agent 独占 `frontend/js/youyuan_seedmap_v2.js` / `frontend/css/youyuan_seedmap_v2.css`，且 UI 参照物是 `D:\种子查询器自研\web\index.html`（"用户要跟那个网址一样"）。
- 结论：**v2 才是用户要改的那个**。`features/seed_map.js` 是另一套独立模块，标题撞名。

### A-3 项目里有 3 套种子模块，玩家能看到 2 个入口
| 文件 | 标题 | 入口 | 状态 |
|---|---|---|---|
| `js/youyuan_seed_page.js`（id=`seed`） | 种子查询（旧） | 旧模块位 | 已被 v2 劫持（`youyuan_seedmap_v2.js:2375-2379` 把 `open` 指向 `yySeedMapPage()`），玩家看不到 |
| `js/youyuan_seedmap_v2.js`（91,467 B） | **种子地图** | 接管 id=`seed` 位置 | 活跃 |
| `js/features/seed_map.js`（53,145 B，YSM_*） | **种子查询器** | 更多工具 → 建筑与世界 | 活跃、独立 |
- 缺口：两个活跃入口标题都带「种子」，玩家容易混。**是否隐藏其中一个，需要用户定。**

---

## B. 真实存在但未修的缺陷

### B-1 `--phx-on-primary` 在 `base.css` 里未定义
- 现象：`youyuan_seedmap_v2.css` 2 处用它当主色按钮文字色（`.sm2-go`、`.sm2-dims button.on`）。
- 依据：`grep -c "phx-on-primary" css/base.css` → 0；该变量只在 `youyuan_design.css` 与 `mcort_theme.css` 定义。
- 风险：某主题未覆盖时，主色按钮文字会继承 `--phx-text`（浅色下 `#233338` 压在 `#087f79` 上，对比度偏低）。
- 状态：**未修**（属全局主题层，改动影响面大，需用户确认是否动 `base.css`）。

### B-2 同一个 CSS 有两个版本戳
- `index.html:26` → `youyuan_seedmap_v2.css?v=2026102826`（本次已升）
- `youyuan_seedmap_v2.js` 内部 `ensureCss()` → `?v=2026102601`
- 现状：因为有 `#yy-seedmap-v2-css` 去重，不会双载；但若哪天从 HTML 删掉那个 `<link>`，就会回落到旧戳，缓存行为不可预期。
- 状态：**未修**（属 js 改动，本次范围外）。

### B-3 `sm2-info-near` 是死 id
- `showInfo()` 写入后从不更新 → 结构数据晚到时「最近结构」不刷新。
- 状态：**未修**。

### B-4 `--sm2-map-line` 是死变量
- 定义 1 处（`youyuan_seedmap_v2.css:9`），使用 0 处。
- 状态：**未修**（无害，可选清理）。

---

## C. 12 Agent 批次的实际完成情况

| # | 角色 | 结果 | 证据 |
|---|---|---|---|
| 1 | 源码地图 | 完成（原判「未产出」有误，见下方勘误） | `outputs/kb/source-map.md` 52,557 B / 466 行 |
| 2 | API 清单 | 完成（原判「未产出」有误，见下方勘误） | `outputs/kb/api-inventory.md` 60,926 B / 570 行 |
| 3 | 前端模块图 | 完成 | `outputs/kb/frontend-map.md` 27,110 B |
| 4 | 插件运行时 | 完成（原判「未产出」有误，见下方勘误） | `outputs/kb/plugin-runtime.md` 39,883 B / 443 行 |
| 5 | MCP 面 | 完成 | `outputs/kb/mcp-surface.md` 68,614 B |
| 6 | 种子查询器 | 完成 | `outputs/kb/seed-v2.md` 38,809 B |
| 7 | 对话归档 | 完成 | `outputs/kb/conversations.md` 44,014 B |
| 8 | 用户原话索引 | 完成 | `outputs/kb/user-words-index.md` 32,178 B |
| 9 | 经验库提炼 | **429 中断** | `experience-log.md` 未追加新条目 |
| 10 | RAG 建库 | **由主 Agent 完成** | `index.jsonl` 11,643 块 / `youyuan.db` 16.5MB |
| 11 | 三语言文档 | **口径待确认**（见 D-1） | 未产出 |
| 12 | 验证挑刺 | **由主 Agent 完成** | 本文件 |

> 说明：429 是**额度上限**（重置时间 2026-10-27 14:21），不是代码问题。按 playbook §4.5.2，已落盘的直接采用、未落盘的降级由主 Agent 兜底。

> **勘误（2026-10-26 18:20，由 verifier 独立验收发现）**
> 本表原先断言 #1 源码地图、#2 API 清单、#4 插件运行时「429 中断，未产出」，**这是错的**。
> 三份文件实际都存在且内容完整：
> - `source-map.md` 52,557 B / 466 行，首行「源码版本 0.10.78，`upstream/youyuan_product.go:14`」
> - `api-inventory.md` 60,926 B / 570 行，口径写明「逐条解析 `mux.HandleFunc(...)` 注册语句」
> - `plugin-runtime.md` 39,883 B / 443 行，基线写明「仓库 HEAD `3a11d92`（2026-10-21）」
>
> 它们的 mtime 是 15:33~15:35，与 `gaps.md`（15:54）同属提交 `8cad63a`。
> **根因**：写本表时只看「Agent 报了 429」这个终态，没有回头 `ls` 目标文件——「报错」≠「没产出」。
> 这正是本项目最高频的事故模式（把「我以为的」当成「实际是的」），记入 `experience-log.md`。

---

## D. 口径分歧（必须用户拍板，不许自己猜）

### D-1 「插件的开发文档，三种语言都要」指的是哪种「三种语言」？
- **读法一（编程语言）**：Lua / Python / C++。→ **三份文档已经存在且同构**：
  - `Lua插件开发说明.md` 179,367 B
  - `Python插件开发说明.md` 189,000 B
  - `C++插件开发说明.md` 188,983 B
  - 三者都是 11 个 API 章节（game / player / world / events / packet / bot / chains / building / media / plugin / util），行号一一对应，差异只在开头「入口约定」那段。
  - 若按这个读法，任务不是"写新文档"，而是**拿源码核对这三份有没有过期**。
- **读法二（自然语言）**：中文 / English / 日本語。→ 需要新写三份，工作量大，且现有项目没有 i18n 传统。
- **当前倾向**：读法一（项目本来就是三宿主插件系统，且文档已存在）。
- **待用户确认。**

### D-2 三套种子模块要不要合并/隐藏？
- 见 A-3。合并属于「推倒重做」，明确不在本次范围。隐藏其中一个只需改导航注册，风险低。
- **待用户确认隐藏哪个。**

### D-3 浅色主题下地图要不要变浅？
- 现在「卡片跟随主题，地图永远是暗房」（地图恒定 `#0b1218`）。浅色主题下这块深色很突兀。
- 但一旦改浅，`rgba(9,15,21,.74)` 的读数条、hover 气泡全部要反向重做，否则文字看不清。
- **待用户确认。** 本次**没动**（这是唯一一条我主动放弃的增量）。

---

## E. 本次已交付（可核对）

| 项 | 证据 |
|---|---|
| 种子地图 v2 CSS 增量 5 条 | 原内容前缀字节比对 `True`（零改动），23,954→27,417 B，括号 202/202 |
| Worker 部署 | Version `9d4fd0f4-4252-49ee-9e1b-87ab925b326f`，线上 CSS `HTTP 200 / 18,548 B`（压缩后），5 条规则特征全命中 |
| box 部署 | sha256 `7afab0e7…` 双验一致，`active`，pid 225459，19429 在听；线上 CSS `27,417 B`、`?v=2026102826`、增量标记 ×2 |
| RAG | 216 文件 → 11,643 块；SQLite 16.5MB，FTS5 `MATCH '头像'` → 267 行 |
| 历史聊天归档 | 49 会话 → 735 用户消息 / 5,853 助手消息 → 48 个 md / 1,708,800 字 |
| MCP | 6/6 真握手通过（desktop 17 / rea 207 / filesystem 14 / memory 9+1 / sequential-thinking 1 / sqlite 8 tools） |
