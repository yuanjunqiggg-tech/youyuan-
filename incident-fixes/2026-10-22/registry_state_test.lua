package.preload["modules.yy_txt_autostart_r64"]=function()return {apply=function()end}end
util={schedule=function()end};plugin={};mcp={connection=function()end,tools=function()end,call=function()end};plugin_api={}
local active=false;local starts,stops=0,0
local saved
function_registry_load=function()return {items={a={name="menu.txt",enabled=true,loaded=true}}}end
function_registry_save=function(reg)saved=reg end
chains={start_marker=true,list=function()return {files={{config={name="menu.txt",enabled=active},state=active and "运行中" or "已停止",error=""}}}end,
 start=function()active=true;starts=starts+1 end,stop=function()active=false;stops=stops+1 end}
local m=assert(loadfile(SOURCE))();m.afterCore()
local d=function_registry_load();assert(d.items.a.enabled==false and d.items.a.native_state=="已停止","menu showed stale enabled flag")
function_registry_save(d);assert(starts==0 and stops==0,"metadata save changed actual state")
d.items.a.enabled=true;function_registry_save(d);assert(active and starts==1,"menu start not applied: "..tostring(active)..":"..starts..":"..stops)
d=function_registry_load();assert(d.items.a.enabled and d.items.a.native_state=="运行中")
d.items.a.enabled=false;function_registry_save(d);assert(not active and stops==1,"menu stop not applied")
active=true;d=function_registry_load();assert(d.items.a.enabled,"native external start not reflected")
function_registry_save(d);assert(stops==1,"stale metadata undid external start")
print("PASS: menu reads native state, metadata does not toggle, explicit start/stop propagates")
