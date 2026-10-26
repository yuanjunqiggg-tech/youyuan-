// 群服互通 · 卡片模板 —— Lua 卡片模板引擎的界面。
//
// 从「群服互通」面板（qq_bridge.js）进入的子页：挑模板 / 改源码 / 保存即热重载 /
// 恢复内置 / 复制 AI 提示词 / 实时预览。接口见 qq_template_http.go：
//   GET    /api/qq/templates            → {ok,templates:[{name,from_user,active_event,active_chat}],builtin:[]}
//   GET    /api/qq/template?name=X      → {ok,name,source,from_user}
//   POST   /api/qq/template             ← {name,source}，沙箱先跑一遍再落盘
//   DELETE /api/qq/template?name=X      → 幂等，删掉用户版本后内置版本接管
//   GET    /api/qq/ai-prompt            → {ok,prompt}
//   POST   /api/qq/template/preview     ← {name?,source?,kind?}，**成功回 PNG**
//   POST   /api/qq/config               ← 窄补丁，把某类卡的活动模板切过去
//
// 容器沿用 plugin_mgmt.js 里「数据目录文件编辑器」那一套（不是 migrate.js 的向导）：
// 进页面前 delete subPageCache 再 openSubPage，内容全部按接口现拉。理由与那边一样 ——
// 源码和预览图都是活数据，缓存一份上次的 DOM 只会让人对着旧内容改。
//
// 前端两步走：**源码只走 textarea.value / textContent，绝不 innerHTML**
// （它是用户自己敲的、服务器原样回显的文本）；生成串里的插值一律 escHtml。

// ── 状态 ──
// 子页每次打开都重建，这份状态只活在内存里，不落任何缓存。
var _qbTpl={list:[],builtin:[],cur:'',kind:'event',fromUser:false,prompt:'',
            pvSeq:0,pickSeq:0,pvUrl:'',busy:false};

// 预览用的 object URL 是手工管理的：换一张就 revoke 上一张，迟到的响应也要 revoke。
// 只留最后一个引用，页面上不需要别的副本。
function qbTplDropUrl(u){ if(u){try{URL.revokeObjectURL(u)}catch(e){}} }

// 离开子页时把预览图放掉：不禁的话每进出一次就漏一个 blob（子页 DOM 会被重建，
// 手里那个 URL 再也没有人能 revoke）。同时把 pvSeq 推一格，让在途的预览响应作废——
// 否则它回来时会往一个已经不存在的页面上写。
// 注册方式照 ai_assist.js 的 __subPageDetachHooks（子页清理的统一出口）。
window.__subPageDetachHooks=window.__subPageDetachHooks||[];
window.__subPageDetachHooks.push({key:'卡片模板',fn:function(){
  _qbTpl.pvSeq++;
  _qbTpl.pickSeq++;   // 在途的源码请求作废：子页已经没了，回来也不该往上写
  qbTplDropUrl(_qbTpl.pvUrl);
  _qbTpl.pvUrl='';
  _qbTpl.busy=false;
}});

// ═══ 打开 / 渲染 ═══

function qbOpenTemplates(){
  var title='卡片模板';
  qbTplDropUrl(_qbTpl.pvUrl);
  _qbTpl={list:[],builtin:[],cur:'',kind:'event',fromUser:false,prompt:'',
          pvSeq:0,pickSeq:0,pvUrl:'',busy:false};
  delete subPageCache[title];   // 每次进来都现拉，不用上一次的缓存（同 plgDataOpen）
  openSubPage(title,qbTplHTML());
  qbTplBind();
  qbTplLoad();
}

function qbTplHTML(){
  return ''+
  '<div class="qb-tpl-kinds" id="qb-tpl-kinds">'+
    '<button type="button" class="qb-perm on" data-kind="event">事件卡</button>'+
    '<button type="button" class="qb-perm" data-kind="chat">聊天卡</button>'+
  '</div>'+
  '<div class="sr-note" style="margin:8px 0 0">事件卡是进服 / 退服 / 死亡播报用的宽卡（720 像素），'+
  '聊天卡是群里发言用的窄卡（360 像素）。两边的模板各选各的，上面这个开关决定下面所有操作作用在哪一边。</div>'+

  '<div class="sr-panel"><div class="sr-chips">'+
    '<div class="sr-chips-t">模板</div>'+
    '<div id="qb-tpl-list"><div class="sr-loading">加载中…</div></div>'+
    '<div class="qb-acts">'+
      '<button class="sr-mini" id="qb-tpl-use" onclick="qbTplUse()">设为当前模板</button>'+
      '<button class="sr-mini" onclick="qbTplNew()">新建模板</button>'+
    '</div>'+
  '</div></div>'+

  '<div class="sr-panel"><div class="sr-chips">'+
    '<div class="sr-chips-t">源码</div>'+
    '<div class="code-wrap"><pre class="code-pre" id="qb-tpl-pre" aria-hidden="true"></pre>'+
    '<textarea class="code-input" id="qb-tpl-ta" spellcheck="false" aria-label="模板源码"></textarea></div>'+
    '<div class="qb-tpl-msg" id="qb-tpl-msg"></div>'+
    '<div class="qb-acts qb-tpl-btns">'+
      '<button class="sr-mini ok" id="qb-tpl-save" onclick="qbTplSave()">保存并启用</button>'+
      '<button class="sr-mini" onclick="qbTplPreviewDraft()">预览草稿</button>'+
      '<button class="sr-mini" onclick="qbTplRevert()">恢复内置</button>'+
    '</div>'+
    '<div class="qb-hint">保存前服务器会先把这份代码跑一遍：跑不过就<b>只报错、不落盘</b>，'+
    '群里用的还是上一份好模板。存下去立刻生效，不用重启也不用重连。</div>'+
  '</div></div>'+

  '<div class="sr-panel"><div class="sr-chips">'+
    '<div class="sr-chips-t">实时预览</div>'+
    '<img id="qb-tpl-img" class="qb-tpl-img" alt="卡片预览图" style="display:none">'+
    '<div class="qb-tpl-msg" id="qb-tpl-pv-err"></div>'+
    '<div class="qb-hint">预览用的是固定的一组样例数据（玩家 Steve / Alex / Herobrine、示例群消息），'+
    '同一份模板每次出来的图都一样。</div>'+
  '</div></div>'+

  '<div class="sr-panel"><div class="sr-chips">'+
    '<div class="sr-chips-t">用网页 AI 生成模板</div>'+
    '<div class="qb-hint" style="margin:0 0 8px">复制下面的提示词 → 粘到网页 AI → '+
    '<b>在末尾补上你自己的要求</b> → 把 AI 写出来的代码贴回上面的编辑器 → 点「保存并启用」。'+
    '提示词的最后一行就是留给你写要求的位置。</div>'+
    '<pre class="qb-prompt" id="qb-tpl-prompt">加载中…</pre>'+
    '<div class="qb-acts"><button class="sr-mini" id="qb-tpl-copy" onclick="qbTplCopyPrompt()">复制提示词</button></div>'+
  '</div></div>';
}

// 事件委托挂在容器上（容器不进缓存、每次打开都是新的，所以只绑一次）。
// **不用内联 onclick 传模板名**：模板名只做了文件名级校验（safeName 只挡斜杠与控制字符），
// 引号是合法的，拼进 onclick 属性里就是一个注入口子。
function qbTplBind(){
  var box=document.getElementById('qb-tpl-list');
  if(box) box.addEventListener('click',function(e){
    var row=e.target&&e.target.closest?e.target.closest('.qb-tpl-row'):null;
    if(!row) return;
    var n=row.getAttribute('data-name');
    if(n) qbTplPick(n);
  });
  var ks=document.getElementById('qb-tpl-kinds');
  if(ks) ks.addEventListener('click',function(e){
    var b=e.target&&e.target.closest?e.target.closest('.qb-perm'):null;
    if(!b) return;
    qbTplSetKind(b.getAttribute('data-kind'));
  });
  qbTplPaint();   // 首次即完成绑定；输入与滚动都由共用的 codeEditorAttach 接管
}

// ═══ 数据 ═══

async function qbTplLoad(){
  var r=await A('GET','/api/qq/templates');
  if(!document.getElementById('qb-tpl-list')) return;   // 等待期间子页已被关掉
  if(!r||!r.ok){ qbTplMsg((r&&r.error)||'读取模板列表失败','e'); return; }
  _qbTpl.list=r.templates||[];
  _qbTpl.builtin=r.builtin||[];
  qbTplRenderList();
  // 默认选中当前类型正在用的那个；没有就选列表第一个。
  var active=qbTplActiveFor(_qbTpl.kind);
  if(!active&&_qbTpl.list.length) active=_qbTpl.list[0];
  if(active) qbTplPick(active.name);
  qbTplLoadPrompt();
}

// 只刷列表（不动编辑器）。存过 / 切过之后调它，省一次源码往返。
async function qbTplRefreshList(){
  var r=await A('GET','/api/qq/templates');
  if(!r||!r.ok||!document.getElementById('qb-tpl-list')) return;
  _qbTpl.list=r.templates||[];
  _qbTpl.builtin=r.builtin||[];
  var t=qbTplFind(_qbTpl.cur);
  if(t) _qbTpl.fromUser=!!t.from_user;
  qbTplRenderList();
  qbTplRenderCur();
}

function qbTplFind(name){
  for(var i=0;i<_qbTpl.list.length;i++) if(_qbTpl.list[i].name===name) return _qbTpl.list[i];
  return null;
}

// ── 模板的类型（决定它出现在哪个页签）──
//
// 内置模板由服务器标类型（列表接口的 kind 字段）。用户自己建的模板类型只记在
// **本机**：清缓存、换设备之后记录就没了，那时它在两个页签里都显示——宁可多显示
// 一个，也不能把谁自己写的模板藏起来。
var QB_TPL_KINDS_KEY='qbTplKinds';

function qbTplUserKinds(){
  try{ return JSON.parse(localStorage.getItem(QB_TPL_KINDS_KEY)||'{}')||{} }catch(e){ return {} }
}

function qbTplRememberKind(name,kind){
  try{
    var m=qbTplUserKinds();
    m[name]=kind;
    localStorage.setItem(QB_TPL_KINDS_KEY,JSON.stringify(m));
  }catch(e){}   // 隐私模式下 localStorage 会抛：记不住就算了，只是少一层分类
}

// 这个模板属于哪一类：'event' / 'chat' / ''（不知道）。
function qbTplKindOf(t){
  if(!t) return '';
  if(t.kind) return t.kind;                    // 内置：服务器说了算
  return qbTplUserKinds()[t.name]||'';
}

// 这个模板在当前页签里该不该出现。类型不明的一律显示。
function qbTplVisibleIn(t,kind){
  var k=qbTplKindOf(t);
  return k===''||k===kind;
}

// qbTplActiveFor 某类卡片当前启用的模板条目，没有活动的就返回 null。
//
// 事件卡与聊天卡各选各的，切类型时靠它把选中项重定位到这一边正在用的模板。
// 不重定位的话编辑器里留的是另一类的源码，点「设为当前模板」就把模板写到了
// 另一边——用户看到的「两边的预设串了」就是这么来的。
function qbTplActiveFor(kind){
  for(var i=0;i<_qbTpl.list.length;i++){
    var t=_qbTpl.list[i];
    if(kind==='chat'?t.active_chat:t.active_event) return t;
  }
  return null;
}

function qbTplRenderList(){
  var box=document.getElementById('qb-tpl-list');
  if(!box) return;
  // 只列**当前这一类**的模板：宽卡模板混在聊天卡列表里，用户选中一次就会得到
  // 一张尺寸对不上的图（事件卡 720 宽、聊天卡 360 宽）。
  var shown=[];
  for(var i=0;i<_qbTpl.list.length;i++){
    if(qbTplVisibleIn(_qbTpl.list[i],_qbTpl.kind)) shown.push(_qbTpl.list[i]);
  }
  var h='';
  if(!shown.length){
    h='<div class="sr-empty">这一类还没有模板</div>';
  }
  for(var j=0;j<shown.length;j++){
    var t=shown[j];
    h+='<button type="button" class="qb-tpl-row'+(t.name===_qbTpl.cur?' on':'')+'" data-name="'+escHtml(t.name)+'">'+
      '<span class="qb-tpl-n">'+escHtml(t.name)+'</span>'+
      // 「内置」= 用的就是随工具箱发的源码；「已改」= 磁盘上有用户版本（from_user）。
      '<span class="qb-tpl-b'+(t.from_user?' mod':'')+'">'+(t.from_user?'已改':'内置')+'</span>'+
      (t.active_event?'<span class="qb-tpl-b act">事件卡</span>':'')+
      (t.active_chat?'<span class="qb-tpl-b act">聊天卡</span>':'')+
    '</button>';
  }
  if(box.innerHTML!==h) box.innerHTML=h;   // 没变就不写 DOM
}

// 「设为当前」按钮的文案与可用性。**不用 disabled 属性拼生成串**（布尔属性写成 ""，
// 与 DOM 序列化回来的值对不上），这里走 property 赋值。
function qbTplRenderCur(){
  var btn=document.getElementById('qb-tpl-use');
  if(!btn) return;
  var kt=_qbTpl.kind==='chat'?'聊天卡':'事件卡';
  var t=qbTplFind(_qbTpl.cur);
  var act=t?(_qbTpl.kind==='chat'?t.active_chat:t.active_event):false;
  btn.textContent=act?('已用于'+kt):('把「'+(_qbTpl.cur||'—')+'」设为'+kt);
  btn.disabled=!!act||!_qbTpl.cur;
}

// 选中一个模板：拉源码填进编辑器，再出一张预览。
async function qbTplPick(name){
  // 竞态：快速连点类型开关或模板行会有两个 GET 在飞，晚回来的那个会把编辑器
  // 写成另一类的源码——「预设串了」的另一条触发路径。领个序号，不是最新的就丢。
  var seq=++_qbTpl.pickSeq;
  var r=await A('GET','/api/qq/template?name='+encodeURIComponent(name));
  if(seq!==_qbTpl.pickSeq) return;   // 期间又选了别的 / 切了类型 / 退了子页
  if(!document.getElementById('qb-tpl-ta')) return;
  if(!r||!r.ok){ qbTplMsg((r&&r.error)||'读取模板失败','e'); return; }
  _qbTpl.cur=r.name||name;
  _qbTpl.fromUser=!!r.from_user;
  var ta=document.getElementById('qb-tpl-ta');
  ta.value=r.source||'';    // 源码是文本，只进 value
  qbTplPaint();
  qbTplMsg('','');
  qbTplRenderList();
  qbTplRenderCur();
  qbTplPreview({name:_qbTpl.cur,kind:_qbTpl.kind});
}

// 高亮层。highlightCode / codeEditorAttach 来自 plugin_mgmt.js（Lua 高亮器与编辑器绑定，
// 全项目共用一个），这里只借它和 .code-wrap/.code-pre/.code-input 那套样式 —— 插件编辑器
// 本体（window.__plgEdit、/api/plugin/file/write、页签与搜索）绑得太死，搬不过来。
// 首次调用完成绑定并渲染；之后只请求重画（视口内高亮，见 plugin_mgmt.js）。
function qbTplPaint(){
  var ta=document.getElementById('qb-tpl-ta'),pre=document.getElementById('qb-tpl-pre');
  if(!ta||!pre) return;
  if(!ta.__ceBound) codeEditorAttach(ta,pre,'lua');
  else if(ta.__cePaint) ta.__cePaint();
}

// 编辑器下面那行提示。t: '' | 'w' | 'e' | 'o'
function qbTplMsg(m,t){
  var el=document.getElementById('qb-tpl-msg');
  if(!el) return;
  el.className='qb-tpl-msg'+(t&&t!=='w'?' '+t:'');
  el.textContent=m||'';
}

// ═══ 卡片类型 ═══

function qbTplSetKind(k){
  if(k!=='event'&&k!=='chat') return;
  if(_qbTpl.busy||k===_qbTpl.kind) return;   // 重复点同一个开关不必重拉一次源码和预览
  _qbTpl.kind=k;
  var bs=document.querySelectorAll('#qb-tpl-kinds .qb-perm');
  for(var i=0;i<bs.length;i++) bs[i].classList.toggle('on',bs[i].getAttribute('data-kind')===k);
  // 选中项跟着类型走：切过去就载入这一边正在用的模板（源码 + 预览都由 pick 负责）。
  // 这一类还没有活动模板时保持当前选中项重载一次——按钮会如实显示「把 X 设为
  // 聊天卡」，用户点下去是明确意图，不再是把另一边的模板误写过来。
  var active=qbTplActiveFor(k);
  var next=active?active.name:_qbTpl.cur;
  if(next) qbTplPick(next);
  else { qbTplRenderList(); qbTplRenderCur(); }
}

// ═══ 新建模板 ═══

// 新模板的起始代码：一份能直接跑通的最小骨架。
// 多数用户是把代码丢给网页 AI 改的，骨架里把可用字段列全，省得对着空白编辑器发呆。
function qbTplSkeleton(kind){
  if(kind==='chat'){
    return [
      '-- 聊天卡：群里有人发言时出的窄卡（360 宽）。',
      '-- ctx 可用字段：',
      '--   ctx.player 玩家名 / ctx.nick 昵称 / ctx.message 消息内容 / ctx.time 时间',
      '--   ctx.skin 头像图 / ctx.width 画布宽',
      '--   ctx.scoreboards、ctx.columns、ctx.scores：积分榜数据（见下面的 AI 提示词）',
      'function render(ctx)',
      '  local W = ctx.width',
      '  local img = draw.new(W, 140)',
      '  draw.fillRect(img, 0, 0, W, 140, "#ece6da")',
      '  draw.text(img, ctx.nick ~= "" and ctx.nick or ctx.player, 20, 40, 24, "#111111", "bold")',
      '  draw.textWrap(img, ctx.message, 20, 80, W - 40, 20, "#3a3a3a", "reg")',
      '  return img',
      'end'
    ].join('\n');
  }
  return [
    '-- 事件卡：玩家进服 / 退服 / 死亡时出的宽卡（720 宽）。',
    '-- ctx 可用字段：',
    '--   ctx.event 事件名（join / leave / death）/ ctx.player 玩家名 / ctx.time 时间',
    '--   ctx.onlineText 在线玩家（顿号拼接）/ ctx.onlineCount 在线人数 / ctx.online 名字数组',
    '--   ctx.server 服务器号 / ctx.tps / ctx.skin 头像图',
    '--   ctx.scoreboards、ctx.columns、ctx.scores：积分榜数据（见下面的 AI 提示词）',
    'function render(ctx)',
    '  local W = ctx.width',
    '  local img = draw.new(W, 260)',
    '  draw.fillRect(img, 0, 0, W, 260, "#ece6da")',
    '  local t = "玩家加入"',
    '  if ctx.event == "leave" then t = "玩家退出" end',
    '  if ctx.event == "death" then t = "玩家死亡" end',
    '  draw.text(img, t, 24, 60, 40, "#111111", "bold")',
    '  draw.text(img, ctx.player, 24, 120, 30, "#ff3d00", "bold")',
    '  draw.textWrap(img, "在线：" .. ctx.onlineText, 24, 180, W - 48, 20, "#3a3a3a", "reg")',
    '  return img',
    'end'
  ].join('\n');
}

// qbTplNew 弹窗命名后新建一个模板。
//
// 重名要先拦下来：保存是**覆盖写**——用户对着一个内置模板点了新建、写完保存，
// 就把内置那份改掉了（「恢复内置」能救回来，但没必要先掉进去）。
function qbTplNew(){
  if(_qbTpl.busy) return;
  var kt=_qbTpl.kind==='chat'?'聊天卡':'事件卡';
  showPrompt({
    title:'新建'+kt+'模板',
    message:'起个名字（不要用斜杠）。建好后在这里写代码，保存即生效。',
    placeholder:'例如 my_card',
    confirmText:'创建',
    onConfirm:function(v){ qbTplCreate(v) }
  });
}

async function qbTplCreate(raw){
  var name=(raw||'').trim();
  if(!name){ T('名字不能为空','e'); return }
  // 只挡明显非法的（斜杠是路径分隔符），其余交给服务器把关——它才是权威。
  if(name.indexOf('/')>=0||name.indexOf('\\')>=0){ T('名字里不能用斜杠','e'); return }
  if(qbTplFind(name)){ T('已经有叫「'+name+'」的模板了，换个名字','e'); return }

  var kind=_qbTpl.kind;
  var r=await A('POST','/api/qq/template',{name:name,source:qbTplSkeleton(kind),kind:kind});
  if(!r||!r.ok){ T((r&&r.error)||'创建失败','e'); return }
  // 类型只记在本机，它决定这个模板出现在哪个页签（见 qbTplKindOf）。
  qbTplRememberKind(name,kind);
  await qbTplRefreshList();
  qbTplPick(name);
  qbTplMsg('已创建，改完点「保存并启用」','o');
}

// ═══ 切换活动模板 / 保存 / 恢复 ═══

// 把当前选中的模板设为这一类的活动模板。窄补丁：只动 template 一项，
// 不碰 enabled / width / show_scoreboard。
async function qbTplUse(){
  if(_qbTpl.busy||!_qbTpl.cur) return;
  var kt=_qbTpl.kind==='chat'?'聊天卡':'事件卡';
  _qbTpl.busy=true;
  var p={};
  p[_qbTpl.kind==='chat'?'chat_image':'event_image']={template:_qbTpl.cur};
  var r=await A('POST','/api/qq/config',p);
  _qbTpl.busy=false;
  if(!r||!r.ok){ T((r&&r.error)||'切换失败','e'); return; }
  T('「'+_qbTpl.cur+'」已成为'+kt+'模板','o');
  await qbTplRefreshList();
  qbTplPreview({name:_qbTpl.cur,kind:_qbTpl.kind});
}

async function qbTplSave(){
  var ta=document.getElementById('qb-tpl-ta');
  if(!ta||_qbTpl.busy) return;
  if(!_qbTpl.cur){ qbTplMsg('请先在上面选一个模板','e'); return; }
  var src=ta.value;
  // 类型与模板名在**发出请求前**定下来：下面两处 await 之后才用到它们，
  // 期间用户切了类型/换了模板的话，补丁就会打到另一边去（同一个串用）。
  var k=_qbTpl.kind;
  var nm=_qbTpl.cur;
  var kt=k==='chat'?'聊天卡':'事件卡';
  _qbTpl.busy=true;
  qbTplMsg('正在保存…','w');
  var r=await A('POST','/api/qq/template',{name:nm,source:src,kind:k});
  _qbTpl.busy=false;
  if(!r||!r.ok){
    // 失败时**编辑器一个字都不动** —— 服务器也没落盘，群里用的还是上一份好模板。
    // 错误是沙箱自己的原话（哪一行、什么错），照原样显示，用户才能改。
    qbTplMsg((r&&r.error)||'保存失败','e');
    return;
  }
  // 保存即启用：同一个名字切成本类的活动模板。
  var p={};
  p[k==='chat'?'chat_image':'event_image']={template:nm};
  var r2=await A('POST','/api/qq/config',p);
  if(!r2||!r2.ok){
    qbTplMsg('模板已存下，但设为'+kt+'模板失败：'+((r2&&r2.error)||''),'e');
  }else{
    // 热重载：后端按 mtime+size 重读盘，不用重启桥接、不用重连。
    qbTplMsg('已保存，'+kt+'立刻用上这份代码（无需重启或重连）','o');
  }
  await qbTplRefreshList();
  qbTplPreview({name:nm,kind:k});
}

// 恢复内置：删掉用户版本。只有真的有用户版本（已改）时才多问一句 —— 没有东西可丢
// 的时候弹确认框，用户点完「确定」什么都没发生，比不弹更让人困惑。
function qbTplRevert(){
  if(_qbTpl.busy||!_qbTpl.cur) return;
  var name=_qbTpl.cur;
  if(!_qbTpl.fromUser){ qbTplDoRevert(name); return; }
  showConfirm({
    title:'恢复内置模板？',
    // name 只过了文件名级校验，可以带引号甚至尖括号，进 innerHTML 前必须转义。
    message:'会删掉你保存的「'+escHtml(name)+'」，回到随工具箱发布的内置版本。\n'+
            '编辑器里的内容也会换成内置源码，改不回来。',
    confirmText:'恢复内置',
    onConfirm:function(){ qbTplDoRevert(name) }
  });
}

async function qbTplDoRevert(name){
  var r=await A('DELETE','/api/qq/template?name='+encodeURIComponent(name));
  if(!r||!r.ok){ T((r&&r.error)||'恢复失败','e'); return; }
  T('已恢复内置模板','o');
  await qbTplRefreshList();
  if(_qbTpl.cur===name) qbTplPick(name);   // 重新拉一遍：现在回来的是内置源码
}

// ═══ 预览 ═══

// 预览「已保存的模板」（服务器走与线上完全相同的兜底阶梯）。
function qbTplPreviewSaved(){
  if(_qbTpl.cur) qbTplPreview({name:_qbTpl.cur,kind:_qbTpl.kind});
}

// 预览编辑器里的**草稿**（没保存的源码）。服务器对草稿不做兜底：编译或渲染失败
// 就把错误原文回给编辑器。
function qbTplPreviewDraft(){
  var ta=document.getElementById('qb-tpl-ta');
  if(!ta) return;
  if(!ta.value.trim()){ qbTplMsg('编辑器是空的，没有可预览的代码','e'); return; }
  qbTplPreview({source:ta.value,kind:_qbTpl.kind});
}

// 发一次预览并渲染结果。
//
// 竞态：连点「预览草稿」或连着换模板时会有多个请求在飞，回来的顺序不保证。
// 每次请求领一个序号，回来时序号不是最新的就整份丢弃（连 blob 一起 revoke）——
// 否则一张旧图会盖掉刚出来的新图，用户会以为自己刚改的代码没生效。
async function qbTplPreview(body){
  var seq=++_qbTpl.pvSeq;
  var err=document.getElementById('qb-tpl-pv-err');
  if(err){ err.className='qb-tpl-msg'; err.textContent='渲染中…' }
  var res=await qbTplPreviewReq(body);
  if(seq!==_qbTpl.pvSeq){ qbTplDropUrl(res.url); return }   // 迟到的响应
  var img=document.getElementById('qb-tpl-img');
  if(!img){ qbTplDropUrl(res.url); return }
  if(!res.ok){
    // 草稿编译不过：图撤掉，只留错误文字（旧的图留着会让人以为这就是新结果）。
    img.style.display='none';
    img.removeAttribute('src');
    qbTplDropUrl(_qbTpl.pvUrl);
    _qbTpl.pvUrl='';
    if(err){ err.className='qb-tpl-msg e'; err.textContent=res.error||'预览失败' }
    return;
  }
  qbTplDropUrl(_qbTpl.pvUrl);       // 换新图前先放掉上一张
  _qbTpl.pvUrl=res.url;
  img.src=res.url;
  img.style.display='block';
  if(err){
    err.className='qb-tpl-msg'+(res.warn?' w':'');
    // 存过的模板渲染失败时，图是 Go 兜底卡片，不是用户的设计 —— 必须说清楚，
    // 否则用户看着一张能出来的图，以为自己的模板没问题。
    err.textContent=res.warn?('模板渲染失败，上面这张是兜底卡片：'+res.warn):'';
  }
}

// 预览请求。**不能走 A()**：那条路径只认 JSON，而预览成功时回来的是 PNG。
// 先看 Content-Type 再决定怎么读 —— 草稿不过时返回的是 JSON 错误体。
async function qbTplPreviewReq(body){
  try{
    var r=await fetch('/api/qq/template/preview',{
      method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    var ct='';
    try{ ct=(r.headers&&r.headers.get)?(r.headers.get('Content-Type')||''):'' }catch(e){}
    var warn=qbTplHdrErr(r);
    if(ct.indexOf('image/png')<0){
      var j=null;
      try{ j=await r.json() }catch(e){}
      return {ok:false,error:(j&&j.error)||'预览失败'};
    }
    var blob=await r.blob();
    return {ok:true,url:URL.createObjectURL(blob),warn:warn};
  }catch(e){
    return {ok:false,error:(e&&e.message)||'预览失败'};
  }
}

// X-QQ-Template-Error 是 url.QueryEscape 编出来的，空格被写成 '+'，
// decodeURIComponent 不认这个，得先换回来再解。
function qbTplHdrErr(r){
  var v=null;
  try{ v=(r.headers&&r.headers.get)?r.headers.get('X-QQ-Template-Error'):null }catch(e){}
  if(!v) return '';
  try{ return decodeURIComponent(String(v).replace(/\+/g,' ')) }catch(e){ return String(v) }
}

// ═══ AI 提示词 ═══

async function qbTplLoadPrompt(){
  var el=document.getElementById('qb-tpl-prompt');
  if(!el||_qbTpl.prompt) return;
  var r=await A('GET','/api/qq/ai-prompt');
  el=document.getElementById('qb-tpl-prompt');
  if(!el) return;
  if(!r||!r.ok){ el.textContent='读取提示词失败：'+((r&&r.error)||''); return; }
  _qbTpl.prompt=r.prompt||'';
  el.textContent=_qbTpl.prompt;   // 服务器原文，只进 textContent
}

function qbTplCopyPrompt(){
  var txt=_qbTpl.prompt||'';
  if(!txt){ T('提示词还没加载出来','w'); return }
  var done=function(){
    T('提示词已复制','o');
    var b=document.getElementById('qb-tpl-copy');
    if(b){ b.textContent='已复制'; setTimeout(function(){ if(b.isConnected) b.textContent='复制提示词' },1500) }
  };
  // 剪贴板 API 在 WebView / 非安全上下文里可能没有，退回 textarea + execCommand。
  if(navigator.clipboard&&navigator.clipboard.writeText){
    navigator.clipboard.writeText(txt).then(done,function(){ qbTplCopyFallback(txt,done) });
    return;
  }
  qbTplCopyFallback(txt,done);
}

function qbTplCopyFallback(txt,done){
  var ta=document.createElement('textarea');
  ta.value=txt;
  ta.setAttribute('readonly','');
  ta.style.position='fixed';
  ta.style.left='-9999px';
  document.body.appendChild(ta);
  ta.select();
  var ok=false;
  try{ ok=document.execCommand('copy') }catch(e){}
  document.body.removeChild(ta);
  if(ok) done(); else T('复制失败，请长按选中提示词手动复制','e');
}
