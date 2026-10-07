# AI能力与运行回归 · 0.10.15

工作区保存项目和任务，并不限制Agent只能改当前插件。默认开放已注册工具，api_call调用工具箱真实功能区API；plugin_manage(action,plugin_id)可指定其他已安装插件，省略plugin_id使用本工作区插件。其他工作区先查询真实ID再读取或修改。玩家显式关闭的工具与技能保持关闭。消息分支和新会话保留显式空工具列表，重载不能自动恢复默认权限。

技能默认可读取全部库，正文按需读取。skill_search(query)支持多个方法名，以相关性排序，返回skill、section、line、offset；直接load_skill(skill,section,offset,max_chars)读相应位置，避免从头读整份手册。主入口仍遵守当前项目语言；不能把Lua语句写进Python入口。

每次运行32次工具后保存检查点等待继续，连续3次失败另列实际错误；单次模型回复不再额外限制8个工具。这是Agent运行保护，独立于插件AI和命令链调度，不限制game.sendCommandFast、CommandBuffer、TXT整链Hz。该检查点不是供应商上下文长度。模型上下文、输出预留和压缩阈值独立保存。

POST /api/youyuan/agent/model {id,model}独立保存模型选择，不改变提示词、技能、工具权限。GET /api/ai/models返回context_window、max_output_tokens、compression_percent。模型编辑留空密钥保留原密钥，保存失败不更改当前有效配置；密钥不在公开列表返回明文。

聊天输入：WaitMsg拥有该玩家的触发词输入，但onChat观察仍收到一次消息；AI wait_chat观察独立缓冲。指定events.subscribe按账号内核隔离，subscribeAll返回完整包封装。Text、LevelEvent、SetActorData、NetworkChunkPublisherUpdate、SyncActorProperty等基础消费所需包应由接收表放行。

listen_packet用真实Go类型名，未知名称或未连接明确失败。收到空列表仅表示该观察窗口没有匹配包，不代表该功能不存在或已经执行。所有玩家的血量与击杀者只能来自实际服务器数据。发送、服务器回包与游戏可见结果分开验收。

本轮Go回归：配置磁盘重载/失败回滚、聊天等待期间观察、跨账号订阅隔离、多词技能定位测试通过。Linux曾真实AI生成并发布Lua，读取计分7；工作区后来归档，测试插件停用。最新版本UI手机/桌面截图、真实玩家.help/死亡显示、普通Chat挂载MCP、完整混合Runtime与LinuxC++仍保留待办；见需求逐项验收表。

确认查询修复：旧WS/玩家WithResp实际无限等待，宿主现改为5秒超时并明确报告反馈/权限/连接问题。R21A初始化用Fast保持sendcommandfeedback=true，commandblockoutput=false，避免关闭自己依赖的查询回包。该超时不限制Fast发包或整链Hz。此前最终构建复验在第9项getScore遇到35秒HTTP超时，已保留失败记录，修复后重新验收。
