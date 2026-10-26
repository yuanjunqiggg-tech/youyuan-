# 互通端 · 玩家列表与详情抽屉（进行中）

用户原话：
> 「互通端消息，玩家总数显示这么多干什么？然后显示他们又做什么呢？」
> 「继续弄，每页 5 个，然后可以翻页」
> 「这些人能看得到 QQ，但是你显示个名字，这就不太好了吧。我希望的是也能像那种网页里面一样，可以点开看一下他的皮肤，而且是预加载的；平常显示他的头像，点开之后还能看到他的一些具体信息和主要说的一些话、他的权限，然后还点进去可以看到他的分数、积分什么的。它的标签，这些插件原生口都有。」
> 「记得这些不要太拥挤哈，别一大堆内容都塞在一起……太长的话又占位置，然后又导致人都不想看下去。」

## 进度

- [x] 现状确认（见下「已知事实」）
- [ ] A 列表：每页 5 条 + 翻页 + 行内只三样
- [ ] B 详情抽屉（底部滑出）
- [ ] C 预加载当前页 5 张 player-card
- [ ] D 聚合接口 `GET /api/qq/server/player?name=`
- [ ] 测试：go vet / go test -run SrvInfo / node --check
- [ ] 实测 375×812 + 截图

### 已知事实（已核实，勿重复调研）

- 玩家数据：`GET /api/qq/server/players`，字段 `player,qq,avatar,online,online_seconds,messages,last_message,joined,last_join,last_leave`；行排序「在线优先 → 最近活跃倒序」。
- 服务端已能渲染玩家卡 PNG：`GET /api/qq/player-card?player=<名字>`（`youyuan_qq_assistant.go:246`，实现见 `yyQQPlayerCard`）。详情页形象预览直接用它。
- 权限：`yyQQRole(cf, qq)` 返回 0/1/2（成员/管理员/超管），名单在 `cf.Assistant.Admins` / `SuperAdmins`。
- 积分：`/api/qq/level` 只回 `config/stats/bindings`，**没有按玩家查**。但包内可直接用 `yyLevelXP(group, qq)` + `yyLevelProgressOf(cf, xp)` 算出等级/当前经验/升级所需；`PlayerBind` 是 QQ→玩家名，`yyQQHelper.Bindings` 是 QQ→玩家名（同一方向）。
- 标签：全仓库没有任何「玩家标签」数据源（插件原生口未接入）。**不编数据**，接口返回空数组 + note。
