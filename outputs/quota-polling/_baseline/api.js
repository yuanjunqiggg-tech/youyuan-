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
      var response=await fetch(p,o);
      // 服务未启动、地址配错、或反向代理返回错误页时，响应体是 HTML 而不是 JSON。
      // 直接 response.json() 会把 "Unexpected token '<'" 抛给用户当错误文案（曾出现在 6 个屏），
      // 这里改为给人话，原始文本只进 console 供排查。
      var ct=(response.headers&&typeof response.headers.get==='function')?(response.headers.get('content-type')||''):'';
      // 只在服务端明确声明了非 JSON 类型时才兜底；拿不到 content-type 时仍按 JSON 解析，
      // 否则会把没有该头的正常响应（含测试替身）误判成坏响应。
      if(ct && ct.toLowerCase().indexOf('json')<0){
        var raw='';try{raw=await response.text()}catch(_){}
        if(typeof console!=='undefined'&&console.warn)console.warn('[api] 非 JSON 响应',m,p,'status='+response.status,raw.slice(0,300));
        return {ok:false,code:'bad_response',status:response.status,
          error:'服务器返回了网页而不是数据（HTTP '+response.status+'），通常是服务未启动或地址配错，请刷新重试'};
      }
      return await response.json();
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
