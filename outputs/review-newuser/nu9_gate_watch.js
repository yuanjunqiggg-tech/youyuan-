const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  const req401 = [];
  page.on('response', r => { if (r.status() === 401) req401.push(r.url().slice(0, 120)); });
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 200)));

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(2000);
  await page.evaluate(() => sessionStorage.setItem('yy-user-control', '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07'));
  await page.reload({waitUntil: 'domcontentloaded'});

  for (let i = 1; i <= 10; i++) {
    await page.waitForTimeout(2000);
    const st = await page.evaluate(() => {
      const g = document.querySelector('#yy-identity-gate');
      return {
        gate: !!g,
        ssKeys: Object.keys(sessionStorage),
        lsKeys: Object.keys(localStorage).slice(0, 20),
      };
    });
    console.log(i * 2 + 's', JSON.stringify(st));
    if (!st.gate) break;
  }
  console.log('[401 urls]', JSON.stringify(req401.slice(0, 10)));

  const gateText = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    return g ? g.innerText.slice(0, 400) : 'no gate';
  });
  console.log('[gate text]', gateText);
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu22-loggedin-gate-20s.png'});
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
