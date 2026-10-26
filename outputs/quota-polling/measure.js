// 请求计数探针：本地静态服务 + /api 打桩，量 60 秒内的 /api/* 条数。
// 用法: node measure.js <frontendRoot> <label>
const http = require('http');
const fs = require('fs');
const path = require('path');

const ROOT = process.argv[2];
const LABEL = process.argv[3] || 'run';
const PORT = 8123;

const MIME = { '.html': 'text/html; charset=utf-8', '.js': 'application/javascript; charset=utf-8', '.css': 'text/css', '.json': 'application/json', '.png': 'image/png', '.svg': 'image/svg+xml' };

const server = http.createServer((req, res) => {
  let p = decodeURIComponent(req.url.split('?')[0]);
  if (p === '/') p = '/index.html';
  const file = path.join(ROOT, p);
  if (!file.startsWith(path.resolve(ROOT))) { res.writeHead(403); return res.end(); }
  fs.readFile(file, (err, buf) => {
    if (err) { res.writeHead(404); return res.end('nf'); }
    res.writeHead(200, { 'Content-Type': MIME[path.extname(file)] || 'application/octet-stream' });
    res.end(buf);
  });
});

function mockFor(url) {
  let u;
  try { u = new URL(url).pathname; } catch (e) { u = url.split('?')[0]; }
  if (u === '/health') return { ok: true, version: 'test', platform: 'test' };
  if (u === '/api/youyuan/snapshot') return { ok: true, snapshot: true, game: { ok: true, connected: false }, remote: { ok: true, state: '未连接', config: {} }, runtime: { ok: true, instances: [] }, tasks: { ok: true, tasks: [] }, plugins: { ok: true, plugins: [] }, qq: { ok: true, state: '未配置' }, diag: { ok: true, drops_today: 0 } };
  if (u === '/api/youyuan/events') return { ok: true, cursor: 0, events: [] };
  if (u === '/api/youyuan/accounts') return { ok: true, accounts: [], main: { configured: false } };
  if (u === '/api/version/check') return { ok: true };
  if (u === '/api/config') return { ok: true, config: {} };
  if (u === '/api/youyuan/remote') return { ok: true, state: '未连接', config: {} };
  if (u === '/api/task/list') return { ok: true, tasks: [] };
  if (u === '/api/plugin/list') return { ok: true, plugins: [] };
  if (u === '/api/qq/status') return { ok: true, state: '未配置' };
  if (u === '/api/youyuan/runtime/list') return { ok: true, instances: [] };
  if (u === '/api/youyuan/connection-diagnostics') return { ok: true, drops_today: 0 };
  if (u === '/api/bot/status') return { ok: true, connected: false, is_op: false };
  return { ok: true };
}

(async () => {
  await new Promise(r => server.listen(PORT, r));
  const { chromium } = require('playwright-core');
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: true,
    args: ['--no-sandbox']
  });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  const hits = [];
  const t0 = Date.now();
  await page.route('**/api/**', async route => {
    const url = route.request().url();
    hits.push({ t: Date.now() - t0, url });
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockFor(url)) });
  });
  await page.route('**/health', async route => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockFor('/health')) });
  });
  page.on('pageerror', e => console.error('[pageerror]', e.message));

  await page.goto('http://127.0.0.1:' + PORT + '/index.html', { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(3000);
  const bootCount = hits.length;

  // ---- 阶段 1：前台可见，停在概览页 60 秒 ----
  const p1start = hits.length;
  await page.waitForTimeout(60000);
  const visible = hits.slice(p1start);

  // ---- 阶段 2：模拟标签页不可见 30 秒 ----
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => true });
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' });
    document.dispatchEvent(new Event('visibilitychange'));
  });
  const p2start = hits.length;
  await page.waitForTimeout(30000);
  const hidden = hits.slice(p2start);

  // ---- 阶段 3：切到"任务"页（SPA 路由）30 秒 ----
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => false });
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' });
    document.dispatchEvent(new Event('visibilitychange'));
  });
  await page.waitForTimeout(1000);
  await page.evaluate(() => { if (typeof tabTo === 'function') tabTo('tasks'); });
  await page.waitForTimeout(2000);
  const p3start = hits.length;
  await page.waitForTimeout(30000);
  const otherTab = hits.slice(p3start);

  const group = arr => {
    const m = {};
    arr.forEach(h => { const k = h.url.split('?')[0].replace('http://127.0.0.1:' + PORT, ''); m[k] = (m[k] || 0) + 1; });
    return m;
  };
  const out = {
    label: LABEL,
    boot: bootCount,
    visible60s: visible.length,
    visible60s_byEndpoint: group(visible),
    hidden30s: hidden.length,
    hidden30s_byEndpoint: group(hidden),
    otherTab30s: otherTab.length,
    otherTab30s_byEndpoint: group(otherTab),
    total: hits.length
  };
  console.log(JSON.stringify(out, null, 2));
  await browser.close();
  server.close();
})();
