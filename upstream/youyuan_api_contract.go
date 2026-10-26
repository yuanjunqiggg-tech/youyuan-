package main

func yyAPIContract(path string) string {
	switch path {
	case "/api/youyuan/game/call":
		return "POST body={method:'命名空间.方法',args:[参数...]}; account放工具顶层，不重复嵌套。game.execute等待服务器确认；game.sendCommandFast仅提交。NBT长作业优先building.placeContainer返回job_id，再building.nbtJob(job_id)查verified。事件订阅需常驻插件，不用一次性HTTP注册回调。"
	case "/api/youyuan/account-pool":
		return "POST body={action:'list',args:[{all:false}]}列自己的账号；start参数=[id,{server,connect:true}]，status参数=[id]，invoke参数=[id,{path,method,body,query}]。status.connected才证明进服；has_token只证明存了凭据。相互独立进程，不是/api/fleet旧同进程槽位。"
	case "/api/fleet/status", "/api/fleet/connect":
		return "这是原版bot_tokens配置的同进程Fleet；空[]可能只是未配置该池。游园账号管理池请调用api_youyuan_account_pool的list/start/status，不把两种池混用。"
	case "/api/youyuan/sense/events", "/api/youyuan/sense/snapshot", "/api/youyuan/packets/capture":
		return "GET通过MCP默认摘要，query={limit:20,types:'swing,health',runtime_id:'十进制ID',after_sequence:0}; query.view='raw'才读取原始数据。packets/capture的POST body={action:'start|stop|clear',config:{types:[],capacity:4096}}。捕获范围只含服务器下发给该账号的包。"
	case "/api/youyuan/observations/summary":
		return "GET query={limit:20,types:'swing,health',entity_type:'minecraft:zombie',runtime_id:'ID',source:'packets'可选};按本账号保留事件/抓包环计数，不扫描全服实体。"
	case "/api/youyuan/mcp/traces":
		return "GET query={trace_id:'上次structuredContent.mcp_meta.trace_id'}；省略ID列最近30次本人调用，管理员可读保留轨迹。只有元数据，不含命令/密钥/源码；写操作timeout_unknown不能盲目重试。"
	case "/api/youyuan/mcp/scope":
		return "本机控制台的 MCP 登记表：POST body={op:'list|save|delete|quick|test'}。scope=public(配在这台机器上，按access给群友用)/private(绑某个人自己的机器，只有owner能调)；access=all|admin|owner_only；transport=stdio|http|sse；http/sse填url，stdio填command+args，密钥放headers(每行Key:Value)，note只做备注。op=test会真的握手一次，回显协议版本、耗时和工具数量；先测再存可以只传server不保存。这是写接口，不在只读白名单里，外部只读连接调不到。"
	case "/api/youyuan/packets/schema":
		return "GET query={name:'Animate'}读取单包字段；省略name列全部注册协议定义。packet.catalog()返回简短目录。定义存在不等于服务器广播，也不证明可写。"
	case "/api/youyuan/snapshot":
		return "GET无参返回首页概览聚合：bot/status、youyuan/remote、runtime/list、task/list、plugin/list、qq/status、connection-diagnostics 七项合并成一次读，字段名 game/remote/runtime/tasks/plugins/qq/diag。为的是给 Cloudflare Worker+DO 的双扣配额止血：一个界面刷新只算一次请求。任何单项失败只影响该字段，整体 ok=false 仅在全部失败时出现。"
	case "/api/youyuan/world/seed":
		return "GET无参返回本机内核在登录StartGame包里实际收到的世界种子（基岩版，seed_known=false表示还没读到字段）；seed=123显式指定种子；structure=village且edition=java才做Java算法试算，结果放在java_preview且applies_to_world=false，不是当前基岩世界的真实结构。"
	case "/api/qq/messages":
		return "GET query={group:'已启用群标识',limit:20}；只读机器人实际收到的近期群消息，不是QQ完整历史。群指令.群日志可分段发送已有记录。"
	}
	return ""
}
