const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  for (let i = 1; i <= 8; i++) {
    await page.waitForTimeout(1000);
    const info = await page.evaluate(() => {
      const g = document.querySelector('#yy-identity-gate');
      if (!g) return {gate: false};
      return {
        gate: true,
        inputs: Array.from(g.querySelectorAll('input')).map(x => ({t: x.type, ph: x.placeholder || '', vis: x.getBoundingClientRect().width > 0})),
        btns: Array.from(g.querySelectorAll('button')).map(b => (b.innerText || '').trim().slice(0, 12)).filter(Boolean),
      };
    });
    console.log(i + 's', JSON.stringify(info));
    if (info.gate && info.inputs && info.inputs.some(x => x.vis)) break;
  }
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
