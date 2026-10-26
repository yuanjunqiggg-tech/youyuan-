const { chromium } = require('playwright-core');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/';

(async () => {
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: true,
  });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  await page.waitForTimeout(4000);

  // find 跳过
  const skip = await page.evaluate(() => {
    const out = [];
    document.querySelectorAll('*').forEach(el => {
      const t = (el.innerText || '').trim();
      if (t === '跳过' && el.children.length === 0) {
        const r = el.getBoundingClientRect();
        out.push({ tag: el.tagName, cls: el.className, x: r.x, y: r.y, w: r.width, h: r.height, vis: getComputedStyle(el).visibility, op: getComputedStyle(el).opacity });
      }
    });
    return out;
  });
  console.log('SKIP ELEMENTS', JSON.stringify(skip, null, 1));

  // check overlay stacking: what is at center of screen
  const stack = await page.evaluate(() => {
    const els = document.elementsFromPoint(720, 450).slice(0, 6).map(e => ({
      tag: e.tagName, cls: (e.className || '').toString().slice(0, 60),
      bg: getComputedStyle(e).backgroundColor, bf: getComputedStyle(e).backdropFilter
    }));
    return els;
  });
  console.log('STACK AT CENTER', JSON.stringify(stack, null, 1));

  const boxes = await page.evaluate(() => {
    return [...document.querySelectorAll('body > *')].map(e => {
      const r = e.getBoundingClientRect();
      const cs = getComputedStyle(e);
      return { tag: e.tagName, id: e.id, cls: (e.className||'').toString().slice(0,50), x: r.x, y: r.y, w: r.width, h: r.height, disp: cs.display, z: cs.zIndex, bg: cs.backgroundColor };
    });
  });
  console.log('BODY CHILDREN', JSON.stringify(boxes, null, 1));

  await page.screenshot({ path: OUT + '03-desktop-before-skip.png' });

  // try click 跳过
  const clicked = await page.evaluate(() => {
    const els = [...document.querySelectorAll('button,a,span,div')].filter(e => (e.innerText||'').trim() === '跳过');
    if (els.length) { els[0].click(); return els.length; }
    return 0;
  });
  console.log('clicked skip count', clicked);
  await page.waitForTimeout(2500);
  await page.screenshot({ path: OUT + '04-desktop-after-skip.png' });
  await page.screenshot({ path: OUT + '05-desktop-after-skip-full.png', fullPage: true });
  console.log('TEXT AFTER SKIP ---');
  console.log((await page.evaluate(() => document.body.innerText)).slice(0, 1500));
  await browser.close();
})();
