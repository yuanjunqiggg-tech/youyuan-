async function yyHostFilesPage(){
 delete subPageCache['执行设备文件'];openSubPage('执行设备文件',yyIntro('工具箱文件','文件上传到当前选择的执行设备。选择 Linux 时保存在云服务器自己的磁盘。')+'<section class="card"><p id="yy-host-file-location" class="yy-hint">正在读取执行设备…</p><input type="file" id="yy-host-file-input" multiple><div class="yy-actions"><button class="btn" id="yy-host-file-submit" onclick="yyUploadHostFiles()">上传所选文件</button><button class="btn-s" onclick="yyRefreshHostFiles()">刷新文件列表</button></div><p class="yy-hint">支持源码、TXT、JSON、日志、ZIP 和其他文件。按块校验本次文件内容；续传时逐块核对已确认的上传前缀，中断后可重新选择同一文件继续。单文件最多2GiB，未完成记录保留24小时。上传不会自动运行或覆盖已有文件。</p><div id="yy-host-file-status" role="status"></div><div id="yy-host-file-list"></div></section>');await yyRefreshHostFiles();
}
async function yyRefreshHostFiles(){
 const scope=yyReadScope(),list=document.getElementById('yy-host-file-list'),location=document.getElementById('yy-host-file-location');if(!list||!location)return;
 const r=await A('GET','/api/youyuan/files');if(scope!==yyReadScope()||document.getElementById('yy-host-file-list')!==list||document.getElementById('yy-host-file-location')!==location)return;
 if(!r.ok){T(r.error,'e');return}location.textContent=r.platform+' · '+r.directory;list.replaceChildren();
 for(const f of r.items){const row=document.createElement('div');row.className='yy-lib-row';const info=document.createElement('div');info.className='yy-lib-info';const title=document.createElement('strong');title.textContent=f.name;const note=document.createElement('small');note.textContent=(f.bytes/1024).toFixed(1)+' KB · '+f.modified;info.append(title,note);const button=document.createElement('button');button.className='yy-secondary';button.textContent='下载到本地';button.onclick=async()=>{if(scope!==yyReadScope()){T('操作账号已切换，请刷新文件列表','e');return}button.disabled=true;try{const response=await fetch('/api/youyuan/files/download?name='+encodeURIComponent(f.name));if(scope!==yyReadScope())return;if(!response.ok){T('下载失败','e');return}const blob=await response.blob();if(scope!==yyReadScope())return;const url=URL.createObjectURL(blob),link=document.createElement('a');link.href=url;link.download=f.name;link.click();setTimeout(()=>URL.revokeObjectURL(url),30000)}catch(error){if(scope===yyReadScope())T(error.message,'e')}finally{button.disabled=false}};row.append(info,button);list.append(row)}
 if(!r.items.length)list.textContent='尚未上传文件。';
}
var yyHostUploadBusy=false;
async function yyUploadHostFiles(){
 if(yyHostUploadBusy){T('所选文件正在上传，请等待结果','w');return}
 const input=document.getElementById('yy-host-file-input'),status=document.getElementById('yy-host-file-status'),button=document.getElementById('yy-host-file-submit');
 if(!input?.files.length){T('请选择文件','w');return}const files=Array.from(input.files),scope=yyReadScope();
 const live=()=>scope===yyReadScope()&&document.getElementById('yy-host-file-status')===status;
 yyHostUploadBusy=true;input.disabled=true;if(button)button.disabled=true;
 try{
  for(const file of files){if(!live())throw Error('操作账号或页面已切换，剩余上传已暂停');status.textContent='正在上传 '+file.name;
   const r=await yyUploadFile(file,{onProgress:p=>{if(live())status.textContent=p.name+' · '+p.message+' · '+(p.offset/1048576).toFixed(1)+' / '+(p.total/1048576).toFixed(1)+' MiB'}});
   if(!r.ok)throw Error(r.error||'上传未确认');if(!live())return;status.textContent='已保存 '+r.name+' · '+r.bytes+' 字节';
  }
  if(live()){input.value='';await yyRefreshHostFiles()}
 }catch(error){if(live())status.textContent='上传已停止：'+error.message}
 finally{yyHostUploadBusy=false;input.disabled=false;if(button)button.disabled=false}
}
