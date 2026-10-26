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
  page.on('response', r => { if (r.url().includes('/api/identity')) console.log('[net]', r.status(), r.request().method(), r.url().replace('https://toolbox.youyuanqi.dpdns.org','')); });
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  await page.waitForTimeout(3000);

  // favicon + video
  const meta = await page.evaluate(() => {
    const links = [...document.querySelectorAll('link[rel*=icon]')].map(l => l.href);
    const v = document.querySelector('video');
    return {
      favicons: links,
      title: document.title,
      video: v ? { src: (v.currentSrc||'').slice(0,120), paused: v.paused, readyState: v.readyState, w: v.videoWidth, h: v.videoHeight, poster: (v.poster||'').slice(0,80) } : null,
      bodyBg: getComputedStyle(document.body).backgroundColor,
      appBg: getComputedStyle(document.querySelector('#app')).backgroundImage.slice(0, 120),
    };
  });
  console.log('META', JSON.stringify(meta, null, 1));

  // existing device key with wrong key
  await page.getByText('已有设备的控制密钥').click();
  await page.waitForTimeout(600);
  await page.fill('#yy-login-existing-key', '1234567890');
  await page.getByRole('button', { name: '连接已有设备' }).click();
  await page.waitForTimeout(3500);
  await page.screenshot({ path: OUT + '27-device-key-error.png' });
  const msg = await page.evaluate(() => ({
    card: (document.querySelector('.yy-identity-card')||{}).innerText || '',
    toast: (document.querySelector('#toast')||{}).innerText || '',
    toastCls: (document.querySelector('#toast')||{}).className || '',
  }));
  console.log(JSON.stringify(msg, null, 1));

  // reload and see if anything remembered
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(2500);
  const after = await page.evaluate(() => ({ name: (document.querySelector('#yy-login-name')||{}).value, card: (document.querySelector('.yy-identity-card')||{}).innerText.slice(0,120) }));
  console.log('AFTER RELOAD', JSON.stringify(after));
  await page.screenshot({ path: OUT + '28-after-reload.png' });
  await browser.close();
})();
