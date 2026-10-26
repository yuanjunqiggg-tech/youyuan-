'use strict';
const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1360, height: 900 } });
  const page = await ctx.newPage();
  const errs = [];
  page.on('pageerror', e => errs.push('pageerror ' + e.message));
  page.on('console', m => { if (m.type() === 'error' && !/Failed to load resource/.test(m.text())) errs.push('console ' + m.text()); });
  let tileReqs = 0;
  page.on('request', r => { if (/\/api\/seed\/v2\/tile\.png/.test(r.url())) tileReqs++; });

  await page.goto('http://127.0.0.1:8787/index.html', { waitUntil: 'domcontentloaded' });
  await page.addStyleTag({ content: '#yy-identity-gate,#yy-intro-overlay,video{display:none!important}' });
  await page.waitForTimeout(900);
  await page.evaluate(() => window.yyOpenModule('seedmap2'));
  await page.waitForTimeout(2200);

  const read = () => page.evaluate(() => ({
    readout: document.querySelector('#sm2-readout').textContent.replace(/\s+/g, ' '),
    tiles: document.querySelectorAll('#sm2-plane img.sm2-tile').length,
    preview: document.querySelectorAll('.sm2-preview').length,
    icons: document.querySelectorAll('#sm2-icons .sm2-icon').length,
    visibleIcons: Array.from(document.querySelectorAll('#sm2-icons .sm2-icon')).filter(e => e.style.display !== 'none').length,
    chips: Array.from(document.querySelectorAll('.sm2-chip')).map(e => e.textContent.trim()),
    list: Array.from(document.querySelectorAll('#sm2-list .sm2-row')).slice(0, 3).map(e => e.textContent.replace(/\s+/g, ' ')),
    sh: Array.from(document.querySelectorAll('#sm2-shlist .sm2-row')).map(e => e.textContent.replace(/\s+/g, ' ')).slice(0, 3)
  }));

  console.log('初始', JSON.stringify(await read()));
  const box = await page.locator('#sm2-map').boundingBox();
  const cx = box.x + box.width / 2, cy = box.y + box.height / 2;

  // 滚轮缩放（不按 Ctrl）
  const before = tileReqs;
  await page.mouse.move(cx, cy);
  await page.mouse.wheel(0, -400);
  await page.waitForTimeout(900);
  console.log('滚轮放大后', JSON.stringify(await read()), '新瓦片请求', tileReqs - before);

  // 拖动
  await page.mouse.move(cx, cy);
  await page.mouse.down();
  await page.mouse.move(cx + 150, cy + 90, { steps: 8 });
  await page.mouse.up();
  await page.waitForTimeout(800);
  console.log('拖动后', JSON.stringify(await read()));

  // 侧栏点击第一条结构
  const row = page.locator('#sm2-list .sm2-row').first();
  const rowText = (await row.textContent()).replace(/\s+/g, ' ').trim();
  await row.click();
  await page.waitForTimeout(700);
  console.log('点击列表', rowText, '=>', JSON.stringify(await read()));

  // 切维度：下界
  await page.click('#sm2-dims button[data-dim="nether"]');
  await page.waitForTimeout(1800);
  const nether = await page.evaluate(() => ({
    chips: Array.from(document.querySelectorAll('.sm2-chip')).map(e => e.textContent.trim()),
    slimeWrap: getComputedStyle(document.querySelector('#sm2-slimewrap')).display,
    empty: document.querySelector('#sm2-empty').hidden ? '' : document.querySelector('#sm2-empty').textContent.trim().slice(0, 40),
    icons: document.querySelectorAll('#sm2-icons .sm2-icon').length
  }));
  console.log('下界', JSON.stringify(nether));

  // 切版本：基岩
  await page.evaluate(() => {
    const sel = document.querySelector('#sm2-version');
    const opt = Array.from(sel.options).find(o => o.value.startsWith('bedrock|'));
    sel.value = opt.value;
    sel.dispatchEvent(new Event('change'));
  });
  await page.waitForTimeout(1800);
  console.log('基岩版', JSON.stringify(await page.evaluate(() => ({
    selected: document.querySelector('#sm2-version').value,
    readout: document.querySelector('#sm2-readout').textContent.replace(/\s+/g, ' '),
    icons: document.querySelectorAll('#sm2-icons .sm2-icon').length
  }))));

  // 全部取消勾选再恢复默认
  await page.evaluate(() => document.querySelector('#sm2-layerbody button[data-act="none"]').click());
  await page.waitForTimeout(800);
  console.log('全不选', JSON.stringify(await page.evaluate(() => ({
    list: document.querySelector('#sm2-list').textContent.trim().slice(0, 40),
    notice: document.querySelector('#sm2-summary').textContent.trim()
  }))));
  await page.evaluate(() => document.querySelector('#sm2-layerbody button[data-act="default"]').click());
  await page.waitForTimeout(1500);
  console.log('恢复默认', JSON.stringify(await page.evaluate(() => ({
    chips: Array.from(document.querySelectorAll('.sm2-chip.on')).length,
    icons: document.querySelectorAll('#sm2-icons .sm2-icon').length
  }))));

  console.log('异常', errs);
  await browser.close();
})().catch(e => { console.error(e); process.exit(1); });
