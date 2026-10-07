// Account routing is shared by file uploads, editors and all API tools.
var yyActiveAccount='main';
var yyRootFetch=window.fetch.bind(window);
function yyAccountPath(path,id){return '/api/youyuan/accounts/invoke/'+encodeURIComponent(id||yyActiveAccount)+path}
window.fetch=function(input,options){
 var path=typeof input==='string'?input:(input&&input.url)||'';
 if(yyActiveAccount!=='main'&&path.startsWith('/api/')&&!path.startsWith('/api/youyuan/accounts')&&!path.startsWith('/api/youyuan/appearance')&&!path.startsWith('/api/youyuan/remote')&&!path.startsWith('/api/youyuan/identity')&&!path.startsWith('/api/identity/'))input=yyAccountPath(path);
 return yyRootFetch(input,options)
};
// Incremental native events support the same per-account route locally and publicly.
window.EventSource=function(){
 var self=this;self.listeners={};self.cursor=0;self.closed=false;self.account=yyActiveAccount;var opened=false,failures=0;
 self.addEventListener=function(name,cb){(self.listeners[name]||(self.listeners[name]=[])).push(cb)};
 self.close=function(){self.closed=true;clearTimeout(self.timer)};
 async function poll(){if(self.closed)return;if(self.account!==yyActiveAccount){self.account=yyActiveAccount;self.cursor=0;opened=false}
  var account=self.account;try{var r=await fetch('/api/youyuan/events?after='+self.cursor),data=await r.json();if(account!==yyActiveAccount){self.timer=setTimeout(poll,100);return}if(!r.ok||!data.ok)throw Error(data.error||'内核尚未上线');
   failures=0;if(!opened){opened=true;self.cursor=data.cursor||0;if(self.onopen)self.onopen({})}
   else (data.events||[]).forEach(function(event){self.cursor=Math.max(self.cursor,event.seq);var message={data:typeof event.data==='string'?event.data:JSON.stringify(event.data)};(self.listeners[event.type]||[]).forEach(function(cb){cb(message)})});
  }catch(error){failures++;if(self.onerror)self.onerror(error)}self.timer=setTimeout(poll,failures?Math.min(60000,1500*Math.pow(2,Math.min(failures,6))):document.hidden?10000:1500)
 }self.timer=setTimeout(poll,100)
};
