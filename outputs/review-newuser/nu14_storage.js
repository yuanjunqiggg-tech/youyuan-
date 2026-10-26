const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3000);
  const dump = await page.evaluate(() => {
    const out = {};
    for (const k of Object.keys(localStorage)) {
      const v = localStorage.getItem(k) || '';
      out['LS:' + k] = v.slice(0, 150);
    }
    for (const k of Object.keys(sessionStorage)) {
      const v = sessionStorage.getItem(k) || '';
      out['SS:' + k] = v.slice(0, 150);
    }
    return out;
  });
  console.log(JSON.stringify(dump, null, 1));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
