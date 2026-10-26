// 首屏稳定性探针：CLS / 高度突变 / XHR 清单
// 用法: node probe.js <tag> [url]
const { chromium } = require('playwright-core');
const fs = require('fs');
const path = require('path');

const TAG = process.argv[2] || 'before';
const URL = process.argv[3] || 'https://toolbox.youyuanqi.dpdns.org/';
const OUT = __dirname;

(async () => {
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });
  const ctx = await browser.newContext({
    viewport: { width: 412, height: 915 },
    deviceScaleFactor: 2,
    isMobile: true,
    hasTouch: true,
    userAgent:
      'Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36',
  });
  const page = await ctx.newPage();

  const requests = [];
  const t0 = Date.now();
  page.on('request', (r) => {
    requests.push({ url: r.url(), method: r.method(), type: r.resourceType(), start: Date.now() - t0 });
  });
  page.on('response', (r) => {
    const rec = requests.find((x) => x.url === r.url() && !x.end);
    if (rec) { rec.end = Date.now() - t0; rec.status = r.status(); }
  });
  page.on('requestfailed', (r) => {
    const rec = requests.find((x) => x.url === r.url() && !x.end);
    if (rec) { rec.end = Date.now() - t0; rec.failed = r.failure() && r.failure().errorText; }
  });

  // 注入探针：layout-shift 观察 + 每帧 scrollHeight
  await page.addInitScript(() => {
    window.__cls = 0;
    window.__clsEntries = [];
    window.__heights = [];
    window.__marks = [];
    try {
      new PerformanceObserver((list) => {
        for (const e of list.getEntries()) {
          if (e.hadRecentInput) continue;
          window.__cls += e.value;
          window.__clsEntries.push({
            value: e.value,
            t: Math.round(e.startTime),
            sources: (e.sources || []).map((s) => ({
              node: s.node ? (s.node.id ? '#' + s.node.id : s.node.className || s.node.nodeName) : null,
              prev: s.previousRect ? [Math.round(s.previousRect.y), Math.round(s.previousRect.height)] : null,
              cur: s.currentRect ? [Math.round(s.currentRect.y), Math.round(s.currentRect.height)] : null,
            })),
          });
        }
      }).observe({ type: 'layout-shift', buffered: true });
    } catch (e) {}

    function sample() {
      const home = document.getElementById('tab-home');
      const inner = document.getElementById('tab-home-inner');
      window.__heights.push({
        t: Math.round(performance.now()),
        sh: document.documentElement.scrollHeight,
        homeH: home ? Math.round(home.getBoundingClientRect().height) : -1,
        innerH: inner ? Math.round(inner.getBoundingClientRect().height) : -1,
        homeChildren: home ? home.children.length : -1,
        textLen: home ? home.innerText.length : 0,
      });
      requestAnimationFrame(sample);
    }
    requestAnimationFrame(sample);
  });

  const navStart = Date.now();
  await page.goto(URL, { waitUntil: 'domcontentloaded', timeout: 60000 });
  const tDom = Date.now() - navStart;
  // 等到网络静默 + 额外 3.5s 覆盖轮询
  try { await page.waitForLoadState('networkidle', { timeout: 25000 }); } catch (e) {}
  await page.waitForTimeout(3500);

  const data = await page.evaluate(() => ({
    cls: window.__cls,
    clsEntries: window.__clsEntries,
    heights: window.__heights,
    docH: document.documentElement.scrollHeight,
    homeText: (document.getElementById('tab-home') || {}).innerText || '',
  }));

  await page.screenshot({ path: path.join(OUT, `${TAG}-viewport.png`), fullPage: false });
  await page.screenshot({ path: path.join(OUT, `${TAG}-full.png`), fullPage: true });

  // 高度突变检测
  const hs = data.heights;
  const jumps = [];
  for (let i = 1; i < hs.length; i++) {
    const d = hs[i].sh - hs[i - 1].sh;
    if (Math.abs(d) >= 30) {
      jumps.push({ t: hs[i].t, from: hs[i - 1].sh, to: hs[i].sh, delta: d });
    }
  }
  // 合并同一时刻连续变化
  const merged = [];
  for (const j of jumps) {
    const last = merged[merged.length - 1];
    if (last && j.t - last.t < 120) { last.to = j.to; last.delta = last.to - last.from; last.t = j.t; }
    else merged.push({ ...j });
  }

  const api = requests.filter((r) => /\/api\//.test(r.url));
  const counts = {};
  api.forEach((r) => {
    const u = r.url.replace(/^https?:\/\/[^/]+/, '').split('?')[0];
    counts[u] = (counts[u] || 0) + 1;
  });
  const dupes = Object.entries(counts).filter(([, n]) => n > 1).map(([u, n]) => ({ url: u, count: n }));
  const slow = api
    .filter((r) => r.end)
    .map((r) => ({ url: r.url.replace(/^https?:\/\/[^/]+/, ''), ms: r.end - r.start, start: r.start }))
    .sort((a, b) => b.ms - a.ms)
    .slice(0, 15);

  const report = {
    tag: TAG,
    url: URL,
    domContentLoadedMs: tDom,
    cls: Number(data.cls.toFixed(4)),
    clsEntries: data.clsEntries,
    scrollHeightJumps: merged,
    jumpCount: merged.length,
    finalScrollHeight: data.docH,
    apiRequestCount: api.length,
    apiRequests: api.map((r) => ({
      url: r.url.replace(/^https?:\/\/[^/]+/, ''),
      start: r.start,
      ms: r.end ? r.end - r.start : null,
      status: r.status,
    })),
    duplicateRequests: dupes,
    slowRequests: slow,
    allRequestCount: requests.length,
    homeTextSample: data.homeText.slice(0, 400),
  };
  fs.writeFileSync(path.join(OUT, `probe-${TAG}.json`), JSON.stringify(report, null, 2));

  console.log('=== ' + TAG + ' ===');
  console.log('DOMContentLoaded:', tDom, 'ms');
  console.log('CLS:', report.cls);
  console.log('scrollHeight 突变次数:', merged.length);
  merged.forEach((j) => console.log('  t=' + j.t + 'ms', j.from, '->', j.to, '(' + j.delta + ')'));
  console.log('API 请求数:', api.length, ' 总请求:', requests.length);
  console.log('重复请求:', JSON.stringify(dupes));
  console.log('慢请求 top5:');
  slow.slice(0, 5).forEach((s) => console.log('  ', s.ms + 'ms', s.url));
  console.log('CLS 明细 top5:');
  report.clsEntries.sort((a, b) => b.value - a.value).slice(0, 5).forEach((e) =>
    console.log('  ', e.value.toFixed(4), 't=' + e.t, JSON.stringify(e.sources)));

  await browser.close();
})().catch((e) => { console.error('PROBE FAILED', e); process.exit(1); });
