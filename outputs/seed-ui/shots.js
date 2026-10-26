/* 种子地图 v2 · 截图与自检（不属于交付代码） */
'use strict';
const path = require('path');
const { chromium } = require('playwright');

const BASE = 'http://127.0.0.1:8787/index.html';
const OUT = __dirname;
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';

async function openPage(browser, w, h) {
  const ctx = await browser.newContext({ viewport: { width: w, height: h }, deviceScaleFactor: 1 });
  const page = await ctx.newPage();
  const errors = [];
  page.on('pageerror', e => errors.push('pageerror: ' + e.message));
  page.on('console', m => { if (m.type() === 'error') errors.push('console.error: ' + m.text()); });
  await page.goto(BASE, { waitUntil: 'domcontentloaded' });
  await page.addStyleTag({ content: '#yy-identity-gate,#yy-intro-overlay,video,.yy-intro,.yy-intro-video{display:none!important}' });
  await page.waitForTimeout(1000);
  // 验收环境没有登录态，移除身份门禁遮罩（只影响这次截图）
  await page.evaluate(() => {
    ['yy-identity-gate', 'yy-boot-mask'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.remove();
    });
    document.querySelectorAll('.modal-overlay, .bs-overlay').forEach(el => el.remove());
  });
  await page.evaluate(() => {
    if (window.yyOpenModule) return window.yyOpenModule('seedmap2');
    if (window.yySeedMapPage) return window.yySeedMapPage();
  });
  await page.waitForTimeout(2600);
  return { page, ctx, errors };
}

async function inspect(page) {
  return page.evaluate(() => {
    const grab = sel => {
      const el = document.querySelector(sel);
      if (!el) return null;
      const r = el.getBoundingClientRect();
      return { w: Math.round(r.width), h: Math.round(r.height), x: Math.round(r.left), y: Math.round(r.top) };
    };
    const root = document.querySelector('.sm2');
    return {
      hasPage: !!root,
      tiles: document.querySelectorAll('#sm2-plane img').length,
      icons: document.querySelectorAll('#sm2-icons .sm2-icon').length,
      listed: document.querySelectorAll('#sm2-list .sm2-row').length,
      chips: document.querySelectorAll('.sm2-chip').length,
      versions: document.querySelectorAll('#sm2-version option').length,
      dims: document.querySelectorAll('#sm2-dims button').length,
      empty: (() => { const e = document.querySelector('#sm2-empty'); return e && !e.hidden ? e.textContent.trim().slice(0, 60) : ''; })(),
      readout: (document.querySelector('#sm2-readout') || {}).textContent,
      overflowX: root ? (root.scrollWidth - root.clientWidth) : -1,
      docOverflow: document.documentElement.scrollWidth - window.innerWidth,
      map: grab('#sm2-map'),
      bar: grab('.sm2-bar'),
      side: grab('.sm2-side'),
      root: grab('.sm2')
    };
  });
}

function report(tag, info, errors) {
  const flags = [];
  if (!info.hasPage) flags.push('页面没打开');
  if (info.overflowX > 1) flags.push('横向溢出 ' + info.overflowX + 'px');
  if (info.docOverflow > 1) flags.push('文档横向溢出 ' + info.docOverflow + 'px');
  if (!info.tiles) flags.push('没有瓦片');
  if (!info.chips) flags.push('没有结构 chip');
  console.log('[' + tag + '] ' + JSON.stringify(info));
  console.log('[' + tag + '] 问题: ' + (flags.length ? flags.join(' / ') : '无'));
  if (errors.length) console.log('[' + tag + '] JS 报错:\n  ' + errors.join('\n  '));
  else console.log('[' + tag + '] JS 报错: 无');
}

(async () => {
  const browser = await chromium.launch({ executablePath: CHROME, headless: true });

  // 桌面 1440
  {
    const { page, ctx, errors } = await openPage(browser, 1440, 900);
    await page.screenshot({ path: path.join(OUT, 'desktop-1440.png') });
    const info = await inspect(page);
    report('桌面 1440', info, errors);

    // 交互：点击地图中心，检查信息面板
    const box = await page.locator('#sm2-map').boundingBox();
    await page.mouse.click(box.x + box.width * 0.55, box.y + box.height * 0.45);
    await page.waitForTimeout(700);
    const infoText = await page.evaluate(() => {
      const el = document.querySelector('#sm2-info');
      return { hidden: el.hidden, text: el.textContent.replace(/\s+/g, ' ').trim().slice(0, 160) };
    });
    console.log('[桌面 1440] 点击面板: ' + JSON.stringify(infoText));
    await page.screenshot({ path: path.join(OUT, 'desktop-1440-click.png') });

    // 交互：勾选史莱姆区块
    await page.evaluate(() => { const c = document.querySelector('#sm2-slime'); c.checked = true; c.dispatchEvent(new Event('change')); });
    await page.waitForTimeout(1500);
    const slimes = await page.evaluate(() => document.querySelectorAll('.sm2-slime').length);
    console.log('[桌面 1440] 史莱姆格数: ' + slimes);
    await page.screenshot({ path: path.join(OUT, 'desktop-1440-slime.png') });
    await ctx.close();
  }

  // 平板 800
  {
    const { page, ctx, errors } = await openPage(browser, 800, 1000);
    await page.screenshot({ path: path.join(OUT, 'tablet-800.png') });
    report('平板 800', await inspect(page), errors);
    await page.click('#sm2-drawer');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(OUT, 'tablet-800-drawer.png') });
    await ctx.close();
  }

  // 手机 390
  {
    const { page, ctx, errors } = await openPage(browser, 390, 844);
    await page.screenshot({ path: path.join(OUT, 'phone-390.png') });
    report('手机 390', await inspect(page), errors);
    await page.click('#sm2-drawer');
    await page.waitForTimeout(600);
    await page.screenshot({ path: path.join(OUT, 'phone-390-drawer.png') });
    const side = await page.evaluate(() => {
      const el = document.querySelector('.sm2-side');
      const r = el.getBoundingClientRect();
      return { top: Math.round(r.top), h: Math.round(r.height), open: document.querySelector('.sm2').classList.contains('drawer') };
    });
    console.log('[手机 390] 抽屉: ' + JSON.stringify(side));
    await ctx.close();
  }

  // 地形引擎 503
  {
    await fetch('http://127.0.0.1:8787/__tilefail?on=1');
    const { page, ctx, errors } = await openPage(browser, 1440, 900);
    await page.screenshot({ path: path.join(OUT, 'desktop-1440-engine-down.png') });
    const info = await inspect(page);
    report('引擎未就绪', info, errors);
    await fetch('http://127.0.0.1:8787/__tilefail?on=0');
    await ctx.close();
  }

  await browser.close();
})().catch(e => { console.error(e); process.exit(1); });
