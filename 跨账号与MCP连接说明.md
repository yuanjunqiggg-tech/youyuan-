# 游园0.10.11交付与迁移说明

新增不同账号之间的定向联通API、URL内独立MCP凭证、只读/管理授权、有效期/撤销与云端设备绑定；保留原来的Bearer入口。设置新增“账号联通与连接授权”。五个accounts接口已同步Lua/Python/C++SDK、独立account-bridge技能与文档。现有账号数据及原项目所有者未重置。

云端网页：https://toolbox.youyuanqi.dpdns.org/?device=linux-server
更新入口：https://updates.youyuanqi.dpdns.org/updates/android/latest.apk
管理员用户名：youyuan_admin。密码只在私有交接文件，不放入公开APK/源码/SDK。

Grok网址：在设置生成专用连接URL，复制到 Custom MCP。要改插件、热重载、运行指令请选择“管理”，默认“查询”不会给这些工具。生成链接和实际MCP握手、调用、撤销可以回归；没有登录用户Grok账号做界面验收，因此不能宣称Grok所有版本已通过。

Cloudflare临时隧道官方限制为200在途请求、无SSE、无SLA；官方未公布固定每日/历史申请数。我们的128个有效MCP连接凭证上限不是隧道供应商限制。长期运行使用已部署Linux与固定域名。

本轮暂停新的游戏压测和倍率测试，不把本地提交速度当游戏完成速度；此前连续180000命令失败、原版倍率、24小时真实游戏挂机未验收等仍保留在备忘录。新跨账号游戏消息/命令副作用不在本轮新压力验收范围。

迁移整合包采用外层ZIP+内层AES256 ZIP，保护所有历史聊天中的私钥和凭据。五种恢复办法是互补路径：原始Codex历史与SQLite一致性快照；Markdown/JSON可读对话；完整Git bundle；全部项目源码、脚本、交付输出；APK/Linux运行数据、用户插件和私有配置。可以不用恢复桌面侧边栏就凭可读记录+AI交接文档继续。

完整范围、数量、每文件SHA256、缺失附件和排除缓存记录，以迁移包目录中的manifest与详细恢复指南为准。登录Codex/Cloudflare等账户应在新电脑自行登录；不复制Codex auth.json、浏览器Cookies或OAuth登录缓存。本包没有公开上传。
