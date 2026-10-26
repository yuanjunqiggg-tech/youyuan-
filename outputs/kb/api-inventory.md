# 游园工具箱 / MCORT — API 全量清单

> 口径：逐条解析 `upstream/*.go`（排除 `_test.go`）中的 `mux.HandleFunc(...)` 注册语句；方法/入参/返回由注册点回溯 handler 函数体静态推断。
> 前端侧解析 `upstream/frontend/js/**/*.js`（排除 `vendor/` 与 `.esb` 打包产物）中的 `/api/...` 字面量。每条结论都可回到标注的 `文件:行` 复核。

## 0. 口径与边界

| 项 | 说明 |
| --- | --- |
| 服务形态 | Go `net/http` + `http.ServeMux`，单进程；`main.go` 用 `net.Listen("tcp", "127.0.0.1:"+port)`，只绑回环 |
| 注册总入口 | `main.go:5010` 调 `yyRegisterAPI(mux, dataDir)`（`youyuan_remote.go:71`），内部再串起 60+ 个 `yy*Routes(mux, ...)` |
| 中间件顺序 | `yyConsoleAuth(yyIntegrityGate(corsHandler))`（`main.go:5280`）；会话鉴权在最外层，只拦 `/api` 与 `/api/*` |
| 方法列 | ServeMux 不按方法分流；方法由 handler 体内 `r.Method ==/!= ...` 推断，「任意」= 该 handler 未做方法判断 |
| 入参列 | `q:` = query 字段；`b:` = JSON body / 表单字段（取 `json:"..."` tag 与 `FormValue`），各最多列 8 个 |
| 计数 | 源码共 **384** 处路径字面量注册（`grep -h 'mux\.HandleFunc(' upstream/*.go` 排除 `_test.go` 得 386 个调用点，其中 2 个参数不是字面量），本表 **384** 条（一一对应）。`youyuan_plugin_outputs.go:127` 在 `for range` 中展开 3 条（净 +2），另加 `main.go:5016` 别名循环 3 条与 `youyuan_accounts.go:318` 子树前缀 1 条，运行时实际注册 **390** 条（见 5.5） |

## 1. 注册点分布（按文件）

| 文件 | 注册数 |
| --- | --- |
| `upstream/main.go` | 182 |
| `upstream/youyuan_runtime.go` | 11 |
| `upstream/youyuan_seedv2_routes.go` | 9 |
| `upstream/youyuan_notices.go` | 8 |
| `upstream/youyuan_remote.go` | 8 |
| `upstream/youyuan_seed_api.go` | 8 |
| `upstream/youyuan_agent.go` | 7 |
| `upstream/youyuan_qq_serverinfo.go` | 7 |
| `upstream/youyuan_bridge_api.go` | 6 |
| `upstream/youyuan_capture.go` | 6 |
| `upstream/youyuan_qq.go` | 6 |
| `upstream/youyuan_qq_knowledge.go` | 5 |
| `upstream/youyuan_theme.go` | 5 |
| `upstream/youyuan_account_bridge.go` | 4 |
| `upstream/youyuan_export_center.go` | 4 |
| `upstream/youyuan_qq_assistant.go` | 4 |
| `upstream/youyuan_qq_caps.go` | 4 |
| `upstream/youyuan_qq_member.go` | 4 |
| `upstream/youyuan_qq_rag.go` | 4 |
| `upstream/youyuan_sdk.go` | 4 |
| `upstream/youyuan_agent_preferences.go` | 3 |
| `upstream/youyuan_agent_sessions.go` | 3 |
| `upstream/youyuan_appearance.go` | 3 |
| `upstream/youyuan_converters.go` | 3 |
| `upstream/youyuan_host_files.go` | 3 |
| `upstream/youyuan_market.go` | 3 |
| `upstream/youyuan_qq_skill.go` | 3 |
| `upstream/youyuan_skill_library.go` | 3 |
| `upstream/youyuan_account_pool.go` | 2 |
| `upstream/youyuan_agent_cleanup.go` | 2 |
| `upstream/youyuan_observations.go` | 2 |
| `upstream/youyuan_qq_actions.go` | 2 |
| `upstream/youyuan_qq_approval.go` | 2 |
| `upstream/youyuan_qq_level.go` | 2 |
| `upstream/youyuan_qq_login.go` | 2 |
| `upstream/youyuan_qq_official.go` | 2 |
| `upstream/youyuan_qq_usage.go` | 2 |
| `upstream/youyuan_runtime_management.go` | 2 |
| `upstream/youyuan_skin_cache.go` | 2 |
| `upstream/youyuan_skin_item.go` | 2 |
| `upstream/youyuan_social.go` | 2 |
| `upstream/youyuan_accounts.go` | 1 |
| `upstream/youyuan_agent_budget.go` | 1 |
| `upstream/youyuan_agent_fork.go` | 1 |
| `upstream/youyuan_benchmark.go` | 1 |
| `upstream/youyuan_block_video.go` | 1 |
| `upstream/youyuan_commands.go` | 1 |
| `upstream/youyuan_docs.go` | 1 |
| `upstream/youyuan_file_upload_sessions.go` | 1 |
| `upstream/youyuan_game_auth.go` | 1 |
| `upstream/youyuan_global_capture.go` | 1 |
| `upstream/youyuan_identity.go` | 1 |
| `upstream/youyuan_mail.go` | 1 |
| `upstream/youyuan_mcp_links.go` | 1 |
| `upstream/youyuan_mcp_scope.go` | 1 |
| `upstream/youyuan_mcp_telemetry.go` | 1 |
| `upstream/youyuan_observation_summary.go` | 1 |
| `upstream/youyuan_observer_bridge.go` | 1 |
| `upstream/youyuan_plugin_outputs.go` | 1 |
| `upstream/youyuan_profiler.go` | 1 |
| `upstream/youyuan_qq_affinity.go` | 1 |
| `upstream/youyuan_qq_claim_qr.go` | 1 |
| `upstream/youyuan_qq_context.go` | 1 |
| `upstream/youyuan_qq_groups.go` | 1 |
| `upstream/youyuan_qq_help.go` | 1 |
| `upstream/youyuan_qq_image.go` | 1 |
| `upstream/youyuan_qq_memory.go` | 1 |
| `upstream/youyuan_qq_messages_multi.go` | 1 |
| `upstream/youyuan_qq_moderation.go` | 1 |
| `upstream/youyuan_qq_moderation_nl.go` | 1 |
| `upstream/youyuan_qq_persona.go` | 1 |
| `upstream/youyuan_qq_proactive.go` | 1 |
| `upstream/youyuan_qq_relay.go` | 1 |
| `upstream/youyuan_qq_social_tools.go` | 1 |
| `upstream/youyuan_qq_toolbox.go` | 1 |
| `upstream/youyuan_qq_triggers.go` | 1 |
| `upstream/youyuan_qq_watchdog.go` | 1 |
| `upstream/youyuan_seed_world.go` | 1 |
| `upstream/youyuan_tunnel.go` | 1 |
| **合计** | **384** |

路由没有集中在一张表里，而是散在 **79 个文件**、由 `yyRegisterAPI` 逐层串联。排查某个接口时先按路径定位文件，再回 `youyuan_remote.go:161-198` 或 `youyuan_qq.go:84-137` 确认是否被真正接线——**注册了不等于接线了**，5.2 节有 8 条反例。

## 2. 主路由表（384 条，按路径排序）

| 方法 | 路径 | 文件:行 | 入参 | 鉴权 | 返回要点 |
| --- | --- | --- | --- | --- | --- |
| 任意 | `/` | upstream/main.go:5024 | — | 无 | JSON {ok,error,path} |
| 任意 | `/api/ai/answer` | upstream/main.go:5086 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/chat` | upstream/main.go:5093 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/models` | upstream/main.go:5083 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/models/fetch` | upstream/main.go:5084 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/sessions` | upstream/main.go:5087 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/sessions/load` | upstream/main.go:5088 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/sessions/recent` | upstream/main.go:5089 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/sessions/rename` | upstream/main.go:5090 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/skills` | upstream/main.go:5085 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/snapshot` | upstream/main.go:5091 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/ai/snapshot/restore` | upstream/main.go:5092 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/announcements` | upstream/main.go:5176 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/bot/connect` | upstream/main.go:5102 | b: token,server,auth,password,use_new_protocol | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/bot/console` | upstream/main.go:5105 | b: input | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/bot/disconnect` | upstream/main.go:5103 | — | 控制台会话 token | JSON {ok} |
| 任意 | `/api/bot/status` | upstream/main.go:5104 | — | 控制台会话 token | JSON {ok,connected,is_op,server,server_target} |
| POST | `/api/bridge/call` | upstream/youyuan_bridge_api.go:359 | b: capability,params | X-Bridge-Token / Bearer / ?token | JSON {ok,error} |
| POST | `/api/bridge/emit` | upstream/youyuan_bridge_api.go:483 | b: event,player,data | X-Bridge-Token / Bearer / ?token | JSON {ok,error} |
| 任意 | `/api/bridge/events` | upstream/youyuan_bridge_api.go:512 | q: since,limit | X-Bridge-Token / Bearer / ?token | JSON {ok,events,seq} |
| 任意 | `/api/bridge/manifest` | upstream/youyuan_bridge_api.go:337 | — | X-Bridge-Token / Bearer / ?token | JSON {ok,version,capabilities,events,id} |
| 任意 | `/api/bridge/rules` | upstream/youyuan_bridge_api.go:540 | b: rules | X-Bridge-Token / Bearer / ?token | JSON {ok,rules} |
| POST | `/api/bridge/rules/delete` | upstream/youyuan_bridge_api.go:615 | q: id | X-Bridge-Token / Bearer / ?token | JSON {ok,error} |
| 任意 | `/api/building/analyze` | upstream/main.go:5135 | b: path,x1,y1,z1,x2,y2,z2 | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/building/stats` | upstream/main.go:5136 | b: path | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/building/voxel` | upstream/main.go:5137 | b: path | 控制台会话 token | JSON {ok,error} |
| POST | `/api/cache/clear` | upstream/main.go:5200 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/camera/clear` | upstream/main.go:5242 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/generate` | upstream/main.go:5239 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/pivot` | upstream/main.go:5235 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/shot/delete` | upstream/main.go:5238 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/shot/update` | upstream/main.go:5237 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/shots` | upstream/main.go:5236 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/start` | upstream/main.go:5232 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/status` | upstream/main.go:5234 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/stop` | upstream/main.go:5233 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/test` | upstream/main.go:5240 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/camera/test/all` | upstream/main.go:5241 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/cb/export/save` | upstream/main.go:5225 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/cb/export/scan` | upstream/main.go:5224 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/cb/import/preview` | upstream/main.go:5226 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/cb/import/start` | upstream/main.go:5227 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/cb/stats` | upstream/main.go:5228 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/cb/stats/preview` | upstream/main.go:5229 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/config` | upstream/main.go:5097 | — | 控制台会话 token | JSON {ok,config} |
| 任意 | `/api/draw/apply` | upstream/main.go:5145 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/close` | upstream/main.go:5150 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/create` | upstream/main.go:5140 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/custom` | upstream/main.go:5149 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/mode` | upstream/main.go:5144 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/palette` | upstream/main.go:5147 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/redo` | upstream/main.go:5143 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/state` | upstream/main.go:5148 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/stroke` | upstream/main.go:5141 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/sync` | upstream/main.go:5146 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/draw/undo` | upstream/main.go:5142 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/file/upload` | upstream/main.go:5134 | b: output_dir,filename | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/files/read` | upstream/main.go:5179 | q: path | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/files/scan` | upstream/main.go:5178 | q: path,filter; b: name,path,type,size,mtime,ext,has_building | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/files/write` | upstream/main.go:5180 | b: path,content | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fleet/connect` | upstream/main.go:5106 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fleet/disconnect` | upstream/main.go:5107 | — | 控制台会话 token | JSON {ok} |
| 任意 | `/api/fleet/status` | upstream/main.go:5108 | — | 控制台会话 token | JSON {ok,workers,status,main_connected,main_op} |
| 任意 | `/api/fly/down` | upstream/main.go:5119 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/jump` | upstream/main.go:5112 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/jump-start` | upstream/main.go:5113 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/jump-stop` | upstream/main.go:5114 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/look` | upstream/main.go:5120 | b: pitch,yaw,seconds | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/move` | upstream/main.go:5121 | b: dx,dy,dz | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/position` | upstream/main.go:5125 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/press` | upstream/main.go:5123 | b: direction | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/release` | upstream/main.go:5124 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/sneak-start` | upstream/main.go:5115 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/sneak-stop` | upstream/main.go:5116 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/sprint-start` | upstream/main.go:5117 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/sprint-stop` | upstream/main.go:5118 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/start` | upstream/main.go:5110 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/status` | upstream/main.go:5109 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/stop` | upstream/main.go:5111 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/fly/teleport` | upstream/main.go:5122 | b: x,y,z | 控制台会话 token | JSON {ok,error} |
| POST | `/api/heartbeat/enable` | upstream/main.go:5057 | b: enabled | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/integrity/check` | upstream/main.go:5177 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/itemmaker/generate` | upstream/main.go:5138 | — | 控制台会话 token | 见 handler |
| POST | `/api/map/display` | upstream/main.go:5182 | b: path | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/map/grid` | upstream/main.go:5186 | — | 控制台会话 token | JSON {ok,error} |
| POST | `/api/map/play` | upstream/main.go:5183 | b: path | 控制台会话 token | JSON {ok,error} |
| POST | `/api/map/scan` | upstream/main.go:5181 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/map/status` | upstream/main.go:5185 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/map/stop` | upstream/main.go:5184 | — | 控制台会话 token | JSON {ok} |
| 任意 | `/api/mapart/preview` | upstream/main.go:5132 | b: path,targetW,targetH,orientation,use_relief,color_space,dither | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/market-proxy/` | upstream/youyuan_notices.go:129 | — | GET 免 token；写需会话 token | 见 handler |
| 任意 | `/api/market/contribute` | upstream/youyuan_notices.go:131 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/market/download` | upstream/youyuan_notices.go:130 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/market/mine` | upstream/youyuan_market.go:387 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/market/plugins` | upstream/youyuan_market.go:385 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/market/plugins/` | upstream/youyuan_market.go:386 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/marquee/preview` | upstream/main.go:5206 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/marquee/start` | upstream/main.go:5203 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/marquee/status` | upstream/main.go:5205 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/marquee/stop` | upstream/main.go:5204 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/mcfunction/execute` | upstream/main.go:5209 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/mcfunction/parse` | upstream/main.go:5208 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/mcfunction/status` | upstream/main.go:5211 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/mcfunction/stop` | upstream/main.go:5210 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/mcfunction/syntax` | upstream/main.go:5212 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/mcort/announcements` | upstream/youyuan_theme.go:819 | b: op,id,title,content,severity,pinned | 控制台会话 token | JSON {ok,announcements,admin,brand} |
| GET | `/api/mcort/plugins` | upstream/youyuan_theme.go:821 | b: op,id,confirm | 控制台会话 token | JSON {ok,plugins,api,note} |
| GET | `/api/mcort/theme` | upstream/youyuan_theme.go:729 | b: id | 控制台会话 token | JSON {ok,active,theme,css_url} |
| GET | `/api/mcort/theme/asset` | upstream/youyuan_theme.go:773 | q: theme,file | 控制台会话 token | 原始字节 |
| GET | `/api/mcort/themes` | upstream/youyuan_theme.go:716 | — | 控制台会话 token | JSON {ok,api,active,themes,brand} |
| 任意 | `/api/media/preview` | upstream/main.go:5133 | b: path,inst,vol,pitch,delay | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/actionbar` | upstream/main.go:5167 | b: msg | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/loop` | upstream/main.go:5172 | b: mode | 控制台会话 token | JSON {ok,loop_mode} |
| 任意 | `/api/music/next` | upstream/main.go:5173 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/note` | upstream/main.go:5166 | b: instrument,pitch,volume | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/pause` | upstream/main.go:5163 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/play` | upstream/main.go:5161 | b: cache_key,target,speed,queue_index | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/prev` | upstream/main.go:5174 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/queue` | upstream/main.go:5168 | — | 控制台会话 token | JSON {ok,queue} |
| 任意 | `/api/music/queue/add` | upstream/main.go:5169 | b: cache_key | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/queue/clear` | upstream/main.go:5171 | — | 控制台会话 token | JSON {ok} |
| 任意 | `/api/music/queue/remove` | upstream/main.go:5170 | b: index | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/resume` | upstream/main.go:5164 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/music/status` | upstream/main.go:5165 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/music/stop` | upstream/main.go:5162 | — | 控制台会话 token | JSON {ok} |
| 任意 | `/api/music/upload` | upstream/main.go:5160 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/pathfinder/follow` | upstream/main.go:5127 | b: player | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/pathfinder/goto` | upstream/main.go:5126 | b: x,y,z | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/pathfinder/log` | upstream/main.go:5129 | — | 控制台会话 token | 原始字节 |
| 任意 | `/api/pathfinder/stop` | upstream/main.go:5128 | — | 控制台会话 token | JSON {ok} |
| 任意 | `/api/permission/storage` | upstream/main.go:5199 | — | 控制台会话 token | JSON {ok,granted} |
| 任意 | `/api/players/list` | upstream/main.go:5207 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/plugin-market-proxy/` | upstream/youyuan_notices.go:132 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/plugin-market/contribute` | upstream/youyuan_notices.go:134 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/plugin-market/install` | upstream/youyuan_notices.go:133 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/plugin/api-doc` | upstream/main.go:5080 | — | 控制台会话 token | JSON {ok,content} |
| GET | `/api/plugin/config` | upstream/main.go:5067 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/plugin/create` | upstream/main.go:5076 | b: type,name,description | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/data` | upstream/main.go:5074 | q: id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/delete` | upstream/main.go:5063 | b: id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/disabled` | upstream/main.go:5075 | b: id,disabled | 控制台会话 token | JSON {ok,error} |
| POST | `/api/plugin/export` | upstream/main.go:5065 | b: id | 控制台会话 token | JSON {ok,error} |
| POST | `/api/plugin/export-saf` | upstream/main.go:5066 | b: id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/file/read` | upstream/main.go:5078 | q: id,scope,file | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/file/write` | upstream/main.go:5079 | b: id,scope,file,content | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/import` | upstream/main.go:5064 | b: path | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/list` | upstream/main.go:5059 | — | 控制台会话 token | JSON {ok,plugins,market_origin} |
| 任意 | `/api/plugin/meta` | upstream/main.go:5077 | b: id,name,description | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/profiler` | upstream/youyuan_profiler.go:40 | q: account,id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/reload` | upstream/main.go:5062 | b: id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/run` | upstream/main.go:5060 | b: id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/stop` | upstream/main.go:5061 | b: id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/plugin/wordbank-doc` | upstream/main.go:5081 | — | 控制台会话 token | JSON {ok,content} |
| 任意 | `/api/preview/cancel` | upstream/main.go:5154 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/preview/generate` | upstream/main.go:5152 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/preview/generate-gif` | upstream/main.go:5159 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/preview/render-isometric` | upstream/main.go:5157 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/preview/save-image` | upstream/main.go:5158 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/preview/scan` | upstream/main.go:5151 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/preview/status` | upstream/main.go:5153 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/preview/zip` | upstream/main.go:5155 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/preview/zip-building` | upstream/main.go:5156 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/push/ack` | upstream/main.go:5202 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/push/pending` | upstream/main.go:5201 | — | 控制台会话 token | 见 handler |
| GET | `/api/qq/actions` | upstream/youyuan_qq_actions.go:329 | b: action,target,times,content,file,tid,group | 控制台会话 token | JSON {ok,actions,caps} |
| 任意 | `/api/qq/affinity` | upstream/youyuan_qq_affinity.go:569 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/ai/selfcheck` | upstream/youyuan_qq_social_tools.go:519 | — | 控制台会话 token | JSON {ok,model_name,model_id,provider,model_found} |
| GET | `/api/qq/ai/tools` | upstream/youyuan_qq_toolbox.go:193 | b: tools | 控制台会话 token | JSON {ok,groups,total} |
| GET | `/api/qq/approvals` | upstream/youyuan_qq_approval.go:558 | — | 控制台会话 token | JSON {ok,error} |
| POST | `/api/qq/assistant` | upstream/youyuan_qq_assistant.go:278 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/assistant/skills` | upstream/youyuan_qq_assistant.go:326 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/qq/bot/profile` | upstream/youyuan_qq_caps.go:395 | — | 控制台会话 token | 见 handler |
| GET | `/api/qq/caps` | upstream/youyuan_qq_caps.go:336 | b: caps,limits | 控制台会话 token | JSON {ok,caps,adapter} |
| GET | `/api/qq/claim/qr` | upstream/youyuan_qq_claim_qr.go:573 | — | 控制台会话 token | JSON {ok,error} |
| POST | `/api/qq/config` | upstream/youyuan_qq.go:147 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/diagnostics` | upstream/youyuan_qq_official.go:200 | — | 控制台会话 token | JSON {ok,enabled,transport,secret_saved,group_number} |
| 任意 | `/api/qq/emoji` | upstream/youyuan_qq_member.go:945 | b: action,id,wording,url | 控制台会话 token | JSON {ok,count,entries,source,manual} |
| POST | `/api/qq/emoji/refresh` | upstream/youyuan_qq_member.go:989 | — | 控制台会话 token | JSON {ok,error,degraded_to_manual,kept_entries,source} |
| 任意 | `/api/qq/faces` | upstream/youyuan_qq_caps.go:427 | — | 控制台会话 token | JSON {ok,error} |
| GET | `/api/qq/groups` | upstream/youyuan_qq_groups.go:89 | b: action,id,name,enabled | 控制台会话 token | JSON {ok,groups,official_numbers_are_notes} |
| GET | `/api/qq/groups/available` | upstream/youyuan_qq_actions.go:448 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/guard` | upstream/youyuan_qq_serverinfo.go:233 | b: disabled,extra | 控制台会话 token | JSON {ok,rules,extra,note} |
| GET | `/api/qq/help` | upstream/youyuan_qq_help.go:292 | q: level,raw | 控制台会话 token | JSON {ok,level,level_text,entries,text} |
| 任意 | `/api/qq/image` | upstream/youyuan_qq_image.go:457 | — | 控制台会话 token | JSON {ok,config,providers,id,label} |
| 任意 | `/api/qq/kb/doc` | upstream/youyuan_qq_knowledge.go:509 | q: id | 控制台会话 token | JSON {ok,error} |
| POST | `/api/qq/kb/search` | upstream/youyuan_qq_knowledge.go:666 | b: query,top_k,category | 控制台会话 token | JSON {ok,error} |
| POST | `/api/qq/kb/text` | upstream/youyuan_qq_knowledge.go:549 | b: category,title,text | 控制台会话 token | JSON {ok,error} |
| GET | `/api/qq/kb/tree` | upstream/youyuan_qq_knowledge.go:477 | — | 控制台会话 token | JSON {ok,error} |
| POST | `/api/qq/kb/upload` | upstream/youyuan_qq_knowledge.go:576 | b: category | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/level (未接线)` | upstream/youyuan_qq_level.go:665 | — | 控制台会话 token | JSON {ok,config,stats,players,groups} |
| GET | `/api/qq/level/top (未接线)` | upstream/youyuan_qq_level.go:693 | q: group,n | 控制台会话 token | JSON {ok,error} |
| GET | `/api/qq/login` | upstream/youyuan_qq_login.go:251 | — | 控制台会话 token | JSON {ok,error} |
| POST | `/api/qq/login/refresh` | upstream/youyuan_qq_login.go:264 | — | 控制台会话 token | JSON {ok,error} |
| GET | `/api/qq/members` | upstream/youyuan_qq_member.go:886 | q: group | 控制台会话 token | JSON {ok,error} |
| POST | `/api/qq/members/alias` | upstream/youyuan_qq_member.go:916 | b: group,qq,alias,action | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/memory` | upstream/youyuan_qq_memory.go:535 | q: group,message_id | 控制台会话 token | JSON {ok,error} |
| GET | `/api/qq/messages` | upstream/youyuan_qq_context.go:267 | q: group,limit | 控制台会话 token | JSON {ok,error} |
| GET | `/api/qq/messages/summary` | upstream/youyuan_qq_messages_multi.go:56 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/moderation` | upstream/youyuan_qq_moderation.go:392 | q: pattern,kind; b: pattern,kind,action,reason,enabled,rules,taboos | 控制台会话 token | JSON {ok,rules,taboos,disk_error,note} |
| GET | `/api/qq/moderation/rules` | upstream/youyuan_qq_moderation_nl.go:247 | — | 控制台会话 token | JSON {ok,rules,at} |
| POST | `/api/qq/onebot` | upstream/youyuan_qq_caps.go:445 | b: action,params | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/persona` | upstream/youyuan_qq_persona.go:584 | b: persona,style | 控制台会话 token | JSON {ok,persona,style,groups} |
| 任意 | `/api/qq/player-card` | upstream/youyuan_qq_assistant.go:327 | q: player | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/player-list` | upstream/youyuan_qq_assistant.go:336 | q: page | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/proactive` | upstream/youyuan_qq_proactive.go:492 | q: group; b: config | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/rag/docs (未接线)` | upstream/youyuan_qq_rag.go:1235 | q: scope,kind,page,id; b: scope,kind,title,text,data_url | 控制台会话 token | JSON {ok,docs,total,indexed_chunks,mode} |
| POST | `/api/qq/rag/reindex (未接线)` | upstream/youyuan_qq_rag.go:1319 | — | 控制台会话 token | JSON {ok,error} |
| POST | `/api/qq/rag/search (未接线)` | upstream/youyuan_qq_rag.go:1333 | b: query,scope,top_k | 控制台会话 token | JSON {ok,error} |
| GET | `/api/qq/rag/stats (未接线)` | upstream/youyuan_qq_rag.go:1360 | — | 控制台会话 token | 文本/空响应 |
| POST | `/api/qq/reconnect` | upstream/youyuan_qq.go:303 | — | 控制台会话 token | JSON {ok} |
| POST | `/api/qq/relay` | upstream/youyuan_qq_relay.go:633 | — | 控制台会话 token | JSON {ok,error} |
| GET | `/api/qq/roles` | upstream/youyuan_qq_approval.go:557 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/sandbox` | upstream/youyuan_qq.go:105 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/qq/send` | upstream/youyuan_qq.go:255 | b: text,group | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/server/events` | upstream/youyuan_qq_serverinfo.go:230 | b: time,type,player,qq | 控制台会话 token | JSON {ok,events,total,limit} |
| 任意 | `/api/qq/server/info` | upstream/youyuan_qq_serverinfo.go:228 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/qq/server/logs` | upstream/youyuan_qq_serverinfo.go:231 | b: time,source,kind,name,target,detail,ok,error | 控制台会话 token | JSON {ok,logs,total,limit,note} |
| 任意 | `/api/qq/server/player` | upstream/youyuan_qq_serverinfo.go:234 | q: name | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/server/players` | upstream/youyuan_qq_serverinfo.go:229 | q: online | 控制台会话 token | 见 handler |
| POST | `/api/qq/server/say` | upstream/youyuan_qq_serverinfo.go:232 | b: text,mode | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/skills` | upstream/youyuan_qq_skill.go:977 | q: name; b: skills | 控制台会话 token | JSON {ok,skills,builtin,runs} |
| GET | `/api/qq/skills/log` | upstream/youyuan_qq_skill.go:1048 | q: limit | 控制台会话 token | JSON {ok,entries} |
| POST | `/api/qq/skills/run` | upstream/youyuan_qq_skill.go:1028 | b: name,params | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/status` | upstream/youyuan_qq.go:235 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/qq/triggers` | upstream/youyuan_qq_triggers.go:572 | b: triggers | 控制台会话 token | JSON {ok,triggers,presets,types} |
| GET | `/api/qq/usage (未接线)` | upstream/youyuan_qq_usage.go:711 | q: scope,from,to | 控制台会话 token | 文本/空响应 |
| GET | `/api/qq/usage/context (未接线)` | upstream/youyuan_qq_usage.go:722 | q: group | 控制台会话 token | 文本/空响应 |
| POST | `/api/qq/verify` | upstream/youyuan_qq_official.go:187 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/watchdog` | upstream/youyuan_qq_watchdog.go:278 | b: enabled,reset,interval_seconds,max_restarts,max_per_hour | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/qq/webhook` | upstream/youyuan_qq.go:322 | b: op,t,id,d | Ed25519 签名 | JSON {plain_token,signature} |
| 任意 | `/api/repair/start` | upstream/main.go:5052 | b: path,x,y,z,speed,rotation,repair_circle,repair_nbt_only | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/seed/biome` | upstream/youyuan_seed_api.go:1167 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/coordinates` | upstream/youyuan_seed_api.go:1125 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/heat` | upstream/youyuan_seed_api.go:1054 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/manifest` | upstream/youyuan_seed_api.go:822 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/query` | upstream/youyuan_seed_api.go:1021 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/slime` | upstream/youyuan_seed_api.go:826 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/stronghold` | upstream/youyuan_seed_api.go:863 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/structure` | upstream/youyuan_seed_api.go:886 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/v2/biome` | upstream/youyuan_seedv2_routes.go:100 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/v2/icon.svg` | upstream/youyuan_seedv2_routes.go:119 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/v2/manifest` | upstream/youyuan_seedv2_routes.go:71 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/v2/meta` | upstream/youyuan_seedv2_routes.go:75 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/v2/slime` | upstream/youyuan_seedv2_routes.go:104 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/v2/stronghold` | upstream/youyuan_seedv2_routes.go:108 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/v2/structures` | upstream/youyuan_seedv2_routes.go:96 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/v2/summary` | upstream/youyuan_seedv2_routes.go:112 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/seed/v2/tile.png` | upstream/youyuan_seedv2_routes.go:92 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/skin/account-head` | upstream/main.go:5197 | — | GET 免 token；写需会话 token | 见 handler |
| 任意 | `/api/skin/account-texture` | upstream/main.go:5198 | — | GET 免 token；写需会话 token | 见 handler |
| 任意 | `/api/skin/build` | upstream/main.go:5187 | b: path | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/head` | upstream/main.go:5193 | q: path | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/item-library` | upstream/youyuan_skin_item.go:665 | b: code,name,by | GET 免 token；写需会话 token | JSON {ok,items} |
| 任意 | `/api/skin/item-texture` | upstream/youyuan_skin_item.go:664 | q: item_id | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/online-build` | upstream/main.go:5192 | b: player | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/online-head` | upstream/main.go:5194 | q: player | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/online-list` | upstream/main.go:5190 | b: name,width,height | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/online-preview` | upstream/main.go:5191 | q: player | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/online-texture` | upstream/main.go:5195 | q: player | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/preview` | upstream/main.go:5188 | q: path | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/proxy` | upstream/youyuan_skin_cache.go:410 | — | GET 免 token；写需会话 token | 见 handler |
| 任意 | `/api/skin/voxel` | upstream/main.go:5189 | q: path | GET 免 token；写需会话 token | JSON {ok,error} |
| 任意 | `/api/skin/warm` | upstream/youyuan_skin_cache.go:411 | — | GET 免 token；写需会话 token | 见 handler |
| 任意 | `/api/system/restart` | upstream/main.go:5244 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/task/delete` | upstream/main.go:5058 | b: task_id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/task/list` | upstream/main.go:5054 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/task/resume` | upstream/main.go:5055 | b: task_id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/task/start` | upstream/main.go:5051 | b: type,params | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/task/stop` | upstream/main.go:5053 | — | 控制台会话 token | JSON {ok} |
| 任意 | `/api/version/check` | upstream/main.go:5175 | — | 控制台会话 token | 见 handler |
| POST | `/api/world/enable` | upstream/main.go:5056 | b: enabled | 控制台会话 token | JSON {ok,error} |
| GET/POST | `/api/youyuan/account-pool` | upstream/youyuan_account_pool.go:656 | q: action,all; b: action,args | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/account/action` | upstream/youyuan_runtime.go:54 | — | 控制台会话 token | 见 handler |
| GET | `/api/youyuan/accounts` | upstream/youyuan_accounts.go:375 | q: view; b: action,account,id,name,profile_role,avatar,defer_connect | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/accounts/bridge/execute` | upstream/youyuan_account_bridge.go:342 | — | 控制台会话 token | 见 handler |
| POST | `/api/youyuan/accounts/bridge/local` | upstream/youyuan_account_bridge.go:357 | — | 控制台会话 token | 文本/空响应 |
| 任意 | `/api/youyuan/accounts/bridge/profiles` | upstream/youyuan_account_bridge.go:335 | — | 控制台会话 token | 文本/空响应 |
| POST | `/api/youyuan/accounts/bridge/rules` | upstream/youyuan_account_bridge.go:297 | — | 控制台会话 token | JSON {ok,rules,accounts} |
| GET | `/api/youyuan/accounts/pool/secret` | upstream/youyuan_account_pool.go:699 | q: id | 控制台会话 token | JSON {ok,id,token} |
| POST | `/api/youyuan/agent/archive` | upstream/youyuan_agent_cleanup.go:78 | b: id | 控制台会话 token | JSON {ok,error} |
| GET | `/api/youyuan/agent/cloud-model` | upstream/youyuan_agent_preferences.go:106 | — | 控制台会话 token | JSON {ok,allowed} |
| GET | `/api/youyuan/agent/conversation` | upstream/youyuan_agent_sessions.go:118 | q: id,before | 控制台会话 token | JSON {ok,error} |
| GET | `/api/youyuan/agent/file` | upstream/youyuan_agent.go:1466 | q: id,path; b: id,path,content,base_hash | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/agent/fork` | upstream/youyuan_agent_fork.go:17 | b: id,turn_index,message | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/agent/mcp` | upstream/youyuan_agent.go:1609 | b: id,action,server | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/agent/model` | upstream/youyuan_agent_preferences.go:195 | b: id,model | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/agent/preferences` | upstream/youyuan_agent_preferences.go:129 | q: id; b: id,custom_prompt,skills,allowed_tools,model | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/agent/run-budget` | upstream/youyuan_agent_budget.go:38 | q: id; b: id,budget | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/agent/save` | upstream/youyuan_agent_sessions.go:93 | b: id | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/agent/session` | upstream/youyuan_agent_sessions.go:15 | b: id,name | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/agent/skills` | upstream/youyuan_skill_library.go:124 | q: id; b: id,skill,enabled,skills | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/agent/sync-plugin` | upstream/youyuan_agent_cleanup.go:96 | b: id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/agent/task` | upstream/youyuan_agent.go:1385 | b: id,action,message,model | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/agent/tool` | upstream/youyuan_agent.go:1432 | b: id,tool,args | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/agent/upload` | upstream/youyuan_agent.go:1509 | q: id,attachment | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/agent/validate` | upstream/youyuan_agent.go:1658 | b: id,path,content,base_hash | 控制台会话 token | JSON {ok,error} |
| GET | `/api/youyuan/agent/workspaces` | upstream/youyuan_agent.go:1281 | q: id,after,compact; b: id,name,language,plugin_id | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/appearance` | upstream/youyuan_appearance.go:55 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/appearance/file` | upstream/youyuan_appearance.go:130 | q: id | 控制台会话 token | 文本/空响应 |
| POST | `/api/youyuan/appearance/upload` | upstream/youyuan_appearance.go:95 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/catalog` | upstream/youyuan_remote.go:140 | — | 控制台会话 token | JSON {ok,routes} |
| GET | `/api/youyuan/chains` | upstream/youyuan_commands.go:747 | q: name; b: action,config,name,content,redstone | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/connection-diagnostics` | upstream/youyuan_remote.go:196 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/youyuan/connection-policy` | upstream/main.go:5098 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/youyuan/convert/command-structure` | upstream/youyuan_converters.go:143 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/youyuan/convert/commands` | upstream/youyuan_converters.go:144 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/convert/structure` | upstream/youyuan_converters.go:227 | b: origin,rotation | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/docs` | upstream/youyuan_docs.go:9 | q: language,method | 控制台会话 token | JSON {ok,resources,usage,query,language} |
| 任意 | `/api/youyuan/events` | upstream/youyuan_remote.go:146 | q: after | 控制台会话 token | JSON {ok,events,cursor} |
| GET | `/api/youyuan/exports` | upstream/youyuan_export_center.go:219 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/exports/download` | upstream/youyuan_export_center.go:251 | — | 能力型 URL（id+token） | 文本/空响应 |
| GET | `/api/youyuan/exports/info` | upstream/youyuan_export_center.go:282 | q: id | 控制台会话 token | JSON {ok,export,download_url,content_type,downloaded} |
| POST | `/api/youyuan/exports/remove` | upstream/youyuan_export_center.go:299 | b: id | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/feedback` | upstream/youyuan_notices.go:99 | b: content,text | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/files` | upstream/youyuan_host_files.go:21 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/files/download` | upstream/youyuan_host_files.go:94 | q: name | 控制台会话 token | 文本/空响应 |
| POST/PUT | `/api/youyuan/files/transfer` | upstream/youyuan_file_upload_sessions.go:372 | q: id,offset,sha256; b: id,name,size | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/files/upload` | upstream/youyuan_host_files.go:37 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/game-auth` | upstream/youyuan_game_auth.go:26 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/game-egress` | upstream/main.go:5101 | — | 控制台会话 token | 见 handler |
| POST | `/api/youyuan/game/call` | upstream/youyuan_capture.go:498 | b: account,method,args | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/global-capture` | upstream/youyuan_global_capture.go:365 | — | 控制台会话 token | JSON {ok,error} |
| GET | `/api/youyuan/identity` | upstream/youyuan_identity.go:112 | b: action,username,email,email_code,password,current_password,new_password,turnstile_token | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/integrity` | upstream/main.go:5013 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/youyuan/library/delete` | upstream/youyuan_runtime.go:58 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/youyuan/library/download` | upstream/youyuan_runtime.go:57 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/youyuan/library/list` | upstream/youyuan_runtime.go:55 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/youyuan/library/upload` | upstream/youyuan_runtime.go:56 | — | 控制台会话 token | 见 handler |
| POST | `/api/youyuan/mail/send` | upstream/youyuan_mail.go:43 | — | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/mcp-links` | upstream/youyuan_mcp_links.go:113 | b: op,token,scope,hours,name | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/mcp/scope` | upstream/youyuan_mcp_scope.go:645 | b: op,id,server,owner,access,enabled | 控制台会话 token | JSON {ok,error} |
| GET | `/api/youyuan/mcp/traces` | upstream/youyuan_mcp_telemetry.go:80 | q: trace_id | 控制台会话 token | JSON {ok,traces,known,retention,boundary} |
| POST | `/api/youyuan/media/block-video` | upstream/youyuan_block_video.go:164 | b: sequence,images,config | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/notices` | upstream/youyuan_notices.go:81 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/observations` | upstream/youyuan_observations.go:68 | — | 控制台会话 token | JSON {ok,connected,online,server,source} |
| POST | `/api/youyuan/observations/export` | upstream/youyuan_observations.go:74 | — | 控制台会话 token | JSON {ok,error} |
| GET | `/api/youyuan/observations/summary` | upstream/youyuan_observation_summary.go:129 | q: account,source | 控制台会话 token | 文本/空响应 |
| POST | `/api/youyuan/observer/ingest` | upstream/youyuan_observer_bridge.go:246 | — | X-Youyuan-Observer-Token | JSON {ok,error} |
| GET | `/api/youyuan/packets/capture` | upstream/youyuan_capture.go:577 | q: account,view; b: action,config | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/packets/export` | upstream/youyuan_capture.go:622 | q: account | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/packets/schema` | upstream/youyuan_capture.go:564 | q: name | 控制台会话 token | JSON {ok,packets} |
| 任意 | `/api/youyuan/player/experience` | upstream/main.go:5100 | — | 控制台会话 token | 见 handler |
| HEAD | `/api/youyuan/plugin-outputs{,/info,/download}` | upstream/youyuan_plugin_outputs.go:127 | q: file | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/remote` | upstream/youyuan_remote.go:95 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/rooms` | upstream/main.go:5099 | — | 控制台会话 token | 见 handler |
| POST | `/api/youyuan/runtime/benchmark` | upstream/youyuan_benchmark.go:142 | b: rounds | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/runtime/delete` | upstream/youyuan_runtime_management.go:66 | b: id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/runtime/event` | upstream/youyuan_runtime.go:168 | b: id,account,event | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/runtime/export` | upstream/youyuan_runtime.go:53 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/youyuan/runtime/files` | upstream/youyuan_runtime_management.go:35 | q: id | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/runtime/import` | upstream/youyuan_runtime.go:52 | — | 控制台会话 token | 见 handler |
| 任意 | `/api/youyuan/runtime/list` | upstream/youyuan_runtime.go:59 | — | 控制台会话 token | JSON {ok,installed,instances,languages} |
| 任意 | `/api/youyuan/runtime/start` | upstream/youyuan_runtime.go:60 | b: id,account,restore | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/runtime/stop` | upstream/youyuan_runtime.go:196 | b: id,account,native_instance | 控制台会话 token | JSON {ok,skipped} |
| POST | `/api/youyuan/sdk/batch` | upstream/youyuan_sdk.go:220 | b: plugin_id,account,method,args,calls | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/sdk/call` | upstream/youyuan_sdk.go:290 | b: plugin_id,account,method,args | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/sdk/catalog` | upstream/youyuan_sdk.go:269 | — | 控制台会话 token | JSON {ok,sdk_version,namespaces,event_callback,binding_lifetime} |
| 任意 | `/api/youyuan/sdk/pages` | upstream/youyuan_sdk.go:256 | — | 控制台会话 token | JSON {ok,pages} |
| 任意 | `/api/youyuan/sense/events` | upstream/youyuan_capture.go:546 | q: account,view | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/sense/snapshot` | upstream/youyuan_capture.go:533 | q: account,view | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/skills/export` | upstream/youyuan_skill_library.go:95 | q: skill | 控制台会话 token | JSON {ok,error} |
| GET | `/api/youyuan/skills/read` | upstream/youyuan_skill_library.go:205 | — | 控制台会话 token | JSON {ok,error} |
| 任意 | `/api/youyuan/snapshot` | upstream/youyuan_remote.go:145 | — | 控制台会话 token | 见 handler |
| GET | `/api/youyuan/social` | upstream/youyuan_social.go:255 | b: action,kind,visits,targets,messages,interval_ms,config | 控制台会话 token | JSON {ok,job,config} |
| 任意 | `/api/youyuan/social/search` | upstream/youyuan_social.go:241 | q: offset,sort_type,keyword | 控制台会话 token | JSON {ok,error} |
| POST | `/api/youyuan/tunnel` | upstream/youyuan_tunnel.go:270 | — | 控制台会话 token | JSON {ok,error} |
| GET | `/api/youyuan/world/seed` | upstream/youyuan_seed_world.go:117 | q: structure,type,edition | 控制台会话 token | 见 handler |
| 任意 | `/blocks/` | upstream/main.go:5029 | — | 无 | 原始字节 |
| 任意 | `/css/` | upstream/main.go:5027 | — | 无 | 原始字节 |
| 任意 | `/draw` | upstream/main.go:5025 | — | 无 | 原始字节 |
| 任意 | `/health` | upstream/youyuan_remote.go:74 | — | 无 | 见 handler |
| 任意 | `/images/` | upstream/main.go:5026 | — | 无 | 原始字节 |
| 任意 | `/js/` | upstream/main.go:5028 | — | 无 | 原始字节 |
| 任意 | `/mcf-syntax.mtsx` | upstream/main.go:5213 | — | 无 | 文本/空响应 |
| 任意 | `/mcp` | upstream/youyuan_remote.go:159 | — | Bearer <远控 Key> | 见 handler |
| 任意 | `/mcp/` | upstream/youyuan_remote.go:160 | — | Bearer <远控 Key> | 见 handler |
| 任意 | `/piano` | upstream/main.go:5043 | — | 无 | 原始字节 |
| 任意 | `/sounds/` | upstream/main.go:5130 | — | 无 | 原始字节 |
| 任意 | `/stream` | upstream/main.go:5049 | — | 无 | 文本/空响应 |
| 任意 | `/uploads/` | upstream/main.go:5131 | — | 无 | 原始字节 |
| 任意 | `/web/` | upstream/main.go:5096 | — | 无 | 见 handler |
| 任意 | `/wizard.css` | upstream/main.go:5030 | — | 无 | 原始字节 |
| 任意 | `/wizard.js` | upstream/main.go:5036 | — | 无 | 原始字节 |
| 任意 | `/ws` | upstream/main.go:5050 | — | 无 | 见 handler |

> 路径后标 (未接线) 的 8 条，其 `yy*Routes()` 注册函数在生产代码里从未被调用（只在 `_test.go` 里调用），线上实际 404，详见 5.4。

## 3. Cloudflare Worker 路由表

Worker 与 Go 服务是两套独立路由。入口 `cloudflare/worker.mjs`，子模块 `identity.mjs` / `oauth.mjs` / `updates.mjs` / `private-distribution.mjs` / `mcp-discovery.mjs`。
Durable Object `YouYuanRelay`（`worker.mjs:152` 起）承担 `/api/*` 与 `/mcp*` 到执行端的 WebSocket 转发。

| 方法 | 路径 | 文件:行 | 入参 | 鉴权 | 返回要点 |
| --- | --- | --- | --- | --- | --- |
| GET/HEAD | `/updates/android/latest.json` | cloudflare/updates.mjs:7 | — | 无（公开） | 版本清单 JSON，no-cache |
| GET/HEAD | `/updates/android/latest.apk` | cloudflare/updates.mjs:7 | header: Range / If-Range | 无（公开） | 分片 APK，支持 206 |
| GET | `/(updates 专用域根)` | cloudflare/updates.mjs:6 | — | 无 | 下载引导页 HTML |
| GET | `/.well-known/oauth-protected-resource[/mcp/oauth/<device>]` | cloudflare/oauth.mjs:34 | path: device | 无（RFC 9728 公开元数据） | resource / scopes |
| GET | `/.well-known/oauth-authorization-server` | cloudflare/oauth.mjs:35 | — | 无 | AS 元数据（S256、code、refresh） |
| POST | `/oauth/register` | cloudflare/oauth.mjs:47 | b: redirect_uris,client_name | 无（DCR，30/h/IP） | client_id 等 201 |
| GET | `/oauth/authorize` | cloudflare/oauth.mjs:55 | q: client_id,redirect_uri,code_challenge,resource,scope,state | 无（校验 client + PKCE） | 授权同意页 HTML |
| POST | `/oauth/authorize` | cloudflare/oauth.mjs:70 | b(form): transaction,consent,username,password | 账号密码（表单） | 302 带 code |
| POST | `/oauth/token` | cloudflare/oauth.mjs:81 | b(form): grant_type,code,code_verifier,refresh_token,client_id,resource | client_id + PKCE | access/refresh token（1h/30d） |
| POST | `/oauth/revoke` | cloudflare/oauth.mjs:99 | b(form): token,client_id | client_id | {} |
| GET | `/api/identity/turnstile-config` | cloudflare/identity.mjs:26 | — | 无（CORS *） | enabled / site_key |
| GET | `/api/identity/pow-challenge` | cloudflare/identity.mjs:32 | — | 无（CORS *） | PoW 挑战 |
| POST | `/api/identity/{register-code,register,login,me,binding,logout,claim,delete,users,entitlement,password,reset-password,mcp-links,backup,delete-user}` | cloudflare/identity.mjs:38 | b: username,password,email,email_code,op,user_id,new_password,role,subscription_until,assigned_devices,device_key | Bearer session_token / control_token；users|entitlement|backup|delete-user 需 owner 或 admin | session_token / user / links |
| GET | `/api/cloud/ai-capability` | cloudflare/worker.mjs:48 | — | Bearer（owner / legacy device / identity） | {ok,allowed} |
| 任意 | `/auth/<path>` | cloudflare/worker.mjs:50 | path 白名单 7 个；header X-YouYuan-Device-Key | 设备密钥（DEVICE_TOKEN 或 identity device） | 转发 auth.youyuanqi.dpdns.org |
| POST | `/ai/v1/chat/completions` | cloudflare/worker.mjs:60 | b: messages,stream,max_tokens,tools | Bearer（仅测试账号 / owner） | OpenAI 兼容 completion |
| POST | `/device/connect` | cloudflare/worker.mjs:80 | header: Upgrade=websocket | Bearer DEVICE_TOKEN（WS 升级） | 101 + pool_hello |
| POST | `/qq/webhook` | cloudflare/worker.mjs:81 | b: QQ 事件 JSON | 无（qqCallback，转 primary-device） | 转发 /api/qq/webhook |
| POST | `/qq/webhook/<uuid>/<slug>[/<account>]` | cloudflare/worker.mjs:81 | path: 租户 uuid / slug / account | 无（qqCallback） | 转发租户执行端 |
| POST | `/qq/webhook/accounts/<id>` | cloudflare/worker.mjs:82 | path: account | 无（qqCallback） | 转发 primary-device |
| GET | `/api/cloud/status` | cloudflare/worker.mjs:103 | — | Bearer 或 identity | 产品/版本/user |
| GET | `/api/cloud/linux-direct` | cloudflare/worker.mjs:110 | — | owner console token 或 legacy_owner session | {use_relay} 或 {origin} |
| GET | `/api/game-auth/accounts` | cloudflare/worker.mjs:99 | — | Bearer（受保护路径） | 转发 AUTH_SERVICE /api/accounts |
| GET | `/api/library/list` | cloudflare/worker.mjs:198 | — | Bearer（受保护路径） | items 列表 |
| POST | `/api/library/upload` | cloudflare/worker.mjs:202 | b(form): file,category | Bearer（DEVICE_TOKEN 可直通） | item 元数据 |
| GET | `/api/library/download` | cloudflare/worker.mjs:218 | q: id | Bearer（DEVICE_TOKEN 可直通） | 二进制 + X-Content-SHA256 |
| POST | `/api/library/delete` | cloudflare/worker.mjs:223 | q: id | Bearer（DEVICE_TOKEN 可直通） | {ok} |
| POST | `/mcp/oauth/<device>` | cloudflare/worker.mjs:19 | b: JSON-RPC | OAuth access token（mcp:read / mcp:control）；linux-server 的 initialize/ping/tools/list 可匿名 | JSON-RPC；转发 /mcp/ |
| 任意 | `/mcp/link/<64hex>` | cloudflare/worker.mjs:33 | path: link token | 能力型 URL（identity mcp-link，带 scope） | 转发 /mcp/ |
| POST | `/mcp, /mcp/` | cloudflare/worker.mjs:86 | b: JSON-RPC | Bearer CONSOLE_TOKEN / 设备令牌 / identity | JSON-RPC 结果 |
| GET | `/health` | cloudflare/worker.mjs:86 | — | Bearer CONSOLE_TOKEN | 执行端探活 |
| 任意 | `/api/*（其余全部）` | cloudflare/worker.mjs:86 | 同 Go 侧 | Bearer CONSOLE_TOKEN 或 identity session | 经 DO relay 转发到执行端 |
| 任意 | `/__identity/*, /__oauth/*` | cloudflare/worker.mjs:162 | — | 仅 DO 内部 | 内部路由，公网不可达 |

Worker 侧三个要点：

1. `isPrivateDistributionPath`（`private-distribution.mjs:3`）在最前面把源码/开发者归档路径直接 404（`.go`、`.mjs`、`.map`、`.git`、`source.zip` 等），
   但第 8 行显式放行 `api` 与 `mcp` 开头——「用户自己的代码可导出，发行包源码不可下载」。
2. 受保护路径判定在 `worker.mjs:86`：只有 `/device/connect`、`/api/*`、`/mcp*`、`/health` 需要凭证，其余落到 `env.ASSETS.fetch`。
3. 能力型免登录下载：路径 `/api/youyuan/exports/download` + `token` 长度 ≥8 + `id` 为 32 位十六进制（`worker.mjs:85`）时完全跳过 Bearer 校验，直通 owner 兜底下载。

## 4. 鉴权模型说明

全项目只有 **5 类**鉴权，分属两个进程，互相不能替代。先给总表，细节见 4.1 / 4.2 / 4.3：

| # | 类别 | 落在哪 | 典型路径 | 带什么 |
|---|---|---|---|---|
| 1 | **无鉴权** | Go + Worker | `/`、`/health`、静态资源、`/api/identity/turnstile-config` | — |
| 2 | **`Authorization: Bearer <control_token>`** | Worker（云端） | `/api/*`、`/mcp*`、`/health`（云端入口） | `env.CONSOLE_TOKEN` 或 `env.DEVICE_TOKEN` 或 identity session |
| 3 | **控制台会话 token** | Go（本机） | 本机 `/api/*` 全部（除豁免） | `X-YouYuan-Console-Token`（前端 fetch 包装自动加） |
| 4 | **设备头 `X-YouYuan-Device`** | Worker（云端） | 所有需要转发的 `/api/*` | 设备**标识**（路由用）+ 配套 `X-YouYuan-Device-Key`（真凭证） |
| 5 | **能力型 URL（带 token 免登录）** | Go + Worker | `/api/youyuan/exports/download`、`/mcp/link/<64hex>` | URL 里的 `token` + `id` |

一句话区分：#2 是「云端的门」，#3 是「本机的门」，#4 决定「进哪台机器的门」，#5 是「门都不要，凭票入场」。
**带了 #3 不会自动满足 #2/#4**，反之亦然。

### 4.1 本机 Go 服务的四种凭证

| 凭证 | 携带方式 | 从哪来 | 适用范围 | 失效表现 |
| --- | --- | --- | --- | --- |
| **控制台会话 token** | `X-YouYuan-Console-Token` / `Authorization: Bearer <t>` / `?token=<t>` | 进程启动时 `crypto/rand` 32 字节 hex（`youyuan_console_auth.go:38`）；`handleIndex` 注入 `<meta name=yy-console-token>`，前端 `youyuan_console_token.js` 包装 `window.fetch` 自动加头 | 除豁免清单外的全部 `/api/*` | 401 `{ok:false,error:"控制台会话已失效，请重新打开工具箱"}`；进程重启即换新 token |
| **远控 key** | `Authorization: Bearer <yyRemote.Key>` | 远控配置 `yyRemote.Key`（`/api/youyuan/remote` 响应里的 `key` 字段） | `/mcp` 与 `/mcp/`（MCP JSON-RPC）；**同时被 `yyConsoleRequestAllowed` 当作控制台 token 的等价物接受**（`youyuan_console_auth.go:75-79`） | MCP 侧 401 `MCP 密钥不正确` |
| **互通令牌** | `X-Bridge-Token` / `Authorization: Bearer` / `?token=` | 群服互通设置里的 MCP 令牌（`qq-bridge.json` 的 `assistant.mcp_token`） | 仅 `/api/bridge/*`（`youyuan_bridge_api.go:283`） | 未配置令牌时**一律拒绝**（`yyBridgeTokenOK` 直接 return false），并回可执行提示 |
| **观测源令牌** | `X-Youyuan-Observer-Token` | 观测桥源登记时下发的 64 位 token，服务端只存 sha256（`youyuan_observer_bridge.go:271`） | 仅 `/api/youyuan/observer/ingest` | `{ok:false,error:...}`（HTTP 200 里带错误） |

### 4.2 会话鉴权的豁免清单（`youyuan_console_auth.go:87-120`）

按代码顺序，以下请求**不校验**控制台 token 就进入 mux：

| 豁免 | 代码位置 | 理由（源码注释） | 风险 |
| --- | --- | --- | --- |
| 所有非 `/api` 前缀路径 | 第 90 行 | 静态资源、`/health`、`/mcp` 各自有独立机制 | `/mcp` 靠远控 key，其余确实无鉴权 |
| `OPTIONS` 预检 | 第 95 行 | 预检带不了自定义头，跨站由 CORS 层 403 | 低 |
| `/api/youyuan/exports/download` | 第 100 行 | 能力型 URL，handler 内校验 id + token | 中：token 会进浏览器历史/Referer |
| GET/HEAD `/api/skin/*` | 第 106-110 行 | `<img src>` 带不了自定义头 | 中：任意网页可探测本机头像/皮肤 |
| GET/HEAD `/api/market-proxy/*` | 第 106-110 行 | 同上，市场预览图 | 低 |

另有两条不在中间件、而在 handler 内的独立鉴权：

- `/api/qq/webhook`：Ed25519 签名校验（`youyuan_qq.go:926`），签名头 `X-Signature-Ed25519` / `X-Signature-Timestamp` / `X-Bot-Appid`。
- `/api/bridge/*`：互通令牌（见 4.1），与会话 token **互不替代**——带了会话 token 但没有 `X-Bridge-Token` 依然 401。

### 4.3 第五类：云端 Worker 的设备头与会话（`cloudflare/worker.mjs`）

本机 Go 服务只有「回环 + 会话 token」；**设备维度只存在于 Worker**，两类东西容易混：

| 头 / 参数 | 是不是凭证 | 作用 | 位置 | 失效表现 |
|---|---|---|---|---|
| `X-YouYuan-Device`（或 `?device=`） | **不是**，是**路由头** | 决定这条请求转发到哪台执行端（`primary-device` / `linux-server` / 租户设备） | `worker.mjs:104` | 不匹配 `^[A-Za-z0-9_-]{1,48}$` → 400 `{"error":"设备标识无效"}`（`:105`） |
| `X-YouYuan-Device-Key` | 是 | 设备密钥，只用于 `/auth/<path>` 白名单转发 | `worker.mjs:51-52` | 401 `{"error":"设备密钥不正确"}` |
| `Authorization: Bearer <CONSOLE_TOKEN>` | 是 | 云端控制台共享密钥（`env.CONSOLE_TOKEN`） | `worker.mjs:92` | 401 `{"ok":false,"error":"登录状态已失效，请重新登录","code":"AUTH_REQUIRED"}` |
| `Authorization: Bearer <DEVICE_TOKEN>` | 是 | 遗留设备令牌（`env.DEVICE_TOKEN`），可直通 `/api/library/*` | `worker.mjs:43,94` | 同上 |
| `Authorization: Bearer <identity session>` | 是 | 账号体系登录态，Worker 内部拿它请求 `/api/identity/me` 换取 `identity.user` | `worker.mjs:119` | 同上；`/mcp*` 改为 `mcpChallenge` 挑战响应 |

两条附加闸门，与「有没有 token」是**独立**的两回事：

1. **订阅闸门**：`protectedPath && identity && !identity.user.cloud_allowed` → 403
   `{"ok":false,"error":"云端工具箱仅限管理员或有效订阅用户","code":"SUBSCRIPTION_REQUIRED"}`（`worker.mjs:88`）。
   即「登录成功」不等于「能用云端执行」。
2. **能力型下载**：`capDownload`（`worker.mjs:85`）= 路径为 `/api/youyuan/exports/download`
   + `token` 长度 ≥ 8 + `id` 为 32 位 hex → 完全跳过 Bearer 校验（box 侧另有 id 校验）。

转发到执行端时，Worker 会**删掉** `X-YouYuan-Device`（`worker.mjs:29,38`）并注入
`X-YouYuan-Actor` / `X-YouYuan-Actor-Signature` / `X-YouYuan-Trace-ID`（`:192`），
执行端据此认人而不是认设备头——所以**伪造设备头不会提权**，只会把请求打到另一台机器。

---

## 5. 前端实际在调哪些接口（与后端表做差集）

### 5.1 口径

| 项 | 值 |
|---|---|
| 扫描范围 | `upstream/frontend/js/**/*.js`（含 `features/` `pages/` `vendor/` 三个子目录），排除 `.js.esb` 打包副本 |
| 文件数 | 顶层 76 个 `.js` + `features/` 30 + `pages/` 5 + `vendor/` 3 |
| 提取方式 | 正则抓 `/api/...` 字符串字面量（单/双/反引号），去 `?query`，去重 |
| 结果 | **309** 个唯一字面量（含 `/api/`、`/api/...` 两个占位） |
| 匹配规则 | 完全相等；或后端为前缀路由（`/api/x/`）；或后端含 `{param}` / `<param>` 时按段通配；或前端只写到父路径（`/api/x` 对 `/api/x/list`） |

### 5.2 结论一：前端在调、后端没有 → **0 条**

307 个有效字面量**全部**能对到 `upstream/*.go` 里的 `mux.HandleFunc` 注册。
这条结论很重要：**不存在「前端点了按钮但接口根本不存在」的现网死链**。

### 5.3 结论二：后端有、前端没调 → 17 条，且全部不是 `/api`

| 路径 | 性质 | 前端实际用法 |
|---|---|---|
| `/` `/draw` `/piano` | 页面 | 浏览器直接导航 |
| `/images/` `/css/` `/js/` `/blocks/` `/sounds/` `/uploads/` `/web/` | 静态目录 | `<link>` / `<script src>` / `<img>` |
| `/wizard.css` `/wizard.js` `/mcf-syntax.mtsx` | 单文件静态 | 同上 |
| `/stream` | SSE | `EventSource`（`sse.js`），不是 `fetch` |
| `/ws` | WebSocket | `new WebSocket`（悬浮窗） |
| `/health` | 探活 | 云端 / 运维脚本调，前端不用 |
| `/mcp` `/mcp/` | MCP JSON-RPC | 外部 AI 客户端调，前端只展示配置 |

即：**`/api/*` 范围内，「后端注册了但前端从不调」的条数是 0**。

### 5.4 结论三：8 条「源码里注册了、运行时 404」的死路由

有 3 个 `yy*Routes` 函数**全仓库零调用点**（只在 `_test.go` 里被调），它们注册的 8 条路径线上永远是 404：

| 注册函数 | 文件 | 覆盖路径 |
|---|---|---|
| `yyLevelRoutes` | `youyuan_qq_level.go` | `/api/qq/level`、`/api/qq/level/top` |
| `yyQQRagRoutes` | `youyuan_qq_rag.go` | `/api/qq/rag/docs`、`/api/qq/rag/reindex`、`/api/qq/rag/search`、`/api/qq/rag/stats` |
| `yyQQUsageRoutes` | `youyuan_qq_usage.go` | `/api/qq/usage`、`/api/qq/usage/context` |

好消息：**前端也没在调这 8 条**（5.1 的 309 个字面量里没有任何 `/api/qq/level` 或 `/api/qq/usage`），
所以是纯死代码，不会造成现网报错；但表里已按源码如实列出并标「未接线」，不要当成可用能力。

### 5.5 结论四：表外还有 4 条真实入口（源码不是字面量）

| 入口 | 文件:行 | 说明 |
|---|---|---|
| `/api/chains/list` | `main.go:5015-5022` | `for alias, target := range map[...]` 循环注册，内部 `r.Clone` + 改 `u.Path` 后回灌 mux，等价 301 内部重写 |
| `/api/youyuan/skills` | 同上 | 别名 → `/api/ai/skills` |
| `/api/youyuan/skills/list` | 同上 | 别名 → `/api/ai/skills` |
| `/api/youyuan/accounts/invoke/` | `youyuan_accounts.go:45`（`yyAccountsPrefix`），注册在 `:318` | 子树路由（结尾带 `/`），匹配该前缀下任意子路径 |

因此**运行时实际注册数** = 384 条字面量 − 1（`plugin-outputs` 那条在 `for range` 里，展开 3 条，净 +2）
+ 3（别名）+ 1（前缀）= **390 条**。

---

## 6. 覆盖对账

- `grep -h 'mux\.HandleFunc(' upstream/*.go`（排除 `_test.go`）= **386** 个调用点
  - 其中 384 个是路径字面量 → 第 2 节表里 384 条，一一对应
  - 2 个不是字面量：`main.go:5016`（别名循环，展开 3 条）、`youyuan_accounts.go:318`（子树前缀，1 条）
  - 384 条字面量里 `youyuan_plugin_outputs.go:127` 在 `for range` 中，展开 3 条（净 +2）
- Cloudflare Worker 侧另计 33 条（第 3 节）

**源码共 384 处注册，本表 384 条**；连同 4 条非字面量入口，运行时共 390 条。
