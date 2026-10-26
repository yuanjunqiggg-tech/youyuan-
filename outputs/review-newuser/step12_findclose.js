const { chromium } = require('playwright-core');
(async () => {
  const browser = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 60000 });
  await page.waitForTimeout(3000);
  const res = await page.evaluate(() => {
    const words = ['游客', '稍后', '试用', '关闭', '取消', '跳过', '先看看', '返回', '×', '✕'];
    const hits = [];
    document.querySelectorAll('button,a,span,div').forEach(el => {
      const t = (el.innerText || '').trim();
      if (t && t.length < 12 && words.some(w => t.includes(w)) && el.children.length === 0) {
        const r = el.getBoundingClientRect();
        const cs = getComputedStyle(el);
        hits.push({ t, x: Math.round(r.x), y: Math.round(r.y), op: cs.opacity, vis: cs.visibility, disp: cs.display, pe: cs.pointerEvents });
      }
    });
    return hits;
  });
  console.log(JSON.stringify(res, null, 1));
  await browser.close();
})();
