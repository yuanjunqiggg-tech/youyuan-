# 游戏协议与玩法技能

游园当前内核实际注册 **215种** 协议包。包名、ID、字段和嵌套类型都可以离线查；保留原版game.sendPacket/events.subscribe/events.subscribeAll并提供packet别名。不是把网上1.26的ID直接填到网易内核。

## 按玩法找包

| 想做的玩法 | 原始包 | 高级入口 | 不能从它单独知道的东西 |
|---|---|---|---|
| 挥臂技能/挥空菜单 | Animate | events.onSwing | 命中目标、击杀者 |
| 实体/BOSS/掉落物出现 | AddActor/AddItemActor/AddPlayer | events.onEntitySpawn | 全世界实体、未同步的血量 |
| 拾取奖励 | TakeItemActor | events.onItemPickup | 未观测掉落物的完整NBT |
| 血量、受伤、死亡 | UpdateAttributes/SetHealth/ActorEvent/DeathInfo | player.health、events.onHealthChange/onHurt/onDeath | 服务器没传的其他玩家血量、准确击杀者 |
| 速度、方向、加速度 | MovePlayer/MoveActorAbsolute/Delta/SetActorMotion | player.position、events.onMove | 未观测位置、服务端权威原因 |
| 持有/穿戴特殊物品 | MobEquipment/MobArmourEquipment | events.subscribe | 所有玩家全部背包 |
| 自己的箱子菜单与槽位 | ContainerOpen/InventoryContent/InventorySlot/Close | events.subscribe | 他人私有容器、伪造交易 |
| 使用物品、挖掘、攻击 | PlayerAuthInput/PlayerAction/InventoryTransaction | world.mineBlock/useBlock、bot.attack | 他人的所有客户端请求 |
| 方块变化、区块与结构 | UpdateBlock/LevelChunk/SubChunk/Structure* | world.blockState/getBlockTile、结构工具 | 确认破坏者、已实现全世界缓存 |
| 命令、权限与计分板 | AvailableCommands/CommandRequest/Output/SetScore | game.execute/sendCommand/getScore | 绕过权限、任意长链同刻原子性 |
| 网易特殊事件 | PyRpc/NeteaseJson/SyncSkin | packet.on/describe/send | 未知事件结构、BDS服务端全权限 |

## 三种语言监听同一个包

```lua
function arm(data)
  if data.action_type == 1 then print("挥臂实体", data.entity_runtime_id) end
end
events.subscribe("Animate", "arm")
```

```python
def arm(data):
    if data.get("action_type") == 1:
        print("挥臂实体", data["entity_runtime_id"])
def on_init(host):
    host.packet.on("Animate", arm)
```

```cpp
// 放在yy::Plugin派生类构造函数中
on("arm", [](const yy::json& args) {
  const auto& data=args.at(0);
  if(data.value("action_type",0)==1) return data.at("entity_runtime_id");
  return yy::json(nullptr);
});
client.packet.on("Animate", "arm");
```

## 查询与发包闭环

先packet.describe("RequestChunkRadius")查询字段，然后packet.send("RequestChunkRadius", {chunk_radius:4,max_chunk_radius:8})请求；订阅ChunkRadiusUpdated查看服务器实际允许范围。数字ID可用，但以当前schema为准。Lua字典用{chunk_radius=4}，Python用{"chunk_radius":4}，C++用yy::json{{"chunk_radius",4}}。payload_hex只包含完整包体，不加header、压缩或加密层；不能与结构字段混合。

支持嵌套struct、数组/向量、map、数字、布尔、string、hex字节/UUID、bitset索引数组。未知字段、溢出、错误类型提前报错；InventoryTransaction等interface联合分支优先高级API，不能把任意字典当合法事务。生命周期握手由连接系统负责。submitted/confirmed=false不是服务器执行成功。监听自己出站请求也不能伪装成旁观收到其他玩家输入。

## 接成原来的命令链

事件回调可以chains.start("技能.txt")启动预编译链，或game.sendCommandsFast(commands)按序提交预加载指令。不要在每次包回调重新读TXT、写JSON、创建VM。需要变量参与时只生成必要命令；同一事件可只选一条链。有条件命令按真实CommandOutput做Barrier；没有结果依赖就保持原sendCommand连续发送方式。20Hz是整轮目标，每轮含所有节点。

## 速度与死亡的可信度

位置样本差分：速度=(新坐标-旧坐标)/真实采样秒；水平速度=sqrt(vx²+vz²)，垂直速度=vy。瞬移、维度变化、离视距、长时间无同步时重置采样；位置噪声不能直接判挂。实体消失不是死亡；受伤+附近玩家也不足以确认击杀者。网易ModEventS2C/OnPlayerDie等必须先用真实包验证事件结构再封装，不把其他框架的事件名直接塞给服务器。

技能章节：index.md为玩法说明；catalog.md为全部包；可用section="Animate"精确读取单包，不要把215包全部加载到简单任务上下文。
