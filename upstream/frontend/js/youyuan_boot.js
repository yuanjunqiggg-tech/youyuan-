// 云端同源前端用用户口令访问 APK；本机 WebView 不需二次口令。
(function(){
 try{localStorage.setItem('prism_wizard_done','true')}catch(error){}
 if(location.hostname==='127.0.0.1'||location.hostname==='localhost')return;
 if(['toolbox.youyuanqi.dpdns.org','youyuan-apk-v030.yuanjunqiggg.workers.dev'].includes(location.hostname)){
  var selected=new URL(location.href);if(!selected.searchParams.get('device')||selected.searchParams.get('device')==='primary-device'){selected.searchParams.set('device','linux-server');history.replaceState(null,'',selected)}
 }
 var originalFetch=window.fetch.bind(window);
 window.yyLinuxDirectOrigin='';
 var discovering=null,discoveryRetry=null;
 function discoverLinux(){
  var token=sessionStorage.getItem('yy-cloud-key');if(!token||discovering)return discovering;
  clearTimeout(discoveryRetry);discovering=originalFetch('/api/cloud/linux-direct?device=linux-server',{headers:{Authorization:'Bearer '+token}}).then(function(r){return r.json()}).then(function(d){if(d.ok&&/^https:\/\/(?:[a-z0-9-]+\.trycloudflare\.com|linux\.youyuanqi\.dpdns\.org)$/.test(d.origin))window.yyLinuxDirectOrigin=d.origin}).catch(function(){}).finally(function(){discovering=null;if(!window.yyLinuxDirectOrigin)discoveryRetry=setTimeout(discoverLinux,60000)});return discovering;
 }
 window.yyDiscoverLinux=discoverLinux;discoverLinux();
 window.fetch=function(input,options){
  var path=typeof input==='string'?input:(input&&input.url)||'';
  if(window.yyCloudBlocked&&path.startsWith('/api/')&&!path.startsWith('/api/identity/'))return Promise.resolve(new Response(JSON.stringify({ok:false,error:'云端订阅未开通或已到期',code:'SUBSCRIPTION_REQUIRED'}),{status:403,headers:{'Content-Type':'application/json'}}));
  if(path.startsWith('/api/')||path.startsWith('/mcp')||path==='/health'){
   options=Object.assign({},options);var headers=new Headers(options.headers||{});if(path.includes('market-proxy')&&headers.has('Authorization'))headers.set('X-YouYuan-Market-Authorization',headers.get('Authorization'));var key=sessionStorage.getItem('yy-cloud-key');if(key)headers.set('Authorization','Bearer '+key);var device=new URLSearchParams(location.search).get('device');if(device&&/^[A-Za-z0-9_-]{1,48}$/.test(device))headers.set('X-YouYuan-Device',device);options.headers=headers;
  }
  var direct=window.yyLinuxDirectOrigin&&(path.startsWith('/api/')||path==='/health')&&!path.startsWith('/api/identity/')&&!path.startsWith('/api/cloud/')&&!path.startsWith('/api/library/')&&new URLSearchParams(location.search).get('device')==='linux-server';
  return originalFetch(direct?window.yyLinuxDirectOrigin+path:input,options).catch(function(error){
   if(!direct)throw error;window.yyLinuxDirectOrigin='';discoverLinux();
   // A transport failure must never replay a write which may already have run.
   if(((options||{}).method||'GET').toUpperCase()==='GET')return originalFetch(input,options);
   throw Error('Linux直连中断，操作结果可能未知，请先检查状态再重试');
  }).then(function(response){if(response.status===403&&path.startsWith('/api/')&&!path.startsWith('/api/identity/'))response.clone().json().then(function(data){if(data.code==='SUBSCRIPTION_REQUIRED'){window.yyCloudBlocked=true;if(typeof yySubscriptionGate==='function'&&data.user)yySubscriptionGate(data.user)}}).catch(function(){});return response});
 };
 // 云端用增量事件批次，避免把永久 SSE 当成一次请求缓冲。
 window.EventSource=function(path){
  this.listeners={};this.closed=false;this.cursor=0;var self=this;
  this.addEventListener=function(name,cb){(self.listeners[name]||(self.listeners[name]=[])).push(cb)};
  this.close=function(){self.closed=true;clearTimeout(self.timer)};
  var opened=false;
  async function poll(){if(self.closed)return;try{
   var response=await fetch('/api/youyuan/events?after='+self.cursor);var data=await response.json();if(!response.ok||!data.ok)throw Error(data.error||'设备尚未上线');
   if(!opened){opened=true;if(self.onopen)self.onopen({})}
   (data.events||[]).forEach(function(event){self.cursor=Math.max(self.cursor,event.seq);var message={data:typeof event.data==='string'?event.data:JSON.stringify(event.data)};(self.listeners[event.type]||[]).forEach(function(cb){cb(message)})});
  }catch(error){if(self.onerror)self.onerror(error)}self.timer=setTimeout(poll,1500)}
  self.timer=setTimeout(poll,100);
 };
})();
