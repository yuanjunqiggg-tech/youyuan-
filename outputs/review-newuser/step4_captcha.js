const { chromium } = require('playwright-core');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/';

(async () => {
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: false,
    args: ['--window-size=1440,960'],
  });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  page.on('console', m => { if (m.type() === 'error') console.log('[console.error]', m.text().slice(0, 200)); });
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  await page.waitForTimeout(3000);

  // click the turnstile checkbox
  const iframeCount = page.frames().length;
  console.log('frames', iframeCount);
  const tf = page.frames().find(f => f.url().includes('challenges.cloudflare.com'));
  console.log('turnstile frame', tf ? tf.url() : 'none');
  if (tf) {
    try {
      const box = await tf.locator('body').boundingBox();
      console.log('tf body box', JSON.stringify(box));
      // click the checkbox inside
      await tf.click('input[type=checkbox]', { timeout: 5000 }).catch(e => console.log('cb click fail', e.message.slice(0,100)));
    } catch (e) { console.log('tf err', e.message.slice(0,120)); }
  }
  await page.waitForTimeout(4000);
  await page.screenshot({ path: OUT + '08-after-captcha-click.png' });

  // switch to register
  await page.getByRole('button', { name: '注册账号' }).click();
  await page.waitForTimeout(1200);
  await page.screenshot({ path: OUT + '09-register-tab.png' });
  const fields = await page.evaluate(() => [...document.querySelectorAll('input,textarea,select')].map(i => {
    const r = i.getBoundingClientRect();
    return { type: i.type, name: i.name, ph: i.placeholder, id: i.id, x: Math.round(r.x), y: Math.round(r.y) };
  }));
  console.log('FIELDS', JSON.stringify(fields, null, 1));
  console.log('TEXT', (await page.evaluate(() => document.body.innerText)).slice(-1200));
  await browser.close();
})();
