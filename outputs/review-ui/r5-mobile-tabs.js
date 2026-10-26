// review-ui 探查 5：移动视口逐个走查导航功能
const { chromium } = require('playwright-core');
const BASE = 'https://toolbox.youyuanqi.dpdns.org/';
const EXE = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const TOKEN = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';
const TABS = ['工具总览', '插件中心', '终端', '导入建筑', '导出建筑', '命令链', '连接与设置', '游戏账号', '群服互通', '远程设备'];

(async () => {
  const browser = await chromium.launch({ executablePath: EXE, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 });
  const page = await ctx.newPage();
  const errors = [];
  page.on('console', m => { if (m.type() === 'error') errors.push(m.text().slice(0, 160)); });
  page.on('pageerror', e => errors.push('PAGEERROR ' + String(e).slice(0, 160)));

  await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.evaluate((t) => { sessionStorage.setItem('yy-cloud-key', t); sessionStorage.setItem('yy-user-control', t); }, TOKEN);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.waitForTimeout(5000);

  const results = [];
  for (const tab of TABS) {
    const t0 = Date.now();
    // 移动端：先打开汉堡菜单（若存在）再点导航
    const clicked = await page.evaluate((name) => {
      const btns = [...document.querySelectorAll('button, a, [role=tab]')];
      const b = btns.find(x => (x.textContent || '').trim().startsWith(name) && x.offsetParent !== null);
      if (b) { b.click(); return true; }
      return false;
    }, tab);
    await page.waitForTimeout(5000);
    const snap = await page.evaluate(() => {
      return {
        url: location.href,
        textLen: document.body.innerText.length,
        textHead: document.body.innerText.slice(0, 500),
        overflowX: document.documentElement.scrollWidth > document.documentElement.clientWidth + 2,
        scrollW: document.documentElement.scrollWidth,
        clientW: document.documentElement.clientWidth,
        bottomNav: [...document.querySelectorAll('[class*=tabbar] button, [class*=bottom] button')].map(b => (b.textContent || '').trim()).filter(Boolean).slice(0, 8),
      };
    });
    const shot = 'm-' + tab + '.png';
    await page.screenshot({ path: shot, fullPage: false });
    results.push({ tab, clicked, ms: Date.now() - t0, snap, shot });
    console.log('=== ' + tab + ' clicked=' + clicked + ' overflowX=' + snap.overflowX + ' scrollW=' + snap.scrollW + ' textLen=' + snap.textLen);
  }
  console.log('\n===== ERRORS =====');
  console.log(JSON.stringify([...new Set(errors)].slice(0, 20), null, 1));
  require('fs').writeFileSync('m-tabs.json', JSON.stringify(results, null, 1));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
