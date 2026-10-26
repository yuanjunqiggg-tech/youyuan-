local M={}; local timers={}; local owned={}; local promptCache={}; local signatures={}; local registryState={}; local pending={}; local eventSeen={}
-- The native chain directory IS this plugin's command_txt directory. Reuse
-- the filename instead of making prefixed copies that scans would reimport.
local function nativeName(n) return tostring(n) end
local function isFunctionTimer(name)
    return name:match("^function_") or name:match("^r20[oxy]_.*[Tt]ick") or name=="r21a_function_tick"
end
function M.beforeCore()
    -- Enable responses before the historical on_init confirmation calls.
    game.sendCommandFast("gamerule sendcommandfeedback true")
    local schedule=util.schedule
    util.schedule=function(sec,handler)
        local name=tostring(handler)
        if isFunctionTimer(name) then return 0 end
        -- Multiple historical wrappers may register the same timer again.
        if timers[name] then return timers[name] end
        local h=schedule(sec,handler); timers[name]=h; return h
    end
    -- 常驻VM下，历史补丁可能重复注册同一个handler。只去重完全相同的注册。
    if type(events)=="table" then
        local sub=events.subscribe
        if type(sub)=="function" then
            -- R68: one native callback per raw packet type, with all existing
            -- Lua handlers called inside that callback. This prevents N
            -- identical callbacks competing for the 128-slot host queue.
            local fanout={UpdateAttributes={},Text={}}
            local registered={}
            local call_counts={UpdateAttributes=0,Text=0}
            local errors=0
            local last_error=""
            local function dispatch(kind,...)
                call_counts[kind]=call_counts[kind]+1
                -- Skip repeated system mode-change UI notices; do not skip deaths.
                if kind=="Text" then
                    local ev=select(1,...)
                    local fields=type(ev)=="table" and (ev.fields or ev) or nil
                    local message=fields and (fields.message or fields.text or fields.rawtext)
                    if type(message)=="string" and message:find("gameMode.changed",1,true) then return true end
                end
                for _,handler in ipairs(fanout[kind]) do
                    local fn=_G[handler]
                    if type(fn)=="function" then
                        local ok,err=pcall(fn,...)
                        if not ok then
                            errors=errors+1;last_error=tostring(err):sub(1,220)
                        end
                    end
                end
                return true
            end
            function yy_native_attributes_fanout(...) return dispatch("UpdateAttributes",...) end
            function yy_native_text_fanout(...) return dispatch("Text",...) end
            events.subscribe=function(kind,handler,...)
                local key="subscribe|"..tostring(kind).."|"..tostring(handler)
                if eventSeen[key] then return true end
                -- Legacy death routes already superseded by strict Text/native death.
                local legacy_key=tostring(kind).."|"..tostring(handler)
                if legacy_key=="PlayerLocation|final_death_location_handler"
                   or legacy_key=="PyRpc|final_death_pyrpc_handler"
                   or legacy_key=="UpdateAttributes|r19_death_health_handler" then
                    eventSeen[key]=true
                    return true
                end
                if fanout[kind] then
                    local list=fanout[kind]
                    if not registered[kind] then
                        local callback=kind=="Text" and "yy_native_text_fanout" or "yy_native_attributes_fanout"
                        local a,b,c=sub(kind,callback)
                        if a==false then return a,b,c end
                        registered[kind]=true
                    end
                    -- Prefer the existing AI chat fallback on a Text packet;
                    -- keep all other handlers (death, anti-cheat) in the chain.
                    if kind=="Text" and handler=="yy_r24_text_chat_fallback" then
                        table.insert(list,1,handler)
                    else
                        list[#list+1]=handler
                    end
                    eventSeen[key]=true
                    return true
                end
                local a,b,c=sub(kind,handler,...)
                if a~=false then eventSeen[key]=true end
                return a,b,c
            end
            M.fanoutStatus=function()
                return {registered=registered,
                    text_handlers=#fanout.Text,attribute_handlers=#fanout.UpdateAttributes,
                    callbacks=call_counts,handler_errors=errors,last_error=last_error}
            end
        end
        -- The host snapshots named callbacks at registration. Route the
        -- single original onChat subscription to the latest patched handler.
        function yy_native_chat_dispatch(name,message)
            if type(on_chat)=="function" then return on_chat(name,message) end
            return false
        end
        for _,method in ipairs({"onChat","onJoin","onLeave","onBotReady","onPlayerDeath","onDeath","onRespawn","onMove"}) do
            local original=events[method]
            if type(original)=="function" then
                events[method]=function(handler,...)
                    local key=method.."|"..tostring(handler)
                    if eventSeen[key] then return true end
                    local actual=(method=="onChat" and handler=="on_chat") and "yy_native_chat_dispatch" or handler
                    local a,b,c=original(actual,...)
                    eventSeen[key]=true
                    return a,b,c
                end
            end
        end
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
    cfg.hz=1/(tonumber(item.delay_sec) or .05)
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
    -- R67: Some older native TXT configuration files are writable by the
    -- toolbox API but not by the Lua VM user. Reload already-installed IR
    -- instead of repeatedly failing a config-file save on reconnect.
    -- Only the unchanged, known snowball chain is eligible.
    if (not ok or out==false) and name=="雪球菜单_综合功能.txt"
       and (tostring(out):find("permission denied",1,true) or tostring(out):find("该链属于其他账号",1,true) or tostring(out):find("belongs to another account",1,true))
       and type(chains.reload)=="function" then
        local good,reloaded=pcall(chains.reload,name)
        if good and reloaded~=false
           and (type(reloaded)~="table" or reloaded.ok~=false) then
            ok=true;out="existing native TXT reloaded (config write permission denied)"
        else
            out=tostring(out).."; reload failed: "..tostring(reloaded)
        end
    end
    item.timer_registered=ok and cfg.enabled; item.timer_id=nil; item.registered_delay=cfg.interval_ms/1000
    item.needs_reload=not ok; item.last_error=ok and nil or tostring(out)
    function_registry_save(reg)
    if not ok then return false,tostring(out) end
    if ok then signatures[key]={text=text,config=configSignature} end
    return true,"原生FUNCTION：独立段、常驻IR、无逐轮文件读取"
end
function M.syncAll()
    local ok,snapshot=pcall(chains.list)
    if not ok or type(snapshot)~="table" or type(snapshot.files)~="table" then return 0 end
    local known={}
    for _,row in ipairs(snapshot.files) do
        if type(row.config)=="table" then known[row.config.name]=true end
    end
    local reg=function_registry_load(); local count=0
    for _,x in ipairs(function_registry_sorted(reg,false)) do
        local name=nativeName(x.item.name)
        if known[name] then
            -- The native config is authoritative on startup, including a saved stop.
            owned[name]=true; count=count+1
        else
            local synced=M.sync(x.id); if synced then count=count+1 end
        end
    end
    return count
end
function M.status()
    local n=0;for _ in pairs(owned) do n=n+1 end
    return {runtime="persistent",engine="native-go",managed_chains=n,prompt_cache=true,
        native_fanout=type(M.fanoutStatus)=="function" and M.fanoutStatus() or nil,
        source_reexecuted_per_event=false,txt_parsed_per_round=false,menu_parser="native-go",menu_toggle="single-native-call"}
end
function M.afterCore()
    local loadRegistry=function_registry_load
    function_registry_load=function()
        local reg=loadRegistry()
        local ok,snapshot=pcall(chains.list)
        if ok and type(snapshot)=="table" and type(snapshot.files)=="table" then
            local rows={}
            for _,row in ipairs(snapshot.files) do
                if type(row.config)=="table" then rows[row.config.name]=row end
            end
            for _,it in pairs(reg.items or {}) do
                local row=rows[it.name]
                if row then
                    it.enabled=row.config.enabled==true
                    it.timer_registered=it.enabled
                    it.native_state=row.state
                    it.last_error=row.error~="" and row.error or nil
                    if tonumber(row.config.hz) and row.config.hz>0 then it.delay_sec=1/row.config.hz end
                    owned[it.name]=true
                    registryState[it.name]=it.enabled and it.loaded~=false and not it.missing
                end
            end
        end
        return reg
    end
    FUNCTION_ENGINE_VERSION="YOUYUAN_NATIVE_SEGMENTS_V1"
    FUNCTION_BUNDLED_TXT={ ["药水系统.txt"]="药水系统.txt",["雪球菜单_综合功能.txt"]="雪球菜单_综合功能.txt" }
    function plugin_code_dir() return plugin.codeDir() end
    function_register_timers=M.syncAll
    function_schedule_one=M.sync
    function function_set_delay(player,id,delay)
        if not is_admin(player) then send(player,"§c仅管理员。") return true end
        delay=tonumber(delay)
        if not delay or delay~=delay or delay<FUNCTION_MIN_DELAY or delay>FUNCTION_MAX_DELAY then
            send(player,"§c延迟范围：0.001 ～ 100000 秒。") return true
        end
        local item,reg=function_registry_get(id)
        if not item then send(player,"§c文件编号不存在。") return true end
        local ok,out=pcall(chains.configure,nativeName(item.name),{hz=1/delay,interval_ms=math.max(1,math.floor(delay*1000+.5))})
        if not ok or out==false or (type(out)=="table" and out.ok==false) then
            send(player,"§c频率未保存："..tostring(type(out)=="table" and out.error or out)) return true
        end
        item.delay_sec=delay;item.needs_reload=false;signatures[tostring(id)]=nil
        function_registry_save(reg)
        send(player,string.format("§a模块频率上限已保存：§b%.8g Hz§7；较慢模块保持原频率。",1/delay))
        return true
    end
    function_runtime_tick=function() return true end -- no duplicate Lua loop

    -- The menu must validate exactly the IR that the native runner executes.
    local legacyParse=function_parse_document
    function_parse_document=function(text,opts)
        if opts and opts.initial_state then return legacyParse(text,opts) end
        local ok,out=pcall(chains.validate,tostring(text or ""))
        local doc={nodes={},errors={},warnings={},ended=true,commands=0,chains=0}
        if not ok or type(out)~="table" or out.ok==false or type(out.plan)~="table" then
            doc.errors[1]=tostring(type(out)=="table" and out.error or out)
            return doc
        end
        for chain_id,chain in ipairs(out.plan.chains or {}) do
            doc.chains=chain_id
            for _,node in ipairs(chain.nodes or {}) do
                doc.nodes[#doc.nodes+1]={index=#doc.nodes+1,chain_id=chain_id,
                    state={kind=node.type,conditional=node.conditional and 1 or 0,
                        redstone=node.redstone and 1 or 0,hz=chain.hz},
                    command=node.control and node.command or "/"..node.command,
                    control=node.control,physical_line=node.line}
            end
        end
        doc.commands=#doc.nodes
        return doc
    end
    local serialize=function_serialize_nodes
    function_serialize_nodes=function(nodes)
        return (serialize(nodes):gsub("^/@","@"):gsub("\n/@","\n@"))
    end
    function_toggle=function(player,id)
        if not is_admin(player) then send(player,"§c仅管理员可管理命令链。") return true end
        local item,reg=function_registry_get(id)
        if not item then send(player,"§c文件编号不存在。") return true end
        local enabled=item.enabled~=true
        local ok,out
        if enabled then
            local text,err=function_read_text(item.name)
            if not text then send(player,"§c读取失败："..tostring(err)) return true end
            local doc=function_parse_document(text)
            if #doc.errors>0 then send(player,"§c无法开启："..table.concat(doc.errors,"；")) return true end
            -- Reload retains the existing cadence/channel; a toggle never saves a second runnable copy.
            ok,out=pcall(chains.reload,nativeName(item.name))
            if not ok and tostring(out):find("命令链不存在",1,true) then
                ok,out=pcall(M.sync,id)
            end
            if not ok or out==false or (type(out)=="table" and out.ok==false) then
                send(player,"§c未开启："..tostring(type(out)=="table" and out.error or out)) return true
            end
        end
        ok,out=pcall(enabled and chains.start or chains.stop,nativeName(item.name))
        if not ok or out==false or (type(out)=="table" and out.ok==false) then
            send(player,"§c操作失败："..tostring(type(out)=="table" and out.error or out)) return true
        end
        item.enabled=enabled;item.timer_registered=enabled;item.loaded=true
        item.needs_reload=false;item.last_error=nil
        registryState[nativeName(item.name)]=enabled
        signatures[tostring(id)]=nil
        function_registry_save(reg)
        send(player,"§a"..tostring(item.name)..(enabled and " 已开启。" or " 已停止。"))
        return true
    end
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
    -- ★ 修复：专项 txt 加载失败时不得 error 整次 AI 静默无响应
    r17_read_prompt=function(kind)
        kind=tostring(kind or "")
        if promptCache[kind] then return promptCache[kind].text,promptCache[kind].path,false end
        local fn=(type(R17_PROMPT_FILES)=="table" and R17_PROMPT_FILES[kind]) or nil
        if not fn then
            local body=type(r17_prompt_default)=="function" and r17_prompt_default(kind) or ""
            return body,"builtin",true
        end
        local candidates={}
        pcall(function()
            local dd=tostring(game.dataDir() or "")
            if dd~="" then candidates[#candidates+1]=dd.."/ai_prompts/"..fn end
        end)
        pcall(function()
            local cd=tostring(plugin.codeDir and plugin.codeDir() or "")
            if cd~="" then candidates[#candidates+1]=cd.."/ai_prompts/"..fn end
        end)
        local text,path,from_resource=nil,nil,false
        for _,p in ipairs(candidates) do
            local ok,v=pcall(util.read_file,p)
            if ok and type(v)=="string" and v~="" then
                if type(r17_prompt_valid)~="function" or r17_prompt_valid(kind,v) then
                    text,path=v,p; break
                end
            end
        end
        if not text then
            local ok,v=pcall(function() return plugin.readResource("ai_prompts/"..fn) end)
            if ok and type(v)=="string" and v~="" then
                text,path,from_resource=v,"resource:ai_prompts/"..fn,true
            end
        end
        if text and type(r17_prompt_valid)=="function" and not r17_prompt_valid(kind,text) then
            -- 校验失败：仍返回原文，避免整次任务被 error 吞掉（AI 表现为「专项无响应」）
            path=(path or "invalid")..":schema_warn"
        end
        if not text or text=="" then
            local body=type(r17_prompt_default)=="function" and r17_prompt_default(kind) or ""
            return body,"builtin",true
        end
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
    local chat=on_chat
    on_chat=function(name,message)
        local ok,result=pcall(chat,name,message)
        if not ok then
            -- 不再向上 error，避免专项加载异常导致整次无回执
            pcall(function()
                local err=tostring(result or "unknown")
                send(name,"§c[AI] 处理异常："..err:sub(1,150))
            end)
            return true
        end
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
    -- TXT lifecycle is independent of player menu sessions.
    local txok,txmod=pcall(require,"modules.yy_txt_autostart_r64")
    if txok and type(txmod)=="table" and type(txmod.apply)=="function" then
        pcall(txmod.apply)
    end
    local stop=on_stop
    on_stop=function()
        for name in pairs(owned) do
            -- Native-owned R83+ snowball gate is an independently installed
            -- account chain. Stopping the Lua plugin must not overwrite the
            -- persisted enable flag of this native chain.
            if name~="雪球菜单_综合功能.txt" and name~="_违禁收纳袋独立清除20Hz.txt" then
                pcall(chains.stop,name)
            end
        end
        if type(stop)=="function" then stop() end
    end
end
return M