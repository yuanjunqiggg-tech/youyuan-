local M={}; local timers={}; local owned={}; local promptCache={}; local signatures={}; local registryState={}; local pending={}
-- The native chain directory IS this plugin's command_txt directory. Reuse
-- the filename instead of making prefixed copies that scans would reimport.
local function nativeName(n) return tostring(n) end
local function isFunctionTimer(name)
    return name:match("^function_") or name:match("^r20[oxy]_.*[Tt]ick") or name=="r21a_function_tick"
end
function M.beforeCore()
    local schedule=util.schedule
    util.schedule=function(sec,handler)
        local name=tostring(handler)
        if isFunctionTimer(name) then return 0 end
        -- Multiple historical wrappers may register the same timer again.
        if timers[name] then return timers[name] end
        local h=schedule(sec,handler); timers[name]=h; return h
    end
end
function M.sync(id)
    local item,reg=function_registry_get(id)
    if not item then return false,"文件编号不存在" end
    local name=nativeName(item.name); owned[name]=true
    local text,err=function_read_text(item.name)
    if not text then return false,err end
    local cfg={mode="fast",channel="ai",interval_ms=math.max(1,math.floor((tonumber(item.delay_sec) or .05)*1000)),
        scheduling="segments",bootstrap=true,enabled=item.enabled==true and item.loaded~=false and not item.missing,run_count=0,rate_cps=0}
    local key=tostring(id); local prev=signatures[key]
    local configSignature=tostring(cfg.enabled)..":"..cfg.interval_ms
    if prev and prev.text==text and prev.config==configSignature then return true,"常驻IR已加载，无需重新编译" end
    local ok,out
    if prev and prev.config==configSignature then
        -- The original editor already wrote this SAME data file. Reload its
        -- IR only; unchanged sibling segments retain their runtime state.
        ok,out=pcall(chains.reload,name)
    else
        ok,out=pcall(chains.save,name,text,cfg)
    end
    item.timer_registered=ok and cfg.enabled; item.timer_id=nil; item.registered_delay=cfg.interval_ms/1000
    item.needs_reload=not ok; item.last_error=ok and nil or tostring(out)
    function_registry_save(reg)
    if not ok then return false,tostring(out) end
    if ok then signatures[key]={text=text,config=configSignature} end
    return true,"原生FUNCTION：独立段、常驻IR、无逐轮文件读取"
end
function M.syncAll()
    local reg=function_registry_load(); local count=0
    for _,x in ipairs(function_registry_sorted(reg,false)) do
        local ok=M.sync(x.id); if ok then count=count+1 end
    end
    return count
end
function M.status()
    local n=0;for _ in pairs(owned) do n=n+1 end
    return {runtime="persistent",engine="native-go",managed_chains=n,prompt_cache=true,
        source_reexecuted_per_event=false,txt_parsed_per_round=false}
end
function M.afterCore()
    FUNCTION_ENGINE_VERSION="YOUYUAN_NATIVE_SEGMENTS_V1"
    FUNCTION_BUNDLED_TXT={ ["药水系统.txt"]="药水系统.txt",["雪球菜单_综合功能.txt"]="雪球菜单_综合功能.txt" }
    function plugin_code_dir() return plugin.codeDir() end
    function_register_timers=M.syncAll
    function_schedule_one=M.sync
    function_runtime_tick=function() return true end -- no duplicate Lua loop
    r21a_prepare_function_registry=function()
        for name in pairs(FUNCTION_BUNDLED_TXT) do
            local path=FUNCTION_DIR.."/"..name
            local ok,text=pcall(util.read_file,path)
            if not ok or type(text)~="string" or text=="" then plugin.extractResource("command_txt/"..name,"command_txt/"..name) end
        end
        function_scan_files(true)
        return M.syncAll()
    end
    -- Keep the complete original editor/parser, but sync successful edits.
    local write=function_write_text
    function_write_text=function(name,text)
        local ok,result=write(name,text)
        if ok then
            local reg=function_registry_load()
            for id,it in pairs(reg.items or {}) do if it.name==name then M.sync(id); break end end
        end
        return ok,result
    end
    local oldSave=function_registry_save
    -- Menus that disable/unload without scheduling must stop the native worker.
    function_registry_save=function(reg)
        for _,it in pairs(reg.items or {}) do
            local name=nativeName(it.name)
            local active=it.enabled==true and it.loaded~=false and not it.missing
            if registryState[name]~=nil and registryState[name]~=active and owned[name] then
                if active then pcall(chains.start,name) else pcall(chains.stop,name) end
            end
            -- Saving menu metadata must not stop a chain the native UI
            -- started independently. Only an explicit state change acts.
            registryState[name]=active
        end
        return oldSave(reg)
    end
    r17_read_prompt=function(kind)
        if promptCache[kind] then return promptCache[kind].text,promptCache[kind].path,false end
        local fn=R17_PROMPT_FILES[kind]
        if not fn then return r17_prompt_default(kind),"builtin",true end
        local path=game.dataDir().."/ai_prompts/"..fn
        local ok,text=pcall(util.read_file,path)
        if not ok or not r17_prompt_valid(kind,text) then
            text=plugin.readResource("ai_prompts/"..fn); path="resource:ai_prompts/"..fn
        end
        if not r17_prompt_valid(kind,text) then error("提示词格式校验失败："..tostring(kind)) end
        promptCache[kind]={text=text,path=path}; return text,path,false
    end
    plugin_api=plugin_api or {}
    plugin_api.reloadPrompts=function() promptCache={};return true end
    -- Read the current native service, never the received hard-coded cloud
    -- inbox/key. Natural-language work is sent to an actual local workbench.
    local function api(path,method,body,query)
        local result=mcp.call("api_call",{path=path,method=method or "GET",body=body,query=query})
        local text=result.content and result.content[1] and result.content[1].text
        local data=text and http.json_decode(text) or {}
        if result.isError or data.ok==false then error(tostring(data.error or text or "MCP工具失败")) end
        return data
    end
    ai_agent_bridge_ready=function() return mcp.connection().configured end
    ai_agent_bridge_key=function() return "host-managed" end
    ai_agent_set_key=function(name,key)
        send(name,"§b内置MCP自动读取本实例，密钥由宿主管理。外部MCP与AI模型请在工作区配置；无需旧平台密钥。")
        return true
    end
    ai_agent_bridge_status=function(name)
        local c=mcp.connection()
        send(name,"§b本机真实MCP\n§7账号："..c.account.."\n§7地址："..c.local_mcp_url.."\n§7鉴权：宿主内部管理\n§7公网："..(c.relay_connected and "已连接" or "未启用"))
        return true
    end
    ai_agent_roster=function()
        local ok,data=pcall(api,"/api/youyuan/agent/workspaces")
        if not ok then return false,tostring(data) end
        local rows={}
        for i,w in ipairs(data.workspaces or {}) do rows[i]={no=i,id=w.id,label=w.name,kind="model",online=w.model~="" and w.model~=nil} end
        return true,rows
    end
    ai_agent_forward=function(name,text,number)
        if not is_admin(name) then return false,"仅管理员可提交控制任务" end
        local ok,rows=ai_agent_roster();if not ok then return false,rows end
        local target=rows[tonumber(number) or 0]
        if not target then for _,w in ipairs(rows) do if w.online then target=w;break end end end
        if not target then return false,"请先在本实例AI工作区选择并配置模型，再提交任务" end
        if pending[target.id] then return false,"该工作区正在处理上一条游戏任务，请等待完成或选择另一个工作区" end
        local before=api("/api/youyuan/agent/workspaces","GET",nil,{id=target.id,compact="1"}).workspace
        if before.state=="运行中" then return false,"工作区正在处理其他任务，请选择空闲工作区" end
        local success,result=pcall(api,"/api/youyuan/agent/task","POST",{id=target.id,action="start",message="游戏管理员 "..tostring(name).." 的任务："..tostring(text)})
        if not success then return false,tostring(result) end
        pending[target.id]={player=name,turns=#(before.turns or {}),input=(before.usage or {}).input_tokens or 0,output=(before.usage or {}).output_tokens or 0}
        return true,{workspace=target.id,accepted=true}
    end
    yy_mcp_task_poll=function()
        -- No model or network HTTP round-trip here: query the actual local
        -- host only while a task is pending. Result is returned to its sender.
        for id,job in pairs(pending) do
            local ok,data=pcall(api,"/api/youyuan/agent/workspaces","GET",nil,{id=id,compact="1"})
            if ok and data.workspace and data.workspace.state~="运行中" then
                local w=api("/api/youyuan/agent/workspaces","GET",nil,{id=id}).workspace;local parts={}
                for i=job.turns+1,#(w.turns or {}) do
                    local t=w.turns[i]
                    if t.role=="assistant" and t.content and t.content~="" then parts[#parts+1]=t.content end
                end
                local answer=table.concat(parts,"\n")
                if answer=="" then answer=(type(w.error)=="string" and w.error~="") and w.error or ("任务状态："..tostring(w.state)) end
                send(job.player,AI_TAG_BLUE.." "..answer)
                local usage=w.usage or {}
                local tokens=math.max(0,(usage.input_tokens or 0)-job.input)+math.max(0,(usage.output_tokens or 0)-job.output)
                send(job.player,"§8本次 Token："..tostring(tokens).."（工作区API用量）")
                pending[id]=nil
            end
        end
    end
    util.schedule(1,"yy_mcp_task_poll")
    yy_mcp_dispatch=function(name,payload)
        if not is_admin(name) then return false,"仅管理员可调用控制工具" end
        local ok,args=pcall(http.json_decode,payload)
        if ok and type(args)=="table" and args.tool then
            local result=mcp.call(tostring(args.tool),args.arguments or {})
            if result.isError then return false,http.json_encode(result) end
            return true,http.json_encode(result)
        end
        local accepted,result=ai_agent_forward(name,payload,nil)
        return accepted,accepted and "已提交本机真实工作区，完成后会返回结果" or result
    end
    plugin_api.mcpConnection=mcp.connection
    plugin_api.mcpTools=mcp.tools
    plugin_api.mcpCall=mcp.call
    local early={}
    local begin=ai_think_begin
    ai_think_begin=function(name,q,cmd,pub)
        if not early[name] then return begin(name,q,cmd,pub) end
        -- Slow prediction/disk work now happens only after the visible notice.
        local sec,kind=ai_predict_wait(q,cmd);local d=ai_think_db()
        d[name]={at=periodic_beat_now(),expect=sec,kind=kind,public=pub==true,question=tostring(q):sub(1,120)}
        ai_think_save(d);return sec,kind
    end
    local chat=on_chat
    on_chat=function(name,message)
        local raw=tostring(message or ""):gsub("^%s+","");local low=raw:lower()
        local q=low:match("^ai%s+(.+)") or raw:match("^指令%s+(.+)")
        if q and not low:match("^ai%s+agent") and not q:find("清除记忆",1,true) and not q:find("清空记忆",1,true) then
            local selector=raw:sub(1,3)=="AI " and "@a" or ('"'..tostring(name):gsub('"','\\"')..'"')
            game.sendCommandFast("tellraw "..selector.." "..http.json_encode({rawtext={{text=AI_TAG_BLUE.." "..AI_THINK_TEXT.."§7……"}}}))
            early[name]=true
        end
        local ok,result=pcall(chat,name,message);early[name]=nil
        if not ok then error(result) end
        return result
    end
    local previous={};local noticeReady=false
    yy_notice_diff=function(lines)
        if not noticeReady then
            game.sendCommand('scoreboard objectives add ds_notice dummy "服务器状态"')
            game.sendCommand("scoreboard objectives setdisplay sidebar ds_notice")
            noticeReady=true
            local ok,saved=pcall(util.json_load,R20W_NOTICE_FILE)
            if ok and type(saved)=="table" then previous=saved.lines or {} end
        end
        local current={};for i,line in ipairs(lines) do current[line]=#lines-i+1 end
        for _,old in ipairs(previous) do if current[old]==nil then
            game.sendCommand('scoreboard players reset "'..tostring(old):gsub('"','\\"')..'" ds_notice')
        end end
        local last={};for i,line in ipairs(previous) do last[line]=#previous-i+1 end
        for _,line in ipairs(lines) do if last[line]~=current[line] then
            game.sendCommand('scoreboard players set "'..tostring(line):gsub('"','\\"')..'" ds_notice '..current[line])
        end end
        previous=lines
    end
    local stop=on_stop
    on_stop=function()
        for name in pairs(owned) do pcall(chains.stop,name) end
        if type(stop)=="function" then stop() end
    end
end
return M
