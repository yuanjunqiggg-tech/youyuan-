# 图片/视频到方块命令链

打开工作台“全部工具→视频方块化”。选择本机图片/视频、两个含端点的坐标、XY/YZ墙面或XZ地面、抽帧间隔和播放轮数。前端在用户设备解码，缩放后逐帧发给 APK；每次最多一个未完成请求，不上传整段视频到第三方。最多两个转换任务。

固定轴须相等。XY水平沿X，上方为最大Y；YZ水平沿Z，上方为最大Y；XZ从最小X、Z递增。16色混凝土调色板；相邻横向同色合并 fill，单个像素用 setblock；下一帧仅写变化方块。同色多行仍各用一条fill，不宣称全局最小矩形划分。

文件自动保存至命令链目录并预解析，默认不启动。同帧命令默认无额外逐条延迟，仅在帧边界等待；帧间隔只表示期望，发送数量和真实命令响应决定实际速度，不能承诺原视频帧率。运行次数为整段视频的次数，0持续。运行过程中随时用 chains.stop；每轮第一帧完整重绘，其后的帧增量更新。

API：POST `/api/youyuan/media/block-video`。
1. `{action:"create",config:{name:"视频.txt",plane:"xy",a:[0,80,0],b:[15,95,0],frame_ms:1000,channel:"ai",run_count:1}}` → id,width,height。
2. `{action:"append",id,sequence:0,image:"data:image/png;base64,..."}`。前端先缩放至上述尺寸。逐帧递增sequence，不并发、不自动重发；返回 frames/commands/changed_blocks/frame_commands/minimum_frame_ms/bytes。
3. `{action:"finish",id}` → 自动导入命令链，尚未启动。
4. `{action:"cancel",id}` → 删除临时转换文件；半小时未活动任务下次创建时清理。

单边512、单帧65536方块、6000帧、32768输出命令或8MiB TXT为当前保护界限。达到上限可先完成已转换帧，或者缩短片段；不是1000方块限制。程序不会为了更高画质耗尽设备内存。

“全部工具→结构与指令转换”提供 MCStructure/BDX/Schem/Schematic → TXT/JSON/MCStructure，以及直接指令/实体计分板包装。结构转换当前最多131072方块；大建筑走建筑管理。转换接口 `/api/youyuan/convert/structure` 使用 base64 data,filename,target,origin；`/api/youyuan/convert/commands` 使用 content,mode=direct/scoreboard,objective,entity,start。计分板包装不自动创建实体或推进分数。

TXT只保留方块指令，不能保留箱子物品NBT。MCStructure转换不还原游走实体和第二水层；不要覆盖唯一原件。ZIP或Java资源包不是这些输入格式。

音频：普通 Bedrock 服务器的 playsound 播放客户端已拥有的音效，不能把任意MP3人声作为参数直接广播。视频原音需要相应客户端/资源包，音符转换属于近似音乐。当前方块视频功能不声称B站拉流、任意原音或实时高帧播放已验收。

## 0.9.8 转换选项
结构输入新增 JSON：{format:"youyuan-blocks-v1",size:[x,y,z],blocks:[{x:0,y:0,z:0,name:"minecraft:stone",states:"[]",nbt:null}]}，也兼容旧版 blocks 数组。整数局部坐标非负且位于 size 内，区域体积不超过131072。输出 JSON 包含尺寸，支持再次导入。MCStructure 保留方块实体 NBT，不保留游走实体、第二水层。
结构 API target=inspect 返回尺寸、非空气 blocks、palette、block_entities。rotation=0/90/180/270 使用内核 Y 轴旋转（含支持的朝向状态）。target=txt 时 origin 为世界原点；relative=true 用 ~相对偏移，merge=true 合并同色 X 横行为 fill（最大32768格，带NBT不合并），includeAir=true 写空气以清空结构空位，默认关闭。生成不执行；界面预览并下载，或导入未启动的独立命令链。
指令 API mode=direct/execute/scoreboard。execute 需要 selector（例如 @a 或 @e[tag=builder]），生成 execute as selector at @s run。scoreboard 需要 objective/entity/start，需要另行创建计分板、实体并推进分数。包装模式只允许纯指令，不能包装链状态行。output=txt 保留链语法，output=mcfunction 拒绝链属性、延迟、分链与周期标记，去掉指令开头的 /。生成结果返回 content/commands，界面可复制、导出、预解析、导入命令链，导入默认 disabled，不覆盖同名文件。

JSON 方块实体另外含 nbt_base64，重新导入优先采用它以保留NBT数字类型。nbt 是阅读用投影；只修改 nbt 不会覆盖 nbt_base64。只编辑方块名、状态与坐标不影响该方块实体原始NBT。


## 0.10.58 视频帧边界
`@frame {"duration_ms":100}`是宿主控制行：等待上一帧开始之后剩余的时间，耗时已经超过目标则不额外补等、不积压旧帧；不向服务器发testfor。video生成TXT默认mode=fast、scheduling=file、delay_ms=0；同帧fill/setblock连续提交，只在帧边界等待。多帧保持原文顺序，不能设置segments并行播放不同视频帧。2D同色矩形合并与变化帧跳过保留每个像素，单fill体积不超过32768。颜色使用明确16色混凝土调色板和OKLab最近色，24位颜色缓存避免旧15位键碰撞；比例可选保留/裁切/拉伸。它不能还原原版不具备的颜色，也不承诺服务端按请求帧率完成。
