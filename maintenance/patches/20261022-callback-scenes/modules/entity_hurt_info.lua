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
       if rid and tostring(rid)~="0" then V.ids[tostring(rid)]=p.name end
       out[#out+1]={name=p.name,p=point,dim=dim,rid=rid,source=source}
     end
   end
   return out
 end
 local function snap(k,a)
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
 local function label(a)
   local t=tostring(a.type or "未知"):gsub("^minecraft:","")
   local translated=names[t] or t:gsub("_"," ")
   if valid(a.name) then return a.name.." ("..translated..")" end
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
   if type(v)~="table" then return "",true end
   local it=v.item_type
   local n=v.name or v.id or (type(it)=="table" and (it.name or it.id))
   if type(n)=="string" and n~="" then return n:lower(),false end
   local network=tonumber(v.network_id or (type(it)=="table" and it.network_id))
   if network==0 or (network==nil and next(v)==nil) then return "",false end
   return "",true
 end
 local weights={leather={1,3,2,1},gold={2,5,3,1},golden={2,5,3,1},
   chainmail={2,5,4,1},iron={2,6,5,2},diamond={3,8,6,3},
   netherite={3,8,6,3},turtle={2,0,0,0}}
 function yy4_armor(ev)
   local f=data(ev);local a=entity(f.entity_runtime_id);if not a then return end
   local sum=0
   local unresolved=false
   for i,part in ipairs({"helmet","chestplate","leggings","boots"}) do
     local s,bad=armorvalue(f[part])
     if bad then unresolved=true end
     if s~="" then
       local matched=false
       for kind,w in pairs(weights) do
         if s:find(kind,1,true) then sum=sum+w[i];matched=true;break end
       end
       if not matched then unresolved=true end
     end
   end
   if not unresolved then a.armor_gear=sum;a.armor_at=now() end
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
   local p=vec(f.position)
   if p then a.p=p;a.pat=now() end
   local h,m,arm=attrs(f.attributes)
   if h~=nil then a.hp=h;a.hp_at=now() end
   if m~=nil then a.mx=m end
   if arm~=nil then a.armor_attr=arm end
   if a.type=="minecraft:arrow" or a.type=="minecraft:snowball" or a.type=="minecraft:trident" then
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
       if ds<=225 then arr[#arr+1]={name=p.name,d=ds} end
     end
   end
   V.player_diag=diagnostics
   table.sort(arr,function(x,y) return x.d<y.d end)
   while #arr>12 do table.remove(arr) end
   return arr
 end
 local function attribution(arr)
   local possible={}
   for _,p in ipairs(arr) do
     local act=V.swing[p.name]
     if act and act.at<=now() and now()-act.at<=3 and p.d<=225 then
       possible[#possible+1]={n=p.name,quality="挥臂关联（非确证）",priority=0,d=p.d}
     end
   end
   if #possible>0 then
     table.sort(possible,function(x,y)return x.d<y.d end)
     if #possible==1 or possible[1].d+1<possible[2].d then return possible[1] end
     return nil
   end
   local shots={}
   for _,v in pairs(V.shots) do
     if v.at<=now() and now()-v.at<=3 then
       for _,p in ipairs(arr) do
         if p.name==v.player and p.d<=225 then
           shots[#shots+1]={n=p.name,quality="疑似远程投射物",priority=1,d=p.d}
         end
       end
     end
   end
   if #shots==1 then return shots[1] end
   -- 机器人可能收不到其他玩家 Animate：单人近战仅可标记“疑似”。
   local close={}
   for _,p in ipairs(arr) do if p.d<=144 then close[#close+1]=p end end
   -- 不再根据距离单独猜测攻击者
   return nil
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
     fast(string.format("scoreboard players set @e[type=%s,x=%.2f,y=%.2f,z=%.2f,r=0.7,c=1] yy_mob_hp %d",
       a.type,x,y,z,math.floor(a.hp+0.5)))
   end
 end
 local function overlay(n,a,damage,quality)
   if not valid(n) then return false end
   local last=V.view[n]
   -- 每个50ms宿主Tick只提交同一玩家一组画面，跨Tick的新血量立即允许更新。
   -- 只在同一Tick重复写时合并；绝不按整秒时间戳限制到1秒。
   if type(last)=="table" and last.rid~=a.runtime_id
      and V.frame-(last.frame or -1000)<4 then
     V.counts.locked=V.counts.locked+1
     return false,"locked"
   end
   if type(last)=="table" and V.frame-(last.frame or -1000)<2 then
     V.counts.throttled=V.counts.throttled+1
     return false,"throttled"
   end
   if type(last)=="table" and last.rid==a.runtime_id
      and last.hp==a.hp and last.damage==damage
      and last.pending==a.pending_only and last.quality==quality and last.seq==a.hurt_seq then
     V.counts.duplicate=V.counts.duplicate+1
     return false,"unchanged"
   end
   local recent=0
   for _,v in pairs(V.view) do
     if type(v)=="table" and now()-(v.at or 0)<5 then recent=recent+1 end
   end
   if (type(last)~="table" or now()-(last.at or 0)>5) and recent>=12 then
     return false,"capacity"
   end
   local h=(not a.pending_only and a.hp~=nil) and string.format("%.1f",a.hp) or "待本次同步"
   local mx=a.mx~=nil and string.format("%.1f",a.mx) or "未知"
   local harm=damage~=nil and string.format("%.1f",damage) or "未知"
   -- 受击即时伤害弹字（屏幕Title）+持续状态（Actionbar）
   local name=label(a)
   if #name>63 then name=name:sub(1,63) end
   local target='@a[name="'..n..'"]'
   local ar=a.armor_attr or a.armor_gear
   local armor=ar~=nil and (" §b护甲§f"..math.floor(ar+0.5)) or ""
   local bar=""
   if a.hp~=nil and a.mx and a.mx>0 then
     local fill=math.max(0,math.min(10,math.floor(a.hp/a.mx*10+0.5)))
     bar=" §a"..string.rep("▮",fill).."§8"..string.rep("▯",10-fill)
   end
   local line=(quality=="nearby_observer" and "§7附近受击 §b" or "§b")..name.." §8│ §c❤ §f"..h.."§8/"..mx..bar..armor
   if damage~=nil and damage>0 then line=line.." §c本次-"..harm end
   if damage~=nil and damage>0 then
     -- 只向15格内、近期有攻击动作的匹配玩家展示；伤害必须已证实。
     fast("titleraw "..target.." times 0 8 4")
     fast("titleraw "..target.." title "..jsontext("§c-"..harm))
   end
   local good=fast("titleraw "..target.." actionbar "..jsontext(line))
   V.last_title={target=n,submitted=good,at=now(),length=#line,quality="combat_health_only",channel="actionbar"}
   if good then
     V.view[n]={at=now(),frame=V.frame,hp=a.hp,damage=damage,
       pending=a.pending_only,rid=a.runtime_id,quality=quality,seq=a.hurt_seq}
     V.counts.shown=V.counts.shown+1
     if not a.pending_only and quality~="nearby_observer" then write_scores(n,a,damage) end
   end
   return good
 end
 local function emit(k,a,damage,tag)
   if not a or a.pending_only then return end
   -- Only verified health decrease plus recent attack action can display combat health.
   if type(damage)~="number" or damage<=0 then
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
   local candidates=viewers(a) -- at most 15 blocks from damaged entity
   if #candidates==0 then
     V.recent={reason="no_players_in_15_blocks",rid=k,entity=a.type,
       entity_pos=a.p,dim=a.dim,player_samples=V.player_diag,at=now(),tag=tag}
     V.counts.skipped=V.counts.skipped+1
     V.counts.no_candidate=V.counts.no_candidate+1
     return
   end
   -- Being an observer alone is never evidence of an attack.
   local matched=attribution(candidates)
   if not matched then
     V.counts.ambiguous=V.counts.ambiguous+1
     V.counts.skipped=V.counts.skipped+1
     V.recent={reason="no_recent_attack_action",rid=k,entity=a.type,
       entity_pos=a.p,near=#candidates,at=now(),tag=tag}
     return
   end
   local victim=V.ids[k] or (a.type=="minecraft:player" and a.name)
   if valid(victim) and victim~=matched.n and matched.priority<=1 then
     V.last_pvp[victim]={name=matched.n,at=now(),quality=matched.quality}
   end
   local ok,why=overlay(matched.n,a,damage,matched.quality)
   if ok then
     if matched.priority==0 then V.counts.matched=V.counts.matched+1
     else V.counts.suggested=V.counts.suggested+1 end
     V.pending[matched.n]=nil
   elseif why=="throttled" or why=="locked" then
     V.pending[matched.n]={rid=k,damage=damage,quality=matched.quality,at=now(),priority=matched.priority}
     V.counts.queued=V.counts.queued+1
   end
   V.recent={reason=ok and "attack_verified_within_15" or tostring(why or "display_not_submitted"),
     rid=k,entity=a.type,entity_pos=a.p,attacker=matched.n,
     near=#candidates,at=now(),tag=tag,viewers_displayed=ok and 1 or 0}
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
       local ok,why=overlay(n,a,p.damage,p.quality)
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
   for k,v in pairs(V.shots) do if t-v.at>8 then V.shots[k]=nil end end
   for n,v in pairs(V.swing) do if t-v.at>5 then V.swing[n]=nil end end
   for n,v in pairs(V.last_pvp) do if t-v.at>7 then V.last_pvp[n]=nil end end
   for n,v in pairs(V.pending) do if t-(v.at or 0)>5 then V.pending[n]=nil end end
   for n,v in pairs(V.view) do
     if type(v)~="table" or t-(v.at or 0)>20 then V.view[n]=nil end
   end
   pcall(util.json_write,game.dataDir().."/yy_combat_v4_status.json",
     {at=t,version="V5.6-attack-gated-15",mode="event_driven",maximum_range=15,max_players=12,
       pending_retry_seconds=0.2,attack_window_seconds=3,target_lock_seconds=0.5,channels="title_damage_plus_actionbar_health",
       display_policy="15_blocks+verified_damage+attack_before_hurt_within_3_seconds",
       rightscreen="simulated_right_shift_actionbar",
       counts=V.counts,last_error=V.error,last_event=V.recent,last_title=V.last_title,
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
   if util and type(util.scheduleTick)=="function" then
     pcall(util.scheduleTick,4,"yy5_flush")
   elseif util and type(util.scheduleSeconds)=="function" then
     pcall(util.scheduleSeconds,1,"yy5_flush")
   end
   if util and type(util.scheduleSeconds)=="function" then
     pcall(util.scheduleSeconds,10,"yy4_cleanup")
   end
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