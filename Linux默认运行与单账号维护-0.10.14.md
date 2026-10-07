# 游园 0.10.14 · Linux 默认运行与单账号维护

日常入口：https://toolbox.youyuanqi.dpdns.org/?device=linux-server
唯一平台账号：幽怨qi。密码单独保存在私有目录，公共源码和APK不包含登录密码。
测试、插件优化与日常网页操作使用同一个Linux执行端、同一个平台账号、同一个主游戏会话。平台账号不是游戏令牌；删除平台测试账号不会删除游戏服务器中的真实玩家。

本次确认原来的“离线”是默认primary-device已停用，Linux仍在线。云端所有者缺省节点现为linux-server，前端旧primary默认链接切到Linux；本机登录保留已有节点slug，避免登录时覆盖到APK节点。

Linux直连网关和cloudflared隧道由systemd管理。前端登录后低频访问/api/cloud/linux-direct获取当前临时HTTPS入口；终端事件、插件文件、工作区、命令链和状态API优先由Linux本机直接处理，不经过Workers/DO中继。identity/OAuth/授权和兼容回退仍由Cloudflare承担。
网关只接收项目所有者和私有维护凭据；普通账号、匿名请求拒绝。云端会话校验在Linux缓存60秒，所以撤销会话最迟60秒生效；退出当前登录会主动清除本机缓存。许可CORS仅为既有toolbox和workers.dev前端。
32个并发网关连接只影响公网HTTP请求，不是命令CPS限制。游戏命令Fast路径不改。
上传仍为32MB/次；大型本机文件可以通过SSH上传至Linux目录。这个是网关传输边界，不是命令条数或建筑方块限制。

临时域名在隧道重启后会变化，Linux健康响应公布新地址；重新打开日常入口会重新发现。域名中断时GET可回退中继；写请求不自动重发，避免重复操作。后台事件轮询10秒；离线指数退避最长60秒。
Quick Tunnel是临时入口，Cloudflare文档说明最多200个在途请求且无SSE；这里使用增量JSON事件。不能将其宣称为生产SLA。固定linux.youyuanqi.dpdns.org尚未部署，现有OAuth凭据没有DNS写权限。

服务：youyuan（游戏/插件原生内核），youyuan-direct（HTTPS入口的本机鉴权网关），youyuan-direct-tunnel（公网传输）。后二者不是额外机器人账号。
代码：linux/direct_gateway.py、linux/direct_tunnel.py。私有配置/etc/youyuan/direct-gateway.json；节点地址/var/lib/youyuan/linux-direct.json；插件仍在/var/lib/youyuan。
Python默认urllib客户端曾触发Cloudflare1010，网关显式使用YouYuan-LinuxGateway/0.10.14标识后真实校验200；未降低站点权限或关闭防护。

先保存修复插件/数据/配置和平台注册表，再删除四个非所有者账号，并撤销对应会话、control/device凭据、MCP链接和OAuth授权记录。历史备份保留在private/single-owner-01014，可恢复资料；公共源码不含该备份。
原有管理员入口users/entitlement保留，日后用户注册与订阅仍可统一管理。当前Linux直连入口只允许所有者，不共享给订阅成员。

实测：30次连续公网状态查询全部200且游戏在线，期间网关Cloudflare鉴权次数没有增加；所有者认证、匿名拒绝、CORS许可/拒绝、当前入口自动发现、R21A运行已验收。没有进行大规模压测或24小时稳定性验收，Chat客户端最终授权仍待完成。

本次临时入口：https://privileges-jacket-versions-sports.trycloudflare.com
