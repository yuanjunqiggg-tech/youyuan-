// 截图 + 首屏高度测量：node shot.js <tag> <width> <height>
const { chromium } = require("playwright-core");

const tag = process.argv[2] || "before";
const W = parseInt(process.argv[3] || "1440", 10);
const H = parseInt(process.argv[4] || "900", 10);
const OUT = "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/qq-tools-declutter";

(async () => {
  const browser = await chromium.launch({
    executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
    headless: true,
    args: ["--no-sandbox", "--force-device-scale-factor=1"],
  });
  const page = await browser.newPage({ viewport: { width: W, height: H }, deviceScaleFactor: 1 });
  const errs = [];
  page.on("pageerror", (e) => errs.push(String(e)));
  await page.goto("http://127.0.0.1:19429/", { waitUntil: "load", timeout: 30000 });
  await page.waitForTimeout(1500);
  await page.evaluate(() => (typeof yyQQPage === "function" ? yyQQPage() : null));
  await page.waitForTimeout(2500);
  // 本地未登录：移除身份门禁遮罩，只看页面本身
  await page.evaluate(() => {
    const g = document.getElementById("yy-identity-gate");
    if (g) g.remove();
  });
  await page.waitForTimeout(300);

  // 只留群服互通页，避免首页导航干扰
  const info = await page.evaluate(() => {
    const root = document.getElementById("yyq-root");
    const sub = document.getElementById("sub-page");
    const q = (s) => { const e = document.querySelector(s); return e ? Math.round(e.getBoundingClientRect().height) : null; };
    return {
      hasRoot: !!root,
      subTop: sub ? Math.round(sub.getBoundingClientRect().top) : null,
      rootH: root ? Math.round(root.getBoundingClientRect().height) : null,
      statusH: q(".yyq-status"),
      heroH: q(".yyq-hero"),
      capsH: q(".yyq-caps"),
      streamH: q(".yyq-stream"),
      foldGridH: q(".yyq-fold-grid"),
      capsRows: document.querySelectorAll("#yyq-caps .yyq-cap").length,
      docH: document.documentElement.scrollHeight,
      folds: Array.from(document.querySelectorAll("#yyq-root .yyq-fold > summary")).map((s) => s.innerText.replace(/\s+/g, " ").trim()),
      firstScreenBottom: (() => {
        const el = document.elementFromPoint(10, window.innerHeight - 4);
        return el ? (el.closest(".yyq-caps, .yyq-stream, .yyq-fold, .yyq-hero, .yyq-status") || {}).className || "" : "";
      })(),
    };
  });
  console.log(JSON.stringify({ tag, W, H, info, errs }, null, 2));

  await page.screenshot({ path: `${OUT}/${tag}-${W}x${H}.png`, fullPage: false });
  await page.screenshot({ path: `${OUT}/${tag}-${W}x${H}-full.png`, fullPage: true });
  await browser.close();
})();
