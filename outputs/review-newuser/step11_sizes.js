const { chromium } = require('playwright-core');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/';

(async () => {
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: true,
  });
  for (const [w, h, name] of [[1366, 768, '30-1366x768'], [1920, 1080, '31-1920x1080'], [1280, 720, '32-1280x720']]) {
    const ctx = await browser.newContext({ viewport: { width: w, height: h } });
    const page = await ctx.newPage();
    await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
    await page.waitForTimeout(3500);
    await page.screenshot({ path: `${OUT}${name}.png` });
    const fit = await page.evaluate(() => {
      const c = document.querySelector('.yy-identity-card');
      const r = c.getBoundingClientRect();
      return { top: Math.round(r.top), bottom: Math.round(r.bottom), h: Math.round(r.height), vh: innerHeight, scroll: document.documentElement.scrollHeight > innerHeight };
    });
    console.log(name, JSON.stringify(fit));
    await ctx.close();
  }
  await browser.close();
})();
