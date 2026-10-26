// review-ui 探查 3：用 yy-cloud-key 登录
const { chromium } = require('playwright-core');
const BASE = 'https://toolbox.youyuanqi.dpdns.org/';
const EXE = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const TOKEN = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';

(async () => {
  const browser = await chromium.launch({ executablePath: EXE, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();

  const fails = [];
  page.on('response', r => { if (r.url().includes('/api/') && r.status >= 400) fails.push(r.url().replace(BASE, '') + ' -> ' + r.status); });

  await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.evaluate((t) => { sessionStorage.setItem('yy-cloud-key', t); sessionStorage.setItem('yy-user-control', t); }, TOKEN);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.waitForTimeout(6000);

  const me = await page.evaluate(async () => {
    const r = await fetch('/api/identity/me');
    return { status: r.status, body: (await r.text()).slice(0, 300) };
  });
  const accounts = await page.evaluate(async () => {
    const r = await fetch('/api/youyuan/accounts?view=summary');
    return { status: r.status, body: (await r.text()).slice(0, 600) };
  });
  console.log(JSON.stringify({ me, accounts, fails: fails.slice(0, 15) }, null, 1));
  await page.screenshot({ path: 'p2-loggedin-desktop.png' });
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
