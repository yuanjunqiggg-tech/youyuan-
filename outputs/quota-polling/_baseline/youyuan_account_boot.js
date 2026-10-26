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
// 云端额度护栏：这一层是长轮询替身，每拉一次都要经过 Worker（免费版 10 万次/天）。
// 可见时按 YY_EVENTS_POLL_MS 拉；页面不可见时直接停拉，visibilitychange 回来立刻补拉，
// 游标（after=）保证不丢事件，后台标签页不再按 10 秒空转。
var YY_EVENTS_POLL_MS=3000;
window.EventSource=function(){
 var self=this;self.listeners={};self.cursor=0;self.closed=false;self.account=yyActiveAccount;var opened=false,failures=0;
 self.addEventListener=function(name,cb){(self.listeners[name]||(self.listeners[name]=[])).push(cb)};
 function onVisible(){if(self.closed||document.hidden)return;clearTimeout(self.timer);self.timer=setTimeout(poll,80)}
 self.close=function(){self.closed=true;clearTimeout(self.timer);if(document.removeEventListener)document.removeEventListener('visibilitychange',onVisible,false)};
 function later(ms){if(self.closed)return;self.timer=setTimeout(poll,ms)}
 function settle(){if(self.closed)return;if(document.hidden){self.timer=null;return}later(failures?Math.min(60000,YY_EVENTS_POLL_MS*Math.pow(2,Math.min(failures,6))):YY_EVENTS_POLL_MS)}
 async function poll(){if(self.closed)return;if(document.hidden){self.timer=null;return}if(self.account!==yyActiveAccount){self.account=yyActiveAccount;self.cursor=0;opened=false}
  var account=self.account;try{var r=await fetch('/api/youyuan/events?after='+self.cursor),data=await r.json();if(account!==yyActiveAccount){later(100);return}if(!r.ok||!data.ok)throw Error(data.error||'内核尚未上线');
   if(self.closed)return;failures=0;if(!opened){opened=true;self.cursor=data.cursor||0;if(self.onopen)self.onopen({})}
   else (data.events||[]).forEach(function(event){if(self.closed||event.seq<=self.cursor)return;self.cursor=event.seq;var message={data:typeof event.data==='string'?event.data:JSON.stringify(event.data)};(self.listeners[event.type]||[]).forEach(function(cb){cb(message)})});
  }catch(error){if(self.closed)return;failures++;if(self.onerror)self.onerror(error)}settle()
 }
 if(document.addEventListener)document.addEventListener('visibilitychange',onVisible,false);
 self.timer=null;if(!document.hidden)later(100);
};
