/* Resumable file bytes only. Never retry import/placement/game commands. */
(function(){
 const CHUNK=1024*1024;
 const hash=async blob=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',await blob.arrayBuffer())),n=>n.toString(16).padStart(2,'0')).join('');
 function ticketId(){return Array.from(crypto.getRandomValues(new Uint8Array(16)),n=>n.toString(16).padStart(2,'0')).join('')}
 function readStore(key){try{return JSON.parse(localStorage.getItem(key)||'null')}catch{return null}}
 function saveStore(key,value){try{if(value)localStorage.setItem(key,JSON.stringify(value));else localStorage.removeItem(key)}catch{}}
 function pause(ms){return new Promise(resolve=>setTimeout(resolve,ms))}
 function invalid(message){const error=Error(message);error.status=409;error.nonRetryable=true;return error}
 function platformID(){return typeof yyPlatformUser!=='undefined'&&yyPlatformUser&&typeof yyPlatformUser.id==='string'?yyPlatformUser.id:''}
 function cloudKey(){try{return sessionStorage.getItem('yy-cloud-key')||'local'}catch{return 'local'}}
 window.yyUploadFile=async function(file,options){
  options=options||{};
  if(!file||!Number.isSafeInteger(file.size)||file.size<0||file.size>2*1024*1024*1024)throw Error('请选择不超过2GiB的文件');
  if(!crypto.subtle)throw Error('当前页面缺少安全校验能力，请使用HTTPS或应用内页面');
  const account=typeof yyActiveAccount==='string'?yyActiveAccount:'main',owner=platformID();
  const base=typeof yyAccountPath==='function'?yyAccountPath('/api/youyuan/files/transfer',account):'/api/youyuan/files/transfer';
  const device=new URLSearchParams(location.search).get('device')||'local',credential=cloudKey();
  const identity=JSON.stringify([location.origin,device,account,owner?'user:'+owner:'key:'+credential]),scope=await hash(new Blob([identity]));
  // The selector fingerprint is sampled. Previously acknowledged bytes are
  // compared one chunk at a time; not-yet-uploaded bytes come from this File.
  const fingerprint=await hash(new Blob([file.name,'|',String(file.size),'|',String(file.lastModified),'|',await hash(file.slice(0,CHUNK)),'|',await hash(file.slice(Math.max(0,file.size-CHUNK)))]));
  const key='yy-upload:'+scope.slice(0,16)+':'+fingerprint;
  let saved=readStore(key),upload,verifiedOffset=0;
  if(!saved){const legacyScope=await hash(new Blob([[location.origin,device,account,credential].join('|')]));saved=readStore('yy-upload:'+legacyScope.slice(0,16)+':'+fingerprint);if(saved)saveStore(key,saved)}
  if(saved&&!/^[a-f0-9]{32}$/.test(saved.id||'')){saveStore(key,null);saved=null}
  const progress=message=>{if(options.onProgress)options.onProgress({name:file.name,total:file.size,offset:upload?.offset||0,message})};
  function active(){
   if(options.signal?.aborted){const e=Error('已暂停上传，重新选择同一文件可从已确认位置继续');e.status=499;e.nonRetryable=true;throw e}
   if((typeof yyActiveAccount==='string'?yyActiveAccount:'main')!==account||(owner?platformID()!==owner:cloudKey()!==credential)){const e=Error('操作账号已切换，上传已暂停；请切回原账号后继续');e.status=499;e.nonRetryable=true;throw e}
  }
  function validateTicket(t){
   if(!t||t.id!==saved.id||t.name!==file.name||t.size!==file.size||!Number.isSafeInteger(t.offset)||t.offset<0||t.offset>file.size||typeof t.complete!=='boolean'||!Array.isArray(t.chunks))throw invalid('续传记录与当前文件不一致，未确认上传成功');
   return t;
  }
  function completed(t,resumed){
   active();validateTicket(t);
   if(!t.complete||t.offset!==file.size||typeof t.path!=='string'||!t.path.trim()||t.path.includes('\0')||!t.path.split(/[\\/]/).pop()||!/^[a-f0-9]{64}$/.test(t.sha256||''))throw invalid('上传结果缺少有效路径、字节数或SHA-256，未确认文件保存成功');
   progress('逐块内容校验通过，文件已保存');return {ok:true,path:t.path,name:t.path.split(/[\\/]/).pop(),bytes:t.size,sha256:t.sha256,...(resumed?{resumed:true}:{})};
  }
  async function request(method,suffix,body){
   active();const control=new AbortController(),timer=setTimeout(()=>control.abort(),45000),abort=()=>control.abort();options.signal?.addEventListener('abort',abort,{once:true});
   try{
    const response=await fetch(base+(suffix||''),{method,headers:{'Content-Type':body instanceof Blob?'application/octet-stream':'application/json'},body:body===undefined?undefined:(body instanceof Blob?body:JSON.stringify(body)),signal:control.signal});
    const result=await response.json().catch(()=>({ok:false,error:'上传接口未返回有效结果'}));active();
    if(!response.ok||!result.ok){const e=Error(result.error||'上传失败');e.status=response.status;throw e}return result;
   }catch(error){active();throw error}finally{clearTimeout(timer);options.signal?.removeEventListener('abort',abort)}
  }
  async function state(){return validateTicket((await request('GET','?id='+saved.id)).upload)}
  async function verifyPrefix(t){
   validateTicket(t);let expected=0;
   for(const c of t.chunks){active();if(!Number.isSafeInteger(c.offset)||c.offset!==expected||!Number.isSafeInteger(c.size)||c.size<=0||c.size>CHUNK||c.offset+c.size>file.size||!/^[a-f0-9]{64}$/.test(c.sha256||'')||await hash(file.slice(c.offset,c.offset+c.size))!==c.sha256)throw invalid('本地文件与已确认的上传前缀不同，请重新选择原文件');expected+=c.size}
   if(expected!==t.offset)throw invalid('续传记录的已确认分块与字节位置不一致');verifiedOffset=t.offset;
  }
  active();
  if(saved){try{upload=await state();progress('正在逐块核对已确认的上传前缀…');await verifyPrefix(upload)}catch(e){if(e.status===404){saveStore(key,null);saved=null}else throw e}}
  if(!saved){saved={id:ticketId()};saveStore(key,saved);progress('正在建立上传…');
   try{upload=validateTicket((await request('POST','',{id:saved.id,name:file.name,size:file.size})).upload)}
   catch(e){if(e.nonRetryable)throw e;try{upload=await state()}catch(check){if(check.nonRetryable)throw check;throw Error('暂未确认上传连接；重新选择同一文件可继续，不会重复导入。'+e.message)}}
   if(upload.offset)await verifyPrefix(upload);
  }
  if(upload.complete)return completed(upload,true);
  let failures=0;
  while(upload.offset<file.size){
   active();const offset=upload.offset,chunk=file.slice(offset,offset+CHUNK),digest=await hash(chunk);
   progress('正在逐块上传 '+Math.floor(offset/Math.max(1,file.size)*100)+'%');
   try{const next=validateTicket((await request('PUT','?id='+saved.id+'&offset='+offset+'&sha256='+digest,chunk)).upload);if(next.offset<offset+chunk.size)throw invalid('服务器未确认当前分块，上传尚未完成');if(next.offset!==offset+chunk.size)await verifyPrefix(next);upload=next;verifiedOffset=upload.offset;failures=0}
   catch(e){
    if(e.nonRetryable||e.status&&e.status<500&&e.status!==429)throw e;
    if(++failures>4)throw Error('上传已暂停；重新选择同一文件即可从已确认位置继续。'+e.message);
    progress('连接中断，正在核对已接收位置…');await pause(400*failures);
    try{const latest=await state();if(latest.offset!==verifiedOffset)await verifyPrefix(latest);upload=latest}catch(check){if(check.nonRetryable||check.status&&check.status<500&&check.status!==429)throw check}
   }
  }
  progress('正在由执行设备复核已接收分块并计算文件SHA-256…');
  try{upload=validateTicket((await request('POST','?id='+saved.id)).upload)}catch(e){if(e.nonRetryable)throw e;const latest=await state();if(!latest.complete)throw e;upload=latest}
  return completed(upload,false);
 };
})();
