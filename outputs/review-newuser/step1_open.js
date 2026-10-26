const { chromium } = require('playwright-core');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/';

(async () => {
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: true,
  });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 });
  const page = await ctx.newPage();
  const logs = [];
  page.on('console', m => logs.push(`[${m.type()}] ${m.text()}`));
  page.on('pageerror', e => logs.push(`[pageerror] ${e.message}`));

  const t0 = Date.now();
  try {
    await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  } catch (e) {
    console.log('GOTO ERR', e.message);
  }
  console.log('domcontentloaded at', Date.now() - t0, 'ms');
  await page.waitForTimeout(3000);
  await page.screenshot({ path: OUT + '01-desktop-first-3s.png', fullPage: false });
  await page.screenshot({ path: OUT + '02-desktop-first-full.png', fullPage: true });
  console.log('TITLE:', await page.title());
  console.log('URL:', page.url());
  const text = await page.evaluate(() => document.body.innerText);
  console.log('---BODY TEXT---');
  console.log(text.slice(0, 4000));
  console.log('---LINKS/BUTTONS---');
  const items = await page.evaluate(() => {
    const arr = [];
    document.querySelectorAll('a,button,[role=button],input,textarea,select').forEach((el, i) => {
      if (i > 120) return;
      const r = el.getBoundingClientRect();
      if (r.width === 0 && r.height === 0) return;
      arr.push({
        tag: el.tagName,
        text: (el.innerText || el.value || el.placeholder || el.getAttribute('aria-label') || '').trim().slice(0, 60),
        href: el.getAttribute('href') || '',
        x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
      });
    });
    return arr;
  });
  console.log(JSON.stringify(items, null, 1));
  console.log('---CONSOLE---');
  console.log(logs.slice(0, 60).join('\n'));
  await browser.close();
})();
