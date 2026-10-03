// 群服互通面板。挂在设置页「服务器连接」卡片的展开区里，是第三组
// （前两组是「连接保持」在线锁 与 「对外开放」服务器信息上报）。
//
// 与那两组不同，本组**对所有人可见**（那两组的显隐由各自的资格决定），
// 功能按三档权限置灰而不是隐藏：
//   所有人   → 消息互通
//   特殊身份 → 加上「群里执行指令」   （_cachedToolboxInfo.online_lock_allowed）
//   订阅     → 加上「图片卡片」       （_cachedToolboxInfo.subscription_active）
//
// 接口：
//   GET  /api/qq/status  → {enabled,adapter,group,state,detail,warnings}
//                          state ∈ unconfigured|connecting|open|reconnecting|stopped
//   GET  /api/qq/config  → {ok,config:{…}}，**凭据只进不出**（只有 has_token/has_secret）
//   POST /api/qq/config  ← 补丁（只带要改的字段）
//
// 类名统一 qb- 前缀；.conn-bar 与 .toast 已被 App 其它部分占用，不能复用
// （与 server_report.js 同一条规矩）。样式尽量复用 .sr-*。

// ── 桥接日志 ──
// 后端经 SSE 的 qq_log 事件推来（sse.js 里调 qbLogPush），**不进全局 LOG_IDS**：
// 那是导入/导出/控制台共用的通道，桥接的日常唠叨会把那些日志冲掉。
// 设置页 DOM 会被 Rcfg 整块重建，所以自己在内存里留一份环形缓冲，重挂时回填。
var QB_LOG_MAX=500;
var _qbLogs=[];
// 日志容器那一行单独抽出来：paintQQPanel 要从生成串里精确摘掉它再比骨架
// （DOM 里那一段是活的行，生成串里永远是空的），两边必须一字不差。
// 只有这一处定义，改它不用改别处。
var QB_LOG_PANE='<div class="log" id="qb-log"></div>';

// 收到一条 qq_log。sse.js 调用；面板不在 DOM 里时只进缓冲，等挂载时回填。
function qbLogPush(msg){
  var s=(msg===null||msg===undefined)?'':String(msg);
  _qbLogs.push(s);
  if(_qbLogs.length>QB_LOG_MAX)_qbLogs.splice(0,_qbLogs.length-QB_LOG_MAX);
  var el=document.getElementById('qb-log');
  if(!el) return;
  var d=document.createElement('div');
  d.textContent=s;   // 群消息是任意文本，只能走 textContent，不能 innerHTML
  el.appendChild(d);
  // 裁剪必须在 append **之后**：先算再补的话，满 500 行时 over 为 0、一条不裁，
  // 补完变 501 而缓冲只有 500 —— 下一次 qbBackfillLog 就判定两边不同步，
  // 走 innerHTML='' 整段重建 500 个节点，每 5 秒一次。同 utils.js 的 capLogDom。
  while(el.childNodes.length>QB_LOG_MAX&&el.firstChild)el.removeChild(el.firstChild);
  el.scrollTop=el.scrollHeight;
  el._n=el.childNodes.length;
}

// 把缓冲里还没画的行补上。面板重建后 el._n 丢失，会整段重画。
function qbBackfillLog(){
  var el=document.getElementById('qb-log');
  if(!el) return;
  var have=el._n||0;
  if(have===_qbLogs.length) return;
  if(have>_qbLogs.length||!el.childNodes.length){el.innerHTML='';have=0}
  for(var i=have;i<_qbLogs.length;i++){
    var d=document.createElement('div');
    d.textContent=_qbLogs[i];
    el.appendChild(d);
  }
  el._n=_qbLogs.length;
  el.scrollTop=el.scrollHeight;
}

// ── 面板状态 ──
var _qbStatus=null;   // /api/qq/status 最新一份；null=还没拿到
var _qbConfig=null;   // /api/qq/config 的 config 段；null=还没拿到
var _qbBusy=false;    // 开关请求在飞，防连点
var _qbTimer=null;    // 状态轮询（设置页停留期间）

var QB_STATE_TEXT={unconfigured:'未配置',connecting:'连接中',open:'已连接',reconnecting:'重连中',stopped:'已停止'};
var QB_PERMS=['群主','管理员','所有人','禁用'];
// 内置占位符清单，与后端 qqPlaceholderNames()（qq_placeholder.go）一字不差。
// 显示名与它们重名时求值走内置、计分板项取不到值，所以向导里要当场提示。
var QB_PLACEHOLDER_BUILTIN=['时间','玩家','消息','服务器','在线人数','TPS'];
// 后端默认计分板表（config/qq_bridge.go 的 DefaultQQBridge）。
// **只在服务端没给 scoreboards 时兜底**：用户存过的表（哪怕是空表）一律照用，
// 绝不用默认值覆盖回去 —— 那正是「静默回写用户设置」。
var QB_SB_DEFAULT=[{name:'e',label:'金币'},{name:'not',label:'权限'},{name:'time',label:'在线时长'},{name:'ID',label:'ID'}];

// 三档权限。_cachedToolboxInfo 在匿名/还没拉回来时是 null —— 那时三档全按没有处理，
// 面板照常渲染，只是功能都锁着，不能因为拿不到账号信息就整个不显示。
function qbTier(){
  var ti=_cachedToolboxInfo;
  return {cmd:!!(ti&&ti.online_lock_allowed),img:!!(ti&&ti.subscription_active)};
}

function qbStateText(s){return QB_STATE_TEXT[s]||'未配置'}
// 状态点的类名同样走白名单。source 是个封闭的 Go 枚举、也只从本机回，
// 理论风险为零；但这是文件里唯一一处把值直接拼进 class 属性的地方，
// 其余插值一律 escHtml —— 不留这个例外。
function qbStateClass(s){return QB_STATE_TEXT[s]?s:'stopped'}

// 配置里那份「谁能执行指令」。取值只认 QB_PERMS 白名单，其它一律按后端默认
// 「群主」显示（config/qq_bridge.go 的 DefaultQQBridge 也是 群主）。
function qbPerm(cf){
  return (cf&&cf.command&&QB_PERMS.indexOf(cf.command.permission)>=0)?cf.command.permission:'群主';
}

// 「群里执行指令」那一行的标签。这里是**权限设置**的口径，不是「你有没有资格」：
//   tier.cmd 为假 → 账号根本没解锁这项功能，只能报「需特殊身份」，
//                   不能把一个他用不了的身份值摆在他面前（那才是误导）。
//   解了锁       → 照实报他自己设的那个值（群主/管理员/所有人），
//                  禁用则连点带字一起灰掉 —— 绿点亮着说「能用」是假的。
function qbCmdTag(cf,tier){
  if(!tier.cmd) return {on:false,tag:'需特殊身份'};
  var p=qbPerm(cf);
  if(p==='禁用') return {on:false,tag:'已禁用'};
  return {on:true,tag:p};
}

// 显示名列表 → 一行文字；空表给一句明确的「空了会怎样」，不能只留一片空白。
function qbBoardLabels(cf){
  var bs=(cf&&cf.scoreboards)||[];
  var out=[];
  for(var i=0;i<bs.length;i++){
    if(!bs[i]||!bs[i].name) continue;
    out.push(String(bs[i].label||bs[i].name));
  }
  return out;
}

// ── 面板 ──
// 拉配置 + 状态后整块重画。refreshCfg() 与轮询都走这里。
async function renderQQPanel(){
  var body=document.getElementById('cfg-qq-body');
  if(!body) return;
  var r=await A('GET','/api/qq/config');
  body=document.getElementById('cfg-qq-body');   // await 期间面板可能已被重建
  if(!body) return;
  _qbConfig=(r&&r.ok&&r.config)?r.config:null;
  await qbRefreshStatus();
}

// 只刷状态（便宜），配置沿用缓存。轮询走这条。
async function qbRefreshStatus(){
  var body=document.getElementById('cfg-qq-body');
  if(!body) return;
  var st=await A('GET','/api/qq/status');
  body=document.getElementById('cfg-qq-body');
  if(!body) return;
  // 状态接口没有 ok 字段，认 state 在不在
  _qbStatus=(st&&typeof st.state==='string')?st:null;
  paintQQPanel();
}

// qq_state 推送（sse.js 调用）：适配器状态一变，后端就 Emit 一份 ——
// 连上/掉线/重连、启动失败、官方机器人自动绑定群，全在 qq_bridge.go 里。
// payload 与 GET /api/qq/status 同形，直接当状态用，不用再回一次接口。
function qbStatePush(d){
  if(!d||typeof d.state!=='string') return;
  _qbStatus=d;
  paintQQPanel();
}

// 设置页的周期刷新（startCfgPoll 起的表）。离开设置页就自己收摊，
// 与 refreshReportDetail 同一套路（不能靠 curTab 提前 return：首次进入时
// startCfgPoll 跑在 tabTo 给 curTab 赋值之前）。
function refreshQQPanel(){
  if(typeof curTab!=='undefined'&&curTab!=='settings'){
    if(_qbTimer){clearInterval(_qbTimer);_qbTimer=null}
    return;
  }
  qbRefreshStatus();
}

function qbExpanded(enabled,configured){
  // 开着当然要展开（要看状态与日志）；没配过也要展开，否则「前往配置」的入口
  // 藏在一个收起的面板里，用户永远找不到。只有「配好了但被用户关掉」才收起。
  return !!enabled||!configured;
}

function paintQQPanel(){
  var body=document.getElementById('cfg-qq-body');
  if(!body) return;
  // 内容没变就不写 DOM：轮询会把日志区连同滚动位置一起重建。
  //
  // 但日志容器的**行**是活内容，生成串里那一段永远是空的 —— 拿整个 innerHTML
  // 去比，只要面板上有日志就永远不等，于是每次轮询都判定「变了」、整块重建，
  // 500 行日志和滚动位置一起丢。所以比之前先把日志容器整个摘下来，
  // 并且把生成串里对应那一段也去掉，两边只比骨架。
  var el=document.getElementById('qb-log');
  var parent=el?el.parentNode:null;
  var st=el?el.scrollTop:0;    // 摘下来的时候节点会丢滚动位置，放回去要还原
  if(el) parent.removeChild(el);
  var html=qbPanelHTML();
  if(body.innerHTML!==html.replace(QB_LOG_PANE,'')){
    body.innerHTML=html;      // 骨架真变了：重建（新日志容器是空的，下面回填）
  }else if(el){
    parent.appendChild(el);   // 骨架没变：原样放回去，行一个都不动
    el.scrollTop=st;
  }
  qbBackfillLog();
}

function qbPanelHTML(){
  var cf=_qbConfig;
  var st=_qbStatus||{};
  var state=st.state||'unconfigured';
  var enabled=!!(cf&&cf.enabled);
  var configured=!!(cf&&cf.configured);
  var tier=qbTier();
  var h=''+
  '<div class="sr-master'+(enabled?' on':'')+'" id="cfg-qq-master">'+
    '<div class="sr-master-txt">'+
      '<div class="sr-master-t">群服互通</div>'+
      '<div class="sr-master-d">'+escHtml(qbMasterDesc(cf,state))+'</div>'+
    '</div>'+
    '<label class="lbl-cb" style="margin:0;flex:0 0 auto">'+
      // 没配过时不给拨：开了也没有适配器可连，只会刷一屏连不上的日志。
      // checked 与 disabled 都必须写成 attr=""（不是裸布尔属性）——裸的序列化回来
      // 会带上 ="",与生成串**永不相等**，于是「没变就不写 DOM」形同虚设：
      // 每次轮询整块重建面板，连带把 500 行的日志区（和它的滚动位置）重建一遍。
      // 下面这个 checked 以前就是裸的，正是这么失效的（同 server_report.js 的教训）。
      '<input type="checkbox" id="cfg-qq-switch"'+(enabled?' checked=""':'')+
        (configured?'':' disabled=""')+' onchange="qbToggle(this.checked)">'+
      '<span class="tgl"></span>'+
    '</label>'+
  '</div>'+
  // 收起的那一档（配好了但被关掉）里明细整块不见了，「重新配置」也随之消失 ——
  // 用户为了改一个坏的 ws 地址把桥关掉，回来就再没有入口进向导，只剩一个拨不动的
  // 开关。所以收起时在开关下面单留一个入口。（这是 .sr-body 的**兄弟**，
  // 不影响它那套「只有一个会 overflow:hidden 的孩子」的收起动画。）
  ((configured&&!enabled)
    ?'<div class="qb-acts"><button class="sr-mini" onclick="qbOpenWizard()">重新配置</button></div>'
    :'')+
  // sr-body 里必须是「唯一一个会 overflow:hidden 的孩子」——收起动画靠
  // grid-template-rows 0fr→1fr，多一个孩子就会多出一行 auto 高度、收不干净。
  '<div class="sr-body'+(qbExpanded(enabled,configured)?' on':'')+'" id="cfg-qq-detail"><div>'+
    qbDetailHTML(cf,state,tier)+
  '</div></div>'+
  // 基础体验版提示：只在「既没订阅、又没特殊身份」时出现。
  // 特殊身份用户能被挡住的只有图片卡片，他不是「基础体验版」，不该看到这句。
  ((tier.cmd||tier.img)?''
    :'<div class="sr-note">基础体验版：仅消息互通可用。群里执行指令需要特殊身份，图片卡片需要订阅，完整功能请购买订阅。</div>');
  return h;
}

function qbMasterDesc(cf,state){
  if(!cf) return '把服务器里的聊天和 QQ 群连起来';
  if(!cf.configured) return '把服务器里的聊天和 QQ 群连起来（还没有配置）';
  // 配置视图给的是 group_id（数字），不是 group —— group 只在状态接口上有
  // （状态那份是适配器当前绑定的群，字符串）。群号约 10 位，远在 JS Number 的精确范围内。
  var g=(cf.group_id>0)?('  群 '+cf.group_id):'';
  var who=(cf.adapter==='qqofficial')?'QQ 官方机器人':'NapCat';
  return who+g+' · '+(cf.enabled?qbStateText(state):'已关闭');
}

function qbDetailHTML(cf,state,tier){
  if(!cf){
    return '<div class="sr-empty">读取配置失败</div>'+
      '<div class="qb-acts"><button class="sr-mini" onclick="renderQQPanel()">重新读取</button></div>';
  }
  var h='';
  if(!cf.configured){
    h+='<div class="sr-empty">还没有配置群服互通</div>'+
      '<div class="sr-note">配置好之后，服务器里的聊天会转发到 QQ 群，群里的消息也会转发进服务器。</div>'+
      '<div class="qb-acts"><button class="sr-mini" onclick="qbOpenWizard()">前往配置</button></div>';
  }else{
    h+='<div class="qb-status">'+
        '<span class="qb-dot qb-dot-'+qbStateClass(state)+'"></span>'+
        '<span class="qb-status-t">'+qbStateText(state)+'</span>'+
        ((_qbStatus&&_qbStatus.group)?'<span class="qb-status-g">群 '+escHtml(String(_qbStatus.group))+'</span>':'')+
      '</div>';
    if(_qbStatus&&_qbStatus.detail) h+='<div class="sr-note">'+escHtml(String(_qbStatus.detail))+'</div>';
    // 后端给的是「当前配置下必须让用户知道的坑」（没绑群 / 官方机器人的角色与资料页设置）。
    // 照实转出来，比前端自己写一遍可靠 —— 它跟着实际配置走。
    var w=(_qbStatus&&_qbStatus.warnings)||[];
    if(w.length){
      h+='<div class="qb-tips">';
      for(var i=0;i<w.length;i++) h+='<div class="qb-tip">'+escHtml(String(w[i]))+'</div>';
      h+='</div>';
    }
    h+='<div class="qb-acts">'+
        '<button class="sr-mini" onclick="qbOpenWizard()">重新配置</button>'+
        // 卡片模板：选内置 / 改代码 / 保存即热重载 / 复制给网页 AI 的提示词 / 实时预览。
        // 放这里而不是向导里——它是「配好之后接着调」的事，不是配置流程的一步。
        '<button class="sr-mini" onclick="qbOpenTemplates()">卡片模板</button>'+
      '</div>';
  }
  h+=qbSummaryHTML(cf,tier);
  h+=qbFeatsHTML(cf,tier);
  h+=qbLogHTML(state);
  return h;
}

// 「当前设置」：把向导写进去、而「可用功能」那三行讲不到的东西摊开。
// 之前这里只有状态行 + 三档权限，用户刚在向导里设完「指令权限=群主」回来，
// 面板上看不到任何一个自己填的值，只会以为设置没保存（或以为「所有人可用」
// 那行讲的就是他的权限设置）。
//
// **每次重画都从 _qbConfig 现读**，不在这里留副本：留副本就会在向导保存之后
// 继续显示上一版的值，而这个面板正靠 qbRefreshStatus 每几秒重画一次。
function qbSummaryHTML(cf,tier){
  if(!cf||!cf.configured) return '';
  var g2g=cf.game_to_group||{};
  var img=cf.chat_image||{};
  // 服务端把空数组编成 null，两处都要按「没配」处理
  var pf=g2g.only_prefixes||[];
  var wl=(cf.command&&cf.command.whitelist)||[];
  var boards=qbBoardLabels(cf);
  // 进服/退服与死亡播报默认开：缺字段按开处理（===false 才算关），与向导同一口径，
  // 否则老配置里那两个缺字段的开关会被这里画成关的。
  function onoff(b){return b===false?'关':'开'}
  var fmt=String(g2g.format||'');
  function kv(k,v,mute){
    return '<div class="qb-kv'+(mute?' mute':'')+'">'+
      '<span class="qb-kv-k">'+escHtml(k)+'</span>'+
      '<span class="qb-kv-v">'+escHtml(String(v))+'</span></div>';
  }
  var h='<div class="sr-panel qb-panel"><div class="sr-chips">'+
    '<div class="sr-chips-t">当前设置</div>'+
    kv('转发格式',fmt||'未设置',!fmt)+
    kv('只转发前缀',pf.length?pf.join('，'):'全部消息',!pf.length)+
    kv('进服退服播报',onoff(g2g.notify_join_leave),false)+
    kv('死亡播报',onoff(g2g.notify_death),false)+
    // 白名单只在 OneBot 下显示，理由见 qbWizStep4：官方模式拿不到 QQ 号，
    // 那里的白名单要比对 member_openid，用户无从获取，也就无从填写。
    (cf.adapter==='qqofficial'?'':kv('白名单 QQ 号',wl.length?wl.join('，'):'无',!wl.length))+
    // 指令权限放在这里，是本次修复的正题：用户刚设完「群主」，面板上必须有这个值。
    // 上面「可用功能」那行报的是**他能不能用**（没解锁就只有「需特殊身份」），
    // 这一行报的是**他配了什么**，两者不是一回事，缺了后者用户就以为设置没保存。
    kv('指令权限',qbPerm(cf),false)+
    kv('积分项',boards.length?boards.join('，'):'无（卡片不出积分榜列）',!boards.length)+
    // 图片卡片是订阅功能：没订阅时开关本来就点不动，这里照实报「需订阅」，
    // 而不是报一个用户改不了的开/关。
    kv('图片卡片',tier.img?(img.enabled?'开':'关'):'需订阅',!tier.img);
  if(boards.length){
    h+='<div class="sr-note">积分项的显示名有两种用法：既是转发格式里的 ${显示名}，'+
      '也是聊天卡片上积分榜的列名。</div>';
  }
  return h+'</div></div>';
}

// 三档权限的可视化。**锁着的不隐藏**，做成灰底 + 灰点 + 「需订阅」/「需特殊身份」，
// 与 server_report.js 对锁定位的处理同一条规矩。
// 刻意不用 .sr-chip：那是按钮的样式，而这几个功能点不动，做成按钮就是在骗点击。
function qbFeatsHTML(cf,tier){
  function row(on,label,tag){
    return '<div class="qb-feat'+(on?'':' locked')+'">'+
      '<span class="qb-feat-m"></span>'+
      '<span class="qb-feat-l">'+escHtml(label)+'</span>'+
      '<span class="qb-feat-t">'+escHtml(tag)+'</span></div>';
  }
  var ct=qbCmdTag(cf,tier);
  return '<div class="sr-panel qb-panel">'+
    '<div class="sr-chips">'+
      '<div class="sr-chips-t">可用功能</div>'+
      // 「所有人可用」以前是这里的一句**分档**说明（哪些账号能用消息互通），
      // 但它和权限设置的措辞撞在一起，用户会读成「谁能执行指令=所有人」——
      // 正是本次要修的误导。改成「全部账号」，只说分档，不与权限用词重叠。
      row(true,'消息互通','全部账号')+
      row(ct.on,'群里执行指令',ct.tag)+
      row(tier.img,'图片卡片',tier.img?'已解锁':'需订阅')+
    '</div></div>';
}

function qbLogHTML(state){
  // 连上了当然要给日志；没连上但有日志（正是排查时最需要的）也照给。
  if(state!=='open'&&!_qbLogs.length) return '';
  return '<div class="sr-panel qb-panel">'+
    '<div class="sr-chips">'+
      '<div class="sr-chips-t">桥接日志</div>'+
      QB_LOG_PANE+
    '</div></div>';
}

// ── 开关 ──
async function qbToggle(on){
  var sw=document.getElementById('cfg-qq-switch');
  if(_qbBusy) return;
  _qbBusy=true;
  if(sw) sw.disabled=true;
  var r=await A('POST','/api/qq/config',{enabled:!!on});
  _qbBusy=false;
  if(!r||!r.ok){
    T((r&&r.error)||'操作失败','e');
    // 失败要把开关拨回真实状态，否则界面显示的和服务端存的不一致
    if(sw){sw.disabled=false;sw.checked=!!(_qbConfig&&_qbConfig.enabled)}
    return;
  }
  if(r.config) _qbConfig=r.config;
  T(on?'已开启群服互通':'已关闭群服互通','o');
  paintQQPanel();
  qbRefreshStatus();
}

// ═══ 配置向导 ═══
// 一步一步来：一个容器 + 步号 + 每步收集表单后再前进（照 migrate.js 的形状，
// 那边没有可复用的向导组件，wizard.js 又是另一套首启流程，不掺和）。
var _qbWizOv=null;
var _qbWizStep=1;
var _qbWizBusy=false;
var _qbWiz={};

var QB_WZ_TITLES={1:'接入方式',2:'连接信息',3:'绑定群',4:'转发与权限',5:'计分板与积分榜'};
// 步数只有这一处写死（渲染、步号、校验、收尾都读它），加一步不用改四处。
var QB_WZ_STEPS=5;

// 官方机器人这条路上必须让用户看见的三条。用户漏看它们，接着就会来报
// 「配好了没反应」——所以直接摆在绑定群那一步，而不是塞进帮助文档。
var QB_TIPS_OFFICIAL=[
  '必须<b>手动在群里 @ 一下机器人</b>，它才能拿到会话并完成绑定。',
  '只填群号、不 @ 机器人，<b>有时会发不出消息</b>。',
  '群内机器人资料卡里要开启「获取群聊消息(全部)」和「主动发言」，否则收不到非 @ 的消息。'
];

function qbListJoin(a,def){
  // 服务端可能回 null（没配过 / 存的是空数组），一律当默认值处理
  if(a===null||a===undefined) return (def||[]).join('，');
  if(!a.length) return '';
  return a.join('，');
}
function qbListSplit(s){
  var out=[];
  String(s||'').split(/[，,]/).forEach(function(x){x=x.trim();if(x)out.push(x)});
  return out;
}

// 配置里的计分板表 → 向导内部结构。**只有服务端没给（null/undefined）才铺默认表**：
// 给了空数组说明用户自己清空过，再铺一遍默认值就是把他删掉的项又塞回去。
function qbSbFromCfg(cf){
  var raw=cf?cf.scoreboards:null;
  if(!raw||!raw.length) return (raw===null||raw===undefined)?QB_SB_DEFAULT.slice():[];
  var out=[];
  for(var i=0;i<raw.length;i++){
    if(!raw[i]||!raw[i].name) continue;   // 后端也跳过没有标识的行（qqBoardDefs）
    out.push({name:String(raw[i].name),label:String(raw[i].label||'')});
  }
  return out;
}

function qbOpenWizard(){
  var cf=_qbConfig;
  _qbWizStep=1;
  _qbWizBusy=false;
  _qbWiz={
    adapter:(cf&&cf.adapter==='qqofficial')?'qqofficial':'onebot',
    ws_url:(cf&&cf.onebot&&cf.onebot.ws_url)||'ws://',
    // 凭据只进不出：服务端从不回明文，所以打开向导时输入框一律留空=保持不变。
    // 但**用户在本次向导里亲手打的** token/secret 必须回填（见 qbWizStep2）——
    // 那是他自己的输入，不是服务端凭据；不回填的话「上一步→下一步」就会把
    // 输入框清空，下次收集把值覆盖成空串，patch 里干脆不带这一项，保存"成功"
    // 而令牌已经丢了，事后只会看到一句连不上的报错。
    token:'',
    app_id:(cf&&cf.qqofficial&&cf.qqofficial.app_id)||'',
    app_secret:'',
    group_id:(cf&&cf.group_id>0)?String(cf.group_id):'',
    format:(cf&&cf.game_to_group&&cf.game_to_group.format)||'<${玩家}> ${消息}',
    // 只转发前缀（白名单）。留空 = 每条游戏消息都转发到群里。
    // 老配置里的 block_prefixes（黑名单）不再读进向导，保存时会被清掉。
    only_prefixes:qbListJoin(cf&&cf.game_to_group&&cf.game_to_group.only_prefixes,[]),
    // 进服/退服、死亡播报：默认开。服务端缺字段时按开处理（===false 才算关），
    // 否则老配置里那两个缺字段的开关会被这里画成关的，用户一保存就真把它关了。
    notify_join:(!cf||!cf.game_to_group||cf.game_to_group.notify_join_leave!==false),
    notify_death:(!cf||!cf.game_to_group||cf.game_to_group.notify_death!==false),
    perm:(cf&&cf.command&&QB_PERMS.indexOf(cf.command.permission)>=0)?cf.command.permission:'群主',
    whitelist:qbListJoin(cf&&cf.command&&cf.command.whitelist,[]),
    image:!!(cf&&cf.chat_image&&cf.chat_image.enabled),
    // 计分板项（服务端计分板名 → 显示名）。服务端给过就是用户自己的表，
    // **空表也照用**（他就是想清空）；只有服务端根本没给这一项时才铺默认表。
    scoreboards:qbSbFromCfg(cf)
  };
  var ov=document.createElement('div');
  ov.className='modal-overlay';
  ov.innerHTML='<div class="modal-box" style="max-width:380px"><div id="qb-wz-body"></div></div>';
  ov.addEventListener('click',function(e){if(e.target===ov)qbCloseWizard()});
  document.body.appendChild(ov);
  _qbWizOv=ov;
  qbWizRender(1);
}

function qbCloseWizard(){
  if(_qbWizOv){closeOverlay(_qbWizOv);_qbWizOv=null}
}

// 步号 1-4；传 5 = 走完（保存）。收集当前表单后再动，和 migrate.js 的 mgrtGoStep 同理。
function qbWizGo(n){
  if(n<1) return;
  if(n===_qbWizStep) return;
  var from=_qbWizStep;
  var forward=n>_qbWizStep;
  qbWizCollect();
  // 校验的是**正要离开的那一步**（值刚从它的表单里收上来）。
  // 传目标步号会校验一个还没填的表单，用户在第 1 步就会被第 2 步的规则挡住。
  if(forward&&!qbWizCheck(from)) return;   // 只挡前进：往回退永远放行
  // 收尾（n>QB_WZ_STEPS）必须排在校验**之后**：以前它写在最前面直接 return，
  // 于是最后一步永远不被校验，清空格式串也能保存。
  if(n>QB_WZ_STEPS){ qbWizFinish(); return; }
  qbWizRender(n);
}

function qbWizCollect(){
  var el;
  if((el=document.getElementById('qb-ws'))) _qbWiz.ws_url=el.value.trim();
  if((el=document.getElementById('qb-tk'))) _qbWiz.token=el.value;
  if((el=document.getElementById('qb-appid'))) _qbWiz.app_id=el.value.trim();
  if((el=document.getElementById('qb-secret'))) _qbWiz.app_secret=el.value;
  if((el=document.getElementById('qb-group'))) _qbWiz.group_id=el.value.trim();
  if((el=document.getElementById('qb-fmt'))) _qbWiz.format=el.value;
  if((el=document.getElementById('qb-pfx'))) _qbWiz.only_prefixes=el.value;
  if((el=document.getElementById('qb-wl'))) _qbWiz.whitelist=el.value;
  if((el=document.getElementById('qb-img'))) _qbWiz.image=el.checked;
  if((el=document.getElementById('qb-join'))) _qbWiz.notify_join=el.checked;
  if((el=document.getElementById('qb-death'))) _qbWiz.notify_death=el.checked;
  var pp=document.getElementById('qb-perm-picker');
  if(pp) _qbWiz.perm=pp.getAttribute('data-value')||'群主';
  qbSbCollect();
}

// 计分板表的行是从 _qbWiz.scoreboards 生成的，两个输入框按**行号**对齐取回：
// 没有 id（行是动态增删的，编号会变），改用容器内按序取。
function qbSbCollect(){
  var box=document.getElementById('qb-sb-rows');
  if(!box) return;
  var ns=box.querySelectorAll('.qb-sb-n');
  var ls=box.querySelectorAll('.qb-sb-l');
  var out=[];
  for(var i=0;i<ns.length;i++){
    out.push({
      name:ns[i].value.trim(),
      label:(ls[i]?ls[i].value:'').trim()
    });
  }
  _qbWiz.scoreboards=out;
}

// 加/删一行。先把当前输入收上来再动，否则用户刚打的字会随重画一起丢。
function qbSbAdd(){
  qbSbCollect();
  _qbWiz.scoreboards.push({name:'',label:''});
  qbWizRender(_qbWizStep);
}
function qbSbDel(i){
  qbSbCollect();
  if(i<0||i>=_qbWiz.scoreboards.length) return;
  _qbWiz.scoreboards.splice(i,1);
  qbWizRender(_qbWizStep);
}

// 显示名与内置占位符重名时就地提示（输入时，不等保存）。
// 后端加载时会把这类显示名改成「X(计分板)」并给出警告——结果虽然不丢，
// 但用户写的 ${X} 取的是内置值，不提前说清楚他就是配了个静默无效的项。
function qbSbHint(el){
  var row=el&&el.parentNode;
  var w=row?row.querySelector('.qb-sb-warn'):null;
  if(!w) return;
  var lab=String(el.value||'').trim();
  w.textContent=(lab&&QB_PLACEHOLDER_BUILTIN.indexOf(lab)>=0)
    ? '「'+lab+'」与内置占位符同名：${'+lab+'} 取的永远是内置值，保存后这个显示名会自动改成「'+lab+'(计分板)」。换个名字才用得上。'
    : '';
}

// 校验「刚离开的那一步」。返回 false 时把原因讲清楚，并停在原地。
function qbWizCheck(from){
  if(from===1) return true;   // 选适配器，点了就是选了
  if(from===2){
    if(_qbWiz.adapter==='onebot'){
      if(!/^wss?:\/\/.+/i.test(_qbWiz.ws_url)){T('请填写 ws:// 或 wss:// 开头的地址','e');return false}
      return true;
    }
    if(!_qbWiz.app_id){T('请填写 AppID','e');return false}
    // 空 secret = 保持原来的：只有本来就没设过才必须填
    if(!_qbWiz.app_secret&&!( _qbConfig&&_qbConfig.qqofficial&&_qbConfig.qqofficial.has_secret)){
      T('请填写 AppSecret','e');return false;
    }
    return true;
  }
  if(from===3){
    if(_qbWiz.adapter==='onebot'){
      var g=parseInt(_qbWiz.group_id,10);
      if(!(g>0)){T('请填写需要互通的 QQ 群号','e');return false}
    }
    // 官方机器人不给填群号：真实群号它拿不到，只能靠群里发消息自动记下
    return true;
  }
  if(from===4){
    // 格式串默认就有值，这里只会挡住「手滑清空」——空格式串转发出去就是一行空文本，
    // 后端会直接丢掉，用户看到的现象是「配好了但群里什么都不来」。
    if(!String(_qbWiz.format||'').trim()){T('请填写游戏到群的转发格式','e');return false}
    return true;
  }
  if(from===5){
    // 只填了显示名、没填标识的行会被后端整行丢掉（qqBoardDefs 跳过 Name 为空的行），
    // 用户以为加了一项、结果什么都没生效，所以这里当场挡住。
    var bs=_qbWiz.scoreboards||[];
    for(var i=0;i<bs.length;i++){
      if(String(bs[i].label||'').trim()&&!String(bs[i].name||'').trim()){
        T('第 '+(i+1)+' 项填了显示名但没填计分板标识，请补上或点 × 删掉这一项','e');
        return false;
      }
    }
    return true;
  }
  return true;
}

function qbWizRender(n){
  _qbWizStep=n;
  var body=document.getElementById('qb-wz-body');
  if(!body) return;
  var inner;
  if(n===1) inner=qbWizStep1();
  else if(n===2) inner=qbWizStep2();
  else if(n===3) inner=qbWizStep3();
  else if(n===4) inner=qbWizStep4();
  else inner=qbWizStep5();
  var dots='';
  for(var i=1;i<=QB_WZ_STEPS;i++) dots+='<span class="qb-step'+(i===n?' on':'')+'"></span>';
  body.innerHTML=
    '<div class="qb-wz-head">'+
      '<span class="qb-wz-n">第 '+n+' / '+QB_WZ_STEPS+' 步</span>'+
      '<button class="qb-wz-x" type="button" onclick="qbCloseWizard()" aria-label="关闭">'+
        '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>'+
      '</button>'+
    '</div>'+
    '<div class="qb-wz-title">'+QB_WZ_TITLES[n]+'</div>'+
    '<div class="qb-wz-inner">'+inner+'</div>'+
    '<div class="qb-steps">'+dots+'</div>'+
    '<div class="row" style="margin-top:12px">'+
      (n>1?'<button class="btn-s" style="margin:0" onclick="qbWizGo('+(n-1)+')">上一步</button>':'')+
      '<button class="btn" id="qb-wz-next" style="flex:1;margin:0" onclick="qbWizGo('+(n+1)+')">'+(n<QB_WZ_STEPS?'下一步':'完成')+'</button>'+
    '</div>';
}

function qbWizStep1(){
  var a=_qbWiz.adapter;
  function opt(v,t,d){
    return '<button type="button" class="qb-opt'+(a===v?' on':'')+'" onclick="qbPickAdapter(\''+v+'\')">'+
      '<span class="qb-opt-t">'+t+'</span><span class="qb-opt-d">'+d+'</span></button>';
  }
  return opt('onebot','NapCat（OneBot）','自己搭一个 NapCat，让它用 WebSocket 连过来。配置简单，不用审核。')
       + opt('qqofficial','QQ 官方机器人','用 QQ 开放平台申请的机器人。消息走官方接口，审批通过才能用。');
}

function qbPickAdapter(v){
  _qbWiz.adapter=v;
  qbWizRender(_qbWizStep);
}

function qbWizStep2(){
  if(_qbWiz.adapter==='qqofficial'){
    var hasS=!!(_qbConfig&&_qbConfig.qqofficial&&_qbConfig.qqofficial.has_secret);
    // 注册入口摆在最前：AppID/AppSecret 都是注册之后才能拿到的，先填再想注册
    // 只会卡在这两个输入框上。链接由 MainActivity 的 shouldOverrideUrlLoading
    // 丢给系统浏览器（WebView 里打不开开放平台）。
    return '<div class="qb-hint" style="margin:0 0 8px">'+
      '<a href="https://q.qq.com/qqbot/openclaw/index.html" target="_blank" rel="noopener">点击注册机器人</a></div>'+
      '<label class="qb-lbl">AppID</label>'+
      '<input id="qb-appid" placeholder="QQ 开放平台的 AppID" value="'+escHtml(_qbWiz.app_id)+'">'+
      '<label class="qb-lbl">AppSecret</label>'+
      '<input id="qb-secret" type="password" placeholder="'+(hasS?'已设置，留空表示不修改':'在开放平台后台查看 AppSecret')+'" value="'+escHtml(_qbWiz.app_secret)+'">'+
      '<div class="qb-hint">AppSecret '+(hasS?'已经设置过。为了安全，这里看不到原值，留空即为保持不变。':'只在保存时使用一次，之后界面上不会回显。')+'</div>';
  }
  var hasT=!!(_qbConfig&&_qbConfig.onebot&&_qbConfig.onebot.has_token);
  return '<label class="qb-lbl">NapCat 的 WebSocket 地址</label>'+
    '<input id="qb-ws" placeholder="ws://192.168.1.5:3001" value="'+escHtml(_qbWiz.ws_url)+'">'+
    '<div class="qb-hint">在 NapCat 的「网络配置」里新建一个 WebSocket 服务器，把它给出的地址填在这里。</div>'+
    '<label class="qb-lbl">访问令牌（可留空）</label>'+
    '<input id="qb-tk" type="password" placeholder="'+(hasT?'已设置，留空表示不修改':'NapCat 没开鉴权就留空')+'" value="'+escHtml(_qbWiz.token)+'">'+
    '<div class="qb-hint">'+(hasT
      ? '已经设置过访问令牌。为了安全，这里看不到原值，留空即为保持不变；要清除只能手改配置文件。'
      : '对应 NapCat 里的 access token。没开鉴权就留空。')+'</div>';
}

function qbWizStep3(){
  if(_qbWiz.adapter==='qqofficial'){
    var tips='';
    for(var i=0;i<QB_TIPS_OFFICIAL.length;i++) tips+='<div class="qb-tip">'+QB_TIPS_OFFICIAL[i]+'</div>';
    return '<div class="sr-note" style="margin:0 0 8px">官方机器人拿不到群号，绑定是<b>自动</b>的：机器人收到群里第一条消息时，会把那个群记下来。所以你不用填群号。</div>'+
      '<div class="qb-tips">'+tips+'</div>';
  }
  return '<label class="qb-lbl">要互通的 QQ 群号</label>'+
    '<input id="qb-group" inputmode="numeric" placeholder="例如 123456789" value="'+escHtml(_qbWiz.group_id)+'">'+
    '<div class="qb-hint">机器人在哪个群里，就填哪个群的群号。群号在 QQ 群的「群资料」里可以看到。</div>';
}

function qbWizStep4(){
  var h=''+
  '<label class="qb-lbl">游戏 → 群的格式</label>'+
  '<input id="qb-fmt" value="'+escHtml(_qbWiz.format)+'">'+
  '<div class="qb-hint">${玩家} 是游戏内昵称，${消息} 是聊天内容，${服务器} 是服务器名。'+
  '下一步还能把计分板项配成 ${金币} 这样的占位符。</div>'+
  '<label class="qb-lbl">只转发到群的前缀（可留空）</label>'+
  '<input id="qb-pfx" placeholder="留空=每条消息都转发" value="'+escHtml(_qbWiz.only_prefixes)+'">'+
  '<div class="qb-hint">只把<b>以这些字符开头</b>的聊天发到群里，多个用逗号分隔。'+
  '<b>留空则转发每条消息</b>（游戏内指令也会进群）。填了前缀时，前缀本身会从消息里去掉。</div>'+
  '<label class="qb-lbl">谁能执行指令</label>'+
  // 四个取值就地铺成一行（同第 1 步的接入方式），**不能**用 showPicker：
  // .bs-overlay 的 z-index(999) 低于 .modal-overlay(2000)，在向导里弹出来会被
  // 自己的遮罩盖住——看不见、点不到；向导关掉之后它还会作为孤儿留在设置页上。
  '<div class="qb-perms" id="qb-perm-picker" data-value="'+escHtml(_qbWiz.perm)+'">'+
    QB_PERMS.map(function(p){
      return '<button type="button" class="qb-perm'+(p===_qbWiz.perm?' on':'')+'" data-value="'+escHtml(p)+
        '" onclick="qbPickPerm(\''+escHtml(p)+'\')">'+escHtml(p)+'</button>';
    }).join('')+
  '</div>'+
  '<div class="qb-hint">'+(qbTier().cmd
    ? '只有达到这个身份的群成员才能在群里执行游戏指令。'
    : '群里执行指令需要特殊身份，当前账号还没解锁，这里只影响以后的配置。')+'</div>';
  // 白名单只在 OneBot 下给。官方机器人拿不到真实 QQ 号：平台一律用 OpenID 体系，
  // 且没有「QQ 号 → member_openid」的接口（union_openid 要特殊申请、只在单独拉
  // 成员信息时才返回）。那一栏让用户填的东西他无从获取，摆着只会白填，所以整个隐藏
  // ——官方模式下权限一律由上面的身份档位决定，那边是能判的（author.member_role）。
  // 隐藏不等于清空：_qbWiz.whitelist 已从配置载入，提交时原样带回，老配置不会丢。
  if(_qbWiz.adapter!=='qqofficial'){
    h+='<label class="qb-lbl">指令白名单（可留空）</label>'+
      '<input id="qb-wl" placeholder="留空=按上面的身份判定" value="'+escHtml(_qbWiz.whitelist)+'">'+
      '<div class="qb-hint">填 QQ 号，多个用逗号分隔。白名单里的人无视上面的身份，永远能执行指令。</div>';
  }
  // 进服/退服、死亡播报：**文字**播报，所有人都有，不按订阅分档。
  // 图片卡片才是订阅功能；图片关掉时后端自动退回文字，这里也这么写清楚。
  h+='<label class="lbl-cb" style="margin:14px 0 0"><input type="checkbox" id="qb-join"'+
    (_qbWiz.notify_join?' checked':'')+'><span class="tgl"></span> 播报玩家进服 / 退服</label>'+
  '<label class="lbl-cb" style="margin:8px 0 0"><input type="checkbox" id="qb-death"'+
    (_qbWiz.notify_death?' checked':'')+'><span class="tgl"></span> 播报玩家死亡</label>'+
  '<div class="qb-hint">这两条是文字播报，所有人都能用。图片卡片关掉时，它们照常以文字发到群里。</div>';
  var tier=qbTier();
  if(tier.img){
    h+='<label class="lbl-cb" style="margin:12px 0 0"><input type="checkbox" id="qb-img"'+
      (_qbWiz.image?' checked':'')+'><span class="tgl"></span> 图片卡片（未订阅时以文字发送）</label>';
  }else{
    h+='<div class="qb-feat locked" style="margin-top:12px">'+
      '<span class="qb-feat-m"></span><span class="qb-feat-l">图片卡片</span>'+
      '<span class="qb-feat-t">需订阅</span></div>'+
      '<div class="qb-hint">把聊天渲染成图片发进群，需要订阅后才能开启；'+
      '没订阅时会以文字发送，播报本身不受影响。</div>';
  }
  return h;
}

// 第 5 步：计分板项。这张表（服务端计分板名 → 显示名）驱动两件事 ——
// 转发格式里的 ${显示名} 占位符，以及聊天卡片上积分榜的列。用户没有别的地方
// 能知道这两件事是同一张表，所以这里必须把它讲明白。
function qbWizStep5(){
  var bs=_qbWiz.scoreboards||[];
  var rows='';
  for(var i=0;i<bs.length;i++){
    // 没有 id：行会增删，编号会变，取回时按容器内顺序对齐（见 qbSbCollect）。
    // .qb-sb-warn 默认是空的，由 qbSbHint 在输入时就地填字。
    rows+='<div class="qb-sb-row">'+
      // 两列都只有 placeholder，没有 .qb-lbl（行是动态增删的，一列一个可见标签
      // 排不下），所以各给一个 aria-label，读屏时才知道这格是标识还是显示名。
      '<input class="qb-sb-n" aria-label="计分板标识" placeholder="标识 如 e" value="'+escHtml(bs[i].name)+'">'+
      '<input class="qb-sb-l" aria-label="显示名" placeholder="显示名 如 金币" value="'+escHtml(bs[i].label)+
        '" oninput="qbSbHint(this)">'+
      '<button type="button" class="qb-sb-x" title="删除这一项" aria-label="删除这一项"'+
        ' onclick="qbSbDel('+i+')">×</button>'+
      '<div class="qb-sb-warn"></div>'+
    '</div>';
  }
  return '<div class="sr-note" style="margin:0 0 8px">计分板项配的是<b>显示名</b>：它既是转发格式里的 '+
    '<b>${显示名}</b> 占位符（例如把 <b>e</b> 配成<b>金币</b>，就能在格式里写 ${金币}），'+
    '也是聊天卡片上积分榜的<b>列名</b>。</div>'+
    '<div id="qb-sb-rows">'+rows+'</div>'+
    '<div class="qb-acts"><button class="sr-mini" type="button" onclick="qbSbAdd()">添加一项</button></div>'+
    '<div class="qb-hint">标识填服务端计分板的内部名（<b>/scoreboard objectives list</b> 里看到的那个），'+
    '显示名随便起，填给自己看的。这张表留空就是<b>没有计分板项</b>：格式里不会有 ${显示名} 占位符，'+
    '卡片上也不出积分榜列。显示名与内置占位符（时间/玩家/消息/服务器/在线人数/TPS）重名时，'+
    '保存后会自动改成「X(计分板)」，否则 ${X} 取到的永远是内置的那个值。</div>';
}

// 就地选中。取值只认 QB_PERMS 白名单 —— 参数来自 onclick 里的字符串，
// 与 qbWizCollect 从 data-value 读回来的是同一份口径。
function qbPickPerm(v){
  if(QB_PERMS.indexOf(v)<0) return;
  _qbWiz.perm=v;
  var box=document.getElementById('qb-perm-picker');
  if(!box) return;
  box.setAttribute('data-value',v);
  var bs=box.querySelectorAll('.qb-perm');
  for(var i=0;i<bs.length;i++) bs[i].classList.toggle('on',bs[i].getAttribute('data-value')===v);
}

// 组装补丁。**只带用户在这次向导里真正设过的字段**：POST 是窄合并，
// 少发一个字段就等于把用户原来那项留着不动；多发一个空值反而会把它清掉。
function qbWizPatch(){
  var p={enabled:true};   // 走完向导就是要用起来，直接开启
  if(_qbWiz.adapter==='qqofficial'){
    p.adapter='qqofficial';
    p.qqofficial={app_id:_qbWiz.app_id};
    // 空 secret = 保持已存的那个。绝不能回传空串——服务端正是按「空=保持不变」处理的，
    // 而若哪天口径变了，回传空串就等于每保存一次就静默清掉一次凭据。
    if(_qbWiz.app_secret) p.qqofficial.app_secret=_qbWiz.app_secret;
  }else{
    p.adapter='onebot';
    p.onebot={ws_url:_qbWiz.ws_url};
    if(_qbWiz.token) p.onebot.access_token=_qbWiz.token;
    p.group_id=parseInt(_qbWiz.group_id,10)||0;
  }
  // 只转发前缀（白名单）：留空 = 每条消息都转发。block_prefixes 是旧版的黑名单，
  // 向导不再提供这一项——**必须显式清空**，否则老配置里那份默认黑名单（. 和 。）
  // 会留在一个空的白名单旁边继续生效，「留空=转发每条消息」就成了空话。
  p.game_to_group={
    format:_qbWiz.format,
    only_prefixes:qbListSplit(_qbWiz.only_prefixes),
    block_prefixes:[],
    notify_join_leave:!!_qbWiz.notify_join,
    notify_death:!!_qbWiz.notify_death
  };
  p.command={permission:_qbWiz.perm,whitelist:qbListSplit(_qbWiz.whitelist)};
  // 计分板项：**每次都整表发**（不是窄合并）。这张表的语义是「以界面为准」——
  // 用户删掉一行就是要把那一项去掉，不发就等于删不掉。空数组是合法值
  // （= 不要任何计分板占位符与积分榜列），不是「保持不变」。
  // 只填了显示名的行在这里丢掉：后端同样会跳过（qqBoardDefs 跳过 Name 为空的行），
  // 留着它只会让界面显示一项实际不存在的配置。qbWizCheck 已经在离步时拦过一道。
  p.scoreboards=[];
  var sbs=_qbWiz.scoreboards||[];
  for(var i=0;i<sbs.length;i++){
    var n=String(sbs[i].name||'').trim();
    if(!n) continue;
    p.scoreboards.push({name:n,label:String(sbs[i].label||'').trim()});
  }
  // 没订阅就不发这一项：界面本来就把开关画成锁着的，不该顺手把用户在配置文件里
  // 写过的值改成 false（那正是「静默回写用户设置」）。
  if(qbTier().img) p.chat_image={enabled:!!_qbWiz.image};
  return p;
}

async function qbWizFinish(){
  if(_qbWizBusy) return;
  qbWizCollect();
  var btn=document.getElementById('qb-wz-next');
  if(btn){btn.disabled=true;btn.textContent='保存中…'}
  _qbWizBusy=true;
  var r=await A('POST','/api/qq/config',qbWizPatch());
  _qbWizBusy=false;
  if(!r||!r.ok){
    if(btn){btn.disabled=false;btn.textContent='完成'}
    T((r&&r.error)||'保存失败','e');
    return;
  }
  if(r.config) _qbConfig=r.config;
  qbCloseWizard();
  T('群服互通已保存，正在连接','o');
  paintQQPanel();
  qbRefreshStatus();
}
