package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func yyDefaultCommandOptions() yyCommandOptions {
	return yyCommandOptions{Channel: "ai", Mode: "ordered", Window: 8, TimeoutMS: 3000}
}
func (s *yySDKContext) pluginCommandCall(method string, args []any) (any, error) {
	s.Mu.Lock()
	options, optionsError := s.CommandOptions, s.CommandOptionsError
	if options.Channel == "" {
		options = yyDefaultCommandOptions()
	}
	if method == "game.setCommandOptions" {
		if len(args) != 1 {
			s.Mu.Unlock()
			return nil, fmt.Errorf("setCommandOptions 需要 options 对象")
		}
		updated, err := yyMergeCommandOptions(options, args[0])
		if err == nil {
			s.CommandOptions, s.CommandOptionsError = updated, nil
		}
		s.Mu.Unlock()
		return yyCommandOptionsMap(updated), err
	}
	s.Mu.Unlock()
	if optionsError != nil {
		return nil, fmt.Errorf("command-options.json: %v", optionsError)
	}
	if method == "game.getCommandOptions" {
		return yyCommandOptionsMap(options), nil
	}
	if _, ok := yyAsyncDocs[method]; ok {
		return yyAsyncCommandCall(yyAsyncCommandKey{yyAccountBot(s.Instance.Account), s.Instance.Key}, method, args, options)
	}
	return yyPluginCommandCall(yyAccountBot(s.Instance.Account), method, args, options)
}
func yyCommandOptionsMap(options yyCommandOptions) map[string]any {
	return map[string]any{"channel": options.Channel, "mode": options.Mode, "transport": options.Transport, "window": options.Window, "timeout_ms": options.TimeoutMS, "rate_cps": options.RateCPS}
}

// A per-call override never changes another plugin instance's defaults.
func yyMergeCommandOptions(base yyCommandOptions, override any) (yyCommandOptions, error) {
	if text, ok := override.(string); ok {
		if text != "" && text != "default" && text != "默认" {
			base.Channel = text
		}
	} else if override != nil {
		object, ok := override.(map[string]any)
		if !ok {
			return base, fmt.Errorf("执行参数应为通道字符串或 options 对象")
		}
		if object["mode"] == "stream" && base.Mode != "stream" {
			if _, set := object["window"]; !set {
				base.Window = 128
			}
		}
		if object["mode"] == "burst" && base.Mode != "burst" {
			if _, set := object["window"]; !set {
				base.Window = 1024
			}
		}
		for key := range object {
			if key != "channel" && key != "mode" && key != "window" && key != "timeout_ms" && key != "transport" && key != "rate_cps" {
				return base, fmt.Errorf("未知执行参数 %q", key)
			}
		}
		if channel, ok := object["channel"].(string); ok && (channel == "default" || channel == "默认" || channel == "") {
			copy := make(map[string]any, len(object))
			for key, value := range object {
				if key != "channel" {
					copy[key] = value
				}
			}
			object = copy
		}
		data, err := json.Marshal(object)
		if err != nil {
			return base, err
		}
		if err = json.Unmarshal(data, &base); err != nil {
			return base, err
		}
	}
	channel, err := normalizeCommandChannel(base.Channel)
	if err != nil {
		return base, err
	}
	base.Channel = string(channel)
	if base.Mode != "ordered" && base.Mode != "pipeline" && base.Mode != "submit" && base.Mode != "burst" && base.Mode != "stream" {
		return base, fmt.Errorf("mode 应为 ordered/pipeline/submit/burst/stream")
	}
	if base.Window < 1 || base.Window > 1024 || (base.Mode != "burst" && base.Mode != "stream" && base.Window > 16) || base.TimeoutMS < 100 || base.TimeoutMS > 10000 {
		return base, fmt.Errorf("pipeline/submit window=1..16，burst/stream=1..1024，timeout_ms=100..10000")
	}
	if base.Transport == "" {
		base.Transport = "command_request"
	}
	if base.Transport != "command_request" && base.Transport != "settings" && base.Transport != "pyrpc" {
		return base, fmt.Errorf("transport 应为 command_request/settings/pyrpc")
	}
	if base.Transport != "command_request" && base.Mode != "submit" {
		return base, fmt.Errorf("settings/pyrpc 无逐条确认，仅支持 submit；条件链请使用带回包的 command_request")
	}
	if base.RateCPS != 0 && (base.RateCPS < 20 || base.RateCPS > 20000) {
		return base, fmt.Errorf("rate_cps=0（无固定CPS）或20..20000")
	}
	return base, nil
}

func yyPluginCommandCall(bm *BotManager, method string, args []any, defaults yyCommandOptions) (any, error) {
	if active, _ := yySpatialJobState(bm); active && len(args) > 0 {
		commands := []string{}
		if c, ok := args[0].(string); ok {
			commands = append(commands, c)
		}
		if cs, ok := args[0].([]any); ok {
			for _, v := range cs {
				if c, ok := v.(string); ok {
					commands = append(commands, c)
				}
			}
		}
		for _, c := range commands {
			if yyPluginTeleportCommand(bm, c) {
				return nil, yyPluginMovementAllowed(bm)
			}
		}
	}
	if method == "game.sendCommandConfirmed" {
		method = "game.execute"
	}
	if method == "game.sendCommandFast" {
		method = "game.sendCommand"
	}
	if method == "game.sendCommandsFast" {
		if len(args) < 1 {
			return nil, fmt.Errorf("sendCommandsFast 需要命令数组")
		}
		items, ok := args[0].([]any)
		if !ok {
			return nil, fmt.Errorf("sendCommandsFast 需要命令数组")
		}
		commands := make([]string, len(items))
		// Validate the complete buffer before submitting anything.
		for i, item := range items {
			command, ok := item.(string)
			if !ok || strings.TrimSpace(command) == "" || strings.ContainsAny(command, "\r\n") {
				return nil, fmt.Errorf("第%d条需要单条非空命令", i+1)
			}
			commands[i] = command
		}
		options := defaults
		var err error
		if len(args) > 1 {
			options, err = yyMergeCommandOptions(defaults, args[1])
			if err != nil {
				return nil, err
			}
		}
		if options.Transport != "" && options.Transport != "command_request" {
			return nil, fmt.Errorf("sendCommandsFast 使用默认快速通道；显式 settings/pyrpc 请用 executeBatch submit，不忽略 transport 参数")
		}
		if bm == nil || !bm.IsConnected() {
			return nil, fmt.Errorf("游戏账号未连接")
		}
		for i, command := range commands {
			if err = bm.SendCommandNoWaitRate(CommandChannel(options.Channel), command, options.RateCPS); err != nil {
				return map[string]any{"submitted": i, "commands": len(commands), "confirmed": false}, err
			}
		}
		return map[string]any{"submitted": len(commands), "commands": len(commands), "confirmed": false}, nil
	}
	if method == "game.executeBatch" {
		return yyCommandBatchDefaults(bm, args, defaults)
	}
	if defaults.Transport != "" && defaults.Transport != "command_request" {
		return nil, fmt.Errorf("settings/pyrpc 请通过 game.executeBatch 使用 submit 模式")
	}
	command, err := yySDKString(args, 0)
	if err != nil {
		return nil, err
	}
	command = strings.TrimSpace(strings.TrimLeft(command, "/"))
	if command == "" || strings.ContainsAny(command, "\r\n") {
		return nil, fmt.Errorf("需要一条非空命令，不得包含换行")
	}
	options := defaults
	if len(args) > 1 {
		options, err = yyMergeCommandOptions(defaults, args[1])
		if err != nil {
			return nil, err
		}
	}
	if options.Transport != "" && options.Transport != "command_request" {
		return nil, fmt.Errorf("settings/pyrpc 仅支持 game.executeBatch 的 submit 模式")
	}
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("游戏账号未连接")
	}
	if method == "game.execute" {
		if commands := yySplitConsole("/" + command); len(commands) > 1 {
			results := []any{}
			success, confirmed := true, true
			output := []string{}
			for _, entry := range commands {
				value, err := yyPluginCommandCall(bm, method, []any{entry, yyCommandOptionsMap(options)}, options)
				if err != nil {
					return nil, err
				}
				row := value.(map[string]any)
				results = append(results, row)
				if row["success"] != true {
					success = false
				}
				if row["confirmed"] != true {
					confirmed = false
				}
				if lines, ok := row["output"].([]string); ok {
					output = append(output, lines...)
				}
			}
			return map[string]any{"ok": true, "results": results, "commands": len(commands), "success": success, "confirmed": confirmed, "output": output, "channel": options.Channel}, nil
		} else if len(commands) == 1 {
			command = strings.TrimPrefix(commands[0], "/")
		}
	}
	channel := CommandChannel(options.Channel)
	if method == "game.sendCommand" {
		err = bm.SendCommandNoWaitRate(channel, command, options.RateCPS)
		return map[string]any{"submitted": err == nil, "confirmed": false, "channel": channel}, err
	}
	response, timedOut, err := bm.SendCommandCh(channel, command, time.Duration(options.TimeoutMS)*time.Millisecond)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"ok": true, "submitted": true, "confirmed": !timedOut && response != nil, "channel": channel}
	if timedOut || response == nil {
		result["unknown"] = true
		result["error"] = "命令响应超时，执行结果未知；没有自动重发"
		return result, nil
	}
	result["success"] = response.SuccessCount > 0
	result["success_count"] = response.SuccessCount
	output := []string{}
	for _, message := range response.OutputMessages {
		output = append(output, formatCommandMessage(message.Message, message.Parameters))
	}
	result["output"] = output
	return result, nil
}

func yyLuaCommandDefaults(L *lua.LState) yyCommandOptions {
	base := yyDefaultCommandOptions()
	if value := L.GetGlobal("__YY_COMMAND_OPTIONS__"); value != lua.LNil {
		if options, err := yyMergeCommandOptions(base, luaToGo(value)); err == nil {
			return options
		}
	}
	return base
}

func registerPluginCommandAPI(L *lua.LState, game *lua.LTable) {
	// Build in Lua; submitFast crosses the Lua/Go boundary only once per buffer.
	if err := L.DoString(`function CommandBuffer()
 local list={}
 return {add=function(self,cmd) assert(type(cmd)=="string","command需字符串");list[#list+1]=cmd;return self end,
 clear=function(self) list={};return self end,size=function(self)return #list end,
 submitFast=function(self,options)return game.sendCommandsFast(list,options)end}
 end`); err != nil {
		panic(err)
	}
	L.SetField(game, "getCommandOptions", L.NewFunction(func(L *lua.LState) int { L.Push(goToLua(L, yyCommandOptionsMap(yyLuaCommandDefaults(L)))); return 1 }))
	L.SetField(game, "setCommandOptions", L.NewFunction(func(L *lua.LState) int {
		options, err := yyMergeCommandOptions(yyLuaCommandDefaults(L), luaToGo(L.Get(1)))
		if err != nil {
			L.RaiseError("%v", err)
			return 0
		}
		L.SetGlobal("__YY_COMMAND_OPTIONS__", goToLua(L, yyCommandOptionsMap(options)))
		L.Push(goToLua(L, yyCommandOptionsMap(options)))
		return 1
	}))
	for _, method := range []string{"game.sendCommand", "game.sendCommandFast", "game.sendCommandsFast", "game.sendCommandConfirmed", "game.sendCommandAsync", "game.getBlockTileAsync", "game.commandResult", "game.execute", "game.executeBatch"} {
		method := method
		L.SetField(game, strings.TrimPrefix(method, "game."), L.NewFunction(func(L *lua.LState) int {
			if method == "game.sendCommand" && L.GetTop() == 1 && L.GetGlobal("__YY_COMMAND_OPTIONS__") == lua.LNil {
				cmd := L.CheckString(1)
				bm, hook := abot(L), getSendCommandHook()
				_, e := yyLuaWaitValue(L, func(_ context.Context) (bool, error) {
					if hook != nil {
						return true, hook(cmd)
					}
					return true, bm.SendCommandNoWait(ChannelAI, cmd)
				})
				if e != nil {
					L.RaiseError("sendCommand: %v", e)
				}
				return 0
			}
			args := []any{}
			for index := 1; index <= L.GetTop(); index++ {
				args = append(args, luaToGo(L.Get(index)))
			}
			if method == "game.sendCommand" {
				if hook := getSendCommandHook(); hook != nil {
					command := L.CheckString(1)
					if _, err := yyLuaWaitValue(L, func(_ context.Context) (bool, error) { return true, hook(command) }); err != nil {
						L.RaiseError("sendCommand: %v", err)
					}
					return 0
				}
			}
			var result any
			var err error
			bm, options := abot(L), yyLuaCommandDefaults(L)
			owner := yyAsyncCommandKey{bm, lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))}
			result, err = yyLuaWaitValue(L, func(_ context.Context) (any, error) {
				if _, ok := yyAsyncDocs[method]; ok {
					return yyAsyncCommandCall(owner, method, args, options)
				}
				return yyPluginCommandCall(bm, method, args, options)
			})
			if err != nil {
				L.RaiseError("%s: %v", method, err)
				return 0
			}
			L.Push(goToLua(L, result))
			return 1
		}))
	}
}
