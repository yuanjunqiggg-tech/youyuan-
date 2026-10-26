# 一、execute 指令：两代语法与执行模型

网易《我的世界》基岩版**同时支持两代 execute 语法**，写之前先确认目标服务器用的是哪一代（或混用）。存量租赁服大量是旧语法，新做的服务器多用新语法。

## 1. 旧版 execute（网易长期支持）

格式：`execute <目标选择器> <坐标> <指令...>`

```mcfunction
# 对所有玩家：把它自己加到"攻击"标签
execute @a ~~~ scoreboard players add @s 子弹 1

# 对每个苦力怕附近的玩家执行
execute @e[type=creeper] ~~~ execute @p ~~~ say hi

# 带条件：目标必须分数为 5
execute @a[scores={State=5}] ~ ~ ~ titleraw @s actionbar {"rawtext":[{"text":"菜单"}]}
```

要点：
- 目标选择器**当成执行者**（`@s` 变成它），坐标是**相对执行者的偏移**。
- 没有 `run`；指令直接跟在坐标后面。
- `execute @e[...] ~ ~ ~ execute @p ~ ~ ~ <cmd>` 这种**双层嵌套**很常见：外层遍历实体，内层定位到对应玩家。
- 旧语法的 `~ ~ ~` 是相对**外层目标**的坐标，不是世界坐标。

## 2. 新版 execute（子命令链 + run）

格式：`execute <子命令…> run <指令>`

网易已在较新版本支持新版；跨版本写法以新语法为准，但**必须知道旧语法的存在**，否则读不懂存量代码、也无法在只支持旧语法的环境里落笔。

### 完整子命令（基岩版共 11 个，Java 有 14 个）

```
execute ...
  align <axes>                              # 坐标按轴向下取整（floor）
  anchored <eyes|feet>                      # 锚点，默认 feet
  as <目标>                                  # 换执行者（改 @s 指向）
  at <目标>                                  # 换位置+朝向+维度为目标
  facing <x y z>                            # 朝向某坐标
  facing entity <目标> <eyes|feet>           # 朝向某实体
  in <维度>                                  # 切换维度（overworld/nether/the_end）
  positioned <x y z>                        # 换固定坐标
  positioned as <目标>                       # 换位置为目标位置（不动朝向/维度）
  rotated <yaw> <pitch>                     # 换朝向
  rotated as <目标>                          # 换朝向为目标朝向
  if|unless block <x y z> <方块> [状态]        # 方块检测
  if|unless blocks <起点> <终点> <对比点> <all|masked>   # 区域方块比对
  if|unless entity <目标>                     # 实体存在检测
  if|unless score <目标> <记分项> <操作符> <源目标> <源记分项>   # 两分数比较
  if|unless score <目标> <记分项> matches <范围>               # 分数落在范围
  run <指令>
```

基岩版**没有** `store`（Java 专有）、没有 `on`、没有 `positioned over`、`if` 条件只有 4 种（block/blocks/entity/score）。

### 执行上下文（execution context）继承规则 —— 最容易搞错

上下文四项：**执行者、位置、朝向、维度**，外加锚点。子命令按**书写顺序**逐条改写：

| 子命令 | 改变什么 | 不动什么 |
|---|---|---|
| `as <t>` | 执行者（`@s` 指向它） | 位置、朝向、维度、锚点 |
| `at <t>` | 位置+朝向+维度 | 执行者（`@s` 不变！） |
| `positioned <x y z>` | 位置（锚点复位为 feet） | 朝向、维度、执行者 |
| `positioned as <t>` | 位置 | 朝向、维度、执行者 |
| `rotated as <t>` / `rotated <yaw> <pitch>` | 朝向 | 其余 |
| `facing <pos>` / `facing entity <t>` | 朝向 | 其余 |
| `in <维度>` | 维度+位置（**尊重维度缩放**：主世界↔下界 X/Z 除以或乘以 8） | 执行者、朝向 |
| `align <axes>` | 位置按轴 floor | 其余 |
| `anchored <eyes/feet>` | 锚点（影响 `^ ^ ^` 起点） | 其余 |

经典等价：旧 `execute <t> <pos> <cmd>` ≈ 新 `execute as <t> at @s run <cmd>`。

**踩坑点**：`at` 不换执行者。`execute at @e[type=sheep] run kill @s` 杀的是**执行者自己**，不是羊。要杀羊得 `execute as @e[type=sheep] run kill @s`。

## 3. 执行分叉（forking）—— 强大但危险

`as`/`at` 命中 N 个实体就**复制 N 条执行实例**；链式嵌套则 N 的幂次，全部在同一 tick 内执行。

```mcfunction
execute as @e[c=2] as @e[c=2] as @e[c=2] run say hi   # 一个 tick 说 8 次
```

- 基岩版分叉是**深度优先**（Java 是广度优先），行为有差异。
- 用途：射线/子弹防穿透、批量实体更新、亚 tick 计时。
- **必须带 `c=` / `type=` 限制**，否则 `@e` 全实体分叉会瞬间刷爆服务器。

## 4. 高级用法清单

```mcfunction
# 嵌套 execute：子弹命中判定（船当子弹，撞到非子弹实体即命中）
execute as @e[scores={射程=5..},type=boat,tag=子弹] at @s positioned ~~-300 ~ if entity @e[type=!boat,c=1,r=1.5] run tag @s add 击中

# 位置校准到方块中心
execute as <t> at @s align xyz run tp @s ~0.5 ~0.5 ~0.5

# 切维度保留坐标
execute in nether positioned as @s run tp ~ ~ ~

# 分数范围判定
execute as @a if score @s t matches 5.. run <cmd>

# 用 tag 做一次性触发（替代已废弃的 testfor）
execute as @a[hasitem={item=diamond},tag=!av17] run tellraw @a {"rawtext":[{"text":"拿到钻石"}]}
tag @a[hasitem={item=diamond},tag=!av17] add av17
```

## 5. 旧语法里也常见的失败写法（照抄会踩坑）

- `execute as@a` / `execute as@a[...]` —— `as` 与选择器之间**缺空格**会解析失败（素材里有这种笔误）。
- `titleraw @s{"rawtext":…}` —— 缺空格，`tellraw @s{"rawtext"` 也会挂。
- 单个 execute 里 `anchored` 写两次只生效一次（基岩版已知 bug）。
