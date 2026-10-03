# 技能：网易基岩版指令系统

在网易《我的世界》中国版基岩版（租赁服）里设计、编写、调试命令系统时使用。

## 何时用

- 要给租赁服写菜单、结算、战斗、成就、传送、地皮等指令逻辑
- 读不懂别人的 `.mcfunction`（尤其是旧语法 `execute @a ~~~`）
- 要发富文本/进度条/血条（rawtext / tellraw / titleraw）
- 怀疑踩了基岩版坑（选择器、分位置、rawtext 不显示、卡顿）
- 需要区分"网易特性"和"微软原版基岩版特性"

## 三条铁律

1. **两代 execute 都要认**。网易长期支持旧语法 `execute <目标> <坐标> <指令>`，新语法 `execute as ... run` 也支持。读存量代码先判断是哪代，别按一代去套。
2. **rawtext 是唯一富文本入口**，机制核心是"槽位 + 下标"。`%%N` 是**绝对下标**，取不到值的槽**不显示**；所以"空槽"才能被用来做条件显示。
3. **不确定就用分支法**，不要照抄嵌套 `%%N`。见 `references/02-rawtext.md` 的"未证实"一节——素材里那套嵌套写法能跑，但机制官方无记载，不可推广。

## 参考索引（按需读取）

| 文件 | 内容 |
|---|---|
| `references/01-execute.md` | 新旧两代语法、11 个子命令、上下文继承表、执行分叉 |
| `references/02-rawtext.md` | 组件类型、`%%` 语法、CornyFlex 范式、未证实写法与替代 |
| `references/03-菜单与状态机.md` | 雪球菜单、状态机、冷却、手持检测、tag 门控 |
| `references/04-实体与战斗.md` | 船当子弹、命中检测、damage 结算、血条、弹道 |
| `references/05-坐标与地皮.md` | 坐标存储、全局 ID、二进制位存储、地皮螺旋分配 |
| `references/06-网易扩展与差异.md` | 自定义指令、魔法指令、ModSDK、版本差异、权威来源 |
| `references/07-性能与调试.md` | 每刻预算、分叉爆炸、扫地机、调试手段、排查顺序 |
| `references/08-速查表.md` | 选择器/记分板/rawtext/execute/命令模板速查 |
| `references/09-粒子作图-分叉取样与大数圆规.md` | 实战拆解：记分板蓄力 + 辅助盔甲架分叉取样 + "大数圆规"坐标变换做粒子作图 |
| `references/10-官方文档摘录-execute与坐标.md` | 内嵌权威摘录：execute 子命令语义、分叉 n^x 公式、局部坐标 ^left^up^forward |

## 快速判断：新老语法对照

| 老语法 | 新语法等价 |
|---|---|
| `execute @a ~~~ say hi` | `execute as @a at @s run say hi` |
| `execute @e[type=zombie] ~~~ execute @p ~~~ kill @s` | `execute as @e[type=zombie] at @s as @p run kill @s` |

## 全局易错（先扫一遍再写）

- `as@a`、`@s{`、`actionbar{` 前**缺空格** → 整条解析失败。
- `at` **不换执行者**：`execute at @e[type=sheep] run kill @s` 杀的是自己。
- 分数**默认 0**；状态机的初始值必须显式 `set`。
- `@e` 不带 `c=`/`type=` 放进嵌套 `as` 会指数级分叉。
- 一个 `execute` 里 `anchored` 写两次只生效一次（基岩版 bug）。
- 网易服定向 tellraw 用 `@a[name="玩家"]`；机器人通道发的会被加 `execute run` 前缀。


## 游园宿主适配说明

此技能是离线指令参考，不提供新的服务器权限或游戏接口。示例的执行者、位置、维度以 game.execute / game.executeBatch 所选通道及实际回执为准。魔法通道不是命令方块；stream 批量发送不保证同 Tick。计分板未初始化的玩家没有分数，不能假设初值为 0。读取旧项目先确认 execute 语法版本，再验证一条小范围命令；FMbe 动画需要相应资源。新增功能必须通过当前服务器实际回包验证。

章节直接用 load_skill(section,offset,max_chars) 读取，不需要联网查找文档。玩家在工作区技能库中加载后才允许 AI 读取，卸载仅影响后续读取，不会抹去已经出现的历史工具结果。
