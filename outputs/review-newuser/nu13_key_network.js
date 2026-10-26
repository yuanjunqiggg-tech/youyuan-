const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 200)));

  page.on('response', async r => {
    if (r.url().includes('/api/identity') || r.url().includes('/api/bot') || r.url().includes('/api/youyuan/remote')) {
      let body = '';
      try { body = (await r.text()).slice(0, 200); } catch (e) {}
      console.log('[resp]', r.request().method(), r.url().replace('https://toolbox.youyuanqi.dpdns.org', ''), '->', r.status(), body);
    }
  });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3500);

  await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const det = g.querySelector('details') || g.querySelector('summary');
    if (det) det.click();
  });
  await page.waitForTimeout(400);

  await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const el = g.querySelector('#yy-login-existing-key');
    el.value = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';
    el.dispatchEvent(new Event('input', {bubbles: true}));
    const btn = Array.from(g.querySelectorAll('button')).find(b => (b.innerText || '').trim() === '连接已有设备');
    if (btn) btn.click();
  });
  await page.waitForTimeout(6000);

  const st = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const cf = document.querySelector('#cf-chl-widget-lradk_response');
    return {gate: !!g, cfValue: cf ? (cf.value || '').slice(0, 20) : null};
  });
  console.log('[state]', JSON.stringify(st));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
