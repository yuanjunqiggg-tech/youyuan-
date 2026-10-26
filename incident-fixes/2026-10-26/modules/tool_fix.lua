-- modules/tool_fix.lua
-- 专项加载 / 工具标签 / 指令回执 / LIST 兜底 / on_chat

local M = {}

local function is_fn(v) return type(v) == "function" end

local function log(msg)
  pcall(function()
    if type(log_event) == "function" then log_event("TOOL_FIX", tostring(msg)) end
    if util and is_fn(util.json_write) and game and is_fn(game.dataDir) then
      util.json_write(game.dataDir() .. "/tool_fix_last.json", {
        at = (util.timestamp and util.timestamp()) or os.time(),
        msg = tostring(msg),
      })
    end
  end)
end

local function tell(name, text)
  name, text = tostring(name or ""), tostring(text or "")
  if name == "" or text == "" then return end
  if is_fn(send) then local ok = send(name, text) if ok then return end end
  local safe = text:gsub("\\", "\\\\"):gsub('"', '\\"'):gsub("\n", "\\n")
  local n = name:gsub('"', '\\"')
  local cmd = string.format('tellraw "%s" {"rawtext":[{"text":"%s"}]}', n, safe)
  if game and is_fn(game.sendCommandFast) then pcall(game.sendCommandFast, cmd)
  elseif game and is_fn(game.sendCommand) then pcall(game.sendCommand, cmd) end
end

local function local_list()
  if is_fn(online_players) then
    local ok, names = pcall(online_players)
    if ok and type(names) == "table" and #names > 0 then
      return "当前在线玩家：" .. table.concat(names, "、")
    end
  end
  if game and is_fn(game.getOnlinePlayers) then
    local ok, names = pcall(game.getOnlinePlayers)
    if ok and type(names) == "table" and #names > 0 then
      local out = {}
      for i, v in ipairs(names) do out[i] = tostring(v) end
      return "当前在线玩家：" .. table.concat(out, "、")
    end
  end
  return "当前没有检测到在线玩家（本地 LIST）"
end

local function format_cmd_resp(res)
  if res == nil then return "(空回执)" end
  if type(res) == "string" then return res end
  if type(res) == "boolean" then return res and "success=true" or "success=false" end
  if type(res) == "table" then
    local parts = {}
    if res.success ~= nil then parts[#parts + 1] = "success=" .. tostring(res.success) end
    if res.ok ~= nil then parts[#parts + 1] = "ok=" .. tostring(res.ok) end
    local msg = res.message or res.msg or res.text or res.output or res.raw or res.result or res.error
    if msg ~= nil then parts[#parts + 1] = tostring(msg) end
    if res.success_count ~= nil then parts[#parts + 1] = "success_count=" .. tostring(res.success_count) end
    if #parts > 0 then return table.concat(parts, " | ") end
    local ok, enc = pcall(function() return http.json_encode(res) end)
    if ok and type(enc) == "string" then return enc:sub(1, 300) end
    return tostring(res)
  end
  return tostring(res)
end

local function exec_with_result(cmd)
  cmd = tostring(cmd or ""):gsub("^/+", ""):gsub("^%s+", ""):gsub("%s+$", "")
  if cmd == "" then return false, "空指令", nil end

  if game and is_fn(game.sendCommandWithResp) then
    local ok, res = pcall(game.sendCommandWithResp, cmd, "ai", 15)
    if ok then
      local text = format_cmd_resp(res)
      if type(res) == "string" and res:find("方法未实现", 1, true) then
        -- fall through
      else
        local success = true
        if type(res) == "table" and (res.success == false or res.ok == false) then success = false end
        if type(res) == "string" then
          local low = res:lower()
          if low:find("失败", 1, true) or low:find("error", 1, true) then success = false end
        end
        return success, text, res
      end
    end
  end

  if game and is_fn(game.isCmdSuccess) then
    local ok, hit = pcall(game.isCmdSuccess, cmd)
    if ok and hit == true then
      return true, "isCmdSuccess=true（服务器确认成功）", true
    end
  end

  if game and is_fn(game.sendCommand) then
    local ok, res = pcall(game.sendCommand, cmd)
    if ok then
      if type(res) == "string" and res:find("方法未实现", 1, true) then
        -- fall through
      else
        return false, "已提交但未确认执行结果(sendCommand)：" .. format_cmd_resp(res), res
      end
    end
  end

  if game and is_fn(game.sendCommandFast) then
    local ok = pcall(game.sendCommandFast, cmd)
    if ok then
      return false, "已Fast提交但没有服务器回执：" .. cmd, nil
    end
  end

  return false, "全部指令通道失败（sendCommand/WithResp/Fast 均不可用）", nil
end

function M.apply()
  if is_fn(r17_read_prompt) then
    local _rp = r17_read_prompt
    r17_read_prompt = function(kind)
      local ok, a, b, c = pcall(_rp, kind)
      if ok and type(a) == "string" and a ~= "" then return a, b, c end
      local fn = (type(R17_PROMPT_FILES) == "table" and R17_PROMPT_FILES[kind]) or (tostring(kind) .. ".txt")
      local text = nil
      pcall(function()
        local p = tostring(game.dataDir() or "") .. "/ai_prompts/" .. fn
        local o, v = pcall(util.read_file, p)
        if o and type(v) == "string" and v ~= "" then text = v end
      end)
      if not text then
        pcall(function()
          local v = plugin.readResource("ai_prompts/" .. fn)
          if type(v) == "string" and v ~= "" then text = v end
        end)
      end
      if not text and is_fn(r17_prompt_default) then text = tostring(r17_prompt_default(kind) or "") end
      return text or "", "tool_fix_fallback", true
    end
  end

  if is_fn(task_prompt_append) then
    local _tpa = task_prompt_append
    task_prompt_append = function(messages, id)
      local ok, ret = pcall(_tpa, messages, id)
      if ok and ret then return true end
      id = tostring(id or "")
      local txt = ""
      if is_fn(r17_read_prompt) then
        local o, t = pcall(r17_read_prompt, id)
        if o and type(t) == "string" then txt = t end
      end
      if txt == "" then return false end
      local add = "\n\n【当前任务专项提示词：" .. id .. "】\n" .. txt
      if messages[1] and messages[1].role == "system" then
        messages[1].content = tostring(messages[1].content or "") .. add
      else
        table.insert(messages, 1, { role = "system", content = add })
      end
      return true
    end
  end

  if is_fn(process_tools) then
    local _pt = process_tools
    process_tools = function(text, playerName, admin, cmd_limit, active_subprompts, task_mode)
      local t = tostring(text or "")
      t = t:gsub("%[LOAD_SUB_PROMPT%]%s*cmd%s*%[/LOAD_SUB_PROMPT%]", "[LOAD_SUB_PROMPT]command[/LOAD_SUB_PROMPT]")
      local tags = {}
      for tag in t:gmatch("%[([%u%d_]+)%]") do tags[#tags + 1] = tag end
      if #tags > 0 then log("模型标签: " .. table.concat(tags, ",")) end
      local ok, results, used, loads, prompt_only = pcall(_pt, t, playerName, admin, cmd_limit, active_subprompts, task_mode)
      if not ok then
        return { "工具执行器异常：" .. tostring(results) }, true, nil, false
      end
      if type(results) == "table" and loads and #loads > 0 then
        results[#results + 1] = "专项提示词加载请求已受理：" .. table.concat(loads, ",")
      end
      return results, used, loads, prompt_only
    end
  end

  if is_fn(run_command) then
    local _run = run_command
    run_command = function(cmd, playerName, admin)
      local c = tostring(cmd or "")
      local success, detail = exec_with_result(c)
      log("run_command " .. c .. " => " .. tostring(success) .. " " .. tostring(detail))
      if success then
        return "✓ 指令执行结果：[" .. c:gsub("^/+", "") .. "] → " .. tostring(detail)
      end
      -- 命令可能已经提交；不允许再执行旧通道导致同一命令重复发送。
      if tostring(detail):find("已提交",1,true) or tostring(detail):find("已Fast提交",1,true) then
        return "△ 指令已发送，服务器尚未返回可验证结果：[" .. c:gsub("^/+", "") .. "] " .. tostring(detail)
      end
      local ok, out = pcall(_run, cmd, playerName, admin)
      if ok and type(out) == "string" then
        local low = out:lower()
        if low:find("失败", 1, true) or low:find("方法未实现", 1, true) then
          return "✗ 指令失败：[" .. c:gsub("^/+", "") .. "] → " .. tostring(detail) .. " | 原路径：" .. out
        end
        return out .. " | 通道回执：" .. tostring(detail)
      end
      return "✗ 指令失败：[" .. c:gsub("^/+", "") .. "] → " .. tostring(detail)
    end
  end

  if is_fn(ai_run_checked_command) then
    local _arc = ai_run_checked_command
    ai_run_checked_command = function(cmd, playerName, admin)
      if admin then
        local c = tostring(cmd or "")
        local success, detail = exec_with_result(c)
        if success then
          return "✓ 指令执行结果：[" .. c:gsub("^/+", "") .. "] → " .. tostring(detail)
        end
        if tostring(detail):find("已提交",1,true) or tostring(detail):find("已Fast提交",1,true) then return "△ 指令已提交但未确认：" .. tostring(detail) end
        local ok, out = pcall(_arc, cmd, playerName, admin)
        if ok then return tostring(out) .. " | 通道：" .. tostring(detail) end
        return "✗ 指令失败：" .. tostring(detail)
      end
      return _arc(cmd, playerName, admin)
    end
  end

  if type(plugin_api) == "table" and is_fn(plugin_api.reloadPrompts) then
    pcall(plugin_api.reloadPrompts)
  end

  if is_fn(on_chat) then
    local _chat = on_chat
    on_chat = function(playerName, message)
      local raw = tostring(message or ""):gsub("^%s+", ""):gsub("%s+$", "")
      local low = raw:lower()
      local cmdw = tostring(COMMAND_WORD or "指令 ")
      local is_cmd = raw:sub(1, #cmdw) == cmdw
      local is_ai = (low:sub(1, 3) == "ai " or raw:sub(1, 3) == "AI ") and low:sub(1, 9) ~= "ai agent"
      local is_public = (raw:sub(1, 3) == "AI ")

      if is_cmd or is_ai then
        local question = is_cmd and raw:sub(#cmdw + 1):match("^%s*(.-)%s*$") or raw:sub(4):match("^%s*(.-)%s*$")
        question = tostring(question or "")
        local ql = question:lower()
        if question == "" then
          tell(playerName, "§e用法：指令 <内容>  或  ai <内容>")
          return true
        end
        if is_cmd and (ql:find("在线", 1, true) or ql:find("谁在", 1, true)
          or ql:find("玩家列表", 1, true) or ql:find("list", 1, true) or ql:find("有谁", 1, true)) then
          tell(playerName, "§b[Ds] §f" .. local_list())
          return true
        end
        if is_fn(run_agent) then
          if is_fn(ai_think_begin) then pcall(ai_think_begin,playerName,question,is_cmd,is_public) end
          local q2 = question
          if is_cmd then
            q2 = question .. "\n\n【系统】必须输出工具标签如 [LIST][/LIST] 或 [CMD]...[/CMD]；以工具回执为准，禁止空口说完成。"
          end
          local window=is_cmd and "command" or "chat"
          local ok, reply = pcall(run_agent, q2, playerName, is_cmd, window)
          if is_fn(ai_think_end) then pcall(ai_think_end,playerName) end
          local spent=tonumber(type(AI_USAGE)=="table" and AI_USAGE.task) or 0
          local total=0
          if spent>0 and is_fn(ai_usage_window_add) then
            local valid,n=pcall(ai_usage_window_add,playerName,is_cmd,window,spent)
            if valid then total=tonumber(n) or 0 end
          elseif is_fn(ai_usage_window_get) then
            local valid,n=pcall(ai_usage_window_get,playerName,is_cmd,window)
            if valid then total=tonumber(n) or 0 end
          end
          -- This wrapper owns ordinary AI; bypass unconfirmed quiet/fast command shims.
          local delivery=require("modules.yy_ai_delivery")
          if not ok then
            delivery.send(playerName,"§c[Ds] 处理失败："..tostring(reply):sub(1,160),false)
          elseif reply and tostring(reply)~="" then
            delivery.send(playerName,"§b[Ds] §f"..tostring(reply),is_public)
          else
            delivery.send(playerName,"§c[Ds] 没有得到有效结果。",false)
          end
          delivery.send(playerName,string.format("§8[Token] §f本次 §b%d §7| §f本窗口累计 §b%d",spent,total),is_public)
          return true
        end
      end
      local ok, ret = pcall(_chat, playerName, message)
      if not ok then
        tell(playerName, "§c处理异常：" .. tostring(ret):sub(1, 100))
        return true
      end
      return ret
    end
  end

  log("tool_fix.apply 完成(含指令回执)")
  return true
end

return M