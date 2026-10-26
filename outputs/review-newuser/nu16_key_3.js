const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 200)));
  page.on('response', async r => {
    if (r.url().includes('/api/identity') && !r.url().includes('turnstile')) {
      let body = '';
      try { body = (await r.text()).slice(0, 160); } catch (e) {}
      console.log('[resp]', r.request().method(), r.url().replace('https://toolbox.youyuanqi.dpdns.org', ''), '->', r.status(), body);
    }
  });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3500);

  // 展开 details
  const expanded = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const det = g.querySelector('details');
    if (det) { det.open = true; return 'details set open'; }
    const sum = g.querySelector('summary');
    if (sum) { sum.click(); return 'summary clicked'; }
    return 'neither';
  });
  console.log('[expand]', expanded);
  await page.waitForTimeout(500);

  const btns = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    return Array.from(g.querySelectorAll('button')).map(b => (b.innerText || '').trim().slice(0, 20));
  });
  console.log('[buttons]', JSON.stringify(btns));

  // 填钥匙 + 点连接
  const clicked = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const el = g.querySelector('#yy-login-existing-key');
    if (el) { el.value = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07'; el.dispatchEvent(new Event('input', {bubbles: true})); }
    const btn = Array.from(g.querySelectorAll('button')).find(b => (b.innerText || '').trim() === '连接已有设备');
    if (btn) { btn.click(); return true; }
    return false;
  });
  console.log('[connect clicked]', clicked);

  for (let i = 1; i <= 6; i++) {
    await page.waitForTimeout(2000);
    const st = await page.evaluate(() => {
      const g = document.querySelector('#yy-identity-gate');
      const hid = document.querySelector('input[name=cf-turnstile-response]');
      return {gate: !!g, cfTokenLen: hid ? (hid.value || '').length : -1};
    });
    console.log(i * 2 + 's', JSON.stringify(st));
    if (!st.gate) break;
  }
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu24-key-connect-2click.png'});
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
