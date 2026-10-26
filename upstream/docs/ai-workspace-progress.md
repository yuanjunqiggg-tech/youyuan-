# AI 帮写工作区 · 移动端与授权改造进度

改动范围：`upstream/frontend/js/youyuan_workspace.js`、`upstream/frontend/css/youyuan_agent_mobile.css`、
`upstream/frontend/index.html`（仅资源引用与版本号）。未触碰 `youyuan_qq_*`、Go 后端、部署与重启。

## 第 1 步 移动端单栏（完成）

- 新增 `css/youyuan_agent_mobile.css`，全部规则收在 `@media (max-width:720px)` 内，桌面端零影响。
- `.yy-workspace` 在移动端改为竖向 flex，一次只显示一个 pane；页签用 `order:3` 移到内容下方（纯 CSS，不改 DOM 结构）。
- `yyWorkspacePane()` 同步写 `#yy-workspace.dataset.pane`，配合 `[data-pane]` 选择器切换。
- 输入区 `.yy-agent-compose` 用 `padding-bottom: max(6px, env(safe-area-inset-bottom))` 贴底并避开 home 指示条。
- 按钮/输入框统一 `min-height:44px`，正文 15px、次要文案不低于 12.5px。

实测（Chromium，`python -m http.server 8793`）：

| 视口 | 页签 | 可见 pane | scrollWidth / clientWidth | compose bottom / innerHeight |
|---|---|---|---|---|
| 390×844 | agent | 仅 agent | 390 / 390 | 787 / 844 |
| 390×844 | code | 仅 code | 390 / 390 | — |
| 390×844 | context | 仅 context | 390 / 390 | — |
| 360×780 | agent | 仅 agent | 360 / 360 | 723 / 780 |
| 360×780 | context | 仅 context | 360 / 360 | — |

三页签 `visibleCount === 1`，无横向滚动，输入区未越出视口。

## 第 2 步 MCP 分组（完成）

- 「历史 / MCP」页签内拆成三组：`本机工具箱（默认 · 只读）` / `外部 MCP（高级）` / `危险操作授权`。
- 添加外部 MCP 的表单收进 `<details class="yy-mcp-external">`，summary 文案
  `外部 MCP（高级 · 会让 AI 调用你自己的外部服务，不是本机工具箱）`，默认不展开。
- 展开后先出现勾选框「我已知晓这会把工具调用发给外部服务」；未勾选时「添加 / 保存」按钮 `disabled`
  且置灰（`yySyncMCPAck()` 联动），勾选状态记入 `localStorage.yyMcpAck`。
- 已配置的外部 MCP 列表项带红色 `外部` 徽章，并显示其 URL；无配置时提示"AI 只会使用本机工具箱"。

实测：初始 `summary` 文案正确、`open=false`、`save.disabled=true`；勾选后 `disabled=false` 且
`yyMcpAck=1`；两行外部服务均带 `外部` 徽章；展开态 `#yy-workspace-context` 的
`scrollWidth === clientWidth === 390`（无横向溢出）。

## 第 3 步 危险操作授权抽屉（完成）

- `yyRiskOf(tool, args)` 判定：工具名/参数命中
  `sendCommand|execute|setblock|summon|gamerule|deop|op|ban|kick|kill|clear|give|tp|remove|delete|rm|format`，
  或工具属于 `mcp_call / project_rollback / project_fetch_artifact / plugin_manage`。
- 命中后弹底部抽屉：标题「这一步有风险」、等宽字体显示工具名与参数、三个按钮
  `允许一次` / `本次全部允许` / `拒绝`，另有「以后同类不再询问」勾选框。
- 支持下滑关闭与点遮罩关闭，均等同拒绝。
- 记忆：勾选后按工具名写入 `localStorage.yyRiskPolicy`（抽屉内部直接落盘，不依赖调用方），
  「本次全部允许」写内存 `yyRiskSessionAllow["*"]`，仅本次会话有效。
- 入口：`yyAgentTool()` 执行前拦截；agent 任务流里检测到 `tool_start` 且任务运行中时由
  `yyRiskInterceptAgent()` 暂停任务并询问，允许后自动 resume。
- 「历史 / MCP → 危险操作授权」列出已记住的工具，可单项「恢复询问」或一键清空。

实测：抽屉 390×844 下位于底部（top 463 / bottom 844 / 宽 390），标题与三按钮文案正确；
点「本次全部允许」返回 `{choice:"allow_all"}` 且遮罩移除；勾选后点「允许一次」写入
`{"mcp_call":"allow"}`，再次调用 `yyAgentTool('mcp_call', …)` 不再弹窗；策略列表显示「已记住 1 项」。

### 已知遗留（不在本次改动边界内）

`#yy-identity-gate` 的 `z-index: 2147480000` 高于抽屉的 `320`。该门只在未完成身份初始化的页面上出现，
真实登录态下不显示；若后续要在门内也弹授权，需要把抽屉 z-index 提到它之上（或复用门内层级）。

## 截图

`upstream/docs/shots/`：`ai-mobile-01-agent.png`、`ai-mobile-02-code.png`、`ai-mobile-03-mcp.png`、
`ai-mobile-04-mcp-external.png`、`ai-mobile-05-risk.png`、`ai-mobile-07-narrow360.png`。

截图时页面停在未初始化身份的门上（`#yy-identity-gate` 全屏不透明，`z-index: 2147480000`），
因此截图前在运行时临时 `display:none` 掉该门——只是测试脚本行为，源码未改。
`ai-mobile-06-keyboard.png` 是上一轮遗留，本轮未复核，仅供参考。

## 资源版本

`index.html` 中 `youyuan_agent_mobile.css` 与 `youyuan_workspace.js` 的 `?v=` 已更新为 `2026102503`，避免旧缓存。
