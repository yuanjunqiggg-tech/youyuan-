const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(3000);

  // 弹窗结构：有关闭按钮吗？
  const modalInfo = await page.evaluate(() => {
    const gate = document.querySelector('#yy-identity-gate');
    if (!gate) return 'no gate';
    const btns = [...gate.querySelectorAll('button')].map(b => (b.innerText||'').trim() || b.getAttribute('aria-label') || '(空)');
    return {cls: gate.className, buttons: btns, text: gate.innerText.slice(0, 500)};
  });
  console.log('[gate]', JSON.stringify(modalInfo, null, 1));

  // 试 Esc
  await page.keyboard.press('Escape');
  await page.waitForTimeout(600);
  const gateAfterEsc = await page.evaluate(() => !!document.querySelector('#yy-identity-gate'));
  console.log('[esc] gate still there?', gateAfterEsc);

  // 试点弹窗外面（左上角空白）
  await page.mouse.click(30, 80);
  await page.waitForTimeout(600);
  const gateAfterOutside = await page.evaluate(() => !!document.querySelector('#yy-identity-gate'));
  console.log('[outside click] gate still there?', gateAfterOutside);
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu05-gate-stuck.png'});

  // 试试「注册账号」
  await page.locator('#yy-identity-gate button:has-text("注册账号")').first().click();
  await page.waitForTimeout(1200);
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu06-register.png'});
  const regText = await page.evaluate(() => {
    const gate = document.querySelector('#yy-identity-gate') || document.body;
    return gate.innerText.slice(0, 2000);
  });
  console.log('[register] ===\n' + regText + '\n=== end');

  // 空着表单直接点注册/登录试试
  const regBtns = await page.evaluate(() => {
    const gate = document.querySelector('#yy-identity-gate') || document.body;
    return [...gate.querySelectorAll('button')].map(b => (b.innerText||'').trim().slice(0,30));
  });
  console.log('[register buttons]', JSON.stringify(regBtns));

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
