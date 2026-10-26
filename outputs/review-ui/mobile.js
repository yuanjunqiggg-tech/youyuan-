const { chromium } = require('playwright-core');
const fs = require('fs');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-ui';
const SITE = 'https://toolbox.youyuanqi.dpdns.org/';
const tabs = ['总览','插件','终端','互通','连接','账号'];
(async () => {
  const b = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
  const c = await b.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
  const p = await c.newPage();
  await p.addInitScript(() => {
    const kill = () => { const g = document.getElementById('yy-identity-gate'); if (g) g.remove(); };
    document.addEventListener('DOMContentLoaded', () => {
      kill(); new MutationObserver(kill).observe(document.documentElement, { childList: true, subtree: true });
      const s = document.createElement('style'); s.textContent = '#yy-identity-gate{display:none!important}'; document.head.append(s);
    });
  });
  let reqs = [];
  p.on('request', r => { if (r.url().includes('/api/')) reqs.push(r.url().replace('https://toolbox.youyuanqi.dpdns.org','')); });
  await p.goto(SITE, { waitUntil: 'domcontentloaded', timeout: 60000 });
  await p.waitForTimeout(5000);
  console.log('BOOT reqs', reqs.length); reqs = [];
  const res = [];
  for (const t of tabs) {
    reqs = [];
    const ok = await p.evaluate((label) => {
      const nav = document.querySelector('.yy-bottom-nav, .yy-mobile-nav, nav');
      const els = Array.from(document.querySelectorAll('.yy-bottom-nav button, .yy-bottom-nav a, nav button, nav a'));
      const el = els.find(e => (e.innerText||'').trim().split('\n').pop().trim() === label) || els.find(e=>(e.innerText||'').includes(label));
      if (!el) return false; el.click(); return true;
    }, t);
    if (!ok) { res.push({ tab: t, error: 'no entry' }); continue; }
    await p.waitForTimeout(2000);
    const info = await p.evaluate(() => {
      const de = document.documentElement;
      const small = [];
      document.querySelectorAll('button,a,input,select,[onclick]').forEach(el => {
        const r = el.getBoundingClientRect();
        if (r.width > 0 && r.height > 0 && r.height < 44) small.push({ h: Math.round(r.height), w: Math.round(r.width), txt: (el.innerText||el.getAttribute('aria-label')||'').trim().slice(0,18) });
      });
      return { scrollH: de.scrollHeight, viewH: innerHeight, overflowX: de.scrollWidth > innerWidth, smallCount: small.length, smallSample: small.slice(0,14),
        textHead: document.body.innerText.slice(0, 1200), crumb: (document.getElementById('yy-crumb')||{}).innerText||'',
        active: (document.querySelector('.yy-bottom-nav .active,nav .active')||{}).innerText||'' };
    });
    await p.screenshot({ path: `${OUT}/m2-${t}.png`, fullPage: true });
    res.push(Object.assign({ tab: t, reqs: reqs.slice(), reqCount: reqs.length }, info));
    console.log('---', t, 'reqs', reqs.length, 'h', info.scrollH, 'overflowX', info.overflowX, 'small', info.smallCount, 'active', info.active.replace(/\n/g,' '));
  }
  fs.writeFileSync(OUT+'/mobile-crawl.json', JSON.stringify(res,null,1));
  await b.close();
})();
