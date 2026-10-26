const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 390, height: 844}, isMobile: true, hasTouch: true, deviceScaleFactor: 2});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  const t0 = Date.now();
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3000);
  console.log('[load]', Date.now()-t0, 'ms');

  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu10-mobile-3s.png'});

  const mobileInfo = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const card = g ? g.querySelector('div') : null;
    const r = card ? card.getBoundingClientRect() : null;
    return {
      gateText: g ? g.innerText.slice(0, 400) : null,
      cardRect: r ? {w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top)} : null,
      viewport: {w: innerWidth, h: innerHeight},
      bodyScrollW: document.body.scrollWidth,
      // 输入框在手机上的大小
      inputRects: [...document.querySelectorAll('#yy-identity-gate input')].map(i => {
        const ir = i.getBoundingClientRect();
        return {type: i.type, w: Math.round(ir.width), h: Math.round(ir.height)};
      }),
    };
  });
  console.log('[mobile]', JSON.stringify(mobileInfo, null, 1));

  // 手机上点输入框，看键盘会不会把内容顶掉（模拟：focus 后截图）
  await page.locator('#yy-identity-gate input[type=text]').first().tap();
  await page.waitForTimeout(800);
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu11-mobile-input-focus.png'});
  const focusInfo = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const card = g ? g.querySelector('div') : null;
    const r = card ? card.getBoundingClientRect() : null;
    return {cardTop: r ? Math.round(r.top) : null, scrollY: window.scrollY, active: document.activeElement ? document.activeElement.type : null};
  });
  console.log('[mobile focus]', JSON.stringify(focusInfo));

  // 弹窗外能否滚动
  await page.touchscreen.tap(195, 60);
  await page.waitForTimeout(400);
  console.log('[mobile outside tap] scrollY =', await page.evaluate(() => window.scrollY));

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
