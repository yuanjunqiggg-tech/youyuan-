# Linux云服务器使用教程（0.10.9）

网页打开 https://toolbox.youyuanqi.dpdns.org/?device=linux-server 。这是你的云服务器上游园工具箱的管理界面。手机APK网页不加device参数，两设备互不挤占连接。按网页登录提示使用你的私有控制台访问配置；公开包不含账号或密钥。

在设置内打开“文件上传”，选择一个或多个文件，列表可下载。实际文件存放 /var/lib/youyuan/用户上传；资料库文件在 /var/lib/youyuan/本地资料库。2026-10-05已从公网实际上传UTF-8文本、下载逐字节校验并清理测试文件。Cloudflare中继当前单次大文件传输约32MiB，超出时用SFTP；这是文件传输限制，不是命令链条数限制。

进入服务器：Windows终端运行 `ssh root@47.102.156.125`，输入你自己的服务器密码（输入时不显示字符）。程序已安装为systemd服务，无须把APK放进Linux。

```sh
systemctl status youyuan --no-pager
journalctl -u youyuan -n 100 --no-pager
systemctl restart youyuan
curl http://127.0.0.1:19429/health
```

服务程序 /opt/youyuan/youyuan；服务配置 /etc/systemd/system/youyuan.service；用户数据 /var/lib/youyuan；监听127.0.0.1:19429。网页经鉴权公网中继访问此进程；不提供任意root网页终端。

网页连接账号：设置选择正确认证站、服务器类型及山头/房间ID，填自己的令牌，连接后查看OP。Linux当前没有拿同一个测试账号重复进服，以免踢掉模拟器；不能把网页可达写成Linux当前已连接游戏。多账号使用独立账号凭据。

Linux MCP地址 https://toolbox.youyuanqi.dpdns.org/mcp/?device=linux-server 。采用Streamable HTTP/HTTP，Authorization: Bearer 后填Linux设备自己的MCP密钥。APK与Linux密钥分别在各自设置查看。密钥不是游戏令牌，也不是AI模型API Key。添加成功后先列tools/resources，再调用health/status，不能只以“连接成功”验收游戏动作。

上传大文件建议用WinSCP/SFTP连接同一服务器，并放在自己的数据目录；不要覆盖用户数据库。更新程序先备份现有文件，停止服务，替换同架构youyuan二进制，chmod +x，启动；保留/var/lib/youyuan全部数据。Android C++ .so不能拿去Linux使用，当前Linux C++宿主尚未支持。

静态入口、认证和公网中继仍在Cloudflare；运行插件、日志、文件、Linux资料库存储在本机磁盘。尚未做确定用户容量的并发压力测试。
