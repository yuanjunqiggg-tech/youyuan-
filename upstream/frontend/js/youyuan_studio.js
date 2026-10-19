/* 游园作业台: preserve the existing business controls and their IDs/handlers. */
(function(){
 const escape=s=>typeof yyEscape==='function'?yyEscape(String(s)):String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 function banner(kind,title,description){return '<header class="yy-studio-head"><div><span class="yy-studio-kicker">游园 / '+kind+'</span><h2>'+title+'</h2><p>'+description+'</p></div><span class="yy-studio-mark" aria-hidden="true">▦</span></header>'}
 function heading(number,title,note){const h=document.createElement('div');h.className='yy-studio-section-title';h.innerHTML='<span>'+number+'</span><div><h3>'+title+'</h3><p>'+note+'</p></div>';return h}
 const importBase=RimpHTML;
 RimpHTML=function(){
  const box=document.createElement('div');box.innerHTML=importBase.apply(this,arguments);const form=box.firstElementChild;
  form.classList.add('yy-studio-config');form.prepend(heading('02','放置与执行','设置位置、写入规则和参与账号。'));
  const waiting=document.createElement('p');waiting.className='yy-studio-await';waiting.textContent='选择并解析源文件后，这里将显示位置、写入规则和执行选项。';form.querySelector('.yy-studio-section-title').after(waiting);
  const source=document.createElement('aside');source.className='card yy-studio-source';source.append(heading('01','建筑源文件','先选文件，再查看解析结果。'));
  for(const selector of ['.pick','#imp-mode-cap','#imp-info','#imp-mcworld-params']){const el=form.querySelector(selector);if(el)source.append(el)}
  const layout=document.createElement('div');layout.className='yy-studio-grid';form.before(layout);layout.append(source,form);
  const wrapper=document.createElement('section');wrapper.className='yy-studio yy-studio-import';wrapper.innerHTML=banner('建筑写入','把建筑带入世界','文件、参数、执行状态分区呈现；导入、多机器人与修补功能完整保留。');while(box.firstChild)wrapper.append(box.firstChild);return wrapper.outerHTML;
 };
 const exportBase=RexpHTML;
 RexpHTML=function(){
  const box=document.createElement('div');box.innerHTML=exportBase.apply(this,arguments);const form=box.firstElementChild;form.classList.add('yy-studio-config');
  const originalTitle=form.firstElementChild;if(originalTitle&&!originalTitle.id&&originalTitle.tagName==='DIV')originalTitle.remove();
  form.prepend(heading('01','框选读取区域','输入两个对角坐标，保留原格式和协作选项。'));
  const wrapper=document.createElement('section');wrapper.className='yy-studio yy-studio-export';wrapper.innerHTML=banner('建筑读取','保存世界里的作品','选择范围与格式，将原始建筑文件保存到当前执行设备。');
  const layout=document.createElement('div');layout.className='yy-studio-export-grid';form.before(layout);layout.append(form);
  const note=document.createElement('aside');note.className='card yy-studio-guide';note.innerHTML='<span class="yy-studio-kicker">导出说明</span><h3>从世界，到文件。</h3><ol><li>确认当前连接的服务器与账号。</li><li>填写区域和文件名，选择需要的格式。</li><li>等待任务完成后下载原文件。</li></ol><p>缺块、断线或失败时以任务实际结果为准；开始任务不等于导出完成。</p><button class="btn-s" onclick="yyOpenExportCenter()">打开文件下载</button>';layout.append(note);
  while(box.firstChild)wrapper.append(box.firstChild);return wrapper.outerHTML;
 };
 const pluginsBase=Rplg;
 Rplg=async function(){
  await pluginsBase.apply(this,arguments);const root=document.querySelector('#tab-tasks>.content-inner');if(!root||root.dataset.yyStudio==='1')return;
  root.dataset.yyStudio='1';root.classList.add('yy-plugin-studio');
  const oldTitle=root.querySelector(':scope>.yy-heading');if(oldTitle)oldTitle.remove();
  root.insertAdjacentHTML('afterbegin',banner('扩展工作台','插件，各就其位。','本地管理与市场来源分开，运行状态和操作结果始终可见。'));
  const cards=root.querySelector('.yy-workspace-cards'),head=cards?.previousElementSibling;
  if(cards&&head){const group=document.createElement('details');group.className='yy-studio-group';group.open=true;const summary=document.createElement('summary');summary.textContent='开发工作区';head.before(group);group.append(summary,head,cards)}
  const runtime=root.querySelector('.yy-installed-runtime')?.closest('section.card')||Array.from(root.children).find(e=>e.tagName==='SECTION'&&e.querySelector('#yy-runtime-result'));
  if(runtime){const d=document.createElement('details');d.className='yy-studio-group';d.open=true;d.innerHTML='<summary>运行环境与已安装实例</summary>';runtime.before(d);d.append(runtime)}
  const compat=root.querySelector('.yy-compat');if(compat){const d=document.createElement('details');d.className='yy-studio-group';d.open=true;d.innerHTML='<summary>Lua 与兼容插件</summary>';compat.before(d);d.append(compat);compat.querySelector(':scope>h3')?.remove()}
  const bar=document.createElement('section');bar.className='yy-plugin-updates card';bar.innerHTML='<div class="yy-section-head"><div><h3>来源与更新</h3><p class="yy-hint">只检查你已关联且有权访问的来源；检查不会安装或覆盖插件。</p></div><button class="btn-s" id="yy-plugin-check" onclick="yyCheckPluginUpdates()">检查更新</button></div><label class="yy-update-switch"><input id="yy-plugin-auto-check" type="checkbox" onchange="yyPluginAutoCheck(this.checked)">进入插件页时自动检查更新</label><p id="yy-plugin-update-result" role="status" class="yy-hint"></p><div class="yy-actions"><button class="btn-s" onclick="yyLibraryPage()">我的插件市场</button><button class="btn-s" onclick="pmOpenMarket()">外部插件市场</button><button class="btn-s" onclick="yyPluginSourceDialog()">关联已安装插件来源</button></div>';
  root.querySelector('.yy-plugin-header')?.after(bar);if(!bar.parentElement)root.append(bar);
  const pref=readPrefs();document.getElementById('yy-plugin-auto-check').checked=pref.auto===true;
  for(const group of root.querySelectorAll('.yy-studio-group')){const key=group.querySelector(':scope>summary').textContent;group.open=typeof pref.folds?.[key]==='boolean'?pref.folds[key]:key==='Lua 与兼容插件';group.addEventListener('toggle',()=>{if(!group.isConnected)return;const p=readPrefs();p.folds=p.folds||{};p.folds[key]=group.open;savePrefs(p)})}
  if(pref.auto===true&&Date.now()-(pref.lastCheck||0)>15*60*1000)yyCheckPluginUpdates();
 };
 function prefKey(){const user=typeof yyPlatformUser==='object'&&yyPlatformUser?yyPlatformUser:{};const account=typeof yyActiveAccount==='string'?yyActiveAccount:'main';let device='local';try{device=new URL(location.href).searchParams.get('device')||'local'}catch{}return 'yy-plugin-updates:'+JSON.stringify([device,account,String(user.id||user.username||'local-owner')])}
 function readPrefs(){try{const r=JSON.parse(localStorage.getItem(prefKey())||'{}');return r&&typeof r==='object'?r:{}}catch{return {}}}
 function savePrefs(p){try{localStorage.setItem(prefKey(),JSON.stringify(p));return true}catch{T('此设备无法保存更新偏好','e');return false}}
 window.yyPluginAutoCheck=function(on){const p=readPrefs();p.auto=on;savePrefs(p);if(on)yyCheckPluginUpdates()};
 window.yyPluginRecordSource=function(id,sourceID,origin){if(!id||!sourceID||!origin)return;const p=readPrefs();p.sources=p.sources||{};p.sources[id]={kind:'external',id:String(sourceID),origin};savePrefs(p)};
 window.yyPluginSourceDialog=async function(){
  const owner=yyReadScope(),r=await A('GET','/api/plugin/list');if(owner!==yyReadScope())return;if(!r.ok){T(r.error||'插件列表读取失败','e');return}
  const p=readPrefs(),rows=r.plugins||[],overlay=document.createElement('div');overlay.className='modal-overlay';
  overlay.innerHTML='<div class="modal-box yy-studio-source-dialog"><h3>关联更新来源</h3><p class="yy-hint">选择已安装插件，再填写授权市场中的数字插件ID。此记录只保存在当前设备和账号视图，不包含令牌，不会自动安装。</p><label>已安装插件<select id="yy-source-plugin">'+rows.map(p=>'<option value="'+escape(p.id)+'">'+escape(p.name||p.id)+'</option>').join('')+'</select></label><label>市场插件ID<input id="yy-source-market" inputmode="numeric" placeholder="例如 123"></label><p class="yy-hint" id="yy-source-result"></p><div class="yy-actions"><button class="btn" id="yy-source-save">保存来源</button><button class="btn-s" id="yy-source-remove">取消关联</button><button class="btn-s" id="yy-source-close">关闭</button></div></div>';document.body.append(overlay);
  const plugin=overlay.querySelector('#yy-source-plugin'),input=overlay.querySelector('#yy-source-market');const fill=()=>input.value=p.sources?.[plugin.value]?.id||'';fill();plugin.onchange=fill;
  overlay.querySelector('#yy-source-save').onclick=()=>{if(owner!==yyReadScope()){overlay.remove();return}if(!plugin.value||!/^\d{1,15}$/.test(input.value.trim())||!r.market_origin){overlay.querySelector('#yy-source-result').textContent='请选择插件，填写1～15位数字市场ID，并确认当前市场已配置。';return}yyPluginRecordSource(plugin.value,input.value.trim(),r.market_origin);overlay.remove();T('来源已关联；可检查版本并打开原市场核对。','o')};
  overlay.querySelector('#yy-source-remove').onclick=()=>{if(owner===yyReadScope()){const now=readPrefs();if(now.sources)delete now.sources[plugin.value];savePrefs(now)}overlay.remove()};overlay.querySelector('#yy-source-close').onclick=()=>overlay.remove();
 };
 let updateBusy=false;
 window.yyCheckPluginUpdates=async function(){
  const result=document.getElementById('yy-plugin-update-result'),button=document.getElementById('yy-plugin-check');if(!result||updateBusy)return;
  const owner=yyReadScope(),p=readPrefs();updateBusy=true;if(button)button.disabled=true;result.textContent='正在检查关联来源…';
  const live=()=>owner===yyReadScope()&&document.getElementById('yy-plugin-update-result')===result;
  try{
   const r=await A('GET','/api/plugin/list');if(!live())return;if(!r.ok)throw Error(r.error||'读取本地插件失败');const rows=r.plugins||[],lines=[];
   for(const plugin of rows){const source=p.sources?.[plugin.id];if(!source||source.kind!=='external'||!/^\d{1,15}$/.test(String(source.id)))continue;
    if(!r.market_origin||source.origin!==r.market_origin){const warning=document.createElement('div');warning.textContent=(plugin.name||plugin.id)+'：市场来源已变更，请重新关联，未向新地址发送旧来源查询。';lines.push(warning);continue}
    const currentOrigin=await A('GET','/api/plugin/list');if(!live())return;
    if(!currentOrigin.ok||currentOrigin.market_origin!==source.origin){const warning=document.createElement('div');warning.textContent=(plugin.name||plugin.id)+'：来源状态已变更或不可确认，请重新检查。';lines.push(warning);continue}
    const info=await APluginMarket('GET','/api/plugin-market/files/'+encodeURIComponent(source.id),null,{'X-YouYuan-Market-Origin':source.origin});if(!live())return;
    const line=document.createElement('div');line.className='yy-update-row';const text=document.createElement('span');line.append(text);
    if(!info.ok){text.textContent=(plugin.name||plugin.id)+'：来源不可用或未获授权，未检查成功。'}
    else{const release=info.versions?.[0],version=release?.version||info.file?.version;const known=typeof version==='string'&&version!=='';text.textContent=(plugin.name||plugin.id)+' · 当前 '+(plugin.version||'未声明版本')+' → 来源 '+(known?version:'未声明版本')+(known&&plugin.version===version?' · 版本一致':known&&plugin.version?' · 版本不同，请核对更新说明':' · 无法自动比较');if(release?.changelog){const small=document.createElement('small');small.textContent=String(release.changelog).slice(0,500);line.append(small)}const open=document.createElement('button');open.className='btn-s';open.textContent='查看来源';open.onclick=async()=>{if(owner!==yyReadScope())return;const current=await A('GET','/api/plugin/list');if(owner!==yyReadScope())return;if(!current.ok||current.market_origin!==source.origin){T('市场来源已变更，请重新检查更新','w');return}pmOpenDetail(Number(source.id),source.origin)};line.append(open)}lines.push(line);
   }
   if(!live())return;result.replaceChildren();if(!lines.length)result.textContent='尚未关联更新来源。本地导入的插件不会被猜测匹配；请先关联来源，或到市场查看。';else result.append(...lines);
   const current=readPrefs();current.lastCheck=Date.now();savePrefs(current);
  }catch(e){if(live())result.textContent='检查失败：'+e.message}finally{updateBusy=false;if(live()&&button)button.disabled=false}
 };
 // Built-in file browser reads the execution device; system upload is explicit.
 const uploadBase=pick;window.yyStudioUploadFile=function(type){return uploadBase(type==='zip'?'plugin':type)};
 const pickerBase=showFilePicker;showFilePicker=function(path,filter,select,mode,cancel){const native=typeof android!=='undefined'&&typeof android.pickFile==='function';return pickerBase(native?path:(path==='/storage/emulated/0'?'':path),filter,select,mode,cancel)};
 fpicker=function(type){if((type==='building'||type==='skin')&&typeof showFilePicker==='function')return new Promise(resolve=>showFilePicker('/storage/emulated/0',type,resolve,null,()=>resolve('')));return uploadBase(type)};
 pickBySetting=function(type,callback,mode){showFilePicker('/storage/emulated/0',type,callback,mode)};
 const qqBase=yyQQPage;yyQQPage=async function(){await qqBase.apply(this,arguments);const panel=document.getElementById('yy-qq-assistant-panel');if(!panel||document.getElementById('yy-qq-memory-panel'))return;const owner=yyReadScope(),r=await Promise.all([A('GET','/api/qq/config'),A('GET','/api/ai/models'),A('GET','/api/qq/groups')]);if(owner!==yyReadScope()||panel!==document.getElementById('yy-qq-assistant-panel'))return;if(r.some(x=>!x.ok))return;await yyQQMemoryMount(r[0].config?.assistant||{},r[1].models||[],r[2].groups||[])};
})();
