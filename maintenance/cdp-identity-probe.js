const WS_PORT = 9333;
function wsUrl() {
  return fetch(`http://127.0.0.1:${WS_PORT}/json/list`).then(r => r.json()).then(list => {
    const p = list.find(t => t.type === 'page' && /toolbox\.youyuanqi/.test(t.url));
    if (!p) throw new Error('no toolbox page');
    return p.webSocketDebuggerUrl;
  });
}
function connect(url) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url); let id = 0; const pending = new Map();
    ws.onmessage = ev => { const m = JSON.parse(ev.data); if (m.id && pending.has(m.id)) { const { res, rej } = pending.get(m.id); pending.delete(m.id); m.error ? rej(new Error(JSON.stringify(m.error))) : res(m.result); } };
    ws.onerror = e => reject(e);
    ws.onopen = () => resolve({ send(method, params) { const mid = ++id; return new Promise((res, rej) => { pending.set(mid, { res, rej }); ws.send(JSON.stringify({ id: mid, method, params: params || {} })); setTimeout(() => { if (pending.has(mid)) { pending.delete(mid); rej(new Error('timeout ' + method)); } }, 40000); }); }, close() { ws.close(); } });
  });
}
async function ev(cdp, expr) {
  const r = await cdp.send('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true });
  if (r.exceptionDetails) return { __err: r.exceptionDetails.text + ' ' + (r.exceptionDetails.exception?.description || '') };
  return r.result.value;
}
(async () => {
  const cdp = await connect(await wsUrl());
  const out = {};
  out.me = await ev(cdp, `(async function(){
    try{
      var r = await fetch('/api/identity/me',{credentials:'include'});
      var j = await r.json();
      return {status:r.status, body:j};
    }catch(e){ return {thrown:String(e)} }
  })()`);
  out.mcpList = await ev(cdp, `(async function(){
    try{
      var r = await fetch('/api/identity/mcp-links',{method:'POST',credentials:'include',headers:{'Content-Type':'application/json'},body:JSON.stringify({action:'mcp-links',op:'list'})});
      return {status:r.status, body:await r.json()};
    }catch(e){ return {thrown:String(e)} }
  })()`);
  out.remote = await ev(cdp, `(async function(){
    try{
      var r = await fetch('/api/youyuan/remote',{credentials:'include'});
      return {status:r.status, body:await r.json()};
    }catch(e){ return {thrown:String(e)} }
  })()`);
  out.storage = await ev(cdp, `(function(){
    var o={}; for(var i=0;i<localStorage.length;i++){var k=localStorage.key(i); if(/token|auth|yy-/i.test(k)) o[k]=String(localStorage.getItem(k)).slice(0,20)}
    return o;
  })()`);
  console.log(JSON.stringify(out, null, 2));
  cdp.close(); process.exit(0);
})().catch(e => { console.error('FAIL', e.message); process.exit(1); });
