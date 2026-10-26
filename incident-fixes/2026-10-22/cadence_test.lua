package.preload["modules.yy_txt_autostart_r64"]=function()return {apply=function()end}end
util={schedule=function()end};plugin={};mcp={connection=function()end,tools=function()end,call=function()end};plugin_api={}
FUNCTION_MIN_DELAY=.001;FUNCTION_MAX_DELAY=100000
local native={name="menu.txt",hz=20,interval_ms=50,enabled=false}
local stored={items={a={name="menu.txt",enabled=false,loaded=true,delay_sec=.05}}}
function_registry_load=function()return stored end
function_registry_save=function(reg)stored=reg end
function_registry_get=function(id)local r=function_registry_load();return r.items[id],r end
is_admin=function(name)return name=="admin"end
local last_message
send=function(_,message)last_message=message end
local configured=0
chains={list=function()return {files={{config=native,state="已停止",error=""}}}end,
 configure=function(name,patch)assert(name=="menu.txt");configured=configured+1;native.hz=patch.hz;native.interval_ms=patch.interval_ms;return {ok=true}end,
 start=function()error("changing stopped chain frequency started it")end,stop=function()error("unexpected stop")end}
local m=assert(loadfile(SOURCE))();m.afterCore()
function_set_delay("guest","a",1);assert(configured==0,"non-admin changed cadence")
function_set_delay("admin","a",1)
assert(configured==1 and native.hz==1 and native.interval_ms==1000 and native.enabled==false,"stopped cadence not persisted coherently")
assert(function_registry_load().items.a.delay_sec==1,"menu did not read native cadence")
native.hz=2;native.interval_ms=500
assert(function_registry_load().items.a.delay_sec==.5,"external Hz edit reverted by stale registry")
chains.configure=function()return {ok=false,error="denied"}end
function_set_delay("admin","a",1)
assert(last_message:find("未保存",1,true) and stored.items.a.delay_sec==.5,"failed edit falsely reported success")
print("PASS: admin cadence edit persists while stopped; native changes reflect in registry; failure stays failure")
