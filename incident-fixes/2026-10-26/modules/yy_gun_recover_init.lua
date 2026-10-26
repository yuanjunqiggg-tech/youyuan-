-- YY gun recovery loader: one menu + one event-driven rod-gun runtime.
local M={}
local started=false
local callbacks={spawn=0,errors=0,last_error="",ticks=0,trigger=0,chat_event=0,wrapped=0,duplicate=0,stops=0,last_chat=""}
function yy_rollback_gun_hook(e)
  -- R108: dispatch expensive gun hook code ONLY for actual fishing hook actors.
  -- Preserve event-driven firing for fishing_hook/fishing_bobber unchanged.
  local f=type(e)=="table" and (e.fields or e.data or e) or nil
  if type(f)~="table" then return end
  local typ=f.entity_type or f.type or f.identifier
  if type(typ)~="string" or
     (not typ:find("fishing_hook",1,true) and not typ:find("fishing_bobber",1,true)) then return end
  callbacks.spawn=callbacks.spawn+1
  local ok,err=pcall(function() require("modules.yy_gun_runtime").hook(e) end)
  if not ok then callbacks.errors=callbacks.errors+1;callbacks.last_error=tostring(err):sub(1,150) end
end
function yy_rollback_gun_tick()
  callbacks.ticks=callbacks.ticks+1
  local ok,err=pcall(function() require("modules.yy_gun_runtime").tick() end)
  if not ok then callbacks.errors=callbacks.errors+1;callbacks.last_error=tostring(err):sub(1,150) end
end
local last={}
local function trim(s)return (tostring(s or ""):gsub("^%s+",""):gsub("%s+$","")) end
local function native_args(v)
 if type(v)=="table" then
  if v.args and v.args~=v then return native_args(v.args) end
  if v.message then return native_args(v.message) end
  if v.text then return native_args(v.text) end
  local words={}
  for _,part in ipairs(v) do words[#words+1]=tostring(part) end
  return table.concat(words," ")
 end
 return trim(v)
end
local function dispatch(name,msg,source)
 if type(name)~="string" or type(msg)~="string" or name=="" then return false end
 local raw=trim(msg)
 if raw~=".枪" and not raw:match("^%.枪%s+") and raw~=".stop" then return false end
 local now=tonumber(util.monotonicMs()) or (tonumber(util.timestamp()) or 0)*1000
 local prev=last[name]
 if prev and prev.raw==raw and now-prev.when>=0 and now-prev.when<350 then
  callbacks.duplicate=callbacks.duplicate+1
  return true
 end
 local menu=require("modules.yy_gun_menu")
 local ok,handled=pcall(menu.onChat,name,raw)
 if not ok then
  callbacks.errors=callbacks.errors+1;callbacks.last_error=tostring(handled):sub(1,180)
  return false
 end
 if handled==true then
  last[name]={raw=raw,when=now}
  callbacks.last_chat=source..":"..name..":"..raw
  if raw==".stop" then callbacks.stops=callbacks.stops+1 end
  pcall(util.json_write,game.dataDir().."/yy_gun_r65_chat_status.json",{
   at=util.timestamp(),registration=callbacks.registration,
   trigger=callbacks.trigger,chat_event=callbacks.chat_event,
   wrapped=callbacks.wrapped,stops=callbacks.stops,duplicates=callbacks.duplicate,
   last_chat=callbacks.last_chat
  })
  return true
 end
 return false
end
function yy_r65_gun_trigger(name,args)
 callbacks.trigger=callbacks.trigger+1
 local text=native_args(args)
 if text=="" then text=".枪"
 elseif text~=".枪" and not text:match("^%.枪%s+") then text=".枪 "..text end
 return dispatch(name,text,"native_trigger")
end
function yy_r65_gun_chat(e,content)
 callbacks.chat_event=callbacks.chat_event+1
 local name,raw=e,content
 if type(e)=="table" then
  name=e.source_name or e.player_name or e.player or e.name or e.sender
  raw=e.message or e.text or e.filtered_message
 end
 if type(name)~="string" or type(raw)~="string" then return false end
 local own,botInfo=pcall(bot.self)
 if own and type(botInfo)=="table" and botInfo.name==name then return false end
 return dispatch(name,raw,"chat_event")
end
function M.apply()
  if started then return true end
  local good,data=pcall(require,"modules.yy_gun_data")
  local menuOK,menu=pcall(require,"modules.yy_gun_menu")
  local runtimeOK,runtime=pcall(require,"modules.yy_gun_runtime")
  if not(good and menuOK and runtimeOK) then
    error("gun modules unavailable: "..tostring(data).." | "..tostring(menu).." | "..tostring(runtime))
  end
  plugin_api=plugin_api or {}
  plugin_api.yyGunV3Status=function()
    return {loaded=true,enabled=data.enabled(),guns=#data.all(),runtime=runtime.stats(),
      callbacks=callbacks,source="roll-back recovered unified rod-gun"}
  end
  -- A remote caller cannot impersonate an in-game OP. Toggle only from .枪 admin menu.
  plugin_api.yyGunV3SetEnabled=function()
    return {ok=false,error="使用管理员 .枪 → 7 → 1"}
  end
  local old=on_chat
  if type(old)=="function" then
    on_chat=function(name,msg)
      callbacks.wrapped=callbacks.wrapped+1
      if dispatch(name,msg,"on_chat") then return true end
      return old(name,msg)
    end
  end
  callbacks.registration={}
  if game and type(game.registerTrigger)=="function" then
    local ok,v=pcall(game.registerTrigger,{".枪"},"yy_r65_gun_trigger")
    callbacks.registration.native=ok and v~=false
  end
  if events and type(events.onChat)=="function" then
    local ok,v=pcall(events.onChat,"yy_r65_gun_chat")
    callbacks.registration.chat=ok and v~=false
  end
  if events then
    local filtered=false
    if type(events.subscribeFiltered)=="function" then
      for _,typ in ipairs({"minecraft:fishing_hook","minecraft:fishing_bobber","fishing_hook","fishing_bobber"}) do
        local ok,value=pcall(events.subscribeFiltered,"AddActor",{fields={entity_type=typ}},"yy_rollback_gun_hook")
        filtered=filtered or (ok and value~=false)
      end
    end
    -- Filter before the resident VM queue; retain the legacy route only on older hosts.
    if not filtered then
      if type(events.onEntitySpawn)=="function" then pcall(events.onEntitySpawn,"yy_rollback_gun_hook") end
      if type(events.subscribe)=="function" then
        pcall(events.subscribe,"AddActor","yy_rollback_gun_hook")
        pcall(events.subscribe,"AddEntity","yy_rollback_gun_hook")
      end
    end
  end
  if util and type(util.scheduleSeconds)=="function" then
    pcall(util.scheduleSeconds,1,"yy_rollback_gun_tick")
  end
  local native_ok,native_mod=pcall(require,"modules.yy_gun_native_r65")
  if native_ok and type(native_mod)=="table" and type(native_mod.apply)=="function" then
    local apply_ok,result=pcall(native_mod.apply)
    callbacks.native={loaded=true,applied=apply_ok and result~=false,result=tostring(result)}
  else
    callbacks.native={loaded=false,error=tostring(native_mod):sub(1,280)}
  end
  started=true
  plugin_api.yyGunR65ChatStatus=function()return callbacks end
  pcall(util.json_write,game.dataDir().."/yy_gun_r65_chat_status.json",{registration=callbacks.registration,started=true,native=callbacks.native})
  return true
end
return M