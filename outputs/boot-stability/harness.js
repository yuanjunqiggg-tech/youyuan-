// 本地首屏稳定性夹具：静态服务 upstream/frontend + 按真实延迟模拟 /api/*
// 用途：在不碰线上、不部署的前提下，可重复地测量首页渲染的 CLS / 高度突变 / 请求数。
// 用法: node harness.js <tag> [root]
const http = require('http');
const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright-core');

const TAG = process.argv[2] || 'local';
const ROOT = process.argv[3] || 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend';
const OUT = __dirname;

const MIME = {
  '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8', '.json': 'application/json; charset=utf-8',
  '.jpg': 'image/jpeg', '.png': 'image/png', '.svg': 'image/svg+xml', '.woff2': 'font/woff2',
};

// ── 模拟内核：字段与 upstream/youyuan_snapshot.go 一致 ──────────────────
const MOCK = {
  game: { ok: true, connected: true, is_op: true, server: '我的世界 · 生存服 #4821', server_target: '4821', game_rtt_ms: 48 },
  remote: { ok: true, state: '已连接', config: { enabled: true, url: 'wss://toolbox.youyuanqi.dpdns.org/device/connect' } },
  runtime: { ok: true, instances: [{ id: 'python_counter', running: true }] },
  tasks: { ok: true, tasks: [
    { task_id: 't1', type: 'import', name: '导入 城堡主塔.mcstructure', status: 'running', progress: 0.42, message: '进度 42%', params: '{}' },
    { task_id: 't2', type: 'export', name: '导出 出生点区域', status: 'done', progress: 1, message: '完成', params: '{}' },
    { task_id: 't3', type: 'mapart', name: '地图画 · 像素猫', status: 'failed', progress: 0.8, message: '失败', params: '{}' },
  ] },
  plugins: { ok: true, plugins: [{ name: 'auto_shop', running: true, disabled: false }] },
  qq: { ok: true, state: '已连接', detail: 'OneBot v11' },
  diag: { ok: true, drops_today: 2, last_drop_label: '08:41 心跳超时' },
};

const DELAY = {
  '/api/youyuan/snapshot': 320,
  '/api/bot/status': 150,
  '/api/task/list': 150,
  '/api/config': 220,
  '/api/youyuan/appearance': 180,
  '/api/identity/me': 260,
  '/api/youyuan/identity': 260,
  '/api/youyuan/accounts': 200,
  '/api/youyuan/events': 130,
};

function body(p) {
  if (p.startsWith('/api/youyuan/snapshot')) return Object.assign({ ok: true, snapshot: true }, MOCK);
  if (p.startsWith('/api/bot/status')) return MOCK.game;
  if (p.startsWith('/api/task/list')) return MOCK.tasks;
  if (p.startsWith('/api/youyuan/remote')) return MOCK.remote;
  if (p.startsWith('/api/youyuan/runtime/list')) return MOCK.runtime;
  if (p.startsWith('/api/plugin/list')) return MOCK.plugins;
  if (p.startsWith('/api/qq/status')) return MOCK.qq;
  if (p.startsWith('/api/youyuan/connection-diagnostics')) return MOCK.diag;
  if (p.startsWith('/api/identity/turnstile-config')) return { ok: true, enabled: false };
  if (p.startsWith('/api/youyuan/identity')) return { ok: true, authenticated: true, control_token: 'demo-token', user: { id: 1, username: 'demo', role: 'admin', email: 'demo@qq.com', email_verified: true, subscription_until: Date.now() + 86400000 * 30 } };
  if (p.startsWith('/api/identity/me')) return { ok: true, user: { id: 1, username: 'demo', role: 'admin', email: 'demo@qq.com', email_verified: true, subscription_until: Date.now() + 86400000 * 30 } };
  if (p.startsWith('/api/config')) return { ok: true, config: { import_speed: -1, exclude_water: true, exclude_waterlogged: true, exclude_lava: true, region_mode: '1', bot_tokens: [] } };
  if (p.startsWith('/api/youyuan/appearance')) return { ok: true, theme: 'light' };
  if (p.startsWith('/api/youyuan/accounts')) return { ok: true, accounts: [] };
  if (p.startsWith('/api/youyuan/events')) return { ok: true, cursor: 0, events: [] };
  if (p.startsWith('/api/version/check')) return { ok: true, latest: '0.0.0' };
  return { ok: true };
}

const server = http.createServer((req, res) => {
  let p = decodeURIComponent(req.url.split('?')[0]);
  if (p === '/') p = '/index.html';
  const f = path.join(ROOT, p);
  if (!f.startsWith(path.resolve(ROOT)) || !fs.existsSync(f) || fs.statSync(f).isDirectory()) {
    res.writeHead(404, { 'Content-Type': 'text/plain' }); res.end('not found'); return;
  }
  res.writeHead(200, { 'Content-Type': MIME[path.extname(f)] || 'application/octet-stream', 'Cache-Control': 'no-store' });
  fs.createReadStream(f).pipe(res);
});

(async () => {
  await new Promise((r) => server.listen(0, '127.0.0.1', r));
  const port = server.address().port;
  const PAGE_URL = `http://127.0.0.1:${port}/`;

  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: true, args: ['--no-sandbox'],
  });
  const ctx = await browser.newContext({
    viewport: { width: 412, height: 915 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true,
  });
  const page = await ctx.newPage();

  // 网络：/api/* 按延迟返回模拟数据，静态资源直接放行到本地服务
  await page.route('**/api/**', async (route) => {
    const u = new URL(route.request().url());
    let ms = 150;
    for (const k of Object.keys(DELAY)) if (u.pathname.startsWith(k)) ms = DELAY[k];
    await new Promise((r) => setTimeout(r, ms));
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body(u.pathname)) });
  });

  const requests = [];
  const t0 = Date.now();
  page.on('request', (r) => requests.push({ url: r.url(), start: Date.now() - t0 }));
  page.on('response', (r) => {
    const rec = requests.find((x) => x.url === r.url() && !x.end);
    if (rec) { rec.end = Date.now() - t0; rec.status = r.status(); }
  });

  await page.addInitScript(() => {
    window.__cls = 0; window.__clsEntries = []; window.__heights = []; window.__renders = [];
    try {
      new PerformanceObserver((list) => {
        for (const e of list.getEntries()) {
          if (e.hadRecentInput) continue;
          window.__cls += e.value;
          window.__clsEntries.push({
            value: e.value, t: Math.round(e.startTime),
            sources: (e.sources || []).map((s) => ({
              node: s.node ? (s.node.id ? '#' + s.node.id : (typeof s.node.className === 'string' ? s.node.className : s.node.nodeName)) : null,
              prev: s.previousRect ? [Math.round(s.previousRect.y), Math.round(s.previousRect.height)] : null,
              cur: s.currentRect ? [Math.round(s.currentRect.y), Math.round(s.currentRect.height)] : null,
            })),
          });
        }
      }).observe({ type: 'layout-shift', buffered: true });
    } catch (e) {}
    // 记录 #tab-home 的 innerHTML 重写次数（"乱刷新"的量化指标）
    const desc = Object.getOwnPropertyDescriptor(Element.prototype, 'innerHTML');
    Object.defineProperty(Element.prototype, 'innerHTML', {
      set(v) {
        if (this.id === 'tab-home') window.__renders.push({ t: Math.round(performance.now()), len: String(v).length });
        return desc.set.call(this, v);
      },
      get() { return desc.get.call(this); },
    });
    function sample() {
      const home = document.getElementById('tab-home');
      const scroller = document.querySelector('.content.show') || document.scrollingElement;
      const box = (sel) => {
        const el = document.querySelector(sel);
        if (!el) return null;
        const r = el.getBoundingClientRect();
        return [Math.round(r.top), Math.round(r.height)];
      };
      window.__heights.push({
        t: Math.round(performance.now()),
        // 真正的滚动容器是 .content.show（.content 有 overflow-y:auto），
        // 所以 document.scrollHeight 永远等于视口高度，不能用来测跳动。
        cH: scroller ? Math.round(scroller.clientHeight) : -1,
        cSH: scroller ? Math.round(scroller.scrollHeight) : -1,
        cST: scroller ? Math.round(scroller.scrollTop) : -1,
        homeH: home ? Math.round(home.getBoundingClientRect().height) : -1,
        welcome: box('.yy-welcome'),
        stats: box('.yy-stats'),
        tasks: box('#yy-home-tasks'),
        catalog: box('#yy-tool-catalog'),
      });
      requestAnimationFrame(sample);
    }
    requestAnimationFrame(sample);
  });

  const navStart = Date.now();
  await page.goto(PAGE_URL, { waitUntil: 'domcontentloaded', timeout: 60000 });
  const tDom = Date.now() - navStart;
  try { await page.waitForLoadState('networkidle', { timeout: 20000 }); } catch (e) {}
  await page.waitForTimeout(3000);

  const data = await page.evaluate(() => ({
    cls: window.__cls, clsEntries: window.__clsEntries, heights: window.__heights, renders: window.__renders,
    docH: document.documentElement.scrollHeight,
    homeText: (document.getElementById('tab-home') || {}).innerText || '',
    gate: !!document.getElementById('yy-identity-gate'),
  }));

  await page.screenshot({ path: path.join(OUT, `${TAG}-local-viewport.png`) });
  await page.screenshot({ path: path.join(OUT, `${TAG}-local-full.png`), fullPage: true });

  const hs = data.heights, jumps = [];
  for (let i = 1; i < hs.length; i++) {
    const d = hs[i].cSH - hs[i - 1].cSH;
    if (Math.abs(d) >= 30) jumps.push({ t: hs[i].t, from: hs[i - 1].cSH, to: hs[i].cSH, delta: d });
  }
  const merged = [];
  for (const j of jumps) {
    const last = merged[merged.length - 1];
    if (last && j.t - last.t < 150) { last.to = j.to; last.delta = last.to - last.from; last.t = j.t; }
    else merged.push({ ...j });
  }
  // 元素被推挤：已渲染的区块 top 在相邻帧之间移动 ≥8px
  const pushes = [];
  const keys = ['welcome', 'stats', 'tasks', 'catalog'];
  for (let i = 1; i < hs.length; i++) {
    for (const k of keys) {
      const a = hs[i - 1][k], b = hs[i][k];
      if (!a || !b) continue;
      if (Math.abs(b[0] - a[0]) >= 8) pushes.push({ t: hs[i].t, block: k, from: a[0], to: b[0], delta: b[0] - a[0] });
    }
  }
  const pushMerged = [];
  for (const p of pushes) {
    const last = pushMerged[pushMerged.length - 1];
    if (last && last.block === p.block && p.t - last.t < 150) { last.to = p.to; last.delta = last.to - last.from; last.t = p.t; }
    else pushMerged.push({ ...p });
  }

  const api = requests.filter((r) => /\/api\//.test(r.url));
  const counts = {};
  api.forEach((r) => { const u = r.url.replace(/^https?:\/\/[^/]+/, '').split('?')[0]; counts[u] = (counts[u] || 0) + 1; });
  const dupes = Object.entries(counts).filter(([, n]) => n > 1).map(([url, n]) => ({ url, count: n }));

  // ── 第二段：点开「群服互通」，测它自己的请求数与首屏内容耗时 ──
  let qq = null;
  try {
    const q0 = Date.now();
    const idx = requests.length;
    await page.evaluate(() => { if (typeof yyOpenModule === 'function') yyOpenModule('qq'); });
    const tOpen = Date.now();
    let tContent = null;
    for (let i = 0; i < 60; i++) {
      const ready = await page.evaluate(() => {
        const b = document.querySelector('#sub-page .sp-body');
        return !!(b && b.innerText.length > 200);
      });
      if (ready) { tContent = Date.now() - tOpen; break; }
      await page.waitForTimeout(100);
    }
    await page.waitForTimeout(2500);
    const qApi = requests.slice(idx).filter((r) => /\/api\//.test(r.url));
    qq = {
      clickToContentMs: tContent,
      apiCount: qApi.length,
      apis: qApi.map((r) => ({ url: r.url.replace(/^https?:\/\/[^/]+/, ''), start: r.start - (q0 - t0), ms: r.end ? r.end - r.start : null })),
    };
  } catch (e) { qq = { error: String(e) }; }

  const report = {
    tag: TAG, mode: 'local-harness', url: PAGE_URL, domContentLoadedMs: tDom,
    cls: Number(data.cls.toFixed(4)), clsEntries: data.clsEntries,
    scrollHeightJumps: merged, jumpCount: merged.length,
    contentHeightJumps: merged, pushedBlocks: pushMerged, pushCount: pushMerged.length,
    finalContentHeight: hs.length ? hs[hs.length - 1].cSH : -1,
    homeInnerHtmlRewrites: data.renders, homeInnerHtmlRewriteCount: data.renders.length,
    apiRequestCount: api.length,
    apiRequests: api.map((r) => ({ url: r.url.replace(/^https?:\/\/[^/]+/, ''), start: r.start, ms: r.end ? r.end - r.start : null })),
    duplicateRequests: dupes,
    qqPage: qq,
    gateShown: data.gate,
    homeTextSample: data.homeText.slice(0, 300),
  };
  fs.writeFileSync(path.join(OUT, `harness-${TAG}.json`), JSON.stringify(report, null, 2));

  console.log('=== harness ' + TAG + ' ===');
  console.log('DOMContentLoaded:', tDom, 'ms');
  console.log('CLS:', report.cls);
  console.log('#tab-home innerHTML 重写次数:', data.renders.length, JSON.stringify(data.renders));
  console.log('内容高度突变:', merged.length, ' 最终内容高度:', report.finalContentHeight);
  merged.forEach((j) => console.log('  t=' + j.t + 'ms', j.from, '->', j.to, '(' + j.delta + ')'));
  console.log('区块被推挤:', pushMerged.length);
  pushMerged.forEach((p) => console.log('  t=' + p.t + 'ms', p.block, p.from, '->', p.to, '(' + p.delta + ')'));
  console.log('API 请求:', api.length);
  console.log('重复:', JSON.stringify(dupes));
  console.log('互通页：点击到出内容', qq && qq.clickToContentMs, 'ms · 请求', qq && qq.apiCount);
  (qq && qq.apis || []).forEach((a) => console.log('   ', a.ms + 'ms', a.url));
  console.log('CLS 明细:');
  report.clsEntries.sort((a, b) => b.value - a.value).slice(0, 6).forEach((e) =>
    console.log('  ', e.value.toFixed(4), 't=' + e.t, JSON.stringify(e.sources)));

  await browser.close();
  server.close();
})().catch((e) => { console.error('FAILED', e); process.exit(1); });
