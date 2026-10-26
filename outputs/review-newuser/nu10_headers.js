const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();

  page.on('request', r => {
    if (r.url().includes('/api/identity/me') || r.url().includes('/api/config')) {
      console.log('[req]', r.url().slice(-40), JSON.stringify(r.headers()));
    }
  });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(1500);
  await page.evaluate(() => sessionStorage.setItem('yy-user-control', '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07'));
  await page.reload({waitUntil: 'domcontentloaded'});
  await page.waitForTimeout(5000);

  // 也看看 cookie
  const cookies = await ctx.cookies();
  console.log('[cookies]', JSON.stringify(cookies.map(c => c.name)));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
