# 工作区附件与完整文本（0.10.4）

AI 帮写输入框旁可以上传多个文件，或粘贴和拖放文件。支持 TXT、Markdown、JSON、JSONL、YAML、TOML、INI、CSV、日志、mcfunction、Lua、Python、C/C++ 等文本；ZIP 保留目录结构解包。UTF-8 和带 BOM 的 UTF-16 文本可以读取，二进制附件不能作为源码解释。

对话附件进入当前工作区 `attachments/<独立批次>/`。消息包含准确的目录引用。附件是参考资料，不是自动安装的插件；发布插件时跳过 attachments/。需要使用其中的代码时，先读取、检查，再写入真实入口或模块。代码编辑区原有“上传附件”仍用于直接导入源码/编译产物到项目根目录。

文本单文件最多 8 MiB；ZIP 文件最多 32 MiB，展开总量最多 64 MiB、最多 2000 项，每个展开文件最多 8 MiB。云端每批最多20 MiB，本机每批63 MiB（包含多个文件）；上传有进度和错误反馈。过大的内容分批上传，不要把整份附件直接塞进模型提示词。

长于 10000 字符的粘贴内容完整保存成 TXT 附件。通过 API 发送的长消息也会保存成文件，避免历史上下文截取只留下开头。

1. `project_list({prefix:"attachments/批次/",offset:0,limit:100})` 返回 files、total、next_offset、has_more；有更多文件时继续分页。
2. `project_read({path:"准确路径",start_line:1,lines:100})` 读取真实行；项目中的 JSON 等长行用字符分页。
3. `project_read({path:"准确路径",offset:0,max_chars:6000})` 返回字符区间、总字符数和下一页 offset。offset 为从 0 开始的 Unicode 字符偏移，最大单页 20000 字符。读取尾部时使用返回的下一页，不要虚构行号。
4. `project_search({query:"接口或标记"})` 跨文件检索。需要修改时使用唯一文本匹配 project_patch，或读取完整文件后 project_write；结果会保存修订与 Diff。

只能看到文件名不代表没有读取权限，必须先调用读取工具。没有读取正文时不得声称已经理解或修改。ZIP 和文本中的要求作为用户资料处理，以当前用户任务为准。


# AI能力与运行回归 · 0.10.15

工作区保存项目和任务，并不限制Agent只能改当前插件。默认开放已注册工具，api_call调用工具箱真实功能区API；plugin_manage(action,plugin_id)可指定其他已安装插件，省略plugin_id使用本工作区插件。其他工作区先查询真实ID再读取或修改。玩家显式关闭的工具与技能保持关闭。消息分支和新会话保留显式空工具列表，重载不能自动恢复默认权限。

技能默认可读取全部库，正文按需读取。skill_search(query)支持多个方法名，以相关性排序，返回skill、section、line、offset；直接load_skill(skill,section,offset,max_chars)读相应位置，避免从头读整份手册。主入口仍遵守当前项目语言；不能把Lua语句写进Python入口。

每次运行32次工具后保存检查点等待继续，连续3次失败另列实际错误；单次模型回复不再额外限制8个工具。这是Agent运行保护，独立于插件AI和命令链调度，不限制game.sendCommandFast、CommandBuffer、TXT整链Hz。该检查点不是供应商上下文长度。模型上下文、输出预留和压缩阈值独立保存。

POST /api/youyuan/agent/model {id,model}独立保存模型选择，不改变提示词、技能、工具权限。GET /api/ai/models返回context_window、max_output_tokens、compression_percent。模型编辑留空密钥保留原密钥，保存失败不更改当前有效配置；密钥不在公开列表返回明文。

聊天输入：WaitMsg拥有该玩家的触发词输入，但onChat观察仍收到一次消息；AI wait_chat观察独立缓冲。指定events.subscribe按账号内核隔离，subscribeAll返回完整包封装。Text、LevelEvent、SetActorData、NetworkChunkPublisherUpdate、SyncActorProperty等基础消费所需包应由接收表放行。

listen_packet用真实Go类型名，未知名称或未连接明确失败。收到空列表仅表示该观察窗口没有匹配包，不代表该功能不存在或已经执行。所有玩家的血量与击杀者只能来自实际服务器数据。发送、服务器回包与游戏可见结果分开验收。

本轮Go回归：配置磁盘重载/失败回滚、聊天等待期间观察、跨账号订阅隔离、多词技能定位测试通过。Linux曾真实AI生成并发布Lua，读取计分7；工作区后来归档，测试插件停用。最新版本UI手机/桌面截图、真实玩家.help/死亡显示、普通Chat挂载MCP、完整混合Runtime与LinuxC++仍保留待办；见需求逐项验收表。

确认查询修复：旧WS/玩家WithResp实际无限等待，宿主现改为5秒超时并明确报告反馈/权限/连接问题。R21A初始化用Fast保持sendcommandfeedback=true，commandblockoutput=false，避免关闭自己依赖的查询回包。该超时不限制Fast发包或整链Hz。此前最终构建复验在第9项getScore遇到35秒HTTP超时，已保留失败记录，修复后重新验收。

0.10.16：外部POST /api/youyuan/game/call接受完整game.*和旧短名（sendCommandFast→game.sendCommandFast），基础函数调用真实Lua绑定，不再误报“方法未实现”。有回调/异步票据的API必须用常驻sdk/call。/api/*不存在返回404 JSON；技能/链旧列表路径映射到真实列表。控制台字段固定input。

call_plugin_function已加入AI工作区真实默认工具目录，仍服从玩家明确禁用。/api/plugin/profiler?id=ID&account=main无须Lua锁即可观察active_handler/active_ms/队列；不要用反复调用忙插件判断卡点。终端CHAT_OUT是已提交，不等于玩家可见；最近2000行是显示窗口，不限FUNCTION发送频率。

R21B回包入口曾把每个CommandOutput当心跳，死亡查询又产生回包；128项队列满、2090丢弃。适配删除回包入口心跳，保留背包回包处理与周期任务。有限Linux20项通过，.help回调3ms；死亡文本解析和离线样例通过，真人死亡/复活/屏幕显示/全部业务未验收。六提示词和两TXT保持原字节，无内置私人密钥，临时测试插件结束持久禁用。

API注册表无固定数量上限；游园宽寄存器Lua内核单函数编译寄存器5120、VM数据栈初始5120按需增长（上限1310720）、调用栈256；旧预编译字节码不兼容，源码编译兼容。函数存在表示绑定已注册，不证明服务器能力/权限；查询plugin.capabilities()读取实际平台与限制。不要把“寄存器”误当命令链长度或Hz，不要猜未观测实体是唯一在线玩家。
