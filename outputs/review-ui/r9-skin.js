// review-ui 探查 9：账号详情皮肤预览 + 皮肤库
const { chromium } = require('playwright-core');
const BASE = 'https://toolbox.youyuanqi.dpdns.org/';
const EXE = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const TOKEN = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';

(async () => {
  const browser = await chromium.launch({ executablePath: EXE, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  const skinLog = [];
  page.on('response', r => { if (r.url().includes('/api/skin/')) skinLog.push(r.url().replace(BASE, '').slice(0, 120) + ' -> ' + r.status()); });

  await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.evaluate((t) => { sessionStorage.setItem('yy-cloud-key', t); sessionStorage.setItem('yy-user-control', t); }, TOKEN);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.waitForTimeout(4000);
  await page.evaluate(() => {
    const b = [...document.querySelectorAll('button')].find(x => (x.textContent || '').trim() === '游戏账号' && x.offsetParent !== null);
    if (b) b.click();
  });
  await page.waitForTimeout(4500);

  // 展开全部账号
  await page.evaluate(() => {
    const labels = [...document.querySelectorAll('label, span, div')].filter(x => (x.textContent || '').trim() === '只显示主令牌' && x.offsetParent !== null);
    if (labels.length) { let p = labels[0].parentElement; for (let i = 0; i < 4 && p; i++) { const sw = p.querySelector('input[type=checkbox], [role=switch], button'); if (sw) { sw.click(); return; } p = p.parentElement; } }
  });
  await page.waitForTimeout(3000);

  // 点"儒雅猎杀者"卡片（机器人账号，有服务器）
  await page.evaluate(() => {
    const cards = [...document.querySelectorAll('*')].filter(x => {
      const t = x.textContent || '';
      return t.includes('儒雅猎杀者') && t.includes('机器人账号') && t.length < 120 && x.offsetParent !== null && x.children.length <= 6;
    });
    if (cards.length) cards[cards.length - 1].click();
  });
  await page.waitForTimeout(8000); // 等皮肤加载
  const detail = await page.evaluate(() => {
    const detailZone = [...document.querySelectorAll('*')].filter(x => (x.textContent || '').includes('拖动旋转') && x.offsetParent !== null);
    const imgs = [...document.images].map(i => ({ src: i.src.slice(0, 130), w: i.naturalWidth, h: i.naturalHeight, vis: i.offsetParent !== null })).filter(i => /skin|head|texture|avatar|ps\.netease|blob/.test(i.src));
    const loadingTexts = [...document.querySelectorAll('*')].filter(x => (x.textContent || '').trim() === '皮肤加载中…' && x.offsetParent !== null).length;
    return {
      hasDetail: detailZone.length > 0,
      imgs: imgs.slice(0, 12),
      loadingTexts,
      detailText: detailZone.length ? detailZone[detailZone.length - 1].textContent.slice(0, 400) : '',
    };
  });
  await page.screenshot({ path: 'a3-skin-detail.png' });
  console.log('DETAIL:', JSON.stringify(detail, null, 1));

  // 返回列表，点皮肤库
  await page.evaluate(() => {
    const b = [...document.querySelectorAll('button')].find(x => (x.textContent || '').trim() === '皮肤库' && x.offsetParent !== null);
    if (b) b.click();
  });
  await page.waitForTimeout(5000);
  const lib = await page.evaluate(() => ({
    head: document.body.innerText.slice(300, 1400),
    imgs: [...document.images].map(i => i.src.slice(0, 120)).filter(s => /skin|head|texture/.test(s)).slice(0, 8),
  }));
  await page.screenshot({ path: 'a4-skinlib.png' });
  console.log('SKINLIB:', JSON.stringify(lib, null, 1));
  console.log('SKINLOG:', JSON.stringify(skinLog, null, 1));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
