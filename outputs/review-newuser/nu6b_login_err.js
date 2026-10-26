const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(4000);

  // 输入框状态体检
  const st = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    if (!g) return 'no gate';
    return Array.from(g.querySelectorAll('input')).map(x => {
      const r = x.getBoundingClientRect();
      return {t: x.type, vis: r.width > 0 && r.height > 0, disabled: x.disabled, readonly: x.readOnly, id: x.id, name: x.name};
    });
  });
  console.log('[inputs]', JSON.stringify(st));

  const txt = page.locator('#yy-identity-gate input[type=text]').first();
  console.log('[count]', await txt.count(), 'visible', await txt.isVisible().catch(e => 'ERR:' + e.message.slice(0,80)));
  await txt.fill('路人甲', {timeout: 8000}).catch(e => console.log('[fill err]', e.message.slice(0, 150)));
  await page.locator('#yy-identity-gate input[type=password]').first().fill('12345678', {timeout: 8000}).catch(e => console.log('[fill2 err]', e.message.slice(0, 150)));

  // 勾选 captcha（如果在登录表单里）
  const cf = page.locator('#yy-identity-gate iframe[src*=turnstile], #yy-identity-gate iframe[src*=challenges.cloudflare]').first();
  console.log('[captcha iframe count]', await cf.count());
  if (await cf.count()) {
    try {
      const box = await cf.boundingBox();
      if (box) await page.mouse.click(box.x + box.width/2, box.y + box.height/2);
      await page.waitForTimeout(2500);
    } catch (e) { console.log('[cf click err]', e.message.slice(0, 120)); }
  }

  await page.locator('#yy-identity-gate button:has-text("登录")').last().click({timeout: 8000}).catch(e => console.log('[login click err]', e.message.slice(0, 120)));
  await page.waitForTimeout(4000);
  const after = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    if (!g) return 'gate gone';
    return Array.from(g.querySelectorAll('p, small, div, span')).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 90).slice(-12);
  });
  console.log('[A after wrong login]', JSON.stringify(after));
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu12-wrong-login.png'});

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
