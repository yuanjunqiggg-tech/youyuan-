# 游园工具箱 / MCORT · MCP 面清单

> 审计对象：`C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project`（源码版本 **0.10.78**，`upstream/youyuan_product.go:14`）
> 生成时间：2026-10-26 · 只读审计：未连接线上服务、未改动源码、未重启
> 对外地址示例：`https://toolbox.youyuanqi.dpdns.org/mcp/?device=linux-server`（Streamable HTTP）

## 0. 一页结论

| 项 | 事实 | 依据 |
|---|---|---|
| 传输 | Streamable HTTP，只接受 POST；GET→405、DELETE→200、通知→202 | `upstream/youyuan_remote.go:253-307` |
| 协议版本 | 仅 `2025-03-26` / `2025-06-18` | `upstream/youyuan_remote.go:495-497` |
| tools 总数 | **315** = 313 条 API 路由 + `api_catalog` + `api_call` | `upstream/youyuan_catalog.go:4-316`、`upstream/youyuan_remote.go:519-520` |
| resources 总数 | **39**，另 1 个 URI 模板 | `upstream/youyuan_mcp_resources.go:11-40`、`upstream/youyuan_remote.go:338-339` |
| 鉴权 | 三套互不通用：本机 MCP 密钥 / 平台账号云端控制密钥 / 专用连接 URL；游戏令牌与模型 Key 都不能当 MCP 密钥 | 见 §4 |
| 白名单 | 只有两级 scope（read / control），没有逐 tool 开关；只读集合硬编码 | 见 §5 |
| 自检锚点 | `GET /health` 返回 `tools = len(yyAPIRoutes)+2` | `upstream/youyuan_remote.go:84` |

## 1. 端点与传输

| 路径 | 用途 | 源码 |
|---|---|---|
| `/mcp`、`/mcp/` | 本机/中继主入口，单次 JSON-RPC POST | `upstream/youyuan_remote.go:159-160`、`:234` |
| `/mcp/link/{64hex}` | 本机「MCP 专用连接」，URL 自带凭证 | `upstream/youyuan_remote.go:240-248`、`upstream/youyuan_mcp_links.go:99-108` |
| `/mcp/oauth/{device}` | 云端 OAuth 入口（如 `linux-server`） | `cloudflare/worker.mjs:19-31` |
| `/mcp/?device=linux-server` | 云端中继，按 device 分桶转 WebSocket | `cloudflare/worker.mjs:104-109`、`:142` |
| `http://127.0.0.1:19429/mcp/` | 同机直连（仅回环） | `upstream/libmain.go:143`、`upstream/main_native.go:19` |

`initialize` 的 capabilities：`tools.listChanged=false`、`resources.subscribe=false`、`resources.listChanged=false`；`serverInfo.name="游园工具箱 MCP"`（`upstream/youyuan_remote.go:333`）。

## 2. tools 全量清单（315 条）

**入参约定**：除特例外共用一份 inputSchema（`upstream/youyuan_remote.go:500`）：

```json
{"method":"GET|POST|PUT|DELETE（默认GET）","account":"独立账号ID，留空=主账号","query":{},"body":{}}
```

- `api_youyuan_docs` 专用：`query{language,method,section,catalog,full}`（`:507-509`）；`api_catalog` 无参数；`api_call` 必填 `path`（`:519-520`）。
- `account != main` 时路径改写为 `/api/youyuan/accounts/invoke/{account}/...`（`:417-423`）。
- `GET` 调 `exports/download`、`plugin-outputs/download` 自动改写 `/info`（`:426-431`）；`GET` 调 `sense/*`、`packets/capture` 自动补 `view=summary`（`:435-437`）。

**「写操作」判定**：① 在 `yyMCPReadPaths` 内→`否`；② 否则 handler 含 POST/PUT/DELETE 分支→`是`；③ handler 不校验 method 的按路径语义判定并人工修正（§5.4）。

> 大量 handler 不检查 `r.Method`（例：`handlePluginFileWrite` 直接解 body 写文件，`upstream/main.go:944`），**不能**用「发 GET 就安全」推断只读；唯一可靠边界是 §5 的白名单。

| # | 工具名 | API 路径 | 入参 | 写操作 | 注册位置 | 处理函数 | 白名单 |
|---|---|---|---|---|---|---|---|
| 1 | `api_ai_answer` | `/api/ai/answer` | 通用 | 是 | `youyuan_catalog.go:4` | `ai_handlers.go:254` | — |
| 2 | `api_ai_chat` | `/api/ai/chat` | 通用 | 是 | `youyuan_catalog.go:5` | `ai_chat.go:39` | — |
| 3 | `api_ai_models` | `/api/ai/models` | 通用 | 是 | `youyuan_catalog.go:6` | `ai_handlers.go:25` | — |
| 4 | `api_ai_models_fetch` | `/api/ai/models/fetch` | 通用 | 是 | `youyuan_catalog.go:7` | `ai_handlers.go:305` | — |
| 5 | `api_ai_sessions` | `/api/ai/sessions` | 通用 | 是 | `youyuan_catalog.go:8` | `ai_handlers.go:127` | — |
| 6 | `api_ai_sessions_load` | `/api/ai/sessions/load` | 通用 | 是 | `youyuan_catalog.go:9` | `ai_handlers.go:189` | — |
| 7 | `api_ai_sessions_recent` | `/api/ai/sessions/recent` | 通用 | 否 | `youyuan_catalog.go:10` | `ai_handlers.go:161` | — |
| 8 | `api_ai_sessions_rename` | `/api/ai/sessions/rename` | 通用 | 是 | `youyuan_catalog.go:11` | `ai_handlers.go:172` | — |
| 9 | `api_ai_skills` | `/api/ai/skills` | 通用 | 是 | `youyuan_catalog.go:12` | `ai_handlers.go:273` | — |
| 10 | `api_ai_snapshot` | `/api/ai/snapshot` | 通用 | 是 | `youyuan_catalog.go:13` | `ai_handlers.go:206` | — |
| 11 | `api_ai_snapshot_restore` | `/api/ai/snapshot/restore` | 通用 | 是 | `youyuan_catalog.go:14` | `ai_handlers.go:237` | — |
| 12 | `api_announcements` | `/api/announcements` | 通用 | 否 | `youyuan_catalog.go:15` | `youyuan_product.go:64` | — |
| 13 | `api_bot_connect` | `/api/bot/connect` | 通用 | 是 | `youyuan_catalog.go:16` | `main.go:1039` | — |
| 14 | `api_bot_console` | `/api/bot/console` | 通用 | 是 | `youyuan_catalog.go:17` | `main.go:1246` | — |
| 15 | `api_bot_disconnect` | `/api/bot/disconnect` | 通用 | 是 | `youyuan_catalog.go:18` | `main.go:1150` | — |
| 16 | `api_bot_status` | `/api/bot/status` | 通用 | 否 | `youyuan_catalog.go:19` | `main.go:1164` | 本机只读 |
| 17 | `api_building_analyze` | `/api/building/analyze` | 通用 | 是 | `youyuan_catalog.go:20` | `main.go:1874` | — |
| 18 | `api_building_stats` | `/api/building/stats` | 通用 | 否 | `youyuan_catalog.go:21` | `main.go:1981` | — |
| 19 | `api_building_voxel` | `/api/building/voxel` | 通用 | 是 | `youyuan_catalog.go:22` | `main.go:1998` | — |
| 20 | `api_cache_clear` | `/api/cache/clear` | 通用 | 是 | `youyuan_catalog.go:23` | `main.go:4778` | — |
| 21 | `api_camera_clear` | `/api/camera/clear` | 通用 | 是 | `youyuan_catalog.go:24` | `camera_handler.go:399` | — |
| 22 | `api_camera_generate` | `/api/camera/generate` | 通用 | 是 | `youyuan_catalog.go:25` | `camera_handler.go:220` | — |
| 23 | `api_camera_pivot` | `/api/camera/pivot` | 通用 | 是 | `youyuan_catalog.go:26` | `camera_handler.go:135` | — |
| 24 | `api_camera_shot_delete` | `/api/camera/shot/delete` | 通用 | 是 | `youyuan_catalog.go:27` | `camera_handler.go:198` | — |
| 25 | `api_camera_shot_update` | `/api/camera/shot/update` | 通用 | 是 | `youyuan_catalog.go:28` | `camera_handler.go:171` | — |
| 26 | `api_camera_shots` | `/api/camera/shots` | 通用 | 是 | `youyuan_catalog.go:29` | `camera_handler.go:163` | — |
| 27 | `api_camera_start` | `/api/camera/start` | 通用 | 是 | `youyuan_catalog.go:30` | `camera_handler.go:82` | — |
| 28 | `api_camera_status` | `/api/camera/status` | 通用 | 否 | `youyuan_catalog.go:31` | `camera_handler.go:129` | — |
| 29 | `api_camera_stop` | `/api/camera/stop` | 通用 | 是 | `youyuan_catalog.go:32` | `camera_handler.go:111` | — |
| 30 | `api_camera_test` | `/api/camera/test` | 通用 | 是 | `youyuan_catalog.go:33` | `camera_handler.go:266` | — |
| 31 | `api_camera_test_all` | `/api/camera/test/all` | 通用 | 是 | `youyuan_catalog.go:34` | `camera_handler.go:338` | — |
| 32 | `api_cb_export_save` | `/api/cb/export/save` | 通用 | 是 | `youyuan_catalog.go:35` | `cb_export.go:667` | — |
| 33 | `api_cb_export_scan` | `/api/cb/export/scan` | 通用 | 是 | `youyuan_catalog.go:36` | `cb_export.go:114` | — |
| 34 | `api_cb_import_preview` | `/api/cb/import/preview` | 通用 | 否 | `youyuan_catalog.go:37` | `cb_import.go:532` | — |
| 35 | `api_cb_import_start` | `/api/cb/import/start` | 通用 | 是 | `youyuan_catalog.go:38` | `cb_import.go:624` | — |
| 36 | `api_cb_stats` | `/api/cb/stats` | 通用 | 否 | `youyuan_catalog.go:39` | `cb_stats.go:49` | — |
| 37 | `api_cb_stats_preview` | `/api/cb/stats/preview` | 通用 | 否 | `youyuan_catalog.go:40` | `cb_stats.go:187` | — |
| 38 | `api_config` | `/api/config` | 通用 | 是 | `youyuan_catalog.go:41` | `main.go:597` | — |
| 39 | `api_draw_apply` | `/api/draw/apply` | 通用 | 是 | `youyuan_catalog.go:42` | `draw_handlers.go:205` | — |
| 40 | `api_draw_close` | `/api/draw/close` | 通用 | 是 | `youyuan_catalog.go:43` | `draw_handlers.go:424` | — |
| 41 | `api_draw_create` | `/api/draw/create` | 通用 | 是 | `youyuan_catalog.go:44` | `draw_handlers.go:93` | — |
| 42 | `api_draw_custom` | `/api/draw/custom` | 通用 | 是 | `youyuan_catalog.go:45` | `draw_handlers.go:430` | — |
| 43 | `api_draw_mode` | `/api/draw/mode` | 通用 | 是 | `youyuan_catalog.go:46` | `draw_handlers.go:184` | — |
| 44 | `api_draw_palette` | `/api/draw/palette` | 通用 | 是 | `youyuan_catalog.go:47` | `draw_handlers.go:323` | — |
| 45 | `api_draw_redo` | `/api/draw/redo` | 通用 | 是 | `youyuan_catalog.go:48` | `draw_handlers.go:174` | — |
| 46 | `api_draw_state` | `/api/draw/state` | 通用 | 否 | `youyuan_catalog.go:49` | `draw_handlers.go:414` | — |
| 47 | `api_draw_stroke` | `/api/draw/stroke` | 通用 | 是 | `youyuan_catalog.go:50` | `draw_handlers.go:135` | — |
| 48 | `api_draw_sync` | `/api/draw/sync` | 通用 | 是 | `youyuan_catalog.go:51` | `draw_handlers.go:221` | — |
| 49 | `api_draw_undo` | `/api/draw/undo` | 通用 | 是 | `youyuan_catalog.go:52` | `draw_handlers.go:164` | — |
| 50 | `api_file_upload` | `/api/file/upload` | 通用 | 是 | `youyuan_catalog.go:53` | `main.go:1918` | — |
| 51 | `api_files_read` | `/api/files/read` | 通用 | 否 | `youyuan_catalog.go:54` | `main.go:4854` | — |
| 52 | `api_files_scan` | `/api/files/scan` | 通用 | 是 | `youyuan_catalog.go:55` | `main.go:4562` | — |
| 53 | `api_files_write` | `/api/files/write` | 通用 | 是 | `youyuan_catalog.go:56` | `main.go:4887` | — |
| 54 | `api_fleet_connect` | `/api/fleet/connect` | 通用 | 是 | `youyuan_catalog.go:57` | `main.go:1210` | — |
| 55 | `api_fleet_disconnect` | `/api/fleet/disconnect` | 通用 | 是 | `youyuan_catalog.go:58` | `main.go:1224` | — |
| 56 | `api_fleet_status` | `/api/fleet/status` | 通用 | 否 | `youyuan_catalog.go:59` | `main.go:1229` | — |
| 57 | `api_fly_down` | `/api/fly/down` | 通用 | 否 | `youyuan_catalog.go:60` | `main.go:1412` | — |
| 58 | `api_fly_jump` | `/api/fly/jump` | 通用 | 是 | `youyuan_catalog.go:61` | `main.go:1321` | — |
| 59 | `api_fly_jump_start` | `/api/fly/jump-start` | 通用 | 是 | `youyuan_catalog.go:62` | `main.go:1334` | — |
| 60 | `api_fly_jump_stop` | `/api/fly/jump-stop` | 通用 | 是 | `youyuan_catalog.go:63` | `main.go:1347` | — |
| 61 | `api_fly_look` | `/api/fly/look` | 通用 | 是 | `youyuan_catalog.go:64` | `main.go:1425` | — |
| 62 | `api_fly_move` | `/api/fly/move` | 通用 | 是 | `youyuan_catalog.go:65` | `main.go:1453` | — |
| 63 | `api_fly_position` | `/api/fly/position` | 通用 | 否 | `youyuan_catalog.go:66` | `main.go:1583` | — |
| 64 | `api_fly_press` | `/api/fly/press` | 通用 | 是 | `youyuan_catalog.go:67` | `main.go:1532` | — |
| 65 | `api_fly_release` | `/api/fly/release` | 通用 | 是 | `youyuan_catalog.go:68` | `main.go:1570` | — |
| 66 | `api_fly_sneak_start` | `/api/fly/sneak-start` | 通用 | 是 | `youyuan_catalog.go:69` | `main.go:1360` | — |
| 67 | `api_fly_sneak_stop` | `/api/fly/sneak-stop` | 通用 | 是 | `youyuan_catalog.go:70` | `main.go:1373` | — |
| 68 | `api_fly_sprint_start` | `/api/fly/sprint-start` | 通用 | 是 | `youyuan_catalog.go:71` | `main.go:1386` | — |
| 69 | `api_fly_sprint_stop` | `/api/fly/sprint-stop` | 通用 | 是 | `youyuan_catalog.go:72` | `main.go:1399` | — |
| 70 | `api_fly_start` | `/api/fly/start` | 通用 | 是 | `youyuan_catalog.go:73` | `main.go:1295` | — |
| 71 | `api_fly_status` | `/api/fly/status` | 通用 | 否 | `youyuan_catalog.go:74` | `main.go:1282` | — |
| 72 | `api_fly_stop` | `/api/fly/stop` | 通用 | 是 | `youyuan_catalog.go:75` | `main.go:1308` | — |
| 73 | `api_fly_teleport` | `/api/fly/teleport` | 通用 | 是 | `youyuan_catalog.go:76` | `main.go:1510` | — |
| 74 | `api_heartbeat_enable` | `/api/heartbeat/enable` | 通用 | 是 | `youyuan_catalog.go:77` | `main.go:1491` | — |
| 75 | `api_integrity_check` | `/api/integrity/check` | 通用 | 否 | `youyuan_catalog.go:78` | `youyuan_product.go:67` | — |
| 76 | `api_itemmaker_generate` | `/api/itemmaker/generate` | 通用 | 是 | `youyuan_catalog.go:79` | `itemmaker_handler.go:23` | — |
| 77 | `api_map_display` | `/api/map/display` | 通用 | 是 | `youyuan_catalog.go:80` | `main.go:4033` | — |
| 78 | `api_map_grid` | `/api/map/grid` | 通用 | 否 | `youyuan_catalog.go:81` | `main.go:4123` | — |
| 79 | `api_map_play` | `/api/map/play` | 通用 | 是 | `youyuan_catalog.go:82` | `main.go:4068` | — |
| 80 | `api_map_scan` | `/api/map/scan` | 通用 | 是 | `youyuan_catalog.go:83` | `main.go:3987` | — |
| 81 | `api_map_status` | `/api/map/status` | 通用 | 否 | `youyuan_catalog.go:84` | `youyuan_map_video.go:46` | — |
| 82 | `api_map_stop` | `/api/map/stop` | 通用 | 是 | `youyuan_catalog.go:85` | `main.go:4113` | — |
| 83 | `api_mapart_preview` | `/api/mapart/preview` | 通用 | 否 | `youyuan_catalog.go:86` | `main.go:1682` | — |
| 84 | `api_market_proxy` | `/api/market-proxy/` | 通用 | 是 | `youyuan_catalog.go:87` | `main.go:2079` | — |
| 85 | `api_market_contribute` | `/api/market/contribute` | 通用 | 是 | `youyuan_catalog.go:88` | `main.go:2199` | — |
| 86 | `api_market_download` | `/api/market/download` | 通用 | 是 | `youyuan_catalog.go:89` | `main.go:2161` | — |
| 87 | `api_market_plugins` | `/api/market/plugins` | 通用 | 否 | `youyuan_catalog.go:90` | `youyuan_market.go:390` | 本机只读 |
| 88 | `api_market_plugin` | `/api/market/plugins/` | 通用 | 是 | `youyuan_catalog.go:91` | `youyuan_market.go:673` | — |
| 89 | `api_market_mine` | `/api/market/mine` | 通用 | 否 | `youyuan_catalog.go:92` | `youyuan_market.go:617` | — |
| 90 | `api_marquee_preview` | `/api/marquee/preview` | 通用 | 否 | `youyuan_catalog.go:93` | `marquee.go:330` | — |
| 91 | `api_marquee_start` | `/api/marquee/start` | 通用 | 是 | `youyuan_catalog.go:94` | `marquee.go:295` | — |
| 92 | `api_marquee_status` | `/api/marquee/status` | 通用 | 否 | `youyuan_catalog.go:95` | `marquee.go:317` | — |
| 93 | `api_marquee_stop` | `/api/marquee/stop` | 通用 | 是 | `youyuan_catalog.go:96` | `marquee.go:306` | — |
| 94 | `api_mcfunction_execute` | `/api/mcfunction/execute` | 通用 | 是 | `youyuan_catalog.go:97` | `mcfunction.go:573` | — |
| 95 | `api_mcfunction_parse` | `/api/mcfunction/parse` | 通用 | 是 | `youyuan_catalog.go:98` | `mcfunction.go:269` | — |
| 96 | `api_mcfunction_status` | `/api/mcfunction/status` | 通用 | 否 | `youyuan_catalog.go:99` | `mcfunction.go:607` | — |
| 97 | `api_mcfunction_stop` | `/api/mcfunction/stop` | 通用 | 是 | `youyuan_catalog.go:100` | `mcfunction.go:597` | — |
| 98 | `api_mcfunction_syntax` | `/api/mcfunction/syntax` | 通用 | 否 | `youyuan_catalog.go:101` | `mcfunction.go:636` | — |
| 99 | `api_mcort_announcements` | `/api/mcort/announcements` | 通用 | 否 | `youyuan_catalog.go:102` | `youyuan_theme.go:915` | 本机只读 |
| 100 | `api_mcort_plugins` | `/api/mcort/plugins` | 通用 | 是 | `youyuan_catalog.go:103` | `youyuan_theme.go:821` | — |
| 101 | `api_mcort_theme` | `/api/mcort/theme` | 通用 | 是 | `youyuan_catalog.go:104` | `youyuan_theme.go:729` | — |
| 102 | `api_mcort_theme_asset` | `/api/mcort/theme/asset` | 通用 | 是 | `youyuan_catalog.go:105` | `youyuan_theme.go:773` | — |
| 103 | `api_mcort_themes` | `/api/mcort/themes` | 通用 | 否 | `youyuan_catalog.go:106` | `youyuan_theme.go:716` | — |
| 104 | `api_media_preview` | `/api/media/preview` | 通用 | 否 | `youyuan_catalog.go:107` | `main.go:1843` | — |
| 105 | `api_music_actionbar` | `/api/music/actionbar` | 通用 | 是 | `youyuan_catalog.go:108` | `main.go:3330` | — |
| 106 | `api_music_loop` | `/api/music/loop` | 通用 | 是 | `youyuan_catalog.go:109` | `main.go:3452` | — |
| 107 | `api_music_next` | `/api/music/next` | 通用 | 是 | `youyuan_catalog.go:110` | `main.go:3489` | — |
| 108 | `api_music_note` | `/api/music/note` | 通用 | 是 | `youyuan_catalog.go:111` | `main.go:3294` | — |
| 109 | `api_music_pause` | `/api/music/pause` | 通用 | 是 | `youyuan_catalog.go:112` | `main.go:3236` | — |
| 110 | `api_music_play` | `/api/music/play` | 通用 | 是 | `youyuan_catalog.go:113` | `main.go:3128` | — |
| 111 | `api_music_prev` | `/api/music/prev` | 通用 | 是 | `youyuan_catalog.go:114` | `main.go:3501` | — |
| 112 | `api_music_queue` | `/api/music/queue` | 通用 | 否 | `youyuan_catalog.go:115` | `main.go:3351` | — |
| 113 | `api_music_queue_add` | `/api/music/queue/add` | 通用 | 是 | `youyuan_catalog.go:116` | `main.go:3369` | — |
| 114 | `api_music_queue_clear` | `/api/music/queue/clear` | 通用 | 是 | `youyuan_catalog.go:117` | `main.go:3442` | — |
| 115 | `api_music_queue_remove` | `/api/music/queue/remove` | 通用 | 是 | `youyuan_catalog.go:118` | `main.go:3420` | — |
| 116 | `api_music_resume` | `/api/music/resume` | 通用 | 是 | `youyuan_catalog.go:119` | `main.go:3247` | — |
| 117 | `api_music_status` | `/api/music/status` | 通用 | 否 | `youyuan_catalog.go:120` | `main.go:3258` | — |
| 118 | `api_music_stop` | `/api/music/stop` | 通用 | 是 | `youyuan_catalog.go:121` | `main.go:3219` | — |
| 119 | `api_music_upload` | `/api/music/upload` | 通用 | 是 | `youyuan_catalog.go:122` | `main.go:3036` | — |
| 120 | `api_pathfinder_follow` | `/api/pathfinder/follow` | 通用 | 是 | `youyuan_catalog.go:123` | `main.go:1632` | — |
| 121 | `api_pathfinder_goto` | `/api/pathfinder/goto` | 通用 | 是 | `youyuan_catalog.go:124` | `main.go:1601` | — |
| 122 | `api_pathfinder_log` | `/api/pathfinder/log` | 通用 | 否 | `youyuan_catalog.go:125` | `main.go:1667` | — |
| 123 | `api_pathfinder_stop` | `/api/pathfinder/stop` | 通用 | 是 | `youyuan_catalog.go:126` | `main.go:1660` | — |
| 124 | `api_permission_storage` | `/api/permission/storage` | 通用 | 是 | `youyuan_catalog.go:127` | `main.go:4847` | — |
| 125 | `api_players_list` | `/api/players/list` | 通用 | 否 | `youyuan_catalog.go:128` | `camera_handler.go:13` | — |
| 126 | `api_plugin_market_proxy` | `/api/plugin-market-proxy/` | 通用 | 是 | `youyuan_catalog.go:129` | `main.go:2405` | — |
| 127 | `api_plugin_market_contribute` | `/api/plugin-market/contribute` | 通用 | 是 | `youyuan_catalog.go:130` | `main.go:2554` | — |
| 128 | `api_plugin_market_install` | `/api/plugin-market/install` | 通用 | 是 | `youyuan_catalog.go:131` | `main.go:2451` | — |
| 129 | `api_plugin_api_doc` | `/api/plugin/api-doc` | 通用 | 否 | `youyuan_catalog.go:132` | `main.go:962` | — |
| 130 | `api_plugin_config` | `/api/plugin/config` | 通用 | 是 | `youyuan_catalog.go:133` | `main.go:5067` | — |
| 131 | `api_plugin_create` | `/api/plugin/create` | 通用 | 是 | `youyuan_catalog.go:134` | `main.go:888` | — |
| 132 | `api_plugin_data` | `/api/plugin/data` | 通用 | 否 | `youyuan_catalog.go:135` | `main.go:879` | — |
| 133 | `api_plugin_delete` | `/api/plugin/delete` | 通用 | 是 | `youyuan_catalog.go:136` | `main.go:753` | — |
| 134 | `api_plugin_disabled` | `/api/plugin/disabled` | 通用 | 是 | `youyuan_catalog.go:137` | `main.go:970` | — |
| 135 | `api_plugin_export` | `/api/plugin/export` | 通用 | 是 | `youyuan_catalog.go:138` | `main.go:792` | — |
| 136 | `api_plugin_export_saf` | `/api/plugin/export-saf` | 通用 | 是 | `youyuan_catalog.go:139` | `main.go:820` | — |
| 137 | `api_plugin_file_read` | `/api/plugin/file/read` | 通用 | 否 | `youyuan_catalog.go:140` | `main.go:932` | — |
| 138 | `api_plugin_file_write` | `/api/plugin/file/write` | 通用 | 是 | `youyuan_catalog.go:141` | `main.go:944` | — |
| 139 | `api_plugin_import` | `/api/plugin/import` | 通用 | 是 | `youyuan_catalog.go:142` | `main.go:774` | — |
| 140 | `api_plugin_list` | `/api/plugin/list` | 通用 | 否 | `youyuan_catalog.go:143` | `main.go:665` | — |
| 141 | `api_plugin_meta` | `/api/plugin/meta` | 通用 | 是 | `youyuan_catalog.go:144` | `main.go:911` | — |
| 142 | `api_plugin_profiler` | `/api/plugin/profiler` | 通用 | 否 | `youyuan_catalog.go:145` | `youyuan_profiler.go:40` | — |
| 143 | `api_plugin_reload` | `/api/plugin/reload` | 通用 | 是 | `youyuan_catalog.go:146` | `main.go:727` | — |
| 144 | `api_plugin_run` | `/api/plugin/run` | 通用 | 是 | `youyuan_catalog.go:147` | `main.go:669` | — |
| 145 | `api_plugin_stop` | `/api/plugin/stop` | 通用 | 是 | `youyuan_catalog.go:148` | `main.go:703` | — |
| 146 | `api_plugin_wordbank_doc` | `/api/plugin/wordbank-doc` | 通用 | 是 | `youyuan_catalog.go:149` | `main.go:966` | — |
| 147 | `api_preview_cancel` | `/api/preview/cancel` | 通用 | 是 | `youyuan_catalog.go:150` | `preview_handler.go:311` | — |
| 148 | `api_preview_generate` | `/api/preview/generate` | 通用 | 是 | `youyuan_catalog.go:151` | `preview_handler.go:259` | — |
| 149 | `api_preview_generate_gif` | `/api/preview/generate-gif` | 通用 | 是 | `youyuan_catalog.go:152` | `preview_handler.go:603` | — |
| 150 | `api_preview_render_isometric` | `/api/preview/render-isometric` | 通用 | 是 | `youyuan_catalog.go:153` | `preview_handler.go:371` | — |
| 151 | `api_preview_save_image` | `/api/preview/save-image` | 通用 | 是 | `youyuan_catalog.go:154` | `preview_handler.go:679` | — |
| 152 | `api_preview_scan` | `/api/preview/scan` | 通用 | 是 | `youyuan_catalog.go:155` | `preview_handler.go:61` | — |
| 153 | `api_preview_status` | `/api/preview/status` | 通用 | 否 | `youyuan_catalog.go:156` | `preview_handler.go:288` | — |
| 154 | `api_preview_zip` | `/api/preview/zip` | 通用 | 是 | `youyuan_catalog.go:157` | `preview_handler.go:351` | — |
| 155 | `api_preview_zip_building` | `/api/preview/zip-building` | 通用 | 是 | `youyuan_catalog.go:158` | `preview_handler.go:328` | — |
| 156 | `api_push_ack` | `/api/push/ack` | 通用 | 是 | `youyuan_catalog.go:159` | `heartbeat.go:233` | — |
| 157 | `api_push_pending` | `/api/push/pending` | 通用 | 否 | `youyuan_catalog.go:160` | `heartbeat.go:225` | — |
| 158 | `api_qq_ai_tools` | `/api/qq/ai/tools` | 通用 | 是 | `youyuan_catalog.go:161` | `youyuan_qq_toolbox.go:193` | — |
| 159 | `api_qq_assistant` | `/api/qq/assistant` | 通用 | 是 | `youyuan_catalog.go:162` | `youyuan_qq_assistant.go:278` | — |
| 160 | `api_qq_assistant_skills` | `/api/qq/assistant/skills` | 通用 | 是 | `youyuan_catalog.go:163` | `youyuan_qq_skills.go:35` | — |
| 161 | `api_qq_claim_qr` | `/api/qq/claim/qr` | 通用 | 是 | `youyuan_catalog.go:164` | `youyuan_qq_claim_qr.go:573` | — |
| 162 | `api_qq_config` | `/api/qq/config` | 通用 | 是 | `youyuan_catalog.go:165` | `youyuan_qq.go:147` | — |
| 163 | `api_qq_diagnostics` | `/api/qq/diagnostics` | 通用 | 否 | `youyuan_catalog.go:166` | `youyuan_qq_official.go:200` | — |
| 164 | `api_qq_groups` | `/api/qq/groups` | 通用 | 是 | `youyuan_catalog.go:167` | `youyuan_qq_groups.go:89` | — |
| 165 | `api_qq_help` | `/api/qq/help` | 通用 | 否 | `youyuan_catalog.go:168` | `youyuan_qq_help.go:292` | 本机只读 |
| 166 | `api_qq_kb_doc` | `/api/qq/kb/doc` | 通用 | 否 | `youyuan_catalog.go:169` | `youyuan_qq_knowledge.go:509` | 本机只读 |
| 167 | `api_qq_kb_search` | `/api/qq/kb/search` | 通用 | 是 | `youyuan_catalog.go:170` | `youyuan_qq_knowledge.go:666` | — |
| 168 | `api_qq_kb_text` | `/api/qq/kb/text` | 通用 | 是 | `youyuan_catalog.go:171` | `youyuan_qq_knowledge.go:549` | — |
| 169 | `api_qq_kb_tree` | `/api/qq/kb/tree` | 通用 | 否 | `youyuan_catalog.go:172` | `youyuan_qq_knowledge.go:477` | 本机只读 |
| 170 | `api_qq_kb_upload` | `/api/qq/kb/upload` | 通用 | 是 | `youyuan_catalog.go:173` | `youyuan_qq_knowledge.go:576` | — |
| 171 | `api_qq_memory` | `/api/qq/memory` | 通用 | 是 | `youyuan_catalog.go:174` | `youyuan_qq_memory.go:535` | — |
| 172 | `api_qq_messages` | `/api/qq/messages` | 通用 | 否 | `youyuan_catalog.go:175` | `youyuan_qq_context.go:267` | — |
| 173 | `api_qq_messages_summary` | `/api/qq/messages/summary` | 通用 | 否 | `youyuan_catalog.go:176` | `youyuan_qq_messages_multi.go:56` | 本机只读 |
| 174 | `api_qq_persona` | `/api/qq/persona` | 通用 | 是 | `youyuan_catalog.go:177` | `youyuan_qq_persona.go:584` | — |
| 175 | `api_qq_player_card` | `/api/qq/player-card` | 通用 | 是 | `youyuan_catalog.go:178` | `youyuan_qq_assistant.go:327` | — |
| 176 | `api_qq_player_list` | `/api/qq/player-list` | 通用 | 是 | `youyuan_catalog.go:179` | `youyuan_qq_assistant.go:336` | — |
| 177 | `api_qq_relay` | `/api/qq/relay` | 通用 | 否 | `youyuan_catalog.go:180` | `youyuan_qq_relay.go:633` | 本机只读 |
| 178 | `api_qq_send` | `/api/qq/send` | 通用 | 是 | `youyuan_catalog.go:181` | `youyuan_qq.go:255` | — |
| 179 | `api_qq_status` | `/api/qq/status` | 通用 | 否 | `youyuan_catalog.go:182` | `youyuan_qq.go:235` | — |
| 180 | `api_qq_verify` | `/api/qq/verify` | 通用 | 是 | `youyuan_catalog.go:183` | `youyuan_qq_official.go:187` | — |
| 181 | `api_qq_watchdog` | `/api/qq/watchdog` | 通用 | 是 | `youyuan_catalog.go:184` | `youyuan_qq_watchdog.go:278` | — |
| 182 | `api_repair_start` | `/api/repair/start` | 通用 | 是 | `youyuan_catalog.go:185` | `main.go:384` | — |
| 183 | `api_seed_biome` | `/api/seed/biome` | 通用 | 否 | `youyuan_catalog.go:186` | `youyuan_seed_api.go:724` | 本机只读 |
| 184 | `api_seed_coordinates` | `/api/seed/coordinates` | 通用 | 否 | `youyuan_catalog.go:187` | `youyuan_seed_api.go:724` | 本机只读 |
| 185 | `api_seed_heat` | `/api/seed/heat` | 通用 | 否 | `youyuan_catalog.go:188` | `youyuan_seed_api.go:724` | 本机只读 |
| 186 | `api_seed_manifest` | `/api/seed/manifest` | 通用 | 否 | `youyuan_catalog.go:189` | `youyuan_seed_api.go:724` | 本机只读 |
| 187 | `api_seed_query` | `/api/seed/query` | 通用 | 否 | `youyuan_catalog.go:190` | `youyuan_seed_api.go:724` | 本机只读 |
| 188 | `api_seed_slime` | `/api/seed/slime` | 通用 | 否 | `youyuan_catalog.go:191` | `youyuan_seed_api.go:724` | 本机只读 |
| 189 | `api_seed_stronghold` | `/api/seed/stronghold` | 通用 | 否 | `youyuan_catalog.go:192` | `youyuan_seed_api.go:724` | 本机只读 |
| 190 | `api_seed_structure` | `/api/seed/structure` | 通用 | 否 | `youyuan_catalog.go:193` | `youyuan_seed_api.go:724` | 本机只读 |
| 191 | `api_seed_v2_biome` | `/api/seed/v2/biome` | 通用 | 否 | `youyuan_catalog.go:194` | `youyuan_seed_api.go:724` | 本机只读 |
| 192 | `api_seed_v2_icon` | `/api/seed/v2/icon.svg` | 通用 | 否 | `youyuan_catalog.go:195` | `youyuan_seed_api.go:724` | 本机只读 |
| 193 | `api_seed_v2_manifest` | `/api/seed/v2/manifest` | 通用 | 否 | `youyuan_catalog.go:196` | `youyuan_seed_api.go:724` | 本机只读 |
| 194 | `api_seed_v2_meta` | `/api/seed/v2/meta` | 通用 | 否 | `youyuan_catalog.go:197` | `youyuan_seed_api.go:724` | 本机只读 |
| 195 | `api_seed_v2_slime` | `/api/seed/v2/slime` | 通用 | 否 | `youyuan_catalog.go:198` | `youyuan_seed_api.go:724` | 本机只读 |
| 196 | `api_seed_v2_stronghold` | `/api/seed/v2/stronghold` | 通用 | 否 | `youyuan_catalog.go:199` | `youyuan_seed_api.go:724` | 本机只读 |
| 197 | `api_seed_v2_structures` | `/api/seed/v2/structures` | 通用 | 否 | `youyuan_catalog.go:200` | `youyuan_seed_api.go:724` | 本机只读 |
| 198 | `api_seed_v2_summary` | `/api/seed/v2/summary` | 通用 | 否 | `youyuan_catalog.go:201` | `youyuan_seed_api.go:724` | 本机只读 |
| 199 | `api_seed_v2_tile` | `/api/seed/v2/tile.png` | 通用 | 否 | `youyuan_catalog.go:202` | `youyuan_seed_api.go:724` | 本机只读 |
| 200 | `api_skin_account_head` | `/api/skin/account-head` | 通用 | 否 | `youyuan_catalog.go:203` | `youyuan_skin_account_media.go:105` | 本机只读 |
| 201 | `api_skin_account_texture` | `/api/skin/account-texture` | 通用 | 否 | `youyuan_catalog.go:204` | `youyuan_skin_account_media.go:88` | 本机只读 |
| 202 | `api_skin_build` | `/api/skin/build` | 通用 | 是 | `youyuan_catalog.go:205` | `main.go:3641` | — |
| 203 | `api_skin_head` | `/api/skin/head` | 通用 | 否 | `youyuan_catalog.go:206` | `main.go:3903` | — |
| 204 | `api_skin_item_library` | `/api/skin/item-library` | 通用 | 否 | `youyuan_catalog.go:207` | `youyuan_skin_item.go:587` | 本机只读 |
| 205 | `api_skin_item_texture` | `/api/skin/item-texture` | 通用 | 否 | `youyuan_catalog.go:208` | `youyuan_skin_item.go:636` | 本机只读 |
| 206 | `api_skin_online_build` | `/api/skin/online-build` | 通用 | 是 | `youyuan_catalog.go:209` | `main.go:3790` | — |
| 207 | `api_skin_online_head` | `/api/skin/online-head` | 通用 | 否 | `youyuan_catalog.go:210` | `main.go:3932` | — |
| 208 | `api_skin_online_list` | `/api/skin/online-list` | 通用 | 否 | `youyuan_catalog.go:211` | `main.go:3715` | — |
| 209 | `api_skin_online_preview` | `/api/skin/online-preview` | 通用 | 否 | `youyuan_catalog.go:212` | `main.go:3743` | — |
| 210 | `api_skin_preview` | `/api/skin/preview` | 通用 | 否 | `youyuan_catalog.go:213` | `main.go:3516` | — |
| 211 | `api_skin_voxel` | `/api/skin/voxel` | 通用 | 否 | `youyuan_catalog.go:214` | `main.go:3568` | — |
| 212 | `api_system_restart` | `/api/system/restart` | 通用 | 是 | `youyuan_catalog.go:215` | `main.go:268` | — |
| 213 | `api_task_delete` | `/api/task/delete` | 通用 | 是 | `youyuan_catalog.go:216` | `main.go:581` | — |
| 214 | `api_task_list` | `/api/task/list` | 通用 | 否 | `youyuan_catalog.go:217` | `main.go:503` | — |
| 215 | `api_task_resume` | `/api/task/resume` | 通用 | 是 | `youyuan_catalog.go:218` | `main.go:519` | — |
| 216 | `api_task_start` | `/api/task/start` | 通用 | 是 | `youyuan_catalog.go:219` | `main.go:322` | — |
| 217 | `api_task_stop` | `/api/task/stop` | 通用 | 是 | `youyuan_catalog.go:220` | `main.go:494` | — |
| 218 | `api_version_check` | `/api/version/check` | 通用 | 否 | `youyuan_catalog.go:221` | `youyuan_product.go:56` | — |
| 219 | `api_world_enable` | `/api/world/enable` | 通用 | 是 | `youyuan_catalog.go:222` | `main.go:1475` | — |
| 220 | `api_youyuan_account_pool` | `/api/youyuan/account-pool` | 通用 | 是 | `youyuan_catalog.go:223` | `youyuan_account_pool.go:656` | — |
| 221 | `api_youyuan_account_action` | `/api/youyuan/account/action` | 通用 | 是 | `youyuan_catalog.go:224` | `youyuan_account.go:26` | — |
| 222 | `api_youyuan_accounts` | `/api/youyuan/accounts` | 通用 | 是 | `youyuan_catalog.go:225` | `youyuan_accounts.go:375` | — |
| 223 | `api_youyuan_accounts_bridge_execute` | `/api/youyuan/accounts/bridge/execute` | 通用 | 是 | `youyuan_catalog.go:226` | `youyuan_account_bridge.go:342` | — |
| 224 | `api_youyuan_accounts_bridge_profiles` | `/api/youyuan/accounts/bridge/profiles` | 通用 | 是 | `youyuan_catalog.go:227` | `youyuan_account_bridge.go:335` | — |
| 225 | `api_youyuan_accounts_bridge_rules` | `/api/youyuan/accounts/bridge/rules` | 通用 | 是 | `youyuan_catalog.go:228` | `youyuan_account_bridge.go:297` | — |
| 226 | `api_youyuan_agent_archive` | `/api/youyuan/agent/archive` | 通用 | 是 | `youyuan_catalog.go:229` | `youyuan_agent_cleanup.go:78` | — |
| 227 | `api_youyuan_agent_cloud_model` | `/api/youyuan/agent/cloud-model` | 通用 | 是 | `youyuan_catalog.go:230` | `youyuan_agent_preferences.go:106` | — |
| 228 | `api_youyuan_agent_conversation` | `/api/youyuan/agent/conversation` | 通用 | 是 | `youyuan_catalog.go:231` | `youyuan_agent_sessions.go:118` | — |
| 229 | `api_youyuan_agent_file` | `/api/youyuan/agent/file` | 通用 | 是 | `youyuan_catalog.go:232` | `youyuan_agent.go:1466` | — |
| 230 | `api_youyuan_agent_fork` | `/api/youyuan/agent/fork` | 通用 | 是 | `youyuan_catalog.go:233` | `youyuan_agent_fork.go:17` | — |
| 231 | `api_youyuan_agent_mcp` | `/api/youyuan/agent/mcp` | 通用 | 是 | `youyuan_catalog.go:234` | `youyuan_agent.go:1609` | — |
| 232 | `api_youyuan_agent_model` | `/api/youyuan/agent/model` | 通用 | 是 | `youyuan_catalog.go:235` | `youyuan_agent_preferences.go:195` | — |
| 233 | `api_youyuan_agent_preferences` | `/api/youyuan/agent/preferences` | 通用 | 是 | `youyuan_catalog.go:236` | `youyuan_agent_preferences.go:129` | — |
| 234 | `api_youyuan_agent_run_budget` | `/api/youyuan/agent/run-budget` | 通用 | 是 | `youyuan_catalog.go:237` | `youyuan_agent_budget.go:38` | — |
| 235 | `api_youyuan_agent_save` | `/api/youyuan/agent/save` | 通用 | 是 | `youyuan_catalog.go:238` | `youyuan_agent_sessions.go:93` | — |
| 236 | `api_youyuan_agent_session` | `/api/youyuan/agent/session` | 通用 | 是 | `youyuan_catalog.go:239` | `youyuan_agent_sessions.go:15` | — |
| 237 | `api_youyuan_agent_skills` | `/api/youyuan/agent/skills` | 通用 | 是 | `youyuan_catalog.go:240` | `youyuan_skill_library.go:124` | — |
| 238 | `api_youyuan_agent_sync_plugin` | `/api/youyuan/agent/sync-plugin` | 通用 | 是 | `youyuan_catalog.go:241` | `youyuan_agent_cleanup.go:96` | — |
| 239 | `api_youyuan_agent_task` | `/api/youyuan/agent/task` | 通用 | 是 | `youyuan_catalog.go:242` | `youyuan_agent.go:1385` | — |
| 240 | `api_youyuan_agent_tool` | `/api/youyuan/agent/tool` | 通用 | 是 | `youyuan_catalog.go:243` | `youyuan_agent.go:1432` | — |
| 241 | `api_youyuan_agent_upload` | `/api/youyuan/agent/upload` | 通用 | 是 | `youyuan_catalog.go:244` | `youyuan_agent.go:1509` | — |
| 242 | `api_youyuan_agent_validate` | `/api/youyuan/agent/validate` | 通用 | 是 | `youyuan_catalog.go:245` | `youyuan_agent.go:1658` | — |
| 243 | `api_youyuan_agent_workspaces` | `/api/youyuan/agent/workspaces` | 通用 | 否 | `youyuan_catalog.go:246` | `youyuan_agent.go:1281` | — |
| 244 | `api_youyuan_appearance` | `/api/youyuan/appearance` | 通用 | 是 | `youyuan_catalog.go:247` | `youyuan_appearance.go:55` | — |
| 245 | `api_youyuan_appearance_file` | `/api/youyuan/appearance/file` | 通用 | 否 | `youyuan_catalog.go:248` | `youyuan_appearance.go:130` | — |
| 246 | `api_youyuan_appearance_upload` | `/api/youyuan/appearance/upload` | 通用 | 是 | `youyuan_catalog.go:249` | `youyuan_appearance.go:95` | — |
| 247 | `api_youyuan_catalog` | `/api/youyuan/catalog` | 通用 | 否 | `youyuan_catalog.go:250` | `youyuan_remote.go:140` | — |
| 248 | `api_youyuan_chains` | `/api/youyuan/chains` | 通用 | 是 | `youyuan_catalog.go:251` | `youyuan_commands.go:747` | — |
| 249 | `api_youyuan_connection_policy` | `/api/youyuan/connection-policy` | 通用 | 是 | `youyuan_catalog.go:252` | `youyuan_connection.go:155` | — |
| 250 | `api_youyuan_connection_diagnostics` | `/api/youyuan/connection-diagnostics` | 通用 | 是 | `youyuan_catalog.go:253` | `youyuan_connection_report.go:579` | — |
| 251 | `api_youyuan_convert_commands` | `/api/youyuan/convert/commands` | 通用 | 是 | `youyuan_catalog.go:254` | `youyuan_converters.go:144` | — |
| 252 | `api_youyuan_convert_structure` | `/api/youyuan/convert/structure` | 通用 | 是 | `youyuan_catalog.go:255` | `youyuan_converters.go:227` | — |
| 253 | `api_youyuan_docs` | `/api/youyuan/docs` | 专用 query{...} | 否 | `youyuan_catalog.go:256` | `youyuan_docs.go:9` | 本机只读 |
| 254 | `api_youyuan_events` | `/api/youyuan/events` | 通用 | 否 | `youyuan_catalog.go:257` | `youyuan_remote.go:146` | — |
| 255 | `api_youyuan_exports` | `/api/youyuan/exports` | 通用 | 是 | `youyuan_catalog.go:258` | `youyuan_export_center.go:219` | — |
| 256 | `api_youyuan_exports_download` | `/api/youyuan/exports/download` | 通用 | 是 | `youyuan_catalog.go:259` | `youyuan_export_center.go:251` | — |
| 257 | `api_youyuan_exports_info` | `/api/youyuan/exports/info` | 通用 | 否 | `youyuan_catalog.go:260` | `youyuan_export_center.go:282` | — |
| 258 | `api_youyuan_exports_remove` | `/api/youyuan/exports/remove` | 通用 | 是 | `youyuan_catalog.go:261` | `youyuan_export_center.go:299` | — |
| 259 | `api_youyuan_feedback` | `/api/youyuan/feedback` | 通用 | 是 | `youyuan_catalog.go:262` | `youyuan_notices.go:99` | — |
| 260 | `api_youyuan_files` | `/api/youyuan/files` | 通用 | 是 | `youyuan_catalog.go:263` | `youyuan_host_files.go:21` | — |
| 261 | `api_youyuan_files_download` | `/api/youyuan/files/download` | 通用 | 是 | `youyuan_catalog.go:264` | `youyuan_host_files.go:94` | — |
| 262 | `api_youyuan_files_transfer` | `/api/youyuan/files/transfer` | 通用 | 是 | `youyuan_catalog.go:265` | `youyuan_file_upload_sessions.go:372` | — |
| 263 | `api_youyuan_files_upload` | `/api/youyuan/files/upload` | 通用 | 是 | `youyuan_catalog.go:266` | `youyuan_host_files.go:37` | — |
| 264 | `api_youyuan_game_auth` | `/api/youyuan/game-auth` | 通用 | 是 | `youyuan_catalog.go:267` | `youyuan_game_auth.go:26` | — |
| 265 | `api_youyuan_game_egress` | `/api/youyuan/game-egress` | 通用 | 是 | `youyuan_catalog.go:268` | `youyuan_egress.go:262` | — |
| 266 | `api_youyuan_game_call` | `/api/youyuan/game/call` | 通用 | 是 | `youyuan_catalog.go:269` | `youyuan_capture.go:498` | — |
| 267 | `api_youyuan_global_capture` | `/api/youyuan/global-capture` | 通用 | 是 | `youyuan_catalog.go:270` | `youyuan_global_capture.go:365` | — |
| 268 | `api_youyuan_identity` | `/api/youyuan/identity` | 通用 | 是 | `youyuan_catalog.go:271` | `youyuan_identity.go:112` | — |
| 269 | `api_youyuan_library_delete` | `/api/youyuan/library/delete` | 通用 | 是 | `youyuan_catalog.go:272` | `youyuan_library.go:14` | — |
| 270 | `api_youyuan_library_download` | `/api/youyuan/library/download` | 通用 | 是 | `youyuan_catalog.go:273` | `youyuan_library.go:14` | — |
| 271 | `api_youyuan_library_list` | `/api/youyuan/library/list` | 通用 | 否 | `youyuan_catalog.go:274` | `youyuan_library.go:14` | — |
| 272 | `api_youyuan_library_upload` | `/api/youyuan/library/upload` | 通用 | 是 | `youyuan_catalog.go:275` | `youyuan_library.go:14` | — |
| 273 | `api_youyuan_mail_send` | `/api/youyuan/mail/send` | 通用 | 是 | `youyuan_catalog.go:276` | `youyuan_mail.go:43` | — |
| 274 | `api_youyuan_mcp_links` | `/api/youyuan/mcp-links` | 通用 | 是 | `youyuan_catalog.go:277` | `youyuan_mcp_links.go:113` | — |
| 275 | `api_youyuan_mcp_scope` | `/api/youyuan/mcp/scope` | 通用 | 是 | `youyuan_catalog.go:278` | `youyuan_mcp_scope.go:645` | — |
| 276 | `api_youyuan_mcp_traces` | `/api/youyuan/mcp/traces` | 通用 | 否 | `youyuan_catalog.go:279` | `youyuan_mcp_telemetry.go:80` | — |
| 277 | `api_youyuan_media_block_video` | `/api/youyuan/media/block-video` | 通用 | 是 | `youyuan_catalog.go:280` | `youyuan_block_video.go:164` | — |
| 278 | `api_youyuan_notices` | `/api/youyuan/notices` | 通用 | 是 | `youyuan_catalog.go:281` | `youyuan_notices.go:81` | — |
| 279 | `api_youyuan_observations` | `/api/youyuan/observations` | 通用 | 否 | `youyuan_catalog.go:282` | `youyuan_observations.go:68` | 本机只读 |
| 280 | `api_youyuan_observations_export` | `/api/youyuan/observations/export` | 通用 | 是 | `youyuan_catalog.go:283` | `youyuan_observations.go:74` | — |
| 281 | `api_youyuan_observations_summary` | `/api/youyuan/observations/summary` | 通用 | 否 | `youyuan_catalog.go:284` | `youyuan_observation_summary.go:129` | — |
| 282 | `api_youyuan_observer_ingest` | `/api/youyuan/observer/ingest` | 通用 | 是 | `youyuan_catalog.go:285` | `youyuan_observer_bridge.go:246` | — |
| 283 | `api_youyuan_packets_capture` | `/api/youyuan/packets/capture` | 通用 | 否 | `youyuan_catalog.go:286` | `youyuan_capture.go:577` | — |
| 284 | `api_youyuan_packets_export` | `/api/youyuan/packets/export` | 通用 | 是 | `youyuan_catalog.go:287` | `youyuan_capture.go:622` | — |
| 285 | `api_youyuan_packets_schema` | `/api/youyuan/packets/schema` | 通用 | 否 | `youyuan_catalog.go:288` | `youyuan_capture.go:564` | — |
| 286 | `api_youyuan_player_experience` | `/api/youyuan/player/experience` | 通用 | 是 | `youyuan_catalog.go:289` | `youyuan_player_experience.go:270` | — |
| 287 | `api_youyuan_plugin_outputs` | `/api/youyuan/plugin-outputs` | 通用 | 是 | `youyuan_catalog.go:290` | `main.go:268` | — |
| 288 | `api_youyuan_plugin_outputs_download` | `/api/youyuan/plugin-outputs/download` | 通用 | 是 | `youyuan_catalog.go:291` | `main.go:268` | — |
| 289 | `api_youyuan_plugin_outputs_info` | `/api/youyuan/plugin-outputs/info` | 通用 | 否 | `youyuan_catalog.go:292` | `main.go:268` | — |
| 290 | `api_youyuan_remote` | `/api/youyuan/remote` | 通用 | 是 | `youyuan_catalog.go:293` | `youyuan_remote.go:95` | — |
| 291 | `api_youyuan_rooms` | `/api/youyuan/rooms` | 通用 | 是 | `youyuan_catalog.go:294` | `youyuan_room_discovery.go:164` | — |
| 292 | `api_youyuan_runtime_benchmark` | `/api/youyuan/runtime/benchmark` | 通用 | 是 | `youyuan_catalog.go:295` | `youyuan_benchmark.go:142` | — |
| 293 | `api_youyuan_runtime_delete` | `/api/youyuan/runtime/delete` | 通用 | 是 | `youyuan_catalog.go:296` | `youyuan_runtime_management.go:66` | — |
| 294 | `api_youyuan_runtime_event` | `/api/youyuan/runtime/event` | 通用 | 是 | `youyuan_catalog.go:297` | `youyuan_runtime.go:168` | — |
| 295 | `api_youyuan_runtime_export` | `/api/youyuan/runtime/export` | 通用 | 是 | `youyuan_catalog.go:298` | `youyuan_runtime_export.go:95` | — |
| 296 | `api_youyuan_runtime_files` | `/api/youyuan/runtime/files` | 通用 | 是 | `youyuan_catalog.go:299` | `youyuan_runtime_management.go:35` | — |
| 297 | `api_youyuan_runtime_import` | `/api/youyuan/runtime/import` | 通用 | 是 | `youyuan_catalog.go:300` | `youyuan_import.go:103` | — |
| 298 | `api_youyuan_runtime_list` | `/api/youyuan/runtime/list` | 通用 | 否 | `youyuan_catalog.go:301` | `youyuan_runtime.go:278` | 本机只读 |
| 299 | `api_youyuan_runtime_start` | `/api/youyuan/runtime/start` | 通用 | 是 | `youyuan_catalog.go:302` | `youyuan_runtime.go:60` | — |
| 300 | `api_youyuan_runtime_stop` | `/api/youyuan/runtime/stop` | 通用 | 是 | `youyuan_catalog.go:303` | `youyuan_runtime.go:196` | — |
| 301 | `api_youyuan_sdk_batch` | `/api/youyuan/sdk/batch` | 通用 | 是 | `youyuan_catalog.go:304` | `youyuan_sdk.go:220` | — |
| 302 | `api_youyuan_sdk_call` | `/api/youyuan/sdk/call` | 通用 | 是 | `youyuan_catalog.go:305` | `youyuan_sdk.go:290` | — |
| 303 | `api_youyuan_sdk_catalog` | `/api/youyuan/sdk/catalog` | 通用 | 否 | `youyuan_catalog.go:306` | `youyuan_sdk.go:269` | 本机只读 |
| 304 | `api_youyuan_sdk_pages` | `/api/youyuan/sdk/pages` | 通用 | 否 | `youyuan_catalog.go:307` | `youyuan_sdk.go:256` | — |
| 305 | `api_youyuan_sense_events` | `/api/youyuan/sense/events` | 通用 | 否 | `youyuan_catalog.go:308` | `youyuan_capture.go:546` | — |
| 306 | `api_youyuan_sense_snapshot` | `/api/youyuan/sense/snapshot` | 通用 | 是 | `youyuan_catalog.go:309` | `youyuan_capture.go:533` | — |
| 307 | `api_youyuan_skills_export` | `/api/youyuan/skills/export` | 通用 | 是 | `youyuan_catalog.go:310` | `youyuan_skill_library.go:95` | — |
| 308 | `api_youyuan_skills_read` | `/api/youyuan/skills/read` | 通用 | 否 | `youyuan_catalog.go:311` | `youyuan_skill_library.go:205` | — |
| 309 | `api_youyuan_social` | `/api/youyuan/social` | 通用 | 否 | `youyuan_catalog.go:312` | `youyuan_social.go:255` | — |
| 310 | `api_youyuan_social_search` | `/api/youyuan/social/search` | 通用 | 否 | `youyuan_catalog.go:313` | `youyuan_social.go:241` | — |
| 311 | `api_youyuan_snapshot` | `/api/youyuan/snapshot` | 通用 | 否 | `youyuan_catalog.go:314` | `youyuan_snapshot.go:19` | 本机只读 |
| 312 | `api_youyuan_tunnel` | `/api/youyuan/tunnel` | 通用 | 是 | `youyuan_catalog.go:315` | `youyuan_tunnel.go:270` | — |
| 313 | `api_youyuan_world_seed` | `/api/youyuan/world/seed` | 通用 | 否 | `youyuan_catalog.go:316` | `youyuan_seed_world.go:117` | 本机只读 |
| 314 | `api_catalog` | `—`（内存返回 yyAPIRoutes） | 无参数 | 否 | `youyuan_remote.go:519` | `youyuan_remote.go:519` | — |
| 315 | `api_call` | `—`（任意 /api/ 路径） | path* + 通用 | 是 | `youyuan_remote.go:520` | `youyuan_remote.go:520` | — |

**合计：源码注册 315 个 tool，本表 315 条。**

313 条来自 `yyAPIRoutes`（`upstream/youyuan_catalog.go:3-317`，一行一条），另 2 条为 `api_catalog`、`api_call`（`upstream/youyuan_remote.go:519-520`）；与 `GET /health` 的 `tools` 字段同源（`:84`）。

## 3. resources 全量清单（39 条 + 1 个模板）

注册入口：`yyMCPResources()`（`upstream/youyuan_mcp_resources.go:11-40`）；读取入口：`yyMCPReadResource()`（同文件 `:41-140`）。

| # | uri | 类型 | 内容 | 源码位置 |
|---|---|---|---|---|
| 1 | `youyuan://api/catalog` | application/json | API 目录（完整 yyAPIRoutes） | `youyuan_mcp_resources.go:13` |
| 2 | `youyuan://runtime/status` | application/json | 转发 GET /api/youyuan/runtime/list | `youyuan_mcp_resources.go:13` |
| 3 | `youyuan://accounts/status` | application/json | 转发 GET /api/youyuan/accounts（令牌脱敏） | `youyuan_mcp_resources.go:13` |
| 4 | `youyuan://docs/python-runtime` | text/markdown | Python 开发说明（index.md） | `youyuan_mcp_resources.go:13` |
| 5 | `youyuan://docs/cpp-runtime` | text/markdown | C++ 开发说明（index.md） | `youyuan_mcp_resources.go:13` |
| 6 | `youyuan://docs/lua-runtime` | text/markdown | Lua 宿主说明（index.md） | `youyuan_mcp_resources.go:13` |
| 7 | `youyuan://docs/command-chain` | text/markdown | 命令链说明（index.md） | `youyuan_mcp_resources.go:13` |
| 8 | `youyuan://docs/bedrock-commands` | text/markdown | Bedrock 指令技能（全部 Sources） | `youyuan_mcp_resources.go:13` |
| 9 | `youyuan://docs/command-chain-control` | text/markdown | 跨链调用与条件等待（control-flow.md） | `youyuan_mcp_resources.go:17-26` |
| 10 | `youyuan://docs/qq-assistant` | text/markdown | 群服互通助手 | `youyuan_mcp_resources.go:17-26` |
| 11 | `youyuan://docs/self-auth` | text/markdown | 游园独立游戏认证 | `youyuan_mcp_resources.go:17-26` |
| 12 | `youyuan://docs/mcp-connection` | text/markdown | MCP接入教程 | `youyuan_mcp_resources.go:17-26` |
| 13 | `youyuan://docs/public-tunnel` | text/markdown | APK 临时公网 MCP | `youyuan_mcp_resources.go:17-26` |
| 14 | `youyuan://docs/command-chain-api` | text/markdown | 插件命令链管理 | `youyuan_mcp_resources.go:17-26` |
| 15 | `youyuan://docs/physical-actions` | text/markdown | 机器人物理动作 | `youyuan_mcp_resources.go:17-26` |
| 16 | `youyuan://docs/block-media` | text/markdown | 方块视频与转换 | `youyuan_mcp_resources.go:17-26` |
| 17 | `youyuan://docs/command-execution` | text/markdown | 插件命令执行 | `youyuan_mcp_resources.go:17-26` |
| 18 | `youyuan://docs/toolbox-live-development` | text/markdown | 运行能力与验收范围 | `youyuan_mcp_resources.go:17-26` |
| 19 | `youyuan://docs/efficient-context` | text/markdown | 内置技能（ID: efficient-context） | `youyuan_mcp_resources.go:31-36` |
| 20 | `youyuan://docs/player-actions` | text/markdown | 内置技能（ID: player-actions） | `youyuan_mcp_resources.go:31-36` |
| 21 | `youyuan://docs/account-pool` | text/markdown | 内置技能（ID: account-pool） | `youyuan_mcp_resources.go:31-36` |
| 22 | `youyuan://docs/observer-bridge` | text/markdown | 内置技能（ID: observer-bridge） | `youyuan_mcp_resources.go:31-36` |
| 23 | `youyuan://docs/network-observation` | text/markdown | 内置技能（ID: network-observation） | `youyuan_mcp_resources.go:31-36` |
| 24 | `youyuan://docs/nbt-workbench` | text/markdown | 内置技能（ID: nbt-workbench） | `youyuan_mcp_resources.go:31-36` |
| 25 | `youyuan://docs/account-bridge` | text/markdown | 内置技能（ID: account-bridge） | `youyuan_mcp_resources.go:31-36` |
| 26 | `youyuan://docs/runtime-kernel` | text/markdown | 内置技能（ID: runtime-kernel） | `youyuan_mcp_resources.go:31-36` |
| 27 | `youyuan://docs/gameplay-events` | text/markdown | 内置技能（ID: gameplay-events） | `youyuan_mcp_resources.go:31-36` |
| 28 | `youyuan://docs/building-transfer` | text/markdown | 内置技能（ID: building-transfer） | `youyuan_mcp_resources.go:31-36` |
| 29 | `youyuan://docs/packet-reference` | text/markdown | 内置技能（ID: packet-reference） | `youyuan_mcp_resources.go:31-36` |
| 30 | `youyuan://docs/mcbe-commands` | text/markdown | 内置技能（ID: mcbe-commands） | `youyuan_mcp_resources.go:31-36` |
| 31 | `youyuan://docs/mcbe-execute` | text/markdown | 内置技能（ID: mcbe-execute） | `youyuan_mcp_resources.go:31-36` |
| 32 | `youyuan://docs/mcbe-rawtext` | text/markdown | 内置技能（ID: mcbe-rawtext） | `youyuan_mcp_resources.go:31-36` |
| 33 | `youyuan://docs/mcbe-systems` | text/markdown | 内置技能（ID: mcbe-systems） | `youyuan_mcp_resources.go:31-36` |
| 34 | `youyuan://docs/mcbe-netease` | text/markdown | 内置技能（ID: mcbe-netease） | `youyuan_mcp_resources.go:31-36` |
| 35 | `youyuan://docs/mcbe-particles` | text/markdown | 内置技能（ID: mcbe-particles） | `youyuan_mcp_resources.go:31-36` |
| 36 | `youyuan://docs/world-seed` | text/markdown | 内置技能（ID: world-seed） | `youyuan_mcp_resources.go:31-36` |
| 37 | `youyuan://docs/fmbe-display` | text/markdown | 内置技能（ID: fmbe-display） | `youyuan_mcp_resources.go:31-36` |
| 38 | `youyuan://skills/catalog` | application/json | 技能加载库与章节目录（yyAllSkillCatalog） | `youyuan_mcp_resources.go:37` |
| 39 | `youyuan://packets/catalog` | application/json | 已注册协议包简短目录（yyPacketCatalog） | `youyuan_mcp_resources.go:38` |

**合计：39 条**（18 条硬编码 + 19 条由 `aiSkills` 动态补齐 + 2 条目录型资源）。

补充说明：

- 硬编码 18 条中的 5 条（python-runtime / cpp-runtime / lua-runtime / command-chain / bedrock-commands）与 `aiSkills` 同名，靠 `seen` 去重，不会重复出现（`:27-36`）。
- URI 模板（`resources/templates/list`，`upstream/youyuan_remote.go:338-339`）：`youyuan://account/{account}/project/{workspace}/{path}`，用于读账号项目真实文件；`account=main` 或独立账号 ID，越界/非法 ID 直接报错（`youyuan_mcp_resources.go:109-134`）。
- 文档资源支持分页参数：`youyuan://docs/<id>?section=&offset=&max_chars=`，`max_chars` 默认 3000（`:46-59`）。
- 只读连接可读的资源仅限 `youyuan://docs/*`、`youyuan://skills/catalog`、`youyuan://runtime/status`、`youyuan://accounts/status`（`upstream/youyuan_mcp_links.go:83`）。

## 4. 鉴权

### 4.1 五种凭证对照

| 凭证 | 生成 | 存放 | 用途 | 能否当 MCP 密钥 |
|---|---|---|---|---|
| 本机 MCP 密钥 `key` | `yyNewKey()`：32 字节 `crypto/rand` → hex（`upstream/youyuan_remote.go:43-49`） | `youyuan-remote.json`，0600，缺失时首次启动自动生成（`:50-61`） | 本机 `http://127.0.0.1:19429/mcp/` 与临时隧道 | **能**（仅本机/隧道） |
| 平台账号云端控制密钥 `control_token` | 注册时 `randomToken()`（`cloudflare/identity.mjs:129`） | 云端 Durable Object，键 `control:<hash>` | 稳定地址 `https://toolbox.../mcp/?device=...` 的 Bearer | **能**（云端） |
| 设备密钥 `device_token` | 同上（`identity.mjs:129`） | 云端 `device:<hash>` + 本机配置 | `/device/connect` 建 WebSocket 中继（`worker.mjs:92`、`youyuan_remote.go:524-535`） | 不能（不是 MCP 凭证） |
| 游戏令牌 | 认证服务签发，形如 `yy//…`（`auth-service/game-bearer.mjs:29`） | 账号库 | 连游戏服 | **不能**，源码注释明确「Token possession is a game capability, never a platform-management login」 |
| 模型 API Key | 用户自填 | 用户模型库 | 调 DeepSeek 等 | 不能，MCP 不做推理（`MCP接入教程.md:62`） |

### 4.2 三条校验链路（源码依据）

**① 本机 / 隧道 Bearer**：`yyMCP` 取 `Authorization` 去掉 `Bearer `，与 `yyRemote.Key` 做 `subtle.ConstantTimeCompare`，不等即 401（`upstream/youyuan_remote.go:238-252`）。

**② 本机专用连接 URL**：路径命中 `/mcp/link/<token>` 时用 `yyMCPLinkLookup` 查 `mcp-links.json`；命中后把 `supplied` 直接置为 `key`，并按 `grant.Scope` 设 `readOnly`（`:240-248`）。token 由 `yyNewKey()` 生成，有效期 1–8760 小时（默认 720），单执行端最多 128 条有效（`upstream/youyuan_mcp_links.go:144-160`），撤销即删记录（`:171-178`）。

**③ 云端**：Bearer 先试 `identityResolve`（`session:` → 回落 `control:`，`cloudflare/identity.mjs:55`）或 `oauthResolve`；read scope 在 worker 层二次过滤 `tools/list` 与 `tools/call`（`cloudflare/worker.mjs:26-31`、`:143`）。OAuth 用 S256 PKCE，access 1 小时、refresh 30 天且轮换（`cloudflare/oauth.mjs:53`），scope 仅 `mcp:read` / `mcp:control`（`:34-35`）。

错误语义：401 = 密钥不对/重复 Bearer/混用密钥；403 = 订阅或节点未授权（`worker.mjs:89`）；405 = 用 GET 打开 `/mcp`；503 = 执行端未连接（`worker.mjs:176`）。

### 4.3 APK 与 Linux 是否各自独立

| 层面 | 是否独立 | 依据 |
|---|---|---|
| 云端控制密钥 | **共享**：一个平台账号只有一组 `device_token` / `control_token`，注册时一次性生成，不是每设备一套 | `cloudflare/identity.mjs:129` |
| 目标设备 | **各自独立**：按 `X-YouYuan-Device` 头（优先）或 `?device=` 分桶，桶名不同即不同执行端 | `cloudflare/worker.mjs:104-109`、`upstream/youyuan_remote.go:529-534` |
| 本机 MCP 密钥 | **各自独立**：每个执行端实例的 `youyuan-remote.json` 各自生成 `key`，APK 与 Linux 互不通用 | `upstream/youyuan_remote.go:50-61` |
| 专用连接（mcp-links） | **各自独立**：`mcp-links.json` 落在各实例 dataDir，撤销只影响本机 | `upstream/youyuan_mcp_links.go:110-112` |

结论：**密钥账号级共享，连接与设备桶各自独立**。同一把云端控制密钥可以指向不同设备，但一次请求只能命中一个 `device`；文档「各平台用户只能访问自己的设备」与此一致（`MCP接入教程.md:48`）。

## 5. 白名单机制

### 5.1 三张只读表（能力不同，别混用）

**表 A：本机只读白名单 `yyMCPReadPaths`（35 条）** — `upstream/youyuan_mcp_links.go:26-64`

`/api/bot/status`、`/api/youyuan/observations`、`/api/youyuan/runtime/list`、`/api/youyuan/docs`、`/api/youyuan/sdk/catalog`、`/api/youyuan/world/seed`、`/api/market/plugins`、`/api/youyuan/snapshot`、`/api/seed/manifest`、`/api/seed/slime`、`/api/seed/stronghold`、`/api/seed/structure`、`/api/seed/query`、`/api/seed/heat`、`/api/seed/coordinates`、`/api/seed/biome`、`/api/seed/v2/manifest`、`/api/seed/v2/icon.svg`、`/api/seed/v2/meta`、`/api/seed/v2/tile.png`、`/api/seed/v2/structures`、`/api/seed/v2/biome`、`/api/seed/v2/slime`、`/api/seed/v2/stronghold`、`/api/seed/v2/summary`、`/api/qq/help`、`/api/qq/messages/summary`、`/api/qq/relay`、`/api/qq/kb/tree`、`/api/qq/kb/doc`、`/api/mcort/announcements`、`/api/skin/account-head`、`/api/skin/account-texture`、`/api/skin/item-texture`、`/api/skin/item-library`

**表 B：云端只读 5 条 `readTools`** — `cloudflare/mcp-link-policy.mjs:1`

`api_bot_status`、`api_youyuan_observations`、`api_youyuan_runtime_list`、`api_youyuan_docs`、`api_youyuan_sdk_catalog`

**表 C：OAuth `mcp:read`** — `cloudflare/mcp-discovery.mjs:5-7` 复用同一份 `readTools`，非只读工具打 `mcp:control` 标。

> 关键差异：同一句「只读连接」，在本机（表 A）能调 35 个工具，走云端（表 B）只剩 5 个。云端在 worker 层先过滤 `tools/list`（`worker.mjs:143`），执行端 `yyMCPReadCheck` 再兜一层（`youyuan_remote.go:318-323`）。

### 5.2 默认开放 / 显式开启

| 连接类型 | 默认可见 tool | 需要显式开启的 |
|---|---|---|
| 主密钥 / `control` 专用连接 / OAuth `mcp:control` | 全部 315 个，无需逐项开关 | 无（只有账号/订阅/节点层面的准入） |
| 本机 `read` 专用连接（`/mcp/link/`） | 表 A 的 35 个 | 其余 280 个：`tools/call` 直接返回 `-32003`「此连接仅授权状态和开发文档查询」（`youyuan_mcp_links.go:97`） |
| 云端 `read` 专用连接 / OAuth `mcp:read` | 表 B 的 5 个 | 其余全部：`tools/call` 返回 `_meta.mcp/www_authenticate` 要求补授权（`mcp-discovery.mjs:9-12`） |
| QQ 群内 `mcp_list`/`mcp_describe`/`mcp_call` | 只有管理员可见；除非登记了 `public`+`all` 的服务器 | 需在 `/api/youyuan/mcp/scope` 登记（`youyuan_mcp_scope.go:303-310`、`:631-638`） |

### 5.3 开关字段叫什么

| 接口 | 字段 | 取值 | 作用 | 源码 |
|---|---|---|---|---|
| `POST /api/youyuan/mcp-links`（本机） | `scope` | `read` / `control` | 生成只读或管理专用连接 | `upstream/youyuan_mcp_links.go:136-143` |
| 同上 | `hours` / `name` | 1–8760（默认 720） / 任意名 | 有效期与备注 | `:144-150` |
| `POST /api/identity/mcp-links`（云端） | `scope` / `hours` / `name` / `device` | 同上 / 设备名 | 绑定设备的专用连接 | `cloudflare/identity.mjs:85-90` |
| OAuth 授权 | `scope` | `mcp:read` / `mcp:control` | 授权粒度 | `cloudflare/oauth.mjs:64-68` |
| `POST /api/youyuan/mcp/scope`（外部 MCP 归属） | `scope` | `public` / `private` | 全群可用 / 仅归属人 | `upstream/youyuan_mcp_scope.go:35-36` |
| 同上 | `access` | `all` / `admin` / `owner_only` | 调用者等级 | `:38-40`、`:252-288` |
| 同上 | `enabled` / `owner` | bool / 群友 ID | 启停与归属 | `:689-712` |
| AI 工作区（非 MCP 服务端） | `allowed_tools` | 工具名数组 | 只约束内置 agent，不改变 MCP 暴露面 | `upstream/youyuan_agent.go:47`、`:1221` |

### 5.4 写操作人工修正清单

以下 20 个 handler 不校验 `r.Method`，按路径语义判为写操作（§2 表中已标 `是`）：

`api_draw_mode`、`api_draw_palette`、`api_preview_cancel`、`api_youyuan_chains`、`api_youyuan_identity`、`api_music_next`、`api_music_prev`、`api_youyuan_agent_tool`、`api_youyuan_agent_file`、`api_plugin_config`、`api_youyuan_convert_commands`、`api_youyuan_convert_structure`、`api_youyuan_files`、`api_youyuan_runtime_files`、`api_qq_claim_qr`、`api_youyuan_agent_conversation`、`api_youyuan_agent_validate`、`api_permission_storage`、`api_youyuan_accounts_bridge_profiles`、`api_youyuan_plugin_outputs`

反向风险：部分被判 `否` 的接口内部仍可能产生副作用（例如 `api_cb_import_preview` 会解析并落临时文件）。调用前请以 handler 源码为准，不要只看本表的列。

## 6. 验收方法（可直接复制）

顺序固定为 **list tools → list resources → health → status**，任一步失败就先停下，不要继续往下猜。

### 6.0 变量与占位符

```bash
# 云端稳定地址（中继；设备必须已连接）
BASE='https://toolbox.youyuanqi.dpdns.org'
DEVICE='linux-server'
# 占位符：替换成你自己的凭证，切勿提交到仓库/群聊
#   MCP_TOKEN = 平台账号的云端 MCP 控制密钥（Bearer）
MCP_TOKEN='<MCP_TOKEN>'

H1="Authorization: Bearer $MCP_TOKEN"
H2='Content-Type: application/json'
H3='Accept: application/json, text/event-stream'
H4='MCP-Protocol-Version: 2025-06-18'
```

| 占位符 | 填什么 | 不能填什么 |
|---|---|---|
| `<MCP_TOKEN>` | 云端：平台账号控制密钥；本机：界面显示的「本机 MCP 密钥」 | 游戏令牌 `yy//…`、模型 API Key、`device_token` |
| `DEVICE` | 执行端桶名，如 `linux-server`、`primary-device` | 不要带 `/` 或空格（`worker.mjs:105` 校验 `[A-Za-z0-9_-]{1,48}`） |
| `<LINK_URL>` | 形如 `https://…/mcp/link/<64hex>` 的专用连接 | 不要同时再填 Bearer |

### 6.1 列出 tools（期望 315）

```bash
curl -sS "$BASE/mcp/?device=$DEVICE" \n  -H "$H1" -H "$H2" -H "$H3" -H "$H4" \n  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \n  | jq '{count:(.result.tools|length), first:(.result.tools[0].name), last:(.result.tools[-1].name)}'
```

期望：`count = 315`，`first = api_ai_answer`，`last = api_call`。若 `count = 5`，说明当前是只读连接（§5 表 B），不是故障。

### 6.2 列出 resources（期望 39）

```bash
curl -sS "$BASE/mcp/?device=$DEVICE" \n  -H "$H1" -H "$H2" -H "$H3" -H "$H4" \n  -d '{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}' \n  | jq '{count:(.result.resources|length), uris:[.result.resources[].uri][0:3]}'
```

期望：`count = 39`。

### 6.3 health（期望 tools=315、ready=true）

```bash
curl -sS "$BASE/health" -H "$H1" | jq .
```

期望字段：`ready:true`、`platform:"linux"`、`tools:315`、`version:"0.10.78"`。`tools` 由 `len(yyAPIRoutes)+2` 实时算出（`upstream/youyuan_remote.go:84`），是最省事的计数校验。

> 云端 `/health` 也受鉴权保护（`worker.mjs:86`），必须带 Bearer；本机 `http://127.0.0.1:19429/health` 不需要。

### 6.4 status（真实游戏连接状态）

```bash
curl -sS "$BASE/mcp/?device=$DEVICE" \n  -H "$H1" -H "$H2" -H "$H3" -H "$H4" \n  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"api_call","arguments":{"path":"/api/bot/status","method":"GET"}}}' \n  | jq '{isError:.result.isError, meta:.result.structuredContent.mcp_meta, body:(.result.content[0].text|fromjson?)}'
```

期望：`isError:false`，`mcp_meta.outcome` 为正常值（非 `not_sent` / `timeout_unknown`），`body` 里有真实的 `connected` / 账号信息。**tools 可见 ≠ 机器人已进服**，这一步才是进服状态。

### 6.5 一次性握手自检（可选，客户端通常自动做）

```bash
curl -sS "$BASE/mcp/?device=$DEVICE" \n  -H "$H1" -H "$H2" -H "$H3" \n  -d '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"1.0"}}}' \n  | jq '{proto:.result.protocolVersion, server:.result.serverInfo.name, caps:.result.capabilities}'
```

期望：`proto:"2025-06-18"`、`server:"游园工具箱 MCP"`。若服务端不支持你传的版本，会回落到 `2025-06-18`（`upstream/youyuan_remote.go:330-332`）。

### 6.6 本机直连变体

```bash
curl -sS 'http://127.0.0.1:19429/mcp/' \n  -H 'Authorization: Bearer <LOCAL_MCP_KEY>' -H 'Content-Type: application/json' \n  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | jq '.result.tools|length'
```

### 6.7 常见非预期结果

| 现象 | 含义 | 处置 |
|---|---|---|
| HTTP 401 `MCP 密钥不正确` | Bearer 不对/重复写 Bearer/混用了游戏令牌 | 换正确密钥，只写一次 `Bearer` |
| HTTP 405 | 用 GET 打开了 `/mcp` | 必须 POST（服务端不提供 SSE GET 流，`youyuan_remote.go:253-257`） |
| HTTP 503 `游园执行端未连接` | 设备没上线 | 先在 APK/Linux 登录并让公网连接显示已连接 |
| `count=5` | 走的是只读连接 | 需要写操作就新建 `control` 连接 |
| `-32003 此连接仅授权状态和开发文档查询` | 本机 read 连接调了白名单外工具 | 同上 |
| `-32601 未知方法` | 方法名拼错（如 `tool/list`） | 只支持 `initialize`/`ping`/`tools/list`/`tools/call`/`resources/list`/`resources/templates/list`/`resources/read` |

## 7. 文档与源码不符 / 文档缺口

| # | 文档说法 | 源码事实 | 判定 |
|---|---|---|---|
| 1 | 「本电脑已通过公网 OAuth 完成…**244 工具目录**」（`MCP接入教程.md:117`，0.10.13 时期） | 现为 313+2=**315**（`youyuan_catalog.go` 313 行 + `youyuan_remote.go:519-520`） | 数字过期，文档未同步 |
| 2 | 「只读连接仅提供连接状态、在线观测、插件运行状态、SDK 目录与开发文档」（`MCP接入教程.md:93`、`:235`） | 云端 5 条符合；**本机 `/mcp/link/` read 连接实际放行 35 条**，还含种子查询、QQ 知识库、皮肤纹理、MCORT 公告（`youyuan_mcp_links.go:26-64`） | 口径不一致，文档只描述了云端 |
| 3 | 「scope=mcp:read 只列出 **5** 个只读工具」（`MCP接入教程.md:110`） | 云端 `readTools` 确为 5（`mcp-link-policy.mjs:1`）；执行端 `yyMCPReadPaths` 为 35 | 同上，两处「只读」不是同一集合 |
| 4 | 「Resources 中可读 python-runtime / cpp-runtime / command-chain-api / physical-actions 等」（`MCP接入教程.md:68`、`:210`） | 实际 39 条，另含 `youyuan://api/catalog`、`youyuan://runtime/status`、`youyuan://accounts/status`、`youyuan://skills/catalog`、`youyuan://packets/catalog` 与 19 条技能文档 | 文档只举例，未列全 |
| 5 | 未提及 | 源码注册了 5 个 `api_mcort_*`（announcements/plugins/theme/theme-asset/themes）、`api_youyuan_mcp_scope`、`api_youyuan_mcp_links`、`api_youyuan_mcp_traces`、`api_youyuan_observations_*`、`api_seed_v2_*`、`api_skin_item_*` 等 | 文档从未逐条列 315 个工具（设计上靠 `api_catalog` 自省），属可接受缺口 |
| 6 | 「mcp.call(tool,args)，参数例如 `{method="GET"}`」（`R21A常驻与本机MCP开发说明.md:44-46`） | 实际 arguments 必须是 JSON 对象；且宿主强制把 `account` 覆盖为当前实例，跨账号直接报错（`upstream/youyuan_plugin_mcp.go:52-65`） | 描述正确但示例易误读，需注意 `account` 不可指定 |
| 7 | 「每个平台账号/本机执行端最多 **128** 个同时有效的专用连接」（`MCP接入教程.md:87`） | 本机 `active >= 128` 拒绝（`youyuan_mcp_links.go:157`）；云端同阈值（`identity.mjs:88`） | 一致 |
| 8 | 「共享 Linux 入口允许匿名发现 initialize/ping/tools/list」（`MCP接入教程.md:133`） | `publicDiscoveryMethods` 仅 4 个方法，且仅对 `linux-server` 生效（`mcp-discovery.mjs:4`、`worker.mjs:24`） | 一致 |
| 9 | 「本机地址通常为 `http://127.0.0.1:19429/mcp/`」（`MCP接入教程.md:52`） | 默认端口 19429（`upstream/libmain.go:143`、`main_native.go:19`） | 一致 |

**没有发现「文档说有 tool 但源码没注册」的情况**：教程中出现的 `api_bot_status`、`api_plugin_api_doc`、`api_plugin_list`、`api_plugin_file_read`、`api_plugin_file_write`、`api_youyuan_docs`、`api_call`、`api_catalog`、`api_qq_messages`、`api_qq_assistant` 全部在 `yyAPIRoutes` 或追加列表中存在。

## 8. 计数校验

| 对象 | 计数 | 校验方式 |
|---|---|---|
| tools | **315** | `youyuan_catalog.go` 内 `{"api_…","/api/…"},` 共 313 行（第 4–316 行）+ `api_catalog`、`api_call`（`youyuan_remote.go:519-520`） |
| tools（运行时） | 315 | `GET /health` → `tools`（`len(yyAPIRoutes)+2`，`youyuan_remote.go:84`）；或 `tools/list` 后 `.result.tools\|length` |
| resources | **39** | 18 条硬编码（`youyuan_mcp_resources.go:13`、`:17-26`）+ 19 条 `aiSkills` 去重后补齐（`ai_skills.go:26-78`）+ 2 条目录型（`:37-38`） |
| 只读白名单（本机） | 35 | `yyMCPReadPaths`（`youyuan_mcp_links.go:26-64`） |
| 只读白名单（云端） | 5 | `readTools`（`cloudflare/mcp-link-policy.mjs:1`） |
| 专用连接上限 | 128 / 执行端 | `youyuan_mcp_links.go:157`、`identity.mjs:88` |

复核命令（只读，不联网）：

```bash
grep -c '^{\"api_' upstream/youyuan_catalog.go      # 期望 313
grep -c '"/api/[^\"]*\": true' upstream/youyuan_mcp_links.go   # 期望 35
```

---

## 附录 A：反方向的 MCP（别混淆）

本项目还有「游园 AI 连外部 MCP」的一面，与本文的「外部 AI 连游园」不是同一套：

| 项 | 内容 | 源码 |
|---|---|---|
| 客户端工具 | `mcp_list`、`mcp_describe`、`mcp_call`（QQ 侧下发） | `upstream/youyuan_mcp_scope.go:631-638` |
| 归属表 | `mcp-servers.json`，字段 `scope(public/private)`、`access(all/admin/owner_only)`、`transport(stdio/http/sse)`、`enabled` | `:9-10`、`:49-67` |
| 准入 | `yyMCPScopeGuard`：private 仅归属人；public 再按 access 判等级 | `:252-288` |
| 管理入口 | `GET/POST /api/youyuan/mcp/scope`，op = list/save/delete/quick/test | `:643-747` |
| 连接测试 | `initialize` → `notifications/initialized` → `tools/list`，返回 `tool_count` 与前 20 个名字 | `:526-606` |

外部 MCP 只对管理员下发，除非存在一台 `public`+`all` 的登记服务器（`:303-310`）。

