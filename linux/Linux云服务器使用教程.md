# 游园0.10.0 Linux云服务器使用教程

本包包含Linux程序、启动脚本、systemd服务示例、MCP教程和SDK。聊天、插件、账号与任务数据保存在你指定的YY_DATA_DIR，属于你自己的服务器；游园公网中继不将聊天记录作为共享数据库存储。

## 选择架构

运行`uname -m`。显示`x86_64`选择linux-x86_64包；显示`aarch64`或`arm64`选择linux-arm64包。Linux程序没有.exe后缀。两种架构都提供编译产物；这次尚未访问你的Linux云服务器，不能把交叉编译当成已经在该服务器通过实测。

Lua、命令链、Go游戏连接/MCP和桌面Python常驻适配器随程序提供。Python需要服务器安装Python3（建议3.12）；验证与运行都会查找python3，也可用YY_PYTHON指定解释器。**当前Linux/Windows的C++动态库适配器尚未接入**；Android C++适配器与Linux不同，不能直接把Android.so当作Linux插件使用。

## 直接启动

解压到自己的目录，例如`~/youyuan`，进入该目录后执行：

```sh
chmod +x youyuan start.sh
./start.sh
```

默认数据目录是当前用户的`~/.local/state/youyuan`，端口19429，日志按北京时间显示。可明确覆盖：

```sh
YY_DATA_DIR="$HOME/youyuan-data" YY_PORT=19429 YY_TIMEZONE=Asia/Shanghai ./start.sh
```

需要其他时区时，YY_TIMEZONE填写IANA时区名称，例如Europe/London。时间戳本身没有加八小时；历史UTC记录在显示时转换。不要再让插件对本地时间重复加八小时。

主面板只监听127.0.0.1。你电脑上可建立SSH端口转发：

```sh
ssh -L 19429:127.0.0.1:19429 你的SSH用户@服务器地址
```

保持SSH会话，然后在电脑浏览器打开`http://127.0.0.1:19429/`。登录游园平台账号，添加自己的游戏账号、验证服务器和服务器号，开启连接及自动重连。服务器进程运行后才可以远程执行工具。

## systemd保活

先把本包程序和文件放到`/opt/youyuan`，创建专用用户及数据目录：

```sh
sudo useradd --system --home-dir /var/lib/youyuan --create-home --shell /usr/sbin/nologin youyuan
sudo mkdir -p /opt/youyuan /var/lib/youyuan
sudo chown -R youyuan:youyuan /var/lib/youyuan
sudo chmod 700 /var/lib/youyuan
sudo cp youyuan.service /etc/systemd/system/youyuan.service
sudo systemctl daemon-reload
sudo systemctl enable --now youyuan
sudo systemctl status youyuan
```

如果已经创建youyuan用户，不要重复执行useradd。服务文件的ExecStart、WorkingDirectory、User和YY_PYTHON应与实际安装位置一致。

`journalctl -u youyuan -n 100 --no-pager`查看最近服务日志。systemd负责程序退出后重启；应用内自动重连负责游戏掉线。服务器拒绝认证、账号风控、断网或权限不足不能用重启保证解决，不要高频无限重试。

## MCP

同一台服务器上的客户端可使用`http://127.0.0.1:19429/mcp/`和该实例的本机MCP密钥。另一台电脑可以使用SSH转发后的地址，或使用设置中的公网连接/临时隧道。具体操作见同包MCP接入教程.md。

两种架构的包分别附带对应架构的cloudflared，用于原生临时公网隧道；不能混用x86_64和ARM64的helper。两个helper已交叉编译并检查ELF架构，尚未在你的Linux云服务器进行隧道实测。稳定域名中继不要求本机安装cloudflared。密钥从自己的实例设置中复制，不把游戏Token当作MCP密钥。

## 更新并保留数据

1. 停止服务：`sudo systemctl stop youyuan`。
2. 在你自己的位置备份YY_DATA_DIR；里面含私有账号配置，不应公开上传。
3. 替换`/opt/youyuan/youyuan`及同架构helper和文档，保留原YY_DATA_DIR。
4. 设置执行权限后启动：`sudo systemctl start youyuan`。
5. 检查版本、已保存账号和插件、游戏连接及MCP状态。

不要删掉数据目录或把不同架构程序混用。Linux包的升级保留数据依靠独立数据目录；APK的检查更新安装流程不适用于替换Linux可执行文件。
