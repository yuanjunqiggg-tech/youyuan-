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
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  await page.waitForTimeout(3500);

  // 1) 已有设备的控制密钥
  const det = page.locator('summary, details').first();
  try {
    await page.getByText('已有设备的控制密钥').click({ timeout: 5000 });
  } catch (e) { console.log('detail click fail', e.message.slice(0, 80)); }
  await page.waitForTimeout(1500);
  await page.screenshot({ path: OUT + '10-device-key-expanded.png' });
  const t1 = await page.evaluate(() => {
    const c = document.querySelector('.yy-identity-card');
    return c ? c.innerText : document.body.innerText.slice(-800);
  });
  console.log('--- CARD AFTER EXPAND ---');
  console.log(t1);

  // 2) login with fake creds
  await page.fill('#yy-login-name', 'testuser123').catch(e => console.log('fill name fail', e.message.slice(0,60)));
  const pwd = await page.evaluate(() => {
    const inputs = [...document.querySelectorAll('input[type=password]')];
    return inputs.length;
  });
  console.log('password inputs', pwd);
  await page.locator('input[type=password]').first().fill('abc123456').catch(e => console.log('fill pwd fail', e.message.slice(0,60)));
  await page.screenshot({ path: OUT + '11-login-filled.png' });
  await page.getByRole('button', { name: '登录', exact: true }).last().click().catch(e => console.log('submit fail', e.message.slice(0,80)));
  await page.waitForTimeout(4000);
  await page.screenshot({ path: OUT + '12-login-result.png' });
  const t2 = await page.evaluate(() => {
    const toast = document.querySelector('#toast');
    const card = document.querySelector('.yy-identity-card');
    return { toast: toast ? toast.innerText : '', card: card ? card.innerText.slice(0, 800) : '' };
  });
  console.log('--- AFTER SUBMIT ---');
  console.log(JSON.stringify(t2, null, 1));
  await browser.close();
})();
