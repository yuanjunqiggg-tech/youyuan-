const { chromium, devices } = require('playwright-core');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/';

(async () => {
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: false,
    args: ['--window-size=420,900'],
  });
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
  const page = await ctx.newPage();
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  await page.waitForTimeout(4000);
  await page.screenshot({ path: OUT + '20-mobile-login.png' });
  await page.screenshot({ path: OUT + '21-mobile-login-full.png', fullPage: true });

  const info = await page.evaluate(() => {
    const card = document.querySelector('.yy-identity-card');
    const r = card ? card.getBoundingClientRect() : null;
    return {
      card: r ? { x: r.x, y: r.y, w: r.width, h: r.height } : null,
      vw: innerWidth, vh: innerHeight,
      docH: document.documentElement.scrollHeight,
      hOverflow: document.documentElement.scrollWidth > innerWidth,
    };
  });
  console.log('MOBILE INFO', JSON.stringify(info));

  // register tab on mobile
  await page.getByRole('button', { name: '注册账号' }).click();
  await page.waitForTimeout(1200);
  await page.screenshot({ path: OUT + '22-mobile-register.png' });
  await page.screenshot({ path: OUT + '23-mobile-register-full.png', fullPage: true });

  // scroll to bottom
  await page.evaluate(() => window.scrollTo(0, 99999));
  await page.waitForTimeout(600);
  await page.screenshot({ path: OUT + '24-mobile-register-bottom.png' });

  await browser.close();
})();
