const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  await page.waitForTimeout(4000);

  // 用原生 JS 填表 + 点登录（绕开定位器抽风）
  await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const set = (id, v) => { const el = g.querySelector('#' + id); if (el) { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); } };
    set('yy-login-name', '路人甲');
    set('yy-login-password', '12345678');
    const btns = Array.from(g.querySelectorAll('button')).filter(b => (b.innerText || '').trim() === '登录');
    btns[btns.length - 1].click();
  });
  await page.waitForTimeout(4500);

  const after = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    if (!g) return 'gate gone';
    const texts = Array.from(g.querySelectorAll('p, small, div, span, label')).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 90);
    return texts.slice(-14);
  });
  console.log('[A after wrong login]', JSON.stringify(after, null, 1));
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu12-wrong-login.png'});

  // B) 注册：非 QQ 邮箱
  await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const tab = Array.from(g.querySelectorAll('button')).find(b => (b.innerText || '').trim() === '注册账号');
    if (tab) tab.click();
  });
  await page.waitForTimeout(800);
  await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    const set = (id, v) => { const el = g.querySelector('#' + id); if (el) { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); } };
    set('yy-login-name', '路人甲');
    set('yy-login-email', 'lurenjia@gmail.com');
    const btn = Array.from(g.querySelectorAll('button')).find(b => (b.innerText || '').trim() === '发送验证码');
    if (btn) btn.click();
  });
  await page.waitForTimeout(4000);
  const afterCode = await page.evaluate(() => {
    const g = document.querySelector('#yy-identity-gate');
    if (!g) return 'gate gone';
    const texts = Array.from(g.querySelectorAll('p, small, div, span, label')).map(e => (e.innerText || '').trim()).filter(t => t && t.length < 90);
    return texts.slice(-14);
  });
  console.log('[B after gmail code]', JSON.stringify(afterCode, null, 1));
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu13-gmail-code.png'});

  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
