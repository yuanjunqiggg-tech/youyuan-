-- AI text uses the verified NetEase ModEvent route; long CommandRequest text disconnects this server.
local M={}
local status={segments=0,submitted=0,failed=0,char_limit=200,confirmed=false}
function M.parts(text)
 local parts,buf,n={}, {},0
 for c in tostring(text or ""):gmatch("[%z\1-\127\194-\244][\128-\191]*") do
  buf[#buf+1]=c;n=n+1
  if n==200 then parts[#parts+1]=table.concat(buf);buf={};n=0 end
 end
 if n>0 then parts[#parts+1]=table.concat(buf) end
 return parts
end
local queue,cooling={},false
local function tick()
 local row=table.remove(queue,1)
 if not row then cooling=false;return end
 local ok,res=pcall(game.sendChat,row.target,row.text)
 row.state.pending=row.state.pending-1
 if not ok or res==false or (type(res)=="table" and (res.ok==false or res.submitted==false or res.success==false)) then
  row.state.failed=row.state.failed+1;row.state.error=tostring(res)
  for i=#queue,1,-1 do if queue[i].state==row.state then table.remove(queue,i);row.state.pending=row.state.pending-1 end end
 else row.state.submitted=row.state.submitted+1 end
 -- The server dropped 3/8 immediate sends; 300ms spacing delivered all 8.
 -- Keep each 200-character paragraph intact, and leave the VM free while waiting.
 local scheduled,err=pcall(util.setTimeout,0.3,"yy_ai_paragraph_tick")
 if not scheduled then
  for _,pending in ipairs(queue) do pending.state.failed=pending.state.failed+1;pending.state.pending=pending.state.pending-1;pending.state.error=tostring(err) end
  queue={};cooling=false
 end
end
function M.send(name,text,public)
 local chunks=M.parts(text)
 status={at=os.time(),segments=#chunks,submitted=0,failed=0,pending=0,char_limit=200,confirmed=false,transport="netease_pyrpc"}
 if not util or type(util.setTimeout)~="function" then status.failed=1;status.error="paragraph scheduler unavailable";return false,status end
 local targets,seen={},{}
 local function add(target)
  if type(target)=="string" and target~="" and target~="@a" and not seen[target] then seen[target]=true;targets[#targets+1]=target end
 end
 if public then
  local provider=type(online_players)=="function" and online_players or (player and player.list)
  if type(provider)=="function" then
   local ok,rows=pcall(provider)
   if ok and type(rows)=="table" then for _,row in pairs(rows) do
    if type(row)=="string" then add(row) elseif type(row)=="table" and row.online~=false then add(row.name) end
   end end
  end
 end
 add(tostring(name or ""))
 local required=#chunks*#targets
 if required+#queue>750 or #targets==0 then status.failed=1;status.error="paragraph queue full or no target";return false,status end
 for _,part in ipairs(chunks) do for _,target in ipairs(targets) do
  queue[#queue+1]={target=target,text=part,state=status};status.pending=status.pending+1
 end end
 if not cooling and #queue>0 then cooling=true;tick() end
 return status.failed==0,status
end
function M.apply()
 yy_ai_paragraph_tick=tick
 yy_ai_delivery_final=M.send
 muse_deliv_push=M.send
 agent_send_long=function(name,text)return M.send(name,text,false)end
 agent_echo_long=function(name,text)
  for _,part in ipairs(M.parts(text)) do agent_echo(name,part) end
  return true
 end
 plugin_api=plugin_api or {}
 plugin_api.yyAiDeliveryStatus=function()return status end
 plugin_api.yyAiDeliveryProbe=function(name,text)
  if type(is_admin)~="function" or not is_admin(name) then return {error="仅管理员"} end
  M.send(name,text,false);return status
 end
 plugin_api.yyAiChatQuery=function(name,words)
  if type(is_admin)~="function" or not is_admin(name) then return {error="仅管理员"} end
  return on_chat(name,"ai "..tostring(words or ""))
 end
 return true
end
return M
