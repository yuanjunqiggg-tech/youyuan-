// review-ui 探查 8：账号页专项——主令牌开关、账号详情、皮肤
const { chromium } = require('playwright-core');
const BASE = 'https://toolbox.youyuanqi.dpdns.org/';
const EXE = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const TOKEN = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';

(async () => {
  const browser = await chromium.launch({ executablePath: EXE, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  const skinReqs = [];
  page.on('response', r => { if (r.url().includes('/api/skin/')) skinReqs.push(r.url().replace(BASE, '') + ' -> ' + r.status()); });

  await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.evaluate((t) => { sessionStorage.setItem('yy-cloud-key', t); sessionStorage.setItem('yy-user-control', t); }, TOKEN);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.waitForTimeout(4000);

  // 进入游戏账号页
  await page.evaluate(() => {
    const b = [...document.querySelectorAll('button')].find(x => (x.textContent || '').trim() === '游戏账号' && x.offsetParent !== null);
    if (b) b.click();
  });
  await page.waitForTimeout(5000);

  // 1. 关闭"只显示主令牌"
  const toggleInfo = await page.evaluate(() => {
    const labels = [...document.querySelectorAll('label, span, div')].filter(x => (x.textContent || '').trim() === '只显示主令牌' && x.offsetParent !== null);
    let sw = null;
    if (labels.length) {
      let p = labels[0].parentElement;
      for (let i = 0; i < 4 && p; i++) { sw = p.querySelector('input[type=checkbox], [role=switch], button'); if (sw) break; p = p.parentElement; }
    }
    return { found: !!sw, tag: sw && sw.tagName, checked: sw && (sw.checked !== undefined ? sw.checked : sw.getAttribute('aria-checked')) };
  });
  if (toggleInfo.found) {
    await page.evaluate(() => {
      const labels = [...document.querySelectorAll('label, span, div')].filter(x => (x.textContent || '').trim() === '只显示主令牌' && x.offsetParent !== null);
      let p = labels[0].parentElement;
      for (let i = 0; i < 4 && p; i++) { const sw = p.querySelector('input[type=checkbox], [role=switch], button'); if (sw) { sw.click(); return; } p = p.parentElement; }
    });
    await page.waitForTimeout(3500);
  }
  const afterToggle = await page.evaluate(() => ({
    pageText: document.body.innerText.match(/第 \d+ \/ \d+ 页[^\n]*/g),
    cards: [...document.querySelectorAll('[class*=account], [class*=card]')].map(c => (c.textContent || '').trim().slice(0, 60)).filter(t => t.includes('主账号') || t.includes('儒雅') || t.length > 6).slice(0, 15),
  }));
  await page.screenshot({ path: 'a1-allaccounts.png' });

  // 2. 点第一个账号卡片进详情
  const before = skinReqs.length;
  const entered = await page.evaluate(() => {
    const cards = [...document.querySelectorAll('div, li, article')].filter(x => {
      const t = x.textContent || '';
      return (t.includes('儒雅猎杀者') || t.includes('布汲岛')) && x.querySelector('img, [class*=avatar], [class*=skin]') && t.length < 200 && x.offsetParent !== null;
    });
    if (cards.length) { cards[cards.length - 1].click(); return true; }
    return false;
  });
  await page.waitForTimeout(6000);
  const detail = await page.evaluate(() => ({
    head: document.body.innerText.slice(0, 1200),
    imgs: [...document.images].map(i => ({ src: i.src.slice(0, 110), w: i.naturalWidth, h: i.naturalHeight, ok: i.complete && i.naturalWidth > 0 })).filter(i => i.src.includes('skin') || i.src.includes('avatar') || i.src.includes('head') || i.src.includes('texture')).slice(0, 10),
    allImgs: [...document.images].filter(i => i.src && !i.src.startsWith('data:')).map(i => i.src.slice(0, 100)).slice(0, 12),
  }));
  await page.screenshot({ path: 'a2-detail.png' });
  console.log(JSON.stringify({ toggleInfo, afterToggle, entered, newSkinReqs: skinReqs.slice(before), detail }, null, 1).slice(0, 7000));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
