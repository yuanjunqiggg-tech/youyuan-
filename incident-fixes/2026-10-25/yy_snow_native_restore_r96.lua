-- R96: restore original always-on exhaustive native snowball chain.
-- The engine, not Lua, executes snowball detection, menu and 111 downhill TPs.
local M={}
local applied=false
local N="雪球菜单_综合功能.txt"
local C="_违禁收纳袋独立清除20Hz.txt"
local function good(ok,v) return ok and v~=false and (type(v)~="table" or v.ok~=false) end
function M.apply()
 if applied then return true end
 applied=true
 local result={version="R96",at=util.timestamp(),files={},intended="always_on_no_gate_111_exhaustive"}
 if type(chains)~="table" then result.error="native chain missing" return false end
 -- Native persistence owns enable state, Hz and mode; this retired migration only reports them.
 result.intended="preserve_native_state"
 if type(chains.list)=="function" then
  local ok,data=pcall(chains.list)
  if ok and type(data)=="table" and type(data.files)=="table" then
   for _,item in ipairs(data.files) do
    if item.config and (item.config.name==N or item.config.name==C) then
     result.files[#result.files+1]={kind="actual",name=item.config.name,
       enabled=item.config.enabled,commands=item.commands,
       error=item.error,state=item.state,cycles=item.cycles}
    end
   end
  end
 end
 pcall(util.json_write,game.dataDir().."/yy_snow_r96_restore_status.json",result)
 return true
end
return M