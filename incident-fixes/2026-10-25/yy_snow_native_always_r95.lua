-- R95: make the original native snowball trigger and 18 clear commands persistent.
-- No special data, no session-gate for always-on projectile scoring/menu.
local M={}
local R={"雪球菜单_综合功能.txt","_违禁收纳袋独立清除20Hz.txt"}
local function pass(ok,res) return ok and res~=false and (type(res)~="table" or res.ok~=false) end
function M.apply()
 local path=game.dataDir().."/yy_snow_r95_activation.json"
 local info={version="R95",at=util.timestamp(),results={},ok=false}
 if type(chains)~="table" or type(chains.list)~="function"
     or type(chains.start)~="function" then
   info.error="native chains unavailable"
   pcall(util.json_write,path,info)
   return false
 end
 local first,snapshot=pcall(chains.list)
 for _,name in ipairs(R) do
   local original
   if first and type(snapshot)=="table" and type(snapshot.files)=="table" then
     for _,v in ipairs(snapshot.files) do
       if v.config and v.config.name==name then original=v;break end
     end
   end
   local rec={name=name}
   if not original then
     rec.error="not registered"
   else
     rec.before_enabled=original.config.enabled
     rec.preserved=true
     rec.started=false -- Startup never overrides a saved stop or starts another worker.
   end
   info.results[#info.results+1]=rec
 end
 local ok2,live=pcall(chains.list)
 if ok2 and type(live)=="table" and type(live.files)=="table" then
   for _,row in ipairs(live.files) do
     for _,rec in ipairs(info.results) do
       if type(row.config)=="table" and rec.name==row.config.name then
         rec.enabled=row.config.enabled
         rec.state=row.state
         rec.nodes=row.commands
         rec.error=(row.error and row.error~="") and row.error or rec.error
         rec.interval_ms=row.config.interval_ms
       end
     end
   end
 end
 info.ok=#info.results==2
 for _,rec in ipairs(info.results) do
   if rec.error or (rec.nodes or 0)==0 then info.ok=false end
 end
 pcall(util.json_write,path,info)
 return info.ok
end
return M