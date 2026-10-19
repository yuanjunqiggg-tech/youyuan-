/* Authenticated, account-aware downloads; exported ZIPs stay on the execution host. */
function yyExportPath(path, account) {
  return account && account !== 'main' ? yyAccountPath(path, account) : path;
}
async function yyDownloadExport(item, account) {
  const response = await yyRootFetch(yyExportPath(item.download_url, account), {method:window.android?.exportDownload?'HEAD':'GET'});
  const attachment=(response.headers.get('Content-Disposition') || '').toLowerCase().startsWith('attachment');
  const verifiedZip=/\.zip$/i.test(item.name||'')&&/^[a-f0-9]{64}$/i.test(item.sha256||'')&&/application\/(zip|octet-stream)/i.test(response.headers.get('Content-Type')||'');
  if (!response.ok || (!attachment&&!verifiedZip)) {
    let reason='HTTP '+response.status+'，返回类型不符合文件下载';
    if((response.headers.get('Content-Type')||'').includes('json')){try{const r=await response.json();reason=r.error||reason}catch(_){}}
    throw Error('下载失败：'+reason);
  }
  if(window.android?.exportDownload) {
    const headers={};const key=sessionStorage.getItem('yy-cloud-key');if(key)headers.Authorization='Bearer '+key;
    const device=new URLSearchParams(location.search).get('device');if(device)headers['X-YouYuan-Device']=device;
    await response.body?.cancel();
    android.exportDownload(response.url,JSON.stringify(headers),item.name||'youyuan-export');
    return;
  }
  const blob = await response.blob();
  if (item.snapshot !== false && item.bytes != null && blob.size !== Number(item.bytes)) throw Error('下载大小不匹配，请重试');
  if(item.sha256&&globalThis.crypto?.subtle){const hash=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',await blob.arrayBuffer())),b=>b.toString(16).padStart(2,'0')).join('');if(hash!==item.sha256.toLowerCase())throw Error('下载校验失败，文件内容不完整，请重新下载');}
  const href = URL.createObjectURL(blob), link = document.createElement('a');
  link.href = href; link.download = item.name || 'youyuan-export';
  document.body.append(link); link.click(); link.remove();
  setTimeout(() => URL.revokeObjectURL(href), 60000);
}
async function yyExportPluginZip(id, runtime) {
  const account = window.yyActiveAccount || 'main';
  const path = runtime ? '/api/youyuan/runtime/export?id=' + encodeURIComponent(id) : '/api/plugin/export';
  const result = await yyRootAPI('POST', yyExportPath(path, account), {id});
  if (!result.ok) throw Error(result.error || '导出失败');
  if (window.android && android.exportFile && !android.exportDownload) {
    android.exportFile(result.src_path || result.path, result.name || id + '.zip');
  } else {
    if (!result.export) throw Error('执行端版本尚未提供下载入口，请更新后重试');
    await yyDownloadExport(result.export, account);
  }
  T('完整插件备份已导出，可在下载中心的‘插件备份’页再次下载', 'o');
}
plgExportZip = async function(id) {
  try { await yyExportPluginZip(id, false); } catch (e) { T(e.message, 'e'); }
};
yyExportRuntimePlugin = async function(id) {
  try { await yyExportPluginZip(id, true); } catch (e) { T(e.message, 'e'); }
};
const yyExportSAFPrevious = plgExportSAF;
plgExportSAF = async function(id) {
  if (window.android && android.exportFile && !android.exportDownload) return yyExportSAFPrevious(id);
  return plgExportZip(id);
};

function yyExportCenterLink(account) {
  const u = new URL(location.href);
  u.hash = ''; u.searchParams.set('exports', '1'); u.searchParams.set('export_account', account || 'main');
  return u.href;
}
async function yyOpenExportCenter(account) {
  if (document.getElementById('yy-export-center')) return;
  account = account || window.yyActiveAccount || 'main';
  const overlay = document.createElement('div'); overlay.className = 'modal-overlay'; overlay.id = 'yy-export-center';
  const box = document.createElement('section'); box.className = 'modal-box';
  box.style.cssText = 'width:min(92vw,760px);max-width:760px;max-height:86vh;overflow:auto;text-align:left';
  const header = document.createElement('div'); header.className = 'yy-section-head';
  const title = document.createElement('h2'); title.textContent = '插件文件下载';
  const close = document.createElement('button'); close.className = 'btn-s'; close.textContent = '关闭'; close.onclick = () => overlay.remove();
  header.append(title, close);
  const note = document.createElement('p'); note.className = 'yy-hint';
  note.textContent = '当前账号：' + account + '。完整 ZIP 保存在执行设备上，包含插件源码、配置及数据；这里只删除备份，不删除插件。';
  const tabs = document.createElement('div'); tabs.className = 'yy-actions';
  const outputs = document.createElement('button'); outputs.className='btn'; outputs.textContent='插件生成文件';
  const backups = document.createElement('button'); backups.className='btn-s'; backups.textContent='完整插件备份';
  const picker = document.createElement('select'); picker.style.cssText='width:100%;margin:14px 0;min-height:44px'; picker.setAttribute('aria-label','选择插件');
  tabs.append(outputs,backups); let mode='outputs';
  const actions = document.createElement('div'); actions.className = 'yy-actions';
  const refresh = document.createElement('button'); refresh.className = 'btn-s'; refresh.textContent = '刷新';
  const copy = document.createElement('button'); copy.className = 'btn-s'; copy.textContent = '复制下载中心网址';
  copy.onclick = async () => { try { await navigator.clipboard.writeText(yyExportCenterLink(account)); T('已复制，打开后需登录自己的账号', 'o'); } catch (_) { T('复制失败，请复制浏览器地址', 'e'); } };
  actions.append(refresh, copy);
  const list = document.createElement('div'); list.setAttribute('aria-live', 'polite');
  box.append(header, note, tabs, picker, actions, list); overlay.append(box); document.body.append(overlay);
  overlay.onclick = e => { if (e.target === overlay) overlay.remove(); };
  async function load() {
    refresh.disabled = true; picker.hidden = mode!=='outputs';
    note.textContent=mode==='outputs'?'当前账号：'+account+'。下载插件在执行设备上生成的建筑、NBT、日志、数据或ZIP，保留原格式。':'完整插件备份包含源码、配置及数据；只删除备份，不删除插件。';
    list.textContent = '正在读取文件…';
    try {
      if(mode==='outputs'){await yyLoadPluginOutputs(picker,list,account);return;}
      const result = await yyRootAPI('GET', yyExportPath('/api/youyuan/exports', account));
      if (!result.ok) throw Error(result.error || '请先登录并进入该账号工作区');
      list.replaceChildren();
      if (!(result.items || []).length) { list.textContent = '暂无导出。到插件列表选择“备份插件”。'; return; }
      for (const item of result.items) {
        const row = document.createElement('article'); row.className = 'card';
        const name = document.createElement('strong'); name.textContent = item.name;
        const details = document.createElement('p'); details.className = 'yy-hint';
        details.textContent = new Date(item.created_at).toLocaleString('zh-CN') + ' · ' + (item.bytes / 1048576).toFixed(2) + ' MB · ' + item.kind;
        const hash = document.createElement('code'); hash.textContent = 'SHA256 ' + item.sha256;
        hash.style.cssText = 'display:block;overflow-wrap:anywhere;font-size:11px;opacity:.7;margin-bottom:12px';
        const buttons = document.createElement('div'); buttons.className = 'yy-actions';
        const download = document.createElement('button'); download.className = 'btn'; download.textContent = '下载完整 ZIP';
        download.onclick = async () => { download.disabled = true; try { await yyDownloadExport(item, account); T('已交给浏览器下载', 'o'); } catch (e) { T(e.message, 'e'); } finally { download.disabled = false; } };
        const remove = document.createElement('button'); remove.className = 'btn-s'; remove.textContent = '删除此备份';
        remove.onclick = async () => {
          if (!confirm('只删除此导出备份，保留插件。继续吗？')) return;
          remove.disabled = true;
          const r = await yyRootAPI('POST', yyExportPath('/api/youyuan/exports/remove', account), {id: item.id});
          if (r.ok) row.remove(); else { T(r.error || '删除失败', 'e'); remove.disabled = false; }
        };
        buttons.append(download, remove); row.append(name, details, hash, buttons); list.append(row);
      }
    } catch (e) { list.textContent = e.message; } finally { refresh.disabled = false; }
  }
  refresh.onclick = load; picker.onchange=load;
  outputs.onclick=()=>{mode='outputs';outputs.className='btn';backups.className='btn-s';load();};
  backups.onclick=()=>{mode='backups';backups.className='btn';outputs.className='btn-s';load();};
  await load();
}
const yyPluginsExportCenterPrevious = Rplg;
Rplg = async function() {
  await yyPluginsExportCenterPrevious();
  const host = document.querySelector('#tab-tasks .content-inner');
  if (!host || host.querySelector('.yy-export-center-button')) return;
  const row = document.createElement('div'); row.className = 'yy-actions yy-export-center-button';
  const button = document.createElement('button'); button.className = 'btn-s'; button.textContent = '插件文件下载'; button.onclick = () => yyOpenExportCenter();
  row.append(button); host.prepend(row);
};
if (new URL(location.href).searchParams.get('exports') === '1') {
  window.addEventListener('load', function() {
    let attempts = 0;
    const timer = setInterval(() => {
      if (++attempts > 120) { clearInterval(timer); return; }
      if (document.getElementById('yy-identity-gate')) return;
      clearInterval(timer); yyOpenExportCenter(new URL(location.href).searchParams.get('export_account') || 'main');
    }, 1000);
  }, {once: true});
}

async function yyDownloadPluginFile(id,file,kind,account,runtimeAccount) {
  try {
    const q=new URLSearchParams({id,file,kind:kind||'lua'}); if(runtimeAccount)q.set('runtime_account',runtimeAccount);
    const r=await yyRootAPI('GET',yyExportPath('/api/youyuan/plugin-outputs/info?'+q,account));
    if(!r.ok)throw Error(r.error||'文件不可读取');
    await yyDownloadExport(r.file,account); T('文件已交给浏览器下载','o');
  }catch(e){T(e.message,'e');}
}
// Subpages restore cached HTML: use delegation so download buttons still work
// after opening a text editor and returning to the directory.
document.addEventListener('click',function(e){
  const b=e.target.closest?.('button[data-output-file]');if(!b)return;
  e.preventDefault();e.stopPropagation();
  yyDownloadPluginFile(decodeURIComponent(b.dataset.outputPlugin),decodeURIComponent(b.dataset.outputFile),'lua',decodeURIComponent(b.dataset.outputAccount));
},true);
async function yyLoadPluginOutputs(picker,list,account) {
  if(!picker.options.length){
    const result=await yyRootAPI('GET',yyExportPath('/api/plugin/list',account));
    if(!result.ok)throw Error(result.error||'插件目录不可读取');
    for(const p of result.plugins||[]){const o=document.createElement('option');o.value=JSON.stringify({id:p.id,kind:'lua'});o.textContent=p.name||p.id;picker.append(o);}
    const runtime=await yyRootAPI('GET',yyExportPath('/api/youyuan/runtime/list',account));
    for(const p of runtime.installed||[]){const o=document.createElement('option');o.value=JSON.stringify({id:p.id,kind:'runtime',runtime_account:'main'});o.textContent=(p.name||p.id)+' · '+p.language;picker.append(o);}
  }
  list.replaceChildren();
  if(!picker.value){list.textContent='该账号尚未安装插件。';return;}
  const q=new URLSearchParams(JSON.parse(picker.value));
  const result=await yyRootAPI('GET',yyExportPath('/api/youyuan/plugin-outputs?'+q,account));
  if(!result.ok)throw Error(result.error||'读取生成文件失败');
  if(!result.items?.length){list.textContent='此插件尚未生成文件。运行插件的导出或打包功能后，点击刷新。';return;}
  for(const item of result.items){
    const row=document.createElement('article');row.className='card';
    const name=document.createElement('strong');name.textContent=item.path;name.style.overflowWrap='anywhere';
    const info=document.createElement('p');info.className='yy-hint';info.textContent=new Date(item.modified_at).toLocaleString('zh-CN')+' · '+(item.bytes/1024).toFixed(1)+' KB';
    const button=document.createElement('button');button.className='btn-s';button.textContent='下载文件';
    button.onclick=async()=>{button.disabled=true;try{await yyDownloadExport(item,account);T('文件已交给浏览器下载','o');}catch(e){T(e.message,'e');}finally{button.disabled=false;}};
    row.append(name,info,button);list.append(row);
  }
}
