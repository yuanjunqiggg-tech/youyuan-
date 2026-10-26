const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 200)));
  page.on('response', async r => {
    if (r.url().includes('/api/identity') || r.url().includes('/api/bot')) {
      let body = '';
      try { body = (await r.text()).slice(0, 160); } catch (e) {}
      console.log('[resp]', r.request().method(), r.url().replace('https://toolbox.youyuanqi.dpdns.org', ''), '->', r.status(), body);
    }
  });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3500);

  const fillKey = () => page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const el = g.querySelector('#yy-login-existing-key');
    if (el) { el.value = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07'; el.dispatchEvent(new Event('input', {bubbles: true})); }
  });
  const clickConnect = () => page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const btn = Array.from(g.querySelectorAll('button')).find(b => (b.innerText || '').trim() === '连接已有设备');
    if (btn) btn.click();
    return !!btn;
  });

  // 展开
  await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const det = g.querySelector('details') || g.querySelector('summary');
    if (det) det.click();
  });
  await page.waitForTimeout(400);
  await fillKey();

  // 第一次点
  console.log('[click1]', await clickConnect());
  await page.waitForTimeout(3000);
  // 等 captcha 出现并通过
  for (let i = 0; i < 6; i++) {
    const cf = await page.evaluate(() => {
      const f = document.querySelector('iframe[src*=turnstile], iframe[src*=challenges.cloudflare]');
      const hid = document.querySelector('input[name=cf-turnstile-response]');
      return {iframe: !!f, token: hid ? (hid.value || '').length : 0};
    });
    console.log('[cf]', JSON.stringify(cf));
    if (cf.token > 10) break;
    await page.waitForTimeout(2000);
  }
  // 第二次点
  console.log('[click2]', await clickConnect());
  await page.waitForTimeout(5000);
  const st = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    return {gate: !!g};
  });
  console.log('[state]', JSON.stringify(st));
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu24-key-connect-2click.png'});
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
