-- YY anti-duplication monitor: inventory deltas, conservative chunk-border observations.
local M={}
local started=false
local LAST,AT={},{}
local cfg={inventory_interval=30,rare_gain=32,normal_gain=128,max_gap=600}
local status_path=game.dataDir().."/yy_ac_inventory_status.json"
local events_path=game.dataDir().."/yy_ac_inventory_alerts.json"
local scans,query_errors,alerts,edges,edge_unknown=0,0,0,0,0
local recent_border={}
local clock=function()return tonumber(util.timestamp()) or os.time()end
local function admin(n)
 if tostring(n)=="疣螈" then return true end
 local ok,op=pcall(player.isOP,n)
 if ok and op==true then return true end
 local good,r=pcall(is_admin,n)
 return good and r==true
end
local function append(item)
 local ok,lines=pcall(util.json_load,events_path)
 if not ok or type(lines)~="table" then lines={} end
 lines[#lines+1]=item
 while #lines>80 do table.remove(lines,1) end
 pcall(util.json_write,events_path,lines)
end
local function warn(message)
 if type(send)=="function" then pcall(send,"疣螈",message) end
end
local names={"diamond","netherite","ancient_debris","elytra","totem_of_undying",
 "enchanted_golden_apple","nether_star","beacon","shulker_shell"}
local function limit(item)
 for _,v in ipairs(names) do if item:find(v,1,true) then return cfg.rare_gain end end
 return cfg.normal_gain
end
local function tally(result)
 if type(result)~="table" or type(result.items)~="table" then return nil end
 local t={}
 for _,x in pairs(result.items) do
  if type(x)=="table" then
   local id=tostring(x.name or x.item_name or x.item_id or ""):lower()
   local qty=tonumber(x.count or x.amount or 0) or 0
   if id~="" and qty>0 and qty<=2000000 then t[id]=(t[id] or 0)+qty end
  end
 end
 return t
end
local function check(n,force)
 if type(n)~="string" or n=="" then return false end
 if not force and admin(n) then return false end
 if not(player and type(player.getInventory)=="function") then return false end
 local ok,r=pcall(game.queryInventory,n)
 local items=ok and tally(r) or nil
 if not items then query_errors=query_errors+1;return false end
 scans=scans+1
 local t=clock()
 local old=LAST[n]
 if type(old)=="table" and t-old.at<=cfg.max_gap then
  for item,count in pairs(items) do
   local earlier=tonumber(old.items[item]) or 0
   local delta=count-earlier
   if delta>=limit(item) then
    alerts=alerts+1
    append({at=t,player=n,item=item,before=earlier,after=count,gain=delta,
       interval_seconds=t-old.at,threshold=limit(item),status="requires_review"})
    warn("§e反作弊物资异常：玩家 "..n.." 的 "..item.." 增加 "..delta.."，请核查来源。")
   end
  end
 end
 LAST[n]={at=t,items=items};AT[n]=t
 return true
end
function yy_ac_inventory_check()
 if not started then return end
 local ok,list=pcall(player.list)
 if not ok or type(list)~="table" then return end
 local chosen=nil;local t0=math.huge
 for _,v in pairs(list) do
  if type(v)=="table" and type(v.name)=="string" and not admin(v.name) then
   local seen=AT[v.name] or 0
   if seen<t0 then chosen=v.name;t0=seen end
  end
 end
 if chosen then check(chosen,false) end
end
function yy_ac_inventory_block(ev)
 local f=type(ev)=="table" and (ev.fields or ev.data or ev) or {}
 local p=f.position
 if type(p)~="table" then return end
 local x,y,z=tonumber(p.x or p[1]),tonumber(p.y or p[2]),tonumber(p.z or p[3])
 if not x or not z then return end
 x=math.floor(x);z=math.floor(z)
 local a=x%16;local b=z%16
 if a>1 and a<14 and b>1 and b<14 then return end
 edges=edges+1
 local prev=f.previous
 if prev==nil or f.cause_known~=true then edge_unknown=edge_unknown+1 end
 local key=math.floor(x/16)..","..math.floor(z/16)
 if clock()-(recent_border[key] or 0)>=120 then
  recent_border[key]=clock()
  -- Unknown builder and unknown block name: log only, never delete a chest.
  append({at=clock(),type="chunk_border_change",chunk=key,
    builder_known=f.cause_known==true,previous_known=prev~=nil})
 end
end
function yy_ac_inventory_status_tick()
 if not started then return end
 local player_count=0;for _ in pairs(LAST)do player_count=player_count+1 end
 pcall(util.json_write,status_path,{at=clock(),loaded=true,
   mode="snapshot_comparison_one_player_per_30s",players_with_baseline=player_count,
   scans=scans,errors=query_errors,suspicious_deltas=alerts,
   edge_events=edges,edge_without_attribution=edge_unknown,
   rare_threshold=cfg.rare_gain,ordinary_threshold=cfg.normal_gain,
   auto_punishment=false,edge_auto_delete=false})
end
function M.apply()
 if started then return true end
 started=true
 if events and type(events.onBlockChange)=="function" then
  pcall(events.onBlockChange,"yy_ac_inventory_block")
 end
 if util and type(util.scheduleSeconds)=="function" then
  pcall(util.scheduleSeconds,cfg.inventory_interval,"yy_ac_inventory_check")
  pcall(util.scheduleSeconds,30,"yy_ac_inventory_status_tick")
 end
 yy_ac_inventory_status_tick()
 plugin_api=plugin_api or {}
 plugin_api.acInventoryStatus=function()
  return {scans=scans,alerts=alerts,edge_events=edges}
 end
 return true
end
return M