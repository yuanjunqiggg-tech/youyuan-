# 已安装 / 候选 Skill（2026-10-26）

> 用户 2026-10-26 11:37 要求：用 `find-skills` 找 UI 设计、修 bug、性能、网络相关 skill，并真实安装生效。
> 安装目录：`C:/Users/Lenovo/.workbuddy-ai/skills/`（本机实际生效目录）。

## 已安装（真实落盘，只有 SKILL.md / _meta.json，无脚本）

| Skill | 目录 | 用途 | 给谁用 |
|---|---|---|---|
| `ui-ux-pro-max` | `skills/ui-ux-pro-max/` | 50+ 风格、161 配色、57 字体搭配、99 条 UX 准则、25 种图表；覆盖 Web/移动端/后台 | **UI 设计常驻 Agent** |
| `frontend-design` | `skills/frontend-design/` | 独特、生产级前端；明确「避免通用 AI 美学」 | **UI 设计常驻 Agent**（反 AI 味） |
| `bug-fix-protocol` | `skills/bug-fix-protocol/` | 结构化修 bug：先诊断再改，防幻觉与修 bug 死循环 | **修 bug 固定模板 Agent** |

验证：三者 `SKILL.md` 均存在，frontmatter `name` 与目录名一致。

## 候选（未安装，按需再装）

| Slug | 用途 | 备注 |
|---|---|---|
| `ui-design-guide-cn` | 中文界面设计与评审（信息层级/组件状态/响应式） | 适合中文文案审查 |
| `premium-ui` | 大厂级、高级简约、品牌化 UI | 适合连接/认证页重做 |
| `design-system-creation` | 从零建设计系统（Token/组件/动效） | 适合玻璃/青桔多主题 |
| `performance-profiling` | Python/Node 性能分析 | 对 Lua/MC 命令链帮助有限，需自写探针 |
| `bug-fix-farming` | 七步修 bug 工作流 + 人工检查点 | 与 `bug-fix-protocol` 二选一即可 |

## Agent → Skill 对应

| Agent | 先用哪个 skill | 提示词落点 |
|---|---|---|
| UI 设计常驻 | `ui-ux-pro-max` + `frontend-design` | `youyuan-agent-playbook/prompts/ui-audit.md` + 本目录 `ui-audit` 报告 |
| 修 bug 固定模板 | `bug-fix-protocol` | `prompts/implementation.md` + `prompts/verification.md` |
| 用户挑刺 | 不加载 UI skill（要“啥也不懂”的视角） | `prompts/fresh-eyes.md` |
| 性能 | 先自写探针；必要时 `performance-profiling` | 待新建 `prompts/perf.md` |
| 网络 | 无现成 skill；按 MC 协议文档 + 图 12 | 待新建 `prompts/network.md` |

## 注意

- 本会话的 Skill 工具列表在会话开始时生成；新装的三个 skill 可能需要**下一次会话**才会出现在可调用列表里。文件已落盘，不影响后续会话生效。
- 安装来源：SkillHub（`https://lightmake.site`）。三者均无 `scripts/`、无二进制，只有说明文档。
