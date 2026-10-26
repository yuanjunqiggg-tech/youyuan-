const { chromium } = require('playwright-core');
const path = require('path');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-ui';

(async () => {
  const browser = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 });
  const page = await ctx.newPage();
  const reqs = [];
  page.on('request', r => { if (r.url().includes('/api/')) reqs.push({ t: Date.now(), m: r.method(), u: r.url() }); });
  const t0 = Date.now();
  const resp = await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  const tDom = Date.now() - t0;
  await page.waitForTimeout(4000);
  const tFull = Date.now() - t0;
  console.log('status', resp && resp.status(), 'dom', tDom, 'settled', tFull);
  console.log('title', await page.title());
  console.log('url', page.url());
  await page.screenshot({ path: path.join(OUT, 'p1-desktop-home.png'), fullPage: true });
  const text = await page.evaluate(() => document.body.innerText.slice(0, 3000));
  console.log('--- TEXT ---\n' + text);
  console.log('--- API ---');
  console.log(reqs.map(r => r.m + ' ' + r.u).join('\n'));
  // visible buttons / nav
  const nav = await page.evaluate(() => {
    const out = [];
    document.querySelectorAll('button,a,[role=tab],.yy-side-nav a,.yy-tab').forEach(el => {
      const r = el.getBoundingClientRect();
      if (r.width > 0 && r.height > 0) out.push((el.tagName + '|' + (el.className || '').toString().slice(0, 40) + '|' + (el.innerText || '').trim().slice(0, 30)));
    });
    return out.slice(0, 120);
  });
  console.log('--- CLICKABLE ---\n' + nav.join('\n'));
  await browser.close();
})();
