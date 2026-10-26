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
  page.on('response', async r => {
    if (r.url().includes('/api/') || r.url().includes('auth')) {
      console.log('[net]', r.status(), r.request().method(), r.url().slice(0, 120));
    }
  });
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  await page.waitForTimeout(3500);

  await page.fill('#yy-login-name', 'testuser123');
  await page.locator('input[type=password]').first().fill('abc123456789');
  const t0 = Date.now();
  await page.getByRole('button', { name: '登录', exact: true }).last().click();
  await page.waitForTimeout(1500);
  await page.screenshot({ path: OUT + '13-login-waiting.png' });
  await page.waitForTimeout(9000);
  console.log('elapsed', Date.now() - t0);
  await page.screenshot({ path: OUT + '14-login-after-10s.png' });
  const st = await page.evaluate(() => ({
    toast: (document.querySelector('#toast') || {}).innerText || '',
    toastCls: (document.querySelector('#toast') || {}).className || '',
    card: (document.querySelector('.yy-identity-card') || {}).innerText || '',
    btn: [...document.querySelectorAll('button')].map(b => b.innerText.trim()).filter(t => t.includes('登录')),
  }));
  console.log(JSON.stringify(st, null, 1));
  await browser.close();
})();
