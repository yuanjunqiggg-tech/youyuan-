// Sequential region migration uses the APK's real export/import task engine.
// The archive remains on this device while the user changes game servers.
var yyMigrateArchive='';
try{yyMigrateArchive=localStorage.getItem('yy-migrate-archive')||''}catch(e){}
function RmgrtLiteHTML(){
  return '<div class="content-inner"><section class="card"><h3>区域迁移 · 顺序模式</h3><p class="yy-hint">先在来源服务器导出到手机，再切换目标服务器并导入。两次任务都使用真实建筑导出 / 导入内核；不要在导出完成前切服。</p>'+
    '<h4>1. 来源区域</h4><div class="yy-form-grid"><label>X1<input type="number" id="mg-x1" value="0"></label><label>Y1<input type="number" id="mg-y1" value="0"></label><label>Z1<input type="number" id="mg-z1" value="0"></label><label>X2<input type="number" id="mg-x2" value="16"></label><label>Y2<input type="number" id="mg-y2" value="16"></label><label>Z2<input type="number" id="mg-z2" value="16"></label></div><div class="yy-actions"><button class="yy-primary" onclick="yyMigrateExport()">导出区域</button><button class="yy-secondary" onclick="yyMigrateCheck()">检查存档</button></div>'+
    '<p class="yy-hint">本地存档：<span id="mg-archive">'+(yyMigrateArchive?escHtml(yyMigrateArchive):'尚未导出')+'</span></p><pre class="yy-readout" id="mg-status"></pre></section>'+
    '<section class="card"><h4>2. 切换目标服并落点</h4><p class="yy-hint">确认存档生成后，在“账号与设置”切换到目标服务器，再返回这里。也可选择手机已有的建筑文件。</p><div class="yy-actions"><button class="yy-secondary" onclick="yyMigratePick()">选择本地存档</button><button class="yy-secondary" onclick="yyNavigate(\'settings\')">前往游戏连接</button></div><div class="yy-form-grid"><label>落点 X<input type="number" id="mg-dx" value="0"></label><label>落点 Y<input type="number" id="mg-dy" value="64"></label><label>落点 Z<input type="number" id="mg-dz" value="0"></label></div><label>导入速度（方块/秒）<input type="number" id="mg-speed" value="9500" min="1"></label><div class="yy-actions"><button class="yy-primary" onclick="yyMigrateImport()">向当前服务器导入</button><button class="yy-secondary" onclick="A(\'POST\',\'/api/task/stop\',{}).then(function(r){T(r.ok?\'任务已停止\':r.error,r.ok?\'o\':\'e\')})">停止当前任务</button></div></section></div>'
}
function yyMigrateNumber(id){var n=Number(document.getElementById(id).value);if(!Number.isSafeInteger(n))throw Error('请填写整数坐标或速度');return n}
async function yyMigrateExport(){
  try{
    if(checkUpdateLock())return;
    var p={format:'mcstructure',output:'/sdcard/Download/游园迁移-'+Date.now()+'.mcstructure'};
    ['x1','y1','z1','x2','y2','z2'].forEach(function(k){p[k]=yyMigrateNumber('mg-'+k)});
    var r=await A('POST','/api/task/start',{type:'export',params:p});
    if(!r.ok)throw Error(r.error||'导出未启动');
    yyMigrateArchive=p.output;try{localStorage.setItem('yy-migrate-archive',p.output)}catch(e){}
    document.getElementById('mg-archive').textContent=p.output;
    document.getElementById('mg-status').textContent='导出已启动。等待终端显示“导出完成”，再点击“检查存档”。';
  }catch(e){T(e.message,'e')}
}
async function yyMigrateCheck(){
  if(!yyMigrateArchive){T('尚未选择存档','e');return false}
  var dir=yyMigrateArchive.substring(0,yyMigrateArchive.lastIndexOf('/'));
  var r=await A('GET','/api/files/scan?path='+encodeURIComponent(dir)+'&filter=building');
  var files=r.files||[];
  var found=files.find(function(f){return f.path===yyMigrateArchive||f.name===yyMigrateArchive.split('/').pop()});
  document.getElementById('mg-status').textContent=found?'存档已生成：'+found.name+' · '+found.size+' 字节。可以连接目标服务器导入。':'尚未发现完整存档；等待导出完成并刷新。'+(r.error?' '+r.error:'');
  return !!found
}
async function yyMigratePick(){var p=await fpicker('building');if(!p)return;yyMigrateArchive=p;try{localStorage.setItem('yy-migrate-archive',p)}catch(e){}document.getElementById('mg-archive').textContent=p;await yyMigrateCheck()}
async function yyMigrateImport(){
  try{
    if(checkUpdateLock())return;
    if(!await yyMigrateCheck())throw Error('存档尚未就绪');
    var speed=yyMigrateNumber('mg-speed');if(speed<1)throw Error('速度必须大于零');
    var p={path:yyMigrateArchive,x:yyMigrateNumber('mg-dx'),y:yyMigrateNumber('mg-dy'),z:yyMigrateNumber('mg-dz'),speed:speed,rotation:0,dimension:'overworld',region_mode:1};
    var r=await A('POST','/api/task/start',{type:'import',params:p});
    if(!r.ok)throw Error(r.error||'导入未启动');
    document.getElementById('mg-status').textContent='导入已启动。真实进度和失败原因请看终端。';
  }catch(e){T(e.message,'e')}
}
