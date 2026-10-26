# scoreboard / python-runtime

本章使用游园真实宿主绑定；先通过known/stale判断观测，不编造游戏完成状态。


### `scoreboard.get`

objective,player；读取SetScore已同步条目，未广播known=false，不同步查询全服。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `scoreboard.increment`

objective,player,amount整数；提交原生scoreboard add/remove，不本地读改写，避免竞态；提交不等于服务器成功。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。

### `scoreboard.set`

objective,player,value整数；提交计分板设置，submitted=true/confirmed=false，调用Confirmed命令或同步SetScore才能验证结果。

返回字段及来源请读取observations.md。本接口在三语言共享目录注册；Linux native C++ 常驻 ELF 运行器已开放，使用同一宿主API；Android native仍在宿主进程内，Windows未开放。
