# 机器人动作与服务器确认

此技能用于 Lua/Python/C++ 插件和 AI 的 game_call。公网 MCP 同名方法可经 `/api/youyuan/game/call` 调用。参数按顺序给数组；game_call.function 使用完整名，例如 bot.attack，而不是猜测函数。

| 方法 | 参数 | 原理 |
|---|---|---|
| bot.self / game.scene | 无 | 机器人/已观测实体的位置、runtime_id、血量与 known/stale |
| bot.startMovement | dx,dy,dz（各 -1..1） | PlayerAuthInput 玩家输入；dz=-1向前，不是世界目标坐标 |
| bot.stopMovement | 无 | 释放移动输入 |
| bot.lookAt | x,y,z | 用观测位置计算视角输入 |
| bot.teleport | x,y,z | tp 指令，需权限，等待命令结果 |
| world.placeBlock | x,y,z,block | setblock 指令，需权限 |
| world.breakBlock | x,y,z,mode="command" | command=setblock air destroy；creative_action=创造破坏动作（需创造/六格） |
| bot.attack | runtime_id 字符串,slot=0 | 真实挥手动画与 UseItemOnEntityActionAttack 库存交易 |
| world.mineBlock | x,y,z,duration_ms=1000,slot=0,face=1 | 真实开始、持续、停止破坏与 UseItemActionBreakBlock 交易，100ms节流 |
| world.useBlock | x,y,z,face=1,slot=0 | 真实手持物品点击方块，可能放置/开容器；取决于手中物品与目标 |
| world.blockState | x,y,z | 最近服务器 UpdateBlock；未观测不能假定空气 |

slot 从0至8，face=0下/1上/2北/3南/4西/5东。近战目标必须已观测、仍存在、同维度、六格内，不攻击 self。点击和挖掘也限六格，需要已观测目标 runtime_id，库存必须同步；未知则报错。每账号物理动作串行，至少100ms间隔，挖掘时长100..30000ms。手持物品、硬度、权限、距离和反作弊最终由服务器判断，不自动改创造、不通过 setblock 冒充生存挖掘。world.useBlock 使用当前库存物品，不能通过 block 参数凭空生成物品。

交易方法返回 submitted=true/confirmed=false。这只证明已写入连接；只有服务器 UpdateBlock、目标 ActorEventHurt、health下降等才证明实际效果。攻击事件缺失时不能推测成功；hurt 和血量下降可能分别到达。用 events.onBlockChange/onHurt/onHealthChange 或过滤抓包读取。位置必须比较服务器后续观测，不把本地移动输入当实际移动。

停止插件时清理自己的计时器和订阅；避免在每个Tick同时执行攻击、抓包和场景全量读取。世界操作前保存/确认测试区域，测试后恢复。生存挖掘时间由插件自行根据手持工具与目标估计；当前没有自动工具选择、路径规划或全部网易协议版本保证。验收记录列出当前服务器实测效果，不能把接口存在说成服务器已接受。

## 0.9.7 实服状态

测试租赁服已确认 setblock 放置、命令破坏与 tp。创造动作破坏和普通挖掘交易已提交但未确认世界变化，不应自动宣称成功。持续移动需用 querytarget 等真实服务器结果核对，单次 StartGame/MovePlayer 观测不能证明已完成寻路。手持放置已实测：空快捷栏8放入石头，world.useBlock 点击已观测石头的上表面，testforblock 确认相邻位置生成石头；测试物品和方块已恢复。它调用工具箱现有 BotClick.PlaceBlock 与同步函数。普通攻击尚未完成成功验收，一次联调发生连接断开，原因未确定；不自动重发。玩家位置是协议原始观测，不同包可能是脚部或眼部参考；将 MovePlayer 原始 Y 直接用于 tp 会产生约一个身高的偏移，操作前可通过 querytarget 确定脚部坐标。AI 应明确标记待确认，收到真实结果再报告。
