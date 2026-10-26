local saved=0
chains={list=function()return {files={{config={name="menu.txt",enabled=false}},{config={name="active.txt",enabled=true}}}}end,
 save=function()saved=saved+1;return {}end}
function_registry_load=function()return {}end
function_registry_sorted=function()return {{id=1,item={name="menu.txt"}},{id=2,item={name="active.txt"}}}end
local m=assert(loadfile(SOURCE))()
assert(m.syncAll()==2 and saved==0,"startup overwrote native chain config")
chains.list=function()error("offline")end
assert(m.syncAll()==0 and saved==0,"unavailable native config must fail closed")
print("PASS: bridge startup preserves stopped/enabled native config; lookup failure fails closed")
