const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(2500);
  await page.evaluate(() => sessionStorage.setItem('yy-user-control', '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07'));
  await page.reload({waitUntil: 'domcontentloaded'});
  await page.waitForTimeout(4000);

  console.log('[title]', await page.title());
  const gate = await page.evaluate(() => !!document.querySelector('#yy-identity-gate'));
  console.log('[gate after login]', gate);
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu20-loggedin-3s.png'});
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu21-loggedin-full.png', fullPage: true});

  const bodyText = await page.evaluate(() => document.body.innerText.slice(0, 3500));
  console.log('[logged bodyText] ===\n' + bodyText + '\n=== end');

  // 顶部账号区显示什么
  const account = await page.evaluate(() => {
    const b = document.querySelector('.yy-account-selector') || document.querySelector('button');
    return b ? b.innerText : null;
  });
  console.log('[account button]', JSON.stringify(account));

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
