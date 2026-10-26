-- 受伤实体战斗速览 v3。事件单次注册，后续仅在真实血量下降时更新。
-- 通过RuntimeID直接定位实体；不加Tag、不全服遍历、不猜伤害归属。
local M,active={},false
local actors,players,swings,shown={},{},{},{}
local counts={attrs=0,swings=0,hurt=0,matched=0,notices=0,unmatched=0,boss_notices=0}
local last_diag=0
local first_hit_pending={}
local first_hit_events=0
local first_hit_shown=0
local health_pending_shown=0
local shown_quality={}
local fallback_hits=0
local fallback_skips=0
local reason_last=''
local rejection={missing_actor_pos=0,missing_candidate=0,known_swing=0,unknown_swing=0,last_actor="",last_swing="",last_player="",last_distance=0,seen_6063=0}
local label={zombie="僵尸",skeleton="骷髅",creeper="苦力怕",spider="蜘蛛",cow="牛",
 sheep="羊",pig="猪",chicken="鸡",warden="监守者",enderman="末影人",villager="村民",
 iron_golem="铁傀儡",wolf="狼",witch="女巫",drowned="溺尸",husk="尸壳",
 blaze="烈焰人",wither="凋灵",ender_dragon="末影龙",elder_guardian="远古守卫者",
 pillager="掠夺者",ravager="劫掠兽",slime="史莱姆",ghast="恶魂"}
local boss={warden=true,wither=true,ender_dragon=true,elder_guardian=true,ravager=true}
local function tm() return tonumber(util.timestamp()) or os.time() end
local function un(ev) return type(ev)=="table" and (type(ev.fields)=="table" and ev.fields or ev) or {} end
local function pos(v)
 if type(v)~="table" then return nil end
 local x,y,z=tonumber(v.x or v[1]),tonumber(v.y or v[2]),tonumber(v.z or v[3])
 if not x or not y or not z then return nil end
 return {x=x,y=y,z=z}
end
local function isnum(n) return type(n)=="number" and n==n and n>=0 and n<=2048 end
local function verified_health(n) return isnum(n) and n or nil end
local display_counters={damage_known=0,damage_unknown=0,motion_updates=0,stale_snapshot_used=0}
local function actor(k)
 local a=actors[k]
 if not a then a={} actors[k]=a end
 return a
end
local function readable(t)
 local raw=tostring(t or "minecraft:unknown"):gsub("^minecraft:","")
 return label[raw] or raw:gsub("_"," ")
end
local function parse(attrs)
 local hp,mx,armor=nil,nil,nil
 if type(attrs)~="table" then return nil end
 for _,a in pairs(attrs) do
  if type(a)=="table" then
   local v=type(a.attribute_value)=="table" and a.attribute_value or a
   if v.name=="minecraft:health" and isnum(v.value) then
    hp=v.value;mx=isnum(v.max) and v.max or nil
   elseif (v.name=="minecraft:armor" or v.name=="minecraft:armour") and isnum(v.value) then armor=v.value end
  end
 end
 return hp,mx,armor
end
local function distance_sq(a,b)
 if not a or not b then return math.huge end
 local x,y,z=a.x-b.x,a.y-b.y,a.z-b.z
 return x*x+y*y+z*z
end
local function write_to(n,a,h,m,boss_mode)
 local t=tm()
 if not n then return false end
 if t-(shown[n] or 0)<1 and (h==nil or shown_quality[n]=="health") then return false end
 if plugin_api and type(plugin_api.profileDown)=="function" then
  local ok,down=pcall(plugin_api.profileDown,n)
  if ok and down then return end
 end
 local visible=(type(a.display_name)=="string" and a.display_name~="") and (a.display_name.." §8("..readable(a.type)..")") or readable(a.type)
 local text="§c⚔ §b"..visible.." §8#"..tostring(a.runtime_id or "?")
 local dmg=(type(a.last_damage)=="number" and a.last_damage>=0 and tm()-(a.last_damage_at or 0)<=3) and a.last_damage or nil
 if dmg then text=text.." §6本次-"..string.format("%.1f",dmg) else text=text.." §7本次--" end
 if type(h)=="number" then
  text=text.." §7│ §c❤ §f"..string.format("%.1f",h)
  if type(m)=="number" then text=text.."/"..string.format("%.1f",m) end
 else text=text.." §7│ §c❤ §7正在同步" end
 local defense=a.armor_attribute or a.armor_equipment
 if defense~=nil then text=text.." §7护甲 §b"..string.format("%.0f",defense) else text=text.." §7护甲 §8待同步" end
 local ok=pcall(game.sendActionbar,n,text)
 if ok then
  shown[n]=t
  shown_quality[n]=(type(h)=="number") and "health" or "pending"
  counts.notices=counts.notices+1
  if not h then health_pending_shown=health_pending_shown+1 end
  if boss_mode then counts.boss_notices=counts.boss_notices+1 end
 end
 return ok
end
local function update_pos_from_cache(p)
 if not p or type(p.name)~="string" then return end
 if not player or type(player.get)~="function" then return end
 local ok,record=pcall(player.get,p.name)
 if not ok or type(record)~="table" or record.online==false then return end
 local x,y,z=tonumber(record.x),tonumber(record.y),tonumber(record.z)
 if x and y and z then
  p.p={x=x,y=y,z=z}
  p.pos_at=tm()
 end
end
local function refresh_actor_on_hurt(k,a)
 -- 只有真实受到攻击时按RuntimeID读取一次观测快照，不扫描全服。
 if a.p and tm()-(a.pos_at or 0)<8 then return end
 if not game or type(game.scene)~="function" then return end
 local ok,scene=pcall(game.scene,{present_only=true})
 if not ok or type(scene)~="table" or type(scene.actors)~="table" then return end
 for _,e in pairs(scene.actors) do
  if type(e)=="table" and tostring(e.runtime_id or "")==k then
   if type(e.entity_type)=="string" then a.type=e.entity_type end
   if type(e.name)=="string" and e.name~="" then a.display_name=e.name end
   if e.present~=false and e.position_known==true
    and (e.position_stale~=true or (tonumber(e.position_age_ms) or 999999)<300000) then
    local p=pos(e.position)
    if p then
     a.p=p;a.pos_at=tm();a.position_source="host_scene"
     a.dimension=tonumber(e.dimension) or a.dimension
     if e.position_stale==true then display_counters.stale_snapshot_used=display_counters.stale_snapshot_used+1 end
    end
   end
   return
  end
 end
end
local identity_probe_at={}
local match_diag={attempts=0,found=0,unresolved=0,ambiguous=0,near_swings=0}
local function match_player_by_runtime(k)
 rejection.last_swing=k
 local known=players[k]
 if known and type(known.name)=="string" and known.name~="" then return known end
 local t=tm()
 if t-(identity_probe_at[k] or 0)<10 then return nil end
 identity_probe_at[k]=t
 match_diag.attempts=match_diag.attempts+1
 if not game or type(game.scene)~="function" then return nil end
 if not player or type(player.list)~="function" then return nil end
 local ok,scene=pcall(game.scene,{present_only=true})
 if not ok or type(scene)~="table" or type(scene.actors)~="table" then return nil end
 local actor_pos=nil
 local actor_dim=nil
 for _,a in pairs(scene.actors) do
  if type(a)=="table" and tostring(a.runtime_id or "")==k then
   if a.stale~=true and a.position_stale~=true then actor_pos=pos(a.position) end
   actor_dim=tonumber(a.dimension)
   break
  end
 end
 if not actor_pos then match_diag.unresolved=match_diag.unresolved+1;return nil end
 local yes,list=pcall(player.list)
 if not yes or type(list)~="table" then return nil end
 if k=="6063" then
  pcall(function()
   local sample={}
   for _,p in pairs(list) do
    if type(p)=="table" then
     sample[#sample+1]={name=p.name,online=p.online,dim=p.dimension,
      x=p.x,y=p.y,z=p.z,px=type(p.position)=="table" and p.position.x or nil}
    end
   end
   util.json_write(game.dataDir().."/yy_combat_match_probe.json",{
    at=t,rid=k,actor_pos=actor_pos,actor_dim=actor_dim,
    actor_from_scene=true,players=sample})
  end)
 end
 local found=nil
 local hits=0
 for _,p in pairs(list) do
  if type(p)=="table" and type(p.name)=="string" and p.name~="" and p.online~=false then
   local pp=pos(p)
   if pp and (actor_dim==nil or tonumber(p.dimension)==nil or tonumber(p.dimension)==actor_dim) then
    local ds=distance_sq(pp,actor_pos)
    if ds<=2.25 then
     hits=hits+1
     found={name=p.name,p=actor_pos,pos_at=t}
    end
   end
  end
 end
 if hits==1 then
  players[k]=found
  match_diag.found=match_diag.found+1
  return found
 end
 if hits>1 then match_diag.ambiguous=match_diag.ambiguous+1 else match_diag.unresolved=match_diag.unresolved+1 end
 return nil
end
local function notify(k,a,h,m)
 refresh_actor_on_hurt(k,a)
 rejection.last_actor=k
 if not a.p or tm()-(a.pos_at or 0)>6 then
  rejection.missing_actor_pos=rejection.missing_actor_pos+1
  counts.unmatched=counts.unmatched+1
  return
 end
 local raw=tostring(a.type or ""):gsub("^minecraft:","")
 local multi=boss[raw] or (m and m>=100)
 local candidates={}
 for id,t in pairs(swings) do
  local p=players[id]
  if not p and tm()-t<=2 then p=match_player_by_runtime(id) end
  if p and p.name and tm()-t<=2 then
   update_pos_from_cache(p)
  end
  if p and p.name then
   rejection.known_swing=rejection.known_swing+1
   rejection.last_player=p.name
   rejection.last_distance=math.floor(distance_sq(p.p,a.p))
  else rejection.unknown_swing=rejection.unknown_swing+1 end
  if p and p.name and p.p and tm()-t<=2 and tm()-(p.pos_at or 0)<=8 then
   local ds=distance_sq(p.p,a.p)
   local limit=multi and 24 or 5
   if ds<=limit*limit then candidates[#candidates+1]={n=p.name,d=ds} end
  end
 end
 if #candidates==0 then
  -- 回退仅在真实血量下降、最近确有挥臂、5格内恰好一名在线玩家时启用。
  -- 实际服务器包没有保证攻击来源；多人场景拒绝推断。
  local recently=false
  for rid,at in pairs(swings) do
   if tm()-at<=2 then recently=true;break end
  end
  if recently and player and type(player.list)=="function" then
   local ok,list=pcall(player.list)
   if ok and type(list)=="table" then
    local nearby={}
    for _,pl in pairs(list) do
     if type(pl)=="table" and type(pl.name)=="string" and pl.name~=""
      and pl.online~=false and (pl.dimension==nil or tonumber(pl.dimension)==0) then
      local pt=pos(pl)
      if pt and distance_sq(pt,a.p)<=25 then nearby[#nearby+1]=pl.name end
     end
    end
    if #nearby==1 then
     candidates[1]={n=nearby[1],d=0}
     fallback_hits=fallback_hits+1
    else
     fallback_skips=fallback_skips+1
     reason_last="nearby_players="..tostring(#nearby)
    end
   else reason_last="player_list_unavailable" end
  else reason_last="no_recent_swing" end
 end
 if #candidates==0 then rejection.missing_candidate=rejection.missing_candidate+1;counts.unmatched=counts.unmatched+1;return end
 table.sort(candidates,function(a,b)return a.d<b.d end)
 counts.matched=counts.matched+1
 local used={}
 local sent=0
 for _,v in ipairs(candidates) do
  if (used.count or 0)>=1 and not multi then break end
  if not used[v.n] then
   if write_to(v.n,a,h,m,multi) then sent=sent+1 end
   used[v.n]=true
   used.count=(used.count or 0)+1
   if used.count>=8 then break end
  end
 end
 return sent>0
end
function yy_battle_actor(ev)
 local p=un(ev)
 local k=p.entity_runtime_id
 if k==nil then return end
 k=tostring(k)
 local a=actor(k)
 if type(p.entity_type)=="string" then a.type=p.entity_type end
 if p.entity_unique_id~=nil then a.unique_id=tostring(p.entity_unique_id) end
 if type(p.name)=="string" and p.name~="" then a.display_name=p.name end
 local pt=pos(p.position)
 if pt then a.p=pt;a.pos_at=tm() end
 local h,m,armor=parse(p.attributes)
 if h~=nil then a.hp=h;a.mx=m or a.mx end
 if armor~=nil then a.armor_attribute=armor end
 a.updated=tm()
 a.runtime_id=k
end
function yy_battle_player(ev)
 local p=un(ev)
 if p.entity_runtime_id==nil or type(p.username)~="string" then return end
 local k=tostring(p.entity_runtime_id)
 players[k]={name=p.username,uuid=p.uuid,p=pos(p.position),pos_at=tm()}
end
local move_match={attempts=0,found=0,no_player_position=0,no_candidates=0,ambiguous=0,last_id=""}
local last_move_match={}
local function identify_moving_player(k,pt)
 if not pt or players[k] and players[k].name then return end
 if tm()-(last_move_match[k] or 0)<2 then return end
 last_move_match[k]=tm()
 move_match.attempts=move_match.attempts+1;move_match.last_id=k
 if not player or type(player.list)~="function" then return end
 local ok,all=pcall(player.list)
 if not ok or type(all)~="table" then return end
 local matches,count={},0
 for _,p in pairs(all) do
  if type(p)=="table" and type(p.name)=="string" and p.name~="" and p.online~=false then
   local ppos=pos(p)
   if ppos then
    if distance_sq(ppos,pt)<=4 then
     count=count+1;matches[#matches+1]={name=p.name,p=pt,pos_at=tm()}
    end
   else move_match.no_player_position=move_match.no_player_position+1 end
  end
 end
 if count==1 then
  players[k]=matches[1]
  move_match.found=move_match.found+1
 elseif count>1 then move_match.ambiguous=move_match.ambiguous+1
 else move_match.no_candidates=move_match.no_candidates+1 end
 if k=="6063" then
  pcall(util.json_write,game.dataDir().."/yy_combat_move_probe.json",{
   at=tm(),rid=k,pitch_known=true,moved_to=pt,matches=count,
   attempts=move_match.attempts,found=move_match.found,
   missing_pos=move_match.no_player_position,
   no_candidates=move_match.no_candidates
  })
 end
end
function yy_battle_move(ev)
 local p=un(ev)
 if p.entity_runtime_id==nil then return end
 local k=tostring(p.entity_runtime_id)
 local pt=pos(p.position)
 if not pt then return end
 if players[k] then players[k].p=pt;players[k].pos_at=tm()
 elseif type(p.pitch)=="number" then identify_moving_player(k,pt) end
 if actors[k] then actors[k].p=pt;actors[k].pos_at=tm() end
end
local delta_move_observed=0
function yy_battle_delta_move(ev)
 -- MoveActorDelta 是相对更新；禁止把原始delta当绝对世界坐标使用。
 delta_move_observed=delta_move_observed+1
end
function yy_battle_observed_move(ev)
 -- 宿主已经把服务器的MoveActorDelta整合为已确认的世界绝对坐标。
 local p=un(ev)
 local k=p.runtime_id or p.entity_runtime_id
 if k==nil then return end
 k=tostring(k)
 local a=actors[k]
 local known_player=players[k]
 if not a and not known_player then return end
 local pt=pos(p.position)
 if not pt then return end
 if a then
  a.p=pt;a.pos_at=tm();a.position_source="host_onMove"
  a.dimension=tonumber(p.dimension) or a.dimension
 end
 if known_player then known_player.p=pt;known_player.pos_at=tm() end
 display_counters.motion_updates=display_counters.motion_updates+1
end
function yy_battle_swing(ev)
 local p=un(ev)
 local id=p.runtime_id or p.entity_runtime_id
 if id==nil then return end
 local k=tostring(id)
 if type(p.player)=="string" and p.player~="" then
  local entry=players[k] or {}
  entry.name=p.player
  players[k]=entry
 end
 swings[k]=tm()
 counts.swings=counts.swings+1
end
local delta_reads=0 -- 已取消每次移动扫描game.scene；受击时直接按Runtime ID读取一次。
function yy_battle_animate(ev)
 local p=un(ev)
 if p.entity_runtime_id==nil or tonumber(p.action_type)~=1 then return end
 local k=tostring(p.entity_runtime_id)
 
 counts.swings=counts.swings+1
 swings[k]=tm()
 if not players[k] then match_player_by_runtime(k) end
end
local function armor_item(v)
 if type(v)~="table" then return type(v)=="string" and v or "" end
 if type(v.name)=="string" then return v.name end
 if type(v.item_type)=="table" and type(v.item_type.name)=="string" then return v.item_type.name end
 return ""
end
local ARMOR={
 leather={1,3,2,1},gold={2,5,3,1},chainmail={2,5,4,1},
 iron={2,6,5,2},diamond={3,8,6,3},netherite={3,8,6,3},turtle={2,0,0,0}
}
function yy_battle_armor(ev)
 local p=un(ev)
 if p.entity_runtime_id==nil then return end
 local a=actor(tostring(p.entity_runtime_id))
 local names={"helmet","chestplate","leggings","boots"}
 local sum=0
 local known=false
 local unknown=false
 for i,slot in ipairs(names) do
  local v=p[slot]
  local name=armor_item(v):lower()
  if name~="" then
   known=true
   for kind,weights in pairs(ARMOR) do
    if name:find(kind,1,true) then
     sum=sum+weights[i]
     break
    end
   end
  elseif type(v)=="table" and (v.item_type~=nil or v.network_id~=nil) then
   unknown=true
  end
 end
 if not unknown then a.armor_equipment=sum;a.updated=tm() end
end
function yy_battle_hurt(ev)
 local p=un(ev)
 local id=p.runtime_id or p.entity_runtime_id
 if id==nil then return end
 local k=tostring(id)
 local a=actor(k)
 local h=verified_health(tonumber(p.health))
 local before=verified_health(tonumber(p.previous))
 a.hurt_at=tm()
 if before and h and before>h then
  a.last_damage=before-h;a.last_damage_at=tm()
  display_counters.damage_known=display_counters.damage_known+1
 end
 first_hit_pending[k]=tm()
 if h and h>=0 and (before and before>h or a.mx or tonumber(p.max_health)) then
  a.hp=h
  a.mx=tonumber(p.max_health) or a.mx
  if not a.last_report_at or tm()-a.last_report_at>=1 then
   counts.hurt=counts.hurt+1
   if notify(k,a,h,a.mx) then
    a.last_report_at=tm()
    first_hit_pending[k]=nil
    first_hit_shown=first_hit_shown+1
   end
  end
 end
end
-- 在真实ActorEvent伤害事件里，优先调用宿主的运行时ID属性读取接口。
-- 这是一次按需本地读，不向所有实体轮询，不把过期快照伪称实时。
local health_reads=0
local health_fresh=0
local health_stale=0
local health_unknown=0
local last_health_probe={at=0,runtime_id="",source="",known=false,stale=false,health=nil,max_health=nil,error=""}
local function probe_hit_health(k,a)
 health_reads=health_reads+1
 last_health_probe={at=tm(),runtime_id=tostring(k),source="player.health",known=false,stale=false}
 if not player or type(player.health)~="function" then
  last_health_probe.error="player.health_not_available"
  health_unknown=health_unknown+1
  return nil,nil
 end
 local ok,v=pcall(player.health,tostring(k))
 if not ok then
  last_health_probe.error=tostring(v):sub(1,180)
  health_unknown=health_unknown+1
  return nil,nil
 end
 if type(v)~="table" then
  last_health_probe.error="unexpected_result_"..type(v)
  health_unknown=health_unknown+1
  return nil,nil
 end
 local h,mx=tonumber(v.health),tonumber(v.max_health)
 last_health_probe.known=v.health_known==true
 last_health_probe.stale=v.stale==true
 last_health_probe.health=verified_health(h)
 last_health_probe.max_health=verified_health(mx)
 if (h~=nil and not isnum(h)) or (mx~=nil and not isnum(mx)) then
  last_health_probe.error="invalid_host_health_value"
 end
 if v.health_known==true and v.stale~=true and h and h>=0 and h<=2048 then
  health_fresh=health_fresh+1
  a.hp=h
  if mx and mx>0 and mx<=2048 then a.mx=mx end
  return h,a.mx
 end
 if v.health_known==true then health_stale=health_stale+1 else health_unknown=health_unknown+1 end
 return nil,nil
end
local packet_hurt_count=0
local packet_hurt_reports=0
function yy_battle_actor_hurt_pkt(ev)
 local p=un(ev)
 if tonumber(p.event_type)~=2 or p.entity_runtime_id==nil then return end
 packet_hurt_count=packet_hurt_count+1
 local k=tostring(p.entity_runtime_id)
 local a=actor(k)
 local now=tm()
 a.hurt_at=now
 first_hit_events=first_hit_events+1
 first_hit_pending[k]=now
 -- 收到实体受击包立刻按该实体RuntimeID读取当前血量。
 -- 如果宿主只返回过期缓存，则继续走UpdateAttributes补全；不会向玩家展示假的剩余血量。
 refresh_actor_on_hurt(k,a)
 local direct_health,direct_max=probe_hit_health(k,a)
 local snapshot_health=direct_health~=nil
 if game and type(game.scene)=="function" then
  local ok,scene=pcall(game.scene,{present_only=true})
  if ok and type(scene)=="table" and type(scene.actors)=="table" then
   for _,e in pairs(scene.actors) do
    if type(e)=="table" and tostring(e.runtime_id or "")==k then
     if type(e.entity_type)=="string" and e.entity_type~="" then a.type=e.entity_type end
     if type(e.name)=="string" and e.name~="" then a.display_name=e.name end
     if e.position_known==true and e.position_stale~=true then
      local pt=pos(e.position)
      if pt then a.p=pt;a.pos_at=now end
     end
     local age=tonumber(e.health_age_ms) or 999999
     if e.health_known==true and e.health_stale~=true
      and age<=2500 and isnum(e.health) then
      a.hp=e.health
      if isnum(e.max_health) then a.mx=e.max_health end
      snapshot_health=isnum(a.mx) and a.mx>0
     end
     break
    end
   end
  end
 end
 local h=snapshot_health and a.hp or nil
 local mx=snapshot_health and a.mx or nil
 -- 无可靠血量时立即提供一次“同步中”的受击提示；待UpdateAttributes到达再补齐精确数值。
 local emitted=notify(k,a,h,mx)
 if emitted then
  if snapshot_health then
   packet_hurt_reports=packet_hurt_reports+1
   first_hit_shown=first_hit_shown+1
   first_hit_pending[k]=nil
  end
  a.last_report_at=now
 end
end
function yy_battle_health(ev)
 local p=un(ev)
 if p.entity_runtime_id==nil then return end
 local h,m,armor=parse(p.attributes)
 if not h then return end
 counts.attrs=counts.attrs+1
 local k=tostring(p.entity_runtime_id)
 local a=actor(k)
 local before=verified_health(a.hp)
 if before and before>h then
  a.last_damage=before-h;a.last_damage_at=tm()
  display_counters.damage_known=display_counters.damage_known+1
 elseif first_hit_pending[k] then
  display_counters.damage_unknown=display_counters.damage_unknown+1
 end
 a.hp=h;a.mx=m or a.mx;if armor~=nil then a.armor_attribute=armor end;a.updated=tm()
 local pending=(first_hit_pending[k] and tm()-first_hit_pending[k]<=4)
 if (before and before-h>0.01) or (not before and a.hurt_at and tm()-a.hurt_at<=3) or pending then
  if not a.last_report_at or tm()-a.last_report_at>=1 or pending then
   if notify(k,a,h,m or a.mx) then
    a.last_report_at=tm()
    if pending then first_hit_pending[k]=nil;first_hit_shown=first_hit_shown+1 end
   end
   counts.hurt=counts.hurt+1
  end
 end
end
function yy_battle_cleanup()
 local t=tm()
 if t-last_diag<20 then return end
 last_diag=t
 local n=0
 for k,a in pairs(actors) do
  if t-(a.updated or 0)>90 then actors[k]=nil else n=n+1 end
 end
 for k,a in pairs(players) do if t-(a.pos_at or 0)>120 then players[k]=nil;swings[k]=nil end end
 for k,at in pairs(swings) do if t-at>4 then swings[k]=nil end end
 for k,at in pairs(first_hit_pending) do if t-at>8 then first_hit_pending[k]=nil end end
 pcall(util.json_write,game.dataDir().."/yy_entity_inspect_status.json",{
  at=t,tracked=n,attributes=counts.attrs,damage_changes=counts.hurt,
  recent_attacks=counts.swings,matched=counts.matched,notices=counts.notices,
  player_match_attempts=match_diag.attempts,player_match_found=match_diag.found,
  player_match_unresolved=match_diag.unresolved,player_match_ambiguous=match_diag.ambiguous,
  fallback_hits=fallback_hits,fallback_skips=fallback_skips,last_skip_reason=reason_last,
  actor_hurt_packets=packet_hurt_count,actor_hurt_reports=packet_hurt_reports,
  first_hit_events=first_hit_events,first_hit_shown=first_hit_shown,
  health_pending_shown=health_pending_shown,
  target_health_reads=health_reads,target_health_fresh=health_fresh,
  target_health_stale=health_stale,target_health_unknown=health_unknown,
  last_health_probe=last_health_probe,
  delta_location_reads=delta_reads,delta_move_observed=delta_move_observed,
  move_match=move_match,
  combat_display=display_counters,
  rejection=rejection,
  boss_notices=counts.boss_notices,unmatched=counts.unmatched,
  mode="runtime_id+recent_swing+distance;no_tags;event_update_only"})
end
function M.apply()
 if active then return true end
 active=true
 -- 重载时一次性接管宿主已观测RuntimeID，之后完全按事件更新。
 if game and type(game.scene)=="function" then
  pcall(function()
   local scene=game.scene({present_only=true})
   local arr=type(scene)=="table" and scene.actors or nil
   if type(arr)=="table" then
    local n=0
    for _,e in pairs(arr) do
     n=n+1
     if n>2000 then break end
     if type(e)=="table" and e.runtime_id~=nil and e.stale~=true then
      local k=tostring(e.runtime_id)
      local p=pos(e.position)
      if e.entity_type=="minecraft:player" and type(e.name)=="string" and e.name~="" then
       players[k]={name=e.name,p=p,pos_at=tm()}
      else
       local a=actor(k)
       if type(e.entity_type)=="string" then a.type=e.entity_type end
       if p and e.position_stale~=true then a.p=p;a.pos_at=tm() end
       a.updated=tm()
      end
     end
    end
   end
  end)
 end
 if events and type(events.subscribe)=="function" then
  for typ,handler in pairs({
   AddActor="yy_battle_actor",AddPlayer="yy_battle_player",
   MovePlayer="yy_battle_move",MoveActorAbsolute="yy_battle_move",MoveActorDelta="yy_battle_delta_move",
   Animate="yy_battle_animate",UpdateAttributes="yy_battle_health",
   ActorEvent="yy_battle_actor_hurt_pkt",
   MobArmourEquipment="yy_battle_armor"
  }) do
   if typ=="MovePlayer" or typ=="MoveActorAbsolute" or typ=="MoveActorDelta" then
    if type(events.subscribeFiltered)=="function" then
     pcall(events.subscribeFiltered,typ,{every_ms=250},handler)
    else pcall(events.subscribe,typ,handler) end
   else
    pcall(events.subscribe,typ,handler)
   end
  end
 end
 if events and type(events.onMove)=="function" then pcall(events.onMove,"yy_battle_observed_move") end
 if events and type(events.onHurt)=="function" then pcall(events.onHurt,"yy_battle_hurt") end
 if events and type(events.onSwing)=="function" then pcall(events.onSwing,"yy_battle_swing") end
 if util and type(util.scheduleSeconds)=="function" then pcall(util.scheduleSeconds,10,"yy_battle_cleanup") end
 pcall(util.json_write,game.dataDir().."/yy_battle_ready.json",{ready=true,at=tm(),mode="hurt-event only"})
 return true
end

-- YY_COMBAT_V4 / 2026-10-09 : 仅替换战斗订阅，不动原有其他模块。
-- 以下模块重载时新的 M.apply 覆盖上面的 V3；旧 V3 处理器不会注册。
do
 local V={actors={},ids={},unique_to_runtime={},swing={},shots={},view={},counts={
   actor_event=0,attr=0,health_drop=0,swing=0,arrow=0,matched=0,
   suggested=0,unknown=0,shown=0,score=0,skipped=0,armor=0,errors=0,
   stale_pos=0,no_candidate=0,ambiguous=0,throttled=0,locked=0,queued=0,
   queue_sent=0,move_events=0,duplicate=0},error="",pending={},frame=0,score_cache={},last_pvp={}}
 local swing_diag={raw_animate=0,action_swing=0,other_actions=0,
  missing_runtime_id=0,unmapped_runtime_id=0,native_swing_events=0,
  native_unmapped=0,mapped_animate=0,mapped_native=0,recent={}}
 local function now() return tonumber(util.timestamp()) or os.time() end
 local function swing_note(source,rid,action,name,reason)
  local list=swing_diag.recent
  list[#list+1]={at=now(),source=source,rid=rid and tostring(rid) or nil,
   action=action,name=name,reason=reason}
  if #list>8 then table.remove(list,1) end
 end
 local function data(e) if type(e)~="table" then return {} end
   return type(e.fields)=="table" and e.fields or (type(e.data)=="table" and e.data or e) end
 local function num(x)
   local n=tonumber(x)
   if n and n==n and n>=0 and n<=2048 then return n end
   return nil
 end
 local function vec(t)
   -- game.scene() 位置会采用 "[x y z]" 字符串；协议事件则多为Vec3表。
   local x,y,z
   if type(t)=="table" then
     x=tonumber(t.x or t.pos_x or t[1])
     y=tonumber(t.y or t.pos_y or t[2])
     z=tonumber(t.z or t.pos_z or t[3])
   elseif type(t)=="string" then
     local body=t:match("^%s*%[%s*(.-)%s*%]%s*$") or t
     local a,b,c=body:match("^%s*(%S+)%s+(%S+)%s+(%S+)%s*$")
     if not a then a,b,c=body:match("^%s*([^,%s]+)%s*,%s*([^,%s]+)%s*,%s*([^,%s]+)%s*$") end
     x,y,z=tonumber(a),tonumber(b),tonumber(c)
   end
   if not x or not y or not z then return nil end
   if math.abs(x)>30000000 or math.abs(z)>30000000 or y< -100 or y>350 then return nil end
   return {x=x,y=y,z=z}
 end
 local function dist(a,b)
   if not a or not b then return math.huge end
   return (a.x-b.x)^2+(a.y-b.y)^2+(a.z-b.z)^2
 end
 local function valid(n)
   return type(n)=="string" and #n>0 and #n<=64 and not n:find('[%c"\\]')
 end
 local function entity(k)
   k=tostring(k or "")
   if k=="" or k=="0" then return nil end
   local a=V.actors[k]
   if not a then a={runtime_id=k};V.actors[k]=a end
   a.used=now()
   return a
 end
 local function health_packet_range(a)
   V.packet_ranges=V.packet_ranges or {unknown=0,r0_32=0,r32_64=0,r64_96=0,r96_128=0,r128_plus=0,stale_position=0}
   local bucket="unknown"
   if a.p and V.botpos then
     local d=math.sqrt(dist(a.p,V.botpos))
     if d<=32 then bucket="r0_32"
     elseif d<=64 then bucket="r32_64"
     elseif d<=96 then bucket="r64_96"
     elseif d<=128 then bucket="r96_128"
     else bucket="r128_plus" end
   end
   V.packet_ranges[bucket]=(V.packet_ranges[bucket] or 0)+1
   if a.p and now()-(a.pat or 0)>90 then
     V.packet_ranges.stale_position=V.packet_ranges.stale_position+1
   end
 end
 local remote_players={}
 local function online()
   local out={}
   if not(player and type(player.list)=="function") then return out end
   local ok,tab=pcall(player.list)
   if not ok or type(tab)~="table" then
     V.recent={reason="player_list_failed",error=tostring(tab)}
     return out
   end
   -- Each online player is enriched by the same per-player native snapshot below.
   for _,p in pairs(tab) do
     if type(p)=="table" and valid(p.name) and p.online~=false then
       local point=vec(p.position) or vec(p)
       local dim=tonumber(p.dimension)
       local rid=p.runtime_id or p.entity_runtime_id
       local source=point and "player.list" or "unknown"
       if player and type(player.position)=="function" then
         local good,v=pcall(player.position,p.name)
         if good and type(v)=="table" and v.position_known==true and v.stale~=true
            and v.position_stale~=true then
           local xyz=vec(v.position)
           if xyz then point=xyz;dim=tonumber(v.dimension) or dim;rid=v.runtime_id or rid;source="player.position" end
         end
       end
       local remote=remote_players[p.name]
       if remote and now()-remote.at<=3 then
         point=remote.p;dim=remote.dim;rid=remote.rid;source="observerBridge"
       end
       if rid and tostring(rid)~="0" then V.ids[tostring(rid)]=p.name end
       out[#out+1]={name=p.name,p=point,dim=dim,rid=rid,source=source}
     end
   end
   return out
 end
 local function snap(k,a)
   if a.observer then return end
   if not(player and type(player.position)=="function") then return end
   local ok,entry=pcall(player.position,k)
   if not ok or type(entry)~="table" or entry.known~=true then return end
   for _,e in ipairs({entry}) do
     if type(e)=="table" and tostring(e.runtime_id or "")==k then
       if type(e.entity_type)=="string" and e.entity_type~="" then a.type=e.entity_type end
       if valid(e.name) then a.name=e.name end
       if tonumber(e.dimension) then a.dim=tonumber(e.dimension) end
       -- 采用宿主绝对位置，MoveActorDelta已经由宿主累计；禁止当作delta使用。
       -- 静止实体不一定继续发移动包：只有同一runtime且仍存在时保留最后准确位置。
       if e.present~=false and e.position_known==true then
         local age=tonumber(e.position_age_ms) or 999999999
         -- 静止生物不会持续推送位移；以实体仍存在且近期生命值包再次出现为依据恢复定位。
         if age<=900000 then
           local p=vec(e.position)
           if p then
             a.p=p
             if age<=15000 then a.pat=now()-math.floor(age/1000)
             elseif a.hp_at and now()-a.hp_at<=3 then
               a.pat=now();a.pos_estimate=true
             end
           end
         end
       end
       if e.entity_type=="minecraft:player" and valid(e.name) then V.ids[k]=e.name end
       return
     end
   end
 end
 local function attrs(array)
   local h,m,arm
   if type(array)~="table" then return nil,nil,nil end
   for _,e in pairs(array) do
     if type(e)=="table" then
       local f=type(e.attribute_value)=="table" and e.attribute_value or e
       local n=f.name or f.id
       if n=="minecraft:health" or n=="health" then
         h=num(f.value or f.current);m=num(f.max or f.maximum or f.default_max)
       elseif n=="minecraft:armor" or n=="minecraft:armour" or n=="minecraft:armor_toughness" then
         if n~="minecraft:armor_toughness" then arm=num(f.value or f.current) end
       end
     end
   end
   return h,m,arm
 end
 local names={zombie="僵尸",skeleton="骷髅",enderman="末影人",creeper="苦力怕",
   spider="蜘蛛",iron_golem="铁傀儡",cow="牛",sheep="羊",pig="猪",
   chicken="鸡",warden="监守者",witch="女巫",drowned="溺尸",husk="尸壳",
   blaze="烈焰人",slime="史莱姆",villager="村民",wither="凋灵",
   ender_dragon="末影龙",arrow="箭",player="玩家"}
 local more_zh={allay="悦灵",axolotl="美西螈",bee="蜜蜂",cat="猫",wolf="狼",fox="狐狸",frog="青蛙",goat="山羊",camel="骆驼",horse="马",llama="羊驼",panda="熊猫",polar_bear="北极熊",rabbit="兔子",turtle="海龟",dolphin="海豚",squid="鱿鱼",glow_squid="发光鱿鱼",stray="流浪者",bogged="沼骸",breeze="旋风人",zombie_villager="僵尸村民",phantom="幻翼",vex="恼鬼",evoker="唤魔者",vindicator="卫道士",pillager="掠夺者",ravager="劫掠兽",guardian="守卫者",elder_guardian="远古守卫者",ghast="恶魂",magma_cube="岩浆怪",piglin="猪灵",piglin_brute="猪灵蛮兵",zombified_piglin="僵尸猪灵",hoglin="疣猪兽",zoglin="僵尸疣猪兽",strider="炽足兽",shulker="潜影贝",wither_skeleton="凋灵骷髅",snow_golem="雪傀儡",mooshroom="哞菇",sniffer="嗅探兽",armadillo="犰狳",creaking="嘎枝",trident="三叉戟"}
 for k,v in pairs(more_zh) do names[k]=v end
 local function label(a)
   local t=tostring(a.type or "未知"):gsub("^minecraft:","")
   local translated=names[t] or (t=="未知" and "生物" or t:gsub("_"," "))
   if valid(a.name) then return a.name end
   return translated
 end
 local function fast(cmd)
   if not(game and type(game.sendCommandFast)=="function") then return false end
   local ok,v=pcall(game.sendCommandFast,cmd,{channel="ai",rate_cps=0})
   if not ok then V.error=tostring(v):sub(1,150);V.counts.errors=V.counts.errors+1 end
   return ok and v~=false
 end
 local function jsontext(s)
   if http and type(http.json_encode)=="function" then
     local ok,v=pcall(http.json_encode,{rawtext={{text=s}}})
     if ok and type(v)=="string" then return v end
   end
   local t=s:gsub("\\","\\\\"):gsub('"','\\"'):gsub("\n","\\n")
   return '{"rawtext":[{"text":"'..t..'"}]}'
 end
 local function armorvalue(v)
   if type(v)=="string" then return v:lower(),false end
   if v==nil or v==false then return "",false end
   if type(v)~="table" then return "",true end
   if type(v.stack)=="table" and v.stack~=v then return armorvalue(v.stack) end
   local it=v.item_type or v.item
   local n=v.name or v.id or v.item_id or v.identifier or v.item_name or v.type_id
     or (type(it)=="table" and (it.name or it.id or it.identifier or it.item_name))
   if type(n)=="string" and n~="" then return n:lower(),false end
   local network=tonumber(v.network_id or (type(it)=="table" and it.network_id))
   if network==0 or (network==nil and next(v)==nil) then return "",false end
   return "",true
 end
 local weights={leather={1,3,2,1},gold={2,5,3,1},golden={2,5,3,1},
   chainmail={2,5,4,1},copper={2,4,3,1},iron={2,6,5,2},diamond={3,8,6,3},
   netherite={3,8,6,3},turtle={2,0,0,0}}
 function yy4_armor(ev)
   local f=data(ev);local a=entity(f.entity_runtime_id);if not a then return end
   a.armor_cache=a.armor_cache or {}
   local sum=0
   local unresolved=false
   local equipped=0
   for i,part in ipairs({"helmet","chestplate","leggings","boots"}) do
     if f[part]~=nil then a.armor_cache[part]=f[part] end
     local raw=a.armor_cache[part]
     local nested=type(raw)=="table" and (raw.stack or raw) or nil
     local ti=type(nested)=="table" and nested.item_type or nil
     local network=tonumber(type(ti)=="table" and ti.network_id or nil)
     if network and network>0 then equipped=equipped+1 end
     local s,bad=armorvalue(raw)
     if bad then unresolved=true end
     if s~="" then
       local matched=false
       for kind,w in pairs(weights) do
         if s:find(kind,1,true) then sum=sum+w[i];matched=true;break end
       end
       if not matched then unresolved=true end
     end
   end
   a.armor_slots_count=equipped
   if not unresolved then a.armor_gear=sum;a.armor_at=now()
   else a.armor_unknown_at=now() end
   if not V.armor_diag or (V.armor_diag.unresolved and V.armor_diag.events<12) then
     local sample={}
     for _,slot in ipairs({"helmet","chestplate","leggings","boots"}) do
       local item=f[slot];local label,bad=armorvalue(item)
       local fields={}
       if type(item)=="table" then
         for key,val in pairs(item) do
           if type(key)=="string" and #fields<12 then
             fields[#fields+1]=key..":"..type(val)
           end
         end
       end
       table.sort(fields)
       local nested={}
       if type(item)=="table" and type(item.stack)=="table" then
         for key,val in pairs(item.stack) do
           if type(key)=="string" and #nested<14 then
             nested[#nested+1]=key..":"..type(val)
           end
         end
       end
       table.sort(nested)
       local inner={}
       local stack=type(item)=="table" and item.stack or nil
       local it=type(stack)=="table" and stack.item_type or nil
       if type(it)=="table" then
         for key,value in pairs(it) do
           if type(key)=="string" and #inner<14 then
             inner[key]=(type(value)=="string" or type(value)=="number" or type(value)=="boolean")
                  and tostring(value):sub(1,65) or type(value)
           end
         end
       end
       sample[slot]={item_type=type(item),name=label,unknown=bad,
         fields=fields,stack_fields=nested,item_type_fields=inner,
         net_id=type(item)=="table" and item.network_id or nil}
     end
     V.armor_diag={at=now(),runtime=tostring(f.entity_runtime_id),unresolved=unresolved,
       armor_gear=a.armor_gear,slots=sample,events=(V.armor_diag and V.armor_diag.events or 0)+1}
   end
   V.counts.armor=V.counts.armor+1
 end
 function yy4_add(ev)
   local f=data(ev);local a=entity(f.entity_runtime_id);if not a then return end
   if f.entity_unique_id~=nil then
     local uid=tostring(f.entity_unique_id)
     V.unique_to_runtime[uid]=tostring(f.entity_runtime_id)
     a.unique_id=uid
   end
   if type(f.entity_type)=="string" then
     a.type=f.entity_type
     if f.entity_type~="minecraft:player" then V.ids[tostring(f.entity_runtime_id)]=nil end
   end
   if valid(f.name) then a.name=f.name end
   if tonumber(f.dimension)~=nil then a.dim=tonumber(f.dimension) end
   local p=vec(f.position)
   if p then a.p=p;a.pat=now() end
   local h,m,arm=attrs(f.attributes)
   if h~=nil then a.hp=h;a.hp_at=now() end
   if m~=nil then a.mx=m end
   if arm~=nil then a.armor_attr=arm end
   if a.type=="minecraft:arrow" or a.type=="minecraft:trident" then
     local found=nil;local n=0
     for _,pl in ipairs(online()) do
       if pl.p and p and dist(pl.p,p)<=16 then found=pl.name;n=n+1 end
     end
     if n==1 then
       V.shots[tostring(f.entity_runtime_id)]={player=found,kind=a.type,
         origin=p,at=now(),dim=a.dim}
       V.counts.arrow=V.counts.arrow+1
     end
   end
 end
 function yy4_player(ev)
   local f=data(ev)
   local n=f.username or f.name
   if f.entity_runtime_id and valid(n) then
     local k=tostring(f.entity_runtime_id)
     V.ids[k]=n
     local a=entity(k);a.type="minecraft:player";a.name=n
     local p=vec(f.position)
     if p then a.p=p;a.pat=now() end
   end
 end
 function yy5_move(ev)
   local f=data(ev)
   local rid=f.runtime_id or f.entity_runtime_id
   if rid==nil then return end
   local a=V.actors[tostring(rid)]
   if not a then return end
   local p=vec(f.position)
   if not p then return end
   a.p=p;a.pat=now()
   if tonumber(f.dimension) then a.dim=tonumber(f.dimension) end
   V.counts.move_events=V.counts.move_events+1
 end
 function yy5_delta_move(ev)
   local f=data(ev)
   local id=f.entity_runtime_id or f.runtime_id
   if id==nil then return end
   V.counts.move_delta_events=(V.counts.move_delta_events or 0)+1
   local k=tostring(id)
   local a=V.actors[k]
   if not a then return end
   -- MoveActorDelta的position不是可直接拿来覆盖的绝对世界坐标。
   -- 仅对近期受击的实体读取宿主已累计的绝对坐标快照。
   local t=now()
   if t-(a.hp_at or 0)>10 and t-(a.hurt_at or 0)>10 then return end
   local ms=type(util.monotonicMs)=="function" and tonumber(util.monotonicMs()) or t*1000
   if ms and ms-(a.last_delta_sync or -999999)>250 then
     a.last_delta_sync=ms
     snap(k,a)
     V.counts.move_delta_resynced=(V.counts.move_delta_resynced or 0)+1
   end
 end
 function yy5_remove(ev)
   local f=data(ev)
   local uid=f.entity_unique_id~=nil and tostring(f.entity_unique_id) or nil
   local k=f.entity_runtime_id and tostring(f.entity_runtime_id) or (uid and V.unique_to_runtime[uid])
   if not k then return end
   local old=V.actors[k]
   if old and old.unique_id then V.unique_to_runtime[old.unique_id]=nil end
   if uid then V.unique_to_runtime[uid]=nil end
   V.actors[k]=nil;V.ids[k]=nil;V.shots[k]=nil
   V.counts.removed=(V.counts.removed or 0)+1
 end
 local function resolve(k)
   if V.ids[k] then return V.ids[k] end
   local a=entity(k);if not a then return nil end
   snap(k,a)
   if V.ids[k] then return V.ids[k] end
   -- 不能用玩家站位直接推算Animate包的runtime归属，避免多人串号。
   -- 只有AddPlayer或game.scene明确给出玩家名才记录。
   return nil
 end
 local COMBAT_NEAR_RADIUS_SQ=5*5
 local COMBAT_REMOTE_RADIUS_SQ=48*48
 local COMBAT_INTENT_SECONDS=8
 local intent_sessions={}
 local awaiting_hit={}
 local swing_seen_ms={}
 local function mark(k,n,kind)
   if not valid(n) then return end
   -- onSwing与Animate通常来自同一个底层动作；防止重复计数。
   local key=tostring(k or n)
   local clock=type(util.monotonicMs)=="function" and tonumber(util.monotonicMs()) or nil
   local last=swing_seen_ms[key]
   if clock and last and clock-last>=0 and clock-last<35 then
     swing_diag.duplicate_callbacks=(swing_diag.duplicate_callbacks or 0)+1
     return
   end
   if clock then swing_seen_ms[key]=clock end
   V.swing[n]={at=now(),type=kind or "挥臂",rid=k}
   V.counts.swing=V.counts.swing+1
 end
 function yy4_swing(ev)
   swing_diag.native_swing_events=swing_diag.native_swing_events+1
   local f=data(ev)
   local k=f.entity_runtime_id or f.runtime_id
   local n=f.player or f.name or f.username or f.player_name
   -- onSwing归一化事件的玩家名可能位于外层，而data(ev)只返回内层。
   if not valid(n) and type(ev)=="table" then
     n=ev.player or ev.name or ev.player_name or ev.username
   end
   if not valid(n) and k then n=resolve(tostring(k)) end
   if valid(n) then
     swing_diag.mapped_native=swing_diag.mapped_native+1
     mark(k,n,"挥臂")
   else
     swing_diag.native_unmapped=swing_diag.native_unmapped+1
   end
   swing_note("onSwing",k,f.action_type,n,valid(n) and "mapped" or "unmapped")
 end
 function yy4_animate(ev)
   local f=data(ev)
   swing_diag.raw_animate=swing_diag.raw_animate+1
   local action=tonumber(f.action_type)
   local k=f.entity_runtime_id or f.runtime_id
   if action~=1 then
     swing_diag.other_actions=swing_diag.other_actions+1
     swing_note("Animate",k,action,nil,"not_arm_swing")
     return
   end
   swing_diag.action_swing=swing_diag.action_swing+1
   if k==nil then
     swing_diag.missing_runtime_id=swing_diag.missing_runtime_id+1
     swing_note("Animate",nil,action,nil,"missing_runtime_id")
     return
   end
   local n=resolve(tostring(k))
   if valid(n) then
     swing_diag.mapped_animate=swing_diag.mapped_animate+1
     mark(k,n,"挥臂")
   else
     swing_diag.unmapped_runtime_id=swing_diag.unmapped_runtime_id+1
   end
   swing_note("Animate",k,action,n,valid(n) and "mapped" or "unmapped")
 end
 local function viewers(a)
   local arr={}
   local diagnostics={}
   for _,p in ipairs(online()) do
     local ds=dist(p.p,a.p)
     diagnostics[#diagnostics+1]={n=p.name,source=p.source,position=p.p,
       dim=p.dim,distance=ds==math.huge and "unknown" or math.floor(math.sqrt(ds)*10)/10}
     if p.p and a.p and (a.dim==nil or p.dim==nil or a.dim==p.dim) then
       if ds<=COMBAT_REMOTE_RADIUS_SQ then arr[#arr+1]={name=p.name,d=ds} end
     end
   end
   V.player_diag=diagnostics
   table.sort(arr,function(x,y) return x.d<y.d end)
   while #arr>12 do table.remove(arr) end
   return arr
 end
 local function attribution(arr,a)
   local tmnow=now()
   local found={}
   for _,p in ipairs(arr) do
     local act=V.swing[p.name]
     local session=intent_sessions[p.name]
     if act and act.at<=tmnow and tmnow-act.at<=COMBAT_INTENT_SECONDS then
       found[#found+1]={n=p.name,quality="攻击前兆与受伤事件",priority=0,d=p.d,age=tmnow-act.at,action_at=act.at}
     elseif session and session.expires>=tmnow then
       found[#found+1]={n=p.name,quality="持续战斗",priority=0,d=p.d,age=tmnow-(session.last_combo or tmnow),session=true}
     end
   end
   for _,shot in pairs(V.shots) do
     if shot.player and shot.at<=tmnow and tmnow-shot.at<=COMBAT_INTENT_SECONDS
        and (not shot.dim or not a.dim or shot.dim==a.dim) then
       for _,p in ipairs(arr) do
         if p.name==shot.player then found[#found+1]={n=p.name,quality="远程投射物与受伤事件",priority=1,d=p.d,age=tmnow-shot.at,action_at=shot.at} end
       end
     end
   end
   local byname={}
   for _,it in ipairs(found) do
     if not byname[it.n] or it.age<byname[it.n].age then byname[it.n]=it end
   end
   local choices={}
   for _,it in pairs(byname) do choices[#choices+1]=it end
   table.sort(choices,function(x,y) if x.age~=y.age then return x.age<y.age end return x.d<y.d end)
   local chosen=choices[1]
   if chosen and chosen.action_at then
     local session=intent_sessions[chosen.n]
     if not session or session.expires<tmnow then session={last_action=0} end
     if chosen.action_at>(session.last_action or 0) then
       session.last_action=chosen.action_at
       session.last_combo=tmnow
       session.expires=tmnow+COMBAT_INTENT_SECONDS
       intent_sessions[chosen.n]=session
     end
   end
   return chosen
 end
 local function write_scores(n,a,damage)
   if damage~=nil then
     if fast('scoreboard players set "'..n..'" yy_hit_dmg10 '..math.floor(damage*10+0.5)) then V.counts.score=V.counts.score+1 end
   end
   if a.hp~=nil then fast('scoreboard players set "'..n..'" yy_hit_hp10 '..math.floor(a.hp*10+0.5)) end
   if a.mx~=nil then fast('scoreboard players set "'..n..'" yy_hit_max10 '..math.floor(a.mx*10+0.5)) end
   local ar=a.armor_attr or a.armor_gear
   if ar~=nil then fast('scoreboard players set "'..n..'" yy_hit_armor '..math.floor(ar+0.5)) end
   -- 独立生物积分与玩家 belowname yy_hp 不冲突。只针对受伤实体就近定位写入。
   if a.type and a.type:match("^[%w_:]+$") and a.p and now()-(a.pat or 0)<=6
     and a.hp~=nil and a.type~="minecraft:player" then
     local x,y,z=a.p.x,a.p.y,a.p.z
     local command=string.format("scoreboard players set @e[type=%s,x=%.2f,y=%.2f,z=%.2f,r=0.7,c=1] yy_mob_hp %d",a.type,x,y,z,math.floor(a.hp+0.5))
     if a.observer and a.dim~=nil then command="execute in "..({[0]="overworld",[1]="the_nether",[2]="the_end"})[a.dim].." run "..command end
     fast(command)
   end
 end
 local function overlay(n,a,damage,quality,distance)
   if not valid(n) or a.pending_only or type(a.hp)~="number" then return false,"no_health" end
   local last=V.view[n]
   if type(last)=="table" and last.rid~=a.runtime_id and now()-(last.at or 0)<2
      and type(last.distance)=="number" and type(distance)=="number"
      and distance>last.distance+0.5 then return false,"farther" end
   if type(last)=="table" and V.frame-(last.frame or -100)<2 then return false,"throttled" end
   if type(last)=="table" and last.rid==a.runtime_id and last.hp==a.hp
      and last.seq==a.hurt_seq then return false,"unchanged" end
   local entityName=label(a)
   local mx=tonumber(a.mx)
   local hp=string.format("%.1f",a.hp)
   local hptext=mx and (hp.."/"..string.format("%.0f",mx)) or hp
   local armor=a.armor_gear
   if type(armor)~="number" then armor=a.armor_attr end
   -- Exact points only; an unknown runtime item ID must never be presented as item count.
   if armor==0 and type(a.armor_slots_count)=="number" and a.armor_slots_count>0
     and a.armor_gear==nil then armor=nil end
   local deftext=type(armor)=="number" and (" §b甲§f"..math.floor(armor+0.5)) or ""
   local disttext=type(distance)=="number" and (" §7"..string.format("%.0f",math.sqrt(distance)).."格") or ""
   local harm=type(damage)=="number" and damage>0 and (" §c-"..string.format("%.1f",damage)) or ""
   local bar=""
   if mx and mx>0 then
     local fill=math.max(0,math.min(6,math.floor(a.hp/mx*6+0.5)))
     bar=" §a"..string.rep("▰",fill).."§8"..string.rep("▱",6-fill)
   end
   local line="§b⚔ §f"..entityName..disttext.." §c❤§f"..hptext..bar..deftext..harm
   local ok=false
   if game and type(game.sendActionbar)=="function" then
     local sent,result=pcall(game.sendActionbar,n,line)
     ok=(sent and result~=false)
   end
   if not ok then ok=fast('titleraw @a[name="'..n..'"] actionbar '..jsontext(line)) end
   if ok then
     V.view[n]={at=now(),frame=V.frame,hp=a.hp,distance=distance,
       rid=a.runtime_id,seq=a.hurt_seq}
     V.counts.shown=V.counts.shown+1
     V.last_title={target=n,submitted=ok,at=now(),length=#line,
       quality="r75_compact",channel="actionbar"}
     write_scores(n,a,damage)
   end
   return ok
 end
 local function emit(k,a,damage,tag)
   if not a or a.pending_only then return end
   -- Never show pending/cooldown placeholders; only actual post-damage health changes.
   if (type(damage)~="number" or damage<=0) and tag~="health_after_hurt" then
     V.counts.skipped=V.counts.skipped+1
     V.recent={reason="no_verified_health_drop",rid=k,at=now(),tag=tag}
     return
   end
   if not a.p or now()-(a.pat or 0)>0 then snap(k,a) end
   if not a.p or now()-(a.pat or 0)>90 then
     V.recent={reason="missing_fresh_entity_position",rid=k,entity=a.type,
       has_pos=a.p~=nil,pos=a.p,age=now()-(a.pat or 0),at=now(),tag=tag}
     V.counts.skipped=V.counts.skipped+1
     V.counts.stale_pos=V.counts.stale_pos+1
     return
   end
   local candidates=viewers(a)
   if #candidates==0 then
     V.recent={reason="no_players_in_48_blocks",rid=k,entity=a.type,
       entity_pos=a.p,dim=a.dim,player_samples=V.player_diag,at=now(),tag=tag}
     V.counts.skipped=V.counts.skipped+1
     V.counts.no_candidate=V.counts.no_candidate+1
     return
   end
   local matched=attribution(candidates,a)
   local allowed={}
   -- 受伤后，不依赖攻击者识别；无条件推送给4.5格内的附近玩家（至多12名）。
   for _,v in ipairs(candidates) do
    if type(v.d)=="number" and v.d<=20.25 then
     allowed[v.name]={name=v.name,quality="hurt_near_4_5",priority=0,age=0}
    end
   end
   if matched then
      allowed[matched.n]={name=matched.n,quality=matched.quality,
         priority=matched.priority,age=matched.age}
   end
   local victim=V.ids[k] or (a.type=="minecraft:player" and a.name)
   if matched and valid(victim) and victim~=matched.n and matched.age<=2
      and matched.priority<=1 then
     V.last_pvp[victim]={name=matched.n,at=now(),quality=matched.quality}
   end
   local shown_count=0
   if not matched and not awaiting_hit[k] then
     awaiting_hit[k]={at=now(),damage=damage,seq=a.hurt_seq,tag=tag}
   elseif matched then awaiting_hit[k]=nil end
   for _,viewer in pairs(allowed) do
     local ds=nil
     for _,candidate in ipairs(candidates) do
       if candidate.name==viewer.name then ds=candidate.d;break end
     end
     local ok,why=overlay(viewer.name,a,damage,viewer.quality,ds)
     if ok then
       shown_count=shown_count+1
       V.counts.matched=V.counts.matched+1
       V.pending[viewer.name]=nil
     elseif why=="throttled" or why=="locked" then
       V.pending[viewer.name]={rid=k,damage=damage,quality=viewer.quality,
         distance=ds,at=now(),priority=viewer.priority}
     end
   end
   if shown_count==0 and not matched then
     V.counts.ambiguous=V.counts.ambiguous+1
     V.counts.skipped=V.counts.skipped+1
   end
   V.recent={reason=shown_count>0 and "nearby_or_intent_display" or "no_confirmed_viewer",
     rid=k,entity=a.type,entity_pos=a.p,attacker=matched and matched.n or nil,
     nearby_auto=0,viewers_displayed=shown_count,
     candidate_count=#candidates,at=now(),tag=tag}
 end
 local function update_damage(k,h,m,previous,source)
   local a=entity(k);if not a or h==nil then return end
   local old=a.hp
   local before=num(previous) or old
   local previous_time=a.hp_at or 0
   local diff=before and before>h and before-h or nil
   a.hp=h;a.hp_at=now();a.pending_only=false
   if m and m>0 then a.mx=m end
   if diff and diff>0.001 then
     a.hurt_seq=(a.hurt_seq or 0)+1
     local precise=num(previous)~=nil or now()-previous_time<=8
     local damage=precise and diff or nil
     V.counts.health_drop=V.counts.health_drop+1
     -- 同一血量可因回血后再次受击出现，不能用血量相等推断没有新伤害。
     emit(k,a,damage,source)
     a.report_hp=h;a.report_at=now()
   elseif a.hurt_at and now()-a.hurt_at<=3 and a.report_hp~=h then
     -- 首次属性包也把可靠的新血量显示出来；没有伤前基线就不猜伤害值。
     emit(k,a,nil,"health_after_hurt")
     a.report_hp=h;a.report_at=now()
   end
 end
 function yy4_native_health(ev)
   -- 使用SDK原生生命值变化事件而非重新解码整包；初始无previous不判伤害。
   local f=data(ev)
   local k=f.runtime_id or f.entity_runtime_id
   local h=num(f.health)
   if k==nil or h==nil then return end
   k=tostring(k)
   local a=entity(k)
   if type(f.entity_type)=="string" then a.type=f.entity_type end
   if type(f.player)=="string" and valid(f.player) then V.ids[k]=f.player end
   local p=vec(f.position)
   if p then a.p=p;a.pat=now() end
   V.counts.native_health_changes=(V.counts.native_health_changes or 0)+1
   update_damage(k,h,num(f.max_health),num(f.previous),"onHealthChange")
 end
 function yy4_attrs(ev)
   local f=data(ev);if f.entity_runtime_id==nil then return end
   local k=tostring(f.entity_runtime_id);local a=entity(k)
   local h,m,arm=attrs(f.attributes)
   if arm~=nil then a.armor_attr=arm end
   V.counts.attr=V.counts.attr+1
   if h~=nil then health_packet_range(a);update_damage(k,h,m,nil,"UpdateAttributes") end
 end
 function yy4_hurt(ev)
   local f=data(ev);local k=f.runtime_id or f.entity_runtime_id
   if k==nil then return end
   k=tostring(k);local a=entity(k)
   local h,m=num(f.health),num(f.max_health)
   local p=vec(f.position)
   if p then a.p=p;a.pat=now() end
   if type(f.entity_type)=="string" then a.type=f.entity_type end
   local prev=num(f.previous)
   if h~=nil and prev and prev>h then
     update_damage(k,h,m,prev,"onHurt")
   elseif tonumber(f.event_type)==2 or h==nil then
     a.hurt_at=now()
     if a.hurt_frame~=V.frame then
       a.hurt_frame=V.frame
       a.hurt_seq=(a.hurt_seq or 0)+1
       -- ActorEvent没有可靠的伤害数值，绝不拿旧快照伪装本次伤害。
       a.pending_only=true
       emit(k,a,nil,"ActorEvent")
       a.report_at=now()
     end
   end
 end
 function yy4_event(ev)
   local f=data(ev)
   if tonumber(f.event_type)~=2 then return end
   V.counts.actor_event=V.counts.actor_event+1
   yy4_hurt({runtime_id=f.entity_runtime_id,event_type=2})
 end
 function yy5_flush()
   -- 攻击/受伤立即触发；待发送项每200ms补发，避免50ms空回调挤满128队列。
   -- 仅刷新近期受击者的待发送项，闲置时无游戏指令。
   V.frame=V.frame+1
   local t=now()
   -- A swing can arrive AFTER the first damage packet during spectator rotation.
   -- Retry only recent real hurt events, never scan all mobs.
   local pending_hurt=0
   for rid,p in pairs(awaiting_hit) do
     pending_hurt=pending_hurt+1
     local actor=V.actors[rid]
     if pending_hurt>16 or t-(p.at or 0)>2 or not actor
        or (actor.hurt_seq or 0)~=p.seq then awaiting_hit[rid]=nil
     elseif not actor.pending_only and tonumber(actor.hp) then
       local candidates=viewers(actor)
       local chosen=attribution(candidates,actor)
       if chosen and chosen.age<=COMBAT_INTENT_SECONDS then
         emit(rid,actor,p.damage,p.tag)
         awaiting_hit[rid]=nil
         V.counts.late_swing_shown=(V.counts.late_swing_shown or 0)+1
       end
     end
   end
   local count=0
   for n,p in pairs(V.pending) do
     count=count+1
     if count>12 then break end
     local a=p and V.actors[p.rid]
     local last=V.view[n]
     if not p or t-(p.at or 0)>1 or not a then
       V.pending[n]=nil
     elseif not last or
       (last.frame~=V.frame and
         (last.rid==p.rid or V.frame-(last.frame or -1000)>=10)) then
       local ok,why=overlay(n,a,p.damage,p.quality,p.distance)
       if ok then
         V.counts.queue_sent=V.counts.queue_sent+1
         V.pending[n]=nil
       elseif why=="unchanged" or why=="capacity" then
         V.pending[n]=nil
       end
     end
   end
   return true
 end
 local world_cursors={}
 local world_stats={polls=0,events=0,health=0,swing=0,stale=0,errors=0,by_account={}}
 function yy_world_combat_poll()
   if not(observerBridge and type(observerBridge.worldEvents)=="function") then return false end
   local ok,result=pcall(observerBridge.worldEvents,{stream="combat",cursors=world_cursors,limit=512})
   world_stats.polls=world_stats.polls+1
   if not ok or type(result)~="table" then world_stats.errors=world_stats.errors+1;world_stats.last_error=tostring(result);return false end
   world_cursors=result.cursors or world_cursors
   world_stats.partial=result.complete==false
   for _,ev in ipairs(result.events or {}) do
     local observer=ev.observer
     if type(observer)=="table" and observer.account~="main" and observer.connected==true then
       local age=now()*1000-(tonumber(ev.timestamp) or 0)
       if age>=-1000 and age<=3000 then
         local raw=ev.runtime_id
         if raw~=nil and tonumber(ev.dimension)~=nil then
           local k="world:"..tostring(observer.account)..":"..tostring(observer.session)..":"..tostring(raw)
           local a=entity(k);a.observer=observer;a.dim=tonumber(ev.dimension)
           local point=vec(ev.position)
           if point and ev.position_stale~=true then a.p=point;a.pat=now() end
           if type(ev.entity_type)=="string" then a.type=ev.entity_type end
           local name=ev.player
           if valid(name) then
             V.ids[k]=name;a.name=name
             if point and ev.position_stale~=true then remote_players[name]={p=point,dim=a.dim,rid=k,at=now()} end
           end
           world_stats.events=world_stats.events+1
           world_stats.by_account[observer.account]=(world_stats.by_account[observer.account] or 0)+1
           if ev.type=="swing" then
             yy4_swing({runtime_id=k,player=name});world_stats.swing=world_stats.swing+1
           elseif ev.type=="health" then
             local local_actor=V.actors[tostring(raw)]
             if not(local_actor and local_actor.dim==a.dim and local_actor.hp==tonumber(ev.health) and now()-(local_actor.hp_at or 0)<=1) then
               yy4_native_health({runtime_id=k,health=ev.health,previous=ev.previous,max_health=ev.max_health,position=point,entity_type=ev.entity_type,player=name})
             end
             world_stats.health=world_stats.health+1
           elseif ev.type=="hurt" and ev.health==nil then
             yy4_hurt({runtime_id=k,event_type=2,position=point,entity_type=ev.entity_type})
           end
         end
       else world_stats.stale=world_stats.stale+1 end
     end
   end
   return true
 end
 function yy4_armor_poll()
  local check=0;local updated=0
  local pool={}
  for k,a in pairs(V.actors) do
    if a and now()-math.max(a.hurt_at or 0,a.hp_at or 0)<30 then
      pool[#pool+1]={id=k,a=a,ts=math.max(a.hurt_at or 0,a.hp_at or 0)}
    end
  end
  table.sort(pool,function(x,y)return x.ts>y.ts end)
  for _,it in ipairs(pool) do
   local k,a=it.id,it.a
   if check>=8 then break end
   if a and now()-(a.hp_at or 0)<25 then
    check=check+1
    local ok,r=false,nil
    local who=valid(a.name) and a.name or resolve(k)
    if valid(who) and not V.player_equipment_probe then
      local okd,rec=pcall(player.equipment,who)
      local fields={}
      if okd and type(rec)=="table" then
        for key,value in pairs(rec) do if type(key)=="string" and #fields<28 then
          fields[#fields+1]=key..":"..type(value)
        end end
      end
      table.sort(fields)
      V.player_equipment_probe={at=now(),name=who,
        success=okd,result_type=type(rec),keys=fields}
      pcall(util.json_write,game.dataDir().."/yy_armor_equipment_probe_r76.json",V.player_equipment_probe)
    end
    if valid(who) and type(player)=="table" and type(player.equipment)=="function" then
      ok,r=pcall(player.equipment,who)
    end
    if ok and type(r)=="table" and not V.equipment_probe then
      local info={}
      local inspect=r.equipment or r.armor or r.armour or r
      if type(inspect)=="table" then
        for itemname,value in pairs(inspect) do
          if type(itemname)=="string" and #info<18 then
            local v=type(value)=="table" and (value.name or value.id or value.item_name
              or (type(value.item_type)=="table" and value.item_type.name)) or value
            info[#info+1]={key=itemname,type=type(value),
              name=type(v)=="string" and v:sub(1,55) or nil}
          end
        end
      end
      V.equipment_probe={at=now(),fields=info,kind="player.equipment",success=true}
      pcall(util.json_write,game.dataDir().."/yy_equipment_r76_probe.json",V.equipment_probe)
    end
    if (not ok or type(r)~="table") and
      type(entities)=="table" and type(entities.get)=="function" then
      ok,r=pcall(entities.get,k)
    end
    if ok and type(r)=="table" then
      local e=r.equipment or r.armor or r.armour or r
      local slots=type(e)=="table" and (e.slots or e.armor_slots or e) or nil
      if type(slots)=="table" then
        local head=slots.helmet or slots.head or slots["armor.head"]
        local chest=slots.chestplate or slots.chest or slots["armor.chest"]
        local legs=slots.leggings or slots.legs or slots["armor.legs"]
        local feet=slots.boots or slots.feet or slots["armor.feet"]
        if head~=nil and chest~=nil and legs~=nil and feet~=nil then
          yy4_armor({entity_runtime_id=k,helmet=head,chestplate=chest,
            leggings=legs,boots=feet})
          updated=updated+1
        end
      end
    end
   end
  end
  V.armor_poll={at=now(),checked=check,updated=updated,interval=15}
  return true
 end
 function yy4_cleanup()
   local t=now()
   swing_seen_ms={}

   -- 每10秒局部统计一次机器人已收到的实体范围；不额外发送游戏命令。
   local ok,ps=pcall(player.list)
   if ok and type(ps)=="table" then
     for _,p in pairs(ps) do
       if type(p)=="table" and p.name=="布汲岛" then
         local pos=vec(p.position) or vec(p)
         if pos then V.botpos=pos;V.botpos_at=t end
         break
       end
     end
   end
   local visible={unknown=0,r0_32=0,r32_64=0,r64_96=0,r96_128=0,r128_plus=0}
   local sample=nil
   if type(game.scene)=="function" then
     local yes,scene=pcall(game.scene,{present_only=true})
     if yes and type(scene)=="table" and type(scene.actors)=="table" then
       local n=0
       for _,e in pairs(scene.actors) do
         n=n+1
         if n>10000 then break end
         if type(e)=="table" and e.present~=false and e.stale~=true then
           if not sample then sample={entity=e.entity_type,known=e.position_known,age_ms=e.position_age_ms,
             position=e.position,position_stale=e.position_stale,stale=e.stale} end
           local p=vec(e.position)
           local d=(p and V.botpos) and math.sqrt(dist(p,V.botpos)) or nil
           local key=not d and "unknown" or
             (d<=32 and "r0_32" or (d<=64 and "r32_64" or
             (d<=96 and "r64_96" or (d<=128 and "r96_128" or "r128_plus"))))
           visible[key]=visible[key]+1
         end
       end
     end
   end
   V.scene_ranges=visible
   V.scene_sample=sample
   for k,a in pairs(V.actors) do if t-(a.used or t)>120 then V.actors[k]=nil end end
   for k,v in pairs(V.shots) do if t-v.at>COMBAT_INTENT_SECONDS then V.shots[k]=nil end end
   for n,v in pairs(V.swing) do if t-v.at>COMBAT_INTENT_SECONDS then V.swing[n]=nil end end
   for n,v in pairs(V.last_pvp) do if t-v.at>7 then V.last_pvp[n]=nil end end
   for n,v in pairs(V.pending) do if t-(v.at or 0)>5 then V.pending[n]=nil end end
   for n,v in pairs(V.view) do
     if type(v)~="table" or t-(v.at or 0)>20 then V.view[n]=nil end
   end
   for n,v in pairs(intent_sessions) do if v.expires<t then intent_sessions[n]=nil end end
   pcall(util.json_write,game.dataDir().."/yy_combat_v4_status.json",
     {at=t,version="V6.2-native-actionbar-exact-armor48",mode="event_driven",maximum_range=48,near_auto_radius=5,max_players=12,
       pending_retry_seconds=0.2,attack_window_seconds=8,attack_window_renewable=true,target_lock_seconds=0.5,channels="native_actionbar_compact_hp_exact_armor_distance",
       display_policy="hurt+attack_in_8s;closest_target_no_pending;range48",
       rightscreen="simulated_right_shift_actionbar",
       counts=V.counts,armor_diag=V.armor_diag,armor_poll=V.armor_poll,last_error=V.error,last_event=V.recent,last_title=V.last_title,
       swing_diagnostics=swing_diag,
       sync_diagnostics={bot_position=V.botpos,bot_position_at=V.botpos_at,
         health_attribute_packets_by_bot_distance=V.packet_ranges,
         actors_currently_synced_by_bot_distance=V.scene_ranges,
         scene_sample=V.scene_sample},
       limit="其他玩家挥臂及射击协议包不保证送达机器人；来源有推断等级"})
   return true
 end
 function M.apply()
   if V.installed then return true end
   V.installed=true
   local cmds={
    'scoreboard objectives add yy_hit_dmg10 dummy "§c伤害x10"',
    'scoreboard objectives add yy_hit_hp10 dummy "§c目标血量x10"',
    'scoreboard objectives add yy_hit_max10 dummy "§c最大血量x10"',
    'scoreboard objectives add yy_hit_armor dummy "§b目标护甲"',
   }
   for _,cmd in ipairs(cmds) do fast(cmd) end
   if events and type(events.subscribe)=="function" then
     for kind,handler in pairs({
       AddActor="yy4_add",AddPlayer="yy4_player",UpdateAttributes="yy4_attrs",
       MobArmourEquipment="yy4_armor",ActorEvent="yy4_event",
       Animate="yy4_animate",RemoveActor="yy5_remove",
     }) do
       local ok,res
       if kind=="MoveActorDelta" then
         if type(events.subscribeFiltered)=="function" then
           ok,res=pcall(events.subscribeFiltered,kind,{every_ms=250},handler)
         else
           ok,res=false,"MoveActorDelta disabled: filtered subscription unavailable"
         end
       else
         ok,res=pcall(events.subscribe,kind,handler)
       end
       if ok and res~=false then
         V.counts.subscribed=(V.counts.subscribed or 0)+1
       else V.counts.errors=V.counts.errors+1;V.error="subscribe "..kind.." "..tostring(res) end
     end
   end
   if events and type(events.onHealthChange)=="function" then pcall(events.onHealthChange,"yy4_native_health") end
    if events and type(events.onHurt)=="function" then pcall(events.onHurt,"yy4_hurt") end
   if events and type(events.onSwing)=="function" then
     local ok,res=pcall(events.onSwing,"yy4_swing")
     swing_diag.native_hook_registered=ok and res~=false
   else swing_diag.native_hook_registered=false end
   -- 位置使用宿主缓存game.scene，伤害发生才按RuntimeID查询；避免高频移动包双重回调。
   if util and type(util.scheduleSeconds)=="function" then pcall(util.scheduleSeconds,15,"yy4_armor_poll") end
   if util and type(util.scheduleTick)=="function" then
     pcall(util.scheduleTick,4,"yy5_flush")
   elseif util and type(util.scheduleSeconds)=="function" then
     pcall(util.scheduleSeconds,1,"yy5_flush")
   end
   if util and type(util.scheduleSeconds)=="function" then
     pcall(util.scheduleSeconds,10,"yy4_cleanup")
   end
   if util and type(util.scheduleSeconds)=="function" then pcall(util.scheduleSeconds,0.2,"yy_world_combat_poll") end
   yy4_cleanup()
   pcall(function()
     local out={}
     for _,p in ipairs(online()) do
       out[#out+1]={name=p.name,position=p.p,dimension=p.dim,source=p.source}
     end
     util.json_write(game.dataDir()..'/yy_combat_v4_position_probe.json',{at=now(),players=out})
   end)
   -- 游戏内管理员可快速查看原始包、动作过滤和名字映射三个阶段。
   local previous_chat=on_chat
   if type(previous_chat)=="function" then
     on_chat=function(name,msg)
       if tostring(msg or "")==".挥臂诊断" then
         local allowed=(name=="疣螈")
         if type(is_admin)=="function" then
           local ok,yes=pcall(is_admin,name)
           if ok and yes then allowed=true end
         end
         if not allowed then
           if type(send)=="function" then pcall(send,name,"§c仅管理权限可查看。") end
           return true
         end
         local d=swing_diag
         local last=d.recent[#d.recent]
         local detail=last and ("\n§7最近: "..tostring(last.source).." ID="..tostring(last.rid or "?")..
           " 玩家="..tostring(last.name or "未映射").." 结果="..tostring(last.reason)) or ""
         if type(send)=="function" then pcall(send,name,
           "§b挥臂诊断 §7(本次插件启动以来)"..
           "\n§7Animate(动作包): §f"..d.raw_animate.." §7挥臂类型: §f"..d.action_swing..
           "\n§7其他动作过滤: §f"..d.other_actions.." §7缺少运行ID: §f"..d.missing_runtime_id..
           "\n§7原始已映射: §a"..d.mapped_animate.." §7原始未映射: §e"..d.unmapped_runtime_id..
           "\n§7onSwing(宿主挥臂事件): §f"..d.native_swing_events..
           " §7已识别: §a"..d.mapped_native.." §7未识别: §e"..d.native_unmapped..
           "\n§7宿主回调挂载: "..(d.native_hook_registered and "§a成功" or "§c失败")..
           detail)
         end
         return true
       end
       return previous_chat(name,msg)
     end
   end
   plugin_api=plugin_api or {}
   plugin_api.combatV4Status=function() return V.counts end
   plugin_api.worldCombatStatus=function() return world_stats end
   plugin_api.recentPvPAttacker=function(victim)
     local p=V.last_pvp[tostring(victim or "")]
     if type(p)=="table" and now()-p.at<=5 then
       return {name=p.name,age=now()-p.at,quality=p.quality}
     end
     return nil
   end
   return true
 end
end

return M