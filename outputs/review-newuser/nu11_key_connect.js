const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 200)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3500);

  // 展开「已有设备的控制密钥」并填入
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
  console.log('[clicked connect]');

  for (let i = 1; i <= 8; i++) {
    await page.waitForTimeout(2000);
    const st = await page.evaluate(() => {
      const g = document.querySelector('#yy-identity-gate');
      const texts = g ? Array.from(g.querySelectorAll('p, small, div, span')).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 90) : [];
      return {gate: !!g, last: texts.slice(-6)};
    });
    console.log(i * 2 + 's', JSON.stringify(st));
    if (!st.gate) break;
  }
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu23-key-connect.png'});
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
