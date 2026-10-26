// review-ui 探查 1：登录、首页结构、导航入口清单
const { chromium } = require('playwright-core');

const BASE = 'https://toolbox.youyuanqi.dpdns.org/';
const EXE = 'C:/Program Files/Google/Chrome/Application/chrome.exe';

(async () => {
  const browser = await chromium.launch({ executablePath: EXE, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();

  const reqs = [];
  page.on('request', r => reqs.push({ url: r.url(), method: r.method(), t: Date.now() }));
  page.on('response', r => reqs.push({ url: r.url(), status: r.status(), t: Date.now() }));

  const t0 = Date.now();
  await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 120000 });
  // 登录
  await page.evaluate(() => sessionStorage.setItem('yy-user-control', '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07'));
  const tLogin = Date.now();
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.waitForTimeout(6000);

  const info = await page.evaluate(() => {
    const links = [...document.querySelectorAll('a[href]')].map(a => ({ href: a.getAttribute('href'), text: (a.textContent || '').trim().slice(0, 30) }));
    const buttons = [...document.querySelectorAll('button')].map(b => ({ text: (b.textContent || '').trim().slice(0, 40), cls: (b.className || '').toString().slice(0, 60) }));
    const navs = [...document.querySelectorAll('nav a, nav button, [class*=nav] a, [class*=nav] button')].map(n => ({ tag: n.tagName, text: (n.textContent || '').trim().slice(0, 30), href: n.getAttribute('href') || '' }));
    return {
      title: document.title,
      url: location.href,
      hash: location.hash,
      links: links.slice(0, 60),
      buttons: buttons.slice(0, 80),
      navs: navs.slice(0, 60),
      bodyTextLen: document.body.innerText.length,
      bodyTextHead: document.body.innerText.slice(0, 1500),
      theme: document.documentElement.getAttribute('data-theme') || document.documentElement.className || getComputedStyle(document.body).backgroundColor,
      hasToast: !!document.querySelector('[class*=toast], [class*=Toast]'),
    };
  });

  await page.screenshot({ path: 'p1-home-desktop.png', fullPage: false });
  const summary = {
    firstLoadMs: tLogin - t0,
    reloadMs: Date.now() - tLogin,
    apiCalls: reqs.filter(r => r.url.includes('/api/')).map(r => ({ u: r.url.replace(BASE, ''), s: r.status })),
    info,
  };
  console.log(JSON.stringify(summary, null, 1).slice(0, 8000));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
