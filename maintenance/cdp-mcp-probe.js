// CDP probe: 检查线上设置页里 MCP 相关入口/网址的真实渲染状态
const WS_PORT = 9333;

function wsUrl() {
  return fetch(`http://127.0.0.1:${WS_PORT}/json/list`)
    .then(r => r.json())
    .then(list => {
      const p = list.find(t => t.type === 'page' && /toolbox\.youyuanqi/.test(t.url));
      if (!p) throw new Error('no toolbox page target');
      return p.webSocketDebuggerUrl;
    });
}

function connect(url) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url);
    let id = 0;
    const pending = new Map();
    ws.onmessage = ev => {
      const msg = JSON.parse(ev.data);
      if (msg.id && pending.has(msg.id)) {
        const { res, rej } = pending.get(msg.id);
        pending.delete(msg.id);
        msg.error ? rej(new Error(JSON.stringify(msg.error))) : res(msg.result);
      }
    };
    ws.onerror = e => reject(e);
    ws.onopen = () => resolve({
      send(method, params) {
        const mid = ++id;
        return new Promise((res, rej) => {
          pending.set(mid, { res, rej });
          ws.send(JSON.stringify({ id: mid, method, params: params || {} }));
          setTimeout(() => {
            if (pending.has(mid)) { pending.delete(mid); rej(new Error('timeout ' + method)); }
          }, 30000);
        });
      },
      close() { ws.close(); }
    });
  });
}

const sleep = ms => new Promise(r => setTimeout(r, ms));

async function evaluate(cdp, expr) {
  const r = await cdp.send('Runtime.evaluate', {
    expression: expr,
    returnByValue: true,
    awaitPromise: true,
    userGesture: true,
  });
  if (r.exceptionDetails) {
    return { __error: r.exceptionDetails.text + ' ' + (r.exceptionDetails.exception?.description || '') };
  }
  return r.result.value;
}

(async () => {
  const url = await wsUrl();
  const cdp = await connect(url);
  await cdp.send('Page.enable');
  await cdp.send('Runtime.enable');
  await cdp.send('Page.reload', { ignoreCache: true });
  await sleep(9000);

  const out = {};

  out.env = await evaluate(cdp, `(function(){
    return {
      href: location.href,
      yyIsCloud: (typeof yyIsCloud!=='undefined')? yyIsCloud : 'undefined',
      curTab: (typeof curTab!=='undefined')? curTab : 'undefined',
      hasRcfg: typeof Rcfg,
      hasMCPLinksPage: typeof yyMCPLinksPage,
      hasRemotePage: typeof yyRemotePage
    };
  })()`);

  // 进入设置页
  await evaluate(cdp, `(function(){ if(typeof tabTo==='function') tabTo('settings'); return 1; })()`);
  await sleep(4500);

  out.settings = await evaluate(cdp, `(function(){
    var root = document.querySelector('#tab-settings>.content-inner');
    if(!root) return {found:false};
    var card = document.getElementById('cfg-mcp-card');
    var res = {found:true, hasCard: !!card};
    if(card){
      var d = card.closest('details');
      res.title = (card.querySelector('h3,.yy-form-head')||{}).textContent || '';
      res.codeText = (card.querySelector('.yy-code')||{}).textContent || '';
      res.inDetails = !!d;
      res.detailsOpen = d? d.open : null;
      res.detailsSummary = d? (d.querySelector('summary')||{}).textContent : null;
      var grp = card.closest('.yy-settings-group');
      res.group = grp? grp.dataset.group : null;
      res.groupOpen = grp? grp.open : null;
      var r = card.getBoundingClientRect();
      res.rect = {w: Math.round(r.width), h: Math.round(r.height)};
      res.visible = !!(r.width && r.height);
    }
    // 账号联通与连接授权卡片
    var boxes = [].slice.call(root.querySelectorAll('section.card'));
    res.bridgeCard = boxes.filter(function(b){ return /账号联通与连接授权/.test(b.textContent||'') }).map(function(b){
      var d=b.closest('details'); var g=b.closest('.yy-settings-group');
      return {inDetails:!!d, open:d?d.open:null, group:g?g.dataset.group:null, groupOpen:g?g.open:null};
    });
    res.mcpButton = !!root.querySelector('[onclick="yyMCPLinksPage()"]');
    res.allGroupStates = [].slice.call(root.querySelectorAll('.yy-settings-group')).map(function(g){
      return {id:g.dataset.group, open:g.open, modules:[].slice.call(g.querySelectorAll('.yy-settings-module')).map(function(m){
        return {t:(m.querySelector('summary')||{}).textContent, open:m.open};
      })};
    });
    return res;
  })()`);

  // 直接调 mcp-links 列表接口
  out.mcpLinksApi = await evaluate(cdp, `(async function(){
    try{
      var r = await yyConnectionRequest({op:'list'});
      return {ok: r && r.ok, error: r && r.error, count: (r&&r.links)?r.links.length:null, raw: JSON.stringify(r).slice(0,600)};
    }catch(e){ return {thrown: String(e)}; }
  })()`);

  console.log(JSON.stringify(out, null, 2));
  cdp.close();
  process.exit(0);
})().catch(e => { console.error('FAIL', e.message); process.exit(1); });
