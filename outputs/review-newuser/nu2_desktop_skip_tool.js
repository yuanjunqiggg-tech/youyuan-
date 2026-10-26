const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3000);

  // 1) 点「跳过」
  const skip = page.locator('button:has-text("跳过")').first();
  console.log('[skip] count', await skip.count());
  if (await skip.count()) {
    await skip.click();
    await page.waitForTimeout(800);
    await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu03-after-skip.png'});
    const stillModal = await page.locator('text=登录游园').count();
    console.log('[skip] login modal still open?', stillModal);
  }

  // 2) 主界面可见性检查：modal 遮罩还在吗
  const backdrop = await page.evaluate(() => {
    const els = [...document.querySelectorAll('.yy-modal-mask, .modal-mask, [class*=mask]')];
    return els.map(e => ({cls: e.className, display: getComputedStyle(e).display, opacity: getComputedStyle(e).opacity}));
  });
  console.log('[backdrop]', JSON.stringify(backdrop));

  // 3) 点第一个想点的：地图画（不需要登录的感觉）
  await page.locator('button.yy-tool-open:has-text("地图画")').first().click();
  await page.waitForTimeout(2500);
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu04-dituhua.png'});
  const mapText = await page.evaluate(() => {
    const main = document.querySelector('.yy-main, main, #yyApp') || document.body;
    return main.innerText.slice(0, 2500);
  });
  console.log('[地图画 page] ===\n' + mapText + '\n=== end');
  console.log('[url]', page.url());

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
