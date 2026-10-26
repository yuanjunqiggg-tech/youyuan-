# 二、rawtext / tellraw / titleraw：JSON 消息高级用法

基岩版富文本**只能**通过 `rawtext` JSON 发送（基岩版没有 Java 式的 `title @a actionbar <纯文本>` 富文本入口，纯文本 `title` 也只吃字符串）。

两个载体命令：

```mcfunction
tellraw <目标> <json>                          # 聊天栏
titleraw <目标> title|subtitle|actionbar <json>  # 标题/副标题/动作栏
```

## 1. rawtext 的基本结构

```json
{"rawtext":[ 组件1, 组件2, ... ]}
```

数组内组件按顺序拼接。**每个组件只能有一个"内容标签"生效**，合法标签只有 4 种：

| 标签 | 作用 | 例 |
|---|---|---|
| `text` | 纯文本字面量 | `{"text":"§a你好"}` |
| `translate` | 本地化键 / 就地 `%%` 模板 | `{"translate":"%%s","with":["Steve"]}` |
| `selector` | 渲染匹配实体的**名字** | `{"selector":"@a[tag=vip]"}` |
| `score` | 渲染记分数值 | `{"score":{"name":"@s","objective":"金币"}}` |

- **权重顺序**（多标签并存时只取一个）：`translate` → `text` → `selector` → `score`。
- **基岩版没有 `keybind`**（那是 Java 版），也没有 `nbt` 节点。
- `score.name` 用 `"*"` 表示"读者本人的分数"，只在单一读者场景有效（tellraw/titleraw/成书；告示牌不行）。

## 2. `with` 的取值形式（只有两种）

```json
"with": ["Steve", "Alex"]                     // 字符串数组
"with": {"rawtext":[ {"text":"A"}, ... ]}      // rawtext 对象（可递归嵌套）
```

**必须**是数组 `[]` 或 `{"rawtext":[...]}`，不能用裸花括号对象。`with` 仅在 `translate` 存在时生效。

## 3. `%%` 占位符语法（重点）

`translate` 有两种用法：**查本地化键**（键名在 .lang 里）或**就地模板**（不需要 .lang 条目）。就地模板用 `%%` 占位：

| 写法 | 含义 |
|---|---|
| `%%s` | 按顺序取下一个参数（可多次出现） |
| `%%1` `%%2` … | 按下标取 `with` 的第 N 个参数 |
| `%%#` | 用数字指定下标 |

### 实测确认的关键规则

- **`%%N` 是绝对下标**：`{"translate":"Hello %%2 and %%1","with":["Steve","Alex"]}` → `Hello Alex and Steve`。
- **槽位取不到值时该槽不显示**（官方原文：*If no argument is provided for a slot, the slot is not displayed*）。selector 匹配不到实体 → 该槽为空。
- 同一个 `%%s` 不能用两次——第二次读到的是下一个参数；要复用同一个参数得用 `%%1` 这种显式下标。

### 官方验证过的"按分数动态选文本"范式（CornyFlex，来自 wiki.bedrock.dev）

```json
/tellraw @a {"rawtext":[{"translate":"Role: %%%%s","with":{"rawtext":[
  {"score":{"name":"*","objective":"wiki:role"}},
  {"text":"Member"},{"text":"Moderator"},{"text":"Admin"}]}}]}
```

效果：`wiki:role` 分数 = 1/2/3 时分别显示 Member/Moderator/Admin。**分数必须是正整数。**

原理（原文 *How It Works* 逐字）：`%%%%s` 的第一个 `%%` 后面不是 `s` 也不是数字，**无效**；第二个 `%%s` 取 `with[0]`（那个 `{score}`，值为 1/2/3）；因为 `%%` 与 `{score}` 之间没有空格，`{score}` 的值被**当作第一个 `%%` 的整数下标** → 于是分数值选择了后面哪个字面量被显示。

> **这条机制是"分数值 = 下标"的巧用，能用但脆**：分数必须落在 1..N 的正整数范围；分数为 0、负数或超出下标 → 该槽不显示。

### ⚠️ 未证实：嵌套 `%%N` 的解析

素材里大量出现这种写法（来自真实运行的雪球菜单）：

```json
{"translate":"%%4","with":{"rawtext":[
  {"selector":"@s[scores={s=1}]"},
  {"selector":"@s[scores={s=2}]"},
  {"selector":"@s[scores={s=1,not=0..2}]"},
  {"text":"⇨可选"},
  {"text":"✘不可选"},
  {"text":" 普通"}]}}
```

它**在线上能跑**，但**下标如何解析、为什么这样写能按条件切换，官方文档没有记载**，本 skill 无法给出确定性解释。

**给 AI 的强制约束**：不要把这种嵌套 `%%N` 当作可推广的范式照抄到新代码里。要"按条件切换文本"，优先用下面**有依据的写法**：

- **分支法（最稳）**：不同条件用各自的 `titleraw`，用 `@a[...]` 选择器区分。
  ```mcfunction
  execute as @a[scores={等级=1}] run titleraw @s actionbar {"rawtext":[{"text":"新手"}]}
  execute as @a[scores={等级=2}] run titleraw @s actionbar {"rawtext":[{"text":"老手"}]}
  ```
- **tag 门控法**：先按条件打 tag，再按 tag 分别发。
- **score 下标法**：只有分数是 1..N 连续正整数时才用 CornyFlex 范式。

## 4. 常用招式

```mcfunction
# 拼接：文本 + 玩家名 + 分数
tellraw @a {"rawtext":[{"text":"§e"},{"selector":"@s"},{"text":" 金币: "},{"score":{"name":"@s","objective":"金币"}}]}

# 换行、转义、Unicode 图标
{"text":"第一行\n第二行"}
{"text":"他说：\"你好\""}
{"text":""}     # Unicode 图标，只在游戏内渲染

# actionbar 进度条（素材实测写法，基于空槽不显示 + 下标）
titleraw @a actionbar {"rawtext":[{"translate":"%%5▊%%6▊%%7▊%%8▊%%9","with":{"rawtext":[
  {"selector":"@s[scores={血量=20..}]"},
  {"selector":"@s[scores={血量=40..}]"},
  {"selector":"@s[scores={血量=60..}]"},
  {"selector":"@s[scores={血量=80..}]"},
  {"text":""},{"text":""},{"text":""},{"text":""},{"text":"§8"}]}}]}
```

> 上例中分数槽在**前面**当"显示开关"（20/40/60/80 分档决定点亮几个 ▊），空 `text` 槽占位，`%%9` 兜底收尾。与 CornyFlex 是同一套"槽位/下标"机制的应用。

## 5. 网易环境注意

- **机器人/AI 通道发的 tellraw 在网易服会被自动加 `execute run` 前缀**；定向消息用 `@a[name="玩家名"]` 选择器（普通通道在网易可能不执行）。
- `tellraw @a` 广播给全服；给单个玩家用 `tellraw @a[name="X"]`。
