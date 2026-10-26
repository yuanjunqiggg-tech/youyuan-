-- 疣螈真实生命值 v2

-- 所有数值只从服务端 UpdateAttributes/健康变化取得。

-- 20Hz 合并队列仅处理变化，不重复发送所有实体分数。

-- Bedrock belowname 仅支持玩家显示；生物血量写独立分数供后续行为包显示。

local M={}

local installed=false

local cache,dirty,waiting={}, {}, {}

local pending,head={},1

local player_rids={}

local player_last_refresh=0

local current_mob_snapshot,scene_at={},{}

local mob_scores,player_scores={},{}

local stats={packets=0,updates=0,player_writes=0,mob_writes=0,mob_skipped=0,events=0,last_error="",at=0}

local last_stat_write=0

local display_last=0

local display_refreshes=0



local function ts() return tonumber(util.timestamp()) or os.time() end

local function valid_name(s)

 s=tostring(s or "")

 if #s<1 or #s>64 or s:find('[%c"\\]') then return nil end

 return s

end

local function rid(v)

 if v==nil then return nil end

 local k=tostring(v)

 if k=="" or k=="0" or #k>32 or k:find("[^%w]") then return nil end

 return k

end

local function fast(cmd)

 if not (game and type(game.sendCommandFast)=="function") then return false end

 local ok,v=pcall(game.sendCommandFast,cmd,{channel="ai",rate_cps=0})

 if not ok then stats.last_error=tostring(v):sub(1,160) end

 return ok and v~=false

end

local function pos_from(v)

 if type(v)=="table" then

  local x=tonumber(v.x or v[1]);local y=tonumber(v.y or v[2]);local z=tonumber(v.z or v[3])

  if x and y and z then return {x=x,y=y,z=z} end

 elseif type(v)=="string" then

  local x,y,z=v:match("%[?([%-%.%d]+)[, ]+([%-%.%d]+)[, ]+([%-%.%d]+)")

  x,y,z=tonumber(x),tonumber(y),tonumber(z)

  if x and y and z then return {x=x,y=y,z=z} end

 end

 return nil

end

local function number_or_nil(n)

 if type(n)~="number" or n~=n or n<0 or n>2048 then return nil end

 return n

end

local function attr_from_packet(attrs)

 if type(attrs)~="table" then return nil end

 for _,v in pairs(attrs) do

  if type(v)=="table" then

   local data=type(v.attribute_value)=="table" and v.attribute_value or v

   local name=tostring(data.name or data.id or "")

   if name=="minecraft:health" or name=="health" then

    local health=number_or_nil(data.value or data.current)

    local maximum=number_or_nil(data.max or data.maximum or v.default_max)

    if health then return health,maximum end

   end

  end

 end

 return nil

end

local function queue(k)

 if not waiting[k] then

  waiting[k]=true

  pending[#pending+1]=k

 end

end

local function write_health(k,h,max,kind)

 if not k then return end

 local old=cache[k]

 if type(old)~="table" then old={} cache[k]=old end

 if type(h)~="number" then return end

 local rounded=h>0 and math.max(1,math.floor(h+0.5)) or 0

 if rounded<0 or rounded>2048 then return end

 local different=(old.h~=rounded or (max~=nil and old.max~=max))

 old.h=rounded;old.raw=h

 if max~=nil then old.max=max end

 old.time=ts()

 if kind then old.kind=kind end

 if different then

  stats.updates=stats.updates+1

  queue(k)

 end

end

local function refresh_players(force)

 local now=ts()

 if not force and now-player_last_refresh<2 then return end

 player_last_refresh=now

 local ok,players=pcall(player.list)

 if not ok or type(players)~="table" then return end

 player_rids={}

 for _,p in pairs(players) do

  if type(p)=="table" and p.online~=false then

   local n=valid_name(p.name)

   local k=rid(p.runtime_id or p.entity_runtime_id)

   if n and k then player_rids[k]=n end

  end

 end

end

local function refresh_scene(k)

 local now=ts()

 if now-(scene_at[k] or 0)<2 then return end

 scene_at[k]=now

 current_mob_snapshot[k]=nil

 if not(player and type(player.position)=="function") then return end

 local ok,a=pcall(player.position,k)

 if not ok or type(a)~="table" or a.known~=true then return end

 local n=valid_name(a.name)

 if a.entity_type=="minecraft:player" and n then

  player_rids[k]=n

 elseif type(a.entity_type)=="string" and a.entity_type:match("^minecraft:[%w_]+$") then

  if a.position_known==true and a.position_stale~=true

   and a.stale~=true and (tonumber(a.position_age_ms) or 999999)<2500 then

   local pos=pos_from(a.position)

   if pos then current_mob_snapshot[k]={type=a.entity_type,pos=pos,at=now} end

  end

 end

end

local function accept(f,source)

 if type(f)=="table" and type(f.fields)=="table" then f=f.fields end

 if type(f)~="table" then return end

 stats.packets=stats.packets+1

 local k=rid(f.entity_runtime_id or f.runtime_id)

 if not k then return end

 if source=="AddPlayer" then

  local n=valid_name(f.username or f.name)

  if n then player_rids[k]=n end

 end

 if source=="AddActor" then

  local et=tostring(f.entity_type or "")

  local pos=pos_from(f.position)

  local old=cache[k]

  if not old then old={} cache[k]=old end

  if et:match("^minecraft:[%w_]+$") then old.type=et end

  if pos then old.pos=pos;old.pos_at=ts() end

 end

 local h,max=attr_from_packet(f.attributes)

 if h then write_health(k,h,max,source) end

end

function yy_hp2_selfhealth(ev)

 if type(ev)=="table" and type(ev.fields)=="table" then ev=ev.fields end

 if type(ev)~="table" then return end

 local v=number_or_nil(ev.health)

 if v==nil then return end

 local ok,botdata=pcall(bot.self)

 local n=ok and type(botdata)=="table" and botdata.name or nil

 if type(n)~="string" or not valid_name(n) then return end

 local points=v>0 and math.max(1,math.floor(v+0.5)) or 0

 if player_scores[n]~=points then

  if fast('scoreboard players set "'..n..'" yy_hp '..points) then

   player_scores[n]=points;stats.player_writes=stats.player_writes+1

  end

 end

end

function yy_hp2_attr(f) accept(f,"UpdateAttributes") end

function yy_hp2_spawn(f) accept(f,"AddActor") end

function yy_hp2_player(f) accept(f,"AddPlayer") end

function yy_hp2_health(ev)

 if type(ev)~="table" then return end

 stats.events=stats.events+1

 local k=rid(ev.runtime_id or ev.entity_runtime_id)

 local h=number_or_nil(ev.health)

 if k and h then

  write_health(k,h,number_or_nil(ev.max_health),"onHealthChange")

 elseif h and type(ev.player)=="string" then

  -- 如果事件只有玩家名，记录到名字；不猜测runtime_id。

  local n=valid_name(ev.player)

  if n then

   local value=h>0 and math.max(1,math.floor(h+0.5)) or 0

   if player_scores[n]~=value and fast('scoreboard players set "'..n..'" yy_hp '..value) then

    player_scores[n]=value;stats.player_writes=stats.player_writes+1

   end

  end

 end

end

local function cmd_for_mob(k,item)

 local mob=current_mob_snapshot[k]

 -- 尚未进入game.scene新快照的新实体，使用AddActor刚刚同步的精确出生位置。

 if not mob and type(item)=="table" and type(item.type)=="string"

   and type(item.pos)=="table" and ts()-(item.pos_at or 0)<=1 then

   mob={type=item.type,pos=item.pos,at=item.pos_at}

 end

 if not mob then return nil end

 -- 只定位服务器刚同步且精确的位置。单实体c=1避免影响整个实体类型。

 local t=mob.type

 local pos=mob.pos

 if not t or not pos then return nil end

 if math.abs(pos.x)>30000000 or math.abs(pos.z)>30000000 or pos.y< -100 or pos.y>330 then return nil end

 local xyz=string.format("x=%.2f,y=%.2f,z=%.2f",pos.x,pos.y,pos.z)

 return string.format("scoreboard players set @e[type=%s,%s,r=0.6,c=1] yy_mob_hp %d",t,xyz,item.h)

end

local function apply(k)

 local item=cache[k]

 if not item or not item.h or ts()-(item.time or 0)>45 then return end

 local n=player_rids[k]

 if n then

  if player_scores[n]~=item.h then

   if fast('scoreboard players set "'..n..'" yy_hp '..item.h) then

    player_scores[n]=item.h;stats.player_writes=stats.player_writes+1

   end

  end

  return

 end

 local command=cmd_for_mob(k,item)

 if command then

  if mob_scores[k]~=item.h and fast(command) then

   mob_scores[k]=item.h;stats.mob_writes=stats.mob_writes+1

  end

 else

  stats.mob_skipped=stats.mob_skipped+1

 end

end

function yy_hp2_tick()

 -- 兼容历史的黑框排查操作：它会清空 belowname 显示槽。

 -- 每20秒确认一次归属；200ms批量处理仅更新变更数据。

 local now=ts()

 if now-display_last>=20 then

  display_last=now

  if fast("scoreboard objectives setdisplay belowname yy_hp") then

    display_refreshes=display_refreshes+1

  end

 end

 refresh_players(false)

 local count=0

 while head<=#pending and count<8 do

  local k=pending[head]

  pending[head]=false

  head=head+1

  waiting[k]=nil

  if k and cache[k] then

   if not player_rids[k] then refresh_scene(k) end

   apply(k)

  end

  count=count+1

 end

 if head>#pending then pending={};head=1 end

 local now=ts()

 if now-last_stat_write>=30 then

  last_stat_write=now

  local n=0;for _ in pairs(cache) do n=n+1 end

  local diag={at=now,packets=stats.packets,updates=stats.updates,events=stats.events,

    player_writes=stats.player_writes,mob_writes=stats.mob_writes,

    display_refreshes=display_refreshes,display_slot="belowname",

    mob_skipped=stats.mob_skipped,tracked=n,last_error=stats.last_error,

    explanation="raw UpdateAttributes; belowname supports players only; mob selector requires recent position"}

  pcall(util.json_write,game.dataDir().."/yy_hp_v2_status.json",diag)

  -- 定时清掉很旧的runtime缓存，免得服务器运行数日逐渐膨胀

  for k,v in pairs(cache) do

   if now-(v.time or v.pos_at or 0)>180 then

    cache[k]=nil;mob_scores[k]=nil;current_mob_snapshot[k]=nil;scene_at[k]=nil

   end

  end

 end

 return true

end

function M.apply()

 if installed then return true end

 installed=true

 -- 写分数但不反复重建计分板；belowname 客户端仅对玩家显示。

 fast('scoreboard objectives add yy_hp dummy "§c❤ 血量"')

 fast('scoreboard objectives add yy_mob_hp dummy "§c❤ 生物血量"')

 fast('scoreboard objectives setdisplay belowname yy_hp')

 display_last=ts()

 -- Login attributes may arrive before this module loads. Reuse only this session's observed player values.
 if game and type(game.scene)=="function" then
  local ok,scene=pcall(game.scene)
  if ok and type(scene)=="table" and scene.connected==true then
   for _,a in ipairs(scene.actors or {}) do
    local n=valid_name(a.name)
    local h=number_or_nil(a.health)
    if a.present~=false and a.entity_type=="minecraft:player" and a.health_known==true and n and h then
     local points=h>0 and math.max(1,math.floor(h+.5)) or 0
     if player_scores[n]~=points and fast('scoreboard players set "'..n..'" yy_hp '..points) then
      player_scores[n]=points;stats.player_writes=stats.player_writes+1
      stats.snapshot_writes=(stats.snapshot_writes or 0)+1
     end
    end
   end
  end
 end

 if events then

  if type(events.subscribe)=="function" then

   pcall(events.subscribe,"UpdateAttributes","yy_hp2_attr")

   pcall(events.subscribe,"AddActor","yy_hp2_spawn")

   pcall(events.subscribe,"AddPlayer","yy_hp2_player")

   pcall(events.subscribe,"SetHealth","yy_hp2_selfhealth")

  end

  if type(events.onHealthChange)=="function" then

   pcall(events.onHealthChange,"yy_hp2_health")

  end

 end

 -- Four native ticks (200ms); avoid fractional-second scheduler coercion to 1 tick.

 if util and type(util.scheduleTick)=="function" then

  pcall(util.scheduleTick,4,"yy_hp2_tick")

 elseif util and type(util.scheduleSeconds)=="function" then

  pcall(util.scheduleSeconds,1,"yy_hp2_tick")

 end

 plugin_api=plugin_api or {}

 plugin_api.hpPlayerValue=function(n)

  local v=player_scores[tostring(n or "")]

  if type(v)=="number" then return v end

  return nil

 end

 plugin_api.hpRuntimeStatus=function()

  return {packets=stats.packets,updates=stats.updates,player_writes=stats.player_writes,

    mob_writes=stats.mob_writes,mob_skipped=stats.mob_skipped,last_error=stats.last_error,snapshot_writes=stats.snapshot_writes or 0}

 end

 pcall(util.json_write,game.dataDir().."/yy_hp_v2_loaded.json",

  {ready=true,at=ts(),parse="attributes[].attribute_value.name/value/max",frequency_ms=50})

 return true

end

return M