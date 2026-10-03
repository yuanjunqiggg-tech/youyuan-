# APK 内置临时公网 MCP

设置→公网远控与MCP→APK直接公网穿透。当前账号独立建立内置隧道（Cloudflare原生程序或Go SSH 443客户端）；不经游园域名中继。手机无需安装其他App、注册域名或登录Cloudflare。自动模式优先Cloudflare，申请失败切换Pinggy；也可选择指定服务。开启时由对应服务申请实际临时域名，关闭时停止本机进程，域名失效；重新建立后域名会变化。

API：GET /api/youyuan/tunnel 返回 available/enabled/state/url/mcp_url/error/provider/selected_provider/temporary。POST 同路径 {action:"start",provider:"auto"} 或 {action:"stop"}；provider 支持 auto/cloudflare/pinggy，切换前先关闭。当前账号的连接密钥 key 必须保密。外部MCP客户端使用 mcp_url 和 HTTP头 Authorization: Bearer <key>，协议为Streamable HTTP，JSON响应；不是旧版SSE。域名根路径只显示连接说明；/api/config、网页设置、文件浏览不在隧道暴露范围内。MCP tools/list、resources/read、tools/call仍使用完整内核工具与按需技能资料。

开启状态保存；下次启动重新建立。内置客户端使用一个HTTP2连接，按照服务端退避恢复网络；进程退出由宿主按10秒至5分钟退避重建，域名申请受限时至少等待1分钟。手机应用主进程退出时终止其隧道子进程，前台保活服务负责持续运行。已分配域名不表示已经连通：先检查state，再实际POST MCP initialize/tools/list测试。临时服务无在线保证、域名非永久、在飞请求最多200，不支持SSE。长期运行可继续使用稳定域名中继或部署固定Cloudflare Tunnel。

不要将密钥写入公开插件、共享文档或AI输出。关闭当前承载连接的隧道可能使本次MCP回复中断，之后应检查APK状态，不能因此自动重复危险工具调用。公网MCP密钥授予当前执行端工具调用权限，不等于只读分享链接。

Android程序打包于当前ABI原生库目录libyy_tunnel.so，APK直接调用，无root要求。Windows同目录需要cloudflared.exe，Linux需要同目录cloudflared可执行文件。程序缺失时available=false，如实报错，不把候选URL当成真实隧道。

Pinggy使用原生Go SSH反向转发，TCP443出口，无需另装SSH应用；当前服务单次免费会话60分钟，重建会改变域名。首次连接将SSH主机公钥记在当前账号私有文件 tunnel-pinggy-host.pub，后续指纹变化拒绝连接。并发转发最多32连接，每连接120秒，目标固定为本机专用MCP端口。若MCP客户端使用浏览器User-Agent，可添加 X-Pinggy-No-Screen: 1；普通MCP HTTP客户端不需要。隧道提供商实际不同，以state中的provider为准，不能把备用SSH成功说成Cloudflare实测通过。
