const {chromium} = require('playwright-core');
const DIR = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/';

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 200)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });
  page.on('response', r => { if (r.status() >= 400) console.log('[http]', r.status(), r.url().replace('https://toolbox.youyuanqi.dpdns.org', '').slice(0, 70)); });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3000);
  await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const det = g.querySelector('details');
    if (det) det.open = true;
    const el = g.querySelector('#yy-login-existing-key');
    el.value = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';
    el.dispatchEvent(new Event('input', {bubbles: true}));
    Array.from(g.querySelectorAll('button')).find(b => (b.innerText || '').trim() === '连接已有设备').click();
  });
  await page.waitForTimeout(3000);
  console.log('--- reload after connect ---');
  await page.reload({waitUntil: 'domcontentloaded'});
  await page.waitForTimeout(6000);

  const st = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const ss = {};
    for (const k of Object.keys(sessionStorage)) ss[k] = (sessionStorage.getItem(k) || '').slice(0, 30);
    const ls = {};
    for (const k of Object.keys(localStorage)) ls[k] = (localStorage.getItem(k) || '').slice(0, 30);
    return {gate: !!g, ss, ls};
  });
  console.log('[after reload]', JSON.stringify(st, null, 1));

  // 连接状态卡在哪
  const conn = await page.evaluate(() => {
    const stats = Array.from(document.querySelectorAll('.yy-stat')).map(s => (s.innerText || '').replace(/\n/g, ' | ').slice(0, 80));
    return stats;
  });
  console.log('[stats]', JSON.stringify(conn, null, 1));
  await page.screenshot({path: DIR + 'nu31-after-reload.png'});
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
