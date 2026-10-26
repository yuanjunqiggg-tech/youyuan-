const { chromium } = require('playwright-core');
const fs = require('fs');
const path = require('path');

const TOKEN = JSON.parse(fs.readFileSync('D:/YouYuan-APK/private/auth-cloud-secrets.json', 'utf8')).CONSOLE_TOKEN;
const TOOLBOX = 'https://toolbox.youyuanqi.dpdns.org/';
const AUTH = 'https://auth.youyuanqi.dpdns.org/';
const OUT = __dirname;
const SAMPLE_MS = 300, SAMPLE_DURATION = 15000;

const seen4xx = new Set();
const results = { desktop: [], mobile: [], auth: { desktop: [], mobile: [] }, meta: {} };

const PAGES = [
  { id: '01', name: '首页工作台', target: '工具总览', navIdx: 0 },
  { id: '02', name: '插件管理', target: '插件中心', navIdx: 1 },
  { id: '03', name: '终端日志', target: '终端', navIdx: 2 },
  { id: '04', name: '游戏账号', target: '游戏账号', navIdx: 5, expandCard: true },
  { id: '05', name: '群服互通', target: '群服互通', navIdx: 3, tabs: ['状态', '能力', '技能', 'AI 工具', '助手', '接入', '人设', '表情'] },
  { id: '06', name: '全局抓包', target: '全局抓包', toolCard: '全局抓包', navIdx: 0 },
  { id: '07', name: '连接诊断与在线玩家', target: '连接与设置', navIdx: 4, expandDetail: true },
  { id: '08', name: '导入建筑', target: '导入建筑', toolCard: '建筑导入', navIdx: 0 },
  { id: '09', name: '导出建筑', target: '导出建筑', toolCard: '建筑导出', navIdx: 0 },
  { id: '10', name: '命令链', target: '命令链', toolCard: '命令链条', navIdx: 0 },
];

function mkBox() {
  return { errs: [], api: 0, req: 0, bad: [], navs: 0 };
}

async function attach(page, box) {
  page.on('pageerror', e => box.errs.push(String(e.message || e).slice(0, 220)));
  page.on('console', m => { if (m.type() === 'error') { const t = m.text().slice(0, 200); if (!/favicon|Turnstile|cloudflareinsights/i.test(t)) box.errs.push('[console] ' + t); } });
  page.on('request', r => {
    box.req++;
    const u = r.url();
    if (u.includes('/api/')) box.api++;
  });
  page.on('response', r => {
    const s = r.status(); const u = r.url();
    if (s >= 400 && !u.includes('favicon')) {
      const key = s + ' ' + u;
      if (!seen4xx.has(key)) { seen4xx.add(key); box.bad.push({ s, u: u.slice(0, 150) }); }
    }
  });
  page.on('framenavigated', f => { if (f === page.mainFrame()) box.navs++; });
}

async function sample(page, ms) {
  const t0 = Date.now();
  let prev = null, changes = 0, samples = 0;
  const seq = [];
  while (Date.now() - t0 < ms) {
    const s = await page.evaluate(() => {
      const main = document.querySelector('main,.yy-main,.yy-content,.yy-page') || document.body;
      return { sh: document.body.scrollHeight, sy: Math.round(window.scrollY), ih: Math.round(main.scrollTop || 0), muts: window.__muts || 0 };
    }).catch(() => null);
    if (!s) { await page.waitForTimeout(SAMPLE_MS); continue; }
    samples++;
    const sig = `${s.sh}|${s.sy}|${s.ih}`;
    if (prev !== null && sig !== prev) { changes++; seq.push(sig); }
    prev = sig;
    await page.waitForTimeout(SAMPLE_MS);
  }
  return { changes, samples, muts: prev ? prev.muts : 0, seq: seq.slice(0, 6) };
}

async function overflow(page) {
  return page.evaluate(() => {
    const iw = innerWidth;
    const docOver = document.documentElement.scrollWidth - iw;
    const off = [];
    document.querySelectorAll('body *').forEach(e => {
      const b = e.getBoundingClientRect();
      if (b.width === 0 || b.height === 0) return;
      if (getComputedStyle(e).position === 'fixed') return;
      if (b.right > iw + 2 || b.left < -2) {
        if (e.children.length > 3) return;
        const cls = (e.className || '').toString().slice(0, 40);
        if (off.length < 5) off.push(cls || e.tagName);
      }
    });
    return { docOver, offCount: off.length, off: off.slice(0, 3) };
  }).catch(() => ({ docOver: 0, offCount: 0, off: [] }));
}

async function clickTiming(page, clickFn) {
  await page.evaluate(() => { window.__e2eMark = 'alive'; window.__muts = 0; new MutationObserver(m => { window.__muts += m.length; }).observe(document.body, { childList: true, subtree: true, attributes: true }); });
  const t0 = Date.now();
  await clickFn();
  let firstContent = null, blank = 0, seen = false, len0 = null, stableAt = null, lastLen = -1, same = 0;
  while (Date.now() - t0 < 9000) {
    const len = await page.evaluate(() => document.body.innerText.length).catch(() => -1);
    if (len0 === null) len0 = len;
    if (len >= 200 && firstContent === null) firstContent = Date.now() - t0;
    if (firstContent !== null && Date.now() - t0 > firstContent && len < 200) blank++;
    if (len === lastLen) { same++; if (same >= 4 && stableAt === null) stableAt = Date.now() - t0 - 300; }
    else { same = 0; lastLen = len; }
    if (stableAt !== null && Date.now() - t0 > (firstContent || 0) + 1500) break;
    await page.waitForTimeout(100);
  }
  const alive = await page.evaluate(() => typeof window.__e2eMark !== 'undefined').catch(() => false);
  return { firstContent, blank, stable: stableAt || Date.now() - t0, reloaded: !alive };
}

async function runViewport(browser, label, vp) {
  const ctx = await browser.newContext({ viewport: vp, isMobile: label === 'mobile', hasTouch: label === 'mobile', deviceScaleFactor: 1 });
  const page = await ctx.newPage();
  page.setDefaultTimeout(15000);
  const boot = mkBox();
  await attach(page, boot);

  await page.goto(TOOLBOX, { waitUntil: 'domcontentloaded', timeout: 45000 });
  await page.evaluate(t => sessionStorage.setItem('yy-cloud-key', t), TOKEN);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 45000 });
  await page.waitForTimeout(4000);

  const navTo = async (p) => {
    if (label === 'desktop') {
      await page.getByRole('button', { name: p.target, exact: true }).first().click({ timeout: 8000 });
      return;
    }
    // mobile: bottom nav then tool card
    const item = page.locator('.yy-player-navigation button, .yy-player-navigation a').nth(p.navIdx);
    await item.click({ force: true, timeout: 8000 });
    await page.waitForTimeout(1500);
    if (p.toolCard) {
      const card = page.locator('.yy-tool-open').filter({ hasText: p.toolCard }).first();
      if (await card.count()) { await card.scrollIntoViewIfNeeded(); await card.click({ force: true, timeout: 8000 }); }
      else throw new Error('tool card not found: ' + p.toolCard);
    }
  };

  for (const p of PAGES) {
    const box = mkBox();
    const handlers = [];
    const onErr = e => box.errs.push(String(e.message || e).slice(0, 220));
    const onCon = m => { if (m.type() === 'error') { const t = m.text().slice(0, 200); if (!/favicon|Turnstile|cloudflareinsights/i.test(t)) box.errs.push('[console] ' + t); } };
    const onReq = r => { box.req++; if (r.url().includes('/api/')) box.api++; };
    const onRes = r => { const s = r.status(), u = r.url(); if (s >= 400 && !u.includes('favicon')) { const k = s + ' ' + u; if (!seen4xx.has(k)) { seen4xx.add(k); box.bad.push({ s, u: u.slice(0, 150) }); } } };
    const onNav = f => { if (f === page.mainFrame()) box.navs++; };
    page.on('pageerror', onErr); page.on('console', onCon); page.on('request', onReq); page.on('response', onRes); page.on('framenavigated', onNav);

    const rec = { id: p.id, name: p.name, target: p.target };
    try {
      const timing = await clickTiming(page, () => navTo(p));
      rec.timing = timing;
      await page.waitForTimeout(900);
      rec.len = await page.evaluate(() => document.body.innerText.length);
      rec.title = await page.evaluate(() => (document.querySelector('h1,h2')?.innerText || '').slice(0, 40));

      if (p.expandCard) {
        const sum = page.locator('details.yy-account-card > summary').first();
        if (await sum.count()) {
          const before = rec.len;
          await sum.click({ force: true });
          await page.waitForTimeout(1200);
          rec.expand = { ok: true, delta: (await page.evaluate(() => document.body.innerText.length)) - before };
        } else rec.expand = { ok: false };
      }
      if (p.expandDetail) {
        const b = page.getByRole('button', { name: /展开详情/ }).first();
        if (await b.count()) { await b.click({ force: true }).catch(() => {}); await page.waitForTimeout(1000); rec.expandDetail = true; }
      }

      const jit = await sample(page, SAMPLE_DURATION);
      rec.jitter = jit.changes; rec.samples = jit.samples; rec.muts = jit.muts; rec.seq = jit.seq;
      rec.lenEnd = await page.evaluate(() => document.body.innerText.length);
      rec.overflow = await overflow(page);
      rec.errs = box.errs.slice(0, 6); rec.api = box.api; rec.req = box.req; rec.bad = box.bad.slice(0, 4); rec.navs = box.navs;

      if (p.tabs) {
        rec.tabs = [];
        for (const t of p.tabs) {
          const tb = { t };
          const e0 = box.errs.length, a0 = box.api;
          try {
            const chip = page.locator('.yyq-quicknav-chip').filter({ hasText: new RegExp('^' + t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '$') }).first();
            if (await chip.count()) {
              await chip.click({ force: true, timeout: 6000 });
              await page.waitForTimeout(1600);
              tb.len = await page.evaluate(() => document.body.innerText.length);
              tb.sy = await page.evaluate(() => Math.round(window.scrollY));
              tb.api = box.api - a0;
              tb.errs = box.errs.length - e0;
              await page.screenshot({ path: path.join(OUT, `${label}-05-群服互通-${t}.png`) });
            } else tb.missing = true;
          } catch (e) { tb.fail = String(e).slice(0, 80); }
          rec.tabs.push(tb);
        }
        rec.api += box.api; // keep total
      }
      await page.screenshot({ path: path.join(OUT, `${label}-${p.id}-${p.name}.png`), fullPage: true });
    } catch (e) {
      rec.fail = String(e.message || e).slice(0, 160);
      rec.len = await page.evaluate(() => document.body.innerText.length).catch(() => -1);
      rec.errs = box.errs.slice(0, 6); rec.api = box.api; rec.bad = box.bad.slice(0, 3);
      await page.screenshot({ path: path.join(OUT, `${label}-${p.id}-${p.name}-FAIL.png`) }).catch(() => {});
    }
    page.off('pageerror', onErr); page.off('console', onCon); page.off('request', onReq); page.off('response', onRes); page.off('framenavigated', onNav);
    results[label].push(rec);
    console.log(label, p.id, p.name, 'len=', rec.len, 'errs=', (rec.errs || []).length, 'api=', rec.api, 'jit=', rec.jitter, 'bad=', (rec.bad || []).length, rec.fail ? 'FAIL:' + rec.fail : '');
  }

  // ---- auth site ----
  const ap = await ctx.newPage();
  const abox = mkBox();
  await attach(ap, abox);
  const pushAuth = async (id, name, fn, shot) => {
    const rec = { id, name };
    const errs = []; abox.errs.length = 0; abox.api = 0; abox.req = 0; abox.bad = []; abox.navs = 0;
    try {
      const timing = await clickTiming(ap, fn);
      rec.timing = timing;
      await ap.waitForTimeout(900);
      rec.len = await ap.evaluate(() => document.body.innerText.length);
      const jit = await sample(ap, 8000);
      rec.jitter = jit.changes; rec.muts = jit.muts;
      rec.overflow = await overflow(ap);
      rec.errs = abox.errs.slice(0, 6); rec.api = abox.api; rec.bad = abox.bad.slice(0, 4); rec.navs = abox.navs;
      if (shot) await ap.screenshot({ path: path.join(OUT, `${label}-A-${id}-${name}.png`), fullPage: true });
    } catch (e) { rec.fail = String(e.message || e).slice(0, 140); }
    results.auth[label].push(rec);
    console.log(label, 'AUTH', id, name, 'len=', rec.len, 'api=', rec.api, 'errs=', (rec.errs || []).length);
  };

  await pushAuth('A1', '入口', async () => { await ap.goto(AUTH, { waitUntil: 'domcontentloaded', timeout: 40000 }); }, true);
  await pushAuth('A2', '注册', async () => { await ap.getByRole('button', { name: '注册账号', exact: true }).first().click({ timeout: 8000 }); }, true);
  const consoleInfo = await ap.evaluate(() => {
    const btn = [...document.querySelectorAll('button')].find(b => /退出平台账号/.test(b.innerText || ''));
    if (!btn) return { found: false };
    let el = btn, chain = [];
    for (let i = 0; i < 6 && el; i++) { chain.push({ tag: el.tagName, id: el.id, cls: (el.className || '').toString().slice(0, 40), hidden: el.hidden || getComputedStyle(el).display === 'none' }); el = el.parentElement; }
    return { found: true, chain };
  }).catch(() => ({ found: false }));
  results.auth[label].push({ id: 'A3', name: '控制台', consoleInfo });
  const deep = [];
  for (const u of ['https://auth.youyuanqi.dpdns.org/console', 'https://auth.youyuanqi.dpdns.org/dashboard']) {
    const r = await ap.goto(u, { waitUntil: 'domcontentloaded', timeout: 30000 }).then(async () => ({ url: u, len: await ap.evaluate(() => document.body.innerText.length), txt: (await ap.evaluate(() => document.body.innerText)).slice(0, 90) })).catch(e => ({ url: u, err: String(e).slice(0, 80) }));
    deep.push(r);
  }
  results.auth[label].push({ id: 'A3b', name: '控制台深链', deep });
  await ap.close();
  await ctx.close();
}

(async () => {
  const browser = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true, args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu'] });
  try {
    await runViewport(browser, 'desktop', { width: 1440, height: 900 });
    await runViewport(browser, 'mobile', { width: 390, height: 844 });
  } finally {
    results.meta = { ts: new Date().toISOString(), distinct4xx: [...seen4xx] };
    fs.writeFileSync(path.join(OUT, 'results.json'), JSON.stringify(results, null, 1));
    await browser.close();
    console.log('DONE');
  }
})();
