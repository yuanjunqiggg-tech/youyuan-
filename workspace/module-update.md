# 更新模块：前端 / 插件 / 命令链 / MCP

> 本文只讲「改一处、发到哪里、怎么确认生效」。
> 部署命令的完整版在 `deploy-cloud.md`；这里不重复环境与回滚。

---

## 0. 先分清三条加载路径

| 你改的东西 | 线上怎么被加载 | 生效方式 |
|---|---|---|
| `upstream/frontend/**`（网页） | ① Worker 静态资源 `cloudflare/public/`；② APK / Linux 二进制里的 `//go:embed frontend/**` | ① `build_frontend.py` + `wrangler deploy`；② 重新 `go build` |
| `upstream/*.go`（宿主） | 编进二进制（Android `.so` / Windows `.exe` / Linux ELF） | 重新构建 + 部署 |
| 数据目录里的插件 / 链 TXT | 运行时从 `/var/lib/youyuan` 读取，**不进二进制** | 热更新：写文件 + 调接口重载 |
| `cloudflare/*.mjs` / `auth-service/*` | 各自独立 Worker | `wrangler deploy` |
| `upstream/skills/**`（30 个技能） | `//go:embed skills` 编进二进制 | 重新构建 |

**最容易犯的错**：改完前端只 `wrangler deploy`，然后去 APK 里看，发现没变——因为 APK 用的是内嵌副本。

---

## 1. 前端：新增/修改一个工具模块

### 1.1 两套注册表，别搞混

| 界面 | 注册表 | 位置 |
|---|---|---|
| 首页工具区（可排序、置顶） | `yyModules` / `yyCategories` / `yyPrefs.order` | `upstream/frontend/js/youyuan_dashboard.js:33-59` |
| 「更多工具」分类页 | `yyToolsCards` + `yyToolsCats` | `upstream/frontend/js/features/others.js:4-18` |

一个页面想同时出现在两处，就要两边都注册（或像 v2 那样在运行时 push，见 `youyuan_seedmap_v2.js:2357`）。

### 1.2 标准步骤（以新增 `youyuan_xxx.js` 子页为例）

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend"
```

1. **写页面脚本**：`js/youyuan_xxx.js`
   - 用 IIFE 包起来，避免污染全局
   - 页面 HTML 用一个 `pageHtml()` 返回字符串，通过 `openSubPage(标题, html)` 打开（`js/app.js:110`）
   - 退出清理：`activeRenderers.push(teardown)`
   - 请求统一走 `yyRootFetch`（多账号路由）或 `app.js` 的 `api()`
2. **写样式**：`css/youyuan_xxx.css`，**所有选择器加前缀**（如 `.sm2-`、`.ysm-`、`.yy-seed-`）。三套种子页能共存，就是因为前缀没撞。
3. **挂载脚本**：在 `index.html` 里加
   ```html
   <link rel="stylesheet" href="/css/youyuan_xxx.css?v=2026102824">
   <script src="/js/youyuan_xxx.js?v=2026102824"></script>
   ```
   位置要放在 `app.js` 之后、`youyuan_navigation.js` 之前（需要 `openSubPage` 已定义）。
4. **注册入口**：
   - 首页：在 `yyModules` 数组加 `{id, name, desc, category, icon, open}`
   - 更多工具：在 `yyToolsCards` 加 `{i:svg, t:标题, d:描述, cat:分类, act:function(){delete subPageCache['标题']; openSubPage(...)}}`
   - 新分类：加进 `yyToolsCats`，否则不渲染
5. **语法检查**（不是 `--test`）：
   ```bash
   node --check js/youyuan_xxx.js
   node --check js/features/others.js
   ```
6. **本地看效果**：直接开 `index.html` 会缺 API；用本机内核起的服务（`http://127.0.0.1:19429`）或 `preview/` 下的静态预览。
7. **构建 + 部署**：见 §1.3。

### 1.3 版本戳：不做这一步，用户永远看到旧文件

`index.html` 当前有：

- 149 处 `?v=2026102824`
- 2 处 `?v=2026102825`（`youyuan_global_capture.js`、`youyuan_qq_console.css`）

改完前端后，把**全部** `?v=YYYYMMDDNN` 统一换成当天新号（建议 `date +%Y%m%d` + 两位序号），例如：

```bash
cd upstream/frontend
NEW=$(date +%Y%m%d)01
python - <<PY
import re,io
p='index.html'
s=io.open(p,encoding='utf-8').read()
s=re.sub(r'\?v=\d{10}', '?v=$NEW', s)
io.open(p,'w',encoding='utf-8',newline='').write(s)
print('stamped $NEW')
PY
grep -o "?v=$NEW" index.html | wc -l
```

⚠️ **还有一个隐藏版本戳**：`js/youyuan_seedmap_v2.js:145` 自己注入 CSS：

```js
link.href = '/css/youyuan_seedmap_v2.css?v=2026102601';
```

它和 `index.html` 里的 `?v=2026102824` 不一致 → 浏览器会当成两张表（`ensureCss` 只按 `id` 去重，`index.html` 已经有同 id 的 link，所以实际不会重复插入；但只要有人删了 index 的 link，就会拿到一个旧缓存 URL）。
**规则**：改 v2 样式时，`index.html` 与 `youyuan_seedmap_v2.js:145` 两处一起改。

### 1.4 双端 embed：网页发了不等于客户端发了

```bash
# A. 网页端（Worker 静态）
D:/YouYuan-Codex/.venv/Scripts/python.exe maintenance/build_frontend.py
cd cloudflare && "C:/Users/Lenovo/.workbuddy-ai/binaries/node/versions/22.22.2-6/node.exe" \
  "C:/Users/Lenovo/AppData/Local/npm-cache/_npx/31df52248099582d/node_modules/wrangler/bin/wrangler.js" deploy

# B. 客户端（APK + Windows + Linux 共用同一份 embed）
cd .. && powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
```

`build.ps1` 会先算 `upstream/frontend` 的整树摘要并注入 `main.frontendHash`（`build.ps1:17-42`）。运行时 `RunIntegrityChecks` 重算并比对；不一致就拒绝启动。所以**不能只替换二进制里的前端文件**，必须整包重编。

### 1.5 前端改动的验收

- [ ] `node --check` 全部改过的 JS
- [ ] 桌面 1440 / 平板 768 / 手机 390 三档无横向溢出
- [ ] 深色 + 浅色都看一遍（CSS 变量是 `--phx-*`，别写死颜色）
- [ ] 控制台无 404（尤其是新增 CSS/JS 的路径与版本戳）
- [ ] 手机触控区 ≥ 44px（历史 UI 审查的固定项）
- [ ] `build_frontend.py` 输出里没有 `.map`

---

## 2. 插件更新（Lua / Python / C++）

### 2.1 插件在哪

运行时根目录 = 数据目录（Linux `/var/lib/youyuan`，Android `/data/data/org.youyuan.toolbox/files`）：

```
<dataDir>/
├─ 插件文件/<id>/                 # 代码：main.lua / main.py / *.so / manifest / docs
├─ 插件配置文件/<id>/             # 用户配置
└─ 插件数据文件/<id>/             # 插件自己的持久数据
```

规则来源：`plugin_paths.go:10-29`、`plugin_backup.go:16`、`ai_filetools.go:45`。

- `<id>.manifest.json`：`id/name/version/description/type/author/pre_plugins`（`plugin_scan.go:12-21`）
- `<id>.docs.md`：给 AI 与用户看的说明
- 插件 id 只能是安全字符；`validPluginID` 拒绝 `' " \` < > ( )`（防 DOM 注入）

### 2.2 更新流程（推荐走接口，不手改远端文件）

| 步骤 | 接口 | 说明 |
|---|---|---|
| 1. 备份 | `GET /api/plugin/export?id=<id>` | ZIP 含 `插件文件/插件配置文件/插件数据文件` 三块，带 manifest |
| 2. 停 | `POST /api/plugin/stop` `{id}` | 先停再改，避免运行中读半截文件 |
| 3. 写 | `POST /api/plugin/file/write` `{id,path,content}` | 或 `POST /api/plugin/import` 上传 ZIP 覆盖 |
| 4. 重载 | `POST /api/plugin/reload` `{id}` | 重新扫描 + 启动（若原本是启用状态） |
| 5. 看状态 | `GET /api/plugin/list` | 核对 `running` / `disabled` / `error` |
| 6. 导出回滚包 | `GET /api/plugin/export?id=<id>` | 改坏了直接用备份 ZIP 导入 |

批量/远端也可以用 `POST /api/plugin/create`、`/api/plugin/meta`、`/api/plugin/disabled`。

### 2.3 三种语言各自的坑

| 类型 | 入口 | 坑 |
|---|---|---|
| Lua | `main.lua`，gopher-lua 常驻 VM | 改完必须 `reload`；`require` 的模块要一起更新；字符串拼接寄存器问题曾导致巡查 AI 空指针 |
| Python | `main.py`，CPython 3.12 子进程 + JNI 线程 | 每实例独立进程/线程、队列 256；`host.save()` 才是跨进程持久化，模块全局变量不保留 |
| C++ | `plugin.so`，稳定 C ABI | APK 里**没有编译器**；必须用对应 ABI 的 NDK 编好再上传；Linux 侧需要 `YY_PYTHON` 与同 ABI `.so` |

### 2.4 验收

- `GET /api/plugin/list` 里目标插件 `running=true`、`error=""`
- 实际触发一次插件命令/事件，看到游戏侧回执（不能只看「已启动」）
- 备份 ZIP 能在另一个空目录导入并通过 `manifest` 校验
- 插件数据文件未被误删（对比更新前后的文件数/大小）

---

## 3. 命令链 TXT 更新

### 3.1 位置与语义

- 目录：`<dataDir>/插件数据文件/ds_ai_agent/command_txt/`（`youyuan_commands.go:707`）
- 同名 `<name>.json` 是配置（`yyChainConfig`），TXT 是命令正文
- 列表 = 磁盘目录 ∪ 内存运行态；进程启动后才放进去的 TXT 会显示「未启动」，点启动时按需装载（`youyuan_commands.go:759-790`）
- 计数语义（别误读）：
  - `executed` = 尝试执行的节点数
  - `submitted` = 宿主已接受
  - `cycles` = 文件轮次或分段轮次之和
  - **三者都不等于服务端真的执行成功**；要 `results_confirmed`

### 3.2 更新步骤

```bash
# 1) 先预览解析结果（不落盘、不启动）
curl -sS -X POST http://127.0.0.1:19429/api/youyuan/chains \
  -H 'Content-Type: application/json' \
  -d '{"action":"preview","name":"demo.txt","content":"say hi\nsay hello","config":{"name":"demo.txt","account":"main","interval_ms":1000}}'

# 2) 保存（会 yyChainCadenceConfig 归一化、校验通道/次数/HZ）
curl -sS -X POST http://127.0.0.1:19429/api/youyuan/chains \
  -H 'Content-Type: application/json' \
  -d @payload.json

# 3) 启动 / 停止 / 单次触发 / 红石开关 / 重载
#    action = start | stop | trigger | redstone | reload
```

支持的 action：`preview` / `save` / `delete` / `start` / `stop` / `trigger` / `redstone` / `reload`（`youyuan_commands.go:897-1031`）。

`config` 关键字段：`name`、`account`（默认 `main`）、`interval_ms`（≥1）、`hz`、`frequency_mode`、`channel`、`run_count`（0=持续，1..1000000）、`delay_ms`、`mode`（`fast` 时 `results_confirmed=false`）。

### 3.3 纪律（血泪）

1. **先备份原 TXT 再改**，文件名带时间戳；改完记 sha256。
2. **先 preview 再 save**，解析失败时不要强推。
3. **不要在生产上直接开原雪球主链复现卡服**；原链当前是停用状态，除非用户明确要求。
4. **频率不是越高越好**：20Hz 是历史事故高发区；改 Hz 要同时看 `queue`、`dropped`、`query` 耗时。
5. 分段/条件模块改动后，至少跑一轮 `reload`，确认 `segments` 与 `condition_modules` 行数正确。
6. 多人同时在服时，任何「1 万条/秒」的结论都不算数，必须注明并发人数与 RTT。

### 3.4 验收

- `GET /api/youyuan/chains` 中目标链 `state=运行中`、`error=""`
- 跑 1–2 轮后看 `cycles/executed/submitted/skipped`
- `GET /api/bot/status` 的 `connected=true`
- 游戏内看到实际效果（回执），而不是只看接口 200

---

## 4. 新增 API 后的「注册三查」

新增一个 `/api/...` 路由，**只改 handler 不够**，还要过三张表，否则要么 MCP 看不到，要么只读连接被误拒。

| # | 表 | 位置 | 作用 | 漏了会怎样 |
|---|---|---|---|---|
| 1 | `yyAPIRoutes`（catalog） | `upstream/youyuan_catalog.go`（313 条） | 给 AI/文档/自检用的路由清单 | 路由存在但 catalog 查不到，AI 不知道能调 |
| 2 | `yyMCPReadPaths` | `upstream/youyuan_mcp_links.go:27`（26 条） | 只读 MCP 连接允许的 GET 白名单 | 只读连接被 403，或反过来把写接口放进来 |
| 3 | `yyExtraMethods` | 各模块 `init()` 里注册（如 `youyuan_capabilities.go:14`） | SDK 方法名 → 描述 | 插件/脚本调不到，AI 工具列表缺项 |

### 4.1 操作顺序

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"

# 1) 先生成 catalog（它会扫所有 mux.HandleFunc("/api/...") 重写 youyuan_catalog.go）
D:/YouYuan-Codex/.venv/Scripts/python.exe generate_catalog.py
#    输出「原生 API 路由数：N」，确认 N 比上次多 1

# 2) 手工确认只读白名单
grep -n "你的路径" upstream/youyuan_mcp_links.go

# 3) 如果要做成 SDK 方法，在对应 init() 里注册
grep -rn "yyExtraMethods\[" upstream/*.go | head
```

### 4.2 三查的验收

- `go build ./...` 通过（`cmd/server` 的既有失败不算）
- `curl /api/youyuan/sdk/catalog` 能看到新路径
- 用只读 MCP 连接 `tools/call` 调一次 GET，返回 200；调 POST 应被拒
- `/mcp` 的 `tools/list` 里能看到新增工具（如果注册了工具名）

---

## 5. 更新后的统一验证顺序

```
node --check（前端）
  → go vet . / go test（宿主）
  → node --test cloudflare auth-service（Worker）
  → build_frontend.py（网页产物）
  → wrangler deploy（Worker / auth）
  → build.ps1（三平台）
  → deploy_linux_bin.py（box）
  → /health + 19429 + 页面 + 实际功能
  → 写 outputs/ 证据 + 更新 协作交接/维护入口.md 顶部
```

**不要跳步**：本项目过去最大的返工来自把「本地测试通过」「已构建」「已部署」「真机验收」混为一谈。
