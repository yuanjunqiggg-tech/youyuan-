const { chromium } = require('playwright-core');
const fs = require('fs');
const path = require('path');
const TOKEN = JSON.parse(fs.readFileSync('D:/YouYuan-APK/private/auth-cloud-secrets.json', 'utf8')).CONSOLE_TOKEN;
const TOOLBOX = 'https://toolbox.youyuanqi.dpdns.org/';
const OUT = __dirname;
const out = { desktop: [], mobile: [], notes: {} };

function attach(page, box) {
  page.on('pageerror', e => box.errs.push({ msg: String(e.message || e).slice(0, 200) }));
  page.on('console', m => {
    if (m.type() !== 'error') return;
    const t = m.text().slice(0, 180);
    if (/favicon|Turnstile|cloudflareinsights/i.test(t)) return;
    box.errs.push({ msg: t, url: (m.location() || {}).url || '' });
  });
  page.on('request', r => { box.req++; if (r.url().includes('/api/')) box.api++; });
  page.on('response', r => { if (r.status() >= 400) box.bad.push({ s: r.status(), u: r.url().slice(0, 130) }); });
}
const mk = () => ({ errs: [], api: 0, req: 0, bad: [] });

async function sample(page, ms) {
  const t0 = Date.now(); let prev = null, ch = 0;
  while (Date.now() - t0 < ms) {
    const s = await page.evaluate(() => ({ sh: document.body.scrollHeight, sy: Math.round(scrollY) })).catch(() => null);
    if (s) { const sig = s.sh + '|' + s.sy; if (prev !== null && sig !== prev) ch++; prev = sig; }
    await page.waitForTimeout(300);
  }
  return ch;
}

(async () => {
  const browser = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true, args: ['--no-sandbox', '--disable-gpu'] });

  // ---------- A/B: desktop pages with 401 URL capture ----------
  {
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await ctx.newPage();
    const box = mk(); attach(page, box);
    await page.goto(TOOLBOX, { waitUntil: 'domcontentloaded' });
    await page.evaluate(t => sessionStorage.setItem('yy-cloud-key', t), TOKEN);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3800);
    box.errs.length = 0; box.bad.length = 0; box.api = 0;

    await page.getByRole('button', { name: '游戏账号', exact: true }).first().click();
    await page.waitForTimeout(5000);
    out.desktop.push({ id: '04b', name: '游戏账号(401定位)', len: await page.evaluate(() => document.body.innerText.length), api: box.api, req: box.req, errs: box.errs.slice(0, 8), bad: box.bad.slice(0, 8) });
    await page.screenshot({ path: path.join(OUT, 'desktop-04b-游戏账号-401.png'), fullPage: true });

    box.errs.length = 0; box.bad.length = 0; box.api = 0;
    await page.getByRole('button', { name: '连接与设置', exact: true }).first().click();
    await page.waitForTimeout(5000);
    out.desktop.push({ id: '07b', name: '连接与设置(401定位)', len: await page.evaluate(() => document.body.innerText.length), api: box.api, req: box.req, errs: box.errs.slice(0, 8), bad: box.bad.slice(0, 8) });

    // ---------- C: 全局抓包 via tool card ----------
    box.errs.length = 0; box.bad.length = 0; box.api = 0;
    await page.getByRole('button', { name: '工具总览', exact: true }).first().click();
    await page.waitForTimeout(2000);
    const card = page.locator('.yy-tool-open').filter({ hasText: '全局抓包' }).first();
    const cnt = await card.count();
    const cardText = cnt ? (await card.innerText()).replace(/\n+/g, '|') : '';
    const t0 = Date.now();
    if (cnt) { await card.scrollIntoViewIfNeeded(); await card.click({ force: true }); }
    let blank = 0, firstContent = null, stable = null, last = -1, same = 0;
    while (Date.now() - t0 < 9000) {
      const len = await page.evaluate(() => document.body.innerText.length).catch(() => -1);
      if (len >= 200 && firstContent === null) firstContent = Date.now() - t0;
      if (firstContent !== null && len < 200) blank++;
      if (len === last) { same++; if (same >= 4 && stable === null) stable = Date.now() - t0 - 300; } else { same = 0; last = len; }
      if (stable && Date.now() - t0 > (firstContent || 0) + 1500) break;
      await page.waitForTimeout(100);
    }
    const jit = await sample(page, 15000);
    const ov = await page.evaluate(() => ({ over: document.documentElement.scrollWidth - innerWidth, len: document.body.innerText.length, head: document.body.innerText.slice(0, 200).replace(/\n+/g, '|') }));
    out.desktop.push({ id: '06', name: '全局抓包', cardFound: cnt, cardText, timing: { firstContent, blank, stable }, jitter: jit, len: ov.len, head: ov.head, overflow: ov.over, api: box.api, req: box.req, errs: box.errs.slice(0, 6), bad: box.bad.slice(0, 6) });
    await page.screenshot({ path: path.join(OUT, 'desktop-06-全局抓包.png'), fullPage: true });
    await ctx.close();
  }

  // ---------- D: mobile 06-10 ----------
  {
    const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
    const page = await ctx.newPage();
    const box = mk(); attach(page, box);
    await page.goto(TOOLBOX, { waitUntil: 'domcontentloaded' });
    await page.evaluate(t => sessionStorage.setItem('yy-cloud-key', t), TOKEN);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3800);

    const navVisible = () => page.evaluate(() => { const n = document.querySelector('.yy-player-navigation'); if (!n) return false; const b = n.getBoundingClientRect(); return b.width > 0 && b.height > 0; });
    const ensureNav = async () => {
      if (await navVisible()) return 'ok';
      const back = await page.evaluate(() => {
        const sub = document.querySelector('.yy-sub-nav');
        const cand = sub ? [...sub.querySelectorAll('button,a')].find(e => /首页/.test(e.innerText || '')) : null;
        if (cand) { cand.click(); return 'subnav'; }
        return 'none';
      });
      await page.waitForTimeout(1500);
      if (await navVisible()) return back;
      await page.goto(TOOLBOX, { waitUntil: 'domcontentloaded' });
      await page.waitForTimeout(3500);
      return 'reload';
    };
    const clickBottom = async (idx) => page.evaluate(i => {
      scrollTo(0, 0);
      const els = [...document.querySelectorAll('.yy-player-navigation button,.yy-player-navigation a')];
      if (!els[i]) return 'no-el';
      els[i].click(); return 'ok';
    }, idx);

    const targets = [
      { id: '06', name: '全局抓包', card: '全局抓包' },
      { id: '07', name: '连接诊断与在线玩家', navIdx: 4 },
      { id: '08', name: '导入建筑', card: '建筑导入' },
      { id: '09', name: '导出建筑', card: '建筑导出' },
      { id: '10', name: '命令链', card: '命令链条' },
    ];
    for (const t of targets) {
      const rec = { id: t.id, name: t.name };
      box.errs.length = 0; box.bad.length = 0; box.api = 0; box.req = 0;
      try {
        rec.navMode = await ensureNav();
        const t0 = Date.now();
        if (t.card) {
          await clickBottom(0);
          await page.waitForTimeout(1600);
          const card = page.locator('.yy-tool-open').filter({ hasText: t.card }).first();
          await card.scrollIntoViewIfNeeded();
          await card.click({ force: true, timeout: 8000 });
        } else {
          await clickBottom(t.navIdx);
        }
        rec.firstContent = await (async () => { let fc = null; while (Date.now() - t0 < 8000) { const l = await page.evaluate(() => document.body.innerText.length).catch(() => 0); if (l >= 200) { fc = Date.now() - t0; break; } await page.waitForTimeout(100); } return fc; })();
        await page.waitForTimeout(1200);
        rec.len = await page.evaluate(() => document.body.innerText.length);
        rec.jitter = await sample(page, 15000);
        rec.lenEnd = await page.evaluate(() => document.body.innerText.length);
        rec.overflow = await page.evaluate(() => ({ over: document.documentElement.scrollWidth - innerWidth, off: [...document.querySelectorAll('body *')].filter(e => { const b = e.getBoundingClientRect(); return b.width > 0 && b.right > innerWidth + 2 && e.children.length <= 3 && getComputedStyle(e).position !== 'fixed'; }).slice(0, 3).map(e => (e.className || '').toString().slice(0, 30)) }));
        rec.api = box.api; rec.req = box.req; rec.errs = box.errs.slice(0, 5); rec.bad = box.bad.slice(0, 5);
        await page.screenshot({ path: path.join(OUT, `mobile-${t.id}-${t.name}.png`), fullPage: true });
      } catch (e) { rec.fail = String(e.message || e).slice(0, 120); rec.len = await page.evaluate(() => document.body.innerText.length).catch(() => -1); rec.api = box.api; }
      out.mobile.push(rec);
      console.log('M', t.id, t.name, rec.len, 'jit', rec.jitter, 'api', rec.api, 'nav', rec.navMode, rec.fail || '');
    }

    // chip overflow check on 群服互通
    await ensureNav();
    await clickBottom(3);
    await page.waitForTimeout(2500);
    out.notes.chipRow = await page.evaluate(() => {
      const chip = document.querySelector('.yyq-quicknav-chip');
      if (!chip) return { found: false };
      let el = chip.parentElement, info = [];
      for (let i = 0; i < 3 && el; i++) { const cs = getComputedStyle(el); info.push({ cls: (el.className || '').toString().slice(0, 30), ox: cs.overflowX, sw: el.scrollWidth, cw: el.clientWidth }); el = el.parentElement; }
      const b = chip.getBoundingClientRect();
      return { found: true, info, chipRect: { x: Math.round(b.x), right: Math.round(b.right), w: Math.round(b.width) }, iw: innerWidth };
    });
    await ctx.close();
  }

  fs.writeFileSync(path.join(OUT, 'results2.json'), JSON.stringify(out, null, 1));
  console.log('DONE2');
  await browser.close();
})();
