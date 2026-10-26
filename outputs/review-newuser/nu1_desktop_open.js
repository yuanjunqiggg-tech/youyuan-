const {chromium} = require('playwright-core');

(async () => {
  const browser = await chromium.launch({executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'});
  const ctx = await browser.newContext({viewport: {width: 1440, height: 900}});
  const page = await ctx.newPage();

  page.on('console', m => console.log('[console]', m.type(), m.text().slice(0, 200)));
  page.on('pageerror', e => console.log('[pageerror]', String(e).slice(0, 300)));
  page.on('dialog', async d => { console.log('[dialog]', d.type(), d.message().slice(0,200)); await d.dismiss(); });

  const t0 = Date.now();
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil: 'domcontentloaded', timeout: 60000});
  console.log('[load] domcontentloaded after', Date.now()-t0, 'ms');

  // 前 3 秒视线
  await page.waitForTimeout(3000);
  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu01-desktop-3s.png'});
  console.log('[title]', await page.title());
  console.log('[url]', page.url());

  // 页面可见文本（首屏）
  const bodyText = await page.evaluate(() => document.body ? document.body.innerText : '(no body)');
  console.log('[bodyText] ===\n' + bodyText.slice(0, 4000) + '\n=== end');

  // 列出可点的东西：按钮/链接
  const clickables = await page.evaluate(() => {
    const out = [];
    document.querySelectorAll('button, a, [role=button], input, select, summary').forEach(el => {
      const r = el.getBoundingClientRect();
      if (r.width > 0 && r.height > 0) {
        out.push({
          tag: el.tagName,
          text: (el.innerText || el.value || el.getAttribute('aria-label') || el.getAttribute('placeholder') || '').trim().slice(0, 60),
          type: el.type || '',
          cls: (el.className || '').toString().slice(0, 60),
        });
      }
    });
    return out.slice(0, 120);
  });
  console.log('[clickables]', JSON.stringify(clickables, null, 1));

  await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/nu02-desktop-full.png', fullPage: true});
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
