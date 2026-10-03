// 云端同源前端用用户口令访问 APK；本机 WebView 不需二次口令。
(function(){
 try{localStorage.setItem('prism_wizard_done','true')}catch(error){}
 if(location.hostname==='127.0.0.1'||location.hostname==='localhost')return;
 var originalFetch=window.fetch.bind(window);
 window.fetch=function(input,options){
  var path=typeof input==='string'?input:(input&&input.url)||'';
  if(path.startsWith('/api/')||path.startsWith('/mcp')||path==='/health'){
   options=Object.assign({},options);var headers=new Headers(options.headers||{});if(path.includes('market-proxy')&&headers.has('Authorization'))headers.set('X-YouYuan-Market-Authorization',headers.get('Authorization'));var key=sessionStorage.getItem('yy-cloud-key');if(key)headers.set('Authorization','Bearer '+key);var device=new URLSearchParams(location.search).get('device');if(device&&/^[A-Za-z0-9_-]{1,48}$/.test(device))headers.set('X-YouYuan-Device',device);options.headers=headers;
  }
  return originalFetch(input,options);
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
