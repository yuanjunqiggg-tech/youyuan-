function tabTo(t){
  document.querySelectorAll('.nav button').forEach(function(b){b.classList.remove('active')});
  var btn=document.querySelector('.nav button[data-tab="'+t+'"]');
  if(btn)btn.classList.add('active');
  updateNavGlide();

  if(!tabInited[t]){tabInited[t]=true;if(t==='home')Rhome();if(t==='terminal')Rconsole();if(t==='settings')Rcfg()}
  // 插件列表每次切回都重新拉取，保证说明/配置更新后无需重启即可看到。
  if(t==='tasks')Rplg();
  activeRenderers.forEach(function(kill){try{kill()}catch(e){}});activeRenderers=[];
  if(t!=='settings'){if(typeof _cfgRefreshTimer!=='undefined'){clearInterval(_cfgRefreshTimer);_cfgRefreshTimer=null}if(typeof _toolboxPollTimer!=='undefined'){clearInterval(_toolboxPollTimer);_toolboxPollTimer=null}}

  var next=document.getElementById('tab-'+t);
  var prev=document.getElementById('tab-'+curTab);
  if(t===curTab){if(next)next.classList.add('show');return}
  var forward=TAB_ORDER.indexOf(t)>TAB_ORDER.indexOf(curTab);
  curTab=t;

  // 新页进入：从对侧淡入滑入
  function enter(){
    if(!next)return;
    next.classList.remove('tab-in-right','tab-in-left','tab-out-right','tab-out-left');
    void next.offsetWidth;
    next.classList.add('show',forward?'tab-in-right':'tab-in-left')
  }

  // 旧页离开：朝一侧淡出，结束后隐藏并进入新页（交接过渡）
  if(prev&&prev.classList.contains('show')){
    prev.classList.remove('tab-in-right','tab-in-left','tab-out-right','tab-out-left');
    prev.classList.add(forward?'tab-out-left':'tab-out-right');
    var done=false;
    function finish(){if(done)return;done=true;prev.classList.remove('show','tab-out-right','tab-out-left');enter()}
    prev.addEventListener('animationend',function(e){if(e.animationName.indexOf('tabOut')===0)finish()},{once:true});
    setTimeout(finish,320)
  }else{
    enter()
  }
}

// Android back button
