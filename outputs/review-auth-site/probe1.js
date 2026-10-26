const { chromium } = require('playwright-core');
const fs = require('fs');
const path = require('path');

const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-auth-site';
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';

(async () => {
  const browser = await chromium.launch({ executablePath: CHROME, headless: true });
  const log = [];
  const shot = async (page, name) => {
    await page.screenshot({ path: path.join(OUT, name + '.png'), fullPage: false });
    log.push('SHOT ' + name);
  };

  for (const vp of [{ w: 1440, h: 900, tag: 'desk' }, { w: 390, h: 844, tag: 'mob' }]) {
    const ctx = await browser.newContext({ viewport: { width: vp.w, height: vp.h }, deviceScaleFactor: 1 });
    const page = await ctx.newPage();
    const errs = [];
    page.on('console', m => { if (m.type() === 'error' || m.type() === 'warning') errs.push('[' + m.type() + '] ' + m.text().slice(0, 200)); });
    page.on('pageerror', e => errs.push('[pageerror] ' + e.message.slice(0, 200)));
    const reqs = [];
    page.on('request', r => { if (r.resourceType() === 'xhr' || r.resourceType() === 'fetch') reqs.push(r.method() + ' ' + r.url().replace('https://auth.youyuanqi.dpdns.org', '')); });

    // ---- 未登录首屏 ----
    const t0 = Date.now();
    await page.goto('https://auth.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 45000 });
    const tDom = Date.now() - t0;
    await page.waitForTimeout(2500);
    await shot(page, vp.tag + '-01-entry');

    const entryInfo = await page.evaluate(() => {
      const vis = el => { if (!el) return false; const r = el.getBoundingClientRect(); const s = getComputedStyle(el); return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && s.display !== 'none'; };
      const q = s => [...document.querySelectorAll(s)];
      const txt = s => (document.querySelector(s)?.innerText || '').trim().slice(0, 300);
      return {
        title: document.title,
        bodyText: document.body.innerText.slice(0, 1200),
        buttons: q('button').filter(vis).map(b => ({ t: b.innerText.trim().slice(0, 40), disabled: b.disabled, cls: b.className })),
        inputs: q('input,select,textarea').filter(vis).map(i => ({ tag: i.tagName, type: i.type, id: i.id, ph: i.placeholder, disabled: i.disabled })),
        disabledOpts: q('option[disabled]').map(o => o.textContent),
        links: q('a').filter(vis).map(a => a.innerText.trim() + ' -> ' + a.href).slice(0, 20),
        skeleton: q('.yy-skel').filter(vis).length,
        fonts: getComputedStyle(document.body).fontFamily,
        bg: getComputedStyle(document.body).backgroundColor,
        primaryBtn: (() => { const b = document.querySelector('button.primary,button[type=submit]'); return b ? getComputedStyle(b).backgroundColor : null; })(),
      };
    });
    log.push('\n=== ' + vp.tag + ' ENTRY (' + tDom + 'ms domcontentloaded) ===');
    log.push(JSON.stringify(entryInfo, null, 1));

    // ---- 切换到注册 tab（如果有）----
    const tabTexts = await page.evaluate(() => [...document.querySelectorAll('button,a,[role=tab]')].filter(b => b.getBoundingClientRect().height > 0).map(b => b.innerText.trim()).filter(Boolean));
    log.push('CLICKABLE: ' + JSON.stringify(tabTexts));

    // 尝试注册/登录 tab
    for (const name of ['注册', '创建账号', '立即注册', '登录']) {
      const loc = page.getByRole('button', { name, exact: false }).first();
      if (await loc.count()) {
        try {
          await loc.click({ timeout: 2000 });
          await page.waitForTimeout(800);
          log.push('CLICKED TAB: ' + name);
          break;
        } catch (e) { log.push('CLICK FAIL ' + name + ': ' + e.message.slice(0, 80)); }
      }
    }
    await shot(page, vp.tag + '-02-register-tab');
    const regInfo = await page.evaluate(() => {
      const vis = el => { if (!el) return false; const r = el.getBoundingClientRect(); return r.width > 0 && r.height > 0; };
      return {
        text: document.body.innerText.slice(0, 1500),
        inputs: [...document.querySelectorAll('input,select,textarea')].filter(vis).map(i => ({ type: i.type, id: i.id, ph: i.placeholder, required: i.required, pattern: i.pattern, maxlength: i.maxlength })),
        buttons: [...document.querySelectorAll('button')].filter(vis).map(b => ({ t: b.innerText.trim().slice(0, 30), disabled: b.disabled })),
      };
    });
    log.push('REG PANEL: ' + JSON.stringify(regInfo, null, 1));

    // ---- 表单校验：空提交 / 非法邮箱 ----
    const submit = page.locator('button[type=submit], button').filter({ hasText: /注册|登录|创建|继续|发送/ }).first();
    if (await submit.count()) {
      await submit.click({ timeout: 3000 }).catch(e => log.push('SUBMIT CLICK ERR ' + e.message.slice(0, 60)));
      await page.waitForTimeout(1200);
      await shot(page, vp.tag + '-03-empty-submit');
      const afterEmpty = await page.evaluate(() => ({
        text: document.body.innerText.slice(0, 800),
        invalid: [...document.querySelectorAll(':invalid')].map(i => i.id || i.name || i.type),
        msg: document.querySelector('.yy-error,.error,[role=alert],#result')?.innerText?.slice(0, 200) || null,
      }));
      log.push('EMPTY SUBMIT RESULT: ' + JSON.stringify(afterEmpty, null, 1));
    }

    // ---- CLS 观测：刷新整页再测量 ----
    await page.goto('https://auth.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded' });
    const cls = await page.evaluate(() => new Promise(res => {
      let total = 0; const shifts = [];
      new PerformanceObserver(list => {
        for (const e of list.getEntries()) { if (!e.hadRecentInput) { total += e.value; shifts.push({ v: +e.value.toFixed(4), t: Math.round(e.startTime) }); } }
      }).observe({ type: 'layout-shift', buffered: true });
      setTimeout(() => res({ cls: +total.toFixed(4), shifts: shifts.slice(0, 10), nav: performance.getEntriesByType('navigation')[0]?.domContentLoadedEventEnd | 0 }), 4000);
    }));
    log.push('CLS ' + vp.tag + ': ' + JSON.stringify(cls));

    log.push('XHR ' + vp.tag + ': ' + JSON.stringify([...new Set(reqs)]));
    log.push('CONSOLE ' + vp.tag + ': ' + JSON.stringify(errs.slice(0, 20)));

    await ctx.close();
  }

  fs.writeFileSync(path.join(OUT, 'probe-1.log'), log.join('\n'), 'utf8');
  await browser.close();
  console.log('DONE');
})().catch(e => { console.error('FATAL', e); process.exit(1); });
