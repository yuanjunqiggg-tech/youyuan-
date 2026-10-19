# MCP联调约定与真实状态 · 0.10.57

宿主提供连接、原始包、账号进程、文件、回调队列和重载；插件实现血条/HUD、业务与跟随策略。API存在不是服务器全局广播保证。摄像机、OP也不保证跨维度目标包。三个常驻旁观按实际连接维度0/1/2分工；有受控下界远距无OP来源20/20记录，不承诺任意真人零丢包。

## 调用格式
MCP工具顶层account为空/main是主号，独立账号用账号池id。method是HTTP方法，query是GET查询参数，body是POST原始对象；游戏方法放body.method，参数放body.args。
例：api_call({path:"/api/youyuan/game/call",method:"POST",account:"main",body:{method:"bot.self",args:[]}})。
game.execute / sendCommandWithResp是确认通道；sendCommandFast只发不等，不代表执行成功。高频FUNCTION不要等待逐条确认；有真实成功依赖的条件才设置确认边界。

## 摘要与原始事件
MCP GET sense/snapshot、sense/events、packets/capture默认view=summary，返回bot简要位置/血量、计数和最近20条元数据。query支持limit、types逗号列表、runtime_id、after_sequence；规范事件还支持entity_type。过滤不创造未收到数据；counts来自保留窗口。完整内容明确query.view=raw；大文件用packets/export下载，不放AI上下文。
Lua events.summary({types={"swing","health"},limit=20})；Python host.events.summary({"types":["swing","health"],"limit":20})；C++ client.events.summary(yy::json{{"types",{"swing","health"}},{"limit",20}})。抓包摘要用events.captureSummary同样参数。
packet.catalog()列所有注册包ID/名称/字段数/是否能抓包。每个普通已解码包可用events.subscribe("包名",handler)；自定义转换events.define({name="my_event",packet="Animate",fields={target="entity_runtime_id"}},handler)。登录/握手隐私数据不进入抓包；大区块包不是小动作事件。UpdateAttributes是真实health来源；Animate只代表实际动作包，不用伤害伪造空挥；ActorEvent不保证带可靠攻击者。所有字段用packet.schema或/api/youyuan/packets/schema?name=Animate按需读取。

## trace与失败恢复
每次MCP工具调用structuredContent.mcp_meta含trace_id、http_status、elapsed_ms、outcome、retryable、execution_confirmed、auto_retried=false。公网中继同trace ID跟随到宿主；GET /api/youyuan/mcp/traces?trace_id=...读取最近元数据。保留256条内存记录，只含本人/管理员可读范围，不存密钥、命令正文、源码或聊天提示词。
not_sent表示中继未提交，可重试；timeout_unknown/write_result_unknown表示写操作可能已执行，必须读任务/状态后再决定，不盲目重复放置、计分或兑换。server_rejected表示收到明确失败回包，submitted_unconfirmed不是执行成功。云端503/504会带submitted/confirmed/retryable；不偷偷重试非幂等写。

## 长NBT操作
优先building.placeContainer(x,y,z,"minecraft:barrel",Items,options)返回job_id，再building.nbtJob(job_id)查询状态与verified。会占用真实作业位置、可能清理制作背包，先检查说明并暂停观察运动；不能在5秒同步调用里假装完成整条NBT自测。placeNBTBlock旧同步接口仍保留兼容，超时结果可能未知。当前没有统一覆盖所有旧同步方法的持久异步任务；重启后内存NBT任务不恢复。

## 账号池最小成功路径
1. accountsPool.list({all=false})获得自己账号id；has_token只证明存了凭据。
2. accountsPool.start(id,{server="服务器标识",connect=true})启动；不是“请求成功就进服成功”。
3. accountsPool.status(id)轮询engine_ready/connected/phase/fleet_ready。只有connected=true才验证当前server，credential_state=verified_for_current_server。
4. accountsPool.invoke(id,{path="/api/youyuan/game/call",method="POST",body={method="bot.self",args={}}})调用该账号。
/api/fleet是旧bot_tokens同进程池，空[]不代表账号管理池没有机器人。4399平台注册/启动器绑定/能进入网易游戏是不同步骤；宿主不能根据存有token就声称全部注册通过。

## 群服互通
死亡和重生不是退出/进入。名单以成功/list的名字为准，坐标查询失败不注销在线状态；周期事件只刷新名单，不逐玩家同步坐标。真正死亡/重生的事件仍由实际包与缓存处理。
.日志查游戏最近聊天；.群日志 [1..80]查机器人真实收到的近期QQ群消息，默认20，可分段。不是QQ全部历史或原生合并转发卡。普通成员仍隐藏坐标。
能发送图片不等于能理解表情包；当前图片发送/绘图配置可用，图像语义识别和表情图库尚未实现，不能让文字模型假装看图。QQ官方窗口/权限仍有效。

## 源码边界
公网宿主源码、SDK压缩包、Git目录、source map等下载路径已拦截；MCP资源来自本机已部署文档，不自动连接GitHub。玩家自己的插件项目仍可读写/导出，不能为隐藏宿主而砍掉开发功能。网页HTML/JS必须发给浏览器所以可查看；APK二进制也不能承诺无法逆向。源码审计不能替代多用户越权与部署目录完整验证。
