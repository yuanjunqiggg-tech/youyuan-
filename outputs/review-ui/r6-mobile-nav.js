// review-ui 探查 6：移动端导航结构探测
const { chromium } = require('playwright-core');
const BASE = 'https://toolbox.youyuanqi.dpdns.org/';
const EXE = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const TOKEN = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';

(async () => {
  const browser = await chromium.launch({ executablePath: EXE, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 });
  const page = await ctx.newPage();
  await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.evaluate((t) => { sessionStorage.setItem('yy-cloud-key', t); sessionStorage.setItem('yy-user-control', t); }, TOKEN);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.waitForTimeout(5000);
  await page.screenshot({ path: 'm0-home.png' });

  // 列出所有可见可点击元素
  const els = await page.evaluate(() => {
    return [...document.querySelectorAll('button, a, [role=tab], [onclick]')]
      .filter(x => x.offsetParent !== null)
      .map(x => ({ tag: x.tagName, text: (x.textContent || '').trim().slice(0, 24), cls: (x.className || '').toString().slice(0, 50) }))
      .slice(0, 70);
  });
  console.log(JSON.stringify(els, null, 1));

  // 尝试点汉堡/菜单按钮
  for (const sel of ['[class*=hamburger]', '[class*=menu]', '[aria-label*=菜单]', '.yy-top-icon', 'header button']) {
    try {
      const el = await page.$(sel);
      if (el) { await el.click(); await page.waitForTimeout(1200); await page.screenshot({ path: 'm0-after-' + sel.replace(/[^a-z]/gi, '') + '.png' }); console.log('clicked', sel); break; }
    } catch (e) {}
  }
  const els2 = await page.evaluate(() => {
    return [...document.querySelectorAll('button, a, [role=tab]')]
      .filter(x => x.offsetParent !== null)
      .map(x => ({ tag: x.tagName, text: (x.textContent || '').trim().slice(0, 24) }))
      .slice(0, 40);
  });
  console.log('after-open:', JSON.stringify(els2));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
