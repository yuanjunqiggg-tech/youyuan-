var curTab='home',tabInited={},tabHistory=['home'];
var TAB_ORDER=['home','tasks','terminal','settings'];
var activeRenderers=[];
var _savedFormState={};
var _voxelData={};

var cfgSpeed=9500,cfgNewProtocol=false,cfgImportCommands=true,cfgCmdDisabled=false,cfgExcludeFluids=false,cfgExcludeWater=true,cfgExcludeWaterlogged=true,cfgExcludeLava=true,cfgGravity=false,cfgPlatform=false;
var cfgToken='';
var cfgRegionMode='1';
var cfgBotTokens=[];
var subPageOpen=false;
var subPageCache={};
var subPageStack=[];   // 子页返回栈：关闭时回到上一级子页而非主页
var yySubPageEpoch=0;
function yyRestoreSubPageLater(key, epoch){
  function live(){return epoch===yySubPageEpoch&&subPageOpen&&subPageStack[subPageStack.length-1]===key}
  setTimeout(function(){if(live())restoreSubPageState(key)},20);
  setTimeout(function(){if(live())runSubPageRestore(key)},60);
}
// 子页恢复钩子(key 或 '*' 通配),供各功能页在从缓存恢复时重建动态内容(无限滚动哨兵/滚动位置等)
// 用 window 级数组注册,规避 app.js 最后加载导致的顺序问题
function runSubPageRestore(key){
  (window.__subPageRestoreHooks||[]).forEach(function(h){
    if(h.key==='*'||h.key===key){ try{h.fn(key)}catch(e){} }
  });
  if(window.onSubPageRestored){ try{window.onSubPageRestored(key)}catch(e){} }
}

// Preload config
var cfgPreClear=0;(async function preloadCfg(){try{var c=await A('GET','/api/config');if(c.ok&&c.config){cfgToken=c.config.token||'';cfgSpeed=c.config.import_speed||9500;cfgNewProtocol=c.config.use_new_protocol||false;cfgImportCommands=c.config.import_commands!==false;cfgCmdDisabled=c.config.cmd_disabled||false;cfgExcludeFluids=c.config.exclude_fluids||false;cfgExcludeWater=c.config.exclude_water!==false;cfgExcludeWaterlogged=c.config.exclude_waterlogged!==false;cfgExcludeLava=c.config.exclude_lava!==false;cfgGravity=c.config.use_gravity_blocks||false;cfgPlatform=c.config.gravity_platform||false;cfgPreClear=c.config.pre_clear_mode||0;cfgRegionMode=c.config.region_mode||'1';cfgBotTokens=c.config.bot_tokens||[]}}catch(e){}})();
// 启动时自动检查版本更新
setTimeout(function(){A('POST','/api/version/check',{}).then(function(r){if(r.ok)handleVersionCheck(r)}).catch(function(){})},3000);
// 启动时自动检查版本更新

function openSubPage(title, htmlContent){
  if(typeof yyNavigateEpoch==='number')yyNavigateEpoch++;
  var epoch=++yySubPageEpoch;
  var cacheKey = title;
  activeRenderers.forEach(function(kill){try{kill()}catch(e){}});activeRenderers=[];
  var sp = document.getElementById('sub-page');
  document.body.classList.add('sp-open');   // 子页面打开时隐藏主页内容，避免透明背景透出主页
  document.getElementById('bottom-nav').classList.add('gone');
  document.getElementById('conn-bar').classList.add('gone');

  if (subPageCache[cacheKey]) {
    sp.innerHTML = subPageCache[cacheKey];
    // defer restore to run after feature init timeouts (10ms) to avoid override
    yyRestoreSubPageLater(cacheKey,epoch);
    // 画布等位图不在缓存里，恢复后重渲染（画板等）
  } else {
    var headerHtml = '<div class="sp-header"><button class="sp-back" onclick="closeSubPage()">' + ICONS.arrowLeft + '</button><span class="sp-title">' + title + '</span></div>';
    subPageCache[cacheKey] = headerHtml + '<div class="sp-body">' + htmlContent + '</div>';
    sp.innerHTML = subPageCache[cacheKey];
  }

  sp.className = 'sub-page show';
  subPageOpen = true;
  // 压入返回栈（同一页在栈顶则不重复压）
  if (subPageStack[subPageStack.length-1] !== cacheKey) subPageStack.push(cacheKey);
}

function closeSubPage(){
  var epoch=++yySubPageEpoch;
  if(typeof yyWorkspaceOpenEpoch!=='undefined')yyWorkspaceOpenEpoch++;
  document.getElementById('yy-agent-profile-modal')?.remove();
  activeRenderers.forEach(function(kill){try{kill()}catch(e){}});activeRenderers=[];
  try {
  // save form state + update cached HTML with current DOM state
  var titleEl=document.querySelector('.sp-title');
  if(titleEl){
    var key=titleEl.textContent;
    var sp=document.getElementById('sub-page');
    var header=sp.querySelector('.sp-header');
    var body=document.querySelector('#sub-page .sp-body');
    if(body){
      // save form values
      _savedFormState[key]=serializeForm(body);
      // update cached HTML with current DOM (preserve file selection, stats, etc.)
      var clone=body.cloneNode(true);
      // remove 3D viewer + canvases from cache (recreated from _voxelData on reopen)
      var vw=clone.querySelector('#viewer');
      if(vw)vw.remove();
      clone.querySelectorAll('canvas').forEach(function(c){c.remove()});
      if(header)subPageCache[key]=header.outerHTML+clone.outerHTML
    }
  }
  } catch(error) { console.warn('游园：保存页面状态失败，仍允许返回',error); }
  // 返回栈：关闭后回到上一级子页（如有），否则回主页
  if (subPageStack.length > 0) subPageStack.pop();
  if (subPageStack.length > 0) {
    var prevKey = subPageStack[subPageStack.length-1];
    var sp2 = document.getElementById('sub-page');
    if (subPageCache[prevKey]) {
      sp2.innerHTML = subPageCache[prevKey];
      sp2.className = 'sub-page show';
      yyRestoreSubPageLater(prevKey,epoch);
      return;
    }
    subPageStack = [];
  }
  var sp=document.getElementById('sub-page');
  sp.classList.remove('show');sp.classList.add('sp-closing');
  setTimeout(function(){
    // A new subpage may have opened while the previous close animation ran.
    if(epoch!==yySubPageEpoch||sp.classList.contains('show'))return;
    sp.className='sub-page';
    document.body.classList.remove('sp-open');   // 子页面关闭后恢复主页内容
    document.getElementById('bottom-nav').classList.remove('gone');
    document.getElementById('conn-bar').classList.remove('gone');
    subPageOpen=false;
    // keep cache, kill renderers (performance)
    activeRenderers.forEach(function(kill){try{kill()}catch(e){}});activeRenderers=[];
    // evict stale cache entries (keep last 10)
    var keys=Object.keys(subPageCache);
    if(keys.length>10){
      var keep=keys.slice(-10);
      Object.keys(subPageCache).forEach(function(k){
        if(keep.indexOf(k)<0)delete subPageCache[k]
      });
      Object.keys(_savedFormState).forEach(function(k){
        if(keep.indexOf(k)<0)delete _savedFormState[k]
      });
      Object.keys(_voxelData).forEach(function(k){
        if(keep.indexOf(k)<0)delete _voxelData[k]
      })
    }
    if(curTab==='home')refreshHomeTasks();
  },290);
}

function serializeForm(root){
  var st={};
  root.querySelectorAll('input,select,textarea').forEach(function(el){
    if(!el.id)return;
    if(el.type==='checkbox')st[el.id]=el.checked;
    else st[el.id]=el.value;
  });
  return st
}

function restoreSubPageState(key){
  // restore form values
  var sv=_savedFormState[key];
  if(sv){
    Object.keys(sv).forEach(function(id){
      var el=document.getElementById(id);
      if(!el)return;
      if(el.type==='checkbox')el.checked=sv[id];
      else el.value=sv[id]
    })
    // re-run conditional toggles
    try{
      toggleSub('imp-cmds','imp-cmd-sub');
      toggleSub('imp-exclude-fluids','imp-fluid-sub');
      toggleDimInput();
      toggleGravitySub();
      toggleMaRelief()
    }catch(e){}
  }
  // recreate 3D preview if voxel data exists
  var vd=_voxelData[key];
  if(vd&&typeof show3D==='function'){
    var vw=document.getElementById('viewer');
    // if viewer was removed from cached HTML on close, re-create it
    if(!vw){
      var info=document.getElementById('imp-info')||document.getElementById('ma-info')||document.getElementById('skin-preview-area');
      if(info){
        vw=document.createElement('div');vw.className='viewer';vw.id='viewer';
        info.appendChild(vw)
      }
    }
    if(vw){
      vw.innerHTML='';
      requestAnimationFrame(function(){
        if(typeof THREE!=='undefined')show3D(vw,vd)
      })
    }
  }
  // restore task-running UI if a task is active
  if(window._currentTask){
    var ct=window._currentTask;
    var t2p={import:'imp',export:'exp',mapart:'md',skin:'skin'};
    var pr=t2p[ct.type];
    if(pr){
      var go=document.getElementById(pr+'-go');
      var stop=document.getElementById(pr+'-stop');
      var pg=document.getElementById(pr+'-pg');
      var lc=document.getElementById(pr+'-log-card');
      if(go)go.classList.add('gone');
      if(stop)stop.classList.remove('gone');
      if(pg)pg.classList.remove('gone');
      if(lc)lc.classList.remove('gone');
      // restore progress bar if known
      if(ct.progress>0){
        var bar=document.getElementById(pr+'-bar');
        if(bar)bar.style.width=(ct.progress*100)+'%'
      }
    }
  }
  // restore marquee state if on marquee page
  if(document.getElementById('mq-text')&&typeof restoreMarqueeState==='function'){
    restoreMarqueeState()
  }
  // restore mcfunction state if on mcfunction page
  if(document.getElementById('mcfn-wrap')&&typeof restoreMcfunctionState==='function'){
    restoreMcfunctionState()
  }
}

function refreshHomeTasks(){
  var section=document.querySelector('#tab-home .tsk-section');
  if(!section)return;
  A('GET','/api/task/list').then(function(t){
    if(!t.ok||!t.tasks)return;
    var h='<div class="tsk-header">任务列表</div>';
    for(var ti=0;ti<t.tasks.length;ti++){
      var tk=t.tasks[ti];
      var tIcon={'import':'<svg width="28" height="28" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>','export':'<svg width="28" height="28" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/><rect x="3" y="3" width="16" height="6" rx="1"/></svg>','mapart':'<svg width="28" height="28" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="16" height="16" rx="2"/><line x1="3" y1="9" x2="19" y2="9"/><line x1="9" y1="3" x2="9" y2="19"/></svg>','skin':'<svg width="28" height="28" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>'}[tk.type]||'·';
      var tSt=tk.status||'unknown';
      var tCl=tSt==='running'?'running':tSt==='done'?'done':tSt==='failed'?'failed':'paused';
      var tLb=tSt==='running'?'运行中':tSt==='done'?'完成':tSt==='failed'?'失败':'暂停';
      var tName=tk.name||tk.type||'未知';
      var tMeta=tk.message||'';
      if(tk.progress!=null)tMeta='进度: '+(tk.progress*100).toFixed(0)+'%';
      h+='<div class="tsk-item">'+
        '<div class="tsk-icon">'+tIcon+'</div>'+
        '<div class="tsk-body"><div class="tsk-name">'+escHtml(tName)+'</div>'+
        '<div class="tsk-meta">'+escHtml(tMeta)+'</div></div>'+
        '<span class="tsk-status '+tCl+'">'+tLb+'</span>';
      if(tSt==='running'||tSt==='paused'){
        h+='<button class="tsk-act" onclick="resumeTask(\''+tk.task_id+'\')">恢复</button>'
      }
      h+='<button class="tsk-act" onclick="delHomeTask(\''+tk.task_id+'\')">删除</button>';
      h+='</div>'
    }
    var s=document.querySelector('#tab-home .tsk-section');
    if(s){var hdr=s.querySelector('.tsk-header');if(hdr)hdr.outerHTML='<div class="tsk-header">任务列表</div>';s.innerHTML=h}
  })
}

function resumeTask(id){
  A('POST','/api/task/resume',{task_id:id}).then(function(r){
    if(r.ok){T('任务已恢复','o');refreshHomeTasks()}else T('恢复失败: '+r.error,'e')
  })
}

async function delHomeTask(id){ await A('POST','/api/task/delete',{task_id:id}); refreshHomeTasks(); }

// Tab navigation
// 拖动/点击共用状态
var _navDrag={active:false,startX:0,lastX:0,startCenter:0,moved:false,centers:[],
              vel:0,            // 拖拽速度(px/事件)
              sx:1,sy:1,        // 当前透镜 scale
              tsx:1,tsy:1,      // 目标 scale
              raf:null};
var _glideBaseScale=1.0; // 按下放大系数
var _glideLens=null;     // 透镜元素缓存

// 计算每个按钮中心（相对 .nav 左缘）
function tabCenters(){
  var nav=document.getElementById('bottom-nav');
  var cs=[];
  if(nav)nav.querySelectorAll('button').forEach(function(b){cs.push(b.offsetLeft+b.offsetWidth/2)});
  return cs;
}
// 让指示器胶囊尺寸=激活按钮尺寸（约一 tab 宽）
function syncGlideSize(){
  var glide=document.getElementById('nav-glide');
  var btn=document.querySelector('.nav button.active')||document.querySelector('.nav button');
  if(!glide||!btn)return;
  glide.style.width=btn.offsetWidth+'px';
  glide.style.height=btn.offsetHeight+'px';
}
// 把指示器平移到某个中心；animate=false 时关 transition 跟手
function positionGlideAt(center, animate){
  var glide=document.getElementById('nav-glide');
  if(!glide)return;
  if(!animate)glide.classList.add('no-anim');
  else glide.classList.remove('no-anim');
  glide.style.transform='translateX('+(center-glide.offsetWidth/2)+'px)';
}
function updateNavGlide(){
  syncGlideSize();
  var btn=document.querySelector('.nav button.active');
  if(!btn)return;
  positionGlideAt(btn.offsetLeft+btn.offsetWidth/2,true);
}
// 应用当前 scale 到透镜（scaleX/scaleY 独立，实现速度拉伸/压缩）
function applyGlideScale(){
  if(!_glideLens)return;
  _glideLens.style.transform='scale('+_navDrag.sx+','+_navDrag.sy+')';
}
// rAF 循环：把 sx/sy 平滑趋向目标(tsx/tsy)，模拟原版 spring 平滑
function glideTick(){
  var d=_navDrag;
  var k=0.28; // 平滑系数
  d.sx+=(d.tsx-d.sx)*k;
  d.sy+=(d.tsy-d.sy)*k;
  applyGlideScale();
  if(Math.abs(d.sx-d.tsx)<0.002&&Math.abs(d.sy-d.tsy)<0.002&&!d.active){
    d.sx=d.tsx;d.sy=d.tsy;applyGlideScale();
    d.raf=null;return;
  }
  d.raf=requestAnimationFrame(glideTick);
}
function startGlideRaf(){
  if(!_navDrag.raf)_navDrag.raf=requestAnimationFrame(glideTick);
}
// 拖拽指示器：按下放大，横移连续跟随 + 速度变形，松手吸附最近 tab
function initNavDrag(){
  var nav=document.getElementById('bottom-nav');
  if(!nav||nav.dataset.dragInit)return;
  nav.dataset.dragInit='1';
  var glide=document.getElementById('nav-glide');
  _glideLens=glide?glide.querySelector('.nav-glide-lens'):null;

  nav.addEventListener('pointerdown',function(e){
    _navDrag.active=true;_navDrag.moved=false;
    _navDrag.startX=e.clientX;_navDrag.lastX=e.clientX;_navDrag.vel=0;
    _navDrag.centers=tabCenters();
    var btn=nav.querySelector('button.active');
    _navDrag.startCenter=(btn?btn.offsetLeft+btn.offsetWidth/2:_navDrag.centers[0]||0);
    // Keep the pressed button as the click target; capture only after an actual drag.
    // 按下：目标放大
    _navDrag.tsx=_navDrag.tsy=_glideBaseScale+0.4;
    startGlideRaf();
  });

  nav.addEventListener('pointermove',function(e){
    if(!_navDrag.active)return;
    var dx=e.clientX-_navDrag.lastX;
    if(Math.abs(e.clientX-_navDrag.startX)>8){if(!_navDrag.moved&&nav.setPointerCapture)try{nav.setPointerCapture(e.pointerId)}catch(err){} _navDrag.moved=true;}
    _navDrag.vel=dx; // 速度(带方向)
    var total=e.clientX-_navDrag.startX;
    // 跟随手指
    positionGlideAt(_navDrag.startCenter+total,false);
    // 速度变形：横拉 scaleX、纵压 scaleY（还原 layerBlock: sx/=1-v, sy*=1-v）
    var v=_navDrag.vel/8; // 归一化
    v=Math.max(-0.35,Math.min(0.35,v));
    _navDrag.tsx=(_glideBaseScale+0.4)/(1-v*0.75);
    _navDrag.tsy=(_glideBaseScale+0.4)*(1-v*0.25);
    _navDrag.lastX=e.clientX;
    startGlideRaf();
  });

  function endDrag(e){
    if(!_navDrag.active)return;
    _navDrag.active=false;
    // 松手：scale 回落到 1
    _navDrag.tsx=_navDrag.tsy=1;
    if(_navDrag.moved){
      // 吸附最近 tab
      var cs=_navDrag.centers||tabCenters();
      var x=(e.clientX||_navDrag.lastX)-nav.getBoundingClientRect().left;
      var idx=0,best=1e9;
      cs.forEach(function(c,i){var d=Math.abs(c-x);if(d<best){best=d;idx=i}});
      var btn=nav.querySelectorAll('button')[idx];
      if(btn){
        var t=btn.getAttribute('data-tab');
        if(t!==curTab)tabHistory.push(t);
        tabTo(t); // tabTo 内部会 updateNavGlide 平滑回弹
      }
    }else{
      updateNavGlide();
    }
    startGlideRaf(); // 继续收敛到 1 后自动停止
  }
  nav.addEventListener('pointerup',endDrag);
  nav.addEventListener('pointercancel',endDrag);
}
initNavDrag();

document.querySelectorAll('.nav button').forEach(function(b){
  b.addEventListener('click',function(){
    if(_navDrag.moved){_navDrag.moved=false;return} // 拖过的手指抬起，忽略此次点击
    var t=b.getAttribute('data-tab');
    if(t!==curTab)tabHistory.push(t);
    tabTo(t);
  })
});

function tabTo(t){
  if(typeof yyNavigateEpoch==='number')yyNavigateEpoch++;
  var next=document.getElementById('tab-'+t);
  if(!next)return;
  var forward=TAB_ORDER.indexOf(t)>TAB_ORDER.indexOf(curTab),changed=t!==curTab;
  // A switch owns the complete viewport immediately. No old animation callback
  // may make its destination visible after a newer navigation.
  document.querySelectorAll('.content').forEach(function(panel){
    panel.classList.remove('show','tab-in-right','tab-in-left','tab-out-right','tab-out-left');
    panel.setAttribute('aria-hidden',panel===next?'false':'true');
    panel.inert=panel!==next;
  });
  curTab=t;
  next.classList.add('show');
  if(changed){void next.offsetWidth;next.classList.add(forward?'tab-in-right':'tab-in-left')}
  document.querySelectorAll('.nav button').forEach(function(b){b.classList.toggle('active',b.getAttribute('data-tab')===t)});
  updateNavGlide();
  activeRenderers.forEach(function(kill){try{kill()}catch(e){}});activeRenderers=[];
  if(t!=='settings'){
    if(typeof _cfgRefreshTimer!=='undefined'){clearInterval(_cfgRefreshTimer);_cfgRefreshTimer=null}
    if(typeof _toolboxPollTimer!=='undefined'){clearInterval(_toolboxPollTimer);_toolboxPollTimer=null}
  }
  if(!tabInited[t]){tabInited[t]=true;if(t==='home')Rhome();if(t==='terminal')Rconsole();if(t==='settings')Rcfg()}
  if(t==='tasks')Rplg();
}

// Android back button
window.__onBackPressed__=function(){
  var modal=document.querySelector('.modal-overlay');if(modal){modal.remove();return'true'}
  if(subPageOpen){closeSubPage();return'true'}
  if(tabHistory.length>1){
    tabHistory.pop();
    tabTo(tabHistory[tabHistory.length-1]);
    return'true'
  }
  return'exit'
};
// 触觉反馈：点击交互元素时轻微震动（Android WebView 支持 navigator.vibrate）
document.addEventListener('click', function(e) {
  var el = e.target && e.target.closest ? e.target.closest('button, .qcard, .grid-card, .fp-item, .bs-option, .pick, .mode-cap-btn, .tsk-act, .sp-back, .cb-btn, a[href], [role="button"]') : null;
  if (el && !el.disabled && navigator && navigator.vibrate) {
    try { navigator.vibrate(12); } catch (err) {}
  }
});

// 首页在全部功能与工作台脚本加载完成后初始化。
// 布局变化（旋转/字体）后重新定位滑动胶囊
window.addEventListener('resize',function(){updateNavGlide()});
