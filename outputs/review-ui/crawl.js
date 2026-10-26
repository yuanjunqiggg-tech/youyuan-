const { chromium } = require('playwright-core');
const fs = require('fs');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-ui';
const SITE = 'https://toolbox.youyuanqi.dpdns.org/';
const tabs = ['工具总览','插件中心','终端','导入建筑','导出建筑','命令链','连接与设置','游戏账号','群服互通','远程设备'];
const W = Number(process.argv[2] || 1440), H = Number(process.argv[3] || 900), tag = process.argv[4] || 'd';

(async () => {
  const b = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
  const c = await b.newContext({ viewport: { width: W, height: H }, deviceScaleFactor: 1, isMobile: W < 600, hasTouch: W < 600 });
  const p = await c.newPage();
  // kill login gate permanently
  await p.addInitScript(() => {
    const kill = () => { const g = document.getElementById('yy-identity-gate'); if (g) g.remove(); };
    document.addEventListener('DOMContentLoaded', () => {
      kill();
      new MutationObserver(kill).observe(document.documentElement, { childList: true, subtree: true });
      const s = document.createElement('style'); s.textContent = '#yy-identity-gate{display:none!important}'; document.head.append(s);
    });
  });
  let reqs = [];
  p.on('request', r => { if (r.url().includes('/api/')) reqs.push(r.url().replace('https://toolbox.youyuanqi.dpdns.org','')); });
  const navs = [];
  p.on('framenavigated', f => { if (f === p.mainFrame()) navs.push(f.url()); });

  const t0 = Date.now();
  await p.goto(SITE, { waitUntil: 'domcontentloaded', timeout: 60000 });
  await p.waitForTimeout(5000);
  const bootReqs = reqs.slice(); reqs = [];
  console.log('BOOT', Date.now()-t0, 'ms, requests', bootReqs.length);

  const results = [];
  for (const t of tabs) {
    reqs = [];
    const before = Date.now();
    const ok = await p.evaluate((label) => {
      const els = Array.from(document.querySelectorAll('button,a'));
      const el = els.find(e => (e.innerText||'').trim() === label && e.offsetParent !== null);
      if (!el) return false; el.click(); return true;
    }, t);
    if (!ok) { results.push({ tab: t, error: '入口找不到' }); console.log('---', t, 'NO ENTRY'); continue; }
    await p.evaluate(() => {
      window.__shifts = [];
      try { window.__obs && window.__obs.disconnect(); } catch(e){}
      window.__obs = new PerformanceObserver(l => l.getEntries().forEach(e => window.__shifts.push({v: e.value, t: e.startTime})));
      try { window.__obs.observe({ type: 'layout-shift', buffered: false }); } catch(e) {}
    });
    await p.waitForTimeout(1800);
    const info = await p.evaluate(() => {
      const main = document.querySelector('#yy-main, .yy-main, main') || document.body;
      const txt = (main.innerText || '').trim();
      const de = document.documentElement;
      const small = [];
      main.querySelectorAll('button,a,input,select,[onclick]').forEach(el => {
        const r = el.getBoundingClientRect();
        if (r.width > 0 && r.height > 0 && r.height < 44) small.push({ h: Math.round(r.height), w: Math.round(r.width), txt: (el.innerText||el.getAttribute('aria-label')||'').trim().slice(0,20) });
      });
      const fonts = {};
      main.querySelectorAll('*').forEach(el => {
        const r = el.getBoundingClientRect(); if (!r.width || !r.height) return;
        const fs = getComputedStyle(el).fontSize; fonts[fs] = (fonts[fs]||0)+1;
      });
      const cards = main.querySelectorAll('.card,section').length;
      return {
        scrollH: de.scrollHeight, viewH: innerHeight, overflowX: de.scrollWidth > innerWidth,
        title: (document.querySelector('.sp-title, .yy-page-title, h2')||{}).innerText || '',
        crumb: (document.getElementById('yy-crumb')||{}).innerText || '',
        activeNav: (document.querySelector('.yy-side-nav .active')||{}).innerText || '',
        textLen: txt.length, textHead: txt.slice(0, 1400),
        smallCount: small.length, smallSample: small.slice(0, 10), fonts, cards,
        shifts: (window.__shifts||[]).reduce((a,s)=>a+s.v,0), shiftCount: (window.__shifts||[]).length,
        loadingText: /加载中|正在加载|载入中|正在同步/.test(txt),
        errorText: /失败|错误|异常|undefined|NaN|Unexpected token|无法/.test(txt),
        emptyText: /暂无|还没有|没有符合|空/.test(txt),
        toast: Array.from(document.querySelectorAll('.toast,.yy-toast,[class*=toast]')).map(e=>e.innerText.trim()).filter(Boolean)
      };
    });
    await p.screenshot({ path: `${OUT}/${tag}-${t}.png`, fullPage: true });
    results.push(Object.assign({ tab: t, ms: Date.now()-before, reqs: reqs.slice(), reqCount: reqs.length }, info));
    console.log('---', t, 'reqs', reqs.length, 'CLS', info.shifts.toFixed(3), 'shifts', info.shiftCount, 'h', info.scrollH, 'small', info.smallCount, 'cards', info.cards, 'err', info.errorText, 'loading', info.loadingText);
    console.log('   title:', info.title, '| crumb:', info.crumb, '| active:', info.activeNav);
  }
  fs.writeFileSync(`${OUT}/${tag}-crawl.json`, JSON.stringify({ boot: { ms: Date.now()-t0, reqs: bootReqs }, results, navs }, null, 1));
  await b.close();
})();
