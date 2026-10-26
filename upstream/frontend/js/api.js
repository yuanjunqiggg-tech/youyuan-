// In-flight reads are shared per account; mutations are always independent.
// 云端每条 /api/* 都要过 Worker + DO 双扣，所以 GET 额外加一层 2 秒结果缓存：
// 同一 URL 在 2 秒内的重复调用直接返回上一次结果，不再发网络请求。
// 任何写操作都会 yyInvalidateReads（revision 变化 → key 变化），所以缓存不会把
// 「刚改完的状态」读成旧值。进度类接口（事件游标、飞行坐标、命令方块进度）必须
// 实时，列在 bypass 里不缓存。
var yyReadRequests=new Map();
var yyReadRevisions=new Map();
var yyReadCache=new Map();
var YY_READ_CACHE_MS=2000;
var YY_READ_CACHE_MAX=300;
var YY_READ_CACHE_BYPASS=/\/events|\/fly\/|\/mcfunction\//;
function yyInvalidateReads(scope){
  scope=scope||yyReadScope();yyReadRevisions.set(scope,(yyReadRevisions.get(scope)||0)+1);
  if(yyReadCache.size)yyReadCache.clear();
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
  if(m==='GET'){
    if(yyReadRequests.has(key))return yyReadRequests.get(key);
    if(!YY_READ_CACHE_BYPASS.test(p)){
      var cached=yyReadCache.get(key);
      if(cached&&Date.now()-cached.at<YY_READ_CACHE_MS)return Promise.resolve(cached.value);
    }
  }
  var pending=(async function(){
    var c=new AbortController(),timer=setTimeout(function(){c.abort()},300000);
    try{var o={method:m,headers:{'Content-Type':'application/json'},signal:c.signal};if(b)o.body=JSON.stringify(b);
      // 计数由 youyuan_poll_policy.js 的 fetch 包装统一做（含 sse.js 的裸 fetch），这里不重复计数。
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
  if(m==='GET'){
    yyReadRequests.set(key,pending);
    pending.finally(function(){if(yyReadRequests.get(key)===pending)yyReadRequests.delete(key)});
    if(!YY_READ_CACHE_BYPASS.test(p))pending.then(function(v){
      if(yyReadCache.size>=YY_READ_CACHE_MAX)yyReadCache.clear();
      yyReadCache.set(key,{at:Date.now(),value:v});
    },function(){});
  }
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
