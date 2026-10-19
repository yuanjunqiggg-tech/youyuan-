# 插件生成文件下载（0.10.43）

这里的导出是插件运行后生成的建筑/NBT/日志/数据/ZIP，不是备份插件源码。

网页：插件 → 插件文件下载 → 插件生成文件 → 选择插件 → 下载文件。数据目录里的每个文件也有下载按钮。Linux 文件保存在 Linux 执行设备，浏览器下载到用户电脑；不上传到原作者服务。完整插件备份在单独的“完整插件备份”页。

旧 Lua 插件只要把文件写到 game.dataDir()/plugin.dataDir()，刷新即可看到，无需修改原导出函数。若脚本把文件写到了任意外部绝对目录，宿主不会自动公开整个磁盘；应把需要交付的结果写进插件数据目录。

## Lua
```lua
util.write_file(plugin.dataDir() .. "/exports/结果.txt", "生成内容")
local file = plugin.exportFile("exports/结果.txt")
local files = plugin.listOutputs("exports")
-- file.name / path / bytes / modified_at / download_url
```

## Python
```python
from pathlib import Path
Path(plugin.dataDir(), "结果.txt").write_text("生成内容", encoding="utf-8")
file = plugin.exportFile("结果.txt")
files = plugin.listOutputs()
```
运行时注入的 plugin 命名空间共用实际宿主 API；无需复制文档里的伪实现。

## C++
```cpp
auto file = client.plugin.exportFile("exports/result.zip");
auto files = client.plugin.listOutputs();
```
使用 youyuan.hpp 的 Client；文件通过标准文件 API 写到 client.plugin.dataDir() 返回的当前账号数据目录。

## MCP 与 HTTP
GET /api/youyuan/plugin-outputs?id=插件ID 列出生成文件。
GET /api/youyuan/plugin-outputs/info?id=插件ID&file=相对路径 返回元信息。
GET /api/youyuan/plugin-outputs/download?id=插件ID&file=相对路径 下载原字节。
Python/C++/lua_compat 加 kind=runtime&runtime_account=main（或有权限的实例账号）。外层跨机器人请求继续走 accounts/API 路由，不要混淆外层机器人 ID 与 runtime_account。

需要登录并有权访问执行账号。拒绝路径越界、符号链接、目录下载。MCP 调用 download 时只返回元信息 downloaded=false，避免二进制塞进 AI 上下文；浏览器使用登录会话下载。snapshot=false 表示读取该文件当前版本，正在写入的文件请等待写完再下载；完整插件备份仍是不可变 ZIP 快照。此功能不证明 NBT 游戏导入或导出算法本身有效。
