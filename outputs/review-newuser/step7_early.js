const { chromium } = require('playwright-core');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/';

(async () => {
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: true,
  });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();

  // early frames: is there a dashboard flash?
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'commit', timeout: 60000 });
  await page.waitForTimeout(400);
  await page.screenshot({ path: OUT + '15-t0400ms.png' });
  await page.waitForTimeout(600);
  await page.screenshot({ path: OUT + '16-t1000ms.png' });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: OUT + '17-t2000ms.png' });

  // find skip coords
  const skip = await page.evaluate(() => {
    const b = [...document.querySelectorAll('button')].find(x => (x.innerText||'').trim() === '跳过');
    if (!b) return null;
    const r = b.getBoundingClientRect();
    const cs = getComputedStyle(b);
    return { x: r.x + r.width/2, y: r.y + r.height/2, color: cs.color, bg: cs.backgroundColor, op: cs.opacity, fs: cs.fontSize, zi: cs.zIndex, text: b.outerHTML.slice(0,200) };
  });
  console.log('skip', JSON.stringify(skip));

  if (skip) {
    await page.mouse.click(skip.x, skip.y);
    await page.waitForTimeout(2000);
    await page.screenshot({ path: OUT + '18-after-real-skip-click.png' });
    const still = await page.evaluate(() => document.body.innerText.includes('登录游园'));
    console.log('still login after real click?', still);
    const toast = await page.evaluate(() => (document.querySelector('#toast')||{}).innerText || '');
    console.log('toast', toast);
  }
  await browser.close();
})();
