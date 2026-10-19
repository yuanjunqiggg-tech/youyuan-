// In-flight reads are shared per account; mutations are always independent.
var yyReadRequests=new Map();
var yyReadRevisions=new Map();
function yyInvalidateReads(scope){
  scope=scope||yyReadScope();yyReadRevisions.set(scope,(yyReadRevisions.get(scope)||0)+1);
}
function yyReadScope(){
  var account=typeof yyActiveAccount==='string'?yyActiveAccount:'main',identity=typeof yyPlatformUser!=='undefined'&&yyPlatformUser?yyPlatformUser.id||yyPlatformUser.username||'':'',key='';
  try{if(typeof sessionStorage!=='undefined')key=sessionStorage.getItem('yy-cloud-key')||''}catch(e){}
  return JSON.stringify([account,identity,key]);
}
function A(m,p,b){
  var scope=yyReadScope();
  if(m!=='GET')yyInvalidateReads(scope);
  var key=JSON.stringify([scope,yyReadRevisions.get(scope)||0,p]);
  if(m==='GET'&&yyReadRequests.has(key))return yyReadRequests.get(key);
  var pending=(async function(){
    var c=new AbortController(),timer=setTimeout(function(){c.abort()},300000);
    try{var o={method:m,headers:{'Content-Type':'application/json'},signal:c.signal};if(b)o.body=JSON.stringify(b);
      var response=await fetch(p,o);return await response.json();
    }catch(e){return {ok:false,error:e.name==='AbortError'?'请求超时，执行结果尚未确认，请刷新状态后再决定是否重试':e.message}}finally{clearTimeout(timer);if(m!=='GET')yyInvalidateReads(scope)}
  })();
  if(m==='GET'){yyReadRequests.set(key,pending);pending.finally(function(){if(yyReadRequests.get(key)===pending)yyReadRequests.delete(key)})}
  return pending;
}
async function APrism(m,p,b){
  var c=new AbortController(),t=setTimeout(function(){c.abort()},15000);
  try{
    var o={method:m,headers:{'Content-Type':'application/json'},signal:c.signal};
    if(b)o.body=JSON.stringify(b);
    var r=await fetch('/api/prism-proxy'+p,o);clearTimeout(t);return r.json()
  }catch(e){clearTimeout(t);return{ok:false,error:e.name==='AbortError'?'超时':e.message}}
}
