const { chromium } = require('playwright-core');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-newuser/';

(async () => {
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: false,
    args: ['--window-size=1440,960'],
  });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  await page.waitForTimeout(8000);
  await page.screenshot({ path: OUT + '06-headed-login-8s.png' });

  // inspect skip button styles
  const skipInfo = await page.evaluate(() => {
    const btns = [...document.querySelectorAll('button')].filter(b => (b.innerText||'').trim() === '跳过');
    if (!btns.length) return null;
    const b = btns[0];
    const cs = getComputedStyle(b);
    const chain = [];
    let e = b;
    for (let i = 0; i < 5 && e; i++) {
      const c = getComputedStyle(e);
      chain.push({ tag: e.tagName, cls: (e.className||'').toString().slice(0,60), op: c.opacity, color: c.color, bg: c.backgroundColor, z: c.zIndex, pos: c.position });
      e = e.parentElement;
    }
    return { color: cs.color, opacity: cs.opacity, chain };
  });
  console.log('SKIP INFO', JSON.stringify(skipInfo, null, 1));

  // try ESC and backdrop click
  await page.keyboard.press('Escape');
  await page.waitForTimeout(800);
  let still = await page.evaluate(() => document.body.innerText.includes('登录游园'));
  console.log('after ESC still login?', still);
  await page.mouse.click(120, 450);
  await page.waitForTimeout(800);
  still = await page.evaluate(() => document.body.innerText.includes('登录游园'));
  console.log('after backdrop click still login?', still);
  await page.screenshot({ path: OUT + '07-headed-after-esc-backdrop.png' });

  // look at 注册账号 tab
  const tabs = await page.evaluate(() => [...document.querySelectorAll('button')].filter(b=>['登录','注册账号'].includes((b.innerText||'').trim())).map(b=>{
    const r=b.getBoundingClientRect(); return {t:b.innerText.trim(),x:r.x,y:r.y,w:r.width,h:r.height};
  }));
  console.log('TABS', JSON.stringify(tabs));
  await browser.close();
})();
