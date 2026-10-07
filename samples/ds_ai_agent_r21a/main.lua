-- 游园 R21A：常驻 Lua 业务 + 原生 FUNCTION，一份插件。
-- modules.core 只 require 一次；事件/定时器不重跑 main.lua。
local bridge=require("modules.native_bridge")
bridge.beforeCore()
require("modules.core")
bridge.afterCore()
plugin_api=plugin_api or {}
plugin_api.frameworkStatus=bridge.status
plugin_api.syncFunctions=bridge.sync
