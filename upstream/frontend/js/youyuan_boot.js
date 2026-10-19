// 云端同源前端用用户口令访问 APK；本机 WebView 不需二次口令。
(function(){
 try{localStorage.setItem('prism_wizard_done','true')}catch(error){}
 if(location.hostname==='127.0.0.1'||location.hostname==='localhost')return;
 if(['toolbox.youyuanqi.dpdns.org','youyuan-apk-v030.yuanjunqiggg.workers.dev'].includes(location.hostname)){
  var selected=new URL(location.href);if(!selected.searchParams.get('device')||selected.searchParams.get('device')==='primary-device'){selected.searchParams.set('device','linux-server');history.replaceState(null,'',selected)}
 }
 var originalFetch=window.fetch.bind(window);
 window.yyLinuxDirectOrigin='';
 var discovering=null,discoveryRetry=null,directRetryAt=0;
 window.yyLinuxTransportStatus={mode:'relay',failures:0,last_error:'',updated_at:Date.now()};
 function directFailed(reason){window.yyLinuxDirectOrigin='';directRetryAt=Date.now()+30000;window.yyLinuxTransportStatus={mode:'relay',failures:window.yyLinuxTransportStatus.failures+1,last_error:reason,updated_at:Date.now()};discoverLinux();}
 function discoverLinux(){
  var token=sessionStorage.getItem('yy-cloud-key');if(!token||discovering)return discovering;
  clearTimeout(discoveryRetry);if(Date.now()<directRetryAt){discoveryRetry=setTimeout(discoverLinux,directRetryAt-Date.now());return Promise.resolve();}
  var controller=new AbortController(),deadline=setTimeout(function(){controller.abort()},8000);
  discovering=originalFetch('/api/cloud/linux-direct?device=linux-server',{headers:{Authorization:'Bearer '+token},signal:controller.signal}).then(function(r){return r.json()}).then(function(d){if(d.ok&&d.use_relay===true){window.yyLinuxDirectOrigin='';window.yyLinuxTransportStatus.mode='relay';window.yyLinuxTransportStatus.updated_at=Date.now();}else if(d.ok&&/^https:\/\/(?:[a-z0-9-]+\.trycloudflare\.com|linux\.youyuanqi\.dpdns\.org)$/.test(d.origin)){window.yyLinuxDirectOrigin=d.origin;window.yyLinuxTransportStatus.mode='direct';window.yyLinuxTransportStatus.updated_at=Date.now();}}).catch(function(){window.yyLinuxTransportStatus.last_error='直连入口发现暂不可用，继续中继';}).finally(function(){clearTimeout(deadline);discovering=null;if(!window.yyLinuxDirectOrigin)discoveryRetry=setTimeout(discoverLinux,60000)});return discovering;
 }
 window.yyDiscoverLinux=discoverLinux;discoverLinux();
 window.fetch=function(input,options){
  var path=typeof input==='string'?input:(input&&input.url)||'';
  if(window.yyCloudBlocked&&path.startsWith('/api/')&&!path.startsWith('/api/identity/'))return Promise.resolve(new Response(JSON.stringify({ok:false,error:'云端订阅未开通或已到期',code:'SUBSCRIPTION_REQUIRED'}),{status:403,headers:{'Content-Type':'application/json'}}));
  if(path.startsWith('/api/')||path.startsWith('/mcp')||path==='/health'){
   options=Object.assign({},options);var headers=new Headers(options.headers||{});if(path.includes('market-proxy')&&headers.has('Authorization'))headers.set('X-YouYuan-Market-Authorization',headers.get('Authorization'));var key=sessionStorage.getItem('yy-cloud-key');if(key)headers.set('Authorization','Bearer '+key);var device=new URLSearchParams(location.search).get('device');if(device&&/^[A-Za-z0-9_-]{1,48}$/.test(device))headers.set('X-YouYuan-Device',device);options.headers=headers;
  }
  var direct=window.yyLinuxDirectOrigin&&(path.startsWith('/api/')||path==='/health')&&!path.startsWith('/api/identity/')&&!path.startsWith('/api/cloud/')&&!path.startsWith('/api/library/')&&new URLSearchParams(location.search).get('device')==='linux-server';
  var relayed=false,isRead=['GET','HEAD'].includes(((options||{}).method||(input&&input.method)||'GET').toUpperCase());
  function cancelled(){return options&&options.signal&&options.signal.aborted||input&&input.signal&&input.signal.aborted;}
  function reusableBody(){
   var body=options&&options.body;
   if(body===undefined&&input&&input.body)return false;
   if(body==null||typeof body==='string')return true;
   return /^\[object (?:URLSearchParams|Blob|File|FormData|ArrayBuffer|(?:Uint|Int|Float|BigInt|BigUint)\d+(?:Clamped)?Array|DataView)\]$/.test(Object.prototype.toString.call(body));
  }
  return originalFetch(direct?window.yyLinuxDirectOrigin+path:input,options).then(async function(response){
   if(!direct)return response;
   if(response.status===401){
    directFailed('直连身份不适用，已切换云端中继');
    if(cancelled())return response;
    if(isRead){relayed=true;return originalFetch(input,options);}
    // Only the gateway's explicit pre-dispatch rejection proves a write did
    // not run. Other 401/5xx responses and transport failures are ambiguous.
    var rejection=null;try{rejection=await response.clone().json()}catch{}
    if(rejection&&rejection.code==='DIRECT_ACTOR_UNSUPPORTED'&&rejection.outcome==='not_sent'&&reusableBody()&&!cancelled()){relayed=true;return originalFetch(input,options);}
   }else if([502,503,504,521,522,523,524].includes(response.status)){
    directFailed('直连HTTP '+response.status);
    if(isRead&&!cancelled()){relayed=true;return originalFetch(input,options);}
   }
   return response;
  }).catch(function(error){
   // Page navigation, cancelled previews and explicitly stopped jobs abort
   // requests routinely. They are not evidence of a Linux connection outage.
   if(error.name==='AbortError'||cancelled())throw error;
   if(!direct||relayed)throw error;directFailed('直连网络请求失败');
   // A transport failure must never replay a write which may already have run.
   if(isRead){relayed=true;return originalFetch(input,options);}
   var unknown=Error('Linux直连中断，操作结果可能未知，请先检查状态再重试');unknown.code='LINUX_DIRECT_RESULT_UNKNOWN';unknown.outcome='unknown';unknown.retryable=false;throw unknown;
  }).then(async function(response){
   if(response.status===401&&(!direct||relayed)&&path.startsWith('/api/')&&!path.startsWith('/api/identity/')){
    var auth=null;try{auth=await response.clone().json()}catch{}
    if(auth&&(auth.code==='AUTH_REQUIRED'||auth.error==='中继密钥不正确')&&key===sessionStorage.getItem('yy-cloud-key')){
     window.yyLinuxTransportStatus.auth_required=true;window.yyLinuxTransportStatus.last_error='登录状态已失效，请重新登录';window.yyLinuxTransportStatus.updated_at=Date.now();
     if(typeof window.dispatchEvent==='function')window.dispatchEvent(new Event('yy-cloud-auth-expired'));
    }
   }
   if(response.status===403&&path.startsWith('/api/')&&!path.startsWith('/api/identity/'))response.clone().json().then(function(data){if(data.code==='SUBSCRIPTION_REQUIRED'&&key===sessionStorage.getItem('yy-cloud-key')){window.yyCloudBlocked=true;if(typeof yySubscriptionGate==='function'&&data.user)yySubscriptionGate(data.user)}}).catch(function(){});return response;
  });
 };
 // 云端用增量事件批次，避免把永久 SSE 当成一次请求缓冲。
 window.EventSource=function(path){
  this.listeners={};this.closed=false;this.cursor=0;var self=this;
  this.addEventListener=function(name,cb){(self.listeners[name]||(self.listeners[name]=[])).push(cb)};
  this.close=function(){self.closed=true;clearTimeout(self.timer)};
  var opened=false;
  async function poll(){if(self.closed)return;try{
   var response=await fetch('/api/youyuan/events?after='+self.cursor);var data=await response.json();if(!response.ok||!data.ok)throw Error(data.error||'设备尚未上线');
   if(self.closed)return;if(!opened){opened=true;if(self.onopen)self.onopen({})}
   (data.events||[]).forEach(function(event){if(self.closed||event.seq<=self.cursor)return;self.cursor=event.seq;var message={data:typeof event.data==='string'?event.data:JSON.stringify(event.data)};(self.listeners[event.type]||[]).forEach(function(cb){cb(message)})});
  }catch(error){if(self.closed)return;if(self.onerror)self.onerror(error)}if(!self.closed)self.timer=setTimeout(poll,1500)}
  self.timer=setTimeout(poll,100);
 };
})();
