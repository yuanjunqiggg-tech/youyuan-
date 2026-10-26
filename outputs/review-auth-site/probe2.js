const { chromium } = require('playwright-core');
const fs = require('fs');
const path = require('path');
const OUT = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/review-auth-site';
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';

const ACCOUNTS = {
  ok: true,
  accounts: [
    { id: 'a1', name: '测试账号A', nickname: '小白', game_id: '114514001', channel: 'netease', status: 'ready', keep_alive: true, created: Date.now() - 86400000 * 9, last_refresh: Date.now() - 600000, avatar_source: 'netease', avatar: 'https://x19.gph.netease.com/head/114514001.png', skin_item_id: '' },
    { id: 'a2', name: '测试账号B', nickname: '老王', game_id: '114514002', channel: '4399', status: 'error', last_error: '登录凭据已过期', keep_alive: false, created: Date.now() - 86400000 * 3, last_refresh: 0, skin_item_id: '1029304' },
  ],
  pending: [], batches: [],
};

(async () => {
  const browser = await chromium.launch({ executablePath: CHROME, headless: true });
  const log = [];
  const shot = async (p, n) => { await p.screenshot({ path: path.join(OUT, n + '.png') }); log.push('SHOT ' + n); };
  const jclick = async (p, sel, label) => { try { await p.locator(sel).first().click({ timeout: 2500 }); log.push('CLICK OK ' + label); } catch (e) { try { await p.evaluate(s => document.querySelector(s)?.click(), sel); log.push('CLICK JS ' + label); } catch (e2) { log.push('CLICK FAIL ' + label + ' ' + e.message.slice(0, 60)); } } };

  for (const vp of [{ w: 1440, h: 900, tag: 'desk' }, { w: 390, h: 844, tag: 'mob' }]) {
    const ctx = await browser.newContext({ viewport: { width: vp.w, height: vp.h } });
    await ctx.addInitScript(() => {
      sessionStorage.setItem('yy-auth-device', 'dev-test');
      sessionStorage.setItem('yy-auth-session', 'sess-test');
    });
    const page = await ctx.newPage();
    const errs = [];
    page.on('pageerror', e => errs.push('[pageerror] ' + e.message.slice(0, 160)));
    page.on('console', m => { if (m.type() === 'error' && !/font-size:0|data:,|challenges.cloudflare/.test(m.text())) errs.push(m.text().slice(0, 160)); });

    await page.route('**/api/accounts*', async route => {
      const req = route.request();
      if (req.method() === 'GET') return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ACCOUNTS) });
      const body = JSON.parse(req.postData() || '{}');
      log.push('POST /api/accounts ' + JSON.stringify(body));
      if (body.action === 'skin_preview') return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: false, error: 'mock: 无皮肤纹理' }) });
      if (body.action === 'token') return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true, login_token: 'yy//MOCKTOKEN123456', auth_url: 'https://auth.youyuanqi.dpdns.org' }) });
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) });
    });
    await page.route('**/api/identity/**', r => r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true, user: { username: 'reviewer' } }) }));
    await page.route('**/api/turnstile-config', r => r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) }));

    try {
    await page.goto('https://auth.youyuanqi.dpdns.org/', { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(2500);
    await shot(page, vp.tag + '-10-console');

    const overview = await page.evaluate(() => ({
      nav: [...document.querySelectorAll('.auth-nav button')].filter(b => !b.hidden).map(b => b.innerText.trim() + (b.getAttribute('aria-selected') === 'true' ? ' *' : '')),
      cards: [...document.querySelectorAll('.auth-account-card')].map(c => c.innerText.replace(/\s+/g, ' ').trim().slice(0, 120)),
      metrics: document.querySelector('#auth-overview')?.innerText.replace(/\n/g, ' '),
      notices: [...document.querySelectorAll('.auth-notice,.auth-hint')].map(n => n.innerText.trim().slice(0, 160)),
      buttons: [...document.querySelectorAll('#auth-page-accounts button')].filter(b => b.offsetHeight > 0).map(b => b.innerText.trim()),
      avatarImgs: [...document.querySelectorAll('.auth-avatar img')].map(i => i.src.slice(0, 90) + ' | alt=' + i.alt + ' | ' + i.naturalWidth + 'x' + i.naturalHeight),
      placeholders: document.querySelectorAll('.auth-avatar svg').length,
      pageH: document.body.scrollHeight, winH: innerHeight,
    }));
    log.push('\n=== ' + vp.tag + ' CONSOLE ===\n' + JSON.stringify(overview, null, 1));

    // 展开第一张卡
    await jclick(page, '.auth-detail-button', 'expand-card');
    await page.waitForTimeout(1200); await shot(page, vp.tag + '-11-expanded');
    const detail = await page.evaluate(() => {
      const d = document.querySelector('.auth-account-details:not([hidden])');
      if (!d) return null;
      return {
        text: d.innerText.replace(/\n{2,}/g, '\n').slice(0, 1600),
        buttons: [...d.querySelectorAll('button')].map(b => b.innerText.trim() + (b.disabled ? ' [disabled]' : '')),
        inputs: [...d.querySelectorAll('input,textarea')].filter(i => i.offsetHeight > 0).map(i => i.type + ':' + (i.id || i.getAttribute('aria-label') || i.placeholder)),
        sections: [...d.querySelectorAll('.auth-section-head')].map(s => s.innerText.trim() + ' expanded=' + s.getAttribute('aria-expanded')),
        emptyImg: d.querySelectorAll('.auth-avatar-empty').length,
        stage: !!d.querySelector('.auth-detail-skin canvas'),
      };
    });
    log.push('DETAIL: ' + JSON.stringify(detail, null, 1));

    // 展开资料区与皮肤区
    for (const t of ['头像与账号资料', '游戏皮肤与组件码', '令牌与凭据维护']) {
      await page.evaluate(t => { const h = [...document.querySelectorAll('.auth-section-head')].find(x => x.innerText.includes(t)); if (h && h.getAttribute('aria-expanded') !== 'true') h.click(); }, t);
      await page.waitForTimeout(400);
    }
    await page.waitForTimeout(600);
    await shot(page, vp.tag + '-12-sections-open');
    const opened = await page.evaluate(() => {
      const d = document.querySelector('.auth-account-details:not([hidden])');
      return d ? { text: d.innerText.replace(/\n{2,}/g, '\n').slice(0, 2500), btns: [...d.querySelectorAll('button')].map(b => b.innerText.trim()) } : null;
    });
    log.push('OPENED SECTIONS: ' + JSON.stringify(opened, null, 1));

    // 滚动到底看皮肤预览区
    await page.evaluate(() => { const d = document.querySelector('.auth-account-details:not([hidden])'); d?.scrollIntoView({ block: 'start' }); });
    await page.waitForTimeout(400);
    await shot(page, vp.tag + '-13-detail-scrolled');

    // 切到 添加与注册 页
    await jclick(page, '#auth-nav-add', 'nav-add');
    await page.waitForTimeout(600);
    await shot(page, vp.tag + '-14-add');
    const addInfo = await page.evaluate(() => {
      const p = document.querySelector('#auth-page-add');
      return { text: p.innerText.replace(/\n{2,}/g, '\n').slice(0, 1400), disabledOpts: [...p.querySelectorAll('option[disabled]')].map(o => o.textContent), selects: [...p.querySelectorAll('select')].map(s => s.id + '=' + s.value), detailsOpen: [...p.querySelectorAll('details')].map(d => d.open) };
    });
    log.push('ADD PAGE: ' + JSON.stringify(addInfo, null, 1));

    // 服务中心
    await jclick(page, '#auth-nav-services', 'nav-services');
    await page.waitForTimeout(500);
    await shot(page, vp.tag + '-15-services');
    log.push('SERVICES: ' + JSON.stringify(await page.evaluate(() => document.querySelector('#auth-page-services').innerText.replace(/\n{2,}/g, '\n').slice(0, 900))));

    // 游戏令牌
    await jclick(page, '#auth-nav-tokens', 'nav-tokens');
    await page.waitForTimeout(500);
    await shot(page, vp.tag + '-16-tokens');
    log.push('TOKENS: ' + JSON.stringify(await page.evaluate(() => document.querySelector('#auth-page-tokens').innerText.replace(/\n{2,}/g, '\n').slice(0, 700))));

    // 回到账号，点第一个头像 → 应该展开详情而不是别的
    await jclick(page, '#auth-nav-accounts', 'nav-accounts');
    await page.waitForTimeout(400);
    await jclick(page, '.auth-avatar', 'avatar-click');
    await page.waitForTimeout(800);
    log.push('AVATAR CLICK -> dialog open? ' + await page.evaluate(() => !!document.querySelector('dialog[open]')) + ' ; expanded=' + await page.evaluate(() => document.querySelectorAll('.auth-account-card.expanded').length));

    // 退出
    await page.evaluate(() => [...document.querySelectorAll('button')].find(b => b.innerText.includes('退出平台账号'))?.click());
    await page.waitForTimeout(800);
    await shot(page, vp.tag + '-17-after-logout');
    log.push('AFTER LOGOUT: login visible=' + await page.evaluate(() => !document.querySelector('#login')?.hidden) + ' workspace visible=' + await page.evaluate(() => !document.querySelector('#workspace')?.hidden));

    log.push('ERRS ' + vp.tag + ': ' + JSON.stringify(errs.slice(0, 15)));
    } catch (e) { log.push('VIEWPORT ' + vp.tag + ' ABORTED: ' + e.message.slice(0, 300)); }
    await ctx.close();
  }
  fs.writeFileSync(path.join(OUT, 'probe-2.log'), log.join('\n'), 'utf8');
  await browser.close();
  console.log('DONE');
})().catch(e => { console.error('FATAL', e); process.exit(1); });
