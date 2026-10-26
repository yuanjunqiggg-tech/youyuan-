const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3000);

  // 等多一会儿，弹窗会自己消失吗？
  await page.waitForTimeout(10000);
  const gate10s = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    return g ? {exists: true, display: getComputedStyle(g).display} : {exists: false};
  });
  console.log('[10s later]', JSON.stringify(gate10s));

  // 背后页面能滚动吗？（用户会试着滚一下）
  await page.mouse.move(700, 450);
  await page.mouse.wheel(0, 600);
  await page.waitForTimeout(800);
  const scrollY = await page.evaluate(() => window.scrollY);
  console.log('[scroll] window.scrollY =', scrollY);

  // 展开「已有设备的控制密钥」看看要什么
  const det = page.locator('#yy-identity-gate summary, #yy-identity-gate details').first();
  if (await det.count()) {
    await det.click();
    await page.waitForTimeout(500);
  }
  const keyText = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    return g ? g.innerText.slice(0, 1200) : '';
  });
  console.log('[gate full text] ===\n' + keyText + '\n=== end');

  // 用错的密钥试试（用户会乱试）
  const pw = page.locator('#yy-identity-gate input[type=password]').last();
  await pw.fill('123456');
  const useBtn = page.locator('#yy-identity-gate button.btn-s').last();
  const btnText = await useBtn.innerText().catch(()=> '(取不到)');
  console.log('[key button text]', JSON.stringify(btnText));
  await useBtn.click();
  await page.waitForTimeout(2000);
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu07-wrong-key.png'});
  const afterKey = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    return {gate: !!g, text: g ? g.innerText.slice(0, 600) : ''};
  });
  console.log('[after wrong key] ===\n' + JSON.stringify(afterKey) + '\n=== end');

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
