const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3500);

  // A) 乱填账号密码登录
  await page.locator('#yy-identity-gate input[type=text]').first().fill('路人甲');
  await page.locator('#yy-identity-gate input[type=password]').first().fill('12345678');
  await page.locator('#yy-identity-gate button:has-text("登录")').last().click();
  await page.waitForTimeout(3500);
  const afterLogin = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    if (!g) return 'gate gone';
    const smalls = Array.from(g.querySelectorAll('p, small, div')).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 80);
    return {gate: true, texts: smalls.slice(-10)};
  });
  console.log('[A wrong login]', JSON.stringify(afterLogin));
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu12-wrong-login.png'});

  // B) 注册：非 QQ 邮箱收验证码
  await page.locator('#yy-identity-gate button:has-text("注册账号")').first().click();
  await page.waitForTimeout(600);
  await page.locator('#yy-identity-gate input[type=text]').first().fill('路人甲');
  await page.locator('#yy-identity-gate input[type=email]').first().fill('lurenjia@gmail.com');
  await page.locator('#yy-identity-gate button:has-text("发送验证码")').first().click();
  await page.waitForTimeout(3500);
  const afterCode = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    if (!g) return 'gate gone';
    const smalls = Array.from(g.querySelectorAll('p, small, div')).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 80);
    return smalls.slice(-10);
  });
  console.log('[B gmail code]', JSON.stringify(afterCode));
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu13-gmail-code.png'});

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
