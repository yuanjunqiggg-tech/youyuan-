# 三维度常驻观察与服务端相对镜头

全局抓包默认auto_players=true，不必逐人添加。默认dimension_routing=true；dimension_accounts={"0":"自己账号ID","1":"自己账号ID","2":"自己账号ID"}配置主世界/下界/末地，三个ID必须不同且在accounts中。不指定映射时按accounts顺序分配0/1/2，账号不足明确缺少维度，不自动宣称覆盖。

observerBridge.assignDimension({dimension=0})：只控制当前机器人。需OP，关闭其auto_creative并按实际game_mode=6维护旁观；身体维度不匹配才正常TP到该维度安全锚点20/120/20。空间导入导出作业暂停角色维护。ready来自实际维度与模式，submitted只代表请求。调用不会给其他玩家授予OP；权限要由服主先正常授予。
observerBridge.dimensionStatus()：真实连接、维度、game_mode、is_op。

watchStart参数新增observe_dimension=0/1/2、camera_relative=true、camera_follow_rotation=true、camera_back_blocks=5。镜头使用execute as观察机器人 at @s at @a[name="目标",rm=0] rotated as @a[...] run camera @s set minecraft:free pos ^ ^4 ^-5 rot 90 ~。rm=0按本服务器实测只匹配当前执行维度；外维度不会改变观察机器人的镜头。依旧需要周期执行；camera set/ease不是自动实体附着。身体维度不符时观察等待分工恢复，不跨维度伪造包。

Lua：observerBridge.assignDimension({dimension=1})；Python：host.observerBridge.assignDimension({"dimension":1})；C++：client.observerBridge.assignDimension(yy::json{{"dimension",1}})。三语言共享同一宿主语义。global配置API=/api/youyuan/global-capture，当前用户只选自己的账号。

2026-10-14测试说明：最初三维度10/10与单机46/60记录用了实体缓存判断维度；复验发现缓存与连接层维度冲突，因此这些维度归属结果撤销验收，原始记录保留。严格复验固定接收者在下界、无OP发送者在主世界，原生同维度半径选择器返回falseCondition；两个开镜头窗口均收到目标挥臂0/10，两个关闭窗口也是0/10。摄像机指令成功不能证明跨维度收包。默认保留三机器人分工，尚不承诺任意真人、全服零丢包或单机快速切维度替代。角色常驻与同维度动作复验另见交付报告。

0.10.56：低频角色维护用服务器确认命令，随后读取连接层真实维度和模式；高频镜头仍只提交不等待回包。bot.self的自身维度改读连接层Spawn/ChangeDimension，避免实体缓存误报；其他实体继续保留实际收到的数据。不匹配的维度不能显示ready。
