// 按用户指定的位置整合设置入口；首页只展示创作与游戏工具。
CFG_SERVERS=[{url:'https://susu.bot.cd',label:'Susu · susu.bot.cd'},{url:'https://prism.adblanlu.qzz.io',label:'预置验证服务 2'}];
getServerLabel=function(url){var base=String(url).replace(/\/$/,'');var found=CFG_SERVERS.find(function(s){return s.url===base});return found?found.label:base};
yyModules.push({id:'search',name:'服务器搜索',desc:'查找服务器与管理投放队列',category:'connect',icon:'search',open:function(){return yySocialPage()}});
var yyHomeIDs=['import','export','mapart','blockvideo','converter','music','skin','search','files','more','library','mcfunction','globalcapture'];
var yyAllModules=yyModules;
var yyModuleLookup=yyModule;
yyModule=function(id){return yyAllModules.find(function(m){return m.id===id})};
yyModules=yyAllModules.filter(function(m){return yyHomeIDs.indexOf(m.id)>=0});
yyCategories.connect='实用工具';
var chainModule=yyModule('mcfunction');chainModule.name='命令链条';chainModule.desc='TXT 实例与条件链调度';chainModule.open=function(){return yyChainsPage()};
yyModule('ai').open=function(){tabTo('tasks');setTimeout(yyCreateWorkspaceDialog,400)};
if(!localStorage.getItem('youyuan-layout-v2')){yyPrefs.pins=['import','mapart','music','mcfunction','globalcapture'];localStorage.setItem('youyuan-layout-v2','1');yySavePrefs()}
yyPrefs.pins=yyPrefs.pins.filter(function(id){return yyHomeIDs.indexOf(id)>=0});
document.querySelector('.yy-sidebar-label:nth-of-type(2)');
var yySideLabels=document.querySelectorAll('.yy-sidebar-label');if(yySideLabels[1])yySideLabels[1].textContent='创作工具';
var yySideNavs=document.querySelectorAll('.yy-side-nav');if(yySideNavs[1])yySideNavs[1].innerHTML='<button onclick="yyOpenModule(\'files\')">'+yyIcon('folder')+'建筑文件</button><button onclick="yyOpenModule(\'mcfunction\')">'+yyIcon('code')+'命令链条</button><button onclick="yyOpenModule(\'music\')">'+yyIcon('music')+'音乐工坊</button>';
document.querySelector('[data-yy-tab="tasks"]').innerHTML=yyIcon('plugin')+'多语言插件';
RbuildingMgmtHTML=function(){return yyIntro('建筑文件','浏览原作者建筑市场，或管理本地建筑文件。')+'<div class="yy-form-grid"><section class="card">'+yyCardHead('建筑市场','box')+'<p>连接 建筑市场，浏览与下载建筑资料。</p><div class="yy-actions"><button class="btn" onclick="bmOpenMarket()">打开建筑市场</button></div></section><section class="card">'+yyCardHead('本地建筑','folder')+'<p>批量浏览设备中的建筑，查看预览与文件信息。</p><div class="yy-actions"><button class="btn-s" onclick="bmOpenBatch()">批量建筑预览</button><button class="btn-s" onclick="bmOpenContribute()">贡献文件</button></div></section></div>'};
pmOpenMarket=yyOriginalPluginMarket;bmOpenMarket=yyOriginalBuildingMarket;bmOpenContribute=yyOriginalBuildingContribute;
var yySettingsData={};cfgSpeed=10000;
var yySettingsRenderEpoch=0;
function yySettingsViewGuard(){var nav=typeof yyNavigateEpoch==='number'?yyNavigateEpoch:0,scope=yyReadScope();return function(){return curTab==='settings'&&!subPageOpen&&scope===yyReadScope()&&nav===(typeof yyNavigateEpoch==='number'?yyNavigateEpoch:0)}}

Rcfg=async function(){
 var epoch=++yySettingsRenderEpoch,scope=yyReadScope();
 function live(){var valid=epoch===yySettingsRenderEpoch&&scope===yyReadScope()&&curTab==='settings'&&!subPageOpen;if(!valid&&epoch===yySettingsRenderEpoch)tabInited.settings=false;return valid}
 document.getElementById('tab-settings').innerHTML='<div class="content-inner">'+yyHeading('账号与设置','服务器连接、群服互通与公网控制，统一在这里管理。')+`<div class="yy-form-grid"><div class="yy-form-stack"><section class="card" id="cfg-conn-card">${yyCardHead('服务器连接','connection','<span class="yy-badge" id="cfg-conn-st">读取中</span>')}<label for="bot-server">服务器号 / 分享链接</label><input id="bot-server" placeholder="输入房间号或分享链接" oninput="toggleServerPass()"><p class="yy-hint">直接填房间号即可：自动识别租赁服、联机房间与本地房间，有多个结果时让你选一次。</p><input id="bot-pass" type="password" class="gone" placeholder="服务器密码"><div class="yy-actions"><button class="btn" onclick="doConn()">连接服务器</button><button class="btn-s" onclick="doDisc()">断开连接</button><button class="btn-s" id="cfg-capture-entry" onclick="yyGlobalCapturePage()">全局抓包设置</button></div>${yySwitch('cfg-auto-connect','应用启动时自动连接',false)}${yySwitch('cfg-auto-reconnect','检测到断开后自动重连',false)}${yySwitch('cfg-auto-creative','自动维护创造模式与飞行（建筑作业可选）',false)}<p class="yy-hint">默认开启创造与飞行维护；受伤、死亡或生存测试时请关闭。</p><label for="cfg-packet-queue">收包缓存 · 包数量</label><input id="cfg-packet-queue" type="number" min="1" step="1" value="4096"><p class="yy-hint">默认4096，保存后下次连接生效。这是待处理包的缓存，不限制命令条数或每秒发送量；增大会占用更多内存。</p><label for="cfg-reconnect-seconds">重连间隔 · 秒</label><div class="row" style="gap:8px;align-items:center"><input id="cfg-reconnect-seconds" type="number" min="1" max="3600" step="1" value="10" onchange="yySaveReconnectSeconds(this)" style="flex:1;margin:0"><button type="button" class="btn-s" onclick="yySaveReconnectSeconds(document.getElementById('cfg-reconnect-seconds'))">保存</button></div><p class="yy-hint" id="cfg-reconnect-note">默认 10 秒；改完立即保存，下次打开仍是这个值。手动断开会暂停重连，点击连接后恢复。</p><button class="btn-s" onclick="if(window.android&amp;&amp;window.android.requestBackgroundProtection)window.android.requestBackgroundProtection();else T('电池保护设置在APK中使用')">设置后台电池保护</button></section><section class="card">${yyCardHead('验证服务器与账号','shield')}<label>验证服务器</label><div class="picker-btn" id="cfg-server-picker" data-url="https://susu.bot.cd" onclick="showServerPicker()"><span class="picker-label">Susu · susu.bot.cd</span><span class="picker-arrow">▾</span></div><p class="yy-hint">已预置三种验证服务：Susu、Prism 与游园独立认证，选择后会保存。</p><label for="cfg-token">游戏登录 Token</label><div class="row"><input id="cfg-token" type="password" oninput="onTokenInput()" autocomplete="off" placeholder="填写登录 Token" style="flex:1;min-width:0;margin:0"><button id="cfg-token-add" class="btn-s" onclick="addSubToken()" style="width:36px;display:none;margin:0;padding:6px">+</button><button class="yy-secondary" onclick="yyRevealField('cfg-token')" aria-label="显示或隐藏游戏 Token">${yyIcon('shield')}</button></div><div id="cfg-sub-token-list"></div><div class="yy-hint" id="cfg-acc-card"></div><div id="cfg-verif-btn-wrap" class="yy-actions"><button class="btn-s" onclick="saveCfg();setTimeout(openVerifSite,200)">打开验证服务器</button></div></section><section class="card" id="cfg-multi-card">${yyCardHead('多机器人作业','user','<span class="yy-badge" id="cfg-fleet-badge">读取中</span>')}<p class="yy-hint">主机器人连接后，子机器人用各自的令牌加入同一服务器；主机器人拥有 OP 时，子机器人权限自动授予。</p><div class="yy-actions"><button class="btn" id="cfg-fleet-conn" onclick="doFleetConn()">连接子机器人</button><button class="btn-s" onclick="doFleetDisc()">断开</button><button class="btn-s" onclick="yyAccountBridgePage()">跨账号联通</button><button class="yy-text-btn" onclick="refreshFleetOverview()">刷新状态</button></div><div id="cfg-fleet-status" class="yy-bot-overview"></div></section><section class="card" id="cfg-qq-card">${yyCardHead('群服互通','chat','<span class="yy-badge" id="cfg-qq-status">读取中</span>')}<p class="yy-hint">配置 OneBot 或 QQ 官方机器人，转发聊天消息并管理群指令。</p><div class="yy-actions"><button class="btn-s" onclick="yyQQPage()">接入与消息配置</button></div><p class="yy-hint">AI 模型不在这里配：语言、画图、语音统一到下面「AI 模型统一配置」里选一次。</p></section><section class="card" id="cfg-models-card">${yyCardHead('AI 模型统一配置','plugin','<span class="yy-badge" id="cfg-models-st">读取中</span>')}<p class="yy-hint">三个用途共用同一份模型库：地址和密钥只在模型库里填一次，这里只决定每个用途用哪一条。群服互通、生图、语音不再各存一套凭证。</p><label for="cfg-model-language">对话模型 · 语言</label><select id="cfg-model-language"><option value="">未选择</option></select><label for="cfg-model-image">画图模型 · 生图</label><select id="cfg-model-image"><option value="">未选择（生图不可用）</option></select><label for="cfg-model-image-size">出图尺寸</label><input id="cfg-model-image-size" placeholder="1024x1024"><label for="cfg-model-speech">语音模型 · TTS</label><select id="cfg-model-speech"><option value="">不用语音模型</option></select><label for="cfg-model-voice">音色</label><input id="cfg-model-voice" placeholder="alloy"><label for="cfg-model-speech-mode">语音模式</label><select id="cfg-model-speech-mode"><option value="off">关闭</option><option value="auto">AI 决定</option><option value="always">总是语音</option></select><div class="yy-actions"><button class="btn" onclick="yySaveModelRoles(this)">保存模型配置</button><button class="btn-s" onclick="yyOpenModelLibrary()">管理模型库</button><button class="yy-text-btn" onclick="yyLoadModelRoles(1)">重新读取</button></div><p class="yy-hint" id="cfg-models-note">正在读取模型库…</p></section><section class="card" id="cfg-watchdog-card">${yyCardHead('掉线智联（掉线自愈）','shield','<span class="yy-badge" id="cfg-wd-st">读取中</span>')}<p class="yy-hint">机器人掉线后自动重启协议端、走快速登录，不删登录态也不用重新扫码。以前这里只能看不能改，现在探测间隔和每小时重启上限都由你定，改完立即保存。</p>${yySwitch('cfg-wd-enabled','掉线后自动重启协议端',true)}<label for="cfg-wd-interval">探测间隔 · 秒（10–3600）</label><input id="cfg-wd-interval" type="number" min="10" max="3600" step="10" value="60"><label for="cfg-wd-max">每小时重启上限 · 次（1–100）</label><input id="cfg-wd-max" type="number" min="1" max="100" step="1" value="3"><div class="yy-actions"><button class="btn" onclick="yySaveWatchdog(this)">保存掉线智联</button><button class="yy-text-btn" onclick="yyLoadWatchdog(1)">重新读取</button></div><p class="yy-hint" id="cfg-wd-note">正在读取…</p></section></div><div class="yy-form-stack"><section class="card">${yyCardHead('作业速度与默认选项','settings')}<label for="cfg-speed">导入速度 · 方块 / 秒</label><input id="cfg-speed" type="number" min="1" value="10000"><label for="cfg-export-speed">导出速度 · 方块 / 秒</label><input id="cfg-export-speed" type="number" min="1" value="10000"><p class="yy-hint">按自定义数值运行，不设人为的最大值。</p>${yySwitch('cfg-import-cmds','导入命令方块指令',true)}${yySwitch('cfg-cmd-disabled','命令方块以关闭状态导入',false)}${yySwitch('cfg-exclude-fluids','排除流体方块',false)}${yySwitch('cfg-exclude-water','排除水',true)}${yySwitch('cfg-exclude-waterlogged','排除含水方块',true)}${yySwitch('cfg-exclude-lava','排除岩浆',true)}</section><section class="card">${yyCardHead('地图画默认选项','image')}${yySwitch('cfg-gravity','使用重力方块',false)}${yySwitch('cfg-platform','在下方放置玻璃平台',false)}</section><section class="card" id="cfg-mcp-card">${yyCardHead('公网远控与 MCP','cloud','<span class="yy-badge" id="cfg-mcp-status">读取中</span>')}<p class="yy-hint">远程调用工具箱全部已注册 API。设备通过自己的域名连接，插件在 APK 中执行。</p><div class="yy-code">https://toolbox.youyuanqi.dpdns.org/mcp/</div><div class="yy-actions"><button class="btn-s" onclick="yyRemotePage()">连接配置与密钥</button><button class="btn-s" onclick="yyMCPLinksPage()">MCP 专用连接</button></div></section><section class="card" id="cfg-file-card">${yyCardHead('文件选择器与界面','folder')}${yySwitch('cfg-picker','使用内置文件选择器',useBuiltinPicker)}<label>配色模式</label><button class="picker-btn" id="cfg-theme-picker" onclick="yyToggleTheme()"><span class="picker-label">${appTheme==='dark'?'深色 · 青绿':'浅色 · 青绿'}</span><span>切换</span></button><div class="yy-actions"><button class="btn-s" onclick="openPersonalizeSheet()">个性化设置</button><button class="btn-s" onclick="yyHostFilesPage()">上传 / 下载工具箱文件</button><button class="btn-s" onclick="clearCache()">清理缓存</button></div></section></div></div><section class="card yy-settings-save" id="yy-settings-save"><div><h3>保存当前设备设置</h3><p>保存连接与作业参数，下次启动继续使用。</p></div><button type="button" class="yy-primary" onclick="saveCfg()">保存设置 ${yyIcon('arrow')}</button></section><section class="card"><div class="yy-actions yy-actions-four" style="margin:0"><button class="btn-s" onclick="yyFeedbackPage()">工单反馈</button><button class="btn-s" onclick="yyCatalogPage()">全部 API</button><button class="btn-s" onclick="checkUpdateManual()">检测更新</button><button class="btn-s" onclick="yyPlatformPasswordPage()">修改平台密码</button></div></section><section class="card" id="cfg-library-card">${yyCardHead('自建云端资料库','folder')}<p class="yy-hint">自己的插件、建筑和音乐文件，保存到游园的云端空间。</p><div class="yy-actions"><button class="btn-s" onclick="yyLibraryPage()">整理与上传资料</button></div></section></div>`;
 document.getElementById('cfg-picker').onchange=function(){setPickerMode(this.checked)};
 var r=await refreshCfg(live);if(!live())return;renderSubTokenList();var c=r.config||{};yySettingsData=c;document.getElementById("cfg-packet-queue").value=c.packet_queue_capacity>0?c.packet_queue_capacity:4096;document.getElementById('cfg-speed').value=c.import_speed>0?c.import_speed:10000;document.getElementById('cfg-export-speed').value=c.export_speed>0?c.export_speed:10000;document.getElementById('cfg-auto-creative').checked=!!c.auto_creative;document.getElementById('cfg-auto-connect').checked=!!c.auto_connect;document.getElementById('cfg-auto-reconnect').checked=!!c.auto_reconnect;document.getElementById('cfg-reconnect-seconds').value=c.reconnect_seconds>0?c.reconnect_seconds:10;
 var states=await Promise.all([A('GET','/api/qq/status'),A('GET','/api/youyuan/remote')]);if(!live())return;if(document.getElementById('cfg-qq-status'))document.getElementById('cfg-qq-status').textContent=yyQQStatusLabel(states[0].state);if(document.getElementById('cfg-mcp-status'))document.getElementById('cfg-mcp-status').textContent=states[1].state||'设备离线';if(_cfgRefreshTimer)clearInterval(_cfgRefreshTimer);var timer=setInterval(function(){if(!live()){clearInterval(timer);if(_cfgRefreshTimer===timer)_cfgRefreshTimer=null;return}refreshCfgAccount()},10000);_cfgRefreshTimer=timer;activeRenderers.push(function(){clearInterval(timer);if(_cfgRefreshTimer===timer)_cfgRefreshTimer=null});
 // 统一模型配置与掉线智联：卡片渲染完再把服务端的选择回填，避免和 refreshCfg 抢同一帧。
 try{yyModelCardsInit()}catch(_){}
};
function yyQQStatusLabel(state){return {stopped:'未启用',disabled:'未启用',connected:'已连接',connecting:'连接中',error:'连接失败'}[state]||state||'未配置'}
saveCfg=async function(){var n=function(id){return Number(document.getElementById(id).value)};if(!Number.isSafeInteger(n('cfg-speed'))||n('cfg-speed')<1||!Number.isSafeInteger(n('cfg-export-speed'))||n('cfg-export-speed')<1){T('导入与导出速度请填写正整数','e');return}if(!Number.isSafeInteger(n('cfg-packet-queue'))||n('cfg-packet-queue')<1){T('收包缓存请填写正整数','e');return}var current=await A('GET','/api/config');if(!current.ok){T(current.error,'e');return}var c=Object.assign({},current.config,{token:document.getElementById('cfg-token').value,auth_url:curServerUrl,server_code:document.getElementById('bot-server').value,server_pass:document.getElementById('bot-pass').value,import_speed:n('cfg-speed'),export_speed:n('cfg-export-speed'),packet_queue_capacity:n('cfg-packet-queue'),auto_creative:document.getElementById('cfg-auto-creative').checked,auto_connect:document.getElementById('cfg-auto-connect').checked,auto_reconnect:document.getElementById('cfg-auto-reconnect').checked,reconnect_seconds:Math.max(1,n('cfg-reconnect-seconds')||10),bot_tokens:getSubTokens()});[['cfg-import-cmds','import_commands'],['cfg-cmd-disabled','cmd_disabled'],['cfg-exclude-fluids','exclude_fluids'],['cfg-exclude-water','exclude_water'],['cfg-exclude-waterlogged','exclude_waterlogged'],['cfg-exclude-lava','exclude_lava'],['cfg-gravity','use_gravity_blocks'],['cfg-platform','gravity_platform']].forEach(function(p){c[p[1]]=document.getElementById(p[0]).checked});var r=await A('POST','/api/config',c);if(r.ok){cfgSpeed=c.import_speed;cfgBotTokens=c.bot_tokens;cfgImportCommands=c.import_commands;cfgCmdDisabled=c.cmd_disabled;cfgExcludeFluids=c.exclude_fluids;cfgExcludeWater=c.exclude_water;cfgExcludeWaterlogged=c.exclude_waterlogged;cfgExcludeLava=c.exclude_lava;cfgGravity=c.use_gravity_blocks;cfgPlatform=c.gravity_platform;yySettingsData=c}T(r.ok?'设置已保存':r.error,r.ok?'o':'e')};
function yyFeedbackPage(){delete subPageCache['工单反馈'];openSubPage('工单反馈',yyIntro('记录问题与建议','描述操作步骤，设备会保存工单和时间，供后续处理。')+'<section class="card"><label for="yy-feedback">问题描述</label><textarea id="yy-feedback" placeholder="做了什么、预期结果、实际结果…"></textarea><div class="yy-actions"><button class="btn" onclick="yySendFeedback()">保存工单</button></div><pre class="yy-readout" id="yy-feedback-result"></pre></section>')}
async function yySendFeedback(){var r=await A('POST','/api/youyuan/feedback',{text:document.getElementById('yy-feedback').value});document.getElementById('yy-feedback-result').textContent=r.ok?'已保存工单：'+r.id:r.error}
// 终端支持一次提交命令串，分段由原生内核处理。
var yyTerminalBase=Rconsole;Rconsole=function(){yyTerminalBase();var input=document.getElementById('console-input');if(input)input.placeholder='/命令一./命令二. 或普通聊天';var hints=document.querySelector('#tab-terminal .content-inner>div:last-child');if(hints)hints.textContent='普通文字发送聊天；/ 开头执行命令；/say A./say B. 按顺序发送两条命令。'};
var yyChainRows=[];
async function yyChainsPage(){delete subPageCache['命令链条'];var r=await A('GET','/api/youyuan/chains');yyChainRows=r.files||[];openSubPage('命令链条',yyIntro('命令链条','每个文件独立启停与调度，条件链以真实命令响应判断成功。')+'<section class="card"><div class="yy-section-head"><h3>'+yyIcon('code')+'文件管理</h3><button class="yy-text-btn" onclick="yyChainsRefresh()">刷新状态</button></div><p class="yy-hint">目录：插件数据文件/ds_ai_agent/command_txt/</p><div class="yy-actions"><button class="btn" onclick="yyChainEdit()">新建命令链</button><button class="btn-s" onclick="yyChainUpload()">添加 TXT / mcfunction</button><button class="btn-s" onclick="yyCreateWorkspaceDialog(\'mcfunction\')">AI 帮写</button></div></section><div id="yy-chain-list"></div>');yyDrawChains();}
async function yyChainsRefresh(){var r=await A('GET','/api/youyuan/chains');if(!document.getElementById('yy-chain-list'))return;yyChainRows=r.files||[];yyDrawChains()}
function yyDrawChains(){var host=document.getElementById('yy-chain-list');if(!host)return;host.innerHTML=yyChainRows.length?yyChainRows.map(function(row){var c=row.config,name=JSON.stringify(c.name).replace(/"/g,'&quot;');return '<section class="card"><div class="yy-form-head"><h3>'+yyIcon('code')+yyEscape(c.name)+'</h3>'+yyBadge(c.mode==='fast'&&row.state==='已完成'?'提交结束 · 结果未逐条确认':row.state,row.state==='运行中')+'</div><div class="yy-meta">'+yyBadge(c.mode==='fast'?'快速提交':c.mode==='burst'?'机器人整批发送':c.mode==='pipeline'?'机器人有界流水线':c.mode==='stream'?'动态滑动窗口':'机器人严格顺序')+yyBadge((1000/c.interval_ms).toFixed(2)+' Hz / 整链')+yyBadge(c.delay_ms+' ms / 命令')+yyBadge(row.commands+' 条命令')+yyBadge(c.run_count?'运行 '+c.run_count+' 轮':'持续运行')+'</div><p class="yy-hint">尝试节点 '+row.executed+' · 实际提交 '+(row.submitted||0)+' · 跳过 '+row.skipped+' · 完成 '+row.cycles+' 轮 · 实际 '+(row.actual_hz||0).toFixed(2)+' 轮/秒 · 上轮 '+(row.last_round_ms||0)+' ms'+'</p><div class="yy-actions"><button class="btn" onclick="yyChainAction('+name+',\'start\')">启动</button><button class="btn-s" onclick="yyChainAction('+name+',\'stop\')">停止</button><button class="btn-s" onclick="yyChainAction('+name+',\'trigger\')">触发等待点</button><button class="btn-s" onclick="yyChainEdit('+name+')">编辑与频率</button></div>'+(row.error?'<pre class="yy-readout">'+yyEscape(row.error)+'</pre>':'')+'</section>'}).join(''):'<div class="yy-empty">还没有命令链文件，添加 TXT 后会显示在这里。</div>'}
async function yyChainAction(name,action,redstone){var r=await A('POST','/api/youyuan/chains',{name:name,action:action,redstone:!!redstone});if(!r.ok)T(r.error,'e');await yyChainsRefresh()}
async function yyChainEdit(name,content){var row=yyChainRows.find(function(r){return r.config.name===name});var c=row?row.config:{};if(name&&content===undefined){var r=await A('GET','/api/youyuan/chains?name='+encodeURIComponent(name));if(!r.ok){T(r.error,'e');return}content=r.content}delete subPageCache['编辑命令链'];openSubPage('编辑命令链',yyIntro('编辑命令链','属性继承 · . 分隔独立链 · ! 结束周期')+'<div class="yy-form-grid"><section class="card"><label for="yy-chain-name">文件名</label><input id="yy-chain-name" value="'+yyEscape(name||'新命令链.txt')+'"><div id="yy-chain-editor" style="height:380px;border:1px solid var(--phx-border);border-radius:9px;overflow:hidden"></div><div class="yy-actions"><button class="btn" onclick="yyChainSave()">保存文件</button></div></section><section class="card">'+yyCardHead('独立调度参数','clock')+'<label for="yy-chain-interval">周期频率 · 毫秒</label><input id="yy-chain-interval" type="number" min="25" value="'+(c.interval_ms||1000)+'"><label for="yy-chain-run-count">运行次数 · 0 为持续运行</label><input id="yy-chain-run-count" type="number" min="0" max="1000000" value="'+(c.run_count||0)+'"><label for="yy-chain-delay">每条命令延迟 · 毫秒</label><input id="yy-chain-delay" type="number" min="0" value="'+(c.delay_ms||0)+'"><label for="yy-chain-account">账号标识</label><input id="yy-chain-account" value="'+yyEscape(c.account||'main')+'"><p class="yy-hint">0 连锁 / 1 脉冲 / 2 循环<br>条件：0 无条件 / 1 有条件<br>第三位为旧格式兼容字段，当前运行器忽略红石门控。<br><br>整个文件按周期运行；0 次为持续，正数为完整轮数，完成后自动停止。有条件命令仅在上一条成功时执行。每条延迟可为0，默认不额外限速；有条件命令按真实结果判断。目标频率属于整条链，执行超时不会堆积轮次。</p><pre class="yy-readout" id="yy-chain-result"></pre></section></div>');window.yyChainEditor=YYEditor.create(document.getElementById('yy-chain-editor'),content||'1,0,0\nsay 开始执行\n0,1,0\nsay 上一条执行成功\n!\n','main.mcfunction','mcfunction',function(){});}
async function yyChainSave(){var r=await A('POST','/api/youyuan/chains',{action:'save',name:document.getElementById('yy-chain-name').value,content:yyChainEditor.state.doc.toString(),config:{account:document.getElementById('yy-chain-account').value,interval_ms:Number(document.getElementById('yy-chain-interval').value),delay_ms:Number(document.getElementById('yy-chain-delay').value),enabled:false}});document.getElementById('yy-chain-result').textContent=r.ok?'文件已保存并预解析；返回管理页启动。':r.error}
function yyChainUpload(){var input=document.createElement('input');input.type='file';input.accept='.txt,.mcfunction';input.onchange=async function(){if(input.files[0])await yyChainEdit(input.files[0].name,await input.files[0].text())};input.click()}

// ===== 令牌体系与机器人总览（任务#97）=====
// 数据来源：/api/bot/status（主机器人）、/api/fleet/status（子机器人）。
// 子令牌在服务端是纯字符串数组，没有名字段；名称取进服后的 DisplayName，未进服时按序号显示。
var yyFleetConnecting=false,yyFleetPoll=null,yyFleetOverviewSeq=0;
function yySubTokenValues(){return _subTokens.filter(function(t){return t&&String(t).trim()})}
// 令牌数量与界面行数不一致时才重绘：避免 10 秒轮询把用户正在输入的内容冲掉。
function yySyncSubTokenList(){var el=document.getElementById('cfg-sub-token-list');if(!el)return;if(el.querySelectorAll('.yy-token-row').length!==_subTokens.length)renderSubTokenList()}
function yyFleetStopPoll(){yyFleetConnecting=false;if(yyFleetPoll){clearInterval(yyFleetPoll);yyFleetPoll=null}var b=document.getElementById('cfg-fleet-conn');if(b)b.disabled=false}
function yyBotRowHTML(bot){
  var dot='<span class="yy-bot-dot'+(bot.connected?' on':'')+'" aria-hidden="true"></span>';
  var avatar='<span class="yy-bot-avatar'+(bot.main?' is-main':'')+'">'+yyEscape(bot.label)+dot+'</span>';
  var badges='';
  if(bot.connected&&bot.op)badges+=yyBadge('OP',true);
  else if(!bot.connected&&bot.connecting)badges+=yyBadge('连接中');
  var meta=bot.error?'<span class="yy-bot-meta is-err">'+yyEscape(bot.error)+'</span>':'<span class="yy-bot-meta">'+yyEscape(bot.meta||'')+'</span>';
  return '<div class="yy-bot-row'+(bot.connected?'':' is-off')+'">'+avatar+'<div class="yy-bot-info"><span class="yy-bot-name">'+yyEscape(bot.name)+badges+'</span>'+meta+'</div></div>';
}
function yyMainBotFrom(bot,fleet){
  var connected=bot?!!bot.connected:!!(fleet&&fleet.main_connected);
  var op=bot?!!bot.is_op:!!(fleet&&fleet.main_op);
  var display='',server='';
  if(fleet&&fleet.workers){var w=fleet.workers.filter(function(x){return x.main})[0];if(w){display=w.name||'';server=w.server||''}}
  if(!server&&bot)server=bot.connected?(bot.server||''):(bot.server_target||'');
  var name='主机器人'+(display?' · '+display:'');
  var meta=connected?('服务器 '+(server||'未知')):(bot&&bot.server_target?'未连接 · 目标 '+bot.server_target:'未连接');
  return {main:true,label:'主',name:name,connected:connected,op:op,connecting:false,error:'',meta:meta,server:connected?server:''};
}
function yySubBotsFrom(fleet,mainServer){
  // workers 只含已进服的机器人，连接失败的子令牌不会进 Subs，位置与 status 索引不对齐；
  // 因此按 DisplayName 匹配（stat[i].Name 与 bm.display 同源），匹配不到时回落到主机器人所在服务器。
  var workers=(fleet&&fleet.workers)?fleet.workers.filter(function(x){return !x.main}):[];
  var stat=(fleet&&fleet.status)?fleet.status:[];
  return yySubTokenValues().map(function(_,i){
    var st=stat.filter(function(x){return x.index===i})[0]||{};
    var w=st.name?workers.filter(function(x){return x.name===st.name})[0]:null;
    var name=st.name||(w&&w.name)||('子机器人 '+(i+1));
    var connected=!!st.connected;
    var server=(w&&w.server)||(connected?mainServer:'');
    var meta=connected?('服务器 '+(server||'未知')):(yyFleetConnecting?'正在加入服务器…':'未连接');
    return {main:false,label:String(i+1),name:name,connected:connected,op:!!st.is_op||!!(w&&w.is_op),connecting:yyFleetConnecting&&!connected,error:st.error||'',meta:meta};
  });
}
function yyRenderBotOverview(fleet,bot){
  var host=document.getElementById('cfg-fleet-status');
  if(!host)return;
  var badge=document.getElementById('cfg-fleet-badge');
  if(!fleet||!fleet.ok){
    if(badge)badge.textContent=yyFleetConnecting?'连接中':'状态未知';
    host.innerHTML='<div class="yy-hint" style="margin:4px 0">'+(yyFleetConnecting?'正在连接子机器人，状态更新中…':'设备离线，状态稍后自动刷新。')+'</div>';
    return;
  }
  var mainBot=yyMainBotFrom(bot,fleet);
  var subs=yySubBotsFrom(fleet,mainBot.server);
  if(badge)badge.textContent=subs.length?(subs.filter(function(b){return b.connected}).length+'/'+subs.length+' 子机器人在线'):(mainBot.connected?'主机器人在线':'主机器人未连接');
  var html=yyBotRowHTML(mainBot)+subs.map(yyBotRowHTML).join('');
  if(!subs.length)html+='<div class="yy-hint" style="margin:4px 0">还没有子机器人。在「验证服务器与账号」填好主令牌后，点 + 添加子令牌。</div>';
  host.innerHTML=html;
}
async function refreshFleetOverview(){
  var seq=++yyFleetOverviewSeq;
  var results=await Promise.all([A('GET','/api/fleet/status'),A('GET','/api/bot/status')]);
  if(seq!==yyFleetOverviewSeq||!document.getElementById('cfg-fleet-status'))return;
  yyRenderBotOverview(results[0],results[1]);
}
// 子令牌行：圆形序号 + 掩码切换 + 删除；末尾一个添加槽。输入框 ID 保持不变，getSubTokens 照旧工作。
renderSubTokenList=function(){
  var el=document.getElementById('cfg-sub-token-list');
  if(!el)return;
  var rows=_subTokens.map(function(t,i){
    return '<div class="yy-token-row"><span class="yy-token-avatar">'+(i+1)+'</span><input id="cfg-sub-token-'+i+'" type="password" autocomplete="off" placeholder="子机器人 '+(i+1)+' Token" value="'+escHtml(t)+'" oninput="onSubTokenInput('+i+')"><button type="button" class="btn-s yy-token-btn" aria-label="显示或隐藏子机器人 '+(i+1)+' Token" onclick="yyRevealField(\'cfg-sub-token-'+i+'\')">'+yyIcon('shield')+'</button><button type="button" class="btn-s yy-token-btn" style="color:var(--phx-error)" aria-label="删除子机器人 '+(i+1)+'" onclick="removeSubToken('+i+')">&times;</button></div>';
  }).join('');
  el.innerHTML=rows+'<button type="button" class="yy-token-slot" onclick="addSubToken()">'+yyIcon('user')+'<span>添加子机器人</span></button>';
};
refreshCfgAccount=async function(){
  var el=document.getElementById('cfg-acc-card');
  if(el)el.textContent='主令牌对应主机器人，加入服务器后统一指挥全部子机器人；每个子令牌对应一个子机器人，在验证服务器单独签发。';
  refreshCfgStatus();
  yySyncSubTokenList();
  refreshFleetOverview();
};
doFleetConn=async function(){
  var btn=document.getElementById('cfg-fleet-conn');
  if(btn)btn.disabled=true;
  await saveCfg();
  var r=await A('POST','/api/fleet/connect');
  if(!r.ok){T(r.error||'连接失败','e');if(btn)btn.disabled=false;return}
  T('正在连接子机器人...','o');
  yyFleetStopPoll();
  yyFleetConnecting=true;
  var tries=0;
  yyFleetPoll=setInterval(async function(){
    if(!document.getElementById('cfg-fleet-status')){yyFleetStopPoll();return}
    yyFleetOverviewSeq++;
    var s=await A('GET','/api/fleet/status');
    yyRenderBotOverview(s,null);
    var done=s.ok&&s.status&&s.status.length>0&&s.status.every(function(b){return b.connected});
    if(done||++tries>40){yyFleetStopPoll();if(done)refreshFleetOverview()}
  },1500);
  // 离开设置页时兜底停轮询（与 10 秒定时器同一清理机制）。
  if(activeRenderers&&activeRenderers.push)activeRenderers.push(yyFleetStopPoll);
};
doFleetDisc=async function(){
  yyFleetStopPoll();
  await A('POST','/api/fleet/disconnect');
  T('子机器人已断开','o');
  refreshFleetOverview();
};

// 重连间隔：改动即保存（不再依赖底部“保存设置”，避免改完没保存、下次又回到旧值）。
// 走 /api/config 读改写，只动 reconnect_seconds 一个字段，失败明确报错并回填旧值。
async function yySaveReconnectSeconds(input) {
  if (!input) return;
  const note = document.getElementById('cfg-reconnect-note');
  const want = Math.max(1, Math.min(3600, Math.round(Number(input.value) || 10)));
  input.disabled = true;
  try {
    const current = await A('GET', '/api/config');
    if (!current.ok) throw Error(current.error || '读取当前配置失败');
    const c = Object.assign({}, current.config, { reconnect_seconds: want });
    const r = await A('POST', '/api/config', c);
    if (!r.ok) throw Error(r.error || '保存失败');
    if (yySettingsData && typeof yySettingsData === 'object') yySettingsData.reconnect_seconds = want;
    input.value = want;
    if (note) note.textContent = '已保存：重连间隔 ' + want + ' 秒。手动断开会暂停重连，点击连接后恢复。';
    T('重连间隔已保存：' + want + ' 秒', 'o');
  } catch (e) {
    const saved = (yySettingsData && yySettingsData.reconnect_seconds > 0) ? yySettingsData.reconnect_seconds : 10;
    input.value = saved;
    if (note) note.textContent = '保存失败，已恢复为 ' + saved + ' 秒：' + (e.message || '未知错误');
    T(e.message || '保存失败，请重试', 'e');
  } finally {
    input.disabled = false;
  }
}

// ===== 统一模型配置（语言 / 画图 / 语音）=====
// 只存「用途指向模型库里的哪一条」，地址与密钥仍然只在模型库填一次。
// 后端 POST /api/ai/models 带 roles 时会把选择写穿到群服互通与生图配置，
// 所以这里保存完必须重新 GET 一次，用服务端回读的值回填（不猜、不自嗨）。
var yyModelRoleList = [], yyModelRoleSel = {};
function yyModelFillSelect(id, cur, emptyLabel) {
  var sel = document.getElementById(id);
  if (!sel) return;
  var html = '<option value="">' + emptyLabel + '</option>';
  yyModelRoleList.forEach(function (m) {
    html += '<option value="' + escHtml(m.name) + '">' +
      escHtml(m.name + (m.model_id ? ' · ' + m.model_id : '')) + '</option>';
  });
  sel.innerHTML = html;
  if (cur && !yyModelRoleList.some(function (m) { return m.name === cur; })) {
    var o = document.createElement('option');
    o.value = cur; o.textContent = cur + '（不在模型库）';
    sel.appendChild(o);
  }
  sel.value = cur || '';
}
async function yyLoadModelRoles(force) {
  var note = document.getElementById('cfg-models-note'), badge = document.getElementById('cfg-models-st');
  if (!note) return;
  if (force) note.textContent = '正在读取…';
  var r = await A('GET', '/api/ai/models');
  if (!document.getElementById('cfg-models-note')) return;
  if (!r || r.ok === false) {
    note.textContent = '读取模型库失败：' + ((r && r.error) || '未知错误');
    if (badge) badge.textContent = '读取失败';
    return;
  }
  yyModelRoleList = r.models || [];
  yyModelRoleSel = r.roles || {};
  var lang = yyModelRoleSel.language || {}, img = yyModelRoleSel.image || {}, sp = yyModelRoleSel.speech || {};
  yyModelFillSelect('cfg-model-language', lang.model, '未选择');
  yyModelFillSelect('cfg-model-image', img.model, '未选择（生图不可用）');
  yyModelFillSelect('cfg-model-speech', sp.model, '不用语音模型');
  var sizeEl = document.getElementById('cfg-model-image-size');
  if (sizeEl) sizeEl.value = img.size || '1024x1024';
  var voiceEl = document.getElementById('cfg-model-voice');
  if (voiceEl) voiceEl.value = sp.voice || '';
  var modeEl = document.getElementById('cfg-model-speech-mode');
  if (modeEl) modeEl.value = sp.mode || 'off';
  if (badge) badge.textContent = yyModelRoleList.length ? (yyModelRoleList.length + ' 个模型') : '模型库为空';
  note.textContent = yyModelRolesLine(lang, img, sp) +
    (yyModelRoleList.length ? '' : '。模型库是空的，点「管理模型库」先添加一条地址与密钥。');
}
function yyModelRolesLine(lang, img, sp) {
  return [['语言', lang], ['画图', img], ['语音', sp]].map(function (p) {
    var it = p[1] || {};
    if (!it.model) return p[0] + '：未选择';
    return p[0] + '：' + it.model + (it.model_id ? '（' + it.model_id + '）' : '') +
      (it.ready === false ? ' · ' + (it.warning || '模型不在库') : '');
  }).join('；');
}
async function yySaveModelRoles(btn) {
  var g = function (id) { var el = document.getElementById(id); return el ? el.value : ''; };
  var roles = {
    language: { model: g('cfg-model-language') },
    image: { model: g('cfg-model-image'), size: g('cfg-model-image-size') },
    speech: { model: g('cfg-model-speech'), voice: g('cfg-model-voice'), mode: g('cfg-model-speech-mode') || 'off' }
  };
  if (btn) btn.disabled = true;
  var r;
  try { r = await A('POST', '/api/ai/models', { roles: roles }); }
  catch (e) { r = { ok: false, error: (e && e.message) || '网络错误' }; }
  if (btn) btn.disabled = false;
  if (!r || r.ok === false) { T((r && r.error) || '保存模型配置失败', 'e'); return; }
  await yyLoadModelRoles(1);
  var note = document.getElementById('cfg-models-note');
  if (note) note.textContent = '已保存并同步到群服互通与生图 · ' + note.textContent;
  T('模型配置已保存', 'o');
}

// ===== 掉线智联（掉线自愈）=====
// 后端 /api/qq/watchdog 一直支持 interval_seconds 与 max_restarts，
// 只是原来的面板把这两个输入框写死成 disabled、还标了「后端待支持」。这里如实开放。
async function yyLoadWatchdog(force) {
  var note = document.getElementById('cfg-wd-note'), badge = document.getElementById('cfg-wd-st');
  if (!note) return;
  if (force) note.textContent = '正在读取…';
  var r = await A('GET', '/api/qq/watchdog');
  if (!document.getElementById('cfg-wd-note')) return;
  if (!r || r.ok === false) {
    note.textContent = '读取掉线智联失败：' + ((r && r.error) || '未知错误');
    if (badge) badge.textContent = '读取失败';
    return;
  }
  var el = document.getElementById('cfg-wd-enabled');
  if (el) el.checked = !!r.enabled;
  el = document.getElementById('cfg-wd-interval');
  if (el) el.value = r.interval_seconds > 0 ? r.interval_seconds : 60;
  el = document.getElementById('cfg-wd-max');
  if (el) el.value = r.max_restarts > 0 ? r.max_restarts : 3;
  if (badge) badge.textContent = r.enabled ? '已开启' : '已关闭';
  var bits = [];
  bits.push(r.has_restart_cmd ? '重启命令已配置' : '还没配置重启命令（YY_NAPCAT_RESTART_CMD），掉线后只能人工扫码');
  bits.push('近 1 小时已自动重启 ' + (r.restarts_1h || 0) + ' 次');
  if (r.cool_seconds) bits.push('冷却中，还需 ' + Math.ceil(r.cool_seconds / 60) + ' 分钟');
  if (r.last_error) bits.push('上次失败：' + r.last_error);
  note.textContent = bits.join(' · ');
}
async function yySaveWatchdog(btn) {
  var num = function (id, d) {
    var el = document.getElementById(id), v = Number(el ? el.value : d);
    return Number.isFinite(v) ? v : d;
  };
  var box = document.getElementById('cfg-wd-enabled');
  var body = {
    enabled: !!(box && box.checked),
    interval_seconds: Math.round(num('cfg-wd-interval', 60)),
    max_restarts: Math.round(num('cfg-wd-max', 3))
  };
  if (body.interval_seconds < 10 || body.interval_seconds > 3600) { T('探测间隔要在 10–3600 秒之间', 'e'); return; }
  if (body.max_restarts < 1 || body.max_restarts > 100) { T('每小时重启上限要在 1–100 次之间', 'e'); return; }
  if (btn) btn.disabled = true;
  var r;
  try { r = await A('POST', '/api/qq/watchdog', body); }
  catch (e) { r = { ok: false, error: (e && e.message) || '网络错误' }; }
  if (btn) btn.disabled = false;
  if (!r || r.ok === false) { T((r && r.error) || '保存掉线智联失败', 'e'); return; }
  await yyLoadWatchdog(1);
  var note = document.getElementById('cfg-wd-note');
  if (note) note.textContent = '已保存：每 ' + body.interval_seconds + ' 秒探测一次，每小时最多重启 ' +
    body.max_restarts + ' 次 · ' + note.textContent;
  T('掉线智联设置已保存', 'o');
}
function yyModelCardsInit() { yyLoadModelRoles(0); yyLoadWatchdog(0); }
// 模型库弹窗在 features/ai_assist.js 里；页面没加载它时给出可读提示，不留一个点了没反应的按钮。
function yyOpenModelLibrary() {
  if (typeof aiOpenManage === "function") { aiOpenManage(); return; }
  T("模型库页面未加载，请刷新后重试", "e");
}
