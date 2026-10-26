const { chromium } = require('playwright-core');
const fs = require('fs');
const path = require('path');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-auth-site';
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';

(async () => {
  const browser = await chromium.launch({ executablePath: CHROME, headless: true });
  const log = [];
  for (const vp of [{ w: 1440, h: 900, tag: 'desk' }, { w: 390, h: 844, tag: 'mob' }]) {
    const ctx = await browser.newContext({ viewport: { width: vp.w, height: vp.h } });
    const page = await ctx.newPage();
    try {
      await page.goto('https://toolbox.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded', timeout: 40000 });
      await page.waitForTimeout(3500);
      await page.screenshot({ path: path.join(OUT, 'toolbox-' + vp.tag + '.png') });
      const info = await page.evaluate(() => {
        const cs = getComputedStyle(document.body);
        const btn = document.querySelector('button');
        return {
          title: document.title,
          text: document.body.innerText.slice(0, 900),
          bg: cs.backgroundColor, font: cs.fontFamily,
          btnBg: btn ? getComputedStyle(btn).backgroundColor : null,
          btnRadius: btn ? getComputedStyle(btn).borderRadius : null,
          primary: (() => { const b = document.querySelector('.primary,button[type=submit]'); return b ? { bg: getComputedStyle(b).backgroundColor, r: getComputedStyle(b).borderRadius, h: b.getBoundingClientRect().height } : null; })(),
          nav: [...document.querySelectorAll('nav a,nav button,.tab')].filter(e => e.offsetHeight > 0).map(e => e.innerText.trim()).slice(0, 15),
          cssVars: (() => { const s = document.documentElement; const out = {}; ['--green', '--ink', '--bg', '--surface', '--radius'].forEach(k => out[k] = s.style.getPropertyValue(k) || getComputedStyle(s).getPropertyValue(k)); return out; })(),
        };
      });
      log.push('\n=== TOOLBOX ' + vp.tag + ' ===\n' + JSON.stringify(info, null, 1));
    } catch (e) { log.push('TOOLBOX ' + vp.tag + ' ERR ' + e.message.slice(0, 200)); }
    await ctx.close();
  }
  fs.writeFileSync(path.join(OUT, 'probe-3.log'), log.join('\n'), 'utf8');
  await browser.close();
  console.log('DONE');
})().catch(e => { console.error('FATAL', e); process.exit(1); });
