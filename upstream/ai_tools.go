package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// aiToolHandler 执行一个工具调用，返回 (结果文本, 是否错误)。
type aiToolHandler func(args map[string]any) (string, bool)

// aiToolHandlers 工具名 → 执行器。
var aiToolHandlers = map[string]aiToolHandler{
	"network_request":      toolNetworkRequest,
	"list_plugins":         toolListPlugins,
	"read_plugin":          toolReadPlugin,
	"write_plugin_file":    toolWritePluginFile,
	"game_call":            toolGameCall,
	"call_plugin_function": toolCallPluginFunction,
	"lua_call":             toolLuaCall,
	// 观察/调试（不 gate OP，连上即可用）
	"wait_chat":      toolWaitChat,
	"listen_packet":  toolListenPacket,
	"list_packets":   toolListPackets,
	"system_shell":   toolSystemShell,
	"shizuku_status": toolShizukuStatus,
	// 技能（由 AI 自动加载）
	"list_skills": toolListSkills,
	"load_skill":  toolLoadSkill,
	// CLI 式沙箱文件工具
	"read_file":  toolReadFile,
	"write_file": toolWriteFile,
	"edit_lines": toolEditLines,
	"list_dir":   toolListDir,
	"read_log":   toolReadLog,
}

// aiToolDefs 返回暴露给模型的工具定义（JSON Schema parameters 用 map 组装）。
func aiToolDefs() []ToolDef {
	return []ToolDef{
		{
			Name:        "network_request",
			Description: "发起任意 HTTP 请求（GET/POST/PUT/PATCH/DELETE）。返回响应体文本（非 2xx 也会返回状态码与响应体）。用于访问外部 API、网页等。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"method":  map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}, "description": "请求方法，默认 GET"},
					"url":     map[string]any{"type": "string", "description": "完整 URL"},
					"headers": map[string]any{"type": "object", "description": "请求头，键值对"},
					"body":    map[string]any{"type": "string", "description": "请求体（字符串）"},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "list_skills",
			Description: "列出可加载的技能知识库（id/名称/描述）。需要专项知识（如基岩版指令、FMbe 显示实体动画）时，先列出来再用 load_skill 加载。",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "load_skill",
			Description: "分页读取玩家已经加载的离线技能。skill 传 id 或名称，section 选择章节；不能替玩家开启技能。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"skill":   map[string]any{"type": "string", "description": "技能 id 或名称"},
					"section": map[string]any{"type": "string"}, "offset": map[string]any{"type": "integer"}, "max_chars": map[string]any{"type": "integer"},
				},
				"required": []string{"skill"},
			},
		},
		{
			Name:        "list_plugins",
			Description: "列出当前所有已安装插件（id/名称/类型/描述）。用于了解已有插件，避免重复、参考他人实现。",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "read_plugin",
			Description: "读取某个插件的源码代码 / 插件说明文档 / 配置文件 / 数据目录文件。用于理解现有插件、参考写法。scope 为 code(源码) / docs(说明) / config(配置) / data(数据文件)。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plugin_id": map[string]any{"type": "string", "description": "插件 id"},
					"scope":     map[string]any{"type": "string", "enum": []string{"code", "docs", "config", "data"}, "description": "读取范围"},
					"file":      map[string]any{"type": "string", "description": "scope=data 时的数据文件名"},
				},
				"required": []string{"plugin_id", "scope"},
			},
		},
		{
			Name:        "write_plugin_file",
			Description: "把内容直接写入某个插件的源码代码 / 插件说明文档 / 数据文件（覆盖）。写入代码后如果机器人在线会尝试热重载。用于直接把写好的插件代码保存到插件文件。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plugin_id": map[string]any{"type": "string", "description": "插件 id"},
					"scope":     map[string]any{"type": "string", "enum": []string{"code", "docs", "data"}, "description": "写入范围：code=插件源码，docs=插件说明，data=数据文件"},
					"file":      map[string]any{"type": "string", "description": "scope=data 时的数据文件名；code/docs 留空"},
					"content":   map[string]any{"type": "string", "description": "要写入的完整内容"},
				},
				"required": []string{"plugin_id", "scope", "content"},
			},
		},
		{
			Name:        "game_call",
			Description: "调用游戏内机器人 API（需机器人已连接）。参数用函数名与位置参数数组 args。指令/消息：sendCommand/sendChat/sendTitle/sendActionbar/sendRichTellraw/sendTargeted；查询：getPlayerPos/getPlayerPosXYZ/getScore/getMultiScore/getBotPos/getTPS/getTarget/getTickingAreaList/isCmdSuccess；记分板写入：setScore(board,target,score)；物品：getPlayerItem(target,itemName,specialID)/queryInventory(selector)；移动飞行：teleport/move/rotate(pitch,yaw[,秒数] 直接调或秒内平滑转)/jump/flyStart/flyStop。方块数据：getBlockTile(x,y,z)/getBlockTileSafe(x,y,z) 读方块名；getBlockData(x,y,z) 读名称+方块状态+NBT（命令方块指令、箱子物品、告示牌文字、旗帜图案都在返回的 nbt 里）；getBlockNBT(x,y,z) 只读 NBT；getBlockState(x,y,z) 读名称+状态；getRegionBlockData(x,y,z,sx,sy,sz) 读一块区域所有方块数据。放置/修改：setBlock(x,y,z,name,states) 放普通方块；placeNBTBlock(x,y,z,name,states,nbt对象) 放命令方块/箱子/告示牌/旗帜等带NBT方块（nbt 用 {Command=...,Items=...,...} 这种对象，告示牌文字用 {FrontText={Text=...}}）；setBlockState(x,y,z,states对象) 改方块状态并保留NBT。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function": map[string]any{"type": "string", "description": "原 game 函数名如 execute，或完整名如 chains.list/chains.configure/bot.attack/world.mineBlock/world.useBlock。按需加载对应技能，不猜参数"},
					"args":     map[string]any{"type": "array", "items": map[string]any{}, "description": "位置参数，按各函数要求传"},
				},
				"required": []string{"function", "args"},
			},
		},
		{
			Name:        "call_plugin_function",
			Description: "直接调用某个已加载插件里的函数（按名字，任意函数都行——先解析插件全局函数，其次 plugin_api 导出）。用于测试插件：比如调用插件的 openMenu(player) 看菜单能不能弹。args 是位置参数数组。返回函数的返回值(JSON)。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"plugin_id": map[string]any{"type": "string", "description": "插件 id（可用 list_plugins 查）"},
					"func_name": map[string]any{"type": "string", "description": "要调用的插件函数名"},
					"args":      map[string]any{"type": "array", "items": map[string]any{}, "description": "位置参数数组"},
				},
				"required": []string{"plugin_id", "func_name"},
			},
		},
		{
			Name:        "lua_call",
			Description: "直接调用开发文档里的任意 Lua API 函数（不写进插件）：module 是全局模块名，func 是函数名，args 是位置参数数组。可用模块：game(游戏操作)/system(系统shell/shizuku)/util(定时)/world(世界方块)/bot(多机器人)/building(建筑)/media(媒体)/player(玩家数据)/http(网络)/web(网页)/api(跨插件)/events(事件)。例：调 system.shell 传 {module=system, func=shell, args=[\"id\"]}；调 game.getBlockData 传 {module=game, func=getBlockData, args=[x,y,z]}。返回函数返回值(JSON)；无返回值返回 nil。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"module": map[string]any{"type": "string", "description": "全局模块名：game/system/util/world/bot/building/media/player/http/web/api/events"},
					"func":   map[string]any{"type": "string", "description": "模块里的函数名"},
					"args":   map[string]any{"type": "array", "items": map[string]any{}, "description": "位置参数数组"},
				},
				"required": []string{"module", "func"},
			},
		},
		{
			Name:        "wait_chat",
			Description: "等待一段时间内的游戏聊天消息并返回给 AI（含玩家与服务器广播）。用于测试插件触发词/菜单是否正确触发：先让机器人发一条指令/说话，再 wait_chat 收集后续聊天看有没有预期回显。不要求 OP，机器人连上即可。返回 [{player,message},...]。seconds 为等待秒数(1-60，默认5)，max_count 为最多收集条数(0不限)。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"seconds":   map[string]any{"type": "integer", "description": "等待秒数(1-60)，默认5"},
					"max_count": map[string]any{"type": "integer", "description": "最多收集条数，0=不限，默认0"},
				},
			},
		},
		{
			Name:        "listen_packet",
			Description: "订阅一个数据包类型并等待接收，返回收到的完整数据（JSON）。用于观察服务器下发给机器人的原始数据包，调试插件/协议。packet_type 为包类型名（如 MovePlayer/SetTime/PlayerList/Text），可通过 list_packets 查看类型名。seconds 为等待秒数(1-60，默认5)，max_count 为最多收集条数。不要求 OP。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"packet_type": map[string]any{"type": "string", "description": "包类型名，如 MovePlayer / SetTime / PlayerList"},
					"seconds":     map[string]any{"type": "integer", "description": "等待秒数(1-60)，默认5"},
					"max_count":   map[string]any{"type": "integer", "description": "最多收集条数，0=不限，默认0"},
				},
				"required": []string{"packet_type"},
			},
		},
		{
			Name:        "list_packets",
			Description: "列出可订阅的数据包类型名（供 listen_packet 的 packet_type 使用）及用途。订阅/调试协议前先查。",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "system_shell",
			Description: "以 Shizuku ADB/root 权限执行一条 shell 命令并返回结果 {ok,exit,stdout,stderr,uid}。用于调试插件/系统（Android 上需已装 Shizuku 并授权，可先调 shizuku_status 确认）。桌面环境不支持返回失败。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string", "description": "要执行的 shell 命令"},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "shizuku_status",
			Description: "查询 Shizuku 可用/授权状态 {available,granted,uid}。用 system_shell 前先确认。",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

// aiToolDefs 返回全部工具定义（基础 + 文件工具 + 用户询问）。
func aiToolDefsAll() []ToolDef {
	return append(append(aiToolDefs(), ToolDef{Name: "api_call", Description: "直接调用游园工具箱所有已注册 API。可操作音乐、导入导出、多机器人、飞行、群服互通等；通过 /api/youyuan/catalog 查询路径。", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "method": map[string]any{"type": "string", "enum": []string{"GET", "POST"}}, "body": map[string]any{"type": "object"}}, "required": []string{"path"}}}), append(aiFileToolDefs(), ToolDef{
		Name:        "user_question",
		Description: "向用户提问（带选项），让用户快速确认意图/选择方案。用于需求不清、有多个做法、或需要用户拍板时。问题简短，选项 2-4 个。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{"type": "string", "description": "要问的问题"},
				"options":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "选项列表"},
			},
			"required": []string{"question"},
		},
	}, ToolDef{
		Name:        "rename_session",
		Description: "重命名当前会话标题为一个更贴切的标题（title）。用于给当前对话起名。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string", "description": "新的会话标题"},
			},
			"required": []string{"title"},
		},
	})...)
}

// execTool 执行工具并返回结果。
func execTool(name string, args map[string]any) (string, bool) {
	if name == "api_call" {
		path := aiArgStr(args, "path")
		method := aiArgStr(args, "method")
		if method == "" {
			method = "GET"
		}
		if path == "/api/ai/chat" {
			return "禁止 AI 对话递归调用自己", true
		}
		status, data, err := yyInvoke(method, path, nil, args["body"])
		if err != nil {
			return err.Error(), true
		}
		var result map[string]any
		_ = json.Unmarshal(data, &result)
		return string(data), status >= 400 || result["ok"] == false
	}
	h, ok := aiToolHandlers[name]
	if !ok {
		return "未知工具: " + name, true
	}
	return h(args)
}

// aiArgStr / aiArgInt 安全取参。
func aiArgStr(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}
func aiArgInt(args map[string]any, key string, def int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
	case int:
		return v
	}
	return def
}

// ── network_request ───────────────────────────────────────────────────

func toolNetworkRequest(args map[string]any) (string, bool) {
	method := strings.ToUpper(aiArgStr(args, "method"))
	if method == "" {
		method = "GET"
	}
	rawURL := aiArgStr(args, "url")
	if rawURL == "" {
		return "参数 url 不能为空", true
	}
	body := aiArgStr(args, "body")
	headers := map[string]string{}
	if hm, ok := args["headers"].(map[string]any); ok {
		for k, v := range hm {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}
	status, _, text, err := httpRequestFull(method, rawURL, body, headers, 30*time.Second)
	if err != nil {
		return "请求失败: " + err.Error(), true
	}
	if len(text) > 12000 {
		text = text[:12000] + "\n…(已截断)"
	}
	return fmt.Sprintf("HTTP %d\n%s", status, text), false
}

// ── list_plugins ──────────────────────────────────────────────────────

func toolListPlugins(args map[string]any) (string, bool) {
	statuses := pluginController.List()
	if len(statuses) == 0 {
		return "暂无已安装插件", false
	}
	type item struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Running     bool   `json:"running"`
	}
	var out []item
	for _, s := range statuses {
		out = append(out, item{ID: s.ID, Name: s.Name, Type: s.Type, Description: s.Description, Running: s.Running})
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	return string(data), false
}

// ── 技能（AI 自动加载） ──────────────────────────────────────────────

func toolListSkills(args map[string]any) (string, bool) {
	type item struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	out := make([]item, 0, len(aiSkills))
	for _, s := range aiSkills {
		out = append(out, item{s.ID, s.Name, s.Description})
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	return string(data), false
}

func toolLoadSkill(args map[string]any) (string, bool) {
	key := strings.TrimSpace(aiArgStr(args, "skill"))
	if key == "" {
		return "参数 skill 不能为空", true
	}
	sk := resolveSkill(key)
	if sk == nil {
		return "未知技能: " + key + "（先用 list_skills 查看可用技能）", true
	}
	skillStore.mu.Lock()
	loaded := skillStore.loaded[sk.ID]
	skillStore.mu.Unlock()
	if !loaded {
		return "请玩家先在技能库加载这份技能", true
	}
	content, err := yyAgentSkillText(sk.ID, aiArgStr(args, "section"), aiArgInt(args, "offset", 0), aiArgInt(args, "max_chars", 3000))
	if err != nil {
		return "加载失败: " + err.Error(), true
	}
	return content, false
}

// ── read_plugin ───────────────────────────────────────────────────────

func toolReadPlugin(args map[string]any) (string, bool) {
	id := aiArgStr(args, "plugin_id")
	scope := aiArgStr(args, "scope")
	if id == "" {
		return "参数 plugin_id 不能为空", true
	}
	switch scope {
	case "code", "docs", "data":
		_, content, err := pluginController.ReadPluginFile(scope, id, aiArgStr(args, "file"))
		if err != nil {
			return "读取失败: " + err.Error(), true
		}
		return content, false
	case "config":
		cfg, err := pluginController.ReadConfig(id)
		if err != nil {
			return "读取配置失败: " + err.Error(), true
		}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		return string(data), false
	default:
		return "scope 必须是 code/docs/config/data 之一", true
	}
}

// ── write_plugin_file ─────────────────────────────────────────────────

func toolWritePluginFile(args map[string]any) (string, bool) {
	id := aiArgStr(args, "plugin_id")
	scope := aiArgStr(args, "scope")
	content := aiArgStr(args, "content")
	if id == "" {
		return "参数 plugin_id 不能为空", true
	}
	if scope != "code" && scope != "docs" && scope != "data" {
		return "scope 必须是 code/docs/data 之一", true
	}
	if err := pluginController.WritePluginFile(scope, id, aiArgStr(args, "file"), content); err != nil {
		return "写入失败: " + err.Error(), true
	}
	// 代码写入后，机器人在线则尝试热重载（未在线/失败静默，下次加载生效）
	if scope == "code" {
		botMgrMu.Lock()
		bm := botMgr
		online := bm != nil && bm.IsConnected()
		botMgrMu.Unlock()
		if online {
			_ = pluginController.Reload(bm, id)
		}
	}
	return "已写入插件 " + id + " 的 " + scope + " 文件", false
}

// 供 ai_chat 判断机器人是否可用于 game 工具。
func botOnline() bool {
	botMgrMu.Lock()
	defer botMgrMu.Unlock()
	return botMgr != nil && botMgr.IsConnected()
}

// ── 观察/调试工具（不 gate OP，连上即可用） ────────────────────────────

// toolWaitChat 等待一段时间内的聊天消息并返回给 AI（测插件触发词/菜单回显）。
func toolWaitChat(args map[string]any) (string, bool) {
	seconds := aiArgInt(args, "seconds", 5)
	max := aiArgInt(args, "max_count", 0)
	seconds = clampTimeout(seconds)
	obs := newAIObserver(max)
	unsub := chatHubSubscribe(func(bm *BotManager, player, msg string) {
		obs.feed(map[string]any{"player": player, "message": msg})
	})
	defer unsub()
	items := obs.wait(time.Duration(seconds) * time.Second)
	if items == nil {
		items = []map[string]any{} // 空也返回 []，不返回 null
	}
	data, _ := json.Marshal(items)
	return string(data), false
}

// toolListenPacket 订阅一个数据包类型并等待接收，返回收到的完整数据。
func toolListenPacket(args map[string]any) (string, bool) {
	ptype := strings.TrimSpace(aiArgStr(args, "packet_type"))
	if ptype == "" {
		return "参数 packet_type 不能为空（如 MovePlayer / SetTime / PlayerList）", true
	}
	if !yyPacketTypeKnown(ptype) {
		return "未知数据包类型 " + ptype + "，请用 list_packets 或 packet_schema 查询真实 Go 类型名", true
	}
	owner := getBotManagerLocked()
	if owner == nil || !owner.IsConnected() {
		return "机器人未连接，不能开始抓包", true
	}
	seconds := aiArgInt(args, "seconds", 5)
	max := aiArgInt(args, "max_count", 0)
	seconds = clampTimeout(seconds)
	obs := newAIObserver(max)
	unsub := packetHubSubscribe(ptype, func(bm *BotManager, data map[string]any) {
		if bm == owner {
			obs.feed(data)
		}
	})
	defer unsub()
	items := obs.wait(time.Duration(seconds) * time.Second)
	if items == nil {
		items = []map[string]any{} // 空也返回 []，不返回 null
	}
	data, _ := json.Marshal(items)
	return string(data), false
}

// clampTimeout 限制等待窗口 1-60 秒，避免 AI 工具把对话挂太久。
func clampTimeout(seconds int) int {
	if seconds <= 0 {
		return 5
	}
	if seconds > 60 {
		return 60
	}
	return seconds
}

// toolCallPluginFunction 直接调用插件里任意函数（先全局作用域，再 plugin_api 导出）。
func toolCallPluginFunction(args map[string]any) (string, bool) {
	id := aiArgStr(args, "plugin_id")
	fnName := aiArgStr(args, "func_name")
	if id == "" {
		return "参数 plugin_id 不能为空（可用 list_plugins 查）", true
	}
	if fnName == "" {
		return "参数 func_name 不能为空", true
	}
	var argsJSON string
	if a, ok := args["args"].([]any); ok {
		j, err := json.Marshal(a)
		if err != nil {
			return "参数序列化失败: " + err.Error(), true
		}
		argsJSON = string(j)
	}
	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()
	res, err := callPluginFunctionAny(bm, id, fnName, argsJSON)
	if err != nil {
		return "调用失败: " + err.Error(), true
	}
	return res, false
}

// toolLuaCall 直接调用开发文档里的任意 Lua API 函数（game./system./util./world. 等），
// 不经过插件——newLuaState 注册全套 API 并带超时保护。
func toolLuaCall(args map[string]any) (string, bool) {
	module := aiArgStr(args, "module")
	fnName := aiArgStr(args, "func")
	if module == "" || fnName == "" {
		return "参数 module 与 func 不能为空（如 game/getBlockData）", true
	}
	botMgrMu.Lock()
	bm := botMgr
	botMgrMu.Unlock()

	L := newLuaState(bm)
	defer L.Close()
	defer cancelLuaCtx(L)
	L.SetGlobal("__PLUGIN_ID__", lua.LString("ai"))

	tbl := L.GetGlobal(module)
	if tbl == lua.LNil {
		return "未知模块: " + module, true
	}
	fn := L.GetField(tbl, fnName)
	if fn == lua.LNil {
		return "模块 " + module + " 无函数 " + fnName, true
	}
	var argv []lua.LValue
	if a, ok := args["args"].([]any); ok {
		for _, v := range a {
			argv = append(argv, goToLua(L, v))
		}
	}
	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, argv...); err != nil {
		return "调用 " + module + "." + fnName + " 出错: " + err.Error(), true
	}
	ret := L.Get(-1)
	L.Pop(1)
	if ret == lua.LNil {
		return "nil", false
	}
	if j, err := json.Marshal(luaToGo(ret)); err == nil {
		return string(j), false
	}
	return lua.LVAsString(ret), false
}

// toolListPackets 返回常用可订阅数据包类型名（调试用，非全量）。
func toolListPackets(args map[string]any) (string, bool) {
	list := []map[string]string{
		{"type": "SetTime", "desc": "游戏时间"},
		{"type": "MovePlayer", "desc": "玩家移动/传送"},
		{"type": "PlayerList", "desc": "玩家列表增删"},
		{"type": "AddPlayer", "desc": "玩家进入服务器"},
		{"type": "Text", "desc": "聊天/系统消息"},
		{"type": "UpdateAbilities", "desc": "权限/能力变化"},
		{"type": "AdventureSettings", "desc": "冒险设置/命令权限"},
		{"type": "CommandOutput", "desc": "命令执行结果"},
		{"type": "InventoryContent", "desc": "背包/容器内容"},
		{"type": "MobEquipment", "desc": "手持物品"},
		{"type": "BlockActorData", "desc": "方块实体 NBT"},
		{"type": "LevelEvent", "desc": "世界事件(音效/粒子)"},
		{"type": "Respawn", "desc": "重生/位置更新"},
		{"type": "GameRulesChanged", "desc": "游戏规则变化"},
		{"type": "ContainerOpen", "desc": "容器打开"},
		{"type": "ContainerClose", "desc": "容器关闭"},
	}
	data, _ := json.MarshalIndent(list, "", "  ")
	return string(data), false
}

// toolSystemShell 以 Shizuku ADB/root 权限执行 shell 命令（复用 callAndroidSystem → FloatingBridge.runShell）。
func toolSystemShell(args map[string]any) (string, bool) {
	cmd := strings.TrimSpace(aiArgStr(args, "command"))
	if cmd == "" {
		return "参数 command 不能为空", true
	}
	res, err := callAndroidSystem("shell", mustJSON(map[string]any{"command": cmd}))
	if err != nil {
		return "系统交互失败（system.shell 仅 Android 支持）: " + err.Error(), true
	}
	return res, false
}

// toolShizukuStatus 查询 Shizuku 授权状态。
func toolShizukuStatus(args map[string]any) (string, bool) {
	res, err := callAndroidSystem("shizukuStatus", "{}")
	if err != nil {
		return "系统交互失败: " + err.Error(), true
	}
	return res, false
}
