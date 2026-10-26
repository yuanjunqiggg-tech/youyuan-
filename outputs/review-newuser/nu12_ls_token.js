const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('request', r => {
    if (r.url().includes('/api/identity/me')) {
      const h = r.headers();
      console.log('[me req] auth=', h['authorization'] || '(none)', 'x-token=', h['x-youyuan-token'] || '(none)', 'cookie=', h['cookie'] || '(none)');
    }
  });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(1500);
  await page.evaluate(() => {
    const k = 'yy-user-control';
    const v = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';
    localStorage.setItem(k, v);
    sessionStorage.setItem(k, v);
  });
  await page.reload({waitUntil: 'domcontentloaded'});
  await page.waitForTimeout(5000);
  const gate = await page.evaluate(() => !!document.querySelector('#yy-identity-gate'));
  console.log('[gate after ls+ss]', gate);
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
