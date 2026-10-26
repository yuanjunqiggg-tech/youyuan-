package.preload["modules.yy_gun_data"]=function()return {all=function()return {}end,save=function()end,enable=function()end}end
package.preload["modules.yy_gun_runtime"]=function()return {}end
util={write_file=function()end,json_write=function()end};game={dataDir=function()return "."end};player={list=function()return {}end};events={};plugin_api={}
for _,enabled in ipairs({false,true}) do
 local saved
 chains={list=function()return {files={{config={name="yy_rodgun_native_r65.txt",enabled=enabled,interval_ms=250,rate_cps=200}}}}end,
 validate=function()return {ok=true}end,save=function(n,text,cfg)saved=cfg;return {ok=true}end}
 assert(assert(loadfile(SOURCE))().apply())
 assert(saved.enabled==enabled and saved.interval_ms==250 and saved.rate_cps==200,"gun startup overrode saved config")
end
chains.list=function()error("offline")end
chains.save=function()error("must not save without lookup")end
assert(assert(loadfile(SOURCE))().apply()==false)
print("PASS: gun startup preserves enabled/disabled, interval, and budget; failed lookup does not save")
