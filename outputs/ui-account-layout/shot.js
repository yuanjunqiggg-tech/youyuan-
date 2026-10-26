// 真实 Chrome 截图：1440x900 与 390x844，同时输出关键元素的对齐量测。
const { chromium } = require("playwright-core");
const path = require("path");

const URL = process.argv[2] || "http://127.0.0.1:8791/__h.html";
const TAG = process.argv[3] || "before";
const PAGE = process.argv[4] || (URL.indexOf("__p.html") >= 0 ? "pool" : "accounts");
const OUT = __dirname;

(async () => {
  const browser = await chromium.launch({ executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe" });
  const report = {};

  for (const vp of [{ name: "desktop", width: 1440, height: 900 }, { name: "mobile", width: 390, height: 844 }]) {
    const ctx = await browser.newContext({ viewport: { width: vp.width, height: vp.height }, deviceScaleFactor: 1, locale: "zh-CN" });
    const page = await ctx.newPage();
    const errs = [];
    page.on("pageerror", (e) => errs.push(String(e)));
    await page.goto(URL, { waitUntil: "networkidle" });
    await page.waitForTimeout(300);

    await page.screenshot({ path: path.join(OUT, `shot-${PAGE}-${vp.name}-${TAG}-fold.png`), fullPage: false });
    await page.screenshot({ path: path.join(OUT, `shot-${PAGE}-${vp.name}-${TAG}.png`), fullPage: true });

    const m = await page.evaluate(() => {
      const r = (el) => { if (!el) return null; const b = el.getBoundingClientRect(); return { x: +b.x.toFixed(1), y: +b.y.toFixed(1), w: +b.width.toFixed(1), h: +b.height.toFixed(1) }; };
      const q = (s) => document.querySelector(s);
      const qa = (s) => Array.from(document.querySelectorAll(s));
      const rows = qa(".yy-account-summary").map((el) => {
        const g = getComputedStyle(el).gridTemplateColumns;
        const av = el.querySelector(".yy-compact-avatar");
        const nm = el.querySelector(".yy-compact-identity strong");
        const st = el.querySelector(".yy-compact-status");
        const ch = el.querySelector(".yy-row-chevron");
        return { grid: g, avatar: r(av), name: r(nm), status: r(st), chevron: r(ch), summary: r(el) };
      });
      return {
        viewport: { w: innerWidth, h: innerHeight },
        contentInner: r(q("#tab-accounts .content-inner")),
        content: r(q("#tab-accounts")),
        heading: r(q(".yy-heading")),
        toolbar: r(q(".yy-account-toolbar.yy-cred-toolbar")),
        overview: r(q(".yy-account-overview")),
        centerTabs: r(q(".yy-center-tabs")),
        projects: r(q(".yy-projects")),
        searchbar: r(q(".yy-account-toolbar:not(.yy-cred-toolbar)")),
        grid: r(q("#yy-accounts-grid")),
        firstCard: r(q(".yy-account-card")),
        hero: r(q(".yy-acct-hero")),
        heroFigure: r(q(".yy-acct-figure")),
        heroFacts: r(q(".yy-acct-hero-facts")),
        rows,
        hiddenNote: r(q(".yy-account-hidden-note")),
        poolPanel: r(q(".yy-pool-panel")),
        poolRow: r(q(".yy-pool-row")),
        poolHit: r(q(".yy-pool-hit")),
        poolSide: r(q(".yy-pool-side")),
        docScrollW: document.documentElement.scrollWidth,
        docClientW: document.documentElement.clientWidth
      };
    });
    report[vp.name] = { metrics: m, errors: errs };
    await ctx.close();
  }

  await browser.close();
  require("fs").writeFileSync(path.join(OUT, `metrics-${PAGE}-${TAG}.json`), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report, null, 2).slice(0, 4000));
})().catch((e) => { console.error("FAIL", e); process.exit(1); });
