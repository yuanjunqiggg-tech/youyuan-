const {chromium} = require('playwright-core');
const DIR = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/';

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 200)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });
  page.on('response', r => { if (r.status() >= 400) console.log('[http]', r.status(), r.url().replace('https://toolbox.youyuanqi.dpdns.org', '').slice(0, 80)); });

  const login = async () => {
    await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
    await page.waitForTimeout(3000);
    await page.evaluate(() => {
      const g = document.querySelector('#yy-identity-gate');
      const det = g.querySelector('details');
      if (det) det.open = true;
      const el = g.querySelector('#yy-login-existing-key');
      el.value = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';
      el.dispatchEvent(new Event('input', {bubbles: true}));
      const btn = Array.from(g.querySelectorAll('button')).find(b => (b.innerText || '').trim() === '连接已有设备');
      btn.click();
    });
    await page.waitForTimeout(3500);
    const gate = await page.evaluate(() => !!document.querySelector('#yy-identity-gate'));
    console.log('[login] gate gone?', !gate);
  };

  await login();
  await page.screenshot({path: DIR + 'nu30-in-3s.png'});

  // 首屏文字
  const t = await page.evaluate(() => document.body.innerText.slice(0, 3000));
  console.log('[in body] ===\n' + t + '\n=== end');

  // 账号区长什么样
  const acct = await page.evaluate(() => {
    const b = document.querySelector('.yy-account-selector');
    return b ? b.innerText : '(no account btn)';
  });
  console.log('[account]', JSON.stringify(acct));

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
