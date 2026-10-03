// ═══════════════════════════════════════════════════════════════
// 命令方块工程 — 导入/导出/统计 全流程 UI
// ═══════════════════════════════════════════════════════════════

var cbState = {tab:'import',content:'',proj:null,stats:null,importing:false,exporting:false};

// ── 入口 ──

function RcbHTML(){
  return '<div id="cb-wrap" style="width:100%">'+
    '<div class="row" style="gap:6px;margin-bottom:10px">'+
      '<button class="btn-s '+(cbState.tab==='import'?'active':'')+'" id="cb-tab-import" onclick="cbSwitchTab(\'import\')" style="flex:1;font-size:12px">📥 导入</button>'+
      '<button class="btn-s '+(cbState.tab==='export'?'active':'')+'" id="cb-tab-export" onclick="cbSwitchTab(\'export\')" style="flex:1;font-size:12px">📤 导出</button>'+
    '</div>'+
    '<div id="cb-body"></div>'+
  '</div>';
}

function cbSwitchTab(tab){
  cbState.tab=tab;
  document.querySelectorAll('#cb-wrap .btn-s').forEach(function(b){b.classList.remove('active')});
  var el=document.getElementById('cb-tab-'+tab);
  if(el)el.classList.add('active');
  cbRenderTab();
}

function cbRenderTab(){
  var b=document.getElementById('cb-body');
  if(!b)return;
  if(cbState.tab==='import')cbRenderImport();
  else cbRenderExport();
}

// ═══════════════════════════════════════════════
// 导入
// ═══════════════════════════════════════════════

function cbRenderImport(){
  var b=document.getElementById('cb-body');
  b.innerHTML=
    '<div class="wz-section-title"><svg width="18" height="18" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>步骤 1/2 · 选择工程文件</div>'+
    '<div class="pick" onclick="cbPickImportFile()" style="margin-top:10px"><i class="pi"><svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg></i><div id="cb-import-pick-txt">点击选择 .mcfunction 工程文件</div><div style="font-size:10px;color:var(--phx-text-secondary)">.mcfunction .txt</div></div>'+
    '<div id="cb-import-preview" class="gone" style="margin-top:10px"></div>'+
    '<div id="cb-import-step2" class="gone" style="margin-top:10px"></div>';
}

function cbPickImportFile(){
  pickBySetting('text',function(path){
    cbState.filePath=path;
    document.getElementById('cb-import-pick-txt').innerHTML='<span class="spin"></span> 加载中...';
    fetch('/api/files/read?path='+encodeURIComponent(path)).then(function(r){if(!r.ok)throw Error('HTTP '+r.status);return r.text()})
    .then(function(c){
      cbState.content=c;
      return A('POST','/api/cb/import/preview',{content:c});
    })
    .then(function(r){
      if(!r.ok){T('解析失败: '+(r.error||''),'e');return}
      cbState.proj=r.project;
      cbState.stats=r.stats;
      var p=r.project;
      document.getElementById('cb-import-pick-txt').textContent=path.split('/').pop();
      cbRenderImportPreview(p,r.stats);
    })
    .catch(function(e){T('读取失败: '+e.message,'e');document.getElementById('cb-import-pick-txt').textContent='点击选择文件'});
  },'text');
}

function cbRenderImportPreview(proj,stats){
  var h='<div class="card" style="padding:14px">'+
    '<div style="font-size:14px;font-weight:800;color:var(--phx-text)">📄 '+escHtml(proj.name||'未命名工程')+'</div>';
  if(proj.version)h+='<div style="font-size:10px;color:var(--phx-text-secondary)">v'+escHtml(proj.version)+'</div>';
  if(proj.author)h+='<div style="font-size:10px;color:var(--phx-text-secondary)">作者: '+escHtml(proj.author)+'</div>';
  if(proj.description)h+='<div style="font-size:11px;color:var(--phx-text);margin-top:4px">'+escHtml(proj.description)+'</div>';

  // 统计概览
  h+='<div style="margin-top:10px;padding:10px;border-radius:8px;background:var(--phx-bg)">';
  h+='<div style="font-size:11px;font-weight:700;color:var(--phx-text);margin-bottom:6px">📊 工程概览</div>';
  h+='<div class="stats" style="gap:2px">';
  h+='<div class="s"><div class="n">'+stats.total+'</div><div class="l">命令方块</div></div>';
  h+='<div class="s"><div class="n">'+stats.chain_count+'</div><div class="l">命令链</div></div>';
  h+='<div class="s"><div class="n">'+stats.breaks+'</div><div class="l">断链</div></div>';
  h+='</div>';
  h+='<div style="font-size:10px;color:var(--phx-text-secondary);margin-top:4px">脉冲:'+stats.pulse+' / 循环:'+stats.repeating+' / 连锁:'+stats.chain+'</div>';
  h+='<div style="font-size:10px;color:var(--phx-text-secondary)">有条件:'+stats.conditional+' / 无条件:'+stats.unconditional+' / 有延迟:'+stats.has_delay+'</div>';
  h+='</div>';

  // 资源依赖
  if(proj.scoreboards&&proj.scoreboards.length>0||proj.tags&&proj.tags.length>0){
    h+='<div style="margin-top:8px;padding:10px;border-radius:8px;background:var(--phx-bg)">';
    h+='<div style="font-size:11px;font-weight:700;color:var(--phx-text);margin-bottom:6px">📦 资源依赖</div>';
    if(proj.scoreboards&&proj.scoreboards.length>0){
      h+='<div style="font-size:10px;color:var(--phx-text-secondary)">积分板: '+proj.scoreboards.map(function(s){return escHtml(s)}).join(', ')+'</div>';
    }
    if(proj.tags&&proj.tags.length>0){
      h+='<div style="font-size:10px;color:var(--phx-text-secondary)">标签: '+proj.tags.map(function(t){return escHtml(t)}).join(', ')+'</div>';
    }
    h+='</div>';
  }

  h+='</div>';

  // 第二步：导入配置
  h+='<div style="margin-top:12px">'+
    '<div style="font-size:12px;font-weight:700;margin-bottom:6px">步骤 2/2 · 导入配置</div>'+
    '<div class="card" style="padding:12px">'+
    '<label>导入起点坐标</label>'+
    '<div class="row" style="gap:6px">'+
      '<input id="cb-import-x" type="number" placeholder="X" style="flex:1">'+
      '<input id="cb-import-y" type="number" placeholder="Y" style="flex:1">'+
      '<input id="cb-import-z" type="number" placeholder="Z" style="flex:1">'+
    '</div>'+
    '<div style="font-size:9px;color:var(--phx-text-secondary);margin-top:4px">首个命令方块将放置在此坐标，后续按朝向自动偏移</div>'+
    '<div class="row" style="margin-top:10px">'+
      '<button class="btn" onclick="cbStartImport()" style="flex:1">🚀 开始导入</button>'+
    '</div>'+
    '</div>'+
  '</div>';

  // 导入进度区
  h+='<div id="cb-import-progress" class="gone" style="margin-top:10px"></div>';

  document.getElementById('cb-import-preview').innerHTML=h;
  document.getElementById('cb-import-preview').classList.remove('gone');
}

function cbStartImport(){
  var getN=function(id){var v=parseInt(document.getElementById(id).value);return isNaN(v)?null:v};
  var x=getN('cb-import-x'),y=getN('cb-import-y'),z=getN('cb-import-z');
  if(x===null||y===null||z===null){T('请填写完整的导入起点坐标 X/Y/Z','e');return}

  if(!cbState.content){T('请先选择工程文件','e');return}

  T('正在启动导入...','o');
  A('POST','/api/cb/import/start',{
    content:cbState.content,
    start_x:x,start_y:y,start_z:z
  }).then(function(r){
    if(!r.ok){T('启动失败: '+(r.error||''),'e');return}
    cbState.importing=true;
    cbShowImportProgress();
    // 进度由 SSE task_progress 驱动，完成/失败由 cbFinalizeImport 收尾
  });
}

function cbShowImportProgress(){
  var el=document.getElementById('cb-import-progress');
  if(!el)return;
  el.classList.remove('gone');
  el.innerHTML=
    '<div class="card" style="padding:12px">'+
    '<div style="display:flex;justify-content:space-between;margin-bottom:6px">'+
      '<span style="font-size:12px;font-weight:700">导入进度</span>'+
      '<span class="dim" style="font-size:10px" id="cb-import-pct">0%</span>'+
    '</div>'+
    '<div class="pg"><div id="cb-import-bar" style="width:0%"></div></div>'+
    '<div id="cb-import-msg" style="font-size:10px;color:var(--phx-text-secondary);margin-top:4px">准备中...</div>'+
    '<div class="row" style="margin-top:8px"><button class="btn-d" onclick="cbStopImport()" style="flex:1">■ 停止</button></div>'+
    '</div>';
}

// cbFinalizeImport 由 SSE task_done/task_error 调用，收尾导入进度 UI。
function cbFinalizeImport(ok, msg){
  cbState.importing=false;
  var bar=document.getElementById('cb-import-bar');
  var pctEl=document.getElementById('cb-import-pct');
  var msgEl=document.getElementById('cb-import-msg');
  if(bar)bar.style.width=ok?'100%':bar.style.width;
  if(pctEl)pctEl.textContent=ok?'完成':'失败';
  if(msgEl)msgEl.innerHTML=ok
    ?'<span style="color:var(--phx-success)">✅ '+(msg||'导入完成')+'</span>'
    :'<span style="color:var(--phx-error)">✗ '+(msg||'导入失败')+'</span>';
  var stopBtn=document.querySelector('#cb-import-progress .btn-d');
  if(stopBtn)stopBtn.classList.add('gone');
}

function cbStopImport(){
  A('POST','/api/task/stop',{}).then(function(r){
    if(r.ok){T('已停止导入','o');cbState.importing=false}
  });
}

// ═══════════════════════════════════════════════
// 导出
// ═══════════════════════════════════════════════

function cbRenderExport(){
  var b=document.getElementById('cb-body');
  b.innerHTML=
    '<div class="wz-section-title"><svg width="18" height="18" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/><rect x="3" y="3" width="16" height="6" rx="1"/></svg>步骤 1/2 · 选择区域</div>'+
    '<div class="card" style="padding:12px;margin-top:10px">'+
    '<label>扫描区域起点</label>'+
    '<div class="row" style="gap:6px">'+
      '<input id="cb-export-x1" type="number" placeholder="X" style="flex:1">'+
      '<input id="cb-export-y1" type="number" placeholder="Y" style="flex:1">'+
      '<input id="cb-export-z1" type="number" placeholder="Z" style="flex:1">'+
    '</div>'+
    '<label style="margin-top:8px">扫描区域终点</label>'+
    '<div class="row" style="gap:6px">'+
      '<input id="cb-export-x2" type="number" placeholder="X" style="flex:1">'+
      '<input id="cb-export-y2" type="number" placeholder="Y" style="flex:1">'+
      '<input id="cb-export-z2" type="number" placeholder="Z" style="flex:1">'+
    '</div>'+
    '<div style="font-size:9px;color:var(--phx-text-secondary);margin-top:4px">扫描区域不宜超过 64×64×64</div>'+
    '<div class="row" style="margin-top:10px">'+
      '<button class="btn" onclick="cbStartScan()" style="flex:1">🔍 开始扫描</button>'+
    '</div>'+
    '</div>'+
    '<div id="cb-export-result" class="gone" style="margin-top:10px"></div>'+
    '<div id="cb-export-progress" class="gone" style="margin-top:10px"></div>';
}

function cbStartScan(){
  var getN=function(id){var v=parseInt(document.getElementById(id).value);return isNaN(v)?null:v};
  var x1=getN('cb-export-x1'),y1=getN('cb-export-y1'),z1=getN('cb-export-z1');
  var x2=getN('cb-export-x2'),y2=getN('cb-export-y2'),z2=getN('cb-export-z2');
  if(x1===null||y1===null||z1===null||x2===null||y2===null||z2===null){
    T('请填写完整的扫描区域坐标（起点 X/Y/Z 和终点 X/Y/Z）','e');
    return
  }

  T('正在扫描...','o');
  var pEl=document.getElementById('cb-export-progress');
  if(pEl){
    pEl.classList.remove('gone');
    pEl.innerHTML='<div class="card" style="padding:12px;text-align:center"><span class="spin"></span> 扫描中...</div>';
  }

  A('POST','/api/cb/export/scan',{x1:x1,y1:y1,z1:z1,x2:x2,y2:y2,z2:z2}).then(function(r){
    if(pEl)pEl.classList.add('gone');
    if(!r.ok){
      if(r.over_limit){T('扫描区域超过 64×64×64 软限制，请缩小范围','e')}
      else{T('扫描失败: '+(r.error||''),'e')}
      return
    }
    cbState.scanResult=r;
    cbRenderExportResult(r);
  });
}

function cbRenderExportResult(r){
  var stats=r.stats;
  var content=r.content;
  var el=document.getElementById('cb-export-result');
  if(!el)return;
  el.classList.remove('gone');

  // 统计概览
  var h='<div class="card" style="padding:14px">'+
    '<div style="font-size:12px;font-weight:700;margin-bottom:8px;color:var(--phx-text)">📊 扫描结果</div>'+
    '<div class="stats" style="gap:2px">'+
    '<div class="s"><div class="n">'+stats.total_blocks+'</div><div class="l">命令方块</div></div>'+
    '<div class="s"><div class="n">'+stats.chain_count+'</div><div class="l">命令链</div></div>'+
    '<div class="s"><div class="n">'+stats.breaks+'</div><div class="l">断链</div></div>'+
    '</div>'+
    '<div style="font-size:10px;color:var(--phx-text-secondary);margin-top:4px">脉冲:'+stats.pulse+' / 循环:'+stats.repeating+' / 连锁:'+stats.chain+'</div>'+
    '<div style="font-size:10px;color:var(--phx-text-secondary)">有条件:'+stats.conditional+' / 无条件:'+stats.unconditional+'</div>';

  // 资源
  if(stats.scoreboards&&stats.scoreboards.length){
    h+='<div style="margin-top:6px;font-size:10px;color:var(--phx-text-secondary)">📦 积分板: '+stats.scoreboards.map(function(s){return escHtml(s)}).join(', ')+'</div>';
  }
  if(stats.tags&&stats.tags.length){
    h+='<div style="font-size:10px;color:var(--phx-text-secondary)">🏷️ 标签: '+stats.tags.map(function(t){return escHtml(t)}).join(', ')+'</div>';
  }

  // 链预览（大工程只预览前 30 条链，避免 WebView 渲染海量 DOM 崩溃）
  var chains=r.result&&r.result.chains;
  if(chains&&chains.length){
    var MAX_CHAINS=30, MAX_STEPS=12;
    var shown=Math.min(chains.length,MAX_CHAINS);
    h+='<div style="margin-top:8px">';
    for(var ci=0;ci<shown;ci++){
      var ch=chains[ci];
      var n=ch.blocks.length;
      h+='<div style="font-size:10px;color:var(--phx-text-secondary);padding:4px 0">🔗 链 #'+(ci+1)+' ('+n+' 方块)';
      var steps=[];
      var bcount=Math.min(n,MAX_STEPS);
      for(var bi=0;bi<bcount;bi++){
        var b=ch.blocks[bi];
        var s=b.facing+' ';
        if(b.type==='impulse')s+='脉冲';
        else if(b.type==='repeating')s+='循环';
        else s+='连锁';
        if(b.conditional)s+='(有条件)';
        steps.push(s);
      }
      if(n>bcount)steps.push('…');
      h+='<br><span style="font-size:9px;color:var(--phx-text-secondary)">'+steps.join(' → ')+'</span></div>';
    }
    if(chains.length>shown)h+='<div style="font-size:10px;color:var(--phx-text-secondary);padding:4px 0">… 其余 '+(chains.length-shown)+' 条链省略</div>';
    h+='</div>';
  }

  h+='</div>';

  // 导出按钮
  h+='<div class="card" style="padding:12px;margin-top:8px">'+
    '<label>文件名</label>'+
    '<input id="cb-export-filename" type="text" value="cb_project.mcfunction" style="width:100%">'+
    '<div class="row" style="margin-top:8px">'+
      '<button class="btn" onclick="cbSaveExport()" style="flex:1">💾 导出工程文件</button>'+
    '</div>'+
  '</div>';

  // 内容预览
  h+='<div class="card" style="padding:12px;margin-top:8px">'+
    '<div style="font-size:11px;font-weight:700;margin-bottom:4px">📄 文件预览</div>'+
    '<pre style="font-size:9px;font-family:monospace;background:var(--phx-bg);padding:8px;border-radius:6px;max-height:200px;overflow:auto;white-space:pre-wrap;word-break:break-all">'+escHtml(content.substring(0,2000))+'</pre>'+
    (content.length>2000?'<div style="font-size:10px;color:var(--phx-text-secondary);text-align:center">... 仅显示前 2000 字符</div>':'')+
  '</div>';

  el.innerHTML=h;
}

function cbSaveExport(){
  var content=cbState.scanResult&&cbState.scanResult.content;
  if(!content){T('没有可导出的内容','e');return}
  var name=document.getElementById('cb-export-filename').value||'cb_project.mcfunction';
  if(!name.endsWith('.mcfunction'))name+='.mcfunction';

  A('POST','/api/cb/export/save',{content:content,file_name:name}).then(function(r){
    if(r.ok){T('导出成功: '+escHtml(r.path),'o')}
    else{T('导出失败: '+(r.error||''),'e')}
  });
}

// ── 页面状态恢复 ──

function restoreCBState(){
  // 导入进度由 SSE task_progress/task_done/task_error 驱动，无需轮询恢复
}