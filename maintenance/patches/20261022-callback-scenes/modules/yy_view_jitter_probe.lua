-- 疣螈视角包诊断：MovePlayer(位置/视角更新)微抖动 != 真实CPS；只观测不处罚。
-- 不扫描世界、不传送、不改原战斗模块；服务端未发送的输入不推测。
local M={}
local loaded=false
local id_to_name={}
local states={}
local counts={move_packets=0,missing_id=0,missing_rotation=0,mapped=0,unmapped=0,
  small_delta=0,large_delta=0,stationary=0,windows=0,high_jitter_windows=0,
  errors=0,identity_refreshes=0,online_players=0}
local last={};local total_fields={};local last_refresh=0;local flush_at=0
local path=game.dataDir().."/yy_view_jitter_probe_status.json"
local function now()return tonumber(util.timestamp()) or os.time() end
local function millis()
 if util and type(util.monotonicMs)=="function" then
  local ok,ms=pcall(util.monotonicMs)
  if ok and type(ms)=="number" then return ms end
 end
 return now()*1000
end
local function fields(e)
 if type(e)~="table" then return {} end
 if type(e.fields)=="table" then return e.fields end
 if type(e.data)=="table" then return e.data end
 return e
end
local function number(v)
 local n=tonumber(v)
 if not n or n~=n or math.abs(n)>1e7 then return nil end
 return n
end
local function name_ok(n)
 return type(n)=="string" and #n>0 and #n<=70 and not n:find('[%c"\\]')
end
local function refresh()
 local t=now()
 if t-last_refresh<4 then return end
 last_refresh=t
 counts.identity_refreshes=counts.identity_refreshes+1
 if player and type(player.list)=="function" then
  local ok,p=pcall(player.list)
  if ok and type(p)=="table" then
   local n=0
   for _,e in pairs(p) do
    if type(e)=="table" and e.online~=false and name_ok(e.name) then
     n=n+1
     -- unique_id并非runtime_id，不作错误对照。
     local rid=e.runtime_id or e.entity_runtime_id
     if rid then id_to_name[tostring(rid)]=e.name end
    end
   end
   counts.online_players=n
  end
 end
 if game and type(game.scene)=="function" then
  local ok,scene=pcall(game.scene,{present_only=true,entity_type="minecraft:player"})
  if ok and type(scene)=="table" and type(scene.actors)=="table" then
   for _,e in pairs(scene.actors) do
    if type(e)=="table" and e.entity_type=="minecraft:player" and
      name_ok(e.name) and e.runtime_id~=nil and e.present~=false then
      id_to_name[tostring(e.runtime_id)]=e.name
    end
   end
  end
 end
end
local function norm_angle(v)
 if v>180 then v=v-360 end
 if v< -180 then v=v+360 end
 return v
end
local function advance(name,rec,t)
 if not rec.start or t-rec.start>=1000 or t<rec.start then
  if rec.start then
   counts.windows=counts.windows+1
   if rec.jitter>20 then counts.high_jitter_windows=counts.high_jitter_windows+1 end
   last[name]={name=name,window_ms=t-rec.start,
     jitter=rec.jitter,observed=rec.observed,at=now(),max_per_s=rec.jitter}
  end
  rec.start=t;rec.jitter=0;rec.observed=0
 end
end
function yy_view_probe_add_player(e)
 local f=fields(e)
 local rid=f.entity_runtime_id or f.runtime_id
 local name=f.username or f.name
 if rid and name_ok(name) then id_to_name[tostring(rid)]=name end
end
function yy_view_probe_move(e)
 if not loaded then return end
 counts.move_packets=counts.move_packets+1
 local f=fields(e)
 local rid=f.entity_runtime_id or f.runtime_id
 if not rid then counts.missing_id=counts.missing_id+1;return end
 local pitch=number(f.pitch);local yaw=number(f.yaw)
 if not pitch or not yaw then
   counts.missing_rotation=counts.missing_rotation+1;return
 end
 local key=tostring(rid)
 local name=id_to_name[key]
 if not name then
  refresh()
  name=id_to_name[key]
 end
 if not name then counts.unmapped=counts.unmapped+1;return end
 counts.mapped=counts.mapped+1
 local t=millis()
 local s=states[key]
 if not s then
  s={pitch=pitch,yaw=yaw,start=t,jitter=0,observed=0,name=name}
  states[key]=s;return
 end
 s.name=name
 advance(name,s,t)
 local dP=math.abs(pitch-s.pitch)
 local dY=math.abs(norm_angle(yaw-s.yaw))
 s.pitch=pitch;s.yaw=yaw;s.observed=s.observed+1
 local d=math.max(dP,dY)
 if d>=0.02 and d<=0.8 then
  s.jitter=s.jitter+1;counts.small_delta=counts.small_delta+1
 elseif d>0.8 then counts.large_delta=counts.large_delta+1
 else counts.stationary=counts.stationary+1 end
end
local function snapshot()
 refresh()
 local players={}
 for _,v in pairs(states) do
  if #players<16 then
   players[#players+1]={name=v.name,window_micro_jitter=v.jitter,
    window_rotation_packets=v.observed,latest_completed=last[v.name],
    note="微小转角变化次数，不是CPS"}
  end
 end
 pcall(util.json_write,path,{loaded=loaded,at=now(),mode="observe_only",
  packet="MovePlayer",filter_every_ms=250,min_angle=0.02,max_angle=0.8,
  kick_enabled=false,teleport_enabled=false,
  counts=counts,players=players,
  caveat="rotation_jitter_is_not_click_count; filtered_sampling_cannot_determine_exact_CPS"})
end
function yy_view_probe_tick()
 if not loaded then return end
 snapshot()
end
local old_chat
local function chat(n,msg)
 if tostring(msg or "")==".视角诊断" then
  local allowed=n=="疣螈"
  if type(is_admin)=="function" then
   local ok,v=pcall(is_admin,n);if ok and v then allowed=true end
  end
  if not allowed then
   if type(send)=="function" then pcall(send,n,"§c仅管理员可查看。") end
   return true
  end
  local lines={"§b视角微抖诊断 §7(观察模式，不踢人)",
   "§7位置与视角包 §f"..counts.move_packets..
   " §7已映射 §a"..counts.mapped.." §7未映射 §e"..counts.unmapped,
   "§7微小转角 §f"..counts.small_delta.." §7疑似高频窗口 §e"..counts.high_jitter_windows,
   "§7本次是按250ms抽样的视角事件，不等于真实CPS"}
  for _,p in pairs(states) do
   if #lines>=8 then break end
   lines[#lines+1]="§f"..tostring(p.name).." §7窗口微抖 "..p.jitter.." / 旋转包 "..p.observed
  end
  if type(send)=="function" then pcall(send,n,table.concat(lines,"\n")) end
  return true
 end
 return old_chat and old_chat(n,msg)
end
function yy_view_bot_invisible()
 if not loaded then return end
 -- 仅机器人自身，12小时补一次；不传送、不修改视距。
 if game and type(game.sendCommandFast)=="function" then
  local ok,reply=pcall(game.sendCommandFast,
   "effect @s invisibility 999999 0 true",{channel="ai",rate_cps=0})
  counts.invisibility_renew_submitted=(counts.invisibility_renew_submitted or 0)+(ok and reply~=false and 1 or 0)
  if not ok or reply==false then counts.errors=counts.errors+1 end
 end
end
function M.apply()
 if loaded then return true end
 loaded=true;refresh()
 if events and type(events.subscribe)=="function" then
  pcall(events.subscribe,"AddPlayer","yy_view_probe_add_player")
 end
 if events and type(events.subscribeFiltered)=="function" then
  local ok,res=pcall(events.subscribeFiltered,"MovePlayer",{every_ms=250},"yy_view_probe_move")
  if not ok or res==false then counts.errors=counts.errors+1 end
 else counts.errors=counts.errors+1 end
 if util and type(util.scheduleSeconds)=="function" then
  pcall(util.scheduleSeconds,60,"yy_view_probe_tick")
  pcall(util.scheduleSeconds,43200,"yy_view_bot_invisible")
 end
 yy_view_bot_invisible()
 old_chat=on_chat
 if type(old_chat)=="function" then on_chat=chat end
 plugin_api=plugin_api or {}
 plugin_api.yyViewProbeStatus=function()return counts end
 snapshot()
 return true
end
return M