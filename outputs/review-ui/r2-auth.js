// review-ui 探查 2：认证机制探测
const { chromium } = require('playwright-core');
const BASE = 'https://toolbox.youyuanqi.dpdns.org/';
const EXE = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const TOKEN = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';

(async () => {
  const browser = await chromium.launch({ executablePath: EXE, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();

  const seen = [];
  page.on('request', r => {
    if (r.url().includes('/api/')) {
      const h = r.headers();
      seen.push({ url: r.url().replace(BASE, ''), auth: h['authorization'] || '', cookie: (h['cookie'] || '').slice(0, 80), xYY: h['x-yy-token'] || h['x-yy'] || '' });
    }
  });

  await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 120000 });
  const before = await page.evaluate(() => ({
    ss: Object.fromEntries(Object.entries(sessionStorage)),
    ls: Object.keys(localStorage),
    cookie: document.cookie.slice(0, 200),
  }));
  await page.evaluate((t) => sessionStorage.setItem('yy-user-control', t), TOKEN);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.waitForTimeout(5000);
  const after = await page.evaluate(async () => {
    const r = await fetch('/api/identity/me', { headers: { 'Accept': 'application/json' } });
    const body = await r.text();
    return {
      ss: Object.fromEntries(Object.entries(sessionStorage)),
      ls: Object.keys(localStorage),
      cookie: document.cookie.slice(0, 200),
      meStatus: r.status,
      meBody: body.slice(0, 300),
    };
  });

  console.log(JSON.stringify({ before, after, seen: seen.slice(0, 20) }, null, 1));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
