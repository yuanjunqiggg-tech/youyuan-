-- muse_snowball_menu.lua —— 雪球菜单 Lua 抽离版（2026-10-25 用户令：命令链内容抽离为插件功能）
-- 原系统：TXT 链「雪球菜单_综合功能.txt」(R96) +「_雪球菜单_回地表模块.txt」(R102) + 互传计时 1Hz。
-- 旧链是无条件常驻 20Hz 空转（实测约 1240 条命令/秒），已于 2026-10-25 从宿主侧停用并持久禁用
-- （chains.configure enabled=false + chains.stop），本模块为其功能等价移植，旧 TXT 仅作退役登记。
-- 移植口径：
--  · 抛掷入口走事件钩子 onEntitySpawn（雪球实体出现即触发原版计分命令）+ 每秒门控探针兜底；
--  · 菜单状态机仍用原计分板/标签（雪球菜单/互传/在线时间/yy_surface_*），由 1 秒门控批次驱动
--    与原链逐字相同的命令；无人开菜单/无互传/无回地表任务时，每秒只有约 3 条探针命令；
--  · 回地表 111 步下探改为「点选才启动的分步任务」：每秒一批 48 组（检测+下探）命令 + 收尾判定，
--    与原子模块的检测条件（脚下非空气/熔岩/水、身上两格空气）、维度顶高（129/256/318）、
--    缓降、超时（age 25）清理逐字一致；
--  · 在线计时用 scheduleSeconds 每 60 秒 +1（原链 0.0167Hz 段）；
--  · 世界边界巡逻（原链第 4 段）随菜单 tick 以 1Hz 执行原版两条命令；
--  · 互传倒计时每秒 -1（原独立 1Hz 链，R96 期实际未运行，本移植一并恢复其设计语义）；
--  · @r→@a：原链靠 20Hz 高频随机轮询覆盖所有玩家；自效命令在 1Hz 下改为对全部符合条件玩家
--    同时生效，语义不变；全局广播/单发起人类命令保留原 @r 单执行者写法。
--  · 未移植：常驻补给（_雪球菜单_常驻补给1Hz.txt，旧配置下本就停用）与其它历史子模块，保持停用。
local M={}
local installed=false
local native_owner=false
local S={intake_hits=0,menu_runs=0,tpa_runs=0,surf_runs=0,probes=0,probe_fail_open=0,last=""}
local function cmd(s)
  if type(game)=="table" and type(game.sendCommand)=="function" then pcall(game.sendCommand,s) end
end
local function run_batch(list)
  if type(list)~="table" or #list==0 then return end
  if type(game)=="table" and type(game.executeBatch)=="function" then
    local ok=pcall(game.executeBatch,list,{mode="stream",window=64,channel="ai"})
    if not ok then S.batch_errors=(S.batch_errors or 0)+1 end
    return -- A failed batch may already have executed a prefix; never replay it.
  end
  for _,s in ipairs(list) do cmd(s) end
end
-- Unknown probe results cannot authorize menu writes.
local function any_match(testcmd)
  S.probes=S.probes+1
  if type(game)=="table" and type(game.executeBatch)=="function" then
    local ok,r=pcall(game.executeBatch,{testcmd},{mode="stream",window=1,channel="ai"})
    if ok and type(r)=="table" and r.confirmed==true then
      if tonumber(r.success_count)~=nil then return tonumber(r.success_count)>0 end
      if type(r.results)=="table" and type(r.results[1])=="table" then
        local x=r.results[1]
        if x.confirmed==true and x.success~=nil then return x.success==true end
      end
    end
  end
  S.probe_failures=(S.probe_failures or 0)+1
  return nil
end

local INTAKE={
 [==[/execute at @e[type=snowball] as @p[r=3] run tag @s remove yy_menu_confirm_sound]==],
 [==[/execute at @e[type=snowball] as @p[r=3] run tag @s remove yy_menu_close_sound]==],
 [==[/execute at @e[type=snowball] as @p[r=3] run tag @s remove yy_surface_confirm_sound]==],
 [==[/execute at @e[type=snowball] as @p[r=3] run scoreboard players add @s 雪球菜单 1]==],
 [==[/execute at @e[type=snowball] run playsound block.end_portal_frame.fill @p[r=3]]==],
 [==[/kill @e[type=snowball]]==],
}
local MENU_A={
 [==[/tag @a[tag=yy_surface_latched,rxm=80] add yy_surface_cancel]==],
 [==[/tag @a[tag=return_surface,rxm=80] add yy_surface_cancel]==],
 [==[/tag @a[scores={雪球菜单=2},rx=-80,tag=!return_surface,tag=!yy_surface_latched,tag=!yy_surface_cancel] add yy_surface_latched]==],
 [==[/execute as @a[tag=yy_surface_latched,tag=!yy_surface_confirm_sound,tag=!yy_surface_cancel] run playsound random.levelup @s]==],
 [==[/tag @a[tag=yy_surface_latched,tag=!yy_surface_cancel] add yy_surface_confirm_sound]==],
 [==[/execute as @a[tag=yy_surface_cancel,tag=!yy_surface_cancel_sound] run playsound random.chestclosed @s]==],
 [==[/tag @a[tag=yy_surface_cancel] add yy_surface_cancel_sound]==],
 [==[/execute as @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp] at @s as @a[tag=yy_surface_cancel] if score @s yy_surface_owner = @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,r=0.2,c=1] yy_surface_owner run tag @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,r=0.2,c=1] add yy_surface_cleanup]==],
 [==[/kill @e[type=armor_stand,name="YY_SURFACE_R83",tag=yy_surface_cleanup]]==],
 [==[/scoreboard players reset @a[tag=yy_surface_cancel] 雪球菜单]==],
 [==[/tag @a[tag=yy_surface_cancel] remove yy_surface_latched]==],
 [==[/tag @a[tag=yy_surface_cancel] remove return_surface]==],
 [==[/tag @a[tag=yy_surface_cancel] remove yy_surface_top]==],
 [==[/tag @a[tag=yy_surface_cancel] remove yy_surface_spawn_issued]==],
 [==[/tag @a[tag=yy_surface_cancel] remove yy_surface_confirm_sound]==],
 [==[/tag @a[tag=yy_surface_cancel] remove yy_surface_cancel_sound]==],
 [==[/tag @a[tag=yy_surface_cancel] remove yy_surface_cancel]==],
}
local MENU_B={
 [==[/execute as @a[scores={雪球菜单=1..6}] at @s run titleraw @s actionbar {"rawtext":[{"text":"§9╭──────── §b✧ §f疣螈 · 雪境终端 §b✧ §9────────╮\n"},{"text":"§9│ §7导航进度  "},{"text":""},{"text":""},{"text":""},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=1}]"},{"text":"§b━§7━§7━§7━§7━§7━"}]}},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=2}]"},{"text":"§7━§b━§7━§7━§7━§7━"}]}},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=3}]"},{"text":"§7━§7━§b━§7━§7━§7━"}]}},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=4}]"},{"text":"§7━§7━§7━§b━§7━§7━"}]}},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=5}]"},{"text":"§7━§7━§7━§7━§b━§7━"}]}},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=6}]"},{"text":"§7━§7━§7━§7━§7━§b━"}]}},{"text":"\n§9├─ §b快捷操作 §9────────────────┤\n§9│ "},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=1}]"},{"text":"§f"},{"text":"§7"}]}},{"text":"▸ 主城传送  "},{"text":"§9│ "},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=2}]"},{"text":"§f"},{"text":"§7"}]}},{"text":"▸ 返回地面\n§9│ "},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=3}]"},{"text":"§f"},{"text":"§7"}]}},{"text":"▸ 返回重生  "},{"text":"§9│ "},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=4}]"},{"text":"§f"},{"text":"§7"}]}},{"text":"▸ 设置重生\n§9│ "},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=5}]"},{"text":"§f"},{"text":"§7"}]}},{"text":"▸ 发起互传  "},{"text":"§9│ "},{"translate":"%%2","with":{"rawtext":[{"selector":"@s[scores={雪球菜单=6}]"},{"text":"§f"},{"text":"§7"}]}},{"text":"▸ 接受互传\n"},{"text":"§9├─ §b游玩记录 §9────────────────┤\n§9│ §7在线 §f"},{"score":{"name":"@s","objective":"在线时间"}},{"text":"§7 分钟     §7复活 §f"},{"score":{"name":"@s","objective":"复活次数"}},{"text":"§7 次"},{"text":"\n§9╰──── §a抬头确认 §8· §c低头取消 §9────╯\n"}]}]==],
 [==[/execute as @a[scores={雪球菜单=1..6},rx=-89,tag=!yy_menu_confirm_sound,tag=!yy_surface_latched,tag=!yy_surface_confirm_sound] run playsound random.levelup @s]==],
 [==[/tag @a[scores={雪球菜单=1..6},rx=-89,tag=!yy_surface_latched] add yy_menu_confirm_sound]==],
 [==[/title @a[scores={雪球菜单=1..},rxm=89] actionbar §o§7║║║§c已关闭§7║║║]==],
 [==[/execute as @a[scores={雪球菜单=1..6},rxm=89,tag=!yy_menu_close_sound,tag=!yy_surface_cancel] run playsound random.chestclosed @s]==],
 [==[/tag @a[scores={雪球菜单=1..6},rxm=89] add yy_menu_close_sound]==],
 [==[/scoreboard players reset @a[scores={雪球菜单=1..6},rxm=80,tag=!yy_surface_latched,tag=!return_surface] 雪球菜单]==],
 [==[/scoreboard players set @a[scores={雪球菜单=7}] 雪球菜单 1]==],
 [==[/execute as @a[scores={雪球菜单=1},rx=-89] in overworld run tp @s 10181.54 128.00 10206.50]==],
 [==[/execute in the_end as @a[scores={雪球菜单=3},rx=-89] run tp @s 5 200.00 5]==],
 [==[/execute as @a[scores={雪球菜单=4},rx=-89] at @s run spawnpoint @s]==],
 [==[/title @a[scores={雪球菜单=1..,雪球菜单=!5..6},rx=-89] actionbar §l | | | §a执行成功§f | | |]==],
 [==[/scoreboard players reset @a[scores={雪球菜单=1..,雪球菜单=!5..6,雪球菜单=!2},rx=-89] 雪球菜单]==],
}
local MENU_D={
 [==[/execute if entity @r[scores={互传=1}] run title @a[scores={雪球菜单=5},rx=-89] actionbar §c互传通道占用中，请稍候§f]==],
 [==[/execute if entity @r[scores={互传=1}] run playsound random.anvil_land @r[scores={雪球菜单=5},rx=-89]]==],
 [==[/execute if entity @r[scores={互传=1}] run scoreboard players reset @r[scores={雪球菜单=5},rx=-89] 雪球菜单]==],
 [==[/scoreboard players set @r[scores={雪球菜单=5},rx=-89] 互传 1]==],
 [==[/execute as @r[scores={雪球菜单=5},rx=-89] at @s run tellraw @a {"rawtext":[{"text":"[§d播报§f] §f玩家§b➢"},{"selector":"@s"},{"text":"§a发起了玩家互传\n§f在30秒内打开雪球雪球菜单同意传送即可接受传送。"}]}]==],
 [==[/execute as @r[scores={雪球菜单=5},rx=-89] run playsound random.orb @a]==],
 [==[/execute if entity @r[scores={雪球菜单=5},rx=-89] run scoreboard players set 时间 互传 30]==],
 [==[/scoreboard players set @r[scores={雪球菜单=5},rx=-89] 雪球菜单 0]==],
}
local MENU_E={
 [==[/execute if entity @a[scores={互传=1}] as @a[scores={雪球菜单=6},rx=-88] run tellraw @a {"rawtext":[{"text":"§l§f〉〉§5〉 §r §r§b"},{"selector":"@a[scores={互传=1}]"},{"text":"§r 已传送至 §b"},{"selector":"@s"}]}]==],
 [==[/execute at @r[scores={雪球菜单=6},rx=-89] run tp @r[scores={互传=1}] ~ ~ ~]==],
 [==[/execute if entity @r[scores={互传=1}] at @a[scores={雪球菜单=6},rx=-89] run title @a[scores={雪球菜单=!1..5},c=2] actionbar §l | | | §a传送成功§f | | |]==],
 [==[/execute at @r[scores={雪球菜单=6},rx=-89] run scoreboard players reset * 互传]==],
 [==[/execute at @r[scores={雪球菜单=6},rx=-89] run scoreboard players reset @a[scores={雪球菜单=0}] 雪球菜单]==],
 [==[/scoreboard players reset @r[scores={雪球菜单=6},rx=-89] 雪球菜单]==],
 [==[/execute unless entity @r[scores={互传=1}] run title @a[scores={雪球菜单=6},rx=-89] actionbar §c无人开启互传通道]==],
}
local TPA_TICK={
 [==[/execute if score 时间 互传 matches 1.. run scoreboard players remove 时间 互传 1]==],
 [==[/titleraw @a[scores={互传=1}] actionbar {"rawtext":[{"text":"您已发起玩家互传  §b剩余时长§f "},{"score":{"name":"时间","objective":"互传"}},{"text":" §as\n§f=========§a低头取消互传§f========="}]}]==],
 [==[/execute as @r[scores={互传=0..},rxm=89] run tellraw @a {"rawtext":[{"selector":"@s"},{"text":"§l§b 已取消互传"}]}]==],
 [==[/execute as @r[scores={互传=1},rxm=89] run scoreboard players reset @s 雪球菜单]==],
 [==[/execute as @r[scores={互传=1},rxm=89] run scoreboard players reset @s 互传]==],
 [==[/execute unless entity @r[scores={互传=1}] run scoreboard players reset 时间 互传]==],
 [==[/execute if score 时间 互传 matches ..0 run tellraw @a {"rawtext":[{"text":"§f[§d播报§f] 时间已到，无人接受互传同意，已重置互传通道"}]}]==],
 [==[/execute if score 时间 互传 matches ..0 run scoreboard players reset @r[scores={雪球菜单=..0}] 雪球菜单]==],
 [==[/execute if score 时间 互传 matches ..0 run scoreboard players reset * 互传]==],
}
local WORLD_GUARD={
 [==[/execute as @r at @s unless entity @s[x=200000,z=200000,dx=-400000,dz=-400000] in overworld run tp @s 10062.41 157.00 10062.56]==],
 [==[/execute as @r at @s unless entity @s[x=200000,z=200000,dx=-400000,dz=-400000] run tellraw @s {"rawtext":[{"text":"§e前方的区域暂不开放\n请以后再来探索吧~~"}]}]==],
}
local SURF_SPAWN={
 [==[/tag @a[tag=yy_surface_latched] remove yy_surface_dim_nether]==],
 [==[/tag @a[tag=yy_surface_latched] remove yy_surface_dim_end]==],
 [==[/tag @a[tag=yy_surface_latched] remove yy_surface_dim_overworld]==],
 [==[/execute in nether positioned 0 -100 0 as @a[rm=1,tag=yy_surface_latched,tag=!yy_surface_cancel] run tag @s add yy_surface_dim_nether]==],
 [==[/execute in the_end positioned 0 -100 0 as @a[rm=1,tag=yy_surface_latched,tag=!yy_surface_cancel] run tag @s add yy_surface_dim_end]==],
 [==[/execute in overworld positioned 0 -100 0 as @a[rm=1,tag=yy_surface_latched,tag=!yy_surface_cancel] run tag @s add yy_surface_dim_overworld]==],
 [==[/scoreboard players set @a[tag=yy_surface_latched,tag=!yy_surface_spawn_issued] yy_surface_age 0]==],
 [==[/scoreboard players random @a[tag=yy_surface_latched,tag=!return_surface,tag=!yy_surface_cancel] yy_surface_owner 1 2000000000]==],
 [==[/execute as @a[tag=yy_surface_latched,tag=!return_surface,tag=!yy_surface_cancel,tag=!yy_surface_spawn_issued] at @s run summon armor_stand "YY_SURFACE_R83" ~ ~ ~]==],
 [==[/tag @a[tag=yy_surface_latched,tag=!return_surface,tag=!yy_surface_cancel] add yy_surface_spawn_issued]==],
 [==[/execute as @a[tag=yy_surface_latched,tag=!return_surface,tag=!yy_surface_cancel] at @s run scoreboard players operation @e[type=armor_stand,name="YY_SURFACE_R83",tag=!surface_tp,r=1,c=1] yy_surface_owner = @s yy_surface_owner]==],
 [==[/execute as @a[tag=yy_surface_latched,tag=!return_surface,tag=!yy_surface_cancel] at @s run tag @e[type=armor_stand,name="YY_SURFACE_R83",tag=!surface_tp,r=1,c=1] add surface_tp]==],
 [==[/execute as @a[tag=yy_surface_latched,tag=!return_surface,tag=!yy_surface_cancel] at @s if entity @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,r=1,c=1] run tag @s add return_surface]==],
 [==[/execute as @a[tag=return_surface,tag=yy_surface_dim_nether,tag=!yy_surface_top,tag=!yy_surface_cancel] at @s run tp @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,r=2,c=1] ~ 129 ~]==],
 [==[/execute as @a[tag=return_surface,tag=yy_surface_dim_end,tag=!yy_surface_top,tag=!yy_surface_cancel] at @s run tp @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,r=2,c=1] ~ 256 ~]==],
 [==[/execute as @a[tag=return_surface,tag=yy_surface_dim_overworld,tag=!yy_surface_top,tag=!yy_surface_cancel] at @s run tp @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,r=2,c=1] ~ 318 ~]==],
 [==[/execute as @a[tag=return_surface,tag=yy_surface_dim_nether,tag=!yy_surface_top,tag=!yy_surface_cancel] at @s positioned ~ 129 ~ if entity @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,r=2,c=1] run tag @s add yy_surface_top]==],
 [==[/execute as @a[tag=return_surface,tag=yy_surface_dim_end,tag=!yy_surface_top,tag=!yy_surface_cancel] at @s positioned ~ 256 ~ if entity @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,r=2,c=1] run tag @s add yy_surface_top]==],
 [==[/execute as @a[tag=return_surface,tag=yy_surface_dim_overworld,tag=!yy_surface_top,tag=!yy_surface_cancel] at @s positioned ~ 318 ~ if entity @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,r=2,c=1] run tag @s add yy_surface_top]==],
 [==[/execute as @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,tag=!yy_surface_active] at @s positioned ~ 129 ~ if entity @s[r=0.2] run tag @s add yy_surface_active]==],
 [==[/execute as @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,tag=!yy_surface_active] at @s positioned ~ 256 ~ if entity @s[r=0.2] run tag @s add yy_surface_active]==],
 [==[/execute as @e[type=armor_stand,name="YY_SURFACE_R83",tag=surface_tp,tag=!yy_surface_active] at @s positioned ~ 318 ~ if entity @s[r=0.2] run tag @s add yy_surface_active]==],
 [==[/scoreboard players reset @a[tag=return_surface,scores={雪球菜单=2}] 雪球菜单]==],
 [==[/scoreboard players add @a[tag=yy_surface_latched] yy_surface_age 1]==],
}
local SURF_DONE={
 [==[/execute as @e[type=armor_stand,tag=yy_surface_active] at @s unless block ~ ~-1 ~ air as @a[tag=return_surface,tag=!yy_surface_cancel] if score @s yy_surface_owner = @e[type=armor_stand,tag=yy_surface_active,r=0.1,c=1] yy_surface_owner run effect @s slow_falling 12 0 true]==],
 [==[/execute as @e[type=armor_stand,tag=yy_surface_active,tag=yy_surface_ready] at @s as @a[tag=return_surface,tag=!yy_surface_cancel] if score @s yy_surface_owner = @e[type=armor_stand,tag=yy_surface_active,r=0.1,c=1] yy_surface_owner run tp @s ~ ~ ~]==],
 [==[/execute as @e[type=armor_stand,tag=yy_surface_active] at @s unless block ~ ~-1 ~ air as @a[tag=return_surface] if score @s yy_surface_owner = @e[type=armor_stand,tag=yy_surface_active,r=0.1,c=1] yy_surface_owner if entity @s[r=2] run tag @s add yy_surface_done]==],
 [==[/execute as @a[tag=yy_surface_done] run titleraw @s actionbar {"rawtext":[{"text":"§b✦ 已安全返回地面"}]}]==],
 [==[/execute as @e[type=armor_stand,tag=yy_surface_active] at @s as @a[tag=yy_surface_done] if score @s yy_surface_owner = @e[type=armor_stand,tag=yy_surface_active,r=0.1,c=1] yy_surface_owner run tag @e[type=armor_stand,tag=yy_surface_active,r=0.1,c=1] add yy_surface_cleanup]==],
 [==[/kill @e[type=armor_stand,tag=yy_surface_cleanup]]==],
 [==[/tag @a[tag=yy_surface_done] remove yy_surface_latched]==],
 [==[/tag @a[tag=yy_surface_done] remove return_surface]==],
 [==[/tag @a[tag=yy_surface_done] remove yy_surface_top]==],
 [==[/tag @a[tag=yy_surface_done] remove yy_surface_spawn_issued]==],
 [==[/tag @a[tag=yy_surface_done] remove yy_surface_confirm_sound]==],
 [==[/scoreboard players reset @a[tag=yy_surface_done] yy_surface_age]==],
 [==[/tag @a[tag=yy_surface_done] remove yy_surface_dim_nether]==],
 [==[/tag @a[tag=yy_surface_done] remove yy_surface_dim_end]==],
 [==[/tag @a[tag=yy_surface_done] remove yy_surface_dim_overworld]==],
 [==[/tag @a[tag=yy_surface_done] remove yy_surface_done]==],
 [==[/tag @a[tag=yy_surface_latched,scores={yy_surface_age=25..}] add yy_surface_failed]==],
 [==[/execute as @a[tag=yy_surface_failed] run tellraw @s {"rawtext":[{"text":"§c寻找地面超时，已停止本次查找。"}]}]==],
 [==[/execute as @e[type=armor_stand,tag=yy_surface_active] at @s as @a[tag=yy_surface_failed] if score @s yy_surface_owner = @e[type=armor_stand,tag=yy_surface_active,r=0.1,c=1] yy_surface_owner run tag @e[type=armor_stand,tag=yy_surface_active,r=0.1,c=1] add yy_surface_cleanup]==],
 [==[/kill @e[type=armor_stand,tag=yy_surface_cleanup]]==],
 [==[/tag @a[tag=yy_surface_failed] remove yy_surface_latched]==],
 [==[/tag @a[tag=yy_surface_failed] remove return_surface]==],
 [==[/tag @a[tag=yy_surface_failed] remove yy_surface_top]==],
 [==[/tag @a[tag=yy_surface_failed] remove yy_surface_spawn_issued]==],
 [==[/tag @a[tag=yy_surface_failed] remove yy_surface_confirm_sound]==],
 [==[/scoreboard players reset @a[tag=yy_surface_failed] yy_surface_age]==],
 [==[/tag @a[tag=yy_surface_failed] remove yy_surface_dim_nether]==],
 [==[/tag @a[tag=yy_surface_failed] remove yy_surface_dim_end]==],
 [==[/tag @a[tag=yy_surface_failed] remove yy_surface_dim_overworld]==],
 [==[/tag @a[tag=yy_surface_failed] remove yy_surface_failed]==],
}
local STEP_CHECK=[==[/execute as @e[type=armor_stand,tag=yy_surface_active,tag=!yy_surface_ready] at @s if block ~ ~ ~ air if block ~ ~1 ~ air unless block ~ ~-1 ~ air unless block ~ ~-1 ~ lava unless block ~ ~-1 ~ water run tag @s add yy_surface_ready]==]
local STEP_DOWN=[==[/execute as @e[type=armor_stand,tag=yy_surface_active,tag=!yy_surface_ready] at @s run tp @s ~ ~-1 ~]==]
local SURF_STEPS_PER_TICK=48
local function run_intake()
  S.intake_hits=S.intake_hits+1
  run_batch(INTAKE)
end
-- 雪球实体生成事件（快速路径）：事件字段形状沿用血月 bn_dig_type 的读法。
function museSnowSpawnEvt(e)
  if native_owner then return false end
  local t=nil
  if type(e)=="table" then
    t=e.entity_type or e.entityType or e.type
      or (type(e.target)=="table" and (e.target.entity_type or e.target.entityType) or nil)
      or (type(e.fields)=="table" and (e.fields.entity_type or e.fields.entityType or e.fields.type) or nil)
      or (type(e.data)=="table" and (e.data.entity_type or e.data.entityType or e.data.type) or nil)
  end
  if type(t)=="string" and t:lower():find("snowball",1,true) then run_intake() end
end
function muse_snow_menu_tick()
  if native_owner then return false end
  -- 世界边界巡逻（原链第 4 段，逐字）
  run_batch(WORLD_GUARD)
  local menu_open=any_match("testfor @a[scores={雪球菜单=1..6}]")
  local latched=any_match("testfor @a[tag=yy_surface_latched]")
  local tpa_on=any_match("testfor @a[scores={互传=1..}]")
  if menu_open==nil or latched==nil or tpa_on==nil then return false end
  if not menu_open and not latched and not tpa_on then
    if any_match("testfor @e[type=snowball]") then run_intake() end
    return true
  end
  if menu_open or latched or tpa_on then
    S.menu_runs=S.menu_runs+1
    run_batch(MENU_A)
    if menu_open then run_batch(MENU_B) end
    run_batch(MENU_D)
    run_batch(MENU_E)
  end
  if tpa_on then
    S.tpa_runs=S.tpa_runs+1
    run_batch(TPA_TICK)
  end
  if latched then
    S.surf_runs=S.surf_runs+1
    run_batch(SURF_SPAWN)
    local steps={}
    for _=1,SURF_STEPS_PER_TICK do steps[#steps+1]=STEP_CHECK; steps[#steps+1]=STEP_DOWN end
    run_batch(steps)
    run_batch(SURF_DONE)
  end
  if any_match("testfor @e[type=snowball]") then run_intake() end
  return true
end
function muse_snow_online_tick()
  if native_owner then return false end
  cmd("/scoreboard players add @a 在线时间 1")
  return true
end
function M.apply()
  if installed then return true end
  plugin_api=plugin_api or {}
  plugin_api.museSnowMenuStatus=function()
    return {version="muse-snowball-lua-v1",owner=native_owner and "native-chain" or "lua",
      probe_failures=S.probe_failures or 0,batch_errors=S.batch_errors or 0,intake_hits=S.intake_hits,menu_runs=S.menu_runs,
      tpa_runs=S.tpa_runs,surf_runs=S.surf_runs,probes=S.probes,probe_fail_open=S.probe_fail_open}
  end
  if type(chains)=="table" and type(chains.list)=="function" then
    local ok,snapshot=pcall(chains.list)
    if not ok or type(snapshot)~="table" or type(snapshot.files)~="table" then return false end
    for _,row in ipairs(snapshot.files) do
      if type(row.config)=="table" and row.config.name=="雪球菜单_综合功能.txt" then
        native_owner=true;installed=true
        return true -- A stopped native menu must stay stopped, including this Lua copy.
      end
    end
  end
  installed=true
  for _,s in ipairs({
    "/scoreboard objectives add 雪球菜单 dummy",
    "/scoreboard objectives add 互传 dummy",
    "/scoreboard objectives add 在线时间 dummy",
    "/scoreboard objectives add yy_surface_owner dummy",
    "/scoreboard objectives add yy_surface_age dummy",
  }) do cmd(s) end
  if type(events)=="table" and type(events.onEntitySpawn)=="function" then
    local ok=pcall(events.onEntitySpawn,"museSnowSpawnEvt")
    if not ok then pcall(events.onEntitySpawn,museSnowSpawnEvt) end
  end
  if type(util)=="table" and type(util.scheduleSeconds)=="function" then
    pcall(util.scheduleSeconds,1,"muse_snow_menu_tick")
    pcall(util.scheduleSeconds,60,"muse_snow_online_tick")
  end

  return true
end
return M
