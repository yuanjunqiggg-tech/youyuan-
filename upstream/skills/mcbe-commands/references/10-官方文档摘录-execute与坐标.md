# 十、官方文档摘录：execute 子命令与坐标

本页是外链的**内嵌摘录**，供离线查阅与交叉引用，避免每次都去联网核对。
内容取自 Bedrock-OSS/bedrock-wiki（`commands/new-execute.md`、`commands/execution-forking.md`、
`commands/relative-coordinates.md`）与 Microsoft Learn 命令文档。
**原文以此为准**；本页只做中文整理，关键句附英文原句以便比对。

> 用途：`09-粒子作图-分叉取样与大数圆规.md` 里凡说"见 10 号文档"处，依据都在这里。
> 若与服务器实际版本行为不符，**以实测为准**。

## 1. execute 各子命令语义（new-execute.md）

- 1.19.50 起 execute 改为"子命令链 + run"：`/execute` 接受**任意多个子命令**，
  按书写顺序改写命令上下文，末尾 `run` 触发。
- 上下文四要素：**执行者（executor）、位置、朝向（rotation）、维度**；锚点（anchor）另算。

| 子命令 | 改什么 | 关键原文 |
|---|---|---|
| `as <target>` | 执行者（`@s` 指向） | "does not change the position, rotation, or dimension context" |
| `at <target>` | 位置+朝向+维度 | "does not change the executor of the command, so `@s` will remain as whoever was targeted last" |
| `in <dimension>` | 维度（含缩放） | "going from the Overworld to The Nether will apply a scale of x0.125 ... vice versa x8" |
| `positioned <x y z>` | 位置 | 相对/局部坐标基于命令当前位置 |
| `positioned as <target>` | 位置（不动朝向/维度） | "only sets the command's position and not its rotation or dimension" |
| `align <axes>` | 坐标按轴 floor | "Aligning a position will floor it" |
| `anchored (eyes\|feet)` | 锚点，影响局部坐标起点 | 默认 feet |
| `rotated <yaw> <pitch>` | 朝向 | 默认 pitch/yaw 均为 0，除非之前改过 |
| `rotated as <target>` | 朝向=目标朝向 | 多目标则逐目标执行一次 |
| `facing <pos>` | 朝向某坐标 | 基于命令当前位置计算 |
| `facing entity <t> (eyes\|feet)` | 朝向某实体 | feet=站立处，eyes=眼位 |

**旧语法**：`/execute <target> <position> <command>`；等价新写法是 `as <target> at @s`。
官文原例：

```yaml
# Old:
/execute @p ~ ~1.62 ~ teleport @s ^ ^ ^3
# New:
/execute as @p at @s positioned ~ ~1.62 ~ run teleport @s ^ ^ ^3
```

**两个已知 bug（官文明确记载）**：

- `anchored` 在同一个 execute 里写第二次**不生效**（MCPE-165051）。
- `anchored eyes` 的偏移**错误地影响相对坐标**，本该只影响局部坐标（MCPE-162681）。

## 2. 执行分叉 / 幂次（execution-forking.md）

> "**Execution Forking** (also known as *Instancing* or *Nesting*) is a technique used to trigger a single
> command multiple times within the same tick."

- 用 `@a`/`@e` 命中多个目标 → 为**每个目标**复制一份命令实例。
- 嵌套 `as`/`at` 时按**乘法**增长，公式（官文原文）：

```
Instances = n^x
```

其中 n = 每层命中实体数，x = 嵌套层数。官文给的例子：

```yaml
execute as @e[c=2] as @e[c=2] as @e[c=2] run say hi   # 2^3 = 8 次
```

- 官文警告："Always use `c=` (count) or `type=` limits when forking. If you accidentally fork `@e`
  (all entities) without a limit, you can create hundreds of instances instantly, leading to significant
  server lag or a 'Command Overflow.'"
- 常见用途：射线防穿墙、批量实体更新、亚 tick 计时。

## 3. 相对坐标 `~` 与局部坐标 `^`（relative-coordinates.md）

- **相对坐标 `~`**：相对世界坐标；`~5 ~-2 ~`＝当前位置 X 偏移 5、Y 偏移 -2。`~90 ~`＝在当前 yaw 上加 90°。
- **局部坐标 `^`**：形式为 **`^left ^up ^forward`**（左/上/前），轴基于**朝向**。
  官文原句："you can think of this as `~x ~y ~z` if both your yaw and pitch rotations are 0 (facing straight ahead, due south)."
- 例：`^10 ^ ^`＝向左 10 格；`^ ^1.5 ^1`＝向上 1.5、向前 1 格。
- 若没有可复制朝向的实体，x / y 旋转视为 0。
- 相对坐标与局部坐标**不能混用**在同一个坐标里。

**与 09 案例的关系**：案例里大量 `positioned ^1 ^ ^`（左移 1）、`positioned ^ ^ ^1`（前进 1），
正是上表的"左/上/前"；而 `facing entity ... feet` 改朝向、`rotated as` 复制朝向，
都是官文列出的、被证实会**按多目标分叉**的子命令。

## 4. 玩家眼高

官文注：玩家眼位在脚上方 **1.62 格**（`~ ~1.62 ~`）。与 09 案例里的 `1.52` 接近但不相等
（案例的 `1.52` 是相对 `feet` 锚点的自定义高度，不是官方眼高常量）。

## 5. 我核对时发现的差异（诚实记录）

- 09 案例的"**用十万格折叠坐标**（大数圆规）"这一手法，**在本页四份官方文档里没有任何记载**。
  它是**由命令结构反推的推断**，不能当作官方机制。
- "2⁵ = 32"这类具体数字，官文只保证 **n^x 公式**；指数必须逐个**数选择器出现次数**，
  基数必须确认选择器**实际匹配数**。凭印象报数会错。
