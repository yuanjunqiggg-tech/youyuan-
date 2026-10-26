# DeepSeek 官方搜索能力接入调研与方案

> 调研人：general-purpose-1 ｜ 日期：2026-10-25 ｜ 性质：只读调研 + 方案设计，未改动任何 .go/.js/.html
> 项目根：`C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream`

## 0. 一句话结论

**DeepSeek 官方确实有搜索 API（不是 R1 深度思考、不是chat内置工具，而是独立的 REST 接口）**：`POST https://api.deepseek.com/v1/tools/search`（另有 `/v1/tools/fetch` 抓网页转 Markdown），**复用项目里已有的 DeepSeek API Key，无需单独密钥**，按成功请求计费 $0.75/千次。本项目只需新增一个 `web_search` 工具并在两条 AI 链路（工具箱 AI 帮写、QQ 机器人）暴露，改动集中在 1 个新文件 + 3 处接线，`openAIBody` 不用动。

---

## 1. 调研结论（事实均来自官方文档，附来源）

### 1.1 DeepSeek 官方搜索能力：存在，名为 Search & Fetch API

| 项目 | 事实 | 来源 |
|---|---|---|
| 产品名 | **Search API / Fetch API**（DeepSeek 自研搜索引擎驱动） | [api-docs.deepseek.com/guides/search](https://api-docs.deepseek.com/guides/search) |
| Search 接口 | `POST https://api.deepseek.com/v1/tools/search` | 同上 |
| Fetch 接口 | `POST https://api.deepseek.com/v1/tools/fetch`（抓单个网页，返回 Markdown） | 同上 |
| API 形态 | 官方原文："Both are **raw JSON APIs**"——是独立 REST 接口，**不是** chat completions 的内置 server-side tool（与 OpenAI web_search tool 形态不同） | 同上 |
| 鉴权 | 与 DeepSeek API 完全相同的 Bearer Key：`Authorization: Bearer <DeepSeek API Key>`，**不需要单独申请密钥** | 同上 |
| 计费 | Search / Fetch 均 **$0.75 / 1000 次成功请求**（按次，不按 token、不按返回条数）。仅 HTTP 200 且有结果才计费；失败、空结果、超时不计费 | 同上（Billing 一节） |
| 计费生效时点 | 经 DeepSeek Harness / Claude Code / Codex 的服务端工具调用**目前免费**，2026-11-01 00:00（北京时间）起按次计费。本项目是直连 api.deepseek.com 的自有 key，一直按次计费 | 同上 |
| 限速 | 账户级：Search 100 QPS，Fetch 25 QPS（互不占用） | 同上（Rate Limits 一节） |
| 当前模型名 | `deepseek-flash`（由 DeepSeek-V4.1-Flash-1017 服务）、`deepseek-v4-pro` | [api-docs.deepseek.com Your First API Call](https://api-docs.deepseek.com/) |

### 1.2 Search API 请求/响应细节（来自官方 API Reference）

请求字段（[api/search](https://api-docs.deepseek.com/api/search)）：

| 字段 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|
| `query` | string | 是 | — | 1–128 个 Unicode 字符，不能全空白；支持 `site:`、`-site:`、`-`、`-"..."`、`"..."`、`lang:`、`after:`、`before:` 操作符 |
| `count` | integer | 否 | 10 | 返回条数 1–50 |
| `ori_question` | string | 否 | null | 触发搜索的原始问题（≤128 字符），提升查询理解 |
| `localization` | string | 否 | "" | 可选 IP，本地化结果 |
| `need_content` | boolean | 否 | false | 是否返回页面全文 `content` |

响应 `results[]` 每项字段：`rerank_score`(0–3，≥1.5 为优质)、`title`、`url`、`snippet`（100–200 字短摘要，**UI 展示用**）、`summary`（600–800 字长摘要，**给模型读**）、`content`（全文，仅 `need_content=true`）、`site_name`、`site_icon`、`date`（RFC 3339 UTC）、`authority`（0–4，≥3 高权威）。

错误码为 OpenAI 兼容格式：400/401/402(余额不足)/403/404/408/413/429(超 QPS)/500/504。

### 1.3 澄清：容易混淆的几件事

- **「DeepSeek 官方搞了一个搜索引擎」指的是这个 Search API**，官网首页原文："新增 API 支持网页搜索与抓取"（[deepseek.com](https://www.deepseek.com/)）。它不是新的对话模型，R1/深度思考与搜索是两回事。
- **DeepSeek 没有类似 OpenAI 的服务端内置 web_search tool**：chat completions 的 function calling 是标准的客户端侧 `tools` 机制（[function_calling 文档](https://api-docs.deepseek.com/zh-cn/guides/function_calling)），官方明确 Search/Fetch 是 raw JSON APIs。所以接入方式是"自己的代码调 REST 接口 + 走工具循环把结果喂回模型"，不是改请求体加个 tool 就行。
- 网络中流传的"chat/completions 加 search_engine 参数"（百度云文章 / CSDN）**未在官方文档中找到任何依据，判定为不实**，不要采用。

---

## 2. 本项目 AI 接入层现状（代码事实）

### 2.1 模型配置层（ai_models.go）

- `AIModel` 结构（ai_models.go:12）：`Name / Provider / BaseURL / APIKey / ModelID / Thinking / Streaming / 价格 / ContextWindow / MaxOutputTokens / CompressionPercent / Builtin`。Provider 只有 `anthropic` 和 `openai`（openai 兼容系含 DeepSeek）。
- `aiStore`（ai_models.go:55）落盘 `<dataDir>/ai_models.json`（setPath, ai_models.go:58-86）。内置模型「游园自定义」从环境变量 `YY_AI_BASE_URL/YY_AI_API_KEY/YY_AI_MODEL` 读。
- **项目已在迁移 DeepSeek 模型名**（ai_models.go:69-82）：`deepseek-chat → deepseek-flash`（thinking=off）、`deepseek-reasoner → deepseek-flash`（thinking=high）。用户当前配置（模型名 "Deep seek"，model_id `deepseek-flash`）与官方现行模型名一致，**密钥就是 Search API 的密钥**。

### 2.2 请求组装层（ai_provider.go）

- `complete → completeOnce → completeOpenAI/completeAnthropic`（ai_provider.go:137-191）；`openAIBody`（ai_provider.go:510）组装 OpenAI 兼容请求体，对 `deepseek.com` 有特判（`reasoning_content` 回传、`thinking` disabled/enabled，ai_provider.go:541-601）。
- **搜索是独立 REST API，与 chat completions 无关，openAIBody / complete 流程零改动。**

### 2.3 工具箱 AI 帮写链路（ai_chat.go + ai_tools.go）

- `handleAIChat`（ai_chat.go:39）：`model := aiStore.Get(req.ModelName)` → 工具循环 `model.complete(ctx, messages, aiToolDefsAll(), hooks)`（ai_chat.go:111）→ `execTool(name, args)`（ai_tools.go:220，签名无模型上下文）。
- `aiToolDefs()`（ai_tools.go:42）**当前没有搜索工具**，只有 `network_request`。注意：系统提示 buildAISystem（ai_chat.go:170）写了"DuckDuckGo 网络搜索"，但**代码里并不存在该工具**（全仓 grep 仅此一处提及），实际只能靠 `network_request` 自己拼 DuckDuckGo 请求——提示与实现不符，属于既存问题。
- `execTool` 里 `user_question`/`rename_session` 是在 handleAIChat 循环内特判的（ai_chat.go:125-154），**model 变量在手边**——新搜索工具可照此特判，无需改 execTool 签名。

### 2.4 QQ 机器人链路（youyuan_qq_ai.go + youyuan_qq_web.go）

- 模型入口：`model := aiStore.Get(cf.Assistant.Model)`（youyuan_qq_ai.go:135）。
- 工具注册：`yyQQAITools(admin)`（youyuan_qq_ai.go:77）。**管理员独享**：`game_call / load_skill / mcp_* / web_fetch / seed_*`；成员固定 4 个基础工具 + 全员社交工具。
- `web_fetch` 是本机直抓（youyuan_qq_web.go，带完整 SSRF 防护：公网 IP 校验、拨号层复检、≤3 跳、512KB/8s/6000 字），执行在 `yyQQRunAIToolEx`（youyuan_qq_ai.go:376）。
- `yyQQRunAIToolEx` 签名不带 model，但分支内可 `aiStore.Get(cf.Assistant.Model)` 现取（与入口一致）。
- 前端选模型：群服互通控制台 `yyQQC.cfg.assistant.model = model`（youyuan_qq_console.js:1354），模型来源就是 aiStore 列表。

### 2.5 前端模型配置入口（ai_assist.js）

- `aiOpenManage()`（ai_assist.js:72）：表单含 供应商/BaseURL/API Key/模型 ID/本地名/上下文容量/输出预留/压缩阈值/思考强度/流式/单价，保存走 `POST /api/ai/models`（ai_handlers.go:54，`AIModel` 直接解码入库）。
- **用户要"配置上"搜索，不需要新表单字段**：Search 复用同一模型的 BaseURL+APIKey。可选优化是在模型管理加一个"联网搜索"开关（见 §3 步骤 5）。

---

## 3. 最小改动接入方案

核心思路：新增 1 个 Go 文件实现 `web_search` 工具（调 DeepSeek Search REST API），在两条 AI 链路各接 2-3 行。**`openAIBody`/`complete`/`aiStore`/前端表单全部不动。**

### 步骤 1：新增 `ai_search.go`（唯一新文件）

```go
// 工具定义：名称 web_search，参数 query(必填)/count(默认5,1-50)/need_content(默认false)
// 描述写明：DeepSeek 官方联网搜索，返回标题/链接/摘要；查最新信息、时事、资料时用。
func webSearchToolDef() ToolDef

// 执行：POST {base}/v1/tools/search，Authorization: Bearer m.APIKey
// base 归一化复用 completeOpenAI 同款逻辑（去掉末尾 /v1 或 /chat/completions 再拼 /v1/tools/search）
func deepseekSearch(ctx context.Context, m *AIModel, query string, count int, needContent bool) (string, error)
```

实现要点：
- **gate**：仅当 `strings.Contains(strings.ToLower(m.BaseURL), "api.deepseek.com")` 且 `m.APIKey != ""` 时真正发请求；否则返回错误文案"当前模型不是 DeepSeek 官方接口，联网搜索不可用，可改用 network_request"。
- 响应格式化：每条结果输出 `[i] title\nurl\nsnippet/summary`，`count` 默认 5，总输出截断 ~4000 字（对齐 ai_chat.go:338 的工具输出截断惯例）。`need_content` 默认 false，要全文时引导模型对具体 URL 用 Fetch 或 web_fetch。
- 错误处理对齐官方建议：429/5xx/408 不重试（工具循环会自然继续），402 提示充值。

### 步骤 2：工具箱链路接线（ai_tools.go + ai_chat.go）

- `aiToolDefs()` 返回值追加 `webSearchToolDef()`（ai_tools.go:42 的切片里加一项）。
- `handleAIChat` 工具循环内特判（仿 ai_chat.go:125 `user_question` 的写法，约 ai_chat.go:155 处）：
  ```go
  if tc.Name == "web_search" {
      res, isErr := runWebSearch(ctx, model, tc.Input) // model 已在作用域（ai_chat.go:58）
      ...same as user_question 分支...
  }
  ```
- 顺手修正 buildAISystem（ai_chat.go:170）：把"DuckDuckGo 网络搜索"改为真实存在的"web_search（DeepSeek 官方联网搜索）与 network_request"，消除提示与实现不符的既存问题。

### 步骤 3：QQ 机器人链路接线（youyuan_qq_ai.go）

- `yyQQAITools` admin 分支追加 `tools = append(tools, webSearchToolDef())`（放在 youyuan_qq_ai.go:107 `yyWebFetchToolDef()` 旁边）——**与 web_fetch 一样只给管理员**，控制外网流量与按次计费风险；成员工具集保持固定不变。
- `yyQQRunAIToolEx` 加分支（youyuan_qq_ai.go:376 `web_fetch` 分支旁）：
  ```go
  if name == "web_search" {
      m := aiStore.Get(cf.Assistant.Model)
      out, e := deepseekSearch(ctx, m, aiArgStr(args,"query"), aiArgInt(args,"count",5), false)
      // 错误/成功都走 mustJSON({ok,...})，与 web_fetch 分支同一形状
  }
  ```
- 系统提示（youyuan_qq_ai.go:183 起）可加一句：管理员可用 web_search 查最新信息，搜索结果要注明来源链接、不确定就说不确定。

### 步骤 4（可选但建议）：Fetch 备用

`/v1/tools/fetch` 同一文件内实现 `deepseekFetch(ctx, m, url)`，作为 web_fetch 的补充（返回 Markdown、质量高于本地 yyWebStrip）。不必替换 web_fetch——本地抓取有 SSRF 防护且不计费，Fetch 按次计费。**建议先只上 Search，Fetch 观察用量后再决定。**

### 步骤 5（可选）：模型管理加开关

若希望非 DeepSeek 模型不暴露 web_search 工具：`AIModel` 加 `WebSearch bool` 字段（ai_models.go:12，JSON tag `web_search,omitempty`），ai_assist.js 表单加复选框，`aiToolDefs()`/`yyQQAITools` 按开关过滤。**默认不加**：工具 gate 已按 BaseURL 判断，非 DeepSeek 模型调用即报错，零配置也对。

### 改动清单汇总

| # | 文件 | 改动 | 量级 |
|---|---|---|---|
| 1 | `ai_search.go`（新建） | webSearchToolDef + deepseekSearch | ~120 行 |
| 2 | `ai_tools.go` | aiToolDefs() 追加 1 项 | 1 行 |
| 3 | `ai_chat.go` | 循环内特判 web_search + 修 buildAISystem 1 句 | ~10 行 |
| 4 | `youyuan_qq_ai.go` | yyQQAITools +1、yyQQRunAIToolEx +1 分支、（提示 1 句） | ~12 行 |
| 5 | （可选）`ai_models.go` + `ai_assist.js` | web_search 开关字段与表单 | ~10 行 |

`ai_provider.go`、`ai_models.go` 的 store、`ai_handlers.go`、路由、`openAIBody`、`complete`：**零改动**。

---

## 4. 风险与备选

1. **没有"服务端一键联网"**：DeepSeek 官方没有 OpenAI 式内置 web_search tool，必须自己调 REST + 工具循环。本方案即是官方推荐形态（raw JSON APIs + function calling 回传）。
2. **计费**：$0.75/千次成功请求，失败/空结果不计费。默认 count=5、need_content=false 可把单次成本与上下文消耗都压到最低。QQ 侧仅管理员可用，天然限流。
3. **非 DeepSeek 模型**：web_search 不可用（gate 报错）。回退：工具箱侧 `network_request` 可自行拼 DuckDuckGo HTML/API；QQ 侧 `web_fetch` 本机直抓（需已知 URL）。
4. **搜索质量与合规**：结果注入上下文前，QQ 侧必须过既有的 `yyQQRedactSecrets`（youyuan_qq_ai.go:337，key 脱敏）与 5000 字截断（:338）——接线时把搜索结果同样过一遍（放在分支内返回前）。系统提示要求模型注明来源、不知为不知。
5. **错误码 402**（余额不足）要在文案里提示用户去 platform.deepseek.com 充值，其他 4xx 不重试。
6. **备选方案**（若不愿按次付费）：继续用 network_request 拼 DuckDuckGo（零成本、质量不稳、无官方 SLA），或只对管理员开放。**不推荐**为了"免费"去接非官方聚合接口——项目红线是只用用户自配置的官方 key。

## 5. 待用户拍板的点

- 搜索工具命名：`web_search`（与官方 Search API 语义一致）还是 `deepseek_search`（标明来源）？本文按 `web_search` 写。
- QQ 侧是否只给管理员（本文建议：是，与 web_fetch 一致）？
- 是否同时上 Fetch API（本文建议：先不上）。
