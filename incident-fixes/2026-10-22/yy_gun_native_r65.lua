-- R65: native game command chain for gun cooldown/reload HUD and hook particles.
-- Only state changes use Lua. No Lua player polling for HUD or hook particles.
local M={};local G=require("modules.yy_gun_data")
local R=require("modules.yy_gun_runtime")
local chain="yy_rodgun_native_r65.txt"
local installed=false
local status={installed=false,chain=chain,legacy_damage_event_retained=true,
 native_hud_hz=10,native_hook_fx_hz=5,requires_mainhand_rod=true,requires_inventory_certificate=true,score_writes=0,errors={},no_lua_hud_tick=true}
local ammo={"恶魂之泪","海晶碎片","烈焰粉","回响碎片"}
local function integer(n) return math.max(0,math.min(99999,math.floor(tonumber(n) or 0))) end
local function valid(n) return type(n)=="string" and #n>0 and #n<=60 and not n:find('[%c"\\]') end
local function cmd(t)
 local ok,v=pcall(game.sendCommandFast,t,{channel="ai",rate_cps=0})
 if not ok or v==false then
  if #status.errors<12 then status.errors[#status.errors+1]=tostring(v) end
  return false
 end
 status.score_writes=status.score_writes+1
 return true
end
local function objective(p,k,v)
 return cmd('scoreboard players set @a[name="'..p..'"] '..k..' '..integer(v))
end
local function sync(p,s)
 if not valid(p) then return false end
 s=s or G.state(p)
 local w=G.get(s.gun)
 if type(w)~="table" then return false end
 local armed=G.enabled() and s.hud~="关闭" and tonumber(s.gun) or 0
 local a=type(s.ammo)=="table" and s.ammo[tostring(s.gun)] or {}
 local mag=type(a)=="table" and a.mag or 0
 -- Do NOT recreate cooldown each tick: this is called only on state mutation.
 local time=G.time()
 local rem=math.max(0,(tonumber(s.nextFire) or 0)-time)
 local reload=math.max(0,(tonumber(s.reloadAt) or 0)-time)
 objective(p,"yy65_type",armed)
 objective(p,"yy65_mag",mag)
 objective(p,"yy65_max",w.mag)
 objective(p,"yy65_cd",math.ceil(rem/100))
 objective(p,"yy65_reload",math.ceil(reload/100))
 if armed>0 then
  cmd('tag @a[name="'..p..'"] add yy65_armed')
 else
  cmd('tag @a[name="'..p..'"] remove yy65_armed')
 end
 return true
end
-- Players already online (after restart) and later join events only.
-- No periodic Lua player list; native hasitem selectors own continuous detection.
local function syncOnlineOnce()
 local ok,list=pcall(player.list)
 if not ok or type(list)~="table" then return 0 end
 local count=0
 for _,item in ipairs(list) do
  local name=type(item)=="table" and item.name or item
  if valid(name) and (type(item)~="table" or item.online~=false) then
   pcall(sync,name,G.state(name))
   count=count+1
  end
 end
 return count
end
function yy_r65_gun_native_join(ev)
 local name=ev
 if type(ev)=="table" then name=ev.name or ev.player_name or ev.player end
 if valid(name) then pcall(sync,name,G.state(name)) end
end
local function build()
 local lines={"1,0,0"}
 for _,name in ipairs({"yy65_type","yy65_mag","yy65_max","yy65_cd","yy65_reload"}) do
  lines[#lines+1]="/scoreboard objectives add "..name.." dummy"
 end
 lines[#lines+1]="."
 lines[#lines+1]="2,0,0 10"
 lines[#lines+1]="/scoreboard players remove @a[scores={yy65_cd=1..}] yy65_cd 1"
 lines[#lines+1]="0,0,0"
 lines[#lines+1]="/scoreboard players remove @a[scores={yy65_reload=1..}] yy65_reload 1"
 local catalog=G.all()
 local certificates={"leather_horse_armor","iron_horse_armor","golden_horse_armor","diamond_horse_armor"}
 for i,w in ipairs(catalog) do
  local color=i%2==0 and "§b" or "§e"
  local msg={rawtext={
   {text="§8[§6▄︻§7┳§6═一§8] "..color..tostring(w.name).." §8┃ §f弹药 §e"},
   {score={name="@s",objective="yy65_mag"}},
   {text="§7/"},{score={name="@s",objective="yy65_max"}},
   {text=" §8┃ §6开火CD "},{score={name="@s",objective="yy65_cd"}},
   {text="§7×0.1s §8┃ §b装弹 "},{score={name="@s",objective="yy65_reload"}},
   {text="§7×0.1s"},
   {text="\n§8✦ §f伤害 §c"..tostring(w.damage or 0).." §8┃ §f射程 §b"..tostring(w.range or 0)..
    " §8┃ §7弹药 "..(ammo[tonumber(w.physical_id) or (((i-1)%4)+1)] or "按枪械模板").." §8┃ §aCD=0可射击"}
  }}
  lines[#lines+1]="0,0,0"
  local group=math.floor(tonumber(w.physical_id) or math.min(4,i))
  if group<1 or group>4 then group=1 end
  -- Native executor checks both selected gun and mainhand slot; certificate in inventory.
  -- No Lua polling or per-player inventory query.
  lines[#lines+1]='/execute as @a[scores={yy65_type='..i..'},hasitem={item=fishing_rod,location=slot.weapon.mainhand}] if entity @s[hasitem={item='..certificates[group]..'}] run titleraw @s actionbar '..http.json_encode(msg)
 end
 -- Native hook processing for *visual effects* only; no entity packet polling.
 -- Actual damage stays on the pre-existing event callback until native owner attribution is proven.
 lines[#lines+1]="."
 lines[#lines+1]="2,0,0 5"
 local candidate="@p[r=3,tag=yy65_armed,hasitem={item=fishing_rod,location=slot.weapon.mainhand}]"
 lines[#lines+1]="/execute as @e[type=fishing_hook,tag=!yy65_fx_seen] at @s if entity "..candidate.." run tag @s add yy65_fx_pending"
 for _,distance in ipairs({2,4,6,8,10,12,16,20,24,28,32,36}) do
  lines[#lines+1]="0,0,0"
  lines[#lines+1]="/execute as @e[type=fishing_hook,tag=yy65_fx_pending,tag=!yy65_fx_seen] at @s as "..candidate.." at @s anchored eyes positioned ^ ^ ^"..distance.." if block ~ ~ ~ air run particle minecraft:basic_flame_particle ~ ~ ~"
 end
 lines[#lines+1]="0,0,0"
 lines[#lines+1]="/tag @e[type=fishing_hook,tag=yy65_fx_pending] add yy65_fx_seen"
 lines[#lines+1]="0,0,0"
 lines[#lines+1]="/tag @e[type=fishing_hook,tag=yy65_fx_pending] remove yy65_fx_pending"
 lines[#lines+1]="!"
 return table.concat(lines,"\n")
end
function M.apply()
 pcall(util.write_file,game.dataDir().."/yy_gun_r65_probe.txt",
  "entered;chains="..tostring(type(chains))..";save="..tostring(type(chains)=="table" and type(chains.save) or "not_table"))
 if installed then return true end
 if type(chains)~="table" or type(chains.save)~="function" then
  pcall(util.write_file,game.dataDir().."/yy_gun_r65_probe.txt","chains_unavailable:"..tostring(type(chains)))
  return false
 end
 local source=build()
 local ok,val=pcall(chains.validate,source)
 if not ok or type(val)~="table" or val.ok~=true then
  status.errors[#status.errors+1]="validate:"..tostring(val)
  pcall(util.json_write,game.dataDir().."/yy_gun_r65_status.json",status)
  return false
 end
 local listed,snapshot=pcall(chains.list)
 if not listed or type(snapshot)~="table" or type(snapshot.files)~="table" then return false end
 local cfg={mode="fast",channel="ai",interval_ms=100,scheduling="segments",bootstrap=true,
  enabled=true,run_count=0,rate_cps=0}
 for _,row in ipairs(snapshot.files) do
  if type(row.config)=="table" and row.config.name==chain then cfg=row.config;break end
 end
 -- Regenerate the gun program without overriding the user's saved run state or pace.
 local saved,out=pcall(chains.save,chain,source,cfg)
 if not saved or type(out)~="table" or out.ok~=true then
  status.errors[#status.errors+1]="chain save:"..tostring(out)
  pcall(util.json_write,game.dataDir().."/yy_gun_r65_status.json",status)
  return false
 end
 status.commands=(val.plan and val.plan.commands) or 0
 status.installed=true
 installed=true
 local previousSave=G.save
 G.save=function(n,s)
  local a,b=previousSave(n,s)
  pcall(sync,n,s)
  return a,b
 end
 local previousEnable=G.enable
 G.enable=function(v)
  local a,b=previousEnable(v)
  if v==false then cmd("scoreboard players set @a yy65_type 0")
  elseif v==true then syncOnlineOnce() end
  return a,b
 end
 R.native_hud=true
 R.hud=function(n) return sync(n,G.state(n)) end
 if events and type(events.onJoin)=="function" then
  pcall(events.onJoin,"yy_r65_gun_native_join")
 end
 status.bootstrap_players=syncOnlineOnce()
 R.native_status=function()return status end
 plugin_api=plugin_api or {}
 plugin_api.yyGunNativeR65=function()return status end
 pcall(util.json_write,game.dataDir().."/yy_gun_r65_status.json",status)
 return true
end
return M