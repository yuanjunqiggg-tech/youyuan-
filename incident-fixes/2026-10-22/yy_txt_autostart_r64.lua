-- R64 | 重连仅恢复已启用的 TXT；用户停止的链保持停止。
-- 只使用宿主原生 chains 引擎；不在 Lua 按 20Hz 派发游戏命令。
local M={};local installed=false
local st={version="R64-autostart",connected=false,active=false,generation=0,
          starts=0,stops=0,errors=0,last="",last_at=0,registered={},failures={}}
local function now()return tonumber(util.timestamp()) or os.time() end
local function persist()
 st.last_at=now()
 pcall(util.json_write,game.dataDir().."/yy_txt_autostart_r64_status.json",st)
end
local function check_connection()
 if type(bot)=="table" and type(bot.status)=="function" then
  local ok,v=pcall(bot.status)
  if ok and type(v)=="table" then
   if type(v.connected)=="boolean" then return v.connected end
   if type(v.online)=="boolean" then return v.online end
   if type(v.is_connected)=="boolean" then return v.is_connected end
   if type(v.status)=="table" and type(v.status.connected)=="boolean" then return v.status.connected end
   if v.state=="connected" or v.state=="online" or v.state=="ready" then return true end
   if v.state=="disconnected" or v.state=="offline" then return false end
  end
 end
 if type(bot)=="table" and type(bot.self)=="function" then
  local ok,v=pcall(bot.self)
  if ok and type(v)=="table" and type(v.name)=="string" and v.name~="" then return true end
 end
 return nil
end
local function filenames(force)
 local names={};local ids={};local native={}
 if type(chains)~="table" or type(chains.list)~="function" then return names,ids,"native_list_unavailable" end
 local good,snapshot=pcall(chains.list)
 if not good or type(snapshot)~="table" or type(snapshot.files)~="table" then return names,ids,"native_list_failed" end
 for _,row in ipairs(snapshot.files) do
  if type(row.config)=="table" and type(row.config.name)=="string" then native[row.config.name]=row.config.enabled==true end
 end
 if type(function_registry_load)~="function" or type(function_registry_sorted)~="function" then
  return names,ids,"registry_unavailable"
 end
 -- Scan on connection, not on every scheduler tick.
 if type(function_scan_files)=="function" then pcall(function_scan_files,true) end
 local ok,reg=pcall(function_registry_load)
 if not ok or type(reg)~="table" then return names,ids,"registry_failed" end
 local list=function_registry_sorted(reg,false)
 if type(list)~="table" then return names,ids,"registry_unsorted" end
 local changed=false
 for _,entry in ipairs(list) do
  local it=entry.item
  if type(it)=="table" and type(it.name)=="string"
    and it.name:lower():sub(-4)==".txt" then
   local readable=false
   if type(function_read_text)=="function" then
    local good,raw=pcall(function_read_text,it.name)
    readable=good and type(raw)=="string" and #raw>0
   end
   if readable then
    if force or (it.enabled==true and native[it.name]~=false) then
     if force and it.enabled~=true then it.enabled=true;changed=true end
     names[#names+1]=it.name
     ids[#ids+1]={id=entry.id,known=native[it.name]~=nil}
    end
   else
    st.failures[it.name]="TXT_unreadable"
   end
  end
 end
 -- Only an explicit manual start changes disabled registry entries.
 if changed and type(function_registry_save)=="function" then
  local saved,err=pcall(function_registry_save,reg)
  if not saved then return {},{},"registry_save:"..tostring(err) end
 end
 return names,ids,nil
end
local function start_all(reason,force)
 if st.active and not force then return true end
 if type(chains)~="table" or type(chains.start)~="function" then
  st.errors=st.errors+1;st.last="native_chains_unavailable";persist();return false
 end
 local names,ids,err=filenames(force)
 if err then st.errors=st.errors+1;st.last=err;persist();return false end
 local okc=0
 for i,name in ipairs(names) do
  local ready=ids[i].known;local why="sync_not_available"
  if not ready and type(function_schedule_one)=="function" then
   local ok,ret,detail=pcall(function_schedule_one,ids[i].id)
   ready=ok and ret~=false
   why=tostring(detail or ret)
  elseif not ready and type(plugin_api)=="table" and type(plugin_api.syncFunctions)=="function" then
   local ok,ret,detail=pcall(plugin_api.syncFunctions,ids[i].id)
   ready=ok and ret~=false
   why=tostring(detail or ret)
  end
  if ready then
   -- chains.start is an idempotent native start operation, not an extra Lua loop.
   local ok,ret=pcall(chains.start,name)
   if ok and ret~=false then
    st.registered[name]="running";okc=okc+1;st.failures[name]=nil
   else
    st.registered[name]="start_failed"
    st.failures[name]=tostring(ret)
   end
  else
   st.registered[name]="sync_failed";st.failures[name]=why
  end
 end
 st.connected=true;st.generation=st.generation+1
 st.starts=st.starts+okc
 st.active=(okc>0)
 st.last="join:"..tostring(reason)..":started="..okc.."/"..#names
 if okc<#names then st.errors=st.errors+(#names-okc) end
 persist()
 return okc==#names
end
local function stop_all(reason,manual)
 if not st.active and not st.connected then return true end
 local count=0
 if manual and type(chains)=="table" and type(chains.stop)=="function" then
  for name,flag in pairs(st.registered) do
   if flag=="running" then
    local ok,res=pcall(chains.stop,name)
    if ok and res~=false then count=count+1;st.registered[name]="stopped" end
   end
  end
 end
 st.stops=st.stops+count
 st.connected=false;st.active=false;st.last="leave:"..tostring(reason)..":stopped="..count
 -- Native chains already wait for reconnection; automatic cleanup must not disable them.
 persist()
 return true
end
function yy_txt_autostart_r64_ready()
 return start_all("onBotReady")
end
function yy_txt_autostart_r64_tick()
 local connected=check_connection()
 if connected==true then
  if not st.active then start_all("connected_tick") end
 elseif connected==false then
  if st.active or st.connected then stop_all("disconnected_tick") end
 end
 return true
end
function M.apply()
 if installed then return true end
 installed=true
 plugin_api=plugin_api or {}
 plugin_api.yyTXTAutostartStatus=function()return st end
 plugin_api.yyTXTAutostartStart=function()return start_all("manual_requested",true) end
 plugin_api.yyTXTAutostartStop=function()return stop_all("manual_requested",true) end
 if type(events)=="table" and type(events.onBotReady)=="function" then
  pcall(events.onBotReady,"yy_txt_autostart_r64_ready")
 end
 if type(util)=="table" and type(util.scheduleSeconds)=="function" then
  pcall(util.scheduleSeconds,5,"yy_txt_autostart_r64_tick")
 end
 -- Also handle plugin reloads while already connected.
 yy_txt_autostart_r64_tick()
 persist()
 return true
end
return M
