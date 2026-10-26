-- 游园 R21D：常驻 Lua + 原生命令链 + 稳定化修复层
for _,name in ipairs({
  "modules.compat",
  "modules.native_bridge",
  "modules.core",
  "modules.ai_fix",
  "modules.tool_fix",
  "modules.death_native",
  "modules.r21d_fix",
  "modules.player_status",
  "modules.health_display",
  "modules.item_translate",
  "modules.inv_fix",
  "modules.nbt_import_fix",
  "modules.yy_recovery_r57",
  "modules.yy_ai_recovery",
  "modules.yy_gun_data",
  "modules.yy_gun_runtime",
  "modules.yy_gun_menu",
  "modules.yy_gun_bridge",
  "modules.yy_gun_recover_init",
  "modules.yy_gun_native_r65",
  "modules.yy_chunk_edge_guard",
  "modules.yy_score_recycle",
  "modules.yy_death_balance_fix",
}) do
  package.loaded[name]=nil
end

pcall(function() require("modules.compat") end)

local bridge=require("modules.native_bridge")
bridge.beforeCore()
require("modules.core")
bridge.afterCore()

plugin_api=plugin_api or {}
plugin_api.frameworkStatus=bridge.status
plugin_api.syncFunctions=bridge.sync

local function apply_module(name)
  local ok,mod=pcall(require,name)
  if not ok then
    pcall(function() util.write_file(game.dataDir().."/module_load_error.txt",name..": "..tostring(mod)) end)
    return false
  end
  if type(mod)=="table" and type(mod.apply)=="function" then
    local ok2,err=pcall(mod.apply)
    if not ok2 then
      pcall(function() util.write_file(game.dataDir().."/module_load_error.txt",name..": "..tostring(err)) end)
      return false
    end
  end
  return true
end

apply_module("modules.ai_fix")
-- Install safe death resolvers before slow extension initialization.
apply_module("modules.yy_death_core_guard_r64")
apply_module("modules.yy_death_balance_fix")
apply_module("modules.tool_fix")
apply_module("modules.death_native")
apply_module("modules.r21d_fix")
-- R66: strict Text death event validation; stop sneak/chat false positives.
apply_module("modules.yy_death_text_guard_r66")
apply_module("modules.player_status")
apply_module("modules.health_display")
apply_module("modules.item_translate")
apply_module("modules.inv_fix")
apply_module("modules.nbt_import_fix")
-- Rollback recovery: keep original health scoreboard and all existing modules.
apply_module("modules.yy_ai_recovery")
apply_module("modules.yy_owner_self_mute")
apply_module("modules.yy_owner_safe_pressure")
-- 2026-10-14 R44 archived AI model and prompt compatibility repair
apply_module("modules.yy_ai_model_select_r44")

pcall(function()
  if type(plugin_api.reloadPrompts)=="function" then plugin_api.reloadPrompts() end
end)

-- YY_ROLLBACK_RESTORE_20261021: small independent restoration modules.
apply_module("modules.yy_chunk_edge_guard")
apply_module("modules.yy_score_recycle")
-- R57 restore native all-player AI broadcast + fresh inventory + limited MCP prompts.
-- R57 is re-applied below all compatibility modules for authoritative chat/command handlers.
apply_module("modules.yy_gun_recover_init")

-- R57 last to preserve existing business routes and recover live AI broadcast.
apply_module("modules.yy_recovery_r57")
-- Death resolver and balance read are already installed immediately after core.
-- R65: native scoreboard HUD and hook visual chain, independent of R57 AI layer.
apply_module("modules.yy_gun_native_r65")
-- R68: read-only, low-frequency diagnostic of native callback coalescing.
function yy_native_fanout_diagnostics()
  if type(bridge.status)=="function" then
    pcall(util.json_write,game.dataDir().."/yy_native_fanout_status.json",bridge.status())
  end
  return true
end
pcall(yy_native_fanout_diagnostics)
if util and type(util.scheduleSeconds)=="function" then
  pcall(util.scheduleSeconds,30,"yy_native_fanout_diagnostics")
end
-- ============================================================
-- .muse 远程工作入口（Muse 于 2026-10-21 追加到本插件入口末尾）
-- 只监听玩家“疣螈”以 .muse 开头的聊天，存入本插件 storage 队列 muse_queue，
-- 供 Muse 巡检通过 plugin_api.museInboxList 读取、museInboxAck 确认。
-- 其他玩家、其他内容一概不处理。
-- ============================================================
local MUSE_OWNER = "疣螈"
local MUSE_PREFIX = ".muse"
local MUSE_QUEUE_KEY = "muse_queue"
local MUSE_MAX_KEEP = 50

local function muse_trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

local function muse_load_queue()
  local q = storage.get(MUSE_QUEUE_KEY)
  if type(q) ~= "table" then q = {} end
  return q
end

-- 前置声明（F3 联动）：专用日志读写在 muse_save_queue 之后定义，此处先声明，
-- 使 50 cap 删头联动能调用它们（Lua 局部变量须先声明，运行时已赋值）。
local muse_load_direct_log, muse_save_direct_log
local function muse_save_queue(q)
  -- 50 cap 删头联动（F3）：被 cap 丢弃的是未处理项，在专用日志里记 dropped=true
  -- （acked 保持 false；“待办” = acked 非 true 且 dropped 非 true；轮转只删 acked 或 dropped 项）。
  local dropped_ids = {}
  while #q > MUSE_MAX_KEEP do
    local head = table.remove(q, 1)
    if head ~= nil and head.log_id ~= nil then dropped_ids[#dropped_ids + 1] = head.log_id end
  end
  storage.set(MUSE_QUEUE_KEY, q)
  if storage.flush then pcall(storage.flush) end
  if #dropped_ids > 0 then
    local log = muse_load_direct_log()
    local changed = false
    for _, e in ipairs(log) do
      for _, lid in ipairs(dropped_ids) do
        if e.id == lid and e.acked ~= true and e.dropped ~= true then e.dropped = true changed = true end
      end
    end
    if changed then muse_save_direct_log(log) end
  end
end
-- .muse 专用日志（2026-10-25 结构性改造）：.muse 提交走单独日志，不跟其它事件混在一起；
-- 以后巡查读这个日志（读路径），队列仍是处理游标（两者一致：同一 intake 双写、同一 id）。
-- 时间戳取插件侧 os.time()（宿主墙钟，与队列 at 同源；展示时 +8h 换算北京时间 Asia/Shanghai；
-- 绝不用游戏内时间——记录时间不是拨表）。
local MUSE_DIRECT_LOG_KEY = "muse_direct_log"
local MUSE_DIRECT_LOG_SEQ_KEY = "muse_direct_log_seq"
local MUSE_DIRECT_LOG_MIGRATED_KEY = "muse_direct_log_migrated"
local MUSE_DIRECT_LOG_KEEP = 500

muse_load_direct_log = function()
  local log = storage.get(MUSE_DIRECT_LOG_KEY)
  if type(log) ~= "table" then log = {} end
  return log
end

muse_save_direct_log = function(log)
  storage.set(MUSE_DIRECT_LOG_KEY, log)
  if storage.flush then pcall(storage.flush) end
end

-- 存量回填（一次性，幂等）：旧队列项 → 专用日志（带 id，acked:false），新日志从下一条开始写；
-- 回填后计数值定位到最大 id，后续 intake 的 id 接着编，不与回填 id 冲突。
local function muse_ensure_direct_log_migrated()
  if storage.get(MUSE_DIRECT_LOG_MIGRATED_KEY) == true then return end
  local q = muse_load_queue()
  local log = muse_load_direct_log()
  if #q > 0 and #log == 0 then
    local seq = 0
    for _, item in ipairs(q) do
      seq = seq + 1
      item.log_id = seq
      log[#log + 1] = { id = seq, ts = tonumber(item.at) or os.time(), sender = tostring(item.player or MUSE_OWNER), text = tostring(item.text or ""), acked = false }
    end
    muse_save_direct_log(log)
    muse_save_queue(q)
    storage.set(MUSE_DIRECT_LOG_SEQ_KEY, seq)
    if storage.flush then pcall(storage.flush) end
  elseif #log > 0 and #log == #q then
    -- 中断修复（F1/F2）：上次回填写了日志、没写回队列就中断时，旗仍未置；
    -- 此时日志与队列条数一致且顺序同源（回填按队列顺序写），按位置补打 log_id。
    -- 条数不一致时不补（期间可能有 ack 删头，位置已错位，宁缺毋错）。
    local changed = false
    for i, item in ipairs(q) do
      if item.log_id == nil and log[i] ~= nil and log[i].id ~= nil then
        item.log_id = log[i].id
        changed = true
      end
    end
    if changed then muse_save_queue(q) end
  end
  storage.set(MUSE_DIRECT_LOG_MIGRATED_KEY, true)
  if storage.flush then pcall(storage.flush) end
end

-- 取下一 id（F2）：SEQ 为空（迁移中断导致）且日志非空时，从日志最大 id 推导，绝不从 1 起撞号；
-- SEQ 正常时仍是 SEQ+1；id 允许断号（失败重试会消耗 id），绝不重用。
local function muse_next_direct_log_seq()
  local saved = tonumber(storage.get(MUSE_DIRECT_LOG_SEQ_KEY))
  if saved ~= nil then return saved + 1 end
  local log = muse_load_direct_log()
  local maxid = 0
  for _, e in ipairs(log) do
    local id = tonumber(e.id)
    if id ~= nil and id > maxid then maxid = id end
  end
  return maxid + 1
end

function plugin_api.museDirectLogList()
  muse_ensure_direct_log_migrated()
  return muse_load_direct_log()
end

-- 处理者完成某条后按 id 置 acked:true；队列的 museInboxAck 前缀删除会按 log_id 联动置 acked（双游标联动，见 F3），处理者仍走本导出。
function plugin_api.museDirectLogAck(id)
  muse_ensure_direct_log_migrated()
  local log = muse_load_direct_log()
  id = tonumber(id)
  for _, e in ipairs(log) do
    if e.id == id then
      local already = (e.acked == true)
      e.acked = true
      muse_save_direct_log(log)
      return { ok = true, found = true, id = id, already = already }
    end
  end
  return { ok = false, found = false, id = id }
end


-- 下次巡检倒计时：主巡检（Muse 日志巡检）每 2 小时一轮，锚点为北京时间奇数小时 :17（01:17、03:17、05:17…）。
-- 仍用 unix 时间戳 +8 小时换算北京时间，不依赖服务器本地时区设置。
local function muse_next_patrol()
local bj = os.time() + 28800
local h = math.floor(bj / 3600) % 24
local sec = bj % 3600
local delta = nil
if h % 2 == 1 and sec < 17 * 60 then delta = 17 * 60 - sec
elseif h % 2 == 1 then delta = 7200 - sec + 17 * 60
else delta = 3600 - sec + 17 * 60 end
local nxt = bj + delta
local mm = math.ceil(delta / 60)
if mm < 1 then mm = 1 end
local at = os.date("!%H:%M", nxt)
return "约 " .. mm .. " 分钟后，" .. at
end

function museChat(player, message)
  if player ~= MUSE_OWNER then return end
  local msg = muse_trim(message)
  if msg:sub(1, #MUSE_PREFIX) ~= MUSE_PREFIX then return end
  local content = muse_trim(msg:sub(#MUSE_PREFIX + 1))
  if content == "" then
    game.sendTargeted(player, "§e[Muse] 用法：在 .muse 后面写下你的要求，例如 §f.muse 测试一下队列")
    return
  end
  muse_ensure_direct_log_migrated()
  local now = os.time()
  local seq = muse_next_direct_log_seq()
  local q = muse_load_queue()
  q[#q + 1] = { player = player, text = content, at = now, log_id = seq }
  local ok_q = pcall(muse_save_queue, q)
  -- 读回校验（F4）：pcall 成功不等于已持久化；读回队尾 log_id 核对。
  -- 区分两种失败：保存抛错（队列未变，重发可安全复用 id）与读回对不上
  -- （队列里可能已残留 log_id=seq 的孤儿项，必须消耗 SEQ 防复用撞号）。
  local q_verified = false
  if ok_q then
    local qv = muse_load_queue()
    if #qv > 0 and qv[#qv].log_id == seq then q_verified = true end
  end
  if not ok_q then
    game.sendTargeted(player, "§c[Muse] 登记失败：队列写入失败，请重发一次。")
    return
  end
  if not q_verified then
    pcall(storage.set, MUSE_DIRECT_LOG_SEQ_KEY, seq)
    if storage.flush then pcall(storage.flush) end
    game.sendTargeted(player, "§c[Muse] 登记失败：队列写入读回校验失败，队列可能残留 id #" .. seq .. " 的条目，请人工核对后重发。")
    return
  end
  local dlog = muse_load_direct_log()
  dlog[#dlog + 1] = { id = seq, ts = now, sender = player, text = content, acked = false }
  while #dlog > MUSE_DIRECT_LOG_KEEP do
    local drop = nil
    for i, e in ipairs(dlog) do if e.acked == true or e.dropped == true then drop = i break end end
    if not drop then break end
    table.remove(dlog, drop)
  end
  local ok_l = pcall(function()
    muse_save_direct_log(dlog)
    storage.set(MUSE_DIRECT_LOG_SEQ_KEY, seq)
    if storage.flush then pcall(storage.flush) end
  end)
  -- 读回校验（F4）：日志尾 id 与 SEQ 值双核对，不符按写失败走回滚分支。
  if ok_l then
    local dlog_v = muse_load_direct_log()
    local seq_v = tonumber(storage.get(MUSE_DIRECT_LOG_SEQ_KEY))
    if #dlog_v == 0 or dlog_v[#dlog_v].id ~= seq or seq_v ~= seq then ok_l = false end
  end
  if not ok_l then
    -- 回滚双边（F3）：队列尾与日志尾都要清（ok_l 可能在写日志后、写 SEQ 前失败，
    -- 只清一边会留孤儿）；两边保存都要检查结果 + 读回核对。
    local q2 = muse_load_queue()
    if #q2 > 0 and q2[#q2].log_id == seq then table.remove(q2) end
    local ok_rb = pcall(muse_save_queue, q2)
    local q2v = muse_load_queue()
    local q_rb_ok = ok_rb and (#q2v == 0 or q2v[#q2v].log_id ~= seq)
    local dlog2 = muse_load_direct_log()
    local log_had = (#dlog2 > 0 and dlog2[#dlog2].id == seq)
    if log_had then table.remove(dlog2) end
    local ok_lb = true
    if log_had then ok_lb = pcall(muse_save_direct_log, dlog2) end
    local dlog2v = muse_load_direct_log()
    local l_rb_ok = ok_lb
    if log_had then
      for _, e in ipairs(dlog2v) do if e.id == seq then l_rb_ok = false break end end
    end
    if q_rb_ok and l_rb_ok then
      game.sendTargeted(player, "§c[Muse] 登记失败：日志写入失败，请重发一次。")
      return
    end
    -- 回滚本身失败：显式处理，不静默留孤儿——先把 SEQ 消耗掉（下一条绝不复用同一 id），
    -- 再定向通知 owner 人工核对（孤儿项 log_id=seq，队列/日志可能各残留一边）。
    pcall(storage.set, MUSE_DIRECT_LOG_SEQ_KEY, seq)
    if storage.flush then pcall(storage.flush) end
    game.sendTargeted(player, "§c[Muse] 登记失败且回滚未完成：队列/日志可能残留 id #" .. seq .. " 的条目，请人工核对后重发。")
    return
  end
  game.sendTargeted(player, "§a任务已登记 #" .. seq .. " §7下次巡检：" .. muse_next_patrol())
  if #q == 45 then
    game.sendTargeted(player, "§e[Muse] 提醒：待处理队列已达 45 条（上限 50 条，每次达到都会提醒），请及时处理/确认，避免最老项被丢弃。")
  end
end

events.onChat("museChat")

plugin_api = plugin_api or {}
function plugin_api.museInboxList()
  return muse_load_queue()
end

function plugin_api.museInboxAck(n)
  muse_ensure_direct_log_migrated()
  local q = muse_load_queue()
  n = tonumber(n) or 0
  local acked_ids = {}
  for i = 1, math.min(n, #q) do
    if q[i].log_id ~= nil then acked_ids[#acked_ids + 1] = q[i].log_id end
  end
  local left = {}
  for i = n + 1, #q do left[#left + 1] = q[i] end
  muse_save_queue(left)
  -- 双游标联动（F3）：队列前缀 ack 的项，按 log_id 在专用日志里同步置 acked；
  -- 找不到对应日志项时跳过（项可能已被 500 轮转删掉，或为回填前的孤儿项）。
  if #acked_ids > 0 then
    local log = muse_load_direct_log()
    local changed = false
    for _, e in ipairs(log) do
      for _, lid in ipairs(acked_ids) do
        if e.id == lid and e.acked ~= true then e.acked = true changed = true end
      end
    end
    if changed then muse_save_direct_log(log) end
  end
  return { remaining = #left }
end
-- 巡查问话专用导出（2026-10-25 协议接线）：巡查 worker 没有定向 tellraw 入口时走这里；
-- 定向只发疣螈本人（不打扰全服）；同一问题一轮内 tellraw 与 say 各至多一次（按 kind+轮次+原文去重）；
-- 问话本身不写进 .muse 队列、不冒用疣螈名义（发送方为插件/机器人语境，内容带 [Muse 巡查] 前缀）。
-- 去重表上限 200 条，超量删最老（新问题永远可问；同轮内最早的记录被删后若重问可能重复，属可接受边界）。
-- kind: "tellraw"（定向）或 "say"（服务器广播）；round：调用方传入的稳定巡查轮次标识（必填，见 F6）。
local MUSE_ASK_KEY = "muse_ask_sent"
local MUSE_ASK_KEEP = 200

function plugin_api.museAskOwner(kind, text, round)
  if kind ~= "tellraw" and kind ~= "say" then return { ok = false, reason = "kind 须为 tellraw 或 say" } end
  text = muse_trim(text or "")
  if text == "" then return { ok = false, reason = "问话内容为空" } end
  -- 净化正文（F7，不是前缀）：换行等控制字符→空格、剥离全部 § 码，防断行/改色注入广播；
  -- 前缀格式码由导出内自带，不由调用文本带入。
  text = text:gsub("%c+", " "):gsub("§", "")
  if muse_trim(text) == "" then return { ok = false, reason = "问话内容净化后为空" } end
  -- F6：round 为必填（无默认值）：调用方须传入稳定巡查轮次标识；禁止小时桶默认，
  -- 否则跨整点的一轮会切分去重桶、同一问题一轮内问两次。巡查 worker 传自己的轮次 id。
  if round == nil or tostring(round) == "" then
    return { ok = false, reason = "round 为必填：请传入稳定巡查轮次标识" }
  end
  round = tostring(round)
  local sent = storage.get(MUSE_ASK_KEY)
  if type(sent) ~= "table" then sent = {} end
  for _, s in ipairs(sent) do
    if s.kind == kind and s.round == round and s.text == text then
      return { ok = false, reason = "本轮已问过同一问题", duplicate = true }
    end
  end
  local delivered = false
  if kind == "tellraw" then
    local ok, res = pcall(game.sendTargeted, MUSE_OWNER, "§e[Muse 巡查] §f" .. text)
    delivered = ok and res ~= false
  else
    local ok, res = pcall(game.sendCommand, "say [Muse 巡查] " .. text)
    delivered = ok and res ~= false
  end
  if not delivered then return { ok = false, reason = "发送调用失败" } end
  sent[#sent + 1] = { kind = kind, round = round, text = text, ts = os.time() }
  while #sent > MUSE_ASK_KEEP do table.remove(sent, 1) end
  storage.set(MUSE_ASK_KEY, sent)
  if storage.flush then pcall(storage.flush) end
  return { ok = true, kind = kind, round = round }
end




-- ============================================================
-- 找结构（基岩版，Muse 于 2026-10-21 追加到本插件入口末尾）
-- 命令名用 .找结构，是为了避开插件自带的 .种子（世界种子设置）和 .结构：
-- 本命令的特点是以发命令的玩家为原点定位；自带 .结构 以机器人为原点，
-- 并带种子引擎兜底。两者并行，互不干扰。
-- 用法（任意玩家，游戏内聊天）：
--   .找结构        查看用法和可查结构
--   .找结构 村庄   查离你最近的村庄（也支持英文 id，如 village）
-- 原理：以发命令的玩家为原点，让服务器执行 /locate structure，
--       解析服务器返回的原始文本，得到坐标 X/Z 与距离。
--       查的是当前世界（当前种子），结果以玩家所在维度为准。
-- ============================================================
local SEED_PREFIX = ".找结构"
local SEED_STRUCTS = {
  ["村庄"] = {"village", "村庄"}, ["village"] = {"village", "村庄"},
  ["要塞"] = {"stronghold", "要塞"}, ["stronghold"] = {"stronghold", "要塞"},
  ["古城"] = {"ancient_city", "远古城市"}, ["远古城市"] = {"ancient_city", "远古城市"},
  ["ancient_city"] = {"ancient_city", "远古城市"}, ["ancientcity"] = {"ancient_city", "远古城市"},
  ["试炼密室"] = {"trial_chambers", "试炼密室"}, ["密室"] = {"trial_chambers", "试炼密室"},
  ["trial_chambers"] = {"trial_chambers", "试炼密室"},
  ["林地府邸"] = {"mansion", "林地府邸"}, ["府邸"] = {"mansion", "林地府邸"}, ["mansion"] = {"mansion", "林地府邸"},
  ["海底神殿"] = {"monument", "海底神殿"}, ["monument"] = {"monument", "海底神殿"},
  ["矿井"] = {"mineshaft", "废弃矿井"}, ["废弃矿井"] = {"mineshaft", "废弃矿井"}, ["mineshaft"] = {"mineshaft", "废弃矿井"},
  ["神殿"] = {"temple", "神殿"}, ["temple"] = {"temple", "神殿"},
  ["前哨站"] = {"pillager_outpost", "掠夺者前哨站"}, ["掠夺者前哨站"] = {"pillager_outpost", "掠夺者前哨站"},
  ["pillager_outpost"] = {"pillager_outpost", "掠夺者前哨站"},
  ["破碎传送门"] = {"ruined_portal", "破碎传送门"}, ["传送门废墟"] = {"ruined_portal", "破碎传送门"},
  ["ruined_portal"] = {"ruined_portal", "破碎传送门"}, ["ruinedportal"] = {"ruined_portal", "破碎传送门"},
  ["沉船"] = {"shipwreck", "沉船"}, ["shipwreck"] = {"shipwreck", "沉船"},
  ["海底废墟"] = {"ruins", "海底废墟"}, ["ruins"] = {"ruins", "海底废墟"},
  ["宝藏"] = {"buried_treasure", "埋藏的宝藏"}, ["埋藏的宝藏"] = {"buried_treasure", "埋藏的宝藏"},
  ["buried_treasure"] = {"buried_treasure", "埋藏的宝藏"},
  ["下界要塞"] = {"fortress", "下界要塞"}, ["fortress"] = {"fortress", "下界要塞"},
  ["堡垒"] = {"bastion_remnant", "堡垒遗迹"}, ["堡垒遗迹"] = {"bastion_remnant", "堡垒遗迹"},
  ["bastion_remnant"] = {"bastion_remnant", "堡垒遗迹"}, ["bastionremnant"] = {"bastion_remnant", "堡垒遗迹"},
  ["末地城"] = {"end_city", "末地城"}, ["end_city"] = {"end_city", "末地城"}, ["endcity"] = {"end_city", "末地城"},
  ["古迹废墟"] = {"trail_ruins", "古迹废墟"}, ["trail_ruins"] = {"trail_ruins", "古迹废墟"},
}
local SEED_LIST_TEXT = "村庄 要塞 古城 试炼密室 林地府邸 海底神殿 矿井 神殿 前哨站 破碎传送门 沉船 海底废墟 宝藏 下界要塞 堡垒遗迹 末地城 古迹废墟"
local seed_last = {}

local function seed_trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

local function seed_parse_locate(out)
  if type(out) ~= "table" then return nil end
  for _, line in ipairs(out) do
    local s = tostring(line)
    if s:find("commands.locate.structure.success", 1, true) then
      local nums = {}
      for n in s:gmatch("-?%d+") do nums[#nums + 1] = tonumber(n) end
      if #nums >= 3 then return nums[1], nums[2], nums[3] end
    end
  end
  return nil
end

function seedChat(player, message)
  local msg = seed_trim(message)
  if msg:sub(1, #SEED_PREFIX) ~= SEED_PREFIX then return end
  local rest = seed_trim(msg:sub(#SEED_PREFIX + 1))
  if rest == "" or rest == "列表" or rest == "帮助" then
    game.sendTargeted(player, "§e[种子查询] 用法：§f.找结构 <结构名>§e，例如 §f.找结构 村庄")
    game.sendTargeted(player, "§7可查：" .. SEED_LIST_TEXT .. "（也支持英文名，如 village、stronghold）")
    return
  end
  local entry = SEED_STRUCTS[rest] or SEED_STRUCTS[rest:lower()]
  if not entry then
    game.sendTargeted(player, "§c[种子查询] 不认识「" .. rest .. "」，发 §f.找结构 §c看可查列表")
    return
  end
  local now = os.time()
  if seed_last[player] and now - seed_last[player] < 5 then
    game.sendTargeted(player, "§e[种子查询] 查太快了，等几秒再试")
    return
  end
  seed_last[player] = now
  local id, label = entry[1], entry[2]
  local pname = tostring(player)
  local x, z, dist, from_player
  -- 先以发命令的玩家为原点查（名字加引号，兼容中文名和带空格的名字）
  if not pname:find('"', 1, true) and not pname:find("\n", 1, true) then
    local ok, res = pcall(game.execute, 'execute as "' .. pname .. '" at @s run locate structure ' .. id, { channel = "ai", timeout_ms = 8000 })
    if ok and type(res) == "table" and res.output then
      x, z, dist = seed_parse_locate(res.output)
      if x then from_player = true end
    end
  end
  -- 以玩家为原点没查到时，回退到以机器人为原点再查一次
  if not x then
    local ok2, res2 = pcall(game.execute, "locate structure " .. id, { channel = "ai", timeout_ms = 8000 })
    if ok2 and type(res2) == "table" and res2.output then
      x, z, dist = seed_parse_locate(res2.output)
    end
  end
  if x and from_player then
    game.sendTargeted(player, "§a[种子查询] 离你最近的" .. label .. "：§fX " .. x .. "，Z " .. z .. "§a，约 " .. dist .. " 格（按你所在维度查的）")
  elseif x then
    game.sendTargeted(player, "§a[种子查询] 离机器人最近的" .. label .. "：§fX " .. x .. "，Z " .. z .. "§a，约 " .. dist .. " 格")
  else
    game.sendTargeted(player, "§c[种子查询] 这次没查到" .. label .. "，可能机器人没连上游戏或附近没有，过一会再试")
  end
end

events.onChat("seedChat")


-- ===== Muse AI 优化补丁 v1（2026-10-21）：去重 / 在线名单放行 / 背包缓存重试 =====
-- 1) 消息去重已退役：统一由 quiet_fix 的双通道认领制负责（2026-10-22 源码层修复）。
-- 2) 普通玩家用 [LIST] 查在线名单：房主要求放行（仅 LIST，其它被禁工具照旧拒绝）。
-- 3) 背包查询拿到的是超过 20 秒的旧缓存时，自动重试一次实时查询。
if not _G.__muse_ai_opt_v1 then
  _G.__muse_ai_opt_v1 = true
  local _muse_seen = {}
  local _muse_prev_disallowed = ai_ordinary_disallowed_tool
  if type(_muse_prev_disallowed) == "function" then
    ai_ordinary_disallowed_tool = function(text, playerName)
      local r = _muse_prev_disallowed(text, playerName)
      if not r then return r end
      local s = tostring(text or "")
      local s2 = s:gsub("%[LIST%]", "[LISTX]"):gsub("%[/LIST%]", "[/LISTX]")
      if s2 == s then return r end
      return _muse_prev_disallowed(s2, playerName)
    end
  end
  local _muse_prev_invsync = ai_inventory_sync
  if type(_muse_prev_invsync) == "function" then
    ai_inventory_sync = function(playerName, targetText, admin)
      local r = _muse_prev_invsync(playerName, targetText, admin)
      local _m = tostring(r or ""):match("最后同步约(%d+)秒前"); local age = _m and tonumber(_m) or nil
      if age and age > 20 then
        local r2 = _muse_prev_invsync(playerName, targetText, admin)
        if type(r2) == "string" and not r2:find("机器人库存缓存", 1, true) then return r2 end
        return r2
      end
      return r
    end
  end
  plugin_api.museAiOptVersion = function() return { ok = true, version = "v1" } end
end
-- YY_R69_MARKET_REDEEM_NBT_20261022
package.loaded["modules.yy_market_redeem_r69"]=nil
apply_module("modules.yy_market_redeem_r69")

-- ===== Muse AI 优化补丁 v2（2026-10-22）：纠错循环防误伤 / 标签自动闭合 =====
-- run_agent 的“声称完成”纠错只看当前轮：任务已用工具真实执行成功后，模型做总结会被误判为
-- “没调用工具就声称完成”，触发纠错并逼模型重复执行同一命令（给物品/杀凋零都中招）。
-- v2：记录每个玩家最近一次真实工具执行时间；纠错判定在该窗口内直接放行。
-- 另：模型偶尔漏写 [/LIST]、[/TIME] 闭合标签导致执行器取不到内容，这里自动补齐。
if not _G.__muse_ai_opt_v2 then
  _G.__muse_ai_opt_v2 = true
  __muse_tools_at = __muse_tools_at or {}
  local _muse_prev_pt = process_tools
  if type(_muse_prev_pt) == "function" then
    process_tools = function(text, playerName, admin, trace)
      local s = tostring(text or "")
      if s:find("[LIST]", 1, true) and not s:find("[/LIST]", 1, true) then s = s .. "[/LIST]" end
      if s:find("[TIME]", 1, true) and not s:find("[/TIME]", 1, true) then s = s .. "[/TIME]" end
      local out, used, promptOnly = _muse_prev_pt(s, playerName, admin, trace)
      if used and playerName then
        local now = 0
        pcall(function() now = tonumber(util.timestamp()) or 0 end)
        if now > 0 then __muse_tools_at[tostring(playerName)] = now end
      end
      return out, used, promptOnly
    end
  end
  local _muse_prev_claims = ai_reply_claims_done
  if type(_muse_prev_claims) == "function" then
    ai_reply_claims_done = function(text)
      local player = nil
      pcall(function() player = _CUR_AI_PLAYER end)
      if player then
        local at = __muse_tools_at[tostring(player)]
        if at then
          local now = 0
          pcall(function() now = tonumber(util.timestamp()) or 0 end)
          if now > 0 and (now - at) < 180 then return false end
        end
      end
      return _muse_prev_claims(text)
    end
  end
  plugin_api.museAiOptVersion2 = function() return { ok = true, version = "v2" } end
end


-- ===== Muse 修复 v4（2026-10-22）：普通玩家时间/天气命令自动转投票 =====
-- 模型习惯输出 [CMD]time set 1000[/CMD] 这类数字时刻，不在投票白名单内会被权限层拒绝，
-- 导致“调白天”永远失败。这里在执行前把非管理员的数字时刻/天气命令规范化为白名单 [VOTE]。
if not _G.__muse_ai_opt_v4 then
  _G.__muse_ai_opt_v4 = true
  local _muse_prev_pt4 = process_tools
  if type(_muse_prev_pt4) == "function" then
    local function muse_time_word(n)
      n = tonumber(n) or 0
      if n < 3000 then return "day" end
      if n < 9000 then return "noon" end
      if n < 16000 then return "night" end
      return "midnight"
    end
    process_tools = function(text, playerName, admin, cmd_limit, active_subprompts, task_mode)
      local s = tostring(text or "")
      if not admin then
        s = s:gsub("%[CMD%]%s*time%s+set%s+(%d+)%s*%[/CMD%]", function(num)
          return "[VOTE]time set " .. muse_time_word(num) .. "[/VOTE]"
        end)
        s = s:gsub("%[VOTE%]%s*time%s+set%s+(%d+)%s*%[/VOTE%]", function(num)
          return "[VOTE]time set " .. muse_time_word(num) .. "[/VOTE]"
        end)
        s = s:gsub("%[CMD%]%s*weather%s+(%a+)[^%]]*%[/CMD%]", function(w)
          local lw = tostring(w or ""):lower()
          if lw == "clear" or lw == "rain" or lw == "thunder" then
            return "[VOTE]weather " .. lw .. "[/VOTE]"
          end
          return nil
        end)
      end
      if admin then
        -- 管理员守卫（2026-10-22 用户规则）：管理员的时间/天气 [VOTE] 一律改直达 [CMD]，
        -- 除非脚本层显式投票通道刚放行过一次（标志匹配玩家且 60 秒内，消费后即清）。
        local vf = _G.__muse_admin_vote_ok
        if type(vf) == "table" and vf.player == playerName and (os.time() - (tonumber(vf.at) or 0)) <= 60 then
          _G.__muse_admin_vote_ok = nil
        else
          s = s:gsub("%[VOTE%]%s*time%s+set%s+(%a+)%s*%[/VOTE%]", function(w)
            local lw = tostring(w or ""):lower()
            if lw == "day" or lw == "night" or lw == "noon" or lw == "midnight" then
              return "[CMD]time set " .. lw .. "[/CMD]"
            end
            return nil
          end)
          s = s:gsub("%[VOTE%]%s*time%s+set%s+(%d+)%s*%[/VOTE%]", function(num)
            return "[CMD]time set " .. muse_time_word(num) .. "[/CMD]"
          end)
          s = s:gsub("%[VOTE%]%s*weather%s+(%a+)[^%]]*%[/VOTE%]", function(w)
            local lw = tostring(w or ""):lower()
            if lw == "clear" or lw == "rain" or lw == "thunder" then
              return "[CMD]weather " .. lw .. "[/CMD]"
            end
            return nil
          end)
        end
      end
      return _muse_prev_pt4(s, playerName, admin, cmd_limit, active_subprompts, task_mode)
    end
  end
  plugin_api.museAiOptVersion4 = function() return { ok = true, version = "v4" } end
end


-- ===== Muse 修复 v5（2026-10-22）：权限判定同样先做投票规范化 =====
-- run_agent 在 process_tools 之后还会用原文再判一次权限；v4 只改了执行文本，
-- 这里把判定入口也接上同一套规范化（数字时刻/天气 CMD 或 VOTE 先转白名单 VOTE 再判定）。
if not _G.__muse_ai_opt_v5 then
  _G.__muse_ai_opt_v5 = true
  local _muse_prev_dis5 = ai_ordinary_disallowed_tool
  if type(_muse_prev_dis5) == "function" then
    local function muse_time_word5(n)
      n = tonumber(n) or 0
      if n < 3000 then return "day" end
      if n < 9000 then return "noon" end
      if n < 16000 then return "night" end
      return "midnight"
    end
    local function muse_norm_vote(s)
      s = tostring(s or "")
      s = s:gsub("%[CMD%]%s*time%s+set%s+(%d+)%s*%[/CMD%]", function(num)
        return "[VOTE]time set " .. muse_time_word5(num) .. "[/VOTE]"
      end)
      s = s:gsub("%[VOTE%]%s*time%s+set%s+(%d+)%s*%[/VOTE%]", function(num)
        return "[VOTE]time set " .. muse_time_word5(num) .. "[/VOTE]"
      end)
      s = s:gsub("%[CMD%]%s*weather%s+(%a+)[^%]]*%[/CMD%]", function(w)
        local lw = tostring(w or ""):lower()
        if lw == "clear" or lw == "rain" or lw == "thunder" then
          return "[VOTE]weather " .. lw .. "[/VOTE]"
        end
        return nil
      end)
      return s
    end
    _G.__muse_norm_vote = muse_norm_vote
    ai_ordinary_disallowed_tool = function(text, playerName)
      return _muse_prev_dis5(muse_norm_vote(text), playerName)
    end
  end
  plugin_api.museAiOptVersion5 = function() return { ok = true, version = "v5" } end
end









-- YY_R70_REMOTE_STRUCT_MARKET_20261022
package.loaded["modules.yy_market_structure_r70"]=nil
apply_module("modules.yy_market_structure_r70")

-- ===== Muse 联网查资料兜底 v1.0（2026-10-22 安装）：脚本层未命中 + wiki/配方/种子/结构四类知识问题时，模型可先经 DeepSeek 官方 Search/Fetch 查证再回答 =====
-- 接口：POST https://api.deepseek.com/v1/tools/search 与 /v1/tools/fetch（同一把 deepseek_key）。
-- 费用：直调不在官方「11月1日前免费」范围（免费仅限经 Harness/Claude Code/Codex），每次成功调用 $0.75/1000 次，失败/空结果不计费。
-- 原 AI 模块与脚本直达层一行不动；本段整段删除 + 重载即完全回滚（无数据文件、无配置变更）。
if not _G.__muse_web_fallback_v1 then
  _G.__muse_web_fallback_v1 = true

  local MUSE_WEB_VERSION = "web-fallback-v1.0"
  -- 本宿主 Lua 方言无 table.pack/table.unpack（core.lua:8636 同款回退写法）
  local muse_web_unpack = (type(table.unpack) == "function" and table.unpack) or unpack

  -- ===== 可调数据（改这里即可，不动逻辑） =====
  local MUSE_WEB_ALLOW_ORDINARY = false -- 普通玩家是否可用（默认关：与现行 [SEARCH_WEB] 管理员姿态一致）
  local MUSE_WEB_MAX_SEARCH_PER_TASK = 2
  local MUSE_WEB_MAX_FETCH_PER_TASK = 2
  local MUSE_WEB_SEARCH_COOLDOWN = 15 -- 同一玩家两次 Search 的最小间隔（秒）
  local MUSE_WEB_SEARCH_COUNT = 5
  local MUSE_WEB_RESULT_MAXCHARS = 6000 -- Search 结果回喂上限
  local MUSE_WEB_FETCH_MAXCHARS = 8000 -- Fetch 正文回喂上限
  local MUSE_WEB_FETCH_OPEN_DOMAINS = { -- 未经本轮 Search 也允许直接抓的域名
    ["minecraft.wiki"] = true, ["zh.minecraft.wiki"] = true,
    ["www.minecraft.net"] = true, ["minecraft.net"] = true,
  }
  local MUSE_WEB_CAT_WORDS = { -- 类别名词（命中其一才可能触发）
    wiki = { "wiki", "维基", "图鉴", "百科" },
    recipe = { "配方", "合成表", "怎么合成", "如何合成", "怎么制作", "制作方法" },
    seed = { "种子", "seed" },
    structure = { "结构", "生成条件", "战利品", "宝藏" },
  }
  local MUSE_WEB_ASK_WORDS = { -- 求知标记（须与类别名词同时命中）
    "怎么", "什么", "为什么", "哪", "有没有", "多少", "掉落", "生成",
    "是什么", "攻略", "帮我查", "搜一下", "联网", "最新", "版本", "推荐",
  }
  local MUSE_WEB_VETO = { "不要查", "别查", "不用查", "不要搜", "别搜", "不想查", "不想搜" }

  -- ===== 内部状态（仅内存，随重载清空） =====
  local muse_web_hits = {} -- 命中日志（只记元数据，不存抓取正文）
  local muse_web_last_search = {} -- player -> os.time

  local function muse_web_log(player, kind, q, ok, cat)
    muse_web_hits[#muse_web_hits + 1] = {
      at = os.time(), player = player, kind = kind,
      q = tostring(q or ""):sub(1, 200), ok = ok and true or false, cat = cat or "",
    }
    while #muse_web_hits > 500 do table.remove(muse_web_hits, 1) end
  end

  local function muse_web_norm(s)
    s = tostring(s or ""):lower()
    s = s:gsub("[%s%p]", "")
    return s
  end

  -- 类别判定：返回类别名或 nil
  local function muse_web_category(question)
    local t = muse_web_norm(question)
    if t == "" then return nil end
    for _, v in ipairs(MUSE_WEB_VETO) do
      if t:find(muse_web_norm(v), 1, true) then return nil end
    end
    local ask = false
    for _, w in ipairs(MUSE_WEB_ASK_WORDS) do
      if t:find(muse_web_norm(w), 1, true) then ask = true break end
    end
    if not ask then return nil end
    for cat, words in pairs(MUSE_WEB_CAT_WORDS) do
      for _, w in ipairs(words) do
        if t:find(muse_web_norm(w), 1, true) then return cat end
      end
    end
    return nil
  end

  -- 密钥只在调用瞬间从配置读，不落盘、不进日志
  local function muse_web_key()
    local k = trim(tostring(game.getConfig("deepseek_key", "") or ""))
    if k == "" then k = trim(tostring(game.getConfig("web_search_key", "") or "")) end
    return k
  end

  local function muse_web_post(url, payload, key)
    local ok, r = pcall(http.post, url, http.json_encode(payload), {
      headers = { ["Content-Type"] = "application/json", ["Authorization"] = "Bearer " .. key },
      timeout = 20,
    })
    if not ok or not r or not r.ok then return nil, "网络请求失败" end
    local status = tonumber(r.status or 0) or 0
    if status < 200 or status >= 300 then
      if status == 402 then return nil, "HTTP 402（DeepSeek 账户余额不足）" end
      return nil, "HTTP " .. tostring(status)
    end
    local okd, data = pcall(http.json_decode, tostring(r.body or ""))
    if not okd or type(data) ~= "table" then return nil, "响应解析失败" end
    return data, nil
  end

  local function muse_web_search(ctx, player, query, ori)
    local q = trim(tostring(query or ""))
    if q == "" then return "联网查询失败：查询内容为空" end
    if utf8 and utf8.len and (utf8.len(q) or 0) > 128 then
      local off = utf8.offset(q, 129)
      q = q:sub(1, off and (off - 1) or #q)
    end
    local now = os.time()
    local last = muse_web_last_search[player] or 0
    if now - last < MUSE_WEB_SEARCH_COOLDOWN then
      muse_web_log(player, "search", q, false, ctx.cat)
      return "联网查询失败：查询过于频繁，请稍后再试"
    end
    local key = muse_web_key()
    if key == "" then
      muse_web_log(player, "search", q, false, ctx.cat)
      return "联网查询失败：未配置 DeepSeek 密钥"
    end
    muse_web_last_search[player] = now
    local data, err = muse_web_post("https://api.deepseek.com/v1/tools/search", {
      query = q, count = MUSE_WEB_SEARCH_COUNT, need_content = false,
      ori_question = trim(tostring(ori or "")):sub(1, 128),
    }, key)
    if not data then
      muse_web_log(player, "search", q, false, ctx.cat)
      return "联网查询失败：" .. tostring(err)
    end
    local results = data.results or {}
    if #results == 0 then
      muse_web_log(player, "search", q, true, ctx.cat)
      return "联网查询完成：没有找到相关结果。"
    end
    muse_web_log(player, "search", q, true, ctx.cat)
    local lines = { "联网查询结果（DeepSeek Search）：" }
    for i, item in ipairs(results) do
      local u = tostring(item.url or "")
      if u ~= "" then ctx.urls[u] = true end
      lines[#lines + 1] = string.format("%d. %s\n%s\n%s%s",
        i, tostring(item.title or ""), u,
        item.date and ("日期：" .. tostring(item.date) .. "\n") or "",
        tostring(item.summary or item.snippet or ""))
    end
    local out = table.concat(lines, "\n")
    if #out > MUSE_WEB_RESULT_MAXCHARS then out = out:sub(1, MUSE_WEB_RESULT_MAXCHARS) .. "\n（结果已截断）" end
    return out
  end

  local function muse_web_fetch(ctx, player, url)
    local u = trim(tostring(url or ""))
    if not (u:sub(1, 7) == "http://" or u:sub(1, 8) == "https://") or #u > 8192
      or u:find("@", 1, true) then
      muse_web_log(player, "fetch", u, false, ctx.cat)
      return "联网抓取失败：网址不合规"
    end
    local host = u:match("^https?://([^/]+)") or ""
    if not ctx.urls[u] and not MUSE_WEB_FETCH_OPEN_DOMAINS[host] then
      muse_web_log(player, "fetch", u, false, ctx.cat)
      return "联网抓取失败：只允许抓取本次搜索结果中的网址或 Minecraft 官方/wiki 站点"
    end
    local key = muse_web_key()
    if key == "" then
      muse_web_log(player, "fetch", u, false, ctx.cat)
      return "联网查询失败：未配置 DeepSeek 密钥"
    end
    local data, err = muse_web_post("https://api.deepseek.com/v1/tools/fetch", { url = u }, key)
    if not data then
      muse_web_log(player, "fetch", u, false, ctx.cat)
      return "联网抓取失败：" .. tostring(err)
    end
    muse_web_log(player, "fetch", u, true, ctx.cat)
    local body = tostring(data.content or "")
    if #body > MUSE_WEB_FETCH_MAXCHARS then body = body:sub(1, MUSE_WEB_FETCH_MAXCHARS) .. "\n（正文已截断）" end
    return "网页正文（DeepSeek Fetch）：《" .. tostring(data.title or "") .. "》\n" .. body
  end

  -- ===== 包装 1：run_agent —— 建立本轮 ctx（脚本层命中时根本不会进 LLM，本段零参与） =====
  local _muse_web_orig_run_agent = run_agent
  if type(_muse_web_orig_run_agent) == "function" then
    run_agent = function(question, playerName, commandMode, windowKey, privileged_override, opts)
      local prev = _G.__muse_web_ctx
      local cat = muse_web_category(question)
      if cat and type(playerName) == "string" and playerName ~= "" then
        local adm = false
        if type(is_admin) == "function" then
          local okc, v = pcall(is_admin, playerName)
          adm = okc and v == true
        end
        _G.__muse_web_ctx = {
          player = playerName, cat = cat, admin = adm,
          searches = 0, fetches = 0, urls = {},
          question = tostring(question or ""),
        }
      else
        _G.__muse_web_ctx = nil
      end
      local res = { pcall(_muse_web_orig_run_agent, question, playerName, commandMode, windowKey, privileged_override, opts) }
      _G.__muse_web_ctx = prev
      if not res[1] then error(res[2]) end
      return muse_web_unpack(res, 2, #res)
    end
  end

  -- ===== 包装 2：build_ai_system —— 仅 ctx 有效时追加查资料指令，其余轮次提示词不变；保留原函数全部返回值 =====
  local _muse_web_orig_build = build_ai_system
  if type(_muse_web_orig_build) == "function" then
    build_ai_system = function(q, admin)
      local s, intents = _muse_web_orig_build(q, admin)
      local ctx = _G.__muse_web_ctx
      if ctx and type(s) == "string" and (ctx.admin or MUSE_WEB_ALLOW_ORDINARY) then
        s = s .. "\n\n遇到你不确定或需要最新资料的 wiki、配方、种子、结构类知识问题，先联网查证再回答：\n"
          .. "[DSEARCH]查询词[/DSEARCH]：联网搜索（可带 site:zh.minecraft.wiki 限定来源）；摘要不够再 [DFETCH]结果网址[/DFETCH] 抓正文。\n"
          .. "回答时注明关键结论的来源；查不到或查询失败就直说，严禁用猜测冒充查证结果。与服务器实时状态有关的问题仍用游戏内真实执行器，不要用联网结果代替。"
      end
      return s, intents
    end
  end

  -- ===== 包装 3：process_tools —— 处理自有标签，其余全部委托原函数；完整保留原函数返回值 =====
  local _muse_web_orig_pt = process_tools
  if type(_muse_web_orig_pt) == "function" then
    process_tools = function(text, playerName, admin, cmd_limit, active_subprompts, task_mode)
      local res = { _muse_web_orig_pt(text, playerName, admin, cmd_limit, active_subprompts, task_mode) }
      local ctx = _G.__muse_web_ctx
      local results = res[1]
      -- ctx 不存在或玩家不符（非本段适用轮次）：标签原样透传，完全不干预
      if ctx and ctx.player == playerName and type(results) == "table" and type(text) == "string"
        and (text:find("[DSEARCH]", 1, true) or text:find("[DFETCH]", 1, true)) then
        local mine = false
        local function gate(kind)
          if not ctx.admin and not MUSE_WEB_ALLOW_ORDINARY then return "权限限制：联网查资料仅管理员可用" end
          if kind == "search" then
            if ctx.searches >= MUSE_WEB_MAX_SEARCH_PER_TASK then return "联网查询失败：本轮搜索次数已达上限" end
            ctx.searches = ctx.searches + 1
          else
            if ctx.fetches >= MUSE_WEB_MAX_FETCH_PER_TASK then return "联网抓取失败：本轮抓取次数已达上限" end
            ctx.fetches = ctx.fetches + 1
          end
          return nil
        end
        if text:find("[DSEARCH]", 1, true) then
          for _, q in ipairs(extract(text, "DSEARCH")) do
            local deny = gate("search")
            results[#results + 1] = deny or muse_web_search(ctx, playerName, q, ctx.question)
            mine = true
          end
        end
        if text:find("[DFETCH]", 1, true) then
          for _, u in ipairs(extract(text, "DFETCH")) do
            local deny = gate("fetch")
            results[#results + 1] = deny or muse_web_fetch(ctx, playerName, u)
            mine = true
          end
        end
        if mine then res[2] = true end
      end
      return muse_web_unpack(res, 1, #res)
    end
  end

  -- ===== 展示清洗：把自有标签登记进原生清洗表（摘除本段后随重载自动消失） =====
  if type(AI_MARKER_TAGS) == "table" then
    AI_MARKER_TAGS[#AI_MARKER_TAGS + 1] = "DSEARCH"
    AI_MARKER_TAGS[#AI_MARKER_TAGS + 1] = "DFETCH"
  end

  -- ===== 导出：命中日志与版本（与 museScriptHits 同形，供巡检收割） =====
  plugin_api = plugin_api or {}
  plugin_api.museWebVersion = function() return { ok = true, version = MUSE_WEB_VERSION } end
  plugin_api.museWebHits = function(clear)
    local n = #muse_web_hits
    local out = {}
    for i, h in ipairs(muse_web_hits) do out[i] = h end
    if clear then muse_web_hits = {} end
    return { ok = true, count = n, hits = out }
  end
end
-- ===== Muse Web Fallback END (v1.0) =====


-- ===== Muse 管线守卫 fix3（2026-10-22）：无祈使意图时剥离时间/天气标签 =====
-- 用户拍板规则：玩家原句没有明确的改时间/改天气祈使意图时，模型（及任何落回通道）发出的
-- 时间/天气变更标签在执行前一律剥离，回复其余部分照常作聊天发出；有祈使意图的真请求原样
-- 放行，交给真正的 AI 去办。判定与脚本层同源：复用脚本层导出的 __muse_sl_timewx_scan
--（fix2 加固逻辑 + “来场X”裸目标祈使），绝不另设一套判定。
-- 与 v4 守卫的分工：v4 在内层管“管理员 [VOTE] 转直达、普通玩家数字时刻规范化”；本守卫在
-- 最外层先管“原句没祈使时根本不许动”——先于 v4 剥离，v4 只会看到放行后的文本。
-- 例外：管理员显式投票的一次性放行标志（__muse_admin_vote_ok，玩家匹配且 60 秒内）在，
-- 则不剥 [VOTE]，且不消费标志（留给 v4 按原逻辑消费）。
-- 覆盖面：本服实际承载时间/天气变更的只有 [VOTE] 与 [CMD]（无 [TIMESET] 标签；[TIME] 是
-- 只读时间查询，不在剥离范围）。[CMD] 块内只剥纯 time set / weather 条目，混有的其他合法
-- 动作原样保留；整块全为时间/天气时整块摘除。
-- fail-open：拿不到该玩家当前原句（ctx 缺失或超过 600 秒）时放行并记 failopen，绝不误伤。
-- 留痕：每次剥离/放行/ fail-open 都记入 __muse_guard_hits（上限 200 条滚动），导出 museGuardHits。
if not _G.__muse_tw_guard_v1 then
  _G.__muse_tw_guard_v1 = true
  _G.__muse_guard_ctx = _G.__muse_guard_ctx or {}
  _G.__muse_guard_hits = _G.__muse_guard_hits or {}
  local function guard_log(rec)
    local h = _G.__muse_guard_hits
    rec.at = os.time()
    h[#h + 1] = rec
    while #h > 200 do table.remove(h, 1) end
  end

  local function guard_is_timewx_cmd(seg)
    local c = tostring(seg or ""):gsub("^%s+", ""):gsub("%s+$", ""):lower():gsub("^/", "")
    if c:match("^time%s+set%s+") or c == "time set" then return true end
    if c:match("^weather%s+") or c == "weather" then return true end
    return false
  end

  -- Same-source local scan (the script layer used to export __muse_sl_timewx_scan
  -- for this). Only judges time/weather imperative intent in the player's own
  -- words: explicit vote + target passes; strong set-verb + target passes;
  -- report-form (already done, no request marker) does not.
  local function guard_local_timewx(rawq, isCmd)
    local q = tostring(rawq or ""):lower()
    q = q:gsub("[%s%p]", ""):gsub("，", ""):gsub("。", ""):gsub("？", ""):gsub("！", ""):gsub("：", ""):gsub("；", ""):gsub("、", ""):gsub("（", ""):gsub("）", ""):gsub("“", ""):gsub("”", "")
    if q == "" then return false end
    local function has(sub) return q:find(sub, 1, true) ~= nil end
    local function any(list) for _, w in ipairs(list) do if has(w) then return true end end return false end
    if not any({ "白天", "天亮", "清晨", "早晨", "早上", "晚上", "夜晚", "夜里", "黑夜", "天黑", "中午", "正午", "半夜", "午夜", "下雨", "雨天", "晴天", "放晴", "天晴", "雷雨", "打雷", "雷暴", "停雨", "timeset", "weather" }) then return false end
    if any({ "刚才", "已经", "刚刚", "之前" }) and not any({ "请", "帮我", "给我", "麻烦", "投票" }) then return false end
    if has("投票") then return true end
    if any({ "停雨", "别下了", "不要下雨", "雨停" }) then return true end
    if q:match("调到%d+") or q:match("调成%d+") or q:match("设为%d+") then return true end
    if any({ "调", "改", "设", "换", "弄成", "搞成", "变成" }) then return true end
    if any({ "来场", "来一场" }) then return true end
    if isCmd then return true end
    if #q <= 24 and (has("吧") or has("呗")) then return true end
    return false
  end

  local function guard_strip(s, stripped)
    s = s:gsub("%[VOTE%](.-)%[/VOTE%]", function(inner)
      local c = tostring(inner or ""):gsub("^%s+", ""):gsub("%s+$", ""):lower()
      if c:match("^time%s+set") or c:match("^weather") then
        stripped[#stripped + 1] = "[VOTE]" .. tostring(inner) .. "[/VOTE]"
        return ""
      end
      return nil
    end)
    s = s:gsub("%[CMD%](.-)%[/CMD%]", function(inner)
      local kept = {}
      local dropped = false
      for seg in tostring(inner or ""):gmatch("[^;\n]+") do
        local t = seg:gsub("^%s+", ""):gsub("%s+$", "")
        if t ~= "" and guard_is_timewx_cmd(t) then
          dropped = true
          stripped[#stripped + 1] = "[CMD]" .. t .. "[/CMD]"
        elseif t ~= "" then
          kept[#kept + 1] = t
        end
      end
      if not dropped then return nil end
      if #kept == 0 then return "" end
      return "[CMD]" .. table.concat(kept, ";") .. "[/CMD]"
    end)
    return s
  end

  local _guard_prev_run_agent = run_agent
  if type(_guard_prev_run_agent) == "function" then
    run_agent = function(question, playerName, commandMode, windowKey, privileged_override, opts)
      if playerName then
        _G.__muse_guard_ctx[playerName] = { text = tostring(question or ""), cmd = (commandMode == true), at = os.time() }
      end
      return _guard_prev_run_agent(question, playerName, commandMode, windowKey, privileged_override, opts)
    end
  end

  local _guard_prev_pt = process_tools
  if type(_guard_prev_pt) == "function" then
    process_tools = function(text, playerName, admin, cmd_limit, active_subprompts, task_mode)
      local s = tostring(text or "")
      if not (s:find("[VOTE]", 1, true) or s:find("[CMD]", 1, true)) then
        return _guard_prev_pt(s, playerName, admin, cmd_limit, active_subprompts, task_mode)
      end
      local ctx = playerName and _G.__muse_guard_ctx[playerName]
      if not ctx or (os.time() - (tonumber(ctx.at) or 0)) > 600 then
        guard_log({ kind = "failopen", player = tostring(playerName or ""), why = ctx and "stale" or "noctx" })
        return _guard_prev_pt(s, playerName, admin, cmd_limit, active_subprompts, task_mode)
      end
      local vf = _G.__muse_admin_vote_ok
      if admin and type(vf) == "table" and vf.player == playerName and (os.time() - (tonumber(vf.at) or 0)) <= 60 then
        guard_log({ kind = "pass", why = "vote_flag", player = tostring(playerName), q = ctx.text })
        return _guard_prev_pt(s, playerName, admin, cmd_limit, active_subprompts, task_mode)
      end
      -- 2026-10-25 删层后注（F5，纠正描述不改行为）：脚本层已删，__muse_sl_timewx_scan
      -- 不存在，imp 恒 false；此处不再是“扫描后跳过不管”，而是一律走 guard_strip
      -- 剥离标签后放行（含 [VOTE]/[CMD] 的模型输出时间/天气祈使内容被中性化，
      -- guard_log 记 kind="strip"）；行为与 10-24 删层同款。
      local imp = false
      if type(_G.__muse_sl_timewx_scan) == "function" then
        local okc, res = pcall(_G.__muse_sl_timewx_scan, ctx.text, playerName, ctx.cmd == true)
        if okc and res then imp = true end
      else
        local okc2, res2 = pcall(guard_local_timewx, ctx.text, ctx.cmd == true)
        if okc2 and res2 then imp = true end
      end
      if imp then
        guard_log({ kind = "pass", why = "imperative", player = tostring(playerName), q = ctx.text })
        return _guard_prev_pt(s, playerName, admin, cmd_limit, active_subprompts, task_mode)
      end
      local stripped = {}
      local s2 = guard_strip(s, stripped)
      if #stripped > 0 then
        guard_log({ kind = "strip", player = tostring(playerName), q = ctx.text, tags = table.concat(stripped, " | ") })
      end
      return _guard_prev_pt(s2, playerName, admin, cmd_limit, active_subprompts, task_mode)
    end
  end
  plugin_api.museGuardVersion = function() return { ok = true, version = "tw-guard-v1" } end
  plugin_api.museGuardHits = function(clear)
    local out = {}
    for i, v in ipairs(_G.__muse_guard_hits) do out[i] = v end
    if clear then _G.__muse_guard_hits = {} end
    return { ok = true, count = #out, hits = out }
  end
end
-- ===== Muse 管线守卫 fix3 END =====
-- R77: account-pool observer exclusions + final chat trigger compatibility.
-- Keep this AFTER all historical chat wrappers and Muse handlers.
package.loaded["modules.yy_secondary_bot_guard_r77"]=nil
apply_module("modules.yy_secondary_bot_guard_r77")
package.loaded["modules.yy_trigger_router_r77"]=nil
apply_module("modules.yy_trigger_router_r77")
-- ===== Muse 审查系统 v2（2026-10-23 修订）：脏话禁言 + 信用分 + 巡查/审查合并 AI + 受限管理员(op)守卫 =====
-- BEGIN Muse Review System v2
if not _G.__muse_review_v2 then
_G.__muse_review_v2 = true

local RS_DB_FILE = game.dataDir().. "/review_system.json"
local RS_FEED_FILE = game.dataDir().. "/ai_logs/event_feed.json"
local RS_PROF_LADDER = {30, 60, 300, 600}
local RS_PROF_RESET = 12 * 3600
local RS_EXPLAIN_GRACE = 900
local RS_DAILY_REGEN = 5
local RS_APPEAL_COMP = 10
local RS_CONSENT_TTL = 600
local RS_OP_GIVE_CAP = 16
local RS_BAN_MAX = 300
local RS_BAN_COOLDOWN = 600
local RS_NEW_PLAYER_SPAN = 7 * 86400
local RS_PROF_WORDS = {
"傻逼", "傻屄", "煞笔", "沙比", "你妈的", "他妈的", "她妈的", "你妈逼",
"妈的", "妈逼", "草泥马", "操你", "肏你", "屌", "鸡巴", "婊子",
"贱逼", "贱人", "王八蛋", "狗日的", "脑残", "智障", "白痴",
"nmsl", "cnm", "wcnm",
}
local RS_WATCH = {
{id = "diamond", th = 20}, {id = "diamond_block", th = 2},
{id = "netherite_ingot", th = 3}, {id = "netherite_block", th = 1},
{id = "ancient_debris", th = 5}, {id = "beacon", th = 1},
{id = "enchanted_golden_apple", th = 3}, {id = "nether_star", th = 1},
{id = "elytra", th = 1}, {id = "dragon_egg", th = 1},
}
local RS_MERGED_PERSONA = table.concat({
"你是本服务器的审查 AI，巡查与裁决是你的两个阶段：巡查阶段，你每 10 分钟自动巡查一次，读取服务器精选日志（聊天、进入、离开、死亡、击杀）与在线玩家背包快照，筛出可疑变化（特珍贵物资无来源暴增、疑似刷物、非权限创造模式等）并立案；审查阶段，你对自己立案的案件与玩家举报案件作最终裁决。一切信用分增减只能由你在审查阶段的裁决产生，巡查阶段只立案、永不扣分。",
"你的长期记忆就是你自己维护的审查记录（案件卷宗、玩家备忘、信用变化）。每次判断前先看随附的历史记录与玩家备忘：不得对同一件事重复定罪，不得与既往结论自相矛盾；玩家已经解释并结案的物资变动，后续不得就同一批物资重复立案，除非出现新的增量。记录必须写清时间、对象、做了什么、依据是什么、最后判定是什么，经得起管理员事后逐条对质。",
"管理员（疣螈）是服务器最高权限者。他通过 .巡查AI 对你说的话是最高权威指令，不是普通咨询——这是他与你对话的唯一窗口。他通常会让你做：把扣错的信用分加回去、撤销某个案件、更正之前的结论、查询某个玩家的案底与信用状况。收到这类指令时：先核对你自己的记录确认对象与数额，然后执行，并把执行动作写入审查记录，最后向管理员回报清楚。你无权拒绝管理员的纠错指令；记录里查不到的东西要直说没有，不许编造。",
}, "\n")

local rs_db = nil
local function rs_now()
local t = nil
pcall(function() t = tonumber(util.timestamp()) end)
return t or os.time()
end
local function rs_fmt_time(at)
return os.date("!%m-%d %H:%M", (tonumber(at) or 0) + 28800)
end
local function rs_fmt_dur(s)
s = tonumber(s) or 0
if s >= 3600 then return tostring(math.floor(s / 3600)).. " 小时" end
if s >= 60 then return tostring(math.floor(s / 60)).. " 分钟" end
return tostring(s).. " 秒"
end
local function rs_db_load()
if rs_db then return rs_db end
local d = nil
pcall(function() local ok, v = pcall(util.json_load, RS_DB_FILE); if ok and type(v) == "table" then d = v end end)
rs_db = d or {}
rs_db.credit = rs_db.credit or {}
rs_db.credit_log = rs_db.credit_log or {}
rs_db.profanity = rs_db.profanity or {}
rs_db.cases = rs_db.cases or {}
rs_db.case_seq = rs_db.case_seq or 0
rs_db.snapshots = rs_db.snapshots or {}
rs_db.ledger = rs_db.ledger or {}
rs_db.notes = rs_db.notes or {}
rs_db.patrol = rs_db.patrol or {last_run = 0}
rs_db.first_seen = rs_db.first_seen or {}
rs_db.explained = rs_db.explained or {}
rs_db.activity = rs_db.activity or {}
rs_db.consent = rs_db.consent or {}
rs_db.op_bans = rs_db.op_bans or {}
rs_db.func_votes = rs_db.func_votes or {}
rs_db.op_kick_pass = rs_db.op_kick_pass or {}
rs_db.token_stats = rs_db.token_stats or {total = 0, patrol = 0, verdict = 0, window = 0, calls = 0}
rs_db.ai_sessions = rs_db.ai_sessions or {}
return rs_db
end
local function rs_db_save()
if rs_db then pcall(util.json_write, RS_DB_FILE, rs_db) end
end

local function rs_token_last()
if type(AI_USAGE) == "table" then return tonumber(AI_USAGE.last_call) or 0 end
return 0
end
local function rs_token_add(bucket, tok)
tok = tonumber(tok) or 0
if tok <= 0 then return end
local d = rs_db_load()
d.token_stats = d.token_stats or {total = 0, patrol = 0, verdict = 0, window = 0, calls = 0}
local ts = d.token_stats
ts.total = (tonumber(ts.total) or 0) + tok
ts[bucket] = (tonumber(ts[bucket]) or 0) + tok
ts.calls = (tonumber(ts.calls) or 0) + 1
rs_db_save()
end
-- ===== 巡查AI 模型（在.AI模型「巡查」里切换；rs_db.ai_model 存模型 ID，nil = 跟随主体AI 当前模型，与改造前行为一致）=====
local function rs_ai_model()
local d = rs_db_load()
local m = d.ai_model
if type(m) == "string" and m ~= "" then
if type(AI_MODEL_CHOICES) == "table" and not AI_MODEL_CHOICES[m] then return nil end
return m
end
return nil
end
local function rs_ai_model_label()
local m = rs_ai_model()
if m then
if type(model_display) == "function" then local ok, v = pcall(model_display, m) if ok and v then return tostring(v) end end
return m
end
local cur = nil
if type(ai_active_model) == "function" then local ok, v = pcall(ai_active_model) if ok then cur = v end end
if cur and type(model_display) == "function" then local ok, v = pcall(model_display, cur) if ok and v then return tostring(v).. "（跟随主体AI）" end end
if cur then return tostring(cur).. "（跟随主体AI）" end
return "跟随主体AI"
end
local function rs_ai_model_set(id)
local d = rs_db_load()
id = trim(tostring(id or ""))
if id == "" then
d.ai_model = nil
rs_db_save()
return true
end
if type(AI_MODEL_CHOICES) == "table" and not AI_MODEL_CHOICES[id] then return false end
d.ai_model = id
rs_db_save()
return true
end

-- UTF-8 感知截断（L2）：按字符数截断，绝不切断多字节中文字符；
-- 非法字节按单字节推进（不死循环）。返回 ≤maxchars 字符，超长补 "…"。
local function muse_utf8_trunc(s, maxchars)
  local n = #s
  local pos = 1
  local count = 0
  local out_end = 0
  while pos <= n do
    local b = s:byte(pos)
    local len = 1
    if b >= 240 then len = 4
    elseif b >= 224 then len = 3
    elseif b >= 192 then len = 2 end
    count = count + 1
    if count > maxchars then break end
    out_end = pos + len - 1
    if out_end > n then out_end = n end
    pos = pos + len
  end
  if count > maxchars then return s:sub(1, out_end) .. "…" end
  return s
end

local function rs_err_summary(e)
local s = tostring(e or "")
s = s:gsub("[\r\n]+", " ")
local file, ln = s:match("^%s*(%S+):(%d+):%s*")
if file and ln then s = s:sub(#file + #ln + 3) end
s = s:gsub("§.", "")
s = trim(s)
s = muse_utf8_trunc(s, 120)
if s == "" then s = "未知错误" end
return s
end

local function rs_credit_score(name)
local d = rs_db_load()
return tonumber(d.credit[name]) or 100
end
local function rs_credit_threshold(score)
score = tonumber(score) or 100
if score >= 90 then return 5
elseif score >= 70 then return 4
elseif score >= 50 then return 3
elseif score >= 30 then return 2
else return 1 end
end
local function rs_ledger_add(actor, target, action, basis, verdict)
local d = rs_db_load()
d.ledger[#d.ledger + 1] = {at = rs_now(), actor = actor, target = target, action = action, basis = basis or "", verdict = verdict or ""}
while #d.ledger > 400 do table.remove(d.ledger, 1) end
end
local function rs_note_add(name, text)
local d = rs_db_load()
local n = d.notes[name] or {}
n[#n + 1] = {at = rs_now(), text = text}
while #n > 20 do table.remove(n, 1) end
d.notes[name] = n
end
local function rs_credit_add(name, delta, reason, source)
local d = rs_db_load()
local before = tonumber(d.credit[name]) or 100
local after = before + (tonumber(delta) or 0)
if after > 100 then after = 100 end
if after < 0 then after = 0 end
d.credit[name] = after
d.credit_log[#d.credit_log + 1] = {at = rs_now(), player = name, delta = after - before, after = after, reason = reason or "", source = source or ""}
while #d.credit_log > 500 do table.remove(d.credit_log, 1) end
rs_ledger_add(source or "系统", name, "信用分 ".. before.. " → ".. after, reason or "", "")
rs_db_save()
return after
end
local function rs_history_text(name)
local d = rs_db_load()
local lines = {}
lines[#lines + 1] = "玩家 ".. name.. " 当前信用分 ".. tostring(rs_credit_score(name)).. "。"
local notes = d.notes[name] or {}
if #notes > 0 then
lines[#lines + 1] = "玩家备忘："
for i = math.max(1, #notes - 5), #notes do
lines[#lines + 1] = "- ".. rs_fmt_time(notes[i].at).. " ".. tostring(notes[i].text)
end
end
local cs = {}
for _, c in ipairs(d.cases) do if c.target == name then cs[#cs + 1] = c end end
if #cs > 0 then
lines[#lines + 1] = "历史案件："
for i = math.max(1, #cs - 4), #cs do
local c = cs[i]
lines[#lines + 1] = "- #".. tostring(c.id).. " ".. tostring(c.kind).. " 状态 ".. tostring(c.status).. "：".. tostring(c.summary or "").. "；判定：".. tostring(c.verdict or "未判定").. (((c.explanation or "") ~= "") and ("；玩家解释：".. tostring(c.explanation)) or "")
end
end
if #notes == 0 and #cs == 0 then lines[#lines + 1] = "无历史案件与备忘记录。" end
return table.concat(lines, "\n")
end

_G.muse_credit_score = rs_credit_score
_G.muse_credit_threshold = rs_credit_threshold
_G.muse_credit_add = rs_credit_add
_G.muse_ledger_add = rs_ledger_add
_G.muse_review_history_text = rs_history_text
_G.muse_review_model_get = rs_ai_model
_G.muse_review_model_label = rs_ai_model_label
_G.muse_review_model_set = rs_ai_model_set

-- 已知玩家名全集（定向查询/口令/卷宗附带的匹配范围）
local function rs_known_players()
local d = rs_db_load()
local set, list = {}, {}
local function add(nm)
if type(nm) == "string" and nm ~= "" and not set[nm] then set[nm] = true list[#list + 1] = nm end
end
for nm in pairs(d.first_seen) do add(nm) end
for nm in pairs(d.activity) do add(nm) end
for nm in pairs(d.notes) do add(nm) end
for nm in pairs(d.credit) do add(nm) end
for nm in pairs(d.snapshots) do add(nm) end
for _, cs in ipairs(d.cases) do add(cs.target) end
for _, e in ipairs(d.ledger) do add(e.target) end
local hist = (d.patrol and d.patrol.history) or {}
for _, r in ipairs(hist) do
if type(r.scanned) == "table" then for _, nm in ipairs(r.scanned) do add(nm) end end
if type(r.handled) == "table" then for _, nm in ipairs(r.handled) do add(nm) end end
end
table.sort(list, function(a, b) return #a > #b end)
return list, set
end
-- 名字命中判定（防子串误命中）：名字须单独成句，或与意图词（查/撤销/补回…）出现在同一分句
local RS_NAME_INTENT = {"查", "信用", "撤销", "撤回", "补回", "加回", "恢复", "案件", "处罚", "案底", "记录", "多少"}
local function rs_name_hit(text, nm)
if not nm or nm == "" then return false end
local tmp = tostring(text or "")
for _, sep in ipairs({"，", "。", "；", "！", "？", "、", ",", ";", "!", "%?", "\n"}) do
tmp = tmp:gsub(sep, "\n")
end
for clause in (tmp.. "\n"):gmatch("([^\n]*)\n") do
if clause:find(nm, 1, true) then
if trim(clause) == nm then return true end
for _, w in ipairs(RS_NAME_INTENT) do
if clause:find(w, 1, true) then return true end
end
end
end
return false
end
-- 玩家完整卷宗：当前分 + 备忘/案件史（rs_history_text）+ 信用流水近 10 条；单份 ≤1400 字符
local function rs_player_dossier(nm)
local d = rs_db_load()
local lines = {}
lines[#lines + 1] = "[玩家卷宗] ".. nm
lines[#lines + 1] = rs_history_text(nm)
local logs = {}
for _, e in ipairs(d.credit_log) do if e.player == nm then logs[#logs + 1] = e end end
if #logs > 0 then
lines[#lines + 1] = "信用流水（近 10 条）："
for i = math.max(1, #logs - 9), #logs do
local e = logs[i]
lines[#lines + 1] = "- ".. rs_fmt_time(e.at).. " ".. ((tonumber(e.delta) or 0) >= 0 and "+" or "").. tostring(e.delta).. " → ".. tostring(e.after).. "（".. tostring(e.reason or "").. "）"
end
end
local text = table.concat(lines, "\n")
if #text > 1400 then
local cut = text:sub(1, 1400)
local nl = cut:find("\n[^\n]*$")
if nl then cut = cut:sub(1, nl) end
text = cut.. "\n（卷宗过长已截断）"
end
return text
end
-- 撤销案件的唯一执行函数：确定性口令与模型 ACTION REVOKE 共用（X1）。
-- 幂等保护 = 案件 status 与 case.restored 标志：status 已 revoked、或扣分已补回（restored=true）时绝不重复补回，重复触发只回「已撤销」回执。
local function rs_revoke_case(case, reason)
if not case then return nil end
if case.status == "revoked" then
local back = tonumber(case.deducted) or 0
if back > 0 and case.restored then
return "案件 #".. tostring(case.id).. "（".. tostring(case.target).. "）此前已撤销、已补回 ".. back.. " 分，不再重复执行"
end
return "案件 #".. tostring(case.id).. "（".. tostring(case.target).. "）此前已撤销，不再重复执行"
end
case.status = "revoked"
local back = 0
if (tonumber(case.deducted) or 0) > 0 and not case.restored then
back = tonumber(case.deducted)
rs_credit_add(case.target, back, "案件 #".. case.id.. " 被管理员撤销，补回扣分", "巡查AI")
case.restored = true
end
rs_ledger_add("管理员", case.target, "撤销案件 #".. case.id, reason or "", "已撤销")
rs_note_add(case.target, "案件 #".. case.id.. " 被管理员撤销")
rs_db_save()
if back > 0 then
return "已撤销案件 #".. tostring(case.id).. "（".. tostring(case.target).. "），补回 ".. back.. " 分，当前 ".. rs_credit_score(case.target).. " 分"
end
return "已撤销案件 #".. tostring(case.id).. "（".. tostring(case.target).. "），该案未扣分"
end
-- 确定性数据摘要（模型不可用时的降级答复内容）
-- 注意：本函数定义位置在 rs_patrol_status_text 之前，不能直接引用它（会解析成全局 nil）；
-- 巡查状态文本改走段尾已装配的 plugin_api.museReviewPatrolStatus 晚绑定获取（运行时段已完整加载）。
local function rs_facts_digest()
local d = rs_db_load()
local lines = {}
local st = ""
pcall(function() local r = plugin_api.museReviewPatrolStatus() if type(r) == "table" and r.text then st = r.text end end)
lines[#lines + 1] = st
local ne = {}
for nm, sc in pairs(d.credit) do
if tonumber(sc) ~= 100 then ne[#ne + 1] = nm.. "：".. tostring(sc).. " 分" end
end
lines[#lines + 1] = "信用分不是 100 的玩家：".. (#ne > 0 and table.concat(ne, "、") or "无")
local op = {}
for _, cs in ipairs(d.cases) do
if cs.status == "open" or cs.status == "escalated" or cs.status == "explained" then op[#op + 1] = "#".. tostring(cs.id).. " ".. tostring(cs.target).. "（".. tostring(cs.status).. "）" end
end
lines[#lines + 1] = "未结案件：".. (#op > 0 and table.concat(op, "、") or "无")
if #d.credit_log > 0 then
lines[#lines + 1] = "最近信用变动："
for i = math.max(1, #d.credit_log - 4), #d.credit_log do
local e = d.credit_log[i]
lines[#lines + 1] = "- ".. rs_fmt_time(e.at).. " ".. tostring(e.player).. " ".. ((tonumber(e.delta) or 0) >= 0 and "+" or "").. tostring(e.delta).. " → ".. tostring(e.after).. "（".. tostring(e.reason or "").. "）"
end
end
return table.concat(lines, "\n")
end

-- ---------- 脏话检测与递增禁言 ----------
local function rs_prof_normalize(s)
s = lower(tostring(s or ""))
s = s:gsub("[%s%p]", "")
return s
end
local function rs_prof_hit(msg)
local n = rs_prof_normalize(msg)
for _, w in ipairs(RS_PROF_WORDS) do
if n:find(w, 1, true) then return w end
end
return nil
end
local function rs_apply_mute(name, seconds)
if (tonumber(seconds) or 0) <= 0 then return end
pcall(function()
local cd = chat_rate_db()
cd.mutes = cd.mutes or {}
local until_t = rs_now() + seconds
local cur = tonumber(cd.mutes[name]) or 0
if cur > until_t then until_t = cur end
cd.mutes[name] = until_t
chat_rate_save(cd)
end)
pcall(game.sendCommand, 'tag "'.. tostring(name):gsub('"', '\\"').. '" add 禁言')
end
local function rs_on_profanity(name, hit)
local d = rs_db_load()
local rec = d.profanity[name]
if type(rec) ~= "table" or (rs_now() - (tonumber(rec.last_at) or 0)) > RS_PROF_RESET then
rec = {count = 0, last_at = 0}
end
rec.count = (tonumber(rec.count) or 0) + 1
rec.last_at = rs_now()
d.profanity[name] = rec
local secs = RS_PROF_LADDER[math.min(rec.count, #RS_PROF_LADDER)]
rs_ledger_add("系统", name, "脏话检出（第 ".. rec.count.. " 次）", "聊天命中词表（词目不外显）", "警告并禁言 ".. rs_fmt_dur(secs))
rs_note_add(name, "说脏话被检出第 ".. rec.count.. " 次")
rs_db_save()
rs_apply_mute(name, secs)
pcall(send, name, "§c[审查系统] 你刚才的话里有不文明用词（第 ".. rec.count.. " 次记录，本计数每 12 小时重置）。\n§7处理：警告，并禁言 §e".. rs_fmt_dur(secs).. "§7（原因：聊天出现不文明用词）。\n§7禁言阶梯：第 1 次 30 秒 → 第 2 次 1 分钟 → 第 3 次 5 分钟 → 第 4 次起 10 分钟。")
end

-- ---------- 巡查：背包快照与可疑扫描 ----------
local function rs_item_name(id)
if type(item_display_name) == "function" then
local ok, v = pcall(item_display_name, id)
if ok and v then return tostring(v) end
end
return id
end
local function rs_collect_inventory(name)
local safe = tostring(name):gsub('"', '\\"')
local ok, inv = pcall(game.queryInventory, '@a[name="'.. safe.. '"]')
if not ok or type(inv) ~= "table" or type(inv.items) ~= "table" then
ok, inv = pcall(player.getInventory, name)
end
if not ok or type(inv) ~= "table" or type(inv.items) ~= "table" then return nil end
local counts = {}
for _, it in ipairs(inv.items) do
local id = nil
if type(item_base_id) == "function" then
local okb, b = pcall(item_base_id, it)
if okb then id = b end
end
id = lower(tostring(id or it.id or it.name or "")):gsub("^minecraft:", "")
local c = tonumber(it.count or it.Count or it.amount or it.Amount or it.quantity or it.Quantity) or 0
if id ~= "" and c > 0 then counts[id] = (counts[id] or 0) + c end
end
return counts
end
local function rs_scan_player(name, counts, gamemode)
local d = rs_db_load()
local snap = d.snapshots[name]
local cands = {}
local admin = false
pcall(function() admin = is_admin(name) == true end)
if gamemode == "creative" and not admin then
cands[#cands + 1] = {type = "creative", text = "非管理权限玩家处于创造模式"}
end
local first_at = tonumber(d.first_seen[name]) or rs_now()
local is_new = (rs_now() - first_at) <= RS_NEW_PLAYER_SPAN
if snap and type(snap.items) == "table" and counts then
local peaks = (type(d.explained[name]) == "table") and d.explained[name] or {}
for _, w in ipairs(RS_WATCH) do
local before = math.max(tonumber(snap.items[w.id]) or 0, tonumber(peaks[w.id]) or 0)
local after = tonumber(counts[w.id]) or 0
local th = w.th
if is_new then th = math.max(1, math.floor(th / 2)) end
local delta = after - before
if delta >= th then
cands[#cands + 1] = {type = "surge", item = w.id, before = before, after = after, delta = delta,
text = rs_item_name(w.id).. " 从 ".. before.. " 增至 ".. after.. "（+".. delta.. "）"}
end
end
end
return cands, snap, is_new
end
local function rs_feed_lines_about(name, since)
local out = {}
local ok, fd = pcall(util.json_load, RS_FEED_FILE)
if ok and type(fd) == "table" and type(fd.logs) == "table" then
for _, e in ipairs(fd.logs) do
if (tonumber(e.at) or 0) >= since and (tostring(e.player) == name or tostring(e.text or ""):find(name, 1, true)) then
out[#out + 1] = rs_fmt_time(e.at).. " [".. tostring(e.kind).. "] ".. tostring(e.player).. "：".. tostring(e.text)
if #out >= 12 then break end
end
end
end
return out
end
local function rs_transfer_evidence(name, since)
local hits = {}
for _, l in ipairs(rs_feed_lines_about(name, since)) do
if l:find("送", 1, true) or l:find("交易", 1, true) or l:find("转账", 1, true) or lower(l):find("give", 1, true) then
hits[#hits + 1] = l
end
end
return hits
end
local function rs_find_open_case(name)
local d = rs_db_load()
local found = nil
for _, c in ipairs(d.cases) do
if c.target == name and (c.status == "open" or c.status == "escalated") then found = c end
end
return found
end

local function rs_patrol_ai_review(name, cand, snap_at)
local lines = {}
lines[#lines + 1] = "玩家：".. name.. "（信用分 ".. rs_credit_score(name).. "，非管理员）"
lines[#lines + 1] = "基线快照时间：".. (snap_at and rs_fmt_time(snap_at) or "无（首次建档）").. "；本轮可疑变化："
for _, c in ipairs(cand) do lines[#lines + 1] = "- ".. c.text end
local tr = rs_transfer_evidence(name, snap_at or (rs_now() - 900))
lines[#lines + 1] = "窗口内涉及该玩家的赠送/交易类日志：".. (#tr > 0 and "" or "未发现")
for _, l in ipairs(tr) do lines[#lines + 1] = " ".. l end
lines[#lines + 1] = rs_history_text(name)
lines[#lines + 1] = "请判断这些变化是否可疑到需要立案让玩家解释。正常游玩（挖矿、交易、他人赠送、管理员发放）能解释的变化判“排除”；无来源暴增、疑似刷物、非权限创造判“立案”。"
lines[#lines + 1] = "输出格式：第一行只写 立案 或 排除；第二行写 理由：一句话。"
local ok, res = pcall(call_ai, {{role = "system", content = RS_MERGED_PERSONA}, {role = "user", content = table.concat(lines, "\n")}}, 60, rs_ai_model(), 800)
if not ok or type(res) ~= "string" then return false, "巡查 AI 调用失败，按排除处理" end
rs_token_add("patrol", rs_token_last())
local first = res:match("^%s*([^\n]+)") or ""
local reason = res:match("理由：([^\n]+)") or res:match("理由:([^\n]+)") or ""
if first:find("立案", 1, true) and not first:find("排除", 1, true) then
return true, trim(reason)
end
return false, trim(reason)
end

local function rs_open_case(name, cand, review_reason, severe, is_new)
local d = rs_db_load()
d.case_seq = d.case_seq + 1
local parts = {}
local items = {}
local has_surge = false
for _, c in ipairs(cand) do
parts[#parts + 1] = c.text
if c.type == "surge" then has_surge = true items[#items + 1] = {id = c.item, before = c.before, after = c.after} end
end
local case = {
id = d.case_seq, kind = severe and "巡查立案（严重）" or "巡查立案", target = name,
created_at = rs_now(), status = severe and "escalated" or "open",
category = severe and "severe" or (has_surge and "wind" or "other"),
new_player = is_new and true or false,
items = items,
summary = table.concat(parts, "；").. (is_new and "（新玩家，风控阈值减半）" or ""),
basis = "巡查阶段复核（审查 AI）：".. tostring(review_reason or ""),
credit_at_open = rs_credit_score(name),
explain_deadline = rs_now() + RS_EXPLAIN_GRACE,
explanation = "", verdict = "", ai_verdict = "", applied = "", deducted = 0, restored = false,
judge_prompt = "", notify_text = "",
}
d.cases[#d.cases + 1] = case
while #d.cases > 200 do table.remove(d.cases, 1) end
rs_ledger_add("审查AI", name, "立案 #".. case.id, case.summary, severe and "严重案件，直接进入审查阶段裁决" or "已私发提示，等待解释")
rs_note_add(name, "被巡查立案 #".. case.id.. "：".. case.summary)
local msg
if severe then
msg = "§c[巡查] 检测到严重异常：§f".. case.summary.. "\n§7本案已直接转交审查 AI 裁决。你仍可输入 §f.审查解释 你的说明 §7进行申诉。"
else
msg = "§e[巡查] 检测到你的物资出现异常变化：§f".. case.summary.. "\n§7请在 §e15 分钟§7 内输入 §f.审查解释 你的说明 §7解释来源（例如谁送的、在哪挖的）。\n§c逾期不解释将转审查 AI 裁决；是否扣分、扣多少由审查 AI 决定（风控案件成立第 1、2 次各警告并扣 20 分）。"
end
case.notify_text = msg
rs_db_save()
pcall(send, name, msg)
return case
end

-- ---------- 审查 AI 裁决（巡查阶段转交案件） ----------
local function rs_mark_peaks(case)
if type(case.items) ~= "table" then return end
local d = rs_db_load()
local pk = d.explained[case.target] or {}
for _, it in ipairs(case.items) do
if (tonumber(pk[it.id]) or 0) < (tonumber(it.after) or 0) then pk[it.id] = it.after end
end
d.explained[case.target] = pk
end
local function rs_wind_upheld_count(name, exclude_id)
local d = rs_db_load()
local n = 0
for _, c in ipairs(d.cases) do
if c.target == name and c.category == "wind" and c.status == "closed_punished" and c.id ~= exclude_id then n = n + 1 end
end
return n
end
local function rs_adjudicate(case, explanation)
local name = case.target
local rules = ""
if type(review_unified_rules) == "function" then
local ok, v = pcall(review_unified_rules)
if ok and type(v) == "string" then rules = v end
end
local prompt = table.concat({
"",
"来源：本案由你在巡查阶段立案（巡查与裁决是你的两个阶段），证据、立案依据与历史记录如下；现在进入审查阶段，由你作最终裁决。一切扣分只能在这一步的裁决里产生。",
rules,
"",
"案件编号：".. tostring(case.id),
"被审查玩家：".. name,
"信用分：".. rs_credit_score(name).. "（立案时 ".. tostring(case.credit_at_open or 100).. "）",
"风控成立史：此前已成立 ".. tostring(rs_wind_upheld_count(name, case.id)).. " 次（本案若是风控案件且成立，为其第 ".. tostring(rs_wind_upheld_count(name, case.id) + 1).. " 次）",
"异常摘要：".. tostring(case.summary or ""),
"巡查依据：".. tostring(case.basis or ""),
"玩家解释：".. ((explanation and explanation ~= "") and explanation or "（未提供解释）"),
rs_history_text(name),
"请裁决。最后一行只能输出以下之一：CLEAR（不成立，结案）/ DEDUCT（成立，扣分结案）/ MUTE（成立，扣分并禁言 1 小时）/ KICK（成立，扣分并踢出）/ BAN（成立，扣分并封禁 2 小时）。扣分额度由系统按规则计算，你不需要写分值：风控（物资异常）案件成立时，第 1、2 次各扣 20 分并以警告为主（系统会把禁言/踢出/封禁自动降级为警告），第 3 次起按你选的措施执行并扣 40 分；严重案件（非权限创造等）成立扣 40 分并按你选的措施执行。最后一行之前可以写一两句理由。",
}, "\n")
case.judge_prompt = prompt
local ok, res = pcall(call_ai, {{role = "system", content = RS_MERGED_PERSONA}, {role = "user", content = prompt}}, 60, rs_ai_model(), 900)
if ok and type(res) == "string" then rs_token_add("verdict", rs_token_last()) end
local verdict = "CLEAR"
local raw = ""
if ok and type(res) == "string" then
raw = res
local last = ""
for line in res:gmatch("[^\r\n]+") do last = line end
last = trim(last):upper()
for _, v in ipairs({"BAN", "KICK", "MUTE", "DEDUCT", "CLEAR"}) do
if last:find(v, 1, true) then verdict = v break end
end
end
case.judge_raw = raw:sub(1, 1000)
return verdict
end

local function rs_apply_verdict(case, verdict)
-- 2026-10-23 纵深补强（用户批准）：服主/机器人为受保护对象，裁决落点前复查，命中直接作废、不落任何处罚。
local _prot = false
if case and type(case.target) == "string" then
if case.target == "疣螈" then _prot = true
elseif type(_G.muse_is_self_account) == "function" then
local _ok, _r = pcall(_G.muse_is_self_account, case.target)
_prot = _ok and _r == true
end
end
if _prot then
case.verdict = "VOID"
case.applied = "受保护对象，裁决作废（未执行处罚）"
case.status = "closed_clear"
rs_ledger_add("审查AI", case.target, "案件 #" .. tostring(case.id) .. " 裁决作废", case.summary or "", "命中受保护对象守卫，未扣分未处罚")
rs_db_save()
return
end
local name = case.target
-- 2026-10-23 状态可见性：裁决计数（巡查轮次历史统计用；本函数是巡查案件裁决的唯一落点，随本函数尾部的 rs_db_save 落盘）
local _pd = rs_db_load()
_pd.patrol.verdicts_total = (tonumber(_pd.patrol.verdicts_total) or 0) + 1
case.ai_verdict = verdict
if verdict == "CLEAR" then
case.verdict = "CLEAR"
case.applied = "不成立结案"
case.status = "closed_clear"
rs_mark_peaks(case)
rs_ledger_add("审查AI", name, "案件 #".. case.id.. " 裁决：不成立", case.summary, "CLEAR 结案；当前信用分 ".. rs_credit_score(name))
pcall(send, name, "§a[审查] 你的案件 #".. case.id.. " 已裁决：不成立，结案。当前信用分 ".. rs_credit_score(name).. "。")
rs_db_save()
return
end
local strike = nil
if case.category == "wind" then strike = rs_wind_upheld_count(name, case.id) + 1 end
local deduct = 40
local warn_only = false
if strike and strike <= 2 then
deduct = 20
warn_only = true
case.verdict = verdict.. "（风控第 ".. strike.. " 次成立，降级为警告）"
case.applied = "第 ".. strike.. " 次警告 + 扣 20 分"
else
case.verdict = verdict
case.applied = "扣 40 分".. (verdict == "MUTE" and " + 禁言 1 小时" or verdict == "KICK" and " + 踢出" or verdict == "BAN" and " + 封禁 2 小时" or "")
end
if (tonumber(case.deducted) or 0) <= 0 then
rs_credit_add(name, -deduct, "案件裁决 ".. verdict.. "（#".. case.id.. "）", "审查AI")
case.deducted = deduct
end
case.status = "closed_punished"
rs_mark_peaks(case)
local now_score = rs_credit_score(name)
if warn_only then
pcall(send, name, "§c[审查] 案件 #".. case.id.. " 裁决成立（第 ".. strike.. " 次警告）：扣 ".. deduct.. " 信用分，当前信用分 ".. now_score.. "。如有异议可用 .审查解释 申诉。")
elseif verdict == "MUTE" then
rs_apply_mute(name, 3600)
pcall(send, name, "§c[审查] 案件 #".. case.id.. " 裁决成立：扣 ".. deduct.. " 信用分并禁言 1 小时，当前信用分 ".. now_score.. "。如有异议可用 .审查解释 申诉。")
elseif verdict == "KICK" then
pcall(game.sendCommand, 'kick "'.. tostring(name):gsub('"', '\\"').. '" §c审查裁决：行为异常，已踢出。可用 .审查解释 申诉。')
pcall(send, name, "§c[审查] 案件 #".. case.id.. " 裁决成立：扣 ".. deduct.. " 信用分并踢出服务器，当前信用分 ".. now_score.. "。如有异议可用 .审查解释 申诉。")
elseif verdict == "BAN" then
local _rs_unban_at = nil
pcall(function() local _dd0 = load_data(); if type(muse_latest_unban) == "function" then _rs_unban_at = muse_latest_unban(_dd0, name) end end)
local _rs_skip = _rs_unban_at ~= nil and (tonumber(case.created_at) or 0) <= _rs_unban_at
if _rs_skip then
if type(log_event) == "function" then pcall(log_event, "UNBAN_REVOKE", "skip rs verdict re-ban " .. tostring(name)) end
pcall(send, name, "§c[审查] 案件 #".. case.id.. " 裁决成立：扣 ".. deduct.. " 信用分，当前信用分 ".. now_score.. "。你已被管理员解封，本次裁决不追加封禁。")
else
pcall(function()
local dd = load_data()
dd.blacklist = dd.blacklist or {}
dd.blacklist[name] = {until_time = rs_now() + 7200, by = "审查AI（巡查转交）", duration = "2小时"}
save_data(dd)
end)
pcall(game.sendCommand, 'kick "'.. tostring(name):gsub('"', '\\"').. '" §c审查裁决成立，已封禁2小时。')
pcall(send, name, "§c[审查] 案件 #".. case.id.. " 裁决成立：扣 ".. deduct.. " 信用分并封禁 2 小时，当前信用分 ".. now_score.. "。")
pcall(broadcast, "§c[审查] §f".. name.. " §7经审查裁决成立，已封禁 2 小时。")
end
else
pcall(send, name, "§c[审查] 案件 #".. case.id.. " 裁决成立：扣 ".. deduct.. " 信用分，当前信用分 ".. now_score.. "。如有异议可用 .审查解释 申诉。")
end
rs_ledger_add("审查AI", name, "案件 #".. case.id.. " 裁决：".. tostring(case.applied), case.summary, "当前信用分 ".. now_score)
rs_note_add(name, "案件 #".. case.id.. " 被审查 AI 裁定 ".. verdict)
rs_db_save()
end

local function rs_on_explain(name, text)
local case = rs_find_open_case(name)
if not case then
local d = rs_db_load()
local last = nil
for _, c in ipairs(d.cases) do
if c.target == name and c.status == "closed_punished" then last = c end
end
if last and not last.appealed then
last.appealed = true
last.explanation = text
last.status = "escalated"
rs_ledger_add("系统", name, "案件 #".. last.id.. " 收到申诉解释", text, "重新裁决")
rs_db_save()
pcall(send, name, "§e[审查] 申诉已收到，正在重新裁决。")
local v = rs_adjudicate(last, text)
if v == "CLEAR" then
if (tonumber(last.deducted) or 0) > 0 and not last.restored then
rs_credit_add(name, last.deducted, "申诉成功补回（#".. last.id.. "）", "审查AI")
rs_credit_add(name, RS_APPEAL_COMP, "申诉成功补偿（#".. last.id.. "）", "审查AI")
last.restored = true
end
last.status = "closed_clear"
last.verdict = "CLEAR（申诉）"
rs_ledger_add("审查AI", name, "案件 #".. last.id.. " 申诉裁决：无问题", text, "扣分已补回")
pcall(send, name, "§a[审查] 你的申诉已通过：案件 #".. last.id.. " 撤销，扣分已补回并另有 ".. RS_APPEAL_COMP.. " 分补偿，当前信用分 ".. rs_credit_score(name).. "。")
rs_db_save()
else
rs_apply_verdict(last, v)
end
return true
end
pcall(send, name, "§e[审查] 你目前没有待解释或可申诉的案件。")
return true
end
case.explanation = text
case.status = "explained"
rs_ledger_add("系统", name, "案件 #".. case.id.. " 收到解释", text, "转审查 AI 裁决")
rs_db_save()
pcall(send, name, "§e[审查] 解释已收到，正在交审查 AI 裁决。")
local v = rs_adjudicate(case, text)
rs_apply_verdict(case, v)
return true
end

local function rs_sweep_cases()
local d = rs_db_load()
local t = rs_now()
for _, c in ipairs(d.cases) do
if c.status == "open" and (tonumber(c.explain_deadline) or 0) <= t then
c.status = "escalated"
rs_ledger_add("审查AI", c.target, "案件 #".. c.id.. " 逾期未解释", c.summary, "转审查阶段裁决（巡查阶段不扣分，扣分由审查决定）")
rs_note_add(c.target, "案件 #".. c.id.. " 逾期未解释，已转审查裁决")
rs_db_save()
local v = rs_adjudicate(c, "")
rs_apply_verdict(c, v)
end
end
end

-- 解封合一（2026-10-24）：巡查 tick 顺带撤销「解封前立案」的在途案件。事实源是 ds_data 的
-- report_history 解封记录（kind="UNBAN"，core 解封动作写入）——两套封禁共用同一份数据，
-- 不再另立豁免表。已申诉（appealed）的重审案件不碰，留给申诉流程；撤销只结案、不动信用分。
local function rs_unban_revoke_sweep()
local okd, dd = pcall(load_data)
if not okd or type(dd) ~= "table" or type(dd.report_history) ~= "table" then return end
local latest = {}
for _, h in ipairs(dd.report_history) do
if type(h) == "table" and h.kind == "UNBAN" and type(h.target) == "string" then
local at = tonumber(h.at) or 0
if not latest[h.target] or at > latest[h.target] then latest[h.target] = at end
end
end
if not next(latest) then return end
local d = rs_db_load()
local changed = false
for _, c in ipairs(d.cases or {}) do
local ua = type(c.target) == "string" and latest[c.target] or nil
if ua and (tonumber(c.created_at) or 0) <= ua
and (c.status == "open" or c.status == "explained" or (c.status == "escalated" and not c.appealed)) then
c.status = "closed_clear"
c.verdict = "VOID（管理员已解封，立案撤销）"
c.applied = "解封撤销，未执行处罚"
rs_ledger_add("审查AI", c.target, "案件 #".. tostring(c.id).. " 解封撤销", c.summary or "", "管理员已解封该玩家，本案立案早于解封，撤销结案")
rs_note_add(c.target, "案件 #".. tostring(c.id).. " 因管理员解封被撤销")
changed = true
end
end
if changed then rs_db_save() end
end

-- 机器人账号豁免名单（来源：工具箱账号池 profile_role=bot，2026-10-23 经用户指令加入）。
-- 池内机器人账号与机器人本体一样，不是风控对象：巡查不立案、不建快照、不记首见。
local RS_BOT_ACCOUNTS = {
["儒雅猎杀者"] = true,
["冷酷清纯车速"] = true,
["火神靠谱军团"] = true,
["野有心皮卡"] = true,
}
local function rs_is_self_account(name)
-- 2026-10-23 装机修复：机器人本体（连接级 OP、无管理员标签、常驻创造模式）不是巡查对象，
-- 否则首轮巡查会把布汲岛误判为「非权限创造」严重案件（活服实测已误扣 40 分，已清理）。
-- 2026-10-23 晚追加：账号池机器人账号（RS_BOT_ACCOUNTS）同等豁免——儒雅猎杀者曾被第 9 轮巡查
-- 当普通新玩家以「非权限创造」严重通道立案 #1 并扣 40 分（误判，已删案回分清理）。
if RS_BOT_ACCOUNTS[name] then return true end
if type(ai_bot_identity) ~= "function" then return false end
local ok, v = pcall(ai_bot_identity)
return ok and v ~= nil and tostring(v) ~= "" and tostring(v) == name
end
local function rs_process_scan(name, counts, gamemode)
if rs_is_self_account(name) then return {} end
local d = rs_db_load()
if not d.first_seen[name] then d.first_seen[name] = rs_now() rs_db_save() end
local cand, snap, is_new = rs_scan_player(name, counts, gamemode)
d.snapshots[name] = {at = rs_now(), items = counts, gamemode = gamemode}
rs_db_save()
if #cand == 0 then return cand end
if rs_find_open_case(name) then return cand end
local severe = false
for _, c in ipairs(cand) do if c.type == "creative" then severe = true end end
local suspect, reason = rs_patrol_ai_review(name, cand, snap and snap.at or nil)
if suspect then
local case = rs_open_case(name, cand, reason, severe, is_new)
if severe then
local v = rs_adjudicate(case, "")
rs_apply_verdict(case, v)
end
else
local parts = {}
for _, c in ipairs(cand) do parts[#parts + 1] = c.text end
rs_ledger_add("巡查AI", name, "巡查候选排除", table.concat(parts, "；"), reason)
rs_db_save()
end
return cand
end

local function rs_patrol_round(force)
local d = rs_db_load()
local t = rs_now()
if not force and t - (tonumber(d.patrol.last_run) or 0) < 600 then return end
d.patrol.last_run = t
rs_db_save()
local scanned = 0
local scanned_names = {}
for _, name in ipairs(online_players() or {}) do
local admin = false
pcall(function() admin = is_admin(name) == true end)
if not admin then
local counts = rs_collect_inventory(name)
if counts then
scanned = scanned + 1
scanned_names[#scanned_names + 1] = name
local gm = nil
pcall(function() gm = detect_gamemode(name) end)
pcall(rs_process_scan, name, counts, gm)
end
end
end
d.patrol.last_note = "扫描 ".. scanned.. " 名在线非管理员玩家"
d.patrol.last_scanned = scanned_names
rs_db_save()
end

local function rs_daily_regen()
local d = rs_db_load()
local today = math.floor(rs_now() / 86400)
local last = tonumber(d.patrol.last_credit_day) or 0
if last == 0 then d.patrol.last_credit_day = today rs_db_save() return end
if today <= last then return end
d.patrol.last_credit_day = today
rs_db_save()
for nm, sc in pairs(d.credit) do
if (tonumber(sc) or 100) < 100 then rs_credit_add(nm, RS_DAILY_REGEN, "每日回血", "系统") end
end
end
-- ---------- 巡查状态可见性（2026-10-23）：轮次/历史/误判统计，供 .巡查AI 问答 ----------
local function rs_misjudge_stats(d)
-- 误判口径（自案件台账推导，不另存原始数据）：申诉成功 = 裁决被申诉推翻（verdict 为 CLEAR（申诉））；管理员撤销 = status 为 revoked。总案件数 = case_seq 累计立案数。
local ap, rv = 0, 0
for _, c in ipairs(d.cases or {}) do
if c.status == "revoked" then rv = rv + 1 end
if c.verdict == "CLEAR（申诉）" then ap = ap + 1 end
end
return ap, rv, tonumber(d.case_seq) or 0
end
local function rs_patrol_status_text()
local d = rs_db_load()
local p = d.patrol or {}
local now = rs_now()
local lines = {}
lines[#lines + 1] = "【巡查状态】（以下为系统实时统计，回答管理员关于巡查轮次、上次/下次时间、处理过哪些人、有没有误判时，只能依据这里的数字与名单，没有记录的就直说没有，不许估算或编造）"
lines[#lines + 1] = "巡查轮次：已完成 ".. tostring(tonumber(p.round_no) or 0).. " 轮（自状态统计上线后累计）。"
local lr = tonumber(p.last_run) or 0
if lr > 0 then
lines[#lines + 1] = "上一轮巡查时间：".. rs_fmt_time(lr).. "（约 ".. rs_fmt_dur(now - lr).. "前）。"
local remain = 600 - (now - lr)
if remain > 0 then
lines[#lines + 1] = "下一轮巡查预计：约 ".. rs_fmt_dur(remain).. "后（按每 10 分钟一轮的节拍估算，实际会受服务器调度影响，有 ±几分钟偏差）。"
else
lines[#lines + 1] = "下一轮巡查预计：节拍时间已到，随时可能开始（估算）。"
end
else
lines[#lines + 1] = "上一轮巡查时间：暂无记录。"
lines[#lines + 1] = "下一轮巡查预计：约 10 分钟内（估算）。"
end
local h = p.history or {}
if #h > 0 then
lines[#lines + 1] = "最近巡查记录（时间 / 扫描到的玩家 / 本轮处理名单 / 立案数 / 裁决数）："
for i = math.max(1, #h - 4), #h do
local r = h[i]
local names = "无"
if type(r.scanned) == "table" and #r.scanned > 0 then names = table.concat(r.scanned, "、") end
local handled = "无"
if type(r.handled) == "table" and #r.handled > 0 then handled = table.concat(r.handled, "、") end
lines[#lines + 1] = "- 第 ".. tostring(r.round or "?").. " 轮 ".. rs_fmt_time(r.at).. "：扫描 ".. names.. "；处理 ".. handled.. "；立案 ".. tostring(r.opened or 0).. " 件；裁决 ".. tostring(r.verdicts or 0).. " 件"
end
else
lines[#lines + 1] = "最近巡查记录：暂无（统计自本次更新后开始积累）。"
end
local ap, rv, tot = rs_misjudge_stats(d)
lines[#lines + 1] = "误判统计：累计立案 ".. tot.. " 件；其中申诉成功（原裁决被推翻）".. ap.. " 件、被管理员撤销 ".. rv.. " 件。"
return table.concat(lines, "\n")
end
local function rs_patrol_tick()
local _d0 = rs_db_load()
local _seq0 = tonumber(_d0.case_seq) or 0
local _ver0 = tonumber(_d0.patrol.verdicts_total) or 0
local _run0 = tonumber(_d0.patrol.last_run) or 0
local _t0 = rs_now()
pcall(rs_daily_regen)
pcall(rs_sweep_cases)
pcall(rs_unban_revoke_sweep)
pcall(rs_patrol_round, false)
local d = rs_db_load()
if (tonumber(d.patrol.last_run) or 0) ~= _run0 then
-- 本次 tick 真正跑了一轮巡查：记账（轮次、历史、误判快照）
d.patrol.round_no = (tonumber(d.patrol.round_no) or 0) + 1
d.patrol.last_run_at = d.patrol.last_run
local handled, seen_h = {}, {}
for _, e in ipairs(d.ledger or {}) do
if (tonumber(e.at) or 0) >= _t0 and type(e.action) == "string" and (e.action:find("立案", 1, true) or e.action:find("裁决", 1, true)) and e.target and not seen_h[e.target] then
seen_h[e.target] = true
handled[#handled + 1] = e.target
end
end
d.patrol.history = d.patrol.history or {}
d.patrol.history[#d.patrol.history + 1] = {
at = d.patrol.last_run, round = d.patrol.round_no,
scanned = d.patrol.last_scanned or {}, opened = (tonumber(d.case_seq) or 0) - _seq0,
verdicts = (tonumber(d.patrol.verdicts_total) or 0) - _ver0, handled = handled,
}
while #d.patrol.history > 8 do table.remove(d.patrol.history, 1) end
local _ap, _rv, _tot = rs_misjudge_stats(d)
d.patrol.misjudge = {appeals_ok = _ap, revoked = _rv, total_cases = _tot}
rs_db_save()
end
end

-- ---------- 两套权限：受限管理员守卫（R8；2026-10-23 号主口径修订：原生 OP 优先） ----------
-- 原生 OP = 宿主框架 player.isOP（服务器原生 OP，与 core 权限自检第④项「框架 isOP」同一信号）。
-- 有原生 OP 的玩家（无论有没有 tag）以原生为准：完整管理员，不算受限管理员、不受本节守卫限制；
-- 只有 tag/名单层权限、没有原生 OP 的，才是受限管理员。
local function rs_is_super(name)
if type(SUPERUSER) == "string" and SUPERUSER ~= "" then return name == SUPERUSER end
return name == "疣螈"
end
local function rs_native_op(name)
if type(player) == "table" and type(player.isOP) == "function" then
local ok, v = pcall(player.isOP, name)
if ok and v == true then return true end
end
return false
end
local function rs_is_op(name)
if rs_is_super(name) then return false end
if rs_native_op(name) then return false end
local ok, v = pcall(check_tag, name, "op")
if ok and v == true then return true end
local ok2, v2 = pcall(is_admin, name)
return ok2 and v2 == true
end
local function rs_is_privileged(name)
-- 权限玩家（.function 投票提示收件人 / .同意 投票资格）：超管 + 原生 OP + 受限管理员
return rs_is_super(name) or rs_native_op(name) or rs_is_op(name)
end
local function rs_touch_activity(name)
local d = rs_db_load()
local t = rs_now()
if t - (tonumber(d.activity[name]) or 0) < 60 then return end
d.activity[name] = t
rs_db_save()
end
local function rs_reporter_active_count(reporters)
local d = rs_db_load()
local n = 0
for nm, _ in pairs(reporters or {}) do
local at = tonumber(d.activity[nm])
if at then
if rs_now() - at <= 7 * 86400 then n = n + 1 end
else
n = n + 1
end
end
return n
end
local function rs_consent_grant(name, scope)
local d = rs_db_load()
local c = d.consent[name] or {}
if scope == "inv" or scope == "all" then c.inv = rs_now() + RS_CONSENT_TTL end
if scope == "pos" or scope == "all" then c.pos = rs_now() + RS_CONSENT_TTL end
d.consent[name] = c
rs_db_save()
end
local function rs_consent_revoke(name)
local d = rs_db_load()
d.consent[name] = nil
rs_db_save()
end
local function rs_consent_ok(name, scope)
local d = rs_db_load()
local c = d.consent[name]
if type(c) ~= "table" then return false end
return (tonumber(c[scope]) or 0) > rs_now()
end
local RS_VALUABLE = {}
for _, w in ipairs(RS_WATCH) do RS_VALUABLE[w.id] = true end
local function rs_str_is_player_ref(x, actor)
if x == "" or x == "@s" then return false end
if x:match("^%-?%d") or x:sub(1, 1) == "~" or x:sub(1, 1) == "^" then return false end
return true
end
local function rs_op_guard(actor, cmd)
if rs_is_super(actor) then return {allow = true, na = true} end
if not rs_is_op(actor) then return {allow = true, na = true} end
local c = lower(tostring(cmd or "")):gsub("^/", ""):gsub("^%s+", "")
local head = c:match("^(%S+)") or ""
local function deny(reason)
rs_ledger_add("op守卫", actor, "拦截命令", tostring(cmd), reason)
rs_db_save()
return {allow = false, reason = reason}
end
if head == "op" or head == "deop" then
return deny("受限管理员不得给任何人授予或撤销 OP 权限（包括自己）。")
end
if head == "gamemode" then
local mode, target = c:match("^gamemode%s+(%S+)%s*(%S*)")
local m = lower(mode or "")
local creative = (m == "creative" or m == "c" or m == "1")
local tgt = target or ""
if creative and (tgt == "" or tgt == "@s" or lower(tgt) == lower(actor)) then
return deny("受限管理员不得给自己开创造模式。")
end
return {allow = true}
end
if head == "tp" or head == "teleport" then
local a1, a2 = c:match("^%S+%s+(%S+)%s*(%S*)")
if a1 then
local self_move = (lower(a1) == lower(actor) or a1 == "@s")
if self_move and a2 ~= "" and rs_str_is_player_ref(a2, actor) then
return deny("受限管理员不得把自己传送到其他玩家的坐标。")
end
if a2 == "" and rs_str_is_player_ref(a1, actor) and lower(a1) ~= lower(actor) then
return deny("受限管理员不得把自己传送到其他玩家的坐标。")
end
end
return {allow = true}
end
if head == "kick" then
local target = c:match("^kick%s+(%S+)") or ""
local d = rs_db_load()
local pass = target ~= "" and tonumber(d.op_kick_pass[actor.. "|".. target]) or nil
if pass and pass > rs_now() then
d.op_kick_pass[actor.. "|".. target] = nil
rs_db_save()
return {allow = true, via = "kickpass"}
end
rs_ledger_add("op守卫", actor, "拦截踢人（待审查 AI 裁量）", tostring(cmd), "无一次性踢人许可")
rs_db_save()
return {allow = false, reason = "受限管理员踢人需先经审查 AI 判断合理（暂无一次性许可），已记案待裁量。"}
end
if head == "give" then
local _, item, amount = c:match("^give%s+(%S+)%s+(%S+)%s*(%d*)")
local id = lower(tostring(item or "")):gsub("^minecraft:", "")
local amt = tonumber(amount) or 1
if RS_VALUABLE[id] and amt > RS_OP_GIVE_CAP then
return deny("受限管理员一次发放高级物资不得超过 ".. RS_OP_GIVE_CAP.. " 个（本次 ".. amt.." 个）。")
end
return {allow = true}
end
return {allow = true}
end
local function rs_ban_guard(actor)
if rs_is_super(actor) then return nil end
if not rs_is_op(actor) then return nil end
local d = rs_db_load()
local last = tonumber(d.op_bans[actor]) or 0
if rs_now() - last < RS_BAN_COOLDOWN then
return {allow = false, reason = "受限管理员封禁冷却中，还需 ".. (RS_BAN_COOLDOWN - (rs_now() - last)).. " 秒。", max_seconds = RS_BAN_MAX}
end
d.op_bans[actor] = rs_now()
rs_db_save()
return {allow = true, max_seconds = RS_BAN_MAX}
end
local function rs_notify_privileged(text, except)
local sent = {}
local function push(nm)
if nm and nm ~= "" and nm ~= except and not sent[nm] then
sent[nm] = true
pcall(send, nm, text)
end
end
if type(SUPERUSER) == "string" then push(SUPERUSER) end
push("疣螈")
pcall(function()
for _, nm in ipairs(online_players() or {}) do
if rs_is_privileged(nm) then push(nm) end
end
end)
end
local function rs_function_gate(actor, label)
if rs_is_super(actor) then return "allow" end
if not rs_is_op(actor) then return "allow" end
local d = rs_db_load()
local t = rs_now()
local ap = d.func_votes.approved
if type(ap) == "table" and ap.actor == actor and t - (tonumber(ap.at) or 0) <= 120 then
d.func_votes.approved = nil
rs_db_save()
return "allow"
end
local v = d.func_votes.current
if type(v) == "table" and t - (tonumber(v.at) or 0) > 120 then d.func_votes.current = nil v = nil end
if type(v) ~= "table" then
d.func_votes.current = {actor = actor, label = tostring(label or ""), at = t, votes = {[actor] = true}}
rs_db_save()
rs_notify_privileged("§e[function 投票] 受限管理员 ".. actor.. " 申请运行命令链「".. tostring(label or "").."」，需要另一名 op 在 2 分钟内在聊天里发 .同意 才会放行；本提示仅权限玩家可见。", actor)
return "pending"
end
rs_db_save()
return "pending"
end
_G.muse_is_super = rs_is_super
_G.muse_is_op = rs_is_op
_G.muse_is_native_op = rs_native_op
_G.muse_is_privileged = rs_is_privileged
_G.muse_touch_activity = rs_touch_activity
_G.muse_reporter_active_count = rs_reporter_active_count
_G.muse_consent_ok = rs_consent_ok
_G.muse_op_guard = rs_op_guard
_G.muse_ban_guard = rs_ban_guard
_G.muse_function_gate = rs_function_gate
_G.muse_is_self_account = rs_is_self_account

-- ---------- .巡查AI（管理员与合并后的审查 AI 对话的唯一窗口） ----------
-- ===== 长回复分段（借鉴主体AI agent_send_long 的按行打包思路 + send_long 的长行硬切兜底）=====
-- 规则：全文（含前缀）超过 RS_SEND_LIMIT 个字（按 UTF-8 字符计）才分段；正文按行打包、每段 ≤200 字，
-- 单行超长时按字符硬切（不切断多字节字符）。tail 是代码追加的尾部行（执行回执之外的 token 统计行等），
-- 永不拆开：能并入最后一段（合计 ≤200 字）就并入，否则自成最后一段完整发出。
local RS_SEND_LIMIT = 200
local RS_SEND_MAXSEG = 8
local function rs_utf8_len(s)
local n = 0
for _ in tostring(s):gmatch("[%z\1-\127\194-\244][\128-\191]*") do n = n + 1 end
return n
end
local function rs_split_chars(s, limit)
local chunks, cur, n = {}, "", 0
for c in tostring(s):gmatch("[%z\1-\127\194-\244][\128-\191]*") do
n = n + 1
if n > limit then chunks[#chunks + 1] = cur cur = c n = 1 else cur = cur.. c end
end
if cur ~= "" then chunks[#chunks + 1] = cur end
return chunks
end
local function rs_split_body(text, limit)
local segs, buf, bufn = {}, "", 0
local function flush() if buf ~= "" then segs[#segs + 1] = buf buf = "" bufn = 0 end end
for raw_line in (tostring(text).. "\n"):gmatch("(.-)\n") do
local line = raw_line:gsub("\r$", "")
local ln = rs_utf8_len(line)
if ln > limit then
flush()
for _, ch in ipairs(rs_split_chars(line, limit)) do segs[#segs + 1] = ch end
elseif buf == "" then buf = line bufn = ln
elseif bufn + 1 + ln <= limit then buf = buf.. "\n".. line bufn = bufn + 1 + ln
else flush() buf = line bufn = ln end
end
flush()
return segs
end
local rs_delivery = {segments=0,confirmed=0,failed=0}
local rs_outbox={}
local rs_sending=false
local rs_step
function museReviewDeliveryStep() if rs_step then rs_step() end end
local function rs_wire_commands(admin,text)
local selector = "@a[name="..http.json_encode(tostring(admin)).."]"
local function command(s) return "tellraw "..selector.." "..http.json_encode({rawtext={{text=s}}}) end
local base=#command("")
if base+12>180 then error("玩家名称编码后过长，无法安全投递消息") end
local commands,buf,bytes={},"",base
-- The wire limit is bytes after JSON escaping, not the number of displayed characters.
for c in tostring(text):gmatch("[%z\1-\127\194-\244][\128-\191]*") do
local cost=#http.json_encode(c)-2
if bytes+cost>180 and buf~="" then commands[#commands+1]=command(buf) buf="" bytes=base end
buf=buf..c bytes=bytes+cost
end
if buf~="" then commands[#commands+1]=command(buf) end
return commands
end
local function rs_deliver(admin,text)
local good,commands=pcall(rs_wire_commands,admin,text)
if not good then rs_delivery.failed=rs_delivery.failed+1 rs_delivery.error=tostring(commands) return false end
local status=rs_delivery
status.pending=(tonumber(status.pending) or 0)+#commands
for _,command in ipairs(commands) do rs_outbox[#rs_outbox+1]={command=command,status=status} end
if rs_sending then return true end
rs_sending=true
local function step()
local item=table.remove(rs_outbox,1)
if not item then rs_sending=false return end
local command=item.command
local rs_delivery=item.status
local ok, result = pcall(function()
return game.execute(command,{channel="ai",timeout_ms=2500})
end)
rs_delivery.segments=rs_delivery.segments+1
rs_delivery.pending=rs_delivery.pending-1
rs_delivery.max_command_bytes=math.max(tonumber(rs_delivery.max_command_bytes) or 0,#command)
if ok and type(result)=="table" and result.confirmed==true and result.success==true then
rs_delivery.confirmed=rs_delivery.confirmed+1
else
rs_delivery.failed=rs_delivery.failed+1
rs_delivery.error=ok and "玩家消息未获游戏端成功确认" or tostring(result)
if not rs_delivery.first_failure then rs_delivery.first_failure={command_bytes=#command,result=result} end
pcall(util.write_file,game.dataDir().."/review_delivery_status.json",http.json_encode(rs_delivery))
end
pcall(util.write_file,game.dataDir().."/review_delivery_status.json",http.json_encode(rs_delivery))
if #rs_outbox>0 then util.setTimeout(0.4,"museReviewDeliveryStep") else rs_sending=false rs_step=nil end
end
-- Pace only long AI text; do not throttle command chains or hold the VM while waiting.
rs_step=step
util.setTimeout(0.4,"museReviewDeliveryStep")
return true
end
local function rs_send_reply(admin, body, tail)
rs_delivery={at=os.time(),target=tostring(admin),segments=0,confirmed=0,failed=0}
body = tostring(body or "")
tail = tostring(tail or "")
local prefix = "§c[巡查AI] §f"
local full = prefix.. body.. (tail ~= "" and ("\n".. tail) or "")
if rs_utf8_len(full) <= RS_SEND_LIMIT then rs_deliver(admin, full)
pcall(util.write_file,game.dataDir().."/review_delivery_status.json",http.json_encode(rs_delivery))
return end
local segs = rs_split_body(prefix.. body, RS_SEND_LIMIT)
if #segs > RS_SEND_MAXSEG then
local kept = {}
for i = 1, RS_SEND_MAXSEG do kept[#kept + 1] = segs[i] end
kept[RS_SEND_MAXSEG] = kept[RS_SEND_MAXSEG].. "\n§7（回复过长，已截断）"
segs = kept
end
if tail ~= "" then
local last = segs[#segs]
if last and rs_utf8_len(last) + 1 + rs_utf8_len(tail) <= RS_SEND_LIMIT then
segs[#segs] = last.. "\n".. tail
else
segs[#segs + 1] = tail
end
end
for _, s in ipairs(segs) do rs_deliver(admin, s) end
pcall(util.write_file,game.dataDir().."/review_delivery_status.json",http.json_encode(rs_delivery))
end

-- ===== 会话记忆（压缩式，无轮数硬上限）=====
-- 存储结构：ai_sessions[管理员] = {summary = "早前对话摘要", rounds = {{q=, a=},...}}（旧的纯数组格式自动迁移）
-- 口径：单轮存档 问 ≤300 字节、答 ≤800 字节；会话总存量 > 6000 字节且轮数 > 4 时，把最旧的（轮数-4）轮
-- 连同旧摘要一起压成新摘要（滚动压缩），最近 4 轮保留原文；摘要上限 1200 字节。
-- 压缩调用的 token 记入 token_stats 的 window 桶。压缩失败兜底：只保留最近 8 轮、摘要不变，绝不因此中断对话。
local RS_SESS_QMAX = 300
local RS_SESS_AMAX = 800
local RS_SESS_THRESHOLD = 6000
local RS_SESS_KEEP = 4
local RS_SESS_SUMMARY_MAX = 1200
local RS_SESS_FALLBACK_KEEP = 8
local function rs_sess_load(admin)
local d = rs_db_load()
local s = d.ai_sessions[admin]
if type(s) == "table" and type(s.rounds) == "table" then
if type(s.summary) ~= "string" then s.summary = "" end
return s
end
if type(s) == "table" then
local rounds = {}
for _, h in ipairs(s) do
if type(h) == "table" then rounds[#rounds + 1] = {q = tostring(h.q or ""), a = tostring(h.a or "")} end
end
local ns = {summary = "", rounds = rounds}
d.ai_sessions[admin] = ns
return ns
end
local ns = {summary = "", rounds = {}}
d.ai_sessions[admin] = ns
return ns
end
local function rs_sess_size(s)
local n = #(s.summary or "")
for _, h in ipairs(s.rounds) do n = n + #(h.q or "") + #(h.a or "") end
return n
end
local function rs_sess_touch(admin, q, a)
local s = rs_sess_load(admin)
s.rounds[#s.rounds + 1] = {q = tostring(q):sub(1, RS_SESS_QMAX), a = tostring(a):sub(1, RS_SESS_AMAX)}
rs_db_save()
if rs_sess_size(s) > RS_SESS_THRESHOLD and #s.rounds > RS_SESS_KEEP then
local cut = #s.rounds - RS_SESS_KEEP
local parts = {}
if s.summary ~= "" then parts[#parts + 1] = "（早前摘要）\n".. s.summary end
for i = 1, cut do
local h = s.rounds[i]
parts[#parts + 1] = "管理员：".. h.q.. "\n巡查AI：".. h.a
end
local ok, res = pcall(call_ai, {
{role = "system", content = "你是会话记忆压缩器。把给定的多轮对话（可能附带一份早前摘要）压缩成一段中文摘要，总长不超过 400 字。必须保留：涉及的玩家名、案件编号、信用分的增减数字与原因、已经执行过的动作（撤销/补回/扣分/备忘等）及其结果、尚未办完的事项。可以丢弃：寒暄、重复确认、过程性措辞。只输出摘要正文，不要加标题或解释。"},
{role = "user", content = table.concat(parts, "\n---\n")},
}, 30, rs_ai_model(), 600)
if ok and type(res) == "string" and trim(res) ~= "" then
rs_token_add("window", rs_token_last())
local sum = trim(res)
if #sum > RS_SESS_SUMMARY_MAX then
local c2 = sum:sub(1, RS_SESS_SUMMARY_MAX)
local nl = c2:find("\n[^\n]*$")
if nl then c2 = c2:sub(1, nl) end
sum = c2
end
s.summary = sum
local kept = {}
for i = cut + 1, #s.rounds do kept[#kept + 1] = s.rounds[i] end
s.rounds = kept
else
if #s.rounds > RS_SESS_FALLBACK_KEEP then
local kept = {}
for i = #s.rounds - RS_SESS_FALLBACK_KEEP + 1, #s.rounds do kept[#kept + 1] = s.rounds[i] end
s.rounds = kept
end
end
rs_db_save()
end
end

local function rs_patrol_ai_command_impl(admin, words)
local d = rs_db_load()
-- ===== 确定性口令（脚本直达层：命中即执行/直答，不经模型）=====
local known_list, known_set = rs_known_players()
local function find_names(text)
local hits = {}
for _, nm in ipairs(known_list) do
if rs_name_hit(text, nm) then hits[#hits + 1] = nm end
end
return hits
end
local function det_reply(text, verdict_note)
rs_ledger_add("管理员", admin, "巡查AI 指令", words, verdict_note or "")
rs_db_save()
rs_send_reply(admin, text, "")
return true
end
-- 撤销（按案件号 / 按玩家名；执行走 rs_revoke_case，与 ACTION REVOKE 共用同一幂等补回）
if words:find("撤销", 1, true) or words:find("撤回", 1, true) then
local idm = words:match("案件%s*#?%s*(%d+)") or words:match("#(%d+)")
if idm then
local done = nil
for _, c in ipairs(d.cases) do
if tonumber(c.id) == tonumber(idm) then done = rs_revoke_case(c, "管理员经巡查AI口令撤销") break end
end
if done then return det_reply(done.. "。", done) end
return det_reply("没找到案件 #".. tostring(idm).. "，无法撤销（案件可能已被清理删除）。", "撤销失败：案件不存在")
end
local hits = find_names(words)
if #hits == 1 then
local nm = hits[1]
local target_case, any_case = nil, nil
for i = #d.cases, 1, -1 do
local c = d.cases[i]
if c.target == nm then
any_case = any_case or c
if c.status ~= "revoked" then
if (tonumber(c.deducted) or 0) > 0 and not c.restored then target_case = c break end
target_case = target_case or c
end
end
end
if target_case then
local r = rs_revoke_case(target_case, "管理员经巡查AI口令撤销")
return det_reply(r.. "。", r)
end
if any_case then
local r = rs_revoke_case(any_case, "管理员经巡查AI口令撤销")
return det_reply(r.. "。", r)
end
return det_reply(nm.. " 名下没有任何案件记录，无可撤销。", "撤销失败：无案件")
end
end
-- 补回 / 恢复满分
if words:find("补回", 1, true) or words:find("加回", 1, true) or words:find("恢复", 1, true) then
local hits = find_names(words)
if #hits == 1 then
local nm = hits[1]
local cur = rs_credit_score(nm)
if words:find("恢复", 1, true) and (words:find("100", 1, true) or words:find("满分", 1, true)) then
if cur >= 100 then return det_reply(nm.. " 当前已经是 100 分，无需恢复。", "恢复满分：已是100") end
local after = rs_credit_add(nm, 100 - cur, "管理员经巡查AI口令恢复满分", "巡查AI")
return det_reply("已把 ".. nm.. " 的信用分恢复到 ".. tostring(after).. " 分。", "恢复满分：".. nm)
end
local pts = words:match("(%d+)%s*分")
if pts and (words:find("补回", 1, true) or words:find("加回", 1, true)) then
local after = rs_credit_add(nm, tonumber(pts), "管理员经巡查AI口令补回", "巡查AI")
return det_reply("已给 ".. nm.. " 补回 ".. pts.. " 分，当前 ".. tostring(after).. " 分。", "补回 ".. nm.. " ".. pts.. " 分")
end
end
end
-- 查分直答（问句带查分意图且只精确命中一名已知玩家）
if (words:find("查", 1, true) or words:find("多少", 1, true) or words:find("信用", 1, true)) and (words:find("分", 1, true) or words:find("案", 1, true) or words:find("记录", 1, true)) then
local hits = find_names(words)
if #hits == 1 then
local nm = hits[1]
local lines = {}
lines[#lines + 1] = nm.. " 当前信用分 ".. rs_credit_score(nm).. "（满分 100；未被扣过分的玩家默认 100）。"
local logs = {}
for _, e in ipairs(d.credit_log) do if e.player == nm then logs[#logs + 1] = e end end
if #logs > 0 then
lines[#lines + 1] = "最近变动："
for i = math.max(1, #logs - 4), #logs do
local e = logs[i]
lines[#lines + 1] = "- ".. rs_fmt_time(e.at).. " ".. ((tonumber(e.delta) or 0) >= 0 and "+" or "").. tostring(e.delta).. " → ".. tostring(e.after).. "（".. tostring(e.reason or "").. "）"
end
else
lines[#lines + 1] = "没有任何信用变动记录。"
end
local total_c, open_c = 0, 0
for _, c in ipairs(d.cases) do
if c.target == nm then
total_c = total_c + 1
if c.status == "open" or c.status == "escalated" or c.status == "explained" then open_c = open_c + 1 end
end
end
lines[#lines + 1] = "历史案件 ".. total_c.. " 件，未结 ".. open_c.. " 件。"
lines[#lines + 1] = "§8（已定向载入 ".. nm.. " 的卷宗：脚本直查）"
return det_reply(table.concat(lines, "\n"), "脚本直查：".. nm)
end
end
-- ===== 组装上下文（明细版，带字符预算与裁剪顺序）=====
local function build_ctx(cl_n, case_n, led_n, with_last_obj)
local c = {}
c[#c + 1] = ""
c[#c + 1] = "当前时间：".. os.date("%Y-%m-%d %H:%M:%S").. "（服务器本地时间）"
c[#c + 1] = rs_patrol_status_text()
c[#c + 1] = "信用分不是 100 的玩家："
local any = false
for nm, sc in pairs(d.credit) do
if tonumber(sc) ~= 100 then c[#c + 1] = "- ".. nm.. "：".. tostring(sc).. " 分" any = true end
end
if not any then c[#c + 1] = "（无）" end
local full = {}
for _, nm in ipairs(known_list) do
if rs_credit_score(nm) == 100 then full[#full + 1] = nm end
end
if #full > 0 then c[#c + 1] = "其余已知玩家信用分均为 100（未入表即 100）：".. table.concat(full, "、") end
c[#c + 1] = "案件（最近 ".. case_n.. " 件，含已结与已扣分额）："
if #d.cases == 0 then c[#c + 1] = "（无）" else
for i = math.max(1, #d.cases - case_n + 1), #d.cases do
local cs = d.cases[i]
c[#c + 1] = "- #".. tostring(cs.id).. " ".. tostring(cs.target).. " 状态 ".. tostring(cs.status).. "：".. tostring(cs.summary or "").. ((tonumber(cs.deducted) or 0) > 0 and ("；已扣 ".. tostring(cs.deducted).. " 分") or "").. "；判定：".. tostring(cs.applied or cs.verdict or "未判定")
end
end
c[#c + 1] = "信用流水（最近 ".. cl_n.. " 条：谁、何时、被扣/加多少、为什么）："
if #d.credit_log == 0 then c[#c + 1] = "（无）" else
for i = math.max(1, #d.credit_log - cl_n + 1), #d.credit_log do
local e = d.credit_log[i]
c[#c + 1] = "- ".. rs_fmt_time(e.at).. " ".. tostring(e.player).. " ".. ((tonumber(e.delta) or 0) >= 0 and "+" or "").. tostring(e.delta).. " → ".. tostring(e.after).. " 分（".. tostring(e.reason or "").. "）"
end
end
local attached, attached_names = {}, {}
local function attach(nm)
if nm and not attached[nm] and rs_name_hit(words, nm) then
attached[nm] = true
attached_names[#attached_names + 1] = nm
c[#c + 1] = rs_player_dossier(nm)
return true
end
return false
end
local n_att = 0
for _, nm in ipairs(known_list) do
if attach(nm) then n_att = n_att + 1 if n_att >= 3 then break end end
end
if with_last_obj then
local last_nm = nil
if #d.credit_log > 0 then last_nm = d.credit_log[#d.credit_log].player end
if not last_nm then
for i = #d.cases, 1, -1 do
local cs = d.cases[i]
if cs.status == "closed_punished" or cs.status == "closed_clear" or cs.status == "revoked" then last_nm = cs.target break end
end
end
if last_nm and not attached[last_nm] then
attached[last_nm] = true
attached_names[#attached_names + 1] = last_nm
c[#c + 1] = "[最近处理对象] 管理员用「他/刚才那个」指代时默认指这位，其卷宗如下："
c[#c + 1] = rs_player_dossier(last_nm)
end
end
if led_n > 0 then
c[#c + 1] = "最近审查记录（已滤除窗口指令条目）："
local kept = 0
for i = #d.ledger, 1, -1 do
local e = d.ledger[i]
if e.action ~= "巡查AI 指令" then
kept = kept + 1
local line = "- ".. rs_fmt_time(e.at).. " [".. tostring(e.actor).. "] ".. tostring(e.target).. " ".. tostring(e.action)
if e.basis and e.basis ~= "" then line = line.. "｜依据：".. tostring(e.basis):sub(1, 80) end
if e.verdict and e.verdict ~= "" then line = line.. "｜判定：".. tostring(e.verdict):sub(1, 80) end
c[#c + 1] = line
if kept >= led_n then break end
end
end
if kept == 0 then c[#c + 1] = "（无）" end
end
return table.concat(c, "\n"), attached_names
end
-- 注入预算：ctx 总量 ≤6000 字符；超限裁剪顺序 = 先去 ledger 段并收紧流水/案件条数 → 再去最近处理对象卷宗 → 最后按行截断
local ctx_text, attached_names = build_ctx(15, 10, 12, true)
if #ctx_text > 6000 then ctx_text, attached_names = build_ctx(8, 6, 0, true) end
if #ctx_text > 6000 then ctx_text, attached_names = build_ctx(8, 6, 0, false) end
if #ctx_text > 6000 then
local cut = ctx_text:sub(1, 6000)
local nl = cut:find("\n[^\n]*$")
if nl then cut = cut:sub(1, nl) end
ctx_text = cut.. "\n（卷宗过长，已按预算截断：优先保留状态、案件与信用流水的最近记录）"
end
local sys = RS_MERGED_PERSONA.. "\n注意：随附的历史对话仅供你理解管理员的指代（例如『他』『刚才那个』指谁），一切分数、案件状态、流水数字都以本次注入的实时卷宗为准，历史对话里出现过的旧数字不得当作现状引用。\n管理员用「撤销对 X 的处罚」「撤销案件 #N」「给 X 补回 N 分」「把 X 的分恢复到 100」「查 X 多少分」这类口令时，脚本会在你之前直接执行或答复，你无需再为同一件事输出 ACTION。\n如果需要执行动作，在回复的最后一行严格输出一条：\nACTION RESTORE <玩家名> <分值> <理由> —— 把信用分加回去（误扣补回用这条）\nACTION DEDUCT <玩家名> <分值> <理由> —— 按管理员指令扣分\nACTION REVOKE <案件编号> <理由> —— 撤销案件（与口令撤销走同一幂等逻辑：扣过分且未补回的才补回）\nACTION REVOKE @<玩家名> <理由> —— 撤销该玩家最近一件未撤销案件\nACTION NOTE <玩家名> <内容> —— 给玩家加一条备忘\nACTION KICKPASS <执行者> <目标玩家> —— 你判断受限管理员踢人理由合理时，给他一张一次性踢人许可（10 分钟内有效）\nACTION NONE —— 不需要执行动作（只是回答问题）\n分值只写数字。不要编造不存在的玩家、案件或记录；记录里没有的就直说没有。"
-- ===== 会话记忆（压缩式：摘要 + 全部保留轮次注入；轮数无硬上限，滚动压缩见 rs_sess_touch）=====
local sess = rs_sess_load(admin)
local msgs = {{role = "system", content = sys}}
if sess.summary ~= "" then
msgs[#msgs + 1] = {role = "user", content = "管理员 ".. admin.. " 与你的早前对话摘要（由系统压缩生成，仅供理解指代，事实以本次实时卷宗为准）：\n".. sess.summary}
msgs[#msgs + 1] = {role = "assistant", content = "（已了解早前对话摘要，后续以本次实时卷宗为准。）"}
end
for _, h in ipairs(sess.rounds) do
msgs[#msgs + 1] = {role = "user", content = "管理员 ".. admin.. " 对你说：".. tostring(h.q)}
msgs[#msgs + 1] = {role = "assistant", content = tostring(h.a)}
end
msgs[#msgs + 1] = {role = "user", content = "管理员 ".. admin.. " 对你说：".. words.. "\n\n".. ctx_text}
-- ===== 模型调用（失败自动重试 1 次；再失败给出原因 + 确定性数据降级答复）=====
local ok, res = pcall(call_ai, msgs, 60, rs_ai_model(), 1200)
if not ok or type(res) ~= "string" or res == "" then
ok, res = pcall(call_ai, msgs, 30, rs_ai_model(), 1200)
end
if not ok or type(res) ~= "string" or res == "" then
local reason = rs_err_summary(res)
rs_ledger_add("管理员", admin, "巡查AI 指令", words, "模型调用失败（已重试1次）：".. reason)
rs_db_save()
rs_send_reply(admin, "模型暂时不可用，已重试 1 次仍失败。原因：".. reason.. "\n§7先把确定性数据给你（模型恢复后可继续问我）：\n".. rs_facts_digest(), "")
return
end
local win_tok = rs_token_last()
rs_token_add("window", win_tok)
local body_lines = {}
for line in res:gmatch("[^\r\n]+") do body_lines[#body_lines + 1] = line end
local action_line = nil
for i = #body_lines, 1, -1 do
if body_lines[i]:find("^%s*ACTION%s+") then
action_line = trim(body_lines[i])
table.remove(body_lines, i)
break
end
end
local exec_note = ""
local fail_note = ""
if action_line then
local act, rest = action_line:match("^ACTION%s+(%S+)%s*(.*)$")
act = act and trim(act):upper() or ""
if act ~= "" and act ~= "NONE" then
if (act == "RESTORE" or act == "DEDUCT") and rest then
local nm, pts, rsn = rest:match("^(%S+)%s+(%d+)%s*(.*)$")
if nm and pts and known_set[nm] then
local delta = tonumber(pts) * (act == "RESTORE" and 1 or -1)
rs_credit_add(nm, delta, "管理员经巡查AI指令：".. tostring(rsn or ""), "巡查AI")
exec_note = "已执行：".. act.. " ".. nm.. " ".. pts.. " 分（当前 ".. rs_credit_score(nm).. " 分）"
end
elseif act == "REVOKE" and rest then
local done = nil
local idm = rest:match("^#?(%d+)")
if idm then
for _, cs in ipairs(d.cases) do
if tonumber(cs.id) == tonumber(idm) then done = rs_revoke_case(cs, "管理员经巡查AI撤销") break end
end
else
local nm = rest:match("^@(%S+)") or rest:match("^(%S+)")
if nm then
for i = #d.cases, 1, -1 do
local cs = d.cases[i]
if cs.target == nm and cs.status ~= "revoked" then done = rs_revoke_case(cs, "管理员经巡查AI撤销") break end
end
end
end
if done then exec_note = "已执行：".. done end
elseif act == "KICKPASS" and rest then
local an, tn = rest:match("^(%S+)%s+(%S+)")
if an and tn then
local d2 = rs_db_load()
d2.op_kick_pass[an.. "|".. tn] = rs_now() + 600
rs_db_save()
exec_note = "已执行：给 ".. an.. " 一次性踢人许可（目标 ".. tn.. "，10 分钟内有效）"
end
elseif act == "NOTE" and rest then
local nm, txt = rest:match("^(%S+)%s+(.+)$")
if nm then
rs_note_add(nm, "管理员备注：".. txt)
rs_db_save()
exec_note = "已执行：给 ".. nm.. " 加备忘"
end
end
if exec_note == "" then
fail_note = "§c（动作未执行：ACTION 格式不对、玩家不在已知名单或案件不存在。可用确定性口令，如「撤销对 玩家名 的处罚」「给 玩家名 补回 10 分」）"
end
end
end
local reply = trim(table.concat(body_lines, "\n"))
if reply == "" then reply = "（已收到）" end
rs_ledger_add("管理员", admin, "巡查AI 指令", words, exec_note ~= "" and exec_note or "仅答复")
local ts = d.token_stats or {}
local body_out = reply.. (exec_note ~= "" and ("\n§7".. exec_note) or "").. (fail_note ~= "" and ("\n".. fail_note) or "").. (#attached_names > 0 and ("\n§8（已定向载入 ".. table.concat(attached_names, "、").. " 的卷宗）") or "")
local tail_out = "§8[Token] 本次 §b".. tostring(win_tok).. " §8| 窗口累计 §b".. tostring(tonumber(ts.window) or 0).. " §8| 审查总累计 §b".. tostring(tonumber(ts.total) or 0).. " §8token §7| §8模型 ".. rs_ai_model_label().. " §7| §8".. os.date("!%Y-%m-%d %H:%M", os.time() + 28800)
rs_send_reply(admin, body_out, tail_out)
-- 会话记账放发送之后：压缩本身是一次模型调用，别让管理员等它（压缩失败有兜底，见 rs_sess_touch）
rs_sess_touch(admin, words, reply)
end

local function rs_patrol_ai_command(admin, words)
pcall(function()
if type(ai_think_begin) == "function" then ai_think_begin(admin, words, false, false) end
end)
local ok, err = xpcall(function() return rs_patrol_ai_command_impl(admin, words) end, function(e)
local trace=debug.traceback(tostring(e),2)
pcall(util.write_file,game.dataDir().."/review_query_error_trace.txt",trace)
return e
end)
pcall(function()
if type(ai_think_end) == "function" then ai_think_end(admin) end
end)
if not ok then
rs_send_reply(admin, "处理出错：".. rs_err_summary(err), "")
end
end

-- ----------.审查日志 ----------
local function rs_review_log_text()
local d = rs_db_load()
local lines = {"§b━━ 审查日志（审查 AI 维护）━━"}
local _ts = d.token_stats or {}
lines[#lines + 1] = "§7审查 AI 累计消耗 token：".. tostring(tonumber(_ts.total) or 0).. "（巡查 ".. tostring(tonumber(_ts.patrol) or 0).. "｜裁决 ".. tostring(tonumber(_ts.verdict) or 0).. "｜窗口 ".. tostring(tonumber(_ts.window) or 0).. "）"
lines[#lines + 1] = "§7巡查AI 当前模型：".. rs_ai_model_label().. (rs_ai_model() and ("｜ID：".. rs_ai_model()) or "")
local over = {}
for nm, sc in pairs(d.credit) do
if tonumber(sc) ~= 100 then over[#over + 1] = nm.. " ".. tostring(sc).. "分" end
end
lines[#lines + 1] = "§7信用分低于 100 的玩家：".. (#over > 0 and table.concat(over, "、") or "无")
local items = {}
for _, e in ipairs(d.ledger) do
items[#items + 1] = {at = e.at or 0, text = "§f[".. rs_fmt_time(e.at).. "] §7[".. tostring(e.actor).. "] §f".. tostring(e.target).. " §7".. tostring(e.action).. ((e.verdict and e.verdict ~= "") and (" → ".. e.verdict) or "").. ((e.basis and e.basis ~= "") and ("｜依据：".. e.basis) or "")}
end
local okd, dd = pcall(load_data)
if okd and type(dd) == "table" and type(dd.report_history) == "table" then
for _, h in ipairs(dd.report_history) do
if h.judgment then
items[#items + 1] = {at = h.at or 0, text = "§f[".. rs_fmt_time(h.at).. "] §7[举报审查] §f".. tostring(h.target).. " §7判定 ".. tostring(h.judgment).. "（".. tostring(h.count or 0).. " 人举报）"}
end
end
end
table.sort(items, function(a, b) return (a.at or 0) > (b.at or 0) end)
lines[#lines + 1] = "§b── 最近记录（新 → 旧）──"
for i = 1, math.min(#items, 25) do lines[#lines + 1] = items[i].text end
local open = {}
for _, c in ipairs(d.cases) do
if c.status == "open" or c.status == "escalated" or c.status == "explained" then open[#open + 1] = c end
end
if #open > 0 then
lines[#lines + 1] = "§b── 未结案件 ──"
for _, c in ipairs(open) do
lines[#lines + 1] = "§e#".. tostring(c.id).. " §f".. c.target.. " §7".. c.status.. "：".. tostring(c.summary)
end
end
return table.concat(lines, "\n")
end

-- ---------- 聊天入口：命令 + 脏话观察 ----------
local rs_seen = {}
function museReviewChat(player, message)
if type(player) ~= "string" or player == "" then return nil end
local msg = trim(tostring(message or ""))
if msg == "" then return nil end
local key = player.. "|".. msg
local t = rs_now()
if rs_seen[key] and t - rs_seen[key] < 3 then return "dup" end
rs_seen[key] = t
rs_touch_activity(player)

if msg:sub(1, #".审查解释") == ".审查解释" then
local text = trim(msg:sub(#".审查解释" + 1))
if text == "" then
pcall(send, player, "§e[审查] 用法：§f.审查解释 你的说明")
return "explain_usage"
end
rs_on_explain(player, text)
return "explain"
end
if msg == ".同意" then
local d = rs_db_load()
local v = d.func_votes.current
if type(v) == "table" and (rs_now() - (tonumber(v.at) or 0)) <= 120 and player ~= v.actor and rs_is_privileged(player) and not v.votes[player] then
v.votes[player] = true
local n = 0
for _ in pairs(v.votes) do n = n + 1 end
if n >= 2 then
d.func_votes.approved = {actor = v.actor, at = rs_now()}
d.func_votes.current = nil
rs_ledger_add("op守卫", v.actor, "function 命令链投票通过", tostring(v.label), "同意者：".. player)
rs_notify_privileged("§a[function 投票] 命令链「".. tostring(v.label).."」已获 2 名权限玩家同意，发起者重新执行即可放行。", nil)
else
rs_notify_privileged("§e[function 投票] ".. player.. " 已同意（".. n.. "/2）。", v.actor)
end
rs_db_save()
return "funcvote"
end
end
if msg == ".信用" then
local d = rs_db_load()
local open_n = 0
for _, c in ipairs(d.cases) do
if c.target == player and (c.status == "open" or c.status == "escalated" or c.status == "explained") then open_n = open_n + 1 end
end
local sc = rs_credit_score(player)
pcall(send, player, "§b[信用] 你的信用分：§e".. sc.. "§b（满分 100，每天自动回 5 分，申诉成功有补偿）。\n§7你的举报参考门槛档位：".. rs_credit_threshold(sc).. " 人（只作审查参考）；你当前未结案件：".. open_n.. " 件。")
return "credit"
end
if msg:sub(1, #".审查日志") == ".审查日志" then
-- 判定对齐 native-OP-first（2026-10-24）：服主/原生 OP/管理员标签任一命中放行；机器人本体（连接级 OP、按名 isOP 查不到）走 self-account 认领
local admin = rs_is_privileged(player)
if not admin then local _oksc, _rsc = pcall(rs_is_self_account, player); admin = _oksc and _rsc == true end
if not admin then
pcall(send, player, "§c[审查] 只有管理权限可以查看审查日志。")
return "denied"
end
local text = rs_review_log_text()
local buf = {}
for l in text:gmatch("[^\n]+") do
buf[#buf + 1] = l
if #buf >= 10 then pcall(send, player, table.concat(buf, "\n")) buf = {} end
end
if #buf > 0 then pcall(send, player, table.concat(buf, "\n")) end
return text
end
if lower(msg):sub(1, #".巡查ai") == ".巡查ai" then
-- 判定对齐 native-OP-first（2026-10-24）：服主/原生 OP/管理员标签任一命中放行；机器人本体（连接级 OP、按名 isOP 查不到）走 self-account 认领
local admin = rs_is_privileged(player)
if not admin then local _oksc, _rsc = pcall(rs_is_self_account, player); admin = _oksc and _rsc == true end
if not admin then
pcall(send, player, "§c[巡查AI] 只有管理员可以和巡查 AI 对话。")
return "denied"
end
local words = trim(msg:sub(#".巡查ai" + 1))
if words == "" then
pcall(send, player, "§e[巡查AI] 用法：§f.巡查AI 你要交代的事 §7（例如：把某玩家被扣的分加回去）")
return "usage"
end
rs_patrol_ai_command(player, words)
return "patrolai"
end

if msg:sub(1, 1) ~= "." and msg:sub(1, 1) ~= "/" then
if msg:find("不同意", 1, true) or msg:find("取消同意", 1, true) then
rs_consent_revoke(player)
pcall(send, player, "§e[审查] 已取消你先前对 AI 查询背包/坐标的同意。")
return "consent_revoke"
end
if msg:find("同意", 1, true) and (msg:find("查", 1, true) or msg:find("看", 1, true)) and (msg:find("背包", 1, true) or msg:find("坐标", 1, true)) then
local scope = "all"
if msg:find("背包", 1, true) and not msg:find("坐标", 1, true) then scope = "inv" end
if msg:find("坐标", 1, true) and not msg:find("背包", 1, true) then scope = "pos" end
rs_consent_grant(player, scope)
pcall(send, player, "§a[审查] 已记录你的同意：10 分钟内受限管理员可经 AI 查询你的".. (scope == "inv" and "背包" or scope == "pos" and "坐标" or "背包与坐标").. "。随时发「不同意」可取消。")
return "consent"
end
local admin = false
pcall(function() admin = is_admin(player) == true end)
if not admin then
local hit = rs_prof_hit(msg)
if hit then
rs_on_profanity(player, hit)
return "profanity"
end
end
end
return nil
end
events.onChat("museReviewChat")

-- ---------- 定时：挂到既有 10 秒过期清理上做巡查调度 ----------
if type(expire_system_state) == "function" then
local rs_prev_expire = expire_system_state
expire_system_state = function(...)
local r = rs_prev_expire(...)
pcall(rs_patrol_tick)
return r
end
end

-- Uses the same in-game permission check and query path; no separate moderation route.
plugin_api.museReviewDelivery = function() return rs_delivery end
plugin_api.museReviewQuery = function(admin, words) return museReviewChat(admin, ".巡查AI "..tostring(words or "")) end
plugin_api.museReviewVersion = function() return {ok = true, version = "review-system-v2"} end
plugin_api.museReviewPatrolStatus = function() local _ok, _v = pcall(rs_patrol_status_text) if _ok then return {ok = true, text = _v} end return {ok = false, err = tostring(_v)} end

-- 内部接口（供巡检/测试调用本系统能力；命名空间隔离）
_G.__muse_review_internals = {
    db_load = rs_db_load, db_save = rs_db_save, now = rs_now,
    process_scan = rs_process_scan, sweep_cases = rs_sweep_cases,
    patrol_round = rs_patrol_round, review_log_text = rs_review_log_text,
    history_text = rs_history_text, prof_hit = rs_prof_hit,
    on_explain = rs_on_explain, patrol_ai_command = rs_patrol_ai_command,
send_reply = rs_send_reply, sess_load = rs_sess_load, sess_touch = rs_sess_touch, ai_model = rs_ai_model, ai_model_set = rs_ai_model_set,
    op_guard = rs_op_guard, ban_guard = rs_ban_guard, function_gate = rs_function_gate,
    is_op = rs_is_op, is_super = rs_is_super, is_native_op = rs_native_op, is_privileged = rs_is_privileged,
    consent_ok = rs_consent_ok, consent_grant = rs_consent_grant,
    daily_regen = rs_daily_regen, wind_upheld_count = rs_wind_upheld_count,
}

end
-- END Muse Review System v2
-- === MUSE BLOODNIGHT v3 BEGIN ===
-- ===== Muse 血月玩法 v3（2026-10-23 晚改造）：分组独立制 · 分档数值 · 制式装备/命名 · 虚空清场 =====
-- v2 沿革：红雾雷雨 · 积分制 · 命令链行为层；起止/冻结/周期走真实世界时间（BN2_RT，v2.1 整改，不回退）。
-- v3 改造（用户 2026-10-23 18:57/19:01/19:03/19:05 裁定）：废全服人数缩放，改玩家簇独立运行——
--   开场按距离聚类（质心半径 100/150/200/250 格扩到最多 5 人，4 人按 3 人档），分组定型整晚不变，
--   每组独立波次/高潮/档位；Boss/骑士/射手/精英按组档分级；全怪制式装备 + 类别命名（防消失）；
--   生成后沿锚点方向外推 15 格防进基地；结束清场改同维度虚空 TP + kill（execute in 维度子命令本宿主实测不支持）。
-- 依据：v2 设计文档 + 用户 2026-10-23 16:58 四项裁定 + 独立审查 B1–B14 修订。
-- 架构：行为（拆/搭/buff/Boss 技能/射手狙击）下沉到原生命令链（command_txt 三份 TXT，插件按需启停）；
--       插件只做编排：日历（以带回执命令查询的真实世界 day/daytime 为准——2026-10-23 18:31 时间来源整改）、召唤、登记、结算、清场。
-- 积分：只有登记在案且带 tag=bn2 的血月怪才结算；归属优先级（用户 2026-10-23 17:39 裁定）：最后攻击者证据 > 并列取最近攻击者 > 无证据时记给死亡点 16 格内最近的在场玩家，半径内无人不给分。
-- 理智系统整套删除（用户拍板）；紫水晶块废除，改 bn_score 积分（跨夜累计落盘）。
-- 起止跟随真实世界时间：真实 daytime ∈ [13000,23000) 为血月窗口（与公告栏阶段表夜窗同口径）；真实 daytime 到 23000 天亮收尾。
-- 夜长拉长（用户 2026-10-23 18:15 裁定「中夜冻结」，取代 17:39 的中段拨回一次）：夜到中夜（真实 daytime≥18000）
--   关 doDaylightCycle，把世界时钟停在中夜（暂停，不是拨表）；
--   按真实时钟停到整晚约 20 分钟（解冻时刻 = 开夜 + 1200 秒 − 尾段 250 秒）再恢复原值，放行至天亮收尾。
if not _G.__muse_bloodnight_v2 then
_G.__muse_bloodnight_v2 = true
_G.museBloodNightVersion = "bloodnight-v3"

-- ================= 配置区 =================
local BN2_PERIOD_DAYS = 8            -- 每 N 个游戏日一次血月
local BN2_FOG_ID = "minecraft:fog_crimson_forest"  -- 用户指定：绯红森林雾（天空红）
local BN2_NIGHT_TARGET_SEC = 720     -- #26 整晚约 12 分钟（2026-10-25 改定；原 20 分钟/中夜冻结口径废止，改开场即全程冻结）
local BN2_FREEZE_AT_TICK = 18000     -- 夜到中夜（真实 daytime 达此值）时关 doDaylightCycle 冻结
local BN2_FREEZE_LATEST_TICK = 21000 -- 冻结触发窗口上限：越过子夜（21000）太多不再冻，保住尾段时长
local BN2_FREEZE_TAIL_SEC = 250      -- 解冻后尾段预留：18000→23000 共 5000 tick ≈ 250 秒走到天亮
local BN2_FREEZE_HARD_CAP = 840      -- #34 冻结硬上限 = 总时长 720 + 120 缓冲，异常到点强制解冻
local BN2_FALLBACK_RADIUS = 16       -- 归属兜底档：死亡点最近玩家搜索半径（格）
local BN2_EXTEND_MAX = 180           -- B5：高潮怪活着越过黎明时有界延长上限（秒）
local BN2_HURT_WINDOW = 15           -- B2 宽松口径：最后攻击证据窗口（秒）
local BN2_TIE_WINDOW = 2             -- B2：两名攻击者时间差 ≤ 此值视为并列，取离死亡点最近者
local BN2_VANISH_WINDOW = 5          -- 消失结算：最后受击后 N 秒内实体消失且血量曾 ≤35% 才认账
local BN2_CAP_BASE, BN2_CAP_PER, BN2_CAP_MAX = 10, 3, 26  -- B11：登记实体总量上限（含小兵/坐骑）
local BN2_WAVE_AT = {12, 132, 252, 372}   -- 波次相对开夜秒数
local BN2_CLIMAX_AT = 405                 -- B5：高潮提前登场
-- ===== 2026-10-25 .muse 血月轮配置（队列 #25-#47；取值理由见 blood_change_notes.md） =====
local BN2_SPAWN_MIN, BN2_SPAWN_MAX = 18, 24  -- #25/#28 生成环带（原 27~33 太远）
local BN_ELITE_PTS = 15                      -- #38 精英分 10 -> 15
local BN2_WAVE_SURVIVE_BONUS = {10, 20, 35, 55}  -- #29/#35/#37 撑过每波金币奖励，逐波递增（共 4 波）
local BN2_FULL_CLEAR_BONUS = 50              -- #36 全波+高潮撑过且整夜未死的满额奖励（金币）
local BN2_SCORE_COIN_DIV = 2                 -- #29 今夜积分换算：2 分 = 1 金币（收尾一次性）
local BN_BOMBER_CAP = {5, 5, 5, 8}           -- #42-#44 爆破兵每组每波硬顶（末波 8）
local BN_AIRDROP_CHANCE = 0.35               -- #45/#46 空降批次概率
local BN_DUSK_FROM, BN_DUSK_TO = 12000, 13500  -- #40-#44 自动开场傍晚窗（真实 daytime）
local BN_MUSIC_SOUND = "record.tears"        -- #30「艾莫斯罗迪」= 作曲家 Amos Roddy，其唱片为 Tears（基岩版 1.21.90 加入，音效 id record.tears，2026-10-25 服主确认方向后查证填入）
-- ===== v3 分组独立制配置（用户 2026-10-23 18:57/19:01/19:03 定版） =====
-- 档位按组人数定：1 人→档1、2 人→档2、3~4 人→档3、5 人→档4（4 人按 3 人档是用户明文裁定）
local BN3_TIER_MULT = {0.40, 0.55, 0.70, 1.00}      -- 伤害倍率总表（重锤五式写死在各档命令链里；骑士/射手/精英同吃此梯度）
local BN3_BOSS_HP_AMP = {30, 50, 70, 145}           -- 重锤血量：health_boost 等级（底血 20 + 4/级）= 总血 140/220/300/600
local BN3_BOSS_RATE = {0.02, 0.04, 0.06, 0.10}      -- 重锤替换骑士概率（随档上升）
local BN3_KNIGHT_HP_AMP = {1, 2, 3, 4}              -- 骑士血量：总血 24/28/32/36（僵尸底血 20 + 4/级；满档 amp4 = 现行 100% 基准）
local BN3_KNIGHT_WEAPON = {"iron_sword", "iron_sword", "diamond_sword", "netherite_sword"}  -- 骑士武器随档
local BN3_ELITE_HP_AMP = {1, 2, 3, 5}               -- 精英血量强化等级（随档上升）
local BN3_CLUSTER_R = {100, 150, 200, 250}          -- 扩组半径（水平距离）：加入第 2/3/4/5 人时与组质心的距离上限
local BN3_GROUP_CAP_BASE, BN3_GROUP_CAP_PER = 400, 0  -- 【2026-10-23 晚·用户裁定】血月不设玩法数量上限：原每组 6+4×人数 已废除；400 仅为防跑飞保险丝（正常波次表永远打不到），不是玩法上限
local BN3_GLOBAL_CAP = 600                          -- 【用户裁定】原全局硬上限 48 作废；600 仅为防崩服保险丝（数值与理由见施工报告专段），刷怪规模只由波次表决定
local BN3_PUSH_DIST = 15                            -- 落点外推：生成后沿锚点→落点方向再向外推的格数
local BN3_HAMMER_CHAINS = {"血月_重锤T1.txt", "血月_重锤T2.txt", "血月_重锤T3.txt", "血月_重锤.txt"}    -- 下标=档位，档4 沿用原文件
local BN3_SHOOTER_CHAINS = {"血月_射手T1.txt", "血月_射手T2.txt", "血月_射手T3.txt", "血月_射手.txt"}  -- 下标=档位，档4 沿用原文件
local function bn_tier_of(size)
  size = tonumber(size) or 1
  if size >= 5 then return 4 elseif size >= 3 then return 3 elseif size == 2 then return 2 end
  return 1
end
local BN2_RESTORE_DIFFICULTY = "normal"
local BN2_RESTORE_MOBGRIEFING = true
local BN2_DB_FILE = game.dataDir() .. "/bloodnight.json"
local BN2_PHASES = {CALM = "CALM", DESCENT = "DESCENT", HUNT = "HUNT", CLIMAX = "CLIMAX", EXTEND = "EXTEND", SETTLE = "SETTLE"}
local BN2_PHASE_CN = {CALM = "平静", DESCENT = "血月降临", HUNT = "血月进行", CLIMAX = "高潮", EXTEND = "决战延长", SETTLE = "结算"}
local BN2_KNIGHT_NAME = "血月骑士"
local BN2_BOSS_NAME = "重锤战士"
local BN2_CHAIN_BEHAVIOR = "血月_行为.txt"
local BN2_CHAIN_HAMMER = "血月_重锤.txt"
local BN2_CHAIN_SHOOTER = "血月_射手.txt"
-- 拆方块白名单（用户 2026-10-23 17:39 裁定：含泥土、泥巴）——与命令链 TXT 保持一致，仅供插件侧参考/校验
local BN2_BREAKABLE = {grass_block = true, dirt = true, mud = true, stone = true, glass = true,
  oak_planks = true, spruce_planks = true, birch_planks = true, jungle_planks = true,
  acacia_planks = true, dark_oak_planks = true, mangrove_planks = true, cherry_planks = true,
  oak_leaves = true, spruce_leaves = true, birch_leaves = true, jungle_leaves = true,
  acacia_leaves = true, dark_oak_leaves = true, mangrove_leaves = true, cherry_leaves = true, azalea_leaves = true}

-- ================= 基础工具（与既有段同款写法） =================
local function bn_now()
  local t = nil
  pcall(function() t = tonumber(util.timestamp()) end)
  return t or os.time()
end
local function bn_lower(s)
  local f = rawget(_G, "lower")
  if type(f) == "function" then local ok, v = pcall(f, tostring(s or "")); if ok then return v end end
  return string.lower(tostring(s or ""))
end
local function bn_trim(s)
  local f = rawget(_G, "trim")
  if type(f) == "function" then local ok, v = pcall(f, tostring(s or "")); if ok then return v end end
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end
local function bn_esc(name) return tostring(name or ""):gsub('"', '\\"') end
local function bn_psel(name) return '@a[name="' .. bn_esc(name) .. '"]' end
local function bn_cmd(s) if s and s ~= "" then pcall(game.sendCommand, s) end end
local function bn_log(s)
  if type(plugin_api_log) == "function" then pcall(plugin_api_log, "[bloodnight-v3] " .. tostring(s)) end
end
local function bn_sub(s, a, b) return string.sub(tostring(s or ""), a or 1, b or -1) end
local function bn_cmd_resp(s)
  -- R107 callback latency guard: polling the server clock is best-effort.
  -- Slow command responses must not block all plugin callbacks for 8s
  -- while bloodnight's existing stale-time guard already handles failure.
  local timeout = (type(s)=="string" and s:match("^time query ")) and 3 or 8
  local ok, resp = pcall(game.sendCommandWithResp, s, "ai", timeout)
  if ok then return resp end
  return nil
end
local function bn_resp_leaves(resp, out, depth)
  out = out or {}
  depth = depth or 0
  if depth > 3 or resp == nil then return out end
  local t = type(resp)
  if t == "string" or t == "number" then out[#out + 1] = resp; return out end
  if t == "table" then
    for k, v in pairs(resp) do
      local kt = type(k)
      if kt == "string" and (type(v) == "number" or type(v) == "string") then out[#out + 1] = k .. "=" .. tostring(v) end
      bn_resp_leaves(v, out, depth + 1)
    end
  end
  return out
end
local function bn_resp_text(resp)
  local lv = bn_resp_leaves(resp)
  local parts = {}
  for _, v in ipairs(lv) do parts[#parts + 1] = tostring(v) end
  return table.concat(parts, " | ")
end
local function bn_rawtext(text)
  if type(json_rawtext) == "function" then local ok, v = pcall(json_rawtext, tostring(text or "")); if ok and v then return v end end
  if http and type(http.json_encode) == "function" then
    local ok, v = pcall(http.json_encode, {rawtext = {{text = tostring(text or "")}}})
    if ok and v then return v end
  end
  return nil
end
local function bn_broadcast(text)
  if type(broadcast) == "function" then local ok = pcall(broadcast, text); if ok then return end end
  local p = bn_rawtext(text)
  if p then bn_cmd("tellraw @a " .. p) end
end
local function bn_send(name, text)
  if type(send) == "function" then local ok = pcall(send, name, text); if ok then return end end
  pcall(game.sendTargeted, name, text)
end
local function bn_actionbar(name, text)
  local p = bn_rawtext(text)
  if not p then return end
  if name then bn_cmd('titleraw ' .. bn_psel(name) .. ' actionbar ' .. p)
  else bn_cmd("titleraw @a actionbar " .. p) end
end
local function bn_title(text, sub)
  bn_cmd("title @a times 15 80 25")
  local p = bn_rawtext(text)
  if p then bn_cmd("titleraw @a title " .. p) end
  if sub then local p2 = bn_rawtext(sub); if p2 then bn_cmd("titleraw @a subtitle " .. p2) end end
end
local function bn_online()
  if type(online_players) == "function" then
    local ok, v = pcall(online_players)
    if ok and type(v) == "table" then return v end
  end
  return {}
end
local function bn_pos(name)
  if type(player_position) == "function" then
    local ok, v = pcall(player_position, name)
    if ok and type(v) == "table" and tonumber(v.x) and tonumber(v.y) and tonumber(v.z) then
      return {x = tonumber(v.x), y = tonumber(v.y), z = tonumber(v.z)}
    end
  end
  if game and type(game.getPlayerPos) == "function" then
    local ok, v = pcall(game.getPlayerPos, name)
    if ok and type(v) == "table" and tonumber(v.x) then return {x = tonumber(v.x), y = tonumber(v.y), z = tonumber(v.z)} end
  end
  return nil
end
-- 在玩家本人位置放声音（v1 的教训：playsound 坐标占位会把声源锚在机器人身上，离远了听不见）
local function bn_sound_at(name, sound, vol, pitch)
  local pos = bn_pos(name)
  local s = "playsound " .. sound .. " " .. bn_psel(name)
  if pos then
    s = s .. " " .. math.floor(pos.x) .. " " .. math.floor(pos.y + 1) .. " " .. math.floor(pos.z)
    s = s .. " " .. tostring(vol or 1) .. " " .. tostring(pitch or 1) .. " 1"
  end
  bn_cmd(s)
end
local function bn_is_admin(name)
  if type(is_admin) == "function" then local ok, v = pcall(is_admin, name); if ok and v == true then return true end end
  if type(rawget(_G, "SUPERUSER")) == "string" and name == rawget(_G, "SUPERUSER") then return true end
  local ok, v = pcall(function() return player.isOP(name) end)
  if ok and v == true then return true end
  return false
end
local function bn_bot_name()
  if type(ai_bot_identity) == "function" then
    local ok, v = pcall(ai_bot_identity)
    if ok and v then return tostring(v) end
  end
  return nil
end
-- 区域读取缓存（2026-10-24）：宿主对每次 game.getBlockData 调用都会写一条事件日志，
-- 血月刷怪校验时同一坐标被反复读取（约 10 条/秒）会淹没事件流。同一坐标 10 秒内复用结果，
-- 读取失败不缓存（保留排查价值）。游戏语义不变：10 秒内的方块变化不影响刷怪落点校验。
local bn_block_cache = {}
local function bn_block_id_raw(x, y, z)
  x, y, z = math.floor(x), math.floor(y), math.floor(z)
  if game and type(game.getBlockData) == "function" then
    local ok, v = pcall(game.getBlockData, x, y, z)
    if ok and v then
      if type(v) == "string" and v ~= "" then return bn_lower(v):gsub("^minecraft:", "") end
      if type(v) == "table" then
        local id = v.name or v.id or v.block or v.blockName or v.type
        if id then return bn_lower(tostring(id)):gsub("^minecraft:", "") end
      end
    end
  end
  if game and type(game.getBlockNBT) == "function" then
    local ok, v = pcall(game.getBlockNBT, x, y, z)
    if ok and type(v) == "string" then
      local id = v:match('id:%s*"([^"]+)"') or v:match("id:%s*'([^']+)'")
      if id then return bn_lower(id):gsub("^minecraft:", "") end
    end
  end
  return nil
end
local function bn_block_id(x, y, z)
  x, y, z = math.floor(x), math.floor(y), math.floor(z)
  local ck = x .. "," .. y .. "," .. z
  local ce = bn_block_cache[ck]
  local now = os.time()
  if ce and (now - ce.t) <= 10 then return ce.id end
  local rid = bn_block_id_raw(x, y, z)
  if rid ~= nil then
    bn_block_cache[ck] = { id = rid, t = now }
    local n = 0
    for _ in pairs(bn_block_cache) do n = n + 1 end
    if n > 2000 then bn_block_cache = {} end
  end
  return rid
end
local function bn_count_tag(tag)
  if game and type(game.getTarget) == "function" then
    local ok, v = pcall(game.getTarget, "@e[tag=" .. tag .. "]")
    if ok and type(v) == "table" then
      if type(v.targets) == "table" then return #v.targets end
      return #v
    end
  end
  return nil
end
local function bn_query_difficulty()
  local resp = bn_cmd_resp("difficulty")
  if resp == nil then return nil end
  local txt = bn_lower(bn_resp_text(resp))
  for _, w in ipairs({"peaceful", "easy", "normal", "hard"}) do if txt:find(w, 1, true) then return w end end
  return nil
end
local function bn_query_mobgriefing()
  local resp = bn_cmd_resp("gamerule mobgriefing")
  if resp == nil then return nil end
  local txt = bn_lower(bn_resp_text(resp))
  if txt:find("false", 1, true) then return false end
  if txt:find("true", 1, true) then return true end
  return nil
end
local function bn_query_daylight()
  local resp = bn_cmd_resp("gamerule doDaylightCycle")
  if resp == nil then return nil end
  local txt = bn_lower(bn_resp_text(resp))
  if txt:find("false", 1, true) then return false end
  if txt:find("true", 1, true) then return true end
  return nil
end
local function bn_query_weather()
  local resp = bn_cmd_resp("weather query")
  if resp == nil then return nil end
  local txt = bn_lower(bn_resp_text(resp))
  if txt:find("thunder", 1, true) then return "thunder" end
  if txt:find("rain", 1, true) then return "rain" end
  if txt:find("clear", 1, true) then return "clear" end
  return nil
end

-- 【2026-10-23 时间来源整改】原 bn_board（读 R20W/R21D 公告栏模拟 ticks）与按自数 dayCount 的
-- bn_is_blood_day 已退役删除：起止 / 冻结 / 周期一律改走 BN2_RT 真实时间层（见运行时状态之后的
-- 「真实世界时间」节）。dayCount 字段保留在数据文件里，但不再作为任何事实来源。
local function bn_keep_inventory()
  local t = bn_resp_text(bn_cmd_resp("gamerule keepinventory"))
  if t == "" then return false end
  local low = bn_lower(t)
  if low:find("keepinventory") and low:find("true") then return true end
  return false
end

-- ================= 持久化（data/bloodnight.json，v2 账本） =================
local bn_db = nil
local function bn_db_load()
  if bn_db then return bn_db end
  local d = nil
  pcall(function() local ok, v = pcall(util.json_load, BN2_DB_FILE); if ok and type(v) == "table" then d = v end end)
  d = d or {}
  if tonumber(d.version) ~= 2 then
    -- v1 → v2 迁移：旧血晶（紫水晶块）累计按设计作废、理智 pstate 整表删除；夜数/日数/总击杀史保留
    local nd = {
      version = 2,
      phase = tostring(d.phase or BN2_PHASES.CALM),
      nightNo = tonumber(d.nightNo) or 0,
      dayCount = tonumber(d.dayCount) or 0,
      lastBloodDay = tonumber(d.lastBloodDay) or -1,
      score = {}, tonight = {}, participants = {}, died = {},
      stats = {nights = tonumber(d.stats and d.stats.nights) or 0, kills = tonumber(d.stats and d.stats.kills) or 0},
      origDifficulty = d.origDifficulty, origMobGriefing = d.origMobGriefing, origWeather = nil,
    }
    if nd.phase == BN2_PHASES.DESCENT or nd.phase == BN2_PHASES.HUNT or nd.phase == BN2_PHASES.CLIMAX then nd.phase = BN2_PHASES.CALM end
    bn_db = nd
    pcall(util.json_write, BN2_DB_FILE, bn_db)
    return bn_db
  end
  d.score = d.score or {}
  d.tonight = d.tonight or {}
  d.participants = d.participants or {}
  d.died = d.died or {}
  d.stats = d.stats or {nights = 0, kills = 0}
  d.keepSeenOff = d.keepSeenOff == true  -- keep 收尾武装标记（2026-10-23 热修）；旧档缺字段默认未武装
  bn_db = d
  return bn_db
end
local function bn_db_save()
  if bn_db then pcall(util.json_write, BN2_DB_FILE, bn_db) end
end

-- ================= 运行时状态（只存内存） =================
local BN2 = {
  booted = false, lastTick = 0, nightStart = 0, manual = false,
  waveDone = 0, climaxUid = nil, climaxKind = nil, extendUntil = 0, extendAnnounced = false,
  queue = {}, mobReg = {}, pending = {}, rid2uid = {}, killEventIds = {},
  players = {}, bar = {}, boardPrev = nil, lastWrapAt = 0, omenAt = 0, calAt = 0,
  keepTagAt = 0, chainHygAt = 0, settleAt = 0, saveAt = 0,
  chains = {}, selftest = {}, events = {}, lastResp = "",
  lastTickAt = 0, nextBoardAt = 0, nextKeepAt = 0, nextChainAt = 0, nextSyncAt = 0,
  prevTicks = nil, prevKeep = nil, showStage = 0, extendStart = 0, manualNight = false,
  counters = {spawnSeen = 0, claimed = 0, hurtSeen = 0, hurtKnown = 0, hurtNoUid = 0, spawnOk = 0, spawnFail = 0,
              healthSeen = 0, killSeen = 0, killSettled = 0, settleHealth = 0, settleKill = 0,
              settlePlayerKill = 0, settleVanish = 0, settleFallback = 0, noEvidence = 0, points = 0,
              push = 0, pushFallback = 0},
  groups = {}, groupOf = {}, anyClimax = false, waveAnnounced = {},
  causeSeen = {},
}
-- ================= 真实世界时间（2026-10-23 18:31 裁定：废自数天数与一切 time set） =================
-- 血月的时间与天数一律以真实世界为准：用带回执命令查 time query daytime / time query day，
-- 结果带缓存与新鲜度守卫；查询失败时回退最近一次有效值并在 diag 标注 rtStale，绝不编造。
-- 周期锚点 rtAnchor（存 bloodnight.json）：真实 day % 8 == rtAnchor 的日子是血月日；
-- 迁移时按退役自数口径的「还有 X 天」把首个血月夜对齐到真实 day，此后每 8 个真实日一轮。
local BN2_RT_POLL_SEC = 30      -- 真实时间正常轮询间隔（秒）
local BN2_RT_RETRY_SEC = 10     -- 数据陈旧时的重试间隔（秒）
local BN2_RT_FRESH_SEC = 120    -- 新鲜度守卫：距上次成功查询超过此秒数视为陈旧
local BN2_RT_NIGHT_FROM = 13000 -- 入夜界（与公告栏阶段表 keep=false 夜窗同口径）
local BN2_RT_NIGHT_TO = 23000   -- 天亮界（同上）
local BN2_RT = {daytime = nil, day = nil, atDaytime = 0, atDay = 0, lastTry = 0,
                rawDaytime = "", rawDay = "", fail = 0}
local function bn_resp_leaf_num(resp, key)
  local found = nil
  local function scan(x, depth)
    if found ~= nil or depth > 3 or x == nil then return end
    local t = type(x)
    if t == "table" then
      for k, v in pairs(x) do
        if found ~= nil then return end
        if type(v) == "table" then scan(v, depth + 1)
        elseif (type(v) == "number" or type(v) == "string") and tostring(k) == key then
          local n = tonumber(v); if n then found = n end
        end
      end
    elseif t == "string" then
      for k, v in x:gmatch("([%a_]+)%s*=%s*(%-?%d+)") do
        if k == key then found = tonumber(v); return end
      end
    end
  end
  scan(resp, 0)
  return found
end
local function bn_rt_parse_daytime(resp)
  local flat = bn_resp_text(resp)
  local function okv(n) n = tonumber(n); if n and n >= 0 and n < 24000 then return n end return nil end
  local v = okv(bn_resp_leaf_num(resp, "daytime")) or okv(bn_resp_leaf_num(resp, "time"))
  if v then return v, flat end
  v = okv(flat:match("commands%.time%.query%.daytime:%s*(%d+)"))
  if v then return v, flat end
  v = okv(flat:match("[Dd]aytime[^%d]*(%d+)"))
  if v then return v, flat end
  v = okv(flat:match("时间[^%d]*(%d+)"))
  if v then return v, flat end
  return nil, flat
end
local function bn_rt_parse_day(resp)
  local flat = bn_resp_text(resp)
  local function okv(n) n = tonumber(n); if n and n >= 0 then return n end return nil end
  local v = okv(bn_resp_leaf_num(resp, "day"))
  if v then return v, flat end
  -- 注意：daytime 的消息键 commands.time.query.daytime 不会命中下面这条（day 后须紧跟冒号）
  v = okv(flat:match("commands%.time%.query%.day:%s*(%d+)"))
  if v then return v, flat end
  local stripped = flat:gsub("[Dd]aytime[^%d]*%d+", " ")
  v = okv(stripped:match("[Dd]ay%s*[:=]%s*(%d+)")) or okv(stripped:match("[Dd]ay[^%d]*(%d+)"))
    or okv(stripped:match("天数[^%d]*(%d+)"))
  if v then return v, flat end
  return nil, flat
end
local function bn_rt_legacy_days_left()
  -- 迁移专用：按退役的自数口径算「还有几天」，只用一次把锚点对齐到真实 day，之后永不再用
  local d = bn_db_load()
  local dc = tonumber(d.dayCount) or 0
  local lb = tonumber(d.lastBloodDay) or -1
  if lb >= 0 and dc >= lb then return lb + BN2_PERIOD_DAYS - dc end
  local m = dc % BN2_PERIOD_DAYS
  return (BN2_PERIOD_DAYS - m) % BN2_PERIOD_DAYS
end
local function bn_rt_migrate(rd)
  local d = bn_db_load()
  if d.rtAnchor ~= nil then return end
  local left = bn_rt_legacy_days_left()
  d.rtAnchor = (rd + left) % BN2_PERIOD_DAYS
  d.lastBloodDay = rd + left - BN2_PERIOD_DAYS  -- 真实尺度上的上一个血月日（虚拟锚点，此后由真实开夜回写）
  d.rtMigrated = true
  bn_db_save()
  bn_log("rt migrate realDay=" .. tostring(rd) .. " legacyLeft=" .. tostring(left) .. " anchor=" .. tostring(d.rtAnchor))
end
local function bn_rt_refresh(force)
  local now = bn_now()
  if not force and now - BN2_RT.lastTry < 5 then return BN2_RT.day ~= nil and BN2_RT.daytime ~= nil end
  BN2_RT.lastTry = now
  local dv, draw = bn_rt_parse_daytime(bn_cmd_resp("time query daytime"))
  if dv ~= nil then
    BN2_RT.daytime = dv; BN2_RT.atDaytime = now
    BN2_RT.rawDaytime = bn_sub(draw, 1, 160)
    BN2.lastResp = bn_sub(draw, 1, 120)
  end
  local dy, yraw = bn_rt_parse_day(bn_cmd_resp("time query day"))
  if dy ~= nil then
    BN2_RT.day = dy; BN2_RT.atDay = now
    BN2_RT.rawDay = bn_sub(yraw, 1, 160)
    bn_rt_migrate(dy)
  end
  if dv == nil or dy == nil then BN2_RT.fail = BN2_RT.fail + 1 end
  return dv ~= nil and dy ~= nil
end
local function bn_rt_age()
  if BN2_RT.day == nil or BN2_RT.daytime == nil then return nil end
  return bn_now() - math.min(BN2_RT.atDay, BN2_RT.atDaytime)
end
local function bn_rt_stale()
  local age = bn_rt_age()
  return age == nil or age > BN2_RT_FRESH_SEC
end
local function bn_rt_fresh() return not bn_rt_stale() end
local function bn_is_night_dt(dt)
  return dt ~= nil and dt >= BN2_RT_NIGHT_FROM and dt < BN2_RT_NIGHT_TO
end
local function bn_is_blood_day(rd)
  local d = bn_db_load()
  if rd == nil or d.rtAnchor == nil then return false end
  if rd % BN2_PERIOD_DAYS ~= tonumber(d.rtAnchor) then return false end
  return rd ~= (tonumber(d.lastBloodDay) or -1)
end
local function bn_phase() return bn_db_load().phase end
local function bn_set_phase(p)
  local d = bn_db_load()
  d.phase = p
  bn_db_save()
  bn_cmd('scoreboard players set #phase bn_state ' .. (p == BN2_PHASES.CALM and 0 or p == BN2_PHASES.DESCENT and 2 or p == BN2_PHASES.HUNT and 3 or (p == BN2_PHASES.CLIMAX or p == BN2_PHASES.EXTEND) and 4 or 6))
end
local function bn_queue(delay, fn) BN2.queue[#BN2.queue + 1] = {at = bn_now() + delay, run = fn} end
local function bn_queue_run(now)
  if #BN2.queue == 0 then return end
  local keep = {}
  for _, it in ipairs(BN2.queue) do
    if now >= it.at then pcall(it.run) else keep[#keep + 1] = it end
  end
  BN2.queue = keep
end
local function bn_score_set(name, obj, val) bn_cmd('scoreboard players set "' .. bn_esc(name) .. '" ' .. obj .. ' ' .. math.floor(tonumber(val) or 0)) end
-- #47 机器人判定：沿用插件现有 _G.muse_is_self_account（布汲岛本体 + 账号池 bot 名单），不新造名单
local function bn_is_bot_name(name)
  if not name then return false end
  local bn = bn_bot_name()
  if bn and bn == name then return true end
  local f = rawget(_G, "muse_is_self_account")
  if type(f) == "function" then local ok, v = pcall(f, name); if ok and v == true then return true end end
  return false
end
local function bn_humans_online()
  local out = {}
  for _, nm in ipairs(bn_online()) do if not bn_is_bot_name(nm) then out[#out + 1] = nm end end
  return out
end
-- #29 服务器通用货币 = core 的 scoreboard_name() 积分/金币计分板（金币排行/转账/市场共用）
local function bn_coin_board()
  if type(scoreboard_name) == "function" then
    local ok, v = pcall(scoreboard_name)
    if ok and type(v) == "string" and v:match("^[%w_%-]+$") then return v end
  end
  return nil
end
local function bn_pay_coin(name, amt)
  amt = math.floor(tonumber(amt) or 0)
  if amt <= 0 or not name then return end
  local board = bn_coin_board()
  if not board then bn_log("coin pay skipped (no board) name=" .. tostring(name) .. " amt=" .. tostring(amt)); return end
  bn_cmd('scoreboard players add "' .. bn_esc(name) .. '" ' .. board .. ' ' .. tostring(amt))
end
local function bn_setup_scoreboards()
  bn_cmd('scoreboard objectives add bn_state dummy "血月状态"')
  bn_cmd('scoreboard objectives add bn_score dummy "血月积分"')
  bn_cmd('scoreboard objectives add bn_kills dummy "夜猎"')
  bn_cmd('scoreboard objectives add bn2cd dummy')
  bn_cmd('scoreboard objectives add bn2pill dummy')
  bn_cmd('scoreboard objectives remove bn_crystal')
  bn_cmd('scoreboard objectives remove bn_sanity')
end
local function bn_reg_count()
  local n = 0
  for _ in pairs(BN2.mobReg) do n = n + 1 end
  return n + #BN2.pending
end
local function bn_cap()
  -- v3：全局登记实体硬上限（每组另有独立上限，见 bn_group_cap）
  return BN3_GLOBAL_CAP
end
local function bn_group_cap(g)
  if not g then return BN3_GLOBAL_CAP end
  return BN3_GROUP_CAP_BASE + BN3_GROUP_CAP_PER * (tonumber(g.size) or 1)
end
local function bn_group_reg_count(idx)
  local n = 0
  for _, r in pairs(BN2.mobReg) do if r.grp == idx then n = n + 1 end end
  for _, p in ipairs(BN2.pending) do if p.grp == idx then n = n + 1 end end
  return n
end
local function bn_days_left()
  -- 真实 day 口径：血月日 = 真实 day % 8 == rtAnchor；真实数据从未取到时返回 -1（调用方显示「查询中」，不编造）
  local d = bn_db_load()
  local rd = BN2_RT.day
  if rd == nil or d.rtAnchor == nil then return -1 end
  local left = (tonumber(d.rtAnchor) - (rd % BN2_PERIOD_DAYS) + BN2_PERIOD_DAYS) % BN2_PERIOD_DAYS
  if left == 0 and (tonumber(d.lastBloodDay) or -1) == rd then left = BN2_PERIOD_DAYS end
  return left
end

-- 公告栏行（B1）：r21d_fix 的 notice_lines 会来取这一行；平时报距下次血月，进行中只报状态不带倒计时
_G.muse_bloodnight_notice_line = function()
  local d = bn_db_load()
  local phase = d.phase
  if phase ~= BN2_PHASES.CALM and phase ~= BN2_PHASES.SETTLE then
    return "§4血月 §c第 " .. tostring(d.nightNo) .. " 夜 · 第" .. tostring(BN2.waveDone or 0) .. "/4波 · 约剩" .. tostring(math.ceil(math.max(0, ((tonumber(d.started_at) or bn_now()) + BN2_NIGHT_TARGET_SEC - bn_now()) / 60))) .. "分钟"
  end
  local left = bn_days_left()
  if left < 0 then return "§4血月 §7时间查询中" end
  if left <= 0 then return "§4血月 §c今晚降临" end
  if left == 1 then return "§4血月 §e明晚降临" end
  return "§4血月 §7还有 " .. tostring(left) .. " 天"
end

-- ================= 命令链管理（B3：按需启停，禁止整晚空转） =================
local function bn_chain_set(fname, on)
  if BN2.chains[fname] == on then return end
  if type(function_registry_load) ~= "function" or type(function_registry_save) ~= "function" then return end
  local ok, d = pcall(function_registry_load)
  if not ok or type(d) ~= "table" or type(d.items) ~= "table" then return end
  local id = nil
  for k, it in pairs(d.items) do
    if type(it) == "table" and tostring(it.name) == fname then id = tonumber(it.id or k); break end
  end
  if not id then
    if type(function_register_filename) == "function" then
      local ok2, nid = pcall(function_register_filename, fname)
      if ok2 and nid then id = tonumber(nid) end
    end
    if not id then return end
    local ok3, d2 = pcall(function_registry_load)
    if ok3 and type(d2) == "table" then d = d2 end
  end
  local it = d.items[tostring(id)]
  if type(it) ~= "table" then return end
  it.enabled = on and true or false
  pcall(function_registry_save, d)
  if type(function_schedule_one) == "function" then pcall(function_schedule_one, id) end
  BN2.chains[fname] = on
end
local function bn3_all_chains()
  local t = {BN2_CHAIN_BEHAVIOR}
  for i = 1, 4 do t[#t + 1] = BN3_HAMMER_CHAINS[i] end
  for i = 1, 4 do t[#t + 1] = BN3_SHOOTER_CHAINS[i] end
  return t
end
local function bn_chains_boot()
  for _, f in ipairs(bn3_all_chains()) do
    if type(function_read_text) == "function" then
      local ok, txt = pcall(function_read_text, f)
      BN2.selftest["chain_" .. f] = (ok and type(txt) == "string" and txt ~= "") and true or false
    end
    bn_chain_set(f, false)
  end
end

-- ================= 领地（用户 2026-10-23 17:39 裁定：不避让，B4 撤销） =================
-- 拆方块可以拆进领地：原 bn2keep 禁拆标记、落点领地避让已全部移除；拆块仍只限白名单方块种类。

-- ================= 召唤落点（B14；2026-10-24 起无地面校验、环带直生） =================
-- bn_point_ok / BN2_PASSABLE / BN2_LIQUID 仅余存档的 legacy 外推函数引用，现行选点不再调用。
local BN2_PASSABLE = {air = true, grass = true, tall_grass = true, fern = true, large_fern = true,
  snow_layer = true, torch = true, soul_torch = true, redstone_torch = true, vine = true,
  dead_bush = true, seagrass = true, kelp = true, kelp_plant = true}
local BN2_LIQUID = {water = true, flowing_water = true, lava = true, flowing_lava = true}
local function bn_point_ok(pt, kind)
  -- 查不到方块（宿主接口缺失）时不阻断召唤，走优雅降级
  local feet = bn_block_id(pt.x, pt.y, pt.z)
  local head = bn_block_id(pt.x, pt.y + 1, pt.z)
  if feet and not BN2_PASSABLE[feet] and not BN2_LIQUID[feet] then return false end
  if head and not BN2_PASSABLE[head] then return false end
  if kind == "air" then return true end
  if kind == "water" then return feet == nil or BN2_LIQUID[feet] or BN2_PASSABLE[feet] end
  -- 地面怪：脚下 3 格内要有实心落脚，且落点不是岩浆/火
  if feet and (feet == "lava" or feet == "flowing_lava" or feet == "fire" or feet == "soul_fire") then return false end
  local grounded = false
  for dy = 1, 3 do
    local b = bn_block_id(pt.x, pt.y - dy, pt.z)
    if b and not BN2_PASSABLE[b] and not BN2_LIQUID[b] then grounded = true; break end
    if b and BN2_LIQUID[b] then break end
  end
  if feet ~= nil and not grounded then return false end
  return true
end
-- 选点（2026-10-24 服主新规格，覆盖上一轮 r3 的竖直扫描+锚边兜底）：27~33 格环带取到点
-- 直接在该点召唤，不验地面、不扫地表、不作废；y 取锚点玩家 y（空中怪为锚点上方），
-- 无地面就让怪自行坠落。落地成败由波次 spawnOk/spawnFail 记账，保险丝（600/400）不变。
local function bn_pick_point(anchor, kind, role)
local dirs = {{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
local dir = dirs[math.random(#dirs)]
local dist = BN2_SPAWN_MIN + math.random(0, BN2_SPAWN_MAX - BN2_SPAWN_MIN)  -- #25/#28 环带 18~24 格（原 27~33）
local x = math.floor(anchor.x + dir[1] * dist)
local z = math.floor(anchor.z + dir[2] * dist)
local y = math.floor(anchor.y)
if kind == "air" then y = y + 14 + math.random(0, 6) end
return {x = x, y = y, z = z}
end

-- ================= 刷怪导演 =================
local bn_reg_pending  -- 前置声明（认领层在事件区，实现先于事件注册即可）
local function bn_gear(sel, items)
  for slot, item in pairs(items) do
    bn_cmd("replaceitem entity " .. sel .. " " .. slot .. " 0 " .. item)
  end
end
local function bn_heal_full(sel, mtype)
  -- 亡灵（僵尸/尸壳/骷髅/凋灵骷髅/溺尸/幻翼）用瞬间伤害回血，其余用瞬间治疗
  local undead = {zombie = true, husk = true, skeleton = true, wither_skeleton = true, drowned = true, phantom = true, stray = true, bogged = true, zombie_villager = true, zoglin = true}
  if undead[mtype] then bn_cmd("effect " .. sel .. " instant_damage 1 12 true")
  else bn_cmd("effect " .. sel .. " instant_health 1 12 true") end
end
local BN2_MOUNT_OF = {chicken = true, pig = true, spider = true}
-- ================= v3 制式装备与类别命名（用户 2026-10-23 19:05/18:57） =================
-- 每只直接 summon 的血月怪都带类别名：命名实体不被系统消失（despawn）清掉，也便于按名统一操作。
-- 坐骑（ride summon_ride 召唤）基岩版无事后改名命令，是唯一例外——它们靠 tag=bn2/bn2mount 管理（报告明示）。
local BN3_MOB_NAME = {zombie = "血月僵尸", husk = "血月尸壳", skeleton = "血月骷髅", stray = "血月流浪者",
  bogged = "血月沼骸", drowned = "血月溺尸", zombie_villager = "血月僵尸村民", spider = "血月蜘蛛",
  cave_spider = "血月洞穴蜘蛛", creeper = "血月苦力怕", pillager = "血月掠夺者", vindicator = "血月卫道士",
  evoker = "血月唤魔者", witch = "血月女巫", phantom = "血月幻翼", vex = "血月恼鬼", wither_skeleton = "重锤战士"}
local function bn3_mob_name(mtype, opts)
  if opts.name then return opts.name end
  if opts.role == "she" then return "血月射手" end
  if opts.role == "po" then return "血月破阵者" end
  if opts.role == "da" then return "血月攀登者" end
  if opts.role == "pv" then return "血月铺路者" end
  if opts.role == "bm" then return "血月爆破兵" end
  if opts.role == "camp" then return "血月惩戒者" end
  return BN3_MOB_NAME[mtype] or "血月怪"
end
-- 可穿戴护甲的类人亡灵（基岩版里只有这些类型的 armor 槽 replaceitem 真能生效；其余类型改走行为链 buff+命名，报告有表）
local BN3_WEARABLE = {zombie = true, husk = true, drowned = true, zombie_villager = true,
  skeleton = true, stray = true, bogged = true, wither_skeleton = true}
local function bn3_gear_for(tagged, mtype, opts)
  if opts.boss or opts.knight then return end  -- 骑士/重锤的下界合金套由高潮代码单独配（全服仅此两类用下界合金）
  local tier = tonumber(opts.tier) or 1
  if BN3_WEARABLE[mtype] then
    local set
    if opts.elite then set = {"diamond_helmet", "diamond_chestplate", "diamond_leggings", "diamond_boots"}  -- 高阶：钻石套
    elseif opts.role == "she" then set = {"iron_helmet", "iron_chestplate", "iron_leggings", "iron_boots"}   -- 射手：铁套
    else set = {"chainmail_helmet", "chainmail_chestplate", "chainmail_leggings", "chainmail_boots"} end     -- 低阶：锁链套
    bn_gear(tagged, {["slot.armor.head"] = set[1], ["slot.armor.chest"] = set[2],
                     ["slot.armor.legs"] = set[3], ["slot.armor.feet"] = set[4]})
  end
  local weapon = nil
  if opts.elite then
    weapon = opts.weapon
    if tier >= 4 then  -- 满档精英武器升钻石（弓除外：基岩版无钻石弓）
      if weapon == "iron_sword" then weapon = "diamond_sword"
      elseif weapon == "iron_axe" then weapon = "diamond_axe" end
    end
  elseif mtype == "zombie" or mtype == "husk" or mtype == "drowned" or mtype == "zombie_villager" then
    weapon = (opts.role == "po") and "iron_pickaxe" or "iron_sword"
  elseif mtype == "skeleton" or mtype == "stray" or mtype == "bogged" then
    weapon = "bow"
  elseif mtype == "vindicator" then
    weapon = "iron_axe"
  end
  -- pillager 保留自带弩；evoker/witch/creeper/spider/cave_spider/phantom 无可用主手槽，不发
  if weapon then bn_gear(tagged, {["slot.weapon.mainhand"] = weapon}) end
end
-- 落点外推（丙3）：沿锚点→落点方向再向外推 BN3_PUSH_DIST 格，新点须过 bn_point_ok；逐级回退（15/12/9/6/3），全不过保留原点并计数
-- 【2026-10-23 晚·手感3】生成点已改约 30 格环带直生（bn_pick_point），外推 TP 修正整条停用：
-- 本函数保留签名直接返回原点；旧实现改名 legacy 留档，不再执行。
local function bn3_push_point(anchorName, pt, kind)
  return pt
end
local function bn3_push_point_legacy(anchorName, pt, kind)
  if kind == "air" then return pt end  -- 幻翼空中点不推（无基地问题）
  local anchor = bn_pos(anchorName)
  if not anchor then return pt end
  local dx, dz = pt.x - anchor.x, pt.z - anchor.z
  local len = math.sqrt(dx * dx + dz * dz)
  if len < 0.5 then return pt end
  local ux, uz = dx / len, dz / len
  for _, dist in ipairs({BN3_PUSH_DIST, 12, 9, 6, 3}) do
    local cand = {x = math.floor(pt.x + ux * dist), y = pt.y, z = math.floor(pt.z + uz * dist)}
    if bn_point_ok(cand, kind) then
      bn_cmd("tp @e[tag=bn2,x=" .. pt.x .. ",y=" .. pt.y .. ",z=" .. pt.z .. ",r=3,c=1] " .. cand.x .. " " .. cand.y .. " " .. cand.z)
      BN2.counters.push = BN2.counters.push + 1
      return cand
    end
  end
  BN2.counters.pushFallback = BN2.counters.pushFallback + 1
  return pt
end
-- opts: {role, pts, name, elite, knight, boss, ride, tier, grp, kind, weapon}；返回最终落点（外推后）或 nil
local bn_dress_one  -- 前置声明（实现紧随 bn_spawn_one 之后）：配装/效果统一补发
local function bn_spawn_one(anchorName, pt, mtype, opts)
  opts = opts or {}
  if bn_reg_count() >= bn_cap() then return nil end
  if opts.grp and bn_group_reg_count(opts.grp) >= bn_group_cap(BN2.groups[opts.grp]) then return nil end
  local kind = opts.kind or "ground"
  local sel = "@e[type=" .. mtype .. ",x=" .. pt.x .. ",y=" .. pt.y .. ",z=" .. pt.z .. ",r=3,c=1]"
  local nm = bn3_mob_name(mtype, opts)
  bn_cmd('summon ' .. mtype .. ' "' .. nm .. '" ' .. pt.x .. " " .. pt.y .. " " .. pt.z)
  bn_cmd("tag " .. sel .. " add bn2")
  if opts.role == "po" then bn_cmd("tag " .. sel .. " add bn2po")
  elseif opts.role == "da" then
    bn_cmd("tag " .. sel .. " add bn2da")
    bn_cmd("tag " .. sel .. " add " .. (opts.matTag or "bn2tu"))
  elseif opts.role == "she" then
    bn_cmd("tag " .. sel .. " add bn2she")
    bn_cmd("tag " .. sel .. " add bn2sht" .. tostring(tonumber(opts.tier) or 4))
  end
  if opts.role == "pv" then bn_cmd("tag " .. sel .. " add bn2pv")
  elseif opts.role == "bm" then bn_cmd("tag " .. sel .. " add bn2bm") end
  if opts.knight then bn_cmd("tag " .. sel .. " add bn2knight") end
  if opts.boss then
    bn_cmd("tag " .. sel .. " add bn2boss")
    bn_cmd("tag " .. sel .. " add bn2bt" .. tostring(tonumber(opts.tier) or 4))
  end
  pt = bn3_push_point(anchorName, pt, kind)  -- 外推已停用（约 30 格环带直生），pt 即最终落点
  local tagged = "@e[tag=bn2,type=" .. mtype .. ",x=" .. pt.x .. ",y=" .. pt.y .. ",z=" .. pt.z .. ",r=4,c=1]"
  if opts.deferDress then
    -- 手感2批量并发：波次分两段——先全波 summon+打标+登记（怪成批同时出现），配装/效果随后统一补
    BN2.dressQueue = BN2.dressQueue or {}
    BN2.dressQueue[#BN2.dressQueue + 1] = {tagged = tagged, mtype = mtype, opts = opts, pt = pt}
  else
    bn_dress_one(tagged, mtype, opts, pt)
  end
  if opts.ride then
    bn_cmd("execute as " .. tagged .. " at @s run ride @s summon_ride " .. opts.ride .. " skip_riders")
    local msel = "@e[type=" .. opts.ride .. ",x=" .. pt.x .. ",y=" .. pt.y .. ",z=" .. pt.z .. ",r=3,c=1]"
    bn_cmd("tag " .. msel .. " add bn2")
    bn_cmd("tag " .. msel .. " add bn2mount")
    if opts.ride == "pig" then bn_cmd("replaceitem entity " .. msel .. " slot.saddle 0 saddle") end
    bn_cmd("effect " .. msel .. " health_boost 9999 2 true")
    bn_cmd("effect " .. msel .. " speed 9999 0 true")
    bn_heal_full(msel, opts.ride)
    bn_reg_pending(opts.ride, pt, {pts = 0, mount = true, grp = opts.grp, tier = opts.tier})
  end
  opts.name = nm
  bn_reg_pending(mtype, pt, opts)
  return pt
end

bn_dress_one = function(tagged, mtype, opts, pt)
  bn3_gear_for(tagged, mtype, opts)
  if opts.elite then
    bn_cmd("effect " .. tagged .. " health_boost 9999 " .. (BN3_ELITE_HP_AMP[tonumber(opts.tier) or 3] or 3) .. " true")
    bn_heal_full(tagged, mtype)
  end
  -- 手感4药水效果（2026-10-23 晚）：全体速度 I；精英/骑士/重锤再叠力量+抗性（随档位）
  if opts.airdrop then bn_cmd("effect " .. tagged .. " resistance 4 2 true") end  -- #45/#46 空降怪生成后 4 秒抗性 III（amplifier 2）
  bn_cmd("effect " .. tagged .. " speed 9999 0 true")
  if opts.elite or opts.knight or opts.boss then
    local tier = tonumber(opts.tier) or 1
    local strAmp = 0
    if opts.boss then strAmp = (tier >= 4) and 2 or 1
    elseif opts.knight then strAmp = 1
    elseif tier >= 3 then strAmp = 1 end
    bn_cmd("effect " .. tagged .. " strength 9999 " .. strAmp .. " true")
    bn_cmd("effect " .. tagged .. " resistance 9999 " .. (opts.boss and 1 or 0) .. " true")
  end
end

local function bn_dress_flush()
  local q = BN2.dressQueue
  if not q or #q == 0 then return end
  BN2.dressQueue = {}
  for _, it in ipairs(q) do pcall(bn_dress_one, it.tagged, it.mtype, it.opts, it.pt) end
end

local BN2_ELITE_NAME = {zombie = "血仆·锐", husk = "血仆·蛮", skeleton = "血仆·影", vindicator = "血仆·刑"}
local function bn_spawn_for(anchorName, spec)
  local anchor = bn_pos(anchorName)
  if not anchor then return nil end
  local kind = "ground"
  if spec.type == "phantom" then kind = "air" end
  if spec.airdrop then kind = "air" end  -- #45 空降批次走空中点（锚点上方 14~20 格）
  if spec.type == "drowned" then kind = "water" end
local pt = bn_pick_point(anchor, kind, spec.role)
if not pt then return nil end
  spec.kind = kind
  return bn_spawn_one(anchorName, pt, spec.type, spec)
end

local BN3_ELITE_PLAN = {{0, 1, 1}, {1, 1, 2}, {1, 2, 2}, {2, 2, 3}}  -- [档][波2/3/4] 精英数（波1 恒 0）；2026-10-23 晚按手感上调
-- #29/#35/#37 生存奖励：组推进到下一波/高潮/夜终时，给本组本波没死过的在线真人发金币（逐波递增）
local function bn_pay_survival(g, upto)  -- note: bn_tell is declared later in the file, so messaging here must use bn_send (declared near the top of the section)
  if not g then return end
  upto = math.min(tonumber(upto) or 0, #BN2_WAVE_SURVIVE_BONUS)
  local paid = tonumber(g.survPaid) or 0
  if upto <= paid then return end
  local online = {}
  for _, nm in ipairs(bn_online()) do online[nm] = true end
  for w = paid + 1, upto do
    local bonus = BN2_WAVE_SURVIVE_BONUS[w] or 0
    for _, nm in ipairs(g.members) do
      if online[nm] and not bn_is_bot_name(nm) and not (g.waveDied and g.waveDied[nm]) then
        bn_pay_coin(nm, bonus)
        bn_send(nm, "§4[血月] §a✓ 撑过第 " .. tostring(w) .. " 波！§6+" .. tostring(bonus) .. " 金币 §7（服务器积分计分板）")
      end
    end
  end
  g.survPaid = upto
  g.waveDied = {}
end

local function bn_compose_wave(n, g)
  -- v3：按组组装——预算随组人数缩放，精英数/强度随组档上升；每只 spec 都带本组 grp/tier
  local specs = {}
  local function add(s) s.grp = g.idx; s.tier = g.tier; specs[#specs + 1] = s end
  local s = g.size
  local budget = ({7 + 3 * s, 9 + 4 * s, 10 + 4 * s, 9 + 3 * s})[n] or (8 + 3 * s)  -- 2026-10-23 晚手感上调：波次预算约 x1.5，给足压迫感
  local ground = {{"zombie", 1}, {"husk", 1}, {"skeleton", 1}, {"spider", 1}, {"cave_spider", 1}, {"creeper", 2}}
  local function spend(pool, cost)
    local gr = pool[math.random(#pool)]
    add({type = gr[1], pts = gr[2] or gr[3] or 1})
    return cost
  end
  local used = 0
  while used < budget do
    local r = math.random()
    if n >= 2 and r < 0.20 then
      add({type = "zombie", role = "po", pts = 2}); used = used + 2
    elseif n >= 2 and r < 0.36 then
      local mats = {{"bn2tu"}, {"bn2mu"}, {"bn2shi"}}
      local m = mats[math.random(n >= 3 and 3 or 1)][1]
      add({type = "zombie", role = "da", matTag = m, pts = 2}); used = used + 2
    elseif n >= 2 and r < 0.36 then
      add({type = "pillager", pts = 3}); used = used + 3
    elseif n >= 2 and r < 0.44 then
      add({type = "phantom", pts = 2}); used = used + 2
    elseif n >= 3 and r < 0.52 then
      add({type = "evoker", pts = 5}); used = used + 3
    elseif n >= 3 and r < 0.58 then
      add({type = "witch", pts = 3}); used = used + 3
    elseif n >= 3 and r < 0.64 then
      add({type = "skeleton", role = "she", pts = 3}); used = used + 3
    elseif n >= 3 and r < 0.70 then
      local rides = {{"zombie", "chicken"}, {"zombie", "pig"}, {"skeleton", "spider"}}
      local rd = rides[math.random(#rides)]
      add({type = rd[1], ride = rd[2], pts = 3}); used = used + 3
    elseif n >= 3 and r < 0.75 then
      add({type = "creeper", pts = 3, charged = true}); used = used + 3
    elseif n >= 2 and r < 0.80 then
      add({type = "drowned", pts = 2}); used = used + 2
    elseif n >= 2 and r < 0.86 then
      add({type = "zombie", role = "pv", pts = 2}); used = used + 2  -- #32 铺路者：移动时脚下铺路（bn_paver_tick），不搭柱
    else
      used = used + spend(ground, 1)
    end
    if #specs >= 24 then break end
  end
  -- 精英位（按组档）：档1 仅第 4 波 1 只且 50% 概率落空；档3 第 4 波 50% 概率再 +1
  local elites = 0
  if n >= 2 then elites = BN3_ELITE_PLAN[g.tier][n - 1] or 0 end
  if g.tier == 1 and n == 4 and math.random() < 0.5 then elites = 0 end
  if g.tier == 3 and n == 4 and math.random() < 0.5 then elites = elites + 1 end
  local etypes = {"zombie", "husk", "skeleton", "vindicator"}
  for _ = 1, elites do
    local et = etypes[math.random(#etypes)]
    add({type = et, elite = true, name = BN2_ELITE_NAME[et], pts = BN_ELITE_PTS,
         weapon = (et == "vindicator") and "iron_axe" or (et == "skeleton") and "bow" or "iron_sword"})
  end
  -- #42-#44 爆破兵（僵尸/骷髅原型，死亡自爆点燃 TNT，见 bn_settle）：每组每波 1~2 只，硬顶 BN_BOMBER_CAP（前三波 5 / 末波 8）
  if n >= 2 then
    local capN = BN_BOMBER_CAP[n] or 5
    local have = 0
    for _, s in ipairs(specs) do if s.role == "bm" then have = have + 1 end end
    local want = math.min(math.random(1, 2), math.max(0, capN - have))
    for _ = 1, want do add({type = (math.random() < 0.5) and "zombie" or "skeleton", role = "bm", pts = 3}) end
  end
  return specs
end

local function bn_group_online_members(g)
  local online = {}
  for _, nm in ipairs(bn_online()) do online[nm] = true end
  local ms = {}
  for _, nm in ipairs(g.members) do if online[nm] and bn_pos(nm) then ms[#ms + 1] = nm end end
  return ms
end
local function bn_spawn_group_wave(g, n)
  local anchors = bn_group_online_members(g)
  g.waveDone = n
  bn_pay_survival(g, n - 1)  -- #35 推进到第 n 波 = 上一波已撑过，先结上一波生存奖励
  if #anchors == 0 then return end  -- 组员全离线：本组本波跳过（波次照常推进，不拖住整夜节奏）
local specs = bn_compose_wave(n, g)
for _, spec in ipairs(specs) do spec.deferDress = true end -- 手感2：本波先成批生成，配装统一后补
local degraded = bn_group_reg_count(g.idx) >= bn_group_cap(g) - 4 or bn_reg_count() >= bn_cap() - 4
local landed = 0
for i, spec in ipairs(specs) do
if degraded and not spec.role and not spec.elite and i % 2 == 0 then  -- #31 修复：吃紧时不再减半破阵/攀登等职能怪（旧口径是「会拆/会搭的怪没了」的原因之一），只减半无职能普通怪
spec = {type = "zombie", pts = 1, grp = g.idx, tier = g.tier} -- 实体吃紧时职能怪减半（沿袭 B11 口径，改按组计）
end
if spec.charged then
local pt = bn_spawn_for(anchors[((i - 1) % #anchors) + 1], spec)
if pt then landed = landed + 1; bn_cmd("event entity @e[tag=bn2,type=creeper,x=".. pt.x.. ",y=".. pt.y.. ",z=".. pt.z.. ",r=3,c=1] minecraft:become_charged") end
else
if bn_spawn_for(anchors[((i - 1) % #anchors) + 1], spec) then landed = landed + 1 end
end
end
-- 刷怪落地记账：整波零落地（此处锚点必非空，函数开头已早退）必须留痕，不许再无声无息
-- #45/#46 空降批次：每组每波 35% 概率额外空降 3~5 只（首只幻翼，其余僵尸/骷髅坠落步兵），抗性 III 在 bn_dress_one 发放
if math.random() < BN_AIRDROP_CHANCE then
  local cnt = math.random(3, 5)
  for k = 1, cnt do
    local at = (k == 1) and "phantom" or ((math.random() < 0.5) and "zombie" or "skeleton")
    local aspec = {type = at, pts = 2, grp = g.idx, tier = g.tier, airdrop = true, deferDress = true}
    if bn_spawn_for(anchors[((k - 1) % #anchors) + 1], aspec) then landed = landed + 1 end
  end
  bn_broadcast("§4[血月] §c天上掉下来东西了——§7抬头！空降的一批带着护光落地。")
end
BN2.counters.spawnOk = (BN2.counters.spawnOk or 0) + landed
BN2.counters.spawnFail = (BN2.counters.spawnFail or 0) + (#specs - landed)
if landed == 0 then
bn_log("wave landed ZERO grp=".. tostring(g.idx).. " wave=".. tostring(n).. " specs=".. tostring(#specs))
end
  bn_dress_flush()
  for _, nm in ipairs(anchors) do
    bn_actionbar(nm, "§4§l▓ 第 " .. n .. " 波 · 它们来了 ▓")
    bn_sound_at(nm, "mob.warden.heartbeat", 1, 0.8)
  end
  if not BN2.waveAnnounced[n] then
    BN2.waveAnnounced[n] = true
    bn_broadcast("§4[血月] §c第 " .. n .. " 波。§7红雾里有动静，背靠背，别落单。")
  end
end

function bn_any_shooter()
  for _, reg in pairs(BN2.mobReg) do if reg.role == "she" and not reg.settled then return true end end
  return false
end
function bn_any_boss()
  for _, reg in pairs(BN2.mobReg) do if reg.boss and not reg.settled then return true end end
  return false
end
local function bn_any_boss_tier(t)
  for _, reg in pairs(BN2.mobReg) do if reg.boss and not reg.settled and (tonumber(reg.tier) or 4) == t then return true end end
  for _, p in ipairs(BN2.pending) do if p.boss and (tonumber(p.tier) or 4) == t then return true end end
  return false
end
local function bn_any_shooter_tier(t)
  for _, reg in pairs(BN2.mobReg) do if reg.role == "she" and not reg.settled and (tonumber(reg.tier) or 4) == t then return true end end
  for _, p in ipairs(BN2.pending) do if p.role == "she" and (tonumber(p.tier) or 4) == t then return true end end
  return false
end
-- 命令链卫生：重锤/射手链按档启停（每档一条独立链、伤害写死链内，组间互不串扰）
local function bn_refresh_chains()
  -- #31 修复：行为链（拆/搭所依赖的 血月_行为.txt）旧版只在开场开一次，中途被关便整晚不恢复；现随 5 秒链卫生复位
  if bn_phase() ~= BN2_PHASES.CALM then bn_chain_set(BN2_CHAIN_BEHAVIOR, true) end
  for t = 1, 4 do
    bn_chain_set(BN3_HAMMER_CHAINS[t], bn_any_boss_tier(t))
    bn_chain_set(BN3_SHOOTER_CHAINS[t], bn_any_shooter_tier(t))
  end
end
local function bn_spawn_group_climax(g)
  bn_pay_survival(g, 4)  -- #35 高潮登场 = 第 4 波已撑过
  local anchors = bn_group_online_members(g)
  if #anchors == 0 then return end
  local anchorName = anchors[1]
  local anchor = bn_pos(anchorName)
  if not anchor then return end
  local pt = bn_pick_point(anchor, "ground", nil) or {x = math.floor(anchor.x) + 8, y = math.floor(anchor.y), z = math.floor(anchor.z)}
  local tier = g.tier
  local isBoss = math.random() < BN3_BOSS_RATE[tier]
  g.climaxDone = true
  BN2.anyClimax = true
  if isBoss then
    local fpt = bn_spawn_one(anchorName, pt, "wither_skeleton", {boss = true, name = BN2_BOSS_NAME, pts = 80, grp = g.idx, tier = tier}) or pt
    local sel = "@e[tag=bn2boss,type=wither_skeleton,x=" .. fpt.x .. ",y=" .. fpt.y .. ",z=" .. fpt.z .. ",r=4,c=1]"
    bn_gear(sel, {["slot.armor.head"] = "netherite_helmet", ["slot.armor.chest"] = "netherite_chestplate",
                  ["slot.armor.legs"] = "netherite_leggings", ["slot.armor.feet"] = "netherite_boots",
                  ["slot.weapon.mainhand"] = "mace", ["slot.weapon.offhand"] = "mace"})
    bn_cmd("effect " .. sel .. " health_boost 9999 " .. BN3_BOSS_HP_AMP[tier] .. " true")
    bn_cmd("effect " .. sel .. " instant_damage 1 255 true")
    bn_cmd("tag " .. sel .. " add Buff")
    bn_chain_set(BN3_HAMMER_CHAINS[tier], true)
    g.climaxKind = "boss"
    bn_title("§4§l重 锤 战 士", "§c极凶 · 它抡起了双锤")
    bn_broadcast("§4[血月] §c" .. anchorName .. " 那一组的高潮不是骑士——§f重锤战士§c 来了。小心它的五式战技，围而歼之。")
    for _, nm in ipairs(g.members) do bn_sound_at(nm, "mob.wither.spawn", 1, 0.8) end
  else
    local fpt = bn_spawn_one(anchorName, pt, "zombie", {knight = true, name = BN2_KNIGHT_NAME, pts = 30, grp = g.idx, tier = tier}) or pt
    local sel = "@e[tag=bn2knight,type=zombie,x=" .. fpt.x .. ",y=" .. fpt.y .. ",z=" .. fpt.z .. ",r=4,c=1]"
    bn_gear(sel, {["slot.armor.head"] = "netherite_helmet", ["slot.armor.chest"] = "netherite_chestplate",
                  ["slot.armor.legs"] = "netherite_leggings", ["slot.armor.feet"] = "netherite_boots",
                  ["slot.weapon.mainhand"] = BN3_KNIGHT_WEAPON[tier]})
    bn_cmd("effect " .. sel .. " health_boost 9999 " .. BN3_KNIGHT_HP_AMP[tier] .. " true")
    if tier >= 4 then bn_cmd("effect " .. sel .. " strength 9999 1 true") end  -- 满档骑士额外力量II（压过行为链统一力量I）
    bn_heal_full(sel, "zombie")
    g.climaxKind = "knight"
    bn_title("§4§l血 月 骑 士", "§c它来收今夜的账")
    bn_broadcast("§4[血月] §c血月骑士到了 §f" .. anchorName .. " §c那一组。§7打死它，或者撑到天亮。")
    for _, nm in ipairs(g.members) do bn_sound_at(nm, "mob.wither.spawn", 1, 0.7) end
    for i = 1, 2 do
      local ept = {x = fpt.x + (i == 1 and 3 or -3), y = fpt.y, z = fpt.z + 2}
      bn_spawn_one(anchorName, ept, i == 1 and "zombie" or "skeleton", {pts = 1, grp = g.idx, tier = tier})
    end
  end
end

function bn_climax_alive()
  for _, reg in pairs(BN2.mobReg) do
    if (reg.boss or reg.knight) and not reg.settled then return true end
  end
  for _, p in ipairs(BN2.pending) do
    if p.boss or p.knight then return true end
  end
  return false
end

-- ================= 实体事件层（字段宽容挖掘；宿主观测事件形状随版本变） =================
local function bn_dig_uid(e)
  if type(e) ~= "table" then return nil end
  local cands = {e.unique_id, e.entity_unique_id, e.entityUniqueId, e.uniqueId, e.uid}
  if type(e.target) == "table" then cands[#cands + 1] = e.target.unique_id; cands[#cands + 1] = e.target.entity_unique_id; cands[#cands + 1] = e.target.uniqueId end
  if type(e.fields) == "table" then cands[#cands + 1] = e.fields.unique_id; cands[#cands + 1] = e.fields.entity_unique_id; cands[#cands + 1] = e.fields.entityUniqueId end
  for _, v in ipairs(cands) do if v ~= nil and tostring(v) ~= "" then return tostring(v) end end
  return nil
end
local function bn_dig_rid(e)
  if type(e) ~= "table" then return nil end
  local cands = {e.target_runtime_id, e.runtime_id, e.entity_runtime_id}
  if type(e.target) == "table" then cands[#cands + 1] = e.target.runtime_id; cands[#cands + 1] = e.target.runtimeId end
  if type(e.fields) == "table" then cands[#cands + 1] = e.fields.runtime_id; cands[#cands + 1] = e.fields.target_runtime_id end
  for _, v in ipairs(cands) do if v ~= nil and tostring(v) ~= "" then return tostring(v) end end
  return nil
end
local function bn_dig_pos(e)
  if type(e) ~= "table" then return nil end
  local p = e.position or e.pos or (type(e.target) == "table" and (e.target.position or e.target.pos) or nil)
    or (type(e.fields) == "table" and (e.fields.position or e.fields.pos) or nil)
  if type(p) == "table" then
    local x, y, z = tonumber(p.x or p[1]), tonumber(p.y or p[2]), tonumber(p.z or p[3])
    if x and y and z then return {x = x, y = y, z = z} end
  end
  local x, y, z = tonumber(e.x), tonumber(e.y), tonumber(e.z)
  if x and y and z then return {x = x, y = y, z = z} end
  return nil
end
local function bn_dig_type(e)
  if type(e) ~= "table" then return nil end
  local t = e.entity_type or e.entityType or e.type
    or (type(e.target) == "table" and (e.target.entity_type or e.target.entityType) or nil)
    or (type(e.fields) == "table" and (e.fields.entity_type or e.fields.entityType or e.fields.type) or nil)
  if t then return bn_lower(tostring(t)):gsub("^minecraft:", "") end
  return nil
end
local function bn_dig_health(e)
  if type(e) ~= "table" then return nil, nil end
  local h = tonumber(e.health) or tonumber(e.current_health)
    or (type(e.fields) == "table" and (tonumber(e.fields.health) or tonumber(e.fields.current_health)) or nil)
    or (type(e.health_state) == "table" and tonumber(e.health_state.current) or nil)
  local m = tonumber(e.max_health) or tonumber(e.maxHealth)
    or (type(e.fields) == "table" and (tonumber(e.fields.max_health) or tonumber(e.fields.maxHealth)) or nil)
    or (type(e.health_state) == "table" and tonumber(e.health_state.max) or nil)
  return h, m
end
local function bn_dig_attacker(e)
  if type(e) ~= "table" then return nil, false end
  local name, known = nil, e.attacker_known == true
  if type(e.attacker) == "table" then name = e.attacker.name or e.attacker.Name end
  if not name then name = e.attacker_name or e.killer_name or e.killer end
  if type(name) == "table" then name = name.name or name.Name end
  if name ~= nil and tostring(name) ~= "" then known = true; name = tostring(name) else name = nil end
  return name, known
end
bn_reg_pending = function(ptype, pos, opts)
  opts = opts or {}
  BN2.pending[#BN2.pending + 1] = {t = ptype, x = pos.x, y = pos.y, z = pos.z, at = bn_now(),
    role = opts.role, pts = tonumber(opts.pts) or 1, elite = opts.elite == true, knight = opts.knight == true, bomber = opts.role == "bm",
    boss = opts.boss == true, mount = opts.mount == true, name = opts.name, grp = opts.grp, tier = opts.tier}
end
local function bn_try_claim(uid, etype, pos, rid)
  if BN2.mobReg[uid] then return end
  for i, p in ipairs(BN2.pending) do
    if etype == nil or p.t == etype then
      local okPos = true
      if pos then local dx, dy, dz = pos.x - p.x, pos.y - p.y, pos.z - p.z; okPos = (dx * dx + dy * dy + dz * dz) <= 49 end
      if okPos then
        BN2.mobReg[uid] = {pts = p.pts, role = p.role, elite = p.elite, knight = p.knight, boss = p.boss, bomber = p.bomber == true,
          mount = p.mount, name = p.name, grp = p.grp, tier = p.tier, etype = p.t, settled = false, half = false,
          lastPos = {x = p.x, y = p.y, z = p.z}, lastHurt = nil, prevHurt = nil, lastHpPct = nil}
        if rid then BN2.rid2uid[rid] = uid end
        table.remove(BN2.pending, i)
        BN2.counters.claimed = BN2.counters.claimed + 1
        return
      end
    end
  end
end
local function bn_claim_sweep()
  if #BN2.pending == 0 then return end
  if not (entities and type(entities.nearby) == "function") then return end
  local now = bn_now()
  local keep = {}
  for _, p in ipairs(BN2.pending) do
    if now - p.at <= 20 then
      local done = false
      local ok, list = pcall(entities.nearby, {x = p.x, y = p.y, z = p.z}, 7)
      if ok and type(list) == "table" then
        for _, ent in ipairs(list) do
          if type(ent) == "table" then
            local uid = ent.unique_id or ent.entity_unique_id or ent.uniqueId or ent.entityUniqueId
            local et = ent.entity_type or ent.entityType or ent.type
            if uid and (et == nil or bn_lower(tostring(et)):gsub("^minecraft:", "") == p.t) and not BN2.mobReg[tostring(uid)] then
              bn_try_claim(tostring(uid), p.t, bn_dig_pos(ent) or {x = p.x, y = p.y, z = p.z}, nil)
              done = true
              break
            end
          end
        end
      end
      if not done then keep[#keep + 1] = p end
    end
  end
  BN2.pending = keep
end

-- ================= 结算管线（B2：只认证据；B13：多通道兜底） =================
local bn_settle  -- 前置声明
local function bn_bar_add(name, pts)
  local bar = BN2.bar[name]
  if not bar then bar = {sum = 0, at = 0}; BN2.bar[name] = bar end
  bar.sum = bar.sum + pts
  bar.at = bn_now()
end
local function bn_bar_flush(now)
  for name, bar in pairs(BN2.bar) do
    if bar.sum > 0 and now - bar.at >= 1.5 then
      local d = bn_db_load()
      local tn = d.tonight[name] or {}
      bn_actionbar(name, "§d+" .. tostring(bar.sum) .. " 血月积分 §7（今夜 " .. tostring(tonumber(tn.score) or 0) .. " · 累计 " .. tostring(tonumber(d.score[name]) or 0) .. "）")
      bar.sum = 0
    end
  end
end
local function bn_attribute(reg, now)
  local last, prev = reg.lastHurt, reg.prevHurt
  local cands = {}
  if last and now - last.at <= BN2_HURT_WINDOW then cands[#cands + 1] = last end
  if prev and now - prev.at <= BN2_HURT_WINDOW and (not last or prev.name ~= last.name) then cands[#cands + 1] = prev end
  if #cands == 0 then return nil end
  if #cands == 1 then return cands[1].name end
  if math.abs(cands[1].at - cands[2].at) > BN2_TIE_WINDOW then
    return (cands[1].at >= cands[2].at) and cands[1].name or cands[2].name
  end
  -- 并列：取离死亡点最近者
  local dp = reg.lastPos or {x = 0, y = 0, z = 0}
  local best, bestD = nil, nil
  for _, c in ipairs(cands) do
    local pp = bn_pos(c.name)
    local dd = 1e18
    if pp then local dx, dy, dz = pp.x - dp.x, pp.y - dp.y, pp.z - dp.z; dd = dx * dx + dy * dy + dz * dz end
    if not bestD or dd < bestD then best, bestD = c.name, dd end
  end
  return best
end
bn_settle = function(uid, channel)
  local reg = BN2.mobReg[uid]
  if not reg or reg.settled then return end
  reg.settled = true
  BN2.mobReg[uid] = nil
  -- #42-#44 爆破兵死亡自爆：死亡点召唤一个点燃的 TNT 实体（基岩版实体 id 为 tnt，生成即引燃）；放在归属判定前，无归属也照爆
  if reg.bomber and reg.lastPos then
    local bp = reg.lastPos
    bn_cmd("summon tnt " .. tostring(math.floor(bp.x)) .. " " .. tostring(math.floor(bp.y)) .. " " .. tostring(math.floor(bp.z)))
  end
  local now = bn_now()
  local killer = bn_attribute(reg, now)
  if channel == "kill" then BN2.counters.settleKill = BN2.counters.settleKill + 1
  elseif channel == "health" then BN2.counters.settleHealth = BN2.counters.settleHealth + 1
  elseif channel == "playerkill" then BN2.counters.settlePlayerKill = BN2.counters.settlePlayerKill + 1
  elseif channel == "vanish" then BN2.counters.settleVanish = BN2.counters.settleVanish + 1 end
  if reg.boss or reg.knight then
    bn_refresh_chains()
  end
  if not killer then
    -- 兜底档（用户 2026-10-23 17:39 裁定）：认不出攻击者时，记给死亡点 16 格内最近的在场玩家；半径内无人则不给分
    local dp = reg.lastPos
    if dp then
      local bestD = BN2_FALLBACK_RADIUS * BN2_FALLBACK_RADIUS
      for _, nm in ipairs(bn_online()) do
        local pp = bn_pos(nm)
        if pp then
          local dx, dy, dz = pp.x - dp.x, pp.y - dp.y, pp.z - dp.z
          local dd = dx * dx + dy * dy + dz * dz
          if dd <= bestD then killer, bestD = nm, dd end
        end
      end
    end
    if killer then BN2.counters.settleFallback = BN2.counters.settleFallback + 1 end
  end
  if not killer then
    BN2.counters.noEvidence = BN2.counters.noEvidence + 1
    return
  end
  local d = bn_db_load()
  local pts = reg.mount and 0 or (tonumber(reg.pts) or 1)
  if pts > 0 then
    d.score[killer] = (tonumber(d.score[killer]) or 0) + pts
    local tn = d.tonight[killer] or {score = 0, kills = 0}
    tn.score = (tonumber(tn.score) or 0) + pts
    tn.kills = (tonumber(tn.kills) or 0) + 1
    d.tonight[killer] = tn
    d.stats.kills = (tonumber(d.stats.kills) or 0) + 1
    BN2.counters.points = BN2.counters.points + pts
    bn_score_set(killer, "bn_score", d.score[killer])
    bn_score_set(killer, "bn_kills", tn.kills)
    bn_bar_add(killer, pts)
  end
  if reg.elite then bn_cmd('give "' .. bn_esc(killer) .. '" emerald 3') end
  if reg.knight then
    bn_cmd('give "' .. bn_esc(killer) .. '" emerald 8')
    bn_broadcast("§4[血月] §c血月骑士倒在了 §f" .. killer .. " §c手里。")
  end
  if reg.boss then
    -- v3 奖励升级（用户 2026-10-23 批「奖励弄好一点」提案数值）
    bn_cmd('give "' .. bn_esc(killer) .. '" emerald 16')
    bn_cmd('give "' .. bn_esc(killer) .. '" diamond 4')
    bn_cmd('give "' .. bn_esc(killer) .. '" netherite_ingot 1')
    bn_cmd('give "' .. bn_esc(killer) .. '" enchanted_golden_apple 1')
    bn_broadcast("§4[血月] §c重锤战士被 §f" .. killer .. " §c终结了！")
  end
  bn_db_save()
end
local function bn_elite_half(reg)
  if reg.half or not reg.elite then return end
  reg.half = true
  local pos = reg.lastPos
  if not pos then return end
  -- 精英半血演出：参考稿 playanimation 咆哮位（命令已被宿主接受并下发；该动画是否在该实体客户端定义内属客户端侧，见 v3 报告说明）
  if reg.name then
    bn_cmd('playanimation @e[name="' .. bn_esc(reg.name) .. '",x=' .. math.floor(pos.x) .. ",y=" .. math.floor(pos.y) .. ",z=" .. math.floor(pos.z) .. ',r=4,c=1] animation.armor_stand.zombie_pose default')
  end
  for i = 1, 2 do
    local pt = {x = pos.x + (i == 1 and 2 or -2), y = pos.y, z = pos.z + 1}
    bn_spawn_one("召唤", pt, i == 1 and "zombie" or "skeleton", {pts = 1, grp = reg.grp, tier = reg.tier})
  end
end
local function bn_reg_from_event(e)
  local uid = bn_dig_uid(e)
  if not uid then
    local rid = bn_dig_rid(e)
    if rid and BN2.rid2uid[rid] then uid = BN2.rid2uid[rid] end
  end
  if not uid then return nil, nil end
  local reg = BN2.mobReg[uid]
  if not reg then
    -- 事件先于刷怪广播到达时，借这次事件就地认领，避免早期攻击证据丢失
    bn_try_claim(uid, bn_dig_type(e), bn_dig_pos(e), bn_dig_rid(e))
    reg = BN2.mobReg[uid]
  end
  if reg then
    local pos = bn_dig_pos(e)
    if pos then reg.lastPos = pos end
    local rid = bn_dig_rid(e)
    if rid then BN2.rid2uid[rid] = uid end
  end
  return uid, reg
end
local bn_last_hurt_key, bn_last_hurt_at = nil, 0
function museBloodNight2Hurt(e)
  if type(e) ~= "table" then return end
  local uid, reg = bn_reg_from_event(e)
  if not uid then BN2.counters.hurtNoUid = BN2.counters.hurtNoUid + 1; return end
  local aname, aknown = bn_dig_attacker(e)
  local cause = tostring(e.cause or e.damage_cause or e.damage_source or e.source or "unknown")
  -- 双注册去重：同一事件（uid+攻击者+原因）在 0.1 秒内只计一次
  local key = uid .. "|" .. tostring(aname) .. "|" .. cause
  local now = bn_now()
  if key == bn_last_hurt_key and now - bn_last_hurt_at < 0.1 then return end
  bn_last_hurt_key, bn_last_hurt_at = key, now
  BN2.counters.hurtSeen = BN2.counters.hurtSeen + 1
  BN2.causeSeen[cause] = (BN2.causeSeen[cause] or 0) + 1
  if aknown and aname then BN2.counters.hurtKnown = BN2.counters.hurtKnown + 1 end
  if not reg or reg.settled then return end
  local rid = bn_dig_rid(e)
  if rid and entities and type(entities.health) == "function" then
    local ok, h = pcall(entities.health, rid)
    if ok and type(h) == "table" and tonumber(h.current) and tonumber(h.max) and tonumber(h.max) > 0 then
      reg.lastHpPct = tonumber(h.current) / tonumber(h.max)
      if reg.elite and reg.lastHpPct <= 0.5 then bn_elite_half(reg) end
    end
  end
  if aknown and aname and aname ~= "" then
    if not reg.lastHurt or reg.lastHurt.name ~= aname then
      reg.prevHurt = reg.lastHurt
    end
    reg.lastHurt = {name = aname, at = now}
  end
end
function museBloodNight2Health(e)
  if type(e) ~= "table" then return end
  BN2.counters.healthSeen = BN2.counters.healthSeen + 1
  local uid, reg = bn_reg_from_event(e)
  if not reg or reg.settled then return end
  local h, m = bn_dig_health(e)
  if h ~= nil and m and m > 0 then
    reg.lastHpPct = h / m
    if reg.elite and h / m <= 0.5 then bn_elite_half(reg) end
    if h <= 0 then bn_settle(uid, "health") end
  elseif h ~= nil and h <= 0 then
    bn_settle(uid, "health")
  end
end
function museBloodNight2Kill(e)
  if type(e) ~= "table" then return end
  BN2.counters.killSeen = BN2.counters.killSeen + 1
  local eid = e.event_id and tostring(e.event_id) or nil
  if eid then
    if BN2.killEventIds[eid] then return end
    BN2.killEventIds[eid] = true
  end
  local uid, reg = bn_reg_from_event(e)
  if not reg or reg.settled then return end
  local aname, aknown = bn_dig_attacker(e)
  if aknown and aname and aname ~= "" then
    reg.lastHurt = {name = aname, at = bn_now()}
    bn_settle(uid, "kill")
  end
end
function museBloodNight2PlayerKill(e)
  -- 宿主 onPlayerKill：死亡翻译消息关联的击杀（多见于命名怪）；只对骑士/Boss 这种唯一名目标认领
  if type(e) ~= "table" then return end
  local killer = e.killer or e.killer_name or e.attacker_name
  if type(killer) == "table" then killer = killer.name end
  if not killer or tostring(killer) == "" then return end
  killer = tostring(killer)
  local tname = nil
  if type(e.target) == "table" then tname = e.target.name end
  tname = tname or e.target_name or e.victim_name or e.entity_name
  if not tname then return end
  for uid, reg in pairs(BN2.mobReg) do
    if not reg.settled and (reg.boss or reg.knight) and reg.name == tostring(tname) then
      reg.lastHurt = {name = killer, at = bn_now()}
      bn_settle(uid, "playerkill")
      return
    end
  end
end
function museBloodNight2Spawn(e)
  -- R108: no bloodnight mob tracking while game phase is CALM.
  -- Prevent tens of thousands of unrelated actor packets being parsed.
  if bn_phase()==BN2_PHASES.CALM then return end
  BN2.counters.spawnSeen = BN2.counters.spawnSeen + 1
  local uid = bn_dig_uid(e)
  if not uid then return end
  bn_try_claim(uid, bn_dig_type(e), bn_dig_pos(e), bn_dig_rid(e))
end
function museBloodNight2Remove(e)
  local uid = bn_dig_uid(e)
  if not uid then
    local rid = bn_dig_rid(e)
    if rid then uid = BN2.rid2uid[rid] end
  end
  if not uid then return end
  local reg = BN2.mobReg[uid]
  if reg and not reg.settled then
    local now = bn_now()
    if reg.lastHpPct and reg.lastHpPct <= 0.35 and reg.lastHurt and now - reg.lastHurt.at <= BN2_VANISH_WINDOW then
      bn_settle(uid, "vanish")
      return
    end
    BN2.mobReg[uid] = nil
  end
end
function museBloodNight2PlayerDeath(e)
  local phase = bn_phase()
  if phase ~= BN2_PHASES.HUNT and phase ~= BN2_PHASES.CLIMAX and phase ~= BN2_PHASES.EXTEND then return end
  if type(e) ~= "table" then return end
  local dd = type(e.data) == "table" and e.data or {}
  local raw = e.player or e.name or dd.player or dd.name
  if type(raw) == "table" then raw = raw.name or raw.Name end
  local name = tostring(raw or "")
  if name == "" then return end
  local d = bn_db_load()
  d.died[name] = true
  bn_db_save()
  local gi = BN2.groupOf and BN2.groupOf[name]
  local gg = gi and BN2.groups and BN2.groups[gi]
  if gg then gg.waveDied = gg.waveDied or {}; gg.waveDied[name] = true end  -- #35 本波死过不领本波生存奖励
end

-- ================= 夜流程 =================
local function bn_tell(name, text)
  local j = text:gsub("\\", "\\\\"):gsub('"', '\\"'):gsub("\n", " ")
  bn_cmd('tellraw "' .. bn_esc(name) .. '" {"rawtext":[{"text":"' .. j .. '"}]}')
end
local function bn_privileged(name)
  local ok = false
  local okp, r = pcall(function() return _G.muse_is_privileged and _G.muse_is_privileged(name) end)
  if okp and r then ok = true end
  local okn, r2 = pcall(function() return _G.muse_is_native_op and _G.muse_is_native_op(name) end)
  if okn and r2 then ok = true end
  if name == "疣螈" then ok = true end
  return ok
end
local function bn_store_rules()
  local d = bn_db_load()
  d.rules = d.rules or {}
  local w = bn_resp_text(bn_cmd_resp("weather query"))
  if w:find("rain") or w:find("雨") then d.rules.weather = "rain"
  elseif w:find("thunder") or w:find("雷") then d.rules.weather = "thunder" else d.rules.weather = "clear" end
  local df = bn_lower(bn_resp_text(bn_cmd_resp("difficulty")))
  if df:find("peaceful") or df:find("和平") then d.rules.difficulty = "peaceful"
  elseif df:find("easy") or df:find("简单") then d.rules.difficulty = "easy"
  elseif df:find("hard") or df:find("困难") then d.rules.difficulty = "hard" else d.rules.difficulty = "normal" end
  d.rulesKnown = true
  bn_db_save()
end
-- ================= 中夜冻结（用户 2026-10-23 18:15 裁定） =================
-- 夜到中夜关 doDaylightCycle：世界时钟停住；公告栏时钟靠 SetTime 包与 time query 命令校准到世界时间，会随之停在中夜。
-- 冻结长度按真实时钟倒算：解冻时刻 = 开夜时刻 + 目标总长 BN2_NIGHT_TARGET_SEC − 尾段预留 BN2_FREEZE_TAIL_SEC。
local function bn_unfreeze(why)
  local d = bn_db_load()
  if not d.frozen then return end
  local orig = d.freeze_orig
  if orig == nil then orig = true end
  bn_cmd("gamerule doDaylightCycle " .. (orig and "true" or "false"))
  d.frozen = false
  d.freeze_start = nil
  d.freeze_until = nil
  d.freeze_orig = nil
  bn_db_save()
  bn_log("unfreeze why=" .. tostring(why))
end
local function bn_freeze_begin(now)
  local d = bn_db_load()
  if d.frozen or d.freeze_done then return end
  local orig = bn_query_daylight()
  if orig == nil then orig = true end  -- 查不到按基岩版默认 true 记账（本服公告栏昼夜系统本就依赖它为 true）
  d.freeze_orig = orig
  bn_cmd("gamerule doDaylightCycle false")
  d.frozen = true
  d.freeze_done = true
  d.freeze_start = now
  d.freeze_until = (tonumber(d.started_at) or now) + BN2_NIGHT_TARGET_SEC  -- #34 全程冻结：冻满整晚目标时长（原中夜尾段预留公式废止）
  bn_db_save()
  bn_broadcast("§4[血月] §7夜到中天，月亮停住了——今夜还长。")
  bn_log("freeze begin until=" .. tostring(d.freeze_until))
end
local function bn_freeze_timer(now)
  local d = bn_db_load()
  if not d.frozen then return end
  if bn_phase() == BN2_PHASES.CALM then bn_unfreeze("residual"); return end
  local until_ = tonumber(d.freeze_until) or 0
  local start_ = tonumber(d.freeze_start) or now
  if now >= until_ or now - start_ >= BN2_FREEZE_HARD_CAP then bn_unfreeze("timer") end
end
local function bn_cleanup_world()
  local d = bn_db_load()
  bn_unfreeze("cleanup")
  -- v3 虚空清场（用户裁定：残怪不原地抹掉，先送虚空再删）：跨维度 execute in 本宿主实测不支持（2026-10-23 探针语法错误），
  -- 退同维度虚空：全体血月怪先 TP 到 y=-100（基岩层下虚空）坠毁，随后 kill 扫尾仍留在加载区的残余。
  bn_cmd("execute as @e[tag=bn2] at @s run tp @s ~ -100 ~")
  bn_cmd("kill @e[tag=bn2]")
  if type(_G.r21d_fog_clear_all_for_player) == "function" then
    for _, nm in ipairs(bn_online()) do pcall(_G.r21d_fog_clear_all_for_player, nm, false) end
  end
  bn_cmd("fog @a remove bn_red")
  bn_cmd("fog @a remove bn_deep")
  bn_cmd("fog @a remove bn_fade")
  bn_cmd("fog @a pop minecraft:fog_crimson_forest")
  -- #33 清理天空变暗残留：红雾 pop 之外再 remove 兜底，并清可能残留的黑暗/失明；昼夜循环已由上方 bn_unfreeze 按原值恢复
  bn_cmd("fog @a remove minecraft:fog_crimson_forest")
  bn_cmd("effect @a darkness 0 0 true")
  bn_cmd("effect @a blindness 0 0 true")
  if d.rulesKnown and d.rules then
    if d.rules.weather == "rain" then bn_cmd("weather rain 6000")
    elseif d.rules.weather == "thunder" then bn_cmd("weather thunder 6000")
    else bn_cmd("weather clear 6000") end
    if d.rules.difficulty then bn_cmd("difficulty " .. d.rules.difficulty) end
  else
    bn_cmd("weather clear 6000")
  end
  bn_cmd("gamerule mobgriefing false")
  bn_chain_set(BN2_CHAIN_BEHAVIOR, false)
  for t = 1, 4 do
    bn_chain_set(BN3_HAMMER_CHAINS[t], false)
    bn_chain_set(BN3_SHOOTER_CHAINS[t], false)
  end
end

-- ================= v3 分组（开场聚类，整晚定型） =================
-- 算法（贪心质心聚类，水平距离）：按在线顺序取未分组者为种子；反复把「与当前组质心水平距离 ≤ 扩组半径」
-- 的最近未分组者吸入（加入第 2/3/4/5 人的半径 = 100/150/200/250 格），满 5 人或无人可吸为止；无坐标者自成单人组。
local function bn_cluster(online)
  local entries = {}
  for _, nm in ipairs(online) do entries[#entries + 1] = {name = nm, pos = bn_pos(nm)} end
  local used, groups = {}, {}
  for _, seed in ipairs(entries) do
    if not used[seed.name] then
      used[seed.name] = true
      local members, poss = {seed.name}, {}
      if seed.pos then
        poss[#poss + 1] = seed.pos
        while #members < 5 do
          local cx, cz, cnt = 0, 0, 0
          for _, p in ipairs(poss) do cx = cx + p.x; cz = cz + p.z; cnt = cnt + 1 end
          cx, cz = cx / cnt, cz / cnt
          local limit = BN3_CLUSTER_R[#members] or 250
          local best, bestD = nil, nil
          for _, e in ipairs(entries) do
            if not used[e.name] and e.pos then
              local dx, dz = e.pos.x - cx, e.pos.z - cz
              local dd = math.sqrt(dx * dx + dz * dz)
              if dd <= limit and (not bestD or dd < bestD) then best, bestD = e, dd end
            end
          end
          if not best then break end
          used[best.name] = true
          members[#members + 1] = best.name
          poss[#poss + 1] = best.pos
        end
      end
      groups[#groups + 1] = {idx = #groups + 1, members = members, size = #members,
        tier = bn_tier_of(#members), waveDone = 0, climaxDone = false, climaxKind = nil}
    end
  end
  return groups
end

local function bn_begin_night(manual)
  local d = bn_db_load()
  if bn_phase() ~= BN2_PHASES.CALM then return false, "已经在血月里了" end
  local online = bn_online()
  if #online == 0 then return false, "现在没人在线" end
  -- #47 只有机器人在线时不开血月（不递增夜数、不启动计时）：至少 1 名真人在线
  local humans = bn_humans_online()
  if #humans == 0 then return false, "只有机器人在线，不开启血月" end
  bn_store_rules()
  d = bn_db_load()
  d.nightNo = (tonumber(d.nightNo) or 0) + 1
  d.lastBloodDay = BN2_RT.day or tonumber(d.lastBloodDay) or -1  -- 真实 day 回写（2026-10-23 整改；自数 dayCount 退役）
  d.started_at = bn_now()
  if d.frozen then bn_unfreeze("begin_guard") end
  d.frozen = false
  d.freeze_done = false
  d.freeze_start = nil
  d.freeze_until = nil
  d.freeze_orig = nil
  d.keepSeenOff = false  -- 遗留字段：keep 收尾已于 2026-10-23 晚整条删除，本字段不再参与任何判定
  d.participants = humans  -- #47 参与者只算真人（机器人不进组、不当锚点、不领奖励）；中途加入真人由 bn_sync_participants 补登记
  d.tonight = {}
  d.died = {}
  BN2.manualNight = manual == true
  BN2.nightSeen = bn_is_night_dt(BN2_RT.daytime)  -- 开场时已在夜窗内也算见过夜窗
  BN2.waveDone = 0
  BN2.showStage = 0
  BN2.climaxKind = nil
  BN2.extendStart = 0
  BN2.mobReg, BN2.pending, BN2.rid2uid, BN2.bar = {}, {}, {}, {}
  -- v3 分组：开场聚类定型，整晚不变（中途加入者只进 participants 记分，不进组、不当锚点、不改档位）
  BN2.groups = bn_cluster(humans)
  BN2.groupOf = {}
  BN2.anyClimax = false
  BN2.waveAnnounced = {}
  local gsum = {}
  for _, g in ipairs(BN2.groups) do
    for _, nm in ipairs(g.members) do BN2.groupOf[nm] = g.idx end
    gsum[#gsum + 1] = {members = g.members, size = g.size, tier = g.tier}
  end
  d.groups = gsum
  bn_db_save()
  bn_log("groups formed: " .. tostring(#BN2.groups))
  bn_set_phase(BN2_PHASES.DESCENT)
  bn_cmd("difficulty hard")
  bn_cmd("gamerule mobgriefing true")
  bn_cmd("weather thunder 9999")
  if type(_G.r21d_fog_clear_all_for_player) == "function" then
    for _, nm in ipairs(online) do pcall(_G.r21d_fog_clear_all_for_player, nm, false) end
  end
  bn_cmd("fog @a pop minecraft:fog_crimson_forest")
  bn_cmd("fog @a push minecraft:fog_crimson_forest bn_red")
  bn_cmd("fog @a remove bn_fade")
  bn_chain_set(BN2_CHAIN_BEHAVIOR, true)
  bn_freeze_begin(bn_now())  -- #34 全程冻结：开场即关昼夜循环整晚不动（暂停而非拨表，符合真实时间神圣）；到点由真实时钟收尾
  bn_title("§4§l血 月", "§c今夜的月亮是红的")
  for _, nm in ipairs(online) do bn_sound_at(nm, "bell.hit", 1, 1.0) end
  bn_broadcast("§4[血月] §c第 " .. tostring(d.nightNo) .. " 个血月夜。§7红雾压境，雷雨将至——活下去，或者打出今晚的排名。")
  if #BN2.groups >= 2 then
    bn_broadcast("§4[血月] §7今晚分 " .. tostring(#BN2.groups) .. " 组各自为战——离得近的，背靠背。")
  end
  return true, "第 " .. tostring(d.nightNo) .. " 夜开始"
end
local function bn_finish_night(reason)
  local d = bn_db_load()
  if bn_phase() == BN2_PHASES.CALM then return end
  -- 今夜战绩播报
  local top, topScore, earners = nil, 0, 0
  for nm, tn in pairs(d.tonight) do
    if (tonumber(tn.score) or 0) > 0 then earners = earners + 1 end
    if (tonumber(tn.score) or 0) > topScore then top, topScore = nm, tonumber(tn.score) end
  end
  -- #29/#35/#36 夜终结算（手动关闭不补最后一波）：补发最后一波生存奖励 + 今夜积分 2:1 换金币（一次性） + 全满奖励
  if reason ~= "manual" then
    for _, g in ipairs(BN2.groups or {}) do bn_pay_survival(g, tonumber(g.waveDone) or 0) end
  end
  if not BN2.coinSettled then
    BN2.coinSettled = true
    for nm, tn in pairs(d.tonight) do
      if not bn_is_bot_name(nm) then
        local conv = math.floor((tonumber(tn.score) or 0) / BN2_SCORE_COIN_DIV)
        if conv > 0 then bn_pay_coin(nm, conv); bn_tell(nm, "§4[血月] §7今夜积分换算 §6+" .. tostring(conv) .. " 金币 §7（2 分 = 1 金币）") end
        local gi2 = BN2.groupOf and BN2.groupOf[nm]
        local gg2 = gi2 and BN2.groups and BN2.groups[gi2]
        if reason ~= "manual" and gg2 and (tonumber(gg2.waveDone) or 0) >= 4 and gg2.climaxDone and not d.died[nm] then
          bn_pay_coin(nm, BN2_FULL_CLEAR_BONUS)
          bn_tell(nm, "§4[血月] §6✦ 全波撑过、整夜未倒——满额奖励 §6+" .. tostring(BN2_FULL_CLEAR_BONUS) .. " 金币")
        end
      end
    end
  end
  bn_cleanup_world()
  bn_cleanup_world()
  -- 死亡玩家夜终复活点：送到一位存活参与者身边
  local anchor = nil
  for _, nm in ipairs(d.participants) do
    if not d.died[nm] and bn_pos(nm) then anchor = nm; break end
  end
  if anchor then
    for nm in pairs(d.died) do
      if nm ~= anchor and bn_pos(nm) then bn_cmd('tp "' .. bn_esc(nm) .. '" "' .. bn_esc(anchor) .. '"') end
    end
  end
  d.stats.nights = (tonumber(d.stats.nights) or 0) + 1
  d.died = {}
  bn_set_phase(BN2_PHASES.CALM)
  BN2.mobReg, BN2.pending, BN2.rid2uid, BN2.bar = {}, {}, {}, {}
  BN2.queue = {}  -- 手感1：关闭即取消全部已排队调度（波次/演出队列一并清，不许残留再刷）
  BN2.dressQueue = {}
  BN2.waveDone = 0
  -- 手感1补强：宿主命令队列里可能还压着未落地的 summon，收尾后 4 秒、12 秒各补扫一次残怪
  BN2.finishEpoch = (BN2.finishEpoch or 0) + 1
  local fep = BN2.finishEpoch
  -- 补扫四档（4/12/30/60 秒）：实测宿主命令积压时关夜前发出的 summon 会迟到落地（22:0x 实测
  -- +12 秒档后仍有 5 只残怪靠人工补扫才清掉），故拉长覆盖窗；每档仍带相位/epoch 自检。
  for _, delay in ipairs({4, 12, 30, 60}) do
    bn_queue(delay, function() if bn_phase() == BN2_PHASES.CALM and BN2.finishEpoch == fep then bn_cmd("kill @e[tag=bn2]") end end)
  end
  bn_title("§7§l天 亮 了", "§f血月散去")
  if top then
    bn_broadcast("§4[血月] §7今夜 " .. tostring(earners) .. " 人挣到积分，§f" .. top .. " §7以 §d" .. tostring(topScore) .. " §7分领跑。各回各家，天亮了。")
  else
    bn_broadcast("§4[血月] §7血月散了。今晚没人挣到积分——下次记得动手。")
  end
end
local function bn_sync_participants()
  -- v3：中途加入者只登记进 participants（可得分/兜底/看状态），不进组、不当刷怪锚点、不改变已定型档位
  local d = bn_db_load()
  if bn_phase() == BN2_PHASES.CALM then return end
  local seen, changed = {}, false
  for _, nm in ipairs(d.participants) do seen[nm] = true end
  for _, nm in ipairs(bn_online()) do
    if not seen[nm] then d.participants[#d.participants + 1] = nm; changed = true end
  end
  if changed then bn_db_save() end
end
-- ================= 主 tick =================
local function bn_tick_inner()
  local now = bn_now()
  if now - BN2.lastTickAt < 0.9 then return end
  BN2.lastTickAt = now
  local d = bn_db_load()
  local phase = bn_phase()
  if phase ~= BN2_PHASES.CALM then
    bn_claim_sweep()
    bn_bar_flush(now)
  end
  bn_queue_run(now)  -- 队列在 CALM 相位也要跑：收尾补扫靠它执行；队列项均为血月内部闭包且自带相位/epoch 自检
  bn_paver_tick(now)   -- #32 铺路者
  bn_camp_tick(now)    -- #39 躲避制裁
  bn_music_tick(now)   -- #30 音乐（id 未确认前空转）
  bn_freeze_timer(now)
  -- 真实时间轮询（2026-10-23 整改）：每 30 秒带回执刷新真实 day/daytime（陈旧时 10 秒重试）；
  -- 起止、冻结、周期全部以真实世界为准，不再读公告栏模拟 ticks、不再自数天数。
  if now >= (BN2.nextRtAt or 0) then
    bn_rt_refresh(false)
    BN2.nextRtAt = now + (bn_rt_stale() and BN2_RT_RETRY_SEC or BN2_RT_POLL_SEC)
  end
  if now >= BN2.nextBoardAt then
    BN2.nextBoardAt = now + 10
    local dt = BN2_RT.daytime
    local rd = BN2_RT.day
    if bn_is_night_dt(dt) then BN2.nightSeen = true end
    if bn_phase() == BN2_PHASES.CALM then
      -- 自动开场：真实 daytime 已入夜 + 真实 day 命中血月日 + 有人在线；数据不新鲜宁可不开，绝不编造
      -- #40-#44 自动开场锁定傍晚：真实 daytime 入傍晚窗 [12000,13500)（天边变红）且命中血月日才开；错过窗口等下个血月日，不深夜补开
      if bn_rt_fresh() and dt ~= nil and dt >= BN_DUSK_FROM and dt < BN_DUSK_TO and bn_is_blood_day(rd) then
        if #bn_humans_online() > 0 then bn_begin_night(false) end  -- #47 只有机器人在线不开
      end
    else
      local ph_now = bn_phase()
      -- 中夜冻结触发改看真实 daytime（冻结本身仍是暂停昼夜循环，不是拨表）；解冻仍由 bn_freeze_timer 按真实时钟触发
      if dt ~= nil and not d.freeze_done and not d.frozen
        and (ph_now == BN2_PHASES.DESCENT or ph_now == BN2_PHASES.HUNT or ph_now == BN2_PHASES.CLIMAX) then
        if dt >= BN2_FREEZE_AT_TICK and dt < BN2_FREEZE_LATEST_TICK then
          bn_freeze_begin(now)
        elseif dt >= BN2_FREEZE_LATEST_TICK then
          d.freeze_done = true  -- 越过子夜太多：本晚不再冻结，保住尾段（落盘防每轮重判）
          bn_db_save()
        end
      end
      -- 【2026-10-23 晚·根因修复：keep 收尾整条删除】实证链：keepInventory 的所有权在 core 昼夜
      -- 阶段系统，其时钟（r21d_fix 的 current_ticks）只按「上次 SetTime 包 + 真实秒数×20」外推：
      -- 中夜冻结期世界钟停摆、SetTime 不再刷新时它仍向前跑，约 3~4 分钟即越过自家 23000 相位把
      -- keepinventory 误翻 true；且 bn_keep_inventory() 在回执为空时返回 false，一次命令抖动
      -- 就能给本收尾「上膛」。用户 21:36 复测仍自动变白天即此故。keep 从此不作任何收尾信号：
      -- 收尾只认真实时间（BN2_RT 白天界）/高潮延长/30 分钟真实钟硬顶/手动关闭（真实时间神圣）。
      if ph_now == BN2_PHASES.HUNT or ph_now == BN2_PHASES.CLIMAX then
        -- 天亮收尾改看真实 daytime：本晚确实进过夜窗（nightSeen）且真实时间已到 23000
        if (BN2.nightSeen and dt ~= nil and dt >= BN2_RT_NIGHT_TO) or (now - (tonumber(d.started_at) or now) >= BN2_NIGHT_TARGET_SEC) then  -- #26/#34 全程冻结下 daytime 走不到 23000，整晚时长由真实时钟 720 秒到点收尾（与天亮并列）
          if bn_climax_alive() then
            BN2.extendStart = now
            bn_set_phase(BN2_PHASES.EXTEND)
            bn_broadcast("§4[血月] §7天快亮了——但高潮还没分出胜负，再给你们 3 分钟。")
          else
            bn_finish_night("dawn")
          end
        end
      elseif ph_now == BN2_PHASES.EXTEND then
        if not bn_climax_alive() then bn_finish_night("dawn")
        elseif now - (BN2.extendStart or now) >= BN2_EXTEND_MAX then bn_finish_night("extend") end
      end
      -- 安全兜底：真实时间通道异常时也不许整夜挂住，开夜超过 30 分钟强制收尾（正常一晚约 20 分钟）
      if bn_phase() ~= BN2_PHASES.CALM and now - (tonumber(d.started_at) or now) >= BN2_NIGHT_TARGET_SEC + BN2_EXTEND_MAX + 120 then  -- 硬顶随 12 分钟口径收紧（720+延长180+缓冲120）
        bn_log("rt hard cap finish")
        bn_finish_night("cap")
      end
    end
  end
  phase = bn_phase()
  if phase == BN2_PHASES.CALM then return end
  d = bn_db_load()
  local elapsed = now - (tonumber(d.started_at) or now)
  -- 降临演出：钟声第 2、3 响在玩家本人位置（裁定：只敲 3 响、音调 1.0）
  if phase == BN2_PHASES.DESCENT then
    if elapsed >= 3 and BN2.showStage < 1 then
      BN2.showStage = 1
      for _, nm in ipairs(d.participants) do bn_sound_at(nm, "bell.hit", 1, 1.0) end
    end
    if elapsed >= 6 and BN2.showStage < 2 then
      BN2.showStage = 2
      for _, nm in ipairs(d.participants) do bn_sound_at(nm, "bell.hit", 1, 1.0) end
      bn_title("§4§l它 们 来 了", "§c第 1 波将至")
    end
    if elapsed >= BN2_WAVE_AT[1] then
      bn_set_phase(BN2_PHASES.HUNT)
      phase = BN2_PHASES.HUNT
    end
  end
  if phase == BN2_PHASES.HUNT or phase == BN2_PHASES.CLIMAX or phase == BN2_PHASES.EXTEND then
    if phase ~= BN2_PHASES.EXTEND then
      -- v3：波次/高潮按组独立推进（同一时刻表，各组互不干扰）；BN2.waveDone 只作状态展示的最大值
      local maxDone = BN2.waveDone or 0
      for _, g in ipairs(BN2.groups) do
        local nxt = BN2_WAVE_AT[g.waveDone + 1]
        if nxt and elapsed >= nxt then bn_spawn_group_wave(g, g.waveDone + 1) end
        if g.waveDone >= 4 and elapsed >= BN2_CLIMAX_AT and not g.climaxDone then
          bn_spawn_group_climax(g)
          if g.climaxDone and bn_phase() == BN2_PHASES.HUNT then bn_set_phase(BN2_PHASES.CLIMAX) end
        end
        if g.waveDone > maxDone then maxDone = g.waveDone end
      end
      BN2.waveDone = maxDone
    end
  end
  if now >= BN2.nextChainAt then
    BN2.nextChainAt = now + 5
    bn_refresh_chains()
  end
  if now >= BN2.nextSyncAt then
    BN2.nextSyncAt = now + 30
    bn_sync_participants()
  end
end
-- #32 铺路者：每 2 秒一条原生命令——铺路怪脚下为空/水时垫一块土；不抬头不搭柱，随移动自然成路
local function bn_paver_tick(now)
  if now < (BN2.nextPaveAt or 0) then return end
  BN2.nextPaveAt = now + 2
  if bn_phase() == BN2_PHASES.CALM then return end
  bn_cmd("execute as @e[tag=bn2pv] at @s if block ~ ~-1 ~ air run setblock ~ ~-1 ~ dirt")
  bn_cmd("execute as @e[tag=bn2pv] at @s if block ~ ~-1 ~ water run setblock ~ ~-1 ~ dirt")
end
-- #39 制裁挖三填一躲避（保守，宁漏勿误伤）：头顶 1 格实心封顶 + 脚层四周至少 3 面实心，持续满 60 秒才触发；
-- 在其环带召唤 1 只唤魔者（原生召恼鬼）+ 2 只恼鬼；每人冷却 180 秒；管理员/机器人豁免；条件一断即清零
local function bn_camp_tick(now)
  if now < (BN2.nextCampAt or 0) then return end
  BN2.nextCampAt = now + 5
  if bn_phase() == BN2_PHASES.CALM then return end
  BN2.campTrack = BN2.campTrack or {}
  local function solid(x, y, z)
    local b = bn_block_id(x, y, z)
    return b ~= nil and not BN2_PASSABLE[b] and not BN2_LIQUID[b]
  end
  for _, nm in ipairs(bn_online()) do
    if bn_is_bot_name(nm) or bn_privileged(nm) or bn_is_admin(nm) then
      BN2.campTrack[nm] = nil
    else
      local pp = bn_pos(nm)
      local enclosed = false
      if pp then
        local fx, fy, fz = math.floor(pp.x), math.floor(pp.y), math.floor(pp.z)
        if solid(fx, fy + 2, fz) then
          local walls = 0
          if solid(fx + 1, fy, fz) then walls = walls + 1 end
          if solid(fx - 1, fy, fz) then walls = walls + 1 end
          if solid(fx, fy, fz + 1) then walls = walls + 1 end
          if solid(fx, fy, fz - 1) then walls = walls + 1 end
          enclosed = walls >= 3
        end
      end
      if enclosed then
        local tr = BN2.campTrack[nm] or {since = now, lastPunish = 0}
        BN2.campTrack[nm] = tr
        if now - tr.since >= 60 and now - (tr.lastPunish or 0) >= 180 then
          tr.lastPunish = now
          local pt = bn_pick_point(pp, "ground", nil)
          if pt then
            local gi = BN2.groupOf and BN2.groupOf[nm]
            bn_spawn_one(nm, pt, "evoker", {role = "camp", pts = 0, grp = gi})
            bn_spawn_one(nm, {x = pt.x + 2, y = pt.y + 1, z = pt.z}, "vex", {pts = 0, grp = gi})
            bn_spawn_one(nm, {x = pt.x - 2, y = pt.y + 1, z = pt.z}, "vex", {pts = 0, grp = gi})
          end
          bn_tell(nm, "§4[血月] §c封顶躲洞也没用——唤魔者带着恼鬼来找你了。")
          bn_log("camp punish -> " .. tostring(nm))
        end
      else
        BN2.campTrack[nm] = nil
      end
    end
  end
end
-- #30 音乐：BN_MUSIC_SOUND = record.tears（Amos Roddy – Tears），每 180 秒在每位参与者位置循环播放
local function bn_music_tick(now)
  if not BN_MUSIC_SOUND then return end
  if now < (BN2.nextMusicAt or 0) then return end
  BN2.nextMusicAt = now + 180
  if bn_phase() == BN2_PHASES.CALM then return end
  local d = bn_db_load()
  for _, nm in ipairs(d.participants or {}) do bn_sound_at(nm, BN_MUSIC_SOUND, 1, 1) end
end

function museBloodNight2Tick()
  local ok, err = pcall(bn_tick_inner)
  if not ok then bn_log("tick err " .. tostring(err)) end
end
_G.museBloodNightTick = museBloodNight2Tick
-- ================= 聊天命令 =================
local function bn_status_text()
  local d = bn_db_load()
  local phase = bn_phase()
  local lines = {}
  if phase == BN2_PHASES.CALM then
    local left = bn_days_left()
    if left < 0 then lines[#lines + 1] = "§4[血月] §7真实天数查询中，稍后再看。"
    elseif left <= 0 then lines[#lines + 1] = "§4[血月] §7今晚就是血月夜，天黑之后小心。"
    else lines[#lines + 1] = "§4[血月] §7下个血月夜还有 " .. tostring(left) .. " 天（已过 " .. tostring(tonumber(d.nightNo) or 0) .. " 夜）。" end
  else
    lines[#lines + 1] = "§4[血月] §c第 " .. tostring(tonumber(d.nightNo) or 0) .. " 夜进行中（第 " .. tostring(BN2.waveDone) .. " 波）。"
    if BN2.groups and #BN2.groups >= 2 then
      lines[#lines + 1] = "§4[血月] §7今晚分 " .. tostring(#BN2.groups) .. " 组各自为战。"
    end
    if d.frozen then
      local remain = math.max(0, math.ceil(((tonumber(d.freeze_until) or bn_now()) - bn_now()) / 60))
      lines[#lines + 1] = "§4[血月] §7时间冻结中·全程（约剩 " .. tostring(remain) .. " 分钟）。"
    end
    -- （旧「死亡不掉落提前收尾」提示已随 keep 收尾一并删除，2026-10-23 晚）
    local rows = {}
    for nm, tn in pairs(d.tonight) do rows[#rows + 1] = {nm = nm, sc = tonumber(tn.score) or 0} end
    table.sort(rows, function(a, b) return a.sc > b.sc end)
    for i = 1, math.min(3, #rows) do
      if rows[i].sc > 0 then lines[#lines + 1] = "§7今夜第 " .. tostring(i) .. "：§f" .. rows[i].nm .. " §d" .. tostring(rows[i].sc) .. " 分" end
    end
  end
  return table.concat(lines, "\n")
end
local function bn_handle_chat(name, msg)
  if type(msg) ~= "string" then return end
  local cmd = bn_trim(msg)
  if cmd == ".血晶" then
    local d = bn_db_load()
    local tn = d.tonight[name] or {score = 0}
    bn_tell(name, "§4[血月] §7血晶已经改成积分啦：打死带血月标记的怪得分，精英掉绿宝石，骑士和重锤另有奖励。\n§7你累计 §d" .. tostring(tonumber(d.score[name]) or 0) .. " §7分，今夜 §d" .. tostring(tonumber(tn.score) or 0) .. " §7分。用 §f.血月 积分 §7看排名。")
    return
  end
  if cmd == ".血月" or cmd == ".血月 状态" then
    bn_tell(name, bn_status_text())
    return
  end
  if cmd == ".血月 积分" then
    local d = bn_db_load()
    local rows = {}
    for nm, sc in pairs(d.score) do rows[#rows + 1] = {nm = nm, sc = tonumber(sc) or 0} end
    table.sort(rows, function(a, b) return a.sc > b.sc end)
    local rank = "未上榜"
    for i, r in ipairs(rows) do if r.nm == name then rank = "第 " .. tostring(i) .. " 名" end end
    local tn = d.tonight[name] or {score = 0, kills = 0}
    bn_tell(name, "§4[血月积分] §7累计 §d" .. tostring(tonumber(d.score[name]) or 0) .. " §7分（" .. rank .. "），今夜 §d" .. tostring(tonumber(tn.score) or 0) .. " §7分 / " .. tostring(tonumber(tn.kills) or 0) .. " 杀。")
    return
  end
  if cmd == ".血月 开启" then
    if not bn_privileged(name) then bn_tell(name, "§4[血月] §7只有管理员能手动开启血月。"); return end
    -- 2026-10-23 整改：手动开启不再拨世界时间（已删 time set 14000），按当前真实时刻直接开场
    bn_rt_refresh(true)
    local ok, why = bn_begin_night(true)
    if ok and BN2_RT.daytime ~= nil and not bn_is_night_dt(BN2_RT.daytime) then
      bn_tell(name, "§4[血月] §c" .. why .. "，祝好运。§7（现在是白天，已按当前时刻开启）")
    else
      bn_tell(name, ok and ("§4[血月] §c" .. why .. "，祝好运。") or ("§4[血月] §7开不了：" .. tostring(why)))
    end
    return
  end
  if cmd == ".血月 关闭" then
    if not bn_privileged(name) then bn_tell(name, "§4[血月] §7只有管理员能手动结束血月。"); return end
    if bn_phase() == BN2_PHASES.CALM then bn_tell(name, "§4[血月] §7现在没有血月。"); return end
    bn_finish_night("manual")
    bn_tell(name, "§4[血月] §7已手动收尾，天亮了。")
    return
  end
end
-- ================= 导出 =================
function museBloodNightStatus()
  local d = bn_db_load()
  local top, topScore = nil, 0
  for nm, tn in pairs(d.tonight) do
    if (tonumber(tn.score) or 0) > topScore then top, topScore = nm, tonumber(tn.score) end
  end
  local climaxKind = BN2.climaxKind
  if not climaxKind then for _, g in ipairs(BN2.groups or {}) do if g.climaxKind then climaxKind = g.climaxKind; break end end end
  local gsizes = {}
  for _, g in ipairs(BN2.groups or {}) do gsizes[#gsizes + 1] = tostring(g.size) .. "/T" .. tostring(g.tier) end
  return {phase = bn_phase(), night = tonumber(d.nightNo) or 0, dayCount = BN2_RT.day or -1, realDay = BN2_RT.day, realDaytime = BN2_RT.daytime, rtStale = bn_rt_stale(),
    daysLeft = bn_days_left(), climax = climaxKind, topTonight = top, topScore = topScore,
    registered = bn_reg_count(), cap = bn_cap(), groups = #(BN2.groups or {}), groupSizes = table.concat(gsizes, ",")}
end
function museBloodNightDiag()
  local parts = {}
  parts[#parts + 1] = "ver=" .. tostring(_G.museBloodNightVersion)
  parts[#parts + 1] = "phase=" .. bn_phase()
  parts[#parts + 1] = "reg=" .. tostring(bn_reg_count()) .. "/" .. tostring(bn_cap())
  parts[#parts + 1] = "pending=" .. tostring(#BN2.pending)
  do
    local gs = {}
    for _, g in ipairs(BN2.groups or {}) do gs[#gs + 1] = tostring(g.size) .. "人/T" .. tostring(g.tier) .. "/w" .. tostring(g.waveDone) end
    parts[#parts + 1] = "groups{" .. table.concat(gs, ",") .. "}"
  end
  local fz = bn_db_load()
  parts[#parts + 1] = "frozen=" .. tostring(fz.frozen == true)
  if fz.frozen then parts[#parts + 1] = "freezeLeft=" .. tostring(math.max(0, (tonumber(fz.freeze_until) or 0) - bn_now())) end
  local ev = {}
  for k, v in pairs(BN2.events) do ev[#ev + 1] = k .. "=" .. tostring(v) end
  table.sort(ev)
  parts[#parts + 1] = "events{" .. table.concat(ev, ",") .. "}"
  local cc = {}
  for k, v in pairs(BN2.counters) do cc[#cc + 1] = k .. "=" .. tostring(v) end
  table.sort(cc)
  parts[#parts + 1] = "counters{" .. table.concat(cc, ",") .. "}"
  local cs = {}
  for k, v in pairs(BN2.causeSeen) do cs[#cs + 1] = k .. "=" .. tostring(v) end
  table.sort(cs)
  parts[#parts + 1] = "causes{" .. table.concat(cs, ",") .. "}"
  local st = {}
  for k, v in pairs(BN2.selftest) do st[#st + 1] = k .. "=" .. tostring(v) end
  table.sort(st)
  parts[#parts + 1] = "selftest{" .. table.concat(st, ",") .. "}"
  local ch = {}
  for k, v in pairs(BN2.chains) do ch[#ch + 1] = k .. "=" .. tostring(v) end
  table.sort(ch)
  parts[#parts + 1] = "chains{" .. table.concat(ch, ",") .. "}"
  parts[#parts + 1] = "lastResp=" .. bn_sub(BN2.lastResp, 1, 120)
  parts[#parts + 1] = "realDay=" .. tostring(BN2_RT.day)
  parts[#parts + 1] = "realDaytime=" .. tostring(BN2_RT.daytime)
  parts[#parts + 1] = "rtStale=" .. tostring(bn_rt_stale())
  parts[#parts + 1] = "rtFail=" .. tostring(BN2_RT.fail)
  parts[#parts + 1] = "anchor=" .. tostring(bn_db_load().rtAnchor)
  local function rt_raw_clean(s) return (tostring(s or ""):gsub("[|\r\n\"]", " ")):sub(1, 120) end
  parts[#parts + 1] = "rtRawDay=[" .. rt_raw_clean(BN2_RT.rawDay) .. "]"
  parts[#parts + 1] = "rtRawDaytime=[" .. rt_raw_clean(BN2_RT.rawDaytime) .. "]"
  return table.concat(parts, " | ")
end
_G.muse_bloodnight_status = function(name) return bn_status_text() end
-- 宿主导出走 plugin_api 表（与 v1 同契约，plugin.call 才能找到）
plugin_api = plugin_api or {}
plugin_api.museBloodNightVersion = function() return "bloodnight-v3" end
plugin_api.museBloodNightStatus = function(name)
  local s = museBloodNightStatus()
  s.ok = true
  s.version = "bloodnight-v3"
  s.text = bn_status_text()
  return s
end
plugin_api.museBloodNightDiag = function()
  return {ok = true, version = "bloodnight-v3", text = museBloodNightDiag()}
end
_G.__muse_bloodnight_internals = {
  tick = museBloodNight2Tick, begin = bn_begin_night, finish = bn_finish_night,
  status_text = bn_status_text, db_load = bn_db_load, state = BN2,
  spawn_one = bn_spawn_one, cluster = bn_cluster, tier_of = bn_tier_of,
  compose = bn_compose_wave, refresh_chains = bn_refresh_chains,
}
-- ================= 注册与自检 =================
local function bn_evt(name, fnName, fn)
  if not (events and type(events[name]) == "function") then BN2.events[name] = "missing"; return end
  local ok = pcall(events[name], fnName)
  if ok then BN2.events[name] = "string"; return end
  local ok2 = pcall(events[name], fn)
  BN2.events[name] = ok2 and "function" or "fail"
end
_G.museBloodNight2OnChat = function(name, msgOrE, maybeMsg)
  local msg = msgOrE
  if type(msgOrE) == "table" then msg = maybeMsg or msgOrE.message or msgOrE.msg or msgOrE.text end
  local nm = name
  if type(name) == "table" then nm = name.name or name.player or name.sender end
  if type(nm) == "string" and type(msg) == "string" then
    local ok, err = pcall(bn_handle_chat, nm, msg)
    if not ok then bn_log("chat err " .. tostring(err)) end
  end
end
local function bn_register_all()
  bn_evt("onEntitySpawn", "museBloodNight2Spawn", museBloodNight2Spawn)
  bn_evt("onEntityHurt", "museBloodNight2Hurt", museBloodNight2Hurt)
  bn_evt("onHealthChange", "museBloodNight2Health", museBloodNight2Health)
  bn_evt("onEntityKill", "museBloodNight2Kill", museBloodNight2Kill)
  bn_evt("onPlayerKill", "museBloodNight2PlayerKill", museBloodNight2PlayerKill)
  bn_evt("onEntityRemove", "museBloodNight2Remove", museBloodNight2Remove)
  bn_evt("onPlayerDie", "museBloodNight2PlayerDeath", museBloodNight2PlayerDeath)
  bn_evt("onPlayerDeath", "museBloodNight2PlayerDeath", museBloodNight2PlayerDeath)
  bn_evt("onPlayerDead", "museBloodNight2PlayerDeath", museBloodNight2PlayerDeath)
  if events and type(events.onChat) == "function" then
    local ok = pcall(events.onChat, "museBloodNight2OnChat")
    BN2.events.onChat = ok and "string" or "fail-string"
    if not ok then
      local ok2 = pcall(events.onChat, _G.museBloodNight2OnChat)
      BN2.events.onChat = ok2 and "function" or "fail"
    end
  else
    BN2.events.onChat = "missing"
  end
end
local function bn_obj_exists(name)
  local t = bn_resp_text(bn_cmd_resp("scoreboard objectives list"))
  return t:find(name, 1, true) ~= nil
end
local function bn_resp_ok(t)
  -- 宿主回执形状多变：中英成功文案 / 结构化 success_count / successCount 任一命中即算成功
  return t:find("成功") or t:find("找到") or t:find("Found") or t:find("success_count=1", 1, true)
    or t:find('"successCount":1', 1, true) or t:find('"success_count": 1', 1, true)
end
local function bn_selftests()
  -- 中文计分板目标：建目标后用假玩家 巡检测试员 写入探活（不碰真人）
  bn_cmd_resp("scoreboard objectives add bn2selftest dummy 血月自检")
  local r1 = bn_resp_text(bn_cmd_resp("scoreboard players set 巡检测试员 bn2selftest 7"))
  BN2.selftest.scoreboardCN = bn_resp_ok(r1) and "ok" or ("fail:" .. bn_sub(r1, 1, 40))
  if BN2.selftest.scoreboardCN == "ok" then
    local r2 = bn_resp_text(bn_cmd_resp("scoreboard players list 巡检测试员"))
    BN2.selftest.scoreboardRead = (r2:find("bn2selftest") and r2:find("7")) and "ok" or "fail"
    bn_cmd("scoreboard objectives remove bn2selftest")
  end
  -- 中文标签：用机器人账号 布汲岛 自测（打上立刻摘掉）
  if bn_pos("布汲岛") then
    bn_cmd_resp('tag "布汲岛" add 血月')
    local r3 = bn_resp_text(bn_cmd_resp('testfor @a[name="布汲岛",tag=血月]'))
    BN2.selftest.tagCN = bn_resp_ok(r3) and "ok" or ("fail:" .. bn_sub(r3, 1, 40))
    bn_cmd_resp('tag "布汲岛" remove 血月')
  else
    BN2.selftest.tagCN = "skip-bot-offline"
  end
end
local function bn_boot()
  bn_chains_boot()
  local d = bn_db_load()
  if bn_phase() ~= BN2_PHASES.CALM then
    bn_cleanup_world()
    d.died = {}
    bn_set_phase(BN2_PHASES.CALM)
  else
    if d.frozen then bn_unfreeze("boot_residual") end
    bn_cmd("fog @a remove bn_red")
    bn_cmd("fog @a remove bn_deep")
    bn_cmd("fog @a remove bn_fade")
    bn_cmd("fog @a pop minecraft:fog_crimson_forest")
  end
  -- 计分板：建新（bn_state/bn_score/bn_kills/bn2cd/bn2pill）删旧（bn_crystal/bn_sanity 已下线）
  bn_setup_scoreboards()
  bn_rt_refresh(true)  -- 2026-10-23 整改：装机即查真实 day/daytime 并完成周期锚点迁移（只读查询）
  bn_log("rt boot realDay=" .. tostring(BN2_RT.day) .. " realDaytime=" .. tostring(BN2_RT.daytime) .. " anchor=" .. tostring(bn_db_load().rtAnchor))
  bn_register_all()
  local ok, err = pcall(bn_selftests)
  if not ok then bn_log("selftest err " .. tostring(err)); BN2.selftest.error = bn_sub(tostring(err), 1, 60) end
  bn_log("bloodnight v3 boot ok; lastResp=" .. bn_sub(BN2.lastResp, 1, 80))
end
local bn_boot_ok, bn_boot_err = pcall(bn_boot)
if not bn_boot_ok then bn_log("boot err " .. tostring(bn_boot_err)) end
if util and type(util.schedule) == "function" then pcall(util.schedule, 1, "museBloodNight2Tick") end
if type(expire_system_state) == "function" and not _G.__muse_bn2_expire_wrapped then
  _G.__muse_bn2_expire_wrapped = true
  local bn_orig_expire = expire_system_state
  expire_system_state = function(...)
    local r = {bn_orig_expire(...)}
    pcall(museBloodNight2Tick)
    return table.unpack(r)
  end
end
end
-- === MUSE BLOODNIGHT v3 END ===

-- YY ISSUEFIX 2026-10-24: additive soul / unban / combat-log compatibility fixes.
-- Keep this import after existing hooks, including unrelated Muse features.
apply_module("modules.yy_issuefix_20261024")
-- R103 Lua/Native A-B comparison, disabled until command explicitly enables.
apply_module("modules.yy_snow_benchmark")
-- R104: paced AI final-answer delivery; token and model unchanged.
apply_module("modules.yy_ai_delivery_r104")
-- R106: fix system shop barrel inventory decoding; fail closed if NBT unreadable.
apply_module("modules.yy_market_recycle_r106")
-- R107 system-shop one-click sell; keep original prices and settlement.
apply_module("modules.yy_market_quicksell_r107")
-- 雪球菜单 Lua 抽离版（2026-10-25 用户令：命令链功能抽离进插件；旧 TXT 链已停用退役）
apply_module("modules.muse_snowball_menu")
-- R108: no-bot-TP market reads; memoized shop catalog and deferred page persistence.
apply_module("modules.yy_market_fastpage_r108")
-- R109 system-shop safe validation and sandbox test API
apply_module("modules.yy_market_integrity_r109")
-- R110: dimension identity regression probes, no modification to existing records.
apply_module("modules.yy_dimension_integrity_r110")
-- R113: 72h ephemeral death/chat/AI history cleanup; exclude player economy and waypoints.
apply_module("modules.yy_retention_r113")
