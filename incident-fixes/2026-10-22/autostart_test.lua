local source=assert(SOURCE)
local function setup(enabled,registered)
 local calls={start=0,stop=0,sync=0};local online=true
 local native=enabled;local item={name="menu.txt",enabled=registered,loaded=true}
 util={timestamp=function()return 1 end,json_write=function()end,scheduleSeconds=function()end}
 game={dataDir=function()return "." end};bot={status=function()return {connected=online}end};events={onBotReady=function()end};plugin_api={}
 function_scan_files=function()end;function_registry_load=function()return {}end
 function_registry_sorted=function()return {{id=1,item=item}}end;function_read_text=function()return "say hello"end
 function_registry_save=function()end
 function_schedule_one=function()calls.sync=calls.sync+1;native=true;return true end
 chains={list=function()local rows={};if native~=nil then rows[1]={config={name="menu.txt",enabled=native}}end;return {files=rows}end,
 start=function()calls.start=calls.start+1;native=true;return true end,
 stop=function()calls.stop=calls.stop+1;native=false;return true end}
 local m=assert(loadfile(source))();m.apply()
 return calls,function(v)online=v;yy_txt_autostart_r64_tick()end,function()return native end
end
local c,tick,state=setup(false,true)
assert(c.start==0 and c.sync==0 and state()==false,"stopped native chain restarted")
tick(false);tick(true);assert(c.start==0,"reconnect restarted stopped chain")
plugin_api.yyTXTAutostartStart();assert(c.start==1 and c.sync==0 and state()==true,"explicit start failed")
plugin_api.yyTXTAutostartStop();assert(c.stop==1 and state()==false)
tick(true);assert(c.start==1 and state()==false,"tick undid manual stop")
c,tick,state=setup(true,true);assert(c.start==1 and c.sync==0,"existing config resaved")
tick(false);assert(c.stop==0 and state()==true,"disconnect disabled enabled intent")
tick(true);assert(c.start==2 and c.sync==0)
c=setup(nil,false);assert(c.start==0 and c.sync==0,"disabled registry auto-enabled")
c=setup(nil,true);assert(c.start==1 and c.sync==1,"new enabled registration not synchronized")
chains.list=function()error("unavailable")end;tick(false);tick(true)
print("PASS: stopped intent, reconnect, explicit start/stop, unchanged config, new registration")
