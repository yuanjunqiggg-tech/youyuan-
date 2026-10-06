# 云端订阅与Linux使用

入口：https://toolbox.youyuanqi.dpdns.org/?device=linux-server

使用自己的项目管理员控制密钥登录后，在设置→订阅与用户权限→管理订阅用户，选择已注册账号，设置订阅到期日期、续期或撤销，并按需要分配linux-server。只有项目拥有者可以授予管理员或修改管理员；普通管理员可管理普通成员订阅。到期/撤销由服务端每次调用校验，旧网页登录态和MCP控制令牌也被阻止。

Linux共享节点分配意味着授权操作整个工具箱，包含这个节点上的机器人、插件、文件与日志；不是每个订阅用户自动独立的Linux容器。需要隔离请分配各自的独立工具箱节点。默认不分配项目主APK。

网站是工具箱控制面板，不是root系统Shell。设置→文件选择器与界面→上传/下载工具箱文件，可上传到Linux本地用户目录；普通上传32MiB，大文件用SFTP。Linux服务通过systemd开机启动并恢复进程退出，数据在/var/lib/youyuan，程序在/opt/youyuan。升级替换程序，不删除数据。

MCP入口：https://toolbox.youyuanqi.dpdns.org/mcp/?device=linux-server 。Streamable HTTP，自定义MCP客户端Authorization: Bearer 自己的有效控制密钥。资源按需读取，对应语言文档/Skill和工具权限分别配置，不能把拥有文档理解成拥有管理员工具权限。

APK设置→服务器连接→后台电池保护可请求系统排除省电，开启自动连接/自动重连。厂商后台限制需在手机实际设置；用户主动强制停止仍需再次启动。本轮暂停新的游戏测试，24小时游戏挂机和实体手机网络切换没有标成功。
