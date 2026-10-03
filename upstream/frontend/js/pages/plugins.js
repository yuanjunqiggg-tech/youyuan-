// 插件管理页：手风琴卡片（状态栏 + 启停开关 + 展开区显示操作文档/停用/重载/删除），实时刷新。
// 复用 css/components.css 的 .tgl 复选框开关（<label class="lbl-cb"><input type=checkbox><span class="tgl"></span></label>）。
// 停用=持久禁用（后端 disabled 字段，重启仍在，重连不自动加载）；开关仅控制瞬态运行。
async function Rplg(){
  var r=await A('GET','/api/plugin/list');
  var h='<div class="plg-section" style="padding:14px">'
    + '<div style="display:flex;align-items:center;gap:8px;margin-bottom:10px">'
    + '<div class="tsk-header" style="font-size:15px;font-weight:600;flex:1">插件管理</div>'
    + '<button class="btn" style="width:auto;padding:6px 12px;font-size:13px" onclick="plgImport()">导入插件</button>'
    + '<button class="btn" style="width:auto;padding:6px 12px;font-size:13px" onclick="pmOpenMarket()">市场</button></div>';
  if(!r.ok||!r.plugins||!r.plugins.length){
    h+='<div class="card"><p style="text-align:center;color:var(--phx-text-secondary);padding:20px;font-size:13px">暂无插件</p></div>';
  } else {
    // 排序：停用(disabled)排底部，其余按插件名排序
    var plist=r.plugins.slice().sort(function(a,b){
      var ad=!!a.disabled,bd=!!b.disabled;
      if(ad!==bd)return ad?1:-1;
      return (a.name||'').localeCompare(b.name||'','zh');
    });
    plist.forEach(function(p){
      var railClr = p.state==='loaded' ? '#2ecc71' : p.state==='error' ? '#e74c3c' : '#888';
      var disabled = !!p.disabled; // 持久禁用（来自后端）
      var running = !!p.running && !disabled; // 停用后开关视为关闭
      // 复用 .tgl 开关：checked 为开，disabled 置灰不可点
      var sw = '<label class="lbl-cb plg-sw" style="margin:0;flex-shrink:0;cursor:pointer;' + (disabled ? 'pointer-events:none;opacity:.45;' : '') + '" onclick="event.stopPropagation()">'
        + '<input type="checkbox" ' + (running ? 'checked' : '') + (disabled ? ' disabled' : '') + ' onchange="plgSwitch(\'' + p.id + '\',this.checked)">'
        + '<span class="tgl"></span></label>';
      h+='<div class="plg-card' + (disabled ? ' plg-disabled' : '') + '" style="border:1px solid var(--phx-border-light);border-left:4px solid ' + railClr + ';border-radius:10px;background:var(--phx-bg-card);margin-bottom:10px;padding:0 12px;overflow:hidden">'
        + '<div class="plg-head" style="display:flex;align-items:center;gap:12px;padding:12px 0;cursor:pointer" onclick="plgToggle(this)">'
          + '<div class="plg-summary"><div class="plg-name">' + escHtml(p.name) + '</div>'
          + '<div class="plg-description">' + escHtml(p.type + ' · ' + (p.description||'')) + '</div></div>'
          + '<div class="plg-controls">'+sw+'<span class="plg-state">'+(disabled?'已停用':running?'运行中':'未启动')+'</span>'
          + '<button class="tsk-act" style="flex-shrink:0" onclick="event.stopPropagation();plgDataDir(\'' + p.id + '\')">目录</button>'
          + '<button class="tsk-act" style="flex-shrink:0" onclick="event.stopPropagation();' + (disabled ? 'plgEnable(\'' + p.id + '\')' : 'plgDisable(\'' + p.id + '\')') + '">' + (disabled ? '启用' : '停用') + '</button>'
          + '<button class="tsk-act tsk-act-danger" style="flex-shrink:0" onclick="event.stopPropagation();plgDel(\'' + p.id + '\')">删除</button>'
          + '<span style="opacity:.5;flex-shrink:0;transition:transform .2s" data-fold="1">▾</span></div></div>'
        + '<div class="plg-body" style="display:none;padding:10px 0 12px 0;border-top:1px solid rgba(128,128,128,.15)">'
          + (p.state==='error' ? '<div style="color:var(--phx-error);font-size:12px;margin-bottom:8px;word-break:break-all">⚠ ' + escHtml(p.error) + '</div>' : '')
          + '<div style="margin-bottom:12px;display:flex;flex-direction:column;gap:9px">'
            + '<div style="display:flex;gap:8px">'
              + '<button class="tsk-act tsk-act-primary" style="flex:1" onclick="plgEdit(\'' + p.id + '\')">编辑</button>'
              + '<button class="tsk-act" style="flex:1" onclick="' + (disabled ? 'plgEnable(\'' + p.id + '\')' : 'plgDisable(\'' + p.id + '\')') + '">' + (disabled ? '启用' : '停用') + '</button>'
            + '</div>'
            + '<div style="display:flex;gap:8px;flex-wrap:wrap">'
            + '<button class="tsk-act" onclick="plgReload(\'' + p.id + '\')">重载</button>'
            + '<button class="tsk-act" onclick="plgEditConfig(\'' + p.id + '\')">编辑配置</button>'
            + '<button class="tsk-act" onclick="plgDataDir(\'' + p.id + '\')">数据目录</button>'
            + '<button class="tsk-act" onclick="plgRename(\'' + p.id + '\',\'' + String(p.name).replace(/'/g, "\\'") + '\',\'' + String(p.description||'').replace(/'/g, "\\'") + '\')">重命名</button>'
            + '<button class="tsk-act" onclick="pmOpenUpload(\'' + p.id + '\')">上传</button>'
            + '<button class="tsk-act" onclick="plgExportSAF(\'' + p.id + '\')">导出</button>'
            + '<button class="tsk-act tsk-act-danger" onclick="plgDel(\'' + p.id + '\')">删除</button>'
            + '</div></div>'
          + '<p class="plg-full-description">'+escHtml(p.description||'')+'</p>'
          + '<div class="plg-docs" style="font-size:12px;line-height:1.6">' + (p.docs ? marked.parse(p.docs) : '<span style="color:var(--phx-text-secondary);font-size:12px">暂无操作文档</span>') + '</div>'
        + '</div></div>';
    });
  }
  h+='</div>';
  document.getElementById('tab-tasks').innerHTML=h;
}
function plgToggle(el){
  var body = el.parentNode.querySelector('.plg-body');
  var fold = el.querySelector('[data-fold]');
  if(body.style.display==='none'){ body.style.display='block'; if(fold)fold.style.transform='rotate(180deg)'; }
  else { body.style.display='none'; if(fold)fold.style.transform='rotate(0deg)'; }
}
async function plgSwitch(id,on){
  await A('POST', on ? '/api/plugin/run' : '/api/plugin/stop', {id:id});
  Rplg();
}
// 停用 = 永久禁用（后端持久化，重连不自动加载）
async function plgDisable(id){
  await A('POST','/api/plugin/disabled',{id:id, disabled:true});
  Rplg();
}
async function plgEnable(id){
  await A('POST','/api/plugin/disabled',{id:id, disabled:false});
  Rplg();
}
async function plgReload(id){ await A('POST','/api/plugin/reload',{id:id}); Rplg(); }
function plgDel(id){
  showConfirm({title:'删除插件',message:'确定删除插件「'+id+'」吗？此操作不可恢复。',confirmText:'删除',onConfirm:function(){ A('POST','/api/plugin/delete',{id:id}).then(function(){ Rplg(); }); }});
}
// 重命名插件 + 改小描述（自定义弹窗，非浏览器原生 prompt）。
function plgRename(id, name, desc){
  var ov=document.createElement('div'); ov.className='modal-overlay';
  var escId=String(id).replace(/'/g,"\\'");
  ov.innerHTML='<div class="modal-box" style="max-width:340px">'+
    '<div style="font-size:15px;font-weight:800;margin-bottom:4px">重命名插件</div>'+
    '<div style="font-size:12px;color:var(--phx-text-secondary);margin-bottom:12px">修改插件展示名与小描述</div>'+
    '<div style="font-size:12px;color:var(--phx-text-secondary);margin-bottom:4px">插件名称</div>'+
    '<input id="plg-rn-name" type="text" value="'+escHtml(name||'')+'" placeholder="插件名称" style="width:100%;padding:10px;border:2px solid var(--phx-border);border-radius:8px;background:var(--phx-bg);font-size:14px;color:var(--phx-text);box-sizing:border-box;margin-bottom:10px;outline:none">'+
    '<div style="font-size:12px;color:var(--phx-text-secondary);margin-bottom:4px">小描述</div>'+
    '<input id="plg-rn-desc" type="text" value="'+escHtml(desc||'')+'" placeholder="一句话说明" style="width:100%;padding:10px;border:2px solid var(--phx-border);border-radius:8px;background:var(--phx-bg);font-size:14px;color:var(--phx-text);box-sizing:border-box;margin-bottom:14px;outline:none">'+
    '<div style="display:flex;gap:8px">'+
    '<button class="bs-btn bs-btn-cancel" style="flex:1" onclick="closeOverlay(this.closest(\'.modal-overlay\'))">取消</button>'+
    '<button class="bs-btn bs-btn-confirm" style="flex:1" onclick="plgRenameSave(\''+escId+'\')">保存</button></div></div>';
  document.body.appendChild(ov);
  ov.addEventListener('click',function(e){if(e.target===ov)closeOverlay(ov)});
  var inp=document.getElementById('plg-rn-name'); if(inp)inp.focus();
}
async function plgRenameSave(id){
  var name=((document.getElementById('plg-rn-name')||{}).value||'').trim();
  var desc=(document.getElementById('plg-rn-desc')||{}).value||'';
  if(!name){T('插件名称不能为空','e');return;}
  T('正在保存...','w');
  var r=await A('POST','/api/plugin/meta',{id:id,name:name,description:desc});
  if(!r.ok){T(r.error||'保存失败','e');return;}
  T('已保存','o');
  closeOverlay(document.querySelector('.modal-overlay'));
  Rplg();
}
