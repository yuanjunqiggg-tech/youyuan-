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
  await page.waitForTimeout(3000);

  // empty submit on login
  await page.getByRole('button', { name: '登录', exact: true }).last().click();
  await page.waitForTimeout(800);
  await page.screenshot({ path: OUT + '25-empty-login-submit.png' });

  // register: non-QQ email
  await page.getByRole('button', { name: '注册账号' }).click();
  await page.waitForTimeout(1000);
  await page.fill('#yy-login-name', 'testuser123');
  const emailInput = page.locator('input[type=email]').first();
  await emailInput.fill('test@gmail.com');
  await page.getByRole('button', { name: /发送验证码/ }).click();
  await page.waitForTimeout(2000);
  await page.screenshot({ path: OUT + '26-register-gmail.png' });
  const msg = await page.evaluate(() => {
    const card = document.querySelector('.yy-identity-card');
    return { card: card.innerText, toast: (document.querySelector('#toast')||{}).innerText||'' };
  });
  console.log(JSON.stringify(msg, null, 1));

  // 6-digit wrong code
  const codeInput = page.locator('input').filter({ hasNot: page.locator('[type=password]') });
  const inputs = await page.evaluate(() => [...document.querySelectorAll('.yy-identity-card input')].map(i => ({type:i.type, id:i.id, ph:i.placeholder})));
  console.log('CARD INPUTS', JSON.stringify(inputs));
  await browser.close();
})();
